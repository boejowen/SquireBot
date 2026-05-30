---
phase: 13-watcher-re-target-onboarding
plan: 03
subsystem: infra
tags: [go, net/http, bearer-auth, dpapi, wincred, ingest, onboarding, deletion, go-mod-tidy, ci]

# Dependency graph
requires:
  - phase: 13-watcher-re-target-onboarding (Plan 01)
    provides: "GET /api/v1/whoami validate endpoint + the 426 min-watcher-version gate the watcher classifies against"
  - phase: 13-watcher-re-target-onboarding (Plan 02)
    provides: "internal/backend.Client.Ingest + internal/credstore (DPAPI guild-code) + internal/onboarding (native Win32 dialog + relocated sqweek EQ-folder picker)"
  - phase: 11-backend-foundation-ingest-api
    provides: "the live POST /api/v1/ingest endpoint + internal/parse (shared, server-imported — NOT deleted)"
provides:
  - "Re-targeted watcher upload SINK: on a file change the callback re-reads, CP1252-decodes ONCE via parse.CP1252Reader, and POSTs the raw UTF-8 content to backend.Ingest — NO client-side parse.Parse (the server parses; D-1/D-8). The fsnotify 500ms debounce + always-re-read survive verbatim."
  - "Native first-run onboarding: tray 'Enter guild code…' → onboarding.PromptGuildCode → backend.Validate(/whoami) → credstore.Store → onboarding.PickEQFolder → eqfind.ValidateFolder → cfg.Save. Zero browser, zero loopback (D-3)."
  - "401 terminal handling (re-prompt + tray red, NO retry loop — D-5/Pitfall 5); 426 'update needed'; 409 cross-owner logged-and-skipped."
  - "WATCH-11 first-launch v1→v2 migration (app.MigrateFromV1): deletes the stale SquireBot:<google-email> wincred entry + drops the dead config.json fields (google_email, spreadsheet_id); idempotent; preserves EQFolders + LastKnown*Mtime maps."
  - "DELETION of the ~8k-LOC Google stack: internal/auth, internal/sheet, internal/scaffold, internal/picker, internal/wizard, internal/heartbeat + internal/app/reauth.go gone; go mod tidy drops the oauth2/google.golang.org/api dependency tree; NO Google secret baked into the build (OAuth ldflags stripped from build_constants.go + release.yml)."
affects: [13-04-polish, 14-web-frontend, 16-cutover-decommission]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Rewire-then-delete: every importer of a doomed package is re-pointed at the new client/credstore/onboarding/migration BEFORE the package is git-rm'd, so go build ./... is green at every commit (the tree never fails to compile)"
    - "Cross-package test seam without exposing internals: app-side tests redirect config.Path() by t.Setenv(LOCALAPPDATA, tmp) (config resolves %LOCALAPPDATA%) instead of reaching into config's unexported pathFn; backend.Client.SetBackoffForTest exposes the retry cap to out-of-package callback tests"
    - "First-launch idempotent migration keyed off a sentinel (both v1-only keys absent ⇒ already migrated/fresh ⇒ no-op), re-reading the RAW config.json to recover fields the new struct dropped"

key-files:
  created:
    - internal/app/migrate.go
    - internal/app/migrate_test.go
  modified:
    - internal/app/runapp.go
    - internal/app/runapp_test.go
    - internal/backend/client.go
    - internal/backend/client_test.go
    - internal/config/config.go
    - internal/config/config_test.go
    - cmd/squirebot/main.go
    - cmd/squirebot/build_constants.go
    - internal/tray/tray.go
    - internal/tray/tray_test.go
    - .github/workflows/release.yml
    - go.mod
    - go.sum
  deleted:
    - internal/auth/ (entire package)
    - internal/sheet/ (entire package)
    - internal/scaffold/ (entire package)
    - internal/picker/ (entire package)
    - internal/wizard/ (entire package)
    - internal/heartbeat/ (entire package)
    - internal/app/reauth.go + reauth_test.go

