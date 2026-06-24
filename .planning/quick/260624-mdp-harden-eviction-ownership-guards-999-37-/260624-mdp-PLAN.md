---
quick_id: 260624-mdp
title: Harden eviction/ownership guards (backlog 999.37–39)
status: in-progress
date: 2026-06-24
---

# Quick Task 260624-mdp: Harden eviction/ownership guards (999.37–39)

Three related backend defense-in-depth cleanups deferred from the v2.5 (Phase 35/36)
code review. Backend-only; no schema change; no watcher change; no `v*` tag.

## Task 1 — 999.37 (P36 WR-01): preview-handler guard parity

**File:** `internal/backendsrv/webadmin/eviction.go`

`EvictionPreviewHandler` returns an owner's roster without the floor/sentinel guards
`EvictHandler`/`EvictOwnerTx` enforce — a peer/officer can preview a floor-protected
owner's (or the guild sentinel's) characters (read-only info-disclosure asymmetry).

- **Action:** Before the store reads, mirror the action's guards:
  1. sentinel guard — `if ownerID == store.GuildSentinelOwnerID` → 403 `cannot_evict_sentinel`.
  2. floor guard — `callerMayNotEvictFloor(ctx, db, ownerID, caller(ctx))` → if protected, 403 `owner_floor_protected`.
- **Verify:** new tests — preview of the sentinel → 403 `cannot_evict_sentinel`; a peer
  preview of the floor-protected owner → 403 `owner_floor_protected`; a normal officer
  preview of an ordinary owner still → 200 with the roster.

## Task 2 — 999.38 (P36 WR-02): repoint skips already-evicted stewards

**File:** `internal/backendsrv/store/eviction.go` (+ `eviction_test.go`)

The D-03 `recentOtherSharerSubquery` repoint picks the most-recent OTHER
`cross_owner_write` uploader without excluding evicted owners, so a surviving shared
char can be repointed to an already-evicted steward (cosmetic stale attribution —
`owner_id` is non-binding, the char still survives).

- **Action:** add a live-steward filter to `recentOtherSharerSubquery`:
  `AND EXISTS (SELECT 1 FROM character c2 WHERE c2.owner_id = a.attempting_owner_id AND c2.is_removed = 0)`.
  A candidate sharer with no live char (i.e. evicted) is skipped; if none qualify the
  subquery is NULL → the char is left on its current steward (same as the no-sharer case).
- **Test scope (the backlog under-counted this):**
  - `TestEvictOwnerTx_RepointsSurvivingSharedChar` / `...RepointPicksMostRecentSharer`
    give their sharer owners (Y/Z) NO live char → they now fail. Fix the fixtures: give
    each repoint-target sharer a live owned char so it qualifies as a live steward.
  - Add `TestEvictOwnerTx_RepointSkipsEvictedSharer`: a more-recent but evicted sharer is
    skipped in favour of an earlier LIVE sharer (proves the new filter).
  - Re-scope `TestRepointSubquery_LocksPredicateToSharedPredicate`: keep the three shared
    load-bearing tokens locked across BOTH consts, document the intentional divergence,
    and lock the new live-steward `EXISTS` clause into `recentOtherSharerSubquery` only.

## Task 3 — 999.39 (P35 IN-02): label-bridge can't resolve the reserved sentinel

**Files:** `internal/backendsrv/store/linking.go`, `internal/backendsrv/auth/store.go`,
`internal/backendsrv/webadmin/eviction.go`

Owner-by-label SELECTs can match the sentinel owner (id 1000000, label `guild`) if a
guildie's Discord username / mint `--owner` arg were ever literally "guild". Newly
reachable since the reserved label was introduced (00015).

- **Action:** exclude the sentinel from each owner-by-label SELECT via
  `AND id <> store.GuildSentinelOwnerID` (referenced as the local const inside `store`;
  imported into `auth` — verified no import cycle, `store` does not import `auth`):
  - `linking.go` ResolveOrCreateOwnerByDiscordTx label-bridge SELECT.
  - `auth/store.go` `upsertOwner` SELECT (the mint `--owner <label>` path).
  - `webadmin/eviction.go` `callerMayNotEvictFloor` `TRIM(label)` bridge SELECT.
- **Verify:** `go build ./...` (no cycle) + `go test ./...` green.

## Done when
- `go test ./...` green (Windows host).
- All three guards in place; new/updated tests pass.
