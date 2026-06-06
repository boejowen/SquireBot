# Phase 21: EC-Tunnel Auction Monitor - Pattern Map

**Mapped:** 2026-06-05
**Files analyzed:** 8 (1 spike artifact + 4 new + 3 modified)
**Analogs found:** 8 / 8 (every file has a strong in-repo twin; this phase is ~80% glue per RESEARCH)

This phase is a Go backend-only integration (D-06 resolved to the wiki fallback — **zero `web/` scope**). The Phase 20 spine (`notify` / `wantmatch` / `alert_log` / `bot`) is built and deployed; Phase 21 adds one producer that rides it. Every analog below is in the live, tested codebase.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `.planning/phases/21-.../21-SPIKE.md` (or inline in plan 01) | spike artifact | n/a | `RESEARCH §"The Mandatory Feasibility Spike"` + `enrich/jobs/urls.go` URL builder | n/a (gate task) |
| `internal/backendsrv/migrations/00008_ec_cursor.sql` | migration | batch | `migrations/00007_notify.sql` | exact (forward-only goose) |
| `internal/backendsrv/enrich/pigdetails.go` (NEW parser) | utility (pure parser) | transform | `enrich/pigparse.go` (`ParseToRows`) | role+flow match (DIFFERENT schema — do NOT reuse) |
| `internal/backendsrv/store/eccursor.go` (cursor get/set + poll-set query) | store | CRUD | `store/jobstate.go` (`Get/SetJobRun`) + `wantmatch/match.go` (poll-set SELECT) | exact (upsert cursor) |
| `internal/backendsrv/ec/ec.go` (NEW pkg: `RunMatch`) | service (scheduler job body) | event-driven (poll-and-diff) | `enrich/jobs/pigparse.go` (`RunPigparse`) | role+flow match |
| `internal/backendsrv/notify/dm.go` (add `SendEmbed` + `Sender.ChannelMessageSendEmbed`) | service | request-response (DM send) | `notify/dm.go` `Send` (extend in place) | exact (mirror existing) |
| `internal/backendsrv/scheduler/scheduler.go` (register `ec_auction_match`; thread `botSession`) | config (registry wiring) | event-driven | `scheduler.go` `pigparse_daily` registry entry | exact |
| `cmd/squirebot-server/main.go` (reorder `bot.Start` before `scheduler.Start`) | config (wiring) | n/a | `main.go:237-251` (current order) | exact (reorder + thread session) |
| `wantmatch/match.go` (OPTIONAL: add `note` to `Hit`/`ForItem` SELECT — D-05) | service | CRUD | `wantmatch.ForItem` (extend SELECT) | exact (additive column) |

## Pattern Assignments

### `migrations/00008_ec_cursor.sql` (migration, batch)

**Analog:** `internal/backendsrv/migrations/00007_notify.sql`

**Header + forward-only idiom** (00007:1-7, 63-65) — copy the `+goose Up` block, the "00001-00007 are SHIPPED and NOT edited" note, and the no-op `+goose Down`:
```sql
-- +goose Up
-- Phase 21 plan 21-XX (WANT-05). The EC-tunnel auction diff cursor:
-- ec_auction_cursor (per-item last-seen auction timestamp). Forward-only;
-- 00001-00007 are SHIPPED and NOT edited.

CREATE TABLE ec_auction_cursor (
  item_id     INTEGER PRIMARY KEY,           -- the stable join key (CLAUDE.md); one row per polled item
  last_seen_t TEXT NOT NULL,                 -- max(ItemAuctionDetail.t) seen, RFC3339 date-time string
  updated_at  INTEGER NOT NULL               -- unix epoch secs
);

-- +goose Down
-- Forward-only in practice (mirrors 00004/00005/00006/00007): explicit no-op.
SELECT 1;
```
Note the `migrations/migrate_test.go` is EXTENDED (not new) for the `00008` apply-idempotent assertion (RESEARCH Test Map). `ec_auction_cursor` is backend-only — **no `_meta.schema_version` bump, no `WatcherMaxSchemaVersion` change** (the watcher never reads these tables — RESEARCH §Project Constraints).

---

### `enrich/pigdetails.go` (utility, transform) — NEW parser, DO NOT reuse `ParseToRows`

