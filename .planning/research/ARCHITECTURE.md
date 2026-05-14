# Architecture Research

**Domain:** Distributed system — Windows watcher (Go) + Google Sheets workbook with Apps Script (TypeScript via clasp) + light data scraping. ~12 watcher instances writing to one shared workbook.
**Researched:** 2026-04-30
**Confidence:** HIGH for the sheet schema (drives directly off the file format and the wiki/PigParse data shapes which are all known); HIGH for the watcher → sheet write contract (Sheets API surface is small and well-documented); HIGH for the build-order analysis (each phase boundary justifies itself dependency-wise); MEDIUM for cross-cutting observability (Apps Script's logging surface is limited and the proposed pattern works but is not battle-tested at this scope); HIGH for the identity model (OAuth email is the only stable identity signal we have).

---

## TL;DR

- **The Sheet is a 3-layer pancake:** raw landing tabs (one per char per file type, owned by the watcher) → dimension tabs (item master, wiki tier, wiki spell list, PigParse prices, owned by Apps Script) → view tabs (per-char, shared bank, search, owned by Apps Script and read-only to the watcher).
- **Watcher writes are full-snapshot replaces per character.** `spreadsheets.values.update` against `'inv:<CharName>'!A2:E` with the entire file contents on every detected change. No row-level diffing. Idempotency is free because the same file produces the same range. Schema version is stamped on a hidden `_meta` tab, not in the data rows.
- **Apps Script is the only thing that writes the dimension and view tabs.** Time-driven triggers refresh PigParse daily and the wiki weekly. An `onChange` trigger rebuilds the joined views when watchers write to landing tabs.
- **Identity is OAuth email.** The first time `<CharName>-Inventory.txt` is uploaded by a watcher running under a Google account, a row is inserted into a `_char_owner` mapping tab linking char → google email. Subsequent writes from a different account require a manual override (rare; covers the "I gave a toon to a guildmate" case).
- **Five-phase build order is correct in spirit but the boundary between phases needs to move.** "Watcher writes raw inventory to one tab" must ship before any dimension tabs exist — otherwise we have no data to test the joins against. The proposed phase ordering is refined below.
- **Schema evolution: never break, only extend.** Tabs are versioned (`inv_v1:<CharName>` would become `inv_v2:<CharName>` if columns ever break compatibility). All Apps Script code reads the `_meta.schema_version` cell first and either migrates or warns.

---

## System Overview

```
┌──────────────────────────────────────────────────────────────────────┐
│                        Watcher tier (Go, ~12 instances)              │
├──────────────────────────────────────────────────────────────────────┤
│  ┌──────────┐   ┌──────────┐   ┌──────────────┐   ┌─────────────┐    │
│  │ fsnotify │ → │ debounce │ → │ file parser  │ → │ sheets push │    │
│  └──────────┘   └──────────┘   │ (TSV → rows) │   │ (values.    │    │
│                                 └──────────────┘   │  update)    │    │
│                                                    └──────┬──────┘    │
│  ┌─────────────┐   ┌──────────────────┐   ┌─────────────┐ │           │
│  │ tray + UI   │   │ OAuth (loopback) │   │ wincred     │ │           │
│  └─────────────┘   └──────────────────┘   └─────────────┘ │           │
└────────────────────────────────────────────────────────────┼──────────┘
                                                             │
                                                  Sheets API v4 (HTTPS)
                                                             │
                                                             ▼
┌──────────────────────────────────────────────────────────────────────┐
│                    Sheet tier (Google Sheets workbook)               │
├──────────────────────────────────────────────────────────────────────┤
│   ┌─────────────────────  Landing tabs (watcher-owned) ──────────┐   │
│   │  inv:<Char1>   inv:<Char2>   ...   spell:<Char1>   ...       │   │
│   └──────────────────────────────────────────────────────────────┘   │
│                                  │                                    │
│                       onChange / time triggers                        │
│                                  ▼                                    │
│   ┌──────────────────  Apps Script (V8 / TS via clasp) ──────────┐    │
│   │  build_views.ts    refresh_pigparse.ts   refresh_wiki.ts     │    │
│   │  search_sidebar.ts onOpen.ts             schema_migrate.ts   │    │
│   └──────────────────────────────────────────────────────────────┘    │
│                                  │                                    │
│   ┌─────────────────  Dimension tabs (script-owned) ─────────────┐    │
│   │  _item_master   _wiki_spells   _wiki_gear_tier   _pigparse   │    │
│   │  _char_owner    _meta          _quest_items                  │    │
│   └──────────────────────────────────────────────────────────────┘    │
│                                  │                                    │
│   ┌──────────────────  View tabs (script-owned) ────────────────┐     │
│   │  view:<Char>   view_spells:<Char>   gear_check:<Char>       │     │
│   │  spell_check:<Char>   bank   search (sidebar-only)          │     │
│   └─────────────────────────────────────────────────────────────┘     │
└──────────────────────┬───────────────────────────┬───────────────────┘
                       │                           │
              UrlFetchApp                  UrlFetchApp
                       │                           │
                       ▼                           ▼
              ┌─────────────────┐         ┌──────────────────┐
              │ PigParse REST   │         │ MediaWiki API    │
              │ (Azure)         │         │ (P1999 wiki)     │
              └─────────────────┘         └──────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Owner / Implementation |
|-----------|---------------|------------------------|
| **fsnotify watcher** | Detect creates/writes on `<EQ folder>/*-Inventory.txt` and `*-Spellbook.txt` | Go pkg `internal/watch` |
| **Debouncer** | Coalesce rapid events into one read (AV-induced spurious events, multi-write game saves) | Go pkg `internal/watch` |
| **File parser** | TSV → typed rows; validate column count; strip BOM if present | Go pkg `internal/parse` |
| **Sheets pusher** | Authenticate; resolve target tab; full-range write via `values.update`; create tab if first sighting of a character | Go pkg `internal/sheet` |
| **OAuth flow** | Loopback PKCE, refresh token rotation, Picker for spreadsheet ID selection (first run only) | Go pkg `internal/auth` |
| **Tray UI** | Quit, "Open Setup", status string ("Connected as foo@gmail.com — last upload 12s ago") | Go pkg `internal/tray` |
| **Updater** | Check `latest.json`, download, swap on next start | Go pkg `internal/updater` |
| **Landing tabs** | Raw character data (watcher's write target) | Sheet, watcher writes |
| **Dimension tabs** | Reference data: item master, wiki tier, spell lists, PigParse prices, char→owner mapping | Sheet, Apps Script writes |
| **View tabs** | Joined renderings: per-char inventory + tooltips, gear/spell checklists, bank, search | Sheet, Apps Script writes |
| **Refresh jobs** | Time-driven Apps Script: PigParse daily, wiki weekly, item-master rebuild on schedule | Apps Script (`refresh_*.ts`) |
| **Build views** | onChange-triggered Apps Script: rebuild affected view tab when its landing tab changes | Apps Script (`build_views.ts`) |
| **Search sidebar** | HtmlService sidebar; cross-character item lookup with "who has it" output | Apps Script (`search_sidebar.ts`) |
| **Schema manager** | Read `_meta.schema_version`; run any pending migrations; idempotent | Apps Script (`schema_migrate.ts`) |

---

## Sheet Schema

### Tab Naming Convention

Prefix indicates owner and lifecycle:

- `inv:<CharName>` — landing tab for inventory, watcher-written, one per character (created on demand).
- `spell:<CharName>` — landing tab for spellbook, watcher-written, one per character.
- `view:<CharName>` — rendered inventory view with tooltips (script-written; per character).
- `gear_check:<CharName>` — gear progression checklist (script-written; per character).
- `spell_check:<CharName>` — spell progression checklist (script-written; per character).
- `_<name>` — dimension/internal tab (script-written; hidden by default; underscore prefix sorts to the end and signals "do not edit").
- `bank` — special-purpose view tab for the designated bank toon, includes the manual coin field.
- `search` — placeholder tab; the actual search UX is the HtmlService sidebar.

### Landing Tabs (Watcher-Written)

#### `inv:<CharName>`

The five columns from `<CharName>-Inventory.txt` plus one provenance column. Headers in row 1, data starts row 2.

| Column | Type | Source | Notes |
|--------|------|--------|-------|
| A: `Location` | string | TSV col 1 | E.g., `General1`, `Bank2-Slot3`, `Charm`. Free-form; we don't validate. |
| B: `Name` | string | TSV col 2 | Display name. Drift-prone — DO NOT use as join key. |
| C: `ID` | int | TSV col 3 | EQ item ID. Canonical join key. `0` for empty slots — filter at view build time. |
| D: `Count` | int | TSV col 4 | Stack count. |
| E: `Slots` | int | TSV col 5 | Container slot count (relevant for bags). |
| F: `_uploaded_at` | ISO8601 string | watcher | Timestamp of last write from the watcher. Useful for staleness checks. |

**Primary key:** None at the row level; the *tab itself* is the unit of identity. Watcher does full-range replace.

**Row count guidance:** A maxed-out P99 character with bags + bank is ~250 rows. Sheet's row limit is 10M cells / workbook; ~50 chars × 250 rows × 6 cols = 75K cells. Trivial.

**Hidden:** No (visible by default in case Apps Script breaks and a guildie wants to read raw data). Could be hidden in a polish pass.

#### `spell:<CharName>`

Mirror structure for `<CharName>-Spellbook.txt`. The actual spellbook file is two columns: spell **level** (1–60) + spell **name**. Verified against the committed `internal/parse/testdata/Slampeach-Spellbook.txt` fixture (49 spells, level range 9–53 — proves col 1 is level, NOT slot, since /outputfile spellbook dumps the scribed list in level order with no relation to the 8 active mem slots). Phase 1 docs/code that called col 1 `Slot` were wrong; renamed to `Level` in Phase 2 (Plan 02-01 Task 5).

| Column | Type | Source | Notes |
|--------|------|--------|-------|
| A: `Level` | int | TSV col 1 | Spell level (1–60). |
| B: `Name` | string | TSV col 2 | Spell name. |
| C: `_uploaded_at` | ISO8601 | watcher | Same provenance pattern as inventory. |

**Primary key:** Tab is the unit of identity; full-range replace per upload.

**NOTE:** Spellbook file does NOT contain spell IDs — only names. This means the wiki spell-list join uses normalized name (lowercase, trimmed) as the key, which is more brittle than the inventory ID-based join. Plan to canonicalize names in `_wiki_spells` to the same normalization.

### Dimension Tabs (Apps Script-Written, Hidden)

#### `_meta`

Single row of workbook-level state.

| Cell | Purpose |
|------|---------|
| `A1` (header), `B1` `schema_version` | Integer. Bumped on breaking schema changes. Apps Script reads this on every trigger and refuses to run if `< MIN_SUPPORTED`. |
| `A2`/`B2` `last_pigparse_refresh` | ISO8601 timestamp. |
| `A3`/`B3` `last_wiki_refresh` | ISO8601. |
| `A4`/`B4` `last_view_rebuild` | ISO8601. |
| `A5`/`B5` `bank_toon_name` | String. Identifies which character is treated as the shared bank for the `bank` tab. |
| `A6`/`B6` `bank_coin_pp` / `_gp` / `_sp` / `_cp` | Manual coin entries. Editable by anyone with edit access. |
| `A7+` | Free for additional config rows. |

**Hidden:** Yes. Editable by all guildies (so anyone can update bank coin), but Apps Script also exposes a sidebar form for it to avoid raw-cell-edit mistakes.

#### `_char_owner`

Maps `<CharName>` → guildie identity. Populated automatically when a watcher first writes a new char tab.

| Column | Type | Notes |
|--------|------|-------|
| A: `char_name` | string | Primary key. |
| B: `owner_email` | string | Google email of the OAuth account that wrote this character. |
| C: `display_name` | string | Optional human-friendly guildie name. Manually filled in. |
| D: `discord_handle` | string | v2 prep — hidden in v1 if desired but include the column. |
| E: `first_seen` | ISO8601 | When the char first appeared. |
| F: `is_bank_toon` | bool | Marks the shared bank character. Set manually. |

**Conflict resolution:** If a watcher writes `inv:Foo` and the existing `_char_owner.Foo.owner_email` differs, the build_views script logs a warning and stamps the new email into a hidden audit column, but does not silently overwrite. The "who owns Foo?" question is then surfaced in the sidebar for an officer to resolve.

#### `_item_master`

The denormalized item dimension table. Built by Apps Script on a weekly schedule (or whenever a new item ID appears in any landing tab and is missing here).

| Column | Type | Source |
|--------|------|--------|
| A: `item_id` | int | Primary key. |
| B: `name` | string | Wiki canonical name. |
| C: `wiki_url` | string | `https://wiki.project1999.com/<name_underscored>`. |
| D: `wiki_summary` | string | First paragraph of wiki page (~200 chars), trimmed. |
| E: `is_quest_item` | bool | Wiki category check. |
| F: `quest_names` | string | Comma-joined quest names this item participates in. |
| G: `gear_tier` | string | Comma-joined: `velious_pre_raid`, `velious_raiding`, `iksar`. Empty if not in any tier list. |
| H: `gear_tier_class_slot` | string | JSON-encoded `{"warrior": ["chest","legs"], "ranger": ["chest"]}` for tier inclusion. |
| I: `pigparse_avg_price` | int | Latest avg from `_pigparse`. Denormalized for view-render speed. |
| J: `pigparse_last_seen` | ISO8601 | When PigParse last saw this item. |
| K: `_last_built` | ISO8601 | Audit column. |

**Build strategy:** This is a join across `_pigparse`, `_wiki_*` tabs. Rebuild is idempotent and re-run on every wiki refresh (weekly) and every PigParse refresh (daily, only price columns updated). Item rows persist across rebuilds.

**Hidden:** Yes.

#### `_pigparse`

Raw landing for the daily `GET /api/item/getall/1` (Blue server) call.

| Column | Type | Source |
|--------|------|--------|
| A: `item_id` | int | PigParse `itemid`. |
| B: `name` | string | PigParse name. |
| C: `avg_price` | int | Average bazaar price. |
| D: `min_price` | int | Recent minimum. |
| E: `max_price` | int | Recent maximum. |
| F: `sample_count` | int | How many auctions back this price. |
| G: `_pulled_at` | ISO8601 | Refresh timestamp. |

**Update strategy:** Full replace daily. ~5K rows is the rough size of PigParse's tracked-item universe; trivial.

**Hidden:** Yes.

#### `_wiki_spells`

Class spell list scraped weekly from per-class wiki pages.

| Column | Type | Source |
|--------|------|--------|
| A: `class` | string | `Wizard`, `Cleric`, etc. — composite primary key part 1. |
| B: `level` | int | Trainable level — composite primary key part 2. |
| C: `spell_name` | string | Composite primary key part 3. |
| D: `spell_name_normalized` | string | Lowercase/trimmed for spellbook join. |
| E: `wiki_url` | string | Spell wiki page. |
| F: `_pulled_at` | ISO8601 | Refresh timestamp. |

**Hidden:** Yes.

#### `_wiki_gear_tier`

Velious + Iksar gear-tier recommendations scraped weekly.

| Column | Type | Source |
|--------|------|--------|
| A: `tier` | string | `velious_pre_raid` / `velious_raiding` / `iksar`. |
| B: `class` | string | Class name. |
| C: `slot` | string | `chest`, `legs`, `head`, ... canonicalized. |
| D: `item_id` | int | Recommended item. |
| E: `item_name` | string | For display. |
| F: `rank` | int | If wiki lists multiple options per slot, ordering. |
| G: `wiki_url` | string | Tier-page section anchor. |
| H: `_pulled_at` | ISO8601 | Refresh timestamp. |

**Composite primary key:** `(tier, class, slot, item_id)`.

**Hidden:** Yes.

#### `_quest_items`

Per-item quest associations scraped from per-item wiki pages.

| Column | Type | Source |
|--------|------|--------|
| A: `item_id` | int | Composite PK part 1. |
| B: `quest_name` | string | Composite PK part 2. |
| C: `role` | string | `turn_in` / `reward` / `component` / `unknown`. |
| D: `quest_wiki_url` | string | Quest page. |
| E: `_pulled_at` | ISO8601 | Refresh timestamp. |

**Hidden:** Yes.

### View Tabs (Apps Script-Written, Visible)

#### `view:<CharName>`

The user-facing per-character inventory. One per character; visible.

| Column | Type | Source |
|--------|------|--------|
| A: `Location` | string | from `inv:<CharName>` |
| B: `Item` | string | hyperlink formula → wiki URL from `_item_master` |
| C: `Count` | int | from landing |
| D: `Avg Price (pp)` | int | denormalized from `_item_master.pigparse_avg_price` |
| E: `Quest?` | symbol | "Q" if quest item, else blank |
| F: `Tier?` | string | comma-joined `_item_master.gear_tier` |
| (cell note on column B) | string | Wiki summary + quest name list + price range — composed by Apps Script via `Range.setNote` |

**Build:** Rebuilt by `onChange` trigger when the corresponding `inv:<CharName>` tab changes. Also rebuilt globally when `_item_master` changes (since price/tooltip data updated).

**Hidden:** No.

#### `view_spells:<CharName>`

Per-character spellbook in display form (just spell name + wiki link). Lightweight; primarily a navigation tab. The interesting view is `spell_check:<CharName>`.

#### `gear_check:<CharName>`

The gear progression checklist — the headline differentiator feature.

| Column | Type | Source |
|--------|------|--------|
| A: `Tier` | string | `velious_pre_raid` / `velious_raiding` / `iksar` |
| B: `Slot` | string | Equipment slot |
| C: `Recommended` | string | hyperlink → wiki URL of recommended item |
| D: `Currently Equipped` | string | from `inv:<CharName>` joined on `Location` matching slot |
| E: `Status` | string | `OK` / `MISSING` / `OTHER (not on tier list)` |
| F: `Avg Price` | int | from `_item_master` |

**Build:** Joined from `inv:<CharName>` (slot equipped items) × `_wiki_gear_tier` (recommendations for char's class) × `_item_master` (price). Rebuilt on `onChange` of the landing tab and on `_wiki_gear_tier` refresh.

**Char class detection:** Class is not in `<Char>-Inventory.txt`. Two options: (a) infer from `<Char>-Spellbook.txt` cross-reference against `_wiki_spells`, or (b) ask once in the `_char_owner` tab. **Recommendation: ask once.** Add a `class` column to `_char_owner` and prompt on first sight via the sidebar. Spellbook inference is too fragile — a fresh character has no spells.

**Hidden:** No.

#### `spell_check:<CharName>`

Mirrors `gear_check` for spells. Trainable-at-current-level vs. spellbook. Requires `level` column on `_char_owner` (also asked once at first sight). Level-tracking is "trust the user to update it"; we don't auto-detect level (`/outputfile` doesn't give it).

#### `bank`

Special view of the designated bank toon (`_meta.bank_toon_name`). Same shape as `view:<CharName>` plus the manual coin row at the top:

```
A1: Coins (manual)    B1: PP    C1: GP    D1: SP    E1: CP
A2: <values from _meta>
A4+: same as view:<bank toon>
```

#### `search` (sidebar-only)

The cross-character search lives in an HtmlService sidebar (per STACK.md). The `search` tab itself is just a placeholder with a "click here to open search" image/link, since custom menus are also good (`onOpen` adds a "SquireBot → Search" menu item).

The sidebar queries an in-memory join across all `inv:*` tabs filtered by item name or item ID, returning:

```
"<ItemName> (id <id>)"
   ↳ <CharName1>: <Location1>, count 3
   ↳ <CharName2>: <Bank-Slot4>, count 1
```

---

## Watcher → Sheet Write Contract

**This is load-bearing.** Get this wrong and you have row duplication, lost updates, or unbounded API calls.

### Write Strategy: Full-Snapshot Replace

When the watcher detects a change to `<CharName>-Inventory.txt`:

1. Re-read the entire file (debounced 500ms).
2. Parse into TSV rows. Validate: 5 columns, integer ID/Count/Slots.
3. **One** API call: `spreadsheets.values.update`
   - Range: `'inv:<CharName>'!A1:F<N+1>` where N is the parsed row count (header in row 1).
   - `valueInputOption=RAW`.
   - Body: rows including header (we re-write the header every time — cheap and idempotent).
4. If the tab does not exist, first call `spreadsheets.batchUpdate` with `addSheet` to create it, then go back to step 3.
5. After successful write, also call `values.update` against `_char_owner!<find row>` to update `first_seen` (insert) or just `last_seen` (update); this is a small trailing call.

**Why full replace, not row-level upsert:**

- The file is canonical and small (~250 rows max). The cost of resending is bounded.
- Row-level upsert requires diffing, primary keys, and handling deletions (item deposited then withdrawn). All complexity for no benefit.
- Idempotency is automatic: same file content → same write → same result. No "did my last write succeed?" reasoning needed; just retry.
- Conflicts between watchers are impossible: each watcher only writes its own char tabs, and each char tab has exactly one owner (per `_char_owner`). Two-watcher race on the same char tab is the "I gave my toon to a guildmate and we both still run watchers" edge case — solved at the OAuth-email layer, not here.

**Why `values.update` not `values.append`:**

- `append` adds rows at the end. We want **replace**, otherwise old data accumulates forever.
- `update` with a tighter range than the existing data would leave stale rows below — so the watcher's range MUST extend to "current row count" at minimum. To handle a shrinking inventory (item dropped), we always write `A1:F<some-large-cap>` with explicit empty rows, OR we issue a `values.clear` against `A2:F` first, then update. **Recommendation: clear + update in a single `batchUpdate` request** (Sheets API supports this atomically — see `Request.updateCells` with `fields=*` over an extended range). Two-call alternative is acceptable too; the rare race window (someone reading between clear and update) is harmless.

**Idempotency / retry policy:**

- Network error: exponential backoff (2s, 4s, 8s, max 60s, max 5 retries).
- 403 (token expired): refresh token, retry once.
- 429 (rate limited): respect `Retry-After`, exponential backoff.
- 401 (token revoked): pop the tray icon to "Reauthorize" state; do not retry blindly.
- After all retries fail: log to local file, drop the event (file is unchanged on disk; next change re-triggers).

**Rate limit budget:**

- Sheets API per-user write quota: 60 requests/min/user. Per-watcher.
- Realistic load per watcher: ~10 inventory writes/day (player checks gear a few times). Trivial.
- Combined load on the workbook: 12 watchers × 10 writes/day = 120 writes/day. Negligible.

### `_char_owner` Auto-Insert Flow

First time watcher pushes `inv:Foo`:

1. Watcher (or Apps Script `onChange` trigger) checks if `Foo` exists in `_char_owner`.
2. If not, append: `(Foo, <oauth-email>, "", "", <now>, false)`.
3. The owner can then fill `display_name`, `discord_handle`, `class`, `level`, `is_bank_toon` via sidebar form.

**Implementation choice:** Do this from the watcher rather than Apps Script. The watcher knows its OAuth email cleanly via the OAuth response (`userinfo.email` after token exchange); Apps Script has no clean way to know which guildie's watcher made the write. This makes the watcher slightly more privileged (it writes to a dimension tab) but the alternative is much worse — Apps Script `Session.getActiveUser().getEmail()` returns the **script owner**, not the writer.

**Risk:** the watcher needs `_char_owner` write access. With `drive.file` scope, it has full access to the workbook; this is fine. But it means the watcher's code contains the magic tab name `_char_owner` and depends on its schema. Schema version stamp must catch incompatible changes.

---

## Data Flow

### Flow A: Inventory upload (every-time path)

```
EQ writes Foo-Inventory.txt
       │
       ▼
fsnotify Modify event ── (debounce 500ms) ──┐
                                            │
                                            ▼
                                       Re-stat + read file
                                            │
                                            ▼
                                       Parse TSV → rows
                                            │
                                            ▼
                                       Sheets API:
                                       1. ensure tab inv:Foo exists
                                       2. clear A2:F + write A1:F<N+1>
                                       3. upsert _char_owner row
                                            │
                                            ▼
                              onChange trigger fires server-side
                                            │
                                            ▼
                          Apps Script build_views.ts:
                          - rebuild view:Foo
                          - rebuild gear_check:Foo
                          - if Foo == bank_toon, rebuild bank
                                            │
                                            ▼
                                  Update _meta.last_view_rebuild
```

### Flow B: PigParse refresh (daily)

```
Time-driven trigger 03:00 PT
       │
       ▼
refresh_pigparse.ts:
1. UrlFetchApp GET /api/item/getall/1
2. Parse JSON
3. Replace _pigparse range (full clear + write)
4. Update _meta.last_pigparse_refresh
       │
       ▼
build_item_master.ts (chained):
- For each item_id in _pigparse, upsert _item_master price columns
- Touch all view: tabs that reference any updated item (or just rebuild all — N=12, cheap)
       │
       ▼
Update _meta.last_view_rebuild
```

### Flow C: Wiki refresh (weekly)

```
Time-driven trigger Sunday 04:00 PT
       │
       ▼
refresh_wiki.ts:
For each (class, level) in supported set:
  UrlFetchApp GET wiki.project1999.com/api.php?action=parse&page=...
  Parse wikitext into spell rows
  ETag stored in PropertiesService → 304 = skip
       │
       ▼
For each tier page (Velious_Pre-Raid_Gear, _Raiding_Gear, Iksar):
  Same as above; rows into _wiki_gear_tier
       │
       ▼
For each item_id seen in landing tabs but missing from _item_master:
  GET wiki page for that item (politeFetch with cache)
  Parse summary, quest associations, categories
  Upsert _item_master, _quest_items
       │
       ▼
6-min execution cap approached?
  Save lastProcessedKey to PropertiesService
  Schedule self-resume in 5 min
       │
       ▼
On full completion:
- Rebuild all gear_check:* and spell_check:* tabs
- Update _meta.last_wiki_refresh
```

### Flow D: User opens Search sidebar

```
User clicks SquireBot → Search menu (added by onOpen)
       │
       ▼
HtmlService sidebar opens (300px)
       │
       ▼
User types "Fungi"
       │
       ▼
google.script.run.searchInventory("Fungi")
       │
       ▼
search_sidebar.ts server-side:
1. Read all inv:* tabs (cached for 60s in CacheService)
2. Filter rows where Name contains "Fungi" or ID equals "Fungi" if numeric
3. Group by item, list (char, location, count) tuples
4. Return JSON
       │
       ▼
Sidebar renders:
"Fungi Tunic (id 13128)"
   ↳ Foo: Bank-Slot4, count 1
   ↳ Bar: General1, count 1
```

### Flow E: User updates bank coin manually

Two-path:
- Direct cell edit on `_meta` (works but unguarded; risk of typo overwriting the bank toon name).
- Sidebar form ("Update Bank Coin") with PP/GP/SP/CP fields → server function writes to `_meta`. Recommended.

---

## Build Order — Critique and Refinement

The stack research proposed:
1. P1: OAuth + first-run UX vertical slice
2. P2: file watching + sheet writes
3. P3: Apps Script TS scaffolding + first scrape
4. P4: Sheet UI/tooltips
5. P5: installer/updater

**Critique:** The phase boundaries conflate concerns and put the installer too late. Also, the Apps Script scaffolding work is a prerequisite for *any* sheet-side feature, so blocking it behind two phases of watcher work creates a long latency between when guildies see watcher events arrive and when those events do anything visible. Refined ordering below.

### Refined Phase Decomposition

**Phase 1 — End-to-end thin slice ("upload one inventory file, see it in the sheet")**

Goal: a single guildie can install, OAuth, point at their EQ folder, run `/outputfile inventory`, and see the raw rows appear in a tab. No views, no enrichment, no checklists. **Minimum demonstrable user value.**

Components built:
- Watcher: OAuth (loopback PKCE), wincred storage, fsnotify+debounce, TSV parse, `values.update` write
- Apps Script: `_meta` tab init, `_char_owner` auto-insert helper (called from watcher), basic onOpen menu
- Installer: NSIS per-user, **no auto-update yet**, no code signing yet (warn in README)
- First-run UX: tray icon + browser-rendered "click here to consent" page

Deliverable: developer can run the installer and demonstrate the full happy path. Failure modes (no consent, wrong folder, no internet) all surface in the tray status string.

Does NOT include: spellbook watcher, any wiki/PigParse work, any view tabs, gear check, spell check, search.

**Phase 2 — Watcher polish + spellbook**

Goal: the watcher is robust enough to leave running unattended for weeks.

Components built:
- Spellbook file watcher (mirrors inventory; near-zero new code)
- Watcher autostart on logon (Run-key registry entry in NSIS)
- Logging + log rotation (lumberjack)
- Retry/backoff on Sheets errors; reauth flow when token revoked
- Auto-update pipeline (`minio/selfupdate` + `latest.json`)
- Code signing (if cert is purchased; otherwise documented workaround)

Deliverable: one guildie can install the watcher and forget about it. The other 11 can be onboarded next phase.

Does NOT include: any view tabs or wiki/PigParse data yet.

**Phase 3 — Sheet enrichment foundation (Apps Script + first scrapes)**

Goal: dimension tabs exist with real data. Per-character views render with wiki summaries + prices. The "differentiator" features (gear check, spell check) are still placeholders.

Components built:
- Apps Script TS+esbuild+clasp scaffolding
- `_meta` schema management
- PigParse client (`refresh_pigparse.ts`) on daily trigger
- MediaWiki API client + `politeFetch` helper with ETag/CacheService
- `_item_master` builder (joins `_pigparse` + per-item wiki summaries)
- `_quest_items` builder
- `view:<CharName>` rendering with hyperlink, price column, cell-note tooltip
- onChange trigger wiring

Deliverable: a watcher upload becomes a tooltipped, priced, wiki-linked view tab within ~30 seconds.

Does NOT include: gear/spell progression checklists yet (those need wiki tier scrape + class/level data on `_char_owner`).

**Phase 4 — Differentiator features (gear + spell progression)**

Goal: the unique value propositions ship.

Components built:
- `_wiki_gear_tier` scrape (Velious Pre-Raid, Velious Raiding, Iksar — per class)
- `_wiki_spells` scrape (per class, per level)
- `gear_check:<CharName>` tab builder
- `spell_check:<CharName>` tab builder
- Sidebar form to set class/level on `_char_owner` (one-time per char)
- `bank` tab + manual coin sidebar form

Deliverable: every guildie sees their gear gaps and spell gaps as actionable rows. THE feature.

**Phase 5 — Search + onboarding polish**

Goal: cross-character search works, and onboarding the remaining 11 guildies is a non-event.

Components built:
- HtmlService search sidebar
- onOpen menu polish ("SquireBot" submenu with Search, Refresh, Update Bank Coin, Setup status)
- Onboarding doc / README with screenshots
- Telemetry/observability for support — see "Observability" below
- Sheet-edit guards: hide all `_*` tabs by default; surface a "schema check" warning if a guildie accidentally edits a dimension tab

Deliverable: v1 is feature-complete and onboarding-ready. Roll out to all 12 guildies.

### Build-Order Rationale (Why This Order, Not Another)

The fault line that matters most is **between Phase 2 and Phase 3**, *not* between Phase 3 and Phase 4 as you might initially guess. After Phase 2, you have a robust watcher writing raw data; you can run it for a month, validate the file format, and find watcher bugs without any sheet-side feature work being wasted. After Phase 3, you have rendered views — but no progression checklists. Phases 3 and 4 share a lot of infrastructure (`_item_master`, the scrape harness, the onChange trigger), so splitting them lets Phase 4 deliver "two big features" rather than "one big feature plus a lot of plumbing."

The stack research's original ordering puts the installer last. **This is wrong.** The installer is a Phase 1 deliverable because the OAuth flow IS the installer flow IS the first-run UX. They're inextricable. The auto-updater can come in Phase 2 (unsigned and manual updates work fine for one developer), but the bare installer must ship in Phase 1.

### Dependency Graph (Component-Level)

```
                                   Installer (NSIS)
                                          │
                                          ▼
                                    OAuth (loopback)
                                          │
                                          ▼
                                  Wincred token storage
                                          │
                                          ▼
                              Sheets API client (Go)
                                  │            │
                                  ▼            ▼
                          fsnotify+parse   _char_owner upsert
                              │
              ┌───────────────┼───────────────┐
              ▼                               ▼
      inv: tab writes                  spell: tab writes
              │                               │
              └───────────┬───────────────────┘
                          │
                  Apps Script scaffolding
                  (TS + esbuild + clasp)
                          │
            ┌─────────────┼─────────────────────┐
            ▼             ▼                     ▼
       _meta init    onChange trigger      Time-driven triggers
                          │                     │
                          ▼                     │
                 build_views.ts                 │
                          │            ┌────────┴────────┐
                          │            ▼                 ▼
                          │      refresh_pigparse   refresh_wiki
                          │            │                 │
                          │            ▼                 ▼
                          │       _pigparse          _wiki_spells
                          │                          _wiki_gear_tier
                          │                          _quest_items
                          │            │                 │
                          │            └─────┬───────────┘
                          │                  ▼
                          │            _item_master
                          │                  │
                          ▼                  ▼
                  view:<Char>          (used by all builders)
                  bank
                          │
                          ▼
                  gear_check:<Char>
                  spell_check:<Char>
                          │
                          ▼
                  search sidebar
                  onOpen menu
                          │
                          ▼
              Watcher polish (autostart, updater, signing)
```

### Component → Phase Assignment (Roadmap-Ready)

| Phase | Components | Depends On |
|-------|-----------|------------|
| **P1** | NSIS installer, OAuth+PKCE, wincred, Sheets client, fsnotify+parse, inv: writes, _char_owner upsert, _meta init, onOpen | (greenfield) |
| **P2** | spell: writes, autostart, logging+rotation, retry/backoff, reauth flow, auto-updater, code signing | P1 (entire watcher) |
| **P3** | Apps Script scaffolding, refresh_pigparse, _pigparse, refresh_wiki (item summaries only), _quest_items, _item_master, build_views.ts, view:<Char>, onChange trigger | P1 (data in landing tabs); independent of P2 |
| **P4** | refresh_wiki (gear tier + spell list), _wiki_gear_tier, _wiki_spells, _char_owner.class/level, gear_check:<Char>, spell_check:<Char>, bank tab, manual coin sidebar | P3 (item_master + scrape harness) |
| **P5** | search sidebar, onOpen polish, schema migration, _meta guards, onboarding doc | P3 + P4 |

**Note:** P2 and P3 are technically parallelizable if you have two contributors. They share no code paths (P2 is Go; P3 is Apps Script TS). With one contributor, do them sequentially in the order shown.

---

## Cross-Cutting Concerns

### Observability — "How do I debug Bob's broken watcher without his cooperation?"

Three layers:

1. **Watcher local log** (`%AppData%\SquireBot\squirebot.log`, lumberjack-rotated 5MB × 3). Structured `slog` JSON with timestamp, event type, file path, sheet write status, error code. Bob runs `Reveal in Explorer` from the tray menu, zips the log, sends it to support.

2. **Sheet-side audit trail** in a hidden `_audit` tab, written by the watcher on every successful upload (or every Nth upload to control volume): `(timestamp, char, owner_email, row_count, latency_ms)`. This gives the developer a real-time view of "are uploads happening?" without bothering Bob. **CRITICAL for debugging "guildie X's data is stale" complaints.**

3. **Apps Script execution logs** (`Stackdriver` / Apps Script logs view) for refresh job failures. Apps Script logs are visible to script owners only. Add a "last error" field to `_meta` so the workbook itself surfaces "the wiki refresh failed last Sunday" without the developer needing to check the Apps Script editor.

**Limitations to flag:**
- Apps Script logs are 7-day retention by default; no easy tail-on-error.
- Cell-level "who edited what when" requires `Sheet.getProtections()` + version history checks; not real-time. The `_audit` tab fills this gap for upload provenance.
- Watcher errors that prevent the watcher from starting (OAuth failure, missing config) won't reach the sheet at all — the only signal is the local log + tray status. Tray status string must be informative enough to debug "is your light red?" over Discord chat.

**Rationale for `_audit`:** This is the single highest-leverage observability investment. With it, developer can answer "why is X's data stale?" in 30s by inspecting the tab. Without it, every staleness complaint requires a Discord round-trip with Bob.

### Configuration — Watcher vs. Sheet

**Watcher config (`%AppData%\SquireBot\config.json`):**

```json
{
  "version": 1,
  "eq_folder": "C:\\Users\\Bob\\EverQuest",
  "spreadsheet_id": "1aBcDeFg...",
  "log_level": "info",
  "last_known_inventory_mtime": { "Foo-Inventory.txt": "2026-04-30T10:23:15Z", ... },
  "google_email": "bob@gmail.com"
}
```

Notes:
- `eq_folder` is set during first-run UX (file picker dialog defaulting to common EQ install paths).
- `spreadsheet_id` is set during first-run UX via the Google Picker (per STACK.md `drive.file` scope guidance).
- `last_known_inventory_mtime` lets the watcher do a startup catch-up: on launch, scan the EQ folder, compare mtimes, and re-upload anything that changed while the watcher wasn't running. Critical for "I closed my laptop for a week" scenarios.
- `google_email` is cached for the tray status display; the source of truth is wincred + `userinfo.email` lookup.
- **Refresh tokens DO NOT live here** — those are wincred-only.

**Sheet-side config (in `_meta` tab):**

- `schema_version` — tracked by Apps Script and watcher both
- `bank_toon_name` — which char is the bank
- `bank_coin_*` — manually-edited coin amounts
- `pigparse_refresh_hour` (default 3) — cadence override
- `wiki_refresh_dow` (default 0=Sunday), `wiki_refresh_hour` (default 4) — cadence override
- `bank_toon_class`, `bank_toon_level` — for the bank toon since it's special-cased

**`_char_owner` tab:** char-level config (class, level, display name, discord handle, is_bank_toon flag). Editable by guildies via sidebar form.

**Why this split?** Anything that varies per-watcher-install is in the local config (folder path, OAuth identity). Anything that's a property of the workbook universe (which char is the bank, what schema version we're on) is in `_meta`. Anything per-character is in `_char_owner`. No config is duplicated across watchers; the "12 guildies must agree on X" problem doesn't exist for any setting we have.

### Schema Evolution

**Principle:** Apps Script can't ALTER TABLE. So we never break, only extend.

**Allowed changes (no schema bump):**
- Add a new column to a tab (always at the right edge; existing readers ignore it).
- Add a new dimension tab.
- Add a new row to `_meta`.

**Disallowed changes (require schema_version bump + migration):**
- Renaming a column or tab.
- Changing column types or semantics.
- Removing a column or tab.

**Migration mechanism:**

1. Bump the constant `SCHEMA_VERSION` in both watcher Go code and Apps Script TS code.
2. Add a migration function `migrate_v1_to_v2(spreadsheet)` in `schema_migrate.ts`.
3. On every Apps Script trigger entry point, call `ensureSchema()` which reads `_meta.schema_version`, runs any pending migrations (which are idempotent), and updates `_meta.schema_version` on success.
4. Watcher refuses to run if `_meta.schema_version > WATCHER_MAX_SCHEMA_VERSION`. This forces auto-updater path.
5. Apps Script refuses to run if `_meta.schema_version < SCRIPT_MIN_SCHEMA_VERSION`. This catches "someone restored an old backup."

**Risk: workbook duplication.** A guildie copies the workbook (e.g., to make a personal scratch copy). Their scratch copy has a `_meta` and might trigger time-driven jobs they didn't ask for. Mitigation: the script checks `SpreadsheetApp.getActiveSpreadsheet().getId()` against a hardcoded canonical ID stored in `_meta.canonical_id`; if mismatched, all triggers no-op with a warning. (The watcher writes to a specific spreadsheet ID by config, so this is a sheet-side problem only.)

### Identity — "Is this character mine?"

The model:

1. **Primary identity = OAuth email.** The watcher knows its email from the OAuth `userinfo` endpoint. It includes the email in writes to `_char_owner` for any new character. This is the only stable, automatic identity signal we have.

2. **First-write wins for `_char_owner.owner_email`.** When watcher A first writes `inv:Foo`, the row `Foo → A's email` is inserted. If watcher B later tries to write `inv:Foo`:
   - The actual `inv:Foo` write succeeds (we don't gate writes on owner — that's brittle).
   - The watcher checks `_char_owner.Foo.owner_email` and if mismatched, writes a row to `_audit` flagging the discrepancy. Surface via sidebar in the SquireBot menu.
   - An officer manually resolves: edit `_char_owner.Foo.owner_email` if the toon was legitimately handed off, or contact whoever's misconfigured.

3. **Filename convention is NOT used as identity.** P99 character names are unique per-server but not unique per-guildie (a guildie can have many alts). So `<CharName>` in the filename is the *character* identity, not the *guildie* identity. The OAuth email gives us guildie identity.

4. **The "I have two PCs" case.** Bob installs the watcher on his desktop AND his laptop, both writing the same chars. Both watchers OAuth as bob@gmail.com → no conflict, `_char_owner` agrees. The two watchers will race on the same `inv:Bob's-Char` tab, but since both are writing the same canonical file content from EQ on the same machine... wait, they're on different machines, so each PC's EQ folder has its own state. **Race condition: if Bob plays on his desktop on Monday and his laptop on Tuesday, both watchers will see the file but only one's data will be current.** The current = whichever watcher saw the most recent local change. This is fine in practice (he's the same person, only playing on one PC at a time), but document it.

5. **v2 prep: Discord handle.** `_char_owner.discord_handle` is editable in the sidebar. Pre-populate it as a v1 column even though v1 doesn't use it; this avoids a schema bump in v2.

**Confidence:** HIGH. This model maps cleanly to OAuth's identity guarantees and avoids cross-cutting state.

---

## Architectural Patterns

### Pattern 1: Full-Snapshot Replace for Watcher Writes

**What:** Watcher always writes the *entire current state* of a file as a range update; never appends, never row-diffs.

**When to use:** When the source data is small (<10K rows) and you control both the producer and consumer.

**Trade-offs:**
- Pro: idempotent, retry-safe, no diff logic, no PK management, deletions handled for free.
- Con: more bytes per write (negligible at our scale); race window where a reader sees post-clear/pre-write state (mitigated by `batchUpdate` atomicity).

**Example:**
```go
// Pseudo-Go
rows := parseInventory(file)
ssvc.Spreadsheets.BatchUpdate(spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
    Requests: []*sheets.Request{
        // Clear A2:F (header stays)
        {UpdateCells: &sheets.UpdateCellsRequest{
            Range: tabRange("inv:Foo", "A2:F"),
            Fields: "*",
        }},
        // Write all rows
        {UpdateCells: &sheets.UpdateCellsRequest{
            Range: tabRange("inv:Foo", "A1:F" + strconv.Itoa(len(rows)+1)),
            Rows: toRowData(rows),
            Fields: "userEnteredValue",
        }},
    },
}).Do()
```

### Pattern 2: Versioned Tab Names for Schema Migration

**What:** When a tab's schema breaks, the new version becomes a new tab name (`inv_v2:<Char>` next to legacy `inv:<Char>`). Old tab is read-only thereafter.

**When to use:** When you need to evolve schema without forcing every client to upgrade simultaneously.

**Trade-offs:**
- Pro: zero-downtime migration; old watchers still write to `inv:<Char>` until they update; new watchers write to `inv_v2:<Char>`; readers (Apps Script) understand both.
- Con: tab proliferation; readers must know which version they're handling.

**When NOT overkill:** if you're confident the watcher will always self-update (`minio/selfupdate`) before Apps Script changes ship, you can do in-place schema bumps with a migration. For a 12-person guild, in-place is probably fine. Versioned-tabs is the escape hatch for "Bob's watcher has been offline for 3 months."

### Pattern 3: Onchange-Triggered View Builds + Time-Triggered Dimension Refreshes

**What:** Two distinct trigger types: "rebuild the affected view immediately when raw data lands" (onChange) vs. "refresh external data on a schedule" (time-driven). Don't conflate.

**When to use:** Whenever you have a clear separation between "data the user touches" (event-driven) and "data the world generates" (schedule-driven).

**Trade-offs:**
- Pro: the view is fresh seconds after upload; external scrapes don't burn quota on every upload.
- Con: two trigger paths to maintain; `onChange` fires on any cell edit (including manual edits to `_meta`), so the handler must be cheap and idempotent.

**Example:**
```typescript
function onChange(e: GoogleAppsScript.Events.SheetsOnChange): void {
  if (!e.source) return;
  const sheet = e.source.getActiveSheet();
  const name = sheet.getName();
  if (name.startsWith("inv:")) {
    rebuildView(name.slice(4));  // "inv:Foo" → rebuild view:Foo, gear_check:Foo
  } else if (name.startsWith("spell:")) {
    rebuildSpellViews(name.slice(6));
  }
  // Manual edits to _meta etc. are no-ops.
}
```

### Pattern 4: Polite Fetch with ETag + CacheService

**What:** Every external HTTP request goes through `politeFetch(url)` which: (a) checks CacheService, (b) sends `If-None-Match` from PropertiesService, (c) honors 304 fast-path, (d) caches response 6h, (e) backs off on 429/503.

**When to use:** Always, when fetching from external services in Apps Script. Single helper, every caller benefits.

**Trade-offs:**
- Pro: significantly reduces UrlFetch quota burn; respects external services; resumes correctly on retry.
- Con: cache + ETag + retry interactions can mask "real" errors if not logged carefully.

---

## Anti-Patterns

### Anti-Pattern 1: Row-Level Upsert from the Watcher

**What people do:** Watcher reads inventory, diffs against last known state, sends only changed rows via `values.update`.

**Why it's wrong:** Requires per-row primary keys (`Location` is the obvious one but locations move when a player rearranges bags). Diff logic is error-prone. Deletions need explicit "delete this row" calls. State must be persisted locally to compute the diff. Recovery from "watcher state is corrupted" requires a full resync — which is exactly what full-snapshot replace does every time anyway.

**Do this instead:** Pattern 1. Always full-snapshot replace.

### Anti-Pattern 2: Apps Script Polls Sheets for "New Uploads"

**What people do:** Time-driven trigger every minute that scans `inv:*` tabs for `_uploaded_at > last_seen` to trigger view rebuilds.

**Why it's wrong:** Burns trigger budget (90 min/day total cap is finite if you also have refresh jobs). 60-second latency on view freshness when onChange is instant. Adds a stateful "last_seen" timestamp to manage.

**Do this instead:** `installable onChange` trigger. Fires on writes from the watcher (Sheets API writes do trigger onChange), and the handler dispatches to the right view rebuilder by tab name prefix.

### Anti-Pattern 3: Putting Wiki/PigParse Refresh in the Watcher

**What people do:** Have the Go watcher hit the wiki + PigParse on a schedule and write enriched data to the sheet.

**Why it's wrong:** 12 watchers × wiki = 12× the load on a community resource. If only one designated watcher does refreshes, that PC becomes a single point of failure (Bob is on vacation = no price updates for a week). Apps Script runs server-side on Google's infrastructure with guaranteed cadence.

**Do this instead:** All scraping in Apps Script. Watcher is a dumb pipe for landing data.

### Anti-Pattern 4: Service Account or Shared Credentials

**What people do:** Single service account JSON key distributed to all guildies; or one "guild bot" Google account whose password is shared.

**Why it's wrong:** Already-rejected per PROJECT.md. Distributes a long-lived credential, violates idiot-proof setup, kills attribution (`_char_owner` can't tell who wrote what).

**Do this instead:** Per-guildie OAuth with `drive.file` scope. STACK.md is locked on this.

### Anti-Pattern 5: Cell Notes for Rich Tooltips

**What people do:** Try to put HTML/links/formatted text in `Range.setNote` to get rich tooltips.

**Why it's wrong:** `setNote` is plain text only. HTML is rendered as text. Verified in Apps Script docs.

**Do this instead:** Two-tier UX. Cell notes for short, hover-to-glance info ("Quest: Tradeskill", "Avg 1500pp", first 80 chars of summary). Sidebar (HtmlService) for the rich, clickable detail when the user wants to dig in. Wiki link is a hyperlink in the cell formula, not in the note.

### Anti-Pattern 6: Globally Hidden Apps Script Errors

**What people do:** Wrap top-level trigger handlers in `try { ... } catch (e) { /* ignore */ }` to avoid users seeing errors.

**Why it's wrong:** Refresh jobs silently fail. Days later, prices are stale and nobody knows why. Apps Script's default behavior (email script owner on error) is the right default — keep it.

**Do this instead:** Catch + log + write to `_meta.last_error` so the workbook surfaces the failure visibly. Re-throw so Apps Script's email-on-error fires.

---

## Integration Points

### External Services

| Service | Integration Pattern | Notes / Gotchas |
|---------|---------------------|-----------------|
| **Google Sheets API v4** | Watcher uses `google.golang.org/api/sheets/v4` with `batchUpdate` for atomic clear+write; Apps Script uses `SpreadsheetApp` global | 60 req/min/user write quota; we're 2 orders below it. `batchUpdate` is the only way to do atomic clear+write. |
| **Google OAuth (auth)** | Loopback PKCE on `127.0.0.1:N` per STACK.md; `drive.file` scope; refresh token in wincred | Picker required on first run because `drive.file` only grants per-file access. Picker JS in a tiny embedded HTML page on the loopback server. |
| **Google Drive Picker** | Loaded once on first run via embedded HTML; user selects the shared workbook | Requires Picker API JS; one-time permission grant; spreadsheet ID returned and stored in `config.json`. |
| **PigParse REST** | `UrlFetchApp.fetch("/api/item/getall/1")` from Apps Script daily | Verified Swagger endpoints in STACK.md. JSON response. No auth required currently — verify before launch. |
| **MediaWiki API (P1999 wiki)** | `UrlFetchApp.fetch("api.php?action=parse&page=...")` from Apps Script weekly | `action=parse` for HTML, `action=query&prop=revisions&rvprop=content` for raw wikitext. Polite User-Agent, ETag caching. |
| **GitHub Releases** | Watcher's auto-updater fetches `latest.json` manifest and signed binary from a specific repo's releases | Requires public repo or unauth-readable releases. Signed binary recommended; if unsigned, document SmartScreen workaround. |
| **(v2) Discord API** | Always-on bot reading designated channels in 3 external Discord servers | Requires admin invite from those server owners — PROJECT.md flags as currently un-negotiated. Runs on Cloudflare Workers or GitHub Actions cron per STACK.md, NOT in Apps Script. |

### Internal Boundaries

| Boundary | Communication Mechanism | Considerations |
|----------|------------------------|----------------|
| **Watcher (Go) ↔ Sheets API** | HTTPS (gRPC-over-HTTP via google-api-go-client) | All writes are batched; watcher has no other dependencies on the sheet's contents (does not read view tabs). |
| **Watcher ↔ Apps Script** | Indirect, via the sheet (watcher writes, onChange fires script) | NO direct invocation from watcher to Apps Script. Sheet IS the API contract. |
| **Apps Script ↔ Sheet** | `SpreadsheetApp` globals (synchronous, batched via `Range.setValues`) | All script writes go through the script. Apps Script should NEVER write to landing tabs (`inv:`, `spell:`). |
| **Apps Script triggers ↔ Apps Script triggers** | Indirect, via PropertiesService state + chained scheduling | E.g., `refresh_wiki` saves `lastProcessedKey` and schedules itself if it bumps the 6-min cap. |
| **Watcher (Go) ↔ Wincred** | CGO-free Go wrapper (wincred uses syscall) | DPAPI under the hood. User-scoped — moving config dir between users won't work; document this for Windows multi-user PCs. |
| **Watcher ↔ Tray UI** | In-process (`fyne.io/systray` callbacks) | Single goroutine; menu callbacks must not block on I/O — fire-and-forget. |
| **Search sidebar ↔ Apps Script server** | `google.script.run` async calls | Must serialize JSON; can't pass functions or large blobs. |

---

## Risk Callouts (Per Architectural Choice)

| Choice | Risk | Mitigation |
|--------|------|------------|
| Full-snapshot replace | Range races: a reader sees post-clear/pre-write state. | Use `batchUpdate` to make clear+write atomic in one API call. |
| Tab-per-character | Tab count grows unbounded as guildies create alts. Sheet's hard limit is 200 tabs/workbook. | 12 guildies × ~10 chars each × 5 view types = 600 tabs. **EXCEEDS LIMIT.** Mitigation: consolidate per-char view tabs into one filterable consolidated view per type (consolidated `view`, `gear_check`, `spell_check` with a Char column, filterable by views/dropdowns). Keep landing tabs per-char (they're 60 max for inventory/spellbook combined; well under limit). **HIGH-CONFIDENCE FLAG: revisit "tab-per-character" for views; it likely won't survive scale.** |
| Sheets API as the only watcher↔script channel | Dependency on onChange trigger reliability. | onChange has been stable per Apps Script docs; but add a "manual rebuild" sidebar button as escape hatch. |
| Apps Script as scrape host | 6-min execution cap; weekly wiki refresh might exceed when many new items appear. | Re-entrant design with `PropertiesService.lastProcessedKey` state. Self-reschedule if cap approached. |
| OAuth email as identity | Guildie changes Google accounts and re-OAuths under a different email. | Surface mismatch warning in `_audit`; officer manually updates `_char_owner.owner_email`. |
| Schema versioning | Old watchers running schema_v1 against schema_v2 sheet write malformed data. | Watcher refuses to run if `_meta.schema_version > WATCHER_MAX`. Tied to auto-updater. Bob's watcher will pop an "Update required" tray notification. |
| `drive.file` scope + Picker | Guildie picks the wrong spreadsheet by accident. | Validate the picked spreadsheet has expected `_meta` content on first connect; show name + ID confirmation in tray. |
| Manual coin field on `_meta` | Anyone can typo-overwrite `bank_toon_name`. | Sidebar form for coin edits; `_meta` should also have read-only protection ranges except for the explicit coin cells. Apps Script `Sheet.protect()` API supports this. |
| Apps Script logs are 7-day | Debugging "what failed last month" is impossible. | `_audit` tab + `_meta.last_error` provide longer-lived traces in the workbook itself. |
| 12 watchers writing concurrently to one workbook | Per-spreadsheet write quota is 60/min combined. | 12 guildies × ~10 writes/day = 120/day = 0.08/min average. Burst limit: 12 simultaneous `/outputfile` events would be 12 writes in <1s, well under 60/min. Safe. |

---

## Sources

- STACK.md (sibling doc) — locked stack decisions
- FEATURES.md (sibling doc) — feature inventory and dependency analysis
- PROJECT.md — requirements and constraints
- [Google Sheets API: batchUpdate](https://developers.google.com/sheets/api/reference/rest/v4/spreadsheets/batchUpdate) — atomic clear+write pattern
- [Apps Script Triggers](https://developers.google.com/apps-script/guides/triggers/installable) — onChange + time-driven
- [Apps Script Sheets limits](https://developers.google.com/apps-script/guides/services/quotas) — 200 sheets/workbook is the hard tab limit driving the per-char view consolidation flag
- [PigParse Swagger](https://pigparse.azurewebsites.net/swagger/index.html) — verified endpoints
- [P1999 wiki API](https://wiki.project1999.com/api.php) — verified MediaWiki API
- [SpreadsheetApp.protect()](https://developers.google.com/apps-script/reference/spreadsheet/sheet#protect) — for guarding `_meta` ranges

---
*Architecture research for: SquireBot (Windows watcher + Google Sheets + scraping)*
*Researched: 2026-04-30*
