---
phase: 31-characters-tab-in-game-inventory-window
plan: 04
subsystem: web-inventory-window
tags: [sveltekit, svelte5-runes, master-detail, paperdoll, examine, colored-tile-fallback, node-tested-pure-helper, xss-escaped-sink, deploy-pending]

# Dependency graph
requires:
  - phase: 31-characters-tab-in-game-inventory-window (plan 31-03)
    provides: "the /characters page (3-band viewer-first list + scoped search + ?c= selection) + api.ts fetchInventory(char)/CharacterInventory/InventorySlot interfaces — the window-slot region + the fetch seam 31-04 renders into"
  - phase: 31-characters-tab-in-game-inventory-window (plan 31-01)
    provides: "the per-slot icon_id + per-char last_seen JSON fields the window renders (Item_<iconId>.png + the examine 'Last synced')"
  - phase: 31-characters-tab-in-game-inventory-window (plan 31-02)
    provides: "the two session-gated read routes (GET /api/v1/characters + GET /api/v1/inventory/{char}) — live on this plan's deploy"
  - phase: 14 (composeNotes)
    provides: "composeItemNote/escapeHtml/safeHttpUrl — the ONE audited {@html} sink the examine wiki link routes through (T-31-14)"
provides:
  - "web/src/lib/examine.ts: pure node-tested examineFields (D-08 order + D-09 omission; last_seen NOT last_listed)"
  - "web/src/lib/components/PaperdollSlot.svelte: one 62px slot tile — Item_<iconId>.png over colored-tile onerror fallback (D-02) + count + bag marker + a11y"
  - "web/src/lib/components/ExaminePanel.svelte: the single pinned examine (D-08 rows; the one escaped composeItemNote {@html} sink)"
  - "web/src/lib/components/InventoryWindow.svelte: GENERIC prop-driven window over CharacterInventory (23-slot paperdoll + general/bank grids + inline bag expand + hover/pin) — REUSED by P33, feeds P34"
  - "web/src/routes/characters/+page.svelte: the window wired in over ?c= selection with per-character loading/error/no-inventory states"
affects: [33-banks-tab, 34-wishlist-rework]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Pure examine field order/omission extracted to a plain .ts (examine.ts) — node-tested (the myview.ts/roster.ts precedent), the panel DOM stays a browser-smoke gap"
    - "Generic prop-driven window over the compute JSON contract (CharacterInventory) — one component, reused per bank toon in P33 and as P34's equipped-slot source; no Characters-tab-only assumptions"
    - "One reusable grid renderer (a Svelte {#snippet}) for general + bank (D-05); inline bag expand as a full-row grid item (display:contents cell + grid-column 1/-1) — NOT a pop-out overlay (D-04)"
    - "Hover preview emitted by the slot's OWN <button> (onhover/onleave props) so each tile is a single interactive element (no nested-button a11y violation); the floating preview shares the examineFields body with the pin"

key-files:
  created:
    - web/src/lib/examine.ts
    - web/src/lib/__tests__/examine.test.ts
    - web/src/lib/components/PaperdollSlot.svelte
    - web/src/lib/components/ExaminePanel.svelte
    - web/src/lib/components/InventoryWindow.svelte
  modified:
    - web/src/routes/characters/+page.svelte

key-decisions:
  - "examineFields renders the stored wiki_summary as the single 'stats' block and omits the discrete DMG/DLY/AC/wt-size/class-race D-08 rows — the read contract exposes a prose summary, not discrete fields, so per D-09 the panel shows what's known and never fabricates a structured row"
  - "The examine wiki link is the ONLY {@html} this plan adds and routes through composeItemNote (item name + safeHttpUrl'd href, both escaped) — every other D-08 row renders via plain {} interpolation (Svelte auto-escapes); no other {@html} anywhere in the window/slot/page (T-31-14)"
  - "Hover preview is emitted by PaperdollSlot's own button via onhover/onleave callbacks (not a wrapping element) — keeps one interactive element per tile, avoids the nested-<button> a11y error, and the preview shares the examineFields body with the pin so they never drift"
  - "The window fetch is a window-scoped state machine on +page.svelte (winStatus/inv/invFor) driven by a $effect on `selected`; a captured-char stale-response guard (invFor !== char) drops a late response when the user re-selects; a 401/403 routes to the same AuthGate guard as the roster load"
  - "The .doll center figure shows the silhouette glyph + char name only — no fabricated AC/ATK (the contract exposes none; §E 'never a fabricated stat' / D-09)"

