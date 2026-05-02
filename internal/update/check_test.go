package update

// Tests for the OPS-04 daily-check goroutine + manual fire (Plan 02-06
// Task 3). Six behaviours:
//
//   1. CheckOnce when manifest version equals current -> nil; no staging
//      files written; statusFn NOT called.
//   2. CheckOnce when manifest is newer + binary download SHA-256 matches
//      -> writes <exe>.new + <exe>.expected-sha256; statusFn invoked with
//      "Update ready (X.Y.Z); restart to apply".
//   3. CheckOnce when downloaded binary's SHA-256 mismatches manifest ->
//      returns error wrapping 'hash mismatch'; .new.tmp deleted; .new and
//      sidecar NOT written.
//   4. CheckOnce when manifest fetch fails (404) -> returns error wrapping
//      the HTTP status; no staging; statusFn NOT called.
//   5. RunDailyCheck launches a goroutine; first fire is immediate; the
//      next sleep is requested with d=Interval (24h); ctx cancellation
//      cleanly exits.
//   6. CheckOnce when manifest is newer + a matching .new + sidecar
//      already exist -> returns nil without re-downloading; statusFn
//      called with "Update ready" (idempotent re-stage).
//
// Bonus behaviours pinned by tests:
//   - CheckOnce when manifest is newer but BinaryURL is empty
//     (Phase 1 manifest, or pre-Plan-02-08 release) -> returns nil with
//     a "skipped: no binary asset" log path; no error, no staging.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// makeManifestServer returns an httptest.Server whose / serves the given
// manifest JSON. Tests pass the server URL as the manifestURL parameter
// to checkOnceWithURLs.
func makeManifestServer(t *testing.T, body string, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if status != 0 && status != http.StatusOK {
			http.Error(w, body, status)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
}

// makeBinaryServer returns an httptest.Server that serves the given byte
// stream as the binary. The Content-Type doesn't matter; the manifest
// drives the SHA-256 verification.
func makeBinaryServer(t *testing.T, payload []byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
}

// captureStatus is a goroutine-safe sink for StatusUpdater calls.
type captureStatus struct {
	mu  sync.Mutex
	all []string
}

func (c *captureStatus) fn(msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.all = append(c.all, msg)
}

func (c *captureStatus) snapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.all))
	copy(out, c.all)
	return out
}

// installFakeCheckSleep swaps checkSleepFn for tests. Same pattern as
// heartbeat.installFakeSleep. Returns a release function so tests can
// drive the next CheckOnce iteration deterministically.
type checkSleepCapture struct {
	mu        sync.Mutex
	durations []time.Duration
	gate      chan error
	closed    bool
}

func newCheckSleepCapture() *checkSleepCapture {
	return &checkSleepCapture{gate: make(chan error, 16)}
}

func (s *checkSleepCapture) install(t *testing.T) {
	t.Helper()
	prev := checkSleepFn
	checkSleepFn = func(ctx context.Context, d time.Duration) error {
		s.mu.Lock()
		s.durations = append(s.durations, d)
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err, ok := <-s.gate:
			if !ok {
				return ctx.Err()
			}
			return err
		}
	}
	t.Cleanup(func() {
		checkSleepFn = prev
		s.mu.Lock()
		if !s.closed {
			close(s.gate)
			s.closed = true
		}
		s.mu.Unlock()
	})
}

func (s *checkSleepCapture) durationsCopy() []time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]time.Duration, len(s.durations))
	copy(out, s.durations)
	return out
}

// Test 1: CheckOnce when manifest equals current -> nil, no staging,
// statusFn NOT called.
func TestCheckOnce_NoNewerVersionIsNoop(t *testing.T) {
	manifest := `{"version":"1.0.0","installer_url":"http://x","installer_sha256":"abc","binary_url":"http://x/squirebot.exe","binary_sha256":"def","released_at":"2026-05-02T00:00:00Z"}`
	mSrv := makeManifestServer(t, manifest, http.StatusOK)
	t.Cleanup(mSrv.Close)

	dir := t.TempDir()
	exe := filepath.Join(dir, "squirebot.exe")
	cap := &captureStatus{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	err := checkOnceWithURL(ctx, mSrv.URL, "1.0.0", exe, cap.fn)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if _, statErr := os.Stat(exe + ".new"); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf(".new was written for an equal-version manifest")
	}
	if got := cap.snapshot(); len(got) != 0 {
		t.Errorf("statusFn called %d times, want 0: %v", len(got), got)
	}
}

