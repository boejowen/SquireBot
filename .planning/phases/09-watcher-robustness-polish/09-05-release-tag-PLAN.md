---
phase: 09-watcher-robustness-polish
plan: 05
plan_id: 09-05-release-tag
type: execute
wave: 3
depends_on:
  - 09-01-tray-prereadyqueue
  - 09-02-freeconsole-foreground-detach
  - 09-03-config-bom-strip
  - 09-04-auth07-boot-invalidgrant
files_modified: []
autonomous: false
requirements: [AUTH-07, OPS-06, OPS-07, CONFIG-01]
tags: [release, ci, tag, latest-json, github-release, ship-gate, wave3]

must_haves:
  truths:
    - "A new git tag `v1.0.2` exists on origin/master at a commit that contains all four prior Phase 9 plan's changes (Plans 09-01..09-04)."
    - "The GitHub Release for `v1.0.2` exists at https://github.com/boejowen/SquireBot/releases/tag/v1.0.2 with three attached artifacts: `SquireBot-Setup-1.0.2.exe`, `squirebot.exe` (bare binary), `latest.json`."
    - "The `latest.json` manifest contains `\"version\": \"1.0.2\"` and fresh sha256 hashes for both the installer and the bare binary."
    - "The release workflow's AUTH-03 PRODUCTION consent_screen gate (release.yml lines 136-139, unchanged since v1.0.1) passes — i.e., the build did NOT regress to Testing-mode OAuth."
    - "Existing v1.0.1 watchers running on guildie machines self-update to v1.0.2 via the auto-update path (`minio/selfupdate` reading the updated `latest.json`) on their next periodic check or via manual `Check for updates` tray menu. (Smoke evidence captured during this plan's UAT step.)"
    - "Manual UAT confirms all 4 fixes (AUTH-07, OPS-06, OPS-07, CONFIG-01) work end-to-end on a fresh v1.0.2 install on a Win11 VM, per the multi-fix UAT script in PATTERNS.md §'Plan 09-05 → Differs from analog'."
  artifacts:
    - path: (git tag)
      provides: "v1.0.2 tag on master at HEAD containing Plans 09-01..09-04 merges"
    - path: (GitHub Release)
      provides: "SquireBot-Setup-1.0.2.exe, squirebot.exe, latest.json"
    - path: (latest.json contents)
      provides: "version=1.0.2, fresh installer_sha256 and binary_sha256, fresh download URLs pointing at the v1.0.2 release assets"
  key_links:
    - from: "git tag v1.0.2"
      to: ".github/workflows/release.yml (tag-push trigger)"
      via: "on.push.tags: ['v*']"
      pattern: "v\\d+\\.\\d+\\.\\d+"
    - from: "release.yml"
      to: "GitHub Releases page (artifacts + body)"
      via: "softprops/action-gh-release@v3"
      pattern: "github\\.com/boejowen/SquireBot/releases"
    - from: "v1.0.2 latest.json"
      to: "v1.0.1 watcher auto-update path (minio/selfupdate)"
      via: "OPS-04 periodic check + Apply()"
      pattern: "minio/selfupdate"
---

<objective>
Ship the v1.0.2 watcher binary release that closes Phase 9 and the v1.0.2 Robustness Polish milestone. This plan is the ship gate per ROADMAP §57 success criterion 5: "Watcher binary v1.0.2 is built, tagged, and published on GitHub Releases; the `latest.json` manifest is updated so existing v1.0.1 watchers auto-update to it cleanly."

Per CONTEXT.md D-08 the release tag IS the ship gate. The release pipeline at `.github/workflows/release.yml` is already configured for tag-driven releases (`on.push.tags: ['v*']`) and will, on `git push origin v1.0.2`:
1. Verify the AUTH-03 PRODUCTION consent_screen gate (release.yml lines 128-147, unchanged since v1.0.1).
2. Build `dist/squirebot.exe` with v1.0.2 ldflags injecting `main.Version=1.0.2` (release.yml lines 149-168).
3. Run `makensis` against `installer/squirebot.nsi` (release.yml lines 170-178) — picks up the v1.0.1 NSIS pre-install shim automatically (unchanged).
4. Compute SHA-256 for installer + bare binary (release.yml lines 230-241).
5. Write `dist/latest.json` with the new version + fresh hashes + correct URLs (release.yml lines 243-266).
6. Create the GitHub Release with all three artifacts attached (release.yml lines 278-317).

