# Phase 27: My-Characters Inventory Filter - Pattern Map

**Mapped:** 2026-06-08
**Files analyzed:** 4 (1 edit, 2 new, 0 backend/migration)
**Analogs found:** 4 / 4 (every new/modified file has an exact in-repo precedent)

> Purely client-side `web/` feature. NO backend, NO schema/migration, NO new dependency.
> The CLAUDE.md consolidated-views LOCK is preserved by construction: ONE reusable
> `DataGrid`, fed pre-filtered `data`. No new tabs, no per-character view tabs, no second grid.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `web/src/routes/+page.svelte` | route/page (view orchestrator) | request-response → client transform | itself (EDIT) + `WantlistPanel.svelte` (filter-data-upstream precedent) | exact (same file) |
| `web/src/lib/myview.ts` | utility (pure predicate) | transform (filter) | `web/src/lib/assignments.ts` | exact (same role + DOM-free + node-tested) |
| `web/src/lib/__tests__/myview.test.ts` | test (node vitest) | transform assertions | `web/src/lib/__tests__/assignments.test.ts` | exact |
| `web/src/lib/api.ts` | service (API wrapper) | request-response | **NO EDIT** — `fetchMyCharacters()` + `MyCharacter` already shipped (api.ts:832, 906) | reuse |

**NO-EDIT (fed pre-filtered data, must stay view-agnostic):** `DataGrid.svelte`, `columns.ts`.

## Pattern Assignments

### `web/src/routes/+page.svelte` (route/page, EDIT) — the four-view orchestrator

**Analog:** itself + `WantlistPanel.svelte` (the "derive `data` client-side, feed the grid" precedent).

**1. The `Promise.all` to extend — add ONE entry** (`+page.svelte:99-113`):
```typescript
const [v, g, s, b, m, bt] = await Promise.all([
    fetchView(),
    fetchGearCheck(),
    fetchSpellCheck(),
    fetchBank(),
    fetchMeta(),
    fetchBankToons()
]);
viewRows = v;
gearRows = g;
spellRows = s;
bankRows = b.rows;
meta = m;
bankToons = bt;
status = 'ready';
```
Phase 27: add `fetchMyCharacters()` as the 7th element; add `let myCharacters = $state<MyCharacter[]>([])` (mirror the `$state` declarations at `:85-94`); assign `myCharacters = mc`. Import `fetchMyCharacters, type MyCharacter` from `$lib/api` in the existing import block (`:21-33`).

