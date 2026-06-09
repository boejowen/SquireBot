# Roadmap: SquireBot

**Core Value:** Every guildie can answer "what does my character still need, and where in the guild is it?" — now delivered via a self-hosted website instead of the Google Sheet.

## Milestones

- ✅ **v1.0** — Watcher + Workbook + Onboarding (initial release) — shipped 2026-05-11 as tag `v1.0.0`
- ✅ **v1.0.1** — Installer + Permissions Hardening — shipped 2026-05-12 (binary tag `v1.0.1`)
- ✅ **v1.0.2** — Robustness Polish — binary shipped 2026-05-13 (tag `v1.0.2`); milestone close superseded by v2.0
- ✅ **v2.0** — "Off Google" — Website Frontend — Phases 11–16 (shipped 2026-05-31 as tag `v2.0.0`) — archive: [`milestones/v2.0-ROADMAP.md`](milestones/v2.0-ROADMAP.md)
- ✅ **v2.1** — Self-Service Watcher Linking — Phases 17–18 (shipped 2026-06-02 as tag `v2.1`) — archive: [`milestones/v2.1-ROADMAP.md`](milestones/v2.1-ROADMAP.md)
- 🔄 **v2.2** — Wantlist + Discord Pinger — Phases 19–25 (**Track 1 SHIPPED LIVE 2026-06-06** — Phases 19–21 deployed to api.squirebot.quest; Phases 24–25 quality/platform shipped; **Track 2 (Phases 22–23) PARKED — invite-gated** on the 3 Raid Alliance bot invites, not abandoned; milestone held open, **no tag** until Track 2 lands)
- 🔄 **v2.3** — Character Assignment & Per-Character Wantlists — Phases 26–28 (**all 3 phases SHIPPED + deployed live 2026-06-08/09** — character assignment, a "my characters" inventory filter, and character-tagged wantlists rolling up to the guildwide list; backend + web only, **watcher untouched**; migrations `00009` (P26 assignment) + `00010` (P28 wantlist `character_id`) → schema **v10**; **feature-complete, pending milestone audit/close**)

## Phases

<details>
<summary>✅ v1.0 — Watcher + Workbook + Onboarding (Phases 1–5) — SHIPPED 2026-05-11</summary>

Full details in [`milestones/v1.0-ROADMAP.md`](milestones/v1.0-ROADMAP.md).

- [x] Phase 1: End-to-End Thin Slice (8 plans) — shipped v0.1.0, 2026-05-02
- [x] Phase 2: Watcher Robustness + Schema Lock (10 plans) — shipped v0.2.0 + v0.2.1 hotfix, 2026-05-09
- [x] Phase 3: Apps Script Enrichment Foundation (4 plans) — shipped v0.3.0, 2026-05-10
- [x] Phase 4: Differentiator Features (4 plans) — shipped v0.4.0, 2026-05-11
- [x] Phase 5: Search + Onboarding + Privacy Polish (5 plans) — shipped v1.0.0, 2026-05-11

**Total:** 31 plans · 5 phases · 11 days kickoff to ship · 203 commits.

</details>

<details>
<summary>✅ v1.0.1 — Installer + Permissions Hardening (Phases 6–8) — SHIPPED 2026-05-12</summary>

Full details in [`milestones/v1.0.1-ROADMAP.md`](milestones/v1.0.1-ROADMAP.md).

- [x] Phase 6: Installer Overwrite-Running Shim (5 plans) — shipped + UAT-verified as tag `v1.0.1`, 2026-05-11
- [x] Phase 7: Admin Allowlist + Eviction Enforcement (3 plans) — shipped via dev-workbook 5-hook smoke, 2026-05-12
- [x] Phase 8: Test Infra + Persistence + Docs Backfill (4 plans) — shipped (5/5 must-haves; 336/336 vitest), 2026-05-12

**Total:** 12 plans · 3 phases · 2 days kickoff to ship · 63 commits since v1.0.0.

</details>

<details>
<summary>✅ v1.0.2 — Robustness Polish (Phases 9–10) — binary shipped 2026-05-13; milestone close superseded by v2.0</summary>

Binary `v1.0.2` shipped 2026-05-13; its milestone close was superseded by the v2.0 "Off Google" pivot (the Sheet it targeted is being replaced). Phase directories 09 and 10 exist on disk; the next milestone continues at Phase 11 (never reuses 9 or 10).

- [x] Phase 9: Watcher Robustness Polish (5 plans) — AUTH-07, OPS-06, OPS-07, CONFIG-01; shipped as watcher v1.0.2 binary, 2026-05-13. HUMAN-UAT was blocked on 999.19 (Google brand verification), then superseded.
- [x] Phase 10: Apps Script Test Quality (3 plans) — TEST-03, TEST-04; shipped via `clasp push` + green CI, 2026-05-13.

</details>

<details>
<summary>✅ v2.0 — “Off Google” — Website Frontend (Phases 11–16) — SHIPPED 2026-05-31</summary>

Full details in [`milestones/v2.0-ROADMAP.md`](milestones/v2.0-ROADMAP.md).

**Milestone Goal:** Replace the shared Google Sheet (both the UI *and* the data store) with a self-hosted Go + SQLite backend and a static web frontend — permanently eliminating the Google OAuth dependency that blocked the guild after Google's 2026-05-15 brand-verification rejection. Goal MET: backend + website live, Google fully decommissioned.

- [x] Phase 11: Backend Foundation + Ingest API (7 plans) — Hetzner Cloud VPS (US) + Caddy auto-HTTPS live at api.squirebot.quest; SQLite + `goose`; bearer-token auth; ingest endpoint; nightly R2 backup. ✅ 2026-05-29
- [x] Phase 12: Enrichment Job Migration (5 plans) — daily PigParse + weekly P1999 wiki as in-process scheduled jobs; 4 parsers + politeFetch byte-parity-ported from Apps Script. ✅ 2026-05-29
- [x] Phase 13: Watcher Re-Target + Onboarding (4 plans) — watcher off Google (~8k LOC deleted, binary 57% smaller, no Google secret); native “paste your guild code” onboarding via the auto-updater. ✅ 2026-05-30
- [x] Phase 14: Web Frontend (4 plans) — static SvelteKit site (4 views as one reusable filterable/sortable DataGrid, fuzzy search + “did you mean?”, rich HTML tooltips, 5-theme EQ aesthetic) over a versioned Go read API; live at squirebot.quest. ✅ 2026-05-30
- [x] Phase 15: Admin Web Forms + Login (5 plans) — Discord OAuth2 login (guild-membership-gated, capturing per-user Discord identity) + officer web forms (eviction, bank-coin, admin management) porting v1 enforcement. ✅ 2026-05-31
- [x] Phase 16: Cutover + Decommission (4 plans) — published the clean `v2.0.0` release, minted 11 per-guildie codes, flipped the guild via auto-update, decommissioned Google (triggers + OAuth client retired; Sheet abandoned in place). ✅ 2026-05-31

