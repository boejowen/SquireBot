package eqfind

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeFakeEQDir creates a tempDir containing the two sentinel files (or omits
// one when `omit` is non-empty). Returns the dir.
func makeFakeEQDir(t *testing.T, omit string) string {
	t.Helper()
	dir := t.TempDir()
	for _, fname := range []string{"eqgame.exe", "eqclient.ini"} {
		if fname == omit {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, fname), []byte{0}, 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// Test 1: ValidateFolder on a folder with both sentinel files returns nil.
func TestValidateFolder_BothFilesPresent(t *testing.T) {
	dir := makeFakeEQDir(t, "")
	if err := ValidateFolder(dir); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

// Test 2: ValidateFolder missing eqgame.exe returns an error mentioning eqgame.exe.
func TestValidateFolder_MissingEQGame(t *testing.T) {
	dir := makeFakeEQDir(t, "eqgame.exe")
	err := ValidateFolder(dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "eqgame.exe") {
		t.Errorf("expected error mentioning eqgame.exe, got %q", err.Error())
	}
}

// Test 3: ValidateFolder missing eqclient.ini returns an error mentioning eqclient.ini.
func TestValidateFolder_MissingEQClient(t *testing.T) {
	dir := makeFakeEQDir(t, "eqclient.ini")
	err := ValidateFolder(dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "eqclient.ini") {
		t.Errorf("expected error mentioning eqclient.ini, got %q", err.Error())
	}
}

// Test 4: ValidateFolder on a non-existent path returns an error.
func TestValidateFolder_NonExistent(t *testing.T) {
	bogus := filepath.Join(t.TempDir(), "no-such-subdir")
	if err := ValidateFolder(bogus); err == nil {
		t.Fatal("expected error for non-existent dir, got nil")
	}
	// Also: empty path returns an error.
	if err := ValidateFolder(""); err == nil {
		t.Fatal("expected error for empty path, got nil")
	}
}

// Test 5: Discover honours the known-paths layer when the probe finds a real folder.
func TestDiscover_KnownPathsLayerHit(t *testing.T) {
	dir := makeFakeEQDir(t, "")
	orig := knownPathsProbe
	knownPathsProbe = func() string { return dir }
	t.Cleanup(func() { knownPathsProbe = orig })

	got, err := Discover()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != dir {
		t.Errorf("got %q, want %q", got, dir)
	}
}

// Test 5b: Discover falls through to registry layer when known paths returns "".
func TestDiscover_RegistryLayerHit(t *testing.T) {
	dir := makeFakeEQDir(t, "")
	origK, origR, origH := knownPathsProbe, registryProbe, heuristicProbe
	knownPathsProbe = func() string { return "" }
	registryProbe = func() string { return dir }
	heuristicProbe = func() string { t.Fatal("heuristic probe should not run when registry hits"); return "" }
	t.Cleanup(func() {
		knownPathsProbe = origK
		registryProbe = origR
		heuristicProbe = origH
	})

	got, err := Discover()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != dir {
		t.Errorf("got %q, want %q", got, dir)
	}
}

// Test 5c: Discover falls through to heuristic layer when knownPaths + registry both miss.
func TestDiscover_HeuristicLayerHit(t *testing.T) {
	dir := makeFakeEQDir(t, "")
	origK, origR, origH := knownPathsProbe, registryProbe, heuristicProbe
	knownPathsProbe = func() string { return "" }
	registryProbe = func() string { return "" }
	heuristicProbe = func() string { return dir }
	t.Cleanup(func() {
		knownPathsProbe = origK
		registryProbe = origR
		heuristicProbe = origH
	})

	got, err := Discover()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != dir {
		t.Errorf("got %q, want %q", got, dir)
	}
}

// Test 6: Discover returns ErrNotFound when all three layers return "".
func TestDiscover_AllLayersMissReturnsErrNotFound(t *testing.T) {
	origK, origR, origH := knownPathsProbe, registryProbe, heuristicProbe
	knownPathsProbe = func() string { return "" }
	registryProbe = func() string { return "" }
	heuristicProbe = func() string { return "" }
	t.Cleanup(func() {
		knownPathsProbe = origK
		registryProbe = origR
		heuristicProbe = origH
	})

	got, err := Discover()
	if got != "" {
		t.Errorf("expected empty path on miss, got %q", got)
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
