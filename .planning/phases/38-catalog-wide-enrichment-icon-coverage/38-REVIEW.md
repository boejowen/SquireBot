---
phase: 38-catalog-wide-enrichment-icon-coverage
reviewed: 2026-06-25T00:00:00Z
depth: deep
files_reviewed: 8
files_reviewed_list:
  - internal/backendsrv/migrations/00017_catalog_enrichment.sql
  - internal/backendsrv/migrations/migrate_test.go
  - internal/backendsrv/store/catalogenrich.go
  - internal/backendsrv/store/catalogenrich_test.go
  - internal/backendsrv/store/itemids.go
  - internal/backendsrv/store/itemids_test.go
  - internal/backendsrv/enrich/jobs/wiki.go
  - internal/backendsrv/enrich/jobs/wiki_test.go
findings:
  critical: 0
  warning: 2
  info: 4
  total: 6
status: resolved
resolution: "WR-01/WR-02 (one root cause) fixed-forward 9735f59 (key catalog_enrichment by ref.Name not item.ItemName + divergent-name regression test). IN-01 fixed-forward c7dae81 (docstring honesty). IN-02 (redundant slog) + IN-03 (int narrowing) → backlog 999.43 (cosmetic). IN-04 (catalog quest-links discarded) = no action — research-ratified intentional scope, confirmed vs Phase-39 Clicky/Haste facets. 0 BLOCKER, 0 open WARNING. Phase not deployed when fixed."
---

# Phase 38: Code Review Report

> **RESOLVED 2026-06-25.** WR-01/WR-02 (the single name-keying root cause) fixed-forward in `9735f59`
> — `catalog_enrichment` now keys on the PigParse `ref.Name` (the same name the union dedups on +
> Phase 39 joins on), with `item.ItemName` kept as the display column, plus
> `TestUpsertItemAndQuests_CatalogKeyedByRefName` (divergent PigParse-vs-wiki name). IN-01 (docstring
> over-claim) fixed-forward in `c7dae81`. IN-02 (redundant `slog.Error`) + IN-03 (`int(ref.ItemID)`
> narrowing, identical to the held path) → **backlog 999.43** (cosmetic). IN-04 (catalog quest links
> discarded) = no action — research-ratified intentional scope, confirmed against Phase-39 facets.

**Reviewed:** 2026-06-25
**Depth:** deep (cross-file: store ↔ job ↔ parser ↔ migration, plus held-reader blast-radius trace)
**Files Reviewed:** 8 (3 created, 5 modified)
**Status:** issues_found

## Summary

Phase 38 is the NAME-KEYED re-implementation (D-04 reversed from id-keyed). I reviewed it against the
eight load-bearing focus areas plus a full held-reader blast-radius trace. The structural goals are met
and verified:

- **Held-reader blast radius is provably zero.** `git diff 8217504..HEAD` of `store/enrich.go`,
  `store/readviews.go`, `store/backfill.go`, `compute/itemrollup.go`, and `cmd/` is EMPTY, and the parser
  `enrich/wikiitem.go` is unchanged. `item_master` keeps its EQ-id keying byte-for-byte. (Focus 7.)
- **Migration 00017** is additive (new table only), goose Up/Down correct, no ALTER/DROP of any existing
  table; `TestMigrate_00017` proves create-empty + `ON CONFLICT(norm_name)` + goose idempotency. (Focus 7.)
- **The write branch keys strictly on `ref.Held`** (wiki.go:295), not on row existence — Pitfall 2 is
  respected; held → `item_master`+`quest_items`, catalog-only → `catalog_enrichment`, no quest write. (Focus 1.)
- **The Option-A collision guard is genuinely dropped** from the `catalog` CTE (itemids.go:132-138) — the
  `item_id NOT IN (...)` exclusion is GONE from the SQL; `TestDistinctEnrichmentRefs` Case C is flipped to
  assert the colliding catalog row is now INCLUDED with `Held=false`. The shim from 38-01 is fully removed
  (no `store.ItemRef{}` adapter, no `ItemMasterIconCoverage`, no `BackfillCatalogEnrichment` anywhere). (Focus 8.)
- **Freshness self-heal is a byte-identical 4-field parallel** of the held getter (catalogenrich.go:132-144
  vs enrich.go:282-294 — same SELECT, same `NullString`/`NullInt64` zero-values). All four fields
  (sha/icon/stats/flags) participate in the compare on both branches (wiki.go:305-306). (Focus 3.)
- **V5/V7 are clean.** All 20 untrusted values bind through `?` placeholders in `catalogEnrichmentUpsert`
  (20 placeholders, 20 columns); icon coerces through `parseIconID` to a non-negative sentinel-0; the
  coverage slog logs counts + public page names only, never statsblock/wikitext bodies. (Focus 5, 6.)
- `go build`, `go vet`, and the full migrations/store/jobs test suites all pass.

