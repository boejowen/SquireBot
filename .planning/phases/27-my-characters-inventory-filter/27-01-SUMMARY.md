---
phase: 27-my-characters-inventory-filter
plan: 01
subsystem: web
tags: [web, svelte5, consolidated-views, character-assignment, client-filter]
requires:
  - "GET /api/v1/assignments/mine (fetchMyCharacters — Phase 26, already live)"
  - "GET /api/v1/views/{view,gear_check,spell_check,bank} (Phase 14, already live)"
  - "web/src/lib/api.ts MyCharacter + *Row types (already shipped — imported, not edited)"
provides:
  - "web/src/lib/myview.ts — pure DOM-free myCharNameSet + applyMyFilter helpers"
  - "Additive 'My characters' quick-filter + single-char drill-down over the four consolidated views"
affects:
  - "web/src/routes/+page.svelte (the product page — the only file edited in Task 2)"
tech-stack:
  added: []
  patterns:
    - "Pure node-testable filter helper in a plain .ts (mirrors $lib/assignments) — the .svelte wiring stays a browser-smoke gap"
    - "Client-side $derived filtering UPSTREAM of the single reusable DataGrid (grid fed pre-filtered data, never forked)"
key-files:
  created:
    - web/src/lib/myview.ts
    - web/src/lib/__tests__/myview.test.ts
  modified:
    - web/src/routes/+page.svelte
decisions:
  - "ONE <select> (All members[default] / My characters / per-assigned-char) satisfies BOTH MYVIEW-01 and MYVIEW-02"
  - "Default mineOnly=false — all-members visibility unchanged for everyone; the filter narrows THIS browser only"
  - "Drill-down (selectedChar) DOMINATES mine-only in the predicate"
  - "SearchBox stays guild-wide (reads full viewRows, NOT the filtered arrays)"
  - "Case-insensitive join is a defensive belt — the name-join is byte-exact server-side"
metrics:
  duration: ~30m
  completed: 2026-06-08
  tasks: 2
  files-changed: 3
  commits: 3
---

# Phase 27 Plan 01: My-Characters Inventory Filter Summary

An additive, purely client-side "My characters" quick-filter + single-character drill-down over the four consolidated views (inventory / bank / gear / spell), delivered by one EQ-themed `<select>` in `+page.svelte` driving `$derived` filtered arrays through a pure, node-tested predicate (`web/src/lib/myview.ts`) — zero backend change, zero migration, the single reusable DataGrid untouched.

## What Was Built

- **`web/src/lib/myview.ts`** — two pure, DOM-free exports mirroring `$lib/assignments`:
  - `myCharNameSet(mine)` → a lower-cased `Set` of the caller's assigned character names (the join key against each view row's `char` string).
  - `applyMyFilter(rows, mineNames, mineOnly, selectedChar)` → drill-down dominates; `mineOnly=false` passes rows through UNCHANGED (additive default); `mineOnly=true` keeps only in-set rows. Generic over any `{ char: string }` row, so the one helper serves ViewRow / GearCheckRow / SpellCheckRow.
  - The header documents the load-bearing **negative** security property (T-27-01): the helper is presentation only, NEVER a security boundary — the server's `RequireSession` is the gate, the all-members rows are intentionally shown to every session, and no `?mine=1`/server scope was added.
- **`web/src/lib/__tests__/myview.test.ts`** — 9 node-vitest cases (factory fixtures, real EQ names `Slampeach`/`Findom`): passthrough, mine-only, drill-down-dominates (incl. drilling a non-mine char), empty-mine→`[]`, name-join exactness, case-insensitive defensive join, and a cross-row-shape (gear) case.
- **`web/src/routes/+page.svelte`** (ONLY file edited in Task 2):
  - `fetchMyCharacters()` added as the 7th `Promise.all` element → `myCharacters` state; `mineOnly=$state(false)` (default OFF) + `selectedChar=$state<string|null>(null)`.
  - `mineNames` + four `filtered*Rows` `$derived` arrays feed the four `<DataGrid>` instances.
  - One `<select>` (`All members` default / `My characters` / one option per assigned char) with `onchange` translating to the two primitives; `aria-label`, 44px touch target, `.facet`-style EQ tokens (`--panel`/`--text`/`--accent`/`--font-body`, `:focus-visible` outline).
  - Per-char options sourced ONLY from `myCharacters` (NOT `meta.characters` — Pitfall 4 / IDOR-safe, T-27-03); names render via plain `{}` auto-escape (T-27-02).
  - Zero-claimed member: the `My characters` option is disabled and a hint links to `/my-characters` (Pitfall 5 — never a dead toggle).
  - Each per-view empty guard tests the FILTERED count and shows a DISTINCT "None of your characters have rows in this view" note when the filter is active (Pitfall 3/5) rather than the generic all-members "no data" StateBlock.
  - SearchBox left reading the full `viewRows` — cross-guild search is NOT narrowed by the filter.

