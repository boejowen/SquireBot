package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"regexp"
	"testing"
)

// TestPKCEPairLength asserts the verifier is exactly 43 characters
// (32 random bytes base64url-NoPadding-encoded). Per RFC 7636 §4.1
// the verifier MUST be in [43, 128]; 43 is the minimum that satisfies
// the entropy floor while keeping URLs compact.
func TestPKCEPairLength(t *testing.T) {
	v, c, err := NewPKCEPair()
	if err != nil {
		t.Fatalf("NewPKCEPair: %v", err)
	}
	if len(v) != 43 {
		t.Errorf("verifier length = %d, want 43", len(v))
	}
	// SHA-256 → 32 bytes → base64url-NoPadding → 43 chars
	if len(c) != 43 {
		t.Errorf("challenge length = %d, want 43", len(c))
	}
}

// TestPKCEVerifierCharClass asserts the verifier matches RFC 7636's
// allowed character class [A-Za-z0-9_-] (base64url alphabet, no padding).
// The full RFC 7636 alphabet also permits . and ~, but base64url-NoPadding
// only emits the strict subset, which is a valid subset.
func TestPKCEVerifierCharClass(t *testing.T) {
	re := regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	for i := 0; i < 32; i++ {
		v, _, err := NewPKCEPair()
		if err != nil {
			t.Fatalf("NewPKCEPair: %v", err)
		}
		if !re.MatchString(v) {
			t.Errorf("verifier %q does not match [A-Za-z0-9_-]+", v)
		}
	}
}

// TestPKCEChallengeIsSHA256OfVerifier asserts that the S256 challenge
// is base64url-NoPadding(SHA256(verifier)) — the exact RFC 7636 §4.2
// transformation.
func TestPKCEChallengeIsSHA256OfVerifier(t *testing.T) {
	v, c, err := NewPKCEPair()
	if err != nil {
		t.Fatalf("NewPKCEPair: %v", err)
	}
	sum := sha256.Sum256([]byte(v))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	if c != want {
		t.Errorf("challenge mismatch: got %q, want %q", c, want)
	}
}

// TestPKCEEntropy asserts that 1000 calls produce 1000 distinct
// verifiers. With 256 bits of entropy this is statistically certain;
// any failure indicates broken randomness, not a normal collision.
func TestPKCEEntropy(t *testing.T) {
	const n = 1000
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		v, _, err := NewPKCEPair()
		if err != nil {
			t.Fatalf("NewPKCEPair: %v", err)
		}
		if _, dup := seen[v]; dup {
			t.Fatalf("duplicate verifier on iteration %d: %q", i, v)
		}
		seen[v] = struct{}{}
	}
}
