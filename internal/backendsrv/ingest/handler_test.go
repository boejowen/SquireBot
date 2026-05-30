package ingest_test

// handler_test.go proves POST /api/v1/ingest end to end at the test tier
// (httptest + the 11-02 temp-DB fixture + the 11-04 mint), covering the
// BACKEND-03 round-trip (valid upload → row queryable, shrinking snapshot drops
// rows) and the BACKEND-04 / threat-register guarantees (401 writes nothing,
// cross-owner 409, oversized/bad-kind/malformed rejected with no write).
//
// It is an EXTERNAL test package (ingest_test) so it exercises the handler
// exactly as cmd/squirebot-server does: through the exported New() constructor
// and the exported auth/store/migrations APIs — no access to package internals.

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/auth"
	"github.com/boejowen/SquireBot/internal/backendsrv/ingest"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// newHandler wires a fresh migrated temp DB + bearer guard + ingest handler.
func newHandler(t *testing.T) (*ingest.Handler, *sql.DB) {
	t.Helper()
	db := store.NewTestDB(t) // Open + goose.Up + t.Cleanup (11-02 shared fixture)
	h := ingest.New(auth.New(db), db)
	return h, db
}

// post builds a POST /api/v1/ingest request (optionally with a Bearer token) and
// serves it through the handler, returning the recorder.
func post(t *testing.T, h *ingest.Handler, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// invBody builds an inventory envelope whose content has `n` real item rows.
func invBody(char string, n int) string {
	var sb strings.Builder
	sb.WriteString(`{"character":"` + char + `","kind":"inventory","watcher_version":"2.0.0","content":"`)
	for i := 0; i < n; i++ {
		// Location\tName\tID\tCount\tSlots — \n escaped for the JSON string.
		fmt.Fprintf(&sb, "General%d\\tItem%d\\t%d\\t1\\t0\\n", i, i, 1000+i)
	}
	sb.WriteString(`"}`)
	return sb.String()
}

// invBodyVer builds a 1-row inventory envelope with an explicit watcher_version
// (used by the 426 min-version gate cases). An empty ver omits nothing — it sends
// `"watcher_version":""` so the gate's `!= ""` empty-allow rule is exercised.
func invBodyVer(char, ver string) string {
	return `{"character":"` + char + `","kind":"inventory","watcher_version":"` + ver +
		`","content":"General0\tItem0\t1000\t1\t0\n"}`
}

// countInv returns the inventory_item row count for a character name (0 if the
// character does not exist yet).
func countInv(t *testing.T, db *sql.DB, char string) int {
	t.Helper()
	var n int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM inventory_item ii
		JOIN character c ON c.id = ii.character_id
		WHERE c.name = ?`, char).Scan(&n)
	if err != nil {
		t.Fatalf("countInv(%q): %v", char, err)
	}
	return n
}

// totalInv returns the total inventory_item row count across all characters.
func totalInv(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM inventory_item`).Scan(&n); err != nil {
		t.Fatalf("totalInv: %v", err)
	}
	return n
}

// TestIngest_ValidInventory_ReplacesRows: a valid Bearer + 3-row inventory POST
// returns 2xx AND the 3 rows are queryable back out (BACKEND-03 round-trip).
func TestIngest_ValidInventory_ReplacesRows(t *testing.T) {
	h, db := newHandler(t)
	code, err := auth.MintCode(db, "alice")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	rec := post(t, h, code, invBody("Slampeach", 3))

	if rec.Code != http.StatusNoContent && rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 204/200; body=%q", rec.Code, rec.Body.String())
	}
	if got := countInv(t, db, "Slampeach"); got != 3 {
		t.Errorf("inventory rows = %d, want 3", got)
	}
}

// TestIngest_NoAuthHeader_401_WritesNothing: a POST with NO Authorization header
// returns 401 AND the store is never touched (BACKEND-04 / T-11.05-01).
func TestIngest_NoAuthHeader_401_WritesNothing(t *testing.T) {
	h, db := newHandler(t)

	rec := post(t, h, "", invBody("Slampeach", 3))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if got := totalInv(t, db); got != 0 {
		t.Errorf("inventory_item count = %d, want 0 (401 must write nothing)", got)
	}
	// The character row must also not exist (no bind happened).
	if got := countInv(t, db, "Slampeach"); got != 0 {
		t.Errorf("rows for Slampeach = %d, want 0", got)
	}
}