key-decisions:
  - "reauth.go + reauth_test.go were git-rm'd in TASK 1 (not Task 3): the OAuth-specific reauth state machine consumed the helpers (buildTokenSourceFromWincred) that the runapp.go rewrite deleted, so leaving it for Task 3 would have left internal/app non-compiling at the Task-1 boundary. The plan's deletes-frontmatter already listed both files; pulling their removal one task earlier is the faithful rewire-then-delete ordering."
  - "Onboarding base URL resolves cfg.BackendBaseURL (advanced/self-host override) else the hardcoded build_constants.go default https://api.squirebot.quest — the canonical path needs no override (A6/D-5)."
  - "MigrateFromV1 re-reads the RAW config.json (not cfg) because 1b removed the SpreadsheetID/GoogleEmail struct fields, so the in-memory cfg can no longer surface the v1 values; the raw read recovers google_email to target the stale wincred delete, then cfg.Save() (new struct) rewrites config.json without the dead keys."
  - "Tray collapsed Continue-setup / Reauthorize / Change-Workbook / Open-Workbook into a single always-visible 'Enter guild code…' item (re-running RunApp re-enters onboarding via its credstore.Read branch) — a re-prompt is always allowed, so no Show/Hide visibility machinery is needed (D-3)."
  - "The 8 dangling 'mirrors internal/auth|sheet|heartbeat…' provenance doc-comments in the SURVIVING backend files (store/binding.go, store/replace.go, store/db.go, scheduler.go, auth/mint.go, auth/store.go, auth/mint_test.go) are LEFT as known-stale: they document design lineage (the pattern these files were transliterated from), not a live dependency; refreshing them would churn P11/P12-authored code for marginal value (checker WARNING is non-blocking)."

patterns-established:
  - "Watcher upload callback: re-stat (mtime BEFORE read) → os.Open → io.ReadAll(parse.CP1252Reader(f)) → skip-if-bytes.TrimSpace-empty → backend.Ingest(ctx, code, char, kind, utf8, version) → status switch (401 red+terminal, 426 update-needed, 409 log, other warn) → on success persist cfg.LastKnown*Mtime[char]+cfg.Save()"
  - "Field-drop migration: removing a struct field is the on-disk drop (encoding/json ignores the now-unknown key on Load; the next Save omits it); the companion wincred cleanup runs once via the idempotency sentinel"

requirements-completed: [WATCH-08, WATCH-09, WATCH-10, WATCH-11]

# Metrics
duration: 17min
completed: 2026-05-30
---

# Phase 13 Plan 03: Watcher Re-Target Integration + Google-Stack Deletion Summary

**The watcher's terminal SINK is re-pointed from Google Sheets to the live backend ingest API (raw UTF-8 POST, server parses — no client-side parse.Parse), first-run onboarding is a native "paste your guild code" dialog (validate via /whoami → DPAPI → EQ folder, zero browser/loopback), a WATCH-11 first-launch migration purges the stale Google credential + dead config fields, and the entire ~8k-LOC Google OAuth/PKCE/Sheets/Drive-Picker stack (5 packages + reauth.go + heartbeat) is DELETED — `go build/vet/test ./...` green, `go list -deps ./cmd/squirebot` Google-free, no Google secret in the build.**

## Performance

- **Duration:** ~17 min
- **Started:** 2026-05-30T06:07:13Z
- **Completed:** 2026-05-30T06:24:08Z
- **Tasks:** 3 (Tasks 1 & 2 TDD RED→GREEN; Task 3 `auto`)
- **Files modified:** 14 modified/created + 6 packages (41 files) + reauth.go/reauth_test.go deleted

