//go:build !windows

package onboarding

// The watcher is Windows-only (Win32 dialog, DPAPI, console). These stubs keep
// `go build ./...` / `go test ./...` green on a non-Windows CI runner; they are
// never reached in production. Both return ErrUnsupported so a caller on the
// wrong platform fails loudly rather than silently no-op'ing.

// PromptGuildCode is unsupported off Windows.
func PromptGuildCode(title, prompt string) (string, error) {
	return "", ErrUnsupported
}

// PickEQFolder is unsupported off Windows.
func PickEQFolder(title string) (string, error) {
	return "", ErrUnsupported
}
