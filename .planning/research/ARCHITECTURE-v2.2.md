# Architecture Research

**Domain:** Wantlist + Discord-pinger features bolted onto an existing live Go+SQLite backend (SquireBot v2.2)
**Researched:** 2026-06-02
**Confidence:** HIGH (integration points read directly from the live codebase; Discord platform constraints verified against Discord dev docs)

> **NOTE — this file was repointed for the v2.2 milestone.** The prior v1/v2.0 architecture research
> (the canonical schema doc CLAUDE.md references) is preserved verbatim at
> `.planning/research/ARCHITECTURE-v2.0-archived.md`. This document is INTEGRATION research for v2.2,
> not a greenfield redesign. The existing v2.0/v2.1 architecture (single static `cmd/squirebot-server`
> binary, `internal/backendsrv/{store,enrich,ingest,readapi,webadmin,webauth,scheduler}`, SvelteKit
> `web/`, SQLite via modernc + goose) is taken as fixed. Every recommendation below either ADDS a
> package/table/route or EXTENDS an existing seam. New vs. modified is called out explicitly throughout.

---

## Standard Architecture

### System Overview — where v2.2 plugs in

```
┌──────────────────────────────────────────────────────────────────────────┐
│                    cmd/squirebot-server  (ONE static binary, systemd)      │
│                                                                            │
│  runServe(): goose.Up → scheduler.Start(ctx,db) → bot.Start(ctx,db) [NEW]  │
│              → http.ListenAndServe(mux)                                     │
│                                                                            │
│  ┌─────────────────────────┐   ┌──────────────────────────────────────┐   │
│  │ HTTP mux (net/http)      │   │ scheduler (in-proc time.Ticker)      │   │
│  │  existing read/write +   │   │  pigparse_daily · wiki_weekly ·      │   │
│  │  NEW wantlist CRUD routes │   │  eviction_archive                    │   │
│  │  (RequireSession-gated)   │   │  + NEW ec_auction_match  [NEW job]   │   │
│  └────────────┬─────────────┘   └───────────────┬──────────────────────┘   │
│               │                                 │                          │
│  ┌────────────┴─────────────────────────────────┴──────────────────────┐  │
│  │           internal/backendsrv/store  (SQLite, SetMaxOpenConns(1))     │  │
│  │  EXISTING: owner, character, inventory_item, item_master,            │  │
│  │            pigparse_price, web_user, web_session, audit_log …        │  │
│  │  NEW:      wantlist_item · alert_log · quest_target [+ guild_channel] │  │
│  └────────────┬─────────────────────────────────────┬──────────────────┘  │
│               │ match logic (NEW internal/backendsrv/wantmatch)            │
│               │                                       │                     │
│  ┌────────────┴───────────────┐         ┌─────────────┴─────────────────┐  │
│  │  bot  [NEW package]         │         │  notify  [NEW: DM sender]      │  │
│  │  discordgo gateway conn     │────────▶│  opens DM channel + sends      │  │
│  │  (always-on goroutine):     │         │  (shared-guild required)       │  │
│  │   • MESSAGE_CREATE handler  │         └────────────────────────────────┘  │
│  │     on WTS channels [GATED] │                                            │
│  │   • raid-target detector    │         ↑ all three alert sources funnel   │
│  │     [GATED]                 │           through wantmatch → alert_log →   │
│  └─────────────────────────────┘           notify.Send                      │
└──────────────────────────────────────────────────────────────────────────┘
        │ apex (Caddy)                              ▲ Discord Gateway (WSS, outbound)
        ▼                                           │  + Discord REST (DM send)
┌──────────────────────────┐          ┌────────────────────────────────────────┐
│  web/  SvelteKit static   │          │  Discord: guild's OWN server (DM-capable │
│  NEW /wantlist route       │          │  for all guildies) + 3 Raid Alliance    │
│  (RequireSession behind    │          │  servers (WTS channels — INVITE-GATED)  │
│  Discord login)            │          └────────────────────────────────────────┘
└──────────────────────────┘
```

### Component Responsibilities

