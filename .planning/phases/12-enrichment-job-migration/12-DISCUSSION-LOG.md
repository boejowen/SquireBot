# Phase 12 — Discussion Log

**Mode:** `discuss --auto` (2026-05-29). No interactive Q&A — Claude picked recommended defaults for a faithful port, per the user's "proceed into Phase 12" + `yolo` mode.

**Basis for defaults:** the phase is a low-ambiguity migration. The ROADMAP §Phase 12 Note and the findings doc `04-data-enrichment-migration.md` §2 already pre-answer the substantive choices (pure parsers port near-verbatim; only the Apps Script I/O wrappers are replaced; the 6-min-cap resumable cursor + the two Sheets watchdogs are deleted; all politeness controls are kept).

**Gray areas auto-resolved → see 12-CONTEXT.md `<decisions>` D-1…D-8:**
- Port the 4 pure parsers to Go verbatim, reusing fixtures (D-1)
- Two scheduler jobs, existing cadence, persisted last-run (D-2)
- `politeFetch` ported verbatim with all controls; ETag state → DB (D-3)
- Graceful-degradation upsert + truncation guard as a log (D-4)
- Delete the resumable-cursor + Sheets watchdogs; DB single-writer replaces LockService (D-5)
- Populate 11-02's empty dimension tables; reconcile columns via a new `00003` migration (D-6)
- Parity vs the live Sheet is the acceptance proof (D-7)
- Strictly the two enrichment feeds; no eviction/backfill (D-8)

**Flagged for the planner/researcher** (12-CONTEXT.md `<open_for_planner>`): the dimension-table column reconciliation (11-02 empty tables vs parser outputs) is the top research item.

**No scope creep raised; no deferred ideas beyond those already in the findings doc.**
