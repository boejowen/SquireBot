# Phase 1 Smoke Checklist

Used by Plan 01-08 Task 3 (the human-action smoke checkpoint). Copy this
file to the clean Win11 VM (or print it), then walk through every step
and check the boxes ONLY when actually observed. The completed checklist
is the artefact attached to `01-08-SUMMARY.md`.

If a step fails, write a `Failure detail:` line directly under that step
and continue to the next criterion -- a single failure does NOT stop the
run; we want to surface every issue Phase 1 produced.

## Run header (fill before starting)

- **Smoke run date:** `__________`
- **Tester:** `__________`
- **Win11 VM build:** `__________` (winver -> "OS Build", e.g. `26100.4351`)
- **VM clean state confirmed?** `[ ]` (no prior squirebot install ever)
- **Squirebot version (-X main.Version):** `__________`
- **OAuth project (gcp_project_number):** `262087828393`
- **Workbook spreadsheetId:** `__________`
- **Test character name:** `__________`
- **Installer SHA-256 (must match `dist/latest.json`):** `__________`

## Pre-flight (on the dev machine, BEFORE copying installer to VM)

- [ ] `dist/squirebot.exe` exists and is approximately 16-17 MB (proves
      ldflags applied; smaller binaries mean OAuthClientID etc. did not
      bake in)
- [ ] `dist/SquireBot-Setup-<version>.exe` produced by `makensis -V2`
      with zero warnings
- [ ] SHA-256 of the installer recorded above
- [ ] Installer copied to VM (USB, OneDrive, RDP clipboard, scp -- whichever)

## Success Criterion 1 -- Clean install, no UAC

- [ ] Signed in to the VM as a **non-admin** user (admin users see no
      UAC prompts even for things that would normally trigger them, so
      admin smoke runs do not validate INST-01)
- [ ] Double-click `SquireBot-Setup-<version>.exe`
- [ ] SmartScreen "Unknown publisher" wall appears (per D-13)
- [ ] Click "More info"; the "Run anyway" button is now visible
- [ ] Click "Run anyway" -- elapsed time from double-click to NSIS wizard
      under 30 s
- [ ] NSIS wizard appears with **NO additional UAC prompt**
      (this is the load-bearing check; if a UAC prompt fires here,
      INST-01 has failed)
- [ ] Click Install
- [ ] Files installed to `%LOCALAPPDATA%\Programs\SquireBot\` (verify in
      File Explorer or `Get-ChildItem $env:LOCALAPPDATA\Programs\SquireBot`).
      Expected files: `squirebot.exe`, `icon.ico`, `uninstall.exe`
- [ ] **NOT** installed under `Program Files` or `Program Files (x86)`
      (a quick PowerShell check: `Test-Path 'C:\Program Files\SquireBot'`
      should return `False`)
- [ ] HKCU Uninstall registry entry exists; verify with PowerShell:
      ```powershell
      Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\SquireBot' |
          Select-Object DisplayName, DisplayVersion, InstallLocation, UninstallString
      ```
      Expected: `DisplayName=SquireBot`, `DisplayVersion=<version>`,
      `InstallLocation=...\Programs\SquireBot`, `UninstallString=..."uninstall.exe"`
- [ ] System tray icon appears (bottom-right; may need to expand the
      hidden-icons drawer)
- [ ] Default browser opens automatically to `http://127.0.0.1:<port>/start`
      (port is in the ephemeral range 49152-65535)

Failure detail (if any):

## Success Criterion 2 -- OAuth + Picker, Production state

- [ ] `/start` page loads; click "Connect Google"
- [ ] Google consent screen appears
- [ ] Consent screen does **NOT** show the yellow "This app isn't verified"
      warning (validates Plan 02's Production publish; if this warning
      DOES appear, AUTH-03 has regressed and SC5 will fail in 7 days)
- [ ] Consent screen shows the publisher name and logo configured in
      Plan 02; only the `drive.file` scope is requested
- [ ] Click Allow; browser redirects (still in same tab) and progresses
      to `/picker`
- [ ] Drive Picker dialog renders inside the page
- [ ] Picker shows the SquireBot guild workbook (it must have been shared
      with this Google account before the smoke run)
