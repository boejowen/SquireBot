package update

// check.go implements the OPS-04 daily background check + manual "Check
// for updates" trigger (Plan 02-06 Task 3). The flow per fire:
//
//   1. Fetch latest.json from the GitHub Releases CDN.
//   2. If manifest.version <= current  -> return nil (no-op).
//   3. If manifest.binary_url is empty -> return nil (Phase 1 release;
//      no bare-binary asset; the in-app swap is not possible).
//   4. If <exe>.new + <exe>.expected-sha256 already exist with matching
//      hash -> return nil (idempotent — already staged).
//   5. Download manifest.binary_url to <exe>.new.tmp, hashing in stream.
//   6. SHA-256-verify against manifest.binary_sha256.
//   7. On match: rename .tmp -> .new; write .expected-sha256 sidecar;
//      call statusFn("Update ready (vX.Y.Z); restart to apply").
//   8. On mismatch: delete .tmp; return error wrapping "hash mismatch".
//
// Concurrency: checkMu serializes CheckOnce calls so the daily tick
// firing concurrently with a manual "Check for updates" click can never
// double-stage. The next call sees the .new + sidecar already present
// and short-circuits (step 4).
//
// Coordination with heartbeat: this goroutine and heartbeat.Run both
// fire on a 24h cadence but they don't share state. The heartbeat goes
// through sheet.Client (mutex-funneled batchUpdate from Plan 02-03);
// the auto-update fetches + downloads via direct net/http, no Sheets
// API. They are independent.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// checkInterval is the cadence between automatic update checks. 24h
// matches heartbeat.Interval; chosen for symmetry with the OPS-05
// heartbeat — not a technical requirement.
const checkInterval = 24 * time.Hour

// checkSleepFn is the package-level seam for the reschedule wait.
// Production = realCheckSleep. Tests override via checkSleepCapture
// (mirrors heartbeat.sleepFn).
var checkSleepFn = realCheckSleep

func realCheckSleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// checkMu serializes CheckOnce. Protects against the race where the 24h
// tick fires while the user clicks "Check for updates".
var checkMu sync.Mutex

// StatusUpdater is the small callback the tray plumbs in. CheckOnce
// invokes it on a successful staging to surface the "Update ready"
// message. statusFn may be nil (CheckOnce no-ops the call).
type StatusUpdater func(msg string)

// CheckOnce performs a single update check + staging cycle. owner/repo
// build the canonical GitHub Releases CDN manifest URL.
//
// Errors are returned to the caller; production callers (the daily
// goroutine, the tray click handler) log them and continue.
func CheckOnce(ctx context.Context, owner, repo, currentVersion, exePath string, statusFn StatusUpdater) error {
	return checkOnceWithURL(ctx, ManifestURL(owner, repo), currentVersion, exePath, statusFn)
}

