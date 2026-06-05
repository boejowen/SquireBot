---
phase: 19-wantlist-crud
verified: 2026-06-05T00:00:00Z
status: passed
score: 16/16 must-haves verified
overrides_applied: 0
re_verification:
  # No previous VERIFICATION.md — this is the initial verification.
---

# Phase 19: Wantlist CRUD Verification Report

**Phase Goal:** A signed-in guildie can maintain a personal, Discord-identity-tied wantlist on squirebot.quest — add items from the catalog with a buy/quest reason, view and remove them, and see whether each is already in the guild bank.
**Verified:** 2026-06-05
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

This phase ships across three plans (01 store/migration, 02 HTTP handlers/routes, 03 SvelteKit UI). All three plans' must-have truths were checked against the actual codebase — not the SUMMARY narrative. Every backend behavior is exercised by a real-DB Go test; the DOM-free frontend logic is exercised by node vitest; the rendering surface was closed by the mandatory live browser-smoke (Task 3, 9/9 approved on prod). The three cross-AI review fixes (HIGH `holdersFor` reduce-by-char-and-sum, MEDIUM typed `ErrDuplicateWant` 409 path, MEDIUM "In guild" relabel) were verified to have landed in the actual source, not merely in the revised plans.

### Observable Truths

| #   | Truth | Status | Evidence |
| --- | ----- | ------ | -------- |
| 1 | 00006 migration applies idempotently, creates wantlist_item + alert_log, leaves 00001-00005 | ✓ VERIFIED | `00006_wantlist.sql` creates both tables; `TestMigrate_00006_AddsWantlist` + `TestRunMigrations_Idempotent` PASS |
| 2 | wantlist_item carries reason/priority CHECK constraints (DB-level enum guard) | ✓ VERIFIED | `CHECK (reason IN ('buy','quest'))` line 25, `CHECK (priority IN ('low','med','high'))` line 26; migration test asserts bad-enum insert fails |
| 3 | Want insert / owner-scoped active-only list / soft-remove; cross-owner remove is silent no-op | ✓ VERIFIED | `wantlist.go` UPDATE `active = 0 WHERE id=? AND discord_user_id=? AND active=1`; `TestRemoveOwnWantTx_CrossOwnerSilentNoOp` PASS |
| 4 | Same item twice with different reasons OK; exact duplicate returns TYPED `store.ErrDuplicateWant` via sqlite code 2067 | ✓ VERIFIED | `errors.As(err,&sqliteErr) && sqliteErr.Code()==2067` (line 79); `TestAddWantTx_DuplicateReturnsTypedSentinel` + custom-dup PASS; no `"UNIQUE constraint failed"` string-match |
| 5 | SearchCatalog over pigparse_price, case-insensitive, ESCAPE-literal %/_, LIMIT, NULL-name-safe | ✓ VERIFIED | `itemsearch.go` FROM pigparse_price, `sql.NullString` name scan, ESCAPE '\\'; 8 `TestSearchCatalog_*` PASS incl. `IdMatchOnNullNameRow` |
| 6 | Signed-in guildie can POST want, GET own list, POST-remove — owner from session never body | ✓ VERIFIED | `wantlist.go` handlers use `caller(r.Context())`; zero `req.Owner`/`owner_id`; handler tests PASS |
| 7 | Server validation rejects bad enums, >280-rune trimmed note, blank custom label; exact dup → 409 `{"error":"duplicate"}` | ✓ VERIFIED | `validWant` + `mapWantErr` errors.Is(ErrDuplicateWant)→409; `TestAddWant_Validation` + `TestAddWant_Duplicate_409_OtherReason_200` PASS |
| 8 | validWant trims note BEFORE 280-rune check (280 spaces → empty/NULL) | ✓ VERIFIED | `utf8.RuneCountInString(strings.TrimSpace(req.Note))` line 77; `note_280_spaces_stored_empty` subtest PASS |
| 9 | Cross-owner remove is silent no-op (removed:false), no leak | ✓ VERIFIED | `RemoveOwnWantHandler` delegates to owner-scoped store; `TestRemoveOwnWant_CrossOwnerNoOp_OwnRemoved` PASS |
| 10 | GET /api/v1/items/search?q= returns ≤25 matches; q<2 → []; q never logged | ✓ VERIFIED | `itemsearch.go` (readapi) `RuneCountInString(q)<2`→[], searchLimit=25, slog carries qlen only; 4 `TestItemSearch_*` PASS |
| 11 | All four routes behind RequireSession; anon → 401 | ✓ VERIFIED | `main.go` lines 319-324 all `webauth.RequireSession`; grep count = 4; cmd tests PASS |
| 12 | /wantlist page: add-item block above a DataGrid of the owner's wants | ✓ VERIFIED | `routes/wantlist/+page.svelte` + `WantlistPanel.svelte` (WantAddForm + 5th DataGrid + no-wants StateBlock); build emits route chunk |
| 13 | Debounced catalog search; pick pins item_id; no-match offers custom escape hatch flagged "won't trigger alerts" | ✓ VERIFIED | `WantAddForm.svelte` setTimeout debounce + searchCatalog + searchSeq out-of-order guard; "Custom — won't trigger alerts" chip line 201 |
| 14 | Catalog rows show In guild/Not in guild + per-holder `↳ Char: count` summed once per char; custom → "—" | ✓ VERIFIED (review HIGH fix) | `holdersFor` reduces via `Map<string,number>` + sum; `InGuildCell.svelte` renders the three states; holders.test `count:2` single-line assertion PASS |
| 15 | Add/remove re-fetch from server (authoritative); remove via ConfirmDialog; empty → no-wants StateBlock | ✓ VERIFIED | `WantlistPanel.svelte` `fetchOwnWants()` on add+remove (never optimistic), `ConfirmDialog`, `kind="no-wants"` branch |
| 16 | Item names/labels/notes render via plain {} (auto-escaped); only {@html} sink stays ItemTooltip | ✓ VERIFIED | grep `{@html` in WantAddForm/WantlistPanel/InGuildCell/columns = 0; browser-smoke step 8 (`<b>x</b>` literal) approved |

