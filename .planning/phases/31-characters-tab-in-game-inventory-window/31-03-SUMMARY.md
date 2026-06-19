---
phase: 31-characters-tab-in-game-inventory-window
plan: 03
subsystem: web-characters-tab
tags: [sveltekit, svelte5-runes, typed-fetch, roster, viewer-first, scoped-search, master-detail, node-tested-pure-helper]

# Dependency graph
requires:
  - phase: 31-characters-tab-in-game-inventory-window (plan 31-02)
    provides: "GET /api/v1/characters (viewer-first band-tagged roster: name/level/race/class/is_mine/is_bank_toon/is_guild_bot/last_seen) + GET /api/v1/inventory/{char} (CharacterInventory incl. per-slot icon_id + per-char last_seen), both RequireSession-gated"
  - phase: 30-app-shell-5-tab-navigation
    provides: "the /characters route slot inside SiteShell (5-tab strip; shell-main 32px/16px gutters) + AuthGate context (AUTH_GUARD_KEY / AuthGuard) the page routes 401/403 to"
  - phase: 27-my-characters-inventory-filter
    provides: "the myview.ts pure-helper precedent (DOM-free, node-testable, type-only import from api.ts) that roster.ts mirrors"
provides:
  - "web/src/lib/api.ts: RosterCharacter/InventorySlot/CharacterInventory interfaces (snake_case Go contract) + fetchCharacters() + fetchInventory(char) credentialed wrappers (reused by 31-04 + P33)"
  - "web/src/lib/roster.ts: pure node-testable viewer-first sort (viewerFirst/bandOf) + viewer-priority search (filterRoster) — the D-10 contract"
  - "web/src/routes/characters/+page.svelte: the real Characters tab (3-band viewer-first list + scoped search + ?c=<name> selection wiring) replacing the Phase-30 placeholder"
affects: [31-04]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Typed credentialed fetch wrapper = getJSON<T> + snake_case interface mirroring the Go JSON contract (fetchMeta/ViewRow precedent); char path-param encodeURIComponent'd"
    - "Pure viewer-first sort/filter extracted to a plain .ts (node-testable; myview.ts precedent) — the DOM render stays a browser-smoke gap"
    - "Master-detail on ONE route: selection URL-reflected via ?c=<name> (history.replaceState), NOT N per-character route files (consolidated-views relaxation)"

key-files:
  created:
    - web/src/lib/roster.ts
    - web/src/lib/__tests__/roster.test.ts
  modified:
    - web/src/lib/api.ts
    - web/src/routes/characters/+page.svelte

key-decisions:
  - "Reused the existing PriceDetail interface (api.ts:79 — {direction,a30,t30}) for InventorySlot.prices rather than redefining it (deviation_protocol: reuse existing TS interfaces)"
  - "?c=<name> selection reflected via history.replaceState (no $app/navigation goto — the project has zero existing goto usage; replaceState avoids any full navigation and matches the guild-views window.location.search read idIom)"
  - "Dropped the role=list/role=listitem overlay on the selectable <button> rows — aria-pressed is the load-bearing selection a11y signal per UI-SPEC §I.6/§I.7, and listitem rejects aria-pressed (Rule 1 fix to clear the a11y warnings and satisfy the §I contract)"
  - "DOM is NOT browser-verified in this plan — the list/search/selection render is DOM-blind under node vitest; verification is folded into 31-04's deploy-then-browser-smoke gate"

patterns-established:
  - "A node-tested pure roster helper (roster.ts) holds ALL the testable logic (viewer-first banding + tie-break + viewer-priority filter); +page.svelte imports it so the D-10 contract is asserted in CI even though the render is DOM-blind"

requirements-completed: [CHAR-01, CHAR-02, CHAR-03]

# Metrics
duration: 7min
completed: 2026-06-18
---

# Phase 31 Plan 03: Characters Tab — List + Scoped Search + Selection Wiring Summary

