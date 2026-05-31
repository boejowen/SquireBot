package webauth

// session_test.go drives the session-cookie helpers + the RequireSession /
// RequireOfficer middleware (Task 2, D-05) against httptest + a migrated temp DB
// from the 15-01 store. It proves:
//   - SetSessionCookie emits HttpOnly + Secure + SameSite=Lax + Domain + a 30-day
//     MaxAge (the cross-subdomain, XSS-resistant cookie — T-15-07/T-15-10);
//   - ClearSessionCookie emits MaxAge<0 (delete);
//   - RequireSession is fail-closed: no cookie / invalid cookie → 401
//     {"error":"unauthorized"}; a valid session → next runs with the
//     discord_user_id in context AND the expiry is rolled (TouchSession);
//   - RequireOfficer: a non-officer session → 403 {"error":"not_authorized"};
//     an officer session → next runs.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

func TestSetSessionCookie_Attributes(t *testing.T) {
	rec := httptest.NewRecorder()
	SetSessionCookie(rec, "opaque-session-id", CookieOpts{Domain: "squirebot.quest", Secure: true})

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("Set-Cookie count = %d, want 1", len(cookies))
	}
	c := cookies[0]
	if c.Name != SessionCookieName {
		t.Errorf("cookie name = %q, want %q", c.Name, SessionCookieName)
	}
	if c.Value != "opaque-session-id" {
		t.Errorf("cookie value = %q, want opaque-session-id", c.Value)
	}
	if !c.HttpOnly {
		t.Error("cookie HttpOnly = false, want true (XSS-resistant — T-15-07)")
	}
	if !c.Secure {
		t.Error("cookie Secure = false, want true (HTTPS only)")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie SameSite = %v, want Lax (cross-subdomain + OAuth return)", c.SameSite)
	}
	if c.Domain != "squirebot.quest" {
		t.Errorf("cookie Domain = %q, want squirebot.quest (registrable domain → rides api.)", c.Domain)
	}
	if c.Path != "/" {
		t.Errorf("cookie Path = %q, want /", c.Path)
	}
	if c.MaxAge != int(store.SessionTTLSeconds) {
		t.Errorf("cookie MaxAge = %d, want %d (30 days)", c.MaxAge, store.SessionTTLSeconds)
	}
}

func TestClearSessionCookie_Deletes(t *testing.T) {
	rec := httptest.NewRecorder()
	ClearSessionCookie(rec, CookieOpts{Domain: "squirebot.quest", Secure: true})
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("Set-Cookie count = %d, want 1", len(cookies))
	}
	if cookies[0].MaxAge >= 0 {
		t.Errorf("clear cookie MaxAge = %d, want < 0 (delete)", cookies[0].MaxAge)
	}
}

func TestRequireSession_NoCookie_401(t *testing.T) {
	db := store.NewTestDB(t)
	called := false
	h := RequireSession(db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/views/view", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no-cookie status = %d, want 401", rec.Code)
	}
	if called {
		t.Error("next handler ran despite no session (must be fail-closed)")
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v (body=%s)", err, rec.Body.String())
	}
	if body["error"] != "unauthorized" {
		t.Errorf("error code = %q, want unauthorized", body["error"])
	}
}

func TestRequireSession_InvalidCookie_401(t *testing.T) {
	db := store.NewTestDB(t)
	h := RequireSession(db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next ran for an invalid session cookie")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/views/view", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "not-a-real-session"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("invalid-cookie status = %d, want 401", rec.Code)
	}
}

func TestRequireSession_ValidCookie_PassesAndRolls(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	now := time.Now().Unix()

	const uid = "222222222222222222"
	if err := store.UpsertWebUser(ctx, db, uid, "Slampeach", "abc", now); err != nil {
		t.Fatalf("upsert web_user: %v", err)
	}
	sid, err := store.GenerateSessionID()
	if err != nil {
		t.Fatalf("gen session id: %v", err)
	}
	// Seed with a SHORT ttl so we can observe the rolling bump to ~30 days.
	if err := store.CreateSession(ctx, db, uid, sid, now, 60); err != nil {
		t.Fatalf("create session: %v", err)
	}

	var gotUser string
	var sawCtx bool
	h := RequireSession(db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, sawCtx = UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/views/view", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sid})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("valid-cookie status = %d, want 200", rec.Code)
	}
	if !sawCtx || gotUser != uid {
		t.Fatalf("UserFromContext = (%q,%v), want (%q,true)", gotUser, sawCtx, uid)
	}

	// Rolling expiry: the seed expiry was now+60; after a successful gate it must
	// be bumped toward now+SessionTTLSeconds (well beyond 60s out).
	var expiresAt int64
	if err := db.QueryRow(`SELECT expires_at FROM web_session WHERE session_hash = ?`,
		store.HashSession(sid)).Scan(&expiresAt); err != nil {
		t.Fatalf("read expires_at: %v", err)
	}
	if expiresAt < now+store.SessionTTLSeconds-300 {
		t.Errorf("expires_at = %d, want rolled to ~now+%d (rolling TTL not applied)", expiresAt, store.SessionTTLSeconds)
	}
}

func TestRequireOfficer_NonOfficer_403(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	now := time.Now().Unix()

	const uid = "333333333333333333"
	if err := store.UpsertWebUser(ctx, db, uid, "PlainMember", "", now); err != nil {
		t.Fatalf("upsert web_user: %v", err)
	}
	sid, _ := store.GenerateSessionID()
	if err := store.CreateSession(ctx, db, uid, sid, now, store.SessionTTLSeconds); err != nil {
		t.Fatalf("create session: %v", err)
	}

	h := RequireOfficer(db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next ran for a non-officer session (must be 403)")
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admins", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sid})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-officer status = %d, want 403", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v (body=%s)", err, rec.Body.String())
	}
	if body["error"] != "not_authorized" {
		t.Errorf("error code = %q, want not_authorized", body["error"])
	}
}

func TestRequireOfficer_Officer_Passes(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	now := time.Now().Unix()

	const uid = "444444444444444444"
	// SetOwnerFloor makes uid the bootstrap officer (seeds web_user + guild_admins).
	if err := store.SetOwnerFloor(ctx, db, uid, now); err != nil {
		t.Fatalf("set owner floor: %v", err)
	}
	sid, _ := store.GenerateSessionID()
	if err := store.CreateSession(ctx, db, uid, sid, now, store.SessionTTLSeconds); err != nil {
		t.Fatalf("create session: %v", err)
	}

	called := false
	h := RequireOfficer(db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admins", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sid})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("officer status = %d, want 200", rec.Code)
	}
	if !called {
		t.Error("next did not run for an officer session")
	}
}
