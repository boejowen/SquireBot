---
phase: 40-item-display-polish-flag-outlines-quest-links
plan: 02
subsystem: web
tags: [svelte, sveltekit, css-tokens, item-flags, quest-links, examine, tooltip, itemui, browser-smoke, deploy]

# Dependency graph
requires:
  - phase: 40-01-api
    provides: "InventorySlot + ItemRollup JSON now carry is_no_drop/is_lore/is_magic + quest_links[] (each with source_url)"
  - phase: 39-faceted-item-search
    provides: "the is_clicky/has_haste api.ts mirror + FacetBar precedent + the deploy-then-browser-smoke pattern"
  - phase: 37-item-enrichment-backbone
    provides: "the item_master.is_no_drop/is_lore/is_magic flag columns (migration 00016) the rings read"
provides:
  - "Flag-coded 62px tile ring (No-Drop red > Lore gold > Magic blue priority) on every PaperdollSlot surface (paperdoll / general grid / bank grid / bag sub-grids) via a ::before inset box-shadow that coexists with the accent hover/focus border (D-03)"
  - "Examine panel priority flag chip beside the item name + clickable named 'Used in:' quest links (notes_link only) on all three detail surfaces (Characters / Inventory / Wishlist)"
  - "Legacy view/bank item tooltip renders each named quest as a clickable wiki link via source_url through safeHttpUrl; the generic Quest item badge stays the in_game_flag-only fallback"
  - "Three new per-theme flag tokens (--flag-nodrop/--flag-lore/--flag-magic) in BOTH app.css and themes.ts in lockstep, parity-tested"
affects: [41-character-paperdoll, 42-wishlist-polish]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Pure, node-testable priority resolver (flags.ts flagColorVar/flagChipLabel) shared by PaperdollSlot + examine.ts — the S-4 pure-helper pattern (items.ts facetItems precedent)"
    - "::before inset box-shadow flag ring keyed off an inline --flag-color CSS var — never border-color, so the existing accent hover/focus affordance survives (D-03)"
    - "Two escaped render paths for the same quest data: native-Svelte {q.quest_name} interpolation (examine) + escapeHtml inside the single sanctioned composeItemNote {@html} sink (tooltip); every href passes safeHttpUrl FIRST (T-40-04/T-40-05)"
    - "Additive per-theme design tokens land in app.css + themes.ts in lockstep with a REQUIRED_TOKENS parity test asserting all 5 themes carry them"

key-files:
  created:
    - web/src/lib/flags.ts
    - web/src/lib/__tests__/flags.test.ts
  modified:
    - web/src/lib/api.ts
    - web/src/lib/theme/themes.ts
    - web/src/app.css
    - web/src/lib/__tests__/themes.test.ts
    - web/src/lib/__tests__/banks.test.ts
    - web/src/lib/__tests__/items.test.ts
    - web/src/lib/examine.ts
    - web/src/lib/__tests__/examine.test.ts
    - web/src/lib/tooltip/composeNotes.ts
    - web/src/lib/__tests__/composeNotes.test.ts
    - web/src/lib/components/PaperdollSlot.svelte
    - web/src/lib/components/ExaminePanel.svelte
    - web/src/lib/components/ItemTooltip.svelte
    - web/src/routes/inventory/+page.svelte
    - web/src/routes/wishlist/+page.svelte

key-decisions:
  - "Flag ring rides a ::before inset box-shadow keyed off an inline --flag-color var (never border-color) so it coexists with the accent hover/focus border/box-shadow (D-03) and inherits to every InventoryWindow grid for free (no call-site change) — SC-2 consistency by construction"
  - "One pure priority resolver (flags.ts) is the single source of the No-Drop>Lore>Magic order (D-01), reused by both the tile ring (flagColorVar) and the examine chip (flagChipLabel) — node-tested, so the DOM-blind vitest still proves the priority logic"
  - "Examine named quests use the NATIVE Svelte path ({q.quest_name} auto-escaped + href={safeHttpUrl(...)}) — no new {@html} sink; the tooltip reuses the existing single composeItemNote {@html} sink with escapeHtml (T-40-04/T-40-05)"
  - "notes_link-only filter (D-06): only real named quests render as 'Used in:' links; an item with only the in_game_flag entry keeps the generic QUEST ITEM / Quest item badge, and the field is omitted entirely when there are zero named quests (D-09 graceful omission)"

patterns-established:
  - "Per-theme non-text-color tokens (flag rings) added to app.css + themes.ts in lockstep with a parity test — the template any future flag/effect color follows"

requirements-completed: [ITEMUI-01, ITEMUI-02]

# Metrics
duration: ~35min execute + browser-smoke + 1 fix-forward
completed: 2026-06-26
---

# Phase 40 Plan 02: Item display polish — web flag rings + named quest links Summary

