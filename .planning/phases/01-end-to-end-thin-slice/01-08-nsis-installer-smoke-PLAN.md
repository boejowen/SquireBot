---
phase: 01-end-to-end-thin-slice
plan: 08
type: execute
wave: 6
depends_on: [01, 02, 07]
files_modified:
  - installer/squirebot.nsi
  - installer/icon.ico
  - .github/workflows/release.yml
  - docs/build-and-install.md
  - .planning/phases/01-end-to-end-thin-slice/smoke-checklist.md
autonomous: false
requirements: [INST-01]
must_haves:
  truths:
    - "`makensis -V2 installer/squirebot.nsi` produces `dist/SquireBot-Setup-X.Y.Z.exe` from a built `dist/squirebot.exe` with NO errors"
    - "Running the installer on a clean Windows 11 VM as a non-admin user shows the SmartScreen 'unknown publisher' wall (per D-13), then after 'More info → Run anyway' the NSIS wizard opens with NO second UAC prompt"
    - "Files install to %LOCALAPPDATA%\\Programs\\SquireBot — NOT to Program Files"
    - "The HKCU Uninstall registry entry is created at HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\SquireBot with DisplayName, DisplayVersion, InstallLocation, UninstallString"
    - "After install, squirebot.exe auto-launches; tray icon appears; default browser opens to http://127.0.0.1:&lt;port&gt;/start"
    - "Within 30 seconds of dropping a sample &lt;Char&gt;-Inventory.txt into the chosen EQ folder, an inv:&lt;Char&gt; tab appears in the picked workbook with the parsed 5-column rows"
    - "wincred shows entry SquireBot:&lt;email&gt;; %LOCALAPPDATA%\\SquireBot\\config.json contains google_email but NO refresh_token-shaped value"
    - "Phase 1 success criterion #5 (10-day token survival) is left as a SCHEDULED follow-up checkpoint — Plan 08 records the install date and the developer revisits on day 10+"
  artifacts:
    - path: "installer/squirebot.nsi"
      provides: "NSIS 3.10+ per-user installer producing SquireBot-Setup-X.Y.Z.exe"
      min_lines: 50
      contains: "RequestExecutionLevel user"
    - path: "docs/build-and-install.md"
      provides: "End-to-end build + sign + install runbook for the developer's local machine"
      min_lines: 40
    - path: ".planning/phases/01-end-to-end-thin-slice/smoke-checklist.md"
      provides: "Numbered checklist matching the 5 ROADMAP success criteria, used by Task 3 checkpoint and recorded in 01-08-SUMMARY.md"
  key_links:
    - from: "installer/squirebot.nsi"
      to: "$LOCALAPPDATA\\Programs\\SquireBot"
      via: "InstallDir directive"
      pattern: "LOCALAPPDATA.*SquireBot"
    - from: "installer/squirebot.nsi"
      to: "HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\SquireBot"
      via: "WriteRegStr HKCU during Section Install"
      pattern: "WriteRegStr HKCU"
    - from: ".github/workflows/release.yml"
      to: "installer/squirebot.nsi"
      via: "CI step calls makensis after building squirebot.exe"
      pattern: "makensis"
---

<objective>
Wrap the Plan 07 binary in an NSIS per-user installer that satisfies INST-01 (no UAC, single .exe,
no command-line steps) and execute the load-bearing Phase 1 smoke test against a clean Windows 11
VM. This is the final Phase 1 plan — its success checkpoint is what proves Plans 01-07 actually
work end-to-end on a fresh machine, not just on the dev's primed environment.

Purpose: Phase 1 ROADMAP success criteria 1-5 all collapse to "does this thing work on a clean
Win11 VM the way we say it does?" Plan 08 is where we prove that. INST-01 is the only requirement
formally owned here; the smoke checkpoint validates the entire phase.

