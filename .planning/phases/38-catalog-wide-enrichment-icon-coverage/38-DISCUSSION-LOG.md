# Phase 38: Catalog-wide enrichment + icon coverage - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-06-25
**Phase:** 38-catalog-wide-enrichment-icon-coverage
**Areas discussed:** Crawl cadence & control, Catalog scope, Coverage diagnostic, Storage for unheld items

---

## Crawl cadence & control

| Option | Description | Selected |
|--------|-------------|----------|
| Seed once, weekly re-validates all | First run does the full paced crawl (~70 min); every weekly pass re-validates ALL items via If-None-Match (cheap 304s on unchanged). Simplest, always self-healing, courteous at 1 req/s; long but capless. | ✓ |
| Seed once, weekly re-checks held only | After seed, weekly re-validates held only; unheld refresh on a slow rotation. Short weekly runs, unheld edits land slowly, more moving parts. | |
| Throttle initial crawl across days | Spread the first crawl over several days (N/day). Gentlest, but slower to first-complete + day-boundary bookkeeping. | |

**User's choice:** Seed once, weekly re-validates all.
**Notes:** Justified by `wiki.go`'s "a backend job has NO execution cap" design — a long Sunday background run is harmless, so simplicity + self-healing wins. Crawl runs automatically as part of the existing weekly job (no new manual trigger this phase); a maintainer-triggered re-crawl button deferred.

---

## Catalog scope

| Option | Description | Selected |
|--------|-------------|----------|
| PigParse auction catalog only | Exactly `pigparse_price` (~4,341 auctioned items). Matches ENRICH-14 wording, carries prices, natural "what can I get" set. | ✓ |
| Catalog + never-auctioned wiki gear | Also union no-drop/raid items from `wiki_gear_tier`. Broader "what exists", but no price + crawl bloat with un-buyable items. | |

**User's choice:** PigParse auction catalog only.
**Notes:** No-drop raid loot that never auctions already surfaces via the Wishlist gear-tier suggestions (Phase 34/42), so excluding it from the catalog crawl loses nothing.

---

## Coverage diagnostic

| Option | Description | Selected |
|--------|-------------|----------|
| Per-run log summary line | Weekly job logs total / enriched / icon-covered / icon-less counts (+ residue names). Greppable on the VPS, zero new UI, fits the ops pattern. | ✓ |
| Officer Admin settings panel | A "Item coverage" section in the Admin panel. In-browser to officers, but needs a new read-API + web surface. | |
| Queryable / downloadable report | An admin endpoint returning the icon-less list (JSON/CSV). No UI, but a new gated endpoint. | |

**User's choice:** Per-run structured `slog` summary line.
**Notes:** Maintainer already greps slog JSON on the VPS; lowest-cost surface that satisfies ENRICH-15's "a maintainer can see." Admin panel / report deferred unless the log proves insufficient.

---

## Storage for unheld items

| Option | Description | Selected |
|--------|-------------|----------|
| Delegate to research + planning | Capture as a flagged research question (name-keyed store vs. synthetic ids vs. separate table; + name-dedup rule for SEARCH-06). Researcher/planner resolve against the codebase. | ✓ |
| I want to weigh in on the approach | Talk through the storage model now before it's locked. | |

**User's choice:** Delegate to research + planning.
**Notes:** This is the load-bearing technical risk — `item_master` is keyed by EQ-inventory `item_id`, the PigParse catalog uses a different id namespace (join by normalized name only). Captured as CONTEXT.md D-04 with the three sub-questions the researcher must resolve.

---

## Claude's Discretion

- **D-04** (storage/identity model for unheld items + SEARCH-06 name-dedup) — explicitly delegated by the user to research + planning; flagged with candidate options and the namespace-collision risk in CONTEXT.md.
- Migration mechanics (00017 if needed, possibly none), `WatcherMaxSchemaVersion` confirmation (backend-only — almost certainly untouched), icon-backfill reuse of the 00012 freshness pattern.

## Deferred Ideas

- Maintainer-triggered re-crawl button (manual kickoff from Admin) — automatic weekly job suffices.
- Officer Admin "Item coverage" panel / queryable coverage report — richer residue surfaces than the log line; revisit only if needed.
- Search facets + the SEARCH-06 UI — Phase 39, not here.

---

## D-04 reversal — id-keyed Option A → name-keyed (2026-06-25, post-execution)

> Audit entry for the storage-model reversal. The decision was DELEGATED above; research
> resolved it to id-keyed Option A; the pre-deploy prod probe invalidated that choice and
> the user re-decided. Binding constraint is recorded in CONTEXT.md (⚠ D-04 REVERSED block).

| Option | Description | Originally | Now |
|--------|-------------|-----------|-----|
| A — id-keyed `item_master` (PigParse id) + collision guard | Catalog-only rows go into `item_master` keyed by PigParse `item_id`; an `item_id NOT IN item_master` guard drops any PigParse id that numerically equals a held EQ id. Zero migration, zero held-reader blast radius — but drops the colliding catalog names. | ✓ (research pick) | ✗ REJECTED |
| B — name-keyed `catalog_enrichment` table | Separate table keyed by `lower(trim(name))` for unheld items; `item_master` stays held-only. +1 migration (00017) + a parallel store/freshness layer; Phase 39 COALESCEs held∪unheld by name. Covers ALL catalog items, no drop. | (rejected for blast radius) | ✓ SELECTED |

**Why the reversal.** Research Assumption A2 held that a PigParse id numerically equal to a
held EQ id for a *different* item was "rare … ≈0", so Option A's guard would drop ~nothing.
The pre-deploy prod probe (live DB: `item_master`=953, `pigparse_price`=4,343) found **60
raw collisions / 43 genuinely-unheld catalog items dropped** (e.g. *Cured Silk Gi*, *Ancient
Tarnished Breastplate*, *Etched Velium Brawl Stick*) — ~1% of the catalog, no correctness
risk (the guard correctly protects held rows), but a permanent coverage hole. The plan's
STOP-at-~20 gate fired; surfaced to the user, who chose to **hold the deploy and re-plan
name-keyed** (cover all 4,343, no drop) over shipping the gap. Significant rework accepted:
re-architects the storage D-04 deliberately avoided, +migration 00017, held-reader blast
radius re-verified (stays zero — held items remain in `item_master`).

**Bonus:** name-keying makes Phase 39's catalog↔enrichment facet join name-keyed end-to-end,
dissolving the namespace-bridge hazard flagged in 39-CONTEXT.md (the catalog scope reads the
same `lower(trim(name))` key throughout).
