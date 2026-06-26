# Phase 40: Item display polish — flag outlines + named quest links - Pattern Map

**Mapped:** 2026-06-25
**Files analyzed:** 12 (4 backend Go, 8 web Svelte/TS)
**Analogs found:** 12 / 12 (every file extends an in-repo precedent; no greenfield)

> This is a CONFORM-to-existing-pattern phase. NO migration, NO new data pipeline, watcher untouched (→ no `v*` tag). Every file below modifies an existing file and copies a same-file or sibling precedent. The two load-bearing precedents the planner leans on hardest:
> - **Backend flag plumbing** = the Phase-39 `is_clicky`/`has_haste` path (`item_master` column → `store.IconStats` → `compute.ItemRollup` → `api.ts`).
> - **Backend quest plumbing** = the existing `QuestLinksByItem` → `compute.QuestLink` → `ViewRow.quest_links` path (already wired into `view.go`/`bank.go`; this phase mirrors it onto the modern `InventorySlot` + `ItemRollup` builders + adds a `source_url` column).

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/backendsrv/store/readviews.go` | store (read) | request-response | same-file: `QuestLinksByItem`+`IconStats`/`ItemMasterIconStats` | exact (extend self) |
| `internal/backendsrv/compute/types.go` | model (JSON contract) | request-response | same-file: `QuestLink`, `InventorySlot`, `ItemRollup` (P39 `IsClicky`/`HasHaste`) | exact (extend self) |
| `internal/backendsrv/compute/inventory.go` | service (compute builder) | transform | same-file: `slotFromRow` + view.go `questLinksFor` | exact (role+flow) |
| `internal/backendsrv/compute/itemrollup.go` | service (compute builder) | transform | same-file: `buildItemRollups` (P39 `IsClicky`/`HasHaste` attach) | exact (role+flow) |
| `web/src/lib/api.ts` | model (TS contract) | request-response | same-file: `ItemRollup.is_clicky`/`has_haste` (250-251), `ViewRow.quest_links` (107) | exact (extend self) |
| `web/src/lib/theme/themes.ts` | config (token registry) | n/a | same-file: `ThemeTokens` `statusOther`/`statusMissing` per-theme blocks | exact (extend self) |
| `web/src/app.css` | config (CSS tokens) | n/a | same-file: `--status-other`/`--status-missing` per `[data-theme]` block | exact (extend self) |
| `web/src/lib/__tests__/themes.test.ts` | test | n/a | same-file: `REQUIRED_TOKENS` parity + velious spot-checks | exact (extend self) |
| `web/src/lib/components/PaperdollSlot.svelte` | component | event-driven (render) | same-file: `.slot.filled:hover` rules + `$derived` hue/ariaLabel | exact (extend self) |
| `web/src/lib/examine.ts` | utility (pure helper) | transform | same-file: `examineFields` flags/wiki + `wikiHref`; sibling `items.ts facetItems` | exact (pure-helper precedent) |
| `web/src/lib/components/ExaminePanel.svelte` | component | event-driven (render) | same-file: `.ex-wiki` `{@html}` sink + `.tooltip-wiki-link` style | exact (extend self) |
| `web/src/lib/tooltip/composeNotes.ts` + `web/src/lib/components/ItemTooltip.svelte` | utility + component | transform + render | same-file: `composeNotes.ts:154-161` quest list + `:global(.tooltip-wiki-link)` | exact (extend self) |

**No file has a "no analog" result** — every change is an additive extension of an existing, tested code path. The "No Analog Found" section below is intentionally empty.

---

## Shared Patterns (apply across multiple files)

### S-1 — The Phase-39 flag plumbing template (backend, ITEMUI-01)
**Source path:** `item_master` column (00016) → `store.IconStats` → `compute.ItemRollup` → `web/api.ts`
**Apply to:** all three new flag booleans `is_no_drop` / `is_lore` / `is_magic`
The exact precedent is `is_clicky`/`has_haste`. Copy it line-for-line at each layer:

- **Store struct + scan** (`readviews.go:759-803`):
```go
type IconStats struct {
	IconID     int64
	Statsblock string
	IsClicky   bool // Phase 39 — item_master.is_clicky (00016), EQ-namespace correct here
	HasHaste   bool // Phase 39 — item_master.has_haste (00016)
}

