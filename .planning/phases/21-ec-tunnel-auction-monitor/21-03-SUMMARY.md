---
phase: 21-ec-tunnel-auction-monitor
plan: 03
subsystem: ec-producer
tags: [ec-tunnel, discordgo, embed, scheduler, wantmatch, notify, poll-and-diff, WANT-05]

# Dependency graph
requires:
  - phase: 21-ec-tunnel-auction-monitor
    plan: 01
    provides: "enrich.ParseItemDetail + ItemAuctionDetail{U,I,P,T}; migration 00008 ec_auction_cursor; store.GetECCursor/SetECCursor/ECPollSet; the SPIKE (server=0, NAME key form)"
  - phase: 21-ec-tunnel-auction-monitor
    plan: 02
    provides: "notify.Alert.Embed + notify.Sender.ChannelMessageSendEmbed routed through the SAME Send gate/dedup/alert_log core; wantmatch.Hit.Note"
  - phase: 20-bot-dm-notification-infrastructure
    provides: "notify.Send two-gate + cooldownEC=22h + dedup + alert_log; wantmatch.ForItem; the live *discordgo.Session via bot.Start"
provides:
  - "internal/backendsrv/ec package: RunMatch (poll→diff→match→embed→send), the EC producer body"
  - "ec.buildEmbed: the discordgo rich embed (item + WTS tag + price + seen + seller + why-you-wanted-it), null-price/unresolved-seller omitting"
  - "scheduler.ec_auction_match registry job (~10-min dueEC cadence) wired to ec.RunMatch"
  - "scheduler.Start(ctx, db, botSession) signature + main.go bot-before-scheduler reorder threading the live session"
affects: [22-wts-cross-server-monitor, 23-raid-target-monitor]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Thin-producer composition: a new package composes enrich + store + wantmatch + notify without re-implementing any spine gate/dedup/send logic"
    - "Per-item poll-and-diff with first-sight baseline (no replay) + advance-only-on-success, distinct from the job-level cadence cursor"
    - "Typed-nil-interface guard (isNilSender) for a possibly-nil *discordgo.Session boxed into notify.Sender"
    - "Best-effort poll loop: a per-item fetch/parse/match failure logs + continues, never aborting the whole run (D-07)"

key-files:
  created:
    - "internal/backendsrv/ec/ec.go"
    - "internal/backendsrv/ec/embed.go"
    - "internal/backendsrv/ec/urls.go"
    - "internal/backendsrv/ec/ec_test.go"
  modified:
    - "internal/backendsrv/scheduler/scheduler.go"
    - "internal/backendsrv/scheduler/scheduler_test.go"
    - "cmd/squirebot-server/main.go"

key-decisions:
  - "Honored the SPIKE over the plan's stale URL text: poll getdetails/0/{itemname} (server=0 live feed, NAME key form) — the plan action mentioned the id form in a couple places; the SPIKE + 21-01 SUMMARY explicitly override that"
  - "isNilSender guards the typed-nil *discordgo.Session (Go nil-interface gotcha) so a disabled bot is a clean no-op, not a panic inside notify.Send"
  - "WTS-only alerts (u in {0,2}); WTB (u=1) never alerts but STILL advances the cursor (seen, not re-diffed)"
  - "Cursor advances to max(t) only after a successful poll; first-sight baselines without DMing history (no restart replay)"
  - "Embed links to the P1999 wiki via wikiURLFor (D-06, backend-only — zero web/ scope)"

requirements-completed: [WANT-05]

# Metrics
duration: ~10min
completed: 2026-06-06
tasks: 3
files-changed: 7
commits: 3
---

# Phase 21 Plan 03: EC Auction Monitor — End-to-End Producer Summary

**The integration finale (WANT-05): a new `internal/backendsrv/ec` package whose `RunMatch` polls PigParse `getdetails/0/{name}` per wanted item, diffs new WTS auctions against the per-item `ec_auction_cursor`, matches via `wantmatch.ForItem`, and DMs a rich discordgo embed through `notify.Send` — re-implementing none of the Phase 20 spine — registered as the `ec_auction_match` scheduler job with the live bot session threaded `main.go → scheduler → ec`.**

