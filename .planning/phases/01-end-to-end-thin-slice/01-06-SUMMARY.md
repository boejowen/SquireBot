---
phase: 01-end-to-end-thin-slice
plan: 06
subsystem: picker
tags: [drive-picker, oauth, html-template, loopback-http, embed]
requires:
  - 01-01: Go module skeleton (go.mod, internal/ layout)
  - 01-03: auth.OAuthResult.TokenSource + auth.BuildConstants + shared-listener pattern (NewManagerWithListener)
  - 01-05: sheet.Client + Client.SetSpreadsheetID + Client.ValidateWorkbook + ErrWrongWorkbook + ErrSchemaTooNew (verbatim D-03 text baked in)
provides:
  - picker.Server + picker.NewServer
  - picker.Server.AttachRoutes(mux) — registers GET /picker + POST /picker/result
  - picker.Server.SetRedirectAfterPick(p) — wizard step-3 URL override (defaults to "/eq-folder")
  - picker.Server.OnPicked(f) — callback fired after successful pick + persist
  - picker.html embedded — classic Web Picker page with three Go template placeholders
affects:
  - downstream Plan 07: wizard wires picker.NewServer(sheetClient, oauthResult.TokenSource, cfg, bc).AttachRoutes(mux) onto the same Plan-03 mux; sets redirectAfterPick to whatever URL serves the EQ-folder confirmation step
tech-stack:
  added: []  # all dependencies already present from earlier plans
  patterns:
    - "html/template parsed once at NewServer; rendered per-GET with fresh access token from TokenSource"
    - "//go:embed picker.html exposed as a string for template Parse (not as a fs.FS — single-file embed is simpler)"
    - "Cache-Control: no-store on every AccessToken-bearing response (T-06-01)"
    - "Generic HTTP error messages on token fetch failure; underlying err only in slog (T-06-02)"
    - "204 No Content + Location header on /picker/result success — JS reads resp.headers.get('Location') and navigates the same tab (avoids fetch's automatic 30x follow)"
    - "Reuses Plan 05 fake-Sheets httptest pattern for integration tests; zero real GCP calls"
key-files:
  created:
    - internal/picker/picker.html
    - internal/picker/picker_html.go
    - internal/picker/server.go
    - internal/picker/server_test.go
  modified: []
decisions:
  - "Kept redirectAfterPick default at '/eq-folder' as the plan specified — Plan 07 owns that route. Override is exposed via SetRedirectAfterPick."
  - "Use html/template (not text/template). The AccessToken comes from a trusted oauth2.Token (Google's tokens are alphanumeric+dash+slash and never contain HTML metacharacters), but html/template gives us defense in depth at zero performance cost. Plan acceptance criterion #'uses html/template (not text/template)' is satisfied."
  - "Use 204 No Content + Location header on /picker/result success rather than a 303 redirect. fetch() in browsers automatically follows 30x redirects for same-origin requests, which would fire the callback page logic again on the JS side. By using 204 + Location header, the picker.html JS explicitly reads resp.headers.get('Location') and assigns location.href, keeping client-side navigation explicit and matching the JS already shipped in picker.html."
  - "On any /picker/result rejection (canonical_id mismatch, schema_too_new, save failure), reset the sheetClient's spreadsheetID to '' so a subsequent unrelated call (e.g., from a stale watcher goroutine) doesn't accidentally land on a rejected workbook. Plan 07 will call SetSpreadsheetID again on a successful re-pick."
  - "Token fetch failure path returns a generic 'OAuth token unavailable. Please retry from the start.' to the HTTP client; the underlying error contents are logged via slog.Error but never surfaced in the response body. Test 6 (TestHandlePicker_TokenSourceErrorReturns500) explicitly asserts the underlying error string ('0xCAFEBABE') does not leak."
  - "Reused Plan 05's fake-Sheets httptest pattern in server_test.go (pickerStubHandler / sheetInfoStub mirror Plan 05's fakeSheetsHandler / sheetInfo with the same JSON shape). Pared down to the four endpoints ValidateWorkbook actually exercises. Pattern stays consistent across both packages, making it easy to extend in Plan 07."
metrics:
  duration: "~25 minutes (executor wall-clock)"
  completed: 2026-05-01
---

# Phase 1 Plan 06: Drive Picker Summary