**Score:** 16/16 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `internal/backendsrv/migrations/00006_wantlist.sql` | wantlist_item (+CHECKs) + alert_log + partial indexes | ✓ VERIFIED | Both partial unique indexes scoped `WHERE … active = 1`; both CHECK constraints present |
| `internal/backendsrv/store/wantlist.go` | Add/List/soft-Remove + ErrDuplicateWant sentinel | ✓ VERIFIED | Typed sentinel via code 2067; IDOR-safe soft-remove; non-nil slice |
| `internal/backendsrv/store/itemsearch.go` | SearchCatalog over pigparse_price (NULL-safe) | ✓ VERIFIED | Bound LIKE+ESCAPE, COLLATE NOCASE only on ORDER BY, sql.NullString name |
| `internal/backendsrv/webadmin/wantlist.go` | AddWant/ListOwnWants/RemoveOwnWant + mapWantErr | ✓ VERIFIED | caller(ctx) owner, validWant, audited ids-only, 409 mapping |
| `internal/backendsrv/readapi/itemsearch.go` | ItemSearch + NewItemSearch | ✓ VERIFIED | GET-only, q<2→[], nil→[], q never logged |
| `cmd/squirebot-server/main.go` | 4 RequireSession routes | ✓ VERIFIED | Lines 319-324; grep gate = 4 |
| `web/src/lib/wantlist/holders.ts` | reduce-by-char-and-sum | ✓ VERIFIED | `Map<string,number>` sum; zero `.filter` map-not-reduce shape |
| `web/src/lib/api.ts` | typed wrappers + interfaces | ✓ VERIFIED | fetchOwnWants/searchCatalog/addWant/removeWant + WantlistRow/CatalogItem |
| `web/src/lib/components/WantlistPanel.svelte` | server-truth-reload grid | ✓ VERIFIED | Promise.all load, holdersByItem memo, ConfirmDialog remove |
| `web/src/lib/components/WantAddForm.svelte` | debounced catalog search + custom hatch | ✓ VERIFIED | searchCatalog debounce, custom flag chip, FormField reason/priority/note |
| `web/src/lib/components/cells/InGuildCell.svelte` | In guild/Not in guild/— display | ✓ VERIFIED | "In guild"/"Not in guild" words; summed `↳ {char}: {count}`; plain {} |
| `web/src/routes/wantlist/+page.svelte` | /wantlist route shell | ✓ VERIFIED | Hosts WantlistPanel; route chunk in build/_app |

### Key Link Verification

