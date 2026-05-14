---
phase: 06-installer-overwrite-running-shim
plan: 05
type: execute
wave: 4
depends_on:
  - 06-01-shutdown-signal-package-PLAN
  - 06-02-main-go-wiring-PLAN
  - 06-03-nsis-preinstall-shim-PLAN
  - 06-04-docs-update-PLAN
files_modified: []
autonomous: false
requirements: [INST-06]
tags: [release, ci, tag, latest-json, github-release]

must_haves:
  truths:
    - "A new git tag `v1.0.1` exists on origin/master at a commit that contains all four prior plan's changes (Plans 01-04)."
    - "The GitHub Release for `v1.0.1` exists at https://github.com/boejowen/SquireBot/releases/tag/v1.0.1 with three attached artifacts: `SquireBot-Setup-1.0.1.exe`, `squirebot.exe` (bare binary), `latest.json`."
    - "The `latest.json` manifest contains `\"version\": \"1.0.1\"` and fresh sha256 hashes for both the installer and the bare binary."
    - "The release workflow's AUTH-03 PRODUCTION consent_screen gate (release.yml lines 136-139) passes — i.e., the build did NOT regress to Testing-mode OAuth."
    - "An existing v1.0.0 watcher running on a soak host can self-update to v1.0.1 via the auto-update path (`minio/selfupdate`) consuming the updated `latest.json` (deferred verification: smoke at next daily check or via manual `Check for updates` tray menu)."
  artifacts:
    - path: (git tag)
      provides: "v1.0.1 tag on master at HEAD-of-Phase-6-merge"
    - path: (GitHub Release)
      provides: "SquireBot-Setup-1.0.1.exe, squirebot.exe, latest.json"
    - path: (latest.json contents)
      provides: "version=1.0.1, fresh installer_sha256 and binary_sha256, fresh download URLs pointing at the v1.0.1 release assets"
  key_links:
    - from: git tag v1.0.1
      to: .github/workflows/release.yml (tag-push trigger)
      via: "on.push.tags: ['v*']"
      pattern: "v\\d+\\.\\d+\\.\\d+"
    - from: release.yml
      to: GitHub Releases page (artifacts + body)
      via: "softprops/action-gh-release@v3"
      pattern: "github\\.com/boejowen/SquireBot/releases"
---

<objective>
Ship the v1.0.1 watcher binary release that closes Phase 6. This plan is the ship gate per ROADMAP §49 success criterion 5: "Watcher binary v1.0.1 is built, tagged, and published on GitHub Releases; the `latest.json` manifest is updated so existing watchers can self-update to it."

Per CONTEXT.md D-07 the release tag IS the ship gate. The release pipeline at `.github/workflows/release.yml` is already configured for tag-driven releases (`on.push.tags: ['v*']`) and will:
1. Verify the AUTH-03 PRODUCTION consent_screen gate (lines 128-147).
2. Build `dist/squirebot.exe` with v1.0.1 ldflags (lines 149-168).
3. Run `makensis` against `installer/squirebot.nsi` (lines 170-178) — this picks up the Plan 03 pre-install shim automatically.
4. Compute SHA-256 for installer + bare binary (lines 230-241).
5. Write `dist/latest.json` with the new version + fresh hashes + correct URLs (lines 243-266).
6. Create the GitHub Release with all three artifacts attached (lines 278-317).

No edits to `release.yml` are required (CONTEXT.md confirms: "No CI changes needed for Phase 6 unless planner discovers a missing NSIS plugin"). Plan 03 uses only bundled NSIS primitives per D-05, so the CI's vanilla `choco install nsis` is sufficient.

This plan is NON-AUTONOMOUS because:
- The tag-push triggers GitHub Actions on the user's account; the user must be present to authenticate the push and to verify the CI run.
- The post-tag verification (GitHub Release page exists, artifacts attached, latest.json correct) requires the user's GitHub access.
- The end-user UAT (v1.0.0 → v1.0.1 upgrade on a clean Win11 VM) is the canonical proof of ROADMAP §44-46 — the user owns the VM and the install.

