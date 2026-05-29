---
phase: 11-backend-foundation-ingest-api
plan: 05
subsystem: api
tags: [go, net-http, servemux, ingest, bearer-auth, sqlite, transaction, scheduler, slog, cli, cross-compile]

# Dependency graph
requires:
  - phase: 11-backend-foundation-ingest-api (Plan 01)
    provides: "D-01 verdict HAND-ROLLED Go (net/http ServeMux 1.22+ + time.Ticker, NOT PocketBase); spike-tree + pocketbase-dep cleanup chores"
  - phase: 11-backend-foundation-ingest-api (Plan 02)
    provides: "store.Open (modernc *sql.DB, single-writer DSN), store.NewTestDB fixture, migrations.RunMigrations (goose.Up on startup)"
  - phase: 11-backend-foundation-ingest-api (Plan 03)
    provides: "store.bindCharacter / ReplaceInventory / ReplaceSpellbook (*sql.Tx-based atomic replace + first-sighting bind + cross-owner audit); parse.Parse / ParseSpellbook (UTF-8)"
  - phase: 11-backend-foundation-ingest-api (Plan 04)
    provides: "auth.New(db) + (*auth.Auth).resolveToken bearer guard (SHA-256 + constant-time compare); auth.MintCode / RevokeCode"
provides:
  - "POST /api/v1/ingest handler (ingest.Handler/New): MaxBytesReader body cap -> bearer guard FIRST (401 writes nothing) -> envelope decode+validate (4xx) -> UTF-8 parse -> first-sighting bind + atomic replace in ONE *sql.Tx -> 204; cross-owner -> 409 (audit row persisted)"
  - "ingest.Envelope (D-04) + DecodeAndValidate: required-field + kind-enum validation with typed sentinel errors (ErrMalformedJSON / ErrMissingCharacter / ErrInvalidKind)"
  - "cmd/squirebot-server: single Go binary dispatching serve | mint-code | revoke-code; goose.Up on startup; loopback bind; graceful shutdown; verdict-driven net/http ServeMux"
  - "scheduler.Start(ctx): in-process time.Ticker scheduler SKELETON (no real jobs until P12)"
  - "internal/backendsrv/logging.Setup(): Linux stdout JSON slog handler (journald-captured), distinct from the watcher's lumberjack-to-LOCALAPPDATA"
  - "Exported single-tx entry points (no second SQL path): auth.(*Auth).ResolveToken, store.BindCharacter, store.ReplaceInventoryTx, store.ReplaceSpellbookTx (public Store.Replace* now delegate)"
  - "PocketBase dependency + spike/ tree REMOVED (go mod tidy clean); the production server carries zero pre-1.0 framework"
affects: [11-06, 11-07, 12-enrichment-job-migration, 13-watcher-re-target, 14-web-frontend, 15-admin-web-forms-login]

# Tech tracking
tech-stack:
  added: []  # stdlib only (net/http, flag, os/signal); REMOVED github.com/pocketbase/pocketbase + dbx
  patterns:
    - "Guard-first request ordering: http.MaxBytesReader -> auth guard (401 + RETURN before ANY store call) -> validate -> parse -> ONE-tx bind+replace. The 401-writes-nothing guarantee is structural (the guard returns before the first BeginTx)."
    - "Single tested SQL path via Tx-delegation: the public Store.ReplaceInventory/ReplaceSpellbook now BeginTx+Commit around exported ReplaceInventoryTx/ReplaceSpellbookTx; the ingest handler calls the same Tx functions inside its own one tx — so 11-03's store tests are the single coverage for the atomic-replace + cross-owner-reject logic (no second, test-uncovered SQL path)."
    - "Cross-owner reject COMMITS the tx (not rollback) so the in-tx audit_log row is durable even though the ingest is refused (D-07/V4) — the only write on that path is the audit INSERT; mirrors 11-03's bindInTx contract."
    - "net/http ServeMux 1.22+ method+pattern routing ('POST /api/v1/ingest') — no router dependency (D-02, FALLBACK verdict)."
    - "time.Ticker goroutine scheduler skeleton tied to ctx (signal.NotifyContext) — mirrors internal/heartbeat ergonomics; NOT app.Cron()."
    - "Linux server logs JSON to os.Stdout (journald) — same slog handler shape as the watcher, different sink (no lumberjack/LOCALAPPDATA)."
    - "Testable CLI entrypoint: run([]string) int with os.Exit(run(os.Args[1:])) so mint/revoke dispatch is unit-tested without spawning a process."

