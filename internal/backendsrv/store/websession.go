package store

// websession.go is the Discord-login session + web-user persistence layer
// (15-01 Task 2, AUTH-08/AUTH-09 / D-03 / D-05). The Discord OAuth callback
// (15-02) calls UpsertWebUser + CreateSession on a successful membership pass;
// the session-gate middleware (15-02) calls ResolveSession + TouchSession on
// every authenticated request; sign-out calls DeleteSession.
//
// SECURITY — hash-only at rest (T-15-01). The opaque session id's plaintext
// lives ONLY in the user's cookie. Only its SHA-256 hash is ever persisted
// (web_session.session_hash). This mirrors the bearer-token discipline in
// auth/guard.go + auth/mint.go (crypto/rand -> hex/base64 -> sha256, the
// plaintext never stored, never logged). A DB leak yields no usable tokens.
//
// These are package-level funcs taking (ctx, db *sql.DB, ...) — the same shape
// the read helpers (readviews.go) and the *Tx mutators (binding.go) use, so the
// 15-02 handlers compose them directly. Parameterized ? placeholders ONLY (V5).
// slog is left to the handler layer; this layer returns %w-wrapped errors.

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
)

// ErrSessionExpired is returned by ResolveSession when a session row exists but
// its expires_at is in the past — fail-closed: an expired session authenticates
// nobody. The 15-02 middleware maps it (like ErrSessionNotFound) to 401.
var ErrSessionExpired = errors.New("session expired")

// ErrSessionNotFound is returned by ResolveSession when no web_session row
// matches the presented (hashed) session id. The 15-02 middleware maps it to
// 401, writing nothing.
var ErrSessionNotFound = errors.New("session not found")

// SessionTTLSeconds is the rolling session lifetime (D-05): 30 days. Low-
// sensitivity hobby data + 12 users ⇒ re-login friction is worse than a long
// bounded window. The window is ROLLING — TouchSession bumps expires_at on each
// authenticated hit, so an active user is never logged out mid-use, while a
// departed guildie's session lapses within 30 days of their last visit.
const SessionTTLSeconds int64 = 30 * 24 * 60 * 60 // 2592000

// GenerateSessionID returns a fresh opaque session id: 32 bytes from crypto/rand
// (NOT math/rand — mirrors auth/mint.go:31-34), hex-encoded. The plaintext is
// what goes in the user's cookie; only HashSession(id) is ever stored.
func GenerateSessionID() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil { // crypto/rand (NOT math/rand)
		return "", fmt.Errorf("generate session entropy: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

// HashSession returns the SHA-256 hex digest of the opaque session id. This is
// the ONLY form persisted (web_session.session_hash) and the value every lookup
// queries by — the plaintext is never compared directly and never stored
// (mirrors auth/guard.go's sha256.Sum256 of the presented bearer code).
func HashSession(id string) string {
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:])
}

// UpsertWebUser records (or refreshes) the Discord identity captured at login
// (AUTH-09 / D-03). first_seen is set once on the first INSERT and NEVER changed
// on a re-login (ON CONFLICT updates only username/avatar/last_login). The
// snowflake (discord_user_id) is the stable key the deferred v2 pinger will DM.
// Idempotent on discord_user_id: re-logins update in place, no duplicate rows.
func UpsertWebUser(ctx context.Context, db *sql.DB, discordUserID, username, avatar string, now int64) error {
	if _, err := db.ExecContext(ctx,
		`INSERT INTO web_user (discord_user_id, username, avatar, first_seen, last_login)
		 VALUES (?,?,?,?,?)
		 ON CONFLICT(discord_user_id) DO UPDATE SET
		   username=excluded.username, avatar=excluded.avatar, last_login=excluded.last_login`,
		discordUserID, username, nullIfEmpty(avatar), now, now,
	); err != nil {
		return fmt.Errorf("upsert web_user (discord_user_id=%q): %w", discordUserID, err)
	}
	return nil
}

