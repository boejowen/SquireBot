# Phase 5 Distribution Report

**Run date:** 2026-05-11
**Workbook:** RiverFAIL VanFRAUD (dev workbook)
**Run by:** boejowen@gmail.com
**Probe:** `_phase5DistributionProbe` against `_char_owner` (Apps Script editor, manual run)

## Counts

- Distinct active guildie emails (non-`is_removed`): **1**
- Distinct emails with `last_seen` in last 7 days: **1**

## Per-email heartbeat (sorted by last_seen desc)

| owner_email | last_seen (UTC) |
|---|---|
| boejowen@gmail.com | 2026-05-11T00:54:13.000Z |

## Notable gaps

Dev workbook only — no guildies have onboarded yet. The Phase 5 deliverables
(search sidebar, eviction sidebar, archive backend, onboarding docs at
`https://boejowen.github.io/SquireBot/`) are the prerequisites for guildie
rollout. Guild distribution is a v1.0.1 / post-ship activity, not a Phase 5
ship gate. The user explicitly authorized this scope during Phase 5
discuss (CONTEXT.md §Scope Changes 2026-05-11 — `12 guildies` relaxed to
`even a handful is fine`).

## TL;DR

Phase 5 code is functionally complete and validated end-to-end on the dev
workbook (DOC-02 fake-guildie eviction smoke PASS 2026-05-11, all 7
checkpoints green). The single-writer count is an honest reflection of
rollout state, not a deployment regression. Ship decision is the user's
call; this report supports proceeding to `phase-5-shipped` and tagging
milestone v1.0.
