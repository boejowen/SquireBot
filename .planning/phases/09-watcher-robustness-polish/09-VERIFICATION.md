---
phase: 09-watcher-robustness-polish
verified: 2026-05-13T00:39:27Z
status: human_needed
score: 5/5 must-haves verified
overrides_applied: 0
human_verification:
  - test: "On-VM smoke of AUTH-07 boot-time invalid_grant on fresh v1.0.2 install"
    expected: "After revoking the refresh token in Google Account settings between sessions, restart the watcher: red tray icon AND visible Reauthorize menu item appear from boot — clicking Reauthorize reopens the OAuth flow and recovers without restart."
    why_human: "Requires real Google Account token revocation + real Windows tray UI + visual confirmation of menu state at the moment OnReady fires. Unit tests assert the classifier + tray-call sequence (TestApplyBootAuthError_Revoked, OPS-06 pre-Ready queue tests), but the live tray-icon-color + menu-item-visibility from boot can only be confirmed on a real Windows VM with a real revoked token."
  - test: "On-VM smoke of OPS-06 wincred fast-fail recovery on fresh v1.0.2 install"
    expected: "When buildTokenSourceFromWincred fails on boot (simulate by corrupting wincred or denying read access), the tray menu opens with a working state (red icon + Reauthorize or ContinueSetup visible) — NOT stuck at 'Initialising…' with no recovery path."
    why_human: "Requires real wincred corruption + real systray.OnReady ordering. Unit tests prove drainPending replays in FIFO order with the queue mechanism intact, but the real-Windows interleaving of fast-fail vs. OnReady firing can only be observed live."
  - test: "On-VM smoke of OPS-07 foreground-shell-close on fresh v1.0.2 install"
    expected: "Launch squirebot.exe from cmd.exe (NOT via Start-Process), then close the cmd window. The watcher process keeps running (visible in Task Manager / tray icon stays alive). No silent death."
    why_human: "FreeConsole detach behavior depends on the exact Win32 console-inheritance state of the parent shell; only a real cmd.exe / PowerShell session can confirm the detach actually severs the SIGHUP-equivalent on shell close."
  - test: "On-VM smoke of CONFIG-01 BOM-prefixed config.json on fresh v1.0.2 install"
    expected: "Hand-edit %LOCALAPPDATA%\\SquireBot\\config.json in Notepad (which writes a UTF-8 BOM by default), save, restart watcher. The watcher boots normally — no `invalid character 'ï' looking for beginning of value` error visible in the log file."
    why_human: "Unit test TestLoad_StripsUTF8BOM proves bytes.TrimPrefix behavior on a synthetic BOM-prefixed []byte, but only a real Notepad save can confirm Notepad's actual BOM-encoding behavior (vs. test fixture bytes) round-trips through Load() cleanly."
  - test: "v1.0.1 watcher → v1.0.2 auto-update smoke"
    expected: "An existing v1.0.1 watcher install picks up latest.json on its next periodic check, downloads SquireBot-Setup-1.0.2.exe (or squirebot.exe per OPS-04 self-update path), and successfully upgrades to v1.0.2 on next restart."
    why_human: "Requires a real v1.0.1 install on a Windows VM running long enough for the periodic update check to fire. The latest.json manifest is verified statically (version=1.0.2 + fresh SHA-256s + valid binary_url) but the actual minio/selfupdate Apply() round-trip from an in-the-field v1.0.1 watcher needs live observation."
---

# Phase 9: Watcher Robustness Polish — Verification Report

**Phase Goal:** Eliminate the 4 v1.0.1-UAT-surfaced foot-guns in the Go watcher and ship a clean v1.0.2 binary release that's the new recommended download for every guildie.