patterns-established:
  - "InventoryWindow is the canonical per-character master-detail drill-down (consolidated-views lock RELAXED) — a single reusable component rendered on selection, NOT N materialized routes"

requirements-completed: [INV-01, INV-02, INV-03, INV-04]

# Metrics
duration: 12min
completed: 2026-06-18
---

# Phase 31 Plan 04: In-Game Inventory Window Summary

**The in-game-style inventory window (INV-01..04): a pure node-tested `examine.ts` (the D-08 field order + D-09 omission, `last_seen` NOT `last_listed`), a `PaperdollSlot` tile (wiki `Item_<iconId>.png` over a deterministic colored-tile `onerror` fallback + stack count + bag marker + a11y), an `ExaminePanel` (the D-08 rows with the single escaped `composeItemNote` `{@html}` sink), and a GENERIC prop-driven `InventoryWindow` (23-slot paperdoll + general/bank grids on one renderer + INLINE bag expand + hover-preview/click-to-pin examine) — wired into `/characters` over the 31-03 `?c=` selection with per-character loading/error/no-inventory states. CODE COMPLETE + web gates green; the mandatory backend+migration+web DEPLOY and the browser-smoke are PENDING (the Task-3 human-verify checkpoint — node vitest is DOM-blind).**

## Performance

- **Duration:** ~12 min (code tasks; deploy + browser-smoke pending)
- **Started:** 2026-06-18T08:14:20Z
- **Tasks:** 2 code tasks complete (Task 3 = the deploy + browser-smoke checkpoint, NOT executed here)
- **Files modified:** 6 (5 created, 1 modified)

