// Package auth implements the SquireBot backend's bearer-token authentication
// (BACKEND-04): the maintainer-run mint-code/revoke-code CLI logic and the
// request-time guard that resolves an Authorization: Bearer <code> to its owner.
//
// SECURITY — hash-only at rest. A guild code's 32-byte crypto/rand plaintext
// exists exactly ONCE, at mint time, where it is printed to stdout and returned
// to the caller (cmd/squirebot-server in 11-05). Only its SHA-256 hash is ever
// persisted (guild_code.token_hash, a 32-byte BLOB). This mirrors the watcher's
// hash-only credential discipline (internal/auth/store.go: "the secret never
// leaves as plaintext — only the hashed/encrypted blob is persisted"), with a
// SQLite BLOB column standing in for wincred/DPAPI. The plaintext is NEVER
// written to the DB and NEVER passed to slog (V6/V7).
//
// All crypto is stdlib (crypto/rand, crypto/sha256, crypto/subtle) — never
// hand-rolled (V6). The token-gen shape is the canonical repo analog
// internal/auth/pkce.go:27-34 (crypto/rand -> base64.RawURLEncoding -> sha256).
//
// Verdict-agnostic. resolveToken (guard.go) and MintCode/RevokeCode here are
// pure crypto + database/sql against the 11-02 schema; 11-05 imports them
// UNCHANGED whether the HTTP shell ends up net/http or PocketBase (the 11-01
// verdict was HAND-ROLLED net/http, but this package does not depend on that).
package auth

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// upsertOwner returns the id of the owner row labeled label, creating it on
// first use. The owner table has no UNIQUE constraint on label (a label is a
// human-friendly handle, not an identity key — D-13), so this is a SELECT-then-
// INSERT rather than an INSERT ... ON CONFLICT: an existing label is reused so
// re-minting for the same guildie does not spawn duplicate owners. Parameterized
// ? placeholders only (V5).
func upsertOwner(db *sql.DB, label string) (int64, error) {
	var id int64
	// IN-02: the reserved sentinel owner (store.GuildSentinelOwnerID, label 'guild') is
	// NEVER reused by a mint — `mint-code --owner guild` for a real guildie must not bind
	// their codes to the guild-held bank/bot owner. Exclude it by id so a literal "guild"
	// label falls through to the INSERT (a fresh, distinct owner row).
	err := db.QueryRow(`SELECT id FROM owner WHERE label = ? AND id <> ?`,
		label, store.GuildSentinelOwnerID).Scan(&id)
	switch {
	case err == nil:
		return id, nil
	case errors.Is(err, sql.ErrNoRows):
		res, ierr := db.Exec(`INSERT INTO owner (label) VALUES (?)`, label)
		if ierr != nil {
			return 0, fmt.Errorf("insert owner %q: %w", label, ierr)
		}
		id, ierr = res.LastInsertId()
		if ierr != nil {
			return 0, fmt.Errorf("last insert id for owner %q: %w", label, ierr)
		}
		return id, nil
	default:
		return 0, fmt.Errorf("lookup owner %q: %w", label, err)
	}
}

// RevokeCode disables the active guild_code row(s) matching idOrLabel by setting
// disabled_at, so a subsequent resolveToken with that code returns ok=false
// (D-09 / T-11.04-04). idOrLabel matches EITHER the row id OR the owner label
// (D-09 accepts either) — already-disabled rows are left untouched by the
// `disabled_at IS NULL` guard, so revoking twice is idempotent. The token
// plaintext is irrelevant here (we never have it post-mint); revocation works
// purely off the stored row identity. Parameterized ? placeholders only (V5).
func RevokeCode(db *sql.DB, idOrLabel string) error {
	if _, err := db.Exec(
		`UPDATE guild_code SET disabled_at = datetime('now')
		 WHERE (label = ? OR id = ?) AND disabled_at IS NULL`,
		idOrLabel, idOrLabel); err != nil {
		return fmt.Errorf("revoke guild code %q: %w", idOrLabel, err)
	}
	return nil
}
