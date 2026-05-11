# SquireBot

## What This Is

SquireBot is a small Windows app that every member of a ~12-person Project 1999 (Classic EverQuest emulator) guild installs on their PC. It watches the EQ folder for the tab-separated text files produced by the in-game `/outputfile inventory` and `/outputfile spellbook` commands, and pushes their contents into a single shared Google Sheet. The sheet is the real product — it joins each guildie's character data with information scraped from the [P1999 wiki](https://wiki.project1999.com/) and prices from [PigParse](https://pigparse.azurewebsites.net) to give the guild a unified view of every character's gear, spells, progression toward Velious-tier readiness, the shared bank's contents, cross-character search, and item tooltips.

## Core Value

**Every guildie can answer "what does my character still need, and where in the guild is it?" without leaving the spreadsheet.** Inventory and spell data lands in the sheet automatically; progression, gaps, and prices are computed for them. If everything else fails, this must work.

## Current State

**Shipped: v1.0.0 (2026-05-11).** Five phases, 31 plans, 11 days kickoff to ship. Watcher v0.4.0 binary + Apps Script Phase-5 bundle deployed via clasp to the dev workbook. Tag `v1.0.0` pushed; Pages onboarding site live at https://boejowen.github.io/SquireBot/. See `.planning/MILESTONES.md` and `.planning/milestones/v1.0-ROADMAP.md` for the full v1.0 record.

**What's working end-to-end (verified):** Installer → OAuth → watcher → inv/spell landing tabs → daily PigParse + weekly wiki enrichment → consolidated `view` + `gear_check` + `spell_check` + `bank` tabs with cell-note tooltips and conditional Last-Synced formatting → cross-character search sidebar → lock-guarded eviction workflow with 30-day grace → text-only Pages onboarding site. All cumulative tests pass (297/297 vitest, full Go test suite). Schema at `_meta.schema_version = 3`; watcher's `WatcherMaxSchemaVersion = 3`; ErrSchemaTooNew startup gate verified.

**Next milestone:** undefined. Either v1.0.1 (patch — installer overwrite-running shim, sidebar test setup, SignPath OSS resolution) or v1.1 (feature — TBD). Start via `/gsd-new-milestone`.

## Requirements

### Validated

<!-- Shipped and confirmed valuable in v1.0. -->

**v1 — Core watcher and shared sheet (shipped 2026-05-11 as v1.0.0)**

- ✓ Idiot-proof Windows installer — single .exe, no UAC, click-Allow-twice setup — v1.0
- ✓ Per-guildie Google OAuth with `drive.file` non-sensitive scope, Production consent screen — v1.0
- ✓ Inventory file watcher + spellbook file watcher (fsnotify, 500ms debounce, multi-folder) — v1.0
- ✓ Wiki scraper for per-class spell lists + Velious gear tiers + per-item summaries + quest items — v1.0
- ✓ PigParse REST API daily price refresh — v1.0
- ✓ Per-character inventory view (consolidated mega-tab with Char filter) — v1.0
- ✓ Per-character spellbook view + spell progression checklist (`spell_check` KNOWN/MISSING) — v1.0
- ✓ Per-character gear progression checklist (`gear_check` OK/MISSING/OTHER vs Velious tiers) — v1.0
- ✓ Shared bank character view + manual platinum/gold/silver/copper sidebar (Range.protected) — v1.0
- ✓ Shared bank cross-character search (300px HtmlService sidebar, <2s, CacheService 60s) — v1.0
- ✓ Item tooltips on every inventory row (cell-note: wiki summary + price + quest info) — v1.0
- ✓ Direct wiki link on every item — v1.0
- ✓ Universal visibility (every guildie sees every character) — v1.0
- ✓ Eviction workflow (sidebar + 30-day grace + auto-archive) — v1.0
- ✓ Onboarding site (Jekyll Pages, text-only walkthrough, troubleshooting + dev pages) — v1.0

See `milestones/v1.0-REQUIREMENTS.md` for the full 69-REQ-ID reconciliation.

### Active

<!-- No active milestone scope yet. Define via /gsd-new-milestone. -->

(None yet — milestone v1.0 complete; v1.1+ not yet planned.)

### v1.0 Partials / Waivers (user-authorized; v1.0.1 candidates)

- ⚠ **INST-05** (SmartScreen video) — shipped text-only; SignPath OSS approval in flight could retire the partial to full
- ⚠ **SEARCH-03** (inline staleness in search results) — shipped via Path 2 (existing view/bank Last Synced + search-button cache-freshness tooltip); user explicitly chose this over inline
- — **ENRICH-09** (PigParse + wiki courtesy emails) — waived; politeFetch throttling sufficient

### v2 Requirements (deferred; prerequisites still open)

