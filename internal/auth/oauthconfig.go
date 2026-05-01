package auth

import "errors"

// ErrMissingConstants is returned by BuildConstants.Validate when the
// binary was built without the three -ldflags values that come from
// .planning/phases/01-end-to-end-thin-slice/oauth-config.json. The
// binary must refuse to run OAuth in this state — every authorisation
// URL would be rejected by Google with an opaque "invalid_client".
var ErrMissingConstants = errors.New("auth: build-time OAuth constants missing — rebuild with -ldflags='-X main.OAuthClientID=... -X main.PickerAPIKey=... -X main.GCPProjectNumber=...' (per docs/oauth-setup.md)")

// BuildConstants holds the three Cloud-Console values the binary needs
// to talk to Google. Each is loaded from -ldflags at build time and
// passed in from cmd/squirebot's package-main vars (see
// cmd/squirebot/build_constants.go). The Picker constants are unused by
// Plan 03 itself but are defined here so Plan 06 (Drive Picker) and
// Plan 07 (wizard) can consume the same struct without circular
// imports.
type BuildConstants struct {
	OAuthClientID    string // public per RESEARCH.md §4.1 — desktop client uses PKCE, no secret
	PickerAPIKey     string // public per RESEARCH.md §5.4 — restricted to Google Picker API
	GCPProjectNumber string // public — Picker AppID (Plan 06)
}

// Validate returns ErrMissingConstants if any of the three values is
// empty. Plan 07 calls this at startup before invoking RunOAuth.
func (b BuildConstants) Validate() error {
	if b.OAuthClientID == "" || b.PickerAPIKey == "" || b.GCPProjectNumber == "" {
		return ErrMissingConstants
	}
	return nil
}
