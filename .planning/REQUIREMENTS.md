# Milestone v2.2 — Requirements: Wantlist + Discord Pinger

**Status:** 🔄 In progress — opened 2026-06-02. Continues phase numbering at **19**.

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

- [ ] **WANT-05**: The EC-tunnel auction monitor polls PigParse per wanted item (~10-min cadence, diffing on the auction timestamp cursor) and DMs the guildie when a wanted item is auctioned (price + WTS/WTB; seller best-effort). *(Includes an upfront feasibility spike confirming PigParse timestamps advance + coverage.)*
- [ ] **WANT-06**: The WTS cross-server monitor reads the WTS trade channels of the 3 Raid Alliance Discord servers (via `MESSAGE_CONTENT`), matches messages against wantlists (exact item-ID + a curated alias table), and DMs the matching guildie. *(Invite-gated.)*
- [ ] **WANT-07**: The quest-target raid monitor DMs a guildie when a raid-target NPC tied to a wanted item's quest is announced in those servers, resolved via a curated `quest → raid-target NPC(s)` lookup that reuses the existing `quest_items` table. *(Invite-gated + needs the curated table.)*

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
| WANT-05 | Phase 21 — EC-Tunnel Auction Monitor | Pending |
| WANT-06 | Phase 22 — WTS Cross-Server Monitor *(invite-gated)* | Pending |
| WANT-07 | Phase 23 — Quest-Target Raid Monitor *(invite-gated)* | Pending |

**Coverage:** 8/8 requirements mapped across Phases 19–23. Track 1 unblocked: WANT-01/02 (P19) · WANT-03/04/08 (P20) · WANT-05 (P21). Track 2 invite-gated: WANT-06 (P22) · WANT-07 (P23).

---

*Requirements defined: 2026-06-02 for v2.2 "Wantlist + Discord Pinger". Traceability filled by the roadmapper 2026-06-02 (8/8 mapped to Phases 19–23). Prior milestones: v1.0, v1.0.1, v2.0, v2.1 (see `milestones/`).*
