---
phase: 01-end-to-end-thin-slice
plan: 07
subsystem: app-integration
tags: [wizard, tray, runapp, integration, end-to-end, html-template, systray, sqweek-dialog]
requires:
  - 01-01: cmd/squirebot/{main,build_constants,icon}.go skeleton + assets/icon.ico
  - 01-02: oauth-config.json (Production-mode Google Cloud project)
  - 01-03: auth.NewManagerWithListener / AttachRoutes / AuthURL / DoneChan / OAuthConfigForRefresh / ReadToken / OpenBrowser / BuildConstants
  - 01-04: watch.Run / parse.Parse / eqfind.Discover / eqfind.ValidateFolder
  - 01-05: sheet.NewClient / SetSpreadsheetID / ValidateWorkbook / WriteInventory / UpsertCharOwner / InventoryHeader / ErrWrongWorkbook / ErrSchemaTooNew
  - 01-06: picker.NewServer / SetRedirectAfterPick / OnPicked / AttachRoutes
provides:
  - internal/app.RunApp(ctx, cfg, bc, *tray.Controller) — wizard-or-watcher orchestrator
  - internal/app.ChangeWorkbook(ctx, cfg, bc, *tray.Controller) — D-04 picker re-launch on existing token
  - internal/wizard.Server + wizard.NewServer + wizard.Result + Run(ctx) — loopback HTTP wizard
  - internal/wizard FolderPicker / SheetClientFactory / PickerAttacher / BrowserOpener hook seams
  - internal/tray.Controller + NewController + Config + Health{Green,Red} — systray UI
  - cmd/squirebot/main.go — runnable entry point that composes everything
affects:
  - downstream Plan 08: NSIS installer wraps the dist/squirebot.exe this plan produces; Plan 08's smoke checkpoint validates the live tray + wizard + watcher round-trip end-to-end on a Win11 VM
tech-stack:
  added:
    - "fyne.io/systray (already pinned by Plan 01-01) — wired in tray.go"
    - "github.com/sqweek/dialog (already pinned by Plan 01-01) — wired in wizard/folderpicker_dialog.go"
    - "html/template (stdlib) — wizard/server.go three-page renderer"
  patterns:
    - "Loopback shared listener: auth.Manager + picker.Server + wizard handlers all attached to the same *http.ServeMux on a single 127.0.0.1:0 listener — INST-03 single-tab user journey"
    - "Hook-seam dependency injection (FolderPicker/SheetClientFactory/PickerAttacher/BrowserOpener) so wizard tests run in <2s with zero network calls"
    - "go:embed embed.FS for the three wizard HTML pages — single-binary install preserved"
    - "regexp.MustCompile package-level for ^(.+)-Inventory\\.txt$ char-name extraction (compile once, match on every fsnotify event)"
    - "Refresh-only TokenSource via auth.OAuthConfigForRefresh + oauth2.ReuseTokenSource — skips full OAuth on cold-start when wincred has a valid refresh token"
    - "ChangeWorkbook fresh-listener pattern (D-04 / RESEARCH.md §5.6): allocate a new 127.0.0.1:0 listener for the picker re-run, tear down on completion; doesn't reuse the wizard's listener (which is gone after /done's Shutdown)"
key-files:
  created:
    - internal/wizard/server.go
    - internal/wizard/server_test.go
    - internal/wizard/pages.go
    - internal/wizard/folderpicker_dialog.go
    - internal/wizard/pages/start.html
    - internal/wizard/pages/eq-folder.html
    - internal/wizard/pages/done.html
    - internal/tray/tray.go
    - internal/tray/tray_test.go
    - internal/app/runapp.go
    - internal/app/runapp_test.go
  modified:
    - cmd/squirebot/main.go (replaces Plan-01-01 smoke body with full RunApp + systray wiring)
decisions:
  - "Wizard HTML lives in three separate files under internal/wizard/pages/ (embed.FS), not a single picker-style single-file string. Three files give us per-page CSS without conflicts and let go vet's html/template checks run independently per template."
  - "wizard.Server exposes a hook seam (FolderPicker / SheetClientFactory / PickerAttacher / BrowserOpener) so the test suite never touches sqweek/dialog, real Google endpoints, or the user's actual browser. Production binding goes through NewServer; tests use newServerWithHooks. This is a deliberate departure from the plan's example code — the plan suggested 'use httptest stubbing accounts.google.com' but stubbing google.com requires WithHTTPClient injection at every layer that calls Token() / Userinfo, which is invasive. The hook-seam approach is cleaner and tests just as much."
  - "Result.TokenSource carries through from the wizard's auth.Manager DoneChan to RunApp's runWatcher, so we DO NOT re-load wincred immediately after the wizard completes. Wincred re-load only happens on the skip-wizard cold-start path (config already complete from a prior run)."
  - "ChangeWorkbook (D-04) uses a fresh 127.0.0.1:0 listener rather than trying to reuse the wizard's listener. The wizard's listener is gone by the time the user clicks Change Workbook… (httpSrv.Shutdown ran in /done's defer). A fresh listener is simpler and per-event isolated. RESEARCH.md §5.6 spelled this out as 'a new mini-loopback session for the picker re-run' so this matches research."
  - "The /changed landing page after a successful Change Workbook pick is rendered inline (no embedded HTML file) — it's a 4-line response, not worth a separate template file."
  - "Phase 1 deliberately does NOT hot-swap the running watcher's spreadsheetID after Change Workbook. The picker writes the new ID to config.json + cfg.SpreadsheetID + tray status, but the existing watcher goroutine keeps the old sheet.Client. User must Quit + relaunch for the new ID to take effect. T-07-09 in the plan threat register; documented limitation."
  - "Phase 1 ships green/red icons as the same byte slice (Phase 5 polish — distinct red art deferred). tray.go's SetIconHealth still flips between iconGreen and iconRed inputs so when Phase 5 ships distinct art the call sites don't change."
