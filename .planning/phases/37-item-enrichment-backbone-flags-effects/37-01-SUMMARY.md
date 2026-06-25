---
phase: 37-item-enrichment-backbone-flags-effects
plan: 01
subsystem: api
tags: [go, wiki-parser, enrichment, item-flags, clicky, haste, mediawiki]

# Dependency graph
requires:
  - phase: 31-item-enrichment (INV-04 icon parse)
    provides: parseIconID — the "parse one field off the page defensively, add it to ParsedWikiItem" precedent these helpers mirror
  - phase: INV-02 examine stats
    provides: parseStatsblock flags/kv maps + cleanStatsblock — the already-built source the new fields are derived from
provides:
  - ParsedWikiItem extended with IsLore/IsNoDrop/IsMagic/IsTemporary (ENRICH-12 queried flags)
  - Flags []string — the FULL sorted detected all-caps flag set (ENRICH-12 / D-03)
  - IsClicky + ClickyEffect (ENRICH-13 / D-01 / D-02 — activatable click only)
  - HasHaste + HastePct (ENRICH-13 / D-02 — integer % value)
  - parseClicky / parseHastePct / flagSet unexported helpers
  - wiki-parse-staff-of-temperate-flux.json — the clicky-positive test fixture
affects: [37-02 (store struct + columns + migration 00016 + backfill + freshness), 38-enrichment-coverage, 39-search-facets, 40-item-ui-outlines]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Derive new ParsedWikiItem fields from the existing parseStatsblock flags/kv maps — never a new wikitext scan (ReDoS-safe, T-37-01)"
    - "Defensive single-field parse helpers (nil/empty/non-numeric -> zero value) mirroring parseIconID"

key-files:
  created:
    - internal/backendsrv/enrich/testdata/wiki-parse-staff-of-temperate-flux.json
  modified:
    - internal/backendsrv/enrich/wikiitem.go
    - internal/backendsrv/enrich/wikiitem_test.go

key-decisions:
  - "Clicky classification (D-01): IsClicky=true iff the LAST parenthesized qualifier on the Effect line contains 'click' (case-insensitive); (Worn) and (Combat) -> false"
  - "ClickyEffect is the effect display name with [[wiki-link]] brackets stripped (reusing summaryLinkRe) and the trailing (...) qualifier removed; set ONLY when IsClicky"
  - "HastePct stores the magnitude (the '-' is trimmed, not preserved) — the wiki always writes +NN%; negative haste is not a real item"
  - "Flags is the full sorted detected set (D-03) so a future flag needs no parser/migration change"
  - "Exact TS-oracle flag spellings reused verbatim; IsNoDrop OR's flags['NO DROP'] with flags['NO-DROP']"

patterns-established:
  - "Pattern 1: Surface-don't-rescan — flag/effect fields read from the pre-built flags/kv maps, keeping the parser single-pass and ReDoS-safe"
  - "Pattern 2: (bool, value) helper return shape — parseClicky/parseHastePct return both the presence boolean and the parsed value in one call, matching the D-02 booleans+values storage shape"

requirements-completed: [ENRICH-12, ENRICH-13]

# Metrics
duration: 5min
completed: 2026-06-25
---

# Phase 37 Plan 01: Item enrichment backbone — flags + effects (parse half) Summary

**ParseItempage now surfaces the wiki item flags (Lore/No-Drop/Magic/Temporary + the full detected flag set) and the Clicky/Haste effects (activatable-click name + integer haste %) it already computed internally, derived purely from the existing parseStatsblock maps — no new wikitext scan, no DB/network.**

## Performance

- **Duration:** 5 min
- **Started:** 2026-06-25T03:26:56Z
- **Completed:** 2026-06-25T03:31:40Z
- **Tasks:** 2
- **Files modified:** 3 (2 modified, 1 created)

## Accomplishments
- Extended `ParsedWikiItem` with nine new fields: `IsLore`, `IsNoDrop`, `IsMagic`, `IsTemporary`, `Flags []string` (ENRICH-12), and `IsClicky`, `ClickyEffect`, `HasHaste`, `HastePct` (ENRICH-13).
- Added three defensive unexported helpers — `flagSet` (sorted keys), `parseHastePct` (`+NN%`→int), `parseClicky` (Click-vs-Worn-vs-Combat classification) — mirroring `parseIconID`'s nil/empty guards.
- Wired all nine fields into `ParseItempage` reading the already-built `flags`/`kv` maps (no new statsblock scan — T-37-01 ReDoS mitigation honored).
- Added the `wiki-parse-staff-of-temperate-flux.json` clicky-positive fixture (the five real fixtures had only a haste-positive and a worn-effect-negative case).
- Added table tests proving the four flags, the full flag set, clicky-vs-worn-vs-combat classification, and haste %.
- Updated the D-8 scope-guard comment to record that the byte-parity proof is dead post-v2.0 and these derived fields are now intentionally surfaced.

## Task Commits

Each task was committed atomically:

1. **Task 1: Extend ParsedWikiItem + add flag/clicky/haste derivation** - `3ab6463` (feat) — TDD: RED test `TestParseItempage_NewFieldsSurfaced` written first (compile-fail confirmed), then GREEN implementation; committed together as one task.
2. **Task 2: Click-effect fixture + table tests** - `f656a7e` (test)

