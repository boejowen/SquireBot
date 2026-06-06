---
status: partial
phase: 21-ec-tunnel-auction-monitor
source: [21-VERIFICATION.md]
started: 2026-06-05
updated: 2026-06-06
---

## Current Test

DEPLOYED LIVE 2026-06-06 (schema v8; bot connected; scheduler jobs:4). UAT #1 accepted as ORGANIC-confirm; UAT #2 confirmed at the first scheduler tick.

## Tests

### 1. Live EC-auction DM end-to-end
expected: Deploy the new server binary; trigger (or wait for) a real EC-tunnel WTS auction of a wanted item. The wantlister receives a discordgo rich-embed DM titled `<item> — WTS` with a price field (or omitted when unknown), a "Seen ~N min ago" field, optional seller, a "Why you wanted it" field, and a P1999 wiki link; `alert_log` records a `sent` row; a repeat poll within the 22h cooldown does NOT re-DM.
result: [organic — pending] Deploy done 2026-06-06. Cannot be forced (D-07: fires only on a real new WTS auction of a wanted item while the tunnel is being parsed). User accepted organic confirmation: it will fire on the first real match. Send path already proven live in P20 ("test alert" DM); EC→embed→notify path fully unit-tested (TestRunMatch_NewWTS_Sends, TestBuildEmbed_*). Confirm by checking for the DM + an `alert_log` row (source=ec_auction, send_status=sent) once an organic match occurs.

### 2. Live scheduler cadence + panic isolation
expected: On the deployed box, journal/logs show periodic `ec_auction_match` poll lines (source=ec_auction) on the ~10-min cadence and the per-item `ec_auction_cursor` advances; the HTTP API + ingest stay up (bot/scheduler recover-isolation holds).
result: [confirming at ~10-min tick] At startup: `scheduler started interval 10m0s jobs:4` (the +1 is `ec_auction_match`); service active, API up. First live poll confirmed at the next tick (see STATE.md / background check 2026-06-06).

## Summary

total: 2
passed: 0
issues: 0
pending: 2
skipped: 0
blocked: 0

## Gaps