- **Per-user wantlist** in the sheet — mark items each guildie wants to buy or quest for
- **EC tunnel auction monitor (P1999 Blue)** — DM via Discord when wantlisted item is auctioned (fed by PigParse, not chat-log parsing)
- **WTS monitor across three Raid Alliance Discord servers** — Discord bot reads designated trade channels, regex-matches wantlist items, DMs the guildie
- **Quest-target raid monitor across the same three Discord servers** — DM guildie when a raid target tied to a wantlisted-item quest is announced

**v2 prerequisites (must clear before v2 phase starts):**
1. Raid Alliance bot invites — admin/owner permission in all three Discord servers (not yet negotiated)
2. PigParse REST confirmed (✓ Phase 3 unblocked this); courtesy contact still pending if/when load becomes meaningful
3. Per-user Discord identity capture via sidebar
4. Curated `quest → raid-target NPC(s)` lookup populated

### Out of Scope (still valid post-v1.0)

| Feature | Reason |
|---------|--------|
| Other servers (P99 Green, P99 Red, live EQ) | Guild plays Blue; cross-server is multiplicative complexity for zero value to this guild. |
| Mobile app | Sheet is reachable from any browser; native mobile is unnecessary scope. |
| Inventory privacy tiers (per-user-visible-only) | Universal visibility was an explicit v1 choice; revisit only on opt-out request. |
| DKP / loot council systems | Adjacent problem space; EQDKP / OpenDKP cover this well. |
| Real-time inventory diffing alerts | Interesting but not core to "what's missing?" Core Value. |
| Magelo-style external character profile pages | Sheet is the front-end; we don't publish public pages. |
| Coin tracking from `/outputfile inventory` | File format does not contain coin amounts; manual sidebar field is the only honest option. |
| Service-account or shared-credentials Google auth | Incompatible with the idiot-proof setup goal. |
| Macros / hotkey / GINA-trigger management | WinEQDB already covers this; not Core Value. |
| In-game overlays | Out of P99 ToS-comfort and out of scope. |
| HTML scraping of PigParse or the wiki | Both expose APIs; never scrape. (Locked decision; CLAUDE.md enforces.) |
| Apps Script Rhino runtime | Rhino was retired 2026-01-31; V8 only. |

## Context

