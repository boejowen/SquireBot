# Phase 13: Watcher Re-Target + Onboarding - Research

**Researched:** 2026-05-29
**Domain:** Go Windows desktop app re-target (HTTP client swap + large package deletion + onboarding UX) against a live bearer-token HTTP API
**Confidence:** HIGH (codebase fully mapped; target contract is shipped + test-proven; all claims verified against source unless tagged)

## Summary

Phase 13 is a **deletion-and-re-target**, not a greenfield build. The watcher's upload *pipeline* (fsnotify 500 ms debounce → re-stat → re-read → CP1252-decode → parse) **survives unchanged**; only the **sink** changes — from `sheet.Client.WriteInventory/WriteSpellbook` (Google Sheets `batchUpdate`) to a new `internal/backend` HTTP client that POSTs to the already-live `POST /api/v1/ingest` (BACKEND-03/04, shipped Plan 11-05, live at `https://api.squirebot.quest`). The ingest contract already exists and is test-proven: a JSON `Envelope {character, kind, content, watcher_version}`, `Authorization: Bearer <guild-code>`, server-side parsing of raw `content`, atomic full-snapshot replace, 401-writes-nothing. **The watcher becomes dramatically thinner**: it stops parsing-then-writing-structured-cells and instead POSTs the raw file text; the parser stays only as the disk-read CP1252 decode source (the *server* re-parses).

The deletion is the load-bearing risk. Five packages totaling ~8.1k LOC are Google-coupled (`internal/auth` 1471, `internal/sheet` 4160, `internal/scaffold` 820, `internal/picker` 740, `internal/wizard` 916), plus the two `internal/app` orchestrators (`runapp.go` 786 + `reauth.go` 290) that wire them. **Not all of `wizard` dies** — its EQ-folder-selection + config-save logic (`handleEQFolderGET/Pick/Confirm` + the `eqfind` integration + the sqweek native folder dialog in `folderpicker_dialog.go`) is reusable and MUST be preserved; only the OAuth/picker phases die. The import-site map is bounded and clean: the five deleted packages are imported only by `cmd/squirebot/main.go`, `internal/app/runapp.go`, `internal/app/reauth.go`, and the deleted packages' own tests. [VERIFIED: grep of `internal/(auth|sheet|scaffold|picker|wizard)` imports]

**Primary recommendation:** Build `internal/backend` (a ~150-LOC stdlib `net/http` POST client mirroring the backend's own `politefetch` ergonomics), store the guild code in DPAPI via a thin survivor of `internal/auth/store.go`, and pick **onboarding Option A (tray menu → native Windows input dialog)** for the least-code, no-loopback-server path — but surface the onboarding UX and the version-rejection scope as the two genuine forks for a user decision round before planning. The version-gate "fork" is mostly resolved by what already exists: the `Envelope.watcher_version` field and the `inventory_item.watcher_version` DB column ship today; P13 only needs the watcher to *populate* it and the backend to *optionally* reject below a floor — a ~30-LOC backend addition.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| File-change detection (fsnotify, debounce, re-read) | Watcher (client) | — | OS-local filesystem events; unchanged from v1 (`internal/watch` survivor) |
| CP1252→UTF-8 decode | Watcher (client) | — | The raw disk file is CP1252; decode happens ONCE on the read side (`parse.CP1252Reader`); the server receives UTF-8 (A1) |
| Inventory/spellbook *parsing* | **Backend (server)** | — | D-03: server parses `content`; the watcher no longer parses-to-cells. The watcher's `parse.Parse`/`ParseSpellbook` are now only the server's job; watcher keeps `parse.CP1252Reader` only |
| Upload transport | Watcher (client) → Backend API | — | New `internal/backend` HTTP POST replaces `sheet.Client.batchUpdate` |
| Identity / character→owner binding | **Backend (server)** | — | Derived server-side from the bearer token's owner (first-sighting bind, 11-03); `UpsertCharOwner` deleted — the watcher no longer asserts identity |
| Atomic full-snapshot replace | **Backend (server)** | — | One server tx (delete-all-then-insert); the watcher's clear+write Sheets contract is gone |
| Credential storage (guild code) | Watcher (client) — DPAPI | — | Windows Credential Manager via wincred (survivor of `auth/store.go`); read on every POST |
| Onboarding (paste guild code) | Watcher (client) — tray/native dialog | Backend (validation probe) | Local UX; validates against the backend (needs a lightweight authed probe — see Design Forks) |
| Auto-update | Watcher (client) → GitHub Releases | — | `internal/update` survivor; unchanged transport (already direct net/http to GitHub CDN, never Google) |
| Config migration (drop dead fields) | Watcher (client) | — | First-launch cleanup of OAuth/sheet config fields + stale wincred entry |

## Standard Stack

### Core (all ALREADY in the watcher's go.mod — no new deps for the happy path)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `net/http` (stdlib) | go1.25 | The new `internal/backend` POST client | Mirrors the backend's own hand-rolled net/http verdict (D-02); zero new deps; the watcher already uses net/http for the loopback servers being deleted and for `internal/update` |
| `github.com/danieljoos/wincred` | v1.2.0 | DPAPI-backed guild-code storage | ALREADY the watcher's refresh-token store (`internal/auth/store.go`); survives the deletion as a thin guild-code helper [VERIFIED: go.mod] |
| `github.com/fsnotify/fsnotify` | v1.7.0 | File-change pipeline (survivor) | Unchanged — the watch pipeline is the SINK swap's invariant [VERIFIED: go.mod] |
| `golang.org/x/text` | v0.37.0 | CP1252 decode (`charmap.Windows1252`) | Used by `parse.CP1252Reader`; stays on the watcher read side [VERIFIED: go.mod] |
| `github.com/minio/selfupdate` | v0.6.0 | Auto-update swap (survivor) | `internal/update/swap.go`; unchanged [VERIFIED: go.mod] |
| `fyne.io/systray` | v1.10.0 | Tray UI (survivor; menu re-labeled) | The onboarding-prompt trigger + status surface [VERIFIED: go.mod] |
| `github.com/sqweek/dialog` | pinned 2026-01-23 | Native folder dialog (survivor) — and the basis for Option A's input dialog | ALREADY used by `wizard/folderpicker_dialog.go`; cross-platform; pinned [VERIFIED: go.mod] |

### Supporting (Windows input dialog for Onboarding Option A)
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `golang.org/x/sys/windows` | v0.45.0 (present) | Win32 dialog (`MessageBox`/`DialogBox`) or input prompt | If Option A (native dialog) is chosen and a text-input box is needed beyond what `sqweek/dialog` offers (sqweek has file/dir/message/yesno but **no text-entry box**). [VERIFIED: go.mod has x/sys v0.45.0; `console_windows.go` already uses `windows.NewLazySystemDLL`] |
| `cmd/squirebot/icon.go` PowerShell shim or a tiny Win32 `InputBox` | — | A one-field "paste your guild code" prompt | A pure-Win32 input box requires a custom dialog template (verbose). A **simpler** route: VBScript `InputBox` via `wscript` is fragile; the **cleanest least-code option is Option B-lite** (reuse a minimal loopback HTML form) OR a `walk`-style dialog. See Design Forks for the trade-off — this is the crux of the onboarding fork. |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| New `internal/backend` net/http client | Reuse `internal/backendsrv/enrich/politefetch` | `politefetch` is a *server-side* package (lives under `backendsrv`, has ETag/backoff/LimitReader tuned for polite scraping of PigParse/wiki). Importing a `backendsrv` package into the watcher binary is an architectural smell (couples client to server internals) and would drag server-only concerns. **Recommendation: write a fresh small watcher-side client**; optionally mirror politefetch's backoff *shape* (`[2s,4s,8s,16s,32s]`) without importing it. |
| Win32 native input dialog (Option A) | Minimal loopback HTML form (Option B) | A loopback form reuses the wizard's HTTP machinery but keeps a localhost listener alive (the exact thing the deletion is shrinking); a native dialog is more code to write but zero network surface. The user decision picks the trade-off. |

**Installation:** No new dependencies required for the happy path (POST client + DPAPI + tray are all already in `go.mod`). A native-input-dialog onboarding (Option A) may add a small Win32 helper using the already-present `golang.org/x/sys/windows`.

**Version verification:** All listed libraries are pinned in the current `go.mod` (go 1.25.7) and unchanged by this phase. [VERIFIED: `go.mod` read 2026-05-29]. No registry lookups needed — P13 adds no third-party dependency on the happy path. The deletion will let `go mod tidy` DROP the Google dependency tree (`golang.org/x/oauth2`, `google.golang.org/api`, `cloud.google.com/go/auth`, `google.golang.org/grpc`, `google.golang.org/genproto`, plus their transitive `s2a-go`, `gax-go`, `enterprise-certificate-proxy`, etc.) — this is the materially-smaller-binary success criterion (SC-2). [VERIFIED: go.mod indirect block lists the full Google tree]

## Architecture Patterns

### System Architecture Diagram (the re-targeted upload path)

```
  EQ writes <Char>-Inventory.txt / <Char>-Spellbook.txt  (CP1252 on disk)
                          │
                          ▼
        ┌─────────────────────────────────────────┐
        │  internal/watch (SURVIVOR — unchanged)   │
        │  fsnotify event → 500ms debounce →       │
        │  dispatch path to OnChange callback      │
        └─────────────────────────────────────────┘
                          │  path
                          ▼
        ┌─────────────────────────────────────────┐
        │  internal/app callback (REWRITTEN)        │
        │  re-stat → os.Open →                      │
        │  parse.CP1252Reader(f) → read ALL bytes   │   ← decode ONCE here (A1)
        │  as UTF-8 string  (NO parse.Parse!)       │   ← server parses, not watcher
        │  skip-if-empty guard (T-07-05 carry-over) │
        └─────────────────────────────────────────┘
                          │  (charName, kind, utf8Content, version)
                          ▼
        ┌─────────────────────────────────────────┐
        │  internal/backend (NEW — ~150 LOC)        │
        │  ReadGuildCode() from DPAPI wincred        │
        │  build Envelope{character,kind,content,    │
        │     watcher_version}                       │
        │  POST {base}/api/v1/ingest                  │
        │     Authorization: Bearer <code>            │
        │     User-Agent: SquireBot/<ver>             │
        │  classify response:                         │
        │   2xx(204)→ok  401→re-prompt  409→cross-own │
        │   426/4xx-version→"update needed"  5xx→retry│
        └─────────────────────────────────────────┘
                          │  HTTPS
                          ▼
            api.squirebot.quest  (Caddy → loopback Go server)
            POST /api/v1/ingest  (LIVE, P11 — server parses + atomic replace)
```

File-to-implementation mapping is in the Deletion Map and Component tables below, not in this diagram.

### Recommended Project Structure (post-deletion watcher)
```
cmd/squirebot/
├── main.go              # REWRITTEN: drop OAuth BuildConstants, drop reauth/changeWorkbook wiring,
│                        #            add first-launch config migration call
├── build_constants.go   # GUTTED: drop OAuthClientID/Secret/PickerAPIKey/GCPProjectNumber; keep Version
│                        #         (optionally add BackendBaseURL default + ldflag)
└── console_windows.go   # SURVIVOR (fix 999.20 gofmt + 999.21 freeConsole doc here)
internal/
├── backend/             # NEW: HTTP ingest client (POST envelope + bearer + version header + classify)
├── credstore/  (or keep in a slimmed internal/auth)  # NEW/SURVIVOR: DPAPI guild-code Read/Store/Delete
├── onboarding/ (or fold into tray/app)  # NEW: "paste your guild code" prompt + validate-against-backend
├── app/
│   ├── runapp.go        # HEAVILY REWRITTEN: watch→read→POST flow; drop ts/sheet/scaffold/heartbeat-to-sheets
│   └── (reauth.go DELETED)
├── watch/   parse/   config/   tray/   update/   eqfind/   system/   logging/   # SURVIVORS
│   (config.go: drop SpreadsheetID/GoogleEmail fields; add GuildCodeStored bool? + BackendBaseURL?)
│   (parse: keep CP1252Reader + the parsers' tests; the parsers themselves are now only used server-side
│            BUT they still live in the shared module — the BACKEND imports internal/parse, so DO NOT delete them)
└── (auth/ sheet/ scaffold/ picker/ wizard/ DELETED — except the wizard EQ-folder logic, relocated)
```

> **Critical nuance:** `internal/parse` MUST NOT be deleted — the **backend** (`internal/backendsrv/ingest/handler.go`) imports `github.com/boejowen/SquireBot/internal/parse` for `parse.Parse`/`ParseSpellbook`. It is a *shared* package in the same Go module. The watcher stops calling `parse.Parse` but still calls `parse.CP1252Reader`; both stay. [VERIFIED: handler.go line 42 imports internal/parse]

### Pattern 1: Thin POST client mirroring the server's net/http verdict
**What:** A small stdlib client; no router, no SDK. POST a JSON envelope with a bearer header.
**When to use:** The `internal/backend` upload sink.
**Example:**
```go
// internal/backend/client.go  (NEW — sketch; mirrors update/check.go's net/http shape + buildinfo UA pattern)
type Client struct {
    baseURL string
    http    *http.Client
}
func New(baseURL string) *Client {
    return &Client{baseURL: baseURL, http: &http.Client{Timeout: 30 * time.Second}}
}
// Ingest POSTs one snapshot. content is UTF-8 (already CP1252-decoded by the caller — DO NOT decode here).
func (c *Client) Ingest(ctx context.Context, code, character, kind, content, version string) error {
    env := map[string]string{
        "character": character, "kind": kind,
        "content": content, "watcher_version": version,
    }
    body, _ := json.Marshal(env)
    req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/ingest", bytes.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+code)
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("User-Agent", "SquireBot/"+version) // version header carried for the version-gate (mirrors buildinfo.UserAgent)
    resp, err := c.http.Do(req)
    if err != nil { return fmt.Errorf("ingest POST: %w", err) }
    defer resp.Body.Close()
    return classify(resp.StatusCode) // 204→nil; 401→ErrUnauthorized; 409→ErrCrossOwner; 426/4xx→ErrVersionTooOld; 5xx→retryable
}
```
Source pattern: `internal/update/check.go` (net/http + ctx + UA) and `internal/backendsrv/buildinfo/buildinfo.go` (the `SquireBot/<Version>` UA convention). [VERIFIED: both files read]

### Pattern 2: Bounded retry/backoff (match the server's politeness shape without importing it)
**What:** On a transient (network/5xx) failure, retry a small bounded number of times with backoff; on 401/4xx/409 do NOT retry (terminal — surface to tray/onboarding).
**When to use:** Inside `Ingest` or its caller in the rewritten `runapp` callback.
**Recommendation:** Reuse the **shape** of the existing `internal/sheet/retry.go` `withRetry` envelope (the v1 watcher already has a tested retry pattern for the hot path) but simplified — the new sink has cleaner error semantics than Sheets' 403-flavored auth errors. A simple `[1s,2s,4s]` bounded retry on network/5xx is sufficient; uploads are idempotent (full-snapshot replace), so a re-POST is safe. **Do NOT build an unbounded retry loop** (CONTEXT.md locked invariant from v1: "no silent retry-loop after invalid_grant" — the analog here is "no silent retry after 401"; a 401 means a bad/missing code → re-prompt, don't hammer).