## Accomplishments
- **SINK swap (WATCH-08, D-1/D-8):** `internal/app/runapp.go` rewritten — `makeOnInventoryChange`/`makeOnSpellbookChange` now re-stat → `os.Open` → `io.ReadAll(parse.CP1252Reader(f))` (decode ONCE) → skip-empty → `backend.Ingest(ctx, code, char, kind, string(utf8Bytes), version)`. The watcher NO LONGER calls `parse.Parse` on the upload path (grep-proven: `parse.Parse(` is absent from runapp.go); it POSTs the RAW UTF-8 and the server parses. The fsnotify 500ms debounce + always-re-read (`internal/watch`) + `rescanCatchUp` are untouched. Error switch: 401 → tray red + "re-enter" TERMINAL (no retry — Pitfall 5), 426 → "update needed", 409 → logged-and-skipped, other → warn; on 204 persist the mtime + `cfg.Save()`.
- **Native onboarding (WATCH-10, D-3):** `runOnboarding` loops `onboarding.PromptGuildCode` → `backend.Validate` (GET /api/v1/whoami) → on 200 `credstore.Store` + `onboarding.PickEQFolder` → `eqfind.ValidateFolder` (verbatim "doesn't look like an EverQuest install" re-prompt) → `cfg.Save`; on 401 re-prompts. Zero browser, zero loopback. `backend.Client.Validate` added (200→nil, 401→ErrUnauthorized, else→generic; one-shot, no retry).
- **First-launch migration (WATCH-11, D-7):** `internal/app/migrate.go` `MigrateFromV1` re-reads the raw config.json, deletes the stale `SquireBot:<google-email>` wincred entry, and `cfg.Save()`s the new struct (dropping `google_email`/`spreadsheet_id`); idempotent (both-keys-absent sentinel); preserves EQFolders + mtime maps (Pitfall 4). Wired into `main.go` after `config.Load`, non-fatal.
- **main.go + build_constants + tray rewire (WATCH-09, D-2/D-3):** `main.go` drops `internal/auth`/`auth.BuildConstants`, routes `--uninstall-wipe-credentials` to `credstore.Delete()`, calls `MigrateFromV1`, and uses the new `RunApp(ctx, cfg, baseURL, Version, trayCtl)` signature. `build_constants.go` GUTTED to `Version` + `BackendBaseURL` (no Google secret). `tray.go` sheds Open-Workbook/Change-Workbook/Reauthorize/Continue-setup and adds a single always-visible "Enter guild code…" item.
- **DELETION (WATCH-09, D-2/D-10):** `git rm -r` of `internal/{auth,sheet,scaffold,picker,wizard,heartbeat}` (41 files) + the already-removed `reauth.go`/`reauth_test.go`. `internal/parse` PRESERVED (the backend imports it). `go mod tidy` dropped the entire Google dependency tree. `.github/workflows/release.yml` stripped of the four OAuth `-X` ldflags + the `OAUTH_CONFIG_JSON` materialize step + the `consent_screen_status` PRODUCTION gate; `binary_url`/`binary_sha256` preserved.
- Whole-repo `go build ./...` + `go vet ./...` + `go test ./...` all green (22 test packages, 0 fail — the P11/P12 backend tree is untouched); `go list -deps ./cmd/squirebot` carries NO oauth2/google.golang.org/api/cloud.google.com (and not even `database/sql/driver` — the sqlite dep stays server-side).

## Task Commits

Each task was committed atomically (Tasks 1 & 2 are TDD: a genuine compile-failure RED was captured in the test run, then GREEN — see TDD Gate Compliance):

1. **Task 1: Re-target the sink + onboarding + config migration + Validate** — `81172b1` (feat)
2. **Task 2: Rewire main.go + build_constants + tray** — `e66d9ec` (feat)
3. **Task 3: DELETE the Google stack + strip OAuth ldflags + go mod tidy** — `d2e8025` (feat)

**Plan metadata:** (this SUMMARY + STATE.md + ROADMAP.md + REQUIREMENTS.md) committed separately.

