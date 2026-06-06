# Phase 21: EC-Tunnel Auction Monitor - Research

**Researched:** 2026-06-05
**Domain:** Integration work on a LIVE Go + SQLite + SvelteKit backend — adding one in-process scheduler job (`ec_auction_match`) that polls PigParse per wanted item, diffs on an auction-timestamp cursor, exact-item-ID matches against guildie wantlists, and DMs via the deployed Phase 20 spine.
**Confidence:** HIGH (every codebase claim cited to path:line; PigParse Swagger re-verified live 2026-06-05; discordgo embed API verified from the module cache)

## Summary

The Phase 20 spine is **built and deployed** (`notify`, `wantmatch`, `alert_log`, `bot`, plus the two-gate model and per-source cooldown constants). Phase 21 is a **thin producer**: a new scheduler job derives a poll set from `wantlist_item` rows, polls PigParse per item, diffs new auctions against a per-item cursor, and for each new WTS auction calls `wantmatch.ForItem(itemID)` → `notify.Send(...)`. It re-implements nothing in the spine. The vast majority of the work is (a) the PigParse per-item auction fetch+parse (a NEW parser — the existing `enrich.ParseToRows` parses the `getall` *aggregate* feed, NOT the per-item `getdetails` auction feed), (b) the cursor table + advance-only-on-success diff logic, (c) the rich-embed DM body, and (d) the mandatory upfront feasibility spike that picks the trigger mechanism.

**Two load-bearing facts the planner must internalize.** First: PigParse has **no global "auctions since T" feed** — re-verified against the live Swagger 2026-06-05. The only per-auction timestamped data is `GET /api/item/getdetails/{server}/{itemname}` (or `/getdetails/{itemid}`), returning `ItemDetail{ items: ItemAuctionDetail[], itemName, players }` where `ItemAuctionDetail = { u (0=WTS,1=WTB,2=BOTH), i (itemid), p (price, nullable), t (timestamp) }`. The monitor MUST be a per-item poll-and-diff differ, never a subscriber. Second: the existing `notify.Send` sends a **plain string `Body`** — it has no embed path. D-04 (rich embed) therefore requires a NEW send path (the planner must decide: extend `notify` with an embed-capable send, or render the embed in the EC job and call a new `notify` function). Recommendation below: add `notify.SendEmbed` mirroring `notify.Send`'s gates/dedup/alert_log discipline, since gates+dedup+`alert_log` write must NOT be duplicated.

**Primary recommendation:** Plan Task 1 = the PigParse feasibility spike (gate, ROADMAP criterion 1). Then: migration `00008_ec_cursor.sql` (cursor table) → a NEW `enrich` PigParse-details parser + a NEW `wantmatch`/`notify` glue job in `scheduler` → a NEW `notify.SendEmbed` path → register `ec_auction_match` in `scheduler.Start`. D-06 item link has **no per-item frontend route** (confirmed) — fall back to the P1999 wiki URL (the existing `wikiUrlFor` idiom) unless the planner sizes a new `/item/[id]` SvelteKit route.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Match scope (WANT-05):**
- **D-01:** EC fires for **any catalog want regardless of reason** — `reason IN ('buy','quest')`, as long as the row has a real `item_id`. The buy-vs-quest split does NOT gate the EC monitor.
- **D-02:** Alert **only on WTS** (`u`=0 or `u`=2/BOTH includes WTS). WTB-only sightings (`u`=1) **never** alert. The DM carries a direction tag for clarity but always reads WTS.
- **D-03:** Custom wants (`item_id IS NULL`) **cannot** be exact-ID-matched, so the EC job **silently skips** them — no user-facing warning. They are `wantmatch.ForName`'s job (P22). EC only matches catalog wants (`item_id` present).

**DM content & format (WANT-05):**
- **D-04:** The EC alert is a **discordgo rich embed**, not a plain string.
- **D-05:** Fields = **item name + price + WTS tag** (essentials), plus when available: **seller name** (best-effort via the `players` map; omit silently when unresolvable), **why-you-wanted-it** (echo reason and/or note), **a clickable item link**, **auction time** ("~3 min ago").
- **D-06:** Link target = **the project's own frontend item/tooltip view** — NOT the wiki. ⚠ **Researcher/planner MUST verify a stable per-item URL exists**; if none, treat building one (or wiki fallback) as a small in-phase dependency. **[RESEARCH FINDING: no per-item route exists — see Open Questions. Fall back to the wiki idiom or build a route.]**

**Spike outcome & risk posture:**
- **D-07:** Thin coverage → **ship anyway, document the gap**. Do NOT defer the phase on thin coverage.
- **D-08:** Go/no-go **delegated to a threshold — no checkpoint**. Claude runs the spike, applies a stated coverage rule (default: use per-auction `getdetails` if per-auction timestamps are present and advancing; else fall back to coarser `lastWTSSeen` new-sighting), proceeds to plan without stopping. Spike findings + path taken MUST be documented.
- **D-09:** **No courtesy-contact** the PigParse operator. Stay courteous in code: conditional requests / backoff, sane cadence, and **only poll items that are actually on someone's wantlist**.

**Cooldown & re-list:**
- **D-10:** Cooldown = **per-source tunable constant** — Claude picks a sane placeholder (~20–24h) exposed as a constant adjustable in soak. Mechanism locked: suppress repeat DMs for the same `(wantlist_item, source, item_id)` within the window.
- **D-11:** A genuinely NEW later auction past the cooldown window **re-DMs**. Cooldown is **purely time-based**; price is NOT part of the dedup key; a cheaper re-list does NOT break the cooldown early (price logic stays deferred — REQUIREMENTS.md:47).

**Locked upstream (DO NOT re-decide):**
- Poll-and-diff on an auction-timestamp cursor (`ec_auction_cursor`), exact item-ID match (`wantmatch.ForItem`), ~10-min cadence. **Advance the cursor only after a successful poll; never replay backlog on restart.**
- In-process scheduler job in the existing registry; reuses `enrich`/`politefetch`, no new HTTP client, no cron daemon.
- Dedup + cooldown via `alert_log` (the P20 spine); every attempt recorded.
- Both gates from P20 apply: officer monitor flag (`ec_auction`) AND user opt-in/prefs (per-monitor + per-want mute) must both allow.
- No Discord bot/OAuth in the watcher (HARD CONSTRAINT) — all EC work is backend + the guild's own Discord.