**Verified:** 2026-05-13T00:39:27Z
**Status:** human_needed (all programmatic checks passed; on-VM UAT remains the canonical user follow-up per Plan 09-05 / PATTERNS.md)
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Boot-time invalid_grant → red icon + visible Reauthorize from boot (AUTH-07) | VERIFIED | `internal/app/runapp.go:650-668` `applyBootAuthError` classifier uses `auth.IsRevokedRefreshToken(err)`; revoked branch calls `SetIconHealth(HealthRed)` + `SetStatus("Reauthorize: refresh token died. Click Reauthorize…")` + `ShowReauthorize()` — the exact AUTH-05 triple from `suspendForAuth` line 681 (verbatim match). Non-revoked branch preserves `ShowContinueSetup()` (line 666). Wired into `RunApp` at line 118 via `_ = applyBootAuthError(t, err)`. OPS-06 pre-Ready queue carries these calls into OnReady. Unit tests: `TestApplyBootAuthError_Revoked` + `TestApplyBootAuthError_NonRevoked` in `internal/app/runapp_test.go:238,254`. |
| 2 | RunApp fast-fail tray menu opens with state, not "Initialising…" forever (OPS-06) | VERIFIED | `internal/tray/tray.go`: `ready bool` (line 127), `pending []pendingAction` (line 128), `drainPending()` (line 165) called from `OnReady` at line 246. All 7 mutators (`SetStatus`, `SetIconHealth`, `Show/HideContinueSetup`, `Show/HideReauthorize`, `SetSpreadsheetID`) queue-or-execute under `t.mu` (lines 338-420). Tests: `TestPreReady_EnqueuesNotDrops`, `TestPreReady_FIFOOrder` in `internal/tray/tray_test.go:52,71`. |
| 3 | Foreground-shell-close no longer kills watcher (OPS-07) | VERIFIED | `cmd/squirebot/console_windows.go:48` `freeConsole()` helper via `windows.NewLazySystemDLL("kernel32.dll").NewProc("FreeConsole")` (LazySystemDLL fallback per Plan 09-02 deviation note — functionally equivalent to direct `windows.FreeConsole`, which isn't bound in x/sys/windows v0.43.0). Wired into `cmd/squirebot/main.go:109` between `update.Apply()` (line 91) and `logging.Setup()` (line 111) — exactly the documented ordering. `cmd/squirebot/console_other.go` provides the non-Windows no-op stub. Doc note also landed at `docs/build-and-install.md:383-390` "Foreground launch (cmd / PowerShell)" — above-the-fold per AUTH-07 acceptance option (b). |
| 4 | Notepad-written UTF-8-BOM config.json loads without `invalid character 'ï'` error (CONFIG-01) | VERIFIED | `internal/config/config.go:65` `data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})` placed between `os.ReadFile` (line 55) and `json.Unmarshal` (line 67) — exactly per plan. ≤5 LOC change. Unit test: `TestLoad_StripsUTF8BOM` in `internal/config/config_test.go:271` proves Load() round-trips BOM-prefixed bytes cleanly. |
| 5 | v1.0.2 binary tagged + published + latest.json refreshed (ship gate) | VERIFIED | Annotated tag `v1.0.2` at SHA `3bfa49163205f2b2ed7d52568751b525ff264a65` (git cat-file confirms tag object type). Remote: `git ls-remote --tags origin v1.0.2` returns the tag. GitHub Release v1.0.2 (`gh release view`): `isDraft=false`, 4 assets — `latest.json` (499 bytes), `SquireBot-Setup-1.0.2.exe` (4746331 bytes), `SquireBot-Setup.exe` (versionless alias, byte-identical SHA256 to versioned), `squirebot.exe` (16992768 bytes). `latest.json` content verified: `version=1.0.2`, `installer_sha256=b54adcf3…`, `binary_sha256=9dfb803c…`, `binary_url=…/v1.0.2/squirebot.exe`, `released_at=2026-05-13T00:31:52.1257485Z`, `signed=false`, `phase=2`. Workflow run 25770477750 documented green in 09-05-SUMMARY. |

**Score:** 5/5 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/tray/tray.go` | OPS-06 pre-Ready queue: ready bool + pending []pendingAction + drainPending | VERIFIED | All three symbols present (lines 127, 128, 165); drainPending called from OnReady line 246 and offlineDrainForTests line 459. |
| `cmd/squirebot/main.go` | OPS-07: freeConsole call between update.Apply and logging.Setup | VERIFIED | Line 109: `_ = freeConsole()` between Apply (line 91) and Setup (line 111). |
| `cmd/squirebot/console_windows.go` | OPS-07: freeConsole helper (LazySystemDLL fallback) | VERIFIED | Documented LazySystemDLL approach (kernel32 + NewProc) per Plan 09-02 deviation; functionally equivalent to direct windows.FreeConsole; safe-call contract returns nil when no console attached. |
| `cmd/squirebot/console_other.go` | OPS-07: non-Windows no-op stub | VERIFIED | Build-tag pair (`//go:build !windows`) exists per commit 4e3c8a2. |
| `internal/config/config.go` | CONFIG-01: bytes.TrimPrefix with [0xEF 0xBB 0xBF] between os.ReadFile and json.Unmarshal | VERIFIED | Line 65 trim between lines 55 (ReadFile) and 67 (Unmarshal). |
| `internal/app/runapp.go` | AUTH-07: applyBootAuthError helper using auth.IsRevokedRefreshToken; canonical status string verbatim; non-revoked preserves ContinueSetup | VERIFIED | Helper at line 650; revoked path uses identical "Reauthorize: refresh token died. Click Reauthorize…" string as suspendForAuth (lines 657 + 681 match verbatim); non-revoked path (line 666) calls ShowContinueSetup as originally. Wired at line 118. |
| `internal/sheet/client.go` | WatcherMaxSchemaVersion unchanged at 3 | VERIFIED | Per 09-05-SUMMARY Task 1 step C: `WatcherMaxSchemaVersion = 3` (1 match, exact). No schema impact in Phase 9. |
| v1.0.2 annotated tag | Phase 9 ship gate | VERIFIED | `git tag -l v1.0.2 -n5` returns annotated message; pushed to origin. |
| GitHub Release v1.0.2 | Published, not draft, with 4 artifacts including latest.json | VERIFIED | gh release view: isDraft=false, 4 assets, latest.json advertises version=1.0.2. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `RunApp` (runapp.go:118) | `applyBootAuthError` | direct call inside cold-start wincred-rebuild failure branch | WIRED | Single call site replaces the previous inline ContinueSetup-only block; both classifier branches reachable. |
| `applyBootAuthError` (runapp.go:652) | `auth.IsRevokedRefreshToken` | classifier call | WIRED | Imported as `"github.com/boejowen/SquireBot/internal/auth"` (same classifier AUTH-05 uses at runapp.go:631). |
| `applyBootAuthError` revoked branch | `tray.Controller.SetIconHealth/SetStatus/ShowReauthorize` | direct calls | WIRED | Status string exact-match with suspendForAuth (canonical AUTH-05 source). |
| Tray mutators (7 total) | `t.pending` queue | queue-or-execute under t.mu | WIRED | Each mutator inspects `t.ready`; if false, appends pendingAction; if true, executes inline. |
| `OnReady` (tray.go:216) | `drainPending` | direct call after `t.ready = true` | WIRED | Line 246 invocation; FIFO replay verified by TestPreReady_FIFOOrder. |
| `main.go:109` | `freeConsole()` | direct call | WIRED | Ordering correct: after Apply, before Setup; Windows-only via build-tag pair. |
| `config.Load` | `bytes.TrimPrefix` | direct call | WIRED | Line 65, between ReadFile (55) and Unmarshal (67). |
| GitHub Release v1.0.2 | `latest.json` | release asset upload by release.yml workflow run 25770477750 | WIRED | Asset present; content advertises v1.0.2 with fresh SHA-256s; existing v1.0.1 watchers' OPS-04 self-update will pick this up. |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|--------------|--------|---------------------|--------|
| `latest.json` (release asset) | `version`, `binary_sha256`, `installer_sha256`, `binary_url` | Generated by release.yml workflow from actual build artifacts | YES — version="1.0.2", real SHA-256s computed by CI, real download URLs | FLOWING |
| `applyBootAuthError` classification | `bootAuthClassification` enum | `auth.IsRevokedRefreshToken(err)` matches against real OAuth2 error shapes (Plan 02-04 Task 1) | YES — same classifier AUTH-05 uses for the running-state path | FLOWING |
| Tray `pending` queue | `[]pendingAction` | Real mutator invocations during pre-Ready window | YES — actionKind-tagged payloads carry SetStatus/Health/Show/Hide/SetID arguments through to OnReady drain | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Config BOM-strip unit test passes | `go test ./internal/config/...` | `ok github.com/boejowen/SquireBot/internal/config 2.700s` | PASS |
| Tray pre-Ready queue tests pass | `go test ./internal/tray/...` | `ok github.com/boejowen/SquireBot/internal/tray 2.208s` | PASS |
| AUTH-07 classifier tests pass | `go test ./internal/app/...` | `ok github.com/boejowen/SquireBot/internal/app 0.602s` | PASS |
| v1.0.2 tag exists locally and is annotated | `git cat-file -t v1.0.2` | `tag` (annotated, not lightweight) | PASS |
| v1.0.2 tag pushed to origin | `git ls-remote --tags origin v1.0.2` | `3bfa491… refs/tags/v1.0.2` | PASS |
| GitHub Release v1.0.2 is published (not draft) | `gh release view v1.0.2 --json isDraft` | `isDraft=false` | PASS |
| latest.json advertises version=1.0.2 | `gh release download v1.0.2 --pattern latest.json && cat` | `{"version":"1.0.2", "installer_sha256":"b54adcf3…", "binary_sha256":"9dfb803c…", "binary_url":"…/v1.0.2/squirebot.exe", "signed":false, "phase":2, …}` | PASS |
| All 4 release artifacts uploaded | `gh release view v1.0.2 --json assets` | latest.json (499 B), SquireBot-Setup-1.0.2.exe (4.7 MB), SquireBot-Setup.exe (4.7 MB, same SHA256), squirebot.exe (17 MB) — all `state=uploaded` | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| AUTH-07 | 09-04 | Boot-time invalid_grant → red icon + visible Reauthorize from boot | SATISFIED | `applyBootAuthError` helper in runapp.go:650 with revoked-branch tray triple verbatim-matching AUTH-05; wired via runapp.go:118; OPS-06 queue carries the calls into OnReady; 2 unit tests pass. |
| OPS-06 | 09-01 | Pre-Ready tray calls queued and replayed in-order once OnReady fires | SATISFIED | `ready bool` + `pending []pendingAction` + `drainPending()` in tray.go; all 7 mutators queue-or-execute under t.mu; drainPending called from OnReady line 246; FIFO + post-Ready tests pass. |
| OPS-07 | 09-02 | Foreground-launched watcher MUST NOT die silently when parent shell closes | SATISFIED | `freeConsole()` via LazySystemDLL in console_windows.go:48 (acceptance option a); called from main.go:109 between Apply and Setup. Doc note also present in docs/build-and-install.md:383 (belt-and-suspenders covers acceptance option b). |
| CONFIG-01 | 09-03 | `config.Load` strips leading UTF-8 BOM before json.Unmarshal | SATISFIED | `bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})` at config.go:65 between ReadFile and Unmarshal; TestLoad_StripsUTF8BOM passes. |

No orphaned requirements: Phase 9 declares [AUTH-07, OPS-06, OPS-07, CONFIG-01] in REQUIREMENTS.md traceability table (lines 55-58); all four are claimed by plans 09-01 through 09-04, and Plan 09-05 is the ship-gate (no requirement claim, ships the result).

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (none) | — | — | — | No TODO/FIXME/PLACEHOLDER markers, no stub returns, no console.log-only handlers in Phase 9 source files. The `_ = freeConsole()` swallowed error in main.go:109 is intentional and documented (Plan 09-02 deviation note — failure surfaces via slog Warn inside the helper itself; cold-path detach failure is informational only). |

### Human Verification Required

5 items need human testing on a Windows VM with a fresh v1.0.2 install. The full UAT script lives in `09-PATTERNS.md` (per Plan 09-05). See frontmatter `human_verification:` for the 5 specific scenarios:

1. AUTH-07: revoke-between-sessions → red icon + Reauthorize visible from boot
2. OPS-06: wincred fast-fail → tray menu opens with state, not stuck at "Initialising…"
3. OPS-07: cmd.exe foreground launch → close shell → watcher keeps running
4. CONFIG-01: Notepad-edit config.json (writes BOM by default) → watcher boots clean
5. Auto-update: v1.0.1 watcher → v1.0.2 via latest.json round-trip

All five are listed in the YAML frontmatter `human_verification:` block with test, expected, and why_human fields.

### Gaps Summary

No programmatic gaps. All 5 ROADMAP Success Criteria have direct codebase evidence:

- 4 of 4 requirements (AUTH-07, OPS-06, OPS-07, CONFIG-01) have implementation + unit tests + correct wiring + intact data flow
- v1.0.2 annotated tag exists locally, is pushed to origin, points at master HEAD ab6da3f which contains all 4 fix merges
- GitHub Release v1.0.2 is published (not draft) with all 4 expected assets uploaded; latest.json content advertises version=1.0.2 with fresh SHA-256s and correct download URLs
- All three targeted test packages (config, tray, app) pass green
- Schema invariant preserved (`WatcherMaxSchemaVersion = 3`, unchanged)

The phase goal — "ship a clean v1.0.2 binary release that's the new recommended download for every guildie" — is achieved at the artifact level. Status is `human_needed` (not `passed`) solely because the ROADMAP acceptance criteria are written in terms of guildie-visible behavior (e.g., "sees a red tray icon AND a visible Reauthorize menu item from boot"), and those behaviors can only be confirmed live on a real Windows VM. The canonical follow-up is the on-VM UAT script in 09-PATTERNS.md, which Plan 09-05 explicitly designated as user follow-up (Task 3, out-of-scope for the agent).

---

_Verified: 2026-05-13T00:39:27Z_
_Verifier: Claude (gsd-verifier)_
