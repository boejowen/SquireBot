---
phase: 36-shared-character-safe-eviction
verified: 2026-06-22T21:05:00Z
status: passed
score: 13/13 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: none
  note: "Initial verification. Code review (36-REVIEW.md) PASSED earlier (0 BLOCKER, 2 non-blocking WARNINGs deferred to backlog)."
---

# Phase 36: Shared-Character-Safe Eviction Verification Report

**Phase Goal:** Eviction removes only the evicted member's own characters — never shared characters that other guildies also play/upload (banks/bots already eviction-safe via Phase 35's sentinel). PLUS the user-ratified edge case: an all-shared owner stays evictable (code-only revoke) end-to-end through the web form.
**Verified:** 2026-06-22T21:05:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

#### Backend (36-01)

| #  | Truth | Status | Evidence |
| -- | ----- | ------ | -------- |
| 1  | A shared char (cross_owner_write row, attempting_owner_id <> X) survives eviction: is_removed stays 0, grace_until NULL | ✓ VERIFIED | `eviction.go:276-280` cascade `WHERE owner_id=? AND is_removed=0 AND NOT `+sharedCharPredicate. `TestEvictOwnerTx_SharedCharSurvives` PASS — Sharedtoon is_removed=0, grace NULL after EvictOwnerTx(X). |
| 2  | Sole-owned live chars still removed: is_removed→1, grace_until stamped as before | ✓ VERIFIED | Same test asserts Soletoon is_removed=1 + grace==graceUntil; removedCount==1. `TestEvictOwnerTx_CascadesAndRevokesCodeInOneTx` (no audit rows) still flips both chars — sole-owned path unregressed. |
| 3  | All-shared owner: removedCount=0 but disabled_at set (code still revoked) | ✓ VERIFIED | `eviction.go:314-318` code-revoke UPDATE is unconditional on removedCount (after the cascade, separate statement). `TestEvictOwnerTx_AllSharedOwnerStillRevokesCode` PASS — removedCount==0, char untouched, guild_code disabled. |
| 4  | PreviewEviction(X) returns the narrowed remove-set (same predicate); CountPreservedShared returns the shared survivors; preview never empty for an owner with live chars | ✓ VERIFIED | `eviction.go:202-207` PreviewEviction + `:235-245` CountPreservedShared both splice the SAME `sharedCharPredicate`. `TestPreviewEviction_OmitsSharedChars` proves preview list == cascade removed-set byte-for-byte. `TestCountPreservedShared_CountsSurvivors` covers mixed(1)/all-shared([]+1)/sole-owned(0). |
| 5  | Preview HTTP reply gains additive snake_case preserved_shared_count int | ✓ VERIFIED | `webadmin/eviction.go:120-132` makes the second store read and adds `"preserved_shared_count": preservedShared` to the reply map. grep count=4 in handler. |
| 6  | Surviving shared char stewarded by X is repointed to a remaining sharer; survival holds even if repoint target unresolvable | ✓ VERIFIED | `eviction.go:300-309` repoint UPDATE, NULL-guarded (`recentOtherSharerSubquery IS NOT NULL` — never writes NULL into NOT-NULL owner_id). `TestEvictOwnerTx_RepointsSurvivingSharedChar` (→Y) + `RepointPicksMostRecentSharer` (→Z, highest audit id) PASS. |
| 7  | Banks/bots (sentinel) + a second guildie's data untouched; ErrCannotEvictSentinel guard FIRST; ListEvictableOwners lists any owner with >=1 live char | ✓ VERIFIED | `eviction.go:267-269` sentinel guard is the FIRST statement (unchanged). `TestEvictOwnerTx_RefusesSentinel` + `GuildBankSurvivesEviction` + `ListEvictableOwners_ExcludesGuildSentinel` PASS. ListEvictableOwners (`:109-117`) NOT narrowed by the predicate. |

#### Web (36-02)

