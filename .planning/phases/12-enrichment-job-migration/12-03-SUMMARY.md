---
phase: 12-enrichment-job-migration
plan: 03
subsystem: api
tags: [go, net-http, politefetch, etag, 304, backoff, retry-after, user-agent, limitreader, enrichment]

# Dependency graph
requires: []   # independent Wave-1 plan; politefetch imports the sibling buildinfo package created in this same plan
provides:
  - "internal/backendsrv/buildinfo: Version (ldflags -X settable, 'dev' default) + UserAgent() => 'SquireBot/<ver> (+https://github.com/boejowen/SquireBot)'"
  - "internal/backendsrv/enrich/politefetch: Fetch (production Fetcher) + Fetcher seam + FetchResult + Options — a faithful Go net/http port of politeFetch.ts with all 12 politeness controls"
  - "304 short-circuit (FromCache=true, empty body) so the job skips re-parse/re-write; 200 captures ETag + Last-Modified"
  - "exponential backoff [2s,4s,8s,16s,32s] on {429,503,504} honoring integer Retry-After (clamped 0-600s); non-retriable statuses surface immediately"
  - "io.LimitReader(~16MB) OOM guard + ctx-aware sleep seam (clean shutdown + instant tests)"
affects: [12-04 jobs, 12-05 scheduler]

# Tech tracking
tech-stack:
  added: []   # stdlib only (net/http, io, context, time, strconv, log/slog)
  patterns:
    - "net/http polite client ported 1:1 from politeFetch.ts; TLS verification ON (no custom tls.Config); default redirect follow"
    - "ctx-aware sleep seam (time.NewTimer + select ctx.Done()/t.C) mirroring internal/update/check.go's checkSleepFn — backoff unwinds on SIGTERM, tests inject no-op sleep"
    - "io.LimitReader body cap mirroring the ingest handler's http.MaxBytesReader discipline (OOM guard the TS got implicitly from Apps Script)"
    - "buildinfo as its own package (not package main) so politefetch imports Version without an import cycle; mirrors the watcher's main.Version ldflags pattern"

key-files:
  created:
    - "internal/backendsrv/buildinfo/buildinfo.go"
    - "internal/backendsrv/buildinfo/buildinfo_test.go"
    - "internal/backendsrv/enrich/politefetch/politefetch.go"
    - "internal/backendsrv/enrich/politefetch/politefetch_test.go"
  modified: []

key-decisions:
  - "If-Modified-Since SENT in addition to If-None-Match (an ADD beyond the TS, which sent only If-None-Match) — SC-3 names If-Modified-Since and the wiki emits Last-Modified, so sending both maximizes 304 hits"
  - "The 1-second inter-request wiki sleep is deliberately NOT in the client — it lives in the wiki job (12-04) between page fetches; politeFetch only sleeps between retries of a single failing call"
  - "Retry-After parsed as integer delta-seconds ONLY (TS parity — no HTTP-date form), clamped [0,600] so a hostile server can't make us sleep for hours"
  - "var Version='dev' default (D-11) keeps an un-stamped build's UA valid + identifying (the GitHub URL is the contactable reference); wiring the actual -ldflags into the deploy build is deferred to 12-05 / the deploy step"
  - "maxResponseBytes + httpClient + sleepFn are package vars with test-only seams (setMaxResponseBytesForTest) so the cap/backoff are observable without serving 16MB or waiting real seconds"

requirements-completed: [ENRICH-10, ENRICH-11]   # contributes-to: the politeness controls serve both jobs; REQUIREMENTS.md checkboxes stay unchecked until the jobs+scheduler land (12-04/12-05)

# Metrics
duration: ~5min (executor interrupted by a socket close after the 4 task commits; metadata backfilled by the orchestrator)
completed: 2026-05-30
---

# Phase 12 Plan 03: politeFetch Go Port + Backend Version Summary

**`politeFetch.ts` ported verbatim to a Go `net/http` polite client at `internal/backendsrv/enrich/politefetch/` carrying every politeness control (identifying UA, If-None-Match/If-Modified-Since→304 short-circuit, `[2s,4s,8s,16s,32s]` backoff honoring Retry-After), plus the two REQUIRED Go-side additions (`io.LimitReader` OOM cap + a ctx-aware sleep seam), and a backend `buildinfo.Version` var feeding the User-Agent.**

> **Provenance note:** the executor agent completed all four task commits but its terminal returned `API Error: socket connection closed` before it wrote this SUMMARY or updated STATE/ROADMAP (the documented Windows stdio-hang pattern). This SUMMARY was reconstructed by the orchestrator from the committed source + commit log; the code was independently re-verified green (`go build ./...` exit 0, `go vet ./internal/backendsrv/...` exit 0, `go test ./internal/backendsrv/enrich/politefetch/... ./internal/backendsrv/buildinfo/...` ok). No code was changed during backfill.

## Performance
- **Duration:** ~5 min of executor work (task commits 21:07→21:11 local) before the socket close; metadata backfilled post-hoc.
- **Completed:** 2026-05-30
- **Tasks:** 2 (TDD: each a test→feat pair)
- **Files created:** 4 (2 buildinfo + 2 politefetch)

