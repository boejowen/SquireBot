//go:build windows

package eqfind

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// plantEQAt creates root/sub.../{eqgame.exe,eqclient.ini} and returns the leaf
// dir holding the sentinel pair. Mirrors discover_test.go:makeFakeEQDir but lets
// the caller bury the install N levels deep to drive walkRoot's depth cap + prune.
func plantEQAt(t *testing.T, root string, sub ...string) string {
	t.Helper()
	leaf := filepath.Join(append([]string{root}, sub...)...)
	if err := os.MkdirAll(leaf, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, fname := range []string{"eqgame.exe", "eqclient.ini"} {
		if err := os.WriteFile(filepath.Join(leaf, fname), []byte{0}, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return leaf
}

// TestWalkRoot_FindsSentinelPairAtDepth: a pair at depths 1, 2, and 3 (all within
// maxHeuristicDepth=5) is found.
func TestWalkRoot_FindsSentinelPairAtDepth(t *testing.T) {
	cases := [][]string{
		{"Depth1"},
		{"Games", "Depth2"},
		{"Games", "Sub", "Depth3"},
	}
	for _, sub := range cases {
		root := t.TempDir()
		want := plantEQAt(t, root, sub...)
		got := walkRoot(context.Background(), root)
		if got != want {
			t.Errorf("walkRoot(depth %d) = %q, want %q", len(sub), got, want)
		}
	}
}

// TestWalkRoot_BeyondDepthCapNotFound: a pair buried deeper than maxHeuristicDepth
// is pruned by the depth cap → walkRoot returns "".
func TestWalkRoot_BeyondDepthCapNotFound(t *testing.T) {
	root := t.TempDir()
	// 6 levels below root → curDepth 6 > maxHeuristicDepth (5); the holding dir is
	// SkipDir'd before ValidateFolder runs.
	plantEQAt(t, root, "a", "b", "c", "d", "e", "f")
	if got := walkRoot(context.Background(), root); got != "" {
		t.Errorf("walkRoot found a pair beyond the depth cap: %q, want \"\"", got)
	}
}

// TestWalkRoot_PrunedDirNotFound: a full pair inside a pruned dir name
// (node_modules) is never matched — the subtree is SkipDir'd.
func TestWalkRoot_PrunedDirNotFound(t *testing.T) {
	root := t.TempDir()
	plantEQAt(t, root, "node_modules", "EverQuest")
	if got := walkRoot(context.Background(), root); got != "" {
		t.Errorf("walkRoot matched inside a pruned dir: %q, want \"\"", got)
	}
}

// TestWalkRoot_DecoyMissingFileIgnoredRealPairFound: a decoy dir with ONLY
// eqgame.exe (no eqclient.ini) is not matched; the real complete pair elsewhere is
// returned.
func TestWalkRoot_DecoyMissingFileIgnoredRealPairFound(t *testing.T) {
	root := t.TempDir()

	// Decoy: only eqgame.exe, missing eqclient.ini → ValidateFolder fails here.
	decoy := filepath.Join(root, "Decoy")
	if err := os.MkdirAll(decoy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(decoy, "eqgame.exe"), []byte{0}, 0o644); err != nil {
		t.Fatal(err)
	}

	// Real complete pair elsewhere.
	want := plantEQAt(t, root, "RealEQ")

	got := walkRoot(context.Background(), root)
	if got != want {
		t.Errorf("walkRoot = %q, want the real pair %q (decoy must be ignored)", got, want)
	}
}

// TestWalkRoot_EmptyTreeReturnsEmpty: no sentinels anywhere → "".
func TestWalkRoot_EmptyTreeReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "just", "some", "dirs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := walkRoot(context.Background(), root); got != "" {
		t.Errorf("walkRoot on an empty tree = %q, want \"\"", got)
	}
}