**No edits to `release.yml` are required.** CONTEXT.md D-08 explicitly says "Reuse `release.yml` GitHub Action unchanged where possible". Phase 9 changes are pure Go source (no new NSIS plugins, no new build steps). The CI workflow handles the v1.0.2 tag exactly the same way it handled v1.0.1.

**No source-side `Version` constant bump required.** Per PATTERNS.md analysis of `cmd/squirebot/build_constants.go:29` (`Version = "0.1.0-dev"`), the dev-default constant is overridden at release time by the ldflag `-X main.Version=${{ steps.ver.outputs.version }}`. Phase 6's v1.0.1 release did not bump the source constant; mirror that.

This plan is NON-AUTONOMOUS because:
- The tag-push triggers GitHub Actions on the user's account; the user must be present to authenticate the push and to verify the CI run.
- The post-tag verification (GitHub Release page exists, artifacts attached, latest.json correct) requires the user's GitHub access.
- The end-user UAT (v1.0.1 → v1.0.2 upgrade on a clean Win11 VM, plus exercising all 4 fixes end-to-end) is the canonical proof of ROADMAP §52-57 — the user owns the VM and the install.

Output: tag `v1.0.2` pushed to origin; CI green; GitHub Release populated; user has confirmed all 4 fixes end-to-end via UAT (or scheduled a routine to verify).

**Scope discipline (per CONTEXT.md domain section + D-08):**
- No edits to `release.yml`. If the build fails, fix the underlying code first; only edit CI as a last resort and flag it as a scope expansion.
- No new milestone closure work in this plan. Milestone close (`.planning/milestones/v1.0.2-ROADMAP.md` archive, REQUIREMENTS.md retirement, MILESTONES.md append) happens via `/gsd-complete-milestone` AFTER this plan ships AND Phase 10 ships.
- No schema changes (D-06): `WatcherMaxSchemaVersion = 3` MUST remain at 3 in the released binary; verifier grep gate is part of Task 1.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/STATE.md
@.planning/phases/09-watcher-robustness-polish/09-CONTEXT.md
@.planning/phases/09-watcher-robustness-polish/09-PATTERNS.md
@.planning/phases/09-watcher-robustness-polish/09-01-SUMMARY.md
@.planning/phases/09-watcher-robustness-polish/09-02-SUMMARY.md
@.planning/phases/09-watcher-robustness-polish/09-03-SUMMARY.md
@.planning/phases/09-watcher-robustness-polish/09-04-SUMMARY.md
@.planning/phases/06-installer-overwrite-running-shim/06-05-release-tag-PLAN.md
@.github/workflows/release.yml
@CLAUDE.md
</context>

<tasks>

