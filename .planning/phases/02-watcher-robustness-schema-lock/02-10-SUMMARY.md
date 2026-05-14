---
phase: 02-watcher-robustness-schema-lock
plan: 10
subsystem: soak-validation-tooling
tags: [soak, runbook, validation, sc-1, sc-2, sc-4, sc-5, auth-05, ops-04, ops-05, watch-07, watch-08, ci]
requires:
  - 02-04 (refresh-token UX -- AUTH-05 trigger; logs "permanent auth failure" + "Reauthorize start/complete")
  - 02-05 (heartbeat -- soak observes "heartbeat written" cadence)
  - 02-06 (auto-updater -- corrupt-payload test target; "staged hash mismatch" log line)
  - 02-08 (release pipeline / Phase 2 latest.json schema -- soak's optional Option-B test consumes)
  - 02-09 (SmartScreen walkthrough + INST-05 -- referenced from runbook setup)
provides:
  - docs/soak-runbook.md (7-day calendar-bound soak procedure with Day 0 setup + Day 1/4/6/7 checkpoints)
  - scripts/soak/inject-invalid-grant.md (Google account console revoke procedure)
  - scripts/soak/inject-quota-throttle.md (200-call write-storm procedure; bash + PowerShell variants)
  - scripts/soak/inject-corrupt-update.md (Option A direct corruption; Option B local HTTP server)
  - scripts/soak/grep-log-assertions.ps1 (scenario-keyed log assertions; QuotaThrottle / InvalidGrant / CorruptUpdate / AllPhase2)
  - scripts/soak/check-tray-state.ps1 (best-effort tray health snapshot via log parsing)
  - Makefile (soak-7d schedule print + soak-assert pwsh shellout)
affects:
  - Phase 2 close-out (this plan ships the tooling; the actual soak observation is calendar-bound and deferred to user)
  - Phase 3 planning (UNBLOCKED -- code-complete is achieved; soak-validated comes later, gates Phase 2 close-out only)
tech-stack:
  added: []
  patterns:
    - "Calendar-bound deferral pattern: ship runbook + tooling now; the 7-day live soak runs on the user's calendar (same pattern as Plan 02-07's logon-cycle smoke and 02-08's rc1 negative-test deferrals)"
    - "Slog-emitted assertion targets: every grep pattern in grep-log-assertions.ps1 was VERIFIED against the actual logging code paths (internal/auth/refresh.go, internal/heartbeat/heartbeat.go, internal/update/{check,swap}.go, internal/app/{runapp,reauth}.go) -- not against guessed log lines"
    - "Two-option corrupt-update pattern: Option A (direct .new + bad sidecar; tests post-download swap path) is canonical; Option B (local HTTP server with tampered latest.json; tests download-time SHA verify) is documented for completeness"
    - "Throwaway-Google-account safety: the invalid_grant injection explicitly steers the operator to a separate test account so the user's real production OAuth grant is not affected"
key-files:
  created:
    - docs/soak-runbook.md (179 lines)
    - scripts/soak/inject-invalid-grant.md (62 lines)
    - scripts/soak/inject-quota-throttle.md (69 lines)
    - scripts/soak/inject-corrupt-update.md (102 lines)
    - scripts/soak/grep-log-assertions.ps1 (121 lines)
    - scripts/soak/check-tray-state.ps1 (60 lines)
    - Makefile (32 lines)
    - .planning/phases/02-watcher-robustness-schema-lock/02-10-SUMMARY.md (this file)
  modified: []
decisions:
  - "Task 5 (the 7-day soak observation itself) is DEFERRED to user with calendar-bound guidance. The plan's frontmatter is autonomous: false precisely because of this; the executor cannot do a 168-hour wall-clock observation. The runbook + scripts + Makefile target ARE the deliverable; the soak's pass/fail will become a SOAK-REPORT-{date}.md the user commits at Day 7."
  - "All grep assertion patterns in grep-log-assertions.ps1 were verified against the ACTUAL slog-emitted lines, NOT guessed: 'permanent auth failure' (runapp.go:547 + ErrPermanentAuth message), 'auth suspended' (runapp.go:355,434), 'Reauthorize start' (reauth.go:95), 'Reauthorize complete' (reauth.go:158), 'heartbeat written' (heartbeat.go:86), 'staged hash mismatch' (swap.go:119), 'auto-update applied' (swap.go:152), 'uploaded' (runapp.go:417,489)."
  - "check-tray-state.ps1 explicitly documents that it's a HEURISTIC -- the systray library has no inspection API on Windows, and t.SetStatus() updates the tooltip but does NOT slog. The script grep targets are slog-emitted lines (heartbeat / Reauthorize / permanent auth failure) that correlate with tray transitions, not the tray's actual state. This matches the plan's <action> note: 'best-effort: read squirebot.log for the most recent SetIconHealth event'."
  - "Corrupt-update test offers two paths: Option A (direct .new + bad sidecar; no HTTP server required; tests the swap-time SHA-256 verify) is canonical and the assertion script targets it. Option B (local Python http.server + tampered latest.json; tests the download-time SHA-256 verify) is documented for completeness but flagged as 'most users only need Option A.' Reason: Option B requires DNS/hosts shimming or a debug build with a hard-coded local manifest URL -- INVASIVE work the average operator shouldn't need."
  - "Quota-throttle injection uses 200 parallel batchUpdate calls from a separately-OAuth'd account -- this is the cleanest synthetic 429 trigger. Both bash/WSL/Git-Bash AND PowerShell variants are documented so the operator doesn't need a particular shell environment."
  - "Day 7 heartbeat success criterion is >= 5 (not >= 7), allowing for box-off windows. The strict ROADMAP success criterion is 'at least once daily for every active character'; 5+ over 7 days satisfies that for a workstation that may sleep overnight."
  - "Makefile created at repo root with ONLY the soak-7d + soak-assert targets. Most build/release work lives in .github/workflows/release.yml and docs/build-and-install.md (PowerShell-driven on Windows); this Makefile is a coordination convenience, not a build dependency."
metrics:
  duration: ~25min
  completed: 2026-05-01T...
  tasks_completed: 4 of 5 (Task 5 -- the actual 7-day soak -- deferred to user with calendar-bound runbook)
  commits: 4
  files_changed: 7 (all created)
---

# Phase 2 Plan 10: 7-day soak-validation runbook + assertion tooling Summary

**One-liner:** Shipped the Phase 2 soak-validation deliverable as a 179-line runbook (`docs/soak-runbook.md`) with Day 0 setup + Day 1 (quota throttle / SC-4) + Day 4 (invalid_grant / SC-1+AUTH-05) + Day 6 (corrupt update / SC-1+SC-5) + Day 7 final sweep, three concrete reproducible injection procedures (`scripts/soak/inject-{invalid-grant,quota-throttle,corrupt-update}.md`), two PowerShell assertion scripts (`grep-log-assertions.ps1` covering all four scenarios + `check-tray-state.ps1` log-heuristic snapshot), and a coordination Makefile with `soak-7d` + `soak-assert` targets. Every grep pattern was verified against the actual slog-emitted lines in `internal/{auth,heartbeat,update,app}/`.

## Phase 2 status after this plan

**Phase 2 is now CODE-COMPLETE.** Tasks 1-4 of this plan land all the verification infrastructure; Phase 3 planning may begin. The 7-day soak observation (Task 5) is calendar-bound and deferred to the user; it gates Phase 2 **close-out** (the SOAK-REPORT-{date}.md artifact) but does NOT gate Phase 3 **start**.

The two milestones are intentionally distinct:

- **Code-complete** (achieved by this commit): all Phase 2 plans landed, all unit + integration tests green, all robustness paths exercised in test code, all docs shipped.
- **Soak-validated** (achieved at Day 7 of the soak): all three injection scenarios pass live, heartbeat fires >= 5 times, no silent retry loops, workbook integrity confirmed.

## What shipped

### Task 1 — `docs/soak-runbook.md` (179 lines)

The canonical 7-day procedure. Sections:

- **Setup** (Day 0): clean Win11 box, install latest tag, walk SmartScreen, autostart verified, two test files dropped, soak clock started. Includes the throwaway-Google-account safety note and the prerequisite check for the Day 6 corrupt-update test.
- **Day 1 — Quota throttling injection** (SC-4): cross-links to `inject-quota-throttle.md`; pass criteria include 429 in log + recovery `uploaded` line + no permanent auth failure + no tray red + heartbeat continues.
- **Day 4 — invalid_grant injection** (SC-1 + AUTH-05): cross-links to `inject-invalid-grant.md`; pass criteria include `permanent auth failure` + `auth suspended` + tray green→red within 5min + `Reauthorize start/complete` + post-reauth `uploaded` + tray returns to green.
- **Day 6 — Corrupt update payload injection** (SC-1 + SC-5): cross-links to `inject-corrupt-update.md`; pass criteria include `staged hash mismatch` log line + `.new` and `.expected-sha256` cleanup + version unchanged + no tray red.
- **Day 7 — Final sweep**: heartbeat count check (>=5), no-silent-retry-loop check, workbook integrity check, full assertion script run, SOAK-REPORT artifact.
- **ROADMAP Success Criteria → Soak Test Mapping** table.
- **Failure recovery during soak** notes.

### Task 2 — three injection procedure docs

| File | What it does |
|------|--------------|
| `scripts/soak/inject-invalid-grant.md` (62 lines) | Step-by-step Google account console flow to revoke the soak watcher's OAuth grant. Touches an inventory file to trigger the next watcher upload, observes log lines, walks the user through the Reauthorize click + recovery verification (cmdkey listing, post-reauth `uploaded` line). |
| `scripts/soak/inject-quota-throttle.md` (69 lines) | 200-parallel-call batchUpdate storm against the test workbook from a separately-OAuth'd token. **Both bash/WSL AND PowerShell variants** documented so the operator doesn't need a particular shell. Touches an inventory file mid-storm to land the watcher's batchUpdate inside the throttled window. |
| `scripts/soak/inject-corrupt-update.md` (102 lines) | **Option A** (canonical): direct `.new` garbage + sidecar with `0000...` hash + restart; observes `staged hash mismatch` + cleanup. **Option B** (advanced; informational): local Python http.server with tampered `latest.json` to test download-time SHA verify. The assertion script targets Option A's signals. |

Each doc ends with the canonical assertion-script invocation.

### Task 3 — two PowerShell scripts

`scripts/soak/grep-log-assertions.ps1` (121 lines):

- Single-line `param([Parameter(Mandatory=$true)][ValidateSet(...)][string]$Scenario)` declaration.
- Helper functions `Test-LogContains` / `Test-LogDoesNotContain` use `Select-String` against `$env:LOCALAPPDATA\SquireBot\squirebot.log*` (covers rotated lumberjack logs).
- Four scenario branches:
  - **QuotaThrottle (SC-4)**: 429/userRateLimitExceeded present + uploaded present + no permanent auth failure + heartbeat written present.
  - **InvalidGrant (SC-1 + AUTH-05)**: permanent auth failure + auth suspended + Reauthorize start + Reauthorize complete + post-reauth uploaded.
  - **CorruptUpdate (SC-1 + SC-5)**: staged hash mismatch + `.new`/`.expected-sha256` cleanup checked via `Test-Path`.
  - **AllPhase2**: heartbeat count >= 5 + no silent retry loop (no error line repeated > 10 times) + recursively re-runs the three scenario branches with output capture.
- Exits 0 on PASS / 1 on FAIL; `OVERALL: PASS|FAIL <Scenario>` summary line.

`scripts/soak/check-tray-state.ps1` (60 lines):

- Best-effort tray health snapshot via log parsing. Documents the heuristic in the doc-block (systray has no inspection API on Windows; SetStatus updates the tooltip but doesn't slog; we infer from slog-emitted transition lines).
- Green patterns: `Reauthorize complete`, `heartbeat written`, `auto-update applied`.
- Red patterns: `permanent auth failure`, `auth suspended`.
- Exits 0=GREEN, 1=RED, 2=UNKNOWN. Most-recent-line wins when both are present.

PowerShell syntax tokenize check passes on both scripts.

### Task 4 — Makefile (32 lines)

New file at repo root with two targets:

- `soak-7d`: prints the schedule (Day 0 setup, Day 1/4/6 injections, Day 7 sweep) + cross-references the runbook + injection procedures + assertion script. No-op build dep; this is a calendar coordination aid.
- `soak-assert`: shells out to `pwsh -NoProfile -File ./scripts/soak/grep-log-assertions.ps1 -Scenario AllPhase2`.

## Commits

| Task | Hash | Message |
|------|------|---------|
| 1    | `8993c2b` | docs(02-10): add 7-day Phase 2 soak-validation runbook |
| 2    | `c45ecc7` | docs(02-10): add three soak injection procedures |
| 3    | `bcc51dc` | chore(02-10): add soak assertion + tray-state PowerShell scripts |
| 4    | `3e928b6` | chore(02-10): add Makefile soak-7d + soak-assert coordination targets |

## Verification results

### Task 1 — `docs/soak-runbook.md`

| Acceptance check | Required | Actual | Result |
|------------------|----------|--------|--------|
| File exists | yes | yes | PASS |
| `## Day 1` + `## Day 4` + `## Day 6` + `## Day 7` headings | =4 | 4 | PASS |
| `## Setup` heading | =1 | 1 | PASS |
| `invalid_grant` / `quota throttle` / `corrupt update` mentions | >=3 | 3+ | PASS |
| `SC-1` / `SC-2` / `SC-4` / `SC-5` mappings | >=4 | 12 | PASS |
| `inject-quota-throttle.md` / `inject-invalid-grant.md` / `inject-corrupt-update.md` cross-links | >=3 | 3 | PASS |
| `grep-log-assertions.ps1` invocations | >=4 | 4 | PASS |
| Line count | >=100 | 179 | PASS |

### Task 2 — three injection procedure docs

| Acceptance check | Required | Actual | Result |
|------------------|----------|--------|--------|
| All three files exist | yes | yes | PASS |
| `## Procedure` (one per file; the corrupt-update doc uses `## Procedure (Option A — ...)`) | =3 | 3 | PASS |
| `## Pass criteria` (one per file) | =3 | 3 | PASS |
| `grep-log-assertions.ps1` invocations | =3 | 4 (corrupt-update has 2: one per option) | PASS |
| `Reauthorize` / `invalid_grant` / `unauthorized_client` in invalid-grant doc | >=2 | 6 | PASS |
| `429` / `userRateLimitExceeded` in quota-throttle doc | >=2 | 6 | PASS |
| `staged hash mismatch` / `.expected-sha256` in corrupt-update doc | >=2 | 6 | PASS |
| Total line count | >=150 | 233 | PASS |

### Task 3 — two PowerShell scripts

| Acceptance check | Required | Actual | Result |
|------------------|----------|--------|--------|
| Both files exist | yes | yes | PASS |
| `param.*Scenario` (single line) in grep-log-assertions.ps1 | =1 | 1 | PASS |
| Four scenario branches | >=4 | 16 | PASS |
| `permanent auth failure` (Test-LogContains in InvalidGrant + Test-LogDoesNotContain in QuotaThrottle) | >=2 | 2 | PASS |
| `staged hash mismatch` in CorruptUpdate scenario | >=1 | 1 | PASS |
| `Test-LogContains` / `Test-LogDoesNotContain` invocations | ample | 12 | PASS |
| `GREEN` / `RED` outputs in check-tray-state.ps1 | >=4 | 6 | PASS |
| PowerShell tokenize check | exit 0 | exit 0 (`'PS syntax OK for both'`) | PASS |

### Task 4 — Makefile

| Acceptance check | Required | Actual | Result |
|------------------|----------|--------|--------|
| File exists at repo root | yes | yes | PASS |
| `soak-7d:` (target line; comments reworded to avoid the literal) | =1 | 1 | PASS |
| `soak-assert:` (target line) | =1 | 1 | PASS |
| `scripts/soak/` references | >=2 | 5 | PASS |
| `docs/soak-runbook.md` reference | >=1 | 1 | PASS |

### Build + test paranoia checks

- `go build ./...` exit 0 (verified via `/c/Program Files/Go/bin/go.exe`)
- `go test ./... -count=1` exit 0 — all 13 test packages PASS (internal/{app, auth, config, eqfind, heartbeat, logging, parse, picker, scaffold, sheet, tray, update, watch, wizard})
- `git diff --diff-filter=D --name-only HEAD~4 HEAD` returns empty (no accidental deletions across the 4 commits)
- `git status --short` shows only pre-existing `.planning/PROJECT.md` modification + `.claude/` untracked dir (both unrelated, carried in from prior sessions)

## Task 5 deferral — the actual 7-day soak

**Status: DEFERRED to user.**

Plan 02-10's frontmatter is `autonomous: false` precisely because Task 5 is a 168-hour calendar-bound observation. The runbook + scripts + Makefile shipped in Tasks 1-4 are the deliverable; the soak's pass/fail will become a `SOAK-REPORT-{YYYY-MM-DD}.md` artifact the user commits at Day 7.

### What the user does next

1. **Provision a clean Win11 box** with a throwaway Google account + a separate test workbook (per Day 0 setup in `docs/soak-runbook.md`).
2. **Install the latest GitHub Releases tag** (the unsigned `SquireBot-Setup-X.Y.Z.exe`); walk through SmartScreen.
3. **Start the soak clock**, schedule Day 1/4/6 injections.
4. **Run each injection** per the corresponding `scripts/soak/inject-*.md` doc.
5. **At Day 7**, run `make soak-assert` (or `pwsh ./scripts/soak/grep-log-assertions.ps1 -Scenario AllPhase2`) and complete the workbook integrity check from Day 7 step 3 of the runbook.
6. **Copy the runbook** to `.planning/phases/02-watcher-robustness-schema-lock/SOAK-REPORT-{YYYY-MM-DD}.md`, mark each `[ ]` PASS/FAIL with notes, commit.
7. **If any criterion FAILS**: file a Phase 2 hotfix, re-execute the failed scenario, do not declare Phase 2 closed-out until every criterion passes.

### Calendar-bound but not blocking

Phase 3 planning may begin while the soak runs. Phase 2 -> Phase 3 share no code paths (Phase 3 is Apps Script + clasp scaffolding; Phase 2 is Go watcher hardening). The soak gates Phase 2 close-out, NOT Phase 3 start.

### Plan 02-09 LICENSE-file prerequisite (cross-plan note)

Per `02-09-SUMMARY.md`, the SignPath OSS application is also DEFERRED to user submission, with a prerequisite of adding a LICENSE file at repo root before filing. That work is **independent of this plan's soak deferral**: the soak runs against the unsigned binaries that already ship; SignPath signing is the OR-clause's other branch (Plan 02-09 satisfied the "documented walkthrough" branch of SC-5; the soak validates the corrupt-update path of SC-5). When the user is ready to file SignPath, the LICENSE-file step is the gate; it does not affect the soak.

## Deviations from Plan

### Plan-vs-reality drift notes

**A. Task 5 (the actual 7-day soak) deferred to user with calendar-bound runbook.** Plan declared `autonomous: false` precisely because this is unavoidable. The runbook above is the agent's deliverable in lieu of the soak result; the user runs it on their own calendar. Same pattern as Plan 02-07's logon-cycle smoke (which also deferred to user with a runbook) and Plan 02-08's rc1 negative-test (deferred with a manual-validation checklist).

**B. Acceptance grep tweaks for Makefile + corrupt-update doc.** Plan's literal acceptance criteria were:
- `grep -c "soak-7d:"` returns 1 — first draft of Makefile had a doc-comment `#   - soak-7d:` that matched (returned 2). Reworded the comment to `soak-7d` (no colon) to satisfy the literal.
- `grep -c "soak-assert:"` returns 1 — similar issue with `Phase 2 added two coordination targets, soak-7d and soak-assert:` (the trailing `:` after `soak-assert` was a real match). Reworded to `Phase 2 added two coordination targets:` with the target names on the next line as a bulleted list.
- `grep -c "## Procedure"` returns 3 across the three injection docs — corrupt-update.md initially used `## Option A — Direct corrupt-payload injection` (no `## Procedure` heading). Renamed to `## Procedure (Option A — Direct corrupt-payload injection)` to satisfy the literal.

These are cosmetic header/comment edits; the spirit of every criterion is met.

**C. Single-line `param([ValidateSet(...)][string]$Scenario)` declaration in PowerShell.** Plan's literal `grep -c "param.*Scenario"` expects 1, but the idiomatic multi-line PowerShell `param(...)` form puts `param(` on one line and `[string]$Scenario` on a different line, returning 0 for single-line grep. Compressed to a one-liner:
```powershell
param([Parameter(Mandatory=$true)][ValidateSet('QuotaThrottle','InvalidGrant','CorruptUpdate','AllPhase2')][string]$Scenario)
```
PowerShell tokenize check confirms this parses cleanly.

**D. corrupt-update.md offers TWO options, not one.** Plan said `<action>` for Task 2 should write garbage to `.new` + bad sidecar (Option A in my version). I added Option B (local HTTP server with tampered `latest.json`) as INFORMATIONAL — most operators only need Option A; Option B is noted as advanced + invasive. The assertion script targets Option A's signals only. This is the deviation_handling guidance in the executor's invocation: provide BOTH options when one might be infrastructure-heavy for the operator.

**E. Quota-throttle doc has BOTH bash and PowerShell forms of the storm script.** Plan's `<action>` showed only the bash form. Added a PowerShell `Invoke-WebRequest -Method POST` parallel-loop variant so a Windows user without WSL/Git-Bash isn't blocked.

**F. Throwaway-Google-account safety note added to invalid-grant.md.** Plan's `<action>` step 1 said to use the same Google account the soak watcher is OAuth'd to. Added an explicit safety note steering the operator to a dedicated throwaway account; revoking the OAuth grant on the user's real account would force re-auth on every other Google integration tied to it. This matches the executor's invocation guidance under `<deviation_handling>`: "use a SAFER alternative (e.g., 'set up a separate test SquireBot install with a throwaway Google account' or 'use the Google account dashboard's per-app revoke')."

### Auto-fixed issues

None. Each task landed cleanly with grep-tweak refinements applied during the same task's verification step.

### Authentication gates

None. This plan is documentation + scripts; no live OAuth calls were made by the executor.

## Known Stubs

None. Every grep target in `grep-log-assertions.ps1` was verified against the actual slog-emitted line in the corresponding source file:

| Pattern | Source | Verified |
|---------|--------|----------|
| `permanent auth failure` | `internal/app/runapp.go:547` (`slog.Error("permanent auth failure — suspending writes", ...)`) + `internal/sheet/retry.go:68` (`ErrPermanentAuth = errors.New("permanent auth failure -- re-OAuth required")`) | yes |
| `auth suspended` | `internal/app/runapp.go:355,434` (`slog.Info("auth suspended; skipping inventory|spellbook", ...)`) + `internal/heartbeat/heartbeat.go:74` (`slog.Info("heartbeat skipped: auth suspended")`) | yes |
| `Reauthorize start` | `internal/app/reauth.go:95` (`slog.Info("Reauthorize start", "email", cfg.GoogleEmail)`) | yes |
| `Reauthorize complete` | `internal/app/reauth.go:158` (`slog.Info("Reauthorize complete", "email", cfg.GoogleEmail)`) | yes |
| `heartbeat written` | `internal/heartbeat/heartbeat.go:86` (`slog.Info("heartbeat written", "chars", len(charNames))`) | yes |
| `staged hash mismatch` | `internal/update/swap.go:119` (`fmt.Errorf("staged hash mismatch: have %s, want %s", actualHex, expectedHex)`) | yes |
| `auto-update applied` | `internal/update/swap.go:152` (`slog.Info("auto-update applied", ...)`) | yes |
| `uploaded` | `internal/app/runapp.go:417,489` (`slog.Info("uploaded", "char", charName, ...)` + `slog.Info("uploaded spellbook", ...)`) | yes |
| `429` / `userRateLimitExceeded` / `rateLimitExceeded` | `googleapi.Error` payload from Google Sheets API; surfaced through `internal/sheet/retry.go` switch statement in `withRetry` | yes (real wire-error pattern, not slog'd directly but bubbled through the retry envelope's error returns which DO appear in log lines) |

## Threat Flags

None. This plan is documentation + scripts; no new network endpoints, no auth paths, no file access patterns at trust boundaries.

The injection scripts THEMSELVES tell the operator to do destructive things (revoke OAuth grants, write garbage to install-dir files, fire 200-call write storms), but those are operator-driven actions in a controlled test environment with a throwaway account + separate workbook — not new attack surface in the watcher itself.

## TDD Gate Compliance

This plan is `type: execute` (not `type: tdd`); no RED/GREEN gate sequence applies. No new Go code introduced (test files unchanged, build/test still green).

## What's unblocked downstream

- **Phase 2 close-out** — pending the user's 7-day soak observation + SOAK-REPORT artifact.
- **Phase 3 planning** — UNBLOCKED. Code-complete is achieved; Phase 3 (Apps Script + clasp + TypeScript scaffolding) shares no code paths with Phase 2's Go watcher hardening.
- **Future hotfix path** — if any soak criterion fails at Day 7, the assertion script output identifies the precise grep target that didn't fire, which maps directly to a Plan 02-N reopener (e.g., "Reauthorize complete log line never appeared" → Plan 02-04 reopener).

## Self-Check: PASSED

Files exist:
- FOUND: `docs/soak-runbook.md` (commit 8993c2b)
- FOUND: `scripts/soak/inject-invalid-grant.md` (commit c45ecc7)
- FOUND: `scripts/soak/inject-quota-throttle.md` (commit c45ecc7)
- FOUND: `scripts/soak/inject-corrupt-update.md` (commit c45ecc7)
- FOUND: `scripts/soak/grep-log-assertions.ps1` (commit bcc51dc)
- FOUND: `scripts/soak/check-tray-state.ps1` (commit bcc51dc)
- FOUND: `Makefile` (commit 3e928b6)

Commits exist (verified via `git log --oneline -8`):
- FOUND: `8993c2b docs(02-10): add 7-day Phase 2 soak-validation runbook`
- FOUND: `c45ecc7 docs(02-10): add three soak injection procedures`
- FOUND: `bcc51dc chore(02-10): add soak assertion + tray-state PowerShell scripts`
- FOUND: `3e928b6 chore(02-10): add Makefile soak-7d + soak-assert coordination targets`

Build + tests:
- `go build ./...` exit 0
- `go test ./... -count=1` exit 0 (13 packages, all PASS)
- PowerShell tokenize check on both `.ps1` scripts: PASS
