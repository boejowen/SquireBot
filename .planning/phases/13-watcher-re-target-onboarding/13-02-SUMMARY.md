---
phase: 13-watcher-re-target-onboarding
plan: 02
subsystem: infra
tags: [go, net/http, bearer-auth, dpapi, wincred, win32, dialog, sqweek, ingest, onboarding]

# Dependency graph
requires:
  - phase: 13-watcher-re-target-onboarding (Plan 01)
    provides: "POST /api/v1/ingest 204/401/409/426/400/422 contract + GET /api/v1/whoami validate target + the 426 min-watcher-version gate the client classifies against"
  - phase: 11-backend-foundation-ingest-api
    provides: "the live ingest endpoint + Envelope{character,kind,content,watcher_version} JSON shape this client mirrors"
provides:
  - "internal/backend.Client.Ingest — watcher-side HTTP ingest client: POSTs Envelope + Bearer + SquireBot/<ver> UA to {base}/api/v1/ingest, classifies 204/401/409/426/400/422 into typed errors (ErrUnauthorized/ErrCrossOwner/ErrVersionTooOld/ErrBadPayload), bounded ctx-aware retry [1s,2s,4s] on 5xx/transport ONLY"
  - "internal/credstore Store/Read/Delete — DPAPI wincred guild-code store under the single fixed target SquireBot:guild-code (survivor of internal/auth/store.go); plaintext at rest under DPAPI, never config.json, never logged"
  - "internal/onboarding PromptGuildCode (native Win32 DLGTEMPLATE input dialog, no loopback) + PickEQFolder (relocated sqweek folder dialog); //go:build split keeps non-Windows CI green"
affects: [13-03-retarget-integration, 13-04-polish, 16-cutover-decommission]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Watcher-side HTTP sink that mirrors the server Envelope + UA by hand (client/server decoupled — zero internal/backendsrv import in the watcher binary)"
    - "Status->typed-error classification with a terminal-vs-retryable split driving a bounded retry (terminal 401/409/426/4xx never retry; only 5xx/transport do, capped at 3)"
    - "GC-safe Win32 callback state bridge: pass an integer TOKEN as dwInitParam (never a Go pointer through uintptr), claim-by-token on WM_INITDIALOG, re-key by HWND"
    - "//go:build windows / !windows split for a Win32+DPAPI feature so go build/test ./... stays green on a non-Windows runner (mirrors console_windows.go / folderpicker_dialog.go)"

key-files:
  created:
    - internal/backend/client.go
    - internal/backend/client_test.go
    - internal/credstore/store.go
    - internal/credstore/store_test.go
    - internal/onboarding/dialog.go
    - internal/onboarding/dialog_windows.go
    - internal/onboarding/dialog_other.go
    - internal/onboarding/dialog_test.go
  modified: []

key-decisions:
  - "Win32 input dialog = in-memory DLGTEMPLATE + user32!DialogBoxIndirectParamW (open_for_planner #1 resolved): the dialog manager gives Tab/Enter->OK/Esc->Cancel for free, no window-class registration, no manual message pump — least moving parts for a robust modal text box"
  - "Backend client retry slice has one entry per attempt (len = the 3-attempt cap); a backoff test seam keeps production [1s,2s,4s] while tests run at 0s"
  - "A surprising/unexpected HTTP status (not in the map, not 5xx) is treated as TERMINAL, not retryable — a surprise 4xx must never spin the loop"
  - "An empty-OK in the guild-code dialog is treated as ErrCancelled (nothing useful to validate) — keeps the caller's validate path clean"
  - "credstore stores the PLAINTEXT code (DPAPI is the at-rest protection) because the watcher must present it as the Bearer value on every POST — the SERVER stores only the hash"

