---
phase: 33-banks-tab-valuation
reviewed: 2026-06-18T21:12:40Z
depth: deep
files_reviewed: 9
files_reviewed_list:
  - internal/backendsrv/store/readviews.go
  - internal/backendsrv/store/coin.go
  - internal/backendsrv/compute/banks.go
  - internal/backendsrv/compute/types.go
  - internal/backendsrv/readapi/banks.go
  - cmd/squirebot-server/main.go
  - web/src/lib/api.ts
  - web/src/lib/banks.ts
  - web/src/routes/banks/+page.svelte
findings:
  blocker: 0
  high: 0
  medium: 1
  warn: 3
  info: 3
  total: 7
status: clean
---

# Phase 33: Code Review Report — Banks Tab + Valuation

**Reviewed:** 2026-06-18T21:12:40Z
**Depth:** deep (cross-file: store ↔ compute ↔ readapi ↔ web)
**Files Reviewed:** 9
**Status:** clean (no BLOCKER / HIGH)

## Summary

Phase 33 adds a guild-wide Banks tab: a dedicated bank+bot store read (`InventoryJoinBanksAndBots`, `ListBankAndBotToons`), a pure `compute.Banks` valuation transform, a session-gated `GET /api/v1/banks` route, the typed `fetchBanks()` client wrapper + `BanksView`/`BankRowSummary` interfaces, pure `$lib/banks` helpers (node-tested), and the `banks/+page.svelte` master-detail tab.

Every load-bearing invariant the prompt flagged was checked and holds:

- **Option B integrity — VERIFIED.** The legacy `/views/bank` grid still flows through `compute.Bank` (`bank.go:35`) → `InventoryJoin(ctx, true)` (the bank-toons-only branch), completely untouched. The new code adds a *separate* `InventoryJoinBanksAndBots` method that shares the read-only `inventoryJoinBase` const but appends its own `(c.is_bank_toon = 1 OR c.is_guild_bot = 1)` predicate. No second caller of the shared `bankOnly` branch was widened. `compute.BankValuationFor` (`inventory.go:332`) likewise still calls `InventoryJoin(ctx, true)`.
- **Platinum asymmetry (OQ1) — VERIFIED.** `SetCoinTx` (`coin.go:124`) is unchanged and still rejects non-bank-toons with `ErrNotBankToon`, so a bot can't hold coin. `TotalPlatinum` (`inventory.go:385`) skips nil `Plat`. `BankRowSummary.Plat` is `*int64`, carried through `buildBanks` as `t.Plat` (nil → nil), and the UI renders nil as "not recorded" (never "0 plat") at `+page.svelte:347` and `:375`.
- **Bank-slice qty recompute (Pitfall 3) — VERIFIED.** `bankItemSearch` (`banks.ts:38`) filters to `is_bank` holders, recomputes `summed_qty` (Σ kept-holder qty) and `holder_count` (distinct kept chars) from the bank slice, and never passes through the guild-wide `r.summed_qty`/`r.holder_count`. The node test (`banks.test.ts:105`) proves the "40× guild / 3× bank" trap is closed.
- **SQL discipline — VERIFIED.** `compute/` authors zero SQL; the new predicate is a compile-time fixed string (no `fmt.Sprintf`, no name/value interpolation). The only user-bound value anywhere in the chain (`char` in `InventoryForChar`, reused by the window column) is `?`-bound.
- **Session gate + guild-wide — VERIFIED.** `GET /api/v1/banks` is registered under `webauth.RequireSession` (`main.go:378`), reads no viewer id and no query param, and has no IDOR surface.
- **XSS — VERIFIED.** Bank/item/char/slot names render via plain `{}` (auto-escaped). The only `{@html}` sink is the reused `ExaminePanel` (transitively via `InventoryWindow`) — no new raw-HTML sink introduced. The `?b=` deep-link is `encodeURIComponent`'d on write (`+page.svelte:139`).
- **V7 logging — VERIFIED.** `banks.go` logs op + err on failure and `rows` count + status on success — never a bank/item name, value, or plat figure.

