# Phase 27: My-Characters Inventory Filter - Research

**Researched:** 2026-06-08
**Domain:** SvelteKit 5 (runes) client-side view filtering over an existing @tanstack/table-core DataGrid; reuse of the Phase 26 assignment read API
**Confidence:** HIGH (the entire surface is in-repo, read directly; no external/library uncertainty)

## Summary

Phase 27 is a **purely client-side `web/` feature with ZERO backend change and ZERO schema/migration work.** Every input it needs already exists and is live in production: the consolidated view payloads (`fetchView`/`fetchGearCheck`/`fetchSpellCheck`/`fetchBank`, all session-gated, all returning the SAME all-members rows to every caller) and the authoritative "which characters are mine" answer (`fetchMyCharacters()` → `MyCharacter[]`, backing `GET /api/v1/assignments/mine`, RequireSession, shipped + deployed live in Phase 26). The job is to add an ADDITIVE quick-filter and a single-character drill-down over the existing `+page.svelte` four-view surface and the ONE reusable `DataGrid.svelte`.

The architecture is forgiving here precisely BECAUSE the read API is not row-scoped: every member already fetches the identical all-members view rows (the API does `RequireSession` for auth, not per-caller row filtering). That makes the "my characters" filter structurally incapable of being access control — it can only narrow what THIS browser displays, never what another member can fetch. The CLAUDE.md consolidated-views LOCK is preserved by construction: no new tabs, no per-character view tabs, no second grid — one filter predicate over the existing in-memory rows.

**Primary recommendation:** Add a "my characters" toggle + single-character `<select>` drill-down in `+page.svelte`, backed by a PURE, node-testable predicate helper in a new `web/src/lib/myview.ts` (the `$lib/assignments.ts` precedent). Filter the four already-fetched row arrays by the set of assigned character NAMES (the join key is the `char` string column — `MyCharacter.name` matches `ViewRow.char`) BEFORE passing `data` to the existing `DataGrid`. Fetch `fetchMyCharacters()` once alongside the existing `Promise.all`. The DataGrid, columns.ts, sort state, and search all stay untouched.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| "Which characters are mine?" | API / Backend (Phase 26, shipped) | — | Authoritative assignment data; `GET /api/v1/assignments/mine`, session-derived owner (never trust the client for identity) |
| All-members view row data | API / Backend (Phase 14, shipped) | — | `/api/v1/views/*` returns identical all-members rows to every session — already live, untouched |
| "My characters" filter predicate | Browser / Client (NEW) | — | Pure presentation narrowing of already-fetched rows; NOT access control (the API already gave the client every row) |
| Single-character drill-down | Browser / Client (NEW) | — | Selecting one of the caller's assigned chars = a further client-side narrowing of the same in-memory rows |
| The grid (sort/filter/render) | Browser / Client (Phase 14, shipped) | — | The one reusable `DataGrid` is fed pre-filtered `data`; it needs no awareness of "mine" |

**Load-bearing point:** because the row data is NOT server-scoped per caller, the filter lives entirely in the browser and is correct-by-construction as "additive convenience, not access control." Do not add any server-side row filtering — it would contradict the consolidated all-members LOCK and is explicitly out of scope.

## Standard Stack

No new dependencies. Everything is already installed and in use.

### Core (all already present)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Svelte | 5 (runes: `$state`/`$derived`/`$props`) | Reactive client state for the toggle + selected char | Already the whole `web/` stack `[VERIFIED: web/src reads]` |
| @tanstack/table-core | (in `web/package.json`) | The headless grid engine behind `DataGrid.svelte` | The ONE reusable grid, instantiated 4x (CLAUDE.md LOCKED) `[VERIFIED: DataGrid.svelte:23-32]` |
| SvelteKit | (static adapter, `prerender=false` data pages) | The route/page shell (`+page.svelte`) | Existing product page `[VERIFIED: web/src/routes/+page.svelte]` |
| vitest | (node project, no jsdom) | Unit-test the pure filter predicate | `web/vite.config.ts` runs `environment:node`, excludes `*.svelte.{test,spec}` `[VERIFIED: web/vite.config.ts]` |