**The SvelteKit Characters tab over the Plan 31-02 read API: typed `fetchCharacters`/`fetchInventory` wrappers + `RosterCharacter`/`CharacterInventory`/`InventorySlot` interfaces in `api.ts`, a pure node-tested `roster.ts` (viewer-first banding + viewer-priority search), and the rebuilt `/characters` page — a bespoke 3-band viewer-first list, a scoped search, and `?c=<name>` selection wiring that drives the inventory window (the window component itself lands in Plan 31-04, so the window column prompts "Pick a character" until then).**

## Performance

- **Duration:** ~7 min
- **Started:** 2026-06-18T08:02:23Z
- **Completed:** 2026-06-18T08:09:03Z
- **Tasks:** 2
- **Files modified:** 4 (2 created, 2 modified)

## Accomplishments
- **`api.ts` typed fetch layer** — `RosterCharacter` (name/level/race/class + `is_mine`/`is_bank_toon`/`is_guild_bot` band flags + `last_seen`), `InventorySlot` (incl. `icon_id`, `children`, `canonical_slot`, `last_listed`, reusing the existing `PriceDetail` for `prices`), and `CharacterInventory` (`{char, last_seen, equipment[], general[], bank[]}`) — each mirroring the Plan 31-02 Go snake_case contract. Two credentialed wrappers over the existing private `getJSON<T>`: `fetchCharacters()` → `/api/v1/characters`, and `fetchInventory(char)` → `/api/v1/inventory/${encodeURIComponent(char)}` (names are guildie-controlled). No hand-rolled fetch — `getJSON` carries `credentials:'include'` + typed `Unauthenticated`/`Forbidden` the AuthGate re-routes on.
- **`roster.ts` pure helper (node-tested)** — `bandOf` (D-10 band classification, `is_mine` WINS the bank/bot tie-break), `viewerFirst` (stable mine → guild → banks, A-Z case-insensitive within each band, returns a new array), `filterRoster` (case-insensitive substring filter that PRESERVES viewer-first ranking among matches; empty/whitespace → full set, no-match → `[]`). Pure + DOM-free (type-only import from `api.ts`), mirroring the `myview.ts` precedent.
- **`roster.test.ts`** — 14 node cases covering bandOf (mine/banks/guild + the is_mine tie-break over bank/bot), viewerFirst (band order, A-Z within band, viewer-owned bank toon ranks in mine, no-mutation), filterRoster (empty → full viewer-first, whitespace-only → full, case-insensitive substring keeping viewer-first ranking among matches, no-match → `[]`, no-mutation).
- **`/characters/+page.svelte`** — replaced the Phase-30 "coming soon" placeholder with the real tab (Svelte 5 runes): a `$state` `status` machine + `onMount` one-shot `fetchCharacters()` load (401/403 → `getContext` AuthGuard, else error StateBlock + Retry); a scoped search input (`--panel`/`--border`, 44px, leading `Search` lucide glyph, "Search characters…" + "Your characters match first." hint) bound to `query`; `$derived` `filterRoster(roster, query)` → grouped into the three D-10 bands (`YOUR CHARACTERS`/`GUILD`/`BANKS & BOTS`, empty bands omitted); each row a `<button aria-pressed>` with graceful D-11 meta (`Level {N} {Race} {Class}`, missing tokens dropped — never "null"), `yours`/`bank`/`bot` tags, and a selected accent left-border + name; selection sets `selected` + reflects `?c=<name>` via `history.replaceState` (and pre-selects from `?c=` on mount); the window column shows the §K "Pick a character" prompt until a character is selected, then a temporary `Selected: {name}` marker (the `InventoryWindow` mounts here in 31-04). StateBlock loading/error/empty/no-results reused verbatim; theme tokens only; NO raw-HTML directive (T-31-10/11).

## Web Checks (run before this SUMMARY)

All in `web/` (the established green-gate set):

