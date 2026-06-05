# Phase 20: Bot + DM + Notification Infrastructure - Pattern Map

**Mapped:** 2026-06-05
**Files analyzed:** 24 (12 backend Go, 11 frontend SvelteKit, 1 deploy-doc edit)
**Analogs found:** 23 / 24 (the only no-analog file is the `wantmatch` matcher seam)

> Phase 20 is the **alerting spine** — almost everything is "clone an existing twin and adapt." The codebase already ships the exact security/lifecycle shapes this phase needs: the in-process recover-isolated scheduler goroutine (→ bot), the owner-scoped audited `account.go`/`wantlist.go` handlers (→ `/notifications` prefs+inbox + per-want mute), the `RequireOfficer` eviction/officer forms (→ `/admin` Monitors), the `00006_wantlist.sql` extend-only goose migration + its `migrate_test.go` (→ `00007`), and the `/account` + `/wantlist` SvelteKit page/panel/cell/api stack (→ `/notifications` page + `Toggle`/`WantMuteCell`). Use these as literal templates.

---

## File Classification

### Backend (Go)

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/backendsrv/migrations/00007_*.sql` | migration | DDL | `migrations/00006_wantlist.sql` | exact |
| `internal/backendsrv/migrations/migrate_test.go` (EXTEND) | test | DDL-assert | `migrate_test.go::TestMigrate_00006_AddsWantlist` | exact |
| `internal/backendsrv/bot/bot.go` (NEW pkg) | service | event-driven (gateway goroutine) | `scheduler/scheduler.go` | role-match (recover-isolated in-proc goroutine) |
| `internal/backendsrv/notify/dm.go` (NEW pkg) | service | request-response (REST DM send) + write-audit | `webadmin/account.go` (mint+audit) + `scheduler` (slog/recover) | role-match |
| `internal/backendsrv/wantmatch/match.go` (NEW pkg) | service | transform (lookup) | — (no analog) | none |
| `internal/backendsrv/store/notifyprefs.go` (NEW) | store | CRUD (owner-scoped) | `store/wantlist.go` | exact |
| `internal/backendsrv/store/guildchannel.go` (NEW) | store | CRUD (officer) | `store/wantlist.go` (mutator shape) + `store/admins.go` | role-match |
| `internal/backendsrv/store/alertlog.go` (NEW) | store | CRUD (inbox read + mark-read + insert) | `store/wantlist.go` (ListOwn / RemoveOwnTx) | role-match |
| `internal/backendsrv/store/wantlist.go` (EXTEND: `SetMutedTx`) | store | CRUD | `store/wantlist.go::RemoveOwnWantTx` | exact |
| `internal/backendsrv/webadmin/notifications.go` (NEW) | controller | request-response (login-only CRUD) | `webadmin/account.go` + `webadmin/wantlist.go` | exact |
| `internal/backendsrv/webadmin/monitors.go` (NEW) | controller | request-response (officer CRUD + test-alert) | `webadmin/eviction.go` + `webadmin/officers.go` | exact |
| `internal/backendsrv/webadmin/wantlist.go` (EXTEND: `MuteWantHandler`) | controller | request-response | `webadmin/wantlist.go::RemoveOwnWantHandler` | exact |
| `cmd/squirebot-server/main.go` (EXTEND) | config/route | wiring | `main.go::runServe` (scheduler.Start + RequireSession/RequireOfficer blocks) | exact |
| `docs/backend-deploy.md` §7.1 (EXTEND: add `DISCORD_BOT_TOKEN`) | config | secret-plumbing | `docs/backend-deploy.md` §7.1 | exact |

### Frontend (SvelteKit)

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `web/src/routes/notifications/+page.svelte` (NEW) | route | page-shell | `routes/account/+page.svelte` / `routes/wantlist/+page.svelte` | exact |
| `web/src/lib/components/NotificationPrefsPanel.svelte` (NEW) | component | load→toggle→reload | `WatcherCodesPanel.svelte` | exact |
| `web/src/lib/components/NotificationInbox.svelte` (NEW) | component | load→list/grid→reload | `WantlistPanel.svelte` | exact |
| `web/src/lib/components/NotificationRow.svelte` (NEW) | component | presentational row | `WatcherCodesPanel` `.code-row` markup + `StatusCell` | role-match |
| `web/src/lib/components/Toggle.svelte` (NEW primitive) | component | input | `FormField.svelte` (label rhythm) + `.primary`/`.revoke-btn` button a11y | partial |
| `web/src/lib/components/MonitorAdminPanel.svelte` (NEW) | component | officer form (load→toggle/add/remove/test) | `BankCoinForm.svelte` + `WatcherCodesPanel.svelte` (revoke+ConfirmDialog) | exact |
| `web/src/lib/components/cells/WantMuteCell.svelte` (NEW cell) | component | grid-cell toggle | `cells/WantRemoveCell.svelte` | exact |
| `web/src/lib/components/StateBlock.svelte` (EXTEND: `no-notifications`) | component | empty-state | `StateBlock.svelte` (`no-wants`/`no-codes` kinds) | exact |
| `web/src/lib/components/SiteShell.svelte` (EXTEND: nav link + badge) | component | nav | `SiteShell.svelte` (`/wantlist` `.char-meta-nav` entry) | exact |
| `web/src/lib/columns.ts` (EXTEND: `WantMuteCell` column) | config | grid-column | `columns.ts::wantlistColumns` (the `remove`/`WantRemoveCell` column) | exact |
| `web/src/lib/api.ts` (EXTEND: notify wrappers) | utility | typed-fetch | `api.ts` (`/account/codes` + `/wantlist` wrapper blocks) | exact |

---

## Pattern Assignments

### `internal/backendsrv/migrations/00007_*.sql` (migration, DDL)

**Analog:** `internal/backendsrv/migrations/00006_wantlist.sql`

The Phase-20 migration is forward-only (00001–00006 are SHIPPED and NOT edited). It must: (a) CREATE `guild_channel`, (b) CREATE a per-user notify-prefs table (master + 3 per-monitor booleans), (c) `ALTER TABLE alert_log ADD COLUMN read_at`, (d) `ALTER TABLE wantlist_item ADD COLUMN muted`. **`alert_log` already exists at full shape from 00006 (`00006_wantlist.sql:39-49`) — only `read_at` is added.**

**Goose header + forward-only convention** (`00006_wantlist.sql:1-15, 59-61`):
```sql
-- +goose Up
-- Phase 20 plan 20-0X (WANT-03/04/08). ...
CREATE TABLE guild_channel ( ... );
-- +goose Down
-- Forward-only in practice (mirrors 00004/00005): explicit no-op.
SELECT 1;
```

**CHECK-as-defense-in-depth + epoch-secs + FK-to-`web_user` patterns** to copy (`00006_wantlist.sql:20-34`):
```sql
CREATE TABLE wantlist_item (
  id              INTEGER PRIMARY KEY,
  discord_user_id TEXT NOT NULL REFERENCES web_user(discord_user_id) ON DELETE CASCADE,
  ...
  reason          TEXT NOT NULL CHECK (reason IN ('buy','quest')),
  active          INTEGER NOT NULL DEFAULT 1,
  created_at      INTEGER NOT NULL               -- unix epoch secs (nowUnix())
);
CREATE UNIQUE INDEX wantlist_catalog_uidx ON wantlist_item(discord_user_id, item_id, reason) WHERE item_id IS NOT NULL AND active = 1;
```

**LANDMINE (carried from 00005, noted in `migrate_test.go:287-289`):** SQLite cannot `ALTER TABLE … ADD COLUMN` with a UNIQUE constraint. `read_at` / `muted` are plain nullable/boolean columns — fine. The notify-prefs table is a fresh `CREATE` — also fine. Do NOT add a UNIQUE column via ALTER. Recommended shapes: notify-prefs `discord_user_id TEXT PRIMARY KEY REFERENCES web_user(...)` + `master/ec/wts/raid INTEGER NOT NULL DEFAULT 1`; `guild_channel` per ARCHITECTURE `00006` block (`ARCHITECTURE-v2.2.md:324-330`): `channel_id TEXT PRIMARY KEY, guild_id TEXT, role TEXT, label TEXT, enabled INTEGER DEFAULT 1` — **D-07 note:** add a `monitor`/`role` enum incl. `ec_auction`/`wts`/`raid_target`, and a `created_at`. `wantlist_item.muted INTEGER NOT NULL DEFAULT 0`; `alert_log.read_at INTEGER` (nullable; NULL = unread, per D-05).

---

### `migrate_test.go` (test, DDL-assert) — EXTEND

**Analog:** `migrate_test.go::TestMigrate_00006_AddsWantlist` (`migrate_test.go:313-392`)

Add a `TestMigrate_00007_*` following the exact 00006 shape: assert new tables exist (`tableExists`, line 29), assert new columns via `columnSet` (line 188) on `alert_log`/`wantlist_item`, assert a default-row insert + the new-table FK holds, and the closing **idempotent re-run** (`migrations.RunMigrations(db)` returns nil — `migrate_test.go:387-391`). The helpers `tableExists` / `columnSet` / `indexExists` are already present; reuse verbatim.

```go
// pattern to clone (migrate_test.go:337-342)
wantCols := columnSet(t, db, "wantlist_item")
for _, c := range []string{"muted"} {
    if !wantCols[c] { t.Errorf("expected wantlist_item.%q after 00007 (have: %v)", c, wantCols) }
}
```

---

### `internal/backendsrv/bot/bot.go` (service, event-driven gateway goroutine) — NEW package

**Analog:** `internal/backendsrv/scheduler/scheduler.go` (`scheduler.go:104-150`, `runJob:229-250`)

The bot is the scheduler's twin: a non-blocking `Start(ctx, db, cfg)` that launches a goroutine and returns immediately, lifetime tied to `ctx` (cancelled on SIGINT/SIGTERM). **discordgo v0.29.0 is a NEW dependency — not yet in `go.mod`** (CGO-free; STACK-v2.2). The bot owns the single `*discordgo.Session` that `notify` shares.

**Non-blocking Start signature + go run(ctx)** (`scheduler.go:115, 149`):
```go
func Start(ctx context.Context, db *sql.DB) { /* build registry */ go run(ctx, db, registry) }
```

**`ConfigFromEnv` (Enabled flag + token from env)** — mirror `webauth.ConfigFromEnv` (`oauth.go:62-69`):
```go
func ConfigFromEnv() Config {
    return Config{
        Token:   os.Getenv("DISCORD_BOT_TOKEN"),
        GuildID: os.Getenv("DISCORD_GUILD_ID"), // reuse the existing guild snowflake
        Enabled: os.Getenv("DISCORD_BOT_TOKEN") != "", // off unless a token is set
    }
}
```

**recover() boundary (LOCKED — every gateway handler):** the scheduler's `runJob` is the recover-isolation precedent in grain, but the bot must add an explicit `defer func(){ if r:=recover(); r!=nil { slog.Error(...) } }()` in each MESSAGE_CREATE/event handler (ARCHITECTURE Pattern 1 / Anti-Pattern 5, `ARCHITECTURE-v2.2.md:147, 388-392`). Phase 20 builds NO message handlers (those are P22/P23), but the connect/reconnect logging + the `recover()` scaffold belong here. **Non-fatal start** (`ARCHITECTURE-v2.2.md:130-138`): a failed `Start` logs and continues; the HTTP API + scheduler must serve regardless.

**slog convention** (`scheduler.go:178`): `slog.Info("bot connected", "guild", cfg.GuildID, ...)`; never log message content/PII (V7).

---

### `internal/backendsrv/notify/dm.go` (service, REST DM + write-audit) — NEW package

**Analog:** the audit/write half of `webadmin/account.go` (the `withTx`+audit shape) + the slog discipline of `scheduler.go`.

`notify.Send(ctx, session, db, hit)` opens a DM (`UserChannelCreate`) → sends (`ChannelMessageSend`) → records EVERY attempt in `alert_log` with `send_status ∈ {sent, dm_blocked, error}` (`ARCHITECTURE-v2.2.md:188-197`). **50007 (`discordgo.ErrCodeCannotSendMessagesToThisUser` = 50007) → `dm_blocked`, NEVER silently dropped** (D-04, Pitfall 3, `ARCHITECTURE-v2.2.md:208-212`). The `alert_log` INSERT is the same parameterized-`?`, `%w`-wrapped store-func style as `store/wantlist.go::AddWantTx` (`wantlist.go:69-89`). The `*discordgo.Session` is injected from `bot` (one shared session — `ARCHITECTURE-v2.2.md:413`).

**Dedup/cooldown** (LOCKED, `ARCHITECTURE-v2.2.md:180-205`): before send, check `alert_log` for a recent `(wantlist_item_id, source, item_id)` row inside a per-source window; the dedup index `alert_log_dedup_idx` already exists (`00006_wantlist.sql:49`). Cooldown values are a per-source tunable constant (Claude's discretion: ~20–24h EC, ~1–2h WTS/raid). The **mute gate** (`wantlist_item.muted`) + **user prefs** + **officer flag** are all checked before a send fires (D-08 "both gates").

---

### `internal/backendsrv/wantmatch/match.go` (service, lookup transform) — NEW package — NO ANALOG

`wantmatch.ForItem(ctx, db, itemID)` / `ForName(ctx, db, name)` → `[]wantHit` (`ARCHITECTURE-v2.2.md:161-179`). Phase 20 builds the SEAM; consumers are P21+. Closest structural reference is `store/wantlist.go::ListOwnWants` (a parameterized owner-scoped SELECT returning a non-nil slice, `wantlist.go:96-131`) — use that as the query/scan skeleton even though the package is new. No existing matcher exists in-repo → planner falls back to the ARCHITECTURE Pattern 2 spec.

---

### `internal/backendsrv/store/{notifyprefs,guildchannel,alertlog}.go` + `wantlist.go` (store, CRUD)

**Analog:** `internal/backendsrv/store/wantlist.go` (the canonical owner-scoped store layer)

ALL new store funcs copy `wantlist.go`'s structure verbatim: owner-scoped `*sql.Tx` mutators + plain `*sql.DB` readers, `%w`-wrapped errors, parameterized `?`, non-nil slices for list readers.

**Owner-scoped IDOR-safe mutator (the load-bearing pattern)** (`wantlist.go:143-155`) — clone for `notifyprefs` upsert, `alert_log` mark-read, and `wantlist_item.muted`:
```go
func RemoveOwnWantTx(ctx context.Context, tx *sql.Tx, wantID int64, discordID string) (bool, error) {
    res, err := tx.ExecContext(ctx,
        `UPDATE wantlist_item SET active = 0 WHERE id = ? AND discord_user_id = ? AND active = 1`,
        wantID, discordID)
    ...
    return n > 0, nil // cross-owner → RowsAffected=0 → (false,nil): silent IDOR no-op
}
```
→ `SetMutedTx(ctx, tx, wantID, discordID, muted)` is line-for-line this (`UPDATE wantlist_item SET muted=? WHERE id=? AND discord_user_id=?`). `MarkAlertReadTx` is the same owner-scoped `UPDATE alert_log SET read_at=? WHERE id=? AND discord_user_id=?`; `MarkAllAlertsReadTx` drops the `id=?`. The unread-count reader is a `SELECT COUNT(*) … WHERE discord_user_id=? AND read_at IS NULL`.

**Non-nil-slice list reader** (`wantlist.go:96-131`) — clone for `ListInbox` (newest-first, `ORDER BY sent_at DESC`) and `ListGuildChannels`:
```go
out := make([]WantlistRow, 0) // non-nil → JSON []
for rows.Next() { ... out = append(out, r) }
```

**notify-prefs upsert:** an `INSERT … ON CONFLICT(discord_user_id) DO UPDATE` (D-01 all-default-ON → a row absent ⇒ treat as all-ON in the reader). `guild_channel` officer CRUD (insert/list/delete) is a non-owner-scoped variant — the caller is officer-gated at the route, so these mutators don't owner-scope (mirror `store/admins.go` officer mutators rather than the owner-scoped want ones).

---

### `internal/backendsrv/webadmin/notifications.go` (controller, login-only CRUD) — NEW

**Analog:** `internal/backendsrv/webadmin/account.go` + `internal/backendsrv/webadmin/wantlist.go`

The `/notifications` prefs + inbox handlers are the `account.go`/`wantlist.go` twin: **owner from `caller(ctx)` (session, D-02), NEVER the body**; `withTx`+`AppendAuditTx`; `writeJSON`/`writeJSONError`; method-check first; `errors.Is` typed-error mapping.

**Owner-from-session + withTx + audit** (`wantlist.go:140-156`, `account.go:48-72`):
```go
callerID := caller(r.Context()) // D-02: owner from session, body carries NO owner
now := nowUnix()
err := withTx(ctx, db, func(tx *sql.Tx) error {
    // store mutator (owner-scoped) ...
    return AppendAuditTx(ctx, tx, "<event>", callerID, map[string]any{"...": ...}, now)
})
```

**Login-only list reader returning a non-nil array** (`wantlist.go:174-190`) → `ListInboxHandler` (GET, owner-scoped) + `GetPrefsHandler` + `UnreadCountHandler`. Pref-write handlers (`POST /notifications/prefs`) decode `{master?, ec?, wts?, raid?}` (no owner field), validate, upsert; `POST /notifications/read` decodes `{id}` (the alert id only, like `removeWantReq` `wantlist.go:194-196`), `POST /notifications/read-all` has empty body. The shared helpers (`caller`/`nowUnix`/`writeJSON`/`writeJSONError`) live in `officers.go:37-61` — reuse verbatim, no new helpers.

---

### `internal/backendsrv/webadmin/monitors.go` (controller, officer CRUD + test-alert) — NEW

**Analog:** `internal/backendsrv/webadmin/eviction.go` + `internal/backendsrv/webadmin/officers.go`

The `/admin` Monitors section is the officer-form twin (`RequireOfficer` at the route + in-tx authorize-under-tx re-check). Clone `officers.go`'s add/remove handler shape (`officers.go:108-196`): method-check, decode `{...}` (no owner), `withTx` with `store.IsOfficerTx` re-check (WR-04 TOCTOU close), audit only on a real mutation.

**In-tx officer re-check (WR-04)** (`eviction.go:167-183`):
```go
err = withTx(ctx, db, func(tx *sql.Tx) error {
    okOfficer, e := store.IsOfficerTx(ctx, tx, callerID)
    if e != nil { return e }
    if !okOfficer { return store.ErrNotAuthorized } // just-removed officer → rollback
    // store mutator (toggle / add-channel / remove-channel) ...
    return AppendAuditTx(ctx, tx, "monitor_<verb>", callerID, map[string]any{...}, now)
})
```

**Handlers:** `MonitorFlagsHandler` (GET flags) + `SetMonitorFlagHandler` (POST toggle, D-07/D-08 guild-wide kill-switch), `AddGuildChannelHandler` (POST `{label, channel_id, monitor}` — server re-validates the numeric channel id + label non-blank, the `validWant`-style V5 re-check at `wantlist.go:67-85`), `ListGuildChannelsHandler` (GET), `RemoveGuildChannelHandler` (POST `{channel_id}`).

**`SendTestAlertHandler` (D-10):** the bot-pulse — DMs the CLICKING officer (`callerID` from session) a sample alert via `notify.Send`, logs it to their `alert_log` inbox, returns a status the frontend maps to the 3 toasts (success / 50007-blocked / bot-down). It needs the `*discordgo.Session` from `bot` injected into the handler closure (a new constructor arg, e.g. `SendTestAlertHandler(db, botSession)`). **Error mapping** mirrors `mapEvictionErr` (`eviction.go:315-325`): a typed sentinel for "bot offline" → a distinct JSON `{"error":"bot_unavailable"}` code, `dm_blocked` → its own code.

---

### `webadmin/wantlist.go` (controller) — EXTEND: `MuteWantHandler`

**Analog:** `webadmin/wantlist.go::RemoveOwnWantHandler` (`wantlist.go:205-239`)

The per-want mute toggle is the remove-handler twin: `POST /api/v1/wantlist/mute` decoding `{id, muted}` (the want id + new state — NEVER an owner, D-09), owner-scoped `store.SetMutedTx`, audit `wantlist_mute` with `want_id` only (V7). Same silent-no-op-on-cross-owner contract (`{muted: false}` style return). Custom wants (`item_id` NULL) are un-mutable at the UI; the handler may accept-and-store regardless (harmless) or reject — Claude's discretion.

---

### `cmd/squirebot-server/main.go` (config/route) — EXTEND

**Analog:** `cmd/squirebot-server/main.go::runServe` (`main.go:199-345`)

Two edits, both cloning existing blocks:

**1. Bot goroutine start** — insert AFTER `scheduler.Start(ctx, db)` (`main.go:234`) and BEFORE the mux/`ListenAndServe`, non-fatal (`ARCHITECTURE-v2.2.md:130-138`):
```go
scheduler.Start(ctx, db)
botCfg := bot.ConfigFromEnv()
var botSession *discordgo.Session
if botCfg.Enabled {
    s, err := bot.Start(ctx, db, botCfg)
    if err != nil { slog.Error("bot start failed; continuing without it", "err", err) } // NON-FATAL
    botSession = s
}
```

**2. New routes** — clone the existing `mux.Handle` gate-wrapped blocks (`main.go:287-324`). RequireSession for prefs/inbox/mute (the `/wantlist` block `main.go:316-321` is the exact template); RequireOfficer for monitors (the `/admin/*` block `main.go:287-294` is the template):
```go
// Notifications — LOGIN-ONLY (RequireSession), owner session-derived (D-02):
mux.Handle("GET /api/v1/notifications/prefs",  webauth.RequireSession(db, webadmin.GetPrefsHandler(db)))
mux.Handle("POST /api/v1/notifications/prefs", webauth.RequireSession(db, webadmin.SetPrefsHandler(db)))
mux.Handle("GET /api/v1/notifications/inbox",  webauth.RequireSession(db, webadmin.ListInboxHandler(db)))
mux.Handle("POST /api/v1/notifications/read",  webauth.RequireSession(db, webadmin.MarkReadHandler(db)))
mux.Handle("POST /api/v1/wantlist/mute",       webauth.RequireSession(db, webadmin.MuteWantHandler(db)))
// Monitors — OFFICER-ONLY (RequireOfficer):
mux.Handle("GET /api/v1/admin/monitors",        webauth.RequireOfficer(db, webadmin.MonitorFlagsHandler(db)))
mux.Handle("POST /api/v1/admin/monitors/flag",  webauth.RequireOfficer(db, webadmin.SetMonitorFlagHandler(db)))
mux.Handle("POST /api/v1/admin/monitors/channel", webauth.RequireOfficer(db, webadmin.AddGuildChannelHandler(db)))
mux.Handle("POST /api/v1/admin/monitors/test",  webauth.RequireOfficer(db, webadmin.SendTestAlertHandler(db, botSession)))
```
The whole mux is already CORS-wrapped (`main.go:341-344`) — new routes inherit it free. Optionally expose bot connection state on `/healthz` (Claude's discretion, PITFALLS Pitfall 7).

---

### `docs/backend-deploy.md` §7.1 (config) — EXTEND

**Analog:** `docs/backend-deploy.md:220-244`

Add `DISCORD_BOT_TOKEN=<bot-token>` to the root-only `chmod 600 /etc/squirebot/squirebot.env` heredoc (`backend-deploy.md:225-234`) alongside the existing `DISCORD_CLIENT_ID`/`DISCORD_CLIENT_SECRET`/`DISCORD_GUILD_ID`. The `EnvironmentFile=-/etc/squirebot/squirebot.env` systemd line (`backend-deploy.md:243`) already loads it — no unit change. Document the dev-portal MESSAGE_CONTENT toggle note for P22 (not required this phase, but the token + intents declaration belong here).

---

### `web/src/routes/notifications/+page.svelte` (route, page-shell) — NEW

**Analog:** `web/src/routes/account/+page.svelte` (`account/+page.svelte:1-58`) — near-identical to `wantlist/+page.svelte`

A thin route: `<svelte:head>` title + a single `.form-card` `<section>` (max-width 720px, `--panel`, 1px `--border`, 6px radius, 24px padding/gap — `account/+page.svelte:29-38`) wrapping a `<header>` (`.form-title` 20px + `.form-purpose` 16px) then the two panels. Per UI-SPEC the page stacks `NotificationPrefsPanel` (top) + a `.divider` + `NotificationInbox` (bottom). Data-driven → inherits the layout `prerender=false` (no `+page.ts`). Copy the `.form-card`/`.form-title`/`.form-purpose` `<style>` block verbatim.

---

### `web/src/lib/components/NotificationPrefsPanel.svelte` (component, load→toggle→reload) — NEW

**Analog:** `web/src/lib/components/WatcherCodesPanel.svelte` (`WatcherCodesPanel.svelte:47-229`)

Clone the `onMount → load() → phase('loading'|'error'|'ready')` lifecycle + the `route(err)` authGuard re-route (`WatcherCodesPanel.svelte:105-114, 201-212`) + the `$state` rune phase machine. Renders the master `Toggle` + 3 per-monitor `Toggle`s; each toggle writes server-side immediately then **server-truth reloads** (never optimistic — the `WantlistPanel` rule `WantlistPanel.svelte:92-103`), with a polite `aria-live` confirm line (the `.result.success`/`.result.error` lines `WatcherCodesPanel.svelte:540-550`).

**Phase machine + load + authGuard route** (`WatcherCodesPanel.svelte:84-114`):
```ts
type Phase = 'loading' | 'error' | 'ready';
let phase = $state<Phase>('loading');
async function load() { phase='loading'; try { prefs = await fetchPrefs(); phase='ready'; } catch (err) { if (route(err)) return; phase='error'; } }
```

---

### `web/src/lib/components/NotificationInbox.svelte` (component, load→list→reload) — NEW

**Analog:** `web/src/lib/components/WantlistPanel.svelte` (`WantlistPanel.svelte:1-200`)

Clone the `WantlistPanel` load→grid/list→`StateBlock` phase lifecycle (`WantlistPanel.svelte:78-90, 161-187`) and the server-truth reload on every mutation (mark-read / mark-all-read re-fetch, never optimistic — `WantlistPanel.svelte:111-136`). UI-SPEC default = a stacked `NotificationRow` list (not the grid); the grid alternative reuses `DataGrid` + a new `inboxColumns` if facet-filtering is wanted. Empty state → `StateBlock kind="no-notifications"` (`WantlistPanel.svelte:182-184` shows the `no-wants` precedent). "Mark all read" is the right-aligned accent-text action. **XSS:** alert text/source/NPC render via plain `{}` only — never `{@html}` (`WantlistPanel.svelte:13-15`).

---

### `web/src/lib/components/NotificationRow.svelte` (component, presentational row) — NEW

**Analog:** `WatcherCodesPanel` `.code-row` markup (`WatcherCodesPanel.svelte:298-314`) + `StatusCell.svelte` (delivery badge)

A single inbox row: alert text (Body) + a **delivery badge** + relative timestamp + a per-row mark-read button. The delivery badge reuses the `StatusCell` text-badge idiom (`StatusCell.svelte:27-43`): the literal WORD (`DELIVERED`/`CAN'T DM`/`ERROR`) at Label type in a `--status-*` color over an ~8%-alpha `color-mix` pill — **color is never the only signal**. A small `DeliveryBadge` wrapper maps the 3 states to tokens exactly as `StatusCell`'s `TOKEN` map does (`StatusCell.svelte:13-18`): `DELIVERED→--status-ok`, `CAN'T DM→--status-missing`, `ERROR→--status-other`. The relative-timestamp helper clones `formatLastSeen` (`WatcherCodesPanel.svelte:24-44`, the module-block DOM-free pattern for node-vitest testability). CAN'T-DM rows carry the load-bearing hint line (UI-SPEC Copywriting).

---

### `web/src/lib/components/Toggle.svelte` (component, NEW primitive)

**Analog:** none exact — compose `FormField.svelte` (label rhythm `FormField.svelte:32-38`) + the `.primary`/`.revoke-btn` button-a11y patterns (`WatcherCodesPanel.svelte:345-370, 516-538`)

The one genuinely-new primitive, and it is tiny. UI-SPEC § Toggle Vocabulary: a `<button role="switch" aria-checked={on}>` (NOT a checkbox) with a visible **ON/OFF** word (Label 13px/600) beside the track, 44px hit target, `--accent` track when ON / muted `--border` when OFF, focus ring `--accent`, honors `prefers-reduced-motion`. The button-style boilerplate (min-height 44px, font-display 13px uppercase, `:focus-visible{outline:2px solid var(--accent)}`) is liftable from `.revoke-btn` (`WatcherCodesPanel.svelte:516-538`). Deliberately generic — reused by prefs (×4), officer kill-switches (×3); the per-want mute uses a bell-glyph variant sharing the a11y contract.

---

### `web/src/lib/components/MonitorAdminPanel.svelte` (component, officer form) — NEW

**Analog:** `web/src/lib/components/BankCoinForm.svelte` (`BankCoinForm.svelte:1-60`) + `WatcherCodesPanel.svelte` (revoke + ConfirmDialog)

The `/admin` Monitors section. Clone the `BankCoinForm` officer-form lifecycle (`onMount → fetch → FormField form → save`, `BankCoinForm.svelte:42-54`) for the "add channel" form (3 `FormField`s: server label / channel ID `inputmode="numeric"` / monitor-type `<select>`), the `WatcherCodesPanel` revoke+`ConfirmDialog` pattern (`WatcherCodesPanel.svelte:159-192, 320-330`) for the destructive **remove-channel** (the ONE destructive action this phase), the `Toggle` ×3 for the kill-switches, and a `.primary` "Send me a test alert" button feeding the 3 status toasts. Mounts inside `/admin` as a third `.form-card` (see next). The officer-error routing reuses `classifyAdminError` (`api.ts:517-532`).

---

### `web/src/routes/admin/+page.svelte` (route) — EXTEND

**Analog:** `web/src/routes/admin/+page.svelte` (`admin/+page.svelte:38-49`)

Append a third `<section class="form-card"><h2 class="form-title">Monitors</h2><MonitorAdminPanel /></section>` to the existing `.admin-area` stack (alongside Evict guildie / Manage officers, `admin/+page.svelte:39-48`). The two-layer auth (Layer-1 `StateBlock kind="officers-only"` for non-officer direct-nav `admin/+page.svelte:35-37` + the server re-check) already wraps the whole `.admin-area` — the new section inherits it.

---

### `web/src/lib/components/cells/WantMuteCell.svelte` (component, grid-cell) — NEW

**Analog:** `web/src/lib/components/cells/WantRemoveCell.svelte` (`WantRemoveCell.svelte:1-49`)

The trailing "Alerts" mute bell on the `/wantlist` grid. Clone `WantRemoveCell` exactly: a `$props()` of `{ row, onMute, busy }`, an icon button (`bell`/`bell-off` Lucide glyph swap carrying state — color-not-only), the same 44px/`:focus-visible` button CSS (`WantRemoveCell.svelte:23-47`). The panel owns the actual server-truth toggle + reload (mirror `WantlistPanel`'s remove handler). Custom wants (`item_id` NULL) render a disabled `—` bell.

---

### `web/src/lib/columns.ts` (config, grid-column) — EXTEND

**Analog:** `web/src/lib/columns.ts::wantlistColumns` — the `remove` column (`columns.ts:252-260`)

Add an "Alerts" column to the `wantlistColumns` factory before `remove`, mounted via the identical `renderComponent(WantMuteCell, { row, onMute, busy })` FlexRender path the `remove` column uses (`columns.ts:252-260`). It needs the panel's `onMute` callback + `muteBusy` threaded through the factory args (`columns.ts:197-201`), exactly like `onRemove`/`removeBusy`. `enableSorting:false, enableColumnFilter:false, enableGlobalFilter:false` (the `remove` column flags).

---

### `web/src/lib/components/StateBlock.svelte` (component) — EXTEND: `no-notifications` kind

**Analog:** `StateBlock.svelte` — the `no-wants` / `no-codes` kinds (`StateBlock.svelte:109-120`)

Add `'no-notifications'` to the `StateKind` union (`StateBlock.svelte:6-29`) and a new `{:else if kind === 'no-notifications'}` block cloning the `no-wants` markup (`StateBlock.svelte:109-120`): the `CircleAlert` glyph + `.state-empty` layout + heading "No alerts yet" + the UI-SPEC body copy. No new CSS.

---

### `web/src/lib/components/SiteShell.svelte` (component) — EXTEND: nav link + unread badge

**Analog:** `SiteShell.svelte` — the `/wantlist` nav entry inside `session?.authenticated` (`SiteShell.svelte:54-66`)

Add `<a href="/notifications" class="char-meta-nav">Notifications</a>` beside the Wantlist link (`SiteShell.svelte:63`), styled by the existing `.char-meta-nav` class (`SiteShell.svelte:145-167`, 13px/600 uppercase, 0.08em tracking, 44px target). The **unread-count badge** (D-05) is the load-bearing add: a small accent-fill pill at the link's top-right showing the count, with the count in the link's `aria-label` (`aria-label="Notifications, N unread"` / `"Notifications"` when zero — UI-SPEC § Nav Badge). The count comes from a new `fetchUnreadCount()` (api.ts) refreshed on load/route-change + after mark-read (no websocket). The shell already pulls session from context (`SiteShell.svelte:32-33`) — the badge fetch hooks the authenticated branch.

---

### `web/src/lib/api.ts` (utility, typed-fetch wrappers) — EXTEND

**Analog:** `api.ts` — the `/account/codes` block (`api.ts:534-564`) + the `/wantlist` block (`api.ts:566-621`)

Add a `// --- Notifications (20-0X / WANT-03/04/08) ---` section cloning the existing wrapper idiom: `interface`s for the row contracts (snake_case mirroring the Go JSON) + one thin `getJSON`/`postJSON` wrapper per endpoint, all cookie-credentialed via the shared cores (`api.ts:154-196, 240-269`). Mirror the `fetchOwnWants`/`addWant`/`removeWant` shapes (`api.ts:595-621`):
```ts
export interface NotifyPrefs { master: boolean; ec: boolean; wts: boolean; raid: boolean; }
export interface AlertLogRow { id: number; source: string; item_name: string; status: 'sent'|'dm_blocked'|'error'; sent_at: number; read_at: number | null; }
export function fetchPrefs(f=fetch) { return getJSON<NotifyPrefs>('/api/v1/notifications/prefs', f); }
export function savePrefs(body: Partial<NotifyPrefs>, f=fetch) { return postJSON<NotifyPrefs>('/api/v1/notifications/prefs', body, f); }
export function fetchInbox(f=fetch) { return getJSON<AlertLogRow[]>('/api/v1/notifications/inbox', f); }
export function markRead(id: number, f=fetch) { return postJSON<{read:boolean}>('/api/v1/notifications/read', { id }, f); }
export function muteWant(id: number, muted: boolean, f=fetch) { return postJSON<{muted:boolean}>('/api/v1/wantlist/mute', { id, muted }, f); }
// + officer monitor wrappers (postJSON, classifyAdminError-routed) + fetchUnreadCount for the nav badge.
```
The body carries NO owner (D-02). Officer monitor wrappers route caught errors through the existing `classifyAdminError` (`api.ts:517-532`).

---

## Shared Patterns

### Owner-from-session (IDOR / D-02) — the security spine
**Source:** `webadmin/account.go:48-49`, `webadmin/wantlist.go:140`, `store/wantlist.go:143-155`
**Apply to:** EVERY login-only handler (notifications prefs/inbox/read, want-mute) + their store funcs.
```go
callerID := caller(r.Context()) // owner from SESSION, request body carries NO owner
// store: ... WHERE id=? AND discord_user_id=? ...  → cross-owner = silent no-op, never leaks existence
```

### Audited atomic write (withTx + AppendAuditTx)
**Source:** `webadmin/audit.go:57-107`
**Apply to:** every mutating handler (prefs upsert, mark-read, mute, monitor flag/channel, test-alert). One `withTx` (BEGIN IMMEDIATE) composes the store mutator + `AppendAuditTx` so the write and its audit row land atomically; detail carries ids ONLY, never PII/message text (V7).
```go
err := withTx(ctx, db, func(tx *sql.Tx) error {
    // store mutator ...
    return AppendAuditTx(ctx, tx, "<event>", callerID, map[string]any{"id": id}, now)
})
```

### Officer gate + authorize-under-tx (WR-04 TOCTOU)
**Source:** `webauth/session.go:217-237` (`RequireOfficer`) + `webadmin/eviction.go:167-174` (`store.IsOfficerTx` in-tx re-check)
**Apply to:** all `/admin` Monitors mutators. `RequireOfficer` at the route is the cheap outer gate; the mutator re-checks `store.IsOfficerTx` INSIDE the BEGIN-IMMEDIATE tx so a just-removed officer can't land a final write.

### In-process recover-isolated goroutine (non-fatal)
**Source:** `scheduler/scheduler.go:104-150` + `cmd/squirebot-server/main.go:234`
**Apply to:** `bot.Start`. Non-blocking `Start(ctx,...)` → `go run(ctx)`, lifetime tied to the SIGINT/SIGTERM-cancelled `ctx`; a failed start logs + continues (the HTTP API + scheduler must serve). Every gateway handler wraps `recover()` (LOCKED).

### EnvironmentFile secret (root-only)
**Source:** `docs/backend-deploy.md:220-244` + `webauth/oauth.go:62-69`
**Apply to:** `DISCORD_BOT_TOKEN` (bot config) — rides the existing `chmod 600 /etc/squirebot/squirebot.env`; `bot.ConfigFromEnv()` reads it like `webauth.ConfigFromEnv` reads the OAuth secrets; an empty token tolerated (bot `Enabled=false`, server still boots).

### SvelteKit load→mutate→server-truth-reload (NEVER optimistic) + authGuard route
**Source:** `WantlistPanel.svelte:78-154`, `WatcherCodesPanel.svelte:105-212`
**Apply to:** all `/notifications` + Monitors panels. `$state` phase machine (loading/error/ready), every mutation re-fetches authoritative state, a caught `Unauthenticated`/`Forbidden` routes through `authGuard` (401→Login, 403→officers-only collapse).

### Color-is-never-the-only-signal status badge
**Source:** `StatusCell.svelte:13-43`
**Apply to:** the inbox `DeliveryBadge` (DELIVERED/CAN'T DM/ERROR — word + icon + `--status-*` tinted pill) and the `Toggle` (ON/OFF word) and the mute bell (`bell`/`bell-off` glyph swap).

### XSS trust boundary (plain `{}` only)
**Source:** `WantlistPanel.svelte:13-15`, `FormField.svelte:10-13`
**Apply to:** all alert text / item names / source labels / officer-entered channel labels — render via Svelte `{}` auto-escape ONLY, never `{@html}` (the single sanctioned raw-HTML sink stays `ItemTooltip`).

---

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/backendsrv/wantmatch/match.go` | service | transform (lookup) | No matcher exists in-repo yet; Phase 20 builds the seam, consumers are P21+. Use `store/wantlist.go::ListOwnWants` as the parameterized-query/scan skeleton + ARCHITECTURE-v2.2 Pattern 2 for the contract. |

> The `discordgo` gateway/session/DM calls themselves (`bot.go`, `notify/dm.go`) have no in-repo precedent — discordgo v0.29.0 is the sole new dependency. The Go grain (recover-isolated goroutine, env config, slog, parameterized store writes) IS established; only the library surface is new. Planner should lean on STACK-v2.2 + the discordgo godoc for the session/`UserChannelCreate`/`ChannelMessageSend`/50007 specifics.

---

## Metadata

**Analog search scope:** `internal/backendsrv/{migrations,store,scheduler,webadmin,webauth}`, `cmd/squirebot-server`, `web/src/{routes,lib,lib/components,lib/components/cells}`, `docs/backend-deploy.md`, `go.mod`.
**Files scanned (read in full):** 18 source files + 3 research docs + CONTEXT/UI-SPEC.
**Confirmed new dependency:** `github.com/bwmarrin/discordgo` v0.29.0 — NOT yet in `go.mod` (verified).
**Pattern extraction date:** 2026-06-05
