# Phase 29: Data Foundation — Inventory Parse + Price/Value Aggregation - Research

**Researched:** 2026-06-17
**Domain:** Go backend (`internal/backendsrv`) — compute-on-read transforms over SQLite, EQ inventory-string parsing, PigParse name-join aggregation
**Confidence:** HIGH on the join/schema/compute findings (read directly from current code + git history); MEDIUM on the exact EQ container-nesting sub-slot indexing (community-corroborated, but the repo has no real nested fixture yet — which is precisely what success-criterion #4 requires this phase to add)

---

## Summary

The single load-bearing finding: **the item_id-vs-name join contradiction is already resolved in the live code, in favor of normalized name.** On 2026-06-06, commit `0a169f3` ("fix(view): price bank/view rows by normalized name, not catalog item_id") rewrote `store.InventoryJoin` (`readviews.go:135-213`) to bridge inventory→price by `lower(trim(name))` through a `pp_rep` CTE, because the PigParse catalog item_ids and the EQ `/outputfile` inventory item_ids are **different namespaces** (~58/713 inventory ids matched the catalog by id, vs ~559 by name — the old `pp.item_id = ii.item_id` join silently left ~91% of held rows unpriced). DATA-01 is therefore not a new pattern to invent; it is the *already-shipped* pattern that Phase 29 extends to the gear-tier (no-item_id) rows. **The stale `[ASSUMED]`-style comments in `compute/view.go:84-95` and `compute/types.go:62-63` still claim "the join is one row per item (pigparse_price.item_id is the PK)" — those comments are WRONG as of `0a169f3` and must not be trusted by the planner.** CLAUDE.md's "item_id is the stable join key" is also wrong for the price join (it remains correct only for `item_master`, the watcher's own EQ-namespace enrichment).

The recommended implementation is **pure compute-on-read, zero schema migration.** All three requirements (INV-05 slot taxonomy/nesting, DATA-01 name-keyed price+last-listed, DATA-02 bank valuation+platinum) are derivable from data already in the store: `inventory_item.location/slots/count`, `pigparse_price.a30/direction/last_seen` (joined by name), and `character.plat` (via `ListBankToons`). The slot-classification + container-nesting logic is a **pure string parse** over `inventory_item.location` — no new column needed; guild data is tiny (<100 MB, ~50–150 writes/day) so recomputing on each read is free. The new code is a `compute/inventory.go` (or `compute/slotmodel.go`) pure transform + a handful of new `store` read methods + new `BankValuation`/structured-inventory functions, mirroring the existing `compute/view.go` ↔ `store/readviews.go` seam exactly.

**Primary recommendation:** Build a pure `compute` slot-taxonomy + nesting transform over a new name-keyed store read that returns *all* inventory rows (including empty-slot/`item_id=0` rows, unlike `InventoryJoin` which filters them out — INV-05 needs container shells and bag structure). Reuse `pickPrice` verbatim for valuation. Add `pigparse_price.last_seen` to the price-join projection (it is already stored — it is the DATA-01 "last-listed" date). Do NOT migrate the schema. Fix the stale `view.go`/`types.go` comments as part of the work. Do NOT expose new HTTP endpoints in Phase 29 unless a thin read endpoint is the cheapest way to unit-prove the aggregates — leave the tab-shaped endpoints to Phases 31–34.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Parse `Location`/`Slots` → slot taxonomy + nesting (INV-05) | API / Backend (`compute`) | — | Watcher is untouched (CLAUDE.md hard constraint); the watcher passes the raw 5 columns through (`internal/parse/inventory.go`). All interpretation is server-side compute. |
| Name-keyed PigParse price + last-listed join (DATA-01) | Database / Storage (`store` SQL) | API (`compute` shaping) | The join is a parameterized SELECT in `store`; `compute` shapes the result. Already the live pattern (`readviews.go` `pp_rep` CTE). |
| Bank valuation + total platinum (DATA-02) | API / Backend (`compute`) | Database (`store` reads) | Aggregation is a pure sum over store reads (`InventoryJoin(bankOnly)` + `ListBankToons`). |
| Surfacing the model to the web | (deferred to Phases 31–34) | — | Phase 29 produces compute functions + store reads; the HTTP/tab surfaces are explicitly out of scope (CONTEXT `Out of phase scope`). |

---

## Standard Stack

No new dependencies. Phase 29 is pure stdlib Go over the existing seams.

### Core (existing, reused verbatim)
| Component | Location | Purpose | Why Standard |
|-----------|----------|---------|--------------|
| `compute` package | `internal/backendsrv/compute/` | Pure compute-on-read transforms (View/Bank/GearCheck/SpellCheck) | The established BACKEND-05 pattern; `compute` imports `store`+`enrich`, authors zero SQL `[VERIFIED: compute/types.go:1-11]` |
| `store` read seam | `internal/backendsrv/store/readviews.go` | Parameterized SELECT/JOIN read methods | The single tested SQL path; `?` placeholders only `[VERIFIED: readviews.go:14-24]` |
| `pickPrice` | `compute/view.go:117-138` | WTS `a30` → WTB `a30` fallback price pick | D-03 mandates reusing this verbatim for valuation `[VERIFIED]` |
| `pp_rep` name-join CTE | `readviews.go:144-159` | inventory→price bridge by `lower(trim(name))` | DATA-01's join strategy, already shipped `[VERIFIED: git 0a169f3]` |
| `ListBankToons` | `store/coin.go:46-69` | live bank toons (`is_bank_toon=1 AND is_removed=0`) + coin | D-04 platinum source `[VERIFIED]` |
| `goose` migrations | `internal/backendsrv/migrations/` | extend-only schema (current = `00011`) | Only needed IF a column is added — research recommends NOT `[VERIFIED: 11 migration files]` |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Pure compute-on-read | Materialized parsed-slot column on `inventory_item` (`00012` migration) | Adds write-path coupling (every `ReplaceInventory` would need to compute the column), a migration, and a backfill — all to save microseconds on a <150-row-per-char read. **Rejected**: guild scale makes compute-on-read free; CONTEXT D defaults to compute-on-read; materialize "only if research surfaces a concrete reason" — none found. |
| Reusing `InventoryJoin` for the slot model | New `store` read that keeps empty-slot + container rows | `InventoryJoin` filters `ii.item_id > 0` (`readviews.go:159`), dropping empty equipment slots and (potentially) empty bag shells the paperdoll/nesting needs. **A new read method is required** (see Files to Create). |

---

## Research Question 1 — PigParse item_id namespace reconciliation (VERDICT)

**VERDICT: Join by normalized name. This is already the live behavior; the contradiction is in the stale comments and in CLAUDE.md, not in the running SQL. Do NOT reintroduce an item_id price join. Phase 29 extends the name-join to gear-tier rows.**

