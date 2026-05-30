package ingest

// version_test.go is the table test for the server-side SemVer-aware version
// compare (IsOlder). It lives in the INTERNAL test package (package ingest, not
// ingest_test) because IsOlder is the package-internal gate primitive that
// handler.go consumes directly — testing it in-package keeps it the single
// coverage for the version-compare truth (its twin is the watcher-side 999.22
// fix in Plan 04; the two are deliberately separate copies that behave
// identically — neither side imports the other's internals).
//
// Every case below maps 1:1 to a bullet in the plan's Task 1 <behavior> block.

import "testing"

func TestIsOlder(t *testing.T) {
	cases := []struct {
		name    string
		present string
		floor   string
		want    bool
	}{
		// Core MAJOR.MINOR.PATCH ordering.
		{"older major", "1.9.9", "2.0.0", true},
		{"equal core", "2.0.0", "2.0.0", false},
		{"newer patch", "2.0.1", "2.0.0", false},
		{"newer minor below-floor major still older", "1.99.99", "2.0.0", true},
		{"newer major", "3.0.0", "2.0.0", false},

		// Leading "v" stripped from both inputs.
		{"v-prefixed present older", "v1.0.0", "2.0.0", true},
		{"v-prefixed both equal", "v2.0.0", "v2.0.0", false},

		// Pre-release sorts BELOW its final release (SemVer §11).
		{"prerelease below final", "2.0.0-rc1", "2.0.0", true},
		{"final above prerelease floor", "2.0.0", "2.0.0-rc1", false},
		{"prerelease ordering rc1<rc2", "2.0.0-rc1", "2.0.0-rc2", true},
		{"prerelease ordering rc2 not < rc1", "2.0.0-rc2", "2.0.0-rc1", false},
		{"equal prerelease not older", "2.0.0-rc1", "2.0.0-rc1", false},

		// Unparseable / empty PRESENT version ⇒ treated as too-old (fail-closed):
		// a watcher that cannot state a real version is below the floor; the gate
		// must not silently pass it (SC-5).
		{"empty present is older", "", "2.0.0", true},
		{"garbage present is older", "garbage", "2.0.0", true},
		{"two-part present is older", "2.0", "2.0.0", true},
		{"non-numeric present is older", "2.x.0", "2.0.0", true},

		// A bad FLOOR (the const WE control) never spuriously rejects a real
		// client — fail-open on OUR misconfig only (never on the client's value).
		{"bad floor never rejects", "1.0.0", "garbage", false},
		{"empty floor never rejects", "1.0.0", "", false},
		{"two-part floor never rejects", "1.0.0", "2.0", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsOlder(tc.present, tc.floor); got != tc.want {
				t.Errorf("IsOlder(%q, %q) = %v, want %v", tc.present, tc.floor, got, tc.want)
			}
		})
	}
}
