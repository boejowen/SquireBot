---
phase: 38-catalog-wide-enrichment-icon-coverage
verified: 2026-06-25T19:05:00Z
status: human_needed
score: 4/4 success criteria verified (code); 2/2 requirements satisfied (code)
overrides_applied: 0
re_verification:
  previous_status: passed
  previous_score: 5/5
  note: >
    Prior 38-VERIFICATION was for the id-keyed Option-A implementation (5/5, never deployed,
    held the deploy after the pre-deploy prod probe found 43 dropped catalog items). That
    VERIFICATION.md was retired when the phase re-opened (722edbc) and the name-keyed re-plan
    regenerated all artifacts (8217504). This is a fresh verification of the NAME-KEYED code.
  gaps_closed:
    - "The 43 catalog items the Option-A id-collision guard silently dropped are now enriched (name-keyed, no drop) — proven by the FLIPPED Case C in itemids_test.go:219-225"
  gaps_remaining: []
  regressions: []
human_verification:
  - test: "Deploy 37+38 (migration 00017 goose-on-boot) and let one weekly wiki job run the full ~72-min seed crawl over the live ~4,343-row pigparse_price catalog."
    expected: "catalog_enrichment populates for the full catalog (held items keep their item_master rows); previously colored-tile-only items render real wiki icons; the D-03 'wiki_weekly: items coverage' slog line reports total/icon_covered/icon_less spanning both stores with a bounded icon-less residue sample."
    why_human: "Prod-runtime behavior (the actual paced seed crawl populating catalog_enrichment live + icons rendering in the web client) is only observable post-deploy; the code is verified but nothing in this phase has been deployed (prod schema v15, 00017 not yet applied)."
  - test: "After the seed crawl, browser-smoke an item that previously showed only the colored-tile fallback (e.g. one of the 43 recovered collision items: Cured Silk Gi, Ancient Tarnished Breastplate, Etched Velium Brawl Stick)."
    expected: "The item renders its real wiki icon (icon_id > 0 in catalog_enrichment) wherever the wiki provides one; genuinely icon-less items keep the colored tile and appear in the slog residue sample."
    why_human: "Visual icon rendering + the specific recovered-item coverage is a post-deploy, in-browser observation; web/ vitest is node-only and blind to the DOM (per the web-tests-node-only memory)."
---

# Phase 38: Catalog-wide Enrichment + Icon Coverage Verification Report

**Phase Goal:** Enrichment covers the full PigParse Blue catalog (not only held items), fixing missing icons + providing the full-catalog data Phase 39's scope toggle reads; a maintainer can see which items remain genuinely icon-less. **(NAME-KEYED re-implementation — D-04 reversed from id-keyed Option A.)**
**Verified:** 2026-06-25T19:05:00Z
**Status:** human_needed — all code criteria VERIFIED; the live seed-crawl runtime behavior is the expected post-deploy human gate (not a code FAIL).
**Re-verification:** Yes — fresh verification of the name-keyed re-plan (prior 5/5 was the retired id-keyed Option A).

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
| --- | ----- | ------ | -------- |
| 1 | Weekly enrichment fetches+stores wiki data across the FULL PigParse Blue catalog — a catalog item nobody holds still gets enriched, with NO collision drop (the reversal bar: all ~4,343, incl. the 43). | ✓ VERIFIED | `store/itemids.go:119-162` `DistinctEnrichmentRefs` UNIONs held∪catalog; the Option-A guard `AND item_id NOT IN (SELECT item_id FROM item_master)` is GONE from the `catalog` CTE (only the held-name dedup `NOT IN (SELECT norm FROM held_names)` remains, :136). `wiki.go:165` the items pass iterates `DistinctEnrichmentRefs`; `wiki.go:295-342` the `!ref.Held` branch writes `catalog_enrichment` by norm_name. **FLIPPED Case C** (`itemids_test.go:219-225`) asserts a catalog id colliding with a held EQ id is now INCLUDED `Held=false` (was EXCLUDED under Option A) — recovers the 43. |
| 2 | Items that previously showed only the colored-tile fallback now render their real wiki icon. | ✓ VERIFIED (code) | The catalog branch carries `IconID: item.IconID` into `catalog_enrichment` (`wiki.go:321`) and the 4-field freshness compare includes the icon (`wiki.go:305` `existingIcon == int64(item.IconID)`), so an unheld item's icon backfills exactly like held items. Job test `TestRunWiki_EnrichesUnheldCatalogItem` (`wiki_test.go:360-366`) asserts Cloak of Flames gets `icon_id > 0` in `catalog_enrichment`. Visual render is the post-deploy human gate. |
| 3 | A maintainer can see a coverage diagnostic listing which items are still icon-less (distinguishing genuinely-no-icon from a gap). | ✓ VERIFIED | `store/itemids.go:187-229` `CatalogIconCoverage` reads BOTH stores via `UNION ALL` (item_master ∪ catalog_enrichment) for the Total/IconCovered count AND the name-ordered, `LIMIT ?`-bounded residue sample. `wiki.go:233` reads it; `wiki.go:252-262` `logItemsCoverage` emits the D-03 slog line (`total`/`icon_covered`/`icon_less`/`residue_sample`, public names only). `TestCatalogIconCoverage` (`itemids_test.go:298-336`) proves Total spans both stores (5+3=8). |
| 4 | The crawl is politefetch-paced; `go test ./...` green; watcher untouched, no `v*` tag. | ✓ VERIFIED | `wiki.go:173` `wikiSleepFn(ctx, interRequestSleep)` 1s courtesy sleep BEFORE every fetch (unchanged); `wiki.go:185` ETag `fetchUnchanged` 304 short-circuit; `wiki.go:188-198` log-and-skip-one-bad-page resilience — all intact for the widened set. `go test ./...` green (ran full module: zero non-ok lines). `git diff 8217504 HEAD` over watcher dirs is EMPTY; no `v*` tag points at any P38 commit (latest tag v2.1.2 predates the milestone). |