_Note: Task 1 was `tdd="true"`; the RED test and GREEN implementation were committed in the single `feat` task commit (the comprehensive table suite is Task 2)._

## Files Created/Modified
- `internal/backendsrv/enrich/wikiitem.go` - Extended `ParsedWikiItem` struct (+9 fields), wired derivations into `ParseItempage`, added `flagSet`/`parseHastePct`/`parseClicky` helpers, added `sort` import, rewrote the D-8 scope-guard comment.
- `internal/backendsrv/enrich/wikiitem_test.go` - Added `TestParseItempage_NewFieldsSurfaced` (Task 1 RED→GREEN gate), `TestParseItempage_FlagsAndEffects` (3-fixture table), `TestParseClicky`, `TestParseHastePct`.
- `internal/backendsrv/enrich/testdata/wiki-parse-staff-of-temperate-flux.json` - Synthetic MAGIC+LORE item with an `Effect: [[Shock of Frost]] (Click from Inventory)` line — the clicky-positive fixture.

## Final field names (for 37-02 wiring)

37-02 wires these EXACT struct field names into the store struct + columns:

| Field | Type | Meaning |
|-------|------|---------|
| `IsLore` | bool | `flags["LORE ITEM"]` |
| `IsNoDrop` | bool | `flags["NO DROP"] \|\| flags["NO-DROP"]` |
| `IsMagic` | bool | `flags["MAGIC ITEM"]` |
| `IsTemporary` | bool | `flags["TEMPORARY"]` |
| `Flags` | `[]string` | full sorted detected all-caps flag set (D-03 — store as JSON/TEXT per D-04) |
| `IsClicky` | bool | activatable Click effect only (Worn/Combat → false) |
| `ClickyEffect` | string | clicky display name (links + qualifier stripped); "" unless `IsClicky` |
| `HasHaste` | bool | a `Haste:` stat line is present |
| `HastePct` | int | integer haste % magnitude (0 when absent) |

**Clicky-classification rule (for 37-02 / downstream):** the Effect line is a clicky iff its LAST parenthesized qualifier contains the word "click" (case-insensitive). `(Worn)` and `(Combat)` are NOT clickies (D-01).

## Decisions Made
- **Clicky qualifier match (D-01):** matched on the LAST `(...)` qualifier containing "click" case-insensitively, so `(Click)` and `(Click from Inventory)` both classify true while `(Worn)`/`(Combat)`/no-qualifier classify false.
- **HastePct magnitude:** `parseHastePct` trims a leading `+`/`-` and the trailing `%`, so it stores the magnitude. The wiki always writes `+NN%`; there is no negative haste item, so no negative value is ever stored (consistent with `parseIconID` returning 0 for negatives).
- **Helper placement / reuse:** the three helpers sit beside `parseIconID` and reuse the bounded `summaryLinkRe` for link-stripping (the only regex involved), matching the existing `cleanStatsblock` shape — no new regex over raw wikitext.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None. The Cloak of Flames fixture's `Haste: +36%  <br>` (trailing spaces before `<br>`) parses cleanly because `parseStatsblock` already `TrimSpace`s the value to `"+36%"` before it reaches `parseHastePct`.

## Threat Model Compliance
- **T-37-01 (ReDoS):** mitigated — every new field is derived from the already-parsed `flags`/`kv` maps; the only regex reused is the bounded `summaryLinkRe` (linear, no nested quantifiers). No new regex over raw wikitext.
- **T-37-02 (panic/tampering):** mitigated — `parseHastePct`/`parseClicky`/`flagSet` guard empty/nil/garbage inputs (return zero values); `parseClicky` bounds-checks the `(`/`)` indices before slicing. Covered by `TestParseHastePct`/`TestParseClicky` garbage cases.
- **T-37-03 (info disclosure at render):** accepted/deferred — parsed values are surfaced (not rendered); escaping is a Phase 40 (ITEMUI) concern. No rendering surface exists this phase.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Plan 37-02 can now wire these nine fields into the `item_master` store struct + columns (migration 00016), backfill from the stored `statsblock` TEXT (D-05), and extend `GetItemMasterFreshnessTx` (D-06). The exact field names + the clicky rule are recorded above.
- No files outside `internal/backendsrv/enrich/` were touched — the struct-to-SQL wiring is deliberately deferred to 37-02.

## Self-Check: PASSED

- FOUND: internal/backendsrv/enrich/wikiitem.go (modified, builds)
- FOUND: internal/backendsrv/enrich/wikiitem_test.go (modified, tests pass)
- FOUND: internal/backendsrv/enrich/testdata/wiki-parse-staff-of-temperate-flux.json (created)
- FOUND commit 3ab6463 (Task 1 feat)
- FOUND commit f656a7e (Task 2 test)
- `go build ./...` exit 0, `go vet ./enrich/` clean, `go test ./enrich/ -count=1` green

---
*Phase: 37-item-enrichment-backbone-flags-effects*
*Completed: 2026-06-25*
