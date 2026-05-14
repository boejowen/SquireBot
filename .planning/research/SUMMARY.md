# Project Research Summary — SquireBot

**Project:** SquireBot
**Domain:** Per-guildie Windows watcher (Go) + shared Google Sheets workbook (Apps Script TS) + light scraping (P1999 MediaWiki API, PigParse REST). 12-user P99 guild scale.
**Researched:** 2026-04-30
**Confidence:** HIGH overall.

---

## Executive Summary

SquireBot is the first **automated multi-user** P99 inventory tool — every existing one (EQHTML, WinEQDB, P99 Inventory Parser, EQ1999-Bank, P99 Companion) is single-user; the realistic competition is a **manually-maintained Google Sheet**. The product wins on three differentiators that nothing else in the ecosystem combines: **guild-wide aggregation** (automatic), **wiki-driven gear/spell progression checklists** vs. Velious tier pages (character-aware), and **live PigParse pricing on every inventory row**. v2 (Discord pinger) is correctly deferred because it is gated on three external prerequisites — Raid Alliance bot invites, Discord identity collection, and PigParse access confirmation — none of which are settled.

The recommended build is a **Go 1.24 single-binary watcher** (NSIS per-user installer, no UAC, autostart via `HKCU\...\Run`) using **OAuth loopback + PKCE with `drive.file` scope**, refresh tokens in **Windows Credential Manager via `wincred`**, and `fsnotify` with a 500ms debounce that re-reads the file (never trusts event payload). The watcher writes **full-snapshot replaces** of per-character landing tabs (`inv:<CharName>`, `spell:<CharName>`) via `spreadsheets.batchUpdate` with atomic clear+write — never appends, never row-diffs. Apps Script (V8, authored in TypeScript via `clasp` + `esbuild`) owns everything else: dimension tabs (`_item_master`, `_pigparse`, `_wiki_*`, `_char_owner`, `_meta`), view tabs, an `HtmlService` search sidebar, and weekly/daily refresh triggers against the **MediaWiki API** (`api.php?action=parse`) and the **PigParse REST API** (`/api/item/getall/1` — verified Swagger; the "PigParse scraper" is actually a typed JSON client, not HTML scraping).

Five pitfalls are existential and must be designed against in Phase 1–2: **(1) OAuth Testing-mode 7-day refresh-token expiry** (fix: publish to Production before shipping; with `drive.file` non-sensitive scope, no Google verification audit is required); **(2) SmartScreen "Unknown publisher" wall** (fix: EV code-signing cert ~$300–600/yr OR a pre-recorded "More info → Run anyway" walkthrough); **(3) concurrent-write lost updates** (fix: per-character non-overlapping ranges + `batchUpdate` atomicity; no shared mutable ranges from watchers); **(4) stale-data trust collapse** (fix: per-character `last_synced` timestamp surfaced prominently with conditional-formatting age coloring + watcher heartbeat once daily even when file unchanged); and **(5) the proposed per-character view tab layout breaches Google's hard 200-tabs-per-workbook limit at our scale** — 12 guildies × ~10 chars × ~5 view types ≈ 600 tabs. **Architecture resolution: keep landing tabs per-character (~120 tabs max, well under limit), but consolidate views into 3–5 filterable mega-tabs (`view`, `gear_check`, `spell_check`, `bank`) with a Char column and dropdown filters.** This is non-negotiable and must be locked in the Phase 2 schema.

---

## Key Findings

### Recommended Stack

Single-binary Windows watcher in **Go**, sheet-side logic in **Apps Script V8 (TypeScript)**, no separate server in v1. All scraping runs in Apps Script (server-side, single-source-of-truth) — never in 12 watchers in parallel. PigParse and the P1999 wiki both expose proper APIs, so there is no HTML scraping in v1.

**Core technologies (LOCKED):**

