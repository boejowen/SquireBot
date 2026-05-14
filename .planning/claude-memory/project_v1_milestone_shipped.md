---
name: SquireBot v1.0 milestone SHIPPED + archive structure
description: v1.0 SHIPPED 2026-05-11 as tag v1.0.0. All 5 phases / 31 plans complete. Archive in .planning/milestones/. Next milestone undefined.
type: project
originSessionId: 73d699a8-e0e1-44af-b055-33c890e7a913
---
**v1.0 SHIPPED 2026-05-11** as tag `v1.0.0` (commit `bd36379`). Five phases, 31 plans, 203 commits, 11 days kickoff to ship. Effective requirements coverage 69/69 (66 satisfied + 2 partial via user-authorized scope reductions + 1 waived). DOC-02 fake-guildie eviction smoke PASS all 7 checkpoints. Pages onboarding site live at https://boejowen.github.io/SquireBot/. 297/297 vitest tests green. Schema at `_meta.schema_version = 3`.

**Archive structure** at `.planning/milestones/`:
- `v1.0-ROADMAP.md` — full phase details + locked decisions + deferred items
- `v1.0-REQUIREMENTS.md` — reconciled traceability for all 69 REQ-IDs
- `v1.0-MILESTONE-AUDIT.md` — audit (verdict: tech_debt, doc-only)

**Top-level files** at `.planning/`:
- `MILESTONES.md` — historical record (one entry per shipped version)
- `RETROSPECTIVE.md` — lessons learned (cross-milestone trends accumulate)
- `ROADMAP.md` — collapsed to milestone groupings + 12-item backlog (999.1..12)
- `PROJECT.md` — evolved with Current State + Validated reqs
- `STATE.md` — status `milestone-v1.0-complete`
- `REQUIREMENTS.md` — DELETED at milestone close (gitignored; fresh file via `/gsd-new-milestone`)

**v1.0 scope reductions / waivers** (user-authorized; all recorded in CONTEXT.md §Scope Changes):
- INST-05 — SmartScreen "video or screenshot sequence" DROPPED 2026-05-11; text-only walkthrough ships
- SEARCH-03 — inline per-row staleness DROPPED 2026-05-11; satisfied via Path 2 (existing view/bank Last Synced + cache-freshness tooltip)
- ENRICH-09 — courtesy emails to PigParse/wiki admins WAIVED 2026-05-09

**v1.0.1 / v1.1+ backlog** (12 items in ROADMAP.md): installer overwrite-running shim, SignPath OSS approval (in flight), sidebar inline-JS unit tests, PropertiesService recent-query persistence, `_meta.guild_admins` allowlist, polished theme picker tile UI, backfill Phase 3+4 SUMMARY.md, v1.1+ verification doctrine decision, v2 Wantlist + Discord pinger (prerequisites still open).

**Next milestone undefined.** Start via `/gsd-new-milestone` when ready.

**Important:** Phases 3 + 4 never wrote SUMMARY.md files (8 plans without summaries). Verification lives in STATE.md commentary + `03-SMOKE-TEST.md` + Phase 4 live-smoke commentary + git tags. Phases 1, 2, 5 have all SUMMARY.md files. Project doctrine has been live smoke tests instead of `/gsd-verify-work` — no VERIFICATION.md files anywhere. Decide explicitly at v1.1 kickoff.