### Claude's Discretion
- The exact **cooldown value** (D-10) and the **spike coverage threshold** numbers (D-08).
- Exact **embed layout/wording**, the WTS tag copy, and the "why-you-wanted-it" phrasing (D-04/D-05).
- **Seller-resolution effort** — best-effort via the `players` map; how hard to try is Claude's call (omit silently when unresolvable).
- The **`ec_auction_cursor` table/column shape** and the precise PigParse endpoint(s) (`getdetails`/`getmultiple`/`lastWTSSeen`) — pinned by the spike.
- Surfacing **EC job state on `/healthz`** and the job's structured-log fields (observability).

### Deferred Ideas (OUT OF SCOPE)
- **Price-threshold / "only DM if under X pp"** — deferred (REQUIREMENTS.md:47); D-11 keeps EC cooldown purely time-based.
- **Custom (free-text) want matching on EC** — D-03 skips them; `wantmatch.ForName` (P22).
- **`lastWTSSeen` coarse fallback as a permanent mode** — only if the spike shows per-auction coverage too thin (D-08); not a default.
- **"Retry delivery" / digest / quiet-hours** — P20-deferred polish, unaffected here.
- **WTB-direction alerts** — rejected as noise (D-02).
- **WTS cross-server (P22) and quest-target raid (P23) monitors** — out of scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| WANT-05 | When a wanted item is auctioned in the EC tunnel, the wantlister gets a DM (price + WTS tag; seller best-effort) — all on the guild's own Discord. | Poll set = `wantmatch`-eligible `wantlist_item` rows (`active=1`, `item_id NOT NULL`); per-item poll via PigParse `getdetails` ([VERIFIED: live Swagger 2026-06-05]); diff on a NEW `ec_auction_cursor` table; `wantmatch.ForItem` ([VERIFIED: `wantmatch/match.go:51`]) → `notify.Send`/`SendEmbed` ([VERIFIED: `notify/dm.go:112`]); cooldown constant `cooldownEC = 22h` already present ([VERIFIED: `notify/dm.go:87`]); both gates enforced in `notify.Send` ([VERIFIED: `notify/dm.go:115-146`]). The spike (ROADMAP criterion 1) confirms feasibility before commit. |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Poll PigParse per wanted item | API/Backend (`scheduler` job + `enrich`/`politefetch`) | — | Outbound HTTP + parse + diff is backend-only; reuses the existing polite client. No frontend/watcher involvement (HARD CONSTRAINT). |
| Derive the poll set from wantlists | API/Backend (`store` query over `wantlist_item`) | — | The poll set is live DB state; a new owner-agnostic read (`DISTINCT item_id WHERE active=1 AND item_id NOT NULL`). |
| Diff new auctions vs. last-seen cursor | API/Backend (`ec_auction_cursor` table + job logic) | — | Restart-safety + dedup live in the cursor; mirrors `job_run`'s advance-only pattern. |
| Match auction → wantlisters | API/Backend (`wantmatch.ForItem`) | — | The single shared match seam, already built and tested (P20). EC reuses it verbatim. |
| Gate + dedup + send DM + log | API/Backend (`notify.Send`/`SendEmbed`) | — | The two gates, per-source cooldown, 50007 handling, and `alert_log` write are all in `notify`. EC must NOT duplicate them. |
| Render the rich embed | API/Backend (EC job builds `*discordgo.MessageEmbed`) | — | D-04 embed is composed backend-side and sent via the shared `*discordgo.Session`. |
| Item deep-link target | Frontend (SvelteKit) — **does not yet exist** | wiki fallback | D-06 wants the project's own item view; no per-item route exists (Open Question). |
| Officer enable + per-user prefs | API/Backend (`monitor_flag.ec_auction`, `notify_prefs.ec`) | Frontend (`/admin`, `/notifications` — already shipped P20) | Both gates already wired in P20; EC inherits them with zero new UI. |

## Standard Stack

### Core (ALL already in the binary — no new dependencies)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/bwmarrin/discordgo` | v0.29.0 | DM send (`UserChannelCreate` + `ChannelMessageSend`/`ChannelMessageSendEmbed`) | Already in `go.mod` ([VERIFIED: `go.mod` line `github.com/bwmarrin/discordgo v0.29.0`]); the shared `*Session` is owned by `bot` and injected into `notify`. |
| `modernc.org/sqlite` | (existing) | `ec_auction_cursor` table via goose | Pure-Go, CGO-free, the locked DB. No new dependency. |
| stdlib `net/http` via `politefetch` | (existing) | Per-item PigParse polling | `internal/backendsrv/enrich/politefetch` already does UA + ETag/304 + backoff + Retry-After ([VERIFIED: `politefetch/politefetch.go:43-291`]). |
| stdlib `time.Ticker` via `scheduler` | (existing) | The ~10-min `ec_auction_match` job | `scheduler.checkInterval = 10*time.Minute` already ([VERIFIED: `scheduler/scheduler.go:59`]); register one more `*Job`. |

**No new Go dependency is required for Phase 21.** `go.mod` already carries discordgo (added in P20).

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Per-item `getdetails` poll | `getmultiple/{server}` (aggregate, bulk) | `getmultiple` returns `Item` aggregates with `lastWTSSeen` (a coarser last-sighting signal) but NOT per-auction `ItemAuctionDetail[]`. It is the D-08 fallback signal if per-auction `t` coverage is too thin — NOT the default. |
| `notify.SendEmbed` (new) | Render embed in EC job + reuse `notify.Send`'s string path | `Send` only takes a string `Body` ([VERIFIED: `notify/dm.go:62-69, 163`]). Embeds need a new send call; do NOT bypass `notify`'s gates/dedup/alert_log by sending the embed directly from the job. |

**Installation:** none. (`go.mod` unchanged; verify with `go list -m github.com/bwmarrin/discordgo` → `v0.29.0`.)

## PigParse Auction API Contract (the load-bearing data source)

