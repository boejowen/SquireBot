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

!define APPNAME    "SquireBot"
!define EXE_NAME   "squirebot.exe"
!define PUBLISHER  "boejowen"
!define ABOUTURL   "https://github.com/boejowen/SquireBot"
!define REGPATH_UNINSTSUBKEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APPNAME}"

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

VIProductVersion "${APPVERSION}.0"
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