| #  | Truth | Status | Evidence |
| -- | ----- | ------ | -------- |
| 8  | api.ts EvictionPreview gains preserved_shared_count: number, mirroring the backend field-for-field | ✓ VERIFIED | `web/src/lib/api.ts:552` `preserved_shared_count: number;` added; owner_id/characters/grace_until unchanged + unreordered. `npm run check` 0 errors. |
| 9  | All-shared owner (characters:[] BUT preserved_shared_count>0) keeps Evict ENABLED with explicit code-only framing | ✓ VERIFIED | `eviction.ts:59-61` canEvictPreview returns true on preserved_shared_count>0; `:77-86` evictPreviewSummary returns kind:'code-only' with "0 characters removed; {N} shared character(s) preserved; guild code will be revoked." Form gates `canEvict` on canEvictPreview (`EvictionForm.svelte:75-77`). Tests assert all 3 branches. |
| 10 | Genuine zero-live-chars owner (characters:[] AND preserved_shared_count==0) keeps Evict DISABLED | ✓ VERIFIED | canEvictPreview returns false for that shape; `evictPreviewSummary`→kind:'empty' ("No characters found"). Unit test `canEvictPreview empty → false` PASS. |
| 11 | Normal owner with sole-owned chars keeps existing enabled-with-cascade-list behaviour | ✓ VERIFIED | kind:'cascade' branch renders "Characters affected (N):" list (`EvictionForm.svelte:241-247`); canEvictPreview true on characters.length>0. Unit test `cascade → {kind:'cascade'}` PASS. |
| 12 | Gating + framing lives in a PURE node-testable helper; EvictionForm is a thin renderer over it | ✓ VERIFIED | Both helpers are DOM-free pure functions in eviction.ts; the form imports + uses them (`EvictionForm.svelte:36,71,76`). Source-inspection test asserts the form contains canEvictPreview/evictPreviewSummary and NOT cascadeEmpty. |
| 13 | All-shared owner evictable end-to-end through the web form (the user-ratified edge case) | ✓ VERIFIED | End-to-end chain present: backend revokes code at removedCount=0 (truth 3) + emits preserved_shared_count (truth 5) → api.ts mirror (truth 8) → canEvictPreview keeps button enabled + code-only framing (truth 9). Live browser-smoke APPROVED by user (per task brief — all 3 owner scenarios + theme check passed). |

**Score:** 13/13 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `internal/backendsrv/store/eviction.go` | sharedCharPredicate + recentOtherSharerSubquery consts; narrowed cascade + repoint; CountPreservedShared; narrowed PreviewEviction | ✓ VERIFIED | Both consts present (`:59`, `:76`), each DEFINED exactly once (grep gate: 1/1 after comment-strip). Cascade narrowed (`:276`), repoint added (`:300`), CountPreservedShared added (`:235`), PreviewEviction narrowed (`:202`). |
| `internal/backendsrv/webadmin/eviction.go` | EvictionPreviewHandler emitting preserved_shared_count | ✓ VERIFIED | `:120-132` — two store reads, additive snake_case field in reply. |
| `internal/backendsrv/store/eviction_test.go` | shared-survival, all-shared-revoke, preview-parity, repoint, repoint-predicate-lock tests | ✓ VERIFIED | 8 new tests + insertCrossOwnerWrite helper. All PASS. |
| `web/src/lib/api.ts` | EvictionPreview.preserved_shared_count | ✓ VERIFIED | `:552`. |
| `web/src/lib/eviction.ts` | canEvictPreview / evictPreviewSummary pure helpers | ✓ VERIFIED | `:59`, `:77`. |
| `web/src/lib/components/EvictionForm.svelte` | all-shared-evictable gating + code-only framing | ✓ VERIFIED | gate `:75`, render switch `:239-261`; no `{@html}`, no `cascadeEmpty`. |
| `web/src/lib/__tests__/eviction.test.ts` | node tests for the helpers + form-wiring source inspection | ✓ VERIFIED | `describe('eviction preview gating + framing (D-06)')` 6 cases + 4 source-inspection assertions. 17/17 PASS. |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | -- | --- | ------ | ------- |
| EvictOwnerTx cascade UPDATE | audit_log cross_owner_write trail | `AND NOT `+sharedCharPredicate | ✓ WIRED | `eviction.go:278`. Predicate reads `event='cross_owner_write' AND attempting_owner_id <> ?`, the exact row binding.go:100-104 writes (verified). |
| PreviewEviction + CountPreservedShared | audit_log trail | SAME sharedCharPredicate const | ✓ WIRED | Both splice the const verbatim (`:204`, `:239`). Parity proven by TestPreviewEviction_OmitsSharedChars. |
| Repoint UPDATE (SET + guard) | audit_log trail | recentOtherSharerSubquery (one const, reused) | ✓ WIRED | `:302` + `:305` both use the const; drift-locked by TestRepointSubquery_LocksPredicateToSharedPredicate (PASS). |
| EvictionPreviewHandler | store.CountPreservedShared | preserved_shared_count in JSON reply | ✓ WIRED | `webadmin/eviction.go:120`→`:131`. |
| EvictionForm canEvict gate | eviction.ts helper | canEvictPreview reading preview.preserved_shared_count | ✓ WIRED | `EvictionForm.svelte:76` calls canEvictPreview(preview). |
| previewEviction() reply | EvictionPreview.preserved_shared_count | snake_case field mirrored in api.ts | ✓ WIRED | api.ts:552. |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ----------- | ----------- | ------ | -------- |
| OWN-03 | 36-01, 36-02 | Evicting a guildie removes only that member's own characters, not shared characters that other guildies also play and upload | ✓ SATISFIED | Backend cascade narrowed (truths 1-7) + web all-shared-evictable (truths 8-13). REQUIREMENTS.md line 33 confirms backend + web code-complete; live deploy + officer browser-smoke now APPROVED per the verification brief, fully meeting the requirement. |