func (s *Store) ItemMasterIconStats(ctx context.Context) (map[int64]IconStats, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT item_id, icon_id, statsblock, is_clicky, has_haste FROM item_master`)
	...
	var (
		id    int64
		icon  sql.NullInt64
		stats sql.NullString
		clk   sql.NullInt64 // Phase 39 — NULL/0 → false (a pre-00016 / un-flagged row)
		hst   sql.NullInt64
	)
	if err := rows.Scan(&id, &icon, &stats, &clk, &hst); err != nil { ... }
	out[id] = IconStats{ // NULL → 0 / "" / false
		IconID:     icon.Int64,
		Statsblock: stats.String,
		IsClicky:   clk.Int64 != 0, // the established NullInt64 → bool idiom
		HasHaste:   hst.Int64 != 0,
	}
```
The three new flags follow the `clk`/`hst` shape exactly: add `is_no_drop, is_lore, is_magic` to the SELECT column list, declare three `sql.NullInt64` locals, append them to the `Scan`, and resolve each with `nd.Int64 != 0`. The `NULL → false` semantics are correct (a pre-00016 / un-backfilled row is "no flag").

- **JSON struct** (`compute/types.go:237-238`):
```go
IsClicky    bool          `json:"is_clicky"`  // Phase 39 — from item_master (00016); client holdings facet (SC-4)
HasHaste    bool          `json:"has_haste"`  // Phase 39 — from item_master (00016)
```
Add `IsNoDrop`/`IsLore`/`IsMagic bool` with `json:"is_no_drop"/"is_lore"/"is_magic"` at the right edge (append-only, the locked contract rule).

- **api.ts mirror** (`api.ts:250-251`):
```ts
is_clicky: boolean; // Phase 39 — mirrors compute.ItemRollup (item_master); client holdings facet
has_haste: boolean; // Phase 39
```

### S-2 — The quest-link attach precedent (backend, ITEMUI-02)
**Source:** `compute/view.go:47-112`
**Apply to:** `compute/inventory.go` (InventorySlot) + `compute/itemrollup.go` (ItemRollup)
`View()` already fetches `s.QuestLinksByItem(ctx)` and maps it per-item with `questLinksFor`:
```go
func View(ctx context.Context, s *store.Store) ([]ViewRow, error) {
	joinRows, err := s.InventoryJoin(ctx, false)
	...
	links, err := s.QuestLinksByItem(ctx)
	...
	return buildViewRows(joinRows, links), nil
}

// questLinksFor maps the store quest links for itemID into the public QuestLink shape (nil when none).
func questLinksFor(itemID int64, links map[int64][]store.QuestLinkRow) []QuestLink {
	src := links[itemID]
	if len(src) == 0 {
		return nil
	}
	out := make([]QuestLink, 0, len(src))
	for _, l := range src {
		out = append(out, QuestLink{QuestName: l.QuestName, Source: l.Source}) // ← add SourceURL: l.SourceURL
	}
	return out
}
```
Reuse `questLinksFor` verbatim (after the `SourceURL` field is added). The modern builders just need to fetch the `links` map and call `questLinksFor(<itemID>, links)`.

### S-3 — The `safeHttpUrl` + escaped-link discipline (web, ITEMUI-02 / D-05)
**Source:** `web/src/lib/tooltip/composeNotes.ts:69-72` (`safeHttpUrl`) + `:108-120` (name+wiki link composition)
**Apply to:** every quest `<a href>` in `examine.ts`/`ExaminePanel.svelte` AND `composeNotes.ts`/`ItemTooltip.svelte`
This is the load-bearing security control (D-05). The href MUST pass `safeHttpUrl` FIRST; the name is `escapeHtml`'d (tooltip `{@html}` path) or Svelte-auto-escaped (examine `{#each}` path). Never interpolate a `source_url` or `quest_name` raw.
```ts
export function safeHttpUrl(url: string): string {
  const trimmed = String(url).trim();
  return /^https?:\/\//i.test(trimmed) ? trimmed : '';
}
// usage (existing wiki-link composition — the exact shape quest links copy):
const safeUrl = safeHttpUrl(wikiUrl);
if (safeUrl) {
  parts.push(`<a class="tooltip-wiki-link" href="${escapeHtml(safeUrl)}" target="_blank" rel="noopener">wiki</a>`);
}
```

### S-4 — Pure DOM-free helper for vitest coverage (web)
**Source:** `web/src/lib/items.ts:38-50` (`facetItems`) and `web/src/lib/examine.ts` (whole file)
**Apply to:** the D-01 priority-flag resolution + the `notes_link` filter
web vitest here is DOM-blind (no `@testing-library/svelte`). The priority resolver (`is_no_drop`>`is_lore`>`is_magic` → token name + chip label) and the `notes_link`-only filter are PURE functions — put them in `examine.ts` (or a sibling pure `.ts`) so `npm test` covers the LOGIC; the `.svelte` render stays a browser-smoke gap. `items.ts:facetItems` is the existing precedent of a pure AND-combined flag predicate that is node-tested.

---

## Pattern Assignments

### `internal/backendsrv/store/readviews.go` (store, request-response)

**Analog:** same file — two existing methods to extend.

**D-08 quest plumbing — extend `QuestLinksByItem` + `QuestLinkRow`** (lines 94-99, 460-488):
```go
// QuestLinkRow is one quest link for an item (quest_items row), grouped by item_id.
type QuestLinkRow struct {
	QuestName string
	Source    string
	// ← ADD: SourceURL string
}

func (s *Store) QuestLinksByItem(ctx context.Context) (map[int64][]QuestLinkRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT item_id, quest_name, source FROM quest_items ORDER BY item_id, quest_name`)
		// ← ADD source_url to the column list: SELECT item_id, quest_name, source, source_url FROM quest_items ...
	...
	for rows.Next() {
		var (
			itemID int64
			link   QuestLinkRow
			source sql.NullString
			// ← ADD: sourceURL sql.NullString
		)
		if err := rows.Scan(&itemID, &link.QuestName, &source); err != nil { ... }
		// ← change scan to: &itemID, &link.QuestName, &source, &sourceURL
		link.Source = source.String
		// ← ADD: link.SourceURL = sourceURL.String
		out[itemID] = append(out[itemID], link)
	}
