---
status: resolved
trigger: |
  Wantlist "in guild" indicator shows "Not in guild" for items that guildies actually hold (user added "10 Dose Ant's Potion"; at least one character holds it, but the row reads "Not in guild").
created: 2026-06-06
updated: 2026-06-06
slug: wantlist-in-guild-id-mismatch
---

# Debug: Wantlist "in guild" indicator false-negative (catalog vs inventory item-ID namespace mismatch)

## Symptoms

- **Expected:** A wantlist row for an item that ≥1 guild character holds shows "In guild" (with holders).
- **Actual:** Shows "Not in guild" even though characters hold the item (reported for "10 Dose Ant's Potion"; Findom & Slampeach hold it).
- **Errors:** None — silent false-negative.
- **Timeline:** Present since the wantlist in-guild feature shipped (Phase 19); surfaced 2026-06-06 after Phase 21 deploy when the user exercised the wantlist.
- **Reproduction:** Add a catalog item to the wantlist whose name also exists in held inventory under a different in-game ID → row reads "Not in guild".

## Current Focus

hypothesis: The in-guild join keys on `item_id`, but the wantlist catalog (PigParse `pigparse_price` IDs) and the inventory (`inventory_item`, EQ in-game `/outputfile` IDs) are DIFFERENT item-ID namespaces, so the ID match fails even when the item is held under the same name.
test: Confirm on the live DB that a held, wanted item matches by NAME but not by ID across the two tables; quantify namespace overlap.
expecting: Catalog item_id ≠ inventory item_id for the same item name; low ID-overlap, high name-overlap.
next_action: CONFIRMED + FIXED (frontend name-bridge). Awaiting user deploy + a scope decision on the related bank/view price-coverage degradation.
reasoning_checkpoint: Root cause confirmed against the in-repo `normalized_name` mechanism (readviews.go:362 / gear_check :323-324). Fix mirrors that name-bridge client-side; EC monitor untouched (no Go change).

## Evidence

- timestamp: 2026-06-06 — Wantlist row "10 Dose Ant's Potion" has `wantlist_item.item_id = 19450` (from the catalog/`pigparse_price` add-item search). `pigparse_price` row for that name = `19450`.
- timestamp: 2026-06-06 — Inventory holders (Findom, Slampeach) have the same-named item as `inventory_item.item_id = 14536`. `pigparse_price` does NOT contain `14536` at all (`SELECT COUNT(*) FROM pigparse_price WHERE item_id=14536` → 0).
- timestamp: 2026-06-06 — Namespace overlap is small and systemic: of 713 distinct `inventory_item.item_id`, only **58** exist in `pigparse_price` by ID; yet **559** distinct inventory names match catalog names by name. Both `item_id` columns are INTEGER (no type-affinity artifact).
- timestamp: 2026-06-06 — Same root cause degrades the bank/view price column: `readviews.go:134-135` LEFT JOINs `item_master`/`pigparse_price` on `pp.item_id = ii.item_id`; only **156 / 1773** inventory rows get a price (≈9%). (RELATED issue — flag, scope decision needed.)
- timestamp: 2026-06-06 — The in-guild join lives on the frontend: `web/src/lib/wantlist/holders.ts` `holdersFor(itemId, viewRows)` skips any row where `r.id !== itemId` (raw ID equality). `viewRows` come from `fetchView()` → the consolidated all-inventory view (in-game IDs).
- timestamp: 2026-06-06 — Counter-example that WORKS: `gear_check` deliberately does NOT join by item_id — it matches on the location slot token + item NAME (`readviews.go:323-324`) and uses a materialized `normalized_name` (`readviews.go:362`). This is the proven name-bridge pattern in-repo.
- timestamp: 2026-06-06 — NOT affected: the Phase 21 EC monitor matches the want's catalog item_id against PigParse *auction* item IDs (both in the PigParse namespace), so it is internally consistent and correct.
- timestamp: 2026-06-06 — FIX APPLIED + verified. `holdersFor` now matches by NORMALIZED NAME (`lower(trim(name))`, the gear_check/spell_check convention) over `WantlistRow.item_name` vs `ViewRow.item`, not raw `item_id`. New regression test (catalog id 19450 want vs inventory id 14536 holders, same name) reads "In guild". Frontend-only change — no Go/schema/migration touched, EC monitor unaffected. `npx vitest --run` 261/261 pass (11/11 in holders.test.ts including the regression); `svelte-check` 0 errors; `npm run build` succeeds.

