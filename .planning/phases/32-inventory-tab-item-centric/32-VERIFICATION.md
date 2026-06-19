---
phase: 32-inventory-tab-item-centric
verified: 2026-06-18T19:40:00Z
status: passed
score: 6/6 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: none
  note: initial verification
---

# Phase 32: Inventory Tab (Item-Centric) Verification Report

**Phase Goal:** A guildie can answer "which characters have item X?" — a guild-wide item list with quantities + wiki/PigParse links, where selecting an item reveals exactly who holds it, in which slot, how many, and how fresh the data is (master-detail).
**Verified:** 2026-06-18T19:40:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| #   | Truth | Status | Evidence |
| --- | ----- | ------ | -------- |
| 1 | `GET /api/v1/items` returns one JSON object per normalized item name held anywhere in the guild | ✓ VERIFIED | `compute.Items` (itemrollup.go:38) composes `View`+`RosterFor`+`ItemMasterIconStats`; `buildItemRollups` groups by `strings.ToLower(strings.TrimSpace(vr.Item))` (itemrollup.go:69) into one `ItemRollup` per name. `TestItems_OK_GroupedRollup` proves Jade Reaver held by 2 chars collapses to one rollup. |
| 2 | Each item carries summed_qty, holder_count, is_mine, holders[] {char, slot_label, qty, last_synced, is_mine, is_bank} | ✓ VERIFIED | `ItemRollup`/`ItemHolder` structs (types.go:225,243) with exact snake_case tags; `buildItemRollups` accumulates `SummedQty += vr.Count`, distinct-char `HolderCount`, `IsMine` from any viewer-assigned holder, one `ItemHolder` per holding (itemrollup.go:89-110). |
| 3 | Items grouped by lower(trim(name)), NEVER item_id (the namespace landmine) | ✓ VERIFIED | Group key = `strings.ToLower(strings.TrimSpace(vr.Item))` (itemrollup.go:69). `vr.ID` is used ONLY for the id-correct `item_master` icon/stats lookup (itemrollup.go:72), never for grouping or price. Iron-law doc comment + memory `pigparse-vs-ingame-item-id-namespaces` honored. |
| 4 | The route is session-gated (RequireSession) and 401s without a cookie | ✓ VERIFIED | `mux.Handle("GET /api/v1/items", webauth.RequireSession(db, readapi.NewItems(st)))` (main.go:370). `TestItems_RequireSession_401WithoutCookie` asserts 401 fail-closed. Operator-confirmed external curl → 401 (was 404 pre-deploy). |
| 5 | An unpriced item returns price:null; a priced item returns its pickPrice value | ✓ VERIFIED | `Price *float64 json:"price"` copied from the representative `vr.Price` (itemrollup.go:75); name-bridged by `View`'s `pp_rep` CTE, never re-selected. readapi test asserts the `price` key in the payload. |
| 6 | The /inventory tab renders a viewer-first selectable list (qty·holders + price + wiki), viewer-priority search, and a master-detail holders table whose rows deep-link to /characters?c= | ✓ VERIFIED | `inventory/+page.svelte` renders one `<button aria-pressed>` per item with icon, name, `{summed_qty} · {holder_count} holders` headline, inline price (omitted when null, line 232), Wiki link; `filterItems` viewer-first search; `ExaminePanel` + holders table whose rows `<a href={/characters?c=${encodeURIComponent(h.char)}}>` (line 320). |

**Score:** 6/6 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `internal/backendsrv/compute/itemrollup.go` | `Items` + pure `buildItemRollups` + `ItemRollup`/`ItemHolder` | ✓ VERIFIED | `func Items(` + `func buildItemRollups(` present; group-by-normalized-name; reuses `classifySlot`/`splitChild`; no `pickPrice` token (0 matches). |
| `internal/backendsrv/compute/itemrollup_test.go` + `_internal_test.go` | table tests over seeded rows + flags | ✓ VERIFIED | Both exist; end-to-end `TestItems_*` + white-box `TestBuildItemRollups`/`TestSlotLabel`. `go test ./compute/...` ok. |
| `internal/backendsrv/readapi/items.go` | `ItemsHandler` + `NewItems` serving `compute.Items` | ✓ VERIFIED | `compute.Items(ctx, h.store, uid)` (line 63); `[]compute.ItemRollup{}` []-not-null coercion (line 73); no `NewItemSearch`/`SearchCatalog` (0 matches). |
| `internal/backendsrv/readapi/items_test.go` | 401 + []-not-null + grouped 200 + 405 | ✓ VERIFIED | All four tests present and substantive (holder_count assertions). |
| `internal/backendsrv/store/readviews.go` | `ItemMasterIconStats` + `IconStats` | ✓ VERIFIED | Full-table `?`-free `SELECT item_id, icon_id, statsblock FROM item_master`; NULL→0/"" via sql.Null* (line 682). |
| `web/src/lib/api.ts` | `ItemRollup`/`ItemHolder` interfaces + `fetchItems()` | ✓ VERIFIED | Both interfaces (field-for-field snake_case mirror), `fetchItems()` over `getJSON('/api/v1/items')`; single `PriceDetail` (no redeclare). |
| `web/src/lib/items.ts` | pure `viewerFirstItems`/`filterItems`/`sortHolders` | ✓ VERIFIED | All three exported, immutable (spread-copy), DOM-free. 12/12 node tests pass. |
| `web/src/routes/inventory/+page.svelte` | master-detail tab (replaces P30 placeholder) | ✓ VERIFIED | Full master-detail; `fetchItems`/`ExaminePanel`/`filterItems`/`sortHolders`/`/characters?c=`/`charLastSeen=""`/`aria-pressed` present; no `@html`/`fonts.googleapis`/"coming soon" (0 matches). |
| `cmd/squirebot-server/main.go` | route registration line | ✓ VERIFIED | `GET /api/v1/items` under `RequireSession` (line 370), distinct from `/items/search` (line 351). |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | --- | --- | ------ | ------- |
| `readapi/items.go` | `compute.Items` | handler calls `compute.Items(ctx, h.store, uid)` | ✓ WIRED | items.go:63 |
| `main.go` | `/api/v1/items` | `mux.Handle GET ... RequireSession` | ✓ WIRED | main.go:370; operator-confirmed 401-not-404 live |
| `itemrollup.go` | `store.RosterFor` | is_mine/bank/bot flags joined by char name | ✓ WIRED | itemrollup.go:43 + RosterFor (readviews.go:722) |
| `inventory/+page.svelte` | `GET /api/v1/items` | `fetchItems()` on mount | ✓ WIRED | onMount → load() → `await fetchItems()` (line 53,83) |
| `inventory/+page.svelte` | `/characters?c=` | holder row deep-link | ✓ WIRED | line 320; target route reads `?c=` and pre-selects (characters/+page.svelte:89) — live, not a dead-end |
| `items.ts` | `ItemRollup` | viewer-first sort over the rollup | ✓ WIRED | `viewerFirstItems`/`filterItems` typed over `ItemRollup` |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| -------- | ------------- | ------ | ------------------ | ------ |
| `inventory/+page.svelte` | `items` | `await fetchItems()` → `GET /api/v1/items` → `compute.Items` → `View`(InventoryJoin/pp_rep) + `RosterFor` + `ItemMasterIconStats` (real SQLite reads) | ✓ Yes — DB-backed compute, no static/hardcoded payload | ✓ FLOWING |
| holders table | `selectedRollup.holders` | server-built `ItemHolder[]` per holding | ✓ Yes — one row per real `ViewRow` holding | ✓ FLOWING |

