---
phase: 01-end-to-end-thin-slice
plan: 07
type: execute
wave: 5
depends_on: [01, 03, 04, 05, 06]
files_modified:
  - internal/wizard/server.go
  - internal/wizard/server_test.go
  - internal/wizard/pages.go
  - internal/wizard/pages/start.html
  - internal/wizard/pages/eq-folder.html
  - internal/wizard/pages/done.html
  - internal/tray/tray.go
  - internal/app/runapp.go
  - cmd/squirebot/main.go
files_modified_notes: |
  internal/auth/oauth.go is OWNED by Plan 03. Plan 07 CONSUMES Plan 03's shared-listener API
  (NewManagerWithListener / AttachRoutes / AuthURL / HandlePastedRedirect / DoneChan /
  OAuthConfigForRefresh) — Plan 07 does NOT edit oauth.go. If those symbols don't exist when
  Plan 07 runs, Plan 03 has a defect; do not paper over it from inside this plan.
autonomous: true
requirements: [INST-03, AUTH-06, OPS-01]
must_haves:
  truths:
    - "Running squirebot.exe with no prior config opens the browser ONCE to /start; user clicks through OAuth (Plan 03) → Picker (Plan 06) → /eq-folder confirm → /done. No second browser launch."
    - "After /done, the wizard HTTP server gracefully shuts down (server.Shutdown(ctx)); only the tray + watcher loop remain."
    - "When config is fully populated (google_email + spreadsheet_id + eq_folder), running squirebot.exe SKIPS the wizard entirely and starts the watcher directly."
    - "The watcher's onChange callback (from Plan 04) calls parse.Parse → sheet.WriteInventory → sheet.UpsertCharOwner. The character name is derived from the basename of the inventory file via regex `^(.+)-Inventory\\.txt$`."
    - "The tray menu has the four mandatory items per CONTEXT.md Claude's-discretion list: Status (read-only label), Open Workbook (browser), Open log folder, Quit. The 'Continue setup…' item appears ONLY when wizard is incomplete (D-07)."
    - "Per D-04, the tray menu has a 'Change Workbook…' item that re-runs the Drive Picker on the existing OAuth listener (token still valid → skip OAuth) and writes the new spreadsheetID to config.json on successful pick + ValidateWorkbook. Phase 1 minimum: menu item exists and re-runs Picker."
    - "The tray Status label updates after every successful sheet write to `Last upload: <Char> at HH:MM` per RESEARCH.md §11"
    - "On OAuth manual-paste timeout (60s — AUTH-01) or wizard mid-flow dismissal (D-07), the tray icon turns red and Status reads 'Setup needed'. Continue setup… reopens /start."
    - "Plan 07 consumes Plan 03's shared-listener API (NewManagerWithListener / AttachRoutes / AuthURL / HandlePastedRedirect / DoneChan / OAuthConfigForRefresh) without editing internal/auth/oauth.go. Plan 03 ships those symbols; Plan 07 only imports them."
  artifacts:
    - path: "internal/app/runapp.go"
      provides: "RunApp(ctx, log, cfg, bc) — the background goroutine launched from main.go that orchestrates wizard-vs-watch mode based on config completeness"
      contains: "wizard.Server"
    - path: "internal/wizard/server.go"
      provides: "Wizard.Server.Run(ctx) — boots the loopback HTTP server, attaches /start, OAuth /oauth/callback, /picker, /picker/result, /eq-folder, /done routes; returns when /done is reached"
      contains: "ListenAndServe"
    - path: "internal/tray/tray.go"
      provides: "Tray controller with SetStatus(string), SetIconHealth(green|red), AppendContinueSetup(), RemoveContinueSetup(), and a 'Change Workbook…' menu item per D-04"
      contains: "Change Workbook"
    - path: "cmd/squirebot/main.go"
      provides: "Updated entry point that runs RunApp in a goroutine and blocks on systray.Run"
      contains: "systray.Run"
  key_links:
    - from: "cmd/squirebot/main.go"
      to: "internal/app/runapp.go"
      via: "go RunApp(coreCtx, log, cfg, bc) before systray.Run"
      pattern: "go\\s+app\\.RunApp"
    - from: "internal/app/runapp.go"
      to: "internal/wizard/server.go"
      via: "wizard.Server orchestrates auth + picker + eq-folder flow"
      pattern: "wizard\\.NewServer|wizard\\.Run"
    - from: "internal/app/runapp.go"
      to: "internal/watch/watcher.go"
      via: "watch.Run(ctx, cfg.EQFolder, onInventoryChange) after wizard completes (or immediately if config is complete)"
      pattern: "watch\\.Run"
    - from: "internal/app/runapp.go (onInventoryChange)"
      to: "internal/parse/inventory.go + internal/sheet/write.go + internal/sheet/owner.go"
      via: "parse.Parse(file) -> sheet.WriteInventory(charName, ...) -> sheet.UpsertCharOwner(charName, email)"
      pattern: "parse\\.Parse|WriteInventory|UpsertCharOwner"
    - from: "internal/tray/tray.go"
      to: "internal/wizard/server.go"
      via: "Continue setup menu item triggers wizard restart via tray.OnContinueSetup callback"
      pattern: "OnContinueSetup|ContinueSetup"
    - from: "internal/tray/tray.go"
      to: "internal/picker/server.go (via runapp.go orchestrator)"
      via: "Change Workbook… menu item triggers tray.OnChangeWorkbook callback, which re-launches picker.NewServer on the existing OAuth listener (D-04)"
      pattern: "OnChangeWorkbook|ChangeWorkbook"
---

<objective>
Wire every Phase 1 component into a working end-to-end app. This is the load-bearing integration
plan — Plans 01-06 produce isolated packages; Plan 07 produces the squirebot.exe that on launch
auto-discovers state, runs the wizard if needed, then watches and writes. Plus the tray UI shell
that surfaces status and lets the user open the workbook, view logs, change workbook (D-04), and
quit.

Purpose: Phase 1's success criterion #3 demands "within 30 seconds of saving Foo-Inventory.txt to
the configured EQ folder, an inv:Foo tab containing the parsed five-column rows appears in the
selected workbook." That demands every package work in concert: watcher fires → parser decodes →
sheets writes → owner upserts. This plan implements that wire, plus the wizard glue (D-06, D-07),
the D-04 "Change Workbook…" tray item, and the tray (RESEARCH.md §11). Two of Phase 1's three
"still uncovered after Plans 01-06" requirements land here: INST-03 (browser-once-each), and the
OPS-01 reinforcement that the wired write call uses the per-character non-overlapping range Plan
05 already enforced.

Output: A runnable `squirebot.exe` that, given a populated oauth-config.json + a Plan 02
Production-published OAuth project, performs the entire end-to-end flow on a real Windows
machine. Plan 08 wraps it in NSIS and runs the smoke checkpoint.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/phases/01-end-to-end-thin-slice/01-CONTEXT.md
@.planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md
@.planning/research/ARCHITECTURE.md
@./CLAUDE.md
@internal/auth/oauth.go
@internal/auth/oauthconfig.go
@internal/picker/server.go
@internal/sheet/client.go
@internal/sheet/write.go
@internal/sheet/owner.go
@internal/sheet/meta.go
@internal/watch/watcher.go
@internal/parse/inventory.go
@internal/eqfind/discover.go
@internal/config/config.go
@internal/logging/logger.go
</context>

<tasks>