| From | To | Via | Status |
| ---- | -- | --- | ------ |
| store/wantlist.go RemoveOwnWantTx | wantlist_item | `active = 0 WHERE id=? AND discord_user_id=?` | ✓ WIRED |
| store/wantlist.go AddWantTx | store.ErrDuplicateWant | errors.As + Code()==2067 | ✓ WIRED |
| store/itemsearch.go SearchCatalog | pigparse_price | bound LIKE+ESCAPE | ✓ WIRED |
| main.go | webadmin handlers / readapi.NewItemSearch | RequireSession (×4) | ✓ WIRED |
| webadmin/wantlist.go mapWantErr | store.ErrDuplicateWant | errors.Is → 409 duplicate | ✓ WIRED |
| WantAddForm.svelte | /api/v1/items/search | debounced searchCatalog() | ✓ WIRED |
| WantlistPanel.svelte | /api/v1/wantlist | fetchOwnWants/removeWant + reload | ✓ WIRED |
| WantlistPanel/InGuildCell | fetchView() ViewRow[] | holdersFor group-by-char-sum | ✓ WIRED |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| -------- | ------------- | ------ | ------------------ | ------ |
| WantlistPanel grid | `wants` | `fetchOwnWants()` → GET /api/v1/wantlist → `ListOwnWants` (real SELECT) | Yes | ✓ FLOWING |
| InGuildCell holders | `viewRows` | `fetchView()` → InventoryJoin (real DB join) → holdersFor sum | Yes | ✓ FLOWING |
| WantAddForm results | catalog `items` | `searchCatalog()` → SearchCatalog (real pigparse_price query) | Yes | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| All phase-19 Go packages compile + test | `go test ./...migrations ./...store ./...webadmin ./...readapi ./...cmd` | all ok | ✓ PASS |
| Store CRUD + dedupe + NULL-name search | `go test ./...store -run "Want|Catalog|Search" -v` | 14 PASS | ✓ PASS |
| Migration + idempotency + CHECK | `go test ./...migrations -run "00006|Idempotent"` | 2 PASS | ✓ PASS |
| Handler validation + 409 + IDOR | `go test ./...webadmin -run Want -v` | all PASS incl 280-spaces, dup-409 | ✓ PASS |
| Search handler short-circuit/405 | `go test ./...readapi -run ItemSearch -v` | 4 PASS | ✓ PASS |
| DOM-free holders reduce-by-char-sum | `npx vitest run src/lib/wantlist/` | 13 PASS (incl count:2 single-line) | ✓ PASS |
| Frontend type safety | `npx svelte-check --threshold error` | 0 errors / 0 warnings | ✓ PASS |
| Production build emits /wantlist | `npm run build` + grep build/_app | built, route chunk present | ✓ PASS |
| Whole Go tree builds | `go build ./...` | exit 0 | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ----------- | ----------- | ------ | -------- |
| WANT-01 | 19-01, 19-02, 19-03 | Add item via catalog search, tag reason/priority/note, tied to web_user Discord identity | ✓ SATISFIED | discord_user_id-keyed store, catalog search endpoint, WantAddForm + addWant; live browser-smoke add approved |
| WANT-02 | 19-01, 19-02, 19-03 | View/manage wantlist: list, remove, "already in guild bank?" indicator | ✓ SATISFIED | ListOwnWants + RemoveOwnWant, InGuildCell in-guild indicator (relabeled to honest "In guild" superset); browser-smoke list/remove approved |

No orphaned requirements: REQUIREMENTS.md maps exactly WANT-01 and WANT-02 to Phase 19, both declared in all three plans' frontmatter.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| WantAddForm.svelte | 158 | `placeholder="Search items to add…"` | ℹ️ Info | Legitimate HTML input placeholder attribute, not a stub |

No blocker or warning anti-patterns. No TODO/FIXME/"not yet implemented"/return-null stubs in any wantlist file. The accepted JUDGMENT-CALL 8 (client-supplied item_name snapshot) is documented in code as an integrity smell, not a vuln (join keys on item_id, name auto-escaped).

### Human Verification Required

None outstanding. The mandatory browser-smoke (Plan 03 Task 3, blocking checkpoint) was completed on the LIVE production deploy (squirebot.quest/wantlist, schema v6) with all 9 steps APPROVED, including the review MUST-FIX-1 multi-row-same-char summed-holder line, the debounce single-call check, the duplicate-409 path, ConfirmDialog removal, XSS literal-text render, and owner-scoping.

### Gaps Summary

No gaps. All 16 observable truths across the three plans are verified against the actual codebase. The three cross-AI review findings were confirmed landed in source (not just in the revised plans):

1. **HIGH — holdersFor reduce-by-char-and-sum:** `web/src/lib/wantlist/holders.ts` uses `Map<string,number>` accumulation; the node test asserts two count-1 rows for one char collapse to a single `{char:'Borticus', count:2}` entry. The map-not-reduce double-count is gone (zero `.filter`).
2. **MEDIUM — typed ErrDuplicateWant 409:** `store/wantlist.go` returns the typed sentinel via `errors.As` + extended code 2067 (no `"UNIQUE constraint failed"` string-match); `webadmin/wantlist.go` `mapWantErr` maps it via `errors.Is` to exactly 409 `{"error":"duplicate"}`. Both store and handler tests prove the path.
3. **MEDIUM — "In guild" relabel:** `InGuildCell.svelte` renders "In guild"/"Not in guild" (zero "In bank" wording anywhere in the wantlist UI), honestly reflecting the all-inventory fetchView() superset.

Worth-fix defense-in-depth items also landed: NULL-safe name scan (sql.NullString), reason/priority CHECK constraints in the migration, note-trim-before-280-rune-count, and the corrected pigparse_name_idx DoS-claim comment.

Backend, frontend, requirements, and the live deploy all corroborate that the phase goal is achieved.

---

_Verified: 2026-06-05_
_Verifier: Claude (gsd-verifier)_
