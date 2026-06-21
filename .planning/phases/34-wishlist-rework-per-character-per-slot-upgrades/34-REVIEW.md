---
phase: 34-wishlist-rework-per-character-per-slot-upgrades
reviewed: 2026-06-21T17:00:00Z
depth: deep
files_reviewed: 14
files_reviewed_list:
  - internal/backendsrv/migrations/00014_wishlist.sql
  - internal/backendsrv/migrations/migrate_test.go
  - internal/backendsrv/store/wishlist.go
  - internal/backendsrv/store/readviews.go
  - internal/backendsrv/store/eccursor.go
  - internal/backendsrv/compute/wishlist.go
  - internal/backendsrv/compute/types.go
  - internal/backendsrv/wantmatch/match.go
  - internal/backendsrv/ec/embed.go
  - internal/backendsrv/webadmin/wishlist.go
  - internal/backendsrv/readapi/wishlist.go
  - cmd/squirebot-server/main.go
  - web/src/lib/api.ts
  - web/src/lib/wishlist/wishlist.ts
  - web/src/routes/wishlist/+page.svelte
findings:
  blocker: 0
  high: 0
  medium: 1
  warn: 2
  info: 2
  total: 5
status: clean
---

# Phase 34: Code Review Report — Wishlist Rework (per-character / per-slot)

**Reviewed:** 2026-06-21T17:00:00Z
**Depth:** deep (cross-file: migration FK chain, owner-scoping call chains, slot-vocabulary bridge, matcher repoint)
**Files Reviewed:** 14 source files (+ their tests cross-referenced)
**Status:** clean (0 BLOCKER / 0 HIGH)

## Summary

Phase 34 replaces the retired item-centric wantlist with a per-character / per-slot
upgrade wishlist (the D-01 clean break). I reviewed it adversarially with the two
highest-value surfaces — the 00014 migration's FK-rebuild ordering and the owner-scoping
on every write — as the primary targets, and traced the matcher repoint, the
slot-vocabulary bridge, and the WISH-07 corpus scope across module boundaries.

**Verdict: no BLOCKER or HIGH findings. The load-bearing invariants hold.**

Verified-correct (the things most likely to be wrong, proven right):

- **Migration FK ordering (00014).** alert_log is rebuilt to FK `wishlist_item(id)`
  BEFORE `DROP TABLE wantlist_item`, so the drop never FK-violates. The column name
  `wantlist_item_id` is kept (Pitfall 6 option B) so `store/alertlog.go` is untouched.
  Full goose chain 00001→00014 applies cleanly in every store/migrate test; idempotent
  re-run proven; NULL-FK test-alert insert + real-FK insert both proven post-rebuild.
- **Owner-scoping (the HIGH-risk surface).** Add authorizes the untrusted `character_id`
  in-tx via `IsCharAssignedToTx` BEFORE the insert → `ErrCharNotAssigned` → 403, rollback
  leaves no row/no audit. Remove/ping are `WHERE id=? AND discord_user_id=? AND active=1`
  → cross-owner mutation is RowsAffected=0 → silent no-op, never leaks existence. All
  three IDOR paths (forged-tag add 403, cross-owner remove no-op, cross-owner ping no-op)
  are tested in `webadmin/wishlist_test.go`. A guildie cannot mutate another's wishlist.
- **Matcher repoint.** `wantmatch` queries `wishlist_item` with `active=1 AND pinged=1`,
  INNER JOIN character (display-only). The DM target is `wishlist_item.discord_user_id`
  (the want owner), proven independent of character owner/assignee by
  `TestForItem_DMTargetIsWishOwner_NotCharacterOwner` (char owned by a 3rd party, assigned
  to a 4th). The dropped note column is safely handled — `whyWanted` ignores the `Hit`
  and returns a constant, so no nil-deref; embed test asserts the never-empty contract.
- **Slot-vocabulary bridge (Pitfall 2, highest-risk).** `invSlotToWiki` inverts
  `enrich.WIKI_SLOT_TO_INV_SLOTS`; all 21 `wornSlots` Title-case tokens upper-case to
  exactly the inv tokens in `equipmentSlots`/the wiki map (Finger1+Finger2→Fingers, etc.);
  Ammo/Charm/Power→"" (empty suggestions, intended). `equippedBySlot` keys on the same
  Title-case `CanonicalSlot` the structured inventory emits. Verified by table tests.
