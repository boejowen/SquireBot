---
status: partial
phase: 21-ec-tunnel-auction-monitor
source: [21-VERIFICATION.md]
started: 2026-06-05
updated: 2026-06-05
---

## Current Test

[awaiting human testing — deploy-time smoke, P20 precedent]

## Tests

### 1. Live EC-auction DM end-to-end
expected: Deploy the new server binary; trigger (or wait for) a real EC-tunnel WTS auction of a wanted item. The wantlister receives a discordgo rich-embed DM titled `<item> — WTS` with a price field (or omitted when unknown), a "Seen ~N min ago" field, optional seller, a "Why you wanted it" field, and a P1999 wiki link; `alert_log` records a `sent` row; a repeat poll within the 22h cooldown does NOT re-DM.
result: [pending]

### 2. Live scheduler cadence + panic isolation
expected: On the deployed box, journal/logs show periodic `ec_auction_match` poll lines (source=ec_auction) on the ~10-min cadence and the per-item `ec_auction_cursor` advances; the HTTP API + ingest stay up (bot/scheduler recover-isolation holds).
result: [pending]

## Summary

total: 2
passed: 0
issues: 0
pending: 2
skipped: 0
blocked: 0

## Gaps
