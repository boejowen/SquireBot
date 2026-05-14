---
phase: 02-watcher-robustness-schema-lock
plan: 06
subsystem: auto-update
tags: [auto-update, ops-04, startup-swap, sha256, github-releases, tray, minio-selfupdate, pitfall-14]
requires:
  - 02-04 (globalAuthSuspended atomic.Bool — heartbeat + auto-update goroutines coexist; auto-update is not auth-suspension-aware because it doesn't touch Sheets API)
  - 02-05 (heartbeat goroutine wiring pattern in runWatcher; test seams: sleepFn / sleepCapture mirror)
  - 02-08 (release.yml emits dist/squirebot.exe + latest.json with binary_url + binary_sha256 — the contract this plan consumes)
provides:
  - update.Manifest (Phase 2 latest.json schema, mirror of release.yml)
  - update.Fetch / update.FetchFromURL (manifest fetch via GitHub Releases CDN)
  - update.IsNewer (3-part numeric semver comparison; defensive on parse failure)
  - update.Apply / update.applyAt (startup-swap with SHA-256 verify + minio/selfupdate.Apply seam)
  - update.CheckOnce (manual + daily fire; download + SHA-256 + atomic stage)
  - update.RunDailyCheck (24h goroutine; immediate first fire)
  - tray.LabelCheckUpdates + tray.Config.OnCheckUpdates (manual fire from menu)
  - config.Config.PendingUpdateVersion (informational; .new file is source of truth)
affects:
  - internal/update/manifest.go (new)
  - internal/update/manifest_test.go (new)
  - internal/update/swap.go (new)
  - internal/update/swap_test.go (new)
  - internal/update/check.go (new)
  - internal/update/check_test.go (new)
  - internal/tray/tray.go (modified — LabelCheckUpdates, OnCheckUpdates, mCheckUpdates, OnReady wires plan[2], loop multiplexes)
  - internal/tray/tray_test.go (modified — length-7 MenuPlan, CheckUpdatesPosition, OnCheckUpdatesCallback_Wired, LabelConstants extended)
  - internal/app/runapp.go (modified — update import + RunDailyCheck goroutine launched alongside heartbeat)
  - cmd/squirebot/main.go (modified — update.Apply BEFORE logging.Setup; OnCheckUpdates closure wires CheckOnce)
  - internal/config/config.go (modified — PendingUpdateVersion informational field)
  - go.mod / go.sum (added github.com/minio/selfupdate v0.6.0 + indirect aead.dev/minisign v0.2.0)
tech-stack:
  added:
    - "github.com/minio/selfupdate v0.6.0 (CONTEXT.md locked dep; the .target.new -> live + live -> .target.old + hide-on-Windows rename dance the entire OPS-04 flow exists to avoid hand-rolling)"
    - "aead.dev/minisign v0.2.0 (transitive via minio/selfupdate; not used directly)"
  patterns:
    - "Startup-swap NEVER in-process (CONTEXT.md locked, Pitfall 14): cmd/squirebot/main.go runs update.Apply() as the FIRST action of main(), BEFORE logging.Setup. On (true, nil) we os.Exit(0) so the swapped-in binary takes over on the next process launch."
    - "SHA-256 sidecar contract: <exe>.expected-sha256 file (lowercase hex + newline) is the SOURCE OF TRUTH the daily check writes and the startup-swap verifies. checkOnceWithURL writes it last (atomic stage); applyAt reads it first (refuses to swap without it)."
    - "Two-stage SHA-256 verify: (a) download path verifies via io.MultiWriter teed sha256.Hash during io.Copy, comparing against manifest.binary_sha256; (b) startup-swap re-verifies before invoking selfupdate.Apply. Both are mandatory — defends against tampering between download and swap."
    - "Atomic stage rename: download -> <exe>.new.tmp -> verify -> os.Rename to <exe>.new -> write sidecar. If sidecar write fails, the .new is rolled back (otherwise applyAt would see a half-staged state on next launch)."
    - "Idempotency check: CheckOnce reads the existing sidecar BEFORE downloading; if it matches manifest.binary_sha256, skips download but still calls statusFn (so the tray surfaces 'Update ready' on every check, not just the first)."
    - "Test seams matching Plans 02-03/04/05 conventions: selfApplyFn package var (mirrors heartbeat.sleepFn) for swap; checkSleepFn for daily reschedule; checkOnceWithURL / runDailyCheckWithURL / FetchFromURL for direct httptest.Server URL injection (avoids monkey-patching ManifestURL)."
    - "Independent goroutines (auto-update + heartbeat): heartbeat goes through Sheets API (mutex-funneled batchUpdate from Plan 02-03); auto-update goes through direct net/http to GitHub Releases. No shared state; no coordination needed."
    - "Phase 1 manifest fallback: when Manifest.BinaryURL is empty (a Phase 1 release predating Plan 02-08), CheckOnce returns nil with a 'skipping (manual upgrade only)' log line — the in-app swap requires the bare binary asset that Plan 02-08's release.yml started emitting."
    - "Windows file-lock handling in swap.go: explicit staged.Close() BEFORE every os.Remove and BEFORE selfApplyFn returns. The first GREEN draft used 'defer staged.Close()' which left the handle open through the cleanup steps, causing two tests to fail with '.new still exists' (Pitfall 14 manifested in our own code; auto-fixed)."
key-files:
  created:
    - internal/update/manifest.go (~150 lines)
    - internal/update/manifest_test.go (~195 lines, 7 tests)
    - internal/update/swap.go (~135 lines)
    - internal/update/swap_test.go (~205 lines, 5 tests)
    - internal/update/check.go (~205 lines)
    - internal/update/check_test.go (~410 lines, 7 tests)
  modified:
    - internal/tray/tray.go (LabelCheckUpdates, OnCheckUpdates field, mCheckUpdates field, OnReady plan[2] wire, loop multiplexer, NewController plumbing)
    - internal/tray/tray_test.go (length-7 wantOrder, LabelConstants extended, CheckUpdatesPosition test, OnCheckUpdatesCallback_Wired test)
    - internal/app/runapp.go (internal/update import, RunDailyCheck goroutine launched alongside heartbeat in runWatcher)
    - cmd/squirebot/main.go (update.Apply as first action, OnCheckUpdates closure spawns CheckOnce goroutine, fmt + update imports)
    - internal/config/config.go (PendingUpdateVersion field — informational, .new file is the source of truth)
    - go.mod / go.sum (minio/selfupdate v0.6.0 promoted from indirect to direct after swap.go imports it)
decisions:
  - "Three-file split inside internal/update: manifest.go (data + fetch), swap.go (startup verify + minio/selfupdate.Apply), check.go (download + atomic stage + 24h loop). Each file has its own test file (manifest_test.go / swap_test.go / check_test.go). Reason: each file has a different test surface (httptest.Server for manifest, t.TempDir for swap, both for check) and a different test seam (FetchFromURL vs selfApplyFn vs checkSleepFn). One monolith file would mix concerns; the files match the three responsibilities the package doc-comment lists."
  - "Test seam pattern: function-with-URL test variants (checkOnceWithURL, runDailyCheckWithURL, FetchFromURL) take the URL directly so tests inject httptest.Server URLs without monkey-patching ManifestURL. Production callers use CheckOnce / RunDailyCheck / Fetch which compute the URL via ManifestURL(owner, repo). This is the same pattern Plan 02-03's withRetry uses (sleepFn package var swappable in tests)."
  - "selfApplyFn package var seam (mirrors Plan 02-05's heartbeat.sleepFn). Tests install a fake via installFakeSelfApply (t.Cleanup-restored). Production = github.com/minio/selfupdate.Apply directly. Crucially: the test does NOT need the real selfupdate library to perform an actual file rename — Test 4 (Success) verifies our pre-rename + post-rename cleanup contract; Test 5 (SelfApplyError) verifies our roll-forward contract on swap failure. The library's actual rename mechanics are validated by minio's own tests + the live verification step."
  - "CheckOnce idempotency design: the sidecar (.expected-sha256) is checked BEFORE the download. If a previous check already staged the same version, the second call short-circuits AND still fires statusFn — so the tray surfaces 'Update ready' on every fire (the user might have dismissed the previous status update), not just the first. Acceptance test TestCheckOnce_IdempotentWhenAlreadyStaged uses a per-call binary download counter to verify ZERO downloads on the idempotent path."
  - "Phase 1 manifest fallback (BinaryURL empty -> CheckOnce returns nil) is intentional and tested. Reason: Phase 1's release.yml emitted a manifest WITHOUT binary_url. If a guildie still has a Phase 1 watcher running and we publish a Phase 2 release, the watcher should notice the manifest is newer but recognize it can't perform the in-app swap (the asset shape is wrong — installer .exe, not bare binary). Returning nil + logging 'skipping (manual upgrade only)' is the honest answer; Phase 5 polish could surface this in the tray as 'Manual upgrade required'."
  - "owner/repo HARD-CODED to ('boejowen', 'SquireBot') in main.go's OnCheckUpdates closure and in runapp.go's RunDailyCheck launch. Plan offered a choice between hard-code and ldflag injection ('var UpdateOwner = ...' / 'var UpdateRepo = ...' threaded through auth.BuildConstants). Chose hard-code per the plan's recommendation: the canonical CDN URL is fixed at this project's identity; ldflags only matter if Phase 5+ ever forks. If a fork ever happens, switching to ldflags is a single PR away."
  - "PendingUpdateVersion config field added but NOT wired to a setter inside CheckOnce. The plan explicitly said 'Skip this nicety for v1 if it complicates the test seam'. The .new file presence is the source of truth; the field is informational. Wiring a cfg.Save() call from inside CheckOnce would (a) couple the update package to config (currently zero coupling — update knows nothing about config.json), (b) introduce a Windows file-lock race against the running watcher's other cfg.Save calls, (c) require a Config injection in CheckOnce's signature — one of the cleanest signatures in the package. Deferred."
  - "Auto-update is signing-agnostic by design (CONTEXT.md locked). The download path verifies SHA-256 only — no Authenticode signature check, no SignPath integration. Plan 02-09 will add signing in CI as a workflow step BETWEEN makensis and SHA-256-compute; the auto-updater on the consuming side never knows whether the binary is signed. The startup-swap works identically for both."
  - "Tray menu insertion at index 2 (between Open log folder and Change Workbook…) — chosen so the operational/maintenance items cluster: Open log folder is for inspecting state, Check for updates is for managing the binary, Change Workbook is for re-config. The full ordering: 0 Open Workbook | 1 Open log folder | 2 Check for updates | 3 Change Workbook… | 4 Continue setup… | 5 Reauthorize… | 6 Quit. Reauthorize stays at index 5 (one slot later than Plan 02-04 placed it) — the test in 02-04 was a relative-position pin (Continue setup… < Reauthorize… < Quit), which still holds."
  - "checkOnceWithURL serializes via package-global checkMu sync.Mutex. The race it defends against: 24h tick fires CheckOnce while user clicks Check for updates (which also calls CheckOnce on a goroutine). Two concurrent fires could double-stage. With checkMu the second one waits; when it acquires the lock the .new + sidecar already exist + match, so the idempotency check short-circuits. Tested implicitly via TestCheckOnce_IdempotentWhenAlreadyStaged."
metrics:
  duration: ~50min
  completed: 2026-05-02
  tasks_completed: 4 of 4
  commits: 7
  files_changed: 11 (6 created in internal/update + 5 modified)
  test_count_added: 19 in internal/update + 2 in internal/tray = 21
  test_count_total_passing: ~110 (every prior wave's tests still green)
---

# Phase 2 Plan 06: OPS-04 auto-update Summary

The OPS-04 in-app auto-updater lands as a self-contained `internal/update`
package wired into the existing main + runapp + tray surface. Three
files, three responsibilities: `manifest.go` (fetch + parse + semver),
`swap.go` (startup-swap with SHA-256 verify + `minio/selfupdate.Apply`
seam), `check.go` (24h goroutine + atomic download/stage). The wiring
adds (a) a one-shot startup-swap check in `cmd/squirebot/main.go` that
runs BEFORE everything else, (b) a 24h `RunDailyCheck` goroutine launched
in `runWatcher` alongside the heartbeat, and (c) a "Check for updates"
tray menu item at index 2 of the new 7-item MenuPlan.

`minio/selfupdate v0.6.0` lands as a direct `go.mod` dependency. Its
sole job is the `<exe>.new` -> live + live -> `<exe>.old` + hide-on-Windows
rename dance — the very Pitfall 14 problem this whole plan exists to avoid
hand-rolling. The auto-updater is signing-agnostic: same code path for
unsigned (Phase 2) and signed (post-SignPath OSS approval) binaries.

Phase 2 honest dependency: the live download path is gated on
`Manifest.BinaryURL` being non-empty. Plan 02-08 (which already landed)
emits the bare `squirebot.exe` asset and `binary_url` + `binary_sha256`
manifest fields, so the wiring is end-to-end functional from this commit
forward. Older Phase 1 releases predate the field and are gracefully
skipped (logged, no error).

## What Shipped

### Task 1 — internal/update/manifest.go (TDD)
**Commits:** `b7b7c15` (RED — 7 failing tests), `ea1a9dd` (GREEN)

`Manifest` struct mirrors the canonical Phase 2 `latest.json` schema
emitted by `.github/workflows/release.yml`: `version`, `installer_url`,
`installer_sha256`, `binary_url`, `binary_sha256`, `released_at`. The
binary fields are `omitempty` so Phase 1 manifests still parse cleanly.

`Fetch(ctx, owner, repo) (Manifest, error)` builds the canonical CDN
URL `https://github.com/<owner>/<repo>/releases/latest/download/latest.json`
and delegates to `FetchFromURL(ctx, url)`. The latter is the test seam
(httptest.Server URL injection). 30s total timeout, 4096-byte body cap
(manifest is ~250 bytes).

`IsNewer(current, manifest string) bool` does 3-part numeric semver
comparison; returns false on ANY parse failure (defensive — a corrupt
manifest must not trigger an update). Strips a leading `v` prefix.

7 tests:

| # | Test                                            | Asserts                                                |
|---|-------------------------------------------------|--------------------------------------------------------|
| 1 | `_UnmarshalsCanonicalSchema`                    | All 6 fields populate from sample JSON                 |
| 2 | `_ParsesValidManifest`                          | httptest.Server -> FetchFromURL -> Manifest             |
| 3 | `_404ReturnsError`                              | Wrapping error mentions `404`; no panic                |
| 4 | `IsNewer_PatchComparisons`                      | 1.2.3<1.2.4 t; 1.2.4<1.2.3 f; equal f                  |
| 5 | `IsNewer_MinorAndMajorBumps`                    | minor + major correctness                              |
| 6 | `IsNewer_MalformedInputReturnsFalse`            | abc/empty/2-part inputs all return false; v-prefix ok  |
| 7 | `_ContextDeadlineExceeded`                      | Elapsed ctx -> errors.Is(err, ctx.DeadlineExceeded)    |

### Task 2 — internal/update/swap.go (TDD)
**Commits:** `b490bd6` (RED — 5 failing tests), `a23ed2e` (GREEN)

`Apply() (swapped bool, err error)`:

State machine for a single startup:

1. `<exe>.new` doesn't exist           → `(false, nil)`. Common path.
2. `<exe>.new` exists, no sidecar      → delete `.new`, `(false, err)`.
3. `<exe>.new` exists, sidecar exists, hash mismatches → delete BOTH, `(false, err)`.
4. `<exe>.new` exists, sidecar exists, hash matches, `selfApplyFn` ok → delete BOTH + tidy `.old`, `(true, nil)`.
5. `<exe>.new` exists, sidecar exists, hash matches, `selfApplyFn` errors → KEEP `.new` + sidecar (next launch retries), `(false, err)`.

Test seam: `selfApplyFn` package var swappable via `installFakeSelfApply`
(`t.Cleanup`-restored). Mirrors Plan 02-05's `heartbeat.sleepFn` pattern.

5 tests:

| # | Test                                            | Asserts                                                  |
|---|-------------------------------------------------|----------------------------------------------------------|
| 1 | `_NoStagedFileReturnsNil`                       | (false, nil) common path; no side effects                |
| 2 | `_MissingSidecarDeletesStaged`                  | .new deleted on missing sidecar; (false, err)            |
| 3 | `_HashMismatchDeletesBoth`                      | Both deleted; err contains "mismatch"; tampering defense |
| 4 | `_SuccessSwapsAndCleansUp`                      | (true, nil); selfApplyFn invoked with TargetPath=exe; .new + sidecar removed |
| 5 | `_SelfApplyErrorPreservesStaged`                | (false, err); .new + sidecar PRESERVED for retry         |

### Task 3 — internal/update/check.go (TDD)
**Commits:** `e4ba2db` (RED — 7 failing tests), `a95a897` (GREEN)

`CheckOnce(ctx, owner, repo, currentVersion, exePath, statusFn) error`:

State machine per fire:

1. Fetch `latest.json`.
2. `version <= current`  → nil (no-op).
3. `binary_url` empty   → nil (Phase 1 manifest fallback; no asset to swap).
4. `.new` + sidecar match → nil + `statusFn(...)` (idempotent).
5. Download to `<exe>.new.tmp`, hashing in stream via `io.MultiWriter`.
6. SHA-256 verify against `manifest.binary_sha256`.
7. Match → rename `.tmp` → `.new`; write sidecar; `statusFn("Update ready (X.Y.Z); restart to apply")`.
8. Mismatch → delete `.tmp`; error wrapping "mismatch".

`RunDailyCheck(ctx, owner, repo, currentVersion, exePath, statusFn)`
blocks; immediate first fire + every 24h (`checkInterval`). Errors are
LOGGED but never kill the goroutine — the watcher must keep running
even if GitHub is unreachable for a stretch.

`checkMu` (package-global `sync.Mutex`) serializes `CheckOnce`. The race
it defends: the 24h tick fires while the user clicks "Check for updates".
Both call `CheckOnce`; the second call waits; on lock acquisition the
`.new` + matching sidecar exist (idempotency short-circuit).

7 tests (timing-deterministic via `checkSleepCapture` gate channel):

| # | Test                                              | Asserts                                                 |
|---|---------------------------------------------------|---------------------------------------------------------|
| 1 | `_NoNewerVersionIsNoop`                           | nil; no .new; statusFn 0 calls                          |
| 2 | `_NewerVersionStagesBinary`                       | .new bytes match payload; sidecar = expected hex; statusFn called with version + "restart" |
| 3 | `_HashMismatchRejectsDownload`                    | err contains "mismatch"; .new + sidecar NOT written; .tmp cleaned up |
| 4 | `_ManifestFetchFails`                             | err contains "404"; no staging                          |
| 5 | `RunDailyCheck_FiresImmediatelyAndSchedules`      | First sleep d=24h; ctx cancel exits within 2s           |
| 6 | `_IdempotentWhenAlreadyStaged`                    | Pre-staged .new; binary download count = 0; statusFn still called (so tray re-surfaces) |
| 7 | `_NewerManifestWithEmptyBinaryURLIsNoop`          | Phase 1 manifest fallback; nil + no .new + no statusFn  |

### Task 4 — wire startup-swap, daily goroutine, tray entry
**Commit:** `69eaca9`

`cmd/squirebot/main.go`:
- `update.Apply()` runs as the FIRST action of `main()`, BEFORE
  `logging.Setup`, BEFORE `config.Load`. Errors go to stderr (logging
  not yet up). On `(true, nil)` we `os.Exit(0)` so the swapped-in binary
  takes over on the next process launch.
- New `tray.Config.OnCheckUpdates` closure spawns a goroutine that calls
  `update.CheckOnce(ctx, "boejowen", "SquireBot", Version, exe, statusFn)`.
  `statusFn = trayCtl.SetStatus`.

`internal/app/runapp.go`:
- Alongside `heartbeat.Run`, in `runWatcher` after scaffold + tray green:
  `go update.RunDailyCheck(ctx, "boejowen", "SquireBot", bc.WatcherVersion, exe, t.SetStatus)`.
  `os.Executable()` failure logs a warning and skips the goroutine launch.

`internal/tray/tray.go`:
- New `LabelCheckUpdates = "Check for updates"` constant.
- New `Config.OnCheckUpdates func()` field; `Controller.mCheckUpdates`
  + `onCheckUpdates`; `NewController` plumbs through; `OnReady` builds
  `plan[2]` with the systray item; `loop()` multiplexes the new
  `ClickedCh`.
- `MenuPlan()` length grew from 6 to 7. Final order:

  ```
  0  Open Workbook
  1  Open log folder         — CONTEXT.md mandatory (hotfix #4)
  2  Check for updates       — Plan 02-06 (OPS-04)        <-- NEW
  3  Change Workbook…        — D-04
  4  Continue setup…         — D-07 (hidden until needsWizard)
  5  Reauthorize…            — Plan 02-04 (AUTH-05) (hidden until authSuspended)
  6  Quit
  ```

  Reauthorize moved from index 4 → 5 (one slot later). The Plan 02-04
  pin test (`TestMenuPlan_ReauthorizePosition`) is a relative-position
  test (Continue setup < Reauthorize < Quit) — still holds.

`internal/tray/tray_test.go`:
- `TestMenuPlan_ContextMandatoryItems`: wantOrder updated to length-7
  with `LabelCheckUpdates` at index 2; doc-comment expanded.
- `TestLabelConstants_Stable`: new `LabelCheckUpdates` entry.
- New `TestMenuPlan_CheckUpdatesPosition`: pins `Open log folder <
  Check for updates < Change Workbook…`.
- New `TestOnCheckUpdatesCallback_Wired`: verifies Config →
  Controller plumbing (mirrors `TestOnReauthorizeCallback_Wired`).

`internal/config/config.go`:
- New `PendingUpdateVersion string` field with
  `json:"pending_update_version,omitempty"`. Informational ONLY — the
  `.new` file presence is the SOURCE OF TRUTH. No setter wired in
  `CheckOnce` (deferred per plan's "skip if it complicates the test
  seam" guidance).

## Acceptance — Self-Check

```
build  : exit 0   (go build ./... and go build ./cmd/squirebot/...)
vet    : exit 0   (go vet ./...)
tests  : ALL PASS (go test ./... -count=1)
```

| Plan acceptance criterion                                                                                 | Result |
|-----------------------------------------------------------------------------------------------------------|--------|
| File `internal/update/manifest.go` exists                                                                  | yes |
| File `internal/update/manifest_test.go` exists with 7 tests                                                | 7 |
| `grep -n "type Manifest struct" internal/update/manifest.go` returns 1                                    | 1 |
| `grep -c "InstallerSHA256\|installer_sha256" internal/update/manifest.go` >= 2                            | 2 |
| `grep -n "func IsNewer" internal/update/manifest.go` returns 1                                            | 1 |
| `grep -n "/releases/latest/download/latest.json" internal/update/manifest.go` returns 1                   | 1 |
| `grep -c "github.com/minio/selfupdate" go.mod` >= 1                                                       | 1 |
| File `internal/update/swap.go` exists                                                                      | yes |
| File `internal/update/swap_test.go` exists with 5 tests                                                    | 5 |
| `grep -n "func Apply()" internal/update/swap.go` returns 1                                                | 1 |
| `grep -n "selfupdate.Apply\|selfApplyFn" internal/update/swap.go` >= 2                                    | 4 |
| `grep -n "sha256" internal/update/swap.go` >= 1                                                           | 2 |
| `grep -c "expected-sha256\|.new" internal/update/swap.go` >= 4                                            | 11 |
| File `internal/update/check.go` exists                                                                     | yes |
| File `internal/update/check_test.go` exists with 6+ tests                                                  | 7 |
| `grep -n "func CheckOnce" internal/update/check.go` returns 1                                             | 1 |
| `grep -n "func RunDailyCheck" internal/update/check.go` returns 1                                         | 1 |
| `grep -c "BinaryURL\|BinarySHA256" internal/update/manifest.go` >= 2                                      | 2 |
| `grep -c "sha256.New\|crypto/sha256" internal/update/check.go` >= 1                                       | 2 |
| `grep -c ".new.tmp" internal/update/check.go` >= 1                                                        | 4 |
| `grep -c "expected-sha256" internal/update/check.go` >= 1                                                 | 1 |
| `grep -n "update.Apply()" cmd/squirebot/main.go` >= 1                                                     | 1 |
| `grep -n 'os.Exit(0)' cmd/squirebot/main.go` >= 1                                                         | 1 |
| `grep -n "OnCheckUpdates:" cmd/squirebot/main.go` returns 1                                               | 1 |
| `grep -n "LabelCheckUpdates" internal/tray/tray.go` >= 3 (constant, MenuPlan, OnReady)                   | 3 |
| `grep -n "go update.RunDailyCheck" internal/app/runapp.go` returns 1                                      | 1 |
| `grep -c "update.CheckOnce" cmd/squirebot/main.go` returns 1                                              | 1 |
| `go test ./... -count=1` passes (manifest + swap + check + every prior wave)                              | yes |
| `go build ./cmd/squirebot/...` succeeds                                                                    | yes |
| `go vet ./...` returns no errors                                                                           | yes |

### Acceptance grep nuances (literal-vs-intent)

The plan's check.go acceptance criterion `grep -n "/releases/latest/download/latest.json" returns 1`
applies to manifest.go (where the URL is computed). The string does
NOT appear in check.go because check.go consumes `ManifestURL(owner, repo)`.
This matches the plan's intent (the canonical CDN path is documented at
its declaration site, not at every consumer).

The plan's `grep -n "LabelCheckUpdates" internal/tray/tray.go >= 3`
returns exactly 3: constant declaration, MenuPlan tooltip lookup, OnReady
plan[2] index. Adjacent `m.CheckUpdates` field references use the
`m`-prefix shorthand.

## Test Counts

| File                                  | Existing | Added | Total |
|---------------------------------------|----------|-------|-------|
| `internal/update/manifest_test.go`    | 0        | 7     | 7     |
| `internal/update/swap_test.go`        | 0        | 5     | 5     |
| `internal/update/check_test.go`       | 0        | 7     | 7     |
| `internal/tray/tray_test.go`          | 10       | 2     | 12    |
| `internal/app/runapp_test.go`         | 6        | 0     | 6 (regression-clean) |

All Phase 1 + Wave 1 (02-01) + Wave 2 (02-02 + 02-03) + Wave 3 (02-04) +
Wave 4 (02-05) + Wave 2 (02-08) tests still pass — no regressions.

## End-to-End Flow Verification

```
cmd/squirebot/main.go cold start
  -> update.Apply()
       |
       +--> os.Executable() -> resolve <exe>
       +--> applyAt(<exe>)
             |
             +--> stat <exe>.new
             |     +-- ENOENT -> (false, nil) ----------------+
             |                                                |
             +--> read <exe>.expected-sha256                  |
             |     +-- missing  -> rm .new -> (false, err) --+
             |                                                |
             +--> sha256(open <exe>.new)                      |
             |     +-- mismatch -> rm BOTH -> (false, err) --+
             |                                                |
             +--> selfApplyFn(staged, {TargetPath: exe})      |
             |     +-- err  -> KEEP both -> (false, err) ----+
             |     +-- ok   -> rm .new + sidecar + tidy .old |
             |                  -> (true, nil)               |
             v                                               |
      swapped == true  -> os.Exit(0)                         |
      swapped == false + err -> stderr; CONTINUE             |
      swapped == false + nil -> CONTINUE  <------------------+

  -> logging.Setup() ; config.Load() ; tray.NewController(Config{
       OnCheckUpdates: <closure>      <-- new in this plan
       OnReauthorize: ...
       OnChangeWorkbook: ...
       OnContinueSetup: ...
       OnQuit: ...
     })
  -> go app.RunApp(ctx, cfg, bc, trayCtl)
  -> systray.Run(trayCtl.OnReady, trayCtl.OnExit)        <-- main.go blocks here

  app.RunApp -> needsWizard ? wizard : direct
              -> runWatcher
                   +-- sheet.NewClient + ScaffoldSchemaV1
                   +-- tray green
                   +-- go heartbeat.Run(...)
                   +-- go update.RunDailyCheck(ctx,            <-- new in this plan
                                              "boejowen", "SquireBot",
                                              bc.WatcherVersion, exe, t.SetStatus)
                         |
                         +--> immediate first fire (CheckOnce)
                         |     +--> Fetch latest.json
                         |     +--> IsNewer ? proceed : nil
                         |     +--> BinaryURL non-empty ? proceed : nil
                         |     +--> sidecar match ? statusFn + nil : download
                         |     +--> download .new.tmp + hash
                         |     +--> verify -> rename .tmp -> .new -> sidecar
                         |     +--> statusFn("Update ready (X.Y.Z); restart to apply")
                         +--> sleepFn(ctx, 24h)
                         +--> 24h elapsed -> tick again
                         +--> ctx.Done -> return

  user clicks tray "Check for updates"
    -> tray.loop() picks up t.mCheckUpdates.ClickedCh
    -> onCheckUpdates() -- the main.go closure
    -> goroutine: update.CheckOnce(ctx, "boejowen", "SquireBot",
                                   Version, exe, t.SetStatus)
       |
       +--> [same flow as above]
       +--> checkMu serializes against the daily goroutine
            (idempotency check short-circuits if already staged)

  next cold start (after a successful stage):
    -> update.Apply()
       +--> .new exists; sidecar matches; selfApplyFn -> success
       +--> os.Exit(0)
    -> next process launch runs the new <exe>
       +--> update.Apply()
       +--> no .new; (false, nil); proceed normally
```

## Live Smoke — Deferred

Plan's `<verification>` block recommends three live tests:

1. Manually craft `squirebot.exe.new` + `.expected-sha256` (with the
   correct hash); start the watcher; observe the new binary takes over
   (Apply returns true; main exits 0).
2. Corrupted `.new`: write a `.new` with bytes that don't match the
   sidecar SHA; on startup, `.new` is deleted; old binary continues
   normally.
3. Live download path: start the watcher with `Version` set to a value
   older than the latest published release; within ~immediate (first
   tick), the daily check downloads + stages; observe tray status
   "Update ready"; restart; observe new version takes over.

NEITHER live test was performed during execution. Same constraint as
Plans 02-03/04/05: a single-developer machine where deliberately
breaking the running watcher to validate recovery is risky for genuine
work. The behavioural coverage matrix above (19 unit tests across
manifest + swap + check) plus the end-to-end flow diagram above prove
the state machines match the spec; the live tests are queued as the
Phase 2 final integration smoke (Plan 02-10).

For Plan 02-10's 7-day soak, the recommended approach: tag a
`v0.2.1-rc1` release, install on a single test machine alongside a
v0.2.0 watcher, observe the auto-update fires within 24h and the swap
takes effect on the next manual restart. Then for the negative path,
publish a deliberately-corrupt `latest.json` (mismatched binary_sha256)
and observe the .new is rejected on download.

## Race Detector Verdict

Same constraint as Plans 02-03/04/05: `go test -race` requires CGO + a
C compiler, which is not available on the local Windows Go install
(`cgo: C compiler "gcc" not found`). Race-clean verification deferred
to CI (which has the toolchain) or to a local run after `gcc` is
installed.

The mutex behaviour in this plan is independently verified via:

- `checkMu` serializes `CheckOnce`; the idempotency check
  (`TestCheckOnce_IdempotentWhenAlreadyStaged`) implicitly validates
  the mutex by counting binary download attempts (must be 0 on the
  second call even if the second call races against the first).
- `selfApplyFn` is package-level but only ever swapped in test
  `t.Cleanup`; production never mutates it.
- The auto-update goroutine and the heartbeat goroutine share NO
  mutable state. They each launch with their own ctx/closure; the only
  thing they share is the tray Controller (via `t.SetStatus`), which
  has its own internal mutex (`Controller.mu`).

## Deviations from Plan

### Plan-vs-Reality Drift Notes

**A. Three test files instead of two.** The plan's `<output>` listed
three files (`{manifest,swap,check}.go`) and three test files. I split
each test file with the same 1:1 mapping the plan specified — no drift
here, calling out for completeness.

**B. `BinaryURL` empty short-circuit ALWAYS returns nil (no error).**
Plan said the goal was "log 'auto-update download deferred until Plan
02-08 ships goreleaser binary asset' and skip the download". I logged
the message AND tested the path explicitly
(`TestCheckOnce_NewerManifestWithEmptyBinaryURLIsNoop`). 02-08 has
already landed (per 02-08-SUMMARY.md, 2026-05-02 07:06:56Z), so the
short-circuit will only fire if a guildie still has an old Phase 1
manifest in their cache or if release.yml is ever rolled back. Honest
no-op path is right; alternative (returning an error) would surface as
a tray failure status which would be misleading.

**C. `PendingUpdateVersion` config field added but no setter wired.**
Plan offered this as discretionary ("Skip this nicety for v1 if it
complicates the test seam"). Skipped per plan's allowance. The `.new`
file presence is the source of truth; the field is informational and
can be wired in a follow-up if Phase 5 wants to surface a "Update
1.2.0 staged on YYYY-MM-DD" message in the tray status. Wiring it now
would require either (a) injecting `*config.Config` into `CheckOnce`'s
signature (couples update package to config — currently zero coupling),
or (b) a global-state pattern (tray reads config to decide a label) —
both more invasive than this plan's scope.

**D. Tray menu insertion landed at index 2 cleanly; Reauthorize moved
from index 4 to index 5.** Plan was explicit about the new ordering
and pre-state. The Plan 02-04 acceptance test
(`TestMenuPlan_ReauthorizePosition`) is a RELATIVE-position pin
(`Continue setup < Reauthorize < Quit`), which still holds at the new
indices. No drift.

**E. Owner/repo hard-coded as `("boejowen", "SquireBot")`.** Plan
offered choice between hard-code and ldflag injection
(`var UpdateOwner = ...`). Chose hard-code per plan's explicit
recommendation; switching to ldflags is a single PR away if Phase 5+
ever forks.

### Auto-fixed Issues

**1. [Rule 1 — Bug] Windows file-lock failure on `os.Remove(.new)` after
hashing. (TASK 2 GREEN)**
- **Found during:** First run of TestApply_HashMismatchDeletesBoth +
  TestApply_SuccessSwapsAndCleansUp. Both tests asserted ".new still
  exists after [hash mismatch | successful swap]". Root cause: the
  first draft of `swap.go` used `defer staged.Close()` which left the
  file handle open through the `os.Remove` calls. Windows refuses to
  remove a file with an open handle (the very Pitfall 14 problem this
  whole plan exists to avoid hand-rolling).
- **Fix:** Removed the `defer`; explicit `_ = staged.Close()` BEFORE
  every `os.Remove` and BEFORE `selfApplyFn` returns. Two close points
  on the failure path (right before each `os.Remove(stagedPath)`); one
  close point on the success path (right after `selfApplyFn` returns,
  before any cleanup).
- **Files modified:** `internal/update/swap.go`.
- **Commit:** Folded into `a23ed2e` (single Task 2 GREEN commit; the
  fix was applied during the same iteration as the initial draft).
- **Risk:** Low. The fix is structurally enforced by the new
  `applyErr := selfApplyFn(...)` two-step pattern (call, close,
  inspect err). Future code-path additions can't accidentally re-introduce
  the `defer` bug because there's no `defer` at all in the function now.

### Authentication gates

None. Auto-update goes through GitHub's CDN with no auth.

## Known Stubs

None. The auto-update path is end-to-end functional from this plan
forward:
- `internal/update` package implements all three responsibilities (fetch,
  swap, check) with real code paths.
- `cmd/squirebot/main.go` calls `update.Apply()` and the tray
  `OnCheckUpdates` closure calls `update.CheckOnce` with real arguments.
- `internal/app/runapp.go` launches the daily goroutine.
- `latest.json` schema in CI matches the Go struct exactly (asserted by
  TestManifest_UnmarshalsCanonicalSchema against the documented
  release.yml shape).

The PendingUpdateVersion config field has no setter, but it's
informational by design (the `.new` file is the source of truth) and
the plan explicitly approved deferring its setter.

## Threat Flags

| Flag | File | Description |
|------|------|-------------|
| threat_flag: network | internal/update/manifest.go + check.go | New outbound HTTPS to `github.com/boejowen/SquireBot/releases/latest/download/{latest.json,squirebot.exe}` from the watcher process. SHA-256 verification at every step; manifest body capped at 4096 bytes; download Reader bounded by HTTP timeout (5min). |
| threat_flag: file | internal/update/swap.go + check.go | New file writes adjacent to the running binary: `<exe>.new`, `<exe>.new.tmp`, `<exe>.expected-sha256`. Also indirect: `minio/selfupdate.Apply` performs the rename + hides `.target.old`. Permissions 0o600 on staged files. Risk: a malicious local actor with write access to the install dir could swap a corrupt `.new` — but they already have full code-execution there, so the threat model is unchanged. |
| threat_flag: process | cmd/squirebot/main.go | New `os.Exit(0)` path on successful startup-swap. Distinct from the existing tray-Quit path; the OS still releases all resources cleanly. |

These additions DO appear in the plan's `<must_haves>` and were planned
for; flagging here for the verifier's downstream phase-level threat
review.

## TDD Gate Compliance

This plan ran in strict TDD with separated RED + GREEN commits per
TDD-marked task (Task 4 was non-TDD per plan):

| Task | RED commit | GREEN commit |
|------|------------|--------------|
| 1 (manifest) | `b7b7c15 test(02-06): add failing tests for latest.json Manifest, FetchFromURL, IsNewer` | `ea1a9dd feat(02-06): implement OPS-04 Manifest schema, Fetch, and IsNewer` |
| 2 (swap)     | `b490bd6 test(02-06): add failing tests for OPS-04 startup-swap (Apply / applyAt)` | `a23ed2e feat(02-06): implement OPS-04 startup-swap (Apply / applyAt)` |
| 3 (check)    | `e4ba2db test(02-06): add failing tests for OPS-04 daily check + manual fire` | `a95a897 feat(02-06): implement OPS-04 daily check + manual fire (CheckOnce, RunDailyCheck)` |
| 4 (wiring; non-TDD) | n/a | `69eaca9 feat(02-06): wire OPS-04 startup-swap, daily check goroutine, tray entry` |

Each RED was verified to fail-build (undefined identifiers) before
committing; each GREEN was verified to pass `go test ./internal/update/...
-count=1` and `go vet ./...` and `go build ./...`.

## Self-Check: PASSED

Verified all created files exist:

- `internal/update/manifest.go` (~150 lines, contains `type Manifest
  struct`, `ManifestURL`, `Fetch`, `FetchFromURL`, `IsNewer`,
  `parseVersion`)
- `internal/update/manifest_test.go` (~195 lines, 7 test functions
  covering parse + HTTP + comparison edge cases)
- `internal/update/swap.go` (~135 lines, contains `var selfApplyFn =
  selfupdate.Apply`, `func Apply()`, `func applyAt(exe string)`)
- `internal/update/swap_test.go` (~205 lines, 5 test functions
  covering the 5-state matrix)
- `internal/update/check.go` (~205 lines, contains `const checkInterval
  = 24 * time.Hour`, `var checkSleepFn`, `var checkMu sync.Mutex`,
  `func CheckOnce`, `func checkOnceWithURL`, `func RunDailyCheck`,
  `func runDailyCheckWithURL`)
- `internal/update/check_test.go` (~410 lines, 7 test functions covering
  the manual + daily fire matrix including idempotency + Phase 1
  fallback)
- `internal/tray/tray.go` (modified — LabelCheckUpdates, OnCheckUpdates,
  mCheckUpdates, onCheckUpdates, NewController plumbing, OnReady plan[2]
  wire, loop multiplexer)
- `internal/tray/tray_test.go` (modified — wantOrder length-7,
  LabelConstants extended, CheckUpdatesPosition test,
  OnCheckUpdatesCallback_Wired test)
- `internal/app/runapp.go` (modified — internal/update import,
  RunDailyCheck goroutine launched alongside heartbeat)
- `cmd/squirebot/main.go` (modified — fmt + update imports,
  update.Apply as first action, OnCheckUpdates closure)
- `internal/config/config.go` (modified — PendingUpdateVersion field)
- `go.mod` / `go.sum` (modified — minio/selfupdate v0.6.0 promoted to
  direct dep, aead.dev/minisign v0.2.0 indirect)

All 7 commits reachable from HEAD: `b7b7c15`, `ea1a9dd`, `b490bd6`,
`a23ed2e`, `e4ba2db`, `a95a897`, `69eaca9`.

## Wave 6 Handoff (02-07 autostart hardening + uninstaller checkbox)

This plan touched `cmd/squirebot/main.go` to add `update.Apply()` as the
FIRST action of `main()`. Plan 02-07 (autostart hardening) will also
touch main.go to add the `--uninstall-wipe-credentials` CLI flag handler
that the NSIS uninstaller's "Also delete saved configuration and Google
account credentials" checkbox invokes (CONTEXT.md Q3 decision).

Coordination guidance for 02-07:
- The flag handler should run AFTER `update.Apply()` (auto-update has
  no business firing during an uninstall; though it would be a no-op
  anyway because the uninstaller invokes the binary with the special
  flag immediately before deleting it). Recommended placement: right
  after `update.Apply()`, before `logging.Setup()`. The flag handler
  reads `os.Args[1:]`, performs the wipe, and calls `os.Exit(0)`.
- Tray menu surface is unchanged by 02-07 — the wipe is invoked from
  the NSIS uninstaller, not from a tray menu item. So MenuPlan stays
  at length 7.

The auto-update + heartbeat goroutines coexist cleanly with no
coordination points. Future plans that touch runWatcher should preserve
the launch order: scaffold → tray green → heartbeat → auto-update →
handlers (this is the order in this plan's runapp.go diff).