**Total:** 29 plans · 6 phases · 4 days kickoff to ship (2026-05-28 → 2026-05-31). The cutover was REFRAMED by 16-CONTEXT: fresh-start char-meta form (no Sheet backfill) + abandon-Sheet-in-place. All 26 v2.0 requirements shipped.

</details>

<details>
<summary>✅ v2.1 — Self-Service Watcher Linking (Phases 17–18) — SHIPPED 2026-06-02</summary>

Full details in [`milestones/v2.1-ROADMAP.md`](milestones/v2.1-ROADMAP.md).

**Milestone Goal:** Let any guildie link their own watcher from squirebot.quest via the existing P15 Discord login — no maintainer hand-minting + DMing codes — while keeping the watcher credential a static, reusable bearer token (the v2.0 model; **no watcher change**). Goal MET.

- [x] Phase 17: Self-Service Watcher Linking (web feature) (3 plans) — `/account` self-mint (owner from Discord session) + Discord↔owner FK linkage + additive codes + per-token revoke + show-once copy-to-clipboard panel; `mint-code` CLI removed. Deployed live; 15/15 verified; browser-smoke approved. ✅ 2026-06-02
- [x] Phase 18: Watcher Cleanups — Verify-or-Close (1 plan) — confirmed 999.20/21/22 all live-correct with zero new code; the "stuck 0.4.0-rc1 watcher" residual was found to be the disposable Azure test VM, not a production watcher. ✅ 2026-06-02

**Total:** 4 plans · 2 phases · 2 days kickoff to ship · 20 commits since `v2.0`. All 9 requirements (LINK-01..06 + WATCH-12/13/14) shipped.

</details>

### 🔄 v2.2 — Wantlist + Discord Pinger (Phases 19–23)

**Milestone Goal:** Guildies maintain a personal wantlist on squirebot.quest and get DMed on Discord when a wanted item appears — at EC-tunnel auction (PigParse), in cross-server WTS channels, or as a raid-target tied to a wanted item's quest. (Backlog 999.12 / WANT-01..08 — the long-deferred "v2 feature," unblocked now that per-user Discord identity is paid by AUTH-09 + LINK-02.)

**Delivery/reading split (the load-bearing insight):** DM *delivery* is UNBLOCKED — the bot lives in the guild's OWN Discord, where every guildie already is via the v2.0 membership-gated login, so it can DM all of them. Only WTS/raid-channel *reading* is invite-gated on the 3 un-negotiated Raid Alliance server invites. **Track 1** (Phases 19–21: wantlist → DM/notification infra → EC monitor) ships complete, valuable value with zero dependency on the external invites. **Track 2** (Phases 22–23: WTS monitor → quest-target raid monitor) is hard-gated on the invites; a bot feature-flag + `guild_channel` rows let Track 1 ship with WTS/raid dark and flip on per-server as invites arrive — no rebuild.

