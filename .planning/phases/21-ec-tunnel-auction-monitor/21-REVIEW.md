---
phase: 21-ec-tunnel-auction-monitor
reviewed: 2026-06-05T00:00:00Z
depth: standard
files_reviewed: 17
files_reviewed_list:
  - internal/backendsrv/enrich/pigdetails.go
  - internal/backendsrv/enrich/pigdetails_test.go
  - internal/backendsrv/migrations/00008_ec_cursor.sql
  - internal/backendsrv/migrations/migrate_test.go
  - internal/backendsrv/store/eccursor.go
  - internal/backendsrv/store/eccursor_test.go
  - internal/backendsrv/notify/dm.go
  - internal/backendsrv/notify/dm_test.go
  - internal/backendsrv/wantmatch/match.go
  - internal/backendsrv/wantmatch/match_test.go
  - internal/backendsrv/ec/ec.go
  - internal/backendsrv/ec/embed.go
  - internal/backendsrv/ec/urls.go
  - internal/backendsrv/ec/ec_test.go
  - internal/backendsrv/scheduler/scheduler.go
  - internal/backendsrv/scheduler/scheduler_test.go
  - cmd/squirebot-server/main.go
findings:
  critical: 0
  warning: 4
  info: 4
  total: 8
status: issues_found
---

# Phase 21: Code Review Report

**Reviewed:** 2026-06-05
**Depth:** standard
**Files Reviewed:** 17
**Status:** issues_found

## Summary

Phase 21 wires the EC-tunnel auction monitor into the existing Phase 20 notify spine. The code is careful and well-commented, and the high-risk areas the prompt called out are largely handled correctly:

- **`pigdetails.go` parser** correctly guards the `t`/`u` collision, the nullable `*int` price (explicit `null`-before-`jsonNumber` guard at line 145 prevents `null→0pp`), and malformed-record tolerance. No panics on garbage/truncated/array bodies (object `Unmarshal` fails closed).
- **`notify/dm.go` embed branch** is a single send/record path — the embed-vs-string choice is an inner `func()` (lines 178-185) BEFORE one shared `recordAttempt`/slog block. The gate/dedup/cooldown code is NOT duplicated. Verified against `dm_test.go` (cooldown/gate tests parameterized over both paths).
- **`ec/ec.go` typed-nil guard** (`isNilSender`, lines 229-232) correctly defuses the Go nil-interface gotcha; `RunMatch` no-ops before any fetch (proven by `TestRunMatch_NilSession_NoOp`).
- **`ec/urls.go`** interpolates only the item name into a hardcoded host/scheme via `EncodeURIComponent`, which percent-encodes `/` (path traversal blocked) — confirmed against `wikiitem.go`'s unreserved set.
- **SQLite single-writer discipline** is respected; the scheduler's per-job mutex plus `SetMaxOpenConns(1)` serialize writes.

No BLOCKER-class defects were found. The findings below are correctness edge cases and quality issues — the most notable is a first-sight baseline window (WR-01) that can silently swallow the first real auction for a brand-new want, and a dead 304 branch (WR-02) caused by never passing an ETag.

## Warnings

### WR-01: First-sight baseline can silently swallow the first real auction in a narrow window

**File:** `internal/backendsrv/ec/ec.go:112-130`
**Issue:** When a wanted item currently has zero auctions, `pollItem` returns early at line 112-115 (`len(detail.Items) == 0`) WITHOUT writing a cursor — so the item stays "never-cursored" (`ok=false`). If a genuinely new WTS auction then appears on the next poll, the `!ok` first-sight branch (line 123) fires and records `max(t)` as a *baseline* with NO DM. The first real auction for that want is consumed as baseline history and never alerted. The window is one poll cycle (~10 min) after a want is added for an item that has no live auctions, so it will bite in practice for newly-added wants on quiet items.

This is partially by-design (first-sight = no replay), but the "empty first poll then a new auction" path is not the standing-backlog case the baseline rule targets — it is a true new event being suppressed.

**Fix:** On an empty first poll, still record a baseline cursor (e.g. the empty-string `""`) so the next poll's auctions are treated as a diff, not a baseline:
```go
if len(detail.Items) == 0 {
    if !ok { // never-cursored + no auctions yet: baseline to empty so the next real auction diffs
        _ = s.SetECCursor(ctx, item.ItemID, "", time.Now().Unix())
    }
    slog.Info("ec_auction_match: no auctions", "source", source, "item_id", item.ItemID, "status", "empty")
    return
}
```
Since `maxTimestamp` returns `""` for an empty slice and any real RFC3339 `t > ""` lexically, a `""` baseline makes the next poll's auctions all diff as new — the desired "alert the first auction we actually witness after the want exists" behavior. (Confirm this matches the ROADMAP intent; if the intent really is "never alert anything seen in the first poll, cursor or not," document the empty-first-poll case explicitly.)

### WR-02: The 304 / `FromCache` branch is dead in production — every poll re-fetches and re-parses