- **WISH-07 corpus scope.** `searchWishlistItems` groups over the WHOLE passed-in corpus
  (every non-bank/bot char's wishlist), with an explicit no-scope-to-loaded-view test.
- **Compute authors zero SQL; reads are `?`-bound.** All new SQL lives in store/ with `?`
  placeholders (slot bridge is in-Go; the auto-removal name join is in-Go over already-read
  structs). `{char}`/uid bind only as `?`; V7 slog is counts/status only.
- **XSS.** All names render via plain Svelte `{}`; the only `{@html}` is the reused
  ExaminePanel (no new sink). Wiki/custom-target hrefs are encodeURIComponent'd.

Ground truth: `go build ./...` clean; full backend `go test` green; `svelte-check`
508 files / 0 errors / 0 warnings; 15 wishlist node tests pass.

The findings below are all quality/cleanliness — none affects correctness or security of
the shipping wishlist path.

## Medium

### MD-01: Dead client wantlist API surface now points at removed server routes

**File:** `web/src/lib/api.ts:735-837, 999-1005`
**Issue:** Phase 34 removed the 5 `/api/v1/wantlist*` server routes (`main.go` confirms;
the route comment even notes "they would 500 on the missing table"). But the client
wrappers that call them remain live exports in `api.ts`: `fetchOwnWants` (GET
`/api/v1/wantlist`), `fetchGuildWants` (GET `/api/v1/wantlist/guild`), `addWant` (POST
`/api/v1/wantlist`), `removeWant` (POST `/api/v1/wantlist/remove`), and `muteWant` (POST
`/api/v1/wantlist/mute`), plus the `WantlistRow`/`GuildWantRow` interfaces. Any call now
hits a route the server no longer registers (404/500). They are presently unreachable from
any live route (`/wantlist` is only a 308 redirect stub to `/wishlist`; nothing on a live
page imports them — the sole reference is a code comment in `+page.svelte:394`), so this is
latent rather than active, but it is a real divergence: the client believes endpoints exist
that the server deleted. The orphan cluster that depends on these wrappers
(`columns.ts` wantlist/guild-wantlist columns, `WantAddForm.svelte`,
`cells/{WantItemCell,WantMuteCell,WantRemoveCell,InGuildCell}.svelte`,
`wantlist/holders.ts`, `wantlist/priority.ts`) is correspondingly dead. Note the task spec
explicitly KEPT `holders.ts`/`priority.ts`, so leaving those is intentional; the issue is
the wrappers + the components that 404.
**Fix:** Delete the retired wantlist wrappers + their now-orphaned interfaces/components
(or, if a quick targeted cleanup is preferred, at minimum remove the 5 fetch wrappers and
the `WantlistRow`/`GuildWantRow` types so no live or test code can call a deleted route).
This is a deliberate-scope call for the user — flag, don't auto-fix.

## Warnings

### WR-01: api.test.ts asserts the shape of a deleted endpoint (`muteWant` → `/api/v1/wantlist/mute`)

**File:** `web/src/lib/__tests__/api.test.ts:26, 295-298`
**Issue:** The test `'muteWant POSTs /api/v1/wantlist/mute with {id, muted}'` still runs and
passes (fetch is mocked), but `/api/v1/wantlist/mute` was removed from the server in this
phase. A green test now guards a 404 contract — it gives false confidence that the mute path
works end-to-end. This is the test-side tail of MD-01.
**Fix:** Remove the `muteWant` import + its test block when MD-01's wrappers are deleted. If
`muteWant` is kept for now, add an xfail/skip note that the server route no longer exists so
the green test isn't read as proof the endpoint is live.

### WR-02: Catalog target accepts an empty `item_name` (validation gap on the `item_id != null` branch)

**File:** `internal/backendsrv/webadmin/wishlist.go:73-89` (`validWishlist`)
**Issue:** `validWishlist` requires a non-blank label only when `ItemID == nil`
(`if req.ItemID == nil && name == ""`). A request `{"character_id":N,"slot":"Chest",
"item_id":500,"item_name":""}` passes validation and stores a catalog target with an empty
`item_name`. Consequences: the matcher's `ForName` can never match it (empty name); the
wishlist row + the examine render a blank name; the compute target-price lookup keys on
`norm("")`. It is not exploitable and the live UI always sends a non-empty `item.name` from
the catalog pick (`pickCatalog`), so this is a robustness gap, not a live bug — but the
server is the trust boundary and should not persist an empty-named target.
**Fix:** Require a non-blank trimmed `item_name` unconditionally (the name is always a
snapshot the row needs), e.g.:
```go
name := strings.TrimSpace(req.ItemName)
if name == "" {
    return false
}
if utf8.RuneCountInString(name) > 200 {
    return false
}
```
This collapses the two name branches into one and closes the empty-catalog-name case.

## Info

### IN-01: `RemoveOwnWishlistHandler` / `SetWishlistPingHandler` route through `mapWishlistErr`, which can only ever return 500 for them

**File:** `internal/backendsrv/webadmin/wishlist.go:215, 266`
**Issue:** Both handlers call `mapWishlistErr` on their tx error, but neither can produce
`ErrDuplicateWishlist` or `ErrCharNotAssigned` (those are add-only) — so the 409/403 arms
are dead for these two callers; only the default 500 path is reachable. This is harmless
(the shared helper keeps the handlers symmetric and the dead arms cost nothing), and arguably
better than divergent error mapping. Noting only so a future reader doesn't assume remove/ping
can 409/403.
**Fix:** None required. Optionally a one-line comment on each call site clarifying that only
the 500 arm applies here.

### IN-02: `corpusEnsured` one-shot guard captures `wlChars` at first-fire; relies on render-gating to be non-empty

**File:** `web/src/routes/wishlist/+page.svelte:167-198`
**Issue:** `ensureCorpus()` sets `corpusEnsured = true` before awaiting, then iterates
`wlChars` (a `$derived` of the roster). If it ever fired while `wlChars` were empty, the
corpus would never populate (the guard is permanent). In practice the search input — the only
thing that flips `searching` true and triggers the `$effect` → `ensureCorpus` — renders only
inside the `{:else}` branch reached when `wlChars.length > 0`, so the roster is always loaded
first. The invariant holds today but is implicit (a future refactor that surfaces the search
box before the roster guard would silently break the corpus). Not a current bug.
**Fix:** None required. Optionally make the guard tolerant — only latch `corpusEnsured` once a
non-empty `wlChars` has actually been iterated, so an early fire is retried rather than
permanently swallowed.

---

_Reviewed: 2026-06-21T17:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
