---
phase: 06-installer-overwrite-running-shim
plan: 03
type: execute
wave: 3
depends_on: [06-02-main-go-wiring-PLAN]
files_modified:
  - installer/squirebot.nsi
autonomous: true
requirements: [INST-06]
tags: [nsis, installer, version-gate, taskkill]

must_haves:
  truths:
    - "When the v1.0.1 installer is re-run on a machine that has v1.0.1 already installed AND running, the pre-install shim invokes `squirebot.exe --quit`, polls for process exit up to 10s, and falls back to `taskkill /IM squirebot.exe /F` only on timeout."
    - "When the v1.0.1 installer is re-run on a machine with v1.0.0 installed (whose binary does NOT recognize `--quit`), the version gate skips the `--quit` step and goes STRAIGHT to `taskkill /IM squirebot.exe /F` (avoids spawning a duplicate tray per CONTEXT.md D-02)."
    - "On a clean machine with no prior install, the pre-install shim is a sequence of no-ops: the version-compare evaluates against an empty DisplayVersion -> skip --quit -> taskkill /F finds no process -> continue with normal install."
    - "The post-install `Exec '$INSTDIR\\${EXE_NAME}'` line on line 105 is UNCHANGED (D-04)."
    - "The installer continues to honor `RequestExecutionLevel user` on line 48 — no new admin/elevated calls are added (HKCU-only registry reads, user-session-scoped taskkill)."
  artifacts:
    - path: installer/squirebot.nsi
      provides: "Pre-install shim block at the TOP of Section Install — reads DisplayVersion, version-compares, ExecWait --quit if eligible, polls via tasklist probe, falls back to taskkill /F"
      contains: "INST-06"
  key_links:
    - from: installer/squirebot.nsi (pre-install block)
      to: cmd/squirebot/main.go --quit handler
      via: "ExecWait '\"$INSTDIR\\${EXE_NAME}\" --quit'"
      pattern: "--quit"
    - from: installer/squirebot.nsi (version gate)
      to: HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\SquireBot\DisplayVersion
      via: "ReadRegStr against REGPATH_UNINSTSUBKEY define"
      pattern: "ReadRegStr HKCU"
---

<objective>
Add the NSIS pre-install shim at the TOP of `Section "Install"` in `installer/squirebot.nsi` so that re-running the installer over a running watcher upgrades cleanly without a manual stop step.

Sequence:
1. **Version gate** (D-02): `ReadRegStr` `DisplayVersion` from `HKCU\${REGPATH_UNINSTSUBKEY}`. Use `${VersionCompare}` from `WordFunc.nsh` (NSIS built-in, no plugin) to compare against `"1.0.1"`. If empty OR `< 1.0.1`, jump to the `taskkill /F` fallback (skip `--quit`).
2. **Graceful signal** (D-01, requires Plan 02 shipped): `ExecWait '"$INSTDIR\${EXE_NAME}" --quit'`.
3. **Poll loop** (D-03, D-05): up to 10s, 250ms interval. Probe with `nsExec::Exec 'tasklist /FI "IMAGENAME eq ${EXE_NAME}" /NH'` and parse output for the EXE name. No `nsProcess` plugin (per D-05).
4. **Hard-kill fallback** (always runs): `ExecWait 'taskkill /IM "${EXE_NAME}" /F'` — copy syntax verbatim from uninstaller line 136.

Place the entire block at the TOP of `Section "Install"`, BEFORE the first `SetOutPath`. Add `!include "WordFunc.nsh"` near the existing `!define` block.

Purpose: closes ROADMAP §44 success criterion 1 (clean upgrade) and §45 success criterion 2 (graceful → wait → hard-kill fallback). Post-install `Exec` line 105 and autostart Run-key line 102 remain unchanged, satisfying §46 (autostarts; no token re-auth needed because config + wincred are untouched).

Output: one modified `.nsi` file with three diff regions (top-of-file `!include` + `StrContains` helper + top-of-install-section shim block). Net add: ~80 lines of NSIS.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/STATE.md
@.planning/phases/06-installer-overwrite-running-shim/06-CONTEXT.md
@.planning/phases/06-installer-overwrite-running-shim/06-PATTERNS.md
@.planning/phases/06-installer-overwrite-running-shim/06-02-SUMMARY.md
@installer/squirebot.nsi