| Component | New/Mod | Responsibility | Implementation |
|-----------|---------|----------------|----------------|
| `store` (wantlist tables) | **NEW DDL** in `00006_wantlist.sql` | Own `wantlist_item`, `alert_log`, `quest_target`, `guild_channel` | goose migration + store funcs (mirror `account.go` Tx style) |
| `webadmin/wantlist.go` | **NEW** | Wantlist CRUD HTTP handlers (login-only) | Same shape as `account.go` — `caller(ctx)`, `withTx`, `writeJSON` |
| `web/src/routes/wantlist/` | **NEW** | Wantlist page (list / add-via-item-search / remove) | SvelteKit page behind `RequireSession`; reuses item-catalog search |
| `wantmatch` | **NEW pkg** | The ONE match function: given an item id/name, return wantlisters to DM | Pure-ish lookup over `wantlist_item` + dedup check vs `alert_log` |
| `notify` (DM sender) | **NEW pkg** | Open DM channel for a `discord_user_id` + send message; record in `alert_log` | discordgo `UserChannelCreate` + `ChannelMessageSend` |
| `bot` | **NEW pkg** | Long-lived gateway connection; reconnect loop; MESSAGE_CREATE handler; raid-target detector | discordgo `Session.Open()`, run as a goroutine off `runServe` |
| scheduler `ec_auction_match` | **NEW job** in existing registry | After the daily PigParse pull, diff new EC auction signal → `wantmatch` → DM | One more `*Job{}` entry in `scheduler.Start` |
| `quest_target` lookup | **NEW data** (curated) | `quest → raid-target NPC(s)` mapping for the raid monitor | Seeded table; populated via a `run-job`-style CLI or migration seed |

---

## Recommended Project Structure

```
internal/backendsrv/
├── store/
│   └── (existing) + NEW wantlist.go, alertlog.go, questtarget.go   # store funcs
├── migrations/
│   └── 00006_wantlist.sql           # NEW: wantlist_item, alert_log, quest_target, guild_channel
├── webadmin/
│   └── wantlist.go                  # NEW: CRUD handlers (login-only, account.go twin)
├── wantmatch/                       # NEW pkg: the single match seam (all 3 sources call it)
│   └── match.go
├── notify/                          # NEW pkg: DM delivery + alert_log recording
│   └── dm.go
├── bot/                             # NEW pkg: discordgo gateway connection + handlers
│   ├── bot.go                       #   Start(ctx, db, deps) → goroutine, reconnect-managed
│   ├── wts.go                       #   MESSAGE_CREATE → regex → wantmatch (INVITE-GATED feature)
│   └── raidtarget.go                #   MESSAGE_CREATE → quest_target match (INVITE-GATED)
└── scheduler/
    └── scheduler.go                 # MODIFIED: add ec_auction_match job to the registry

web/src/routes/
└── wantlist/                        # NEW: +page.svelte (+ +page.ts loader)
```

### Structure Rationale

- **`wantmatch/` as its own package, not buried in a handler:** all THREE alert sources (EC poll, WTS
  message, raid-target message) run the identical "does anyone want this item?" lookup and the identical
  dedup. Centralizing it means the unblocked EC path and the gated WTS/raid paths share one tested code
  path; the gated work becomes "wire a new event source into an existing matcher."
- **`notify/` (DM sender) separate from `bot/`:** the scheduler's `ec_auction_match` job must send DMs
  WITHOUT being a gateway-message handler. Both the scheduler job and the bot's message handlers call
  `notify.Send`. The discordgo `*Session` (needed to send a DM) is owned by `bot` and injected into
  `notify` — see the bot-hosting decision for why one shared session is correct.
- **`webadmin/wantlist.go` next to `account.go`:** wantlist CRUD is the exact same security shape as the
  v2.1 self-service code endpoints — login-only, identity derived from the session (`caller(ctx)`),
  `withTx` + `AppendAuditTx`, owner-scoped reads/writes. Copying that file's structure inherits the
  IDOR-safe, audited pattern proven in P17.

---

## Architectural Patterns

### Pattern 1: The Discord bot is an in-binary goroutine, NOT a separate service

**Decision (RECOMMENDED):** Run the always-on Discord gateway connection as a goroutine inside the
existing `cmd/squirebot-server` binary, started from `runServe` right after `scheduler.Start(ctx, db)`
and before `srv.ListenAndServe()`. It shares the same `*sql.DB`, the same env-loaded config, the same
systemd unit, the same `ctx` lifecycle, and the same slog pipeline.

