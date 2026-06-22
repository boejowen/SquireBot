---
phase: 36-shared-character-safe-eviction
plan: 01
subsystem: api
tags: [sqlite, eviction, audit-log, cross-owner-write, go, tdd]

# Dependency graph
requires:
  - phase: 35-owner-less-guild-banks-bots
    provides: "GuildSentinelOwnerID + the ErrCannotEvictSentinel write-path guard (banks/bots are eviction-safe by construction; this plan does not re-litigate bank survival)"
  - phase: 11-backend (binding.go / 260621-u6j)
    provides: "the audit_log cross_owner_write trail (auditCrossOwnerWrite) — the SHARING signal this plan is the first reader of"
provides:
  - "sharedCharPredicate const — the single source of truth for 'this char is shared by another guildie' (a cross_owner_write row with attempting_owner_id <> the evicted owner, COLLATE NOCASE); reused VERBATIM by the cascade, the preview, and the count"
  - "Narrowed EvictOwnerTx cascade: shared chars survive (is_removed stays 0, grace_until NULL); sole-owned chars still removed; code revoke unconditional on removedCount"
  - "CountPreservedShared(ctx, db, ownerID) — the live shared-survivor count (inverse complement of the remove-set, off the same predicate)"
  - "recentOtherSharerSubquery const + the surviving-shared repoint (owner_id moves off the evicted owner to the most-recent remaining sharer, NULL-guarded)"
  - "EvictionPreviewHandler emits the additive snake_case preserved_shared_count int field (36-02 mirrors it field-for-field to keep an all-shared owner evictable)"
affects: [36-02-web-eviction-form, v2.5-milestone-close]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Single-const SQL predicate as source of truth (the Phase-35 CR-01 lesson): one Go const spliced VERBATIM into the cascade + preview + count so the officer preview can never claim a different remove-set than the action removes"
    - "A SECOND named const (recentOtherSharerSubquery) reused in BOTH a repoint UPDATE's SET clause and its IS-NOT-NULL guard so the two copies can never drift; locked by a token-equality test (TestRepointSubquery_LocksPredicateToSharedPredicate) + a single-const grep gate"
    - "Detection-on-read: 'shared' is derived live from audit_log + character — NO new table, NO migration (schema stays v15)"

key-files:
  created: []
  modified:
    - "internal/backendsrv/store/eviction.go — sharedCharPredicate + recentOtherSharerSubquery consts; narrowed EvictOwnerTx cascade + surviving-shared repoint; narrowed PreviewEviction; new CountPreservedShared"
    - "internal/backendsrv/store/eviction_test.go — 6 new tests + 3 local helpers (insertCrossOwnerWrite, ownerCodeDisabled, removedCharNames, ownerOfChar)"
    - "internal/backendsrv/webadmin/eviction.go — EvictionPreviewHandler emits the additive preserved_shared_count field (two store reads now)"

key-decisions:
  - "owner_id is a non-binding steward marker (binding.go / 260621-u6j), so the repoint is clean-data polish — the HARD survival guarantee is met by the narrowed cascade alone; an unresolvable repoint target leaves owner_id on the evicted owner (cosmetic fallback), never writes NULL into the NOT-NULL column"
  - "preserved_shared_count is additive + snake_case and crosses the API boundary; 36-02 mirrors it to keep an all-shared owner (characters:[] + preserved_shared_count>0) evictable for a code-only revoke"
  - "No migration (D-05): detection is computed on read; schema stays v15; watcher untouched; no v* tag"

patterns-established:
  - "Single-const SQL predicate spliced VERBATIM into every consumer (cascade/preview/count) — the CR-01 divergence is structurally impossible"
  - "A drift-locking token-equality test pins load-bearing SQL tokens identical across two related consts"

requirements-completed: [OWN-03]

# Metrics
duration: 9min
completed: 2026-06-22
---

# Phase 36 Plan 01: Shared-Character-Safe Eviction (Backend) Summary

**Narrowed the per-owner eviction cascade so evicting a guildie removes only their OWN characters — a shared char another guildie uploads (a `cross_owner_write` audit row, attempting_owner_id <> the evicted owner) survives — backed by a single `sharedCharPredicate` const shared by the cascade, preview, and a new `CountPreservedShared`, plus a drift-locked surviving-shared repoint and an additive `preserved_shared_count` preview field (OWN-03).**