## What Was Built

### Task 1 — scheduler signature + main.go reorder (commit `bec919c`)
- `scheduler.Start` gains a `botSession *discordgo.Session` param (imports `discordgo`); the registry/run-loop/recover-isolation/ticker are otherwise unchanged — the session is captured by the EC job closure added in Task 3.
- `cmd/squirebot-server/main.go`: moved `bot.Start` + `botSession := b.Session()` ABOVE `scheduler.Start`, then `scheduler.Start(ctx, db, botSession)`. The non-fatal/recover-isolated bot start is preserved; both goroutines stay independent and ctx-tied, so a bot/job panic never takes down the HTTP/ingest surface. `botSession` is nil when the bot is disabled.
- `scheduler_test.go`'s `Start(ctx, db)` call updated to thread a nil session.

### Task 2 — the `ec` package: RunMatch + embed builder (commit `38412ec`, TDD)
- **NEW package `internal/backendsrv/ec`** (outside `enrich/jobs` to avoid the `enrich → notify/wantmatch` import cycle).
- `ec.go` `RunMatch(ctx, db, sender notify.Sender, fetch politefetch.Fetcher)`:
  1. nil/typed-nil session → clean no-op (`isNilSender` guards the boxed `*discordgo.Session`).
  2. `ECPollSet` → per item: `GetECCursor` → `fetch(getDetailsURL(name))` → on `!OK` skip-no-advance, on 304 skip → `ParseItemDetail` → `maxTimestamp`.
  3. **First-sight baseline** (`!ok`): set cursor to max(t), DM nothing (no history replay).
  4. **Diff**: for each `t > cursor` AND `u ∈ {0,2}` (WTS — WTB `u=1` never alerts), `wantmatch.ForItem` → `buildEmbed` → `notify.Send(Source:"ec_auction", WantID:&hit.WantID, Embed)`.
  5. **Advance the cursor only after a successful poll** (advances even past seen WTB records, but only WTS records DM).
  6. Send sentinels (`ErrCooledDown`/`ErrGatedOff`/`ErrDMBlocked`) swallowed at debug; per-item failures logged + survived (D-07). V7 logging: ids/status only.
- `embed.go`: `buildEmbed` (Title `<name> — WTS`, wiki URL, Fields: Price [omit when nil — never 0pp], Seen [omit when t unparseable], Seller [omit when unresolved], "Why you wanted it" = Reason + Note); `wikiURLFor` (D-06 idiom), `seenAgo`, `resolveSeller` (best-effort players-map lookup), `buildDetail` (inbox summary).
- `urls.go`: `getDetailsURL` — hardcoded `getdetails/0/` base + `enrich.EncodeURIComponent(name)` (T-21-08 SSRF mitigation).
- `ec_test.go` (fake Fetcher + fake Sender): new-WTS sends an embed, WTB-ignored-but-cursor-advances, BOTH-alerts, first-sight-baseline-no-replay, advance-only-on-success (one item fails, the next still polls), nil-session no-op, Send-sentinel tolerance, and two `buildEmbed` field-omission cases.

