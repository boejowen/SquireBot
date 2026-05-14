# Stack Research

**Domain:** Windows desktop "watcher" + Google-Sheets-as-backend with Apps Script + light scraping (P1999 wiki, PigParse REST API)
**Researched:** 2026-04-30
**Confidence:** HIGH for the watcher language pick, OAuth pattern, Apps Script runtime; HIGH for PigParse (verified the Swagger API exists and exposes item lookup by ID); MEDIUM for the systray library and auto-updater specifics; MEDIUM for installer mechanics (multiple equally valid options).

---

## TL;DR Recommendation

| Concern | Pick |
|---|---|
| Watcher language | **Go 1.24.x** (single static binary, no runtime, mature Google client) |
| Watcher GUI | **Headless + system tray + browser-rendered first-run page** — no GUI framework |
| OAuth flow | **Loopback redirect + PKCE** to `127.0.0.1:<random-port>`, scope `drive.file` only |
| Token storage | **Windows Credential Manager via `github.com/danieljoos/wincred`** (DPAPI under the hood) |
| File watcher | **`github.com/fsnotify/fsnotify` v1.7+** with manual debounce |
| Sheets client | **`google.golang.org/api/sheets/v4`** + **`golang.org/x/oauth2`** |
| Auto-update | **`github.com/minio/selfupdate`** pulling signed releases from a GitHub Releases JSON manifest |
| Installer | **NSIS** in per-user mode (`RequestExecutionLevel user`, install to `%LocalAppData%\Programs\SquireBot`), no admin prompt |
| Sheet logic | **Apps Script V8 runtime, authored in TypeScript via `clasp` + `esbuild`** — V8 is mandatory (Rhino dies 2026-01-31) |
| Wiki scrape | Run inside **Apps Script** on a weekly time-driven trigger using **`UrlFetchApp` against `wiki.project1999.com/api.php?action=parse`** (MediaWiki API is enabled on the wiki) |
| PigParse | **Direct REST calls** from Apps Script — `/api/item/getmultiple/1` (server=1 = Blue) on a daily trigger; PigParse exposes a real Swagger API, no scraping required |
| Sheet UI | **HtmlService sidebar** for the search experience; **cell notes via `Range.setNote`** for inline tooltips |

