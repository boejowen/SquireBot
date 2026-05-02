package auth

// Plan 02-04 Task 1 — IsRevokedRefreshToken behaviour matrix.
//
// The helper recognises the canonical Google "this refresh token is
// permanently dead, prompt re-OAuth" shape (Pitfall #1 + #7 + Pitfall A
// from 02-RESEARCH.md). Two detection paths:
//
//   1. Typed: errors.As against *oauth2.RetrieveError + ErrorCode whitelist.
//   2. Defensive string match (case-insensitive) against the same three
//      OAuth codes — covers wrapping paths that flatten the typed error
//      into a generic error whose message still contains the code.
//
// Plan 02-04 wires this into runapp's makeOnInventoryChange /
// makeOnSpellbookChange so a refresh-token death surfaces as
// authSuspended + tray red within one upload cycle.

import (
	"errors"
	"fmt"
	"testing"

	"golang.org/x/oauth2"
)

func TestIsRevokedRefreshToken_NilError(t *testing.T) {
	if IsRevokedRefreshToken(nil) {
		t.Fatal("IsRevokedRefreshToken(nil) = true, want false")
	}
}

func TestIsRevokedRefreshToken_TypedInvalidGrant(t *testing.T) {
	err := &oauth2.RetrieveError{ErrorCode: "invalid_grant"}
	if !IsRevokedRefreshToken(err) {
		t.Fatal("IsRevokedRefreshToken(invalid_grant) = false, want true")
	}
}

func TestIsRevokedRefreshToken_TypedUnauthorizedClient(t *testing.T) {
	err := &oauth2.RetrieveError{ErrorCode: "unauthorized_client"}
	if !IsRevokedRefreshToken(err) {
		t.Fatal("IsRevokedRefreshToken(unauthorized_client) = false, want true")
	}
}

func TestIsRevokedRefreshToken_TypedInvalidClient(t *testing.T) {
	err := &oauth2.RetrieveError{ErrorCode: "invalid_client"}
	if !IsRevokedRefreshToken(err) {
		t.Fatal("IsRevokedRefreshToken(invalid_client) = false, want true")
	}
}

func TestIsRevokedRefreshToken_TypedInvalidRequestNotRevocation(t *testing.T) {
	// invalid_request is an OAuth-spec error but signals a malformed request,
	// NOT a dead refresh token. Must not trigger the re-OAuth UX.
	err := &oauth2.RetrieveError{ErrorCode: "invalid_request"}
	if IsRevokedRefreshToken(err) {
		t.Fatal("IsRevokedRefreshToken(invalid_request) = true, want false")
	}
}

func TestIsRevokedRefreshToken_WrappedTypedError(t *testing.T) {
	// errors.As must unwrap fmt.Errorf("...%w", ...) chains so a
	// caller-wrapped *oauth2.RetrieveError still triggers the typed path.
	inner := &oauth2.RetrieveError{ErrorCode: "invalid_grant"}
	wrapped := fmt.Errorf("token exchange: %w", inner)
	if !IsRevokedRefreshToken(wrapped) {
		t.Fatal("IsRevokedRefreshToken(wrapped invalid_grant) = false, want true")
	}
}

func TestIsRevokedRefreshToken_PlainErrorContainsInvalidGrant(t *testing.T) {
	// Defensive string-match fallback — some library wrapping flattens
	// the typed error into a generic errors.New whose message still has
	// the OAuth code embedded.
	err := errors.New("oauth2: cannot fetch token: 400 Bad Request\nResponse: {\"error\":\"invalid_grant\"}")
	if !IsRevokedRefreshToken(err) {
		t.Fatal("IsRevokedRefreshToken(string invalid_grant) = false, want true")
	}
}

func TestIsRevokedRefreshToken_PlainErrorNoMatchReturnsFalse(t *testing.T) {
	err := errors.New("network: connection refused")
	if IsRevokedRefreshToken(err) {
		t.Fatal("IsRevokedRefreshToken(network err) = true, want false")
	}
}

func TestIsRevokedRefreshToken_PlainErrorCaseInsensitive(t *testing.T) {
	// Some wrapping paths uppercase the code; the string fallback must
	// be case-insensitive.
	err := errors.New("OAUTH2: TOKEN REVOKED -- INVALID_GRANT")
	if !IsRevokedRefreshToken(err) {
		t.Fatal("IsRevokedRefreshToken(uppercase invalid_grant) = false, want true")
	}
}
