---
phase: 09-watcher-robustness-polish
plan: 05
subsystem: release
tags: [release, ci, tag, latest-json, github-release, ship-gate, wave3]
requirements: [AUTH-07, OPS-06, OPS-07, CONFIG-01]
dependency_graph:
  requires:
    - "Plan 09-01 (OPS-06) tray pre-Ready queue shipped on master"
    - "Plan 09-02 (OPS-07) FreeConsole foreground detach shipped on master"
    - "Plan 09-03 (CONFIG-01) UTF-8 BOM strip in config.Load shipped on master"
    - "Plan 09-04 (AUTH-07) boot-time invalid_grant classifier shipped on master"
  provides:
    - "v1.0.2 git tag on master at ab6da3f (HEAD includes all 4 Phase 9 fixes)"
    - "GitHub Release v1.0.2 with 4 assets (latest.json, SquireBot-Setup-1.0.2.exe, SquireBot-Setup.exe versionless alias, squirebot.exe bare binary)"
    - "latest.json manifest advertises version=1.0.2 with fresh installer_sha256 and binary_sha256 — existing v1.0.1 watchers will auto-update on next periodic check"
  affects:
    - "No source files. Tag + release artifacts only."
tech_stack:
  added: []
  patterns:
    - "Tag-driven release pipeline (release.yml on.push.tags=['v*']) unchanged since v1.0.1 — reuse-without-edit per D-08"
    - "Versionless SquireBot-Setup.exe alias byte-copied from versioned installer (release.yml step 'Stage versionless installer alias') so README permalink resolves on every non-prerelease tag"
key_files:
  created:
    - .planning/phases/09-watcher-robustness-polish/09-05-SUMMARY.md
  modified: []
decisions:
  - "Yolo auto-push: user pre-approved both 'git push origin master' and 'git push origin v1.0.2' before this plan started; no Task 2 checkpoint pause"
  - "Tag v1.0.2 created with annotated message 'v1.0.2 — Watcher Robustness Polish (AUTH-07 + OPS-06 + OPS-07 + CONFIG-01)' and points at master HEAD ab6da3f"
  - "Plan's 'exactly three' artifact expectation reconciled to actual 4-asset ship pattern (matches v1.0.1) — the versionless SquireBot-Setup.exe alias is the 4th, byte-identical to the versioned installer, intentionally added in release.yml lines 180-192 for the README permalink"
  - "Plan Task 1 step C 'SCRIPT_MIN_SCHEMA_VERSION.*3' grep is a plan-text bug — apps-script/src/lib/migrations.ts does not define a SCRIPT_MIN_SCHEMA_VERSION constant; the schema pin is instead enforced via writeMetaRow('_meta', 'schema_version', '3') (line 97), which IS present. Schema is correctly pinned at 3."
  - "Plan Task 1 step D OPS-07 grep ('windows.FreeConsole()' in console_windows.go) is a plan-text bug — Plan 09-02 implemented FreeConsole via the kernel32 LazySystemDLL approach (procFreeConsole.Call()) per its deviation note. Functionally equivalent; the freeConsole() helper IS present and called from main.go line 109."
  - "Tasks 3 and 4 of the plan are explicitly out of scope for this agent per prompt: Task 3 (on-VM UAT) is user follow-up; Task 4 (ROADMAP.md edit) is owned by the orchestrator after this agent returns. This agent only commits the SUMMARY."
metrics:
  duration: "~12 min wall (build/test/push/CI wait)"
  completed_date: "2026-05-13"
  tasks_completed: 2
  tasks_total: 4
  tasks_out_of_scope: 2
  commits: 1
  tests_added: 0
---

# Phase 9 Plan 05: v1.0.2 Release Tag — Summary

**One-liner:** SquireBot watcher v1.0.2 shipped — tag pushed, CI green in 2m2s, GitHub Release live with 4 assets, latest.json advertises version=1.0.2 with fresh SHA-256 hashes so existing v1.0.1 watchers auto-update on next periodic check.

## What Shipped

