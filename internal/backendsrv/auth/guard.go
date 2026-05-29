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

// bearerPrefix is the required Authorization scheme. A header that does not start
// with exactly "Bearer " is malformed and rejected (D-08).
const bearerPrefix = "Bearer "

// resolveToken hashes the presented Bearer value and constant-time-compares it
// against every ACTIVE guild_code row (disabled_at IS NULL). It returns
// (ownerID, true) on the first match and (0, false) for a missing, malformed,
// unknown, or revoked token — which 11-05's handler maps to 401, writing nothing.
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
func (a *Auth) resolveToken(ctx context.Context, authHeader string) (ownerID int64, ok bool) {
	if !strings.HasPrefix(authHeader, bearerPrefix) { // malformed / wrong scheme -> 401
		slog.Debug("auth_reject", "reason", "missing_bearer_prefix")
		return 0, false
	}
	raw := strings.TrimPrefix(authHeader, bearerPrefix)
	sum := sha256.Sum256([]byte(raw)) // hash the presented code; never compare plaintext

	rows, err := a.db.QueryContext(ctx,
		`SELECT owner_id, token_hash FROM guild_code WHERE disabled_at IS NULL`)
	if err != nil {
		slog.Error("auth_reject", "reason", "query_active_codes_failed", "err", err)
		return 0, false
	}
	defer rows.Close()

	for rows.Next() {
		var oid int64
		var stored []byte
		if err := rows.Scan(&oid, &stored); err != nil {
			continue
		}
		if subtle.ConstantTimeCompare(sum[:], stored) == 1 { // timing-safe match
			return oid, true
		}
	}
	if err := rows.Err(); err != nil {
		slog.Error("auth_reject", "reason", "iterate_active_codes_failed", "err", err)
		return 0, false
	}

	slog.Debug("auth_reject", "reason", "no_active_match") // unknown / revoked -> 401
	return 0, false
}
