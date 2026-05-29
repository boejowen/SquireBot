package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
)

// MintCode mints a per-guildie opaque bearer token for the owner labeled
// ownerLabel and returns its plaintext. It is the logic behind the
// `squirebot-server mint-code --owner <label>` CLI subcommand wired in 11-05
// (D-05); there is NO admin HTTP mint endpoint in P11 (that is P15).
//
// Flow (token-gen shape mirrors internal/auth/pkce.go:27-34):
//
//  1. raw = 32 bytes from crypto/rand (NOT math/rand — V6 / T-11.04-06).
//  2. code = base64.RawURLEncoding(raw) — the plaintext, shown ONCE.
//  3. token_hash = sha256(code) — the ONLY form persisted (hash-only at rest).
//  4. upsert the owner by label, INSERT the guild_code row with the HASH.
//  5. print the plaintext to stdout with a "store now — not shown again"
//     message and return it.
//
// The plaintext is NEVER stored and NEVER logged via slog (V6/V7): it crosses to
// the maintainer exactly once, via the explicit fmt.Printf below (stdout), and
// as the returned string. token_hash is a 32-byte BLOB and is UNIQUE in the
// schema, so a (cryptographically impossible) collision would surface as an
// INSERT error rather than silently overwriting another code.
func MintCode(db *sql.DB, ownerLabel string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil { // crypto/rand (NOT math/rand)
		return "", fmt.Errorf("generate token entropy: %w", err)
	}
	code := base64.RawURLEncoding.EncodeToString(raw) // plaintext, shown ONCE
	sum := sha256.Sum256([]byte(code))

	ownerID, err := upsertOwner(db, ownerLabel)
	if err != nil {
		return "", err
	}

	// Store ONLY the SHA-256 hash (sum[:]), never the plaintext. Parameterized
	// ? placeholders only (V5).
	if _, err := db.Exec(
		`INSERT INTO guild_code (owner_id, token_hash, label) VALUES (?, ?, ?)`,
		ownerID, sum[:], ownerLabel); err != nil {
		return "", fmt.Errorf("insert guild code for %q: %w", ownerLabel, err)
	}

	// The plaintext goes to stdout ONLY (never slog — V7); it is not shown again.
	fmt.Printf("Guild code for %s (store now — not shown again):\n\n  %s\n\n", ownerLabel, code)
	return code, nil
}