**Score:** 4/4 success criteria verified in code.

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `migrations/00017_catalog_enrichment.sql` | Additive CREATE TABLE catalog_enrichment (norm_name PK) + goose Down | ✓ VERIFIED | 20-column name-keyed table mirroring item_master's enrichment set; `norm_name TEXT PRIMARY KEY`; no ALTER of any existing table; `-- +goose Down DROP TABLE`. `TestMigrate_00017_AddsCatalogEnrichment` PASS (table + 20 cols + ON CONFLICT(norm_name) one-row + idempotency). |
| `store/catalogenrich.go` | CatalogEnrichment + UpsertCatalogEnrichmentTx + GetCatalogEnrichmentFreshnessTx | ✓ VERIFIED | All three exported; `ON CONFLICT(norm_name) DO UPDATE` (:78); every untrusted value `?`-bound (:109-114); `b2i` reused (no redefinition); NO quest_items write; 4-field freshness getter (:132-144) returns zero-values for absent norm_name. Round-trip/conflict/self-heal tests all PASS. |
| `store/itemids.go` | EnrichmentRef{ItemID,Name,Held} + guard-dropped union + both-stores CatalogIconCoverage | ✓ VERIFIED | `type EnrichmentRef struct {ItemID; Name; Held}` (:38-42); collision guard removed from SQL (only a docstring mention remains, :108); `CatalogIconCoverage` UNION ALLs both tables (:193-195, :207-209). `ItemMasterIconCoverage` renamed away. |
| `store/catalogenrich_test.go` | Upsert round-trip + freshness self-heal by norm_name | ✓ VERIFIED | 3 tests PASS (RoundTrip, ConflictUpdatesInPlace, SelfHeal). |
| `enrich/jobs/wiki.go` | Branched upsertItemAndQuests (ref.Held) + both-stores coverage log | ✓ VERIFIED | Branches strictly on `ref.Held` (:295); catalog arm → `catalog_enrichment` by norm (:300-342), no quest write (`_ = questLinks` :338); held arm → item_master + quest_items verbatim (:346-403); coverage = `s.CatalogIconCoverage` (:233). |
| `enrich/jobs/wiki_test.go` | TestRunWiki_EnrichesUnheldCatalogItem asserting catalog_enrichment by norm_name | ✓ VERIFIED | Asserts catalog_enrichment row by norm_name with icon>0 (:360-366) + Pitfall-2 NO item_master leak (:373-379); held Cloth Cap + job-ok + stale-icon/flags regressions stay green. |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | -- | --- | ------ | ------- |
| `catalogenrich.go` | catalog_enrichment table | `ON CONFLICT(norm_name)` upsert + `WHERE norm_name = ?` read | ✓ WIRED | catalogenrich.go:78 (upsert), :136 (freshness read). |
| `itemids.go` | catalog_enrichment + item_master | `UNION ALL` coverage read | ✓ WIRED | itemids.go:193-196 (count), :206-210 (residue). 4 `FROM`/UNION references across both queries. |
| `wiki.go` (`!ref.Held` branch) | `store.UpsertCatalogEnrichmentTx` / `GetCatalogEnrichmentFreshnessTx` | the catalog write branch | ✓ WIRED | wiki.go:301 (freshness), :311 (upsert) — both inside the `if !ref.Held` block. |
| `wiki.go` end-of-pass | `store.CatalogIconCoverage` | the D-03 coverage read | ✓ WIRED | wiki.go:233 → logItemsCoverage:237. `ItemMasterIconCoverage` fully removed (0 references). |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| -------- | ------------- | ------ | ------------------ | ------ |
| `catalog_enrichment` rows | `item.IconID`/`item.Flags`/`item.Statsblock` | `enrich.ParseItempage(page.wikitext)` over the widened `DistinctEnrichmentRefs` set → `UpsertCatalogEnrichmentTx` | ✓ FLOWING (code) — wiki text → parse → name-keyed upsert; tests prove the row + icon land. Live population is the post-deploy seed crawl (human gate). | ✓ VERIFIED (code) |
| `CatalogIconCoverage` totals | `cov.Total/IconCovered/IconLess` | `SELECT count … FROM (item_master UNION ALL catalog_enrichment)` | ✓ FLOWING — reads real stored icon_id from both tables; not hardcoded. | ✓ VERIFIED |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Migration 00017 creates table + ON CONFLICT(norm_name) one-row + idempotent | `go test ./internal/backendsrv/migrations/... -run TestMigrate_00017` | PASS (0.44s) | ✓ PASS |
| Name-keyed store round-trip + self-heal by norm_name | `go test ./internal/backendsrv/store/... -run "CatalogEnrich"` | 3 PASS | ✓ PASS |
| Collision guard dropped — colliding catalog name now INCLUDED Held=false | `go test … -run "DistinctEnrichmentRefs"` (flipped Case C) | PASS | ✓ PASS |
| Both-stores icon coverage | `go test … -run "IconCoverage"` (Total=8 spans both) | PASS | ✓ PASS |
| Unheld catalog item → catalog_enrichment w/ icon, NO item_master leak | `go test ./internal/backendsrv/enrich/jobs/... -run TestRunWiki_EnrichesUnheldCatalogItem` | PASS (0.71s) | ✓ PASS |
| Held-path regressions intact | `go test … -run "TestRunWiki_BackfillsStale"` | 2 PASS | ✓ PASS |
| Build + vet | `go build ./...` (exit 0); `go vet ./internal/backendsrv/...` (clean) | clean | ✓ PASS |
| Full module | `go test ./... -count=1` | all green (0 non-ok) | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ----------- | ----------- | ------ | -------- |
| ENRICH-14 | 38-01, 38-02 | Item enrichment covers the full PigParse Blue catalog, not only held items (politefetch-paced, no-drop). | ✓ SATISFIED | `DistinctEnrichmentRefs` widens held→held∪catalog by name with the id-collision guard DROPPED (itemids.go:119-162); the write branch persists catalog-only items name-keyed (wiki.go:295-342). Pacing unchanged (wiki.go:173 1s sleep). The reversal "no-drop" bar is met: the FLIPPED Case C proves the formerly-dropped collision items are recovered (itemids_test.go:219-225). |
| ENRICH-15 | 38-01, 38-02 | Icon coverage backfilled for every item whose wiki page provides one + a maintainer can see which items are still icon-less. | ✓ SATISFIED | Icon backfill rides the 4-field freshness short-circuit in BOTH stores (wiki.go:305, :321; catalogenrich.go:132); maintainer diagnostic = both-stores `CatalogIconCoverage` + D-03 slog residue sample (itemids.go:187-229, wiki.go:233-262). Icon-render visual is the post-deploy human gate. |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| store/itemids.go | 108 | Literal string `item_id NOT IN (SELECT item_id FROM item_master)` present | ℹ Info | It is in the DOCSTRING explaining WHY the guard was dropped — the actual `catalog` CTE (:132-138) has NO such guard. Confirmed by reading the SQL. The Plan-01 SUMMARY disclosed this as a deliberate documentation choice (grep-count note). NOT a defect. |
| store/itemids.go | 41 | `grep "Held bool"` returns 0 | ℹ Info | gofmt-aligned as `Held   bool` (multiple spaces); the field exists (:41) and is exercised by tests. Plan-01 SUMMARY disclosed this text-count artifact. NOT a defect. |