Output: tag `v1.0.1` pushed to origin; CI green; GitHub Release populated; user has confirmed end-to-end upgrade smoke (or scheduled it as a follow-up routine).
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/STATE.md
@.planning/phases/06-installer-overwrite-running-shim/06-CONTEXT.md
@.planning/phases/06-installer-overwrite-running-shim/06-01-SUMMARY.md
@.planning/phases/06-installer-overwrite-running-shim/06-02-SUMMARY.md
@.planning/phases/06-installer-overwrite-running-shim/06-03-SUMMARY.md
@.planning/phases/06-installer-overwrite-running-shim/06-04-SUMMARY.md
@.github/workflows/release.yml
</context>

<tasks>

<task type="auto">
  <name>Task 1: Pre-tag readiness sweep</name>
  <files></files>
  <read_first>
    - .planning/phases/06-installer-overwrite-running-shim/06-01-SUMMARY.md
    - .planning/phases/06-installer-overwrite-running-shim/06-02-SUMMARY.md
    - .planning/phases/06-installer-overwrite-running-shim/06-03-SUMMARY.md
    - .planning/phases/06-installer-overwrite-running-shim/06-04-SUMMARY.md
    - .github/workflows/release.yml (the entire workflow — confirm no changes needed)
  </read_first>
  <action>
Run the following checks BEFORE pushing the tag. All must pass.

**Code state on master:**

