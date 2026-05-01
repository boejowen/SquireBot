// Package auth implements the OAuth 2.0 loopback PKCE flow described in
// .planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md §4 and the
// supporting wincred-backed refresh-token store (AUTH-04).
//
// SECURITY: refresh tokens NEVER leave this package as plaintext — they
// are written to Windows Credential Manager via store.go and read back
// only by callers that hand them straight to oauth2.TokenSource. The
// Config struct in internal/config has no field for any OAuth credential.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// NewPKCEPair returns (code_verifier, code_challenge_S256) per RFC 7636.
//
// The verifier is 32 random bytes base64url-NoPadding-encoded, yielding
// exactly 43 characters in [A-Za-z0-9_-] — the minimum length that
// satisfies the RFC's entropy floor. The challenge is the base64url
// encoding of SHA-256(verifier), used with code_challenge_method=S256.
//
// Cited: RFC 7636 — verifier is high-entropy [A-Z][a-z][0-9]-._~,
// min 43 / max 128 chars; S256 is base64url(sha256(verifier)).
func NewPKCEPair() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}
