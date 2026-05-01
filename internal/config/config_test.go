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