<nsis_primer>
**`!include "WordFunc.nsh"`** — bundled with NSIS since 2.46. Adds `${VersionCompare}`.

**`${VersionCompare} "$0" "1.0.1" $1`** — sets `$1` to:
  - `0` if `$0 == "1.0.1"`
  - `1` if `$0 > "1.0.1"`
  - `2` if `$0 < "1.0.1"` (INCLUDING the empty-string case, which compares as 0.0.0.0)

**`ReadRegStr $0 HKCU "subkey" "valuename"`** — reads to `$0`; missing key/value -> `$0` is empty string. We don't check `${errors}` because the version-compare against empty correctly returns "2".

**`nsExec::Exec '...'`** — built-in plugin shipped with every NSIS install (NOT an external plugin per D-05; bundled). Runs a command without a visible console window and pushes the exit code + output onto the stack.

**`StrCmp $1 "2" SkipLabel`** — jump to `SkipLabel` if `$1 == "2"`. NSIS uses label-style control flow (matches existing squirebot.nsi style; do NOT introduce `${If}` macros).

**Substring search:** NSIS does not have a built-in. Either include `StrFunc.nsh` (more `!include` dependency surface) or inline a tiny helper. We inline `StrContains` to match the project's pattern of minimizing includes.
</nsis_primer>
</context>

<tasks>

