//go:build !windows

package eqfind

import (
	"os"
	"path/filepath"
)

// systemKnownHits are absolute Linux system-path direct-hits for a P99 "EQLite"
// portable bundle installed outside any WINE prefix (real case:
// /opt/everquest/EQLite). Probed via ValidateFolder after the WINE-prefix and
// HOME layers. Package-level so tests can override it (save+restore) to assert
// the override path is reached without touching the real filesystem.
var systemKnownHits = []string{
	"/opt/everquest/EQLite",
	"/opt/EQLite",
	"/usr/local/games/EQLite",
}

// defaultKnownPaths probes a curated list of Linux EQ install direct-hits (the
// fast no-walk first layer, Phase 25 / LNX-03 / RESEARCH §2). First-match wins;
// ValidateFolder is the gate. It covers, in priority order: (1) WINE-prefix
// drive_c subdirs (mirrors the Windows P99/Project1999 list, plus the EQLite
// portable bundle), (2) HOME-relative EQLite bundles, (3) absolute system-path
// EQLite bundles (systemKnownHits). If none hit, Discover falls through to the
// registry no-op then the heuristic walk (heuristic_other.go).
func defaultKnownPaths() string {
	var roots []string
	if wp := os.Getenv("WINEPREFIX"); wp != "" {
		roots = append(roots, filepath.Join(wp, "drive_c"))
	}
	if home := os.Getenv("HOME"); home != "" {
		roots = append(roots, filepath.Join(home, ".wine", "drive_c"))
	}

	// Per-root EQ subfolder candidates (mirror the Windows P99/Project1999 list,
	// including the common "Program Files" install location under WINE, plus the
	// EQLite portable bundle at drive_c root and under an "everquest" folder).
	subdirs := []string{
		"P99",
		"Project1999",
		filepath.Join("Program Files", "Project1999"),
		filepath.Join("Program Files (x86)", "Project1999"),
		"EverQuest",
		filepath.Join("Program Files", "Sony", "EverQuest"),
		"EQLite",
		filepath.Join("everquest", "EQLite"),
	}

	for _, root := range roots {
		for _, sub := range subdirs {
			p := filepath.Join(root, sub)
			if ValidateFolder(p) == nil {
				return p
			}
		}
	}

	// HOME-relative EQLite bundles (portable bundle dropped in the user's home,
	// Desktop, or an everquest/ folder — outside any WINE prefix).
	if home := os.Getenv("HOME"); home != "" {
		homeHits := []string{
			filepath.Join(home, "EQLite"),
			filepath.Join(home, "Desktop", "EQLite"),
			filepath.Join(home, "everquest", "EQLite"),
		}
		for _, p := range homeHits {
			if ValidateFolder(p) == nil {
				return p
			}
		}
	}

	// Absolute system-path EQLite bundles (e.g. /opt/everquest/EQLite).
	for _, p := range systemKnownHits {
		if ValidateFolder(p) == nil {
			return p
		}
	}
	return ""
}