---

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|---|---|---|---|
| **Go** | 1.24.x (1.24.13 is current; pin minor) | Watcher language | Single statically-linked Windows .exe with no runtime to install, no Python interpreter, no .NET runtime, no JRE. Mature first-party Google API client (`google.golang.org/api/sheets/v4`). Low memory footprint, fast cold start, trivial cross-compilation from any platform. Directly addresses the "idiot-proof installer" hard requirement. |
| **`google.golang.org/api/sheets/v4`** | latest (rolling) | Google Sheets API client | First-party, auto-generated, actively maintained Google client. The canonical way to write to a Sheet from Go. |
| **`golang.org/x/oauth2`** + `oauth2/google` | latest | OAuth 2.0 + Google helpers | The canonical Go OAuth library. Implements PKCE, refresh-token rotation, and `TokenSource` plumbing that integrates cleanly with `sheets.NewService(ctx, option.WithTokenSource(...))`. |
| **`github.com/fsnotify/fsnotify`** | v1.7.0+ | File-system events | De facto standard cross-platform watcher; on Windows uses `ReadDirectoryChangesW`. Reliable for inventory/spellbook drop detection. Known caveats: AV software can fire spurious write events (we debounce and re-read the file rather than trusting the event payload), and Chmod is unsupported on Windows (irrelevant to us — we only care about Modify/Create). |
| **`github.com/danieljoos/wincred`** | v1.2.x | Windows Credential Manager wrapper | Thin Go wrapper over the Windows Credential Manager Win32 API — backed by DPAPI, tied to the user profile, no extra dependencies. Stores the OAuth refresh token under the key `SquireBot:<google-email>`. Preferred over `zalando/go-keyring` because we are Windows-only and want zero indirection (zalando wraps wincred anyway). |
| **`github.com/minio/selfupdate`** | latest | In-place binary self-update | Active fork of the dormant `inconshreveable/go-update`. Downloads new binary from a signed URL, atomically swaps the running .exe on next start. Pairs with a tiny `latest.json` manifest published to GitHub Releases. No platform-specific update service required. |
| **`fyne.io/systray`** | v1.10+ | System tray icon + menu | Maintained fork of the original `getlantern/systray` (the upstream is now under the Fyne org). Cross-platform but we only need Windows. Lets us avoid bundling a full GUI toolkit; the only UI we need is "icon + Quit + Open Setup + Status: connected as <email>". |
| **NSIS** | 3.10+ | Installer generator | Generates a small (~150 KB overhead) `.exe` installer. With `RequestExecutionLevel user` and install path `$LOCALAPPDATA\Programs\SquireBot` it does not trigger UAC. Adds a Run-key entry under `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` so the watcher autostarts on logon — no Task Scheduler complexity. |
| **Apps Script (V8 runtime)** | V8 (mandatory; Rhino EOL 2026-01-31) | Sheet-side logic, scrape orchestration, sidebar UI | Hosted-and-free, no server to operate, native access to the Sheet, native `UrlFetchApp` for outbound HTTP, native time-driven triggers, native `HtmlService` for the search sidebar. The right tool because the Sheet *is* the product. |
| **`clasp`** | v3.0+ | Local Apps Script dev | Pulls/pushes Apps Script projects from git. Supports TypeScript transpilation indirectly (clasp itself no longer transpiles TS — see "What NOT to use" — but we transpile locally and `clasp push` the JS). |
| **TypeScript + esbuild + `@types/google-apps-script`** | TS 5.x, esbuild 0.20+ | Type-safe Apps Script authoring | TypeScript catches the common Apps Script footguns (typos against the global services, wrong return types, missing `null` checks). `esbuild` bundles into a single `Code.gs` that `clasp push` uploads. `@types/google-apps-script` covers the entire API surface. |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---|---|---|---|
| `github.com/spf13/viper` or stdlib `encoding/json` | latest | Config file (EQ folder path, last-known-modified-times) | Config lives in `%AppData%\SquireBot\config.json`. Use stdlib if config stays simple. |
| `log/slog` (stdlib, Go 1.21+) | stdlib | Structured logging | Built-in, no dep. Log to `%AppData%\SquireBot\squirebot.log` with daily rotation via `gopkg.in/natefinch/lumberjack.v2`. |
| `gopkg.in/natefinch/lumberjack.v2` | v2.x | Log rotation | Cap log file at e.g. 5 MB, keep last 3 — important since this runs unattended for months. |
| `github.com/lestrrat-go/jwx` | not needed | — | We do *not* need JWT/JWK manipulation — Google's standard OAuth flow gives us refresh tokens directly. Skip. |
| **Apps Script side:** `cheerio`-style HTML parsing | n/a in Apps Script | — | No `cheerio` in Apps Script. Use the **MediaWiki API** (`action=parse&prop=wikitext` or `action=query&prop=revisions&rvprop=content`) to get *structured* wikitext or rendered HTML rather than scraping rendered pages. Falls back to `XmlService.parse` only if a specific page resists API extraction. |
| **Apps Script side:** `CacheService` | built-in | Caching scraped wiki/PigParse responses | 6-hour cache for PigParse responses (still well within the daily refresh cadence) and 7-day cache for parsed wiki pages. Reduces UrlFetchApp quota burn. |
| **Apps Script side:** `PropertiesService` | built-in | Cross-execution state (last-scrape ETag, last-run timestamp) | Survives between trigger invocations; used to resume long-running multi-page wiki scrapes that bump up against the 6-minute execution cap. |

### Development Tools

