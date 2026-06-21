---
phase: 34-wishlist-rework-per-character-per-slot-upgrades
verified: 2026-06-21T00:00:00Z
status: passed
score: 7/7 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: none
  previous_score: n/a
---

# Phase 34: Wishlist Rework — Per-Character Per-Slot Upgrades Verification Report

**Phase Goal:** The live wantlist becomes a per-character, per-equipment-slot upgrade wishlist — open-ended targets per slot, complete Velious wiki suggestions (price/wiki/last-listed), a Discord ping toggle with an EC-hit badge, the right-click examine, and a wishlist search.
**Verified:** 2026-06-21
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (the 7 WISH requirements + the 5 ROADMAP success criteria)

| #   | Truth (requirement) | Status | Evidence |
| --- | ------------------- | ------ | -------- |
| WISH-01 | Char list viewer-first A-Z, excludes banks/bots, per-character | ✓ VERIFIED | `web/src/lib/wishlist/wishlist.ts` `wishlistRoster` filters `is_bank_toon\|\|is_guild_bot` OUTRIGHT then sorts mine→guild A-Z; `+page.svelte` renders the two-band list over `fetchCharacters()`; node test `wishlist.test.ts` 15/15 PASS |
| WISH-02 | Select char → equipped slots w/ equipped item per slot | ✓ VERIFIED | `compute.WishlistFor` builds the fixed 21-worn-slot list (`wornSlots`), `equippedBySlot` from `StructuredInventory().Equipment` ("" = empty, D-04); `+page.svelte` per-slot accordion renders `slot.equipped \|\| Empty`; `TestBuildWishlistView_AllWornSlotsIncludingEmpty` PASS |
| WISH-03 | Open-ended user targets per slot; auto-removed when seen or removed | ✓ VERIFIED | `wishlist_item` table + owner-scoped `AddWishlistTx`/`RemoveOwnWishlistTx` (soft-delete) + write API; D-02 auto-removal = `buildWishlistView` hides a target whose `norm(name)` is in the held-set (`TestBuildWishlistView_AutoRemovalHidesHeldTargets` PASS); UI add (typed/custom) + ConfirmDialog remove, server-truth re-fetch |
| WISH-04 | Per-slot complete Velious Pre-raid/Grouping + Raiding suggestions for class+slot, each price+wiki+last-listed, Raid → not-for-sale | ✓ VERIFIED | `buildWishlistView` filters `store.GearTierPrices` by `Class==class && Slot==wikiSlotFor(canonical)`; slot bridge UNIT-TESTED (`TestWikiSlotFor_CanonicalToWikiBridge` — Finger1&Finger2→Fingers); `IsRaid = Tier=="Velious Raiding"` (no invented no_drop column); price via `PriceByName`/`GearTierPrices` name-key; UI renders Raid tag + "Not for sale" |
| WISH-05 | Per-item Discord ping toggle + EC-hit badge (reuse EC/notify spine) | ✓ VERIFIED | `wantmatch/match.go` repointed `FROM wishlist_item`, `pinged=1` gate, INNER JOIN, no note; DM target = `w.discord_user_id` (owner, not char owner — `TestForItem_DMTargetIsWishOwner_NotCharacterOwner` PASS); sole caller `ec/ec.go:211` unchanged; `SetPingedTx` toggle + `AlertedWishlistIDs` badge set → `pinged_hit`; UI Toggle (server-truth) + "Seen in EC" badge |
| WISH-06 | Hover/tap examine | ✓ VERIFIED | `+page.svelte` reuses `ExaminePanel` via the `asSlot` seam (category:'general', charLastSeen="") on every target/equipped/suggestion; no new `{@html}` sink |
| WISH-07 | Wishlist search over all wishlists + non-bank/bot chars | ✓ VERIFIED | `searchWishlistItems` groups over the FULL passed-in corpus (no scope-down); `+page.svelte` `ensureCorpus()` lazily fetches EVERY `wlChars` (non-bank/bot) wishlist into `wishlistCache`, two result groups CHARACTERS + WISHLIST ITEMS; node test covers a cross-char match |