## Files Created/Modified
- `internal/app/runapp.go` — the re-targeted orchestrator: `RunApp` (credstore.Read branch), `runOnboarding`, `pickAndSaveEQFolder`, `runWatcher` (backend sink, no sheet/scaffold/heartbeat), `makeOnInventoryChange`/`makeOnSpellbookChange` (read→POST), `rescanCatchUp` + `extractCharName*` (survivors).
- `internal/app/migrate.go` (+migrate_test.go) — `MigrateFromV1` (WATCH-11), 4 tests (drops-keys-preserves-rest, idempotent, no-config-file, already-v2).
- `internal/backend/client.go` (+client_test.go) — `Validate` (GET /api/v1/whoami) + `SetBackoffForTest` seam; 4 Validate httptest cases.
- `internal/config/config.go` (+config_test.go) — dropped `SpreadsheetID`/`GoogleEmail`, added `BackendBaseURL`; tests now assert the keys drop on Save + a v1 config (with the keys) still Loads.
- `cmd/squirebot/main.go` — OAuth wiring removed, migration call added, new RunApp signature, baseURL resolution.
- `cmd/squirebot/build_constants.go` — `Version` + `BackendBaseURL` only.
- `internal/tray/tray.go` (+tray_test.go) — Phase-13 menu (Open log folder / Check for updates / Enter guild code… / Quit); `OnEnterGuildCode`.
- `.github/workflows/release.yml` — OAuth ldflags + materialize step + PRODUCTION gate stripped; Version+BackendBaseURL stamped; manifest contract preserved; doc/body re-framed as the backend watcher.
- `go.mod` / `go.sum` — Google dependency tree dropped.

## Decisions Made
See frontmatter `key-decisions`. Headlines: reauth.go removal pulled into Task 1 (rewire-then-delete integrity); baseURL = config-override-else-build-default; MigrateFromV1 raw-reads config.json to recover the dropped fields; tray collapses all setup/recovery items into one always-visible "Enter guild code…"; the 8 dangling backend provenance doc-comments left as known-stale.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Deleted reauth.go/reauth_test.go in Task 1 instead of Task 3**
- **Found during:** Task 1 (runapp.go rewrite)
- **Issue:** The `runapp.go` rewrite deletes `buildTokenSourceFromWincred` (and the OAuth globals), but `internal/app/reauth.go` *consumes* them and imports the deleted `auth`/`picker`/`sheet` packages. Leaving reauth.go in place until Task 3 (as the plan's task ordering nominally implied) would have left `internal/app` non-compiling at the Task-1 verification boundary (`go test ./internal/app/`).
- **Fix:** `git rm internal/app/reauth.go internal/app/reauth_test.go` during Task 1. Both files are OAuth-specific dead code with no v2 analog (a bad guild code is a re-prompt, not a 2-phase reauth) and were ALREADY listed in the plan's `deletes:` frontmatter — only their removal *timing* moved one task earlier, which is the faithful rewire-then-delete order.
- **Files modified:** removed `internal/app/reauth.go`, `internal/app/reauth_test.go`.
- **Verification:** `go test ./internal/app/ ./internal/config/ ./internal/backend/` exits 0 at the Task-1 boundary; whole-repo green after Task 3.
- **Committed in:** `81172b1` (Task 1 commit).

---

**Total deviations:** 1 auto-fixed (1 blocking).
**Impact on plan:** The fix preserves the load-bearing "tree never fails to compile" invariant the plan's ORDER section mandates; it moved an already-planned deletion one task earlier and introduced no scope change. No new files, no behavior change beyond what the plan specified.

## Issues Encountered
- **Cross-package config path seam:** the migration test lives in package `app` and cannot reach `config`'s unexported `pathFn` test seam. Resolved by `t.Setenv("LOCALAPPDATA", tmpdir)` (config's `defaultPath()` resolves `%LOCALAPPDATA%\SquireBot\config.json`) — a clean redirect with no new production API. The same env-redirect doubles as the seam for the `makeOnInventoryChange` callback tests (which `cfg.Save()` to disk).
- **`backend.Client.backoff` is package-private:** the `app`-side callback tests needed near-zero retry timing. Added an exported `SetBackoffForTest` seam (Rule-3-adjacent, but a deliberate test affordance, not a production path) so the 401/426 single-request assertions don't sleep.
- **Pre-existing gofmt nits (out of scope, NOT fixed):** `gofmt -l` flags several files this plan did not author (console_windows.go [999.20, deferred to 13-04], cmd/squirebot-server/main.go [logged to deferred-items.md by 13-01], and a handful of P11/P12 backend files). Per the SCOPE BOUNDARY they were left untouched; the repo's pre-commit hooks do not enforce gofmt (Task 1 committed cleanly with the pre-existing recordingServer alignment nit in the already-13-02-committed client_test.go).

