---
phase: 35-owner-less-guild-banks-bots
verified: 2026-06-22T00:00:00Z
status: passed
score: 6/6 must-haves verified
overrides_applied: 0
re_verification:
  note: "First verification of the FIXED state (post code-review CR-01/WR-01/WR-02/IN-01 fixes); no prior VERIFICATION.md existed"
deferred:
  - truth: "Label-bridge lookups could resolve the reserved 'guild' label to the sentinel owner (IN-02)"
    addressed_in: "backlog (future hardening pass)"
    evidence: "35-REVIEW.md IN-02 classifies it 'No action required for this phase' — pre-existing, out of phase scope, requires a guildie/--owner literally named 'guild'; tracked for future label-bridge exclusion"
---

# Phase 35: Owner-Less Guild Banks & Bots Verification Report

**Phase Goal:** Make designated guild banks/bots GUILD-HELD (owner-less) rather than tied to whoever first uploaded them — so (a) an officer can designate any character as a bank/bot WITHOUT owning it, (b) a designated bank/bot survives its first-uploader's eviction, and (c) existing owner-bound banks migrate automatically. Design = Option A (reserved "guild" sentinel owner, id 1000000).
**Verified:** 2026-06-22
**Status:** passed
**Re-verification:** No — initial verification (of the post-code-review-fix state)

## Goal Achievement

### Observable Truths

| #   | Truth | Status | Evidence |
| --- | ----- | ------ | -------- |
| 1   | An officer can designate any character as a guild bank/bot WITHOUT owning it (no claim) — DesignateCharTx repoints owner_id to the sentinel, gated only by the officer re-check (OWN-01) | ✓ VERIFIED | `assignment.go:428-484` — `isOfficerTx` is the FIRST statement (→ `ErrNotAuthorized`); the repoint `UPDATE character SET owner_id=? WHERE id=?` (GuildSentinelOwnerID) sits inside the `mode==bank\|\|bot` block. No ownership precondition. Tests `TestDesignateCharTx_Bank/BotRepointsOwnerToSentinel` PASS; `TestDesignateCharTx_NonOfficerNoRepoint` proves a non-officer is rejected before any write |
| 2   | A designated bank/bot is owner-less: its owner_id equals the reserved guild sentinel id, not any individual guildie (OWN-02 enabling condition) | ✓ VERIFIED | `owner.go:30` `const GuildSentinelOwnerID int64 = 1000000`; the repoint sets exactly this. Test asserts `owner != GuildSentinelOwnerID → fail`. PASS |
| 3   | A designated bank/bot is NOT removed when its first-uploader is evicted (OWN-02) | ✓ VERIFIED | `EvictOwnerTx` cascade is `WHERE owner_id=? AND is_removed=0`; the sentinel id is never that of a real guildie. `TestEvictOwnerTx_GuildBankSurvivesEviction` PASS — evicting realOwner flips ONLY their char (removedCount=1), the sentinel-owned bank stays is_removed=0 / grace NULL. **PLUS the CR-01 fix:** `EvictOwnerTx`/`RestoreOwnerTx` reject `GuildSentinelOwnerID` at the top (`ErrCannotEvictSentinel`); webadmin maps it to 403. `TestEvictOwnerTx_RefusesSentinel`, `TestRestoreOwnerTx_RefusesSentinel`, `TestEvict_RefusesGuildSentinel` all PASS |
| 4   | Existing owner-bound banks/bots (Findom→owner 9) migrate automatically with no manual fixup (OWN-04) | ✓ VERIFIED | `00015_guild_owner.sql:38-41` backfills `UPDATE character SET owner_id=1000000 WHERE (is_bank_toon=1 OR is_guild_bot=1) AND owner_id<>1000000`. `TestMigrate_00015_BackfillRunsOverPreExistingData` drives the ACTUAL embedded SQL over v14 data (not a copy): Findom+Botchar repoint to sentinel, Normalchar untouched. PASS |
| 5   | The guild sentinel never appears as an evictable/restorable guildie (picker) | ✓ VERIFIED | `eviction.go:73,131` both queries carry `WHERE o.id <> ?` bound to `GuildSentinelOwnerID`. `TestListEvictableOwners_ExcludesGuildSentinel` + `TestListRestorableOwners_ExcludesGuildSentinel` PASS — only the normal owner returned |
| 6   | `go build ./...` + `go test ./internal/backendsrv/...` green; watcher untouched; no v* tag | ✓ VERIFIED | `go build ./...` rc=0; all 18 backend packages PASS; `go vet ./store/... ./migrations/... ./webadmin/...` clean; `git status` of watcher tree empty; latest tag still v2.1.2 (no v2.5/v* added) |