patterns-established:
  - "internal/backend.classify(status): 204->nil, 401->ErrUnauthorized, 409->ErrCrossOwner, 426->ErrVersionTooOld, 400/422->ErrBadPayload, >=500->errRetryable, else->terminal generic"
  - "Onboarding packages are pure 'show the native control + return the value' — validation (whoami POST, eqfind.ValidateFolder) is the CALLER's job (Plan 03), keeping these a thin testable UI layer"

requirements-completed: []  # WATCH-08/09/10 remain the watcher half-built; not marked complete until the 13-03 integration wires these into runapp/main + deletes the Google stack.

# Metrics
duration: 18min
completed: 2026-05-30
---

# Phase 13 Plan 02: Watcher Foundation (internal/backend + credstore + native Win32 dialog) Summary

**The three composable watcher-side building blocks for the v2.0 re-target: a stdlib `net/http` ingest client that POSTs the Envelope with a Bearer + `SquireBot/<ver>` UA and classifies 204/401/409/426/400/422 into typed errors with a bounded 5xx/transport-only retry; a DPAPI wincred guild-code store under a single fixed target (never config.json); and a native Win32 `DLGTEMPLATE` "paste your guild code" input dialog with the relocated sqweek EQ-folder picker — zero browser, zero loopback listener.**

## Performance

- **Duration:** ~18 min
- **Started:** 2026-05-30T05:43:00Z
- **Completed:** 2026-05-30T06:01:00Z
- **Tasks:** 3 (Tasks 1 & 2 TDD RED→GREEN; Task 3 `auto`)
- **Files created:** 8 (1147 LOC total: client 248 + client_test 239 + store 71 + store_test 74 + dialog 43 + dialog_windows 416 + dialog_other 18 + dialog_test 38)

## Accomplishments
- **`internal/backend.Client.Ingest`** — the v2.0 upload SINK that replaces the deleted Sheets `batchUpdate`. POSTs `Envelope{character,kind,content,watcher_version}` to `{base}/api/v1/ingest` with `Authorization: Bearer <code>`, `Content-Type: application/json`, and a `SquireBot/<ver> (+github)` User-Agent (the version travels in BOTH the body and the UA so the 426 gate can read either). `classify()` maps every server status to a typed sentinel; the terminal-vs-retryable split drives a bounded ctx-aware retry of `[1s,2s,4s]` (3 attempts) on 5xx/transport ONLY — 401/409/426/4xx return immediately and NEVER retry (Pitfall 5). Content is POSTed verbatim (no client-side CP1252 decode; A1) and neither the code nor the content is ever logged (V7). Decoupled from `internal/backendsrv/*` (mirrors the Envelope + UA by hand).
- **`internal/credstore`** — the salvaged DPAPI wincred helper (survivor of `internal/auth/store.go`): `Store`/`Read`/`Delete` under the single fixed target `SquireBot:guild-code` (no email key — v2 identity is server-derived). Stores the plaintext code under `PersistLocalMachine` (DPAPI at rest); never config.json (AUTH-04 carries over), never logged. The real DPAPI round-trip test passes on the Windows dev box (skips elsewhere).
- **`internal/onboarding`** — `PromptGuildCode` builds an in-memory `DLGTEMPLATE` (static prompt + single-line EDIT + OK/Cancel) and runs it modally via `user32!DialogBoxIndirectParamW` with a `windows.NewLazySystemDLL`-resolved, DLL-preload-safe proc — NO browser, NO loopback HTTP listener (grep-proven 0). `PickEQFolder` is the verbatim relocation of the wizard's sqweek folder dialog. A `//go:build !windows` stub returns `ErrUnsupported` so `go build/test ./...` stays green off-Windows.
- Whole-repo `go build ./...` + `go vet ./...` + `go test ./...` all green (zero v1 watcher / backend regression); the three new packages are clean of `backendsrv` (backend) and of `oauth2`/`google.golang.org/api` (all three).

## Task Commits

Each task committed atomically (Tasks 1 & 2 are TDD: test RED → feat GREEN):