**Web (SvelteKit) half of Phase 40: render ITEMUI-01's flag-coded tile ring + examine flag chip and ITEMUI-02's clickable named quest links, consuming the additive JSON the 40-01 backend now ships. Adds three per-theme flag tokens (app.css + themes.ts lockstep), a pure priority-flag resolver, the examine `flagchip` + `quests` fields, the `PaperdollSlot` `::before` ring, and clickable quest links in the examine panel + the legacy tooltip. DEPLOYED LIVE (backend 40-01 + fix + web 40-02 together); prod schema v17 → v18.**

## Performance

- **Duration:** ~35 min execute (3 code tasks) + a browser-smoke checkpoint + one backend fix-forward
- **Completed:** 2026-06-26
- **Tasks:** 4 (3 code + 1 blocking browser-smoke checkpoint)
- **Files:** 2 created, 15 modified

## Accomplishments
- **Contract mirror + tokens (Task 1, `02b563a`):** `api.ts` — `QuestLink.source_url`; `InventorySlot` + `ItemRollup` each gain `is_no_drop`/`is_lore`/`is_magic` + `quest_links[]` (the Phase-39 `is_clicky`/`has_haste` precedent). Three new per-theme tokens `--flag-nodrop`/`--flag-lore`/`--flag-magic` added to all five themes in **both** `themes.ts` and `app.css` in lockstep with the prescriptive per-theme hex; `themes.test.ts` `REQUIRED_TOKENS` extended + a velious spot-check; the `items.test.ts`/`banks.test.ts` fixtures backfilled with the new required `InventorySlot`/`ItemRollup` fields.
- **Pure resolver + logic fields (Task 2, `27b09bd`):** new `flags.ts` — `flagColorVar`/`flagChipLabel` priority resolver (No-Drop > Lore > Magic, D-01), node-tested in `flags.test.ts`. `examine.ts` gains a `flagchip` field (priority chip after the name) + a `quests` field (after wiki, before lastsynced; `notes_link`-only per D-06; graceful omission per D-09), asserted for order + omission. `composeNotes.ts` — `QuestLinkForNote.source_url`; each `notes_link` name renders a clickable `<a>` via `safeHttpUrl` (D-05), plain escaped text on a blocked/blank URL; `composeNotes.test.ts` asserts the link, the fallback, and XSS-escaping inside the link.
- **Component renders (Task 3, `33cd982`):** `PaperdollSlot.svelte` — the `::before` inset box-shadow ring keyed off `--flag-color` (priority via `flagColorVar`) with a `class:flagged` gate, never touching `border-color` so the accent hover/focus coexists (D-03). `ExaminePanel.svelte` — the `flagchip` branch (bordered priority chip beside the name) + the `quests` branch (native-Svelte named "Used in:" links, every href through `safeHttpUrl` first). `ItemTooltip.svelte` — the `.tooltip-quest-link` accent-link style. The two synthetic `asSlot` builders (`inventory/+page.svelte` held flags/quests, `wishlist/+page.svelte` safe defaults) thread the four new `InventorySlot` fields.
- **Gates green (re-verified at close-out):** `npm run check` 0 errors / 0 warnings (500 files); `npm test` **399 passed (28 files)**; `npm run build` OK. Backend module `go build ./...` + `go test ./internal/backendsrv/...` green (see 40-01 + the fix below).

## Task Commits

1. **Task 1: api.ts contract mirror + three per-theme flag tokens (themes.ts + app.css lockstep) + parity tests** — `02b563a` (feat)
2. **Task 2: Pure flag resolver + examine flagchip/quests fields + composeNotes clickable quest links** — `27b09bd` (feat)
3. **Task 3: Component renders — PaperdollSlot ring, ExaminePanel branches, ItemTooltip link style, asSlot builders** — `33cd982` (feat)
4. **Task 4: Browser-smoke checkpoint (blocking human-verify)** — surfaced two smoke observations; one real backend bug fixed-forward (below), one non-bug (stale cache). See "Browser-smoke outcome".

## Browser-smoke outcome (Task 4)

The blocking browser-smoke checkpoint (web vitest is DOM-blind — no `@testing-library/svelte`) ran deploy-then-smoke-on-prod. Two observations:

