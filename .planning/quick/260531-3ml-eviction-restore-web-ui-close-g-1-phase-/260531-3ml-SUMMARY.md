---
quick_id: 260531-3ml
title: Eviction restore web UI (close G-1) + Phase 15 deploy wrap-up docs
type: quick
completed: 2026-05-31
tasks_completed: 3
commits:
  - 61b7861  # feat: GET /api/v1/admin/restorable
  - 6bfabd1  # feat: eviction restore web UI (close G-1)
  - 2b2f263  # docs: Phase 15 auth deploy + EnvironmentFile
files_modified:
  - internal/backendsrv/store/eviction.go
  - internal/backendsrv/store/eviction_test.go
  - internal/backendsrv/webadmin/eviction.go
  - cmd/squirebot-server/main.go
  - cmd/squirebot-server/main_test.go
  - web/src/lib/api.ts
  - web/src/lib/eviction.ts
  - web/src/lib/components/EvictionForm.svelte
  - web/src/lib/__tests__/eviction.test.ts
  - deploy/squirebot-server.service
  - docs/backend-deploy.md
---

# Quick 260531-3ml: Eviction restore web UI (close G-1) + Phase 15 deploy wrap-up docs Summary

Closed the G-1 gap (eviction restore had a backend + `api.ts` wrapper but no way to LIST restorable owners and no UI) by adding an officer-only `GET /api/v1/admin/restorable` endpoint and a Restore section in `EvictionForm.svelte`, and documented the Phase 15 auth deploy (root-only secrets env file + owner-floor seed + frontend bundle) so the next deploy is not tribal knowledge. All verify commands green; the tree is left clean.

## What was built

### Task 1 — `GET /api/v1/admin/restorable` (commit `61b7861`)
- **`store.ListRestorableOwners(ctx, db, now)` + `RestorableOwner`** — the exact inverse of `ListEvictableOwners`/`EvictableOwner`: owners with >=1 character that is `is_removed = 1 AND grace_until IS NOT NULL AND grace_until > ? AND archived_at IS NULL` (still in grace, not archived). `GROUP BY o.id, o.label`, `char_count = COUNT(c.id)`, `grace_until = MIN(c.grace_until)` (soonest deadline), `ORDER BY o.label COLLATE NOCASE`. Parameterized `?` only.
- **`webadmin.RestorableListHandler(db)`** — mirrors `EvictableListHandler` (GET-only method check, `nil → []`, `writeJSON`, `slog.Error` on failure). Calls `store.ListRestorableOwners(r.Context(), db, nowUnix())`.
- **Route** — `mux.Handle("GET /api/v1/admin/restorable", webauth.RequireOfficer(db, webadmin.RestorableListHandler(db)))` registered next to the evictable route. The existing `RestoreHandler` (in-tx officer re-check, WR-04; re-mint to journald only, WR-02) was NOT touched.
- **Tests** — store: `TestListRestorableOwners` seeds a live owner, an in-grace evicted owner (2 chars), and an archived/past-grace owner, and asserts ONLY the in-grace one is returned (live + archived excluded), with `char_count == 2` and `grace_until == now + 30d`. `cmd/squirebot-server` `TestWriteRoutes_Gates`: added `/api/v1/admin/restorable` to the officer-gate coverage (no session → 401, member session → 403).

### Task 2 — Eviction restore web UI (commit `6bfabd1`)
- **`api.ts`** — `RestorableOwner` interface (`owner_id/label/char_count/grace_until`, grace as epoch SECONDS) + `fetchRestorable(fetchFn=fetch)` → `getJSON('/api/v1/admin/restorable')`, mirroring `fetchEvictable`.
- **`eviction.ts`** — pure `restoreResultMessage(res: RestoreResult, label: string): string`. WR-02-correct copy: `new_code_issued` → "Restored {label}. A fresh guild code was minted on the SERVER (read it from the server logs / re-run `mint-code`) and hand it to them — it is not shown here."; `code_mint_failed` → "Restored {label}, but re-minting the guild code failed — re-issue it on the server with `mint-code`." (WR-01). Pure + node-testable; label passed in as an arg.
- **`EvictionForm.svelte`** — a "Restore evicted guildies" section rendered in BOTH ready branches via a `{#snippet restoreSection()}` (so it shows even when there is no live evictable guildie). On mount `load()` now `Promise.all([fetchEvictable(), fetchRestorable()])`. Each restorable row shows `label`, `char_count`, and `graceDate(grace_until)` deadline, with a Restore button → `ConfirmDialog` (heading "Restore guildie") → `restoreEviction(owner_id)` → on success `restoreResultMessage(...)` + drop the row. Errors route through the SAME `route()` server-truth helper (officers-only collapse / 401 bubble / owner-floor inline; `grace_expired` 409 → generic inline via the existing `reason()` which already maps it). Empty restorable list renders a quiet note, not an error. All labels via plain `{}` (no `{@html}`, T-15-28). Non-destructive accent styling for the Restore button (vs. the destructive Evict token).
- **`eviction.test.ts`** — node-only `restoreResultMessage` cases (both `new_code_issued` and `code_mint_failed` branches, asserting the WR-02 "SERVER" / "not shown here" / `mint-code` copy and the WR-01 "failed" wording) + a `.svelte` source-inspection block (reads `../components/EvictionForm.svelte` via `readFileSync`/`fileURLToPath`, the AuthGate.test.ts idiom) asserting the Restore section imports/calls `restoreEviction` + `fetchRestorable` + `restoreResultMessage`, gates behind a `ConfirmDialog` (`onConfirm={doRestore}`), and contains no `{@html}`. NO new test dependency.