- **Tag:** `v1.0.2` annotated tag on `origin/master`, target commit `ab6da3fa1e967f347750529718ed0debb1da70b7` (which contains all 4 Phase 9 fix merges).
- **GitHub Release:** `https://github.com/boejowen/SquireBot/releases/tag/v1.0.2`
- **Release assets (4):**
  - `SquireBot-Setup-1.0.2.exe` — versioned NSIS installer (per-user, no UAC)
  - `SquireBot-Setup.exe` — versionless byte-identical alias for the README permalink
  - `squirebot.exe` — bare binary consumed by the in-app auto-updater
  - `latest.json` — Phase 2 manifest with version=1.0.2, fresh installer_sha256 + binary_sha256
- **Workflow run:** `25770477750` (release.yml, completed in 2m2s at 2026-05-13T00:32:00Z)

## Pre-tag Readiness Sweep (Task 1 evidence)

### A. Working tree & branch state — PASS

- Branch: `master`
- HEAD: `ab6da3fa1e967f347750529718ed0debb1da70b7` (Plan 09-04 Wave 2 merge top)
- `git status --short`: `M .planning/ROADMAP.md` and `?? .claude/` reported by initial check
  - The `M` flag on ROADMAP.md was a stale git stat cache; `git update-index --refresh` confirmed `git hash-object .planning/ROADMAP.md` matches the index/HEAD blob exactly (`ca508b43bf2d8cb0af99018a9bfb7da7cbc69cb2`). No actual modification.
  - `.claude/` is the local agent worktree/lockfile directory; not tracked.
- Recent commits: 4 distinct Phase 9 fix merges visible (Plans 09-01..09-04 with their SUMMARY + source commits) plus the Wave 2 tracking-update commit at HEAD.

### B. Build + vet + test — PASS

- `go build ./...` → exit 0
- `go vet ./...` → exit 0
- `go test ./... -count=1` → exit 0 (16 packages OK, ~50s wall)

### C. Schema-impact verifier (D-06) — PASS

- `internal/sheet/client.go:44 — WatcherMaxSchemaVersion = 3` (1 match, exact).
- Apps-script `SCRIPT_MIN_SCHEMA_VERSION` constant: NOT PRESENT in `apps-script/src/lib/migrations.ts`. The plan grep is wrong; the schema is pinned via `writeMetaRow('_meta', 'schema_version', '3')` on line 97 of that file, which IS present. Schema correctly stays at 3. **No regression.**

### D. Fix-presence greps — PASS

- **OPS-06** (Plan 09-01): `pendingAction` struct at `internal/tray/tray.go:94`; `drainPending` at line 165; `t.ready = true` at lines 245 (OnReady) + 458 (simulateReady, 2 lines as expected).
- **OPS-07** (Plan 09-02): `cmd/squirebot/console_windows.go` and `cmd/squirebot/console_other.go` both exist; `main.go:109` has `_ = freeConsole()`. Note: Plan 09-02 implemented `FreeConsole` via `kernel32` LazySystemDLL (`procFreeConsole.Call()`) instead of the direct `windows.FreeConsole()` symbol — see 09-02 deviation note. The plan's literal grep for `windows.FreeConsole()` is therefore overly strict; the functional fix is verifiably present.
- **CONFIG-01** (Plan 09-03): `internal/config/config.go:65 — bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})`; `TestLoad_StripsUTF8BOM` at `internal/config/config_test.go:271`.
- **AUTH-07** (Plan 09-04): `applyBootAuthError` at `internal/app/runapp.go:650`; canonical status string at lines 657 + 681 (2 lines, parity between `suspendForAuth` and `applyBootAuthError` as expected); `TestApplyBootAuthError_Revoked` at `internal/app/runapp_test.go:238`.

### E. Release workflow integrity — PASS

- `.github/workflows/release.yml:41-42` has `on.push.tags: ['v*']`.
- ldflag injection at line 166: `-X main.Version=${{ steps.ver.outputs.version }}` (confirmed).
- `latest.json` generation step (lines 243-266) intact.

### F. v1.0.* tag baseline — PASS

- Local: `v1.0.0` and `v1.0.1` present; `v1.0.2` absent.
- Remote (`git ls-remote --tags origin v1.0.2`): empty (no existing remote tag).

## Tag Push (Task 2 execution)

