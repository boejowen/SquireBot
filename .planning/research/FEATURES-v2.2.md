# Feature Research

**Domain:** Per-user wantlist + Discord-DM alerting for a ~12-person Project 1999 EverQuest guild (v2.2 milestone, on a live Go+SQLite backend / SvelteKit site with Discord OAuth identity)
**Researched:** 2026-06-02
**Confidence:** HIGH for PigParse API shape + Discord DM constraints (verified against the live OpenAPI spec + Discord docs); MEDIUM for matching norms (verified against TunnelQuestBot, the canonical prior art); MEDIUM for quest→NPC chain (verified against P1999 wiki structure)

> **Note:** the prior `FEATURES.md` covered the v1 inventory-tool landscape (2026-04-30) and is preserved in git history (last committed version). This file is scoped to the **v2.2 Wantlist + Discord-pinger** milestone only, per the milestone research request.

> Scope discipline: this is a **12-person trust-rich guild**, not a SaaS product. Several "table stakes" that a public notification product would need (granular consent UX, multi-channel routing, abuse controls) collapse to trivial or skippable here. Where that's true it is called out so the roadmap does not over-engineer.

---

## Critical Upstream Finding: PigParse exposes NO live auction event stream

This reshapes the entire EC-tunnel-monitor design, so it leads.

Verified against the live OpenAPI spec (`/swagger/v1/swagger.json`, "P99 Pricing Data API v1"):

| Endpoint | What it returns | Useful for alerting? |
|----------|-----------------|----------------------|
| `GET /api/item/getall/{server}` | Array of `AuctionItem` — per-item **aggregates** (`tc`/`ta` total count/avg, 30/60/90d/6m windows) + `l` = **last-seen timestamp**. "Rebuilt every 10 minutes." | **Yes, by polling + diffing `l`** |
| `GET /api/item/get/{server}/{itemname}` | One `Item` — aggregates + `lastWTSSeen` / `lastWTBSeen` timestamps | Yes (per-item recency probe) |
| `GET /api/item/getmultiple/{server}` | Bulk `Item[]` | Yes (batch recency probe) |
| `GET /api/item/getdetails/{itemid}` | `ItemDetail.items[]` = per-auction events `{u: WTS/WTB/BOTH, p: price, t: timestamp}` + `players` map | Has real events, **but spec says "DO NOT USE THIS… transfer ALOT of data"** — not a supported feed |
| `POST /api/item/auctionParse` | Stateless: you POST one chat line, it returns structured `Auction{player, items[]}` | **Reusable WTS-channel parser** (see below) |

**Implication:** there is no webhook/SSE/"give me auctions since T" endpoint. The EC monitor must be a **scheduled poll-and-diff**: every N minutes pull `getall/1` (or `getmultiple` for just wantlisted item IDs), compare each item's `lastWTSSeen`/`l` against the value stored at the last poll, and fire when it advances. This is the single biggest complexity driver in the milestone and the reason the EC monitor's plumbing is MEDIUM (not LOW).

**Second-order win:** `POST /api/item/auctionParse` already turns a raw EC line like `"Fuxi auctions, 'WTS Silver Chitin Hand Wraps 1.3 / …'"` into `{player, items:[{name, price, auctionType}]}` (its own default example is exactly this). The WTS-channel monitor (area 3) can reuse PigParse's battle-tested parser instead of writing a fresh EC-grammar regex — a strong build-vs-reuse lever.

---

## Feature Landscape

### Area 1 — Per-user Wantlist

#### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Add item to wantlist by searching the existing item catalog | A wantlist is meaningless without "what do I want" | LOW | Reuse the existing cross-character fuzzy search / item catalog; **store EQ item ID as the row key**, not a name string (names drift; IDs are the stable join key, already established system-wide) |
| Remove item from wantlist | Wants change constantly | LOW | Soft- or hard-delete; at 12 users either is fine |
| List "my wantlist" (per-user, Discord-identity-tied) | Each guildie owns their list | LOW | Key rows by `discord_user_id` (AUTH-09 / LINK-02 already capture it); not by character — wants are person-scoped, not toon-scoped |
| Want-reason: **buy** vs **quest** | Drives which monitors fire (buy → EC/WTS price alerts; quest → raid-target alerts). Hard requirement from the milestone, not cosmetic | LOW | Enum column; this reason is the routing key for the 4 monitors |

#### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Per-item **price threshold** ("only alert under X pp") | The #1 noise-killer for buy-wants; an unbounded "alert me on any Hierophant's Cloak" is useless when one's up at 3× market | LOW | Single int column compared to parsed/aggregate price at alert time. Seed the placeholder from PigParse's 30-day average for a sane default |
| Per-item priority | Lets a future digest sort + lets users mentally triage | LOW | Small enum (low/normal/high); pure data, no behavior needed for MVP |
| Per-item free-text note ("for my epic 1.0", "alt only") | Self-authored context; shows in the DM and on the site | LOW | Plain text column |
| **"Already in the guild bank?" inline flag** on the wantlist | Ties straight to **Core Value** ("what do I need and where in the guild is it?") — a want satisfiable from the shared bank shouldn't need an EC alert | LOW–MEDIUM | Join wantlist item IDs against the existing consolidated `bank`/`view` data. High value, cheap, uniquely on-brand |
| Auto-suggest wants from `gear_check`/`spell_check` MISSING rows | The guild already computes "what you're missing vs Velious tiers"; one-click "add my MISSING gear" | MEDIUM | Compelling but defer past MVP — convenience layer on top of working CRUD |

#### Anti-Features

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Shared / guild-wide aggregated wantlist | "We could coordinate buys!" | A *cross-user* wantlist view invites a permissions/social rabbit hole and isn't asked for | Per-user lists for MVP; a read-only aggregate is cheap to add later if requested |
| Quantity / "I want 4 of these" | Stacking exists in EQ | Adds matching + partial-fill dedup complexity for near-zero value to 12 people questing/buying single gear pieces | Single-want rows; re-add after purchase |
| Wantlist for arbitrary free-text (non-catalog) items | "I want something not in the catalog" | Breaks the item-ID model everything joins on; reintroduces name-drift matching everywhere | Require catalog selection; the catalog already covers P99 items |

---

### Area 2 — Notification UX (shared across all 4 monitors)

The layer that decides whether guildies keep notifications on or mute the bot in week one. It is mostly **shared infrastructure** and should be built **once**, before any individual monitor.

#### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Opt-in / consent to DM** | Discord bots **cannot force a DM** — the recipient must share a server with the bot AND have "DMs from server members" enabled, else the API returns `403 Cannot send messages to this user`. Consent is a hard API gate, not a nicety | MEDIUM | On first enable, have the user DM the bot once (or run a `/start` command) to open the channel; detect+surface the 403 and explain the privacy-setting fix. **Without this the milestone silently fails.** |
| **Per-monitor on/off toggle** | A user may want EC price alerts but not raid pings | LOW | A few booleans keyed to `discord_user_id` |
| **Alert dedup + cooldown** | The explicit milestone concern: don't DM the same auction 5×. EC polling re-sees the same standing auction; WTS channels repeat the same line on a timer | MEDIUM | Store `(user, item_id, source) → last_alerted_at`; suppress re-alert within a window (suggest **30–60 min** default — note: **no canonical value exists even in prior art**, so this is a tunable decision). For EC, also key on the advancing `lastSeen` timestamp so a genuinely new auction after cooldown still fires |
| **In-site notification log / history** | The reliable fallback when a DM bounces (403 / DMs off) or gets buried; also the "did the bot catch it?" audit trail | LOW–MEDIUM | Append every fired/attempted alert to a table; render a `/notifications` page. This is the **safety net that makes the unreliable DM channel acceptable** — treat as table stakes |

#### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Snooze / mute a single want** | The targeted relief valve — "stop pinging me about this for 24h" without deleting the want | LOW | `snoozed_until` timestamp per want, checked at alert time |
| **Digest mode vs instant DM** | One "everything that matched today" message instead of trickle DMs; big noise cut for low-priority wants | MEDIUM | Per-user (or per-priority) channel choice; needs a scheduled batch + pending-digest queue. Defer to v1.x unless instant DMs prove too spammy in soak |
| **Quiet hours** | No 4am DMs | LOW–MEDIUM | Per-user window; suppress or queue into digest. Cheap if a digest queue exists; a small standalone gate otherwise |

#### Anti-Features

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Multi-channel routing (email/SMS/push/webhook) | "Let me pick my channel" | Each channel is its own delivery integration + failure mode for 12 people who all live in the guild Discord | Discord DM only, with the in-site log as universal fallback |
| Posting alerts to a shared guild channel | "Everyone could see deals" | Turns a personal wantlist into channel spam | Per-user DM; a public "deals" feed would be a separate explicit opt-in feature |
| Read receipts / "mark as actioned" workflow | "Track what I bought" | CRM-ifies a hobby tool; nobody maintains this at 12 people | Just delete/snooze the want once satisfied |
| Real-time / sub-minute alerting | "Beat other buyers!" | PigParse rebuilds only every 10 min and has no event stream — sub-minute is physically impossible upstream | Poll on PigParse's ~10-min cadence; that's the ceiling |

---

### Area 3 — Auction / WTS Matching Accuracy

Spans the EC monitor (matching against PigParse data, where item IDs are already resolved) and the WTS-channel monitor (matching against **free-text** trade messages from 3 external Discord servers).

**Key asymmetry:**
- **EC-tunnel side is easy:** PigParse already resolved each auction to an EQ **item ID**. Match = exact item-ID equality against the wantlist. No fuzzy logic. This is why the EC monitor's *matching* is LOW even though its *polling* is MEDIUM.
- **WTS-channel side is hard:** raw human trade chat ("WTS Hierophant's, FBSS, cloak of flames pst") must be matched to wantlist item IDs. This is where false positives live.

#### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Exact item-ID match for EC-tunnel alerts | PigParse gives you the ID; anything fuzzier adds error | LOW | Direct equality |
| WTS-channel: parse the line first, then match | Trade lines pack many items + prices + noise ("PST", "OBO", "last call") | MEDIUM | **Reuse `POST /api/item/auctionParse`** — PigParse already strips acronyms, splits on `/` and `l`/`\|`, tags WTS/WTB, extracts prices. Don't hand-roll the grammar |
| WTS/WTB discrimination | A buy-want should only fire on **WTS** (someone selling to me); WTB is noise to a buyer | LOW | `auctionType` (`0=WTS,1=WTB,2=BOTH`) is in the parsed output; filter per want-reason |
| Price threshold enforcement at match time | Stops "match but it's 3× market" alerts | LOW | Compare parsed price (or PigParse aggregate) to the Area-1 per-item threshold |

#### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Name-variant / abbreviation handling** (EQ players type "FBSS", "SCHW", "cloak of flames") | EQ trade chat is abbreviation-soup; without it the WTS monitor misses most real matches | MEDIUM–HIGH | TunnelQuestBot's proven rule: **known items → exact full-name match only; unknown/user terms → substring match.** Add a small curated `alias → item_id` table for the abbreviations the guild actually trades. No general fuzzy NLP |
| **Per-want custom keyword/alias override** | "For THIS want, also match 'banded'" | LOW | Optional free-text keyword per want; routes into the "unknown term → substring" path |
| **False-positive guardrails** (word-boundary, min length, no inside-word hits) | "Banded" shouldn't match "abandoned"; short tokens cause carnage | MEDIUM | Word-boundary matching + min token length; uppercase-normalize (PigParse already does). Aho-Corasick (TunnelQuestBot's choice) is overkill at this scale — a compiled per-poll matcher suffices |

