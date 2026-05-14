---
phase: 01-end-to-end-thin-slice
plan: 06
type: execute
wave: 4
depends_on: [03, 05]
files_modified:
  - internal/picker/server.go
  - internal/picker/server_test.go
  - internal/picker/picker_html.go
  - internal/picker/picker.html
autonomous: true
requirements: [INST-03]
must_haves:
  truths:
    - "After Plan 03's RunOAuth completes, the same loopback HTTP server is repurposed: a /picker GET handler serves an embedded HTML page that loads `apis.google.com/js/api.js` + `accounts.google.com/gsi/client` and renders the classic Web Picker"
    - "The Picker HTML uses the issued OAuth access token (via TokenSource), bakes APP_ID and API_KEY at server-startup time from build constants (Plan 03's BuildConstants), and limits view to mimeType `application/vnd.google-apps.spreadsheet`"
    - "When user picks a sheet, the JS POSTs `{spreadsheetId, name}` to `/picker/result`; the Go handler calls `sheet.Client.SetSpreadsheetID + ValidateWorkbook` and either redirects to `/eq-folder` (Plan 07 owns) or returns the verbatim D-03 rejection message as HTTP 400 body"
    - "The single browser tab opened by Plan 03's OAuth redirect carries the user through OAuth → Picker without a second browser-open (INST-03 'browser opens once for OAuth, once for Drive Picker' satisfied — the OAuth open IS the Picker open from the user's perspective; one tab, two consents)"
    - "Picker uses the classic Web Picker, NOT the Desktop Picker mode with prompt=consent&trigger_onepick=true (RESEARCH.md §5.1 + Pitfall #5)"
  artifacts:
    - path: "internal/picker/server.go"
      provides: "Server.AttachRoutes(mux, cfg) registers GET /picker and POST /picker/result on the loopback HTTP server passed in from Plan 03's OAuthResult"
      contains: "/picker/result"
    - path: "internal/picker/picker.html"
      provides: "Embedded HTML page rendering the classic Web Picker per RESEARCH.md §5.2"
      contains: "google.picker.PickerBuilder"
    - path: "internal/picker/picker_html.go"
      provides: "go:embed picker.html — exposes the page bytes for the Go template render"
      contains: "//go:embed"
  key_links:
    - from: "internal/picker/server.go"
      to: "internal/sheet/meta.go"
      via: "Server.handleResult calls Client.SetSpreadsheetID then Client.ValidateWorkbook"
      pattern: "ValidateWorkbook"
    - from: "internal/picker/server.go"
      to: "internal/auth/oauth.go (OAuthResult)"
      via: "Plan 07 hands the server's *http.Server + access token to picker.Server.AttachRoutes"
      pattern: "AccessToken|TokenSource"
    - from: "internal/picker/picker.html"
      to: "google.com Picker JS"
      via: "&lt;script src='https://apis.google.com/js/api.js'&gt; + setOAuthToken(ACCESS_TOKEN)"
      pattern: "apis\\.google\\.com/js/api\\.js"
---

