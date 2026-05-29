---
gsd_state_version: 1.0
milestone: v2.0
milestone_name: Off Google — Website Frontend
status: planning
last_updated: "2026-05-28T00:00:00.000Z"
last_activity: 2026-05-28
progress:
  total_phases: 6
  completed_phases: 0
  total_plans: 0
  completed_plans: 0
  percent: 0
---

# State: SquireBot

**Initialized:** 2026-04-30
**Last updated:** 2026-05-28 (v2.0 "Off Google" Website Frontend ROADMAP created — Phases 11–16)

## Project Reference

See: `.planning/PROJECT.md` (updated 2026-05-28 with v2.0 milestone scope)

- **Core value:** Every guildie can answer "what does my character still need, and where in the guild is it?" — now delivered via a self-hosted website instead of the Google Sheet.
- **Current focus:** Milestone v2.0 "Off Google" — replace the Google Sheet (UI + data store) with a self-hosted Go + SQLite backend (Oracle Cloud Always Free, ARM64) + static web frontend (SvelteKit), eliminating the Google OAuth dependency. Phases 11–16. ≈ $12/yr (domain only).
- **Mode:** yolo
- **Granularity:** coarse

## Current Position

Phase: 11 — Backend Foundation + Ingest API (not started; roadmap just created)
Plan: —
Status: Roadmap created; ready to plan Phase 11
Last activity: 2026-05-28 — v2.0 ROADMAP.md written (Phases 11–16); REQUIREMENTS.md traceability finalized (26/26 mapped)

### v2.0 Phase Plan (2026-05-28)

Coverage: 26/26 v2.0 requirements mapped to exactly one phase. No orphans, no duplicates. Full success criteria in `.planning/ROADMAP.md` § Phase Details.

| Phase | Name | Requirements | Stack | Depends on | Status |
|-------|------|--------------|-------|-----------|--------|
| 11 | Backend Foundation + Ingest API | BACKEND-01, BACKEND-02, BACKEND-03, BACKEND-04, BACKEND-06 | Go + SQLite + goose + Caddy (Oracle Always Free, ARM64) | — | Not started |
| 12 | Enrichment Job Migration | ENRICH-10, ENRICH-11 | Go in-process scheduler (PigParse + wiki parsers ported) | P11 | Not started |
| 13 | Watcher Re-Target + Onboarding | WATCH-08, WATCH-09, WATCH-10, WATCH-11 | Go watcher (`internal/backend` HTTP client; OAuth/Sheets/Picker deleted) | P11 | Not started |
| 14 | Web Frontend | BACKEND-05, WEB-01, WEB-02, WEB-03, WEB-04, WEB-05 | SvelteKit static + TanStack Table + Tailwind; Go read API | P11 (read API) + P12 (data) | Not started |
| 15 | Admin Web Forms + Login | AUTH-08, AUTH-09, ADMIN-04, ADMIN-05, ADMIN-06 | Discord OAuth2 login; web forms | P14 + P11 | Not started |
| 16 | Cutover + Decommission | CUTOVER-01, CUTOVER-02, CUTOVER-03, CUTOVER-04 | shadow soak + backfill + coordinated self-update flip | P13 + P14 + P15 + P12 | Not started |

**Sequencing rationale (FRONT-LOAD THE INGEST PATH):**

1. **P11 first** because the ~12 guildies are dark on the Sheet during the build (Google walled off their watchers); P11 stands up the authenticated ingest endpoint so data has somewhere to go. No dependencies — it is the foundation.
2. **P12 + P13 next** (both depend only on P11): P12 makes the new DB self-populate its dimension data; P13 re-points the watcher at the ingest API and ships it via the auto-updater — together these restore the live data pipeline end-to-end before any polished UI exists.
3. **P14** (depends on P11 read API + P12 data) rebuilds the visible product — the 4 views, search, tooltips, theming.
4. **P15** (depends on P14 + P11) adds Discord login + the officer-only write forms; must exist before the Sheet's admin sidebars can retire.
5. **P16 last** (depends on P13 + P14 + P15 + P12) — shadow soak, human-data backfill, one coordinated watcher flip, then decommission Sheet + Apps Script + Google OAuth client. Meets the "Off Google" goal.

**Locked stack overrides research:** SCOPE.md + the 4 findings docs recommended Hetzner VPS + PostgreSQL; that is **superseded** by the locked decision — Oracle Cloud Always Free (ARM64) + SQLite + `goose`. DDL ports with `CITEXT`→`TEXT COLLATE NOCASE` + identity-syntax changes; `pg_trgm` search → SQLite FTS5 / `LIKE`.