**Analog:** `internal/backendsrv/enrich/pigparse.go` (same package, same purity discipline — **different JSON schema**)

**Package doc + purity + malformation-tolerance pattern** (pigparse.go:1-24): pure (no network/SQL imports), returns valid rows in source order, tolerates a small % malformed (skip+log, never log raw content — V7). Mirror the doc-comment style, INCLUDING an explicit callout of the **`t` collision** (RESEARCH Pitfall): in `getall` `t`=direction; in `getdetails` `t`=timestamp and `u`=direction.

**Struct + json-tag pattern** (pigparse.go:42-55) — define the NEW shapes from the VERIFIED Swagger (RESEARCH §"Auction record shapes"):
```go
// ItemAuctionDetail: u=direction (0=WTS,1=WTB,2=BOTH), i=itemid, p=price (NULLABLE), t=timestamp (date-time string).
type ItemAuctionDetail struct {
    U int      `json:"u"`           // direction: 0=WTS, 1=WTB, 2=BOTH (NOT the getall `t`)
    I int      `json:"i"`           // EQ item ID
    P *int     `json:"p"`           // price pp — NULLABLE; render "price unknown"/omit when nil (never 0pp)
    T string   `json:"t"`           // auction timestamp (RFC3339); the DIFF cursor key
}
type ItemDetail struct {
    Items    []ItemAuctionDetail `json:"items"`    // nullable in the API → empty slice
    ItemName string             `json:"itemName"`
    Players  map[string]string  `json:"players"`  // seller best-effort (D-05); key→auction relationship UNDOCUMENTED
}
```

**Unmarshal-into-RawMessage tolerance pattern** (pigparse.go:67-100) — copy the `json.RawMessage` per-record validate/skip loop and the `>1%` malformed → error / `≤1%` → skip+`slog.Warn("...", "skipped", n, "total", m)` discipline (`malformationTolerancePct = 0.01`, pigparse.go:24,93-99). The body here is an OBJECT (`ItemDetail`), not a top-level array — guard `json.Unmarshal` on the object, iterate `Items`.

---

### `store/eccursor.go` (store, CRUD) — cursor get/set + the poll-set read

**Analog A (cursor upsert):** `internal/backendsrv/store/jobstate.go` (`GetJobRun`/`SetJobRun`)

**Get-with-absent-is-zero pattern** (jobstate.go:36-61) — a missing row is the "never seen" signal (first-sight baseline, Pattern 3). Return `(lastT string, ok bool, err error)` with `sql.ErrNoRows → ("", false, nil)`:
```go
func (s *Store) GetECCursor(ctx context.Context, itemID int64) (lastT string, ok bool, err error) {
    var t sql.NullString
    qerr := s.db.QueryRowContext(ctx,
        `SELECT last_seen_t FROM ec_auction_cursor WHERE item_id = ?`, itemID).Scan(&t)
    switch {
    case qerr == sql.ErrNoRows:
        return "", false, nil          // never seen ⇒ first-sight baseline, do NOT DM history
    case qerr != nil:
        return "", false, fmt.Errorf("read ec_auction_cursor (item=%d): %w", itemID, qerr)
    }
    return t.String, true, nil
}
```

**Upsert-after-success pattern** (jobstate.go:87-100) — copy the `INSERT ... ON CONFLICT(item_id) DO UPDATE SET ... excluded.*` shape verbatim; one row per item, called ONLY after the item's poll succeeds (advance-only-on-success — RESEARCH Pattern 3, Pitfall 5).

**Analog B (poll-set query):** `internal/backendsrv/wantmatch/match.go:51-61` (the owner-agnostic read shape) + `migrations/00006_wantlist.sql:20-34` (the columns)

**Poll-set SELECT** (RESEARCH §"Poll-set query") — DISTINCT so a popular item polls once; D-01 reason NOT filtered; D-03 NULL item_id skipped:
```go
// SELECT DISTINCT item_id, item_name FROM wantlist_item
//  WHERE active = 1 AND item_id IS NOT NULL;
```
Use parameterized `?` placeholders + the non-nil-slice + `rows.Err()` discipline from `scanHits` (match.go:82-102).

---

### `ec/ec.go` (service, event-driven poll-and-diff) — NEW package `internal/backendsrv/ec`

