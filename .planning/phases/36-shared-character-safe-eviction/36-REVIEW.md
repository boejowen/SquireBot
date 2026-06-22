---
phase: 36-shared-character-safe-eviction
reviewed: 2026-06-22T21:23:01Z
depth: standard
files_reviewed: 7
files_reviewed_list:
  - internal/backendsrv/store/eviction.go
  - internal/backendsrv/store/eviction_test.go
  - internal/backendsrv/webadmin/eviction.go
  - web/src/lib/api.ts
  - web/src/lib/eviction.ts
  - web/src/lib/components/EvictionForm.svelte
  - web/src/lib/__tests__/eviction.test.ts
findings:
  critical: 0
  warning: 2
  info: 2
  total: 4
status: issues_found
---

# Phase 36: Code Review Report

**Reviewed:** 2026-06-22T21:23:01Z
**Depth:** standard
**Files Reviewed:** 7
**Status:** issues_found

## Summary

Phase 36 (OWN-03) narrows the per-owner eviction cascade so SHARED characters (a
`cross_owner_write` audit row from a second guildie) survive when their first-uploader
is evicted. This is data-DELETION logic, so the review centered on the shared-detection
predicate, preview/cascade parity, the preserved-shared count, the D-03 steward repoint,
the Phase-35 sentinel guards, parameterized SQL, and the web Evict gating.

**The core correctness claims hold and are well-defended.** I traced every concern in
the review brief and verified it against the source and tests:

1. **Shared predicate direction is CORRECT.** The cascade flips `is_removed=1` only on
   `owner_id=X AND is_removed=0 AND NOT EXISTS(cross_owner_write with attempting_owner_id <> X)`.
   A sole-owned char (no audit row, or only self-attributed rows) is removed; a char another
   guildie uploaded is preserved. `TestEvictOwnerTx_SharedCharSurvives`,
   `TestEvictOwnerTx_SelfAttemptingRowIsNotShared`, and the unchanged
   `TestEvictOwnerTx_CascadesAndRevokesCodeInOneTx` pin both directions. The `<> X` guard
   correctly treats a self-write as NOT-shared (verified against `binding.go`
   `auditCrossOwnerWrite`, which writes `attempting_owner_id = tokenOwnerID`).

2. **COLLATE NOCASE join is CORRECT.** `audit_log.char_name` is plain `TEXT` (00002),
   `character.name` is `UNIQUE COLLATE NOCASE` (00001). The predicate forces
   `a.char_name = character.name COLLATE NOCASE`, so a case-drifted upload name still matches.

3. **D-04 preview parity is GENUINELY single-source.** `EvictOwnerTx`, `PreviewEviction`,
   and `CountPreservedShared` all splice the *same* `sharedCharPredicate` const (verbatim
   string concat, no hand-inlined copy). `TestPreviewEviction_OmitsSharedChars` proves the
   preview list is byte-identical to the cascade's actual `is_removed=1` set. This is the
   inverse of the Phase-35 CR-01 picker/write divergence — closed.

4. **`preserved_shared_count` is accurate.** It is the exact inverse-complement of the
   preview remove-set off the same predicate (`len(PreviewEviction) + CountPreservedShared
   == live-char count`), proven by `TestCountPreservedShared_CountsSurvivors` across mixed /
   all-shared / sole-owned cases. No off-by-one or double-count.

5. **D-03 repoint NULL/FK safety holds.** The repoint sets `owner_id` only when
   `recentOtherSharerSubquery IS NOT NULL`, so it never writes NULL into the NOT-NULL column.
   The repointed value is an `attempting_owner_id` that always came from a valid guild code's
   `tokenOwnerID` (`binding.go`), and owners are never hard-deleted (no `DELETE FROM owner`
   anywhere), so the `character.owner_id REFERENCES owner(id)` FK is always satisfiable. The
   SET clause and the guard share one const, drift-locked by
   `TestRepointSubquery_LocksPredicateToSharedPredicate`.

6. **Phase-35 regression intact.** The `ownerID == GuildSentinelOwnerID` guard is the FIRST
   statement of both `EvictOwnerTx` and `RestoreOwnerTx`; `ListEvictableOwners`/
   `ListRestorableOwners` keep the `o.id <> ?` sentinel exclusion. The repoint's
   `WHERE owner_id = ?` can never match a sentinel-owned bank/bot (the target is always a real
   owner, and the sentinel guard already rejected the sentinel id). Five sentinel/bank-survival
   tests still pass.

7. **Parameterized SQL only (V5).** Both new consts use positional `?` for the only bound
   value (the evicted owner id); the event string `'cross_owner_write'` is a literal, not
   interpolated. No string-built values anywhere.

8. **Web gating is correct and pure.** `canEvictPreview` keeps the all-shared owner
   (`characters:[]` + `preserved_shared_count>0`) ENABLED for the code-only revoke and a
   genuine zero-live-chars owner DISABLED. The gate and the render branch both derive from the
   pure `evictPreviewSummary`, so they cannot drift. No `{@html}` sink (asserted by test). The
   `preserved_shared_count` snake_case field mirrors the Go reply.

All Go store + webadmin eviction tests pass (`go test ./store/... ./webadmin/...`), `go vet`
is clean, and the 17 web vitest cases pass.

The findings below are quality/robustness improvements, not correctness blockers. None gate the
deploy.

## Warnings

### WR-01: Eviction PREVIEW does not enforce floor-protection or sentinel-rejection that the EVICT action does

