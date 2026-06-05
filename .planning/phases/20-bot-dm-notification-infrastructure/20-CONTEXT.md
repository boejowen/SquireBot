# Phase 20: Bot + DM + Notification Infrastructure - Context

**Gathered:** 2026-06-05
**Status:** Ready for planning

<domain>
## Phase Boundary

Build the **keystone alerting spine** that all three monitors (EC/WTS/raid — Phases 21–23) will ride. Phase 20 ships:
- An **in-process Discord bot** (`discordgo` gateway goroutine inside `squirebot-server`) that can DM any guildie from the guild's own Discord server.
- The shared **`wantmatch` + `notify` + `alert_log`** machinery (the single match seam, with DM open+send, 50007 handling, and dedup + cooldown).
- A guildie-facing **opt-in / preferences** surface + an **in-site notification inbox** (the can't-DM fallback) on a new `/notifications` page.
- Officer **feature-flags** to enable/disable each monitor + register source channels (`guild_channel`), so invite-gated monitors ship dark and flip on with no rebuild.

**This phase builds NO actual monitors.** There is nothing matching real events yet — EC-auction (P21), WTS (P22), and raid-target (P23) plug into this spine later. WANT-03 (the bot CAN DM), WANT-04 (opt-in + prefs + dedup/cooldown + inbox fallback), and WANT-08 (officer enable/disable + channel registration) are delivered here.

**Explicitly NOT in this phase** (redirect, don't fold): any real monitor/poller (EC/WTS/raid), the PigParse auction spike, `MESSAGE_CONTENT` reading, the quest→NPC table, DM message formats per monitor, digest mode / quiet hours, guild-aggregate wantlist.
</domain>

<decisions>
## Implementation Decisions

### Opt-in model & preferences (WANT-04)
- **D-01 (Default ON):** Guildies are alerted by **default** once monitors go live — adding a want IS the opt-in signal (they added it because they want to be pinged). No "must enable first" friction. A global mute is always one click away on `/notifications`.
- **D-02 (Granularity):** A user's own control is a **global master toggle + per-monitor toggles** (EC-auction / WTS / raid-target) — the three monitors have very different noise profiles. (Plus per-want mute, D-09.) NOT a single global-only switch; NOT per-want-only.
- **D-03 (Location):** A new **login-gated `/notifications` page** hosts BOTH the user's prefs (D-02) AND the inbox (D-04) — one "my alerts" home. Add a nav link beside "Wantlist" (the unread badge lives here too, D-05).

### Notification inbox (WANT-04 fallback)
- **D-04 (Scope — full history):** The inbox shows **every alert attempt** — delivered, `dm_blocked` (50007), and error — with can't-DM rows flagged. It is both the "what was I pinged about?" log and the 50007 safety net. Reuses `alert_log` (which records every attempt — essentially free). NOT dm_blocked-only.
- **D-05 (UX — unread state):** Read/unread with an **unread-count badge** on the nav "Notifications" link + **mark-read** (per-row and "mark all read"). The badge is load-bearing: it's how an undeliverable alert actually gets noticed. Requires a read-state on alerts (e.g. an `alert_log.read_at` column). Can't-DM rows carry a short actionable hint ("enable server DMs to receive these" — exact copy = Claude's discretion).

### Officer monitor controls (WANT-08)
- **D-06 (Location):** Officer controls live as a new **"Monitors" section on the existing `/admin` officer surface** (RequireOfficer), alongside eviction / bank-coin / officer-mgmt. NOT a separate `/admin/monitors` route.
- **D-07 (Method — DB-backed UI):** Per-monitor **enable/disable toggles** + an **"add channel" form** (server label + channel ID + monitor type) writing **`guild_channel`** rows. Officers flip monitors on with **NO redeploy**. WTS + raid-target default **OFF/dark** (invite-gated, Phases 22–23); the channel-registration UI is built slightly **ahead of its consumers by design** ("ship dark, flip on as invites arrive"). NOT CLI-only, NOT env/config-only.
- **D-08 (Officer flag = global kill-switch):** A per-monitor enable/disable is a **guild-wide** on/off (the officer decides whether a monitor runs at all) — distinct from a *user's* own per-monitor opt-in (D-02). An alert fires only when BOTH the officer flag is ON and the user opted in.

### Per-want control & DM proof (WANT-03)
- **D-09 (Per-want mute):** Ship a per-want **mute** now — a `muted` boolean on `wantlist_item`, surfaced as a bell/bell-off toggle on the `/wantlist` grid ("stop pinging me about THIS item"). Timed **snooze-until is deferred** (mute/unmute is enough at this scale).
- **D-10 (DM proof = officer test button):** WANT-03's "the bot CAN DM a guildie" is proven via an officer **"Send me a test alert"** button in the `/admin` Monitors section — DMs a sample alert to the clicking officer and logs it to their inbox. Doubles as a bot-health pulse and naturally exercises the 50007 path if the officer has DMs off. (A CLI `test-dm` could be added later but is not required now.)

### Locked upstream (v2.2 research — DO NOT re-decide; planner must honor)
- **In-process `discordgo` v0.29.0 goroutine** started in `runServe()` after `scheduler.Start` and before `ListenAndServe` — **non-fatal start** (HTTP API + scheduler must serve even if the bot can't connect), behind an `Enabled` feature flag, with a **`recover()` boundary in every gateway handler** (a bot panic must never take down the website/ingest). `DISCORD_BOT_TOKEN` via the root-only `/etc/squirebot/squirebot.env` EnvironmentFile. In-process (not a 2nd process) to avoid two SQLite writers.
- **One match seam, three sources:** `wantmatch` (`ForItem` for stable IDs / `ForName` for fuzzy chat) + `notify` (`UserChannelCreate` → `ChannelMessageSend`) + `alert_log`, built once.
- **Error 50007 (can't-DM) is first-class:** mark `dm_blocked`, surface in the inbox, never silently drop.
- **Dedup + cooldown mandatory:** dedup on a stable event identity (per `wantlist_item` × source × item) + per-source cooldown; advance-cursor-only-after-success; never replay backlog on restart.
- **HARD CONSTRAINT:** no Discord bot or OAuth in the watcher — all bot/DM work is backend + the guild's own Discord.

### Claude's Discretion
- Per-source **cooldown default *values*** — the mechanism is a per-source tunable constant in Phase 20; the EC value is finalized in Phase 21's spike. Pick sane placeholders (research suggests ~20–24h EC, ~1–2h WTS/raid), adjust in soak.
- Exact **DM message format** — the test-alert (D-10) is a representative sample; real per-monitor formats are Phase 21+.
- **Bot presence/status/activity** string and exposing **bot connection state on `/healthz`** (observability — PITFALLS-v2.2 Pitfall 7).
- The Phase-20 **goose migration shape** (next after `00006`): `guild_channel`, a per-user notify-prefs table (master + per-monitor toggles), `alert_log.read_at` (read-state), `wantlist_item.muted`. Extend-only; `alert_log` already exists from `00006` (full shape, zero rows).
- Exact **copy** for the can't-DM hint and the `/notifications` page.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Milestone scope & locked decisions
- `.planning/REQUIREMENTS.md` — WANT-03, WANT-04, WANT-08 (the 3 requirements this phase delivers) + the v2.2 locked decisions.
- `.planning/ROADMAP.md` §"Phase 20: Bot + DM + Notification Infrastructure" — goal + 5 success criteria (the acceptance contract).
- `.planning/PROJECT.md` — v2.2 milestone scope, the HARD CONSTRAINT (no bot/OAuth in the watcher), delivery/reading split.
- `CLAUDE.md` — item-ID join key, in-process scheduler, structured `slog` logging, Discord identity, schema-evolution (extend-only) rules.

### v2.2 research (authoritative technical guidance — already done; planning can likely skip a fresh research pass)
- `.planning/research/SUMMARY-v2.2.md` — synthesized guidance; §"Phase 20 — DM infrastructure + bot gateway" (the spine patterns + which pitfalls it must avoid).
- `.planning/research/ARCHITECTURE-v2.2.md` — component/pattern detail: the `bot` / `notify` / `wantmatch` / `alert_log` / `guild_channel` design, Patterns 1–4 (in-binary goroutine, one match seam, dedup/audit, guild-own-server delivery).
- `.planning/research/PITFALLS-v2.2.md` — Pitfall 3 (50007 first-class + fallback inbox), 5 (dedup/cooldown + gateway crash), 7 (recover boundary, RESUME, `/healthz` bot state), 8 (no queue/shard — table + ticker).
- `.planning/research/STACK-v2.2.md` — `bwmarrin/discordgo` v0.29.0 rationale (pure-Go/CGO-free, only new dependency).
- `.planning/research/FEATURES-v2.2.md` — notification-infra table-stakes vs deferred (consent/50007, per-monitor toggle, dedup/cooldown, fallback inbox).

### Pattern twins (closest existing analogs to copy)
- `internal/backendsrv/webadmin/account.go` (+ `account_test.go`) — login-only, owner-scoped, IDOR-safe, audited handler twin for the `/notifications` prefs + inbox handlers (the v2.1 D-02 security shape: owner from session, never the body).
- `internal/backendsrv/webadmin/` officer forms (eviction / bank-coin / officer-mgmt) — the **RequireOfficer** twin for the `/admin` Monitors section.
- `internal/backendsrv/webadmin/audit.go` — the audit-log seam (`AppendAuditTx`, `withTx`) for the officer monitor mutations.
- `cmd/squirebot-server/main.go` — `runServe()` startup order (scheduler → bot goroutine → `ListenAndServe`); the `mux.Handle(... RequireSession/RequireOfficer ...)` route block.
- `internal/backendsrv/scheduler/scheduler.go` — the in-process job registry the bot goroutine starts beside (the recover-isolated goroutine precedent).
- `internal/backendsrv/webauth/session.go` — `RequireSession` (prefs/inbox routes) + `RequireOfficer` (monitor routes) gates.
- `internal/backendsrv/migrations/00006_wantlist.sql` — `alert_log` already exists here (full shape, zero rows); the Phase-20 migration extends from this.
- `docs/backend-deploy.md` §7.1 — the root-only `/etc/squirebot/squirebot.env` EnvironmentFile pattern (where `DISCORD_BOT_TOKEN` goes).
- `web/src/routes/account/`, `web/src/lib/components/WatcherCodesPanel.svelte`, `StateBlock.svelte`, `ConfirmDialog.svelte`, `DataGrid.svelte` — SvelteKit twins for the `/notifications` page (load → grid/list → empty state).
- `web/src/lib/components/SiteShell.svelte` — the nav link + where the unread badge attaches.
- `web/src/routes/wantlist/` + `web/src/lib/components/WantlistPanel.svelte` — the Phase-19 grid that gains the per-want mute bell (D-09).
</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`webadmin/account.go`** — direct structural template for the `/notifications` prefs + inbox handlers (session-derived owner, owner-scoped queries, IDOR guards, audit calls, table-driven tests).
- **The `/admin` officer forms + `RequireOfficer`** — template for the Monitors section (WANT-08).
- **`scheduler` in-process job registry + the runServe startup order** — the recover-isolated goroutine precedent; the bot goroutine starts the same way.
- **`alert_log` (from `00006`)** — already created (full Phase-20 shape, zero rows); the inbox reads it, `notify` writes it; add only a `read_at` read-state column.
- **`wantlist_item` (from `00006`)** — gains a `muted` column (D-09).
- **The EnvironmentFile secret pattern** — `DISCORD_BOT_TOKEN` rides the existing root-only `squirebot.env` (like the Discord OAuth secrets).
- **SvelteKit `StateBlock` / `DataGrid` / `ConfirmDialog` / `SiteShell` nav** — compose the `/notifications` page + the nav badge with existing primitives.

### Established Patterns
- In-process goroutine started in `runServe()` (non-fatal, recover-guarded) — the bot mirrors the scheduler.
- Extend-only, idempotent `goose` migrations (`00001`→`00006`); the Phase-20 migration is the next forward-only one.
- `RequireSession` (member, login-only) vs `RequireOfficer` (officer-only) gating — prefs/inbox = session; monitors = officer.
- `item_id` is the stable join key (wantmatch keys on it).
- Server-truth reload + IDOR owner-from-session (v2.1 D-02) — prefs/inbox/mute mutations derive the owner from `caller(ctx)`.
- Structured `slog` logging (never log a user's query / PII).

### Integration Points
- Bot goroutine wired into `runServe()` (after `scheduler.Start`, before `ListenAndServe`); `notify` shares the single `discordgo.Session`.
- `alert_log` is the shared write target (`notify`) + read source (inbox); add `read_at`.
- New `/notifications` SvelteKit route + new `/admin` Monitors section + `SiteShell` nav badge.
- `wantlist_item.muted` (D-09) is consulted by `wantmatch`/`notify` before sending; the user notify-prefs (D-02) + officer `guild_channel`/enable flags (D-07/08) are the two gates a would-be alert must pass.
</code_context>

<specifics>
## Specific Ideas

- The `/notifications` page is the single "my alerts" home — prefs on top, inbox below — so a guildie has exactly one place to answer "what am I subscribed to and what have I been pinged about?"
- The inbox is the **safety net that makes the unreliable DM channel acceptable** — can't-DM (50007) rows must be visually obvious and actionable (tell the user to enable server DMs), and the unread badge ensures they're seen.
- The officer **"Send me a test alert"** button is the bot's pulse — one click confirms gateway-up + DM-send + inbox-log end to end, and surfaces the 50007 path if the officer's own DMs are closed.
- Both gates model: officer flag (guild-wide) AND user opt-in (per-user) must allow an alert — so officers can kill a noisy monitor for everyone, and individuals can quiet what they don't want, independently.
</specifics>

<deferred>
## Deferred Ideas

- **Timed snooze-until per want** — mute ships now (D-09); a "snooze until <date>" is a later polish.
- **Digest mode + quiet hours** — research-deferred unless soak shows instant DMs are too noisy.
- **CLI `test-dm`** — the officer button (D-10) covers the proof; a box-side CLI could be added if ops want it.
- **"Retry delivery" action from the inbox** — re-send a `dm_blocked` alert once the user enables DMs; nice future polish.
- **Auto-suggest wants from `gear_check` / `spell_check` MISSING rows** — separate feature.
- **Read-only guild-aggregate wantlist** — separate feature, on explicit request only.
- **The actual monitors** — EC-auction (Phase 21), WTS (Phase 22), raid-target (Phase 23). Phase 20 is the spine they plug into; the PigParse spike, `MESSAGE_CONTENT` reading, and the quest→NPC table all belong to those phases.

### Reviewed Todos (not folded)
None — no pending todos matched this phase.
</deferred>

---

*Phase: 20-bot-dm-notification-infrastructure*
*Context gathered: 2026-06-05*
