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
; -- INST-04 (autostart) is DEFERRED TO PHASE 2 --
; Phase 1 deliberately does NOT register HKCU\Software\Microsoft\Windows\
; CurrentVersion\Run. The watcher only runs when launched explicitly. This
; keeps the Phase 1 install reversible and avoids surprising guildies whose
; OAuth grant might silently expire while they aren't watching.
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

    ; INST-04 autostart is Phase 2. Phase 1 does NOT write the Run key --
    ; the watcher only runs when launched explicitly (post-install Exec
    ; below, or by the user from Start Menu / file explorer).

    ; Phase 1: launch the wizard immediately after install.
    Exec '"$INSTDIR\${EXE_NAME}"'
SectionEnd

Section "Uninstall"
    ; Stop a running instance before deleting the .exe (graceful kill).
    ExecWait 'taskkill /IM "${EXE_NAME}" /F'

    Delete "$INSTDIR\${EXE_NAME}"
    Delete "$INSTDIR\icon.ico"
    Delete "$INSTDIR\uninstall.exe"
    RMDir  "$INSTDIR"

    ; Cleanup user data per Phase 1 uninstall policy.
    Delete "$LOCALAPPDATA\${APPNAME}\config.json"
    Delete "$LOCALAPPDATA\${APPNAME}\squirebot.log"
    Delete "$LOCALAPPDATA\${APPNAME}\squirebot.log.*"
    RMDir  "$LOCALAPPDATA\${APPNAME}"

    ; NOTE: wincred entry SquireBot:<email> is NOT auto-deleted -- DPAPI
    ; tokens survive uninstall by design (re-install reuses the cached
    ; refresh token, sparing the guildie a second OAuth round trip).
    ; Manual cleanup if a guildie wants a full wipe:
    ;   cmdkey /list                              (find SquireBot:<email>)
    ;   cmdkey /delete:SquireBot:<email>
    ; Documented in docs/build-and-install.md "Uninstalling".

    DeleteRegKey HKCU "${REGPATH_UNINSTSUBKEY}"
SectionEnd
