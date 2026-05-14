---
phase: 02-watcher-robustness-schema-lock
plan: 07
subsystem: installer + autostart + uninstaller-UX
tags: [inst-04, autostart, uninstaller, wincred, dpapi, hkcu-run, nsis, message-box, cli-flag]
requires:
  - 02-06 (update.Apply is the FIRST action of main(); the new --uninstall-wipe-credentials handler is inserted BEFORE update.Apply so the uninstall path can't accidentally fire an auto-update)
  - Phase 1 plan 01-08 (NSIS installer skeleton — squirebot.nsi)
  - Phase 1 plan 03 OAuth (auth.DeleteToken + StoredToken contract — already shipped)
provides:
  - INST-04 (Windows logon autostart via HKCU\\...\\Run)
  - --uninstall-wipe-credentials CLI flag (delegated wincred wipe from NSIS)
  - NSIS uninstaller wipe-or-preserve prompt (CONTEXT.md Q3, default = preserve)
affects:
  - installer/squirebot.nsi (modified — Run-key write/delete + MessageBox prompt + conditional ExecWait)
  - cmd/squirebot/main.go (modified — flag handler at TOP of main, before update.Apply)
  - docs/build-and-install.md (modified — Uninstalling section rewritten)
tech-stack:
  added:
    - "(no new Go deps; NSIS 3.10+ MessageBox + WriteRegStr/DeleteRegValue/StrCpy/StrCmp/IfFileExists/ExecWait built-ins)"
  patterns:
    - "Per-user autostart via HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run\\SquireBot. NEVER HKLM (would need UAC). Path is double-quoted in the registry value to handle $INSTDIR with spaces."
    - "Delegated wincred wipe: NSIS doesn't speak DPAPI, only the Go binary does. Uninstaller invokes squirebot.exe --uninstall-wipe-credentials BEFORE deleting the binary; the flag handler does config.Load -> auth.DeleteToken -> os.Exit(0)."
    - "Default-preserve UX (CONTEXT.md Q3 locked): MessageBox MB_YESNO|MB_DEFBUTTON2 puts focus on No; reinstalling preserves OAuth state."
    - "Always-exit-0 on the flag handler: even on config-load failure, missing email, or wincred delete failure. The uninstaller MUST not block on partial state (a guildie who never completed the wizard still needs to be able to uninstall)."
    - "Flag handler runs FIRST in main(), BEFORE update.Apply. Auto-update has no business firing during an uninstall."
key-files:
  created: []
  modified:
    - installer/squirebot.nsi
    - cmd/squirebot/main.go
    - docs/build-and-install.md
decisions:
  - "Flag handler placement: FIRST in main(), BEFORE update.Apply (Plan 02-06's startup-swap entry point). Plan 02-06's hand-off note suggested 'right AFTER update.Apply()' but the PLAN.md's Task 2 explicit body says 'AS THE FIRST action, before that update.Apply() block'. Followed the plan. Rationale: an uninstall in flight should not race against a startup-swap that resurrects a .new binary the user just asked to remove. The flag handler is a pure kill-switch + side-effect (wincred delete) + exit; no I/O outside config.Load + wincred."
  - "Autostart path is quoted with NSIS single-quote string delimiters wrapping double-quote characters: WriteRegStr ... 'SquireBot' '\"$INSTDIR\\${EXE_NAME}\"'. The Run-key value parser respects the double-quotes; without them a $INSTDIR containing a space (e.g., username 'Virus Canary') would silently fail."
  - "Always-removed-on-uninstall set: binary, icon, uninstaller exe, log files (squirebot.log + .log.* rotations), HKCU Run key, HKCU Uninstall subkey. The LOCALAPPDATA\\SquireBot dir is removed via RMDir which is no-op-if-not-empty -- so preserve-mode (config.json still inside) keeps the dir, full-wipe-mode (config.json deleted) removes it."
  - "DeleteRegValue HKCU ...\\Run\\SquireBot is in the unconditional cleanup section (after the StrCmp/SkipWipeBinary block). The Run key is removed regardless of preserve/wipe choice — preserving autostart of a deleted binary makes no sense."
  - "Var /GLOBAL UninstallWipe declared inside Section 'Uninstall' (NSIS allows this; the /GLOBAL flag scopes the var to the whole script even when declared inside a Section). Branching uses NSIS labels (UninstallWipeYes/No/Done, SkipWipeBinary, RunWipeBinary, SkipConfigDelete) — NSIS has no if/else; labels + Goto + StrCmp are the canonical pattern."
  - "Manual logon-cycle smoke test (PLAN Task 5) is DEFERRED to user. Requires a real Windows logout/logon cycle on a clean Win11 VM, plus building the installer locally with makensis -- not something an executor agent can do. Detailed runbook included below in 'Manual Smoke Test — Deferred to User'."
metrics:
  duration: ~12min
  completed: 2026-05-01
  tasks_completed: 4 of 4 code/config (Task 5 manual smoke deferred to user with runbook)
  commits: 4
  files_changed: 3 (installer/squirebot.nsi, cmd/squirebot/main.go, docs/build-and-install.md)
  test_count_added: 0 (NSIS has no unit-test framework; flag handler is exercised manually per the runbook)
  test_count_total_passing: ~110 (every prior wave's tests still green; full go test ./... -count=1 passes)
---

# Phase 2 Plan 07: Autostart Hardening + Uninstaller UX Summary

INST-04 (Windows logon autostart) lands as a single `WriteRegStr HKCU
...\Run\SquireBot` line in the NSIS installer's Install section, with
the matching `DeleteRegValue` in the Uninstall section. This was the
last load-bearing requirement keeping "install once and forget" from
being literally true; without it the watcher only ran when a guildie
explicitly launched it, defeating the WATCH-08 heartbeat.

The CONTEXT.md-locked uninstaller UX (Q3: "checkbox to wipe config +
wincred, default = preserve") lands as a `MessageBox MB_YESNO|MB_DEFBUTTON2`
prompt — NSIS-native, no MUI plumbing required. The wincred wipe is
delegated to a new `--uninstall-wipe-credentials` flag on the existing
`squirebot.exe` because NSIS doesn't speak DPAPI. The flag handler runs
as the FIRST action of `main()` — before `update.Apply` — so the
uninstall path is a pure kill-switch and can't race against the
startup-swap goroutine.

Phase 2 honest constraint: the live logon-cycle smoke test (Task 5)
requires a real Windows logout/logon on a clean VM. That's deferred to
the user; a step-by-step runbook is below.

## What Shipped

### Task 1 — installer/squirebot.nsi: HKCU Run-key write + delete (INST-04)
**Commit:** `2482322 feat(02-07): add HKCU Run-key autostart to NSIS installer (INST-04)`

- Retired the Phase 1 deferral comment block ("INST-04 (autostart) is
  DEFERRED TO PHASE 2 — Phase 1 deliberately does NOT register
  HKCU\\...\\Run") and replaced it with an explainer:
  ```
  ; -- INST-04 (autostart) --
  ; Per-user HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run\\SquireBot
  ; pointing at $INSTDIR\\squirebot.exe. No UAC needed (HKCU, not HKLM).
  ; The uninstaller removes this key unconditionally.
  ```
- In `Section "Install"`, just before the existing post-install
  `Exec '"$INSTDIR\\${EXE_NAME}"'`:
  ```
  WriteRegStr HKCU "Software\\Microsoft\\Windows\\CurrentVersion\\Run" "SquireBot" '"$INSTDIR\\${EXE_NAME}"'
  ```
- In `Section "Uninstall"`, just before `DeleteRegKey HKCU "${REGPATH_UNINSTSUBKEY}"`:
  ```
  DeleteRegValue HKCU "Software\\Microsoft\\Windows\\CurrentVersion\\Run" "SquireBot"
  ```

### Task 2 — cmd/squirebot/main.go: --uninstall-wipe-credentials flag
**Commit:** `60b52c8 feat(02-07): add --uninstall-wipe-credentials flag handler in main`

Inserted as the FIRST action of `main()`, BEFORE the `update.Apply()`
block (Plan 02-06):

```go
if len(os.Args) >= 2 && os.Args[1] == "--uninstall-wipe-credentials" {
    cfg, err := config.Load()
    if err != nil {
        fmt.Fprintf(os.Stderr, "config load failed: %v\n", err)
        os.Exit(0)
    }
    if cfg.GoogleEmail == "" {
        fmt.Fprintln(os.Stderr, "no email in config; nothing to wipe")
        os.Exit(0)
    }
    if err := auth.DeleteToken(cfg.GoogleEmail); err != nil {
        fmt.Fprintf(os.Stderr, "wincred delete failed for %s: %v\n", cfg.GoogleEmail, err)
        os.Exit(0)
    }
    fmt.Fprintf(os.Stderr, "wincred entry removed for %s\n", cfg.GoogleEmail)
    os.Exit(0)
}
```

Always exits 0 — the uninstaller MUST not block on partial state. All
imports (`fmt`, `os`, `config`, `auth`) were already present from
Phase 1 + Plan 02-06 wiring.

### Task 3 — installer/squirebot.nsi: Uninstall section refactor
**Commit:** `ac7d5fd feat(02-07): add uninstaller wipe-or-preserve prompt (CONTEXT.md Q3)`

Replaced the entire Uninstall section body with the wipe-or-preserve
state machine:

1. `MessageBox MB_YESNO|MB_ICONQUESTION|MB_DEFBUTTON2` prompts the user.
   Default focus is **No** (preserve). Sets `$UninstallWipe` to "1"
   (Yes) or "0" (No).
2. If `$UninstallWipe == "1"` AND the binary still exists, ExecWait the
   binary with `--uninstall-wipe-credentials` to delete the wincred entry
   BEFORE deleting the binary itself.
3. Always: taskkill the running instance, delete binary/icon/uninstaller,
   delete rotated log files.
4. Conditional: if `$UninstallWipe == "1"`, also delete config.json.
5. Always: try to RMDir LOCALAPPDATA\\SquireBot (no-op if not empty —
   preserve-mode keeps the dir because config.json is still there).
6. Always: DeleteRegValue HKCU\\...\\Run\\SquireBot (the Run-key from
   Task 1's autostart write).
7. Always: DeleteRegKey HKCU\\...\\Uninstall\\SquireBot.

Retired the Phase 1 manual-cleanup comment block (the "wincred entry
SquireBot:<email> is NOT auto-deleted" + cmdkey manual-cleanup
instructions) — Phase 2 fixes the underlying gap.

### Task 4 — docs/build-and-install.md: Uninstalling section
**Commit:** `d13d36a docs(02-07): document new uninstaller wipe-or-preserve UX`

Rewrote the "Uninstalling" section to document:
- The new prompt UX (with the exact prompt text quoted as a blockquote)
- The Yes-vs-No outcomes (full wipe vs preserve)
- The always-removed-regardless list (binary, icon, HKCU Run key, HKCU
  Uninstall subkey, log files)
- A new "Manual recovery" subsection for users who chose No but later
  want a full wipe (the cmdkey + Remove-Item PowerShell commands)

## Acceptance — Self-Check

```
build  : exit 0   (go build ./cmd/squirebot/...)
vet    : exit 0   (go vet ./...)
tests  : ALL PASS (go test ./... -count=1, ~110 tests across 14 packages)
```

| Plan acceptance criterion                                                                            | Result |
|------------------------------------------------------------------------------------------------------|--------|
| `grep -c 'WriteRegStr HKCU "Software\\Microsoft\\Windows\\CurrentVersion\\Run"' installer/squirebot.nsi` returns 1 | 1 |
| `grep -c 'DeleteRegValue HKCU "Software\\Microsoft\\Windows\\CurrentVersion\\Run"' installer/squirebot.nsi` returns 1 | 1 |
| `grep -c 'DEFERRED TO PHASE 2' installer/squirebot.nsi` returns 0                                    | 0 |
| `grep -cE 'INST-04' installer/squirebot.nsi` >= 2                                                    | 3 |
| `grep -n '\\-\\-uninstall-wipe-credentials' cmd/squirebot/main.go` >= 2                              | 2 |
| `grep -c 'auth.DeleteToken' cmd/squirebot/main.go` >= 1                                              | 1 |
| `grep -c 'os.Exit(0)' cmd/squirebot/main.go` >= 4                                                    | 6 |
| `go build ./cmd/squirebot/...` succeeds                                                              | yes |
| `go vet ./...` returns no errors                                                                     | yes |
| `grep -c 'MessageBox MB_YESNO' installer/squirebot.nsi` returns 1                                    | 1 |
| `grep -c 'MB_DEFBUTTON2' installer/squirebot.nsi` >= 1 (directive present)                           | 2 (1 directive + 1 comment) |
| `grep -c 'uninstall-wipe-credentials' installer/squirebot.nsi` returns 1                             | 1 |
| `grep -c 'wincred entry SquireBot' installer/squirebot.nsi` returns 0                                | 0 |
| `grep -c '\\$UninstallWipe' installer/squirebot.nsi` >= 4                                            | 5 |
| `grep -c 'Also delete saved configuration' docs/build-and-install.md` returns 1                      | 1 |
| `grep -c 'cmdkey /delete:SquireBot' docs/build-and-install.md` >= 1                                  | 1 |
| `grep -c 'HKCU\\\\Software\\\\Microsoft\\\\Windows\\\\CurrentVersion\\\\Run\\\\SquireBot' docs/build-and-install.md` >= 1 | 1 |
| `grep -c 'preserve config.json' docs/build-and-install.md` >= 1                                      | 1 |
| `grep -c 'recommended; default' docs/build-and-install.md` >= 1                                      | 1 |

## End-to-End Flow Verification

```
INSTALL (NSIS Section "Install"):
  ...existing payload + uninstaller-registration...
  WriteRegStr HKCU ...\\Run\\SquireBot = '"$INSTDIR\\squirebot.exe"'    <-- NEW (Task 1)
  Exec '"$INSTDIR\\squirebot.exe"'                                      (existing)

WINDOWS LOGON (next user sign-in):
  Windows reads HKCU ...\\Run\\SquireBot
  Launches "$INSTDIR\\squirebot.exe"
  -> cmd/squirebot/main.go main() entry
  -> os.Args[1] != "--uninstall-wipe-credentials" → skip flag handler
  -> update.Apply() → no .new staged → (false, nil); proceed
  -> logging.Setup → config.Load → tray + RunApp → systray.Run
  Tray icon appears in 1-2 seconds. WATCH-08 heartbeat fires within 24h.

UNINSTALL (NSIS Section "Uninstall"):
  MessageBox MB_YESNO|MB_DEFBUTTON2 → user clicks button
    | Yes → $UninstallWipe = "1"
    | No (default focus) → $UninstallWipe = "0"

  if $UninstallWipe == "1" AND $INSTDIR\\squirebot.exe exists:
    ExecWait '"$INSTDIR\\squirebot.exe" --uninstall-wipe-credentials'
      → cmd/squirebot/main.go main() entry
      → os.Args[1] == "--uninstall-wipe-credentials" → flag handler
        → config.Load() → cfg
        → if cfg.GoogleEmail == "" → log + Exit(0)
        → auth.DeleteToken(cfg.GoogleEmail) → wincred entry removed
        → log "wincred entry removed for <email>" → Exit(0)

  Always: taskkill /IM squirebot.exe /F
  Always: Delete $INSTDIR\\squirebot.exe + icon.ico + uninstall.exe + RMDir $INSTDIR
  Always: Delete log files
  if $UninstallWipe == "1":
    Delete $LOCALAPPDATA\\SquireBot\\config.json
  Always: RMDir $LOCALAPPDATA\\SquireBot (succeeds only if empty —
                                            preserve mode keeps it)
  Always: DeleteRegValue HKCU ...\\Run\\SquireBot                       <-- NEW (Task 1)
  Always: DeleteRegKey HKCU ...\\Uninstall\\SquireBot

REINSTALL after preserve-mode uninstall:
  Installer drops new squirebot.exe, sets HKCU ...\\Run again, post-install Exec
  → main() → no flag → update.Apply (no-op) → logging → config.Load
  → cfg.GoogleEmail present, cfg.SpreadsheetID present
  → wizard NOT re-run; watcher resumes immediately on the same workbook
  → wincred entry SquireBot:<email> still present from previous install
  → Sheets writes succeed without re-OAuth

REINSTALL after full-wipe-mode uninstall:
  Installer drops new squirebot.exe, sets HKCU ...\\Run again, post-install Exec
  → main() → no flag → update.Apply (no-op) → logging → config.Load
  → config.json absent → Load returns zero-Config + LogLevel="info"
  → cfg.GoogleEmail empty → wizard re-runs from start
  → fresh OAuth flow, fresh picker, fresh EQ folder selection
```

## Manual Smoke Test — Deferred to User

PLAN.md Task 5 requires a real Windows logout/logon cycle on a clean
Win11 VM — not executable by an agent. Below is the exact runbook the
user (jbowen) should run before declaring Plan 02-07 complete.

### Pre-flight (one-time per build)

```powershell
# 1. From repo root. Build the binary with the OAuth ldflags.
$cfg                  = Get-Content .planning/phases/01-end-to-end-thin-slice/oauth-config.json -Raw | ConvertFrom-Json
$OAUTH_CLIENT_ID      = $cfg.oauth_client_id
$OAUTH_CLIENT_SECRET  = $cfg.oauth_client_secret
$PICKER_API_KEY       = $cfg.picker_api_key
$GCP_PROJECT_NUMBER   = $cfg.gcp_project_number
$VERSION              = "0.2.0-rc1"

if ($cfg.consent_screen_status -ne "PRODUCTION") {
    Write-Error "oauth-config.json consent_screen_status must be PRODUCTION before building."
    exit 1
}

New-Item -ItemType Directory -Force -Path dist | Out-Null
$env:GOOS   = "windows"
$env:GOARCH = "amd64"
go build -ldflags="-H=windowsgui -s -w `
  -X main.OAuthClientID=$OAUTH_CLIENT_ID `
  -X main.OAuthClientSecret=$OAUTH_CLIENT_SECRET `
  -X main.PickerAPIKey=$PICKER_API_KEY `
  -X main.GCPProjectNumber=$GCP_PROJECT_NUMBER `
  -X main.Version=$VERSION" `
  -o dist/squirebot.exe ./cmd/squirebot

# 2. Build the installer.
makensis -V2 -DAPPVERSION=$VERSION installer/squirebot.nsi
# Produces dist/SquireBot-Setup-0.2.0-rc1.exe.
```

### Step 1 — Install + autostart verification

1. Copy `dist/SquireBot-Setup-0.2.0-rc1.exe` to a clean Win11 VM (or a
   fresh user account on this machine).
2. Run the installer; click through SmartScreen "More info → Run anyway";
   click Install.
3. Walk through the post-install wizard (OAuth + Picker + EQ folder).
4. Drop a `Foo-Inventory.txt` into the watched folder; verify the
   `inv:Foo` tab appears in the workbook within 30s.

5. **Verify the Run-key was written:**
   ```powershell
   Get-ItemProperty -Path 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run' SquireBot
   ```
   **Expected:** `SquireBot : "C:\Users\<you>\AppData\Local\Programs\SquireBot\squirebot.exe"`

   **If missing:** Task 1 regression. Inspect the installer log; the
   WriteRegStr line in installer/squirebot.nsi line ~92 didn't fire.

### Step 2 — Logon-cycle smoke test (the load-bearing test)

1. Sign out of Windows (Start → user icon → Sign out). DO NOT just
   restart explorer.exe — sign out is the canonical test.
2. Sign back in.
3. Wait 30-60 seconds for the desktop to settle.
4. **Expected:** SquireBot tray icon appears in the system tray
   (bottom-right) WITHOUT any manual launch.
5. **Verify the watcher is alive:**
   ```powershell
   Get-Content $env:LOCALAPPDATA\SquireBot\squirebot.log -Tail 20
   ```
   **Expected:** Recent log entries include `squirebot starting`,
   `tray ready`, watcher loop active.
6. Drop another `Bar-Inventory.txt` into the watched folder; verify
   the `inv:Bar` tab appears within 30s. Confirms the autostarted
   watcher is fully functional, not just "the process exists".

   **If the tray icon does NOT appear within 60s after sign-in:**
   Task 1 regression. Check `Get-ItemProperty` output again; if the
   key is present but the binary doesn't launch, check Task Manager
   → Startup tab to see if Windows disabled the entry.

### Step 3 — Uninstall test A: preserve mode (default)

1. Open **Settings → Apps → Apps & features → SquireBot → Uninstall**.
2. The MessageBox prompt appears.
3. **Verify the focus is on No** (Tab/Enter would select No).
   Press Enter (or click No).
4. The uninstaller proceeds.

5. **Verify config.json STILL exists:**
   ```powershell
   Test-Path "$env:LOCALAPPDATA\SquireBot\config.json"
   ```
   **Expected:** `True`.

6. **Verify wincred STILL exists:**
   ```powershell
   cmdkey /list | Select-String 'SquireBot:'
   ```
   **Expected:** `Target: SquireBot:<your-email>` line appears.

7. **Verify Run key is GONE:**
   ```powershell
   Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run' SquireBot -ErrorAction SilentlyContinue
   ```
   **Expected:** No output (property not found).

8. **Verify binary is GONE:**
   ```powershell
   Test-Path "$env:LOCALAPPDATA\Programs\SquireBot\squirebot.exe"
   ```
   **Expected:** `False`.

### Step 4 — Reinstall after preserve

1. Run the installer again.
2. The post-install Exec launches `squirebot.exe`.
3. **Expected:** wizard does NOT re-run. The watcher resumes
   immediately on the previously-picked workbook (config.json
   preserved) without re-OAuth (wincred preserved).
4. Drop another file; verify upload succeeds.

### Step 5 — Uninstall test B: full wipe mode

1. Uninstall again. This time click **Yes**.
2. The uninstaller invokes `squirebot.exe --uninstall-wipe-credentials`
   internally (you may see a brief flash; the binary exits within
   ~100ms).
3. **Verify config.json is GONE:**
   ```powershell
   Test-Path "$env:LOCALAPPDATA\SquireBot\config.json"
   ```
   **Expected:** `False`.

4. **Verify wincred is GONE:**
   ```powershell
   cmdkey /list | Select-String 'SquireBot:'
   ```
   **Expected:** No output (no SquireBot: entry).

5. **Verify Run key is GONE** (same as preserve mode):
   ```powershell
   Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run' SquireBot -ErrorAction SilentlyContinue
   ```
   **Expected:** No output.

### Step 6 — Reinstall after full wipe

1. Run the installer again.
2. **Expected:** wizard re-runs FROM SCRATCH. Browser opens to the
   OAuth consent flow; Picker re-runs; EQ folder must be re-selected.
   Confirms the full-wipe path actually wiped both config.json and
   wincred.

### Acceptance for Task 5

All 6 steps must pass. If any step fails, return to the executor with
the specific step number + observed-vs-expected and tag the SUMMARY's
"Deviations from Plan" section with the regression. Otherwise mark
Plan 02-07 fully complete and proceed to Wave 7 (02-09 SmartScreen +
SignPath application).

## Deviations from Plan

### Plan-vs-Reality Drift Notes

**A. Task 5 (logon-cycle smoke) deferred to user with runbook.** Plan
declared `autonomous: false` precisely because this manual test is
unavoidable. The runbook above is the agent's deliverable in lieu of
the test result; the user runs it before declaring Plan 02-07
unconditionally complete.

**B. MB_DEFBUTTON2 grep count = 2 instead of 1.** Acceptance criterion
expected exactly 1 occurrence (the directive). My implementation has 2
because I added a comment line `MB_DEFBUTTON2 puts focus on No` for
clarity. The directive itself is present and correct; the spirit of
the criterion (default-No-focus) is satisfied. Treating as no-deviation.

**C. `Var /GLOBAL UninstallWipe` declared inside the Section.** NSIS
allows `Var /GLOBAL` declarations inside any Section (with the /GLOBAL
flag); the plan template showed this exact pattern. No deviation.

### Auto-fixed Issues

None. All four code/config tasks landed cleanly on the first pass.

### Authentication gates

None. The flag handler is invoked only by the local NSIS uninstaller;
no network, no OAuth, no Sheets API. wincred operations are local-machine
DPAPI calls that don't need user re-consent.

## Known Stubs

None. The implementation is end-to-end functional from this commit
forward:
- `installer/squirebot.nsi` writes the Run key on install + removes it
  on uninstall (verifiable via Steps 1, 3, and 5 of the runbook).
- `cmd/squirebot/main.go` handles the flag with real `auth.DeleteToken`
  + `config.Load` calls (verifiable via Step 5).
- `docs/build-and-install.md` documents the user-visible UX.

The only remaining dependency is the manual logon-cycle smoke (Task 5),
which is a verification step, not a stub.

## Threat Flags

| Flag | File | Description |
|------|------|-------------|
| threat_flag: process | cmd/squirebot/main.go | New CLI entry point with side effect (wincred entry deletion). The flag is consumed only by the NSIS uninstaller; an attacker who can run the binary already has full code-execution on the user's account, so they can wipe the entry anyway via `cmdkey /delete:SquireBot:<email>`. Threat surface unchanged. |
| threat_flag: registry | installer/squirebot.nsi | New HKCU\\...\\Run\\SquireBot value; this is the canonical Windows autostart pattern, no UAC required, fully expected by the OS. The risk is a Run-key squatter (some other installer overwriting our key with a malicious binary path), which is a category-wide risk, not specific to this plan. Mitigation: standard Windows account hygiene; the watcher itself does no privileged operations. |

## TDD Gate Compliance

This plan is `type: execute` (not `type: tdd`); no RED/GREEN gate
sequence applies. The flag handler in main.go has NO unit test —
intentional, because:
- `auth.DeleteToken` is already covered by Phase 1's `internal/auth`
  tests.
- `config.Load` is already covered by Phase 1's `internal/config` tests.
- The flag handler's own logic is a 4-branch state machine (load fail
  / no email / delete fail / success) where each branch is a stderr
  print + os.Exit(0). Adding a unit test would require a TestMain or
  exec.Command harness — the cost-benefit favors manual verification
  via the runbook (Step 5 of which exercises the success branch end-to-end).

If a future plan wants to harden coverage, the recommended approach is
to extract the flag handler into a function (`runUninstallWipe(args
[]string, exitFn func(int))`) and unit-test it with a fake exitFn.
Deferred — yagni at this stage.

## Self-Check: PASSED

Verified all modified files contain the expected changes:

- `installer/squirebot.nsi`:
  - Line ~92 `WriteRegStr HKCU "Software\\Microsoft\\Windows\\CurrentVersion\\Run"` PRESENT
  - Line ~145 `DeleteRegValue HKCU "Software\\Microsoft\\Windows\\CurrentVersion\\Run"` PRESENT
  - Line ~105 `MessageBox MB_YESNO|MB_ICONQUESTION|MB_DEFBUTTON2` PRESENT
  - Line ~119 `ExecWait '"$INSTDIR\\${EXE_NAME}" --uninstall-wipe-credentials'` PRESENT
  - Phase 1 deferral comment ABSENT
  - Phase 1 manual-cleanup wincred comment block ABSENT

- `cmd/squirebot/main.go`:
  - Flag handler block (lines ~26-43) PRESENT
  - 4 `os.Exit(0)` calls inside the flag handler PRESENT
  - `auth.DeleteToken(cfg.GoogleEmail)` PRESENT
  - Inserted BEFORE the `update.Apply()` block (Plan 02-06)

- `docs/build-and-install.md`:
  - "Also delete saved configuration and Google account credentials?"
    blockquote PRESENT
  - "(recommended; default)" PRESENT
  - "Manual recovery" subsection with cmdkey + Remove-Item PRESENT

All 4 commits reachable from HEAD:
- `2482322 feat(02-07): add HKCU Run-key autostart to NSIS installer (INST-04)`
- `60b52c8 feat(02-07): add --uninstall-wipe-credentials flag handler in main`
- `ac7d5fd feat(02-07): add uninstaller wipe-or-preserve prompt (CONTEXT.md Q3)`
- `d13d36a docs(02-07): document new uninstaller wipe-or-preserve UX`

Build + vet + test all pass:
- `go build ./cmd/squirebot/...` exit 0
- `go vet ./...` exit 0
- `go test ./... -count=1` ALL PASS (~110 tests across 14 packages,
  including the 21 tests added by Plan 02-06 in internal/update +
  internal/tray)

## Wave 7 Handoff (02-09 SmartScreen + SignPath OSS application)

Plan 02-07 closes the watcher-side robustness story. Wave 7 (02-09)
applies for SignPath OSS code-signing and updates the SmartScreen
walkthrough copy in docs/build-and-install.md to reflect the
expected post-signing UX shift (SmartScreen wall disappears once a
signed binary accumulates Microsoft reputation).

Coordination guidance for 02-09:
- The autostart Run key path will not change post-signing — the
  binary path is the same `$INSTDIR\\squirebot.exe`, just with an
  embedded Authenticode signature.
- The auto-update path (Plan 02-06) is signing-agnostic by design;
  the same code that works for unsigned binaries works for signed.
- The flag handler is unaffected by signing — it's a local-only
  operation with no network surface.

After 02-09 ships, Wave 8 (02-10) is the 7-day soak; the manual smoke
runbook above is a strict subset of 02-10's broader soak protocol.