### Supporting (already present)
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| @lucide/svelte | (in use) | Icons (a filter/user glyph for the toggle, if desired) | Match the existing `Search`/`Settings` icon idiom |

**Installation:** None. `npm install` is not required for this phase.

**Version verification:** No package version research needed — the phase adds no dependency. (Confirmed by the locked decision "Read-only — no schema change, no new backend write" and verified against the existing import graph.)

## Architecture Patterns

### System Architecture Diagram

```
                       Discord session cookie (Domain=squirebot.quest)
                                       │ credentials:'include'
                                       ▼
  ┌──────────────────────────────  +page.svelte (the product page)  ──────────────────────────────┐
  │                                                                                                 │
  │   onMount → Promise.all([                                                                        │
  │       fetchView(), fetchGearCheck(), fetchSpellCheck(), fetchBank(), fetchMeta(),               │
  │       fetchBankToons(),                                                                          │
  │       fetchMyCharacters()   ◄── NEW (Phase 26 API, already live)                                │
  │   ])                                                                                             │
  │        │                              │                                                          │
  │        │ all-members rows             │ MyCharacter[] → Set<string> of assigned char NAMES       │
  │        ▼                              ▼                                                          │
  │   viewRows / gearRows / spellRows / bankRows          myCharNames + mineOnly + selectedChar      │
  │        │                                                       │ (client $state)                 │
  │        └──────────────►  applyMyFilter(rows, names, mineOnly, selectedChar)  ◄── NEW pure helper │
  │                                       │  ($lib/myview.ts — node-tested)                          │
  │                                       ▼                                                          │
  │                          $derived filtered row arrays                                            │
  │                                       │                                                          │
  │                                       ▼                                                          │
  │                      <DataGrid data={filtered} columns={…} defaultSorting={…} />  (UNCHANGED)    │
  └─────────────────────────────────────────────────────────────────────────────────────────────────┘

  Toggle "My characters" (off by default → all-members visible)  ─┐
  <select> drill-down (one of MY assigned chars)                  ─┴─► drive the pure predicate
```

The filter is a transform applied to `data` BEFORE the grid. The grid's own per-column filters, global "Filter all columns" box, sort, and the SearchBox keep working — they operate on whatever `data` they receive. "My characters" is just a coarser pre-narrowing.

### Component Responsibilities

| File | Phase 27 role |
|------|---------------|
| `web/src/routes/+page.svelte` | EDIT — add `fetchMyCharacters()` to the `Promise.all`; add `mineOnly` (bool) + `selectedChar` (string\|null) `$state`; add the toggle + `<select>` controls; pass `$derived` filtered arrays into the four `<DataGrid>` instances |
| `web/src/lib/myview.ts` | NEW — pure helpers: `myCharNameSet(MyCharacter[]) → Set<string>`, `applyMyFilter<T extends {char:string}>(rows, names, mineOnly, selectedChar) → T[]`. DOM-free, node-testable |
| `web/src/lib/__tests__/myview.test.ts` | NEW — node vitest over the pure helper (the `assignments.test.ts` pattern) |
| `web/src/lib/api.ts` | NO EDIT — `fetchMyCharacters()` + `MyCharacter` already exist (api.ts:832-908) |
| `web/src/lib/components/DataGrid.svelte` | NO EDIT — fed pre-filtered `data` |
| `web/src/lib/columns.ts` | NO EDIT — column defs unchanged (all four views already carry a `char`/`Char` column) |

### Pattern 1: Pure predicate in `$lib/*.ts`, thin wiring in `.svelte` (the repo's testability spine)
**What:** Keep ALL decision logic (which rows survive the filter) in a plain `.ts` exporting pure functions; the `.svelte` only holds `$state` and renders.
**When to use:** Always in this repo — `web/` vitest is **node-only with NO jsdom and NO @testing-library/svelte** (toolchain-install rule), so a `.svelte` file cannot be imported by a node test, and DOM behavior is invisible to `npm test`.
**Example (the established precedent — DO mirror this exactly):**
```typescript
// Source: web/src/lib/assignments.ts (Phase 26, in-repo)
// Pure, DOM-free decision helpers … extracted to a plain .ts (NOT a .svelte
// module export) so they're unit-testable under the repo's node vitest project.
export function partitionClaimable(claimable: ClaimableCharacter[]): { … } { … }
```
The new `applyMyFilter` follows the same shape. The test (`myview.test.ts`) mirrors `assignments.test.ts` (in-repo, verified).

