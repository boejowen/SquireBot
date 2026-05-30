// Package config persists non-secret SquireBot settings to
// %LOCALAPPDATA%\SquireBot\config.json.
//
// SECURITY: NEVER add a refresh_token (or any OAuth credential) field to
// Config. Refresh tokens live in Windows Credential Manager only — see
// AUTH-04 in REQUIREMENTS.md and the "DPAPI via wincred" rule in CLAUDE.md.
package config

import (
	"bytes"
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
	Version                 int               `json:"version"`                          // schema version, =1
	EQFolder                string            `json:"eq_folder"`                        // Phase 1 single-folder; preserved for back-compat
	EQFolders               []string          `json:"eq_folders,omitempty"`             // Plan 02-02 WATCH-03 multi-folder
	BackendBaseURL          string            `json:"backend_base_url,omitempty"`       // Phase 13 WATCH-08: overrides the hardcoded https://api.squirebot.quest fallback (build_constants.go); blank = use the default. Advanced/self-host only.
	LastKnownInventoryMtime map[string]string `json:"last_known_inventory_mtime"`       // Plan 02-02 WATCH-09 catch-up: per-char inventory mtime
	LastKnownSpellbookMtime map[string]string `json:"last_known_spellbook_mtime"`       // Plan 02-02 WATCH-09 catch-up: per-char spellbook mtime
	LogLevel                string            `json:"log_level"`                        // "info" default
	PendingUpdateVersion    string            `json:"pending_update_version,omitempty"` // Plan 02-06 OPS-04 informational; the .new file presence is the SOURCE OF TRUTH for whether a swap will happen on next launch — this field is for diagnostics only

	// Phase 13 (WATCH-11) NOTE: the v1 fields SpreadsheetID (`spreadsheet_id`)
	// and GoogleEmail (`google_email`) were REMOVED — the watcher no longer
	// targets a Google Sheet. A v1 config.json carrying those keys still Loads
	// cleanly (encoding/json ignores unknown keys) and the next Save() drops
	// them from disk; app.MigrateFromV1 additionally deletes the stale
	// SquireBot:<google-email> wincred entry on first launch.
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
	// CONFIG-01 (Plan 09-03): strip a leading UTF-8 BOM. Notepad and PowerShell 5.1
	// `Set-Content -Encoding utf8` both write a BOM by default; encoding/json does
	// not auto-strip it and would reject the file with "invalid character 'ï'".
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
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
	// Plan 02-02 WATCH-03 back-compat: Phase 1 stored a single eq_folder; Phase 2
	// uses eq_folders. If only eq_folder is set, migrate it forward into
	// eq_folders. Preserve eq_folder for any tooling still reading it (delete
	// in a later phase). When both are present, eq_folders wins.
	if len(c.EQFolders) == 0 && c.EQFolder != "" {
		c.EQFolders = []string{c.EQFolder}
	}
	if c.LastKnownInventoryMtime == nil {
		c.LastKnownInventoryMtime = make(map[string]string)
	}
	if c.LastKnownSpellbookMtime == nil {
		c.LastKnownSpellbookMtime = make(map[string]string)
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
