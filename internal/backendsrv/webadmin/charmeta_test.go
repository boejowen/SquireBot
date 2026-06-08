package webadmin

// charmeta_test.go — Phase 16 Task 1 (TDD). Proves the char-meta endpoints are
// gated by login ONLY (D-03 — a NON-officer authenticated member can write, and
// the class/level/race/is_bank_toon columns actually change), value-set + range
// validated server-side (class ∈ enrich.CLASSES, race ∈ enrich.RACES, level blank
// → NULL or 1–60 — never trust the form's <select>, T-15-29 / Pitfall 5), scoped to
// existing non-removed characters (ErrCharNotFound → 400 invalid_input), and
// audited ("char_meta_set"). The route-level gate (RequireSession, NOT
// RequireOfficer) is asserted by TestWriteRoutes_Gates in
// cmd/squirebot-server/main_test.go (anon → 401, plain MEMBER session → admitted);
// here we exercise the handler logic with the caller injected via withCaller — and
// the caller is a PLAIN MEMBER (never seeded into guild_admins) to prove D-03.
//
// Shared helpers reused from officers_test.go / coin_test.go / eviction_test.go
// (same package): withCaller, postJSON, decodeErr, auditCount, seedPlainMember,
// itoa, store.NewTestDB.

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// charMetaInsertChar inserts a plain character (is_bank_toon=0, is_removed=0,
// class/level/race NULL) under a throwaway owner and returns its character id —
// the "existing char created by its first watcher upload" the form edits.
func charMetaInsertChar(t *testing.T, ctx context.Context, db *sql.DB, name string) int64 {
	t.Helper()
	res, err := db.ExecContext(ctx, `INSERT INTO owner (label) VALUES (?)`, "owner-"+name)
	if err != nil {
		t.Fatalf("insert owner: %v", err)
	}
	ownerID, _ := res.LastInsertId()
	res, err = db.ExecContext(ctx,
		`INSERT INTO character (owner_id, name) VALUES (?, ?)`, ownerID, name)
	if err != nil {
		t.Fatalf("insert char %q: %v", name, err)
	}
	id, _ := res.LastInsertId()
	return id
}

// charMetaInsertRemovedChar inserts a soft-removed character (is_removed=1) — the
// form must NOT be able to edit it (ErrCharNotFound → 400).
func charMetaInsertRemovedChar(t *testing.T, ctx context.Context, db *sql.DB, name string) int64 {
	t.Helper()
	res, err := db.ExecContext(ctx, `INSERT INTO owner (label) VALUES (?)`, "owner-"+name)
	if err != nil {
		t.Fatalf("insert owner: %v", err)
	}
	ownerID, _ := res.LastInsertId()
	res, err = db.ExecContext(ctx,
		`INSERT INTO character (owner_id, name, is_removed) VALUES (?, ?, 1)`, ownerID, name)
	if err != nil {
		t.Fatalf("insert removed char %q: %v", name, err)
	}
	id, _ := res.LastInsertId()
	return id
}

// readCharMeta reads back the four char-meta columns as nullable values.
func readCharMeta(t *testing.T, ctx context.Context, db *sql.DB, charID int64) (class sql.NullString, level sql.NullInt64, race sql.NullString, isBankToon int) {
	t.Helper()
	if err := db.QueryRowContext(ctx,
		`SELECT class, level, race, is_bank_toon FROM character WHERE id = ?`, charID,
	).Scan(&class, &level, &race, &isBankToon); err != nil {
		t.Fatalf("read char meta (id=%d): %v", charID, err)
	}
	return
}

