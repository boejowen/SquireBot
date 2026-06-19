# Phase 32: Inventory Tab (Item-Centric) - Pattern Map

**Mapped:** 2026-06-18
**Files analyzed:** 9 (4 new backend + 1 modified backend + 2 new web + 2 modified web; 1 optional)
**Analogs found:** 9 / 9 (every new/modified file has an exact in-repo analog — this is a composition phase)

This map pins the REAL signatures each new file must mirror. Every excerpt below was read from the
live tree this session; line numbers are accurate as of 2026-06-18. The phase introduces exactly ONE
new algorithm (group inventory instances by normalized name into per-item rollups with per-holder
detail); everything else is wiring already-shipped, code-read parts.

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/backendsrv/compute/itemrollup.go` | service (compute-on-read) | transform / aggregate | `compute/view.go` (`View`/`buildViewRows`) + `compute/inventory.go` (`StructuredInventory`/`classifySlot`) | exact (public-fn → pure-helper split) |
| `internal/backendsrv/compute/itemrollup_test.go` | test | batch | `compute/view_test.go` + `compute/fixtures_test.go` | exact |
| `internal/backendsrv/readapi/items.go` | route (handler) | request-response | `readapi/characters.go` (viewer-id-from-ctx twin) | exact |
| `internal/backendsrv/store/*` (icon/stats map — OPTIONAL, Pattern-1 option b) | model (store read) | CRUD (read) | `store/readviews.go` (`CharFreshness` simple full-table SELECT) | role-match |
| `cmd/squirebot-server/main.go` (1 line) | config (route registration) | request-response | `main.go:362-363` (P31 route lines) | exact |
| `web/src/lib/items.ts` | utility (pure helper) | transform | `web/src/lib/roster.ts` (`viewerFirst`/`filterRoster`) | exact |
| `web/src/lib/__tests__/items.test.ts` | test | batch | `web/src/lib/__tests__/roster.test.ts` | exact |
| `web/src/lib/api.ts` (interfaces + `fetchItems()`) | utility (fetch wrapper) | request-response | `fetchCharacters()` (api.ts:289-293) + `RosterCharacter`/`InventorySlot` interfaces | exact |
| `web/src/routes/inventory/+page.svelte` (replace placeholder) | component (page) | request-response | `web/src/routes/characters/+page.svelte` (master-detail + `?c=` seam) | exact |
| `web/src/lib/components/PaperdollSlot.svelte` (OPTIONAL extract) | component | event-driven | `PaperdollSlot.svelte` `.ico` tile mechanic | self (extract a shared tile) |

---

## Pattern Assignments

### `internal/backendsrv/compute/itemrollup.go` (service, transform/aggregate)

**Analog:** `internal/backendsrv/compute/view.go` (public-fn → pure-helper split) + `inventory.go` (slot labels)

**Package-doc + import pattern** (`view.go:1-23`, `inventory.go:18-26`) — `compute` authors ZERO SQL;
imports only `context` + `store`. State the iron law in the package doc:
```go
// itemrollup.go groups View's per-instance rows by NORMALIZED NAME into one-row-per-item
// rollups (D-01), with per-holder detail (ITEM-03). The public Items(...) fetches via the
// store, then delegates to a pure buildItemRollups(...) that takes typed slices and returns
// the model with no ctx/store inside — directly table-testable (the view.go split). Group by
// lower(trim(name)), NEVER item_id (the EQ-inventory vs PigParse-catalog id-namespace split).
import (
    "context"
    "strings"

    "github.com/boejowen/SquireBot/internal/backendsrv/store"
)
```

**Public-fn → pure-helper split** (MIRROR `view.go:47-57` `View`, and `inventory.go:118-124`
`StructuredInventory`). RESEARCH §Pattern 1 recommends composing `compute.View` (reuses `pickPrice`
+ `WikiURL`/`Prices`/`IsQuestItem`/`LastSynced` for free) + a small `item_master` icon/stats map
(option b — leaves the shared `InventoryJoin`/`View`/`Bank` query untouched):
```go
// Source: internal/backendsrv/compute/view.go:47-57 — the exact public-fn shape.
func Items(ctx context.Context, s *store.Store, viewerDiscordID string) ([]ItemRollup, error) {
    viewRows, err := View(ctx, s) // reuses pickPrice + inline enrichment (view.go)
    if err != nil {
        return nil, err
    }
    roster, err := s.RosterFor(ctx, viewerDiscordID) // is_mine / bank / bot flags (readviews.go:682)
    if err != nil {
        return nil, err
    }
    iconStats, err := s.ItemMasterIconStats(ctx) // OPTIONAL Pattern-1 option (b) — see store read below
    if err != nil {
        return nil, err
    }
    return buildItemRollups(viewRows, roster, iconStats), nil
}
```

**Pure transform — group-by-name + flag join** (MIRROR `view.go:62-82` `buildViewRows` loop +
`inventory.go:360-380` `buildBankValuation` which already joins a per-char map by `r.Char`). Build a
`char → RosterRow` flags map first, then loop `ViewRow`s into a `map[normName]*ItemRollup`:
```go
// MIRROR view.go:62-82 (the pure loop) + inventory.go:365-377 (per-char map join by name).
func buildItemRollups(viewRows []ViewRow, roster []store.RosterRow, iconStats map[int64]IconStats) []ItemRollup {
    flags := make(map[string]store.RosterRow, len(roster))
    for _, r := range roster {
        flags[r.Name] = r // join holders → flags by char NAME (RosterRow has no id on ViewRow)
    }
    byName := make(map[string]*ItemRollup)
    for _, vr := range viewRows {
        key := strings.ToLower(strings.TrimSpace(vr.Item)) // GROUP BY NORMALIZED NAME — never vr.ID
        roll := byName[key]
        if roll == nil {
            roll = &ItemRollup{Name: vr.Item /* first-seen casing */, ...}
            byName[key] = roll
        }
        roll.SummedQty += vr.Count
        // holder row + is_mine/is_bank from flags[vr.Char]; slot_label from classifySlot(vr.Slot)
    }
    // ... distinct holder count, representative price/wiki/icon, then collect to a slice
}
```

**Per-holder slot label** (REUSE `inventory.go:60-88` `classifySlot` + `inventory.go:311-321`
`splitChild` — both already exported WITHIN the package, callable directly):
```go
// Source: internal/backendsrv/compute/inventory.go:60-88 (classifySlot) + 311-321 (splitChild).
cat, canonical := classifySlot(vr.Slot)   // vr.Slot is the raw Location, e.g. "General4-Slot1", "Chest", "Bank2"
_, isChild := splitChild(vr.Slot)          // true for a "*-Slot<N>" bagged copy
// map (cat=SlotEquipment|SlotGeneral|SlotBank, canonical, isChild) → the UI-SPEC §F label:
//   SlotEquipment        → "Worn · {canonical}"   (e.g. "Worn · Back")
//   SlotGeneral !isChild → "General · {canonical}" or "General · Slot {N}"
//   SlotBank             → "Bank" / "Bank · Slot {N}"
//   isChild (bagged)     → "Bag · Slot {N}" or just "Bag"  (A2: the parent bag NAME is NOT on ViewRow)
```
> Slot taxonomy lives in `compute/slotconst.go` (`SlotEquipment`/`SlotGeneral`/`SlotBank` consts).
> The literal `Bag · {bag name}` label needs the parent row's item name (not joined here) — RESEARCH
> Open Q1 / A2: ship `Bag · Slot {N}` unless the planner adds the within-char parent lookup.

**Price selection** — DO NOT re-implement. `View` already calls `pickPrice` (`view.go:120-130`) per row;
`ViewRow.Price` (`*float64`, nil when unpriced) and `ViewRow.Prices` (`[]PriceDetail`) are already
populated. The rollup copies the representative row's `Price`/`Prices` onto `ItemRollup` — never a new
price lookup (RESEARCH "Don't Hand-Roll": the `pp_rep` name-bridge CTE in the store is the only correct
path; the id-join leaves ~91% unpriced).

**The struct contract** (append to `compute/types.go`, snake_case — the FIXED cross-plan JSON contract
documented at `types.go:13-56`). RESEARCH pinned shape:
```go
// ItemRollup — one guild-wide item (grouped by normalized name, D-01). snake_case JSON tags.
type ItemRollup struct {
    Name        string         `json:"name"`         // representative display name (first-seen casing)
    SummedQty   int64          `json:"summed_qty"`   // Σ Count across all holdings (D-01/D-04)
    HolderCount int64          `json:"holder_count"` // distinct holding characters (D-04)
    IsMine      bool           `json:"is_mine"`      // any holder on a viewer-assigned char (D-02/ITEM-02)
    Price       *float64       `json:"price"`        // pickPrice; null when unpriced (D-04/D-09)
    Prices      []PriceDetail  `json:"prices"`       // raw WTS/WTB detail (examine)
    WikiURL     string         `json:"wiki_url"`
    WikiSummary string         `json:"wiki_summary"`
    IsQuestItem bool           `json:"is_quest_item"`
    IconID      int64          `json:"icon_id"`      // 0 → colored-tile fallback (D-02)
    Statsblock  string         `json:"statsblock"`   // "" → examine omits the stats line (D-09)
    Holders     []ItemHolder   `json:"holders"`      // one per holding (ITEM-03)
}
type ItemHolder struct {
    Char       string `json:"char"`
    SlotLabel  string `json:"slot_label"`  // from classifySlot (P29)
    Qty        int64  `json:"qty"`
    LastSynced string `json:"last_synced"` // ViewRow.LastSynced (= character.last_seen)
    IsMine     bool   `json:"is_mine"`
    IsBank     bool   `json:"is_bank"`     // is_bank_toon || is_guild_bot
}
```
Reuse the EXISTING `compute.PriceDetail` (`types.go:65-69`) — do not redeclare it.

---

### `internal/backendsrv/store/*` — `ItemMasterIconStats(ctx)` (model, CRUD read — OPTIONAL, Pattern-1 option b)

**Analog:** `store/readviews.go:621-648` `CharFreshness` (a simple full-table SELECT → slice, no `?` bind)

`ViewRow` does NOT carry `icon_id`/`statsblock` (verified `InventoryJoinRow`, `readviews.go:48-64` —
those live only on `InventoryRow`, `readviews.go:73-92`). RESEARCH recommends a tiny new read keyed by
the representative item id. The id-join is correct HERE (`item_master` is the watcher's own EQ namespace
— Pitfall 3/1):
```go
// MIRROR store/readviews.go:621-648 (CharFreshness) — a full-table SELECT into a map.
type IconStats struct { IconID int64; Statsblock string }
func (s *Store) ItemMasterIconStats(ctx context.Context) (map[int64]IconStats, error) {
    rows, err := s.db.QueryContext(ctx, `SELECT item_id, icon_id, statsblock FROM item_master`)
    // scan icon_id (sql.NullInt64 → 0 when NULL), statsblock (sql.NullString → "") per the
    // nullable-scan idiom at readviews.go:314-340; key by item_id; `?`-free (no untrusted input).
}
```
The `icon_id`/`statsblock` columns already exist (P31 migrations 00012/00013, schema v13) — NO new
migration this phase. If the planner instead picks option (a) — widen the shared `InventoryJoin`
SELECT + `ViewRow` — accept the `View`/`Bank`/`gear_check` blast radius (RESEARCH Alternatives flags it;
option b is recommended).

---

### `internal/backendsrv/readapi/items.go` (route, request-response)

**Analog:** `internal/backendsrv/readapi/characters.go` (the viewer-id-from-ctx twin — NOT
`inventory.go`, which reads a `{char}` path wildcard; NOT `itemsearch.go`, the P19 catalog search)

**Handler struct + constructor** (MIRROR `characters.go:45-58`):
```go
// Source: readapi/characters.go:45-58 — read-side *store.Store; viewer id from the
// RequireSession-populated context (no *sql.DB in the handler).
type ItemsHandler struct{ store *store.Store }
func NewItems(s *store.Store) *ItemsHandler { return &ItemsHandler{store: s} }
```

**ServeHTTP** (MIRROR `characters.go:64-103` — GET-only 405, `webauth.UserFromContext`, `[]` not null,
V7 slog count+status only):
```go
// Source: readapi/characters.go:64-103.
func (h *ItemsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    ctx := r.Context()
    uid, _ := webauth.UserFromContext(ctx) // "" → nothing flagged is_mine; still a valid list
    rows, err := compute.Items(ctx, h.store, uid)
    if err != nil {
        slog.Error("items read failed", "err", err) // V7: op + err only — never an item/char name
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }
    out := rows
    if out == nil {
        out = []compute.ItemRollup{} // [] not null (the characters.go pre-size discipline)
    }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    if err := json.NewEncoder(w).Encode(out); err != nil {
        slog.Error("items encode failed", "err", err)
        return
    }
    slog.Info("items ok", "rows", len(out), "status", http.StatusOK) // V7: count + status only
}
```
Imports: `encoding/json`, `log/slog`, `net/http`, `compute`, `webauth` (mirror `characters.go:21-28`;
note `characters.go` imports `store` for its local struct — `items.go` returns `compute.ItemRollup`, so
import `compute` not `store`, the `inventory.go:27-29` import set).

---

### `cmd/squirebot-server/main.go` (config — ONE line)

**Analog:** `main.go:362-363` (the P31 route-registration lines). Add ONE line beside them, under
`webauth.RequireSession` (login-only, NEVER `RequireOfficer` — V4 membership gate):
```go
// Source: cmd/squirebot-server/main.go:362-363 — the P31 routes; add the new line beside them.
mux.Handle("GET /api/v1/inventory/{char}", webauth.RequireSession(db, readapi.NewInventory(st)))
mux.Handle("GET /api/v1/characters", webauth.RequireSession(db, readapi.NewCharacters(st)))
mux.Handle("GET /api/v1/items", webauth.RequireSession(db, readapi.NewItems(st))) // NEW (P32 / ITEM-01..03)
```
> Pitfall 2: `GET /api/v1/items` is DISTINCT from the existing `GET /api/v1/items/search` (`main.go:351`,
> P19 catalog search) — Go 1.22+ `ServeMux` treats them as separate patterns; no shadowing. Do NOT reuse
> `NewItemSearch`/`SearchCatalog`.

---

### `web/src/lib/items.ts` (utility, transform)

**Analog:** `web/src/lib/roster.ts` (exact two-function shape `viewerFirst`/`filterRoster`)

**Pure viewer-first + filter** (MIRROR `roster.ts:36-51`). `is_mine` is server-stamped on the rollup
(client never recomputes assignment — the `myview.ts` T-27-01 negative property; presentation not access
control):
```ts
// Source: web/src/lib/roster.ts:36-51 — the exact pattern to mirror.
import type { ItemRollup, ItemHolder } from './api';

/** Stable viewer-first: is_mine rows first, then A-Z (case-insensitive). NEW array. */
export function viewerFirstItems(rows: ItemRollup[]): ItemRollup[] {
    return [...rows].sort(
        (a, b) =>
            (a.is_mine ? 0 : 1) - (b.is_mine ? 0 : 1) ||
            a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })
    );
}
/** Name filter PRESERVING viewer-first order; empty query → full set; no-match → []. */
export function filterItems(rows: ItemRollup[], query: string): ItemRollup[] {
    const q = query.trim().toLowerCase();
    const matched = q === '' ? rows : rows.filter((r) => r.name.toLowerCase().includes(q));
    return viewerFirstItems(matched);
}
```

**Holder sort** (the holders-table viewer-first band order, UI-SPEC §F — REUSE the `roster.ts:26-32`
band idea, applied to `ItemHolder.is_mine`/`is_bank`):
```ts
// Band order: mine → guild → banks (mirror roster.ts BAND_ORDER), A-Z within each.
export function sortHolders(holders: ItemHolder[]): ItemHolder[] {
    const band = (h: ItemHolder) => (h.is_mine ? 0 : h.is_bank ? 2 : 1);
    return [...holders].sort(
        (a, b) => band(a) - band(b) || a.char.localeCompare(b.char, undefined, { sensitivity: 'base' })
    );
}
```

---

### `web/src/lib/__tests__/items.test.ts` (test, batch)

**Analog:** `web/src/lib/__tests__/roster.test.ts` (factory-fixture + describe/it idiom, node-only)

**Factory fixture + describe/it** (MIRROR `roster.test.ts:13-25`):
```ts
// Source: web/src/lib/__tests__/roster.test.ts:9-25.
import { describe, it, expect } from 'vitest';
import type { ItemRollup } from '../api';
import { viewerFirstItems, filterItems, sortHolders } from '../items';

