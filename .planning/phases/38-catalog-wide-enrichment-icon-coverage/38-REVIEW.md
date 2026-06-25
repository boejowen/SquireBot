---
phase: 38-catalog-wide-enrichment-icon-coverage
reviewed: 2026-06-25T00:00:00Z
resolution: "MD-01/MD-02/LO-01/LO-02 fixed-forward 2026-06-25 (commit 6d6588f); NIT-01 dissolved by that fix; NIT-02 → backlog 999.42"
depth: standard
mode: advisory
files_reviewed: 4
files_reviewed_list:
  - internal/backendsrv/store/itemids.go
  - internal/backendsrv/store/itemids_test.go
  - internal/backendsrv/enrich/jobs/wiki.go
  - internal/backendsrv/enrich/jobs/wiki_test.go
findings:
  blocker: 0
  high: 0
  medium: 2
  low: 2
  nit: 2
  total: 6
status: issues_found
---

# Phase 38: Code Review Report — Catalog-wide enrichment + icon coverage

**Reviewed:** 2026-06-25
**Depth:** standard
**Mode:** advisory (never blocks)
**Files Reviewed:** 4
**Status:** issues_found (0 BLOCKER / 0 HIGH — the load-bearing data-integrity guard is correct)

## Summary

The change is exactly what the plan promised: a single store union read
(`DistinctEnrichmentRefs`) + a one-line loop-input swap in `runWikiItems` + the D-03
coverage `slog` line. I traced the data-integrity guard that this whole phase rests on
and it holds: the catalog arm genuinely excludes both held normalized names and any
`pigparse_price.item_id` already present in `item_master`, so `ON CONFLICT(item_id) DO
UPDATE` can never overwrite a held item's enrichment with a different PigParse item. The
SQL is parameterless / fixed-projection (no injection surface), the dedup is by
`lower(trim(name))` (not by id, so each wiki page is fetched at most once), and the D-03
line logs only public page names with a bounded (≤50) residue sample — V7 / T-38-03 /
T-38-04 honored. The loop body downstream of the swap is byte-for-byte the prior behavior
(304 short-circuit, courtesy sleep, log-and-skip, ETag persistence). `go build`, `go vet`,
and the full backend `go test ./...` (17 packages) are all green; the new tests assert the
four D-04 invariants (held-wins, unheld→keyed-by-PigParse-id, id-collision exclusion,
one-ref-per-normalized-name) plus blank-name exclusion and the unheld→icon_id>0 win.

**No BLOCKER or HIGH findings.** The two MEDIUM findings are a diagnostic-semantics mismatch
in the D-03 counters (the counts mean "parsed this pass", not "current catalog state", which
diverges on steady-state weekly runs) and a docstring that over-claims a "fetched EXACTLY
ONCE" invariant the held arm cannot guarantee. Neither affects correctness of the enrichment
data; both affect how the maintainer should read the coverage log. The rest are LOW/NIT.

## Medium

### MD-01: D-03 coverage counters report "items parsed THIS pass", not "current catalog coverage" — misleading on every steady-state weekly run

**File:** `internal/backendsrv/enrich/jobs/wiki.go:188-217, 261-273`
**Issue:** `iconCovered` / `iconLess` are incremented ONLY for items that reach the parse
branch this pass. The dominant steady-state path is `fetchUnchanged` (ETag 304), which is
counted as `unchanged++` but is deliberately NOT classified as covered or icon-less (the
comment at :189-190 acknowledges this). Consequences once the catalog is seeded:
- On a weekly 304-heavy run, `icon_covered` and `icon_less` will both be near-zero and the
  `residue_sample` near-empty, even though hundreds of items genuinely render the colored
  tile. The maintainer greps the line to answer "which items are still icon-less" (the
  literal ENRICH-15 / D-03 goal) and gets an answer that reflects only the handful of pages
  that changed that week — not the residue they care about.
- The plan's PART-B field semantics describe a store snapshot ("items whose
  `item_master.icon_id > 0`" / "0/NULL OR no row at all"), but the implementation describes
  this-pass parse outcomes. They coincide only on the first seed run.