## Known Stubs
None — the watcher fully wires the live pipeline (read → CP1252-decode → POST → backend). The onboarding controls (`PromptGuildCode`/`PickEQFolder`) are real native dialogs from 13-02; validation is wired (`backend.Validate` → /whoami). No placeholder data, no "coming soon", no TODO/FIXME in the changed watcher files.

## Threat Flags
None — this plan is a net attack-surface REDUCTION (deletes the loopback OAuth/picker HTTP listeners, the Drive Picker, the drive.file scope, the publicly-baked OAuth client secret, and the refresh-token store). All `mitigate` dispositions in the plan's threat register are realized and test-backed: T-13.03-01 (stale-credential delete — migrate_test), T-13.03-02 (no baked Google secret — release.yml grep clean + go mod tidy), T-13.03-03 (401 terminal no-loop — callback test asserts exactly 1 request), T-13.03-04 (no double-decode — `parse.Parse(` absent from runapp.go), T-13.03-06 (internal/parse preserved — whole-repo build green), T-13.03-07 (EQFolders/mtime preserved — migrate_test). No NEW security-relevant surface introduced.

## User Setup Required
None — no external service configuration. The backend (`/whoami` + 426 gate) shipped in 13-01 and rides the already-pending P12+13-01 VPS redeploy. The re-targeted binary's GitHub-Releases publish (so existing watchers auto-update) is the P13 rollout / P16 coordinated-flip concern, not a code/config prerequisite of this plan.

## Next Phase Readiness
- **13-04 (polish)** carries the 999.20 (`console_windows.go` gofmt) + 999.21 (`freeConsole()` doc) + 999.22 (watcher-side SemVer twin of `ingest.IsOlder`) ride-alongs, plus the explicit binary-size + no-secret confirmation (the dependency-tree drop here is the structural signal; 13-04 measures it).
- **Requirements WATCH-08/09/10/11 are now COMPLETE** (the backend half landed in 13-01/13-02; the watcher re-target + onboarding + DPAPI storage + migration land here). Marked complete in REQUIREMENTS.md.
- The live data pipeline is restored end-to-end at the code level: watcher → `POST /api/v1/ingest` → backend parse + self-populating DB (P12). The actual guild-wide flip is P16 (CUTOVER-03).
- No blockers.

## Self-Check: PASSED

- Created files verified on disk: `internal/app/migrate.go`, `migrate_test.go`, `internal/backend/client.go` (+Validate), `internal/app/runapp.go`, `cmd/squirebot/build_constants.go`, `internal/tray/tray.go`, `13-03-SUMMARY.md` — all FOUND.
- Deleted packages verified absent: `internal/{auth,sheet,scaffold,picker,wizard,heartbeat}` — all GONE; `internal/parse` (survivor) still present.
- Task commits verified in git: `81172b1`, `e66d9ec`, `d2e8025` — all FOUND.
- Gates: `go build ./...` exit 0, `go vet ./...` exit 0, `go test ./...` = 22 packages all green (zero broken); `go list -deps ./cmd/squirebot` clean of oauth2/google.golang.org/api/cloud.google.com; release.yml grep clean of OAuthClientSecret/consent_screen_status/OAUTH_CONFIG_JSON with binary_url/binary_sha256 preserved.

---
*Phase: 13-watcher-re-target-onboarding*
*Completed: 2026-05-30*
