package update

// Tests for the OPS-04 startup-swap (Plan 02-06 Task 2). Five behaviours:
//
//   1. No staged file -> applyAt returns (false, nil). Common path on every
//      cold start.
//   2. .new exists but the .expected-sha256 sidecar is missing -> .new is
//      deleted; (false, error).
//   3. .new exists, sidecar exists, SHA-256 mismatches -> both are deleted;
//      (false, error wrapping "hash mismatch"). Defends against tampering
//      between download and swap.
//   4. .new exists, sidecar exists, SHA-256 matches -> selfApplyFn invoked
//      (mocked); on success returns (true, nil); .new + sidecar removed.
//   5. .new exists, sidecar exists, SHA-256 matches, but selfApplyFn errors
//      -> returns (false, err); .new + sidecar are preserved so the next
//      launch retries the swap.

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minio/selfupdate"
)

// stageStaged writes contents to <exe>.new and a sidecar .expected-sha256
// containing the SHA-256 hex of contents.
func stageStaged(t *testing.T, exe string, contents []byte) {
	t.Helper()
	if err := os.WriteFile(exe+".new", contents, 0o600); err != nil {
		t.Fatalf("write staged: %v", err)
	}
	h := sha256.Sum256(contents)
	if err := os.WriteFile(exe+".expected-sha256", []byte(hex.EncodeToString(h[:])+"\n"), 0o600); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}
}

// stageStagedWithBadSidecar writes the staged file but a sidecar containing
// the wrong hash (so SHA-256 verify will fail).
func stageStagedWithBadSidecar(t *testing.T, exe string, contents []byte) {
	t.Helper()
	if err := os.WriteFile(exe+".new", contents, 0o600); err != nil {
		t.Fatalf("write staged: %v", err)
	}
	bogusHash := strings.Repeat("00", sha256.Size)
	if err := os.WriteFile(exe+".expected-sha256", []byte(bogusHash+"\n"), 0o600); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}
}

// installFakeSelfApply replaces the package-level seam with a mock; restored
// via t.Cleanup. The fn parameter receives the staged Reader so tests can
// inspect / drain it.
func installFakeSelfApply(t *testing.T, fn func(r io.Reader, opts selfupdate.Options) error) {
	t.Helper()
	prev := selfApplyFn
	selfApplyFn = fn
	t.Cleanup(func() { selfApplyFn = prev })
}

// Test 1: No staged file -> (false, nil). Common path.
func TestApply_NoStagedFileReturnsNil(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "squirebot.exe")
	// Touch the exe path so applyAt's os.Stat for the staged file is the
	// only thing that's missing.
	if err := os.WriteFile(exe, []byte("dummy"), 0o600); err != nil {
		t.Fatalf("write exe: %v", err)
	}

	swapped, err := applyAt(exe)
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if swapped {
		t.Errorf("swapped = true, want false")
	}
}

// Test 2: .new exists but sidecar is missing -> .new deleted, (false, err).
func TestApply_MissingSidecarDeletesStaged(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "squirebot.exe")
	if err := os.WriteFile(exe+".new", []byte("staged-content"), 0o600); err != nil {
		t.Fatalf("write staged: %v", err)
	}
	// Intentionally no sidecar.

	swapped, err := applyAt(exe)
	if err == nil {
		t.Fatal("err = nil, want non-nil (sidecar missing)")
	}
	if swapped {
		t.Errorf("swapped = true, want false")
	}
	// .new should have been deleted to clear the half-staged state.
	if _, statErr := os.Stat(exe + ".new"); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf(".new still exists after sidecar-missing failure")
	}
}

// Test 3: SHA-256 mismatch -> both .new and sidecar deleted, (false, err).
func TestApply_HashMismatchDeletesBoth(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "squirebot.exe")
	stageStagedWithBadSidecar(t, exe, []byte("staged-content"))

	swapped, err := applyAt(exe)
	if err == nil {
		t.Fatal("err = nil, want non-nil (hash mismatch)")
	}
	if swapped {
		t.Errorf("swapped = true, want false")
	}
	if !strings.Contains(err.Error(), "mismatch") {
		t.Errorf("err = %q does not contain 'mismatch'", err.Error())
	}
	if _, statErr := os.Stat(exe + ".new"); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf(".new still exists after hash mismatch")
	}
	if _, statErr := os.Stat(exe + ".expected-sha256"); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("sidecar still exists after hash mismatch")
	}
}

// Test 4: matching SHA-256 + selfApplyFn success -> (true, nil); cleanup.
func TestApply_SuccessSwapsAndCleansUp(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "squirebot.exe")
	contents := []byte("new-binary-bytes")
	stageStaged(t, exe, contents)

	called := false
	installFakeSelfApply(t, func(r io.Reader, opts selfupdate.Options) error {
		called = true
		// Drain the reader so we exercise the same code path the real
		// selfupdate library would.
		_, _ = io.Copy(io.Discard, r)
		if opts.TargetPath != exe {
			t.Errorf("TargetPath = %q, want %q", opts.TargetPath, exe)
		}
		return nil
	})

	swapped, err := applyAt(exe)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !swapped {
		t.Errorf("swapped = false, want true")
	}
	if !called {
		t.Errorf("selfApplyFn was not invoked")
	}
	// .new + sidecar should be removed after a successful swap.
	if _, statErr := os.Stat(exe + ".new"); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf(".new still exists after successful swap")
	}
	if _, statErr := os.Stat(exe + ".expected-sha256"); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("sidecar still exists after successful swap")
	}
}

// Test 5: matching SHA-256 + selfApplyFn error -> (false, err); .new +
// sidecar PRESERVED so the next launch retries.
func TestApply_SelfApplyErrorPreservesStaged(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "squirebot.exe")
	contents := []byte("new-binary-bytes")
	stageStaged(t, exe, contents)

	installFakeSelfApply(t, func(r io.Reader, opts selfupdate.Options) error {
		return errors.New("simulated swap failure")
	})

	swapped, err := applyAt(exe)
	if err == nil {
		t.Fatal("err = nil, want non-nil (selfApplyFn returned error)")
	}
	if swapped {
		t.Errorf("swapped = true, want false")
	}
	// .new + sidecar must be preserved so the next launch can retry.
	if _, statErr := os.Stat(exe + ".new"); errors.Is(statErr, os.ErrNotExist) {
		t.Errorf(".new was deleted after selfApplyFn error; should be preserved for retry")
	}
	if _, statErr := os.Stat(exe + ".expected-sha256"); errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("sidecar was deleted after selfApplyFn error; should be preserved for retry")
	}
}
