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
