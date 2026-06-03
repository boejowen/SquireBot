# Stack Research — v2.2 Wantlist + Discord Pinger

> NOTE: `.planning/research/STACK.md` already exists and holds the **v1/v2.0-era** stack
> research (dated 2026-04-30) that CLAUDE.md still references. To avoid clobbering that
> historical doc, this v2.2 milestone research is written here as `STACK-v2.2.md`.
> If the orchestrator wants it merged into `STACK.md`, do that deliberately.

**Domain:** v2.2 Wantlist + Discord-pinger additions to an existing live Go/SQLite/SvelteKit backend (Project 1999 EverQuest guild tool)
**Researched:** 2026-06-02
**Confidence:** HIGH (discordgo + PigParse Swagger verified from source; CGO-free confirmed)

---

## TL;DR for the roadmap

- **Add exactly ONE Go dependency:** `github.com/bwmarrin/discordgo@v0.29.0` (pure Go, CGO-free, actively maintained). Everything else is REUSED.
- **Wantlist = new SQLite tables + one goose migration. No new dependency.** Reuse `modernc.org/sqlite` + `store` + Discord-session identity.
- **EC-tunnel auction monitor IS viable via PigParse — but only by polling `GET /api/item/getdetails/{server}/{itemname}` per watched item and diffing on the `t` (timestamp) field.** There is NO global auction-event feed and NO push. See the PigParse section — this is the load-bearing finding and it shapes the whole EC-monitor design.
- **Run the Discord gateway as a goroutine inside the existing `squirebot-server` binary**, not a separate systemd service. discordgo's `Session` auto-reconnects and is built for exactly this.
- **Bot token → reuse the existing `/etc/squirebot/squirebot.env` root-only `EnvironmentFile` pattern** (mirror `DISCORD_CLIENT_SECRET`). No secret-manager dependency.
- **Privileged intent gate:** reading WTS channel text requires the `MESSAGE_CONTENT` privileged intent. Toggle it in the Discord Developer Portal per bot application; <100-server bots self-enable without Discord review. This gates the WTS + raid-target features (already flagged "blocked on bot invites").

---

## Recommended Stack

### Core Technologies (NEW)

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| `github.com/bwmarrin/discordgo` | **v0.29.0** (released 2025-05-24, latest) | Discord gateway client (read WTS/raid channels) + REST (send DMs) | The de-facto Go Discord library; **pure Go / CGO-free** (deps: `gorilla/websocket` + `golang.org/x/...`, all pure Go) so it cross-compiles cleanly under the project's `CGO_ENABLED=0` static build. Actively maintained. The only credible alternative (`andersfylling/disgord`) was **archived July 2024**. Auto-reconnecting `Session` fits the "goroutine inside the existing binary" model. |

That is the **entire** new-dependency footprint. Resist adding anything else.

### Supporting Libraries — REUSE, do NOT add

| Capability | REUSE (already in the binary) | Do NOT add |
|------------|-------------------------------|------------|
| Wantlist storage | `modernc.org/sqlite` (pure-Go, CGO-free) + the existing `internal/backendsrv/store` seam + `goose` migration (currently at `00004`) | No ORM, no second DB |
| User identity for wantlist | Existing Discord-OAuth2 server-side sessions — `web_user.discord_user_id` (captured by AUTH-09 / LINK-02). The wantlist FK is `discord_user_id`. | No new auth |
| Website wantlist CRUD | Existing hand-rolled `net/http` + `internal/backendsrv/{readapi,webadmin}` handlers + SvelteKit `web/` | No web framework |
| PigParse polling for the EC monitor | Existing `internal/backendsrv/enrich/politefetch` (User-Agent, ETag/If-Modified-Since, backoff, Retry-After) + the in-process `scheduler` | No new HTTP client, no cron daemon |
| Daily/periodic jobs | Existing in-process `scheduler` (daily-PigParse / weekly-wiki) — add an EC-poll job and a wantlist-match job here | No `robfig/cron` etc. |
| DM rate-limit / HTTP retries | discordgo's built-in REST rate-limiter (it auto-handles 429s) | No custom limiter |
| Bot-token secret | Existing `/etc/squirebot/squirebot.env` root-only systemd `EnvironmentFile` | No Vault/SOPS/secret-manager |

### Development Tools (REUSE)

