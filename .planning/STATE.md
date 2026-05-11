---
gsd_state_version: 1.0
milestone: v1.0.1
milestone_name: Installer + Permissions Hardening
status: planning
last_updated: "2026-05-11T16:00:00.000Z"
last_activity: 2026-05-11
progress:
  total_phases: 3
  completed_phases: 0
  total_plans: 0
  completed_plans: 0
  percent: 0
---

# State: SquireBot

**Initialized:** 2026-04-30
**Last updated:** 2026-05-11 (v1.0.1 roadmap landed — 3 phases, 8 requirements, ship gate = watcher v1.0.1 binary release)

## Project Reference

See: `.planning/PROJECT.md` (updated 2026-05-11 after v1.0 milestone close + v1.0.1 milestone open)

- **Core value:** Every guildie can answer "what does my character still need, and where in the guild is it?" without leaving the spreadsheet.
- **Current focus:** Milestone v1.0.1 Installer + Permissions Hardening — retire v1.0 carry-over debt (installer upgrade path, officer-only eviction by code, sidebar test + docs debt). 3 phases planned (Phase 6 = installer shim, Phase 7 = admin allowlist, Phase 8 = test infra + persistence + docs backfill).
- **Mode:** yolo
- **Granularity:** coarse
- **Total v1.0.1 phases:** 3 (none started)
- **Total milestone v1.0 phases:** 5 (all shipped — archived in `.planning/milestones/v1.0-ROADMAP.md`)

## Current Position

Phase: Not started (Phase 6 next)
Plan: —
Status: v1.0.1 roadmap landed; awaiting `/gsd-plan-phase 6`
Last activity: 2026-05-11 — v1.0.1 ROADMAP.md written, 8/8 requirements mapped

### v1.0.1 Phase Plan (2026-05-11)

| Phase | Name | Requirements | Stack | Ship gate |
|-------|------|--------------|-------|-----------|
| 6 | Installer Overwrite-Running Shim | INST-06 | Go + NSIS | watcher v1.0.1 binary release |
| 7 | Admin Allowlist + Eviction Enforcement | ADMIN-01, ADMIN-02, ADMIN-03 | Apps Script | `clasp push` of apps-script bundle |
| 8 | Test Infra + Persistence + Docs Backfill | TEST-01, TEST-02, SEARCH-05, DOC-04 | Apps Script + docs | `clasp push` + green CI + milestone retrospective |

**Sequencing rationale:**

1. Phase 6 first because it is the only Go-side change and produces the binary release that tags the milestone (`v1.0.1`). Independent of the apps-script work.
2. Phase 7 second because ADMIN-02 (eviction enforcement) is the highest user-value bit — it closes the code-gap behind the v1.0 "social convention only" eviction model. ADMIN-01 is the prerequisite extend-only `_meta` row; ADMIN-03 is the management UX.
3. Phase 8 last because TEST-02 covers the new eviction + admin sidebars landing in Phase 7 — testing them after they ship is the natural order. SEARCH-05 and DOC-04 are independent but ride along in the same apps-script deploy.

**Schema impact:** NONE expected across all 3 phases. `_meta.schema_version` stays at 3; `WatcherMaxSchemaVersion` stays at 3 (the watcher binary rebuild in Phase 6 is for the NSIS install path, not for schema reasons). `_meta.guild_admins` is an extend-only row addition.

### v1.0 SHIPPED (2026-05-11)

Milestone v1.0 complete. 5/5 phases shipped. 69/69 effective requirements covered. Tag `v1.0.0` pushed. Pages site live at https://boejowen.github.io/SquireBot/. Archive lives in `.planning/milestones/v1.0-*.md`. See archive for the full execution log.

## Performance Metrics

| Metric | Value |
|--------|-------|
| Phases planned (v1.0.1) | 3 / 3 |
| Phases complete (v1.0.1) | 0 / 3 |
| Plans complete (v1.0.1) | 0 / 0 (no plans yet) |
| Requirements mapped (v1.0.1) | 8 / 8 |
| Requirements complete (v1.0.1) | 0 / 8 |
| Active blockers | 0 |
| Milestone v1.0 final | 69/69 effective requirements; 5/5 phases; 31/31 plans (archived) |

## Accumulated Context

### Decisions Log (v1.0.1)

Decisions locked at milestone open and roadmap creation:

1. **v1.0.1 is a patch milestone — small scope, 3 phases.** Eight requirements; granularity=coarse. Resist over-fragmenting. (Roadmap, 2026-05-11.)
2. **No schema bump.** `_meta.schema_version` stays at 3; `WatcherMaxSchemaVersion` stays at 3. `_meta.guild_admins` is an extend-only row addition. No watcher migration logic; no `SCRIPT_MIN_SCHEMA_VERSION` change. (Roadmap, 2026-05-11.)
3. **Phase 6 is the only Go-side change.** Everything else is apps-script + docs. The Phase 6 ship produces the binary release that tags the milestone (`v1.0.1`). (Roadmap, 2026-05-11.)
4. **Ship-gate model = single ship gate per phase.** Phase 6 ships a binary release; Phase 7 + Phase 8 each ship via `clasp push` of the apps-script bundle to the dev workbook. (Roadmap, 2026-05-11.)
5. **Owner-floor lockout protection on ADMIN-03.** Workbook owner email cannot be removed from `_meta.guild_admins` by anyone other than the owner themselves. (REQUIREMENTS.md ADMIN-03.)
6. **SEARCH-05 = PropertiesService per-user scope, NOT Document scope.** Recent-3 MRU is per-guildie state, not workbook-shared state. Migrating from CacheService to PropertiesService keeps the same scope semantics. (REQUIREMENTS.md SEARCH-05.)

