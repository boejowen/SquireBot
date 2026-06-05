---
phase: 19-wantlist-crud
plan: 03
subsystem: ui
tags: [sveltekit, wantlist, datagrid, catalog-search, debounce, xss-escape, in-guild-holders, svelte-check]
requires:
  - phase: 19-wantlist-crud (Plan 02)
    provides: "wantlist + item-search JSON contracts: GET/POST /api/v1/wantlist, POST /api/v1/wantlist/remove, GET /api/v1/items/search"
  - phase: 19-wantlist-crud (Plan 01)
    provides: "wantlist_item store + CatalogItem + ErrDuplicateWant (409 surfaced to the UI)"
provides:
  - "The /wantlist SvelteKit route (login-gated) — add-item block above a DataGrid of the owner's wants"
  - "api.ts typed wrappers: WantlistRow + CatalogItem interfaces + fetchOwnWants / searchCatalog / addWant / removeWant"
  - "DOM-free node-testable logic: holdersFor (reduce-by-char-and-SUM) + priorityRank / noteRuneCount"
  - "wantlistColumns + prioritySort + the no-wants StateBlock kind"
  - "WantAddForm (debounced catalog search + custom escape hatch) + WantlistPanel (server-truth-reload grid) + Wantlist nav link"
affects:
  - "Phase 20 alert pipeline (the custom-want 'won't trigger alerts' flag is the UI contract for which wants do/don't page Discord)"
tech-stack:
  added: []  # zero new dependencies — composes existing DataGrid/ItemTooltip/ConfirmDialog/StateBlock/FormField
  patterns:
    - "DOM-free logic modules (holders.ts/priority.ts) with node vitest tests — the formatLastSeen precedent — keep render-independent rules node-testable despite vitest being DOM-blind"
    - "holdersFor REDUCES the raw per-row `view` payload by char and SUMS counts (Map<string,number>), so each character renders on EXACTLY ONE ↳ line — never the map-not-reduce double-count shape"
    - "Server-truth reload after every add/remove (Promise.all([fetchOwnWants(), fetchView()])) — never optimistic-mutate the grid"
    - "Debounced (~250ms) catalog search so the input does not fire per-keystroke (DoS guard, paired with the server's len(q)>=2)"
    - "Zero {@html} on user-controlled data (item names / custom labels / notes via plain {} auto-escape); the only {@html} sink stays ItemTooltip→composeItemNote"
    - "Mandatory browser-smoke checkpoint after node-green — the P15 '165 green tests, 2 crashing BLOCKERs' node-blind lesson"
key-files:
  created:
    - web/src/lib/wantlist/holders.ts
    - web/src/lib/wantlist/holders.test.ts
    - web/src/lib/wantlist/priority.ts
    - web/src/lib/wantlist/priority.test.ts
    - web/src/lib/components/WantAddForm.svelte
    - web/src/lib/components/WantlistPanel.svelte
    - web/src/routes/wantlist/+page.svelte
  modified:
    - web/src/lib/api.ts
    - web/src/lib/columns.ts
    - web/src/lib/components/StateBlock.svelte
    - web/src/lib/components/SiteShell.svelte
key-decisions:
  - "holdersFor reduces-by-char-and-SUMS (Map<string,number>), honoring review MUST-FIX 1 — the `view` payload is raw per-row (one ViewRow per location/stack), so a single character recurs for one item_id and counts MUST sum; superseded RESEARCH.md's older map-only sketch"
  - "The in-guild indicator reads 'In guild' / 'Not in guild' (review MUST-FIX 3 relabel) — never the stale 'In bank' wording; the badge word is always present so color is never the only signal"
  - "Zero {@html} on user data (review-honored XSS boundary T-19-13) — item names / custom labels / notes render via plain {} auto-escape; only ItemTooltip keeps a {@html} sink"
  - "Catalog search debounced ~250ms (review-honored T-19-14) — one network call per pause, not per keystroke"
  - "Add/remove always re-fetch from the server (authoritative grid, T-19-16) — never optimistic; remove gated by ConfirmDialog (Cancel default-focused)"
patterns-established:
  - "DOM-free wantlist logic module + node vitest: render-independent rules (group-by-char-sum, rank, rune-count) live outside .svelte so they are unit-testable despite the DOM-blind harness"
  - "Browser-smoke checkpoint on the LIVE prod deploy as the verification gate for frontend plans — node green is necessary but not sufficient"
requirements-completed: [WANT-01, WANT-02]
duration: ~2 sessions (paused at the blocking browser-smoke checkpoint between Task 2 and approval)
completed: 2026-06-05
---

# Phase 19 Plan 03: Wantlist /wantlist Page Summary

**The SvelteKit /wantlist surface — a debounced catalog-search add form (with a custom-want escape hatch flagged "won't trigger alerts"), a server-truth DataGrid of the owner's wants with the deep "In guild" holder display (one summed ↳ line per character), and ConfirmDialog removal — composed from existing components atop DOM-free reduce-by-char-and-SUM logic. DEPLOYED + browser-verified live at squirebot.quest/wantlist (schema v6).**

