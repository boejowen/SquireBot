---
phase: 17-self-service-watcher-linking
fixed_at: 2026-06-02T00:00:00Z
review_path: .planning/phases/17-self-service-watcher-linking/17-REVIEW.md
iteration: 1
findings_in_scope: 4
fixed: 4
skipped: 0
status: all_fixed
---

# Phase 17: Code Review Fix Report

**Fixed at:** 2026-06-02
**Source review:** .planning/phases/17-self-service-watcher-linking/17-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 4 (the 4 Warnings; the 5 Info items were out of scope)
- Fixed: 4
- Skipped: 0

All in-scope findings were fixed faithfully per the report's suggested fixes. The
feature is already deployed live, so these are source-only quality fixes that ship on
the next deploy (no redeploy attempted).

**Verification (run in the isolated worktree):**
- `go build ./...` → exit 0
- `go test ./cmd/... ./internal/backendsrv/...` → all packages ok (ingest, store,
  migrations, webadmin, cmd/squirebot-server all pass)
- `cd web && npm run check` → 443 files, 0 errors, 0 warnings

## Fixed Issues

### WR-01: Down-migration drops the index before (failing to) drop its column

**Files modified:** `internal/backendsrv/migrations/00005_self_service_linking.sql`
**Commit:** 7c7c23d
**Applied fix:** Replaced the `+goose Down` block's `DROP INDEX owner_discord_user_id_uidx;`
(a partial revert that would strip the uniqueness guard while leaving the column writable)
with an explicit `SELECT 1;` no-op plus a rationale comment that mirrors 00004. The
down path is now a clean no-op rather than a partial revert.

### WR-02: Ingest `last_seen` stamp inlines SQL, orphaning `StampCodeLastSeen`

**Files modified:** `internal/backendsrv/ingest/handler.go`
**Commit:** 6246071
**Applied fix:** Replaced the inlined `UPDATE guild_code SET last_seen = datetime('now')
WHERE id = ?` in step [5] with `store.StampCodeLastSeen(r.Context(), h.db, codeID)`. The
`store` package was already imported and the helper's `(ctx, *sql.DB, int64)` signature
matched the call site exactly. Restores the handler's documented "no inline SQL"
invariant and makes the store helper's test coverage load-bearing for the production
path. The non-fatal `slog.Warn` on error is preserved.

### WR-03: `formatLastSeen` reports "0 years ago" for last_seen ~360-364 days old

**Files modified:** `web/src/lib/components/WatcherCodesPanel.svelte`
**Commit:** b7fa5c9
**Applied fix:** Changed the month tier from `mon < 12` to a consistent `day < 365`
boundary and floored the month count at `Math.max(1, Math.floor(day / 30))`, closing the
[360, 364]-day gap that previously rendered "last used 0 years ago".

### WR-04: Revoke success message can show a stale ordinal after an optimistic mutation

**Files modified:** `web/src/lib/components/WatcherCodesPanel.svelte`
**Commit:** b7fa5c9
**Applied fix:** After a successful revoke, replaced the optimistic local
`codes.filter(...)` with `codes = await fetchOwnCodes();` (mirroring `generate()`), so the
surviving rows' server-assigned ordinals stay authoritative instead of diverging until the
next full reload.

_Note: WR-03 and WR-04 both edit `WatcherCodesPanel.svelte`. Because the two changes live in
the same file and cannot be split into separate atomic commits without interactive hunk
staging, they were committed together (b7fa5c9). The commit message documents both findings._

---

_Fixed: 2026-06-02_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
