---
phase: 37-item-enrichment-backbone-flags-effects
verified: 2026-06-24T00:00:00Z
status: passed
score: 8/8 must-haves verified
overrides_applied: 0
---

# Phase 37: Item enrichment backbone — flags + effects Verification Report

**Phase Goal:** Re-enable the dropped wiki stat-block parse so LORE/NO-DROP/MAGIC/TEMPORARY flags + click-effect (Clicky) + Haste land in discrete, queryable columns (new migration 00016 + the enrich freshness short-circuit updated so existing rows backfill). DATA LAYER ONLY (no UI). Foundational — gates P39 facets + P40 outlines.
**Verified:** 2026-06-24
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth (goal-backward) | Status | Evidence (file:line) |
|---|------|--------|----------|
| 1 | The four flags (Lore/No-Drop/Magic/Temporary) are stored as discrete queryable columns AND surfaced by the parser (ENRICH-12) | ✓ VERIFIED | Parser: `wikiitem.go:62-69` (struct fields) + `:190-194` (`deriveFromMaps` reads `flags["LORE ITEM"]`, `flags["NO DROP"]\|\|flags["NO-DROP"]`, `flags["MAGIC ITEM"]`, `flags["TEMPORARY"]` — exact TS-oracle spellings). Columns: `00016_item_flags_effects.sql:19-22` (`is_lore/is_no_drop/is_magic/is_temporary INTEGER`). Upsert binds them `enrich.go:204-216,241`. `TestParseItempage_FlagsAndEffects` PASS (3 fixtures), `TestMigrate_00016` PASS (column round-trip). |
| 2 | Clicky (bool + effect name) + Haste (bool + % int) are discrete columns; clicky = activatable-Click only (D-01 — excludes Worn/Combat) | ✓ VERIFIED | `parseClicky` (`wikiitem.go:649-677`): clicky iff the LAST `(...)` qualifier contains "click" (case-insens.); `(Worn)`/`(Combat)`/no-qualifier → `(false,"")` (`:664-666`). `parseHastePct` (`:624-638`) → magnitude int. Columns `is_clicky/clicky_effect/has_haste/haste_pct` (`00016:23-26`). `TestParseClicky` PASS (Click/Click-from-Inv→true; Worn/Combat→false), `TestParseHastePct` PASS. Fixture-level: Cloak of Flames (Haste, MAGIC) → IsClicky=false; Fungus tunic (Worn) → IsClicky=false; Staff of Temperate Flux (Click) → IsClicky=true, name="Shock of Frost". |
| 3 | The FULL detected flag set is retained (flags_json, D-03) via the single canonical MarshalFlags (empty→"[]") | ✓ VERIFIED | `flagSet` returns sorted full set (`wikiitem.go:606-616`); `Flags` populated `:194`. `MarshalFlags` is the ONE encoder: nil/empty→`"[]"`, else `json.Marshal` (`enrich.go:51-60`). Bound to `flags_json TEXT` (`00016:27`). `TestMarshalFlags` PASS (nil/empty→"[]", 2-elem round-trip). |
| 4 | Backfill re-parses the ALREADY-STORED statsblock with NO network, is idempotent (flags_json IS NULL), AND a CLICKY backfills from the cleaned (bracket-stripped) Effect line (the WARNING-1 fold, D-05) | ✓ VERIFIED | `BackfillItemFlags` (`backfill.go:55-106`): SELECT `WHERE statsblock IS NOT NULL AND statsblock != '' AND flags_json IS NULL` (`:32-34` idempotency key); calls `enrich.DeriveFlagsAndEffects` (pure CPU, no net, `:89`); writes via `MarshalFlags` (`:93`). `TestBackfillItemFlags_ClickyFromStoredStatsblock` PASS — stored `Effect: Shock of Frost (Click from Inventory)` (no brackets) → is_clicky=1, clicky_effect="Shock of Frost", + 2nd pass updated=0 (idempotent). `TestBackfillItemFlags_FlaglessStoresEmptyArray` PASS. |
| 5 | The freshness short-circuit self-heals pre-00016 rows (D-06) | ✓ VERIFIED | `GetItemMasterFreshnessTx` now returns `flags_json` (`enrich.go:282-294`); job skip-condition requires `existingFlagsJSON == parsedFlagsJSON` ALSO (`wiki.go:237-238`); a pre-00016 NULL reads "" ≠ "[]" → re-writes once. `TestRunWiki_BackfillsStaleFlags` PASS (correct SHA-1 + icon + statsblock but NULL flags_json → re-written). |
| 6 | Live parse and backfill share ONE derivation (DeriveFlagsAndEffects) — no divergence | ✓ VERIFIED | `ParseItempage` calls `DeriveFlagsAndEffects(statsblockRaw)` (`wikiitem.go:117`); backfill calls the SAME (`backfill.go:89`); both funnel through `deriveFromMaps` (`wikiitem.go:176-201`). Splitter `brOrNlRe` handles `<br>` OR `\n` so raw + stored-cleaned forms parse identically (`:363,377`). `TestDeriveFlagsAndEffects_NewlineForm/newline_form_matches_the_<br>_form` PASS. `grep DeriveFlagsAndEffects` = 2 call sites; `grep MarshalFlags(` = 3 sites (upsert via job literal, backfill, freshness compare). |
| 7 | `go test ./...` green, build/vet clean; watcher untouched (no internal/sheet pkg); migration is 00016 additive | ✓ VERIFIED | `go build ./...` exit 0; `go vet ./internal/backendsrv/... ./cmd/squirebot-server/` exit 0; backendsrv enrich/store/migrations suites all `ok`. `internal/sheet/**` → NO files found (the watcher write path / `WatcherMaxSchemaVersion` const does not exist in Go source — only in planning docs + migration comments). 00016 = 9× `ADD COLUMN` (nullable, no DEFAULT/UNIQUE), forward-only. |
| 8 | SCOPE: no UI/facet/outline work leaked in (P39/P40) | ✓ VERIFIED | Phase commit range `3ab6463..2dcc0b3`: zero `.svelte`/`.ts`/`.css`/`web/` files touched (grep empty). All 11 touched files are backend Go (parser, store, migration, job, main.go) + 1 JSON fixture. DATA LAYER ONLY. |

