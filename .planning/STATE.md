---
gsd_state_version: 1.0
milestone: v2.0
milestone_name: Off Google — Website Frontend
status: planning
last_updated: "2026-05-29T02:04:56.119Z"
last_activity: 2026-05-29
progress:
  total_phases: 0
  completed_phases: 0
  total_plans: 0
  completed_plans: 0
  percent: 0
---

# State: SquireBot

**Initialized:** 2026-04-30
**Last updated:** 2026-05-28 (v2.0 "Off Google" Website Frontend opened — Phases 11–16)

## Project Reference

See: `.planning/PROJECT.md` (updated 2026-05-28 with v2.0 milestone scope)

- **Core value:** Every guildie can answer "what does my character still need, and where in the guild is it?" without leaving the spreadsheet — now delivered via a self-hosted website instead of the Google Sheet.
- **Current focus:** Milestone v2.0 "Off Google" — replace the Google Sheet (UI + data store) with a self-hosted Go + SQLite backend (Oracle Cloud Always Free, ARM) + static web frontend (SvelteKit), eliminating the Google OAuth dependency. Phases 11–16. ≈ $12/yr (domain only).
- **Mode:** yolo
- **Granularity:** coarse

## Current Position

Phase: Not started (defining requirements)
Plan: —
Status: Defining requirements
Last activity: 2026-05-29 — Milestone v2.0 started

### Superseded: 999.19 Google brand verification

