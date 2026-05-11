---
phase: 06-installer-overwrite-running-shim
plan: 03
subsystem: installer
tags: [nsis, installer, version-gate, taskkill, inst-06]
requirements: [INST-06]
status: complete
completed: 2026-05-11
dependency-graph:
  requires:
    - "cmd/squirebot/main.go --quit handler (shipped in Plan 06-02, commits 5256382 + a36e72f)"
    - "internal/system.SignalShutdown / WaitForShutdown (shipped in Plan 06-01)"
  provides:
    - "NSIS pre-install shim that gracefully quits a running watcher before file overwrite (closes ROADMAP §44, §45)"
    - "v1.0.0 -> v1.0.1 upgrade path that skips --quit (avoids duplicate-tray bug from unknown CLI flag in v1.0.0 binary)"
  affects:
    - installer/squirebot.nsi
tech-stack:
  added:
    - "WordFunc.nsh include (NSIS-bundled, not a plugin)"
  patterns:
    - "Label-style NSIS control flow (StrCmp/IfFileExists/Goto/IntCmp) — matches existing file convention; no LogicLib ${If} macros"
    - "Inline StrContains helper (vs. !include StrFunc.nsh) to minimize include surface"
    - "Poll-via-tasklist using bundled nsExec::Exec (no nsProcess plugin needed — D-05)"
    - "Hard-kill fallback verbatim from uninstaller's existing taskkill (consistency)"
key-files:
  created: []
  modified:
    - installer/squirebot.nsi
decisions:
  - "Followed CONTEXT.md D-02: version gate skips --quit for DisplayVersion < 1.0.1 (or empty/fresh-install)"
  - "Followed CONTEXT.md D-03: 10s hard cap (40 iterations * 250ms Sleep)"
  - "Followed CONTEXT.md D-04: post-install Exec line (now line 204) untouched"
  - "Followed CONTEXT.md D-05: poll path uses bundled nsExec::Exec + tasklist, NOT nsProcess plugin and NOT System::Call against kernel32"
  - "Picked tasklist+nsExec over System::Call OpenProcess: more readable NSIS, zero kernel32 marshaling, output parsing is trivially testable manually. The deferred_ideas note from CONTEXT.md about plugin standardization is moot — no plugin was added."
metrics:
  duration: ~12 min
  tasks_completed: 1
  files_modified: 1
  commits: 1
---

# Phase 6 Plan 03: NSIS Pre-Install Shim Summary

One-liner: Inserts the INST-06 pre-install shim at the top of `Section "Install"` in `installer/squirebot.nsi` — reads `DisplayVersion` from HKCU, version-gates against `1.0.1` via `${VersionCompare}`, `ExecWait`s `squirebot.exe --quit` for eligible upgrades, polls `tasklist` via bundled `nsExec::Exec` for up to 10s, then runs `taskkill /IM /F` as the always-fires fallback — all under `RequestExecutionLevel user`, no new NSIS plugins.

## Insertion Points (Final Line Numbers, Post-Insert)

| Insertion | Lines (final) | Notes |
| --------- | ------------- | ----- |
| `!include "WordFunc.nsh"` | line 45 | One blank line above + below the `!define REGPATH_UNINSTSUBKEY` block ending at line 43 |
| `Function StrContains` ... `FunctionEnd` | lines 52-78 | Sits between the include and `RequestExecutionLevel user` (line 83); file-scope as required by NSIS |
| Pre-install shim block | lines 113-175 | First content of `Section "Install"`; lead comment at 113, hard-kill fallback `ExecWait` at 174, end marker at 175 |
| `RequestExecutionLevel user` | now line 83 (was line 48) | Shifted +35 lines by the include + StrContains insert |
| Post-install `Exec '"$INSTDIR\${EXE_NAME}"'` | now line 204 (was line 105) | Shifted +99 lines total by all inserts; **content UNCHANGED** (D-04 satisfied) |
| Autostart `WriteRegStr HKCU "...\Run" "SquireBot"` | now line 201 (was line 102) | Shifted; **content UNCHANGED** |
| Uninstaller `ExecWait 'taskkill /IM "${EXE_NAME}" /F'` | now line 235 (was line 136) | Shifted; **content UNCHANGED** |

Total file length: 263 lines (was 164). Net add: +99 lines.

## Polling Approach Picked (CONTEXT.md D-05 unresolved question)

CONTEXT.md left the planner to pick between two pure-NSIS-bundled options:
- (a) **`nsExec::Exec 'tasklist /FI ...'`** — bundled `nsExec` plugin (ships with every NSIS install; not a separate download), parses tasklist stdout for the EXE name.
- (b) **`System::Call 'kernel32::OpenProcess(...)' + WaitForSingleObject`** — bundled `System` plugin, more kernel-API marshaling.

