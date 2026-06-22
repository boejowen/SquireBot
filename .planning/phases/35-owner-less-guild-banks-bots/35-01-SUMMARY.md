---
phase: 35-owner-less-guild-banks-bots
plan: 01
subsystem: database
tags: [sqlite, goose, migration, eviction, character-ownership, guild-bank]

# Dependency graph
requires:
  - phase: 26-character-assignment
    provides: "DesignateCharTx (the officer-only bank/bot designator) + character.is_guild_bot + the character_assignment/assignment_request tables"
  - phase: 11-backend-foundation
    provides: "character.owner_id (NOT NULL REFERENCES owner(id)) + the per-owner EvictOwnerTx cascade + goose migration runner"
provides:
  - "A reserved 'guild' sentinel owner row (id 1000000, label 'guild') seeded by migration 00015 that holds owner-less (guild-held) banks/bots"
  - "store.GuildSentinelOwnerID — the single Go-side source of truth for the sentinel id"
  - "DesignateCharTx now repoints a bank/bot's owner_id to the sentinel (decouples designation from upload-ownership — no claim; survives the first uploader's eviction)"
  - "A one-time backfill repointing every pre-existing is_bank_toon/is_guild_bot char to the sentinel (OWN-04, no manual fixup)"
  - "ListEvictableOwners / ListRestorableOwners exclude the sentinel so an officer can never evict the whole guild bank"
affects: [36-shared-character-safe-eviction]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Reserved-sentinel-owner (Option A): a real owner row at a fixed id far above the organic autoincrement range, keeping the NOT-NULL FK satisfied while making banks/bots owner-less"
    - "INSERT OR IGNORE PK seed + idempotent guarded backfill (owner_id <> sentinel) in a forward-only goose migration (00009 replay-safe-seed precedent)"

key-files:
  created:
    - internal/backendsrv/migrations/00015_guild_owner.sql
    - internal/backendsrv/store/owner.go
    - internal/backendsrv/store/owner_test.go
  modified:
    - internal/backendsrv/store/assignment.go
    - internal/backendsrv/store/eviction.go
    - internal/backendsrv/store/eviction_test.go
    - internal/backendsrv/migrations/migrate_test.go

key-decisions:
  - "Option A (reserved 'guild' sentinel owner id 1000000) over Option B (nullable owner_id) — smallest blast radius: 1 additive migration + 1 constant + 1 DesignateCharTx line + 2 eviction-list exclusions; Option B would need a 12-table NOT-NULL-drop rebuild + every owner_id consumer learning NULL"
  - "Sentinel id = the literal 1000000 in BOTH the migration .sql and store.GuildSentinelOwnerID (they must match) — far above the ~12-owner working range so it can never collide with an organic INSERT"
  - "Clearing a designation (DesignateNeither) does NOT re-home owner_id — a former bank stays sentinel-owned (out of scope to re-home; harmless for a trusted guild)"
  - "EvictOwnerTx/RestoreOwnerTx/PreviewEviction cascade left UNCHANGED — the sentinel id is simply never passed; Phase 36 (OWN-03) refines the cascade for shared NON-bank chars"

patterns-established:
  - "Guild-held resource = sentinel-owned: a single reserved owner id that no eviction targets makes banks/bots eviction-proof by construction"

requirements-completed: [OWN-01, OWN-02, OWN-04]

# Metrics
duration: 10min
completed: 2026-06-22
---

# Phase 35 Plan 01: Owner-Less Guild Banks & Bots Summary

**A reserved 'guild' sentinel owner (id 1000000, schema v15 via goose migration 00015) makes designated banks/bots GUILD-HELD — DesignateCharTx repoints owner_id to it, a backfill migrates existing banks, and the eviction owner-lists exclude it so a guild bank survives any individual guildie's eviction (OWN-01/02/04).**

## Performance

- **Duration:** ~10 min
- **Started:** 2026-06-22T19:24:16Z
- **Completed:** 2026-06-22T19:34:36Z
- **Tasks:** 3
- **Files modified:** 7 (3 created, 4 modified)

## Accomplishments
- **Migration 00015 (schema v15)** seeds the reserved sentinel owner (id 1000000, label `guild`) via `INSERT OR IGNORE` and backfills every existing `is_bank_toon=1 OR is_guild_bot=1` char's `owner_id` to it — extend-only, forward-only (goose Down = `SELECT 1;`), idempotent via the `owner_id <> 1000000` guard (OWN-04).
- **`store.GuildSentinelOwnerID`** (`const … = 1000000`) — the single Go-side source of truth, matching the migration literal.
- **`DesignateCharTx`** now repoints `owner_id` to the sentinel inside the SAME officer-gated tx when designating bank/bot, so the char is guild-held and survives its first uploader's eviction (OWN-01/02). `DesignateNeither` leaves `owner_id` untouched.
- **`ListEvictableOwners` / `ListRestorableOwners`** exclude the sentinel (`o.id <> ?`), so an officer can never pick "evict the guild bank" (OWN-02).
- **The OWN-02 proof** (`TestEvictOwnerTx_GuildBankSurvivesEviction`): evicting a real guildie flips `is_removed=1` only on their own chars; a sentinel-owned bank stays `is_removed=0` / `grace_until` NULL.

## Task Commits

Each task was committed atomically:

