---
phase: 35-owner-less-guild-banks-bots
reviewed: 2026-06-22T00:00:00Z
depth: standard
files_reviewed: 7
files_reviewed_list:
  - internal/backendsrv/migrations/00015_guild_owner.sql
  - internal/backendsrv/migrations/migrate_test.go
  - internal/backendsrv/store/assignment.go
  - internal/backendsrv/store/eviction.go
  - internal/backendsrv/store/eviction_test.go
  - internal/backendsrv/store/owner.go
  - internal/backendsrv/store/owner_test.go
findings:
  critical: 1
  warning: 2
  info: 2
  total: 5
status: fixed
fix_summary: 35-REVIEW-FIX.md
resolution:
  CR-01: fixed
  WR-01: fixed
  WR-02: fixed
  IN-01: fixed
  IN-02: deferred-to-backlog
---

# Phase 35: Code Review Report

**Reviewed:** 2026-06-22
**Depth:** standard
**Files Reviewed:** 7
**Status:** issues_found

## Summary

This phase introduces a reserved "guild" sentinel owner (id `1000000`, label `guild`) so designated guild banks/bots become owner-less and survive their first-uploader's eviction. The migration (00015) seeds the sentinel and backfills existing banks/bots; `DesignateCharTx` repoints `owner_id` to the sentinel on bank/bot designation; and the two eviction owner-list reads (`ListEvictableOwners`/`ListRestorableOwners`) exclude the sentinel from the picker UI.

The mechanics are mostly sound and well-tested: the migration is forward-only and idempotent (`INSERT OR IGNORE` on the PK + `owner_id <> sentinel` backfill guard); the FK ordering is correct (sentinel seeded in step 1, before the step-2 backfill, with `foreign_keys(ON)` enforced via the DSN); the repoint sits correctly behind the in-tx `isOfficerTx` gate; and the list-exclusion `?` placeholders bind `GuildSentinelOwnerID` in the right positional order. Tests cover the survives-eviction proof, the repoint, the officer gate, and the non-re-homing of `neither`.

However, there is one load-bearing gap: **the destructive eviction endpoint and `EvictOwnerTx` itself have NO sentinel guard.** The list exclusions only protect the picker UI; an officer can directly POST `{"owner_id": 1000000}` and evict the entire guild bank — defeating the phase's core OWN-02 invariant. The phase's own comment claims the list exclusion "stops an officer from accidentally evicting the whole guild bank," which is not true of the directly-POST-able write path.

## Critical Issues

### CR-01: Guild sentinel is excluded from the eviction picker UI but NOT from the destructive evict path — OWN-02 is bypassable

**Status: FIXED** (commit `11ebcc6`). Added typed `store.ErrCannotEvictSentinel`; guarded the WRITE path at the top of both `EvictOwnerTx` and `RestoreOwnerTx`; mapped to 403 `cannot_evict_sentinel` in `mapEvictionErr` (surfaced by both handlers). Tests: store-level `TestEvictOwnerTx_RefusesSentinel` + `TestRestoreOwnerTx_RefusesSentinel`, handler-level `TestEvict_RefusesGuildSentinel` (POSTs `owner_id=1000000` → 403 + bank survives + no audit row).

**File:** `internal/backendsrv/store/eviction.go:182` (EvictOwnerTx) and `internal/backendsrv/webadmin/eviction.go:134-191` (EvictHandler)

**Issue:** OWN-02 requires the sentinel to be eviction-proof. This phase added `o.id <> ?` exclusions to `ListEvictableOwners`/`ListRestorableOwners` (the picker reads) and added a comment stating the exclusion "stops an officer from accidentally evicting the whole guild bank." It does not. `EvictHandler` decodes an arbitrary `req.OwnerID` from the request body (only validating `> 0`) and passes it straight to `store.EvictOwnerTx(ctx, tx, req.OwnerID, now)`. `EvictOwnerTx` runs `UPDATE character SET is_removed=1, grace_until=? WHERE owner_id=? AND is_removed=0` with no sentinel check. A POST of `{"owner_id": 1000000}` therefore soft-removes every guild-held bank/bot, stamps a 30-day grace, and (because the sentinel owns no `guild_code`, that part no-ops) leaves the entire guild bank evicted — the exact outcome OWN-02 exists to prevent.