| Tool | Purpose | Notes |
|---|---|---|
| **`goreleaser`** | Build + sign + publish Windows release | Single YAML config produces `.exe`, code-signs it (when you have a cert), generates the `latest.json` manifest, and uploads to GitHub Releases. Pair with the auto-updater. |
| **NSIS via `goreleaser` plugin** or hand-written `.nsi` | Installer | Hand-written `.nsi` is ~80 lines for our scope; recommend hand-writing rather than fighting tool abstractions. |
| **Code-signing certificate** | Avoid SmartScreen "unrecognized app" warning | Cheapest path is a sectigo / DigiCert OV cert (~$200/yr) — **strongly recommended** because the SmartScreen warning is the #1 risk to the "click Allow once" UX. Without a signing cert, ~30% of guildies will see a scary blue screen. EV cert ($400+/yr) is overkill for 12 users. **MEDIUM confidence — verify pricing at purchase time.** |
| **`clasp`** | Apps Script git workflow | `clasp pull`, edit locally, `clasp push`. Lets you keep the script in the same repo as the watcher. |
| **GitHub Actions** | CI for the Go binary + Apps Script | Build watcher on tag, run `goreleaser`, attach artifacts to the GitHub Release. For Apps Script: `clasp push` on merge to main, optionally `clasp deploy` to bump the deployment version. |

---

## Installation

```bash
# Watcher (Go)
go mod init github.com/<owner>/squirebot
go get google.golang.org/api/sheets/v4
go get golang.org/x/oauth2/google
go get github.com/fsnotify/fsnotify
go get github.com/danieljoos/wincred
go get github.com/minio/selfupdate
go get fyne.io/systray
go get gopkg.in/natefinch/lumberjack.v2

# Build
GOOS=windows GOARCH=amd64 go build -ldflags="-H=windowsgui -s -w" -o squirebot.exe ./cmd/squirebot
# -H=windowsgui suppresses the console window
# -s -w strips debug symbols (~30% size reduction)
```

```bash
# Apps Script (sheet side)
npm install -g @google/clasp
npm install -D typescript esbuild @types/google-apps-script
clasp login
clasp clone <SCRIPT_ID>     # script bound to the shared workbook
# write TS, bundle with esbuild, then:
clasp push
```

---

## Auth: Per-User Google OAuth from a Windows Desktop App

**Recommended pattern (current Google guidance):**

1. Watcher generates a random `code_verifier` (PKCE) and a random localhost port `N` (e.g. 49152–65535 ephemeral range), then opens `https://accounts.google.com/o/oauth2/v2/auth?...&redirect_uri=http://127.0.0.1:N&code_challenge=...` in the user's default browser via `os/exec`.
2. Watcher starts an `http.Server` on `127.0.0.1:N` to receive the redirect carrying `?code=...`.
3. Watcher exchanges the code for an access token + refresh token (PKCE; **no client secret needed for Public OAuth client type "Desktop app"**).
4. Refresh token is stored in **Windows Credential Manager** via `wincred` (target name `SquireBot:<google-email>`, persistence `LocalMachine` or `Enterprise` — choose `LocalMachine`).
5. Subsequent runs read the refresh token, mint access tokens via `oauth2.TokenSource`, never bother the user again.

**Scope minimization:**

- Use **`https://www.googleapis.com/auth/drive.file`** — *not* the broader `https://www.googleapis.com/auth/spreadsheets`.
- `drive.file` is a **non-sensitive scope**, which means: (a) no Google verification process required, (b) per-file consent that the user can revoke per-file, (c) the consent screen says "See and edit only the specific files you choose with this app" — much less alarming than the spreadsheets scope's "See, edit, create, and delete all your spreadsheets."
- The flow: user is told to first open the shared workbook in their browser, then SquireBot's first-run page presents a Google file picker (via the Picker API JS in a tiny embedded HTML page) where they explicitly select the workbook. After that, `drive.file` grants access to that exact file forever.
- Trade-off: requires a Picker on first run (one extra click). Worth it.

**Sources verified:**
- Google explicitly states the loopback flow is *kept* for desktop while being deprecated for mobile/Chrome (April 2026 docs).
- PKCE is required for the loopback flow — not optional.

**Confidence:** HIGH on the flow shape; HIGH on `drive.file` being preferable to `spreadsheets`; HIGH on PKCE-no-secret being current Google guidance.

---

## Apps Script: Sheet-Side Specifics

### Runtime

- **V8 only.** Rhino is dead — Google announced deprecation 2025-02-20; final shutdown 2026-01-31. Any tutorial showing `function.apply(this, arguments)` is Rhino-era and should be ignored.
- ES2020+ syntax (arrow functions, classes, template literals, async/await) is supported.

