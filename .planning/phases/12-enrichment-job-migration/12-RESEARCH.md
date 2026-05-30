# Phase 12: Enrichment Job Migration - Research

**Researched:** 2026-05-29
**Domain:** Go in-process scheduled jobs; porting 4 pure TS parsers + a polite HTTP client to Go; populating SQLite dimension tables from PigParse + P1999 wiki feeds
**Confidence:** HIGH (every claim below verified against the actual source files in this repo — DDL, parsers, triggers, politeFetch, scheduler skeleton)

## Summary

Phase 12 replaces two Apps Script time-driven triggers (daily PigParse, weekly P1999 wiki) with two real jobs in the Go backend's in-process scheduler — the `time.Ticker` skeleton that 11-05 stood up registering `0` jobs. The work splits cleanly into four porting tasks plus a schema-reconciliation task:

1. **Port the 4 pure parsers verbatim** (`parseToRows`, `parseItempage`, `parseClassPage`, `parseGearTierPage`) from TypeScript to Go. They are framework-agnostic string→struct functions; the only Apps-Script API they touch is `Utilities.computeDigest` (SHA-1, in `wiki-parser.ts` only) which becomes `crypto/sha1`. Their existing fixtures in `apps-script/src/__fixtures__/` load directly as Go test fixtures for byte-checkable parity.
2. **Port `politeFetch`** from a `UrlFetchApp` wrapper to a `net/http` client, carrying over every politeness control. The only state that moves is the ETag (PropertiesService → a new `etag_cache` DB table).
3. **Flesh out the scheduler** with two registered jobs, persisted last-run timestamps (a new `job_run` table) so "due-on-startup-if-missed" is deterministic across restarts, and a per-job mutex.
4. **Replace the Apps Script I/O wrappers** in the four `refresh*.ts` triggers with Go job orchestration that upserts into the five SQLite dimension tables. **Delete** the Sheets-specific workarounds (6-min cursor, `monitorCellCount`, `weeklySchemaHealthcheck`, `LockService`).
5. **Reconcile + extend the dimension-table columns** via a forward-only `00003_*.sql` goose migration.

**The single most important finding:** the 11-02 dimension tables are NOT the bare findings-doc shape — the 11-02 author already copied the richer columns (`item_master.wiki_summary`/`wikitext_sha1`, `wiki_gear_tier.tier`/`rank`, `quest_items.source`) into `00001_init.sql`. The reconciliation gap is therefore **narrow**: only `pigparse_price` is missing the 30/60/6-month/year price-history columns (`t30,a30,t60,a60,t6m,a6m,ty,ay`) that the PigParse parser emits, plus a thin set of helper additions. The `00003` migration is small.

**Primary recommendation:** Port parsers to `internal/backendsrv/enrich/`, orchestration to `internal/backendsrv/enrich/jobs/`, the HTTP client to `internal/backendsrv/enrich/politefetch/`; add ONE forward-only migration `00003_enrich_columns.sql` (per-column `ALTER TABLE ADD COLUMN` — SQLite forbids multi-column adds), creating `job_run` + `etag_cache` and adding the 8 PigParse price columns; upsert per natural key with `INSERT … ON CONFLICT … DO UPDATE`; keep the truncation guard as a LOG, never a hard abort.

## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-1 — Port the 4 pure parsers to Go verbatim.** `parseToRows` (PigParse), `parseItempage` (wiki item summary), the wiki-spell parser, and the wiki-gear-tier parser transliterate 1:1. **Reuse their existing fixtures** (`apps-script/src/__fixtures__/`) as the Go test fixtures so parity is byte-checkable. New Go home: `internal/backendsrv/enrich/` (parsers) + `internal/backendsrv/enrich/jobs/` (orchestration) — planner decides exact layout.
- **D-2 — Register two jobs into the in-process scheduler.** Keep cadence: **PigParse daily**, **wiki items + spells + gear-tier weekly (Sunday)**. Replace 11-05's no-op `time.Ticker` skeleton with real job registration that computes next-run and **persists last-run timestamps in the DB** so a process restart neither double-runs nor skips a due job. A small cron helper / compute-next-run is fine; no external scheduler.
- **D-3 — Port `politeFetch` verbatim as a Go `net/http` client.** Carry over ALL politeness controls: identifying `User-Agent`, `If-None-Match`/`If-Modified-Since` → **304 short-circuit**, **exponential backoff `[2s,4s,8s,16s,32s]` honoring `Retry-After`** on 429/503/504, **1-second inter-request sleep** between wiki fetches. ETag/Last-Modified state moves from `PropertiesService` to a DB table. `CacheService` response cache → drop.
- **D-4 — Graceful degradation via upsert, not all-or-nothing.** PigParse writes `INSERT … ON CONFLICT (item_id) DO UPDATE` so a truncated/partial response updates what it got and leaves the rest. Keep the **truncation guard** (today's row count < 90% of last-known) as a sanity **log** (not a hard abort). Weekly wiki tables upsert per natural key the same way.
- **D-5 — Delete the Apps Script workarounds.** Remove the **6-minute-cap resumable-cursor** machinery (`CURSOR_KEY` in `refreshWikiItems.ts`). Drop `monitorCellCount` and `weeklySchemaHealthcheck`. `LockService` → the single-writer DB (`SetMaxOpenConns(1)`) plus, if needed, a small in-process per-job mutex.
- **D-6 — Populate the dimension tables 11-02 already created (empty).** ⚠ OPEN FOR PLANNER: diff actual 11-02 columns vs each parser's output; add a forward-only `00003_*.sql` for gaps (extend-only). **Resolved in this doc — see Column Reconciliation.**
- **D-7 — Parity check is the acceptance proof.** After one daily + one weekly cycle, spot-check the backend's dimension data against the live Sheet's `_item_master`/`_pigparse`/`_wiki_spells`/`_wiki_gear_tier`/`_quest_items`.
- **D-8 — Strictly scope to the two enrichment feeds.** ENRICH-10 + ENRICH-11 only. Eviction/stale-archive jobs and any Sheet→DB backfill/cutover are NOT in this phase (P15/P16).

### Claude's Discretion (the 4 `<open_for_planner>` items — all resolved in this doc)

1. Dimension-table column reconciliation (D-6) → **Column Reconciliation** section + exact `00003` SQL.
2. Exact Go package layout → **Go Package Layout** section.
3. Scheduler design (compute-next-run + persisted last-run; deterministic due-on-startup) → **Scheduler Design** section.
4. `pigparse_price` vs findings' `item_price` naming drift → **resolved: `pigparse_price` is authoritative** (it's the name in `00001_init.sql`); `item_price` is findings-doc drift. See Column Reconciliation §2.

### Deferred Ideas (OUT OF SCOPE)

- Eviction / stale-character archive jobs (`weeklyEvictionArchive`, `weeklyStaleCharArchive`) — P15.
- Sheet→DB one-time backfill + shadow-soak cutover — P16.
- Optional inventory history (append-only snapshots) — parked, not Core Value.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| ENRICH-10 | Daily PigParse price pull (server=1=Blue) as in-process scheduled job, reusing the existing parser + `politeFetch` controls (UA, ETag/If-Modified-Since, backoff). | Parser port (`parseToRows`); politeFetch port; `pigparse_price` upsert on `item_id`; truncation-guard-as-log; `00003` adds 8 price columns. |
| ENRICH-11 | Weekly P1999 wiki scrape (per-item summaries, per-class spell lists, Velious gear tiers, quest items) as in-process scheduled job with same politeness controls. | 3 parser ports (`parseItempage`, `parseClassPage`, `parseGearTierPage`); single uninterrupted run (no cursor); upserts on `item_master`/`wiki_spells`/`wiki_gear_tier`/`quest_items` natural keys. |

## Project Constraints (from CLAUDE.md)

The planner MUST honor these — they have the same authority as locked decisions:

