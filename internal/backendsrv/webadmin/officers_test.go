package webadmin

// officers_test.go — Task 1 (TDD). Proves the officer-management endpoints port
// v1's admin.ts enforcement over HTTP: officer-only (RequireOfficer at the route,
// re-authorized inside the tx), idempotent add/remove, owner-floor protection
// (a peer cannot remove the floor → 403 owner_floor_protected), and an
// append-only audit_log row per write. Behavioral oracle: apps-script/src/lib/admin.ts.
//
// The handlers expect webauth.UserFromContext(ctx) to carry the acting
// discord_user_id (the route wraps them in RequireOfficer, which sets it). These
// unit tests inject that identity directly via withCaller so the handler logic is
// exercised without standing up the full session machinery (the route-level gate
// is covered by main_test.go in Task 3).

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
	"github.com/boejowen/SquireBot/internal/backendsrv/webauth"
)

// withCaller wraps next so the request context carries discordUserID exactly as
// webauth.RequireOfficer/RequireSession would have placed it — the seam the
// handlers read via webauth.UserFromContext. This lets the handler tests inject
// the acting identity without a real session cookie.
func withCaller(discordUserID string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := webauth.WithUser(r.Context(), discordUserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// seedFloorAndUsers seeds the owner-floor (also the bootstrap officer) plus a set
// of plain web_users (promotable). Returns nothing — callers know the ids.
func seedFloorAndUsers(t *testing.T, ctx context.Context, db *sql.DB, floor string, users map[string]string) {
	t.Helper()
	if err := store.SetOwnerFloor(ctx, db, floor, 1700000000); err != nil {
		t.Fatalf("SetOwnerFloor: %v", err)
	}
	for id, name := range users {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO web_user (discord_user_id, username, avatar, first_seen, last_login)
			 VALUES (?, ?, NULL, 0, 0)`, id, name); err != nil {
			t.Fatalf("insert web_user %q: %v", id, err)
		}
	}
}

// guildAdminCount reads the guild_admins row count (the "nothing was written"
// assertion for a rejected non-officer write).
func guildAdminCount(t *testing.T, ctx context.Context, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM guild_admins`).Scan(&n); err != nil {
		t.Fatalf("count guild_admins: %v", err)
	}
	return n
}

// auditCount reads the audit_log row count for a given event (proves a write was
// audited, append-only).
func auditCount(t *testing.T, ctx context.Context, db *sql.DB, event string) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_log WHERE event = ?`, event).Scan(&n); err != nil {
		t.Fatalf("count audit_log(%s): %v", event, err)
	}
	return n
}

func postJSON(t *testing.T, handler http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestOfficerAdd_NonOfficerRejected_NothingWritten(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	floor := "111111111111111111"
	target := "222222222222222222"
	stranger := "333333333333333333"
	seedFloorAndUsers(t, ctx, db, floor, map[string]string{target: "TargetToon", stranger: "Stranger"})

	before := guildAdminCount(t, ctx, db)

	// Stranger (not an officer) tries to promote target.
	h := withCaller(stranger, OfficerAddHandler(db))
	rec := postJSON(t, h, `{"discord_user_id":"`+target+`"}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if got := decodeErr(t, rec); got != "not_authorized" {
		t.Errorf("error = %q, want not_authorized", got)
	}
	// Authorize-under-tx rejected → the guild_admins row count is UNCHANGED.
	if after := guildAdminCount(t, ctx, db); after != before {
		t.Errorf("guild_admins count changed (%d → %d) despite unauthorized caller", before, after)
	}
	if c := auditCount(t, ctx, db, "officer_add"); c != 0 {
		t.Errorf("officer_add audit rows = %d, want 0 (nothing written)", c)
	}
}

func TestOfficerAdd_FloorPromotes_Idempotent_Audits(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	floor := "111111111111111111"
	target := "222222222222222222"
	seedFloorAndUsers(t, ctx, db, floor, map[string]string{target: "TargetToon"})

	h := withCaller(floor, OfficerAddHandler(db))

	// First promotion → added=true.
	rec := postJSON(t, h, `{"discord_user_id":"`+target+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		Added    bool   `json:"added"`
		Username string `json:"username"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode add resp: %v", err)
	}
	if !resp.Added {
		t.Errorf("added = false, want true (fresh promotion)")
	}
	if ok, _ := store.IsOfficer(ctx, db, target); !ok {
		t.Errorf("target is not an officer after promotion")
	}
	if c := auditCount(t, ctx, db, "officer_add"); c != 1 {
		t.Errorf("officer_add audit rows = %d, want 1", c)
	}

	// Idempotent re-add → added=false (still 200, no second audit duplication concern
	// is required, but the row count must not flip the officer state).
	rec2 := postJSON(t, h, `{"discord_user_id":"`+target+`"}`)
	if rec2.Code != http.StatusOK {
		t.Fatalf("idempotent status = %d, want 200", rec2.Code)
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode 2nd add resp: %v", err)
	}
	if resp.Added {
		t.Errorf("added = true on re-add, want false (idempotent)")
	}
}

func TestOfficerRemove_PeerCannotRemoveFloor(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	floor := "111111111111111111"
	peer := "222222222222222222"
	seedFloorAndUsers(t, ctx, db, floor, map[string]string{peer: "PeerOfficer"})

	// Promote peer first (floor does it).
	addH := withCaller(floor, OfficerAddHandler(db))
	if rec := postJSON(t, addH, `{"discord_user_id":"`+peer+`"}`); rec.Code != http.StatusOK {
		t.Fatalf("promote peer status = %d, want 200", rec.Code)
	}

	// Peer tries to remove the floor → 403 owner_floor_protected, floor still officer.
	rmH := withCaller(peer, OfficerRemoveHandler(db))
	rec := postJSON(t, rmH, `{"discord_user_id":"`+floor+`"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if got := decodeErr(t, rec); got != "owner_floor_protected" {
		t.Errorf("error = %q, want owner_floor_protected", got)
	}
	if ok, _ := store.IsOfficer(ctx, db, floor); !ok {
		t.Errorf("floor was removed despite owner-floor protection")
	}
}

func TestOfficerRemove_FloorRemovesPeer_Audits(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	floor := "111111111111111111"
	peer := "222222222222222222"
	seedFloorAndUsers(t, ctx, db, floor, map[string]string{peer: "PeerOfficer"})

	addH := withCaller(floor, OfficerAddHandler(db))
	if rec := postJSON(t, addH, `{"discord_user_id":"`+peer+`"}`); rec.Code != http.StatusOK {
		t.Fatalf("promote peer status = %d, want 200", rec.Code)
	}

	rmH := withCaller(floor, OfficerRemoveHandler(db))
	rec := postJSON(t, rmH, `{"discord_user_id":"`+peer+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		Removed bool `json:"removed"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode remove resp: %v", err)
	}
	if !resp.Removed {
		t.Errorf("removed = false, want true")
	}
	if ok, _ := store.IsOfficer(ctx, db, peer); ok {
		t.Errorf("peer still an officer after removal")
	}
	if c := auditCount(t, ctx, db, "officer_remove"); c != 1 {
		t.Errorf("officer_remove audit rows = %d, want 1", c)
	}
}

func TestOfficersList_ReturnsOfficersAndPromotable(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	floor := "111111111111111111"
	candidate := "222222222222222222"
	seedFloorAndUsers(t, ctx, db, floor, map[string]string{candidate: "Candidate"})

	h := withCaller(floor, OfficersListHandler(db))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp struct {
		Officers   []store.Officer `json:"officers"`
		Promotable []store.Officer `json:"promotable"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode list resp: %v", err)
	}
	if len(resp.Officers) != 1 || resp.Officers[0].DiscordUserID != floor || !resp.Officers[0].IsFloor {
		t.Errorf("officers = %+v, want the floor with IsFloor=true", resp.Officers)
	}
	var sawCandidate bool
	for _, p := range resp.Promotable {
		if p.DiscordUserID == candidate {
			sawCandidate = true
		}
	}
	if !sawCandidate {
		t.Errorf("promotable = %+v, want the candidate listed", resp.Promotable)
	}
}

// decodeErr extracts the {"error":"code"} field from a JSON error response.
func decodeErr(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body %q: %v", rec.Body.String(), err)
	}
	return body.Error
}
