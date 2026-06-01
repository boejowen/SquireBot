---
phase: 16-cutover-decommission
plan: 04
subsystem: decommission-ops
tags: [decommission, apps-script, oauth, google, milestone-close, ops, human-action]

# Dependency graph
requires:
  - phase: 16-cutover-decommission
    provides: "16-03 flip (guild reporting in on the backend) — the gate before tearing down Google"
provides:
  - "no LIVE Google machinery remains: Apps Script triggers deleted + OAuth 2.0 client deleted"
  - "docs/decommission-checklist.md — the committed CUTOVER-04 proof artifact (all boxes ticked)"
affects: [milestone-close-v2.0]

key-files:
  created: []
  modified:
    - "docs/decommission-checklist.md — drafted (16-04 Task 1) then completed: reporting-in gate + both retirements ticked + post-teardown sweep recorded"

key-decisions:
  - "Decommission executed at 3/11 guildies reporting in — maintainer-accepted as representative (D-05, no hard %); strands no one (Google dead since 2026-05-15)"
  - "Sheet abandoned in place (D-12) — no export/delete/freeze. Apps Script triggers DELETED (all 10, not 7 — the installTriggers.ts comment undercounts). OAuth client DELETED (restorable 30 days, no consumer)"

requirements-completed: [CUTOVER-01, CUTOVER-04]

# Metrics
completed: 2026-05-31
---

# Phase 16 Plan 04: Decommission Google + Milestone Close Summary

**Retired the two live Google assets (Apps Script triggers + OAuth 2.0 client) and recorded the CUTOVER-01 reporting-in confirmation — the operational endgame of v2.0 "Off Google."**

> **Reconciliation note:** 16-04 Task 1 (draft the checklist) was done by the orchestrator; Tasks 2–3 (the two Google-console retirements) were performed by the maintainer (their Google login); Task 4 (the post-teardown sweep) was verified by the orchestrator over curl/SSH. Not a `gsd-executor` agent.

## What was done
- **Task 1 — checklist (auto):** drafted `docs/decommission-checklist.md` (eviction-runbook style), correcting the trigger count from the stale `installTriggers.ts` header (7) to the actual **10**.
- **Task 2 — reporting-in + triggers (D-05/D-10):** confirmed **3/11 guildies reporting in** (5+ chars, 721+ items, 179+ spells — representative per D-05). Maintainer **deleted all 10 Apps Script triggers** (zero remain; no further scheduled enrichment runs).
- **Task 3 — OAuth client (D-11):** maintainer **deleted the v1.0.2 desktop OAuth 2.0 Client ID** (revokes outstanding tokens; restorable 30 days, no consumer).
- **Task 4 — final sweep:** orchestrator verified the live system is **unaffected** — backend `active`; `api.squirebot.quest` routes `401`; `squirebot.quest` serves `200`; guild still ingesting (3 guildies / 7 chars / 909 items, climbing). Sheet left untouched (D-12).

## Result
**CUTOVER-01 and CUTOVER-04 satisfied** → no live Google machinery and no Google dependency remain (code-level Google-freedom already proven in P13). The v2.0 **"Off Google"** goal is met.

## User Setup Required
None remaining for the goal. Ongoing: the other 8 guildies onboard whenever (installer + their guild code).

## Self-Check: PASSED
`docs/decommission-checklist.md` exists with all actionable boxes ticked + the sweep recorded. Live system verified healthy post-teardown (curl 401/200 + SSH `systemctl is-active`=active + data intact).

---
*Phase: 16-cutover-decommission*
*Completed: 2026-05-31*