**Schema-evolution change:** the `_meta.schema_version` ↔ `WatcherMaxSchemaVersion` handshake retires in favour of forward-only `goose` DB migrations + an explicit API version (`/api/v1/...`). The watcher's Sheets schema gate is removed in P13.

**Open decision surfaced to P11 (not a separate phase):** optional 1-day **PocketBase** spike at the start of P11 (open-source single Go binary = SQLite + auth + REST + admin UI, self-hosted on the same Oracle box) could compress P11 **and** P15 by ~5–8 days. Evaluate before committing to a hand-rolled Go server; verdict captured in the P11 CONTEXT.

### Superseded: 999.19 Google brand verification

Google REJECTED brand verification 2026-05-15 ("home page not registered to you" — `github.io` can't satisfy it), walling off all ~12 guildie watchers. **v2.0 "Off Google" supersedes this** by removing Google entirely rather than renting a domain to placate it. The guild stays dark on the sheet during the v2.0 build (P11 ingest is front-loaded to restore data flow first). Full incident trail at `.planning/debug/v1-0-2-oauth-invalid-client-incident.md`. Anti-pattern note for future Google-OAuth incidents: check OAuth consent screen Verification status FIRST when seeing `Access blocked / Error 400: invalid_request`; the multiple-secrets / loopback-IP / redirect-URI angles are red herrings.

**Shipped to date:**

| Milestone | Phases | Plans | Tag | Date |
|-----------|--------|-------|-----|------|
| v1.0 — Watcher + Workbook + Onboarding | 5 (Phases 1–5) | 31 | `v1.0.0` | 2026-05-11 |
| v1.0.1 — Installer + Permissions Hardening | 3 (Phases 6–8) | 12 | `v1.0.1` (binary; Phase 6 ship gate) | 2026-05-12 |
| v1.0.2 — Robustness Polish (binary; milestone close superseded by v2.0) | 2 (Phases 9–10) | 8 | `v1.0.2` (binary) | 2026-05-13 |

> v1.0.2 binary shipped but its milestone close was superseded by the v2.0 pivot (the Sheet it targeted is being replaced). Phase directories 09 + 10 exist on disk; v2.0 continues at **Phase 11** (never reuses 9 or 10). v1.0.2 was never written to MILESTONES.md.

## Performance Metrics

| Metric | Value |
|--------|-------|
| Total milestones shipped | 2 fully closed (v1.0, v1.0.1) + 1 binary-only (v1.0.2, close superseded) |
| Total phases shipped | 10 (1–10) |
| Total plans shipped | 51 (31 + 12 + 8) |
| Total commits since init | 266+ (203 v1.0 + 63 v1.0.1 + v1.0.2 work) |
| Watcher LOC (Go) | 14,507 (pre-v2.0; expected to shrink ~2k in P13) |
| Apps-script LOC (TypeScript) | 13,266 (to be decommissioned in P16) |
| Vitest tests | 336/336 passing (apps-script; pre-v2.0) |
| Active blockers | 0 |

## Accumulated Context

### Decisions Log

All locked decisions live in `PROJECT.md` Key Decisions table and the per-milestone archive ROADMAPs. v1.0 contributed 11 PROJECT-level decisions; v1.0.1 contributed 5 more.

**v2.0 milestone-open decisions (locked, see PROJECT.md Current Milestone):**

- **Off Google entirely** — replace the Sheet (UI + data store), not placate Google's brand verification.
- **Backend = Oracle Cloud Always Free (Ampere A1, ARM64) + Caddy + SQLite + `goose`** (overrides the research docs' Hetzner+Postgres recommendation).
- **Frontend = SvelteKit static (adapter-static) + TanStack Table + Tailwind** on Cloudflare/GitHub Pages.
- **Watcher↔backend auth = per-guildie opaque bearer token** ("guild code"), hashed server-side, stored client-side in DPAPI wincred; character ownership derived server-side from the credential (`_char_owner`/`UpsertCharOwner` deleted).
- **Website login = Discord OAuth2** gated on guild Discord membership (gate-free; pre-pays AUTH-09). "Sign in with Google" rejected outright (re-introduces the gate).
- **Schema evolution = forward-only `goose` migrations + API version**; retire `_meta.schema_version`/`WatcherMaxSchemaVersion`.
- **Cutover = hybrid shadow-mode** (never writes to the Sheet; self-healing inventory + enrichment).

### Open TODOs

- **(P11 PocketBase spike)** 1-day evaluation at the start of Phase 11 — self-hosted PocketBase vs. hand-rolled Go server; could compress P11 + P15 by ~5–8 days. Capture verdict in P11 CONTEXT.
- **(Domain registration)** ~$12/yr website-only domain needs registering before the backend goes live (P11 / P16). No Google verification required.
- **(Port-relevant backlog into P14)** 999.28 (`didYouMean('')` empty-query contract) + 999.30 (`didYouMean` Levenshtein contract mismatch) should be resolved when porting search logic to the frontend (WEB-03).
- **(Watcher gofmt/console nits into P13)** 999.20 (`console_windows.go` gofmt) + 999.21 (`freeConsole()` doc-vs-impl) + 999.22 (SemVer-aware auto-update comparison — load-bearing for the P16 coordinated flip) ride along with the watcher re-target.
- **(SignPath OSS, separate track, 999.9)** still in flight; orthogonal to the backend swap; lands as a hotfix when the Foundation review completes.

### Active Blockers

None. Roadmap created; Phase 11 ready to plan.

## Session Continuity

### Last Session Summary

**2026-05-28 (v2.0 ROADMAP created):** Roadmapper transformed the 26 v2.0 requirements into a 6-phase structure (Phases 11–16), accepting REQUIREMENTS.md's provisional A–F mapping unchanged (no concrete coverage problem found). Coverage validated 26/26 (P11=5, P12=2, P13=4, P14=6, P15=5, P16=4); no orphans, no duplicates. Dependencies set: P11 has none; P12→P11; P13→P11; P14→P11+P12; P15→P14+P11; P16→P13+P14+P15+P12. Each phase got 2–5 observable success criteria. Honored the locked stack (Oracle Always Free + SQLite + goose; SvelteKit; Discord OAuth2; bearer token) over the research docs' Hetzner+Postgres recommendation, with an explicit "locked overrides research" note in ROADMAP. Front-loaded the ingest path (P11 + P13 restore data flow before the polished P14 frontend). Surfaced the optional PocketBase spike as a P11 decision note. Wrote `.planning/ROADMAP.md` (v2.0 section + Phase Details + progress tables + backlog re-annotated for Sheet-mooting), finalized `.planning/REQUIREMENTS.md` traceability (Phase column finalized), and reset this STATE.md to the v2.0 phase plan (replacing the stale v1.0.2 phase-plan content). Phase numbering starts at 11 (Phase dirs 09+10 exist on disk from the superseded v1.0.2 binary; never reuse 9/10).

**2026-05-13 (v1.0.2 binary shipped):** Phases 9 + 10 shipped as binary tag `v1.0.2`; HUMAN-UAT was blocked on 999.19 (Google brand verification). Milestone close subsequently superseded by the v2.0 "Off Google" pivot — the Sheet it targeted is being replaced, so a Google-OAuth-dependent UAT close became moot.

### Files of Record

- `.planning/PROJECT.md` — core value, requirements (v1 Validated + v2.0 Active), constraints, Key Decisions (updated 2026-05-28 with v2.0 milestone).
- `.planning/REQUIREMENTS.md` — v2.0 requirements (26 across 7 categories) + finalized traceability (Phases 11–16).
- `.planning/ROADMAP.md` — v2.0 Phases 11–16 (full Phase Details + success criteria) + collapsed v1.0/v1.0.1/v1.0.2 + Backlog (updated 2026-05-28).
- `.planning/MILESTONES.md` — historical record (v1.0 + v1.0.1; v1.0.2 not recorded — close superseded).
- `.planning/explorations/website-milestone/SCOPE.md` — authoritative v2.0 research (A–F phase plan, dependencies, effort, open decisions).
- `.planning/explorations/website-milestone/{01-backend-hosting,02-frontend-stack,03-watcher-auth,04-data-enrichment-migration}.md` — 4 findings docs.
- `.planning/RETROSPECTIVE.md` — living retrospective (v1.0 + v1.0.1 + Cross-Milestone Trends).
- `.planning/STATE.md` — this file.
- `.planning/config.json` — granularity (coarse), mode (yolo), workflow toggles.

### Next Action

`/clear` then `/gsd-plan-phase 11` — Phase 11 (Backend Foundation + Ingest API) is the first v2.0 phase, no dependencies. Planner reads ROADMAP.md § Phase 11 + the 4 findings docs (esp. `01-backend-hosting.md` for SQLite/goose/Caddy/Oracle + `04-data-enrichment-migration.md` §1 for the SQLite schema) + REQUIREMENTS.md (BACKEND-01/02/03/04/06). First plan should fold in the optional 1-day PocketBase spike decision.

---

*State initialized: 2026-04-30. v1.0 COMPLETE: 2026-05-11. v1.0.1 COMPLETE: 2026-05-12. v1.0.2 binary shipped: 2026-05-13 (close superseded). v2.0 OPENED + ROADMAP created: 2026-05-28 (Phases 11–16).*
