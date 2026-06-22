---
quick_id: 260621-u6j
title: "Allow any guild member to upload any character (drop cross-owner reject)"
status: complete
date: 2026-06-22
---

# Quick Task 260621-u6j — Drop the cross-owner write reject — SUMMARY

Backend-only / no migration: dropped the first-uploader-wins cross-owner reject so
ANY valid (non-revoked) guild code may upload ANY character. Deploying the new binary
unblocks every guildie at once (incl. Kim's Aenriel re-uploads and lern41's Findom) —
including those still on watcher 2.1.1. Reverses the locked D-07/V4 single-owner
decision per the user's 2026-06-21 ratification.

## Commits (code only — `.planning/` committed by the orchestrator)

| # | Hash | Message |
|---|------|---------|
| 1 | `fc6aee5` | `feat(backend): allow any guild member to upload any character (shared chars/banks)` |
| 2 | `2dfd871` | `test(backend): cross-owner uploads now succeed + audit as cross_owner_write` |
| 3 | `3f11edd` | `chore(watcher): drop now-unreachable cross-owner tray branch` |

## What changed

### Task 1 — Backend allow + audit (`fc6aee5`)
- `internal/backendsrv/store/binding.go`
  - `bindCharacter` cross-owner branch now ALLOWS the write: appends a
    `cross_owner_write` audit row, `slog.Info` ("cross-owner upload allowed (shared
    character)"), and returns the **existing `charID` + nil**. `owner_id` is NEVER
    overwritten — the first uploader stays a non-binding steward record.
  - Renamed `auditCrossOwnerReject` → `auditCrossOwnerWrite`; event string
    `cross_owner_reject` → `cross_owner_write` (same columns; audit_log stays
    append-only).
  - Removed `ErrCharOwnedByAnother` (no longer returned anywhere).
  - Rewrote the package-doc model comment to the new shared-chars/banks doctrine and
    fixed the stale `binding.go:12` "Reassignment is a P15 admin action" line
    (owner_id is now a non-gating first-sighting steward marker).
- `internal/backendsrv/ingest/handler.go`
  - `bindAndReplace`: removed the `ErrCharOwnedByAnother` commit-the-audit-then-return
    sub-branch; bind error handling collapses to `if err != nil { rollback; return }`.
    Bind + replace + the cross_owner_write audit row all commit in ONE tx.
  - `ServeHTTP`: removed the dead 409 mapping; any `bindAndReplace` error is now a 500.
  - Updated the request-flow / single-tx doc comments to the allow-and-audit model.

### Task 2 — Tests to the new behavior (`2dfd871`)
- `binding_test.go`: `TestBindCharacter_CrossOwnerRejects` →
  `TestBindCharacter_CrossOwnerWriteAllowed` — cross-owner bind returns the existing
  charID + nil, asserts `owner_id` UNCHANGED (the non-binding-record guarantee), and
  one `cross_owner_write` audit row. Dropped the now-unused `errors` import and
  refreshed the `bindInTx` helper comment.
- `handler_test.go`: `TestIngest_CrossOwner_409` → `TestIngest_CrossOwner_AllowedReplaces`
  — B's upload of A's char returns **2xx** and full-snapshot **replaces** A's rows
  (3 → 1), with one `cross_owner_write` audit row. Updated the package-doc line.

### Task 3 — Watcher prune (`3f11edd`)
- `internal/app/runapp.go`: removed `handleIngestErr`'s `backend.ErrCrossOwner`
  tray-red branch (added in quick 260621-td4) — now unreachable. A stray 409 falls
  through to the default `err != nil` branch (terminal, no-retry, mtime not
  persisted), which is the correct defensive behavior. Replaced the branch with an
  explanatory comment. `internal/backend/client.go`'s 409 → `ErrCrossOwner` mapping
  left AS-IS (harmless defensive HTTP mapping).

## Verification

```
go build ./...                                  → BUILD_OK
go vet ./internal/backendsrv/...                → VET_OK
go test ./internal/backendsrv/... ./internal/app/...  → all packages ok
```

All 18 backendsrv packages + `internal/app` pass. No file deletions across the three
commits.

**409 watcher test confirmed UNCHANGED:** `TestMakeOnSpellbookChange_409CrossOwnerNoPersist`
still PASSES (not modified). With the `ErrCrossOwner` tray branch removed, a 409 now
hits the default `err != nil` branch — logged as `ERROR upload spellbook char=Foo
err="backend: character owned by another guildie"` — which is still terminal/no-retry
and does NOT persist the mtime, so its assertions (exactly 1 request; mtime untouched)
hold as written.

## Constraints honored
- No migration, no schema change, no `v*` tag.
- `commit_docs:false` — only `internal/` code committed in the three task commits;
  `.planning/` (this SUMMARY + PLAN + STATE) left for the orchestrator.
- audit_log stays append-only; event renamed in place (same columns).
- `owner_id` never overwritten on a cross-owner write.
- V7 preserved: no token/content logged; single bind+replace transaction shape kept.
- No CLAUDE.md edit (it carries no cross-owner doctrine).

## Deviations
None — plan executed exactly as written.

## Deferred follow-ups (non-blocking)
- **Q2 — owner-less / eviction-safe guild banks:** deferred per the user's
  AskUserQuestion; this relaxation already unblocks Findom without a schema change.
- **Eviction edge:** owner-scoped eviction (revoking an owner) now removes shared
  characters bound under that owner — worth revisiting alongside the owner-less-banks
  follow-up.

## Remaining step (orchestrator)
Backend-only — **a prod deploy of the new server binary is required** to take effect
(swap the binary on root@5.78.232.85; no goose-run, no migration; proof: a cross-owner
upload returns 2xx instead of 409). The orchestrator handles deploy, not the executor.

## Self-Check: PASSED
- `260621-u6j-SUMMARY.md` — FOUND
- commit `fc6aee5` — FOUND
- commit `2dfd871` — FOUND
- commit `3f11edd` — FOUND
- working tree clean except the untracked `.planning/quick/260621-u6j-*` dir (docs,
  committed by the orchestrator)