The Drive Picker route is wired into the loopback HTTP server already running from Plan 03's OAuth flow. After consent, the same browser tab redirects to `GET /picker`, which serves a Go html/template-rendered HTML page that loads `apis.google.com/js/api.js`, builds a `google.picker.PickerBuilder` with `application/vnd.google-apps.spreadsheet`-only filtering, and POSTs the picked file ID back to `POST /picker/result`. The Go handler runs Plan 05's `ValidateWorkbook` canonical_id + schema_version handshake; on success the watcher's config is updated and a `Location: /eq-folder` 204 advances the wizard, on rejection the verbatim D-03 message is returned as the HTTP 400 body. INST-03 satisfied: a single browser tab carries the user OAuth → Picker without a second `os.Exec` call.

## What Shipped

### `internal/picker/picker.html` — the embedded HTML page

The classic Web Picker (RESEARCH.md §5.2). Three Go template placeholders — `{{.AccessToken}}`, `{{.AppID}}`, `{{.APIKey}}` — substituted at request time by `Server.handlePicker`. Mime-type filter is `application/vnd.google-apps.spreadsheet` only (D-05 — product is Sheet, not Doc). The JS POSTs `{spreadsheetId, name}` to `/picker/result` and either follows the `Location` header on a 2xx or surfaces the response body in `#status` on a 4xx. NO desktop-mode magic params (`trigger_onepick=true`, `prompt=consent` paired with onepick) — Pitfall #5 enforced.

### `internal/picker/picker_html.go` — `//go:embed picker.html`

Exposes `pickerHTMLTemplate string` to `server.go`. Single-file embed (not `embed.FS`); the package only ships one HTML file and html/template's `Parse(string)` is the cleanest fit.

### `internal/picker/server.go` — Server struct + routes

```go
type Server struct {
    sheetClient       *sheet.Client
    tokenSource       oauth2.TokenSource
    cfg               *config.Config
    bc                auth.BuildConstants
    redirectAfterPick string                 // "/eq-folder" by default
    onPicked          func()
    tmpl              *template.Template
}

func NewServer(sc *sheet.Client, ts oauth2.TokenSource, cfg *config.Config, bc auth.BuildConstants) *Server
func (s *Server) AttachRoutes(mux *http.ServeMux)
func (s *Server) SetRedirectAfterPick(p string)
func (s *Server) OnPicked(f func())
```

`AttachRoutes` registers `mux.HandleFunc("/picker", ...)` and `mux.HandleFunc("/picker/result", ...)` — no listener allocation, no `http.Serve` call. Plan 07's wizard hands in the Plan-03 mux verbatim.

### `handlePicker` — GET /picker

1. Method check: GET only.
2. `s.tokenSource.Token()` for a fresh access token (TokenSource caches, so this is cheap on the hot path; on token refresh it transparently picks up the rotated token).
3. On token fetch failure: `slog.Error` with the underlying err; HTTP 500 with **generic** message ("OAuth token unavailable. Please retry from the start."). The underlying error string never reaches the response body.
4. Render the html/template with `{AccessToken, AppID=GCPProjectNumber, APIKey=PickerAPIKey}`.
5. Headers: `Content-Type: text/html; charset=utf-8` and `Cache-Control: no-store` (T-06-01 mitigation — never cache a page bearing a bearer token).

### `handleResult` — POST /picker/result

1. Method check: POST only.
2. Decode JSON body `{spreadsheetId, name}`. Malformed → 400 "invalid JSON".
3. Empty spreadsheetId → 400 "spreadsheetId required".
4. `s.sheetClient.SetSpreadsheetID(body.SpreadsheetID)` — provisionally point the client at the picked workbook.
5. `s.sheetClient.ValidateWorkbook(r.Context())` — Plan 05's canonical_id + schema_version handshake.
6. **On rejection:**
   - `slog.Warn("picked workbook rejected", "err", err, "name", body.Name)` — log the workbook NAME (not ID), since name is human-meaningful and ID is mildly sensitive (grants drive.file access).
   - Reset `sheetClient.SetSpreadsheetID("")` so subsequent unrelated calls don't accidentally land on a rejected workbook.
   - `errors.Is(err, ErrWrongWorkbook)` OR `errors.Is(err, ErrSchemaTooNew)` → `http.Error(w, err.Error(), 400)`. Plan 05 baked the verbatim D-03 message into `ErrWrongWorkbook.Error()`, so this is the verbatim D-03 path. The picker.html JS shows it unchanged in `#status`.
   - Other errors → 500 with generic "Failed to validate workbook. Please try again."