| Command | Result |
|---------|--------|
| `npm test -- roster` (Task 1 gate) | **PASS** — 1 file, 14 tests |
| `npm run check` (svelte-check, after Task 2 a11y fix) | **PASS** — 500 files, **0 errors, 0 warnings** |
| `npm test` (full suite, after Task 2) | **PASS** — 26 files, **331 tests** (incl. the 14 new roster cases) |
| `npm run build` (adapter-static) | **PASS** — built in 9.74s, site written to `build/` |

Acceptance greps on `+page.svelte`: `fetchCharacters` ✓, `filterRoster` ✓, `StateBlock` ✓, the three band labels ✓, `Search characters…` ✓, `Pick a character` ✓, `?c=`/`encodeURIComponent` ✓, `aria-pressed` ✓; **NO `{@html}` directive** (the two earlier comment mentions of the token were reworded so the threat-model grep is unambiguous) ✓; **NO literal hex color** outside comments ✓; the `class="placeholder"` "coming soon" card is gone ✓.

## Task Commits

1. **Task 1: api.ts typed fetch + interfaces + pure roster helper (node-tested)** — `d53fe6f` (feat)
2. **Task 2: /characters tab — list + scoped search + ?c= selection wiring** — `72ec4ab` (feat)

## Files Created/Modified
- `web/src/lib/api.ts` — added the `RosterCharacter`/`InventorySlot`/`CharacterInventory` interfaces (reusing the existing `PriceDetail`) + `fetchCharacters()`/`fetchInventory(char)` wrappers (after `fetchMeta`).
- `web/src/lib/roster.ts` — NEW: pure `bandOf`/`viewerFirst`/`filterRoster` (D-10) + the `Band` type.
- `web/src/lib/__tests__/roster.test.ts` — NEW: 14 node cases (banding, tie-break, A-Z, filter, immutability).
- `web/src/routes/characters/+page.svelte` — REPLACED the placeholder with the real tab (list + search + selection + window-slot prompt).

