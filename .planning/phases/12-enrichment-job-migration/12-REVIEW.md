---
phase: 12-enrichment-job-migration
reviewed: 2026-05-29T22:40:00Z
depth: deep
reviewer: gsd-code-reviewer
advisory: true
files_reviewed: 16
files_reviewed_list:
  - internal/backendsrv/migrations/00003_enrich_columns.sql
  - internal/backendsrv/store/enrich.go
  - internal/backendsrv/store/jobstate.go
  - internal/backendsrv/store/itemids.go
  - internal/backendsrv/enrich/eqconst.go
  - internal/backendsrv/enrich/pigparse.go
  - internal/backendsrv/enrich/wikiitem.go
  - internal/backendsrv/enrich/wikispell.go
  - internal/backendsrv/enrich/wikigear.go
  - internal/backendsrv/enrich/politefetch/politefetch.go
  - internal/backendsrv/buildinfo/buildinfo.go
  - internal/backendsrv/enrich/jobs/pigparse.go
  - internal/backendsrv/enrich/jobs/wiki.go
  - internal/backendsrv/enrich/jobs/urls.go
  - internal/backendsrv/scheduler/scheduler.go
  - cmd/squirebot-server/main.go
findings:
  high: 1
  medium: 2
  low: 4
  total: 7
status: issues_found
---

# Phase 12: Code Review Report — Enrichment Job Migration (TS → Go)

**Reviewed:** 2026-05-29T22:40:00Z
**Depth:** deep (cross-file + TS-source transliteration comparison + empirical repro)
**Files Reviewed:** 16 Go/SQL source files (+ their tests + the 7 TS sources they port)
**Status:** issues_found (1 HIGH, 2 MEDIUM)
**Advisory:** this review does NOT block phase completion. Act on findings via `/gsd-code-review-fix 12`.

## Summary

This is a careful, well-structured port. The whole Phase 12 suite **builds clean, vets clean, and every test passes**. The hard parts are genuinely correct: the SHA-1 path is byte-identical to the TS even on multi-byte UTF-8 (verified by direct comparison); `politefetch` faithfully reproduces the backoff schedule, Retry-After honor+clamp, 304 short-circuit, bounded body read, body-always-closed, and ctx-aware backoff unwind, with strong test coverage; all five store dimension-write methods are fully parameterized (no injection surface), the `wiki_gear_tier` full-table replace correctly works around the NULL-poisoned UNIQUE (Pitfall 1), and the t=0 WTS dedup respects the `item_id` PK. The scheduler's due predicates, immediate-startup check, advance-always cursor, and clean `ctx.Done()` shutdown are all tested and correct. Security posture (ASVS L1, outbound-only) holds: TLS verification never disabled, hardcoded API hosts (no SSRF), no secret on outbound requests, parameterized SQL throughout, bounded reads against OOM.

One **HIGH** correctness regression was found and **empirically reproduced**: the weekly gear-tier pass added per-page ETag/304 conditional requests that the TS source never used, which — combined with the "replace only when both pages are fresh" rule — creates a self-perpetuating staleness trap where a single-page wiki edit is permanently dropped. Two **MEDIUM** transliteration divergences round out the actionable set: a byte-vs-rune truncation in `extractSummary` that can emit invalid UTF-8, and a `url.QueryEscape`-vs-`encodeURIComponent` mismatch in the request-URL builder (the same bug-class the prompt flagged as already-bitten). The remaining items are LOW.

---

## Findings