```
`source_url` is `TEXT` (nullable in `00001_init.sql:63` — `quest_items(... source_url TEXT, source TEXT ...)`), so scan into `sql.NullString` and resolve `.String` (the exact idiom `source` already uses two lines up — the `in_game_flag` pseudo-row's empty `source_url` resolves to `""`).

**D-07 flag plumbing — extend `IconStats` + `ItemMasterIconStats`** (lines 759-803): copy the `is_clicky`/`has_haste` shape verbatim for `is_no_drop`/`is_lore`/`is_magic` — see **S-1** above for the full excerpt.

**D-07 flag plumbing — extend `InventoryRow` + `InventoryForChar`** (lines 73-92, 387-458): `InventoryForChar` is the read that feeds the modern per-character `InventorySlot` builder. It ALREADY joins `item_master` id-keyed and scans `icon_id`/`statsblock`/`is_quest_item`. Add the three flags alongside `iconID`/`statsblock` — exact same place, exact same nullable-scan idiom:
```go
// the existing SELECT already pulls im.* columns id-keyed:
SELECT c.name, ii.location, ii.name, ii.item_id, ii.count, ii.slots,
       im.wiki_url, im.wiki_summary, im.is_quest_item, im.icon_id, im.statsblock,
       // ← ADD: im.is_no_drop, im.is_lore, im.is_magic
       pp.direction, pp.a30, pp.t30, pp.last_seen,
       c.last_seen, ii.row_ordinal
