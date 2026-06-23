# SquireBot

## What This Is

SquireBot is a small Windows app that every member of a ~12-person Project 1999 (Classic EverQuest emulator) guild installs on their PC. It watches the EQ folder for the tab-separated text files produced by the in-game `/outputfile inventory` and `/outputfile spellbook` commands, and pushes their contents into a single shared Google Sheet. The sheet is the real product — it joins each guildie's character data with information scraped from the [P1999 wiki](https://wiki.project1999.com/) and prices from [PigParse](https://pigparse.azurewebsites.net) to give the guild a unified view of every character's gear, spells, progression toward Velious-tier readiness, the shared bank's contents, cross-character search, and item tooltips.

## Core Value

**Every guildie can answer "what does my character still need, and where in the guild is it?" without leaving the spreadsheet.** Inventory and spell data lands in the sheet automatically; progression, gaps, and prices are computed for them. If everything else fails, this must work.

## Shipped: v2.5 — Ownership Cleanup (2026-06-22)

**Goal (MET):** Reconciled character-ownership semantics for a guild that shares P99 logins — guild banks/bots are now owner-less, and eviction no longer over-deletes shared characters. Both phases (35–36, 3 plans, OWN-01..04) shipped + deployed live to squirebot.quest + officer browser-smoke approved; **backend + web, watcher UNTOUCHED → NO `v*` tag**. Schema **v15** (migration `00015`, Phase 35; Phase 36 is compute-on-read, no migration). Prod schema went v14→v15 on the Phase-36 deploy boot (the first prod application of `00015`).

**What shipped:**
- **Owner-less guild banks/bots (OWN-01/02/04)** — a reserved "guild" sentinel owner (id `1000000`) holds every designated bank/bot; designation no longer requires "claiming" the char, and a bank survives any individual member's eviction (the sentinel is never an eviction target + a destructive-write-path guard). Migration `00015` auto-migrated the two live prod banks (Findom + Slowscales) with no manual fixup.
- **Shared-character-safe eviction (OWN-03)** — the per-owner eviction cascade is narrowed via the existing `audit_log` `cross_owner_write` trail (Phase 36 is its first reader): a character uploaded by more than one guildie survives its first-uploader's eviction; the evicted member's guild code is still revoked. An "all-shared owner" stays evictable (code-only revoke) via an additive `preserved_shared_count` preview field + the `EvictionForm` web fix. One shared SQL predicate backs both the cascade and the preview so they can't diverge.