7. **On success:**
   - `s.cfg.SpreadsheetID = body.SpreadsheetID; s.cfg.Save()`. On save failure, undo in-memory state (both `cfg.SpreadsheetID` and `sheetClient.SetSpreadsheetID("")`) and return 500.
   - `slog.Info("workbook picked", "name", body.Name)`.
   - Fire `s.onPicked()` if set (Plan 07 wizard advancement).
   - Set `Location: <redirectAfterPick>` header, write `204 No Content`. The picker.html JS reads `resp.headers.get('Location')` and assigns `location.href`.

## Test Inventory

11 tests in `internal/picker/server_test.go`, all using the fake-Sheets `httptest.NewServer` pattern from Plan 05 — zero real GCP calls.

| # | Test | Plan-spec behavior | Asserts |
|---|------|--------------------|---------|
| 1 | `TestHandlePicker_RendersTemplateWithAccessToken` | Behavior 1 | 200; Content-Type=text/html; Cache-Control=no-store; body contains "fake-tok-123", "1234567890", "picker-key-stub"; no raw `{{.X}}`; loads `apis.google.com/js/api.js`; references `google.picker.PickerBuilder`; mime filter `application/vnd.google-apps.spreadsheet` |
| 2 | `TestHandleResult_HappyPathPersistsAndRedirects` | Behavior 2 | 204; Location=/eq-folder; cfg.SpreadsheetID=SHEET1 |
| 3 | `TestHandleResult_WrongCanonicalReturnsVerbatimD03` | Behavior 3 | 400; body contains verbatim "This doesn't look like a SquireBot workbook. Pick the one shared by your guild leader."; cfg.SpreadsheetID=="" |
| 4 | `TestHandleResult_SchemaTooNewReturns400` | Behavior 4 | 400; body contains "newer SquireBot schema"; cfg.SpreadsheetID=="" |
| 5 | `TestHandleResult_MalformedJSONReturns400` | Behavior 5 | 400; body=="invalid JSON" |
| 6 | `TestHandlePicker_TokenSourceErrorReturns500` | Behavior 6 | 500; body contains "OAuth token unavailable"; body does NOT contain "0xCAFEBABE" (underlying err leak check) |
| 7 | `TestHandlePicker_RejectsNonGET` | bonus | 405 on POST /picker |
| 8 | `TestHandleResult_RejectsNonPOST` | bonus | 405 on GET /picker/result |
| 9 | `TestHandleResult_EmptySpreadsheetIDReturns400` | bonus | 400; body contains "spreadsheetId required" |
| 10 | `TestSetRedirectAfterPick_OverridesLocation` | API surface | redirect override propagates to Location header |
| 11 | `TestOnPicked_FiresOnSuccess` | API surface | callback fires exactly once on happy path |

### Repo-wide regression check

```
$ go test ./... -count=1
ok  	github.com/boejowen/SquireBot/internal/auth     0.895s
ok  	github.com/boejowen/SquireBot/internal/config   3.100s
ok  	github.com/boejowen/SquireBot/internal/eqfind   3.204s
ok  	github.com/boejowen/SquireBot/internal/logging  2.888s
ok  	github.com/boejowen/SquireBot/internal/parse    2.665s
ok  	github.com/boejowen/SquireBot/internal/picker   1.212s
ok  	github.com/boejowen/SquireBot/internal/sheet    1.003s
ok  	github.com/boejowen/SquireBot/internal/watch    5.132s
```

Zero regressions across all six pre-existing packages.