**Analog:** `internal/backendsrv/enrich/jobs/pigparse.go` (`RunPigparse`) — the fetch→parse→diff→write job shape

> RESEARCH §"Recommended new files" recommends a DEDICATED `ec` package (not `enrich/jobs/`): `ec` composes `enrich` + `store` + `wantmatch` + `notify`, but `enrich` parsers must NOT depend on `notify`/`wantmatch` (import-direction). `pigdetails.go` (the pure parser) stays in `enrich`; the orchestration lives in `ec`.

**Job signature + injected fetcher** (pigparse.go:56) — `fetch politefetch.Fetcher` is injected so tests use a fake (no network). The `Fetcher` seam is `func(ctx, url string, opts Options) FetchResult` (politefetch.go:80); `FetchResult{OK, Status, Body, ETag, FromCache, Err}` (politefetch.go:60-69):
```go
func RunMatch(ctx context.Context, db *sql.DB, sender notify.Sender, fetch politefetch.Fetcher) error
```

**Fetch-failure / 304-skip discipline** (pigparse.go:70-89) — on `!res.OK` log+return (do NOT advance the per-item cursor); on `res.FromCache` (304) SKIP parse entirely. Per-item: a fetch/parse failure for one item must NOT advance THAT item's `ec_auction_cursor` (advance-only-on-success), but must not abort the whole poll loop (best-effort coverage — D-07).

**URL build (SSRF-safe, hardcoded host + escaped segment)** — mirror `enrich/jobs/urls.go:28,52-54`: a hardcoded `PigDetailsURL` constant base + `enrich.EncodeURIComponent` (wikiitem.go:115) on the item segment (NOT `url.QueryEscape` — over-escapes). RESEARCH Open Question 2 / spike pins name-form vs id-form (`getdetails/{server}/{name}` vs `getdetails/{itemid}`); RESEARCH recommends the **id form** (stable, avoids name-drift — CLAUDE.md).

**The diff + match + send core** (RESEARCH §"System data flow"):
```go
// for each new ItemAuctionDetail where t > cursor AND u ∈ {0,2} (WTS — D-02; WTB u==1 NEVER alerts):
hits, err := wantmatch.ForItem(ctx, db, int64(itemID))      // match.go:51 — call VERBATIM (active=1 AND muted=0, all users)
for _, hit := range hits {
    embed := buildEmbed(hit, detail, sellerName, itemLink)   // D-04/05 (see SendEmbed below)
    err := notify.SendEmbed(ctx, sender, db, alert, time.Now().Unix())  // routes BOTH gates + dedup + alert_log
    // branch: nil | notify.ErrCooledDown | notify.ErrDMBlocked | notify.ErrGatedOff (expected, log at debug) | other
}
// advance ec_auction_cursor[item_id] = max(t)  ONLY after this item's poll succeeded
```

**First-sight baseline (CRITICAL — Pattern 3, RESEARCH A3):** when `GetECCursor` returns `ok=false`, set the cursor to `max(t)` of the first poll **without DMing** — establishes a baseline so a standing auction is never carpet-bombed and a restart never replays history (ROADMAP criterion 4 / Pitfall 5). Verify in `ec_test.go`.

**Structured logging (V7):** log `source + item_id + want id + status` only — NEVER the DM body, item names beyond ids, or the raw `players` map (notify.go:29-30,154 precedent).

---

### `notify/dm.go` (service, request-response) — MODIFY: add embed send path

**Analog:** `notify/dm.go` `Send` itself (dm.go:108-197) — `SendEmbed` MUST reuse the SAME two-gate + dedup + `alert_log` core; do NOT duplicate or bypass it (RESEARCH Anti-Pattern).

**Extend the `Sender` interface** (dm.go:49-52) — add the one method the real `*discordgo.Session` already satisfies (`restapi.go:1812`):
```go
type Sender interface {
    UserChannelCreate(userID string, options ...discordgo.RequestOption) (*discordgo.Channel, error)
    ChannelMessageSend(channelID, content string, options ...discordgo.RequestOption) (*discordgo.Message, error)
    ChannelMessageSendEmbed(channelID string, embed *discordgo.MessageEmbed, options ...discordgo.RequestOption) (*discordgo.Message, error) // NEW
}
var _ Sender = (*discordgo.Session)(nil) // dm.go:56 — still compiles
```

