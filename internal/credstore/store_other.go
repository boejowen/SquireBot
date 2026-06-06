//go:build !windows

// Package credstore — Linux/Unix variant (Phase 25, D-03). The Windows build
// keeps the bearer guild code in Windows Credential Manager (DPAPI, see
// store_windows.go); a headless Linux box has no DPAPI/keyring equivalent
// without a CGO/D-Bus dependency, so this variant stores the code in a 0600
// file under XDG config — the standard CLI-tool token convention (gh/aws/ssh).
//
// SECURITY (carries over verbatim from the Windows rule, V7 / AUTH-04):
//   - The guild code is a STATIC, REUSABLE BEARER TOKEN — treat it like an API
//     token. The watcher must present the PLAINTEXT code as the Bearer value on
//     every POST to /api/v1/ingest, so it is stored as plaintext at rest; file
//     permissions (0600 file, 0700 dir, per-user $HOME) are the at-rest control.
//   - It is NEVER written to config.json (internal/config forbids a secret
//     field), NEVER world-readable (0600 enforced via an atomic fresh-file
//     write — see Store), and NEVER logged (this file imports no logger).
package credstore

import (
	"os"
	"path/filepath"
	"strings"
)

// storePath returns the on-disk location of the guild-code file:
// $XDG_CONFIG_HOME/squirebot/guild_code (default ~/.config/squirebot/guild_code).
// os.UserConfigDir() resolves $XDG_CONFIG_HOME or ~/.config on Unix. The code
// lives in a DEDICATED file (not inside config.json) so Delete is a plain
// os.Remove and the secret never touches the non-secret config struct.
func storePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "squirebot", "guild_code"), nil
}

// Store persists the plaintext guild code to the 0600 file via an atomic
// tmp+rename (mirroring internal/config's atomic write). Writing to a FRESH
// temp file then renaming over the destination guarantees mode 0600 even if a
// guild_code file already existed with looser perms — os.WriteFile does NOT
// tighten an existing file's mode (RESEARCH Pitfall 4 / T-25-05).
//
// SECURITY: the code is NEVER written to config.json and NEVER logged.
func Store(code string) error {
	p, err := storePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, []byte(code), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// Read returns the trimmed guild code. A missing file returns a non-nil error
// (the os.ReadFile ENOENT) — the caller (app.RunApp) treats err != nil ||
// code == "" as "first run / needs onboarding".
func Read() (string, error) {
	p, err := storePath()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// Delete removes the guild-code file. It returns the not-found error when the
// file is absent; callers wanting idempotent teardown (--uninstall, re-onboard)
// ignore not-found.
func Delete() error {
	p, err := storePath()
	if err != nil {
		return err
	}
	return os.Remove(p)
}
