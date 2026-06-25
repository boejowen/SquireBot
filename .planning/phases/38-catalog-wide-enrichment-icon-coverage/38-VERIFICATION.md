---
phase: 38-catalog-wide-enrichment-icon-coverage
verified: 2026-06-25T11:35:00Z
status: passed
score: 5/5 must-haves verified
overrides_applied: 0
re_verification:
  # No previous verification existed — initial verification.
---

# Phase 38: Catalog-wide Enrichment + Icon Coverage Verification Report

**Phase Goal:** Widen item enrichment from held-only to the full PigParse Blue catalog (politefetch-paced) and backfill the wiki icon for every catalog item whose wiki page provides one, with a maintainer-visible coverage diagnostic for the icon-less residue.
**Verified:** 2026-06-25T11:35:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | The weekly wiki items pass fetches a wiki page for every item in the full PigParse Blue catalog, not only held items (ENRICH-14). | ✓ VERIFIED | `wiki.go:164` `refs, rerr := s.DistinctEnrichmentRefs(ctx)` replaced the held-only `DistinctInventoryItemIDs`. `DistinctEnrichmentRefs` (itemids.go:93-135) UNION ALLs the held arm with a `catalog` CTE selecting from `pigparse_price`. `grep -c DistinctInventoryItemIDs wiki.go`=0; `grep -c DistinctEnrichmentRefs wiki.go`=1. `TestRunWiki_EnrichesUnheldCatalogItem` PASSES — an unheld catalog row gets enriched. |
| 2 | A catalog-only (unheld) item gets an item_master row with a non-zero icon_id when its wiki page provides a lucy_img_ID (ENRICH-15 icon backfill). | ✓ VERIFIED | Icon rides the existing freshness comparison: wiki.go:296 `existingIcon == int64(item.IconID)` is unchanged; `upsertItemAndQuests` writes `IconID: item.IconID` (wiki.go:315). No new icon code, no `BackfillItemIcon` (grep=0 across `internal/backendsrv/`). `TestRunWiki_EnrichesUnheldCatalogItem` reads back `SELECT name, icon_id FROM item_master WHERE lower(trim(name))=lower(trim('Cloak of Flames'))` and asserts `icon_id > 0` — PASSES. |
| 3 | Each distinct item (by normalized name) is fetched exactly once — held and catalog rows for one item never both appear (politeness; D-04 dedup). | ✓ VERIFIED | `DistinctEnrichmentRefs` catalog arm: `GROUP BY lower(trim(name))` + `lower(trim(name)) NOT IN (SELECT norm FROM held_names)` (itemids.go:110,112). Deduped by name, NOT id. `TestDistinctEnrichmentRefs` Case B/B'/D assert exactly one ref per normalized name (incl. a casing/whitespace held-name variant) — PASSES. |
| 4 | A catalog PigParse item_id numerically equal to an existing item_master row id is EXCLUDED from the catalog arm (D-04 collision guard). | ✓ VERIFIED | itemids.go:111 `AND item_id NOT IN (SELECT item_id FROM item_master)` (grep count=2 incl. doc comment). `TestDistinctEnrichmentRefs` Case C seeds `item_master`@1001 + a DIFFERENT catalog name @ id 1001 and asserts the catalog row is excluded — PASSES. So `ON CONFLICT(item_id) DO UPDATE` can never overwrite the held row. |
| 5 | After each items pass the job logs ONE structured slog summary line with total/enriched/icon-covered/icon-less + a bounded residue name sample (ENRICH-15 / D-03). | ✓ VERIFIED | `logItemsCoverage` (wiki.go:265-273) emits `slog.Info(wikiJobName+": items coverage", "total", "enriched", "icon_covered", "icon_less", "residue_sample", ...)`. Called at runWikiItems end (wiki.go:243) and on ctx-cancel (wiki.go:181). Residue bounded at 50 via `appendBoundedResidue`/`residueSampleCap` (wiki.go:249-259). `grep -v '^//' wiki.go \| grep -c 'items coverage'`=1 (real code, not a comment). |