## Performance

- **Duration:** ~2 sessions (Tasks 1 & 2 executed, then paused at the blocking browser-smoke checkpoint until live-prod approval)
- **Tasks:** 3 (Task 1 + Task 2 auto, Task 3 human-verified)
- **Files modified:** 11 (7 created, 4 modified)

## Accomplishments

- **`/wantlist` route shipped and login-gated** — the add-item block above the 5th DataGrid instantiation, behind the existing Discord-login AuthGate, with a Wantlist nav link in SiteShell.
- **Typed api.ts wrappers** — `WantlistRow` + `CatalogItem` interfaces and `fetchOwnWants` / `searchCatalog` / `addWant` / `removeWant`, cloned from the verbatim `/account/codes` twin; bodies carry NO owner (D-02 IDOR boundary; the session cookie is the identity).
- **DOM-free in-guild holder logic (review MUST-FIX 1 honored)** — `holdersFor` reduces the raw per-row `view` payload by character into a `Map<string,number>` and SUMS counts, so a character holding an item across multiple rows (e.g. worn + bank) renders ONE `↳ Char: <summed>` line, not duplicate `↳ Char: 1` lines.
- **Add flow** — debounced (~250ms) catalog search with ItemTooltip-on-hover results, a `Add "<query>" as a custom want` escape hatch staging an `item_id: null` want with the mandatory neutral "Custom — won't trigger alerts" chip, FormField Reason/Priority/Note detail with a live N/280 rune counter.
- **View + remove flow** — the grid renders "In guild" / "Not in guild" / "—" (custom) with the deep holder lines, and removal routes through ConfirmDialog → `removeWant` → server-truth re-fetch (never optimistic).
- **Live-prod browser-smoke APPROVED** — all 9 verification steps walked on squirebot.quest/wantlist after the Phase 19 prod deploy (backend binary + goose migration 00006 → schema v6; frontend bundle).

## Task Commits

1. **Task 1: api.ts wrappers + columns.ts wantlistColumns + StateBlock no-wants + DOM-free holders/priority logic** — `b5e775f` (feat)
2. **Task 2: WantAddForm + WantlistPanel + /wantlist page + SiteShell nav** — `44c7194` (feat)
3. **Task 3: MANDATORY browser-smoke of /wantlist** — human-verified/APPROVED on the LIVE prod deploy (no code commit — verification checkpoint)

_Note: Task 1 carried the tdd flag; api.ts/columns.ts/StateBlock plus the holders/priority modules and their node tests landed together as the single `b5e775f` feat commit._

## Files Created/Modified