<objective>
Add the Drive Picker route to the loopback HTTP server already running from Plan 03's OAuth flow.
The user clicks Connect → consents → is redirected to /picker → picks the guild workbook → POSTs
the ID back → ValidateWorkbook checks canonical_id → on success, browser advances to /eq-folder
(Plan 07's wizard step 3). On rejection, the Picker page shows the D-03 verbatim error.

Purpose: INST-03 demands that "the first-run flow opens the user's default browser exactly once
for Google OAuth and exactly once for Drive Picker" — Plan 03 opens the browser; THIS plan reuses
that already-open tab to load the Picker. No second `os.Exec` to launch the browser. Pitfall #5
forbids the Desktop Picker mode (which would require a public HTTPS redirect) — we use the classic
Web Picker per RESEARCH.md §5.1.

Output: An `internal/picker/` package that adds /picker (HTML page) and /picker/result (JSON
callback) routes to a passed-in *http.ServeMux. Plan 07 wires this into the Plan-03 server lifetime.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/phases/01-end-to-end-thin-slice/01-CONTEXT.md
@.planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md
@./CLAUDE.md
@internal/auth/oauth.go
@internal/auth/oauthconfig.go
@internal/sheet/meta.go
@internal/sheet/client.go
</context>

<interfaces>
<!-- Contracts this plan exports for downstream plans (07). -->

From internal/picker/server.go:
```go
package picker

// Server holds the dependencies needed to serve the Picker page and handle the result.
type Server struct {
    sheetClient *sheet.Client       // for ValidateWorkbook and SetSpreadsheetID
    tokenSource oauth2.TokenSource  // to mint a fresh access token for the Picker JS
    cfg         *config.Config      // to write spreadsheet_id on success
    bc          auth.BuildConstants // OAuthClientID, PickerAPIKey, GCPProjectNumber
    redirectAfterPick string         // "/eq-folder" — Plan 07 owns this route; for tests can be "/done"
    onPicked    func()              // optional notify callback (for Plan 07 wizard advancement)
}

// NewServer constructs a Server. All four fields are required.
func NewServer(sheetClient *sheet.Client, ts oauth2.TokenSource, cfg *config.Config, bc auth.BuildConstants) *Server

// AttachRoutes registers GET /picker and POST /picker/result on mux.
// Plan 07 calls this on the same *http.ServeMux Plan 03 used for /oauth/callback.
func (s *Server) AttachRoutes(mux *http.ServeMux)

// SetRedirectAfterPick lets Plan 07 override the next-page URL ("/eq-folder" by default).
func (s *Server) SetRedirectAfterPick(path string)
```
</interfaces>

<tasks>

<task type="auto" tdd="false">
  <name>Task 1: Embedded Picker HTML page</name>
  <files>internal/picker/picker.html, internal/picker/picker_html.go</files>
  <read_first>
    - .planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md (§5 entire — §5.1 design choice classic Web Picker lines 632-642; §5.2 mechanism with full HTML lines 644-706 — copy verbatim with minor template adaptation; §5.3 what Picker returns; §5.4 OAuth client console setup; §5.5 drive.file semantic)
    - .planning/phases/01-end-to-end-thin-slice/01-CONTEXT.md (D-03 verbatim rejection text; D-05 product is Sheet not Doc — mimeType filter)
    - ./CLAUDE.md ("OAuth scope: drive.file ONLY"; the Picker is HOW drive.file works)
  </read_first>
  <action>
    Create `internal/picker/picker.html` containing the EXACT HTML from RESEARCH.md §5.2 lines
    646-706. Use Go template syntax `{{.AccessToken}}`, `{{.AppID}}`, `{{.APIKey}}` placeholders
    (these will be filled at request time by Server.handlePicker — NOT bake into the embedded
    file). Reproduce the full file:

    ```html
    <!DOCTYPE html>
    <html>
    <head>
      <meta charset="utf-8">
      <title>SquireBot — pick your guild workbook</title>
      <style>
        body { font-family: -apple-system, Segoe UI, Roboto, sans-serif; padding: 2rem; max-width: 600px; margin: 0 auto; }
        h1 { font-size: 1.4rem; }
        #status { color: #555; margin-top: 1rem; }
        #status.error { color: #b00020; font-weight: 500; }
        button { font-size: 1rem; padding: 0.5rem 1rem; }
      </style>
    </head>
    <body>
      <h1>SquireBot — pick your guild workbook</h1>
      <p>Click below to open Google Drive Picker. Choose the SquireBot workbook your guild leader shared with you.</p>
      <button id="open">Open picker</button>
      <div id="status">Loading picker libraries…</div>
      <script src="https://apis.google.com/js/api.js"></script>
      <script src="https://accounts.google.com/gsi/client"></script>
      <script>
        const ACCESS_TOKEN = "{{.AccessToken}}";
        const APP_ID       = "{{.AppID}}";
        const API_KEY      = "{{.APIKey}}";

        let pickerInited = false;
        function onApiLoad() {
          gapi.load('picker', { callback: () => {
            pickerInited = true;
            document.getElementById('status').textContent = "Ready. Click 'Open picker'.";
          }});
        }

        function createPicker() {
          if (!pickerInited) { document.getElementById('status').textContent = "Picker still loading…"; return; }
          const view = new google.picker.DocsView(google.picker.ViewId.SPREADSHEETS)
              .setIncludeFolders(false)
              .setMimeTypes('application/vnd.google-apps.spreadsheet')
              .setOwnedByMe(false)
              .setSelectFolderEnabled(false);
          const sharedView = new google.picker.DocsView(google.picker.ViewId.SPREADSHEETS)
              .setMimeTypes('application/vnd.google-apps.spreadsheet')
              .setEnableDrives(true)
              .setOwnedByMe(false);
          const picker = new google.picker.PickerBuilder()
              .setAppId(APP_ID)
              .setOAuthToken(ACCESS_TOKEN)
              .setDeveloperKey(API_KEY)
              .addView(view)
              .addView(sharedView)
              .enableFeature(google.picker.Feature.NAV_HIDDEN)
              .setTitle("Pick the SquireBot guild workbook")
              .setCallback(pickerCallback)
              .build();
          picker.setVisible(true);
        }

        async function pickerCallback(data) {
          const status = document.getElementById('status');
          if (data.action === google.picker.Action.PICKED) {
            const file = data.docs[0];
            status.classList.remove('error');
            status.textContent = "Validating workbook…";
            try {
              const resp = await fetch('/picker/result', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ spreadsheetId: file.id, name: file.name }),
              });
              if (resp.ok) {
                // Server returns the next-page URL in body or sets Location.
                const next = resp.headers.get('Location') || '/eq-folder';
                location.href = next;
              } else {
                const msg = await resp.text();
                status.classList.add('error');
                status.textContent = msg;
              }
            } catch (e) {
              status.classList.add('error');
              status.textContent = "Network error: " + e.message + " — retry?";
            }
          } else if (data.action === google.picker.Action.CANCEL) {
            status.classList.add('error');
            status.textContent = "No workbook picked. Click 'Open picker' to retry.";
          }
        }

        document.getElementById('open').addEventListener('click', createPicker);
        window.onload = onApiLoad;
      </script>
    </body>
    </html>
    ```

    Create `internal/picker/picker_html.go`:
    ```go
    package picker

    import _ "embed"

    //go:embed picker.html
    var pickerHTMLTemplate string
    ```

    Note: the HTML is intentionally a Go html/template input (the `{{.X}}` placeholders are
    filled at request time by handlePicker in Task 2). Plain `text/template` would also work
    but `html/template` is safer for HTML contexts.
  </action>
  <verify>
    <automated>test -s internal/picker/picker.html &amp;&amp; grep -q 'apis\.google\.com/js/api\.js' internal/picker/picker.html &amp;&amp; grep -q 'google\.picker\.PickerBuilder' internal/picker/picker.html &amp;&amp; grep -q 'application/vnd\.google-apps\.spreadsheet' internal/picker/picker.html &amp;&amp; grep -q '/picker/result' internal/picker/picker.html &amp;&amp; grep -q '{{\.AccessToken}}' internal/picker/picker.html &amp;&amp; grep -q '{{\.AppID}}' internal/picker/picker.html &amp;&amp; grep -q '{{\.APIKey}}' internal/picker/picker.html &amp;&amp; grep -q '//go:embed picker.html' internal/picker/picker_html.go &amp;&amp; ! grep -q 'trigger_onepick' internal/picker/picker.html &amp;&amp; ! grep -q 'prompt=consent&amp;trigger_onepick' internal/picker/picker.html</automated>
  </verify>
  <acceptance_criteria>
    - `internal/picker/picker.html` exists
    - Contains `<script src="https://apis.google.com/js/api.js"></script>` (loads classic Web Picker)
    - Contains `google.picker.PickerBuilder` (uses Web Picker, not Desktop Picker)
    - Contains `application/vnd.google-apps.spreadsheet` (filters to Sheets only — D-05)
    - Contains `setOAuthToken(ACCESS_TOKEN)` and `setDeveloperKey(API_KEY)` and `setAppId(APP_ID)`
    - Contains POST to `/picker/result`
    - Has all three Go template placeholders: `{{.AccessToken}}`, `{{.AppID}}`, `{{.APIKey}}`
    - Does NOT contain the substring `trigger_onepick` (Pitfall #5 enforcement — Desktop Picker mode forbidden)
    - `internal/picker/picker_html.go` contains `//go:embed picker.html` directive
    - Embedded var is named `pickerHTMLTemplate` and is a `string`
  </acceptance_criteria>
  <done>
    The classic Web Picker HTML is embedded into the binary. Three template placeholders allow
    request-time access-token/api-key/app-id injection. No Desktop Picker mode magic params.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Picker server routes — GET /picker and POST /picker/result</name>
  <files>internal/picker/server.go, internal/picker/server_test.go</files>
  <read_first>
    - .planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md (§5.2 — Go side `pickerResult` handler lines 711-723; §2.4 Pattern 2 lines 369-389 lifecycle of loopback as wizard server; §12.3-12.4 ValidateWorkbook semantics)
    - internal/sheet/meta.go (Plan 05 just created — ValidateWorkbook signature + ErrWrongWorkbook + ErrSchemaTooNew)
    - internal/sheet/client.go (Plan 05 — Client.SetSpreadsheetID)
    - internal/auth/oauth.go (Plan 03 — OAuthResult.TokenSource)
    - internal/auth/oauthconfig.go (Plan 03 — BuildConstants)
    - internal/config/config.go (Plan 01 — Config + Save)
    - .planning/phases/01-end-to-end-thin-slice/01-CONTEXT.md (D-03 verbatim text; D-04 Change Workbook tray menu re-runs picker)
  </read_first>
  <behavior>
    - Test 1: GET /picker renders picker.html with template values substituted. Response Content-Type is `text/html; charset=utf-8`. Response body contains the access token returned by the test TokenSource.
    - Test 2: POST /picker/result with `{"spreadsheetId":"SHEET1","name":"Test Workbook"}` calls Client.ValidateWorkbook. On success (mock returns nil), the Server writes spreadsheetId to config, sets Location header to "/eq-folder", and returns 204 No Content (the JS reads Location and navigates).
    - Test 3: POST /picker/result when ValidateWorkbook returns ErrWrongWorkbook returns HTTP 400 with body equal to the verbatim ErrWrongWorkbook message. Config is NOT updated.
    - Test 4: POST /picker/result when ValidateWorkbook returns ErrSchemaTooNew returns HTTP 400 with body containing "newer SquireBot schema". Config is NOT updated.
    - Test 5: POST /picker/result with malformed JSON returns HTTP 400 with body "invalid JSON".
    - Test 6: GET /picker when TokenSource returns an error logs slog.Error and returns HTTP 500 with a generic "OAuth token unavailable" message (do NOT leak token error contents).
  </behavior>
  <action>
    Create `internal/picker/server.go`:
    ```go
    package picker

    import (
        "encoding/json"
        "fmt"
        "html/template"
        "log/slog"
        "net/http"

        "golang.org/x/oauth2"

        "github.com/<owner>/squirebot/internal/auth"
        "github.com/<owner>/squirebot/internal/config"
        "github.com/<owner>/squirebot/internal/sheet"
    )

    type Server struct {
        sheetClient        *sheet.Client
        tokenSource        oauth2.TokenSource
        cfg                *config.Config
        bc                 auth.BuildConstants
        redirectAfterPick  string
        onPicked           func()
        tmpl               *template.Template
    }

    func NewServer(sc *sheet.Client, ts oauth2.TokenSource, cfg *config.Config, bc auth.BuildConstants) *Server {
        tmpl := template.Must(template.New("picker").Parse(pickerHTMLTemplate))
        return &Server{
            sheetClient:       sc,
            tokenSource:       ts,
            cfg:               cfg,
            bc:                bc,
            redirectAfterPick: "/eq-folder",
            tmpl:              tmpl,
        }
    }

    func (s *Server) SetRedirectAfterPick(p string) { s.redirectAfterPick = p }
    func (s *Server) OnPicked(f func())             { s.onPicked = f }

    func (s *Server) AttachRoutes(mux *http.ServeMux) {
        mux.HandleFunc("/picker", s.handlePicker)
        mux.HandleFunc("/picker/result", s.handleResult)
    }

    func (s *Server) handlePicker(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodGet {
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return
        }
        tok, err := s.tokenSource.Token()
        if err != nil {
            slog.Error("picker token fetch failed", "err", err)
            http.Error(w, "OAuth token unavailable. Please retry from the start.", http.StatusInternalServerError)
            return
        }
        // SECURITY: AccessToken is short-lived (~1h) and bound to the user's session.
        // Embedding in HTML for the Picker JS is the documented pattern (RESEARCH.md §5.4).
        // It is NOT logged anywhere.
        data := struct {
            AccessToken string
            AppID       string
            APIKey      string
        }{
            AccessToken: tok.AccessToken,
            AppID:       s.bc.GCPProjectNumber,
            APIKey:      s.bc.PickerAPIKey,
        }
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        w.Header().Set("Cache-Control", "no-store")  // never cache the access-token-bearing page
        if err := s.tmpl.Execute(w, data); err != nil {
            slog.Error("picker template render failed", "err", err)
        }
    }

    type pickerResultBody struct {
        SpreadsheetID string `json:"spreadsheetId"`
        Name          string `json:"name"`
    }

    func (s *Server) handleResult(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return
        }
        var body pickerResultBody
        if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
            http.Error(w, "invalid JSON", http.StatusBadRequest); return
        }
        if body.SpreadsheetID == "" {
            http.Error(w, "spreadsheetId required", http.StatusBadRequest); return
        }

        s.sheetClient.SetSpreadsheetID(body.SpreadsheetID)
        if err := s.sheetClient.ValidateWorkbook(r.Context()); err != nil {
            slog.Warn("picked workbook rejected", "err", err, "name", body.Name)
            // Plan 05 returns the verbatim D-03 message for ErrWrongWorkbook and a clear
            // "update SquireBot" message for ErrSchemaTooNew. Surface the err.Error() text directly.
            // Reset spreadsheetID so a re-pick starts clean.
            s.sheetClient.SetSpreadsheetID("")
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }

        // Persist on success.
        s.cfg.SpreadsheetID = body.SpreadsheetID
        if err := s.cfg.Save(); err != nil {
            slog.Error("config save after picker", "err", err)
            http.Error(w, "Failed to save workbook selection", http.StatusInternalServerError)
            return
        }

        slog.Info("workbook picked", "name", body.Name)
        if s.onPicked != nil {
            s.onPicked()
        }

        // Tell the JS where to navigate next. Use Location header — the JS in picker.html
        // reads `resp.headers.get('Location')` and falls back to '/eq-folder' if absent.
        w.Header().Set("Location", s.redirectAfterPick)
        w.WriteHeader(http.StatusNoContent)
    }
    ```

    Replace `<owner>` with the actual module owner.

    Create `internal/picker/server_test.go` exercising the six behaviors. Use a fake
    `oauth2.TokenSource` (`oauth2.StaticTokenSource(&oauth2.Token{AccessToken:"fake-tok-123"})`)
    and a fake sheet.Client built against an httptest server (Plan 05's pattern). For Test 6,
    use a custom TokenSource that returns an error.

    For tests that need to assert ValidateWorkbook outcomes, build a real sheet.Client wired
    against an httptest stub mirroring Plan 05's tests (canonical_id matches → success;
    canonical_id mismatch → ErrWrongWorkbook; etc.). Or, refactor sheet.Client to accept a
    Validator interface and inject mocks — but keeping the integration test against an httptest
    Sheets stub is closer to real behavior and worth the test code.

    NOTE on access token in HTML: per RESEARCH.md §5.4, embedding the access_token in the page
    served from 127.0.0.1 is the documented Picker pattern. The token is short-lived (~1h),
    bound to the user's session, and the page is served only locally. Mark this in a code
    comment as the established pattern (NOT a security issue) and document that the page MUST
    NOT be served with `Cache-Control: public` (we set `no-store`).
  </action>
  <verify>
    <automated>go test ./internal/picker/... -count=1 -timeout 30s &amp;&amp; grep -nE "AttachRoutes" internal/picker/server.go &amp;&amp; grep -nE "/picker/result" internal/picker/server.go &amp;&amp; grep -nE "ValidateWorkbook" internal/picker/server.go &amp;&amp; grep -nE "Cache-Control.*no-store" internal/picker/server.go &amp;&amp; grep -nE "redirectAfterPick" internal/picker/server.go &amp;&amp; ! grep -vE "^\s*//" internal/picker/server.go | grep -E "slog\.(Info|Warn|Error|Debug).*AccessToken"</automated>
  </verify>
  <acceptance_criteria>
    - `internal/picker/server.go` exports `Server`, `NewServer`, `AttachRoutes`, `SetRedirectAfterPick`, `OnPicked`
    - server.go contains literal `/picker/result`
    - server.go contains literal `/picker` (the GET route)
    - server.go calls `ValidateWorkbook`
    - server.go sets `Cache-Control` to `no-store` on the picker HTML response (no token caching)
    - server.go does NOT log AccessToken in any slog call (`grep -E "slog\\.(Info|Warn|Error|Debug).*AccessToken" internal/picker/server.go` returns 0 non-comment matches)
    - server.go uses `html/template` (not `text/template`) for the HTML render
    - `internal/picker/server_test.go` contains at least 6 test cases
    - `go test ./internal/picker/... -count=1 -timeout 30s` exits 0
    - `go vet ./internal/picker/...` exits 0
  </acceptance_criteria>
  <done>
    Picker server is fully wired: GET /picker renders the embedded HTML with template-injected
    AccessToken/AppID/APIKey; POST /picker/result validates via Plan 05's ValidateWorkbook and
    persists or rejects per D-03 verbatim. Plan 07 will mount these routes on the Plan 03 server.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| browser ↔ /picker (loopback) | Server returns access-token-bearing HTML; same-origin discipline — only 127.0.0.1 reaches the page |
| picker.html JS ↔ google.com Picker | Cross-origin call to apis.google.com; relies on Google's CSP and the access token's session binding |
| picker.html ↔ /picker/result POST | Untrusted spreadsheetId arrives; ValidateWorkbook is the gate |
| AccessToken lifetime ↔ HTML render | Token expires ~1h; if user takes longer, Picker call fails and the JS surfaces a retry message |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-06-01 | Information Disclosure | AccessToken embedded in /picker HTML cached by browser or proxy | mitigate | Cache-Control: no-store header on /picker response; HTTPS not applicable on loopback but only 127.0.0.1 sees the page; token bound to short-lived session |
| T-06-02 | Information Disclosure | AccessToken logged via slog when handlePicker writes a debug message | mitigate | Acceptance grep enforces no slog.* calls reference AccessToken; data struct never serialised to logs |
| T-06-03 | Spoofing | Attacker tricks user's browser to POST /picker/result with attacker's spreadsheetId | mitigate | ValidateWorkbook canonical_id check (Plan 05) is the workbook-authentication gate; on mismatch, request is rejected with D-03 message; loopback server is bound to 127.0.0.1 (cannot be reached from network) |
| T-06-04 | Tampering | Picker JS modified by browser extension to bypass canonical_id check | mitigate | The check is server-side in handleResult — modifying client JS doesn't help. Server always calls ValidateWorkbook on the POSTed ID before persisting |
| T-06-05 | Privilege Escalation | Picker mimeType filter bypassed (user picks a Doc instead of a Sheet) | mitigate | mimeType filter on JS prevents UI from showing Docs; even if bypassed, ValidateWorkbook reads `_meta!A1:B2` which only Sheets have — Doc would 4xx the API call and rejection cascades |
| T-06-06 | Tampering | Cross-Origin attack from a malicious site that ALSO loads on 127.0.0.1:&lt;same-port&gt; (port reuse race) | accept | Same-origin policy + the lifetime of the loopback server is bounded to the OAuth+Picker flow (~minutes); after wizard completes, server shuts down (Plan 07). Real exploitation requires the attacker to win a port-allocation race AND the user to visit their page mid-flow — accepted residual risk |
| T-06-07 | Denial of Service | User repeatedly cancels Picker → Server holds OAuth token forever waiting | mitigate | Plan 07 wizard provides "Continue setup…" affordance; user can quit and re-launch; 60-second OAuth manual-paste timer (Plan 03) provides an upper bound |
| T-06-08 | Tampering | Picker URL params altered to load wrong workbook by reflecting a query string | mitigate | handleResult ignores ALL query string; only the JSON body's spreadsheetId is used; CSRF-style reflection attacks have nothing to reflect |
| T-06-09 | Privilege Escalation | drive.file scope expanded by silently picking a Doc that gets file-access | accept | Per RESEARCH.md §5.5, drive.file grants access only to files explicitly handed via Picker. Picking the wrong file gives access only to that file — bounded blast radius. |
</threat_model>

<verification>
- `go build ./internal/picker/...` exits 0
- `go vet ./internal/picker/...` exits 0
- `go test ./internal/picker/... -count=1 -timeout 30s` exits 0
- All six behaviors covered by tests
- `grep -rE "trigger_onepick" internal/picker/` returns 0 matches (Desktop Picker mode forbidden — Pitfall #5)
- `grep -rE "slog\\.(Info|Warn|Error|Debug).*\\bAccessToken\\b" internal/picker/ | grep -v "_test\\.go" | grep -v '^[^:]*:\\s*//'` returns 0 non-comment matches
- The picker.html template includes the three required script tags: api.js, gsi/client (and references google.picker.PickerBuilder)
</verification>

<success_criteria>
- INST-03 satisfied: Picker is reachable in the same browser tab Plan 03 opened (no second `os.Exec`); user picks workbook in one click after consent
- D-03 satisfied: rejection path returns the verbatim "This doesn't look like a SquireBot workbook…" message
- D-05 honored: mimeType filter is `application/vnd.google-apps.spreadsheet` only
- Pitfall #5 enforcement: classic Web Picker, NOT Desktop Picker mode
- AccessToken never logged; embedding in HTML is the documented Picker pattern
- Plan 07 has a clean mount-point: `picker.NewServer(...).AttachRoutes(mux)` on the Plan 03 mux
</success_criteria>

<output>
After completion, create `.planning/phases/01-end-to-end-thin-slice/01-06-SUMMARY.md` documenting:
- Whether the redirectAfterPick default was kept at `/eq-folder` or changed
- Any deviations from the RESEARCH.md §5.2 HTML
- Edge cases observed in testing (e.g., Picker JS load failure, expired access token mid-pick)
- A note for Plan 07 explaining how to wire `picker.NewServer(sheetClient, oauthResult.TokenSource, cfg, bc).AttachRoutes(mux)` into the Plan 03 server lifetime — and to set redirectAfterPick to "/eq-folder" so the wizard advances naturally
</output>
