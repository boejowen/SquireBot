# Project Research Summary -- v2.2 Wantlist + Discord Pinger

**Project:** SquireBot (Project 1999 EverQuest guild tool, ~12 guildies)
**Domain:** Per-user wantlist + Discord-DM alerting bolted onto an existing live Go + SQLite + SvelteKit backend
**Researched:** 2026-06-02
**Confidence:** HIGH

> Source docs: STACK-v2.2.md, FEATURES-v2.2.md, ARCHITECTURE-v2.2.md, PITFALLS-v2.2.md.
> These are the v2.2-milestone research files; the unsuffixed STACK.md / FEATURES.md / ARCHITECTURE.md / PITFALLS.md / SUMMARY.md hold the older v1/v2.0 research CLAUDE.md references and are intentionally untouched.

## Executive Summary

v2.2 adds a personal wantlist (website CRUD, tied to each guildie Discord identity) plus four alert monitors that DM a guildie when a wanted item shows up: at EC-tunnel auction (via PigParse), in cross-server WTS trade channels, and as a raid-target NPC tied to a wanted-item quest. This is not a greenfield build -- it is integration work on the v2.0/v2.1 stack (single static cmd/squirebot-server binary, internal/backendsrv/*, SQLite via modernc + goose, SvelteKit web/). The four researchers converged hard: add exactly one new Go dependency (bwmarrin/discordgo v0.29.0, pure-Go/CGO-free so the static cross-compile is unchanged), one goose migration, three new backend packages, and a SvelteKit route. Everything else is reuse.

The load-bearing architectural insight that shapes the whole milestone is a delivery/reading split: DM delivery is fully UNBLOCKED because the bot lives in the guild OWN Discord (where every guildie already is via the v2.0 membership-gated login), so it shares a server with every alert recipient and can DM all of them. Only WTS/raid-channel reading is invite-gated, depending on the 3 un-negotiated Raid Alliance server invites. This cleanly splits the work into Track 1 (wantlist -> DM/notification infra -> EC monitor, ships immediately) and Track 2 (WTS monitor -> quest-target raid monitor, gated on invites). A bot feature-flag + guild_channel rows let Track 1 ship with WTS/raid dark and flip them on as invites arrive -- no rebuild.

The two biggest risks both echo the v2.0 origin story (escaping an external verification gate) -- but resolve favorably. (1) PigParse exposes no auction push/stream and no global auctions-since-T endpoint; per-item timestamped records DO exist (getdetails/{server}/{item} -> ItemAuctionDetail[] with u/i/p/t, plus Item.lastWTSSeen/lastWTBSeen), so the EC monitor is a per-wanted-item poll-and-diff on a ~10-min cadence (PigParse rebuilds every ~10 min). The feature is FEASIBLE but warrants an early live spike (confirm timestamps advance + coverage) as the first task of the EC phase; seller-name resolution is best-effort. (2) The MESSAGE_CONTENT privileged intent is required to read WTS text, but it is a self-serve dev-portal toggle with NO Discord audit under 100 servers (~4 here) -- the opposite of the Google brand-verification trap; the only risk is forgetting the toggle. Beyond those: error 50007 (cannot DM this user) must be first-class with an in-site notification-log fallback, dedup/cooldown is mandatory to avoid notification fatigue, and at ~12 users the whole thing stays a SQLite table + a ticker + a recover boundary -- no queues, no sharding.

## Key Findings

### Recommended Stack

The entire new-dependency footprint is **one** library. Wantlist storage is new SQLite tables via one goose migration (next after 00005); CRUD reuses the hand-rolled net/http + webadmin handlers and the existing Discord-OAuth session identity; EC polling reuses enrich/politefetch + the in-process scheduler; the bot token reuses the root-only /etc/squirebot/squirebot.env EnvironmentFile. No ORM, no second DB, no cron lib, no secret manager, no second binary.

**Core technologies:**
- **github.com/bwmarrin/discordgo v0.29.0** -- Discord gateway client (read WTS/raid channels) + REST (send DMs); the de-facto Go library, pure-Go/CGO-free (so CGO_ENABLED=0 static build is unchanged), actively maintained. The only credible alternative (andersfylling/disgord) was archived July 2024.
- **modernc.org/sqlite + goose** (REUSE) -- wantlist + alert + cursor tables in one forward-only migration (00006_wantlist.sql), same pattern as 00001-00005.
- **Existing in-process scheduler + politefetch** (REUSE) -- the EC poll job rides alongside the daily-PigParse / weekly-wiki jobs; no new HTTP client, no cron daemon.

### Expected Features

This is a 12-person trust-rich guild, not a SaaS product -- several public-product table stakes (granular consent UX, multi-channel routing, abuse controls) collapse to trivial or skippable.

**Must have (table stakes):**
- **Wantlist CRUD** -- add/remove by catalog search, **item-ID keyed** (not name strings), Discord-identity-tied, with **want-reason (buy vs quest)** as the routing key for which monitors fire.
- **Notification infrastructure (shared, built ONCE)** -- DM consent / 50007 handling, per-monitor on/off toggle, **alert dedup + cooldown** (~30-60 min tunable; no canonical value exists), and an **in-site notification log / fallback inbox** (the safety net that makes the unreliable DM channel acceptable -- table stakes, not a nicety).
- **EC-tunnel auction monitor** -- scheduled PigParse poll-and-diff on the auction timestamp cursor, exact item-ID match, WTS filter + price threshold, DM via the shared infra.

**Should have (competitive / differentiators):**
- **Per-item price threshold** (only alert under X pp) -- the #1 noise-killer for buy-wants (seed default from PigParse 30-day avg).
- **Already-in-the-guild-bank inline flag** on wantlist rows -- joins straight to the existing consolidated bank/view data; cheap, on-brand, reinforces Core Value.
- **Curated quest-item -> raid-target NPC(s) lookup** + WTS name-variant/alias handling (TunnelQuestBot proven rule: known items -> exact full-name match, unknown/user terms -> bounded substring; small curated alias table for FBSS/Fungi-class abbreviations).
- **Snooze / mute per-want.**

**Defer (v2.x / future):**
- Digest mode + quiet hours (only if soak shows instant DMs are too noisy).
- Auto-suggest wants from gear_check / spell_check MISSING rows.
- Read-only guild-aggregate wantlist (only on explicit request).
- Anti-features to reject outright: free-form fuzzy/Levenshtein chat matching, matching WTB for buy-wants, cross-server (Green/Red) reconciliation (Blue-only locked), multi-channel routing, shared-channel alert spam, spawn-window prediction, raid/DKP management.

### Architecture Approach

Everything ships inside the existing single static binary. runServe() starts the bot as an **in-process goroutine** right after scheduler.Start and before ListenAndServe -- a non-fatal start (the HTTP API + scheduler must serve even if the bot cannot connect), behind an Enabled feature flag, with a recover() boundary in every gateway handler. In-process is chosen over a separate service primarily to avoid **two SQLite writers on one file** (the classic database-is-locked trap); discordgo owns its own reconnect/heartbeat goroutines and does not block the HTTP listener, and systemd Restart=always already covers crash recovery. The headline pattern is **one match seam, three sources**: a wantmatch package + notify DM sender + alert_log (dedup/cooldown) spine, built once, with the EC poll, WTS messages, and raid-target messages all fanning in -- so Track 1 builds and tests the matcher and the gated paths become wire-a-new-event-source-into-an-existing-tested-matcher.

**Major components (NEW unless noted):**
1. **store + 00006_wantlist.sql** -- new tables wantlist_item, alert_log, quest_target, guild_channel, plus a per-item EC poll cursor (ec_auction_cursor). Quest-target reuses the EXISTING quest_items table (item <-> quest); quest_target only adds the inverse quest -> raid-NPC lookup (human-curated, can start in parallel now).
2. **wantmatch** -- the single match function (ForItem for stable EC/PigParse IDs; ForName for fuzzy chat text), shared by all three sources.
3. **notify** -- opens the DM channel (UserChannelCreate) + sends (ChannelMessageSend), records every attempt in alert_log (sent / dm_blocked / error).
4. **bot** -- long-lived discordgo gateway connection (in-binary goroutine, reconnect-managed, recover()-guarded); wts.go + raidtarget.go MESSAGE_CREATE handlers (invite-gated). Owns the single Session shared with notify.
5. **webadmin/wantlist.go + web/src/routes/wantlist/** -- CRUD handlers (the account.go twin: login-only, identity from session, IDOR-safe, audited) + the SvelteKit page reusing the existing item catalog for add-item search.
6. **scheduler.ec_auction_match** -- one new job in the existing registry; PigParse poll-and-diff -> wantmatch -> notify.

### Critical Pitfalls

1. **PigParse auction data assumed, never verified** -- there is NO push/stream and NO since-T delta endpoint; only per-item timestamped history. Avoid by running a **2-4h live spike as the FIRST task of the EC phase** (confirm tunnelTimestamp/lastWTSSeen advance during a live tunnel, measure coverage); documented fallback is new-sighting-within-window alerting if coverage is thin. De-risk EARLY.
2. **MESSAGE_CONTENT intent treated as automatic** -- without the dev-portal toggle, message content arrives empty and matching silently fails (gateway still connects, so it looks like a regex bug). Good news: NO Discord audit at ~4 servers. Avoid with a 30-min toggle + a content-non-empty smoke test + a startup intents= log line.
3. **Un-DMable users (error 50007)** -- possessing discord_user_id is NOT enough; the bot must share a server (the guild OWN Discord) with the recipient and the user must allow DMs. Treat 50007 as a **first-class expected outcome**: mark dm_blocked, surface on the in-site notification log; never silently drop.
4. **Invite-gated code built before invites exist (stranded code)** -- sequence so invite-independent work ships first; make 3-invites-confirmed-in-writing a hard phase entry-precondition, not an in-phase task. Build the WTS/raid matcher against recorded **fixture** messages so it is testable without the live servers.
5. **Alert spam / dedup / cooldown failure + gateway crash** -- poll-and-diff re-emits the same standing auction every poll without a dedup key on a stable event identity + per-user-per-item cooldown + advance-cursor-only-after-success (and do not replay backlog on restart). Separately, an unrecovered bot panic must never take down the live website/ingest -- the recover() boundary is non-negotiable.

## Implications for Roadmap

Phases continue at **19** (after v2.1 Phases 17-18). The delivery/reading split drives two tracks; Track 1 ships complete, valuable value (wantlist + EC pings on the guild own Discord) with zero dependency on the external invites.

### Phase 19 -- Wantlist CRUD (Track 1, UNBLOCKED)
**Rationale:** Pure web feature, no Discord at all yet; nothing to match until wants exist. The product surface -- everything else is plumbing.
**Delivers:** 00006_wantlist.sql (at least wantlist_item, alert_log); webadmin/wantlist.go (account.go twin, login-only, IDOR-safe, audited); web/src/routes/wantlist/ reusing the item catalog for add-item search.
**Addresses:** Wantlist CRUD (item-ID keyed, buy/quest reason, price threshold, note), already-in-guild-bank flag.
**Avoids:** Pitfall 6 source-side ambiguity (catalog selection forces canonical item IDs; seed the alias table here).

### Phase 20 -- DM infrastructure + bot gateway (Track 1, UNBLOCKED)
**Rationale:** The spine all alerts ride. Build the matcher + sender + dedup once; prove end-to-end DM on the guild own server (needs NO privileged intent and NO external invites -- DM send is pure REST).
**Delivers:** bot package (discordgo session, in-binary goroutine, non-fatal start, reconnect, recover() guards, DISCORD_BOT_TOKEN env); notify (DM open+send + 50007 -> dm_blocked); wantmatch (shared matcher); alert_log dedup/cooldown; in-site notification log page.
**Uses:** bwmarrin/discordgo v0.29.0; existing EnvironmentFile secret pattern.
**Implements:** Pattern 1 (in-binary goroutine), Pattern 2 (one match seam), Pattern 3 (dedup/audit), Pattern 4 (guild-own-server delivery).
**Avoids:** Pitfalls 3 (50007 first-class + fallback inbox), 5 (dedup/cooldown), 7 (recover boundary, RESUME, /healthz bot state), 8 (no queue/shard -- table + ticker).

### Phase 21 -- EC-tunnel auction monitor (Track 1, UNBLOCKED)
**Rationale:** First real end-to-end alert; the highest-value unblocked monitor. Depends only on the scheduler + PigParse + the Phase 20 spine.
**Delivers:** scheduler.ec_auction_match job (PigParse getdetails/getmultiple poll-and-diff on the t cursor, exact item-ID match, WTS filter + price threshold) -> wantmatch -> notify; ec_auction_cursor table.
**Addresses:** EC-tunnel auction monitor (WANT-02-class).
**Avoids:** Pitfall 1 (gated behind the PigParse live spike as its first plan task) + Pitfall 5 (advance-cursor-only-after-success, no backlog replay on restart).

### Phase 22 -- WTS cross-server monitor (Track 2, INVITE-GATED)
**Rationale:** Adds a new event source to the Phase 20 spine. **Hard entry-precondition: the 3 Raid Alliance bot invites confirmed in writing** + MESSAGE_CONTENT toggled.
**Delivers:** guild_channel rows for the WTS channels; bot/wts.go MESSAGE_CREATE -> reuse PigParse POST /api/item/auctionParse for line parsing -> wantmatch.ForName (exact-known + curated-alias + bounded-substring, WTS filter) -> existing notify/alert_log.
**Uses:** MESSAGE_CONTENT privileged intent (self-serve toggle, no audit at this scale).
**Avoids:** Pitfall 6 (name-variant matcher built/tested on fixtures earlier) + Pitfall 4 (not done until it runs against at least one real invited server).

### Phase 23 -- Quest-target raid monitor (Track 2, INVITE-GATED)
**Rationale:** Last -- needs BOTH the invites AND the curated quest->NPC lookup. The chain is NPC -> quest -> item -> wantlister, reusing the existing quest_items table.
**Delivers:** seeded quest_target (curated quest->NPC); bot/raidtarget.go MESSAGE_CREATE -> NPC detect -> quest_target -> existing quest_items -> wantmatch.ForItem -> notify/alert_log.
**Addresses:** Quest-target raid monitor (WANT-04-class).

### Phase Ordering Rationale
- **19 -> 20 -> 21** is a strict dependency chain: CRUD before there is anything to match; the shared spine before its first consumer; EC as the first matcher consumer.
- **22/23 depend on 20 spine** but are otherwise independent of each other and gated purely on external prerequisites; if invites land early they can move up; if they never land, Track 1 still delivers a complete feature. The bot Enabled flag + guild_channel rows let the binary ship Track 1 with WTS/raid dark, flipped on per-server as invites arrive -- no rebuild.
- **Two de-risk tasks front-loaded:** the PigParse live spike (first task of Phase 21) and the MESSAGE_CONTENT toggle + content-non-empty smoke test (in Phase 20 / start of Phase 22).
- **Curate-in-parallel:** populating quest_target is data work that needs no code and can begin during Track 1.

### Research Flags

Phases likely needing deeper research / a spike during planning:
- **Phase 21 (EC monitor):** the **PigParse auction spike is mandatory** -- confirm timestamps advance live + coverage before committing the plan; defines whether the trigger is per-auction (getdetails) or coarser new-sighting (lastWTSSeen). Also a courtesy-contact question on ~10-min poll cadence (parallels waived ENRICH-09).
- **Phase 22 (WTS monitor):** name-variant/alias matching accuracy is the false-positive minefield; needs fixture-corpus design + the curated alias table.
- **Phase 23 (quest-target):** the quest->raid-NPC lookup is human-curation work seeded from the wiki -- size it during planning (only items guildies actually quest-want).

Phases with standard patterns (skip a research-phase):
- **Phase 19 (Wantlist CRUD):** direct twin of the v2.1 account.go self-service pattern -- established, IDOR-safe, audited.
- **Phase 20 (DM infra):** discordgo session lifecycle + DM send are well-documented; the spine patterns are fully specified in ARCHITECTURE-v2.2.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | discordgo v0.29.0 + CGO-free verified from source/proxy; PigParse Swagger fetched directly 2026-06-02. |
| Features | HIGH | PigParse API shape + Discord DM constraints verified against live OpenAPI spec + Discord docs; matching norms MEDIUM (TunnelQuestBot prior art); quest->NPC chain MEDIUM. |
| Architecture | HIGH | Integration points read directly from the live codebase; Discord platform constraints verified against Discord dev docs. |
| Pitfalls | HIGH | Discord DM/intent/gateway facts verified against official docs; PigParse auction-event availability verified directly against the live OpenAPI schema. |

**Overall confidence:** HIGH

### Gaps to Address
- **PigParse auction coverage/freshness in practice** -- feasibility is confirmed but real-world coverage (the tunnel is only fed when a human is parked parsing in EC; weekend-bursty) is unknown until the Phase 21 spike. Handle: spike-first, with the lastWTSSeen-sighting fallback pre-documented.
- **Seller-name resolution** -- ItemAuctionDetail has no seller field; only the players map may resolve it. Handle: DM names item + price reliably, seller only when resolvable.
- **The 3 Raid Alliance bot invites (external negotiation)** -- not resolvable by research; gates Track 2. Handle: hard entry-precondition invites-confirmed-in-writing; build matchers on fixtures meanwhile.
- **Quest->NPC curated table** -- data not in any API. Handle: wiki-assisted human curation, started in parallel during Track 1.
- **Cooldown interval** -- no canonical value exists even in prior art. Handle: per-source tunable constant (~20-24h EC, ~1-2h WTS/raid), finalized in REQUIREMENTS and adjusted in soak.

## Sources

### Primary (HIGH confidence)
- PigParse live OpenAPI spec -- https://pigparse.azurewebsites.net/swagger/v1/swagger.json (fetched 2026-06-02): full endpoint inventory; Auction / AuctionItem / Item / ItemDetail / ItemAuctionDetail schemas; **definitively no global recent-auction feed; per-item timestamped history only via getdetails**; ~10-min rebuild.
- bwmarrin/discordgo (Context7, 289 snippets) + proxy.golang.org + GitHub releases -- v0.29.0 latest (2025-05-24), intents / session-lifecycle / DM API; CGO-free composition.
- Discord docs -- Message Content Privileged Intent FAQ (review only at 75+/100+ servers; portal toggle required regardless), error 50007 (mutual-server + DM-privacy precondition), Rate Limits, Gateway (heartbeat/RESUME/backoff).
- Existing SquireBot codebase (read directly) -- cmd/squirebot-server/main.go, scheduler.go, webadmin/account.go, webauth/session.go, migrations 00001-00005, enrich/jobs/pigparse.go, go.mod, web/src/routes/.
- .planning/PROJECT.md -- v2.2 milestone scope, open prereqs, locked decisions; CLAUDE.md -- item-ID join key, scheduler, Discord identity, HARD CONSTRAINTs.

### Secondary (MEDIUM confidence)
- TunnelQuestBot (jamesjamail/TunnelQuestBot) -- canonical P99 auction-watch prior art: known-exact / unknown-substring matching, acronym stripping, WTS/WTB tagging, message-hash dedup.
- P1999 Wiki -- Category:Raid_Encounters / Category:Quests / epic pages (quest-item -> raid-target NPC chain structure).

### Tertiary (LOW confidence)
- Discord privileged-intent review threshold (75+ vs 100+ servers) -- community + docs consensus; not the binding constraint here (~4 servers).

---
*Research completed: 2026-06-02*
*Ready for roadmap: yes*
