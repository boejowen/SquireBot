package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/boejowen/SquireBot/internal/config"
)

// withTempLOCALAPPDATA redirects config.Path() (which resolves
// %LOCALAPPDATA%\SquireBot\config.json) under a per-test temp dir, and returns
// the resolved config.json path. This is the cross-package seam the app-side
// migration test uses without reaching into config's unexported pathFn.
func withTempLOCALAPPDATA(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)
	return filepath.Join(dir, "SquireBot", "config.json")
}

// seedV1Config writes a raw v1.x config.json (the pre-Phase-13 shape, carrying
// google_email + spreadsheet_id alongside the surviving fields) to p.
func seedV1Config(t *testing.T, p, email, spreadsheet string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := `{
  "version": 1,
  "eq_folder": "C:\\P99",
  "eq_folders": ["C:\\P99", "C:\\P99-Box2"],
  "spreadsheet_id": "` + spreadsheet + `",
  "google_email": "` + email + `",
  "last_known_inventory_mtime": {"Foo": "2026-04-30T12:00:00Z"},
  "last_known_spellbook_mtime": {"Foo": "2026-05-01T12:00:00Z"},
  "log_level": "info"
}`
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// TestMigrateFromV1_DropsGoogleKeysPreservesRest is the WATCH-11 contract: after
// MigrateFromV1, the re-read config.json carries NEITHER google_email NOR
// spreadsheet_id, while EQFolders + the mtime maps are preserved untouched
// (RESEARCH Pitfall 4 — the migration must NOT wipe the EQ folder or mtime).
func TestMigrateFromV1_DropsGoogleKeysPreservesRest(t *testing.T) {
	p := withTempLOCALAPPDATA(t)
	seedV1Config(t, p, "guildie@example.com", "SHEET123")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := MigrateFromV1(cfg); err != nil {
		t.Fatalf("MigrateFromV1: %v", err)
	}

	// The re-read JSON must have dropped both Google keys.
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	body := string(raw)
	for _, banned := range []string{"google_email", "spreadsheet_id"} {
		if strings.Contains(body, banned) {
			t.Errorf("migrated config.json still contains %q: %s", banned, body)
		}
	}

	// EQFolders + mtime maps preserved (in-memory cfg AND on-disk re-read).
	var disk struct {
		EQFolders   []string          `json:"eq_folders"`
		InvMtime    map[string]string `json:"last_known_inventory_mtime"`
		SpbMtime    map[string]string `json:"last_known_spellbook_mtime"`
		EQFolderOne string            `json:"eq_folder"`
	}
	if err := json.Unmarshal(raw, &disk); err != nil {
		t.Fatalf("unmarshal re-read: %v", err)
	}
	if len(disk.EQFolders) != 2 || disk.EQFolders[0] != `C:\P99` || disk.EQFolders[1] != `C:\P99-Box2` {
		t.Errorf("EQFolders not preserved: %#v", disk.EQFolders)
	}
	if disk.InvMtime["Foo"] != "2026-04-30T12:00:00Z" {
		t.Errorf("inventory mtime not preserved: %#v", disk.InvMtime)
	}
	if disk.SpbMtime["Foo"] != "2026-05-01T12:00:00Z" {
		t.Errorf("spellbook mtime not preserved: %#v", disk.SpbMtime)
	}
	if cfg.EQFolder != `C:\P99` {
		t.Errorf("in-memory EQFolder not preserved: %q", cfg.EQFolder)
	}
}

// TestMigrateFromV1_Idempotent: a second MigrateFromV1 on an already-migrated
// config (no Google keys present) is a no-op and does NOT error. The raw read
// finds both fields empty → the idempotency sentinel returns nil.
func TestMigrateFromV1_Idempotent(t *testing.T) {
	p := withTempLOCALAPPDATA(t)
	seedV1Config(t, p, "guildie@example.com", "SHEET123")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := MigrateFromV1(cfg); err != nil {
		t.Fatalf("first MigrateFromV1: %v", err)
	}
	// Capture the post-migration bytes; a second run must not change them.
	first, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read after first: %v", err)
	}

	if err := MigrateFromV1(cfg); err != nil {
		t.Fatalf("second MigrateFromV1 (idempotent) errored: %v", err)
	}
	second, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read after second: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("second migration mutated the config:\n first =%s\nsecond =%s", first, second)
	}
}

// TestMigrateFromV1_NoConfigFile: a fresh install (no config.json on disk) is a
// no-op — MigrateFromV1 must not error when the file does not exist.
func TestMigrateFromV1_NoConfigFile(t *testing.T) {
	withTempLOCALAPPDATA(t) // sets LOCALAPPDATA but writes no file

	// Load on a missing file returns a zero-value config (not an error).
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load on missing file: %v", err)
	}
	if err := MigrateFromV1(cfg); err != nil {
		t.Fatalf("MigrateFromV1 on a fresh install must be a no-op, got: %v", err)
	}
}

// TestMigrateFromV1_AlreadyV2: a config with no Google keys (a clean v2 install)
// migrates to a no-op without touching the EQ data. Guards against the migration
// ever firing on a config that never had v1 state.
func TestMigrateFromV1_AlreadyV2(t *testing.T) {
	p := withTempLOCALAPPDATA(t)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := `{"version":1,"eq_folder":"C:\\P99","backend_base_url":"https://api.squirebot.quest","log_level":"info"}`
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := MigrateFromV1(cfg); err != nil {
		t.Fatalf("MigrateFromV1 on a v2 config: %v", err)
	}
	if cfg.EQFolder != `C:\P99` {
		t.Errorf("EQFolder mangled by no-op migration: %q", cfg.EQFolder)
	}
	if runtime.GOOS != "windows" {
		// On non-Windows CI the wincred delete cannot run; the config-side
		// no-op is what we assert here. (The Windows dev box exercises the
		// real wincred delete path through the v1 test above.)
		_ = body
	}
}