function item(over: Partial<ItemRollup> = {}): ItemRollup {
    return {
        name: 'Cloak of Flames', summed_qty: 1, holder_count: 1, is_mine: false,
        price: null, prices: [], wiki_url: '', wiki_summary: '', is_quest_item: false,
        icon_id: 0, statsblock: '', holders: [], ...over
    };
}
```
Cases (RESEARCH Verification): `viewerFirstItems` floats `is_mine` first then A-Z; `filterItems`
preserves that among matches; empty query → full viewer-first set; no-match → `[]`; `sortHolders`
mine → guild → banks, A-Z within. (`vite.config.ts` runs `environment:node`, EXCLUDES
`*.svelte.{test,spec}.ts` — node test ONLY the pure helper; the DOM is a browser-smoke gap.)

---

### `web/src/lib/api.ts` (utility, request-response) — add interfaces + `fetchItems()`

**Analog:** `fetchCharacters()` (api.ts:289-293) + the `RosterCharacter`/`InventorySlot` interfaces
(api.ts:158-197)

**Interfaces** (mirror the Go snake_case contract; place under the P31 block at `api.ts:147+`; REUSE the
existing `PriceDetail` interface at `api.ts:79-83`):
```ts
// Mirror compute/types.go ItemRollup/ItemHolder (snake_case). Reuses PriceDetail (api.ts:79).
export interface ItemHolder {
    char: string; slot_label: string; qty: number; last_synced: string; is_mine: boolean; is_bank: boolean;
}
export interface ItemRollup {
    name: string; summed_qty: number; holder_count: number; is_mine: boolean;
    price: number | null; prices: PriceDetail[]; wiki_url: string; wiki_summary: string;
    is_quest_item: boolean; icon_id: number; statsblock: string; holders: ItemHolder[];
}
```

**Fetch wrapper** (MIRROR `api.ts:289-293` `fetchCharacters` — credentialed `getJSON`, `[]` on empty,
typed 401/403 the AuthGate re-routes):
```ts
// Source: web/src/lib/api.ts:289-293 — the fetchCharacters twin.
/** GET /api/v1/items → ItemRollup[] ([] when empty). Session-gated; the server returns
 *  the guild-wide rollup with the viewer's items flagged is_mine. */