### Anti-Patterns to Avoid
- **Double-decoding CP1252 (A1 violation):** The watcher's callback decodes via `parse.CP1252Reader` ONCE on the disk read, producing UTF-8. The `internal/backend` client POSTs that UTF-8 string straight into `content` with **no further decode**. The server's `parseContent` feeds `content` straight into the parser with **no CP1252 decode** (handler.go line 145 — `strings.NewReader(env.Content)`). Decoding twice mojibakes curly apostrophes (U+2019 → garbage). [VERIFIED: handler.go comment lines 102-104 + 137-141, and the locked STATE.md A1 decision]
- **Calling `parse.Parse` in the watcher before POST:** v1 parsed to cells then wrote cells. v2 sends RAW text (D-03). Parsing client-side and sending parsed rows would (a) duplicate the server's parsing, (b) break the contract (the server expects raw `/outputfile` text in `content`), (c) re-introduce a second parsing truth. Send the raw (CP1252-decoded-to-UTF-8) file text.
- **Importing a `backendsrv/*` package into the watcher:** keeps client/server decoupled. Mirror patterns; don't import server internals.
- **Keeping a loopback HTTP listener "just in case":** if Option A (native dialog) is chosen, delete the wizard's HTTP server entirely — a localhost listener is attack surface and complexity the deletion is meant to remove.
- **Sending `valueInputOption`/Sheets concepts:** all gone. The server owns persistence.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| DPAPI at-rest credential storage | A custom encrypt-to-file scheme | The SURVIVING `wincred` helper (rename `auth.Store/Read/DeleteToken` → guild-code variants) | Already shipped + tested; DPAPI keyed to user profile; `cmdkey /list` groups under `SquireBot:` prefix [VERIFIED: auth/store.go] |
| File-change pipeline | A new poller / new debouncer | `internal/watch` (SURVIVOR) | 500 ms debounce + always-re-read + parent-dir-watch + Windows-payload-distrust are all hard-won; the SINK swap must not touch them |
| HTTP retry/backoff | A bespoke exponential algorithm from scratch | The shape of `internal/sheet/retry.go` `withRetry` (simplified) OR a tiny bounded loop | A tested envelope exists; the new error semantics are simpler |
| Version comparison for "update needed" | A new SemVer parser | `internal/update/manifest.go` `IsNewer`/`parseVersion` (SURVIVOR; **but see 999.22**) | Already does a 3-part numeric compare; reuse for any client-side version logic [VERIFIED: manifest.go] |
| Native folder selection (onboarding still needs the EQ folder) | A new dialog | `wizard/folderpicker_dialog.go` `defaultPickFolder` (sqweek/dialog) — RELOCATE, don't rewrite | Cross-platform, pinned, tested [VERIFIED] |
| Guild-code validation endpoint | (decision) a brand-new auth subsystem | A tiny authed backend probe — either reuse `ResolveToken` behind a new `GET /api/v1/whoami` OR validate-via-trivial-authed-call | The backend's `auth.ResolveToken` guard already exists; a validation endpoint is a ~20-LOC handler reusing it. See Design Forks. |

**Key insight:** This phase's value is in *deleting* code safely, not writing new code. The new surface (`internal/backend` + a credstore rename + an onboarding prompt) is small (~300-400 LOC); the deleted surface is ~8k LOC. The risk is in the rewire (every call site of a deleted package) and in NOT breaking the survivor pipeline — not in novel algorithms.