| Severity | File:Line | Issue | Fix |
|----------|-----------|-------|-----|
| HIGH | jobs/wiki.go:343-406 (esp. 390) | Gear-tier pass sends per-page ETags but only replaces when BOTH pages are fresh; a single changed page advances its ETag without writing, so its update is permanently lost (self-perpetuating via the cached ETag). The TS gear trigger sent NO ETag — always fetched fresh, always replaced. | Fetch the 2 gear pages **unconditionally** (no ETag) to match the TS, OR advance a page's ETag only inside the replace tx and treat a 304 as "re-fetch unconditionally because the other page changed". |
| MEDIUM | enrich/wikiitem.go:367-375 | `extractSummary` truncates with byte length (`len(text)`, `text[:200]`); the TS uses UTF-16 char length + `slice`. A multi-byte rune straddling byte 200 is sliced mid-sequence → **invalid UTF-8** stored in the summary (proven: output bytes end `… e2 | e2 80 a6`). | Operate on `[]rune`: `r := []rune(text); if len(r) <= maxSummaryLen { return text }; cut := string(r[:maxSummaryLen])`, then word-boundary trim on the rune-cut string. |
| MEDIUM | jobs/urls.go:47-50 | `wikiParseURL` escapes the page title with `url.QueryEscape`, but the code comment claims byte-faithfulness to the TS `encodeURIComponent`. They diverge: `Lord_Nagafen's_Lair` → `%27`, `Cloak_of_Flames_(Quest)` → `%28..%29`. Same bug-class as the pre-review `url.PathEscape` slip. Functionally benign today (MediaWiki percent-decodes `page=`), but untested and fragile. | Reuse the proven escaper: export `enrich.EncodeURIComponent` (already correct, wikiitem.go:113) and use it here instead of `url.QueryEscape`. Also align `redirects=1` → `redirects=true` if exact URL/etag-key parity with the TS is desired. |
| LOW | enrich/wikispell.go + store/enrich.go:239-253 | `wiki_spells` has `UNIQUE(class, level, spell_name)` but the parser does not dedup within a class and the write uses plain INSERT (not upsert). A duplicate `(class,level,name)` from a wiki/template quirk would fail the whole class's replace (rolled back, logged, skipped) where the TS Sheet-write tolerated it. Not triggered by current fixtures. | Dedup spell rows per class before insert, or use `INSERT ... ON CONFLICT(class,level,spell_name) DO NOTHING` / `INSERT OR IGNORE`. |
| LOW | enrich/politefetch/politefetch.go:274 | `parseRetryAfter` uses `strconv.Atoi`, which rejects `"30 "` / `"30; x"`; JS `parseInt` would return `30`. Divergence from TS, but **fails safe** (falls back to schedule). Tests even pin the Go behavior. | Optional: trim and parse a leading integer run if exact TS parity on malformed Retry-After is wanted. Otherwise leave (safe). |
| LOW | scheduler/scheduler.go:70,213-217 | The per-job `sync.Mutex`/`TryLock` is effectively dead in the real flow: `checkAndRun` calls `runJob` **synchronously** from one goroutine, so two cycles of the same job can never overlap. The overlap-skip is only exercised by a test that manually holds the lock. Harmless, but the comment overstates its role. | None required. Optionally drop the mutex or document it as defensive-only. The same applies to the "two jobs may run concurrently" comment (line 30) — `SetMaxOpenConns(1)` serializes their DB writes. |
| LOW | enrich/pigparse.go:118-119,138 | `coerceRow` accepts a JSON number for `i` then does `int(i)`, truncating a fractional item id (e.g. `1234.9` → `1234`); the TS passes `obj.i` through verbatim. Item ids are always integers in practice, so this never bites. | None required (document the integer assumption if desired). |

---

## HIGH — detail

### H-01: Gear-tier weekly pass permanently drops single-page wiki edits (ETag staleness trap)

**File:** `internal/backendsrv/enrich/jobs/wiki.go:343-406` (root cause at line 390)

**What the code does.** `runWikiGearTier` fetches both Velious gear pages through `fetchWikiPage`, which reads the cached ETag/Last-Modified and sends them as conditional headers. The full-table replace fires only when **both** pages return a fresh 200 (`allFresh`). Critically, for any page that DID return 200, line 390 persists its new ETag **immediately** via `s.SetETag(...)` — which runs its own `INSERT ... ON CONFLICT` against the DB, independent of whether the replace later happens. The inline comment ("Note the page ETag now; only committed below if we do the replace") describes intended behavior that is **not** implemented.

**The TS source never had this.** `apps-script/src/triggers/refreshWikiGearTier.ts:194` calls `politeFetch(url)` with **no etag option** — the gear pages are always fetched fresh (no conditional request), and the all-or-nothing replace rebuilds the table on every successful weekly run. The Go port ADDED ETag/304 to the gear pages, which is what introduces the bug.