#### Anti-Features

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Free-form fuzzy / Levenshtein matching on trade chat | "Catch typos!" | Explodes false positives in a domain full of similar item names; users learn to ignore the bot | Exact-for-known + curated alias table + bounded substring for unknown |
| Matching WTB auctions for buy-wants | "More coverage" | A buy-want firing because someone *else* wants to buy it is pure noise | Filter strictly on WTS for buy-wants |
| Cross-server name reconciliation (Green/Red/Quarm) | PigParse exposes them | Guild plays **Blue only** (locked out-of-scope); multiplicative complexity for zero value | Hardcode `server=1` (Blue) everywhere |

---

### Area 4 — Quest-Target Raid Monitor

DM a guildie when a raid-target NPC tied to a **quest-reason** want is announced in the 3 Raid Alliance Discord servers. The most novel and most curation-heavy area.

**The chain:** `wantlisted item (reason=quest)` → `quest that grants it` → `raid-target NPC(s) gating the quest` → `match that NPC name in raid-announcement channels` → DM. Verified from the wiki: e.g. the Monk epic "Immortals" book drops from named mobs in Skyfire; quest gear gates on encounters like Trakanon/Severilous. The item→NPC relationship is **not in PigParse and not cleanly machine-derivable** — it must be curated.

#### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Curated `quest-item → raid-target NPC(s)` lookup** | Explicit open prerequisite in PROJECT.md; nothing automates it | MEDIUM–HIGH (mostly **data/curation** cost, low code) | A maintained table mapping item ID → one-or-more NPC names. Seed from P1999 wiki quest pages (wiki API can assist; a human must confirm the gating NPC). **Roadmap should treat populating this as a real work item, not a footnote** |
| Match an announced NPC name against the lookup | The whole point | LOW–MEDIUM | Substring/keyword match on raid-announcement text; same matcher infra as Area 3, but matching **NPC names** |
| DM only users whose quest-reason want chains to that NPC | Targeting | LOW | Join announced NPC → lookup → quest-reason wants → `discord_user_id` |

#### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| One NPC → many wanted items fan-out | A single raid mob gates loot for several quests/wants | LOW | Lookup is many-to-many; fan-out at alert time |
| Show the *why* in the DM ("Trakanon up — needed for your quest X") | Turns a cryptic NPC ping into actionable context | LOW | Carry quest/item context from the lookup into the DM body |

#### Anti-Features

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Auto-derive the quest→NPC chain by scraping wiki quest prose | "Avoid manual curation" | Quest pages are unstructured; extracting *which* NPC gates *which* want is brittle and often wrong enough to erode trust | Wiki-assisted **human-curated** table; the dataset is small (only items guildies actually quest-want) |
| Monitoring raid batphone / DKP / spawn logistics | "While we're reading those channels…" | Scope explosion into raid management; DKP is explicitly out-of-scope (EQDKP/OpenDKP own it) | Match only NPC names tied to quest-reason wants; ignore everything else |
| Predicting spawn windows / variance timers | "Tell me when it'll pop" | Needs spawn-state machine + ToD feeds; huge separate-product scope | React to *announcements only*; the guild has raid tools for timing |

---

## Feature Dependencies

```
Discord identity (AUTH-09 / LINK-02)  ── already shipped ──> EVERYTHING below

Item catalog (existing item_master-equivalent + fuzzy search)
    └──requires──> Wantlist CRUD (item-ID keyed)

Wantlist CRUD
    └──requires──> Notification infrastructure (consent, dedup/cooldown, log, toggles)
                       ├──requires──> EC-tunnel monitor (PigParse poll-and-diff + scheduler)
                       ├──requires──> WTS-channel monitor (Discord bot in 3 servers + auctionParse reuse)
                       └──requires──> Quest-target monitor (curated quest→NPC table + Discord bot)

Existing scheduler (in-process daily PigParse / weekly wiki jobs)
    └──enhances──> EC-tunnel monitor (add a ~10-min poll job alongside existing jobs)

PigParse auctionParse endpoint ──enhances──> WTS-channel monitor (free reuse of the EC grammar)

Raid Alliance bot invites (OPEN PREREQ) ──gates──> WTS-channel monitor
Raid Alliance bot invites (OPEN PREREQ) ──gates──> Quest-target monitor
Curated quest→NPC table (OPEN PREREQ)   ──gates──> Quest-target monitor

Digest mode ──conflicts/overlaps──> instant DM (one delivery mode per user/priority)
```

