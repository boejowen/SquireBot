# Website Milestone — Slice 04: Data Model & Enrichment Migration

**Scope:** Translate the Google Sheet tab schema into a relational DB; migrate the
enrichment jobs (PigParse daily, wiki weekly) to backend scheduled jobs; replace
the extend-only `_meta.schema_version` scheme with real DB migrations; design a
low-risk cutover with one-time backfill; address eviction/privacy in a DB world.

**Status:** Research / scoping only. No application code changed.

**Sources read:** `CLAUDE.md`, `.planning/PROJECT.md`, `.planning/research/ARCHITECTURE.md`,
`.planning/research/SUMMARY.md`, `apps-script/src/lib/migrations.ts`,
`apps-script/src/lib/politeFetch.ts`, `apps-script/src/triggers/refreshPigparse.ts`,
`apps-script/src/triggers/refreshWikiItems.ts`, `apps-script/src/triggers/installTriggers.ts`,
`apps-script/src/lib/archive.ts`, `internal/scaffold/scaffold.go`,
`internal/sheet/write.go`, `internal/sheet/owner.go`, `internal/sheet/heartbeat.go`.

---

## 0. Context: what the sheet actually is today

The workbook is a hand-rolled database whose "tables" are tabs. The current
schema is **`_meta.schema_version = 3`** with three layers:

- **Landing tabs** (watcher-written, one per char per file type): `inv:<Char>`
  (`Location, Name, ID, Count, Slots, _uploaded_at`), `spell:<Char>`
  (`Level, Name, _uploaded_at`). Written as full-snapshot atomic replace via
  one `batchUpdate` `UpdateCellsRequest` over a fixed range (`A1:F500` /
  `A1:C600`) — cells past the data are cleared in the same call.
- **Dimension tabs** (Apps Script-written, hidden, `_`-prefixed): `_meta`,
  `_char_owner`, `_item_master`, `_pigparse`, `_wiki_spells`, `_wiki_gear_tier`,
  `_quest_items`, `_audit`, `_status`, plus the lazily-created `_archive`.
- **View tabs** (consolidated, filterable): `view`, `gear_check`, `spell_check`,
  `bank`. These are **derived/computed** — they are joins of landing × dimension
  data. In a DB world they become **SQL queries / API endpoints / materialized
  views**, not stored tables. This is the single biggest simplification: ~4 of
  the 13 "tabs" stop being storage entirely.

Key shape facts that drive the DB design:

- Item `ID` is the canonical, stable join key; item `Name` drifts.
- Spellbook files carry **no spell IDs** — the only join key to `_wiki_spells`
  is normalized (lowercase/trimmed) spell name. This brittleness carries over
  to the DB; keep a `normalized_name` column.
- Identity is OAuth `userinfo.email`. The watcher stamps it; first-write-wins
  on `owner_email`; mismatches are logged, not overwritten.
- `_char_owner` carries soft-delete flags (`is_hidden`, `is_removed`) and
  per-char attributes the watcher cannot know (`class`, `level`, `race`,
  `is_bank_toon`) — populated via Apps Script sidebars.
- `_status` and `_audit` are observability/heartbeat tables.
- Coin data does **not** exist in any file; bank coin is 4 manual KV rows in
  `_meta`.

---

## 1. Proposed Database Schema

**Engine recommendation: PostgreSQL** (see Recommendation §6 for why over SQLite).
DDL below is PostgreSQL flavored. All timestamps are `timestamptz` (UTC). The
design collapses the per-character tab proliferation into normal rows keyed by a
`character_id` FK — the 200-tab Google limit simply ceases to exist, and the
"consolidated vs per-character views" locked decision becomes a non-issue
(every view is just a `WHERE`/`GROUP BY`).

### 1.1 Identity: owners & characters

`_char_owner` splits into two tables. Today one tab conflates the guildie
(owner) and the character; a proper model separates them so an owner row
survives all their characters and an owner-email change is a one-row update.