No orphaned requirements: REQUIREMENTS.md maps only OWN-03 to Phase 36, and both plans declare `requirements: [OWN-03]`.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| (none) | — | — | — | No blocker/warning anti-patterns. The repoint runs unconditionally (IN-01, info only — harmless no-op for sole-owned owners). The two helpers re-derive the same two-field logic (IN-02, info only — both pure + node-tested). Neither affects goal achievement. |

### Gates Run

| Gate | Command | Result |
| ---- | ------- | ------ |
| Backend build | `go build ./...` | ✓ rc=0 |
| Backend vet | `go vet ./internal/backendsrv/store/... ./internal/backendsrv/webadmin/...` | ✓ rc=0 |
| Eviction store tests | `go test ./internal/backendsrv/store/... -run "TestEvict*\|TestPreview*\|TestCount*\|TestRepoint*\|TestList*\|TestRestore*\|TestArchive*" -v` | ✓ 20/20 PASS |
| Full backendsrv suite | `go test ./internal/backendsrv/...` | ✓ all packages ok |
| Full module (ROADMAP criterion 4) | `go test ./...` | ✓ exit 0, no failures |
| Web eviction unit | `npm run test:unit -- --run src/lib/__tests__/eviction.test.ts` | ✓ 17/17 PASS |
| Full web suite | `npm run test:unit -- --run` | ✓ 369/369 (27 files) |
| Svelte typecheck | `npm run check` | ✓ 0 errors / 0 warnings (497 files) |
| Web build | `npm run build` | ✓ built, wrote build/ |
| No new migration (schema v15) | Glob migrations/*.sql | ✓ latest is 00015_guild_owner.sql |
| Single-source consts | comment-stripped grep -c | ✓ sharedCharPredicate=1, recentOtherSharerSubquery=1 |
| Watcher untouched | git diff --name-only across all P36 code commits | ✓ NONE under internal/app, internal/eqfind, internal/sheet, cmd/squirebot |
| No v* tag | git tag --list v2.5* | ✓ none (latest tag v2.1.2) |

### ROADMAP Success Criteria

| # | Criterion | Status | Evidence |
| - | --------- | ------ | -------- |
| 1 | Evicting a member preserves shared characters other guildies play | ✓ MET | Truth 1 + TestEvictOwnerTx_SharedCharSurvives. |
| 2 | Guild banks/bots are never removed by an eviction | ✓ MET | Truth 7 — sentinel guard FIRST + sentinel-owned chars never matched by `WHERE owner_id=realOwner`; TestEvictOwnerTx_RefusesSentinel + GuildBankSurvivesEviction PASS (Phase-35 regression intact). |
| 3 | The officer eviction-preview reflects the narrowed set | ✓ MET | Truth 4 — PreviewEviction shares the cascade predicate; preview list == removed-set proven byte-identical. |
| 4 | `go test ./...` green | ✓ MET | Full module suite exit 0, no failures. |
| + | User-ratified edge case: all-shared owner stays evictable (code-only revoke) end-to-end through the web form | ✓ MET | Truths 3, 5, 8-13 — backend revokes code at removedCount=0, web keeps button enabled with code-only framing; live browser-smoke APPROVED. |

### Human Verification Required

None outstanding. The live prod deploy (backend binary swap + web atomic-swap, schema v15, no migration, no v* tag) and the officer browser-smoke (all 3 owner scenarios + 2-theme check) are reported APPROVED by the user per the verification brief — the live/human gate is already satisfied. This verification confirms the code-level goal achievement that backs that smoke.

### Gaps Summary

No gaps. Every observable truth, artifact, key link, and ROADMAP success criterion is verified directly against the codebase (not SUMMARY claims). The core deletion-safety logic is single-source-of-truth (one `sharedCharPredicate` const backs the cascade, the preview, and the survivor count — structurally closing the Phase-35 CR-01 picker/write divergence), the Phase-35 sentinel/bank regression is intact, the watcher tree is untouched, no schema migration shipped, and no `v*` tag was created. The detection predicate was confirmed to read the exact `cross_owner_write` row shape that `binding.go` writes on a cross-owner upload — the trail and the reader are correctly wired. The two code-review WARNINGs (WR-01 preview guard asymmetry; WR-02 repoint-to-evicted-steward) are pre-existing/cosmetic, deliberately deferred to backlog, and do not affect the hard guarantee (a shared char survives) — they are not phase gaps.

---

_Verified: 2026-06-22T21:05:00Z_
_Verifier: Claude (gsd-verifier)_
