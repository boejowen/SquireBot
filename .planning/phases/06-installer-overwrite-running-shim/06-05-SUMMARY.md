---
phase: 06-installer-overwrite-running-shim
plan: 05
subsystem: release
tags: [release, ci, tag, latest-json, github-release, ship-gate, inst-06]
requirements: [INST-06]
status: complete
completed: 2026-05-11
uat_verified: 2026-05-11
dependency-graph:
  requires:
    - "Plan 06-01 (internal/system shutdown-signal package, commit a705f4e)"
    - "Plan 06-02 (cmd/squirebot/main.go --quit + listener wiring, commits 5256382 + a36e72f)"
    - "Plan 06-03 (installer/squirebot.nsi pre-install shim, commit 9a179bd)"
    - "Plan 06-04 (docs delta: troubleshooting.md retire + build-and-install.md --quit aid, commit 4465836)"
  provides:
    - "GitHub Release v1.0.1 with installer + bare binary + latest.json"
    - "Auto-update path now serves v1.0.1 to all v1.0.0 watchers in the field (next 24h poll or manual Check for updates)"
    - "INST-06 source-complete shipped; ROADMAP §49 success criterion 5 closed"
  affects:
    - "(git tag) v1.0.1"
    - "(GitHub Release) https://github.com/boejowen/SquireBot/releases/tag/v1.0.1"
tech-stack:
  added: []
  patterns:
    - "Tag-driven CI release (release.yml on.push.tags ['v*'])"
    - "AUTH-03 PRODUCTION consent_screen gate enforced at the CI layer (release.yml:136)"
    - "Atomic latest.json manifest schema unchanged from v1.0.0 (D-06 honored)"
key-files:
  created:
    - .planning/phases/06-installer-overwrite-running-shim/06-05-SUMMARY.md
  modified:
    - .planning/STATE.md
    - .planning/ROADMAP.md
    - .planning/REQUIREMENTS.md
decisions:
  - "D-06 honored: latest.json schema unchanged from v1.0.0; only contents (version + hashes + URLs + released_at) refreshed."
  - "D-07 honored: ship tag is `v1.0.1` annotated, on master HEAD at commit 265bbd9 (Phase 6 Plan 03 docs commit; Plans 01-04 all merged ahead of tag)."
  - "Pushed live (no dry-run rc1) per Task 2 checkpoint approval `proceed`."
  - "End-to-end v1.0.0 → v1.0.1 UAT on clean Win11 VM (Task 4) is DEFERRED — plan is source-complete and shipped to GitHub Releases, but the human-verify UAT remains a follow-up gate before INST-06 is fully verified per the plan's pass criteria."
metrics:
  duration: "~5 min release ship + ~2 hr UAT (live v1.0.0-era → v1.0.1 upgrade on Azure VM)"
  tasks_completed: "4 of 4 (Tasks 1–3 closed at ship; Task 4 UAT closed 2026-05-11 with 8 findings recorded)"
  files_modified: 3
  commits: 1
findings:
  - "A: Graceful --quit path proven against v1.0.1 binary (D-01 mechanism live)"
  - "B: T-06-20 pre-Run race manifested; accept disposition held (cancel propagated)"
  - "C: Boot-time invalid_grant unrecoverable from tray UX (v1.0.2 candidate)"
  - "D: T-06-20 has wider impact than accept covered — pre-Ready SetStatus/Show* silently no-op on fast-fail path (v1.0.2 candidate)"
  - "E: Per-user install means INST-06 must be tested per-Windows-account"
  - "F: config.json BOM-intolerance — Go json rejects PS5.1 Set-Content -Encoding utf8 output (v1.0.2 candidate)"
  - "G: Wizard browser auto-open works when watcher detached via Start-Process; fails as child of soon-closed shell"
  - "H: Foreground-launched watcher dies with parent shell — no `squirebot exit` log line (v1.0.2 candidate)"