// TestCharMetaSet_NonOfficerCanWrite is the D-03 proof: a plain authenticated
// member (no guild_admins row) can POST char-meta AND the columns actually change
// (read back, not just asserted on the response).
func TestCharMetaSet_NonOfficerCanWrite(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	member := "555555555555555555"
	seedPlainMember(t, ctx, db, member, "PlainMember")

	// Sanity: this caller is NOT an officer.
	if ok, _ := store.IsOfficer(ctx, db, member); ok {
		t.Fatalf("test setup wrong: member must NOT be an officer")
	}

	charID := charMetaInsertChar(t, ctx, db, "Slampeach")

	h := withCaller(member, CharMetaSetHandler(db))
	rec := postJSON(t, h, `{"character_id":`+itoa(charID)+`,"class":"WAR","level":50,"race":"IKS","is_bank_toon":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}

	// The columns ACTUALLY changed (D-03 proven, not just asserted on the response).
	// Phase 26 reconciliation: char-meta no longer writes is_bank_toon (it became the
	// officer-only "guild bank" designation, store.DesignateCharTx), so the body's
	// is_bank_toon is now ignored by the member path — only class/level/race land.
	class, level, race, isBank := readCharMeta(t, ctx, db, charID)
	if !class.Valid || class.String != "WAR" {
		t.Errorf("class = %v, want WAR", class)
	}
	if !level.Valid || level.Int64 != 50 {
		t.Errorf("level = %v, want 50", level)
	}
	if !race.Valid || race.String != "IKS" {
		t.Errorf("race = %v, want IKS", race)
	}
	// is_bank_toon stays 0: the member char-meta path no longer touches it (officer-only now).
	if isBank != 0 {
		t.Errorf("is_bank_toon = %d, want 0 (member char-meta no longer writes it; officer-only)", isBank)
	}
	// Audited (the writer's discord id is recorded for accountability).
	if c := auditCount(t, ctx, db, "char_meta_set"); c != 1 {
		t.Errorf("char_meta_set audit rows = %d, want 1", c)
	}
}

func TestCharMetaSet_RejectsBadClass(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	member := "555555555555555555"
	seedPlainMember(t, ctx, db, member, "PlainMember")
	charID := charMetaInsertChar(t, ctx, db, "Slampeach")

	h := withCaller(member, CharMetaSetHandler(db))
	// A display name ("Warrior") instead of the abbreviation ("WAR") — Pitfall 5.
	rec := postJSON(t, h, `{"character_id":`+itoa(charID)+`,"class":"Warrior","level":50,"race":"IKS","is_bank_toon":false}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	if got := decodeErr(t, rec); got != "invalid_input" {
		t.Errorf("error = %q, want invalid_input", got)
	}
	// Nothing was written (still NULL).
	class, _, _, _ := readCharMeta(t, ctx, db, charID)
	if class.Valid {
		t.Errorf("class written despite invalid attempt: %v", class)
	}
}

func TestCharMetaSet_RejectsBadRace(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	member := "555555555555555555"
	seedPlainMember(t, ctx, db, member, "PlainMember")
	charID := charMetaInsertChar(t, ctx, db, "Slampeach")

	h := withCaller(member, CharMetaSetHandler(db))
	// A display name ("Iksar") instead of the abbreviation ("IKS") — Pitfall 5.
	rec := postJSON(t, h, `{"character_id":`+itoa(charID)+`,"class":"WAR","level":50,"race":"Iksar","is_bank_toon":false}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	if got := decodeErr(t, rec); got != "invalid_input" {
		t.Errorf("error = %q, want invalid_input", got)
	}
	race := sql.NullString{}
	if err := db.QueryRowContext(ctx, `SELECT race FROM character WHERE id = ?`, charID).Scan(&race); err != nil {
		t.Fatalf("read race: %v", err)
	}
	if race.Valid {
		t.Errorf("race written despite invalid attempt: %v", race)
	}
}

func TestCharMetaSet_RejectsBadLevel(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	member := "555555555555555555"
	seedPlainMember(t, ctx, db, member, "PlainMember")

	h := withCaller(member, CharMetaSetHandler(db))

	// level 0 and 61 are out of the 1–60 range → 400 invalid_input, nothing written.
	for _, tc := range []struct {
		name  string
		level string
	}{
		{"level=0", "0"},
		{"level=61", "61"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			charID := charMetaInsertChar(t, ctx, db, "BadLevel-"+tc.name)
			rec := postJSON(t, h, `{"character_id":`+itoa(charID)+`,"class":"WAR","level":`+tc.level+`,"race":"IKS","is_bank_toon":false}`)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
			}
			if got := decodeErr(t, rec); got != "invalid_input" {
				t.Errorf("error = %q, want invalid_input", got)
			}
			class, level, _, _ := readCharMeta(t, ctx, db, charID)
			if class.Valid || level.Valid {
				t.Errorf("meta written despite invalid level: class=%v level=%v", class, level)
			}
		})
	}

	// A null/omitted level is allowed (blank = unset) → 200 with the level column NULL.
	t.Run("level=null is allowed", func(t *testing.T) {
		charID := charMetaInsertChar(t, ctx, db, "NullLevel")
		rec := postJSON(t, h, `{"character_id":`+itoa(charID)+`,"class":"WAR","level":null,"race":"IKS","is_bank_toon":false}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
		}
		class, level, _, _ := readCharMeta(t, ctx, db, charID)
		if !class.Valid || class.String != "WAR" {
			t.Errorf("class = %v, want WAR (the rest of the write should still land)", class)
		}
		if level.Valid {
			t.Errorf("level = %v, want NULL (blank level stays unset)", level)
		}
	})
}

