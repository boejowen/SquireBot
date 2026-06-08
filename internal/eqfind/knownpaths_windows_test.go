//go:build windows

package eqfind

import (
	"os"
	"path/filepath"
	"testing"
)

// plantSentinelPair writes the eqgame.exe + eqclient.ini sentinel pair into dir
// (creating dir). Local to the windows-tagged tests — plantSentinels lives in
// the !windows test file and is not in scope here.
func plantSentinelPair(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"eqgame.exe", "eqclient.ini"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte{0}, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestDefaultKnownPaths_UserProfileDesktopEQLiteHit points USERPROFILE at a tmp
// dir, plants an EQLite bundle at %USERPROFILE%\Desktop\EQLite (the maintainer's
// real location), and asserts defaultKnownPaths returns it. The C:\ literals
// ahead of it in the candidate list don't exist on a fresh runner, so the
// Desktop hit is reached. RUNS natively on the Windows host.
func TestDefaultKnownPaths_UserProfileDesktopEQLiteHit(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("USERPROFILE", tmp)

	eqDir := filepath.Join(tmp, "Desktop", "EQLite")
	plantSentinelPair(t, eqDir)

	if got := defaultKnownPaths(); got != eqDir {
		t.Fatalf("defaultKnownPaths() = %q, want %q", got, eqDir)
	}
}
