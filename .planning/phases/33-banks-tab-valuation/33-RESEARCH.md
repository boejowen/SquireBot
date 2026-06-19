# Phase 33: Banks Tab + Valuation - Research

**Researched:** 2026-06-18
**Domain:** Go backend (surface a P29 valuation that no route exposes yet + a bank-scoped roster) + SvelteKit master-detail web tab reusing the P31 `InventoryWindow` and the P32 item-rollup/holders pattern.
**Confidence:** HIGH — every input is already-shipped, code-read internal. The ONE genuinely-new decision (a backend scope-reconciliation for `IsBankToon || IsGuildBot`) is fully characterized below; the rest is composition of P29/P31/P32 surfaces.

## Summary

Phase 33 is a **surfacing + composition** phase with one sharp backend wrinkle. Three requirements over already-built data:

- **BANK-01** (banks-only list + window) is the `/characters` master-detail page minus two of its three bands, reusing `RosterFor` (filtered to bank/bot rows, plain A-Z) for the list and the live `GET /api/v1/inventory/{char}` + the generic prop-driven `InventoryWindow` for the detail. `InventoryWindow`'s own header comment already says *"Phase 33 reuses it per bank toon"* — zero component change. `[VERIFIED: InventoryWindow.svelte:1-21, characters/+page.svelte, inventory.go]`
- **BANK-02** (guild-wide value + platinum) is `compute.BankValuationFor(ctx, store) → BankValuation` — a function shipped in P29 that **is never wired to an HTTP route** (grep: referenced only in `types.go`/`inventory.go`/its test + planning docs). The struct already carries `PerBank map[string]Valuation`, `GuildTotal Valuation`, and `TotalPlatinum int64` — so BOTH the D-02 guild summary AND the D-04 per-bank item value are already computed. `[VERIFIED: compute/inventory.go:331-393, types.go:191-205, grep BankValuationFor]`
- **BANK-03** (per-item bank search) is the P32 `/api/v1/items` rollup filtered client-side to holders where `is_bank === true` (every `ItemHolder` already carries `is_bank = is_bank_toon || is_guild_bot`), dropping items with zero bank holders. No new backend needed. `[VERIFIED: compute/itemrollup.go:87-100, api.ts:222-251]`

