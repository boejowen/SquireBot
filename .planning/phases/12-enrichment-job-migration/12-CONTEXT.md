---
phase: 12-enrichment-job-migration
created: 2026-05-29
mode: auto (discuss --auto; defaults picked by Claude for a faithful port)
requirements: [ENRICH-10, ENRICH-11]
---

# Phase 12: Enrichment Job Migration — CONTEXT

<domain>
Move the two enrichment feeds off Apps Script time-driven triggers into the Go backend's in-process scheduler (the `time.Ticker` skeleton stood up in 11-05), so the new SQLite DB self-populates its dimension tables: the **daily PigParse price pull** (ENRICH-10) and the **weekly P1999 wiki scrape** (ENRICH-11). The parsers are pure, host-agnostic functions that port near-verbatim to Go; only the Apps Script I/O wrappers (`UrlFetchApp`/`PropertiesService`/`CacheService`/`LockService`/time-triggers) are replaced. Backend scheduled jobs only — no UI.
</domain>

<decisions>
Defaults locked for a faithful port (per ROADMAP §Phase 12 Note + findings doc §2). All low-ambiguity — the existing parsers + politeness controls are well-tested and carry over.

- **D-1 — Port the 4 pure parsers to Go verbatim.** `parseToRows` (PigParse), `parseItempage` (wiki item summary), the wiki-spell parser, and the wiki-gear-tier parser are framework-agnostic and transliterate 1:1. **Reuse their existing fixtures** (`apps-script/src/__fixtures__/`, e.g. `wiki-class-necromancer.json`, plus the pigparse/wiki test fixtures) as the Go test fixtures so parity is byte-checkable. New Go home: `internal/backendsrv/enrich/` (parsers) + `internal/backendsrv/enrich/jobs/` (orchestration), or similar — planner decides exact package layout.

- **D-2 — Register two jobs into the in-process scheduler.** Keep the existing cadence (well-tuned): **PigParse daily**, **wiki items + spells + gear-tier weekly (Sunday)**. Replace 11-05's no-op `time.Ticker` skeleton with real job registration that computes next-run and **persists last-run timestamps in the DB** (an `app_config`/`job_run` row) so a process restart neither double-runs nor skips a due job. A small cron-expression helper or compute-next-run is fine; no external scheduler.

- **D-3 — Port `politeFetch` verbatim as a Go `net/http` client.** Carry over ALL politeness controls (they are good-external-citizen behavior toward community-run services and are an explicit success criterion): identifying `User-Agent` (`SquireBot/<ver> (+github url)`), `If-None-Match`/`If-Modified-Since` → **304 short-circuit**, **exponential backoff `[2s,4s,8s,16s,32s]` honoring `Retry-After`** on 429/503/504, and the **1-second inter-request sleep** between wiki fetches. ETag/Last-Modified state moves from `PropertiesService` to a DB row/table (`etag_cache(url, etag, fetched_at)` or `app_config`). `CacheService` response cache → drop (the ETag/304 path is the real politeness control; at ~12 users the load is trivial).