1. **Task 1: Migration 00015 — seed sentinel + backfill existing bank/bot chars** — `7a238b8` (feat)
2. **Task 2: store/owner.go sentinel constant + repoint owner_id in DesignateCharTx** — `c8305c2` (feat)
3. **Task 3: exclude the sentinel from eviction owner-lists + survives-eviction proof** — `4d38389` (feat)

_Note: the plan marked all tasks `tdd=true`; since project `tdd_mode` is off, each task shipped its production change + its tests in one atomic commit (the migration + test, the constant/repoint + tests, the exclusions + tests) rather than split RED/GREEN commits._

## Files Created/Modified
- `internal/backendsrv/migrations/00015_guild_owner.sql` (created) — sentinel seed + bank/bot owner_id backfill.
- `internal/backendsrv/store/owner.go` (created) — `GuildSentinelOwnerID` constant + the doc explaining the guild-held sentinel model.
- `internal/backendsrv/store/owner_test.go` (created) — bank/bot repoint, neither-no-repoint, non-officer-no-write tests.
- `internal/backendsrv/store/assignment.go` (modified) — `DesignateCharTx` owner_id repoint (bank/bot only) + updated doc comment.
- `internal/backendsrv/store/eviction.go` (modified) — `o.id <> ?` sentinel exclusion in both owner-list queries + OWN-02 doc comments.
- `internal/backendsrv/store/eviction_test.go` (modified) — 3 new tests (evictable-excludes, restorable-excludes, survives-eviction proof).
- `internal/backendsrv/migrations/migrate_test.go` (modified) — `TestMigrate_00015_SeedsGuildOwnerAndBackfillsBanks` + the `backfillBankOwnerSQL`/`guildSentinelOwnerID`/`ownerIDOfChar` test helpers.

## Tests Added
- `TestMigrate_00015_SeedsGuildOwnerAndBackfillsBanks` — sentinel seeded (label 'guild'); owner-bound Findom (bank) + Botchar (bot) repoint to the sentinel; Normalchar untouched; backfill + RunMigrations idempotent.
- `TestDesignateCharTx_BankRepointsOwnerToSentinel` — bank designation → owner_id == sentinel, is_bank_toon=1, prior assignment cleared.
- `TestDesignateCharTx_BotRepointsOwnerToSentinel` — bot designation → owner_id == sentinel, is_guild_bot=1.
- `TestDesignateCharTx_NeitherDoesNotRepoint` — clearing a designation leaves owner_id at the sentinel (no re-home).
- `TestDesignateCharTx_NonOfficerNoRepoint` — non-officer → ErrNotAuthorized, owner_id unchanged (no write).
- `TestListEvictableOwners_ExcludesGuildSentinel` / `TestListRestorableOwners_ExcludesGuildSentinel` — the sentinel never appears in either picker.
- `TestEvictOwnerTx_GuildBankSurvivesEviction` — the OWN-02 proof.

## Decisions Made
- **Option A (reserved sentinel owner)** over Option B (nullable owner_id) — already resolved in the plan; confirmed in execution as the smallest-blast-radius change (the only two production `character.owner_id` consumers — binding.go + eviction.go — are exactly the seams touched).
- **Sentinel id = literal 1000000** kept in lockstep between `00015_guild_owner.sql` and `store.GuildSentinelOwnerID`.
- **Cascade left untouched** — `EvictOwnerTx`/`RestoreOwnerTx`/`PreviewEviction` are already `WHERE owner_id = ?`; the sentinel id is simply never passed (no picker offers it). This is the seam Phase 36/OWN-03 will refine for shared NON-bank chars.

## Deviations from Plan

None - plan executed exactly as written. Every file, SQL statement, Go edit, and test name matches the plan's task specifications.

## Issues Encountered
None. All three task verifications passed first try; the full `go build ./...` (rc=0), `go vet ./internal/backendsrv/{store,migrations}/...` (clean), and `go test ./internal/backendsrv/...` (all 18 packages green) confirm no regressions.

## Watcher + Tags Confirmation
- **Watcher tree UNTOUCHED:** `git status --short internal/app/ internal/eqfind/ internal/sheet/ cmd/squirebot/` is empty; the only changed files this session are under `internal/backendsrv/`.
- **NO `v*` git tag created** (a `v*` tag would fire the watcher release CI; the watcher is unchanged). Latest tag remains `v2.1.2`.

## User Setup Required
None - no external service configuration required. Migration 00015 applies automatically on backend boot (the deploy of this migration to prod is a future step — this plan is code + schema only, no deploy).

## Next Phase Readiness
- The sentinel-owner model is in place: every designated guild bank/bot now lives under one owner id that no eviction targets, so banks are eviction-safe by construction.
- **Phase 36 (OWN-03)** can focus purely on shared NON-bank characters (a char multiple guildies play should not be over-deleted when one is evicted) without re-litigating bank survival; the `WHERE owner_id = ?` cascade in `EvictOwnerTx` is the seam it will refine, and it already correctly skips the sentinel.

## Self-Check: PASSED

- Created files exist: 00015_guild_owner.sql, store/owner.go, store/owner_test.go, 35-01-SUMMARY.md (all FOUND).
- Task commits exist: 7a238b8, c8305c2, 4d38389 (all FOUND).

---
*Phase: 35-owner-less-guild-banks-bots*
*Completed: 2026-06-22*
