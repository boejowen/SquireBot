// Package config persists non-secret SquireBot settings to
// %LOCALAPPDATA%\SquireBot\config.json.
//
// SECURITY: NEVER add a refresh_token (or any OAuth credential) field to
// Config. Refresh tokens live in Windows Credential Manager only — see
// AUTH-04 in REQUIREMENTS.md and the "DPAPI via wincred" rule in CLAUDE.md.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Config is the on-disk shape of %LOCALAPPDATA%\SquireBot\config.json.
//
// SECURITY: NEVER add a refresh_token field. Refresh tokens live in wincred only (AUTH-04).
type Config struct {
	Version                 int               `json:"version"`                    // schema version, =1
	EQFolder                string            `json:"eq_folder"`                  // set by Plan 04 wizard step
	SpreadsheetID           string            `json:"spreadsheet_id"`             // set by Plan 06 picker
	GoogleEmail             string            `json:"google_email"`               // set by Plan 03 OAuth (cached)
	LastKnownInventoryMtime map[string]string `json:"last_known_inventory_mtime"` // Phase 2 will populate; empty in Phase 1
	LogLevel                string            `json:"log_level"`                  // "info" default
}

// pathFn is the directory resolver used to compute the config path.
// Tests may swap this to redirect the file under t.TempDir().
var pathFn = defaultPath

// defaultPath returns %LOCALAPPDATA%\SquireBot\config.json.
func defaultPath() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "SquireBot", "config.json")
}

// Path returns the absolute path of the config file (for diagnostics).
func Path() string {
	return pathFn()
}

// Load reads %LOCALAPPDATA%\SquireBot\config.json.
//
// Returns a zero-value Config (NOT an error) when the file does not exist —
// first-run callers can immediately call Save() without special-casing.
// Returns an error only on parse failure or unexpected I/O error.
func Load() (*Config, error) {
	p := Path()
	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &Config{Version: 1, LogLevel: "info"}, nil
		}
		return nil, fmt.Errorf("config load %s: %w", p, err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("config load %s: %w", p, err)
	}
	if c.Version == 0 {
		c.Version = 1
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	return &c, nil
}

// Save writes c to %LOCALAPPDATA%\SquireBot\config.json atomically:
// it writes to <path>.tmp first, then os.Renames over the destination.
// On Windows NTFS, single-volume Rename is atomic (T-01-03 mitigation).
func (c *Config) Save() error {
	p := Path()
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("config save mkdir %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("config save marshal: %w", err)
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("config save write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, p); err != nil {
		// Windows os.Rename will fail if the destination exists on some older
		// builds; remove and retry once for safety.
		_ = os.Remove(p)
		if err2 := os.Rename(tmp, p); err2 != nil {
			return fmt.Errorf("config save rename %s -> %s: %w", tmp, p, err2)
		}
	}
	return nil
}
