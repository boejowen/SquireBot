// Package update implements OPS-04: the SquireBot in-app auto-updater.
//
// Three responsibilities, three files:
//
//   - manifest.go (this file) — fetch + parse the canonical latest.json
//     emitted by .github/workflows/release.yml, plus the IsNewer 3-part
//     semver comparison.
//   - check.go — the 24h background goroutine + manual "Check for updates"
//     trigger. Downloads the bare binary, SHA-256-verifies it, stages it
//     as <exepath>.new + <exepath>.expected-sha256.
//   - swap.go — the startup-swap step. Runs BEFORE the main goroutine in
//     cmd/squirebot/main.go. If a staged update exists with a matching
//     SHA-256 sidecar, calls minio/selfupdate.Apply to perform the
//     .new -> live + live -> .old rename dance, then re-execs.
//
// CONTEXT.md (Phase 2, locked):
//   - Startup-swap NEVER in-process (Windows refuses to replace a running
//     .exe; the library handles the rename + hide-instead-of-delete dance).
//   - SHA-256 verification is mandatory at every step (download AND swap).
//   - latest.json schema must match what 02-08's release.yml emits;
//     see .github/workflows/release.yml step "Write latest.json".
//   - Auto-update works for both unsigned and signed binaries (no
//     signing-aware code paths). SignPath OSS integration (Plan 02-09)
//     adds a SIGN step in CI; the auto-updater is signing-agnostic.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Manifest is the exact shape produced by .github/workflows/release.yml's
// "Write latest.json" step (Plan 02-08). Schema is locked at Phase 2;
// any future field additions must be backwards-compatible (new fields
// only, never rename, never delete).
//
// Fields:
//   - Version: bare semver, no leading "v" (e.g. "1.2.3").
//   - InstallerURL / InstallerSHA256: NSIS installer .exe — for new
//     installs and major-upgrade flows where users walk through SmartScreen.
//   - BinaryURL / BinarySHA256: bare squirebot.exe — what the in-app
//     auto-updater downloads + SHA-256-verifies for the startup-swap.
//     Optional in JSON (`omitempty`) only because Phase 1 manifests
//     predate this field; Phase 2 release.yml ALWAYS emits both.
//   - ReleasedAt: RFC3339 — informational, surfaced in tray status.
//
// Phase 1 latest.json shape (no binary_url) still parses cleanly under
// the documented "absent binary_url = installer-only release; skip the
// in-app swap and surface 'manual upgrade available' instead" fallback.
type Manifest struct {
	Version         string `json:"version"`
	InstallerURL    string `json:"installer_url"`
	InstallerSHA256 string `json:"installer_sha256"`
	BinaryURL       string `json:"binary_url,omitempty"`
	BinarySHA256    string `json:"binary_sha256,omitempty"`
	ReleasedAt      string `json:"released_at"`
}

// ManifestURL returns the canonical CDN URL for the latest manifest.
// The "/releases/latest/download/<filename>" path is GitHub's stable
// shortcut to the latest published release's assets.
func ManifestURL(owner, repo string) string {
	return fmt.Sprintf("https://github.com/%s/%s/releases/latest/download/latest.json", owner, repo)
}

// Fetch downloads and parses the latest.json for the given owner/repo via
// GitHub's CDN. 30-second total timeout (well above GitHub CDN's typical
// <500ms response). The body is bounded by io.LimitReader at 4096 bytes
// because the manifest is ~250 bytes — anything larger is malformed.
func Fetch(ctx context.Context, owner, repo string) (Manifest, error) {
	return FetchFromURL(ctx, ManifestURL(owner, repo))
}

// FetchFromURL is the test seam for Fetch. Production callers should use
// Fetch(owner, repo). Tests inject an httptest.Server URL here directly.
func FetchFromURL(ctx context.Context, url string) (Manifest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Manifest{}, fmt.Errorf("manifest req: %w", err)
	}
	req.Header.Set("User-Agent", "SquireBot-AutoUpdate")
	c := &http.Client{Timeout: 30 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return Manifest{}, fmt.Errorf("manifest fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Manifest{}, fmt.Errorf("manifest fetch %s: HTTP %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return Manifest{}, fmt.Errorf("manifest read: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return Manifest{}, fmt.Errorf("manifest parse: %w", err)
	}
	return m, nil
}

// IsNewer returns true if the manifest version is strictly newer than the
// current version under a 3-part numeric semver comparison. Returns false
// on ANY parse failure — defensive: a corrupt manifest must not trigger
// an update. Leading "v" prefix is stripped from both inputs.
func IsNewer(current, manifest string) bool {
	cParts, ok := parseVersion(current)
	if !ok {
		return false
	}
	mParts, ok := parseVersion(manifest)
	if !ok {
		return false
	}
	for i := 0; i < 3; i++ {
		if mParts[i] > cParts[i] {
			return true
		}
		if mParts[i] < cParts[i] {
			return false
		}
	}
	return false // equal
}

// parseVersion splits "1.2.3" (or "v1.2.3") into [1, 2, 3]. Returns
// (zero-value, false) on any failure to keep IsNewer's defensive contract.
func parseVersion(v string) ([3]int, bool) {
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return [3]int{}, false
	}
	var out [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return [3]int{}, false
		}
		out[i] = n
	}
	return out, true
}