**Score:** 6/6 truths verified

### Deferred Items

| # | Item | Addressed In | Evidence |
|---|------|-------------|----------|
| 1 | IN-02: label-bridge `'guild'`-label collision could resolve to the sentinel | backlog (future hardening) | 35-REVIEW.md IN-02: "No action required for this phase"; pre-existing paths (linking.go/eviction.go/auth/store.go), requires a guildie or `--owner` literally named "guild"; deliberately out of phase scope per the do-not list |

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `migrations/00015_guild_owner.sql` | sentinel seed + bank/bot backfill | ✓ VERIFIED | `INSERT OR IGNORE INTO owner (id,label) VALUES (1000000,'guild')` + guarded backfill; forward-only `SELECT 1;` Down + IRREVERSIBLE note (WR-02 fix); embedded via `//go:embed *.sql` so it auto-applies on boot |
| `store/owner.go` | GuildSentinelOwnerID constant | ✓ VERIFIED | `const GuildSentinelOwnerID int64 = 1000000` + full doc comment. No DB resolver (correctly deemed unnecessary — id is fixed) |
| `store/assignment.go` | DesignateCharTx repoints owner_id on bank/bot | ✓ VERIFIED | repoint at `assignment.go:476-481`, inside the officer-gated `mode==bank\|\|bot` block; `DesignateNeither` does not repoint |
| `store/eviction.go` | owner-lists exclude sentinel + write-path guard | ✓ VERIFIED | `o.id <> ?` in both lists; `ErrCannotEvictSentinel` guard at top of EvictOwnerTx/RestoreOwnerTx |
| `webadmin/eviction.go` | maps ErrCannotEvictSentinel → 403 | ✓ VERIFIED | `mapEvictionErr:319-323` → `StatusForbidden, "cannot_evict_sentinel"` (CR-01 fix, beyond original plan scope) |
| `store/owner_test.go` + test files | the behavioral proofs | ✓ VERIFIED | 13 Phase-35 tests, all PASS, all substantive (assert actual column state, not response shape) |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | -- | --- | ------ | ------- |
| webadmin/assignment_admin.go DesignateCharHandler | store.DesignateCharTx | officer designate POST | ✓ WIRED | `assignment_admin.go:333` calls `DesignateCharTx(ctx, tx, req.CharacterID, mode, callerID, now)` inside `withTx`, composed with `AppendAuditTx("char_designate", ...)` — the repoint rides the audited tx (T-35-05) |
| store.DesignateCharTx (bank/bot) | character.owner_id = sentinel | in-tx UPDATE | ✓ WIRED | `assignment.go:476` `UPDATE character SET owner_id = ? WHERE id = ?` binds `GuildSentinelOwnerID` |
| migration 00015 backfill | owner_id of existing bank/bot chars | UPDATE … WHERE is_bank_toon=1 OR is_guild_bot=1 | ✓ WIRED | `00015_guild_owner.sql:38-41`; exercised over pre-existing data by `TestMigrate_00015_BackfillRunsOverPreExistingData` |
| webadmin EvictHandler | store.EvictOwnerTx (write-path sentinel guard) | POST {owner_id} | ✓ WIRED | `eviction.go:175` → `EvictOwnerTx` → guard → `mapEvictionErr` → 403; `TestEvict_RefusesGuildSentinel` proves end-to-end (status 403, bank survives, 0 audit rows) |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ----------- | ----------- | ------ | -------- |
| OWN-01 | 35-01-PLAN | Officer designates any char a bank/bot without owning it (no claim) | ✓ SATISFIED | Truth 1 — repoint gated only by officer re-check; no ownership precondition; tests PASS |
| OWN-02 | 35-01-PLAN | Designated bank/bot is owner-less and NOT removed/archived when first-uploader evicted | ✓ SATISFIED | Truths 2/3/5 — survives-different-owner-eviction proof AND sentinel-can't-be-directly-evicted guard (CR-01 fix) both present and passing |
| OWN-04 | 35-01-PLAN | Existing owner-bound banks migrate automatically, no manual fixup | ✓ SATISFIED | Truth 4 — embedded-SQL backfill test over pre-existing v14 data PASS |
| OWN-03 | (Phase 36) | Eviction removes only the member's own chars, not shared chars | n/a — not this phase | REQUIREMENTS.md maps OWN-03 → Phase 36; correctly NOT claimed by Phase 35 |