## Accomplishments
- **`examine.ts` (pure, node-tested)** — `examineFields(slot, charLastSeen)` builds the LOCKED D-08 field list (`name → flags → slot → stats → price → wiki → lastsynced`) and OMITS any field whose source is empty (D-09 — never a blank/"null"/"—" row). The name is always first and always present; the PigParse price line is dropped entirely when `price === null`; the wiki href is the stored `wiki_url` or a derived P1999 page URL; "Last synced" uses the per-CHARACTER `charLastSeen` (`CharacterInventory.last_seen`), explicitly NOT the per-slot `last_listed` (the price last-listed date — 31-RESEARCH Pitfall 2). Discrete DMG/DLY/AC/wt-size/class-race rows are omitted because the read contract exposes a prose summary, not discrete fields (D-09 "show what's known").
- **`examine.test.ts`** — 14 node cases: name-always-first (incl. a bare + a sourceless slot), `price === null` omits the price (and no literal "null"), `charLastSeen === ''` omits last-synced, blank `wiki_summary`/`canonical_slot`/`is_quest_item=false` omit their rows, the full D-08 relative order, omitted-fields-collapse-without-reordering, and the load-bearing "last-synced uses `charLastSeen`, NOT `slot.last_listed`" case (distinct values; the price date appears nowhere).
- **`PaperdollSlot.svelte`** — a 62×62 prop-driven tile: a FILLED slot is a `<button>` with an item-naming `aria-label`; an EMPTY equipment slot is a non-interactive labelled tile (D-11). The icon is `Item_${iconId}.png` (`iconId` an integer — T-31-15) over a deterministic per-item `hsl()` gradient under-layer; `onerror` hides the `<img>` so the colored tile shows through, and `iconId === 0` skips the `<img>` entirely (D-02). A stack-count badge (`count > 1`, tabular-nums, `var(--text)` + shadow) and a `⊞` bag marker (`slots > 0`, accent) with `aria-expanded`. Hover/focus → accent border + box-shadow + `scale(1.04)` (reduced-motion honored). It emits `onpin`/`onopen`/`onhover`/`onleave` so the parent owns the pin/open/preview decisions.
- **`ExaminePanel.svelte`** — the single pinned panel: when `slot === null` the dimmed "Click an item…" prompt; else the `examineFields()` rows in D-08 order — name (accent Heading), flags (`--status-other`), stats (`--status-ok`), price (tabular-nums `--status-other`), last-synced (dimmed footer), all via plain `{}` interpolation; the wiki link is the ONE escaped `{@html}` sink, `composeItemNote(item, safeHttpUrl(wiki_url || wikiUrlFor(item)), null, [], [])` (T-31-14).
- **`InventoryWindow.svelte`** — a GENERIC prop-driven component over one `CharacterInventory` (no Characters-tab-only assumptions — P33 reuses it per bank toon, P34 reads its equipped slots). Char-head → the 23-slot paperdoll (§E left col: Ear1/Head/Face/Ear2/Neck/Shoulders/Arms/Back · center `.doll` · right col: Wrist1/Wrist2/Hands/Finger1/Finger2/Chest/Legs/Feet · WORN row: Charm/Primary/Secondary/Range/Ammo/Waist/Power) matched to `equipment[]` by `canonical_slot` (empty positions kept, D-11) → `GENERAL INVENTORY` + `BANK — STORED ITEMS` grids over ONE reusable `{#snippet}` renderer (D-05) → the hint line. Clicking a container (`slots > 0`) toggles an INLINE expand in-flow (full-row grid item, `{Bag} — {used} of {slots} slots` + child sub-grid; empty bag → "Empty") — NOT a pop-out overlay (D-04). A filled non-container hover shows a transient `pointer-events:none` preview (Esc/leave-dismiss, sharing the `examineFields` body); a click PINS into the sticky right-column `ExaminePanel` (single, replace-on-click — D-07). `inventory.last_seen` is the examine "Last synced" for every item. Two-pane desktop (`1fr 290px`), ≤900px the panel drops below, tiles stay 62px.
- **`/characters/+page.svelte` wiring** — a window-scoped state machine (`winStatus`/`inv`/`invFor`) driven by a `$effect` on `selected`: `fetchInventory(selected)` with `StateBlock kind="loading"`/`kind="error"`+Retry inside the window column; a captured-char stale-response guard; a 401/403 routes to the shared AuthGate guard. When the fetched inventory has zero items everywhere → the §K "No inventory synced yet" block (D-11); else `<InventoryWindow inventory={inv} />`. The §K "Pick a character" prompt stays for `selected === null`.

## Web Checks (run before this SUMMARY)

All in `web/` (the established green-gate set):

| Command | Result |
|---------|--------|
| `npm test -- examine` (Task 1 gate) | **PASS** — 1 file, 14 tests |
| `npm run check` (svelte-check, after Task 1) | **PASS** — 504 files, 0 errors, 0 warnings |
| `npm run check` (svelte-check, after Task 2) | **PASS** — 505 files, **0 errors, 0 warnings** |
| `npm test` (full suite, after Task 2) | **PASS** — 27 files, **345 tests** (331 prior + 14 new examine) |
| `npm run build` (adapter-static) | **PASS** — built in 7.40s, site written to `build/` |

**Acceptance greps (verified):**
- `ExaminePanel.svelte`: contains `composeItemNote` + `safeHttpUrl`; exactly ONE `{@html}` (the composed escaped body) ✓.
- `PaperdollSlot.svelte`: contains `Item_${`, `onerror`, `aria-label`, `var(--accent)`; no literal hex outside the locked `hsl()` fallback (the count badge is `var(--text)` + shadow) ✓.
- `InventoryWindow.svelte`: contains `PaperdollSlot`, `ExaminePanel`, `GENERAL INVENTORY`, `BANK — STORED ITEMS`, `WORN — WEAPONS`, `aria-expanded`; **0** `scrim`/`modal` (inline expand, Pitfall 5); **0** `@html`; all **23** canonical slot names referenced (Pitfall 6) ✓.
- `+page.svelte`: contains `InventoryWindow`, `fetchInventory(`, `No inventory synced yet`; passes `inventory.last_seen` (NOT a per-slot `last_listed`) to the examine; **0** `@html` ✓.