> [VERIFIED: live `https://pigparse.azurewebsites.net/swagger/v1/swagger.json` fetched 2026-06-05] — matches `.planning/research/STACK-v2.2.md` §"What PigParse actually exposes" exactly.

**There is NO global "auctions since T" feed.** [VERIFIED: Swagger — "No endpoint exists that retrieves recent auctions across all items since a specified timestamp."] The monitor MUST poll-and-diff per item.

### Endpoints relevant to the EC monitor
| Path | Returns | Use in this phase |
|------|---------|-------------------|
| `GET /api/item/getdetails/{server}/{itemname}` | `ItemDetail` — full per-auction history for one item | **Primary** per-auction source (the spike's default path). `{server}=1` = Blue. |
| `GET /api/item/getdetails/{itemid}` | `ItemDetail` (keyed by id, no server segment) | Alternative key — but the wantlist stores both `item_id` and `item_name`. Prefer the **name** form for parity with the existing `getall`/wiki name idiom, OR the id form to avoid name-drift; the spike pins this. |
| `GET /api/item/getmultiple/{server}` (query `itemnames`) | bulk `Item` aggregates (incl. `lastWTSSeen`/`lastWTBSeen`) | **Fallback signal** (D-08) — coarser "new sighting" trigger if per-auction `t` coverage is thin. NOT per-auction. |
| `GET /api/item/getall/1` | `getall` aggregate feed (the daily price job) | NOT used by EC; this is the existing daily pull ([VERIFIED: `jobs/urls.go:28`]). |

### Auction record shapes [VERIFIED: Swagger 2026-06-05]
```
ItemDetail   = { items: ItemAuctionDetail[] (nullable), itemName: string (nullable), players: { <string>: string } (nullable) }
ItemAuctionDetail = { u: int32 (0=WTS, 1=WTB, 2=BOTH), i: int32 (itemid), p: int32|null (price), t: string (date-time) }
Item         = { ... , lastWTSSeen: string|null (date-time), lastWTBSeen: string|null (date-time), ... }
```

Critical design consequences (carry into the plan):
1. **Diff on `t`.** Each `ItemAuctionDetail.t` is a date-time string. The cursor stores the max `t` seen per item; on each poll, any `ItemAuctionDetail` with `t` > cursor AND `u ∈ {0, 2}` (WTS or BOTH — D-02) is a NEW WTS auction.
2. **`ItemAuctionDetail` has NO seller field.** [VERIFIED: Swagger] The seller (D-05 best-effort) appears ONLY in the `players` map. The map's key relationship to a specific auction record is NOT documented — the keys are stringified indices/handles, values are player names. **Seller resolution is genuinely best-effort and may be unreliable; omit silently when unresolvable (D-05).** Do NOT block the DM on it.
3. **`p` (price) is nullable.** Render "price unknown" or omit the price field when `p` is null — do not emit `0pp`.
4. **The existing `enrich.ParseToRows` does NOT parse this shape.** [VERIFIED: `enrich/pigparse.go:42-55`] — it parses the `getall` *aggregate* `PigparseRow` (`i/t/n/l/t30/a30/...`), where `t` is a DIRECTION flag (0/1/2), NOT a timestamp. The EC monitor needs a **NEW parser** for `ItemDetail`/`ItemAuctionDetail`. Do not reuse `ParseToRows`. (Note the `t` collision: in `getall`, `t` = direction; in `getdetails`, `t` = timestamp and `u` = direction. Easy to confuse — call it out in the parser doc.)
5. **Latency floor ≈ 10 min** (PigParse rebuilds aggregates every ~10 min); the ~10-min poll cadence is the honest design point. Document "near-real-time," never "instant."
6. **Coverage is best-effort/bursty** — the tunnel is only fed when a human is parked in EC running a parser (weekend-heavy). D-07: ship anyway, document the gap.

## The Mandatory Feasibility Spike (ROADMAP criterion 1 / Pitfall 1)

The spike is **the phase's FIRST plan task and gates the rest** (D-08, no checkpoint — Claude runs it, applies the threshold rule, proceeds).

### What the spike runs
1. Pick 2–4 known hot tunnel items (e.g. by name: a Fungi Covered Scale Tunic, a Flowing Black Silk Sash / "FBSS", a common spell or resist piece). Hit `GET /api/item/getdetails/1/{itemname}` via `politefetch.Fetch` (or a one-off `go run` spike binary / a `run-job`-style subcommand).
2. Record, per item: count of `ItemAuctionDetail` returned, the min/max/distribution of `t` timestamps, how many carry `u ∈ {0,2}` (WTS), how many carry a non-null `p`, and whether the `players` map resolves a seller for any record.
3. **Sample over a live tunnel window** (ideally a weekend / Sunday auction window per Pitfall 1) — poll the same items ~2–3 times ≥10 min apart and confirm `max(t)` **advances** between polls (proving new auctions land and the cursor will detect them).
4. Cross-check `getmultiple`/`Item.lastWTSSeen` for the same items as the fallback signal's coverage.

### The stated coverage threshold rule (D-08 — proposed concrete rule)
Adopt **per-auction `getdetails`** (the default path) iff BOTH hold across the spike sample:
- **(a) Presence:** ≥ 1 of the probed items returns ≥ 1 `ItemAuctionDetail` with a parseable `t` AND `u ∈ {0,2}`, AND
- **(b) Advancement:** `max(t)` for at least one probed item is observed to **advance** between two polls ≥ 10 min apart during a live tunnel window (proves new events are detectable, not a frozen snapshot).

Otherwise, **fall back to the coarser `lastWTSSeen` new-sighting trigger** (poll `getmultiple`, cursor on `Item.lastWTSSeen` advancing). Either way: **proceed to plan** (D-07/D-08 — never defer). Document in the plan/spike artifact which path was taken and the measured coverage numbers.

**Spike output artifact:** a short markdown note (e.g. `21-SPIKE.md` or inline in the first plan) recording: items probed, record counts, `t` distribution, advancement observed Y/N, seller-resolution hit-rate, path chosen, coverage caveat for D-07.

**Warning signs the plan got it wrong** (Pitfall 1/5): any wording like "subscribe to PigParse auctions" / "real-time feed"; latency promises < 10 min; no cursor in the design.

## Architecture Patterns

### System data flow
```
scheduler ticker (~10 min)  ──►  ec_auction_match job  (NEW *Job in scheduler.Start)
                                        │
        ┌───────────────────────────────┘
        ▼
  store: poll set ──► SELECT DISTINCT item_id, item_name
        │             FROM wantlist_item
        │             WHERE active=1 AND item_id IS NOT NULL   (D-01 reason NOT filtered; D-03 NULL skipped)
        ▼
  for each (item_id, item_name):
        politefetch.Fetch(getdetails URL)  ──► NEW enrich parser ──► ItemAuctionDetail[]
        │
        ▼
  read ec_auction_cursor[item_id]  ──► filter: t > cursor AND u ∈ {0,2} (WTS, D-02)
        │                                          │
        │   (no new WTS auctions) ── skip          ▼  (one or more new WTS auctions)
        │                              wantmatch.ForItem(db, item_id)  ──► []Hit  (active=1, muted=0, ALL users)
        │                                          │
        │                                          ▼  for each Hit:
        │                              build *discordgo.MessageEmbed (D-04/05)
        │                              notify.SendEmbed(ctx, session, db, alert, now)
        │                                   │  GATE 1 monitor_flag.ec_auction (officer)
        │                                   │  GATE 2 notify_prefs.ec (user, master-gated)
        │                                   │  DEDUP RecentAlertExists (cooldownEC=22h, D-10/11)
        │                                   │  SEND DM + InsertAlertTx (sent|dm_blocked|error)
        ▼                                   ▼
  ADVANCE ec_auction_cursor[item_id] = max(t)   ◄── ONLY after the item's poll succeeded
  (never on a fetch/parse failure — no-replay-on-restart, advance-only-on-success)
```

### Pattern 1: New scheduler job (copy `pigparse_daily`)
**What:** Add one `*Job{}` to the `registry` slice in `scheduler.Start`.
**When:** Always — this is the integration point.
**Cadence:** EC needs ~10-min cadence. The scheduler's `checkInterval` IS 10 min ([VERIFIED: `scheduler/scheduler.go:59`]) and the existing `Due` predicates are per-job. A `dueEC(last, now)` returning `now.Sub(last) >= 10*time.Minute` (or simply `true` to run every tick, since `checkInterval` already paces it) fits. **Note:** the existing per-job `job_run` cursor advances after each run regardless of outcome ([VERIFIED: `scheduler/scheduler.go:229-250`]). For EC, the job-level `job_run` advance is for cadence/observability; the **per-item** `ec_auction_cursor` is the diff state and must advance ONLY on a successful per-item poll (a distinct concern — do not conflate the two cursors).
```go
// Source: scheduler/scheduler.go:116-130 (pigparse_daily registry entry — copy this shape)
{
    Name: "ec_auction_match",
    Due:  func(last, now time.Time) bool { return now.Sub(last) >= 10*time.Minute },
    Run: func(ctx context.Context) error {
        return ec.RunMatch(ctx, db, botSession, politefetch.Fetch) // NEW package
    },
},
```
**Wiring caveat:** the current `scheduler.Start(ctx, db)` signature ([VERIFIED: `scheduler/scheduler.go:115`]) does NOT receive the `*discordgo.Session`. The EC job needs the session to send DMs. The planner must thread `botSession` into the scheduler — EITHER change `scheduler.Start` to `Start(ctx, db, botSession)` (and update the single caller `main.go:237`), OR have the EC job pull the session from a `*bot.Bot` handle. **Ordering note:** `main.go` calls `scheduler.Start(ctx, db)` at line 237 BEFORE `bot.Start` at line 244 ([VERIFIED: `cmd/squirebot-server/main.go:237-251`]). To pass the live session in, the planner must **reorder** (`bot.Start` before `scheduler.Start`) or inject lazily. Flag this as a concrete wiring task.

### Pattern 2: `notify.SendEmbed` — new send path, same discipline
**What:** `notify.Send` takes a string `Body` and calls `s.ChannelMessageSend(ch.ID, a.Body)` ([VERIFIED: `notify/dm.go:163`]). D-04 needs `s.ChannelMessageSendEmbed(ch.ID, embed)`. The `Sender` interface ([VERIFIED: `notify/dm.go:49-52`]) declares only `UserChannelCreate` + `ChannelMessageSend` — it must gain `ChannelMessageSendEmbed` (the real `*discordgo.Session` already satisfies it — [VERIFIED: `discordgo@v0.29.0/restapi.go:1812`]).
**Recommended shape:** add an `Embed *discordgo.MessageEmbed` field to `Alert` (or a parallel `SendEmbed` func) so the SAME two-gate + dedup + `alert_log` write path runs ([VERIFIED: `notify/dm.go:108-179` is the reusable core — gates at :115-146, dedup at :137-146, record at :174-197]). DO NOT write a second gate/dedup implementation in the EC job. The `Detail` field (`*string`) is the natural place to stash a short alert detail (price/seller) for the inbox row.
**discordgo embed API** [VERIFIED: `discordgo@v0.29.0/message.go:429-443, 422-426, 378-382`]:
```go
embed := &discordgo.MessageEmbed{
    Title:       "Fungi Covered Scale Tunic — WTS",   // D-05 item + WTS tag
    URL:         itemLink,                              // D-06 (wiki fallback or new route)
    Color:       0x..., // theme accent
    Fields: []*discordgo.MessageEmbedField{
        {Name: "Price", Value: "~2000 pp", Inline: true},        // D-05 (omit if p==nil)
        {Name: "Seen",  Value: "~3 min ago", Inline: true},      // D-05 auction time from t
        {Name: "Seller", Value: sellerName, Inline: true},       // D-05 best-effort; omit field if unresolved
        {Name: "Why you wanted it", Value: reasonOrNote},        // D-05 echo Hit.Reason + want.note
    },
    Footer: &discordgo.MessageEmbedFooter{Text: "EC-tunnel auction · SquireBot"},
}
// session.ChannelMessageSendEmbed(channelID, embed) — restapi.go:1812
```
**Note on `Hit.note`:** `wantmatch.Hit` carries `WantID, DiscordUserID, ItemID, ItemName, Reason` ([VERIFIED: `wantmatch/match.go:38-44`]) but NOT the want's `note`. If the embed wants the saved note (D-05 "why-you-wanted-it"), either extend `ForItem`'s SELECT to include `note` (the `wantlist_item.note` column exists — [VERIFIED: `migrations/00006_wantlist.sql:27`]) or have the EC job read it separately. Extending the Hit is cleaner; size it.

### Pattern 3: Cursor table + advance-only-on-success (copy `job_run` discipline)
**What:** A new `ec_auction_cursor(item_id, last_seen_t, updated_at)` table; read-before-diff, write-after-success-per-item. Mirrors `job_run`'s upsert grain ([VERIFIED: `store/jobstate.go:82-100`]).
**Anti-replay:** On restart, the cursor persists in SQLite, so the diff resumes where it left off — no backlog replay (ROADMAP criterion 4 / Pitfall 5). A never-seen item (no cursor row) should **NOT** DM its entire history on first sight — initialize the cursor to "now" (or to `max(t)` of the first poll WITHOUT DMing) so the first poll establishes a baseline and only subsequent new auctions alert. **This is a critical correctness detail — call it out explicitly in the plan.** (First-sight-establishes-baseline, then alert-on-advance.)

### Recommended new files
```
internal/backendsrv/
├── migrations/00008_ec_cursor.sql        # NEW: ec_auction_cursor table (forward-only, extend-only)
├── ec/                                    # NEW pkg: the EC monitor job (or jobs/ec_auction.go beside pigparse.go)
│   ├── ec.go                              #   RunMatch(ctx, db, session, fetch): poll-set → fetch → diff → wantmatch → notify
│   └── ec_test.go
├── enrich/
│   └── pigdetails.go                      # NEW: ParseItemDetail([]byte) → ItemDetail (do NOT reuse ParseToRows)
├── notify/
│   └── dm.go                              # MODIFIED: add ChannelMessageSendEmbed to Sender + SendEmbed (or Embed field on Alert)
├── store/
│   ├── eccursor.go                        # NEW: Get/SetECCursor + the poll-set query (DISTINCT item_id)
│   └── wantmatch hit note (optional)      # MODIFIED ForItem SELECT to add note (D-05) — size it
└── scheduler/scheduler.go                 # MODIFIED: register ec_auction_match + thread botSession
cmd/squirebot-server/main.go               # MODIFIED: reorder bot.Start before scheduler.Start; thread session
```
**Decision for the planner:** the EC job can live as a NEW package `internal/backendsrv/ec` (cleaner separation, the ARCHITECTURE-v2.2 doc names `ec_auction_match` as a registry job) OR as `enrich/jobs/ec_auction.go` beside `pigparse.go` (consistency with the existing job home). Recommend a dedicated `ec` package since it composes `enrich` + `store` + `wantmatch` + `notify` and would create an import-direction question if placed in `enrich/jobs` (which `enrich` parsers must not depend on `notify`/`wantmatch`).

### Anti-Patterns to Avoid
- **Reusing `enrich.ParseToRows` for `getdetails`:** different schema; `t` means direction there, timestamp here. Write a NEW parser.
- **Sending the embed directly from the EC job** (bypassing `notify`): skips both gates + dedup + `alert_log` → DM spam + ungated alerts. Route every send through `notify`.
- **DMing the full auction history on first sight of a never-cursored item:** baseline the cursor first (Pattern 3).
- **Conflating the `job_run` cadence cursor with the per-item `ec_auction_cursor`:** two distinct cursors, two distinct concerns.
- **Polling the whole catalog:** D-09 — derive the poll set ONLY from live wantlist rows.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Polite outbound HTTP (UA, ETag/304, backoff, Retry-After) | A new `http.Client` loop | `politefetch.Fetch` ([VERIFIED: `politefetch/politefetch.go:140`]) | Already implements all of D-09's "polite in code." |
| DM send + 50007 handling + dedup/cooldown + alert_log | A direct `ChannelMessageSend` in the job | `notify.Send`/`SendEmbed` ([VERIFIED: `notify/dm.go:108-197`]) | Two gates, typed 50007 (`ErrDMBlocked`), cooldown, and the inbox write are all here and tested. |
| Item → wantlisters match (active + mute gates, all-users fan-out) | A custom `SELECT` in the job | `wantmatch.ForItem` ([VERIFIED: `wantmatch/match.go:51`]) | The single shared seam; applies `active=1 AND muted=0` so no consumer can forget the mute gate. |
| Scheduled cadence + restart-safe cursor | A `time.Tick` loop / a cron lib | `scheduler` registry + `job_run` ([VERIFIED: `scheduler/scheduler.go:66-150`]) | Recover-isolated, immediate-on-startup, advance-after-run already proven. |
| Officer kill-switch + per-user opt-in | New flag tables/checks | `monitor_flag.ec_auction` + `notify_prefs.ec` (read inside `notify.Send`) | Both gates shipped in P20; EC inherits them with no new UI. |
| Cooldown window | A new constant scattered in the job | `cooldownEC = 22 * time.Hour` ([VERIFIED: `notify/dm.go:87`]) | Already the EC source's window (D-10 placeholder); soak-tunable in one place. |

**Key insight:** Phase 21 is ~80% glue. The only genuinely new logic is (a) the `getdetails` parser, (b) the per-item cursor diff with first-sight baselining, (c) the embed body, and (d) the spike. Everything else is calling shipped, tested code.

## Common Pitfalls

### Pitfall 1: PigParse assumed, never verified (the spike exists to kill this)
**What goes wrong:** Building the monitor assuming a real-time auction feed. **Resolution:** [VERIFIED 2026-06-05] per-auction data exists via `getdetails`; no global feed. The mandatory spike (above) confirms `t` advances live + measures coverage before the plan commits. **Warning signs:** "subscribe"/"stream"/"real-time"/"feed" wording; latency < 10 min; no cursor.

### Pitfall 5: Alert spam / cursor / restart replay
**What goes wrong:** Re-DMing the same auction every 10-min poll, or flooding old alerts on restart. **How to avoid (all locked):** dedup key = `(wantlist_item_id, source, item_id)` within the cooldown window — already enforced by `RecentAlertExists` ([VERIFIED: `store/alertlog.go:180-197`]) + `cooldownEC=22h`; advance the per-item `ec_auction_cursor` ONLY after a successful poll; baseline the cursor on first sight (Pattern 3) so a standing auction is never re-DMed and a restart never replays history. **Note:** the dedup filter is `send_status IN ('sent','dm_blocked')` ([VERIFIED: `store/alertlog.go:186`]) — a `dm_blocked` (50007) ALSO suppresses repeats, so a DMs-off user does not accrue a `dm_blocked` inbox row every cycle.

### Pitfall: the `t` field collision
**What goes wrong:** `getall.PigparseRow.t` is a DIRECTION (0/1/2); `getdetails.ItemAuctionDetail.t` is a TIMESTAMP and `u` is the direction. Confusing them produces a parser that diffs on direction or matches WTS on the wrong field. **How to avoid:** new dedicated parser + a clear doc comment; in `getdetails`, filter WTS on `u ∈ {0,2}` and diff on `t` (the date-time string).

### Pitfall: nullable price and best-effort seller
**What goes wrong:** Emitting `0pp` for a null `p`, or blocking the DM when the `players` map can't resolve a seller. **How to avoid:** `p` is `int32|null` — omit/label the price field when null; seller is genuinely best-effort (no seller field on `ItemAuctionDetail`) — omit the field silently when unresolved (D-05).

### Pitfall: scheduler/bot wiring order
**What goes wrong:** `scheduler.Start(ctx, db)` runs before `bot.Start` in `main.go` ([VERIFIED: `main.go:237` vs `:244`]) and doesn't receive the session — the EC job has no `*Session` to DM with. **How to avoid:** reorder (`bot.Start` first) and thread `botSession` into `scheduler.Start`, OR inject the session lazily. Concrete plan task.

## Code Examples

### Poll-set query (NEW store func — the only owner-agnostic read of wantlist_item)
```sql
-- D-01 (reason NOT filtered), D-03 (NULL item_id skipped). DISTINCT so a popular item polls once.
SELECT DISTINCT item_id, item_name
  FROM wantlist_item
 WHERE active = 1 AND item_id IS NOT NULL;
```
Source: derived from `migrations/00006_wantlist.sql:20-34` (the `wantlist_item` schema) — active/item_id columns [VERIFIED].

### wantmatch.ForItem (existing — call verbatim)
```go
// Source: wantmatch/match.go:51 [VERIFIED]
hits, err := wantmatch.ForItem(ctx, db, itemID) // []Hit, all users, active=1 AND muted=0
// each Hit: {WantID int64, DiscordUserID string, ItemID *int64, ItemName, Reason string}
```

### notify.Send call shape (existing — SendEmbed mirrors it)
```go
// Source: notify/dm.go:112 [VERIFIED] — Send(ctx, sender, db, Alert, now int64) error
alert := notify.Alert{
    WantID:        &hit.WantID,             // real id ⇒ dedup applies (NOT nil — that's the test path)
    DiscordUserID: hit.DiscordUserID,
    Source:        "ec_auction",            // ⇒ cooldownEC=22h, GATE on monitor_flag.ec_auction + notify_prefs.ec
    ItemID:        hit.ItemID,
    Detail:        &shortDetail,            // e.g. "~2000pp · seen 3m ago" — shows in the inbox row (alert_log.detail)
    // Body or Embed: the rendered DM (string today; SendEmbed adds the *MessageEmbed)
}
err := notify.Send(ctx, botSession, db, alert, time.Now().Unix())
// branch on: nil | notify.ErrCooledDown | notify.ErrDMBlocked | notify.ErrGatedOff | other
```
Source: `notify/dm.go:62-106, 112` [VERIFIED]. The `ErrCooledDown`/`ErrGatedOff` returns are EXPECTED outcomes the EC job logs at debug, not errors that fail the poll.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| EC monitor as a greenfield design (v2.2 research) | The spine (`notify`/`wantmatch`/`alert_log`/`bot`) is now BUILT + deployed (P20, 2026-06-05) | P20 shipped | Phase 21 is integration, not greenfield — research read the live code, not the research docs' "NEW pkg" framing. |
| `getall` for prices (daily) | `getdetails` for per-auction events (~10-min EC poll) | This phase | A second, different PigParse code path with a NEW parser. |

**Not deprecated, but note:** `.planning/research/STACK-v2.2.md` proposed `notify.Send` would handle both — but the SHIPPED `Send` is string-only. D-04's embed needs the new path. Trust the code, not the pre-build research.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The `players` map in `ItemDetail` may not reliably map to a specific auction record (keys are stringified indices/handles; relationship undocumented). | PigParse contract / Pitfall | LOW — seller is best-effort by design (D-05); unresolved → omit. The spike measures the actual hit-rate. |
| A2 | A ~10-min `dueEC` predicate (or run-every-tick) is the right cadence; `checkInterval` already paces at 10 min. | Pattern 1 | LOW — matches the locked ~10-min cadence; trivially tunable. |
| A3 | First-sight baselining (cursor = max(t) of first poll, no DM) is the correct anti-replay seed for a never-cursored item. | Pattern 3 | MEDIUM — if wrong, either the first poll floods history (too eager) or misses the first real auction (too conservative). Recommend baseline-then-alert; verify in the plan's tests. |
| A4 | Seller resolution is genuinely best-effort and may have a low hit-rate; the spike quantifies it. | PigParse contract | LOW — D-05 already frames it best-effort. |
| A5 | Reordering `bot.Start` before `scheduler.Start` (to thread the session) has no hidden dependency. | Pattern 1 wiring | LOW — both are non-blocking, ctx-tied, independent goroutines ([VERIFIED: `bot/bot.go:86-146`, `scheduler/scheduler.go:115-150`]); order is currently arbitrary. |

## Open Questions

1. **D-06 item link target — NO per-item frontend route exists.**
   - What we know: [VERIFIED] `web/src/routes/` has `account, admin, bank-coin, char-meta, notifications, wantlist` — **no `item` route, no `[id]` dynamic route** (`find web/src/routes -iname "*item*"` → empty). The frontend renders items inside a DataGrid; the existing per-item link idiom is the **P1999 wiki URL**: `wikiUrlFor(name)` → `https://wiki.project1999.com/<Item_Name>` ([VERIFIED: `web/src/lib/tooltip/composeNotes.ts:81-84`]) and the stored `wiki_url` column ([VERIFIED: `web/src/lib/api.ts:98`]). There is no deep-linkable `https://squirebot.quest/item/<id>` view.
   - What's unclear: whether to (a) **fall back to the wiki URL** (zero new frontend work; but D-06 explicitly prefers the project's own view), or (b) **build a small `/item/[id]` SvelteKit route** (in-ecosystem, shows in-bank holders + enrichment, but new frontend + a read-API item endpoint).
   - Recommendation: **Default to the wiki fallback** for this phase to keep it backend-only and unblocked (D-06's own ⚠ allows it), and size "build a per-item route" as an OPTIONAL stretch task OR a follow-up. The planner should make this a single explicit decision early (it affects whether the phase touches `web/` at all). If a route IS built, the embed `URL` becomes `https://squirebot.quest/item/<item_id>`.

2. **getdetails key: name vs. id.**
   - What we know: both `getdetails/{server}/{itemname}` and `getdetails/{itemid}` exist [VERIFIED]. The wantlist stores both `item_id` and `item_name` ([VERIFIED: `migrations/00006_wantlist.sql:23-24`]).
   - Recommendation: the spike pins this. The **id form** avoids name-drift (CLAUDE.md: "item Name strings can drift but IDs are stable") and the poll set already carries `item_id`. Prefer id unless the spike shows the id-keyed endpoint behaves differently. Document the choice.

3. **`wantmatch.Hit` lacks `note`/`priority` for the embed's "why-you-wanted-it" (D-05).**
   - What we know: `Hit` has `Reason` but not `note` ([VERIFIED: `wantmatch/match.go:38-44`]); `wantlist_item.note` exists ([VERIFIED: `migrations/00006_wantlist.sql:27`]).
   - Recommendation: extend `ForItem`'s SELECT + `Hit` to include `note` (small, additive) OR read the note in the EC job. Size it; the embed can ship with just `Reason` if note is descoped.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `github.com/bwmarrin/discordgo` | embed DM send | ✓ | v0.29.0 (in `go.mod`, module cache) | — |
| PigParse `getdetails` endpoint | the monitor's data | ✓ | live (Swagger fetched 2026-06-05) | `getmultiple`/`lastWTSSeen` coarse signal (D-08) |
| Live EC tunnel coverage (a human parsing) | actual auction events | ⚠ best-effort/bursty | — | D-07: ship anyway, document the gap |
| Go toolchain (1.24) | build | ✓ (assumed; user installs toolchains) | 1.24 | — |
| Discord bot token (`DISCORD_BOT_TOKEN` on the box) | live DM send | ✓ (set on the VPS per P20) | — | bot disabled ⇒ EC job sends nothing (clean, non-fatal — [VERIFIED: `bot/bot.go:104-107`]) |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** EC tunnel coverage is best-effort by nature (D-07).

## Validation Architecture

> `.planning/config.json` not re-read for an explicit `nyquist_validation: false`; the project uses Go `testing` throughout (every package has `*_test.go`). Included as enabled.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` |
| Config file | none (standard `go test`) |
| Quick run command | `go test ./internal/backendsrv/ec/... ./internal/backendsrv/enrich/... ./internal/backendsrv/notify/... ./internal/backendsrv/store/...` |
| Full suite command | `go test ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| WANT-05 | `getdetails` body parses to `ItemAuctionDetail[]`; malformed tolerated | unit | `go test ./internal/backendsrv/enrich/ -run TestParseItemDetail` | ❌ Wave 0 |
| WANT-05 | new WTS auction (`t`>cursor, `u∈{0,2}`) → match → send; WTB-only (`u`=1) does NOT | unit | `go test ./internal/backendsrv/ec/ -run TestRunMatch` | ❌ Wave 0 |
| WANT-05 | cursor advances only after success; first-sight baselines (no replay) | unit | `go test ./internal/backendsrv/ec/ -run TestCursor` | ❌ Wave 0 |
| WANT-05 | `notify.SendEmbed` enforces both gates + dedup + alert_log (mirror `dm_test.go`) | unit | `go test ./internal/backendsrv/notify/ -run TestSendEmbed` | ❌ Wave 0 (extend `notify/dm_test.go`) |
| WANT-05 | poll-set query returns DISTINCT active catalog item_ids only (skips NULL/custom) | unit | `go test ./internal/backendsrv/store/ -run TestECPollSet` | ❌ Wave 0 |
| WANT-05 | migration `00008` applies idempotently; `ec_auction_cursor` created | unit | `go test ./internal/backendsrv/migrations/ -run TestMigrate` | ✅ (extend `migrate_test.go`) |

### Sampling Rate
- **Per task commit:** the quick run command for the touched package(s).
- **Per wave merge:** `go test ./...`.
- **Phase gate:** `go test ./...` green + (D-08) the spike artifact documented + a live deploy DM smoke (P20 precedent: the test-alert path proved DM works; EC adds the embed path — smoke an embed DM on the box).

### Wave 0 Gaps
- [ ] `internal/backendsrv/enrich/pigdetails_test.go` — `getdetails` parser (fixture from the spike's real capture, named `pigparse-getdetails-<item>.json` per CLAUDE.md fixture convention)
- [ ] `internal/backendsrv/ec/ec_test.go` — the diff/match/send job (fake fetch + fake Sender, the `dm_test.go` precedent)
- [ ] `internal/backendsrv/store/eccursor_test.go` — cursor get/set + poll-set query
- [ ] `internal/backendsrv/notify/dm_test.go` — EXTEND with `SendEmbed` cases (mirror the existing `Send` gate/dedup/50007 tests)
- [ ] `internal/backendsrv/migrations/migrate_test.go` — EXTEND for `00008`
- Framework install: none (Go testing built-in).

## Security Domain

> `security_enforcement` not explicitly disabled — included.

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | EC is a backend job; no new auth surface. DM recipient identity comes from `wantmatch.Hit.DiscordUserID` (resolved from the want row, never a request body). |
| V4 Access Control | yes | Both gates enforced by `notify.Send` (officer `monitor_flag.ec_auction` + user `notify_prefs.ec`); per-want mute via `wantmatch` (`muted=0`). The EC job adds NO new HTTP route, so no IDOR surface. |
| V5 Input Validation | yes | The `getdetails` parser must tolerate malformed PigParse data (the `enrich.ParseToRows` 1%-tolerance precedent — [VERIFIED: `enrich/pigparse.go:24,93-97`]); `p` nullable; coerce/skip bad records. PigParse URL is a hardcoded constant (SSRF mitigation — the `PigparseURL` precedent, [VERIFIED: `jobs/urls.go:24-28`]); the item name/id in the path must be URL-escaped (the `enrich.EncodeURIComponent` idiom, [VERIFIED: `jobs/urls.go:53`]) — an item name with special chars must not break the URL or enable path traversal. |
| V6 Cryptography | no | none. |
| V9/V7 Logging | yes | NEVER log the DM Body, item names beyond ids, or the `players` map raw — the `notify` V7 discipline ([VERIFIED: `notify/dm.go:29-30,154`]). Log source + item_id + want id + status only. |

### Known Threat Patterns for Go + PigParse + Discord
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| SSRF via attacker-controlled item name in the `getdetails` URL | Tampering | Hardcoded host/scheme constant + `EncodeURIComponent` the name segment; item names originate from the catalog/wantlist, not arbitrary user input. |
| DM spam (carpet-bomb on every poll) → users mute the bot → permanent 50007 | DoS / trust | Cursor diff + `RecentAlertExists` cooldown + first-sight baselining (Pitfall 5). |
| Logging the `players` map / DM body (other guildies' data) | Info disclosure | V7 — log ids/status only. |
| Oversized/runaway PigParse response | DoS | `politefetch` already caps the body at ~16 MB ([VERIFIED: `politefetch/politefetch.go:105`]). |

## Project Constraints (from CLAUDE.md)
- **PigParse is a typed API — NEVER scrape.** The `getdetails` endpoint is JSON; parse it, never HTML-scrape. [VERIFIED: this phase uses `getdetails` JSON only.]
- **item_id is the stable join key.** `wantmatch.ForItem` keys on it; custom NULL-item wants can't participate (D-03). [VERIFIED in code.]
- **In-process scheduler, no cron daemon.** Add a `*Job` to the existing registry. [VERIFIED.]
- **Schema evolution extend-only.** `ec_auction_cursor` is a forward-only `00008` migration; `00001`–`00007` are SHIPPED and NOT edited (the `00007` header states this — [VERIFIED: `migrations/00007_notify.sql:6-7`]).
- **Structured `slog` logging; never log PII/user queries/secrets.** [VERIFIED: the `notify`/`scheduler`/`politefetch` precedent.]
- **No Discord bot/OAuth in the watcher (HARD CONSTRAINT).** All EC work is backend + the guild's own Discord. [VERIFIED: the watcher is `cmd/squirebot`, untouched by this phase.]
- **Watcher `WatcherMaxSchemaVersion`:** N/A — `ec_auction_cursor` is a backend-only table the watcher never reads/writes (no `_meta.schema_version` bump; the watcher write-contract is unaffected). Confirm no watcher schema check trips (it won't — the watcher targets the ingest API, not these tables).

## Sources

### Primary (HIGH confidence — read directly this session)
- Codebase (cited path:line throughout): `scheduler/scheduler.go`, `enrich/jobs/pigparse.go`, `enrich/pigparse.go`, `enrich/jobs/urls.go`, `enrich/politefetch/politefetch.go`, `notify/dm.go`, `wantmatch/match.go`, `store/alertlog.go`, `store/notifyprefs.go`, `store/jobstate.go`, `webadmin/monitors.go`, `bot/bot.go`, `cmd/squirebot-server/main.go`, `migrations/00006_wantlist.sql`, `migrations/00007_notify.sql`.
- `github.com/bwmarrin/discordgo@v0.29.0` module cache — `message.go:422-443` (embed structs), `restapi.go:1812` (`ChannelMessageSendEmbed`). HIGH.
- `web/src/routes/` listing + `web/src/lib/tooltip/composeNotes.ts:81-84` + `web/src/lib/api.ts:98` (D-06 finding: no per-item route; wiki idiom). HIGH.
- `https://pigparse.azurewebsites.net/swagger/v1/swagger.json` — re-fetched 2026-06-05; confirms `getdetails` shapes, `ItemAuctionDetail{u,i,p,t}`, `Item.lastWTSSeen/lastWTBSeen`, and NO global "since-T" feed. HIGH.

### Secondary (HIGH — project research docs, cross-checked against code)
- `.planning/research/STACK-v2.2.md` (PigParse endpoint inventory + discordgo) — matches the live Swagger.
- `.planning/research/PITFALLS-v2.2.md` (Pitfall 1 spike-first, Pitfall 5 dedup/cursor/replay).
- `.planning/research/ARCHITECTURE-v2.2.md` (`ec_auction_match` registry job, the one-match seam).
- `.planning/phases/21-ec-tunnel-auction-monitor/21-CONTEXT.md` (D-01..D-11 — authoritative).
- `.planning/ROADMAP.md` §"Phase 21" (4 success criteria).

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every library is already in the deployed binary; no new dependency.
- Architecture / integration points: HIGH — read directly from live code with line citations.
- PigParse contract: HIGH — re-verified against the live Swagger this session.
- Spike feasibility (will `t` actually advance live?): MEDIUM — that is precisely what the mandatory spike measures; the API SHAPE is confirmed, the live COVERAGE is not (D-07/D-08 handle this honestly).
- D-06 link target: HIGH (finding) — no per-item route exists; the contingency is real and surfaced.

**Research date:** 2026-06-05
**Valid until:** ~2026-07-05 for the codebase facts (stable, deployed); the PigParse live-coverage assessment is only as current as the spike (re-run if the plan stalls > 2 weeks).
