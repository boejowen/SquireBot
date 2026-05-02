package update

// swap.go implements the OPS-04 startup-swap (Plan 02-06 Task 2).
//
// Called from cmd/squirebot/main.go BEFORE any other goroutine (before
// logging.Setup, before config.Load, before the systray loop). The
// pattern is fixed by 02-CONTEXT.md ("Auto-Update", locked):
//
//   - Startup-swap NEVER in-process. Windows refuses to replace a
//     running .exe; minio/selfupdate.Apply handles the
//     <target>.new -> <target> rename + <target> -> <target>.old + hide
//     dance (the OS won't let .target.old be deleted while any handle
//     is open, so the library hides it instead).
//   - SHA-256 verification is mandatory at every step. The sidecar
//     <target>.expected-sha256 file is the contract between the daily
//     check goroutine (writes it) and Apply (verifies it).
//
// State machine for a single startup:
//
//   1. <exe>.new doesn't exist           -> return (false, nil). Common.
//   2. <exe>.new exists, no sidecar      -> delete .new, return (false, err).
//      (Half-staged state from a crash mid-stage; recover by re-staging
//      next time.)
//   3. <exe>.new exists, sidecar exists, hash mismatches
//                                         -> delete BOTH, return (false, err).
//      (Tampering or download corruption; force a fresh download next time.)
//   4. <exe>.new exists, sidecar exists, hash matches, selfApplyFn ok
//                                         -> delete BOTH, return (true, nil).
//      Caller MUST os.Exit(0) so the swapped-in binary takes over on the
//      next process launch.
//   5. <exe>.new exists, sidecar exists, hash matches, selfApplyFn errors
//                                         -> KEEP .new + sidecar, return (false, err).
//      The next launch will retry the swap. (If it keeps failing, the
//      user can manually delete the .new file and force a fresh stage.)

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/minio/selfupdate"
)

// selfApplyFn is the package-level seam wrapping selfupdate.Apply. Tests
// override it via installFakeSelfApply (in swap_test.go); production
// callers use the real library function. Same pattern as Plan 02-05's
// heartbeat.sleepFn.
var selfApplyFn = selfupdate.Apply

// Apply checks for a staged update adjacent to the running binary and,
// if present + valid, performs the startup-swap via minio/selfupdate.
//
// Returns:
//   - (true, nil)  -> swap succeeded; caller MUST os.Exit(0).
//   - (false, nil) -> no staged file (common cold-start path).
//   - (false, err) -> any failure mode; caller logs to stderr (logging
//     not yet set up at this point in main) and continues running the
//     OLD binary.
//
// Apply is called BEFORE logging.Setup in main.go, so it cannot use
// slog. Errors must be returned for the caller to surface via fmt.Fprintf
// to stderr. (slog.Info is safe AFTER selfApplyFn succeeds because at
// that point we'll os.Exit(0) and the next launch sets up logging.)
func Apply() (swapped bool, err error) {
	exe, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("os.Executable: %w", err)
	}
	return applyAt(exe)
}

// applyAt is the testable form of Apply. Tests construct an exe path
// inside t.TempDir() so they can stage .new + sidecar files without
// touching the real running binary.
func applyAt(exe string) (swapped bool, err error) {
	stagedPath := exe + ".new"
	hashPath := exe + ".expected-sha256"

	// Common path: nothing staged.
	if _, statErr := os.Stat(stagedPath); errors.Is(statErr, os.ErrNotExist) {
		return false, nil
	}

	// State 2: .new exists but sidecar is missing. Half-staged.
	expected, err := os.ReadFile(hashPath)
	if err != nil {
		_ = os.Remove(stagedPath)
		return false, fmt.Errorf("read sidecar hash: %w", err)
	}
	expectedHex := strings.ToLower(strings.TrimSpace(string(expected)))

	// Hash the staged file. We deliberately do NOT defer Close here —
	// Windows refuses to os.Remove a file with an open handle, so each
	// branch below explicitly closes before any remove/swap step.
	staged, err := os.Open(stagedPath)
	if err != nil {
		return false, fmt.Errorf("open staged: %w", err)
	}

	h := sha256.New()
	if _, err := io.Copy(h, staged); err != nil {
		_ = staged.Close()
		return false, fmt.Errorf("hash staged: %w", err)
	}
	actualHex := hex.EncodeToString(h.Sum(nil))

	// State 3: SHA-256 mismatch. Force a fresh download by deleting both.
	// Close the handle FIRST so Windows lets os.Remove succeed.
	if actualHex != expectedHex {
		_ = staged.Close()
		_ = os.Remove(stagedPath)
		_ = os.Remove(hashPath)
		return false, fmt.Errorf("staged hash mismatch: have %s, want %s", actualHex, expectedHex)
	}

	// Rewind for selfApplyFn (which expects to read from the start).
	if _, err := staged.Seek(0, io.SeekStart); err != nil {
		_ = staged.Close()
		return false, fmt.Errorf("rewind staged: %w", err)
	}

	// State 4 vs 5: invoke the swap. selfApplyFn must read the entire
	// staged Reader before returning; selfupdate.Apply's contract is to
	// drain the reader synchronously inside Apply.
	applyErr := selfApplyFn(staged, selfupdate.Options{TargetPath: exe})
	// Close the file handle BEFORE attempting any os.Remove on stagedPath
	// (Windows file-lock contract).
	_ = staged.Close()

	if applyErr != nil {
		// State 5: PRESERVE .new + sidecar so the next launch retries.
		// Do NOT delete on this path. (selfupdate.Apply leaves a .target.old
		// behind on Windows by design — see CONTEXT.md.)
		return false, fmt.Errorf("selfupdate.Apply: %w", applyErr)
	}

	// State 4: success. Clean up the stage + sidecar. Best-effort tidy of
	// any .target.old residue selfupdate may have left (Windows hides it
	// instead of deleting because the file handle for the still-running
	// old process keeps it locked; on the NEXT launch with the new binary
	// the .target.old can be removed).
	_ = os.Remove(stagedPath)
	_ = os.Remove(hashPath)
	_ = os.Remove(filepath.Clean(exe + ".old"))

	slog.Info("auto-update applied", "exe", exe, "sha256", actualHex)
	return true, nil
}