No blocker or warning anti-patterns. No TODO/FIXME/placeholder, no `return nil`/empty-impl stubs, no hardcoded-empty data flowing to output. The two info items are literal-grep artifacts the Plan-01 SUMMARY pre-disclosed and that I independently confirmed are non-defects by reading the SQL/struct.

### Human Verification Required

1. **Live seed crawl populates catalog_enrichment** — Deploy 37+38 (00017 goose-on-boot) and let one weekly wiki job run the full ~72-min paced crawl over the live ~4,343-row catalog. Expected: catalog_enrichment populates (held items keep item_master rows); the D-03 slog line reports both-stores coverage with a bounded icon-less residue. *Why human:* prod-runtime, only observable post-deploy (nothing in this phase is deployed; prod schema v15, 00017 not applied).
2. **Recovered/previously-icon-less items render real icons** — After the crawl, browser-smoke an item that previously showed only the colored tile (e.g. one of the 43 recovered collision items: Cured Silk Gi, Ancient Tarnished Breastplate, Etched Velium Brawl Stick). Expected: real wiki icon where the wiki provides one; genuinely icon-less items keep the tile + appear in the residue sample. *Why human:* visual DOM rendering — web vitest is node-only/DOM-blind.

### Gaps Summary

No code gaps. Every ROADMAP success criterion and both requirements (ENRICH-14, ENRICH-15) are satisfied in the actually-committed name-keyed code, independently verified by reading the SQL/Go (not by trusting the SUMMARYs) and by running the test suite:

