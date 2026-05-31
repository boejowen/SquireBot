package webadmin

// eviction_test.go — Task 2 (TDD). Proves the eviction/restore endpoints port
// v1's per-owner cascade + 30-day grace + guild-code revoke (D-09/D-10) over
// HTTP, that authorization is re-checked INSIDE the write tx (WR-04 — a
// non-officer evict writes nothing), that restore-during-grace re-mints a fresh
// guild code and refuses once archived (W-2/D-10), and that the DAILY archive job
// is idempotent (W-3). Behavioral oracle: showEvictionSidebar.ts +
// weeklyEvictionArchive.ts.

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// --- local test fixtures (the store package's test helpers are not exported) --

func evInsertOwner(t *testing.T, ctx context.Context, db *sql.DB, label string) int64 {
	t.Helper()
	res, err := db.ExecContext(ctx, `INSERT INTO owner (label) VALUES (?)`, label)
	if err != nil {
		t.Fatalf("insert owner %q: %v", label, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("owner last insert id: %v", err)
	}
	return id
}

func evInsertChar(t *testing.T, ctx context.Context, db *sql.DB, ownerID int64, name string) int64 {
	t.Helper()
	res, err := db.ExecContext(ctx,
		`INSERT INTO character (owner_id, name, is_bank_toon) VALUES (?, ?, 0)`, ownerID, name)
	if err != nil {
		t.Fatalf("insert character %q: %v", name, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("character last insert id: %v", err)
	}
	return id
}

func evInsertGuildCode(t *testing.T, ctx context.Context, db *sql.DB, ownerID int64, label string) {
	t.Helper()
	if _, err := db.ExecContext(ctx,
		`INSERT INTO guild_code (owner_id, token_hash, label) VALUES (?, ?, ?)`,
		ownerID, []byte("hash-"+label), label); err != nil {
		t.Fatalf("insert guild_code for owner %d: %v", ownerID, err)
	}
}

func charIsRemoved(t *testing.T, ctx context.Context, db *sql.DB, charID int64) int {
	t.Helper()
	var r int
	if err := db.QueryRowContext(ctx, `SELECT is_removed FROM character WHERE id = ?`, charID).Scan(&r); err != nil {
		t.Fatalf("read is_removed (id=%d): %v", charID, err)
	}
	return r
}

// activeCodeCount counts the owner's still-active (not-disabled) guild codes.
func activeCodeCount(t *testing.T, ctx context.Context, db *sql.DB, ownerID int64) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM guild_code WHERE owner_id = ? AND disabled_at IS NULL`, ownerID).Scan(&n); err != nil {
		t.Fatalf("count active codes (owner=%d): %v", ownerID, err)
	}
	return n
}

// totalCodeCount counts all guild_code rows for the owner (active + disabled).
func totalCodeCount(t *testing.T, ctx context.Context, db *sql.DB, ownerID int64) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM guild_code WHERE owner_id = ?`, ownerID).Scan(&n); err != nil {
		t.Fatalf("count codes (owner=%d): %v", ownerID, err)
	}
	return n
}

// --- evict ------------------------------------------------------------------

func TestEvict_OfficerCascadesAndRevokes_Audits(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	floor := "111111111111111111"
	seedFloorAndUsers(t, ctx, db, floor, nil)

	ownerID := evInsertOwner(t, ctx, db, "Guildie-A")
	c1 := evInsertChar(t, ctx, db, ownerID, "Charone")
	c2 := evInsertChar(t, ctx, db, ownerID, "Chartwo")
	evInsertGuildCode(t, ctx, db, ownerID, "code-A")

	h := withCaller(floor, EvictHandler(db))
	rec := postJSON(t, h, `{"owner_id":`+itoa(ownerID)+`}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		RemovedCount int   `json:"removed_count"`
		GraceUntil   int64 `json:"grace_until"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode evict resp: %v", err)
	}
	if resp.RemovedCount != 2 {
		t.Errorf("removed_count = %d, want 2", resp.RemovedCount)
	}
	if resp.GraceUntil == 0 {
		t.Errorf("grace_until = 0, want a future epoch")
	}
	// Both chars removed in the one operation.
	if charIsRemoved(t, ctx, db, c1) != 1 || charIsRemoved(t, ctx, db, c2) != 1 {
		t.Errorf("not all chars removed: c1=%d c2=%d", charIsRemoved(t, ctx, db, c1), charIsRemoved(t, ctx, db, c2))
	}
	// Guild code revoked (D-10) in the same operation.
	if activeCodeCount(t, ctx, db, ownerID) != 0 {
		t.Errorf("active code count = %d, want 0 (code revoked)", activeCodeCount(t, ctx, db, ownerID))
	}
	// Audited.
	if c := auditCount(t, ctx, db, "eviction"); c != 1 {
		t.Errorf("eviction audit rows = %d, want 1", c)
	}
}