export function fetchItems(f: typeof fetch = fetch): Promise<ItemRollup[]> {
    return getJSON<ItemRollup[]>('/api/v1/items', f);
}
```

---

### `web/src/routes/inventory/+page.svelte` (component — replace the P30 placeholder)

**Analog:** `web/src/routes/characters/+page.svelte` (master-detail + scoped search + `?c=` selection seam)

**State machine + onMount one-shot load + AuthGate guard** (MIRROR `characters/+page.svelte:23-92`):
```svelte
<!-- Source: web/src/routes/characters/+page.svelte:23-92. -->
import { onMount, getContext } from 'svelte';
import Search from '@lucide/svelte/icons/search';
import StateBlock from '$lib/components/StateBlock.svelte';
import ExaminePanel from '$lib/components/ExaminePanel.svelte';
import { AUTH_GUARD_KEY, type AuthGuard } from '$lib/components/AuthGate.svelte';
import { Unauthenticated, Forbidden, fetchItems, type ItemRollup, type InventorySlot } from '$lib/api';
import { filterItems, sortHolders } from '$lib/items';

const authGuard = getContext<AuthGuard>(AUTH_GUARD_KEY);
let status = $state<'loading' | 'error' | 'ready'>('loading');
let items = $state<ItemRollup[]>([]);
let query = $state('');
let selected = $state<string | null>(null); // the normalized name of the pinned item

