---
phase: 13-watcher-re-target-onboarding
verified: 2026-05-30T07:05:00Z
status: passed
score: 5/5 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: none
  previous_score: n/a
human_verification:
  - test: "Clean install of the published GitHub-Releases watcher .exe on a fresh Windows machine, then paste a real guild code in the native dialog and pick the EQ folder"
    expected: "The native Win32 input dialog appears (no browser, no localhost), the code validates against /api/v1/whoami, the tray goes green, and an /outputfile inventory edit lands in the backend within seconds"
    why_human: "The Win32 DialogBoxIndirectParamW dialog is modal + interactive and is not headless-testable; the data-path underneath it (backend.Client.Ingest/Validate) was proven against LIVE prod via a tagged Go harness using the shipped client (204 ingest, 200/401 validate, 426 version-gate all confirmed against https://api.squirebot.quest)"
notes_non_blocking:
  - "gofmt: internal/onboarding/dialog_windows.go (a 13-02 file) + internal/backend/client_test.go (a 13-02 file) are NOT gofmt -l clean (cosmetic const-block / struct-literal alignment). Does NOT break build/vet/test and does NOT block the goal. The 13-03/13-04 SUMMARYs were transparent about this (never claimed whole-repo gofmt cleanliness; explicitly noted pre-commit hooks don't enforce gofmt). 9 OTHER non-clean files are pre-existing from P11/P12. Recommend a `gofmt -w` sweep in a future polish plan."
  - "Schema note: the deployed inventory_item table has NO watcher_version column (CONTEXT D-4 said it 'already ships'); the 426 gate reads env.WatcherVersion from the request envelope, not a stored column, so the gate works regardless. Not goal-affecting; flagged for doc accuracy."
---

# Phase 13: Watcher Re-Target + Onboarding — Verification Report

**Phase Goal:** Re-point the Go watcher from Google Sheets to the backend `POST /api/v1/ingest` API, DELETE the entire Google OAuth/PKCE/Sheets/Drive-Picker stack, replace the loopback-OAuth wizard with a native "paste your guild code" onboarding (no browser, no loopback), and ship via the existing GitHub-Releases auto-updater. Plus a small backend addition (`GET /api/v1/whoami` + a `426` min-watcher-version gate).
**Verified:** 2026-05-30T07:05:00Z
**Status:** passed (with 1 human-verification ship-gate item: the interactive clean-install dialog)
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (the 5 ROADMAP Success Criteria)

