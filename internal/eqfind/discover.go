// Package eqfind locates the user's EverQuest install folder via a four-step
// cascade per CONTEXT.md D-09: (a) prior config (caller's responsibility, not
// in scope here), (b) known paths, (c) registry uninstall keys, (d) heuristic
// recursive scan. The validation predicate (D-10) is "folder contains BOTH
// eqgame.exe AND eqclient.ini." If all three layers fail, Discover returns
// ErrNotFound so Plan 07's wizard can surface the native folder picker.
//
// Threat model (Plan 04 §threat_model):
//   - T-04-02 path traversal via registry-supplied InstallLocation: ValidateFolder
//     stops it (System32 has neither eqgame.exe nor eqclient.ini).
//   - T-04-03 heuristic scan walking entire C:\: bounded depth + pruned exclusions
//     + 30s context timeout; see heuristic_windows.go.
//   - T-04-05 attacker plants sentinel pair: accepted risk (local-only Phase 1
//     watcher; user owns their own filesystem).
package eqfind

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// ErrNotFound is returned by Discover when all four cascade layers fail.
// Plan 07's wizard step-3 catches this and renders the native folder picker.
var ErrNotFound = errors.New("eqfind: no EQ folder found in known paths, registry, or heuristic scan")

// For tests: each layer is a func var so tests can swap implementations
// without needing a real registry or filesystem layout.
var (
	knownPathsProbe = defaultKnownPaths
	registryProbe   = defaultRegistryProbe
	heuristicProbe  = defaultHeuristicScan
)

// Discover runs the four-step cascade per D-09 and returns the first folder
// containing both eqgame.exe AND eqclient.ini per D-10.
// Returns ErrNotFound if all three layers fail.
func Discover() (string, error) {
	if p := knownPathsProbe(); p != "" {
		return p, nil
	}
	if p := registryProbe(); p != "" {
		return p, nil
	}
	if p := heuristicProbe(); p != "" {
		return p, nil
	}
	return "", ErrNotFound
}

// ValidateFolder enforces D-10: folder must contain BOTH eqgame.exe AND eqclient.ini.
// Used by both Discover (during cascade) and Plan 07 (to validate user-picked
// folders — "This folder doesn't look like an EQ install").
func ValidateFolder(dir string) error {
	if dir == "" {
		return errors.New("eqfind: empty path")
	}
	for _, fname := range []string{"eqgame.exe", "eqclient.ini"} {
		p := filepath.Join(dir, fname)
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("eqfind: %s missing in %q (%w)", fname, dir, err)
		}
	}
	return nil
}

// defaultKnownPaths probes a curated list of common EQ install locations.
// First-match wins. ValidateFolder is the gate.
func defaultKnownPaths() string {
	userProfile := os.Getenv("USERPROFILE")
	candidates := []string{
		`C:\P99`,
		`C:\Project1999`,
		`C:\Games\Project1999`,
		`C:\EverQuest`,
		`C:\Games\EverQuest`,
		`C:\Program Files (x86)\Sony\EverQuest`,
	}
	if userProfile != "" {
		candidates = append(candidates,
			filepath.Join(userProfile, "EverQuest"),
			filepath.Join(userProfile, "P99"),
			filepath.Join(userProfile, "Project1999"),
		)
	}
	for _, p := range candidates {
		if ValidateFolder(p) == nil {
			return p
		}
	}
	return ""
}

// defaultRegistryProbe is a no-op on non-Windows; Windows implementation lives
// in registry_windows.go.
func defaultRegistryProbe() string {
	if runtime.GOOS != "windows" {
		return ""
	}
	return scanUninstallKeys()
}

// defaultHeuristicScan is a no-op on non-Windows; Windows implementation lives
// in heuristic_windows.go.
//
// NOTE: the Windows heuristic scan is not unit-tested in CI because it requires
// a real Windows filesystem with realistic structure. It is exercised in Plan 08's
// smoke checkpoint.
func defaultHeuristicScan() string {
	if runtime.GOOS != "windows" {
		return ""
	}
	return heuristicScan()
}