Output: A signed-or-unsigned (D-13: unsigned acceptable for Phase 1) NSIS installer at
`dist/SquireBot-Setup-X.Y.Z.exe`, a build runbook documenting the local build sequence, a smoke
checklist whose execution is recorded in 01-08-SUMMARY.md, and an updated GitHub Actions workflow
that produces release artefacts on tag push. Also: a 10-day-later follow-up checkpoint for AUTH-03
validation (success criterion #5).
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/phases/01-end-to-end-thin-slice/01-CONTEXT.md
@.planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md
@.planning/phases/01-end-to-end-thin-slice/oauth-config.json
@./CLAUDE.md
@cmd/squirebot/main.go
@.github/workflows/release.yml
</context>

<tasks>

<task type="auto" tdd="false">
  <name>Task 1: NSIS installer script (per-user, no UAC) + build runbook</name>
  <files>installer/squirebot.nsi, installer/icon.ico, docs/build-and-install.md</files>
  <read_first>
    - .planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md (§6 entire — §6.1 minimum directives lines 758-822 — copy verbatim with version variable substitutions; §6.2 UAC trip avoidance lines 824-836; §6.3 silent install + auto-update interaction; §6.4 smoke test expectations)
    - .planning/phases/01-end-to-end-thin-slice/01-CONTEXT.md (D-12 GitHub Releases hosting; D-13 unsigned in Phase 1; D-14 minimal README)
    - ./CLAUDE.md ("NSIS 3.10+ per-user installer (no UAC), autostart via HKCU\\...\\Run (note: autostart is INST-04, deferred to Phase 2)")
    - cmd/squirebot/main.go (Plan 07 — confirms the binary name `squirebot.exe` and Version variable)
    - assets/icon.ico (Plan 01 — base icon to also use for installer)
  </read_first>
  <action>
    Create `installer/squirebot.nsi` containing the EXACT NSIS script from RESEARCH.md §6.1 lines
    760-821, parameterised by version. Reproduce verbatim, with the only change being `APPVERSION`
    pulled from a `!define` that CI can override:

    ```nsi
    ; installer/squirebot.nsi
    ; SquireBot per-user installer (NSIS 3.10+).
    ; Per CONTEXT.md D-13: Phase 1 ships unsigned. SmartScreen "More info → Run anyway"
    ; walkthrough is the documented Phase 1 install path.
    ;
    ; Per CLAUDE.md / RESEARCH.md §6: NO UAC. Install to %LOCALAPPDATA%\Programs\SquireBot.
    ; Autostart via HKCU\...\Run is INST-04, DEFERRED TO PHASE 2 — Phase 1 does NOT register it.

    !ifndef APPVERSION
        !define APPVERSION "0.1.0"
    !endif

    !define APPNAME    "SquireBot"
    !define EXE_NAME   "squirebot.exe"
    !define REGPATH_UNINSTSUBKEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APPNAME}"

    ; --- THE critical directive: no UAC. ---
    RequestExecutionLevel user

    Name           "${APPNAME}"
    OutFile        "..\dist\SquireBot-Setup-${APPVERSION}.exe"
    Unicode        true
    SetCompressor  /SOLID lzma
    ShowInstDetails show

    ; Install path: %LOCALAPPDATA%\Programs\SquireBot. Never under Program Files (would need UAC).
    InstallDir       "$LOCALAPPDATA\Programs\${APPNAME}"
    InstallDirRegKey HKCU "${REGPATH_UNINSTSUBKEY}" "InstallLocation"

    Page directory
    Page instfiles
    UninstPage uninstConfirm
    UninstPage instfiles

    Section "Install"
        SetOutPath "$INSTDIR"
        File "..\dist\${EXE_NAME}"
        File "icon.ico"

        ; Uninstaller registration — HKCU only (no admin needed).
        WriteUninstaller "$INSTDIR\uninstall.exe"
        WriteRegStr HKCU "${REGPATH_UNINSTSUBKEY}" "DisplayName"          "${APPNAME}"
        WriteRegStr HKCU "${REGPATH_UNINSTSUBKEY}" "DisplayVersion"       "${APPVERSION}"
        WriteRegStr HKCU "${REGPATH_UNINSTSUBKEY}" "InstallLocation"      "$INSTDIR"
        WriteRegStr HKCU "${REGPATH_UNINSTSUBKEY}" "DisplayIcon"          "$INSTDIR\${EXE_NAME}"
        WriteRegStr HKCU "${REGPATH_UNINSTSUBKEY}" "UninstallString"      '"$INSTDIR\uninstall.exe"'
        WriteRegStr HKCU "${REGPATH_UNINSTSUBKEY}" "QuietUninstallString" '"$INSTDIR\uninstall.exe" /S'
        WriteRegDWORD HKCU "${REGPATH_UNINSTSUBKEY}" "NoModify" 1
        WriteRegDWORD HKCU "${REGPATH_UNINSTSUBKEY}" "NoRepair" 1

        ; INST-04 autostart is Phase 2. Phase 1 does NOT write the Run key.

        ; Phase 1: launch the wizard immediately after install.
        Exec '"$INSTDIR\${EXE_NAME}"'
    SectionEnd

    Section "Uninstall"
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

        ; NOTE: wincred entry SquireBot:<email> is NOT auto-deleted — DPAPI tokens
        ; survive uninstall by design (re-install reuses the cached refresh token).
        ; Manual cleanup: cmdkey /delete:SquireBot:<email>

        DeleteRegKey HKCU "${REGPATH_UNINSTSUBKEY}"
    SectionEnd
    ```

    Copy `assets/icon.ico` from Plan 01 to `installer/icon.ico` so the .nsi `File "icon.ico"`
    directive resolves. (Or use `File "..\assets\icon.ico"` and skip the duplicate.)

    Create `docs/build-and-install.md` — a runbook for the developer. Required sections:

    ## Prerequisites
    - Go 1.24+ (`go version`)
    - NSIS 3.10+ (`makensis /VERSION` should report 3.10 or later)
    - Optional: PowerShell 7+ for the local build script
    - oauth-config.json filled by Plan 02

    ## Building the binary
    Bash one-liner that loads oauth-config.json values and runs `go build` (mirror the README
    block from Plan 03):
    ```bash
    OAUTH_CLIENT_ID=$(jq -r '.oauth_client_id' .planning/phases/01-end-to-end-thin-slice/oauth-config.json)
    PICKER_API_KEY=$(jq -r '.picker_api_key' .planning/phases/01-end-to-end-thin-slice/oauth-config.json)
    GCP_PROJECT_NUMBER=$(jq -r '.gcp_project_number' .planning/phases/01-end-to-end-thin-slice/oauth-config.json)
    VERSION="0.1.0"

    GOOS=windows GOARCH=amd64 go build \
      -ldflags="-H=windowsgui -s -w \
                -X main.OAuthClientID=${OAUTH_CLIENT_ID} \
                -X main.PickerAPIKey=${PICKER_API_KEY} \
                -X main.GCPProjectNumber=${GCP_PROJECT_NUMBER} \
                -X main.Version=${VERSION}" \
      -o dist/squirebot.exe ./cmd/squirebot
    ```

    PowerShell variant for Windows-native development.

    ## Building the installer
    ```bash
    cd installer
    makensis -DAPPVERSION=0.1.0 -V2 squirebot.nsi
    # Produces ../dist/SquireBot-Setup-0.1.0.exe
    ```

    ## Installing on a clean Windows 11 VM (manual)
    Step-by-step to support Task 3's smoke checkpoint. Reference RESEARCH.md §6.4 expectations.

    ## Uninstalling
    - Either: Settings → Apps → SquireBot → Uninstall (uses HKCU registration)
    - Or: `%LOCALAPPDATA%\Programs\SquireBot\uninstall.exe`
    - Manual wincred cleanup: `cmdkey /list | findstr SquireBot` to enumerate, `cmdkey /delete:SquireBot:<email>`

    ## SmartScreen "More info → Run anyway" walkthrough (D-13 Phase 1 documented path)
    Step-by-step screenshots placeholder (Plan 08 ships text only; Phase 5 polish adds images
    per DOC-03).
  </action>
  <verify>
    <automated>test -s installer/squirebot.nsi &amp;&amp; grep -q 'RequestExecutionLevel user' installer/squirebot.nsi &amp;&amp; grep -q 'LOCALAPPDATA\\\\Programs\\\\${APPNAME}\|LOCALAPPDATA.Programs.SquireBot' installer/squirebot.nsi &amp;&amp; grep -q 'WriteRegStr HKCU' installer/squirebot.nsi &amp;&amp; grep -q 'DisplayName' installer/squirebot.nsi &amp;&amp; grep -q 'UninstallString' installer/squirebot.nsi &amp;&amp; ! grep -q 'RequestExecutionLevel admin' installer/squirebot.nsi &amp;&amp; ! grep -q 'RequestExecutionLevel highest' installer/squirebot.nsi &amp;&amp; ! grep -q 'PROGRAMFILES' installer/squirebot.nsi &amp;&amp; ! grep -q 'WriteRegStr HKLM' installer/squirebot.nsi &amp;&amp; test -s docs/build-and-install.md &amp;&amp; grep -q "makensis" docs/build-and-install.md &amp;&amp; grep -q "oauth_client_id" docs/build-and-install.md &amp;&amp; grep -q "More info" docs/build-and-install.md &amp;&amp; (test -s installer/icon.ico || test -s assets/icon.ico)</automated>
  </verify>
  <acceptance_criteria>
    - `installer/squirebot.nsi` exists and is at least 50 lines
    - Contains literal `RequestExecutionLevel user` (Pitfall #7 enforcement; Critical path)
    - Contains `$LOCALAPPDATA\Programs\${APPNAME}` (or equivalent literal) — installs per-user
    - Contains `WriteRegStr HKCU` for the Uninstall subkey
    - Does NOT contain `RequestExecutionLevel admin` or `RequestExecutionLevel highest`
    - Does NOT write to `$PROGRAMFILES` or `HKLM` (per RESEARCH.md §6.2 UAC triggers)
    - Does NOT contain a Run-key registration (INST-04 is Phase 2 — `grep -i "Run"` should not find Software\\Microsoft\\Windows\\CurrentVersion\\Run)
    - Has BOTH a Section "Install" AND a Section "Uninstall"
    - Uninstaller wipes `%LOCALAPPDATA%\\SquireBot\\config.json` and the log files
    - `installer/icon.ico` (or `assets/icon.ico` referenced from .nsi) exists
    - `docs/build-and-install.md` exists with at least 40 lines and references makensis + oauth_client_id + SmartScreen
  </acceptance_criteria>
  <done>
    NSIS script is RESEARCH.md §6.1-faithful, no-UAC by construction (RequestExecutionLevel user),
    HKCU-only registry writes, %LOCALAPPDATA% install path. docs/build-and-install.md is the
    reproducible runbook for Task 3's smoke checkpoint.
  </done>
</task>

<task type="auto" tdd="false">
  <name>Task 2: GitHub Actions release workflow + smoke checklist scaffold</name>
  <files>.github/workflows/release.yml, .planning/phases/01-end-to-end-thin-slice/smoke-checklist.md</files>
  <read_first>
    - .planning/phases/01-end-to-end-thin-slice/01-CONTEXT.md (D-12 GitHub Releases canonical install URL; D-13 unsigned Phase 1; D-14 README)
    - .planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md (§6.4 Smoke Test expectations lines 842-849; §1 build invocation)
    - .planning/ROADMAP.md (Phase 1 Success Criteria 1-5 — these are the smoke checklist items)
    - .planning/phases/01-end-to-end-thin-slice/oauth-config.json (Plan 02 — values used at build time)
    - .github/workflows/release.yml (Plan 01 stub — extending here)
  </read_first>
  <action>
    Replace `.github/workflows/release.yml` with the full Phase 1 release pipeline (still
    minimal — no signing, no goreleaser per D-13). Use `windows-latest` runner; install NSIS via
    chocolatey or download; build squirebot.exe with ldflags from oauth-config.json; run makensis;
    upload `SquireBot-Setup-*.exe` and `latest.json` (stub) to the GitHub Release.

    ```yaml
    name: release
    on:
      push:
        tags: ['v*']
      workflow_dispatch:
        inputs:
          version:
            description: "Version (e.g., 0.1.0)"
            required: true

    jobs:
      release:
        runs-on: windows-latest
        permissions:
          contents: write   # to attach artifacts to the GitHub Release
        steps:
          - uses: actions/checkout@v4

          - uses: actions/setup-go@v5
            with:
              go-version: '1.24'

          - name: Install NSIS
            run: choco install nsis --no-progress --version=3.10 || choco install nsis --no-progress

          - name: Verify NSIS version
            shell: pwsh
            run: |
              $v = & "C:\Program Files (x86)\NSIS\makensis.exe" /VERSION
              Write-Host "NSIS version: $v"
              if (-not ($v -match 'v3\.(1[0-9]|[2-9][0-9])')) {
                Write-Error "NSIS version $v is &lt; 3.10 — aborting (RESEARCH.md §1)"
                exit 1
              }

          - name: Compute version
            id: ver
            shell: pwsh
            run: |
              if ($env:GITHUB_REF -match '^refs/tags/v(.+)$') {
                "version=$($Matches[1])" | Out-File $env:GITHUB_OUTPUT -Append
              } else {
                "version=${{ github.event.inputs.version }}" | Out-File $env:GITHUB_OUTPUT -Append
              }

          - name: Load OAuth constants from oauth-config.json
            id: oauth
            shell: pwsh
            run: |
              $cfg = Get-Content .planning/phases/01-end-to-end-thin-slice/oauth-config.json | ConvertFrom-Json
              if ($cfg.consent_screen_status -ne "PRODUCTION") {
                Write-Error "oauth-config.json consent_screen_status is '$($cfg.consent_screen_status)' — must be 'PRODUCTION' before release per AUTH-03 / Plan 02"
                exit 1
              }
              "client_id=$($cfg.oauth_client_id)" | Out-File $env:GITHUB_OUTPUT -Append
              "api_key=$($cfg.picker_api_key)" | Out-File $env:GITHUB_OUTPUT -Append
              "project_num=$($cfg.gcp_project_number)" | Out-File $env:GITHUB_OUTPUT -Append

          - name: Build squirebot.exe
            shell: pwsh
            env:
              GOOS: windows
              GOARCH: amd64
            run: |
              New-Item -ItemType Directory -Force -Path dist | Out-Null
              go build -ldflags="-H=windowsgui -s -w `
                -X main.OAuthClientID=${{ steps.oauth.outputs.client_id }} `
                -X main.PickerAPIKey=${{ steps.oauth.outputs.api_key }} `
                -X main.GCPProjectNumber=${{ steps.oauth.outputs.project_num }} `
                -X main.Version=${{ steps.ver.outputs.version }}" `
                -o dist/squirebot.exe ./cmd/squirebot

          - name: Build NSIS installer
            shell: pwsh
            run: |
              & "C:\Program Files (x86)\NSIS\makensis.exe" /V2 /DAPPVERSION=${{ steps.ver.outputs.version }} installer/squirebot.nsi

          - name: Compute SHA-256 for latest.json
            id: hash
            shell: pwsh
            run: |
              $h = (Get-FileHash dist/SquireBot-Setup-${{ steps.ver.outputs.version }}.exe -Algorithm SHA256).Hash.ToLower()
              "sha256=$h" | Out-File $env:GITHUB_OUTPUT -Append

          - name: Write latest.json (Phase 2 auto-updater will consume this)
            shell: pwsh
            run: |
              @{
                version = "${{ steps.ver.outputs.version }}"
                installer_url = "https://github.com/${{ github.repository }}/releases/download/v${{ steps.ver.outputs.version }}/SquireBot-Setup-${{ steps.ver.outputs.version }}.exe"
                installer_sha256 = "${{ steps.hash.outputs.sha256 }}"
                released_at = (Get-Date).ToUniversalTime().ToString("o")
              } | ConvertTo-Json | Out-File dist/latest.json -Encoding utf8

          - uses: actions/upload-artifact@v4
            with:
              name: squirebot-installer
              path: |
                dist/SquireBot-Setup-*.exe
                dist/latest.json

          - name: Create GitHub Release
            if: startsWith(github.ref, 'refs/tags/v')
            uses: softprops/action-gh-release@v2
            with:
              files: |
                dist/SquireBot-Setup-*.exe
                dist/latest.json
              body: |
                SquireBot ${{ steps.ver.outputs.version }} — Phase 1 unsigned release.
                See README for SmartScreen "More info → Run anyway" walkthrough.
    ```

    Create `.planning/phases/01-end-to-end-thin-slice/smoke-checklist.md` — a numbered checklist
    that maps DIRECTLY to the 5 ROADMAP Phase 1 success criteria. The developer fills this in
    during Task 3.

    ```markdown
    # Phase 1 Smoke Checklist

    Used by Plan 08 Task 3 (the human-action smoke checkpoint). Record the date, the VM build,
    and the outcome of every step in this file. The completed checklist is the artefact attached
    to 01-08-SUMMARY.md.

    **Smoke run date:** ___________
    **Win11 VM build:** ___________
    **Squirebot version (-X main.Version):** ___________
    **OAuth project (gcp_project_number):** ___________
    **Workbook spreadsheetId:** ___________
    **Test character name:** ___________

    ## Success Criterion 1 — Clean install, no UAC
    - [ ] Installer .exe downloaded from GitHub Releases (or copied to clean VM)
    - [ ] SmartScreen wall appears; "More info → Run anyway" succeeds in &lt;30s
    - [ ] NSIS wizard opens with NO UAC prompt
    - [ ] Files installed to `%LOCALAPPDATA%\Programs\SquireBot\`
    - [ ] Tray icon appears
    - [ ] Browser auto-opens to `http://127.0.0.1:&lt;port&gt;/start`

    ## Success Criterion 2 — Single OAuth + single Picker, Production state
    - [ ] /start → Connect Google → consent screen does NOT show "This app isn't verified"
    - [ ] After consent, browser redirects to /picker (still in same tab)
    - [ ] Picker shows; user clicks the SquireBot guild workbook; picker closes
    - [ ] Browser advances to /eq-folder → /done
    - [ ] No second `os.Exec` browser launch observed

    ## Success Criterion 3 — Inventory upload visible within 30 seconds
    - [ ] Drop a `&lt;Char&gt;-Inventory.txt` (TSV, 5 cols) into the configured EQ folder
    - [ ] Within 30 seconds, an `inv:&lt;Char&gt;` tab appears in the workbook
    - [ ] The tab has 5 columns + _uploaded_at: header row + N data rows
    - [ ] `_char_owner` has a new row `(<Char>, &lt;email&gt;, "", "", &lt;ISO timestamp&gt;)`

    ## Success Criterion 4 — Refresh token in wincred only, NOT in config.json
    - [ ] `cmdkey /list | findstr SquireBot` shows `Target: SquireBot:&lt;email&gt;`
    - [ ] `Get-Content $env:LOCALAPPDATA\SquireBot\config.json` shows google_email but
          NO refresh_token / access_token / client_secret strings
    - [ ] `findstr /i "refresh_token" %LOCALAPPDATA%\SquireBot\config.json` returns 0 matches

    ## Success Criterion 5 — 10-day-later token survival (SCHEDULED)
    - [ ] Install date recorded: ___________
    - [ ] Day-10 follow-up scheduled in calendar / STATE.md TODO
    - [ ] On day 10+: re-launch squirebot.exe, drop a fresh inventory file, confirm upload
          succeeds without re-OAuth prompt — proves AUTH-03 Production publish escaped Testing-mode 7-day expiry

    ## Outcomes
    - [ ] All criteria PASS → Phase 1 complete; transition to /gsd-transition + Phase 2
    - [ ] Some criteria FAIL → log specific failure in 01-08-SUMMARY.md → open blocker in STATE.md → revise affected plan and re-execute
    ```
  </action>
  <verify>
    <automated>test -s .github/workflows/release.yml &amp;&amp; grep -q "windows-latest" .github/workflows/release.yml &amp;&amp; grep -q "makensis" .github/workflows/release.yml &amp;&amp; grep -q "consent_screen_status" .github/workflows/release.yml &amp;&amp; grep -q "PRODUCTION" .github/workflows/release.yml &amp;&amp; grep -q "main.OAuthClientID" .github/workflows/release.yml &amp;&amp; grep -q "latest.json" .github/workflows/release.yml &amp;&amp; grep -q "SHA256\|Get-FileHash" .github/workflows/release.yml &amp;&amp; test -s .planning/phases/01-end-to-end-thin-slice/smoke-checklist.md &amp;&amp; grep -q "Success Criterion 1" .planning/phases/01-end-to-end-thin-slice/smoke-checklist.md &amp;&amp; grep -q "Success Criterion 5" .planning/phases/01-end-to-end-thin-slice/smoke-checklist.md</automated>
  </verify>
  <acceptance_criteria>
    - `.github/workflows/release.yml` exists and runs on tag push (`refs/tags/v*`)
    - Workflow uses `windows-latest` runner
    - Workflow installs NSIS and verifies version ≥ 3.10
    - Workflow reads oauth-config.json and FAILS the build if `consent_screen_status != "PRODUCTION"` (AUTH-03 enforcement at CI)
    - Workflow builds squirebot.exe with all four `-X main.X=Y` ldflags (OAuthClientID, PickerAPIKey, GCPProjectNumber, Version)
    - Workflow runs makensis with `-DAPPVERSION=`
    - Workflow computes SHA-256 of the installer and writes a `latest.json` (Phase 2 auto-updater consumes this)
    - Workflow uploads artifacts; on tag push, creates a GitHub Release
    - `.planning/phases/01-end-to-end-thin-slice/smoke-checklist.md` exists with all 5 ROADMAP success criteria as numbered checklists
    - smoke-checklist.md includes both placeholder fields (date, version, etc.) AND actionable verify steps
  </acceptance_criteria>
  <done>
    GitHub Actions release workflow is wired and gates on AUTH-03 (consent_screen_status ==
    PRODUCTION). Smoke checklist scaffold is ready for Task 3 to fill in.
  </done>
</task>

<task type="checkpoint:human-verify" gate="blocking">
  <name>Task 3: Smoke test on a clean Windows 11 VM (Phase 1 success criteria 1-4)</name>
  <what-built>
    Tasks 1 and 2 produced (a) `installer/squirebot.nsi` (the per-user NSIS script), (b)
    `.github/workflows/release.yml` (the automated build pipeline that loads oauth-config.json
    and fails on non-Production consent), (c) `docs/build-and-install.md` (the local build
    runbook), and (d) `.planning/phases/01-end-to-end-thin-slice/smoke-checklist.md` (the 5-item
    checklist mapping ROADMAP success criteria).

    Plans 01-07 produce the binary; Plan 08 wraps and validates. The only thing left is to run
    the smoke checklist on a clean Windows 11 VM and record the outcome.
  </what-built>
  <how-to-verify>
    1. **Provision a clean Windows 11 VM** (Hyper-V, VMware, VirtualBox, Parallels, or a
       reformatted spare laptop — anything that has NEVER seen squirebot.exe). Boot, sign in
       as a non-admin user (this matters for INST-01 — admin users see no UAC prompts even
       for things that would normally trigger them).

    2. **Install P99 EverQuest into the VM** if not already present. Or — to skip this — use
       the SquireBot heuristic-scan layer's reject path: write an `eqgame.exe` (any non-empty
       file) and `eqclient.ini` into a folder on the VM and point the wizard there manually
       in step 4. The smoke test cares about WATCH-01 firing on a `*-Inventory.txt` file, NOT
       about EQ being present.

    3. **Build SquireBot locally and copy installer to VM** (or trigger a GitHub Actions release
       if the repo is public):
       ```bash
       # On dev machine, after all Plans 01-07 land:
       cd /path/to/squirebot
       OAUTH_CLIENT_ID=$(jq -r '.oauth_client_id' .planning/phases/01-end-to-end-thin-slice/oauth-config.json)
       PICKER_API_KEY=$(jq -r '.picker_api_key' .planning/phases/01-end-to-end-thin-slice/oauth-config.json)
       GCP_PROJECT_NUMBER=$(jq -r '.gcp_project_number' .planning/phases/01-end-to-end-thin-slice/oauth-config.json)
       VERSION="0.1.0"

       GOOS=windows GOARCH=amd64 go build \
         -ldflags="-H=windowsgui -s -w \
                   -X main.OAuthClientID=$OAUTH_CLIENT_ID \
                   -X main.PickerAPIKey=$PICKER_API_KEY \
                   -X main.GCPProjectNumber=$GCP_PROJECT_NUMBER \
                   -X main.Version=$VERSION" \
         -o dist/squirebot.exe ./cmd/squirebot

       cd installer
       makensis -DAPPVERSION=$VERSION -V2 squirebot.nsi
       # Resulting file: dist/SquireBot-Setup-0.1.0.exe — copy this to the VM
       ```

    4. **On the VM, run smoke-checklist.md step by step**:
       - Open `.planning/phases/01-end-to-end-thin-slice/smoke-checklist.md`
       - Fill the "Smoke run date / VM build / version / OAuth project / workbook ID / test character"
         header fields
       - For each of Success Criteria 1-4, check the boxes ONLY when actually observed
       - When a step fails, write what failed in a "Failure detail" line under that step
       - Specifically:
         - **SC1:** double-click installer; expect SmartScreen → "More info → Run anyway" → NSIS
           wizard with NO UAC prompt → installs to `%LOCALAPPDATA%\Programs\SquireBot\` → tray
           icon appears → browser auto-opens
         - **SC2:** complete OAuth on the consent screen — verify it does NOT show "This app
           isn't verified" (validates Plan 02). After consent, picker appears in same tab.
           Pick the workbook. Wizard advances to /eq-folder (auto-discovered or manual pick),
           then /done.
         - **SC3:** copy a sample `<TestChar>-Inventory.txt` (use the Plan 04
           testdata/sample-inventory.txt as a starting point) into the picked EQ folder.
           Within 30 seconds: open the workbook in a browser, verify `inv:<TestChar>` tab
           exists with header + N data rows. Verify `_char_owner` has the row
           `(<TestChar>, <email>, "", "", <ISO timestamp>)`.
         - **SC4:** open Windows "Credential Manager" (Web Credentials → Generic Credentials,
           or run `cmdkey /list` in an admin cmd) — verify entry under target name
           `SquireBot:<email>`. Then open `%LOCALAPPDATA%\SquireBot\config.json` in Notepad —
           verify it has `google_email` but NO `refresh_token` / `access_token` / `client_secret`.

    5. **Record the outcome in 01-08-SUMMARY.md** (Plan 08's output file). For each criterion,
       record either "PASS" or "FAIL: &lt;detail&gt;". Attach the filled-in smoke-checklist.md.

    6. **Schedule the day-10 follow-up checkpoint (Success Criterion 5):**
       - Add a TODO to STATE.md → Open TODOs section: `[Phase 1 day-10 follow-up] revisit
         smoke-checklist.md SC5 on or after &lt;install_date + 10 days&gt; — verify watcher still
         writes successfully without re-OAuth (proves AUTH-03 Production escaped 7-day Testing
         expiry).`
       - Commit the install date in 01-08-SUMMARY.md so the date can be reconstructed later.

    7. **DO NOT mark Phase 1 complete in ROADMAP.md / STATE.md until SC1-4 PASS.** SC5 is
       inherently asynchronous (10 days later); the phase IS allowed to proceed to Phase 2 with
       SC5 marked "scheduled" — Phase 2's planning explicitly depends on Plan 02's Production
       publish having succeeded, so SC5 will validate retroactively. If SC5 ever fails, treat
       it as a Phase 1 regression and escalate.

    Failure modes and where to look:
    - **UAC prompt fires** → squirebot.nsi has the wrong RequestExecutionLevel or the installer
      filename triggered the heuristic; revise Task 1
    - **SmartScreen blocks "Run anyway"** → unsigned binary edge case; verify D-13 walkthrough
      still works on this Win11 build; if blocked completely, document as "Phase 1 dev-machine-only
      validation; full guild rollout requires Phase 2 code-signing"
    - **Browser doesn't open** → check Plan 03's OpenBrowser launcher; `rundll32 url.dll,...`
      might be blocked by a hardening profile
    - **inv:&lt;Char&gt; tab doesn't appear** → check Plan 05's WriteInventory; check log file at
      `%LOCALAPPDATA%\SquireBot\squirebot.log` for the slog.Error from the failed write
    - **wincred entry missing** → Plan 03's StoreToken did not run; check the OAuth callback
      handler executed and StoreToken completed without error
    - **config.json contains refresh_token** → REGRESSION on Plan 01 / Plan 03 acceptance criteria;
      this MUST NOT ship; revise immediately
  </how-to-verify>
  <resume-signal>
    Type **"smoke pass"** if all 4 success criteria 1-4 are checked off and SC5 is scheduled.
    The phase is then complete pending the asynchronous 10-day follow-up.

    Type **"smoke fail: &lt;criterion N&gt; - &lt;detail&gt;"** if any of SC1-4 failed. Examples:
    `smoke fail: SC1 - UAC fired`, `smoke fail: SC4 - config.json had refresh_token`. Do NOT
    proceed to /gsd-transition with failing criteria.

    Type **"deferred: I'll do the smoke run later"** if you cannot do the VM run right now.
    Phase 1 is then "code complete pending smoke" — record state in 01-08-SUMMARY.md and STATE.md.
  </resume-signal>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Developer machine ↔ clean VM | The smoke test environment must be untainted by prior installs |
| oauth-config.json ↔ CI environment | Build-time constants flow into the binary; non-Production state breaks the phase |
| Unsigned installer ↔ SmartScreen | D-13 accepted risk; user must click through the warning |
| HKCU registry ↔ uninstall flow | Per-user; safe even on shared-PC scenarios |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-08-01 | Tampering | Compromised binary distribution (no code-signing) | accept | Phase 1 explicitly ships unsigned per D-13; release workflow records SHA-256 in latest.json so users can verify out-of-band; Phase 2 adds code signing |
| T-08-02 | Spoofing | Installer filename triggers NSIS auto-elevate heuristic ("setup", "install", "update") | mitigate | RESEARCH.md §6.2 Pitfall #7: `RequestExecutionLevel user` explicitly overrides the heuristic; acceptance grep enforces; smoke checklist SC1 verifies no UAC fires |
| T-08-03 | Tampering | CI builds with stale oauth-config.json containing TODO sentinels | mitigate | Workflow's "Load OAuth constants" step parses consent_screen_status and fails build if not "PRODUCTION"; AUTH-03 enforcement at CI layer |
| T-08-04 | Information Disclosure | latest.json or release artifacts leak client_secret | mitigate | oauth-config.json schema explicitly omits client_secret per Plan 02; CI never reads any secret material; build flow is secret-free for Phase 1 |
| T-08-05 | Privilege Escalation | Installer writes to HKLM or PROGRAMFILES (would need UAC) | mitigate | Acceptance grep enforces "no HKLM, no PROGRAMFILES" in squirebot.nsi; Section "Install" only writes HKCU and $LOCALAPPDATA |
| T-08-06 | Tampering | Uninstaller leaves wincred token behind, allowing re-install to bypass new OAuth | accept | DPAPI tokens survive uninstall by design; documented in Task 1 build-and-install.md; users wanting full wipe run `cmdkey /delete:SquireBot:&lt;email&gt;` manually |
| T-08-07 | Denial of Service | Smoke test runs on a tainted VM (e.g., previous install state) → results invalid | mitigate | Task 3 step 1 explicitly demands a clean VM; SUMMARY records VM build for reproducibility |
| T-08-08 | Repudiation | No record of who ran the smoke test or when | mitigate | smoke-checklist.md header fields capture date, VM build, tester; signed (typed name) into 01-08-SUMMARY.md |
| T-08-09 | Tampering | 10-day follow-up never executed → SC5 silently fails → guildies hit invalid_grant on day 8+ in Phase 5 rollout | mitigate | Task 3 step 6 forces a STATE.md TODO entry; Phase 2 planning's first step should re-validate; explicit "DO NOT proceed past SC5" gate documented |
</threat_model>

<verification>
- `installer/squirebot.nsi` passes acceptance grep (Task 1 verify)
- `.github/workflows/release.yml` passes acceptance grep (Task 2 verify)
- `docs/build-and-install.md` exists with all required sections
- `.planning/phases/01-end-to-end-thin-slice/smoke-checklist.md` exists with all 5 success criteria
- Task 3 checkpoint produces a filled-in smoke-checklist.md with SC1-4 PASS and SC5 SCHEDULED
- `01-08-SUMMARY.md` records the smoke outcome and the SC5 follow-up date
- STATE.md has a new TODO entry for the SC5 day-10 check
</verification>

<success_criteria>
- INST-01 satisfied (final): a guildie can install SquireBot by running a single `.exe` from a download link, with no UAC prompt and no command-line steps. Smoke checklist SC1 verifies on real Win11 VM.
- ROADMAP Phase 1 success criteria 1-4 PASS on a clean VM (criterion 5 scheduled for day-10 follow-up)
- D-12 satisfied: GitHub Releases canonical install URL is `https://github.com/&lt;owner&gt;/squirebot/releases/download/v&lt;version&gt;/SquireBot-Setup-&lt;version&gt;.exe`
- D-13 honored: Phase 1 ships unsigned; SmartScreen walkthrough is documented and verified
- AUTH-03 enforced at CI: workflow refuses to build if consent_screen_status is not PRODUCTION
- 10-day follow-up scheduled in STATE.md
</success_criteria>

<output>
After completion, create `.planning/phases/01-end-to-end-thin-slice/01-08-SUMMARY.md` documenting:
- The exact installer filename and SHA-256 produced (for reproducibility)
- The clean VM build used for smoke testing (Win11 build number, last patch date)
- A copy of the filled-in smoke-checklist.md inline in the SUMMARY
- The actual outcome of each of SC1-4 (PASS / FAIL with detail)
- The recorded install date for SC5 follow-up + the calendar date the day-10 check should run
- Any deviations from RESEARCH.md §6 and why (e.g., chose `assets/icon.ico` over `installer/icon.ico`)
- Open issues found during smoke testing that did NOT block PASS but should be tracked (Phase 2 polish or Phase 5 onboarding work)
</output>