<task type="auto">
  <name>Task 1: Pre-tag readiness sweep — verify clean working tree, all 4 fixes present, schema unchanged, full test suite green</name>
  <files></files>
  <read_first>
    - .planning/phases/09-watcher-robustness-polish/09-01-SUMMARY.md
    - .planning/phases/09-watcher-robustness-polish/09-02-SUMMARY.md
    - .planning/phases/09-watcher-robustness-polish/09-03-SUMMARY.md
    - .planning/phases/09-watcher-robustness-polish/09-04-SUMMARY.md
    - .planning/phases/06-installer-overwrite-running-shim/06-05-release-tag-PLAN.md §Task 1 (the canonical pre-tag readiness sweep)
    - .github/workflows/release.yml (full file — confirm the `on.push.tags: ['v*']` trigger, the ldflag injection at `-X main.Version=...`, and the `latest.json` generation step are intact; flag any regression vs. the v1.0.1 ship)
    - CLAUDE.md (review schema-evolution and slog rules — these must be respected by the merged-in code)
  </read_first>
  <action>
    Run each of the following bash/PowerShell checks, in order. STOP and surface any failure before proceeding to Task 2.

    **A. Working-tree and branch state:**
    ```
    git status
    git branch --show-current
    git log --oneline -25
    ```
    Expect: `git status` reports clean working tree, current branch is `master` (or whatever the user confirms is the canonical merge target), recent commits include 4 distinct Phase 9 fix-plan commits (09-01, 09-02, 09-03, 09-04 SUMMARY+source). If any work is uncommitted, stop and surface.

    **B. Full Go build + vet + test:**
    ```
    go build ./...
    go vet ./...
    go test ./... -count=1
    ```
    All three MUST exit 0. If any test fails, the failure mode is more important than the release — stop and surface.

    **C. Schema-impact assertion (CONTEXT.md D-06 verifier grep gate):**
    ```
    Select-String -Path internal/sheet/client.go -Pattern 'WatcherMaxSchemaVersion\s*=\s*3'
    Select-String -Path apps-script/src/lib/migrations.ts -Pattern 'SCRIPT_MIN_SCHEMA_VERSION.*3'
    ```
    Both MUST match exactly 1 line each. Any deviation is a phase-scope violation; stop and surface.

    **D. Phase 9 fix-presence grep gates (each fix must be in the working tree):**

    - **OPS-06 (Plan 09-01):**
      ```
      Select-String -Path internal/tray/tray.go -Pattern 'type pendingAction struct'
      Select-String -Path internal/tray/tray.go -Pattern 'func \(t \*Controller\) drainPending\(\)'
      Select-String -Path internal/tray/tray.go -Pattern 't\.ready = true'
      ```
      First two MUST each match 1 line. Third MUST match 2 lines (OnReady + simulateReady).

    - **OPS-07 (Plan 09-02):**
      ```
      Test-Path cmd/squirebot/console_windows.go
      Test-Path cmd/squirebot/console_other.go
      Select-String -Path cmd/squirebot/console_windows.go -Pattern 'windows\.FreeConsole\(\)'
      Select-String -Path cmd/squirebot/main.go -Pattern '_ = freeConsole\(\)'
      ```
      Both Test-Path calls MUST return True. Each Select-String MUST match exactly 1 line.

    - **CONFIG-01 (Plan 09-03):**
      ```
      Select-String -Path internal/config/config.go -Pattern 'bytes\.TrimPrefix\(data, \[\]byte\{0xEF, 0xBB, 0xBF\}\)'
      Select-String -Path internal/config/config_test.go -Pattern 'TestLoad_StripsUTF8BOM'
      ```
      Each MUST match exactly 1 line.

    - **AUTH-07 (Plan 09-04):**
      ```
      Select-String -Path internal/app/runapp.go -Pattern 'func applyBootAuthError\(t \*tray\.Controller, err error\) bootAuthClassification'
      Select-String -Path internal/app/runapp.go -Pattern 'Reauthorize: refresh token died\. Click Reauthorize…'
      Select-String -Path internal/app/runapp_test.go -Pattern 'TestApplyBootAuthError_Revoked'
      ```
      First MUST match exactly 1 line. Second MUST match 2 lines (suspendForAuth + applyBootAuthError verbatim parity). Third MUST match exactly 1 line.

    **E. Release-workflow integrity (no edits expected, but verify):**
    ```
    Select-String -Path .github/workflows/release.yml -Pattern "tags:\s*\['v\*'\]"
    Select-String -Path .github/workflows/release.yml -Pattern '-X main\.Version='
    Select-String -Path .github/workflows/release.yml -Pattern 'latest\.json'
    ```
    Each MUST match at least 1 line. If any are missing, the v1.0.2 release will not build correctly; stop and surface (this would be a CI regression introduced since v1.0.1).

    **F. v1.0.1 baseline confirmation:**
    ```
    git tag -l 'v1.0.*' | Sort-Object
    ```
    Expect: `v1.0.0` and `v1.0.1` both present (from v1.0 and v1.0.1 ship gates respectively). If `v1.0.2` is ALREADY present, the tag has already been pushed — stop and surface; do not double-tag.

    Capture the results of all checks A-F in a brief readiness note that gets included in this plan's SUMMARY.md.
  </action>
  <verify>
    <automated>go build ./... && go vet ./... && go test ./... -count=1</automated>
  </verify>
  <acceptance_criteria>
    - `git status --short` returns no lines (clean working tree).
    - `git branch --show-current` returns `master` (or the user-confirmed merge target).
    - `go build ./...` exits 0.
    - `go vet ./...` exits 0.
    - `go test ./... -count=1` exits 0.
    - `Select-String -Path internal/sheet/client.go -Pattern 'WatcherMaxSchemaVersion\s*=\s*3'` returns 1 match.
    - `Select-String -Path apps-script/src/lib/migrations.ts -Pattern 'SCRIPT_MIN_SCHEMA_VERSION.*3'` returns ≥ 1 match.
    - All four fix-presence grep gates (D above) pass with exact match counts noted.
    - All three release.yml integrity greps (E above) return ≥ 1 match each.
    - `v1.0.2` is NOT yet present in `git tag -l 'v1.0.*'` output.
    - Readiness note drafted for inclusion in 09-05-SUMMARY.md.
  </acceptance_criteria>
  <done>All pre-tag readiness checks pass; readiness note drafted; ready to push the v1.0.2 tag in Task 2.</done>