1. `git push origin master` → fast-forward `acb4c58..ab6da3f master -> master` (26 Phase 9 commits pushed).
2. `git tag -a v1.0.2 -m "v1.0.2 — Watcher Robustness Polish (AUTH-07 + OPS-06 + OPS-07 + CONFIG-01)"` — annotated tag object `3bfa49163205f2b2ed7d52568751b525ff264a65` pointing at master HEAD `ab6da3f`.
3. `git push origin v1.0.2` → `[new tag] v1.0.2 -> v1.0.2`.

## Release Pipeline Outcome

**Workflow run:** `25770477750` (release.yml, tag-trigger).
**Duration:** 2m2s (identical to v1.0.1 ship).
**Status:** ✓ success — all 17 steps green.

Step-by-step ✓:
- Set up job
- actions/checkout@v6
- actions/setup-go@v6
- Materialise oauth-config.json from secret
- Install NSIS (>= 3.10)
- Verify NSIS version
- Compute version (extracted `1.0.2` from `refs/tags/v1.0.2`; NSIS numeric_version `1.0.2.0`)
- **Load OAuth constants (AUTH-03 PRODUCTION consent_screen gate)** ✓ — confirms the OAuth client is still in Production mode, refresh tokens won't silently expire (Pitfall #1 still mitigated)
- Build bare squirebot.exe (with `-X main.Version=1.0.2` ldflag)
- Build NSIS installer (`SquireBot-Setup-1.0.2.exe`)
- Stage versionless installer alias (`SquireBot-Setup.exe` byte-copy)
- Compute SHA-256 sums (installer + bare binary)
- Write latest.json (Phase 2 manifest with both URLs)
- Upload build artifacts
- Create GitHub Release (softprops/action-gh-release@v3)
- Post-job cleanup

One annotation (informational): `windows-2025` runner image redirect notice — non-blocking, unrelated.

## latest.json Verification

Downloaded via `gh release download v1.0.2 -p latest.json`. Contents:

```json
{
  "installer_sha256": "b54adcf379fa48cb2f1ee2ce2160e1715ab98d1e5e391fc3b623422b1b63f8f4",
  "released_at": "2026-05-13T00:31:52.1257485Z",
  "installer_url": "https://github.com/boejowen/SquireBot/releases/download/v1.0.2/SquireBot-Setup-1.0.2.exe",
  "signed": false,
  "phase": 2,
  "binary_sha256": "9dfb803cbec0b6629186eb633b2297d497bdaef22073a81ea0139a1b72f69edd",
  "binary_url": "https://github.com/boejowen/SquireBot/releases/download/v1.0.2/squirebot.exe",
  "version": "1.0.2"
}
```

- `version: "1.0.2"` ✓ (ldflag injection worked end-to-end)
- `installer_sha256` differs from v1.0.1 (`38a447bf...`) → fresh build ✓
- `binary_sha256` differs from v1.0.1 (`4707c92d...`) → fresh build ✓
- URLs correctly point at `releases/download/v1.0.2/...` ✓
- `signed: false` — D-13 still in force (SignPath OSS not yet approved); SmartScreen walkthrough remains the documented install path.
- `phase: 2` — manifest schema is the Phase 2 manifest, unchanged.

## Asset Inventory

| Asset                          | Purpose                                                                                |
| ------------------------------ | -------------------------------------------------------------------------------------- |
| `SquireBot-Setup-1.0.2.exe`    | NSIS installer (per-user, no UAC). The canonical end-user download.                    |
| `SquireBot-Setup.exe`          | Versionless byte-identical alias for the README permalink (`releases/latest/download`) |
| `squirebot.exe`                | Bare binary consumed by the in-app auto-updater via `binary_url` in latest.json        |
| `latest.json`                  | Phase 2 update manifest with version + fresh SHA-256s + download URLs                  |

**Asset-count reconciliation:** The plan's `must_haves.truths` specified "exactly three" assets. Actual ship is 4, matching the v1.0.1 ship pattern. The 4th asset is the versionless `SquireBot-Setup.exe` byte-copy of the versioned installer — intentionally added by `release.yml` "Stage versionless installer alias" step (lines 180-192) so the README's `releases/latest/download/SquireBot-Setup.exe` permalink resolves on every non-prerelease tag without a doc update. Both installer files are byte-identical; the manifest hashes the versioned file. This is established release shape, not a deviation.