...
var (
	...
	iconID       sql.NullInt64  // item_master.icon_id — NULL until enrichment runs (INV-04)
	statsblock   sql.NullString // item_master.statsblock — NULL until enrichment runs (INV-02)
	// ← ADD: isNoDrop, isLore, isMagic sql.NullInt64
	...
)
// in the scan + assignment, mirror the iconID/statsblock NULL→zero idiom:
r.IconID = iconID.Int64          // 0 when NULL
r.Statsblock = statsblock.String // "" when NULL
// ← ADD: r.IsNoDrop = isNoDrop.Int64 != 0  (etc.)
```
`InventoryRow` (line 73) gains `IsNoDrop`/`IsLore`/`IsMagic bool` next to `IconID`/`Statsblock`.

> **Builder-source note (load-bearing):** `InventorySlot` flags come from the `item_master` join inside **`InventoryForChar`** (the held-item read — D-07: "the `item_master` (held) read is the source"). `ItemRollup` flags come from **`ItemMasterIconStats`** (the same id-correct `item_master` map `itemrollup.go` already uses for icon/stats). `catalog_enrichment` (P38, name-keyed/unheld) is NOT involved — outlines are a held-item-only concern.

---

### `internal/backendsrv/compute/types.go` (model, request-response)

**Analog:** same file.

**`QuestLink` gains `SourceURL`** (lines 71-76):
```go
type QuestLink struct {
	QuestName string `json:"quest_name"`
	Source    string `json:"source"`
	// ← ADD: SourceURL string `json:"source_url"`
}
```
Also update the package-doc JSON contract block (lines 39-41) to list the new `SourceURL → "source_url"` mapping.

**`InventorySlot` gains 3 flags + quest_links** (lines 158-175): append at the right edge after `Statsblock` —
```go
IconID        int64           `json:"icon_id"`
Statsblock    string          `json:"statsblock"`
// ← ADD:
// IsNoDrop  bool        `json:"is_no_drop"`   // item_master flag (00016); D-07 tile outline
// IsLore    bool        `json:"is_lore"`
// IsMagic   bool        `json:"is_magic"`
// QuestLinks []QuestLink `json:"quest_links"` // notes_link named quests (ITEMUI-02); same shape as ViewRow.QuestLinks
```

**`ItemRollup` gains 3 flags + quest_links** (lines 225-240): the `IsClicky`/`HasHaste` lines (237-238) are the EXACT insertion-point precedent —
```go
IsClicky    bool          `json:"is_clicky"`  // Phase 39 — from item_master (00016)
HasHaste    bool          `json:"has_haste"`  // Phase 39
// ← ADD IsNoDrop/IsLore/IsMagic bool + QuestLinks []QuestLink here (right edge, before Holders)
Holders     []ItemHolder  `json:"holders"`
```

---

### `internal/backendsrv/compute/inventory.go` (service, transform)

**Analog:** same-file `slotFromRow` (lines 262-283) + `view.go`'s `questLinksFor` (S-2).

**Flags onto the slot** — `slotFromRow` maps `InventoryRow` → `InventorySlot`; it already copies `IconID`/`Statsblock`. Add the three flags the same way:
```go
func slotFromRow(row store.InventoryRow, cat SlotCategory, canonical string) InventorySlot {
	prices := pricesFromRow(row)
	return InventorySlot{
		...
		IconID:        row.IconID,
		Statsblock:    row.Statsblock,
		// ← ADD: IsNoDrop: row.IsNoDrop, IsLore: row.IsLore, IsMagic: row.IsMagic,
	}
}
```

**Quest links onto the slot** — `StructuredInventory` (lines 118-124) currently fetches ONLY `InventoryForChar`. It must additionally fetch `s.QuestLinksByItem(ctx)` and thread the map into `buildStructuredInventory` so each slot gets `questLinksFor(row.ItemID, links)`:
```go
func StructuredInventory(ctx context.Context, s *store.Store, char string) (CharacterInventory, error) {
	rows, err := s.InventoryForChar(ctx, char)
	if err != nil { return CharacterInventory{}, err }
	// ← ADD: links, err := s.QuestLinksByItem(ctx)  (handle err)
	return buildStructuredInventory(char, rows /*, links */), nil
}
```
`buildStructuredInventory` gains a `links map[int64][]store.QuestLinkRow` param; `slotFromRow` gains the same param (or the caller sets `slot.QuestLinks = questLinksFor(row.ItemID, links)` after `slotFromRow` returns — keeps `slotFromRow` signature pure-ish, executor's choice). `questLinksFor` lives in `view.go` (same package) — reuse it, do NOT re-implement. **Purity preserved:** `buildStructuredInventory` stays a pure transform (the store fetch is in the public `StructuredInventory`), exactly as the file's doc-comment promises (lines 113-117).

---

### `internal/backendsrv/compute/itemrollup.go` (service, transform)

**Analog:** same-file `buildItemRollups` (lines 59-119) — the P39 `IsClicky`/`HasHaste` attach (lines 82-83) is the precedent.

**Flags onto the rollup** — the `iconStats[vr.ID]` lookup already yields `IsClicky`/`HasHaste`; once `IconStats` carries the three new flags (S-1) just copy them in the same struct literal:
```go
ic := iconStats[vr.ID] // representative id-correct icon/stats (item_master EQ namespace)
roll = &ItemRollup{
	Name:        vr.Item,
	...
	IsClicky:    ic.IsClicky, // Phase 39 — holdings facet (SC-4), id-correct from item_master
	HasHaste:    ic.HasHaste, // Phase 39
	// ← ADD: IsNoDrop: ic.IsNoDrop, IsLore: ic.IsLore, IsMagic: ic.IsMagic,
}
```

**Quest links onto the rollup** — `Items()` (lines 38-52) composes `View()` (which already carries `QuestLinks` on each `ViewRow`). The representative `ViewRow` already has `vr.QuestLinks` populated — copy it onto the rollup in the `roll == nil` first-seen branch (alongside `WikiURL`/`Prices`, which are likewise copied from the representative row):
```go
roll = &ItemRollup{
	...
	WikiURL:     vr.WikiURL,
	WikiSummary: vr.WikiSummary,
	IsQuestItem: vr.IsQuestItem,
	// ← ADD: QuestLinks: vr.QuestLinks,  (representative row already carries them — no re-fetch)
}
```
No new store call needed in `itemrollup.go` — `View()` already fetched the links. This is the cleanest path (mirrors how Price/WikiURL are "copied from the representative, NEVER re-selected" — the file's IRON LAW, lines 10-15).

---

### `web/src/lib/api.ts` (model, request-response)

**Analog:** same file — `ItemRollup.is_clicky`/`has_haste` (250-251) + `ViewRow.quest_links` (107).

`InventorySlot` (177-197) gains `is_no_drop`/`is_lore`/`is_magic: boolean` + `quest_links: QuestLink[]` at the right edge (after `statsblock`). `ItemRollup` (237-253) gains the same three booleans (next to `is_clicky`/`has_haste`) + `quest_links: QuestLink[]`. The `QuestLink` interface gains `source_url: string` (mirror the Go `SourceURL → "source_url"` tag). `ViewRow.quest_links` (107) is left untouched per D-08, but its `QuestLink` element type now also carries `source_url` (harmless — the legacy views just ignore it).

> Locate the existing `QuestLink` interface in api.ts (referenced by `ViewRow.quest_links` at line 107) and add `source_url: string` there. Grep `interface QuestLink` to find it.

---

### `web/src/lib/theme/themes.ts` + `web/src/app.css` (config — MUST be in lockstep)

**Analog:** the `statusOther`/`statusMissing` per-theme token, defined in BOTH files.

**themes.ts** — `ThemeTokens` interface (31-51) gains three keys; each of the 5 `THEMES` entries (53-116) gains the three values. The interface declaration shape to copy:
```ts
export interface ThemeTokens {
  ...
  statusOk: string;
  statusMissing: string;
  statusOther: string;
  // ← ADD: flagNodrop: string; flagLore: string; flagMagic: string;
  ...
}
// e.g. velious entry:
velious: {
  ...
  statusMissing: '#e88a8a',
  statusOther: '#a8c5e0',
  // ← ADD (per UI-SPEC §Color per-theme table):
  //   flagNodrop: '#e88a8a', flagLore: '#e3c46b', flagMagic: '#6db3e0',
  ...
},
```
Per-theme values are prescriptive in UI-SPEC §Color (lines 102-108) — velious `#e88a8a`/`#e3c46b`/`#6db3e0`, vanilla `#d96b6b`/`#d4af37`/`#6fa0d4`, kunark `#d96b6b`/`#d4a020`/`#5f9fd4`, minimalist `#d98a8a`/`#d4b15c`/`#7fa8d4`, heavy `#c0392b`/`#d4a017`/`#3f78c0`.