---

## Deletion Map (the load-bearing deliverable)

> Legend: **DELETE** = remove the package/file entirely · **PRESERVE-PARTS** = salvage named logic, delete the rest · **REWIRE** = a call site that must change · **GUT** = keep the file, strip most contents.

### Packages — DELETE entirely
| Package | LOC | Verdict | Notes |
|---------|-----|---------|-------|
| `internal/auth` | 1471 | **DELETE most; PRESERVE the wincred store** | `oauth.go`, `oauthconfig.go`, `pkce.go`, `refresh.go`, `browser.go`, `userinfo.go`, `BuildConstants` + their tests all DIE (OAuth/PKCE/Drive). **PRESERVE `store.go`** (DPAPI wincred Read/Store/Delete) — relocate to `internal/credstore` (or a slimmed `internal/auth`) and rename `StoredToken{RefreshToken,...}` → a guild-code shape. `IsRevokedRefreshToken` (refresh.go) DIES. |
| `internal/sheet` | 4160 | **DELETE entirely** | The whole Sheets client: `client.go`, `write.go`, `heartbeat.go`, `meta.go`, `owner.go`, `retry.go`, `ensure_tab.go`, `scaffold_helpers.go`, `client_helpers.go` + tests. **`WatcherMaxSchemaVersion` + `CanonicalID` + `ErrSchemaTooNew`/`ErrWrongWorkbook` all DIE** (schema-evolution gate retires → API versioning, SC-5). The `retry.go` *pattern* can be mirrored into `internal/backend` but the package goes. [VERIFIED: client.go, write_test exist] |
| `internal/scaffold` | 820 | **DELETE entirely** | Workbook scaffolding (`ScaffoldSchemaV1`) — no workbook exists in v2. Imports `internal/sheet` (dies with it). [VERIFIED: scaffold.go imports internal/sheet] |
| `internal/picker` | 740 | **DELETE entirely** | Drive Picker loopback server + `picker.html`. Imports `auth` + `sheet`. [VERIFIED] |
| `internal/wizard` | 916 | **DELETE most; PRESERVE EQ-folder logic** | See the wizard split table below — this is the one package that needs surgical salvage. |

### `internal/wizard` — the surgical split (CRITICAL)
| File / function | Verdict | Reason |
|-----------------|---------|--------|
| `server.go` Run() OAuth phase (Phase A: `authMgr.DoneChan()`, `auth.NewManagerWithListener`) | **DELETE** | OAuth loopback flow |
| `server.go` picker attach (Phase B prelude: `s.attachPicker`, `defaultSheetClient`, `defaultAttachPicker`) | **DELETE** | Drive Picker |
| `server.go` `handleEQFolderGET` / `handleEQFolderPick` / `handleEQFolderConfirm` | **PRESERVE-PARTS** | EQ-folder selection + validation + `cfg.Save()`. This is the reusable onboarding heart. The `eqfind.Discover()` + `eqfind.ValidateFolder()` + sqweek dialog + the "doesn't look like EverQuest" message all stay. |
| `server.go` `handleStart`/`handleDone`/`handleShutdown` + `start.html`/`done.html`/`eq-folder.html` | **DELETE or REDUCE** | If Option A (native dialog) → delete all HTML + the loopback server. If Option B (loopback form) → keep a reduced `eq-folder.html` + a new one-field `guild-code.html`, delete `start.html` (OAuth CTA) + `done.html`'s OAuth framing. |
| `folderpicker_dialog.go` `defaultPickFolder` | **PRESERVE (relocate)** | sqweek native folder dialog — survives wherever the EQ-folder step lands. |
| `Result{Email, SpreadsheetID, EQFolder, TokenSource}` | **REWIRE** | Drop Email/SpreadsheetID/TokenSource; the onboarding result is just `{EQFolder, (guild code already stored in wincred)}`. |
| `server_test.go` | **DELETE/REWRITE** | Imports `auth` + `sheet`; rewrite for the slimmed flow. |

### `internal/app` — REWIRE (the orchestrator)
| File / function | Verdict | Reason |
|-----------------|---------|--------|
| `runapp.go` `RunApp` (wizard-vs-watcher branch, `oauth2.TokenSource`, `bc.Validate`) | **HEAVILY REWRITTEN** | Drop the TokenSource plumbing; branch on "guild code present in wincred?" instead of `needsWizard`. On absent code → onboarding prompt; on present → start watcher. |
| `runapp.go` `runWatcher` (sheet.NewClient, ValidateWorkbook, ScaffoldSchemaV1, heartbeat.Run-to-sheets, SetOnRefresh) | **HEAVILY REWRITTEN** | Replace `sheet.NewClient`+validate+scaffold with `backend.New(baseURL)`. Drop `ValidateWorkbook`/`ScaffoldSchemaV1` entirely. Keep the folder resolution + `rescanCatchUp` + `watch.Run` wiring. **Decide heartbeat fate (see below).** |
| `runapp.go` `makeOnInventoryChange` / `makeOnSpellbookChange` | **REWRITTEN (core change)** | Keep: re-stat, mtime capture, `os.Open`, `parse.CP1252Reader(f)`, skip-if-empty, mtime persist + `cfg.Save`. **Change:** instead of `parse.Parse(...)` + `sc.WriteInventory(...)` + `UpsertCharOwner(...)`, do `io.ReadAll(parse.CP1252Reader(f))` → `backend.Ingest(ctx, code, char, "inventory", string(utf8Bytes), version)`. Drop all the `ErrPermanentAuth`/`globalPostReauthPending`/`runPostReauthProbe`/`suspendForAuth` Google-propagation machinery. New error handling: 401 → re-prompt/red; 409 → log cross-owner; version-too-old → "update needed" tray; transient → bounded retry then warn. |
| `runapp.go` `buildTokenSourceFromWincred` / `swappableTS` / `applyBootAuthError` / `runPostReauthProbe` / `isPermanentAuthErr` / `suspendForAuth` | **DELETE** | All OAuth-token / drive.file-propagation specific. |
| `runapp.go` `ChangeWorkbook` (D-04) | **DELETE** | No workbook to change. The tray "Change Workbook…" item dies (see tray below). |
| `reauth.go` (entire file, 290 LOC) | **DELETE** | The whole AUTH-05 reauthorize state machine + `globalAuthSuspended`/`globalReauthTSCh`/`globalPostReauthPending` + `Reauthorize`/`RunReauthorize`. v2 has no refresh-token-death path; a bad guild code is a re-prompt, not a reauth. |
| `runapp_test.go` / `reauth_test.go` | **DELETE/REWRITE** | Import `auth`+`sheet`. |

### `cmd/squirebot` — REWIRE
| File / function | Verdict | Reason |
|-----------------|---------|--------|
| `main.go` `bc := auth.BuildConstants{...}` + the `auth` import | **DELETE** | No OAuth constants. |
| `main.go` `--uninstall-wipe-credentials` (reads `cfg.GoogleEmail`, calls `auth.DeleteToken`) | **REWIRE** | Change to delete the new guild-code wincred entry (no email key — the new target is e.g. `SquireBot:guild-code` or `SquireBot:backend-token`). |
| `main.go` tray.Config `OnChangeWorkbook` / `OnReauthorize` callbacks + `Open Workbook` (spreadsheet URL) | **DELETE/REWIRE** | Drop both callbacks. The onboarding-prompt trigger replaces them (e.g. an `OnEnterGuildCode` callback). |
| `main.go` startup-swap (`update.Apply`), `--quit`, `freeConsole`, logging, systray.Run | **SURVIVOR** | Unchanged. Add a first-launch config-migration call (see WATCH-11). |
| `build_constants.go` `OAuthClientID/Secret/PickerAPIKey/GCPProjectNumber` | **DELETE** | Keep `Version`. Optionally add `BackendBaseURL = "https://api.squirebot.quest"` (ldflag-injectable) — or put the default in config. |
| `console_windows.go` | **SURVIVOR** + fix 999.20/999.21 here | gofmt + freeConsole doc nits ride along. |

### Survivors (touch only as noted)
`internal/watch` (untouched — the pipeline invariant) · `internal/parse` (keep BOTH the parsers (server uses them) AND `CP1252Reader` (watcher uses it)) · `internal/config` (drop `SpreadsheetID`/`GoogleEmail` fields; see migration) · `internal/tray` (re-label menu; drop Change-Workbook/Reauthorize/Open-Workbook-to-Sheets) · `internal/update` (untouched transport; fix 999.22) · `internal/eqfind` (untouched — onboarding still discovers the EQ folder) · `internal/system` (untouched) · `internal/logging` (untouched) · `internal/heartbeat` (**DECISION — see below**).