### Dependency Notes

- **Notification infra is the keystone.** All 4 monitors are just *producers* feeding one shared delivery+dedup+log pipeline. Build the pipeline once, first; bolt monitors on after. Per-monitor dedup/cooldown/consent would triple the work and fragment the hygiene logic.
- **EC monitor depends on the scheduler, not on bot invites** — pure PigParse poll-and-diff, entirely backend. This is the *unblocked* monitor to ship first (matches PROJECT.md sequencing).
- **WTS + Quest monitors both depend on Discord bot presence** in the 3 external servers (admin invite, same open prereq) — sequence them together, after invites land.
- **Quest monitor additionally depends on the curated table** — its long pole is data curation, which can start in parallel now (no code needed to begin populating).
- **Digest overlaps instant-DM** — one mode per user at a time; don't build both in MVP.

---

## MVP Definition

### Launch With (this milestone, unblocked half)

- [ ] **Wantlist CRUD** — add/remove by catalog search, item-ID keyed, Discord-tied, with want-reason (buy/quest) + price threshold + note. *The product surface; everything else is plumbing.*
- [ ] **Notification infrastructure** — DM consent/403-handling, per-monitor toggle, dedup+cooldown, in-site notification log. *The keystone; unreliable DMs are unacceptable without the log fallback.*
- [ ] **EC-tunnel auction monitor** — scheduled PigParse `getall/1` (or `getmultiple` of wantlisted IDs) poll-and-diff on `lastWTSSeen`, exact item-ID match, price-threshold + WTS filter, DM via the infra. *The unblocked, highest-value monitor.*
- [ ] **"Already in guild bank?" flag** on wantlist rows — *cheap, on-brand, reinforces Core Value.*

### Add After Validation / Once Invites Land

- [ ] **WTS-channel monitor** — Discord bot in the 3 Raid Alliance servers, reuse `auctionParse`, exact-known + curated-alias matching, WTS filter, price threshold. *Gated on bot invites.*
- [ ] **Quest-target raid monitor** — curated quest→NPC table + NPC-name match in raid channels. *Gated on invites + the table (start curating now).*
- [ ] **Snooze per-want** — once instant DMs are live and users want a per-item mute.

### Future Consideration

- [ ] **Digest mode + quiet hours** — only if soak shows instant DMs are too noisy.
- [ ] **Auto-suggest wants from gear_check/spell_check MISSING** — convenience on working CRUD.
- [ ] **Read-only guild-aggregate wantlist** — only on explicit request.

---

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Wantlist CRUD (item-ID keyed, reason, threshold, note) | HIGH | LOW | P1 |
| Notification infra (consent, dedup/cooldown, in-site log, toggle) | HIGH | MEDIUM | P1 |
| EC-tunnel monitor (PigParse poll-and-diff) | HIGH | MEDIUM | P1 |
| "Already in guild bank?" flag | MEDIUM | LOW | P1 |
| WTS-channel monitor (bot + auctionParse + aliases) | HIGH | HIGH | P2 (invite-gated) |
| Quest-target monitor (curated table + NPC match) | MEDIUM | HIGH | P2 (invite + data gated) |
| Name-variant / alias table | MEDIUM | MEDIUM | P2 (needed by WTS monitor) |
| Snooze per-want | MEDIUM | LOW | P2 |
| Digest mode / quiet hours | LOW–MEDIUM | MEDIUM | P3 |
| Auto-suggest from MISSING | MEDIUM | MEDIUM | P3 |

---

## Competitor / Prior-Art Feature Analysis