- **Domain:** Project 1999 ("P99") is a community-run Classic EverQuest emulator. Players use the in-game `/outputfile inventory` and `/outputfile spellbook` commands to dump tab-separated text files into the EQ install folder, named `<CharName>-Inventory.txt` and `<CharName>-Spellbook.txt`.
- **Inventory file format:** tab-separated, five columns: `Location | Name | ID | Count | Slots`. The `ID` column is the canonical EQ item ID and is the right join key against wiki + PigParse data; item `Name` strings can drift but IDs are stable. The file does **not** contain coin/platinum totals (verified against community parsers + Fanra wiki).
- **Spellbook file format:** tab-separated, two columns: `Level | Name`. (Existing docs called column 1 `Slot` historically; that's a misnomer — verified from real SK sample 2026-05-01 to be spell-grant Level.)
- **Audience:** a single P99 guild, ~12 active members, mixed technical comfort. The setup ceiling is "click the installer, click Allow on a Google sheet permission, click Allow on a Windows permission." Anything more complex is hidden behind the wizard.
- **Existing community tools** (for inspiration / non-overlap): EQHTML and "P99 Inventory Parser" both read these same files but render local-only views. SquireBot's differentiator is *guild-wide aggregation*, *progression checklists vs. wiki tiers*, and *price awareness*.
- **External data sources:** P1999 MediaWiki API (`https://wiki.project1999.com/api.php`) + PigParse REST (`https://pigparse.azurewebsites.net`, Swagger at `/swagger/index.html`). Both are community-run and well-mannered citizens via the shared `politeFetch()` helper (User-Agent, ETag/If-Modified-Since, CacheService, exponential backoff, 1s sleep between wiki requests).
- **Sheet as primary UI:** the workbook does the heavy lifting (lookups, search, tooltips). Apps Script handles the dynamic bits (cross-character search sidebar, eviction sidebar, bank-coin sidebar, char-info sidebar, theme picker). The watcher stays small (~14k LOC Go; ~12 MB binary).
- **Codebase at v1.0 ship:** ~14k LOC Go (watcher; `cmd/squirebot` + `internal/{auth,config,eqfind,heartbeat,oauth,sheet,system,tray,update,watch,wincred,wizard}`) + ~11k LOC TypeScript (`apps-script/src/{lib,triggers,tabs,__tests__,__fixtures__}`); 203 commits since project init; 297 vitest tests passing.

## Constraints

- **Tech stack (LOCKED):** Go 1.24 single-binary Windows watcher with `fsnotify` v1.7+ / `wincred` / `lumberjack` / `fyne.io/systray` / `minio/selfupdate` / NSIS 3.10+ installer. Apps Script V8 (TypeScript via clasp v2.4 — NOT 3.x — + esbuild 0.20+). PigParse REST + MediaWiki API (never HTML scraping).
- **Auth (LOCKED):** OAuth 2.0 loopback PKCE with `client_secret` per Google's contract; `drive.file` non-sensitive scope only; refresh tokens in Windows Credential Manager via DPAPI; Production consent screen (Testing mode silently expires refresh tokens every 7 days).
- **Backend (LOCKED):** Single shared Google Sheet workbook per guild. No separate server, database, or web app in v1. (v2's Discord bot would be the first piece of always-on infrastructure.)
- **Schema evolution (LOCKED):** Extend-only — add columns at right edge, add tabs, add `_meta` rows. Breaking changes require `_meta.schema_version` bump + idempotent migration + watcher's `WatcherMaxSchemaVersion` check + script's `SCRIPT_MIN_SCHEMA_VERSION` check. v1.0 ships at v3.
- **View architecture (LOCKED):** Consolidated filterable mega-tabs (`view`, `gear_check`, `spell_check`, `bank`) with leading `Char` column. **Never** per-character view tabs — would breach Google's 200-tab/workbook limit at guild scale.
- **Code signing:** Default = unsigned + walkthrough; SignPath Foundation OSS approval submitted (in flight). EV certs no longer grant instant SmartScreen reputation (Microsoft removed perk March 2024) so EV is not the path.
- **Compatibility:** Windows-only watcher (P99 client is Windows-native; Mac/Linux Wine users remain a non-goal). Sheet is OS-agnostic.

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Per-guildie Google OAuth (not service account) | Service accounts require distributing a JSON key, violating the idiot-proof setup constraint. | ✓ Good (v1.0 shipped on this model) |
| Universal visibility (everyone sees everything) | Guild is small and trust-rich; tiered permissions are pure complexity tax for v1. | ✓ Good (no opt-out requests as of v1.0 ship) |
| Manual platinum entry for the bank toon | `/outputfile inventory` does not contain coin amounts; no honest alternative. | ✓ Locked by file format |
| Wiki data via weekly scrape, not curated tab | Wiki is the source of truth and shifts as the meta evolves; weekly refresh keeps things current with negligible upkeep. Item IDs are stable enough to absorb name brittleness. | ✓ Good (Phase 3 + 4 + 5 all consume `_item_master`, `_wiki_spells`, `_wiki_gear_tier`, `_quest_items`) |
| Discord pinger deferred to v2 | Different infrastructure (always-on bot, external-server permissions) than the core watcher+sheet; shipping core first delivered value months earlier. | ✓ Good (v1.0 shipped without it; v2 still has open prerequisites) |
| Sheet-as-UI rather than custom web app | Spreadsheet is already a familiar guild artifact; Apps Script handles the dynamic bits; avoids hosting anything in v1. | ✓ Good |
| **Consolidated filterable view tabs (not per-character)** | Per-character views would breach Google's hard 200-tab/workbook limit at guild scale (12 × ~10 × ~5 ≈ 600 tabs). Landing tabs ARE per-character (~120 total, comfortable). | ✓ Locked by Google tab limit |
| Stack: Go watcher + TypeScript/Apps Script V8 sheet | Go gives single-binary install (no runtime to ship), `drive.file` avoids Google verification audit, Apps Script V8 + clasp + esbuild is the only path that works post Rhino EOL. | ✓ Good (v1.0 shipped on this stack) |
| OAuth consent screen flips to Production before first guildie installs | Testing-mode refresh tokens silently expire every 7 days for non-Workspace users; `drive.file` is non-sensitive so Production does NOT require Google's verification audit. | ✓ Good (Phase 1 day-10 routine validates persistence) |
| **Google's `/token` requires `client_secret` for Desktop OAuth + PKCE** (contrary to spec) | Spec ≠ contract. Google's docs note the desktop secret is effectively public; it's baked into the binary alongside the client ID. | ✓ Locked (PROJECT.md Key Decision #10; would have crashed Phase 1 ship without this fix) |
| **EV code-signing certs no longer grant instant SmartScreen reputation** | Microsoft removed the perk March 2024; EV ≡ OV on UX. Default = unsigned + walkthrough; SignPath OSS in parallel. | ✓ Locked (saved buying an EV cert; updated STACK.md + PITFALLS.md #2) |
| **Phase 5 D-12 Path A — no `WatcherMaxSchemaVersion` bump for Phase 5** | Phase 5 was apps-script-only; held schema_version=3 end-to-end. | ✓ Good (no watcher rebuild required for v1.0; deploy was clasp push only) |
| **Instructional images + SmartScreen GIF DROPPED 2026-05-11** (mid-execution of Phase 5 plan 05-05) | Capture overhead not worth it for a 12-guildie audience; text-only walkthrough satisfies the documentation spirit. | ✓ Good (saved ~60 min of one-time capture work + ongoing maintenance of binary assets) |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`, if used):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections (done for v1.0 → 2026-05-11)
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state
5. Mark Key Decisions outcomes (✓ Good / ⚠️ Revisit / — Pending)

---

*Last updated: 2026-05-11 after v1.0.0 milestone close.*
