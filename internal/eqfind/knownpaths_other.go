//go:build !windows

package eqfind

import (
	"os"
	"path/filepath"
)

// defaultKnownPaths probes a curated list of Linux WINE-prefix EQ install
// direct-hits (the fast no-walk first layer, Phase 25 / LNX-03 / RESEARCH §2).
// First-match wins; ValidateFolder is the gate. These mirror the Windows
// P99 / Project1999 known-paths list, but rooted at the user's WINE prefix
// ($WINEPREFIX or the default ~/.wine) drive_c. If none hit, Discover falls
// through to the registry no-op then the heuristic WINE walk (heuristic_other.go).
func defaultKnownPaths() string {
	var roots []string
	if wp := os.Getenv("WINEPREFIX"); wp != "" {
		roots = append(roots, filepath.Join(wp, "drive_c"))
	}
	if home := os.Getenv("HOME"); home != "" {
		roots = append(roots, filepath.Join(home, ".wine", "drive_c"))
	}

	// Per-root EQ subfolder candidates (mirror the Windows P99/Project1999 list,
	// including the common "Program Files" install location under WINE).
	subdirs := []string{
		"P99",
		"Project1999",
		filepath.Join("Program Files", "Project1999"),
		filepath.Join("Program Files (x86)", "Project1999"),
		"EverQuest",
		filepath.Join("Program Files", "Sony", "EverQuest"),
	}

	for _, root := range roots {
		for _, sub := range subdirs {
			p := filepath.Join(root, sub)
			if ValidateFolder(p) == nil {
				return p
			}
		}
	}
	return ""
}