**app.css** — each `[data-theme="<key>"]` block (33-113) gains three custom props. The shape (mirroring `--status-other`):
```css
:root,
[data-theme="velious"] {
  ...
  --status-missing: #e88a8a;
  --status-other: #a8c5e0;
  /* ← ADD: --flag-nodrop / --flag-lore / --flag-magic (UI-SPEC §Color) */
  --flag-nodrop: #e88a8a;
  --flag-lore: #e3c46b;
  --flag-magic: #6db3e0;
  ...
}
```
The `:root` fallback shares the velious block (line 33-34), so it gets the velious values automatically. Add the three lines to ALL FIVE `[data-theme]` blocks. Keep the file-header token-list comment (28-30) updated.

---

### `web/src/lib/__tests__/themes.test.ts` (test)

**Analog:** same file — `REQUIRED_TOKENS` parity list (16-27) + velious spot-checks (69-75).

Extend `REQUIRED_TOKENS` with `'flagNodrop','flagLore','flagMagic'` (the "defines all required tokens for every theme" loop at 52-59 then asserts each theme carries them — the parity gate). Add a velious spot-check (mirror line 73) asserting the three new velious values match UI-SPEC, e.g.:
```ts
const REQUIRED_TOKENS: (keyof ThemeTokens)[] = [
  'bg','panel','text','accent','statusOk','statusMissing','statusOther',
  // ← ADD: 'flagNodrop','flagLore','flagMagic',
  'fontDisplay','fontBody','weightDisplay',
];
// and a spot-check (mirror the existing 'matches the UI-SPEC catalog' it-block):
expect(THEMES.velious.flagNodrop).toBe('#e88a8a');
expect(THEMES.velious.flagLore).toBe('#e3c46b');
expect(THEMES.velious.flagMagic).toBe('#6db3e0');
```
> Note: the "defines all 10 required tokens" test name (line 52) says "10" — update to the new count when extending `REQUIRED_TOKENS`, or de-hardcode it. `themeApply.test.ts` asserts the apply/persist WIRING (not token shape) — it needs NO change for the new tokens.

