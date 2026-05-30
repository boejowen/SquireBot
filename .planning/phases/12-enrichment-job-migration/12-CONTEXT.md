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

**Locked from 12-RESEARCH.md (2026-05-29) per this phase's faithful-port criterion (mode=auto; Claude picks low-ambiguity defaults). Each is isolated + cheaply reversible if the maintainer later disagrees.**

- **D-9 — PigParse t=0/t=1 duplicate `item_id` → keep only the WTS (t=0) row before upsert.** The `getall` response carries two rows per `item_id` (t=0 WTS/sell, t=1 WTB/buy) but `pigparse_price.item_id` is a PRIMARY KEY (one row/item). The Sheet kept both (dup ids) yet its views read the direction-agnostic `current_avg`/`blue_volume` (a30/t30) aliases, so `direction` was decorative. Lock: keep the **WTS (t=0, sell-side)** row — the standard "what's it worth to sell" value, satisfies the PK, most defensible single price. Implement as one isolated filter so flipping to last-wins or a `(item_id,direction)` key later is a one-line change. (Backend will have fewer `pigparse_price` rows than the Sheet's dup-keeping tab — expected, note it in the D-7 parity check, NOT a failure.)
- **D-10 — Job cadence = simple interval, not wall-clock pinning.** PigParse due when `now - last_run_at >= 24h`; wiki due when it is Sunday (UTC) AND `last_run_at` precedes this Sunday 00:00 UTC. Restart-robust, no timezone math. (Pin to a UTC off-peak hour only if the maintainer later wants politeness timing — not required.)
- **D-11 — Add a backend `var Version` (settable via `-ldflags -X`) for the politeFetch User-Agent**, mirroring the watcher's `main.Version`; a hardcoded `SquireBot/dev (+https://github.com/boejowen/SquireBot)` fallback is acceptable. The UA must stay identifying + contactable (the GitHub URL satisfies that).
- **D-12 — Stale-row cleanup = Sheet-faithful per-key replace.** `wiki_spells`: per-class DELETE+INSERT in one tx. `quest_items`: per-`item_id` DELETE+INSERT. `wiki_gear_tier`: **full-table replace** (its declared `UNIQUE(…, item_id)` is broken — `item_id` is always NULL and NULLs are distinct in SQLite UNIQUE → upsert never fires). `item_master`: pure per-item upsert (items aren't "removed"; keep the wikitext-SHA-1 short-circuit). `pigparse_price`: per-item upsert (D-4 graceful degradation).
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
**All 4 resolved by 12-RESEARCH.md (2026-05-29) — kept here for traceability.**
1. **Dimension-table column reconciliation (D-6)** — RESOLVED. The 11-02 tables are already "rich"; only `pigparse_price` is short the 8 price-history cols (`t30,a30,t60,a60,t6m,a6m,ty,ay`). Exact forward-only `00003_enrich_columns.sql` (8 `ADD COLUMN` + new `job_run` + `etag_cache` tables) is in RESEARCH §"Exact 00003 migration SQL". SQLite allows only one column per `ALTER TABLE ADD COLUMN`.
2. **Go package layout** — RESOLVED: `internal/backendsrv/enrich/` (4 pure parsers) + `enrich/politefetch/` (net/http client) + `enrich/jobs/` (orchestration); all upsert SQL lives in `store/` (11-05 single-tested-SQL-path rule). See RESEARCH §"Go Package Layout".
3. **Scheduler design** — RESOLVED: `job_run.last_run_at` DB cursor + an immediate check pass on startup makes "due-on-startup-if-missed" deterministic; per-job `sync.Mutex` replaces `LockService`; compute-next-run predicates, no cron lib. See RESEARCH §"Scheduler Design" + D-10.
4. **`pigparse_price` vs `item_price` naming drift** — RESOLVED: **`pigparse_price` is authoritative** (the name in `00001_init.sql`); the findings doc's `item_price` does not exist in the DB — map its proposed columns onto `pigparse_price`.

⚠ The one genuine decision the research surfaced (PigParse t=0/t=1 PK collision) is locked as **D-9** above.
</open_for_planner>
