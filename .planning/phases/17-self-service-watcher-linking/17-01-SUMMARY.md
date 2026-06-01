---
phase: 17-self-service-watcher-linking
plan: 01
subsystem: backend (Go + SQLite — migrations, bearer guard, ingest)
tags: [migration, goose, sqlite, auth, bearer-guard, ingest, last-seen, fk, link-02, link-05]
requires:
  - "00004 web_auth schema (web_user.discord_user_id TEXT PRIMARY KEY — the FK target)"
  - "auth.MintCode / RevokeCode + guild_code table (00001)"
  - "ingest.Handler + auth.Auth bearer guard"
provides:
  - "owner.discord_user_id FK→web_user + partial unique index owner_discord_user_id_uidx"
  - "guild_code.last_seen TEXT column"
  - "auth.ResolveToken/resolveToken returning (ownerID, codeID, ok)"
  - "best-effort guild_code.last_seen stamp on the ingest path"
affects:
  - "any future Phase 17 handler/store work that resolves owner-by-Discord or lists/revokes codes"
  - "webadmin eviction owner-floor rewire (D-05, a later plan) — relies on owner.discord_user_id"
tech-stack:
  added: []
  patterns:
    - "forward-only goose migration 00005 (extend-only ALTER + partial unique index)"
    - "SQLite ADD-UNIQUE landmine avoided via separate CREATE UNIQUE INDEX ... WHERE col IS NOT NULL"
    - "bearer guard threads matched guild_code.id out for last_seen stamping"
    - "advisory write OUTSIDE the atomic replace tx (best-effort, non-fatal)"
key-files:
  created:
    - "internal/backendsrv/migrations/00005_self_service_linking.sql"
  modified:
    - "internal/backendsrv/migrations/migrate_test.go"
    - "internal/backendsrv/auth/guard.go"
    - "internal/backendsrv/auth/guard_test.go"
    - "internal/backendsrv/ingest/handler.go"
    - "internal/backendsrv/ingest/whoami.go"
decisions:
  - "last_seen is TEXT/datetime('now') to match the sibling guild_code 00001 columns (NOT epoch INTEGER like the 00004 web columns) — keeps guild_code internally consistent"
  - "uniqueness via partial CREATE UNIQUE INDEX (one owner per Discord id, many NULLs allowed) — SQLite forbids ADD COLUMN ... UNIQUE"
  - "the last_seen stamp is a separate best-effort UPDATE outside the replace tx — a failed stamp never fails the ingest (advisory UI metadata, not data integrity)"
metrics:
  duration: ~6 min
  completed: 2026-06-01
  tasks: 3
  files: 6
  commits: 3
---

# Phase 17 Plan 01: Schema + Bearer-Guard Foundation Summary

The `00005` forward-only migration plus the two code changes that make a per-code `last_seen` stampable: `owner.discord_user_id` FK→`web_user` with a partial unique index (LINK-02), `guild_code.last_seen` (LINK-05/D-07), the bearer guard widened to thread the matched `guild_code.id` out as `(ownerID, codeID, ok)`, and a best-effort `last_seen` stamp on the authenticated ingest path — written outside the atomic replace tx so a stamp failure never fails an upload.

## What Was Built

- **`00005_self_service_linking.sql`** — three forward-only statements:
  - `ALTER TABLE owner ADD COLUMN discord_user_id TEXT REFERENCES web_user(discord_user_id)` (TEXT matches the `web_user.discord_user_id TEXT PRIMARY KEY` FK target).
  - `CREATE UNIQUE INDEX owner_discord_user_id_uidx ON owner(discord_user_id) WHERE discord_user_id IS NOT NULL` — the partial index enforces one-owner-per-Discord-id while letting the ~11 existing owners stay NULL (avoids the SQLite "cannot ADD a UNIQUE column" landmine — RESEARCH Pitfall 1).
  - `ALTER TABLE guild_code ADD COLUMN last_seen TEXT` — TEXT/`datetime('now')` to match the sibling 00001 `guild_code` timestamp columns.
  - `migrate_test.go` gained `TestMigrate_00005_AddsSelfServiceLinking` (+ an `indexExists` PRAGMA `index_list` helper) asserting the FK column, the partial index, and `last_seen` all exist and that a second `RunMigrations` is a clean no-op.

- **`auth/guard.go`** — `resolveToken`/`ResolveToken` now return `(ownerID, codeID int64, ok bool)`; the SELECT widened to `SELECT id, owner_id, token_hash FROM guild_code WHERE disabled_at IS NULL`. Every miss/error path returns `(0, 0, false)`. The `subtle.ConstantTimeCompare` timing-safe compare and the never-log-the-token discipline are unchanged. `guard_test.go` updated: the table test asserts `codeID` tracks `ownerID`'s presence (non-zero on a match, 0 on every miss); `TestResolveToken_ReturnsMintingOwner` asserts a valid code yields a non-zero `codeID`.

- **`ingest/handler.go`** — call site widened to `(ownerID, codeID, ok)`; after a successful `bindAndReplace` and BEFORE `w.WriteHeader(status)`, a separate best-effort `UPDATE guild_code SET last_seen = datetime('now') WHERE id = ?` stamps the uploading code. A failed stamp logs `code_id` + `err` only (never the token — V7) and is dropped; it does NOT fold into the replace tx and does NOT change the ingest response.

- **`ingest/whoami.go`** — call site updated to `ownerID, _, ok := ...` (read-only path discards the codeID; only the ingest write path stamps).

## Verification

- `go test ./migrations/...` — PASS (00005 applies cleanly on a fresh DB; no "Cannot add a UNIQUE column").
- `go test ./auth/...` — PASS (widened guard + updated tests).
- `go build ./...` (repo root) — PASS (the 3-return signature propagates cleanly to both call sites).
- `go test ./internal/backendsrv/...` — PASS (all packages: auth, compute, enrich, ingest, migrations, readapi, scheduler, store, webadmin, webauth).

## Deviations from Plan

None — plan executed exactly as written. (One cosmetic adjustment: a comment line in the migration originally contained the literal phrase `ADD COLUMN ... UNIQUE`, which tripped the acceptance grep `ADD COLUMN[^;]*UNIQUE`; the comment was reworded to "cannot add a UNIQUE column via ALTER" so the grep correctly matches no inline-UNIQUE DDL. No behavior change.)

## Notes for Downstream Plans

- The FK column is added NULL on existing owners; it stamps lazily — a later plan's resolve-or-create-owner algorithm (D-03/D-04) is what populates it on first self-mint. No backfill was done or needed.
- `codeID` is now available end-to-end in the ingest path; the bearer guard is the single source of the matched code-row id (do not re-hash the token in handlers to recover it).
- The owner-floor FK rewire (D-05) and the three `/account` handlers (mint/list/revoke) are NOT in this plan — they build on this foundation.

## Self-Check: PASSED

- FOUND: internal/backendsrv/migrations/00005_self_service_linking.sql
- FOUND commit c318f1a (Task 1), 7e40301 (Task 2), 4a886e1 (Task 3)
