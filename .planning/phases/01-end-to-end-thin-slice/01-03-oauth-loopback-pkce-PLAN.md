---
phase: 01-end-to-end-thin-slice
plan: 03
type: execute
wave: 2
depends_on: [01, 02]
files_modified:
  - internal/auth/pkce.go
  - internal/auth/pkce_test.go
  - internal/auth/oauth.go
  - internal/auth/store.go
  - internal/auth/store_test.go
  - internal/auth/userinfo.go
  - internal/auth/browser.go
  - internal/auth/oauthconfig.go
  - cmd/squirebot/build_constants.go
files_modified_notes: |
  cmd/squirebot/main.go is NOT modified by this plan — Plan 07 wires runApp() that calls auth.
  Plan 03 only delivers the auth package + the build constants Plan 07 imports.
  Plan 07 CONSUMES (does not extend) the shared-listener API surface defined in <interfaces>:
  NewManagerWithListener / AttachRoutes / AuthURL / HandlePastedRedirect / DoneChan /
  OAuthConfigForRefresh — all shipped by THIS plan, not by Plan 07.
autonomous: true
requirements: [AUTH-01, AUTH-02, AUTH-04, AUTH-06]
must_haves:
  truths:
    - "User runs `squirebot.exe oauth` (or invokes the auth package from a test) and a browser opens to accounts.google.com with redirect_uri=http://127.0.0.1:&lt;port&gt;/oauth/callback (NOT localhost) and scope=drive.file+openid+userinfo.email"
    - "After consent, the loopback server captures the code and exchanges it for a refresh token in &lt;5 seconds"
    - "The refresh token lands in Windows Credential Manager under target name `SquireBot:&lt;email&gt;`"
    - "%LOCALAPPDATA%\\SquireBot\\config.json contains the cached google_email but NOT the refresh token"
    - "config.json subjected to grep for `refresh_token`-shaped values returns zero matches"
    - "Plan 03 ships a shared-listener API (NewManagerWithListener / AttachRoutes / AuthURL / HandlePastedRedirect / DoneChan / OAuthConfigForRefresh) that Plan 07 consumes WITHOUT modifying internal/auth/oauth.go"
  artifacts:
    - path: "internal/auth/pkce.go"
      provides: "PKCE code_verifier + S256 code_challenge generation per RFC 7636"
      contains: "base64.RawURLEncoding.EncodeToString"
    - path: "internal/auth/oauth.go"
      provides: "Loopback HTTP listener on 127.0.0.1:0, OAuth URL builder, /oauth/callback handler, token exchange, redirect to /picker; PLUS shared-listener API for Plan 07"
      min_lines: 160
      contains: "http.Server"
    - path: "internal/auth/store.go"
      provides: "wincred GetGenericCredential / NewGenericCredential / Delete wrappers under target prefix `SquireBot:`"
      contains: "wincred"
    - path: "internal/auth/userinfo.go"
      provides: "GetUserEmail using google.golang.org/api/oauth2/v2 Userinfo endpoint"
      contains: "Userinfo"
    - path: "internal/auth/oauthconfig.go"
      provides: "Loads the three Cloud Console constants from build-time -ldflags variables"
      contains: "OAuthClientID"
  key_links:
    - from: "internal/auth/oauth.go"
      to: "internal/auth/store.go"
      via: "StoreToken called inside /oauth/callback after Exchange"
      pattern: "store\\.StoreToken\\("
    - from: "internal/auth/oauth.go"
      to: "internal/auth/userinfo.go"
      via: "GetUserEmail called after Exchange to discover the canonical identity"
      pattern: "userinfo\\.GetUserEmail\\("
    - from: "internal/auth/oauth.go"
      to: "internal/config/config.go"
      via: "config.GoogleEmail is set after userinfo lookup; config.Save persists"
      pattern: "config\\..*GoogleEmail"
    - from: "internal/auth/store.go"
      to: "github.com/danieljoos/wincred"
      via: "NewGenericCredential / GetGenericCredential / Delete"
      pattern: "wincred\\.(NewGenericCredential|GetGenericCredential)"
---

<objective>
Build the OAuth 2.0 loopback PKCE flow end-to-end. After this plan, a single Go function call
(`auth.RunOAuth(ctx, cfg)`) opens the user's default browser to Google's consent screen on
`127.0.0.1:&lt;random-ephemeral-port&gt;`, captures the redirect, exchanges the code, looks up the
canonical email via `userinfo`, and writes the refresh token into Windows Credential Manager
under target `SquireBot:&lt;email&gt;` — leaving zero credentials in any plaintext file.

Plan 03 ALSO ships the shared-listener API surface that Plan 07's wizard consumes without
extending oauth.go: `NewManagerWithListener`, `AttachRoutes`, `AuthURL`, `HandlePastedRedirect`,
`DoneChan`, and the helper `OAuthConfigForRefresh`. Shipping these from the start avoids forcing
Plan 07 to modify internal/auth/oauth.go (file-ownership conflict that the plan-checker flagged).

Purpose: This plan satisfies four of the twelve Phase 1 requirements (AUTH-01, AUTH-02, AUTH-04,
AUTH-06) and unblocks Plans 05 (Sheets writer needs an oauth2.TokenSource), 06 (Drive Picker
needs the same access token issued from this flow), and 07 (wizard composes the loopback HTTP
mux from this plan's AttachRoutes + picker.AttachRoutes + wizard's own routes). Without it, no
other plan can authenticate to Google.

