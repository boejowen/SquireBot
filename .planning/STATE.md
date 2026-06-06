---
gsd_state_version: 1.0
milestone: v2.2
milestone_name: — Wantlist + Discord Pinger
status: executing
last_updated: "2026-06-06T04:30:00.000Z"
last_activity: 2026-06-06
progress:
  total_phases: 6
  completed_phases: 3
  total_plans: 10
  completed_plans: 10
  percent: 100
---

# State: SquireBot

**Initialized:** 2026-04-30
**Last updated:** 2026-06-02 (v2.2 "Wantlist + Discord Pinger" roadmap created — Phases 19–23)

## Project Reference

See: `.planning/PROJECT.md` (updated 2026-06-02 after v2.1 shipped)

- **Core value:** Every guildie can answer "what does my character still need, and where in the guild is it?" — delivered via the self-hosted website (squirebot.quest).
- **Current focus:** Phase 21 — EC-Tunnel Auction Monitor COMPLETE (21-03 ec.RunMatch + ec_auction_match scheduler job + main.go reorder shipped; WANT-05 delivered, Track-1 finale done; next = `/gsd-discuss-phase 22` (invite-gated) or close the milestone)
- **Mode:** yolo
- **Granularity:** coarse

## Current Position

Phase: 25 — Linux Watcher (cross-cutting platform; appended outside the v2.2 Track-1/2 numbering, like Phase 24)
Plan: 25-01 COMPLETE (1/3 plans) — CGO-free headless build seam
Status: 25-01 done — `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./...` green; **zero `fyne.io/systray` in the cmd/squirebot linux closure**; run_other.go installs the mandatory SIGINT/SIGTERM→cancel() handler (LNX-05 graceful systemd stop); config+logs branch onto XDG on Linux (Windows `%LOCALAPPDATA%` untouched). Windows build compiles + host `go test ./...` green (additive guarantee). Next: `/gsd-execute-phase 25` plan 25-02 (0600-file credstore + WINE-prefix eqfind walk + CLI --setup/--status). v2.2 Track 1 remains SHIPPED LIVE; Track 2 (22–23) parked, invite-gated.
Last activity: 2026-06-06

