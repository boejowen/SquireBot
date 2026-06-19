---
phase: 33-banks-tab-valuation
verified: 2026-06-19T21:20:00Z
status: passed
score: 12/12 must-haves verified
overrides_applied: 0
---

# Phase 33: Banks Tab + Valuation Verification Report

**Phase Goal:** A guildie can answer "what's in the guild banks, and what is it worth?" — a banks-only list where selecting a bank opens its in-game inventory window, a guild-wide summary (total PigParse item value + total platinum), and a per-item search across bank holdings.
**Verified:** 2026-06-19T21:20:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| #   | Truth | Status | Evidence |
| --- | ----- | ------ | -------- |
| 1 | `GET /api/v1/banks` returns the `IsBankToon\|\|IsGuildBot` roster A-Z, each with per-bank item count, value, nullable plat | ✓ VERIFIED | `compute.Banks` → `ListBankAndBotToons` (coin.go:81, `WHERE (is_bank_toon=1 OR is_guild_bot=1) ... ORDER BY name COLLATE NOCASE`) + per-bank `BankRowSummary` (banks.go:67-76). `TestBanks_OK_EncodesView` + `TestListBankAndBotToons` PASS (fresh `-count=1`). |
| 2 | The guild summary (total item value across bank+bot holdings + total platinum) is computed and returned | ✓ VERIFIED | `BanksView.GuildValue/TotalPlatinum` from `buildBankValuation` (banks.go:63-65), NOT recomputed. `TestBanks_GuildBotValueIncluded` asserts `GuildValue==180` (bank 100 + bot 80), `TotalPlatinum==500`. PASS. |
| 3 | A guild bot's held goods ARE included in the guild item-value total (the scope-widen regression) | ✓ VERIFIED | `InventoryJoinBanksAndBots` (readviews.go:296-304) widened WHERE `(is_bank_toon=1 OR is_guild_bot=1)`; the legacy `InventoryJoin(ctx,true)` bankOnly branch (readviews.go:203) is UNCHANGED. Pitfall-1 test PASS. |
| 4 | The route is session-gated (`RequireSession`), NOT officer, NOT public; returns `[]` not null on empty | ✓ VERIFIED | main.go:378 `mux.Handle("GET /api/v1/banks", webauth.RequireSession(db, readapi.NewBanks(st)))`; `TestBanks_RequireSession_401WithoutCookie` (401) + `TestBanks_Empty_EncodesArrayNotNull` (`banks:[]`) PASS. No `RequireOfficer`, no `UserFromContext` in banks.go. |
| 5 | The Banks tab shows a guild-wide valuation summary header (total item value pp + total platinum) | ✓ VERIFIED | banks/+page.svelte:221-230 — `GUILD BANKS` eyebrow + `{guild_value} pp · {total_platinum} plat` from real `banksView`. |
| 6 | The bank list shows only `IsBankToon\|\|IsGuildBot` chars, A-Z, name + item count + tag | ✓ VERIFIED | banks/+page.svelte:248-272 — `sortBanksAZ(banksView.banks)` rows (name + `{item_count} items` + "bank" tag), `select(b.name)` on click. Source roster is the bank+bot store read. |
| 7 | Selecting a bank pins a per-bank value/plat header + the reused `InventoryWindow` | ✓ VERIFIED | banks/+page.svelte:363-384 — D-04 header `{selectedBank.value} pp · {plat}` ABOVE `<InventoryWindow inventory={inv} />` (reused P31 component, mounted unchanged). `loadInventory`→`fetchInventory(bank)` window machine copied from /characters. |
| 8 | Typing a query toggles the left column to item-search scoped to bank holders, with bank-slice qty | ✓ VERIFIED | banks/+page.svelte:118-119 `searching`/`searchResults = bankItemSearch(items, query)`; banks.ts:38-51 filters `is_bank` holders + RECOMPUTES `summed_qty`/`holder_count`. Pitfall-3 test asserts 3/2 not 40/8. PASS. |
| 9 | Clicking an item-search holder pins THAT bank's window in-tab (no route change) | ✓ VERIFIED | banks/+page.svelte:297-311 — holder `<button onclick={() => select(h.char)}>`; same `select()` the list rows use; URL via `history.replaceState` (`?b=`), no nav. Zero `/characters?c=` occurrences. |
| 10 | Nil plat renders "not recorded", never "0 plat" | ✓ VERIFIED | banks/+page.svelte:347-352 and 375-380 — `{#if selectedBank.plat === null}<span>not recorded</span>{:else}{plat} plat{/if}`. `Plat *int64` carried nil end-to-end (types.go:276, banks.go:74). |
| 11 | Per-bank value/platinum header above the reused window (D-04), sourced from the loaded list (no second fetch) | ✓ VERIFIED | banks/+page.svelte:127-129 `selectedBank = bankByName(banksView.banks, selected)`; banks.ts:66-71. Header at :369-382. |
| 12 | NO new migration — schema stays v13 | ✓ VERIFIED | Glob of migrations dir: highest is `00013_item_statsblock.sql`; no `00014_*`. SUMMARY 33-03 records live `goose: no migrations to run. current version: 13`. |

