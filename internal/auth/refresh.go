package auth

// Plan 02-04 Task 1 — refresh-token death detection (AUTH-05).
//
// IsRevokedRefreshToken recognises the canonical Google "this refresh
// token is permanently dead, prompt re-OAuth" shape. It covers all four
// invalidation modes documented in PITFALLS.md Pitfall #7 (Testing-mode
// 7-day expiry, six-month inactivity, user-revoked, password-change with
// token-revoke setting) plus the OAuth-spec sibling errors that surface
// when the OAuth client itself has been deleted/suspended
// (unauthorized_client, invalid_client).
//
// Detection priority:
//   1. errors.As against *oauth2.RetrieveError + ErrorCode whitelist (typed path).
//   2. Defensive case-insensitive string match for wrapping paths that
//      flatten the typed error into a generic error whose message still
//      contains the OAuth code.
//
// Plan 02-04 Task 3 wires this into runapp.makeOnInventoryChange /
// makeOnSpellbookChange next to errors.Is(err, sheet.ErrPermanentAuth).
// Either signal trips authSuspended → tray red → Reauthorize menu item
// surfaced. CONTEXT.md (Refresh-Token UX, locked): no silent retry-loop;
// surface within one upload cycle.

import (
	"errors"
	"strings"

	"golang.org/x/oauth2"
)

// IsRevokedRefreshToken returns true if err signals a refresh token that
// will never succeed again. See package comment for the four invalidation
// modes covered.
func IsRevokedRefreshToken(err error) bool {
	if err == nil {
		return false
	}
	var re *oauth2.RetrieveError
	if errors.As(err, &re) {
		switch re.ErrorCode {
		case "invalid_grant", "unauthorized_client", "invalid_client":
			return true
		}
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "invalid_grant") ||
		strings.Contains(s, "unauthorized_client") ||
		strings.Contains(s, "invalid_client") {
		return true
	}
	return false
}
