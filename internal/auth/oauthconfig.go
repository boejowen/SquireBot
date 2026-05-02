package auth

import "errors"

// ErrMissingConstants is returned by BuildConstants.Validate when the
// binary was built without the four -ldflags values that come from
// .planning/phases/01-end-to-end-thin-slice/oauth-config.json. The
// binary must refuse to run OAuth in this state — every authorisation
// URL would be rejected by Google with an opaque "invalid_client" or
// the token exchange with "client_secret is missing".
var ErrMissingConstants = errors.New("auth: build-time OAuth constants missing — rebuild with -ldflags='-X main.OAuthClientID=... -X main.OAuthClientSecret=... -X main.PickerAPIKey=... -X main.GCPProjectNumber=...' (per docs/oauth-setup.md)")

// BuildConstants holds the four Cloud-Console values the binary needs
// to talk to Google. Each is loaded from -ldflags at build time and
// passed in from cmd/squirebot's package-main vars (see
// cmd/squirebot/build_constants.go). The Picker constants are unused by
// Plan 03 itself but are defined here so Plan 06 (Drive Picker) and
// Plan 07 (wizard) can consume the same struct without circular
// imports.
type BuildConstants struct {
	OAuthClientID     string // public per RESEARCH.md §4.1 — desktop client uses PKCE
	OAuthClientSecret string // effectively public for desktop apps; Google's token endpoint requires it as a parameter even with PKCE — see docs/build-and-install.md "About the client secret"
	PickerAPIKey      string // public per RESEARCH.md §5.4 — restricted to Google Picker API
	GCPProjectNumber  string // public — Picker AppID (Plan 06)
	// WatcherVersion is the build-time Version constant from
	// cmd/squirebot/main.go, plumbed here so internal packages
	// (sheet.UpsertCharOwner, the heartbeat in Phase 2 plans) can stamp
	// it onto _char_owner.watcher_version + _status.watcher_version
	// rows without importing main. Empty string is acceptable — Validate
	// does not require it (the four OAuth values remain mandatory).
	WatcherVersion string
}

// Validate returns ErrMissingConstants if any of the four values is
// empty. Plan 07 calls this at startup before invoking RunOAuth.
func (b BuildConstants) Validate() error {
	if b.OAuthClientID == "" || b.OAuthClientSecret == "" || b.PickerAPIKey == "" || b.GCPProjectNumber == "" {
		return ErrMissingConstants
	}
	return nil
}
