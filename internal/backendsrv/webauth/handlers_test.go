package webauth

// handlers_test.go drives the login / callback / whoami-web / logout handlers
// (Task 3, D-01/D-03/D-05) against httptest + a migrated temp DB, with the
// Discord token + identity/guilds endpoints pointed at httptest fakes (the Task 1
// seams). It proves the load-bearing security behaviors:
//   - login 302s to Discord AND sets the short-lived sb_oauth_state cookie (CSRF);
//   - callback with a missing/mismatched state → 400 BEFORE any code exchange;
//   - callback for a NON-member → redirect to webOrigin+"/?not_member=1" with NO
//     session Set-Cookie (AUTH-08 refusal, no allowlist);
//   - callback for a member → a fresh session Set-Cookie + a web_session row + a
//     redirect to webOrigin+"/";
//   - W-4 open-redirect: a member callback carrying &redirect=https://evil.example
//     STILL 302s to webOrigin+"/" (the handler ignores caller-supplied redirect);
//   - whoami-web returns the right shape for an authed vs an anonymous request;
//   - logout deletes the session + clears the cookie.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

const testGuildID = "111111111111111111"
const testWebOrigin = "https://squirebot.quest"

// fakeDiscord stands up the token + identity + guilds endpoints. memberGuilds is
// the guild list /users/@me/guilds returns (controls the membership branch).
func fakeDiscord(t *testing.T, userID, username, avatar string, memberGuilds []string) (cfg Config, restore func()) {
	t.Helper()
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"fake-token","token_type":"Bearer","expires_in":604800}`))
	}))
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/users/@me":
			_ = json.NewEncoder(w).Encode(map[string]string{"id": userID, "username": username, "avatar": avatar})
		case "/users/@me/guilds":
			arr := make([]map[string]string, 0, len(memberGuilds))
			for _, g := range memberGuilds {
				arr = append(arr, map[string]string{"id": g})
			}
			_ = json.NewEncoder(w).Encode(arr)
		default:
			http.NotFound(w, r)
		}
	}))
	restoreEnds := setOAuthEndpointsForTest("https://discord.com/oauth2/authorize", tokenSrv.URL)
	restoreBase := setDiscordAPIBaseForTest(apiSrv.URL)
	restoreOrigin := setWebOriginForTest(testWebOrigin)
	cfg = Config{
		ClientID:     "cid",
		ClientSecret: "csecret-not-real",
		RedirectURI:  "https://api.squirebot.quest/api/v1/auth/callback",
		GuildID:      testGuildID,
	}
	return cfg, func() {
		restoreEnds()
		restoreBase()
		restoreOrigin()
		tokenSrv.Close()
		apiSrv.Close()
	}
}

func TestLoginHandler_RedirectsAndSetsStateCookie(t *testing.T) {
	db := store.NewTestDB(t)
	cfg, restore := fakeDiscord(t, "u1", "User", "", []string{testGuildID})
	defer restore()

	rec := httptest.NewRecorder()
	LoginHandler(db, cfg).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/auth/login", nil))

	if rec.Code != http.StatusFound {
		t.Fatalf("login status = %d, want 302", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "https://discord.com/oauth2/authorize") {
		t.Fatalf("login Location = %q, want a Discord authorize URL", loc)
	}
	// The state cookie must be set (CSRF) and its value must equal the state in
	// the redirect URL.
	var stateCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == oauthStateCookieName {
			stateCookie = c
		}
	}
	if stateCookie == nil || stateCookie.Value == "" {
		t.Fatalf("login did not set a non-empty %s cookie", oauthStateCookieName)
	}
	if !stateCookie.HttpOnly {
		t.Error("state cookie must be HttpOnly")
	}
	u, _ := url.Parse(loc)
	if u.Query().Get("state") != stateCookie.Value {
		t.Errorf("authorize state %q != state cookie %q", u.Query().Get("state"), stateCookie.Value)
	}
}

func TestCallbackHandler_MismatchedState_400(t *testing.T) {
	db := store.NewTestDB(t)
	cfg, restore := fakeDiscord(t, "u1", "User", "", []string{testGuildID})
	defer restore()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/callback?code=abc&state=QUERY_STATE", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "DIFFERENT_COOKIE_STATE"})
	rec := httptest.NewRecorder()
	CallbackHandler(db, cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("mismatched-state status = %d, want 400", rec.Code)
	}
	// No session cookie should be set on a CSRF rejection.
	for _, c := range rec.Result().Cookies() {
		if c.Name == SessionCookieName && c.Value != "" {
			t.Errorf("a session cookie was set on a CSRF rejection")
		}
	}
}

func TestCallbackHandler_MissingState_400(t *testing.T) {
	db := store.NewTestDB(t)
	cfg, restore := fakeDiscord(t, "u1", "User", "", []string{testGuildID})
	defer restore()

	// No state query param, no state cookie → 400.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/callback?code=abc", nil)
	rec := httptest.NewRecorder()
	CallbackHandler(db, cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing-state status = %d, want 400", rec.Code)
	}
}

func TestCallbackHandler_NonMember_NoSession_RedirectNotMember(t *testing.T) {
	db := store.NewTestDB(t)
	// The user is in some OTHER guild, not testGuildID → not a member.
	cfg, restore := fakeDiscord(t, "stranger", "Stranger", "", []string{"999000999000"})
	defer restore()

	const state = "matched-state"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/callback?code=abc&state="+state, nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: state})
	rec := httptest.NewRecorder()
	CallbackHandler(db, cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("non-member status = %d, want 302", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "not_member") {
		t.Errorf("non-member Location = %q, want it to contain not_member", loc)
	}
	if !strings.HasPrefix(loc, testWebOrigin) {
		t.Errorf("non-member Location = %q, want it to start with %q", loc, testWebOrigin)
	}
	// NO session minted for a non-member (AUTH-08).
	for _, c := range rec.Result().Cookies() {
		if c.Name == SessionCookieName && c.Value != "" {
			t.Errorf("a session cookie was set for a non-member (must be refused)")
		}
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM web_session`).Scan(&n); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if n != 0 {
		t.Errorf("web_session rows = %d, want 0 for a non-member", n)
	}
}