- **Bug A — no red/gold rings (only blue rendered): REAL BACKEND BUG, fixed-forward `39674a8`.** Root cause was a **Phase-37 parser defect**, not a Phase-40 web defect: the P1999 wiki packs multiple flags onto ONE statsblock line separated by single spaces (`MAGIC ITEM NO DROP`, `MAGIC ITEM LORE ITEM`, `LORE ITEM NO DROP`); `parseStatsblock` stored the whole line as a single all-caps map key, and `deriveFromMaps` matched the four queried flags by **exact** key lookup (`flags["NO DROP"]`) — so every clustered line missed, silently zeroing `is_no_drop`/`is_lore` for ~95% of held flag-bearing items (prod: 160/168 no-drop, 330/360 lore). Only pure-`MAGIC ITEM` lines (blue) resolved, which is exactly what rendered. **Fix:** `enrich/wikiitem.go` `hasFlag()` substring-containment applied to `is_lore`/`is_no_drop`/`is_magic`/`is_temporary`/`is_quest_item` (full flag phrases are specific enough to carry no false positives) + a regression test over four verbatim prod statsblocks; **migration 00018** NULLs `item_master.flags_json` so the 00016 boot backfill (idempotency-keyed on `flags_json IS NULL`) re-derives all 953 rows with the fixed parser on the next restart (pure CPU over the stored statsblock; `flags_json` repopulates to identical bytes so the weekly freshness pass does not churn; `catalog_enrichment` is empty so its first crawl uses the fixed parser).
- **Bug B — quest link not clickable: NOT A BUG (stale browser cache).** The whole API→DB→JS→CSS pipeline verified correct; the user hard-refreshed and the links work.

## Deploy

- **Backend (40-01 + the `39674a8` fix) + web (40-02) deployed together to prod** (Windows ssh-agent SERVICE key + PowerShell ssh.exe/scp.exe to root@5.78.232.85; R2 backup first; web §7.5 atomic swap).
- **goose applied migration 00018 on boot → prod schema v17 → v18**; the 00016 backfill re-derived 941 rows with the fixed parser (8→168 no-drop, 30→359 lore, 535 magic).
- Watcher untouched (off the read path) → **NO `v*` tag**.

## Files Created/Modified
See the frontmatter `key-files` block. Highlights: `web/src/lib/flags.ts` (new pure resolver), `api.ts` (contract mirror), `themes.ts` + `app.css` (3 lockstep tokens), `examine.ts`/`composeNotes.ts` (fields + links), `PaperdollSlot.svelte`/`ExaminePanel.svelte`/`ItemTooltip.svelte` (renders), the two `asSlot` builders. Backend fix-forward: `enrich/wikiitem.go` (+ test) + `migrations/00018_reflag_item_master.sql`.

## Decisions Made
- **Flag ring = `::before` inset box-shadow off `--flag-color`** (not border-color) — coexists with the accent hover/focus (D-03) and inherits to every PaperdollSlot surface for free (SC-2).
- **One pure priority resolver** (`flags.ts`) is the single source of the No-Drop>Lore>Magic order, node-tested so the DOM-blind vitest still proves the priority.
- **Native-Svelte escaped examine path + the existing single tooltip `{@html}` sink**, every href through `safeHttpUrl` first — no new HTML sink introduced (T-40-04/T-40-05).
- **`notes_link`-only** named quests (D-06); generic `QUEST ITEM` badge stays the in_game_flag-only fallback; the field is omitted at zero named quests (D-09).

## Deviations from Plan
- **One backend fix-forward beyond the plan's web scope: the clustered-flag parser bug + migration 00018** (`39674a8`). The 40-02 plan assumed the P37 flags were correct in `item_master` and stated "NO migration this phase"; the browser smoke proved the P37 parser was silently mis-flagging ~95% of clustered lines, so the ring feature could not be verified without fixing the data source. The fix is a Phase-37 correctness repair surfaced by the Phase-40 smoke; migration 00018 is a data-only re-derive (no schema shape change). This is the correct fix-forward, not a scope creep — the ring web code was already correct (blue rendered).

## Issues Encountered
- Root-causing "only blue rings render" required tracing back through the web ring code (correct) → the api.ts contract (correct) → the compute plumbing (correct) → the stored `item_master` flags (WRONG) → `deriveFromMaps` exact-key lookup vs the wiki's space-clustered flag lines. Documented as [[project_resume_point_2026-06-17]] Smoke bug A.

## User Setup Required
None — deployed. One user action remains: a visual re-confirm of the ring colors (red/gold/blue) after a plain refresh (the data is confirmed correct server-side; the 00018 re-derive ran on the deploy boot).

## Next Phase Readiness
- ITEMUI-01 + ITEMUI-02 delivered and live; Phase 40 closes v2.6 to 4/6.
- Phases 41 (character paper-doll compaction + portrait photo) and 42 (wishlist compaction + sub-Velious tiers) are independent — either can plan next. Recommended next: `/gsd-plan-phase 41`.

## Self-Check: PASSED
- All 17 modified/created source files exist on disk; `flags.ts` + `flags.test.ts` created.
- Three task commits exist: `02b563a`, `27b09bd`, `33cd982`; the fix-forward `39674a8`.
- Web gates re-run green at close-out: check 0 errors, 399 tests pass, build OK.

---
*Phase: 40-item-display-polish-flag-outlines-quest-links*
*Completed: 2026-06-26*