### Heartbeat fate (a sub-decision the planner must resolve)
`internal/heartbeat` (`heartbeat.Run`) fires every 24h and calls `writer.WriteHeartbeat` — which is **`sheet.Client.WriteHeartbeat`** (writes `_char_owner.last_seen` + `_status` cells in the Sheet). [VERIFIED: heartbeat.go interface + sheet/heartbeat.go]. With Sheets gone there is **no `_status` table consumer in the backend** (the P11 schema has `owner`/`character`/`inventory_item`/`spellbook_entry`/`guild_code` + dimension tables — no heartbeat/status table). **Recommendation: DROP the heartbeat goroutine for P13** (the backend already stamps `uploaded_at` on every ingest, which is a truer liveness signal than a separate heartbeat; per-char freshness is derivable from `inventory_item.uploaded_at`). If a future "last seen" admin view is wanted (P15), add a backend heartbeat endpoint then — out of P13 scope. The `internal/heartbeat` package can be deleted (it's Sheets-coupled via the `writer` interface's only impl) OR kept dormant; recommend delete for the SC-2 "no Google code" cleanliness. Note: the v1 `WATCH-08` requirement ID referred to the *heartbeat*; the v2 `WATCH-08` is the *re-target* — different requirement, same ID namespace reused across milestones.

---

## `internal/backend` client design

| Aspect | Decision | Rationale |
|--------|----------|-----------|
| **Package** | New `internal/backend` (watcher-side) | Decoupled from `internal/backendsrv/*` (server). |
| **Call shape** | `Ingest(ctx, code, character, kind, content, version) error` | One method; mirrors the Envelope. `kind ∈ {"inventory","spellbook"}` (reuse the literals; the server validates). |
| **Transport** | stdlib `net/http`, `http.Client{Timeout: 30s}`, `http.NewRequestWithContext` | Mirrors `update/check.go`; no SDK. |
| **Endpoint** | `POST {baseURL}/api/v1/ingest` | The shipped contract. [VERIFIED: cmd/squirebot-server/main.go:257] |
| **Headers** | `Authorization: Bearer <code>`, `Content-Type: application/json`, `User-Agent: SquireBot/<version>` | Bearer per D-08; UA carries the version (the version-gate vector — mirrors `buildinfo.UserAgent`). |
| **Body** | `json.Marshal(Envelope{Character, Kind, Content, WatcherVersion})` | `content` is the UTF-8 (already-CP1252-decoded) raw file text. **The watcher_version ALSO travels in the body** (the envelope already has the field + the server already stores it) — sending it in BOTH the UA header and the JSON field is belt-and-braces; the version-gate can read either (see Design Forks). |
| **Base URL source** | Config field `backend_base_url` (default `https://api.squirebot.quest`) OR an ldflag in `build_constants.go` | A config default keeps it overridable for testing/self-hosting; recommend config with a hardcoded fallback constant. |
| **Token source** | Read from DPAPI wincred on each POST (or cache in memory after first read) | The guild code lives ONLY in wincred (never config — the AUTH-04 "no secret in config.json" rule carries over verbatim). |
| **Error classification** | `204→nil`; `401→ErrUnauthorized` (re-prompt); `409→ErrCrossOwner` (log, surface); `426 Upgrade Required` or whatever the version-gate returns `→ErrVersionTooOld` (tray "update needed"); `400/422→ErrBadPayload` (log — should not happen); `5xx/network→retryable` | Terminal vs retryable split drives the retry decision. |
| **Retry** | Bounded (e.g. 3 attempts, `[1s,2s,4s]`) on retryable only; never on 401/409/version | Idempotent full-snapshot replace makes re-POST safe; NO unbounded loop (locked invariant analog). |
| **Body cap awareness** | The server caps at 1 MiB (`MaxBytesReader`); a maxed char snapshot is <50 KB | No client-side cap needed; document the server cap. |
| **Logging (V7)** | NEVER log the bearer code or raw `content`; log char/kind/status/err only | Mirrors the server's handler logging discipline. |

---

## Design Forks for User Decision

> These are the genuine forks. Each has 2-3 concrete options + a recommended default + the trade-off, phrased so the orchestrator can drop them into one AskUserQuestion round before planning.

### FORK 1 (PRIMARY): Onboarding UX — how does the guildie paste the guild code?

The watcher must collect a one-line opaque token on first run (and on re-prompt after a 401), validate it against the backend, store it in DPAPI, and go green. No browser OAuth. Three realistic options:

| Option | What it is | Code impact | Validation path | Trade-off |
|--------|-----------|-------------|-----------------|-----------|
| **A (RECOMMENDED): Tray → native Windows input dialog** | Tray menu item "Enter guild code…" opens a small native text-input box (Win32 `DialogBox`/`InputBox` via `golang.org/x/sys/windows`, or a `walk`-style helper). No browser, no loopback server. | **Lowest network surface, most code to write** (sqweek/dialog has no text-entry box, so a small Win32 input dialog must be written — ~80-120 LOC of dialog template, OR a pragmatic shell-out to a PowerShell/VBScript InputBox which is fragile). Deletes the wizard HTTP server entirely. | Calls `backend.Validate(code)` (a lightweight authed probe — see FORK 2 dependency) before storing. | Cleanest end state (zero localhost listener — the deletion's spirit); the native dialog is the one piece of genuinely new UI code. |
| **B: Reduced loopback HTML form** | Reuse the wizard's loopback `net/http` listener but collapse it to ONE page: a single `<input>` for the guild code (+ the surviving EQ-folder step). Opens the browser once to `http://127.0.0.1:PORT/setup`. | **Least *new* code** (reuses the wizard's proven HTTP+template+browser-open machinery + the EQ-folder handlers verbatim). But it KEEPS a localhost listener — the exact surface the deletion is shrinking. | The form POSTs the code to a local handler that calls `backend.Validate(code)`; on 200 stores + redirects to "done". | Fastest to build, but retains ~200 LOC of loopback server + an open port + a browser dependency that Option A removes. Philosophically at odds with "shed the loopback stack." |
| **C: Config-file field + tray "Validate" action** | The guildie pastes the code into `config.json` (a `guild_code` field) and clicks a tray "Validate & save" item that reads it, validates, moves it to wincred, and BLANKS the config field. | **Least UI code**, but **worst UX** (asks a non-technical guildie to hand-edit JSON) AND a security smell (the plaintext code transits a plaintext file before moving to DPAPI). | Tray action reads `cfg.GuildCode`, calls `backend.Validate`, on success `credstore.Store` + clear the config field + `cfg.Save`. | Rejected for UX + the plaintext-in-config window. Listed only for completeness. |

**Recommended default: Option A.** It best honors the milestone's "shed the loopback/OAuth stack" goal (SC-2/WATCH-09) — the deletion removes the wizard's HTTP server entirely rather than keeping a reduced one. The cost is writing one small native input dialog. **If the user prefers to minimize *new* code over minimizing *surface*, Option B is the pragmatic alternative** (reuses the wizard machinery, ships faster, but keeps a localhost listener). The orchestrator should present A vs B as the real choice (C is a fallback).

> **Note for the planner:** whichever option wins, the EQ-folder selection step (sqweek dialog + `eqfind` validation + `cfg.Save`) is preserved from the wizard and runs as part of the same onboarding (a first-run guildie needs BOTH their guild code AND their EQ folder). Option A folds the EQ-folder step into a second native dialog (or reuses the existing one — sqweek HAS a folder dialog); Option B keeps the `eq-folder.html` page.

### FORK 2: Version-rejection scope — does P13 pull a backend edit into scope?

Success criterion 5 says "the watcher sends its version and the backend rejects-with-clear-message a watcher too old for the API version." **Current state (verified):** the `Envelope.watcher_version` field already exists and is sent in tests; the `inventory_item.watcher_version` DB column exists; the handler already STORES it via `ReplaceInventoryTx(..., env.WatcherVersion)`; the field is explicitly documented "accepted now; the version-gate *reject* lands in P13." [VERIFIED: envelope.go:30, 00001_init.sql:19, handler.go:204]. So this is **not** a from-scratch fork — it's "where does the floor check live?"