```go
// in runServe(), after scheduler.Start(ctx, db):
botCfg := bot.ConfigFromEnv()            // DISCORD_BOT_TOKEN, channel ids, etc.
if botCfg.Enabled {                      // feature flag — bot off until invites land
    if err := bot.Start(ctx, db, botCfg); err != nil {
        slog.Error("bot start failed; continuing without it", "err", err)
        // NON-FATAL: the HTTP API + scheduler must serve even if the bot can't connect.
    }
}
```

**Why in-binary (the trade-off analysis):**

| Dimension | In-binary goroutine (CHOSEN) | Separate process/service |
|-----------|------------------------------|--------------------------|
| Deploy | Already solved — one binary, one `systemd Restart=always`, one R2 backup, one Caddy. Drop-binary-and-restart deploy stays intact. | Second unit, second deploy step, second log stream, second thing to forget to restart. |
| SQLite access | Direct, same `*sql.DB` (`SetMaxOpenConns(1)` already serializes). No IPC. | Two processes on one SQLite file = the classic `database is locked` writer-contention trap; forces WAL + retry policy or an HTTP shim between them. **Real risk at this seam.** |
| Config/secrets | One `/etc/squirebot/squirebot.env` EnvironmentFile already read by systemd; add `DISCORD_BOT_TOKEN`. | Duplicate the secret plumbing. |
| Failure isolation | A panic in a bot handler could take the HTTP server down — **mitigated** by `recover()` in every gateway handler + non-fatal start + discordgo's reconnect goroutines being isolated from the HTTP `ListenAndServe` goroutine. | True crash isolation — the API survives a bot OOM/panic independently. This is the ONLY real advantage. |
| Reconnect loop coexistence | discordgo runs its websocket read/heartbeat/reconnect on ITS OWN goroutines; it does not block the HTTP listener (which is on the main goroutine via the existing `serveErr` channel + `ctx.Done()` select in `runServe`). They coexist cleanly. | n/a |
| Scale fit | ~12 guildies, ~4 servers, low message volume. One process is abundant. | Over-engineering for this scale. |

**Verdict:** failure isolation is the lone point for separation, and it is cheaply bought back with
per-handler `recover()` + a non-fatal bot-start + the existing `Restart=always`. Everything else —
especially the **two-processes-one-SQLite-file** hazard — points hard at in-binary. This also honors the
project's whole architectural grain (single static binary; the in-process scheduler already lives in the
same binary and is the exact precedent). **Run the bot as a goroutine in `squirebot-server`.**
(Confidence: HIGH.)

**When you'd revisit:** if the bot ever needed to be in 100+ servers (privileged-intent verification +
sharding) or if message volume grew enough to starve the SQLite writer — neither is true for one guild.

### Pattern 2: One match seam, three event sources (fan-in)

**What:** Every alert path converges on `wantmatch.ForItem(ctx, db, itemID)` / `wantmatch.ForName(ctx,
db, name)` → `[]wantHit`, then `notify.Send(...)` per hit, then an `alert_log` insert. The three sources
differ only in how they discover "an item showed up":

```
EC auction (PigParse poll, scheduler) ─┐
WTS channel msg (bot, gated)           ─┼─▶ wantmatch ─▶ dedup vs alert_log ─▶ notify.Send ─▶ alert_log INSERT
raid-target msg (bot, gated)           ─┘
```

**When to use:** always — it is what makes the unblocked path (EC) and the gated paths (WTS, raid) share
one tested matcher and one dedup policy. Build EC first; WTS/raid become "new caller, same core."

**Trade-offs:** the matcher needs two entry points — an exact `item_id` (EC/PigParse has stable ids) and
a fuzzy `item_name` (chat text has only names → match against `item_master.name` / `wantlist_item.
item_name`). Keep both; the name path is the brittle one (regex + normalization), so isolate it.

### Pattern 3: Dedup + delivery audit via `alert_log`

**What:** Before sending, check `alert_log` for a recent matching `(wantlist_item_id, source, item_id)`
row inside a dedup window; after a successful send, insert one. This stops the daily PigParse poll from
re-DMing the same standing want every day, and stops a repeated WTS line from spamming.

