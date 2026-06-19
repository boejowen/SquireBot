# Phase 33: Banks Tab + Valuation - Pattern Map

**Mapped:** 2026-06-18
**Files analyzed:** 11 (4 backend new, 3 backend modify, 2 web new, 2 web modify)
**Analogs found:** 11 / 11 (every file has an in-repo analog — this is a surfacing + composition phase, zero net-new algorithm)

> **Read first:** `33-RESEARCH.md` (the BankValuation/widen-scope wrinkle, the file-by-file plan, Pitfalls 1–6) and `33-UI-SPEC.md` (layout/states). This map pins the *exact analog + excerpt* each new/modified file mirrors. The two load-bearing facts the planner must carry from RESEARCH are encoded here: **(1) the bank-set widen** (`is_bank_toon` → `IsBankToon || IsGuildBot`) and **(2) per-bank plat is nullable** (`*int64`, nil ≠ 0).

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/backendsrv/compute/banks.go` | service (compute) | transform / CRUD-read | `internal/backendsrv/compute/inventory.go` (`BankValuationFor`+`buildBankValuation`) + `compute/itemrollup.go` (`Items`+`buildItemRollups`) | exact |
| `internal/backendsrv/compute/banks_test.go` | test | transform | `internal/backendsrv/compute/inventory_test.go` (BankValuation seed/assert) | role-match (analog not re-read — pattern named) |
| `internal/backendsrv/readapi/banks.go` | route (handler) | request-response | `internal/backendsrv/readapi/items.go` (no-viewer-shape via the early lines) / `characters.go` | exact (NO viewer id) |
| `internal/backendsrv/store/readviews.go` (MODIFY) | model (store read) | CRUD-read | itself — `InventoryJoin` bank branch (readviews.go:214-218) | exact (the branch to widen/twin) |
| `internal/backendsrv/store/coin.go` (MODIFY) | model (store read) | CRUD-read | itself — `ListBankToons` (coin.go:46-69) | exact (the coin read to widen-or-twin) |
| `cmd/squirebot-server/main.go` (MODIFY) | config (route reg) | request-response | itself — the `/api/v1/items` registration (main.go:370) | exact |
| `web/src/lib/banks.ts` | utility (pure helper) | transform | `web/src/lib/items.ts` (`filterItems`/`sortHolders`) + `roster.ts` (`viewerFirst`) | exact |
| `web/src/lib/__tests__/banks.test.ts` | test | transform | `web/src/lib/__tests__/items.test.ts` / `roster.test.ts` | role-match |
| `web/src/lib/api.ts` (MODIFY) | utility (fetch + types) | request-response | itself — `fetchItems`/`fetchCharacters` (api.ts:333-350) + `ItemRollup`/`ItemHolder` ifaces (api.ts:222-251) | exact |
| `web/src/routes/banks/+page.svelte` (MODIFY, replace placeholder) | component (page) | request-response / master-detail | `web/src/routes/inventory/+page.svelte` (list+search+holders deep-link) + `characters/+page.svelte` (window-column state machine) | exact (two analogs, one per pane) |
| `InventoryWindow.svelte` / `StateBlock.svelte` / `LastSyncedCell.svelte` (REUSE, no change) | component | — | themselves (P31) | reuse verbatim |

---

## Pattern Assignments

### `internal/backendsrv/compute/banks.go` (service/compute, transform)

**Analogs:** `compute/inventory.go` (the `BankValuationFor` → `buildBankValuation` public-fn→pure-helper split it composes) + `compute/itemrollup.go` (the `Items` → `buildItemRollups` split shape).

**Public-fn → pure-helper split to mirror** (`compute/inventory.go:331-341`):
```go
func BankValuationFor(ctx context.Context, s *store.Store) (BankValuation, error) {
	rows, err := s.InventoryJoin(ctx, true) // bankOnly — flat list incl. *-Slot* children (each is its own row)
	if err != nil {
		return BankValuation{}, err
	}
	toons, err := store.ListBankToons(ctx, s.DB())
	if err != nil {
		return BankValuation{}, err
	}
	return buildBankValuation(rows, toons), nil
}
```
> `compute.Banks(ctx, s)` mirrors this 1:1 but reads the **widened** bank+bot scope (the new store read — see store/readviews.go below) + the widened coin read, then delegates to a pure `buildBanks(rows, toons)`. RESEARCH §"Code Examples" pins the new `Banks` / `buildBanks` signatures + `BanksView` / `BankRowSummary` structs (append to types.go).

**The pure-transform body to mirror** (`compute/inventory.go:360-380` — `buildBankValuation`). This already computes BOTH the per-bank value AND the guild total in one walk; `buildBanks` wraps it and joins per-bank plat + per-bank item count:
```go
func buildBankValuation(rows []store.InventoryJoinRow, toons []store.BankToon) BankValuation {
	bv := BankValuation{PerBank: make(map[string]Valuation, len(toons))}
	for _, t := range toons {
		bv.PerBank[t.Name] = Valuation{} // ensure every live bank toon has a row (MR-02)
	}
	for _, r := range rows {
		price := pickPrice(pricesFromJoin(r)) // reuse view.go's selector + name-joined prices
		per := bv.PerBank[r.Char]
		if price == nil {
			per.UnpricedCount++
			bv.GuildTotal.UnpricedCount++
		} else {
			value := *price * float64(r.Count)
			per.TotalValue += value
			bv.GuildTotal.TotalValue += value
		}
		bv.PerBank[r.Char] = per
	}
	bv.TotalPlatinum = TotalPlatinum(toons)
	return bv
}
```
> **MR-02 carry-forward (load-bearing):** seed `PerBank`/the bank-row map from the **toon list FIRST** so a coin-only bank toon (plat entered, zero inventory rows) still gets a `0 items` row — `buildBanks` must do the same so a coin-only bank appears in the list with `0 items` (UI-SPEC §G).

**Per-bank PLATINUM join (Pattern 2 / Pitfall 2 — the D-04 gap to fill):** `Valuation` (`types.go:194-197`) carries `TotalValue`+`UnpricedCount` but **no plat**; plat lives on `store.BankToon.Plat` (`*int64`, nil ≠ 0 — see coin.go below). `buildBanks` must emit each bank's `Plat` as a **nullable** `*int64` on `BankRowSummary` — NEVER coerce nil→0.

**TotalPlatinum to reuse verbatim** (`compute/inventory.go:385-393`) — skips nil plats, never treats nil as 0:
```go
func TotalPlatinum(banks []store.BankToon) int64 {
	var sum int64
	for _, b := range banks {
		if b.Plat != nil { sum += *b.Plat }
	}
	return sum
}
```

**THE IRON LAW (copy the file-header discipline from itemrollup.go:11-21):** this file authors **ZERO SQL** and **NEVER re-selects a price** — it reuses `pickPrice`+`pricesFromJoin` over the store rows. Group/aggregate by char name; the price was already name-bridged in the store's `pp_rep` CTE (never raw `item_id`).

---

### `internal/backendsrv/compute/banks_test.go` (test, transform)

**Analog:** `internal/backendsrv/compute/inventory_test.go` (the BankValuation seed/assert pattern — referenced from RESEARCH §"File-by-File", not re-read here to save context; the planner should open it when writing the test).

**Required assertions (RESEARCH §"Verification Approach" + the Pitfalls this phase introduces):**
- Seed **a bank toon AND a guild bot** both holding priced + unpriced items + plat; assert the **guild total INCLUDES the bot** (Pitfall 1 — the regression that proves the scope widen).
- Per-bank value/plat correct; **nil-plat carried as null** (Pitfall 2), not 0.
- Unpriced count correct ("+N unpriced").
- A-Z order of `BanksView.Banks`.
- A coin-only bank toon (zero inventory rows) still emits a `0 items` row (MR-02).
> Re-run `inventory_test.go` after the store widen (regression) — confirm the legacy `BankValuationFor` path is untouched (it will be, if the planner picks Option B below).

---

### `internal/backendsrv/readapi/banks.go` (route, request-response)

**Analog:** `readapi/items.go` — but **drop the viewer id** (banks are NOT viewer-scoped; D-01 is plain A-Z). The struct/constructor shape + `[] not null` + V7 logging come from `items.go`; the "no UserFromContext" simplification is the only deviation.

**Handler struct + constructor to mirror** (`readapi/items.go:41-48`):
```go
type ItemsHandler struct {
	store *store.Store
}
func NewItems(s *store.Store) *ItemsHandler {
	return &ItemsHandler{store: s}
}
```

**ServeHTTP to mirror** (`readapi/items.go:54-83`) — the GET-only guard, the compute call, the `[] not null` coercion, and the V7 slog (count + status, NEVER a name/value):
```go
func (h *ItemsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	uid, _ := webauth.UserFromContext(ctx)   // ← banks.go DROPS this line (no viewer scope, D-01)
	rows, err := compute.Items(ctx, h.store, uid)
	if err != nil {
		slog.Error("items read failed", "err", err) // V7: op + err only
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	out := rows
	if out == nil {
		out = []compute.ItemRollup{} // [] not null — a stable client shape
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(out); err != nil { ... }
	slog.Info("items ok", "rows", len(out), "status", http.StatusOK) // V7: count + status only
}
```
> For banks: `bv, err := compute.Banks(r.Context(), h.store)`; coerce `bv.Banks` to `[]compute.BankRowSummary{}` when nil; encode the whole `BanksView` object (NOT a bare array — it carries the guild summary alongside the rows). RESEARCH §"Code Examples" pins the exact `BanksHandler.ServeHTTP`.

**Security/doc-comment posture to copy from `items.go:1-24`:** RequireSession-gated (login-only, NOT officer); guild-wide read; V5 (no user input server-side — banks takes no query param, no viewer id); V7 (count+status only).

---

### `internal/backendsrv/store/readviews.go` (MODIFY — widen the bank scope)

**Analog:** itself. The exact branch (`readviews.go:214-218`):
```go
	// Fixed-string WHERE switch on the bool (NOT a value interpolation).
	query := base + orderBy
	if bankOnly {
		query = base + ` AND c.is_bank_toon = 1` + orderBy
	}
```

**BLAST-RADIUS FINDING (resolves RESEARCH Open Q2 / A3 — the planner must NOT pick Option A blind):**
`InventoryJoin(ctx, true)` has **TWO callers**, not one:
- `compute/inventory.go:332` — `BankValuationFor` (this phase wants it widened).
- `compute/bank.go:35` — `compute.Bank`, the **legacy consolidated `bank` grid** served at `GET /api/v1/views/bank` (`main.go:290`). Its doc comment (`bank.go:27`) explicitly pins `WHERE c.is_bank_toon = 1`.

> **Therefore Option A (widening the shared `bankOnly` branch) WOULD silently change the legacy `bank` grid to include bots.** Default to **Option B**: add a dedicated read — `InventoryJoinBanksAndBots(ctx)` (or a 3-value scope enum) — consumed ONLY by `compute.Banks`, leaving the `bankOnly` branch untouched. This is the researcher's recommendation when a second caller exists, now confirmed.

**The widened WHERE to write (Option B — a new fixed-string branch/literal, the SAME no-interpolation discipline):**
```go
// New read OR new branch — fixed string, NEVER a value interpolation:
   ... ` AND (c.is_bank_toon = 1 OR c.is_guild_bot = 1)` + orderBy
```
The whole `InventoryJoin` body (the `pp_rep` name-join CTE at `readviews.go:195-212`, the scan loop) is reused verbatim — only the WHERE predicate differs. Never concat a name into SQL.

---

### `internal/backendsrv/store/coin.go` (MODIFY — the platinum scope decision)

**Analog:** itself — `ListBankToons` (`coin.go:46-69`):
```go
func ListBankToons(ctx context.Context, db *sql.DB) ([]BankToon, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, name, plat, gold, silver, copper
		   FROM character
		  WHERE is_bank_toon = 1 AND is_removed = 0
		  ORDER BY name COLLATE NOCASE`)
	...
}
```

**The `BankToon.Plat` nullable to carry through to D-04** (`coin.go:33-40`) — `*int64`, nil ≠ 0:
```go
type BankToon struct {
	CharacterID int64  `json:"character_id"`
	Name        string `json:"name"`
	Plat        *int64 `json:"plat"`   // ← nil = never entered; do NOT coerce to 0 (Pitfall 2)
	...
}
```

**The value/plat ASYMMETRY the plan must document (RESEARCH Pitfall 1 / Open Q1):** coin is **bank-toon-gated at the store** — `SetCoinTx` rejects a non-bank-toon write with `ErrNotBankToon` (`coin.go:89-110`), so a guild *bot* cannot hold plat today. Therefore:
- **Item VALUE** = bank + bot (use the widened `InventoryJoin` read above).
- **PLATINUM** may legitimately stay **bank-toons-only** (`ListBankToons` unchanged) because a bot can't have coin.
> Either (a) keep `ListBankToons` for coin and widen ONLY the item read, OR (b) add a `ListBankAndBotToons` (`is_bank_toon = 1 OR is_guild_bot = 1`) if the plan also wants bot toons listed with `plat: null`. **The list of bank ROWS must include bots** (D-01) — so the toon list `compute.Banks` iterates is the widened set; bots just carry `Plat == nil`. Flag the bot-coin question for the user (RESEARCH Open Q1) — if they want bot plat *enterable*, that's a bigger `SetCoinTx` gate change, out of this phase's recommended scope.

---

### `cmd/squirebot-server/main.go` (MODIFY — one-line route registration)

**Analog:** itself — the `/api/v1/items` registration (`main.go:370`), sitting beside `/characters` (`main.go:363`):
```go
	mux.Handle("GET /api/v1/inventory/{char}", webauth.RequireSession(db, readapi.NewInventory(st)))
	mux.Handle("GET /api/v1/characters", webauth.RequireSession(db, readapi.NewCharacters(st)))
	...
	mux.Handle("GET /api/v1/items", webauth.RequireSession(db, readapi.NewItems(st)))
```

**Add (RESEARCH §"Code Examples"):**
```go
	mux.Handle("GET /api/v1/banks", webauth.RequireSession(db, readapi.NewBanks(st))) // Phase 33 / BANK-01/02
```
> **Route-collision check (CONFIRMED clean):** `/api/v1/banks` is distinct from `/api/v1/views/bank` (main.go:290, the legacy grid) and `/api/v1/coin/bank-toons` (main.go:318, the coin form). Go 1.22+ ServeMux treats them as separate patterns. Do NOT reuse `NewViews(st,"bank")`.

---

### `web/src/lib/banks.ts` (utility, transform)

**Analogs:** `web/src/lib/items.ts` (`filterItems`/`sortHolders`) + `web/src/lib/roster.ts` (`viewerFirst`).

**The name-filter shape to mirror — but A-Z, NOT viewer-first** (`items.ts:29-36`):
```ts
export function filterItems(rows: ItemRollup[], query: string): ItemRollup[] {
	const q = query.trim().toLowerCase();
	const matched = q === '' ? rows : rows.filter((r) => r.name.toLowerCase().includes(q));
	return viewerFirstItems(matched);
}
```
> `banks.ts` `bankItemSearch(rows, query)` mirrors the trim/`includes` filter, but **A-Z** (banks aren't viewer-scoped, D-01) — so replace `viewerFirstItems(...)` with a plain `localeCompare` A-Z sort. The A-Z sort itself mirrors the `localeCompare` clause in `roster.ts:36-41` / `items.ts:21-27`, dropping the band/`is_mine` key.

**The `is_bank` holder filter (Pattern 3 / Pitfall 3 — the ONE new web algorithm):** the P32 `ItemHolder` already carries `is_bank` (the flag set in `compute/itemrollup.go:88,99` = `IsBankToon || IsGuildBot`). `bankItemSearch` must:
1. map each `ItemRollup` to ONLY its `holders[]` where `is_bank === true`;
2. drop items left with zero bank holders;
3. **recompute** the item's displayed `summed_qty` (Σ kept-holder qty) and `holder_count` (distinct kept chars) from the bank slice — do NOT pass through `ItemRollup.summed_qty`/`holder_count`, which are **guild-wide** (`itemrollup.go:104-109`) and would leak non-bank holdings (Pitfall 3, the "Blue Diamond 40× guild / 3× bank" trap);
4. name-filter + A-Z.

**The holder-banding helper to adapt** (`items.ts:42-47` — `sortHolders`). For banks, the band collapses (all holders are bank holders), so this reduces to a plain A-Z `localeCompare` on `char`:
```ts
export function sortHolders(holders: ItemHolder[]): ItemHolder[] {
	const band = (h: ItemHolder) => (h.is_mine ? 0 : h.is_bank ? 2 : 1);
	return [...holders].sort(
		(a, b) => band(a) - band(b) || a.char.localeCompare(b.char, undefined, { sensitivity: 'base' })
	);
}
```

**Also add `bankByName(banks, name)`** — a trivial `.find` so the D-04 detail header reads its per-bank value/plat off the already-loaded list with no second fetch (Pattern 2).

**File-header discipline to copy** (`items.ts:1-15` / `roster.ts:1-15`): DOM-free, immutable (return NEW arrays), node-testable; presentation only, NEVER access control (`is_bank` is server-stamped — the client never recomputes designation).

---

### `web/src/lib/__tests__/banks.test.ts` (test, transform)

**Analog:** `web/src/lib/__tests__/items.test.ts` / `roster.test.ts` (named in RESEARCH §"File-by-File"; not re-read here). Node cases: A-Z `sortBanksAZ`; `bankItemSearch` drops zero-bank-holder items + keeps only `is_bank` holders + **recomputes** the bank-slice qty/holder count; `bankByName` lookup; an empty/no-match query returns the right shape. (DOM-blind — `banks.test.ts` covers the pure helper ONLY; the rendered tab is a browser-smoke gap, Pitfall 5.)

---

### `web/src/lib/api.ts` (MODIFY — `fetchBanks()` + the contract interfaces)

**Analog:** itself. The `fetchItems`/`fetchCharacters`/`fetchInventory` wrappers (`api.ts:333-350`):
```ts
export function fetchCharacters(f: typeof fetch = fetch): Promise<RosterCharacter[]> {
	return getJSON<RosterCharacter[]>('/api/v1/characters', f);
}
export function fetchInventory(char: string, f: typeof fetch = fetch): Promise<CharacterInventory> {
	return getJSON<CharacterInventory>(`/api/v1/inventory/${encodeURIComponent(char)}`, f);
}
export function fetchItems(f: typeof fetch = fetch): Promise<ItemRollup[]> {
	return getJSON<ItemRollup[]>('/api/v1/items', f);
}
```
**Add `fetchBanks()`** (RESEARCH §"Code Examples"):
```ts
export function fetchBanks(f: typeof fetch = fetch): Promise<BanksView> {
	return getJSON<BanksView>('/api/v1/banks', f);
}
```
> Note `/api/v1/banks` returns an **object** (`BanksView`), not a bare array — so the `getJSON<BanksView>` generic differs from `fetchItems`'s `<ItemRollup[]>`; no `[] not null` array concern at the wrapper (the array is the nested `banks` field, coerced server-side per readapi/banks.go).

**The interface shape to mirror** (`api.ts:222-251` — `ItemHolder`/`ItemRollup`, snake_case, append-only). Add `BanksView` + `BankRowSummary` mirroring the Go contract; `plat: number | null` (the `*int64` → nullable, matching the `RosterCharacter.last_seen ""`-vs-value and the coin nullable discipline):
```ts
export interface ItemHolder {
	char: string;
	slot_label: string;
	qty: number;
	last_synced: string;
	is_mine: boolean;
	is_bank: boolean;   // ← the flag banks.ts filters on (already shipped, P32)
}
```
> **REUSE `ItemRollup`/`ItemHolder` verbatim for the BANK-03 search** — do NOT redeclare them; `bankItemSearch` consumes the existing P32 types. Only `BanksView`/`BankRowSummary` are new.

---

### `web/src/routes/banks/+page.svelte` (MODIFY — replace the P30 placeholder)

This page has TWO analogs, one per pane. The current file is a dashed "coming soon" placeholder (`banks/+page.svelte:1-69`) to be fully replaced.

**Analog A — the list + scoped search + holders deep-link: `inventory/+page.svelte` (verbatim shape).** Copy the master-detail scaffold: `onMount` one-shot load + `?`-param preselect, the `load()`/`refetch()` + 401/403→`authGuard` machine, the search box, the bespoke selectable row list, the no-results `StateBlock`, the holder deep-link rows.

The `onMount` preselect + load (`inventory/+page.svelte:81-85`) — for banks use `?b=` (UI-SPEC §A allows `?b=`; `?c=` also fine per RESEARCH §"Code Examples"):
```ts
onMount(() => {
	const i = new URLSearchParams(window.location.search).get('i');
	if (i) selected = i;
	void load();
});
```

The `load()` + AuthGate machine to mirror (`inventory/+page.svelte:51-72`):
```ts
async function load() {
	status = 'loading';
	try {
		items = await fetchItems();
		const sel = selected;
		if (sel && !items.some((r) => r.name === sel)) selected = null;
		status = 'ready';
	} catch (err) {
		if (authGuard && (err instanceof Unauthenticated || err instanceof Forbidden)) authGuard(err);
		else status = 'error';
	}
}
```
> Banks loads `fetchBanks()` for the list/summary AND `fetchItems()` (once, for the BANK-03 search) — mirror this same try/AuthGate shape for each.

The bespoke row + `select()` + URL-reflect (`inventory/+page.svelte:198-247` for the row markup, `127-138` for `select`). The whole `.row`/`.search`/`.detail`/`.prompt` CSS block (`inventory/+page.svelte:359-739`) is the verbatim style source — but **drop the 32px icon tile** on bank-list rows (UI-SPEC §Spacing: "No icon tiles in the bank list" — name + item count only). Add the `bank`/`bot` `.tag` (the `is_mine?"you"` tag at `inventory:223` / `340` is the exact tag idiom — swap "you" for "bank"/"bot").

The holder deep-link — **stays in-tab** (`/banks?b=` not `/characters?c=`). The P32 holder row (`inventory/+page.svelte:329-349`) navigates cross-tab via `<a href="/characters?c=...">`; for banks, RESEARCH §"Code Examples" + UI-SPEC §F.3 resolve it to an **in-tab `select(bankName)`** (a `<button>` or `<a href="/banks?b=...">` calling the same `select` the list rows call) so clicking a holder PINS that bank's window without a route change:
```svelte
<!-- P32 cross-tab (inventory:330-335) → banks in-tab: -->
<a href={`/banks?c=${encodeURIComponent(h.char)}`} aria-label={`View ${h.char}`}>{h.char}</a>
```

**Analog B — the per-bank detail window column: `characters/+page.svelte` (the window-column state machine, verbatim).** The D-04 detail = a per-bank header + the reused `InventoryWindow`. Copy the `winStatus`/`inv`/`invFor` machine, `loadInventory` (with the stale-response guard), and the `$effect` selection driver — it is already correct (handles the in-flight-selection-change drop + 401/403).

`loadInventory` with the stale-drop (`characters/+page.svelte:146-163`):
```ts
async function loadInventory(char: string) {
	winStatus = 'loading';
	invFor = char;
	try {
		const got = await fetchInventory(char);
		if (invFor !== char) return; // drop a stale response (user picked another bank mid-flight)
		inv = got;
		winStatus = 'ready';
	} catch (err) {
		if (invFor !== char) return;
		if (authGuard && (err instanceof Unauthenticated || err instanceof Forbidden)) authGuard(err);
		else winStatus = 'error';
	}
}
```

The `$effect` selection driver (`characters/+page.svelte:171-180`) + `noInventory` derived (`184-190`) — reuse verbatim.

The window-column markup to mirror (`characters/+page.svelte:256-279`) — the per-bank D-04 header goes ABOVE the `<InventoryWindow>`:
```svelte
<div class="window-col">
	{#if selected === null}
		<div class="prompt">
			<h2 class="prompt-heading">Pick a character</h2>   <!-- banks: "Pick a bank" -->
			<p class="prompt-body">Choose a character from the list to see their gear and bags.</p>
		</div>
	{:else if winStatus === 'loading'}
		<StateBlock kind="loading" />
	{:else if winStatus === 'error'}
		<StateBlock kind="error" onRetry={retryInventory} />
	{:else if noInventory}
		<div class="prompt">
			<h2 class="prompt-heading">No inventory synced yet</h2>
			<p class="prompt-body">{selected} hasn't uploaded inventory yet. ...</p>
		</div>
	{:else if inv !== null}
		<!-- banks: insert the D-04 per-bank header HERE, above the window:
		     {bankByName(selected).name} — {value} pp · {plat} plat  -->
		<InventoryWindow inventory={inv} />
	{/if}
</div>
```
> The D-04 per-bank header reads its `value`/`plat` from `bankByName(selected)` off the already-loaded `BanksView.banks` (Pattern 2 — no second fetch). Use the `.detail-name` (20px accent) + `.detail-meta` idiom from `inventory/+page.svelte:572-588`. Render nil plat as "not recorded"/"— plat", never "0 plat" (Pitfall 2). (Note: 33-UI-SPEC §Typography says the per-bank value/plat are real sums → `0 pp`/`0 plat` clean; but the *platinum* is nullable per the Go `*int64` — the planner reconciles: a recorded-0 reads `0 plat`, a never-recorded reads "not recorded". Confirm the exact string in planning.)

**The summary header (D-02) is the ONE net-new piece** — no exact analog; build it from the UI-SPEC §B token recipe (a `--panel` band, eyebrow "GUILD BANKS" + the value/plat number line). The number/unit treatment mirrors the `.num`/`.unit` idiom (`inventory/+page.svelte:478-490`).

**Security carry-forward (copy the `inventory/+page.svelte:18-23` header note):** bank + item + char names render via plain `{}` (Svelte auto-escapes); the ONLY raw-HTML sink is `ExaminePanel`'s `composeItemNote` INSIDE the reused window. Add NO new `{@html}`. `encodeURIComponent` any name that round-trips through `?b=`.

---

### Reused components (NO change — mount verbatim)

| Component | Source | How Phase 33 uses it |
|-----------|--------|----------------------|
| `InventoryWindow.svelte` | P31 (`web/src/lib/components/InventoryWindow.svelte`) | `<InventoryWindow inventory={bankInv} />` — its header comment (lines 1-8) explicitly names "**Phase 33 reuses it per bank toon**". Generic, prop-driven over one `CharacterInventory`. Zero change. |
| `StateBlock.svelte` | shared | loading / error / empty / no-results — reuse the kinds the analogs pass (`kind="loading|error|empty|no-results"`). A `no-bank-toons` kind may already exist (P15) — UI-SPEC §H/Copywriting. |
| `LastSyncedCell.svelte` | `web/src/lib/components/cells/LastSyncedCell.svelte` | the per-holder "last synced" cell in the item-search results — imported exactly as `inventory/+page.svelte:29,347` does. |
| `ExaminePanel.svelte` | P31 (transitively, inside `InventoryWindow`) | the per-item examine + the single escaped `composeItemNote` sink. Not imported directly by the page. |

---

## Shared Patterns

### Session-gated read route (NOT officer)
**Source:** `readapi/items.go:36-40` (the doc comment) + `cmd/squirebot-server/main.go:370` (the registration).
**Apply to:** `readapi/banks.go` + its `main.go` line.
```go
// mux.Handle("GET /api/v1/banks", webauth.RequireSession(db, readapi.NewBanks(st)))
```
Membership gate, never ownership/officer — every signed-in member sees every bank (V4). `/api/v1/items` (reused for search) is already session-gated.

### `[] not null` + V7 logging
**Source:** `readapi/items.go:71-82` / `characters.go:81-102`.
**Apply to:** `readapi/banks.go`.
- Coerce an empty slice to `[]compute.BankRowSummary{}` before encoding (stable client shape).
- `slog.Info("banks ok", "rows", len(...), "status", 200)` — count + status ONLY; NEVER a bank name / value / platinum.

### Compute-on-read, ZERO SQL in `compute/`, name-join never item_id
**Source:** `compute/itemrollup.go:11-21` (THE IRON LAW header) + `inventory.go:13-16`.
**Apply to:** `compute/banks.go`.
The price is already bridged by NORMALIZED NAME in the store's `pp_rep` CTE (`readviews.go:195-209`). `compute.Banks` reuses `pickPrice`+`pricesFromJoin`; it NEVER re-selects a price and NEVER joins catalog↔inventory by raw `item_id` (the canonical landmine, memory `pigparse-vs-ingame-item-id-namespaces`).

### Nullable coin discipline (`*int64`, nil ≠ 0)
**Source:** `store/coin.go:30-40` (`BankToon.Plat *int64`) + `nullableToPtr` (`coin.go:143-149`).
**Apply to:** `compute.BankRowSummary.Plat` (Go) + `BankRowSummary.plat: number | null` (TS) + the D-04 detail header render.
Never coerce a never-entered plat to 0 (Pitfall 2).

### Pure, node-testable, DOM-free web helpers
**Source:** `web/src/lib/items.ts:1-15` + `roster.ts:1-15` (the header rationale) + `vite.config.ts` (node `server` project excludes `*.svelte`).
**Apply to:** `web/src/lib/banks.ts` + `__tests__/banks.test.ts`.
Return NEW arrays (immutable); presentation only (`is_bank` is server-stamped); the rendered DOM stays a browser-smoke gap (Pitfall 5 — `npm run dev` can't auth against prod; deploy-then-smoke).

### Master-detail selection seam (`?param` + `history.replaceState`, single reusable detail)
**Source:** `inventory/+page.svelte:127-138` (`select` + URL-reflect) + `characters/+page.svelte:88-92` (`onMount` preselect) + `:171-180` (`$effect` driver).
**Apply to:** `banks/+page.svelte`.
ONE reusable detail rendered on selection (the relaxed consolidated-views rule, ratified 2026-06-17) — NOT N persistent routes. The Banks holder deep-link stays IN-TAB (`select(bankName)`), unlike the P32 cross-tab `/characters?c=` jump.

### Auto-escaped names; single `{@html}` sink
**Source:** `inventory/+page.svelte:18-23` + `characters/+page.svelte:18-21`.
**Apply to:** `banks/+page.svelte`.
All guildie-controlled names (bank, item, char, slot) via plain `{}`. The ONLY raw-HTML is `composeItemNote` inside the reused `ExaminePanel`. Add NO new `{@html}`. `encodeURIComponent` any name round-tripping a URL/`?b=`.

---

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| (the D-02 valuation **summary header** markup inside `banks/+page.svelte`) | component (sub-block) | — | No existing tab renders a guild-wide value/platinum banner. Build from the UI-SPEC §B token recipe (`--panel` band + eyebrow + number line). The number/unit *styling* still mirrors `inventory/+page.svelte`'s `.num`/`.unit` (`:478-490`) and the eyebrow mirrors the holders `.holders-eyebrow` (`:603-611`) — so even this has token-level precedent; only the composition is new. |

Everything else has a direct in-repo analog (this is a surfacing + composition phase — RESEARCH "Phase 33 writes almost no new algorithm").

---

## Metadata

**Analog search scope:** `internal/backendsrv/{compute,store,readapi}`, `cmd/squirebot-server/main.go`, `web/src/lib/{api.ts,items.ts,roster.ts}`, `web/src/lib/components/`, `web/src/routes/{inventory,characters,banks}/+page.svelte`.
**Files scanned (read this session):** `compute/itemrollup.go`, `compute/inventory.go`, `compute/types.go` (structs), `compute/bank.go` (blast-radius), `store/coin.go`, `store/readviews.go` (InventoryJoin + RosterFor), `readapi/items.go`, `readapi/characters.go`, `main.go` (route block), `web/src/lib/{items.ts,roster.ts,api.ts}`, `web/src/routes/{inventory,characters,banks}/+page.svelte`, `InventoryWindow.svelte` (header). Analog test files (`inventory_test.go`, `items.test.ts`, `roster.test.ts`) named from RESEARCH, not re-read.
**Key cross-file finding:** `InventoryJoin(ctx, true)` has TWO callers (`BankValuationFor` + the legacy `compute.Bank` grid) → planner should prefer **Option B** (a dedicated bank+bot read), not Option A (widen the shared branch). This resolves RESEARCH Open Q2/A3.
**Pattern extraction date:** 2026-06-18

---

## PATTERN MAPPING COMPLETE

**Phase:** 33 - Banks Tab + Valuation
**Files classified:** 11
**Analogs found:** 11 / 11

### Coverage
- Files with exact analog: 9 (compute/banks.go, readapi/banks.go, both store modifies, main.go, banks.ts, api.ts, banks/+page.svelte, + the reused components)
- Files with role-match analog: 2 (banks_test.go, banks.test.ts — analog test files named, not re-read)
- Files with no analog: 0 net-new files (only the in-page D-02 summary-header sub-block has no whole-file analog; built from token-level precedent)

### Key Patterns Identified
- **Backend = surface, don't recompute:** `compute.Banks` mirrors the `BankValuationFor`→`buildBankValuation` public-fn→pure-helper split and reuses `pickPrice`/`pricesFromJoin`/`TotalPlatinum` over a WIDENED bank+bot read; the route is the `items.go` twin minus the viewer id.
- **The bank-set widen is Option B, not A:** `InventoryJoin(ctx, true)` has two callers (BankValuationFor + the legacy `/views/bank` grid), so a dedicated `InventoryJoinBanksAndBots` read avoids changing the legacy grid. Value = bank+bot; platinum stays bank-toon-gated (the coin write-gate). Plat is nullable (`*int64`, nil ≠ 0).
- **Web = master-detail mirror of two shipped pages:** the list/search/holders come verbatim from `inventory/+page.svelte`; the per-bank window column comes verbatim from `characters/+page.svelte` (the `winStatus`/stale-drop machine); the only new web algorithm is `banks.ts`'s `is_bank` holder filter (recompute the bank-slice qty — Pitfall 3); the reused `InventoryWindow` is named in its own header as a Phase 33 consumer.

### File Created
`.planning/phases/33-banks-tab-valuation/33-PATTERNS.md`

### Ready for Planning
Pattern mapping complete. The planner can reference each analog + the line-numbered excerpts directly in the PLAN.md action sections. Two decisions are pre-resolved with evidence: Option B for the store widen (two `InventoryJoin(ctx,true)` callers), and the value/platinum asymmetry (RESEARCH Open Q1 still needs a user call on whether bot plat should be *enterable*).