**Score:** 7/7 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `internal/backendsrv/migrations/00014_wishlist.sql` | wishlist_item + alert_log FK rebuild + drop wantlist_item | ✓ VERIFIED | alert_log rebuilt to FK `wishlist_item(id)` (column name `wantlist_item_id` kept) BEFORE `DROP TABLE wantlist_item`; down = `SELECT 1;` no-op; NO schema gate added |
| `internal/backendsrv/migrations/migrate_test.go` | idempotent apply + final schema | ✓ VERIFIED | `TestMigrate_00014_AddsWishlist`: wishlist_item 9 cols, wantlist_item GONE, NULL-FK + real-FK alert_log inserts, idempotent re-run; historical tests pinned via `openAtVersion`/`UpTo` |
| `internal/backendsrv/store/wishlist.go` | owner-scoped CRUD + ping + badge read | ✓ VERIFIED | `AddWishlistTx` (2067→`ErrDuplicateWishlist`), `RemoveOwnWishlistTx`/`SetPingedTx` (WHERE discord_user_id=? → silent no-op), `ListOwnWishlist`, `AlertedWishlistIDs` |
| `internal/backendsrv/store/readviews.go` `PriceByName` | name-keyed full-catalog price (pp_rep bridge) | ✓ VERIFIED | pp_rep CTE (lower(trim(name)), MIN(item_id)) → `map[string]PriceByNameRow`; the WARNING-3 fix (target price not limited to gear-tier slice) |
| `internal/backendsrv/compute/wishlist.go` | WishlistFor + slot bridge + auto-removal + suggestions | ✓ VERIFIED | pure `buildWishlistView`; `wikiSlotFor` inverts `enrich.WIKI_SLOT_TO_INV_SLOTS` (Ammo/Charm/Power→"") ; 5 compute tests PASS |
| `internal/backendsrv/wantmatch/match.go` | matcher repointed to wishlist_item | ✓ VERIFIED | `FROM wishlist_item`, pinged=1, INNER JOIN, no note; ForName `= ? COLLATE NOCASE`; T-28-06 owner invariant preserved |
| `internal/backendsrv/webadmin/wishlist.go` | owner-scoped add/remove/ping handlers | ✓ VERIFIED | `caller(ctx)` owner, in-tx `IsCharAssignedToTx`→403, `ErrDuplicateWishlist`→409, slot-enum→400, audit IDs-only |
| `internal/backendsrv/readapi/wishlist.go` | GET /api/v1/wishlist/{char} | ✓ VERIFIED | `compute.WishlistFor(ctx, store, uid, char)`; {char} PathValue `?`-bound; nil→[] coercion; empty-not-404 |
| `cmd/squirebot-server/main.go` | 4 wishlist routes registered, 5 wantlist removed | ✓ VERIFIED | all 4 under `RequireSession`; zero `/api/v1/wantlist*` registrations remain |
| `web/src/lib/api.ts` | WishlistView interfaces + fetch/mutation wrappers | ✓ VERIFIED | 4 interfaces field-for-field w/ Go; `fetchWishlist`/`addWishlist`/`removeWishlist`/`setWishlistPing` |
| `web/src/lib/wishlist/wishlist.ts` | banks/bots-excluded viewer-first + cross-wishlist search | ✓ VERIFIED | `wishlistRoster`/`filterWishlistRoster`/`searchWishlistItems`; 15/15 node tests PASS |
| `web/src/routes/wishlist/+page.svelte` | per-character per-slot master-detail tab | ✓ VERIFIED | full accordion + lazy WISH-07 corpus + server-truth writes + owner-gating + ExaminePanel + Notifications region kept |
| `WantlistPanel.svelte` + `wantlist/groupByChar.ts` | DELETED | ✓ VERIFIED | both gone; `priority.ts`+`holders.ts` KEPT (live consumers); `/wantlist`→`/wishlist` 308 intact |

### Key Link Verification

| From | To | Via | Status |
| ---- | -- | --- | ------ |
| `compute/wishlist.go` | `store.GearTierPrices` | class + wiki-slot bridge filter | ✓ WIRED |
| `compute/wishlist.go` | `store.PriceByName` | target price by normalized name (full catalog) | ✓ WIRED |
| `compute/wishlist.go` | `compute.StructuredInventory` | equipped per slot + held-name auto-removal set | ✓ WIRED |
| `00014.sql` | `alert_log` | DROP+CREATE FK→wishlist_item BEFORE drop wantlist_item | ✓ WIRED |
| `ec/ec.go:211` | `wantmatch.ForItem` | sole live caller, unchanged signature | ✓ WIRED |
| `webadmin/wishlist.go` | `store.IsCharAssignedToTx` | in-tx char-ownership authz before add | ✓ WIRED |
| `main.go` | `readapi.NewWishlist` / `webadmin.*Wishlist*` | RequireSession route registration | ✓ WIRED |
| `+page.svelte` | `/api/v1/wishlist/{char}` | `fetchWishlist` on select + lazy corpus | ✓ WIRED |
| `+page.svelte` | `ExaminePanel` | asSlot reuse seam | ✓ WIRED |
| `+page.svelte` | `addWishlist`/`removeWishlist`/`setWishlistPing` | await POST → re-fetch (server-truth) | ✓ WIRED |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Full Go module green (no clean-break regression) | `go test ./...` | 0 failures | ✓ PASS |
| Migration 00014 idempotent + final schema | `go test ./internal/backendsrv/migrations -run TestMigrate_00014` | PASS | ✓ PASS |
| Slot bridge (Finger1&Finger2→Fingers, Ammo→"") | `go test ./compute -run TestWikiSlotFor` | PASS | ✓ PASS |
| Auto-removal (D-02) + name-keyed price (WARNING-3) | `go test ./compute -run TestBuildWishlistView` | PASS | ✓ PASS |
| DM-target-is-owner regression (T-28-06) | `go test ./wantmatch -run DMTarget` | PASS | ✓ PASS |
| IDOR / owner-scoping (403 + silent no-op) | `go test ./webadmin -run Wishlist` | PASS | ✓ PASS |
| Web type/lint gate | `npm run check` | 0/0, 508 files | ✓ PASS |
| Web full node suite | `npx vitest run` | 380/380, 29 files | ✓ PASS |
| Wishlist node helpers | `npx vitest run src/lib/wishlist` | 15/15 | ✓ PASS |
| Go module builds | `go build ./...` | rc=0 | ✓ PASS |
| Live route surface (`/api/v1/wishlist/{char}` 401, `/api/v1/wantlist` 404) | `curl` from this env | sandbox-intercepted (uniform 200/405) — NOT verifiable here | ? SKIP → operator-confirmed in 34-04-SUMMARY |

