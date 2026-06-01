---
phase: 16-cutover-decommission
plan: 02
subsystem: release-ops
tags: [github-release, release.yml, minio-selfupdate, semver, ops, human-action]

# Dependency graph
requires:
  - phase: 13-watcher-retarget-onboarding
    provides: "the re-targeted Google-free ~7MB watcher + release.yml (on push tags v*) + the minio/selfupdate auto-updater + the 999.22 SemVer compare"
provides:
  - "a published, non-prerelease v2.0.0 GitHub Release carrying squirebot.exe + latest.json (version 2.0.0) — the auto-update flip trigger"
  - "/releases/latest/download/latest.json now resolves to v2.0.0 (the flip is ARMED for running v1.0.x watchers)"
affects: [16-03-flip-deploy-mint-herd, milestone-close-v2.0]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Clean v2.0.0 tag (no hyphen) so release.yml does NOT mark it prerelease (Pitfall 4 — /releases/latest ignores prereleases)"

key-files:
  created:
    - "(external artifact) GitHub Release v2.0.0 — published by release.yml on the pushed tag; assets: squirebot.exe, latest.json, SquireBot-Setup-2.0.0.exe, SquireBot-Setup.exe"
  modified: []

key-decisions:
  - "Executed by the orchestrator on the maintainer's explicit authorization ('please do the publish yourself'), NOT a gsd-executor agent — 16-02 is autonomous:false (a public release that auto-updates ~12 users); the maintainer directed and the orchestrator performed + verified"
  - "Pushed master (188 commits) first so the full v2.0 history is on the remote, then the clean v2.0.0 tag"

patterns-established:
  - "999.22 prerelease-comparison caveat surfaced live: final-vs-final watchers (v1.0.x) auto-update to 2.0.0; a watcher stuck on a PRE-1.0 prerelease (e.g. 0.4.0-rc1) mis-judges newer versions and needs a MANUAL reinstall — documented in install.md/troubleshooting.md"

requirements-completed: [CUTOVER-03]

# Metrics
completed: 2026-05-31
---

# Phase 16 Plan 02: Publish the v2.0.0 GitHub Release Summary

**Published a clean, non-prerelease `v2.0.0` GitHub Release carrying the re-targeted Google-free `squirebot.exe` + `latest.json` (version 2.0.0) — the literal trigger that arms the guild-wide auto-update flip (CUTOVER-03).**

> **Reconciliation note:** 16-02 is `autonomous: false`. It was executed conversationally by the orchestrator on the maintainer's explicit instruction ("please do the publish yourself"), not by a `gsd-executor` agent. This SUMMARY is written after the fact to record what happened and keep GSD state consistent.

## What was done (orchestrator, maintainer-authorized)

1. **Pre-flight (go/no-go):** confirmed no `v2.x` tag/release existed, `v1.0.2` was still GitHub "Latest", `release.yml` present, HEAD = `b7c4939`, branch `master`. Green.
2. **Push:** `git push origin master` (188 commits — the full v2.0 history) → `git tag v2.0.0` → `git push origin v2.0.0`. Remote tag confirmed at `b7c4939`.
3. **Build:** `release.yml` ran on the tag — green in **1m34s** (Go build of the bare squirebot.exe, NSIS installer, SHA-256 sums, latest.json manifest, GitHub Release created).

## Verification (the Pitfall-4 gate)

| Check | Result |
|---|---|
| `isPrerelease` | `false` |
| GitHub `/releases/latest` tag | `v2.0.0` |
| Assets | `squirebot.exe`, `latest.json`, `SquireBot-Setup-2.0.0.exe`, `SquireBot-Setup.exe` |
| `latest.json` | `version: 2.0.0`, `binary_url` → `.../v2.0.0/squirebot.exe` |
| `gh release list` | `v2.0.0` = Latest (above v1.0.2) |

So `/releases/latest/download/latest.json` resolves to v2.0.0 and any running v1.0.x watcher computes `IsNewer("1.0.x","2.0.0") == true` → auto-updates on its next check. **The flip is ARMED.**

## Issues / findings
- **Prerelease-stuck watchers won't auto-update (999.22):** the maintainer's own box was on `0.4.0-rc1` (a 16 MB Google-era prerelease). Its old comparison logs "auto-update: no newer version" even against manifest 2.0.0 — `IsNewer("0.4.0-rc1","2.0.0")` wrongly returns false. Such watchers need a MANUAL reinstall from the v2.0.0 installer; v1.0.x FINAL watchers are unaffected. Captured in the updated `install.md`/`troubleshooting.md`.

## User Setup Required
The herd/announce + per-guildie code distribution are Plan 16-03. The maintainer holds the installer permalink (`/releases/latest/download/SquireBot-Setup.exe`) for the channel announcement.

## Self-Check: PASSED

`gh release view v2.0.0` confirms `isPrerelease:false`; `gh api .../releases/latest` returns `v2.0.0`; the published `latest.json` reports version 2.0.0 with a v2.0.0 binary_url; `gh release list` shows v2.0.0 as Latest. No repo source files changed (this plan publishes an artifact).

---
*Phase: 16-cutover-decommission*
*Completed: 2026-05-31*