- **Go 1.24.x** — watcher language. Single statically-linked Windows `.exe` (~12 MB). Build: `GOOS=windows GOARCH=amd64 go build -ldflags="-H=windowsgui -s -w"`.
- **`google.golang.org/api/sheets/v4`** + **`golang.org/x/oauth2`** + **`oauth2/google`** — Sheets API v4 client + canonical OAuth library with PKCE.
- **`github.com/fsnotify/fsnotify` v1.7+** — file-system events (Windows: `ReadDirectoryChangesW`). Always re-stat and re-read on event; never trust event payload. 500ms per-path debounce.
- **`github.com/danieljoos/wincred` v1.2.x** — DPAPI-backed Credential Manager wrapper. Refresh token under target name `SquireBot:<google-email>`, persistence `LocalMachine`. Plaintext config storage forbidden.
- **`github.com/minio/selfupdate`** — auto-update; `latest.json` manifest with SHA-256 in GitHub Releases; startup-swap pattern; atomic-rename handles Windows file-locking.
- **`fyne.io/systray` v1.10+** — tray icon + menu (Quit, Open Setup, status). Only watcher UI.
- **NSIS 3.10+** — installer. `RequestExecutionLevel user`, install to `$LOCALAPPDATA\Programs\SquireBot`, autostart via `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`. **No UAC, no Task Scheduler, no Windows Service.**
- **Apps Script V8 runtime** (mandatory; Rhino EOL 2026-01-31) authored in **TypeScript 5.x** + **esbuild 0.20+** + **`@types/google-apps-script`**, deployed via **`clasp` v3.0+** (clasp 3.x no longer transpiles TS itself; we transpile locally).
- **MediaWiki API** at `https://wiki.project1999.com/api.php` — `action=parse&prop=wikitext`. Verified enabled.
- **PigParse REST API** at `https://pigparse.azurewebsites.net` — Swagger-documented, not a scrape target. `/api/item/getall/1` (server=1=Blue) once daily.
- **Code-signing certificate (EV preferred, ~$300–600/yr)** — strongly recommended; without it ~30% of guildies hit SmartScreen.

**Supporting libraries:** `log/slog` (stdlib), `gopkg.in/natefinch/lumberjack.v2` (5MB×3 rotation), Apps Script `CacheService` (6h PigParse, 7d wiki), `PropertiesService` (cursor state, ETags).

**Scopes (LOCKED):** `https://www.googleapis.com/auth/drive.file` only — non-sensitive, no Google verification audit. **Never** `spreadsheets` or `drive`. Workbook selection on first run via Google Picker (mandatory under `drive.file`).

**What NOT to use:** Python+PyInstaller, Electron, service-account JSON keys, `oob` OAuth flow (deprecated), `spreadsheets` scope (sensitive), Apps Script Rhino runtime (EOL), clasp 1.x bundled TS transpilation (removed), per-machine MSI requiring admin, Task Scheduler XML for autostart, polling with `time.Tick`, trusting `fsnotify` event payloads on Windows, HTML-scraping the wiki, scraping PigParse.

### Expected Features

**Must have (table stakes):**

- One-click NSIS installer + per-guildie Google OAuth (loopback PKCE, `drive.file`, Picker on first run)
- Inventory file watcher (`<CharName>-Inventory.txt`, 5-column TSV `Location | Name | ID | Count | Slots`)
- Spellbook file watcher (`<CharName>-Spellbook.txt`, 2-column `Slot | Name` — verify on first sample; spellbook has NO IDs, normalized name is the only join key)
- Per-character inventory and spellbook views
- Cross-character item search (HtmlService sidebar, guild-wide)
- Shared bank character view + cross-character search inclusion
- Manual coin field on bank toon (`/outputfile inventory` does NOT contain coin amounts — locked by file format)
- Direct wiki link on every item row
- Item ID as canonical join key (names drift, IDs are stable)
- Watcher autostarts on Windows logon
- No data loss on watcher restart

**Should have (differentiators that justify SquireBot existing):**