**Created:**
- `web/src/lib/wantlist/holders.ts` — DOM-free `holdersFor(itemId, viewRows)`: null itemId → `[]` (custom want → "—"); else groups every ViewRow whose `id === itemId` by `char` into a `Map<string,number>`, SUMS counts, returns `{char,count}[]` sorted by `localeCompare`. Reduce-by-char, not map-not-reduce.
- `web/src/lib/wantlist/holders.test.ts` — 13-test node suite incl. the review-MUST-FIX-1 multi-row-same-char assertion (two count-1 rows for one char → a SINGLE `{char, count:2}` entry: `result.length === 1` && `result[0].count === 2`), the null-itemId → `[]` custom case, and the multi-character `localeCompare` ordering / distinct-char case.
- `web/src/lib/wantlist/priority.ts` — `priorityRank` (high=3/med=2/low=1/else→0) + `noteRuneCount` (`[...s].length`, matching the server's RuneCountInString N/280 display).
- `web/src/lib/wantlist/priority.test.ts` — rank mapping + ordering + the else→0 fallthrough.
- `web/src/lib/components/WantAddForm.svelte` — debounced `searchCatalog` add form, ItemTooltip results, custom escape hatch + "won't trigger alerts" chip, FormField Reason/Priority/Note with the live N/280 counter; dispatches success so the panel re-fetches.
- `web/src/lib/components/WantlistPanel.svelte` — `onMount`→`load()` (`Promise.all([fetchOwnWants(), fetchView()])`), StateBlock loading/error/no-wants phases, the 5th DataGrid fed `wantlistColumns` + the `[{id:'priority',desc:true},{id:'in_guild',desc:false}]` default sort, the in-guild cell via `holdersFor`, embedded WantAddForm, ConfirmDialog-gated remove, server-truth reload after every mutation.
- `web/src/routes/wantlist/+page.svelte` — the `.form-card` route shell (cloned from `routes/account/+page.svelte`) hosting `<WantlistPanel />`, behind the existing login-gated layout.

**Modified:**
- `web/src/lib/api.ts` — added `WantlistRow` + `CatalogItem` interfaces and the four cookie-credentialed wrappers (no owner in any body).
- `web/src/lib/columns.ts` — added `prioritySort` (cloned from `tierSort`, using `priorityRank`) and `wantlistColumns` in the UI-SPEC order (Priority · Item · Reason · In guild? · Note · Remove); `enableGlobalFilter: false` on the computed `in_guild` column + `item_id`.
- `web/src/lib/components/StateBlock.svelte` — added `| 'no-wants'` to the StateKind union + the cloned render branch ("Your wantlist is empty" / "Search the catalog above…").
- `web/src/lib/components/SiteShell.svelte` — added the `Wantlist` nav link inside the `session?.authenticated` block, styled like the existing `.char-meta-nav` entries (nav-only, no logic change).

## Decisions Made

- **holdersFor reduces-by-char-and-SUMS** (review MUST-FIX 1) — the raw `view` payload has no server GROUP BY, so the SUM is mandatory to avoid duplicated `↳` lines. RESEARCH.md's older map-only sketch (lines 502-514) was explicitly superseded.
- **"In guild" / "Not in guild" relabel** (review MUST-FIX 3) — never the stale "In bank" wording; the indicator word is always present so color is never the sole signal.
- **Zero {@html} on user data** (review-honored T-19-13 XSS boundary) — names / labels / notes via plain `{}` auto-escape; ItemTooltip remains the only `{@html}` sink.
- **Debounced search** (review-honored T-19-14) — ~250ms so it does not storm the catalog endpoint per keystroke.
- **Server-truth reload, ConfirmDialog remove** (T-19-16 / D-06) — the grid never optimistically mutates.

## Deviations from Plan

None — plan executed exactly as written. The two `type="auto"` tasks landed as committed; the blocking browser-smoke checkpoint paused execution between Task 2 and human approval (the intended gate, not a deviation).

## Issues Encountered

None during planned work. The blocking checkpoint was intentional: node vitest is DOM-blind (no jsdom / @testing-library — the toolchain-install rule), so green unit tests do not prove the page renders. Verification was therefore deferred to the live browser-smoke, which the human walked on the production deploy.

## Verification

- **node tests:** `npx vitest run src/lib/wantlist/` → 13 passed, including the review-MUST-FIX-1 multi-row-same-char summed-line assertion (`result.length === 1` && `count === 2`).
- **svelte-check:** `npx svelte-check --threshold error` → 0 errors (clean after both Task 1 and Task 2).
- **build:** `npm run build` → exit 0; the static build emitted the new `/wantlist` route.
- **XSS / relabel grep gates:** 0 `{@html}` in WantAddForm/WantlistPanel; ≥1 "In guild" and 0 "In bank" in WantlistPanel.
- **Live browser-smoke (Task 3) — APPROVED on prod (squirebot.quest/wantlist, schema v6):** all 9 steps confirmed — (1) Wantlist nav link, (2) "Your wantlist is empty" StateBlock, (3) debounced single-call catalog add with the N/280 counter, (4) the in-guild summed-holder display (one `↳ Char: <summed>` line for a multi-row character — MUST-FIX-1 verified live), (5) custom-want chip + "—" cell, (6) duplicate → friendly 409 / same-item-other-reason adds, (7) ConfirmDialog remove (Cancel focused, Esc dismisses), (8) `<b>x</b>` renders as literal text (escape holds), (9) owner-scoped (another guildie's list not visible/mutable).

## Threat Model Coverage

| Threat | Disposition | How met |
|--------|-------------|---------|
| T-19-13 Tampering (stored XSS) | mitigate | Names / labels / notes via plain `{}` auto-escape; zero `{@html}` on user data (grep-gated); only ItemTooltip keeps a sink. Browser-smoke step 8 confirmed `<b>x</b>` renders literal. |
| T-19-14 DoS (search storm) | mitigate | Catalog input debounced ~250ms; server guards `len(q)>=2` + LIMIT. Browser-smoke step 3 confirmed one network call, not per-keystroke. |
| T-19-15 EoP (IDOR) | mitigate | add/remove bodies carry NO owner (session-derived server-side, D-02); wrappers send `{item...}` / `{id}` only. Browser-smoke step 9 confirmed another guildie's list is unreachable. |
| T-19-16 Spoofing (stale/optimistic UI) | mitigate | Every add/remove re-fetches from the server (`fetchOwnWants` + `fetchView`); the grid never optimistically mutates. |

## Next Phase Readiness

- WANT-01 + WANT-02 delivered, deployed to prod, and live-verified — the wantlist CRUD surface is complete.
- Phase 20 (alert pipeline) can consume the `wantlist_item` rows and the custom-want "won't trigger alerts" flag is the established UI contract for which wants page Discord.

## Self-Check: PASSED

- Created/modified files exist: `holders.ts`, `holders.test.ts`, `priority.ts`, `priority.test.ts`, `WantAddForm.svelte`, `WantlistPanel.svelte`, `routes/wantlist/+page.svelte`, `api.ts`, `columns.ts`, `StateBlock.svelte`, `SiteShell.svelte` — all FOUND.
- Commits exist: `b5e775f`, `44c7194` — both FOUND.

---
*Phase: 19-wantlist-crud*
*Completed: 2026-06-05*