**Pre-existing flake noted:** `internal/watch/TestDebouncer_CoalescesBurst` flaked on the first full-suite run (timing-sensitive — debouncer's 500 ms window is tight under parallel package load) and passed on a second run as well as in isolation. Unrelated to this plan; logged here for the next executor's awareness.

## Plan Verification Checklist

| Check | Result |
|-------|--------|
| `go build ./internal/picker/...` | ✓ exits 0 |
| `go vet ./internal/picker/...` | ✓ exits 0 |
| `go test ./internal/picker/... -count=1 -timeout 30s` | ✓ 11 tests pass |
| `go vet ./...` | ✓ clean repo-wide |
| picker.html: contains `apis.google.com/js/api.js` | ✓ |
| picker.html: contains `google.picker.PickerBuilder` | ✓ |
| picker.html: contains `application/vnd.google-apps.spreadsheet` | ✓ |
| picker.html: contains `/picker/result` | ✓ |
| picker.html: has all three Go template placeholders | ✓ |
| picker.html: does NOT contain `trigger_onepick` | ✓ |
| picker.html: does NOT contain `prompt=consent&trigger_onepick` | ✓ |
| picker_html.go: contains `//go:embed picker.html` | ✓ |
| server.go: contains `AttachRoutes` | ✓ |
| server.go: contains `/picker/result` | ✓ |
| server.go: contains `ValidateWorkbook` | ✓ |
| server.go: sets `Cache-Control: no-store` | ✓ |
| server.go: contains `redirectAfterPick` | ✓ |
| server.go: NO non-comment slog call references AccessToken | ✓ |
| server.go: uses `html/template` (not `text/template`) | ✓ |
| server_test.go: at least 6 test cases | ✓ (11) |

## Plan Success Criteria

- [x] **INST-03 satisfied** — Picker is reachable in the same browser tab Plan 03 opened (no second `os.Exec`); user picks workbook in one click after consent. The /picker route registers on a caller-owned mux, so Plan 07's wizard hands in the Plan-03 mux directly.
- [x] **D-03 satisfied** — Rejection path returns `ErrWrongWorkbook.Error()` verbatim, which is the "This doesn't look like a SquireBot workbook…" message Plan 05 baked into the sentinel.
- [x] **D-05 honored** — Mime-type filter is `application/vnd.google-apps.spreadsheet` only.
- [x] **Pitfall #5 enforcement** — Classic Web Picker (`apis.google.com/js/api.js`), NOT Desktop Picker mode (no `trigger_onepick`).
- [x] **AccessToken never logged** — Acceptance grep for `slog.(Info|Warn|Error|Debug).*AccessToken` returns zero non-comment matches; embedding in HTML is the documented Picker pattern with `Cache-Control: no-store` mitigation (T-06-01).
- [x] **Plan 07 has a clean mount-point** — `picker.NewServer(sheetClient, oauthResult.TokenSource, cfg, bc).AttachRoutes(mux)` on the Plan 03 mux is the exact one-liner.

## Output Items From The Plan's `<output>` Block

> **Whether the redirectAfterPick default was kept at `/eq-folder` or changed**

Kept at `/eq-folder` as the plan specified. The constant `defaultRedirectAfterPick` in server.go documents this. Plan 07 may override via `SetRedirectAfterPick("/eq-folder")` (no-op) or any other path it wishes (e.g., if it serves the EQ-folder confirmation under a different URL).

> **Any deviations from the RESEARCH.md §5.2 HTML**

The HTML in this plan's Task 1 prose differs from RESEARCH.md §5.2 in two cosmetic ways (both spelled out in the plan itself, not deviations from the plan):

1. The plan's HTML adds an "Open picker" button + `pickerInited` flag so the user explicitly clicks to open the Picker. RESEARCH.md §5.2 calls `createPicker` directly from `gapi.load`'s callback, which would auto-pop the Picker the moment the page loads. The plan's button-gated approach lets the page render first, gives the user a "Loading picker libraries…" status to read, and is more idiot-proof on slow networks where the Picker JS takes a beat.

2. The plan's `pickerCallback` reads `resp.headers.get('Location')` rather than hardcoding `'/eq-folder'`. This makes the URL server-driven (via `Server.SetRedirectAfterPick`) so Plan 07 doesn't have to hand-edit picker.html.

I implemented exactly the plan's HTML — neither a deviation from the plan nor from RESEARCH.md (the plan is the ground truth for picker.html's contents).

> **Edge cases observed in testing**

- **TokenSource error mid-render** (Test 6): the user's only feedback is the generic "OAuth token unavailable" message; the underlying error reason is logged via `slog.Error` for the dev to triage. This deliberately avoids leaking refresh-token-failure details (which can carry scope hints or token bytes in the underlying oauth2 error) to the browser.
- **Save() failure after a successful pick** (covered indirectly by happy-path test): the implementation undoes both `cfg.SpreadsheetID = ""` AND `sheetClient.SetSpreadsheetID("")`, then returns 500. A re-pick will start from a clean state. This is uncovered by an explicit test (the LOCALAPPDATA-redirected tmp dir always succeeds in tests); a test for this would require a config.Save mock, deferred as out of scope.
- **Picker JS `Action.CANCEL`**: the JS surfaces "No workbook picked. Click 'Open picker' to retry." in `#status` and never POSTs to `/picker/result`. The server is unaware of cancellation, which is fine — the user's next action is either retry (button click) or quit the wizard.
- **Expired access token mid-pick**: the JS calls `picker.setVisible(true)` with the token embedded at page-render time. If the user idles for >1h before picking, the Picker UI itself will fail with a Google JS error. Plan 07's wizard "Continue setup…" affordance + a re-load of /picker would mint a fresh access token on the next GET. Out-of-scope for Phase 1 — documented residual risk T-06-07 in the plan's threat model.