func TestEvict_NonOfficerRejected_NothingChanged(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	floor := "111111111111111111"
	stranger := "999999999999999999"
	seedFloorAndUsers(t, ctx, db, floor, map[string]string{stranger: "Stranger"})

	ownerID := evInsertOwner(t, ctx, db, "Guildie-A")
	c1 := evInsertChar(t, ctx, db, ownerID, "Charone")
	evInsertGuildCode(t, ctx, db, ownerID, "code-A")

	// Stranger is NOT an officer → the in-tx re-check must reject (WR-04).
	h := withCaller(stranger, EvictHandler(db))
	rec := postJSON(t, h, `{"owner_id":`+itoa(ownerID)+`}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (body=%s)", rec.Code, rec.Body.String())
	}
	if got := decodeErr(t, rec); got != "not_authorized" {
		t.Errorf("error = %q, want not_authorized", got)
	}
	// NOTHING changed: char still live, code still active, no audit row.
	if charIsRemoved(t, ctx, db, c1) != 0 {
		t.Errorf("char removed despite unauthorized caller")
	}
	if activeCodeCount(t, ctx, db, ownerID) != 1 {
		t.Errorf("code revoked despite unauthorized caller")
	}
	if c := auditCount(t, ctx, db, "eviction"); c != 0 {
		t.Errorf("eviction audit rows = %d, want 0", c)
	}
}

// TestEvict_PeerCannotEvictFloorData proves the D-09 owner-floor data protection:
// a peer officer cannot evict the owner whose data belongs to the maintainer
// (resolved by owner.label == the floor web_user's username — the documented
// linkage absent a real owner↔discord FK).
func TestEvict_PeerCannotEvictFloorData(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	floor := "111111111111111111"
	peer := "222222222222222222"
	// The floor's web_user.username doubles as the owner label bridge.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO web_user (discord_user_id, username, avatar, first_seen, last_login)
		 VALUES (?, ?, NULL, 0, 0)`, floor, "MaintainerLabel"); err != nil {
		t.Fatalf("seed floor web_user: %v", err)
	}
	if err := store.SetOwnerFloor(ctx, db, floor, 1700000000); err != nil {
		t.Fatalf("SetOwnerFloor: %v", err)
	}
	// SetOwnerFloor seeds a placeholder web_user with username=snowflake via INSERT
	// OR IGNORE; our explicit row above wins (inserted first), so username is the label.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO web_user (discord_user_id, username, avatar, first_seen, last_login)
		 VALUES (?, ?, NULL, 0, 0)`, peer, "PeerOfficer"); err != nil {
		t.Fatalf("seed peer web_user: %v", err)
	}
	if err := commitTxHelper(t, ctx, db, peer, floor); err != nil {
		t.Fatalf("promote peer: %v", err)
	}

	// The floor's protected owner (label == the floor's username).
	floorOwnerID := evInsertOwner(t, ctx, db, "MaintainerLabel")
	cf := evInsertChar(t, ctx, db, floorOwnerID, "Floortoon")

	h := withCaller(peer, EvictHandler(db))
	rec := postJSON(t, h, `{"owner_id":`+itoa(floorOwnerID)+`}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (body=%s)", rec.Code, rec.Body.String())
	}
	if got := decodeErr(t, rec); got != "owner_floor_protected" {
		t.Errorf("error = %q, want owner_floor_protected", got)
	}
	if charIsRemoved(t, ctx, db, cf) != 0 {
		t.Errorf("floor's char was evicted by a peer (owner-floor protection failed)")
	}
}