```sql
-- A guildie. Canonical identity = Google OAuth email.
CREATE TABLE owner (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email           CITEXT NOT NULL UNIQUE,        -- OAuth userinfo.email, case-insensitive
    display_name    TEXT,
    discord_handle  TEXT,                          -- v2 prep; nullable
    is_admin        BOOLEAN NOT NULL DEFAULT FALSE,-- replaces _meta.guild_admins allowlist
    is_owner_floor  BOOLEAN NOT NULL DEFAULT FALSE,-- replaces _meta.workbook_owner_floor
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

-- A P99 character. One owner has many characters.
CREATE TABLE character (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name            TEXT NOT NULL,                 -- P99 char name (unique per server)
    server          TEXT NOT NULL DEFAULT 'blue',
    owner_id        BIGINT NOT NULL REFERENCES owner(id),
    class           TEXT,                          -- nullable; set via UI form
    level           SMALLINT,                      -- nullable; set via UI form
    race            TEXT,                          -- nullable; set via UI form
    is_bank_toon    BOOLEAN NOT NULL DEFAULT FALSE,
    is_hidden       BOOLEAN NOT NULL DEFAULT FALSE,-- soft hide (user choice)
    is_removed      BOOLEAN NOT NULL DEFAULT FALSE,-- soft-delete / archived
    first_seen      timestamptz NOT NULL DEFAULT now(),
    last_seen       timestamptz NOT NULL DEFAULT now(),  -- heartbeat / upload touch
    watcher_version TEXT,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (server, name)                          -- char name unique per server
);
CREATE INDEX character_owner_idx   ON character(owner_id);
CREATE INDEX character_active_idx  ON character(server) WHERE is_removed = FALSE;
```

**Owner-email-change handling:** today a re-OAuth under a new email creates a
silent mismatch surfaced in `_audit`. In the DB world: `UPDATE owner SET email=…`
is a clean one-row change and every `character` FK follows automatically. The
"first-write-wins" conflict logic at the watcher↔sheet seam disappears because
the backend, not 12 racing watchers, owns the write.

### 1.2 Inventory & spellbook snapshots

The watcher's "full-snapshot replace per character per file" maps cleanly to a
**delete-all-then-insert within one transaction**, keyed on `character_id`. This
preserves the exact idempotency property the sheet has today (re-uploading the
same file yields the same DB state) without any row-diffing.

```sql
-- Current inventory rows for a character. Full-replace per upload.
CREATE TABLE inventory_item (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    character_id    BIGINT NOT NULL REFERENCES character(id) ON DELETE CASCADE,
    location        TEXT   NOT NULL,               -- 'General1', 'Bank2-Slot3', 'Charm'
    item_name       TEXT   NOT NULL,               -- display name (drifts)
    item_id         INTEGER,                       -- EQ item ID; 0/NULL for empty slot
    count           INTEGER NOT NULL DEFAULT 1,
    slots           INTEGER,                       -- container slot count (bags)
    row_ordinal     INTEGER NOT NULL,              -- file line order; stable display sort
    uploaded_at     timestamptz NOT NULL
);
CREATE INDEX inventory_char_idx ON inventory_item(character_id);
CREATE INDEX inventory_item_idx ON inventory_item(item_id) WHERE item_id IS NOT NULL;
-- Trigram index powers the cross-character search sidebar replacement:
CREATE INDEX inventory_name_trgm ON inventory_item USING gin (item_name gin_trgm_ops);

CREATE TABLE spellbook_entry (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    character_id    BIGINT NOT NULL REFERENCES character(id) ON DELETE CASCADE,
    spell_level     SMALLINT NOT NULL,             -- spell-grant level (1-60)
    spell_name      TEXT NOT NULL,
    normalized_name TEXT NOT NULL,                 -- lower(trim(name)) — join key
    uploaded_at     timestamptz NOT NULL
);
CREATE INDEX spellbook_char_idx ON spellbook_entry(character_id);
CREATE INDEX spellbook_norm_idx ON spellbook_entry(normalized_name);
```