### Pattern 2: Filter `data` upstream of the reusable grid (never fork the grid)
**What:** The DataGrid is headless and data-agnostic; pass it `data={mineOnly ? filtered : all}`. Never add a "mine" concept inside `DataGrid.svelte` or `columns.ts`.
**When to use:** Here — it keeps the ONE-reusable-grid contract intact (CLAUDE.md LOCKED) and means the grid's sort/filter/search are untouched.
**Example (the in-repo precedent — WantlistPanel feeds the grid a client-derived `data` + reactive `columns`):**
```svelte
<!-- Source: web/src/lib/components/WantlistPanel.svelte (in-repo) -->
{#if wants.length === 0}
    <StateBlock kind="no-wants" />
{:else}
    <DataGrid data={wants} {columns} {defaultSorting} />
{/if}
```
Phase 27 does the same but with `data={filteredViewRows}` where `filteredViewRows` is `$derived` from the pure helper.

### Pattern 3: Default OFF — the toggle starts "all members" (additive, not restrictive)
**What:** `let mineOnly = $state(false)` — the page loads showing the existing all-members views unchanged; the member opts INTO the narrower "my characters" view.
**When to use:** Required by MYVIEW-01 ("WITHOUT changing the existing all-members visibility … any member can still toggle back to all members"). The default state is the existing behavior verbatim.

### Anti-Patterns to Avoid
- **Per-character view tabs / a second grid:** FORBIDDEN (CLAUDE.md LOCKED; would re-introduce the 200-tab rationale and breach the consolidated-views architecture). The drill-down is a `<select>` that narrows the SAME grid, not a new tab.
- **Server-side row scoping:** Do NOT add a `?mine=1` query param or a per-caller filtered endpoint. The rows are intentionally all-members; the filter is a client convenience. A server filter would BE access control, which the locked decision explicitly excludes ("ADDITIVE — not an access-control restriction").
- **Joining on character ID instead of name:** The view rows key on the `char` string (`ViewRow.char`, `GearCheckRow.char`, `SpellCheckRow.char`), NOT a numeric character_id. `MyCharacter` carries both `character_id` and `name`. The grid rows have only the name. Join on NAME. (See Pitfall 1.)
- **Putting the toggle inside DataGrid's toolbar:** Keep the my-characters controls in `+page.svelte` (the view-orchestration layer), not inside the reusable grid — the grid must stay view-agnostic and reusable by the wantlist too.
- **Optimistic/derived security assumptions:** The toggle is UX. It never hides a row from another member's fetch. Do not document or imply it as privacy/access control.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| "Which characters are mine" | A new endpoint, a client-side owner-guess, or a join over `meta`/`owner_id` | `fetchMyCharacters()` (api.ts:906) → `GET /api/v1/assignments/mine` | The authoritative, session-scoped, IDOR-safe Phase-26 read; already deployed live |
| Filtering/sorting the grid | A bespoke filtered list | The existing `DataGrid` fed pre-filtered `data` | Sort, per-column filters, sticky Char column, a11y all already solved |
| Testing the filter logic | A `.svelte` component test (impossible in node) | A pure helper in `$lib/myview.ts` + node vitest | `web/` vitest is node-only (no jsdom); see Pitfall 2 |
| The view payloads | Re-fetching per character | The single `Promise.all` already in `+page.svelte:99` | Data is tiny (~12 members); fetch once, filter in memory |

**Key insight:** This phase is almost entirely composition of shipped parts. The ONLY genuinely new code is ~one pure helper + a toggle/select + an extra entry in the existing `Promise.all`. Resist scope creep into new endpoints or grid changes.

## Common Pitfalls