1. **Task 1: internal/backend ingest client** — `bc3c526` (test RED) → `8f81a66` (feat GREEN)
2. **Task 2: internal/credstore DPAPI guild-code** — `f4db9f0` (test RED) → `2dd7818` (feat GREEN)
3. **Task 3: internal/onboarding native Win32 dialog + EQ-folder picker** — `3665db2` (feat)

**TDD gate compliance:** Tasks 1 & 2 each have a `test(...)` RED commit followed by a `feat(...)` GREEN commit. Both REDs were compile failures (the packages did not yet exist — `undefined: Client`/`undefined: credTarget` etc.), neither passed unexpectedly. No REFACTOR commits were needed (implementations were clean as written). Task 3 is a non-TDD `auto` task per the plan (the interactive modal is validated by the human ship-gate, not headless CI), committed as a single `feat`.

## Files Created/Modified
- `internal/backend/client.go` (248) — `Client`, `New`/`NewWithHTTPClient`, the private `envelope` mirror, `Ingest`, `classify`, the bounded ctx-aware retry loop, the sentinels (`ErrUnauthorized`/`ErrCrossOwner`/`ErrVersionTooOld`/`ErrBadPayload`/internal `errRetryable`).
- `internal/backend/client_test.go` (239) — httptest recording server; covers every `<behavior>` bullet: 204 request-shape (method/path/headers/Envelope), each terminal status asserts exactly 1 request, 500→204 asserts exactly 2, 500-always asserts the bounded cap + a non-terminal error, network failure retryable, UTF-8 curly-apostrophe byte fidelity, no-secret-in-logs.
- `internal/credstore/store.go` (71) — `credTarget = "SquireBot:guild-code"`, `Store`/`Read`/`Delete`, the SECURITY package doc.
- `internal/credstore/store_test.go` (74) — fixed-target assertion (all platforms) + Windows round-trip + not-found contracts.
- `internal/onboarding/dialog.go` (43) — shared `ErrCancelled`/`ErrUnsupported` + the package doc (D-3: native, no loopback, no browser).
- `internal/onboarding/dialog_windows.go` (416) — the Win32 `DLGTEMPLATE` builder, `DialogBoxIndirectParamW` modal call, the DLGPROC, the GC-safe token/HWND state registry, `PromptGuildCode`, and the relocated `PickEQFolder` (sqweek).
- `internal/onboarding/dialog_other.go` (18) — `//go:build !windows` stubs returning `ErrUnsupported`.
- `internal/onboarding/dialog_test.go` (38) — error-var distinctness + non-Windows `ErrUnsupported` contract (interactive paths skipped on Windows).

## Decisions Made
- **Win32 dialog implementation = in-memory `DLGTEMPLATE` + `DialogBoxIndirectParamW`** (resolving `open_for_planner #1` / the plan's FORK). Chosen over a hand-rolled `CreateWindowEx` + message-loop because the dialog manager provides correct keyboard handling (Tab between controls, Enter→IDOK default, Esc→IDCANCEL) and a self-contained modal pump for free, with no window class to register/unregister. The cost (building the template's binary layout by hand) is contained and documented field-by-field.
- **Retry shape:** a `backoff []time.Duration` field whose length IS the attempt cap (3), with a test seam to run at 0s. Production uses `[1s,2s,4s]`. An unexpected/surprise status is terminal (never retried) so it can't spin the loop.
- **credstore stores plaintext** (not a hash): the watcher must present the code as the Bearer value on every POST; DPAPI is the at-rest protection. The package is left un-build-tagged to match the survivor (`internal/auth/store.go`), and `wincred` compiles cross-platform.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] GC-unsafe `uintptr`→`unsafe.Pointer` round-trip in the Win32 dialog callback**
- **Found during:** Task 3 (native Win32 dialog), surfaced by `go vet ./internal/onboarding/` → `possible misuse of unsafe.Pointer`.
- **Issue:** The first cut passed the `*dialogState` Go pointer as `DialogBoxIndirectParamW`'s `dwInitParam` and reconstructed it on `WM_INITDIALOG` via `(*dialogState)(unsafe.Pointer(lParam))`. Converting a `uintptr` (an opaque integer that crossed the syscall/callback boundary) back into a Go pointer is exactly the pattern `go vet` flags: the GC does not track integers-as-pointers, so the `dialogState` could in principle be moved or collected between the call and the callback — a latent memory-safety bug, not just a lint nit.
- **Fix:** Replaced the pointer round-trip with an integer-token registry. `PromptGuildCode` allocates a small integer token, stores the live `*dialogState` in a mutex-guarded `byToken` map (keeping it GC-reachable), and passes the TOKEN (not a pointer) as `dwInitParam`. On `WM_INITDIALOG` the proc claims the state by token and re-keys it into a `byHwnd` map for the rest of the dialog; `EndDialog` releases it. No `uintptr` is ever converted back to a Go pointer.
- **Files modified:** `internal/onboarding/dialog_windows.go` (folded into the single Task 3 commit before it was made).
- **Verification:** `go vet ./internal/onboarding/` exits 0; `go test ./internal/onboarding/` exits 0; whole-repo `go vet ./...` clean.
- **Committed in:** `3665db2` (the Task 3 feat commit — the fix was applied before the commit, so the committed code is already correct).