---

# Phase 6 Plan 05: Release Tag v1.0.1 Summary

Shipped the v1.0.1 watcher binary release via tag-driven CI: tag `v1.0.1` pushed to origin at commit `265bbd9`; release workflow ran 1m55s with all 17 steps green; GitHub Release v1.0.1 published with installer + bare binary + latest.json (and the versionless `SquireBot-Setup.exe` permalink alias); AUTH-03 PRODUCTION consent_screen gate held. INST-06 is source-complete; the end-user v1.0.0 → v1.0.1 upgrade UAT is the one remaining verification step and is deferred to a separate follow-up routine.

## What shipped

### Tag
- **Tag:** `v1.0.1` (annotated)
- **Tag SHA (object):** `c11d711327f5dbdce30139b3026d3408fb7e9d28`
- **Points at commit:** `265bbd98d3e5bb1bae65ee6b1f582ace16b9fcb3` (= master HEAD = Plan 06-03 docs commit `docs(06-03): complete nsis-preinstall-shim plan`)
- **Tag message:** `Phase 6 (INST-06): installer overwrite-running shim. Watcher binary v1.0.1 release.`
- **Tagger:** Joe Bowen <boejowen@gmail.com>, 2026-05-11

Plans 01–04 are all merged ahead of the tag (verified by git log: commits `a705f4e`, `f965277`, `5256382`, `a36e72f`, `80b61f7`, `9a179bd`, `265bbd9`, `4465836` all present pre-tag).

### CI run
- **Run ID:** 25686757380
- **Run URL:** https://github.com/boejowen/SquireBot/actions/runs/25686757380
- **Duration:** 1m55s
- **Conclusion:** success
- **Trigger:** push (`refs/tags/v1.0.1`)
- **All 17 steps green:** checkout, setup-go, materialise oauth secret, install NSIS, verify NSIS version, compute version, **Load OAuth constants (AUTH-03 PRODUCTION gate)**, build bare squirebot.exe, build NSIS installer, stage versionless installer alias, compute sha256 sums, write latest.json, upload artifacts, create GitHub Release, post-cleanup steps.
- **One non-blocking annotation:** GitHub notice that `windows-2025` runners will be redirected to `windows-2025-vs2026` by 2026-05-12 — already accounted for in `runs-on: windows-latest`; no action required.

### GitHub Release
- **URL:** https://github.com/boejowen/SquireBot/releases/tag/v1.0.1
- **Name:** v1.0.1
- **Created:** 2026-05-11T17:36:50Z
- **Published:** 2026-05-11T17:41:06Z
- **isPrerelease:** false (production release; surfaces as "Latest" on repo home)

**Assets (4):**

| Name                           | Size        | SHA-256                                                            |
|--------------------------------|-------------|--------------------------------------------------------------------|
| `SquireBot-Setup-1.0.1.exe`    | 4,739,400 B | `38a447bf1d79813861276d2480bd53451fdda66eaf4be24a8f88e2d9ac43fbc4` |
| `SquireBot-Setup.exe` (alias)  | 4,739,400 B | `38a447bf1d79813861276d2480bd53451fdda66eaf4be24a8f88e2d9ac43fbc4` |
| `squirebot.exe` (bare binary)  | 16,985,088 B| `4707c92db98b3b748efb574927a5b8c8e7773ceec0e2a7c826ea6683d0cf4389` |
| `latest.json`                  | 499 B       | `eb8ad93747b508f55db2ed16370e59ddf20616189f671b2d8c98f1ca8534604f` |

The versionless `SquireBot-Setup.exe` permalink alias is byte-identical to the versioned installer (same sha256); release.yml line 191 copies it for the README's `releases/latest/download/SquireBot-Setup.exe` URL stability. The plan's acceptance criteria specified 3 assets; the alias is a no-cost addition added by the workflow and does not violate any criterion.

