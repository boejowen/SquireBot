//go:build !windows

package eqfind

import (
	"path/filepath"
	"testing"
)

// TestDefaultKnownPaths_WinePrefixEQLiteDirectHit asserts the WINE-prefix
// direct-hit layer finds an EQLite portable bundle at
// $WINEPREFIX/drive_c/EQLite (the new "EQLite" subdir entry). Reuses
// plantSentinels + isolateWineEnv from heuristic_other_test.go (same package +
// build tag).
func TestDefaultKnownPaths_WinePrefixEQLiteDirectHit(t *testing.T) {
	tmp := t.TempDir()
	isolateWineEnv(t, tmp) // WINEPREFIX = tmp/.wine, HOME = tmp

	eqDir := filepath.Join(tmp, ".wine", "drive_c", "EQLite")
	plantSentinels(t, eqDir, "eqgame.exe", "eqclient.ini")

	if got := defaultKnownPaths(); got != eqDir {
		t.Fatalf("defaultKnownPaths() = %q, want %q", got, eqDir)
	}
}

// TestDefaultKnownPaths_HomeDesktopEQLiteHit asserts the HOME-relative layer
// finds an EQLite bundle dropped at ~/Desktop/EQLite (HOME isolated to tmp).
// No WINE-prefix or system hit is planted, so the HOME block is reached.
func TestDefaultKnownPaths_HomeDesktopEQLiteHit(t *testing.T) {
	tmp := t.TempDir()
	isolateWineEnv(t, tmp) // HOME = tmp

	eqDir := filepath.Join(tmp, "Desktop", "EQLite")
	plantSentinels(t, eqDir, "eqgame.exe", "eqclient.ini")

	if got := defaultKnownPaths(); got != eqDir {
		t.Fatalf("defaultKnownPaths() = %q, want %q", got, eqDir)
	}
}

// TestDefaultKnownPaths_SystemKnownHitsOverride overrides systemKnownHits to a
// planted absolute path and asserts the system-path layer (last in the
// first-match-wins order) returns it. isolateWineEnv keeps the WINE + HOME
// layers from leaking a real hit so the override is reached.
func TestDefaultKnownPaths_SystemKnownHitsOverride(t *testing.T) {
	tmp := t.TempDir()
	isolateWineEnv(t, tmp)

	hit := filepath.Join(t.TempDir(), "sys", "EQLite")
	plantSentinels(t, hit, "eqgame.exe", "eqclient.ini")

	orig := systemKnownHits
	systemKnownHits = []string{hit}
	t.Cleanup(func() { systemKnownHits = orig })

	if got := defaultKnownPaths(); got != hit {
		t.Fatalf("defaultKnownPaths() = %q, want %q", got, hit)
	}
}
