package update

// Tests for the latest.json Manifest schema, Fetch, and IsNewer
// (Plan 02-06 Task 1). Seven behaviours:
//
//   1. Manifest struct unmarshals the canonical Phase 2 release.yml-emitted
//      JSON shape correctly (version + installer_url + installer_sha256 +
//      binary_url + binary_sha256 + released_at).
//   2. FetchFromURL against an httptest.Server returning a valid manifest
//      JSON returns the parsed struct.
//   3. FetchFromURL on a 404 returns a recognizable wrapping error (not a
//      panic).
//   4. IsNewer("1.2.3", "1.2.4") true; IsNewer("1.2.4", "1.2.3") false;
//      IsNewer("1.2.3", "1.2.3") false.
//   5. IsNewer minor + major bump cases:
//      ("1.2.3", "1.3.0") true; ("0.1.0", "1.0.0") true.
//   6. IsNewer with malformed input ("abc", "1.2.3") returns false defensively
//      (never panic, never claim newer on a parse failure).
//   7. FetchFromURL with a context whose deadline elapses returns the
//      ctx.Err() (DeadlineExceeded).

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const sampleManifestJSON = `{
  "version": "1.2.3",
  "installer_url": "https://github.com/boejowen/SquireBot/releases/download/v1.2.3/SquireBot-Setup-1.2.3.exe",
  "installer_sha256": "abc123def456",
  "binary_url": "https://github.com/boejowen/SquireBot/releases/download/v1.2.3/squirebot.exe",
  "binary_sha256": "fed987cba654",
  "released_at": "2026-05-02T10:00:00Z"
}`

// Test 1: Manifest struct unmarshals the canonical JSON shape correctly.
func TestManifest_UnmarshalsCanonicalSchema(t *testing.T) {
	var m Manifest
	if err := json.Unmarshal([]byte(sampleManifestJSON), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.Version != "1.2.3" {
		t.Errorf("Version = %q, want 1.2.3", m.Version)
	}
	if !strings.HasSuffix(m.InstallerURL, "SquireBot-Setup-1.2.3.exe") {
		t.Errorf("InstallerURL = %q", m.InstallerURL)
	}
	if m.InstallerSHA256 != "abc123def456" {
		t.Errorf("InstallerSHA256 = %q, want abc123def456", m.InstallerSHA256)
	}
	if !strings.HasSuffix(m.BinaryURL, "squirebot.exe") {
		t.Errorf("BinaryURL = %q", m.BinaryURL)
	}
	if m.BinarySHA256 != "fed987cba654" {
		t.Errorf("BinarySHA256 = %q, want fed987cba654", m.BinarySHA256)
	}
	if m.ReleasedAt != "2026-05-02T10:00:00Z" {
		t.Errorf("ReleasedAt = %q", m.ReleasedAt)
	}
}

// Test 2: FetchFromURL against an httptest.Server returning a valid manifest.
func TestFetchFromURL_ParsesValidManifest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleManifestJSON))
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	m, err := FetchFromURL(ctx, srv.URL)
	if err != nil {
		t.Fatalf("FetchFromURL: %v", err)
	}
	if m.Version != "1.2.3" {
		t.Errorf("Version = %q, want 1.2.3", m.Version)
	}
	if m.BinarySHA256 != "fed987cba654" {
		t.Errorf("BinarySHA256 = %q, want fed987cba654", m.BinarySHA256)
	}
}

// Test 3: 404 returns a wrapping error (not a panic).
func TestFetchFromURL_404ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	_, err := FetchFromURL(ctx, srv.URL)
	if err == nil {
		t.Fatal("FetchFromURL on 404 returned nil, want error")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error %q does not mention 404", err.Error())
	}
}

// Test 4: IsNewer 3-part patch comparison.
func TestIsNewer_PatchComparisons(t *testing.T) {
	cases := []struct {
		current, manifest string
		want              bool
	}{
		{"1.2.3", "1.2.4", true},
		{"1.2.4", "1.2.3", false},
		{"1.2.3", "1.2.3", false},
	}
	for _, c := range cases {
		if got := IsNewer(c.current, c.manifest); got != c.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", c.current, c.manifest, got, c.want)
		}
	}
}

// Test 5: IsNewer minor + major bump cases.
func TestIsNewer_MinorAndMajorBumps(t *testing.T) {
	cases := []struct {
		current, manifest string
		want              bool
	}{
		{"1.2.3", "1.3.0", true},
		{"0.1.0", "1.0.0", true},
		{"1.3.0", "1.2.9", false},
		{"1.0.0", "0.99.99", false},
	}
	for _, c := range cases {
		if got := IsNewer(c.current, c.manifest); got != c.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", c.current, c.manifest, got, c.want)
		}
	}
}