key-files:
  created:
    - "cmd/squirebot-server/main.go"
    - "cmd/squirebot-server/main_test.go"
    - "internal/backendsrv/ingest/envelope.go"
    - "internal/backendsrv/ingest/envelope_test.go"
    - "internal/backendsrv/ingest/handler.go"
    - "internal/backendsrv/ingest/handler_test.go"
    - "internal/backendsrv/scheduler/scheduler.go"
    - "internal/backendsrv/scheduler/scheduler_test.go"
    - "internal/backendsrv/logging/logging.go"
  modified:
    - "internal/backendsrv/auth/guard.go (added exported ResolveToken wrapper)"
    - "internal/backendsrv/store/replace.go (extracted exported ReplaceInventoryTx/ReplaceSpellbookTx; public methods delegate)"
    - "internal/backendsrv/store/binding.go (added exported BindCharacter wrapper)"
    - "go.mod / go.sum (dropped pocketbase + dbx; go mod tidy)"
    - ".gitignore (removed dead spike entries)"
    - "spike/pocketbase/ (DELETED)"

key-decisions:
  - "VERDICT CONFIRMED FALLBACK (from 11-01): route = net/http ServeMux 'POST /api/v1/ingest'; cron = time.Ticker goroutine; no PocketBase APIs. Dropped the pocketbase + dbx deps and deleted spike/pocketbase/, then go mod tidy (0 pocketbase refs in go.mod/go.sum)."
  - "Tx-reuse wired via shared-store *sql.Tx (option a): extracted ReplaceInventoryTx/ReplaceSpellbookTx that the public Store.Replace* delegate to; the handler runs BindCharacter + Replace*Tx inside ONE db.BeginTx — no inline DELETE/INSERT SQL in handler.go (grep-confirmed). 11-03's tests remain the single coverage."
  - "Exported ResolveToken / BindCharacter wrappers (Rule 2): 11-04 left resolveToken / 11-03 left bindCharacter package-unexported by design; the ingest handler is a separate package, so thin exported wrappers delegate to the unchanged internals (one tested path, security behavior identical)."
  - "Cross-owner 409 commits the tx to persist the audit row (Rule 1 fix): a blanket defer tx.Rollback() discarded the audit_log trace; corrected so ErrCharOwnedByAnother commits (audit is the only write) while all other errors roll back."
  - "Server binds 127.0.0.1:8090 only (Caddy fronts 443 in 11-06); flags --addr/--db match the RESEARCH systemd invocation; goose.Up on startup (D-10); NO Google/OAuth/Sheets dep (go list -deps confirms zero)."
  - "Body cap = http.MaxBytesReader(w, r.Body, 1<<20) before decode (V5; a maxed char snapshot is <50 KB); oversized body surfaces as a decode error -> 400, no write."

patterns-established:
  - "Pattern: compose verdict-agnostic packages (auth/parse/store) behind a thin net/http handler; the handler authors NO SQL — it only orchestrates exported Tx functions in one transaction."
  - "Pattern: 401/4xx happen BEFORE the first store call, so every rejection writes nothing by construction (proven by row-count-unchanged tests)."
  - "Pattern: in-process scheduler skeleton is a ctx-bound time.Ticker that P12 fills with real jobs; shutdown is root-context cancellation."

requirements-completed: [BACKEND-01, BACKEND-03, BACKEND-04]

# Metrics
duration: 27 min
completed: 2026-05-29
---

# Phase 11 Plan 05: Verdict-Dependent HTTP/Hosting Shell (POST /api/v1/ingest + cmd/squirebot-server) Summary

