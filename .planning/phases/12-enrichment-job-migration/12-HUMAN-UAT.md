---
status: partial
phase: 12-enrichment-job-migration
source: [12-VERIFICATION.md]
started: 2026-05-30
updated: 2026-05-30
---

## Current Test

[awaiting human testing — operational, post-deploy]

## Tests

### 1. Live daily+weekly cycle Sheet-parity spot-check (SC-4 / D-7)
expected: |
  After the redeployed backend binary runs one daily PigParse cycle and one weekly wiki
  cycle on its production timer (or via `squirebot-server run-job pigparse|wiki` on the
  VPS), spot-check the backend's dimension tables against the live Google Sheet for ~10
  well-known item_ids (Cloak of Flames, Fungi Tunic, etc.):
  - `pigparse_price.current_avg` / `t30` / `a30` EQUAL the Sheet's `_pigparse` values per item_id.
  - backend `pigparse_price` row count is LOWER than the Sheet's `_pigparse` (D-9 keeps only
    the WTS t=0 row per item — EXPECTED, not a failure).
  - `item_master.wikitext_sha1` matches `_item_master` for the same items (strongest parity signal).
  - `wiki_spells` (class,level,spell_name) and `wiki_gear_tier` (tier,class,slot,item_name,rank)
    row sets match `_wiki_spells` / `_wiki_gear_tier`; `quest_items` matches `_quest_items`.
result: [pending]
why_human: |
  Requires the jobs to fire on the production timer (24h / Sunday) on the deployed Hetzner
  VPS AND a side-by-side read of the still-live Google Sheet — an operational comparison
  against an external system, not verifiable from the codebase. The verifier already
  exercised BOTH jobs end-to-end against the LIVE PigParse + P1999 wiki APIs via `run-job`
  (pigparse = 4,338 WTS rows; wiki = 14 classes + 1,183 gear rows; all 4 tables populated),
  so the code path is proven — only the backend-vs-Sheet equality assertion remains.

## Summary

total: 1
passed: 0
issues: 0
pending: 1
skipped: 0
blocked: 0

## Gaps

(none — all 8 code-level must-haves verified; this is a deferred operational confirmation,
not a code gap. Prerequisite: deploy the rebuilt `squirebot-server` binary to the VPS so
`goose.Up` applies `00003` and the scheduler registers the two jobs. See STATE.md deploy note.)