<task type="auto" tdd="false">
  <name>Task 1: Wizard pages and server (start, eq-folder, done) attached to the loopback mux</name>
  <files>internal/wizard/server.go, internal/wizard/server_test.go, internal/wizard/pages.go, internal/wizard/pages/start.html, internal/wizard/pages/eq-folder.html, internal/wizard/pages/done.html</files>
  <read_first>
    - .planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md (§2.4 Pattern 2 lines 369-389 lifecycle of loopback as wizard server; §6.6 native folder picker via sqweek/dialog lines 894-901; §11 tray section informs lifecycle)
    - .planning/phases/01-end-to-end-thin-slice/01-CONTEXT.md (D-06 four-step wizard; D-07 dismissible/resumable; D-09 cascade; D-10 eqgame.exe validation; D-11 single-folder Phase 1)
    - internal/auth/oauth.go (Plan 03 — Manager + the shared-listener API: NewManagerWithListener, AttachRoutes, AuthURL, HandlePastedRedirect, DoneChan, OAuthConfigForRefresh — all owned and shipped by Plan 03)
    - internal/picker/server.go (Plan 06 — picker.NewServer + AttachRoutes signatures; redirectAfterPick = "/eq-folder")
    - internal/eqfind/discover.go (Plan 04 — Discover + ValidateFolder + ErrNotFound)
    - internal/sheet/client.go (Plan 05 — Client + NewClient)
    - internal/config/config.go (Plan 01 — Config struct + Save)
  </read_first>
  <action>
    **PREREQUISITE — Interface alignment check (run BEFORE authoring runapp.go or wizard/server.go):**
    Run `go build ./internal/...` and require exit code 0. This proves Plans 01-06 packages compile
    together with their final exported interfaces — including Plan 03's shared-listener API
    (NewManagerWithListener / AttachRoutes / AuthURL / HandlePastedRedirect / DoneChan /
    OAuthConfigForRefresh). If this fails, the upstream plan that owns the missing/changed symbol
    has a defect — surface it to the orchestrator before authoring this plan's code. Do NOT add the
    missing symbols from inside this plan.

    Then create three small HTML pages under `internal/wizard/pages/`:

    **start.html** — wizard step 1 placeholder. Contains a `<a href="{{.AuthURL}}">Connect Google</a>`
    button, brief explanation, and on `window.onload` auto-click after 800ms (gives the browser
    time to settle, then redirects to Google). Page also accepts `&error=...` query string and
    surfaces it as a red banner (for the AUTH-01 manual-paste fallback case where Plan 03's flow
    returns to /start with an error message).

    Include a hidden form for the manual-paste fallback (AUTH-01): a textarea where the user can
    paste the redirect URL Google sent if the loopback callback never landed. POSTs to
    `/start_paste`. Plan 03's Manager exposes `HandlePastedRedirect(ctx, raw)` that parses
    `code` + `state` from the pasted URL and runs the same callback handler. The /start_paste
    route is REGISTERED BY Plan 03's `Manager.AttachRoutes(mux)` call — Plan 07's wizard does NOT
    re-register it; it just renders the textarea form that POSTs to that route.

    **eq-folder.html** — wizard step 3. Renders:
    - The auto-discovered path (if any) from `eqfind.Discover()`, with a "Use this folder" button
    - A native folder picker button: "Pick a different folder" → POST to `/eq-folder/pick`
      which calls `dialog.Directory().Title(...).Browse()` (sqweek/dialog) on the Go side, returning
      the picked path
    - On submit: POST to `/eq-folder/confirm` with form field `path`; the server validates with
      `eqfind.ValidateFolder` and either advances to /done or shows the D-10 verbatim rejection.

    **done.html** — wizard step 4. "✓ You're all set" message. JS calls `setTimeout(() => fetch('/wizard/shutdown', {method:'POST'}), 3000)` to trigger the wizard server's graceful shutdown (D-06's "minimizes to tray with one-shot toast"). Plan 07 surfaces the toast via the OS notification API in Task 3.

    Create `internal/wizard/pages.go` with go:embed:
    ```go
    package wizard

    import "embed"

    //go:embed pages
    var pagesFS embed.FS
    ```

    Create `internal/wizard/server.go`. The wizard CONSUMES Plan 03's shared-listener API by name:
    ```go
    package wizard

    import (
        "context"
        "errors"
        "fmt"
        "html/template"
        "log/slog"
        "net"
        "net/http"
        "time"

        "golang.org/x/oauth2"
        "github.com/sqweek/dialog"

        "github.com/<owner>/squirebot/internal/auth"
        "github.com/<owner>/squirebot/internal/config"
        "github.com/<owner>/squirebot/internal/eqfind"
        "github.com/<owner>/squirebot/internal/picker"
        "github.com/<owner>/squirebot/internal/sheet"
    )

    // Result is what RunWizard returns on completion. The Listener and Server are SHUT DOWN
    // by RunWizard before returning; caller does not need to manage them.
    type Result struct {
        Email         string
        SpreadsheetID string
        EQFolder      string
        TokenSource   oauth2.TokenSource
        Err           error
    }

    type Server struct {
        cfg          *config.Config
        bc           auth.BuildConstants
        sheetClient  *sheet.Client       // built after OAuth completes; nil before
        authMgr      *auth.Manager       // shared-listener Manager from auth.NewManagerWithListener
        pickerSrv    *picker.Server
        listener     net.Listener
        httpSrv      *http.Server
        mux          *http.ServeMux
        startTmpl    *template.Template
        eqTmpl       *template.Template
        doneTmpl     *template.Template
        done         chan Result
        oauthDone    <-chan auth.OAuthResult  // = authMgr.DoneChan()
    }

    func NewServer(cfg *config.Config, bc auth.BuildConstants) *Server {
        startTmpl := template.Must(template.ParseFS(pagesFS, "pages/start.html"))
        eqTmpl    := template.Must(template.ParseFS(pagesFS, "pages/eq-folder.html"))
        doneTmpl  := template.Must(template.ParseFS(pagesFS, "pages/done.html"))
        return &Server{cfg: cfg, bc: bc, startTmpl: startTmpl, eqTmpl: eqTmpl, doneTmpl: doneTmpl,
            done: make(chan Result, 1)}
    }

    // Run boots the wizard. Blocks until /done is reached, ctx is cancelled, or 60s OAuth
    // timeout + dismissal occurs. Returns a Result.
    func (s *Server) Run(ctx context.Context) Result {
        // Allocate ephemeral port; bind to 127.0.0.1 literal (Pitfall #6).
        ln, err := net.Listen("tcp", "127.0.0.1:0")
        if err != nil { return Result{Err: fmt.Errorf("listen: %w", err)} }
        s.listener = ln
        s.mux = http.NewServeMux()
        s.httpSrv = &http.Server{Handler: s.mux, ReadHeaderTimeout: 10 * time.Second}

        // Build auth.Manager bound to this listener. CONSUMES Plan 03's shared-listener API.
        s.authMgr = auth.NewManagerWithListener(s.cfg, s.bc, ln)
        s.authMgr.AttachRoutes(s.mux) // registers /oauth/callback + /start_paste on our mux
        s.oauthDone = s.authMgr.DoneChan()

        // Wizard's own routes
        s.mux.HandleFunc("/start", s.handleStart)
        s.mux.HandleFunc("/eq-folder", s.handleEQFolderGET)
        s.mux.HandleFunc("/eq-folder/pick", s.handleEQFolderPick)
        s.mux.HandleFunc("/eq-folder/confirm", s.handleEQFolderConfirm)
        s.mux.HandleFunc("/done", s.handleDone)
        s.mux.HandleFunc("/wizard/shutdown", s.handleShutdown)

        port := ln.Addr().(*net.TCPAddr).Port
        startURL := fmt.Sprintf("http://127.0.0.1:%d/start", port)

        // Serve in background.
        go func() { _ = s.httpSrv.Serve(ln) }()
        defer s.httpSrv.Shutdown(context.Background())

        // Open browser ONCE (INST-03).
        if err := auth.OpenBrowser(startURL); err != nil {
            slog.Warn("could not auto-open browser; copy this URL", "url", startURL)
            // Plan 07 tray will surface this as a clickable status; for Plan 07 unit tests we
            // just log and continue — the user can paste manually.
        }

        // Block until oauthDone OR ctx cancellation. Then build sheet.Client + attach Picker.
        select {
        case <-ctx.Done():
            return Result{Err: ctx.Err()}
        case oauthRes := <-s.oauthDone:
            if oauthRes.Err != nil {
                return Result{Err: fmt.Errorf("oauth: %w", oauthRes.Err)}
            }
            // OAuth done; build sheet.Client and attach picker routes.
            sc, err := sheet.NewClient(ctx, oauthRes.TokenSource, "")
            if err != nil { return Result{Err: fmt.Errorf("sheet client: %w", err)} }
            s.sheetClient = sc
            s.pickerSrv = picker.NewServer(sc, oauthRes.TokenSource, s.cfg, s.bc)
            s.pickerSrv.SetRedirectAfterPick("/eq-folder")
            s.pickerSrv.OnPicked(func() { slog.Info("picker step complete") })
            s.pickerSrv.AttachRoutes(s.mux)
            // Now block until /done.
            return <-s.done
        }
    }

    // handleStart renders start.html with the auth URL from authMgr.AuthURL().
    func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
        var data struct {
            AuthURL string
            Error   string
        }
        data.AuthURL = s.authMgr.AuthURL()
        data.Error = r.URL.Query().Get("error")
        s.startTmpl.Execute(w, data)
    }

    func (s *Server) handleEQFolderGET(w http.ResponseWriter, r *http.Request) {
        var data struct {
            Discovered string
            Error      string
        }
        if p, err := eqfind.Discover(); err == nil {
            data.Discovered = p
        }
        // Render eq-folder.html
        s.eqTmpl.Execute(w, data)
    }

    func (s *Server) handleEQFolderPick(w http.ResponseWriter, r *http.Request) {
        // Native Windows folder picker via sqweek/dialog.
        // dialog.Directory().Title("Pick your EverQuest folder").Browse() returns (string, error).
        path, err := dialog.Directory().Title("Pick your EverQuest folder").Browse()
        if err != nil {
            // User cancelled or error; return 204 so JS can re-render eq-folder.html.
            w.WriteHeader(http.StatusNoContent); return
        }
        if err := eqfind.ValidateFolder(path); err != nil {
            // Verbatim D-10 rejection: "This folder doesn't look like an EverQuest install
            // (no eqgame.exe found). Pick a different folder."
            http.Error(w, "This folder doesn't look like an EverQuest install (no eqgame.exe found). Pick a different folder.", http.StatusBadRequest)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"path": path})
    }

    func (s *Server) handleEQFolderConfirm(w http.ResponseWriter, r *http.Request) {
        _ = r.ParseForm()
        path := r.PostForm.Get("path")
        if err := eqfind.ValidateFolder(path); err != nil {
            http.Error(w, "This folder doesn't look like an EverQuest install (no eqgame.exe found). Pick a different folder.", http.StatusBadRequest)
            return
        }
        s.cfg.EQFolder = path
        if err := s.cfg.Save(); err != nil {
            http.Error(w, "Failed to save: "+err.Error(), http.StatusInternalServerError); return
        }
        http.Redirect(w, r, "/done", http.StatusFound)
    }

    func (s *Server) handleDone(w http.ResponseWriter, r *http.Request) {
        // Render done.html. JS will POST /wizard/shutdown after 3s.
        s.doneTmpl.Execute(w, nil)
    }

    func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusNoContent)
        // Send result and let Server.Run's select unblock; then httpSrv.Shutdown() in defer.
        s.done <- Result{
            Email:         s.cfg.GoogleEmail,
            SpreadsheetID: s.cfg.SpreadsheetID,
            EQFolder:      s.cfg.EQFolder,
            // TokenSource: ... — preserved through s.sheetClient; we re-load on watcher startup
            // by calling auth.ReadToken(email) and re-binding a TokenSource.
        }
    }
    ```

    Add `internal/wizard/server_test.go` with at least these scenarios:
    1. End-to-end happy path with stub auth + stub picker (use httptest server stubbing
       accounts.google.com and sheets.googleapis.com).
    2. /eq-folder/confirm with a valid folder path → redirects to /done.
    3. /eq-folder/confirm with an invalid path → returns 400 with the verbatim D-10 message.
    4. /wizard/shutdown POST sends a Result on the done channel.
    5. handleStart renders the AuthURL placeholder (smoke test that authMgr.AuthURL() is wired
       into the template).

    These are the LIGHTEST integration tests; full end-to-end is exercised by Plan 08's smoke
    checkpoint with a real binary and real Google Cloud project.
  </action>
  <verify>
    <automated>go build ./internal/... &amp;&amp; go build ./internal/wizard/... &amp;&amp; go vet ./internal/wizard/... &amp;&amp; go test ./internal/wizard/... -count=1 -timeout 60s &amp;&amp; test -s internal/wizard/pages/start.html &amp;&amp; test -s internal/wizard/pages/eq-folder.html &amp;&amp; test -s internal/wizard/pages/done.html &amp;&amp; grep -nE "127\\.0\\.0\\.1:0" internal/wizard/server.go &amp;&amp; grep -nE "/start" internal/wizard/server.go &amp;&amp; grep -nE "/eq-folder" internal/wizard/server.go &amp;&amp; grep -nE "/done" internal/wizard/server.go &amp;&amp; grep -nE "auth\\.NewManagerWithListener" internal/wizard/server.go &amp;&amp; grep -nE "\\.AttachRoutes\\(" internal/wizard/server.go &amp;&amp; grep -nE "\\.AuthURL\\(\\)" internal/wizard/server.go &amp;&amp; grep -nE "\\.DoneChan\\(\\)" internal/wizard/server.go &amp;&amp; grep -nE "dialog\\.Directory" internal/wizard/server.go</automated>
  </verify>
  <acceptance_criteria>
    - `go build ./internal/...` exits 0 (interface alignment check) BEFORE runapp.go authored — proves Plans 01-06 export their final symbols
    - All three HTML pages exist and contain valid templates
    - `internal/wizard/server.go` exports `NewServer`, `Run`, `Result`
    - server.go binds to `127.0.0.1:0` (NOT `localhost`)
    - server.go registers all five wizard routes: /start, /eq-folder, /eq-folder/pick, /eq-folder/confirm, /done, /wizard/shutdown (NOTE: /start_paste is registered BY auth.Manager.AttachRoutes, not by wizard)
    - server.go uses `dialog.Directory()` for the native folder picker (sqweek/dialog)
    - server.go contains the verbatim D-10 message: `This folder doesn't look like an EverQuest install (no eqgame.exe found). Pick a different folder.`
    - server.go calls `auth.NewManagerWithListener`, `Manager.AttachRoutes(mux)`, `Manager.AuthURL()`, `Manager.DoneChan()` — all symbols owned by Plan 03; this plan does NOT modify internal/auth/oauth.go
    - `go test ./internal/wizard/... -count=1` exits 0
    - `go vet ./internal/wizard/...` exits 0
    - `git diff --stat internal/auth/oauth.go` (from Plan 07's branch) shows ZERO lines changed (file ownership invariant)
  </acceptance_criteria>
  <done>
    Wizard server is wired: /start (OAuth gateway) → Plan 03 OAuth → /picker (Plan 06) →
    /eq-folder (this plan) → /done. Plan 03's shared-listener API is consumed by import — no
    edits to oauth.go. The sqweek/dialog native folder picker is wired into /eq-folder/pick.
    Interface-alignment prerequisite (`go build ./internal/...`) passed before code authored.
  </done>
</task>

<task type="auto" tdd="false">
  <name>Task 2: Tray UI controller (systray) with status updates, "Continue setup…" affordance, and "Change Workbook…" (D-04)</name>
  <files>internal/tray/tray.go</files>
  <read_first>
    - .planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md (§11 entire — §11.1 Lifecycle lines 1106-1153 with full main.go skeleton; §11.2 goroutine layout; §11.3 crash safety; §5.6 Picker re-launch flow on existing OAuth listener)
    - .planning/phases/01-end-to-end-thin-slice/01-CONTEXT.md (Claude's Discretion: tray menu surface — Status, Open Workbook, Continue setup…, Quit; D-04 "Change Workbook…" required; D-07 dismissible/resumable)
    - cmd/squirebot/main.go (Plan 01 skeleton — Version variable, iconBytes embedded — to be wrapped by Task 3)
    - internal/logging/logger.go (Plan 01 — Setup returns logDir for "Open log folder" menu item)
    - internal/picker/server.go (Plan 06 — picker.NewServer + AttachRoutes; D-04 re-launch reuses these)
  </read_first>
  <action>
    Create `internal/tray/tray.go`. Per D-04 (locked decision), the tray MUST include a "Change
    Workbook…" menu item. Phase 1 minimum: the menu item exists and triggers a callback
    (`OnChangeWorkbook`) that re-runs Picker on the existing OAuth listener. The runapp.go
    orchestrator (Task 3) owns the callback implementation; tray.go just owns the menu surface +
    the click handler that fires the callback.
    ```go
    package tray

    import (
        "fmt"
        "log/slog"
        "os/exec"
        "path/filepath"
        "sync"

        "fyne.io/systray"
    )

    type Health int
    const (
        HealthGreen Health = iota
        HealthRed
    )

    type Controller struct {
        mu              sync.Mutex
        iconGreen       []byte
        iconRed         []byte
        logDir          string
        spreadsheetID   string

        mStatus           *systray.MenuItem
        mWorkbook         *systray.MenuItem
        mLogs             *systray.MenuItem
        mContinueSetup    *systray.MenuItem
        mChangeWorkbook   *systray.MenuItem  // D-04
        mQuit             *systray.MenuItem

        onContinueSetup  func()
        onChangeWorkbook func()  // D-04: re-run Picker on existing OAuth listener
        onQuit           func()
    }

    type Config struct {
        IconGreen        []byte
        IconRed          []byte
        LogDir           string
        SpreadsheetID    string  // initial; can be empty
        OnContinueSetup  func()  // wizard.Run trigger
        OnChangeWorkbook func()  // D-04: change workbook callback (re-launch picker)
        OnQuit           func()  // app shutdown trigger
    }

    func NewController(c Config) *Controller {
        return &Controller{
            iconGreen: c.IconGreen, iconRed: c.IconRed,
            logDir: c.LogDir, spreadsheetID: c.SpreadsheetID,
            onContinueSetup: c.OnContinueSetup,
            onChangeWorkbook: c.OnChangeWorkbook,
            onQuit: c.OnQuit,
        }
    }

    // OnReady wires the tray on startup. systray.Run(t.OnReady, t.OnExit) blocks main.
    func (t *Controller) OnReady() {
        systray.SetIcon(t.iconGreen)
        systray.SetTooltip("SquireBot")

        t.mStatus = systray.AddMenuItem("Initialising…", "")
        t.mStatus.Disable()

        systray.AddSeparator()
        t.mWorkbook       = systray.AddMenuItem("Open Workbook", "Open the configured Google Sheet in your browser")
        t.mLogs           = systray.AddMenuItem("Open log folder", "Open %LOCALAPPDATA%\\SquireBot in Explorer")
        t.mChangeWorkbook = systray.AddMenuItem("Change Workbook…", "Pick a different SquireBot workbook (re-runs Picker)")  // D-04 — literal label
        t.mContinueSetup  = systray.AddMenuItem("Continue setup…", "Resume the SquireBot wizard")
        t.mContinueSetup.Hide() // shown only when needed (D-07)
        systray.AddSeparator()
        t.mQuit = systray.AddMenuItem("Quit", "Exit SquireBot")

        // Click handlers run in their own goroutines.
        go t.loop()
    }

    func (t *Controller) loop() {
        for {
            select {
            case <-t.mWorkbook.ClickedCh:
                t.mu.Lock()
                id := t.spreadsheetID
                t.mu.Unlock()
                if id == "" {
                    slog.Info("Open Workbook clicked but no spreadsheet configured")
                    continue
                }
                _ = exec.Command("rundll32", "url.dll,FileProtocolHandler",
                    "https://docs.google.com/spreadsheets/d/"+id).Start()
            case <-t.mLogs.ClickedCh:
                _ = exec.Command("explorer.exe", t.logDir).Start()
            case <-t.mChangeWorkbook.ClickedCh:
                // D-04: re-run Picker on the existing OAuth listener (token still valid).
                // runapp.go owns the implementation: bring loopback server back up, open browser
                // to /picker (skipping OAuth), on successful pick + ValidateWorkbook write the
                // new spreadsheetID to config.json.
                if t.onChangeWorkbook != nil { t.onChangeWorkbook() }
            case <-t.mContinueSetup.ClickedCh:
                if t.onContinueSetup != nil { t.onContinueSetup() }
            case <-t.mQuit.ClickedCh:
                if t.onQuit != nil { t.onQuit() }
                systray.Quit()
                return
            }
        }
    }

    func (t *Controller) OnExit() {}

    // SetStatus updates the disabled top menu label.
    func (t *Controller) SetStatus(s string) {
        if t.mStatus != nil { t.mStatus.SetTitle(s) }
    }

    func (t *Controller) SetIconHealth(h Health) {
        switch h {
        case HealthGreen: systray.SetIcon(t.iconGreen)
        case HealthRed:   systray.SetIcon(t.iconRed)
        }
    }

    func (t *Controller) ShowContinueSetup()  { if t.mContinueSetup != nil { t.mContinueSetup.Show() } }
    func (t *Controller) HideContinueSetup()  { if t.mContinueSetup != nil { t.mContinueSetup.Hide() } }

    func (t *Controller) SetSpreadsheetID(id string) {
        t.mu.Lock(); defer t.mu.Unlock()
        t.spreadsheetID = id
    }

    // OpenLogFolder is exposed for use by other packages (e.g., crash recover).
    func (t *Controller) OpenLogFolder() {
        _ = exec.Command("explorer.exe", filepath.Clean(t.logDir)).Start()
    }
    ```

    For Phase 1, the green/red icons can be the same icon (with a small overlay for red, or just
    swap to a stand-in red variant — Phase 5 is the polish phase). Embed a second icon
    `assets/icon-red.ico` if desired; otherwise reuse the same iconBytes for both. Document this
    in code comment: "Phase 5 will produce distinct green/red art."

    No unit tests for tray.go — systray requires a desktop session and is not test-friendly. Plan
    08's smoke checkpoint is the verification.
  </action>
  <verify>
    <automated>go build ./internal/tray/... &amp;&amp; go vet ./internal/tray/... &amp;&amp; grep -nE "fyne\\.io/systray" internal/tray/tray.go &amp;&amp; grep -nE "Open Workbook" internal/tray/tray.go &amp;&amp; grep -nE "Open log folder" internal/tray/tray.go &amp;&amp; grep -nE "Continue setup" internal/tray/tray.go &amp;&amp; grep -nE "Change Workbook" internal/tray/tray.go &amp;&amp; grep -nE "OnChangeWorkbook" internal/tray/tray.go &amp;&amp; grep -nE "rundll32" internal/tray/tray.go &amp;&amp; grep -nE "explorer\\.exe" internal/tray/tray.go &amp;&amp; grep -nE "ShowContinueSetup" internal/tray/tray.go &amp;&amp; grep -nE "SetStatus" internal/tray/tray.go &amp;&amp; grep -nE "SetIconHealth" internal/tray/tray.go</automated>
  </verify>
  <acceptance_criteria>
    - `internal/tray/tray.go` exports `Controller`, `NewController`, `Config`, `Health`, `OnReady`, `OnExit`, `SetStatus`, `SetIconHealth`, `ShowContinueSetup`, `HideContinueSetup`, `SetSpreadsheetID`
    - tray.go uses `fyne.io/systray` (per locked stack)
    - tray.go has the four mandatory menu items: Status (disabled), Open Workbook, Open log folder, Quit (CONTEXT.md Claude's Discretion)
    - tray.go has a `Continue setup…` item that starts hidden (D-07)
    - tray.go has a `Change Workbook…` item per D-04. The literal string `Change Workbook` MUST appear in tray.go (greppable). The Config struct MUST include `OnChangeWorkbook func()` and the click handler MUST invoke it.
    - tray.go uses `rundll32 url.dll,FileProtocolHandler` for browser launch (Pitfall #6 — consistent with auth/browser.go)
    - tray.go uses `explorer.exe` for log folder open
    - `go build ./internal/tray/...` exits 0 on Windows; on Linux it builds (systray has cross-platform stubs) — verify with cross-compile to GOOS=windows
    - `go vet ./internal/tray/...` exits 0
  </acceptance_criteria>
  <done>
    Tray controller exposes the four mandatory menu items + Continue setup + Change Workbook…
    (D-04). Status label and icon health are mutable from other goroutines. Plan 08's smoke run
    on a real machine validates visual behavior.
  </done>
</task>

<task type="auto" tdd="false">
  <name>Task 3: RunApp orchestrator + main.go wiring (the load-bearing integration) + D-04 ChangeWorkbook implementation</name>
  <files>internal/app/runapp.go, cmd/squirebot/main.go</files>
  <read_first>
    - All previous Phase 1 plan SUMMARYs (01-01-SUMMARY.md through 01-06-SUMMARY.md) — confirm the actual exported signatures (in case any plan deviated)
    - .planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md (§5.6 Change-Workbook re-launch flow on existing OAuth listener; §12 identity bootstrap §12.1-12.6 — first-run sequence; §11.1 main.go skeleton; §10 logging)
    - .planning/phases/01-end-to-end-thin-slice/01-CONTEXT.md (D-04 Change Workbook…; D-06 wizard auto-launch; D-07 resumable)
    - All files listed in `<context>` block above (the entire Phase 1 import surface)
  </read_first>
  <action>
    Create `internal/app/runapp.go` — the orchestrator. This package depends on every Phase 1
    package; it lives in `internal/app/` so cmd/squirebot/main.go is thin. Plan 03's
    `OAuthConfigForRefresh` is consumed here (NOT defined here — Plan 03 owns it).

    ```go
    package app

    import (
        "context"
        "fmt"
        "log/slog"
        "net"
        "net/http"
        "os"
        "path/filepath"
        "regexp"
        "time"

        "golang.org/x/oauth2"

        "github.com/<owner>/squirebot/internal/auth"
        "github.com/<owner>/squirebot/internal/config"
        "github.com/<owner>/squirebot/internal/parse"
        "github.com/<owner>/squirebot/internal/picker"
        "github.com/<owner>/squirebot/internal/sheet"
        "github.com/<owner>/squirebot/internal/tray"
        "github.com/<owner>/squirebot/internal/watch"
        "github.com/<owner>/squirebot/internal/wizard"
    )

    var charNameRE = regexp.MustCompile(`^(.+)-Inventory\.txt$`)

    // RunApp is the background goroutine launched from main.go. It blocks until ctx is cancelled.
    // - If config is incomplete (missing email / spreadsheet_id / eq_folder) → run wizard, then watcher.
    // - If config is complete → start watcher directly.
    // Tray Continue setup… invokes wizard re-run.
    // Tray Change Workbook… invokes ChangeWorkbook (D-04 — re-runs Picker only).
    func RunApp(ctx context.Context, cfg *config.Config, bc auth.BuildConstants, t *tray.Controller) {
        if err := bc.Validate(); err != nil {
            slog.Error("build constants missing", "err", err)
            t.SetStatus("Build error: missing OAuth constants")
            t.SetIconHealth(tray.HealthRed)
            return
        }

        for ctx.Err() == nil {
            if needsWizard(cfg) {
                t.SetStatus("Setup needed")
                t.SetIconHealth(tray.HealthRed)
                t.ShowContinueSetup()

                ws := wizard.NewServer(cfg, bc)
                res := ws.Run(ctx)
                if res.Err != nil {
                    slog.Error("wizard failed", "err", res.Err)
                    t.SetStatus(fmt.Sprintf("Setup error: %v", res.Err))
                    // Fall through; loop will re-enter wizard via Continue setup… click.
                    return
                }
                t.HideContinueSetup()
            }

            // Config is complete. Build sheet client + start watcher.
            if err := runWatcher(ctx, cfg, t); err != nil {
                slog.Error("watcher exited", "err", err)
                t.SetStatus(fmt.Sprintf("Watcher error: %v", err))
                t.SetIconHealth(tray.HealthRed)
                // Phase 1: don't auto-recover. Phase 2 AUTH-05 owns the reauth flow.
                return
            }
            return // ctx cancelled
        }
    }

    func needsWizard(cfg *config.Config) bool {
        return cfg.GoogleEmail == "" || cfg.SpreadsheetID == "" || cfg.EQFolder == ""
    }

    // ChangeWorkbook implements the D-04 tray flow. Phase 1 minimum: bring up a fresh loopback
    // listener, build a refresh-only TokenSource from wincred (token still valid → skip OAuth),
    // attach picker.Server, open browser to /picker, on successful pick + ValidateWorkbook
    // write the new spreadsheetID to config.json. RESEARCH.md §5.6 describes the flow.
    //
    // This is the callback installed in tray.Config.OnChangeWorkbook (main.go).
    func ChangeWorkbook(ctx context.Context, cfg *config.Config, bc auth.BuildConstants, t *tray.Controller) {
        slog.Info("Change Workbook clicked — re-launching picker on existing token", "email", cfg.GoogleEmail)
        if cfg.GoogleEmail == "" {
            slog.Warn("Change Workbook with no email — running full wizard instead")
            // No email yet → fall through to full wizard (Continue setup… UX). Phase 1: just log.
            return
        }
        // Load the stored refresh token; build a refresh-only TokenSource.
        st, err := auth.ReadToken(cfg.GoogleEmail)
        if err != nil {
            slog.Error("read wincred token", "err", err)
            t.SetStatus("Change Workbook: token read failed")
            return
        }
        oauthCfg := auth.OAuthConfigForRefresh(auth.Config{OAuthClientID: bc.OAuthClientID})  // Plan 03 helper
        tok := &oauth2.Token{RefreshToken: st.RefreshToken}
        ts := oauth2.ReuseTokenSource(tok, oauthCfg.TokenSource(ctx, tok))

        // Bring up a fresh loopback listener — token is valid, no OAuth needed; we ONLY need
        // the picker route + a /done redirect to capture the picked spreadsheet id.
        ln, err := net.Listen("tcp", "127.0.0.1:0")
        if err != nil { slog.Error("change workbook: listen", "err", err); return }
        defer ln.Close()
        port := ln.Addr().(*net.TCPAddr).Port
        mux := http.NewServeMux()
        srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}

        // Build sheet client for ValidateWorkbook.
        sc, err := sheet.NewClient(ctx, ts, "")
        if err != nil { slog.Error("change workbook: sheet client", "err", err); return }

        pickerSrv := picker.NewServer(sc, ts, cfg, bc)
        pickerSrv.SetRedirectAfterPick("/changed")  // capture-only landing page
        done := make(chan struct{}, 1)
        pickerSrv.OnPicked(func() {
            slog.Info("change workbook: pick complete", "new_id", cfg.SpreadsheetID)
            done <- struct{}{}
        })
        pickerSrv.AttachRoutes(mux)
        mux.HandleFunc("/changed", func(w http.ResponseWriter, r *http.Request) {
            w.Write([]byte("<html><body>✓ Workbook changed. You can close this tab.</body></html>"))
        })

        go func() { _ = srv.Serve(ln) }()
        defer srv.Shutdown(context.Background())

        // Open browser straight to /picker (skipping OAuth — token already valid).
        startURL := fmt.Sprintf("http://127.0.0.1:%d/picker", port)
        if err := auth.OpenBrowser(startURL); err != nil {
            slog.Warn("change workbook: open browser", "err", err, "url", startURL)
        }

        // Wait for pick or ctx cancellation.
        select {
        case <-ctx.Done(): return
        case <-done:
            // pickerSrv.OnPicked already wrote cfg.SpreadsheetID via picker's ValidateWorkbook
            // path; surface the change in tray status.
            t.SetSpreadsheetID(cfg.SpreadsheetID)
            t.SetStatus(fmt.Sprintf("Workbook changed: %s", cfg.SpreadsheetID[:min(8, len(cfg.SpreadsheetID))]+"…"))
            slog.Info("change workbook: complete")
        case <-time.After(5 * time.Minute):
            slog.Warn("change workbook: timeout waiting for pick")
        }
    }

    func min(a, b int) int { if a < b { return a }; return b }

    // runWatcher loads the refresh token from wincred, builds a TokenSource + sheet.Client,
    // then runs the watcher loop dispatching parse/write/upsert per inventory event.
    func runWatcher(ctx context.Context, cfg *config.Config, t *tray.Controller) error {
        st, err := auth.ReadToken(cfg.GoogleEmail)
        if err != nil {
            return fmt.Errorf("read wincred token for %s: %w", cfg.GoogleEmail, err)
        }
        // Re-build a TokenSource from the stored refresh token. Use Plan 03's helper
        // (OAuthConfigForRefresh) so the scope set matches consent-time and refresh succeeds.
        oauthCfg := auth.OAuthConfigForRefresh(auth.Config{OAuthClientID: st.ClientID})  // Plan 03-owned helper
        tok := &oauth2.Token{RefreshToken: st.RefreshToken}
        ts := oauth2.ReuseTokenSource(tok, oauthCfg.TokenSource(ctx, tok))

        sc, err := sheet.NewClient(ctx, ts, cfg.SpreadsheetID)
        if err != nil {
            return fmt.Errorf("sheet client: %w", err)
        }
        // Re-validate workbook on startup — handles "user changed schema_version" case.
        if err := sc.ValidateWorkbook(ctx); err != nil {
            return fmt.Errorf("validate workbook on startup: %w", err)
        }

        t.SetSpreadsheetID(cfg.SpreadsheetID)
        t.SetIconHealth(tray.HealthGreen)
        t.SetStatus(fmt.Sprintf("Connected as %s — watching %s", cfg.GoogleEmail, filepath.Base(cfg.EQFolder)))

        onChange := func(path string) {
            charName := extractCharName(path)
            if charName == "" {
                slog.Warn("invalid inventory filename", "path", filepath.Base(path))
                return
            }
            // Always re-stat + re-read fresh (CLAUDE.md / RESEARCH.md §8.3).
            f, err := os.Open(path)
            if err != nil {
                slog.Error("open inventory", "path", filepath.Base(path), "err", err)
                return
            }
            rows, err := parse.Parse(f)
            f.Close()
            if err != nil {
                slog.Error("parse inventory", "char", charName, "err", err)
                return
            }
            if len(rows) == 0 {
                slog.Info("inventory empty (possibly mid-write or empty char)", "char", charName)
                return
            }
            uploadedAt := time.Now().UTC().Format(time.RFC3339)
            if err := sc.WriteInventory(ctx, charName, sheet.InventoryHeader, rows, uploadedAt); err != nil {
                slog.Error("write inventory", "char", charName, "err", err)
                t.SetStatus(fmt.Sprintf("Last upload failed: %s", charName))
                return
            }
            if err := sc.UpsertCharOwner(ctx, charName, cfg.GoogleEmail); err != nil {
                slog.Warn("upsert char_owner", "char", charName, "err", err)
                // not fatal — inv:Char write succeeded; surface warning, continue
            }
            slog.Info("uploaded", "char", charName, "rows", len(rows))
            t.SetStatus(fmt.Sprintf("Last upload: %s at %s", charName, time.Now().Format("15:04")))
        }

        return watch.Run(ctx, cfg.EQFolder, onChange)
    }

    func extractCharName(path string) string {
        base := filepath.Base(path)
        m := charNameRE.FindStringSubmatch(base)
        if len(m) != 2 { return "" }
        return m[1]
    }
    ```

    Replace `<owner>` with the actual module owner.

    NOTE: `auth.ReadToken(email)` and `auth.OAuthConfigForRefresh(auth.Config{...})` are exported
    by Plan 03 (Plan 03's `<interfaces>` block is the source of truth). If they don't compile,
    Plan 03 has a defect — surface to orchestrator; do NOT add them from inside this plan.

    Update `cmd/squirebot/main.go` to wire RunApp + tray + the D-04 OnChangeWorkbook callback:
    ```go
    package main

    import (
        "context"
        "log/slog"
        "os"

        "fyne.io/systray"

        "github.com/<owner>/squirebot/internal/app"
        "github.com/<owner>/squirebot/internal/auth"
        "github.com/<owner>/squirebot/internal/config"
        "github.com/<owner>/squirebot/internal/logging"
        "github.com/<owner>/squirebot/internal/tray"
    )

    func main() {
        log, logDir := logging.Setup()
        slog.SetDefault(log)

        cfg, err := config.Load()
        if err != nil {
            slog.Error("config load failed", "err", err); os.Exit(1)
        }

        bc := auth.BuildConstants{
            OAuthClientID:    OAuthClientID,
            PickerAPIKey:     PickerAPIKey,
            GCPProjectNumber: GCPProjectNumber,
        }

        ctx, cancel := context.WithCancel(context.Background())
        defer cancel()

        var trayCtl *tray.Controller
        trayCtl = tray.NewController(tray.Config{
            IconGreen:     iconBytes,
            IconRed:       iconBytes,             // Phase 5 polish
            LogDir:        logDir,
            SpreadsheetID: cfg.SpreadsheetID,
            OnContinueSetup: func() {
                slog.Info("Continue setup clicked")
                go app.RunApp(ctx, cfg, bc, trayCtl)
            },
            OnChangeWorkbook: func() {
                // D-04: re-run Picker on existing token. RESEARCH.md §5.6.
                slog.Info("Change Workbook clicked")
                go app.ChangeWorkbook(ctx, cfg, bc, trayCtl)
            },
            OnQuit: func() { cancel() },
        })

        // Background goroutine: wizard + watcher.
        go app.RunApp(ctx, cfg, bc, trayCtl)

        slog.Info("squirebot starting",
            "version", Version,
            "pid", os.Getpid(),
            "log_dir", logDir,
        )

        // Main goroutine: blocking systray loop. systray.Run takes onReady + onExit callbacks.
        systray.Run(trayCtl.OnReady, trayCtl.OnExit)
        cancel() // systray quit → tear down background.
    }
    ```

    The OnContinueSetup race-on-double-click is not addressed in Phase 1 (D-07 deferred polish);
    document as known-limitation in SUMMARY. Same goes for OnChangeWorkbook double-click.

    Add `internal/app/runapp_test.go` with at least:
    - Test `extractCharName` extracts "Foo" from "Foo-Inventory.txt", "Cool Toon-Inventory.txt", returns "" for "Foo-Spellbook.txt".
    - Test `needsWizard` returns true when any of email/spreadsheetID/eqFolder is empty.

    Verify cross-compile to Windows works with all the wiring:
    ```
    GOOS=windows GOARCH=amd64 go build -ldflags="-H=windowsgui -s -w \
      -X main.OAuthClientID=test-client \
      -X main.PickerAPIKey=test-key \
      -X main.GCPProjectNumber=test-num \
      -X main.Version=0.1.0-plan07" \
      -o dist/squirebot.exe ./cmd/squirebot
    ```
  </action>
  <verify>
    <automated>go build ./internal/... &amp;&amp; GOOS=windows GOARCH=amd64 go build -ldflags="-H=windowsgui -s -w -X main.OAuthClientID=test -X main.PickerAPIKey=test -X main.GCPProjectNumber=1234 -X main.Version=0.1.0-plan07" -o dist/squirebot.exe ./cmd/squirebot &amp;&amp; test -s dist/squirebot.exe &amp;&amp; go test ./internal/app/... -count=1 -timeout 30s &amp;&amp; go vet ./... &amp;&amp; grep -nE "watch\\.Run" internal/app/runapp.go &amp;&amp; grep -nE "parse\\.Parse" internal/app/runapp.go &amp;&amp; grep -nE "WriteInventory" internal/app/runapp.go &amp;&amp; grep -nE "UpsertCharOwner" internal/app/runapp.go &amp;&amp; grep -nE "wizard\\.NewServer" internal/app/runapp.go &amp;&amp; grep -nE "auth\\.ReadToken" internal/app/runapp.go &amp;&amp; grep -nE "auth\\.OAuthConfigForRefresh" internal/app/runapp.go &amp;&amp; grep -nE "ReuseTokenSource" internal/app/runapp.go &amp;&amp; grep -nE "func ChangeWorkbook" internal/app/runapp.go &amp;&amp; grep -nE "picker\\.NewServer" internal/app/runapp.go &amp;&amp; grep -nE "systray\\.Run" cmd/squirebot/main.go &amp;&amp; grep -nE "app\\.RunApp" cmd/squirebot/main.go &amp;&amp; grep -nE "OnChangeWorkbook" cmd/squirebot/main.go &amp;&amp; grep -nE "app\\.ChangeWorkbook" cmd/squirebot/main.go</automated>
  </verify>
  <acceptance_criteria>
    - `internal/app/runapp.go` exports `RunApp` and `ChangeWorkbook`
    - runapp.go calls `watch.Run`, `parse.Parse`, `WriteInventory`, `UpsertCharOwner`, `wizard.NewServer`, `auth.ReadToken`, `auth.OAuthConfigForRefresh`, `ReuseTokenSource` in that order in the wired pipeline (all Plan 03/04/05/06-owned symbols imported, never redefined)
    - runapp.go contains the regex `^(.+)-Inventory\.txt$` for char-name extraction
    - `ChangeWorkbook` calls `picker.NewServer`, `pickerSrv.AttachRoutes`, `auth.OpenBrowser` to /picker on a fresh loopback listener (D-04 / RESEARCH.md §5.6)
    - `cmd/squirebot/main.go` calls `systray.Run(trayCtl.OnReady, trayCtl.OnExit)` and runs `app.RunApp` in a background goroutine
    - main.go wires `OnChangeWorkbook: func() { go app.ChangeWorkbook(ctx, cfg, bc, trayCtl) }` (D-04)
    - `dist/squirebot.exe` cross-compiles successfully with the test ldflags
    - `go test ./internal/app/... -count=1` exits 0 (extractCharName + needsWizard tests)
    - `go vet ./...` exits 0 across the entire repo
    - `go build ./...` exits 0 across the entire repo
    - `git diff --stat internal/auth/oauth.go` (from Plan 07's branch) shows ZERO lines changed (file ownership invariant — Plan 07 does not touch Plan 03's file)
  </acceptance_criteria>
  <done>
    The end-to-end pipeline is wired. Running squirebot.exe with empty config opens the wizard;
    with full config, starts the watcher. Watcher events trigger parse → WriteInventory →
    UpsertCharOwner. Tray surfaces Status, Open Workbook, Open log folder, Change Workbook…
    (D-04), Continue setup, Quit. Change Workbook re-runs Picker on the existing token.
    squirebot.exe cross-compiles cleanly. internal/auth/oauth.go is unmodified (Plan 03 owns it).
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| watcher onChange ↔ filesystem | `path` arrives from fsnotify; we re-stat and re-open — must validate filename pattern (`-Inventory.txt$`) |
| RunApp goroutine ↔ tray click handlers | OnContinueSetup runs in tray goroutine and spawns RunApp again; race risk |
| ChangeWorkbook goroutine ↔ existing OAuth listener | D-04 re-launches picker on a fresh listener; must not race with RunApp's wizard listener if wizard is mid-flow |
| wincred ↔ runWatcher | Refresh token round-tripped from wincred to memory; must zero in defer |
| wizard.Server ↔ Plan 03 Manager | Shared listener; lifetime managed by wizard |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-07-01 | Spoofing | Watcher onChange called with a path that does NOT match `-Inventory.txt` (paranoid against Plan 04 regression) | mitigate | extractCharName returns "" on mismatch; runWatcher's onChange logs a warning and skips writes. Acceptance test exercises both inventory and spellbook filenames |
| T-07-02 | Information Disclosure | RefreshToken loaded into RunApp memory and never zeroed | mitigate | After ReuseTokenSource is built, the local `tok.RefreshToken = ""` zero is applied; the StoredToken value is dropped at the end of runWatcher and ChangeWorkbook |
| T-07-03 | Tampering | Race: tray's OnContinueSetup OR OnChangeWorkbook clicked twice → two flows contend for loopback ports | mitigate | Phase 1 documented as known-limitation; future fix is a sync.Once or single-flight wrapper around RunApp/ChangeWorkbook re-entry; click-to-click is rare in real use; logged for SUMMARY |
| T-07-04 | Denial of Service | Wizard hung indefinitely (user closed browser without completing) | mitigate | Plan 03's 60-second OAuth timer + ctx cancellation propagation; Plan 07 wizard.Run honors ctx.Done; user can also Quit from tray which cancels |
| T-07-05 | Tampering | Watcher catches a partial-write file (EQ flushing) → parse returns 0 rows → WriteInventory called with empty data, clearing the tab | mitigate | runWatcher checks `len(rows) == 0` BEFORE calling WriteInventory; logs and skips. Avoids "user lost their inventory because watcher ran during flush" — combined with Plan 04's 500ms debounce + Pitfall #10 |
| T-07-06 | Privilege Escalation | Tray menu allows opening arbitrary URLs via Open Workbook (if attacker controls config.SpreadsheetID) | accept | Config is local; if attacker has write access to %LOCALAPPDATA%\\SquireBot\\config.json, they have the user's machine — bigger problems. URL is hardcoded prefix `https://docs.google.com/spreadsheets/d/`, only the spreadsheet ID is variable |
| T-07-07 | Repudiation | No log of wizard step transitions | mitigate | runApp + wizard.handle* both slog.Info on each step (oauth completed, picker step complete, eq-folder confirmed, wizard done, change workbook complete) — verifiable in squirebot.log |
| T-07-08 | Information Disclosure | RunApp logs config.GoogleEmail (PII concern) | accept | Status string and slog calls include the email; this is the canonical identity per AUTH-06 and is required for diagnostics. Log file is local to the user's profile |
| T-07-09 | Tampering | Watcher onChange dispatches WriteInventory before ValidateWorkbook re-runs (e.g., if user replaced the workbook via Change Workbook…) | mitigate | runWatcher calls ValidateWorkbook ONCE at startup; ChangeWorkbook flow exits the runWatcher goroutine, picks new id, then user must restart app for the watcher to pick up the new id (Phase 1 limitation — Phase 2 polish will hot-swap). Documented as accepted Phase-1 limitation. |
| T-07-10 | Denial of Service | EQ folder deleted while watcher is running → fsnotify errors loop forever | mitigate | Plan 04 watcher's `case <-w.Errors` logs and continues; the worst case is no events arrive until the folder is restored; tray status shows last-upload-time staleness |
</threat_model>

<verification>
- `go build ./internal/...` exits 0 (interface alignment check; FIRST verify step) — proves Plans 01-06 packages compile together with their final exported interfaces
- `GOOS=windows GOARCH=amd64 go build ./cmd/squirebot/...` (cross-compile) exits 0
- `go vet ./...` exits 0
- `go test ./... -count=1 -timeout 60s` exits 0 (whole-repo test pass)
- `dist/squirebot.exe` compiled with test ldflags is a valid PE32+ executable
- `internal/app/runapp.go` correctly chains parse → WriteInventory → UpsertCharOwner per inventory event
- charNameRE extraction tested against "Foo-Inventory.txt", "Foo Bar-Inventory.txt", "Foo-Spellbook.txt"
- Tray menu ordering matches CONTEXT.md Claude's Discretion list (Status, Open Workbook, Open log folder, Change Workbook…, Continue setup [hidden], Quit)
- `grep -n "Change Workbook" internal/tray/tray.go` returns ≥1 match (D-04 enforcement)
- `grep -rE "time\\.Tick" --include="*.go" internal/` returns 0 matches (CLAUDE.md polling prohibition still respected)
- `grep -rE 'slog\\.(Info|Warn|Error|Debug).*\\b(RefreshToken|AccessToken|client_secret)\\b' --include="*.go" internal/ | grep -v "_test\\.go" | grep -v '^[^:]*:\\s*//'` returns 0 non-comment matches
- `git diff --stat internal/auth/oauth.go` (Plan 07 branch vs. Plan 03 baseline) shows ZERO lines changed — file ownership invariant
</verification>

<success_criteria>
- INST-03 satisfied (final): browser opens ONCE for OAuth (Plan 03 OpenBrowser call) and the same tab carries the user through Picker → eq-folder → done. No second browser launch.
- AUTH-06 satisfied (final): runWatcher.onChange calls UpsertCharOwner(charName, cfg.GoogleEmail) on every inventory write. The first sighting of a character creates the _char_owner row.
- OPS-01 satisfied (reinforced): runWatcher.onChange calls WriteInventory with the per-char tab name; OPS-01's "per-character non-overlapping ranges" rule is honored at the wiring layer (the only writes are the inv:&lt;Char&gt; tab + _char_owner upsert + _meta bootstrap, all on per-key ranges).
- D-04 satisfied (Phase 1 minimum): tray "Change Workbook…" menu item exists and re-runs Picker via app.ChangeWorkbook on the existing OAuth token; new spreadsheetID is written to config.json on successful pick + ValidateWorkbook.
- D-06 satisfied: wizard auto-launches on first run with /start; D-07 satisfied: Continue setup… restores the flow on demand.
- Tray surface matches CONTEXT.md Claude's Discretion list PLUS the D-04 Change Workbook… item.
- Watcher → parser → sheets pipeline is end-to-end-tested in unit form (extractCharName + needsWizard) and ready for Plan 08's full smoke checkpoint on a real machine.
- File ownership invariant: internal/auth/oauth.go is unmodified by this plan (Plan 03 ships the shared-listener API; Plan 07 only consumes it).
</success_criteria>

<output>
After completion, create `.planning/phases/01-end-to-end-thin-slice/01-07-SUMMARY.md` documenting:
- The exact wizard step → URL mapping (/start, /oauth/callback, /picker, /picker/result, /eq-folder, /done, /wizard/shutdown, /start_paste)
- The exact ChangeWorkbook flow (D-04): listener allocation, picker.NewServer attached on /picker, /changed landing page, OnPicked callback persisting cfg.SpreadsheetID
- Whether OnContinueSetup / OnChangeWorkbook re-entry races were observed and how handled
- The actual chain of slog.Info messages a successful run produces (for Plan 08 smoke verification)
- Any deviations from RESEARCH.md §5.6 / §11 / §12 and why
- A note for Plan 08 listing the smoke-test verification points: clean install on Win11 VM, single browser open, OAuth + picker + eq-folder, /outputfile inventory in EQ → see inv:&lt;Char&gt; in the workbook within 30s, Change Workbook… menu item appears and re-launches picker, refresh token in wincred, no token in config.json, 10-day refresh-token survival check
- Confirmation that internal/auth/oauth.go was NOT modified by this plan (file ownership invariant); the six Plan-03-owned symbols (NewManagerWithListener, AttachRoutes, AuthURL, HandlePastedRedirect, DoneChan, OAuthConfigForRefresh) were consumed by import only
</output>