**A runnable single Go binary that serves the project's first authenticated network surface: `POST /api/v1/ingest` composes the 11-02/03/04 pieces (MaxBytesReader body cap -> bearer guard FIRST/401-writes-nothing -> envelope validation -> UTF-8 parse -> first-sighting bind + atomic full-snapshot replace in ONE `*sql.Tx` -> 204; cross-owner -> 409 with a durable audit row), reusing 11-03's `*sql.Tx`-based bind+replace so there is ONE tested SQL path; `cmd/squirebot-server` dispatches `serve`/`mint-code`/`revoke-code` and runs `goose.Up` on startup with a `time.Ticker` scheduler skeleton — all in the HAND-ROLLED `net/http` form the 11-01 verdict dictated, PocketBase dependency + spike tree removed, with the static `linux/amd64` deploy binary cross-compiling clean.**

## Resolved Verdict (the load-bearing branch)

**VERDICT = HAND-ROLLED Go fallback** (read from `11-01-SUMMARY.md`: "VERDICT: HAND-ROLLED Go fallback — net/http ServeMux 1.22+ ... Do NOT adopt PocketBase"). Consequently every Task 2/3 wiring took the FALLBACK form:

- **Route:** stdlib `net/http` ServeMux with Go 1.22+ method+pattern routing — `mux.Handle("POST /api/v1/ingest", ingest.New(auth.New(db), db))`. No `app.OnServe().BindFunc` / `e.Router.POST(...).Bind(...)`.
- **Cron:** a single `time.Ticker` goroutine (`scheduler.Start(ctx)`) registering a no-op heartbeat. No `app.Cron().MustAdd`.
- **Transaction:** the shared `store` `*sql.DB` owns one `db.BeginTx` (BEGIN IMMEDIATE via the 11-02 DSN); the handler calls 11-03's `BindCharacter` + `Replace*Tx` over that SAME tx. No `app.RunInTransaction`, no PB-native SQL path.
- **Cleanup:** dropped `github.com/pocketbase/pocketbase` + `pocketbase/dbx` from `go.mod`, deleted `spike/pocketbase/`, ran `go mod tidy` — **0 pocketbase references** in `go.mod`/`go.sum`; `go list -m github.com/pocketbase/pocketbase` errors (not a known dependency).

**Tx-reuse wiring (for 11-06):** shared-store `*sql.Tx` (plan option a). The public `Store.ReplaceInventory`/`ReplaceSpellbook` now `BeginTx`+`Commit` around the extracted exported `ReplaceInventoryTx`/`ReplaceSpellbookTx`; the ingest handler runs `store.BindCharacter` + `store.Replace*Tx` inside its own single `db.BeginTx`. The handler authors NO inline bind/replace SQL (grep-confirmed) — 11-03's store tests are the single coverage for the atomic-replace + cross-owner-reject logic.

## Performance

- **Duration:** 27 min
- **Started:** 2026-05-29T22:05Z (after 11-04)
- **Completed:** 2026-05-29T22:32Z
- **Tasks:** 3
- **Files modified:** 17 (9 created, 6 modified, 2 deleted — the spike tree)

## Accomplishments

- **POST /api/v1/ingest (BACKEND-03 delivery + BACKEND-04 401-writes-nothing):** ordered request flow — `http.MaxBytesReader(w, r.Body, 1<<20)` (V5/DoS) -> `auth.ResolveToken` FIRST (`!ok` -> 401 and RETURN before any store call) -> `DecodeAndValidate` (4xx) -> `parse.Parse`/`ParseSpellbook` on `strings.NewReader(content)` (UTF-8, A1 — no CP1252 decode) -> `store.BindCharacter` + `store.Replace*Tx` in ONE `db.BeginTx` -> 204. Cross-owner -> 409 (tx committed so the `audit_log` row persists); other failures roll back. Round-trip proven via httptest: valid upload -> rows queryable; shrinking 3->1 snapshot drops rows; empty content is a no-op that clears rows.
- **Envelope validation (V5):** `ingest.Envelope` (D-04 shape verbatim) + `DecodeAndValidate` with typed sentinels (`ErrMalformedJSON` -> 400, `ErrMissingCharacter`/`ErrInvalidKind` -> 422); empty content allowed (no-op snapshot); unknown fields ignored (forward-compatible). Never logs raw content or token (V7).
- **cmd/squirebot-server single binary (BACKEND-01 "single binary + in-process scheduler" half):** testable `run([]string) int` dispatching `mint-code --owner <label>` / `revoke-code <id|label>` (both Open + `goose.Up` + act + exit early) and the default `serve --addr 127.0.0.1:8090 --db ...` (logging.Setup stdout slog -> `store.Open` -> `RunMigrations` (goose.Up on startup, D-10) -> `scheduler.Start(ctx)` -> ServeMux on loopback -> graceful shutdown on SIGINT/SIGTERM). NO Google/OAuth/Sheets dep anywhere (`go list -deps` confirms zero).
- **In-process scheduler skeleton:** `scheduler.Start(ctx)` launches one `time.Ticker` goroutine that logs a heartbeat and returns cleanly on ctx cancel — no real jobs (P12 fills it in).
- **Linux stdout logging:** `internal/backendsrv/logging.Setup()` builds the watcher's JSON slog handler shape writing to `os.Stdout` (journald), NOT lumberjack-to-LOCALAPPDATA.
- **Deploy binary:** `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/squirebot-server` produces a static ELF (`7f 45 4c 46`, go1.26.2) — 11-06 ships it.
- **No regression:** FULL `go test ./...` green (every watcher AND backend package, incl. new `ingest`/`scheduler`/`cmd/squirebot-server`); `go build ./...` + `go vet ./...` clean.