**Gate catches (both fixed before close):** code review caught a BLOCKER (the sentinel was guarded in the eviction picker but not the destructive write path — an officer POST could've wiped the bank); the plan-checker caught a BLOCKER (the narrowed preview disabled the Evict button for all-shared owners). Verifiers PASSED 6/6 + 13/13.

**Status:** SHIPPED + archived 2026-06-22 (`milestones/v2.5-{ROADMAP,REQUIREMENTS}.md`). No formal milestone audit (both phases independently verified + live browser-smoke this session). No `v*` tag (watcher untouched). Backlog follow-ups: P36 WR-01/WR-02, P35 IN-02, v2.4 MD-01. Next milestone undefined — start via `/gsd-new-milestone`.

## Shipped: v2.4 — Web UI Revamp (5-Tab Restructure) (2026-06-21)

**Goal (MET):** Reorganized squirebot.quest around five top-level tabs — **Characters · Inventory · Banks · Wishlist · Settings** — each answering one user question, backed by the new data architecture this required. All 6 phases (29–34, 34 plans, 27 requirements) shipped + deployed live + browser-smoke approved across 5 themes; schema v14 (migrations 00012 item_icon / 00013 item_statsblock / 00014 wishlist). Milestone audit PASSED 2026-06-21 (27/27 requirements · 6/6 phases · integration wired · 4/4 E2E flows; the one finding — a stale apex redirect — fixed during the audit). Watcher untouched throughout; NO `v*` tag (would fire the watcher release CI needlessly). Archive: `milestones/v2.4-{ROADMAP,REQUIREMENTS,MILESTONE-AUDIT}.md`. Open follow-up: dead wantlist code (api.ts orphaned wrappers + webadmin/wantlist.go twin) — a `/gsd-quick` cleanup.

**Source spec:** `Future Features.txt` (user-authored, 2026-06-17, on the user's desktop) — the authoritative description of the target UX.

**The five tabs:**
- **Characters** ("what does character X have?") — guild character list (own chars first A-Z, then others, then bots/banks) + per-character search; click a character → an **in-game-style inventory window** (real EQ slot layout, stack counts, backpack drill-down, bank-window view, right-click-style item tooltip with wiki stats + PigParse price + wiki link + last-synced).
- **Inventory** ("which characters have item X?") — guild-wide item list (name, guild-wide count, expandable holders, wiki + PigParse links); click an item → holders + slot + last-synced.
- **Banks** ("what's in the guild banks?") — banks-only list + total item value (PigParse) + total platinum across banks.
- **Wishlist** ("what can I get to improve my characters?") — per-character, per-equipment-slot upgrade wishlist (3 blanks per equipped slot, wiki-suggested from the Velious Pre-raid/Grouping + Velious Raiding sections), with Discord ping toggle + EC-hit badge. Reworks the v2.2/v2.3 wantlist + EC monitor.
- **Settings** ("how do I manage all this?") — folds in existing Theme / Notifications / Watcher Codes / Set Class & Level / My Characters / Admin.

**Approach:** Sketch-first — `/gsd-sketch` mocks the 5-tab structure + headline surfaces (in-game inventory window, per-slot wishlist) for the user to react to; then requirements + roadmap.

**Locked/ratified decisions (2026-06-17):**
- Consolidated-views lock **RELAXED** for the web app — per-character master-detail drill-down ALLOWED (one reusable detail view on selection); guild-wide consolidated grids remain. (CLAUDE.md Architecture updated; [[project_consolidated_views]].)
- Scope spans backend (inventory `Location`→slot-layout parsing + container nesting; bank valuation aggregation; per-slot wishlist data model + wiki-section suggestion engine) + web; watcher untouched (ingest already captures `Location`/`Slots` — this is parsing/surfacing, not new watcher work). Phases continue from v2.3 → v2.4 starts at **Phase 29**.

**The five tabs (all live):** Characters (in-game inventory window + examine), Inventory (item-centric "who has X"), Banks (banks-only list + guild valuation/platinum), Wishlist (per-character per-slot upgrade targets + Velious suggestions + Discord ping/EC-badge), Settings (consolidated Theme/Notifications/Watcher-codes/Class&Level/My-characters/Admin). `/` lands on Characters; the legacy consolidated grids remain at `/guild-views`.

**Status:** SHIPPED + archived 2026-06-21. Next milestone undefined — start via `/gsd-new-milestone`.

## Current State

**Shipped: v2.0 "Off Google" (2026-05-31).** The Google Sheet — both the UI and the data store — has been replaced by a self-hosted system, and Google is fully decommissioned. What shipped:
- A self-hosted **Go + SQLite backend** live over HTTPS at `https://api.squirebot.quest` (Hetzner Cloud VPS, US/amd64) — a single static Go binary behind Caddy auto-HTTPS, systemd `Restart=always` (reboot-survival verified), a `goose`-migrated schema, per-guildie bearer-token auth, the atomic-replace ingest endpoint, in-process daily-PigParse + weekly-wiki enrichment jobs, and a nightly Cloudflare R2 off-box backup with a drilled restore.
- A static **SvelteKit website** live at `https://squirebot.quest` (served by Caddy on the same VPS at the apex) — the 4 views (`view`/`gear_check`/`spell_check`/`bank`) as one reusable filterable/sortable DataGrid, cross-character fuzzy search + "did you mean?", rich HTML item tooltips (XSS-escaped), and a 5-theme EQ aesthetic.
- The **watcher off Google** — re-pointed to the backend HTTP API, ~8k LOC of OAuth/Sheets/Picker code deleted (binary 57% smaller, no Google secret), native "paste your guild code" onboarding shipped via the existing auto-updater.
- **Discord OAuth2 login** (gated on guild Discord membership, capturing per-user Discord identity) + officer web forms (eviction, bank-coin, admin management) porting v1 enforcement.
- **Google decommissioned** — the v2.0.0 release published + the guild flipped via auto-update; all 10 Apps Script triggers + the OAuth 2.0 client deleted; the Sheet abandoned in place. **The "Off Google" goal is met.** Guild migration is operational and climbing (3/11 reporting in at close).

**Milestone arc (Phases 11–16, 29 plans, 4 days):** P11 stood up the backend + ingest API (LIVE; BACKEND-01/02/03/04/06; PocketBase evaluated and rejected in favour of hand-rolled `net/http` + `modernc.org/sqlite`). P12 migrated enrichment to in-process scheduled jobs (4 pure parsers + `politeFetch` byte-parity-ported from Apps Script; ENRICH-10/11; a code review caught + fixed a HIGH gear-tier ETag data-loss bug). P13 re-targeted the watcher off Google (WATCH-08/09/10/11). P14 rebuilt the read UI as the SvelteKit site over a versioned Go read API (BACKEND-05 + WEB-01..05; WEB-02 parity proven by Go table-tests translated from the v1 vitest fixtures). P15 added Discord login + the 3 officer write forms (AUTH-08/09 + ADMIN-04/05/06; code review fixed 2 node-suite-invisible BLOCKERs). P16 cut over + decommissioned Google (CUTOVER-01..04, reframed by 16-CONTEXT to fresh-start char-meta form + abandon-Sheet-in-place). Full record in `.planning/MILESTONES.md` + `.planning/milestones/v2.0-ROADMAP.md` (+ `v2.0-REQUIREMENTS.md`).

**Prior state (v1.x lineage):** v1.0 shipped 2026-05-11 (5 phases, 31 plans — installer + OAuth watcher + Apps Script Sheet); v1.0.1 patch 2026-05-12 (installer shim + officer-only eviction + test/doc debt); v1.0.2 binary 2026-05-13 (robustness polish — milestone close superseded by the v2.0 pivot). Archives in `.planning/milestones/`. The v1 Google-Sheet system is now decommissioned.

## Shipped: v2.3 — Character Assignment & Per-Character Wantlists (2026-06-09)

**Goal (MET):** Associate SquireBot users with specific characters, let them view those characters' inventory, and create character-tagged wantlist items that roll up to the guildwide wantlist. All three delivered, deployed live (schema v10), browser-smoke verified; the milestone audit PASSED 2026-06-09 (14/14 requirements · 3/3 phases · 6/6 integration). Archive: `milestones/v2.3-{ROADMAP,REQUIREMENTS,MILESTONE-AUDIT}.md`.

**What shipped (Phases 26–28, 7 plans, 2 days):**
- **Character assignment** — members self-claim/release their characters (including ones uploaded under an unlinked/legacy owner); officers assign/reassign/remove + approve/deny requests + designate bank/bot characters from the admin panel. `00009` migration (schema v9): `character_assignment` (single-assignee PK), `assignment_request`, `character.is_guild_bot`, idempotent auto-seed; every change audited.
- **"My characters" inventory filter** — an additive client-side quick-filter + single-character drill-down over the all-members consolidated views; visibility stays all-members (CLAUDE.md consolidated-views rule intact, zero backend change).
- **Character-tagged wantlist** — wants gain an optional character dimension (`00010` migration → schema v10; nullable `character_id` with COALESCE dedup, existing wants backfill to NULL with no data loss); tagged wants aggregate into the guildwide wantlist with per-want character + owner attribution; member filter/group-by-character; the EC-monitor DM still targets the character's OWNER (embed names the character via a display-only "For <char>" field).

**Scope:** backend (`internal/backendsrv`) + web (`web/`). The Go **watcher was untouched** (backend-only schema; no `WatcherMaxSchemaVersion` concern).

**Locked decisions (2026-06-08):**
- Assignment = **both** (member self-claim + officer assign/override); single-assignee PK + officer-only bank/bot designation (ASSIGN-05 resolved).
- Inventory = **all-members views + a "my characters" filter** (additive; consolidated-views rule preserved).
- Wantlist = **character-tagged wants** rolling up to the guildwide list (with character/owner attribution, private `note` excluded — CWANT-04 resolved); DM still to the owner (the embed names the character — CWANT-05 resolved).

**Carry-forward fix:** 999.33 (officer-reversible bank/bot designation) resolved + deployed live 2026-06-09 via quick `260609-d2o`. Deferred: 999.34 cosmetic LOWs/NITs + 2 account-specific UATs (non-blocking).

**Next milestone:** undefined — start via `/gsd-new-milestone`. v2.2 Track 2 (the Discord pinger WTS/raid monitors, Phases 22–23) remains parked on the 3 Raid Alliance bot invites.

## In Progress (parked): v2.2 — Wantlist + Discord Pinger

**Status (2026-06-08):** Track 1 **SHIPPED LIVE** (Phases 19–21: per-user wantlist CRUD, notifications/DM infrastructure, EC-tunnel auction monitor). Track 2 (Phases 22–23: cross-server WTS + quest-target raid monitors) is **PARKED on the 3 Raid Alliance Discord bot invites** — deferred to revisit when the invites land, NOT abandoned. v2.3 opened ahead of v2.2's full close because Track 2 is invite-gated. (Phases 24 test-hardening + 25 Linux watcher also shipped under this window; watcher released as `v2.1.1` 2026-06-08.)

**Goal:** Guildies maintain a personal wantlist on squirebot.quest and get DMed on Discord when a wanted item appears — at EC-tunnel auction, in cross-server WTS channels, or as a raid-target tied to a wanted item's quest. (Backlog 999.12 / WANT-01..08 — the long-deferred "v2 feature," unblocked now that per-user Discord identity is paid by AUTH-09 + LINK-02.)

**Target features (4 areas → WANT-01..08):**
- **Per-user wantlist** — website CRUD to mark items you want (buy or quest), tied to your Discord identity. *(unblocked)*
- **EC-tunnel auction monitor** — DM when a wanted item is auctioned, fed by PigParse. *(unblocked; research must confirm PigParse exposes auction events vs only daily prices)*
- **WTS monitor** — a Discord bot reads trade channels across the 3 Raid Alliance servers, regex-matches your wantlist, DMs you. *(blocked on bot invites)*
- **Quest-target raid monitor** — DM when a raid target tied to a wanted item's quest is announced in those servers. *(blocked on bot invites + needs the curated quest→NPC lookup)*

**Sequencing:** front-load the new Discord-bot/DM infrastructure + the unblocked areas (wantlist + EC monitor); gate the cross-server monitors (WTS + raid-target) on the 3 Raid Alliance bot invites.

**Open prerequisites:** the 3 Raid Alliance Discord bot invites (admin permission, not yet negotiated); a curated `quest → raid-target NPC(s)` lookup.

**HARD CONSTRAINT (carried from v2.1):** never put Discord OAuth in the watcher — the bot/DM work is all backend + the guild's own Discord, not the watcher.

## Shipped: v2.1 — Self-Service Watcher Linking (2026-06-02)

**Goal (MET):** Let any guildie link their own watcher from squirebot.quest via Discord login — no maintainer hand-minting codes — while keeping the watcher credential a static bearer token.

**What shipped (Phases 17–18, 4 plans, 2 days):**
- **Self-service `/account` page** behind the P15 Discord login — Generate mints a guild code whose owner is derived server-side from the Discord session, shown once with copy-to-clipboard + paste instructions; list your active codes (#N / created / last-seen) and revoke any one. Minting is additive (multiple PCs). Deployed live; 15/15 verified; browser-smoke approved.
- **Identity unification (LINK-02)** — minted codes tie to the Discord `web_user` via a new `owner.discord_user_id` FK; the eviction owner-floor now resolves through it instead of the loose `owner.label == web_user.username` match.
- **`mint-code` CLI retired** — self-service is the only mint path (`revoke-code` retained as ops backstop).
- **Watcher cleanups verified-closed (Phase 18, verify-or-close, zero new code):** 999.20 (gofmt), 999.21 (`freeConsole()` doc/impl + Debug-not-Warn), 999.22 (SemVer-aware `IsNewer`) all confirmed live. The "stuck 0.4.0-rc1 watcher" residual was a stale premise — it was the disposable Azure test VM, not a production watcher.

**Next milestone:** undefined — top candidate is the v2 Wantlist + Discord pinger (999.12 / WANT-01..08), whose per-user Discord-identity prerequisite is now fully pre-paid by AUTH-09 + LINK-02. Start via `/gsd-new-milestone`. Archive: `.planning/milestones/v2.1-{ROADMAP,REQUIREMENTS}.md`.

**Locked decisions (2026-06-01 discussion):**
- **Re-link = additive** — each link issues an *additional* valid token; multiple PCs per guildie work simultaneously; revocation is per-token.
- **Link code = the bearer token itself (v2.0 model, reusable)** — `auth.MintCode` shows the plaintext exactly once at mint time (hash-only at rest); the watcher pastes it and reuses it as its `Bearer` forever. No expiry. **NO watcher change** — onboarding is identical to v2.0; "self-service" is entirely a website + backend-endpoint change.
- **Manual `mint-code` CLI removed** — self-service fully replaces it; the ~11 existing guildies are already linked from the v2.0 cutover, so nobody is stranded.
- **HARD CONSTRAINT:** Discord identity is captured at link-time only; never put Discord OAuth *in the watcher* (re-introduces the ~7-day-expiry / public-secret / loopback fragility v2.0 deliberately escaped — P13 made the watcher browser-free on purpose). AUTH-09 already captures per-user Discord identity, so this dovetails with future v2 Wantlist/pinger work (999.12).

**Previous milestone:** v2.0 "Off Google" — ✅ SHIPPED 2026-05-31 (tag `v2.0.0`); all 26 requirements delivered, Google fully decommissioned, backend + website live. Archive: `.planning/milestones/v2.0-{ROADMAP,REQUIREMENTS}.md`.

<details>
<summary>Historical v2.0 scope (as planned at milestone open 2026-05-28)</summary>

**Goal:** Replace the shared Google Sheet (both the UI *and* the data store) with a self-hosted Go + SQLite backend and a static web frontend — permanently eliminating the Google OAuth dependency that blocked the guild.

**Why:** Google's brand-verification gate rejected the OAuth client (2026-05-15: *"home page not registered to you"* — `github.io` can't satisfy it), walling off all ~12 guildie watchers. Rather than rent a domain just to placate Google, v2.0 removed Google from the system entirely. Full research: `.planning/explorations/website-milestone/SCOPE.md` + 4 findings docs.

**Delivered features (Phases 11–16):**
- **P11 — Backend foundation + ingest API:** Hetzner Cloud VPS (US) + Caddy auto-HTTPS; SQLite schema with `goose` migrations; per-guildie bearer-token auth; the upload-receiving endpoint. *Front-loaded so guild data flowed again ASAP.*
- **P12 — Enrichment migration:** PigParse daily + P1999 wiki weekly → in-process scheduled jobs (parsers ported near-verbatim; `politeFetch` controls carried over).
- **P13 — Watcher re-target:** swapped the Sheets client for an `internal/backend` HTTP client; deleted the OAuth/PKCE/Sheets/Drive-Picker machinery (~8k LOC); "paste your guild code" onboarding via the existing auto-updater.
- **P14 — Web frontend:** SvelteKit static app; the 4 views as a reusable data grid; client-side search + "did you mean?"; HTML tooltips; EQ theming.
- **P15 — Admin web forms + login:** Discord OAuth2 login (gated on guild Discord membership); eviction / bank-coin / admin-management as web forms.
- **P16 — Cutover:** REFRAMED by 16-CONTEXT — no formal soak (guild already dark on the Sheet), a fresh-start char-meta form instead of a Sheet backfill, auto-update + Discord herding, then decommission Google + abandon the Sheet in place.

**Stack (LOCKED + costed, ≈ $67/yr total):** Hetzner Cloud shared-vCPU VPS (US, amd64) backend (~$55/yr) · SQLite ($0) · SvelteKit static (apex on Caddy) ($0) · Discord OAuth2 website login ($0) · per-guildie opaque bearer token for watcher↔backend ($0) · ~$12/yr **website-only** domain. Azure was rejected (Pay-As-You-Go ~$160–250/yr for an always-on VM); Heroku was rejected (no free tier + ephemeral filesystem that destroys an on-disk SQLite store).

**Schema impact:** The old `_meta.schema_version` ↔ `WatcherMaxSchemaVersion` handshake **retired** in favour of forward-only DB migrations (`goose`) + an API version. The watcher's Sheets schema gate was removed in P13.

</details>

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

**v1.0.1 — Installer + Permissions Hardening (shipped 2026-05-12)**

- ✓ Installer overwrite-running shim (NSIS pre-install detects running watcher, `--quit` IPC, 10s poll, `taskkill /F` fallback) — INST-06, v1.0.1
- ✓ `_meta.guild_admins` allowlist — code-enforced officer-only eviction (ADMIN-01..03), v1.0.1
- ✓ Sidebar inline-JS unit tests via JSDOM (5/5 shipping sidebars covered) — TEST-01, TEST-02, v1.0.1
- ✓ PropertiesService recent-query persistence (Search sidebar MRU now durable per-user) — SEARCH-05, v1.0.1
- ✓ SUMMARY.md backfill for Phase 3 + 4 (documentation debt) — DOC-04, v1.0.1

See `milestones/v1.0.1-REQUIREMENTS.md` for the full 8-REQ-ID reconciliation.

**v2.0 — Off Google / Website Frontend (shipped 2026-05-31 as `v2.0.0`)** — full 26-REQ-ID reconciliation in `milestones/v2.0-REQUIREMENTS.md`.

- ✓ Self-hosted Go + SQLite backend on a Hetzner VPS — Caddy auto-HTTPS, `goose` migrations, atomic-replace ingest, per-guildie hashed bearer-token auth, nightly R2 backup (BACKEND-01..04, 06) — v2.0
- ✓ Versioned Go read API powering the four views (BACKEND-05) — v2.0
- ✓ Enrichment as in-process scheduled jobs — daily PigParse + weekly P1999 wiki, parsers + politeFetch byte-parity-ported from Apps Script (ENRICH-10, ENRICH-11) — v2.0
- ✓ Watcher off Google — uploads to the backend HTTP API; all OAuth/Sheets/Picker code deleted; "paste your guild code" onboarding via the auto-updater (WATCH-08..11) — v2.0
- ✓ Static SvelteKit website — the 4 consolidated views as one filterable/sortable DataGrid; gear/spell progression status; cross-character fuzzy search + "did you mean?"; rich HTML item tooltips; 5-theme EQ aesthetic (WEB-01..05) — v2.0
- ✓ Discord OAuth2 website login gated on guild Discord membership + per-user Discord identity capture (AUTH-08, AUTH-09) — v2.0
- ✓ Officer web forms — eviction (30-day grace + archive, owner-floor), manual bank-coin entry, admin/officer management (ADMIN-04, ADMIN-05, ADMIN-06) — v2.0
- ✓ Cutover + decommission — fresh-start char-meta form (no Sheet backfill), coordinated auto-update flip, Google killed (triggers + OAuth client), Sheet abandoned in place (CUTOVER-01..04, reframed by 16-CONTEXT) — v2.0

**v1.0.2 — Robustness Polish (binary shipped `v1.0.2` 2026-05-13; milestone close superseded by v2.0; closed):**

- ✓ Boot-time invalid_grant Reauthorize recovery (AUTH-07) — in binary; live UAT was blocked by Google verification, then the Google OAuth path was deleted entirely in v2.0
- ✓ Tray controller pre-Ready call queue / RunApp retry (OPS-06) — in binary (the v2.0 tray was rebuilt Google-free in P13)
- ✓ UTF-8 BOM strip in config loader (CONFIG-01) — in binary (carried into the v2.0 watcher)
- ✓ Foreground-shell-close silent death fix (OPS-07) — in binary
- ✓ Admin-Mgmt sidebar inline-JS test coverage (TEST-03) — shipped (dev-workbook smoke passed; the Sheet sidebars are now decommissioned)
- ✓ Phase 8 advisory test-quality fixes (TEST-04) — shipped (apps-script suite now decommissioned)

**v2.1 — Self-Service Watcher Linking (shipped 2026-06-02 as `v2.1`)** — full 9-REQ-ID reconciliation in `milestones/v2.1-REQUIREMENTS.md`.

- ✓ Self-service mint: signed-in guildie mints their own code, owner derived server-side from the Discord session (LINK-01) — v2.1
- ✓ Discord-identity↔owner FK linkage; eviction owner-floor resolves via `owner.discord_user_id` (LINK-02) — v2.1
- ✓ Additive minting — multiple PCs per guildie, per-token revoke (LINK-03, LINK-05) — v2.1
- ✓ "Link your watcher" `/account` page — show-once plaintext, copy-to-clipboard, paste instructions, never re-revealed (LINK-04) — v2.1
- ✓ `mint-code` CLI removed; self-service is the only mint path (LINK-06) — v2.1
- ✓ Watcher cleanups verify-or-closed — gofmt (WATCH-12), `freeConsole()` doc/impl + Debug-not-Warn (WATCH-13), SemVer-aware `IsNewer` (WATCH-14) — v2.1

**v2.3 — Character Assignment & Per-Character Wantlists (shipped 2026-06-09)** — full 14-REQ-ID reconciliation in `milestones/v2.3-REQUIREMENTS.md`.

- ✓ Character assignment — members self-claim/release; officers assign/reassign/remove + approve/deny + designate bank/bot from the admin panel; `00009` migration (schema v9): `character_assignment` (single-assignee PK), `assignment_request`, `character.is_guild_bot`; IDOR-safe + audited (ASSIGN-01..06) — v2.3
- ✓ "My characters" inventory filter — additive client-side quick-filter + single-character drill-down over the all-members consolidated views; consolidated-views rule intact, zero backend change (MYVIEW-01, MYVIEW-02) — v2.3
- ✓ Character-tagged wantlist — optional `character_id` on `wantlist_item` (`00010` → schema v10, NULL backfill, COALESCE dedup); guildwide roll-up with character/owner attribution; member filter/group-by-character; EC-monitor DM still targets the owner (embed names the character) (CWANT-01..06) — v2.3

### Active

<!-- v2.4 Web UI Revamp — requirements TBD (sketch-first; defined after the design direction is chosen). v2.2 Wantlist Track 2 still parked. -->

**v2.4 — Web UI Revamp (in progress, opened 2026-06-17)** — web-only IA/navigation + component/interaction polish; requirements being defined via sketch-first design exploration (REQUIREMENTS.md filled once the direction is chosen). Phases start at 29.

**v2.2 — Wantlist + Discord Pinger (Track 2 parked, started 2026-06-02)** — full REQ-ID list in `REQUIREMENTS.md` (WANT-01..08).

- Per-user wantlist (website CRUD, Discord-identity-tied)
- EC-tunnel auction monitor → Discord DM (PigParse-fed)
- WTS monitor across 3 Raid Alliance Discord servers → Discord DM (bot; invite-gated)
- Quest-target raid monitor across the same servers → Discord DM (bot; invite-gated + quest→NPC lookup)

### v1.0 Partials / Waivers (user-authorized; still tracked post-v1.0.1)

- ⚠ **INST-05** (SmartScreen video) — shipped text-only; SignPath OSS approval (999.9) still in flight could retire the partial to full
- ⚠ **SEARCH-03** (inline staleness in search results) — shipped via Path 2 (existing view/bank Last Synced + search-button cache-freshness tooltip); user explicitly chose this over inline
- — **ENRICH-09** (PigParse + wiki courtesy emails) — waived; politeFetch throttling sufficient

### Wantlist + Discord-pinger (deferred to a future milestone; prerequisites still open)

_Still deferred after v2.0. v2.0 pre-paid prerequisite #3 (per-user Discord identity capture) via AUTH-09. Tracked as backlog 999.12 / WANT-01..08._

- **Per-user wantlist** — mark items each guildie wants to buy or quest for (now a website feature, not the sheet)
- **EC tunnel auction monitor (P1999 Blue)** — DM via Discord when a wantlisted item is auctioned (fed by PigParse, not chat-log parsing)
- **WTS monitor across three Raid Alliance Discord servers** — Discord bot reads designated trade channels, regex-matches wantlist items, DMs the guildie
- **Quest-target raid monitor across the same three Discord servers** — DM guildie when a raid target tied to a wantlisted-item quest is announced

**Prerequisites (must clear before this milestone starts):**
1. Raid Alliance bot invites — admin/owner permission in all three Discord servers (not yet negotiated)
2. PigParse REST confirmed (✓ Phase 3 unblocked this); courtesy contact still pending if/when load becomes meaningful
3. Per-user Discord identity capture — ✓ **pre-paid by v2.0 AUTH-09** (Discord OAuth2 login captures discord_user_id/username on every sign-in)
4. Curated `quest → raid-target NPC(s)` lookup populated

### Out of Scope (still valid post-v1.0)

| Feature | Reason |
|---------|--------|
| Other servers (P99 Green, P99 Red, live EQ) | Guild plays Blue; cross-server is multiplicative complexity for zero value to this guild. |
| Mobile app | The website is reachable from any browser; native mobile is unnecessary scope. |
| Inventory privacy tiers (per-user-visible-only) | Universal visibility was an explicit v1 choice; the DB makes tiers cheap to add later, but revisit only on opt-out request. |
| DKP / loot council systems | Adjacent problem space; EQDKP / OpenDKP cover this well. |
| Real-time inventory diffing alerts | Interesting but not core to "what's missing?" Core Value. |
| Inventory history / per-item time-series | Not in the Core Value; adds real complexity. Parked as backlog (finding 04 §1.2). |
| Magelo-style external character profile pages | The website is internal (Discord-gated, noindex); we don't publish public pages. |
| Coin tracking from `/outputfile inventory` | File format does not contain coin amounts; the manual web-form field is the only honest option. |
| Postgres / managed DB | SQLite fits the tiny data (<100 MB, ~50–150 writes/day); Postgres is needless ops overhead at this scale (retained against the research recommendation; no regret at v2.0 close). |
| "Sign in with Google" website login | Re-introduces the exact brand-verification gate v2.0 exists to escape; Discord OAuth2 (gate-free) is the choice. |
| Service-account or shared-credentials Google auth | Incompatible with the idiot-proof setup goal — and Google is now fully out of the system. |
| Macros / hotkey / GINA-trigger management | WinEQDB already covers this; not Core Value. |
| In-game overlays | Out of P99 ToS-comfort and out of scope. |
| HTML scraping of PigParse or the wiki | Both expose APIs; never scrape. (Locked decision; CLAUDE.md enforces.) |

## Context

- **Domain:** Project 1999 ("P99") is a community-run Classic EverQuest emulator. Players use the in-game `/outputfile inventory` and `/outputfile spellbook` commands to dump tab-separated text files into the EQ install folder, named `<CharName>-Inventory.txt` and `<CharName>-Spellbook.txt`.
- **Inventory file format:** tab-separated, five columns: `Location | Name | ID | Count | Slots`. The `ID` column is the canonical EQ item ID and is the right join key against wiki + PigParse data; item `Name` strings can drift but IDs are stable. The file does **not** contain coin/platinum totals (verified against community parsers + Fanra wiki).
- **Spellbook file format:** tab-separated, two columns: `Level | Name`. (Existing docs called column 1 `Slot` historically; that's a misnomer — verified from real SK sample 2026-05-01 to be spell-grant Level.)
- **Audience:** a single P99 guild, ~12 active members, mixed technical comfort. The setup ceiling is "click the installer, click Allow on a Google sheet permission, click Allow on a Windows permission." Anything more complex is hidden behind the wizard.
- **Existing community tools** (for inspiration / non-overlap): EQHTML and "P99 Inventory Parser" both read these same files but render local-only views. SquireBot's differentiator is *guild-wide aggregation*, *progression checklists vs. wiki tiers*, and *price awareness*.
- **External data sources:** P1999 MediaWiki API (`https://wiki.project1999.com/api.php`) + PigParse REST (`https://pigparse.azurewebsites.net`, Swagger at `/swagger/index.html`). Both are community-run and well-mannered citizens via the shared polite-fetch helper (now `internal/backendsrv/enrich/politefetch` in Go: User-Agent, ETag/If-Modified-Since, exponential backoff honoring Retry-After, 1s sleep between wiki requests). v2.0 byte-parity-ported the Apps Script `politeFetch` to this Go client.
- **System shape (v2.0):** the product is now a **Go backend** + a **SvelteKit website** + the **slimmed Google-free watcher**. The backend (`internal/backendsrv/{store,enrich,ingest,readapi,webadmin,scheduler,...}` + `cmd/squirebot-server`) is a single static Go binary on a Hetzner VPS behind Caddy — it owns the SQLite store, the atomic-replace ingest endpoint, the versioned read API, the in-process enrichment scheduler, Discord-OAuth2 sessions, and the officer write surface. The frontend (`web/` — SvelteKit adapter-static SPA) renders the 4 views as a reusable DataGrid + client-side search/tooltips/theming and is served by Caddy at the apex. The watcher (`cmd/squirebot` + `internal/{app,backend,credstore,onboarding,config,eqfind,tray,update,watch,wincred}`) just watches the EQ folder and POSTs raw UTF-8 snapshots with its bearer token. **The Google Sheet + the Apps Script project are decommissioned** (triggers + OAuth client deleted; Sheet abandoned in place).
- **Codebase at v2.0 ship:** the re-targeted watcher binary is **7.07 MB (57% smaller** than the 16.44 MB v1 Google build), with the entire Google dependency tree shed and zero Google secret. The backend is a static linux/amd64 ELF; the SQLite schema is at `goose` migration `00004`. Web tests run node-only via pure helpers (172 web vitest tests at P15 close; +28 for P16's char-meta = 200). The old apps-script suite (336 vitest) is retired with the Sheet. The watcher's `WatcherMaxSchemaVersion` ↔ `_meta.schema_version` handshake is gone, replaced by the `/api/v1/...` API version + a 426-too-old gate.

## Constraints

- **Tech stack (LOCKED):** Go 1.24 single-binary Windows watcher with `fsnotify` v1.7+ / `wincred` / `lumberjack` / `fyne.io/systray` / `minio/selfupdate` / NSIS 3.10+ installer. Apps Script V8 (TypeScript via clasp v2.4 — NOT 3.x — + esbuild 0.20+). PigParse REST + MediaWiki API (never HTML scraping).
- **Auth (LOCKED):** OAuth 2.0 loopback PKCE with `client_secret` per Google's contract; `drive.file` non-sensitive scope only; refresh tokens in Windows Credential Manager via DPAPI; Production consent screen (Testing mode silently expires refresh tokens every 7 days).
- **Backend (CHANGING in v2.0):** v1.x used a single shared Google Sheet workbook per guild — no separate server, database, or web app. **v2.0 "Off Google" replaces this** with a self-hosted Go + SQLite backend on a Hetzner Cloud VPS (US) + Caddy + a static web frontend, retiring the Google Sheet and the Google OAuth dependency entirely. See `## Current Milestone`.
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
| **v1.0.1: No schema bump across all 3 phases** | `_meta.guild_admins`, `_meta.workbook_owner_floor`, `_meta.admin_log` are extend-only row additions; SEARCH-05 PropertiesService swap is a persistence-layer change, not a tab schema change | ✓ Good (no migration logic; no watcher rebuild for schema reasons; binary v1.0.1 rebuild was for NSIS install path only) |
| **v1.0.1 ADMIN-03: Owner-floor lockout protection** | Workbook owner email cannot be removed from `_meta.guild_admins` by anyone other than themselves; prevents catastrophic peer-admin lockout | ✓ Locked + UAT-verified live in dev workbook (Hook 4 PASS) |
| **v1.0.1 SEARCH-05: PropertiesService per-user, NOT Document scope** | Recent-3 MRU is per-guildie state, not workbook-shared state; migrating from CacheService keeps the same scope semantics while gaining durability across the 25-min TTL | ✓ Good |
| **v1.0.1 mid-smoke: Apps Script manifest must declare `userinfo.email` under consumer @gmail.com** | Without it, `Session.getEffectiveUser().getEmail()` silently returns empty; broke every Phase 7 admin guard during dev-workbook Hook 3; retired latent v1.0 `initiated_by='unknown'` audit-log silent fallback | ✓ Locked (commit `544bef8`; non-sensitive scope; future rule: any future scope addition stays non-sensitive so the consent screen does not require Production-consent re-review) |
| **v1.0.1: TRIGGER_GLOBALS ↔ Code.ts sync assertion is load-bearing for every plan that adds Apps Script globals** | The Phase 3 `d0a2645`-style sync assertion fires at build time and blocks the build if the two lists diverge; surfaced as a "minor deviation" during Plan 07-02 because the plan text didn't enumerate `build.mjs` in its `<files>` block but the assertion is non-negotiable | ✓ Locked (future plans adding `google.script.run` callbacks must update both `Code.ts` AND `build.mjs` TRIGGER_GLOBALS) |
| **v2.0 backend host = Hetzner Cloud VPS (US, amd64)** (switched from Oracle Cloud Always Free 2026-05-29) | v2.0 first overrode the research-recommended Hetzner VPS to Oracle Always Free chasing a $0 box, then reverted to the **research-recommended Hetzner Cloud VPS** — always-on, no idle-reclamation, `ufw` works normally (no Oracle iptables trap), US region for the P14 frontend. **SQLite + `goose` retained** (not the research's Postgres). Heroku was evaluated and rejected (no free tier + ephemeral filesystem destroys on-disk SQLite). Backup target moved Oracle Object Storage → Cloudflare R2 via `rclone`; cross-compile arch ARM64 → amd64. See Phase 11 CONTEXT D-11/D-12/D-14. | ✓ Good (live + reboot-survival proven; ~$55/yr VPS + ~$12/yr domain ≈ $67/yr — the "$0 backend" premise retired) |
| **v2.0: SQLite retained over Postgres** | SQLite fits the tiny data (<100 MB, ~50–150 writes/day); Postgres is needless ops overhead at this scale. DDL ports with `CITEXT`→`TEXT COLLATE NOCASE`; `pg_trgm` search → SQLite `LIKE`/FTS5. Kept against the four findings docs' Postgres recommendation. | ✓ Good (no regret at v2.0 close; 4 migrations applied idempotently) |
| **v2.0: hand-rolled Go (`net/http` + `modernc.org/sqlite`) over PocketBase** (D-01 spike verdict, P11) | All 4 spike probes passed, but the opaque-token auth + plain-SQL-table design bypasses PocketBase's auth-record + collection leverage points; not worth a 22.9 MB pre-1.0 framework + migration tax. | ✓ Good (no framework churn; P15 Discord OAuth2 consequently hand-rolled `golang.org/x/oauth2`) |
| **v2.0: website login = Discord OAuth2, never "Sign in with Google"** | Discord OAuth2 has no brand-verification/app-review gate; "Sign in with Google" would re-introduce the exact gate v2.0 exists to escape. Gated on guild Discord membership (no allowlist upkeep); pre-pays AUTH-09. | ✓ Good (live gate returns 401 for non-members; per-user Discord identity captured) |
| **v2.0 cutover REFRAMED — fresh-start char-meta form (not Sheet backfill) + abandon the Sheet in place** (16-CONTEXT) | The guild had been dark on the Sheet since 2026-05-15 and P13–P15 were already live, voiding the classic shadow-soak/backfill/parity dance; inventory + enrichment self-heal, so a clean start (one login-only char-meta form for class/level/race/is_bank_toon) is simpler than a one-time read-only Sheet import. The Sheet was left untouched — no export/delete/freeze. | ✓ Good (Google decommissioned at 3/11 reporting in, climbing; live system verified unaffected) |
| **v2.0: single-bank-toon invariant enforced at the store seam** (P16 MD-01) | `compute.Bank` assumes exactly one bank toon; `SetCharMetaTx` enforced no uniqueness, so flagging 2+ would silently merge bank-view rows. Setting `is_bank_toon=true` now clears it on all other chars in the same tx (matches v1's single-value `_meta.bank_toon_name`). | ✓ Good (fixed post-close in `0e31023` + store regression tests) |
| **v2.1: self-mint owner derived server-side from the Discord session, never client-supplied** (P17 D-02) | The v1 `mint-code --owner <free-text>` path is an identity-spoofing hazard; the self-service endpoint instead resolves the owner from `caller(ctx)` (the session `discord_user_id`) via resolve-or-create-owner (adopt-by-label / create-fresh / refuse-on-ambiguity). Link code stays the reusable bearer token (no watcher change). | ✓ Good (live; owner-scoped list/revoke close IDOR; `mint-code` CLI removed) |
| **v2.1: verify-or-close phases verify against LIVE state, not carried-forward notes** (P18) | The "maintainer's watcher stuck on 0.4.0-rc1" residual was carried from the v2.0 close into v2.1's requirements/roadmap/plan before anyone checked it. A live `watcher_version` query + registry check debunked it (the box was a disposable Azure test VM). | ✓ Good (zero rework shipped; cheap checks beat trusting stale notes — now a documented retro pattern) |
| **v2.3: single-assignee `character_assignment` PK + officer-only bank/bot designation** (P26, ASSIGN-05 resolved) | A character maps to one assignee; the shared-bank case is handled by officer-only bank/bot designation rather than a many-to-many owner layer — simpler model, no schema rework needed to flip it. | ✓ Good (live; audited; surfaced the 999.33 panel-reachability gap, fixed same milestone) |
| **v2.3: "my characters" inventory view is an ADDITIVE filter, NOT access control** (P27) | The all-members consolidated views (CLAUDE.md LOCKED — per-character view tabs would breach Google's old 200-tab limit; the rule carries to the web DataGrid) stay all-members-visible; "my characters" is a client-side `<select>` filter feeding the single reusable grid pre-filtered data. | ✓ Good (live; zero backend change; no row hidden from any member) |
| **v2.3: character-tagged wantlist with COALESCE dedup + IDOR guard + owner-DM invariant** (P28) | `character_id` on `wantlist_item` is NULLABLE (`00010`/v10; existing wants backfill to NULL, COALESCE(character_id,-1) keys the dedup index); `AddWantHandler`'s `IsCharAssignedToTx` rejects forged tags (403); the EC-monitor DM still targets the want OWNER (`discord_user_id`) — the P28 LEFT JOIN supplies only a display "For <char>" embed field. | ✓ Good (live, schema v10; integration verdict CLEAN; owner-DM invariant holds in code) |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`, if used):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections (done for v1.0 → 2026-05-11, v1.0.1 → 2026-05-12, v2.0 → 2026-05-31, v2.1 → 2026-06-02, v2.3 → 2026-06-09)
2. Core Value check — still the right priority? (✓ unchanged at v2.0; "what's missing, where is it in the guild" is still the load-bearing question — now answered via the website, not the Sheet)
3. Audit Out of Scope — reasons still valid? (✓ at v2.0; the Sheet-phrased reasons were refreshed to the website, and Postgres + "Sign in with Google" added; Wantlist/pinger still validly deferred)
4. Update Context with current state (✓ at v2.0 — now a Go backend + SvelteKit frontend + Google-free watcher; Sheet/Apps Script decommissioned)
5. Mark Key Decisions outcomes (✓ Good / ⚠️ Revisit / — Pending) (✓ at v2.0 — 6 v2.0 rows appended)

---

*Last updated: 2026-06-22 — milestone **v2.5 "Ownership Cleanup"** SHIPPED + archived (Phases 35–36, 3 plans, OWN-01..04; owner-less guild banks/bots + shared-character-safe eviction; deployed live, schema v15, watcher untouched, no `v*` tag). Prior: **v2.4 "Web UI Revamp"** SHIPPED + archived (6 phases 29–34, 34 plans, 27 reqs; 5-tab restructure live; schema v14; audit PASSED). Prior: v2.3 (Phases 26–28) SHIPPED 2026-06-09 (schema v10). v2.2 Track-2 (Discord pinger WTS/raid monitors, Phases 22–23) still parked on the Raid Alliance bot invites. Next milestone undefined — `/gsd-new-milestone`.*
