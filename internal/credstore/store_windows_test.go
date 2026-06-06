//go:build windows

package credstore

import (
	"runtime"
	"testing"
)

// TestCredTarget_Fixed asserts the single fixed target name (D-6 / A4): one
// credential per machine, no email/identity key. This runs on every platform (a
// pure string assertion, no wincred call).
func TestCredTarget_Fixed(t *testing.T) {
	if credTarget != "SquireBot:guild-code" {
		t.Fatalf("credTarget = %q, want %q", credTarget, "SquireBot:guild-code")
	}
}

// TestRoundTrip exercises the real DPAPI wincred round-trip on the Windows dev
// box and skips elsewhere (wincred is Windows-only), so `go test ./...` passes on
// any platform but actually proves the round-trip on Windows.
func TestRoundTrip(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("wincred is Windows-only")
	}

	// Best-effort clean slate (ignore a not-found error from a prior run).
	_ = Delete()
	t.Cleanup(func() { _ = Delete() })

	const code = "ABC123-round-trip"

	// Store then Read returns the same code.
	if err := Store(code); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, err := Read()
	if err != nil {
		t.Fatalf("Read after Store: %v", err)
	}
	if got != code {
		t.Fatalf("Read = %q, want %q", got, code)
	}

	// Delete removes it; a subsequent Read is not-found.
	if err := Delete(); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := Read(); err == nil {
		t.Fatal("Read after Delete = nil error, want not-found (needs onboarding)")
	}
}

// TestRead_NotFound_NeedsOnboarding asserts that Read() with no stored credential
// returns a non-nil error so callers treat it as "first run / needs onboarding".
func TestRead_NotFound_NeedsOnboarding(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("wincred is Windows-only")
	}
	_ = Delete() // ensure absent
	if _, err := Read(); err == nil {
		t.Fatal("Read with no credential = nil error, want a not-found error")
	}
}

// TestDelete_NotFound returns the not-found error when nothing is stored (callers
// ignore not-found — idempotent enough).
func TestDelete_NotFound(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("wincred is Windows-only")
	}
	_ = Delete() // ensure absent
	if err := Delete(); err == nil {
		t.Fatal("Delete with no credential = nil error, want a not-found error")
	}
}
