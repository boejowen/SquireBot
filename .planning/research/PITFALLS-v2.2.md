# Pitfalls Research

**Domain:** Adding a Discord bot + alerting subsystem to an existing self-hosted Go/SQLite app (P1999 EverQuest guild tool, ~12 users)
**Researched:** 2026-06-02
**Confidence:** HIGH (Discord DM/intent/gateway facts verified against official docs; PigParse auction-event availability verified directly against the live OpenAPI schema)

> Scope note: this milestone (v2.2) bolts a **stateful, always-on Discord gateway bot** and a **wantlist→DM alerting pipeline** onto a system whose whole identity (v2.0) was *escaping an external platform's verification gate*. The single biggest risks are repeating that trap with Discord, building invite-gated code before the invites exist, and assuming PigParse exposes auction events it might not. Pitfalls below are ordered by how early they must be de-risked.

---

## Critical Pitfalls

### Pitfall 1: PigParse auction-event data assumed, never verified (EC-monitor built on sand)

**What goes wrong:**
The EC-tunnel auction monitor (WANT-04) is specified as "DM when a wanted item is auctioned, fed by PigParse." The phase gets built assuming PigParse exposes *auction events* (who auctioned what, when). If it only exposed daily averages, the entire feature would be infeasible as written and the phase would be stranded after the code is done.

**Resolution from this research (de-risk already partially done):**
PigParse's live OpenAPI schema (`/swagger/v1/swagger.json`) **does** carry per-auction event data, so the feature is feasible — but with sharp caveats that must shape the design:
- `Auction { player, tunnelTimestamp, items[] }` and `ItemAuctionDetail { u, i, p, t }` are individual, timestamped auction records — not just averages.
- `Item.lastWTSSeen` / `Item.lastWTBSeen` are last-sighting timestamps — a cheap signal to poll.
- `GET /api/item/get/{server}/{itemname}` is the fast averages+counts call; `GET /api/item/getdetails/...` returns "Full history on item" and is explicitly flagged "might transfer ALOT of data — only use if you need it."
- **The data is rebuilt every 10 minutes** (per the `getall` summary) → alerting latency floor is ~10 min, not real-time. Acceptable for this guild; do not promise instant.
- **There is no documented "auctions since timestamp T" delta endpoint.** The monitor must poll-and-diff: track the last-seen `tunnelTimestamp`/`lastWTSSeen` per watched item and emit on advance.
- **PigParse's auction data only exists when a human is parked in EC running a parser feeding it.** Coverage is best-effort and bursty (the tunnel is busiest on weekends/Sunday auction day). Quiet periods = no events, not a bug.

**Why it happens:**
"Fed by PigParse" reads like a solved problem because PigParse already powers the daily price enrichment (v1/v2). But *price aggregation* and *event streaming* are different capabilities; the existing integration only uses `getall` for averages.

**How to avoid:**
Run a **2–4 hour spike BEFORE committing the EC-monitor phase plan**: hit `getdetails` for a known item, confirm `tunnelTimestamp`/`lastWTSSeen` actually advance during a live tunnel window, measure the rebuild cadence, and confirm poll-and-diff reliably detects a new auction. Write the spike result into the phase CONTEXT. If coverage is too thin, the documented fallback is **"new sighting within window" alerting on `lastWTSSeen` + a price snapshot** rather than per-message auction text — still useful, lower fidelity.

**Warning signs:**
A plan that says "subscribe to PigParse auctions" or "real-time auction feed" (there is no subscription/stream — it is poll-only). Latency expectations under ~10 min. No last-seen cursor in the design.

**Phase to address:**
First wantlist-infra phase (the EC-monitor phase) — gated behind an explicit PigParse spike task as its first plan. **De-risk EARLY** (this and Pitfall 2 are the two "spike before you commit" items).

---

### Pitfall 2: MESSAGE_CONTENT privileged intent treated as automatic (verification-gate déjà vu)

**What goes wrong:**
Reading WTS/raid-target message *text* from the 3 Raid Alliance trade channels requires Discord's **MESSAGE_CONTENT** privileged intent. If the team assumes message content "just works," the WTS-monitor and raid-target-monitor read empty `content` fields and silently match nothing — a re-run of the exact class of failure that killed v1.0.2 (Google brand-verification wall, see PROJECT.md Key Decision: *"Discord OAuth2 has no brand-verification gate"* — that protection covered *login*, NOT a bot's gateway intents).