**Write pattern (replaces the `batchUpdate` clear+write):**

```sql
BEGIN;
  DELETE FROM inventory_item WHERE character_id = $1;
  INSERT INTO inventory_item (character_id, location, item_name, item_id,
                              count, slots, row_ordinal, uploaded_at)
  VALUES …;                                        -- bulk insert all parsed rows
  UPDATE character SET last_seen = now(), watcher_version = $2 WHERE id = $1;
COMMIT;
```

This is *better* than the sheet version: the transaction makes the clear+insert
truly atomic with no reader-visible intermediate state (the sheet's single
`batchUpdate` approximates this; a real `BEGIN/COMMIT` guarantees it), and a
shrinking inventory (item dropped) is handled for free by the `DELETE`.

**Optional history:** if the guild ever wants "what did Bob have last month",
add an append-only `inventory_snapshot` table (a `snapshot_id` + JSONB blob, or
a soft `valid_from/valid_to` on `inventory_item`). **Recommendation: do NOT
build this for v1 of the website** — it is not in the Core Value statement
("what does my character still need, and where is it") and adds real complexity.
Park it as a backlog item.

### 1.3 Item master & PigParse pricing

`_item_master` (wiki-sourced) and `_pigparse` (price-sourced) are two separate
enrichment feeds keyed on the same `item_id`. Keep them as two tables joined on
`item_id` — do not denormalize price into the item table the way the sheet does
(the sheet denormalizes only because cross-tab `VLOOKUP` is slow; SQL joins are
free).

```sql
-- Wiki-sourced item dimension. PK is the EQ item ID itself.
CREATE TABLE item_master (
    item_id         INTEGER PRIMARY KEY,           -- EQ item ID — natural key
    name            TEXT NOT NULL,
    wiki_summary    TEXT,
    wiki_url        TEXT,
    slot            TEXT,
    is_quest_item   BOOLEAN NOT NULL DEFAULT FALSE,
    wikitext_sha1   TEXT,                          -- change-detection (skip unchanged)
    last_refreshed  timestamptz
);

-- PigParse daily pricing. PK = item_id (one current-price row per item).
CREATE TABLE item_price (
    item_id         INTEGER PRIMARY KEY,
    name            TEXT,
    current_avg     NUMERIC(12,2),
    blue_volume     INTEGER,
    last_seen       TEXT,                          -- PigParse's own last-seen string
    direction       TEXT,                          -- price trend
    t30 INTEGER, a30 NUMERIC(12,2),
    t60 INTEGER, a60 NUMERIC(12,2),
    t6m INTEGER, a6m NUMERIC(12,2),
    ty  INTEGER, ay  NUMERIC(12,2),
    last_refreshed  timestamptz NOT NULL DEFAULT now()
);
```

Note `item_master` / `item_price` are **not** FK'd to `inventory_item.item_id`:
a character can hold an item the enrichment jobs have not yet fetched. The join
is a `LEFT JOIN` at query time — exactly the sheet's behavior (a missing master
row just yields a blank tooltip).

### 1.4 Wiki dimension tables

```sql
-- Per-class trainable spell list (weekly wiki scrape).
CREATE TABLE wiki_spell (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    class           TEXT NOT NULL,
    level           SMALLINT NOT NULL,
    spell_name      TEXT NOT NULL,
    normalized_name TEXT NOT NULL,                 -- joins spellbook_entry.normalized_name
    last_refreshed  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (class, level, spell_name)
);
CREATE INDEX wiki_spell_class_idx ON wiki_spell(class, level);
CREATE INDEX wiki_spell_norm_idx  ON wiki_spell(normalized_name);

-- Velious / Iksar gear tier recommendations (weekly wiki scrape).
CREATE TABLE wiki_gear_tier (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tier            TEXT NOT NULL,                 -- 'velious_pre_raid' | 'velious_raiding' | 'iksar'
    class           TEXT NOT NULL,
    slot            TEXT NOT NULL,
    item_id         INTEGER,
    item_name       TEXT,
    rank            SMALLINT,                      -- ordering when multiple options per slot
    last_refreshed  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tier, class, slot, item_id)
);
CREATE INDEX wiki_gear_class_idx ON wiki_gear_tier(class, tier);

-- Per-item quest associations (weekly wiki scrape).
CREATE TABLE quest_item (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    item_id         INTEGER NOT NULL,
    quest_name      TEXT NOT NULL,
    source_url      TEXT,
    source          TEXT,                          -- 'in_game_flag' | 'notes_link'
    last_refreshed  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (item_id, quest_name)
);
CREATE INDEX quest_item_item_idx ON quest_item(item_id);
```

