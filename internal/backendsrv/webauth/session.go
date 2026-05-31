package webauth

// session.go is the opaque-session cookie + request-gate layer (Task 2, D-05).
// It composes the 15-01 store session helpers (ResolveSession / TouchSession /
// IsOfficer) into:
//   - the cross-subdomain session cookie (SetSessionCookie / ClearSessionCookie)
//     — httpOnly + Secure + SameSite=Lax + Domain=squirebot.quest so it rides
//     from the apex SvelteKit origin to api.squirebot.quest (same registrable
//     domain ⇒ same-site for Lax) yet stays out of JS (T-15-07/T-15-10);
//   - RequireSession — the D-01 read-API gate: a missing/invalid/expired session
//     → 401, fail-closed; a valid session → the request runs with the
//     discord_user_id in context AND the rolling expiry is bumped (TouchSession);
//   - RequireOfficer — the officer gate for the 15-03 officer-only write forms:
//     a valid session that is NOT in guild_admins → 403 {"error":"not_authorized"}.
//
// The cookie carries only the OPAQUE id; the store persists only its sha256
// hash (T-15-12 — a forged cookie value simply fails ResolveSession). This is
// the SIBLING of the watcher bearer guard: the bearer path stays the ingest
// contract; this cookie path gates the human read + write API.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// SessionCookieName is the name of the opaque-session cookie (D-05).
const SessionCookieName = "sb_session"

// defaultCookieDomain is the registrable domain the session cookie is scoped to
// (D-05 / 15-RESEARCH §2). Scoping to the registrable domain (NOT the api.
// subdomain) is what makes the cookie ride to api.squirebot.quest from the apex
// SvelteKit origin; the leading-dot form is implied by the modern spec.
const defaultCookieDomain = "squirebot.quest"

// CookieOpts are the deploy-environment-dependent session-cookie attributes.
// Domain is the registrable domain; Secure is true in prod (Caddy serves TLS)
// and may be overridden to false only for a local plain-http dev run (a Secure
// cookie is dropped by the browser over http).
type CookieOpts struct {
	Domain string
	Secure bool
}

// CookieOptsFromEnv reads the cookie attributes from the environment:
//   - SQUIREBOT_COOKIE_DOMAIN (default "squirebot.quest" — the registrable domain);
//   - Secure defaults to true and is set to false ONLY when
//     SQUIREBOT_COOKIE_INSECURE=1 (a deliberate local-http dev override). Prod
//     never sets that var, so Secure stays true.
func CookieOptsFromEnv() CookieOpts {
	domain := os.Getenv("SQUIREBOT_COOKIE_DOMAIN")
	if domain == "" {
		domain = defaultCookieDomain
	}
	return CookieOpts{
		Domain: domain,
		Secure: os.Getenv("SQUIREBOT_COOKIE_INSECURE") != "1",
	}
}

// SetSessionCookie writes the opaque session id as the session cookie with the
// full locked attribute set: HttpOnly + Secure + SameSite=Lax + Domain + Path=/
// + a 30-day MaxAge (store.SessionTTLSeconds). Built via http.Cookie (NOT a
// hand-assembled string) so Domain/SameSite serialize correctly.
func SetSessionCookie(w http.ResponseWriter, sessionID string, opts CookieOpts) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    sessionID, // OPAQUE id; only its sha256 is in web_session
		Path:     "/",
		Domain:   opts.Domain,
		HttpOnly: true, // JS cannot read it (XSS-resistant — T-15-07)
		Secure:   opts.Secure,
		SameSite: http.SameSiteLaxMode, // cross-subdomain + permits the OAuth return GET
		MaxAge:   int(store.SessionTTLSeconds),
	})
}

// ClearSessionCookie expires the session cookie (sign-out). Same Name/Path/
// Domain/attribute identity as SetSessionCookie with MaxAge:-1 + an empty value
// so the browser deletes it.
func ClearSessionCookie(w http.ResponseWriter, opts CookieOpts) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		Domain:   opts.Domain,
		HttpOnly: true,
		Secure:   opts.Secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1, // delete now
	})
}

