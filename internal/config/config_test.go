package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withTempConfig redirects pathFn to a per-test temp file and restores it on cleanup.
func withTempConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.json")
	orig := pathFn
	pathFn = func() string { return p }
	t.Cleanup(func() { pathFn = orig })
	return p
}

// TestLoadMissingReturnsZeroValue: Load on a nonexistent file returns a
// zero-value Config (Version=1, LogLevel="info"), NOT an error.
func TestLoadMissingReturnsZeroValue(t *testing.T) {
	withTempConfig(t)

	c, err := Load()
	if err != nil {
		t.Fatalf("Load on missing file should not error; got: %v", err)
	}
	if c == nil {
		t.Fatal("Load returned nil config")
	}
	if c.Version != 1 {
		t.Errorf("Version = %d; want 1", c.Version)
	}
	if c.LogLevel != "info" {
		t.Errorf("LogLevel = %q; want \"info\"", c.LogLevel)
	}
	if c.EQFolder != "" {
		t.Errorf("EQFolder = %q; want empty", c.EQFolder)
	}
}

// TestSaveLoadRoundTrip: Save then Load yields the same config.
func TestSaveLoadRoundTrip(t *testing.T) {
	withTempConfig(t)

	in := &Config{
		Version:                 1,
		EQFolder:                `C:\Project1999`,
		SpreadsheetID:           "abc123",
		GoogleEmail:             "guildie@example.com",
		LastKnownInventoryMtime: map[string]string{"Foo": "2026-04-30T12:00:00Z"},
		LogLevel:                "info",
	}
	if err := in.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	out, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if out.EQFolder != in.EQFolder ||
		out.SpreadsheetID != in.SpreadsheetID ||
		out.GoogleEmail != in.GoogleEmail ||
		out.LogLevel != in.LogLevel ||
		out.Version != in.Version {
		t.Errorf("round-trip mismatch:\n in = %+v\nout = %+v", in, out)
	}
	if got, want := out.LastKnownInventoryMtime["Foo"], in.LastKnownInventoryMtime["Foo"]; got != want {
		t.Errorf("LastKnownInventoryMtime[Foo] = %q; want %q", got, want)
	}
}

// TestSaveDoesNotEmitRefreshToken (T-01-02 / AUTH-04 regression guard):
// the marshalled JSON MUST NOT contain the substring "refresh_token", even if
// future fields are added accidentally.
func TestSaveDoesNotEmitRefreshToken(t *testing.T) {
	p := withTempConfig(t)

	c := &Config{
		Version:       1,
		EQFolder:      `C:\Project1999`,
		SpreadsheetID: "abc123",
		GoogleEmail:   "guildie@example.com",
		LogLevel:      "info",
	}
	if err := c.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	body := strings.ToLower(string(data))
	for _, banned := range []string{"refresh_token", "refresh-token", "refreshtoken", "access_token"} {
		if strings.Contains(body, banned) {
			t.Errorf("config JSON contains forbidden substring %q (AUTH-04 violation): %s", banned, data)
		}
	}
}

// TestAtomicSaveLeavesNoTmp: after Save returns, no <path>.tmp leftover exists.
func TestAtomicSaveLeavesNoTmp(t *testing.T) {
	p := withTempConfig(t)
	c := &Config{Version: 1, LogLevel: "info"}
	if err := c.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(p + ".tmp"); err == nil {
		t.Errorf("temp file still exists at %s; rename should have moved it", p+".tmp")
	}
}

// ---------------------------------------------------------------------------
// Plan 02-02 Task 4 tests: EQFolders + LastKnownSpellbookMtime + back-compat.
// ---------------------------------------------------------------------------

// TestLoad_Phase1ConfigBackCompat: a config.json with `eq_folder` but no
// `eq_folders` (the Phase 1 shape) loads forward into a Config whose
// EQFolders is a single-element slice copied from EQFolder.
func TestLoad_Phase1ConfigBackCompat(t *testing.T) {
	p := withTempConfig(t)
	body := `{
  "version": 1,
  "eq_folder": "C:\\P99",
  "spreadsheet_id": "abc",
  "google_email": "g@example.com",
  "log_level": "info"
}`
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(c.EQFolders) != 1 || c.EQFolders[0] != `C:\P99` {
		t.Errorf("EQFolders = %#v, want [C:\\P99] migrated from eq_folder", c.EQFolders)
	}
	if c.EQFolder != `C:\P99` {
		t.Errorf("EQFolder preserved unchanged for back-compat; got %q", c.EQFolder)
	}
}