The two WARNINGs both stem from ONE root cause: the catalog write path keys `catalog_enrichment` by the
**wiki-parsed** name (`item.ItemName`), while the union's held-name dedup operates on the **PigParse**
name (`ref.Name`). When those two names normalize differently — which `&redirects=true` and the
`itemname=` template param make possible — the "catalog_enrichment never holds a held name" invariant that
the coverage UNION and the Phase-39 read both depend on can be violated, and two distinct catalog rows can
silently collapse into one. Neither corrupts held data or crashes, so neither is a BLOCKER, but both should
be addressed (or explicitly accepted) before the Phase-39 join is built on top of this contract.

The two 38-01 deviations are confirmed cosmetic/resolved: the bridge shim is GONE in the final tree, and
the two grep-count notes (a docstring mention of the dropped guard + a gofmt-aligned `Held bool`) are
documentation/formatting only — the SQL guard is removed and the field is present and exercised.

## Warnings

### WR-01: Catalog row keyed by the wiki-parsed name (`item.ItemName`), not the deduped PigParse name (`ref.Name`)

**File:** `internal/backendsrv/enrich/jobs/wiki.go:300`
**Issue:**
The catalog branch computes its primary key from the *wiki* name:
```go
norm := strings.ToLower(strings.TrimSpace(item.ItemName))
```
where `item.ItemName` is `getParam(params, "itemname", pageTitle)` — the `{{Itempage|itemname=...}}`
template param, falling back to the redirect-resolved page title (wikiitem.go:104,120). The wiki URL is
fetched with `&redirects=true` (jobs/urls.go:54), so the canonical title (and the `itemname` param) can
differ from the PigParse catalog name `ref.Name`.

But the union's held-name exclusion (itemids.go:136) dedups the catalog arm on the **PigParse** name:
`lower(trim(pigparse.name)) NOT IN (SELECT norm FROM held_names)`. So the dedup and the write key are
computed from two different strings. The RESEARCH's load-bearing invariant — "catalog_enrichment NEVER
holds a held name, so a plain UNION ALL yields one row per item, no precedence logic" (38-RESEARCH §"The
Phase-39 read contract", and the basis for `CatalogIconCoverage` and the entire Phase-39 COALESCE) — is
only guaranteed when `lower(trim(ref.Name)) == lower(trim(item.ItemName))`.

Concretely, when a catalog item's PigParse spelling is unheld but its wiki canonical/`itemname`
normalizes to a **held** item's name (e.g. PigParse "Cloak Of Flame" redirecting to the held "Cloak of
Flames" page), the catalog branch writes a `catalog_enrichment` row under a held `norm_name`. That row
then double-counts in `CatalogIconCoverage` (the UNION ALL counts it once in `item_master` AND once in
`catalog_enrichment`) and, more importantly, breaks the collision-free precondition Phase 39 is told it
can rely on.

`TestRunWiki_EnrichesUnheldCatalogItem` does NOT catch this: its fixture's `itemname` is exactly
"Cloak of Flames", equal to the seeded PigParse name, so the two strings coincide by fixture luck rather
than by the code being divergence-safe.

The held path is immune (it keys `item_master` by `ref.ItemID`, never by the parsed name), which is why
this surfaces only on the new catalog branch.

**Fix:** Key the catalog row on the SAME normalized name the union deduped on — `ref.Name` — so the write
key and the held-name exclusion are computed from one string. Carry the parsed `item.ItemName` only as the
non-key representative `Name` display column:
```go
// norm is the SAME normalized name the union deduped on (ref.Name, the PigParse
// spelling), so the held-name exclusion and the catalog_enrichment PK are computed
// from ONE string — guaranteeing catalog_enrichment never holds a held name (the
// invariant CatalogIconCoverage + the Phase-39 COALESCE both depend on).
norm := strings.ToLower(strings.TrimSpace(ref.Name))
...
store.CatalogEnrichment{
    NormName: norm,
    Name:     item.ItemName, // representative display name (wiki casing) — NON-key
    ...
}
```
Then add a regression test whose fixture `itemname` deliberately differs from the seeded PigParse name
(e.g. PigParse "cloak of flame" → fixture itemname "Cloak of Flames") and assert the row is keyed by
`lower(trim(ref.Name))`, and that a catalog row whose wiki name normalizes to a held name does NOT appear
in `catalog_enrichment`.

### WR-02: Two distinct catalog rows that resolve to the same wiki page silently collapse via `ON CONFLICT(norm_name)`