```sql
-- NEW in 00006_wantlist.sql
CREATE TABLE alert_log (
  id               INTEGER PRIMARY KEY,
  wantlist_item_id INTEGER NOT NULL REFERENCES wantlist_item(id) ON DELETE CASCADE,
  discord_user_id  TEXT NOT NULL,         -- recipient (denormalized for the DM + querying)
  source           TEXT NOT NULL,         -- 'ec_auction' | 'wts' | 'raid_target'
  item_id          INTEGER,               -- nullable (chat-name matches may lack an id)
  detail           TEXT,                  -- JSON: price / channel / message snippet (log-safe)
  sent_at          INTEGER NOT NULL,      -- unix epoch secs (web_user/audit_log convention)
  send_status      TEXT NOT NULL          -- 'sent' | 'dm_blocked' | 'error'
);
CREATE INDEX alert_log_dedup_idx ON alert_log(wantlist_item_id, source, item_id, sent_at);
```

**Dedup policy (recommend; finalize in REQUIREMENTS):** per-source window. EC: suppress if a `sent` row
for that `(wantlist_item_id, source)` exists within ~20–24h (daily poll cadence). WTS/raid: shorter
window (1–2h) since chat is bursty. Make the window a per-source constant so it is one-line tunable.
`send_status` records `dm_blocked` (user shares no guild / DMs off) so a guildie who never gets pings is
diagnosable rather than silently dropped.

### Pattern 4: DM delivery requires a shared guild — the guild's OWN server IS the delivery vehicle

**Platform constraint (verified):** a Discord bot can only DM a user with whom it shares a guild, and only
if that user permits DMs from server members. The bot opens a DM channel via REST
(`UserChannelCreate` / `POST /users/@me/channels`) then sends to it.

**Architectural consequence — and the key unblocking insight:** the bot must be a member of the **guild's
own Discord server** (the same server whose membership gates `web_user` login, AUTH-08). Since every
guildie is in that server (that membership IS the OAuth gate), the bot shares a guild with **every alert
recipient** via the guild's own server. **Therefore DM delivery for ALL three alert types is UNBLOCKED**
and does not depend on the 3 Raid Alliance invites. The invites are needed only to *read* the external
WTS/raid-announce channels — i.e. for the alert *source*, never for the alert *delivery*. This is what
cleanly splits the two build tracks (see Build Order).

### Pattern 5: MESSAGE_CONTENT intent — privileged but free at this scale

**Verified:** `MESSAGE_CONTENT` is a privileged gateway intent, but Discord's verification/audit
requirement only applies to bots in **100+ servers**. This bot lives in ~4 servers (guild's own + 3 Raid
Alliance). So the intent is enabled with a single toggle in the Developer Portal — **no Discord
app-review audit, no verification gate** (a welcome contrast to the Google brand-verification trauma that
birthed v2.0). Declare `IntentsGuilds | IntentsGuildMessages | IntentsMessageContent |
IntentsDirectMessages` in the discordgo identify. This is required for `wts.go`/`raidtarget.go` to read
message text.

---

## Data Flow

### Flow A — EC auction monitor (UNBLOCKED path)

```
scheduler tick → ec_auction_match.Due? → after/with pigparse_daily pull
   PigParse getall/1 (existing politefetch) → detect newly-auctioned items
   for each item_id:
       wantmatch.ForItem(db, item_id) → []{wantlist_item_id, discord_user_id}
           per hit: alert_log dedup check (window) → skip if recently sent
                    notify.Send(session, discord_user_id, msg) → DM
                    alert_log INSERT (sent | dm_blocked | error)
```

**Open question for STACK/FEATURES research, NOT architecture:** does PigParse expose *auction events* or
only daily aggregate prices? PROJECT.md flags this. If PigParse is daily-prices-only, the "EC auction
monitor" degrades to "DM when a wanted item newly appears in / moves in the daily price feed" — still
useful, still this exact data flow, just a coarser trigger. The architecture is unaffected; only the
trigger definition inside `ec_auction_match` changes.

### Flow B — WTS channel monitor (INVITE-GATED path)

```
Discord Gateway MESSAGE_CREATE (in a watched WTS channel) → bot/wts.go handler
   recover() guard → is this a watched WTS channel? (guild_channel lookup)
   parse the WTS line → candidate item name(s) (regex/normalization)
   wantmatch.ForName(db, name) → []hits   (name path — fuzzy, item_master-backed)
       per hit: alert_log dedup → notify.Send → alert_log INSERT
```

### Flow C — Quest-target raid monitor (INVITE-GATED path)