**File:** `internal/backendsrv/webadmin/eviction.go:99-134` (`EvictionPreviewHandler`)
**Issue:** The preview handler is officer-gated at the route but, unlike `EvictHandler`,
performs no `callerMayNotEvictFloor` check and no sentinel rejection. A peer officer can
preview the maintainer's floor-protected character list (and pass `owner_id=1000000` to enumerate
the guild bank's chars). This is read-only — the *destructive* path correctly re-checks floor +
sentinel — so there is no data-loss or over-deletion risk. But it is an information-disclosure
asymmetry: the officer sees a per-character remove-set they are then refused at commit
(`owner_floor_protected` / `cannot_evict_sentinel`), and the preview leaks the floor owner's
roster to a peer. Worth a deliberate decision before deploy: either accept it (officers are
already trusted to see the roster via other reads) or short-circuit the preview for a
floor/sentinel target so the UI never renders a remove-set the action will reject.
**Fix:** If the leak matters, mirror the action's guards in the preview (return an empty
`characters` + `preserved_shared_count:0`, or a 403, for a floor-protected/sentinel target):
```go
// after parseOwnerIDQuery, before the store reads:
if ownerID == store.GuildSentinelOwnerID {
    writeJSONError(w, http.StatusForbidden, "cannot_evict_sentinel")
    return
}
protected, err := callerMayNotEvictFloor(ctx, db, ownerID, caller(ctx))
if err != nil { /* 500 */ }
if protected {
    writeJSONError(w, http.StatusForbidden, "owner_floor_protected")
    return
}
```
If the asymmetry is accepted, add a one-line comment to `EvictionPreviewHandler` stating the
preview is intentionally unguarded (read-only) so a future reader does not mistake it for an
oversight.

### WR-02: D-03 repoint can hand a surviving shared char to an already-evicted (in-grace) steward

**File:** `internal/backendsrv/store/eviction.go:66-82, 300-309` (`recentOtherSharerSubquery` + repoint UPDATE)
**Issue:** The repoint picks the most-recent `attempting_owner_id <> X` from the audit trail,
with no filter excluding owners who are themselves currently evicted (in-grace) or whose chars
are all removed. Scenario: guildies X and Y both upload Sharedtoon; Y is evicted first (Y's own
chars go to grace, Y's code is revoked), then X is evicted. The repoint moves Sharedtoon's
`owner_id` to Y — a steward who has been evicted and can no longer upload. The char still
SURVIVES (`is_removed=0`, the hard guarantee is unaffected by `owner_id`, which is a non-binding
marker per `binding.go`), so this is not data loss. But the surviving shared char is now
attributed to a departed steward, which is exactly the "clean-data" outcome D-03 was added to
avoid, and it can recur (a later eviction of Y would re-evaluate Sharedtoon's sharing afresh, so
the char is not permanently endangered, but the attribution is stale). The plan documents the
no-resolvable-target fallback (leave `owner_id=X`) but does not address an evicted resolvable
target.
**Fix:** Prefer a remaining sharer who still has a live, non-removed character (i.e. is not
fully evicted). One option — restrict the subselect to attempting owners that still steward at
least one live char:
```sql
(
  SELECT a.attempting_owner_id FROM audit_log a
   WHERE a.event = 'cross_owner_write'
     AND a.char_name = character.name COLLATE NOCASE
     AND a.attempting_owner_id <> ?
     AND EXISTS (SELECT 1 FROM character c2
                  WHERE c2.owner_id = a.attempting_owner_id AND c2.is_removed = 0)
   ORDER BY a.id DESC LIMIT 1
)
```
Note this would diverge `recentOtherSharerSubquery` from `sharedCharPredicate`, so the
drift-lock test (`TestRepointSubquery_LocksPredicateToSharedPredicate`) would need its token set
re-scoped to the still-shared tokens only. Given `owner_id` is cosmetic and survival is
guaranteed, deferring this with a tracked note is also defensible — but the current code has no
comment acknowledging the evicted-steward case, only the no-target case; add one if deferring.

## Info

### IN-01: Repoint runs even for owners with zero surviving shared chars (no-op write)

**File:** `internal/backendsrv/store/eviction.go:300-309`
**Issue:** The repoint UPDATE executes unconditionally for every eviction, including the common
case (a sole-owned owner) where it matches zero rows because no `is_removed=0` char with a
resolvable other sharer remains. This is a harmless extra statement in the tx (performance is
out of v1 scope and SQLite handles it trivially), but it is a small amount of work on the hot
path of every eviction. No fix required; noted for awareness.
**Fix:** None needed. (If ever desired, gate the repoint on `CountPreservedShared > 0`, but that
adds a read and the current unconditional form is simpler and correct.)

### IN-02: `evictPreviewSummary` and `canEvictPreview` independently re-derive the same two-field logic

**File:** `web/src/lib/eviction.ts:59-86`
**Issue:** `canEvictPreview` (`characters.length>0 || preserved_shared_count>0`) and
`evictPreviewSummary` (cascade / code-only / empty) read the same two fields with the same
thresholds. They are consistent today and both node-tested, but they encode the gate twice. A
future change to one (e.g. a new "code-only only when count >= N" rule) could silently desync
the button-enable from the render branch. Low risk because both are pure and covered, but a
single source would be more robust.
**Fix:** Optionally derive the boolean from the summary so there is one classifier:
```ts
export function canEvictPreview(p: EvictionPreview): boolean {
    return evictPreviewSummary(p).kind !== 'empty';
}
```

---

_Reviewed: 2026-06-22T21:23:01Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
