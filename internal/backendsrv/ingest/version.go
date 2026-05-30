package ingest

// version.go is the server-side, SemVer-aware version-compare truth behind the
// min-watcher-version 426 gate (SC-5 / CONTEXT D-4). It exports ONE function,
// IsOlder, used by handler.go to reject an ingest from a watcher whose
// watcher_version is below the published floor (minWatcherVersion).
//
// ONE version-compare truth, PER SIDE. This server-side IsOlder and the
// watcher-side SemVer fix (Plan 04 / backlog 999.22, which upgrades
// internal/update/manifest.go::IsNewer/parseVersion) are DELIBERATELY separate
// copies: the watcher binary and the backend binary must not import each other's
// internals (RESEARCH "do not import server↔client internals"). They are kept
// behaviorally identical by sharing this doctrine, not code:
//
//   - Only MAJOR.MINOR.PATCH FINAL tags are ever published (the release workflow
//     stamps a bare semver). Pre-release tags (e.g. "2.0.0-rc1") are a dev-only
//     safety rail; per SemVer §11 a version WITH a pre-release tail sorts BELOW
//     the same version WITHOUT one.
//   - Pre-release identifier comparison here is intentionally simplified to a
//     lexical strings.Compare of the tail. The full SemVer §11 rule (dot-separated
//     identifiers, numeric-vs-alphanumeric precedence) is overkill for our only
//     ever scheme ("rcN" / "betaN") — and the path is a rail, not a hot path.
//
// The two failure modes are ASYMMETRIC by design (the load-bearing inversion vs.
// the watcher's defensive IsNewer, which returns false on ANY parse failure):
//
//   - An unparseable PRESENT version (empty, wrong arity, non-numeric core) is
//     treated as OLDER THAN the floor (fail-CLOSED). A watcher that cannot state
//     a valid version must not slip past the gate (SC-5: a forged/blank-but-
//     nonempty version cannot masquerade as current).
//   - An unparseable FLOOR (the const WE control) makes IsOlder return false
//     (fail-OPEN). If we misconfigure minWatcherVersion we must never reject a
//     real client over our own bug.

import (
	"strconv"
	"strings"
)

// IsOlder reports whether present is strictly older than floor under SemVer-with-
// pre-release ordering. See the file header for the fail-closed (bad present) /
// fail-open (bad floor) asymmetry and the "one truth per side" doctrine.
func IsOlder(present, floor string) bool {
	fCore, fPre, fOK := parseSemver(floor)
	if !fOK {
		// Bad floor = OUR misconfiguration → never reject a real client (fail-open).
		return false
	}
	pCore, pPre, pOK := parseSemver(present)
	if !pOK {
		// Bad/empty/forged present version → treat as below the floor (fail-closed).
		return true
	}

	// Compare the three core ints in order; the first difference decides.
	for i := 0; i < 3; i++ {
		if pCore[i] < fCore[i] {
			return true
		}
		if pCore[i] > fCore[i] {
			return false
		}
	}

	// Cores are EQUAL — apply pre-release precedence (SemVer §11): a version WITH
	// a pre-release tail is older than the same version WITHOUT one.
	pHasPre := pPre != ""
	fHasPre := fPre != ""
	switch {
	case pHasPre && !fHasPre:
		// present is a pre-release of floor's final → older.
		return true
	case !pHasPre && fHasPre:
		// present is the final, floor is a pre-release of it → not older.
		return false
	case pHasPre && fHasPre:
		// Both pre-releases of the same core → lexical compare of the tails
		// (sufficient for our rcN/betaN scheme; see file header).
		return strings.Compare(pPre, fPre) < 0
	default:
		// Equal cores, neither has a tail → not older (equal).
		return false
	}
}

// parseSemver splits "v1.2.3" or "v1.2.3-rc1" into its MAJOR.MINOR.PATCH int core
// and an optional pre-release tail (the substring after the FIRST '-'). It returns
// ok=false if the core is not exactly three numeric parts. A leading "v" is
// stripped first. The tail (if any) is returned verbatim for the caller's
// precedence comparison; it is NOT validated (any non-empty tail counts as a
// pre-release marker).
func parseSemver(v string) (core [3]int, pre string, ok bool) {
	v = strings.TrimPrefix(v, "v")
	if v == "" {
		return [3]int{}, "", false
	}

	// Split off the pre-release tail on the FIRST '-' (so "2.0.0-rc-1" keeps
	// "rc-1" as the tail; the core is the part before the first '-').
	coreStr := v
	if i := strings.IndexByte(v, '-'); i >= 0 {
		coreStr = v[:i]
		pre = v[i+1:]
	}

	parts := strings.Split(coreStr, ".")
	if len(parts) != 3 {
		return [3]int{}, "", false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return [3]int{}, "", false
		}
		core[i] = n
	}
	return core, pre, true
}
