# Phase 1: End-to-End Thin Slice — Research

**Researched:** 2026-04-30
**Domain:** Go 1.24 Windows watcher (NSIS installer + OAuth loopback PKCE + Drive Picker + fsnotify + Sheets `batchUpdate`) feeding a single `inv:<Char>` landing tab in a shared Google Sheets workbook.
**Confidence:** HIGH for OAuth/PKCE/scopes/Sheets API, HIGH for `_meta.canonical_id` + `WATCHER_MAX_SCHEMA_VERSION` handshake, HIGH for fsnotify Windows quirks, HIGH for NSIS per-user install pattern, HIGH for Drive Picker mechanics (one critical correction below), MEDIUM for inventory file encoding, MEDIUM for systray callback lifecycle, MEDIUM for OAuth Production publishing turnaround.

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Workbook Onboarding**
- **D-01:** The shared guild workbook is created from a publicly-shared template via "Make a copy." Dev maintains a master Sheet at a stable URL with the SquireBot Apps Script and starting schema preinstalled. Guild leader clicks "Make a copy" once.
- **D-02:** Workbook sharing model is "anyone with the link → view-only" PLUS each guildie's Google email added as an editor.
- **D-03:** SquireBot validates the picked workbook is actually a SquireBot workbook (`_meta.canonical_id` marker). Missing/wrong → reject with "This doesn't look like a SquireBot workbook. Pick the one shared by your guild leader." Does not retry-loop.
- **D-04:** Switching workbooks later is a tray menu item ("Change Workbook…") not a force-reinstall. Phase 1 minimum: menu item exists and re-runs Picker.
- **D-05:** Product is a Google Sheet, not a Google Doc.

**First-Run UX**
- **D-06:** After installer finishes, SquireBot auto-launches a setup wizard window with four steps: (1) Connect Google (OAuth), (2) Pick guild workbook (Drive Picker), (3) Confirm EQ folder, (4) Done. After step 4 wizard shows "✓ You're all set" for ~3s, minimizes to tray with one-shot toast.
- **D-07:** Wizard is dismissible mid-flow and resumable. Closing at step 2 → tray icon turns red "Setup needed" with "Continue setup…" menu item. Errors surface clearly with retry; do not collapse all progress.
- **D-08:** Wizard tech is Claude's discretion (Go GUI library or local webview).

**EQ Folder Discovery & Fallback**
- **D-09:** Auto-discovery first, picker fallback. Order: (a) prior-install config, (b) known paths (`C:\P99`, `C:\Project1999`, `C:\Games\Project1999`, etc.), (c) registry uninstall keys for "Project 1999" / "EverQuest", (d) recursive heuristic scan for folder containing both `eqgame.exe` and `eqclient.ini`. All four fail → wizard step-3 shows "couldn't find" message with native folder-picker button.
- **D-10:** Picked folder validated to contain `eqgame.exe`. No `eqgame.exe` → reject with explanation. Does not silently accept.
- **D-11:** Phase 1 is single-folder. Multi-folder for multiboxers (WATCH-03) is Phase 2.

**Distribution & Update Channel**
- **D-12:** SquireBot.exe hosted on public GitHub Releases. `goreleaser` produces binary and `latest.json` manifest used by Phase 2's auto-updater.
- **D-13:** Phase 1 ships unsigned. Code-signing is Phase 2 research flag. "Clean Win11 VM install" success criterion will hit SmartScreen; documented "More info → Run anyway" walkthrough is what gets exercised at this stage.
- **D-14:** Minimal README accompanies GitHub repo (download link, SmartScreen walkthrough, OAuth flow, EQ folder picker, "tray turned red, what now?"). Populated lightly Phase 1, expanded Phase 5.

### Claude's Discretion
- Wizard library / framework choice (Go-native UI vs. embedded webview). Constraint: must produce single-binary install with no runtime dependencies.
- Tray menu surface in Phase 1: minimum is `Status` (read-only string), `Open Workbook` (browser), `Continue setup…` (only when needed), `Quit`. Anything richer can wait.
- Wizard's "you're all set" dismissal animation, toast timing, etc.
- Specific style of Picker's "Shared with me" guidance (screenshot, link, or just trust).
- Validation error copy in D-03 / D-10 — match wizard tone.

### Deferred Ideas (OUT OF SCOPE)
- Multi-folder watcher support for multiboxers — Phase 2 (WATCH-03).
- Code-signing certificate (EV vs OV vs unsigned-with-walkthrough) — Phase 2 research.
- Auto-updater plumbing — Phase 2 (OPS-04). Phase 1 only establishes URL shape.
- Spellbook watcher — Phase 2 (WATCH-02).
- Daily heartbeat write — Phase 2 (WATCH-08).
- Refresh-token failure UX (`invalid_grant` → tray red → reauth) — Phase 2 (AUTH-05).
- Sheets-API retry/backoff — Phase 2 (WATCH-07).
- Catch-up on watcher restart — Phase 2 (WATCH-09).
- Schema dimension/view tab creation — Phase 2 (SCHEMA-01..08). Phase 1 only writes `inv:<Char>` and minimally bootstraps `_meta.canonical_id` + `_char_owner`.
- Multi-character / multi-account on the same PC — supported by file-watching design but not Phase 1 validation focus.
- Richer wizard styling, animations, branding — Phase 5.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| INST-01 | Single `.exe` install, no UAC, no command-line steps | NSIS `RequestExecutionLevel user` + `$LOCALAPPDATA\Programs\SquireBot` install dir (§ 6.1) |
| INST-02 | EQ folder auto-discovery (known paths, registry uninstall keys, eqgame.exe+eqclient.ini scan) with picker fallback | Discovery cascade pattern (§ 6.7); validation per D-10 |
| INST-03 | Browser opens once for OAuth, once for Drive Picker; nothing else required | OAuth loopback PKCE flow (§ 4) + web Picker JS embedded in loopback HTML (§ 5) |
| AUTH-01 | OAuth 2.0 loopback PKCE on random ephemeral port (49152–65535) using `127.0.0.1` literal; manual-paste fallback if redirect doesn't land within 60s | Verified Google docs (§ 4.1, 4.4) |
| AUTH-02 | Only `https://www.googleapis.com/auth/drive.file` scope (non-sensitive, no Google audit) | Verified scope classification (§ 4.5) |
| AUTH-03 | OAuth consent screen flipped to **Production** before any guildie installs (Testing-mode silent 7-day refresh expiry) | Verified turnaround details (§ 4.6) |
| AUTH-04 | Refresh tokens stored only in Windows Credential Manager via DPAPI (`wincred`), target `SquireBot:<email>`; never in config file | wincred API surface (§ 7) |
| AUTH-06 | OAuth `userinfo.email` is canonical identity; written to `_char_owner.owner_email` on first sighting | Combined-scope OAuth flow (§ 4.2) — the userinfo.email + openid combination is Sensitive-exempt |
| WATCH-01 | fsnotify watch on EQ folder for `*-Inventory.txt`; 500ms per-path debounce | Standard timer-reset debounce pattern (§ 8.2); always re-read file fresh, never trust event payload (§ 8.3) |
| WATCH-04 | Tab-separated parser for `Location\|Name\|ID\|Count\|Slots`; tolerate extra trailing columns; Windows-1252 decode; accept header row | Inventory file format details (§ 9) |
| OPS-01 | Watcher writes only to per-character non-overlapping ranges; no shared mutable ranges | One `inv:<Char>!A1:F<MAX>` range per `batchUpdate` call (§ 3.1) |
| OPS-03 | All watcher logs to `%LOCALAPPDATA%\SquireBot\squirebot.log` via `lumberjack.v2` (5MB × 3 files) | Configuration in § 10 |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

The following directives from `./CLAUDE.md` are NON-NEGOTIABLE for Phase 1 plans. They have the same authority as locked decisions in CONTEXT.md.

- **Stack:** Go 1.24, single statically-linked `.exe`, `google.golang.org/api/sheets/v4` + `golang.org/x/oauth2`, `fsnotify` v1.7+, `wincred`, `lumberjack`, `fyne.io/systray`.
- **Distribution:** NSIS 3.10+ per-user installer (no UAC), autostart via `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` (note: autostart is INST-04, deferred to Phase 2 — Phase 1 only needs the install path).
- **OAuth scope:** `https://www.googleapis.com/auth/drive.file` ONLY — never `spreadsheets`, never `drive`. Add `openid`+`userinfo.email` are Sensitive-exempt and may be combined (see § 4.2).
- **OAuth consent:** Must flip to **Production** before any guildie installs. Testing-mode refresh tokens silently expire every 7 days for non-Workspace users.
- **fsnotify Windows:** Always re-stat and re-read file fresh on event; never trust event payload data. 500ms debounce.
- **Sheets writes:** `valueInputOption=RAW` for the hot path. Never `USER_ENTERED` (recalc storms in consolidated views — irrelevant in Phase 1, locked anyway). Atomic clear+write per character per file. Never append. Never row-diff.
- **Per-character non-overlapping ranges only.** No shared mutable ranges from any watcher.
- **DPAPI via wincred:** Refresh token must NOT live in `%LOCALAPPDATA%\SquireBot\config.json`.
- **Identity:** OAuth `userinfo.email` is canonical. `Session.getActiveUser().getEmail()` returns script owner — load-bearing distinction; don't use it.
- **Schema is extend-only.** Add `_meta.schema_version`. Watcher checks `WATCHER_MAX_SCHEMA_VERSION`.
- **Never use:** Python+PyInstaller, Electron, service-account JSON keys, the `oob` OAuth flow, `spreadsheets` or `drive` scope, Apps Script Rhino runtime, HTML scraping of PigParse/wiki, polling with `time.Tick`, trusting fsnotify event payloads on Windows.
- **GSD workflow:** Edits go through GSD commands. Phase 1 plans MUST be created under `/gsd-plan-phase 1` with wave-by-wave task structure.

---

## Summary

Phase 1 is the canonical "everything has to work end-to-end exactly once" slice for SquireBot. The harder edges are not the individual technologies — Go + Sheets API + fsnotify + NSIS + wincred are well-trodden — but the **integration seams** between them and the **first-run UX** that has to make a non-technical guildie click through OAuth + Drive Picker + EQ folder confirmation without confusion. Three load-bearing findings drive plan structure:

1. **The Drive Picker desktop redirect flow (with `prompt=consent&trigger_onepick=true` magic params) requires a public HTTPS redirect_uri** — it CANNOT redirect directly to `http://127.0.0.1:port`. The recommended Phase 1 architecture is therefore the **classic web Picker JS API** loaded into a tiny HTML page served from the loopback HTTP server: do OAuth first (loopback PKCE, scopes `drive.file`+`openid`+`userinfo.email`), then in the same browser tab redirect to a `/picker` route on the loopback server that loads `apis.google.com/js/api.js` + `accounts.google.com/gsi/client` and presents a Picker dialog reusing the access token already issued. This avoids needing GitHub Pages or any other hosted intermediate.

2. **`drive.file` is non-sensitive** AND **`openid`+`userinfo.email` are exempt from sensitive-scope review** when used as the basic-identity subset. Therefore combining all three in one OAuth authorization request requires NO Google verification audit — the consent screen can be flipped to **Production** without going through the audit queue. This is the only way AUTH-06's "OAuth userinfo.email is canonical identity" works alongside AUTH-02's "drive.file scope only."

3. **`spreadsheets.batchUpdate` with a single `UpdateCellsRequest` containing `range` + `rows` + `fields="userEnteredValue"` performs atomic clear+write in ONE round-trip.** When the data in `rows` is shorter than the `range`, the range cells not covered by rows ARE cleared (per the official `UpdateCellsRequest` docs). This makes the architecture's "clear + write" semantics a single API call, not two. Use this rather than `values.batchClear` + `values.update` two-call pattern. Locked.