// TestLoad_Phase2ConfigEQFoldersOnly: a config.json with `eq_folders` but no
// `eq_folder` → EQFolders is the loaded slice; EQFolder stays empty.
func TestLoad_Phase2ConfigEQFoldersOnly(t *testing.T) {
	p := withTempConfig(t)
	body := `{
  "version": 1,
  "eq_folders": ["C:\\EQ1", "C:\\EQ2"],
  "spreadsheet_id": "abc",
  "google_email": "g@example.com",
  "log_level": "info"
}`
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(c.EQFolders) != 2 || c.EQFolders[0] != `C:\EQ1` || c.EQFolders[1] != `C:\EQ2` {
		t.Errorf("EQFolders = %#v, want [C:\\EQ1, C:\\EQ2]", c.EQFolders)
	}
	if c.EQFolder != "" {
		t.Errorf("EQFolder = %q, want empty for Phase 2 config", c.EQFolder)
	}
}

// TestLoad_BothFieldsPresent: when both `eq_folder` and `eq_folders` are set,
// EQFolders takes precedence; EQFolder is preserved unchanged.
func TestLoad_BothFieldsPresent(t *testing.T) {
	p := withTempConfig(t)
	body := `{
  "version": 1,
  "eq_folder": "C:\\Old",
  "eq_folders": ["C:\\New1", "C:\\New2"],
  "log_level": "info"
}`
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(c.EQFolders) != 2 || c.EQFolders[0] != `C:\New1` {
		t.Errorf("EQFolders should not be overwritten by eq_folder back-compat; got %#v", c.EQFolders)
	}
	if c.EQFolder != `C:\Old` {
		t.Errorf("EQFolder = %q, want preserved C:\\Old", c.EQFolder)
	}
}

// TestSaveLoad_BothFieldsPersist: round-tripping a config with both EQFolder
// and EQFolders writes both keys to JSON.
func TestSaveLoad_BothFieldsPersist(t *testing.T) {
	p := withTempConfig(t)
	in := &Config{
		Version:                  1,
		EQFolder:                 `C:\Project1999`,
		EQFolders:                []string{`C:\EQ1`, `C:\EQ2`},
		SpreadsheetID:            "abc",
		GoogleEmail:              "g@example.com",
		LastKnownInventoryMtime:  map[string]string{"Foo": "2026-04-30T12:00:00Z"},
		LastKnownSpellbookMtime:  map[string]string{"Foo": "2026-05-01T12:00:00Z"},
		LogLevel:                 "info",
	}
	if err := in.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	body, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got := string(body)
	if !strings.Contains(got, `"eq_folder"`) {
		t.Errorf("expected JSON to contain \"eq_folder\" key; got: %s", got)
	}
	if !strings.Contains(got, `"eq_folders"`) {
		t.Errorf("expected JSON to contain \"eq_folders\" key; got: %s", got)
	}
	if !strings.Contains(got, `"last_known_spellbook_mtime"`) {
		t.Errorf("expected JSON to contain \"last_known_spellbook_mtime\" key; got: %s", got)
	}
	out, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if out.LastKnownSpellbookMtime["Foo"] != "2026-05-01T12:00:00Z" {
		t.Errorf("LastKnownSpellbookMtime[Foo] = %q; want 2026-05-01T12:00:00Z", out.LastKnownSpellbookMtime["Foo"])
	}
}

// TestLoad_MissingMtimeMapsAreInitialized: empty/missing maps become non-nil
// after Load so callers can write to them without nil-checks.
func TestLoad_MissingMtimeMapsAreInitialized(t *testing.T) {
	p := withTempConfig(t)
	body := `{"version": 1, "eq_folder": "C:\\P99", "log_level": "info"}`
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.LastKnownInventoryMtime == nil {
		t.Error("LastKnownInventoryMtime is nil; expected initialized empty map")
	}
	if c.LastKnownSpellbookMtime == nil {
		t.Error("LastKnownSpellbookMtime is nil; expected initialized empty map")
	}
	// Verify we can write to it without panic.
	c.LastKnownSpellbookMtime["Bar"] = "2026-05-01T12:00:00Z"
	if c.LastKnownSpellbookMtime["Bar"] != "2026-05-01T12:00:00Z" {
		t.Errorf("write to map failed")
	}
}

// TestPathPointsUnderLOCALAPPDATA confirms the default path resolver uses
// %LOCALAPPDATA% (not hardcoded). This is a smoke test of pathFn=defaultPath.
func TestPathPointsUnderLOCALAPPDATA(t *testing.T) {
	t.Setenv("LOCALAPPDATA", `C:\Users\Test\AppData\Local`)
	orig := pathFn
	pathFn = defaultPath
	defer func() { pathFn = orig }()

	got := Path()
	want := filepath.Join(`C:\Users\Test\AppData\Local`, "SquireBot", "config.json")
	if got != want {
		t.Errorf("Path() = %q; want %q", got, want)
	}
}