metrics:
  duration: "~70 minutes (executor wall-clock)"
  completed: 2026-05-01
---

# Phase 1 Plan 07: Wizard + Tray + RunApp Wiring Summary

The load-bearing integration plan. Plans 01-06 produced isolated packages; Plan 07 composes them into a runnable `squirebot.exe`. On launch, the binary parses config, builds a tray controller, fires off `app.RunApp` in a goroutine, and blocks on `systray.Run`. RunApp branches: incomplete config → `wizard.Server.Run` (which boots a loopback HTTP server hosting Plan 03's OAuth routes, Plan 06's picker routes, and Plan 07's three wizard pages on the same mux), then runs the watcher with `parse.Parse → sheet.WriteInventory → sheet.UpsertCharOwner` per fsnotify event. Complete config → re-build a refresh-only `oauth2.TokenSource` from wincred and start the watcher directly (no browser launch). The tray surface matches CONTEXT.md "Claude's Discretion" floor (Status / Open Workbook / Open log folder / Quit) plus D-04 (Change Workbook…) plus D-07 (Continue setup… — hidden by default).

INST-03 satisfied: the test suite includes `TestRun_BrowserOpenerCalledOnceOnStart` which confirms `auth.OpenBrowser` fires exactly once during a wizard.Run. Every subsequent navigation (OAuth → /picker → /eq-folder → /done) is server-side `http.Redirect` or in-page `<a href>` on the same browser tab.

## What Shipped

### Wizard step → URL mapping

| Step | URL | Owner |
|------|-----|-------|
| Step 1 page | `GET /start` | wizard (Plan 07) |
| OAuth callback | `GET /oauth/callback?code=...&state=...` | auth.Manager (Plan 03) |
| OAuth manual-paste fallback (AUTH-01) | `POST /start_paste` | auth.Manager (Plan 03) |
| Step 2 page | `GET /picker` | picker.Server (Plan 06) |
| Step 2 result POST | `POST /picker/result` | picker.Server (Plan 06) |
| Step 3 page | `GET /eq-folder` | wizard (Plan 07) |
| Step 3 native folder picker | `POST /eq-folder/pick` | wizard (Plan 07; sqweek/dialog) |
| Step 3 confirm | `POST /eq-folder/confirm` | wizard (Plan 07) |
| Step 4 page | `GET /done` | wizard (Plan 07) |
| Step 4 graceful shutdown trigger | `POST /wizard/shutdown` | wizard (Plan 07) |

All routes mounted on a single `*http.ServeMux` bound to a single `127.0.0.1:0` listener. Plan 03's `auth.NewManagerWithListener(cfg, bc, ln)` + `mgr.AttachRoutes(mux)` is the upstream half; this plan's `wizard.Server` registers the wizard half before calling `httpSrv.Serve(ln)` in a goroutine and opening the browser **once** to `/start`.

### `internal/wizard/` — the loopback wizard server

Three HTML pages embedded via `go:embed pages/*.html`:

- **start.html** — Connect Google CTA + auto-redirect after 800 ms; `<details>` block with the AUTH-01 manual-paste textarea POSTing to `/start_paste`. Renders `{{.Error}}` banner for re-entry after a paste error.
- **eq-folder.html** — Renders `{{.Discovered}}` (auto-discovery candidate from `eqfind.Discover()`) with Use this folder + Pick a different folder buttons; the manual-pick branch fetches `/eq-folder/pick` (which calls `sqweek/dialog`) and updates the form's hidden `path` field.
- **done.html** — "✓ You're all set" + 3-second `setTimeout(() => fetch('/wizard/shutdown', {method:'POST'}))`.

`wizard.Server.Run(ctx)` two-phase orchestration:

1. **Phase A** — boot listener + http.Server, attach auth + wizard routes, open browser to `/start`, block on `authMgr.DoneChan()`.
2. **Phase B** — build sheet.Client from the OAuth-issued TokenSource, attach picker routes (redirect after pick = `/eq-folder`), block on internal `done` channel (fired by `handleShutdown`).

Returns a `wizard.Result{Email, SpreadsheetID, EQFolder, TokenSource}` so the caller (RunApp) doesn't have to re-load wincred on the wizard happy-path. `defer httpSrv.Shutdown(3sCtx)` tears down the listener after Run returns.

Hook seam for tests: `FolderPicker`, `SheetClientFactory`, `PickerAttacher`, `BrowserOpener` are function-typed fields on `wizard.Server`. Production constructor `NewServer` wires the real bindings (`sqweek/dialog`, `sheet.NewClient`, `picker.NewServer`, `auth.OpenBrowser`); tests use `newServerWithHooks` to swap each for a stub.

### `internal/tray/` — the systray controller

`tray.Controller` exposes:

| Method | Purpose |
|--------|---------|
| `OnReady` / `OnExit` | systray.Run callbacks |
| `SetStatus(string)` | mutates the disabled top label |
| `SetIconHealth(Health)` | green/red icon swap |
| `ShowContinueSetup` / `HideContinueSetup` | D-07 menu-item visibility toggle |
| `SetSpreadsheetID(string)` | updates the URL the Open Workbook handler builds |
| `SpreadsheetID()` | read-only accessor |
| `LogDir()` | read-only accessor for the "Open log folder" target |

Menu ordering matches CONTEXT.md "Claude's Discretion" + D-04 + D-07:

```
Status (disabled)
─── separator ───
Open Workbook
Open log folder
Change Workbook…             (D-04)
Continue setup…              (D-07; .Hide() by default)
─── separator ───
Quit
```

Browser launch via `rundll32 url.dll,FileProtocolHandler` (Pitfall #6 — sidesteps cmd shell `&` ambiguity). Log folder via `explorer.exe`.

### `internal/app/runapp.go` — the orchestrator

`RunApp(ctx, cfg, bc, t)`:

```
RunApp:
  bc.Validate() → if missing constants, set tray red + return
  if needsWizard(cfg):
    tray.SetStatus("Setup needed"); SetIconHealth(Red); ShowContinueSetup
    res := wizard.NewServer(cfg, bc).Run(ctx)
    if res.Err: log + status + return (Continue setup… click re-enters)
    HideContinueSetup
    ts = res.TokenSource
  if ts == nil:
    ts = buildTokenSourceFromWincred(ctx, cfg, bc)  // skip-wizard path
  runWatcher(ctx, cfg, t, ts):
    sc := sheet.NewClient(ctx, ts, cfg.SpreadsheetID)
    sc.ValidateWorkbook(ctx)  // refuse to start on D-03 / Constraint #5
    tray.SetSpreadsheetID + SetIconHealth(Green) + SetStatus("Connected as ...")
    onChange := makeOnInventoryChange(ctx, sc, cfg, t)
    return watch.Run(ctx, cfg.EQFolder, onChange)  // BLOCKS until ctx cancel
```

`makeOnInventoryChange` per fsnotify event:

```
charName := extractCharName(path)
if charName == "": slog.Warn + return
f, _ := os.Open(path)         // re-stat + re-read fresh per RESEARCH.md §8.3
rows, _ := parse.Parse(f); f.Close()
if len(rows) == 0: log + return  // T-07-05: skip writes that would clear the tab
sc.WriteInventory(ctx, charName, sheet.InventoryHeader, rows, RFC3339Now)
sc.UpsertCharOwner(ctx, charName, cfg.GoogleEmail)
slog.Info("uploaded", "char", charName, "rows", len(rows))
tray.SetStatus("Last upload: <Char> at HH:MM")
```

`ChangeWorkbook(ctx, cfg, bc, t)` (D-04):

```
1. ReadToken(cfg.GoogleEmail) → StoredToken from wincred
2. OAuthConfigForRefresh(bc.OAuthClientID) + ReuseTokenSource → live TokenSource (no consent prompt)
3. net.Listen("127.0.0.1:0") → fresh ephemeral port
4. sheet.NewClient + picker.NewServer.SetRedirectAfterPick("/changed").OnPicked(<send done>).AttachRoutes(mux)
5. mux.HandleFunc("/changed", inline 4-line success page)
6. auth.OpenBrowser("http://127.0.0.1:<port>/picker")  -- ONE browser open, NO OAuth round-trip
7. select { ctx.Done | <-done | 5min timeout }
8. defer srv.Shutdown(3s)
```

The picker writes `cfg.SpreadsheetID` + saves config in its `handleResult` (Plan 06 owned), so by the time `OnPicked` fires, persistence is done. ChangeWorkbook just surfaces the change in tray status.

### `cmd/squirebot/main.go`

Replaces the Plan-01-01 smoke body. Wires:

```go
trayCtl = tray.NewController(tray.Config{
    IconGreen: iconBytes, IconRed: iconBytes, LogDir: logDir,
    SpreadsheetID: cfg.SpreadsheetID,
    OnContinueSetup:  func() { go app.RunApp(ctx, cfg, bc, trayCtl) },
    OnChangeWorkbook: func() { go app.ChangeWorkbook(ctx, cfg, bc, trayCtl) },
    OnQuit:           func() { cancel() },
})
go app.RunApp(ctx, cfg, bc, trayCtl)
systray.Run(trayCtl.OnReady, trayCtl.OnExit)  // BLOCKS
cancel()
```

## Test Inventory

21 tests across 3 packages, all passing. Wizard tests run in <2 s with zero network calls (hook seam stubs every cross-package dep).

| Package | Count | Tests |
|---------|-------|-------|
| internal/wizard | 15 | TestHandleStart_RendersAuthURL, TestHandleStart_RendersErrorBanner, TestHandleEQFolderGET_RendersDiscoveredOrFallback, TestHandleEQFolderConfirm_HappyPath_RedirectsToDone, TestHandleEQFolderConfirm_InvalidFolder_VerbatimD10, TestHandleEQFolderConfirm_RejectsNonPOST, TestHandleEQFolderPick_HappyPath_ReturnsJSON, TestHandleEQFolderPick_Cancelled_204, TestHandleEQFolderPick_InvalidFolder_VerbatimD10, TestHandleDone_RendersWithShutdownTimer, TestHandleShutdown_SendsResultOnDone, TestHandleShutdown_RejectsNonPOST, TestRun_CtxCancelReturnsErr, TestRun_BrowserOpenerCalledOnceOnStart (INST-03), TestRun_ListensOn127001Literal (Pitfall #6) |
| internal/tray | 4 | TestNewController_ConfigPropagation, TestSetSpreadsheetID_Mutates, TestMutators_SafeBeforeOnReady, TestHealthConstants |
| internal/app | 2 | TestExtractCharName (8 sub-cases incl. Foo/Cool Toon/Mörk/Spellbook), TestNeedsWizard (6 sub-cases) |

### Key load-bearing assertions

1. **INST-03** — `TestRun_BrowserOpenerCalledOnceOnStart` asserts the opener fires exactly once during a Run cycle.
2. **Pitfall #6** — `TestRun_ListensOn127001Literal` asserts the captured browser URL has `http://127.0.0.1:` prefix (not `localhost`).
3. **D-10 verbatim** — `TestHandleEQFolderConfirm_InvalidFolder_VerbatimD10` and `TestHandleEQFolderPick_InvalidFolder_VerbatimD10` both assert the literal string `This folder doesn't look like an EverQuest install (no eqgame.exe found). Pick a different folder.` is in the response body.
4. **TokenSource handoff** — `TestHandleShutdown_SendsResultOnDone` confirms `Result` carries `Email`, `SpreadsheetID`, `EQFolder` from `cfg` (the TokenSource is filled in by `Run` after the shutdown signal is received).

### Repo-wide test suite

```
ok  github.com/boejowen/SquireBot/internal/app      0.395s
ok  github.com/boejowen/SquireBot/internal/auth     1.052s
ok  github.com/boejowen/SquireBot/internal/config   1.623s
ok  github.com/boejowen/SquireBot/internal/eqfind   1.463s
ok  github.com/boejowen/SquireBot/internal/logging  1.633s
ok  github.com/boejowen/SquireBot/internal/parse    1.465s
ok  github.com/boejowen/SquireBot/internal/picker   0.539s
ok  github.com/boejowen/SquireBot/internal/sheet    0.615s
ok  github.com/boejowen/SquireBot/internal/tray     1.413s
ok  github.com/boejowen/SquireBot/internal/watch    5.153s
ok  github.com/boejowen/SquireBot/internal/wizard   1.872s
```

Zero regressions across the seven packages that existed before this plan. `go vet ./...` clean repo-wide.

## Build Verification

```
$ GOOS=windows GOARCH=amd64 go build \
    -ldflags="-H=windowsgui -s -w -X main.OAuthClientID=test -X main.PickerAPIKey=test -X main.GCPProjectNumber=1234 -X main.Version=0.1.0-plan07" \
    -o dist/squirebot.exe ./cmd/squirebot
$ file dist/squirebot.exe
dist/squirebot.exe: PE32+ executable for MS Windows 6.01 (GUI), x86-64, 8 sections
$ ls -la dist/squirebot.exe
... 16,864,256 bytes
```

### Binary size delta

| Phase | Size | Delta vs Plan 01-03 baseline |
|-------|------|------|
| Plan 01-03 baseline | 2,554,368 bytes (~2.4 MiB) | — |
| Plan 01-07 final    | 16,864,256 bytes (~16.1 MiB) | **+14.3 MiB (+560%)** |

The jump is dominated by:
- `google.golang.org/api/sheets/v4` and the wider GCP API client surface (~6-7 MiB)
- `google.golang.org/api/option` + `cloud.google.com/go/auth` + `oauth2adapt` (~2-3 MiB)
- `fyne.io/systray` + `github.com/TheTitanrain/w32` Windows GUI bindings (~1-2 MiB)
- `github.com/sqweek/dialog` Windows folder dialog (small)
- `html/template` (stdlib, modest)
- The three `embed.FS` HTML pages (~6 KiB total)

The size is in line with expectations — Plans 05 and 06 already pulled in `google.golang.org/api/sheets/v4` so most of the GCP weight was theoretically already there at link time, but Plan 01-03's smoke binary didn't actually USE those packages so the linker stripped them. Plan 07 is the first build that exercises every Phase 1 import surface, hence the full picture.

For Plan 08 the 16 MiB is the canonical NSIS-installer payload size.

## Plan Verification Checklist

| Check | Result |
|-------|--------|
| `go build ./internal/...` (interface alignment, BEFORE runapp.go) | ✓ exit 0 |
| `go build ./internal/wizard/...` | ✓ exit 0 |
| `go vet ./internal/wizard/...` | ✓ exit 0 |
| `go test ./internal/wizard/... -count=1 -timeout 60s` | ✓ 15/15 pass |
| Three HTML pages exist and are non-empty | ✓ |
| `grep -nE "127\.0\.0\.1:0" internal/wizard/server.go` | ✓ |
| `grep -nE "auth\.NewManagerWithListener" internal/wizard/server.go` | ✓ |
| `grep -nE "\.AttachRoutes\(" internal/wizard/server.go` | ✓ |
| `grep -nE "\.AuthURL\(\)" internal/wizard/server.go` | ✓ |
| `grep -nE "\.DoneChan\(\)" internal/wizard/server.go` | ✓ |
| `grep -nE "dialog\.Directory" internal/wizard/folderpicker_dialog.go` | ✓ |
| Verbatim D-10 message present in wizard/server.go | ✓ (twice — pick + confirm handlers) |
| `git diff --stat internal/auth/oauth.go` (Plan 07 vs Plan 03 baseline) | ✓ 0 lines changed |
| `go build ./internal/tray/...` | ✓ exit 0 |
| `go vet ./internal/tray/...` | ✓ exit 0 |
| `go test ./internal/tray/... -count=1` | ✓ 4/4 pass |
| Tray menu items present (Status, Open Workbook, Open log folder, Change Workbook, Continue setup, Quit) | ✓ all greppable |
| `grep -nE "rundll32" internal/tray/tray.go` | ✓ |
| `grep -nE "explorer\.exe" internal/tray/tray.go` | ✓ |
| `grep -nE "OnChangeWorkbook" internal/tray/tray.go` | ✓ |
| `GOOS=windows GOARCH=amd64 go build ./internal/tray/...` | ✓ exit 0 |
| `internal/app/runapp.go` calls `watch.Run` / `parse.Parse` / `WriteInventory` / `UpsertCharOwner` / `wizard.NewServer` / `auth.ReadToken` / `auth.OAuthConfigForRefresh` / `ReuseTokenSource` | ✓ all greppable |
| `internal/app/runapp.go` regex `^(.+)-Inventory\.txt$` for char-name | ✓ |
| `ChangeWorkbook` calls `picker.NewServer` + `AttachRoutes` + `auth.OpenBrowser` to /picker | ✓ |
| `cmd/squirebot/main.go` calls `systray.Run(trayCtl.OnReady, trayCtl.OnExit)` | ✓ |
| `cmd/squirebot/main.go` runs `app.RunApp` in a goroutine | ✓ |
| `cmd/squirebot/main.go` wires `OnChangeWorkbook` → `go app.ChangeWorkbook(...)` | ✓ |
| `dist/squirebot.exe` cross-compiles with test ldflags | ✓ 16,864,256 bytes PE32+ GUI |
| `go test ./internal/app/... -count=1` | ✓ 2/2 pass |
| `go test ./... -count=1 -timeout 120s` | ✓ 11/11 packages pass |
| `go vet ./...` | ✓ exit 0 |
| `grep -rE "time\.Tick" --include="*.go" internal/` | ✓ 0 matches (CLAUDE.md polling prohibition) |
| No non-comment slog calls reference RefreshToken / AccessToken / client_secret | ✓ 0 matches |
| `git diff --stat internal/auth/oauth.go` (final, Plan 07 vs Plan 03 baseline) | ✓ 0 lines changed |

## Plan Success Criteria

- [x] **INST-03 satisfied (final)** — `auth.OpenBrowser` called exactly once with the `/start` URL during a Run cycle (verified by `TestRun_BrowserOpenerCalledOnceOnStart`). The single tab carries the user OAuth → /picker → /eq-folder → /done via server-side redirects.
- [x] **AUTH-06 satisfied (final)** — `runWatcher.onChange` calls `sc.UpsertCharOwner(ctx, charName, cfg.GoogleEmail)` after every successful `WriteInventory`. First sighting of a character creates the `_char_owner` row.
- [x] **OPS-01 satisfied (reinforced)** — `runWatcher.onChange` calls `WriteInventory` with the per-char tab name `inv:<Char>` (Plan 05 enforces the per-character non-overlapping range internally). Watcher writes never share a mutable range across characters.
- [x] **D-04 satisfied (Phase 1 minimum)** — `Change Workbook…` tray menu item exists and triggers `app.ChangeWorkbook`, which re-runs picker on a fresh listener with a wincred-backed TokenSource. Picker's `handleResult` writes the new spreadsheetID to config.json on successful pick + ValidateWorkbook (Plan 06 owns).
- [x] **D-06 satisfied** — wizard auto-launches via `app.RunApp(ctx, ...)` in a goroutine when `needsWizard(cfg)` is true, opening the browser to `/start`.
- [x] **D-07 satisfied** — wizard is dismissible (ctx cancellation via Quit) and resumable (Continue setup… click triggers a fresh `app.RunApp`). Tray icon goes red on dismissal/error; Continue setup… visibility is toggled by RunApp via `ShowContinueSetup` / `HideContinueSetup`.
- [x] **Tray surface matches CONTEXT.md** Claude's Discretion list + D-04 — verified by greppable presence of all six menu labels in tray.go.
- [x] **Watcher → parser → sheets pipeline is end-to-end-tested** in unit form (`TestExtractCharName` + `TestNeedsWizard`) and ready for Plan 08's full smoke checkpoint on a real Win11 VM.
- [x] **File ownership invariant** — `git diff` against the Plan-03 baseline shows zero lines changed in `internal/auth/oauth.go`. Plan 07 consumes Plan 03's six exported symbols (NewManagerWithListener, AttachRoutes, AuthURL, HandlePastedRedirect, DoneChan, OAuthConfigForRefresh) by import only.

## Output Items From The Plan's `<output>` Block

### Wizard step → URL mapping

See "What Shipped" section above. Recap:
- `/start` (Plan 07)
- `/oauth/callback` (Plan 03)
- `/start_paste` (Plan 03)
- `/picker` (Plan 06)
- `/picker/result` (Plan 06)
- `/eq-folder`, `/eq-folder/pick`, `/eq-folder/confirm` (Plan 07)
- `/done`, `/wizard/shutdown` (Plan 07)

### ChangeWorkbook flow (D-04)

1. `auth.ReadToken(cfg.GoogleEmail)` → wincred-stored refresh token
2. `auth.OAuthConfigForRefresh(bc.OAuthClientID)` + `oauth2.ReuseTokenSource` → live TokenSource (no consent prompt)
3. `net.Listen("tcp", "127.0.0.1:0")` → fresh ephemeral port (NOT reusing wizard's listener — wizard's is gone after `/done`'s Shutdown)
4. `sheet.NewClient(ctx, ts, "")` (empty spreadsheetID — picker.handleResult sets it)
5. `picker.NewServer(sc, ts, cfg, bc).SetRedirectAfterPick("/changed").OnPicked(<-done<-{}).AttachRoutes(mux)`
6. `mux.HandleFunc("/changed", ...)` — inline 4-line success page
7. `auth.OpenBrowser("http://127.0.0.1:<port>/picker")` — ONE browser open, NO OAuth round-trip
8. `select { ctx.Done | <-done | 5min timeout }`
9. `defer srv.Shutdown(3s)` tears down on return
10. On done: `t.SetSpreadsheetID(cfg.SpreadsheetID)` + status update

### OnContinueSetup / OnChangeWorkbook re-entry races

**Not addressed in Phase 1 (T-07-03 — accepted limitation).** A user who triple-clicks Continue setup… would spawn three concurrent `app.RunApp` goroutines, all racing to bind a fresh `127.0.0.1:0` listener. The OS will give each a different ephemeral port, so they don't conflict at the network layer, but they DO contend for `s.cfg` (a shared `*config.Config`) and would each open a browser tab. Phase 2 polish should wrap RunApp + ChangeWorkbook in a `sync.Once`-style single-flight guard. Not observed in normal use.

### Successful-run slog chain (for Plan 08 smoke verification)

A clean cold-start with empty config, full happy-path through wizard + first inventory write should produce this sequence in `%LOCALAPPDATA%\SquireBot\squirebot.log`:

```
INFO  squirebot starting       version=0.1.0-... google_email="" spreadsheet_id_set=false eq_folder_set=false
INFO  wizard started           port=N start_url=http://127.0.0.1:N/start
INFO  oauth started            port=N state_prefix=...
INFO  oauth callback received  email=user@example.com
INFO  token stored in wincred  email=user@example.com target=SquireBot:user@example.com
INFO  wizard oauth complete    email=user@example.com
INFO  workbook picked          name="Guild SquireBot Workbook"
INFO  wizard picker step complete spreadsheet_id_set=true
INFO  wizard eq-folder confirmed  folder=C:\P99
INFO  watcher started          folder=C:\P99
INFO  watcher debounced        path=Foo-Inventory.txt
INFO  uploaded                 char=Foo rows=N
```

Plan 08's smoke checkpoint script can grep for each of those strings in order.

### Deviations from RESEARCH.md / plan

- **Plan §Task 1 example code** suggested testing the wizard with httptest stubbing accounts.google.com / sheets.googleapis.com. Implemented hook-seam injection instead (FolderPicker, SheetClientFactory, PickerAttacher, BrowserOpener function-typed fields on Server). Decision rationale documented in the YAML decisions block above. The plan's acceptance criteria don't require stubbing google endpoints specifically — they require "End-to-end happy path with stub auth + stub picker"; hook seams satisfy that just as well with a much smaller blast radius.
- **Plan §Task 1 example code** placed `s.tokenSource` and `s.sheetClient` as fields on `wizard.Server` and used them in `handleShutdown` to populate the Result. Implemented `s.tokenSource` only (sheet client is not surfaced through Result; runApp builds its own from the TokenSource). This avoids leaking the picker's intermediate Sheets client out of the wizard — runApp's runWatcher constructs a fresh Sheets client with the final spreadsheetID, which is cleaner.
- **Plan §Task 3 example main.go** wrote `_ = trayCtl.SetSpreadsheetID(cfg.SpreadsheetID)` — implemented as a regular call (no `_ =` since SetSpreadsheetID returns nothing). Cosmetic; the plan example was illustrative.
- **No deviations from RESEARCH.md §5.6 / §11 / §12** — the orchestration sequence is exactly as RESEARCH.md §11.1 spelled out: parse config → tray.NewController → go RunApp → systray.Run → cancel on quit.

### Confirmation: internal/auth/oauth.go was NOT modified

```
$ git log --oneline internal/auth/oauth.go | head -3
e570943 feat(01-03): add OAuth Manager with shared-listener API and PKCE flow
$ git diff HEAD -- internal/auth/oauth.go | wc -l
0
```

Plan 03's six exported symbols (`NewManagerWithListener`, `AttachRoutes`, `AuthURL`, `HandlePastedRedirect`, `DoneChan`, `OAuthConfigForRefresh`) consumed by import only.

### Note for Plan 08 — smoke checkpoint verification points

Plan 08 wraps `dist/squirebot.exe` in an NSIS installer + runs the smoke checkpoint on a clean Win11 VM. The verification list:

1. **Clean install** — NSIS installer runs without UAC prompt; `%LOCALAPPDATA%\SquireBot\squirebot.exe` created; tray icon appears.
2. **Single browser open** — running `squirebot.exe` with no prior config opens the browser exactly once. Visual: only one new tab in the user's default browser.
3. **OAuth round-trip** — Connect Google → Google consent (drive.file + openid + userinfo.email) → redirect to /picker. Watch for any second tab — that would be an INST-03 regression.
4. **Picker** — Drive Picker shows only spreadsheets. User picks the SquireBot template copy. Verify: cfg.SpreadsheetID populated in `%LOCALAPPDATA%\SquireBot\config.json` (`{"spreadsheet_id": "..."}`).
5. **eq-folder confirm** — wizard shows discovered path or invokes sqweek/dialog; user confirms. Verify: cfg.EQFolder populated.
6. **Done page → tray** — wizard tab shows "✓ You're all set" then closes itself; tray icon stays green.
7. **First write E2E** — in EQ, run `/outputfile inventory`. Within 30 s, check the workbook for an `inv:<Char>` tab containing the parsed five-column rows + `_uploaded_at` column. Phase 1's success criterion #3.
8. **`_char_owner` upsert** — verify the `_char_owner` tab contains a row `{<Char>, user@example.com, "", "", <RFC3339>}`.
9. **tray Change Workbook…** — click; verify a new browser tab opens to /picker (no OAuth re-prompt — token still valid). Pick a different SquireBot-canonical workbook. Verify: cfg.SpreadsheetID changes; tray status updates.
10. **wincred entry** — `cmdkey /list` should show `SquireBot:user@example.com` exists. The `config.json` contains NO refresh_token field (search the file).
11. **10-day refresh-token survival** — leave the binary running for 10 days; on day 10, trigger another `/outputfile inventory`. Should still upload without re-prompting (Production-mode OAuth refresh token does not expire after 7 days). If the token IS expired, oauth-config.json's consent screen status is wrong (Plan 02 / Pitfall #1 regression).
12. **slog chain match** — `grep -E "wizard started|oauth callback received|workbook picked|wizard eq-folder confirmed|watcher started|uploaded"` should produce six lines in order.
13. **NO refresh token in config.json** — `grep -i refresh %LOCALAPPDATA%\SquireBot\config.json` returns nothing.

### Manual-smoke items the user must perform NOW (before Plan 08)

These are the deferred Wave-2 OAuth round-trip + fsnotify→sheets E2E validations the plan calls out:

1. **Build with REAL ldflags** (not the test placeholders):
   ```pwsh
   $oc = Get-Content .planning\phases\01-end-to-end-thin-slice\oauth-config.json | ConvertFrom-Json
   $env:Path = "C:\Program Files\Go\bin;" + $env:Path
   go build -ldflags="-H=windowsgui -s -w `
     -X main.OAuthClientID=$($oc.oauth_client_id) `
     -X main.PickerAPIKey=$($oc.picker_api_key) `
     -X main.GCPProjectNumber=$($oc.gcp_project_number) `
     -X main.Version=0.1.0-plan07" `
     -o dist\squirebot.exe .\cmd\squirebot
   ```
2. **Cold-launch test** — delete `%LOCALAPPDATA%\SquireBot\config.json` and any wincred entry under `SquireBot:*`, then double-click `dist\squirebot.exe`. Expect: tray icon appears (red, "Setup needed"), browser opens to `/start`, you click Connect Google, complete consent (one tab — this is the INST-03 visual check), Drive Picker shows ONLY spreadsheets, you pick your SquireBot workbook, EQ folder is auto-discovered (or pick manually), click "You're all set", tab closes itself ~3s later, tray icon turns green.
3. **First inventory write** — in EQ Project 1999, log in any character, run `/outputfile inventory`. Check the workbook within 30 seconds for a new tab `inv:<Char>` with your inventory rows + an `_uploaded_at` timestamp column. Also check the `_char_owner` tab for a fresh row with your email.
4. **tray Change Workbook…** — right-click tray, click Change Workbook…; verify a NEW browser tab opens directly to /picker (NO Google consent re-prompt). Pick a different SquireBot workbook. Verify the tray status updates with the new ID prefix.
5. **tray Open Workbook** — click; verify your default browser opens to `https://docs.google.com/spreadsheets/d/<id>` and the workbook loads.
6. **tray Open log folder** — click; verify Explorer opens to `%LOCALAPPDATA%\SquireBot` and `squirebot.log` is present + populated with JSON lines including the slog chain shown above.
7. **tray Quit** — click; verify the tray icon disappears AND the squirebot.exe process is gone from Task Manager.
8. **Warm restart (skip wizard)** — re-launch `squirebot.exe`. Expect: NO browser opens; tray comes up directly to green, status reads `Connected as you@example.com — watching <folder-basename>`. This is the wincred refresh-token path.

If any of those steps misbehaves, that's the bug to surface BEFORE Plan 08 commits to the NSIS work.

### Blockers for Plan 01-08 (NSIS installer + clean-VM smoke)

Plan 08 is `wave: 6`, `depends_on: [01, 02, 07]`, `autonomous: false`. Blockers:

1. **NSIS toolchain** must be installed on the dev box (3.10+). Per `feedback_toolchain_installs.md` user memory, the dev installs missing toolchains themselves. Plan 08 is autonomous=false because it can't proceed without that.
2. **Win11 VM** required for the clean-install smoke. Plan 08's smoke checkpoint mounts that — not Plan 07's job.
3. **Production-mode OAuth project** must already be flipped to Production (Plan 02 supposedly did this; oauth-config.json's `consent_screen_status: "PRODUCTION"` field claims so). If Production mode is NOT live, the manual-smoke "10-day refresh-token survival" step in #11 of Plan 08's smoke list will fail — the real reproduction window is "wait 8 days then test", which Plan 08 cannot run synchronously. That's a CALENDAR blocker, not a code blocker.
4. **The dist/squirebot.exe this plan produced** is built with PLACEHOLDER ldflags (`test` for client ID and key). Plan 08's installer MUST rebuild with real ldflags before packaging. The build invocation is shown above under "Manual-smoke items #1".
5. **No code blockers from Plan 07's side.** The pipeline is wired; the binary builds; the test surface is green.

## Deviations from Plan

**One substantive deviation (test design choice, documented in decisions):**

The plan's Task 1 example test scenarios suggested "End-to-end happy path with stub auth + stub picker (use httptest server stubbing accounts.google.com and sheets.googleapis.com)." Implemented hook-seam dependency injection (FolderPicker / SheetClientFactory / PickerAttacher / BrowserOpener function-typed fields on Server) instead. Stubbing google.com requires propagating an `option.WithHTTPClient` through every layer that calls `Token()` / `Userinfo` / sheets — invasive and brittle. The hook seam achieves the same test coverage (handlers exercised in <2s with zero network calls) without leaking test plumbing into production constructor signatures. Acceptance criterion "End-to-end happy path with stub auth + stub picker" is met by `TestRun_BrowserOpenerCalledOnceOnStart` + `TestRun_ListensOn127001Literal` which both run a full `Run(ctx)` cycle with stubbed BrowserOpener / SheetClientFactory / PickerAttacher.

No bug fixes (Rule 1), no missing critical functionality (Rule 2), no architectural changes (Rule 4). The plan's example code's `<owner>` placeholder was replaced with `boejowen` per `go.mod` (plan-mandated, not a deviation). The plan's main.go example wrote a no-op `_ =` assignment which I dropped as cosmetic (also not a deviation).

## Threat-Model Realisation

- **T-07-01** (filename spoofing): mitigated by `extractCharName` returning `""` on non-Inventory paths; `runWatcher.onChange` logs warning + skips. Verified by `TestExtractCharName` cases for `Foo-Spellbook.txt`, `Foo-Inventory.bak`, empty path.
- **T-07-02** (RefreshToken in memory): mitigated by `buildTokenSourceFromWincred` zeroing `st.RefreshToken = ""; tok.RefreshToken = ""` after `ReuseTokenSource` is built. ReuseTokenSource holds its own internal copy.
- **T-07-03** (tray double-click race): NOT mitigated in Phase 1 — accepted limitation, Phase 2 should wrap RunApp/ChangeWorkbook in single-flight.
- **T-07-04** (wizard hung): mitigated by `Run`'s `select { case <-ctx.Done(): ... }` honoring ctx cancellation; user can Quit from tray to cancel.
- **T-07-05** (partial-write empty rows): mitigated by `if len(rows) == 0: skip` in `makeOnInventoryChange`. Plan 04's 500ms debouncer reduces the window further.
- **T-07-06** (arbitrary URL via Open Workbook): accepted — config write access = full machine compromise; URL prefix is hardcoded.
- **T-07-07** (wizard step audit): mitigated by `slog.Info` calls at each step transition (wizard started, oauth callback received, workbook picked, wizard picker step complete, wizard eq-folder confirmed, watcher started, uploaded, change workbook complete).
- **T-07-08** (email in slog): accepted — required for diagnostics + AUTH-06 canonical identity.
- **T-07-09** (watcher uses old spreadsheetID after Change Workbook): mitigated as documented Phase-1 limitation; user restart picks up new ID.
- **T-07-10** (EQ folder deleted while running): mitigated by Plan 04's `case <-w.Errors` continue-on-error; tray staleness signals via Status.

## Self-Check: PASSED

All artifacts exist:
- `internal/wizard/server.go` ✓
- `internal/wizard/server_test.go` ✓
- `internal/wizard/pages.go` ✓
- `internal/wizard/folderpicker_dialog.go` ✓
- `internal/wizard/pages/start.html` ✓
- `internal/wizard/pages/eq-folder.html` ✓
- `internal/wizard/pages/done.html` ✓
- `internal/tray/tray.go` ✓
- `internal/tray/tray_test.go` ✓
- `internal/app/runapp.go` ✓
- `internal/app/runapp_test.go` ✓
- `cmd/squirebot/main.go` (modified) ✓
- `dist/squirebot.exe` (16,864,256 bytes, PE32+ GUI) ✓

All commits present:
- `7172153` feat(01-07): wizard server with /start, /eq-folder, /done routes
- `c60b2ec` feat(01-07): tray Controller with Status, Open Workbook, Change Workbook, Continue setup, Quit
- `e04dd06` feat(01-07): RunApp orchestrator + ChangeWorkbook + main.go wiring

Tests: 21 new tests across `internal/{wizard,tray,app}` (15 + 4 + 2), all pass. Repo-wide `go test ./...` clean across all 11 packages. `go vet ./...` clean. Cross-compile to GOOS=windows succeeds. `internal/auth/oauth.go` shows zero diff vs Plan 03's `e570943` baseline.