**File:** `internal/backendsrv/ec/ec.go:95, 101-105`
**Issue:** `pollItem` calls `fetch(ctx, getDetailsURL(...), politefetch.Options{})` with an empty `Options` — no `ETag` / `LastModified`. `politefetch.Fetch` only sends `If-None-Match`/`If-Modified-Since` when those Options fields are non-empty (politefetch.go:151-156), and it does NOT consult `etag_cache` itself. So the server never receives a conditional request, never returns 304, and the `res.FromCache` branch (lines 101-105) is unreachable in production. Every 10-min poll for every wanted item does a full fetch + JSON parse of the getdetails body. The comment at line 101 ("304 — nothing changed") describes behavior that cannot occur. Given the per-item fan-out across the whole poll set every 10 minutes, this is the politeness/load concern the design explicitly tried to avoid (D-09 "keep the per-item getdetails fan-out polite").

**Fix:** Either (a) thread the per-item ETag through `etag_cache` (read before the fetch, pass in `Options{ETag: …}`, persist the returned ETag on a 200) so the 304 short-circuit actually fires; or (b) if conditional GETs are intentionally out of scope for P21, remove the dead `FromCache` branch and update the comment so a future reader doesn't assume 304 handling is live. Option (a) is the real fix and materially reduces upstream load.

### WR-03: `wantmatch.ForItem` is re-queried once per new auction instead of once per item

**File:** `internal/backendsrv/ec/ec.go:137-152`
**Issue:** The diff loop calls `wantmatch.ForItem(ctx, db, item.ItemID)` *inside* the per-auction loop (line 144). For an item with N new WTS auctions in one poll, this runs the same `SELECT … FROM wantlist_item WHERE item_id=?` N times and produces N×(hits) `sendHit` calls. Correctness is preserved (notify dedup collapses the duplicate sends to one `ErrCooledDown` each), but the wantlister set does not change between auctions of the same item in a single poll, so the repeated query and redundant send attempts are wasted work — and after the first new auction records a `sent` row, every subsequent auction in the same poll is pure churn against the DB dedup probe.

**Fix:** Hoist `wantmatch.ForItem` out of the auction loop — resolve hits once per item, then iterate auctions. Since dedup means only one alert per item per window lands anyway, resolving hits once and sending for a single representative (newest alertable) auction is sufficient.

### WR-04: Send-then-advance ordering relies on the 22h cooldown to prevent duplicate DMs on a cursor-write failure

**File:** `internal/backendsrv/ec/ec.go:149-159`
**Issue:** `sendHit` DMs and records `sent` rows (lines 149-151) BEFORE the cursor advance at line 157. If `SetECCursor` then fails (line 157-159), the failure is only logged at `Warn` and the cursor does NOT advance — so the next poll re-diffs the same auctions and re-enters `sendHit`. The sole guard against a duplicate DM is the 22h notify cooldown (`RecentAlertExists`). That backstop works here, but the durability of cursor advance is now implicitly coupled to the cooldown window, and alert_log can hold a `sent` row for an auction whose cursor never advanced (audit/cursor divergence).

**Fix:** Acceptable given the 22h cooldown, but make the coupling explicit and louder: escalate the `SetECCursor` failure from `slog.Warn` to `slog.Error` (it is a durability failure, not a transient skip), and add a comment at line 157 noting the cooldown is the de-dup backstop for a failed advance so a future cooldown change doesn't silently remove the protection.

## Info

### IN-01: PigParse getdetails URL (carrying the item name) is logged at the politefetch layer

**File:** `internal/backendsrv/enrich/politefetch/politefetch.go:164, 186` (called via `ec/ec.go:95`)
**Issue:** On transport / body-read errors, `politefetch.Fetch` logs the full `url`, which for the EC path embeds the (percent-encoded) item name. The V7 rule targets user/DM data; item names are public catalog data, so this is not a PII leak. `ec.go` itself is clean (ids/status only). No change required unless the project treats item names as sensitive.

### IN-02: `resolveSeller` join key (`a.I`) is a best-effort guess with no documented mapping

**File:** `internal/backendsrv/ec/embed.go:83-91`
**Issue:** `resolveSeller` uses `strconv.Itoa(a.I)` (the auction-instance id) as the `players` map key. The comments acknowledge the key→auction relationship is undocumented (D-05), and it fails closed to `""` (omits the field), so there is no crash risk. The Seller field will likely be wrong or absent for most auctions until the mapping is confirmed. Acceptable as documented best-effort; flagged so it isn't mistaken for a verified join.

### IN-03: `seenAgo` and `maxTimestamp` make different assumptions about the same `t` field

**File:** `internal/backendsrv/ec/embed.go:56-60`, `internal/backendsrv/ec/ec.go:210-218`
**Issue:** The diff cursor (`maxTimestamp`, lexical string compare) and the display parse (`seenAgo`, `time.Parse(RFC3339, …)`) interpret the same `t` differently. No bug today — both handle the documented `…+00:00` shape. But if PigParse ever changes the timestamp format, the cursor keeps working (lexical) while Seen silently disappears, masking the drift. Consider a counts-only `slog.Warn` when `seenAgo` fails to parse a non-empty `t`.

### IN-04: Cursor boundary uses `<=` against a max that spans WTB records

**File:** `internal/backendsrv/ec/ec.go:118, 138`
**Issue:** `maxTimestamp` spans ALL items (WTS + WTB), so the cursor can jump past WTB records (intended). Combined with `a.T <= cursor` (line 138), an auction whose `t` exactly equals the stored cursor is treated as already-seen. Identical sub-second RFC3339 timestamps across two distinct auctions are improbable, but if two auctions share a `t` and one lands exactly on the cursor boundary, the second could be skipped. Documented for completeness — extremely unlikely with observed millisecond precision.

---

_Reviewed: 2026-06-05_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