async function load() { /* items = await fetchItems(); 401/403 → authGuard(err); else status='error' */ }
onMount(() => {
    const i = new URLSearchParams(window.location.search).get('i'); // ?i=<name> deep-link
    if (i) selected = i;
    void load();
});
let shown = $derived(filterItems(items, query));
let noResults = $derived(status === 'ready' && shown.length === 0 && query.trim() !== '');
```

**Selection (URL-reflect via `history.replaceState`, NO new route)** (MIRROR `characters/+page.svelte:118-129`):
```svelte
<!-- Source: characters/+page.svelte:118-129 — ?c= seam; here use ?i=<itemName>. -->
function select(name: string) {
    selected = name;
    if (typeof history !== 'undefined') {
        const url = new URL(window.location.href);
        url.searchParams.set('i', name); // encodeURIComponent in the URL constructor's setter
        history.replaceState(history.state, '', url);
    }
}
```

**The selectable list rows + search + states** (MIRROR `characters/+page.svelte:205-251` markup AND its
`<style>` block at lines 283-447 — the `.search`, `.row`, `.row.selected`, hover/focus-visible, the
44px touch target, the `grid-template-columns: minmax(280px,360px) 1fr; gap:24px` two-pane, the
`@media (max-width:640px)` stack are ALL reusable verbatim with item-flavored copy). Key deltas vs
`/characters`: NO band group-labels (single viewer-first run, UI-SPEC §B); the row carries the
`{summed_qty} · {holder_count} holders` headline + inline price + `Wiki ↗` (UI-SPEC §B/§Typography);
the "you" tag where `item.is_mine`.

**The examine reuse seam (load-bearing — D-03 / UI-SPEC §C):** build a representative `InventorySlot`-
shaped object from the selected `ItemRollup` and pass `charLastSeen=""` so the examine footer is omitted
(per-holder last-synced lives in the holders table). `ExaminePanel` (`ExaminePanel.svelte:23-29`) takes
`{ slot?: InventorySlot | null; charLastSeen?: string }` — pass it UNCHANGED:
```svelte
<!-- Source: ExaminePanel.svelte:23-29 props + examine.ts:65-110 field omission. -->
let selectedRollup = $derived(items.find((r) => r.name === selected) ?? null);
let asSlot = $derived<InventorySlot | null>(
    selectedRollup
        ? {
            item: selectedRollup.name, icon_id: selectedRollup.icon_id, statsblock: selectedRollup.statsblock,
            wiki_summary: selectedRollup.wiki_summary, is_quest_item: selectedRollup.is_quest_item,
            price: selectedRollup.price, prices: selectedRollup.prices, wiki_url: selectedRollup.wiki_url,
            // list-context-irrelevant — zero/empty (examine ignores them):
            location: '', category: 'general', canonical_slot: '', id: 0, count: 0, slots: 0,
            last_listed: '', children: []
          }
        : null
);
<!-- ... -->
<ExaminePanel slot={asSlot} charLastSeen="" />
```

**The holders table + holder deep-link (D-03 — the live P31 seam, ZERO P31 change):** `/characters`
pre-selects from `?c=` on mount (`characters/+page.svelte:88-92`), so a holder row links straight into
that window:
```svelte
<!-- Source: characters/+page.svelte:88-92 (the ?c= pre-select) + UI-SPEC §F. -->
{#each sortHolders(selectedRollup.holders) as h (h.char + h.slot_label)}
    <a href={`/characters?c=${encodeURIComponent(h.char)}`}> <!-- char names are guildie-controlled -->
        {h.char} <!-- plain {} — Svelte auto-escapes; NO {@html} (T-31-14) -->
    </a>
    <!-- + h.slot_label · ×{h.qty} · {h.last_synced}; "you"/"bank" tags per UI-SPEC §F -->
{/each}
```

**Constraints carried (UI-SPEC + RESEARCH):** theme tokens ONLY (no literal hex / no `--radius` global /
no Google Fonts link — Pitfall 6); the ONLY `{@html}` is `ExaminePanel`'s `composeItemNote` (no new sink);
the `<title>` head pattern (`characters/+page.svelte:193-195`); DOM-blind node tests → browser-smoke the
rendered tab on a DEPLOYED build (Pitfall 4).

---

### `web/src/lib/components/PaperdollSlot.svelte` (component — OPTIONAL extract)

**Analog:** itself — `PaperdollSlot.svelte:84-195` (the `.ico` tile: `<img>` over a deterministic
`hueFor` gradient + `onerror` fallback)

UI-SPEC §Color recommends extracting the `.ico` colored-tile mechanic (lines 84-88 `onImgError`,
107-118 the `<img>` + `.ico`, 177-195 the gradient `<style>`) into a small shared tile so the list-row
(32px) / detail-header (40px) icons don't drift from the 62px paperdoll. The hue function
(`PaperdollSlot.svelte:61-67`) and the `Item_${iconId}.png` URL (line 112, trusted-integer T-31-15) are
the contract to preserve. Planner's discretion (low effort, additive).

---

## Shared Patterns

### Compute-on-read public-fn → pure-helper split
**Source:** `compute/view.go:47-82` (`View`/`buildViewRows`) + `compute/inventory.go:118-145`
**Apply to:** `itemrollup.go` `Items`/`buildItemRollups`. The public fn fetches via `*store.Store`; the
pure helper takes typed slices, returns the model, holds NO ctx/store — directly table-testable. `compute`
authors ZERO SQL (imports `store`+`enrich` only).

### Group/join by NORMALIZED NAME, never item_id (the canonical landmine)
**Source:** `store/readviews.go:33-47` (the `InventoryJoinRow` doc + `pp_rep` CTE rationale)
**Apply to:** `buildItemRollups` group key = `strings.ToLower(strings.TrimSpace(name))`. The
EQ-inventory ids and PigParse-catalog ids are different namespaces; the representative `ViewRow.ID` is
used ONLY for the id-correct `item_master` icon/stats lookup (the watcher's own namespace), NEVER for
price. (Pitfall 1; memory `pigparse-vs-ingame-item-id-namespaces`.)

### Session-gated read route + viewer-id-from-context
**Source:** `readapi/characters.go:60-103` + `main.go:362-363`
**Apply to:** `items.go` + the new `main.go` line. `webauth.RequireSession` at registration (login-only
since P15 — NOT public, NOT `RequireOfficer`); the viewer id is server-truth from
`webauth.UserFromContext(ctx)`, bound only as `RosterFor`'s single `?` placeholder (V4/V5). The read is
guild-wide BY DESIGN — ORDERED viewer-first, never SCOPED to the viewer.

### V7 structured logging (count + status only)
**Source:** `characters.go:76,102` + `inventory.go:61,89-91`
**Apply to:** `items.go`. slog carries op + row count + status + err ONLY — NEVER an item name, char
name, or holder content.

### `[]` not null on empty (stable client shape)
**Source:** `characters.go:81-82` (pre-size) + `inventory.go:71-79` (nil→[] coercion)
**Apply to:** `items.go` (`out == nil → []compute.ItemRollup{}`).

### Pure DOM-free helper extracted to `.ts` + node-tested
**Source:** `web/src/lib/roster.ts` + `web/src/lib/__tests__/roster.test.ts` (+ `examine.ts` precedent)
**Apply to:** `items.ts` + `items.test.ts`. `vite.config.ts` `environment:node` EXCLUDES
`*.svelte.{test,spec}.ts` — the sort/filter/holder-sort is node-tested; the rendered DOM stays a
browser-smoke gap (Pitfall 4).

### Credentialed fetch wrapper + typed-error contract
**Source:** `web/src/lib/api.ts:218-260` (`getJSON`) + `api.ts:289-293` (`fetchCharacters`)
**Apply to:** `fetchItems()`. `credentials:'include'`; 401 → `Unauthenticated`, 403 → `Forbidden`
(the AuthGate re-routes), other non-2xx / transport / malformed-body → `ApiError`.

### Master-detail two-pane + `?<key>=` selection seam (relaxed consolidated-views rule)
**Source:** `web/src/routes/characters/+page.svelte` (whole file — markup + `<style>`)
**Apply to:** `inventory/+page.svelte`. ONE reusable detail panel rendered on selection (NOT N routes);
`history.replaceState` URL-reflect (`?i=`); the `.row`/`.search`/two-pane grid/mobile-stack styles are
reusable verbatim.

### The single `{@html}` sink rule (T-31-14)
**Source:** `ExaminePanel.svelte:8-17,40-45,59-62` (`composeItemNote`, escaped + scheme-allow-listed)
**Apply to:** the whole web surface. The ONLY raw-HTML is `ExaminePanel`'s wiki body (reused unchanged).
Item names + char names render via plain `{}` interpolation everywhere else (list rows, detail header,
holders table). This phase adds NO new `{@html}` sink.

### The colored-tile icon fallback (D-02 / T-31-15)
**Source:** `PaperdollSlot.svelte:61-67` (`hueFor`) + `84-118` (`<img onerror>` over `.ico` gradient)
**Apply to:** the list-row + detail-header icons. `Item_${iconId}.png` (trusted integer — no guildie
string in the path); `icon_id === 0` skips the `<img>`; the gradient is the ONE sanctioned non-token
color. Extract a shared tile to prevent drift (optional).

### Compute test: seeded temp-DB + raw-insert fixtures
**Source:** `compute/fixtures_test.go:25-68` (`newTestDB`/`seedChar`/`seedInv`/`seedItemMaster`/
`seedPigparse`) + `compute/view_test.go:19-45`
**Apply to:** `itemrollup_test.go` (external `compute_test` package). Seed a couple chars (one
viewer-assigned), an item held by 2+ chars in different slots, an unpriced item, an icon/stats item;
assert group-by-name (not id), summed qty across bag+loose, distinct holder count, `is_mine` propagation,
slot labels, coin/empty-slot exclusion.

---

## No Analog Found

None. Every new/modified file has an exact or close in-repo analog (this is a composition phase over
already-shipped P29/P30/P31 internals). The one genuinely-new algorithm — `buildItemRollups` grouping —
mirrors the `buildViewRows` + `buildBankValuation` pure-loop shape, so even it has a structural analog.

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| — | — | — | — |

---

## Metadata

**Analog search scope:** `internal/backendsrv/{compute,store,readapi}/`, `cmd/squirebot-server/main.go`,
`web/src/lib/`, `web/src/lib/components/`, `web/src/lib/__tests__/`, `web/src/routes/{characters,inventory}/`
**Files scanned:** ~18 (read in full or targeted ranges)
**Pattern extraction date:** 2026-06-18

---

## PATTERN MAPPING COMPLETE