### Requirements Coverage

| Requirement | Source Plan | Status | Evidence |
| ----------- | ----------- | ------ | -------- |
| WISH-01 | 34-02, 34-03 | ✓ SATISFIED | wishlistRoster banks/bots-excluded viewer-first list |
| WISH-02 | 34-01, 34-03 | ✓ SATISFIED | compute.WishlistFor 21-slot equipped + per-slot accordion |
| WISH-03 | 34-01, 34-02, 34-03 | ✓ SATISFIED | wishlist_item + owner-scoped CRUD + D-02 auto-removal + add/remove UI |
| WISH-04 | 34-01, 34-03 | ✓ SATISFIED | class+slot gear-tier suggestions + slot bridge + Raid tag + name-keyed price |
| WISH-05 | 34-02, 34-03 | ✓ SATISFIED | matcher repoint (pinged gate, owner DM) + ping Toggle + EC-hit badge |
| WISH-06 | 34-03 | ✓ SATISFIED | reused ExaminePanel via asSlot seam on hover/tap |
| WISH-07 | 34-03 | ✓ SATISFIED | two-group search; corpus = every non-bank/bot char's wishlist (lazy fetch, no scope-down) |

No orphaned requirements — all 7 WISH IDs map to delivered code; REQUIREMENTS.md marks all 7 ✅ Phase 34.

### Anti-Patterns Found

None blocking. The genuinely-dead `store/wantlist.go` + `webadmin/wantlist.go` remain in-tree (their routes are unregistered; they compile because SQL is a runtime string) — intentional per the 34-02/34-03 plan notes (DELETE-only scope was limited to the web dead surface). No live runtime path references the dropped `wantlist_item` table (only historical migration 00011 + the kept-by-name `alert_log.wantlist_item_id` column + comments/tests).

### Human Verification Required

The live-runtime + visual legs were ALREADY performed and approved by the operator (34-04-SUMMARY, "approved"): goose→v14 in the restart log, on-box schema (wishlist_item present / wantlist_item gone), `/api/v1/wishlist/{char}` 401-not-404, `/api/v1/wantlist` 404, R2 pre-migration backup confirmed, and the 10-point browser-smoke across all 5 EQ themes (add/remove, ping toggle, Raid-tag suggestion add, auto-hide, the cross-character WISH-07 search, owner-scoped read-only). These cannot be re-verified programmatically from this sandbox (outbound HTTPS is intercepted), but the CODE that backs every one of them is verified above, and the operator record closes the human leg. No NEW human verification is requested.

### Gaps Summary

None. All 7 WISH requirements map to substantive, wired, data-flowing code. The D-01 clean break is correctly ordered (alert_log rebuilt before the drop) and the migrate test asserts idempotent apply + the final schema. D-02 auto-removal is a pure compute-on-read name-join (unit-tested). WISH-04's slot bridge + Raid-tag + catalog-name price resolution are unit-tested. The WISH-05 matcher repoint preserves the DM-target-is-owner invariant (ported regression PASS), and its sole caller is unchanged. Owner-scoping (IDOR) is enforced in-tx (IsCharAssignedToTx→403 + silent cross-owner no-op, all tested). WISH-07's corpus is all non-bank/bot wishlists with no scope-to-loaded escape hatch. The watcher is untouched and no schema gate was added (none exists in the off-Google backend). `go test ./...` is fully green and the web gates (check 0/0, test 380/380, build) pass.

---

_Verified: 2026-06-21_
_Verifier: Claude (gsd-verifier)_
