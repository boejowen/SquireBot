; installer/squirebot.nsi
; SquireBot per-user NSIS installer (NSIS 3.10+).
;
; -- INST-01 (no UAC, no command-line steps) --
; This installer satisfies INST-01 by writing only to %LOCALAPPDATA% and HKCU.
; The directive `RequestExecutionLevel user` explicitly overrides the NSIS
; auto-elevate heuristic that would otherwise fire on filenames containing
; "setup" or "install" (RESEARCH.md §6.2 row 2).
;
; -- D-13 (Phase 1 ships unsigned) --
; The produced SquireBot-Setup-X.Y.Z.exe is NOT code-signed in Phase 1. The
; first run on a clean Win11 VM will hit Microsoft Defender SmartScreen with
; an "Unknown publisher" wall. The user clicks "More info -> Run anyway".
; This is the documented path; Phase 2 adds code signing.
;
; -- INST-04 (autostart) --
; Per-user HKCU\Software\Microsoft\Windows\CurrentVersion\Run\SquireBot
; pointing at $INSTDIR\squirebot.exe. No UAC needed (HKCU, not HKLM).
; The uninstaller removes this key unconditionally.
;
; -- Build invocation --
;   makensis -DAPPVERSION=0.1.0 -V2 installer\squirebot.nsi
; CI passes -DAPPVERSION=<tag-stripped-of-leading-v>. Local rebuild is
; documented in docs/build-and-install.md.

!ifndef APPVERSION
    !define APPVERSION "0.1.0"
!endif

; APPVERSIONNUMERIC is the strict X.X.X.X numeric form NSIS VIProductVersion
; requires (it rejects prerelease suffixes like '-rc1'). CI passes both
; APPVERSION (display, may include '-rc1') and APPVERSIONNUMERIC (numeric).
; For local builds without a prerelease suffix, the fallback "${APPVERSION}.0"
; works because non-prerelease APPVERSION values like "0.1.0" pad cleanly.
!ifndef APPVERSIONNUMERIC
    !define APPVERSIONNUMERIC "${APPVERSION}.0"
!endif

!define APPNAME    "SquireBot"
!define EXE_NAME   "squirebot.exe"
!define PUBLISHER  "boejowen"
!define ABOUTURL   "https://github.com/boejowen/SquireBot"
!define REGPATH_UNINSTSUBKEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APPNAME}"

!include "WordFunc.nsh"  ; for ${VersionCompare} used by the INST-06 pre-install shim

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

; --- THE critical directive: no UAC. ---
; Pitfall #7 enforcement (RESEARCH.md §6.2): explicit `user` overrides the
; filename heuristic that would otherwise auto-request elevation.
RequestExecutionLevel user

Name           "${APPNAME}"
OutFile        "..\dist\SquireBot-Setup-${APPVERSION}.exe"
Unicode        true
SetCompressor  /SOLID lzma
ShowInstDetails show
Icon           "icon.ico"
UninstallIcon  "icon.ico"
BrandingText   "${APPNAME} ${APPVERSION}"

; Install path: %LOCALAPPDATA%\Programs\SquireBot. Never under Program Files
; (that would need UAC). RESEARCH.md §6.1 + §6.2 explicit.
InstallDir       "$LOCALAPPDATA\Programs\${APPNAME}"
InstallDirRegKey HKCU "${REGPATH_UNINSTSUBKEY}" "InstallLocation"

VIProductVersion "${APPVERSIONNUMERIC}"
VIAddVersionKey  "ProductName"      "${APPNAME}"
VIAddVersionKey  "CompanyName"      "${PUBLISHER}"
VIAddVersionKey  "FileDescription"  "SquireBot per-guildie watcher (P99 inventory -> Google Sheets)"
VIAddVersionKey  "FileVersion"      "${APPVERSION}"
VIAddVersionKey  "ProductVersion"   "${APPVERSION}"
VIAddVersionKey  "LegalCopyright"   ""

Page directory
Page instfiles
UninstPage uninstConfirm
UninstPage instfiles

