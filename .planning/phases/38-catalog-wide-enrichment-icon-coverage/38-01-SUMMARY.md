---
phase: 38-catalog-wide-enrichment-icon-coverage
plan: 01
subsystem: database
tags: [go, sqlite, goose, enrichment, name-keyed, catalog_enrichment, item_master, icon-coverage]

# Dependency graph
requires:
  - phase: 37-item-enrichment-backbone-flags-effects
    provides: item_master flags/effects columns (00016) + GetItemMasterFreshnessTx 4-field self-heal + store.MarshalFlags canonical encoder
  - phase: 31-item-icon (00012)
    provides: item_master.icon_id nullable column + the "NULL/0 = colored-tile fallback" contract + freshness-comparison precedent
provides:
  - "Migration 00017: additive catalog_enrichment table keyed by norm_name (lower(trim(name))) carrying the full item_master enrichment column set + representative name/PigParse id"
  - "store.CatalogEnrichment struct + UpsertCatalogEnrichmentTx (ON CONFLICT(norm_name)) + GetCatalogEnrichmentFreshnessTx (4-field self-heal) — the name-keyed parallel of the held item_master path"
  - "store.EnrichmentRef{ItemID,Name,Held} + DistinctEnrichmentRefs returning []EnrichmentRef with the Option-A id-collision guard DROPPED"
  - "store.CatalogIconCoverage — icon coverage counted across BOTH item_master and catalog_enrichment"
