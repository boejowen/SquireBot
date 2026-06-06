//go:build !windows

package eqfind

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Phase 25 / LNX-03 (RESEARCH §2): the Linux heuristic scan walks common WINE /
// Lutris / Bottles / Proton prefix drive_c roots for the eqgame.exe +
// eqclient.ini sentinel pair. It mirrors the Windows heuristic
// (heuristic_windows.go): depth cap (5), 30s context timeout, no symlink
// following (filepath.WalkDir does not follow symlinks), prune list, first
// ValidateFolder match wins. The scan is GENEROUS (many roots) but BOUNDED
// (depth + timeout + no-symlink) — T-25-07 mitigations; an incomplete root list
// degrades to the CLI prompt (D-02/D-04), not a failure.

// heuristicScanTimeoutOther caps total wall-time across all WINE roots.
const heuristicScanTimeoutOther = 30 * time.Second

// maxHeuristicDepthOther bounds recursion per root. EQ-under-WINE typically
// lives within a few levels of drive_c (e.g. drive_c/Program Files/Project1999).
const maxHeuristicDepthOther = 5

// pruneNamesOther are directory base-names to skip inside a WINE drive_c. Unlike
// the native-Windows scan we deliberately do NOT prune "Program Files" /
// "Program Files (x86)" — EQ-under-WINE often installs there (RESEARCH A1). We
// prune only the dirs that cannot host an EQ install (and, in $Recycle.Bin's
// case, are denied).
var pruneNamesOther = map[string]struct{}{
	"users":        {},
	"ProgramData":  {},
	"$Recycle.Bin": {},
	"windows":      {},
	"Windows":      {},
}

// wineCandidateRoots returns the existing WINE/Lutris/Bottles/Proton drive_c
// roots to walk, in priority order (RESEARCH §2). Non-existent roots are
// skipped; glob (`*`) entries are expanded with filepath.Glob.
func wineCandidateRoots() []string {
	home := os.Getenv("HOME")
	join := func(parts ...string) string { return filepath.Join(parts...) }

	// Literal (non-glob) roots, highest priority first.
	literal := []string{}
	if wp := os.Getenv("WINEPREFIX"); wp != "" {
		literal = append(literal, join(wp, "drive_c"))
	}
	if home != "" {
		literal = append(literal, join(home, ".wine", "drive_c"))
	}

	// Glob roots (expanded below). Skip silently if HOME is unset.
	var globs []string
	if home != "" {
		globs = []string{
			join(home, "Games", "*", "drive_c"),                                              // Lutris default install root
			join(home, ".local", "share", "lutris", "runners", "winegames", "*", "drive_c"),  // Lutris winegames
			join(home, ".var", "app", "net.lutris.Lutris", "data", "lutris", "*", "drive_c"), // Lutris Flatpak
			join(home, ".local", "share", "bottles", "bottles", "*", "drive_c"),              // Bottles native
			join(home, ".var", "app", "com.usebottles.bottles", "data", "bottles", "bottles", "*", "drive_c"), // Bottles Flatpak
			join(home, ".local", "share", "Steam", "steamapps", "compatdata", "*", "pfx", "drive_c"),          // Steam Proton
			join(home, ".steam", "steam", "steamapps", "compatdata", "*", "pfx", "drive_c"),                   // Steam alt symlink
		}
	}

	out := make([]string, 0, len(literal)+len(globs))
	seen := map[string]struct{}{}
	add := func(p string) {
		if p == "" {
			return
		}
		if _, dup := seen[p]; dup {
			return
		}
		if fi, err := os.Stat(p); err != nil || !fi.IsDir() {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}

	for _, p := range literal {
		add(p)
	}
	for _, g := range globs {
		matches, err := filepath.Glob(g)
		if err != nil {
			continue
		}
		for _, m := range matches {
			add(m)
		}
	}
	return out
}

// heuristicScan walks each existing WINE-prefix drive_c root for a folder
// satisfying ValidateFolder. Returns the first match, or "" if none / timeout.
func heuristicScan() string {
	ctx, cancel := context.WithTimeout(context.Background(), heuristicScanTimeoutOther)
	defer cancel()

	for _, root := range wineCandidateRoots() {
		select {
		case <-ctx.Done():
			return ""
		default:
		}
		if got := walkWineRoot(ctx, root); got != "" {
			return got
		}
	}
	return ""
}

// walkWineRoot is a single drive_c walk with depth limit and prune list. It does
// NOT follow symlinks (filepath.WalkDir's default) — WINE prefixes are full of
// symlinks into the host fs (T-25-07).
func walkWineRoot(ctx context.Context, root string) string {
	rootDepth := strings.Count(filepath.Clean(root), string(os.PathSeparator))
	var found string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		select {
		case <-ctx.Done():
			return filepath.SkipAll
		default:
		}
		if err != nil {
			// Permission-denied / unreadable dir: prune that subtree.
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		// Depth cap.
		curDepth := strings.Count(filepath.Clean(path), string(os.PathSeparator)) - rootDepth
		if curDepth > maxHeuristicDepthOther {
			return filepath.SkipDir
		}
		// Prune (but keep Program Files — EQ-under-WINE installs there).
		if _, prune := pruneNamesOther[filepath.Base(path)]; prune {
			return filepath.SkipDir
		}
		// Validate: both sentinels present.
		if ValidateFolder(path) == nil {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	return found
}
