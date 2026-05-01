package auth

import (
	"os/exec"
	"runtime"
)

// OpenBrowser launches the user's default browser at url.
//
// On Windows this uses `rundll32 url.dll,FileProtocolHandler <url>` per
// RESEARCH.md §11. The simpler `cmd /c start <url>` form has shell
// metacharacter edge cases on URLs that contain `&` (every OAuth URL
// does — it's a query-parameter delimiter). rundll32 sidesteps the
// shell entirely.
//
// On darwin/linux the standard system openers are used so `go test`
// developer ergonomics work on dev machines that aren't Windows. The
// production binary is Windows-only.
//
// If the launcher cannot be spawned (rare — it would mean rundll32
// is missing from PATH or AV blocked it), the error is returned
// non-nil. RunOAuth surfaces that as a slog.Warn and falls back to
// the AUTH-01 60-second manual-paste path; the wizard's start.html
// (Plan 07) shows a copy-this-URL textbox.
func OpenBrowser(url string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
