package main

// Build-time OAuth + Picker constants.
//
// These vars are populated at link time by:
//
//	go build -ldflags="-X main.OAuthClientID=... -X main.PickerAPIKey=... -X main.GCPProjectNumber=..."
//
// The values come from .planning/phases/01-end-to-end-thin-slice/oauth-config.json
// (gitignored, local-only — see Plan 01-02 SUMMARY). All three values are
// public per RESEARCH.md §4.1 / §5.4 — desktop OAuth client IDs and
// API-restricted Picker keys are visible in the consent flow and the JS
// bundle respectively. The OAuth client SECRET is intentionally absent
// because desktop clients use PKCE per RFC 7636.
//
// At runtime cmd/squirebot copies these into auth.BuildConstants and
// calls Validate(); any empty value returns auth.ErrMissingConstants
// and the binary refuses to start the OAuth flow.
var (
	OAuthClientID    = ""
	PickerAPIKey     = ""
	GCPProjectNumber = ""
	Version          = "0.1.0-dev"
)