## Task Commits

Each task was committed atomically:

1. **Verdict cleanup (Task 1 prereq): remove PocketBase dep + spike tree** — `e45f2c5` (chore)
2. **Task 1: Linux stdout slog + ingest envelope decode/validation** — `44cc28d` (feat)
3. **Task 2: POST /api/v1/ingest handler (guard-first, one-tx bind+replace)** — `bec40e0` (feat)
4. **Task 3: cmd/squirebot-server entrypoint + scheduler skeleton** — `ffb5ddf` (feat)

**Plan metadata:** committed separately (this SUMMARY + STATE + ROADMAP + REQUIREMENTS).

## Files Created/Modified

- `internal/backendsrv/logging/logging.go` (NEW) — `Setup()` JSON slog to os.Stdout (journald), Linux server analog of the watcher's lumberjack logger.
- `internal/backendsrv/ingest/envelope.go` (NEW) — `Envelope` (D-04) + `DecodeAndValidate` (required fields, kind enum, typed sentinel errors); empty content allowed; never logs content/token.
- `internal/backendsrv/ingest/envelope_test.go` (NEW) — table test: valid inv/spellbook, missing/empty char, empty/bad kind, malformed JSON, empty content, unknown fields, json-tag mapping.
- `internal/backendsrv/ingest/handler.go` (NEW) — `Handler`/`New` + `ServeHTTP`: the ordered flow; `bindAndReplace` runs `BindCharacter` + `Replace*Tx` in one tx (cross-owner commits the audit row, else rolls back); `parseContent`; `writeEnvelopeError` 4xx mapping. No inline SQL.
- `internal/backendsrv/ingest/handler_test.go` (NEW) — httptest + NewTestDB + MintCode round-trip tests: valid inv/spellbook rows queryable, 401-writes-nothing (no header / unknown / revoked), shrinking snapshot, cross-owner 409 + audit + untouched, bad-kind/malformed/oversized 4xx no-write, empty-content no-op.
- `internal/backendsrv/scheduler/scheduler.go` (NEW) — `Start(ctx)` + `run(ctx)`: time.Ticker skeleton, returns on ctx cancel, no real jobs.
- `internal/backendsrv/scheduler/scheduler_test.go` (NEW) — returns-on-cancel + non-blocking-Start smoke tests.
- `cmd/squirebot-server/main.go` (NEW) — `main` -> `run([]string) int`; mint/revoke/serve dispatch; goose.Up on startup; loopback ServeMux; graceful shutdown; `splitFlagsAndPositionals` (position-independent revoke flags).
- `cmd/squirebot-server/main_test.go` (NEW) — `run()` mint/revoke dispatch against a temp DB (exit codes + persisted/disabled rows) + usage-error cases.
- `internal/backendsrv/auth/guard.go` (MOD) — added exported `(*Auth).ResolveToken` delegating to `resolveToken` (one tested path).
- `internal/backendsrv/store/replace.go` (MOD) — extracted exported `ReplaceInventoryTx`/`ReplaceSpellbookTx`; public `ReplaceInventory`/`ReplaceSpellbook` now delegate (same SQL body).
- `internal/backendsrv/store/binding.go` (MOD) — added exported `BindCharacter` delegating to `bindCharacter`.
- `go.mod` / `go.sum` (MOD) — dropped `pocketbase` + `dbx`; `go mod tidy`.
- `.gitignore` (MOD) — removed dead `pb_data/` + `spike-amd64` entries.
- `spike/pocketbase/main.go` + `README.md` (DELETED) — throwaway 11-01 harness.