### Triggers

- **Daily wiki refresh:** `ScriptApp.newTrigger('refreshWiki').timeBased().everyDays(7).atHour(4).create()` — once a week, at 04:00 in script timezone.
- **Hourly/daily PigParse refresh:** `everyHours(6)` is a defensible default given PigParse rebuilds aggregates every 10 minutes server-side and our guild's price latency tolerance is hours, not minutes. Verify cadence with PigParse operator (project requirement says daily is the floor).
- **Each trigger function must complete in 6 minutes.** For the wiki refresh, store `lastProcessedPage` in `PropertiesService` and design the function to be re-entrant — if it bumps against the limit, it schedules itself to resume.

### Quotas (consumer Gmail accounts — relevant since guildies may not have Workspace)

| Quota | Limit | Implication |
|---|---|---|
| `UrlFetchApp` calls/day | **20,000** (consumer) / 100,000 (Workspace) | Plenty of headroom for our scrapes (we'll hit ~200/day even at the high end). |
| Single execution time | **6 minutes** | Drives our re-entrant scrape design. |
| Total trigger time/day | **90 minutes** | Non-issue at our scale. |
| `UrlFetch` GET response size | **50 MB** | Non-issue. |

### Sheet UI

- **Cell-note tooltips** (`range.setNote("...")`): inline, zero-cost, hover-to-reveal. Right tool for *short* per-row item info (quest indicator, summary blurb, current price). **Limitation:** plain text only, no links, no formatting, no images.
- **`HtmlService` sidebar** (`SpreadsheetApp.getUi().showSidebar(html)`): for the cross-character search bar and richer item detail (clickable wiki links, formatted price history). Sidebar is fixed 300px wide. Use this as the "second click" experience after a user finds something interesting in a cell-note.
- **Modal dialog** (`showModalDialog`): for one-off setup steps (e.g., "paste your bank toon's plat amount"), not as a primary UI.

### HTML Parsing in Apps Script

- **Don't.** Use the MediaWiki API (`action=parse` for HTML, `action=query&prop=revisions&rvprop=content` for raw wikitext). Confirmed enabled at `https://wiki.project1999.com/api.php`.
- For the few pages where the API output is messy, the community libraries `gas-commons/HtmlParser` and `tadaken3/html-parser-gas` wrap `XmlService.parse` with browser-like methods (`getElementById`, `getElementsByClassName`). **Both are unmaintained-but-working** — copy the source into our project rather than depending on them. **MEDIUM confidence** — verify they still work on current Apps Script V8.
- **Avoid raw regex on HTML** unless the field you want is genuinely simple (e.g., extracting `Item ID:&nbsp;(\d+)` from an item infobox). Regex on HTML is a known footgun even at small scale.

---

## Scraping Layer: Where Does It Run?

**Recommendation: scraping runs in Apps Script, not in the watcher and not as a separate scheduled job.**

Reasoning:
1. **Idempotency and ownership.** If scraping ran in the watcher, it would either run 12 times (every guildie scraping = rude to the wiki) or run on one designated guildie's machine (single point of failure when they're on vacation). Running it server-side in Apps Script means the Sheet's own owner credentials handle it, on Google's infrastructure, on a guaranteed schedule.
2. **No new infrastructure.** GitHub Actions or Cloudflare Workers would solve the "where does it run" problem but add a separate deployment surface, separate secrets, separate logs. Project explicitly wants "no separate server, database, or web app in v1."
3. **Quotas are sufficient.** 20,000 UrlFetch calls/day vs. our ~200 calls/day — two orders of magnitude of headroom.

**Polite-scraping checklist (implement in a single `politeFetch(url)` helper):**

- Set `User-Agent: SquireBot/1.0 (+https://github.com/<owner>/squirebot; contact: <email>)` — descriptive, contactable. P1999 wiki's `robots.txt` blocks AI crawlers but *does not* block normal bots; no crawl-delay specified, but we should self-impose ~1s between requests anyway.
- Honor `If-Modified-Since` / `ETag` — store last response's ETag in `PropertiesService` keyed by URL, send `If-None-Match` next time. 304 responses skip processing.
- 6-hour `CacheService.getScriptCache().put(url, body, 21600)` to absorb retry storms.
- Exponential backoff on 429 / 503 / 504. Honor `Retry-After`.

