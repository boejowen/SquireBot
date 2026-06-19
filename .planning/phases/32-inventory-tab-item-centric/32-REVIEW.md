---
phase: 32-inventory-tab-item-centric
reviewed: 2026-06-19T00:33:50Z
depth: deep
files_reviewed: 13
files_reviewed_list:
  - internal/backendsrv/compute/itemrollup.go
  - internal/backendsrv/compute/itemrollup_test.go
  - internal/backendsrv/compute/itemrollup_internal_test.go
  - internal/backendsrv/compute/types.go
  - internal/backendsrv/store/readviews.go
  - internal/backendsrv/store/readviews_test.go
  - internal/backendsrv/readapi/items.go
  - internal/backendsrv/readapi/items_test.go
  - cmd/squirebot-server/main.go
  - web/src/lib/api.ts
  - web/src/lib/items.ts
  - web/src/lib/__tests__/items.test.ts
  - web/src/routes/inventory/+page.svelte
findings:
  blocker: 1
  high: 0
  medium: 0
  warning: 2
  info: 2
  total: 5
status: resolved
resolution: "All actioned findings fixed in commit 2637db9 (web-only, redeployed). CR-01 (BLOCKER each_key_duplicate crash) keyed by holding index; WR-01 (stale detail-icon display:none) keyed the header <img> on the selected name; WR-02 (raw ISO last-synced) swapped to LastSyncedCell. The 2 INFO items are advisory/no-op. check 0/0 + 359 web tests green; live."
---

# Phase 32: Code Review Report — Inventory Tab (item-centric)

> **RESOLVED 2026-06-18 (commit `2637db9`, web-only redeploy).** CR-01 (BLOCKER),
> WR-01, and WR-02 are fixed and live; the 2 INFO items are advisory only. See
> `32-03-SUMMARY.md` → "Fix-forward (round 2 — code review)" for the resolution detail.

**Reviewed:** 2026-06-19T00:33:50Z
**Depth:** deep (cross-file: store → compute → readapi → main.go route; api.ts → items.ts → +page.svelte → ExaminePanel reuse)
**Files Reviewed:** 13
**Status:** issues_found

## Summary