func TestCallbackHandler_Member_MintsSession_RedirectHome(t *testing.T) {
	db := store.NewTestDB(t)
	cfg, restore := fakeDiscord(t, "888888888888888888", "Memberer", "av1", []string{testGuildID, "extra"})
	defer restore()

	const state = "matched-state"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/callback?code=abc&state="+state, nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: state})
	rec := httptest.NewRecorder()
	CallbackHandler(db, cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("member status = %d, want 302", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != testWebOrigin+"/" {
		t.Fatalf("member Location = %q, want %q", loc, testWebOrigin+"/")
	}
	// A fresh session cookie must be set.
	var sessCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == SessionCookieName {
			sessCookie = c
		}
	}
	if sessCookie == nil || sessCookie.Value == "" {
		t.Fatalf("member callback did not set a session cookie")
	}
	// The web_session row exists for that opaque id (stored hashed).
	var uid string
	if err := db.QueryRow(`SELECT discord_user_id FROM web_session WHERE session_hash = ?`,
		store.HashSession(sessCookie.Value)).Scan(&uid); err != nil {
		t.Fatalf("session row lookup: %v", err)
	}
	if uid != "888888888888888888" {
		t.Errorf("session discord_user_id = %q, want 888888888888888888", uid)
	}
	// AUTH-09: the web_user identity was captured.
	var username string
	if err := db.QueryRow(`SELECT username FROM web_user WHERE discord_user_id = ?`,
		"888888888888888888").Scan(&username); err != nil {
		t.Fatalf("web_user lookup: %v", err)
	}
	if username != "Memberer" {
		t.Errorf("captured username = %q, want Memberer", username)
	}
}

// W-4: the open-redirect regression. A member callback that carries a
// caller-supplied &redirect=https://evil.example MUST still land on
// webOrigin+"/" — the handler must never honor a request-supplied redirect.
func TestCallbackHandler_IgnoresCallerRedirect_W4(t *testing.T) {
	db := store.NewTestDB(t)
	cfg, restore := fakeDiscord(t, "777", "Victim", "", []string{testGuildID})
	defer restore()

	const state = "matched-state"
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/auth/callback?code=abc&state="+state+"&redirect=https://evil.example&return_to=https://evil.example&next=https://evil.example", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: state})
	rec := httptest.NewRecorder()
	CallbackHandler(db, cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if loc != testWebOrigin+"/" {
		t.Fatalf("open-redirect: Location = %q, want exactly %q (caller redirect must be ignored)", loc, testWebOrigin+"/")
	}
	if strings.Contains(loc, "evil.example") {
		t.Fatalf("open-redirect: Location leaked the attacker host: %q", loc)
	}
}

func TestWhoamiWebHandler_AuthedShape(t *testing.T) {
	db := store.NewTestDB(t)
	cfg := Config{GuildID: testGuildID}
	ctx := context.Background()
	now := time.Now().Unix()

	const uid = "555555555555555555"
	// Make uid an officer (owner-floor seeds web_user + guild_admins).
	if err := store.SetOwnerFloor(ctx, db, uid, now); err != nil {
		t.Fatalf("set owner floor: %v", err)
	}
	// Refresh the placeholder username/avatar with a real login capture.
	if err := store.UpsertWebUser(ctx, db, uid, "TheOfficer", "avhash", now); err != nil {
		t.Fatalf("upsert web_user: %v", err)
	}
	sid, _ := store.GenerateSessionID()
	if err := store.CreateSession(ctx, db, uid, sid, now, store.SessionTTLSeconds); err != nil {
		t.Fatalf("create session: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/whoami-web", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sid})
	rec := httptest.NewRecorder()
	WhoamiWebHandler(db, cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("whoami status = %d, want 200", rec.Code)
	}
	var body struct {
		Authenticated bool   `json:"authenticated"`
		IsMember      bool   `json:"isMember"`
		IsOfficer     bool   `json:"isOfficer"`
		Username      string `json:"username"`
		Avatar        string `json:"avatar"`
		DiscordUserID string `json:"discord_user_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode whoami body: %v (body=%s)", err, rec.Body.String())
	}
	if !body.Authenticated || !body.IsMember || !body.IsOfficer {
		t.Errorf("authed/member/officer = %v/%v/%v, want all true", body.Authenticated, body.IsMember, body.IsOfficer)
	}
	if body.Username != "TheOfficer" || body.Avatar != "avhash" || body.DiscordUserID != uid {
		t.Errorf("identity = (%q,%q,%q), want (TheOfficer,avhash,%s)", body.Username, body.Avatar, body.DiscordUserID, uid)
	}
}

func TestWhoamiWebHandler_AnonShape(t *testing.T) {
	db := store.NewTestDB(t)
	cfg := Config{GuildID: testGuildID}

	rec := httptest.NewRecorder()
	WhoamiWebHandler(db, cfg).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/auth/whoami-web", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("anon whoami status = %d, want 200 (this endpoint is always 200)", rec.Code)
	}
	var body struct {
		Authenticated bool `json:"authenticated"`
		IsMember      bool `json:"isMember"`
		IsOfficer     bool `json:"isOfficer"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode anon whoami: %v (body=%s)", err, rec.Body.String())
	}
	if body.Authenticated || body.IsMember || body.IsOfficer {
		t.Errorf("anon authed/member/officer = %v/%v/%v, want all false", body.Authenticated, body.IsMember, body.IsOfficer)
	}
}

func TestLogoutHandler_DeletesSessionAndClearsCookie(t *testing.T) {
	db := store.NewTestDB(t)
	cfg := Config{GuildID: testGuildID}
	ctx := context.Background()
	now := time.Now().Unix()

	const uid = "666666666666666666"
	if err := store.UpsertWebUser(ctx, db, uid, "ByeUser", "", now); err != nil {
		t.Fatalf("upsert web_user: %v", err)
	}
	sid, _ := store.GenerateSessionID()
	if err := store.CreateSession(ctx, db, uid, sid, now, store.SessionTTLSeconds); err != nil {
		t.Fatalf("create session: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sid})
	rec := httptest.NewRecorder()
	LogoutHandler(db, cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want 204", rec.Code)
	}
	// The session row is gone.
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM web_session WHERE session_hash = ?`,
		store.HashSession(sid)).Scan(&n); err != nil {
		t.Fatalf("count session: %v", err)
	}
	if n != 0 {
		t.Errorf("web_session rows after logout = %d, want 0", n)
	}
	// The cookie is cleared (MaxAge < 0).
	var cleared bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == SessionCookieName && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Error("logout did not emit a cleared session cookie (MaxAge<0)")
	}
}