```
Discord Gateway MESSAGE_CREATE (raid-announce channel) → bot/raidtarget.go
   detect a raid-target NPC name in the message (quest_target lookup, curated)
   quest_target → quest_name → the EXISTING quest_items table → the quest's wanted item_id(s)
   wantmatch.ForItem(db, item_id) for wantlisters who want that quest's item
       per hit: alert_log dedup → notify.Send → alert_log INSERT
```

Flow C reuses the EXISTING `quest_items` dimension table (item_id ↔ quest_name) and only ADDS the inverse
`quest_target` (quest_name → raid NPC) lookup. The chain is NPC → quest → item → wantlister.

### Wantlist CRUD flow (UNBLOCKED)

```
web /wantlist (SvelteKit) → fetch GET /api/v1/wantlist (RequireSession)
   add: user searches the EXISTING item catalog (reuse the item data the views/tooltips already load)
        → POST /api/v1/wantlist {item_id, reason:'buy'|'quest', priority}  (identity from session)
   remove: POST /api/v1/wantlist/remove {id}   (owner-scoped, IDOR-safe like account.go)
```

### State / identity

The wantlist is tied to the **Discord identity** (`web_user.discord_user_id`), NOT to `owner` (the
watcher-token concept). A guildie's wants are theirs as a person; the DM target is their
`discord_user_id`. So `wantlist_item.discord_user_id TEXT REFERENCES web_user(discord_user_id)`. This
also means the wantlist works for any logged-in member even before they have linked a watcher.

---

## Suggested Schema (NEW — `00006_wantlist.sql`)

```sql
-- +goose Up  (forward-only; 00001..00005 shipped and NOT edited — convention)

CREATE TABLE wantlist_item (
  id              INTEGER PRIMARY KEY,
  discord_user_id TEXT NOT NULL REFERENCES web_user(discord_user_id) ON DELETE CASCADE,
  item_id         INTEGER,                 -- canonical EQ item id (joins item_master; nullable for free-text wants)
  item_name       TEXT NOT NULL,           -- snapshot of the name at add-time (chat matching + display)
  reason          TEXT NOT NULL,           -- 'buy' | 'quest'
  priority        INTEGER NOT NULL DEFAULT 0,
  active          INTEGER NOT NULL DEFAULT 1,  -- soft-delete keeps alert_log FKs intact
  created_at      INTEGER NOT NULL          -- unix epoch secs (web convention)
);
CREATE INDEX wantlist_user_idx ON wantlist_item(discord_user_id);
CREATE INDEX wantlist_item_idx ON wantlist_item(item_id);

-- alert_log: see Pattern 3 (dedup + delivery audit).

CREATE TABLE quest_target (
  id             INTEGER PRIMARY KEY,
  quest_name     TEXT NOT NULL,
  npc_name       TEXT NOT NULL,            -- raid-target NPC announced in chat
  normalized_npc TEXT NOT NULL,            -- lower(trim) for chat matching (mirror spellbook normalized_name)
  source         TEXT,
  UNIQUE(quest_name, npc_name)
);
CREATE INDEX quest_target_npc_idx ON quest_target(normalized_npc);

CREATE TABLE guild_channel (
  channel_id  TEXT PRIMARY KEY,            -- Discord snowflake
  guild_id    TEXT NOT NULL,
  role        TEXT NOT NULL,               -- 'wts' | 'raid_announce'
  label       TEXT,
  enabled     INTEGER NOT NULL DEFAULT 1
);
```

Convention checks honored: epoch-secs for web-side timestamps (matches `web_user`/`audit_log` `at`);
`normalized_*` mirrors `spellbook_entry`; soft-delete (`active`) so `alert_log` FKs survive a removed
want; snowflakes as `TEXT` (matches `web_user.discord_user_id`); forward-only goose with 00001..00005
untouched; the partial-unique-index landmine from 00005 does not recur (no UNIQUE columns added via
ALTER here — all new tables are fresh CREATEs).

---

## Scaling Considerations

| Scale | Architecture Adjustments |
|-------|--------------------------|
| This guild (~12 users, ~4 servers, low msg volume) | Everything in one binary. One discordgo session, no sharding, single SQLite writer. Abundant. |
| Hypothetical 100+ Discord servers | MESSAGE_CONTENT would need Discord verification + the bot would need sharding — at which point a separate bot process becomes worth it. NOT a v2.2 concern. |
| Message burst (busy WTS channel) | The matcher does a small indexed SQLite read per message; `recover()` + the per-source dedup window keep it cheap. If it ever mattered, debounce/queue MESSAGE_CREATE — premature now. |