## Decisions Made

- **Shared-store `*sql.Tx` for the one-transaction reuse** (plan's preferred shape, option a). The atomic-replace + cross-owner-reject logic stays owned by 11-03; the handler composes it via exported Tx functions over one `db.BeginTx`. This keeps a single SQL path (11-03's tests cover it through the delegating public methods) and avoids a second, test-uncovered SQL path — the explicit WARNING-3 constraint.
- **Exported `ResolveToken`/`BindCharacter` thin wrappers** rather than exporting/renaming the internals — the 11-03/11-04 package-internal functions and their direct tests stay intact; the wrappers are the only cross-package surface (security behavior unchanged).
- **204 No Content on success** (the ingest produces no response body); 409 carries a clear human message for cross-owner; 401/4xx are terse and leak nothing.
- **`mint-code`/`revoke-code` run `goose.Up`** so a fresh box can mint a guild code before the first `serve` (the schema exists either way; goose is idempotent).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added exported cross-package entry points (`auth.ResolveToken`, `store.BindCharacter`, `store.ReplaceInventoryTx`/`ReplaceSpellbookTx`)**
- **Found during:** Task 2 (handler)
- **Issue:** `resolveToken` (11-04) and `bindCharacter` (11-03) are package-unexported, and `Store.ReplaceInventory`/`ReplaceSpellbook` open their OWN `BeginTx`. The ingest handler lives in a different package (`ingest`) and the plan's load-bearing constraint requires bind+replace in ONE tx via the SAME 11-03 functions — none of which is reachable as written.
- **Fix:** Added thin exported wrappers (`ResolveToken`, `BindCharacter`) that delegate UNCHANGED to the internals, and extracted exported `*sql.Tx`-taking `ReplaceInventoryTx`/`ReplaceSpellbookTx` that the public methods now delegate to. The handler composes `BindCharacter` + `Replace*Tx` over one `db.BeginTx`. No new SQL authored; 11-03's tests still cover the single path (the public methods call the same Tx bodies).
- **Files modified:** `internal/backendsrv/auth/guard.go`, `internal/backendsrv/store/binding.go`, `internal/backendsrv/store/replace.go`
- **Verification:** `go build ./...` + `go vet ./...` clean; store + auth tests still green (single path intact); grep confirms NO inline DELETE/INSERT/character SQL in `handler.go`. 11-04's SUMMARY explicitly anticipated this one-line export.
- **Committed in:** `bec40e0` (Task 2 commit)

