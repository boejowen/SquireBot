package ingest_test

// whoami_test.go proves GET /api/v1/whoami at the test tier (httptest + the
// 11-02 temp-DB fixture + the 11-04 mint), covering the onboarding-validation
// contract (CONTEXT D-4): a valid active code → 200 + owner label with ZERO
// side effects; a missing / unknown / revoked code → 401; a non-GET method →
// 405; and the V7 discipline that the raw Bearer token never appears in a log.
//
// It is an EXTERNAL test package (ingest_test) so it exercises the handler
// exactly as cmd/squirebot-server does: through the exported NewWhoami()
// constructor and the exported auth/store APIs — no package internals.

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/auth"
	"github.com/boejowen/SquireBot/internal/backendsrv/ingest"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// newWhoami wires a fresh migrated temp DB + bearer guard + whoami handler.
func newWhoami(t *testing.T) (*ingest.WhoamiHandler, *sql.DB) {
	t.Helper()
	db := store.NewTestDB(t) // Open + goose.Up + t.Cleanup (11-02 shared fixture)
	h := ingest.NewWhoami(auth.New(db), db)
	return h, db
}

// getWhoami builds a GET /api/v1/whoami request (optionally with a Bearer token)
// and serves it through the handler, returning the recorder.
func getWhoami(t *testing.T, h *ingest.WhoamiHandler, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/whoami", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// rowCounts snapshots the three mutable tables whoami must NEVER touch.
func rowCounts(t *testing.T, db *sql.DB) (owners, chars, items int) {
	t.Helper()
	q := func(sqlStr string) int {
		var n int
		if err := db.QueryRow(sqlStr).Scan(&n); err != nil {
			t.Fatalf("count (%s): %v", sqlStr, err)
		}
		return n
	}
	return q(`SELECT COUNT(*) FROM owner`),
		q(`SELECT COUNT(*) FROM character`),
		q(`SELECT COUNT(*) FROM inventory_item`)
}

// TestWhoami_ValidCode_200_WithLabel_NoSideEffects: a valid active Bearer code
// returns 200, a JSON body carrying the owner label, and leaves every mutable
// table's row count UNCHANGED (side-effect-free — D-4 / T-13.01-04).
func TestWhoami_ValidCode_200_WithLabel_NoSideEffects(t *testing.T) {
	h, db := newWhoami(t)
	code, err := auth.MintCode(db, "alice")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	// Snapshot AFTER mint (mint creates the owner row) — whoami must not change it.
	ownersBefore, charsBefore, itemsBefore := rowCounts(t, db)

	rec := getWhoami(t, h, code)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%q", rec.Code, rec.Body.String())
	}

	var body struct {
		OwnerID    int64  `json:"owner_id"`
		OwnerLabel string `json:"owner_label"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	if body.OwnerLabel != "alice" {
		t.Errorf("owner_label = %q, want %q", body.OwnerLabel, "alice")
	}
	if body.OwnerID == 0 {
		t.Errorf("owner_id = %d, want a real (non-zero) id", body.OwnerID)
	}

	ownersAfter, charsAfter, itemsAfter := rowCounts(t, db)
	if ownersAfter != ownersBefore || charsAfter != charsBefore || itemsAfter != itemsBefore {
		t.Errorf("whoami had side effects: owner %d->%d, character %d->%d, inventory_item %d->%d (want all unchanged)",
			ownersBefore, ownersAfter, charsBefore, charsAfter, itemsBefore, itemsAfter)
	}
}

// TestWhoami_NoAuthHeader_401: a GET with NO Authorization header returns 401
// and does not leak internals (the body is the static "unauthorized").
func TestWhoami_NoAuthHeader_401(t *testing.T) {
	h, _ := newWhoami(t)

	rec := getWhoami(t, h, "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "unauthorized" {
		t.Errorf("body = %q, want %q (no internals leaked)", got, "unauthorized")
	}
}

// TestWhoami_UnknownCode_401: a random (never-minted) bearer token returns 401.
func TestWhoami_UnknownCode_401(t *testing.T) {
	h, _ := newWhoami(t)

	rec := getWhoami(t, h, "totally-bogus-token-that-was-never-minted")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

// TestWhoami_RevokedCode_401: a minted-then-revoked code returns 401 (D-9 wired
// through whoami the same way it is through ingest).
func TestWhoami_RevokedCode_401(t *testing.T) {
	h, db := newWhoami(t)
	code, err := auth.MintCode(db, "alice")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	if err := auth.RevokeCode(db, "alice"); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	rec := getWhoami(t, h, code)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 for a revoked code", rec.Code)
	}
}

// TestWhoami_NonGetMethod_405: a non-GET method on the route is rejected 405
// (defensive — the ServeMux "GET " pattern already filters, but ServeHTTP guards
// too so a direct/mis-registered call is still safe).
func TestWhoami_NonGetMethod_405(t *testing.T) {
	h, db := newWhoami(t)
	code, err := auth.MintCode(db, "alice")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/whoami", nil)
	req.Header.Set("Authorization", "Bearer "+code)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405 for POST", rec.Code)
	}
}

// TestWhoami_NeverLogsToken (V7): the raw Bearer value must never appear in any
// slog record. We capture slog output to a buffer for one valid request and one
// rejected request and assert the token substring is absent from both.
func TestWhoami_NeverLogsToken(t *testing.T) {
	h, db := newWhoami(t)
	code, err := auth.MintCode(db, "alice")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	var buf strings.Builder
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	// Valid request (logs "whoami ok") + an invalid one (logs "whoami rejected").
	if rec := getWhoami(t, h, code); rec.Code != http.StatusOK {
		t.Fatalf("valid whoami status = %d, want 200", rec.Code)
	}
	if rec := getWhoami(t, h, "bad-"+code); rec.Code != http.StatusUnauthorized {
		t.Fatalf("invalid whoami status = %d, want 401", rec.Code)
	}

	if logs := buf.String(); strings.Contains(logs, code) {
		t.Errorf("slog output contains the raw bearer token (V7 violation):\n%s", logs)
	}
}