### Task 3 — Phase 15 deploy wrap-up docs (commit `2b2f263`)
- **`deploy/squirebot-server.service`** — added `EnvironmentFile=-/etc/squirebot/squirebot.env` under `[Service]` (leading `-` = optional, so local/test/no-file starts still boot — matching the live box). Rest of the unit unchanged.
- **`docs/backend-deploy.md`** — new "## 7. Phase 15 — auth deploy (Discord login + admin forms)" with sub-sections: 7.1 the root-only `/etc/squirebot/squirebot.env` (chmod 600) carrying `DISCORD_CLIENT_ID/SECRET/GUILD_ID`, `DISCORD_REDIRECT_URI`, `SQUIREBOT_WEB_ORIGIN`, `SQUIREBOT_COOKIE_DOMAIN` (secret backend-only, never in repo/bundle/logs); 7.2 the `EnvironmentFile=` line + daemon-reload/restart; 7.3 `00004_web_auth.sql` on boot (goose.Up, schema v4) + verification; 7.4 `set-owner-floor <maintainer-discord-USER-id>` seed; 7.5 the frontend bundle deploy (`npm run build` → ship `web/build/` to `/var/www/squirebot`, Caddy `file_server`). Terse, command-first style matching the existing sections.

## Verification (real results)

| Command | Result |
|---------|--------|
| `go test ./internal/backendsrv/... ./cmd/squirebot-server/...` (Task 1) | PASS (store 28.4s, webadmin 14.5s, cmd 4.5s) |
| `go test ./...` (final, whole repo) | PASS — all 27 packages `ok`, exit 0 |
| `go build ./...` | PASS (`BUILD_OK`) |
| `GOOS=linux GOARCH=amd64 go build ./cmd/squirebot-server` | PASS (`CROSS_OK`) |
| `npm run check` (web) | PASS — 432 files, 0 errors, 0 warnings |
| `npm run test:unit -- --run` (web) | PASS — 14 files, 178/178 tests (was 172; +6 new) |
| `npm run build` (web) | PASS — `built in 6.63s`, `build/index.html` emitted |
| Task 3 greps (`EnvironmentFile` / `set-owner-floor` / `squirebot.env`) | PASS (all three) |

## Acceptance criteria (all met)
- [x] 3 tasks executed; 3 atomic commits (feat restorable endpoint / feat restore UI / docs deploy)
- [x] `GET /api/v1/admin/restorable` (officer-only) lists evicted-in-grace owners; store test proves live + archived excluded, in-grace included
- [x] `EvictionForm` has a Restore section (list + ConfirmDialog + `restoreEviction`) with WR-02-correct copy (code retrieved server-side, not shown in-browser)
- [x] `deploy/squirebot-server.service` has the optional `EnvironmentFile` line; `docs/backend-deploy.md` documents the Phase 15 auth deploy
- [x] All verify commands green (Go test/build/cross-compile + web check/test/build)
- [x] No docs/ROADMAP commits in the code commits (PLAN/SUMMARY/STATE handled by orchestrator); deviations recorded below

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Doc accuracy] Section 7.5 originally documented a Caddyfile apex `file_server` block that does not exist in `deploy/Caddyfile`**
- **Found during:** Task 3 (verifying the doc against the actual `deploy/Caddyfile`).
- **Issue:** The first draft of §7.5 asserted "the apex block in `deploy/Caddyfile` uses `root * /var/www/squirebot` + `file_server`". The committed `deploy/Caddyfile` contains ONLY the `api.squirebot.quest` reverse-proxy block — no apex block — so this documented a fiction the operator would trust.
- **Fix:** Reworded §7.5 to state accurately that the repo Caddyfile currently holds only the `api` block, and to show the apex `squirebot.quest { root … / try_files /200.html / file_server }` block the operator must ADD. The Task 3 verify greps were unaffected (no change to the `squirebot.env` / `set-owner-floor` / `EnvironmentFile` lines).
- **Files modified:** `docs/backend-deploy.md`
- **Commit:** `2b2f263` (fixed before the commit)

### Other notes (not deviations)
- **`webadmin/eviction_test.go` not modified** (it appears in the plan's `files_modified` list). The plan's Task 1 action said the route-gate coverage could go in "main_test (or a webadmin test)"; I put it in `cmd/squirebot-server/main_test.go` to extend the existing `TestWriteRoutes_Gates`, which is the canonical officer-gate test and already wires the sibling admin routes — so no `webadmin/eviction_test.go` change was needed. The handler-logic for `RestorableListHandler` is a trivial mirror of the already-tested `EvictableListHandler`; the gate test covers the route.
- **CRLF normalization warnings** appeared on the web/docs commits ("LF will be replaced by CRLF") — benign Git line-ending notices on a Windows checkout; no content lost (verified via `git show --stat`, additions only, no spurious deletions).

## Known Stubs
None. The Restore section is fully wired end-to-end (real endpoint → real `fetchRestorable` → real `restoreEviction` → server-truth routing). The new code is intentionally NOT surfaced in-browser (WR-02 — it is retrieved server-side via journald / `mint-code`); the success copy states this explicitly. This is a security requirement, not a stub.

## Threat Flags
None. No new network endpoint surface beyond the officer-only `GET /api/v1/admin/restorable` (read-only list, `RequireOfficer` at the route, parameterized SQL). The destructive restore mutation (`RestoreHandler`) was unchanged and retains its in-tx officer re-check (WR-04). No new auth path, file access, or trust-boundary schema change.

## Self-Check: PASSED
- Commits `61b7861`, `6bfabd1`, `2b2f263` exist in `git log` (verified).
- All 11 modified files present with the described changes.
- `store.ListRestorableOwners`, `webadmin.RestorableListHandler`, `fetchRestorable`, `restoreResultMessage` all grep-confirmed in their files.
- `go test ./...` exit 0; web `npm run check` 0/0, 178/178, `build/index.html` emitted.