**2. Client `$state` + `$derived` filter wiring** (mirror `noCharacters`/`coinToons` at `:145-151`):
```typescript
// existing precedent — a $derived narrowing of already-fetched rows
let coinToons = $derived(bankToons.filter(hasRecordedCoin));
```
Phase 27 adds (the research's recommended single-control surface, Open-Q2):
```typescript
let mineOnly = $state(false);              // default OFF — all-members visible (MYVIEW-01, Pattern 3)
let selectedChar = $state<string | null>(null);
let mineNames = $derived(myCharNameSet(myCharacters));
let filteredViewRows  = $derived(applyMyFilter(viewRows,  mineNames, mineOnly, selectedChar));
let filteredGearRows  = $derived(applyMyFilter(gearRows,  mineNames, mineOnly, selectedChar));
let filteredSpellRows = $derived(applyMyFilter(spellRows, mineNames, mineOnly, selectedChar));
let filteredBankRows  = $derived(applyMyFilter(bankRows,  mineNames, mineOnly, selectedChar));
```

**3. Feed the grid the FILTERED arrays** (the existing call sites at `:191-243`):
```svelte
{#if active === 'view'}
    {#if viewRows.length === 0}
        <StateBlock kind="view-empty" viewName="inventory" />
    {:else}
        <DataGrid data={viewRows} columns={viewColumns} defaultSorting={SORT.view} />
    {/if}
```
Phase 27: swap `data={viewRows}` → `data={filteredViewRows}` (and the other three). The empty-state guard should test the FILTERED count and add a distinct "none of your characters" affordance (Pitfall 3/5 — decide copy in plan). Keep the `SearchBox` at `:172` reading the FULL `viewRows` (search stays guild-wide, A3/Open-Q1).

**4. The control's place — in `+page.svelte`, NEVER inside DataGrid** (anti-pattern). It sits in the view-orchestration layer alongside the `view-nav` (`:176-188`) / `search-section` (`:171-173`), styled to match `.tab` / `.record-coin` (see Shared Patterns → EQ-theme select).

---

### `web/src/lib/myview.ts` (utility, NEW) — the pure filter predicate

**Analog:** `web/src/lib/assignments.ts` (exact — same role, DOM-free, node-tested, same header rationale).

**Header + import shape to copy** (`assignments.ts:1-16`):
```typescript
// Pure, DOM-free decision helpers … Extracted to a plain .ts (NOT a .svelte module
// export) so they're unit-testable under the repo's node vitest project: vite.config.ts
// runs the `server` project with environment:node … and EXCLUDES *.svelte.{test,spec}.ts.
import type { ClaimableCharacter } from './api';
```
Phase 27 imports `import type { MyCharacter } from './api';` and adds the same "filter is presentation, NOT a security boundary; the server's `RequireSession` is the gate" note (security V4 negative requirement).

**Function shape to copy** (the pure, single-responsibility export style of `partitionClaimable`, `assignments.ts:29-44`):
```typescript
export function partitionClaimable(claimable: ClaimableCharacter[]): {
    unassigned: ClaimableCharacter[];
    assignedToOthers: ClaimableCharacter[];
} {
    // … plain loop / filter, no DOM, no Svelte …
}
```
Phase 27 helpers (from RESEARCH Code Examples, verified against the row types):
```typescript
export function myCharNameSet(mine: MyCharacter[]): Set<string> {
    return new Set(mine.map((c) => c.name));   // join key = NAME (Pitfall 1)
}
export function applyMyFilter<T extends { char: string }>(
    rows: T[], mineNames: Set<string>, mineOnly: boolean, selectedChar: string | null
): T[] {
    if (selectedChar) return rows.filter((r) => r.char === selectedChar);
    if (!mineOnly) return rows;                // default OFF → passthrough (MYVIEW-01)
    return rows.filter((r) => mineNames.has(r.char));
}
```
**Join-key fact (Pitfall 1, verified):** `ViewRow`, `GearCheckRow`, `SpellCheckRow` ALL declare `char: string` (api.ts:93, 112, 123) and NO `character_id`. `MyCharacter` has both `character_id: number` and `name: string` (api.ts:832-838). Join on NAME (`set.has(row.char)`), never id. `bank` reuses `ViewRow` (api.ts:131), so the single generic `<T extends {char:string}>` covers all four.

---

### `web/src/lib/__tests__/myview.test.ts` (test, NEW) — node vitest

**Analog:** `web/src/lib/__tests__/assignments.test.ts` (exact — same project, same factory-fixture idiom).

**Fixture factory + describe/it shape to copy** (`assignments.test.ts:9-43`):
```typescript
import { describe, it, expect } from 'vitest';
import type { ClaimableCharacter } from '../api';
import { partitionClaimable, requestStatusLabel } from '../assignments';

function claimable(over: Partial<ClaimableCharacter> = {}): ClaimableCharacter {
    return { character_id: 1, name: 'Slampeach', ...over };
}
describe('partitionClaimable …', () => {
    it('puts no-assignee rows in unassigned …', () => { /* expect(…).toEqual(…) */ });
    it('an empty list partitions to two empty arrays', () => { … });
});
```
Phase 27 mirrors this with a `mine(over)` / `row(over)` factory and asserts the cases RESEARCH lists (myview.test scaffold): `mineOnly=false` → passthrough; `mineOnly=true` → only my-named rows; `selectedChar` set → only that char (dominates `mineOnly`); empty `mine` + `mineOnly=true` → `[]`; name-join exactness (a row whose `char` isn't in the set is excluded). Use real EQ proper-noun names (`'Slampeach'`) per the repo fixture convention.

---

## Shared Patterns

### Filter `data` upstream of the reusable grid (NEVER fork the grid)
**Source:** `web/src/lib/components/WantlistPanel.svelte:84-96, 231-235`
**Apply to:** `+page.svelte` (all four `<DataGrid>` instances)
```svelte
let columns = $derived(wantlistColumns(holdersByItem, …));   // client-derived inputs
…
{#if wants.length === 0}
    <StateBlock kind="no-wants" />
{:else}
    <DataGrid data={wants} {columns} {defaultSorting} />     // grid is data-agnostic
{/if}
```
WantlistPanel feeds the grid a CLIENT-DERIVED `data` (the owner's wants) + reactive `columns` — the exact precedent for `data={filteredViewRows}`. The grid stays headless; sort/per-column-filter/global-filter all keep working on whatever `data` arrives.

### The reusable grid is fed data, never told about "mine"
**Source:** `web/src/lib/components/DataGrid.svelte:1-13, 40-49`
**Apply to:** confirm NO edit to DataGrid / columns.ts
```svelte
<script lang="ts" generics="TData">
    // DataGrid — the ONE reusable grid … NEVER per-character tabs (CLAUDE.md LOCKED) …
    let { data, columns, defaultSorting = [{ id: 'char', desc: false }] }: {
        data: TData[]; columns: ColumnDef<TData, unknown>[]; defaultSorting?: SortingState;
    } = $props();
```
The grid's only inputs are `data` / `columns` / `defaultSorting`. The "mine" concept never crosses this boundary (anti-pattern: putting the toggle in DataGrid's toolbar).

### The authoritative "my characters" read (already shipped — import, don't build)
**Source:** `web/src/lib/api.ts:832-838, 905-908`
**Apply to:** `+page.svelte` (`Promise.all`) and `myview.ts` (`import type`)
```typescript
export interface MyCharacter {
    character_id: number;
    name: string;            // ← the join key against ViewRow.char
    discord_user_id: string;
    assigned_at: number;
    assigned_by: string;
}
/** GET /api/v1/assignments/mine → MyCharacter[] ([] when none). Login-only. */
export function fetchMyCharacters(f: typeof fetch = fetch): Promise<MyCharacter[]> {
    return getJSON<MyCharacter[]>('/api/v1/assignments/mine', f);
}
```
Session-scoped + IDOR-safe server-side (Phase 26). The drill-down `<select>` options come ONLY from this list (Pitfall 4) — never from `meta.characters`.

### EQ-theme `<select>` styling for the new drill-down control
**Source:** `web/src/lib/components/DataGrid.svelte:120-131 (markup)` + `:246-272 (CSS)`
**Apply to:** the new "my characters" control in `+page.svelte`
```svelte
<select class="facet" aria-label={`Filter by ${col.id}`} value={…} onchange={…}>
    <option value="">{col.id} (all)</option>
    {#each facetOptions(col.id) as opt (opt)}
        <option value={opt}>{opt}</option>
    {/each}
</select>
```
```css
.facet {
    border: 1px solid var(--border, var(--accent));
    border-radius: 4px;
    background: var(--panel);
    color: var(--text);
    font-family: var(--font-body);
    font-size: 16px;
    padding: 8px;
    min-height: 44px;            /* touch target */
}
.facet:focus-visible { outline: 2px solid var(--accent); outline-offset: 1px; }
```
The same design-system tokens (`--accent`, `--panel`, `--text`, `--font-display`, 44px touch target, `:focus-visible` accent outline) appear on `.tab` and `.record-coin` in `+page.svelte:258-357`. The single-`<select>` surface RESEARCH recommends (Open-Q2: options `All members` · `My characters` · then each of my chars by name) matches `.facet` exactly. **XSS:** render every char name via plain `{}` (Svelte auto-escapes), NEVER `{@html}` (security V5, repo standing rule).

### Default-OFF, additive, NEVER access control
**Source:** RESEARCH Pattern 3 + Security V4 (negative requirement)
**Apply to:** `+page.svelte` (`mineOnly` init) and `myview.ts` (header note)
- `let mineOnly = $state(false)` — page loads showing existing all-members views verbatim (MYVIEW-01).
- The filter narrows ONLY this browser's display; the API already returned every all-members row to this session. Do not add a `?mine=1` param or per-caller server filter (anti-pattern — that would BE access control, explicitly out of scope).

## No Analog Found

None. Every new/modified file has an exact in-repo precedent. (No external research, no new dependency.)

## Browser-Smoke Gap (carry into the plan — NOT covered by `npm test`)

`web/` vitest is **node-only, no jsdom, no @testing-library/svelte** (`web/vite.config.ts` runs only the `server` project; project memory `web-tests-node-only-blind-to-dom`). The node test proves `myview.ts` (the predicate) but CANNOT see the toggle/`<select>` wiring, default-off behavior, the "toggle back to all" round-trip, or the empty-state copy. The plan MUST flag these as a `/gsd-ui-review` + browser-smoke gap. Smoke requires deploy-then-smoke OR a full local stack (memory `web-local-dev-cant-auth-against-prod`: `npm run dev` hits the live API + Discord login bounces to prod).

## Metadata

**Analog search scope:** `web/src/routes/`, `web/src/lib/`, `web/src/lib/components/`, `web/src/lib/__tests__/`
**Files scanned/read:** `+page.svelte`, `assignments.ts`, `assignments.test.ts`, `WantlistPanel.svelte`, `api.ts` (832-928), `DataGrid.svelte`, `columns.ts`
**Pattern extraction date:** 2026-06-08