| Feature | TunnelQuestBot (jamesjamail) | PigParse / EqTool (Scott) | SquireBot's Approach |
|---------|------------------------------|---------------------------|----------------------|
| Auction data source | Reads the user's **own EQ log file** locally | Crowd-sourced uploads → aggregated REST API | Consume PigParse REST (no log reading — watcher stays log-free per HARD CONSTRAINT) |
| Item matching | Known→exact, unknown→substring; Aho-Corasick; strips PST/OBO; uppercase-normalize | `auctionParse` does the same normalization server-side | Reuse PigParse `auctionParse`; adopt known-exact/unknown-substring + a small alias table |
| WTS/WTB tagging | Parses first word (WTB/Buying vs WTS/Selling) | `auctionType` enum in API | Filter on `auctionType` per want-reason |
| Price parse | Best-effort price after the item | Aggregates + per-event `p` price | Threshold-compare against parsed/aggregate price |
| Notification channel | Discord (watch notifications) | n/a (data API only) | Discord DM + in-site log fallback |
| Dedup/cooldown | Message-hash cache to skip reprocessing identical lines | n/a | `(user,item,source)→last_alerted_at` cooldown + advancing-timestamp key |
| Live event stream | Has it (local log tail) | **None** — only 10-min aggregate rebuild | Poll-and-diff (no stream available upstream) |

---

## Confidence & Open Questions

- **HIGH:** PigParse has no push/event endpoint; EC monitor must poll-and-diff. (Verified against the live OpenAPI spec.) `auctionParse` is reusable for WTS-channel parsing. (Verified — its default example is a real multi-item EC line.)
- **HIGH:** Discord bots cannot force DMs; consent + 403 handling + in-site log fallback are mandatory, not optional. (Verified against Discord docs.)
- **MEDIUM:** Cooldown interval (30–60 min suggested) has **no canonical value** even in prior art — it's a tunable design decision for soak, not a copyable constant.
- **MEDIUM:** Quest→NPC mapping is human-curation work seeded from the wiki; size = only the items guildies actually quest-want (small, but real).
- **OPEN (carried from PROJECT.md, not resolvable by research):** the 3 Raid Alliance bot invites (gates Areas 3+4) and populating the quest→NPC table.
- **OPEN for requirements:** is there an officer-courtesy concern with polling PigParse every ~10 min? The existing `politeFetch` (User-Agent, backoff, ETag) should cover it, but a heavier cadence than the current daily job may warrant a courtesy note to PigParse's maintainer (Scott / EqTool) — flagged, parallels the waived ENRICH-09.

## Sources

- PigParse live OpenAPI spec — `https://pigparse.azurewebsites.net/swagger/v1/swagger.json` (P99 Pricing Data API v1; `Auction`, `AuctionItem`, `Item`, `ItemDetail`, `ItemAuctionDetail` schemas; `getall`/`get`/`getmultiple`/`getdetails`/`auctionParse` endpoints) — HIGH
- TunnelQuestBot (jamesjamail) — `https://github.com/jamesjamail/TunnelQuestBot` — canonical prior art for P99 auction-watch matching (known-exact / unknown-substring, acronym stripping, WTS/WTB tagging, message-hash dedup) — MEDIUM
- Discord support — Blocking & Privacy / Message Requests — `https://support.discord.com/hc/en-us/articles/217916488` , `https://support.discord.com/hc/en-us/articles/7924992471191` (bot DMs require shared server + DM-enabled; 403 otherwise) — HIGH
- P1999 Wiki — Category:Raid_Encounters, Category:Quests, epic-quest pages (quest-item → raid-target NPC chain structure) — `https://wiki.project1999.com/Category:Raid_Encounters` — MEDIUM
- Existing SquireBot context — `.planning/PROJECT.md` (v2.2 milestone scope, open prereqs, locked decisions), `CLAUDE.md` (item-ID join key, scheduler, Discord identity)

---
*Feature research for: v2.2 per-user wantlist + Discord-DM alerting (P1999 guild tool)*
*Researched: 2026-06-02*