</task>

<task type="checkpoint:human-action" gate="blocking">
  <name>Task 2: Push v1.0.2 tag, monitor CI workflow, verify GitHub Release artifacts + latest.json</name>
  <what-built>
    Task 1 (just completed) verified that all 4 Phase 9 fixes (Plans 09-01..09-04) are present in the working tree, the schema constants are unchanged at version 3, `go build` / `go vet` / `go test` all pass, and the existing `.github/workflows/release.yml` workflow (last used for v1.0.1) is structurally intact. The pre-tag checks created a readiness note for the SUMMARY.

    Nothing has been pushed yet. The tag push triggers the CI pipeline.
  </what-built>
  <how-to-verify>
    You (the user) need to execute the tag-push + verify steps below. This requires your GitHub authentication and your eyes on the CI run.

    **1. Create and push the v1.0.2 tag:**

    ```
    git tag -a v1.0.2 -m "Phase 9 (AUTH-07/OPS-06/OPS-07/CONFIG-01): watcher robustness polish. Watcher binary v1.0.2 release."
    git push origin v1.0.2
    ```

    **2. Monitor the workflow run (in a separate terminal):**

    ```
    gh run watch --exit-status
    ```

    If you don't have `gh` available, watch the run at:
    `https://github.com/boejowen/SquireBot/actions`

    **3. After the run completes successfully, verify the GitHub Release:**

    ```
    gh release view v1.0.2
    ```

    Expect the output to list:
    - Release title containing `v1.0.2` (or `1.0.2`)
    - Three attached assets: `SquireBot-Setup-1.0.2.exe`, `squirebot.exe`, `latest.json`
    - Release body containing changelog text (auto-generated by softprops/action-gh-release@v3)

    **4. Verify the latest.json contents (CRITICAL — this is what triggers auto-update for existing v1.0.1 watchers):**

    ```
    gh release download v1.0.2 -p latest.json -O ./_tmp-latest.json
    cat ./_tmp-latest.json
    ```

    Expect a JSON object with:
    - `"version": "1.0.2"` (NOT "1.0.1" — would be a CI bug)
    - `"installer_sha256"`: fresh 64-char hex hash, different from the v1.0.1 hash
    - `"binary_sha256"`: fresh 64-char hex hash
    - URLs pointing at `github.com/boejowen/SquireBot/releases/download/v1.0.2/...`

    Delete `./_tmp-latest.json` after inspecting:
    ```
    Remove-Item ./_tmp-latest.json
    ```

    **5. Verify the AUTH-03 PRODUCTION consent_screen gate passed:**

    In the workflow run output, find the step named something like "Verify AUTH-03 PRODUCTION consent screen" (release.yml lines 128-147). Confirm it printed PASS / did not fail the run. This is the load-bearing OAuth integrity check carried forward from v1.0.

    **6. If anything is missing or wrong:**

    - If the workflow fails: read the failed step's logs. Common failure modes from prior releases:
      - NSIS plugin missing → CI choco install regression
      - OAuth client ID env var unset → user needs to verify GitHub secrets
      - SHA-256 step mismatch → indicates a build artifact integrity issue

    - If the release page is missing assets: the `softprops/action-gh-release@v3` step may have errored silently — check the logs.

    - If `latest.json` has wrong version: indicates the ldflag injection failed; do NOT proceed to UAT until fixed.

    **7. Report back with:**
    - "Tag pushed, workflow green, release populated with 3 assets, latest.json shows version 1.0.2 with fresh hashes" (success path), OR
    - "Workflow failed at step X with error Y" (failure path — we stop and fix before re-tagging).
  </how-to-verify>
  <resume-signal>Type "release published" once the GitHub Release for v1.0.2 is live with all three artifacts and the latest.json version field reads "1.0.2". Or describe the failure mode and stop.</resume-signal>