**Score:** 8/8 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/backendsrv/enrich/wikiitem.go` | ParsedWikiItem + 9 fields, DeriveFlagsAndEffects, parseClicky/parseHastePct/flagSet | ✓ VERIFIED | 9 fields `:62-76`; `DeriveFlagsAndEffects` `:176`; helpers `:606/:624/:649`. Wired into `ParseItempage` `:117-139`. |
| `internal/backendsrv/migrations/00016_item_flags_effects.sql` | 9 additive ADD COLUMN | ✓ VERIFIED | 9 nullable ADD COLUMN (Up) + 9 reverse DROP (Down); no DEFAULT/UNIQUE. |
| `internal/backendsrv/migrations/migrate_test.go` | TestMigrate_00016 asserting 9 cols + idempotency | ✓ VERIFIED | `TestMigrate_00016_AddsItemFlagsEffects` `:1140` over `itemFlagsEffectsColumns` (9) + 2nd-Up no-op. PASS. |
| `internal/backendsrv/store/enrich.go` | MarshalFlags + 19-col upsert + freshness returns flags_json | ✓ VERIFIED | `MarshalFlags` `:51`; `itemMasterUpsert` 19 cols/19 `?` `:204-216`; `GetItemMasterFreshnessTx` returns flags_json `:282-294`. |
| `internal/backendsrv/store/backfill.go` | no-network BackfillItemFlags | ✓ VERIFIED | `:55-106`, idempotency key flags_json IS NULL, shares DeriveFlagsAndEffects + MarshalFlags. |
| `internal/backendsrv/enrich/jobs/wiki.go` | self-heal freshness compare + new fields in store literal | ✓ VERIFIED | freshness compare extended `:237-238`; store literal carries 9 fields `:260-268`; parsedFlagsJSON via MarshalFlags `:236`. |
| `cmd/squirebot-server/main.go` | boot backfill after RunMigrations (non-fatal) | ✓ VERIFIED | `BackfillItemFlags(context.Background())` `:235`, after `RunMigrations` `:223`, log-and-continue `:236`. |
| `testdata/wiki-parse-staff-of-temperate-flux.json` | clicky-positive fixture | ✓ VERIFIED | Present (799 bytes); contains `Effect: [[Shock of Frost]] (Click from Inventory)`. |

### Key Link Verification

| From | To | Via | Status |
|------|----|----|--------|
| ParseItempage | DeriveFlagsAndEffects | one derivation seam | ✓ WIRED (`wikiitem.go:117`) |
| BackfillItemFlags | enrich.DeriveFlagsAndEffects | same derivation, no net | ✓ WIRED (`backfill.go:89`) |
| upsert + backfill + job freshness | store.MarshalFlags | one canonical encoder (3 sites) | ✓ WIRED (grep = 3 call sites) |
| main.go | store.BackfillItemFlags | boot call after RunMigrations | ✓ WIRED (`main.go:235`) |
| wiki.go freshness | flags_json compare | self-heal pre-00016 | ✓ WIRED (`wiki.go:237-238`) |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Whole-module builds | `go build ./...` | exit 0 | ✓ PASS |
| Backend vet clean | `go vet ./internal/backendsrv/... ./cmd/squirebot-server/` | exit 0 | ✓ PASS |
| Parser flags/clicky/haste | `go test -run TestParseItempage_FlagsAndEffects\|TestParseClicky\|TestParseHastePct` | PASS | ✓ PASS |
| One derivation, no drift | `TestDeriveFlagsAndEffects_NewlineForm` (incl. br-vs-nl parity) | PASS | ✓ PASS |
| No-network backfill + WARNING-1 clicky fold + idempotent | `TestBackfillItemFlags_*` (3) | PASS | ✓ PASS |
| Canonical encoder | `TestMarshalFlags` | PASS | ✓ PASS |
| Migration 9 cols + idempotent | `TestMigrate_00016_AddsItemFlagsEffects` | PASS | ✓ PASS |
| Freshness self-heal (D-06) | `TestRunWiki_BackfillsStaleFlags` | PASS | ✓ PASS |
| Backend suites green | `go test ./internal/backendsrv/enrich/... ./store/... ./migrations/...` | all `ok` | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| ENRICH-12 | 37-01, 37-02 | Item attribute flags (LORE/NO DROP/MAGIC/TEMPORARY) → discrete queryable fields | ✓ SATISFIED | Truths 1, 3 — parser surfaces + 00016 persists discrete bool columns + full flags_json set. |
| ENRICH-13 | 37-01, 37-02 | Click-effect (Clicky) + Haste → discrete queryable fields | ✓ SATISFIED | Truth 2 — is_clicky/clicky_effect/has_haste/haste_pct columns; D-01 Click-only classification verified. |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (none) | — | No stubs, TODO/FIXME, empty handlers, or hardcoded-empty render data | ℹ️ Info | Backfill SELECT filters `flags_json IS NULL` (idempotency, not a stub); `return "[]"` in MarshalFlags is the documented empty-set contract (D-06), not a placeholder. |

### Human Verification Required

None. This is a backend data-layer phase with no UI/render surface (the parsed values are stored, not yet rendered — render-time escaping is the explicitly-deferred T-37-07 / Phase 40 concern). All behaviors are verifiable via the green Go test suite (no server start, no external service, no visual/real-time element). First prod boot of 00016 + the live backfill is a deploy-time operational step, not a goal-achievement gap.

### Gaps Summary

No gaps. Every goal-backward truth resolves to VERIFIED against the committed code, corroborated by an explicitly-passing test (not merely SUMMARY claims). The single-derivation contract (DeriveFlagsAndEffects) and single-encoder contract (MarshalFlags, empty→"[]") — the two load-bearing D-06 idempotency keystones — are both real, grep-confirmed at exactly the expected call-site counts (2 and 3), and behaviorally proven by the br-vs-nl parity test and the flagless→"[]" backfill test. The D-01 clicky classification correctly excludes (Worn)/(Combat). The migration is additive 00016; the watcher is untouched (no `internal/sheet` Go package exists). Scope held to the data layer — no UI/facet/outline leakage.

---

_Verified: 2026-06-24_
_Verifier: Claude (gsd-verifier)_