// TestIngest_UnknownToken_401: a random (never-minted) bearer token returns 401
// and writes nothing.
func TestIngest_UnknownToken_401(t *testing.T) {
	h, db := newHandler(t)

	rec := post(t, h, "totally-bogus-token-that-was-never-minted", invBody("Slampeach", 2))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if got := totalInv(t, db); got != 0 {
		t.Errorf("inventory_item count = %d, want 0", got)
	}
}

// TestIngest_RevokedToken_401: a minted-then-revoked code returns 401 and writes
// nothing (D-09 wired through the handler).
func TestIngest_RevokedToken_401(t *testing.T) {
	h, db := newHandler(t)
	code, err := auth.MintCode(db, "alice")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	if err := auth.RevokeCode(db, "alice"); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	rec := post(t, h, code, invBody("Slampeach", 2))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 for revoked code", rec.Code)
	}
	if got := totalInv(t, db); got != 0 {
		t.Errorf("inventory_item count = %d, want 0", got)
	}
}

// TestIngest_ShrinkingSnapshot_DropsRows: POST 3 rows then POST 1 row (same
// char+code); only the new 1 row remains (full-snapshot replace, not append).
func TestIngest_ShrinkingSnapshot_DropsRows(t *testing.T) {
	h, db := newHandler(t)
	code, err := auth.MintCode(db, "alice")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	if rec := post(t, h, code, invBody("Slampeach", 3)); rec.Code >= 300 {
		t.Fatalf("first upload status = %d", rec.Code)
	}
	if got := countInv(t, db, "Slampeach"); got != 3 {
		t.Fatalf("after first upload rows = %d, want 3", got)
	}

	if rec := post(t, h, code, invBody("Slampeach", 1)); rec.Code >= 300 {
		t.Fatalf("second upload status = %d", rec.Code)
	}
	if got := countInv(t, db, "Slampeach"); got != 1 {
		t.Errorf("after shrinking upload rows = %d, want 1", got)
	}
}

