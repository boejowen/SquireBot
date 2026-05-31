package webauth

// handlers.go holds the four human-facing OAuth/session HTTP handlers (Task 3,
// D-01/D-03/D-05), composing the Task 1 OAuth helpers + the Task 2 session
// layer + the 15-01 store. Each is a func(db, cfg) http.HandlerFunc mirroring
// the ingest handler convention (method-check first; JSON {"error":"code"}).
//
//	GET  /api/v1/auth/login       — mint state, set sb_oauth_state cookie, 302 → Discord
//	GET  /api/v1/auth/callback    — verify state (CSRF), exchange, gate membership, mint session
//	GET  /api/v1/auth/whoami-web  — the AuthGate feed {authenticated,isMember,isOfficer,...}
//	POST /api/v1/auth/logout      — delete the session + clear the cookie
//
// W-4 OPEN-REDIRECT RULE (15-RESEARCH §1, T-15-13): the post-callback browser
// redirect Location is derived ONLY from the webOrigin server constant
// (hardcoded https://squirebot.quest, env-overridable for staging). The callback
// MUST NEVER read a request-supplied redirect/return_to/next query param and
// MUST NEVER use one to build the Location — so an attacker cannot bounce a
// victim through our trusted callback to an arbitrary host. (There is an
// acceptance grep-gate enforcing the absence of any such param read.)
//
// REGENERATE-ON-LOGIN (T-15-07): every successful callback mints a BRAND-NEW
// opaque session id (store.GenerateSessionID) — there is no session fixation
// because we never adopt a caller-presented id.

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// oauthStateCookieName is the short-lived httpOnly cookie that carries the OAuth
// CSRF state between /auth/login and /auth/callback (T-15-06). Distinct from the
// session cookie.
const oauthStateCookieName = "sb_oauth_state"

// oauthStateTTLSeconds bounds the login→callback window (5 minutes): a user has
// 5 minutes to complete the Discord consent screen before the state cookie
// lapses and they must restart the login.
const oauthStateTTLSeconds = 5 * 60

// defaultWebOrigin is the HARDCODED site origin the callback redirects back to
// (W-4). The deploy sets SQUIREBOT_WEB_ORIGIN=https://squirebot.quest explicitly,
// but the default IS the production origin so a missing env var is still safe
// (never an attacker-controlled value).
const defaultWebOrigin = "https://squirebot.quest"

// webOrigin is the resolved site origin. A package var ONLY so handlers_test.go
// can pin it to a known value via setWebOriginForTest; production resolves it
// once from the env (or the hardcoded default) at first use.
var webOrigin = resolveWebOrigin()

// resolveWebOrigin reads SQUIREBOT_WEB_ORIGIN, falling back to the hardcoded
// production origin. The value is a SERVER constant — it is NEVER built from any
// request input (W-4).
func resolveWebOrigin() string {
	if v := os.Getenv("SQUIREBOT_WEB_ORIGIN"); v != "" {
		return v
	}
	return defaultWebOrigin
}

// setWebOriginForTest pins webOrigin to a known value for the handler tests and
// returns a restore func. TEST-ONLY.
func setWebOriginForTest(origin string) func() {
	prev := webOrigin
	webOrigin = origin
	return func() { webOrigin = prev }
}

// LoginHandler serves GET /api/v1/auth/login: mint a CSRF state, set it in a
// short-lived httpOnly+Secure+SameSite=Lax state cookie, and 302-redirect to the
// Discord authorize URL (scope identify+guilds). SameSite=Lax permits the
// top-level GET return from Discord's redirect to carry the state cookie back.
func LoginHandler(db *sql.DB, cfg Config) http.HandlerFunc {
	opts := CookieOptsFromEnv()
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		state, err := GenerateState()
		if err != nil {
			slog.Error("login: generate state failed", "err", err)
			http.Error(w, "internal", http.StatusInternalServerError)
			return
		}
		// Short-lived state cookie (CSRF). Same security attributes as the session
		// cookie but a 5-minute MaxAge and its own name.
		http.SetCookie(w, &http.Cookie{
			Name:     oauthStateCookieName,
			Value:    state,
			Path:     "/",
			Domain:   opts.Domain,
			HttpOnly: true,
			Secure:   opts.Secure,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   oauthStateTTLSeconds,
		})
		http.Redirect(w, r, AuthCodeURL(cfg, state), http.StatusFound)
	}
}

