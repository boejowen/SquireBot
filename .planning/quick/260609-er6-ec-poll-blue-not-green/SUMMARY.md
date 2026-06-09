---
quick_id: 260609-er6
slug: ec-poll-blue-not-green
status: complete
date: 2026-06-09
relates_to: Phase 21 (EC-Tunnel Auction Monitor) — corrects the 21-SPIKE server finding
commits: [de15933]
deploy: pending (backend binary swap + DELETE FROM ec_auction_cursor; no migration)
---

# Hotfix — EC monitor polled the WRONG P99 server (Green, not Blue)

**Bug (user-reported 2026-06-09):** guildies who play **Blue** received EC-tunnel DMs whose listed seller was auctioning the item on **Green**. Root cause: `ec/urls.go` hardcoded the PigParse `getdetails` feed to server segment **0**, which the Phase 21 spike (`21-SPIKE.md`) wrongly concluded was "LIVE Blue." The spike only proved server=0 was *fresher* (more parsers) — never which P99 server it was. It is **Green**.

**Evidence (live probe 2026-06-09):** server=1 is live Blue and matches the catalog convention `getall/1`=Blue. Per-item max_t, server=0 (Green) vs server=1 (Blue):
- Cloak of Flames: 0→2026-06-08T21:55 · **1→2026-06-08T23:02** (Blue fresher)
- Fungi Tunic: 0→2026-06-09T13:58 · 1→2026-06-07T16:53
- Manastone / FBSS: server=1 sparser (Blue has fewer parsers) but live.
getdetails records carry NO per-auction server field (u/i/p/t only) → the URL server segment is the only lever.

## Changed (code only; NO logic change beyond the server segment)
- `internal/backendsrv/ec/urls.go` — `getDetailsBase` `/getdetails/0/` → `/getdetails/1/`; comment rewritten (server=1=Blue per `getall/1`=Blue; 2026-06-09 correction that server=0=Green); SSRF note kept.
- `internal/backendsrv/ec/urls_test.go` — expected URL `/0/` → `/1/` (full-equality + Contains).
- `internal/backendsrv/ec/ec.go` — flow comment `getdetails/0/` → `/1/` + correction note (comment-only).
- `internal/backendsrv/enrich/pigdetails.go` — endpoint doc `{server}=0 LIVE Blue (NOT 1)` → `{server}=1` Blue + correction (doc-only; parser/`t`-collision/`u`-direction docs untouched).
- `internal/backendsrv/scheduler/scheduler.go` — stray `getdetails/0/` job comment → `/1/` (comment-only; caught by grep-verify).

Parser, cursor diff, WTS `u∈{0,2}` filter, first-sight baseline, and the notify spine are all unchanged. Gates green: `go build ./...`, `go vet ./internal/backendsrv/...`, `go test ./internal/backendsrv/ec/ ./internal/backendsrv/enrich/`. Grep-verify: no `getdetails/0` anywhere in `internal/backendsrv/`.

## Deploy (orchestrator — separate step)
1. **Backend binary swap** (no migration) so the live `ec_auction_match` job polls `/getdetails/1/` (Blue).
2. **`DELETE FROM ec_auction_cursor;` — REQUIRED.** Stored cursors were recorded from the Green feed; without a reset the first Blue poll diffs Blue auctions against Green-derived cursors → spurious alerts or a suppression window. Clearing forces every wanted item back through the first-sight baseline (records max-t, no DM) against Blue, then alerts on genuinely new Blue auctions.
3. Old `/0/` `etag_cache` rows go unused (new `/1/` URLs are different keys → fetch fresh); no cleanup needed.

## Honest expectation
Blue alerts will be **less frequent** than the Green noise (fewer people parse the Blue EC tunnel) but **correct** — per-item coverage depends on someone running a PigParse parser in Blue's EC tunnel (the feature's documented best-effort/coverage caveat, 21-SPIKE D-07).