// TestIngest_CrossOwner_409: char bound under owner A, then the same name POSTed
// under owner B's code → 409 AND A's rows are untouched (D-07 / T-11.05-04).
func TestIngest_CrossOwner_409(t *testing.T) {
	h, db := newHandler(t)
	codeA, err := auth.MintCode(db, "alice")
	if err != nil {
		t.Fatalf("mint A: %v", err)
	}
	codeB, err := auth.MintCode(db, "bob")
	if err != nil {
		t.Fatalf("mint B: %v", err)
	}

	// A binds Slampeach with 3 rows.
	if rec := post(t, h, codeA, invBody("Slampeach", 3)); rec.Code >= 300 {
		t.Fatalf("owner A upload status = %d", rec.Code)
	}

	// B attempts the same char name.
	rec := post(t, h, codeB, invBody("Slampeach", 1))
	if rec.Code != http.StatusConflict {
		t.Fatalf("cross-owner status = %d, want 409; body=%q", rec.Code, rec.Body.String())
	}
	// A's rows are untouched (the cross-owner tx rolled back).
	if got := countInv(t, db, "Slampeach"); got != 3 {
		t.Errorf("after cross-owner reject rows = %d, want 3 (A untouched)", got)
	}
	// And an audit row was written (cross_owner_reject — D-07 / V4).
	var audits int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE event = 'cross_owner_reject'`).Scan(&audits); err != nil {
		t.Fatalf("audit query: %v", err)
	}
	if audits != 1 {
		t.Errorf("audit_log cross_owner_reject rows = %d, want 1", audits)
	}
}

// TestIngest_BadKind_4xx: a well-formed envelope with an unknown kind is rejected
// (422) and writes nothing.
func TestIngest_BadKind_4xx(t *testing.T) {
	h, db := newHandler(t)
	code, err := auth.MintCode(db, "alice")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	body := `{"character":"Slampeach","kind":"badkind","content":"x","watcher_version":"2.0.0"}`
	rec := post(t, h, code, body)

	if rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("status = %d, want a 4xx", rec.Code)
	}
	if got := totalInv(t, db); got != 0 {
		t.Errorf("inventory_item count = %d, want 0", got)
	}
}

// TestIngest_MalformedJSON_4xx: a truncated JSON body is rejected (400) and
// writes nothing.
func TestIngest_MalformedJSON_4xx(t *testing.T) {
	h, db := newHandler(t)
	code, err := auth.MintCode(db, "alice")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	body := `{"character":"Slampeach","kind":"inventory",` // truncated
	rec := post(t, h, code, body)

	if rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("status = %d, want a 4xx", rec.Code)
	}
	if got := totalInv(t, db); got != 0 {
		t.Errorf("inventory_item count = %d, want 0", got)
	}
}

// TestIngest_OversizedBody_413or400: a >1 MB body is rejected via MaxBytesReader
// (the decode trips the cap → 4xx) and writes nothing (T-11.05-02).
func TestIngest_OversizedBody_413or400(t *testing.T) {
	h, db := newHandler(t)
	code, err := auth.MintCode(db, "alice")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	// Build a > 1 MiB content payload (valid JSON shape, but the body exceeds
	// the MaxBytesReader cap so the decode fails before any row is written).
	huge := strings.Repeat("A", (1<<20)+1024)
	body := `{"character":"Slampeach","kind":"inventory","watcher_version":"2.0.0","content":"` + huge + `"}`
	rec := post(t, h, code, body)

	if rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("status = %d, want a 4xx (oversized body rejected)", rec.Code)
	}
	if got := totalInv(t, db); got != 0 {
		t.Errorf("inventory_item count = %d, want 0 (oversized writes nothing)", got)
	}
}

// TestIngest_ValidSpellbook_ReplacesRows: a valid Bearer + spellbook POST stores
// the spell rows (proves the spellbook branch of the handler + the spellbook Tx
// replace are wired, not just inventory).
func TestIngest_ValidSpellbook_ReplacesRows(t *testing.T) {
	h, db := newHandler(t)
	code, err := auth.MintCode(db, "alice")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	body := `{"character":"Slampeach","kind":"spellbook","watcher_version":"2.0.0","content":"1\tMinor Healing\n4\tLight Healing\n"}`
	rec := post(t, h, code, body)

	if rec.Code != http.StatusNoContent && rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 2xx; body=%q", rec.Code, rec.Body.String())
	}
	var n int
	err = db.QueryRow(`
		SELECT COUNT(*) FROM spellbook_entry se
		JOIN character c ON c.id = se.character_id
		WHERE c.name = ?`, "Slampeach").Scan(&n)
	if err != nil {
		t.Fatalf("spellbook count: %v", err)
	}
	if n != 2 {
		t.Errorf("spellbook rows = %d, want 2", n)
	}
}

// TestIngest_EmptyContent_NoOp: an empty-content snapshot is a valid no-op that
// binds the character and leaves zero rows (mirrors the watcher's "empty file =
// clear" semantics; DecodeAndValidate allows empty content).
func TestIngest_EmptyContent_NoOp(t *testing.T) {
	h, db := newHandler(t)
	code, err := auth.MintCode(db, "alice")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	// First seed 3 rows, then send an empty snapshot — the rows must clear.
	if rec := post(t, h, code, invBody("Slampeach", 3)); rec.Code >= 300 {
		t.Fatalf("seed upload status = %d", rec.Code)
	}
	body := `{"character":"Slampeach","kind":"inventory","content":"","watcher_version":"2.0.0"}`
	rec := post(t, h, code, body)
	if rec.Code != http.StatusNoContent && rec.Code != http.StatusOK {
		t.Fatalf("empty-content status = %d, want 2xx", rec.Code)
	}
	if got := countInv(t, db, "Slampeach"); got != 0 {
		t.Errorf("after empty snapshot rows = %d, want 0", got)
	}
}

// TestIngest_TooOldVersion_426_WritesNothing: a valid Bearer + watcher_version
// below the floor ("1.9.9" < "2.0.0") is rejected 426 Upgrade Required with a
// clear human message, and the store is never touched (SC-5 / D-4). The 426 runs
// after decode and before any store call, so nothing is written.
func TestIngest_TooOldVersion_426_WritesNothing(t *testing.T) {
	h, db := newHandler(t)
	code, err := auth.MintCode(db, "alice")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	rec := post(t, h, code, invBodyVer("Slampeach", "1.9.9"))

	if rec.Code != http.StatusUpgradeRequired {
		t.Fatalf("status = %d, want 426 Upgrade Required; body=%q", rec.Code, rec.Body.String())
	}
	if body := strings.ToLower(rec.Body.String()); !strings.Contains(body, "too old") {
		t.Errorf("body = %q, want a clear 'too old' message", rec.Body.String())
	}
	if got := totalInv(t, db); got != 0 {
		t.Errorf("inventory_item count = %d, want 0 (426 must write nothing)", got)
	}
	// No character row should have been bound either.
	if got := countInv(t, db, "Slampeach"); got != 0 {
		t.Errorf("rows for Slampeach = %d, want 0", got)
	}
}

// TestIngest_FloorVersion_204: watcher_version exactly at the floor ("2.0.0") is
// NOT version-rejected and proceeds to a normal 2xx on a valid snapshot.
func TestIngest_FloorVersion_204(t *testing.T) {
	h, db := newHandler(t)
	code, err := auth.MintCode(db, "alice")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	rec := post(t, h, code, invBodyVer("Slampeach", "2.0.0"))

	if rec.Code != http.StatusNoContent && rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 2xx for floor version; body=%q", rec.Code, rec.Body.String())
	}
	if got := countInv(t, db, "Slampeach"); got != 1 {
		t.Errorf("inventory rows = %d, want 1 (floor version proceeds)", got)
	}
}

// TestIngest_NewerVersion_204: watcher_version above the floor ("2.1.0") proceeds.
func TestIngest_NewerVersion_204(t *testing.T) {
	h, db := newHandler(t)
	code, err := auth.MintCode(db, "alice")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	rec := post(t, h, code, invBodyVer("Slampeach", "2.1.0"))

	if rec.Code != http.StatusNoContent && rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 2xx for newer version; body=%q", rec.Code, rec.Body.String())
	}
	if got := countInv(t, db, "Slampeach"); got != 1 {
		t.Errorf("inventory rows = %d, want 1 (newer version proceeds)", got)
	}
}

// TestIngest_EmptyVersion_NotRejected_204: an EMPTY watcher_version is the ONE
// intentional exception to IsOlder's "empty present ⇒ older" rule — the gate
// guards `if env.WatcherVersion != "" && IsOlder(...)`, so a legacy/blank version
// is NOT actively rejected (forward-compat with the "accepted now" envelope
// contract) and proceeds through the normal ingest flow.
func TestIngest_EmptyVersion_NotRejected_204(t *testing.T) {
	h, db := newHandler(t)
	code, err := auth.MintCode(db, "alice")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	rec := post(t, h, code, invBodyVer("Slampeach", ""))

	if rec.Code == http.StatusUpgradeRequired {
		t.Fatalf("status = 426; an empty watcher_version must NOT be version-rejected")
	}
	if rec.Code != http.StatusNoContent && rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 2xx for empty version; body=%q", rec.Code, rec.Body.String())
	}
	if got := countInv(t, db, "Slampeach"); got != 1 {
		t.Errorf("inventory rows = %d, want 1 (empty version proceeds)", got)
	}
}

// TestIngest_TooOldVersion_NoAuth_401NotGate426: the version gate runs AFTER the
// bearer guard — a too-old version WITHOUT a token is still 401 (never 426), and
// writes nothing. This pins the load-bearing ordering (401-writes-nothing is
// structurally first).
func TestIngest_TooOldVersion_NoAuth_401NotGate426(t *testing.T) {
	h, db := newHandler(t)

	rec := post(t, h, "", invBodyVer("Slampeach", "1.9.9"))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (auth precedes the version gate)", rec.Code)
	}
	if got := totalInv(t, db); got != 0 {
		t.Errorf("inventory_item count = %d, want 0", got)
	}
}