affects: [38-02 (branches the wiki.go items-pass write on ref.Held), 39 (Clicky/Haste faceted search reads held∪catalog by name)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Name-keyed enrichment store: catalog-only (unheld) items keyed by norm_name in a SEPARATE table; item_master stays held-only by EQ item_id (zero held-reader blast radius)"
    - "Parallel freshness short-circuit per store (sha|icon|stats|flags), the icon carried for free in both"
    - "Both-stores coverage UNION ALL (held names never in catalog_enrichment → one row per item, no precedence logic)"

key-files:
  created:
    - internal/backendsrv/migrations/00017_catalog_enrichment.sql
    - internal/backendsrv/store/catalogenrich.go
    - internal/backendsrv/store/catalogenrich_test.go
  modified:
    - internal/backendsrv/store/itemids.go
    - internal/backendsrv/store/itemids_test.go
    - internal/backendsrv/migrations/migrate_test.go
    - internal/backendsrv/enrich/jobs/wiki.go

key-decisions:
  - "Kept the dropped-collision-guard rationale + EnrichmentRef.Held purpose in itemids.go docstrings even though the literal prose trips two of the plan's text-count acceptance greps — the SQL guard IS removed and the field IS present; documentation > literal grep count"
  - "Added a minimal behavior-preserving shim in wiki.go (adapt EnrichmentRef→ItemRef at the upsertItemAndQuests call site + rename the coverage call to CatalogIconCoverage) to keep `go build ./...` + the held-path tests green; Plan 02 replaces the shim with the real ref.Held write branch"

patterns-established:
  - "Name-keyed catalog_enrichment mirrors item_master re-PK'd on norm_name; held path byte-for-byte unchanged"
  - "EnrichmentRef carries a Held flag so the write path branches by held-ness without re-querying"

requirements-completed: [ENRICH-14, ENRICH-15]

# Metrics
duration: 10min
completed: 2026-06-25
---

# Phase 38 Plan 01: Catalog-wide Enrichment — Name-Keyed Foundation Summary

**A new name-keyed `catalog_enrichment` table (migration 00017) + its upsert/freshness store layer mirroring `item_master` on `norm_name`, with `DistinctEnrichmentRefs` reworked to drop the Option-A id-collision guard, carry a `Held` flag, and count icon coverage across both stores — `item_master` left byte-for-byte unchanged.**

## Performance

- **Duration:** ~10 min
- **Started:** 2026-06-25T18:18:59Z
- **Completed:** 2026-06-25T18:28:38Z
- **Tasks:** 3 of 3
- **Files modified:** 7 (3 created, 4 modified)

## Accomplishments
- **Migration 00017** creates the additive `catalog_enrichment(norm_name PK, …)` table (20 columns = the full `item_master` enrichment shape re-keyed on the normalized name + representative `name`/PigParse `item_id`). No ALTER of any existing table; goose Up/Down; no `WatcherMaxSchemaVersion` gate; no `v*` tag.
- **`store/catalogenrich.go`** — `CatalogEnrichment` struct + `UpsertCatalogEnrichmentTx` (`ON CONFLICT(norm_name)`, `?`-bound untrusted text, `b2i` booleans reused) + `Store.UpsertCatalogEnrichment` wrapper + `GetCatalogEnrichmentFreshnessTx` (4-field sha|icon|stats|flags self-heal). The exact name-keyed parallel of the held `item_master` path. No `quest_items` write (the catalog path stays out of the EQ-namespace quest table).
- **`store/itemids.go` reworked** — new `EnrichmentRef{ItemID,Name,Held}` struct; `DistinctEnrichmentRefs` returns `[]EnrichmentRef`, both arms emit a `held` literal, and the Option-A `item_id NOT IN (SELECT item_id FROM item_master)` collision guard is DROPPED (recovering the formerly-dropped colliding catalog items). `ItemMasterIconCoverage` → `CatalogIconCoverage` now `UNION ALL`s `item_master` + `catalog_enrichment` for both the count and the residue sample.
- **Held-reader blast radius is provably zero** — `git diff --stat internal/backendsrv/store/enrich.go` shows no change; the held write path (`UpsertItemMasterTx`/`GetItemMasterFreshnessTx`) and the `item_master` schema are byte-for-byte untouched.

## Task Commits

1. **Task 1: Migration 00017 + TestMigrate_00017** — `00636f2` (feat)
2. **Task 2: catalog_enrichment store layer (upsert + freshness) + tests** — `c8c8755` (feat, TDD — implementation + 4 behaviors in one commit)
3. **Task 3: itemids.go rework (drop guard, Held flag, both-stores coverage) + wiki.go shim + tests** — `b019c50` (feat)

**Plan metadata commit:** created with this SUMMARY (the orchestrator owns STATE/ROADMAP writes for this project).

## Files Created/Modified
- `internal/backendsrv/migrations/00017_catalog_enrichment.sql` — additive `catalog_enrichment(norm_name PK, …)` table (goose Up/Down).
- `internal/backendsrv/store/catalogenrich.go` — `CatalogEnrichment` + `UpsertCatalogEnrichmentTx` + `GetCatalogEnrichmentFreshnessTx` + `Store.UpsertCatalogEnrichment`.
- `internal/backendsrv/store/catalogenrich_test.go` — round-trip / absent-row zero-values / `ON CONFLICT(norm_name)` update-in-place / 4-field icon+flags self-heal.
- `internal/backendsrv/store/itemids.go` — `EnrichmentRef` struct; `DistinctEnrichmentRefs` (`[]EnrichmentRef`, guard dropped, `held` literal); `CatalogIconCoverage` (both-stores UNION ALL).
- `internal/backendsrv/store/itemids_test.go` — flipped Case C (colliding catalog name now INCLUDED, `Held=false`), added `Held` assertions, `TestCatalogIconCoverage` seeds + spans both stores (`seedCatalogEnrich` helper).
- `internal/backendsrv/migrations/migrate_test.go` — `TestMigrate_00017_AddsCatalogEnrichment` (table + 20 columns + created-empty + `ON CONFLICT(norm_name)` one-row + idempotency tail).
- `internal/backendsrv/enrich/jobs/wiki.go` — minimal compile-keeping shim (adapt `EnrichmentRef`→`store.ItemRef` at the held-path call site; rename the coverage call to `CatalogIconCoverage`). Behavior preserved; Plan 02 replaces it with the `ref.Held` branch.

## Decisions Made
- **Job write branch deferred to Plan 02 (per the plan's wave split).** Plan 01 lands the store/migration foundation only. To keep `go build ./...` and the held-path tests green after the signature changes, `wiki.go` got a behavior-preserving shim rather than the real branch. This is the intended Plan 01/02 boundary — the existing Option-A `wiki_test.go` (`TestRunWiki_EnrichesUnheldCatalogItem`) still passes because the shim preserves the prior runtime behavior; Plan 02 flips that test to assert the catalog row lands in `catalog_enrichment`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Kept `wiki.go` compiling after the store signature changes**
- **Found during:** Task 3 (itemids.go rework)
- **Issue:** Changing `DistinctEnrichmentRefs` to return `[]EnrichmentRef` and renaming `ItemMasterIconCoverage`→`CatalogIconCoverage` broke the build at the two call sites in `enrich/jobs/wiki.go` (lines 200, 227). Plan 01's success criteria require `go build ./...` green, but the real `ref.Held` write branch is explicitly Plan 02's scope.
- **Fix:** A minimal, behavior-preserving shim — adapt the new `EnrichmentRef` to the unchanged held-path `upsertItemAndQuests` via `store.ItemRef{ItemID: ref.ItemID, Name: ref.Name}`, and rename the coverage call to `CatalogIconCoverage`. The held-only runtime behavior is identical; Plan 02 replaces the shim with the `ref.Held` branch.
- **Files modified:** `internal/backendsrv/enrich/jobs/wiki.go`
- **Verification:** `go build ./...` OK; `go test ./internal/backendsrv/enrich/jobs/...` green (the Option-A `wiki_test.go` still passes).
- **Committed in:** `b019c50` (Task 3 commit)

### Acceptance-criteria text-count notes (no functional impact)

Two of Task 3's literal `grep -c` acceptance counts do not hit their stated numbers, both because of explanatory comments / gofmt alignment — the underlying functional requirements ARE met:

- `grep -c "item_id NOT IN (SELECT item_id FROM item_master)" itemids.go` returns **1** (plan expected 0). The single match is in the new **docstring** ("The Option-A id-collision guard (…) is DROPPED"), which documents *why* the guard is gone. The SQL query no longer contains the guard (verified by reading the `catalog` CTE). Keeping the prose was a deliberate documentation choice.
- `grep -c "Held bool" itemids.go` returns **0** (plan expected ≥1). The field exists as `Held   bool` (gofmt-aligned with spaces); the single-space literal simply doesn't match the aligned form. The `must_haves` artifact (`contains: "type EnrichmentRef"`) and the `Held` field are both present and exercised by the tests.

These are literal-text-count artifacts, not defects. The `must_haves` truths/artifacts/key_links (incl. the `ON CONFLICT\(norm_name\)` and `UNION ALL` patterns) all hold.

---

**Total deviations:** 1 auto-fixed (Rule 3 — blocking build) + 2 text-count notes (no code change needed).
**Impact on plan:** The shim is required to satisfy Plan 01's "build green" criterion without doing Plan 02's work; it preserves behavior exactly. No scope creep.

## Issues Encountered
None — all three tasks executed per the plan's concrete actions; every named verification and the full build/vet/test suite passed first try.

## Self-Check: PASSED

- All 7 key files exist on disk (verified).
- All 3 task commits exist in git history (`00636f2`, `c8c8755`, `b019c50`).
- `go build ./...` OK; `go vet ./internal/backendsrv/...` clean.
- `go test ./internal/backendsrv/migrations/... ./internal/backendsrv/store/... ./internal/backendsrv/enrich/... -count=1` all green (migrations 15.9s, store 70.6s, enrich + jobs + politefetch all ok).
- `git diff --stat internal/backendsrv/store/enrich.go` empty (held path byte-for-byte unchanged — held-reader blast radius zero).
- No `.planning/STATE.md` or `.planning/ROADMAP.md` modifications (orchestrator-owned).

## User Setup Required
None — backend-only, additive migration (goose-on-boot), no external service configuration. Watcher untouched; no `v*` tag.

## Next Phase Readiness
- The name-keyed foundation is in place: Plan 02 (Wave 2, depends_on 38-01) can now branch the `wiki.go` items-pass write on `ref.Held` (held → `item_master`+`quest_items` unchanged; catalog-only → `catalog_enrichment` by `norm_name`, no quest write) and re-point the D-03 coverage `slog` to the both-stores `CatalogIconCoverage`, then flip `wiki_test.go`'s `TestRunWiki_EnrichesUnheldCatalogItem` to assert the catalog row lands in `catalog_enrichment`.
- Migration 00017 + this code are NOT yet deployed (prod schema v15); the 37+38 deploy is gated behind Plan 02 per the milestone plan.

---
*Phase: 38-catalog-wide-enrichment-icon-coverage*
*Completed: 2026-06-25*
