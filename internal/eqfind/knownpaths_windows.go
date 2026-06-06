//go:build windows

package eqfind

import (
	"os"
	"path/filepath"
)

// defaultKnownPaths probes a curated list of common Windows EQ install
// locations. First-match wins; ValidateFolder is the gate. (Moved verbatim out
// of discover.go in Phase 25 so the Linux path can supply its own WINE-prefix
// list without C:\ literals — the bodies are byte-equivalent to the original.)
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