**2. [Rule 1 - Bug] Cross-owner 409 must COMMIT the tx so the audit_log row is durable**
- **Found during:** Task 2 (handler — `TestIngest_CrossOwner_409` failed: `audit_log cross_owner_reject rows = 0, want 1`)
- **Issue:** `BindCharacter` writes the cross-owner audit row INSIDE the tx then returns `ErrCharOwnedByAnother`. My handler's blanket `defer tx.Rollback()` rolled the whole tx back — discarding the audit trace. The 409 + "A's rows untouched" assertions passed, but the durable audit record (D-07/V4/T-11.03-05) was silently lost.
- **Fix:** Restructured `bindAndReplace` so the `ErrCharOwnedByAnother` branch COMMITS the tx (the only write on that path is the audit INSERT — owner_id is never overwritten, no rows mutated), while all other bind/replace errors `tx.Rollback()`. This mirrors 11-03's own `bindInTx` test helper ("commit anyway so the audit row is durable ... even though the ingest itself is rejected").
- **Files modified:** `internal/backendsrv/ingest/handler.go`
- **Verification:** `TestIngest_CrossOwner_409` now passes (409 + 1 audit row + A's 3 rows untouched); full ingest suite green.
- **Committed in:** `bec40e0` (Task 2 commit)

**3. [Rule 1 - Bug] revoke-code flag parsing was position-dependent**
- **Found during:** Task 3 (`TestRun_RevokeDispatch` failed: "unable to open database file (14)")
- **Issue:** `revoke-code bob --db <path>` — Go's `flag` package stops at the first non-flag token (`bob`), leaving `--db <path>` unparsed, so the command silently used the default DB path (`/var/lib/squirebot/...`) which does not exist on the dev box.
- **Fix:** Added `splitFlagsAndPositionals` so the id/label positional can appear before OR after the flags; `flag.Parse` then receives only the flag tokens. (Mint uses `--owner` and serve uses only flags, so revoke was the sole position-sensitive command.)
- **Files modified:** `cmd/squirebot-server/main.go`
- **Verification:** all 4 CLI tests pass, including the flag-after-positional ordering.
- **Committed in:** `ffb5ddf` (Task 3 commit)

---

**Total deviations:** 3 auto-fixed (1 missing-critical, 2 bugs).
**Impact on plan:** No scope creep. (1) is the minimal cross-package surface the plan's one-tx constraint requires (anticipated by 11-04). (2) and (3) corrected real correctness bugs surfaced by the plan's own required tests (cross-owner audit durability; CLI flag robustness). No production behavior changed beyond these fixes; the single-tested-SQL-path guarantee holds.

## Issues Encountered

None beyond the three auto-fixed deviations above (all surfaced and resolved within their tasks via the plan's mandated tests). The cross-compile ELF-magic verification was momentarily obscured by a PowerShell-via-bash quoting error, resolved by reading the bytes with `od` (confirmed `7f 45 4c 46`); the verification artifact was deleted (never committed).

## Threat Model Compliance

All `mitigate`-disposition threats from the plan's STRIDE register are covered and test-proven:

- **T-11.05-01 (unauthenticated write reaches the store):** `ResolveToken` is called FIRST; `!ok` -> 401 and RETURN before any store call. `TestIngest_NoAuthHeader_401_WritesNothing` (+ unknown/revoked) prove the row count stays 0.
- **T-11.05-02 (oversized payload DoS):** `http.MaxBytesReader(w, r.Body, 1<<20)` before decode. `TestIngest_OversizedBody_413or400` proves rejection with no write.
- **T-11.05-03 (malformed/unknown-kind tampering):** `DecodeAndValidate` enforces required fields + kind enum before parse; `TestIngest_BadKind_4xx` / `TestIngest_MalformedJSON_4xx` prove 4xx with no write.
- **T-11.05-04 (cross-owner overwrite):** `BindCharacter` -> `ErrCharOwnedByAnother` -> 409; the audit row is committed; A's rows untouched. `TestIngest_CrossOwner_409` proves all three.
- **T-11.05-05 (content/token leak in logs):** handler slog calls log operation + status + char name only — never raw content or the Authorization value (grep-confirmed; runtime log output verified).
- **T-11.05-06 (unauthenticated data-leaking route):** the only registered route is the authenticated ingest; no debug/health route returns data.
- **T-11.05-07 (Google secret baked into backend):** `go list -deps ./cmd/squirebot-server` returns ZERO google/oauth2/sheets imports; no `-X main.OAuthClientID` ldflag; grep-confirmed.
- **T-11.05-08 (second, test-uncovered SQL path):** the handler reuses 11-03's `*sql.Tx` functions over one tx; no inline DELETE/INSERT SQL in `handler.go` (grep-confirmed). 11-03's tests remain the single coverage.

ASVS L1 in this plan: V2 (401-writes-nothing wiring complete — guard before store), V5 (body cap + envelope validation + kind enum), V7 (no raw content/token logging). V4 (cross-owner 409) and V6 (token crypto) inherited from 11-03/11-04 and composed here. TLS (V6) is Caddy's job in 11-06.

## Known Stubs

The `scheduler` is an intentional SKELETON per BACKEND-01 / the plan: `scheduler.Start(ctx)` registers NO real jobs — the heartbeat tick is a placeholder proving the loop fires and shuts down cleanly. P12 fills in the real PigParse/wiki enrichment jobs. This is the locked design (RESEARCH "Don't Hand-Roll": do not build a general scheduler in P11), not an unfinished stub. No other stubs.

## User Setup Required

None - no external service configuration required for this plan. All tests run in CI on the Windows dev box (pure-Go modernc, no cgo, no live box). The on-box run (systemd/Caddy/firewall/TLS) is 11-06; the A-record for `api.squirebot.quest` is the pending 11-06 chore.

## Next Phase Readiness

- **BACKEND-01 (single binary + in-process scheduler half), BACKEND-03 (HTTP delivery), BACKEND-04 (401-writes-nothing wiring) satisfied at the build/test tier.** `cmd/squirebot-server` runs `serve`/`mint-code`/`revoke-code`, applies migrations on startup, and serves the authenticated ingest on loopback; the static `linux/amd64` binary cross-compiles clean.
- **11-06 (deploy/TLS/systemd/firewall)** can now: cross-compile `./cmd/squirebot-server` (`CGO_ENABLED=0 GOOS=linux GOARCH=amd64`), scp it to the Hetzner box, wire the systemd unit (`serve --addr 127.0.0.1:8090 --db /var/lib/squirebot/squirebot.db`), front it with Caddy (HTTP-01 cert for `api.squirebot.quest`), and add the pending DNS A-record. The tx-reuse is shared-store `*sql.Tx` (no PB path to account for).
- **11-07 (backup)** builds on the same DB file + binary.
- **P12** fills the `scheduler` skeleton with real enrichment jobs; **P13** points the watcher at `POST /api/v1/ingest` (must CP1252->UTF-8 decode via `parse.CP1252Reader` before POSTing `content` — do NOT double-decode).
- **No blockers.** Phase 11 is now 5/7 plans complete (Wave 4 done; Waves 5-6 = 11-06/11-07 deploy + backup remain).

## Self-Check: PASSED

- **Files on disk:** all 9 created source files FOUND (`cmd/squirebot-server/main.go` + `main_test.go`, `ingest/envelope.go` + `_test.go`, `ingest/handler.go` + `_test.go`, `scheduler/scheduler.go` + `_test.go`, `logging/logging.go`) + `11-05-SUMMARY.md` FOUND; `spike/` tree GONE (correct).
- **Commits exist:** `e45f2c5` (verdict cleanup chore), `44cc28d` (Task 1 feat), `bec40e0` (Task 2 feat), `ffb5ddf` (Task 3 feat) — all FOUND.
- **Plan `<verification>` re-run:**
  - `go test ./internal/backendsrv/ingest/... ./internal/backendsrv/scheduler/...` exit 0.
  - `go build ./cmd/squirebot-server/` exit 0 (host); FULL `go build ./...` + `go vet ./...` exit 0.
  - FULL `go test ./...` exit 0 — every watcher AND backend package green (no v1 regression).
  - `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/squirebot-server` exit 0 → static ELF (`7f 45 4c 46`, go1.26.2).
  - Round-trip (valid Bearer + inventory POST -> 2xx + rows queryable; shrinking 3->1 drops rows) PROVEN; 401-writes-nothing (no header/unknown/revoked -> 401, count unchanged) PROVEN; malformed/bad-kind/oversized -> 4xx no-write PROVEN; cross-owner -> 409 + audit + untouched PROVEN.
  - `handler.go` contains `MaxBytesReader`; `ResolveToken` precedes the first `store.` call; NO inline `DELETE/INSERT inventory_item|spellbook_entry` or `INSERT INTO character` SQL in `handler.go` (grep exit 1). No raw content/Authorization passed to slog.
  - `cmd/squirebot-server/main.go` contains `RunMigrations` (`GOOSE_ON_STARTUP_OK`); dispatches `mint-code`/`revoke-code`/`serve`; `--addr` default `127.0.0.1:8090` + `--db`; NO Google/OAuth/Sheets refs (only the "Off Google" doc comment), `go list -deps` shows zero google/oauth2/pocketbase deps.
  - pocketbase: 0 refs in `go.mod`/`go.sum`; `go list -m github.com/pocketbase/pocketbase` → "not a known dependency".

---
*Phase: 11-backend-foundation-ingest-api*
*Completed: 2026-05-29*