### Evidence (decisive)

1. **The live `InventoryJoin` already joins price by name, not item_id.** `store/readviews.go:144-159`:
   ```
   WITH pp_rep AS (
     SELECT lower(trim(name)) AS norm_name, MIN(item_id) AS rep_item_id
     FROM pigparse_price WHERE name IS NOT NULL AND trim(name) <> ''
     GROUP BY lower(trim(name))
   )
   ...
   LEFT JOIN item_master im     ON im.item_id = ii.item_id      -- id-keyed (correct: watcher's own EQ namespace)
   LEFT JOIN pp_rep             ON pp_rep.norm_name = lower(trim(ii.name))
   LEFT JOIN pigparse_price pp  ON pp.item_id = pp_rep.rep_item_id   -- price reached via name → rep id
   ```
   The doc comment at `readviews.go:33-45` states the rationale verbatim: *"the PigParse catalog (pigparse_price) and the EQ /outputfile inventory (inventory_item) are DIFFERENT item_id namespaces (only ~58/713 inventory ids exist in the catalog by id, vs ~559 names matching by name), so the old pp.item_id = ii.item_id join silently left ~91% of held rows unpriced."* `[VERIFIED: readviews.go:33-45, 144-159]`

2. **Git history confirms this was a deliberate fix, dated 2026-06-06.** `git log -S "pp_rep"` → commit `0a169f3` "fix(view): price bank/view rows by normalized name, not catalog item_id". `[VERIFIED: git log]`

3. **The PigParse parser *claims* `i` is the "EQ item ID"** (`enrich/pigparse.go:43`, json tag `i` commented "EQ item ID"), and the stored `pigparse_price.item_id` is that value. **But the project's lived experience (and the 0a169f3 measurement) proves the catalog id space does not reliably coincide with the in-game inventory id space** — consistent with project memory `pigparse-vs-ingame-item-id-namespaces`. The parser's comment is aspirational; the join behavior is empirical. Trust the measurement. `[VERIFIED: pigparse.go:43 + readviews.go:33-45]`

4. **The stale comments that the planner must IGNORE:** `compute/view.go:84-95` (`pricesFromJoin` says "the join is one row per item (pigparse_price.item_id is the PK)") and `compute/types.go:62-63` (`PriceDetail` doc says "Because pigparse_price.item_id is the PRIMARY KEY, the join yields at most ONE price row per item"). These describe the *pre-0a169f3* join. They are now wrong about *why* there's one row per item — the one-row guarantee now comes from the `pp_rep` CTE's `GROUP BY norm_name` + `MIN(item_id)` fan-out guard, NOT from the PK. The result (≤1 PriceDetail per ViewRow) is still correct; only the explanation rotted. `[VERIFIED]` **Recommend: fix these two comments in Phase 29** (cheap, prevents the next dev re-introducing the bug).

### What this means for Phase 29

- **The inventory price join is CORRECT today** — it is name-keyed and prices ~559 names instead of ~58 ids. Do not "fix" it back.
- **DATA-01's gear-tier join is the same pattern, one hop further.** `wiki_gear_tier.item_id` is *always NULL* (`store/enrich.go:84-92`, `enrich/wikigear.go:30-40`), so gear-tier rows can ONLY be priced by name. The exact same `lower(trim(item_name))` → `pp_rep.norm_name` bridge applies. A gear-tier price lookup is: `lower(trim(wiki_gear_tier.item_name))` joined to `pp_rep`. This is what Phase 34 (WISH-04) consumes; Phase 29 builds the store read + compute helper.
- **Safest normalization = `lower(trim(name))`** — the project-wide convention (`ReplaceSpellbookTx` `replace.go:169`, `wiki_spells.normalized_name`, the `pp_rep` CTE, the `seedPigparse` test helper note `fixtures_test.go:84-86`). CONTEXT D-discretion already locks this. Do NOT invent a fancier normalizer (no punctuation stripping, no apostrophe folding) — it must match the existing join key exactly or the existing `view` prices break.

`[ASSUMED]` flag: the precise "~58/713 / ~559" counts come from the `0a169f3` author's one-time measurement embedded in the comment; they are not re-verified this session and will drift as the catalog/guild changes. The *direction* of the finding (name >> id coverage) is HIGH confidence; the exact ratios are illustrative.

---

## Research Question 2 — Real `/outputfile inventory` container-nesting format