### Pitfall 1: Joining "my characters" to view rows by the WRONG key
**What goes wrong:** `MyCharacter` has `{ character_id, name, … }`; view rows (`ViewRow`/`GearCheckRow`/`SpellCheckRow`) have a `char` STRING and NO character_id. If the plan assumes a numeric id join, nothing matches and the filter silently returns zero rows.
**Why it happens:** The assignment API is id-centric (correct for mutation/IDOR safety), but the view layer is name-centric (the `Char` column is a display string, mirroring the v1 Sheet).
**How to avoid:** Build `Set<string>` of `MyCharacter.name` and test `set.has(row.char)`. Decide case-sensitivity explicitly — character names are stable in-game proper nouns; an exact match is correct, but document it. The pure helper makes this one tested line.
**Warning signs:** "My characters" toggle yields an empty grid even though the member clearly owns visible characters.

### Pitfall 2: "Green node tests, broken DOM" (this exact gap bit Phase 26 + Phase 15)
**What goes wrong:** `npm test` passes (165+ green) while the actual toggle/drill-down is broken in the browser — because the node vitest project has NO jsdom and CANNOT see `.svelte` rendering or events. Memory entry `web-tests-node-only-blind-to-dom` records P15 shipping 2 crashing BLOCKERs under all-green tests; P26's panels' rendering stayed a flagged browser-smoke gap.
**Why it happens:** `web/vite.config.ts` runs only the `server` project (`environment:node`, excludes `*.svelte.{test,spec}`). Component interaction is structurally untestable in CI.
**How to avoid:** (1) Put the filter LOGIC in `$lib/myview.ts` so the testable surface is maximized (node tests prove the predicate). (2) Explicitly FLAG the toggle wiring, the `<select>` drill-down, the default-off behavior, and the "toggle back to all" round-trip as a **browser-smoke / `/gsd-ui-review` gap** — they are NOT covered by `npm test`. (3) Browser-smoke must be done against a deployed build or a full local stack (the memory entry `web-local-dev-cant-auth-against-prod`: `npm run dev` hits the live API and Discord login bounces to prod, so a logged-in smoke needs deploy-then-smoke OR a seeded local backend).
**Warning signs:** A plan that claims the phase is "verified" on green `npm test` alone — insufficient for any DOM behavior here.

### Pitfall 3: Filtering breaking the SearchBox or the empty-state copy
**What goes wrong:** The cross-character SearchBox (`+page.svelte:172`) runs over `viewRows`. If "my characters" filtering replaces the array the SearchBox sees, search silently becomes "search MY items" — possibly desired, possibly a surprise. Also, the `noCharacters` / per-view `view-empty` StateBlocks key off row counts; a filter that empties the grid needs a "no rows for your characters" affordance distinct from "no data at all."
**Why it happens:** Multiple consumers read the same row arrays.
**How to avoid:** Decide deliberately (flag as an open question for the plan): does "my characters" narrow the SearchBox too, or only the grid? Recommended: keep SearchBox over the FULL `viewRows` (search is explicitly "across the guild" per its copy) and apply the my-filter only to the grid `data`. For the empty case, add a distinct StateBlock variant or inline note ("None of your characters have rows in this view") so the member doesn't read it as "no data."
**Warning signs:** "Filter all columns" box or search behaving inconsistently between views; an ambiguous empty grid.

### Pitfall 4: The drill-down `<select>` listing characters the member doesn't own
**What goes wrong:** Populating the single-character drill-down from `meta.characters` (all guild chars) instead of `myCharacters` lets a member "drill into" a character they don't hold — contradicting MYVIEW-02 ("drill into a single specific ASSIGNED character").
**Why it happens:** `meta` is already fetched and tempting as the char list source.
**How to avoid:** The drill-down options come ONLY from `fetchMyCharacters()`. (Reading another member's single character is still possible via the un-narrowed all-members grid — that's fine and intended; the DRILL-DOWN affordance is scoped to mine.)
**Warning signs:** The drill-down `<select>` shows characters not in "My characters."