// Test 2: CheckOnce on a newer-version manifest + matching SHA -> stages
// .new + sidecar; statusFn called with "Update ready".
func TestCheckOnce_NewerVersionStagesBinary(t *testing.T) {
	binaryPayload := []byte("the-new-squirebot-binary-bytes")
	binSrv := makeBinaryServer(t, binaryPayload)
	t.Cleanup(binSrv.Close)
	hash := sha256.Sum256(binaryPayload)
	hashHex := hex.EncodeToString(hash[:])

	manifest := `{"version":"1.2.0","installer_url":"http://x","installer_sha256":"abc","binary_url":"` + binSrv.URL + `","binary_sha256":"` + hashHex + `","released_at":"2026-05-02T00:00:00Z"}`
	mSrv := makeManifestServer(t, manifest, http.StatusOK)
	t.Cleanup(mSrv.Close)

	dir := t.TempDir()
	exe := filepath.Join(dir, "squirebot.exe")
	cap := &captureStatus{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	err := checkOnceWithURL(ctx, mSrv.URL, "1.0.0", exe, cap.fn)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	got, readErr := os.ReadFile(exe + ".new")
	if readErr != nil {
		t.Fatalf("read .new: %v", readErr)
	}
	if string(got) != string(binaryPayload) {
		t.Errorf(".new contents mismatch: got %q, want %q", got, binaryPayload)
	}
	sidecar, sErr := os.ReadFile(exe + ".expected-sha256")
	if sErr != nil {
		t.Fatalf("read sidecar: %v", sErr)
	}
	if strings.TrimSpace(string(sidecar)) != hashHex {
		t.Errorf("sidecar = %q, want %q", string(sidecar), hashHex)
	}
	statuses := cap.snapshot()
	if len(statuses) != 1 {
		t.Fatalf("statusFn called %d times, want 1: %v", len(statuses), statuses)
	}
	if !strings.Contains(statuses[0], "1.2.0") {
		t.Errorf("status = %q, want substring 1.2.0", statuses[0])
	}
	if !strings.Contains(statuses[0], "restart") {
		t.Errorf("status = %q, want substring 'restart'", statuses[0])
	}
}

// Test 3: SHA-256 mismatch -> error, .tmp deleted, .new + sidecar NOT
// written.
func TestCheckOnce_HashMismatchRejectsDownload(t *testing.T) {
	binaryPayload := []byte("the-actual-bytes")
	binSrv := makeBinaryServer(t, binaryPayload)
	t.Cleanup(binSrv.Close)

	// Manifest claims a DIFFERENT sha (e.g., the hash of a different
	// payload). The download should be rejected.
	bogusHash := strings.Repeat("00", sha256.Size)

	manifest := `{"version":"1.2.0","installer_url":"http://x","installer_sha256":"abc","binary_url":"` + binSrv.URL + `","binary_sha256":"` + bogusHash + `","released_at":"2026-05-02T00:00:00Z"}`
	mSrv := makeManifestServer(t, manifest, http.StatusOK)
	t.Cleanup(mSrv.Close)

	dir := t.TempDir()
	exe := filepath.Join(dir, "squirebot.exe")
	cap := &captureStatus{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	err := checkOnceWithURL(ctx, mSrv.URL, "1.0.0", exe, cap.fn)
	if err == nil {
		t.Fatal("err = nil, want non-nil (hash mismatch)")
	}
	if !strings.Contains(err.Error(), "mismatch") {
		t.Errorf("err = %q, want substring 'mismatch'", err.Error())
	}
	if _, statErr := os.Stat(exe + ".new"); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf(".new was written despite hash mismatch")
	}
	if _, statErr := os.Stat(exe + ".expected-sha256"); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("sidecar was written despite hash mismatch")
	}
	if _, statErr := os.Stat(exe + ".new.tmp"); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf(".new.tmp was not cleaned up after hash mismatch")
	}
	if got := cap.snapshot(); len(got) != 0 {
		t.Errorf("statusFn called %d times, want 0 on mismatch: %v", len(got), got)
	}
}

// Test 4: manifest 404 -> error, no staging, no statusFn call.
func TestCheckOnce_ManifestFetchFails(t *testing.T) {
	mSrv := makeManifestServer(t, "not found", http.StatusNotFound)
	t.Cleanup(mSrv.Close)

	dir := t.TempDir()
	exe := filepath.Join(dir, "squirebot.exe")
	cap := &captureStatus{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	err := checkOnceWithURL(ctx, mSrv.URL, "1.0.0", exe, cap.fn)
	if err == nil {
		t.Fatal("err = nil, want non-nil (404)")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("err = %q, want substring '404'", err.Error())
	}
	if _, statErr := os.Stat(exe + ".new"); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf(".new exists after manifest 404")
	}
	if got := cap.snapshot(); len(got) != 0 {
		t.Errorf("statusFn called %d times, want 0 on manifest fail: %v", len(got), got)
	}
}