- **D-4 — Graceful degradation via upsert, not all-or-nothing.** PigParse writes `INSERT … ON CONFLICT (item_id) DO UPDATE` so a truncated/partial response **updates what it got and leaves the rest** rather than clobbering good data. Keep the existing **truncation guard** (today's row count < 90% of last-known) as a belt-and-braces sanity **log** (not a hard abort). Weekly wiki tables upsert per natural key the same way.

- **D-5 — Delete the Apps Script workarounds.** Remove the **6-minute-cap resumable-cursor** machinery (`CURSOR_KEY` checkpoint/self-rescheduling in `refreshWikiItems.ts`) — a backend job has no execution cap, so the weekly scrape is **one uninterrupted run**. Drop `monitorCellCount` (10M-cell watchdog) and `weeklySchemaHealthcheck` (expected-tab watchdog) — both Sheets-specific. `LockService` → the single-writer DB (`SetMaxOpenConns(1)` from 11-02) plus, if needed, a small in-process per-job mutex so two cycles can't overlap.

- **D-6 — Populate the dimension tables 11-02 already created (empty).** The 5 dimension tables exist from 11-02's `00001_init.sql`: `item_master`, `pigparse_price`, `wiki_spells`, `wiki_gear_tier`, `quest_items`. ⚠ **OPEN FOR PLANNER:** the findings doc §1.3–1.4 proposes richer columns (e.g. `item_price.t30/a30/…`, `item_master.wiki_summary/wikitext_sha1`, `wiki_gear_tier.tier/rank`, `quest_item.source`) than the empty 11-02 tables may currently have. The planner/researcher MUST diff the **actual** 11-02 table columns against each parser's output struct and add a **forward-only `goose` migration `00003_*.sql`** for any missing columns (extend-only, never edit a shipped migration).

- **D-7 — Parity check is the acceptance proof.** After one daily + one weekly cycle, spot-check the backend's dimension data against the **live Sheet's** `_item_master`/`_pigparse`/`_wiki_spells`/`_wiki_gear_tier`/`_quest_items`. This is the success criterion, not a separate feature.

- **D-8 — Strictly scope to the two enrichment feeds.** ENRICH-10 + ENRICH-11 only. The eviction/stale-archive jobs (`weeklyEvictionArchive`, `weeklyStaleCharArchive`) from findings §5 and any Sheet→DB **backfill/cutover** are NOT in this phase (they belong to P15/P16). Do not scope-creep.
</decisions>

<canonical_refs>
MUST read before/while planning (full relative paths):
- `.planning/explorations/website-milestone/04-data-enrichment-migration.md` — §2 (Enrichment Job Migration) is the authoritative migration guide: what ports cleanly, the Apps-Script→backend construct table, politeness controls, cadence. **Primary ref.**
- `.planning/ROADMAP.md` §Phase 12 — goal, 4 success criteria, and the "parsers are pure / I/O wrappers replaced / watchdogs dropped" Note.
- `.planning/REQUIREMENTS.md` — ENRICH-10, ENRICH-11.
- Existing pure parsers to port (with their tests + fixtures):
  - `apps-script/src/lib/pigparse-types.ts` (`parseToRows`) + `apps-script/src/__tests__/pigparse-types.test.ts`
  - `apps-script/src/lib/wiki-parser.ts` (`parseItempage`) + `apps-script/src/__tests__/wiki-parser.test.ts`
  - `apps-script/src/lib/wiki-spell-parser.ts` + `apps-script/src/lib/wiki-spell-types.ts`
  - `apps-script/src/lib/wiki-gear-tier-parser.ts` + `apps-script/src/lib/wiki-gear-tier-types.ts` + `apps-script/src/__tests__/wiki-gear-tier-parser.test.ts`
  - `apps-script/src/__fixtures__/` (reuse as Go test fixtures)
- Existing politeness control to port: `apps-script/src/lib/politeFetch.ts` + `apps-script/src/__tests__/politeFetch.test.ts`
- Existing trigger orchestration (the I/O to replace, NOT port verbatim): `apps-script/src/triggers/refreshPigparse.ts`, `refreshWikiItems.ts`, `refreshWikiSpells.ts`, `refreshWikiGearTier.ts`
- Backend foundation this builds on (Phase 11): `internal/backendsrv/scheduler/scheduler.go` (the skeleton to flesh out), `internal/backendsrv/store/db.go` (single-writer DB), `internal/backendsrv/migrations/` (goose), `internal/backendsrv/migrations/00001_init.sql` (the empty dimension tables).
- External APIs (NOT scraping targets — typed APIs): PigParse `GET /api/item/getall/1` (server=1=Blue, daily); P1999 MediaWiki `action=parse&prop=wikitext` (weekly). Per CLAUDE.md.
</canonical_refs>

<code_context>
- The scheduler exists as a skeleton: `internal/backendsrv/scheduler/scheduler.go` (`scheduler.Start(ctx)`, `time.Ticker`, `jobs:0`). Phase 12 fleshes it out with the two real jobs.
- DB single-writer + goose migrations are in place (11-02). The 5 dimension tables are created but empty.
- The watcher's own `internal/parse` is the inventory/spellbook parser (already UTF-8, 11-03) — DISTINCT from these enrichment parsers; don't conflate.
- Structured logging convention: Go `slog` JSON to stdout (journald), set up in `internal/backendsrv/logging` (11-05).
- Tests run on the Windows dev box (`go test ./...`); jobs are pure + DB-backed, deterministic with fixtures.
</code_context>

<deferred>
- Eviction / stale-character archive jobs (`weeklyEvictionArchive`, `weeklyStaleCharArchive`) — findings §5; future phase (admin/privacy, ~P15).
- Sheet→DB one-time backfill + shadow-soak cutover — findings §4; that's P16 (Cutover + Decommission).
- Optional inventory history (append-only snapshots) — findings §1.2; explicitly parked (not Core Value).
</deferred>

<open_for_planner>
1. **Dimension-table column reconciliation (D-6)** — diff actual 11-02 table columns vs each parser's output; author `00003_*.sql` for gaps. Highest-priority research item.
2. Exact Go package layout for parsers vs job orchestration vs the polite HTTP client.
3. Scheduler design: compute-next-run + persisted last-run vs a tiny cron lib; how to make "due on startup if missed" deterministic.
4. Whether `pigparse_price` table name + columns from 11-02 match the findings' `item_price` shape (naming drift to resolve).
</open_for_planner>