| Option | What it is | Backend change? | Trade-off |
|--------|-----------|-----------------|-----------|
| **A (RECOMMENDED): Tiny backend min-version gate in the ingest handler** | Add a `MIN_WATCHER_VERSION` constant (or config) to the backend; in `ServeHTTP`, after decode, if `IsOlder(env.WatcherVersion, MIN)` → return `426 Upgrade Required` (or `400`) with a clear message ("Your SquireBot is too old; it will auto-update shortly."). Watcher classifies that status → tray "update needed" + lets the auto-updater handle it. | **YES — small (~30 LOC + 1 const + a version-compare helper)**, in `internal/backendsrv/ingest/handler.go`. Reuses a 3-part numeric compare (port `update/manifest.go`'s `parseVersion` server-side, or add to `buildinfo`). | Honors SC-5 ("old watcher refuses to corrupt data" guarantee survives the move from `WatcherMaxSchemaVersion` to API versioning). Pulls a *small, well-bounded* backend edit into P13. The backend is live, so this ships with a redeploy (already the P12 deploy cadence). **This is the faithful realization of SC-5.** |
| **B: API-path versioning only (no min-version gate)** | Rely solely on `/api/v1/` in the path. A future breaking change ships `/api/v2/`; old watchers keep hitting `/api/v1/` (which stays working) until they auto-update. No per-version reject. | **NO backend change.** | Cheaper, but does NOT satisfy SC-5's "rejects-with-clear-message a watcher too old" literally — there's no active reject, just path coexistence. SC-5 would be only partially met. Acceptable ONLY if the user reads SC-5 as "the guarantee is structural via path versioning" rather than "an explicit reject." |
| **C: Watcher self-checks its version against the live GitHub manifest** | The watcher's existing `update` check already knows the latest version; gate uploads client-side if too far behind. | **NO backend change**, but moves the guarantee to the client (which is the thing that might be compromised/old — weaker). | Weakest: a too-old/forked watcher is exactly the client you can't trust to self-gate. Not recommended. |

**Recommended default: Option A.** It is the only option that literally satisfies SC-5, the backend hook is tiny and well-scoped, and the deploy path already exists (P12 left a DEPLOY-PENDING redeploy anyway). The version field + DB column already ship, so A is ~30 LOC + a redeploy. **The fork to surface:** "SC-5 requires an active backend reject (Option A, small backend edit) — confirm P13 may include that ~30-LOC backend change, or descope to path-versioning-only (Option B)."

### FORK 3 (minor): Where does the guild-code validation endpoint live?

Onboarding (FORK 1, any option) needs to *validate* a pasted code before storing it. **Current state (verified):** the backend serves EXACTLY ONE route — `POST /api/v1/ingest` — there is **no `whoami`/validate endpoint**. [VERIFIED: only `mux.Handle("POST /api/v1/ingest", ...)` exists]. Two ways to validate:

| Option | What it is | Backend change? | Trade-off |
|--------|-----------|-----------------|-----------|
| **A (RECOMMENDED): Add a tiny `GET /api/v1/whoami` (authed)** | A ~20-LOC handler that runs the existing `auth.ResolveToken` guard and returns `200 {owner_label}` on a valid code, `401` otherwise. Onboarding calls it; clean, side-effect-free. | **YES — ~20 LOC**, reuses `ResolveToken` verbatim. | Clean validation with no side effects; gives the onboarding a crisp success/fail. Pairs naturally with FORK 2 Option A (both are small ingest-adjacent backend additions, ship in one redeploy). Also useful for P15. |
| **B: Validate-via-trivial-ingest** | Onboarding POSTs a harmless empty-content snapshot (or a probe) to `/api/v1/ingest` and reads the status (401 = bad code, 204/4xx = good code). | **NO backend change**, but it's a side-effecting write path used as a read probe — ugly, and an empty-content ingest with a fabricated character name could create/touch state. | Avoids a backend edit but abuses the ingest path; risk of unintended writes (the empty snapshot "clears a character's rows" — if the probe uses a real char name it could wipe data). Not recommended. |

**Recommended default: Option A** (`GET /api/v1/whoami`). Bundle it with FORK 2 Option A — both are small, both reuse existing guards, both ship in the same backend redeploy. **If the user wants ZERO backend changes in P13**, FORK 3-B + FORK 2-B together keep P13 watcher-only — but at the cost of an abused probe and a weaker SC-5. **The cleanest, smallest-honest path is: one backend redeploy adding `GET /api/v1/whoami` (~20 LOC) + a min-version gate in the ingest handler (~30 LOC).**

> **Bundling guidance for the orchestrator:** FORKS 2 and 3 are really one question — *"May P13 include a small (~50-LOC total) backend addition (`/whoami` + min-version reject), shipped in one redeploy?"* If YES → SC-5 is faithfully met + onboarding validates cleanly. If NO (watcher-only P13) → both fall back to weaker watcher-side/probe approaches. FORK 1 is the independent, primary UX question (native dialog vs loopback form).

---

## Token Storage (WATCH-10)

| Aspect | Decision | Source |
|--------|----------|--------|
| **Mechanism** | Windows Credential Manager via `github.com/danieljoos/wincred`, `PersistLocalMachine`, DPAPI-encrypted at rest keyed to the user profile | SURVIVOR of `internal/auth/store.go` [VERIFIED] |
| **What survives** | The `wincred.NewGenericCredential` / `.Write` / `GetGenericCredential` / `.Delete` mechanics + the `SquireBot:` target prefix convention | `auth/store.go` `StoreToken`/`ReadToken`/`DeleteToken` |
| **What changes** | `StoredToken{RefreshToken, Email, ClientID}` JSON blob → a guild-code shape. Target name: drop the `<email>` key (no email in v2) → a fixed target e.g. `SquireBot:guild-code` (single credential per machine). The blob can be the raw code or `{code, backend_base_url}` JSON for forward-compat. | Rename `StoreToken(email, st)` → `StoreGuildCode(code)`; `ReadToken(email)` → `ReadGuildCode()`; `DeleteToken(email)` → `DeleteGuildCode()`. |
| **What's OAuth-specific (dies)** | `ClientID` field, the "issued for the current OAuth client" sanity-check rationale, the `Email`-keyed target | — |
| **Relocation** | Move to `internal/credstore` (or keep a slimmed `internal/auth` with ONLY the store) — but the rest of `internal/auth` dies, so a fresh small `internal/credstore` is cleaner | — |
| **Storage discipline** | The code NEVER touches `config.json` (AUTH-04 rule carries verbatim: "NEVER add a refresh_token / any secret field to Config") — the config package even documents this. Apply the same to the guild code. | `config.go:5-6` SECURITY comment |
| **Hash-at-rest?** | NO — the watcher needs the *plaintext* code to send as the Bearer value on every POST (unlike the server, which stores only the hash). DPAPI encryption-at-rest is the protection; the plaintext is decryptable only by the user's profile. | The server hashes (`auth/store.go` server-side); the client must keep plaintext to present it. |

---

## Auto-Update Migration (WATCH-11)

**Flow:** An existing v1.x watcher auto-updates to the re-targeted binary via the EXISTING GitHub-Releases pipeline (`internal/update`, untouched transport — it already fetches `latest.json` + the bare binary via direct net/http to GitHub's CDN, **never** Google). On first launch of the new binary:

| Step | Action | Source/Detail |
|------|--------|---------------|
| 1. Startup-swap runs | `update.Apply()` swaps `.new`→live (unchanged) | `cmd/squirebot/main.go:91` (SURVIVOR) |
| 2. First-launch migration | A new migration function runs BEFORE the watcher branch: detect "v1 config present" (e.g. `cfg.GoogleEmail != ""` or `cfg.SpreadsheetID != ""`) and clean it | NEW — call from `main.go` after `config.Load`, or in `RunApp` |
| 3. Drop dead config fields | Remove `SpreadsheetID`, `GoogleEmail` from `Config` struct + `cfg.Save()` to rewrite `config.json` without them. Keep `EQFolder(s)`, `LastKnown*Mtime`, `LogLevel`, `Version`, `PendingUpdateVersion`. Add `backend_base_url` (default). | `config.go` struct edit — fields drop on next marshal [VERIFIED: config.go:22-32] |
| 4. Delete stale Google wincred entry | The v1 entry is `SquireBot:<google-email>`. Read the (about-to-be-removed) `cfg.GoogleEmail`, call the OLD-style delete on `SquireBot:<email>` ONCE, then never again. (Or: enumerate `SquireBot:*` and delete any non-`guild-code` entry.) | Mirrors `--uninstall-wipe-credentials` logic [VERIFIED: main.go:39-55] — but as a one-time forward migration, not an uninstall |
| 5. No backend credential found | `ReadGuildCode()` returns not-found → trigger the onboarding prompt (FORK 1) ONCE | The "one manual step: paste guild code" of WATCH-11 |
| 6. After paste + validate + store | Watcher goes green, starts the watch→read→POST pipeline | — |

**Migration idempotency:** gate on a sentinel (e.g. once `GoogleEmail`/`SpreadsheetID` are absent AND a `guild_code` exists in wincred, the migration is a no-op). Re-running on an already-migrated config does nothing. Note `config.Load` already strips a UTF-8 BOM and shims `EQFolder→EQFolders` — the migration adds to that forward-compat layer. [VERIFIED: config.go:62-88]

**Auto-update version compare (999.22 — load-bearing for P16):** the coordinated cutover flip (CUTOVER-03) relies on the auto-updater firing across all ~12 watchers. The current `IsNewer`/`parseVersion` (`update/manifest.go`) does a strict 3-part numeric compare and **returns false on ANY parse failure** (defensive) — it does NOT understand pre-release suffixes (`v2.0.0-rc1` would fail `parseVersion`'s `len(parts)!=3` → never newer). For P13/P16 this means a pre-release tag would be invisible to the updater. **Fix in P13:** make the compare SemVer-pre-release-aware (or document that only `MAJOR.MINOR.PATCH` final tags are ever published). Since the coordinated flip ships a *final* `v2.x.0` tag, the minimal fix is to confirm the release tag is `MAJOR.MINOR.PATCH` (no suffix) — but the robust fix (sort pre-releases correctly) de-risks the P16 flip. [VERIFIED: manifest.go:112-149]

---

## Runtime State Inventory

> This is a re-target/deletion phase. A grep finds files; it does NOT find runtime state. Each category answered explicitly.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| **Stored data** | **Per-guildie DPAPI wincred entry** `SquireBot:<google-email>` holding the v1 OAuth refresh token (one per installed machine, ~12 machines). Encrypted, not in any repo. | **Data migration (per-machine, runs in the watcher):** WATCH-11 step 4 — delete the stale entry on first launch. Cannot be done centrally; the watcher must do it on each guildie's PC. |
| **Live service config** | (1) **Google OAuth consent screen / client** (Production mode, brand-verification-rejected) — lives in Google Cloud Console, NOT in git. (2) **The live Google Sheet workbook + Apps Script project** — holds all current guild data. | **NONE in P13.** Decommissioning the OAuth client + Sheet + Apps Script is **CUTOVER-04 (Phase 16)**, explicitly out of P13 scope. P13 only stops the *new* binary from using them. The old Sheet stays live (read-only fallback) through the P16 soak. |
| **OS-registered state** | (1) **`HKCU\...\Run` autostart entry** `SquireBot` → `squirebot.exe` (set by the NSIS installer). (2) **No Task Scheduler / pm2 / systemd** on the watcher side. | **NONE** — the autostart entry points at `squirebot.exe` by path; the auto-update swaps the binary in place (same path), so the Run key needs no change. Verified: autostart is `HKCU\...\Run` per CLAUDE.md; the binary path is stable across auto-update (`.new`→live rename). |
| **Secrets / env vars** | (1) The **guild code** (new secret) — minted server-side, DM'd over Discord, pasted by the guildie → stored in DPAPI (never env/config). (2) The **v1 OAuth client secret** baked into the v1 binary via `-ldflags` (`build_constants.go`) — effectively public per Google's own docs; dies with the rebuild (no longer baked in). (3) **No `.env` on the watcher side.** | **Code change:** drop the OAuth ldflags from the build (`build_constants.go` + CI/release workflow). The guild code is new state collected at onboarding. No key-rename of an existing secret (the guild code is brand new). |
| **Build artifacts / installed packages** | (1) The **v1 `squirebot.exe`** on ~12 machines (replaced by auto-update). (2) **NSIS installer** + `latest.json` manifest in GitHub Releases (the auto-update source — unchanged mechanism, new binary). (3) **No egg-info / compiled caches** (Go statically links). | **Build/release change:** the P13 binary must be published to GitHub Releases with a `latest.json` whose `binary_url`+`binary_sha256` are present (so the in-app swap fires — `check.go` skips swap if `binary_url == ""`). The CI release workflow (`.github/workflows/release.yml`) must drop the OAuth `-ldflags` and (optionally) inject `BackendBaseURL`/`Version`. |

**The canonical question — after every repo file is updated, what runtime systems still carry the old state?** (1) Each guildie's DPAPI store still holds the dead Google refresh token (→ WATCH-11 cleans it per-machine). (2) Google Cloud + the Sheet + Apps Script still exist and hold live data (→ untouched until P16/CUTOVER-04, by design). (3) The autostart Run key + binary path are stable (no action). Nothing else.

---

## Common Pitfalls

### Pitfall 1: Double-decoding CP1252 (mojibake)
**What goes wrong:** The watcher decodes the disk file via `parse.CP1252Reader` (UTF-8 out), then the server *also* CP1252-decodes `content` → curly apostrophes and accented item names become garbage.
**Why it happens:** v1 muscle memory — the parser used to decode. In v2 the decode is split: watcher decodes off disk, server treats `content` as UTF-8.
**How to avoid:** The `internal/backend` client POSTs the already-decoded UTF-8 string verbatim. The server's `parseContent` feeds `content` straight in with NO decode (already true — handler.go:145). Add a test: a CP1252 input file with a curly apostrophe round-trips to the correct U+2019 in the DB.
**Warning signs:** `T_`-prefixed garbage or `â€™` in item names in the DB after a real upload.

### Pitfall 2: Deleting `internal/parse` along with the Sheets stack
**What goes wrong:** The watcher stops calling `parse.Parse`, so it looks deletable — but the BACKEND imports `internal/parse` (same module). Deleting it breaks the server build.
**Why it happens:** It's a shared package in a monorepo; the deletion sweep targets "things the watcher no longer uses."
**How to avoid:** Keep `internal/parse` entirely (parsers + `CP1252Reader`). Only the watcher's *call* to `parse.Parse` moves to the server. [VERIFIED: handler.go:42 imports internal/parse]
**Warning signs:** `go build ./...` fails in `internal/backendsrv/ingest` after the deletion.

### Pitfall 3: Leaving the loopback HTTP server in the binary "for the picker"
**What goes wrong:** The wizard's `net/http` listener + browser-open survive the deletion, keeping a localhost port open and ~200 LOC of attack surface the phase meant to remove.
**Why it happens:** Option B onboarding reuses it; or the EQ-folder step is left on the loopback server out of inertia.
**How to avoid:** If Option A is chosen, delete the wizard HTTP server entirely; run the EQ-folder step via the native sqweek dialog (which already exists). Verify no `net.Listen("tcp"...)` remains in the watcher except where genuinely needed.
**Warning signs:** A `netstat` shows a listening loopback port after onboarding completes; `grep net.Listen cmd/ internal/` finds wizard/picker/reauth remnants.

### Pitfall 4: The migration wipes the EQ folder or mtime maps
**What goes wrong:** The first-launch config migration over-zealously resets `Config` and loses `EQFolder(s)` / `LastKnown*Mtime`, forcing a re-scan/re-upload of everything (cheap but noisy) or a re-pick of the EQ folder (annoying).
**Why it happens:** A blunt "reset config" instead of a surgical "drop only the Google fields."
**How to avoid:** The migration drops ONLY `GoogleEmail` + `SpreadsheetID` (and they drop automatically by removing the struct fields). Preserve everything else. Add a test: a v1 config with EQFolders + mtime maps migrates with those intact.
**Warning signs:** Guildies re-pick their EQ folder or see a full re-upload storm after the update.

### Pitfall 5: 401 triggers an unbounded re-POST loop
**What goes wrong:** A bad/revoked guild code returns 401; a naive retry loop hammers the backend forever.
**Why it happens:** Treating 401 as transient.
**How to avoid:** 401 is TERMINAL → suspend uploads, surface the onboarding prompt / tray-red ("Guild code invalid — re-enter"), do NOT retry. Mirrors the v1 locked invariant "no silent retry-loop after invalid_grant" (the analog: no silent retry after 401). [VERIFIED: CONTEXT.md locked invariant in reauth.go header]
**Warning signs:** Backend logs show a flood of 401s from one watcher.

### Pitfall 6: `watcher_version` not actually populated (SC-5 silently unmet)
**What goes wrong:** The envelope field exists and the server stores it, but the watcher sends `""` (or the dev default), so the version-gate has nothing real to check.
**Why it happens:** The version is in `build_constants.go` `Version` but must be threaded into the `Ingest` call; easy to forget.
**How to avoid:** Thread `Version` (from `main.go`/`build_constants.go`) → `backend.Ingest(..., version)`. Add a test asserting the POST body carries the real version. The release build stamps `Version` via ldflag (already the pattern).
**Warning signs:** DB `inventory_item.watcher_version` shows `"0.1.0-dev"` or empty for real guildie uploads.

---

## Code Examples

### Reading the guild code from DPAPI (survivor pattern, renamed)
```go
// internal/credstore/store.go  (relocated + renamed from internal/auth/store.go)
// Source: internal/auth/store.go (VERIFIED) — mechanics unchanged, blob reshaped.
const credTarget = "SquireBot:guild-code"

func StoreGuildCode(code string) error {
    cred := wincred.NewGenericCredential(credTarget)
    cred.CredentialBlob = []byte(code) // or json.Marshal(struct{Code,BaseURL string}{...})
    cred.Persist = wincred.PersistLocalMachine // DPAPI at rest, keyed to user profile
    return cred.Write()
}
func ReadGuildCode() (string, error) {
    cred, err := wincred.GetGenericCredential(credTarget)
    if err != nil { return "", err } // not-found ⇒ "needs onboarding"
    return string(cred.CredentialBlob), nil
}
func DeleteGuildCode() error {
    cred, err := wincred.GetGenericCredential(credTarget)
    if err != nil { return err }
    return cred.Delete()
}
```

### The rewritten inventory callback core (the SINK swap)
```go
// internal/app/runapp.go (REWRITTEN makeOnInventoryChange core — sketch)
// Source: existing makeOnInventoryChange (VERIFIED) with the parse+write replaced by read+POST.
f, err := os.Open(path)
if err != nil { slog.Error("open inventory", "char", charName, "err", err); return }
utf8Bytes, rerr := io.ReadAll(parse.CP1252Reader(f)) // decode ONCE (A1); do NOT parse.Parse
_ = f.Close()
if rerr != nil { slog.Error("read inventory", "char", charName, "err", rerr); return }
if len(bytes.TrimSpace(utf8Bytes)) == 0 {            // T-07-05 carry-over: skip empty
    slog.Info("inventory empty; skipping upload", "char", charName); return
}
if err := backendClient.Ingest(ctx, code, charName, "inventory", string(utf8Bytes), version); err != nil {
    switch {
    case errors.Is(err, backend.ErrUnauthorized): suspendAndPrompt(t, "Guild code invalid — re-enter"); return
    case errors.Is(err, backend.ErrVersionTooOld): t.SetStatus("Update needed — SquireBot will auto-update"); return
    case errors.Is(err, backend.ErrCrossOwner):   slog.Warn("cross-owner reject", "char", charName); return
    default: slog.Error("upload inventory", "char", charName, "err", err); t.SetStatus("Last upload failed: "+charName); return
    }
}
// persist mtime + cfg.Save() (UNCHANGED from v1)
```

### Backend min-version gate (FORK 2 Option A — server-side, ~30 LOC)
```go
// internal/backendsrv/ingest/handler.go (NEW addition after DecodeAndValidate — sketch)
// minWatcherVersion is the floor; older watchers get a clear 426. Reuses a 3-part compare.
const minWatcherVersion = "2.0.0"
// ... in ServeHTTP, after env decoded:
if env.WatcherVersion != "" && versionOlder(env.WatcherVersion, minWatcherVersion) {
    slog.Info("ingest rejected", "reason", "watcher_too_old", "ver", env.WatcherVersion, "status", 426)
    http.Error(w, "Your SquireBot is too old for this server; it will auto-update shortly.", http.StatusUpgradeRequired)
    return
}
```

### Optional `GET /api/v1/whoami` (FORK 3 Option A — ~20 LOC, reuses ResolveToken)
```go
// internal/backendsrv/ingest/whoami.go (NEW — sketch). Registered: mux.Handle("GET /api/v1/whoami", ...)
func (h *WhoamiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    ownerID, ok := h.guard.ResolveToken(r.Context(), r.Header.Get("Authorization"))
    if !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return }
    // optionally look up owner label; minimal version just 204s
    w.WriteHeader(http.StatusOK)
    _ = json.NewEncoder(w).Encode(map[string]any{"owner_id": ownerID})
}
```

---

## State of the Art

| Old Approach (v1) | Current Approach (v2 / P13) | When Changed | Impact |
|--------------------|------------------------------|--------------|--------|
| Watcher parses file → writes structured cells via Sheets `batchUpdate` | Watcher POSTs RAW file text; server parses + persists | D-03 (P11 CONTEXT) | Watcher is thinner; one parsing truth (server) |
| OAuth 2.0 loopback PKCE + Drive Picker + refresh tokens in wincred | Opaque per-guildie bearer token (guild code) in wincred; no OAuth | v2.0 milestone decision | ~8k LOC deleted; Google dependency tree dropped; no brand-verification gate |
| `_meta.schema_version` ↔ `WatcherMaxSchemaVersion` handshake | Forward-only `goose` DB migrations + `/api/v1/` API version (+ optional min-watcher-version gate) | v2.0 schema-evolution change | The watcher's Sheets schema gate (`ErrSchemaTooNew`) retires; SC-5 realized via the version-gate |
| Refresh-token-death → Reauthorize 2-phase (OAuth + re-pick workbook) | Bad guild code → simple re-prompt (paste again) | P13 | The 290-LOC reauth state machine + drive.file propagation probe all deleted |
| Heartbeat writes `_char_owner.last_seen` + `_status` cells | (dropped; `inventory_item.uploaded_at` is the liveness signal) | P13 recommendation | `internal/heartbeat` + the Sheets `_status` machinery gone |

**Deprecated/outdated:**
- `internal/sheet` (entire Google Sheets v4 client) — replaced by `internal/backend` HTTP POST.
- `internal/auth` OAuth/PKCE/Drive (all but the wincred store) — replaced by bearer-token + DPAPI guild-code storage.
- `internal/scaffold`, `internal/picker`, most of `internal/wizard` — no workbook, no Drive, no OAuth browser flow.
- `WatcherMaxSchemaVersion` / `CanonicalID` — schema-evolution moved to the backend (goose + API version).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The backend serves only `POST /api/v1/ingest` (no validation endpoint exists) | Design Forks 1/3, Architecture | LOW — verified by grep of all `mux.Handle`/`HandleFunc` in `internal/backendsrv` + `cmd/squirebot-server`; if a route was added post-11-05 it would only make FORK 3 easier |
| A2 | Dropping the heartbeat is acceptable (no backend `_status`/heartbeat consumer; `uploaded_at` suffices for liveness) | Deletion Map (heartbeat fate) | MEDIUM — if the maintainer wants a "last seen per guildie" admin view in P15, a backend heartbeat endpoint is needed; deferring it to P15 is the recommendation but the user may want it now. This is a sub-decision for the planner/user. |
| A3 | The version-gate returning `426 Upgrade Required` (vs `400`) is the right status | Design Fork 2, Code Examples | LOW — cosmetic; the watcher classifies whatever status the gate returns. `426` is semantically correct ("upgrade required") but `400`/`409` would also work. |
| A4 | The guild-code wincred target should be a single fixed name (`SquireBot:guild-code`), not email-keyed | Token Storage | LOW — v2 has no email identity client-side (identity is server-derived from the code); a single credential per machine is the natural shape. If a machine ever runs multiple guild codes (not a use case), this would need revisiting. |
| A5 | The release CI workflow (`.github/workflows/release.yml`) bakes OAuth via `-ldflags` and must be edited to drop them + ensure `latest.json` carries `binary_url`/`binary_sha256` | Runtime State Inventory, Auto-Update | MEDIUM — inferred from `build_constants.go`'s ldflag documentation + `check.go`'s "skip swap if binary_url empty"; the actual workflow file was not read in this session. The planner should read `.github/workflows/release.yml` to confirm the exact ldflag lines + manifest emission before editing. |
| A6 | `BackendBaseURL` default `https://api.squirebot.quest` belongs in config (overridable) with a hardcoded fallback | internal/backend design | LOW — could equally be an ldflag-only constant like the old OAuth values; config is recommended for testability/self-hosting but it's a discretion call. |

## Open Questions

1. **Does the maintainer want a per-guildie "last seen" liveness signal in v2 (heartbeat replacement)?**
   - What we know: v1 had a Sheets `_status`/heartbeat; v2's backend has no such table; `inventory_item.uploaded_at` gives per-character upload freshness.
   - What's unclear: whether an explicit "this watcher is alive but hasn't uploaded" signal is wanted (e.g. a guildie running but with no file changes).
   - Recommendation: DROP the heartbeat in P13; if wanted, add a backend `POST /api/v1/heartbeat` + a `watcher_status` table in P15 (admin views). Out of P13 scope.

2. **Exact contents of `.github/workflows/release.yml` — the OAuth ldflag lines + `latest.json` emission.**
   - What we know: `build_constants.go` documents the four OAuth `-X` ldflags; `check.go` requires `binary_url`+`binary_sha256` in the manifest for the in-app swap.
   - What's unclear: the precise workflow steps (not read this session).
   - Recommendation: the planner reads the release workflow before the build-config task; drop the OAuth ldflags, keep/confirm the bare-binary asset + manifest fields, optionally add `BackendBaseURL`/`Version` stamping.

3. **Should `internal/backend` cache the guild code in memory or re-read wincred per POST?**
   - What we know: wincred reads are cheap but not free; the code is stable for the process lifetime.
   - What's unclear: marginal.
   - Recommendation: read once at watcher start, cache in the `backend.Client`; re-read on a 401 (in case the user re-onboarded). Minor; planner's discretion.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Live backend `api.squirebot.quest` | WATCH-08 upload target; onboarding validation | ✓ | P11 LIVE (Hetzner VPS 5.78.232.85, Caddy TLS) | — (it's the whole point; verified live per STATE.md/memory) |
| GitHub Releases pipeline | WATCH-11 auto-update | ✓ | Existing (`internal/update` + `release.yml`) | — |
| Windows Credential Manager (DPAPI) | WATCH-10 token storage | ✓ | OS-native (wincred v1.2.0) | — |
| Go toolchain (1.25.7) | Build | ✓ | go 1.25.7 (go.mod) | — |
| NSIS (re-package installer, if needed) | Distribution | (?) | per CLAUDE.md NSIS 3.10+ | Auto-update ships the bare binary; a fresh installer is only needed for NEW installs. **User installs missing toolchains themselves** (memory: do not run installers). |

**Missing dependencies with no fallback:** None — the backend is live, the auto-update pipeline exists, DPAPI is OS-native.
**Missing dependencies with fallback:** NSIS (only for re-packaging the installer for *new* installs; existing guildies update via the bare-binary auto-update, which needs no NSIS). If NSIS isn't installed on the dev box, STOP and wait for the user per the toolchain-install memory.

## Security Domain

> `security_enforcement: true` (config). Included.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | Bearer token (guild code) over TLS; backend `ResolveToken` (SHA-256 + `crypto/subtle` constant-time compare) is the SHIPPED guard — the watcher just presents the plaintext code. 401-writes-nothing already proven. |
| V3 Session Management | no | No sessions — opaque static bearer token, no login/session lifecycle on the watcher side. |
| V4 Access Control | yes (server-side) | Character→owner binding is server-derived from the token (first-sighting bind; cross-owner → 409 + audit). The watcher cannot assert identity. |
| V5 Input Validation | yes | Server validates the envelope (required fields, kind enum, 1 MiB body cap). The watcher sends well-formed JSON; the version-gate (FORK 2) adds a min-version check. |
| V6 Cryptography | yes | TLS to `api.squirebot.quest` (Caddy/Let's Encrypt). DPAPI at-rest for the guild code (never hand-rolled crypto on the client). The token plaintext lives only in DPAPI + memory. |
| V7 Errors & Logging | yes | NEVER log the bearer code or raw `content` (mirrors the server's handler discipline + the v1 "defer-zero the refresh-token bytes" practice). |

### Known Threat Patterns for the re-targeted watcher

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Guild code intercepted in transit | Information Disclosure | TLS-only POST to `https://api.squirebot.quest` (HTTP→HTTPS enforced by Caddy); never POST over plain HTTP. The client should refuse a non-HTTPS base URL (or at least default to HTTPS). |
| Guild code stolen from disk | Information Disclosure | DPAPI encryption-at-rest keyed to the user profile (wincred `PersistLocalMachine`); never in `config.json` or logs. |
| Guild code leaked via logs/crash | Information Disclosure | V7 — log char/kind/status/err only; never the `Authorization` value or the code. Mirror the v1 `st.RefreshToken = ""` defer-zero practice for any in-memory copy on a panic path. |
| Bad/revoked code hammering the backend | Denial of Service | 401 is terminal → re-prompt, no retry loop (Pitfall 5). |
| Stale v1 OAuth refresh token left in wincred | Information Disclosure (residual) | WATCH-11 migration deletes the `SquireBot:<email>` entry on first launch. |
| Old/forked watcher uploading against a changed API | Tampering | Version-gate (FORK 2): backend rejects `watcher_version < floor` with a clear 426; the "old watcher refuses to corrupt data" guarantee (SC-5) realized server-side. |
| MITM via a forged base URL (if config-overridable) | Tampering/Spoofing | If `backend_base_url` is config-overridable, pin the scheme to HTTPS and document that overriding it is an advanced/self-host action; the hardcoded default is the canonical host. |
| New outbound authed client → SSRF/egress concerns | (server-side) | N/A on the client (it POSTs to one fixed host). The deleted attack surface (loopback OAuth/picker servers, Drive scope, the baked-in OAuth secret) is a NET REDUCTION — no inbound listener, no browser flow, no Google scope, no public-but-baked secret. |

**Security inputs for the planner's threat_model:** (1) A NEW outbound authed client carrying a bearer token over TLS — threat-model the token at-rest (DPAPI), in-transit (TLS-only), and in-logs (V7). (2) DPAPI at-rest for the guild code — reuse the v1 wincred discipline verbatim. (3) The DELETED attack surface is a security WIN: the loopback OAuth/picker HTTP listeners, the Drive Picker, the `drive.file` scope, the publicly-baked OAuth client secret, and the refresh-token store all go away — P13 SHRINKS the watcher's attack surface. (4) If P13 includes the backend `/whoami` + min-version additions (FORKS 2/3 Option A), threat-model them as ingest-adjacent authed routes reusing the existing `ResolveToken` guard (the patterns from `11-SECURITY.md` T-11.04-* / T-11.05-* apply directly).

## Sources

### Primary (HIGH confidence — verified against source this session)
- `internal/backendsrv/ingest/envelope.go` + `handler.go` — the live ingest contract (Envelope shape, bearer-first ordering, UTF-8 no-double-decode, `watcher_version` field "gated in P13")
- `cmd/squirebot-server/main.go` — the single route `POST /api/v1/ingest`; no validation endpoint
- `internal/app/runapp.go` + `reauth.go` — the v1 watch→parse→write→upsert flow + OAuth/reauth machinery to delete
- `internal/sheet/client.go` + `heartbeat.go` — the sink being replaced + `WatcherMaxSchemaVersion`/heartbeat-to-Sheets
- `internal/auth/store.go` + `pkce.go` — the DPAPI wincred store (survivor) + crypto-shape reference
- `internal/wizard/server.go` + `pages.go` + `folderpicker_dialog.go` — the wizard split (EQ-folder logic preserved, OAuth/picker deleted)
- `internal/config/config.go` — the fields to drop + the "no secret in config" rule + BOM/EQFolder forward-compat
- `internal/update/check.go` + `manifest.go` — auto-update transport (survivor) + the 999.22 SemVer compare
- `internal/watch/watcher.go` + `internal/heartbeat/heartbeat.go` — the survivor pipeline + heartbeat coupling
- `cmd/squirebot/main.go` + `build_constants.go` + `console_windows.go` — main wiring, OAuth ldflags to drop, 999.20/21 nits
- `internal/backendsrv/auth/guard.go` + `store.go` — server-side `ResolveToken` (reuse for `/whoami`) + hash-only-at-rest
- `internal/backendsrv/buildinfo/buildinfo.go` — the `SquireBot/<Version>` UA convention to mirror
- `go.mod` — the full dependency set; the Google tree that drops on deletion
- `.planning/phases/11-backend-foundation-ingest-api/11-CONTEXT.md` (D-03/04/05/07/08) + `11-05-SUMMARY.md` + `11-SECURITY.md` — the locked contract + threat patterns
- `.planning/REQUIREMENTS.md` (WATCH-08/09/10/11) + `.planning/ROADMAP.md` §Phase 13 (5 success criteria) + `.planning/STATE.md` (A1 decision, Open TODOs 999.20/21/22)

### Secondary (MEDIUM confidence)
- `.github/workflows/release.yml` — INFERRED to carry OAuth ldflags + emit `latest.json` (not read this session; A5 — planner should read it)

### Tertiary (LOW confidence)
- None — all load-bearing claims are source-verified or explicitly tagged ASSUMED in the Assumptions Log.

## Metadata

**Confidence breakdown:**
- Deletion map: HIGH — every package read or its imports grepped; LOC counted; call sites enumerated (only 3 non-test importers of the deleted packages).
- `internal/backend` client design: HIGH — the target contract is shipped + test-proven; the client is a straightforward net/http POST mirroring existing in-repo patterns.
- Onboarding fork: HIGH on the options/trade-offs; the recommendation (Option A) is a judgment call surfaced for user decision (sqweek lacks a text-input box — verified — which is the cost of Option A).
- Version-gate fork: HIGH — the envelope field + DB column + handler storage already ship; only the floor-check + (optional) `/whoami` are new, both small + reusing existing guards.
- Pitfalls: HIGH — each tied to a verified source fact (A1 decode split, shared `internal/parse`, locked no-retry-loop invariant).
- Auto-update migration: MEDIUM-HIGH — the config fields + wincred mechanics are verified; the release-workflow specifics (A5) need a planner read.

**Research date:** 2026-05-29
**Valid until:** ~2026-06-28 (30 days — the codebase is stable; the only volatility is if the backend adds routes or the release workflow changes before planning). Re-verify A5 (release.yml) and A1 (route list) at plan time.

## RESEARCH COMPLETE

**Phase:** 13 - Watcher Re-Target + Onboarding
**Confidence:** HIGH

### Key Findings
- **It's a SINK swap on an unchanged pipeline.** `internal/watch` + the re-stat/re-read/CP1252-decode flow survive; only the write target changes from Sheets `batchUpdate` to a new ~150-LOC `internal/backend` POST client hitting the LIVE `POST /api/v1/ingest`. The watcher gets THINNER (sends raw text; the server parses).
- **Deletion map is bounded and verified:** ~8.1k LOC across 5 Google-coupled packages, imported by only 3 non-test files (`cmd/squirebot/main.go`, `app/runapp.go`, `app/reauth.go`). The wizard's EQ-folder logic + the sqweek folder dialog + the wincred DPAPI store are the surgical survivors. **`internal/parse` must NOT be deleted** (the backend imports it).
- **The version-gate "fork" is half-built already:** `Envelope.watcher_version` + `inventory_item.watcher_version` + handler storage all ship today, marked "gated in P13." SC-5 needs only a ~30-LOC backend min-version reject (+ optional ~20-LOC `/whoami` for onboarding validation, since the backend currently serves ONLY the ingest route).
- **Three genuine forks surfaced for one decision round:** (1) PRIMARY — onboarding UX: native dialog (Option A, recommended, zero loopback surface) vs reduced loopback form (Option B, least new code, keeps a port). (2) version-rejection scope: small backend reject (A, faithful to SC-5) vs path-versioning-only (B). (3) validation endpoint: tiny `/whoami` (A) vs abuse-the-ingest-probe (B). FORKS 2+3 bundle into one "~50-LOC backend addition in one redeploy — yes/no?" question.
- **Ride-along nits located:** 999.20 (`console_windows.go` gofmt), 999.21 (`freeConsole()` doc, same file), 999.22 (`update/manifest.go` `IsNewer`/`parseVersion` — no pre-release handling, load-bearing for the P16 coordinated flip). A1 encoding caution confirmed: decode ONCE on the watcher read side, POST UTF-8, server does NOT re-decode.

### File Created
`.planning/phases/13-watcher-re-target-onboarding/13-RESEARCH.md`

### Confidence Assessment
| Area | Level | Reason |
|------|-------|--------|
| Deletion map | HIGH | All packages read/grepped; LOC counted; 3 importers enumerated |
| `internal/backend` design | HIGH | Target contract shipped + test-proven; client mirrors in-repo net/http patterns |
| Onboarding fork | HIGH (options) | Trade-offs verified (sqweek has no text-input box); recommendation surfaced for user |
| Version-gate fork | HIGH | Envelope field + DB column + storage already ship; new code is small + reuses guards |
| Pitfalls | HIGH | Each tied to a verified source fact |
| Auto-update migration | MEDIUM-HIGH | Config + wincred verified; release.yml specifics need a plan-time read (A5) |

### Open Questions (for the decision round / planner)
- Heartbeat fate: recommend DROP for P13 (no backend `_status`; `uploaded_at` is liveness). Confirm or defer a "last seen" signal to P15.
- `.github/workflows/release.yml` exact OAuth ldflags + `latest.json` emission — planner should read before the build-config task (A5).
- The two backend-addition forks (FORKS 2+3) — confirm P13 may include the ~50-LOC backend change (or go watcher-only with weaker SC-5).

### Ready for Planning
Research complete. The orchestrator can run a focused decision round on the 3 forks (FORK 1 primary; FORKS 2+3 bundled), then `/gsd-plan-phase 13` can create PLAN.md files against the deletion map + the `internal/backend` design.