Each weekly scrape is a per-key upsert (`INSERT … ON CONFLICT … DO UPDATE`) —
the same idempotent shape the Apps Script `upsertItemMasterRow` /
`replaceQuestItemRowsForId` use today, but expressed as one SQL statement.

### 1.5 Audit, status/heartbeat, archive, config

```sql
-- Upload provenance log (replaces _audit tab). Append-only.
CREATE TABLE audit_log (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    occurred_at     timestamptz NOT NULL DEFAULT now(),
    owner_email     CITEXT,
    character_name  TEXT,
    file_type       TEXT,                          -- 'inventory' | 'spellbook' | 'heartbeat'
    rows_written    INTEGER,
    watcher_version TEXT
);
CREATE INDEX audit_log_time_idx ON audit_log(occurred_at DESC);

-- Per-character watcher heartbeat / freshness (replaces _status tab).
-- One row per character; merged into `character` is also viable, but keeping
-- it separate matches the sheet split and isolates high-churn columns.
CREATE TABLE watcher_status (
    character_id          BIGINT PRIMARY KEY REFERENCES character(id) ON DELETE CASCADE,
    owner_email           CITEXT,
    watcher_version       TEXT,
    last_inventory_upload timestamptz,
    last_spellbook_upload timestamptz,
    last_heartbeat        timestamptz
);

-- Archived character snapshots (replaces lazily-created _archive tab).
CREATE TABLE character_archive (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    archived_at     timestamptz NOT NULL DEFAULT now(),
    character_name  TEXT NOT NULL,
    owner_email     CITEXT,
    reason          TEXT NOT NULL,                 -- 'stale_90d' | 'evicted'
    inventory_json  JSONB,                         -- frozen snapshot at archive time
    spellbook_json  JSONB
);

-- Workbook-level KV config (replaces _meta tab non-schema rows).
CREATE TABLE app_config (
    key             TEXT PRIMARY KEY,
    value           TEXT
);
-- seed rows: bank_toon_name, bank_coin_pp/gp/sp/cp, theme, contact_email,
--            last_pigparse_refresh, last_wiki_*_refresh, last_error.
```

`app_config` is deliberately a thin KV table for direct lift-and-shift of the
`_meta` non-versioning rows. The `schema_version` row, by contrast, **does not
carry over** — DB schema versioning becomes the migration tool's job (§3).

### 1.6 View tabs → queries (no tables)

`view`, `gear_check`, `spell_check`, `bank` become **read endpoints**. Examples:

- `view` / `bank` = `inventory_item JOIN item_master USING(item_id) LEFT JOIN
  item_price USING(item_id) JOIN character ON …` with the tooltip composed
  server-side. `bank` is just `view` filtered to `character.is_bank_toon = TRUE`.
- `gear_check` = `wiki_gear_tier` for the char's `class`/`race` `LEFT JOIN`'d
  against the character's equipped `inventory_item` rows (by `slot`), status
  computed `OK`/`MISSING`/`OTHER`.
