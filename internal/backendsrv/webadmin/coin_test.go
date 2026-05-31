package webadmin

// coin_test.go — Task 3 (TDD). Proves the bank-coin endpoints are gated by login
// ONLY (D-12/B-1 — a NON-officer authenticated member can write, and the coin
// columns actually change), range-validated (gold/silver/copper 0–999, plat >= 0),
// bank-toon-gated (not_bank_toon), and audited. The route-level gate (RequireSession
// vs RequireOfficer) is asserted in cmd/squirebot-server/main_test.go; here we
// exercise the handler logic with the caller injected via withCaller — and the
// caller is a PLAIN MEMBER (never seeded into guild_admins) to prove D-12.

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// coinInsertBankToon inserts a bank toon (is_bank_toon=1) under a throwaway owner
// and returns its character id.
func coinInsertBankToon(t *testing.T, ctx context.Context, db *sql.DB, name string) int64 {
	t.Helper()
	res, err := db.ExecContext(ctx, `INSERT INTO owner (label) VALUES (?)`, "owner-"+name)
	if err != nil {
		t.Fatalf("insert owner: %v", err)
	}
	ownerID, _ := res.LastInsertId()
	res, err = db.ExecContext(ctx,
		`INSERT INTO character (owner_id, name, is_bank_toon) VALUES (?, ?, 1)`, ownerID, name)
	if err != nil {
		t.Fatalf("insert bank toon %q: %v", name, err)
	}
	id, _ := res.LastInsertId()
	return id
}

// coinInsertNonBankChar inserts a NON-bank character (is_bank_toon=0).
func coinInsertNonBankChar(t *testing.T, ctx context.Context, db *sql.DB, name string) int64 {
	t.Helper()
	res, err := db.ExecContext(ctx, `INSERT INTO owner (label) VALUES (?)`, "owner-"+name)
	if err != nil {
		t.Fatalf("insert owner: %v", err)
	}
	ownerID, _ := res.LastInsertId()
	res, err = db.ExecContext(ctx,
		`INSERT INTO character (owner_id, name, is_bank_toon) VALUES (?, ?, 0)`, ownerID, name)
	if err != nil {
		t.Fatalf("insert non-bank char %q: %v", name, err)
	}
	id, _ := res.LastInsertId()
	return id
}

// readCoin reads back the four coin columns as nullable ints.
func readCoin(t *testing.T, ctx context.Context, db *sql.DB, charID int64) (plat, gold, silver, copper sql.NullInt64) {
	t.Helper()
	if err := db.QueryRowContext(ctx,
		`SELECT plat, gold, silver, copper FROM character WHERE id = ?`, charID,
	).Scan(&plat, &gold, &silver, &copper); err != nil {
		t.Fatalf("read coin (id=%d): %v", charID, err)
	}
	return
}

// seedPlainMember inserts a web_user that is NOT an officer (the D-12 actor). No
// guild_admins row — so if the coin handler ever consulted officer status, this
// caller would be rejected, and the test would catch the regression.
func seedPlainMember(t *testing.T, ctx context.Context, db *sql.DB, id, name string) {
	t.Helper()
	if _, err := db.ExecContext(ctx,
		`INSERT INTO web_user (discord_user_id, username, avatar, first_seen, last_login)
		 VALUES (?, ?, NULL, 0, 0)`, id, name); err != nil {
		t.Fatalf("insert web_user %q: %v", id, err)
	}
}