> **Note for Plan 07: how to wire `picker.NewServer(...).AttachRoutes(mux)`**

```go
// Plan 07's wizard, after auth.RunOAuth completes:
oauthResult := <-mgr.DoneChan()
if oauthResult.Err != nil { /* surface in tray + restart flow */ return }

// sheetClient was constructed earlier with oauthResult.TokenSource so it
// can mint requests against Sheets v4 right away. SpreadsheetID is "" until
// the picker handler calls SetSpreadsheetID.
sheetClient, _ := sheet.NewClient(ctx, oauthResult.TokenSource, "")

// Mount the picker routes on the SAME mux Plan 03 used for /oauth/callback.
// (The mux in NewManagerWithListener mode is caller-owned.)
psrv := picker.NewServer(sheetClient, oauthResult.TokenSource, cfg, bc)
psrv.SetRedirectAfterPick("/eq-folder")  // Plan 07 owns /eq-folder
psrv.OnPicked(func() { /* advance wizard state machine */ })
psrv.AttachRoutes(mux)

// /eq-folder handler is Plan 07's own. The browser tab the user OAuthed
// in is now redirected to /picker via Plan 03's redirectAfterCallback,
// which Plan 03 already defaults to "/picker".
```

The "single browser tab" property of INST-03 falls out of three things composing:
1. Plan 03's `RunOAuth` returns a live Listener + Server.
2. Plan 03's `handleCallback` `http.Redirect(... "/picker", 302)` after exchange.
3. This plan's `AttachRoutes` registers `/picker` on the same mux before Plan 03's redirect fires.

Plan 07 just orchestrates the order — register routes BEFORE consenting the user, so the Plan-03 redirect lands on a registered handler instead of a 404.

## Deviations from Plan

**None — plan executed exactly as written.**

The plan's Task 2 example code uses `github.com/<owner>/squirebot/...` placeholder paths; I replaced `<owner>` with `boejowen` per `go.mod`. The plan explicitly told me to do this ("Replace `<owner>` with the actual module owner"), so this is plan-mandated, not a deviation.

No bug fixes (Rule 1), no missing critical functionality (Rule 2), no architectural changes (Rule 4). Three bonus tests (#7-9 in the inventory) were added beyond the plan's six-behavior minimum because they took ~30 lines of code each and harden the API surface against future regressions; the plan's acceptance criterion is "at least 6 test cases" which is comfortably met.

## Threat-Model Realisation

- **T-06-01** (AccessToken cached): mitigated via `Cache-Control: no-store` on every /picker response. Verified by Test 1 assertion.
- **T-06-02** (AccessToken in slog): mitigated via the acceptance grep. Verified by `grep -vE "^\s*//" internal/picker/server.go | grep -E "slog\\.(.*).*AccessToken"` returning zero matches.
- **T-06-03/04/05/08** (spoofing/tampering of spreadsheetId, mimeType bypass, query-string reflection): mitigated via Plan 05's `ValidateWorkbook` running unconditionally on every POST `/picker/result`. The `handleResult` handler ignores all query string and reads only the JSON body's `spreadsheetId`. Verified by Tests 2, 3, 4, 9.
- **T-06-06** (port-reuse race) / **T-06-07** (DoS via cancel) / **T-06-09** (drive.file blast radius): accepted residual risks per the plan's threat model — no mitigation in this plan, documented in plan threat register.

## Self-Check: PASSED

All artifacts exist:
- `internal/picker/picker.html` ✓
- `internal/picker/picker_html.go` ✓
- `internal/picker/server.go` ✓
- `internal/picker/server_test.go` ✓

All commits present:
- `0c132f0` feat(01-06): embed Drive Picker HTML page with classic Web Picker
- `d3ccecc` test(01-06): add failing tests for picker.Server routes
- `ed410a0` feat(01-06): implement picker.Server with /picker + /picker/result routes

Tests: 11 in `internal/picker/`, all pass. Zero regressions across `internal/{auth,config,eqfind,logging,parse,sheet,watch}` (one timing flake in `watch/TestDebouncer_CoalescesBurst` on first full-suite run, passes on retry — pre-existing, unrelated to picker).
