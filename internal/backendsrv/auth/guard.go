package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"log/slog"
	"strings"
)

// Auth is the request-time bearer-token guard. It wraps the backend's *sql.DB
// (the single-writer modernc handle from internal/backendsrv/store) and resolves
// an Authorization: Bearer <code> header to the owner that minted the code.
//
// The db field is unexported and the constructor is intentionally minimal: 11-05
// (the HTTP shell) builds an *Auth around store.Open's handle and calls
// resolveToken from its middleware. The guard is framework-agnostic — 11-05 may
// wrap it in a net/http middleware or (had the 11-01 spike won) a PocketBase
// func(*core.RequestEvent) error; this package binds it to NEITHER and does NOT
// use PocketBase's apis.RequireAuth()/JWT auth-record system (guild codes are
// opaque static tokens, not PB auth records).
type Auth struct {
	db *sql.DB
}

// New returns an *Auth guarding db. 11-05 calls this once at server startup.
func New(db *sql.DB) *Auth { return &Auth{db: db} }

// ResolveToken is the exported entry point to the bearer guard, called by the
// ingest handler (internal/backendsrv/ingest, a different package) as the FIRST
// step of POST /api/v1/ingest. It delegates UNCHANGED to resolveToken (the
// package-internal implementation that 11-04's guard_test.go covers directly),
// so there is one tested code path. It returns (ownerID, codeID, true) for a
// valid active code and (0, 0, false) for a missing/malformed/unknown/revoked
// token — which the handler maps to 401, writing nothing (D-08 / BACKEND-04 / V2).
//
// The matched guild_code.id (codeID) is threaded out (Phase 17 / D-07) so the
// ingest path can stamp guild_code.last_seen for the uploading code; callers that
// don't need it (whoami) discard it with `_`.
//
// The 11-04 contract deliberately kept resolveToken lowercase until its first
// cross-package consumer existed; this export is that consumer (the security
// behavior is identical — the only new logic is threading the code-row id out).
func (a *Auth) ResolveToken(ctx context.Context, authHeader string) (ownerID, codeID int64, ok bool) {
	return a.resolveToken(ctx, authHeader)
}

// bearerPrefix is the required Authorization scheme. A header that does not start
// with exactly "Bearer " is malformed and rejected (D-08).
const bearerPrefix = "Bearer "

// resolveToken hashes the presented Bearer value and constant-time-compares it
// against every ACTIVE guild_code row (disabled_at IS NULL). It returns
// (ownerID, codeID, true) on the first match and (0, 0, false) for a missing,
// malformed, unknown, or revoked token — which 11-05's handler maps to 401,
// writing nothing. codeID is the matched guild_code.id (Phase 17 / D-07), used by
// the ingest path to stamp guild_code.last_seen.
//
// Security (D-08 / V2 / V6):
//   - The presented code is SHA-256-hashed before any comparison or DB use; the
//     plaintext is never stored and never compared directly.
//   - subtle.ConstantTimeCompare on the hash bytes makes the compare timing-safe
//     (T-11.04-02). Iterating the active rows keeps it constant-time per row; at
//     ~12 guild codes this is trivially cheap (RESEARCH Pattern 3).
//   - Only active rows are candidates, so a revoked code (disabled_at set) is
//     excluded and returns not-authenticated (T-11.04-04).
//   - No failure path reveals which check failed (all return (0, false)), and the
//     raw token / Authorization header value is NEVER logged (V7) — on a miss we
//     emit at most an "auth_reject" record carrying no token material.
//
// The token is hashed, so the WHERE clause filters only on disabled_at (no
// user-supplied SQL fragment); the query is fully parameterized in spirit — there
// is no interpolation of the presented value into SQL at all (T-11.04-05 / V5).
func (a *Auth) resolveToken(ctx context.Context, authHeader string) (ownerID, codeID int64, ok bool) {
	if !strings.HasPrefix(authHeader, bearerPrefix) { // malformed / wrong scheme -> 401
		slog.Debug("auth_reject", "reason", "missing_bearer_prefix")
		return 0, 0, false
	}
	raw := strings.TrimPrefix(authHeader, bearerPrefix)
	sum := sha256.Sum256([]byte(raw)) // hash the presented code; never compare plaintext

	rows, err := a.db.QueryContext(ctx,
		`SELECT id, owner_id, token_hash FROM guild_code WHERE disabled_at IS NULL`)
	if err != nil {
		slog.Error("auth_reject", "reason", "query_active_codes_failed", "err", err)
		return 0, 0, false
	}
	defer rows.Close()

	for rows.Next() {
		var cid, oid int64
		var stored []byte
		if err := rows.Scan(&cid, &oid, &stored); err != nil {
			continue
		}
		if subtle.ConstantTimeCompare(sum[:], stored) == 1 { // timing-safe match
			return oid, cid, true
		}
	}
	if err := rows.Err(); err != nil {
		slog.Error("auth_reject", "reason", "iterate_active_codes_failed", "err", err)
		return 0, 0, false
	}

	slog.Debug("auth_reject", "reason", "no_active_match") // unknown / revoked -> 401
	return 0, 0, false
}