- **Go 1.24** (note: `go.mod` declares `go 1.25.7` — the toolchain is ahead of the CLAUDE.md floor; use the repo's actual `go 1.25.7` directive, it is the source of truth for what compiles here).
- Backend is **HAND-ROLLED** `net/http` + `modernc.org/sqlite` (pure-Go, **no cgo**) + `goose`. No PocketBase (removed in 11-05).
- Structured logging is Go **`slog` JSON to stdout** (journald). Set up in `internal/backendsrv/logging`. The Apps Script side uses a `log(level, op, fields)` JSON helper — the Go port's `slog.Info(op, "field", val…)` is the equivalent.
- PigParse: `GET https://pigparse.azurewebsites.net/api/item/getall/1` (server=1=Blue), **daily**. TYPED API — never HTML-scrape.
- P1999 MediaWiki: `https://wiki.project1999.com/api.php?action=parse&prop=wikitext`, **weekly**. TYPED API — never HTML-scrape.
- **No external scheduler lib** preferred. Forward-only migrations, extend-only schema. **Never edit a shipped migration** (`00001`/`00002`).
- `WatcherMaxSchemaVersion` handshake is RETIRED for v2.0 (forward-only goose + API version). This phase does NOT touch any `_meta.schema_version` machinery — that's Apps Script only.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Daily PigParse price pull | Backend scheduler (`enrich/jobs`) | Backend HTTP client (`politefetch`) | Outbound fetch + parse + upsert; runs server-side on a timer. No UI, no watcher involvement. |
| Weekly wiki scrape (items/spells/gear/quests) | Backend scheduler (`enrich/jobs`) | Backend HTTP client (`politefetch`) | Same — server-side scheduled job. The watcher and frontend never participate. |
| Pure parsing (4 parsers) | Backend `enrich` package | — | Host-agnostic string→struct; no I/O. Same tier as the jobs that call them. |
| Politeness state (ETag) | Backend store (`etag_cache` table) | — | DB is the single durable store; replaces PropertiesService. |
| Job last-run bookkeeping | Backend store (`job_run` table) | Backend scheduler | DB persistence is what makes restart-safety deterministic. |
| Dimension data persistence | Backend store (5 dimension tables) | — | The single-writer SQLite DB owns all dimension rows. |

**Note (NOT this tier):** Reading dimension data back out into the 4 views is **P14** (BACKEND-05 read API + WEB-*). Phase 12 only WRITES the dimension tables. The watcher's `internal/parse` (inventory/spellbook, already UTF-8 per 11-03) is a DISTINCT parser — do not conflate it with these enrichment parsers.

---

## Column Reconciliation (HIGHEST PRIORITY — D-6)

This is the load-bearing deliverable. Below is a column-by-column diff for each of the 5 dimension tables: **current `00001_init.sql` columns** vs **what the ported parser emits** vs **the gap**. Source DDL is `internal/backendsrv/migrations/00001_init.sql` lines 59–63 (verified, quoted verbatim). Parser output shapes are from the TS type files (verified).

### Key finding: the 11-02 tables are already "rich"

The 11-02 author copied the dimension DDL from RESEARCH §"Migration SQL Sketch", which had already absorbed most of the findings-doc §1.3–1.4 column proposals. So `item_master`, `wiki_spells`, `wiki_gear_tier`, and `quest_items` need **zero new columns** — every field the parser emits already has a home. **Only `pigparse_price` is short** the price-history columns. This makes `00003` small and low-risk.

### §1 — `item_master` (wiki item summary; written by `parseItempage` job)

Current DDL: `item_master(item_id PK, name, wiki_summary, wiki_url, slot, is_quest_item DEFAULT 0, wikitext_sha1, last_refreshed)`

The Sheet trigger (`refreshWikiItems.ts` `ITEM_MASTER_HEADERS`) wrote exactly: `item_id, name, wiki_summary, wiki_url, slot, is_quest_item, last_refreshed, wikitext_sha1`.

| Parser field (`ParsedWikiItem`) | Maps to column | Present in 00001? | Gap |
|---|---|---|---|
| `itemname` | `name` | ✅ | — |
| `summary` | `wiki_summary` | ✅ | — |
| `wiki_url` | `wiki_url` | ✅ | — |
| `slot` (nullable) | `slot` | ✅ | — |
| `is_quest_item` (bool) | `is_quest_item` (INTEGER 0/1) | ✅ | — |
| `wikitext_sha1` | `wikitext_sha1` | ✅ | — |
| (job clock) | `last_refreshed` | ✅ | — |
| `item_id` (caller-supplied from inv union) | `item_id` PK | ✅ | — |

**Gap: NONE.** `parseItempage` also emits `is_no_drop`, `is_lore`, `is_magic`, `is_temporary`, `classes[]`, `ac`, `weight`, `effect` — but the Sheet never persisted these (the trigger dropped them) and they are not in `item_master`. **Do NOT add them** (D-8 scope guard + parity-with-Sheet is the acceptance test; adding columns the Sheet never had breaks the byte-for-byte parity goal). They are recoverable later if a view needs them.

### §2 — `pigparse_price` (PigParse daily; written by `parseToRows` job) — THE ONLY TABLE WITH A GAP

Current DDL: `pigparse_price(item_id PK, name, current_avg REAL, blue_volume INTEGER, last_seen TEXT, direction TEXT, last_refreshed TEXT)`

**Naming drift resolved:** the real table is **`pigparse_price`** (the name in `00001_init.sql`). The findings doc §1.3 calls it `item_price` — that name does NOT exist in the DB and must be ignored. Map the findings' proposed `item_price` columns onto `pigparse_price`.

The Sheet trigger (`refreshPigparse.ts` `PIGPARSE_HEADERS` + `buildRow`) wrote 15 columns: `item_id, name, current_avg(=a30), last_seen, blue_volume(=t30), last_refreshed, direction, t30, a30, t60, a60, t6m, a6m, ty, ay`. The `current_avg`/`blue_volume` are *aliases* (denormalized duplicates of `a30`/`t30`) kept for the Sheet's VLOOKUP convenience.

| Parser field (`PigparseRowRaw`) | Maps to column | Present in 00001? | Gap → add in 00003 |
|---|---|---|---|
| `i` | `item_id` PK | ✅ | — |
| `n` | `name` | ✅ | — |
| `a30` | `current_avg` (alias) | ✅ | — |
| `l` | `last_seen` | ✅ | — |
| `t30` | `blue_volume` (alias) | ✅ | — |
| `t` (0/1/2 direction) | `direction` | ✅ | — (store as TEXT — see note) |
| (job clock) | `last_refreshed` | ✅ | — |
| `t30` | `t30` (canonical) | ❌ | **ADD `t30 INTEGER`** |
| `a30` | `a30` | ❌ | **ADD `a30 REAL`** |
| `t60` | `t60` | ❌ | **ADD `t60 INTEGER`** |
| `a60` | `a60` | ❌ | **ADD `a60 REAL`** |
| `t6m` | `t6m` | ❌ | **ADD `t6m INTEGER`** |
| `a6m` | `a6m` | ❌ | **ADD `a6m REAL`** |
| `ty` | `ty` | ❌ | **ADD `ty INTEGER`** |
| `ay` | `ay` | ❌ | **ADD `ay REAL`** |

**Gap: 8 columns** (`t30,a30,t60,a60,t6m,a6m,ty,ay`). The parser also has `t90,a90` (90-day) which the Sheet `buildRow` did NOT write — **omit them** for Sheet parity (or add them too if the planner wants completeness; recommend OMIT to match the Sheet exactly per D-7).

**`direction` representation note:** the parser's `t` is `0|1|2` (WTS/WTB/BOTH). The Sheet's `buildRow` wrote the raw integer `r.t` into the TEXT `direction` column (so the cell shows `0`/`1`/`2`). For byte-parity keep it identical: write the integer's string form, or store the int — but `direction` is declared `TEXT`. Recommend: write `strconv.Itoa(int(t))` (matches the Sheet's stringified value). The findings doc imagined `direction` as a "price trend" string, but the actual Sheet semantics is the WTS/WTB flag — **follow the Sheet, not the findings doc.**

**⚠ PigParse natural-key subtlety (load-bearing for the upsert):** the PigParse response has **TWO rows per `item_id`** — one with `t=0` (WTS/sell) and one with `t=1` (WTB/buy). Verified in the fixture: item `19450` ("10 Dose Ant's Potion") appears twice, t=0 and t=1. But `pigparse_price.item_id` is a **PRIMARY KEY** (one row per item). The Sheet's `buildRow` mapped **every** raw row to an output row, so the Sheet's `_pigparse` tab had duplicate `item_id`s across t=0/t=1. **This is a real semantic decision the planner must make** (see Open Questions Q1): either (a) the upsert on PK `item_id` means the LAST-written direction wins (last row for that id clobbers), or (b) the natural key should be `(item_id, direction)` not `item_id` alone. **Recommendation:** because the v1 Sheet kept both and downstream `view`/`bank` read `current_avg`/`blue_volume` (the a30/t30 aliases, direction-agnostic), and the PK is already `item_id`, the simplest Sheet-faithful behavior is to **upsert on `item_id` and let the WTS (t=0, sell) row win** — filter to `t===0` rows before upsert (sell price is what the tooltip wants), OR last-wins. Flag this for the planner to lock; it is the one genuine ambiguity in the port. Do NOT silently double-insert (PK violation).

### §3 — `wiki_spells` (per-class spell list; written by `parseClassPage` job)

Current DDL: `wiki_spells(id PK, class NOT NULL, level NOT NULL, spell_name NOT NULL, normalized_name NOT NULL, last_refreshed, UNIQUE(class, level, spell_name))`

Sheet trigger (`refreshWikiSpells.ts` `WIKI_SPELLS_HEADERS`) wrote: `class, level, spell_name, normalized_name, last_refreshed`.

| Parser field (`WikiSpellRow`) | Maps to column | Present? | Gap |
|---|---|---|---|
| `class` | `class` | ✅ | — |
| `level` | `level` | ✅ | — |
| `spell_name` | `spell_name` | ✅ | — |
| `normalized_name` | `normalized_name` | ✅ | — |
| `last_refreshed` | `last_refreshed` | ✅ | — |

**Gap: NONE.** Natural/conflict key = `UNIQUE(class, level, spell_name)` (already declared). ✅

### §4 — `wiki_gear_tier` (Velious gear tiers; written by `parseGearTierPage` job)

Current DDL: `wiki_gear_tier(id PK, tier NOT NULL, class NOT NULL, slot NOT NULL, item_id, item_name, rank, last_refreshed, UNIQUE(tier, class, slot, item_id))`

Sheet trigger (`refreshWikiGearTier.ts` `WIKI_GEAR_TIER_HEADERS`) wrote: `tier, class, slot, item_id, item_name, rank, last_refreshed`.

| Parser field (`WikiGearTierRow`) | Maps to column | Present? | Gap |
|---|---|---|---|
| `tier` (`'Velious Pre-Raid/Group'`/`'Velious Raiding'`/`'Iksar'`) | `tier` | ✅ | — |
| `class` | `class` | ✅ | — |
| `slot` | `slot` | ✅ | — |
| `item_id` (always `null` — wiki transclusions have no IDs) | `item_id` (nullable) | ✅ | — |
| `item_name` | `item_name` | ✅ | — |
| `rank` (1-based) | `rank` | ✅ | — |
| `last_refreshed` | `last_refreshed` | ✅ | — |

**Gap: NONE.** ⚠ **Conflict-key hazard:** the declared `UNIQUE(tier, class, slot, item_id)` includes `item_id`, which is **always NULL** from this parser. In SQLite, **NULLs are distinct in a UNIQUE constraint** — so `(Velious Raiding, WAR, Chest, NULL)` and a second `(Velious Raiding, WAR, Chest, NULL)` do NOT collide; `ON CONFLICT` will NOT fire and you get duplicate rows on every weekly run. The Sheet sidestepped this with a **full clear+rewrite** (`replaceAllWikiGearTier` deletes all rows then inserts). **Recommendation for the port:** mirror the Sheet — do a **full-table replace** for `wiki_gear_tier` inside one transaction (DELETE all + INSERT all), NOT a per-row upsert, because the natural key is `(tier, class, slot, item_name, rank)` not `item_id`. Alternatively, the planner could add a `UNIQUE(tier, class, slot, item_name)` to make upsert work — but full-replace is simpler and Sheet-faithful (only 2 source pages → ~1000 rows, trivial). See Upsert section.