| Tool | Purpose | Notes |
|------|---------|-------|
| `goose` | One new migration for wantlist + (optional) EC-monitor cursor tables | Forward-only, same pattern as `00001`–`00004` |
| `CGO_ENABLED=0` static cross-compile | Build the linux/amd64 server ELF | discordgo adds no cgo; the existing build line is unchanged |
| systemd `Restart=always` | Already supervises `squirebot-server` | The gateway goroutine restarts with the process; discordgo also self-reconnects within the process |

---

## CRITICAL: What PigParse actually exposes (verified from `/swagger/v1/swagger.json`, 2026-06-02)

**Verdict on the EC-tunnel auction monitor: VIABLE, but pull-only and per-item.** PigParse does **not** provide a real-time auction event stream or a global "recent auctions" feed. The monitor must be built as a **polling differ**, not a subscriber.

### Full endpoint inventory (exact paths)

| Path | Method | What it returns |
|------|--------|-----------------|
| `GET /api/item/getall/{server}` | GET | All items for a server with **price averages**, rebuilt every ~10 min. Returns `AuctionItem[]` (aggregates + time-window counts, NOT individual auctions). `{server}`: `1`=Blue. |
| `GET /api/item/get/{server}/{itemname}` | GET | Averages + counts for one item (`Item`). |
| `GET /api/item/getmultiple/{server}` | GET | Bulk version of `get`. |
| `POST /api/item/postmultiple` | POST | Bulk `get` via POST body (use this to batch-query the union of all wanted item names in one call). |
| `GET /api/item/getdetails/{server}/{itemname}` | GET | **Full per-auction history for one item** — `ItemDetail` containing `ItemAuctionDetail[]`. **This is the only endpoint with individual, timestamped auction records.** |
| `GET /api/item/getdetails/{itemid}` | GET | Same, keyed by item ID. |
| `POST /api/item/auctionParse` | POST | Parses a raw in-game auction line you supply → structured `Auction`. (Input tool, not a feed.) |
| `POST /api/item/wiki` | POST | Raw wiki info passthrough. |
| `POST /api/inventory/upload` | POST | (PigParse's own ingest — not for us.) |
| `GET /api/secured/test` | GET | Auth probe. |

### The auction-record shapes (the data the EC monitor diffs on)

`ItemDetail` (from `getdetails`):
```
{ items: ItemAuctionDetail[], itemName: string, players: { <string>: string } }
```
`ItemAuctionDetail` (one auction occurrence):
```
{ u: int (0=WTS,1=WTB,2=BOTH), i: int, p: int|null (price), t: string (date-time) }
```
`players` is a map (likely auction-index/key → player name) — the only place a seller name appears; **`ItemAuctionDetail` itself has NO seller field and NO raw message.**

`Auction` (output of `auctionParse` only): `{ player, tunnelTimestamp, items: AuctionItem[] }`.

### Design consequences (carry into the EC-monitor plan)

1. **No push, no global feed.** "DM when a wanted item is auctioned" must be implemented as: scheduler job → for the **distinct set of wanted item names/IDs across all users**, call `getdetails` (or batch via `postmultiple` for the cheap aggregate pre-filter) → compare `t` timestamps against a stored "last seen auction timestamp" cursor per item → any `ItemAuctionDetail` newer than the cursor with `u`∈{WTS, BOTH} → match against each user's wantlist → DM. Then advance the cursor.
2. **Latency floor ≈ the poll interval.** `getall` rebuilds every ~10 min, so PigParse data itself is not instant; sub-10-min polling buys nothing. A 10–15 min poll is the honest design point. Document this expectation — "near-real-time," not "the moment it's auctioned."
3. **Politeness:** poll only the union of *currently-wanted* items, not the catalog. Route every call through the existing `politefetch` helper. This is the courtesy-contact trigger noted in PROJECT.md (prereq #2) if load grows.
4. **A per-item cursor table is needed** (last-seen auction `t`). New SQLite table in the same goose migration. No new dependency.
5. **Seller name is best-effort** via the `players` map; the DM can name a wanted item and price reliably, seller only when resolvable.

---

## Discord bot: gateway, intents, DM API, lifecycle

### Gateway intents needed

| Feature | Intent(s) | Privileged? |
|---------|-----------|-------------|
| Read WTS trade-channel text (regex-match wantlist) | `IntentsGuildMessages` **+ `IntentsMessageContent`** | **`MESSAGE_CONTENT` is PRIVILEGED** |
| Read raid-target announcements in those channels | same as above | same |
| Send DMs to guildies | (no intent needed — DM send is a REST call; you only need a DM channel + the user's `discord_user_id`) | n/a |

Request the minimum:
```go
dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsMessageContent
```

**Privileged-intent approval requirement (verified, Discord policy):** `MESSAGE_CONTENT` is one of Discord's three privileged intents. A bot **must enable it in the Discord Developer Portal** (Application → Bot → Privileged Gateway Intents) or the gateway connection is rejected (`disallowed intent`). For bots in **fewer than 100 servers** (this bot will be in ~3 Raid Alliance servers + the guild's own), the toggle self-serves with **no Discord review**. Verification/review is only forced once a bot crosses 100 servers and wants to stay verified. So: no Discord audit at this guild's scale — but the toggle is mandatory and the **3 Raid Alliance server admins must invite the bot with the right scope/permissions** (the "blocked on bot invites" prereq). Without `MESSAGE_CONTENT`, the bot receives empty `Content` on messages it wasn't directly mentioned in — fatal for WTS scanning.

### DM-sending API (REST, no gateway dependency)

```go
ch, err := s.UserChannelCreate(discordUserID) // opens/returns the DM channel
if err != nil { /* ... */ }
_, err = s.ChannelMessageSend(ch.ID, "WTS match: Fungi Tunic ~25kpp in EC") // or ...SendEmbed
```
- DMs are pure REST — they work even if the gateway is only ever used for reading. discordgo's built-in rate-limiter handles 429s automatically.
- **Caveat (verified, community-known):** opening many DM channels rapidly can trip Discord's abuse heuristics. Cache the resolved DM channel ID per user (reuse across pings) and fan out at a sane pace. With ~12 guildies this is a non-issue, but cache anyway.
- A guildie can only be DMed if they share a server with the bot **or** have DMs open. **The website OAuth client and the bot are distinct Discord entities** — invite the bot to the guild's OWN Discord (not only the 3 external servers) so it shares a server with every guildie and DM delivery is guaranteed.

### Lifecycle: goroutine in the existing binary (RECOMMENDED) vs separate service

**Run it in-process, as a goroutine, inside `cmd/squirebot-server`.** Rationale:

- discordgo's `Session` is **self-contained and auto-reconnecting**: `discordgo.New("Bot "+token)` auto-configures the rate limiter, state cache, and reconnect logic; `dg.Open()` starts the WebSocket; `dg.Close()` on shutdown. It owns its internal goroutines — you don't manage the socket.
- Pattern: at server startup, after the HTTP listener is wired, register `dg.AddHandler(onMessageCreate)` then call `dg.Open()` (non-blocking — it spawns the gateway loop), and `defer dg.Close()` on graceful shutdown. The `net/http` server and the gateway coexist as independent goroutine trees in one process.
- One binary, one systemd unit (`Restart=always` already covers crash recovery), one deploy, one `EnvironmentFile`, one slog stream. A separate service doubles ops surface for zero benefit at this scale.
- **The DM-send path is also reusable by the EC monitor** (which has nothing to do with the gateway) — both live in-process and share the same `*discordgo.Session`.

Only split into a separate service if the gateway's reconnect churn ever threatened ingest availability — not a real risk for a ~3-server read-only bot. Document the in-process decision as locked.

---

## Wantlist storage — confirmed: just SQLite + one goose migration

- **No new dependency.** New tables via a `goose` migration (next after `00004`), e.g.:
  - `wantlist(id, discord_user_id, item_id, item_name, kind TEXT /* 'buy'|'quest' */, created_at)` — FK `discord_user_id` → `web_user.discord_user_id`.
  - `ec_auction_cursor(item_id, item_name, last_seen_t TEXT)` — per-item poll cursor for the EC monitor.
  - (optional) `quest_target(item_id, npc_name)` — the curated quest→raid-target NPC lookup (prereq #4); a seeded table keeps it queryable (preferred over static JSON).
- CRUD via existing hand-rolled `net/http` handlers + SvelteKit form pages. Identity comes from the Discord session (`caller(ctx)`), same server-side-derived pattern locked in v2.1 (P17 D-02) — **never trust a client-supplied user id** for wantlist ownership.
- Search/match on item name uses SQLite `LIKE`/`COLLATE NOCASE` (or FTS5), consistent with the v2.0 search approach; item **ID** is the stable join key per the project's locked item-ID convention.

---

## Bot-token secret management — mirror the existing pattern

- Add `DISCORD_BOT_TOKEN=...` to **`/etc/squirebot/squirebot.env`** (root-only, `chmod 600`), the same root-only systemd `EnvironmentFile` that already holds `DISCORD_CLIENT_SECRET`. The unit's `EnvironmentFile=` already loads it; the process reads `os.Getenv("DISCORD_BOT_TOKEN")`.
- **No new secret-management dependency.** Do not commit the token, do not bake it into the binary, do not conflate it with the OAuth client secret.
- The bot token is for the **bot user**; `DISCORD_CLIENT_ID/SECRET` are for the **website OAuth login**. Recommend hosting both under the **same Discord application** (one app with both an OAuth client and a Bot user) to simplify guild-membership reasoning, but they remain distinct credentials in the env file.

---

## Installation

```bash
# Backend (the ONLY new dependency)
go get github.com/bwmarrin/discordgo@v0.29.0
go mod tidy

# Build is UNCHANGED — still CGO-free static:
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o squirebot-server ./cmd/squirebot-server

# Wantlist + cursor schema (after authoring migration 00005)
goose -dir internal/backendsrv/store/migrations sqlite3 squirebot.db up

# No frontend dependency additions — wantlist pages are plain SvelteKit on the existing app.
```

---

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| `bwmarrin/discordgo` v0.29.0 | `andersfylling/disgord` | **Never (new code).** Archived & read-only since July 2024; its auto-sharding/scaling is irrelevant for a 3-server bot. |
| `bwmarrin/discordgo` v0.29.0 | `diamondburned/arikawa`, `disgoorg/disgo` | Viable pure-Go alternatives if you wanted slash-command/interaction-first ergonomics. Not worth diverging — discordgo is the most documented, has Context7 coverage, and the project favors minimal, battle-tested deps. |
| In-process gateway goroutine | Separate systemd `squirebot-bot.service` | Only if gateway instability ever threatened ingest SLA (not a real risk at this scale). |
| PigParse `getdetails` polling differ | Parse EC `/auction` chat from a logged-in EQ client | Out of scope and against the "use the API, never log-parse for prices" decision; PROJECT.md says EC monitor is "fed by PigParse, not chat-log parsing." |
| Bot token in root-only `EnvironmentFile` | Vault / SOPS / cloud secret manager | Only at larger ops scale; pure overhead for one VPS, one guild. |
| `discordgo` bot DMs | Discord REST webhook | Webhooks can't DM users; the bot path (`UserChannelCreate`+`ChannelMessageSend`) is required for per-user DMs. |

---

## What NOT to Use / What NOT to Add

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| Any Discord/bot/OAuth code **in the watcher** (`cmd/squirebot`) | HARD CONSTRAINT (PROJECT.md, carried from v2.1). Re-introduces 7-day-expiry / public-secret / loopback fragility P13 removed. | All bot + DM + wantlist work lives in the **backend** + the guild's own Discord. |
| `andersfylling/disgord` | Archived read-only since 2024-07-11. | `bwmarrin/discordgo` v0.29.0. |
| A second binary / extra systemd service for the bot | Doubles deploy + ops surface; discordgo self-reconnects in-process. | Goroutine inside `cmd/squirebot-server`. |
| New HTTP client / cron lib for EC polling | `politefetch` + the in-process scheduler already do this, politely. | Reuse `internal/backendsrv/enrich/politefetch` + `scheduler`. |
| New DB / ORM / Postgres for wantlist | <100 MB data, locked SQLite decision (v2.0). | `modernc.org/sqlite` + goose migration. |
| Assuming a PigParse "recent auctions" push/feed exists | **It does not** (verified Swagger). Designing the EC monitor around a feed dead-ends the milestone. | Poll `getdetails` per wanted item; diff on the `t` timestamp via a cursor table. |
| Sub-10-minute EC polling | PigParse aggregates rebuild only every ~10 min; faster polling adds load for no freshness. | 10–15 min poll; set expectation to "near-real-time." |
| Running the gateway WITHOUT `MESSAGE_CONTENT` and expecting WTS text | Without the privileged intent, `MessageCreate.Content` is empty for non-mention messages. | Enable `MESSAGE_CONTENT` in the Developer Portal (free self-serve <100 servers) **before** building the WTS matcher. |
| Building WTS/raid-target before invites land | The bot won't see those guilds' messages until invited with the right perms. | Front-load wantlist + EC monitor (unblocked); gate WTS + raid-target on the 3 invites (matches PROJECT.md sequencing). |

---

## Stack Patterns by Variant

**If the 3 Raid Alliance bot invites are NOT yet secured (current state):**
- Ship wantlist (SQLite + CRUD) and the EC-tunnel monitor (PigParse `getdetails` polling differ + DM) **first** — both fully unblocked.
- The bot exists for DM-sending in the EC monitor, but the DM half needs **no privileged intent and no external-server invites** — only `discordgo.New`, `Open()`, and `UserChannelCreate`/`ChannelMessageSend`. So the DM half ships in the unblocked tranche.
- Defer the gateway **message-reading** half (and `MESSAGE_CONTENT`) until invites land.

**If/when the invites land:**
- Enable `MESSAGE_CONTENT` in the Developer Portal, add the `MessageCreate` handler with the wantlist regex matcher, and add the curated quest→NPC lookup for the raid-target monitor.

**If DM delivery to a guildie fails (DMs closed / not sharing a server):**
- Ensure the bot is invited to the guild's OWN Discord too. Fall back to logging an undeliverable-DM event.

---

## Version Compatibility

| Package | Compatible With | Notes |
|---------|-----------------|-------|
| `discordgo@v0.29.0` | Go 1.24 | Requires Go ≥ 1.18; project is on 1.24. Fine. |
| `discordgo@v0.29.0` | `CGO_ENABLED=0` static build | **Pure Go** — deps are `gorilla/websocket` (pure Go) + `golang.org/x/...` (pure Go). No `import "C"`. Cross-compiles to linux/amd64 static ELF unchanged. **CGO-free confirmed.** |
| `discordgo@v0.29.0` | `modernc.org/sqlite` | Independent; no shared transitive conflict expected. Run `go mod tidy` to confirm. |
| `MESSAGE_CONTENT` intent | Discord Gateway | Privileged; self-serve toggle for <100-server bots; mandatory to receive message text. |
| PigParse API | (no client lib) | Plain JSON over the existing `politefetch` HTTP client; `{server}=1` is Blue. |

---

## Sources

- **`/bwmarrin/discordgo`** (Context7, 289 snippets, High reputation) — gateway intents (`IntentsMessageContent`, `MakeIntent`, `IntentsAll`), `Session` lifecycle (`New`/`Open`/`Close`/auto-reconnect), `AddHandler(MessageCreate)`, `ChannelMessageSend`/`SendEmbed`/`SendComplex`, `UserChannelCreate`. — HIGH
- **`https://proxy.golang.org/github.com/bwmarrin/discordgo/@latest`** — latest version v0.29.0, 2025-05-24T11:19:24Z. — HIGH
- **`https://github.com/bwmarrin/discordgo/releases`** — v0.29.0 (2025-05-24) latest tag confirmed. — HIGH
- **`https://pigparse.azurewebsites.net/swagger/v1/swagger.json`** (fetched 2026-06-02) — full endpoint inventory + `ItemDetail`/`ItemAuctionDetail`/`Auction`/`AuctionItem`/`Item` schemas; **definitively NO global recent-auction feed; per-item timestamped history only via `getdetails`**. — HIGH
- **`https://github.com/andersfylling/disgord`** — archived read-only since 2024-07-11 (rejected alternative). — HIGH (multiple sources agree)
- Discord privileged-intent policy (`MESSAGE_CONTENT`; self-serve <100 servers, review only at verification scale) — MEDIUM (community + Discord docs consensus; not re-fetched from Discord developer docs this pass)
- discordgo CGO-free / static-binary build (`gorilla/websocket` + `golang.org/x` pure-Go deps) — HIGH (library composition is pure Go; standard CGO_ENABLED=0 cross-compile applies)

---
*Stack research for: v2.2 Wantlist + Discord pinger (additions to existing Go/SQLite/SvelteKit stack)*
*Researched: 2026-06-02*