Google REJECTED brand verification 2026-05-15 ("home page not registered to you" — `github.io` can't satisfy it), walling off all ~12 guildie watchers. **v2.0 "Off Google" supersedes this** by removing Google entirely rather than renting a domain to placate it. The guild stays dark on the sheet during the v2.0 build (P11 ingest is front-loaded to restore data flow first). Full incident trail at `.planning/debug/v1-0-2-oauth-invalid-client-incident.md`. Anti-pattern note for future Google-OAuth incidents: check OAuth consent screen Verification status FIRST when seeing `Access blocked / Error 400: invalid_request`; the multiple-secrets / loopback-IP / redirect-URI angles are red herrings.

### v1.0.2 Phase Plan (2026-05-12)

| Phase | Name | Requirements | Stack | Ship gate | Status |
|-------|------|--------------|-------|-----------|--------|
| 9 | Watcher Robustness Polish | AUTH-07, OPS-06, OPS-07, CONFIG-01 | Go (watcher) | watcher v1.0.2 binary release | Not started |
| 10 | Apps Script Test Quality | TEST-03, TEST-04 | Apps Script | `clasp push` of apps-script bundle + green CI | Not started |

**Sequencing rationale:**

1. Phase 9 first because it produces the binary release that tags the milestone (`v1.0.2`). Mirrors v1.0.1's Phase 6 ship-gate pattern. Independent of apps-script work.
2. Phase 10 second because it's apps-script-only (no schema impact); naturally bundles with Phase 9 ship for a single coherent v1.0.2 milestone close.

**Schema impact:** NONE across both phases. `_meta.schema_version` stays at 3; `WatcherMaxSchemaVersion` stays at 3 (the watcher binary rebuild in Phase 9 is for new robustness fixes, not for schema reasons — same shape as v1.0.1 Phase 6).

**Shipped to date:**

| Milestone | Phases | Plans | Tag | Date |
|-----------|--------|-------|-----|------|
| v1.0 — Watcher + Workbook + Onboarding | 5 (Phases 1–5) | 31 | `v1.0.0` | 2026-05-11 |
| v1.0.1 — Installer + Permissions Hardening | 3 (Phases 6–8) | 12 | `v1.0.1` (binary; Phase 6 ship gate) | 2026-05-12 |

**Candidates for the next milestone (from `.planning/ROADMAP.md` § Backlog):**

- **v1.0.2 patch** — would bundle 999.13 (Reauthorize on boot-time `invalid_grant`) + 999.14 (tray `OnReady` queuing) + 999.15 (UTF-8 BOM strip in config loader) + 999.16 (console detach OR `Start-Process` documentation) + optionally 999.17 (Admin-Mgmt inline-JS tests) + 999.18 (Phase 8 test-quality findings) + 999.9 (SignPath OSS approval landing). 4 v1.0.2 candidates are Phase-6-UAT-surfaced and Go-side; 999.9 is third-party-gated.
- **v1.1 polish** — would bundle 999.1 (bank-coin permission lock) + 999.2 (theme picker tile UI) + 999.7 (inline SIDEBAR_BODY → external `.html`) + 999.11 (verification doctrine decision). Apps-script-side polish; no schema impact.
- **v2 — Wantlist + Discord pinger** — 999.12 / WANT-01..08. Prerequisites still open: Raid Alliance Discord bot invites (admin/owner permission in 3 servers, not yet negotiated), per-user Discord identity capture sidebar, curated `quest → raid-target NPC(s)` lookup.

## Performance Metrics

| Metric | Value |
|--------|-------|
| Total milestones shipped | 2 (v1.0, v1.0.1) |
| Total phases shipped | 8 (1–8) |
| Total plans shipped | 43 (31 + 12) |
| Total commits since init | 266 (203 v1.0 + 63 v1.0.1) |
| Watcher LOC (Go) | 14,507 |
| Apps-script LOC (TypeScript) | 13,266 |
| Vitest tests | 336/336 passing |
| Active blockers | 0 |

## Accumulated Context

### Decisions Log

All locked decisions live in `PROJECT.md` Key Decisions table and the per-milestone archive ROADMAPs. v1.0 contributed 11 PROJECT-level decisions; v1.0.1 contributed 5 more (no schema bump, owner-floor lockout, SEARCH-05 per-user scope, manifest `userinfo.email` requirement, TRIGGER_GLOBALS sync assertion is load-bearing).

### Open TODOs

- **(Day-10 token-survival check, fires automatically)** Routine `trig_01Uog2muQ22CBsjZfqPiSH9r` fires 2026-05-13T15:00:00Z to validate AUTH-03 / Pitfall #1 (refresh token survives the 7-day Testing-mode expiry boundary). If it succeeds, AUTH-03 is structurally validated permanently. If it fails, re-open consent-screen publishing investigation.
- **(SignPath OSS, separate track, 999.9)** SignPath Foundation OSS approval is in flight; lands as a hotfix (NOT a planned phase) when the Foundation review completes. Would retire INST-05 partial → full.
- **(Roadmap audit carry-over from v1.0)** Reconcile the "56 v1 requirements" figure mentioned historically with the actual literal count of 69 REQ-IDs from v1.0. Already noted in v1.0 milestone audit; not blocking.
- **(v1.0.2 candidates accumulating)** 6 entries in the backlog now (999.13–999.18). If 4+ become urgent, consider scoping a v1.0.2 patch milestone before the next minor.

### Active Blockers

None. Both milestones shipped clean.

## Session Continuity

### Last Session Summary

**2026-05-12 (v1.0.2 milestone OPENED via `/gsd-new-milestone`):** Scoped a patch milestone from the backlog. User selected all 4 Go-side v1.0.2 candidates (999.13 AUTH-07 boot-time Reauthorize, 999.14 OPS-06 tray controller queueing, 999.15 CONFIG-01 BOM strip, 999.16 OPS-07 console-detach OR docs) + both apps-script test-quality items (999.17 TEST-03 Admin-Mgmt inline-JS, 999.18 TEST-04 Phase 8 advisory fixes). SignPath OSS (999.9) excluded — treated as hotfix-when-approved per third-party-gated timeline. Schema impact: NONE (mirrors v1.0.1 shape). PROJECT.md updated with Current Milestone section + Active checkboxes; STATE.md reset to planning mode. REQUIREMENTS.md + ROADMAP.md still pending in this same workflow.

**2026-05-12 (v1.0.1 milestone CLOSED via `/gsd-complete-milestone`):** v1.0.1 archive landed. Created `.planning/milestones/v1.0.1-ROADMAP.md` (full phase details, decisions, deferred items) and `.planning/milestones/v1.0.1-REQUIREMENTS.md` (8/8 reconciliation, traceability table, v1.0.2 candidates surfaced during execution). ROADMAP.md collapsed v1.0.1 into milestone-archive form with Backlog preserved + 2 new entries (999.17 Admin-Mgmt inline-JS tests; 999.18 Phase 8 advisory test-quality findings). MILESTONES.md appended v1.0.1 entry with stats (3 phases, 12 plans, 63 commits, 2 days). PROJECT.md evolved: Current State now reflects v1.0.1 ship; v1.0.1 reqs moved into Validated section; 5 new Key Decisions appended (no schema bump, owner-floor lockout, SEARCH-05 per-user PropertiesService, manifest `userinfo.email` requirement, TRIGGER_GLOBALS sync assertion load-bearing); footer updated to 2026-05-12. RETROSPECTIVE.md appended v1.0.1 milestone section before the existing Cross-Milestone Trends section; trends tables updated (decision quality, phase velocity, test coverage trajectory, watcher + TS LOC trajectory, tags shipped); new "Milestone Sizing Heuristic" section added with patch-vs-minor bounds emerging from the 2 data points. STATE.md reset to between-milestones state (this file). REQUIREMENTS.md removed via `git rm` (fresh for next milestone). Audit skipped per yolo-mode close (phase verifier already PASSED 5/5 must-haves on Phase 8 + dev-workbook 5-hook smoke on Phase 7 provided sufficient signal). Binary tag `v1.0.1` already exists from Phase 6 ship; not re-tagged at milestone close.

### Files of Record

- `.planning/PROJECT.md` — core value, requirements (v1 + v1.0.1 Validated), constraints, 16 Key Decisions (updated 2026-05-12).
- `.planning/ROADMAP.md` — collapsed milestone view + Backlog (12 entries) (updated 2026-05-12).
- `.planning/MILESTONES.md` — historical record (v1.0 + v1.0.1).
- `.planning/RETROSPECTIVE.md` — living retrospective (v1.0 + v1.0.1 + Cross-Milestone Trends).
- `.planning/STATE.md` — this file.
- `.planning/milestones/v1.0-ROADMAP.md` — v1.0 archive.
- `.planning/milestones/v1.0-REQUIREMENTS.md` — v1.0 archive (69 REQ-ID reconciliation).
- `.planning/milestones/v1.0-MILESTONE-AUDIT.md` — v1.0 audit (tech-debt verdict).
- `.planning/milestones/v1.0.1-ROADMAP.md` — v1.0.1 archive (created 2026-05-12).
- `.planning/milestones/v1.0.1-REQUIREMENTS.md` — v1.0.1 archive (8 REQ-ID reconciliation, created 2026-05-12).
- `.planning/research/SUMMARY.md` — research synthesis (v1.0-era; still in force).
- `.planning/research/STACK.md` — locked stack decisions.
- `.planning/research/PITFALLS.md` — 27 pitfalls catalogue.
- `.planning/config.json` — granularity (coarse), mode (yolo), workflow toggles.

### Next Action

`/clear` then `/gsd-plan-phase 9` — Phase 9 context locked at `.planning/phases/09-watcher-robustness-polish/09-CONTEXT.md`. Planner reads CONTEXT.md + 8 decisions + canonical refs (Phase 6 UAT findings C/D/F/H, Phase 6 release plan 06-05 template, watcher source files for tray/main/runapp/config) and produces 5 PLANs.

---

*State initialized: 2026-04-30 after roadmap creation. v1.0 milestone COMPLETE: 2026-05-11. v1.0.1 milestone COMPLETE: 2026-05-12. v1.0.2 milestone OPENED: 2026-05-12.*