func TestCharMetaSet_RejectsRemovedOrMissingChar(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	member := "555555555555555555"
	seedPlainMember(t, ctx, db, member, "PlainMember")

	h := withCaller(member, CharMetaSetHandler(db))

	// A non-existent character id → ErrCharNotFound → 400 invalid_input.
	t.Run("missing char", func(t *testing.T) {
		rec := postJSON(t, h, `{"character_id":999999,"class":"WAR","level":50,"race":"IKS","is_bank_toon":false}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
		}
		if got := decodeErr(t, rec); got != "invalid_input" {
			t.Errorf("error = %q, want invalid_input", got)
		}
	})

	// An is_removed=1 char must NOT be editable (the form edits live chars only).
	t.Run("removed char", func(t *testing.T) {
		charID := charMetaInsertRemovedChar(t, ctx, db, "Removed")
		rec := postJSON(t, h, `{"character_id":`+itoa(charID)+`,"class":"WAR","level":50,"race":"IKS","is_bank_toon":false}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
		}
		if got := decodeErr(t, rec); got != "invalid_input" {
			t.Errorf("error = %q, want invalid_input", got)
		}
		// Nothing changed on the removed row.
		class, _, _, _ := readCharMeta(t, ctx, db, charID)
		if class.Valid {
			t.Errorf("class written on a removed char: %v", class)
		}
	})

	// No audit row for either rejected write.
	if c := auditCount(t, ctx, db, "char_meta_set"); c != 0 {
		t.Errorf("char_meta_set audit rows = %d, want 0 (rejected writes audit nothing)", c)
	}
}

func TestCharMetaList_ReturnsExistingChars(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	member := "555555555555555555"
	seedPlainMember(t, ctx, db, member, "PlainMember")

	idA := charMetaInsertChar(t, ctx, db, "Aaachar")
	idB := charMetaInsertChar(t, ctx, db, "Bbbchar")

	listH := withCaller(member, CharMetaListHandler(db))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	listH.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d (body=%s)", rec.Code, rec.Body.String())
	}

	var chars []store.CharMeta
	if err := json.Unmarshal(rec.Body.Bytes(), &chars); err != nil {
		t.Fatalf("decode char list: %v", err)
	}
	foundA, foundB := false, false
	for _, c := range chars {
		if c.ID == idA && c.Name == "Aaachar" {
			foundA = true
		}
		if c.ID == idB && c.Name == "Bbbchar" {
			foundB = true
		}
	}
	if !foundA || !foundB {
		t.Errorf("char list missing entries: foundA=%v foundB=%v (got %+v)", foundA, foundB, chars)
	}
}

func TestCharMetaList_EmptyIsArrayNotNull(t *testing.T) {
	db := store.NewTestDB(t)
	member := "555555555555555555"
	seedPlainMember(t, context.Background(), db, member, "PlainMember")

	listH := withCaller(member, CharMetaListHandler(db))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	listH.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d", rec.Code)
	}
	// Empty must serialize as [] (not null) so the UI shows the no-chars state.
	if body := rec.Body.String(); body != "[]\n" && body != "[]" {
		t.Errorf("empty char list body = %q, want []", body)
	}
}
