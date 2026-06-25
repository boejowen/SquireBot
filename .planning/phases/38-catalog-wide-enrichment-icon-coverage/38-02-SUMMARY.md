---
phase: 38-catalog-wide-enrichment-icon-coverage
plan: 02
subsystem: enrichment-job
tags: [go, sqlite, enrichment, name-keyed, catalog_enrichment, item_master, icon-coverage, wiki-job]

# Dependency graph
requires:
  - phase: 38-catalog-wide-enrichment-icon-coverage
    plan: 01
    provides: "catalog_enrichment table (00017) + UpsertCatalogEnrichmentTx/GetCatalogEnrichmentFreshnessTx + EnrichmentRef{ItemID,Name,Held} + DistinctEnrichmentRefs (guard dropped) + CatalogIconCoverage (both stores)"
  - phase: 37-item-enrichment-backbone-flags-effects
    provides: "store.MarshalFlags canonical encoder + GetItemMasterFreshnessTx 4-field self-heal"
provides:
  - "The weekly wiki items pass (runWikiItems/upsertItemAndQuests) now BRANCHES the per-item write on ref.Held: held → item_master (by EQ id) + quest_items (unchanged); catalog-only → catalog_enrichment (by norm_name), no quest_items write"
  - "An unheld catalog item with a wiki icon gets a catalog_enrichment row with a non-zero icon_id and NO item_master row (recovers the 43 items the Option-A collision guard dropped; covers all ~4,343)"
  - "The D-03 coverage slog reads CatalogIconCoverage (item_master ∪ catalog_enrichment), so the maintainer diagnostic spans the whole catalog, not just the ~953 held rows"