### Pitfall 5: Member with zero assigned characters
**What goes wrong:** A member who hasn't claimed any characters toggles "my characters" → empty everything, with no guidance.
**Why it happens:** Assignment is opt-in (Phase 26 self-claim); a fresh member may hold none.
**How to avoid:** When `myCharacters.length === 0`, either disable the toggle with a hint ("Claim characters on My characters to use this filter") linking to `/my-characters`, or show a clear empty affordance. Decide in the plan.
**Warning signs:** Confused empty grid for new members.

## Code Examples

Verified patterns from in-repo sources (these are the exact seams to extend).

### The existing parallel fetch to extend (add ONE entry)
```typescript
// Source: web/src/routes/+page.svelte:99-113 (in-repo)
const [v, g, s, b, m, bt] = await Promise.all([
    fetchView(), fetchGearCheck(), fetchSpellCheck(), fetchBank(), fetchMeta(), fetchBankToons()
]);
// Phase 27: add fetchMyCharacters() to this array; assign myCharacters = mc.
```

### The assignment read wrapper (already exists — just import it)
```typescript
// Source: web/src/lib/api.ts:832-908 (in-repo)
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

### The pure helper to ADD (shape mirrors $lib/assignments.ts)
```typescript
// NEW: web/src/lib/myview.ts (proposed)
import type { MyCharacter } from './api';

/** The set of the caller's assigned character NAMES — the join key against the
 *  `char` string every view row carries (ViewRow/GearCheckRow/SpellCheckRow). */
export function myCharNameSet(mine: MyCharacter[]): Set<string> {
    return new Set(mine.map((c) => c.name));
}

/** Narrow any char-bearing rows to the caller's characters. `mineOnly` off →
 *  the rows pass through UNCHANGED (additive default, MYVIEW-01). When a single
 *  `selectedChar` is set (the drill-down, MYVIEW-02) it dominates: only that
 *  char's rows survive. Pure + DOM-free → node-testable. */