**Failure scenario (the common case — the two pages are edited independently on the wiki):**
1. Pre-Raid page changes; Raiding page unchanged.
2. Pre-Raid → 200 (new rows accumulated into `allRows`); **its new ETag is persisted to `etag_cache` at line 390.**
3. Raiding → 304 → `allFresh = false` → `continue`.
4. `!allFresh` ⇒ return without replacing. The table keeps the OLD rows; the Pre-Raid change is discarded.
5. **Next run:** Pre-Raid now sends `If-None-Match: <new etag>` → wiki returns 304 → `allFresh = false` again. Replace skipped — **forever** — until the Raiding page also happens to change in the same run, or the `etag_cache` row is manually cleared.

**Empirical proof.** A throwaway repro (run #2 changes the Pre-Raid page, Raiding 304s; run #3 lets Pre-Raid's new ETag 304 too) produced:
```
run #2 log: gear page unchanged (304) ... page="Players:Velious Raiding Gear"
            gear_replaced=false gear_rows=1
CHANGED Helm rows: run2=0 run3=0
```
The changed content never reaches `wiki_gear_tier` on run #2 or run #3. The gear-tier data is frozen at the first successful run's content after any single-page edit.

**Impact.** `gear_check` (the Velious progression view P14 builds on this table) silently shows stale gear recommendations indefinitely. No error is surfaced — only an INFO log line. This is a data-correctness regression vs. the system being ported.

**Fix (preferred, matches the TS).** Fetch the gear pages unconditionally — they require the complete combined set every run, so conditional requests buy nothing and break the replace. Add an unconditional fetch path (skip `GetETag`/SetETag for gear sources), or pass empty `Options{}` to the fetch for these two pages and stop calling `SetETag` in the gear loop. If 304-politeness for gear is genuinely wanted, the ETag must only advance inside the replace tx, AND a 304 on one page must force an unconditional re-fetch of the other so a complete set can still be assembled — materially more complex than just dropping the conditional request.

---

## MEDIUM — detail

### M-01: `extractSummary` byte-truncation can emit invalid UTF-8 (byte-vs-rune divergence)

**File:** `internal/backendsrv/enrich/wikiitem.go:367-375`

The TS `extractSummary` (`wiki-parser.ts:237-241`) measures and slices by `string.length` (UTF-16 code units) and `string.slice`. The Go port uses `len(text)` (bytes) and `text[:maxSummaryLen]` (byte slice). For a summary whose 200th byte falls inside a multi-byte UTF-8 rune, the byte slice cuts the rune in half:

- Constructed input (199 ASCII chars + `☃` + filler): Go output's tail bytes are `61 61 e2 | e2 80 a6` — a lone `e2` (the truncated snowman, invalid UTF-8) immediately followed by the `…` ellipsis (`e2 80 a6`). `valid_utf8 = false`.
- The same input through the TS keeps the whole `☃` and stays valid.

P1999 item notes do contain non-ASCII (em-dashes, curly quotes, accented zone/mob names), so this is reachable for long summaries. The fixtures are pure ASCII (char count == byte count), which is why the test suite did not catch it. Not a crash and not data loss — but the stored summary contains a broken character on a subset of items, diverging from the source.

**Fix:** convert once to `[]rune` and do length/slice/word-boundary work on runes:
```go
r := []rune(text)
if len(r) <= maxSummaryLen {
    return text
}
cut := string(r[:maxSummaryLen])
lastSpace := strings.LastIndex(cut, " ")
if lastSpace > len(cut)-30 { // recompute the boundary on the rune-cut string
    cut = cut[:lastSpace]
}
return strings.TrimSpace(cut) + "…"
```

### M-02: `wikiParseURL` uses `url.QueryEscape`, not `encodeURIComponent` (and the comment claims otherwise)

**File:** `internal/backendsrv/enrich/jobs/urls.go:47-50`

`wikiParseURL` builds the request URL with `url.QueryEscape(strings.ReplaceAll(pageTitle, " ", "_"))`. The TS builds wiki request URLs with `encodeURIComponent(name.replace(/ /g, '_'))` (e.g. `refreshWikiItems.ts:176`, `refreshWikiSpells.ts:170`, `refreshWikiGearTier.ts:193`). These differ for characters common in EQ titles:

| Title (underscored) | `encodeURIComponent` (TS) | `url.QueryEscape` (Go) |
|---|---|---|
| `Lord_Nagafen's_Lair` | `Lord_Nagafen's_Lair` | `Lord_Nagafen%27s_Lair` |
| `Cloak_of_Flames_(Quest)` | `Cloak_of_Flames_(Quest)` | `Cloak_of_Flames_%28Quest%29` |

The package comment explicitly claims the result is "byte-faithful to the TS encodeURIComponent" — it is **not**. This is the same defect class the prompt flagged as already-slipped (`url.PathEscape` vs `encodeURIComponent`). It is functionally benign **today** because MediaWiki percent-decodes the `page=` parameter, so `%27`/`%28` resolve to the same article — and the value is only used as the request URL + the internally-consistent `etag_cache` key, never as stored display data (the stored `wiki_url` is built separately by the parser's correct `encodeURIComponent`). But the divergence is untested (the test server's `r.URL.Query().Get("page")` decodes both forms identically, so the mismatch is invisible to the suite) and contradicts its own documented contract.

**Fix:** the codebase already has a correct `encodeURIComponent` (`wikiitem.go:113`). Export it (`EncodeURIComponent`) and use it here so there is ONE escaper for wiki page names:
```go
escaped := enrich.EncodeURIComponent(strings.ReplaceAll(pageTitle, " ", "_"))
```
Consider also `redirects=true` (TS) instead of `redirects=1` if exact URL / etag-key parity is wanted; both work against MediaWiki, so this part is cosmetic.

---

## What was checked and is correct (no findings)

- **SHA-1 path** (`wikiitem.go:151-154`): byte-identical to the TS on multi-byte UTF-8 (`4b6cb5a2…3a25bbf` from both Go `sha1.Sum([]byte(s))` and Node). The dropped signed-byte fixup is correct — Go bytes are unsigned.
- **politefetch** (`politefetch.go`): backoff `[2,4,8,16,32]s`; Retry-After honored + clamped to 600s; 304 returns empty body + `FromCache` (job never re-parses — Pitfall 6); `io.LimitReader(16MB)` cap applied and tested; `resp.Body` closed on every path (200/304/retry/non-retry/read-error) via `closeBody`; ctx-cancel unwinds backoff promptly (real-timer test); retry set `{429,503,504}` vs immediate-surface partition correct; no `tls.Config` ⇒ verification stays on; UA carries the contactable GitHub URL, no secret on the request.
- **store SQL** (`enrich.go`, `jobstate.go`, `itemids.go`): every value bound via `?`; `pigparse_price`/`item_master` upserts keyed on the real `item_id` PK; `wiki_gear_tier` FULL-table replace (correct for the NULL-poisoned `UNIQUE(tier,class,slot,item_id)`); per-class and per-item-id scoped DELETE+INSERT leave other keys untouched; every `*Tx` composes in the caller's tx with `defer tx.Rollback()` + explicit `Commit`; prepared stmts `Close`d.
- **pigparse job** (`jobs/pigparse.go`): t=0 WTS filter respects the PK (tested: 4333 distinct WTS rows); truncation guard is a LOG that still writes (D-4, tested); 304 skips parse+write and preserves the sentinel row (tested); fetch failure records `error` + returns (advance-always).
- **scheduler** (`scheduler.go`): `duePigparse` (>=24h) and `dueWiki` (Sunday-UTC, once per Sunday) both tested incl. boundaries; immediate startup check tested; advance-always cursor on success and error tested; clean `ctx.Done()` return tested; network/DB fetches happen OUTSIDE the held tx, so `SetMaxOpenConns(1)` serializes writes without holding the single conn across slow I/O.
- **migration 00003** (`00003_enrich_columns.sql`): forward-only, one-column-per-ALTER (SQLite constraint), nullable adds; `job_run`/`etag_cache` PKs correct; idempotent re-run tested.
- **security (ASVS L1, outbound-only):** `PigparseURL`/`WikiAPIBase` hardcoded (no caller-controlled host/scheme — SSRF mitigation holds); untrusted JSON parsed into typed structs / `json.RawMessage` with type guards (no panic on malformed input — pigparse 1% tolerance, wiki per-page log-and-continue); bounded body read guards OOM; no cloud-auth/secret dependency introduced (main.go preserves the v2 invariant).

---

_Reviewed: 2026-05-29T22:40:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
