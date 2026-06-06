---
phase: 21-ec-tunnel-auction-monitor
fixed_at: 2026-06-05T00:00:00Z
review_path: .planning/phases/21-ec-tunnel-auction-monitor/21-REVIEW.md
iteration: 1
findings_in_scope: 4
fixed: 4
skipped: 0
status: all_fixed
---

# Phase 21: Code Review Fix Report

**Fixed at:** 2026-06-05
**Source review:** .planning/phases/21-ec-tunnel-auction-monitor/21-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 4 (all WARNINGS; the 4 Info items are out of scope)
- Fixed: 4
- Skipped: 0

All four warnings are fixed and committed atomically. Final verification:
`go build ./...` PASS, `go vet ./internal/backendsrv/...` PASS, `go test ./...` PASS
(every package green, incl. the two new EC regression tests).

The fixes stay inside the locked Phase 21 decisions: D-02 (WTS-only, u∈{0,2}),
D-09 (polite-in-code), D-10 (~22h cooldown), D-07 (advance-only-on-success). Every
send still routes through `notify.Send` — no direct `ChannelMessageSendEmbed` was
added. Backend-only; no `web/` changes. The 21-SPIKE invariants (server=0, NAME
lookup key) are untouched.

## Fixed Issues

### WR-01: First-sight baseline could silently swallow the first real auction

**Files modified:** `internal/backendsrv/ec/ec.go`, `internal/backendsrv/ec/ec_test.go`
**Commit:** daba251
**Applied fix:** On an EMPTY first poll for a never-cursored item (zero live
auctions), `pollItem` now baselines the cursor to the empty string `""` instead of
returning without writing a cursor. Because `maxTimestamp("") < any RFC3339 t`
lexically, the first REAL auction on a later poll diffs as new and DMs, rather than
being routed through the first-sight baseline branch and swallowed as "history".
The standing-backlog no-replay intent (criterion 4) is preserved: an item that
already has auctions on its first poll still baselines to `max(t)`. The empty-poll
cursor-write failure also logs at Error (durability fault, like WR-04). Added
`TestRunMatch_EmptyFirstPoll_BaselinesEmpty_ThenNewAuctionDMs` proving: empty first
poll → cursor `""` (ok=true) → subsequent real auction DMs and advances the cursor.

### WR-02: The 304 / FromCache branch was dead in production

**Files modified:** `internal/backendsrv/ec/ec.go`, `internal/backendsrv/ec/ec_test.go`
**Commit:** 2103af3
**Applied fix:** Threaded a per-item ETag/Last-Modified through `pollItem` using the
existing URL-keyed `etag_cache` store (`store.GetETag`/`store.SetETag`) — the same
conditional-request mechanism the daily PigParse and weekly wiki jobs use. The
getdetails URL is per-item, so it doubles as the per-item cache key. `pollItem` now
reads the cached ETag before the fetch (falling back to an unconditional fetch on a
cache-read error), passes `politefetch.Options{ETag, LastModified}`, and persists
the response ETag after a successful 200+parse. An unchanged item now receives a 304
and short-circuits the body fetch + JSON parse (D-09 politeness). Added
`TestRunMatch_ETag304_SkipsParseAndSend` proving poll 1 persists the ETag, poll 2
sends it as If-None-Match, and the 304 skips parse + send with the cursor untouched.

### WR-03: wantmatch.ForItem re-queried once per auction instead of once per item

**Files modified:** `internal/backendsrv/ec/ec.go`
**Commit:** 7d6aad5
**Applied fix:** Hoisted `wantmatch.ForItem` out of the per-auction loop. The
wantlister set does not change between auctions of the same item in a single poll,
so hits are now resolved ONCE per item (lazily, on the first alertable auction — an
item whose only new records are WTB never queries). Behavior is unchanged: the
notify dedup (cooldownEC=22h) already collapses the duplicate sends to one DM per
window, so reusing the resolved hits across the poll's new auctions is equivalent —
purely fewer DB round-trips. A wantmatch error now `break`s (equivalent to the old
`continue`, since the query result would not change within the same poll) and the
cursor still advances afterward.

### WR-04: Send-then-advance ordering relied on the 22h cooldown implicitly

**Files modified:** `internal/backendsrv/ec/ec.go`
**Commit:** cdb5a0d
**Applied fix:** Escalated the `SetECCursor` cursor-advance failure log from
`slog.Warn` to `slog.Error` (a durability fault — the cursor is now divergent from
the alert_log rows already written — not a transient skip) and added a comment
documenting the send-then-advance coupling: the cooldownEC=22h notify dedup
(`RecentAlertExists`, D-10) is the SOLE de-dup backstop against a duplicate DM if the
advance fails, so a future cooldown change must keep cooldownEC well above the poll
cadence or the protection silently weakens.

## Notes on Out-of-Scope Info Findings

IN-01..IN-04 were left as-is per scope (warnings only). None is trivially coupled to
a warning fix. IN-01 (item name in politefetch URL logs) is public catalog data, not
PII. IN-02 (`resolveSeller` best-effort join) is documented best-effort. IN-03/IN-04
are extremely-unlikely timestamp-format / boundary edge cases already documented in
the code.

---

_Fixed: 2026-06-05_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