## Name-Join Verdict (Pitfall 1 / Assumption A1 — MANDATORY)

**Verified: the view builder and the assignment store both emit raw `character.name` — exact join correct; the case-insensitive predicate is defensive.**

- `internal/backendsrv/store/readviews.go:150` → `SELECT c.name, …` from `JOIN character c ON c.id = ii.character_id` — no UPPER/LOWER/trim/normalization on the emitted name.
- `internal/backendsrv/store/assignment.go:448` (`ListMyAssignments`) → `SELECT a.character_id, c.name, … JOIN character c ON c.id = a.character_id` — likewise raw. (The `ORDER BY c.name COLLATE NOCASE` affects only sort order, not the value emitted on the wire.)

So `MyCharacter.name` ≡ `ViewRow.char` byte-for-byte; an exact-match join is correct. The predicate's `.toLowerCase()` is a cheap belt-and-suspenders against any future casing drift — it cannot break the byte-exact case.

## Scope / Locks Held

- Changes confined to `web/src/` — **ZERO** files under `internal/`, `cmd/`, or `*.sql` (confirmed via `git status --porcelain`). No backend change, no migration, as the locked research requires.
- **`DataGrid.svelte`, `columns.ts`, and `api.ts` were NOT modified** (consolidated-views LOCK — the one reusable grid stays view-agnostic, fed pre-filtered data; verified empty `git status` on all three).
- No `?mine=1` query param or per-caller server filter added (the negative security property — this is UX, not access control).

## Phase Gate Results

- `cd web && npm run check` → **0 errors / 0 warnings** (484 files).
- `cd web && npm test` → **296 passed (23 files)**, including the new `myview.test.ts` (9 cases).
- `cd web && npm run build` → **succeeded** (`@sveltejs/adapter-static`, site written to `build/`).

## TDD Gate Compliance

- RED: `test(27-01)` commit `1d400bf` — test failed (module `../myview` missing) before the helper existed.
- GREEN: `feat(27-01)` commit `972da5c` — helper added, 9/9 green. No REFACTOR commit needed (helper landed clean to spec).

## Deviations from Plan

None — plan executed exactly as written. (One cosmetic in-comment rephrase: the literal token `{@html}` inside a "never use" warning comment was reworded to "the raw-HTML directive" so the acceptance grep `grep -c '{@html}' +page.svelte == 0` reads clean; no behavioral change — there was never an actual raw-HTML sink.)

## Open Gap — Browser Smoke + UI Review (NOT covered by `npm test`)

`web/` vitest is node-only (no jsdom / `@testing-library/svelte`; project memory `web-tests-node-only-blind-to-dom`). The node test proves the `myview.ts` predicate but is BLIND to the Task-2 Svelte wiring: the `<select>` render/onchange, default-off "All members", the toggle-back round-trip, the drill-down narrowing the grid, the distinct filter-active empty copy, and the zero-claimed disabled/hint state are ALL DOM behavior invisible to CI (this exact gap shipped 2 crashing BLOCKERs in P15).

**Required before calling Phase 27 verified:** run `/gsd-ui-review 27`, then browser-smoke against a DEPLOYED build or a full local stack (NOT `npm run dev` against prod — `web-local-dev-cant-auth-against-prod`: a logged-in smoke needs deploy-then-smoke OR a local backend with `SQUIREBOT_COOKIE_INSECURE` + `PUBLIC_API_BASE` + a seeded `sb_session`). Smoke checklist: (1) page loads showing ALL members by default; (2) "My characters" narrows all four grids to the caller's chars; (3) a single character drills into just that char; (4) "All members" restores the full grid; (5) the SearchBox still searches everyone with the filter on; (6) a zero-claimed member sees the disabled/hint affordance, not a dead grid; (7) character names in the `<select>` render escaped.

## Self-Check: PASSED

- `web/src/lib/myview.ts` — FOUND
- `web/src/lib/__tests__/myview.test.ts` — FOUND
- `web/src/routes/+page.svelte` — FOUND (modified)
- Commit `1d400bf` (test RED) — FOUND
- Commit `972da5c` (feat helper GREEN) — FOUND
- Commit `9bd86b2` (feat +page.svelte wiring) — FOUND
