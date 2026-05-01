package main

// Build-time OAuth + Picker constants.
//
// These vars are populated at link time by:
//
//	go build -ldflags="-X main.OAuthClientID=... -X main.OAuthClientSecret=... -X main.PickerAPIKey=... -X main.GCPProjectNumber=..."
//
// The values come from .planning/phases/01-end-to-end-thin-slice/oauth-config.json
// (gitignored, local-only — see Plan 01-02 SUMMARY). All four values are
// effectively public per RESEARCH.md §4.1 / §5.4 — desktop OAuth client
// IDs and API-restricted Picker keys are visible in the consent flow and
// the JS bundle respectively. The OAuth client SECRET, despite the name,
// is also effectively public for desktop apps: per Google's docs, "When
// a client runs on a device, the client_secret is no longer truly
// confidential." Google's token endpoint nonetheless REQUIRES it as a
// parameter even when PKCE is in use, so we bake it in via -ldflags
// alongside the others. Blast radius is bounded by the Picker API
// restriction and the drive.file scope.
//
// At runtime cmd/squirebot copies these into auth.BuildConstants and
// calls Validate(); any empty value returns auth.ErrMissingConstants
// and the binary refuses to start the OAuth flow.
var (
	OAuthClientID     = ""
	OAuthClientSecret = ""
	PickerAPIKey      = ""
	GCPProjectNumber  = ""
	Version           = "0.1.0-dev"
)