## Eliminated

- hypothesis: The indicator only checks the bank toon (so a non-bank holder reads "not in guild"). — ELIMINATED: `holders.ts` joins against the full consolidated view (all-inventory "honest superset", WantlistPanel.svelte:15-16), not bank-toon-only.
- hypothesis: TEXT-vs-INTEGER column affinity makes the `item_id` comparison falsely unequal. — ELIMINATED: both `inventory_item.item_id` and `pigparse_price.item_id` are INTEGER; and the concrete IDs genuinely differ (19450 vs 14536).
- hypothesis: The item simply isn't in the PigParse catalog (coverage gap). — ELIMINATED: it IS in the catalog by name (19450), just under a different ID than inventory (14536); 559 names match by name vs only 58 by ID.
- hypothesis: The holder's watcher hasn't uploaded / item not in the view. — ELIMINATED: inventory_item has the item (id 14536) for Findom & Slampeach; it's present, just under the in-game ID.

## Proposed Fix (to validate + implement)

Bridge catalog↔inventory by **normalized name** for the wantlist in-guild indicator, mirroring `gear_check`'s `normalized_name` approach (readviews.go:362) rather than raw `item_id` equality. Likely shape: have the view/wantlist payload carry a normalized name and join `holdersFor` on it (or resolve the want's catalog ID → normalized name → inventory holders server-side). Add a regression test proving a same-name/different-ID held item now shows "In guild". Keep custom (null-item) wants → "—". Confirm the EC monitor is untouched.

Scope decision for the session: fix the **in-guild indicator** first (the reported bug). The **bank/view price-coverage** degradation (≈91% of held items unpriced) shares the exact root cause — decide whether to fix it in the same pass or split into a tracked follow-up.

### Follow-up: bank/view price-coverage fix (2026-06-06)

reasoning_checkpoint:
  hypothesis: "readviews.go InventoryJoin LEFT JOINs pigparse_price on pp.item_id = ii.item_id, but PigParse catalog ids and EQ /outputfile inventory ids are different namespaces (only 58/713 inventory ids in catalog by id), so ~91% of held rows get no price. Bridging by normalized name lower(trim(name)) — the gear_check/spell_check convention — restores coverage."
  confirming_evidence:
    - "Evidence line: 156/1773 inventory rows priced (~9%) under the id-join; 559 names match by name vs 58 ids by id (same DB)."
    - "gear_check/spell_check already bridge by lower(trim(name)) successfully (readviews.go:362, :323-324)."
  falsification_test: "If a held item whose catalog id != inventory id (same name) still shows no price after the name-bridge, the hypothesis is wrong."
  fix_rationale: "Bridges the cross-namespace gap at the exact join that drops the price; item_master join stays id-keyed (it is the watcher's own EQ-namespace enrichment, correctly id-keyed)."
  blind_spots: "Names that differ only by trailing punctuation/whitespace across the two sources would still miss; and a name shared by 2 catalog ids must be de-duped to avoid bank row fan-out (handled by the representative-row CTE)."

approach: Expression-join via CTE — NO migration. A CTE `pp_by_name` pre-aggregates pigparse_price to ONE representative row per `lower(trim(name))` (representative = the row whose item_id = MIN(item_id) within that normalized-name group), then InventoryJoin LEFT JOINs that CTE on `lower(trim(ii.name)) = pp_by_name.norm_name`. The item_master join is left UNCHANGED on `im.item_id = ii.item_id` (EQ-namespace, correctly id-keyed). No goose migration / no watcher change / no _meta schema bump — chosen over a materialized normalized_name column because the column would also require changing the P12 pigparse enrich writer to keep it populated (broader blast radius than "change only the join"), and perf is a non-issue at this scale.

fan_out_guard: The CTE collapses pigparse_price to exactly one row per normalized name BEFORE the LEFT JOIN (GROUP BY norm_name, representative = MIN(item_id)). So even when two catalog ids share a normalized name, an inventory row joins to at most one price row → no duplicate view rows, no inflated bank counts. Verified by an explicit regression test.

Deploy note: backend is Go + SQLite live on Hetzner (`api.squirebot.quest`); any backend change needs the cross-compile → scp → `systemctl restart` redeploy per `docs/backend-deploy.md`. A frontend-only fix needs `npm run build` → `scp web/build/*` → `chmod -R a+rX` → `caddy reload`. SSH workaround: Windows `ssh.exe`/`scp.exe` full path + the loaded Windows agent (see `v2-backend-live-and-ops-access` memory).