---

### `web/src/lib/components/PaperdollSlot.svelte` (component, render)

**Analog:** same file — the `.slot.filled:hover`/`:focus-visible` rules (149-158), the `$derived` props (45-74), and the `style={...--tile-hue...}` inline-var pattern (105).

**D-01 priority `--flag-color` derivation** — add a `$derived` mirroring the existing `hue`/`isBag` deriveds, computing the priority token from the three slot flags:
```ts
// existing precedent for an inline CSS var derived from slot data:
let hue = $derived(hueFor(slot));
// ← ADD a flag-color derived (priority No-Drop > Lore > Magic; '' when none):
let flagColor = $derived.by(() => {
	if (!slot || !filled) return '';
	if (slot.is_no_drop) return 'var(--flag-nodrop)';
	if (slot.is_lore)    return 'var(--flag-lore)';
	if (slot.is_magic)   return 'var(--flag-magic)';
	return '';
});
```
> The priority logic is small but ideal for the **S-4** pure-helper pattern — extract `flagColorFor(slot)` (returns the token string or `''`) and/or `flagPriority(slot)` into `examine.ts` (or a sibling pure `.ts`) so node-vitest covers it; the `.svelte` just calls it. This is the same DOM-blind-coverage reasoning `items.ts:facetItems` follows.

**Wire the inline var + class** — the existing `style={...}` already sets one inline var; append `--flag-color` conditionally, and gate the `::before` on a `class:flagged`:
```svelte
<button
	class="slot filled"
	class:bag={isBag}
	class:flagged={flagColor !== ''}
	style={`--tile-hue: ${hue};${flagColor ? ` --flag-color: ${flagColor};` : ''}`}
	...
```