**Score:** 12/12 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `internal/backendsrv/store/readviews.go` | `InventoryJoinBanksAndBots` dedicated twin (Option B) | ✓ VERIFIED | :296-304; shares `inventoryJoinBase`/`inventoryJoinOrderBy`/`scanInventoryJoinRows` with legacy read; fixed-string WHERE `(c.is_bank_toon=1 OR c.is_guild_bot=1)`. |
| `internal/backendsrv/store/coin.go` | `ListBankAndBotToons` widened toon list; `SetCoinTx` untouched | ✓ VERIFIED | `ListBankAndBotToons` :81-104; `ListBankToons` (legacy) :46-69 unchanged; `SetCoinTx` :124 unchanged. |
| `internal/backendsrv/compute/banks.go` | `Banks` + `buildBanks` (zero SQL, reuse `buildBankValuation`) | ✓ VERIFIED | :32-78; reads widened twins, never `InventoryJoin(ctx,true)` (grep 0). |
| `internal/backendsrv/compute/types.go` | `BanksView` + `BankRowSummary` (snake_case, `Plat *int64`) | ✓ VERIFIED | :262-277, append-only, nullable plat. |
| `internal/backendsrv/readapi/banks.go` | `BanksHandler` (no viewer id; `[]` not null; V7 slog) | ✓ VERIFIED | :35-72; no `UserFromContext`, no `RequireOfficer`. |
| `cmd/squirebot-server/main.go` | `GET /api/v1/banks` under `RequireSession` | ✓ VERIFIED | :378, distinct from `/views/bank` + `/coin/bank-toons`. |
| `web/src/lib/api.ts` | `BanksView`/`BankRowSummary` + `fetchBanks()` | ✓ VERIFIED | :364-387; `plat: number\|null`; `getJSON<BanksView>('/api/v1/banks')`. P32 `ItemRollup`/`ItemHolder` reused, not redeclared. |
| `web/src/lib/banks.ts` | `sortBanksAZ` + `bankItemSearch` (recompute) + `bankByName` | ✓ VERIFIED | :23-71; DOM-free; bank-slice qty recompute. |
| `web/src/routes/banks/+page.svelte` | master-detail tab (summary + list/search + per-bank detail + window) | ✓ VERIFIED | full file; placeholder gone (grep "coming soon" = 0), no `{@html}`, no cross-tab nav. |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | -- | --- | ------ | ------- |
| readapi/banks.go | compute.Banks | `ServeHTTP` calls `compute.Banks(r.Context(), h.store)` | ✓ WIRED | banks.go:53 |
| compute/banks.go | store.InventoryJoinBanksAndBots | `Banks` reads widened scope (NOT `InventoryJoin(ctx,true)`) | ✓ WIRED | banks.go:33,37 |
| main.go | readapi.NewBanks | `mux.Handle GET /api/v1/banks RequireSession` | ✓ WIRED | main.go:378 |
| banks/+page.svelte | /api/v1/banks | `fetchBanks()` on mount | ✓ WIRED | :74 (Promise.all) |
| banks/+page.svelte | /api/v1/items | `fetchItems()` once → `bankItemSearch` | ✓ WIRED | :74, :119 |
| banks/+page.svelte | /api/v1/inventory/{char} | `fetchInventory(bank)` on selection → InventoryWindow | ✓ WIRED | :152, :384 |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| -------- | ------------- | ------ | ------------------ | ------ |
| banks/+page.svelte summary | `banksView.guild_value/total_platinum` | `fetchBanks()` → `compute.Banks` → `buildBankValuation` over live SQLite join | ✓ Yes (DB-backed) | ✓ FLOWING |
| banks/+page.svelte list | `banksView.banks` | same `fetchBanks()` | ✓ Yes | ✓ FLOWING |
| banks/+page.svelte search | `items` → `bankItemSearch` | `fetchItems()` → `compute.Items` rollup over live DB | ✓ Yes | ✓ FLOWING |
| banks/+page.svelte window | `inv` | `fetchInventory(bank)` → P31 read | ✓ Yes | ✓ FLOWING |