- [ ] Click the workbook; picker closes; page advances to `/eq-folder`
- [ ] EQ folder discovery either auto-finds the EQ install or prompts
      with a "We couldn't find your EverQuest folder -- pick it" button.
      Pick the folder containing `eqgame.exe` and `eqclient.ini`.
- [ ] Page advances to `/done`; "You're all set" message visible
- [ ] No second `os.Exec` browser launch observed (one wizard, one tab,
      one continuous flow)

Failure detail (if any):

## Success Criterion 3 -- Inventory upload visible within 30 seconds

- [ ] Drop a sample `<TestChar>-Inventory.txt` (TSV, 5 cols: Location,
      Name, ID, Count, Slots) into the configured EQ folder
      (use `internal/parse/testdata/sample-inventory.txt` as a starting
      point if needed)
- [ ] Within 30 seconds of the file landing, an `inv:<TestChar>` tab
      appears in the picked workbook (refresh the Sheets tab in your
      browser to observe)
- [ ] The tab has the expected header row: `Location | Name | ID |
      Count | Slots | _uploaded_at`
- [ ] N data rows below the header (where N matches the non-comment row
      count of the source file)
- [ ] `_uploaded_at` column contains a recent ISO-8601 timestamp on
      every data row
- [ ] `_char_owner` tab has a new row `(<TestChar>, <google-email>, "",
      "", <ISO timestamp>)` appended (proves the watcher's userinfo email
      lookup is wired -- AUTH-04 / WATCH-01)

Failure detail (if any):

## Success Criterion 4 -- Refresh token in wincred only, NOT in config.json

- [ ] Open Credential Manager (`Win+R` -> `control /name Microsoft.CredentialManager`),
      switch to Web Credentials, OR run in PowerShell:
      ```powershell
      cmdkey /list | Select-String SquireBot
      ```
- [ ] An entry with target name `SquireBot:<your-email>` is present
- [ ] Open `%LOCALAPPDATA%\SquireBot\config.json` in Notepad (or
      PowerShell: `Get-Content $env:LOCALAPPDATA\SquireBot\config.json`)
- [ ] `google_email` field is present and matches the OAuth account
- [ ] **NO `refresh_token` value** -- the file must not contain the
      string `refresh_token` even as a key (per AUTH-04 / Plan 03)
- [ ] **NO `access_token` value**
- [ ] **NO `client_secret` value** (desktop OAuth has no client secret;
      it must never appear)
- [ ] PowerShell sanity check returns 0 matches:
      ```powershell
      Select-String -Path $env:LOCALAPPDATA\SquireBot\config.json `
                    -Pattern 'refresh_token|access_token|client_secret'
      ```

Failure detail (if any) -- **THIS CRITERION FAILING IS A SHIP-STOPPER**:

## Success Criterion 5 -- 10-day-later token survival (SCHEDULED)

This is asynchronous and will be revisited 10+ days after the install.
Phase 1 is allowed to proceed to Phase 2 with SC5 marked "scheduled" --
but if SC5 ever fails, Phase 1 is a regression and rollout to the rest
of the guild is blocked.

- [ ] Install date recorded above in the run header
- [ ] Day-10 follow-up scheduled in calendar / STATE.md TODO with the
      exact target date `<install_date + 10 days>`
- [ ] On day 10 or later: re-launch `squirebot.exe` (don't reinstall),
      drop a fresh `<TestChar>-Inventory.txt` into the EQ folder, and
      confirm the upload succeeds without re-OAuth prompt
- [ ] If the day-10 check passes -- proves AUTH-03 Production publish
      escaped Testing-mode 7-day silent expiry
- [ ] If the day-10 check fails -- escalate as Phase 1 regression in
      STATE.md; do NOT proceed with guild rollout in Phase 5

Day-10 target date: `__________` (computed: install date + 10 days)

## Outcomes

- [ ] **All criteria PASS (SC1-4 PASS, SC5 scheduled)** -> Phase 1 is
      code-complete; transition to `/gsd-transition` and Phase 2 planning
- [ ] **Some criteria FAIL** -> log specific failures above, open
      blockers in STATE.md, revise the affected plan(s) and re-execute

## Sign-off

- **Tester (typed name):** `__________`
- **Date:** `__________`
- **Outcome:** `__________` (PASS / FAIL with criterion list)
- **Linked SUMMARY:** `.planning/phases/01-end-to-end-thin-slice/01-08-SUMMARY.md`
