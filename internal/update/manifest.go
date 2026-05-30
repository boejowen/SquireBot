// Package update implements OPS-04: the SquireBot in-app auto-updater.
//
// Three responsibilities, three files:
//
//   - manifest.go (this file) — fetch + parse the canonical latest.json
//     emitted by .github/workflows/release.yml, plus the IsNewer
//     SemVer-pre-release-aware version comparison.
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

// IsNewer reports whether the manifest version is strictly newer than the
// current version under SemVer-with-pre-release ordering. It is the gate the
// 24h goroutine + manual "Check for updates" use to decide whether to download
// and stage the manifest's binary (check.go).
//
// Comparison rules:
//   - Leading "v" is stripped from both inputs.
//   - MAJOR.MINOR.PATCH compared numerically; the first differing field decides.
//   - On a core tie, SemVer §11 pre-release precedence applies: a version WITH a
//     pre-release tail is LOWER than the same version WITHOUT one. So a final
//     v2.0.0 IS newer than its own "2.0.0-rc1" (an rc watcher should update to
//     the final), and a final must NOT "downgrade" to a manifest rc. Two
//     pre-releases of the same core compare by a lexical strings.Compare of the
//     tail (sufficient for our only-ever "rcN"/"betaN" scheme; the full §11
//     dot-identifier rule is overkill — see the doctrine note below).
//
// Defensive contract (PRESERVED, load-bearing): IsNewer returns false on ANY
// parse failure of EITHER input. A corrupt/forged manifest version must never
// trigger an update (the second gate, check.go/swap.go's SHA-256 verify, is the
// belt; this is the braces), and a current (running) version the watcher cannot
// itself parse is treated conservatively — the watcher does not auto-update off
// an unparseable running version. This is the watcher-side inversion of the
// server's gate: here we fail-CLOSED on the UPDATE decision (never update on
// doubt); the server's ingest.IsOlder fails-CLOSED on the GATE (always reject on
// doubt) — opposite directions, same "doubt is safe" spirit.
//
// ONE version-compare doctrine, PER SIDE. This watcher-side IsNewer and the
// server-side internal/backendsrv/ingest/version.go::IsOlder (Plan 01) are
// DELIBERATELY separate, behaviorally-consistent copies: the watcher binary and
// the backend binary must not import each other's internals. They agree in
// direction (a pre-release of a given core is older than that core's final) so
// the P16 coordinated self-update flip is not surprised by a pre-release safety
// manifest — only MAJOR.MINOR.PATCH finals are ever published; the pre-release
// path is a dev-only safety rail, kept correct here so a stray "-rcN" manifest
// can never make a final watcher downgrade.
func IsNewer(current, manifest string) bool {
	cCore, cPre, cOK := parseVersion(current)
	if !cOK {
		// Unparseable running version → do not auto-update off it (conservative).
		return false
	}
	mCore, mPre, mOK := parseVersion(manifest)
	if !mOK {
		// Corrupt/forged manifest → never trigger an update (defensive).
		return false
	}

	// Compare the three core ints in order; the first difference decides.
	for i := 0; i < 3; i++ {
		if mCore[i] > cCore[i] {
			return true
		}
		if mCore[i] < cCore[i] {
			return false
		}
	}

	// Cores are EQUAL — apply pre-release precedence (SemVer §11): a version WITH
	// a pre-release tail is older than the same version WITHOUT one.
	cHasPre := cPre != ""
	mHasPre := mPre != ""
	switch {
	case cHasPre && !mHasPre:
		// current is a pre-release of the manifest's final → the final is newer.
		return true
	case !cHasPre && mHasPre:
		// current is the final, manifest is a pre-release of it → not newer
		// (a final must not downgrade to an rc).
		return false
	case cHasPre && mHasPre:
		// Both pre-releases of the same core → lexical compare of the tails
		// (sufficient for our rcN/betaN scheme; see the doctrine note above).
		return strings.Compare(mPre, cPre) > 0
	default:
		// Equal cores, neither has a tail → not newer (truly equal).
		return false
	}
}

// parseVersion splits "1.2.3" or "1.2.3-rc1" (a leading "v" is stripped first)
// into its MAJOR.MINOR.PATCH int core and an optional pre-release tail (the
// substring after the FIRST '-', so "2.0.0-rc-1" keeps "rc-1" as the tail).
// It returns ok=false if the core is not exactly three numeric parts, keeping
// IsNewer's defensive "false on any parse failure" contract. The tail (if any)
// is returned verbatim for the caller's precedence comparison; any non-empty
// tail counts as a pre-release marker (it is not otherwise validated).
func parseVersion(v string) (core [3]int, pre string, ok bool) {
	v = strings.TrimPrefix(v, "v")
	if v == "" {
		return [3]int{}, "", false
	}

	// Split off the pre-release tail on the FIRST '-' (the core is everything
	// before it).
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