## Accomplishments
- **`internal/backendsrv/buildinfo`** — `Version` (link-time settable via `-ldflags "-X .../buildinfo.Version=..."`, `"dev"` fallback) + `UserAgent()` returning the exact Apps Script UA shape `SquireBot/<Version> (+https://github.com/boejowen/SquireBot)`. Its own package so politefetch imports it without an import cycle (mirrors the watcher's `main.Version`).
- **`internal/backendsrv/enrich/politefetch`** — the full 12-control polite client (enumerated verbatim in the package doc comment), as a `Fetch` function behind a `Fetcher` seam so the jobs (12-04) can inject a fake fetcher in tests:
  1. identifying User-Agent on every request; 2/2b. conditional `If-None-Match` + `If-Modified-Since`; 3. 304 → `FromCache=true`, empty body (skip re-parse, Pitfall 6); 4. 200 captures response ETag + Last-Modified; 5. exponential backoff `[2s,4s,8s,16s,32s]`; 6. integer `Retry-After` honored (clamp 0-600); 7. retry only `{429,503,504}`; 8. non-retriable statuses surface immediately (no sleep); 9. transport errors retry the schedule; 10. the 1s inter-request sleep is the wiki job's, not the client's; 11. TLS verification ON; 12. redirects followed.
- **Two REQUIRED Go-side additions:** `io.LimitReader(resp.Body, 16<<20)` so a runaway response can't OOM the VPS, and a ctx-aware `sleepFn` so a backoff wait aborts promptly on shutdown (and tests run instantly).
- **httptest-driven tests** cover 200 (ETag/Last-Modified capture), 304 (FromCache/skip), 429/503/504 + Retry-After (backoff honored), non-retriable 4xx (immediate, no sleep), oversized body (LimitReader cap), and ctx-cancel during backoff. `go test` green for both new packages.

## Task Commits
Each task committed atomically (TDD RED→GREEN):
1. **Task 1: backend buildinfo (Version + UserAgent)** — `1a32144` (test, RED) → `33aa224` (feat, GREEN)
2. **Task 2: politeFetch net/http client (12 controls)** — `0aefd01` (test, RED) → `124c55b` (feat, GREEN)

**Plan metadata:** this SUMMARY + the STATE/ROADMAP updates were committed by the orchestrator after the socket-close interrupted the executor's own metadata step.

## Files Created/Modified
- `internal/backendsrv/buildinfo/buildinfo.go` — `Version` var + `UserAgent()` helper.
- `internal/backendsrv/buildinfo/buildinfo_test.go` — UA shape + default-version assertions.
- `internal/backendsrv/enrich/politefetch/politefetch.go` — `Fetch`/`Fetcher`/`FetchResult`/`Options` + `parseRetryAfter` (clamp) + `realSleep` (ctx-aware) + the `retryDelays`/`retryStatuses`/`maxResponseBytes` constants.
- `internal/backendsrv/enrich/politefetch/politefetch_test.go` — the httptest matrix above.

## Decisions Made
(See `key-decisions` frontmatter — If-Modified-Since ADD per SC-3; 1s sleep belongs to the job; Retry-After integer-seconds-only TS parity; buildinfo as a separate package; ldflags wiring deferred to the deploy step.)

## Deviations from Plan
The executor's own deviation report was lost to the socket close. Post-hoc orchestrator review of the committed code found it faithful to the plan with all 12 controls present, the two REQUIRED Go additions in place, and `database/sql`/inline-SQL correctly absent (this is the HTTP client tier). No fix-up commits are present in the log and the code is green; any cosmetic doc-grep rewordings (as seen in sibling plans 12-01/12-02) are not separately recorded. **No code was changed during the metadata backfill.**

## Issues Encountered
- The executor terminal hit `API Error: socket connection closed` after committing all four task commits but before its metadata step — the Windows stdio-hang pattern both GSD workflows document. Recovered per the workflow's completion-signal fallback: spot-checked commits + files present, re-ran build/vet/test green, backfilled the SUMMARY + tracking. No work lost.

## User Setup Required
None — pure stdlib HTTP client + a build-time version var. The `-ldflags -X` version stamping is wired at deploy time (12-05 / deploy).

## Next Phase Readiness
- **12-04 (jobs)** can now compose `politefetch.Fetcher` with the 12-02 parsers + 12-01 store methods: each job reads `store.GetETag(url)` → `politefetch.Fetch(ctx, url, Options{ETag,LastModified})` → on 200 parse + upsert + `store.SetETag`; on 304 skip. The wiki job adds the 1s inter-request sleep between page fetches.
- Wave 1 is now COMPLETE (12-01 store/schema, 12-02 parsers, 12-03 politefetch). Wave 2 (12-04) is unblocked.

## Self-Check: PASSED (orchestrator-verified)
- All 4 Go files exist on disk; both new packages compile.
- All 4 task commits present in git history: `1a32144`, `33aa224`, `0aefd01`, `124c55b` (each `git cat-file -t` → commit).
- `go build ./...` exit 0; `go vet ./internal/backendsrv/...` exit 0; `go test ./internal/backendsrv/enrich/politefetch/... ./internal/backendsrv/buildinfo/...` → ok.

---
*Phase: 12-enrichment-job-migration*
*Completed: 2026-05-30 (SUMMARY backfilled by orchestrator after executor socket-close; code unchanged)*
