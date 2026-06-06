package main

import "testing"

// TestStatusConfigured pins the CR-01 exit-code contract for `--status`:
// the watcher is "configured" (runStatus → true → os.Exit(0)) ONLY when a
// guild code is present AND at least one EQ folder is set. Any unconfigured
// combination must report false so the process exits non-zero and install.sh
// runs first-time `--setup` on a fresh box.
func TestStatusConfigured(t *testing.T) {
	cases := []struct {
		name          string
		codeConfigured bool
		eqFolderCount int
		want          bool
	}{
		{"code+folder → configured", true, 1, true},
		{"code+multiple folders → configured", true, 3, true},
		{"code but no folder → unconfigured", true, 0, false},
		{"folder but no code → unconfigured", false, 1, false},
		{"neither → unconfigured", false, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := statusConfigured(tc.codeConfigured, tc.eqFolderCount); got != tc.want {
				t.Fatalf("statusConfigured(code=%v, folders=%d) = %v, want %v",
					tc.codeConfigured, tc.eqFolderCount, got, tc.want)
			}
		})
	}
}
