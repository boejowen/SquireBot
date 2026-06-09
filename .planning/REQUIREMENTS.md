# Milestone v2.3 — Requirements: Character Assignment & Per-Character Wantlists

**Status:** 🔄 In progress — opened 2026-06-08. Continues phase numbering at **26**.

**Milestone goal:** Associate SquireBot users with specific characters, let them view those characters' inventory, and create character-tagged wantlist items that roll up to the guildwide wantlist.

**Builds on (verified in code, do not rebuild):** `character.owner_id → owner.discord_user_id → web_user` already links each character to one user (bound by whoever's watcher code uploaded it). Inventory is per-character; read views are all-members consolidated (Char column). `wantlist_item` (00006) is per-user with NO character_id; the EC-tunnel monitor (`internal/backendsrv/ec`) DMs the user via `wantmatch.ForItem`. Schema at v8 (latest `00008_ec_cursor`); this milestone adds `00009` → schema **v9**.

**Locked decisions (2026-06-08):**
- **Assignment = BOTH** — members self-claim (incl. characters under an unlinked/legacy owner) AND officers assign/override/reassign.
- **Inventory = ALL-MEMBERS + FILTER** — keep the all-members consolidated views (CLAUDE.md LOCKED), ADD a "my characters" filter/drill-down. ADDITIVE — not an access-control restriction.
- **Wantlist = CHARACTER-TAGGED** — wants gain an optional character dimension and aggregate into the guildwide wantlist; the EC-monitor DM still targets the character's OWNER (notifications key on `discord_user_id`).

**Scope:** backend (`internal/backendsrv`) + web (`web/`). The Go **watcher is untouched** (backend-only schema; no `WatcherMaxSchemaVersion` concern).

**Open sub-decisions (⚠ to resolve in spec/plan, flagged on the affected REQ-IDs below):** multi-user assignment for shared bank toons (ASSIGN-05); whether the guildwide wantlist displays character/owner attribution (CWANT-04); whether the EC-monitor embed names the character (CWANT-05).

---

## v2.3 Requirements

### Character Assignment (ASSIGN)

- [ ] **ASSIGN-01**: A signed-in member can see "My characters" — the list of characters currently assigned to them.
- [ ] **ASSIGN-02**: A signed-in member can self-claim a character (including one uploaded under an unlinked/legacy owner, or currently unassigned) so it appears under "My characters."
- [ ] **ASSIGN-03**: A member can release/unclaim a character they hold, returning it to unassigned so it can be reassigned.
- [ ] **ASSIGN-04**: An officer can assign any character to any member, and reassign/override an existing assignment, from the admin panel.
- [ ] **ASSIGN-05**: ⚠ *(open)* A character may be assigned to more than one member (shared bank toons) — resolve single- vs multi-owner in spec; likely a many-to-many `character_assignment` table layered over `owner_id` (upload provenance).
- [ ] **ASSIGN-06**: Every assignment change (self-claim, release, officer assign/reassign) is recorded in the audit log (actor, character, action, time).

### My-Characters Inventory Filter (MYVIEW)

- [ ] **MYVIEW-01**: A member can filter the consolidated views (inventory/bank/gear/spell) to just their assigned characters — a "my characters" quick-filter — WITHOUT changing the existing all-members visibility.
- [ ] **MYVIEW-02**: A member can drill into a single specific assigned character's inventory.

### Character-Tagged Wantlist (CWANT)

- [ ] **CWANT-01**: When adding a wantlist item, a member can optionally tag it to one of their assigned characters.
- [ ] **CWANT-02**: Wants without a character (account-level, and all pre-existing wants) remain valid — the character tag is OPTIONAL; the `00010` migration backfills existing wants to no-character with no data loss (schema → v10). *(00010/v10, NOT 00009 — 00009 shipped as the P26 character_assignment migration.)*
- [ ] **CWANT-03**: Character-tagged wants aggregate ("filter up") into the guildwide wantlist alongside untagged wants.
- [ ] **CWANT-04**: ⚠ *(open)* The guildwide wantlist surfaces character/owner attribution per want (who / which character wants it) — confirm display in spec.
- [ ] **CWANT-05**: The EC-tunnel monitor DM for a character-tagged want still targets the character's OWNER (keys on `discord_user_id`); ⚠ *(open)* the embed MAY name the character — confirm in spec.
- [ ] **CWANT-06**: A member can filter/group their own wantlist by character.

---

## v2.3 Traceability

_Maps each v2.3 REQ-ID to exactly one phase. Phases continue at **26**. All 14 v2.3 requirements mapped; no orphans, no duplicates._

| REQ-ID | Phase | Status |
|--------|-------|--------|
| ASSIGN-01 | Phase 26 — Character Assignment | Pending |
| ASSIGN-02 | Phase 26 — Character Assignment | Pending |
| ASSIGN-03 | Phase 26 — Character Assignment | Pending |
| ASSIGN-04 | Phase 26 — Character Assignment | Pending |
| ASSIGN-05 | Phase 26 — Character Assignment *(open: single- vs multi-owner)* | Pending |
| ASSIGN-06 | Phase 26 — Character Assignment | Pending |
| MYVIEW-01 | Phase 27 — My-Characters Inventory Filter | Pending |
| MYVIEW-02 | Phase 27 — My-Characters Inventory Filter | Pending |
| CWANT-01 | Phase 28 — Character-Tagged Wantlist | Pending |
| CWANT-02 | Phase 28 — Character-Tagged Wantlist | Pending |
| CWANT-03 | Phase 28 — Character-Tagged Wantlist | Pending |
| CWANT-04 | Phase 28 — Character-Tagged Wantlist *(open: guildwide attribution display)* | Pending |
| CWANT-05 | Phase 28 — Character-Tagged Wantlist *(open: EC embed names character)* | Pending |
| CWANT-06 | Phase 28 — Character-Tagged Wantlist | Pending |

**Coverage:** 14/14 v2.3 requirements mapped across Phases 26–28 (ASSIGN-01..06 → P26 · MYVIEW-01/02 → P27 · CWANT-01..06 → P28). Strict dependency order 26 → 27 → 28. Backend (`internal/backendsrv`) + web (`web/`) only; the Go watcher is untouched; `00009` migration → schema v9 (`character_id` NULLABLE).

---

# (PARKED) Milestone v2.2 — Requirements: Wantlist + Discord Pinger

**Status:** ⏸ Parked 2026-06-08 — Track 1 (WANT-01/02/03/04/05/08, Phases 19–21) SHIPPED LIVE; Track 2 (WANT-06/07, Phases 22–23) parked on the 3 Raid Alliance bot invites. LNX-01..06 (Phase 25) shipped (watcher v2.1.1). Revisit Track 2 when the invites land. Opened 2026-06-02; continued phase numbering at **19**.

**Milestone goal:** Guildies maintain a personal wantlist on squirebot.quest and get DMed on Discord when a wanted item appears — at EC-tunnel auction, in cross-server WTS channels, or as a raid-target tied to a wanted item's quest. (Backlog 999.12 / WANT-01..08.)

**Why now:** The long-deferred "v2 feature" is finally unblocked — per-user Discord identity (the #3 prerequisite) is fully paid by v2.0 AUTH-09 + v2.1 LINK-02, PigParse is confirmed usable (poll-and-diff), and the website + Discord-OAuth platform already exists. Two external prerequisites remain and gate only part of the work (see Out of Scope / sequencing).

**Locked decisions (from v2.2 research, 2026-06-02 — see `research/SUMMARY-v2.2.md`):**
- **Bot = in-process goroutine** inside `cmd/squirebot-server` (NOT a separate process — avoids two SQLite writers), `recover()`-isolated + non-fatal start + `Restart=always`. Library: `bwmarrin/discordgo` v0.29.0 (CGO-free, the only new dependency).
- **Delivery is unblocked; reading is gated.** The bot lives in the guild's OWN Discord, so it can DM every guildie today (Track 1). The 3 Raid Alliance invites are needed ONLY to READ their WTS channels (Track 2). Delivery and reading are separated by design.
- **EC monitor = PigParse poll-and-diff** (~10-min cadence; no push feed exists). An early live spike confirms timestamps advance + coverage before the phase commits.
- **One match seam, three sources** — a shared `wantmatch` + `notify` + `alert_log` (dedup/cooldown) spine built once; EC poll, WTS messages, and raid-target messages fan in.
- **50007 (can't-DM) is first-class** → an in-site notification inbox is table stakes, not optional.
- **`MESSAGE_CONTENT`** is a self-serve dev-portal toggle (no Discord audit under 100 servers) — gates Track 2 reading only.
- **HARD CONSTRAINT (carried):** never put a Discord bot or OAuth in the watcher; the watcher is untouched this milestone.

**Stack:** unchanged from v2.x — Go + SQLite (modernc, CGO-free) backend on the Hetzner VPS, SvelteKit static site, Discord OAuth2 sessions — PLUS `bwmarrin/discordgo` v0.29.0. New SQLite tables via one goose migration (`00006`): `wantlist_item`, `alert_log`, `quest_target`, `guild_channel`, `ec_auction_cursor`.

---

## v2.2 Requirements

### Wantlist (WANT)

- [ ] **WANT-01**: A signed-in guildie can add an item to their personal wantlist by searching the existing item catalog, tagging a reason (buy vs quest), and optionally a priority and note — the entry is tied to their Discord identity (`web_user`).
- [ ] **WANT-02**: A guildie can view and manage their wantlist on squirebot.quest (list all wanted items, remove any, and see an "already in the guild bank?" indicator for each).

### Notification & Bot Infrastructure (WANT)

- [ ] **WANT-03**: An in-process Discord bot (gateway goroutine inside `squirebot-server`, `recover()`-isolated, non-fatal on start) can DM a guildie from the guild's own Discord server.
- [ ] **WANT-04**: A guildie can opt in/out of alerts and set notification preferences; alerts are deduplicated with a tunable cooldown; every alert is recorded in an in-site notification inbox that serves as the fallback when a DM cannot be delivered (Discord error 50007).
- [ ] **WANT-08**: An officer can enable/disable each monitor and register the source channels per server (feature flags + `guild_channel` config), so Track-1 features ship with the invite-gated monitors dark and flip on as invites arrive — no rebuild.

### Alert Monitors (WANT)

- [x] **WANT-05**: The EC-tunnel auction monitor polls PigParse per wanted item (~10-min cadence, diffing on the auction timestamp cursor) and DMs the guildie when a wanted item is auctioned (price + WTS/WTB; seller best-effort). *(Includes an upfront feasibility spike confirming PigParse timestamps advance + coverage.)* — delivered Phase 21 (2026-06-06)
- [ ] **WANT-06**: The WTS cross-server monitor reads the WTS trade channels of the 3 Raid Alliance Discord servers (via `MESSAGE_CONTENT`), matches messages against wantlists (exact item-ID + a curated alias table), and DMs the matching guildie. *(Invite-gated.)*
- [ ] **WANT-07**: The quest-target raid monitor DMs a guildie when a raid-target NPC tied to a wanted item's quest is announced in those servers, resolved via a curated `quest → raid-target NPC(s)` lookup that reuses the existing `quest_items` table. *(Invite-gated + needs the curated table.)*

### Linux Watcher (LNX) — cross-cutting platform (Phase 25)

- [x] **LNX-01**: The watcher cross-compiles `GOOS=linux GOARCH=amd64 CGO_ENABLED=0` to a single static binary and runs **headless** (no systray); the tray controller is a no-op on Linux, `RunApp` is unchanged, and the Windows build + `go test ./...` are unaffected (additive behind build tags / `runtime.GOOS`).
- [x] **LNX-02**: The bearer guild code persists in a `0600` file under `$XDG_CONFIG_HOME/squirebot/` (no OS keyring / secret-service dependency); config + logs follow XDG base dirs; Windows `wincred`/`%LOCALAPPDATA%` paths are untouched.
- [x] **LNX-03**: EQ-folder discovery finds the install inside a WINE prefix (`$WINEPREFIX` → `~/.wine/drive_c` → common Lutris/Proton/Bottles paths via the bounded `eqfind` walk for `eqgame.exe`/`eqclient.ini`), falling back to a CLI prompt that persists the chosen path.
- [x] **LNX-04**: First-run onboarding + control are CLI — `--setup` prompts for the guild code + EQ folder over stdin, `--status` prints health/config — with no Win32 dialog and no localhost/browser surface (watcher stays browser-free).
- [x] **LNX-05**: A `.tar.gz` ships the static binary + README + a systemd **user** unit + `install.sh` (installs to `~/.local/bin`, enables the unit for autostart, runs first-time `--setup`); `minio/selfupdate` auto-update works on Linux with the linux asset in the update manifest.
- [x] **LNX-06**: The fsnotify watch + 500 ms debounce + full-snapshot-replace upload + `WatcherMaxSchemaVersion` gate + log rotation all function on Linux, covered by the existing suite plus new Linux-path unit tests for credstore / eqfind / config.

---

## Future Requirements (deferred, not this milestone)

- **Wantlist sharing / guild-wide "who wants what" view** — a roll-up of everyone's wants; useful but a separate feature once individual wantlists exist.
- **WTB (want-to-buy) monitoring** — alerting on others' WTB posts matching items you have to sell; inverse of the wantlist; deferred.
- **Price-threshold alerts** ("only DM if under X pp") — a refinement on WANT-05/06; ships later if noise warrants.
- **Auto-derived quest→NPC table** — machine-deriving the chain from the wiki rather than curating; deferred (curation is the honest near-term path).

## Out of Scope (explicit exclusions)

| Exclusion | Reason |
|-----------|--------|
| A Discord bot or OAuth *in the watcher* | HARD CONSTRAINT — the watcher stays browser-free / bot-free; all bot work is backend + the guild's own Discord. |
| A separate bot process/service | In-process goroutine avoids two SQLite writers (`database is locked`); isolation bought back with `recover()` + `Restart=always`. |
| Real-time auction push / webhooks | PigParse exposes no push feed or since-T delta — poll-and-diff is the only honest design. |
| Building WTS/raid monitors before invites exist | Track 2 (WANT-06/07) is hard-gated on the 3 Raid Alliance bot invites being confirmed in writing; the matcher is built against fixtures meanwhile, but the live monitors do not start without invites. |
| Queues / sharding / scale infrastructure | ~12 guildies — right-size everything; no notification fatigue, no scale machinery. |
| Negotiating the Raid Alliance invites / populating the quest→NPC table | External/human work, tracked as prerequisites, not engineering requirements. |

---

## Prerequisites (external / human — gate Track 2 only)

1. **3 Raid Alliance Discord bot invites** with WTS-channel read permission + `MESSAGE_CONTENT` enabled — un-negotiated. Gates WANT-06 and WANT-07.
2. **Curated `quest → raid-target NPC(s)` lookup** populated (seeded from the wiki). Gates WANT-07. Curation can start in parallel now.

---

## Traceability

_Maps each REQ-ID to exactly one phase. Phases continue at 19. All 8 v2.2 requirements mapped; no orphans, no duplicates._

| REQ-ID | Phase | Status |
|--------|-------|--------|
| WANT-01 | Phase 19 — Wantlist CRUD | Pending |
| WANT-02 | Phase 19 — Wantlist CRUD | Pending |
| WANT-03 | Phase 20 — Bot + DM + Notification Infrastructure | Pending |
| WANT-04 | Phase 20 — Bot + DM + Notification Infrastructure | Pending |
| WANT-08 | Phase 20 — Bot + DM + Notification Infrastructure | Pending |
| WANT-05 | Phase 21 — EC-Tunnel Auction Monitor | Complete (2026-06-06) |
| WANT-06 | Phase 22 — WTS Cross-Server Monitor *(invite-gated)* | Pending |
| WANT-07 | Phase 23 — Quest-Target Raid Monitor *(invite-gated)* | Pending |
| LNX-01 | Phase 25 — Linux Watcher | Complete (code, 2026-06-06; live UAT pending) |
| LNX-02 | Phase 25 — Linux Watcher | Complete (code, 2026-06-06; live UAT pending) |
| LNX-03 | Phase 25 — Linux Watcher | Complete (code, 2026-06-06; live UAT pending) |
| LNX-04 | Phase 25 — Linux Watcher | Complete (code, 2026-06-06; live UAT pending) |
| LNX-05 | Phase 25 — Linux Watcher | Complete (25-03 ✅ 2026-06-06) |
| LNX-06 | Phase 25 — Linux Watcher | Complete (code, 2026-06-06; live UAT pending) |

**Coverage:** 8/8 v2.2 wantlist requirements mapped across Phases 19–23 (Track 1 unblocked: WANT-01/02 P19 · WANT-03/04/08 P20 · WANT-05 P21; Track 2 invite-gated: WANT-06 P22 · WANT-07 P23). Plus 6 cross-cutting platform requirements (LNX-01..06 → Phase 25, Linux Watcher).

---

*Requirements defined: 2026-06-02 for v2.2 "Wantlist + Discord Pinger". Traceability filled by the roadmapper 2026-06-02 (8/8 mapped to Phases 19–23). Prior milestones: v1.0, v1.0.1, v2.0, v2.1 (see `milestones/`).*