## Decisions Made
- **Reused `PriceDetail`** (the existing `api.ts:79` `{direction,a30,t30}` interface) for `InventorySlot.prices` instead of redefining it — per the deviation protocol's "reuse existing TS interfaces if present."
- **`history.replaceState` for `?c=` reflection** (not `$app/navigation`'s `goto`) — the codebase has zero existing `goto`/`replaceState`/`pushState` usage; `replaceState` reflects the selection without any full navigation or a per-character route file, and reads back via `new URLSearchParams(window.location.search)` exactly as `guild-views` reads `?view=`.
- **Selection a11y via `aria-pressed` on a plain `<button>`** (the load-bearing UI-SPEC §I.6/§I.7 signal) — the initial draft also carried `role="list"`/`role="listitem"`, which conflicts with `aria-pressed` and triggered svelte-check a11y warnings; removed the role overlay (Rule 1) so the page is 0 errors / 0 warnings AND honors the §I contract.
- **The DOM is NOT browser-verified in this plan** — node vitest is DOM-blind; the list/search/selection render verification is deferred to 31-04's deploy-then-browser-smoke gate (the window and the list ship to prod together).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Removed conflicting `role="list"`/`role="listitem"` from the selectable rows**
- **Found during:** Task 2 (`npm run check`)
- **Issue:** The first draft wrapped the band list in `role="list"` and gave each `<button>` `role="listitem"` + `aria-pressed`. svelte-check flagged two a11y violations: a `<button>` cannot take `role="listitem"`, and `aria-pressed` is not supported by the `listitem` role — which would silently break the UI-SPEC §I.6/§I.7 "selected character announced via `aria-pressed`" contract.
- **Fix:** Dropped both `role` attributes; the `<button type="button" aria-pressed={…}>` is the correct selectable affordance per §I and announces selection cleanly.
- **Files modified:** `web/src/routes/characters/+page.svelte`
- **Verification:** `npm run check` → 0 errors / 0 warnings.
- **Committed in:** `72ec4ab` (Task 2 commit)

**2. [Rule 1 - Bug] Reworded comments so the literal `{@html}` token does not appear in the file**
- **Found during:** Task 2 (acceptance grep)
- **Issue:** Two explanatory comments mentioned the literal `{@html}` token; the plan's acceptance + the threat model (T-31-10) grep for the *absence* of `{@html}`, so a naive `grep "@html"` would false-positive on the comments.
- **Fix:** Reworded the comments ("raw-HTML directive" / "escaped-HTML sink") so no `@html` substring remains; the file genuinely contains no raw-HTML directive (names render via plain `{}` auto-escape).
- **Files modified:** `web/src/routes/characters/+page.svelte`
- **Verification:** `grep -c "@html"` → 0; `npm run check` re-green.
- **Committed in:** `72ec4ab` (Task 2 commit)

**Total deviations:** 2 auto-fixed (both Rule 1 — a11y correctness + grep-clean XSS-gate evidence). No architectural changes, no scope creep; both stay within the plan's named `+page.svelte`.

## Issues Encountered
None blocking. The private `getJSON<T>` in `api.ts` is module-scoped (not exported) — the two new wrappers live in `api.ts` itself (the established pattern), so no export change was needed.

## Known Stubs
- **The window column** renders the §K "Pick a character" prompt (no selection) or a temporary `Selected: {name}` marker (selection wired) — this is the **intended, plan-scoped** boundary: the actual `<InventoryWindow>` component lands in Plan 31-04 (CHAR-03's window render). The selection + `fetchInventory` wiring is in place (the `fetchInventory` wrapper + `?c=<name>` reflection are exercised), so 31-04 drops in the window over a fully-wired selection. `fetchInventory` is defined but not yet *called* in this plan (31-04 calls it on selection) — documented here as the deliberate hand-off, not a dead stub.

## Threat Flags
None. The page introduces no new network/auth/file/schema surface beyond the two read routes the plan's `<threat_model>` already registers (T-31-10..13). Names render via plain `{}` auto-escape (T-31-10), the search query is echoed only through `StateBlock kind="no-results"` `{query}` (T-31-11, no `{@html}`), `fetchCharacters`/`fetchInventory` ride `getJSON`'s `credentials:'include'` + typed 401/403 (T-31-12 — the server `RequireSession` is authoritative, the client filter is presentation only), and `?c=<name>` is `encodeURIComponent`'d into the query + passed to `fetchInventory` (which re-encodes for the path), never concatenated into markup (T-31-13).

## User Setup Required
None. No external service configuration. The tab consumes the Plan 31-02 routes, which go live on 31-04's backend deploy; the web build ships in 31-04's web deploy. Manual browser-smoke (deferred to 31-04) will verify: list orders viewer → guild → banks/bots; search prioritizes the viewer; selecting a row reflects `?c=` and shows the window slot.

## Next Phase Readiness
- The typed fetch layer (`fetchCharacters`/`fetchInventory` + `CharacterInventory`/`InventorySlot`/`RosterCharacter`) and the wired `/characters` master pane are in place. **Plan 31-04** drops the `<InventoryWindow>` (paperdoll + general/bank grids + inline bag expand + icons + hover/pin examine) into the window column over the already-wired `selected`/`?c=` selection + the `fetchInventory(char)` call, then runs the mandatory **deploy-then-browser-smoke** (node vitest is DOM-blind for everything rendered).
- `InventorySlot` (incl. `icon_id`/`children`) and `CharacterInventory.last_seen` are already typed for the 31-04 window + examine "Last synced" footer.

## Self-Check: PASSED

- Created/modified files verified present on disk: `web/src/lib/roster.ts`, `web/src/lib/__tests__/roster.test.ts`, `web/src/lib/api.ts`, `web/src/routes/characters/+page.svelte`, this SUMMARY.
- Both task commits verified in git log: `d53fe6f` (Task 1), `72ec4ab` (Task 2).
- Web gates green: `npm run check` (0 errors / 0 warnings, 500 files), `npm test` (331 tests, 26 files, incl. 14 new roster cases), `npm run build` (success). No file deletions in either commit (`git diff --diff-filter=D HEAD~1` empty per task).

---
*Phase: 31-characters-tab-in-game-inventory-window*
*Completed: 2026-06-18*