**Resolution from this research (the news is good — but only because of scale):**
- MESSAGE_CONTENT **must be toggled on in the Discord Developer Portal** for the bot regardless of size. Forgetting the toggle = empty content, no error.
- **Discord's verification/approval review for privileged intents only applies to bots in 75+ servers** (verification is required at 100+). **This bot will live in ~4 servers** (the guild's + 3 Raid Alliance). It is therefore **under the review threshold** — the toggle alone suffices, no Discord approval audit. *This is the opposite of the Google trap and should be stated as such so nobody over-worries — but the toggle is still a hard gate.*
- The intent grants `content`, `embeds`, `attachments`, `components`. Without it those fields arrive empty on messages the bot didn't author and isn't @mentioned in.
- **Keep the bot small on purpose.** Inviting it to more servers later (implausible here) would cross into the review regime. Document the "stay small" constraint.

**Why it happens:**
The Gateway connection succeeds and events flow; only the `content` field is blank, so it looks like a regex/matching bug rather than a missing capability — burns hours chasing the wrong layer.

**How to avoid:**
A 30-minute check in the bot-infra phase: enable MESSAGE_CONTENT in the dev portal, request the intent bit in the IDENTIFY, and assert in a smoke test that a real message's `content` is non-empty. Add a startup log line `intents=...` so a misconfigured deploy is greppable.

**Warning signs:**
Bot connects and sees message events but `content == ""` for every message. Gateway closes with code 4014 (disallowed intents — intent requested but not enabled in portal).

**Phase to address:**
Bot-infra phase (the gateway-connection phase), as an explicit checklist item — verified by a content-non-empty smoke test. **De-risk EARLY** alongside Pitfall 1.

---

### Pitfall 3: Un-DMable users — the bot cannot DM someone it shares no server with

**What goes wrong:**
A Discord bot can only DM a user who (a) shares at least one server with the bot AND (b) has not disabled "DMs from server members" or blocked the bot. The WTS source servers are the **3 Raid Alliance servers**, but the wantlist owners are **guildies** — some of whom may not be in those Raid Alliance servers, and the bot's "home" is the guild's own server. If the bot tries to DM a guildie it shares no mutual server with, or who has DMs off, the send fails with **error 50007 "Cannot send messages to this user."** Silently, unless handled.

**Why it happens:**
The mental model "I have their discord_user_id (captured at login), so I can DM them" is wrong. Discord requires a mutual-server + DM-privacy precondition that ID possession doesn't satisfy. 50007 is also overloaded (DMs-off vs not-mutual vs blocked) — the API won't tell you which.

**How to avoid:**
- **The bot MUST be in the guild's own Discord server** (which every wantlister is in via the v2.0 membership-gated login) so a mutual server always exists for delivery — even though the *content source* is the Raid Alliance servers. Separating "where it reads" from "where it delivers" is the key design move.
- Treat 50007 as a **first-class, expected outcome**, not an exception: catch it, mark the user `dm_unreachable` with a timestamp, and **surface the alert on the website** (a notifications inbox / "you missed N alerts" banner) as the fallback channel. Never silently drop.
- During onboarding to the wantlist feature, show a one-time "enable DMs from this server / send the bot a `/ping` so it can DM you" instruction, and a live "DM test" button that reports reachable/unreachable.
- Make alerting **opt-in** (you only get DMs for items you explicitly wantlist) — consent is implicit in adding an item, which matches Discord's anti-spam expectations.

**Warning signs:**
50007 in logs with no corresponding user-visible fallback. Wantlist owners who never appear in any server the bot is in. "I never got pinged" reports with successful match logs.

**Phase to address:**
Bot-infra / DM-delivery phase — encode the mutual-server precondition + 50007 handling + website-fallback inbox as success criteria.

---

### Pitfall 4: Invite-gated code built before the invites exist (stranded code)