</task>

<task type="checkpoint:human-verify" gate="blocking">
  <name>Task 3: End-to-end UAT — exercise all 4 Phase 9 fixes on a fresh v1.0.1 → v1.0.2 install</name>
  <what-built>
    Task 2 published the v1.0.2 GitHub Release. The `latest.json` manifest now advertises v1.0.2 with fresh hashes; existing v1.0.1 watchers will auto-update on their next periodic check.

    Task 3 verifies that all 4 Phase 9 fixes work end-to-end on a real Win11 install — both via the auto-update path (v1.0.1 → v1.0.2) and via a fresh install. This is the canonical UAT for ROADMAP §52-57 success criteria.
  </what-built>
  <how-to-verify>
    You (the user) need a Win11 VM or a Win11 host machine where you have admin (or installer) access. The UAT walks through all 4 fixes plus the auto-update path. Capture results in a brief UAT note for inclusion in 09-05-SUMMARY.md.

    **Pre-stage (one-time):**

    - Have either a clean Win11 VM with NO SquireBot, OR a Win11 machine currently running v1.0.1.
    - You'll need access to your Google Account settings to deliberately revoke the SquireBot OAuth grant (for AUTH-07 reproduction).
    - Tools: Notepad, PowerShell, the SquireBot installer.

    ---

    **UAT 1 — AUTH-07 + OPS-06 (boot-time invalid_grant → red icon + Reauthorize):**

    1. On the test machine, ensure SquireBot v1.0.1 (or fresh v1.0.2) is installed and has successfully completed the wizard at least once (so a refresh token exists in wincred).
    2. Quit SquireBot from the tray menu.
    3. Open https://myaccount.google.com/permissions in a browser, find "SquireBot" in the list of third-party apps with access, and click "Remove Access". This invalidates the refresh token in wincred.
    4. Launch SquireBot again (from Start menu, double-click, or `& squirebot.exe` in PowerShell — any path).
    5. **Expected:** The tray icon turns RED from boot (NOT green, NOT "Initialising…" hanging), and the Reauthorize… menu item is VISIBLE in the tray menu. The status row shows: `Reauthorize: refresh token died. Click Reauthorize…`.
    6. Click Reauthorize…; complete the OAuth flow in the browser; return to SquireBot.
    7. **Expected:** Within seconds, the tray icon goes GREEN and the Reauthorize menu item is HIDDEN. Status shows a normal upload-status string.

    Capture: tray icon color from boot (red/green/initialising), status string at boot, Reauthorize visibility at boot, recovery time after click.

    ---

    **UAT 2 — Auto-update path (v1.0.1 → v1.0.2 via latest.json):**

    Skip this step if you already installed v1.0.2 directly. Run it on a machine still running v1.0.1.

    1. Confirm the currently-installed version. The Workbook tab in your Sheet (or the tray menu's "About" if present, or the log file's first slog line) reports the version.
    2. Either wait for the next periodic check (OPS-04, typically daily), or click "Check for updates" in the tray menu to force an immediate check.
    3. **Expected:** Within ~60 seconds the watcher downloads `squirebot.exe.new` adjacent to the live binary (visible in `%LOCALAPPDATA%\SquireBot\` or wherever the install path is). On next launch of SquireBot (quit + re-launch, or wait for the auto-restart if implemented), the swap activates and the running version is v1.0.2.

    Capture: did the .new file land, did the swap activate on next launch, what version is running after the swap.

    ---

    **UAT 3 — CONFIG-01 (Notepad-edited config.json with BOM loads cleanly):**

    1. Quit SquireBot.
    2. Open `%LOCALAPPDATA%\SquireBot\config.json` in Notepad. Make any harmless cosmetic edit (e.g., add a trailing space inside a string field, then remove it). Save. Notepad writes a UTF-8 BOM by default on save in modern Win11 builds.
    3. (Optional cross-check: open the file in a hex-aware viewer or run `Format-Hex -Path "$env:LOCALAPPDATA\SquireBot\config.json" -Count 8` in PowerShell — the first 3 bytes should be `EF BB BF`.)
    4. Launch SquireBot.
    5. **Expected:** Watcher starts NORMALLY. No "invalid character 'ï' looking for beginning of value" error in the log file or in a foreground console.

    Capture: did the BOM-prefixed config load without error.

    ---

    **UAT 4 — OPS-07 (foreground PowerShell launch + shell close):**

    1. Quit SquireBot.
    2. Open a fresh PowerShell window.
    3. Launch SquireBot via: `& "$env:LOCALAPPDATA\SquireBot\squirebot.exe"` (or whatever the install path resolves to).
    4. **Expected:** Tray icon appears; watcher boots normally.
    5. Close the PowerShell window (just the X button or `exit`).
    6. **Expected:** Tray icon PERSISTS. SquireBot continues running.
    7. Confirm via `Get-Process squirebot` in a NEW PowerShell window — the process is still alive.
    8. Confirm via the log file (`%LOCALAPPDATA%\SquireBot\logs\` newest file) — slog lines continue being written after the PowerShell close timestamp.

    Capture: did the watcher survive the parent shell close.

    ---

    **UAT 5 — Optional regression sweep (existing functionality):**

    1. Confirm an inventory or spellbook upload still works (drop a test `.txt` into the EQ folder, watch the sheet update).
    2. Confirm the tray menu items all click correctly (Open Workbook, Open log folder, Change Workbook…, Check for updates, Quit).

    Capture: any regression vs. v1.0.1 behavior.

    ---

    **Report back with a brief table:**

    | UAT | Pass/Fail | Notes |
    |-----|-----------|-------|
    | 1 — AUTH-07 + OPS-06 boot recovery | ? | ... |
    | 2 — Auto-update path | ? | ... |
    | 3 — CONFIG-01 BOM tolerance | ? | ... |
    | 4 — OPS-07 foreground detach | ? | ... |
    | 5 — Regression sweep | ? | ... |

    If anything fails, surface it. Phase 9 cannot ship with a failing UAT case; we'll plan a hotfix.
  </how-to-verify>
  <resume-signal>Type "uat passed all five" if every case passes. Or paste the table with failure details and stop.</resume-signal>
</task>

<task type="auto">
  <name>Task 4: Update ROADMAP.md to mark Phase 9 complete; commit; do NOT close milestone</name>
  <files>.planning/ROADMAP.md</files>
  <read_first>
    - .planning/ROADMAP.md (full file — find the Phase 9 entry under "🚧 v1.0.2 — Robustness Polish (in progress)")
    - .planning/phases/06-installer-overwrite-running-shim/06-05-SUMMARY.md (analog: how Phase 6 marked completion in ROADMAP)
    - .planning/STATE.md (note for an STATE.md update post-Phase-9; not part of this plan but useful context)
  </read_first>
  <action>
    Open `.planning/ROADMAP.md`. Locate the Phase 9 entry under "### 🚧 v1.0.2 — Robustness Polish (in progress)" section. It currently looks like:

    ```
    - [ ] **Phase 9: Watcher Robustness Polish** — close 4 v1.0.1-UAT-surfaced robustness gaps ...
    ```

    Change `[ ]` to `[x]` and append a completion suffix in line with Phase 6/7/8 entry style. After the change, the entry should look something like:

    ```
    - [x] **Phase 9: Watcher Robustness Polish** — close 4 v1.0.1-UAT-surfaced robustness gaps (boot-time `invalid_grant` Reauthorize recovery, tray controller pre-Ready call queue, UTF-8 BOM strip in config loader, foreground-shell-close silent death fix); shipped + UAT-verified as tag `v1.0.2`, 2026-05-MM (replace MM with actual day). Requirements: AUTH-07, OPS-06, OPS-07, CONFIG-01.
    ```

    Also update the Progress table near the bottom of ROADMAP.md (the row for Phase 9) to reflect:
    - Status: `Shipped` (replacing `Not started`)
    - Completed: the actual date

    Do NOT modify the v1.0.2 milestone-level row in the milestone Progress table yet (it should still show `🚧 In progress` because Phase 10 hasn't shipped). The milestone ships as a unit after Phase 10.

    Do NOT delete REQUIREMENTS.md, do NOT create `.planning/milestones/v1.0.2-ROADMAP.md`, do NOT touch MILESTONES.md — those are `/gsd-complete-milestone` operations executed AFTER Phase 10 ships.

    Commit the change:
    ```
    git add .planning/ROADMAP.md
    git commit -m "docs(09): mark Phase 9 shipped as v1.0.2"
    ```

    Do NOT push the commit to remote until the user confirms — they may want to bundle it with the Phase 10 work into a single rolled-up push, or push immediately. Surface the commit hash and let them decide.
  </action>
  <verify>
    <automated>git log --oneline -3</automated>
  </verify>
  <acceptance_criteria>
    - `Select-String -Path .planning/ROADMAP.md -Pattern '\[x\] \*\*Phase 9: Watcher Robustness Polish\*\*'` returns 1 match.
    - `Select-String -Path .planning/ROADMAP.md -Pattern 'shipped \+ UAT-verified as tag .v1\.0\.2.'` returns 1 match.
    - `Select-String -Path .planning/ROADMAP.md -Pattern '🚧 In progress'` still returns at least 1 match (the v1.0.2 milestone-level row remains in-progress because Phase 10 is unshipped).
    - `Test-Path .planning/REQUIREMENTS.md` returns True (file NOT deleted).
    - `Test-Path .planning/milestones/v1.0.2-ROADMAP.md` returns False (milestone archive NOT created yet).
    - `git log --oneline -3` shows a commit with message containing `docs(09): mark Phase 9 shipped as v1.0.2`.
    - `git status --short` returns no lines after the commit (working tree clean).
  </acceptance_criteria>
  <done>Phase 9 is checked off in ROADMAP.md; v1.0.2 milestone remains in-progress; commit is local; user can decide when to push.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| local dev → GitHub origin | git push origin v1.0.2 — authenticated by user's git credentials |
| GitHub Actions → Release artifacts | release.yml produces signed (well, SHA-256-hashed) artifacts; supply-chain integrity rests on the CI's provenance |
| latest.json → existing v1.0.1 watchers | The manifest authoritatively triggers auto-update; a bogus latest.json would cause guildies to download a wrong binary |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-09-05-01 | Tampering | Tag points at a commit that does NOT contain all 4 fixes (partial-merge race) | mitigate | Task 1's grep gates (D in Task 1) verify each fix's signature string is present in the working tree BEFORE the tag is created. `git status` clean + correct branch ensures no uncommitted work is silently excluded. |
| T-09-05-02 | Tampering | latest.json mismatch (advertises wrong SHA-256, redirects users to a wrong asset) | mitigate | release.yml lines 230-241 + 243-266 compute SHA-256 IN-PIPELINE from the same artifacts published to the Release — no manual hash entry. Task 2's verification step opens the published latest.json and confirms version=1.0.2 + fresh hashes. |
| T-09-05-03 | Spoofing | Untrusted release source (someone other than the user pushes the tag) | mitigate | GitHub branch / tag protection on `master` and on `v*` tags (assumed configured at project setup) — only authenticated git pushes from the owner's credentials succeed. CI runs only on tags pushed to origin. |
| T-09-05-04 | Elevation of Privilege | OAuth Production gate regression (release.yml AUTH-03 check skipped or bypassed) | mitigate | Task 1's release-workflow integrity grep (E) confirms the AUTH-03 PRODUCTION consent_screen gate (release.yml lines 128-147) is still present and unchanged. Task 2's verification step confirms it ran green in the actual workflow execution. |
| T-09-05-05 | Denial of Service | Auto-update path to v1.0.2 breaks existing v1.0.1 guildies | mitigate | UAT 2 in Task 3 explicitly tests the v1.0.1 → v1.0.2 auto-update path on a real machine BEFORE we claim ship. If it fails, we don't claim ship; we plan a hotfix. |
| T-09-05-06 | Tampering | Schema regression (someone bumped WatcherMaxSchemaVersion) | mitigate | Task 1 step C verifies `WatcherMaxSchemaVersion = 3` and `SCRIPT_MIN_SCHEMA_VERSION = 3` both still match. Any deviation halts the release. |

**Schema impact:** NONE. The binary shipped at v1.0.2 has `WatcherMaxSchemaVersion = 3`. `_meta.schema_version` in the workbook stays at 3. No migration runs. Verifier grep gate (run in Task 1): `grep -nE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go` matches exactly 1 line.
</threat_model>

<verification>
1. `go build ./...`, `go vet ./...`, `go test ./... -count=1` all exit 0 (Task 1).
2. Schema grep gates (`WatcherMaxSchemaVersion = 3` and `SCRIPT_MIN_SCHEMA_VERSION = 3`) match exactly per file (Task 1 step C).
3. All 4 fix-presence grep gates pass (Task 1 step D — pendingAction, FreeConsole, TrimPrefix BOM, applyBootAuthError).
4. Tag `v1.0.2` exists on origin (Task 2).
5. GitHub Actions workflow run green; release page shows 3 attached artifacts (Task 2).
6. `latest.json` advertises `"version": "1.0.2"` with fresh SHA-256 hashes (Task 2).
7. AUTH-03 PRODUCTION consent_screen gate passed in the workflow run (Task 2 step 5).
8. UAT 1 (AUTH-07 + OPS-06 boot recovery) PASS on a real Win11 install (Task 3).
9. UAT 2 (auto-update v1.0.1 → v1.0.2) PASS (Task 3).
10. UAT 3 (CONFIG-01 BOM tolerance) PASS (Task 3).
11. UAT 4 (OPS-07 foreground detach) PASS (Task 3).
12. UAT 5 (no broader regression) PASS (Task 3).
13. ROADMAP.md updated; Phase 9 checked off; v1.0.2 milestone row remains in-progress (Task 4).
14. REQUIREMENTS.md, MILESTONES.md, and `.planning/milestones/v1.0.2-*` are UNTOUCHED (those are `/gsd-complete-milestone` operations, not this plan).
</verification>

<success_criteria>
- Tag `v1.0.2` is live on origin/master, pointing at a commit that contains all 4 Phase 9 fixes.
- GitHub Release v1.0.2 is published with the installer, bare binary, and latest.json artifacts.
- All four Phase 9 fixes (AUTH-07, OPS-06, OPS-07, CONFIG-01) UAT-verified end-to-end on a real Win11 install.
- Auto-update path from v1.0.1 → v1.0.2 confirmed working.
- Schema version constants unchanged at 3 in both watcher and apps-script.
- Phase 9 marked complete in ROADMAP.md; v1.0.2 milestone closure (REQUIREMENTS.md deletion, milestone archive, etc.) deferred to `/gsd-complete-milestone` after Phase 10 ships.
</success_criteria>

<output>
After completion, create `.planning/phases/09-watcher-robustness-polish/09-05-SUMMARY.md` summarizing:
- Pre-tag readiness sweep results (Task 1 grep matches + test/vet/build status).
- Tag push + CI workflow outcome (timestamp, run ID if captured, green/red status).
- GitHub Release URL + latest.json version/hashes summary.
- UAT table (5 rows from Task 3) with pass/fail + brief evidence.
- ROADMAP.md update commit hash.
- Schema constant `WatcherMaxSchemaVersion = 3` confirmed unchanged (with the grep evidence).
- Hand-off: "Phase 9 shipped as v1.0.2. Phase 10 (Apps Script Test Quality, TEST-03 + TEST-04) is the remaining work for v1.0.2 milestone close. `/gsd-complete-milestone` runs after Phase 10 ships."
</output>