// CallbackHandler serves GET /api/v1/auth/callback: the OAuth return. It verifies
// the state (CSRF) against the state cookie, exchanges the code server-side,
// fetches identity + guilds, and gates on membership. A non-member is refused
// (no session, redirect to the NotMember screen); a member gets a fresh session.
//
// Both redirect targets are built ONLY from the webOrigin server constant — the
// handler reads ONLY `state` and `code` from the query, NEVER a caller-supplied
// redirect target (W-4 / T-15-13).
func CallbackHandler(db *sql.DB, cfg Config) http.HandlerFunc {
	opts := CookieOptsFromEnv()
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// CSRF: the state query param must be present AND equal the state cookie
		// (which login set). A missing/empty/mismatched state → 400 BEFORE any code
		// exchange (T-15-06). We read ONLY `state` (and `code` below) from the URL.
		queryState := r.URL.Query().Get("state")
		stateCookie, cerr := r.Cookie(oauthStateCookieName)
		if queryState == "" || cerr != nil || stateCookie.Value == "" || stateCookie.Value != queryState {
			// Clear any stale state cookie and reject.
			clearOAuthStateCookie(w, opts)
			http.Error(w, "bad state", http.StatusBadRequest)
			return
		}
		// State consumed — clear the cookie regardless of the outcome below.
		clearOAuthStateCookie(w, opts)

		tok, err := Exchange(r.Context(), cfg, r.URL.Query().Get("code"))
		if err != nil {
			slog.Warn("callback: code exchange failed", "err", err)
			http.Error(w, "exchange failed", http.StatusBadGateway)
			return
		}
		id, username, avatar, err := FetchIdentity(r.Context(), cfg, tok)
		if err != nil {
			slog.Warn("callback: fetch identity failed", "err", err)
			http.Error(w, "discord error", http.StatusBadGateway)
			return
		}
		guilds, err := FetchGuilds(r.Context(), cfg, tok)
		if err != nil {
			slog.Warn("callback: fetch guilds failed", "err", err)
			http.Error(w, "discord error", http.StatusBadGateway)
			return
		}

		now := time.Now().Unix()
		if !IsGuildMember(guilds, cfg.GuildID) {
			// AUTH-08 refusal: NO session minted. Bounce to the NotMember screen.
			// The Location is the webOrigin constant + a fixed marker query — never
			// a request-supplied target (W-4).
			slog.Info("callback: membership refused", "discord_user_id", id)
			http.Redirect(w, r, webOrigin+"/?not_member=1", http.StatusFound)
			return
		}

		// Member: capture identity (AUTH-09), mint a FRESH session id
		// (regenerate-on-login), persist it (hash-only), set the cookie.
		if err := store.UpsertWebUser(r.Context(), db, id, username, avatar, now); err != nil {
			slog.Error("callback: upsert web_user failed", "err", err)
			http.Error(w, "internal", http.StatusInternalServerError)
			return
		}
		sid, err := store.GenerateSessionID()
		if err != nil {
			slog.Error("callback: generate session id failed", "err", err)
			http.Error(w, "internal", http.StatusInternalServerError)
			return
		}
		if err := store.CreateSession(r.Context(), db, id, sid, now, store.SessionTTLSeconds); err != nil {
			slog.Error("callback: create session failed", "err", err)
			http.Error(w, "internal", http.StatusInternalServerError)
			return
		}
		SetSessionCookie(w, sid, opts)
		// Success → the hardcoded site home (W-4: webOrigin constant only).
		http.Redirect(w, r, webOrigin+"/", http.StatusFound)
	}
}

// WhoamiWebHandler serves GET /api/v1/auth/whoami-web: the AuthGate feed. It is
// the ONE read endpoint that returns 200 even when unauthenticated (so the
// frontend can branch). Shape (UI-SPEC): {authenticated,isMember,isOfficer,
// username,avatar,discord_user_id}. A session is minted ONLY for a member, so a
// valid session ⇒ isMember:true; the anonymous default is all-false.
func WhoamiWebHandler(db *sql.DB, cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// Fail-closed defaults.
		out := map[string]any{
			"authenticated":   false,
			"isMember":        false,
			"isOfficer":       false,
			"username":        "",
			"avatar":          "",
			"discord_user_id": "",
		}
		// WR-06: the READ-ONLY resolve (no TouchSession) — whoami-web is the
		// documented side-effect-free AuthGate feed the frontend hits on every
		// mount/refresh, so it must not roll the session's expiry. The rolling-window
		// bump happens only on the gated API hits (RequireSession/RequireOfficer).
		uid, ok := resolveSessionUserReadOnly(r, db)
		if !ok {
			_ = json.NewEncoder(w).Encode(out)
			return
		}
		username, avatar, err := store.GetWebUser(r.Context(), db, uid)
		if err != nil {
			// A valid session whose web_user vanished is anomalous; degrade to the
			// authed-but-minimal shape rather than 500 (the session is real).
			slog.Warn("whoami-web: get web_user failed", "err", err)
		}
		isOfficer, oerr := store.IsOfficer(r.Context(), db, uid)
		if oerr != nil {
			slog.Warn("whoami-web: officer check failed", "err", oerr)
		}
		out["authenticated"] = true
		out["isMember"] = true // a session is only ever minted for a member
		out["isOfficer"] = isOfficer
		out["username"] = username
		if avatar != nil {
			out["avatar"] = *avatar
		}
		out["discord_user_id"] = uid
		_ = json.NewEncoder(w).Encode(out)
	}
}

// LogoutHandler serves POST /api/v1/auth/logout: delete the session row + clear
// the cookie, then 204 (the client routes to the LoginScreen). Idempotent — a
// logout with no/invalid cookie still clears + 204s.
func LogoutHandler(db *sql.DB, cfg Config) http.HandlerFunc {
	opts := CookieOptsFromEnv()
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if c, err := r.Cookie(SessionCookieName); err == nil && c.Value != "" {
			if derr := store.DeleteSession(r.Context(), db, c.Value); derr != nil {
				slog.Warn("logout: delete session failed", "err", derr)
			}
		}
		ClearSessionCookie(w, opts)
		w.WriteHeader(http.StatusNoContent)
	}
}

// clearOAuthStateCookie expires the short-lived state cookie (same identity as
// the one login set, MaxAge<0).
func clearOAuthStateCookie(w http.ResponseWriter, opts CookieOpts) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    "",
		Path:     "/",
		Domain:   opts.Domain,
		HttpOnly: true,
		Secure:   opts.Secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
