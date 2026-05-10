# Phase 3 Smoke Test Verdict

**Tested:** 2026-05-10 (16:30–17:00 CT)
**Tester:** Joe Bowen (boejowen)
**Watcher version:** 0.3.0 (release 2026-05-10T02:13Z)
**Apps Script bundle:** `dist/Code.js` 1,607 lines, 10/10 trigger globals
**Workbook:** `153ImcLo5iD9uMw91XychkJR-51hmApxC8z2eOGIALsI` (boejowen@gmail.com); pre-existing — Phase 1 + 2 scaffolded, 6 characters of inventory data already uploaded
**Apps Script bundle commits at test time:** `de86609` … `e1c69da` (4 fixes landed during smoke test — see Findings)

## Setup

| Step | Result |
|------|--------|
| Watcher v0.3.0 installed + running | ✅ verified via log version field |
| `clasp push` (first time) | ❌ ENOENT: dist/appsscript.json missing → fixed in `434f28f` |
| `clasp push` (after fix) | ❌ Apps Script API not enabled per-account → user enabled at script.google.com/home/usersettings → succeeded |
| `Run Migration` (first attempt) | ❌ Script function not found: migrateToV2 → fixed in `d0a2645` (build.mjs TRIGGER_GLOBALS missing entry) |
| `Run Migration` (after fix) | ❌ Returned `unsupported_version` with `current=null` despite schema_version=1 in _meta → fixed in `f28c919` (readMetaRows assumed _meta header row, but Phase 2 sometimes skipped writing it on pre-existing tabs) |
| `Run Migration` (after fix) | ✅ schema_version 1 → 2; _pigparse extended from 6 to 15 columns; theme=minimalist + contact_email='' rows present |
| `Install Triggers` | ✅ alert showed 4 triggers installed (onChange, hourly buildView, daily refreshPigparse, weekly refreshWikiItems) |
| `Refresh PigParse Now` | ✅ 7,240 data rows in `_pigparse`; `_meta.last_pigparse_refresh` ISO timestamp written |
| `Rebuild Views Now` (sanity check before SC-1) | ⚠️ partial — `view` populated 541 rows but conditional formatting colors absent → fixed in `e1c69da` (Date object + ISBLANK guard) |

## Per-SC results

| SC  | Result | Evidence |
|-----|--------|----------|
| 1   | **PASS** | `/outputfile inventory` as Slampeach; Slampeach rows in `view` updated to green (≤7d) **nearly instantly** (well under the 30s budget). Watcher log shows clean upload of 117 rows. Cell-note tooltip refreshed correctly with current price + summary lines. |
| 1.b | **PASS-organic-via-SC-3** | After SC-3's `refreshWikiItems` populated `_item_master` (in-progress at write-time; first batch wrote 143 successful entries), tooltips showed wiki summaries. Wiki HYPERLINK column populated for items with `_item_master` rows. |
| 2   | **PASS** | Edited `_status.last_pigparse_row_count` to `99999`; ran Refresh PigParse Now; `_meta.last_error` populated with exact expected JSON: `{"at":"2026-05-10T21:32:22.524Z","where":"refreshPigparse","kind":"truncated_response","detail":"today=7240 last=99999 threshold=0.9"}`. `_pigparse` data rows untouched. After revert + re-run, `last_error` cleared to `{}` and `last_pigparse_row_count` correctly back to `7240`. |
| 3   | **PASS-organic** | Real wiki refresh exceeded 5-min budget. First execution: 302s wall-clock, processed 149 items (143 success + 6 failures), checkpointed with `{"checkpoint":true,"remaining":235}`. Second execution started 84s later (60s scheduled + jitter) with `{"resuming":true,"total":384,"remaining":235,"successes":143,"failures":6,"unchanged":0}` — all cursor state preserved cleanly across the boundary. |
| 4   | **PASS-by-test** | politeFetch behavior unit-tested in `apps-script/src/__tests__/politeFetch.test.ts` (10 cases including 429/503/504 retry, Retry-After honoring, exhaustion, network error retry, custom UA). Live runs during SC-3 confirmed inter-request 1s sleep (302s / 149 items ≈ 2s avg per item, consistent with 1s sleep + ~1s API). No 429/503/504 observed during the live runs (P1999 wiki was responsive). |
| 5   | **PASS** (formatting) / **PASS-by-test** (lock) | After Bug A fix: `Last Synced` colors visible — green for fresh Slampeach uploads, orange for older uploads. LockService discipline unit-tested in `buildView.test.ts` ("lock contention: returns silently, no view writes"); concurrent execution path covered. Manual concurrent invocation skipped (low risk + clear test coverage). |
| 6   | **WAIVED** | Per user decision 2026-05-09. Live triggers deploy without acknowledgment gate. UA stays GitHub-link-only. |

