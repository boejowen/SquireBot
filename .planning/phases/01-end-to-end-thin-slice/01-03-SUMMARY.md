---
phase: 01-end-to-end-thin-slice
plan: 03
subsystem: auth
tags: [go, oauth2, pkce, loopback, wincred, dpapi, ldflags, drive.file, openid, userinfo, csrf, rfc7636]

# Dependency graph
requires: [01, 02]
provides:
  - "internal/auth.NewPKCEPair — RFC 7636 verifier+challenge generator"
  - "internal/auth.BuildConstants{OAuthClientID, PickerAPIKey, GCPProjectNumber} + Validate / ErrMissingConstants"
  - "internal/auth.StoreToken / ReadToken / DeleteToken — wincred-backed refresh-token store under SquireBot:<email>"
  - "internal/auth.GetUserEmail — Userinfo.Get against Google's oauth2/v2 endpoint"
  - "internal/auth.OpenBrowser — rundll32 url.dll,FileProtocolHandler URL launcher (Win) + darwin/linux fallbacks"
  - "internal/auth.NewManager / RunOAuth — standalone loopback flow that owns its own listener"
  - "internal/auth.NewManagerWithListener / AttachRoutes / AuthURL / HandlePastedRedirect / DoneChan — shared-listener API for Plan 07"
  - "internal/auth.OAuthConfigForRefresh — refresh-only oauth2.Config helper (Plan 07's runWatcher)"
  - "cmd/squirebot/build_constants.go — package-main vars for -ldflags injection (also home of Version, moved from main.go)"
  - ".github/workflows/release.yml — materialise oauth-config.json from OAUTH_CONFIG_JSON secret + ConvertFrom-Json + -ldflags injection"
  - "README.md — canonical jq build line + PowerShell fallback"
affects: [01-05, 01-06, 01-07]

# Tech tracking
tech-stack:
  added:
    - "google.golang.org/api/oauth2/v2 (svc.Userinfo.Get) — already in go.mod from 01-01, now first direct caller"
    - "google.golang.org/api/option (option.WithTokenSource)"
    - "golang.org/x/oauth2/google (google.Endpoint) — first direct caller"
    - "github.com/danieljoos/wincred — first direct caller (was indirect via 01-01 pin)"
    - "crypto/subtle (constant-time state compare in handleCallback + HandlePastedRedirect)"
  patterns:
    - "Shared-listener API: NewManagerWithListener accepts caller-owned net.Listener + AttachRoutes registers handlers on caller-owned mux. Plan 07 composes OAuth + Picker + wizard pages on a single port → user only sees one browser tab."
    - "Defer-zero of tok.RefreshToken/AccessToken after StoreToken returns — defence in depth against later log.Printf or panic stack-trace leaks (T-03-03)."
    - "sync.Once-protected DoneChan signal: doneOnce.Do(func() { m.done <- res; close(m.done) }) so race between handleCallback and ctx-cancel cannot deadlock or panic."
    - "scopeSet is a single package-level []string; consent flow and refresh flow both share-by-copy via append([]string(nil), scopeSet...)."
    - "Build-time -ldflags pipeline: oauth-config.json (gitignored) → release.yml ConvertFrom-Json → -X main.OAuthClientID=... → cmd/squirebot/build_constants.go vars → auth.BuildConstants struct passed at runtime."
    - "constant-time state compare (subtle.ConstantTimeCompare) on every CSRF check — same primitive used in both /oauth/callback and HandlePastedRedirect."
    - "safePrefix() helper for slog correlation: logs first 8 chars + '...' of state value, never the full value or any token byte."

key-files:
  created:
    - "internal/auth/pkce.go"
    - "internal/auth/pkce_test.go"
    - "internal/auth/oauthconfig.go"
    - "internal/auth/store.go"
    - "internal/auth/store_test.go"
    - "internal/auth/userinfo.go"
    - "internal/auth/browser.go"
    - "internal/auth/oauth.go"
    - "internal/auth/oauth_test.go"
    - "cmd/squirebot/build_constants.go"
  modified:
    - "cmd/squirebot/main.go (removed `var Version = ...`; canonical home moved to build_constants.go)"
    - ".github/workflows/release.yml (added OAUTH_CONFIG_JSON secret materialisation + -ldflags injection)"
    - "README.md (added jq + PowerShell build invocations with OAuth -ldflags)"