## Performance

- **Duration:** ~9 min
- **Started:** 2026-06-22T20:55:37Z
- **Completed:** 2026-06-22T21:04:29Z
- **Tasks:** 3 (all TDD)
- **Files modified:** 3

## Accomplishments
- **OWN-03 met:** `EvictOwnerTx(X)` now flips `is_removed=1`/`grace_until` ONLY on X's live, NON-shared chars via `AND NOT sharedCharPredicate`; a shared char stays `is_removed=0` / `grace_until` NULL (survives). Sole-owned chars are still removed + grace-stamped exactly as before.
- **All-shared owner stays revocable (D-04):** an owner whose every live char is shared flips 0 chars (`removedCount=0`) but still has their `guild_code` revoked — the revoke is unconditional on `removedCount`, and `ListEvictableOwners` is NOT narrowed.
- **Preview/cascade parity (CR-01 lesson):** `PreviewEviction` and `CountPreservedShared` embed the SAME `sharedCharPredicate` const as the cascade, so `len(PreviewEviction) + CountPreservedShared == the owner's live-char count` and the officer confirm UI can never disagree with the action.
- **Additive contract for 36-02:** the eviction-preview JSON gains `preserved_shared_count` (int, snake_case) — an all-shared owner sends `characters:[]` + `preserved_shared_count>0` so the web Evict button stays enabled (code-only revoke; the BLOCKER fix).
- **Surviving-shared repoint (D-03), drift-locked:** a surviving shared char still stewarded by the evicted owner is repointed to the most-recent remaining sharer via `recentOtherSharerSubquery`, reused VERBATIM in both the SET clause and the IS-NOT-NULL guard; NULL-guarded so an unresolvable target leaves owner_id on X (still surviving).

## Task Commits

Each task was committed atomically (all TDD — RED then GREEN in one commit per task, with the new tests written failing first):

1. **Task 1: Define `sharedCharPredicate` + narrow the `EvictOwnerTx` cascade** — `652f6d2` (feat)
2. **Task 2: Lock `PreviewEviction` to the same predicate + add `CountPreservedShared` + emit `preserved_shared_count`** — `6969080` (feat)
3. **Task 3: Repoint a surviving shared char (single named subquery const, drift-locked) + full regression sweep** — `a1d0861` (feat)

_Plan metadata commit (SUMMARY/STATE/ROADMAP) is separate._

## Files Created/Modified
- `internal/backendsrv/store/eviction.go` — added `sharedCharPredicate` + `recentOtherSharerSubquery` package consts; narrowed the `EvictOwnerTx` cascade; added the surviving-shared repoint UPDATE (after the cascade, before the code revoke, same tx); narrowed `PreviewEviction`; added `CountPreservedShared`.
- `internal/backendsrv/store/eviction_test.go` — 6 new tests + 4 local helpers (`insertCrossOwnerWrite`, `ownerCodeDisabled`, `removedCharNames`, `ownerOfChar`).
- `internal/backendsrv/webadmin/eviction.go` — `EvictionPreviewHandler` now makes a second store read (`CountPreservedShared`) and emits the additive `preserved_shared_count` field.

### Tests added
- `TestEvictOwnerTx_SharedCharSurvives` (the OWN-03 proof)
- `TestEvictOwnerTx_AllSharedOwnerStillRevokesCode` (D-04 edge: removedCount 0 but code revoked)
- `TestEvictOwnerTx_SelfAttemptingRowIsNotShared` (the `<> X` guard: a self-attributed row is not sharing)
- `TestPreviewEviction_OmitsSharedChars` (the parity proof: preview-set == cascade remove-set)
- `TestCountPreservedShared_CountsSurvivors` (mixed / all-shared / sole-owned cases)
- `TestEvictOwnerTx_RepointsSurvivingSharedChar` + `TestEvictOwnerTx_RepointPicksMostRecentSharer` + `TestRepointSubquery_LocksPredicateToSharedPredicate` (D-03 repoint + the drift lock)