No hollow props, no static fallbacks — every render path is a real fetch over the live store.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Go module builds | `go build ./...` | rc=0 | ✓ PASS |
| Bank backend tests (compute/store/readapi, fresh) | `go test -count=1 -run "Banks\|BankAndBot\|InventoryJoinBanksAndBots"` | 9 tests PASS (Pitfall-1, MR-02, empty, twin read, toon list, 401, []-not-null, OK-encode, 405) | ✓ PASS |
| Web banks unit tests | `npx vitest run banks` | 11/11 PASS (incl. Pitfall-3 recompute 3/2 not 40/8) | ✓ PASS |
| Web typecheck | `npm run check` | 0 errors / 0 warnings (509 files) | ✓ PASS |
| Web production build | `npm run build` | built in 9.21s, adapter-static ok | ✓ PASS |
| No new migration | Glob `migrations/*.sql` | highest = `00013_item_statsblock.sql` | ✓ PASS |
| Live route registered | (operator-confirmed, 33-03-SUMMARY) | external `/api/v1/banks` → 401 (was 404) | ✓ PASS (operator) |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ----------- | ----------- | ------ | -------- |
| BANK-01 | 33-01, 33-02, 33-03 | Lists only guild-bank characters (same ordering style as Characters); each opens its inventory window | ✓ SATISFIED | `ListBankAndBotToons` A-Z roster + banks list rows + `select`→`fetchInventory`→`InventoryWindow` (truths 1, 6, 7, 9) |
| BANK-02 | 33-01, 33-02, 33-03 | Total PigParse value of all items held by bank characters + total platinum across guild banks | ✓ SATISFIED | `compute.Banks` guild summary (value includes bots; plat bank-toon-gated) + D-02 header (truths 2, 3, 5) |
| BANK-03 | 33-02, 33-03 | Per-item name search across items held by the guild banks | ✓ SATISFIED | `bankItemSearch` `is_bank` filter + bank-slice qty recompute + in-tab holder deep-link (truths 8, 9) |

No orphaned requirements — REQUIREMENTS.md maps exactly BANK-01/02/03 to Phase 33, all claimed by plans.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| (none) | — | — | — | No stubs, TODOs, placeholder data, or empty-handler patterns found in any Phase 33 file. All data paths are real fetches; the only `return BanksView{}` paths (banks.go:35,39) are error returns, not stubs. |

### Human Verification Required

None outstanding. The load-bearing browser-smoke (the only DOM-blind gap, per the project's node-vitest limitation) was already performed by the operator: 33-03-SUMMARY records the 10-point UI-SPEC checklist PASSED across all 5 EQ themes (Velious, Vanilla, Kunark, Minimalist, Heavy) on the live deployed build — including the value/platinum numbers, the bank-slice search qty, the nil-plat "not recorded" copy, and the in-tab holder deep-link. No fix-forward was needed. This verifier independently confirmed the code behind those claims compiles, tests green, and is wired end-to-end.

### Gaps Summary

No gaps. All 12 must-haves verified against the actual codebase (not SUMMARY claims):

- **Option B (D-01)** is genuinely a dedicated twin (`InventoryJoinBanksAndBots`) sharing the legacy read's SQL body via extracted consts; the legacy `InventoryJoin(ctx,true)` bankOnly branch and its two callers (`BankValuationFor` + the `/views/bank` grid) are byte-for-byte untouched. The shared `InventoryJoin(ctx, true)` was NOT widened.
- **D-02** surfaces the existing `buildBankValuation` totals (not recomputed); value includes bots, platinum is bank-toon-only (bot `Plat` nil → 0). `SetCoinTx` unchanged.
- **D-03** reuses `/api/v1/items`, filters to `is_bank`, and RECOMPUTES the bank-slice qty (proven by the Pitfall-3 test); holder click pins the bank window in-tab (no `/characters?c=`).
- **D-04** renders the per-bank value/plat header above the reused P31 `InventoryWindow`, sourced from the loaded list with no second fetch.
- **No migration** — schema confirmed v13; highest file `00013_item_statsblock.sql`.
- All gates green locally: `go build`, fresh `go test` on the bank packages, `npm run check` (0/0), `npx vitest run banks` (11/11), `npm run build`.

**Minor doc-sync observation (non-blocking, not a code gap):** REQUIREMENTS.md still shows BANK-01/02/03 as `[ ]` / "Pending" (lines 39-41, 86-88). This is planning-doc bookkeeping that lags the code (planning docs are committed separately under `commit_docs:false`); the code fully delivers all three. No action required for goal achievement — flagging only so the requirement checkboxes get flipped when the planning docs are next synced.

---

_Verified: 2026-06-19T21:20:00Z_
_Verifier: Claude (gsd-verifier)_