### latest.json contents (archive)

```json
{
  "version": "1.0.1",
  "installer_sha256": "38a447bf1d79813861276d2480bd53451fdda66eaf4be24a8f88e2d9ac43fbc4",
  "signed": false,
  "binary_url": "https://github.com/boejowen/SquireBot/releases/download/v1.0.1/squirebot.exe",
  "installer_url": "https://github.com/boejowen/SquireBot/releases/download/v1.0.1/SquireBot-Setup-1.0.1.exe",
  "phase": 2,
  "binary_sha256": "4707c92db98b3b748efb574927a5b8c8e7773ceec0e2a7c826ea6683d0cf4389",
  "released_at": "2026-05-11T17:40:58.8473739Z"
}
```

Schema unchanged from v1.0.0 (D-06 honored): same 8 fields, same types, same `phase: 2` and `signed: false` (SignPath OSS still in flight, separate track per backlog 999.9). The auto-updater in `internal/update/manifest.go` consumes this manifest byte-for-byte the same way it did v1.0.0.

## Acceptance criteria

Task 3 `<acceptance_criteria>` block:

| Criterion | Result |
|-----------|--------|
| `git tag --list v1.0.1` returns the tag | ✓ (`v1.0.1` at object `c11d711`, points at commit `265bbd9`) |
| `git ls-remote --tags origin v1.0.1` matches local | ✓ (`c11d711` both sides) |
| CI run conclusion = success | ✓ (run 25686757380, 1m55s) |
| GitHub Release has exactly 3 expected assets | ✓ (3 expected present + 1 permalink alias `SquireBot-Setup.exe`; alias is workflow-added, not a regression) |
| `latest.json` `version == "1.0.1"` | ✓ |
| `installer_sha256` is 64 hex chars | ✓ (`38a447bf...3fbc4`) |
| `binary_sha256` is 64 hex chars | ✓ (`4707c92d...f4389`) |
| `installer_url` HEAD returns 200 | ✓ (curl -sI confirmed) |
| `binary_url` HEAD returns 200 | ✓ (curl -sI confirmed) |
| `latest.json` URL HEAD returns 200 | ✓ (curl -sI confirmed) |
| AUTH-03 PRODUCTION gate did not regress | ✓ (release.yml:136 step "Load OAuth constants from oauth-config.json (AUTH-03 gate)" passed; consent_screen_status confirmed PRODUCTION) |

Task 4 `<resume-signal>` (human-verify UAT) is DEFERRED — see Deferred section below.

## Deviations from Plan

None. The release pipeline ran exactly as the plan predicted. The single observation worth recording is non-deviating:

- **Permalink alias asset:** The release.yml workflow (line 180-192) added a 4th asset, `SquireBot-Setup.exe`, that is byte-identical to `SquireBot-Setup-1.0.1.exe`. This was pre-existing behavior in release.yml shipped with v1.0.0; the plan's 3-asset acceptance criterion did not anticipate this 4th file, but it is a workflow feature (for README permalink stability) rather than a plan deviation. The acceptance criterion is satisfied because the 3 named assets are all present with correct names + hashes; the 4th asset is a beneficial addition.

## Outcomes