**Score:** 5/5 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/backendsrv/store/itemids.go` | `DistinctEnrichmentRefs` union read (held ∪ catalog, dedup by name, held wins, EQ-id-collision exclusion) | ✓ VERIFIED | Method at :93; both exclusions present (`NOT IN (SELECT norm FROM held_names)` + `NOT IN (SELECT item_id FROM item_master)`); catalog dedup `GROUP BY lower(trim(name))`; held arm byte-identical to `DistinctInventoryItemIDs`. Full doc comment on the two exclusions + politeness rule. |
| `internal/backendsrv/store/itemids_test.go` | `TestDistinctEnrichmentRefs` incl. unheld-catalog, held-wins, id-collision cases | ✓ VERIFIED | `func TestDistinctEnrichmentRefs` at :101 covers held-dedup, Case A (unheld×2), Case B/B' (held-wins + casing variant), Case C (id-collision exclusion), blank-name exclusion, Case D (one-ref-per-name). PASSES. |
| `internal/backendsrv/enrich/jobs/wiki.go` | `runWikiItems` iterating `DistinctEnrichmentRefs` + D-03 slog | ✓ VERIFIED | Loop input swapped (:164); D-03 `logItemsCoverage` emitted (:243). Job authors NO inline `pigparse_price` SQL (grep=0). No `BackfillItemIcon` boot pass. |
| `internal/backendsrv/enrich/jobs/wiki_test.go` | Widened test asserting an unheld catalog item gets enriched + iconned | ✓ VERIFIED | `TestRunWiki_EnrichesUnheldCatalogItem` (:324) seeds an unheld "Cloak of Flames" catalog row + a junk catalog name, runs RunWiki, asserts item_master row + `icon_id > 0` + held item preserved + job status "ok". `seedCatalogItem` raw-insert helper added. PASSES. |

All four artifacts: exist + substantive + wired + data flows (Level 1–4 all pass).

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `runWikiItems` | `DistinctEnrichmentRefs` | `s.DistinctEnrichmentRefs(ctx)` is the loop input | ✓ WIRED | wiki.go:164; old `DistinctInventoryItemIDs` reference removed from the job (grep=0). |
| `DistinctEnrichmentRefs` catalog arm | `item_master.item_id` | `AND item_id NOT IN (SELECT item_id FROM item_master)` collision-exclusion | ✓ WIRED | itemids.go:111; tested by Case C. |
| catalog ref's PigParse id | `item_master` row | `UpsertItemMasterTx` `ON CONFLICT(item_id) DO UPDATE` (namespace-agnostic, `ItemID: int(ref.ItemID)`) | ✓ WIRED | wiki.go:307; proven end-to-end by `TestRunWiki_EnrichesUnheldCatalogItem` reading back the row by name. |
| parsed `item.IconID` | `item_master.icon_id` via freshness short-circuit | `existingIcon == int64(item.IconID)` already in the comparison | ✓ WIRED | wiki.go:296 (unchanged); the widened ref set is the entire icon-backfill mechanism. |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|--------------|--------|--------------------|--------|
| `runWikiItems` enrichment set | `refs` | `store.DistinctEnrichmentRefs` (real `inventory_item` ∪ `pigparse_price` SQL union, ~4,341 catalog rows in prod) | Yes — tested against a real seeded SQLite | ✓ FLOWING |
| icon backfill | `item.IconID` | `enrich.ParseItempage` of the real wiki page body → `UpsertItemMasterTx` | Yes — `TestRunWiki_EnrichesUnheldCatalogItem` reads `icon_id > 0` back from the DB | ✓ FLOWING |
| D-03 coverage line | `iconCovered/iconLess/residueNames` | Loop-accumulated from each `item.IconID` + fetch-skip/parse-fail branches | Yes — real counters, bounded residue | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Union read correct (held-dedup, unheld-catalog, held-wins, id-collision, one-ref-per-name) | `go test ./store/ -run TestDistinctEnrichmentRefs` | PASS | ✓ PASS |
| Unheld catalog item enriched + iconned; junk name doesn't abort | `go test ./enrich/jobs/ -run TestRunWiki` (9 tests) | All 9 PASS (incl. EnrichesUnheldCatalogItem + 8 pre-existing) | ✓ PASS |
| Build + vet clean | `go build ./...` && `go vet ./...` | exit 0 / exit 0 | ✓ PASS |
| Full backend suite green (zero blast radius on held readers P31/32/37) | `go test ./...` | 17 packages ok, 0 FAIL | ✓ PASS |
| 00016 is the last migration (no 00017) | `ls migrations/ \| grep 00017` + goose-on-boot log | "migrated database to version: 16"; no 00017 | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| ENRICH-14 | 38-01-PLAN | Item enrichment covers the full PigParse Blue catalog, not only held items (politefetch-paced). | ✓ SATISFIED | `DistinctEnrichmentRefs` unions `pigparse_price` into the crawl set; runWikiItems iterates it at the existing 1s `wikiSleepFn` courtesy pace (unchanged). REQUIREMENTS.md:58 maps ENRICH-14→Phase 38. |
| ENRICH-15 | 38-01-PLAN | Icon coverage backfilled for every item whose wiki page provides one; maintainer can see the icon-less residue. | ✓ SATISFIED | Icon backfill rides the existing freshness comparison (no new code); D-03 `logItemsCoverage` slog line exposes total/enriched/icon_covered/icon_less + bounded residue sample. REQUIREMENTS.md:59 maps ENRICH-15→Phase 38. |

No orphaned requirements: REQUIREMENTS.md maps exactly ENRICH-14/15 to Phase 38, both declared in the PLAN `requirements` frontmatter and both verified.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| — | — | — | — | None. No TODO/FIXME/placeholder/stub patterns introduced. `iconCovered, iconLess := 0, 0` and `residueNames := make([]string, 0, 64)` are loop-accumulator initial state populated each iteration (not stubs). The `slog.Info` line emits real counters. |

### CLAUDE.md Compliance

| Rule | Status | Evidence |
|------|--------|----------|
| Extend-only schema (no migration) | ✓ | 00016 is the last migration on disk; no 00017; goose boots to v16. |
| Structured slog (Go side) | ✓ | `logItemsCoverage` uses keyed `slog.Info` fields; logs counts + IDs + public wiki page NAMES only (no statsblock/wikitext bodies — T-38-03 honored). |
| Watcher UNTOUCHED | ✓ | Only 4 files changed, all under `internal/backendsrv/`. No `internal/app`, `internal/watch`, `internal/eqfind`, `internal/update`, `cmd/squirebot` touched (`internal/sheet` already gone post-v2.0). |
| No `v*` tag | ✓ | `git tag --points-at` on both commits (59e7180, ac85211) returns empty. |
| No `WatcherMaxSchemaVersion` task (moot post-v2.0) | ✓ | The only references are pre-existing explanatory comments in `migrate_test.go` (not touched by Phase 38); `internal/sheet` is gone. |

### Human Verification Required

None. All must-haves are verifiable programmatically and were confirmed by passing tests + grep evidence against real source. The phase is backend-only, automated, and has no UI/visual/real-time/external-service surface that requires manual testing.

### Gaps Summary

No gaps. All 5 observable truths VERIFIED, all 4 artifacts pass Levels 1–4, all 4 key links WIRED, both requirements SATISFIED, full backend suite green (17 packages, 0 fail), and all CLAUDE.md constraints honored (extend-only/no migration, structured slog, watcher untouched, no `v*` tag).

**Note (operational, not a gap):** The D-04 prod collision probe was DEFERRED per the plan's documented disposition — prod SSH was unavailable in the non-interactive execution environment. The EQ-id-collision exclusion is the safe default and is correct independent of the count (verified by Case C). The SUMMARY records the deferral and the instruction to run the read-only probe before the next backend deploy. This is a pre-deploy operational checklist item, not a code gap — the code is correct regardless of the probe's outcome (a material count would only mean more catalog names land icon-less and surface in the D-03 residue, which the plan's escalation path covers).

---

_Verified: 2026-06-25T11:35:00Z_
_Verifier: Claude (gsd-verifier)_