### Decisions Log (v1.0, mirrored for continuity)

The 10 v1.0-era decisions are preserved in `PROJECT.md` Key Decisions table and in the archived `.planning/milestones/v1.0-ROADMAP.md` — they remain in force for v1.0.1.

### Open TODOs

- **(Day-10 token-survival check, fires automatically)** Routine `trig_01Uog2muQ22CBsjZfqPiSH9r` fires 2026-05-13T15:00:00Z to validate AUTH-03 / Pitfall #1 (refresh token survives the 7-day Testing-mode expiry boundary). If it succeeds, AUTH-03 is structurally validated permanently. If it fails, re-open consent-screen publishing investigation.
- **(Roadmap audit, v1.0 carry)** Reconcile the "56 v1 requirements" figure mentioned historically with the actual literal count of 69 REQ-IDs from v1.0. Already noted in v1.0 milestone audit; not blocking v1.0.1.
- **(SignPath OSS, separate track)** SignPath Foundation OSS approval is in flight; lands as a hotfix (NOT a v1.0.1 phase) when the Foundation review completes. Would retire INST-05 partial → full.

### Active Blockers

None. v1.0 shipped clean; v1.0.1 is planning-stage. Phase 6 is unblocked and ready for `/gsd-plan-phase 6`.

## Session Continuity

### Last Session Summary

**2026-05-11 (v1.0.1 milestone open + roadmap):** v1.0 shipped clean as tag `v1.0.0`. Milestone v1.0.1 Installer + Permissions Hardening opened the same day. v1.0.1 REQUIREMENTS.md defined 8 requirements across 5 carry-over features (INST-06, ADMIN-01..03, TEST-01/02, SEARCH-05, DOC-04). v1.0.1 ROADMAP.md mapped all 8 requirements onto 3 phases:

- Phase 6 (Installer Overwrite-Running Shim) — INST-06 only, Go + NSIS, ships watcher v1.0.1 binary
- Phase 7 (Admin Allowlist + Eviction Enforcement) — ADMIN-01..03, apps-script
- Phase 8 (Test Infra + Persistence + Docs Backfill) — TEST-01/02 + SEARCH-05 + DOC-04, apps-script + docs

Schema impact NONE across all 3 phases. 100% requirement coverage validated. REQUIREMENTS.md Traceability table updated.

**2026-05-01 → 2026-05-11 (v1.0 execution):** 5 phases shipped end-to-end over 11 days. Full execution log archived in `.planning/milestones/v1.0-ROADMAP.md`.

### Next Action

**Plan Phase 6 — Installer Overwrite-Running Shim.** Run `/gsd-plan-phase 6` to decompose INST-06 into executable plans. Expected scope: small (1–2 plans) — NSIS pre-install detection script + watcher graceful-shutdown signal handling + release CI bump + troubleshooting.md workaround removal. Output binary: tag `v1.0.1`.

After Phase 6 ships, plan Phase 7 (apps-script admin allowlist). After Phase 7 ships, plan Phase 8 (test infra + persistence + docs).

**Independent of v1.0.1:** Day-10 token-survival check fires automatically 2026-05-13T15:00:00Z (routine `trig_01Uog2muQ22CBsjZfqPiSH9r`).

### Files of Record

- `.planning/PROJECT.md` — core value, requirements summary, constraints, key decisions (updated 2026-05-11 with v1.0.1 milestone scope).
- `.planning/REQUIREMENTS.md` — v1.0.1 scope (8 requirements + Traceability table filled).
- `.planning/ROADMAP.md` — v1.0.1 phase structure (Phases 6–8) with success criteria.
- `.planning/STATE.md` — this file.
- `.planning/MILESTONES.md` — historical record (v1.0 archived; v1.0.1 next entry on ship).
- `.planning/milestones/v1.0-ROADMAP.md` — v1.0 archive (5 phases, 31 plans, 69/69 reqs).
- `.planning/milestones/v1.0-REQUIREMENTS.md` — v1.0 archive (full 69 REQ-ID reconciliation).
- `.planning/milestones/v1.0-MILESTONE-AUDIT.md` — v1.0 retrospective + tech debt log.
- `.planning/research/SUMMARY.md` — research synthesis (v1.0-era; still in force for v1.0.1).
- `.planning/research/STACK.md` — locked stack decisions.
- `.planning/research/PITFALLS.md` — 27 pitfalls catalogue.
- `.planning/config.json` — granularity (coarse), mode (yolo), workflow toggles.
- `docs/troubleshooting.md` — installer manual-stop workaround that Phase 6 retires.
- `docs/apps-script-deploy.md` — clasp-push deploy runbook used by Phase 7 + Phase 8 ship steps.

---

*State initialized: 2026-04-30 after roadmap creation. v1.0 milestone COMPLETE: 2026-05-11 (5/5 phases shipped). v1.0.1 roadmap landed: 2026-05-11 (3 phases planned, 0 started).*