The plan explicitly sanctioned loop-accumulation ("Pick the loop-accumulation path unless it
materially complicates the loop"), so this is within bounds — but the field *names*
(`icon_covered`, `icon_less`) and the doc comment ("the colored-tile set") promise a
catalog-state reading the numbers don't deliver after week 1.
**Fix:** Either (a) rename the fields to reflect their true meaning (e.g.
`icon_covered_this_pass` / `icon_less_this_pass`) and drop the "the colored-tile set"
framing, or (b) compute true coverage with one post-loop store read over `item_master ⋈ the
union names` (the plan's first PART-B option) so the line answers the maintainer's actual
question every week. Given ENRICH-15's literal "a maintainer can SEE which items are still
icon-less," option (b) is the stronger fit; if (a) is chosen, also fold the 304'd-but-stored
icon-less rows into the residue so the steady-state log stays useful. Advisory — does not
block; the data written to `item_master` is correct regardless.

### MD-02: `DistinctEnrichmentRefs` doc claims each page is "fetched EXACTLY ONCE", but the held arm (GROUP BY item_id) can emit one normalized name twice

**File:** `internal/backendsrv/store/itemids.go:65-70` (and the `held` CTE at the query, `GROUP BY item_id`)
**Issue:** The catalog arm correctly dedups by `lower(trim(name))`, but the held arm is
`GROUP BY item_id` (inherited verbatim from `DistinctInventoryItemIDs`). If two DISTINCT EQ
`item_id`s ever share a normalized name (e.g. id 1001 "Cloth Cap" and a different id "cloth
cap" — same display name, different uploaded id), the held arm emits BOTH, the union contains
two refs for one normalized name, and the wiki page is fetched twice. The method's docstring
asserts the stronger "deduped by normalized name ... so each wiki page is fetched EXACTLY
ONCE" and the `held_names` exclusion only protects the *catalog* arm from re-adding a held
name — it does nothing about a held-vs-held name collision. The Case D test ("exactly one ref
per normalized name") only seeds distinct names per held id, so it never exercises this path
and the assertion would silently pass even though the invariant is held-arm-conditional.

In practice EQ item names are effectively 1:1 with item ids, so the real-world probability is
low and this is NOT a regression (`DistinctInventoryItemIDs` has always been id-keyed). But
the docstring over-promises and the test gives false confidence in an invariant the SQL does
not enforce across the held arm.
**Fix:** Soften the docstring to state the dedup-by-name guarantee applies to the held↔catalog
boundary and within the catalog arm, while the held arm dedups by EQ id (the pre-existing
politeness rule) — OR, if the stronger guarantee is wanted, make the held arm also collapse by
normalized name (`GROUP BY lower(trim(name))`, keeping a MIN(item_id) representative) and add a
held-arm duplicate-name test fixture (two held ids, same normalized name) proving Case D still
holds. Advisory.

## Low

### LO-01: `iconLess`/`residue` and `failed` double-count the same item, and `enriched`/`icon_*` use inconsistent denominators — the line's internal arithmetic doesn't reconcile

**File:** `internal/backendsrv/enrich/jobs/wiki.go:193-217, 219-225, 265-272`
**Issue:** An item that parses OK with `IconID==0` is counted `iconLess++` + added to residue
(at :214-216), and if its subsequent `upsertItemAndQuests` returns a DB error it is ALSO
counted `failed++` (at :222-224). Separately, a `fetchSkip` item is `failed++`+`iconLess++`.
So `icon_covered + icon_less` does not partition `total`, and `enriched (= written+unchanged)`,
`icon_covered/icon_less`, and the (unlogged) `failed` count overlap. A maintainer cannot do
`total == enriched + icon_less` or any clean reconciliation from the line. This is a
readability/diagnostic-trust issue, not a data bug.
**Fix:** Document in the `logItemsCoverage` comment that the counters are non-partitioning
(an item can contribute to more than one), or add the `failed`/`written`/`unchanged` raw
counts to the same line so the maintainer can cross-check (the sibling `wiki_weekly: ok`
detail line already carries `items_fail`; consider referencing it). Advisory.

### LO-02: `TestRunWiki_EnrichesUnheldCatalogItem` does not assert the catalog row is keyed by its PigParse id, nor that the D-03 line fired — the id-keying half of D-04 Option A is untested at the job level

**File:** `internal/backendsrv/enrich/jobs/wiki_test.go:316-380`
**Issue:** The test proves the unheld catalog item gets an `item_master` row with `icon_id>0`
(the ENRICH-15 win) and that a junk name doesn't abort — good. But it looks the row up by
`lower(trim(name))` and never asserts `item_id == 90950` (the PigParse id), so a regression
that keyed the catalog row by some other id (or collided it onto a held id) would still pass.
The store-level Case C covers the *exclusion*, but the job-level *admission keyed by PigParse
id* (the core of Option A) is only implicitly exercised. The D-03 `items coverage` line is
also unasserted (no log capture) — acceptable per package convention (no slog-capture harness
exists), but it means the bounded-residue / public-names-only behavior is verified by reading
the code, not by a test.
**Fix:** Add `if id != 90950 { t.Errorf(...) }` to the `SELECT name, item_id, icon_id` read-back
so the PigParse-id keying is pinned. (Optional, larger:) introduce a slog capture handler to
assert the coverage line's fields and the ≤50 residue bound. Advisory.

## Nit

### NIT-01: `residueNames` initial capacity (64) exceeds the hard cap (50) — a wasted 14-slot allocation

**File:** `internal/backendsrv/enrich/jobs/wiki.go:175, 249-258`
**Issue:** `residueNames := make([]string, 0, 64)` pre-allocates 64 slots, but
`appendBoundedResidue` never lets the slice exceed `residueSampleCap = 50`. Harmless, but the
capacity should match the cap for clarity (and to make the cap the single source of truth).
**Fix:** `make([]string, 0, residueSampleCap)`.

### NIT-02: store-test imports `strings` solely for a one-line `normEnrich` helper that re-implements the SQL key — fine, but worth a comment tie-back

**File:** `internal/backendsrv/store/itemids_test.go:6, 12`
**Issue:** `normEnrich` mirrors `lower(trim(name))` in Go to drive the multiset assertions.
It's correct and well-commented, but if the SQL dedup key ever changes (e.g. NFC
normalization), this helper silently drifts. Minor.
**Fix:** None required; optionally add `// keep in sync with the catalog CTE's GROUP BY
lower(trim(name))` cross-reference to the union method. Advisory.

---

## Verification performed

- `go build ./...` — exit 0.
- `go vet ./store/ ./enrich/jobs/` — clean.
- `go test ./store/ -run TestDistinctEnrichmentRefs -v` — PASS.
- `go test ./enrich/jobs/ -run TestRunWiki -v` — PASS (new + all existing TestRunWiki_*).
- `go test ./...` (whole backend) — 17 packages, all `ok`, 0 FAIL.
- Acceptance greps confirmed: `DistinctInventoryItemIDs` gone from the job (0), no
  `pigparse_price` inline SQL in the job (0), no `BackfillItemIcon` (none), no 00017
  migration (0), both D-04 exclusions present in the union SQL.

## What was verified correct (the load-bearing claims held)

- **id-collision exclusion works:** `item_id NOT IN (SELECT item_id FROM item_master)` is
  present and Case C proves a PigParse id equal to an existing `item_master` row id is dropped
  before the upsert — `ON CONFLICT(item_id)` cannot overwrite a held row.
- **held-name exclusion / held-wins:** `lower(trim(name)) NOT IN (SELECT norm FROM
  held_names)` + Case B/B' prove a name that is both held and in the catalog yields exactly one
  ref keyed by the held EQ id; casing/whitespace variants are excluded.
- **dedup by name, not id:** the catalog arm `GROUP BY lower(trim(name))` — no per-(id,name)
  refetch politeness regression in the catalog arm.
- **SQL injection surface:** none — parameterless, fixed projection, only column-name literals;
  downstream upsert binds via `?` placeholders (unchanged).
- **D-03 V7 / DoS guards:** logs counts + public page NAMES only (no statsblock/wikitext
  bodies); residue bounded at ≤50 via `appendBoundedResidue`.
- **loop body unchanged:** courtesy sleep, ETag 304 short-circuit, `ParseItempage` log-and-skip,
  ETag-persist-after-fetch, and the freshness short-circuit (which already carries `icon_id`)
  are byte-for-byte the prior behavior — no politeness regression on the widened set.
- **no goroutine/perf correctness issue:** the ~4,341-row iteration is a serial loop over a
  fully-materialized `[]ItemRef`; the residue slice is capped; no shared mutable state, no
  goroutine, no unbounded growth.

---

_Reviewed: 2026-06-25_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard · Mode: advisory_