- `spell_check` = `wiki_spell` for the char's `class` `LEFT JOIN spellbook_entry
  USING(normalized_name)`, status `KNOWN`/`MISSING`.
- Cross-character search = a single `inventory_item` query against the
  `gin_trgm` index — replaces the `searchIndex` / `CacheService` machinery
  entirely; the 60s cache and `prewarmSearchCache` become unnecessary.

The Apps Script `onChange`/`buildView` rebuild trigger and the hourly backstop
**disappear** — there is no materialized state to rebuild.

---

## 2. Enrichment Job Migration

### 2.1 What ports cleanly

The fetch/parse logic ports **very cleanly** — it is the most reusable code in
the project. The parsers (`pigparse-types.ts` `parseToRows`, `wiki-parser.ts`
`parseItempage`, `wiki-spell-parser.ts`, `wiki-gear-tier-parser.ts`) are pure
functions over response bodies; they are framework-agnostic TypeScript with no
`SpreadsheetApp` dependency and would run unchanged on Node, or transliterate
1:1 to Go. The endpoints, request shapes, and JSON/wikitext contracts are
identical regardless of host.

**Recommendation:** if the backend is **Go** (the maintainer's strength, and the
watcher is already Go), port the parsers to Go — they are small and well-tested,
and a single-language backend is the lower-maintenance choice for a part-time
solo maintainer. If the backend is **Node/TypeScript**, lift the existing parser
files near-verbatim and keep their vitest suites. Either way the *I/O wrappers*
(`UrlFetchApp`, `PropertiesService`, `CacheService`, `LockService`) get replaced.

### 2.2 What changes

| Apps Script construct | Backend replacement |
|---|---|
| Time-driven trigger (daily 03:00 / weekly Sun) | OS `cron` / systemd timer / hosted scheduler / a `gocron`-style in-process scheduler |
| `UrlFetchApp.fetch` + `politeFetch` | `net/http` (Go) or `fetch`/`undici` (Node); `politeFetch`'s retry/backoff/Retry-After/User-Agent logic ports verbatim — it is already host-agnostic policy code |
| ETag stored in `PropertiesService` | `app_config` row or a small `etag_cache(url, etag, fetched_at)` table |
| `CacheService` 6h/7d response cache | DB-backed cache table, or just skip — at 12 users the wiki/PigParse load is already trivial; the ETag/304 path is the real politeness control |
| 6-minute execution cap → resumable cursor + self-rescheduling one-shot trigger | **Deleted.** A backend job has no 6-minute cap; the whole `CURSOR_KEY` checkpoint/resume machinery in `refreshWikiItems.ts` is removed. This is a *net simplification* — the resumable cursor exists solely to work around an Apps Script limitation. |
| `LockService.getDocumentLock()` | DB transaction + `SELECT … FOR UPDATE` or a Postgres advisory lock; mostly unneeded since the backend is the single writer |
| `_meta.last_error` write on failure | structured logging + an `app_config` `last_error` row or a `job_run` table |

### 2.3 Politeness controls — keep them

The current politeness controls are: identifying `User-Agent`
(`SquireBot/<ver> (+github url)`), `If-None-Match`/304 ETag handling,
exponential backoff `[2s,4s,8s,16s,32s]` honoring `Retry-After` on 429/503/504,
a 1-second inter-request sleep between wiki fetches, and a truncation guard
(refuse to clobber `_pigparse` if today's row count < 90% of last known).

**All of these must carry over** — they are good external-citizen behavior and
the wiki/PigParse are community-run. They are cheap to keep: `politeFetch` is
already policy code, and the truncation guard becomes a `WHERE`-count check
before the upsert transaction commits. ENRICH-09 (courtesy emails) was waived
on the grounds that throttling is sufficient; that reasoning still holds.

One improvement the DB enables: instead of full-replace of `_pigparse`, do
`INSERT … ON CONFLICT (item_id) DO UPDATE` so a partial/truncated PigParse
response degrades gracefully (updates what it got, leaves the rest) rather than
the all-or-nothing guard. Keep the truncation guard as a belt-and-braces sanity
log either way.

### 2.4 Job cadence

Keep the existing cadence — it is well-tuned: PigParse daily, wiki items/spells
weekly Sunday, wiki gear tier weekly Sunday. The `monitorCellCount` watchdog
(10M-cell cap) and `weeklySchemaHealthcheck` (expected-tab watchdog) are
**Google-Sheets-specific and get deleted**. `weeklyStaleCharArchive` and
`weeklyEvictionArchive` survive as scheduled jobs (see §5).

---

## 3. Schema-Evolution Strategy

### 3.1 Retire the extend-only `_meta.schema_version` scheme

The extend-only discipline (`migrateToV2`/`migrateToV3` append columns at the
right edge, write `schema_version` last, watcher checks `WatcherMaxSchemaVersion`)
exists **entirely because Apps Script cannot `ALTER TABLE`**. A real database
can. The whole scheme — version-gated migration functions, the
`WatcherMaxSchemaVersion`/`SCRIPT_MIN_SCHEMA_VERSION` handshake, the "write
schema_version LAST so partial runs replay" rule — is replaced by a standard
migration tool.

### 3.2 Recommended tool

- **If the backend is Go:** [`golang-migrate`](https://github.com/golang-migrate/migrate)
  or [`goose`](https://github.com/pressly/goose). `goose` is the slightly
  friendlier choice for a solo maintainer (Go-native, supports Go-code migrations
  for data backfills, embeddable). Either is fine.
- **If the backend is Node:** the ORM's bundled migrator — Prisma Migrate, or
  Drizzle Kit. Drizzle is the lighter-weight pick.

**Recommendation: Go backend + `goose`**, with plain-SQL migration files checked
into the repo under `backend/migrations/NNNN_description.sql`.

### 3.3 Discipline

- Migrations are **forward-only, append-numbered, immutable once merged** (never
  edit a migration that has run anywhere). This mirrors the existing
  "extend-only / never break" instinct — keep that culture, it is healthy.
- Run migrations automatically on backend startup (`goose up`) so deploy =
  migrate. At 12 users / single backend instance there is no rolling-deploy
  skew to worry about — the hard part the sheet scheme solved (12 watchers at
  mixed versions) **no longer exists** because watchers will talk to one
  versioned API, not the raw schema.
- The watcher↔backend contract moves to an **API version** (`/api/v1/...`),
  not a schema version. The watcher no longer knows or cares about table
  shapes. This eliminates the `WatcherMaxSchemaVersion` foot-gun entirely
  (a recurring source of incident risk per CLAUDE.md).
- Keep destructive migrations (drop column, rename) behind an
  expand/contract two-step if the watcher fleet is mid-rollout — but for a
  12-user guild a brief maintenance window is acceptable and far simpler.

---

## 4. Cutover & Backfill Strategy

### 4.1 Can the new system run in parallel with the sheet?

**Yes** — and it should. Three options were considered:

**Option A — Watcher dual-writes (watcher → both Sheet API and new backend).**
Lowest user-facing risk (sheet keeps working untouched), but requires shipping a
new watcher binary to all 12 guildies *before* the backend is trusted, doubles
the watcher's write paths, and re-introduces OAuth-to-Google in the watcher that
the new architecture is trying to eliminate. Rejected as the primary path.

**Option B — Backend one-time import from the sheet, then watcher switch.**
The new backend reads the existing workbook once (read-only, via the Sheets API
or an exported copy) to backfill all historical data, runs in parallel in
read-only/shadow mode for a soak period, then watchers are flipped to the new
ingest endpoint in one coordinated update. **Recommended.**

**Option C — Hard cutover, no backfill.** Stand up the backend empty; the next
`/outputfile` from each guildie repopulates inventory/spellbook within days
(landing data is disposable — the watcher re-uploads the current file state on
every change anyway). Only the *dimension* data (wiki/PigParse) and *config*
(bank coin, owners, class/level/race) need migrating. This is viable precisely
because the watcher's full-snapshot model means inventory has no irreplaceable
history.

### 4.2 Recommended cutover (hybrid B/C)

1. **Build & soak the backend with its own enrichment jobs.** Run the PigParse
   and wiki scrapes on schedule into the new DB. This populates
   `item_master`/`item_price`/`wiki_*`/`quest_item` *natively* — no need to
   import dimension data from the sheet at all; the jobs regenerate it in one
   daily + one weekly cycle. Verify against the live sheet's dimension tabs.

2. **One-time backfill of the irreplaceable data only:** `owner`, `character`
   (including the human-supplied `class`/`level`/`race`/`is_bank_toon`/
   `is_hidden`/`is_removed` that the watcher can never re-derive), `app_config`
   bank-coin rows, and `character_archive`. Write a one-shot import script that
   reads these specific tabs via the Sheets API (read-only OAuth) and inserts
   rows. This is a small, bounded dataset (~12 owners, ~120 characters, a
   handful of config rows).

3. **Backfill current inventory/spellbook** the same way for day-one parity, OR
   rely on Option C's "next upload repopulates" — either is fine. Backfilling
   gives a non-empty site on launch day; recommended for UX.

4. **Parallel/shadow period (1–2 weeks).** Backend live with enrichment jobs;
   sheet still the source of truth for watchers. Maintainer compares the new
   site's views against the sheet's `view`/`gear_check`/`spell_check`/`bank`.

5. **Coordinated watcher flip.** Ship a watcher update (via the existing
   `minio/selfupdate` channel — already proven) that points ingest at the new
   backend API and drops the Google OAuth/Sheets code. Because the guild is 12
   people, a "everyone update this week" announcement is realistic; the
   self-updater makes it near-automatic.

6. **Decommission.** Once all 12 watchers report in on the new endpoint, set the
   sheet read-only and keep it as a frozen archive for a grace period, then
   retire it.

**Why this is lowest-risk:** the sheet is never modified by the new system
(read-only import), so a backend bug cannot corrupt the live product; the
enrichment data is self-healing (jobs regenerate it); inventory data is
self-healing (next upload); only the ~120-row human-supplied character metadata
needs careful one-time migration, and that is small enough to eyeball.

### 4.3 Backfill mechanics

- Import script authenticates with **read-only** Sheets scope and reads the
  specific dimension tabs by name. It does not need the watcher's `drive.file`
  picker dance — the maintainer runs it once against the known workbook ID.
- Normalize during import: `_char_owner` rows fan out into `owner` (dedup by
  email) + `character` rows; the conflated tab becomes the two-table model.
- `_archive` tab's `snapshot_json` columns map straight into
  `character_archive.inventory_json`/`spellbook_json`.
- Idempotent import (`ON CONFLICT DO NOTHING`/`DO UPDATE`) so it can be re-run
  if the soak period surfaces a mapping bug.

---

## 5. Eviction / Privacy in a DB World

### 5.1 Today

Data is guild-internal; everyone sees everything (universal visibility, an
explicit v1 decision). Eviction = an officer-only sidebar marks a guildie's
characters `is_removed`, a 30-day grace period runs, then `weeklyEvictionArchive`
moves the character's tabs into `_archive` (snapshot JSON) and hides the source
tabs. Stale characters (>90d no upload) get the same treatment via
`weeklyStaleCharArchive`. The sheet itself is shared via Google Drive ACLs, so
removing a guildie's *access* is a separate Drive un-share action.

### 5.2 In the DB world

The mechanics map cleanly and actually get **cleaner and more enforceable**:

- **Eviction** = set `owner` (and cascaded `character`) `is_removed = TRUE`,
  record the grace deadline. A scheduled job (`weeklyEvictionArchive` ported as
  a backend cron job) copies the evicted characters' inventory/spellbook into
  `character_archive` as JSONB snapshots and deletes the live
  `inventory_item`/`spellbook_entry` rows after the 30-day grace. Keep the
  grace period — it is a good "oops, un-evict" safety net.
- **Stale archival** (>90d) = identical, `reason = 'stale_90d'`.
- **Access revocation becomes a real thing the app controls.** Today "remove a
  guildie" means un-sharing a Google Sheet (a Drive-level action outside the
  app). In the DB world the website has its own auth; revoking a guildie is
  *one action* — deactivate their `owner` login — and it is enforced by the
  app, not by Drive ACLs. This is a genuine improvement: eviction is no longer
  a two-system dance.
- **Privacy posture:** keep universal visibility for v1 (the decision rationale
  — small trust-rich guild — still holds). But the DB makes per-owner visibility
  tiers *cheap* to add later (`WHERE owner_id = current_user OR …`) if the guild
  ever asks — note this as a future option, not v1 scope.

### 5.3 New privacy obligations the DB introduces

- **The maintainer now hosts guild data on infrastructure they control.** With
  the sheet, Google held the data and the OAuth `drive.file` scope kept the
  blast radius tiny. Now there is a database to back up, secure, and patch.
  This is a real new responsibility — call it out in the hosting/ops slice.
- **Deleted means deleted.** When a guildie asks to be removed, the DB should
  *actually delete* their PII (`owner.email`, `discord_handle`) after the grace
  period, not just flag `is_removed`. Keep the inventory snapshot anonymized in
  `character_archive` if guild history is wanted, or hard-delete — a guild
  policy decision worth making explicit.
- **Backups contain everyone's data** — a backup file is now a copy of the
  whole guild's inventory + emails. Encrypt backups at rest.

---

## 6. Recommendation

**Engine — PostgreSQL.** SQLite would be tempting for a 12-user hobby app and
would genuinely work, but Postgres is recommended because: (a) the
cross-character search wants `pg_trgm` GIN indexes (SQLite's FTS5 is workable
but less ergonomic for substring/ID search); (b) `CITEXT` cleanly models the
case-insensitive OAuth-email identity that is load-bearing here; (c) a hosted
Postgres (Fly.io, Neon, Supabase free tier, or a $5 VPS) removes the "back up
the SQLite file" chore; (d) `timestamptz` and real transactions match the
data-integrity properties the sheet's `batchUpdate` only approximates. If
hosting cost/simplicity dominates, SQLite + Litestream is an acceptable
fallback — the schema DDL above ports with only `CITEXT`→`TEXT COLLATE NOCASE`
and identity-column syntax changes.

**Schema shape — separate `owner` from `character`** (the sheet conflates them),
keep inventory/spellbook as plain FK-keyed rows (the per-character tab explosion
and the 200-tab limit simply vanish), keep the two enrichment feeds
(`item_master`, `item_price`) as separate tables joined at query time, and turn
all four view tabs into SQL queries / API endpoints. ~13 tabs collapse to ~12
tables, ~4 of which (`view`, `gear_check`, `spell_check`, `bank`) stop being
storage entirely.

**Migrations — `goose` with forward-only, immutable, append-numbered SQL files.**
Retire `_meta.schema_version` and the `WatcherMaxSchemaVersion` handshake; the
watcher↔backend contract becomes an *API version*, killing a known incident
class.

**Enrichment — port the parsers verbatim** (they are pure, host-agnostic, and
well-tested), replace the I/O wrappers, **delete the 6-minute-cap resumable-cursor
machinery** (a pure Apps Script workaround), and keep every politeness control
(User-Agent, ETag/304, backoff, inter-request sleep, truncation guard).

**Cutover — hybrid B/C.** Stand up the backend with its own enrichment jobs
(dimension data self-populates — no import needed); one-time read-only import of
*only* the irreplaceable human-supplied data (`owner`/`character` metadata,
bank coin, archives, optionally current inventory); soak in shadow mode 1–2
weeks against the still-live sheet; then a single coordinated watcher self-update
flips ingest to the new API and drops Google OAuth. The sheet is never written
by the new system, so the cutover cannot corrupt the live product, and both
inventory and enrichment data are self-healing.