**The `::before` ring** — add to `<style>` (the prescriptive CSS is UI-SPEC §ITEMUI-01 lines 172-184). It must NOT touch `border-color` (the hover/focus rules at 149-158 flip that to `--accent` — the ring rides a separate inset box-shadow so both coexist, D-03):
```css
/* Only on a flagged FILLED tile — the inset 2px flag ring (D-01/D-03). Coexists with
   the accent border + box-shadow that :hover/:focus-visible add. */
.slot.filled.flagged::before {
	content: '';
	position: absolute;
	inset: 0;
	border-radius: 3px;            /* match .slot */
	box-shadow: inset 0 0 0 2px var(--flag-color);
	pointer-events: none;          /* never steal hover/click from the button */
	z-index: 1;                    /* above .ico gradient, below .count/.bag-marker */
}
```
The existing corner markers `.count` (196-208, `top:1px right:3px`) and `.bag-marker` (209-217, `top:1px left:3px`) sit inset-1px on the content layer; the perimeter inset ring reads around them. `@media (prefers-reduced-motion)` (218-226) is unchanged — the ring is static.

**Surfaces for free (D-02):** `InventoryWindow.svelte` renders `PaperdollSlot` for the worn / general / bank grids (5 `<PaperdollSlot>` call-sites at lines 147/166/186/259/289) and the Banks tab reuses `InventoryWindow` — so the ring rides the component everywhere automatically; no call-site change needed. `api.ts` carrying the three booleans on `InventorySlot` is the only prerequisite.

---

### `web/src/lib/examine.ts` (utility, transform — pure/node-tested)

**Analog:** same file — `examineFields` flags/wiki fields (65-110) + `wikiHref` (117-123); sibling `items.ts:facetItems` (S-4).

**D-02 flag chip** — add a new `kind` to `ExamineField` and a chip field after `name`. The `flags` field (lines 71-75) is the precedent for a conditional uppercase label field:
```ts
// 2. Flags — the is_quest_item badge ...
if (slot.is_quest_item) {
	fields.push({ kind: 'flags', text: 'QUEST ITEM' });
}
// ← ADD a priority flag-chip field (NO-DROP / LORE / MAGIC) using the SAME priority as the tile:
//   compute the priority once (reuse flagPriority/flagColorFor), push { kind: 'flagchip', text: 'NO-DROP' } when present.
```
Extend the `kind` union (lines 22-30) with `'quests'` and `'flagchip'`. The priority resolver is the pure helper (S-4) shared with `PaperdollSlot`.

**D-04/D-05 named quests field** — add a `kind: 'quests'` field positioned AFTER `wiki` and BEFORE `lastsynced` (UI-SPEC line 211). It carries the `notes_link`-only list. Because `ExamineField.text` is a single string today, the quests field needs structured data — extend the interface with an optional `quests?: { quest_name: string; source_url: string }[]` (the `href?` optional-field precedent at line 33 shows the pattern of a kind-specific optional). Filter to `notes_link` only (D-06):
```ts
// notes_link-only filter (NEVER render the in_game_flag '[in-game QUEST flag]' pseudo-name):
const named = (slot.quest_links ?? []).filter((q) => q.source === 'notes_link');
if (named.length > 0) {
	fields.push({ kind: 'quests', text: 'Used in:', quests: named.map((q) => ({ quest_name: q.quest_name, source_url: q.source_url })) });
}
```
Omit the field entirely when zero `notes_link` quests (D-09 graceful-omission — the same `if (...)` guard every other field uses). The `wikiHref` (117-123) is the URL-derivation precedent; here the URL is the stored `source_url`, run through `safeHttpUrl` at the render sink (S-3).

---

### `web/src/lib/components/ExaminePanel.svelte` (component, render)

**Analog:** same file — the `{#each fields as f (f.kind)}` switch (54-76), the `.ex-wiki` `{@html}` sink (60-62, 40-45), and the `:global(.tooltip-wiki-link)` style (167-172).

Add `{:else if f.kind === 'flagchip'}` and `{:else if f.kind === 'quests'}` branches to the `{#each}` switch. For quests, either (a) extend `composeItemNote` to emit escaped `<a>`s through the one `{@html}` sink (preferred — single escaping chokepoint, S-3), or (b) render natively with `{#each f.quests}` + `<a href={safeHttpUrl(q.source_url)}>{q.quest_name}</a>` (Svelte auto-escapes the name; href passes `safeHttpUrl` first — no `{@html}` needed). The flag chip is a bordered text chip colored `var(--flag-color)` (UI-SPEC lines 198-206). Reuse the existing `.tooltip-wiki-link` link styling (167-172) for the quest `<a>` — a quest link and a wiki link look identical (`color: var(--accent)` + 1px accent underline). Add an `.ex-flagchip` style mirroring `.ex-flags` (112-120) but with `color: var(--flag-color)` + a 1px `var(--flag-color)` bordered transparent chip.