**Add an `Embed` field to `Alert`** (dm.go:62-69) so the SAME `Send` path (gates dm.go:115-146, dedup dm.go:137-146, record dm.go:174-197) runs — branch the SEND step (dm.go:163) on `a.Embed != nil → s.ChannelMessageSendEmbed(ch.ID, a.Embed)` else `s.ChannelMessageSend(ch.ID, a.Body)`. The `Detail *string` (dm.go:68) is the inbox-row summary (`alert_log.detail`), e.g. `"~2000pp · seen 3m ago"`.

**Source constants already wired for EC** — pass `Source: "ec_auction"`: cooldown `cooldownEC = 22h` (dm.go:87), gate1 `monitorEnabled`→`f.EC` (dm.go:209-212), gate2 `prefAllows`→`p.EC` (dm.go:223-237). `WantID: &hit.WantID` (non-nil ⇒ dedup applies; nil is the test path only).

**discordgo embed shape** (RESEARCH §Pattern 2, `discordgo@v0.29.0/message.go:422-443`):
```go
embed := &discordgo.MessageEmbed{
    Title: "Fungi Covered Scale Tunic — WTS",       // D-05 item + WTS tag (always WTS — D-02)
    URL:   wikiUrlFor(hit.ItemName),                  // D-06 RESOLVED: P1999 wiki (web/src/lib/tooltip/composeNotes.ts:81-84 idiom)
    Fields: []*discordgo.MessageEmbedField{
        {Name: "Price",  Value: priceStr, Inline: true},     // omit field when p==nil (never 0pp)
        {Name: "Seen",   Value: "~3 min ago", Inline: true}, // from t
        {Name: "Seller", Value: seller, Inline: true},        // best-effort; OMIT field when unresolved (D-05)
        {Name: "Why you wanted it", Value: reasonOrNote},     // Hit.Reason (+ note if added — Open Q3)
    },
    Footer: &discordgo.MessageEmbedFooter{Text: "EC-tunnel auction · SquireBot"},
}
```
`notify/dm_test.go` is EXTENDED with `TestSendEmbed` cases mirroring the existing `Send` gate/dedup/50007 tests (fake `Sender` — RESEARCH Test Map).

---

### `scheduler/scheduler.go` (config, event-driven) — register the job + thread the session

**Analog:** the `pigparse_daily` / `eviction_archive` registry entries (scheduler.go:116-148)

**Registry entry pattern** (scheduler.go:117-130) — add one `*Job{Name, Due, Run}`:
```go
{
    Name: "ec_auction_match",
    Due:  func(last, now time.Time) bool { return now.Sub(last) >= 10*time.Minute }, // ~10-min cadence (A2)
    Run: func(ctx context.Context) error {
        return ec.RunMatch(ctx, db, botSession, politefetch.Fetch)
    },
},
```

**Two-cursor distinction (do NOT conflate — RESEARCH Anti-Pattern):** the job-level `job_run` cursor (advance-always, even on error — scheduler.go:229-250) is for cadence/observability; the per-item `ec_auction_cursor` is the diff state (advance-only-on-success, in `ec`). Different concerns.

**Wiring change** — `Start(ctx, db)` (scheduler.go:115) does NOT receive `*discordgo.Session`. Change the signature to `Start(ctx context.Context, db *sql.DB, botSession *discordgo.Session)` and pass it into the EC job's `Run` closure. Keep the `recover`-isolated `go run(...)` (scheduler.go:149) and the immediate-check-pass + ticker loop (scheduler.go:161-198) UNCHANGED.

---

### `cmd/squirebot-server/main.go` (config) — reorder so the session exists

**Analog:** current ordering at main.go:237-251

**The reorder (RESEARCH Pitfall: scheduler/bot wiring order, A5):** today `scheduler.Start(ctx, db)` (main.go:237) runs BEFORE `bot.Start` (main.go:244), so the scheduler can't get `botSession`. Move `bot.Start` + the `botSession := b.Session()` block (main.go:243-251) ABOVE `scheduler.Start`, then call `scheduler.Start(ctx, db, botSession)`. Both goroutines are non-blocking, ctx-tied, independent (A5 — order is currently arbitrary). `botSession` is `nil` when the bot is disabled (no token) — the EC job must no-op cleanly on a nil session (bot.go:104-107 precedent; non-fatal).