// ctxKey is the unexported context-key type for the resolved discord_user_id, so
// it can never collide with another package's context value.
type ctxKey struct{}

// userCtxKey is the single context key under which RequireSession stashes the
// authenticated discord_user_id.
var userCtxKey = ctxKey{}

// UserFromContext returns the authenticated discord_user_id RequireSession put in
// the request context, and whether one is present. Handlers behind
// RequireSession/RequireOfficer read the caller identity through this.
func UserFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(userCtxKey).(string)
	return v, ok
}

// writeJSONError writes a {"error":"code"} JSON body with the given status —
// the established error shape (mirrors the ingest handlers / the v1 error
// strings the store returns).
func writeJSONError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}

// resolveSessionUser reads + validates the session cookie and returns the
// authenticated discord_user_id. It is fail-closed: no cookie, an empty value,
// or any ResolveSession error (not-found / expired / scan error) → ("", false).
// On success it rolls the expiry forward (TouchSession — the D-05 rolling
// window). The raw cookie value is NEVER logged (V7).
func resolveSessionUser(r *http.Request, db *sql.DB) (string, bool) {
	c, err := r.Cookie(SessionCookieName)
	if err != nil || c.Value == "" {
		return "", false // no cookie → fail-closed
	}
	now := time.Now().Unix()
	uid, err := store.ResolveSession(r.Context(), db, c.Value, now)
	if err != nil {
		// ErrSessionNotFound / ErrSessionExpired / a scan error → not authed.
		// Log only the reason class, never the cookie value.
		if !errors.Is(err, store.ErrSessionNotFound) && !errors.Is(err, store.ErrSessionExpired) {
			slog.Warn("session resolve failed", "err", err)
		}
		return "", false
	}
	// Rolling TTL (D-05): bump expires_at to now+TTL on each authenticated hit so
	// an active user is never logged out mid-use. A touch failure is non-fatal —
	// the request is already authenticated; log and proceed.
	if terr := store.TouchSession(r.Context(), db, c.Value, now+store.SessionTTLSeconds); terr != nil {
		slog.Warn("session touch failed", "err", terr)
	}
	return uid, true
}

// RequireSession is the D-01 read-API gate. It resolves the session cookie; a
// missing/invalid/expired session → 401 {"error":"unauthorized"} and next does
// NOT run (fail-closed — the gate is at the API, not just the frontend). A valid
// session → the discord_user_id is placed in the request context and next runs.
func RequireSession(db *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := resolveSessionUser(r, db)
		if !ok {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		ctx := context.WithValue(r.Context(), userCtxKey, uid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireOfficer gates the officer-only write surfaces (15-03). It first applies
// the RequireSession identity resolution; then it re-checks guild_admins
// membership (store.IsOfficer). A valid-but-non-officer session → 403
// {"error":"not_authorized"} (matches v1's error string + the UI-SPEC routing);
// an unauthenticated request → 401 (same as RequireSession). The discord_user_id
// is placed in context for the wrapped handler.
//
// NB: this is the REQUEST-TIME officer gate. The 15-03 officer-only MUTATORS
// additionally re-authorize INSIDE their *sql.Tx (store.AddOfficerTx /
// RemoveOfficerTx / eviction) to close the TOCTOU window — this gate is the
// outer, cheap rejection, not a substitute for the in-tx re-check.
func RequireOfficer(db *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := resolveSessionUser(r, db)
		if !ok {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		isOfficer, err := store.IsOfficer(r.Context(), db, uid)
		if err != nil {
			slog.Error("officer check failed", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}
		if !isOfficer {
			writeJSONError(w, http.StatusForbidden, "not_authorized")
			return
		}
		ctx := context.WithValue(r.Context(), userCtxKey, uid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