---

## PigParse: It's a Real REST API, Not Scraping

**This is a load-bearing finding** — verified at `https://pigparse.azurewebsites.net/swagger/index.html`.

PigParse exposes an OpenAPI/Swagger documented REST API. Relevant endpoints for SquireBot:

| Endpoint | Use For |
|---|---|
| `GET /api/item/getall/{server}` (server=1 for Blue) | Bulk pull of all currently-tracked items with average prices, rebuilt every 10 min server-side. **Use this** as the daily refresh — one HTTP call, done. |
| `GET /api/item/getmultiple/{server}?itemnames=...` | Targeted multi-item lookup. Useful for the v2 wantlist watcher. |
| `GET /api/item/getdetails/{itemid}` | Full historical price data **by item ID** — perfect since our join key is the EQ item ID, not the (drift-prone) name. Use for the per-item detail tooltip. |
| `POST /api/item/wiki` | PigParse will return wiki info for an item — **possible alternative to scraping the wiki directly**, worth evaluating in research before committing to scraping ourselves. |

**Implication for the scraping plan:** the PigParse "scraper" is not a scraper — it's a typed API client. No HTML parsing, no rate-limit risk beyond ordinary courtesy. **HIGH confidence** — verified the Swagger spec directly.

---

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative | Why Not Default |
|---|---|---|---|
| **Go** | **Rust + Tauri 2** | When you need a polished GUI (settings panel, log viewer, in-app dashboard). | Tauri pulls in WebView2 (~free on Windows 10/11 since it's pre-installed, but adds runtime dependency surface) and a bundled web stack. We have no GUI to speak of — a tray icon, a "click here" first-run page, and silent operation thereafter. Tauri is the right answer when the UI is the product; here the *Sheet* is the product, not the watcher. |
| **Go** | **.NET 9 + PublishAot + single-file** | Team is already C#-shaped; want WPF-quality UI; need direct Win32 COM. | .NET 9 Native AOT *can* produce a single-file binary without a runtime install (this was historically the killer flaw vs. Go), so it now meets the "no runtime install" bar. However: AOT trimming has subtle gotchas with reflection-heavy libraries (the Google .NET client uses some), AOT binaries are larger (~20-40 MB vs Go's ~12 MB), and the toolchain is heavier. Pick this only if there's a strong C# preference. |
| **Go** | **Python + PyInstaller** | Quick prototyping; team is Python-shaped. | PyInstaller bundles the interpreter (~50 MB), AV vendors flag PyInstaller bundles as suspicious at a meaningful rate (heuristic match against actual malware that abused PyInstaller), and OAuth refresh-token lifecycle is harder to make robust. **Violates the spirit of "idiot-proof"** — too many users will hit the SmartScreen / Defender warning. |
| **Go** | **Node + pkg / Electron / Electron-Builder** | The watcher needed an HTML-rendered UI. | Electron is a 80-150 MB binary for a tray app. `pkg` is largely abandoned. `nexe` exists but is niche. Wrong tool for this scope. |
| **`fyne.io/systray`** | **`getlantern/systray`** (original) | n/a | Effectively the same code; the Fyne fork is the maintained one. |
| **NSIS (per-user)** | **WiX / MSIX** | You need full per-machine deployment with Group Policy / SCCM. | Overkill for a 12-person guild. MSIX additionally requires a code-signing cert *just to install*, which is a worse UX than NSIS's "Allow this app to make changes" prompt that NSIS *avoids entirely* in per-user mode. |
| **NSIS** | **Inno Setup** | You like Pascal, you want a slightly nicer default look. | Equivalent quality. NSIS is more widely known and `goreleaser` integrates with it. Pick either — not a load-bearing decision. |
| **`drive.file` scope** | **`spreadsheets` scope** | The user does not yet have a workbook (you create it for them). | We *do* have a pre-existing shared workbook — `drive.file` after a Picker is the right model. `spreadsheets` triggers a sensitive-scope verification process for any non-trivial user count. |
| **Apps Script (V8) for sheet logic** | **Google Sheets API from the watcher only** | If you wanted no Apps Script at all and let the watcher do everything. | Then the search sidebar, scheduled scraping, and tooltip enrichment all need to live somewhere else (server, or one designated guildie's PC). Apps Script is the no-infrastructure win. |
| **Apps Script in Apps Script** | **Apps Script in TypeScript via clasp** | One-off scripts, prototyping. | TS catches enough bugs at compile time to be worth the build step for a 1000+ line project. |
| **MediaWiki API for wiki data** | **HTML scraping** | Specific pages don't expose what you need via the API. | API returns structured data (wikitext, categories, infobox templates) without the brittleness of HTML structure changes. |
| **PigParse REST API** | **Scraping PigParse HTML** | Their API breaks. | The API is documented and stable; scraping would be strictly worse on every axis. |

---

## What NOT to Use

| Avoid | Why | Use Instead |
|---|---|---|
| **Python + PyInstaller** | Bundles 50 MB Python interpreter; AV vendors heuristically flag PyInstaller `.exe`s; "click Run anyway" defeats the idiot-proof goal. | Go. |
| **Electron / Electron-Builder** for the watcher | 100+ MB binary for what's effectively a tray icon and a file watcher. Massive overkill. | Go + `fyne.io/systray`. |
| **Service-account JSON keys** | Already rejected by project — distributing a JSON key violates the idiot-proof setup constraint and is a serious security footgun (key rotation, key revocation, key leaking). | Per-user OAuth with `drive.file` + PKCE loopback. |
| **`oob` (out-of-band) OAuth flow** ("copy this code") | Google deprecated and removed it. Will not work for new clients. | Loopback redirect to `127.0.0.1:N`. |
| **Storing the OAuth refresh token in plaintext config or `%AppData%\config.json`** | Refresh tokens grant indefinite access; anyone with read on the file can impersonate the user against the Sheet. | Windows Credential Manager via `wincred` (DPAPI-encrypted, user-scoped). |
| **`spreadsheets` (or worse, `drive`) scope** | "Sensitive scope" — triggers Google verification process for distribution to >100 users (we're at 12, so technically OK, but the consent screen still says "see and edit *all* your spreadsheets" which is alarming). | `drive.file` scope + Google Picker. |
| **Apps Script Rhino runtime** | Deprecated 2025-02-20, fully removed 2026-01-31. Any project still on Rhino is broken or about to break. | V8 runtime (default for new scripts). |
| **`clasp` 1.x's TypeScript transpilation** | Removed/deprecated; clasp no longer transpiles TS. Tutorials saying "clasp handles TS for you" are stale. | Local transpile with `tsc` + bundle with `esbuild`, then `clasp push` the JS. |
| **Per-machine MSI installer requiring admin** | UAC prompt = scary popup for non-technical guildies. | Per-user NSIS install to `%LocalAppData%`, no UAC. |
| **Task Scheduler XML for autostart** | Fragile, requires admin to write to the system task store, surprisingly hard to debug for end users. | Run-key under `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`. |
| **Polling a directory with `time.Tick`** | Misses fast successive changes, wastes CPU, lags behind reality. | `fsnotify` with debounce. |
| **Trusting the `fsnotify` event payload's "size" or "modtime"** on Windows | Spurious events from AV, ordering with multiple writes is not guaranteed. | Treat events as "something changed, re-stat and re-read the file with a 500ms debounce." |
| **Scraping the wiki when MediaWiki's API is right there** | Brittle to layout changes; inefficient. | `api.php?action=parse` or `action=query`. |
| **Scraping PigParse HTML** | They publish a real Swagger-documented REST API. | The REST API. |

---

## Stack Patterns by Variant

**If a future v2 needs an always-on Discord bot:**
- The Apps Script "polling cron" pattern doesn't fit Discord (you want push, not pull). Move the bot to **Cloudflare Workers (free tier, 100k req/day)** or a **GitHub Actions scheduled workflow** invoking a small Go binary, and have the bot read the Sheet via the same Sheets API the watcher uses (with a *separate* OAuth client for the bot's own Google account).
- This is explicitly v2 scope per PROJECT.md and out of scope for stack research now.

**If signing certificates are a blocker:**
- Ship unsigned for the first weeks. Document the SmartScreen "More info → Run anyway" workaround in the README. Plan to add signing within ~30 days. SmartScreen reputation builds over time, so even an unsigned-but-stable binary becomes less scary after a few hundred installs (we won't have a few hundred — we have 12 — so signing matters more for us than for a popular OSS app).

**If a guildie uses a Microsoft account / OneDrive-mapped EQ folder:**
- `fsnotify` works on the local filesystem path even when that path is OneDrive-synced — the watcher sees local writes from EverQuest. No special handling needed. **MEDIUM confidence — verify with one OneDrive guildie during testing.**

---

## Version Compatibility

| Package A | Compatible With | Notes |
|---|---|---|
| Go 1.24.x | `google.golang.org/api/sheets/v4` (current) | Always works; Google client supports the last 2 major Go versions. |
| `fsnotify` v1.7+ | Go 1.21+ | Pre-1.7 had Windows reliability issues fixed in 1.7. Pin >= 1.7. |
| `fyne.io/systray` | Windows 10+ | Older Windows versions are not a concern (P99 client itself requires Win 7+, in practice ~all guildies are on Win 10/11). |
| Apps Script V8 | ES2020+ syntax | Optional chaining (`?.`), nullish coalescing (`??`), top-level `for...of`, `async`/`await` all work. **No** ES modules (`import`/`export`) at the runtime — esbuild bundles those away. |
| `@types/google-apps-script` | Apps Script V8 | Updated as Google ships new APIs; pin to a recent major. |
| `clasp` 3.x | Apps Script projects | 2.x is fine if 3.x has any regression in your environment; semantics are identical for our usage. |
| Code-signing cert | NSIS / `goreleaser` | `signtool.exe` is in the Windows SDK; integrates cleanly into `goreleaser`'s `signs` block. |

---

## Confidence Assessment

| Recommendation | Confidence | Why |
|---|---|---|
| Go for the watcher | **HIGH** | Single-binary distribution + mature Google client + no runtime is a textbook fit. Verified against Google's quickstart and the OAuth-apps-for-windows samples. |
| `drive.file` over `spreadsheets` scope | **HIGH** | Verified directly in current Google docs. Non-sensitive vs. sensitive scope distinction is canonical. |
| Loopback + PKCE OAuth pattern | **HIGH** | Verified in current Google docs that loopback is *kept* for desktop while deprecated for mobile/Chrome. |
| `wincred` for token storage | **HIGH** | Standard pattern; DPAPI-backed, user-scoped, no exotic dependencies. |
| `fsnotify` for file watching | **HIGH** | Industry standard; Windows-specific gotchas are known and small for our use case (we re-read the file rather than trusting payloads). |
| Apps Script V8 + clasp + TS | **HIGH** | V8 mandate is documented; clasp is Google's own tool. |
| PigParse has a usable REST API | **HIGH** | Directly verified the Swagger spec; itemid lookup endpoint exists. |
| MediaWiki API works on P1999 wiki | **HIGH** | Directly probed `https://wiki.project1999.com/api.php` and got the help page back, confirming `action=parse` and `action=query` are exposed. |
| `fyne.io/systray` is the right systray lib | **MEDIUM** | It works and is maintained, but the systray library landscape is fragmented; `getlantern/systray` and `tailscale/systray` are alternatives. Any of them is fine. |
| Polite-scraping cadence (weekly wiki, daily PigParse) | **MEDIUM** | Defensible defaults; verify with the maintainers before going live (PROJECT.md already calls this out as a "be a good citizen" expectation). |
| NSIS per-user install pattern | **MEDIUM-HIGH** | Well-documented; the only risk is SmartScreen reputation building, which is solved by code signing. |
| `minio/selfupdate` for auto-update | **MEDIUM** | Library is real and active, but the auto-update happy-path on Windows (replace-running-exe) has historic gotchas (file locks). The library handles them, but plan integration testing time. |
| Code-signing cert pricing/process | **MEDIUM** | Pricing fluctuates; verify at purchase. The recommendation that signing matters is HIGH confidence. |

---

## Sources

- [Google: OAuth 2.0 for iOS & Desktop Apps](https://developers.google.com/identity/protocols/oauth2/native-app) — confirmed loopback redirect + PKCE is the current desktop flow
- [Google: Loopback IP Address flow Migration Guide](https://developers.google.com/identity/protocols/oauth2/resources/loopback-migration) — confirmed loopback is *kept* for desktop while deprecated for native iOS/Android/Chrome
- [Google Sheets Go quickstart](https://developers.google.com/workspace/sheets/api/quickstart/go) — installation + auth pattern verified
- [google-api-go-client GitHub](https://github.com/googleapis/google-api-go-client) — first-party, actively maintained
- [Google: Choose Sheets API scopes](https://developers.google.com/workspace/sheets/api/scopes) — `drive.file` non-sensitive vs. `spreadsheets` sensitive distinction
- [fsnotify package docs](https://pkg.go.dev/github.com/fsnotify/fsnotify) — Windows backend uses `ReadDirectoryChangesW`; documented Chmod limitation; buffer-size config
- [danieljoos/wincred](https://github.com/danieljoos/wincred) — Windows Credential Manager wrapper, DPAPI-backed
- [zalando/go-keyring](https://github.com/zalando/go-keyring) — alternative cross-platform wrapper (uses wincred internally)
- [minio/selfupdate](https://github.com/minio/selfupdate) — active fork of the inconshreveable/go-update lineage
- [fyne.io/systray](https://pkg.go.dev/fyne.io/systray) — maintained fork of getlantern/systray
- [Tauri 2 docs](https://v2.tauri.app/) — verified 2.x stable (Oct 2024 GA, current in 2026); used to evaluate the alternative
- [.NET Native AOT overview](https://learn.microsoft.com/en-us/dotnet/core/deploying/native-aot/) — used to evaluate the .NET 9 alternative
- [Apps Script Quotas for Google Services](https://developers.google.com/apps-script/guides/services/quotas) — UrlFetchApp limits (20K consumer / 100K Workspace), 6-min execution cap, 90-min daily trigger cap
- [Apps Script V8 runtime announcement](https://workspace.google.com/blog/developers-practitioners/data-processing-just-got-easier-apps-scripts-new-v8-runtime) and follow-up Rhino EOL announcements — V8 mandatory by 2026-01-31
- [Apps Script HtmlService](https://developers.google.com/apps-script/guides/html) — sidebar 300px width, modal dialog patterns
- [Apps Script Dialogs and Sidebars](https://developers.google.com/apps-script/guides/dialogs) — UI surface options
- [google/clasp](https://github.com/google/clasp) — first-party Apps Script CLI; confirmed clasp no longer transpiles TS itself
- [@types/google-apps-script + esbuild starter](https://github.com/sqrrrl/apps-script-typescript-rollup-starter) — pattern for TS-authored Apps Script
- [PigParse Swagger UI](https://pigparse.azurewebsites.net/swagger/index.html) — **directly probed**; confirmed REST API with `/api/item/getall/{server}`, `/api/item/getdetails/{itemid}`, `/api/item/getmultiple`, `/api/item/wiki` endpoints
- [P1999 wiki robots.txt](https://wiki.project1999.com/robots.txt) — **directly probed**; only AI crawlers blocked; no crawl-delay
- [P1999 wiki api.php help page](https://wiki.project1999.com/api.php?action=help) — **directly probed**; MediaWiki API enabled, `action=parse` and `action=query` available
- [MediaWiki API:Etiquette](https://www.mediawiki.org/wiki/API:Etiquette) — User-Agent + caching guidance applied to our `politeFetch`
- [NSIS per-user install patterns](http://rickdrizin.com/NSIS-and-Windows-Installers-for-MultiUser-Current-User) — `RequestExecutionLevel user` + `$LOCALAPPDATA` install path
- [Go release history](https://go.dev/doc/devel/release) — confirmed 1.24.13 current as of Feb 2026

---
*Stack research for: SquireBot (Windows watcher + Google Sheets + light scraping)*
*Researched: 2026-04-30*