- **The reversal bar is met.** The name-keyed model covers ALL ~4,343 catalog items with NO collision drop. The Option-A `item_id NOT IN (SELECT item_id FROM item_master)` guard is genuinely gone from the `DistinctEnrichmentRefs` SQL (the only residual reference is a docstring explaining its removal). The single most load-bearing assertion — the FLIPPED Case C (`itemids_test.go:219-225`) — proves a catalog id colliding with a held EQ id is now INCLUDED with `Held=false`, recovering the 43 items (Cured Silk Gi, etc.) the id-keyed Option A silently dropped.
- **Held-reader blast radius is provably zero.** `git diff 8217504 HEAD -- internal/backendsrv/store/enrich.go` is EMPTY: the held write path (`UpsertItemMasterTx`/`GetItemMasterFreshnessTx`) and `item_master`'s schema are byte-for-byte unchanged. Catalog-only enrichment lives entirely in the new name-keyed `catalog_enrichment` table; no held reader (Phases 31/32/37) touches it. The job test's Pitfall-2 no-leak assertion (`wiki_test.go:373-379`) confirms a catalog ref never writes item_master.
- **Politefetch pacing, ETag 304 short-circuit, and log-and-skip resilience are intact** for the widened set (wiki.go:173/185/188-198, unchanged).
- **Migration 00017 is additive** (new table only, no ALTER), goose Up/Down round-trips, `WatcherMaxSchemaVersion` not touched (the gate doesn't exist in the off-Google backend).
- **`go test ./...` is green; the watcher is untouched; no `v*` tag** points at any Phase 38 commit (latest tag v2.1.2 predates the milestone).

Status is **human_needed** (not passed) solely because of the unavoidable post-deploy items: the actual ~72-min seed crawl populating `catalog_enrichment` live and the in-browser icon render are only observable after deploy — the expected runtime gate the task brief called out, not a code failure.

---

_Verified: 2026-06-25T19:05:00Z_
_Verifier: Claude (gsd-verifier)_