- Wiki-driven gear progression checklist (Velious Pre-Raid + Velious Raiding + Iksar tiers, per-class, joined against equipped slots; per-slot "shopping list")
- Wiki-driven spell progression checklist (level-aware, scraped per class)
- Live PigParse pricing on every inventory row (daily `/api/item/getall/1` cached, denormalized into `_item_master`)
- Item tooltips combining wiki summary + quest indicator + price (cell-note for short hover, sidebar for rich detail — two-tier UX locked)
- Quest-item awareness from wiki categories

**Defer (v2+; gated on three prerequisites):**

- Per-user wantlist
- EC tunnel auction monitor via PigParse `/api/item/getmultiple` polling (NOT log-parsing)
- WTS monitor across three Raid Alliance Discord servers — **blocked on bot admin invites that have not been negotiated**
- Quest-target raid monitor — blocked on Discord invites + curated `quest → raid target NPC` lookup

**Anti-features (explicitly never):** DKP/loot council, real-time inventory diffing alerts, Magelo-style public char pages, privacy tiers in v1, mobile native app, other servers (Green/Red/live EQ), service-account auth, in-game overlays.

### Architecture Approach

**Three-layer pancake inside the workbook:**

1. **Landing tabs (watcher-written, one per character per file type):** `inv:<CharName>` (cols `A:Location, B:Name, C:ID, D:Count, E:Slots, F:_uploaded_at`), `spell:<CharName>` (cols `A:Level, B:Name, C:_uploaded_at`). Watcher writes full-snapshot replace via `batchUpdate` (atomic clear `A2:F` + write `A1:F<N+1>`). Per-character non-overlapping ranges; no shared mutable ranges from any watcher.
2. **Dimension tabs (Apps Script-written, hidden, `_`-prefixed):** `_meta` (schema_version, bank_toon_name, bank_coin_pp/gp/sp/cp, last_*_refresh, canonical_id), `_char_owner` (char→email mapping with class/level/discord_handle/is_bank_toon/is_hidden/is_removed), `_item_master`, `_pigparse`, `_wiki_spells`, `_wiki_gear_tier`, `_quest_items`, `_audit` (upload provenance), `_status` (watcher version reporting).
3. **View tabs (Apps Script-written, visible, CONSOLIDATED — see critical revision below):** `view` (all characters, filterable by Char column), `gear_check` (all chars), `spell_check` (all chars), `bank` (single special view of `_meta.bank_toon_name`'s data + coin row), search sidebar (HtmlService, not a tab).

**CRITICAL schema revision driven by PITFALLS research:** the architecture research's per-character view tab layout (`view:<CharName>`, `gear_check:<CharName>`, `spell_check:<CharName>`) breaches Google's 200-tab/workbook hard limit at guild scale: 12 × ~10 × ~5 ≈ **600 tabs**. **Resolution (LOCKED): keep landing tabs per-character (~120 tabs combined, comfortable under limit) but consolidate all views into single filterable mega-tabs with a leading Char column.** This is the schema the Phase 2/3 planners must build against; the per-char view-tab proposal in ARCHITECTURE.md is superseded.

**Key contracts:**

- **Watcher → Sheet write contract:** full-snapshot replace via `batchUpdate` atomic clear+write per character per file. `valueInputOption=RAW` (never `USER_ENTERED` for hot path — recalc storms). Idempotent by construction.
- **Watcher → `_char_owner` upsert:** watcher includes its own OAuth `userinfo.email` as identity signal (Apps Script's `Session.getActiveUser().getEmail()` returns script owner, not writer). First-write wins on `owner_email`; mismatch surfaces in `_audit`.
- **Sheet IS the API contract between watcher and Apps Script.** No direct invocation; communication via tab writes triggering `onChange`.
- **Schema evolution:** never break, only extend (add columns at right edge; add new tabs; add new `_meta` rows). Breaking changes require `schema_version` bump + idempotent migration + watcher's `WATCHER_MAX_SCHEMA_VERSION` check + script's `SCRIPT_MIN_SCHEMA_VERSION` check.

**Configuration split:** per-watcher state in `%LOCALAPPDATA%\SquireBot\config.json` (`eq_folder` array for multibox, `spreadsheet_id`, `google_email`, `last_known_inventory_mtime` for catch-up on restart, `log_level` — refresh token NEVER lives here, only wincred). Workbook-universe state in `_meta`. Per-character state in `_char_owner`.

### Critical Pitfalls

PITFALLS.md catalogues 27 pitfalls; the five existential ones drive Phase 1–2 design:

1. **OAuth consent screen left in Testing mode** — refresh tokens silently expire every 7 days for non-Workspace users. **Fix (Phase 1):** flip to **Production** before first guildie installs. With `drive.file` (non-sensitive), Production publication does **not** require Google's third-party verification audit. Verify: wait 10 days post-OAuth on a test install.
2. **SmartScreen "Unknown publisher" wall** — full-screen blue panel; ~30% abandon. **Fix (Phase 1):** EV cert (~$300–600/yr + hardware token) is best; OV (~$100–200/yr) won't accumulate reputation at 12 users; **unsigned + a 30-second pre-recorded "More info → Run anyway" walkthrough** is budget fallback for the first month with a 30-day signing target. **Self-signed certs are worse than unsigned.**
3. **Concurrent writes from 12 watchers cause lost-update bugs.** **Fix (Phase 2):** every watcher writes ONLY to its own per-character non-overlapping ranges; aggregates computed in Apps Script using `LockService.getDocumentLock().tryLock(30000)` with `try/finally`. Sheets API quota is 60/min/user — our load is ~120/day total.
4. **Stale-character data with no freshness signal.** **Fix (Phase 2 schema, Phase 3 UI):** per-character `inventory_mtime` AND `last_sync_attempt` columns; conditional formatting orange after 14d, red after 30d; daily watcher heartbeat (one-cell write even when file unchanged); search sidebar shows staleness inline; auto-archive characters with `inventory_mtime > 90d`.
5. **Per-character view tab layout exceeds Google's 200-tab hard limit** (12 × 10 × 5 ≈ 600 tabs). **Fix (Phase 2 schema — supersedes ARCHITECTURE.md):** keep landing tabs per-character, consolidate views into single filterable mega-tabs.

**Other high-severity pitfalls:** `drive.file` Picker semantics misuse (Phase 1), per-machine UAC installer (Phase 1), refresh-token failure UX (Phase 2), PigParse rate-limiting (Phase 3), MediaWiki etiquette → IP block (Phase 3), Apps Script 6-min execution timeout (Phase 3), workbook 10M cell cap (Phase 2 design, Phase 4 monitoring), recalc storms on bulk inventory writes (Phase 2), Latin-1 / Windows-1252 encoding (Phase 1), EQ folder discovery (Phase 1), OneDrive KFM + DPAPI cross-machine fragility (Phase 1), loopback redirect blocked (Phase 1), wiki page structure changes silently breaking parser (Phase 3).

---

## Implications for Roadmap

5-phase plan, P1 = end-to-end thin slice (installer + OAuth + raw landing tab) so existential pitfalls 1, 2, and 5 are validated **before** further work compounds on a broken foundation.

### Phase 1 — End-to-end thin slice (installer + OAuth + raw landing tab)

**Delivers:** a single guildie can install SquireBot, OAuth once, point at their EQ folder, run `/outputfile inventory`, and see raw TSV rows appear in `inv:<CharName>` within seconds.

**Components:** Go module + goreleaser + GitHub Actions release; NSIS per-user installer; OAuth loopback PKCE on random ephemeral port + `drive.file` + 127.0.0.1 + manual-paste fallback; Drive Picker on first run; wincred token storage; Sheets v4 client with batchUpdate clear+write; fsnotify + 500ms debounce + Windows-1252 decoder + 5-col tolerant TSV parser; EQ folder multi-strategy auto-discovery; minimal `_meta` and `_char_owner` init; lumberjack log rotation; tray icon + status; **OAuth consent screen flipped to Production**.

**Avoids existential pitfalls:** #1 OAuth Testing-mode, #2 SmartScreen, #5 `drive.file` Picker.
**Exit criteria:** clean Win11 VM install, no UAC, no SmartScreen wall (or rehearsed walkthrough), OAuth completes, Picker selects workbook, `/outputfile inventory` produces an `inv:<Char>` tab within 30s. Re-test 10 days later: writes still work.

### Phase 2 — Watcher robustness + sheet schema foundation

**Delivers:** one guildie can install the watcher and not touch it for 6 months. Sheet schema is locked.

**Components:** Spellbook file watcher; autostart on logon (`HKCU\...\Run`); retry/backoff on Sheets errors (2/4/8/16/32/60s exp); refresh-token failure UX (tray red, click reopens OAuth); daily heartbeat write; auto-update pipeline (selfupdate, startup-swap, SHA-256, latest.json); code signing (EV if budget; documented walkthrough fallback + 30-day signing target); **Sheet schema LOCKED** (schema_version=1, full `_char_owner` shape including `is_hidden`/`is_removed`, composite key `<owner_email>:<char_name>`, per-character `inventory_mtime` + `last_sync_attempt`, **consolidated mega-tabs decision recorded in PROJECT.md**); tab IDs over names; `_audit` provenance; soft-delete fields scaffolded.

**Avoids existential pitfalls:** #3 concurrent-write lost updates, #4 stale-data trust collapse, #5 view-tab limit breach.
**Exit criteria:** watcher survives a 7-day continuous run with manual `invalid_grant` injection, deliberate update corruption, Standard-User account install, OneDrive-KFM-enabled box, two PCs as same user, a Velious item with apostrophe in the name. Schema migration smoke-test.

### Phase 3 — Apps Script enrichment foundation (item dimension + first views)

**Delivers:** a watcher upload becomes a tooltipped, priced, wiki-linked row in the consolidated `view` tab within ~30 seconds.

**Components:** TypeScript + esbuild + clasp scaffolding; `schema_migrate.ts` with `_meta.canonical_id` workbook-copy guard; `politeFetch(url)` helper (UA, ETag, CacheService, exp backoff, Utilities.sleep); `refresh_pigparse.ts` (daily 03:00 PT, full replace `_pigparse`, denormalize price into `_item_master`); `refresh_wiki.ts` (item-summaries, re-entrant cursor, schema assertions, weekly trigger); `_item_master` builder; installable `onChange` → `build_views.ts` rebuilds affected character's rows in consolidated `view` (with cell-note tooltip composing summary + price + quest indicator); `bank` tab; manual rebuild escape hatch; courtesy contact to PigParse + wiki admins **before live**.

**Avoids high pitfalls:** PigParse rate-limit, wiki etiquette, 6-min timeout (re-entrant cursor), wiki structure change (assertions + last-known-good).
**Exit criteria:** end-to-end test — watcher upload → 30s later → consolidated `view` row shows wiki link, current PigParse price, quest indicator, hover-summary cell note. Wiki scrape interrupted mid-run resumes from cursor.

### Phase 4 — Differentiator features (gear + spell progression checklists)

**Delivers:** every guildie sees a per-slot "shopping list" of missing Velious gear and a level-aware list of trainable spells they don't yet know. **The reason SquireBot exists.**

**Components:** `_wiki_gear_tier` scrape (Velious Pre-Raid + Velious Raiding + Iksar pages, per-class subpages, slot-by-slot rows); `_wiki_spells` scrape (per-class spell list pages, composite key `(class, level, spell_name)`, normalized name for spellbook join — spellbook has no IDs); sidebar form for `_char_owner.class` and `_char_owner.level`; consolidated `gear_check` mega-tab builder (joined from `inv:<Char>` slot equipment × `_wiki_gear_tier` for char's class × `_item_master` for price; Status = OK/MISSING/OTHER); consolidated `spell_check` mega-tab builder; manual coin update sidebar form for `bank` tab (writes to `_meta.bank_coin_*`); workbook cell count monitoring; cross-version migration testing harness.

**Exit criteria:** for the developer's own characters, `gear_check` shows accurate MISSING rows for Velious tiers; `spell_check` shows accurate trainable-not-yet-learned spells. Wiki scrape covers all 14 classes within trigger budget. `bank` tab shows full inventory + coin row.

### Phase 5 — Search + onboarding + privacy polish

**Delivers:** all 12 guildies running SquireBot. Cross-character "who has Lustrous Russet Coat?" answered in the sidebar. Privacy soft-delete process documented and tested.

**Components:** HtmlService search sidebar (300px, in-memory join across `inv:*` cached 60s, results show `<Item> (id <id>) → <Char>: <Location>, count <N>` with last-sync staleness inline); custom `onOpen` menu polish; hide `_*` system tabs by default; `Range.protect()` where mutation is dangerous; weekly schema healthcheck trigger; documented eviction workflow (remove guildie email from share → mark all their characters `is_removed` → 30-day grace → archive); onboarding doc + screenshots + SmartScreen walkthrough video; auto-archive characters with `inventory_mtime > 90d`; v2 prep: `_char_owner.discord_handle` populatable but no behavior yet.

**Exit criteria:** all 12 guildies installed and writing data; cross-character search returns hits in <2s; eviction workflow tested end-to-end on a fake guildie account.

### Phase Ordering Rationale

- **Phase 1 must be end-to-end** — three of five existential pitfalls live at the install/OAuth seam; one (Testing-mode 7-day expiry) is silent for a week. Validating in Phase 1 with a 10-day re-test cycle is cheap.
- **Phase 2 ringfences schema** — last-updated timestamps, composite keys, soft-delete fields, the consolidated-view-tab decision, tab IDs vs names — cheap at design time, painful migrations after data lands.
- **Phase 2 and Phase 3 are technically parallelizable** (Go vs TS, no shared code paths) — useful note if there are ever two contributors.
- **Phase 3 vs Phase 4 split** — both share scrape harness and `_item_master`; splitting lets Phase 4 deliver the headline differentiator features rather than feature-plus-plumbing.
- **Phase 5 is small but blocking for rollout.** Onboarding the other 11 guildies is the validation gate for v1.

### Research Flags

**Phases likely needing deeper research during planning (`/gsd-research-phase`):**

- **Phase 2** — code-signing certificate procurement: vendor selection, EV vs OV, hardware token logistics, integration with goreleaser, total annual cost.
- **Phase 3** — wiki page structure for per-class spell lists and per-item summary parsing not directly probed; verify template shapes via `api.php?action=parse&prop=wikitext` before code is written. Confirm PigParse `/api/item/getall/1` JSON shape end-to-end with a real curl.
- **Phase 4** — gear-tier wiki pages (`Players:Velious_Pre-Raid_Gear`, `Players:Velious_Raiding_Gear`, `Iksar`) have known multi-page nested-table structures that vary by class. Research the actual template/markup shape and produce a parser spec before implementation. The "gear progression checklist" is the headline differentiator; a parser regression here is high-blast-radius.

**Phases with standard patterns (skip phase-research):** Phase 1, Phase 5.

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Every external API directly probed (PigParse Swagger, P1999 wiki `api.php`, robots.txt). MEDIUM only on systray library choice and code-signing pricing. |
| Features | HIGH | Existing P99 tooling landscape characterized exhaustively. v2 Discord-pinger feature shape is MEDIUM because Raid Alliance admin access has not been validated. |
| Architecture | HIGH | Sheet schema drives directly off the (known) file format and (verified) wiki/PigParse shapes. The CRITICAL revision (consolidated view tabs vs per-char) is high-confidence — Google's tab limit is a documented hard cap. |
| Pitfalls | HIGH | OAuth Testing-mode 7-day expiry verified against current Google docs; SmartScreen verified against Microsoft Learn; Sheets API quotas verified March 2026. |

**Overall confidence:** HIGH.

### Gaps to Address

- **Code-signing certificate budget and procurement** — concrete decision (EV / OV / unsigned-with-walkthrough) needed before Phase 1 release pipeline; aim to flip to signed within 30 days of v1 launch.
- **Wiki tier-page parser specifics** — `Players:Velious_Pre-Raid_Gear` etc. nested per-class structures not concretely sampled. Phase 3/4 research should produce a parser spec from real wikitext samples.
- **PigParse operator courtesy contact** — before Phase 3 daily polling goes live.
- **MediaWiki admin courtesy contact** — before Phase 3 weekly scrape goes live.
- **Spellbook file format verification** — assumed 2-column `Slot | Name`; verify on first sample before locking schema in Phase 2. (No IDs in spellbook is verified.)
- **OneDrive-KFM-enabled testing** — needs at least one real guildie test box during Phase 1/2 validation.
- **EV vs OV cert pricing at purchase time** — market fluctuates.
- **v2 prerequisite resolution** — Raid Alliance bot admin invites, Discord identity collection mechanism, PigParse access mode confirmation. Track as v1.5+ unblocking work outside the v1 critical path.

---

## Sources

### Primary (HIGH confidence — directly probed)

- [PigParse Swagger UI](https://pigparse.azurewebsites.net/swagger/index.html)
- [P1999 wiki MediaWiki API](https://wiki.project1999.com/api.php)
- [P1999 wiki robots.txt](https://wiki.project1999.com/robots.txt)
- [Google: OAuth 2.0 for iOS & Desktop Apps](https://developers.google.com/identity/protocols/oauth2/native-app)
- [Google: Loopback IP Address flow Migration Guide](https://developers.google.com/identity/protocols/oauth2/resources/loopback-migration)
- [Google: Choose Sheets API scopes](https://developers.google.com/workspace/sheets/api/scopes)
- [Google: Manage App Audience (Testing → Production)](https://support.google.com/cloud/answer/15549945)
- [Google Sheets API: Usage limits](https://developers.google.com/workspace/sheets/api/limits)
- [Apps Script: Quotas for Google Services](https://developers.google.com/apps-script/guides/services/quotas)
- [Apps Script: Lock Service](https://developers.google.com/apps-script/reference/lock)
- [Microsoft Learn: SmartScreen reputation](https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/smartscreen-reputation)
- [Microsoft Learn: DPAPI MasterKey backup failures](https://learn.microsoft.com/en-us/troubleshoot/windows-server/certificates-and-public-key-infrastructure-pki/dpapi-masterkey-backup-failures)
- [EQHTML — P99 Wiki](https://wiki.project1999.com/EQHTML)
- [WinEQDB — P99 Wiki](https://wiki.project1999.com/WinEQDB)
- [P99 Inventory Parser](https://www.cs.mun.ca/~dchurchill/eq/inventory/)
- [TunnelQuestBot](https://github.com/jamesjamail/TunnelQuestBot)

### Sibling research

- `.planning/research/STACK.md`
- `.planning/research/FEATURES.md`
- `.planning/research/ARCHITECTURE.md` (whose per-char view tab proposal is **superseded** by the consolidated-view decision in this summary)
- `.planning/research/PITFALLS.md`
- `.planning/PROJECT.md`

---

## SYNTHESIS COMPLETE

Suggested phases: **5** (end-to-end thin slice → watcher robustness + schema lock → Apps Script enrichment foundation → differentiator checklists → search + onboarding + privacy polish).

**Critical cross-document override:** ARCHITECTURE.md's per-character view-tab layout is **superseded** by consolidated filterable mega-tab views. Lock in PROJECT.md Key Decisions during Phase 2.

Ready for requirements.