Section "Install"
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

    SetOutPath "$INSTDIR"

    ; Payload: the watcher binary built upstream by `go build` and the
    ; installer's own icon (also used as the Add/Remove Programs DisplayIcon).
    File "..\dist\${EXE_NAME}"
    File "icon.ico"

    ; Uninstaller registration -- HKCU only (no admin needed).
    ; These six values populate the per-user "Add or Remove Programs" entry.
    WriteUninstaller "$INSTDIR\uninstall.exe"
    WriteRegStr HKCU "${REGPATH_UNINSTSUBKEY}" "DisplayName"          "${APPNAME}"
    WriteRegStr HKCU "${REGPATH_UNINSTSUBKEY}" "DisplayVersion"       "${APPVERSION}"
    WriteRegStr HKCU "${REGPATH_UNINSTSUBKEY}" "InstallLocation"      "$INSTDIR"
    WriteRegStr HKCU "${REGPATH_UNINSTSUBKEY}" "DisplayIcon"          "$INSTDIR\${EXE_NAME}"
    WriteRegStr HKCU "${REGPATH_UNINSTSUBKEY}" "Publisher"            "${PUBLISHER}"
    WriteRegStr HKCU "${REGPATH_UNINSTSUBKEY}" "URLInfoAbout"         "${ABOUTURL}"
    WriteRegStr HKCU "${REGPATH_UNINSTSUBKEY}" "UninstallString"      '"$INSTDIR\uninstall.exe"'
    WriteRegStr HKCU "${REGPATH_UNINSTSUBKEY}" "QuietUninstallString" '"$INSTDIR\uninstall.exe" /S'
    WriteRegDWORD HKCU "${REGPATH_UNINSTSUBKEY}" "NoModify" 1
    WriteRegDWORD HKCU "${REGPATH_UNINSTSUBKEY}" "NoRepair" 1

    ; INST-04: autostart on logon. Per-user Run key, no UAC required.
    ; The double-quoted value handles $INSTDIR paths containing spaces
    ; (e.g., usernames with spaces). The Run-key parser respects quoting.
    WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "SquireBot" '"$INSTDIR\${EXE_NAME}"'

    ; Phase 1: launch the wizard immediately after install.
    Exec '"$INSTDIR\${EXE_NAME}"'
SectionEnd

Section "Uninstall"
    ; Phase 2 (CONTEXT.md Q3): ask whether to fully wipe (config.json +
    ; wincred token). Default focus = No (preserve), so reinstalling later
    ; resumes work without re-OAuth. MB_DEFBUTTON2 puts focus on No.
    Var /GLOBAL UninstallWipe
    StrCpy $UninstallWipe "0"
    MessageBox MB_YESNO|MB_ICONQUESTION|MB_DEFBUTTON2 \
        "Also delete saved configuration and Google account credentials?$\r$\n$\r$\n\
        Yes = full wipe (you will need to re-authenticate Google on next install).$\r$\n\
        No  = preserve config.json and wincred (recommended; default)." \
        IDYES UninstallWipeYes IDNO UninstallWipeNo
    UninstallWipeYes:
        StrCpy $UninstallWipe "1"
        Goto UninstallWipeDone
    UninstallWipeNo:
        StrCpy $UninstallWipe "0"
    UninstallWipeDone:

    ; If full wipe requested AND the binary still exists, run it to
    ; delete the wincred entry BEFORE we delete the binary. NSIS doesn't
    ; speak DPAPI; only the Go binary can.
    StrCmp $UninstallWipe "1" 0 SkipWipeBinary
    IfFileExists "$INSTDIR\${EXE_NAME}" RunWipeBinary SkipWipeBinary
    RunWipeBinary:
        ExecWait '"$INSTDIR\${EXE_NAME}" --uninstall-wipe-credentials'
    SkipWipeBinary:

    ; Always: stop running instance before deleting the .exe (graceful kill).
    ExecWait 'taskkill /IM "${EXE_NAME}" /F'

    ; Always: remove binary, icon, uninstaller.
    Delete "$INSTDIR\${EXE_NAME}"
    Delete "$INSTDIR\icon.ico"
    Delete "$INSTDIR\uninstall.exe"
    RMDir  "$INSTDIR"

    ; Always: remove rotated log files (low-sensitivity; preserving them
    ; serves no recovery purpose).
    Delete "$LOCALAPPDATA\${APPNAME}\squirebot.log"
    Delete "$LOCALAPPDATA\${APPNAME}\squirebot.log.*"

    ; Conditional: full wipe deletes config.json.
    StrCmp $UninstallWipe "1" 0 SkipConfigDelete
    Delete "$LOCALAPPDATA\${APPNAME}\config.json"
    SkipConfigDelete:

    ; Always: try to remove the SquireBot LOCALAPPDATA dir (succeeds
    ; only if empty -- preserves config.json under preserve-mode).
    RMDir "$LOCALAPPDATA\${APPNAME}"

    ; Always: remove autostart Run-key value (Phase 2 INST-04 cleanup).
    DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "SquireBot"

    ; Always: remove uninstall registry subkey.
    DeleteRegKey HKCU "${REGPATH_UNINSTSUBKEY}"
SectionEnd