// GetWebUser reads the username + avatar for discordUserID (for the whoami
// response in 15-02/15-03). avatar is nullable (Discord users without a custom
// avatar) → returned as *string (nil when NULL). A missing user returns a
// %w-wrapped sql.ErrNoRows so the caller can errors.Is it.
func GetWebUser(ctx context.Context, db *sql.DB, discordUserID string) (username string, avatar *string, err error) {
	var av sql.NullString
	qerr := db.QueryRowContext(ctx,
		`SELECT username, avatar FROM web_user WHERE discord_user_id = ?`, discordUserID,
	).Scan(&username, &av)
	if qerr != nil {
		return "", nil, fmt.Errorf("get web_user (discord_user_id=%q): %w", discordUserID, qerr)
	}
	if av.Valid {
		v := av.String
		avatar = &v
	}
	return username, avatar, nil
}

// CreateSession inserts a web_session row storing ONLY HashSession(sessionID)
// (never the plaintext — T-15-01), with created_at=now and expires_at=now+
// ttlSeconds. The web_user row must already exist (FK); 15-02 calls UpsertWebUser
// first in the same login flow.
func CreateSession(ctx context.Context, db *sql.DB, discordUserID, sessionID string, now, ttlSeconds int64) error {
	if _, err := db.ExecContext(ctx,
		`INSERT INTO web_session (session_hash, discord_user_id, created_at, expires_at)
		 VALUES (?,?,?,?)`,
		HashSession(sessionID), discordUserID, now, now+ttlSeconds,
	); err != nil {
		return fmt.Errorf("create web_session (discord_user_id=%q): %w", discordUserID, err)
	}
	return nil
}

// ResolveSession looks up the session by HashSession(sessionID) and returns the
// owning discord_user_id. It is FAIL-CLOSED (T-15-01):
//   - no matching row        → ErrSessionNotFound
//   - row present but expired → ErrSessionExpired (expires_at < now)
//   - live row               → (discord_user_id, nil)
//
// The plaintext sessionID is hashed before the query; it is never interpolated
// into SQL and never compared directly (mirrors auth/guard.go).
func ResolveSession(ctx context.Context, db *sql.DB, sessionID string, now int64) (discordUserID string, err error) {
	var expiresAt int64
	qerr := db.QueryRowContext(ctx,
		`SELECT discord_user_id, expires_at FROM web_session WHERE session_hash = ?`,
		HashSession(sessionID),
	).Scan(&discordUserID, &expiresAt)
	switch {
	case errors.Is(qerr, sql.ErrNoRows):
		return "", ErrSessionNotFound
	case qerr != nil:
		return "", fmt.Errorf("resolve web_session: %w", qerr)
	}
	if expiresAt < now { // fail-closed on expiry
		return "", ErrSessionExpired
	}
	return discordUserID, nil
}

// TouchSession bumps expires_at to newExpiry for the session matching
// HashSession(sessionID) — the rolling-window refresh (D-05) the 15-02
// middleware calls after a successful ResolveSession. A no-match is a silent
// no-op (RowsAffected==0); the caller already authenticated via ResolveSession.
func TouchSession(ctx context.Context, db *sql.DB, sessionID string, newExpiry int64) error {
	if _, err := db.ExecContext(ctx,
		`UPDATE web_session SET expires_at = ? WHERE session_hash = ?`,
		newExpiry, HashSession(sessionID),
	); err != nil {
		return fmt.Errorf("touch web_session: %w", err)
	}
	return nil
}

// DeleteSession removes the web_session row matching HashSession(sessionID)
// (sign-out). Idempotent: deleting an absent session is a no-op.
func DeleteSession(ctx context.Context, db *sql.DB, sessionID string) error {
	if _, err := db.ExecContext(ctx,
		`DELETE FROM web_session WHERE session_hash = ?`, HashSession(sessionID),
	); err != nil {
		return fmt.Errorf("delete web_session: %w", err)
	}
	return nil
}

// nullIfEmpty maps "" to a NULL avatar so the nullable column stores NULL rather
// than an empty string (Discord users without a custom avatar). Keeps GetWebUser's
// *string nil-on-no-avatar contract clean.
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