**What goes wrong:**
The WTS monitor and quest-target raid monitor both require the bot to be **invited with read permission into 3 external Raid Alliance servers** — invites that PROJECT.md states are **un-negotiated**. If those phases are planned and built first, the code cannot be exercised end-to-end, can't be verified live, and sits stranded (recall the v2.1 retro: *"verify-or-close phases verify against LIVE state, not carried-forward notes"* — building against a permission that doesn't exist is the same anti-pattern, pre-emptively).

**Why it happens:**
The features are exciting and feel like the headline of the milestone, so they get pulled forward ahead of the boring infra and the un-owned external dependency.

**How to avoid:**
**Sequence the milestone so the invite-independent work ships first** (PROJECT.md already states this intent — enforce it in the roadmap):
1. Wantlist CRUD (fully unblocked, no Discord bot needed for the data model).
2. Bot-infra + DM delivery + the EC-tunnel monitor (needs only the bot in the *guild's own* server + PigParse — both already in hand).
3. **HARD GATE:** WTS + raid-target monitors are a *separate, later phase that does not start until the 3 invites are confirmed in writing.* Make "invites granted (admin/read perm in all 3 servers)" an explicit phase entry-precondition in the roadmap, not an in-phase task.
4. Build the WTS/raid matcher logic against **recorded fixture messages** (sample WTS lines) so the matching engine is testable without the live servers — but do not call the phase "done" until it runs against ≥1 real invited server.

**Warning signs:**
A roadmap that interleaves WTS-monitor plans before the invite is secured. Phase plans that mock the Raid Alliance gateway connection and never note "blocked on invite." Anyone treating the invite as a task the team controls (it's an external negotiation).

**Phase to address:**
Roadmap sequencing decision (milestone-level) + the WTS/raid-monitor phase's entry-precondition.

---

### Pitfall 5: Alert spam / dedup / cooldown failure (DMing the same auction repeatedly)

**What goes wrong:**
Because the EC monitor is **poll-and-diff against PigParse** (rebuilt every 10 min, no delta cursor) and the WTS monitor is a **passive match loop**, the naïve implementation re-emits the same auction on every poll/every repeated WTS line. A guildie who wantlists a hot item gets DMed every 10 minutes forever. In a 12-person guild, a few false-spam incidents will get the bot muted/blocked — which then trips Pitfall 3 (50007) permanently.

**Why it happens:**
"Match → DM" is the obvious loop; the dedup state (what have I already told this user about?) is an afterthought, and the poll-diff cursor is easy to get wrong (matching on item presence rather than on a new event identity).

**How to avoid:**
- **Dedup key = a stable event identity**, not "item is present." For EC: hash `(player, tunnelTimestamp, item, price)` (TunnelQuestBot uses exactly this — hash the parsed auction message as the dedup key). For WTS: hash the message ID. Persist seen-hashes in SQLite with a TTL; never re-DM a seen hash.
- **Per-user-per-item cooldown** on top of dedup (e.g., "at most one DM per item per user per N hours") so a genuinely re-auctioned item doesn't carpet-bomb.
- **Cursor correctness:** advance the last-seen `tunnelTimestamp`/`lastWTSSeen` only after a successful match+notify pass; on restart, don't replay the backlog (clamp to "events newer than process-start" on cold boot, or persist the cursor).
- Batch/coalesce: if 5 wanted items all hit in one poll, send one digest DM, not five.

**Warning signs:**
The same item DMed on consecutive polls. Restart causes a flood of historical alerts. DM volume that scales with poll frequency rather than with real auction activity. Users muting the bot.

**Phase to address:**
EC-monitor phase (poll-diff cursor + dedup) and WTS-monitor phase (message-ID dedup) — shared dedup/cooldown layer designed once in the bot-infra phase.

---

### Pitfall 6: Regex false-positives + item name-variant mismatches on free-text WTS

**What goes wrong:**
WTS channel messages are unstructured human chat ("WTS Fungi Tunic 30k, also have fungus covered scale tunic"). Naïve substring matching produces false positives ("Sapphire" matches "Black Sapphire Necklace") and false negatives (abbreviations: "FBSS", "Fungi", "BSS"; misspellings; plurals). A guildie either gets junk DMs or misses the item they actually wanted — both erode trust fast.

**Why it happens:**
Item identity in this system is the **stable EQ item ID** (per CLAUDE.md: *"item Name strings can drift but IDs are stable"*), but WTS chat has only free text — there is no ID in a chat line, so the matcher must bridge free text → canonical item, which is genuinely hard.

**How to avoid:**
- Adopt TunnelQuestBot's proven rule: **known items match only on exact item name (no substrings)** — "Black Sapphire" does NOT trigger on "Black Sapphire Necklace"; only unknown/freeform watches use substring. This kills the dominant false-positive class.
- Build the matcher off the existing **item-ID catalog** the system already maintains (wiki + PigParse), plus a **curated abbreviation/alias table** for the handful of famous items (FBSS, Fungi, etc.). Don't fuzzy-match the whole catalog — it generates noise.
- Use an efficient multi-pattern matcher (Aho-Corasick over the alias set — what TunnelQuestBot reaches for) so adding wantlist items doesn't blow up match cost.
- **Show the matched text in the DM** ("matched 'Fungi' → Fungi Covered Scale Tunic") so a false positive is self-evident and the user can refine — make misfires legible, not silent.

**Warning signs:**
DMs for items that merely contain the watched word as a substring. Famous items never matching because of abbreviation. Match logic that fuzzy-scores against the entire item catalog.

**Phase to address:**
WTS-monitor phase (the matcher), but the **alias/normalization table** should be seeded in the wantlist-CRUD phase (so users pick canonical items, reducing free-text ambiguity at the source).

---

### Pitfall 7: Gateway lifecycle in a long-running binary — bot crash takes down the HTTP server

**What goes wrong:**
The Discord gateway is a **single persistent WebSocket** requiring heartbeats, RESUME-vs-reIDENTIFY logic, and reconnect/backoff. If the bot runs as a goroutine inside the existing `squirebot-server` process and a panic in the bot's event loop is unrecovered, **it crashes the whole binary — taking down the live website + ingest API + enrichment scheduler** (the Core Value path). Conversely, sloppy reconnects can burn the IDENTIFY/session-start budget.

**Why it happens:**
The existing server is a single static Go binary (PROJECT.md), and "just add a goroutine" is the path of least resistance. Gateway connections fail constantly in normal operation (network blips, Discord-initiated reconnects, op-7 RECONNECT, op-9 INVALID SESSION) — code that doesn't treat disconnect as the *expected* steady state will crash or thrash.

**How to avoid:**
- **Isolate the bot goroutine with `recover()`** at its top frame; a panic must log + restart the bot loop, never propagate to the HTTP server. Treat the bot as a supervised subsystem with its own restart/backoff.
- Use a maintained gateway library (e.g., `disgo` / `discordgo`) that already implements heartbeat/ACK, RESUME (session_id + last seq), op-7/op-9 handling, and exponential reconnect backoff — do not hand-roll the WSS protocol.
- **Prefer RESUME over re-IDENTIFY** on transient drops (resume replays missed events and doesn't spend a session start). Only full-IDENTIFY on INVALID SESSION(false). Respect the ~1000 session-starts/day IDENTIFY budget; jittered exponential backoff so a Discord outage doesn't become a tight reconnect loop.
- **Health-gate:** expose the bot's connection state in `/healthz` so a silently-dead gateway (heartbeat ACK missing) is observable, and so the website's liveness isn't conflated with the bot's.
- Decide explicitly: in-process supervised goroutine (simplest, one deploy, one VPS — fine at this scale) vs separate process. For ~12 users on one small Hetzner box, in-process-with-recover is the right call — but the recover boundary is non-negotiable.

**Warning signs:**
A bot panic in logs immediately before the website goes down. Reconnect storms (many IDENTIFYs/min). `disallowed intents` (4014) or `authentication failed` (4004) loops. Heartbeat-ACK gaps with no reconnect. The whole binary restarting (systemd `Restart=always` masking a bot crash as a server crash).

**Phase to address:**
Bot-infra / gateway-connection phase — recover boundary, supervised restart, and health-gate are explicit success criteria.

---

### Pitfall 8: Notification fatigue + over-engineering for scale that doesn't exist

**What goes wrong:**
Two opposite failure modes for a **12-person guild**: (a) the alerting is so chatty that everyone mutes the bot within a week (then 50007 forever — Pitfall 3); or (b) the team builds a sharded, queue-backed, multi-tenant notification platform with delivery-retry exponential schedules and per-user notification-preference matrices for ~12 users and a few alerts a day. Both waste the milestone.

**Why it happens:**
Discord-bot tutorials and the "alerting subsystem" framing pull toward enterprise patterns (message queues, sharding, rate-limit token buckets sized for thousands). The actual load is **a handful of guildies, a few wanted items each, ~10-min poll cadence, weekend-bursty tunnel** — trivially within one goroutine and one SQLite table.

**How to avoid:**
- **Right-size to the guild:** no sharding (one shard handles far more than 4 servers), no external queue (SQLite table + a ticker is the queue), no elaborate rate-limit machinery beyond respecting `X-RateLimit-*` headers and the 50 req/sec global ceiling (you will never approach it — a few DMs/day).
- **Default-quiet alerting:** opt-in per item, sane cooldowns (Pitfall 5), and a **digest option** ("daily summary of matches") for users who don't want real-time DMs. Give users an easy **mute/unsubscribe** that doesn't require blocking the bot.
- Apply the project's own muscle memory: PROJECT.md repeatedly chose SQLite/in-process over Postgres/queues at this scale and *"no regret at v2.0 close."* Carry that judgment forward; the burden of proof is on anything heavier than a table + a ticker + a recover boundary.

**Warning signs:**
Roadmap items mentioning sharding, Redis/queue infrastructure, or per-user preference engines. Token-bucket rate limiters tuned for thousands of req/sec. Users muting the bot. Conversely: alert volume so high a 12-person guild complains within days.

**Phase to address:**
Milestone scoping + every alerting phase — keep the alerting layer one shared, simple module; add a digest/mute preference (cheap) rather than a preference engine.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Skip the PigParse auction-event spike, assume it works | Start the EC phase a day sooner | Stranded phase if poll-diff doesn't actually detect events; latency/coverage surprises in prod | **Never** — the spike is 2–4h and de-risks the whole feature |
| Run the gateway goroutine without a `recover()` boundary | Less code | A bot panic kills the live website + ingest (Core Value) | **Never** — recover boundary is non-negotiable |
| Match WTS by naïve substring | Trivial to write | False-positive DM spam → users block bot → permanent 50007 | Only for explicitly "unknown/freeform" watches, never for catalog items |
| Hand-roll the gateway WSS protocol | "No dependency" | Reimplements heartbeat/RESUME/backoff — high bug surface | Never — use `disgo`/`discordgo` |
| DM-only delivery, no website fallback inbox | Simpler | Un-DMable users (50007) silently miss every alert | MVP-only, if a fallback inbox is the very next phase |
| No dedup cursor (re-scan and re-DM) | Fastest to ship a "working" demo | Carpet-bomb DMs on every poll → bot muted | Never past the demo |
| Build WTS/raid monitors before invites land | Feels like progress on the headline feature | Stranded, unverifiable code | Only the *matcher* against fixtures, never the live wiring |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| Discord DM send | Assuming `discord_user_id` (captured at login) is sufficient to DM | Bot must share a server (the guild's own) with the user AND the user must allow DMs; handle 50007 as expected; website fallback |
| Discord MESSAGE_CONTENT | Assuming message `content` is readable by default | Toggle the intent in the dev portal + request the bit; under 75 servers no review needed; smoke-test content is non-empty |
| Discord gateway | Treating disconnects as errors / re-IDENTIFY every reconnect | Disconnects are the steady state; RESUME on transient drops; respect session-start budget; jittered backoff |
| Discord rate limits | Hard-coding limits / ignoring 429 scope | Honor `X-RateLimit-*` headers + 50 req/sec global; you'll never approach it at this scale, but parse the headers |
| PigParse auctions | Expecting a real-time stream or "since T" delta endpoint | Poll `get`/`getdetails`, diff on `tunnelTimestamp`/`lastWTSSeen`; data rebuilds ~every 10 min; coverage is best-effort |
| PigParse load | Hammering `getdetails` (full history, "ALOT of data") on a tight loop | Prefer `get` for averages; only `getdetails` when needed; reuse the existing `politeFetch` throttle/backoff; daily-budget the heavy calls |
| Raid Alliance servers | Building monitors before write-confirmed invites | Gate the phase on invites-in-hand as an entry precondition; build matcher against fixtures meanwhile |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Poll-and-DM without dedup | Same auction DMed every poll | Persist seen-event hashes + per-user-per-item cooldown | Immediately, on any hot item |
| Replaying backlog on restart | Flood of old alerts after every deploy/reboot | Persist the last-seen cursor; clamp cold-boot to "newer than process start" | Every restart (systemd `Restart=always` makes restarts routine) |
| `getdetails` (full history) on every wanted item every poll | PigParse latency, possible throttling, slow loop | Use `get` (averages/last-seen) for the poll; `getdetails` only on a match | When wantlist or poll frequency grows (still small here, but cheap to avoid) |
| Reconnect storm on Discord blip | Many IDENTIFYs, session-start budget burn | Jittered exponential backoff + RESUME | During any Discord outage |
| Over-built queue/sharding | Wasted complexity, more failure surface | One SQLite table + a ticker; one shard | This scale never needs it — the trap is building it anyway |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Bot token in source/env-in-repo/logs | Token leak → anyone controls the bot in all invited servers | Store the bot token like the existing bearer secrets (env/secret on the VPS, never committed, never logged); rotate if exposed |
| Logging full message content | Privacy leak of other guilds' trade chat; log bloat | Log match metadata (item, channel, hash), not raw message bodies |
| Trusting Discord message author / IDs blindly for write actions | Spoofed commands triggering wantlist changes | Wantlist mutations stay on the website behind the existing Discord-OAuth session; the bot is read+DM only, not a command surface for state changes |
| Bot over-permissioned in Raid Alliance servers | Reputational/trust risk; admins revoke invite | Request **minimum** perms (view channel + read message history on the specific trade channels), never admin; document the minimal scope when negotiating invites |
| MESSAGE_CONTENT scope creep | Reading channels beyond the agreed trade channels | Restrict processing to the explicitly-agreed channel IDs even though the intent is server-wide |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Silent DM failure (50007) | User thinks alerting is broken, loses trust | Website fallback inbox + a "DM test / you're unreachable" indicator on the wantlist page |
| No way to mute without blocking the bot | User blocks bot → permanently un-DMable for ALL alerts | In-app per-item unsubscribe + global mute + digest mode |
| False-positive substring matches | Junk DMs, user disables the feature | Exact-match for catalog items; show matched text so misfires are legible |
| Promising real-time auction alerts | Disappointment when alerts lag ~10+ min | Set expectations: "near-real-time, depends on tunnel coverage" |
| Requiring users to understand intents/invites | Confusion, abandonment | Onboarding hides it: "add items, enable DMs, done"; complexity lives in admin/setup docs |

## "Looks Done But Isn't" Checklist

- [ ] **MESSAGE_CONTENT:** Often missing the dev-portal toggle — verify a real message's `content` is non-empty in a smoke test (not just that events arrive).
- [ ] **DM delivery:** Often missing the 50007 path — verify a DM to a user who shares no server / has DMs off is caught and routed to the website fallback, not silently dropped.
- [ ] **Bot crash isolation:** Often missing the recover boundary — verify a forced panic in the bot loop restarts the bot WITHOUT taking down `/healthz` or the website.
- [ ] **Dedup:** Often missing restart behavior — verify a restart does NOT replay historical alerts; verify the same auction isn't re-DMed on the next poll.
- [ ] **PigParse cursor:** Often missing the advance-only logic — verify the last-seen cursor advances only after a successful notify pass.
- [ ] **Invites:** Often "done" against mocks — verify the WTS/raid monitor ran against ≥1 real invited Raid Alliance server, not just fixtures.
- [ ] **Reconnect:** Often missing the RESUME path — verify the bot survives a forced gateway disconnect via RESUME (not a re-IDENTIFY storm).
- [ ] **Item matching:** Often missing abbreviations — verify FBSS/Fungi-class aliases match AND that "Sapphire" does not match "Black Sapphire Necklace."

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| PigParse has no usable auction events | HIGH | Fall back to `lastWTSSeen`-sighting + price-snapshot alerts; re-scope WANT-04 to "sighting alert," not "auction-message alert" |
| MESSAGE_CONTENT not enabled | LOW | Toggle in dev portal, redeploy; no code change |
| Bot crash took down website | MEDIUM | systemd restarts it (already configured), but add the recover boundary so it never recurs; backfill missed alerts from the cursor |
| DM spam already annoyed the guild | MEDIUM | Add dedup+cooldown, apologize in the guild channel, re-enable opt-in; some users may have muted (then 50007 — surface via website) |
| Built WTS monitor before invites | MEDIUM | Code isn't wasted IF the matcher was built against fixtures; just wire the gateway once invites land |
| Bot token leaked | MEDIUM | Reset token in dev portal immediately, redeploy, audit for unauthorized actions |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| 1. PigParse auction data assumed | EC-monitor phase — gated on a **spike task first** | Spike doc proves `tunnelTimestamp`/`lastWTSSeen` advance live; poll-diff detects a real auction |
| 2. MESSAGE_CONTENT intent gate | Bot-infra phase | Smoke test: real message `content != ""`; startup logs `intents` bitmask |
| 3. Un-DMable users (50007) | DM-delivery phase | Forced 50007 routes to website fallback inbox; bot is in the guild's own server |
| 4. Invite-gated stranded code | Roadmap sequencing + WTS/raid phase entry-precondition | WTS/raid phase does not start until invites confirmed; matcher unit-tested on fixtures meanwhile |
| 5. Alert spam / dedup | Shared dedup layer (bot-infra) + EC & WTS phases | Restart doesn't replay; same event never re-DMed; cooldown enforced |
| 6. Regex/name-variant mismatch | WTS-monitor phase (matcher); alias table seeded in wantlist-CRUD phase | Alias matches FBSS/Fungi; exact-match prevents substring false positives |
| 7. Gateway crash / lifecycle | Bot-infra / gateway phase | Forced panic restarts bot, not the server; survives forced disconnect via RESUME; `/healthz` shows bot state |
| 8. Fatigue / over-engineering | Milestone scoping + all alerting phases | No queue/shard infra; opt-in + cooldown + digest/mute present; alerting is one simple module |

## Sources

- Discord — Message Content Privileged Intent FAQ & Review Policy (verification/review applies at 75+/100+ servers; portal toggle required regardless): https://support-dev.discord.com/hc/en-us/articles/4404772028055 and https://support-dev.discord.com/hc/en-us/articles/5324827539479 — HIGH
- Discord — error 50007 "Cannot send messages to this user" (mutual-server + DM-privacy precondition; overloaded code): https://github.com/discord/discord-api-docs/issues/8238 — HIGH
- Discord — Rate Limits (50 req/sec global; honor `X-RateLimit-*`, don't hard-code): https://docs.discord.com/developers/topics/rate-limits — HIGH
- Discord — Gateway (heartbeat/ACK, RESUME vs IDENTIFY, op-7/op-9, session-start budget, backoff): https://docs.discord.com/developers/topics/gateway — HIGH
- PigParse live OpenAPI schema `/swagger/v1/swagger.json` — confirms `Auction{player,tunnelTimestamp,items[]}`, `ItemAuctionDetail{u,i,p,t}`, `Item.lastWTSSeen/lastWTBSeen`, `get`/`getdetails`/`getall` endpoints, "rebuild every 10 minutes," "getdetails transfers ALOT of data" — HIGH (fetched directly 2026-06-02)
- TunnelQuestBot (jamesjamail/TunnelQuestBot) — proven P99 EC-auction→Discord-watch patterns: exact-match for known items, substring for unknown, hash-the-message dedup, Aho-Corasick matching; parses player log files (not PigParse) — confirms the auction-data-source risk and the matching/dedup design: https://github.com/jamesjamail/TunnelQuestBot — MEDIUM
- SquireBot PROJECT.md / CLAUDE.md — v2.2 scope, the v1.0.2 Google brand-verification precedent, SQLite/in-process scale judgments, item-ID-stable matching, the "verify against LIVE state" v2.1 retro — HIGH (project canon)

---
*Pitfalls research for: adding a Discord bot + alerting subsystem to an existing Go/SQLite guild tool (P1999, ~12 users)*
*Researched: 2026-06-02*