// TestEvict_PeerCannotEvictFloorData_CaseAndWhitespaceInsensitive is the WR-05
// regression: the owner.label↔floor-username bridge must match case- and
// whitespace-insensitively. Before the fix the match was a plain `label = ?`, so
// a floor username of "MaintainerLabel" against an owner label of "  maintainerlabel "
// (Discord usernames + watcher-supplied labels drift in case/whitespace) FAILED
// to match → callerMayNotEvictFloor returned "not protected" → a peer COULD evict
// the maintainer's data (fail-OPEN). The hardened TRIM(...) COLLATE NOCASE query
// must still protect it.
func TestEvict_PeerCannotEvictFloorData_CaseAndWhitespaceInsensitive(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	floor := "111111111111111111"
	peer := "222222222222222222"
	// Floor username in one casing.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO web_user (discord_user_id, username, avatar, first_seen, last_login)
		 VALUES (?, ?, NULL, 0, 0)`, floor, "MaintainerLabel"); err != nil {
		t.Fatalf("seed floor web_user: %v", err)
	}
	if err := store.SetOwnerFloor(ctx, db, floor, 1700000000); err != nil {
		t.Fatalf("SetOwnerFloor: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO web_user (discord_user_id, username, avatar, first_seen, last_login)
		 VALUES (?, ?, NULL, 0, 0)`, peer, "PeerOfficer"); err != nil {
		t.Fatalf("seed peer web_user: %v", err)
	}
	if err := commitTxHelper(t, ctx, db, peer, floor); err != nil {
		t.Fatalf("promote peer: %v", err)
	}

	// The owner label differs ONLY by case + surrounding whitespace from the floor
	// username — it must still resolve as the floor's protected owner.
	floorOwnerID := evInsertOwner(t, ctx, db, "  maintainerlabel ")
	cf := evInsertChar(t, ctx, db, floorOwnerID, "Floortoon")

	h := withCaller(peer, EvictHandler(db))
	rec := postJSON(t, h, `{"owner_id":`+itoa(floorOwnerID)+`}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (body=%s)", rec.Code, rec.Body.String())
	}
	if got := decodeErr(t, rec); got != "owner_floor_protected" {
		t.Errorf("error = %q, want owner_floor_protected", got)
	}
	if charIsRemoved(t, ctx, db, cf) != 0 {
		t.Errorf("floor's char was evicted by a peer despite a case/whitespace-only label drift (WR-05 fail-open)")
	}
}

// --- restore (re-mint) ------------------------------------------------------

func TestRestore_DuringGrace_ReMintsCode_Audits(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	floor := "111111111111111111"
	seedFloorAndUsers(t, ctx, db, floor, nil)

	ownerID := evInsertOwner(t, ctx, db, "Guildie-A")
	c1 := evInsertChar(t, ctx, db, ownerID, "Charone")
	evInsertGuildCode(t, ctx, db, ownerID, "code-A")

	// Evict first (officer).
	evictH := withCaller(floor, EvictHandler(db))
	if rec := postJSON(t, evictH, `{"owner_id":`+itoa(ownerID)+`}`); rec.Code != http.StatusOK {
		t.Fatalf("evict status = %d", rec.Code)
	}
	// After eviction: 1 code, disabled. The restore must MINT A NEW one.
	if total := totalCodeCount(t, ctx, db, ownerID); total != 1 {
		t.Fatalf("pre-restore total codes = %d, want 1", total)
	}

	restoreH := withCaller(floor, RestoreHandler(db))
	rec := postJSON(t, restoreH, `{"owner_id":`+itoa(ownerID)+`}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("restore status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		RestoredCount int  `json:"restored_count"`
		NewCodeIssued bool `json:"new_code_issued"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode restore resp: %v", err)
	}
	if resp.RestoredCount != 1 {
		t.Errorf("restored_count = %d, want 1", resp.RestoredCount)
	}
	if !resp.NewCodeIssued {
		t.Errorf("new_code_issued = false, want true (D-10 re-mint)")
	}
	// Char is live again.
	if charIsRemoved(t, ctx, db, c1) != 0 {
		t.Errorf("char still removed after restore")
	}
	// A FRESH guild_code row now exists for the owner (re-mint proven): total grew to
	// 2 (old disabled + new active), and there is exactly one ACTIVE code now.
	if total := totalCodeCount(t, ctx, db, ownerID); total != 2 {
		t.Errorf("post-restore total codes = %d, want 2 (old disabled + new minted)", total)
	}
	if active := activeCodeCount(t, ctx, db, ownerID); active != 1 {
		t.Errorf("post-restore active codes = %d, want 1 (the re-minted code)", active)
	}
	if c := auditCount(t, ctx, db, "eviction_restore"); c != 1 {
		t.Errorf("eviction_restore audit rows = %d, want 1", c)
	}
}

func TestRestore_AfterArchive_GraceExpired(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	floor := "111111111111111111"
	seedFloorAndUsers(t, ctx, db, floor, nil)

	ownerID := evInsertOwner(t, ctx, db, "Guildie-A")
	c1 := evInsertChar(t, ctx, db, ownerID, "Charone")
	evInsertGuildCode(t, ctx, db, ownerID, "code-A")

	// Evict directly via the store with a grace already in the past, then archive —
	// simulating a past-grace owner the restore must refuse.
	if err := commitTxRaw(t, ctx, db, func(tx *sql.Tx) error {
		_, _, e := store.EvictOwnerTx(ctx, tx, ownerID, 1_000_000-store.EvictionGraceSeconds-10)
		return e
	}); err != nil {
		t.Fatalf("seed past-grace eviction: %v", err)
	}
	if n, err := store.ArchiveExpiredEvictions(ctx, db, 1_000_000); err != nil || n != 1 {
		t.Fatalf("archive setup: n=%d err=%v, want 1/nil", n, err)
	}

	restoreH := withCaller(floor, RestoreHandler(db))
	rec := postJSON(t, restoreH, `{"owner_id":`+itoa(ownerID)+`}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (body=%s)", rec.Code, rec.Body.String())
	}
	if got := decodeErr(t, rec); got != "grace_expired" {
		t.Errorf("error = %q, want grace_expired", got)
	}
	// Archived char NOT revived; no new code minted; no restore audit row.
	if charIsRemoved(t, ctx, db, c1) != 1 {
		t.Errorf("archived char was revived despite grace_expired")
	}
	if total := totalCodeCount(t, ctx, db, ownerID); total != 1 {
		t.Errorf("a code was minted on a refused restore (total=%d, want 1)", total)
	}
	if c := auditCount(t, ctx, db, "eviction_restore"); c != 0 {
		t.Errorf("eviction_restore audit rows = %d, want 0 (refused)", c)
	}
}