### Task 3 — register `ec_auction_match` (commit `5dcd384`)
- Added the `ec_auction_match` registry entry (`dueEC` ≥10-min cadence) wired to `ec.RunMatch(ctx, db, botSession, politefetch.Fetch)`; imported `internal/backendsrv/ec`.
- New `dueEC` predicate; the job-level `job_run` cadence cursor is kept DISTINCT from the per-item `ec_auction_cursor` diff state (two-cursor distinction documented in the entry's comment).
- `scheduler_test.go`: `TestDueEC` (boundary cases) + `TestStart_RegistersECAuctionMatch` (non-blocking + clean cancel with a nil session + the 10-min boundary).

## Deviations from Plan

**1. [Spike handoff, not a plan deviation] URL form = NAME + server=0, not the id form.** The plan's `<action>`/`urls.go` text (lines 163, 122 of 21-PATTERNS) still carried the original RESEARCH recommendation of the **id form** (`getdetails/{itemid}`). The 21-SPIKE.md hand-off and the 21-01 SUMMARY explicitly OVERRIDE that: the bare-id form 400s and the id-in-name-slot returns empty, so the **NAME form** (`getdetails/0/{itemname}`) is the only working key, and **server=0** is the live Blue feed (server=1 is ~11h stale). The plan's frontmatter `truths` and the `<critical_handoffs_from_the_spike>` in the execution prompt both mandate the SPIKE path, so honoring it is following the plan's intent, not deviating from it. `urls.go` polls `getdetails/0/` + the escaped item NAME accordingly.

No other deviations — Tasks 1 and 3 executed exactly as written.

## Verification

- `go build ./...` — exit 0.
- `go test ./...` — exit 0 (full tree; this is the integration point).
- `go vet ./internal/backendsrv/...` — exit 0.
- `go test ./internal/backendsrv/ec/` — exit 0; `go test ./internal/backendsrv/scheduler/` — exit 0.
- Acceptance greps: `func RunMatch` (1), `wantmatch.ForItem` (4), `ec_auction` (17), `SetECCursor` (2) in `ec/ec.go`; `discordgo.MessageEmbed` (8) in `ec/embed.go`; `ec_auction_match` (2) + `ec.RunMatch` (4) in `scheduler.go`; `botSession *discordgo.Session` (1) in `scheduler.go`; `scheduler.Start(ctx, db, botSession)` (1) in `main.go` with `bot.Start` (L241) before `scheduler.Start` (L255).
- **No direct embed send in production**: `grep -rl 'ChannelMessageSendEmbed' internal/backendsrv/ec/ --include='*.go' --exclude='*_test.go'` returns NOTHING — every send routes through `notify.Send` (T-21-10 mitigation; the comment mentions were reworded to keep the grep clean).
- First-sight-baseline (no DM on first sighting / no backlog replay) covered by `TestRunMatch_FirstSightBaseline_NoReplay`.

## Security / Threat Notes

- **T-21-08 (SSRF)**: `getDetailsURL` interpolates only the catalog-sourced item NAME into a hardcoded host/scheme, escaped via `enrich.EncodeURIComponent` — a special-char name cannot break the URL or traverse.
- **T-21-09 (DM carpet-bomb)**: cursor diff + first-sight baseline + advance-only-on-success + the inherited `cooldownEC=22h` dedup — a standing auction is never re-DMed; a restart replays nothing.
- **T-21-10 (ungated/duplicated send)**: every send routes through `notify.Send` (both gates + dedup + alert_log); the job never calls the discordgo embed-send method directly (grep-enforced). The mute gate is upstream in `wantmatch.ForItem`.
- **T-21-11 (PII)**: every EC log line carries `source + item_id + want + status` only — never the DM body, embed content, raw item names, or the players map.
- **T-21-12 (malformed body)**: `ParseItemDetail` is malformed-tolerant; politefetch caps the body; a per-item parse failure is logged + skipped (loop survives).

## Coverage Caveat (D-07)

EC alerts are best-effort and bursty: the EC tunnel is fed only when a guildie is parked in EC running a PigParse parser (weekend-heavy). The monitor ships honestly framed as "near-real-time, within ~10–20 min" (the ~10-min PigParse rebuild + the ~10-min poll cadence) — never instant/guaranteed delivery. When the tunnel is quiet the job simply finds nothing to DM.

## Post-Deploy Smoke (phase gate — pending deploy)

Per the plan's verification block (P20 precedent): after deploy, smoke an embed DM on the box and confirm the `ec_auction_match` job logs a poll. This is a deploy-time UAT, not a code-gate; the code path is fully unit-tested here.

## Self-Check: PASSED

- `internal/backendsrv/ec/ec.go` — FOUND
- `internal/backendsrv/ec/embed.go` — FOUND
- `internal/backendsrv/ec/urls.go` — FOUND
- `internal/backendsrv/ec/ec_test.go` — FOUND
- `internal/backendsrv/scheduler/scheduler.go` (ec_auction_match + Start signature) — FOUND
- `cmd/squirebot-server/main.go` (reorder + threaded session) — FOUND
- commits `bec919c`, `38412ec`, `5dcd384` — FOUND in git log

---
*Phase: 21-ec-tunnel-auction-monitor*
*Completed: 2026-06-06*