The existing Phase-35 regression tests stay green: `TestEvictOwnerTx_CascadesAndRevokesCodeInOneTx`, `TestEvictOwnerTx_GuildBankSurvivesEviction`, `TestEvictOwnerTx_RefusesSentinel`, `TestRestoreOwnerTx_RefusesSentinel`, `TestListEvictableOwners_*`, `TestRestoreOwnerTx_*`.

## Decisions Made
None beyond the plan — executed exactly as written. (The key locked decisions from CONTEXT D-01..D-06 are carried in frontmatter `key-decisions`.)

## Deviations from Plan

None - plan executed exactly as written. 0 auto-fixes (Rules 1–4 not triggered); no architectural changes; no authentication gates. The pre-existing `codeDisabled` helper in `linking_test.go` (label-keyed) forced a local rename of my owner-keyed helper to `ownerCodeDisabled` — this is a naming choice inside the new test code, not a behavior deviation.

## Issues Encountered
- A name collision: the planned `codeDisabled(t, ctx, db, ownerID int64)` test helper clashed with an existing `codeDisabled(t, ctx, db, label string)` in `linking_test.go` (same package). Resolved by naming the new owner-keyed helper `ownerCodeDisabled`. Caught immediately by the first RED compile; no behavior impact.

## Verification (gates all green)
- `go build ./...` — rc=0 (whole module compiles)
- `go test ./internal/backendsrv/...` — all 18 packages green (store, webadmin, migrations, ec, notify, wantmatch, readapi, scheduler, …)
- `go vet ./internal/backendsrv/store/... ./internal/backendsrv/webadmin/...` — clean
- Single-source-of-truth grep gates (comment-stripped): exactly 1 `const sharedCharPredicate` AND exactly 1 `const recentOtherSharerSubquery`
- `preserved_shared_count` present in `webadmin/eviction.go` (4 occurrences)
- NO new migration — head stays `00015_guild_owner.sql` (schema v15, D-05)
- Watcher tree UNTOUCHED across all 3 task commits (`internal/app/`, `internal/eqfind/`, `internal/sheet/`, `cmd/squirebot/` — empty diff)
- Web tree UNTOUCHED (`web/` empty diff — that's wave 2, plan 36-02)
- NO `v*` git tag created (latest stays `v2.1.2`)

## Known Stubs
None. The repoint's NULL-fallback (leave owner_id on the evicted owner when no other sharer resolves) is an intentional, documented cosmetic fallback per D-03 — not a stub; the char still survives.

## Flag for 36-02 (web, wave 2)
The eviction-preview JSON now carries an additive field:
```
preserved_shared_count: int   // count of the owner's live chars that SURVIVE because they are shared
```
36-02 must mirror this field-for-field (snake_case) in `web/src/lib/api.ts`'s `EvictionPreview` interface and use it in `EvictionForm.svelte` so an ALL-SHARED owner (`characters:[]` + `preserved_shared_count > 0`) stays evictable (code-only revoke) with the "0 characters removed; {N} shared characters preserved; guild code will be revoked" framing (D-06). Keep the genuine "owner has zero live chars at all" case disabled. Browser-smoke the eviction flow on prod after deploy (officer-auth required; node vitest is DOM-blind).

## Next Phase Readiness
- Backend half of OWN-03 is COMPLETE and committed on master (3 task commits). No deploy yet (no migration; the backend binary swaps in when 36-02's deploy step runs, alongside the web atomic-swap).
- 36-02 (web, wave 2) is unblocked — it depends only on the `preserved_shared_count` contract this plan ships.
- After 36-01 (backend) + 36-02 (web) land + the web change deploys, v2.5 "Ownership Cleanup" is feature-complete (OWN-01/02/04 from Phase 35; OWN-03 here) and ready for milestone audit/close. No watcher change → no `v*` tag for v2.5.

## Self-Check: PASSED

- All 3 modified source files + the SUMMARY exist on disk.
- All 3 task commit hashes (`652f6d2`, `6969080`, `a1d0861`) exist in git history.

---
*Phase: 36-shared-character-safe-eviction*
*Completed: 2026-06-22*