// --- archive job idempotency (W-3) ------------------------------------------

func TestArchiveJob_Idempotent(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()

	ownerID := evInsertOwner(t, ctx, db, "Guildie-A")
	evInsertChar(t, ctx, db, ownerID, "Charone")

	// Past-grace eviction via the store.
	if err := commitTxRaw(t, ctx, db, func(tx *sql.Tx) error {
		_, _, e := store.EvictOwnerTx(ctx, tx, ownerID, 1_000_000-store.EvictionGraceSeconds-10)
		return e
	}); err != nil {
		t.Fatalf("seed past-grace eviction: %v", err)
	}

	first, err := store.ArchiveExpiredEvictions(ctx, db, 1_000_000)
	if err != nil || first != 1 {
		t.Fatalf("first archive = %d (err=%v), want 1", first, err)
	}
	second, err := store.ArchiveExpiredEvictions(ctx, db, 1_000_001)
	if err != nil || second != 0 {
		t.Errorf("second archive = %d (err=%v), want 0 (idempotent)", second, err)
	}
}

// --- tiny helpers -----------------------------------------------------------

// itoa avoids importing strconv at every call site in the table-ish tests.
func itoa(n int64) string {
	return jsonNumber(n)
}

func jsonNumber(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// commitTxRaw runs fn in a committed tx (for seeding via the store mutators).
func commitTxRaw(t *testing.T, ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	t.Helper()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if ferr := fn(tx); ferr != nil {
		_ = tx.Rollback()
		return ferr
	}
	if cerr := tx.Commit(); cerr != nil {
		t.Fatalf("commit tx: %v", cerr)
	}
	return nil
}

// commitTxHelper promotes target to officer (callerID must be an officer).
func commitTxHelper(t *testing.T, ctx context.Context, db *sql.DB, target, callerID string) error {
	t.Helper()
	return commitTxRaw(t, ctx, db, func(tx *sql.Tx) error {
		_, e := store.AddOfficerTx(ctx, tx, target, callerID, 1700000001)
		return e
	})
}