affects: [39 (Clicky/Haste faceted search reads held∪catalog by name end-to-end)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Held-vs-catalog write branch keyed STRICTLY on ref.Held (not on row existence — Pitfall 2): a catalog ref always routes to the name-keyed store, never the held EQ-id store"
    - "Shared canonical 4-field freshness short-circuit (sha|icon|statsblock|flags via store.MarshalFlags) reused verbatim in the catalog branch, so the icon/flag backfill rides for free in both stores"
    - "quest_items stays held-only / EQ-namespace — the catalog branch parses the page but deliberately skips the quest write (T-38-08)"

key-files:
  created: []
  modified:
    - internal/backendsrv/enrich/jobs/wiki.go
    - internal/backendsrv/enrich/jobs/wiki_test.go

key-decisions:
  - "Branched on ref.Held inside upsertItemAndQuests (one function, two tx bodies) rather than splitting into two functions — keeps the call site + the shared MarshalFlags compute + the per-item tx lifecycle in one place; the held body is the prior code verbatim so its blast radius is provably zero"
  - "slog coverage key renamed item_master_total → total (the plan's allowed 'optional' rename) for honesty now that coverage spans both stores"

requirements-completed: [ENRICH-14, ENRICH-15]

# Metrics
duration: 6min
completed: 2026-06-25
---

# Phase 38 Plan 02: Catalog-wide Enrichment — Branched Name-Keyed Write Summary

**The weekly wiki items pass now branches its per-item write on `ref.Held` — held items still write `item_master` (by EQ id) + `quest_items` byte-for-byte as before, while catalog-only items write the name-keyed `catalog_enrichment` table (no quest write) — and the D-03 coverage diagnostic reads icon coverage across BOTH stores, so an unheld catalog item with a wiki icon is now enriched and visible without ever touching `item_master`.**

## Performance

- **Duration:** ~6 min
- **Started:** 2026-06-25T18:33:32Z
- **Completed:** 2026-06-25T18:39:04Z
- **Tasks:** 2 of 2
- **Files modified:** 2 (0 created, 2 modified)

## Accomplishments
- **Replaced the 38-01 bridge shim with the real `ref.Held` branch.** `upsertItemAndQuests` now takes `store.EnrichmentRef` (was the adapted `store.ItemRef`) and branches STRICTLY on `ref.Held` (Pitfall 2 — never on row existence):
  - `ref.Held == true` → the prior tx body VERBATIM: `GetItemMasterFreshnessTx` → 4-field compare → `UpsertItemMasterTx` (the full `ItemMaster` literal) → `ReplaceQuestItemsForIDTx` → Commit.
  - `ref.Held == false` → NEW tx: `norm := strings.ToLower(strings.TrimSpace(item.ItemName))` → `GetCatalogEnrichmentFreshnessTx` → the SAME 4-field compare (`parsedFlagsJSON` via `store.MarshalFlags`) → `UpsertCatalogEnrichmentTx` (keyed by `norm_name`, representative PigParse id as a non-key column) → Commit. NO `ReplaceQuestItemsForIDTx` — `quest_items` stays held-only / EQ-namespace (T-38-08); the branch parses the page but discards `questLinks` (`_ = questLinks`).
- **Call site de-shimmed** — `runWikiItems` now passes `ref` (the `EnrichmentRef` it ranges over from `DistinctEnrichmentRefs`) straight through; the 38-01 `store.ItemRef{...}` adapter is gone.
- **D-03 coverage re-pointed across both stores** — the read is `s.CatalogIconCoverage(ctx, residueSampleCap)` (item_master ∪ catalog_enrichment); the comment block + `logItemsCoverage` doc updated to say it spans both stores; the slog key `item_master_total` → `total` (the plan's allowed optional rename) for honesty. The residue sample is still PUBLIC item names only, bounded at 50 (V7 / T-38-07).
- **`canonical` freshness short-circuit shared** — `parsedFlagsJSON := store.MarshalFlags(item.Flags)` is computed once and used by both branches, so the icon (ENRICH-15) and flag (ENRICH-12/13) backfill rides the existing self-heal in BOTH stores with no parse change.
- **Test flipped + RED-proven** — before editing the test I ran the OLD assertion against the new branched code and confirmed it FAILED (`Cloak of Flames has no item_master row` — proving the behavior actually changed). `TestRunWiki_EnrichesUnheldCatalogItem` now asserts the unheld Cloak of Flames lands in `catalog_enrichment` keyed by `norm_name` with a non-zero `icon_id` and is ABSENT from `item_master` (Pitfall 2 no-leak). The held Cloth Cap (1001), the job-`ok`, and the stale-icon / stale-flags held-path regressions stay green.

## Task Commits

1. **Task 1: Branch the write on ref.Held + both-stores coverage** — `92ff217` (feat)
2. **Task 2: Flip TestRunWiki_EnrichesUnheldCatalogItem to assert catalog_enrichment by norm_name** — `6a231fe` (test, TDD — RED proven against Task-1 code, then GREEN)

**Plan metadata commit:** created with this SUMMARY (the orchestrator owns STATE/ROADMAP writes for this project — neither was touched).

## Files Created/Modified
- `internal/backendsrv/enrich/jobs/wiki.go` — `upsertItemAndQuests` re-signatured to `store.EnrichmentRef` and branched on `ref.Held` (held → item_master+quest_items unchanged; catalog-only → catalog_enrichment by norm_name, no quest write); call site de-shimmed; coverage read `CatalogIconCoverage`; `strings` import added; D-03 comments + slog key (`total`) updated.
- `internal/backendsrv/enrich/jobs/wiki_test.go` — `TestRunWiki_EnrichesUnheldCatalogItem` assertion replaced (catalog_enrichment-by-norm_name + Pitfall-2 item_master no-leak); doc comment updated; held Cloth Cap + job-ok assertions and `TestRunWiki_BackfillsStale*` untouched.

## Decisions Made
- **One branched function, not two.** Kept `upsertItemAndQuests` as a single function with two tx bodies branched on `ref.Held`, rather than extracting `upsertHeld`/`upsertCatalog`. This keeps the shared `MarshalFlags` compute and the per-item tx lifecycle in one place, and lets the held body remain the prior code verbatim (its blast radius is provably zero — `git diff` of the held arm is the old text).
- **`total` slog key rename taken.** The plan offered the `item_master_total` → `total` rename as optional; took it because the coverage now genuinely spans both stores and the old key would mislead a maintainer.

## Deviations from Plan
None — both tasks executed exactly as written. No Rule 1/2/3 auto-fixes were needed; no architectural (Rule 4) decisions arose; no authentication gates.

## Issues Encountered
None — every named acceptance grep hit its exact count, the RED step failed as predicted, and the full build/vet/test suite passed first try.

## Self-Check: PASSED

- Both modified files exist on disk and carry the required markers:
  - `wiki.go` contains `ref.Held` (6×: branch + comments), exactly one each of `store.GetCatalogEnrichmentFreshnessTx` / `store.UpsertCatalogEnrichmentTx` / `s.CatalogIconCoverage`, zero `ItemMasterIconCoverage`, zero `BackfillCatalogEnrichment`; the only `ReplaceQuestItemsForIDTx` **call** is in the held branch (the catalog branch has only the explanatory NO-quest comment).
  - `wiki_test.go` contains exactly one `FROM catalog_enrichment WHERE norm_name`, one `leaked into item_master`, zero `Option A id-keying`.
- Both task commits exist in git history (`92ff217`, `6a231fe`).
- `go build ./...` OK; `go vet ./internal/backendsrv/...` clean.
- `go test ./internal/backendsrv/enrich/... ./internal/backendsrv/store/... -count=1` all green (enrich 2.0s, enrich/jobs 12.1s, politefetch 2.1s, store 71.2s).
- `git diff a3ad32d -- internal/backendsrv/store/enrich.go` is EMPTY — the held write path (`UpsertItemMasterTx`/`GetItemMasterFreshnessTx`) and `item_master` are byte-for-byte unchanged (held-reader blast radius zero).
- The 38-01 store/migration foundation (`migrations/`, `catalogenrich.go`, `itemids.go`) is untouched this wave (empty `git diff` vs wave start).
- No `.planning/STATE.md` or `.planning/ROADMAP.md` modifications (orchestrator-owned).

## User Setup Required
None — backend-only behavioral change to the existing weekly wiki job; no new migration (00017 shipped in 38-01), no external service configuration. Watcher untouched; no `v*` tag.

## Next Phase Readiness
- The name-keyed enrichment is now wired end-to-end: the weekly crawl populates `catalog_enrichment` for the full ~4,343-item PigParse Blue catalog (held items keep their `item_master` rows; the 43 formerly-dropped collision items are now covered) and the D-03 diagnostic reports both-stores coverage.
- 37 + 38 are NOT yet deployed (prod schema v15). The deploy of 37+38 and Phase 39 (Clicky/Haste faceted search reading held∪catalog by name) are gated behind this re-plan per the milestone plan — Phase 39 can now join `item_master` ∪ `catalog_enrichment` by name end-to-end.

---
*Phase: 38-catalog-wide-enrichment-icon-coverage*
*Completed: 2026-06-25*