The backend half is clean and disciplined: `compute/itemrollup.go` authors zero SQL, groups strictly
by normalized name (never `item_id`), copies the name-bridged representative price rather than
re-selecting it, and uses the representative `ID` only for the id-correct `item_master` icon/stats
lookup — exactly the namespace split CLAUDE.md mandates. The route is session-gated (`RequireSession`,
not officer, not public) and does NOT collide with the P19 `GET /api/v1/items/search` catalog route
(distinct Go 1.22+ ServeMux patterns; the more-specific `/items/search` wins its own path, `/items`
matches the exact path). V5 (no server-side user input, viewer id binds only as RosterFor's single `?`)
and V7 (slog carries op + row count + status + err only) hold. No migration was added (extend-only,
schema v13). XSS posture is sound — names render via plain `{}`, the only `{@html}` is the reused
ExaminePanel's escaped `composeItemNote`, the holder deep-link `encodeURIComponent`s the char name,
and the icon `<img>` src interpolates only the trusted integer `icon_id`.

The defect surface is entirely in the new Svelte tab. One BLOCKER: the holders-table keyed `{#each}`
uses `h.char + h.slot_label`, which is NOT unique — a single character holding the same item in two
different bags yields two holders both labeled `"Bag"` (slotLabel collapses every `*-Slot<N>` child to
the literal `"Bag"`), producing a duplicate key that crashes the detail panel under Svelte 5. Two WARN
items (a stale imperative `display:none` that hides a valid detail icon after an errored one, and a raw
ISO last-synced render that diverges from the app's `LastSyncedCell` presentation) and two INFO round
it out.

## Blockers

### CR-01: Holders-table keyed `{#each}` can emit duplicate keys → Svelte 5 detail-panel crash

**File:** `web/src/routes/inventory/+page.svelte:316`
**Issue:**
The holders list is a keyed each over `(h.char + h.slot_label)`:

```svelte
{#each sortHolders(selectedRollup.holders) as h (h.char + h.slot_label)}
```

`slot_label` is produced by `compute.slotLabel` (`internal/backendsrv/compute/itemrollup.go:125-138`),
which collapses **every** bagged copy to the literal string `"Bag"`:

```go
if _, isChild := splitChild(location); isChild {
    return "Bag" // bagged copy; the parent bag name is not joined here (A2)
}
```

`buildItemRollups` appends one `ItemHolder` per holding with no per-(char, label) dedup
(`itemrollup.go:93-100`). So a single character holding the same stackable item in two different bags —
e.g. Bone Chips at `General4-Slot1` and `General7-Slot2`, or gems/Spider Silk/Bat Wings split across
bags, an extremely common P99 inventory state — produces two holders both `{char: "Apple", slot_label:
"Bag"}`. Their `{#each}` key is identical (`"AppleBag"` for both).

Svelte 5 (this repo runs 5.56.0) throws `each_key_duplicate` for a keyed block with non-unique keys.
The error fires while rendering the selected item's detail column, so selecting any such item breaks the
detail panel (and, depending on error-boundary behavior, the tab). The existing tests do not catch it:
the e2e fixture (`itemrollup_test.go:86-87`) pairs a loose copy (`General1` → "General · General1") with
ONE bagged copy (`General4-Slot1` → "Bag"), so the two labels differ and never collide; the internal
test never seeds two bagged copies of one char; and `items.test.ts` is DOM-blind so it never renders the
each block. The `/characters` mirror does not hit this because it keys by `c.name` (unique per roster) —
this is specific to the holders table.

**Fix:**
Make the key unique. Two equally valid options:

- Cheapest: include the raw location/slot uniqueness in the holder model. The `slot_label` is lossy by
  design (A2), so add a per-holder discriminator. Simplest in the template, key by index (acceptable
  here because `sortHolders` returns a fresh array each render and the list is small/static per
  selection):
  ```svelte
  {#each sortHolders(selectedRollup.holders) as h, idx (h.char + ':' + h.slot_label + ':' + idx)}
  ```
- Cleaner: carry the raw `Location` (or `RowOrdinal`) on `ItemHolder` in `compute.buildItemRollups`
  and key by `h.char + h.location`, which is unique per holding. This also future-proofs the label if
  bag names ever get joined.

Add a regression case (one char, two `*-Slot<N>` copies of one item) to either `itemrollup_internal_test.go`
(asserting two distinct holders survive) or a browser-smoke step, since node vitest cannot render the each.

## Warnings

### WR-01: Stale imperative `display:none` hides a valid detail-header icon after an errored one

**File:** `web/src/routes/inventory/+page.svelte:148-151, 273-280`
**Issue:**
`onImgError` imperatively hides the `<img>`:

```js
function onImgError(e: Event) {
    (e.currentTarget as HTMLImageElement).style.display = 'none';
}
```

The detail-header icon is a **single** `<img>` element (not inside an `{#each}`); its `src` updates
reactively from `selectedRollup.icon_id`. When the user selects item A whose icon 404s (`onImgError`
sets `style.display='none'` on that DOM node), then selects item B (icon_id>0, valid), the `{#if
selectedRollup.icon_id > 0}` stays true so Svelte reuses the same `<img>` node and only swaps `src`.
The imperatively-set `display:none` is never reset, so item B's valid icon stays hidden until a full
remount. The list rows are immune (each row is keyed by `it.name`, so each has its own `<img>`), but the
detail header is shared. (`onerror` swallowing the colored-tile-fallback decision into imperative DOM is
the root cause.)

**Fix:**
Reset visibility when `src` changes, or drive the fallback declaratively. Minimal fix — track a
per-selection "icon broken" boolean keyed to the selected name and use it to render the `<img>` vs the
tile, instead of mutating `style.display`. Or, cheapest, also reset on load:
```svelte
<img ... onerror={onImgError} onload={(e) => ((e.currentTarget as HTMLImageElement).style.display = '')} />
```
so a subsequent successful load clears a prior hide. (The declarative approach is preferable — it also
removes the only imperative DOM write in the file.)

### WR-02: Holder "Last synced" renders the raw ISO string, diverging from the app's date presentation

**File:** `web/src/routes/inventory/+page.svelte:331`
**Issue:**
The holders table prints the timestamp verbatim:

```svelte
<span class="holder-synced" role="cell">{h.last_synced}</span>
```

`last_synced` is `character.last_seen`, an ISO string like `2026-06-18T00:00:00Z`. Everywhere else the
app shows freshness via `LastSyncedCell.svelte` (a friendly `YYYY-MM-DD` plus a fresh/aging/stale dot)
or the ExaminePanel footer's "Last synced: …". Rendering the full ISO timestamp in a table cell is
harder to scan, inconsistent with the view/bank grids, and shows a misleading `00:00:00Z` time
component for what is a day-granularity value. It is also unguarded against an empty string (a
never-synced char would render a blank cell with no "—").

**Fix:**
Reuse `LastSyncedCell` (it already handles invalid/empty → "—" and the freshness dot), or at minimum
slice to the date: `{h.last_synced ? h.last_synced.slice(0, 10) : '—'}`. Reusing the cell keeps the
freshness signal consistent with the rest of the tabs.

## Info

### IN-01: `RowOrdinal` is scanned in the store reads but never consumed by the rollup

**File:** `internal/backendsrv/store/readviews.go:204, 245` (and `InventoryForChar`)
**Issue:**
`InventoryJoin` selects and scans `ii.row_ordinal` into `InventoryJoinRow.RowOrdinal`, but the Phase 32
rollup never reads it (grep for `RowOrdinal` in `compute/` returns nothing). It is harmless dead data
flowing through `View` → `buildItemRollups`. Not introduced by Phase 32 (it predates it), but it is the
natural carrier the BLOCKER fix wants for a stable per-holding key — surfacing it on `ItemHolder` would
both fix CR-01 cleanly and give the dead field a purpose.
**Fix:** Optional. If you adopt the "carry raw Location/RowOrdinal on ItemHolder" form of the CR-01 fix,
this field stops being dead. Otherwise no action.

### IN-02: `holderWord` is duplicated logic also present in the per-row aria-label

**File:** `web/src/routes/inventory/+page.svelte:161-163, 204`
**Issue:**
The singular/plural "holder"/"holders" word is centralized in `holderWord(n)` and reused in three
template spots — good. The row `aria-label` at line 204 re-derives the same `${it.holder_count}
${holderWord(it.holder_count)}` inline, which is fine, but the "{qty} guild-wide" phrasing is duplicated
between the aria-label and the visible `.row-headline`/`.detail-meta`, so a future copy change must touch
two strings. Minor maintainability note, not a defect.
**Fix:** Optional — extract a `summaryText(it)` helper if the phrasing is expected to evolve.

---

_Reviewed: 2026-06-19T00:33:50Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