// TestCoinSet_NonOfficerCanWrite is the D-12 / B-1 proof: a plain authenticated
// member (is_officer=false — no guild_admins row) can POST coin AND the coin
// columns change (read back, not just asserted on the response).
func TestCoinSet_NonOfficerCanWrite(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	member := "555555555555555555"
	seedPlainMember(t, ctx, db, member, "PlainMember")

	// Sanity: this caller is NOT an officer.
	if ok, _ := store.IsOfficer(ctx, db, member); ok {
		t.Fatalf("test setup wrong: member must NOT be an officer")
	}

	charID := coinInsertBankToon(t, ctx, db, "Banktoon")

	h := withCaller(member, CoinSetHandler(db))
	rec := postJSON(t, h, `{"character_id":`+itoa(charID)+`,"plat":1000,"gold":12,"silver":34,"copper":56}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}

	// The coin columns ACTUALLY changed (D-12 proven, not just asserted).
	plat, gold, silver, copper := readCoin(t, ctx, db, charID)
	if !plat.Valid || plat.Int64 != 1000 {
		t.Errorf("plat = %v, want 1000", plat)
	}
	if !gold.Valid || gold.Int64 != 12 || !silver.Valid || silver.Int64 != 34 || !copper.Valid || copper.Int64 != 56 {
		t.Errorf("g/s/c = %v/%v/%v, want 12/34/56", gold, silver, copper)
	}
	// Audited (the writer's discord id is recorded for accountability).
	if c := auditCount(t, ctx, db, "coin_set"); c != 1 {
		t.Errorf("coin_set audit rows = %d, want 1", c)
	}
}

func TestCoinSet_RejectsOutOfRange(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	member := "555555555555555555"
	seedPlainMember(t, ctx, db, member, "PlainMember")
	charID := coinInsertBankToon(t, ctx, db, "Banktoon")

	h := withCaller(member, CoinSetHandler(db))

	cases := []struct {
		name string
		body string
	}{
		{"gold=1000", `{"character_id":` + itoa(charID) + `,"plat":0,"gold":1000,"silver":0,"copper":0}`},
		{"silver=1000", `{"character_id":` + itoa(charID) + `,"plat":0,"gold":0,"silver":1000,"copper":0}`},
		{"copper=-1", `{"character_id":` + itoa(charID) + `,"plat":0,"gold":0,"silver":0,"copper":-1}`},
		{"plat=-1", `{"character_id":` + itoa(charID) + `,"plat":-1,"gold":0,"silver":0,"copper":0}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := postJSON(t, h, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
			}
			if got := decodeErr(t, rec); got != "invalid_input" {
				t.Errorf("error = %q, want invalid_input", got)
			}
		})
	}
	// Nothing was written (still NULL).
	plat, gold, silver, copper := readCoin(t, ctx, db, charID)
	if plat.Valid || gold.Valid || silver.Valid || copper.Valid {
		t.Errorf("coin written despite all-invalid attempts: %v/%v/%v/%v", plat, gold, silver, copper)
	}
}

func TestCoinSet_RejectsNonBankToon(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	member := "555555555555555555"
	seedPlainMember(t, ctx, db, member, "PlainMember")
	charID := coinInsertNonBankChar(t, ctx, db, "Regularchar")

	h := withCaller(member, CoinSetHandler(db))
	rec := postJSON(t, h, `{"character_id":`+itoa(charID)+`,"plat":5,"gold":1,"silver":2,"copper":3}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	if got := decodeErr(t, rec); got != "not_bank_toon" {
		t.Errorf("error = %q, want not_bank_toon", got)
	}
}

func TestCoinSet_SuccessPreFillsOnNextGet(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	member := "555555555555555555"
	seedPlainMember(t, ctx, db, member, "PlainMember")
	charID := coinInsertBankToon(t, ctx, db, "Banktoon")

	setH := withCaller(member, CoinSetHandler(db))
	if rec := postJSON(t, setH, `{"character_id":`+itoa(charID)+`,"plat":7,"gold":0,"silver":0,"copper":9}`); rec.Code != http.StatusOK {
		t.Fatalf("set status = %d", rec.Code)
	}

	// The bank-toons list pre-fills from the saved coin.
	listH := withCaller(member, BankToonsHandler(db))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	listH.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d", rec.Code)
	}
	var toons []store.BankToon
	if err := json.Unmarshal(rec.Body.Bytes(), &toons); err != nil {
		t.Fatalf("decode bank toons: %v", err)
	}
	var found bool
	for _, bt := range toons {
		if bt.CharacterID == charID {
			found = true
			if bt.Plat == nil || *bt.Plat != 7 || bt.Copper == nil || *bt.Copper != 9 {
				t.Errorf("pre-fill plat/copper = %v/%v, want 7/9", bt.Plat, bt.Copper)
			}
			// An entered 0 must be 0 (non-nil), distinguishable from unset.
			if bt.Gold == nil || *bt.Gold != 0 {
				t.Errorf("pre-fill gold = %v, want 0 (entered, non-nil)", bt.Gold)
			}
		}
	}
	if !found {
		t.Errorf("bank toon %d not in the list", charID)
	}
}
