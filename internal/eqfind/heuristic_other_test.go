//go:build !windows

package eqfind

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// plantSentinels writes the requested sentinel files into dir (creating dir).
func plantSentinels(t *testing.T, dir string, names ...string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte{0}, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// isolateWineEnv points WINEPREFIX + HOME at tmp so the scan only sees the
// planted tree (no real ~/.wine, no real Steam/Lutris/Bottles prefixes leak in).
func isolateWineEnv(t *testing.T, tmp string) {
	t.Helper()
	t.Setenv("WINEPREFIX", filepath.Join(tmp, ".wine"))
	t.Setenv("HOME", tmp)
}

// TestHeuristicScan_FindsEQUnderWinePrefix plants the sentinel pair under a fake
// $WINEPREFIX/drive_c/Program Files/Project1999 and asserts heuristicScan walks
// it (the WINE walk does NOT prune "Program Files"). LNX-03 / RESEARCH A1.
func TestHeuristicScan_FindsEQUnderWinePrefix(t *testing.T) {
	tmp := t.TempDir()
	isolateWineEnv(t, tmp)

	eqDir := filepath.Join(tmp, ".wine", "drive_c", "Program Files", "Project1999")
	plantSentinels(t, eqDir, "eqgame.exe", "eqclient.ini")

	got := heuristicScan()
	if got != eqDir {
		t.Fatalf("heuristicScan() = %q, want %q", got, eqDir)
	}
}

// TestHeuristicScan_FindsEQUnderSystemRoot overrides systemCandidateRoots to a
// planted tmp tree and asserts the second (system-root) loop in heuristicScan
// finds an out-of-WINE EQLite bundle at tmp/everquest/EQLite (relative depth 2).
// isolateWineEnv stops real WINE/system roots from leaking so the planted root
// is the only possible hit. T-vgb-01 system-root coverage.
func TestHeuristicScan_FindsEQUnderSystemRoot(t *testing.T) {
	tmp := t.TempDir()
	isolateWineEnv(t, tmp)

	orig := systemCandidateRoots
	systemCandidateRoots = []string{tmp}
	t.Cleanup(func() { systemCandidateRoots = orig })

	eqDir := filepath.Join(tmp, "everquest", "EQLite")
	plantSentinels(t, eqDir, "eqgame.exe", "eqclient.ini")

	got := heuristicScan()
	if got != eqDir {
		t.Fatalf("heuristicScan() = %q, want %q", got, eqDir)
	}
}

// TestHeuristicScan_DecoyNoMatch plants only ONE sentinel (eqgame.exe, no
// eqclient.ini) — ValidateFolder requires BOTH, so the decoy is not matched and
// the scan returns "".
func TestHeuristicScan_DecoyNoMatch(t *testing.T) {
	tmp := t.TempDir()
	isolateWineEnv(t, tmp)

	decoy := filepath.Join(tmp, ".wine", "drive_c", "Games", "FakeEQ")
	plantSentinels(t, decoy, "eqgame.exe") // missing eqclient.ini

	if got := heuristicScan(); got != "" {
		t.Fatalf("heuristicScan() = %q, want \"\" (decoy lacks eqclient.ini)", got)
	}
}

// TestHeuristicScan_EmptyReturnsEmpty asserts an empty (but existing) drive_c
// tree yields "".
func TestHeuristicScan_EmptyReturnsEmpty(t *testing.T) {
	tmp := t.TempDir()
	isolateWineEnv(t, tmp)

	// Create an empty drive_c so the root exists but holds no EQ install.
	if err := os.MkdirAll(filepath.Join(tmp, ".wine", "drive_c"), 0o755); err != nil {
		t.Fatal(err)
	}

	if got := heuristicScan(); got != "" {
		t.Fatalf("heuristicScan() = %q, want \"\" (empty tree)", got)
	}
}

// TestHeuristicScan_RespectsDepthCap plants the sentinels DEEPER than the depth
// cap allows; the walk must prune before reaching them and return "". This
// guards the T-25-07 depth bound.
func TestHeuristicScan_RespectsDepthCap(t *testing.T) {
	tmp := t.TempDir()
	isolateWineEnv(t, tmp)

	// drive_c is depth 0; nest well beyond maxHeuristicDepthOther (5).
	deep := filepath.Join(tmp, ".wine", "drive_c", "a", "b", "c", "d", "e", "f", "g", "EQ")
	plantSentinels(t, deep, "eqgame.exe", "eqclient.ini")

	if got := heuristicScan(); got != "" {
		t.Fatalf("heuristicScan() = %q, want \"\" (sentinels beyond depth cap)", got)
	}
}

// TestWalkWineRoot_AtDepthCapFound is the WR-02 boundary guard (Linux parity
// with heuristic_windows_test.go's TestWalkRoot_AtDepthCapFound): a sentinel
// pair planted at EXACTLY maxHeuristicDepthOther (5) below the root MUST be
// found. Together with TestWalkWineRoot_BeyondDepthCapNotFound this pins the
// `curDepth > maxHeuristicDepthOther` boundary — switching it to `>=` would
// stop discovering depth-5 installs and this test would catch it.
func TestWalkWineRoot_AtDepthCapFound(t *testing.T) {
	root := t.TempDir()
	want := filepath.Join(root, "a", "b", "c", "d", "e") // depth 5
	plantSentinels(t, want, "eqgame.exe", "eqclient.ini")
	if got := walkWineRoot(context.Background(), root); got != want {
		t.Fatalf("walkWineRoot at exactly maxHeuristicDepthOther = %q, want %q", got, want)
	}
}

// TestWalkWineRoot_BeyondDepthCapNotFound is the WR-02 upper boundary: a pair
// one level deeper than the cap (depth 6) is pruned before ValidateFolder runs
// → walkWineRoot returns "".
func TestWalkWineRoot_BeyondDepthCapNotFound(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "a", "b", "c", "d", "e", "f") // depth 6
	plantSentinels(t, deep, "eqgame.exe", "eqclient.ini")
	if got := walkWineRoot(context.Background(), root); got != "" {
		t.Fatalf("walkWineRoot found a pair beyond the depth cap: %q, want \"\"", got)
	}
}

// TestDefaultKnownPaths_WinePrefixDirectHit asserts the no-walk direct-hit layer
// finds an install at $WINEPREFIX/drive_c/Project1999.
func TestDefaultKnownPaths_WinePrefixDirectHit(t *testing.T) {
	tmp := t.TempDir()
	isolateWineEnv(t, tmp)

	eqDir := filepath.Join(tmp, ".wine", "drive_c", "Project1999")
	plantSentinels(t, eqDir, "eqgame.exe", "eqclient.ini")

	if got := defaultKnownPaths(); got != eqDir {
		t.Fatalf("defaultKnownPaths() = %q, want %q", got, eqDir)
	}
}

// TestDefaultKnownPaths_NoneReturnsEmpty asserts "" when no direct-hit subdir
// holds a valid install.
func TestDefaultKnownPaths_NoneReturnsEmpty(t *testing.T) {
	tmp := t.TempDir()
	isolateWineEnv(t, tmp)
	if err := os.MkdirAll(filepath.Join(tmp, ".wine", "drive_c"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := defaultKnownPaths(); got != "" {
		t.Fatalf("defaultKnownPaths() = %q, want \"\"", got)
	}
}