// Test 6: IsNewer defensive on malformed input.
func TestIsNewer_MalformedInputReturnsFalse(t *testing.T) {
	cases := []struct {
		current, manifest string
	}{
		{"abc", "1.2.3"},
		{"1.2.3", "abc"},
		{"", ""},
		{"1.2", "1.2.3"},     // 2-part current
		{"1.2.3", "1.2"},     // 2-part manifest
		{"v1.2.3", "v1.2.4"}, // accepted: leading v stripped — still both 3-part numerics
	}
	for _, c := range cases {
		// First five should return false; last (v1.2.3 → v1.2.4) should return true
		// (we strip the leading v prefix in parseVersion). Test the defensive
		// sub-cases (the malformed ones) here.
		if c.current == "v1.2.3" {
			if got := IsNewer(c.current, c.manifest); got != true {
				t.Errorf("IsNewer(%q, %q) = %v, want true (v-prefix should be stripped)", c.current, c.manifest, got)
			}
			continue
		}
		if got := IsNewer(c.current, c.manifest); got != false {
			t.Errorf("IsNewer(%q, %q) = %v, want false (malformed input)", c.current, c.manifest, got)
		}
	}
}

// Test 8 (999.22): IsNewer is SemVer-pre-release-aware. A version WITH a
// pre-release tail sorts BELOW the same version WITHOUT one (SemVer §11), so a
// final v2.0.0 IS newer than its own rc, and a final must NOT "downgrade" to an
// rc. This is the watcher-side twin of the server's ingest.IsOlder (deliberate
// separate copies; identical direction). Load-bearing for the P16 coordinated
// self-update flip (a pre-release safety manifest must rank correctly).
func TestIsNewer_SemVerPreRelease(t *testing.T) {
	cases := []struct {
		name              string
		current, manifest string
		want              bool
	}{
		// Core comparison (unchanged from the strict 3-part behaviour).
		{"older-to-newer-major", "1.0.0", "2.0.0", true},
		{"equal-finals", "2.0.0", "2.0.0", false},
		{"newer-to-older-patch", "2.0.1", "2.0.0", false},
		{"v-prefix-stripped-both", "v1.0.0", "v2.0.0", true},
		// Pre-release ranks BELOW its final.
		{"rc-updates-to-final", "2.0.0-rc1", "2.0.0", true},
		{"final-does-not-downgrade-to-rc", "2.0.0", "2.0.0-rc1", false},
		// Pre-release vs pre-release (lexical tail compare; sufficient for rcN/betaN).
		{"rc1-to-rc2", "2.0.0-rc1", "2.0.0-rc2", true},
		{"rc2-not-to-rc1", "2.0.0-rc2", "2.0.0-rc1", false},
		{"same-rc-equal", "2.0.0-rc1", "2.0.0-rc1", false},
		// A higher core with a pre-release still beats a lower final
		// (core compares first; the pre-release tail only breaks a core tie).
		{"prerelease-of-higher-core-wins", "2.0.0", "2.1.0-rc1", true},
		{"final-not-newer-than-prerelease-of-higher-core", "2.1.0-rc1", "2.0.0", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsNewer(c.current, c.manifest); got != c.want {
				t.Errorf("IsNewer(%q, %q) = %v, want %v", c.current, c.manifest, got, c.want)
			}
		})
	}
}

// Test 9 (999.22): the defensive "false on any parse failure" contract is
// preserved. A CORRUPT or unparseable manifest version must NEVER trigger an
// update (a forged/garbage manifest cannot masquerade as newer), and an
// unparseable CURRENT (running) version is conservative — the watcher does not
// auto-update off a version it cannot itself parse. This is the watcher-side
// inversion of the server's fail-CLOSED gate: here we fail-CLOSED on the UPDATE
// decision (never update on doubt), where the server fails-CLOSED on the GATE
// (always reject on doubt).
func TestIsNewer_DefensiveCorruptVersion(t *testing.T) {
	cases := []struct {
		name              string
		current, manifest string
	}{
		{"corrupt-manifest-garbage", "2.0.0", "garbage"},
		{"corrupt-manifest-empty", "2.0.0", ""},
		{"corrupt-manifest-two-part", "2.0.0", "2.0"},
		{"corrupt-manifest-nonnumeric-core", "2.0.0", "2.x.0"},
		{"corrupt-current-garbage", "garbage", "2.0.0"},
		{"corrupt-current-empty", "", "2.0.0"},
		{"corrupt-current-two-part", "2.0", "2.0.0"},
		{"both-corrupt", "nope", "nope"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsNewer(c.current, c.manifest); got != false {
				t.Errorf("IsNewer(%q, %q) = %v, want false (defensive on unparseable version)", c.current, c.manifest, got)
			}
		})
	}
}

// Test 7: FetchFromURL with a context whose deadline has already elapsed
// returns the ctx error (DeadlineExceeded).
func TestFetchFromURL_ContextDeadlineExceeded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than the deadline so the ctx fires first.
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte(sampleManifestJSON))
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	t.Cleanup(cancel)

	_, err := FetchFromURL(ctx, srv.URL)
	if err == nil {
		t.Fatal("FetchFromURL with elapsed deadline returned nil, want error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, want wrapping context.DeadlineExceeded", err)
	}
}