## Schema Impact: NONE

`WatcherMaxSchemaVersion = 3` unchanged in `internal/sheet/client.go:44`. Migrations file still writes `_meta.schema_version = 3`. No new `_meta` rows, no new tab columns, no apps-script source edits in Phase 9. Existing v1.0.1 workbooks remain compatible with v1.0.2 watchers; the auto-update path swaps in the new binary without any sheet-side action.

## Deviations from Plan

### Plan-text vs. reality drift (informational — no executor action needed)

1. **`SCRIPT_MIN_SCHEMA_VERSION.*3` grep in Task 1.C** — the constant doesn't exist in `apps-script/src/lib/migrations.ts`. The plan author conflated it with the watcher-side `WatcherMaxSchemaVersion`. Schema pin is correctly enforced via `writeMetaRow('_meta', 'schema_version', '3')` (line 97), which IS present. No code change needed; future plans should drop the SCRIPT_MIN_SCHEMA_VERSION grep or replace it with the writeMetaRow check.

2. **`windows.FreeConsole()` literal grep in Task 1.D (OPS-07)** — Plan 09-02 implemented FreeConsole via `kernel32 LazySystemDLL`'s `procFreeConsole.Call()` instead of the direct symbol. This is documented as a deviation in 09-02-SUMMARY.md. The OPS-07 fix is functionally present and exercised by `_ = freeConsole()` at `cmd/squirebot/main.go:109`.

3. **"Exactly three" assets in `must_haves.truths`** — actual ship is 4 (matches v1.0.1). The 4th (versionless installer alias) is established `release.yml` behavior. No remediation; future plan templates copied from Plan 06-05 should be updated to say "four (the canonical installer, the versionless alias, the bare binary, and the manifest)".

None of the above are correctness regressions; all three are stale expectations in the plan's verbatim grep gates. All four Phase 9 fixes are demonstrably present in master HEAD and shipped in the v1.0.2 binary.

### Auto-fix actions in this plan

None. This plan made zero source edits; the only file written is this SUMMARY.

## Out of Scope (by prompt directive)

The orchestrator owns the following work after this agent returns. The plan's Tasks 3 and 4 are NOT executed by this agent:

- **Task 3 — End-to-end UAT on Win11 VM (AUTH-07 + OPS-06 + CONFIG-01 + OPS-07 + auto-update v1.0.1 → v1.0.2 + regression sweep).** User follow-up, requires a Win11 host the agent does not control.
- **Task 4 — ROADMAP.md update marking Phase 9 shipped.** Prompt explicitly forbids ROADMAP modifications by this agent ("the orchestrator owns those writes after you return").
- **STATE.md update.** Same reason — orchestrator-owned.
- **Milestone closure** (`/gsd-complete-milestone`, REQUIREMENTS.md retirement, `.planning/milestones/v1.0.2-ROADMAP.md` archive). Per CONTEXT.md D-08, milestone closes as a unit AFTER Phase 10 ships.

## Hand-off

Phase 9 binary work is shipped as v1.0.2. The remaining v1.0.2 milestone work is **Phase 10 (Apps Script Test Quality — TEST-03 + TEST-04)**. After Phase 10 ships, the user runs `/gsd-complete-milestone` to archive REQUIREMENTS.md, mint `.planning/milestones/v1.0.2-ROADMAP.md`, and append the milestone closure record.

UAT for the four fixes (per plan Task 3) is the user's responsibility on their Win11 VM; results should be captured in a follow-up note or appended to this SUMMARY by the orchestrator.

## Self-Check: PASSED

- `.planning/phases/09-watcher-robustness-polish/09-05-SUMMARY.md` — FOUND (this file)
- Tag `v1.0.2` exists locally — FOUND (`3bfa49163205f2b2ed7d52568751b525ff264a65` → `ab6da3f`)
- Tag `v1.0.2` exists on origin — FOUND (push succeeded with `[new tag] v1.0.2 -> v1.0.2`)
- GitHub Release v1.0.2 exists with 4 assets — FOUND (`gh release view v1.0.2` enumerates all four)
- latest.json contains `"version": "1.0.2"` — FOUND
- Workflow run 25770477750 completed success — FOUND (`gh run watch` exited 0 after 2m2s)
