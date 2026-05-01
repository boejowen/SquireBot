//go:build windows

package eqfind

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// heuristicScanTimeout caps total wall-time for the heuristic scan. The wizard
// MUST NOT hang if the user has many drives or unusual filesystem layouts.
const heuristicScanTimeout = 30 * time.Second

// maxHeuristicDepth bounds recursion to avoid walking deep tree branches.
// EQ installs typically live within 2–3 dir levels of a drive root.
const maxHeuristicDepth = 5

// heuristicScan walks common drive roots looking for a folder that satisfies
// ValidateFolder. Bounded by depth (5), pruned exclusions, and a 30s context
// deadline. Threat T-04-03/T-04-04 mitigations: depth + prune + timeout +
// no-symlink-following (filepath.WalkDir does not follow symlinks by default).
func heuristicScan() string {
	ctx, cancel := context.WithTimeout(context.Background(), heuristicScanTimeout)
	defer cancel()

	roots := candidateDrives()
	for _, root := range roots {
		select {
		case <-ctx.Done():
			return ""
		default:
		}
		if got := walkRoot(ctx, root); got != "" {
			return got
		}
	}
	return ""
}

// candidateDrives enumerates plausible local drive roots. We probe C..E by
// default; mounted disks beyond E: are rare for EQ installs.
func candidateDrives() []string {
	out := []string{}
	for _, letter := range []string{"C", "D", "E"} {
		root := letter + `:\`
		if _, err := os.Stat(root); err == nil {
			out = append(out, root)
		}
	}
	return out
}

// pruneNames are directory base-names to skip. These cannot host an EQ install,
// and walking them is wasteful (or, in $Recycle.Bin's case, denied).
var pruneNames = map[string]struct{}{
	"Windows":            {},
	"Program Files":      {},
	"Program Files (x86)": {},
	"$Recycle.Bin":       {},
	"node_modules":       {},
	"AppData":            {},
	"System Volume Information": {},
	".git":               {},
}

// walkRoot is a single-drive walk with depth limit and prune list.
func walkRoot(ctx context.Context, root string) string {
	rootDepth := strings.Count(filepath.Clean(root), string(os.PathSeparator))
	var found string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		// Honor the deadline.
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
		if curDepth > maxHeuristicDepth {
			return filepath.SkipDir
		}
		// Prune.
		base := filepath.Base(path)
		if _, prune := pruneNames[base]; prune {
			return filepath.SkipDir
		}
		// Validate.
		if ValidateFolder(path) == nil {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	return found
}