| # | Truth (Success Criterion) | Status | Evidence |
|---|---------------------------|--------|----------|
| 1 | **WATCH-08** — watcher POSTs inv/spellbook to the backend (not Sheets), RAW UTF-8 content, no client-side `parse.Parse`, 500ms debounce + always-re-read preserved | ✓ VERIFIED | `runapp.go:328,393` call `bc.Ingest(...)`; `parse.Parse` appears ONLY in comments (grep `parse\.Parse\b` → lines 16/286/313, all `//`); CP1252 decode-once via `parse.CP1252Reader` (lines 315/382) then `string(utf8Bytes)` POSTed; debounce lives in `internal/watch/debounce.go:67` `NewDebouncer(500 * time.Millisecond)` + `watcher.go` always-re-read, untouched. **LIVE: `Ingest(v2.0.0)` → 204 nil against prod; row queried back on the box.** |
| 2 | **WATCH-09** — no Google OAuth/PKCE/Sheets/Drive-Picker code; build bakes no Google secret; binary materially smaller; `internal/parse` preserved | ✓ VERIFIED | `internal/{auth,sheet,scaffold,picker,wizard,heartbeat}` + `internal/app/reauth.go` GONE (dir checks + `ls` confirm); grep for those import paths → comments only, **zero importers**; `go list -deps ./cmd/squirebot` → no oauth2/sheets/google.golang.org/api/drive; `go.mod` Google-free; `release.yml` + `build_constants.go` ldflags inject ONLY `Version`+`BackendBaseURL`; **built binary 7,408,640 B (7.06 MB) vs v1 16.44 MB; 9-pattern byte-scan = 0 Google/OAuth/Sheets/secret hits**; `internal/parse` preserved + imported by `runapp.go` (CP1252Reader) and `backendsrv/ingest/handler.go` (server parse). |
| 3 | **WATCH-10** — native "paste guild code" onboarding, validates `/whoami`, stores bearer in DPAPI, no browser OAuth | ✓ VERIFIED | `internal/onboarding/dialog_windows.go` = pure Win32 `DialogBoxIndirectParamW` + `NewLazySystemDLL`; grep for `net.Listen|ListenAndServe|http.Server|127.0.0.1|localhost|loopback` across whole watcher → **0 code hits** (comments only); `credstore.go` stores plaintext under fixed `SquireBot:guild-code` via wincred `PersistLocalMachine`, never config.json; `runOnboarding` calls `bc.Validate(ctx,code)`→`/api/v1/whoami`; no `exec.Command`/`ShellExecute`/URL-launch except `explorer.exe` for the log folder. **LIVE: `Validate(good)` → 200 nil; `Validate(bad)` → 401 ErrUnauthorized.** |
| 4 | **WATCH-11** — first-launch `MigrateFromV1` deletes stale Google wincred + drops dead config fields, idempotent, preserves EQFolders+mtime; auto-update transport unchanged | ✓ VERIFIED | `migrate.go::MigrateFromV1` re-reads raw config.json, idempotency sentinel (both v1 keys absent → nil), deletes `SquireBot:<google-email>` wincred best-effort, `cfg.Save()` drops `google_email`/`spreadsheet_id`; operates on already-loaded cfg so EQFolders + LastKnown*Mtime untouched; **CALLED in `main.go:114`** (non-fatal); `internal/update` selfupdate (manifest/check/swap) untouched, direct net/http to GitHub. `config.go` documents the removed fields + adds `BackendBaseURL`. |
| 5 | **SC-5 / version gate** — watcher sends version (body+UA), backend rejects too-old with 426; watcher-side SemVer compare agrees | ✓ VERIFIED | `handler.go:119` `if env.WatcherVersion != "" && IsOlder(env.WatcherVersion, minWatcherVersion)` → 426 + human message, placed post-decode/pre-store; `version.go::IsOlder` SemVer-pre-release-aware (fail-closed bad-present, fail-open bad-floor); watcher twin `update/manifest.go::IsNewer` SemVer-pre-release-aware, separate copy, no cross-import; client sends version in BOTH envelope + `SquireBot/<version>` UA. **LIVE: `Ingest(v0.0.1)` → 426 ErrVersionTooOld against prod.** |

