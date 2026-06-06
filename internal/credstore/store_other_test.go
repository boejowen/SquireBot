//go:build !windows

package credstore

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFileStore_RoundTripAndPerms exercises the Linux 0600-file store (LNX-02 /
// T-25-05). XDG_CONFIG_HOME is redirected to a t.TempDir() so the test touches
// no real ~/.config: Store→Read round-trips the exact input, the on-disk file's
// mode is EXACTLY 0600 (not merely "no group/other" — exactly the owner-rw bits),
// and Read after Delete errors (signals "needs onboarding").
func TestFileStore_RoundTripAndPerms(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	const code = "ABC123-link"

	if err := Store(code); err != nil {
		t.Fatalf("Store: %v", err)
	}

	got, err := Read()
	if err != nil {
		t.Fatalf("Read after Store: %v", err)
	}
	if got != code {
		t.Fatalf("Read = %q, want %q", got, code)
	}

	// The guild_code file must be exactly mode 0600 (never world/group-readable):
	// the bearer token is a static reusable secret at rest (T-25-05).
	p, err := storePath()
	if err != nil {
		t.Fatalf("storePath: %v", err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("Stat guild_code: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("guild_code mode = %o, want 0600", perm)
	}
	// The file lives under <XDG>/squirebot/guild_code.
	if base := filepath.Base(p); base != "guild_code" {
		t.Fatalf("guild_code basename = %q, want %q", base, "guild_code")
	}

	// Delete removes it; a subsequent Read is not-found.
	if err := Delete(); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := Read(); err == nil {
		t.Fatal("Read after Delete = nil error, want not-found (needs onboarding)")
	}
}

// TestFileStore_StoreTightensPreexistingLoosePerms verifies that re-storing over
// an EXISTING guild_code file that somehow has loose (0644) perms results in a
// 0600 file — the atomic tmp+rename path replaces the inode, so os.WriteFile's
// "keep existing perms" pitfall (RESEARCH Pitfall 4) does not apply.
func TestFileStore_StoreTightensPreexistingLoosePerms(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	p, err := storePath()
	if err != nil {
		t.Fatalf("storePath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// Plant a world-readable pre-existing file.
	if err := os.WriteFile(p, []byte("stale"), 0o644); err != nil {
		t.Fatalf("seed loose file: %v", err)
	}

	if err := Store("fresh-code"); err != nil {
		t.Fatalf("Store over loose file: %v", err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("after re-Store, mode = %o, want 0600", perm)
	}
	got, err := Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got != "fresh-code" {
		t.Fatalf("Read = %q, want %q", got, "fresh-code")
	}
}