key-decisions:
  - "redirect_uri is `http://127.0.0.1:<port>/oauth/callback` (literal IP, NEVER `localhost`). Google's desktop loopback policy explicitly rejects localhost as of recent updates (RESEARCH.md §4.1)."
  - "Scope set is exactly drive.file + openid + userinfo.email — sensitive-exempt trio per RESEARCH.md §4.2. T-03-07 regression test in oauth_test.go enforces NO drive (without .file), spreadsheets, or drive.readonly."
  - "state value is 32 bytes of crypto/rand → 43-char base64url-NoPadding (same shape as PKCE verifier). Constant-time compare via subtle.ConstantTimeCompare in both HTTP-callback and pasted-URL paths (T-03-01)."
  - "Refresh token never lives in any Go variable after StoreToken returns: defer-zero of tok.RefreshToken + tok.AccessToken; OAuthResult.RefreshToken explicitly set to '' before signalling DoneChan."
  - "Manual-paste fallback (AUTH-01) is a separate `/start_paste` POST endpoint, NOT a polling page. The 60-second timer arms a slog.Warn but does not auto-cancel — Plan 07's wizard surfaces the textarea UI; the timer is purely advisory."
  - "OpenBrowser uses `rundll32 url.dll,FileProtocolHandler <url>` on Windows (NOT `cmd /c start`). The `start` form has shell-injection edge cases on URLs containing `&` (every OAuth URL does). darwin/linux branches exist solely for `go test` ergonomics on dev machines — production binary is Windows-only."
  - "Version = '0.1.0-dev' moved from main.go to cmd/squirebot/build_constants.go. The plan's `<interfaces>` block listed Version in build_constants.go but said main.go is not modified — these were mutually exclusive (duplicate var declaration). Resolved by treating build_constants.go as the canonical home for ALL -ldflags-injected vars (architecturally cleaner anyway)."
  - "OAuthResult.Server is nil in NewManagerWithListener mode — the caller (Plan 07 wizard) owns the *http.Server and is responsible for Shutdown(). RunOAuth refuses to run in shared-listener mode and returns 'use DoneChan' error."
  - "BuildConstants.Validate() returns ErrMissingConstants if any of OAuthClientID / PickerAPIKey / GCPProjectNumber is empty — Plan 07 calls this at startup and refuses to run OAuth with a blank client_id."
  - "store_test.go uses `//go:build windows` build tag — wincred is Windows-only. On Linux/macOS the file is silently excluded; `go test ./...` is green on all three platforms."

requirements-completed: [AUTH-01, AUTH-02, AUTH-04, AUTH-06]

# Metrics
duration: ~20min
completed: 2026-05-01
---

# Phase 1 Plan 03: OAuth Loopback PKCE Summary

**OAuth 2.0 desktop loopback PKCE flow end-to-end — RFC 7636 verifier/challenge generator, sensitive-exempt scope trio (drive.file + openid + userinfo.email), wincred-backed refresh-token store under target name `SquireBot:<email>`, Userinfo email lookup, rundll32-based browser launcher, and the six shared-listener API exports Plan 07's wizard will consume verbatim.**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-05-01T15:14:50Z
- **Completed:** 2026-05-01T15:34:30Z
- **Tasks:** 3 / 3
- **Files created:** 10 (8 in `internal/auth/`, 1 in `cmd/squirebot/`, 1 SUMMARY)
- **Files modified:** 3 (`cmd/squirebot/main.go`, `.github/workflows/release.yml`, `README.md`)
- **Tests landed:** 16 (4 PKCE + 5 store + 7 oauth — all run offline)

## Accomplishments