Output: A self-contained `internal/auth/` package with a tested PKCE generator, a tested wincred
roundtrip, and an oauth.go that orchestrates the flow against the Cloud project provisioned by
Plan 02. Build constants are loaded via -ldflags from oauth-config.json.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/phases/01-end-to-end-thin-slice/01-CONTEXT.md
@.planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md
@.planning/phases/01-end-to-end-thin-slice/oauth-config.json
@.planning/phases/01-end-to-end-thin-slice/01-01-SUMMARY.md
@.planning/research/STACK.md
@./CLAUDE.md
@internal/config/config.go
@internal/logging/logger.go
</context>

<interfaces>
<!-- Contracts this plan exports for downstream plans (05, 06, 07). -->
<!-- Plan 07 must consume these as-named without modifying internal/auth/oauth.go. -->

From internal/auth/oauth.go:
```go
package auth

// Manager owns one in-flight OAuth flow. Single use; do not reuse after RunOAuth returns.
type Manager struct {
    cfg          *oauth2.Config
    codeVerifier string
    expectedState string
    listener     net.Listener
    server       *http.Server
    config       *config.Config // for cache writeback
    redirectAfterCallback string  // "/picker" — Plan 06 will own /picker route on the same server
    done         chan OAuthResult
}

type OAuthResult struct {
    Email        string
    RefreshToken string                // in-memory only; for handing to TokenSource creators
    TokenSource  oauth2.TokenSource    // ready-to-use; uses ReuseTokenSource for refresh hygiene
    Listener     net.Listener          // hand-off to picker.Server (Plan 06) which keeps the server alive for /picker route
    Server       *http.Server          // ditto
    Port         int
    Err          error
}

// NewManager constructs a Manager that owns its own listener (used by `squirebot.exe oauth`
// standalone testing). cfg must be a *config.Config; the function will write GoogleEmail to it
// on success.
func NewManager(cfg *config.Config) (*Manager, error)

// NewManagerWithListener constructs a Manager that SHARES a caller-owned listener and mux.
// Used by Plan 07's wizard which boots one loopback listener for OAuth + Picker + Wizard pages.
// The Manager will NOT call ListenAndServe; the caller is responsible for serving on `listener`.
// `bc` carries the OAuth client ID + Picker constants loaded from -ldflags.
func NewManagerWithListener(cfg *config.Config, bc BuildConstants, listener net.Listener) *Manager

// AttachRoutes registers the OAuth-side routes (/oauth/callback, /start_paste internal helper)
// on a caller-owned mux. NewManagerWithListener + AttachRoutes is the shared-listener pattern.
// AttachRoutes does NOT register /start — the wizard owns that page (it links to AuthURL()).
func (m *Manager) AttachRoutes(mux *http.ServeMux)

// AuthURL returns the consent URL with PKCE parameters (state, code_challenge, code_challenge_method,
// access_type=offline, prompt=consent). Wizard's start.html template calls this.
func (m *Manager) AuthURL() string

// HandlePastedRedirect parses `code` + `state` from a redirect URL the user manually pasted
// (AUTH-01 60-second manual-paste fallback) and runs the same callback handler logic.
// Returns nil on success (DoneChan will fire); error if state mismatches or code missing.
func (m *Manager) HandlePastedRedirect(ctx context.Context, raw string) error

// DoneChan returns the channel that will receive the OAuthResult when the flow completes
// (success or terminal error). Used by Plan 07's wizard to await OAuth completion before
// attaching Plan 06's picker routes.
func (m *Manager) DoneChan() <-chan OAuthResult

// RunOAuth opens the browser and blocks until the user completes (or cancels) consent OR ctx is cancelled.
// On success, refresh token is in wincred and OAuthResult.TokenSource is non-nil.
// IMPORTANT: the returned Listener and Server are STILL ALIVE — Plan 06 picks them up to serve /picker.
// Caller MUST eventually call result.Server.Shutdown() (Plan 07 wizard does this when wizard completes).
// RunOAuth is the convenience wrapper for standalone OAuth testing; the wizard uses
// NewManagerWithListener + AttachRoutes + DoneChan instead.
func (m *Manager) RunOAuth(ctx context.Context) OAuthResult

// OAuthConfigForRefresh returns the *oauth2.Config used for refresh-only flows (no listener,
// no PKCE — just refresh_token → access_token). Plan 07's runWatcher rebuilds a TokenSource
// from a stored refresh token using this helper. `cfg` carries the OAuth client ID; scopes
// match the consent-time scope set so refresh succeeds.
func OAuthConfigForRefresh(cfg Config) *oauth2.Config

// Config is the minimal config view OAuthConfigForRefresh needs (so callers don't have to
// import auth.BuildConstants for refresh-only paths). One of {Config, BuildConstants} works.
type Config struct {
    OAuthClientID string
}
```

From internal/auth/store.go:
```go
package auth

const CredPrefix = "SquireBot:" // target name pattern: SquireBot:<email>

type StoredToken struct {
    RefreshToken string `json:"refresh_token"`
    Email        string `json:"email"`
    ClientID     string `json:"client_id"`
}

func StoreToken(email string, st StoredToken) error
func ReadToken(email string) (StoredToken, error)
func DeleteToken(email string) error
```

From internal/auth/userinfo.go:
```go
package auth

func GetUserEmail(ctx context.Context, ts oauth2.TokenSource) (string, error)
```

From internal/auth/pkce.go:
```go
package auth

// NewPKCEPair returns base64url-NoPadding 43-char verifier + S256 challenge per RFC 7636.
func NewPKCEPair() (verifier, challenge string, err error)
```