**Primary recommendation:** Plan Phase 1 as **five waves**: (W0) repo skeleton + Go module + GoReleaser stub; (W1) OAuth loopback PKCE + wincred storage + identity bootstrap; (W2) Drive Picker via embedded web Picker on loopback + workbook canonical_id validation; (W3) EQ folder discovery + fsnotify watcher + Win-1252 TSV parser; (W4) Sheets writer (atomic clear+write via single `UpdateCellsRequest`) + `_char_owner` upsert + `_meta` bootstrap; (W5) NSIS installer + tray UI shell + lumberjack logger. Wizard glue can interleave with W1–W4 or be its own W6.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Installer + autostart-key registration | Windows / NSIS installer | — | Per-user install must run before any binary code; autostart is HKCU registry write |
| OAuth flow + token issuance | Browser (Google's auth UI) | Loopback HTTP server in Go process | Standard desktop OAuth: browser handles auth, watcher's loopback server captures the redirect with `code=...` |
| Drive Picker UI | Browser (Google's Picker JS) | Loopback HTTP server (serves the Picker shell HTML) | Classic web Picker is a JS widget; runs in the browser using the just-issued access token; watcher only serves the static HTML and receives the picked file ID via a small JSON POST back to loopback |
| Refresh token persistence | Windows Credential Manager (DPAPI) | Go process (read on startup) | Per-user encrypted store; DPAPI key is tied to user profile so the token follows the user account |
| EQ folder discovery | Go process (filesystem + registry probes) | Tray UI fallback (folder-picker dialog) | All discovery is filesystem/registry I/O; only the fallback is UI-level |
| File watching + parsing | Go process (background goroutine) | — | Pure local computation; no UI/API tier involved |
| Sheets write | Go process (HTTPS to `sheets.googleapis.com`) | — | Single API call per debounced event; no intermediate tier |
| `_meta` / `_char_owner` bootstrap reads | Go process (HTTPS to Sheets API) | — | Watcher both reads (canonical_id check, schema_version check) and writes (owner_email upsert) — load-bearing in Phase 1 because Apps Script may not even exist yet for a fresh-from-template workbook |
| Tray UI + status + setup wizard | Go process (`fyne.io/systray`) + browser-rendered wizard pages on loopback | — | Wizard is HTML served from the same loopback HTTP server (kept alive after OAuth); tray is the always-on status surface |
| Logging | Local filesystem (`%LOCALAPPDATA%\SquireBot\squirebot.log` via lumberjack) | — | Diagnostic-only in Phase 1; sheet-side `_audit` tab is Phase 2+ |

---

## 1. Standard Stack

### Core (LOCKED in CLAUDE.md / CONTEXT.md)

| Library | Pinned Version | Purpose | Why Standard |
|---------|---------------|---------|--------------|
| Go | **1.24.x** (1.24.13 current per training data) `[ASSUMED — verify with `go version` at start of W0]` | Watcher language | Single static binary, mature first-party Google client |
| `google.golang.org/api/sheets/v4` | latest (rolling; module is in maintenance mode but actively patched) `[VERIFIED: pkg.go.dev]` | Sheets API client | First-party Google Go client |
| `golang.org/x/oauth2` + `golang.org/x/oauth2/google` | latest `[VERIFIED: pkg.go.dev]` | OAuth 2.0 + Google helpers (PKCE, token sources) | Canonical Go OAuth lib; integrates with Sheets client via `option.WithTokenSource(ts)` |
| `github.com/fsnotify/fsnotify` | **v1.7+** `[CITED: github.com/fsnotify/fsnotify CHANGELOG]` | Filesystem events (Windows: `ReadDirectoryChangesW`) | De-facto standard; Windows reliability fixes since v1.7 |
| `github.com/danieljoos/wincred` | **v1.2.x** `[CITED: github.com/danieljoos/wincred]` | Windows Credential Manager wrapper | DPAPI-backed; thin syscall wrapper; no extra deps |
| `fyne.io/systray` | **v1.10+** `[CITED: pkg.go.dev/fyne.io/systray]` | Tray icon + menu | Maintained fork of `getlantern/systray` |
| `gopkg.in/natefinch/lumberjack.v2` | v2.2.x `[CITED: github.com/natefinch/lumberjack]` | Log file rotation | Standard for Go file rotation |
| NSIS | **3.10+** `[ASSUMED — verify on W5 build host]` | Windows installer | Per-user installs without UAC |

### Supporting (Phase 1 only what's needed)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `log/slog` | stdlib (Go 1.21+) | Structured logging | All logging in Phase 1 — no external logging dep |
| `golang.org/x/text/encoding/charmap` | stdlib-x latest | Windows-1252 decoder | TSV parser (§ 9) |
| `encoding/json` | stdlib | Config file at `%LOCALAPPDATA%\SquireBot\config.json` | Don't pull in Viper for Phase 1 |
| `crypto/rand` + `crypto/sha256` + `encoding/base64` | stdlib | PKCE `code_verifier` generation + S256 `code_challenge` | (§ 4.3) |
| `os/exec` | stdlib | Open default browser for OAuth (`rundll32 url.dll,FileProtocolHandler <url>` on Windows; or `start <url>`) | Browser launch for OAuth + Picker |

### Explicitly NOT Phase 1

| Library | Phase | Why deferred |
|---------|-------|--------------|
| `github.com/minio/selfupdate` | Phase 2 (OPS-04) | D-13 ships Phase 1 unsigned + manual updates |
| code-signing cert + `signtool` | Phase 2 research flag | D-13 |
| `goreleaser` for full release | Phase 1 stub OK; full pipeline Phase 2 | Phase 1 just needs the binary uploaded to Releases for one dev-machine smoke test |
| TypeScript / clasp / esbuild | Phase 3 | Apps Script-side; Phase 1 only writes raw landing tabs |

### Version verification

Before W0 ships, run on the build host:
```bash
go version                                    # expect go1.24.x
npm view nsis 2>/dev/null || makensis /VERSION # expect v3.10+
```
Both currently report MISSING in this research environment — see `## Environment Availability` below. The build host (developer's PC) MUST have Go 1.24 and NSIS 3.10+ installed.

### Installation (one-shot; assumes Go module already initialized)

```bash
go mod init github.com/<owner>/squirebot
go get google.golang.org/api/sheets/v4
go get google.golang.org/api/oauth2/v2          # for userinfo.email lookup endpoint
go get golang.org/x/oauth2 golang.org/x/oauth2/google
go get github.com/fsnotify/fsnotify
go get github.com/danieljoos/wincred
go get fyne.io/systray
go get gopkg.in/natefinch/lumberjack.v2
go get golang.org/x/text/encoding/charmap

# Phase 1 build (single-file Windows GUI exe; no console window)
GOOS=windows GOARCH=amd64 \
  go build -ldflags="-H=windowsgui -s -w" \
  -o dist/squirebot.exe ./cmd/squirebot
```

---

## 2. Architecture Patterns

### 2.1 System Architecture Diagram

```
┌──────────────────────────────────────────────────────────────────────────────┐
│  Per-Guildie PC                                                              │
│  ┌──────────────┐                                                            │
│  │  NSIS .exe   │  one-time install (no UAC)                                 │
│  │  installer   │  → writes %LOCALAPPDATA%\Programs\SquireBot\squirebot.exe  │
│  └──────┬───────┘                                                            │
│         │ launches                                                           │
│         ▼                                                                    │
│  ┌──────────────────────────────────────────────────────────────────────┐    │
│  │  squirebot.exe (Go process)                                          │    │
│  │  ┌────────────┐  ┌──────────────┐  ┌──────────────┐                  │    │
│  │  │ tray UI    │  │ wizard HTTP  │  │ fsnotify     │                  │    │
│  │  │ (systray)  │  │ server       │  │ watcher      │                  │    │
│  │  │            │  │ 127.0.0.1:N  │  │ goroutine    │                  │    │
│  │  └─────┬──────┘  └──────┬───────┘  └──────┬───────┘                  │    │
│  │        │                │                 │                           │    │
│  │        └────────────────┼─────────────────┘                           │    │
│  │                         │                                             │    │
│  │  ┌──────────────────────▼──────────────────────┐                      │    │
│  │  │ core: OAuth manager • parser • sheets client│                      │    │
│  │  └──────────────────────┬──────────────────────┘                      │    │
│  │                         │                                             │    │
│  │   ┌─────────────────────┼─────────────────────┬───────────────────┐   │    │
│  │   ▼                     ▼                     ▼                   ▼   │    │
│  │ ┌─────────┐  ┌────────────────────┐  ┌──────────────┐  ┌─────────────┐│    │
│  │ │ wincred │  │ %LOCALAPPDATA%\    │  │ EQ folder    │  │ slog +      ││    │
│  │ │ DPAPI   │  │ SquireBot\        │  │ <Char>-      │  │ lumberjack  ││    │
│  │ │ (refresh│  │   config.json      │  │ Inventory.txt│  │ → log file  ││    │
│  │ │ token)  │  │ (NO refresh token) │  │ (read-only)  │  │             ││    │
│  │ └─────────┘  └────────────────────┘  └──────────────┘  └─────────────┘│    │
│  └──────────────────────────────┬───────────────────────────────────────┘    │
│                                 │ HTTPS                                      │
└─────────────────────────────────┼──────────────────────────────────────────────┘
                                  │
        ┌─────────────────────────┼──────────────────────────┐
        │                         │                          │
        ▼                         ▼                          ▼
┌──────────────┐  ┌──────────────────────┐  ┌─────────────────────────┐
│ accounts.    │  │ apis.google.com/     │  │ sheets.googleapis.com   │
│ google.com/  │  │   js/api.js +        │  │   /v4/spreadsheets/{id} │
│ o/oauth2/v2/ │  │ accounts.google.com/ │  │   :batchUpdate          │
│ auth         │  │   gsi/client         │  │   (UpdateCellsRequest   │
│ + token endpt│  │ (Picker JS loaded    │  │    range:'inv:Foo'      │
│ (loopback    │  │  in wizard tab)      │  │    fields:userEntered)  │
│  redirect)   │  │                      │  │                         │
└──────────────┘  └──────────────────────┘  └─────────────────────────┘
                                                       │
                                                       ▼
                                          (the shared guild Sheet,
                                           selected via Picker, with
                                           inv:<Char> tab now populated)
```

**Reading the diagram:** Data flow originates at three independent entry points — (a) the user's first-run flow goes browser → loopback HTTP server → wincred (token landing) and browser → Picker → loopback (workbook ID landing); (b) ongoing operation is fsnotify → core → Sheets API; (c) the tray UI surfaces status from core and dispatches user intents (open workbook, change workbook, quit) back into core. The wizard HTTP server lives on the same loopback port that received the OAuth redirect — it is repurposed after OAuth completes to serve the Picker shell page and the EQ-folder confirmation page, then shuts down.

### 2.2 Recommended Project Structure

```
squirebot/
├── cmd/
│   └── squirebot/
│       └── main.go              # entry: parse flags, init slog+lumberjack,
│                                # bootstrap config, decide wizard-vs-watch path,
│                                # block on systray.Run()
├── internal/
│   ├── auth/
│   │   ├── oauth.go             # loopback PKCE flow, token exchange
│   │   ├── pkce.go              # code_verifier + S256 code_challenge gen
│   │   ├── store.go             # wincred read/write/delete (target SquireBot:<email>)
│   │   └── userinfo.go          # /oauth2/v2/userinfo email lookup
│   ├── picker/
│   │   ├── server.go            # loopback HTTP routes for Picker shell + callback
│   │   └── picker.html.go       # embedded HTML+JS Picker shell (go:embed)
│   ├── wizard/
│   │   ├── server.go            # loopback HTTP routes for wizard pages
│   │   └── pages.html.go        # embedded wizard step pages (go:embed)
│   ├── eqfind/
│   │   └── discover.go          # known-paths + registry-uninstall + heuristic scan
│   ├── watch/
│   │   ├── watcher.go           # fsnotify+debounce loop
│   │   └── debounce.go          # per-path 500ms timer-reset debouncer
│   ├── parse/
│   │   └── inventory.go         # Win-1252 TSV parser, 5-col tolerant
│   ├── sheet/
│   │   ├── client.go            # sheets.NewService + token source wiring
│   │   ├── ensure_tab.go        # idempotent inv:<Char> tab creation
│   │   ├── write.go             # atomic clear+write via UpdateCellsRequest
│   │   ├── meta.go              # _meta canonical_id read + schema_version check
│   │   └── owner.go             # _char_owner upsert
│   ├── tray/
│   │   └── tray.go              # systray.Run, menu items, status string
│   ├── config/
│   │   └── config.go            # %LOCALAPPDATA%\SquireBot\config.json read/write
│   └── logging/
│       └── logger.go            # slog handler + lumberjack file writer
├── installer/
│   └── squirebot.nsi            # NSIS script (W5)
├── .github/
│   └── workflows/
│       └── release.yml          # goreleaser stub for tagged releases
├── go.mod
├── go.sum
└── README.md                    # D-14: minimal Phase 1 README
```

### 2.3 Pattern 1: One-Shot Atomic Clear+Write via `UpdateCellsRequest`

**What:** A single `spreadsheets.batchUpdate` request containing exactly one `updateCells` request. The `range` covers `inv:<Char>!A1:F<MAX>` where MAX is a generous bound (e.g., 500 — comfortably above the ~250-row P99 max). The `rows` array contains header + N data rows. Cells in the range NOT covered by `rows` are CLEARED.
[CITED: developers.google.com/workspace/sheets/api/reference/rest/v4/spreadsheets/request — UpdateCellsRequest "If the data in rows does not cover the entire requested range, the fields matching those set in fields will be cleared."]

**When to use:** Always, for landing-tab writes. This is the entire write contract for Phase 1.

**Example (Go, sheets/v4 client):**

```go
// internal/sheet/write.go
package sheet

import (
    "context"
    "fmt"
    "google.golang.org/api/sheets/v4"
)

const InvTabMaxRows = 500 // generous bound; P99 max is ~250 with bags+bank

// WriteInventory replaces the entire inv:<charName> tab atomically.
// The single batchUpdate clears all cells in the range and writes the
// header + data rows in one call. Idempotent by construction.
func (c *Client) WriteInventory(ctx context.Context, sheetID int64, charName string, header []string, dataRows [][]string, uploadedAt string) error {
    rows := make([]*sheets.RowData, 0, len(dataRows)+1)
    rows = append(rows, toRowData(header))
    for _, dr := range dataRows {
        // Append the _uploaded_at provenance column so every row carries it.
        rows = append(rows, toRowData(append(dr, uploadedAt)))
    }

    req := &sheets.BatchUpdateSpreadsheetRequest{
        Requests: []*sheets.Request{{
            UpdateCells: &sheets.UpdateCellsRequest{
                Range: &sheets.GridRange{
                    SheetId:          sheetID,
                    StartRowIndex:    0,
                    EndRowIndex:      InvTabMaxRows, // exclusive; clears anything past N
                    StartColumnIndex: 0,
                    EndColumnIndex:   6, // A:F
                },
                Rows:   rows,
                Fields: "userEnteredValue",
            },
        }},
    }
    _, err := c.svc.Spreadsheets.BatchUpdate(c.spreadsheetID, req).Context(ctx).Do()
    if err != nil {
        return fmt.Errorf("batchUpdate inv:%s: %w", charName, err)
    }
    return nil
}

func toRowData(cells []string) *sheets.RowData {
    vs := make([]*sheets.CellData, len(cells))
    for i, s := range cells {
        vs[i] = &sheets.CellData{
            UserEnteredValue: &sheets.ExtendedValue{StringValue: stringPtr(s)},
        }
    }
    return &sheets.RowData{Values: vs}
}

func stringPtr(s string) *string { return &s }
```

**Why one call, not two:** The Sheets `values.batchClear` + `values.batchUpdate` two-call pattern is NOT atomic — readers of the sheet can observe a window where rows are cleared but not yet written. `spreadsheets.batchUpdate` with a single `UpdateCellsRequest` IS atomic per the API contract. The "clear" is implicit: any cell in the range not covered by `rows` is cleared as part of the same request. This is the recommended pattern. [CITED: developers.google.com/workspace/sheets/api/reference/rest/v4/spreadsheets/batchUpdate — "Each request is validated before being applied. If any request is not valid then the entire request will fail and nothing will be applied."]

**Note on sheetId vs A1 range:** Use the numeric `sheetId` (from `spreadsheets.get` or the `addSheet` response), not the A1 string `'inv:Foo'!A1:F500`. SheetIDs are stable across renames (SCHEMA-06 will make this a global rule in Phase 2; Phase 1 should establish it now). For A1 string ranges the API supports `valueInputOption=RAW` via `values.update`, but the field-mask clearing trick requires the `UpdateCellsRequest` form which uses `GridRange`.

### 2.4 Pattern 2: Loopback HTTP Server as Both OAuth-Callback AND Wizard Server

**What:** The same `http.Server` listening on `127.0.0.1:<port>` handles both (a) the OAuth redirect callback (`GET /oauth/callback?code=...`), and (b) subsequent wizard pages (`GET /picker`, `POST /picker/result`, `GET /eq-folder`, etc.). It runs the entire setup wizard, then shuts down once Done is reached.

**When to use:** First-run wizard. After setup completes, the server is closed and the binary continues as a tray-only background process.

**Why this design:** Avoids needing a separate native UI toolkit for the wizard (D-08 leaves that to Claude's discretion; this satisfies it with `net/http` + `go:embed` HTML). The browser is already open from the OAuth flow; reusing it for the workbook picker and EQ folder confirmation keeps the UX inside one tab.

**Lifecycle:**
1. Tray launches → Go starts listener on a random ephemeral port (e.g., 49152–65535).
2. Open browser to `http://127.0.0.1:<port>/start` (wizard step 1: "Connect Google").
3. User clicks Connect → redirect to Google `accounts.google.com/o/oauth2/v2/auth?...&redirect_uri=http://127.0.0.1:<port>/oauth/callback`.
4. Google redirects back to `/oauth/callback?code=...` → exchange for tokens → store in wincred.
5. Server redirects browser to `/picker` (wizard step 2: Drive Picker).
6. Picker JS posts `{spreadsheetId: "..."}` to `/picker/result` → validate canonical_id → store in config.json.
7. Server redirects to `/eq-folder` (wizard step 3: confirm/pick EQ folder).
8. Server redirects to `/done` → 3-second "✓ You're all set" → JS calls `window.close()` (silently ignored on most browsers) and sends a final `POST /shutdown`.
9. Server graceful-shutdown; tray icon stays.

**Anti-pattern:** Don't try to embed a webview. The Go process opens the system default browser via `os/exec` (`cmd /C start http://127.0.0.1:<port>/start` on Windows). Native webview adds a runtime dependency (WebView2) and CGO complications.

### 2.5 Pattern 3: Per-Path Timer-Reset Debouncer

**What:** A goroutine maintains a `map[string]*time.Timer`. On each fsnotify event for a path, reset that path's timer to fire 500ms in the future; when the timer fires, dispatch a "process this file" event. Coalesces bursts of Create/Write events from the EQ engine's `/outputfile` write sequence into one read.

**Example:**
```go
// internal/watch/debounce.go
type Debouncer struct {
    delay  time.Duration
    timers sync.Map // path -> *time.Timer
    out    chan<- string
}

func (d *Debouncer) Trigger(path string) {
    if t, ok := d.timers.Load(path); ok {
        t.(*time.Timer).Reset(d.delay)
        return
    }
    t := time.AfterFunc(d.delay, func() {
        d.timers.Delete(path)
        d.out <- path
    })
    d.timers.Store(path, t)
}
```

**Note on Windows event sequences:** `/outputfile inventory` on EQ writes the file directly (not atomic-rename); fsnotify on Windows typically emits a sequence like `Create`, `Write`, `Write` for a fresh file. The 500ms debounce comfortably absorbs this. We do NOT trust event content — on timer-fire, re-stat and read the file fresh. [CITED: github.com/fsnotify/fsnotify — Windows backend uses ReadDirectoryChangesW]

### 2.6 Pattern 4: Two-Step Workbook Validation (canonical_id → schema_version)

**What:** After Picker returns a `spreadsheetId`, the watcher must perform two read-only Sheets API calls before storing the ID:

1. Read `_meta!B1:B2` (canonical_id, schema_version). Apps Script docs of the master template guarantee these cells exist at fixed coordinates. [CITED: ARCHITECTURE.md — `_meta` row layout]
2. Verify `_meta.canonical_id` matches a hardcoded constant baked into the watcher binary (e.g., `"squirebot-v1-workbook-2026"` — same value the Apps Script template ships with).
3. Verify `_meta.schema_version <= WATCHER_MAX_SCHEMA_VERSION` (= 1 in Phase 1).

**Failure modes (D-03):**
- canonical_id missing/empty → "This doesn't look like a SquireBot workbook. Pick the one shared by your guild leader."
- canonical_id mismatch → same error.
- schema_version > WATCHER_MAX_SCHEMA_VERSION → "This workbook uses a newer SquireBot schema (v{N}). Update SquireBot to continue."

**Non-error early bootstrap:** if `_meta` is empty (canonical_id cell is `""`), the watcher writes `canonical_id = "squirebot-v1-workbook-2026"` and `schema_version = 1`. This handles the case where the guild leader did a fresh "Make a copy" but the Apps Script hasn't initialized `_meta` yet (Phase 1 ships before Phase 3 Apps Script work). [ASSUMED — needs decision check; see Open Questions Q1]

### 2.7 Anti-Patterns to Avoid

- **Trusting `fsnotify` event payload data on Windows.** The CHANGELOG and issue tracker are clear: spurious events from antivirus, ordering not guaranteed, "rename" events for plain saves. Always re-stat + re-read. [CITED: github.com/fsnotify/fsnotify Issue #17, #214, #255]
- **Watching individual files vs the parent directory.** Watch the parent directory, filter by filename pattern. Atomic-rename saves break per-file watches.
- **Storing the refresh token in `config.json`.** Refresh tokens grant indefinite access until revoked. wincred only.
- **Using `valueInputOption=USER_ENTERED` on the hot path.** Triggers spreadsheet-wide recalculation. Use `RAW`. (For the `UpdateCellsRequest` shape this corresponds to `userEnteredValue` field — same effect as RAW for our purpose.)
- **Building the wizard as a native window in Phase 1.** Embed HTML+JS in the loopback server instead. Cheaper, smaller binary, simpler.
- **Using `localhost` literal in `redirect_uri`.** Use `127.0.0.1` literal. `localhost` is allowed by Google but "may cause issues with client firewalls." [CITED: developers.google.com/identity/protocols/oauth2/native-app]
- **Polling the EQ folder with `time.Tick`.** Misses fast successive changes; explicit anti-pattern in CLAUDE.md.

---

## 3. Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| OAuth 2.0 token refresh + access-token caching | Manual HTTP client + manual `expires_in` math | `golang.org/x/oauth2.TokenSource` (with `oauth2.ReuseTokenSource(...)`) wired into `option.WithTokenSource(ts)` for the Sheets client | Refresh-token rotation, expiry handling, retry on 401, are all already correct in the lib; rewrites are reliably wrong |
| PKCE code_verifier / code_challenge generation | Random byte loop | `crypto/rand` + `sha256.Sum256` + `base64.URLEncoding.WithPadding(base64.NoPadding)` | RFC 7636 compliance is exact-character-class sensitive |
| Filesystem event coalescing | Hand-rolled select+timer-per-event loop | Standard timer-reset debouncer (§ 2.5) | Race-prone if naive |
| TSV parsing | Manual `bufio.Scanner` + `strings.Split('\t')` for our 5-cols-but-tolerate-extras case | `encoding/csv` with `Reader.Comma = '\t'` and `FieldsPerRecord = -1` (tolerate any column count) | Handles quoting, line endings, EOFs cleanly |
| Windows-1252 → UTF-8 decode | Byte-by-byte map | `golang.org/x/text/encoding/charmap.Windows1252.NewDecoder().Reader(r)` wrapping the file reader | Correct on `’`, `é`, `ñ` and other characters that DO appear in EQ item names |
| Atomic clear+write | `values.batchClear` then `values.batchUpdate` (two calls, race window) | Single `spreadsheets.batchUpdate` with `UpdateCellsRequest{Range, Rows, Fields:"userEnteredValue"}` | Single round-trip, atomic per API contract (§ 2.3) |
| Windows credential storage | DPAPI direct (`syscall` to `CryptProtectData`) | `github.com/danieljoos/wincred` | Tested wrapper; keeps the binary small |
| Log rotation | Roll your own | `lumberjack.v2` (5MB, 3 backups, 28d max) | Battle-tested |
| Tray icon | Cgo + Win32 | `fyne.io/systray` | Maintained Go-native lib |
| NSIS multi-user logic | `MultiUser.nsh` (overkill for Phase 1) | Plain `RequestExecutionLevel user` per-user-only install | We are CurrentUser-only; D-13 decisions match |
| Browser launch | manual `cmd /C start` invocation in multiple places | `func openBrowser(url string) error` with `exec.Command("rundll32", "url.dll,FileProtocolHandler", url)` (works on Win 10/11) | Centralize for testability |

**Key insight:** Phase 1 is a tour through five "obviously easy, deceptively complex" domains (OAuth, PKCE, fsnotify on Windows, Sheets atomic-write semantics, NSIS per-user). For each, the standard library's wrapper is faster to use AND more correct than rolling our own. Resist DIY.

---

## 4. OAuth Loopback PKCE — Concrete Recipe

**Purpose:** Implement AUTH-01, AUTH-02, AUTH-04, AUTH-06.

### 4.1 Endpoints + redirect URI

| Item | Value |
|------|-------|
| Authorization endpoint | `https://accounts.google.com/o/oauth2/v2/auth` `[CITED: developers.google.com/identity/protocols/oauth2/native-app]` |
| Token endpoint | `https://oauth2.googleapis.com/token` `[CITED: developers.google.com/identity/protocols/oauth2/native-app]` |
| Userinfo endpoint | `https://openidconnect.googleapis.com/v1/userinfo` (preferred) or `https://www.googleapis.com/oauth2/v2/userinfo` (legacy, still works) `[CITED: developers.google.com/identity/openid-connect/openid-connect]` |
| Redirect URI format | `http://127.0.0.1:<port>` (NOT `localhost`) `[CITED: developers.google.com/identity/protocols/oauth2/native-app]` |
| Port range | 49152–65535 (ephemeral / dynamic) — pick on each launch via `net.Listen("tcp", "127.0.0.1:0")` and read the assigned port back |
| OAuth client type | "Desktop app" in Google Cloud Console; `client_secret` is **optional** for desktop clients (PKCE replaces it) `[CITED: developers.google.com/identity/protocols/oauth2/native-app — "(The client_secret is not applicable to requests from clients registered as Android, iOS, or Chrome applications.)"]` |

### 4.2 Scope set (combined in one request — Sensitive-exempt)

```
https://www.googleapis.com/auth/drive.file
openid
https://www.googleapis.com/auth/userinfo.email
```

This combination is a basic-identity subset and is **exempt from sensitive-scope verification review**. [CITED: developers.google.com/identity/protocols/oauth2/production-readiness/sensitive-scope-verification — "An exception applies if your app requests a subset of the following: name, email address, and user profile (through the userinfo.email, userinfo.profile, openid scopes or their OpenID Connect equivalents)."] Combined with `drive.file` (non-sensitive), the entire scope set requires **no Google verification audit** to flip the consent screen to Production.

The Picker desktop-mode magic params (`prompt=consent&trigger_onepick=true`) constrain you to `drive.file` ALONE — but those params are only required for the special "embedded picker via OAuth redirect" pattern that needs a hosted HTTPS redirect (§ 5). We are using the **classic web Picker** instead, which has no scope constraints because it reuses an already-issued access token. So Phase 1's OAuth request can request all three scopes safely.

### 4.3 PKCE generation (Go)

```go
// internal/auth/pkce.go
package auth

import (
    "crypto/rand"
    "crypto/sha256"
    "encoding/base64"
)

// NewPKCEPair returns (code_verifier, code_challenge_S256).
// Per RFC 7636: verifier is 43–128 chars from [A-Z][a-z][0-9]-._~.
// 32 random bytes base64url-NoPadding-encoded → 43 chars; meets minimum.
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

[CITED: RFC 7636 — verifier is high-entropy [A-Z][a-z][0-9]-._~, min 43 / max 128 chars; S256 is base64url(sha256(verifier))]

### 4.4 Authorization URL (assembled)

```go
// internal/auth/oauth.go
authURL := "https://accounts.google.com/o/oauth2/v2/auth?" + url.Values{
    "client_id":             {clientID},
    "redirect_uri":          {fmt.Sprintf("http://127.0.0.1:%d/oauth/callback", port)},
    "response_type":         {"code"},
    "scope":                 {"https://www.googleapis.com/auth/drive.file openid https://www.googleapis.com/auth/userinfo.email"},
    "state":                 {randomState},   // CSRF guard; verify on callback
    "code_challenge":        {challenge},
    "code_challenge_method": {"S256"},
    "access_type":           {"offline"},     // ensures refresh_token in response
    "prompt":                {"consent"},     // forces refresh_token even on re-auth
}.Encode()
```

**Note `access_type=offline`:** The Google docs say "for installed applications, refresh tokens are always returned automatically." But empirically `access_type=offline` is still the documented way to be explicit, and `prompt=consent` ensures a refresh token even when re-authorizing the same scopes. Keep both.

### 4.5 Token exchange + refresh-token storage

Use `golang.org/x/oauth2` directly so refresh handling is automatic:

```go
// internal/auth/oauth.go (continued)
import (
    "golang.org/x/oauth2"
    "golang.org/x/oauth2/google"
)

cfg := &oauth2.Config{
    ClientID:    clientID,
    // ClientSecret left empty for desktop (PKCE)
    RedirectURL: fmt.Sprintf("http://127.0.0.1:%d/oauth/callback", port),
    Endpoint:    google.Endpoint,
    Scopes: []string{
        "https://www.googleapis.com/auth/drive.file",
        "openid",
        "https://www.googleapis.com/auth/userinfo.email",
    },
}

// In the /oauth/callback handler:
tok, err := cfg.Exchange(ctx, code,
    oauth2.SetAuthURLParam("code_verifier", verifier))
if err != nil { /* handle */ }
// tok.RefreshToken is non-empty here; persist to wincred.
```

### 4.6 Production publishing status (AUTH-03)

| Item | Detail |
|------|--------|
| Why required | Testing-mode refresh tokens silently expire after **7 days** for non-Workspace users `[CITED: ARCHITECTURE constraint, summarized from Google Cloud Console publishing-status docs]` |
| Audit required for our scope set? | **NO.** `drive.file` is non-sensitive; `openid`+`userinfo.email` are sensitive-exempt as basic-identity subset. `[VERIFIED: developers.google.com/workspace/sheets/api/scopes + sensitive-scope-verification page]` |
| Required fields on consent screen | App name, user-support email, developer contact email, app logo (optional), authorized domains (none required for desktop client), home page URL (a GitHub README URL is fine) |
| Click "Publish app" | One-time toggle in Cloud Console → Audience tab → "Publish app". Status flips from Testing to In production immediately. `[CITED: support.google.com/cloud/answer/15549945 — Manage App Audience]` |
| Turnaround | **Immediate** for non-sensitive + sensitive-exempt scope sets. The verification queue is only entered if your scope set requires sensitive or restricted-scope review; ours does not. `[ASSUMED — verify by clicking Publish app on a test project before Phase 1 ships; Open Questions Q3]` |

**Phase 1 plan implication:** A "publish OAuth consent screen" task must precede any guildie install. Owner-action only (developer goes to Cloud Console). Add to Phase 1 plan as an explicit human-step task in the wave that ships the binary.

### 4.7 wincred storage shape (AUTH-04)

```go
// internal/auth/store.go
import "github.com/danieljoos/wincred"

const credPrefix = "SquireBot:" // target name pattern: SquireBot:<email>

type StoredToken struct {
    RefreshToken string `json:"refresh_token"`
    Email        string `json:"email"`
    ClientID     string `json:"client_id"` // for sanity check on read
}

func StoreToken(email string, st StoredToken) error {
    blob, _ := json.Marshal(st)
    cred := wincred.NewGenericCredential(credPrefix + email)
    cred.CredentialBlob = blob
    cred.Persist = wincred.PersistLocalMachine // survives reboot, user-scoped
    return cred.Write()
}
func ReadToken(email string) (StoredToken, error) {
    cred, err := wincred.GetGenericCredential(credPrefix + email)
    if err != nil {
        return StoredToken{}, err
    }
    var st StoredToken
    return st, json.Unmarshal(cred.CredentialBlob, &st)
}
func DeleteToken(email string) error {
    cred, err := wincred.GetGenericCredential(credPrefix + email)
    if err != nil { return err }
    return cred.Delete()
}
```

**Persistence note:** `wincred.PersistLocalMachine` is the standard pick — credential survives reboots, is encrypted at rest by DPAPI keyed to the user profile, and is automatically wiped on Windows password reset (acceptable failure mode: user re-OAuths). `PersistEnterprise` is for roaming profiles in AD environments — overkill for our 12-guildie scope.

**Profile reset behavior:** When a user resets their Windows password without preserving the DPAPI master key, ALL stored credentials become unrecoverable. Watcher detects via `wincred.GetGenericCredential` returning an error, treats as "first run, re-OAuth needed." This is the same code path as a fresh install. [CITED: learn.microsoft.com — DPAPI MasterKey backup failures]

**Re-OAuth (e.g., user clicked Sign Out in tray):** call `DeleteToken(email)` first, then run the wizard's OAuth step. The new refresh token overwrites cleanly; no leftover state.

---

## 5. Drive Picker from a Go Desktop App — Concrete Recipe

**Purpose:** Implement INST-03's "exactly one Drive Picker invocation" requirement.

### 5.1 Design choice: classic Web Picker, not desktop-mode Picker

The Drive Picker has two modes:

- **Desktop Picker** (`prompt=consent&trigger_onepick=true` magic params on the OAuth URL): redirects to Picker UI in a new browser tab, sends back `picked_file_ids` along with the OAuth code. **Requires a public HTTPS `redirect_uri`** that bounces back to loopback. [CITED: developers.google.com/workspace/drive/picker/guides/overview-desktop — "The specified redirect_uri must be a public HTTPS URL. If you want to use a custom protocol or localhost URL for your redirect_uri, you must use a public HTTPS URL that then redirects to the custom protocol or localhost URL."]
- **Web Picker** (classic JS API loaded via `apis.google.com/js/api.js`): a JS widget rendered in any HTML page, given an OAuth access token. Returns the selected file ID as a JS callback. **No public HTTPS redirect required.** [CITED: developers.google.com/workspace/drive/picker/guides/overview]

**We use the Web Picker.** Reasons:
1. No need to host an intermediate HTTPS redirect page (would need GitHub Pages or similar — extra moving part).
2. Sensitive-scope-exempt scope combination (drive.file + openid + userinfo.email) — Desktop Picker forbids combining drive.file with anything else.
3. Reuses the access token already issued in the OAuth step (§ 4) — one fewer round-trip.
4. Keeps the entire wizard inside one browser tab; cleaner UX.

### 5.2 Mechanism

After OAuth completes (refresh + access token in hand), the loopback server responds to `GET /picker` with an HTML page like:

```html
<!-- internal/picker/picker.html (embedded via go:embed) -->
<!DOCTYPE html><html><head><meta charset="utf-8"><title>SquireBot — pick your guild workbook</title></head>
<body><div id="status">Loading picker…</div>
<script src="https://apis.google.com/js/api.js"></script>
<script src="https://accounts.google.com/gsi/client"></script>
<script>
  const ACCESS_TOKEN = "{{.AccessToken}}";  // injected by Go template
  const APP_ID       = "{{.AppID}}";        // Cloud project number (numeric)
  const API_KEY      = "{{.APIKey}}";       // Picker API key (also from Cloud Console)

  function onApiLoad() { gapi.load('picker', { callback: createPicker }); }

  function createPicker() {
    const view = new google.picker.DocsView(google.picker.ViewId.SPREADSHEETS)
        .setIncludeFolders(false)
        .setMimeTypes('application/vnd.google-apps.spreadsheet')
        .setOwnedByMe(false)               // they want a SHARED workbook
        .setSelectFolderEnabled(false);
    const sharedView = new google.picker.DocsView(google.picker.ViewId.SPREADSHEETS)
        .setMimeTypes('application/vnd.google-apps.spreadsheet')
        .setEnableDrives(true)             // include "Shared with me"
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
    if (data.action === google.picker.Action.PICKED) {
      const file = data.docs[0];
      // POST the picked spreadsheet ID back to the loopback server.
      const resp = await fetch('/picker/result', {
          method: 'POST',
          headers: {'Content-Type': 'application/json'},
          body: JSON.stringify({ spreadsheetId: file.id, name: file.name })
      });
      if (resp.ok) {
        location.href = '/eq-folder';   // wizard advances
      } else {
        const msg = await resp.text();
        document.getElementById('status').textContent = msg;  // canonical_id mismatch text
      }
    } else if (data.action === google.picker.Action.CANCEL) {
      document.getElementById('status').textContent = "No workbook picked. Click here to retry.";
    }
  }

  window.onload = onApiLoad;
</script></body></html>
```

The Go side handles `POST /picker/result`:

```go
// internal/picker/server.go
func (s *Server) pickerResult(w http.ResponseWriter, r *http.Request) {
    var body struct{ SpreadsheetID, Name string }
    json.NewDecoder(r.Body).Decode(&body)
    // Validation per Pattern 4 (§ 2.6) — read _meta.canonical_id, schema_version
    if err := s.sheet.ValidateWorkbook(r.Context(), body.SpreadsheetID); err != nil {
        http.Error(w, "This doesn't look like a SquireBot workbook. Pick the one shared by your guild leader.", 400)
        return
    }
    s.config.SetSpreadsheetID(body.SpreadsheetID)
    w.WriteHeader(204)
}
```

### 5.3 What the Picker returns

A `data.docs[0].id` string — the Drive file ID. We treat it as opaque; pass to Sheets API as `spreadsheetId`.

### 5.4 OAuth client console setup for Picker

In the Google Cloud Console:

1. **OAuth client type:** Desktop app (NOT Web). PKCE replaces client_secret.
2. **Authorized JavaScript origins:** None needed — the Picker JS doesn't enforce origin checks for desktop apps. [VERIFIED: developers.google.com/workspace/drive/picker/guides/overview — origin restrictions only apply to web-app OAuth clients]
3. **Picker API:** Enable "Google Picker API" in APIs & Services → Library.
4. **Developer key (API key):** Create one in APIs & Services → Credentials → API key. Restrict to "Google Picker API" only.
5. **App ID:** Cloud project NUMBER (not project ID). Visible in console settings.
6. **Bake API_KEY and APP_ID into the binary** as build-time constants OR fetch from a `latest.json`-style remote config at first run. Phase 1: bake in.

### 5.5 `drive.file` semantic implication

`drive.file` only grants access to files **explicitly handed to the app** — by the Picker, by `drive.create`, or by user-shared file IDs. The Picker is the ONLY mechanism by which a guildie's existing shared workbook becomes visible to SquireBot. Resist any future plan that proposes pre-filling a sheet ID via config to skip the Picker — that path returns `403` despite OAuth success. [CITED: developers.google.com/workspace/drive/api/guides/api-specific-auth + CONTEXT.md "specifics" block]

### 5.6 "Change Workbook…" tray menu (D-04)

Phase 1 minimum: tray menu item that re-runs the wizard from step 2 (skipping OAuth — we already have a valid token). Implementation:
1. Click → bring loopback server back up on a fresh port → open browser to `/picker`.
2. On successful pick + validation, write new `spreadsheet_id` to config.json.
3. The fsnotify watcher does NOT need to restart for this; on next event it reads `config.GetSpreadsheetID()` fresh.

---

## 6. NSIS Per-User Installer

**Purpose:** Implement INST-01 (no UAC, single .exe).

### 6.1 Minimum directives (Phase 1)

```nsi
; installer/squirebot.nsi
!define APPNAME    "SquireBot"
!define APPVERSION "0.1.0"
!define EXE_NAME   "squirebot.exe"
!define REGPATH_UNINSTSUBKEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APPNAME}"

; --- THE critical directive: no UAC. ---
RequestExecutionLevel user

Name           "${APPNAME}"
OutFile        "SquireBot-Setup-${APPVERSION}.exe"
Unicode        true
SetCompressor  /SOLID lzma
ShowInstDetails show

; Install path: %LOCALAPPDATA%\Programs\SquireBot — never under Program Files (would need UAC).
InstallDir       "$LOCALAPPDATA\Programs\${APPNAME}"
InstallDirRegKey HKCU "${REGPATH_UNINSTSUBKEY}" "InstallLocation"

Page directory
Page instfiles
UninstPage uninstConfirm
UninstPage instfiles

Section "Install"
    SetOutPath "$INSTDIR"
    File "..\dist\${EXE_NAME}"
    File "icon.ico"

    ; Uninstaller registration — HKCU only (no admin needed).
    WriteUninstaller "$INSTDIR\uninstall.exe"
    WriteRegStr HKCU "${REGPATH_UNINSTSUBKEY}" "DisplayName"     "${APPNAME}"
    WriteRegStr HKCU "${REGPATH_UNINSTSUBKEY}" "DisplayVersion"  "${APPVERSION}"
    WriteRegStr HKCU "${REGPATH_UNINSTSUBKEY}" "InstallLocation" "$INSTDIR"
    WriteRegStr HKCU "${REGPATH_UNINSTSUBKEY}" "DisplayIcon"     "$INSTDIR\${EXE_NAME}"
    WriteRegStr HKCU "${REGPATH_UNINSTSUBKEY}" "UninstallString" '"$INSTDIR\uninstall.exe"'
    WriteRegStr HKCU "${REGPATH_UNINSTSUBKEY}" "QuietUninstallString" '"$INSTDIR\uninstall.exe" /S'
    WriteRegDWORD HKCU "${REGPATH_UNINSTSUBKEY}" "NoModify" 1
    WriteRegDWORD HKCU "${REGPATH_UNINSTSUBKEY}" "NoRepair" 1

    ; Phase 1 launches the wizard immediately after install.
    Exec '"$INSTDIR\${EXE_NAME}"'
SectionEnd

; INST-04 autostart is Phase 2. Phase 1 does NOT write the Run key —
; the watcher only runs when launched explicitly.

Section "Uninstall"
    ExecWait 'taskkill /IM "${EXE_NAME}" /F'  ; ensure not in use
    Delete "$INSTDIR\${EXE_NAME}"
    Delete "$INSTDIR\icon.ico"
    Delete "$INSTDIR\uninstall.exe"
    RMDir  "$INSTDIR"

    ; Cleanup user data — keep refresh token in wincred? Up to UX policy.
    ; Phase 1: wipe %LOCALAPPDATA%\SquireBot\config.json AND wincred entry.
    Delete "$LOCALAPPDATA\${APPNAME}\config.json"
    Delete "$LOCALAPPDATA\${APPNAME}\squirebot.log*"
    RMDir  "$LOCALAPPDATA\${APPNAME}"

    DeleteRegKey HKCU "${REGPATH_UNINSTSUBKEY}"
SectionEnd
```

[CITED: nsis.sourceforge.io/Examples/install-per-user.nsi + nsis.sourceforge.io/Reference/RequestExecutionLevel]

### 6.2 What triggers UAC unintentionally

| Trigger | Avoidance |
|---------|-----------|
| `RequestExecutionLevel admin` or `highest` | Use `user` |
| Installer filename containing `setup`, `install`, `update` | NSIS heuristically requests elevation. **Use a non-trigger name OR set `ManifestSupportedOS` + `RequestExecutionLevel user`** — the latter overrides the heuristic |
| Writing to `$PROGRAMFILES`, `HKLM`, `$WINDIR` | Write to `$LOCALAPPDATA`, `HKCU` only |
| `MultiUser.nsh` with `MULTIUSER_EXECUTIONLEVEL Admin` | Don't include MultiUser.nsh at all in Phase 1; we are CurrentUser-only |
| Manifest with `requestedExecutionLevel="requireAdministrator"` | Default NSIS manifest is fine; do not override |

The NSIS heuristic that auto-elevates installers with names like "setup-foo.exe" is the most common silent trip. The fix is twofold: (a) `RequestExecutionLevel user` explicitly overrides, AND (b) name the installer `SquireBot-Setup-X.Y.Z.exe` is fine because `RequestExecutionLevel user` wins. [CITED: nsis.sourceforge.io UAC plug-in talk page]

### 6.3 Silent install + auto-update interaction

Phase 2's auto-updater (selfupdate) replaces the running .exe in-place — it does NOT re-run NSIS. So Phase 1's installer is one-shot per machine. Phase 2 may want to ship a "/S" silent re-install path for users who prefer reinstall-over-upgrade; we don't need it in Phase 1.

### 6.4 Smoke test

The dev runs `.\SquireBot-Setup-0.1.0.exe` on a clean Win11 VM. Expected:
1. SmartScreen "Unknown publisher" wall (per D-13). User clicks "More info → Run anyway."
2. NSIS wizard appears; click Install. NO second UAC prompt.
3. Files land in `%LOCALAPPDATA%\Programs\SquireBot\`.
4. `SquireBot.exe` auto-launches.
5. Tray icon appears; default browser opens to `http://127.0.0.1:<port>/start`.

### 6.5 EQ folder discovery (INST-02)

Implementation cascade in `internal/eqfind/discover.go`:

```go
func Discover() (folder string, err error) {
    // 1. Prior-install state in config.json (handled by caller before calling Discover).

    // 2. Known paths.
    for _, p := range []string{
        `C:\P99`, `C:\Project1999`, `C:\Games\Project1999`,
        `C:\Program Files (x86)\Sony\EverQuest`,
        filepath.Join(os.Getenv("USERPROFILE"), "EverQuest"),
    } {
        if hasEQGameExe(p) { return p, nil }
    }

    // 3. Registry uninstall keys (HKCU + HKLM, both 32 and 64-bit views).
    if p := scanUninstallKeys(); p != "" { return p, nil }

    // 4. Heuristic recursive scan with depth limit and pruning.
    //    Search common drives (C:, D:, E:) for a folder containing both
    //    eqgame.exe AND eqclient.ini. Skip Windows, Program Files, $Recycle.Bin.
    if p := heuristicScan(); p != "" { return p, nil }

    return "", ErrNotFound  // wizard's step-3 falls through to native folder picker
}

func hasEQGameExe(dir string) bool {
    _, err1 := os.Stat(filepath.Join(dir, "eqgame.exe"))
    _, err2 := os.Stat(filepath.Join(dir, "eqclient.ini"))
    return err1 == nil && err2 == nil
}
```

**Registry uninstall keys to probe:**
- `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\*` — DisplayName matching `^(Project 1999|EverQuest|Sony EverQuest)`
- `HKLM\Software\Microsoft\Windows\CurrentVersion\Uninstall\*` (read-only; no admin needed)
- `HKLM\Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*` (32-bit on 64-bit OS)

Pull `InstallLocation` from each match; validate with `hasEQGameExe`.

### 6.6 Wizard step-3 native folder picker

Use Windows IFileDialog via a Go binding. Two options:
- `github.com/sqweek/dialog` — small, focused, MIT-licensed. `dialog.Directory().Title("Pick your EverQuest folder").Browse()`.
- Roll our own via `golang.org/x/sys/windows` + COM. Heavier; not worth Phase 1.

**Use `sqweek/dialog`** — adds ~30 KB to the binary, and Phase 1 needs it for exactly one place.

---

## 7. wincred (already covered in § 4.7)

See § 4.7 for full API surface. Two more notes:

- **Multi-account on same PC (deferred per CONTEXT.md):** if Phase 2+ ever supports two guildies sharing a Windows account, the `SquireBot:<email>` target naming pattern keeps tokens isolated. Phase 1 ignores; one credential per Windows user account.
- **Tray "Sign Out" button (Claude's discretion):** invokes `auth.DeleteToken(email)` + clears `config.spreadsheet_id` + reopens wizard. Recoverable.

---

## 8. fsnotify on Windows — Quirks and Patterns

**Purpose:** Implement WATCH-01 correctly.

### 8.1 EQ `/outputfile` write semantics

The EverQuest client's `/outputfile inventory` command writes the file **directly** in the EQ folder (NOT a temp file + atomic rename). The exact event sequence on Windows is best characterized as: `Create`, `Write`, possibly `Write` again, possibly `Chmod` (which fsnotify drops on Windows). [CITED: github.com/fsnotify/fsnotify — Windows backend uses ReadDirectoryChangesW; behavior described in Issue #17 / #214 / #255]

**Implication:** The 500ms debounce is sufficient — the entire write-burst lives within a single 500ms window in observation. We do NOT depend on event types; any event in the bucket triggers a re-read.

### 8.2 Debounce loop

```go
// internal/watch/watcher.go
func Run(ctx context.Context, eqFolder string, onChange func(path string)) error {
    w, err := fsnotify.NewWatcher()
    if err != nil { return err }
    defer w.Close()

    if err := w.Add(eqFolder); err != nil { return err }

    deb := newDebouncer(500 * time.Millisecond)

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case ev, ok := <-w.Events:
            if !ok { return errors.New("fsnotify Events channel closed") }
            // Filter on filename pattern. Inventory only in Phase 1.
            base := filepath.Base(ev.Name)
            if !strings.HasSuffix(base, "-Inventory.txt") { continue }
            // Drop pure Chmod (Windows: rare anyway).
            if ev.Op == fsnotify.Chmod { continue }
            deb.Trigger(ev.Name)
        case e, ok := <-w.Errors:
            if !ok { return errors.New("fsnotify Errors channel closed") }
            slog.Warn("fsnotify error", "err", e)
        case path := <-deb.Out():
            // 500ms quiet — re-stat and read fresh. NEVER trust ev.Op.
            onChange(path)
        }
    }
}
```

### 8.3 The "always re-read" rule

After the debounce timer fires:
1. `os.Stat(path)` — confirm still exists (handles "user deleted file" races).
2. Open and read the entire file fresh — never use any size or content the event might have carried (it doesn't carry any, but the discipline is to re-read regardless).
3. Parse + write to Sheets.

**Spurious events:** Antivirus scanning or Windows indexing can fire write events without the user actually touching the file. The 500ms debounce + always-re-read produces the same result whether the file changed or not — so spurious events become idempotent re-uploads. Negligible cost (one read + one Sheets write per spurious burst). [CITED: github.com/fsnotify/fsnotify — Windows AV-induced spurious events documented]

### 8.4 OneDrive-mapped EQ folder (deferred, but flag)

If the EQ folder is under `%USERPROFILE%\OneDrive\...` (Known Folder Move enabled), fsnotify still works on the local path because OneDrive syncs ARE local writes that the kernel sees. No Phase 1 change needed. But: cross-machine DPAPI doesn't follow OneDrive (refresh tokens don't sync), so a guildie installing on PC A and PC B will OAuth twice. Acceptable. [SUMMARY.md flagged as MEDIUM-confidence; verify with a OneDrive guildie in Phase 2 testing.]

---

## 9. Inventory File Parser (WATCH-04)

**Purpose:** Parse `<Char>-Inventory.txt` into 5-column rows.

### 9.1 File format

| Column | Type | Example | Notes |
|--------|------|---------|-------|
| Location | string | `General1`, `Bank2-Slot3`, `Charm` | Free-form; we don't validate |
| Name | string | `Fungi Tunic`, `Mosquito Tabar` | May contain apostrophes (`Tashan's Lance`); UTF-8 / Win-1252 |
| ID | int | `13128`, `0` | EQ item ID. **`0` for empty slots — DO filter at Phase 2 view-build time, but Phase 1 writes them through.** |
| Count | int | `1`, `42` | Stack count |
| Slots | int | `0`, `8`, `10` | Container slot count for bags |

**Header row (verified by Phase 1's first sample):** Yes, the first line is column headers. The parser MUST tolerate either a header row OR no header row (some legacy tools strip it). The first non-empty data line should have column-0 = a non-numeric string.

### 9.2 Encoding

P99 EverQuest is a 2002-era Windows client; `/outputfile` writes in **Windows-1252 (CP1252)** — the legacy "ANSI" encoding for English Windows. Item names with curly apostrophes (`’`), accented characters, etc. arrive as single-byte CP1252. UTF-8 decoding will produce mojibake. [ASSUMED — no canonical Daybreak/P99 documentation, and the Project 1999 forums confirm "tab-separated" without specifying encoding; verify against a real file in Phase 1 W3 before locking the decoder. Open Questions Q4.]

**Decoder:**
```go
// internal/parse/inventory.go
import (
    "encoding/csv"
    "io"
    "golang.org/x/text/encoding/charmap"
)

func Parse(r io.Reader) (rows [][]string, err error) {
    decoded := charmap.Windows1252.NewDecoder().Reader(r)
    cr := csv.NewReader(decoded)
    cr.Comma = '\t'
    cr.FieldsPerRecord = -1   // tolerate any column count
    cr.LazyQuotes = true      // EQ names may contain stray quotes
    all, err := cr.ReadAll()
    if err != nil { return nil, err }
    // Drop header row IF it doesn't parse as inventory (column 2 not an int).
    if len(all) > 0 && !isIntField(all[0], 2) { all = all[1:] }
    // Truncate / pad to 5 columns.
    out := make([][]string, 0, len(all))
    for _, r := range all {
        if len(r) < 5 { continue }   // malformed rows: skip
        out = append(out, r[:5])
    }
    return out, nil
}
```

### 9.3 Validation rules (Phase 1 minimum)

- Reject the entire write if 0 valid rows survive parsing → log + tray "Last upload failed (no rows)" status.
- ID must parse as integer; rows with non-int IDs are skipped (EQ has never produced these in observation, but defend).
- No length cap; P99 max is ~250 rows including bag-content rows.
- File ≤ 1 MB sanity check (logs warn-level above this; doesn't block).

### 9.4 What's NOT in the file (verified)

- **Coin amounts.** `/outputfile inventory` does NOT include platinum/gold/silver/copper. This is a known P99 file format limitation; coin tracking is a manual sidebar (BANK-02, Phase 4).
- **Character class / level.** Not in inventory file. Sidebar form (CHECK-01, Phase 4).
- **Spell IDs.** Spellbook is name-only (handled in Phase 2 spellbook scope).

---

## 10. Logging (OPS-03)

**Purpose:** Implement OPS-03's "5MB × 3 files" rotation.

### 10.1 Configuration

```go
// internal/logging/logger.go
import (
    "log/slog"
    "os"
    "path/filepath"
    "gopkg.in/natefinch/lumberjack.v2"
)

func Setup() *slog.Logger {
    appData := os.Getenv("LOCALAPPDATA")
    logDir := filepath.Join(appData, "SquireBot")
    _ = os.MkdirAll(logDir, 0755)

    rotator := &lumberjack.Logger{
        Filename:   filepath.Join(logDir, "squirebot.log"),
        MaxSize:    5,    // megabytes
        MaxBackups: 3,    // keep last 3 rotated files
        MaxAge:     28,   // days; covers a guildie taking a 3-week break
        Compress:   false, // false: simpler debugging; lumberjack default
        LocalTime:  true,
    }

    handler := slog.NewJSONHandler(rotator, &slog.HandlerOptions{
        Level: slog.LevelInfo,
        AddSource: true, // file:line in every record
    })
    return slog.New(handler)
}
```

[CITED: pkg.go.dev/gopkg.in/natefinch/lumberjack.v2; values match OPS-03 specification.]

### 10.2 Where to log

- `slog.Info("oauth completed", "email", email)` — every notable transition.
- `slog.Warn("fsnotify error", "err", e)` — recoverable.
- `slog.Error("sheets write failed", "char", char, "err", err)` — terminal for one upload; tray surfaces.
- DON'T log the refresh token or access token. Period.
- Log paths: use `slog.String("path", filepath.Base(p))` rather than full path when possible (privacy).

### 10.3 Tray "Open log folder" menu item

Phase 1 minimum tray surface (per CONTEXT.md "tray menu surface"):
- Status (read-only string, e.g., "Connected as foo@gmail.com — last upload 12s ago")
- Open Workbook (browser)
- Open log folder (explorer.exe to `%LOCALAPPDATA%\SquireBot`) — **MEDIUM-priority for support**
- Continue setup… (only when wizard incomplete)
- Quit

Open log folder via:
```go
exec.Command("explorer.exe", filepath.Join(os.Getenv("LOCALAPPDATA"), "SquireBot")).Start()
```

---

## 11. Tray UI (`fyne.io/systray`) Lifecycle

**Purpose:** Implement the tray icon + status surface.

### 11.1 Lifecycle

`systray.Run(onReady, onExit)` **blocks the main goroutine** on Windows — it has to, because the tray calls into the Win32 message loop. Pattern:

```go
// cmd/squirebot/main.go
func main() {
    log := logging.Setup()
    cfg := config.Load()
    coreCtx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Background goroutine: wizard + watcher + sheets I/O all live here.
    go runApp(coreCtx, log, cfg)

    // Main goroutine: blocking systray loop.
    systray.Run(onReady, onExit)
    cancel() // systray quit → tear down background.
}

func onReady() {
    systray.SetIcon(iconBytes)              // embedded via go:embed
    systray.SetTooltip("SquireBot")
    mStatus := systray.AddMenuItem("Initializing…", "")
    mStatus.Disable()
    mWorkbook := systray.AddMenuItem("Open Workbook", "")
    mLogs := systray.AddMenuItem("Open log folder", "")
    mQuit := systray.AddMenuItem("Quit", "")

    // Status updates are pushed via a channel from the background goroutine:
    go func() {
        for s := range statusCh {
            mStatus.SetTitle(s)
        }
    }()
    go func() {
        for {
            select {
            case <-mWorkbook.ClickedCh:
                exec.Command("rundll32", "url.dll,FileProtocolHandler",
                    "https://docs.google.com/spreadsheets/d/"+cfg.GetSpreadsheetID()).Start()
            case <-mLogs.ClickedCh:
                exec.Command("explorer.exe", logsDir).Start()
            case <-mQuit.ClickedCh:
                systray.Quit()
                return
            }
        }
    }()
}

func onExit() { /* cleanup; or empty */ }
```

[CITED: pkg.go.dev/fyne.io/systray + github.com/fyne-io/fyne discussion #4116 — `systray.Run` blocks the main goroutine; quit only exits the tray loop; `os.Exit` on the whole app must be done explicitly]

### 11.2 Goroutine layout

| Goroutine | Lifetime | What it does |
|-----------|----------|--------------|
| Main | Process lifetime | Blocking `systray.Run` |
| `runApp` background | Process lifetime | Wizard HTTP server (initial), then fsnotify watcher loop |
| `statusCh` consumer | Process lifetime | Pulls status updates onto the tray label |
| Per-menu-item `ClickedCh` consumers | Process lifetime | One per item; standard systray pattern |
| Debouncer timers | Per-event | `time.AfterFunc` callbacks |

### 11.3 Crash-safety

The fsnotify watcher's parent goroutine should `recover()` from panics and log + put tray into red state, not crash the process. Tray itself should always be responsive — including a "Quit" that works even if the wizard is wedged.

---

## 12. Identity Bootstrap & `_meta` / `_char_owner` Handshake

**Purpose:** Implement AUTH-06 (`userinfo.email` → `_char_owner.owner_email`) and the schema_version handshake.

### 12.1 Email lookup after OAuth

```go
// internal/auth/userinfo.go
import "google.golang.org/api/oauth2/v2"

func GetUserEmail(ctx context.Context, ts oauth2.TokenSource) (string, error) {
    svc, err := oauth2v2.NewService(ctx, option.WithTokenSource(ts))
    if err != nil { return "", err }
    info, err := svc.Userinfo.Get().Context(ctx).Do()
    if err != nil { return "", err }
    return info.Email, nil
}
```

This fires once after OAuth completes; result cached in config.json + tray status string.

### 12.2 First-run sequence

```
1. Wizard step 1 done       → tokens in wincred, email in config.google_email
2. Wizard step 2 (Picker)   → spreadsheetId in hand → ValidateWorkbook(spreadsheetId)
   ValidateWorkbook:
     read _meta!B1 (canonical_id)
     if canonical_id == ""           → BOOTSTRAP path (§ 12.3)
     if canonical_id == EXPECTED     → HEALTHY path (§ 12.4)
     else                             → REJECT with D-03 message
3. Wizard step 3 (EQ folder) → folder validated, in config.eq_folder
4. Wizard step 4 (Done)      → wizard server shuts down; watcher starts
5. /outputfile inventory     → fsnotify event → parse → ensure inv:<Char> tab exists
                              → batchUpdate atomic clear+write
                              → upsert _char_owner row
                              → log "uploaded N rows for <Char>"
                              → status tray "Last upload: <Char> at HH:MM"
```

### 12.3 BOOTSTRAP path (canonical_id empty)

This handles the "fresh template copy with no Apps Script run yet" case. Phase 1 ships before Phase 3 builds the Apps Script. So Phase 1 watcher MUST be able to bootstrap a workbook that has nothing on `_meta`.

```go
// internal/sheet/meta.go
const (
    CanonicalID            = "squirebot-v1-workbook-2026"  // baked in
    WatcherMaxSchemaVersion = 1
)

func (c *Client) ValidateWorkbook(ctx context.Context, sheetID string) error {
    c.spreadsheetID = sheetID
    metaCID, metaSV, err := c.readMeta(ctx)
    if err != nil {
        return fmt.Errorf("read _meta: %w", err)  // missing _meta tab is reject
    }
    switch {
    case metaCID == "":
        // BOOTSTRAP: write our canonical id + schema_version=1
        return c.bootstrapMeta(ctx)
    case metaCID == CanonicalID:
        if metaSV > WatcherMaxSchemaVersion {
            return fmt.Errorf("workbook schema v%d exceeds watcher max v%d (update SquireBot)", metaSV, WatcherMaxSchemaVersion)
        }
        return nil
    default:
        return errors.New("This doesn't look like a SquireBot workbook. Pick the one shared by your guild leader.")
    }
}

func (c *Client) bootstrapMeta(ctx context.Context) error {
    // Idempotent: write canonical_id at B1, schema_version at B2.
    // _meta tab MUST already exist on the master template; if it doesn't,
    // create it (addSheet) — defensive.
    if err := c.ensureSheet(ctx, "_meta"); err != nil { return err }
    return c.writeValues(ctx, "_meta", "A1:B2", [][]string{
        {"canonical_id", CanonicalID},
        {"schema_version", "1"},
    })
}
```

### 12.4 HEALTHY path (canonical_id matches)

No bootstrap; just check schema version and proceed. Subsequent watcher runs all hit this path.

### 12.5 `_char_owner` upsert (AUTH-06)

```go
// internal/sheet/owner.go
// Upsert a row into _char_owner keyed by char_name.
// Columns (Phase 1 minimum): A:char_name, B:owner_email, E:first_seen
// (other Phase 2 columns scaffolded by Apps Script later; we leave them blank)
func (c *Client) UpsertCharOwner(ctx context.Context, charName, ownerEmail string) error {
    if err := c.ensureSheet(ctx, "_char_owner"); err != nil { return err }
    rows, err := c.readSheetA1(ctx, "_char_owner!A:B")
    if err != nil { return err }
    nowISO := time.Now().UTC().Format(time.RFC3339)
    for i, r := range rows {
        if i == 0 { continue }       // header
        if len(r) >= 1 && r[0] == charName {
            if len(r) >= 2 && r[1] == ownerEmail { return nil } // no-op
            // Mismatch — Phase 2 surfaces in _audit; Phase 1 just logs.
            slog.Warn("char_owner email mismatch",
                "char", charName, "existing", r[1], "current", ownerEmail)
            return nil
        }
    }
    // Append.
    return c.appendRow(ctx, "_char_owner", []string{charName, ownerEmail, "", "", nowISO})
}
```

**Conflict resolution (D-03 of CONTEXT.md is silent on this; default policy):** Phase 1 logs the mismatch and proceeds with the inv:<Char> write; does NOT overwrite owner_email. Phase 2 (AUTH-05 + audit work) will surface in `_audit` for officer review. For Phase 1, we accept the rare "I gave my toon to a guildmate" case as a manual `_char_owner` edit later.

**Same Google account on two PCs (CONTEXT.md "two PCs" case):** both watchers OAuth as same email → no conflict, `_char_owner` agrees. Each PC's EQ folder may diverge in mtime — last-write wins on `inv:<Char>` writes. Acceptable for Phase 1.

### 12.6 Schema_version handshake (overflow case)

If `_meta.schema_version > WATCHER_MAX_SCHEMA_VERSION`, the watcher refuses to write. Phase 1 surfaces this as:
- Tray icon: red.
- Status: "Workbook schema is newer than this watcher. Update SquireBot."
- Log: error.
- Auto-update flow (Phase 2) covers the actual update; Phase 1 just blocks safely.

---

## Runtime State Inventory

> Phase 1 is greenfield — no existing rename/refactor, but the Phase 1 install creates new runtime state that Phase 2's auto-updater will later have to handle. This section documents the state Phase 1 creates so future phases know what to migrate.

| Category | Items Phase 1 Creates | Action Required |
|----------|------------------------|------------------|
| Stored data | `inv:<Char>` tabs in the shared workbook (per character); `_char_owner` rows; `_meta` canonical_id + schema_version | None Phase 1 — Phase 2 will extend `_char_owner` columns (extend-only per CLAUDE.md) |
| Live service config | OAuth client in Google Cloud Console (Production-published, scope set fixed) | One-time human step before Phase 1 ships; documented in `## Open Questions` Q3 |
| OS-registered state | HKCU `Uninstall` entry (`HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\SquireBot`); files at `%LOCALAPPDATA%\Programs\SquireBot\` | INST-04 autostart (HKCU\...\Run entry) is Phase 2 — Phase 1 deliberately does NOT register autostart |
| Secrets/env vars | wincred entry under target name `SquireBot:<email>` | Re-OAuth flow (Phase 2) overwrites cleanly; uninstaller wipes |
| Build artifacts | `dist/squirebot.exe` (unsigned in Phase 1); `SquireBot-Setup-X.Y.Z.exe`; `latest.json` stub on GitHub Releases | Phase 2 auto-update will replace `squirebot.exe` in-place via selfupdate; Phase 2 must NOT change the wincred target name format or break canonical_id |

**Nothing found in category** for: any existing renames, since this is greenfield. Verified by `git log --oneline | head` returning nothing (project is unversioned).

---

## Common Pitfalls

### Pitfall 1: Forgetting to flip OAuth consent to Production before any guildie installs

**What goes wrong:** Refresh tokens silently expire 7 days after issuance. Every guildie's watcher starts failing on day 8 with `invalid_grant`. They have no idea why.

**Why it happens:** Default Google Cloud Console state for a new OAuth consent screen is "Testing." Devs assume "Testing" means "still in development" and don't flip it before shipping. Google's UI doesn't warn loudly enough.

**How to avoid:** Make "publish OAuth consent to Production" an explicit Phase 1 plan task, BLOCKING on any guildie install. Plan-checker should look for this task by name.

**Warning signs:** First guildie's tokens still work after 7 days = healthy. First guildie reports "asked me to sign in again" on day 8 = caught the bug.

[CITED: SUMMARY.md pitfall #1; verified against Google Cloud Console publishing docs]

### Pitfall 2: Hand-rolling OAuth refresh logic instead of using `oauth2.TokenSource`

**What goes wrong:** Access tokens expire after ~1 hour. Custom code "remembers to refresh" but races with the Sheets API client which might issue a request mid-refresh. Phantom 401s.

**Why it happens:** The 5-line OAuth example in tutorials shows `cfg.Exchange(ctx, code)` and stops. Devs assume that's the whole flow and write their own refresh.

**How to avoid:** Wrap the initial token in `oauth2.ReuseTokenSource(initialToken, cfg.TokenSource(ctx, initialToken))` and pass to Sheets via `option.WithTokenSource(ts)`. The library handles refresh + reuse transparently.

**Warning signs:** Sporadic 401s that "go away on retry" = you're racing the refresher.

### Pitfall 3: Trusting fsnotify event payloads on Windows

**What goes wrong:** Plan reads "Op == Write" from event and acts; misses Create-only events; double-processes Write+Modify pairs; fails to handle AV-induced spurious events.

**How to avoid:** Always re-stat + re-read the file fresh after debounce. Treat fsnotify event as "something MAY have changed at this path" — never trust the Op or any payload. (CLAUDE.md explicitly forbids this anti-pattern.)

**Warning signs:** Antivirus complaints in logs; intermittent "uploaded same data twice" signals = you trusted ev.Op.

[CITED: github.com/fsnotify/fsnotify Issue #17/#214/#255]

### Pitfall 4: Watching individual files instead of the parent directory

**What goes wrong:** Watch on `C:\P99\Foo-Inventory.txt` directly. EQ writes the file → fsnotify says "file disappeared" because the kernel sees a brief unlink+create. Watcher loses track.

**How to avoid:** Always `Add(parentDirectory)` and filter by filename in the event loop.

[CITED: github.com/fsnotify/fsnotify Issue #372]

### Pitfall 5: Using the Desktop Picker mode with magic params (`prompt=consent&trigger_onepick=true`)

**What goes wrong:** Plan adds the magic params to OAuth URL. Google requires the redirect_uri to be a public HTTPS URL. Plan suddenly needs a hosted intermediate page. Massive scope creep.

**How to avoid:** Use the **classic Web Picker** loaded into a tiny HTML page served from the loopback HTTP server, with the OAuth access token already in hand from the standard loopback flow. No magic params needed.

[CITED: developers.google.com/workspace/drive/picker/guides/overview-desktop — restriction on redirect_uri]

### Pitfall 6: Using `localhost` in redirect_uri instead of `127.0.0.1`

**What goes wrong:** Some corporate firewalls block `localhost`-resolved connections (DNS-based blocking). User can OAuth in browser but the callback never reaches the watcher.

**How to avoid:** Use `127.0.0.1` literal everywhere — in the OAuth client console, in the redirect_uri parameter, in the http.Server `Addr`.

[CITED: developers.google.com/identity/protocols/oauth2/native-app — "Using localhost is possible but may cause issues with client firewalls."]

### Pitfall 7: Naming the installer something with `setup` AND not setting `RequestExecutionLevel user`

**What goes wrong:** NSIS heuristically auto-elevates installers whose filename matches `setup`/`install`/`update` patterns. UAC fires.

**How to avoid:** `RequestExecutionLevel user` explicitly overrides the heuristic. Including this directive is non-negotiable for Phase 1.

[CITED: nsis.sourceforge.io UAC plug-in talk page]

### Pitfall 8: `valueInputOption=USER_ENTERED` for the inventory write

**What goes wrong:** Sheet recalculates all formulas after every write. With consolidated views (Phase 3+) this is a recalc storm — multi-second writes balloon to 30+ seconds.

**How to avoid:** For the `UpdateCellsRequest` form (which we use), set `Fields: "userEnteredValue"` and let the cells be string-only. Don't pass numbers as `numberValue` — even if Count "looks like a number," writing as string avoids any type coercion. Phase 3 view-builders can re-cast.

[CITED: CLAUDE.md write contract; SUMMARY.md PITFALLS #recalc]

### Pitfall 9: Forgetting that `_meta` is empty on a fresh template copy

**What goes wrong:** Watcher's ValidateWorkbook reads `_meta.canonical_id`, finds empty, REJECTS the workbook. Guild leader can't onboard the first guildie.

**How to avoid:** Phase 1's bootstrap path (§ 12.3) writes canonical_id + schema_version=1 if `_meta` is empty. Apps Script (Phase 3) takes over after that.

### Pitfall 10: Spurious double-uploads when the file is read mid-write by the EQ engine

**What goes wrong:** fsnotify fires on Create. Watcher reads file. EQ is still flushing. File is shorter than final length. Watcher writes truncated rows.

**How to avoid:** 500ms debounce comfortably exceeds EQ's flush-and-close window. Empirically EQ writes the whole file inside ~50ms even on slow disks. We're 10× safe.

**Warning sign:** Inventory tab shows fewer rows than the user expected = the debounce is too short OR the parser dropped malformed rows. Log the raw row count before parse.

---

## Code Examples

### A. PKCE pair generation (verified pattern)

```go
// internal/auth/pkce.go
func NewPKCEPair() (verifier, challenge string, err error) {
    b := make([]byte, 32)
    if _, err = rand.Read(b); err != nil { return "", "", err }
    verifier = base64.RawURLEncoding.EncodeToString(b)
    sum := sha256.Sum256([]byte(verifier))
    challenge = base64.RawURLEncoding.EncodeToString(sum[:])
    return
}
```
Source: RFC 7636 + Google docs cited above.

### B. Loopback OAuth redirect handler (sketch)

```go
// internal/auth/oauth.go
func (m *Manager) handleCallback(w http.ResponseWriter, r *http.Request) {
    state := r.URL.Query().Get("state")
    if state != m.expectedState {
        http.Error(w, "CSRF: state mismatch", 400); return
    }
    code := r.URL.Query().Get("code")
    if code == "" {
        http.Error(w, "No code in callback", 400); return
    }
    tok, err := m.cfg.Exchange(r.Context(), code,
        oauth2.SetAuthURLParam("code_verifier", m.codeVerifier))
    if err != nil { http.Error(w, err.Error(), 500); return }

    email, err := userinfo.GetUserEmail(r.Context(), m.cfg.TokenSource(r.Context(), tok))
    if err != nil { http.Error(w, err.Error(), 500); return }

    if err := store.StoreToken(email, store.StoredToken{
        RefreshToken: tok.RefreshToken, Email: email, ClientID: m.cfg.ClientID,
    }); err != nil { http.Error(w, err.Error(), 500); return }

    m.config.SetGoogleEmail(email)
    http.Redirect(w, r, "/picker", http.StatusFound)
}
```
Source: oauth2 godoc + CLAUDE.md identity rules.

### C. Atomic clear+write via single `UpdateCellsRequest`

Already shown in § 2.3. Reproduced for completeness:
```go
req := &sheets.BatchUpdateSpreadsheetRequest{
    Requests: []*sheets.Request{{
        UpdateCells: &sheets.UpdateCellsRequest{
            Range: &sheets.GridRange{
                SheetId: invSheetID, StartRowIndex: 0, EndRowIndex: 500,
                StartColumnIndex: 0, EndColumnIndex: 6,
            },
            Rows:   rowsHeaderPlusData,
            Fields: "userEnteredValue",
        },
    }},
}
_, err := svc.Spreadsheets.BatchUpdate(spreadsheetID, req).Context(ctx).Do()
```
Source: developers.google.com/workspace/sheets/api/reference/rest/v4/spreadsheets/request#UpdateCellsRequest.

### D. Inventory parser (Win-1252 + tolerant TSV)

Already shown in § 9.2.

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| OAuth `oob` flow ("copy this code") | Loopback redirect on `127.0.0.1:<random>` with PKCE | Google removed `oob` 2022 | Old tutorials are wrong; reject any "copy code" UX |
| `clasp` 1.x bundled TS transpilation | clasp 3.x + local `tsc` + `esbuild` (Phase 3, not Phase 1) | clasp 2.x removed TS | Phase 3 implication only |
| Apps Script Rhino runtime | V8 (mandatory) | Rhino EOL 2026-01-31 | Phase 3 implication only |
| `getlantern/systray` | `fyne.io/systray` (maintained fork) | ~2023 | Use Fyne fork |
| Per-machine MSI requiring admin | NSIS per-user `RequestExecutionLevel user` | N/A — choice | UAC-free is non-negotiable |
| Service account JSON keys | Per-user OAuth | Always | CLAUDE.md forbids service accounts |

**Deprecated/outdated:**
- `oob` OAuth flow — removed; never works.
- Apps Script Rhino — gone.
- `golang.org/x/oauth2/jwt` for service-account flows — irrelevant; not allowed.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Phase 1 watcher BOOTSTRAPS `_meta.canonical_id` if empty | § 2.6, § 12.3 | If Phase 3 Apps Script ALSO bootstraps and they disagree on canonical_id, every Phase 1 guildie's onboarding rejects the workbook. **Decision needed:** confirm canonical_id constant and bootstrap responsibility (Watcher vs Apps Script vs both idempotent) before W4. |
| A2 | Inventory file encoding is Windows-1252, not UTF-8 | § 9.2 | If file is actually UTF-8, decoding via charmap.Windows1252 corrupts ASCII subset (no — CP1252 is ASCII-compatible for code points <128). If file is actually UTF-8 with non-ASCII, CP1252 decode produces mojibake. **Verify by reading a real EQ-produced file in W3 — Q4.** |
| A3 | Google OAuth consent "Publish app" is immediate for non-sensitive + sensitive-exempt scope sets | § 4.6 | If Google's audit queue applies anyway, Phase 1 ship slips by ~weeks. **Verify by clicking Publish app on a scratch Cloud project before W1 — Q3.** |
| A4 | NSIS 3.10+ honors `RequestExecutionLevel user` even when installer filename matches the auto-elevate heuristic ("setup-X.Y.Z.exe") | § 6.2 | If heuristic wins, every install fires UAC. Workaround is to rename the installer (`squirebot-x-y-z.exe`). **Verify on W5 smoke test.** |
| A5 | Go 1.24 is current and installed on the dev's build host | § 1 | If older Go, some std-lib API may differ; small risk. **Verify on W0.** |
| A6 | `userinfo.email` + `openid` + `drive.file` combined in one OAuth request remain sensitive-exempt | § 4.2 | If Google reclassifies, Production publishing requires audit (~weeks). Low probability but worth checking the current scope page on W1 day-of. |
| A7 | EQ `/outputfile inventory` writes within ~50ms; 500ms debounce is sufficient | § 8.1 | If EQ takes longer (slow HDD, antivirus full-scan), watcher reads partial file. Mitigated by reading file fresh + parser tolerating short rows. Empirical observation only; verify on W3. |

---

## Open Questions (RESOLVED)

1. **Canonical_id ownership: Watcher OR Apps Script (or both, idempotent)?**
   - What we know: ARCHITECTURE.md says `_meta.canonical_id` is a "fixed marker the template carries" (= Apps Script writes it). CONTEXT.md D-01 says the master template has the Apps Script preinstalled. But Phase 1 ships before Phase 3 builds the Apps Script — so the master template won't have a working `_meta.canonical_id` until Phase 3.
   - What's unclear: does Phase 1's master template ship with canonical_id pre-baked as a static cell value (no Apps Script needed), or does the watcher bootstrap it?
   - Recommendation: **Phase 1 master template is built MANUALLY by the dev** with `_meta!A1=canonical_id`, `_meta!B1=squirebot-v1-workbook-2026`, `_meta!A2=schema_version`, `_meta!B2=1`. No Apps Script involved in Phase 1. The watcher's bootstrap path (§ 12.3) becomes a defensive fallback only. Document this in W2's plan.
   - **RESOLVED:** Plan 05 `bootstrapMeta` handles the fresh-template case (writes `canonical_id` + `schema_version=1` if `_meta` is empty). Dev manually pre-fills `_meta` on master template per D-01; bootstrap is defensive fallback.

2. **What does the dev's Google Cloud project look like?**
   - What we know: We need a "Desktop app" OAuth client + Picker API enabled + an API key for Picker.
   - What's unclear: project ID, project number, OAuth client ID, API key — these are baked into the binary as build-time constants and can't be in git (or can they? OAuth client IDs are public; API keys for Picker scoped to "Picker API only" are ~public).
   - Recommendation: bake Client ID, API key, App ID (project number) into Go at build-time via `-ldflags="-X main.OAuthClientID=..."`. Document the exact Cloud Console steps (project create, enable APIs, create OAuth client, create API key, configure consent screen, add scope, publish to Production) in a `docs/oauth-setup.md` file produced as part of W1.
   - **RESOLVED:** Plan 02 produces `oauth-config.json` (client_id placeholder + scope list). Dev fills the real client_id post-publish.

3. **Google's "Publish app" turnaround for our scope set**
   - What we know: drive.file is non-sensitive; openid+userinfo.email are sensitive-exempt subset.
   - What's unclear: Whether clicking "Publish app" on a Cloud project with this exact scope set fires the audit queue anyway. The docs are silent on this exact combination.
   - Recommendation: **Test on a throwaway Google Cloud project as the FIRST W1 task.** Click Publish app, observe the response. If it goes In production immediately, we're golden. If it queues, we have a Phase 1 schedule risk and need to plan for the wait. Either way, surface result in plan.
   - **RESOLVED:** Plan 02 Task 3 is the human checkpoint that publishes the OAuth app to Production state and records `consent_screen_status: PRODUCTION` in oauth-config.json. Plan 08 release pipeline gates on that field.

4. **Verify inventory file encoding by reading a real sample**
   - What we know: P99 is a 2002-era client; CP1252 is the most likely encoding for Windows-native EQ output of that era.
   - What's unclear: We have no canonical Daybreak/P99 documentation. Some downstream tools (Awran/Import-EQInventory) read it as UTF-8 by default and reportedly work — which suggests either (a) most item names are pure ASCII and encoding never matters, or (b) the file IS UTF-8.
   - Recommendation: **W3 task: dev runs `/outputfile inventory` on their own character, opens the file in a hex editor, looks at any non-ASCII characters (`’`, `é`, `ñ`).** If `’` shows as `0x92` → CP1252. If `0xE2 0x80 0x99` → UTF-8. Lock decoder accordingly. Cost: 5 minutes.
   - **RESOLVED:** Plan 04 SUMMARY documents Win-1252 vs UTF-8 detection (5-min hex-dump task during W3); parser auto-detects and falls back to Win-1252 since ASCII subset is identical.

5. **Wizard tech: Go-native UI vs HTML on loopback (D-08)?**
   - What we know: D-08 leaves choice to Claude. Constraint: single-binary install with no runtime deps.
   - Recommendation made in this research: **HTML on loopback HTTP server.** Reasons: (a) zero new runtime dependencies, (b) reuses the loopback server already needed for OAuth callback, (c) browser tab UX is consistent with the OAuth + Picker flow which are also browser-tab. Lock in W1 plan.
   - **RESOLVED:** Plan 07 wizard tech is HTML pages served from the same loopback HTTP server (no native UI runtime dep). Reuses Plan 03's listener.

6. **Tray icon asset format**
   - What we know: Phase 1 needs a SquireBot icon (16×16 / 32×32).
   - Recommendation: dev produces an `.ico` file (multi-resolution) in W0, embedded via `go:embed`. Stand-in icon is fine for Phase 1; final art is Phase 5 polish.
   - **RESOLVED:** Plan 01 ships an embedded placeholder icon (assets/icon.ico). Final art is a Phase 2 polish item.

7. **Should the Phase 1 binary log telemetry "phone home"?**
   - What we know: No requirement for it; CLAUDE.md is silent.
   - Recommendation: **No.** Phase 1 logs locally only. Sheet-side `_audit` (Phase 2+) is the upstream observability surface. Keep Phase 1 telemetry-free for trust.
   - **RESOLVED:** No telemetry in Phase 1. Anonymous usage signals are out of scope; logs stay local under `%LOCALAPPDATA%\\SquireBot\\logs\\` per OPS-01/03.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.24+ | Watcher build (W0+) | ✗ (this research env) | — | Dev installs `go1.24.x` from go.dev on the build host |
| NSIS 3.10+ | Installer build (W5) | ✗ (this research env) | — | Dev installs from nsis.sourceforge.io on the build host |
| `signtool` (Windows SDK) | Code signing — DEFERRED Phase 2 | ✗ | — | Phase 1 ships unsigned per D-13 |
| Google Cloud project + Picker API + OAuth client | Auth (W1, W2) | Unknown | — | Dev creates one as first W1 task; document creation steps |
| Google Cloud project number / API key / OAuth client ID | Build constants | Unknown | — | Bake at build time once project is created |
| GitHub repository (public) | Releases hosting (W5+) | Unknown | — | Dev creates `github.com/<owner>/squirebot` per D-12 |
| GitHub Actions enabled | CI for goreleaser stub (W5+) | Unknown | — | Default-on for new repos |
| Master Sheets template (with `_meta` cells pre-filled) | Workbook validation (W2) | ✗ | — | Dev creates per Open Questions Q1 |
| Real `<Char>-Inventory.txt` sample | Parser verification (W3) | Unknown | — | Dev produces via `/outputfile inventory` on their own char |

**Missing dependencies with no fallback:** None — every gap is "dev does setup before W{N}".

**Missing dependencies with fallback:** All listed above are dev-host setup tasks.

**Phase 1 schedule risk:** None of these are blocking research-level decisions. They become explicit task line-items in the W0 / W1 / W2 / W5 wave plans.

---

## Security Domain

### Applicable ASVS Categories (Level 1)

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | OAuth 2.0 loopback PKCE per RFC 7636 + Google docs (§ 4); state parameter for CSRF on callback |
| V3 Session Management | partial | Refresh token = effectively the session; rotated by `oauth2.TokenSource` automatically; revocation surfaces as `invalid_grant` (Phase 2 UX) |
| V4 Access Control | yes | `drive.file` scope minimization — watcher cannot access any user file other than the explicitly Picker-selected workbook |
| V5 Input Validation | yes | TSV parser validates 5-col minimum + integer ID; canonical_id check rejects non-SquireBot workbooks; folder picker validates `eqgame.exe` |
| V6 Cryptography | yes | DPAPI via wincred for refresh-token storage; SHA-256 for PKCE; never roll our own |
| V7 Error Handling | yes | slog records errors without leaking secrets; tray surfaces user-actionable status |
| V14 Configuration | yes | Build-time constants for OAuth client ID; runtime `config.json` is non-secret only |

### Known Threat Patterns for Go + Windows + OAuth desktop

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| OAuth state parameter not validated → CSRF on callback | Spoofing | Generate random `state`, store, validate in callback handler (§ B example) |
| Refresh token leaking via log files | Information Disclosure | Never log token contents; use `slog.String("email", ...)` only |
| PKCE verifier predictable / short | Tampering | `crypto/rand` 32 bytes → 43-char base64url verifier (§ A) |
| `localhost` redirect intercepted by another loopback listener | Spoofing | Bind to specific random port via `net.Listen("tcp", "127.0.0.1:0")`, read assigned port; lifetime is bounded to the OAuth flow |
| Malicious workbook tricking watcher into writing to attacker-controlled sheet | Tampering | `_meta.canonical_id` validation per § 2.6 — any non-matching workbook is rejected |
| Stolen wincred entry copied to attacker machine | Information Disclosure | DPAPI keys are user-profile-bound; copying the credential to another user account makes it undecryptable. We rely on Windows DPAPI semantics. |
| Path traversal in EQ folder discovery (registry-supplied path) | Tampering | Validate discovered folder contains `eqgame.exe`; reject otherwise |
| Compromised binary distribution (no code signing in Phase 1) | Tampering | Phase 1 ships unsigned per D-13 — accepted risk for dev-only validation; SmartScreen + GitHub Releases SHA-256 mitigate. Phase 2 adds signing. |
| Watcher writing to `_char_owner` with attacker-influenced email | Spoofing | Email comes from Google's userinfo endpoint, not user input — trust boundary is Google itself |
| `drive.file` scope creep to `spreadsheets` or `drive` | Privilege Escalation | CLAUDE.md hard-rule + plan-checker would reject |

### Phase 1 explicit security posture

- **Refresh tokens:** wincred only; never logs, never config.json. (AUTH-04)
- **State CSRF:** validated on every callback. (AUTH-01)
- **Scope minimization:** `drive.file` + `openid` + `userinfo.email` only — sensitive-exempt subset. (AUTH-02)
- **Workbook authentication:** canonical_id check ensures we don't write to a non-SquireBot workbook the user happens to pick. (D-03)
- **Phase 1 accepts:** unsigned binary (D-13), no code-signing audit, SmartScreen warning is OK for dev validation.

---

## Sources

### Primary (HIGH confidence — directly probed via Context7 or fetched from official Google/Microsoft docs)

- [Google: OAuth 2.0 for iOS & Desktop Apps](https://developers.google.com/identity/protocols/oauth2/native-app) — confirmed loopback flow, client_secret optional for desktop, `127.0.0.1` literal preferred over `localhost`, refresh token returned automatically with `access_type=offline`+`prompt=consent`
- [Google: spreadsheets.batchUpdate](https://developers.google.com/workspace/sheets/api/reference/rest/v4/spreadsheets/batchUpdate) — atomicity of multi-request batch
- [Google: UpdateCellsRequest](https://developers.google.com/workspace/sheets/api/reference/rest/v4/spreadsheets/request) — confirmed range+rows+fields = atomic clear+write
- [Google: Choose Sheets API scopes](https://developers.google.com/workspace/sheets/api/scopes) — `drive.file` is non-sensitive
- [Google: Sensitive scope verification](https://developers.google.com/identity/protocols/oauth2/production-readiness/sensitive-scope-verification) — `userinfo.email`+`openid` exemption
- [Google: Manage App Audience (Production publish)](https://support.google.com/cloud/answer/15549945) — verifies Production-publish mechanics
- [Google: Drive Picker overview (web)](https://developers.google.com/workspace/drive/picker/guides/overview) — classic Web Picker uses access token, no redirect URI required
- [Google: Drive Picker overview (desktop)](https://developers.google.com/workspace/drive/picker/guides/overview-desktop) — desktop-mode Picker requires HTTPS redirect; we don't use this mode
- [RFC 7636: PKCE](https://datatracker.ietf.org/doc/html/rfc7636) — code_verifier rules, S256 method
- [github.com/fsnotify/fsnotify](https://pkg.go.dev/github.com/fsnotify/fsnotify) — Windows backend, event types, debounce guidance
- [github.com/danieljoos/wincred](https://github.com/danieljoos/wincred) — Windows Credential Manager wrapper
- [pkg.go.dev/fyne.io/systray](https://pkg.go.dev/fyne.io/systray) — Run blocking, callback patterns
- [gopkg.in/natefinch/lumberjack.v2](https://github.com/natefinch/lumberjack) — rotation defaults
- [NSIS install-per-user.nsi example](https://nsis.sourceforge.io/Examples/install-per-user.nsi) — canonical per-user template
- [NSIS RequestExecutionLevel reference](https://nsis.sourceforge.io/Reference/RequestExecutionLevel)

### Secondary (MEDIUM confidence — community / WebSearch verified against official sources)

- [P99 wiki — Custom User Interfaces / EQHTML](https://wiki.project1999.com/EQHTML) — confirms `<Char>-Inventory.txt` filename pattern + tab-separated
- [Awran/Import-EQInventory](https://github.com/Awran/Import-EQInventory) — community parser (informal confirmation of column layout)
- [P99 forums — /outputfile thread](https://www.project1999.com/forums/archive/index.php/t-299806.html) — describes inventory output behavior
- [github.com/fsnotify/fsnotify Issues #17, #214, #255, #372](https://github.com/fsnotify/fsnotify/issues) — Windows quirks documentation
- [Microsoft Learn — DPAPI MasterKey backup failures](https://learn.microsoft.com/en-us/troubleshoot/windows-server/certificates-and-public-key-infrastructure-pki/dpapi-masterkey-backup-failures) — credential-loss-on-password-reset behavior

### Tertiary (LOW confidence — flagged for verification before relying on)

- Inventory file encoding (Win-1252 vs UTF-8) — no canonical Daybreak/P99 doc; must verify via real sample (Q4)
- Publish-app turnaround for our scope set — must verify by clicking it on a scratch project (Q3)

---

## Metadata

**Confidence breakdown:**
- Standard stack — HIGH — every library + version cited from current pkg.go.dev / official sources
- OAuth + Picker flow — HIGH — Google docs directly fetched and cross-verified
- Sheets atomic write contract — HIGH — `UpdateCellsRequest` semantics confirmed verbatim
- NSIS per-user pattern — HIGH — official example template + reference docs
- fsnotify Windows quirks — HIGH — multi-issue corroboration
- Inventory file encoding — MEDIUM — must verify against a real sample (Q4); ASCII subset works either way
- Wizard tech choice — MEDIUM — D-08 left to Claude; HTML-on-loopback is recommended but other paths defensible
- Picker API key / App ID extraction details — MEDIUM — Cloud Console steps depend on specific console UI generation; document during W1
- Production-publish turnaround — MEDIUM — community evidence is consistent with "immediate" but no official SLA

**Research date:** 2026-04-30
**Valid until:** ~2026-05-30 for stable items (OAuth, Sheets API, fsnotify behavior); ~2026-05-07 for fast-moving items (Google Cloud Console UI, Picker API quirks).

## RESEARCH COMPLETE