All three declared requirement IDs (OWN-01/OWN-02/OWN-04) are present in REQUIREMENTS.md mapped to Phase 35, claimed in the PLAN frontmatter, and SATISFIED. No orphaned requirements (OWN-03 is Phase 36's, correctly out of scope).

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Module compiles | `go build ./...` | rc=0 | ✓ PASS |
| Backend suite green | `go test ./internal/backendsrv/...` | all 18 pkgs ok, rc=0 | ✓ PASS |
| Phase-35 tests run+pass (not skipped) | `go test … -run "Migrate_00015\|GuildSentinel\|…" -v` | 13/13 PASS, 0 SKIP | ✓ PASS |
| Static analysis | `go vet ./store/... ./migrations/... ./webadmin/...` | clean | ✓ PASS |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| (none) | — | — | — | The 3 "placeholder" grep hits are legitimate prose ("? placeholders", "snowflake placeholder") in comments, not stubs. No TODO/FIXME/empty-impl/hardcoded-data in the changed production files |

### Human Verification Required

None. This is a backend-only, schema + store + handler change fully covered by deterministic Go tests (migration idempotency, the OWN-02 survives-eviction proof, the CR-01 write-path 403 proof, the OWN-04 backfill over pre-existing data). No visual/real-time/external-service behavior is in scope; the migration applies automatically on boot (the prod deploy of 00015 is a separate future step, explicitly noted in the SUMMARY as not part of this code-only phase).

### Gaps Summary

No gaps. All 6 observable truths are VERIFIED against the actual codebase, all 3 requirement IDs (OWN-01/OWN-02/OWN-04) SATISFIED, every key link WIRED, and the gates (build, full backend test, vet) are green.

The code-review cycle is reflected in the verified state:
- **CR-01 (BLOCKER) is genuinely fixed in code**, not just in the SUMMARY narrative. `EvictOwnerTx`/`RestoreOwnerTx` reject `GuildSentinelOwnerID` at the top (`ErrCannotEvictSentinel`), `webadmin/eviction.go` maps it to HTTP 403, and three tests (`TestEvictOwnerTx_RefusesSentinel`, `TestRestoreOwnerTx_RefusesSentinel`, `TestEvict_RefusesGuildSentinel`) prove the destructive POST path is closed AND the bank survives. The original survives-when-a-different-owner-is-evicted proof (`TestEvictOwnerTx_GuildBankSurvivesEviction`) is also present — both halves of OWN-02 are covered.
- **WR-01** fixed: the OWN-04 backfill is now exercised against pre-existing v14 data via the ACTUAL embedded migration SQL (`TestMigrate_00015_BackfillRunsOverPreExistingData`), removing the copied-SQL drift risk.
- **WR-02** fixed (doc-only): the migration header documents the irreversible owner_id overwrite + R2-backup recovery path.
- **IN-01** fixed: the migration test binds to `store.GuildSentinelOwnerID` and `TestGuildSentinelOwnerID_MatchesContract` machine-checks the constant == 1000000.
- **IN-02** is an accepted, pre-existing, out-of-scope deferral (label-bridge collision requiring a guildie literally named "guild") — correctly NOT a phase gap.

Watcher tree (`internal/app/`, `internal/eqfind/`, `internal/sheet/`, `cmd/squirebot/`) confirmed untouched; no `v*` git tag created (latest remains v2.1.2). Phase 35 goal is achieved.

---

_Verified: 2026-06-22_
_Verifier: Claude (gsd-verifier)_