From cmd/squirebot/build_constants.go:
```go
package main

// Set via go build -ldflags="-X main.OAuthClientID=... -X main.PickerAPIKey=... -X main.GCPProjectNumber=..."
var (
    OAuthClientID    = ""    // from oauth-config.json
    PickerAPIKey     = ""    // from oauth-config.json (Plan 06 uses this)
    GCPProjectNumber = ""    // from oauth-config.json (Plan 06 uses this as Picker AppID)
    Version          = "0.1.0-dev"
)
```
</interfaces>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: PKCE generator + build-constant bridge</name>
  <files>internal/auth/pkce.go, internal/auth/pkce_test.go, internal/auth/oauthconfig.go, cmd/squirebot/build_constants.go</files>
  <read_first>
    - .planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md (§4.3 PKCE generation lines 493-519 — copy code verbatim; §A "Code Examples" reproduces this; RFC 7636 cite at line 519)
    - .planning/phases/01-end-to-end-thin-slice/oauth-config.json (just populated by Plan 02 — three constants here)
    - .planning/phases/01-end-to-end-thin-slice/01-CONTEXT.md (decisions D-13 about unsigned binary; the constants here are public-safe per Plan 02 rationale)
  </read_first>
  <behavior>
    - NewPKCEPair returns a verifier of length exactly 43 (32 bytes base64url-NoPadding-encoded)
    - The verifier matches the RFC 7636 character class `^[A-Za-z0-9_-]+$` (NoPadding base64url)
    - The challenge is base64url-NoPadding(SHA256(verifier))
    - Calling NewPKCEPair 1000 times produces 1000 distinct verifiers (entropy sanity check)
    - oauthconfig.go LoadFromBuildConstants returns ErrMissingConstants if any of the three -ldflags vars is empty
  </behavior>
  <action>
    Create `internal/auth/pkce.go` containing the EXACT NewPKCEPair function from
    01-RESEARCH.md §4.3. Use `crypto/rand`, `crypto/sha256`, `encoding/base64`. Reproduce verbatim:
    ```go
    package auth

    import (
        "crypto/rand"
        "crypto/sha256"
        "encoding/base64"
    )

    func NewPKCEPair() (verifier, challenge string, err error) {
        b := make([]byte, 32)
        if _, err = rand.Read(b); err != nil {
            return "", "", err
        }
        verifier = base64.RawURLEncoding.EncodeToString(b)
        sum := sha256.Sum256([]byte(verifier))
        challenge = base64.RawURLEncoding.EncodeToString(sum[:])
        return verifier, challenge, nil
    }
    ```

    Create `internal/auth/pkce_test.go` with the four tests in &lt;behavior&gt; above. Use a regexp for
    the character class check.

    Create `cmd/squirebot/build_constants.go` per &lt;interfaces&gt;:
    ```go
    package main

    var (
        OAuthClientID    = ""
        PickerAPIKey     = ""
        GCPProjectNumber = ""
        Version          = "0.1.0-dev"
    )
    ```

    Create `internal/auth/oauthconfig.go`:
    ```go
    package auth

    import "errors"

    var ErrMissingConstants = errors.New("auth: build-time OAuth constants missing — rebuild with -ldflags='-X main.OAuthClientID=... -X main.PickerAPIKey=... -X main.GCPProjectNumber=...' (per docs/oauth-setup.md)")

    // BuildConstants holds the three values the binary needs to talk to Google.
    // Each is loaded from -ldflags at build time. The binary refuses to run OAuth without them.
    type BuildConstants struct {
        OAuthClientID    string
        PickerAPIKey     string
        GCPProjectNumber string
    }

    func (b BuildConstants) Validate() error {
        if b.OAuthClientID == "" || b.PickerAPIKey == "" || b.GCPProjectNumber == "" {
            return ErrMissingConstants
        }
        return nil
    }
    ```
    Plan 07 (main wiring) will populate this struct from the package-main vars and pass into auth.

    Update the `.github/workflows/release.yml` build step (touched in Plan 01 Task 3) to read
    oauth-config.json via `jq -r` and pass the three values via `-X main.X=Y` ldflags. If
    oauth-config.json is gitignored or missing in CI, fail the build with a clear error pointing
    to docs/oauth-setup.md. Concretely add to the build step BEFORE the `go build` line:
    ```yaml
          - name: Load OAuth constants from oauth-config.json
            id: oauth
            run: |
              echo "OAUTH_CLIENT_ID=$(jq -r '.oauth_client_id' .planning/phases/01-end-to-end-thin-slice/oauth-config.json)" &gt;&gt; $env:GITHUB_ENV
              echo "PICKER_API_KEY=$(jq -r '.picker_api_key' .planning/phases/01-end-to-end-thin-slice/oauth-config.json)" &gt;&gt; $env:GITHUB_ENV
              echo "GCP_PROJECT_NUMBER=$(jq -r '.gcp_project_number' .planning/phases/01-end-to-end-thin-slice/oauth-config.json)" &gt;&gt; $env:GITHUB_ENV
    ```
    And replace the build line with one that adds:
    `-X main.OAuthClientID=$env:OAUTH_CLIENT_ID -X main.PickerAPIKey=$env:PICKER_API_KEY -X main.GCPProjectNumber=$env:GCP_PROJECT_NUMBER`.
    Document a local Linux/macOS build line in README.md too:
    ```
    eval $(jq -r '"-X main.OAuthClientID=" + .oauth_client_id + " -X main.PickerAPIKey=" + .picker_api_key + " -X main.GCPProjectNumber=" + .gcp_project_number' .planning/phases/01-end-to-end-thin-slice/oauth-config.json | xargs -I{} echo LDFLAGS_OAUTH=\"{}\")
    GOOS=windows GOARCH=amd64 go build -ldflags="-H=windowsgui -s -w $LDFLAGS_OAUTH" -o dist/squirebot.exe ./cmd/squirebot
    ```
  </action>
  <verify>
    <automated>go test ./internal/auth/... -run TestPKCE -count=1 -timeout 30s &amp;&amp; grep -nE "base64\.RawURLEncoding\.EncodeToString" internal/auth/pkce.go &amp;&amp; grep -nE "OAuthClientID\s*=" cmd/squirebot/build_constants.go &amp;&amp; grep -nE "ErrMissingConstants" internal/auth/oauthconfig.go</automated>
  </verify>
  <acceptance_criteria>
    - `internal/auth/pkce.go` exists and contains literal `base64.RawURLEncoding.EncodeToString`
    - `internal/auth/pkce.go` contains `sha256.Sum256`
    - `internal/auth/pkce_test.go` contains at least 4 `func Test` declarations
    - `go test ./internal/auth/... -run TestPKCE -count=1` exits 0
    - `cmd/squirebot/build_constants.go` declares package-level vars `OAuthClientID`, `PickerAPIKey`, `GCPProjectNumber`, `Version`
    - `internal/auth/oauthconfig.go` declares `ErrMissingConstants` and `BuildConstants` with `Validate()`
    - README.md contains a local-build invocation line that uses `-X main.OAuthClientID`
  </acceptance_criteria>
  <done>
    PKCE generator is RFC 7636 compliant and tested. Build constants pipeline (oauth-config.json
    -&gt; -ldflags -&gt; package main vars -&gt; auth.BuildConstants) is in place and CI documents how
    to use it.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: wincred-backed token store + userinfo email lookup</name>
  <files>internal/auth/store.go, internal/auth/store_test.go, internal/auth/userinfo.go, internal/auth/browser.go</files>
  <read_first>
    - .planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md (§4.7 wincred storage shape lines 583-622 — full StoredToken + Store/Read/Delete code blocks; §12.1 GetUserEmail lines 1182-1196; §11 tray section's exec.Command pattern for browser launch)
    - ./CLAUDE.md ("DPAPI via wincred: Refresh token must NOT live in %LOCALAPPDATA%\\SquireBot\\config.json")
    - internal/config/config.go (Plan 01 — confirm Save returns error and works on tmp+rename)
  </read_first>
  <behavior>
    - StoreToken("alice@gmail.com", st) writes a wincred entry under target `SquireBot:alice@gmail.com`
    - ReadToken("alice@gmail.com") returns the same StoredToken back
    - ReadToken("does-not-exist@gmail.com") returns the wincred not-found error
    - DeleteToken("alice@gmail.com") followed by ReadToken returns not-found
    - StoreToken's underlying CredentialBlob is JSON-marshalled StoredToken (not a string concat)
    - Store_test uses build tag `windows` and t.Skip on non-Windows OS so CI on Linux doesn't break
  </behavior>
  <action>
    Create `internal/auth/store.go` per &lt;interfaces&gt; using the EXACT 01-RESEARCH.md §4.7 code:
    ```go
    package auth

    import (
        "encoding/json"
        "github.com/danieljoos/wincred"
    )

    const CredPrefix = "SquireBot:"

    type StoredToken struct {
        RefreshToken string `json:"refresh_token"`
        Email        string `json:"email"`
        ClientID     string `json:"client_id"`
    }

    func StoreToken(email string, st StoredToken) error {
        blob, err := json.Marshal(st)
        if err != nil { return err }
        cred := wincred.NewGenericCredential(CredPrefix + email)
        cred.CredentialBlob = blob
        cred.Persist = wincred.PersistLocalMachine
        return cred.Write()
    }
    func ReadToken(email string) (StoredToken, error) {
        cred, err := wincred.GetGenericCredential(CredPrefix + email)
        if err != nil { return StoredToken{}, err }
        var st StoredToken
        if err := json.Unmarshal(cred.CredentialBlob, &st); err != nil {
            return StoredToken{}, err
        }
        return st, nil
    }
    func DeleteToken(email string) error {
        cred, err := wincred.GetGenericCredential(CredPrefix + email)
        if err != nil { return err }
        return cred.Delete()
    }
    ```

    Create `internal/auth/store_test.go` (build-tagged `//go:build windows`) covering Behavior
    cases. Use a unique email per test (`fmt.Sprintf("squirebot-test-%d@example.invalid", time.Now().UnixNano())`)
    and `t.Cleanup(func() { _ = DeleteToken(email) })`. On non-Windows: file simply doesn't compile
    so tests don't run.

    Create `internal/auth/userinfo.go` per &lt;interfaces&gt; using 01-RESEARCH.md §12.1 lines 1183-1193:
    ```go
    package auth

    import (
        "context"
        oauth2v2 "google.golang.org/api/oauth2/v2"
        "google.golang.org/api/option"
        "golang.org/x/oauth2"
    )

    func GetUserEmail(ctx context.Context, ts oauth2.TokenSource) (string, error) {
        svc, err := oauth2v2.NewService(ctx, option.WithTokenSource(ts))
        if err != nil { return "", err }
        info, err := svc.Userinfo.Get().Context(ctx).Do()
        if err != nil { return "", err }
        return info.Email, nil
    }
    ```

    Create `internal/auth/browser.go` with a small helper that opens the user's default browser
    on Windows. Per RESEARCH.md §1 ("Supporting") — `os/exec` + `rundll32 url.dll,FileProtocolHandler`:
    ```go
    package auth

    import (
        "os/exec"
        "runtime"
    )

    // OpenBrowser launches the system default browser to the given URL.
    // Returns an error if the launcher could not be spawned (the user may still need to
    // navigate manually — RESEARCH.md §4 manual-paste fallback within 60s — Plan 07 owns the UI fallback).
    func OpenBrowser(url string) error {
        switch runtime.GOOS {
        case "windows":
            return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
        case "darwin":
            return exec.Command("open", url).Start()
        default:
            return exec.Command("xdg-open", url).Start()
        }
    }
    ```
    The `darwin`/`xdg-open` branches exist for `go test` developer ergonomics; production binary
    is Windows-only.

    NOTE: log no token contents anywhere. The slog logger MAY log `email` and `client_id` but MUST
    NOT log `refresh_token`. Pitfall #2 / Threat T-01-01.
  </action>
  <verify>
    <automated>go vet ./internal/auth/... &amp;&amp; grep -nE "wincred\.NewGenericCredential" internal/auth/store.go &amp;&amp; grep -nE "PersistLocalMachine" internal/auth/store.go &amp;&amp; grep -nE "Userinfo\.Get" internal/auth/userinfo.go &amp;&amp; grep -nE "rundll32" internal/auth/browser.go</automated>
  </verify>
  <acceptance_criteria>
    - `internal/auth/store.go` contains the literal `CredPrefix = "SquireBot:"`
    - `internal/auth/store.go` contains `wincred.PersistLocalMachine`
    - `internal/auth/store.go` exports StoreToken, ReadToken, DeleteToken
    - `internal/auth/store_test.go` has a `//go:build windows` build tag at the top
    - `internal/auth/userinfo.go` contains `oauth2v2.NewService`
    - `internal/auth/browser.go` contains `rundll32` literal for Windows
    - `grep -nE "slog\.(Info|Error|Warn|Debug).*RefreshToken" internal/auth/` returns 0 matches
    - `grep -nE "RefreshToken" internal/auth/store.go` returns matches (it IS in the struct, that's correct — wincred IS where it lives)
    - `grep -rE "RefreshToken" internal/auth/oauth.go internal/auth/userinfo.go internal/auth/browser.go internal/auth/oauthconfig.go internal/auth/pkce.go 2&gt;/dev/null | grep -v "^\\s*//"` should return matches ONLY in oauth.go (where it transits from cfg.Exchange to StoreToken) — verify by inspection
    - `go vet ./internal/auth/...` exits 0
  </acceptance_criteria>
  <done>
    wincred wrappers + userinfo lookup + browser launcher are in place, tested where automatable
    (Windows tests skip on Linux). No package outside of store.go and oauth.go ever touches the
    refresh token bytes.
  </done>
</task>

<task type="auto" tdd="false">
  <name>Task 3: OAuth Manager — loopback HTTP server, URL builder, callback handler, token exchange, shared-listener API</name>
  <files>internal/auth/oauth.go</files>
  <read_first>
    - .planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md (§4 entire section: §4.1 endpoints/redirect URI lines 470-478, §4.2 scope set lines 480-490, §4.4 authorization URL lines 521-540, §4.5 token exchange lines 540-570; §B "Code Examples" callback handler lines 1430-1455; §2.4 Pattern 2 loopback as both OAuth+wizard server lines 369-389)
    - internal/auth/store.go (just created — confirm signatures)
    - internal/auth/userinfo.go (just created — confirm signature)
    - internal/auth/pkce.go (just created — confirm signature)
    - internal/auth/oauthconfig.go (just created — BuildConstants struct)
    - internal/config/config.go (Plan 01 — Config.GoogleEmail field; Save method)
    - .planning/phases/01-end-to-end-thin-slice/01-CONTEXT.md (D-07 wizard dismissible mid-flow; D-13 unsigned binary)
  </read_first>
  <action>
    Create `internal/auth/oauth.go` implementing `Manager` and the FULL shared-listener API surface
    per &lt;interfaces&gt;. The seven exports MUST ship from this plan; Plan 07 will consume them
    without editing this file.

    The implementation MUST:

    1. **Listener allocation (NewManager path):** call `net.Listen("tcp", "127.0.0.1:0")` — let
       the kernel pick an ephemeral port in 49152-65535. Read the actual port back via
       `listener.Addr().(*net.TCPAddr).Port`. Use the `127.0.0.1` literal everywhere — never
       `localhost` (Pitfall #6 from RESEARCH.md §11).

    1b. **Listener handoff (NewManagerWithListener path):** the caller (Plan 07 wizard) hands in
        an already-listening `net.Listener`. The Manager records it but does NOT start its own
        http.Server — the caller's `http.Server.Serve(listener)` drives traffic. The Manager's
        AttachRoutes call wires `/oauth/callback` and `/start_paste` onto the caller's
        `*http.ServeMux`. NewManagerWithListener does NOT call OpenBrowser; the wizard's /start
        page does that via `<a href="{{ .AuthURL }}">`.

    2. **State + verifier:** generate a random 32-byte `state` value (base64url-encoded) for CSRF
       protection. Generate the PKCE pair via `NewPKCEPair()`. Both NewManager and
       NewManagerWithListener generate fresh state+verifier per Manager instance.

    3. **OAuth Config:** build an `oauth2.Config` exactly as RESEARCH.md §4.5 lines 551-561:
       ```go
       cfg := &oauth2.Config{
           ClientID:    bc.OAuthClientID,
           // ClientSecret left empty for desktop (PKCE)
           RedirectURL: fmt.Sprintf("http://127.0.0.1:%d/oauth/callback", port),
           Endpoint:    google.Endpoint,
           Scopes: []string{
               "https://www.googleapis.com/auth/drive.file",
               "openid",
               "https://www.googleapis.com/auth/userinfo.email",
           },
       }
       ```

    4. **Authorization URL (AuthURL method):** assemble per §4.4 lines 525-535, with
       `code_challenge`, `code_challenge_method=S256`, `access_type=offline`, `prompt=consent`,
       `state`. The exported `AuthURL() string` method returns this string for the wizard's
       start.html template:
       ```go
       func (m *Manager) AuthURL() string {
           return m.cfg.AuthCodeURL(m.expectedState,
               oauth2.AccessTypeOffline,
               oauth2.SetAuthURLParam("code_challenge", m.codeChallenge),
               oauth2.SetAuthURLParam("code_challenge_method", "S256"),
               oauth2.SetAuthURLParam("prompt", "consent"),
           )
       }
       ```

    5. **HTTP routes (AttachRoutes method) — register on the caller-owned mux:**
       - `GET /oauth/callback` — handler from §B lines 1431-1454:
         a. Validate `state == m.expectedState`. On mismatch: 400 "CSRF: state mismatch".
            (V2 Authentication / Threat T-03-01.)
         b. Read `code` from query string. On empty: 400 "No code in callback".
         c. Call `cfg.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", verifier))`.
         d. Build a token source: `ts := oauth2.ReuseTokenSource(tok, cfg.TokenSource(ctx, tok))`
            (Pitfall #2 — let the lib handle refresh).
         e. Call `userinfo.GetUserEmail(ctx, ts)` to discover the canonical email.
         f. Call `store.StoreToken(email, store.StoredToken{RefreshToken: tok.RefreshToken, Email: email, ClientID: cfg.ClientID})`.
         g. Update `m.config.GoogleEmail = email` and call `m.config.Save()`.
         h. Reply with HTTP 302 redirect to `/picker` (Plan 06 will own the /picker route on the
            same listener; Plan 03 just sets up the redirect target — the route handler returns
            404 in Plan 03's standalone testing, which is fine).
         i. Send `OAuthResult{Email, RefreshToken: tok.RefreshToken (in-memory only), TokenSource: ts, Listener, Server, Port, Err: nil}` on `m.done`.
       - `POST /start_paste` — receives form fields `redirect_url` from the wizard's manual-paste
         textarea (AUTH-01). Internally calls `m.HandlePastedRedirect(ctx, raw)` and on success
         redirects to /picker. On error: 400 with the parsed error message.

       AttachRoutes does NOT register `/start` — the wizard owns that route and uses AuthURL() to
       compose its own page. In NewManager (standalone testing) the Manager DOES register a tiny
       fallback `/start` page on its private mux for `squirebot.exe oauth` standalone invocation:
       a placeholder `<a href="{authURL}">Connect Google</a>` so the dev can test in isolation.
       In NewManagerWithListener mode the wizard's start.html supersedes it.

    5b. **HandlePastedRedirect method:** parses a pasted Google redirect URL of the form
         `http://127.0.0.1:<port>/oauth/callback?code=...&state=...&scope=...`, extracts code+state,
         then runs the same body as the /oauth/callback handler (steps a-i above) synchronously.
         Returns the same OAuthResult on m.done. Used by /start_paste route AND by callers who
         want to inject pasted URLs programmatically.

    5c. **DoneChan method:** returns `m.done` as a receive-only channel:
         `func (m *Manager) DoneChan() <-chan OAuthResult { return m.done }`. Wizard awaits this.

    6. **Browser launch:** in RunOAuth (NewManager path only), call `OpenBrowser(authURL)` after
       the listener is up. If OpenBrowser returns an error, do NOT fail the whole flow — instead
       log a slog.Warn and return an OAuthResult with `Err: errors.New("could not auto-open
       browser; copy this URL manually: ..."+authURL)` so Plan 07 can show the manual-paste
       fallback per AUTH-01 (60-second window). NewManagerWithListener does NOT open the browser
       — that's the wizard's job (/start page redirects).

    7. **Manual-paste fallback timeout (AUTH-01):** start a `time.AfterFunc(60*time.Second, ...)` —
       if no callback arrives within 60s, log a slog.Warn. Plan 07's wizard owns the textarea UI;
       Plan 03 implements the timer + `HandlePastedRedirect` + `/start_paste` route. This is
       exercised by AUTH-01's "manual-paste fallback if the redirect doesn't land within 60 seconds"
       requirement.

    8. **OAuthConfigForRefresh helper (package-level function, NOT a method):**
       ```go
       // Config carries the minimum the helper needs (avoids importing BuildConstants for callers
       // that only have a stored client_id from wincred).
       type Config struct {
           OAuthClientID string
       }

       // OAuthConfigForRefresh returns the *oauth2.Config used for refresh-only flows: no listener,
       // no PKCE, just refresh_token → access_token. Used by Plan 07's runWatcher to rebuild a
       // TokenSource from a stored refresh token without re-running the consent flow.
       // Scope set MUST match the consent-time scope set so refresh succeeds.
       func OAuthConfigForRefresh(cfg Config) *oauth2.Config {
           return &oauth2.Config{
               ClientID: cfg.OAuthClientID,
               Endpoint: google.Endpoint,
               Scopes: []string{
                   "https://www.googleapis.com/auth/drive.file",
                   "openid",
                   "https://www.googleapis.com/auth/userinfo.email",
               },
           }
       }
       ```
       This is a pure function — no listener, no state, safe to call from any goroutine.

    9. **Logging hygiene (Threat T-01-01):** structured slog calls allowed:
       - `slog.Info("oauth started", "port", port, "state", state[:8]+"...")`
       - `slog.Info("oauth callback received", "email", email)` (NEVER log token bytes)
       - `slog.Info("token stored in wincred", "email", email)`
       - `slog.Error("oauth exchange failed", "err", err)` — err message is OAuth lib's; no token contents
       NEVER log: `tok.RefreshToken`, `tok.AccessToken`, `code`, `verifier`.

    10. **RunOAuth signature (NewManager path):** blocks until either (a) callback completes (success
        or error sent to `m.done`), (b) ctx is cancelled, or (c) the 60s manual-paste timer fires
        AND the user later posts via /start_paste. Return the OAuthResult; do NOT shut down the
        listener — Plan 06 picks it up via the OAuthResult.Listener / .Server fields.

    Add `internal/auth/oauth_test.go` if practical: a unit test for the URL-builder logic ("the URL
    contains all 8 expected query params") is quick and useful:
    ```go
    func TestAuthURLContainsAllRequiredParams(t *testing.T) {
        // build URL via cfg.AuthCodeURL + verifier; parse with net/url; assert each key is present
    }
    ```
    Required keys: client_id, redirect_uri, response_type, scope, state, code_challenge,
    code_challenge_method, access_type, prompt.

    Also add a unit test for `OAuthConfigForRefresh`:
    ```go
    func TestOAuthConfigForRefreshHasMatchingScopes(t *testing.T) {
        cfg := OAuthConfigForRefresh(Config{OAuthClientID: "test"})
        // assert scopes slice is exactly the three consent-time scopes
    }
    ```

    Document in oauth.go header comment the fact that `tok.RefreshToken` does NOT remain in any
    Go variable after StoreToken returns — the OAuthResult.RefreshToken field is set then zeroed:
    ```go
    result.RefreshToken = "" // do not retain in struct after store
    ```
    Use a closure to zero the local `tok` after StoreToken returns:
    ```go
    defer func() { tok.RefreshToken = ""; tok.AccessToken = "" }()
    ```
  </action>
  <verify>
    <automated>go vet ./internal/auth/... &amp;&amp; go build ./internal/auth/... &amp;&amp; grep -nE "127\.0\.0\.1" internal/auth/oauth.go &amp;&amp; ! grep -nE "\"localhost\"" internal/auth/oauth.go &amp;&amp; grep -nE "drive\.file" internal/auth/oauth.go &amp;&amp; grep -nE "openid" internal/auth/oauth.go &amp;&amp; grep -nE "userinfo\.email" internal/auth/oauth.go &amp;&amp; grep -nE "code_verifier" internal/auth/oauth.go &amp;&amp; grep -nE "code_challenge_method" internal/auth/oauth.go &amp;&amp; grep -nE "ReuseTokenSource" internal/auth/oauth.go &amp;&amp; grep -nE "StoreToken" internal/auth/oauth.go &amp;&amp; grep -nE "GetUserEmail" internal/auth/oauth.go &amp;&amp; grep -nE "func NewManagerWithListener" internal/auth/oauth.go &amp;&amp; grep -nE "func \(m \*Manager\) AttachRoutes" internal/auth/oauth.go &amp;&amp; grep -nE "func \(m \*Manager\) AuthURL" internal/auth/oauth.go &amp;&amp; grep -nE "func \(m \*Manager\) HandlePastedRedirect" internal/auth/oauth.go &amp;&amp; grep -nE "func \(m \*Manager\) DoneChan" internal/auth/oauth.go &amp;&amp; grep -nE "func OAuthConfigForRefresh" internal/auth/oauth.go &amp;&amp; ! grep -vE "^\s*//" internal/auth/oauth.go | grep -E "slog\.(Info|Warn|Error|Debug).*RefreshToken|slog\.(Info|Warn|Error|Debug).*AccessToken"</automated>
  </verify>
  <acceptance_criteria>
    - `internal/auth/oauth.go` exists and is at least 160 non-blank lines (shared-listener API expands surface vs. previous 120-line target)
    - `grep -n "127\\.0\\.0\\.1" internal/auth/oauth.go` returns ≥1 match
    - `grep -nE '"localhost"' internal/auth/oauth.go` returns 0 matches (Pitfall #6 enforcement)
    - All three scopes appear: `drive.file`, `openid`, `userinfo.email`
    - `code_challenge_method` literal appears
    - `code_verifier` literal appears in the Exchange call
    - `ReuseTokenSource` literal appears (Pitfall #2 enforcement)
    - `StoreToken` and `GetUserEmail` are both called from oauth.go
    - The SIX shared-listener exports all exist with their final names: `NewManagerWithListener`, `AttachRoutes`, `AuthURL`, `HandlePastedRedirect`, `DoneChan`, `OAuthConfigForRefresh`
    - `grep -nE "slog\\.(Info|Warn|Error|Debug).*\\b(RefreshToken|AccessToken|code_verifier)\\b" internal/auth/oauth.go` returns 0 matches in non-comment lines
    - `go build ./internal/auth/...` exits 0
    - `go vet ./internal/auth/...` exits 0
    - The auth URL builder test and OAuthConfigForRefresh scope test (if added) pass via `go test ./internal/auth/...`
  </acceptance_criteria>
  <done>
    `auth.NewManager(cfg).RunOAuth(ctx)` is callable for standalone testing AND
    `auth.NewManagerWithListener(cfg, bc, ln) + AttachRoutes(mux) + DoneChan() + AuthURL()` is the
    shared-listener API Plan 07 will consume verbatim. After RunOAuth: refresh token is in wincred
    under `SquireBot:&lt;email&gt;`, email is cached in config.json, listener is still alive for Plan 06
    to attach /picker. No token bytes are logged anywhere. `OAuthConfigForRefresh` is exported as
    a package-level helper for refresh-only flows.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| browser ↔ loopback HTTP server | Untrusted state/code values arrive via redirect; CSRF window |
| loopback HTTP server ↔ Google token endpoint | Outbound HTTPS; PKCE proof-of-possession protects against intercepted code |
| Go process ↔ wincred (DPAPI) | Sensitive credential handed to OS keystore |
| oauth.go ↔ slog handler | Risk of logging refresh_token / access_token to disk |
| build constants (-ldflags) ↔ binary | Public client_id baked into binary; not a secret |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-03-01 | Spoofing | OAuth callback CSRF — attacker tricks user's browser into hitting /oauth/callback?code=X&state=anything | mitigate | Generate 32-byte random `state` per RFC 6819; callback handler rejects (HTTP 400) when state != expectedState; acceptance criteria greps for the comparison |
| T-03-02 | Tampering | PKCE verifier predictable / short, allowing intercepted code to be exchanged | mitigate | `crypto/rand` 32 bytes → 43-char base64url verifier per RFC 7636; pkce_test.go verifies length and entropy |
| T-03-03 | Information Disclosure | Refresh token logged via slog | mitigate | Acceptance criteria greps for `slog.*RefreshToken` and rejects on match; logging policy comment in oauth.go forbids it; defer-zero of tok.RefreshToken after StoreToken |
| T-03-04 | Information Disclosure | Refresh token written to config.json | mitigate | Plan 01 Config struct has no token field; oauth.go writes only `email` to config; Plan 01 acceptance criteria already enforces this |
| T-03-05 | Spoofing | Local malware listening on a colliding 127.0.0.1 port intercepts the redirect | mitigate | `net.Listen("tcp", "127.0.0.1:0")` lets kernel choose an ephemeral port; lifetime bounded to OAuth flow; `127.0.0.1` literal (firewall-friendly) per Pitfall #6 |
| T-03-06 | Tampering | Picker API key leaked from public repo enables abuse of dev's Cloud project quota | accept | Per RESEARCH.md §5.4 these keys are public for desktop clients; restriction to Google Picker API only bounds blast radius; Plan 02 acceptance criteria already accepted this |
| T-03-07 | Privilege Escalation | Scope creep adds drive or spreadsheets scope at refactor time | mitigate | acceptance grep enforces `drive.file` only and rejects `\"https://www.googleapis.com/auth/drive\"` (without .file suffix) and `\"https://www.googleapis.com/auth/spreadsheets\"` substrings |
| T-03-08 | Information Disclosure | wincred entry copied off the dev's machine and decrypted on attacker's | mitigate | Persistence = `PersistLocalMachine` is user-profile-bound DPAPI; copy to another user account → undecryptable. Documented expected behavior |
| T-03-09 | Denial of Service | OAuth flow hangs forever if user closes browser without consenting | mitigate | RunOAuth honors ctx cancellation; 60-second manual-paste timer surfaces alt UI; Plan 07 wizard UX provides "Continue setup…" on next launch |
| T-03-10 | Repudiation | No log of who authorized what at when | mitigate | slog.Info on each major transition (`oauth started`, `oauth callback received`, `token stored in wincred`) — email + port + timestamp; refresh-token bytes never logged |
</threat_model>

<verification>
- `go build ./internal/auth/...` exits 0
- `go vet ./internal/auth/...` exits 0
- `go test ./internal/auth/... -count=1` exits 0 (Windows tests will run; on Linux the build-tagged store_test.go is silently excluded)
- After running the binary on Windows and completing OAuth in browser:
  - Windows Credential Manager (`rundll32.exe keymgr.dll,KRShowKeyMgr` or `cmdkey /list`) shows a Generic credential under target `SquireBot:<email>`
  - `%LOCALAPPDATA%\SquireBot\config.json` contains `"google_email": "<email>"` AND does NOT contain any field named `refresh_token` or `access_token`
- `grep -rE "\"https://www\\.googleapis\\.com/auth/(drive\"|spreadsheets\")" internal/auth/` returns 0 matches (only drive.file is allowed)
- `grep -rnE "slog\\.(Info|Warn|Error|Debug).*\\b(RefreshToken|AccessToken|code_verifier|client_secret)\\b" --include="*.go" .` returns 0 non-comment matches
- The OAuth consent screen visible in browser shows the three scopes verbatim and does NOT show "This app isn't verified" (validates Plan 02's Production publish)
- All six shared-listener exports compile and are reachable from Plan 07's wizard package without modifying internal/auth/oauth.go
</verification>

<success_criteria>
- AUTH-01 satisfied: loopback PKCE on a random ephemeral port using `127.0.0.1` literal; manual-paste fallback timer in place
- AUTH-02 satisfied: scope set is exactly `drive.file + openid + userinfo.email`; no other scopes appear in the source
- AUTH-04 satisfied: refresh token stored only in wincred under `SquireBot:<email>`; never written to config.json or any other file
- AUTH-06 satisfied: `userinfo.email` is fetched and cached as `config.GoogleEmail`; subsequent plans use this as the canonical identity
- The auth package is importable, tested where feasible, and ready for Plans 05/06/07 to integrate
- Plan 07 can implement its wizard purely by importing `internal/auth` — NO edits to internal/auth/oauth.go required
</success_criteria>

<output>
After completion, create `.planning/phases/01-end-to-end-thin-slice/01-03-SUMMARY.md` documenting:
- The exact `oauth2.Config.Scopes` slice values
- The exact wincred target name format (literal: `SquireBot:<email>`)
- Whether the manual-paste fallback was implemented as a polling page or a separate /start_paste endpoint
- The behavior of RunOAuth when the user closes the browser tab before consenting (ctx cancellation path)
- Any deviations from RESEARCH.md §4 and why
- A note for Plan 06 explaining how to attach /picker route to the still-running listener
- A note for Plan 07 confirming the six shared-listener exports (NewManagerWithListener, AttachRoutes, AuthURL, HandlePastedRedirect, DoneChan, OAuthConfigForRefresh) are available as documented in &lt;interfaces&gt;
</output>