**Locked decisions (v2.2 research, 2026-06-02 — see `research/SUMMARY-v2.2.md`):** in-process bot goroutine (NOT a separate process — avoids two SQLite writers), `recover()`-isolated + non-fatal start; `bwmarrin/discordgo` v0.29.0 (CGO-free, the only new dependency); one match seam (`wantmatch` + `notify` + `alert_log` dedup/cooldown) three sources fan in; EC = PigParse poll-and-diff (~10-min cadence, live spike first); error 50007 (can't-DM) first-class with an in-site notification inbox; `MESSAGE_CONTENT` a self-serve dev-portal toggle (no Discord audit under 100 servers); HARD CONSTRAINT — never put a Discord bot/OAuth in the watcher (untouched this milestone).

**Track 1 — ✅ SHIPPED LIVE** (deployed to api.squirebot.quest 2026-06-06, schema v8; Phases 19–21 complete — wantlist + EC-tunnel Discord pinger is a complete, standalone feature):

- [x] **Phase 19: Wantlist CRUD** — per-user wantlist (website CRUD, item-ID-keyed, buy/quest reason, Discord-identity-tied, already-in-bank flag); the product surface, no Discord yet (completed 2026-06-05)
- [x] **Phase 20: Bot + DM + Notification Infrastructure** — in-process discordgo gateway goroutine + `notify` DM sender + `wantmatch` + `alert_log` dedup/cooldown + opt-in/prefs + in-site notification inbox (50007 fallback) + per-monitor enable/disable + `guild_channel` config; the keystone spine all monitors ride (completed 2026-06-05)
- [x] **Phase 21: EC-Tunnel Auction Monitor** — first real alert; PigParse poll-and-diff → wantmatch → DM, gated behind an upfront PigParse feasibility spike (confirm timestamps advance + coverage) (completed 2026-06-06; **deployed live 2026-06-06**, schema v8, bot connected + `ec_auction_match` job running; WANT-05; live-DM smoke confirms organically per D-07 coverage)

**Track 2 — INVITE-GATED (entry-precondition: the 3 Raid Alliance bot invites confirmed in writing + `MESSAGE_CONTENT` enabled):**

- [ ] **Phase 22: WTS Cross-Server Monitor** — bot reads the 3 Raid Alliance WTS channels → name/alias matcher → DM; matcher built/tested against fixtures so it's testable without live servers *(invite-gated)*
- [ ] **Phase 23: Quest-Target Raid Monitor** — bot detects a raid-target NPC tied to a wanted item's quest → curated `quest → NPC` lookup → existing `quest_items` → DM *(invite-gated + needs the curated quest→NPC table)*
- [x] **Phase 24: Watcher test hardening (C1/C2 coverage)** — quality/tech-debt; close the Church audit's two CRITICAL coverage gaps (spellbook-upload path tests + `eqfind` walk tests) via a twin-handler refactor. Independent of the wantlist track — can run anytime. ✅ 2026-06-03
- [x] **Phase 25: Linux Watcher** — cross-cutting platform; a headless, fully-static (CGO-free) `linux/amd64` watcher build for guildies running P99 under WINE — Linux impls behind the existing build-tag seams (0600-file credential, WINE-prefix EQ discovery, CLI onboarding, XDG paths) + a tarball/systemd-user-unit/install.sh. Additive — Windows build unchanged. (LNX-01..06)

### 🔄 v2.3 — Character Assignment & Per-Character Wantlists (Phases 26–28)

**Milestone Goal:** Associate SquireBot users with specific characters, let them view those characters' inventory, and create character-tagged wantlist items that roll up to the guildwide wantlist.

**Scope:** backend (`internal/backendsrv`) + web (`web/`) ONLY. The Go **watcher is UNTOUCHED** — backend-only schema change; **no `WatcherMaxSchemaVersion` concern**. New goose migration `00009` → schema **v9** (additive/extend-only, version-stamped, idempotent; `character_id` on `wantlist_item` is NULLABLE — existing wants backfill to NULL with no data loss).

**Builds on (verified in code, do not rebuild):** `character.owner_id → owner.discord_user_id → web_user` already links each character to one upload-owner; v2.3 adds the explicit claim/assign surface (likely a many-to-many `character_assignment` layer over upload provenance). Read views stay **all-members consolidated** (CLAUDE.md LOCKED) — MYVIEW is an ADDITIVE filter, NOT access control. `wantlist_item` (00006) is per-user; v2.3 adds the optional character dimension. The EC-tunnel monitor (`internal/backendsrv/ec`) still DMs the **owner** (notifications key on `discord_user_id`).

**Locked decisions (2026-06-08):** assignment = **BOTH** member self-claim + officer assign/override; inventory = **all-members views + a "my characters" filter** (additive); wantlist = **character-tagged** wants rolling up to the guildwide list, DM still to the owner.

**Open sub-decisions (resolve in spec/plan, flagged on the phases):** multi-user assignment for shared bank toons (ASSIGN-05, Phase 26); whether the guildwide wantlist displays character/owner attribution (CWANT-04, Phase 28); whether the EC-monitor embed names the character (CWANT-05, Phase 28).

**Execution order:** strict dependency chain — **26 → 27 → 28**. The character-assignment data layer (P26) is the foundation BOTH the my-characters filter (P27) and the character-tagged wantlist (P28) build on — each needs the authoritative "which characters are mine" answer.

- [ ] **Phase 26: Character Assignment** — the data-layer foundation: `00009` migration (schema v9) adding the `character_assignment` layer; member self-claim/release + officer assign/reassign API (IDOR-safe, owner/officer-scoped, audited); the assign/claim UI ("My characters" + officer admin panel). (ASSIGN-01..06)
- [ ] **Phase 27: My-Characters Inventory Filter** — an ADDITIVE "my characters" quick-filter + single-character drill-down over the existing all-members consolidated views; visibility stays all-members (consolidated-views rule intact). (MYVIEW-01, MYVIEW-02)
- [ ] **Phase 28: Character-Tagged Wantlist** — the wantlist gains an OPTIONAL character dimension (NULLABLE `character_id` on `wantlist_item`, existing wants backfill to NULL); character-tagged wants roll up into the guildwide wantlist; per-user filter/group-by-character; the EC-monitor DM still targets the owner. (CWANT-01..06)

## Phase Details

### Phase 19: Wantlist CRUD
**Goal**: A signed-in guildie can maintain a personal, Discord-identity-tied wantlist on squirebot.quest — add items from the existing catalog with a buy/quest reason, view and remove them, and see whether each is already in the guild bank.
**Track**: 1 (UNBLOCKED)
**Depends on**: Nothing in v2.2 (builds on the live v2.0/v2.1 platform — `webadmin/account.go` pattern, Discord-OAuth session identity, item catalog)
**Requirements**: WANT-01, WANT-02
**Success Criteria** (what must be TRUE):
  1. A signed-in guildie can add an item to their wantlist by searching the existing item catalog, tagging it buy vs quest, and optionally setting a priority and note — the entry is item-ID-keyed and tied to their Discord identity (`web_user`).
  2. A guildie can view their full wantlist on squirebot.quest and remove any entry; another guildie cannot see or mutate it (IDOR-safe, owner-scoped, audited — the `account.go` security shape).
  3. Each wantlist row shows an "already in the guild bank?" indicator joined from the existing consolidated bank/view data.
  4. The `00006_wantlist.sql` goose migration applies idempotently on the live DB, creating at least `wantlist_item` and `alert_log` without disturbing the existing schema.
**Plans**: 3 plans
  - [x] 19-01-PLAN.md — migration 00006 (wantlist_item + alert_log + dedupe indexes) + owner-scoped store CRUD + D-10 catalog search store
  - [x] 19-02-PLAN.md — webadmin add/list/remove handlers (IDOR-safe, audited, validated) + readapi item-search handler + 4 RequireSession routes
  - [x] 19-03-PLAN.md — SvelteKit /wantlist page (catalog-search add form, deep in-bank holder display, remove) + api/columns/StateBlock/nav + browser-smoke
**UI hint**: yes

### Phase 20: Bot + DM + Notification Infrastructure
**Goal**: The keystone alerting spine exists — an in-process Discord bot can DM any guildie from the guild's own server; alerts are opt-in, deduplicated/cooled-down, and recorded in an in-site inbox that serves as the can't-DM fallback; officers can enable/disable each monitor and register source channels per server.
**Track**: 1 (UNBLOCKED — DM send is pure REST on the guild's own server; needs NO privileged intent and NO external invites)
**Depends on**: Phase 19 (wantlist must exist before there is anything to match; reuses `wantlist_item` + `alert_log`)
**Requirements**: WANT-03, WANT-04, WANT-08
**Success Criteria** (what must be TRUE):
  1. The `squirebot-server` binary starts an in-process discordgo gateway goroutine behind an `Enabled` feature flag — `recover()`-isolated, non-fatal on start (the HTTP API + scheduler serve even if the bot can't connect), with reconnect managed by discordgo; a bot panic can never take down the live website/ingest.
  2. The bot can DM a guildie from the guild's own Discord; when a DM is undeliverable (error 50007), the alert is marked `dm_blocked` and surfaced in the in-site notification inbox rather than silently dropped.
  3. A guildie can opt in/out of alerts and set notification preferences; repeat matches for the same `(wantlist_item, source, item)` are suppressed within a tunable per-source cooldown window; every alert attempt is recorded in `alert_log`.
  4. An officer can enable/disable each monitor and register source channels per server (feature flags + `guild_channel` rows), so Track-1 features ship with the invite-gated monitors dark and flip on as invites arrive — no rebuild.
  5. `wantmatch` (the single shared matcher, `ForItem` + `ForName`) is exercised end-to-end via a manual/test trigger that DMs a real guildie, proving the spine all three monitors will ride.
**Plans**: 5 plans
  - [x] 20-01-PLAN.md — migration 00007 (notify_prefs + monitor_flag + guild_channel + alert_log.read_at + wantlist_item.muted) + owner/officer-scoped store layer (WANT-03/04/08)
  - [x] 20-02-PLAN.md — discordgo bot package (recover-isolated, non-fatal, env config) + notify (DM + 50007→dm_blocked + dedup/cooldown + two-gate) + wantmatch seam (WANT-03/04)
  - [x] 20-03-PLAN.md — notifications + monitors handlers + per-want mute + runServe bot wiring + 12 routes + deploy-doc token + Discord-setup checkpoint (WANT-03/04/08)
  - [x] 20-04-PLAN.md — /notifications page (prefs + inbox), Toggle primitive, unread nav badge, api.ts wrappers + browser-smoke (WANT-04)
  - [x] 20-05-PLAN.md — /admin Monitors section (kill-switches + add-channel + test-alert) + /wantlist mute bell + browser-smoke (WANT-08/03)

### Phase 21: EC-Tunnel Auction Monitor
**Goal**: The first real end-to-end alert ships — when a wanted item is auctioned in the EC tunnel, the wantlister gets a DM (price + WTS/WTB; seller best-effort), all on the guild's own Discord.
**Track**: 1 (UNBLOCKED)
**Depends on**: Phase 20 (rides the `wantmatch` + `notify` + `alert_log` spine), Phase 19 (wantlist data)
**Requirements**: WANT-05
**Success Criteria** (what must be TRUE):
  1. An upfront PigParse feasibility spike (the phase's first task/gate) confirms auction timestamps advance during a live tunnel and measures coverage — defining whether the trigger is per-auction (`getdetails`) or coarser new-sighting (`lastWTSSeen`) before the plan commits.
  2. A new `scheduler.ec_auction_match` job polls PigParse per wanted item on a ~10-min cadence, diffing on the auction-timestamp cursor (`ec_auction_cursor`), and matches on exact item ID.
  3. When a wanted item is newly auctioned, the wantlister receives a DM carrying the item, price, and WTS/WTB tag (seller resolved only when resolvable); the alert is deduped/cooled per Phase 20's policy.
  4. The cursor advances only after a successful poll and the job does not replay backlog on restart — a standing auction is not re-DMed every poll.
**Plans**: 3 plans
  - [x] 21-01-PLAN.md — feasibility spike (GATE; path=getdetails, server=0, NAME key) + getdetails parser + 00008 ec_auction_cursor migration + cursor/poll-set store (completed 2026-06-06)
  - [x] 21-02-PLAN.md — notify embed send-path (D-04, same gates/dedup/alert_log) + wantmatch Hit.note (D-05) (completed 2026-06-06)
  - [x] 21-03-PLAN.md — ec package RunMatch (poll→diff→match→embed→send) + scheduler ec_auction_match job + main.go bot/scheduler reorder (completed 2026-06-06)

### Phase 22: WTS Cross-Server Monitor
**Goal**: When a wanted item is posted for sale in a Raid Alliance WTS channel, the wantlister gets a DM — a new event source wired into the Phase 20 spine.
**Track**: 2 (INVITE-GATED)
**Entry-precondition (HARD GATE)**: the 3 Raid Alliance bot invites confirmed **in writing** (admin/read permission on the WTS channels) + `MESSAGE_CONTENT` privileged intent toggled on in the Discord dev portal. The live monitor does NOT start without the invites; the matcher is built/tested against fixtures meanwhile.
**Depends on**: Phase 20 (reuses `notify`/`alert_log`/`wantmatch`) + the external invite prerequisite
**Requirements**: WANT-06
**Success Criteria** (what must be TRUE):
  1. The name/alias matcher (`wantmatch.ForName`: exact item-ID + a curated alias table for FBSS/Fungi-class abbreviations + bounded substring, WTS-filtered) is built and unit-tested against a recorded fixture corpus of real WTS lines — testable with zero live servers.
  2. With the bot invited and `MESSAGE_CONTENT` on, a `bot/wts.go` MESSAGE_CREATE handler reads the registered WTS channels (`guild_channel` rows) and a content-non-empty smoke test confirms message text arrives (not silently empty).
  3. A WTS line for a wanted item DMs the matching guildie via the existing `notify`/`alert_log` path, deduped/cooled per Phase 20's WTS-window policy; until the invites land, the monitor stays dark behind its feature flag with Track 1 unaffected.
**Plans**: TBD

### Phase 23: Quest-Target Raid Monitor
**Goal**: When a raid-target NPC tied to a wanted item's quest is announced in a Raid Alliance channel, the wantlister gets a DM — the last event source on the spine, chaining NPC → quest → item → wantlister.
**Track**: 2 (INVITE-GATED)
**Entry-precondition (HARD GATE)**: the 3 Raid Alliance bot invites confirmed **in writing** + `MESSAGE_CONTENT` enabled (as Phase 22) AND the curated `quest → raid-target NPC(s)` lookup populated (seeded from the wiki; curation can start in parallel during Track 1).
**Depends on**: Phase 20 (reuses `notify`/`alert_log`/`wantmatch`) + the existing `quest_items` table + the external invite + curated-table prerequisites
**Requirements**: WANT-07
**Success Criteria** (what must be TRUE):
  1. A curated `quest_target` table (`quest → raid-target NPC(s)`, seeded from the wiki, scoped to items guildies actually quest-want) is populated and queryable, reusing the existing `quest_items` (item ↔ quest) table for the inverse hop.
  2. A `bot/raidtarget.go` MESSAGE_CREATE handler detects a raid-target NPC name in a registered raid-announce channel and resolves NPC → quest → wanted item(s) via `quest_target` + `quest_items`.
  3. When the resolved item is on a guildie's wantlist (reason = quest), that guildie receives a DM via the existing `notify`/`alert_log` path, deduped/cooled per Phase 20's policy; until the invites + curated table land, the monitor stays dark behind its feature flag.
**Plans**: TBD

### Phase 24: Watcher test hardening (C1/C2 coverage)
**Goal**: Close the two CRITICAL test-coverage gaps from the Church of Clean Code audit (`.planning/CLEAN-CODE-REPORT.md`) so the watcher's spellbook-upload path and EQ-folder discovery are provably correct, not assumed.
**Track**: Quality / tech-debt (cross-cutting — NOT part of the v2.2 wantlist feature theme; appended to the active milestone by sequential numbering).
**Depends on**: Nothing — touches only watcher code (`internal/app/runapp.go`, `internal/eqfind`) that is orthogonal to the v2.2 wantlist/Discord track (web/server/bot). Can run before, during, or after Phases 19–23 without conflict.
**Requirements**: Audit findings C1, C2 (+ the Size/Test twin-handler refactor). No new product requirement.
**Success Criteria** (what must be TRUE):
  1. The duplicated twin upload handlers `makeOnInventoryChange`/`makeOnSpellbookChange` (`internal/app/runapp.go:314`/`:389`) are collapsed into one shared `makeOnFileChange(kind, suffix, mtimeMap …)` helper with an extracted `handleIngestErr(...)` — removing ~50 lines of copy-paste and the verbatim error-switch (`:355-372` ≡ `:419-437`), with no behavior change to the inventory path (existing tests stay green).
  2. The spellbook upload path has behavior tests mirroring the four existing inventory tests — 401-no-loop-sets-red, 426-update-needed, cross-owner reject, empty-file-skips-no-request, and 204-persists-mtime — so a future change to one path can no longer silently rot the other.
  3. `internal/eqfind`'s real filesystem-walk discovery is tested: `walkRoot` + sentinel-matching exercised against a `t.TempDir()` tree with planted `eqgame.exe`/`eqclient.ini` at varying depths plus decoy dirs, lifting the package off its ~15% floor (orchestration-only) onto the actual discovery logic.
**Plans**: 2 plans
  - [x] 24-01-PLAN.md — refactor twin upload handlers (makeOnFileChange + handleIngestErr) + spellbook behavior tests (C1, REFACTOR)
  - [x] 24-02-PLAN.md — eqfind walkRoot/sentinel-walk tests against a t.TempDir() tree (C2)

### Phase 25: Linux Watcher
**Goal**: Guildies who run Project 1999 under WINE on Linux can install and run the watcher — a headless, fully-static (`CGO_ENABLED=0`) `linux/amd64` build that watches the WINE-prefix EQ folder, uploads `/outputfile` `.txt` files over HTTPS with the static bearer guild code, and auto-updates — with the same robustness as the Windows watcher, minus the Windows GUI/installer chrome.
**Track**: Quality / platform (cross-cutting — NOT part of the v2.2 wantlist/Discord theme; appended to the active milestone by sequential numbering).
**Depends on**: Nothing — touches only watcher code + the build/release pipeline; orthogonal to the backend/web/bot. All Linux work is additive behind `//go:build` tags / `runtime.GOOS` so the Windows build is unchanged.
**Requirements**: LNX-01, LNX-02, LNX-03, LNX-04, LNX-05, LNX-06
**Success Criteria** (what must be TRUE):
  1. The watcher cross-compiles `GOOS=linux GOARCH=amd64 CGO_ENABLED=0` to a single static binary and runs **headless** (no systray) — the tray controller is a no-op on Linux and `RunApp` is unchanged; `go test ./...` and the Windows artifact are unaffected.
  2. The bearer guild code persists in a `0600` file under `$XDG_CONFIG_HOME/squirebot/` (no keyring/secret-service dependency); config + logs follow XDG base dirs; the Windows `wincred`/`%LOCALAPPDATA%` paths are untouched.
  3. EQ-folder discovery finds the install inside a WINE prefix (`$WINEPREFIX` → `~/.wine/drive_c` → common Lutris/Proton/Bottles paths via the bounded `eqfind` walk for the `eqgame.exe`/`eqclient.ini` sentinels), falling back to a CLI prompt that persists the chosen path.
  4. First-run onboarding + control are CLI (`--setup` prompts for the guild code + EQ folder over stdin; `--status` prints health/config) — no Win32 dialog, no localhost/browser surface (the watcher stays browser-free).
  5. A `.tar.gz` ships the binary + README + a systemd **user** unit + `install.sh` (installs to `~/.local/bin`, enables the unit for autostart, runs first-time `--setup`); the existing `minio/selfupdate` auto-update works on Linux with the linux asset in the manifest.
  6. The fsnotify watch + debounce + full-snapshot-replace upload + schema-version gate + log rotation all work on Linux (they are platform-agnostic) — verified by the existing suite plus new Linux-path unit tests for credstore/eqfind/config.
**Plans**: 3 plans
  - [x] 25-01-PLAN.md — CGO-free headless build seam: tray split + main.go systray split + config/logging XDG branch (LNX-01, LNX-02) ✅ 2026-06-06
  - [x] 25-02-PLAN.md — Linux runtime impls: 0600-file credstore + WINE-prefix eqfind walk + CLI --setup/--status onboarding + new unit tests (LNX-02/03/04/06) ✅ 2026-06-06
  - [x] 25-03-PLAN.md — packaging + auto-update: release.yml linux build/tarball/systemd-unit/install.sh + manifest OS-asset selection (LNX-05) ✅ 2026-06-06

### Phase 26: Character Assignment
**Goal**: Members can self-claim the characters they play (including ones uploaded under an unlinked/legacy owner) and officers can assign/override/reassign any character from the admin panel — backed by a versioned, audited assignment data layer.
**Milestone**: v2.3
**Track**: Foundation (everything in v2.3 depends on this — "which characters are mine" is the load-bearing answer)
**Depends on**: Nothing in v2.3 (builds on the live v2.0/v2.1 platform — `character.owner_id → owner.discord_user_id → web_user`, the `webadmin/account.go` IDOR-safe pattern, the Discord-OAuth session identity, the officer-gate from ADMIN-04..06, and the audit-log seam)
**Requirements**: ASSIGN-01, ASSIGN-02, ASSIGN-03, ASSIGN-04, ASSIGN-05, ASSIGN-06
**Open sub-decision (resolve in spec/plan)**: ASSIGN-05 — single- vs multi-owner for shared bank toons (likely a many-to-many `character_assignment` table layered over `owner_id`); do NOT force single-vs-multi in the roadmap.
**Success Criteria** (what must be TRUE):
  1. A signed-in member sees "My characters" — the list of characters currently assigned to them — and can self-claim a character (including one uploaded under an unlinked/legacy owner, or currently unassigned) so it appears under "My characters" (ASSIGN-01, ASSIGN-02).
  2. A member can release/unclaim a character they hold, returning it to unassigned so it (or an officer) can reassign it (ASSIGN-03).
  3. An officer can assign any character to any member, and reassign/override an existing assignment, from the admin panel — gated by the existing officer check, IDOR-safe (ASSIGN-04).
  4. The model supports the shared-bank-toon case per the ASSIGN-05 spec decision (single- vs multi-owner resolved at plan time) without a schema rework being needed to flip it.
  5. Every assignment change (self-claim, release, officer assign/reassign) is recorded in the audit log with actor, character, action, and time (ASSIGN-06).
  6. The `00009` goose migration applies idempotently on the live DB, stamping schema v9 and adding the `character_assignment` layer without disturbing the existing schema or any current data.
**Plans**: 3 plans
  - [ ] 26-01-PLAN.md — migration 00009 (character_assignment + assignment_request + is_guild_bot, auto-seed) + store layer (claim/release/request/officer/designate + reads) + is_bank_toon reconciliation (relax single-bank invariant)
  - [ ] 26-02-PLAN.md — backend API: member claim/release/request handlers (RequireSession, IDOR-safe) + officer assign/reassign/remove/approve/deny/designate (RequireOfficer + in-tx IsOfficerTx) + 12 routes + char-meta de-bank-toon + audit
  - [ ] 26-03-PLAN.md — web UI: api.ts wrappers + 'My characters' member panel/route/nav + officer assignment admin section + CharMetaForm bank-toon removal
**UI hint**: yes

### Phase 27: My-Characters Inventory Filter
**Goal**: A member can narrow the existing all-members consolidated views to just their assigned characters — a "my characters" quick-filter plus a single-character drill-down — as a purely ADDITIVE convenience, with all-members visibility unchanged.
**Milestone**: v2.3
**Track**: Read-view (additive filtering only — NOT an access-control change; consolidated-views rule LOCKED)
**Depends on**: Phase 26 (needs the authoritative "which characters are mine" assignment data to drive the filter)
**Requirements**: MYVIEW-01, MYVIEW-02
**Success Criteria** (what must be TRUE):
  1. A member can apply a "my characters" quick-filter to the consolidated views (inventory/bank/gear/spell) so they see only their assigned characters' rows — WITHOUT changing the existing all-members visibility (any member can still toggle back to all members) (MYVIEW-01).
  2. A member can drill into a single specific assigned character's inventory from that filter (MYVIEW-02).
  3. The filter is client-side/read-only over the existing consolidated DataGrid — no per-character view tabs are created and no row becomes hidden from other members (the consolidated-views architecture rule is preserved).
**Plans**: 1 plan
  - [x] 27-01-PLAN.md — pure myview.ts filter helper + node test, then the single 'My characters' <select> (All members / My characters / per-char drill-down) wired into +page.svelte feeding the four grids pre-filtered data (zero backend change) ✅ 2026-06-08
**UI hint**: yes

### Phase 28: Character-Tagged Wantlist
**Goal**: Wants gain an optional character dimension — a member can tag a want to one of their assigned characters, those wants roll up into the guildwide wantlist alongside untagged/pre-existing wants, and the member can filter/group their own wantlist by character — while the EC-tunnel monitor DM still targets the character's OWNER.
**Milestone**: v2.3
**Track**: Wantlist (the `00010`/schema-v10 `character_id` touch lands here — NOT 00009; 00009 shipped as character_assignment in P26)
**Depends on**: Phase 26 (the "my assigned characters" set is what a want can be tagged to). Reuses the live v2.2 `wantlist_item` (00006), the EC-tunnel monitor (`internal/backendsrv/ec` + `wantmatch.ForItem`), and the `notify`/`alert_log` spine.
**Requirements**: CWANT-01, CWANT-02, CWANT-03, CWANT-04, CWANT-05, CWANT-06
**Resolved sub-decisions (locked 2026-06-08)**: CWANT-04 — the guildwide wantlist SHOWS per-want character + owner (`web_user.username`) attribution, EXCLUDING the private `note`; CWANT-05 — the EC-monitor embed NAMES the character via a display-only "For <char>" field (the DM target stays the owner `discord_user_id`, structurally unchanged).
**Success Criteria** (what must be TRUE):
  1. When adding a wantlist item, a member can OPTIONALLY tag it to one of their assigned characters; wants with no character (account-level) and all pre-existing wants remain valid — the `00010` migration backfills existing `wantlist_item` rows to a NULL `character_id` with no data loss (CWANT-01, CWANT-02).
  2. Character-tagged wants aggregate ("filter up") into the guildwide wantlist alongside untagged wants; the guildwide list surfaces character/owner attribution per the CWANT-04 spec decision (CWANT-03, CWANT-04).
  3. A member can filter/group their own wantlist by character (CWANT-06).
  4. The EC-tunnel monitor DM for a character-tagged want still targets the character's OWNER (keys on `discord_user_id`); whether the embed names the character follows the CWANT-05 spec decision (CWANT-05).
**Plans**: 3 plans
  - [x] 28-01-PLAN.md — migration 00010 (nullable character_id + COALESCE-keyed dedup-index rewrite) + store (AddWantTx/ListOwnWants +char, NEW ListGuildWants) + the IsCharAssignedToTx IDOR guard + ErrCharNotAssigned
  - [x] 28-02-PLAN.md — API: AddWantHandler IDOR guard + ListGuildWantsHandler + GET /api/v1/wantlist/guild route; wantmatch.Hit.CharacterName LEFT JOIN (DM target unchanged) + EC embed "For <char>" field
  - [x] 28-03-PLAN.md — web: api.ts char fields + fetchGuildWants; pure groupByChar.ts + node test; WantAddForm character <select>; WantlistPanel My/Guild toggle + group-by-char
**UI hint**: yes

## Progress

**Execution Order:** Phases execute in numeric order. v2.0: 11 → 12 → 13 → 14 → 15 → 16 (complete). v2.1: 17 → 18 (complete). v2.2: 19 → 20 → 21 (Track 1, unblocked) → 22 → 23 (Track 2, invite-gated; can slot earlier if invites land, but Track 1 ships independently).

| Milestone | Phases | Plans Complete | Status | Completed |
|-----------|--------|----------------|--------|-----------|
| v1.0 | 5 | 31/31 | ✅ Shipped | 2026-05-11 |
| v1.0.1 | 3 | 12/12 | ✅ Shipped | 2026-05-12 |
| v1.0.2 | 2 | 8/8 | ✅ Binary shipped (milestone close superseded by v2.0) | 2026-05-13 |
| v2.0 | 6 | 29/29 | ✅ Shipped (tag `v2.0.0`; Google decommissioned) | 2026-05-31 |
| v2.1 | 2 | 4/4 | ✅ Complete (Phases 17–18 shipped) | 2026-06-02 |
| v2.2 | 6 | 13/TBD | 🔄 **Track 1 SHIPPED LIVE** (Phases 19–21 deployed 2026-06-06 + Phase 24 quality done); Track 2 (22–23) invite-gated, parked | — |
| v2.3 | 3 | 7/7 | ✅ Feature-complete — all 3 phases SHIPPED + deployed live (schema v10); pending milestone audit/close | 2026-06-09 |

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 11. Backend Foundation + Ingest API | v2.0 | 7/7 | ✅ Complete | 2026-05-29 |
| 12. Enrichment Job Migration | v2.0 | 5/5 | ✅ Complete | 2026-05-29 |
| 13. Watcher Re-Target + Onboarding | v2.0 | 4/4 | ✅ Complete | 2026-05-30 |
| 14. Web Frontend | v2.0 | 4/4 | ✅ Complete (deployed live) | 2026-05-30 |
| 15. Admin Web Forms + Login | v2.0 | 5/5 | ✅ Complete (deployed live; most UAT smokes exercised during the 16-03 deploy) | 2026-05-31 |
| 16. Cutover + Decommission | v2.0 | 4/4 | ✅ Complete (Google decommissioned; guild migrating) | 2026-05-31 |
| 17. Self-Service Watcher Linking | v2.1 | 3/3 | ✅ Complete (deployed live; browser-smoke approved; 15/15 verified) | 2026-06-02 |
| 18. Watcher Cleanups — Verify-or-Close | v2.1 | 1/1 | ✅ Complete (verify-or-close; zero new code; 0.4.0-rc1 = Azure test VM, not production) | 2026-06-02 |
| 19. Wantlist CRUD | v2.2 | 3/3 | Complete    | 2026-06-05 |
| 20. Bot + DM + Notification Infrastructure | v2.2 | 5/5 | Complete    | 2026-06-05 |
| 21. EC-Tunnel Auction Monitor | v2.2 | 3/3 | ✅ Complete + **DEPLOYED LIVE** (schema v8; bot connected, ec_auction_match job running; WANT-05) | 2026-06-06 |
| 22. WTS Cross-Server Monitor | v2.2 | 0/TBD | Not started (INVITE-GATED) | — |
| 23. Quest-Target Raid Monitor | v2.2 | 0/TBD | Not started (INVITE-GATED) | — |
| 24. Watcher test hardening (C1/C2 coverage) | v2.2 | 2/2 | ✅ Complete (C1/C2/REFACTOR closed; 10/10 verified) | 2026-06-03 |
| 25. Linux Watcher | v2.2 | 3/3 | ✅ Complete (25-03 done — OS-specific manifest assets + runtime.GOOS auto-update selection, tarball + systemd user unit + install.sh, additive release.yml linux build; linux closure CGO-free, Windows NSIS path + `go test ./...` unchanged) | Human UAT on a real Linux+WINE box (watch→upload→autostart→self-update) |
| 26. Character Assignment | v2.3 | 3/3 | ✅ SHIPPED — deployed live + browser-smoke PASS (ASSIGN-01..06) | 2026-06-08 |
| 27. My-Characters Inventory Filter | v2.3 | 1/1 | ✅ SHIPPED — deployed live + browser-smoke PASS (MYVIEW-01/02) | 2026-06-08 |
| 28. Character-Tagged Wantlist | v2.3 | 3/3 | ✅ SHIPPED — deployed live v10 + browser-smoke PASS (CWANT-01..06) | 2026-06-09 |

## Backlog

Carried forward from v1.0 / v1.0.1 / v1.0.2 (candidates for a future Sheet-orthogonal patch or v2.x). Note: several Sheet-side items below are likely **mooted by v2.0** (the Sheet + Apps Script are being decommissioned in Phase 16) — they are retained here for the record and to assess at v2.0 close.

- **999.1** Bank-coin permission lock (Sheet sidebar) — likely mooted by ADMIN-05 (web form replaces the sidebar).
- **999.2** Polished theme picker tile UI (Sheet) — mooted by WEB-05 (theme becomes a per-user client preference on the website).
- **999.5** Self-service eviction (departing guildie quits cleanly without officer action) — v2.x candidate; threat-model deferred.
- **999.7** Extract `SIDEBAR_BODY` constants to external `.html` (Sheet) — mooted by the frontend rebuild.
- **999.9** SignPath Foundation OSS approval — submitted; awaiting review (would retire INST-05 partial → full). Lands as a hotfix when approved; orthogonal to the backend swap.
- **999.11** Decide v2.x verification doctrine — adopt `/gsd-verify-work` per phase, or formalize a live-smoke pattern.
- **999.12 / WANT-01..08** v2: Wantlist + Discord pinger — prerequisites WANT-06/07 still open (Raid Alliance Discord bot invites). v2.0 pre-pays the per-user Discord-identity prerequisite via AUTH-09; v2.1's Discord-tied ownership (LINK-02) further dovetails with it.
- **999.19** Google OAuth brand verification re-approval — **SUPERSEDED by v2.0** (Google removed from the system entirely; see STATE.md). Retained for incident-trail linkage only.
- **999.20** `console_windows.go` not `gofmt -l` clean — RESOLVED in v2.0 Plan 13-04 (`c930fc2`); ✅ **CLOSED in v2.1 Phase 18 / WATCH-12** (confirmed gofmt-clean live).
- **999.21** `freeConsole()` doc-vs-impl contract mismatch (log noise) — RESOLVED in v2.0 Plan 13-04 (`c930fc2`); ✅ **CLOSED in v2.1 Phase 18 / WATCH-13** (confirmed `slog.Debug` not Warn; doc/impl reconciled).
- **999.22** SemVer-aware auto-update comparison — RESOLVED in v2.0 Plan 13-04 (`e758fb0`/`3e8e53b`); ✅ **CLOSED in v2.1 Phase 18 / WATCH-14** (confirmed `IsNewer` pre-release test green). Stuck-watcher reinstall was a stale premise — the `0.4.0-rc1` box is the disposable Azure test VM, not a production watcher; no reinstall needed.
- **999.23** Graceful tray messaging for Google policy/verification block — largely mooted by P13 (the Google OAuth path is deleted); the tray-classifier UX pattern may inform the bearer-token-rejected path.
- **999.24** `COL_RACE`/`COL_COUNT` collision (Sheet `showCharInfoSidebar.ts`) — mooted by the frontend rebuild.
- **999.25** Orphaned `squirebot:search:recent` CacheService key (Sheet) — mooted by decommission.
- **999.26** `evictionSidebar.inline.test.ts` bypasses admin gate at inline-JS layer (Sheet) — mooted by decommission.
- **999.27** `showSearchSidebar.test.ts` narrow negative assertion (Sheet) — mooted by decommission.
- **999.28** `searchIndex.ts` `didYouMean('')` contract bug — **port-relevant**: the search logic ports to the frontend in P14 (WEB-03); fix the empty-query contract during the port.
- **999.29** `test-helpers.ts` CacheService mock TTL nit (Sheet) — mooted by decommission.
- **999.30** `searchIndex.test.ts` Test 4 `didYouMean` Levenshtein contract mismatch — **port-relevant**: resolve when porting `didYouMean` to the frontend in P14 (WEB-03).
- **999.31** Self-service **"Link your watcher via Discord"** onboarding — ✅ **SHIPPED in v2.1 Phase 17 (LINK-01..06), 2026-06-02.** Guildie logs into squirebot.quest with the P15 Discord login → a "Link my watcher" action mints a guild code tied to their Discord identity → paste once into the watcher. Replaces the maintainer manually minting + DMing ~12 codes (no plaintext through the maintainer's hands; unifies web + watcher identity; self-service scales as the guild grows). **HARD CONSTRAINT:** the watcher credential stays a static bearer token — do NOT put Discord OAuth *in the watcher* (that reintroduces the exact v2.0 "Off Google" fragility: ~7-day token expiry/refresh on an unattended uploader, a public desktop client secret, a browser/loopback flow; P13 made the watcher browser-free on purpose). Discord is the identity at **link-time only**.
- **999.34** Phase 28 deferred cosmetic review items (from `28-REVIEW.md`, all **LOW/NIT, non-blocking**, captured at v2.3 audit 2026-06-09): **LOW-01** — a forged-tag `403` (tagging a character not yours) surfaces a generic add-form error rather than a specific message; reachable only via a tampered request since the `<select>` only offers the caller's own characters. **LOW-03** — account-level wantlist adds write `"character_id":null` into the audit-log detail (cosmetic audit noise; account-level is the common case). **NIT-01/02** — minor naming/comment nits. Also the Phase 27 zero-claimed-characters hint affordance (#6) + the Phase 26 non-officer `/admin` 403 UI-collapse remain un-browser-smoked (logic/server-gate code-verified; need specific account states). Batch-fix in a future polish pass; none affect correctness, security, or data.
- **999.33** Officer panel — guild-bank/bot designation is a one-way door in the UI (Phase 26 browser-smoke UAT, 2026-06-08). **MEDIUM, no data loss.** ✅ **RESOLVED + DEPLOYED LIVE 2026-06-09 (quick `260609-d2o`, commits b7b0a48/2b18d65; binary+web swap, new route 401-gated verified)** — added a `ListDesignatedChars` read + `GET /api/v1/admin/characters/designated` (officer-only) + a "Designated characters / Clear designation" section in `AssignmentAdminPanel`, so an officer can return a bank/bot char to `mode:none` from the UI. Original problem: `DesignateCharTx` clears the `character_assignment` row, so a designated char drops out of `ListAllAssignments` — the ONLY list `AssignmentAdminPanel` renders, and the host of the per-row Designate (bank/bot/none) + Reassign/Remove controls. It's also excluded from the member claimable read (`is_bank_toon=0 AND is_guild_bot=0`), so no member can claim it back. Net: once bank/bot, a character is unreachable from every UI surface and can only return to `mode:none` via a direct API/DB call. Data layer is correct and reversible — purely a UI reachability gap. Surfaced when the maintainer designated all 4 assigned chars during UAT and they vanished from the panel, requiring a scoped direct-DB recovery (backup taken; flags cleared on ids 1/14/15/16 + re-seed; real bank toon `Findom` preserved). **Fix:** surface designated chars in the officer panel (an "all characters / designations" view or a dedicated designated-chars section) so the designate control — including `mode:none` — is reachable for already-designated chars. Relates to Phase 26 (v2.3); good candidate to fold into Phase 27/28 web work or fix before v2.3 close.
- **999.32** Single-bank-toon invariant for the char-meta form (Phase 16 code-review **MD-01**) — ✅ **RESOLVED.** `SetCharMetaTx` (`internal/backendsrv/store/charmeta.go`) enforced no uniqueness on `is_bank_toon`, but `compute.Bank` assumes exactly one bank toon; flagging 2+ silently merged bank-view rows. **Fixed in commit `0e31023`:** setting `is_bank_toon=true` now clears it on all other characters in the same tx (matches v1's single-value `_meta.bank_toon_name`) + store regression tests. The same code review's route-gate test gap (**LR-01**) was fixed in `9b608a4`. The originally-bundled **LO-01** (level JSON-null contract — Go `int64` can't emit `null` vs TS `number | null`) and **LO-02** (empty-name success copy on read-back failure) were reclassified as Info/parity in the independent re-review and intentionally left as-is. Full findings in `16-REVIEW.md` / `16-REVIEW-FIX.md`.

---

*Roadmap created: 2026-04-30. v1.0 shipped: 2026-05-11. v1.0.1 shipped: 2026-05-12. v1.0.2 binary shipped: 2026-05-13. v2.0 "Off Google" shipped 2026-05-31, Phases 11–16; milestone archived (`milestones/v2.0-ROADMAP.md`). v2.1 "Self-Service Watcher Linking" shipped 2026-06-02 (Phases 17–18). Last reorganized: 2026-06-02 — appended v2.2 "Wantlist + Discord Pinger" milestone (Phases 19–23); 8/8 v2.2 requirements mapped (Track 1 unblocked: WANT-01/02/03/04/05/08 across Phases 19–21; Track 2 invite-gated: WANT-06/07 across Phases 22–23); Phase 19 ready to plan. **2026-06-08: appended v2.3 "Character Assignment & Per-Character Wantlists" (Phases 26–28); 14/14 v2.3 requirements mapped (ASSIGN-01..06 → P26 · MYVIEW-01/02 → P27 · CWANT-01..06 → P28); v2.2 Track 2 (Phases 22–23) explicitly marked PARKED/invite-gated; Phase 26 ready to plan.***