<task type="auto">
  <name>Task 1: Add WordFunc.nsh include + StrContains helper + REQ-ID-tagged pre-install shim block</name>
  <files>installer/squirebot.nsi</files>
  <read_first>
    - installer/squirebot.nsi (entire file — especially the `!define` block lines 26-43, `RequestExecutionLevel user` line 48, `Section "Install"` lines 77-106, and the uninstaller's `taskkill /F` at line 136 which is the Analog A template)
    - .planning/phases/06-installer-overwrite-running-shim/06-PATTERNS.md (sections "MODIFY installer/squirebot.nsi" + "No Analog Found -> NSIS VersionCompare" + "No Analog Found -> NSIS poll loop")
    - .planning/phases/06-installer-overwrite-running-shim/06-CONTEXT.md (D-02 version-gate decision, D-03 10s timeout, D-05 no plugin)
    - .planning/phases/06-installer-overwrite-running-shim/06-02-SUMMARY.md (confirms --quit handler exists in the shipped main.go)
  </read_first>
  <behavior>
    - `!include "WordFunc.nsh"` appears once near the top of the file (after the existing `!define` block, before `RequestExecutionLevel user`).
    - A `StrContains` helper function is defined at file scope.
    - The pre-install shim block exists at the TOP of `Section "Install"` BEFORE `SetOutPath "$INSTDIR"`.
    - The block has a lead comment `; -- INST-06 (overwrite-running shim) --` matching the file's existing REQ-ID comment convention (lines 4, 16, 99).
    - Version gate: if prior DisplayVersion is empty OR `< "1.0.1"`, jumps DIRECTLY to the taskkill fallback (skips `--quit` per D-02).
    - Graceful path: if prior DisplayVersion `>= "1.0.1"` AND `$INSTDIR\squirebot.exe` exists, `ExecWait '"$INSTDIR\${EXE_NAME}" --quit'`.
    - Poll: up to 10 seconds total (40 iterations × 250ms sleep), checking process existence via `tasklist /FI "IMAGENAME eq ${EXE_NAME}" /NH` and parsing output for the EXE name.
    - Fallback: `ExecWait 'taskkill /IM "${EXE_NAME}" /F'` always runs after the poll (no-op if already gone).
  </behavior>
  <action>
**Step 1: Add the include.**

Insert this single line into `installer/squirebot.nsi` AFTER the `!define REGPATH_UNINSTSUBKEY ...` line (currently line 43) and BEFORE the `; --- THE critical directive: no UAC. ---` comment (currently line 45). Add blank lines above and below for readability:

```nsis

!include "WordFunc.nsh"  ; for ${VersionCompare} used by the INST-06 pre-install shim
```

**Step 2: Add the `StrContains` helper function.**

Insert this function at file scope, AFTER the `!include "WordFunc.nsh"` line and BEFORE `RequestExecutionLevel user`. (Functions can appear anywhere outside of Sections; placing it adjacent to the include keeps related code together.)

```nsis

; StrContains: pop NEEDLE, pop HAYSTACK, push "" if not found OR the
; matching substring if found. Inlined (vs. !include StrFunc.nsh) to
; avoid adding another include dependency. Used by the INST-06
; pre-install shim's poll loop to check whether tasklist output
; mentions squirebot.exe.
Function StrContains
  Exch $R1 ; needle
  Exch
  Exch $R2 ; haystack
  Push $R3 ; counter
  Push $R4 ; substring
  Push $R5 ; needle length
  StrLen $R5 $R1
  StrCpy $R3 0
  StrContainsLoop:
    StrCpy $R4 $R2 $R5 $R3
    StrCmp $R4 $R1 StrContainsFound
    StrCmp $R4 "" StrContainsNotFound
    IntOp $R3 $R3 + 1
    Goto StrContainsLoop
  StrContainsFound:
    StrCpy $R1 $R4
    Goto StrContainsDone
  StrContainsNotFound:
    StrCpy $R1 ""
  StrContainsDone:
    Pop $R5
    Pop $R4
    Pop $R3
    Pop $R2
    Exch $R1
FunctionEnd
```

**Step 3: Add the pre-install shim block.**

Insert this entire block as the FIRST content of `Section "Install"` — immediately after the `Section "Install"` line (currently line 77) and BEFORE the `SetOutPath "$INSTDIR"` line (currently line 78). Paste verbatim:

```nsis
    ; -- INST-06 (overwrite-running shim) --
    ; Phase 6: re-running the installer over a running watcher must NOT
    ; require the user to right-click-Quit the tray first. We try a
    ; graceful --quit (Plan 06-02 added the CLI handler), poll for exit
    ; up to 10s, then fall back to taskkill /F.
    ;
    ; Version gate (D-02): the v1.0.0 binary does not recognize --quit
    ; and would spawn a DUPLICATE tray on the unknown flag. So for any
    ; prior install reporting DisplayVersion < "1.0.1" (or no prior
    ; install), we skip --quit entirely and go straight to taskkill /F.

    ; Read prior DisplayVersion from HKCU uninstaller entry. Missing
    ; value -> $0 is empty string -> VersionCompare returns "2" (less
    ; than 1.0.1) -> skips --quit. Same code path as fresh install.
    ReadRegStr $0 HKCU "${REGPATH_UNINSTSUBKEY}" "DisplayVersion"
    ${VersionCompare} "$0" "1.0.1" $1
    ; $1 == 0 : equal      (>= 1.0.1, run --quit)
    ; $1 == 1 : $0 > 1.0.1 (>= 1.0.1, run --quit)
    ; $1 == 2 : $0 < 1.0.1 or empty (skip --quit, go straight to taskkill /F)
    StrCmp $1 "2" SkipQuitSignal

    ; --quit graceful-signal path. Guard with IfFileExists so we never
    ; ExecWait against a missing binary (would surface a confusing
    ; "command not found" in the install log).
    IfFileExists "$INSTDIR\${EXE_NAME}" RunQuitSignal SkipQuitSignal

    RunQuitSignal:
        ; Fire-and-forget per D-01: the binary signals the named event
        ; and exits 0 within ~1s regardless of whether a listener was
        ; active. The watcher's listener (Plan 06-02) funnels through
        ; cancel() + systray.Quit().
        ExecWait '"$INSTDIR\${EXE_NAME}" --quit'

        ; Poll for process exit. Up to 10 seconds total (40 iterations
        ; * 250ms). We use `tasklist /FI ... /NH` via nsExec::Exec (no
        ; console flash) and parse its output for the EXE name. nsExec
        ; is built-in (bundled with NSIS) so this does NOT violate D-05.
        ; tasklist exit code is 0 in both "found" and "not found" cases,
        ; so we rely on output parsing rather than exit code.
        StrCpy $2 0  ; iteration counter
        PollLoop:
            IntCmp $2 40 PollTimedOut PollContinue PollTimedOut
            PollContinue:
                Sleep 250
                nsExec::Exec 'tasklist /FI "IMAGENAME eq ${EXE_NAME}" /NH'
                Pop $3  ; exit code (ignored)
                Pop $4  ; stdout+stderr
                Push $4
                Push "${EXE_NAME}"
                Call StrContains
                Pop $5
                StrCmp $5 "" PollDone  ; EXE name not in tasklist output -> process gone
                IntOp $2 $2 + 1
                Goto PollLoop
        PollTimedOut:
        PollDone:
    SkipQuitSignal:

    ; Always: hard-kill fallback. No-op if the process is already gone
    ; (taskkill returns 128). Copies the exact syntax from the
    ; uninstaller (squirebot.nsi:136) for consistency.
    ExecWait 'taskkill /IM "${EXE_NAME}" /F'
    ; -- end INST-06 pre-install shim --

```

**Conventions enforced (from PATTERNS.md "MODIFY installer/squirebot.nsi"):**
- Lead comment uses `; -- INST-06 (overwrite-running shim) --` matching the file's existing INST-01, INST-04 style.
- Label-style control flow (`StrCmp`, `IfFileExists`, `Goto`, `IntCmp`) — NOT `${If}` macros from `LogicLib.nsh` (the file does not `!include` LogicLib currently; consistency).
- Hard-kill fallback uses the EXACT taskkill string from line 136: `taskkill /IM "${EXE_NAME}" /F`.
- Variables: `$0` for registry read, `$1` for VersionCompare result, `$2` for poll counter, `$3-$5` for nsExec output + helper. NSIS registers `$0`-`$9` are installer-scope; no `Var /GLOBAL` needed.
- All file paths quoted: `"$INSTDIR\${EXE_NAME}"` — preserves space-containing usernames per line 101 comment.
- No new plugin includes (per D-05): only `WordFunc.nsh` (NSIS built-in) and `nsExec` (bundled plugin shipped with every NSIS install — NOT a separate download).

**Do NOT modify:**
- `RequestExecutionLevel user` (line 48) — locked by INST-01.
- The `WriteRegStr ... "DisplayVersion" "${APPVERSION}"` line (line 89) — we WRITE the new version; we READ the OLD version still present from the prior install (NSIS pre-install runs before any WriteRegStr in this section).
- The autostart `WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Run" ...` line (line 102) — D-04 unchanged.
- The `Exec '"$INSTDIR\${EXE_NAME}"'` post-install launch (line 105) — D-04 unchanged.
- The entire `Section "Uninstall"` block (lines 108-163) — uninstaller stays as-is (deferred per CONTEXT.md deferred_ideas).
  </action>
  <verify>
    <automated>powershell -NoProfile -Command "$mk = 'C:\Program Files (x86)\NSIS\makensis.exe'; if (-not (Test-Path $mk)) { $mk = 'C:\Program Files\NSIS\makensis.exe' }; if (-not (Test-Path $mk)) { Write-Host 'MISSING — Install NSIS 3.10+; CI gates makensis via release.yml. Local verification requires NSIS on PATH.'; exit 0 } else { & $mk /V2 /DAPPVERSION=1.0.1 /DAPPVERSIONNUMERIC=1.0.1.0 installer/squirebot.nsi; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; Get-Item dist/SquireBot-Setup-1.0.1.exe | Select-Object Name, Length }"</automated>
  </verify>
  <acceptance_criteria>
    - `Select-String -Path installer/squirebot.nsi -Pattern '^\!include "WordFunc\.nsh"'` matches exactly 1 line.
    - `Select-String -Path installer/squirebot.nsi -Pattern '^Function StrContains$'` matches exactly 1 line.
    - `Select-String -Path installer/squirebot.nsi -Pattern '^FunctionEnd$'` matches at least 1 line (the StrContains closer; no other functions exist in this file as of v1.0.0).
    - `Select-String -Path installer/squirebot.nsi -Pattern '; -- INST-06 \(overwrite-running shim\) --'` matches exactly 1 line.
    - `Select-String -Path installer/squirebot.nsi -Pattern 'ReadRegStr $0 HKCU "${REGPATH_UNINSTSUBKEY}" "DisplayVersion"' -SimpleMatch` matches exactly 1 line.
    - `Select-String -Path installer/squirebot.nsi -Pattern '${VersionCompare} "$0" "1.0.1" $1' -SimpleMatch` matches exactly 1 line.
    - `Select-String -Path installer/squirebot.nsi -Pattern 'StrCmp $1 "2" SkipQuitSignal' -SimpleMatch` matches exactly 1 line.
    - `Select-String -Path installer/squirebot.nsi -Pattern 'ExecWait ''"$INSTDIR\${EXE_NAME}" --quit' -SimpleMatch` matches exactly 1 line.
    - `Select-String -Path installer/squirebot.nsi -Pattern 'nsExec::Exec ''tasklist /FI "IMAGENAME eq ${EXE_NAME}" /NH' -SimpleMatch` matches exactly 1 line.
    - `Select-String -Path installer/squirebot.nsi -Pattern 'ExecWait ''taskkill /IM "${EXE_NAME}" /F' -SimpleMatch` matches at least 2 lines (existing uninstaller line 136 + the new pre-install fallback). Exactly 2 expected.
    - Ordering check: `Select-String -Path installer/squirebot.nsi -Pattern 'Section "Install"' -SimpleMatch` line N1; `Select-String -Path installer/squirebot.nsi -Pattern 'ReadRegStr $0 HKCU "${REGPATH_UNINSTSUBKEY}" "DisplayVersion"' -SimpleMatch` line N2; `Select-String -Path installer/squirebot.nsi -Pattern 'SetOutPath "$INSTDIR"' -SimpleMatch` line N3 (the first occurrence). Verify N1 < N2 < N3.
    - `Select-String -Path installer/squirebot.nsi -Pattern 'RequestExecutionLevel user'` matches exactly 1 line (still present, unchanged from line 48).
    - `Select-String -Path installer/squirebot.nsi -Pattern 'Exec ''"$INSTDIR\${EXE_NAME}"''' -SimpleMatch` matches exactly 1 line (the post-install launch line 105 is preserved).
    - `Select-String -Path installer/squirebot.nsi -Pattern 'WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "SquireBot"' -SimpleMatch` matches exactly 1 line (autostart Run-key preserved).
    - IF NSIS is installed on the executor's machine: `makensis /V2 /DAPPVERSION=1.0.1 /DAPPVERSIONNUMERIC=1.0.1.0 installer/squirebot.nsi` exits 0 and `Test-Path dist/SquireBot-Setup-1.0.1.exe` returns True. IF NSIS is not installed locally: skip this check (CI in release.yml is the authoritative gate per release workflow lines 75-100) but document the skip in the SUMMARY.
  </acceptance_criteria>
  <done>
    `installer/squirebot.nsi` compiles cleanly (locally or in CI), the pre-install shim is at the top of the Install section, the version gate works for both the v1.0.0 legacy path (skip --quit) and the v1.0.1+ path (try --quit, poll, fallback), and all locked decisions (D-01..D-05) are honored. The autostart + post-install Exec lines are untouched, satisfying D-04 and ROADMAP §46.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| HKCU registry → installer behavior | `DisplayVersion` is read from a user-writable registry path; a malicious user-session process could overwrite it to bypass the version gate. |
| `taskkill /IM "${EXE_NAME}"` → process termination | Targets ANY process named `squirebot.exe` in the user session. A confused-deputy attack would require an attacker to register a different process under this exact name in the user session. |
| Pre-install execution context | The shim runs at user integrity (`RequestExecutionLevel user`). Any privilege escalation here is a regression. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-06-12 | Spoofing | HKCU DisplayVersion overwritten to "1.0.0" by malicious user-session process | accept | If an attacker has user-session write access to HKCU, they can overwrite far worse values (the autostart Run-key payload, the entire SquireBot binary). Bypassing the version gate would force the legacy taskkill /F path — which still results in a clean upgrade, just slightly less graceful. No additional attack surface opened. Per CONTEXT.md security framing: local Windows surface only. |
| T-06-13 | Tampering | `taskkill /IM "squirebot.exe" /F` targets exact name in user session — could be confused-deputy for a different user-spawned process named squirebot.exe | mitigate | The /IM filter does exact name match (NOT pattern match) per CONTEXT.md security context. Without /F, taskkill targets only the user's own processes (per Windows OS contract — non-admin cannot signal other users' processes). The risk is bounded to "another process under the user's own session also named squirebot.exe gets killed" — which is the documented intent of the shim. |
| T-06-14 | Elevation of Privilege | Pre-install shim inadvertently invokes a privileged operation | mitigate | All operations are user-integrity: `ReadRegStr HKCU` (no admin), `IfFileExists` (filesystem read), `ExecWait '"$INSTDIR\..."'` (runs at the installer's own integrity = user), `nsExec::Exec 'tasklist'` (user-scoped enumeration), `Sleep` (no privilege), `taskkill /IM /F` (user-session-scoped without /S). `RequestExecutionLevel user` on line 48 remains the file-scope lock. Verified: no `HKLM` reads, no `SYSTEM`-targeted taskkill, no `ExecShell` with admin verbs. |
| T-06-15 | Denial of Service | Poll loop hangs the installer for 10s if a hostile process keeps re-spawning squirebot.exe | accept | Bounded by the 40-iteration cap (10s wall-clock). After 10s the loop exits via PollTimedOut and the taskkill /F fires. Hostile re-spawn would require attacker code in user session — see T-06-12. Worst case: user sees "Installing..." for 10s, then taskkill /F succeeds, install completes. |
| T-06-16 | Information Disclosure | Install log records the EXE name and HKCU path | accept | Same info already visible in NSIS install logs from the existing INST-01/INST-04 lines and the uninstaller's taskkill at line 136. No new sensitive data exposed. |

ASVS L1: no `high` severity threats. All operations remain at user integrity; no UAC prompt added; no `Global\` namespace touched. Mitigations leverage existing Phase 1 invariants (`RequestExecutionLevel user`, HKCU-only, exact-name taskkill filter).
</threat_model>

<verification>
- `installer/squirebot.nsi` parses cleanly under `makensis /V2 /DAPPVERSION=1.0.1 /DAPPVERSIONNUMERIC=1.0.1.0 installer/squirebot.nsi` (locally or in CI).
- Produced installer (if locally built): `dist/SquireBot-Setup-1.0.1.exe` exists and is a valid PE32 executable.
- Live smoke (deferred to Plan 05 release gate, NOT required to mark this plan done): install v1.0.0 on a clean Win11 VM, let watcher reach steady state, then run v1.0.1 installer — verify tray icon flickers and reappears with no manual stop prompt.
- Static checks (all required to mark done):
  - `RequestExecutionLevel user` still present (line ~50 post-edit).
  - Post-install `Exec '"$INSTDIR\${EXE_NAME}"'` still present (line ~150 post-edit).
  - Autostart `WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Run" ...` still present.
  - Uninstaller `Section "Uninstall"` block unchanged (the existing `taskkill /F` at what was line 136 is now at a higher line number post-insert but the line itself is intact).
</verification>

<success_criteria>
- `installer/squirebot.nsi` has the `!include "WordFunc.nsh"`, the `StrContains` function, and the pre-install shim block at the top of `Section "Install"`.
- Version gate correctly skips `--quit` for prior versions `< "1.0.1"` (including the empty-string fresh-install case).
- Poll loop uses bundled NSIS primitives only (`nsExec`, `Sleep`, `IntCmp`, `Goto`) — no external plugin dependency per D-05.
- Hard-kill fallback always runs; uses the exact verbatim syntax from the uninstaller's line 136.
- ROADMAP §44 success criterion 1 covered: no manual stop step.
- ROADMAP §45 success criterion 2 covered end-to-end: signal (--quit) + wait (10s poll) + fallback (taskkill /F).
- ROADMAP §46 success criterion 3 covered: post-install Exec line is unchanged, so the new watcher autostarts immediately AND HKCU Run-key autostart at next logon both still work. Token (in wincred) and config.json (in %LOCALAPPDATA%) are not touched by the installer — no re-auth required.
- D-01, D-02, D-03, D-04, D-05 all honored.
</success_criteria>

<output>
After completion, create `.planning/phases/06-installer-overwrite-running-shim/06-03-SUMMARY.md` capturing:
- Final line numbers of the shim block, the `!include`, and the `StrContains` function.
- Whether NSIS was available locally for the makensis compile check, OR the explicit deferral note that CI will compile it on tag push (Plan 05).
- Confirmation that the 4 untouched anchors are intact: `RequestExecutionLevel user`, autostart Run-key write, post-install Exec, uninstaller Section.
- Confirmation that `nsExec` is bundled (not a separate plugin) so D-05 is not violated.
</output>
</content>
</invoke>