The route is officer-gated (`RequireOfficer` + in-tx `IsOfficerTx`), so this is not an unauthenticated path; it is an officer-blast-radius/data-loss defect. But "accidental" eviction of the guild bank via a stale/replayed picker entry, a scripted call, or a future UI that surfaces `owner_id` directly is precisely the scenario the phase set out to make impossible "by construction." The list exclusion is necessary but not sufficient; the guard must live on the write path.

`RestoreHandler`/`RestoreOwnerTx` share the same untrusted-`owner_id` shape; while a sentinel restore is largely inert under normal flow (the sentinel's chars are never in grace), it should be guarded for the same reason once a direct sentinel-evict has occurred.

**Fix:** Reject the sentinel id at the destructive boundary, not just in the picker. Cheapest and most robust is to guard inside `EvictOwnerTx` (single source of truth, covers every caller):

```go
// EvictOwnerTx ...
func EvictOwnerTx(ctx context.Context, tx *sql.Tx, ownerID, now int64) (removedCount int, graceUntil int64, err error) {
	// OWN-02: the guild sentinel holds owner-less banks/bots and is NEVER evictable.
	// Guard the WRITE path, not just the picker list (the endpoint takes an untrusted owner_id).
	if ownerID == GuildSentinelOwnerID {
		return 0, 0, ErrCannotEvictSentinel // new typed sentinel; handler maps to 400/403
	}
	graceUntil = now + EvictionGraceSeconds
	// ... unchanged
}
```

Add the typed error and map it in `mapEvictionErr` (e.g. to 403 `not_authorized` or 400 `invalid_input`). Apply the same guard to `RestoreOwnerTx`. Add a test that POSTs `owner_id=1000000` to `EvictHandler` and asserts the bank survives (`is_removed=0`, no grace) and the call is refused — the existing `TestEvictOwnerTx_GuildBankSurvivesEviction` only proves the bank survives when a *different* owner is evicted, not when the sentinel itself is targeted.

## Warnings

### WR-01: Migration 00015's in-line backfill is never exercised against pre-existing data; the test re-runs a hand-copied duplicate of the SQL

**Status: FIXED** (commit `62acc78`). Added `TestMigrate_00015_BackfillRunsOverPreExistingData` mirroring `TestMigrate_00011` part (c): `openAtVersion(t, 14)` → seed an owner-bound bank + bot + a normal char → `migrations.UpTo(raw, 15)`, driving the ACTUAL embedded `00015_guild_owner.sql` backfill over non-empty data; asserts the bank/bot repoint to the sentinel and the normal char is untouched. The existing hand-copied-SQL test was retained for its idempotency + second-`RunMigrations` coverage.

**File:** `internal/backendsrv/migrations/migrate_test.go:1279-1287, 1326-1336`

**Issue:** `TestMigrate_00015_*` seeds owner-bound banks/bots and then runs `backfillBankOwnerSQL` — a string literal *copied* from the migration — rather than exercising the migration's actual embedded backfill against non-empty data. This is forced by `NewTestDB` always migrating an empty DB (so the real in-line backfill touches zero rows), and the test comment acknowledges it. The risk: the migration's embedded SQL and the test's copy can drift independently, and the test would still pass while the shipped migration's backfill is wrong (e.g. a missing `is_guild_bot` term, a wrong guard). The OWN-04 backfill is the highest-stakes correctness claim in this phase (it silently re-homes real guildie-owned banks), so it deserves a test of the *actual* migration step.

**Fix:** Add a raw-handle test that mirrors `TestMigrate_00011`'s part (c): open a fresh `store.Open` DB, `migrations.UpTo(db, 14)`, seed an owner-bound bank + bot + a normal char, then `migrations.UpTo(db, 15)` and assert the bank/bot repointed to `1000000` while the normal char is untouched. That drives the embedded backfill (not a copy) over pre-existing rows and removes the drift risk.

### WR-02: `Down` migration is a no-op `SELECT 1` while the `Up` makes destructive, non-reversible owner_id mutations

**Status: FIXED — doc-only** (commit `96ea4ad`). Added an IRREVERSIBLE note to the Up header + a Down-block clarification stating the pre-backfill first-uploader binding is NOT recoverable via `goose down` (the project's forward-only `SELECT 1;` no-op) — recovery is "restore from the R2 backup". The `Down` stays `SELECT 1;` per the deliberate forward-only convention (no SQL behavior change).

**File:** `internal/backendsrv/migrations/00015_guild_owner.sql:34-36`

**Issue:** The `Up` step overwrites `character.owner_id` for every bank/bot, discarding the original first-uploader binding (the comment notes the steward marker is "intentionally discarded"). The `Down` is `SELECT 1` (consistent with 00004–00014's forward-only convention). This is an accepted project pattern, but it means a `goose down` is a silent lie here: it advances the version backward without restoring the pre-backfill `owner_id` values, which are now unrecoverable from the DB alone. If an operator ever runs `down` expecting a rollback (e.g. to recover a mis-designated bank's original owner), they get a no-op and permanent data loss of the original binding.

**Fix:** This is a deliberate forward-only project decision, so no code change is mandatory — but the original `owner_id` is the only piece of state this migration destroys irreversibly. Consider documenting in the migration header that the pre-backfill binding is NOT recoverable post-00015 (so recovery is "restore from R2 backup," not "goose down"), or, if any reversibility is wanted, snapshot the pre-backfill `(character_id, old_owner_id)` pairs into `_audit`/a side table before the `UPDATE`. At minimum, the irreversibility should be explicit so an operator does not trust `down`.

## Info

### IN-01: Sentinel id duplicated across three locations with only comments guarding the invariant

**Status: FIXED** (commit `62acc78`, alongside WR-01). Replaced the duplicated `const guildSentinelOwnerID = 1000000` in `migrate_test.go` with a binding to `store.GuildSentinelOwnerID` (the Go source of truth), collapsing three drifting literals to two (only the unavoidable `.sql` literal remains independent). Added `TestGuildSentinelOwnerID_MatchesContract` asserting `store.GuildSentinelOwnerID == 1000000` so an accidental edit to the const is caught against the documented contract.

**File:** `internal/backendsrv/store/owner.go:30`, `internal/backendsrv/migrations/00015_guild_owner.sql:23,30`, `internal/backendsrv/migrations/migrate_test.go:1247`

**Issue:** The literal `1000000` lives in (a) `store.GuildSentinelOwnerID`, (b) the migration `.sql` (twice), and (c) a duplicated `const guildSentinelOwnerID` in the migration test. The "they MUST all be equal" invariant is enforced only by comments. The migration SQL legitimately cannot import the Go const, so some duplication is unavoidable; the test-side duplicate, however, is gratuitous — the test is an external package (`migrations_test`) that already imports `store`, so it could reference `store.GuildSentinelOwnerID` directly and let one of the three copies be machine-checked.

**Fix:** In `migrate_test.go`, replace `const guildSentinelOwnerID = 1000000` with `store.GuildSentinelOwnerID` (cast to the needed type). That binds the test to the Go source of truth and collapses three drifting literals to two, leaving only the unavoidable SQL literal. Optionally add a one-line test asserting `store.GuildSentinelOwnerID == 1000000` so an accidental edit to the const is caught against the documented contract.

### IN-02: Label-bridge lookups can match the sentinel's `'guild'` label (pre-existing, out of phase scope but newly reachable)

**Status: DEFERRED-TO-BACKLOG** (not fixed this pass — explicitly out of the scoped fix set; the finding itself states "No action required for this phase"). The collision requires a real guildie or `--owner` argument named exactly "guild" (no real guildie is so named) and the code paths (`linking.go`, `eviction.go` label bridge, `auth/store.go`) are pre-existing/out of phase scope. Left untouched; tracked for a future hardening pass (exclude `o.id <> store.GuildSentinelOwnerID` / `label <> 'guild'` from the label-bridge `SELECT`s).

**File:** `internal/backendsrv/store/linking.go:98`, `internal/backendsrv/webadmin/eviction.go:391`, `internal/backendsrv/auth/store.go:38`

**Issue:** Several owner lookups resolve by label (`WHERE TRIM(label) = TRIM(?) COLLATE NOCASE` / `WHERE label = ?`). This phase introduced an owner row whose label is the literal `'guild'`. If a guildie's Discord username (or a mint `--owner` argument) were ever exactly "guild", these bridges could resolve to the sentinel owner. The sentinel has `discord_user_id = NULL` (unlinked), so `linking.go` would treat it as an unlinked label match and could adopt/return it. This is extraordinarily unlikely (no real guildie is named "guild") and the code paths are pre-existing/out of phase scope, but the new reserved label makes the collision newly reachable.

**Fix:** No action required for this phase. If hardening later: exclude `o.id <> store.GuildSentinelOwnerID` (or `label <> 'guild'`) from the label-bridge `SELECT`s in `linking.go`/`eviction.go`/`auth/store.go`, so the reserved sentinel can never be resolved as a guildie via the textual bridge.

---

_Reviewed: 2026-06-22_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