### Scaling priorities

1. **First thing that could break:** two processes on one SQLite file (`database is locked`). The
   in-binary decision *avoids* this entirely — do not split the bot out.
2. **Second:** DM rate limits if many wants match one item at once (e.g. a popular raid target).
   discordgo handles REST rate-limit backoff; the per-recipient `alert_log` dedup keeps volume sane.

---

## Anti-Patterns

### Anti-Pattern 1: A second binary/service for the bot sharing the SQLite file

**What people do:** ship a `cmd/squirebot-bot` that opens the same `squirebot.db`.
**Why it's wrong:** two writers on one SQLite file → lock contention; doubles deploy/ops; fights the
project's single-static-binary grain. **Do this instead:** in-binary goroutine (Pattern 1).

### Anti-Pattern 2: Putting the Discord bot or its token in the watcher

**What people do:** reuse the watcher process to host Discord logic.
**Why it's wrong:** violates the HARD CONSTRAINT carried from v2.1 — the watcher is deliberately
browser-free / Google-free / OAuth-free; a gateway connection + bot token there re-introduces exactly the
fragility v2.0/v2.1 removed. **Do this instead:** all bot/DM work is backend-only; the watcher is
untouched in v2.2.

### Anti-Pattern 3: DMing on every match with no dedup

**What people do:** poll PigParse daily, DM every standing want every day.
**Why it's wrong:** a wanted-but-unobtainable item nags forever; users mute the bot. **Do this instead:**
`alert_log` dedup window (Pattern 3).

### Anti-Pattern 4: Hardcoding watched channel ids / NPC lists in Go

**What people do:** bake the 3 Raid Alliance WTS channel ids + quest→NPC map into source.
**Why it's wrong:** invites/channels change; a redeploy to edit a list is friction. **Do this instead:**
`guild_channel` + `quest_target` tables (seedable like the other dimension tables via a `run-job`-style
path).

### Anti-Pattern 5: Blocking the HTTP listener (or letting a bot panic kill it)

**What people do:** start the bot synchronously / let an unhandled handler panic propagate.
**Why it's wrong:** the read/write API is the product's core; the bot is an add-on. **Do this instead:**
bot start is non-fatal, runs on its own goroutines (discordgo manages them), and every event handler is
wrapped in `recover()`.

---

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| Discord Gateway (WSS) | `discordgo.Session.Open()` long-lived; library owns reconnect/heartbeat goroutines | Intents: Guilds + GuildMessages + MessageContent + DirectMessages. MESSAGE_CONTENT privileged but **toggle-only** under 100 servers (no audit). |
| Discord REST (DM) | `UserChannelCreate` then `ChannelMessageSend` | **Bot must share a guild with the recipient** → the bot lives in the guild's OWN server → all guildies reachable. discordgo handles REST rate-limit backoff. |
| PigParse REST | EXISTING `politefetch` + `jobs.RunPigparse`; the new `ec_auction_match` reads the freshly-upserted `pigparse_price` (or a new auction signal) | No new external client — reuse the existing polite-fetch + scheduler seam. |
| P1999 wiki | EXISTING weekly job; v2.2 may extend it to seed `quest_target` if NPCs are wiki-derivable | Otherwise `quest_target` is hand-curated (a stated prerequisite). |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| `scheduler.ec_auction_match` → `wantmatch` → `notify` | direct Go calls, same process | The unblocked DM path; needs the discordgo `*Session` injected from `bot`. |
| `bot` (wts/raidtarget handlers) → `wantmatch` → `notify` | direct Go calls | The gated DM path; same matcher + sender as the scheduler job. |
| `bot` ↔ `notify` (shared `*Session`) | `bot` owns the session, passes a send func / the session into `notify` | One session for both gateway events and outbound DMs (do not open a second). |
| `webadmin/wantlist.go` → `store` | `withTx` + store funcs (account.go pattern) | Login-only; identity from session; audited via `AppendAuditTx`. |
| `web/wantlist` → API | credentialed `fetch` (existing CORS + cookie) | Reuses the item catalog the views/tooltips already load for add-item search. |

---

## Build Order — UNBLOCKED first, INVITE-GATED last

Phases continue from 18. The split is enforced by Pattern 4's insight: **DM delivery is unblocked; only
WTS/raid *reading* needs the invites.** So the entire DM infrastructure + the EC monitor + the wantlist
can ship and deliver value on the guild's own Discord with zero dependency on the 3 Raid Alliance invites.

