# Roadmap: SquireBot

**Core Value:** Every guildie can answer "what does my character still need, and where in the guild is it?" without leaving the spreadsheet.

## Milestones

- ✅ **v1.0** — Watcher + Workbook + Onboarding (initial release) — shipped 2026-05-11 as tag `v1.0.0`

## Phases

<details>
<summary>✅ v1.0 — Watcher + Workbook + Onboarding (Phases 1–5) — SHIPPED 2026-05-11</summary>

Full details in [`milestones/v1.0-ROADMAP.md`](milestones/v1.0-ROADMAP.md).

- [x] Phase 1: End-to-End Thin Slice (8 plans) — shipped v0.1.0, 2026-05-02
- [x] Phase 2: Watcher Robustness + Schema Lock (10 plans) — shipped v0.2.0 + v0.2.1 hotfix, 2026-05-09
- [x] Phase 3: Apps Script Enrichment Foundation (4 plans) — shipped v0.3.0, 2026-05-10
- [x] Phase 4: Differentiator Features (4 plans) — shipped v0.4.0, 2026-05-11
- [x] Phase 5: Search + Onboarding + Privacy Polish (5 plans) — shipped v1.0.0, 2026-05-11

**Total:** 31 plans · 5 phases · 11 days kickoff to ship · 203 commits.

</details>

### ▢ Next milestone (planned)

No phases scheduled yet. Start with `/gsd-new-milestone` to define v1.0.1 (patch) or v1.1 (feature).

## Progress

| Milestone | Phases | Plans Complete | Status | Completed |
|-----------|--------|----------------|--------|-----------|
| v1.0 | 5 | 31/31 | ✅ Shipped | 2026-05-11 |

## Backlog

Carried forward from v1.0 (candidates for v1.0.1 / v1.1 / v2; see `milestones/v1.0-MILESTONE-AUDIT.md` § Tech Debt and `milestones/v1.0-ROADMAP.md` § Issues Deferred for context):

- **999.1** Bank-coin permission lock (only bank-toon-owner can use Set Bank Coin sidebar) — Phase 4 deferred
- **999.2** Polished theme picker tile UI (6-tile grid per `docs/design/mockups/eq-aesthetic-picker.html`) — Phase 4 deferred
- **999.3** Sidebar HTML inline-JS unit tests (requires JSDOM setup not currently in vitest) — Phase 4/5 deferred
- **999.4** Installer-driven upgrade UX (NSIS overwrite-running shim; workaround documented in troubleshooting.md) — v1.0.1 candidate
- **999.5** Self-service eviction (departing guildie quits cleanly without officer action) — v2 candidate (threat-model deferred)
- **999.6** `_meta.guild_admins` allowlist (eviction sidebar enforces officer-only by code) — 05-04 v1.0.x polish
- **999.7** Extract `SIDEBAR_BODY` constants to `apps-script/src/sidebars/*.html` — uniform deferral across 05-03 + 05-04
- **999.8** PropertiesService migration for recent-query persistence (CacheService 25-min cap shorter than ideal) — 05-03 v1.0.1 candidate
- **999.9** SignPath Foundation OSS approval — submitted; awaiting review (would retire INST-05 partial → full)
- **999.10** Backfill SUMMARY.md for Phases 3 + 4 (8 plans without summaries) — documentation tech debt
- **999.11** Decide v1.1+ verification doctrine — adopt `/gsd-verify-work` per phase, or formalize live-smoke pattern
- **999.12** v2: Wantlist + Discord pinger (WANT-01..08; prerequisites WANT-06/07 still open)

---

*Roadmap created: 2026-04-30. v1.0 shipped: 2026-05-11. Last reorganized: 2026-05-11 during `/gsd-complete-milestone v1.0`.*