## Resolution

root_cause: The wantlist in-guild join used raw `item_id` equality (`holders.ts` `r.id !== itemId`), but the wantlist catalog item_id (PigParse `pigparse_price` namespace) and the inventory `ViewRow.id` (EQ in-game `/outputfile` namespace) are DIFFERENT id namespaces (only 58/713 inventory ids exist in the catalog by id vs 559 names matching by name), so a held item under a different id false-negatived to "Not in guild".
fix: Changed `holdersFor` to bridge by NORMALIZED NAME (`lower(trim(name))` — the gear_check/spell_check `normalized_name` convention), matching `WantlistRow.item_name` against `ViewRow.item` instead of raw item_id. New `normalizeItemName` helper exported and tested. Null-item custom-want contract (→ "—") preserved. Frontend-only — no Go/schema/goose migration; the Phase 21 EC monitor (PigParse-namespace internal match) is untouched.
verification: `npx vitest --run` → 261/261 pass (holders.test.ts 11/11, including the regression: catalog id 19450 want + inventory id 14536 holders, same name → both holders returned / "In guild"). `npm run check` (svelte-check) → 0 errors. `npm run build` → succeeds. NOT yet deployed (frontend deploy is LIVE on Hetzner and requires the user — checkpoint for the orchestrator).
files_changed:
  - web/src/lib/wantlist/holders.ts (name-bridge match + normalizeItemName helper + signature `holdersFor(itemId, itemName, viewRows)`)
  - web/src/lib/wantlist/holders.test.ts (signature update + cross-namespace regression + normalizeItemName tests)
  - web/src/lib/components/WantlistPanel.svelte (pass `row.item_name` into holdersFor at the join site)

### Follow-up Resolution: bank/view price-coverage (2026-06-06) — BACKEND fix

root_cause (related): Same cross-namespace mismatch on the BACKEND. `store.InventoryJoin` (readviews.go) LEFT JOINed `pigparse_price` on `pp.item_id = ii.item_id`; catalog ids != EQ /outputfile inventory ids, so ~91% of held rows (1617/1773) silently got no price.
fix (backend): Re-keyed ONLY the pigparse_price join to bridge by NORMALIZED NAME (`lower(trim(name))`). Approach = expression-join via a CTE (`pp_rep`) that pre-aggregates pigparse_price to ONE representative row per normalized name (representative = `MIN(item_id)` per `lower(trim(name))` group), then `LEFT JOIN pp_rep ON pp_rep.norm_name = lower(trim(ii.name))` and `LEFT JOIN pigparse_price pp ON pp.item_id = pp_rep.rep_item_id`. The item_master join stays `im.item_id = ii.item_id` (EQ-namespace, correctly id-keyed). NO goose migration / NO watcher change / NO _meta schema bump — chosen over a materialized normalized_name column because that column would also force a change to the P12 pigparse enrich writer to keep it populated (broader blast radius than "change only the join"); perf is a non-issue at this scale (1773×4341, cached/infrequent read).
fan_out_guard: The CTE collapses pigparse_price to exactly one row per normalized name BEFORE the LEFT JOIN, so a name shared by two catalog ids still yields at most ONE price row per inventory row → no duplicate view rows, no inflated bank totals. Proven by `TestReadViews_PriceNoFanOutOnSharedName` (view + bank paths) and `TestView_PriceBridgesByNameAcrossNamespaces` (single Prices entry on the shared-name item).
verification (backend): Falsified against the old id-join (new tests FAIL: "Ant's Potion price = {has:false …}") then confirmed PASS with the CTE. `go build ./...` clean; `go vet ./internal/backendsrv/...` clean; `go test ./...` all packages OK (store, compute, readapi green incl. updated fixtures). NOT deployed (orchestrator handles the live Hetzner redeploy).
files_changed (backend):
  - internal/backendsrv/store/readviews.go (CTE name-bridge join + doc comment update)
  - internal/backendsrv/store/readviews_test.go (seedPigparse name param; 2 new regression tests: name-bridge + no-fan-out)
  - internal/backendsrv/compute/fixtures_test.go (seedPigparse name param)
  - internal/backendsrv/compute/view_test.go (call-site name; new compute-level name-bridge + fan-out regression)
  - internal/backendsrv/compute/bank_test.go (call-site name)
  - internal/backendsrv/readapi/readapi_test.go (seedPigparse name param + call-site name)