**Track 1 — UNBLOCKED (ships without any external invite):**

- **Phase 19 — Wantlist CRUD (data + API + page).** `00006_wantlist.sql` (at least `wantlist_item`,
  `alert_log`); `webadmin/wantlist.go` (account.go twin); `web/src/routes/wantlist/` reusing the item
  catalog for add-item search. *Pure web feature; delivers immediately; no Discord at all yet.* → WANT-01.
- **Phase 20 — DM infrastructure + the bot's gateway connection (guild's own server only).** `bot`
  package: discordgo session, in-binary goroutine, non-fatal start, reconnect, `recover()` guards, env
  config (`DISCORD_BOT_TOKEN`). `notify` (DM open+send) + `wantmatch` (the shared matcher) + `alert_log`
  dedup. Prove end-to-end with a manual/test trigger DMing a guildie. *This builds the spine all alerts
  ride.* → WANT-05 infra.
- **Phase 21 — EC auction monitor.** `scheduler.ec_auction_match` job wired to PigParse (depends on the
  STACK/FEATURES finding re: auction-events vs daily-prices) → `wantmatch` → `notify`. First real
  end-to-end alert, all on the guild's own Discord. → WANT-02.

**Track 2 — INVITE-GATED (blocked on the 3 Raid Alliance bot invites + curated lookups):**

- **Phase 22 — WTS cross-server monitor.** Invite the bot to the 3 servers; `guild_channel` rows for
  their WTS channels; `bot/wts.go` MESSAGE_CREATE → name parse → `wantmatch.ForName` → existing
  `notify`/`alert_log`. *Only adds a new event source to the spine built in Phase 20.* → WANT-03.
- **Phase 23 — Quest-target raid monitor.** Seed `quest_target` (curated quest→NPC); `bot/raidtarget.go`
  MESSAGE_CREATE → NPC detect → `quest_target` → existing `quest_items` (item) → `wantmatch.ForItem` →
  `notify`/`alert_log`. *Last because it needs BOTH the invites AND the curated NPC lookup.* → WANT-04.

**Dependency rationale:** 19 → 20 (CRUD before there's anything to match) → 21 (first matcher consumer).
22 and 23 depend on 20's spine but are otherwise independent of each other and gated purely on external
prerequisites; if the invites land early they can move up, if they never land Track 1 still delivers a
complete, valuable feature (wantlist + EC pings) on its own. The bot's `Enabled` feature flag means the
binary can ship Track 1 with WTS/raid watching dark, then flip on per-server channel watching as invites
arrive — no rebuild needed, just `guild_channel` rows + the flag.

---

## Sources

- Existing codebase (HIGH — read directly): `cmd/squirebot-server/main.go`, `internal/backendsrv/scheduler/scheduler.go`, `internal/backendsrv/webadmin/account.go`, `internal/backendsrv/webauth/session.go`, `internal/backendsrv/migrations/00001_init.sql` / `00003` / `00004` / `00005`, `internal/backendsrv/enrich/jobs/pigparse.go`, `go.mod`, `web/src/routes/` tree.
- `.planning/PROJECT.md` — v2.2 milestone, System shape (v2.0), Key Decisions (HIGH)
- [Discord: Message Content Privileged Intent FAQ](https://support-dev.discord.com/hc/en-us/articles/4404772028055-Message-Content-Privileged-Intent-FAQ) — privileged but audit only at 100+ servers (HIGH)
- [Discord: Why isn't my DM going through?](https://support.discord.com/hc/en-us/articles/360060145013-Why-isn-t-my-DM-going-through) — bot must share a guild with the recipient (HIGH)
- [discordgo (bwmarrin/discordgo) — pkg.go.dev](https://pkg.go.dev/github.com/bwmarrin/discordgo) — minimalist single-session gateway wrapper, intents supported v0.21.0+ (MEDIUM; library choice finalized in STACK.md)
- [disgo (disgoorg/disgo)](https://github.com/disgoorg/disgo) — higher-level alternative considered (MEDIUM)

---
*Architecture research for: SquireBot v2.2 Wantlist + Discord Pinger (integration with existing Go+SQLite backend)*
*Researched: 2026-06-02 · prior v2.0 architecture preserved at ARCHITECTURE-v2.0-archived.md*