> The component sets `--flag-color` from the priority resolver (same value the tile uses) — pass it as an inline style on the chip element, mirroring how PaperdollSlot sets it inline.

---

### `web/src/lib/tooltip/composeNotes.ts` + `web/src/lib/components/ItemTooltip.svelte` (utility + component)

**Analog:** same files — the `Used in quests:` block (`composeNotes.ts:154-161`) + the wiki-link composition (108-120) + `ItemTooltip`'s `:global(.tooltip-wiki-link)` style (166-171).

**composeNotes.ts** — `QuestLinkForNote` (42-45) gains `source_url: string`. The existing notes_link block already filters + caps + escapes; the ONLY change is rendering each name as a clickable `<a>` when a URL is present:
```ts
export interface QuestLinkForNote {
  quest_name: string;
  source: 'in_game_flag' | 'notes_link';
  // ← ADD: source_url: string;
}
// the existing block (154-161) — change the name-mapping to emit <a> via S-3:
const noteLinks = questLinks.filter((l) => l.source === 'notes_link').slice(0, MAX_QUEST_LINKS_IN_NOTE);
if (noteLinks.length > 0) {
  const names = noteLinks.map((l) => {
    const url = safeHttpUrl(l.source_url);
    const name = escapeHtml(l.quest_name);
    return url
      ? `<a class="tooltip-quest-link" href="${escapeHtml(url)}" target="_blank" rel="noopener">${name}</a>`
      : name; // plain escaped text when no/blocked URL (D-05 fallback)
  }).join(', ');
  parts.push(`<p class="tooltip-quests">Used in quests: ${names}</p>`);
}
```
The `notes_link`-only filter, the `MAX_QUEST_LINKS_IN_NOTE = 5` cap (line 47), the `Used in quests:` label, and the `escapeHtml` discipline are all UNCHANGED (UI-SPEC line 224). This stays the existing single `{@html}` sink — vitest can assert the `<a>` is escaped + only emitted for `notes_link` rows (the `composeItemNote` security tests are the precedent).

**ItemTooltip.svelte** — add a `.tooltip-popover :global(.tooltip-quest-link)` rule mirroring `.tooltip-wiki-link` (166-171): `color: var(--accent); border-bottom: 1px solid var(--accent); text-decoration: none;`. The `questLinks` prop (35) and `composeItemNote(... questLinks)` call (46) already thread the list through — once `QuestLinkForNote` carries `source_url` and the API response includes it, no prop-wiring change is needed.

> **Tooltip data source:** the Inventory tab tooltip is fed from `ItemRollup` (which now carries `quest_links` with `source_url` per S-2); the Wishlist tooltip is likewise fed from its row's quest links. The callers that build `questLinks` props must map the new `source_url` through (grep the `ItemTooltip` call-sites that pass `questLinks=`). The same generic `Quest item` badge (composeNotes.ts:150-152) stays the fallback when only `in_game_flag` exists (D-06).

---

## No Analog Found

None. Every file extends an existing, tested in-repo code path. The planner should NOT reach for RESEARCH.md patterns (there is no RESEARCH.md for this phase) — the in-repo precedents above are authoritative.

---

## Metadata

**Analog search scope:** `internal/backendsrv/store/`, `internal/backendsrv/compute/`, `internal/backendsrv/enrich/`, `internal/backendsrv/migrations/`, `web/src/lib/`, `web/src/lib/components/`, `web/src/lib/theme/`, `web/src/lib/tooltip/`, `web/src/lib/__tests__/`, `web/src/app.css`
**Files scanned:** ~22 (4 backend Go source + 3 backend tests + 1 migration + 8 web source + 2 web tests + app.css + grep sweeps for `source_url`/`is_no_drop`/flag-token references)
**Pattern extraction date:** 2026-06-25