**Score:** 5/5 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/backend/client.go` | watcher HTTP ingest client (POST envelope + bearer + version UA + classify + bounded retry) | ✓ VERIFIED | `Ingest` + `Validate`; 204→nil, 401/409/426/400+422 typed sentinels, 5xx→bounded retry `[1s,2s,4s]`, never retries terminal; never logs code/content |
| `internal/credstore/store.go` | DPAPI guild-code Store/Read/Delete | ✓ VERIFIED | fixed `SquireBot:guild-code`, `PersistLocalMachine`, plaintext (needed for Bearer), never config.json |
| `internal/onboarding/dialog_windows.go` | native Win32 input dialog, no loopback | ✓ VERIFIED | `DialogBoxIndirectParamW` + `NewLazySystemDLL`; `PromptGuildCode` + `PickEQFolder`; zero network surface |
| `internal/app/runapp.go` | watch→read→POST sink + onboarding branch + 401/426 handling; no Sheets/OAuth | ✓ VERIFIED | `backend.New`, `bc.Ingest`, `credstore.Read`/`onboarding.*`, HealthGreen on success |
| `internal/app/migrate.go` | first-launch v1→v2 wincred+config migration | ✓ VERIFIED | `MigrateFromV1`, idempotent, wired in main.go |
| `internal/backendsrv/ingest/whoami.go` | authed `/whoami` reusing `auth.ResolveToken` | ✓ VERIFIED | 200+label / 401, read-only single SELECT |
| `internal/backendsrv/ingest/version.go` | SemVer `IsOlder` server gate truth | ✓ VERIFIED | pre-release aware, asymmetric fail-closed/open |
| `internal/backendsrv/ingest/handler.go` | 426 reject post-decode pre-store | ✓ VERIFIED | `StatusUpgradeRequired` at line 121 |
| `cmd/squirebot/build_constants.go` | gutted — Version + BackendBaseURL only | ✓ VERIFIED | no OAuth/Picker vars |
| `cmd/squirebot/console_windows.go` | gofmt-clean + freeConsole doc==impl (999.20/21) | ✓ VERIFIED | aligned var block, `ret==0`→Debug+nil, doc reconciled, no `fmt` import |
| `internal/update/manifest.go` | SemVer-pre-release `IsNewer` (999.22) | ✓ VERIFIED | watcher twin of `IsOlder`, defensive false-on-corrupt preserved |
| `cmd/squirebot-server/main.go` | `/whoami` + `/ingest` route registration | ✓ VERIFIED | `mux.Handle` lines 259-260 |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| runapp.go | backend.Client.Ingest | rewritten callbacks POST instead of WriteInventory | ✓ WIRED | `bc.Ingest(...)` at 328/393 |
| runapp.go | credstore + onboarding | `credstore.Read` branch + native prompt | ✓ WIRED | lines 69, 116, 158 |
| migrate.go | stale Google wincred | delete `SquireBot:<email>` once | ✓ WIRED | line 80-81 |
| whoami.go | auth.ResolveToken | `guard.ResolveToken` | ✓ WIRED | line 66 |
| handler.go | version.go | `IsOlder(env.WatcherVersion, minWatcherVersion)` | ✓ WIRED | line 119 |
| client.go | `{base}/api/v1/ingest` | POST Bearer + Envelope JSON | ✓ WIRED | line 188; **LIVE 204** |
| client.go | `{base}/api/v1/whoami` | GET Bearer | ✓ WIRED | line 126; **LIVE 200/401** |
| main.go | app.MigrateFromV1 | first-launch call | ✓ WIRED | line 114 |
| main.go | app.RunApp | background goroutine | ✓ WIRED | line 165 |

### Data-Flow Trace (Level 4 — LIVE)

| Path | Source | Produces Real Data | Status |
|------|--------|--------------------|--------|
| watcher → POST /api/v1/ingest → SQLite | shipped `backend.Client.Ingest` against live prod | Yes — row `VerifyP13Toon / General1 / VerifyP13 Test Rusty Dagger / item 5076` queried back on the box; first-sighting bind created owner+character | ✓ FLOWING |
| onboarding → GET /api/v1/whoami → auth | shipped `backend.Client.Validate` against live prod | Yes — good code 200, bad code 401, revoked code 401 | ✓ FLOWING |
| version gate → 426 | shipped client `Ingest(v0.0.1)` against live prod | Yes — 426 ErrVersionTooOld returned, store untouched | ✓ FLOWING |

### Behavioral Spot-Checks (LIVE against https://api.squirebot.quest, via the SHIPPED internal/backend.Client behind a `verifylive` build tag)

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Valid code validates | `Client.Validate(good)` | nil (HTTP 200) | ✓ PASS |
| Invalid code rejected | `Client.Validate(bad)` | ErrUnauthorized (401) | ✓ PASS |
| Current-version upload accepted | `Client.Ingest(v2.0.0)` | nil (204); row landed | ✓ PASS |
| Too-old version rejected | `Client.Ingest(v0.0.1)` | ErrVersionTooOld (426) | ✓ PASS |
| Bad bearer on ingest rejected | `Client.Ingest(bad code)` | ErrUnauthorized (401) | ✓ PASS |
| Revocation effective | `curl /whoami` w/ revoked code | HTTP 401 | ✓ PASS |
| Whole watcher suite | `go build/vet/test ./...` | green (22 test pkgs) | ✓ PASS |

All prod test artifacts cleaned up: throwaway code `VerifyP13` revoked; owner/character/inventory rows for VerifyP13 deleted (verified 0 residual); local harness `verify_live_test.go` + scratch binary removed; `git status` clean (only pre-existing `.claude/`).

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|----------------|-------------|--------|----------|
| WATCH-08 | 13-01, 13-02, 13-03 | watcher uploads inv/spellbook to backend (not Sheets), 500ms debounce + always-re-read preserved | ✓ SATISFIED | Truth 1 + LIVE 204 |
| WATCH-09 | 13-03, 13-04 | all Google OAuth/PKCE/Sheets/Picker machinery removed; no secret; smaller binary | ✓ SATISFIED | Truth 2 + byte-scan + dep tree |
| WATCH-10 | 13-02, 13-03 | "paste guild code" onboarding, DPAPI bearer, no browser OAuth | ✓ SATISFIED | Truth 3 + LIVE 200/401 + zero-loopback grep |
| WATCH-11 | 13-03, 13-04 | existing watchers auto-update; one manual step; stale Google state cleaned | ✓ SATISFIED | Truth 4 (migration wired + idempotent) |

PLAN frontmatter `requirements:` (13-01 [08,09,10], 13-02 [08,09,10], 13-03 [08,09,10,11], 13-04 [09,11]) reconcile exactly with REQUIREMENTS.md's P13 mapping (WATCH-08/09/10/11). No orphaned requirements.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| internal/onboarding/dialog_windows.go | const block | gofmt alignment | ℹ️ Info | Cosmetic; build/test green; SUMMARYs disclosed it |
| internal/backend/client_test.go | struct literal | gofmt alignment | ℹ️ Info | Cosmetic; test-only; SUMMARYs disclosed it |
| internal/onboarding/dialog_other.go | 11-17 | stub returning ErrUnsupported | ℹ️ Info (not a real stub) | Correctly build-tagged `!windows`; never reached in prod (watcher is Windows-only; real dialog_windows.go compiles on the dev box) |

No 🛑 blockers, no ⚠️ goal-affecting warnings. No TODO/FIXME/placeholder/not-implemented in any production code path.

### Human Verification Required

#### 1. Clean-install ship-gate (the interactive half of the ROADMAP ship gate)

**Test:** On a fresh Windows machine, install the published GitHub-Releases watcher `.exe`, paste a real guild code into the native Win32 dialog, and select the EverQuest folder; then trigger an `/outputfile inventory` in EQ.
**Expected:** The native input dialog appears (NO browser, NO localhost listener), the code validates, the tray icon goes green, and the inventory snapshot lands in the backend within seconds.
**Why human:** `DialogBoxIndirectParamW` is a modal interactive dialog — not headless-testable. The entire data path UNDER the dialog (`backend.Client.Ingest`/`Validate`, the 426 gate) was proven against LIVE prod with the shipped client (204 ingest + row-back, 200/401 validate, 426 too-old). What remains untested-by-machine is only the GUI rendering + paste + clean-machine auto-update swap.

### Gaps Summary

No goal-blocking gaps. All 5 ROADMAP success criteria are VERIFIED in the codebase and, where a live backend exists, proven end-to-end against prod (`https://api.squirebot.quest`) using the shipped `internal/backend.Client`:

- The watcher's terminal SINK is the backend ingest API; the Sheets/OAuth/Picker stack (6 packages + reauth.go) is deleted with no importers; the binary is 57% smaller and carries zero Google strings.
- Onboarding is a native Win32 dialog with zero loopback/HTTP-server surface anywhere in the watcher; the bearer lives only in DPAPI; validation hits `/whoami` (live 200/401).
- The first-launch migration is wired, idempotent, and preserves EQ state.
- The SC-5 version gate fires live (426 on a fabricated old version; 401 on a bad code).

The only item routed to a human is the interactive clean-install + paste ship-gate (the modal GUI dialog is not headless-testable). The two gofmt nits and the absent `watcher_version` column are non-blocking notes (build/vet/test green; SUMMARYs were transparent about the gofmt state).

---

_Verified: 2026-05-30T07:05:00Z_
_Verifier: Claude (gsd-verifier)_