**Picked (a) — `tasklist` via `nsExec::Exec`.** Rationale:
- More readable NSIS (no Win32 type-encoding strings like `'i,i,*i .r0'`).
- No PID tracking (tasklist filters by image name; we don't need to remember the PID from a CreateProcess).
- Manually debuggable: a guildie can run `tasklist /FI "IMAGENAME eq squirebot.exe" /NH` in a normal cmd to see the same output.
- D-05 still honored: `nsExec` is bundled with NSIS (NOT a separate plugin like `nsProcess`).

The deferred_ideas note from CONTEXT.md about plugin standardization is therefore moot — no plugin dep was added; future installer work can either keep using `nsExec` for process probing or switch to `System::Call` per case.

## Acceptance Criteria Results

All 14 grep checks PASS (verified via PowerShell `Select-String -SimpleMatch`):

| Check | Result | Line(s) |
| ----- | ------ | ------- |
| `!include "WordFunc.nsh"` (1 line) | PASS | 45 |
| `Function StrContains` (1 line) | PASS | 52 |
| `FunctionEnd` (>=1) | PASS | 78 |
| `; -- INST-06 (overwrite-running shim) --` (1 line) | PASS | 113 |
| `ReadRegStr $0 HKCU "${REGPATH_UNINSTSUBKEY}" "DisplayVersion"` (1 line) | PASS | 127 |
| `${VersionCompare} "$0" "1.0.1" $1` (1 line) | PASS | 128 |
| `StrCmp $1 "2" SkipQuitSignal` (1 line) | PASS | 132 |
| `ExecWait '"$INSTDIR\${EXE_NAME}" --quit'` (1 line) | PASS | 144 |
| `nsExec::Exec 'tasklist /FI "IMAGENAME eq ${EXE_NAME}" /NH'` (1 line) | PASS | 157 |
| `ExecWait 'taskkill /IM "${EXE_NAME}" /F'` (exactly 2 lines: new pre-install + existing uninstaller) | PASS | 174, 235 |
| `RequestExecutionLevel user` directive (1 active directive line) | PASS | 83 (plus a pre-existing comment mention at line 6) |
| `Exec '"$INSTDIR\${EXE_NAME}"'` post-install (1 line) | PASS | 204 |
| `WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "SquireBot"` (1 line) | PASS | 201 |
| Ordering: `Section "Install"` (112) < `ReadRegStr ... DisplayVersion` (127) < `SetOutPath "$INSTDIR"` (177) | PASS | — |

## Unchanged Anchors (Verified Verbatim)

- **Line 83 `RequestExecutionLevel user`** — directive intact; the shim adds zero admin/elevated calls (HKCU read, IfFileExists, ExecWait of a user-integrity binary, nsExec tasklist, Sleep, taskkill /IM /F — all user-session-scoped per T-06-14 mitigation).
- **Line 201 `WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "SquireBot" '"$INSTDIR\${EXE_NAME}"'`** — autostart Run-key write byte-identical to v1.0.0.
- **Line 204 `Exec '"$INSTDIR\${EXE_NAME}"'`** — post-install relaunch byte-identical to v1.0.0 (D-04 satisfied).
- **`Section "Uninstall"` (lines 207-262)** — entire block untouched; existing `taskkill /F` at line 235 unchanged.

## NSIS `nsExec` Bundling Note (D-05 Compliance)

`nsExec` is shipped as a built-in plugin with every NSIS install since NSIS 2.x (see NSIS docs: https://nsis.sourceforge.io/Docs/nsExec/nsExec.txt). It is NOT a separate download and requires NO extra `!addplugindir` directive or CI fetch step. The release workflow's existing `nsis` choco package install already ships `nsExec` in its Plugins\x86-unicode directory. D-05 ("no external plugin dependency") is therefore satisfied.

The `WordFunc.nsh` include is similarly bundled: it ships in NSIS's `Include\` directory since NSIS 2.46. No external dep.

## NSIS Build Verification

**Status: DEFERRED to CI.**

- `makensis.exe` is NOT installed on this dev machine (checked `C:\Program Files\NSIS\` and `C:\Program Files (x86)\NSIS\` — neither path exists).
- Per the project rule "User installs missing toolchains themselves" (CLAUDE.md / user-memory), this executor did NOT install NSIS.
- The authoritative compile gate is `.github/workflows/release.yml` on tag push (per CONTEXT.md `<canonical_refs>` and the plan's `<acceptance_criteria>` final bullet which says: *"IF NSIS is not installed locally: skip this check (CI in release.yml is the authoritative gate per release workflow lines 75-100) but document the skip in the SUMMARY."*).
- All other acceptance criteria pass via static grep checks — see table above.

**To verify locally before the v1.0.1 tag push, the user should run:**
```
makensis /V2 /DAPPVERSION=1.0.1 /DAPPVERSIONNUMERIC=1.0.1.0 installer\squirebot.nsi
```
on a machine with NSIS 3.10+ installed. Expected: exit 0, `dist\SquireBot-Setup-1.0.1.exe` produced.

## Manual Smoke (n/a for this plan)

The full end-to-end smoke (install v1.0.0 -> running watcher -> install v1.0.1 -> verify tray flicker + no manual stop) is gated on:
1. v1.0.1 binary being built (Plan 06-04 / Plan 06-05 release plumbing).
2. v1.0.0 actually being installed on a clean Win11 VM.

This is the UAT for ROADMAP §44 success criterion 1 and is owned by the release-gate plan, not by this NSIS source-edit plan.

## Decisions Honored

- **D-01 (named-event mechanism):** Shim invokes `ExecWait '"$INSTDIR\${EXE_NAME}" --quit'` which calls into Plan 06-02's `system.SignalShutdown` and exits 0 within ~1s.
- **D-02 (version-gated hard kill for v1.0.0):** `VersionCompare` returns `"2"` for empty / pre-1.0.1 DisplayVersion -> `StrCmp $1 "2" SkipQuitSignal` jumps over the `--quit` call entirely -> goes straight to `taskkill /IM /F`. Verified by tracing both branches in the inserted block (lines 127-169).
- **D-03 (10s timeout, abandon in-flight writes):** Poll loop is `IntCmp $2 40 PollTimedOut ... Sleep 250 ... IntOp $2 $2 + 1` -> exactly 40 iterations * 250ms = 10000ms hard cap. After timeout, `PollTimedOut:` (line 167) falls through to `PollDone:` (line 168) which falls through to `SkipQuitSignal:` (line 169) which falls through to the always-run `taskkill /IM /F` (line 174).
- **D-04 (post-install relaunch unchanged):** Line 204 byte-identical to pre-edit. Verified by grep + git diff: the only edits are an insert at line 43-44, an insert at line 78-79, and an insert at line 112-113.
- **D-05 (no Find-Process plugin):** Picked `nsExec::Exec 'tasklist'` over `System::Call kernel32::OpenProcess`. Both options are bundled-only; chose readability + manual-debuggability. The `nsExec` plugin ships with NSIS itself, not as a separate download.

## Threat Model — Mitigations Applied

- **T-06-12 (HKCU DisplayVersion tampering — accept):** No additional mitigation; per threat-model disposition, the attacker-can-write-HKCU scenario yields a less-graceful upgrade (forced taskkill /F path) but no data loss or privilege escalation.
- **T-06-13 (taskkill confused-deputy — mitigate):** Used the exact `/IM "${EXE_NAME}" /F` filter from the uninstaller's prior-art line; no `/S` (no remote-session), no `/T` (no child-process tree, which could expand the kill set unintentionally). User-session-scoped per Windows OS contract for non-admin taskkill.
- **T-06-14 (privilege escalation — mitigate):** All operations user-integrity: `ReadRegStr HKCU` (no HKLM), `IfFileExists` (filesystem read), `ExecWait` of a binary at $INSTDIR (= %LOCALAPPDATA% per InstallDir on line 96 — never Program Files), `nsExec::Exec` of `tasklist` (user-scoped enumeration), `Sleep`, `taskkill /IM /F` (no `/S`, no `/U`). `RequestExecutionLevel user` on line 83 is the file-scope lock. Verified no `HKLM`, no `Global\`, no `ExecShell ... runas`, no `SetShellVarContext all`.
- **T-06-15 (poll-loop DoS via re-spawn — accept):** Bounded by 40-iteration cap; after 10s the loop exits via `PollTimedOut` and `taskkill /F` fires.
- **T-06-16 (info disclosure in install log — accept):** Same info already in INST-01/INST-04 lines and the uninstaller's existing taskkill line.

## Deviations from Plan

None — the plan's `<action>` blocks were pasted verbatim. Three diff regions inserted: (1) `!include` line, (2) `StrContains` function, (3) pre-install shim block at top of `Section "Install"`. No additional bug-fix deviations under Rules 1-3.

## Deferred Issues

None.

## Known Stubs

None — the shim is fully functional. The `IntCmp ... PollTimedOut PollContinue PollTimedOut` line has two identical jump targets for the `<` and `>` cases (both go to PollTimedOut) because we only want to advance past the cap in one direction; this is intentional NSIS idiom, not a stub.

## Self-Check: PASSED

- File modified — FOUND: `installer/squirebot.nsi` (now 263 lines, was 164)
- Commit — FOUND: `9a179bd` (feat(06-03): NSIS pre-install shim — graceful quit + hard-kill fallback)
- Acceptance criteria — 14/14 PASS (only "near-fail" was 2 hits for `RequestExecutionLevel user` because line 6 is a pre-existing comment mentioning the directive; the actual directive is at line 83, content-identical to the pre-edit line 48)
- NSIS toolchain — NOT installed locally; build verification deferred to CI per plan's acceptance-criteria fallback clause
- Locked-decision compliance — D-01 through D-05 all honored, verified inline

## CLI Contract Plan 06-04 / 06-05 Will Tag

This plan ships the source-side INST-06 change. The downstream release plan (`v1.0.1` tag push) will:
1. Trigger `.github/workflows/release.yml`.
2. CI will run `makensis /V2 /DAPPVERSION=1.0.1 /DAPPVERSIONNUMERIC=1.0.1.0 installer/squirebot.nsi`.
3. Produce `dist/SquireBot-Setup-1.0.1.exe` + sha256 + updated `latest.json`.
4. Create GitHub Release with all three artifacts attached.

This plan creates none of those artifacts; it only modifies the `.nsi` source that CI will compile.