### §5 — `quest_items` (per-item quest links; written by `parseItempage` job's `questLinks`)

Current DDL: `quest_items(id PK, item_id NOT NULL, quest_name NOT NULL, source_url, source, last_refreshed, UNIQUE(item_id, quest_name))`

Sheet trigger (`refreshWikiItems.ts` `QUEST_ITEMS_HEADERS`) wrote: `item_id, quest_name, source_url, last_refreshed, source`.

| Parser field (`WikiQuestItemLink`) | Maps to column | Present? | Gap |
|---|---|---|---|
| `item_id` (caller fills in) | `item_id` | ✅ | — |
| `quest_name` | `quest_name` | ✅ | — |
| `source` (`'in_game_flag'`/`'notes_link'`) | `source` | ✅ | — |
| derived URL (`wiki.project1999.com/<slug>`, empty for in_game_flag) | `source_url` | ✅ | — |
| `last_refreshed` | `last_refreshed` | ✅ | — |

**Gap: NONE.** Natural key = `UNIQUE(item_id, quest_name)` (declared, no NULLs in the key) → `ON CONFLICT(item_id, quest_name) DO UPDATE` works cleanly. ✅ The Sheet's `replaceQuestItemRowsForId` deleted-then-appended per `item_id`; the DB upsert per `(item_id, quest_name)` is the cleaner equivalent (per findings §1.4).

### Exact `00003` migration SQL

**Format verified against `00001_init.sql` + `embed.go`:** goose annotated SQL, `-- +goose Up` / `-- +goose Down`, `//go:embed *.sql` auto-includes any new `00003_*.sql` file co-located in `internal/backendsrv/migrations/`. Dialect string is `"sqlite3"` (NOT the `"sqlite"` driver name — see embed.go FOOT-GUN comment). **SQLite ADD COLUMN constraints** (verified, see Sources): one column per `ALTER TABLE` statement (no multi-column add); added columns may not be PRIMARY KEY/UNIQUE; NOT NULL adds need a non-NULL default. The 8 price columns are all nullable → no default needed.

Create file `internal/backendsrv/migrations/00003_enrich_columns.sql`:

```sql
-- +goose Up
-- Phase 12 (ENRICH-10/11). Forward-only; 00001/00002 are shipped and NOT edited.
-- SQLite permits only ONE column per ALTER TABLE ADD COLUMN; added columns are
-- nullable (no DEFAULT needed) and carry no UNIQUE/PK constraint.

-- §2 pigparse_price: the 8 canonical price-history columns parseToRows emits but
-- the empty 11-02 table lacks. (current_avg/blue_volume/direction already exist
-- as the Sheet's a30/t30 aliases + WTS-WTB flag.)
ALTER TABLE pigparse_price ADD COLUMN t30 INTEGER;
ALTER TABLE pigparse_price ADD COLUMN a30 REAL;
ALTER TABLE pigparse_price ADD COLUMN t60 INTEGER;
ALTER TABLE pigparse_price ADD COLUMN a60 REAL;
ALTER TABLE pigparse_price ADD COLUMN t6m INTEGER;
ALTER TABLE pigparse_price ADD COLUMN a6m REAL;
ALTER TABLE pigparse_price ADD COLUMN ty  INTEGER;
ALTER TABLE pigparse_price ADD COLUMN ay  REAL;

-- Scheduler last-run bookkeeping (D-2). One row per registered job; the
-- scheduler reads last_run_at on startup to decide "due if missed" and writes
-- it after each successful (or attempted) cycle. status/detail aid observability
-- (replaces the Sheet's _meta.last_error per-job).
CREATE TABLE job_run (
  job_name     TEXT PRIMARY KEY,           -- 'pigparse_daily' | 'wiki_weekly'
  last_run_at  TEXT,                        -- ISO 8601 UTC; NULL = never run
  last_status  TEXT,                        -- 'ok' | 'error' | 'skipped_unchanged'
  last_detail  TEXT,                        -- short diagnostic (row counts, error)
  updated_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

-- ETag/Last-Modified cache (D-3). Replaces PropertiesService ETag storage for
-- the 304 short-circuit. Keyed by the exact request URL.
CREATE TABLE etag_cache (
  url            TEXT PRIMARY KEY,
  etag           TEXT,                       -- value of the ETag response header
  last_modified  TEXT,                       -- value of the Last-Modified header
  fetched_at     TEXT NOT NULL DEFAULT (datetime('now'))
);

-- +goose Down
DROP TABLE etag_cache;
DROP TABLE job_run;
-- SQLite >=3.35 supports DROP COLUMN; modernc bundles a recent SQLite. If a
-- rollback is ever needed, drop the 8 added columns (forward-only in practice;
-- Down is courtesy only — this project never rolls back in prod, deploy=Up).
ALTER TABLE pigparse_price DROP COLUMN ay;
ALTER TABLE pigparse_price DROP COLUMN ty;
ALTER TABLE pigparse_price DROP COLUMN a6m;
ALTER TABLE pigparse_price DROP COLUMN t6m;
ALTER TABLE pigparse_price DROP COLUMN a60;
ALTER TABLE pigparse_price DROP COLUMN t60;
ALTER TABLE pigparse_price DROP COLUMN a30;
ALTER TABLE pigparse_price DROP COLUMN t30;
```