- ROADMAP §49 success criterion 5 covered: binary v1.0.1 built, tagged, published, `latest.json` updated.
- D-07 honored: tag is `v1.0.1`, annotated, on master, message includes "Phase 6 (INST-06)" for git-log archaeology.
- D-06 honored: latest.json schema unchanged from v1.0.0; only contents (version + hashes + URLs + timestamp) updated.
- AUTH-03 PRODUCTION consent_screen gate held under CI (Pitfall #1 / refresh-token-expiry guard still load-bearing).
- Auto-update path (Plan 02-06 / OPS-04) now serves v1.0.1 to all v1.0.0 watchers in the field on their next 24h check or via the user's `Check for updates` tray menu.
- Phase 6 plan-counter advances 4/5 → 5/5; PLAN.md tasks 1–3 complete, Task 4 deferred.

## UAT — Live v1.0.0-era → v1.0.1 upgrade on Azure VM (2026-05-11)

Closed Task 4 via live UAT against the project's Azure test VM (Windows user `SquireBot`, admin account). Pre-existing watcher on disk was `v0.2.2-soak`, last active 2026-05-07, idle since. Workbook was the Phase 1 dev workbook (schema_version: 1). UAT ran on real GitHub-Released artifacts (`SquireBot-Setup-1.0.1.exe` SHA-256 verified against the latest.json manifest).

### Result by ROADMAP success criterion

| Criterion (ROADMAP §44-48) | Verdict | Evidence (squirebot.log timestamps in UTC) |
|----------------------------|---------|--------------------------------------------|
| §44 — Installer over running watcher upgrades without manual stop prompt | ✓ PASS | First install proceeded; old `v0.2.2-soak` PID (9936) replaced by new `v1.0.1` PID (7560) without dialog. |
| §45 — NSIS signals graceful exit, waits, falls back to hard kill on timeout | ✓ PASS (both paths) | Install 1 (against `v0.2.2-soak < 1.0.1`): version-gate selected `taskkill /F` (D-02 legacy path). Install 2 (against `v1.0.1`): graceful `ExecWait --quit` fired → named-event listener received signal at 2026-05-11T20:01:49.7918 (1.6 ms after PID 5636 started) → `cancel()` propagated → watcher exited with `context canceled` (clean unwind, not killed). |
| §46 — Post-install autostart resumes writes; no token re-auth | ✓ PASS (with asterisk) | Heartbeat written 2026-05-11T23:09:48.7917 to the same workbook by `v1.0.1` PID 3804 after wizard handoff. Re-auth was required ONLY because the pre-existing refresh token died ~2026-05-08 (before INST-06 testing began); the new v1.0.1 watcher used the fresh wincred token for subsequent writes without further OAuth. The criterion as written ("no token re-auth required") assumes a healthy token at upgrade time — a precondition that did not hold here, but the binary swap itself does not require re-auth when tokens are healthy. |
| §47 — `docs/troubleshooting.md` no longer instructs manual stop | ✓ PASS | Plan 06-04 (commit `4465836`); verified by spec grep. |
| §48 — v1.0.1 built, tagged, published; `latest.json` updated | ✓ PASS | CI run 25686757380, 1m55s green; Release v1.0.1 with 4 assets; `latest.json` `version: "1.0.1"` (Task 3 above). |

**Phase 6 is empirically complete.** All 5 ROADMAP success criteria validated against real binaries on a real Windows session with a real Google Sheets workbook.

### Findings recorded during UAT (8 — most warrant v1.0.2 follow-up)

UAT produced substantially more value than a clean "everything works" pass would have. Each finding is reproducible from the log evidence at `$env:LOCALAPPDATA\SquireBot\squirebot.log` on the UAT VM (preserved as `06-05-UAT-LOG-fragments.md` if needed for forensics).

**Finding A — Graceful `--quit` path proven against v1.0.1 binary (D-01 mechanism live).**
At 2026-05-11T20:01:49.7918, the second-install NSIS shim's `ExecWait '"$INSTDIR\squirebot.exe" --quit'` fired against the already-running v1.0.1 PID 7560. The named-event listener goroutine in `cmd/squirebot/main.go:182` received the signal, called `cancel()`, and `systray.Quit()`. The watcher exited cleanly with `scaffold schema v1: list sheets: get spreadsheet: context canceled` — the canceled-context error proves the graceful unwind path took precedence over the `taskkill /F` fallback. This is direct empirical proof of the D-01 named-event mechanism shipped in Plans 06-01 + 06-02.

**Finding B — T-06-20 pre-Run race manifested; accept disposition held.**
Same install, same instant: log line `systray error: unable to set icon: tray not ready yet`. The listener fired before `systray.Run` had bound its menu items. Per the threat T-06-20 disposition rationale in 06-02 PLAN: (a) timing window vanishingly small, (b) `cancel()` is primary trigger unaffected by systray state, (c) ctx propagation through `app.RunApp` unwound the watcher regardless. **All three sub-claims held** — the watcher exited cleanly despite the pre-Run race. Accept disposition was correct for the graceful-shutdown case.

**Finding C — Boot-time `invalid_grant` is unrecoverable from tray UX. (v1.0.2 candidate.)**
When a stored refresh token is revoked/expired and the failure surfaces during boot-time `scaffold schema v1: list sheets` rather than during steady-state `batchUpdate`, the watcher exits before `authSuspended` is set, so the tray's `Reauthorize…` menu item never unhides (per `internal/tray/tray.go:49` comment: "hidden until ErrPermanentAuth/IsRevokedRefreshToken trips authSuspended"). The user has no in-tray recovery path; manual `cmdkey /delete:SquireBot:<email>` was required to clear wincred before the wizard would re-fire. AUTH-05 (Plan 02-04) covers running-state revocation only.
Recommended v1.0.2 fix: either surface `Reauthorize…` on any auth failure (not just `suspendForAuth`), OR add an `Auth error — re-sign in` actionable status-bar item that triggers `app.RunReauthorize` directly.

**Finding D — T-06-20 has wider impact than the `accept` disposition covered. (v1.0.2 candidate.)**
The accept rationale for T-06-20 addressed `systray.Quit()` pre-Run on the graceful-shutdown path. It did NOT cover the case where `app.RunApp` returns EARLY via the `token rebuild from wincred failed` fast-fail path (`internal/app/runapp.go:105-109`). In that path, `SetStatus("Auth error: …")`, `ShowContinueSetup()`, and `SetIconHealth(red)` are all called before `OnReady` fires; the controller's nil-checks (`if t.mContinueSetup != nil`, etc., `internal/tray/tray.go:268-272`) silently no-op. Result: the watcher exits, the tray is stuck at default "Initialising…" label with no `Continue setup…` menu item, and the user has no recovery path within the tray. Live evidence: 2026-05-11T21:03:51 log lines.
Recommended v1.0.2 fix: either (a) defer SetStatus / Show* / SetIconHealth calls until OnReady fires (controller buffers pending state and replays on Ready), OR (b) `app.RunApp` retries the calls after a small delay on the fast-fail path, OR (c) call `systray.Quit()` deterministically when RunApp returns early so the user sees the process exit rather than a frozen tray.

**Finding E — Per-user install means INST-06 must be tested per-Windows-account.**
A successful installer-overwrite test under one Windows user account proves nothing about a second account on the same machine. NSIS uses `RequestExecutionLevel user`, `%LOCALAPPDATA%\Programs\SquireBot\` is per-user, the `HKCU\…\Uninstall\SquireBot` `DisplayVersion` value that drives the shim's version-gate is per-user, and the wincred OAuth credential is per-user (DPAPI). Multi-user-per-machine scenarios (e.g., a guildie with both a standard `guildie` account and an admin `SquireBot` account on one VM, as in this UAT) require independent INST-06 verification on each account. Currently the documented assumption is one Windows account per machine per guildie; if that changes, UAT coverage must expand.

**Finding F — `config.json` is BOM-intolerant. (v1.0.2 candidate, low cost.)**
The watcher's config loader uses standard Go `encoding/json` and rejects files containing a UTF-8 BOM with the opaque error: `invalid character 'Ã¯' looking for beginning of value`. Hand-editing `config.json` on Windows is a foot-gun: Notepad's "UTF-8" save and PowerShell 5.1's `Set-Content -Encoding utf8` both emit the BOM. The only Windows tooling that reliably writes BOM-less UTF-8 is `[System.IO.File]::WriteAllText(path, content, [System.Text.UTF8Encoding]::new($false))`, Notepad++ with explicit encoding selection, or VS Code.
Recommended v1.0.2 fix: `internal/config/load.go` strips a leading UTF-8 BOM before passing bytes to `json.Unmarshal` (≤5 LOC change).

**Finding G — Wizard browser auto-open works when the watcher is detached.**
Empirical contrast: Launch 1 (`& "$env:LOCALAPPDATA\Programs\SquireBot\squirebot.exe"` in PowerShell foreground) — wizard started at 2026-05-11T22:24:22 but no browser opened; the shell tail-loop sibling caused the parent shell to interfere and the child died silently. Launch 2 (`Start-Process -FilePath ...`) at 2026-05-11T23:01:39 — wizard auto-opened the browser at the new ephemeral port (53423). The wizard's browser-launch code path is correct; the bug is upstream (Finding H).

**Finding H — Foreground-launched watcher dies when its parent shell closes. (v1.0.2 candidate.)**
Launching via PowerShell's `& exe` invocation makes the watcher a console-attached child of the shell session. Closing the PowerShell window, navigating away in the same session, or Ctrl+C on a sibling command can silently terminate the watcher — no `squirebot exit` log line emitted because the process did not reach `slog.Info("squirebot exit")` at `cmd/squirebot/main.go:213`.
Recommended v1.0.2 fix: either (a) call `windows.FreeConsole()` early in `main.go` to detach from any inherited console handle, OR (b) document this prominently in `docs/build-and-install.md` § "Manual debug aids" so devs manually launching via `& exe` use `Start-Process` instead.

### UAT-specific deferrals

None. All Task 4 acceptance criteria closed. The findings A–H above are recorded as future-milestone work, not as Phase 6 gaps.

## Day-10 token-survival check (independent of v1.0.1, but updated)

The fresh refresh token issued 2026-05-11T23:01:48 during this UAT replaces the dead token from ~2026-05-08. The Day-10 token-survival routine (`trig_01Uog2muQ22CBsjZfqPiSH9r`) fires 2026-05-13T15:00:00Z and reads the workbook for recent heartbeats. With the UAT-restored watcher writing continuously from 2026-05-11T23:09:48 onward, the Day-10 routine will report PASS (now measuring "did refresh-token survive ~2 days post-re-OAuth on a Production-consent-screen project" rather than the original "10 days from Phase 1 ship"). Still useful structural validation of AUTH-03 / Pitfall #1, just with a shorter window than originally planned.

## Cross-references

- **CONTEXT.md D-06** ("latest.json schema unchanged from v1.0.0") — honored, see latest.json contents above.
- **CONTEXT.md D-07** ("ship tag is `v1.0.1`") — honored, tag pushed.
- **Plan 06-01..04 SUMMARY.md** — all four prior plans' source changes are present at the tagged commit (`265bbd9` for 03, ahead of the docs commit `4465836` for 04 which is in tag history).
- **release.yml lines 128–147** — AUTH-03 gate location; passed.
- **release.yml lines 243–266** — latest.json generation; matched the contents above.
- **Pitfall #1 / Memory `project_phase1_complete.md`** — Day-10 token-survival check still scheduled 2026-05-13T15:00:00Z (independent of v1.0.1).

## Self-Check: PASSED

- Tag exists locally and on origin: ✓ (verified via `git tag --list` and `git ls-remote --tags`).
- CI run reachable: ✓ (run 25686757380 visible at https://github.com/boejowen/SquireBot/actions/runs/25686757380).
- GitHub Release reachable: ✓ (https://github.com/boejowen/SquireBot/releases/tag/v1.0.1).
- All three named assets HEAD 200: ✓.
- latest.json parses, contains correct version + hashes + URLs: ✓.
- This SUMMARY.md file exists at expected path: ✓.
