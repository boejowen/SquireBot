package buildinfo

import "testing"

// TestUserAgent_Identifying asserts the User-Agent is always identifying +
// contactable regardless of the Version value: it must carry the "SquireBot/"
// product token (identifying) and the GitHub URL (contactable, so a
// community-server operator can attribute + reach the maintainer). These are
// the SC-3 politeness requirement toward the community-run PigParse + P1999
// wiki services; they must hold even on an un-stamped ("dev") build.
func TestUserAgent_Identifying(t *testing.T) {
	ua := UserAgent()

	if !contains(ua, "SquireBot/") {
		t.Errorf("UserAgent() = %q; want it to contain %q (identifying product token)", ua, "SquireBot/")
	}
	if !contains(ua, "github.com/boejowen/SquireBot") {
		t.Errorf("UserAgent() = %q; want it to contain %q (contactable URL)", ua, "github.com/boejowen/SquireBot")
	}
}

// TestUserAgent_IncludesVersion asserts the current Version value is threaded
// into the UA (so an ldflags-stamped build surfaces its version to the server
// operator). Uses whatever Version is set to (default "dev").
func TestUserAgent_IncludesVersion(t *testing.T) {
	ua := UserAgent()
	if !contains(ua, "SquireBot/"+Version) {
		t.Errorf("UserAgent() = %q; want it to contain the current Version %q as %q", ua, Version, "SquireBot/"+Version)
	}
}

// contains is a tiny substring helper (no strings import needed beyond this) so
// the test reads declaratively.
func contains(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