**File:** `internal/backendsrv/enrich/jobs/wiki.go:300-334`
**Issue:**
Same root cause as WR-01, second consequence. The union dedups the catalog arm by `lower(trim(pigparse.name))`,
so two PigParse rows with *different* names (e.g. "Manastone" and "Mana Stone") are BOTH emitted as
distinct catalog refs. If both wiki pages redirect to the same canonical page (or both `itemname=` params
normalize to the same string), both catalog branches compute the identical `norm_name`, and the second
`UpsertCatalogEnrichmentTx` `ON CONFLICT(norm_name) DO UPDATE` (catalogenrich.go:78) silently overwrites
the first. The two items collapse into one `catalog_enrichment` row and the coverage `Total` under-counts
by one — with no log line indicating an overwrite occurred.

This is lower-impact than WR-01 (it is an under-count / merge of two genuinely-distinct catalog rows, not
a held-data corruption), but it is still a silent correctness loss and a coverage-diagnostic inaccuracy
(ENRICH-15's whole point is an accurate icon-coverage count).

**Fix:** Fixing WR-01 (key on `ref.Name`) removes the redirect-driven half of this. The residual
"two PigParse spellings that normalize identically" case is already collapsed by the union's
`GROUP BY lower(trim(name))`, so after the WR-01 fix the only remaining collapse is two PigParse rows
whose *names* already normalize to the same string — which the union itself intends to dedup, making the
behavior consistent. If you want an audit trail, log at `slog.Debug` when an `ON CONFLICT(norm_name)`
update changes the representative `item_id` (a sign two ids share a name). No schema change required.

## Info

### IN-01: `CatalogIconCoverage` comment overstates "one row per item" for held-vs-held duplicate names

**File:** `internal/backendsrv/store/itemids.go:181-183`
**Issue:**
The docstring asserts a plain `UNION ALL` "already yields one row per item — no precedence logic needed."
That holds across the held/catalog boundary, but NOT within `item_master` itself: two distinct held EQ ids
that share a normalized name (the documented, tested `TestDistinctEnrichmentRefs_HeldVsHeldSameName` case)
produce two `item_master` rows and are counted twice in `Total`/`IconLess`. The count is therefore "one
row per physical enrichment row," not strictly "one per distinct item." This is a pre-existing property of
the held store (the held arm intentionally groups by `item_id`, not by name) and is harmless for a coverage
*diagnostic*, but the comment claims a stronger invariant than the SQL provides.
**Fix:** Soften the comment to "one row per held EQ id ∪ one row per catalog norm_name (held names never in
catalog_enrichment); same-named held rows under distinct EQ ids count once each, matching the held crawl."
No code change needed.

### IN-02: Double error surface on a catalog upsert failure (slog.Error + wrapped return)

**File:** `internal/backendsrv/store/catalogenrich.go:115-116`
**Issue:**
`UpsertCatalogEnrichmentTx` both `slog.Error(...)`s and returns a wrapped error; the job caller
(wiki.go:206-210) then `slog.Warn`s the same failure again. One DB failure thus emits two log lines at two
levels. This mirrors no clear precedent — `UpsertItemMasterTx` (enrich.go) does not self-log, leaving it to
the caller. The redundant log is harmless but inconsistent with the held path it claims to mirror exactly.
**Fix:** Drop the `slog.Error` inside `UpsertCatalogEnrichmentTx` and let the caller own the log line (as
the held path does), or downgrade to `slog.Debug`. Cosmetic.

### IN-03: `int(ref.ItemID)` narrowing of an `int64` id (cosmetic; matches held path)

**File:** `internal/backendsrv/enrich/jobs/wiki.go:314`
**Issue:**
`ItemID: int(ref.ItemID)` narrows the `int64` ref id to the `CatalogEnrichment.ItemID int` field. On the
backend's 64-bit target `int` is 64-bit so there is no truncation in practice, and the held path does the
identical `int(ref.ItemID)` (wiki.go:361), so this is consistent, not a regression. Flagged only because
the representative `item_id` is non-load-bearing (it is not a key), making `int64` end-to-end the cleaner
type if ever touched.
**Fix:** Optional — make `CatalogEnrichment.ItemID` an `int64` to match `EnrichmentRef.ItemID` and drop the
conversion. No correctness impact on 64-bit.

### IN-04: Catalog-only quest links are silently discarded (`_ = questLinks`) — confirm against Phase 39 scope

**File:** `internal/backendsrv/enrich/jobs/wiki.go:338`
**Issue:**
The catalog branch parses the page (which harvests `questLinks`) then discards them via `_ = questLinks`,
because `quest_items` is the EQ-namespace held-only table (T-38-08, RESEARCH A2). This is the
research-ratified decision and is correct for ENRICH-14/15 (icon + flags coverage). Recording it here only
so the reviewer-of-record confirms no current/Phase-39 requirement needs unheld quest links — 39-CONTEXT
facets are Clicky/Haste, not quests, so this is in scope to skip. No change needed; revisit only if a
future requirement wants a name-keyed `catalog_quest_items`.
**Fix:** None required — documented, intentional scope boundary.

---

_Reviewed: 2026-06-25_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