## Task Commits

1. **Task 1: examine.ts (D-08 order/D-09 omit) + PaperdollSlot + ExaminePanel** — `7a6268a` (feat)
2. **Task 2: InventoryWindow (paperdoll+grids+inline bag+hover/pin) wired into /characters** — `68b9f49` (feat)

_(Task 3 — the deploy + browser-smoke — is the `checkpoint: human-verify` gate. It is NOT executed in this run: no deploy command was run, the browser-smoke is unverified — see "Deploy + Browser-Smoke: PENDING" below.)_

## Files Created/Modified
- `web/src/lib/examine.ts` — NEW: pure `examineFields` (D-08 order + D-09 omission) + the `ExamineField` type.
- `web/src/lib/__tests__/examine.test.ts` — NEW: 14 node cases.
- `web/src/lib/components/PaperdollSlot.svelte` — NEW: the slot tile (icon + colored-tile fallback + count + bag marker + a11y + `onhover`/`onleave`).
- `web/src/lib/components/ExaminePanel.svelte` — NEW: the pinned examine (D-08 rows; the one escaped `composeItemNote` `{@html}` sink).
- `web/src/lib/components/InventoryWindow.svelte` — NEW: the generic prop-driven window (paperdoll + grids + inline bag expand + hover/pin).
- `web/src/routes/characters/+page.svelte` — MODIFIED: replaced the 31-03 window-slot prompt with the window-scoped fetch + the `<InventoryWindow>` render + the D-11 no-inventory state.

## Deviations from Plan

The plan's two CODE tasks were executed as written. Small implementation refinements, all within the named files (no scope creep, no architectural change):

### Auto-fixed Issues