- **PKCE generator (RFC 7636 / T-03-02):** `internal/auth.NewPKCEPair()` returns a 32-byte `crypto/rand` verifier base64url-NoPadding-encoded to exactly 43 characters (the RFC's minimum entropy floor) plus the S256 challenge `base64url(SHA256(verifier))`. Four tests cover length, char class `[A-Za-z0-9_-]+`, the SHA-256 transform, and entropy (1000 calls = 1000 distinct verifiers).
- **Build-constant pipeline:** `cmd/squirebot/build_constants.go` declares the three -ldflags injection points (`OAuthClientID`, `PickerAPIKey`, `GCPProjectNumber`) plus `Version`. `internal/auth.BuildConstants` is the typed view callers use; `Validate()` returns `ErrMissingConstants` if any value is empty. `.github/workflows/release.yml` materialises `oauth-config.json` from a repo Secret, parses it via PowerShell `ConvertFrom-Json`, and threads the values into `-X main.X=Y` ldflags. `README.md` documents both a `jq` one-liner (canonical, for CI / developers with jq) and a PowerShell `ConvertFrom-Json` fallback (for present-day dev machines without jq).
- **wincred token store (AUTH-04 / T-03-08):** `internal/auth.StoreToken / ReadToken / DeleteToken` wrap `github.com/danieljoos/wincred` with a JSON-marshalled `StoredToken{RefreshToken, Email, ClientID}` blob, `Persist = wincred.PersistLocalMachine`, and target name pattern `SquireBot:<email>`. Five tests on Windows (build-tagged `//go:build windows`) cover round-trip, missing-credential error, delete-then-read, JSON-blob shape (asserts first byte is `{` AND `Persist == PersistLocalMachine`), and a hard pin on the `CredPrefix == "SquireBot:"` literal.
- **Userinfo lookup (AUTH-06):** `internal/auth.GetUserEmail(ctx, ts)` calls `oauth2v2.NewService(ctx, option.WithTokenSource(ts)).Userinfo.Get().Context(ctx).Do()` and returns `info.Email`. Single round-trip; result cached in `config.GoogleEmail` after `StoreToken` succeeds.
- **Browser launcher:** `internal/auth.OpenBrowser` uses `rundll32 url.dll,FileProtocolHandler <url>` on Windows (sidesteps `cmd /c start`'s `&`-related shell-injection edge cases on OAuth URLs). darwin/linux fallbacks exist for `go test` ergonomics.
- **OAuth Manager (the load-bearing one):** `internal/auth.Manager` owns one in-flight flow. `NewManager` allocates a fresh `net.Listener` on `127.0.0.1:0` and wraps it in an `*http.Server`; `NewManagerWithListener` accepts a caller-owned listener and never calls `ListenAndServe`. `AttachRoutes` registers `/oauth/callback` + `/start_paste` on a caller-owned mux. `AuthURL()` returns the consent URL with all 9 required parameters (client_id, redirect_uri, response_type, scope, state, code_challenge, code_challenge_method, access_type, prompt). `handleCallback` validates state via `subtle.ConstantTimeCompare`, exchanges the code with the PKCE verifier, looks up the email, stores the refresh token in wincred, persists `config.GoogleEmail`, and signals `DoneChan` with `OAuthResult{Email, TokenSource, Listener, Server, Port}` ready for Plan 06 to attach `/picker`. `HandlePastedRedirect` is the AUTH-01 manual-paste fallback — same code path, just driven by a pasted URL string instead of an HTTP request. `RunOAuth` is the convenience wrapper for standalone (`squirebot.exe oauth`) testing; it refuses to run in shared-listener mode.
- **Smoke build:** `dist/squirebot.exe` rebuilt with the real `oauth-config.json` values via `-ldflags`. Binary is 2,554,368 bytes (2.55 MB) — same size class as Plan 01-01's smoke build (delta ~512 bytes from the three short ASCII strings). PE32+ Windows GUI executable. `python -c "open('dist/squirebot.exe', 'rb').read()"` confirms the literal client_id substring `262087828393-8obvbca` and the picker key prefix `AIzaSyA5L1f2Douelcol` ARE in the binary; the scope literals (`drive.file`, `userinfo.email`) are NOT yet because main.go does not import internal/auth — Plan 07 wires that, after which the linker will retain the scope strings.

## Task Commits

Each task is a single Conventional Commit on `master`:

1. **Task 1: PKCE generator + build-constant bridge** — `7de3704` (feat)
2. **Task 2: wincred token store + userinfo email lookup + browser launcher** — `429ec6c` (feat)
3. **Task 3: OAuth Manager — loopback HTTP server, callback, shared-listener API** — `e570943` (feat)

(No final docs commit — `commit_docs: false` in `.planning/config.json`. SUMMARY.md is on disk; `.planning/` is gitignored.)

## TDD Gate Compliance

Plan frontmatter is `type: execute` (not `type: tdd`); per-task discipline:

- **Task 1 (`tdd="true"`):** RED — `pkce_test.go` written first against undefined `NewPKCEPair`. GREEN — `pkce.go` lands the function from RESEARCH.md §4.3 verbatim. Single commit covers both halves of the cycle (the build constants + workflow yaml in the same task are infrastructure, not RED/GREEN candidates).
- **Task 2 (`tdd="true"`):** RED — `store_test.go` written before `store.go`; the `//go:build windows` tag means tests would not even compile pre-implementation on Windows. GREEN — `store.go` lands; tests pass first run. `userinfo.go` and `browser.go` are thin wrappers around external libraries (no behaviour to assert beyond "it called the right thing"); tests for them would be tautological.
- **Task 3 (`tdd="false"`):** Plan-spec choice — the file is too large and too tightly coupled to oauth2 internals for a TDD-first approach. Tests landed alongside the implementation in the same commit, covering URL parameter coverage, OAuthConfigForRefresh scope match, shared-listener constructor behaviour, state-mismatch CSRF rejection (HTTP and pasted), BuildConstants validation, and a compile-time DoneChan receive-only guarantee.

## Exact `oauth2.Config.Scopes`

```go
[]string{
    "https://www.googleapis.com/auth/drive.file",
    "openid",
    "https://www.googleapis.com/auth/userinfo.email",
}
```

Same slice content used in both `Manager.cfg` (consent-time) and `OAuthConfigForRefresh()` (refresh-only). They're spelled out via `append([]string(nil), scopeSet...)` on each construction so a caller mutating one slice cannot affect the other.

## Exact wincred Target Name Format

Literal: `SquireBot:<google-email>`

Concretely: `SquireBot:jbowen@mncivic.com` would be the dev's credential after the first successful OAuth on this machine. `cmdkey /list` lists it under "Generic Credentials". The `:` is the standard wincred grouping convention; all SquireBot-issued credentials sort together.

## Manual-Paste Fallback (AUTH-01)

Implemented as a **separate `/start_paste` POST endpoint**, not a polling page.

- The wizard's start.html (Plan 07) renders a `<textarea name="redirect_url">` plus a submit button posting to `/start_paste`.
- The handler calls `Manager.HandlePastedRedirect(ctx, raw)` which does its own `subtle.ConstantTimeCompare` on the state value and runs the full exchange-and-store body.
- A `time.AfterFunc(60*time.Second, ...)` arms a `slog.Warn("oauth manual-paste fallback armed", ...)` line — purely advisory; the timer does NOT cancel the flow. Plan 07's wizard owns the user-facing "We didn't see a redirect — paste the URL here" surface.
- `HandlePastedRedirect` is also exported so callers can inject pasted URLs programmatically (test path; future tray-icon "Paste OAuth URL" menu item).

## RunOAuth Behaviour When User Closes Browser Mid-Flow

Three distinct paths:

1. **User closes the browser tab BEFORE granting consent.** No callback fires. The 60s timer warns; `RunOAuth` continues to block on `m.done`. The wizard's `ctx` cancels the call when the user clicks "Cancel" in the wizard or quits the app — `RunOAuth` returns `OAuthResult{Err: ctx.Err()}`. `m.server.Close()` is invoked so the goroutine exits. No partial state is persisted.
2. **User closes the browser AFTER consenting but BEFORE the redirect lands.** Google still issues the redirect — the loopback `http.Server` receives the request even with no live browser tab. `handleCallback` runs to completion, refresh token lands in wincred, email persisted to config. `OAuthResult` flows down `DoneChan`. The user sees a closed tab but the wizard advances anyway.
3. **User pastes the redirect URL manually.** `/start_paste` POST → `HandlePastedRedirect` → same exchange-and-store body. Identical OAuthResult. State mismatch (paste from a stale flow) returns 400 with `"state mismatch"` and does NOT signal DoneChan — the user can retry or restart.

## Notes for Plan 06 (Drive Picker)

After `RunOAuth` returns successfully, `OAuthResult.Listener` and `OAuthResult.Server` are STILL ALIVE. To attach `/picker`:

```go
// In Plan 06's wiring code (cmd/squirebot/main.go after Plan 07 lands):
res := manager.RunOAuth(ctx)
if res.Err != nil { /* handle */ }

// The mux is the one Manager.AttachRoutes wrote to. In NewManager mode
// the mux is internal — Plan 06 needs to either:
//   (a) Use NewManagerWithListener instead and own the mux explicitly
//       (this is what Plan 07 will do), OR
//   (b) Pull the mux out of res.Server.Handler and add /picker to it.
// Plan 07's wizard composition is path (a), which is the recommended
// integration. Plan 06's standalone smoke can use (b).

picker := pickergo.NewServer(res.TokenSource, bc.PickerAPIKey, bc.GCPProjectNumber)
picker.AttachRoutes(mux)  // mux from path (a)
```

The wizard composition pattern Plan 07 will implement:

```go
listener, _ := net.Listen("tcp", "127.0.0.1:0")
mux := http.NewServeMux()

mgr := auth.NewManagerWithListener(cfg, bc, listener)
mgr.AttachRoutes(mux)                         // /oauth/callback, /start_paste

picker := pickergo.NewServer(...)             // Plan 06
picker.AttachRoutes(mux)                      // /picker, /picker/result

wizard := wizardgo.NewServer(mgr, picker, ...) // Plan 07
wizard.AttachRoutes(mux)                      // /, /start, /eq-folder, /done

go func() { _ = http.Serve(listener, mux) }()
auth.OpenBrowser(fmt.Sprintf("http://127.0.0.1:%d/", listener.Addr().(*net.TCPAddr).Port))

select {
case res := <-mgr.DoneChan():
    // OAuth done — picker route is already wired and the user's redirect
    // from /oauth/callback will flow into /picker on the same listener.
}
```

## Notes for Plan 07 (Wizard)

The six shared-listener exports are all live and verified compile-clean:

| Export | Signature | Purpose |
|---|---|---|
| `NewManagerWithListener(cfg *config.Config, bc BuildConstants, listener net.Listener) *Manager` | constructor | Builds Manager that shares caller's listener |
| `(m *Manager) AttachRoutes(mux *http.ServeMux)` | route registration | Registers `/oauth/callback` + `/start_paste` |
| `(m *Manager) AuthURL() string` | URL builder | Wizard's start.html `<a href="{{ .AuthURL }}">` template |
| `(m *Manager) HandlePastedRedirect(ctx context.Context, raw string) error` | manual-paste fallback | Wizard's textarea POST handler can call this directly OR rely on `/start_paste` |
| `(m *Manager) DoneChan() <-chan OAuthResult` | completion signal | Wizard awaits OAuth completion before navigating to /picker |
| `OAuthConfigForRefresh(cfg Config) *oauth2.Config` | refresh-only helper | runWatcher rebuilds TokenSource from wincred-stored refresh token |

Plan 07 needs no edits to `internal/auth/oauth.go`. The `BuildConstants` struct flows in as a value parameter; the caller (Plan 07 code in `cmd/squirebot/main.go`) constructs it from the package-main vars: `bc := auth.BuildConstants{OAuthClientID: OAuthClientID, PickerAPIKey: PickerAPIKey, GCPProjectNumber: GCPProjectNumber}; bc.Validate()`.

## Deviations from Plan / RESEARCH.md

### Auto-fixed Issues

**1. [Rule 1 — Plan-spec contradiction] `Version` declared in two places**
- **Found during:** Task 1 (first `go build ./...` after creating `cmd/squirebot/build_constants.go`).
- **Issue:** The plan's `<interfaces>` block listed `Version = "0.1.0-dev"` in `cmd/squirebot/build_constants.go`. The plan's `files_modified_notes` said "cmd/squirebot/main.go is NOT modified by this plan." But Plan 01-01 already declared `var Version = "0.1.0-dev"` in main.go. Two package-level declarations of the same name in the same `package main` produce: `cmd\squirebot\main.go:18:5: Version redeclared in this block`. These constraints are mutually exclusive.
- **Fix:** Removed the line from main.go and replaced it with a one-line pointer comment ("Version moved to build_constants.go in Plan 01-03"). build_constants.go is now the single canonical home for every -ldflags-injected package-main variable — architecturally cleaner anyway.
- **Verification:** `go build ./...` exits 0; `dist/squirebot.exe` rebuilt with `-X main.Version=01-03-smoke` and the literal `01-03-smoke` is present in the binary (verified via `python -c "open(...).read()"`).
- **Committed in:** `7de3704` (Task 1 commit).
- **Recommendation for the planner-loop:** when emitting an `<interfaces>` block that declares package-main vars, also list the file in `files_modified` if any earlier plan already declared the same var. Cross-plan symbol-name collisions are detectable at planning time.

**2. [Rule 3 — Tooling] `jq` not on PATH on this dev machine — substituted PowerShell `ConvertFrom-Json`**
- **Found during:** Task 1 verify gate; precedent set in 01-02 and 01-04.
- **Issue:** The plan's `<action>` for Task 1 specifies a `jq -r '.field' oauth-config.json` form for the `release.yml` build step. `jq` is not on this dev machine's PATH (Windows; would need `scoop install jq` or `choco install jq`).
- **Fix:** Wrote `release.yml` using PowerShell `ConvertFrom-Json` (cross-platform, available everywhere `pwsh` is — and GitHub Actions `windows-latest` ships pwsh). Documented BOTH the canonical `jq` one-liner AND the PowerShell fallback in README.md so contributors can choose. `oauth-config.json` is read at build time identically — only the JSON-parser binary differs.
- **Verification:** `release.yml` syntax-checked locally; the workflow runs against a `secrets.OAUTH_CONFIG_JSON` repo secret that the dev will populate before the first tag push.
- **Recommendation for the planner-loop:** treat `jq` as optional in Phase 1. The PowerShell fallback covers all CI runners and most Windows dev machines.

**3. [Rule 1 — Information disclosure prevention] Defer-zero of `tok.RefreshToken` after `StoreToken`**
- **Found during:** designing Task 3's `exchangeAndStore` (proactive Rule 2 mitigation, not a found bug).
- **Action:** Added `defer func() { tok.RefreshToken = ""; tok.AccessToken = "" }()` immediately after `cfg.Exchange` succeeds, plus an explicit `RefreshToken: ""` in the `OAuthResult` struct sent to `DoneChan`. The plan recommends this in the `<action>` block but it is not a verify-gate item — added anyway because T-03-03 is a load-bearing threat.
- **Verification:** `OAuthResult.RefreshToken` is intentionally documented as "in-memory only; for handing to TokenSource creators" in the struct doc, but in practice we never put a real refresh token there — TokenSource is the way callers get future access tokens, and that lives behind `oauth2.ReuseTokenSource`.
- **No commit-level impact:** part of the Task 3 commit.

### Other Notes

**T-03-07 regression test contains the forbidden scope literals as test data.** `oauth_test.go` lines 96–100 list `https://www.googleapis.com/auth/drive `, `…/spreadsheets`, and `…/drive.readonly` inside a `forbidden := []string{...}` slice — the test asserts these substrings do NOT appear in the AuthURL `scope` query parameter. The plan's verification grep `grep -rE "\"https://www\\.googleapis\\.com/auth/(drive\"|spreadsheets\")" internal/auth/` matches the test's `spreadsheets"` literal as a false positive. Production code (`oauth.go`, `userinfo.go`, etc.) is clean — verified by grepping the production files only.

## Authentication Gates

None encountered. All work was offline; no Google round-trips were necessary because the manager unit-tests use `httptest.NewServer` and stop short of `cfg.Exchange`. The manual browser-smoke is documented as a deferred user action below.

## Logger Hygiene Verification

- `grep -rnE 'slog\.(Info|Warn|Error|Debug).*\b(RefreshToken|AccessToken|code_verifier|client_secret)\b' --include="*.go" .` returns zero non-comment matches.
- The single `safePrefix(state)` helper truncates state to `state[:8] + "..."` for log correlation; never the full value.
- `slog.Info("oauth callback received", "email", email)` is the only PII the auth flow emits — and `email` is not a secret.

## Manual User Actions Required Post-Plan

1. **Browser smoke after Plan 07 lands.** Once Plan 07 wires `auth.NewManagerWithListener` into `cmd/squirebot/main.go`, the dev needs to:
   - Build with the real -ldflags pipeline (jq one-liner or PowerShell fallback in README.md).
   - Launch `dist/squirebot.exe` from a fresh `%LOCALAPPDATA%\SquireBot` (delete that dir first to simulate a clean install).
   - Browser opens to Google's consent screen at `accounts.google.com/o/oauth2/v2/auth?...`.
   - Three scopes appear verbatim in the consent UI: "See, edit, create, and delete only the specific Google Drive files you use with this app", "Associate you with your personal info on Google", "See your primary Google Account email address". (These are the human-readable forms of `drive.file` + `openid` + `userinfo.email`.)
   - **CRITICAL:** the consent screen MUST NOT show "This app isn't verified" — if it does, Plan 02's Production publish has regressed. RESEARCH.md §4.6 / Plan 02 SUMMARY's Q3 resolution.
   - After clicking Allow, the loopback redirect lands within 5 seconds. Browser shows whatever `/picker` route Plan 06 wires (or HTTP 404 if Plan 06 hasn't landed yet — that's the normal Plan 03 → Plan 06 gap).
   - `cmdkey /list` shows a Generic credential under target `SquireBot:<email>`.
   - `%LOCALAPPDATA%\SquireBot\config.json` contains `"google_email": "<email>"` and does NOT contain `refresh_token`, `access_token`, or anything starting with `1//0`.

2. **Populate `OAUTH_CONFIG_JSON` repo secret in GitHub.** Before the first `v*` tag push, the dev needs to copy the contents of `.planning/phases/01-end-to-end-thin-slice/oauth-config.json` (5 lines of JSON) into a new repo secret named `OAUTH_CONFIG_JSON`. The release workflow fails fast with a clear "see docs/oauth-setup.md" message if the secret is empty.

3. **Optional: `scoop install jq` (or `choco install jq`).** Not blocking — the PowerShell fallback in README.md works everywhere — but the canonical jq one-liner is shorter and idiomatic on Linux/macOS dev machines.

## Blockers for Plan 01-05 (Sheets Writer, Wave 3)

**None.** Plan 01-05 (`depends_on: [01, 03]`) needs:

- An `oauth2.TokenSource` to authenticate `spreadsheets.batchUpdate` calls — `OAuthConfigForRefresh` provides the construction path, and Plan 01-07's runWatcher wiring will hand a live `TokenSource` to the sheets package.
- `config.SpreadsheetID` for the workbook being written to — Plan 06's Picker writes this; Plan 05 just reads it.
- The `_meta` / `_char_owner` schema constants — Plan 05 owns those itself.

Nothing in Plan 03's outputs blocks Plan 05 from starting. The Sheets writer can be written entirely against the `oauth2.TokenSource` interface and a `SpreadsheetID` string; integration with this plan happens at the `cmd/squirebot/main.go` wiring layer (Plan 07).

## Self-Check

Created files (verified present on disk):

- FOUND: `internal/auth/pkce.go`
- FOUND: `internal/auth/pkce_test.go`
- FOUND: `internal/auth/oauthconfig.go`
- FOUND: `internal/auth/store.go`
- FOUND: `internal/auth/store_test.go`
- FOUND: `internal/auth/userinfo.go`
- FOUND: `internal/auth/browser.go`
- FOUND: `internal/auth/oauth.go`
- FOUND: `internal/auth/oauth_test.go`
- FOUND: `cmd/squirebot/build_constants.go`

Modified files (verified contain the change):

- FOUND: `cmd/squirebot/main.go` (no longer declares `var Version`; pointer comment in its place)
- FOUND: `.github/workflows/release.yml` (contains `OAUTH_CONFIG_JSON` secret materialise step + `-X main.OAuthClientID=` etc.)
- FOUND: `README.md` (contains both `jq` and `ConvertFrom-Json` invocations, plus warning about `ErrMissingConstants`)

Commits in `git log`:

- FOUND: `7de3704` (Task 1 - feat: add PKCE generator and build-constant pipeline)
- FOUND: `429ec6c` (Task 2 - feat: add wincred token store, userinfo lookup, browser launcher)
- FOUND: `e570943` (Task 3 - feat: add OAuth Manager with shared-listener API and PKCE flow)

Verification gates re-run end-to-end:

- `go build ./...` exit 0
- `go vet ./...` exit 0
- `go test ./internal/auth/... -count=1 -timeout 60s` exit 0 — 16 tests pass (4 PKCE + 5 store + 7 oauth)
- `go test ./... -count=1 -timeout 120s` exit 0 across all six internal packages
- All required greps in `<verify>` pass: 127.0.0.1 (3 occurrences in oauth.go), no `"localhost"` in oauth.go, drive.file + openid + userinfo.email all present, code_verifier + code_challenge_method literals present, ReuseTokenSource called, StoreToken called, GetUserEmail called, six shared-listener exports all defined.
- All required greps in `<verification>` pass: no `slog.*RefreshToken|AccessToken|code_verifier|client_secret` matches in non-comment lines; production auth files have no forbidden scopes (the only `spreadsheets"` substring is in oauth_test.go's defensive forbidden-list — false positive on the plan's verify regex; documented above).
- Smoke build `dist/squirebot.exe` 2,554,368 bytes (PE32+ Windows GUI). Real OAuth client_id substring `262087828393-8obvbca` and Picker key prefix `AIzaSyA5L1f2Douelcol` confirmed present in binary via direct byte scan. Scope literals are not in this binary because main.go does not import internal/auth yet — Plan 07 wires that.

## Self-Check: PASSED

## Threat Flags

| Flag | File | Description |
|------|------|-------------|
| (none) | — | No new security-relevant surface beyond what the plan's threat register T-03-01..T-03-10 already enumerated. The implementation actively defends against T-03-01 (constant-time state compare), T-03-02 (RFC 7636 verifier with 256 bits of entropy + tested char class), T-03-03 (defer-zero + slog hygiene + grep-enforced absence of token bytes), T-03-04 (Config struct still has no refresh-token field), T-03-05 (127.0.0.1:0 ephemeral port; `127.0.0.1` literal), T-03-07 (regression test in oauth_test.go enforces no broader scopes), T-03-08 (PersistLocalMachine documented), T-03-09 (ctx-cancel path returns ctx.Err()), and T-03-10 (slog.Info on each major transition with email + port + state-prefix). T-03-06 (public Picker key) is a Plan 02 acceptance — not affected by this plan's outputs. |

---
*Phase: 01-end-to-end-thin-slice*
*Completed: 2026-05-01*