// Test 5: RunDailyCheck immediate fire + 24h sleep + ctx cancellation
// exits cleanly.
func TestRunDailyCheck_FiresImmediatelyAndSchedules(t *testing.T) {
	sleeps := newCheckSleepCapture()
	sleeps.install(t)

	// Manifest equal to current -> CheckOnce returns nil quickly; no
	// download attempted.
	manifest := `{"version":"1.0.0","installer_url":"http://x","installer_sha256":"abc","binary_url":"http://x/squirebot.exe","binary_sha256":"def","released_at":"2026-05-02T00:00:00Z"}`
	mSrv := makeManifestServer(t, manifest, http.StatusOK)
	t.Cleanup(mSrv.Close)

	dir := t.TempDir()
	exe := filepath.Join(dir, "squirebot.exe")
	cap := &captureStatus{}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		runDailyCheckWithURL(ctx, mSrv.URL, "1.0.0", exe, cap.fn)
		close(done)
	}()

	// Wait for the first sleep to be requested (proxy for "first tick
	// completed").
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(sleeps.durationsCopy()) > 0 {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	durs := sleeps.durationsCopy()
	if len(durs) == 0 {
		t.Fatal("RunDailyCheck did not request a sleep within 2s of immediate first fire")
	}
	if durs[0] != checkInterval {
		t.Errorf("first sleep = %v, want %v", durs[0], checkInterval)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunDailyCheck did not return within 2s of ctx cancellation")
	}
}

// Test 6: Idempotency. Pre-stage a .new + matching sidecar; CheckOnce on
// the same manifest returns nil without re-downloading; statusFn called.
func TestCheckOnce_IdempotentWhenAlreadyStaged(t *testing.T) {
	binaryPayload := []byte("already-staged-bytes")
	hash := sha256.Sum256(binaryPayload)
	hashHex := hex.EncodeToString(hash[:])

	// Binary server records calls via a counter; the test asserts the
	// counter is ZERO (no re-download).
	var binDownloadCount atomic.Int32
	binSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		binDownloadCount.Add(1)
		_, _ = w.Write(binaryPayload)
	}))
	t.Cleanup(binSrv.Close)

	manifest := `{"version":"1.2.0","installer_url":"http://x","installer_sha256":"abc","binary_url":"` + binSrv.URL + `","binary_sha256":"` + hashHex + `","released_at":"2026-05-02T00:00:00Z"}`
	mSrv := makeManifestServer(t, manifest, http.StatusOK)
	t.Cleanup(mSrv.Close)

	dir := t.TempDir()
	exe := filepath.Join(dir, "squirebot.exe")
	cap := &captureStatus{}

	// Pre-stage with the matching SHA.
	if err := os.WriteFile(exe+".new", binaryPayload, 0o600); err != nil {
		t.Fatalf("pre-stage .new: %v", err)
	}
	if err := os.WriteFile(exe+".expected-sha256", []byte(hashHex+"\n"), 0o600); err != nil {
		t.Fatalf("pre-stage sidecar: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	err := checkOnceWithURL(ctx, mSrv.URL, "1.0.0", exe, cap.fn)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if got := binDownloadCount.Load(); got != 0 {
		t.Errorf("binary downloaded %d times, want 0 (idempotent)", got)
	}
	statuses := cap.snapshot()
	if len(statuses) != 1 {
		t.Fatalf("statusFn called %d times, want 1: %v", len(statuses), statuses)
	}
	if !strings.Contains(statuses[0], "1.2.0") {
		t.Errorf("status = %q, want substring 1.2.0", statuses[0])
	}
}

// Bonus: when the manifest is newer but BinaryURL is empty (Phase 1
// manifest, or any release pre-Plan-02-08), CheckOnce returns nil with
// no staging — surfaced as a logged "skipped: no binary asset" path.
func TestCheckOnce_NewerManifestWithEmptyBinaryURLIsNoop(t *testing.T) {
	// No binary_url field in manifest at all.
	manifest := `{"version":"1.2.0","installer_url":"http://x","installer_sha256":"abc","released_at":"2026-05-02T00:00:00Z"}`
	mSrv := makeManifestServer(t, manifest, http.StatusOK)
	t.Cleanup(mSrv.Close)

	dir := t.TempDir()
	exe := filepath.Join(dir, "squirebot.exe")
	cap := &captureStatus{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	err := checkOnceWithURL(ctx, mSrv.URL, "1.0.0", exe, cap.fn)
	if err != nil {
		t.Fatalf("err = %v, want nil (skip path)", err)
	}
	if _, statErr := os.Stat(exe + ".new"); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf(".new written despite empty BinaryURL")
	}
	if got := cap.snapshot(); len(got) != 0 {
		t.Errorf("statusFn called %d times, want 0 on skip: %v", len(got), got)
	}
}