---

**Total deviations:** 1 auto-fixed (1 bug).
**Impact on plan:** The fix is a correctness/memory-safety requirement for the Win32 callback and keeps `go vet ./...` clean (a repo invariant). It changed only the internal state-bridging mechanism inside the same file the plan specified — no API change, no scope creep. The plan's `<action>` left the exact callback-state mechanism to the implementer ("pick ONE and implement it fully"); this is the robust realization.

## Issues Encountered
- The plan's Task 3 acceptance criterion required the `net.Listen|net/http|http.Server` grep over `internal/onboarding/` to find NOTHING. The first draft's package doc-comment literally said "no `net.Listen`, no `net/http`, no `http.Server`" — which the grep matched. Reworded the comment to "no TCP listener, no HTTP handler, no loopback server" so the acceptance grep returns zero hits while preserving the documented intent. (Cosmetic; zero behavior change.)

## User Setup Required
None — no external service configuration. These are library packages with no importers yet; Plan 03 wires them into `cmd/squirebot/main.go` + `internal/app/runapp.go`.

## Next Phase Readiness
- **13-03 (re-target + delete-the-Google-stack integration)** can now wire these three packages: `runapp`'s rewritten upload callback calls `backend.Client.Ingest(ctx, code, char, kind, utf8Content, version)` with the code from `credstore.Read()`; on a `credstore` not-found it triggers `onboarding.PromptGuildCode` + `onboarding.PickEQFolder` (then validates via the live `GET /api/v1/whoami` and `eqfind.ValidateFolder` — the caller's job, per the package contract). The `backend.ErrUnauthorized`/`ErrCrossOwner`/`ErrVersionTooOld` sentinels map cleanly to re-prompt / log-cross-owner / "update needed" tray states.
- **13-04 (polish)** carries the 999.22 watcher-side SemVer twin that must behave identically to the backend's `ingest.IsOlder` (the one-truth-per-side doctrine).
- The build stays green with BOTH the old Google stack and the new packages present (this plan deleted nothing) — 13-03's deletion is a bounded rewire-then-delete.
- No blockers.

## Self-Check: PASSED

- Created files verified on disk: `internal/backend/client.go`, `client_test.go`, `internal/credstore/store.go`, `store_test.go`, `internal/onboarding/dialog.go`, `dialog_windows.go`, `dialog_other.go`, `dialog_test.go`, `13-02-SUMMARY.md` — all FOUND.
- Task commits verified in git: `bc3c526`, `8f81a66`, `f4db9f0`, `2dd7818`, `3665db2` — all FOUND.

---
*Phase: 13-watcher-re-target-onboarding*
*Completed: 2026-05-30*