**Migration verification step for the plan:** after writing `00003`, `go test ./internal/backendsrv/...` must pass (the 11-02 `migrations`/`store` tests run `goose.Up` and assert table state; they'll exercise `00003` automatically). Add an assertion that `job_run` + `etag_cache` exist and that `pragma table_info(pigparse_price)` includes the 8 new columns.

---

## Go Package Layout (open item #2)

### Current `internal/backendsrv/` tree (verified by `ls`)

```
internal/backendsrv/
├── auth/         # MintCode/RevokeCode/ResolveToken (11-04) — bearer guard
├── ingest/       # POST /api/v1/ingest handler + Envelope (11-05)
├── logging/      # slog JSON-to-stdout Setup() (11-05)
├── migrations/   # goose embed + 00001_init.sql + 00002_audit.sql
├── scheduler/    # time.Ticker skeleton (11-05) — Start(ctx), 0 jobs
└── store/        # Open/DSN (modernc sqlite, SetMaxOpenConns(1)) + tx helpers (11-02/03)
```

(There is no `ingest/`-style subdir confusion — the watcher's `internal/parse` lives at repo-root `internal/parse`, fully separate from `internal/backendsrv`.)

### Recommended Phase 12 additions

```
internal/backendsrv/
├── enrich/                    # NEW — the 4 pure parsers (host-agnostic, no I/O)
│   ├── pigparse.go            # port of parseToRows + PigparseRowRaw + isValidRow/coerceRow
│   ├── pigparse_test.go       # loads __fixtures__/pigparse-getall-1.json
│   ├── wikiitem.go            # port of parseItempage + ParsedWikiItem + WikiQuestItemLink + helpers
│   ├── wikiitem_test.go       # loads wiki-parse-*.json fixtures
│   ├── wikispell.go           # port of parseClassPage + WikiSpellRow + normalizeSpellName
│   ├── wikispell_test.go      # loads wiki-class-*.json fixtures
│   ├── wikigear.go            # port of parseGearTierPage + WikiGearTierRow + Tier
│   ├── wikigear_test.go       # loads wiki-velious-*-gear.json fixtures
│   ├── eqconst.go             # port of CLASSES, CLASS_DISPLAY_TO_ABBREV, WIKI_SLOT_TO_INV_SLOTS
│   └── testdata/              # OR symlink/copy fixtures here (see Fixture Reuse)
│       └── *.json
│   └── politefetch/           # NEW — the net/http polite client (D-3)
│       ├── politefetch.go     # Fetch(url, opts) with UA/ETag/backoff/Retry-After
│       └── politefetch_test.go # httptest server driving 200/304/429/503/retry-after
└── enrich/jobs/               # NEW — orchestration (the I/O the triggers did)
    ├── pigparse.go            # daily job: fetch → parseToRows → upsert pigparse_price
    ├── wiki.go                # weekly job: items + spells + gear + quests, sequential
    ├── jobs_test.go           # end-to-end with a NewTestDB + httptest-served fixtures
    └── (store upsert helpers live in store/, called from here)
```

**Rationale:**
- `enrich/` holds the **pure** parsers — mirrors the TS `lib/` split (parsers are libs). Keeping them I/O-free keeps their tests fast + deterministic and makes byte-parity with the TS fixtures trivial.
- `enrich/politefetch/` is its own package because it's reusable policy code with its own test surface (httptest), exactly as `politeFetch.ts` had its own test file.
- `enrich/jobs/` holds the **orchestration** — the part that replaces `refresh*.ts`'s Sheets I/O with HTTP + DB. This is where `politefetch` + parsers + store upserts compose. The scheduler (`scheduler/`) calls into `enrich/jobs`.
- **Upsert SQL belongs in `store/`** (new methods like `store.UpsertPigparsePrices(tx, rows)`, `store.ReplaceWikiGearTier(tx, rows)`, `store.UpsertWikiSpells`, `store.UpsertItemMaster`, `store.UpsertQuestItems`, plus `store.GetJobRun`/`store.SetJobRun` and `store.GetETag`/`store.SetETag`). This follows the 11-05 "single-tested-SQL-path" rule (WARNING-3): the handler/jobs author NO inline DELETE/INSERT SQL; all SQL is exported, tested `store` methods. The planner must honor this convention.
- `scheduler/` keeps the loop but gains a job registry + `job_run` reads/writes; it imports `enrich/jobs`.

**Naming the parser package `enrich` (not `enrichment`/`parse`):** short, and avoids colliding with the watcher's `internal/parse`. The `jobs` subpackage avoids an import cycle (parsers don't import jobs; jobs import parsers + politefetch + store).

---

## Scheduler Design (open item #3)

### Current skeleton (verified — `scheduler/scheduler.go`)

`Start(ctx)` spawns `run(ctx)`: a `time.NewTicker(HeartbeatInterval=1h)` loop that `select`s on `ctx.Done()` vs `ticker.C`, logging a heartbeat, `0` jobs. It mirrors the watcher's `internal/heartbeat/heartbeat.go` ticker shape. The server calls `scheduler.Start(ctx)` in `runServe` (main.go:177) with the `signal.NotifyContext` root ctx.

### Design: a small job registry + DB-persisted last-run + a tick loop

No external scheduler lib (CLAUDE.md). Use a coarse ticker (e.g. every 5–15 min) that, on each tick, checks each registered job's "is it due?" against the persisted `job_run.last_run_at` and the job's cadence. This is the standard "poll-and-check" pattern that makes **due-on-startup-if-missed deterministic**: on process start the first tick (or an immediate check before the loop) sees `last_run_at` is older than the cadence window and runs the job.

```go
// enrich/jobs or scheduler — a Job is a named, cadenced unit of work.
type Job struct {
    Name     string                       // 'pigparse_daily' | 'wiki_weekly'
    Due      func(last time.Time, now time.Time) bool  // cadence predicate
    Run      func(ctx context.Context) error
    mu       sync.Mutex                   // per-job: two cycles can't overlap (LockService replacement)
}
```

**Cadence predicates (compute-next-run, no cron lib needed):**
- **PigParse daily:** `Due` returns true when `now.Sub(last) >= 24h` (or, to pin it near a wall-clock hour like the Sheet's 03:00 PT, when `last` is on a strictly earlier UTC calendar day AND `now.Hour()` is past the target hour). The simpler `>= 24h` is restart-robust and adequate at this scale — recommend it unless the planner wants wall-clock pinning.
- **Wiki weekly (Sunday):** `Due` returns true when `now.Weekday() == time.Sunday` AND `last` is before the start of the current Sunday (i.e. `now.Sub(last) >= ~6 days` AND it's Sunday). A tiny helper `startOfWeek(now)` (most recent Sunday 00:00 UTC) makes this exact: `Due = last.Before(startOfSundayUTC(now)) && now.Weekday()==time.Sunday`. This guarantees one run per Sunday and a missed Sunday (server was down) runs on the next tick after restart if still within Sunday, else the following Sunday — deterministic.

**The loop (replacing the heartbeat body):**
```
on Start:
  load each job's last_run_at from job_run (store.GetJobRun) — NULL => zero time => due
  ticker := NewTicker(checkInterval)  // e.g. 10*time.Minute
  immediate check pass (so a missed job runs right after restart, not after one interval)
  for { select ctx.Done -> return ; ticker.C -> for each job: if Due(last,now) { runJob } }

runJob(job):
  if !job.mu.TryLock() { log "overlap skipped"; return }   // per-job mutex
  defer job.mu.Unlock()
  err := job.Run(ctx)
  store.SetJobRun(job.Name, now, statusFrom(err), detail)  // persist AFTER the run
  last[job.Name] = now
```

**Restart safety (the core requirement):**
- `job_run.last_run_at` is the durable cursor. On restart the registry reloads it. The **immediate check pass** before entering the ticker loop means a job that was due while the process was down fires within seconds of startup — **no skip**.
- Because `SetJobRun` writes `last_run_at = now` after each run, a job can't **double-run**: once it runs, `last_run_at` advances and `Due` returns false until the next window. Even if the process restarts mid-window, `last_run_at` already reflects the completed run.
- **Write `last_run_at` even on error** (with `last_status='error'`) so a persistently-failing fetch doesn't hot-loop every tick — it retries on the next cadence window, not every 10 minutes. (Alternatively, only advance on success but add a cooldown; recommend advance-always for daily/weekly cadence since the politeFetch backoff already handles transient failures within a single run.)

**Per-job mutex + single-writer DB coordination:** `job.mu` (a `sync.Mutex`, `TryLock` to skip rather than queue) replaces `LockService.getDocumentLock()` — it ensures one PigParse cycle never overlaps another. The DB's `SetMaxOpenConns(1)` (11-02) already serializes all writes, so the mutex is about not launching a redundant *fetch+parse* cycle, not about DB safety. The two jobs (`pigparse_daily`, `wiki_weekly`) have separate mutexes and can run concurrently with each other (PigParse daily + wiki weekly could both be due on a Sunday) — their DB writes serialize through the single connection harmlessly (each upsert batch is a short tx).

**`checkInterval` choice:** 10 minutes is fine (a daily job fired up to 10 min late is irrelevant). Do NOT reuse the 1h `HeartbeatInterval` constant — replace it. Keep the loop's `ctx.Done()` clean-shutdown behavior verbatim (it's already correct and tested).

**`job_run` vs reusing a table:** `job_run` (new, in `00003`) is the right home — it's tiny, purpose-built, and keeps job state out of the dimension tables. Do NOT reuse `audit_log` (that's security events) or stuff job state into a KV `app_config` (there is no `app_config` table in this DB — the findings doc proposed one but 11-02 did NOT create it; don't add it for this phase, `job_run` is sufficient). Verified: `00001`+`00002` create no config/KV table.

---

## politeFetch Go Port Spec (D-3)

Source: `apps-script/src/lib/politeFetch.ts` + `politeFetch.test.ts` (both read verbatim). Every control below MUST carry over. Target: `net/http` with TLS verification on (default).

### Controls to port (complete enumeration)

| # | Control | TS source | Go port |
|---|---------|-----------|---------|
| 1 | **Identifying User-Agent** | `SquireBot/${VERSION} (+https://github.com/boejowen/SquireBot)`, default header on every request | Set `req.Header.Set("User-Agent", ua)` where `ua = "SquireBot/"+version+" (+https://github.com/boejowen/SquireBot)"`. See Version note below. |
| 2 | **If-None-Match (ETag)** | `if (opts.etag) headers['If-None-Match'] = opts.etag` | `if etag != "" { req.Header.Set("If-None-Match", etag) }` |
| 2b | **If-Modified-Since** | NOT in the TS (TS only does If-None-Match) | CONTEXT/ROADMAP SC-3 names "If-Modified-Since" — ADD it: `if lastMod != "" { req.Header.Set("If-Modified-Since", lastMod) }`. Pull both ETag + Last-Modified from `etag_cache`. (This is a small ADD beyond verbatim — the wiki sends Last-Modified; sending both maximizes 304 hits. Acceptable since SC-3 explicitly lists If-Modified-Since.) |
| 3 | **304 short-circuit** | `if (status === 304) return {ok:true, body:'', fromCache:true, etag:opts.etag}` | On `resp.StatusCode == 304` return a `FromCache:true` result with empty body; the job then SKIPS re-parsing/re-writing that resource (unchanged). |
| 4 | **200 captures ETag** | reads `allHeaders['ETag']`, returns it | Read `resp.Header.Get("ETag")` + `resp.Header.Get("Last-Modified")`; job persists them to `etag_cache` after a successful parse. |
| 5 | **Exponential backoff** | `RETRY_DELAYS_MS = [2000,4000,8000,16000,32000]`, retried on 429/503/504 | `retryDelays := []time.Duration{2*s,4*s,8*s,16*s,32*s}`; loop `for attempt := 0; attempt <= len(retryDelays); attempt++`. |
| 6 | **Retry-After honored** | parses `Retry-After` seconds (0–600 clamp), overrides the schedule delay | Parse `resp.Header.Get("Retry-After")` as int seconds, clamp 0–600, use it instead of `retryDelays[attempt]` when present. (TS only handles delta-seconds form, not HTTP-date — match that: integer seconds only.) |
| 7 | **Retry statuses** | `RETRY_STATUSES = {429, 503, 504}` | `retryStatuses := map[int]bool{429:true, 503:true, 504:true}`. |
| 8 | **Non-retriable 4xx/5xx surface immediately** | 404/401/etc. → return error, `retriesUsed=0`, NO sleep | On a status not in {200,304,429,503,504} return an error immediately (no retry). |
| 9 | **Network error → retry** | `UrlFetchApp` throw → retry all 5 | A transport error (`client.Do` returns err) → retry up to len(retryDelays), then return error. |
| 10 | **1-second inter-request wiki sleep** | `INTER_REQUEST_SLEEP_MS = 1000` — **in the trigger, NOT in politeFetch** (`Utilities.sleep` between wiki fetches) | Keep this OUT of the client (the TS comment is explicit: politeFetch does NOT sleep between successful calls). The **wiki job** sleeps `time.Sleep(1*time.Second)` between each page fetch. PigParse is a single request → no inter-request sleep. |
| 11 | **validateHttpsCertificates: true** | explicit in fetchOpts | Go's default `http.Client` verifies TLS — do NOT set `InsecureSkipVerify`. (V-threat: TLS verification on outbound.) |
| 12 | **followRedirects: true** | explicit | Go's default client follows up to 10 redirects. Adequate; the wiki API uses `redirects=true` query param for page redirects anyway. |

### Return shape (Go equivalent of `FetchResult`)

```go
type FetchResult struct {
    OK           bool
    Status       int
    Body         []byte            // empty on 304
    ETag         string
    LastModified string
    FromCache    bool              // true on 304
    RetriesUsed  int
    Err          error             // non-nil when !OK
}
```

### Where state moves

- **ETag/Last-Modified:** `PropertiesService` → the new `etag_cache(url, etag, last_modified, fetched_at)` table (`00003`). The job reads `store.GetETag(url)` before the fetch (passes etag+lastMod into the client) and `store.SetETag(url, etag, lastMod)` after a successful 200+parse. On 304 it leaves the cache row untouched and skips the write.
- **`CacheService` response cache → DROPPED** (D-3 confirmed). The 304/ETag path IS the politeness control; at ~12 users the load is trivial. Do not port any response-body caching.

### Body-size hardening (security — NOT in the TS, but REQUIRED for the Go port)

The TS `getContentText()` had Apps Script's implicit response limits. Go's `io.ReadAll(resp.Body)` is **unbounded** → a malicious/runaway response could OOM the small VPS. **Wrap with `io.LimitReader`** (e.g. `io.LimitReader(resp.Body, maxResponseBytes)` with `maxResponseBytes` ~16 MB — the real PigParse fixture is 1.27 MB, wiki pages <100 KB, so 16 MB is generous headroom). This mirrors the ingest handler's `http.MaxBytesReader(1<<20)` discipline (11-05). See Security Domain.

### Version source for the User-Agent (open detail)

The Apps Script side injects `__VERSION__` at esbuild time. The Go **watcher** uses `main.Version` (a `-ldflags -X` string, default `"0.1.0-dev"` — `cmd/squirebot/build_constants.go:29`). The Go **backend** (`cmd/squirebot-server/main.go`) currently has **NO version var**. **Recommendation for the planner:** add a `var Version = "0.0.0-dev"` to the backend main (or a small `internal/backendsrv/buildinfo` package) settable via `-ldflags "-X main.Version=…"`, and thread it into the politefetch client's UA. A hardcoded fallback (`"SquireBot/dev (+https://github.com/boejowen/SquireBot)"`) is acceptable for v1 of this phase — the UA just needs to be identifying + contactable, which the github URL satisfies. This is a minor, self-contained decision; flag it but don't block on it.

---

## Upsert / Conflict-Key Per Table + Truncation Guard (D-4)

All writes use `INSERT … ON CONFLICT(<natural key>) DO UPDATE SET …` (modernc.org/sqlite supports SQLite UPSERT, SQLite ≥ 3.24). Exception: `wiki_gear_tier` (see below). All upserts go through tested `store` methods (single-SQL-path rule).

| Table | Natural / conflict key | Write strategy | Notes |
|-------|------------------------|----------------|-------|
| `pigparse_price` | `item_id` (PK) | `ON CONFLICT(item_id) DO UPDATE` | ⚠ Two raw rows per id (t=0/t=1) — pick WTS (t=0) or last-wins (see §2 + Q1). Per-row upsert means a truncated response updates what it got, leaves the rest (D-4). |
| `item_master` | `item_id` (PK) | `ON CONFLICT(item_id) DO UPDATE` | SHA-1 short-circuit: skip the upsert when `wikitext_sha1` is unchanged (mirrors the Sheet's `readItemMasterSha` early-return → counts as "unchanged"). |
| `wiki_spells` | `UNIQUE(class, level, spell_name)` | `ON CONFLICT(class,level,spell_name) DO UPDATE` | The Sheet did per-class full-replace; per-key upsert is cleaner. But stale rows (a spell removed from the wiki) won't be deleted by upsert alone → optionally DELETE-then-upsert per class to match the Sheet's full-replace-per-class exactly. Recommend per-class replace (DELETE WHERE class=? + INSERT) inside one tx for parity. |
| `wiki_gear_tier` | (declared `UNIQUE(tier,class,slot,item_id)` is BROKEN — item_id always NULL) | **Full-table replace** (DELETE all + INSERT all in one tx) | NULLs are distinct in SQLite UNIQUE → upsert never fires → duplicates. Mirror the Sheet's `replaceAllWikiGearTier`. Only ~1000 rows from 2 pages. |
| `quest_items` | `UNIQUE(item_id, quest_name)` | `ON CONFLICT(item_id,quest_name) DO UPDATE` | Clean key, no NULLs. The Sheet's per-item-id delete+append → per-key upsert. Stale-quest cleanup: optionally DELETE WHERE item_id=? before upserting that item's links (matches `replaceQuestItemRowsForId`). |

**Truncation guard (D-4) — as a LOG, never an abort:**
- The Sheet's `refreshPigparse` compared `rows.length` to `last_pigparse_row_count * 0.90` and **aborted** (refused to write) on truncation.
- For the DB port: because the upsert is graceful (updates what it got, leaves the rest), the guard is **no longer a hard gate**. Compute it and **log a warning** if `today_rows < 0.90 * last_known_rows` (`slog.Warn("pigparse truncation guard", "today", n, "last", last)`), but **proceed with the upsert anyway**. Store the row count in `job_run.last_detail` (or a dedicated counter) to compute the ratio next run. This is the explicit D-4 + findings §2.3 instruction: "Keep the truncation guard as a belt-and-braces sanity log either way."

---

## Fixture Reuse (D-1)

All fixtures in `apps-script/src/__fixtures__/` are JSON and load directly into Go tests. **Confirmed reusable.** Two shapes:

### Shape A — PigParse: flat array, parser-ready

| Fixture | Size | Use |
|---------|------|-----|
| `pigparse-getall-1.json` | 1.27 MB, ~7240 rows | `parseToRows` Go test. It's a raw `[{i,t,n,l,tc,ta,t30,a30,…}]` array — `os.ReadFile` → pass the bytes/string straight into the Go `ParseToRows([]byte)`. Contains the t=0/t=1 duplicate-id case (item 19450) — good for testing the natural-key decision. |
| `pigparse-swagger-v1.json` | 22 KB | Reference only (the API's OpenAPI doc) — documents the `ItemSummary` enum (`t` = 0 WTS / 1 WTB / 2 BOTH). Not a parser input. |

### Shape B — Wiki: MediaWiki `action=parse` envelope (one extraction step needed)

The wiki fixtures are the full API response `{"parse":{"title":"…","wikitext":{"*":"<wikitext>"}}}`. The parsers consume the **inner wikitext string**, not the envelope. So Go wiki-parser tests must first `json.Unmarshal` and pull `parse.wikitext["*"]` (and `parse.title`), then call the parser — exactly as the triggers do (`json.parse?.wikitext?.['*']`). Trivial helper in the test.

| Fixture | Parser | Covers |
|---------|--------|--------|
| `wiki-parse-cloth-cap.json` | `parseItempage` | basic item |
| `wiki-parse-pearl.json` | `parseItempage` | simple/no-stats item |
| `wiki-parse-cloak-of-flames.json` | `parseItempage` | item with effect + flags |
| `wiki-parse-fungus-covered-scale-tunic.json` | `parseItempage` | quest-item + classes |
| `wiki-parse-fungi-tunic-redirect.json` | `parseItempage` | redirect handling (93 bytes — the redirect stub) |
| `wiki-class-necromancer.json` | `parseClassPage` | pure caster, `{{SpellRow}}` variant, many levels |
| `wiki-class-paladin.json` | `parseClassPage` | hybrid, `{{SpellRow}}` |
| `wiki-class-warrior.json` | `parseClassPage` | degenerate no-spells (0 rows expected) |
| `wiki-velious-preraid-gear.json` | `parseGearTierPage` | Pre-Raid + Iksar-tagged items |
| `wiki-velious-raiding-gear.json` | `parseGearTierPage` | Raiding tier |

**Bard `{{SongRow}}` inline-level variant:** there is NO bard fixture in `__fixtures__/` (the TS parser handles it via the inline-level fallback, tested with synthetic strings in `wiki-spell-parser.test.ts`). The Go port should replicate those synthetic-string test cases for the bard fallback — read `apps-script/src/__tests__/wiki-spell-parser.test.ts` for the exact synthetic inputs (not requested in this research pass but the planner should pull them).

**Format adaptation for Go:** copy the fixtures into `internal/backendsrv/enrich/testdata/` (Go's idiomatic test-fixture dir; `go test` ignores `testdata/`). Do NOT try to `//go:embed` across the repo into `apps-script/` — copy them. They're static captures (won't drift); a one-time copy is correct. The byte-parity check is: same wikitext/JSON input → assert the Go parser emits the same field values the TS parser does (the TS tests already encode the expected values; transliterate the assertions).

---

## What NOT to Port (D-5 deletions) + Scope Guard (D-8)

**Delete / do-not-port (Apps-Script-only machinery):**

| Apps Script construct | Where | Why it dies |
|-----------------------|-------|-------------|
| 6-minute-cap **resumable cursor** | `refreshWikiItems.ts` (`CURSOR_KEY`, `checkpoint()`, `freshState()`, `RESUME_DELAY_MS`, self-`ScriptApp.newTrigger`) | A backend job has no execution cap → the weekly scrape is ONE uninterrupted run. (Also in `refreshWikiSpells.ts` + `refreshWikiGearTier.ts` — all three carry the cursor pattern "for consistency"; drop it in all three.) |
| `monitorCellCount` (10M-cell watchdog) | (Sheets-specific) | No cells in a DB. |
| `weeklySchemaHealthcheck` (expected-tab watchdog) | (Sheets-specific) | No tabs in a DB; goose owns schema. |
| `LockService.getDocumentLock()` | all 4 triggers | → single-writer DB (`SetMaxOpenConns(1)`) + per-job `sync.Mutex`. |
| `PropertiesService` (cursor + ETag storage) | triggers + politeFetch | ETag → `etag_cache` table; cursor → deleted. |
| `CacheService` response cache | (politeFetch ecosystem) | Dropped (D-3). |
| `_meta`/`_status` error/refresh-timestamp writes (`writeMetaRow`, `writeError`, `clearError`) | all 4 triggers | → `slog` structured logs + `job_run.last_status`/`last_detail`. |
| Post-run `buildSpellCheck()` / `buildGearCheck()` calls | `refreshWikiSpells.ts` / `refreshWikiGearTier.ts` | Those build Sheet VIEW tabs — P14 (the read API/frontend) owns views, not P12. Do NOT port the view rebuild. |
| `Utilities.computeDigest(SHA_1)` + signed-byte fix-up | `wiki-parser.ts` `computeSha1Hex` | → `crypto/sha1` + `hex.EncodeToString` (Go gives unsigned bytes directly — no `b<0?b+256` fix-up needed). Behavior identical (lowercase hex SHA-1 of the UTF-8 wikitext). |

**Scope guard (D-8) — REJECT if seen in any plan:**
- `weeklyEvictionArchive` / `weeklyStaleCharArchive` (eviction/stale-archive jobs) → **P15**, not here.
- Any Sheet→DB **backfill** or **shadow-soak cutover** → **P16**, not here.
- Building/reading the 4 VIEW tabs (`view`/`gear_check`/`spell_check`/`bank`) or a read API → **P14**, not here. Phase 12 only WRITES dimension tables.
- Adding parser fields the Sheet never persisted (e.g. `item_master.ac`/`weight`/`effect`/`classes`/`is_no_drop`) → out of scope; breaks the D-7 parity test. Persist exactly the columns the Sheet wrote.

---

## Parity Check Method (D-7)

After deploying the `00003` migration + both jobs and triggering one daily + one weekly cycle (the jobs can be invoked on-demand for the check, e.g. a `squirebot-server run-job pigparse|wiki` CLI subcommand the planner may add for testing — recommended, parallels the Sheet's manual "Refresh … Now" menu items), spot-check backend rows vs the live Sheet's dimension tabs:

1. **PigParse:** pick ~10 well-known item_ids (e.g. Cloak of Flames, Fungi Tunic). Compare backend `pigparse_price.current_avg`/`t30`/`a30` against the Sheet's `_pigparse` `current_avg`/`t30`/`a30` for the same item_id. Expect equality (same source API, same parser). Compare total row count (`SELECT count(*) FROM pigparse_price` vs the Sheet `_pigparse` row count — adjust for the t=0/t=1 dedup decision: if the backend dedups to one row per id, it'll have FEWER rows than the Sheet's dup-keeping tab; that's expected and should be noted, not flagged as a failure).
2. **item_master:** compare `wiki_summary`/`slot`/`is_quest_item`/`wikitext_sha1` for the same ~10 item_ids against `_item_master`. SHA-1 equality is the strongest parity signal (same wikitext → same digest).
3. **wiki_spells:** for 2–3 classes (Necromancer, Paladin, Warrior=empty), compare `(class, level, spell_name)` row sets against `_wiki_spells`.
4. **wiki_gear_tier:** compare `(tier, class, slot, item_name, rank)` row sets for a class against `_wiki_gear_tier`; verify Iksar-tagged items carry `tier='Iksar'`.
5. **quest_items:** compare `(item_id, quest_name, source)` against `_quest_items` for a few quest items.

A small SQL script (`SELECT … ORDER BY` per table) + eyeballing against the Sheet tabs is sufficient at this scale. **This parity check IS the acceptance proof — it is not a separate feature** (D-7). Note the live Sheet may be stale (the guild is dark on it per the OAuth incident) but its last-good dimension data is still the reference snapshot.

---

## Common Pitfalls

### Pitfall 1: `wiki_gear_tier` UNIQUE-on-NULL duplicate explosion
**What goes wrong:** `INSERT … ON CONFLICT(tier,class,slot,item_id) DO UPDATE` silently inserts duplicates on every weekly run because `item_id` is always NULL and SQLite treats NULLs as distinct in UNIQUE constraints — the conflict never fires.
**Why it happens:** wiki transclusions don't expose item IDs; the parser emits `item_id=null` for every gear-tier row. The declared UNIQUE key (copied from the findings doc) assumed IDs would be present.
**How to avoid:** Use **full-table replace** (DELETE all + INSERT all in one tx) for `wiki_gear_tier`, mirroring the Sheet's `replaceAllWikiGearTier`. Don't rely on the broken UNIQUE for upsert.
**Warning signs:** row count of `wiki_gear_tier` grows by ~1000 every Sunday.

### Pitfall 2: PigParse PK collision on t=0/t=1 duplicate item_ids
**What goes wrong:** A naive "insert every parsed row" hits a PRIMARY KEY violation on `pigparse_price.item_id` because each item appears twice (WTS + WTB).
**Why it happens:** The PigParse `getall` response has one row per (item, direction); the table has one PK row per item.
**How to avoid:** Decide the natural-key semantics up front (Q1): filter to WTS (t=0) before upsert, OR upsert with last-wins, OR change the key to `(item_id, direction)`. Recommend WTS-only or last-wins for Sheet fidelity.
**Warning signs:** `UNIQUE constraint failed: pigparse_price.item_id` on the first daily run.

### Pitfall 3: Multi-column `ALTER TABLE ADD COLUMN` in one statement
**What goes wrong:** `ALTER TABLE pigparse_price ADD COLUMN (t30 INTEGER, a30 REAL, …)` is a syntax error in SQLite.
**Why it happens:** SQLite (unlike Postgres) permits only ONE column per ALTER TABLE.
**How to avoid:** One `ALTER TABLE … ADD COLUMN` statement per column (see the `00003` SQL above — 8 separate statements).
**Warning signs:** goose migration fails at startup with a SQLite parse error.

### Pitfall 4: goose dialect string vs driver name
**What goes wrong:** Passing `"sqlite"` to `goose.SetDialect` yields "unknown dialect"; opening the driver as `"sqlite3"` yields "unknown driver."
**Why it happens:** modernc registers the `database/sql` driver as `"sqlite"`; goose's dialect is `"sqlite3"`. They differ on purpose.
**How to avoid:** `00003` is auto-included by the existing `//go:embed *.sql` + `RunMigrations` (which already calls `SetDialect("sqlite3")`). Don't touch `embed.go`; just drop the new `.sql` file in the `migrations/` dir. (Documented in `embed.go` as "FOOT-GUN (RESEARCH Pitfall 3)".)

### Pitfall 5: Unbounded `io.ReadAll` on the HTTP response
**What goes wrong:** A huge/runaway response from a community server (or a redirect to something unexpected) is read fully into memory → OOM on the small VPS.
**Why it happens:** `io.ReadAll(resp.Body)` has no size limit; the TS `getContentText()` had Apps Script's implicit cap.
**How to avoid:** Wrap with `io.LimitReader(resp.Body, ~16MB)` in the politefetch client (mirrors the ingest handler's `MaxBytesReader(1<<20)`).
**Warning signs:** memory spike during a fetch; the VPS OOM-kills the process.

### Pitfall 6: Re-parsing on 304
**What goes wrong:** The job re-parses an empty body on a 304 response and writes garbage/empty data over good dimension rows.
**Why it happens:** A 304 returns `Body=""`; feeding that to a parser yields zero rows; a naive full-replace then wipes the table.
**How to avoid:** On `FromCache==true` (304), SKIP parsing and SKIP the DB write for that resource entirely (it's unchanged). The SHA-1 short-circuit in `parseItempage`'s job is the per-item equivalent.
**Warning signs:** dimension tables go empty after a week of "successful" runs.

---

## Code Examples (verified patterns from this repo)

### goose migration file shape (from `00002_audit.sql`, verified)
```sql
-- +goose Up
CREATE TABLE job_run ( job_name TEXT PRIMARY KEY, last_run_at TEXT, … );
-- +goose Down
DROP TABLE job_run;
```

### modernc SQLite UPSERT (SQLite ≥3.24, supported by modernc)
```go
// store.UpsertPigparsePrice (one row; called in a loop inside one tx)
const q = `INSERT INTO pigparse_price
  (item_id, name, current_avg, blue_volume, last_seen, direction,
   t30, a30, t60, a60, t6m, a6m, ty, ay, last_refreshed)
 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
 ON CONFLICT(item_id) DO UPDATE SET
   name=excluded.name, current_avg=excluded.current_avg,
   blue_volume=excluded.blue_volume, last_seen=excluded.last_seen,
   direction=excluded.direction, t30=excluded.t30, a30=excluded.a30,
   t60=excluded.t60, a60=excluded.a60, t6m=excluded.t6m, a6m=excluded.a6m,
   ty=excluded.ty, ay=excluded.ay, last_refreshed=excluded.last_refreshed`
```

### SHA-1 (replaces `Utilities.computeDigest`)
```go
import ("crypto/sha1"; "encoding/hex")
func sha1Hex(s string) string { h := sha1.Sum([]byte(s)); return hex.EncodeToString(h[:]) }
// No signed-byte fix-up needed (Go bytes are unsigned). Lowercase hex, identical to the TS output.
```

### politefetch retry skeleton (transliterated from politeFetch.ts)
```go
retryDelays := []time.Duration{2*time.Second,4*time.Second,8*time.Second,16*time.Second,32*time.Second}
for attempt := 0; attempt <= len(retryDelays); attempt++ {
    resp, err := client.Do(req)
    if err != nil { /* network: retry if attempt<len, else return err */ }
    switch {
    case resp.StatusCode == 200: /* read (LimitReader), capture ETag/Last-Modified, return OK */
    case resp.StatusCode == 304: /* return FromCache:true, empty body */
    case resp.StatusCode==429||resp.StatusCode==503||resp.StatusCode==504:
        if attempt>=len(retryDelays) { return errExhausted }
        wait := retryDelays[attempt]
        if ra := parseRetryAfter(resp.Header.Get("Retry-After")); ra>0 { wait = ra } // clamp 0..600s
        time.Sleep(wait); continue
    default: return errNonRetriable(resp.StatusCode) // 404/401/etc — no sleep, immediate
    }
}
```

---

## Runtime State Inventory

> This is a code-migration phase (Apps Script → Go) that ALSO introduces new DB tables. It is not a rename, but the "what runtime state exists" lens still surfaces two real items.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | The 5 dimension tables in the live SQLite DB on the Hetzner VPS are EMPTY (11-02 created them; nothing populates them yet). The live Google Sheet's `_item_master`/`_pigparse`/`_wiki_spells`/`_wiki_gear_tier`/`_quest_items` tabs hold the last-good dimension data (reference for the D-7 parity check, NOT migrated here). | New jobs self-populate the DB tables; no data migration in P12 (backfill is P16). |
| Live service config | The Apps Script project has time-driven triggers (daily 03:00 PT PigParse, weekly Sun wiki) registered in Google's trigger system — NOT in git, set at `installTriggers()` time. These KEEP RUNNING into the dead Sheet until P16 decommission. | None in P12. The backend's new jobs run in parallel (shadow); the Apps Script triggers are retired in P16. Do NOT disable them here. |
| OS-registered state | None new. The systemd unit (`squirebot-server serve`, 11-06) already runs the binary that hosts the scheduler — the new jobs ride inside the existing process. No new systemd timers/cron (the scheduler is in-process per BACKEND-01). | None — deploy = drop the new binary + restart (D-10); the new jobs start with the process. |
| Secrets/env vars | None. PigParse + wiki are public unauthenticated APIs (no API key). The only secret in the backend is the guild-code hash, untouched here. The politefetch User-Agent is public-by-design. | None. |
| Build artifacts | The backend binary is rebuilt + redeployed (carries the new `enrich`/`jobs`/`politefetch` packages + `00003` embedded). The `00003_*.sql` is compiled into the binary via the existing `//go:embed *.sql`. | Rebuild + redeploy the static linux/amd64 binary; goose.Up applies `00003` on startup. |

**Canonical question — "after the new binary deploys, what runtime state still has the old behavior?":** the Apps Script triggers (intentionally, until P16). Everything else (the scheduler, the dimension tables) is owned by the redeployed binary and updated atomically by the restart + goose.Up.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | building + `go test ./...` | ✓ | `go 1.25.7` (go.mod) | — |
| `modernc.org/sqlite` | DB + migrations | ✓ (direct dep, 11-02) | bundled SQLite ≥3.45 (supports UPSERT + DROP COLUMN) | — |
| `github.com/pressly/goose/v3` | migrations | ✓ (direct dep, 11-02) | — | — |
| `net/http` (stdlib) | politefetch | ✓ | stdlib | — |
| PigParse API (`pigparse.azurewebsites.net`) | ENRICH-10 runtime | ✓ (public, reachable; fixture captured 2026-05-09) | `/api/item/getall/1` | Tests use the captured fixture (no live call in `go test`). |
| P1999 wiki API (`wiki.project1999.com/api.php`) | ENRICH-11 runtime | ✓ (public; fixtures captured 2026-05-10) | `action=parse&prop=wikitext` | Tests use captured fixtures. |
| Live Google Sheet (read) | D-7 parity reference | ✓ but auth-walled for the guild | dimension tabs | Use the last-known dimension values; or read via maintainer's own access for the spot-check. |

**No external dependency blocks development or testing** — every job is unit-testable with the captured fixtures served by `httptest`, against a `store.NewTestDB(t)` temp DB. Live API calls happen only when the scheduled job actually runs on the VPS.

---

## Security Domain (ASVS L1 — `security_enforcement: true`, `security_asvs_level: 1`, `security_block_on: high`)

This phase makes **outbound** HTTP to two **community-run external services** and parses **untrusted responses** into SQL. There is NO new inbound surface (the jobs are timer-triggered, not request-triggered) and NO new authentication. Severity is **LOW** at ~12 users, but the planner MUST include a `<threat_model>` block per PLAN. Below are the threat-model inputs.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control for this phase |
|---------------|---------|----------------------------------|
| V2 Authentication | no | No new auth surface (jobs are internal/timer-driven). The guild-code auth is untouched. |
| V3 Session Management | no | No sessions. |
| V4 Access Control | no | No new request paths; the scheduler runs in-process. |
| V5 Input Validation | **yes** | Responses from PigParse/wiki are UNTRUSTED input. Validate: `parseToRows` already tolerates ≤1% malformed rows + rejects non-array bodies; the wiki parsers guard `MIN_WIKITEXT_LENGTH` and return typed `{ok:false,reason}` on bad input. Port these guards verbatim. Cap response size with `io.LimitReader` (avoid OOM). |
| V6 Cryptography | partial | Only `crypto/sha1` for change-detection (NOT a security hash — it's a content fingerprint; acceptable use). No secrets, no encryption needed in this path. TLS on outbound is the crypto-relevant control (V9-ish). |
| V7 Error Handling / Logging | **yes** | `slog` JSON to stdout; never log raw response bodies wholesale (log counts + status, not the full wikitext/JSON). No secrets in this path to leak. |
| V12 / V13 (Files/API) | partial | Outbound API hygiene: identifying User-Agent, TLS verification ON, bounded reads, politeness (rate-limit self-throttle). |

### Known Threat Patterns for {Go outbound-fetch + untrusted-parse → SQLite}

| Pattern | STRIDE | Standard Mitigation | Severity here |
|---------|--------|---------------------|---------------|
| Unbounded response → OOM / DoS-self | Denial of Service | `io.LimitReader(resp.Body, ~16MB)` on every fetch | LOW (trusted-ish servers, but cheap to guard) |
| Malformed/oversized wiki/pigparse response crashes parser | Denial of Service | Parsers already return typed errors + tolerate malformation (port verbatim); job catches per-item failures and continues (mirror the Sheet's per-item failure accounting) | LOW |
| SQL injection via item names / wikitext into upserts | Tampering | **Parameterized queries only** (`?` placeholders in `database/sql`) — already the norm (11-03/05); NEVER string-concat parsed values into SQL | LOW (but mandatory) |
| Outbound call over plaintext / MITM'd | Tampering / Info Disclosure | TLS verification ON (Go default `http.Client`; never `InsecureSkipVerify`) — matches the TS `validateHttpsCertificates:true` | LOW |
| Redirect to internal/SSRF target | Tampering | URLs are hardcoded constants (PigParse getall, wiki api.php) — NOT user-supplied. Go's default 10-redirect cap. No SSRF vector (no user controls the URL). | NONE (no user input to the URL) |
| Secret leakage in the fetch path | Info Disclosure | There ARE no secrets in this path (public APIs). Confirm no token/credential is ever attached to the outbound request. | NONE |
| Being a bad external citizen (hammering community servers) | (reputational / DoS-other) | The politeness controls ARE a success criterion (SC-3): UA, ETag/304, backoff+Retry-After, 1s inter-request sleep. Porting them faithfully IS the mitigation. | LOW but explicit success criterion |
| Truncated response clobbers good data | Tampering (data integrity) | Graceful upsert (D-4) + truncation-guard-as-log; 304 skips the write entirely | LOW |

**Bottom line for the planner's `<threat_model>`:** the realistic threats are (1) self-inflicted DoS via unbounded reads → fix with `LimitReader`; (2) data-integrity (truncation/304 clobbering) → fix with graceful upsert + skip-on-304; (3) the standing parameterized-SQL discipline. SSRF/secret-leak/auth threats do NOT apply (hardcoded public URLs, no secrets, no inbound surface). Mark these LOW; none should trip `security_block_on: high`.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The live Sheet's dimension tabs still hold last-good data usable as the D-7 parity reference. | Parity Check | LOW — if the Sheet data is gone/stale, parity falls back to "the jobs ran and produced plausible row counts + spot-checked item values against the live wiki/PigParse directly." |
| A2 | Persisting `last_run_at` even on error (advance-always) is acceptable for daily/weekly cadence (avoids hot-loop retry). | Scheduler Design | LOW — alternative is advance-on-success + cooldown; either works. Planner can choose. The politeFetch backoff handles transient failures within a run regardless. |
| A3 | A hardcoded fallback User-Agent (`SquireBot/dev (+github)`) is acceptable for P12 if a backend version var isn't wired. | politeFetch Port | LOW — the UA just needs to be identifying + contactable; the github URL satisfies that. Wiring `-ldflags` Version is a nice-to-have. |
| A4 | Omitting the parser's `t90/a90` (90-day) and `tc/ta` (today, always 0) PigParse fields matches the Sheet (which never wrote them). | Column Reconciliation §2 | LOW — verified against `buildRow` in `refreshPigparse.ts` (it writes t30/a30/t60/a60/t6m/a6m/ty/ay, not t90/a90/tc/ta). If a view later wants 90-day, add columns then. |
| A5 | `io.LimitReader` cap of ~16 MB is generous (real PigParse fixture is 1.27 MB, wiki pages <100 KB). | politeFetch Port / Security | LOW — well above observed sizes; tune if PigParse grows. |

**If user confirmation is wanted before execution:** A1 (parity reference availability) is the only one worth a heads-up; the rest are Claude's-discretion implementation details the planner can lock.

---

## Open Questions (RESOLVED — locked as CONTEXT D-9..D-12 on 2026-05-29 per the phase's faithful-port criterion)

> All four are resolved and locked in 12-CONTEXT.md `<decisions>` (D-9..D-12) and implemented by the plans. Kept here with their analysis for traceability — none remains open.

1. **PigParse t=0/t=1 duplicate-item_id natural-key semantics.** (HIGHEST — the one genuine ambiguity.) → **LOCKED D-9: filter to WTS (t=0) before upsert.**
   - What we know: the response has two rows per item (WTS t=0, WTB t=1); `pigparse_price.item_id` is a PRIMARY KEY (one row/item); the Sheet kept BOTH (dup item_ids in `_pigparse`); downstream `view`/`bank` read `current_avg`/`blue_volume` (a30/t30, direction-agnostic).
   - What's unclear: whether to (a) filter to WTS (t=0, sell price — what a tooltip wants), (b) last-wins on upsert, or (c) change the key to `(item_id, direction)` and keep both.
   - Recommendation: **(a) filter to t=0 (WTS) before upsert** — gives one sensible "what's it worth to sell" price per item, satisfies the PK, and is the most defensible single value. Lock this in the plan (or in discuss-phase). Note it slightly diverges from the Sheet's dup-keeping (backend will have fewer pigparse rows than the Sheet) — acceptable and arguably cleaner.

2. **Wall-clock pinning vs simple interval for job cadence.**
   - What we know: the Sheet ran PigParse ~03:00 PT and wiki ~Sunday 04:00–05:00 PT.
   - What's unclear: whether the backend must hit those exact local times or just "once per day / once per Sunday."
   - Recommendation: **simple interval** (`>=24h` for daily; "before this Sunday's start AND it's Sunday" for weekly) — restart-robust, no timezone math, adequate at this scale. Pin to a UTC hour only if the maintainer wants off-peak timing for the external servers (a politeness nicety, not a requirement).

3. **Backend version var for the User-Agent.**
   - What we know: the watcher has `main.Version` via ldflags; the backend `cmd/squirebot-server` has none.
   - What's unclear: whether to add one now or use a hardcoded fallback.
   - Recommendation: add `var Version` to the backend main (settable via `-ldflags -X`) for a real version in the UA; hardcoded fallback is fine if the planner wants to defer. Self-contained, low-stakes.

4. **Stale-row cleanup for `wiki_spells`/`quest_items` upserts.**
   - What we know: pure upsert never deletes rows that vanished from the wiki (a removed spell lingers); the Sheet did per-class / per-item full-replace.
   - What's unclear: whether lingering stale rows matter for v1.
   - Recommendation: do **per-class DELETE+INSERT** for `wiki_spells` and **per-item-id DELETE+INSERT** for `quest_items` inside one tx (matches the Sheet's replace semantics exactly, avoids stale rows). `wiki_gear_tier` is already full-replace. Only `item_master` stays a pure per-item upsert (items don't get "removed" from the wiki in a way that should delete the master row).

---

## State of the Art

| Old Approach (Apps Script) | New Approach (Go backend) | Why It Changed |
|----------------------------|---------------------------|----------------|
| Time-driven triggers (Google) | In-process `time.Ticker` + DB-persisted `job_run` cursor | No Google; the backend owns scheduling (BACKEND-01). |
| `PropertiesService` ETag storage | `etag_cache` table | No PropertiesService; DB is the durable store. |
| 6-min cap → resumable cursor | Single uninterrupted job | Backend has no execution cap (net simplification). |
| `LockService.getDocumentLock()` | `SetMaxOpenConns(1)` + per-job `sync.Mutex` | DB single-writer + in-process mutex. |
| Full-replace `_pigparse` with hard truncation abort | Graceful `ON CONFLICT DO UPDATE` + truncation-guard-as-LOG | DB upserts degrade gracefully (D-4). |
| `Utilities.computeDigest` + signed-byte fix-up | `crypto/sha1` (unsigned) | Go stdlib; no fix-up needed. |
| `_meta.last_error`/`_status` writes | `slog` JSON + `job_run.last_status` | Structured logging to journald. |

**Deprecated/not carried over:** the entire `_meta.schema_version`/`WatcherMaxSchemaVersion` handshake (retired for v2.0; this phase doesn't touch it), `CacheService` response cache, `monitorCellCount`, `weeklySchemaHealthcheck`, and the post-run view-rebuild calls (P14 owns views).

---

## Sources

### Primary (HIGH confidence — repo files read verbatim this session)
- `internal/backendsrv/migrations/00001_init.sql` (lines 59–63 = the 5 dimension tables) — authoritative current DDL.
- `internal/backendsrv/migrations/00002_audit.sql` + `embed.go` — goose file shape, `//go:embed *.sql`, `"sqlite3"` dialect.
- `internal/backendsrv/scheduler/scheduler.go` — the skeleton to flesh out.
- `internal/backendsrv/store/db.go` — `Open`/`DSN`, `SetMaxOpenConns(1)`, modernc `"sqlite"` driver.
- `cmd/squirebot-server/main.go` — scheduler wiring, no version var, subcommand dispatch.
- `apps-script/src/lib/pigparse-types.ts` — `parseToRows`, `PigparseRowRaw` (15 fields), 1% malformation tolerance.
- `apps-script/src/lib/wiki-parser.ts` + `wiki-types.ts` — `parseItempage`, `ParsedWikiItem`, `WikiQuestItemLink`, `computeSha1Hex`.
- `apps-script/src/lib/wiki-spell-parser.ts` + `wiki-spell-types.ts` — `parseClassPage`, `WikiSpellRow`, `normalizeSpellName`, the 3 template variants + Bard inline-level fallback.
- `apps-script/src/lib/wiki-gear-tier-parser.ts` + `wiki-gear-tier-types.ts` — `parseGearTierPage`, `WikiGearTierRow` (item_id always null), Iksar tagging.
- `apps-script/src/lib/eq-constants.ts` — `CLASSES` (14), `CLASS_DISPLAY_TO_ABBREV`, `WIKI_SLOT_TO_INV_SLOTS`.
- `apps-script/src/lib/politeFetch.ts` + `politeFetch.test.ts` — every politeness control + the test contract.
- `apps-script/src/triggers/refreshPigparse.ts` — `PIGPARSE_HEADERS` (15 cols), `buildRow`, truncation guard, the t-as-direction write.
- `apps-script/src/triggers/refreshWikiItems.ts` — `ITEM_MASTER_HEADERS`, `QUEST_ITEMS_HEADERS`, SHA-1 short-circuit, cursor machinery (to delete).
- `apps-script/src/triggers/refreshWikiSpells.ts` — `WIKI_SPELLS_HEADERS`, per-class full-replace, post-run `buildSpellCheck` (to drop).
- `apps-script/src/triggers/refreshWikiGearTier.ts` — `WIKI_GEAR_TIER_HEADERS`, `replaceAllWikiGearTier` (full-replace), all-or-nothing on partial failure.
- `apps-script/src/__fixtures__/*` — the 12 fixtures (verified shapes + sizes; PigParse flat array, wiki `action=parse` envelopes).
- `.planning/explorations/website-milestone/04-data-enrichment-migration.md` — §1.3–1.4 (richer column proposals), §2 (the authoritative migration guide: ports cleanly, construct table, politeness, cadence, delete-the-cursor).
- `.planning/phases/12-enrichment-job-migration/12-CONTEXT.md` — the 8 locked decisions D-1..D-8 + 4 open items.
- `.planning/ROADMAP.md` §Phase 12 — 4 success criteria + the parsers-pure/watchdogs-dropped Note.
- `.planning/REQUIREMENTS.md` — ENRICH-10, ENRICH-11.
- `go.mod` — `go 1.25.7`; `cmd/squirebot/build_constants.go` — the watcher's `main.Version` ldflags pattern.

### Secondary (MEDIUM confidence — external, verified)
- SQLite `ALTER TABLE` semantics (one column per ADD COLUMN; no UNIQUE/PK on added columns; NOT NULL needs default; NULLs distinct in UNIQUE) — [sqlite.org/lang_altertable.html](https://sqlite.org/lang_altertable.html), [SQLite ALTER TABLE limitations](https://www.sqlitetutorial.net/sqlite-alter-table/).

---

## Metadata

**Confidence breakdown:**
- Column reconciliation + `00003` SQL: **HIGH** — diffed the actual `00001_init.sql` against the verified parser output structs and the verified trigger header constants; the gap is provably just the 8 PigParse columns + 2 new bookkeeping tables.
- Scheduler design: **HIGH** — based on the verified skeleton + a standard poll-and-check pattern; the restart-safety argument is mechanical (DB cursor + immediate check pass).
- politeFetch port: **HIGH** — every control enumerated from the verbatim TS source + its test file.
- Upsert/conflict keys: **HIGH** for the keys that work; the two hazards (gear-tier NULL-UNIQUE, pigparse PK dup) are verified facts about the DDL + the data, with clear mitigations.
- Security: **HIGH** for the threat surface (outbound-only, no secrets, hardcoded URLs); the LOW severity is well-founded at this scale.

**Research date:** 2026-05-29
**Valid until:** 2026-06-28 for the external bits (SQLite semantics are stable); the repo-internal findings are valid until the referenced files change (none expected before planning).

## RESEARCH COMPLETE

**Phase:** 12 - Enrichment Job Migration
**Confidence:** HIGH

### Key Findings
- **The dimension-table gap is narrow:** 11-02's `00001_init.sql` already carries the rich columns for `item_master`/`wiki_spells`/`wiki_gear_tier`/`quest_items` (the author copied them from RESEARCH). Only `pigparse_price` is missing 8 price-history columns (`t30,a30,t60,a60,t6m,a6m,ty,ay`). The `00003` migration adds those 8 columns + `job_run` + `etag_cache` — small and low-risk. Exact SQL is in the doc.
- **`pigparse_price` is the real table name** (00001); the findings doc's `item_price` is drift — resolved.
- **Two genuine upsert hazards, both with verified mitigations:** (1) `wiki_gear_tier`'s declared `UNIQUE(…, item_id)` is broken because `item_id` is always NULL (NULLs distinct in SQLite UNIQUE) → use full-table replace; (2) PigParse has two rows per item_id (t=0/t=1) vs a PK on item_id → filter to WTS or last-wins (the one decision to lock — Open Question 1).
- **Scheduler restart-safety is deterministic** via a `job_run.last_run_at` cursor + an immediate check pass on startup (runs a missed job within seconds, never double-runs).
- **politeFetch ports verbatim** to a `net/http` client; every control enumerated; the only ADDs are `If-Modified-Since` (SC-3 lists it) and an `io.LimitReader` cap (OOM guard). ETag state → `etag_cache` table.

### File Created
`.planning/phases/12-enrichment-job-migration/12-RESEARCH.md`

### Confidence Assessment
| Area | Level | Reason |
|------|-------|--------|
| Column reconciliation + `00003` SQL | HIGH | Diffed actual DDL vs verified parser output structs + trigger header constants |
| Scheduler design | HIGH | Verified skeleton + mechanical restart-safety argument (DB cursor + immediate check) |
| politeFetch port | HIGH | Every control from verbatim TS source + test file |
| Upsert/conflict keys | HIGH | Verified DDL + data; 2 hazards with clear mitigations |
| Security (ASVS L1) | HIGH | Outbound-only, no secrets, hardcoded URLs → LOW severity, well-founded |

### Open Questions — RESOLVED (locked as CONTEXT D-9..D-12, 2026-05-29; nothing left to lock)
1. **PigParse t=0/t=1 natural-key** — LOCKED **D-9**: filter to WTS (t=0). (Was the one real ambiguity.)
2. Wall-clock pinning vs simple interval — LOCKED **D-10**: simple interval.
3. Backend version var for the UA — LOCKED **D-11**: add `var Version` ldflags (hardcoded fallback OK).
4. Stale-row cleanup for `wiki_spells`/`quest_items` — LOCKED **D-12**: per-key DELETE+INSERT (Sheet-faithful).

### Ready for Planning
Research complete. The planner has: a concrete column-diff per table + exact `00003` SQL; the scheduler design with `job_run` DDL, compute-next-run predicates, and the mutex; the full politeFetch control list + `etag_cache` DDL; the Go package layout; the upsert/conflict-key table + truncation-guard-as-log; the fixture-reuse list; the threat-model inputs; and the what-NOT-to-port + scope-guard lists. No re-derivation needed.
