// Package picker serves the Drive Picker page from the loopback HTTP server
// already running for Plan 03's OAuth flow. The same browser tab the user
// completed consent in is redirected to /picker, the embedded HTML below
// loads the classic Web Picker JS, and the user's selection POSTs back to
// /picker/result for canonical_id validation (Plan 05's ValidateWorkbook).
//
// We use the classic Web Picker (apis.google.com/js/api.js + Picker JS API),
// NOT the desktop-mode Picker (RESEARCH.md §5.1 / Pitfall #5). Desktop mode
// requires a public HTTPS redirect_uri and forbids combining drive.file with
// openid+userinfo.email — both incompatible with our loopback + scope set.
package picker

import _ "embed"

// pickerHTMLTemplate is the source of the html/template parsed in NewServer.
// The three placeholders {{.AccessToken}}, {{.AppID}}, {{.APIKey}} are
// substituted at request time by Server.handlePicker (NOT at build time):
// the access token is fetched fresh from the oauth2.TokenSource on every
// GET so a token-refresh during the wizard's lifetime doesn't ship a stale
// token to the page.
//
//go:embed picker.html
var pickerHTMLTemplate string
