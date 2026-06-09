---
quick_id: 260609-d2o
slug: officer-undesignate-bank-bot-char
status: complete
date: 2026-06-09
backlog: 999.33
commits: [b7b0a48, 2b18d65]
deploy: pending (backend binary swap + web atomic-swap; no migration)
---

# Quick 999.33 — Officer-reversible guild-bank/bot designation

**Problem (999.33, MEDIUM):** `DesignateCharTx` clears the `character_assignment` row, so a designated bank/bot character dropped out of `ListAllAssignments` (the only list `AssignmentAdminPanel` rendered) AND was excluded from member claimable → bank/bot was a UI one-way door, reversible to `mode:none` only via a direct API/DB call. (Surfaced live during the Phase 26 UAT; required a scoped direct-DB recovery.)

**Fix:** give officers a UI surface to see designated chars and clear the designation.

## Changed
**Backend (officer-only read; no migration, watcher untouched):**
- `internal/backendsrv/store/assignment.go` — new `DesignatedChar{character_id, name, kind}` + `ListDesignatedChars(ctx, db)`: live chars (`is_removed=0`) where `is_bank_toon=1 OR is_guild_bot=1`, ordered `name COLLATE NOCASE`; `kind` derived deterministically ("bank" preferred if both, else "bot"); non-nil-slice contract (mirrors `ListClaimable`).
- `internal/backendsrv/webadmin/assignment_admin.go` — `ListDesignatedCharsHandler` (GET, `nil→[]`, read-only, no audit/tx; mirrors `ListAllAssignmentsHandler`).
- `cmd/squirebot-server/main.go` — `GET /api/v1/admin/characters/designated` under `webauth.RequireOfficer`.
- `internal/backendsrv/store/assignment_test.go` — `TestListDesignatedChars` (bank+bot+normal+removed-bank → only live bank/bot, correct kind/order) + `TestListDesignatedChars_EmptyIsNil`.

**Web:**
- `web/src/lib/api.ts` — `interface DesignatedChar` + `fetchDesignatedChars()` (credentialed `getJSON`; `designateChar` unchanged).
- `web/src/lib/components/AssignmentAdminPanel.svelte` — new "Designated characters (guild bank / bot)" section; `load()` fetches it via `Promise.all`; each row = `{name}` (plain `{}`, never `{@html}`) + kind chip + "Clear designation" → `designateChar(id, 'none')` via the existing `act()`/`busyKey`+reload pattern; empty-state note; existing styling.

## New route
`GET /api/v1/admin/characters/designated` (officer-only) → `DesignatedChar[]` (`[]` when none). The clear reuses the existing **audited** `POST /api/v1/admin/characters/designate` `{mode:"none"}`.

## Gates — all green
`go build ./...` · `go vet ./internal/backendsrv/... ./cmd/squirebot-server/...` · `go test ./internal/backendsrv/store/ ./internal/backendsrv/webadmin/` ok · web `npm run check` 0/0 (486 files) · `npm test` 303 · `npm run build` ok.

## Confirmation
No migration / no schema change (reads existing columns). Watcher untouched (no `WatcherMaxSchemaVersion` bump). Officer-only (RequireOfficer + the panel 403-collapses for non-officers).

## Deploy (separate step)
Backend binary swap + web atomic-swap (NO `goose` migration). Adds a route → the binary must be redeployed, not web-only.
