---
phase: 35-owner-less-guild-banks-bots
fixed_at: 2026-06-22
review_path: .planning/phases/35-owner-less-guild-banks-bots/35-REVIEW.md
iteration: 1
findings_in_scope: 4
fixed: 4
skipped: 0
deferred: 1
status: all_fixed
---

# Phase 35: Code Review Fix Report

**Fixed at:** 2026-06-22
**Source review:** `.planning/phases/35-owner-less-guild-banks-bots/35-REVIEW.md`
**Iteration:** 1

**Summary:**
- Findings in scope (this pass): 4 (CR-01, WR-01, WR-02, IN-01)
- Fixed: 4
- Skipped: 0
- Deferred to backlog (out of scope, by instruction): 1 (IN-02)

All four scoped findings are resolved. IN-02 (label-bridge `'guild'`-label collision)
was explicitly EXCLUDED from this fix set and left untouched — the review itself
classifies it "No action required for this phase"; it is tracked for a future
hardening pass.

## Verification (all green)

Run from `internal/backendsrv`:

- `go build ./...` → clean
- `go vet ./store/... ./migrations/... ./webadmin/...` → clean
- `go test ./store/... ./migrations/... ./webadmin/... -run "Evict|Restore|Migrate_00015|GuildSentinel|GuildBank|Sentinel"` → ok (store / migrations / webadmin)
- `go test ./...` (full backend module) → ok, every package passing, nothing regressed

## Fixed Issues

### CR-01 (BLOCKER): Sentinel evictable via the directly-POST-able write path

**Commit:** `11ebcc6` (`fix(35-01): ...`)
**Files modified:** `internal/backendsrv/store/eviction.go`, `internal/backendsrv/webadmin/eviction.go`, `internal/backendsrv/store/eviction_test.go`, `internal/backendsrv/webadmin/eviction_test.go`

**Applied fix:**
- Added typed `store.ErrCannotEvictSentinel`.
- Guarded the WRITE path at the top of `EvictOwnerTx` (`return 0, 0, ErrCannotEvictSentinel`) and `RestoreOwnerTx` (`return 0, ErrCannotEvictSentinel`) when `ownerID == GuildSentinelOwnerID`, before any grace compute / UPDATE — single source of truth, covers every caller. Each carries the OWN-02 comment.
- Mapped the error in `mapEvictionErr` → 403 `cannot_evict_sentinel` (mirrors the `not_authorized` guard's 403 status convention; both `EvictHandler` and `RestoreHandler` already route store errors through this mapper).
- Tests: `TestEvictOwnerTx_RefusesSentinel`, `TestRestoreOwnerTx_RefusesSentinel` (store), `TestEvict_RefusesGuildSentinel` (webadmin — POSTs `owner_id=1000000` as the bootstrap officer, asserts 403 + `cannot_evict_sentinel` + bank `is_removed=0` + zero `eviction` audit rows).

### WR-01: Migration 00015 backfill never exercised against pre-existing data

**Commit:** `62acc78` (`test(35-01): ...`)
**Files modified:** `internal/backendsrv/migrations/migrate_test.go`

**Applied fix:** Added `TestMigrate_00015_BackfillRunsOverPreExistingData`, mirroring the
existing `TestMigrate_00011` part-(c) version-pinned pattern (`openAtVersion(t, 14)` +
`migrations.UpTo(raw, 15)`). It seeds an owner-bound bank + bot + a normal char at v14,
then drives the ACTUAL embedded `00015_guild_owner.sql` backfill (not a hand-copied SQL
string) over non-empty data, asserting the bank/bot repoint to the sentinel and the
normal char's `owner_id` is unchanged. The prior hand-copied-SQL test was kept for its
idempotency + second-`RunMigrations` no-op coverage (no coverage lost).

### WR-02: Irreversible `Up` with a no-op `Down` (doc-only)

**Commit:** `96ea4ad` (`docs(35-01): ...`)
**Files modified:** `internal/backendsrv/migrations/00015_guild_owner.sql`

**Applied fix:** Added an IRREVERSIBLE note to the Up header + a Down-block
clarification: the backfill overwrites `character.owner_id` irreversibly and the
pre-backfill first-uploader binding is NOT recoverable via `goose down` (the project's
forward-only `SELECT 1;` no-op) — recovery is "restore from the R2 backup taken before
the 00015 deploy". The `Down` block stays `SELECT 1;` (no SQL behavior change).

### IN-01: Sentinel id duplicated in the migration test

**Commit:** `62acc78` (folded into the WR-01 commit — same file, same finding cluster)
**Files modified:** `internal/backendsrv/migrations/migrate_test.go`

**Applied fix:** Replaced `const guildSentinelOwnerID = 1000000` with a binding to
`store.GuildSentinelOwnerID` (the test is in the external `migrations_test` package,
which already imports `store`), collapsing three drifting literals to two — only the
unavoidable `.sql` literal remains an independent copy. Added
`TestGuildSentinelOwnerID_MatchesContract` asserting `store.GuildSentinelOwnerID == 1000000`
so an accidental edit to the const is caught against the documented contract.

## Deferred (not fixed — by instruction)

### IN-02: Label-bridge lookups can match the sentinel's `'guild'` label

**Reason:** Explicitly excluded from the scoped fix set; the review classifies it "No
action required for this phase." The collision is extraordinarily unlikely (requires a
guildie or `--owner` argument named exactly "guild") and the affected paths
(`store/linking.go`, `webadmin/eviction.go` label bridge, `auth/store.go`) are
pre-existing / out of phase scope. Left untouched per the do-not list. Tracked for a
future hardening pass (exclude `o.id <> store.GuildSentinelOwnerID` / `label <> 'guild'`
from the label-bridge `SELECT`s).

---

_Fixed: 2026-06-22_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