---

### `wantmatch/match.go` (service, CRUD) — OPTIONAL additive: `note` on `Hit` (D-05)

**Analog:** `ForItem` itself (match.go:51-61) + `scanHits` (match.go:82-102)

`Hit` (match.go:38-44) carries `Reason` but NOT `note`. For D-05 "why-you-wanted-it", EITHER add `Note *string` to `Hit` + `note` to the `ForItem` SELECT (`wantlist_item.note` exists — 00006:27) and the `scanHits` Scan, OR read the note separately in `ec`. RESEARCH recommends extending the Hit (cleaner). **Sizable / descopable** — the embed can ship with just `Reason` if `note` is cut (Open Q3).

## Shared Patterns

### Both-gate enforcement (officer flag + user pref) — NEVER re-implement
**Source:** `notify/dm.go:115-146` (`Send` GATE 1 `monitorEnabled`/`store.GetMonitorFlags`, GATE 2 `prefAllows`/`store.GetPrefs`)
**Apply to:** every EC send — route through `notify.SendEmbed`; the EC job adds NO gate logic of its own. The mute gate is upstream at `wantmatch.ForItem` (`muted = 0`, match.go:55).

### Dedup / cooldown — NEVER re-implement
**Source:** `notify/dm.go:137-146` (`RecentAlertExists`, `cooldownFor`) + `store/alertlog.go:180-197` (filter `send_status IN ('sent','dm_blocked')`, so a `dm_blocked` user doesn't re-accrue rows)
**Apply to:** EC inherits `cooldownEC = 22h` (dm.go:87, D-10) automatically by passing `Source: "ec_auction"` + a non-nil `WantID`. D-11 re-list past the window re-DMs (purely time-based; price NOT in the dedup key).

### Polite outbound HTTP (UA, ETag/304, backoff, 16MB cap) — NEVER hand-roll
**Source:** `enrich/politefetch/politefetch.go:80` (`Fetcher` seam), `:140` (`Fetch`), `:100-104` (body cap)
**Apply to:** the per-item `getdetails` poll (D-09 "polite in code"). Inject `politefetch.Fetch`; fake it in `ec_test.go`.

### Advance-only-on-success cursor + no-backlog-replay
**Source:** `store/jobstate.go:87-100` (upsert grain) + `scheduler/scheduler.go:213-250` (advance discipline)
**Apply to:** `ec_auction_cursor` per item — advance ONLY after a successful per-item poll; first-sight baselines (no history replay).

### SSRF-safe URL (hardcoded host + EncodeURIComponent segment)
**Source:** `enrich/jobs/urls.go:24-28,52-54` (hardcoded `PigparseURL`) + `enrich/wikiitem.go:115` (`EncodeURIComponent`)
**Apply to:** the `getdetails` URL builder in `ec`. Host/scheme are a constant; the item id/name segment is escaped.

### Structured slog, no PII (V7)
**Source:** `notify/dm.go:29-30,154` + `scheduler.go` `errDetail` (:255-260)
**Apply to:** every EC log line — `source + item_id + want id + status` only; never the DM body, raw item names, or the `players` map.

## No Analog Found

None. Every new/modified file maps to a strong in-repo twin. The two "newest" pieces — the `getdetails` parser shape and the `discordgo` embed body — have role/flow analogs (`ParseToRows`, `notify.Send`) even though their concrete schema/payload is new; both are cited above with the divergence called out.

## Metadata

**Analog search scope:** `internal/backendsrv/{scheduler,enrich,enrich/jobs,enrich/politefetch,notify,wantmatch,store,migrations}`, `cmd/squirebot-server`, `web/src/lib/tooltip` (D-06 idiom).
**Files scanned:** scheduler.go, notify/dm.go, enrich/jobs/pigparse.go, enrich/jobs/urls.go, enrich/pigparse.go, wantmatch/match.go, store/jobstate.go, store/alertlog.go, migrations/00006_wantlist.sql, migrations/00007_notify.sql, enrich/politefetch/politefetch.go, cmd/squirebot-server/main.go.
**Pattern extraction date:** 2026-06-05