// checkOnceWithURL is the test seam for CheckOnce. Tests inject an
// httptest.Server URL here; production callers use CheckOnce.
func checkOnceWithURL(ctx context.Context, manifestURL, currentVersion, exePath string, statusFn StatusUpdater) error {
	checkMu.Lock()
	defer checkMu.Unlock()

	m, err := FetchFromURL(ctx, manifestURL)
	if err != nil {
		return fmt.Errorf("CheckOnce manifest: %w", err)
	}
	if !IsNewer(currentVersion, m.Version) {
		slog.Info("auto-update: no newer version", "current", currentVersion, "manifest", m.Version)
		return nil
	}

	// Select the bare-binary asset for THIS platform (runtime.GOOS):
	// windows -> the .exe (binary_url/binary_sha256), linux -> the bare
	// linux squirebot (binary_url_linux/binary_sha256_linux). This is the
	// ONE OS-specific seam (Phase 25 LNX-05) — the download/verify/stage
	// flow below is byte-identical regardless of platform.
	binURL, binSHA := m.binaryAsset()

	// Phase 1 manifest (or any pre-Plan-02-08 release) lacks the
	// bare-binary asset; the in-app swap is not possible. Don't waste
	// bytes downloading the installer (it's the wrong shape for
	// selfupdate.Apply to swap over the running binary). On a Linux box
	// reading an OLD Windows-only manifest, binURL/binSHA are empty here
	// (no binary_url_linux), so this no-op fires and the Linux watcher
	// NEVER downloads the Windows .exe (RESEARCH §3 Pitfall 3 / T-25-10).
	if binURL == "" || binSHA == "" {
		slog.Info("auto-update: manifest missing binary_url; skipping (manual upgrade only)",
			"manifest_version", m.Version)
		return nil
	}

	stagedPath := exePath + ".new"
	hashPath := exePath + ".expected-sha256"

	// Idempotency: if .new + sidecar already exist with the manifest's
	// expected hash, skip the download. (Matches the pattern from Task 2's
	// applyAt: the sidecar is the contract.)
	if existingHash, readErr := os.ReadFile(hashPath); readErr == nil {
		if strings.EqualFold(strings.TrimSpace(string(existingHash)), binSHA) {
			slog.Info("auto-update: already staged; skipping download", "version", m.Version)
			if statusFn != nil {
				statusFn(fmt.Sprintf("Update ready (%s); restart to apply", m.Version))
			}
			return nil
		}
	}

	slog.Info("auto-update: downloading", "version", m.Version, "url", binURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, binURL, nil)
	if err != nil {
		return fmt.Errorf("CheckOnce download req: %w", err)
	}
	req.Header.Set("User-Agent", "SquireBot-AutoUpdate")
	c := &http.Client{Timeout: 5 * time.Minute}
	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("CheckOnce download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("CheckOnce download HTTP %d", resp.StatusCode)
	}

	tmpPath := exePath + ".new.tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("CheckOnce create tmp: %w", err)
	}
	h := sha256.New()
	mw := io.MultiWriter(f, h)
	if _, err := io.Copy(mw, resp.Body); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("CheckOnce copy: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("CheckOnce close tmp: %w", err)
	}

	actualHex := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actualHex, binSHA) {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("CheckOnce hash mismatch: got %s, want %s", actualHex, binSHA)
	}

	// Atomic stage: rename .tmp -> .new, then write the sidecar. If the
	// sidecar write fails we MUST roll back .new (otherwise applyAt would
	// see a half-staged state on next launch).
	if err := os.Rename(tmpPath, stagedPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("CheckOnce stage: %w", err)
	}
	if err := os.WriteFile(hashPath, []byte(strings.ToLower(binSHA)+"\n"), 0o600); err != nil {
		_ = os.Remove(stagedPath)
		return fmt.Errorf("CheckOnce sidecar: %w", err)
	}

	slog.Info("auto-update: staged", "version", m.Version, "sha256", actualHex)
	if statusFn != nil {
		statusFn(fmt.Sprintf("Update ready (%s); restart to apply", m.Version))
	}
	return nil
}

// RunDailyCheck blocks; fires CheckOnce immediately, then every
// checkInterval until ctx is cancelled. Errors are LOGGED but never kill
// the goroutine — the watcher must keep running even if GitHub is
// unreachable for a stretch.
func RunDailyCheck(ctx context.Context, owner, repo, currentVersion, exePath string, statusFn StatusUpdater) {
	runDailyCheckWithURL(ctx, ManifestURL(owner, repo), currentVersion, exePath, statusFn)
}

// runDailyCheckWithURL is the test seam for RunDailyCheck. Same
// signature, takes the manifest URL directly (so tests can use an
// httptest.Server).
func runDailyCheckWithURL(ctx context.Context, manifestURL, currentVersion, exePath string, statusFn StatusUpdater) {
	if err := checkOnceWithURL(ctx, manifestURL, currentVersion, exePath, statusFn); err != nil {
		slog.Warn("auto-update check failed", "err", err)
	}
	for {
		if err := checkSleepFn(ctx, checkInterval); err != nil {
			slog.Info("auto-update goroutine exiting", "err", err)
			return
		}
		if err := checkOnceWithURL(ctx, manifestURL, currentVersion, exePath, statusFn); err != nil {
			slog.Warn("auto-update check failed", "err", err)
		}
	}
}