**1. [Rule 1 - Bug] Hover preview emitted by the slot button (not a wrapping element) to avoid a nested-`<button>` a11y error**
- **Found during:** Task 2 (the window's hover-preview wiring)
- **Issue:** The first draft wrapped each `PaperdollSlot` (whose filled tile is a `<button>`) in a second `<button class="hover-wrap">` to catch `mouseenter` for the transient preview — an interactive element nested in an interactive element (invalid HTML; an svelte-check a11y violation + a real keyboard-focus double-stop).
- **Fix:** Added `onhover`/`onleave` props to `PaperdollSlot` so the slot's OWN `<button>` emits the preview (filtering out the bag case); `InventoryWindow` passes the handlers. One interactive element per tile; the floating preview shares the `examineFields` body with the pin so they never drift.
- **Files modified:** `PaperdollSlot.svelte`, `InventoryWindow.svelte`
- **Verification:** `npm run check` → 0 errors / 0 warnings.
- **Committed in:** `68b9f49` (Task 2 commit)

**2. [Rule 1 - Bug] Reworded comments so the literal `modal`/`scrim`/`@html` tokens do not appear in InventoryWindow.svelte**
- **Found during:** Task 2 (acceptance grep)
- **Issue:** Explanatory comments said "NOT a modal; no scrim" and "adds NO {@html}"; the plan's acceptance + the threat model grep for the ABSENCE of `scrim`/`modal` (Pitfall 5) and a second `{@html}` (T-31-14), so a naive grep would false-positive on the negating comments (the same trap 31-03 hit).
- **Fix:** Reworded to "in-flow beneath the grid row, never a pop-out overlay" / "adds NO raw-HTML directive"; the component genuinely has no overlay and no second `{@html}` (the only escaped sink is `ExaminePanel`'s composed body).
- **Files modified:** `InventoryWindow.svelte`
- **Verification:** `grep -ci "scrim\|modal"` → 0; `grep -c "@html"` → 0; `npm run check` re-green.
- **Committed in:** `68b9f49` (Task 2 commit)

**3. [Rule 1 - Bug] Count-badge color → `var(--text)` (the UI-SPEC-sanctioned alternative) to keep zero literal hex**
- **Found during:** Task 1 (acceptance grep — "no literal hex outside the locked hsl()")
- **Issue:** The count badge used `color: #fff` (a literal hex the UI-SPEC §Typography lists, but bracketed with "(or var(--text))").
- **Fix:** Switched to `color: var(--text)` (the explicit UI-SPEC alternative) + the dark text-shadow for contrast; the ONLY non-token color is now the locked `hsl()` colored-tile fallback.
- **Files modified:** `PaperdollSlot.svelte`
- **Verification:** `grep "#[0-9a-fA-F]{3,6}"` → 0 in the file.
- **Committed in:** `7a6268a` (Task 1 commit)

**Total deviations:** 3 auto-fixed (all Rule 1 — a11y correctness + grep-clean evidence + token purity). No architectural changes, no scope creep.

## Deploy + Browser-Smoke: PENDING (Task 3 — the human-verify checkpoint)

**This phase ships a backend binary + migration `00012` + the web bundle (NOT web-only)** — the two 31-02 read routes + the 31-01 `icon_id`/`last_seen` carry-through + migration `00012_item_icon` all go live on this deploy. **The deploy was NOT performed and the browser-smoke is NOT verified.** Node vitest is DOM-blind — the entire window DOM (paperdoll render, hover/pin, inline bag expand, remote icon load + `onerror` fallback, master-detail selection, 5-theme rendering) is invisible to the green node tests and is only provable on a deployed build. This is the explicit `checkpoint: human-verify` gate; the deploy + the browser-smoke checklist are returned to the human as `## CHECKPOINT REACHED` (deploy steps + the exact UI-SPEC §Build-Notes checklist). Until they pass on `squirebot.quest` across all 5 themes, the phase truth "Deployed to prod + browser-smoked" is UNVERIFIED.

## Known Stubs
None. The window renders real `CharacterInventory` data end-to-end (icon → tile, slot → paperdoll position, children → inline bag, fields → examine). The `.doll` center figure shows the silhouette glyph + the char name only — that is the INTENTIONAL "never fabricate a stat" behavior (§E / D-09: the read contract exposes no AC/ATK, so none is shown), not a stub.

## Threat Flags
None. This plan adds NO new network/auth/file/schema surface — it renders the two 31-02 read routes' existing payloads. The single new `{@html}` is the audited `composeItemNote` escaped body in `ExaminePanel` (T-31-14 mitigated); the icon `<img>` src is `Item_${int}.png` from a trusted integer (T-31-15); the wiki href passes `safeHttpUrl`'s scheme allow-list (T-31-16). No other `{@html}` of any wiki/price/name string exists in `InventoryWindow`/`PaperdollSlot`/`+page` (grep-confirmed 0).

## User Setup Required
None for the code. The Task-3 deploy (backend binary + migration `00012` + web) is the operator action the checkpoint describes; no external service configuration beyond the existing prod deploy path (`docs/backend-deploy.md`).

## Next Phase Readiness
- `InventoryWindow` is a generic prop-driven component over `CharacterInventory` — **Phase 33** mounts it per bank toon (same component, a different `inventory` prop) and **Phase 34** reads its equipped slots for the per-slot wishlist.
- The window's correctness is browser-smoke-gated (Task 3); once the deploy + smoke pass, INV-01..04 + CHAR-03 are end-to-end verified.

## Self-Check: PASSED

(Appended after the file/commit verification below.)

---
*Phase: 31-characters-tab-in-game-inventory-window*
*Completed (code): 2026-06-18 — deploy + browser-smoke PENDING (Task 3 human-verify)*
