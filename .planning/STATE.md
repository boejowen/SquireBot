---
gsd_state_version: 1.0
milestone: v2.2
milestone_name: — Wantlist + Discord Pinger
status: planning
last_updated: "2026-06-02T00:00:00.000Z"
last_activity: 2026-06-02 -- v2.2 roadmap created (Phases 19–23); 8/8 requirements mapped; Phase 19 ready to plan
progress:
  total_phases: 5
  completed_phases: 0
  total_plans: 0
  completed_plans: 0
  percent: 0
---

# State: SquireBot

**Initialized:** 2026-04-30
**Last updated:** 2026-06-02 (v2.2 "Wantlist + Discord Pinger" roadmap created — Phases 19–23)

## Project Reference

See: `.planning/PROJECT.md` (updated 2026-06-02 after v2.1 shipped)

- **Core value:** Every guildie can answer "what does my character still need, and where in the guild is it?" — delivered via the self-hosted website (squirebot.quest).
- **Current focus:** v2.2 — Wantlist + Discord Pinger (WANT-01..08): per-user wantlist + EC auction monitor + cross-server WTS/raid monitors, all DMing the guildie via Discord. Roadmap created; Phase 19 ready to plan.
- **Mode:** yolo
- **Granularity:** coarse

## Current Position

Phase: 19 — Wantlist CRUD (ready to plan)
Plan: —
Status: Roadmap created; Phase 19 ready to plan
Last activity: 2026-06-02 -- v2.2 roadmap created (continues phase numbering from 18; Phases 19–23). 8/8 WANT requirements mapped, no orphans (Track 1 unblocked: WANT-01/02 → P19, WANT-03/04/08 → P20, WANT-05 → P21; Track 2 invite-gated: WANT-06 → P22, WANT-07 → P23). ROADMAP.md appended (all prior-milestone history preserved); REQUIREMENTS.md Traceability filled. Next: `/gsd-plan-phase 19`.
Progress: [..........] 0% — 0/5 phases planned

## v2.2 Phase Plan (created 2026-06-02)

Phases continue from v2.1 (which ended at Phase 18). Phase dirs 11–18 exist on disk — never reuse them. v2.2 starts at **19**.

**Execution order:** 19 → 20 → 21 (Track 1, UNBLOCKED — strict dependency chain) → 22 → 23 (Track 2, INVITE-GATED). 22/23 depend on the Phase 20 spine but are otherwise gated purely on the external Raid Alliance invites; if invites land early they can slot earlier, and if they never land Track 1 still delivers a complete, valuable feature.

| Phase | Name | Track | Requirements | Success Criteria | UI |
|-------|------|-------|--------------|------------------|----|
| 19 | Wantlist CRUD | 1 (unblocked) | WANT-01, WANT-02 | 4 | yes |
| 20 | Bot + DM + Notification Infrastructure | 1 (unblocked) | WANT-03, WANT-04, WANT-08 | 5 | no |
| 21 | EC-Tunnel Auction Monitor | 1 (unblocked) | WANT-05 | 4 | no |
| 22 | WTS Cross-Server Monitor | 2 (invite-gated) | WANT-06 | 3 | no |
| 23 | Quest-Target Raid Monitor | 2 (invite-gated) | WANT-07 | 3 | no |

**Phase 19 — Wantlist CRUD:** pure web feature, the product surface — `00006_wantlist.sql` (≥ `wantlist_item`, `alert_log`); `webadmin/wantlist.go` (the `account.go` twin: login-only, IDOR-safe, audited); `web/src/routes/wantlist/` reusing the existing item catalog for add-item search; already-in-bank flag. No Discord yet.

**Phase 20 — Bot + DM + Notification Infrastructure (keystone):** in-process discordgo gateway goroutine (`recover()`-isolated, non-fatal start, reconnect, `DISCORD_BOT_TOKEN` env); `notify` DM open+send + 50007 → `dm_blocked`; `wantmatch` shared matcher (`ForItem`/`ForName`); `alert_log` dedup/cooldown; opt-in/prefs + in-site notification inbox; per-monitor enable/disable + `guild_channel` config feature flags. The spine all three monitors ride.

**Phase 21 — EC-Tunnel Auction Monitor:** first real alert. MANDATORY first task = the PigParse feasibility spike (confirm timestamps advance + coverage; decides per-auction `getdetails` vs coarser `lastWTSSeen` trigger). `scheduler.ec_auction_match` poll-and-diff on `ec_auction_cursor` → `wantmatch.ForItem` → `notify`. Advance-cursor-only-after-success; no backlog replay on restart.

**Phase 22 — WTS Cross-Server Monitor (INVITE-GATED):** entry-precondition = 3 Raid Alliance bot invites confirmed in writing + `MESSAGE_CONTENT` enabled. Build/test the name/alias matcher (`wantmatch.ForName`) against a fixture corpus first; `bot/wts.go` MESSAGE_CREATE + content-non-empty smoke test; reuse `notify`/`alert_log`. Stays dark behind feature flag until invites land.