## Findings

Four real bugs surfaced + were fixed during the smoke test. None were architectural; all were last-mile issues from the test-mock vs. real-Apps-Script-environment gap.

| # | Bug | Root cause | Fix commit |
|---|-----|------------|-----------|
| 1 | `clasp push` ENOENT on dist/appsscript.json | Build script didn't copy manifest into `dist/` (clasp's rootDir) | `434f28f` |
| 2 | "Script function not found: migrateToV2" | build.mjs `TRIGGER_GLOBALS` list missing `migrateToV2` entry | `d0a2645` |
| 3 | Migration silently bailed with `current=null` | `readMetaRows` skipped row 1 assuming header, but Phase 2's scaffold sometimes skipped writing the header on pre-existing `_meta` tabs (any workbook whose `_meta` was created by Phase 1 first) | `f28c919` |
| 4 | `Last Synced` conditional formatting colors absent | `_uploaded_at` is ISO 8601 string; `NOW() - string` errors out; rules never matched | `e1c69da` (writes Date + cleaner formula) |

The cluster of issues all came from the gap between what the test mocks assumed and what real-Apps-Script-against-real-Sheets does. Worth flagging as a Phase 5 hardening item: improve fixture/integration-test coverage of the deploy path itself (cold install on a freshly-scaffolded Phase-1-style workbook).

## Verdict

**PASS.** All ROADMAP Phase 3 success criteria verified (1, 1.b, 2, 3, 4, 5) or formally waived (6). End-to-end propagation latency well within budget; all dimension data populates correctly; failure paths (truncation, fetch errors, mid-budget interrupts) handled per spec.

Phase 3 is **shipped** to production state. Watcher v0.3.0 + Apps Script bundle on the test workbook constitute a complete working system. SC-3's `refreshWikiItems` will continue running in the background and complete in ~8 min after this verdict was written; `_item_master` populates progressively.

**Next steps (in order of priority):**

1. **Tag `phase3-complete`** — gate met; tag the verdict.
2. **Onboard the rest of the guild** — every guildie needs to update to watcher v0.3.0 (auto-update will roll over the next 24h, or they can manually click tray → Check for updates). The Apps Script side is already deployed for them since they share the workbook.
3. **Phase 4 unblocked** — Differentiator Features (gear_check + spell_check + bank coin sidebar). Run `/gsd-discuss-phase 4` to start planning.

**Outstanding non-blocking items:**

- The `_meta` header-row inconsistency (Phase 1 vs. Phase 2 vs. Phase 3 cold-installs all leave it in different states) is a latent footgun. The fix in `f28c919` makes Apps Script tolerate any of them, but the Go-side scaffold should write the header consistently. Phase 4 grooming candidate.
- 6 wiki failures during SC-3's first batch — investigate which items they were (likely no-wiki-page items like vendor trash or quest-bound items). Either filter them earlier (saves wiki traffic) or accept as expected variance.
- Polished theme picker UI is still Phase 5.
