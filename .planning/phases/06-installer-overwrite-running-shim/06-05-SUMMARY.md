---
phase: 06-installer-overwrite-running-shim
plan: 05
subsystem: release
tags: [release, ci, tag, latest-json, github-release, ship-gate, inst-06]
requirements: [INST-06]
status: complete-pending-uat
completed: 2026-05-11
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
  duration: "~5 min (tag push + CI watch + verification + summary)"
  tasks_completed: "3 of 4 (Tasks 1–3 closed; Task 4 deferred to a separate UAT routine)"
  files_modified: 3
  commits: 1
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

## Deferred

**Task 4 — End-to-end v1.0.0 → v1.0.1 UAT on clean Win11 VM.** Per the plan's `<task type="checkpoint:human-verify">` Task 4 spec:

> Walk through the SmartScreen wall with `SquireBot-Setup-1.0.1.exe`, observe the tray flicker, confirm version 1.0.1 displays, confirm workbook heartbeats continue with no token re-auth, confirm `squirebot.log` shows a clean shutdown line followed by a fresh startup.

This UAT is the only proof-of-life for ROADMAP §44-46 success criteria 1–3 (the actual installer-overwrite behavior in the field). The release artifacts are shipped and the CI-side proof (criterion 5) is complete, but the user-side proof (criteria 1–3) requires a clean Win11 VM with a pre-existing v1.0.0 install. **Mark INST-06 as fully verified only after this UAT passes.**

Suggested followup: run the UAT on the user's daily-driver or a clean Hyper-V Win11 VM; record the result in a hotfix-plan note if it fails. If it passes, append a confirmation line to this SUMMARY and STATE.md.

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