Phase 25 activity: 2026-06-06 — 25-01 executed (3 task commits: bb8e214, e84c6c4, 52b96c6). The highest-risk structural item of the phase: `fyne.io/systray` (CGO/GTK on Linux) was imported at TWO un-tagged sites — `internal/tray/tray.go` AND `cmd/squirebot/main.go`. Both build-tag-split: (1) `git mv tray.go→tray_windows.go` (//go:build windows, bytes unchanged) + new `tray_other.go` (//go:build !windows) reproducing the IDENTICAL exported `*tray.Controller` API as slog no-ops (RunApp signature byte-identical, D-07); test files paired the same way. (2) main.go drops the systray import; its shutdown-listener goroutine + `systray.Run` tail extracted into `run_windows.go` (//go:build windows, byte-equivalent) / `run_other.go` (//go:build !windows: `signal.NotifyContext(ctx, SIGINT, SIGTERM)` → cancel() → `<-ctx.Done()`, no systray). (3) `config.defaultPath()` + new `logging.defaultLogDir()` branch on `runtime.GOOS` — XDG config/state on Linux, `%LOCALAPPDATA%` on Windows; new XDG-branch unit tests. Gates all green: linux closure systray count = 0, dialog/sqweek count = 0, `go.mod` still lists systray (windows uses it), Windows build OK, host `go test ./...` 0 failures, `GOOS=linux go vet ./...` clean.

Track-1 closeout 2026-06-06: cross-compiled linux/amd64 → scp → install (kept `.bak`) → `systemctl restart` per `docs/backend-deploy.md`; `goose.Up` auto-applied `00008_ec_cursor` (schema **v7 → v8**, `ec_auction_cursor` table created); pre-deploy R2 backup taken. Startup logs clean: `bot connected` (guild 1483502186351562925), `listening 127.0.0.1:8090`, `scheduler started interval 10m0s jobs:4` (the +1 = `ec_auction_match`). UAT #1 (live EC DM) tracked in `21-HUMAN-UAT.md`, confirms ORGANICALLY on the first real matching tunnel auction (D-07 coverage-dependent; send path already proven live in P20). UAT #2 (live scheduler poll) confirmed at the ~10-min tick. Next: pursue the 3 Raid Alliance invites (user) to unblock Track 2 → `/gsd-discuss-phase 22`; otherwise v2.2 Track 1 stands as the delivered value.

Phase 21 activity: 2026-06-06 -- 21-03 executed (the integration finale, WANT-05). NEW `internal/backendsrv/ec` package: `RunMatch` composes poll→diff→match→embed→send — polls PigParse `getdetails/0/{itemname}` per wanted item (SPIKE: server=0 live feed, NAME key form), diffs new WTS auctions (`u∈{0,2}`; WTB never alerts) against the per-item `ec_auction_cursor` with first-sight baseline (no replay) + advance-only-on-success, matches via `wantmatch.ForItem`, and DMs a discordgo rich embed through `notify.Send` (Source `ec_auction` + WantID + Embed) — re-implementing NONE of the P20 spine (both gates + dedup + cooldownEC=22h + alert_log inherited). Registered as the `scheduler.ec_auction_match` job (~10-min `dueEC` cadence); `scheduler.Start` gained a `botSession *discordgo.Session` param and `main.go` was reordered to start the bot BEFORE the scheduler so the live session is threaded `main.go → scheduler → ec`. nil-session no-op (typed-nil-interface guard). `go test ./...` + `go vet ./internal/backendsrv/...` + `go build ./...` all green. 3 task commits (bec919c, 38412ec, 5dcd384). Next: `/gsd-discuss-phase 22` (WTS Cross-Server, invite-gated) or close v2.2 if Track 2 stays gated.
Earlier: 21-02 executed. Extended the P20 notify spine with a discordgo rich-embed send-path (D-04): added `ChannelMessageSendEmbed` to the `Sender` interface (the real `*discordgo.Session` already satisfies it — assertion holds) + an `Embed` field on `Alert`; the single SEND step branches on `a.Embed != nil` so the embed rides the EXACT same two-gate + dedup/cooldown + alert_log core (verified: `grep -c GetMonitorFlags` = 1, no duplicate gate path). Also added `wantmatch.Hit.Note *string` (D-05 "why you wanted it"), carried by the shared `scanHits` for BOTH `ForItem` and `ForName` (one scanner change). Backend-only, no web/. notify + wantmatch package tests + `go build ./...` + `go vet ./internal/backendsrv/...` green. 2 task commits (72edf87, 92b8fa8). Next: `/gsd-execute-phase 21` plan 21-03 (ec package RunMatch poll→diff→match→embed→send + scheduler ec_auction_match job + main.go bot/scheduler reorder).
Earlier: 21-01 executed (the GATING plan). Mandatory PigParse feasibility spike ran live (no checkpoint, D-08): **path chosen = per-auction `getdetails`** (presence + freshness threshold met). Two research corrections recorded in 21-SPIKE.md: (1) the LIVE Blue getdetails feed is **server=0**, NOT server=1; (2) the only working query key is the item **NAME** (the id form 400s). Delivered: `enrich.ParseItemDetail`, migration `00008_ec_cursor.sql`, and `store.GetECCursor`/`SetECCursor`/`ECPollSet`. 3 task commits (0f29fcc, ff01c77, 486cff2).
Progress: Phase 21 — 3/3 plans complete (COMPLETE)

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
| uat | P12/P14/P15 HUMAN-UAT live-smoke checklists (from v2.0) | ✅ closed 2026-06-06 — P14 (5/5) + P15 (7/7) were already `complete`; P12's lone Sheet-parity test marked **obsolete** (the Sheet was decommissioned in P16; enrichment is proven live on the prod scheduler) |

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