**Format: `<ParentSlot>-Slot<N>`. The synthetic fixture is flat and has NO nested rows, so this is a genuine unknown that Phase 29 must pin with a real-name fixture (success-criterion #4).**

### What the codebase tells us
- `internal/parse/inventory.go` does **zero** Location/Slots interpretation — it splits on TAB, keeps exactly 5 columns `[Location, Name, ID, Count, Slots]`, drops a header, skips non-int-ID rows. No nesting awareness whatsoever. `[VERIFIED: inventory.go:53-81]`
- `store/replace.go:96-114` stores `location` as raw TEXT, `slots` as an INTEGER, no parsing. `[VERIFIED]`
- The synthetic fixture `internal/parse/testdata/sample-inventory.txt` is **flat**: top-level slots only (`General1`..`General10`, `Bank1`..`Bank8`, the 21 equipment slots), `Slots` column is a per-row 0/8/10 "capacity" number with no corresponding `*-SlotN` child rows. It does NOT exercise nesting. `[VERIFIED: read all 250 rows]`

### What community sources confirm (MEDIUM confidence)
- Items inside a container are emitted as additional rows whose `Location` is `<ParentSlot>-Slot<N>` — e.g. `General1-Slot1`, `General4-Slot3`, `Bank1-Slot1`. Augment slots use the same suffix on equipment (`Ear-Slot1`, `Head-Slot1`). `[CITED: Project1999 inventory-parser community sources via WebSearch — "following patterns like 'General1-Slot' for items in bag slots"; "Charm/Ear/Ear-Slot1/Head/Head-Slot1"]`
- General bag slots are `General1`..`General10`; bank main slots `Bank1`..`Bank8`; both can hold containers, and `/outputfile inventory` "drops all your inventory items including bags, contents of bags, bank and shared bank." `[CITED: Fanra wiki / EQ Traders / P99 community]`
- "Slots inside bags are +1 to what Titanium uses" — i.e. **sub-slots are 1-indexed** (`-Slot1` is the first bag slot, not `-Slot0`). `[CITED: eqemu docs community discussion]` — MEDIUM confidence; **confirm against the real dump.**

### Open sub-questions to resolve WITH the real fixture (do not guess in code)
1. **Exact sub-slot indexing** (`-Slot1` first vs `-Slot0` first). Treat as 1-indexed per the source above, but the parser should not *depend* on contiguity — derive nesting from the `-Slot` *suffix presence*, not from an assumed count.
2. **Bag-in-bank**: do bank bags nest as `Bank1-Slot1`? Sources say yes (bank contents are dumped). The parser must treat a `-Slot` suffix on a `Bank<N>` parent identically to a `General<N>` parent.
3. **Bags-in-bags**: classic EQ does NOT allow a container inside a container, so nesting is **one level deep** (parent → children, never grandchildren). Build the tree as parent→children only; if a `-Slot` token ever itself has a `-Slot` child, log and flatten (defensive). `[ASSUMED — classic-EQ rule; confirm no grandchild rows in the real dump]`
4. **`Slots` semantics**: `Slots` = the container's capacity (0 ⇒ not a container). It is the watcher's per-row capacity, NOT the count of occupied children. The number of child rows ≤ `Slots`. Use child-row presence (the `-Slot` suffix), NOT the `Slots` number, to build nesting. `[CITED + matches fixture: containers in the fixture carry Slots=8/10]`

### Parse algorithm the planner should specify
Given all `inventory_item` rows for a character:
1. Classify each `Location` by splitting on the **first** `-` into `(parent, subSlot)`. If no `-`, the row is a **top-level** slot (`General1`, `Bank3`, `Primary`, `Head`, `Charm`, …) or an augment-bearing equipment slot.
2. A row whose `parent` token equals another row's top-level `Location` AND whose suffix matches `^Slot\d+$` is a **child** of that container; nest it.
3. The container row itself stays a first-class item (D-02: "the bag itself is also a priced item that counts").
4. Caveat: equipment augments (`Head-Slot1`) share the `-Slot` shape but the parent (`Head`) is an *equipment* slot, not a container. **Decide explicitly** whether augments are surfaced as nested children of the equipment item or ignored — the in-game inventory window (sketch 002 Variant D) shows bag drill-down, not augments, so augments can be **ignored/flattened for INV-05** but the parser must not crash on them. Flag this to the planner as a one-line decision. `[ASSUMED — augments out of scope for the paperdoll window; confirm with the real fixture which contains augment rows]`

---

## Research Question 3 — last-listed-for-sale semantics (DATA-01)

**`pigparse_price.last_seen` IS the last-listed-for-sale date, already stored, satisfiable from the daily getall data. No `getdetails` feed needed. Surface it as the raw ISO-8601 string.**

### Evidence
- `pigparse_price.last_seen` is populated by the daily job from the getall response's `l` field: `enrich/pigparse.go:51` (`L string json:"l" // ISO 8601 last seen`), coerced at `coerceRow` (`pigparse.go:143-145`), carried through `jobs/pigparse.go:137` (`LastSeen: r.L`), upserted at `store/enrich.go:110/116/147` (`last_seen=excluded.last_seen`). `[VERIFIED]`
- Semantics: `l` is the timestamp PigParse last *saw the item listed in the EC tunnel* for that direction. After the daily job's D-9 WTS filter (`jobs/pigparse.go:99-107`, keep only `t=0`), the surviving row's `last_seen` is **last-seen-listed-WTS** — exactly "last-listed-for-sale." `[VERIFIED + CITED: pigparse.go:51 comment]`
- It is currently **NOT projected** by `InventoryJoin` (the SELECT at `readviews.go:150-153` reads `pp.direction, pp.a30, pp.t30` but not `pp.last_seen`). Note: `c.last_seen` (character freshness) IS selected and surfaced as `ViewRow.LastSynced` — **do not confuse the two.** `character.last_seen` = when the *watcher last uploaded*; `pigparse_price.last_seen` = when the *item was last listed for sale*. DATA-01 wants the latter. `[VERIFIED — landmine flagged below]`

### Recommendation
- Add `pp.last_seen` to the price-join projection in the new/extended store read; surface it as a new field (e.g. `LastListed string`) on the price detail / row struct. Blank string when absent (the `LEFT JOIN` yields NULL → `""`, matching the existing nullable-scan pattern `readviews.go:177-207`).
- **Do NOT use the `getdetails` / `pigdetails.go` feed for DATA-01.** That feed (`enrich/pigdetails.go`) is the per-auction EC-monitor cursor source (Phase 21) — its `t` is a per-auction timestamp and its `i` is an auction-instance id, not the catalog id (`pigdetails.go:43-48`). It is polled per-wanted-item on a 10-min cadence and is the wrong tool for a static "last listed" date across the whole catalog. getall's `last_seen` is the correct, already-stored, daily source. `[VERIFIED]`
- **Format**: surface the raw ISO-8601 string (parity with how `ViewRow.LastSynced` is the raw ISO string — `compute/view.go:13-15`, "freshness coloring is client-side"). Let the web format it.

---

## Research Question 4 — Existing slot constants + the canonical taxonomy (INV-05)

### The canonical equipment paperdoll slot set (from the real `/outputfile` Location vocabulary)
From the synthetic fixture (`sample-inventory.txt`) the equipment Location tokens are, verbatim:
`Charm, Head, Face, Neck, Shoulders, Arms, Back, Wrist1, Wrist2, Range, Hands, Primary, Secondary, Finger1, Finger2, Chest, Legs, Feet, Waist, Power, Ammo`. `[VERIFIED: fixture rows 20-40]`
General: `General1`..`General10`. Bank: `Bank1`..`Bank8`. `[VERIFIED]`

> Note: the fixture has NO `Ear1/Ear2` rows, though EQ has two ear slots. Real dumps will include `Ear1`/`Ear2` (or `Ear`/`Ear-Slot1` augment forms). The taxonomy must include Ear slots even though the synthetic fixture omits them. `[ASSUMED — standard EQ paperdoll has 2 ear slots; confirm in the real fixture]`

### Existing constants (and the mismatch the planner MUST bridge)
`enrich.WIKI_SLOT_TO_INV_SLOTS` (`enrich/eqconst.go:65-83`) maps **wiki prose slots → inv tokens**, but it uses **UPPERCASE** tokens and **plural→pair** forms that do NOT match the inventory `Location` strings:

| Wiki slot (gear-tier vocab) | `WIKI_SLOT_TO_INV_SLOTS` value | Actual inventory `Location` token(s) | Match? |
|---|---|---|---|
| Ears | `EAR1`, `EAR2` | `Ear1`, `Ear2` (real) | **Case mismatch** |
| Fingers | `FINGER1`, `FINGER2` | `Finger1`, `Finger2` | **Case mismatch** |
| Wrists | `WRIST1`, `WRIST2` | `Wrist1`, `Wrist2` | **Case mismatch** |
| Head | `HEAD` | `Head` | **Case mismatch** |
| Primary | `PRIMARY` | `Primary` | **Case mismatch** |

**This is the load-bearing slot-vocabulary landmine.** `gearcheck.go` gets away with it because its slot-pair match (`itemsInSlots`, `gearcheck.go:127-141`) compares `it.Location == slot` where `slot` is the uppercase token — meaning **the existing `gear_check` slot match would only fire if inventory Locations were uppercase.** Either (a) the live data is uppercased somewhere, or (b) `gear_check` slot-matching has a latent case bug that never surfaced because the match is one of three branches and "MISSING" is an acceptable-looking fallback. **The planner must verify the case of `inventory_item.location` in live data** (`sqlite3 squirebot.db "SELECT DISTINCT location FROM inventory_item LIMIT 40"`) before building the classifier. The fixture is `Title`-case (`General1`, `Head`, `Finger1`). `[VERIFIED mismatch in code; live-case UNVERIFIED — flag]`

### Recommended classification approach
Build a **new, inventory-Location-native** classifier in `compute` (do not retrofit `WIKI_SLOT_TO_INV_SLOTS`, which is wiki-vocab-keyed and case-divergent):
- `classifySlot(location string) → (category, canonicalSlot)` where category ∈ {`equipment`, `general`, `bank`}:
  - `^General\d+` (and `General\d+-Slot\d+`) → `general`
  - `^Bank\d+` / shared-bank tokens (and their `-Slot\d+`) → `bank`
  - everything else in the known equipment set → `equipment`, canonicalSlot = the token itself (`Head`, `Finger1`, …)
- Case-normalize the comparison (compare case-insensitively, emit a canonical Title-case key) so the classifier is robust to whichever case the live data uses. This **also** lets DATA-01/WISH-04 bridge the wiki gear-tier `slot` vocab to the inventory canonical slot: build a `WIKI_SLOT_TO_CANONICAL` map (`Ears→[Ear1,Ear2]`, `Fingers→[Finger1,Finger2]`, `Wrists→[Wrist1,Wrist2]`, `Primary→[Primary]`, …) in **inventory-Location case**, distinct from the legacy uppercase `WIKI_SLOT_TO_INV_SLOTS`. Phase 34 consumes this; Phase 29 can define it (or defer to 34 — recommend defining the canonical equipment slot list + classifier here, leaving the wiki→slot bridge map to whoever needs it first).

### The wiki gear-tier slot vocabulary (for the DATA-01/WISH-04 bridge)
Extracted from `testdata/wiki-velious-preraid-gear.json` `[VERIFIED]`:
`Arms, Back, Chest, Ears, Face, Feet, Fingers, Hands, Head, Instruments, Legs, Neck, Primary, Primary-1H, Primary-2H, Range, Secondary, Shoulders, Waist, Wrists`.
Note the gear-tier-only slots `Instruments`, `Primary-1H`, `Primary-2H` have **no direct inventory Location equivalent** (bard instruments map to multiple physical slots; 1H/2H both live in `Primary`). The bridge map must handle these (map `Primary-1H`/`Primary-2H`→`Primary`; decide `Instruments` handling — likely surface as suggestions without an equipped-slot match). Flag to the planner; full resolution belongs to Phase 34 but Phase 29 should at least document the gap so DATA-01's name-join doesn't silently drop these suggestions.

---

## Research Question 5 — Compute seam + schema footprint (RECOMMENDATION)

**Pure compute-on-read. NO migration. New code mirrors the `compute/view.go` ↔ `store/readviews.go` seam. Do NOT add HTTP endpoints in Phase 29.**

### Why no migration
- Everything DATA-01/02/INV-05 needs is already stored: `inventory_item.location/name/item_id/count/slots`, `pigparse_price.a30/direction/last_seen` (name-joinable), `character.plat/is_bank_toon` (`migrations/00001_init.sql:8-36,60` + `00003`/`00004` columns). `[VERIFIED]`
- Guild scale is tiny (<100 MB, ~50–150 writes/day per CONTEXT/PROJECT). A per-read parse of ≤~1500 rows/guild is microseconds.
- A materialized slot-category column would couple the parse into the hot write path (`ReplaceInventory`) and require a backfill migration — net cost, zero benefit. CONTEXT D-discretion: materialize "only if research surfaces a concrete reason" — none found.
- Current schema head is `00011` (`migrations/00011_wantlist_drop_reason_dedup.sql`) = "schema v11" in STATE/memory. Extend-only remains available if a future need arises, but Phase 29 should ship **zero migrations**. `[VERIFIED: 11 migration files]`

### The store seam (new read methods — `?` placeholders only, single tested SQL path)
`InventoryJoin` filters `item_id > 0` and orders Char→item→location (`readviews.go:135-167`), which **drops empty equipment slots and is sorted wrong for a paperdoll/nesting render**. INV-05 needs all rows in `row_ordinal` (file) order including empty slots and container shells. So:

1. **`InventoryForChar(ctx, charName/charID) → []InventoryRow`** (new in `readviews.go`): returns *all* `inventory_item` rows for one character (including `item_id=0` empty slots and `*-Slot*` children), LEFT-JOINed to `item_master` (id-keyed) + `pp_rep`/`pigparse_price` (name-keyed, **including `pp.last_seen`**), ordered by `row_ordinal`. This is the INV-05 surface (and the per-character window for Phase 31). Reuse the exact `pp_rep` CTE from `InventoryJoin`.
2. **`BankInventoryJoin(ctx) → []InventoryJoinRow`** — either reuse `InventoryJoin(ctx, bankOnly=true)` as-is for valuation (it already scopes `is_bank_toon=1` and prices by name), OR add `pp.last_seen` to its projection. **Recommend: extend the existing `InventoryJoin` projection to add `last_seen` once**, so both `view`/`bank` and DATA-01 share it (don't fork the join). Extend `InventoryJoinRow` with a `LastListed string` field (zero-value safe; existing consumers ignore it). `[VERIFIED seam: readviews.go:46-61, 135-213]`
3. **`ListBankToons` is already perfect for D-04** (`store/coin.go:46-69`) — returns live bank toons + `*int64` plat. No new read needed; sum `Plat` (nil → skip/0).

### The compute seam (new pure transforms — `compute/inventory.go`)
1. **`StructuredInventory(rows []store.InventoryRow) → CharacterInventory`** (pure): classify each row (equipment/general/bank + canonical slot via the new classifier), build the parent→children nesting tree (`<ParentSlot>-Slot<N>`), preserve `count`, attach name-joined price + `LastListed`. Pure function → directly unit-testable (mirrors `buildViewRows` purity, `view.go:59-82`).
2. **`BankValuation(rows []store.InventoryJoinRow) → Valuation`** (pure): for each bank row, `pickPrice(row.Prices) × count`, sum; count rows where `pickPrice == nil` as `UnpricedCount` (D-03 "+N items unpriced"). Per-bank (group by Char) + guild-wide totals. Reuse `pickPrice` verbatim (`view.go:117`). `[VERIFIED reuse target]`
3. **`TotalPlatinum(banks []store.BankToon) → int64`** (pure): `SUM(plat)` over live bank toons, nil-safe (D-04 — literal plat only; gp/sp/cp NOT rolled in). `[VERIFIED: coin.go BankToon.Plat *int64]`

### New JSON contract structs (`compute/types.go`, extend-only — never rename existing tags)
Add new structs (e.g. `InventorySlot`, `CharacterInventory`, `BankValuation`) with snake_case tags. Do **not** rename `ViewRow`/`BankView`/`CoinTotals` (the package doc at `types.go:13-57` declares the existing tags a FIXED cross-plan contract). Append only.

### HTTP endpoints — defer
Phase 29 is "no new user surface" (CONTEXT). The compute functions + store reads are the deliverable; the *tab-shaped* endpoints (per-character window for INV-01, item-centric list for ITEM-01, bank totals for BANK-02) are owned by Phases 31/32/33 which know their exact payload shapes. **Recommend: do NOT add `readapi` handlers in Phase 29** — adding them now risks shipping a contract the consuming phase then has to reshape. The ONE exception: if a thin read endpoint is the cleanest way to *integration-prove* an aggregate end-to-end, add a single internal/debug endpoint, but the unit tests (below) are sufficient for the success criteria. (Route-registration pattern, when the consuming phases need it: `cmd/squirebot-server/main.go:287-290`, `mux.Handle("GET /api/v1/...", webauth.RequireSession(db, readapi.NewX(st)))` — note read endpoints are login-gated via `RequireSession` since P15, NOT public. `[VERIFIED: main.go:287-290]`)

---

## Architecture Patterns

### System Architecture Diagram (Phase 29 data flow)

```
                         (already stored — watcher untouched)
  inventory_item ───┐
  (location/slots/  │
   name/item_id/    │     ┌─────────────── store (SQL, ?-placeholders only) ──────────────┐
   count/row_ord)   │     │                                                                │
                    ├────►│ InventoryForChar(char)   ── all rows, row_ordinal order,       │
  item_master ──────┤     │   (NEW)                     +empty slots +container children,  │
  (id-keyed wiki)   │     │                             id-join item_master,               │
                    │     │                             NAME-join pigparse(+last_seen)      │
  pigparse_price ───┤     │ InventoryJoin(bankOnly)   ── existing name-join, +last_seen     │
  (a30/direction/   │     │   (EXTEND projection)                                           │
   last_seen,       │     │ ListBankToons()           ── existing (coin.go)                 │
   name-joinable)   │     └───────────────┬─────────────────────┬──────────────────────────┘
                    │                      │                     │
  character ────────┘                      ▼                     ▼
  (plat,is_bank_toon,            ┌── compute (pure, no SQL) ─────────────────────────┐
   last_seen)                    │ StructuredInventory(rows) ─► CharacterInventory   │  INV-05
                                 │   classifySlot + nesting tree + name-price        │
                                 │ BankValuation(bankRows)  ─► per-bank+guild totals │  DATA-02
                                 │   Σ pickPrice×count, +N-unpriced                   │  (price = DATA-01)
                                 │ TotalPlatinum(bankToons) ─► Σ plat (nil-safe)      │  DATA-02
                                 └───────────────────────────────────────────────────┘
                                                  │
                                                  ▼
                                   (consumed by Phases 31/32/33/34 web tabs —
                                    HTTP endpoints deferred to those phases)
```

### Recommended package structure (additive)
```
internal/backendsrv/
├── compute/
│   ├── inventory.go        # NEW: StructuredInventory, BankValuation, TotalPlatinum, classifySlot
│   ├── inventory_test.go   # NEW: parity tests over real-name fixture
│   ├── slotconst.go        # NEW (or fold into inventory.go): canonical equipment slot set + classifier
│   ├── types.go            # EXTEND: + InventorySlot, CharacterInventory, BankValuation (append-only)
│   ├── view.go             # FIX stale comments (lines 84-95)
│   └── fixtures_test.go    # EXTEND: + a seedInv variant that sets count/slots; nested-row seeds
├── store/
│   ├── readviews.go        # EXTEND: + InventoryForChar; + pp.last_seen on InventoryJoin projection
│   └── readviews_test.go   # EXTEND: new-method coverage
└── (NO new migration)
```

### Anti-Patterns to Avoid
- **Re-introducing the item_id price join** (the exact bug 0a169f3 fixed). Always join price by `lower(trim(name))`.
- **Using `WIKI_SLOT_TO_INV_SLOTS` (uppercase) as the inventory classifier.** It is wiki-vocab-keyed and case-mismatched to `inventory_item.location`. Build a Location-native classifier.
- **Confusing `character.last_seen` (upload freshness) with `pigparse_price.last_seen` (last-listed).** DATA-01 needs the latter; `ViewRow.LastSynced` is the former.
- **Filtering `item_id > 0` for the slot model.** INV-05 needs empty slots + container shells; `InventoryForChar` must NOT filter them.
- **Materializing a parsed-slot column.** Compute-on-read is the locked default; no concrete reason to materialize.
- **Adding HTTP endpoints with guessed payload shapes** ahead of the consuming phases.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Price pick (WTS→WTB a30 fallback) | A new price selector | `compute.pickPrice` (`view.go:117`) verbatim | D-03 mandates it; already TEXT-direction-correct (Pitfall 6) |
| Inventory→price name bridge | A new join | The `pp_rep` CTE from `InventoryJoin` (`readviews.go:144-159`) | Already solves the namespace + fan-out problem |
| Bank-toon + coin read | A new query | `store.ListBankToons` (`coin.go:46`) | Already scopes `is_bank_toon=1 AND is_removed=0` and returns `*int64` plat |
| Name normalization | A custom normalizer | `lower(trim(name))` (project-wide) | Must match the existing join key exactly or live `view` prices break |
| Inventory file parsing | New file parser | (nothing — watcher uploads raw; parse is the *Location string* parse, server-side) | `internal/parse/inventory.go` already produces the 5 columns; Phase 29 parses `Location`, not the file |

**Key insight:** Phase 29 is almost entirely *recombination* of shipped seams. The only genuinely new logic is the `Location`-string classifier + nesting tree (INV-05). Everything else (name-join, price-pick, bank-toon read) already exists and must be reused, not rebuilt.

---

## Common Pitfalls

### Pitfall 1: Trusting the stale "item_id is the PK / one row per item" comments
**What goes wrong:** A dev reads `view.go:84-95` / `types.go:62-63` (or CLAUDE.md "item_id is the stable join key") and re-introduces an item_id price join, dropping ~91% of prices.
**Why:** The comments predate the 0a169f3 name-join fix and were never updated.
**How to avoid:** Join price by name (`pp_rep` CTE). Fix the comments. Treat the running SQL, not the comments, as truth.
**Warning sign:** Any new SQL with `pigparse_price.item_id = inventory_item.item_id`.

### Pitfall 2: `pigparse_price.last_seen` vs `character.last_seen`
**What goes wrong:** DATA-01 surfaces upload-freshness instead of last-listed-for-sale.
**Why:** Both columns are named `last_seen`; `InventoryJoin` already surfaces `c.last_seen` as `LastSynced`.
**How to avoid:** Select `pp.last_seen` explicitly into a distinct `LastListed` field; never alias it to `LastSynced`.
**Warning sign:** A single "last seen" field doing double duty.

### Pitfall 3: Double-counting / under-counting bag contents (D-02)
**What goes wrong:** Either the container *and* its children are summed as if independent (fine — both count per D-02) but the bag's own price is *also* missed, OR children are dropped from valuation entirely.
**Why:** Easy to treat `*-Slot*` rows as non-items.
**How to avoid:** D-02 is explicit — **every** held item counts toward quantity + valuation, including bag contents AND the bag itself. The valuation is a flat sum over ALL bank rows (`InventoryJoin(bankOnly)` already returns child rows as their own rows, since they're individual `inventory_item` rows). The nesting tree is for *display* (INV-03 drill-down), NOT for valuation scoping. Do not deduplicate.
**Warning sign:** Valuation logic that walks the nesting tree instead of summing the flat row list.

### Pitfall 4: NULL `wiki_gear_tier.item_id` (DATA-01 gear-tier join)
**What goes wrong:** A join keyed on `wiki_gear_tier.item_id` returns zero rows (it's always NULL) — silently empty suggestions.
**Why:** The wiki parser exposes no IDs (`wikigear.go:30-40`); the table uses full-replace *because* UNIQUE-on-NULL is broken (`store/enrich.go:84-92,272-277`).
**How to avoid:** Join gear-tier → price by `lower(trim(item_name))` only. Never by `item_id`.
**Warning sign:** Any reference to `wiki_gear_tier.item_id` in a join.

### Pitfall 5: Slot-token case mismatch
**What goes wrong:** The classifier compares `inventory_item.location` against uppercase constants (`HEAD`, `FINGER1`) and never matches; everything classifies as "general"/unknown.
**Why:** `WIKI_SLOT_TO_INV_SLOTS` is uppercase; the fixture Locations are Title-case.
**How to avoid:** Case-insensitive comparison in the classifier; verify live data case (`SELECT DISTINCT location ...`) before finalizing the canonical-slot map.
**Warning sign:** Equipment items showing up unclassified.

### Pitfall 6: `direction` is TEXT ("0"/"1"/"2"), not numeric
**What goes wrong:** A price filter compares `direction == 0` (int) and never matches the stored string `"0"`.
**Why:** `pigparse_price.direction` is TEXT; the daily job stores `strconv.Itoa(t)` (`jobs/pigparse.go:128`).
**How to avoid:** Reuse `pickPrice`, which already compares the stringified consts `directionWTS="0"`/`directionWTB="1"` (`view.go:38-41,117`). Don't write a fresh direction comparison.

---

## Code Examples

### The name-join CTE to reuse (price + last-listed)
```sql
-- Source: store/readviews.go:144-159 (verbatim pattern; EXTEND to add pp.last_seen)
WITH pp_rep AS (
  SELECT lower(trim(name)) AS norm_name, MIN(item_id) AS rep_item_id
  FROM pigparse_price
  WHERE name IS NOT NULL AND trim(name) <> ''
  GROUP BY lower(trim(name))
)
SELECT ii.location, ii.name, ii.item_id, ii.count, ii.slots, ii.row_ordinal,
       im.wiki_url, im.wiki_summary, im.is_quest_item,
       pp.direction, pp.a30, pp.t30, pp.last_seen,        -- last_seen = DATA-01 last-listed (NEW in projection)
       c.last_seen AS char_last_seen                       -- distinct: upload freshness
FROM inventory_item ii
JOIN character c            ON c.id = ii.character_id
LEFT JOIN item_master im     ON im.item_id = ii.item_id        -- id-keyed (correct)
LEFT JOIN pp_rep             ON pp_rep.norm_name = lower(trim(ii.name))
LEFT JOIN pigparse_price pp  ON pp.item_id = pp_rep.rep_item_id -- price via NAME
WHERE c.is_removed = 0 AND c.name = ?                           -- (InventoryForChar: per-char, NO item_id>0 filter)
ORDER BY ii.row_ordinal
```

### The price pick to reuse (D-03 valuation basis)
```go
// Source: compute/view.go:117-138 — reuse verbatim for BankValuation
// pickPrice(row.Prices) returns *float64 (nil when no WTS/WTB a30>0).
// Per-row value = *pickPrice × count; nil → unpriced (count toward "+N items unpriced", value 0).
```

### Test seed pattern (extend for count/slots/nesting)
```go
// Source: compute/fixtures_test.go:55-64 — seedInv hardcodes count=1, slots=0.
// EXTEND with a variant that sets count + slots + a *-SlotN location so nesting/valuation are testable:
//   seedInvFull(t, db, charID, "General4", "Large Bag", 1038, 1 /*count*/, 10 /*slots*/, ordinal)
//   seedInvFull(t, db, charID, "General4-Slot1", "Diamond", 1071, 5 /*count*/, 0, ordinal+1)
// Name-join requires pigparse_price.name == inventory name (seedPigparse note, fixtures_test.go:84-86).
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| inventory→price join by `pigparse_price.item_id = inventory_item.item_id` | join by `lower(trim(name))` via `pp_rep` CTE | 2026-06-06 (`0a169f3`) | ~91% → ~78% of held rows now priced; **DATA-01 builds on this, not against it** |
| `view.go`/`types.go` comments say "PK ⇒ one row per item" | one-row guarantee now from CTE `GROUP BY norm_name`/`MIN(item_id)` | 2026-06-06 | Comments are stale; fix in Phase 29 |

**Deprecated/outdated:**
- CLAUDE.md "item_id is the stable join key" — true for `item_master` (watcher's EQ namespace), **false for `pigparse_price`** (different catalog namespace). Do not generalize it to the price join.
- The 48-day-old memory `project_inventory_file_format` claims "ID is the canonical EQ item ID and is the right primary key for joining against wiki gear-tier data" — **wrong for the price/gear-tier join** (gear-tier has no id; catalog id ≠ inventory id). Name-join only.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Bag-content Location format is `<ParentSlot>-Slot<N>`, 1-indexed sub-slots | RQ2 | Parser builds the wrong nesting tree; INV-03 drill-down breaks. **Mitigated by the required real-name nested fixture** (success-criterion #4) — confirm before coding the index math. |
| A2 | Bags nest only one level deep (no bags-in-bags in classic EQ) | RQ2 | A grandchild row would be mis-nested. Defensive flatten + log recommended. |
| A3 | Augment rows (`Head-Slot1`) are out of scope for the INV-05 paperdoll window and can be flattened/ignored | RQ2/RQ4 | If the window must show augments, the nesting model needs an augment branch. One-line planner decision; confirm vs real fixture. |
| A4 | Real dumps include `Ear1`/`Ear2` even though the synthetic fixture omits them | RQ4 | Missing ear slots in the paperdoll. Add to the canonical set regardless. |
| A5 | `inventory_item.location` live-data case matches the fixture (Title-case) | RQ4 | If live data is uppercase, the classifier's canonical keys differ. **Verify with `SELECT DISTINCT location`.** Mitigated by case-insensitive compare. |
| A6 | The "~58/713 / ~559" name-vs-id coverage ratios | RQ1 | Illustrative only; the *direction* (name >> id) is what matters and is HIGH confidence. |
| A7 | `pigparse_price.last_seen` (after WTS filter) is "last listed for sale WTS" | RQ3 | If `l` is actually last-refresh, DATA-01 surfaces the wrong date. Parser comment says "last seen" listed; confirm on-box with `SELECT name,last_seen FROM pigparse_price LIMIT 5`. |

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | All Phase 29 code/tests | ✓ (assumed — project builds) | 1.24 (CLAUDE.md) | — |
| SQLite (via existing driver) | store reads | ✓ (in-repo, `store.NewTestDB`) | — | — |
| A real-name `<Char>-Inventory.txt` WITH nested bag contents | success-criterion #4 fixture | ✗ **NOT in repo** | — | **No fallback — the user must capture a real `/outputfile inventory` from a P99 char with bags+bank, or the planner must hand-author a realistic nested fixture from the confirmed format.** |
| Live `squirebot.db` (Hetzner) for the case/last_seen spot-checks | RQ3/RQ4 verification | ✓ via SSH (memory `v2-backend-live-and-ops-access`) | — | Fixture-only verification (lower confidence on A5/A7) |

**Missing dependency with no fallback:** the **real-name nested-bag inventory fixture**. This is the gating input for INV-05's nesting tests. The watcher already produces this file in the wild; the user (a P99 player) can run `/outputfile inventory` on a char with bags and bank bags and drop the `.txt` into `internal/backendsrv/compute/testdata/` (real-name per CLAUDE.md convention, e.g. `Slampeach-Inventory.txt`). **Flag to the planner: the phase cannot prove nesting without this; the format from RQ2 lets a synthetic one be authored if a real capture isn't available, but a real capture is strongly preferred.**

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (table + golden-fixture style) |
| Config file | none — `go test ./...` |
| Quick run command | `go test ./internal/backendsrv/compute/... ./internal/backendsrv/store/...` |
| Full suite command | `go test ./...` |

### Phase Requirements → Test Map
| Req | Behavior | Test Type | Command | File Exists? |
|-----|----------|-----------|---------|-------------|
| INV-05 | classify equipment/general/bank + canonical slot | unit | `go test ./internal/backendsrv/compute/ -run TestStructuredInventory_Classify` | ❌ Wave 0 |
| INV-05 | nest `<Parent>-Slot<N>` children under container; preserve count | unit | `... -run TestStructuredInventory_Nesting` | ❌ Wave 0 |
| INV-05 | container shell + empty slots are NOT dropped | unit | `... -run TestInventoryForChar_KeepsEmptyAndContainers` (store) | ❌ Wave 0 |
| DATA-01 | price + last-listed join hits by name; gear-tier (NULL id) name-join hit + miss | unit | `... -run TestNameJoin_HitMiss` | ❌ Wave 0 |
| DATA-01 | `pp.last_seen` surfaced as LastListed, distinct from char freshness | unit | `... -run TestLastListed_NotCharFreshness` | ❌ Wave 0 |
| DATA-02 | bank valuation = Σ pickPrice×count; "+N unpriced" annotation | unit | `... -run TestBankValuation_SumAndUnpriced` | ❌ Wave 0 |
| DATA-02 | bag contents AND the bag itself both count in valuation | unit | `... -run TestBankValuation_CountsBagContents` | ❌ Wave 0 |
| DATA-02 | total platinum = Σ plat over live bank toons; gp/sp/cp excluded; nil-safe | unit | `... -run TestTotalPlatinum_LiteralPlatOnly` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/backendsrv/compute/... ./internal/backendsrv/store/...`
- **Per wave merge:** `go test ./...`
- **Phase gate:** `go test ./...` green before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `internal/backendsrv/compute/testdata/<Real>-Inventory.txt` — real-name fixture WITH nested bag contents (`<ParentSlot>-Slot<N>`) + a bank with bagged items + at least one stacked item + one unpriced item (covers INV-05 nesting, DATA-01 hit/miss, DATA-02 sum + "+N unpriced"). **Blocking input — see Environment Availability.**
- [ ] `internal/backendsrv/compute/inventory_test.go` — the INV-05/DATA-02 parity tests.
- [ ] `internal/backendsrv/store/readviews_test.go` — extend for `InventoryForChar` (keeps empty/container rows) + `last_seen` projection.
- [ ] `internal/backendsrv/compute/fixtures_test.go` — extend `seedInv` to a `seedInvFull(count, slots, location)` variant + a real-name-fixture loader helper.

---

## Files to CREATE and MODIFY

### CREATE
| File | Contents (recommended signatures) |
|------|-----------------------------------|
| `internal/backendsrv/compute/inventory.go` | `func StructuredInventory(rows []store.InventoryRow) CharacterInventory` (pure); `func BankValuation(rows []store.InventoryJoinRow) Valuation` (pure, reuses `pickPrice`); `func TotalPlatinum(banks []store.BankToon) int64` (pure, nil-safe); `func classifySlot(location string) (category SlotCategory, canonical string)` |
| `internal/backendsrv/compute/inventory_test.go` | parity tests over the real-name fixture per the test map |
| `internal/backendsrv/compute/testdata/<Real>-Inventory.txt` | real-name nested-bag fixture (blocking input) |
| (optional) `internal/backendsrv/compute/slotconst.go` | canonical equipment slot set + `WIKI_SLOT_TO_CANONICAL` bridge map (inventory-Location case) — or fold into `inventory.go` |

### MODIFY
| File | Change |
|------|--------|
| `internal/backendsrv/store/readviews.go` | ADD `func (s *Store) InventoryForChar(ctx, char string) ([]InventoryRow, error)` (all rows, `row_ordinal` order, NO `item_id>0` filter, name-join price+`last_seen`); ADD `pp.last_seen` to `InventoryJoin`'s SELECT projection + `LastListed` field on `InventoryJoinRow`; ADD a new `InventoryRow` struct (location/name/item_id/count/slots/row_ordinal + joined enrichment + price + last_listed) |
| `internal/backendsrv/store/readviews_test.go` | coverage for the new read method + `last_seen` projection |
| `internal/backendsrv/compute/types.go` | APPEND new structs (`InventorySlot`, `CharacterInventory`, `BankValuation`, `Valuation`) with snake_case tags — **never rename** existing tags |
| `internal/backendsrv/compute/view.go` | FIX stale comments at lines 84-95 ("item_id is the PK / one row per item") to reflect the name-join reality |
| `internal/backendsrv/compute/types.go` | FIX the stale `PriceDetail` comment at lines 62-63 |
| `internal/backendsrv/compute/fixtures_test.go` | EXTEND `seedInv` → add count/slots/nested-location seed helper + real-fixture loader |

**No new migration.** **No new `readapi` handler** (deferred to Phases 31–34).

---

## Project Constraints (from CLAUDE.md)
- **Watcher UNTOUCHED** — all parse work is server-side (`internal/backendsrv`). No edits to `internal/parse/`, `internal/app/`, `internal/watch/`, `internal/sheet/`. `[HARD]`
- **Extend-only schema** — if any migration were added it must be additive + version-stamped + idempotent with `_meta`/schema-version bump LAST. (Research recommends **zero** migrations.) `[HARD]`
- **Single tested SQL path / `*Tx` rule** — `store` authors all SQL with `?` placeholders only; `compute` authors zero SQL. `[HARD]`
- **Never string-concat untrusted item names into SQL** — names are bound through `?`. `[HARD]`
- **Structured logging (slog), counts/ids/err only, never raw content (V7).** `[HARD]`
- **GSD workflow** — file edits go through a GSD command (this is research only — no edits made). `[HARD]`
- **Consolidated-views lock RELAXED (2026-06-17)** — per-character master-detail allowed; Phase 29 produces per-character inventory data, which is now sanctioned. `[VERIFIED: CLAUDE.md Architecture]`

---

## Open Questions

1. **Live `inventory_item.location` case** (Title vs upper) — verify `SELECT DISTINCT location FROM inventory_item` on the Hetzner box before finalizing the classifier's canonical keys. Mitigated by case-insensitive comparison, but the canonical *output* key should match what the web expects. (A5)
2. **Real nested-bag fixture availability** — does the user have / can they capture a `/outputfile inventory` from a P99 char with bagged items in both inventory and bank? If not, author one from the confirmed RQ2 format. (Blocking — Environment Availability.)
3. **Augment handling** — surface `*-Slot1` augment rows on equipment, or flatten/ignore for the INV-05 window? One-line planner decision (recommend ignore for the paperdoll; confirm vs the real fixture which will contain augment rows). (A3)
4. **Gear-tier-only slots** (`Instruments`, `Primary-1H`, `Primary-2H`) have no 1:1 inventory Location — Phase 29 should document the bridge gap so DATA-01's name-join doesn't silently drop these suggestions; full resolution is Phase 34's. (RQ4)

---

## Sources

### Primary (HIGH confidence — read directly this session)
- `internal/backendsrv/store/readviews.go:33-213` — the live name-join (`pp_rep` CTE), the namespace rationale, `InventoryJoin` projection/filter/order
- `internal/backendsrv/compute/view.go:38-138` — `pickPrice`, `buildViewRows`, TEXT-direction consts, the stale comments
- `internal/backendsrv/compute/types.go:13-119` — the FIXED snake_case JSON contract; the stale `PriceDetail` comment
- `internal/backendsrv/enrich/pigparse.go:42-55,143` + `enrich/jobs/pigparse.go:99-140` — `l`→`last_seen`, WTS filter, direction encoding
- `internal/backendsrv/store/enrich.go:84-92,109-155,272-300` — `pigparse_price` upsert (item_id PK), `wiki_gear_tier` NULL item_id + full-replace
- `internal/backendsrv/store/coin.go:33-69` — `ListBankToons`, `BankToon.Plat *int64` (D-04)
- `internal/backendsrv/store/replace.go:90-123` — raw Location/Slots storage (no parse)
- `internal/backendsrv/enrich/eqconst.go:65-83` + `compute/gearcheck.go:127-157` — `WIKI_SLOT_TO_INV_SLOTS` (uppercase) + the case-mismatch
- `internal/backendsrv/enrich/wikigear.go:30-105` — gear-tier parse, always-NULL item_id, slot vocabulary
- `internal/backendsrv/migrations/00001_init.sql` + `00003` + dir listing (head `00011`) — schema
- `internal/parse/inventory.go` + `inventory_test.go` + `testdata/sample-inventory.txt` — watcher passthrough; flat synthetic fixture (no nesting)
- `internal/backendsrv/compute/fixtures_test.go` + `bank_test.go` — established seed/test pattern
- `cmd/squirebot-server/main.go:286-290` — read-endpoint route + `RequireSession` gating pattern
- `git log -S "pp_rep"` → commit `0a169f3` (2026-06-06) — the name-join fix

### Secondary (MEDIUM — community-corroborated, format)
- WebSearch (P1999 inventory parsers / Fanra wiki / EQ Traders / eqemu docs) — `<ParentSlot>-Slot<N>` bag-content format, 1-indexed sub-slots, General1-10/Bank1-8, augment `-Slot1` suffix

### Tertiary (LOW — flagged for validation)
- Exact sub-slot indexing + bag-in-bank nesting depth — confirm against the real-name fixture (A1/A2)
- Project memory `pigparse-vs-ingame-item-id-namespaces`, `project_inventory_file_format` (48-day-old) — directionally confirmed by code; specifics re-verified above

---

## Metadata

**Confidence breakdown:**
- item_id-vs-name join verdict: **HIGH** — read the running SQL + the git commit that made the change
- Schema footprint (no migration) / compute seam: **HIGH** — derived from existing seams + locked CONTEXT discretion
- DATA-01 last-listed source: **HIGH** — traced `l`→`last_seen` through parser→job→store
- Slot taxonomy / classifier: **HIGH** on the slot SET; **MEDIUM** on live-data case (A5)
- Container nesting format: **MEDIUM** — community-corroborated, no real fixture in repo yet (the phase's job to add)

**Research date:** 2026-06-17
**Valid until:** ~2026-07-17 for the code findings (stable repo); the container-nesting format is permanent EQ behavior. Re-verify A5/A7 on-box before coding.