**Phase 23 — Quest-Target Raid Monitor (INVITE-GATED):** entry-precondition = invites + `MESSAGE_CONTENT` AND the curated `quest → NPC` table populated (curation can start in parallel during Track 1). `bot/raidtarget.go` MESSAGE_CREATE → NPC detect → `quest_target` → existing `quest_items` → `wantmatch.ForItem` → `notify`. Chain: NPC → quest → item → wantlister.

## Milestone v2.2 Scope (locked 2026-06-02)

**Goal:** Guildies maintain a personal wantlist on squirebot.quest and get DMed on Discord when a wanted item appears — at EC-tunnel auction, in cross-server WTS channels, or as a raid-target tied to a wanted item's quest.

**Locked decisions (v2.2 research):**
- **Delivery/reading split** — DM delivery is UNBLOCKED (bot in the guild's own Discord reaches every guildie); only WTS/raid *reading* is invite-gated. Drives Track 1 (P19–21) vs Track 2 (P22–23).
- **In-process bot goroutine, not a separate process** — avoids two SQLite writers; `recover()`-isolated + non-fatal start + `Restart=always`. Lib: `bwmarrin/discordgo` v0.29.0 (CGO-free, the only new dependency).
- **EC = PigParse poll-and-diff** (~10-min cadence; no push feed) — live spike first (Phase 21's first task).
- **One match seam, three sources** — shared `wantmatch` + `notify` + `alert_log` (dedup/cooldown), built once in Phase 20; EC/WTS/raid fan in.
- **50007 (can't-DM) first-class** — in-site notification inbox is table stakes, the fallback that makes the unreliable DM channel acceptable.
- **`MESSAGE_CONTENT`** is a self-serve dev-portal toggle (no Discord audit under 100 servers) — gates Track 2 reading only.
- **HARD CONSTRAINT (carried):** never put a Discord bot or OAuth in the watcher — untouched this milestone.

**New schema:** one goose migration `00006_wantlist.sql` — `wantlist_item`, `alert_log`, `quest_target`, `guild_channel`, `ec_auction_cursor`. `quest_target` adds only the inverse quest → raid-NPC hop; reuses the existing `quest_items` table.

## External prerequisites (gate Track 2 only)

| # | Prerequisite | Gates | Status |
|---|--------------|-------|--------|
| 1 | 3 Raid Alliance Discord bot invites (WTS-channel read + `MESSAGE_CONTENT`) | WANT-06 (P22), WANT-07 (P23) | Un-negotiated (external/human) |
| 2 | Curated `quest → raid-target NPC(s)` lookup populated (seeded from wiki) | WANT-07 (P23) | Not started; can curate in parallel now |

## Deferred Items

Carried forward at v2.1 close (non-blocking):

| Category | Item | Status |
|----------|------|--------|
| ops | Decommission the Azure PAYG test VM (the `0.4.0-rc1` box) to stop billing | ✅ done 2026-06-02 (user decommissioned the VM) |
| signing | 999.9 — SignPath Foundation OSS approval | in flight (lands as a hotfix when approved) |
| uat | P12/P14/P15 HUMAN-UAT live-smoke checklists (from v2.0) | exercised live during deploys; never formally ticked |

## Files of Record

- `.planning/PROJECT.md` — core value, requirements, constraints, Key Decisions (updated 2026-06-02 with the v2.2 milestone).
- `.planning/REQUIREMENTS.md` — v2.2 requirements + Traceability (8/8 mapped to Phases 19–23).
- `.planning/ROADMAP.md` — phase structure (v2.2 Phases 19–23 appended; all prior-milestone history preserved) + Progress tables + Backlog.
- `.planning/research/SUMMARY-v2.2.md` — synthesized v2.2 research (authoritative technical guidance + the 5-phase shape).
- `.planning/research/ARCHITECTURE-v2.2.md` — component/table/build-order detail.
- `.planning/MILESTONES.md` — historical record (v1.0 + v1.0.1 + v2.0 + v2.1).
- `.planning/milestones/v2.1-{ROADMAP,REQUIREMENTS}.md` — the v2.1 archive.
- `.planning/config.json` — granularity (coarse), mode (yolo), workflow toggles.

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260602-u7m | Church of Clean Code audit follow-up — save report + low-risk fixes (.gitignore, CLAUDE.md dep doctrine, enrich naming) | 2026-06-03 | 922b441 | [260602-u7m-church-audit-follow-up-save-report-low-r](./quick/260602-u7m-church-audit-follow-up-save-report-low-r/) |

---

*State updated: 2026-06-02 for v2.2 "Wantlist + Discord Pinger". Roadmap created 2026-06-02 (Phases 19–23; continues numbering from 18). Prior: v1.0 (2026-05-11), v1.0.1 (2026-05-12), v1.0.2 binary (2026-05-13, superseded), v2.0 (2026-05-31), v2.1 (2026-06-02).*