`asSlot` carries list-irrelevant zero fields (count/slots/children) by design — the examine reads only name/icon/stats/price/wiki, which are real rollup values. Not a hollow prop.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| compute/readapi/store Go tests | `go test ./internal/backendsrv/{compute,readapi,store}/...` | all ok | ✓ PASS |
| full build incl. cmd/squirebot-server (new route compiles) | `go build ./...` | exit 0 | ✓ PASS |
| go vet new packages | `go vet ./compute/... ./readapi/...` | exit 0 | ✓ PASS |
| pure item helpers | `npx vitest run items.test.ts` | 12/12 pass | ✓ PASS |
| no new migration (schema stays v13) | list migrations dir | highest = 00013_item_statsblock; no 00014+ | ✓ PASS |
| live route 401-not-404 | external curl (operator) | 401 (was 404 pre-deploy) | ✓ PASS (operator-confirmed) |
| 7-point browser-smoke × 5 themes | live build at squirebot.quest/inventory | operator "approved" | ✓ PASS (operator-confirmed, 32-03-SUMMARY) |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ----------- | ----------- | ------ | -------- |
| ITEM-01 | 32-01/02/03 | Guild item list w/ name, qty, wiki link, PigParse price | ✓ SATISFIED | Rollup list row: icon + name + `{summed_qty} · {N} holders` + inline price (omitted when null) + Wiki ↗. Browser-smoke point 1 PASSED. |
| ITEM-02 | 32-01/02/03 | Per-item search prioritizing the viewer's items | ✓ SATISFIED | Server stamps `is_mine`; `viewerFirstItems`/`filterItems` float mine-first then A-Z among matches. Browser-smoke point 2 PASSED. |
| ITEM-03 | 32-01/02/03 | Selecting an item shows holders·slot·qty·last-synced (master-detail) | ✓ SATISFIED | `holders[]` {char, slot_label, qty, last_synced}; pinned detail = ExaminePanel + sortHolders table deep-linking to `/characters?c=`. Browser-smoke points 3-5 PASSED. |

All three phase requirement IDs accounted for; none orphaned. (REQUIREMENTS.md traceability rows 83-85 still read "deploy-then-browser-smoke pending" — a stale doc artifact; 32-03-SUMMARY records the smoke as PASSED. Informational, not a code gap.)

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| (none) | — | — | — | No TODO/FIXME/placeholder/stub patterns in any Phase 32 source file; no `@html` sink added; no hardcoded-empty data flowing to the UI (the `[]compute.ItemRollup{}` is the correct []-not-null coercion, not a stub). |

### Human Verification Required

None outstanding. The one DOM-blind / human gate (the 7-point browser-smoke across all 5 themes, incl. the `/characters?c=` holder deep-link) was already performed on the live build and operator-approved (recorded in 32-03-SUMMARY.md, with the fix-forward `c5b2bc4` un-sticking the examine panel). The live route's 401-not-404 was operator-confirmed via external curl.

### Gaps Summary

No gaps. Both halves landed and are genuinely wired: the backend `compute.Items` groups by normalized NAME (the namespace landmine avoided — `vr.ID` only for the id-correct `item_master` icon/stats lookup), the `GET /api/v1/items` route is registered under `RequireSession` and distinct from the P19 `/items/search` catalog route, and the web tab renders a real master-detail (qty·holders headline, inline price, wiki link, viewer-first search, holders table deep-linking into the live `/characters?c=` window). All Go + node tests pass, the full module builds, no new migration (schema v13), and the human browser-smoke was already PASSED across all 5 themes. ITEM-01/02/03 each map to delivered, test-and-smoke-proven behavior.

---

_Verified: 2026-06-18T19:40:00Z_
_Verifier: Claude (gsd-verifier)_