**The one wrinkle (Claude's-discretion → resolved below):** D-01's bank set is **`IsBankToon || IsGuildBot`**, but the shipped `BankValuationFor` scopes ONLY to `is_bank_toon=1` (via `InventoryJoin(ctx, true)` and `ListBankToons`, both `is_bank_toon=1 AND is_removed=0`). **Guild bots are excluded from both the value and the platinum.** So `BankValuationFor` as-shipped does NOT satisfy D-01/BANK-02 if any guild *bot* holds goods or coin. The planner must reconcile this (Pattern 1 + Pitfall 1 below). Two clean paths; the recommended one extends the bank scope in the store/compute (extend-only, no schema change). Per-bank **platinum** (D-04) is a separate small gap: `Valuation` carries `TotalValue`+`UnpricedCount` but NOT plat — per-bank plat lives on `BankToon.Plat`, so the per-bank header must also carry the toon's `plat`.

**Primary recommendation:** Add ONE new session-gated read route `GET /api/v1/banks` served by a new `readapi.BanksHandler` over a new `compute.Banks(ctx, store) → BanksView` that returns the bank/bot roster rows (name + item-count + per-bank value + per-bank plat + unpriced count) PLUS the guild summary (total value + total plat + total unpriced), with the bank set widened to `IsBankToon || IsGuildBot`. Reuse `GET /api/v1/items` unchanged for BANK-03 (client-filter to `is_bank` holders). Build `web/src/routes/banks/+page.svelte` as a master-detail mirror of `/inventory` (list + summary header + per-item search) whose detail is a per-bank value/plat header + the reused `InventoryWindow` (mirroring `/characters`'s window column). Add `fetchBanks()` to `api.ts` and a pure node-tested `web/src/lib/banks.ts`. **No new migration.**

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** List bank toons AND guild bots (`IsBankToon || IsGuildBot`), **A-Z** (plain alphabetical — NOT viewer-first; banks are nobody's assigned chars). Both designations hold shared goods and BOTH count toward the BANK-02 totals. Rejected: bank-toons-only.
- **D-02:** ONE guild-wide summary header (total PigParse item value across all bank+bot holdings AND total platinum across the guild banks); bank list rows stay clean (name + item count). Rejected: per-row value/platinum subtotals.
- **D-03:** Item-centric search, Phase-32 style — type an item name → see which bank(s) hold it (qty/slot) → clicking a holder opens that bank's inventory window. Rejected: a bank-list filter.
- **D-04:** Per-bank value/platinum header above the reused P31 `InventoryWindow` when a bank is selected (THAT bank's own item value + platinum). Guild-wide totals stay in the top summary (D-02).

### Claude's Discretion (researcher/planner owns these — RESOLVED below)
- **Backend shape** — bank-scoped endpoint vs compose existing reads. Resolved: ONE new `GET /api/v1/banks` (roster+per-bank+summary) + reuse `GET /api/v1/items` for search. Compute-on-read; extend-only; `?`-bound; session-gated (`RequireSession`, NOT officer); never string-concat names into SQL.
- **Item-search wiring** — reuse `/api/v1/items` (client-filter `is_bank` holders) vs a new bank-scoped endpoint. **Resolved: reuse `/api/v1/items`** (Pattern 3).
- **Per-bank value/platinum sourcing (D-04)** — the per-bank slice. **Resolved: from the new `/api/v1/banks` per-bank rows** (each carries value + plat + unpriced), so the detail header reads its slice off the already-loaded list — no second fetch (Pattern 2).
- **Likely no new migration** — bank/bot designation + bank-coin shipped v2.3; icon/statsblock in P31's 00012/00013 (schema v13). **CONFIRMED: no new migration** (Pitfall 4).
- Exact list/summary/detail layout, mobile reflow, row density → deferred to `33-UI-SPEC.md` (`/gsd-ui-phase 33`, UI hint = yes).

### Deferred Ideas (OUT OF SCOPE)
- **Sort/filter controls on the bank list** (by value, by item count) beyond A-Z + item search — future polish.
- **Per-item value column inside the bank window** — the window is reused as-is from P31; per-item price already lives in its examine.
- **Per-character/per-slot Wishlist** — Phase 34 (WISH-01..07).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| BANK-01 | Lists only guild-bank characters (same ordering style as Characters → reduced to its banks-only case = plain A-Z, D-01); each opens its inventory window. | New `compute.Banks` returns the `IsBankToon \|\| IsGuildBot` roster rows (A-Z) with a per-bank item count; the list mirrors `/characters`'s bespoke list (no bands — single A-Z run). Selection drives the live `fetchInventory(bankName)` → `InventoryWindow` (the generic prop-driven P31 component, built for this reuse) in the detail column. |
| BANK-02 | Total PigParse value of all items held by bank characters + total platinum across the guild banks. | `compute.BankValuationFor` already sums value + plat, BUT scopes to `is_bank_toon=1` ONLY — must widen to `IsBankToon \|\| IsGuildBot` (Pitfall 1). New `compute.Banks` returns `GuildTotal.TotalValue` + `TotalPlatinum` (+ unpriced count for the "+N unpriced" annotation, D-03/P29). The D-02 summary header renders `~{value} pp · {plat} plat`. |
| BANK-03 | Per-item name search across the items held by the guild banks. | Reuse `GET /api/v1/items` (P32). Each `ItemRollup.holders[]` already carries `is_bank`. A pure `banks.ts` filter keeps only items with ≥1 `is_bank` holder, and within each item's holders keeps only the bank holders. Holder rows deep-link `/banks?c=<name>` (the same `?c=` selection seam, now in-tab) so clicking opens that bank's window. |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Bank/bot roster + per-bank value + per-bank plat + guild totals | API / Backend (`compute.Banks`) | Database (`InventoryJoin` bank-scoped + `ListBankToons` widened reads) | Aggregation over the full guild dataset is a server concern; the client receives a bounded, pre-rolled payload. Compute-on-read (no materialized table). |
| Bank-set definition (`IsBankToon \|\| IsGuildBot`) | API/Backend + Database (the WHERE clause) | — | The bank set is a data-truth predicate; it must be applied in the store WHERE (so bots' rows + coin are actually summed), NOT post-filtered in the client (the client never sees non-bank chars in this view). |
| Name-keyed price / value-per-item | Database (`pp_rep` name-join CTE in `InventoryJoin`) | API (`pickPrice` selection in `buildBankValuation`) | Cross-namespace name join already solved (commit 0a169f3); compute reuses it via `pricesFromJoin`+`pickPrice`. |
| Per-item bank search (which bank holds item X) | Frontend (pure `banks.ts` filter over the P32 rollup) | API (`/api/v1/items` already carries `is_bank` per holder) | The bank slice is a presentation filter over a guild-wide rollup; no new server read. NOT access control (every member sees every bank). |
| Master-detail selection + per-bank window | Browser / Client (SvelteKit `?c=` + `InventoryWindow`) | API (`/inventory/{char}` per bank, unchanged) | Selection is a client render (relaxed consolidated-views rule — one reusable window, not N routes). |

## Standard Stack

This phase adds **no new dependency** (backend or web). It composes already-shipped internals.

### Core (all in-tree — verified by code read)
| Library / Module | Version | Purpose | Why Standard |
|------------------|---------|---------|--------------|
| Go std `net/http` `ServeMux` | Go 1.24 | New `GET /api/v1/banks` route + `RequireSession` wrap | The shipped route idiom (`main.go:286-370`); every read route uses it. |
| `internal/backendsrv/compute` | in-repo | New `Banks(...)` pure-grouping function; reuses `BankValuationFor`/`buildBankValuation`/`pickPrice`/`pricesFromJoin` | Compute-on-read; authors ZERO SQL. |
| `internal/backendsrv/store` | in-repo | A widened bank-scoped `InventoryJoin` + a widened bank-toon coin read (the `IsBankToon \|\| IsGuildBot` set) | The tested SQL seam; extend-only. |
| `internal/backendsrv/readapi` | in-repo | New `BanksHandler` (the `characters.go`/`items.go` twin — no viewer id needed; banks aren't viewer-scoped, D-01) | Versioned, session-gated read-API pattern. |
| SvelteKit 5 (runes) + Tailwind v4 + `@lucide/svelte` | as shipped | The web tab | The established web stack; `Search`/`ExternalLink` already in use. |
| `web/src/lib/components/InventoryWindow.svelte` | in-repo (P31) | The per-bank detail window (reused UNCHANGED, prop-driven over one `CharacterInventory`) | Generic by construction — the header comment names Phase 33 as a consumer. |
| `web/src/lib/components/{ExaminePanel,StateBlock,LastSyncedCell}.svelte` | in-repo | examine (inside the window) / loading-error-empty states / the holder last-synced cell | Shared, reuse verbatim. |

### Supporting (new files this phase creates — each mirrors an existing analog)
| File | Purpose | Mirrors |
|------|---------|---------|
| `web/src/lib/banks.ts` | Pure A-Z bank-list sort + the `is_bank`-holder filter for BANK-03 (drop non-bank items, keep bank holders) + a per-bank lookup by name | `web/src/lib/roster.ts` + `web/src/lib/items.ts` |
| `web/src/lib/__tests__/banks.test.ts` | Node tests for the above | `web/src/lib/__tests__/{roster,items}.test.ts` |
| `fetchBanks()` in `web/src/lib/api.ts` | Credentialed `GET /api/v1/banks` wrapper | `fetchCharacters()` (api.ts:333) / `fetchItems()` (api.ts:348) |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| A new `/api/v1/banks` carrying roster+per-bank+summary in ONE payload | Compose `GET /api/v1/characters` (filter bank/bot client-side) + a separate `/api/v1/bank-valuation` | Two fetches + the client would have to derive per-bank item counts from `/items` (it doesn't carry them per bank cleanly). One purpose-built read is simpler, gives the per-bank value/plat for D-04 directly, and matches the `RosterFor`→one-handler precedent. **Recommended: one new read.** |
| Reuse `GET /api/v1/items` for BANK-03 (client-filter `is_bank`) | A new bank-scoped item-rollup endpoint | `/items` already carries `is_bank` per holder; the bank slice is a trivial pure filter. A new endpoint duplicates the rollup. **Recommended: reuse `/items`.** |
| Widen the bank scope inside the store (`InventoryJoin` bank branch + coin read) | Post-filter in compute over the full (all-char) reads | Post-filtering pulls every char's rows into memory then discards non-banks; the store WHERE is the right scope boundary (and the platinum read MUST include bots, which only a widened coin read does). **Recommended: widen in the store** (extend-only). |

**Installation:** none — no `npm install`, no `go get`. `[VERIFIED: config.json + code read — every module is in-repo]`

## Architecture Patterns

### System Architecture Diagram

```
   GET /api/v1/banks  (session cookie)            GET /api/v1/items  (session cookie, REUSED P32)
 Browser /banks ───────────────────────► BanksHandler          Browser /banks ──────► ItemsHandler
 (SvelteKit)    fetchBanks()              (RequireSession)      (search box)   fetchItems()  (unchanged)
     │          (credentials:'include')        │                    │                │
     │                                          ▼                    │                ▼
     │                            compute.Banks(ctx, store)          │      compute.Items(...) → []ItemRollup
     │                                  │                            │      (each holder carries is_bank)
     │                  ┌───────────────┴────────────────┐          │                │
     │                  ▼                                 ▼          │   banks.ts: keep items with ≥1 is_bank
     │   InventoryJoin(ctx, BANK SCOPE)        ListBankToons(BANK    │   holder; within each, keep is_bank
     │   = IsBankToon||IsGuildBot rows          SCOPE) = plat/coin   │   holders → per-item bank search
     │   (flat item rows, name-joined price)    on bank+bot toons    │                │
     │                  │                                 │          │                ▼
     │                  └──────────────┬──────────────────┘          │   holder row → goto('/banks?c='+enc(name))
     │                                 ▼                              │   (in-tab selection seam — opens the
     │            buildBankValuation(rows, toons) →                  │    holding bank's window)
     │              BankValuation{ PerBank{name→{value,unpriced}},   │
     │                             GuildTotal, TotalPlatinum }       ▼
     │            + per-bank plat (from BankToon.Plat)          (right pane window)
     │            + per-bank item-count + A-Z bank name list
     │                                 │
     │   BanksView (JSON, snake_case) ◄┘
     ▼
 banks.ts: sortBanksAZ()  ──► bespoke selectable A-Z list (left pane) + ONE summary header on top
                                   │ row click → select(name)  (?c= URL-reflect)
                                   ▼
                             detail (right pane):
                               per-bank header "{name} — {value} pp · {plat} plat"  (D-04, from the list row)
                               + fetchInventory(name) → InventoryWindow   (the live P31 component, unchanged)
```

### Recommended Project Structure (files touched/created)
```
internal/backendsrv/
├── store/
│   ├── readviews.go      # MODIFY: widen InventoryJoin's bank branch from `c.is_bank_toon = 1`
│   │                     #   to `(c.is_bank_toon = 1 OR c.is_guild_bot = 1)` — see Pattern 1 (Option A).
│   │                     #   (Or add a 3rd scope param/branch if View/Bank must keep bank-toons-only.)
│   └── coin.go           # MODIFY/ADD: a ListBankAndBotToons (is_bank_toon=1 OR is_guild_bot=1) coin read
│                         #   for the widened platinum, OR widen ListBankToons (see Pitfall 1 blast radius).
├── compute/
│   ├── banks.go          # NEW: Banks(ctx, *store.Store) (BanksView, error) + buildBanks(...) pure transform
│   │                     #   + BanksView/BankRowSummary structs. Reuses buildBankValuation OR a widened twin.
│   └── banks_test.go     # NEW: table tests (the inventory_test.go BankValuation seed pattern):
│                         #   a bank toon + a guild bot both hold priced+unpriced items + plat; assert
│                         #   per-bank value/plat, guild totals INCLUDE the bot, unpriced count, A-Z order.
├── readapi/
│   └── banks.go          # NEW: BanksHandler (the characters.go twin, but NO viewer id — banks aren't
│                         #   viewer-scoped, D-01; just GET → compute.Banks → [] not null).
└── (cmd/squirebot-server/main.go)  # MODIFY: register GET /api/v1/banks beside /api/v1/items (~line 370)

web/src/
├── lib/
│   ├── api.ts            # MODIFY: add BanksView + BankRowSummary interfaces + fetchBanks(). REUSE ItemRollup.
│   ├── banks.ts          # NEW: pure sortBanksAZ() + bankItemSearch(items) (filter is_bank holders) +
│   │                     #   bankByName() lookup. DOM-free, immutable.
│   └── __tests__/banks.test.ts  # NEW: node tests (roster.test.ts/items.test.ts twin).
└── routes/banks/+page.svelte    # MODIFY (replace P30 placeholder): the master-detail tab.
```

### Pattern 1: Reconcile the bank set — widen the scope to `IsBankToon || IsGuildBot` (LOAD-BEARING, D-01)
**What:** `BankValuationFor` scopes to `is_bank_toon=1` only. D-01 needs bots too. Two clean options for the planner:

- **Option A (recommended — widen the bank branch in the store):** Change `InventoryJoin`'s `bankOnly` WHERE from `c.is_bank_toon = 1` to `(c.is_bank_toon = 1 OR c.is_guild_bot = 1)`, AND add a coin read `ListBankAndBotToons` (or widen `ListBankToons`) to `(is_bank_toon = 1 OR is_guild_bot = 1) AND is_removed = 0`. Then `compute.Banks` calls these widened reads. **Blast-radius check:** `InventoryJoin(ctx, true)` is currently called ONLY by `BankValuationFor` (grep it). The consolidated `bank` *view* tab is served by `readapi.NewViews(st, "bank")` — VERIFY whether that path also calls `InventoryJoin(ctx, true)` before changing the shared branch; if it does and must stay bank-toons-only, prefer Option B.
- **Option B (a dedicated bank-set read):** Leave `InventoryJoin`'s `bankOnly` branch alone; add a NEW store read `InventoryJoinBanksAndBots(ctx)` (or a scope enum) + the widened coin read, consumed only by `compute.Banks`. Zero risk to the legacy `bank` view. Slightly more store code.

**When to use:** This phase MUST pick one — the unreconciled `BankValuationFor` silently drops bot holdings + bot coin.
```go
// Source: store/readviews.go:216-218 — the exact branch to widen (Option A) or twin (Option B).
//   query = base + ` AND c.is_bank_toon = 1` + orderBy          // ← current
//   query = base + ` AND (c.is_bank_toon = 1 OR c.is_guild_bot = 1)` + orderBy   // ← D-01
// Fixed-string branch, NEVER a value interpolation (the existing discipline holds).
```
> **Recommendation:** Option A IF and ONLY IF `InventoryJoin(ctx, true)` is confirmed used solely by `BankValuationFor` (and the legacy `bank` view's reader is a different path); otherwise Option B. Either way, `ListBankToons` is the *coin* source and its bank-toons-only scope DROPS bot platinum — a bot can't have coin today (coin is gated to `is_bank_toon` in `SetCoinTx`/`ErrNotBankToon`), so the platinum may legitimately stay bank-toons-only — but the **item value** MUST include bots. **Document this asymmetry explicitly in the plan** (value = bank+bot; plat = bank-toons-only by the coin-gate, unless an officer also bank-designates the bot). Confirm with the user if the bot-coin gate matters.

### Pattern 2: Per-bank value + per-bank platinum for D-04 (no second fetch)
**What:** `BankValuation.PerBank[name]` gives per-bank `{TotalValue, UnpricedCount}` but NOT plat (plat lives on `BankToon.Plat`). `compute.Banks` joins them: for each bank/bot toon, emit a `BankRowSummary{ name, item_count, value, unpriced, plat }`. The web list holds these rows; the D-04 detail header reads `bankByName(selected)` off the already-loaded list — no extra request.
```go
// Source: compute/inventory.go:360-380 (buildBankValuation) + store/coin.go:33-40 (BankToon.Plat).
// buildBanks: walk the widened toon list; PerBank[name].TotalValue/UnpricedCount + toon.Plat + a per-bank
// item-count (Σ rows for that char, or len of that char's rows) → one BankRowSummary per bank/bot.
```
> **Note:** `BankToon.Plat` is `*int64` (nil = never entered ≠ 0 — the CoinTotals discipline). Carry it as a nullable in the JSON contract (`plat *int64 → "plat": null`) so the header renders "— plat" / "not recorded", never a fabricated 0. The guild summary `TotalPlatinum` is already a non-null `int64` (nil plats skipped), matching D-02's single number.

### Pattern 3: BANK-03 search = the P32 rollup, client-filtered to bank holders (REUSE `/api/v1/items`)
**What:** Fetch `/api/v1/items` (unchanged). A pure `bankItemSearch(items)` keeps only `ItemRollup`s with ≥1 holder where `is_bank === true`, and within each kept item, narrows `holders[]` to the bank holders. The search input filters by name (the `filterItems` pattern). Holder rows deep-link `/banks?c=<name>` (in-tab `?c=` seam).
```ts
// Source: web/src/lib/items.ts:32-47 (filterItems/sortHolders) — the exact shape to mirror in banks.ts.
export function bankItemSearch(rows: ItemRollup[], query: string): ItemRollup[] {
  // 1) map each item to ONLY its is_bank holders; 2) drop items left with zero bank holders;
  // 3) recompute the item's bank summed_qty/holder_count from the kept holders (D-03 shows bank qty);
  // 4) name-filter + A-Z (banks aren't viewer-scoped → plain A-Z, NOT viewer-first).
}
```
> **Important:** `ItemRollup.summed_qty`/`holder_count` are GUILD-WIDE (every holder). For the bank search the displayed qty/holder count should reflect the BANK slice only (D-03 = "which banks hold it, qty/slot"), so recompute them from the filtered bank holders in `banks.ts` — do NOT show the guild-wide totals here (a Blue Diamond held 40× across the guild but 3× in two banks should read "3 · 2 holders" in the Banks tab). Confirm the exact display in the UI-SPEC.

### Pattern 4: The detail = per-bank header + the reused `InventoryWindow` (MIRROR `/characters`'s window column)
**What:** When a bank is selected (`?c=<name>`), the detail column renders the D-04 header (`{name} — {value} pp · {plat} plat`) then runs the EXACT `/characters` window-column state machine: `loadInventory(name)` → its own loading/error/no-inventory states → `<InventoryWindow inventory={inv} />`. Copy the `loadInventory`/`invFor`/stale-drop logic verbatim from `characters/+page.svelte:146-190` — it is already correct (handles the stale-response race + the 401/403 AuthGate re-route).
```svelte
<!-- Source: web/src/routes/characters/+page.svelte:253-279 — the window column to mirror, plus the
     D-04 header above it. The header value/plat come from bankByName(selected) (the loaded list row). -->
```

### Anti-Patterns to Avoid
- **Surfacing `BankValuationFor` as-is for BANK-02:** it drops guild-bot holdings (Pitfall 1). Widen the set first.
- **Joining catalog↔inventory by raw `item_id`:** the canonical landmine. Value is already name-bridged in the store (`pp_rep` CTE) — `compute.Banks` reuses `pricesFromJoin`+`pickPrice`, never re-selects by id.
- **Showing guild-wide item qty in the bank search:** recompute from the bank-holder slice (Pattern 3).
- **A second fetch for the per-bank D-04 number:** it's already in the `/banks` list rows (Pattern 2).
- **Viewer-first ordering on the bank list:** D-01 is plain A-Z (banks aren't anyone's assigned chars). Do NOT reuse `RosterFor`'s viewer-first banding for this list — sort A-Z.
- **A new `{@html}` sink / a re-skin:** the only raw-HTML is `ExaminePanel`'s escaped `composeItemNote` (inside the reused window). Names render via plain `{}`. The 5 EQ themes are reused unchanged.
- **Route-name collision:** `GET /api/v1/banks` is distinct from `GET /api/v1/views/bank` (the legacy consolidated bank-view tab) and `GET /api/v1/coin/bank-toons` (the coin form). Go 1.22+ ServeMux treats them as separate patterns — but do NOT reuse `NewViews(st,"bank")` (that serves the old grid).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Bank value + total platinum aggregation | A new Σ-price loop | `compute.buildBankValuation` + `TotalPlatinum` (P29) — over the WIDENED bank set | Already sums `pickPrice×count`, tracks "+N unpriced", and skips nil plats. Just widen the input scope. |
| Name-keyed price across the EQ↔PigParse id split | A custom id→price lookup | The `pp_rep` name-join CTE in `InventoryJoin` (reached via the widened bank read) | The id-join leaves ~91% of held rows unpriced (`[VERIFIED: readviews.go:36-47]`). Name bridge is solved. |
| The per-bank inventory window (paperdoll + bags + examine) | A bank-specific window | `InventoryWindow.svelte` (prop-driven, P31) over `fetchInventory(bankName)` | Generic by construction; the component comment names Phase 33 as a consumer. Zero change. |
| "Which bank holds item X" rollup | A bank-scoped item endpoint | `GET /api/v1/items` (P32) client-filtered to `is_bank` holders | The rollup already carries `is_bank` per holder; the bank slice is a pure filter. |
| Loading / error / empty / no-results UI | New state markup | `StateBlock.svelte` | Shared copy + a11y; reuse verbatim. |
| The window-column fetch + stale-response race + auth re-route | New load logic | Copy `characters/+page.svelte`'s `loadInventory`/`invFor`/`$effect` block verbatim | Already correct (handles the in-flight-selection-change drop + 401/403 AuthGate). |
| The colored-tile icon fallback + last-synced cell | New tile/date code | `PaperdollSlot`'s `.ico` mechanic (inside the window) + `LastSyncedCell` (for any holder table) | Keeps icons/dates consistent across tabs. |

**Key insight:** Phase 33 writes almost no new algorithm. The ONE backend decision is *scope* (widen `is_bank_toon` to `IsBankToon || IsGuildBot`), and the ONE web algorithm is the bank-holder filter (`banks.ts`). Everything else is wiring shipped parts. The risk is not "can we build it" but "did we (a) widen the bank set, (b) carry per-bank plat as nullable, (c) recompute bank-slice qty in search, (d) keep A-Z not viewer-first, (e) avoid the route collision" — all flagged.

## Common Pitfalls

### Pitfall 1: `BankValuationFor` silently excludes guild bots (D-01 / BANK-02 mismatch)
**What goes wrong:** The guild summary undercounts — a guild *bot* holding 5,000 pp of goods contributes 0 to the displayed total, and the bot never appears in the bank list.
**Why it happens:** `BankValuationFor` → `InventoryJoin(ctx, true)` (`c.is_bank_toon = 1`) + `ListBankToons` (`is_bank_toon = 1`). Both exclude `is_guild_bot=1`. D-01 explicitly wants `IsBankToon || IsGuildBot` (the ROADMAP SC#1 says "guild-bank characters", but the *discussed-and-locked* D-01 + the `RosterFor` band-2 precedent both = bank OR bot). `[VERIFIED: inventory.go:331-340, readviews.go:216-217, coin.go:50, readviews.go:794]`
**How to avoid:** Widen the bank scope (Pattern 1). For item VALUE: include bots (`InventoryJoin` bank branch widened or a twin read). For PLATINUM: bots can't hold coin today (coin is gated to `is_bank_toon` by `SetCoinTx`/`ErrNotBankToon`), so plat may legitimately stay bank-toons-only — but state this asymmetry in the plan and confirm with the user.
**Warning signs:** A known bot's goods missing from the total; the bot absent from the bank list; the total lower than eyeballing the bank windows suggests.

### Pitfall 2: Per-bank platinum has no home on `Valuation` (D-04)
**What goes wrong:** The D-04 per-bank header shows item value but a wrong/zero platinum (or a fabricated 0 for "never entered").
**Why it happens:** `Valuation{TotalValue, UnpricedCount}` carries NO plat; per-bank plat is on `BankToon.Plat` (`*int64`, nil ≠ 0). `[VERIFIED: types.go:194-197, coin.go:33-40]`
**How to avoid:** `compute.Banks` joins each toon's `Plat` into its `BankRowSummary` as a nullable; the header renders "not recorded" when nil (the CoinTotals discipline). Do NOT coerce nil→0.
**Warning signs:** A bank whose coin was never entered showing "0 plat" as if it were recorded; a header plat that doesn't match the bank-coin form.

### Pitfall 3: Bank search shows guild-wide item qty instead of the bank slice (D-03)
**What goes wrong:** "Blue Diamond — 40 · 8 holders" in the Banks tab when only 2 banks hold 3 of them.
**Why it happens:** `ItemRollup.summed_qty`/`holder_count` are guild-wide (every holder). Reusing them verbatim leaks non-bank holdings into the bank search counts. `[VERIFIED: itemrollup.go:89-109]`
**How to avoid:** In `banks.ts`, after filtering `holders[]` to `is_bank` rows, recompute the displayed qty (Σ bank-holder qty) + holder count (distinct bank chars) from the kept holders.
**Warning signs:** Bank-search counts wildly higher than the per-bank windows show; a holder row for a non-bank character appearing in the bank search detail.

### Pitfall 4: Assuming a migration is needed
**What goes wrong:** A wasted goose migration + a deploy that runs DDL it didn't need.
**Why it happens:** Reflex — "new tab = new schema." But bank/bot designation (`is_bank_toon`/`is_guild_bot`) shipped v2.3; bank-coin (`plat` etc.) shipped P15 (00004); icon/statsblock shipped P31 (00012/00013, schema v13). Everything this phase reads already exists. `[VERIFIED: coin.go, readviews.go RosterRow, ItemMasterIconStats, P31 context]`
**How to avoid:** **No new migration.** Confirm in the plan; the deploy is a backend-binary swap (to register `/api/v1/banks`) + a web atomic-swap — NOT a goose run. Take the R2 backup per `docs/backend-deploy.md` anyway.
**Warning signs:** A `00014_*.sql` appearing in a plan; `WatcherMaxSchemaVersion`/`schema_version` churn (none of which this phase touches).

### Pitfall 5: DOM-blind node tests give false confidence
**What goes wrong:** `npm test` green, but the rendered tab is broken (the P15/P31 trap — number coercion, epoch-sec dates, crashing components — all passed node tests).
**Why it happens:** No `@testing-library/svelte` (toolchain-install rule); node vitest is `environment:node` and excludes `*.svelte.{test,spec}.ts`. `[VERIFIED: web vite.config.ts + memory web-tests-node-only-blind-to-dom]`
**How to avoid:** Node-test ONLY the pure `banks.ts`; the rendered tab MUST be browser-smoked on a DEPLOYED build. `npm run dev` cannot auth against prod (cookie Domain=squirebot.quest + apex-only CORS — `[VERIFIED: memory web-local-dev-cant-auth-against-prod]`).
**Warning signs:** "all tests pass" used as the verification verdict for the rendered Banks tab.

### Pitfall 6: Hardcoding colors/fonts instead of theme tokens
**What goes wrong:** A summary header or list row reads correctly under Velious but breaks under Heavy (parchment) or Minimalist.
**Why it happens:** Copying sketch HTML (helper vars / a Google Fonts link) verbatim.
**How to avoid:** Use ONLY the registry tokens; the sketch helper vars MUST map to real tokens; NO Google Fonts link (fonts self-hosted via `@fontsource`). Spot-check Heavy + Minimalist at build. `[VERIFIED: P32 RESEARCH Pitfall 6 + the live themes registry]`
**Warning signs:** literal hex in `banks/+page.svelte`; a reintroduced `--radius`/`--text-dim` global.

## Code Examples

### The new compute function + structs (the contract the planner pins — APPEND to types.go)
```go
// Source pattern: compute/types.go (append-only, snake_case) + inventory.go (public-fn → pure-helper split).
// BanksView — the GET /api/v1/banks payload: the A-Z bank/bot rows + the guild summary.
type BanksView struct {
    Banks         []BankRowSummary `json:"banks"`           // A-Z; one per IsBankToon||IsGuildBot toon
    GuildValue    float64          `json:"guild_value"`     // GuildTotal.TotalValue (D-02)
    GuildUnpriced int64            `json:"guild_unpriced"`  // GuildTotal.UnpricedCount ("+N unpriced")
    TotalPlatinum int64            `json:"total_platinum"`  // Σ plat over bank toons (D-02 / D-04 guild)
}
// BankRowSummary — one bank/bot's clean row (D-02 clean rows) + its D-04 detail-header numbers.
type BankRowSummary struct {
    Name      string  `json:"name"`
    ItemCount int64   `json:"item_count"`     // Σ held inventory rows for this bank (the clean-row count, D-02)
    Value     float64 `json:"value"`          // PerBank[name].TotalValue (D-04)
    Unpriced  int64   `json:"unpriced"`       // PerBank[name].UnpricedCount ("+N unpriced")
    Plat      *int64  `json:"plat"`           // BankToon.Plat; null = never recorded (D-04, CoinTotals discipline)
}
func Banks(ctx context.Context, s *store.Store) (BanksView, error)
func buildBanks(rows []store.InventoryJoinRow, toons []store.BankToon) BanksView  // pure; mirrors buildBankValuation
```

### The new route handler (the items.go/characters.go twin — NO viewer id)
```go
// Source: readapi/items.go:54-83 — but banks are NOT viewer-scoped (D-01 A-Z), so no UserFromContext.
func (h *BanksHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
    bv, err := compute.Banks(r.Context(), h.store)
    if err != nil { slog.Error("banks read failed", "err", err); http.Error(w, "internal error", 500); return }
    if bv.Banks == nil { bv.Banks = []compute.BankRowSummary{} } // [] not null
    w.Header().Set("Content-Type", "application/json"); w.WriteHeader(200)
    _ = json.NewEncoder(w).Encode(bv)
    slog.Info("banks ok", "rows", len(bv.Banks), "status", 200) // V7: count + status only, never a name/value
}
```

### The route registration (the exact site to edit)
```go
// Source: cmd/squirebot-server/main.go:370 — add beside the Phase 32 /items route.
mux.Handle("GET /api/v1/items", webauth.RequireSession(db, readapi.NewItems(st)))
mux.Handle("GET /api/v1/banks", webauth.RequireSession(db, readapi.NewBanks(st)))   // NEW (Phase 33 / BANK-01/02)
```

### The new fetch wrapper (the fetchItems twin)
```ts
// Source: web/src/lib/api.ts:348-350 — credentialed getJSON; typed 401/403 the AuthGate re-routes.
export function fetchBanks(f: typeof fetch = fetch): Promise<BanksView> {
    return getJSON<BanksView>('/api/v1/banks', f);
}
```

### The in-tab holder deep-link for BANK-03 (the ?c= selection seam, now in /banks)
```svelte
<!-- Source: inventory/+page.svelte:330-333 — there it goes to /characters?c=; here it stays in /banks. -->
<a href={`/banks?c=${encodeURIComponent(h.char)}`} aria-label={`View ${h.char}`}>{h.char}</a>
<!-- /banks pre-selects from ?c= on mount (the characters/+page.svelte:88-92 onMount pattern). -->
```

## File-by-File Plan Inputs

### Backend (create)
| File | Action | Mirrors / Notes |
|------|--------|-----------------|
| `internal/backendsrv/compute/banks.go` | CREATE | `inventory.go` BankValuation pattern. Holds `Banks`, `buildBanks`, `BanksView`, `BankRowSummary`. Reuses `buildBankValuation` (or a widened twin) + `TotalPlatinum`; joins per-bank `Plat` + per-bank item count; sorts A-Z. |
| `internal/backendsrv/compute/banks_test.go` | CREATE | `inventory_test.go` BankValuation seeds (a bank toon + a guild bot both holding priced + unpriced items + plat). Asserts: guild total INCLUDES the bot (Pitfall 1), per-bank value/plat (Pitfall 2), unpriced count, A-Z order, nil-plat → null carried. |
| `internal/backendsrv/readapi/banks.go` | CREATE | `items.go`/`characters.go` twin but NO viewer id (banks aren't viewer-scoped). `[]` not null; V5/V7 discipline. |

### Backend (modify)
| File | Action | Notes |
|------|--------|-------|
| `internal/backendsrv/store/readviews.go` | MODIFY | Widen `InventoryJoin`'s `bankOnly` branch to `(c.is_bank_toon = 1 OR c.is_guild_bot = 1)` (Option A) **after confirming the legacy `bank` view doesn't depend on bank-toons-only**, OR add a twin read `InventoryJoinBanksAndBots(ctx)` (Option B). Fixed-string branch, no interpolation. |
| `internal/backendsrv/store/coin.go` | MODIFY | Add `ListBankAndBotToons` (`is_bank_toon = 1 OR is_guild_bot = 1`) for the bank+bot item-value scope — OR keep `ListBankToons` for *coin* (plat is bank-toon-gated) and use the widened `InventoryJoin` only for value. Decide per Pitfall 1's value/plat asymmetry. |
| `cmd/squirebot-server/main.go` | MODIFY (~line 370) | Register `GET /api/v1/banks` under `webauth.RequireSession` beside `/api/v1/items`. ONE line. |

### Web (create)
| File | Action | Mirrors |
|------|--------|---------|
| `web/src/lib/banks.ts` | CREATE | `roster.ts` + `items.ts` — `sortBanksAZ()`, `bankItemSearch(items, query)` (filter to `is_bank` holders, recompute bank-slice qty), `bankByName()`. Pure, immutable, DOM-free. |
| `web/src/lib/__tests__/banks.test.ts` | CREATE | `roster.test.ts`/`items.test.ts` — node cases: A-Z sort; bankItemSearch drops zero-bank-holder items + keeps only bank holders + recomputes qty; bankByName lookup; nil-plat handling. |

### Web (modify)
| File | Action | Notes |
|------|--------|-------|
| `web/src/lib/api.ts` | MODIFY | Add `BanksView` + `BankRowSummary` interfaces (snake_case, mirroring the Go contract; `plat: number \| null`) + `fetchBanks()`. REUSE `ItemRollup`/`ItemHolder` for the search. |
| `web/src/routes/banks/+page.svelte` | MODIFY (replace placeholder) | Master-detail mirror of `/inventory` + `/characters`: `onMount` `fetchBanks()` (+ pre-select `?c=`); the D-02 summary header on top (`Guild banks: ~{value} pp · {plat} plat` + "+N unpriced"); a search box that `fetchItems()` once + `bankItemSearch` (BANK-03) with holder rows deep-linking `/banks?c=`; the bespoke A-Z bank list (mirror `/characters` `.row`); the detail = the D-04 per-bank header (`bankByName(selected)`) + the `/characters` window-column state machine (`loadInventory`→`InventoryWindow`). Theme tokens only; NO new `{@html}`. |

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Consolidated `bank` *view* tab (the P14 grid, `/api/v1/views/bank` + `/guild-views?view=bank`) | A banks-only master-detail tab over the reused inventory window + a value/plat summary | Phase 33 (this) | The new tab is the v2.4 surface; the legacy grid stays reachable but is no longer the primary banks UX. Do NOT extend `NewViews(st,"bank")`. |
| `BankValuationFor` scoped to `is_bank_toon=1` (P29 D-04 wording) | The Banks tab needs `IsBankToon \|\| IsGuildBot` (D-01) — widen the bank set | Phase 33 (this) | The shipped valuation must be widened for value (bots included); plat stays bank-toon-gated by the coin rule. The central backend decision (Pitfall 1). |
| `item_id` as "the stable join key" (CLAUDE.md text) | Join/group catalog↔inventory by NORMALIZED NAME (`pp_rep` CTE) | 2026-06-06 (commit 0a169f3) | Value reuses the name-bridged price; never re-join by id. |
| Per-character view tabs forbidden (Google 200-tab limit) | Per-bank master-detail drill-down ALLOWED (one reusable window on selection) | 2026-06-17 (consolidated-views RELAXED) | The Banks tab's single reusable `InventoryWindow` is sanctioned. |

**Deprecated/outdated:**
- `GET /api/v1/views/bank` (the P14 consolidated grid) as the v2.4 banks surface — superseded by `/banks`. Reuse nothing from `NewViews`.
- Surfacing `BankValuationFor` verbatim for BANK-02 — it drops bots (Pitfall 1). Widen first.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The new route path is `GET /api/v1/banks` (distinct from `/views/bank` + `/coin/bank-toons`). | Backend route | Low — a one-line change; the only hard constraint is "not `/views/bank`, not reusing `NewViews`". |
| A2 | Guild bots cannot hold platinum today (coin is `is_bank_toon`-gated by `SetCoinTx`/`ErrNotBankToon`), so per-bank/guild PLATINUM may stay bank-toons-only while item VALUE includes bots. | Pitfall 1 / Pattern 1 | Medium — if the user intends bot platinum too, the coin read AND the coin-write gate would need widening (a bigger change touching `SetCoinTx`). Confirm in discuss/planning. **Flag for user confirmation.** |
| A3 | `InventoryJoin(ctx, true)` is used ONLY by `BankValuationFor` (so Option A's WHERE-widen is safe); the legacy `bank` view goes through `compute.Bank`/`NewViews` which I have NOT fully traced. | Pattern 1 (Option A) | Medium — if the legacy `bank` view shares `InventoryJoin(ctx, true)` and must stay bank-toons-only, Option A would change that grid too. **The planner MUST grep `InventoryJoin(ctx, true)` callers before choosing Option A; default to Option B if any other caller exists.** |
| A4 | The bank-search displayed qty/holder-count should reflect the BANK slice (recomputed), not the guild-wide rollup numbers. | Pattern 3 / Pitfall 3 | Low — a presentation choice; if the user wants guild-wide qty shown with a bank tag, that's simpler. Confirm in the UI-SPEC. |
| A5 | The per-bank item count (D-02 clean row) = Σ held inventory rows for that bank (flat, incl. bag contents — same scope as value). | Pattern 2 | Low — matches the value scope (D-02 "name + item count"); if the user means distinct item names, it's a one-line change in `buildBanks`. |
| A6 | No new migration (all reads exist). | Pitfall 4 | Very low — verified against coin.go/RosterRow/ItemMasterIconStats; everything read already shipped. |

## Open Questions

1. **Does guild-bot platinum count toward the total (A2)?**
   - What we know: item VALUE clearly includes bot holdings (D-01). Coin is `is_bank_toon`-gated at the store, so a *bot* can't have plat entered today.
   - What's unclear: whether the user wants bot platinum to be enterable/counted (would need the coin write-gate widened too).
   - Recommendation: ship value = bank+bot, plat = bank-toons-only (the existing coin rule); note it in the summary copy if needed. Confirm with the user; if they want bot coin, scope a coin-gate change (bigger, touches `SetCoinTx`).

2. **Option A (widen the shared bank branch) vs Option B (a dedicated bank+bot read) (A3).**
   - What we know: `BankValuationFor` is the only confirmed caller of `InventoryJoin(ctx, true)`; the legacy `bank` view path (`compute.Bank`/`NewViews`) wasn't fully traced this session.
   - Recommendation: grep `InventoryJoin(ctx, true)` callers in planning; Option A if `BankValuationFor` is the sole caller, else Option B. The plan-checker will sanity-check.

3. **Bank-search qty: bank slice vs guild-wide (A4).**
   - Recommendation: bank slice (recomputed) — matches "what's in the *banks*". Pin the exact display string in the UI-SPEC.

## Environment Availability

> Code/config-only over an already-running stack. The only "external" surface is the LIVE deployed API+web (for the browser-smoke), the established deploy path.

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain (`go build`/`go test`) | backend compute + route + store widen | ✓ (shipped P29–32) | 1.24 | — |
| Node + web `npm` scripts (`check`/`test`/`build`) | web tab + pure `banks.ts` | ✓ (shipped) | adapter-static | — |
| Live prod API+web (squirebot.quest / api.squirebot.quest) | browser-smoke of the rendered tab | ✓ (live since P11; P32 deployed 2026-06-18) | — | deploy-then-smoke OR a full local stack (local backend + `SQUIREBOT_COOKIE_INSECURE` + `PUBLIC_API_BASE` + seeded `sb_session`) per memory |
| SSH to the Hetzner box (for deploy) | deploying the new route + web bundle | ✓ (ssh-agent + id_ed25519) | — | — |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** prod-auth for the smoke — use deploy-then-smoke. **No migration this phase** → the deploy is a backend-binary swap (to register `/api/v1/banks` — the server MUST restart) + a web atomic-swap, NOT a goose run.

## Security Domain

> `security_enforcement: true`, ASVS level 1, block-on `high`. The Banks tab is a READ-ONLY browse/select surface — no destructive actions, no writes.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | `GET /api/v1/banks` is `webauth.RequireSession` (login-only since P15). `/api/v1/items` (reused) is already session-gated. No new auth surface. |
| V3 Session Management | yes | The shipped `getJSON` `credentials:'include'` (cookie Domain=squirebot.quest, cross-subdomain) + typed 401/403 → `AuthGate`. No new session surface. |
| V4 Access Control | yes | Membership gate, NOT ownership/officer: every signed-in member sees every bank + its window. The bank list is NOT viewer-scoped (D-01 A-Z). NEVER `RequireOfficer`. The bank set is a data predicate (`IsBankToon \|\| IsGuildBot`), not an access boundary. |
| V5 Input Validation | yes | `/api/v1/banks` takes NO user input server-side (no query params; no viewer id even). The per-bank window read (`/inventory/{char}`, reused) binds `{char}` as a single `?` placeholder (T-31-06). The bank search is a client-side filter over `/items` — never hits SQL. No name is concatenated into any SQL. |
| V6 Cryptography | no | No crypto in this phase. |

### Known Threat Patterns for {Go read API + SvelteKit render over guildie-controlled names}

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| SQL injection via bank/item names | Tampering | Names are NEVER concatenated into SQL — `compute.Banks` is pure Go over `InventoryJoin`/`ListBankToons` rows; the only dynamic SQL is the fixed-string `is_bank_toon OR is_guild_bot` branch (no value interpolation). The window read binds `{char}` as `?`. |
| Reflected/stored XSS via guildie-controlled bank or item names | Tampering / Elevation | Bank + item + character names render via plain `{}` (Svelte auto-escapes) in the list, summary, per-bank header, search results, and holders. The ONLY raw-HTML is `ExaminePanel`'s `composeItemNote` (escapeHtml + scheme allow-list), reused unchanged inside the window. This phase adds NO new `{@html}` sink. |
| Info disclosure via logs | Info Disclosure | V7: slog on `/api/v1/banks` carries op + row count + status + err ONLY — never a bank name, value, or platinum figure. |
| IDOR / over-broad exposure | Elevation of Privilege | The read is guild-wide BY DESIGN (membership gate). There is NO per-viewer scoping in this tab; nothing client-supplied selects what's returned (no viewer id, no body). The holder deep-link encodeURIComponent's the guildie-controlled char name. |
| DoS via the icon `<img>` source | Tampering | `icon_id` is a trusted INTEGER from the weekly wiki job (`Item_${int}.png`, inside the reused window/examine) — no guildie string in the path; a bad/empty id falls back to the colored tile. |

## Verification Approach

> `nyquist_validation: false` → no formal Validation Architecture section. Verification is the established three-tier path: Go tests + web node tests (pure helpers) + deploy-then-browser-smoke (rendered tab).

**BANK-01 (banks-only list w/ each opening its window):**
- *Automated:* `go test ./internal/backendsrv/compute/...` — `banks_test.go` asserts `compute.Banks` returns BOTH a bank toon AND a guild bot, A-Z, each with a per-bank item count; NON-bank/bot chars are absent. `web vitest banks.test.ts` asserts `sortBanksAZ` ordering.
- *Browser-smoke (deployed):* the list shows only bank+bot chars, A-Z; selecting one opens its `InventoryWindow` (paperdoll + general/bank grids + bag expand + examine); a non-bank char never appears.

**BANK-02 (total item value + total platinum):**
- *Automated:* `banks_test.go` — `GuildValue` = Σ `pickPrice×count` over bank+bot held rows (INCLUDING the bot — Pitfall 1), `GuildUnpriced` counts the unpriced, `TotalPlatinum` = Σ bank-toon plat (nil plats skipped). `banks.test.ts` asserts the summary formatting helper.
- *Browser-smoke (deployed):* the summary header reads `Guild banks: ~{value} pp · {plat} plat` (+ "+N unpriced" when any); a known bot's goods are reflected in the total; an un-priced item doesn't crash the figure.

**BANK-03 (per-item bank search):**
- *Automated:* `banks.test.ts` — `bankItemSearch` keeps only items with ≥1 `is_bank` holder, narrows each item's holders to bank holders, recomputes the bank-slice qty/holder count (Pitfall 3), and name-filters A-Z; a no-match returns []. `itemrollup`'s `is_bank` flag is already covered by P32's tests.
- *Browser-smoke (deployed):* typing an item name lists the bank(s) holding it with qty/slot; the counts reflect the bank slice (not guild-wide); **clicking a holder selects that bank and opens its window in-tab (`/banks?c=`)**; a no-match shows the `StateBlock kind="no-results"`.

**BANK-01/02 detail (D-04 per-bank header):**
- *Browser-smoke (deployed):* selecting a bank shows `{name} — {value} pp · {plat} plat` above the window; a bank with no coin recorded shows "not recorded"/"— plat", not "0 plat"; the per-bank value matches that bank's slice of the guild total.

**Regression (every phase):** `go test ./...` all packages ok (incl. the new `banks_test.go` AND the existing `inventory_test.go` BankValuation tests still green after the scope widen); `web npm run check` 0/0 + `npm test` green (incl. `banks.test.ts`) + `npm run build` ok (adapter-static). **Re-run `inventory_test.go` after the Pattern-1 widen** — if Option A changed the shared bank branch, confirm the legacy `bank` view tests still pass (or that no such coupling exists).

**Deploy note:** NO migration. Backend-binary swap (to register `/api/v1/banks` — the server MUST restart) + a web atomic-swap. R2 backup per `docs/backend-deploy.md` even without a migration. All 5 themes spot-checked (Heavy + Minimalist contrast).

## Sources

### Primary (HIGH confidence — code read this session)
- `internal/backendsrv/compute/inventory.go` — `BankValuationFor`/`buildBankValuation`/`TotalPlatinum`/`classifySlot`/`pickPrice` reuse (the bank-toons-only scope + the per-bank/guild/plat aggregation).
- `internal/backendsrv/compute/itemrollup.go` — `Items`/`buildItemRollups`/`slotLabel` + the `is_bank = IsBankToon||IsGuildBot` per-holder flag (BANK-03 reuse).
- `internal/backendsrv/compute/types.go` — `BankValuation`/`Valuation`/`ItemRollup`/`ItemHolder`/`InventorySlot`/`CharacterInventory` contracts (append-only seam).
- `internal/backendsrv/store/readviews.go` — `InventoryJoin` (`pp_rep` name-join + the `bankOnly` `is_bank_toon=1` branch to widen), `InventoryForChar`, `RosterFor`/`rosterBand` (the `IsBankToon||IsGuildBot` band-2 precedent), `ItemMasterIconStats`.
- `internal/backendsrv/store/coin.go` — `ListBankToons` (`is_bank_toon=1` plat source), `BankToon.Plat` (`*int64` nullable), `SetCoinTx`/`ErrNotBankToon` (the bank-toon coin gate → bot-plat asymmetry).
- `internal/backendsrv/readapi/{items.go,characters.go,inventory.go}` — the route twins (session-gated; `[]` not null; viewer-id vs no-viewer-id; the `{char}` window route reused per bank).
- `cmd/squirebot-server/main.go:280-379` — the `RequireSession` route registration site + the existing `/views/bank`, `/coin/bank-toons`, `/items` routes (collision check).
- `web/src/routes/{characters,inventory,banks}/+page.svelte` — the master-detail mirror (`/characters` window column; `/inventory` list+search+holders), the `?c=` selection seam, the P30 banks placeholder to replace.
- `web/src/lib/components/InventoryWindow.svelte` — confirmed generic/prop-driven; the comment names Phase 33 as a consumer.
- `web/src/lib/{api.ts,items.ts,roster.ts}` — `getJSON`/`fetchItems`/`fetchInventory`/`fetchCharacters` wrappers, the `ItemRollup`/`ItemHolder` interfaces, the pure `filterItems`/`sortHolders`/`viewerFirst`/`bandOf` helpers to mirror.
- `internal/backendsrv/compute/inventory_test.go` — the BankValuation seed/assert pattern for `banks_test.go`.
- `.planning/phases/33-banks-tab-valuation/33-CONTEXT.md` (locked D-01..D-04 + Claude's-discretion), `.planning/phases/{29,31,32}-*` contexts/research, `.planning/REQUIREMENTS.md`, `.planning/ROADMAP.md` Phase 33, `.planning/config.json`.

### Secondary (MEDIUM confidence — project memory, cross-checked against code)
- `pigparse-vs-ingame-item-id-namespaces` — name-join, never raw item_id (confirmed in `readviews.go` `pp_rep`).
- `web-tests-node-only-blind-to-dom` + `web-local-dev-cant-auth-against-prod` — verification path (confirmed by the P32 `vite.config.ts` notes).
- `project_consolidated_views` — per-bank master-detail RELAXED (confirmed in CLAUDE.md + the `InventoryWindow` reuse comment).

### Tertiary (LOW confidence)
- None — every claim is code-verified or cited; no unverified web/training claims were needed.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new dependencies; every module is in-repo and code-read.
- Architecture: HIGH — the tab is an exact mirror of `/characters` + `/inventory`; the valuation + window + rollup are shipped surfaces.
- Pitfalls: HIGH for the load-bearing ones (the `is_bank_toon`→`IsBankToon||IsGuildBot` scope gap, per-bank-plat nullability, bank-slice qty, no-migration, DOM-blind tests) — all verified against code + memory.
- The Option-A-vs-B store decision + the bot-platinum question: MEDIUM — both fully characterized, but need a 1-grep confirmation (`InventoryJoin(ctx, true)` callers) and a user call (bot coin), flagged in Open Questions A2/A3.

**Research date:** 2026-06-18
**Valid until:** 2026-07-18 (stable — internal-only surface; revisit if the backend route idiom, the bank/bot designation model, or the EQ-theme token set changes).

## RESEARCH COMPLETE