1. `git status` — working tree clean. No uncommitted changes from Plans 01-04.
2. `git log --oneline -20` — Plans 01-04 commits are present on the current branch.
3. `git branch --show-current` — confirm we are on `master` (release.yml triggers on `refs/tags/v*` regardless of branch, but tagging the wrong commit is the most common shipping bug).
4. `go build ./...` exits 0.
5. `go vet ./...` exits 0.
6. `go test ./...` exits 0 (Plan 01's `internal/system` tests included).
7. `Select-String -Path cmd/squirebot/main.go -Pattern 'os\.Args\[1\] == "--quit"'` matches 1 line (Plan 02 shipped).
8. `Select-String -Path installer/squirebot.nsi -Pattern '; -- INST-06 \(overwrite-running shim\) --'` matches 1 line (Plan 03 shipped).
9. `Select-String -Path docs/troubleshooting.md -Pattern "Installer won't overwrite a running SquireBot" -SimpleMatch` matches 0 lines (Plan 04 Task 1 shipped).
10. `Select-String -Path docs/build-and-install.md -Pattern '^### Manual debug aids$'` matches 1 line (Plan 04 Task 2 shipped).

**Tag uniqueness:**

11. `git tag --list v1.0.1` returns empty (the tag does not exist yet locally).
12. `git ls-remote --tags origin v1.0.1` returns empty (the tag does not exist on origin yet).

If any check fails, STOP. Fix the missing/broken plan output before tagging. A failed CI on a published tag is messy to recover (re-tagging the same name requires force-push, which violates the project's git safety conventions).

**No git mutations in this task.** Tasks 2-3 perform the tag + push.
  </action>
  <verify>
    <automated>powershell -NoProfile -Command "$ok = $true; git status --porcelain | ForEach-Object { Write-Host \"DIRTY: $_\"; $ok = $false }; if (-not (git rev-parse --abbrev-ref HEAD).Equals('master')) { Write-Host 'NOT_ON_MASTER'; $ok = $false }; if ((git tag --list v1.0.1).Length -gt 0) { Write-Host 'TAG_EXISTS_LOCAL'; $ok = $false }; if ((git ls-remote --tags origin v1.0.1).Length -gt 0) { Write-Host 'TAG_EXISTS_REMOTE'; $ok = $false }; if (-not $ok) { exit 1 } else { Write-Host PASS; exit 0 }"</automated>
  </verify>
  <acceptance_criteria>
    - `git status --porcelain` returns empty (clean tree).
    - `git rev-parse --abbrev-ref HEAD` returns `master`.
    - `git tag --list v1.0.1` returns empty (no prior local tag).
    - `git ls-remote --tags origin v1.0.1` returns empty (no prior remote tag).
    - `go build ./...` exits 0.
    - `go vet ./...` exits 0.
    - `go test ./...` exits 0 with all `internal/system` tests passing.
    - All 4 file-state grep checks above return the expected match counts.
  </acceptance_criteria>
  <done>
    The repository is in a known-good state ready for tagging. Plans 01-04 are all merged to master, all builds/tests/vets are green, and the v1.0.1 tag is not yet claimed locally or remotely.
  </done>
</task>

<task type="checkpoint:decision" gate="blocking">
  <name>Task 2: Confirm tag-push readiness</name>
  <decision>Ready to tag v1.0.1 and trigger the release pipeline?</decision>
  <context>
The next task will:
- Create local tag `v1.0.1` at current HEAD on master.
- Push the tag to origin (`git push origin v1.0.1`).
- The push triggers `.github/workflows/release.yml`. CI takes ~5-10 minutes.

GitHub Actions will:
- Verify AUTH-03 PRODUCTION consent_screen gate.
- Build `dist/squirebot.exe` with v1.0.1 ldflags.
- Run makensis (picks up the Plan 03 pre-install shim).
- Compute sha256s.
- Generate `latest.json`.
- Create the GitHub Release with all 3 artifacts.

If CI fails after the tag push, recovering requires:
- Deleting the tag remotely (`git push origin :refs/tags/v1.0.1`) AND locally (`git tag -d v1.0.1`).
- Fixing the issue.
- Re-tagging (lose the original commit reference — the new tag will be at a new commit).
- Re-pushing.

If CI partially succeeds (e.g., binary built but Release creation failed), `softprops/action-gh-release@v3` has the known bug per release.yml lines 285-292 that re-tagging replaces assets but not body. Acceptable.

Before approving:
- Confirm the OAUTH_CONFIG_JSON repo secret is still populated and consent_screen_status is still PRODUCTION (the gate fails the build if not — release.yml:136).
- Confirm you can monitor https://github.com/boejowen/SquireBot/actions in the next 10 minutes.
  </context>
  <options>
    <option id="proceed">
      <name>Proceed: tag + push v1.0.1</name>
      <pros>Triggers CI immediately; ship-gate process aligns with v1.0.0 precedent.</pros>
      <cons>Cannot un-do without remote tag deletion if CI fails.</cons>
    </option>
    <option id="dry-run">
      <name>Dry-run first: workflow_dispatch the release workflow with version=1.0.1-rc1</name>
      <pros>Tests the CI pipeline end-to-end without committing to a v1.0.1 tag; produces a `prerelease` GitHub Release that is clearly marked as a release candidate.</pros>
      <cons>Adds an extra step; the rc1 binary will not auto-update existing v1.0.0 watchers (latest.json points at the rc; users with v1.0.0 get rc1 as the "latest"). Acceptable for a project with 12 known guildies — they can ignore the rc.</cons>
    </option>
    <option id="abort">
      <name>Abort: surface a blocker</name>
      <pros>If Task 1 sweep revealed any unexpected state, this is the safe option.</pros>
      <cons>Phase 6 does not ship until the blocker is resolved.</cons>
    </option>
  </options>
  <resume-signal>Select: proceed, dry-run, or abort</resume-signal>
</task>

<task type="auto">
  <name>Task 3: Tag + push v1.0.1 + monitor CI</name>
  <files></files>
  <read_first>
    - .github/workflows/release.yml (entire — to understand expected duration + failure modes)
    - .planning/phases/06-installer-overwrite-running-shim/06-CONTEXT.md (D-07 tag convention)
  </read_first>
  <action>
Conditional on Task 2's decision:

**If `proceed`:**

1. Tag the current HEAD:
   ```
   git tag -a v1.0.1 -m "Phase 6 (INST-06): installer overwrite-running shim. Watcher binary v1.0.1 release."
   ```
   Annotated tag (`-a` + `-m`) matches the v1.0.0 precedent (per STATE.md "Tag `v1.0.0` pushed" + v1.0 archive). Body should mention Phase 6 + INST-06 for git-log archaeology.

2. Push the tag:
   ```
   git push origin v1.0.1
   ```

3. Monitor the workflow run:
   ```
   gh run watch --exit-status
   ```
   This blocks until the most recent `release` workflow run completes (success or failure). Expected duration: ~5-10 minutes.

4. After the run completes successfully, verify:
   ```
   gh release view v1.0.1
   ```
   Should list 3 artifacts: `SquireBot-Setup-1.0.1.exe`, `squirebot.exe`, `latest.json`.

5. Verify the latest.json contents:
   ```
   gh release download v1.0.1 -p latest.json -O /tmp/latest.json
   cat /tmp/latest.json
   ```
   Must contain `"version": "1.0.1"`, `"installer_sha256"`, `"binary_sha256"`, and URLs pointing at `/releases/download/v1.0.1/`.

**If `dry-run`:**

Use the workflow_dispatch input flow instead of a tag push:
```
gh workflow run release.yml -f version=1.0.1-rc1
gh run watch --exit-status
gh release view v1.0.1-rc1   # if the workflow created a Release for the dispatch (it does NOT — release creation is gated on startsWith(github.ref, 'refs/tags/v'))
```

Note: `gh workflow run` does NOT push a tag, so the `Create GitHub Release` step at release.yml:278 is SKIPPED (its `if:` clause requires a tag ref). The dry-run produces only the build artifacts under the Actions run; download them via `gh run download <run-id>` and inspect manually. After verification, return to Task 2 and select `proceed` to do the real tag push.

**If `abort`:**

Surface the blocker via STATE.md update (`gsd-sdk query` route as appropriate). Do not push the tag.

**Failure recovery (if CI fails after tag push):**

The single most common failure mode is the AUTH-03 PRODUCTION gate (release.yml:136-139). If the secret has drifted to Testing or been cleared:
1. Re-populate `OAUTH_CONFIG_JSON` from `.planning/phases/01-end-to-end-thin-slice/oauth-config.json` (local file).
2. Delete the failed tag: `git push origin :refs/tags/v1.0.1 && git tag -d v1.0.1`.
3. Restart from Task 2.

DO NOT force-push a different commit under the same tag. The auto-updater hashes the BYTES of the released binary; updating the tag without updating latest.json would orphan all consumers. Always delete + re-create.
  </action>
  <verify>
    <automated>powershell -NoProfile -Command "if (-not (git tag --list v1.0.1)) { Write-Host LOCAL_TAG_MISSING; exit 1 }; $remote = git ls-remote --tags origin v1.0.1; if (-not $remote) { Write-Host REMOTE_TAG_MISSING; exit 1 }; gh release view v1.0.1 --json tagName,assets 2>$null | Out-String | Set-Content /tmp/release.json; if (-not (Test-Path /tmp/release.json) -or (Get-Item /tmp/release.json).Length -lt 10) { Write-Host RELEASE_MISSING; exit 1 }; $assets = (Get-Content /tmp/release.json | ConvertFrom-Json).assets.name; $expected = @('SquireBot-Setup-1.0.1.exe', 'squirebot.exe', 'latest.json'); $missing = $expected | Where-Object { $_ -notin $assets }; if ($missing) { Write-Host \"MISSING_ASSETS: $missing\"; exit 1 }; Write-Host PASS; exit 0"</automated>
  </verify>
  <acceptance_criteria>
    - `git tag --list v1.0.1` returns `v1.0.1` (the local tag was created).
    - `git ls-remote --tags origin v1.0.1` returns a single line with a SHA matching local `git rev-parse v1.0.1^{}` (the tag is on origin).
    - `gh run list --workflow=release.yml --limit 1` shows the latest run with `success` conclusion.
    - `gh release view v1.0.1 --json assets` lists exactly 3 assets: `SquireBot-Setup-1.0.1.exe`, `squirebot.exe`, `latest.json`.
    - `gh release download v1.0.1 -p latest.json -O /tmp/latest.json` succeeds; the downloaded file parses as JSON with `version == "1.0.1"`, non-empty `installer_sha256` (64 hex chars), non-empty `binary_sha256` (64 hex chars).
    - The `installer_url` in latest.json resolves to a 200 OK via `Invoke-WebRequest -Method Head` (asset is downloadable).
    - The `binary_url` in latest.json resolves to a 200 OK via `Invoke-WebRequest -Method Head` (auto-updater consumes this).
    - No AUTH-03 regression: the workflow's `Load OAuth constants` step (release.yml:128) reported PRODUCTION (visible in the run log).
  </acceptance_criteria>
  <done>
    Tag `v1.0.1` is on origin, the GitHub Release exists with all 3 artifacts, latest.json contains the new version + fresh hashes, both download URLs resolve, and CI did not regress the AUTH-03 PRODUCTION gate. Existing watchers in the field will auto-update on their next 24h check or via the user's `Check for updates` tray menu.
  </done>
</task>

<task type="checkpoint:human-verify" gate="blocking">
  <name>Task 4: End-to-end UAT — v1.0.0 to v1.0.1 upgrade on a clean Win11 VM</name>
  <what-built>
The full Phase 6 deliverable: a v1.0.1 installer that upgrades cleanly over a running v1.0.0 or v1.0.1 watcher without prompting the user to quit the tray manually.
  </what-built>
  <how-to-verify>
This UAT validates ROADMAP §44-48 success criteria 1-3 end-to-end. The CI artifact gate (Task 3) covered criterion 5; the docs deletion (Plan 04 Task 1) covered criterion 4. This task closes criteria 1-3.

**Setup (if not already done from a prior soak):**

1. On a clean Windows 11 VM (or your daily-driver if you accept the install):
   - Download the v1.0.0 installer: https://github.com/boejowen/SquireBot/releases/download/v1.0.0/SquireBot-Setup-1.0.0.exe
   - Run it. Walk through the SmartScreen "More info -> Run anyway" wall.
   - Complete the wizard. Confirm tray icon goes green.
   - Make a small edit in your EQ folder (e.g., `/outputfile inventory` from in-game, or copy a fixture .txt file into the folder) to confirm the watcher writes successfully to its workbook.
   - Note `_meta.last_heartbeat` timestamp from the workbook.

**The actual UAT:**

2. Download the v1.0.1 installer: https://github.com/boejowen/SquireBot/releases/download/v1.0.1/SquireBot-Setup-1.0.1.exe

3. WITHOUT first stopping the tray, double-click `SquireBot-Setup-1.0.1.exe`.

4. Walk through SmartScreen (same as v1.0.0; per-binary-hash reputation).

5. Observe the install. Expected behavior:
   - The installer's "Installing..." progress bar advances within ~10 seconds.
   - The tray icon disappears briefly (~1-2 seconds) when --quit fires.
   - A new tray icon appears within seconds (the post-install Exec line 105).
   - No dialog box anywhere asks you to stop SquireBot first.

6. After install completes:
   - Right-click the new tray icon. Confirm version (via About menu or hover tooltip) shows 1.0.1.
   - Make another EQ folder edit (or wait for the next auto-poll).
   - Confirm the workbook's `_meta.last_heartbeat` updates within the watcher's debounce + write window.
   - Confirm NO browser tab opens asking you to re-authenticate Google (tokens in wincred should be untouched per CONTEXT.md prior_decisions row 1).

**Pass criteria (all required):**

- [ ] The installer ran to completion without any "please close SquireBot" dialog.
- [ ] Tray icon flickered (disappeared briefly) then reappeared green.
- [ ] After install, version displayed is `1.0.1`.
- [ ] Workbook continues to receive heartbeat updates (no token re-auth required).
- [ ] `%LOCALAPPDATA%\SquireBot\squirebot.log` shows a clean shutdown line ("shutdown signal received") followed by a fresh startup sequence from the new binary.

**Optional deeper smoke (for the second guildie on rollout):**

- Install v1.0.1 fresh on a NEVER-INSTALLED machine — confirm version gate path "no prior install" is also clean (DisplayVersion empty → skip --quit → taskkill /F finds nothing → install proceeds).
- Re-run v1.0.1 over v1.0.1 (idempotent upgrade) — confirm the graceful path works when both sides know `--quit`.
  </how-to-verify>
  <resume-signal>
Type one of:
- "approved" — all criteria passed; Phase 6 ships.
- "approved-with-followup: <description>" — passed but a deferred item surfaced (e.g., "soak on a second machine is pending"); Phase 6 ships and the followup goes to backlog.
- "blocked: <reason>" — failure observed; document the failure mode, the workaround (manual stop via Task Manager, then re-install), and surface a hotfix plan.
  </resume-signal>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Local repo → GitHub | Tag push triggers a CI workflow that materializes the OAUTH_CONFIG_JSON secret and builds a signed-by-Microsoft-Defender-only binary distributed to users. |
| GitHub Release → end users | The released installer + bare binary are consumed by 12 known guildies (and any drive-by GitHub visitor). Hash integrity comes from `latest.json` sha256 fields. |
| `gh` CLI authentication | Tag push and release verification depend on the user's local GitHub credentials. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-06-20 | Tampering | Release.yml secret OAUTH_CONFIG_JSON is materialized to a known repo path before build | accept | Pre-existing risk inherited from v1.0.0 release. The secret is GitHub-encrypted at rest and only exposed to the build runner. release.yml line 73 writes it to a gitignored path; no leak vector. |
| T-06-21 | Repudiation | Tag was created on the wrong commit | mitigate | Task 1's pre-tag readiness sweep confirms `master` is current and file-state checks confirm all four plans are merged. The annotated tag message (`-m`) includes "Phase 6 (INST-06)" for git-log traceability. |
| T-06-22 | Information Disclosure | The new binary embeds OAuth client secret via ldflags (same as v1.0.0) | accept | Per user memory: "Google /token requires client_secret with PKCE — the desktop secret is effectively public per Google's own docs." This is documented in build-and-install.md and is NOT a Phase 6 regression. |
| T-06-23 | Denial of Service | Re-tagging the same v1.0.1 multiple times on CI failure could replace assets and orphan auto-updater consumers | mitigate | Task 3's failure-recovery instructions explicitly call out the auto-updater hash dependency and prescribe "delete tag, fix, re-tag" rather than force-push. The `softprops/action-gh-release@v3` known bug at release.yml:285-292 affects body text only; latest.json is regenerated each run with current sha256s, so the auto-updater self-corrects. |
| T-06-24 | Spoofing | A v1.0.1 release page could be impersonated via similarly-named org | accept | Unrelated to Phase 6 — same threat surface as every prior release. The README links use the canonical `github.com/boejowen/SquireBot` URL; users typing it manually is the only attack vector and is bounded by GitHub's namespace squatting policies. |

ASVS L1: no `high` severity threats introduced by this plan. All risks inherited from the v1.0.0 ship process.
</threat_model>

<verification>
- `git tag --list v1.0.1` returns the tag.
- `git ls-remote --tags origin v1.0.1` returns the tag with matching SHA.
- GitHub Release page exists at `https://github.com/boejowen/SquireBot/releases/tag/v1.0.1`.
- Three artifacts attached (installer, bare binary, latest.json).
- latest.json contains `version=1.0.1` + fresh sha256s + correct download URLs.
- AUTH-03 PRODUCTION gate did not regress.
- (Human-verified) UAT pass: v1.0.0 → v1.0.1 upgrade on a clean VM with no manual stop prompt; post-install version is 1.0.1; workbook heartbeats continue; no token re-auth.
</verification>

<success_criteria>
- ROADMAP §49 success criterion 5 covered: binary v1.0.1 built, tagged, published, `latest.json` updated.
- ROADMAP §44-46 success criteria 1-3 verified end-to-end via UAT (Task 4).
- Phase 6 ships. STATE.md and ROADMAP.md updated to mark Phase 6 complete + 1/3 v1.0.1 phases done.
- D-07 honored: tag is `v1.0.1`, annotated, on master.
- D-06 honored: latest.json schema unchanged from v1.0.0; only contents (version + hashes + URLs) updated.
- The auto-update path (Plan 02-06 / OPS-04) now serves v1.0.1 to all v1.0.0 watchers in the field.
</success_criteria>

<output>
After Task 4 approval, create `.planning/phases/06-installer-overwrite-running-shim/06-05-SUMMARY.md` capturing:
- Final tag SHA + tag message.
- GitHub Release URL + asset list.
- latest.json contents (paste the full JSON for archive).
- UAT result (pass/pass-with-followup/fail).
- Any observed deviations from the planned behavior (e.g., taskkill /F fired before --quit completed, indicating poll loop tuning needed for next phase).
- Confirmation that ROADMAP.md and STATE.md are updated.

Also update:
- `.planning/STATE.md` — mark Phase 6 complete, increment progress.completed_phases to 1, set last_activity to today, last_updated to now.
- `.planning/ROADMAP.md` — check the Phase 6 box, update Progress table to 1/3 phases complete.
- `.planning/MILESTONES.md` — add a row for v1.0.1 progress (Phase 6 done, Phase 7 + 8 pending).
- Memory note (if appropriate): "Phase 6 shipped 2026-MM-DD; v1.0.1 installer overwrite-running shim live."
</output>
</content>
</invoke>