Findings below are all non-blocking: one MEDIUM (a real cross-endpoint consistency gap that's observable but not corrupting), three WARN (robustness/edge-case), three INFO (advisory). Nothing here should block ship.

## Medium

### MD-01: Per-bank item count and bank-slice search qty are computed from two different endpoints with different grouping scopes — they can disagree on screen

**File:** `web/src/routes/banks/+page.svelte:74`, `internal/backendsrv/compute/banks.go:56-59`
**Issue:** The Banks tab surfaces two different "how much is in this bank" numbers sourced from two independent endpoints:

1. The bank-list row's `item_count` comes from `GET /api/v1/banks` → `compute.Banks` → `buildBanks` (`banks.go:56`), which counts **flat `InventoryJoinBanksAndBots` rows per char** (`counts[r.Char]++`). This is a raw row count: a bag and each of its `*-Slot*` children each count as 1, and the count is "number of distinct inventory_item rows," not summed stack quantity.
2. The search-mode bank-slice numbers come from `GET /api/v1/items` → `bankItemSearch` (`banks.ts:46`), which sums **stack quantities** (`s + h.qty`) grouped by **normalized item name**.

These are computed over different groupings (per-row vs. per-normalized-name) and different aggregations (row count vs. summed qty), so they are not reconcilable by a user. A bank holding 3 stacks of 20 Bone Chips reads `item_count: 3` in the list but the search shows `summed_qty: 60`. More subtly, the two endpoints derive their bank/bot membership independently: `/banks` uses `ListBankAndBotToons` (predicate on `character`), while `/items`'s `is_bank` flag comes from `RosterFor`'s per-char `IsBankToon || IsGuildBot`. They agree today, but if a char is mid-designation or the two reads race a designation write, a holder can appear in one and not the other.

This is not a correctness/data-loss defect — each number is internally correct for what it measures — but it's a quality/consistency gap a guildie will notice and read as a bug. Classifying MEDIUM (degrades trust in the valuation surface; observable; no data corruption).
**Fix:** Either (a) document the distinction in the UI (the list row's "N items" is row-count, the search shows stack qty), or (b) if a single source of truth is desired, derive the bank-list `item_count` from the same `/items` rollup the search uses (Σ bank-slice qty per bank), so the list and the search agree. If left as-is, add a code comment on `banks.go:56` noting the deliberate row-count-vs-qty distinction so a future reader doesn't "fix" one to match the other and break a test.

## Warnings

### WR-01: A search-result holder deep-link can pin a bank that is not in `banksView.banks`, leaving the detail header blank with no explanation

**File:** `web/src/routes/banks/+page.svelte:301`, `:127-129`, `:363-385`
**Issue:** Search results come from `GET /api/v1/items` (the full guild rollup, filtered client-side to `is_bank` holders), while the bank list + per-bank header come from `GET /api/v1/banks` (`ListBankAndBotToons`). These are fetched in parallel (`Promise.all`, `:74`) and are independent reads. If a holder's `char` is flagged `is_bank` in the `/items` rollup but is absent from `bv.banks` (designation race, or a char that's `is_guild_bot` in roster but filtered out of `ListBankAndBotToons` by some future divergence), clicking that holder calls `select(h.char)` → `selected` is set → `selectedBank = bankByName(...)` returns `null`. The window column then fetches inventory fine, but the D-04 detail header (`{#if selectedBank}`) silently renders nothing — no value/plat line, no "unknown bank" message. The user sees a window with no header and no indication why.
**Fix:** When `selected` is set but `selectedBank` is null after a ready load, render a minimal fallback header showing at least `{selected}` (the name is already known), e.g. relax the `{#if selectedBank}` guards at `:342` and `:368` to render `<h2>{selected}</h2>` unconditionally and only gate the value/plat `<p class="detail-meta">` on `selectedBank`. The `detail-name` already uses `{selected}` in the no-inventory branch (`:341`) but uses `{selectedBank.name}` in the ready branch (`:370`) — make them consistent.

### WR-02: `?b=` deep-link is not validated against the loaded bank set, so a stale/forged link selects a nonexistent bank and may briefly fetch garbage

**File:** `web/src/routes/banks/+page.svelte:108-112`, `:173-182`
**Issue:** `onMount` reads `?b=` and sets `selected = b` **before** `load()` resolves. The `$effect` at `:173` fires on `selected` becoming non-null and immediately calls `loadInventory(b)` for whatever string was in the URL — including a bank that no longer exists or was never valid. `load()` does clear `selected` if it's not in `bv.banks` (`:84-87`), but that clearing runs *after* the parallel `fetchInventory(b)` has already been dispatched by the effect. For a since-removed bank this fetches `/inventory/{stale}` (returns empty arrays, harmless), but the window column flashes loading→no-inventory for a bank the list-clearing logic is simultaneously trying to deselect. The ordering between the `load()` clear and the effect-driven fetch is not coordinated.
**Fix:** Gate the `$effect` window fetch on the page being ready and the selection being a known bank, e.g. `if (status === 'ready' && bankByName(banksView?.banks ?? [], sel)) void loadInventory(sel)`. Or defer applying the `?b=` selection until after `load()` confirms the bank exists (set `selected` inside `load()`'s success path rather than in `onMount` before the fetch).

### WR-03: `fmtNum` rounds bank value with `Math.round`, so a large guild total can silently lose precision / read oddly, and there is no thousands-floor for sub-1pp values

**File:** `web/src/routes/banks/+page.svelte:196-198`, `:344`, `:227`
**Issue:** `fmtNum(n)` does `Math.round(n).toLocaleString('en-US')`. The bank `value` is a `float64` sum of `pickPrice × count` (`inventory.go:372`). Rounding to a whole pp is reasonable for display, but two edge cases are unhandled: (1) a small-but-nonzero aggregate value (e.g. 0.4 pp of vendor trash) rounds to `0` and renders "0 pp", visually indistinguishable from a genuinely unpriced/empty bank — the `unpriced` count exists on `BankRowSummary` but is never surfaced in the header, so the "+N unpriced" annotation the valuation model deliberately carries (`types.go:196`) is dropped on the floor in the UI. (2) `Math.round` on a very large float is fine numerically here (guild totals won't approach `2^53`), so no overflow risk — noting only the rounding-to-zero ambiguity.
**Fix:** Surface `selectedBank.unpriced` / `banksView.guild_unpriced` next to the value (e.g. "1,234 pp · +5 unpriced") so a "0 pp" reading is disambiguated from "unpriced." The compute layer already computes the count specifically to avoid silently understating value; the UI currently discards it.

## Info

### IN-01: `buildBanks` allocates `counts` with `len(toons)` capacity but keys it by char name from `rows` — a harmless capacity mismatch

**File:** `internal/backendsrv/compute/banks.go:56`
**Issue:** `counts := make(map[string]int64, len(toons))` pre-sizes the map to the toon count, but the map is populated by iterating `rows` and keying on `r.Char`. The key set is the same (every row's char is a bank/bot toon), so the capacity hint is approximately right, but it's a hint not a guarantee — purely cosmetic. No bug.
**Fix:** None required. Advisory only.

### IN-02: `BankResponse.coin` is typed `null` (literal) in the legacy P14 contract — confirm the new Banks tab does not collide with it

**File:** `web/src/lib/api.ts:130-134`, `:364-380`
**Issue:** The new `BanksView`/`BankRowSummary` interfaces are cleanly append-only and do not redeclare or shadow the legacy `BankResponse` (`{ rows, coin: null }`) used by `/api/v1/views/bank`. The two coexist correctly. Noting only that the two "bank" payload shapes now live side by side — a future reader could confuse `BankResponse` (legacy grid) with `BanksView` (new tab). The doc comments at `:352-360` already call this out.
**Fix:** None required. The existing comments are sufficient.

### IN-03: `select(h.char)` on a search-result holder does not clear `query`, by design — the comment says so but a test would lock it

**File:** `web/src/routes/banks/+page.svelte:296-301`
**Issue:** Clicking a holder in search mode pins that bank's window while keeping the search results visible (intended per the `:296` comment). This is correct behavior, but it's DOM-only and therefore unverified by the node test suite (`banks.test.ts` covers only the pure helpers). The "selecting a holder does not clear the query" contract relies on the browser-smoke step.
**Fix:** None required for correctness. If desired, the deferred 33-03 browser-smoke should explicitly check that selecting a holder leaves the search list in place (it's the kind of interaction a future refactor could silently break).

---

_Reviewed: 2026-06-18T21:12:40Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