export function applyMyFilter<T extends { char: string }>(
    rows: T[],
    mineNames: Set<string>,
    mineOnly: boolean,
    selectedChar: string | null
): T[] {
    if (selectedChar) return rows.filter((r) => r.char === selectedChar);
    if (!mineOnly) return rows;
    return rows.filter((r) => mineNames.has(r.char));
}
```

### Feeding the grid pre-filtered data (the existing call site, made reactive)
```svelte
<!-- Source pattern: web/src/routes/+page.svelte:195 + WantlistPanel.svelte (in-repo) -->
{#if filteredViewRows.length === 0}
    <StateBlock kind="view-empty" viewName="inventory" />  <!-- or a "none of your chars" variant -->
{:else}
    <DataGrid data={filteredViewRows} columns={viewColumns} defaultSorting={SORT.view} />
{/if}
<!-- where: let filteredViewRows = $derived(applyMyFilter(viewRows, mineNames, mineOnly, selectedChar)); -->
```

### The node test to ADD (mirrors assignments.test.ts)
```typescript
// NEW: web/src/lib/__tests__/myview.test.ts (proposed)
// Source pattern: web/src/lib/__tests__/assignments.test.ts (in-repo)
import { describe, it, expect } from 'vitest';
import { myCharNameSet, applyMyFilter } from '../myview';
// assert: mineOnly=false → passthrough; mineOnly=true → only my-named rows;
// selectedChar set → only that char; empty mine + mineOnly → []; name-join correctness.
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Google Sheet per-character `view:<Char>` tabs (rejected) | ONE consolidated filterable DataGrid with a `Char` column | v2.0 (Phase 14) | Per-char tabs are FORBIDDEN; the filter narrows the single grid `[VERIFIED: CLAUDE.md, DataGrid.svelte:1-21]` |
| No assignment concept | `character_assignment` layer + `GET /assignments/mine` | v2.3 Phase 26 (shipped 2026-06-08) | The "mine" set is now an authoritative API call, not a guess `[VERIFIED: api.ts, main.go:368]` |
| Top-bar nav clutter | Gear `SettingsMenu` dropdown | quick 260607-sdh (2026-06-08) | `/my-characters` lives in the gear menu; the inventory filter is a control ON the views page, distinct from that management surface `[VERIFIED: SettingsMenu.svelte:189-191]` |

**Deprecated/outdated:** Nothing this phase touches is deprecated. The `@tanstack/table-core` + local `createSvelteTable` adapter is the current, Svelte-5-correct path (NOT the Svelte-4-only TanStack wrapper — `DataGrid.svelte:6-7`).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The join key between "my characters" and view rows is the character NAME (`MyCharacter.name` ↔ `ViewRow.char`), and names are unique + stable enough for exact-match | Pitfall 1, Code Examples | If a member has two characters with name collisions, or if name normalization differs between the assignment table and the view builder, the filter mis-includes/excludes rows. LOW risk (in-game names are unique proper nouns), but the planner should confirm the view builder emits the same `name` string the assignment store stores. |
| A2 | Per-view filtering applies uniformly to all four views (view/gear/spell/bank), all of which carry a `char` column | Architecture, Code Examples | All four DO carry `char` (verified in columns.ts). If a future view lacks it the generic `<T extends {char:string}>` would not compile for it — acceptable, surfaces at build time. |
| A3 | The SearchBox should keep searching the FULL guild (not narrow with the filter) | Pitfall 3 | If the user expects "my characters" to also scope search, this is a UX miss — flag as an open question for discuss/plan, cheap to flip. |
| A4 | Default-off toggle satisfies "without changing existing all-members visibility" | Pattern 3 | None material — default-off IS the existing behavior verbatim. |

**If the user confirms A1/A3 in discuss-phase, both become locked.** A1 is the one worth a 30-second confirmation against the view builder; A3 is a product call.

## Open Questions

1. **Does "my characters" also narrow the cross-guild SearchBox, or only the grid?**
   - What we know: SearchBox copy says "across the guild"; it reads `viewRows`.
   - What's unclear: user intent when the my-filter is on.
   - Recommendation: keep search guild-wide (only narrow the grid `data`); flag for the plan to confirm.

2. **Where do the controls live — toggle + single-character `<select>` together, or a unified character `<select>` with an "All my characters" + "All members" option set?**
   - What we know: MYVIEW-01 wants a quick-filter (mine vs all); MYVIEW-02 wants single-char drill-down.
   - Recommendation: ONE `<select>` is the simplest surface — options: `All members` (default) · `My characters` · then each of my chars by name. That single control satisfies both requirements with no separate toggle. Confirm in plan/discuss.

3. **Empty-state copy for "filter on, zero matching rows" and "member owns zero characters."**
   - Recommendation: distinct affordances (Pitfall 5); decide in plan.

4. **Case-sensitivity / normalization of the name join (ties to A1).**
   - Recommendation: exact match; confirm the view builder and assignment store agree on the stored name casing.

## Environment Availability

> Skipped — Phase 27 is a code-only `web/` change with no new external dependency. The two data sources it consumes (`/api/v1/views/*`, `/api/v1/assignments/mine`) are already live in production (verified: routes registered in `cmd/squirebot-server/main.go`; the assignment API deployed 2026-06-08 per STATE.md). The existing `web/` toolchain (Node/SvelteKit/vitest) is already installed and green (287 tests per Phase 26).

## Security Domain

> `security_enforcement: true`, ASVS level 1.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V1 Architecture | yes | The filter is client-side UX, NOT a security boundary — documented as such (matches `$lib/assignments.ts` posture: "never a security boundary"). The real gate is the API's `RequireSession`. |
| V2 Authentication | no (reused) | Session is already established by AuthGate + the cross-subdomain cookie; this phase adds no auth surface. |
| V4 Access Control | yes (NEGATIVE requirement) | The filter MUST NOT be presented or implemented as access control. It cannot hide a row from another member's fetch (the API returns all-members rows to every session regardless of this UI). No new authz decision is made client-side. |
| V5 Input Validation | yes (low) | The only "input" is the selected character name driving a client-side `Array.filter` — no injection sink. Character names already render via plain `{}` auto-escaping everywhere (XSS-safe, the repo-wide rule); the filter compares strings, never renders raw HTML. |
| V6 Cryptography | no | None. |

### Known Threat Patterns for {SvelteKit static client + Go read API}

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Treating the client filter as a privacy/access control (info-disclosure false sense) | Information Disclosure | Document explicitly that it is additive UX; the server is the only gate. No row is server-withheld, so there is nothing to leak that the API didn't already authorize for this session. |
| XSS via a malicious character name in the drill-down `<select>` / labels | Tampering / XSS | Render names via plain `{}` (Svelte auto-escapes) — NEVER `{@html}`. The repo's standing rule (MyCharactersPanel T-26-16, SettingsMenu T-15-22). |
| IDOR — a member filtering to another member's characters | Elevation / Access Control | Drill-down options come ONLY from `fetchMyCharacters()` (session-scoped, IDOR-safe server-side). The all-members grid intentionally already shows everyone (by design, not a leak). |

**Net security posture:** Phase 27 adds NO new server endpoint, NO new write, NO new authz decision. The single residual control is "never frame the filter as access control" (V4 negative requirement) and "names render escaped" (already the repo norm). This is among the lowest-risk possible phases.

## Sources

### Primary (HIGH confidence) — all in-repo, read directly this session
- `CLAUDE.md` — consolidated-views LOCK, web build/test conventions, "never per-character view tabs"
- `web/src/routes/+page.svelte` — the four-view product page, the `Promise.all`, the SearchBox + StateBlock wiring
- `web/src/lib/components/DataGrid.svelte` — the ONE reusable grid (headless table-core), data-agnostic
- `web/src/lib/columns.ts` — all four views carry a `Char`/`char` column; the join key
- `web/src/lib/api.ts:832-918` — `MyCharacter`, `fetchMyCharacters()`, `fetchClaimable()` (already shipped)
- `web/src/lib/assignments.ts` + `web/src/lib/__tests__/assignments.test.ts` — the pure-helper + node-test precedent to mirror
- `web/src/lib/components/WantlistPanel.svelte` — precedent for feeding the grid a client-derived `data` + reactive columns
- `web/src/lib/components/MyCharactersPanel.svelte` / `web/src/routes/my-characters/+page.svelte` — the Phase-26 MANAGEMENT surface (distinct from this VIEW/FILTER surface)
- `web/src/lib/components/SettingsMenu.svelte` — nav/IA (where `/my-characters` lives)
- `web/src/lib/auth.ts` + `AuthGate.svelte` — session model (the filter rides existing auth; adds none)
- `web/vite.config.ts` — node-only vitest project (no jsdom) → the testability constraint
- `cmd/squirebot-server/main.go:286-370` — view routes + `/api/v1/assignments/mine` both `RequireSession`, both live
- `internal/backendsrv/webadmin/assignment.go` — `ListMyAssignmentsHandler` (session-scoped, IDOR-safe)
- `.planning/ROADMAP.md` (Phase 27), `.planning/REQUIREMENTS.md` (MYVIEW-01/02), `.planning/STATE.md` (Phase 26 shipped + UAT-verified)

### Secondary (MEDIUM confidence)
- Project memory `web-tests-node-only-blind-to-dom`, `web-local-dev-cant-auth-against-prod` — the browser-smoke gap + how to smoke a web change (deploy-then-smoke or full local stack)

### Tertiary (LOW confidence)
- None — no external/web research was needed; the entire surface is in-repo and verified.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependency; every part read in-repo and confirmed live.
- Architecture: HIGH — the data flow (filter `data` upstream of the reusable grid) is the existing WantlistPanel pattern; the assignment read is shipped + deployed.
- Pitfalls: HIGH — the name-vs-id join (Pitfall 1) and the node-only test blindness (Pitfall 2) are both verified against the actual code and project memory.
- Security: HIGH — the negative access-control requirement and the escaped-name rule are both already-established repo norms.

**Research date:** 2026-06-08
**Valid until:** 2026-07-08 (stable — in-repo surface, no fast-moving external deps; only invalidated if the view builder's `char` field semantics or the assignment API shape change)
