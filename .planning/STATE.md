---
gsd_state_version: 1.0
milestone: v2.0
milestone_name: — "Off Google" — Website Frontend
status: executing
last_updated: "2026-05-29T22:32:00.000Z"
last_activity: 2026-05-29 -- Plan 11-05 complete (POST /api/v1/ingest handler + cmd/squirebot-server entrypoint + scheduler skeleton; BACKEND-01/03/04 wiring; PocketBase dep + spike tree removed)
progress:
  total_phases: 6
  completed_phases: 0
  total_plans: 7
  completed_plans: 5
  percent: 71
---

# State: SquireBot

**Initialized:** 2026-04-30
**Last updated:** 2026-05-28 (v2.0 "Off Google" Website Frontend ROADMAP created — Phases 11–16)

## Project Reference

See: `.planning/PROJECT.md` (updated 2026-05-28 with v2.0 milestone scope)

- **Core value:** Every guildie can answer "what does my character still need, and where in the guild is it?" — now delivered via a self-hosted website instead of the Google Sheet.
- **Current focus:** Milestone v2.0 "Off Google" — replace the Google Sheet (UI + data store) with a self-hosted Go + SQLite backend (Hetzner Cloud VPS, US) + static web frontend (SvelteKit), eliminating the Google OAuth dependency. Phases 11–16. ≈ $67/yr (~$55/yr VPS + ~$12/yr domain).
- **Mode:** yolo
- **Granularity:** coarse

## Current Position

Phase: 11 — Backend Foundation + Ingest API (in progress)
Plan: 11-05 complete (5/7) → next 11-06 (deploy: Hetzner provisioning + Caddy + systemd + TLS + firewall + DNS A-record)
Status: Executing — BACKEND-01 (single-binary + in-process-scheduler half) / BACKEND-03 (HTTP delivery) / BACKEND-04 (401-writes-nothing wiring) done at the build/test tier. `cmd/squirebot-server` is a runnable single Go binary dispatching serve|mint-code|revoke-code, running goose.Up on startup, serving `POST /api/v1/ingest` on loopback (127.0.0.1:8090) with a time.Ticker scheduler skeleton — all HAND-ROLLED net/http (11-01 FALLBACK verdict), PocketBase dep + `spike/` tree REMOVED (go mod tidy clean). The handler composes the pieces in load-bearing order: MaxBytesReader body cap → bearer guard FIRST (401 + RETURN before any store call) → envelope validate (4xx) → UTF-8 parse → first-sighting bind + atomic replace in ONE *sql.Tx → 204; cross-owner → 409 (audit row committed). Reuses 11-03's *sql.Tx bind+replace via exported Tx wrappers (single tested SQL path; no inline SQL in handler.go). Static linux/amd64 cross-compile verified (ELF). Full `go test ./...` green (no v1 watcher regression); no Google dep anywhere (`go list -deps` = 0).
Last activity: 2026-05-29 -- Plan 11-05 complete (POST /api/v1/ingest + cmd/squirebot-server + scheduler skeleton; BACKEND-01/03/04 wiring; PocketBase removed)

### v2.0 Phase Plan (2026-05-28)

Coverage: 26/26 v2.0 requirements mapped to exactly one phase. No orphans, no duplicates. Full success criteria in `.planning/ROADMAP.md` § Phase Details.

| Phase | Name | Requirements | Stack | Depends on | Status |
|-------|------|--------------|-------|-----------|--------|
| 11 | Backend Foundation + Ingest API | BACKEND-01, BACKEND-02, BACKEND-03, BACKEND-04, BACKEND-06 | Go + SQLite + goose + Caddy (Hetzner Cloud VPS, US, amd64) | — | 🚧 In progress (5/7; 11-05 ingest handler + cmd/squirebot-server + scheduler skeleton done — BACKEND-01/03/04 wired, PocketBase removed; next 11-06 deploy) |
| 12 | Enrichment Job Migration | ENRICH-10, ENRICH-11 | Go in-process scheduler (PigParse + wiki parsers ported) | P11 | Not started |
| 13 | Watcher Re-Target + Onboarding | WATCH-08, WATCH-09, WATCH-10, WATCH-11 | Go watcher (`internal/backend` HTTP client; OAuth/Sheets/Picker deleted) | P11 | Not started |
| 14 | Web Frontend | BACKEND-05, WEB-01, WEB-02, WEB-03, WEB-04, WEB-05 | SvelteKit static + TanStack Table + Tailwind; Go read API | P11 (read API) + P12 (data) | Not started |
| 15 | Admin Web Forms + Login | AUTH-08, AUTH-09, ADMIN-04, ADMIN-05, ADMIN-06 | Discord OAuth2 login; web forms | P14 + P11 | Not started |
| 16 | Cutover + Decommission | CUTOVER-01, CUTOVER-02, CUTOVER-03, CUTOVER-04 | shadow soak + backfill + coordinated self-update flip | P13 + P14 + P15 + P12 | Not started |

**Sequencing rationale (FRONT-LOAD THE INGEST PATH):**

1. **P11 first** because the ~12 guildies are dark on the Sheet during the build (Google walled off their watchers); P11 stands up the authenticated ingest endpoint so data has somewhere to go. No dependencies — it is the foundation.
2. **P12 + P13 next** (both depend only on P11): P12 makes the new DB self-populate its dimension data; P13 re-points the watcher at the ingest API and ships it via the auto-updater — together these restore the live data pipeline end-to-end before any polished UI exists.
3. **P14** (depends on P11 read API + P12 data) rebuilds the visible product — the 4 views, search, tooltips, theming.
4. **P15** (depends on P14 + P11) adds Discord login + the officer-only write forms; must exist before the Sheet's admin sidebars can retire.
5. **P16 last** (depends on P13 + P14 + P15 + P12) — shadow soak, human-data backfill, one coordinated watcher flip, then decommission Sheet + Apps Script + Google OAuth client. Meets the "Off Google" goal.

**Reverted to the research-recommended Hetzner VPS; SQLite retained (not Postgres):** SCOPE.md + the 4 findings docs recommended a Hetzner VPS + PostgreSQL. v2.0 initially overrode the host to Oracle Cloud Always Free for $0, then **reverted to the research-recommended Hetzner Cloud VPS (US, amd64)** on 2026-05-29 — while **keeping SQLite + `goose`** (not the research's Postgres). DDL ports with `CITEXT`→`TEXT COLLATE NOCASE` + identity-syntax changes; `pg_trgm` search → SQLite FTS5 / `LIKE`.

**Schema-evolution change:** the `_meta.schema_version` ↔ `WatcherMaxSchemaVersion` handshake retires in favour of forward-only `goose` DB migrations + an explicit API version (`/api/v1/...`). The watcher's Sheets schema gate is removed in P13.

**Open decision surfaced to P11 (not a separate phase):** optional 1-day **PocketBase** spike at the start of P11 (open-source single Go binary = SQLite + auth + REST + admin UI, self-hosted on the same Hetzner VPS) could compress P11 **and** P15 by ~5–8 days. Evaluate before committing to a hand-rolled Go server; verdict captured in the P11 CONTEXT.

### Superseded: 999.19 Google brand verification

Google REJECTED brand verification 2026-05-15 ("home page not registered to you" — `github.io` can't satisfy it), walling off all ~12 guildie watchers. **v2.0 "Off Google" supersedes this** by removing Google entirely rather than renting a domain to placate it. The guild stays dark on the sheet during the v2.0 build (P11 ingest is front-loaded to restore data flow first). Full incident trail at `.planning/debug/v1-0-2-oauth-invalid-client-incident.md`. Anti-pattern note for future Google-OAuth incidents: check OAuth consent screen Verification status FIRST when seeing `Access blocked / Error 400: invalid_request`; the multiple-secrets / loopback-IP / redirect-URI angles are red herrings.

**Shipped to date:**

| Milestone | Phases | Plans | Tag | Date |
|-----------|--------|-------|-----|------|
| v1.0 — Watcher + Workbook + Onboarding | 5 (Phases 1–5) | 31 | `v1.0.0` | 2026-05-11 |
| v1.0.1 — Installer + Permissions Hardening | 3 (Phases 6–8) | 12 | `v1.0.1` (binary; Phase 6 ship gate) | 2026-05-12 |
| v1.0.2 — Robustness Polish (binary; milestone close superseded by v2.0) | 2 (Phases 9–10) | 8 | `v1.0.2` (binary) | 2026-05-13 |

> v1.0.2 binary shipped but its milestone close was superseded by the v2.0 pivot (the Sheet it targeted is being replaced). Phase directories 09 + 10 exist on disk; v2.0 continues at **Phase 11** (never reuses 9 or 10). v1.0.2 was never written to MILESTONES.md.

## Performance Metrics

| Metric | Value |
|--------|-------|
| Total milestones shipped | 2 fully closed (v1.0, v1.0.1) + 1 binary-only (v1.0.2, close superseded) |
| Total phases shipped | 10 (1–10) |
| Total plans shipped | 51 (31 + 12 + 8) |
| Total commits since init | 266+ (203 v1.0 + 63 v1.0.1 + v1.0.2 work) |
| Watcher LOC (Go) | 14,507 (pre-v2.0; expected to shrink ~2k in P13) |
| Apps-script LOC (TypeScript) | 13,266 (to be decommissioned in P16) |
| Vitest tests | 336/336 passing (apps-script; pre-v2.0) |
| Active blockers | 0 |

## Accumulated Context

### Decisions Log

All locked decisions live in `PROJECT.md` Key Decisions table and the per-milestone archive ROADMAPs. v1.0 contributed 11 PROJECT-level decisions; v1.0.1 contributed 5 more.

**v2.0 milestone-open decisions (locked, see PROJECT.md Current Milestone):**

- **Off Google entirely** — replace the Sheet (UI + data store), not placate Google's brand verification.
- **Backend = Hetzner Cloud VPS (US, amd64) + Caddy + SQLite + `goose`** (host switched Oracle Always Free → Hetzner on 2026-05-29 — reverted to the research-recommended Hetzner VPS, SQLite retained not Postgres; see Phase 11 CONTEXT D-12). Cost ~$55/yr VPS + ~$12/yr domain ≈ $67/yr (the "$0 backend" premise is retired).
- **Backend server = HAND-ROLLED Go (`net/http` ServeMux + `modernc.org/sqlite` + `goose`), NOT PocketBase** (D-01 spike verdict, Plan 11-01, 2026-05-29). All four spike probes passed, but the opaque-token auth + plain-SQL-table design bypasses PocketBase's leverage points; the pre-1.0 churn tax isn't worth it. P15 Discord OAuth2 (AUTH-08) is consequently hand-rolled `golang.org/x/oauth2`, not PB's built-in provider.
- **Frontend = SvelteKit static (adapter-static) + TanStack Table + Tailwind** on Cloudflare/GitHub Pages.
- **Watcher↔backend auth = per-guildie opaque bearer token** ("guild code"), hashed server-side, stored client-side in DPAPI wincred; character ownership derived server-side from the credential (`_char_owner`/`UpsertCharOwner` deleted).
- **Website login = Discord OAuth2** gated on guild Discord membership (gate-free; pre-pays AUTH-09). "Sign in with Google" rejected outright (re-introduces the gate).
- **Schema evolution = forward-only `goose` migrations + API version**; retire `_meta.schema_version`/`WatcherMaxSchemaVersion`.
- **Cutover = hybrid shadow-mode** (never writes to the Sheet; self-healing inventory + enrichment).
- **Encoding contract A1 = UTF-8 `content` (locked, Plan 11-03, 2026-05-29):** the shared `internal/parse` is now UTF-8/`io.Reader`-based — the CP1252→UTF-8 decode lives in ONE place (`parse.CP1252Reader`) on the caller's disk-read side. The watcher (incl. P13) wraps; the backend ingest path (UTF-8 JSON `content`) does NOT. **P13 must not double-decode.** The v1 watcher's two runapp.go call sites already wrap in `CP1252Reader` as of 11-03 (zero behavior change off disk). RESEARCH "Encoding Note" Open Question 1 → resolution 1/2.
- **v2 access-control tightening (D-07, Plan 11-03):** v1 first-write-wins warned-and-allowed a cross-owner character; v2 REJECTS (`store.ErrCharOwnedByAnother`) + writes an append-only `audit_log` row, owner_id never overwritten — because the backend (not 12 racing watchers) owns the write.
- **Bearer-token auth = stdlib-only, hash-only-at-rest (BACKEND-04, Plan 11-04):** `auth.MintCode` mints a 32-byte `crypto/rand` token → base64url plaintext shown ONCE (stdout + return), persists ONLY `sha256(plaintext)` in `guild_code.token_hash` (mirrors the watcher's `internal/auth/store.go` DPAPI hash-only discipline). `auth.RevokeCode` sets `disabled_at` (idempotent, label-or-id). `(*auth.Auth).resolveToken` SHA-256-hashes the presented `Bearer` value and `crypto/subtle.ConstantTimeCompare`s it against active (`disabled_at IS NULL`) rows; missing/malformed/unknown/revoked → `(0,false)` (11-05 maps to 401-writes-nothing). NO PocketBase `apis.RequireAuth`/JWT — opaque static tokens via a custom guard. The auth API is verdict-agnostic (pure `crypto/*` + `database/sql`), imported UNCHANGED by 11-05. Added an exported `auth.New(db)` constructor (Rule 2) so the separate 11-05 package can build the guard while keeping `db` unexported.
- **Ingest HTTP shell = HAND-ROLLED net/http, single tested SQL path (BACKEND-01/03/04 wiring, Plan 11-05):** `POST /api/v1/ingest` (`ingest.Handler`) composes the pieces in load-bearing order — `http.MaxBytesReader(1<<20)` (V5) → `auth.ResolveToken` FIRST (`!ok` → 401 + RETURN before any store call; 401-writes-nothing is structural) → `DecodeAndValidate` (typed sentinels → 400/422) → `parse.Parse`/`ParseSpellbook` on UTF-8 `content` (A1, no CP1252) → `store.BindCharacter` + `store.Replace*Tx` in ONE `db.BeginTx` → 204; cross-owner → 409 with the `audit_log` row COMMITTED (the only write on that path; Rule 1 fix — a blanket rollback discarded the audit trace). **Single-tested-SQL-path constraint (WARNING-3):** the handler authors NO inline DELETE/INSERT SQL — exported `store.ReplaceInventoryTx`/`ReplaceSpellbookTx` (the public `Store.Replace*` now delegate to them) + `store.BindCharacter` + `auth.ResolveToken` thin wrappers (Rule 2) keep 11-03/11-04's tests as the single coverage. `cmd/squirebot-server` is one binary: `run([]string) int` dispatches `mint-code`/`revoke-code`/`serve`; `serve` runs `goose.Up` on startup (D-10), binds loopback `127.0.0.1:8090` (Caddy fronts 443), registers a `time.Ticker` scheduler skeleton (no real jobs — P12), graceful-shuts-down on SIGINT/SIGTERM. NO Google/OAuth/Sheets dep anywhere (`go list -deps` = 0). **PocketBase dep + `spike/` tree REMOVED + `go mod tidy` (0 pocketbase refs).** Server logs JSON to stdout (journald), not lumberjack/LOCALAPPDATA. Static `linux/amd64` cross-compile verified (ELF). 3 deviations auto-fixed (1 missing-critical cross-package exports, 2 bugs: cross-owner audit durability + revoke flag ordering).

### Open TODOs

- **(P11 PocketBase spike — RESOLVED 2026-05-29, Plan 11-01; cleanup DONE 11-05)** Verdict = **HAND-ROLLED Go fallback** (reject PocketBase). All four D-01 probes passed technically, but the locked design bypasses PB's auth-record model (opaque guild-code tokens via a custom `crypto/subtle` guard) and its collection model (plain SQL tables), so PB's admin-UI/auto-REST/Discord-OAuth2 leverage is unused — not worth a 22.9 MB pre-1.0 framework + migration tax. **11-05 wired `net/http` ServeMux + `time.Ticker` and REMOVED the pocketbase dep + `spike/` tree (`go mod tidy` clean, 0 pocketbase refs). P15 Discord OAuth2 is now hand-rolled `golang.org/x/oauth2`.** Verdict in `11-01-SUMMARY.md` + appended to `11-CONTEXT.md`.
- **(A-record for `api.squirebot.quest` — blocked on Hetzner VPS provisioning)** Domain `squirebot.quest` registered at Porkbun (2026-05-29); apex/`www` reserved for the P14 frontend. Remaining: add DNS A-record `api` → the Hetzner VPS public IPv4 once the instance is provisioned (P11 Wave 5 / 11-06 Task 2), so Caddy can issue the TLS cert via HTTP-01. Tracked: `.planning/todos/pending/2026-05-29-add-api-squirebot-quest-a-record-after-oracle-provision.md`.
- **(Port-relevant backlog into P14)** 999.28 (`didYouMean('')` empty-query contract) + 999.30 (`didYouMean` Levenshtein contract mismatch) should be resolved when porting search logic to the frontend (WEB-03).
- **(Watcher gofmt/console nits into P13)** 999.20 (`console_windows.go` gofmt) + 999.21 (`freeConsole()` doc-vs-impl) + 999.22 (SemVer-aware auto-update comparison — load-bearing for the P16 coordinated flip) ride along with the watcher re-target.
- **(SignPath OSS, separate track, 999.9)** still in flight; orthogonal to the backend swap; lands as a hotfix when the Foundation review completes.

### Active Blockers

None. Roadmap created; Phase 11 ready to plan.

## Session Continuity

### Last Session Summary

**2026-05-29 (Plan 11-05 executed — POST /api/v1/ingest + cmd/squirebot-server + scheduler skeleton; BACKEND-01/03/04 wiring):** Wired the backend into a runnable single binary in 3 tasks (4 atomic commits: 1 chore + 3 feat) + metadata. **Verdict resolved (FALLBACK from 11-01):** dropped `pocketbase` + `dbx` from go.mod, deleted `spike/pocketbase/`, `go mod tidy` → 0 pocketbase refs in go.mod/go.sum; all wiring is HAND-ROLLED `net/http`. **Task 1:** `internal/backendsrv/logging.Setup()` (JSON slog to os.Stdout/journald, NOT lumberjack/LOCALAPPDATA) + `ingest.Envelope` (D-04) + `DecodeAndValidate` (required-field + kind-enum, typed sentinels `ErrMalformedJSON`/`ErrMissingCharacter`/`ErrInvalidKind`; empty content = no-op; unknown fields ignored). **Task 2:** `ingest.Handler`/`New` + `ServeHTTP` — ordered flow `MaxBytesReader(1<<20)` → `auth.ResolveToken` FIRST (`!ok`→401+RETURN before any store call) → `DecodeAndValidate` (4xx) → `parse.Parse`/`ParseSpellbook` (UTF-8, no CP1252) → `store.BindCharacter` + `store.Replace*Tx` in ONE `db.BeginTx` → 204; cross-owner→409 (audit row COMMITTED). **Single-tested-SQL-path (WARNING-3):** added exported `auth.(*Auth).ResolveToken`, `store.BindCharacter`, `store.ReplaceInventoryTx`/`ReplaceSpellbookTx` (public `Store.Replace*` now delegate) — handler authors NO inline SQL (grep-confirmed). 11 httptest cases: valid inv/spellbook rows queryable, 401-writes-nothing (no-header/unknown/revoked), shrinking 3→1 drops rows, cross-owner 409+audit+untouched, bad-kind/malformed/oversized 4xx no-write, empty-content no-op. **Task 3:** `scheduler.Start(ctx)` time.Ticker skeleton (no jobs, returns on cancel) + `cmd/squirebot-server/main.go` testable `run([]string) int` dispatching mint/revoke/serve; serve = logging.Setup + store.Open + RunMigrations (goose.Up on startup, D-10) + scheduler + ServeMux `POST /api/v1/ingest` on loopback + graceful shutdown. NO Google/OAuth/Sheets dep (`go list -deps`=0). **3 deviations auto-fixed:** Rule 2 (cross-package exported wrappers — 11-03/04 internals are unexported; anticipated by 11-04); Rule 1 (cross-owner 409 must COMMIT to persist the audit row, a blanket rollback discarded it — caught by the plan's own `TestIngest_CrossOwner_409`); Rule 1 (revoke-code flag parsing was position-dependent — `splitFlagsAndPositionals` fix). **FULL `go test ./...` green** (every watcher + backend pkg, no v1 regression); `go build`/`go vet` clean; **`CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/squirebot-server` produces a static ELF** (the deploy binary 11-06 ships). BACKEND-01 (single-binary+scheduler half) / BACKEND-03 (delivery) / BACKEND-04 (401-writes-nothing) satisfied at the build/test tier; 11-06 (deploy) + 11-07 (backup) remain.

**2026-05-29 (Plan 11-04 executed — bearer-token auth: mint/revoke CLI + resolveToken guard, BACKEND-04):** Built the project's first authenticated network surface in two TDD tasks (RED→GREEN each; 4 atomic commits + metadata). New package `internal/backendsrv/auth/` (stdlib crypto only — no new third-party deps). **Task 1 (mint/revoke):** `MintCode(db, ownerLabel)` = 32-byte `crypto/rand` → `base64.RawURLEncoding` plaintext shown ONCE via `fmt.Printf` (stdout) + returned, stores ONLY `sha256(plaintext)` in `guild_code.token_hash` (hash-only at rest, mirrors `internal/auth/pkce.go` crypto shape + `internal/auth/store.go` "secret never leaves as plaintext" discipline); `upsertOwner` SELECT-then-INSERTs by label (owner.label has no UNIQUE → reuse, no duplicate owners); `RevokeCode(db, idOrLabel)` sets `disabled_at WHERE (label OR id) AND disabled_at IS NULL` (idempotent, D-09). **Task 2 (guard):** `Auth{db}` + `New(db)` + `resolveToken(ctx, authHeader) (ownerID, ok)` strips `Bearer ` (or fails), `sha256.Sum256`-hashes the presented code, queries active rows (`disabled_at IS NULL`), `subtle.ConstantTimeCompare`s each (timing-safe, RESEARCH Pattern 3); missing/malformed/unknown/revoked/wrong-scheme → `(0,false)`. No PocketBase `apis.RequireAuth` (opaque static tokens); the token is hashed before any DB use (no SQL interpolation); never logged (V7 — `auth_reject` slog records carry only a reason). 5 mint/revoke tests + 3 guard tests (7-case table + minting-owner + AST-parsed structural check) all pass. **`go build ./...`, `go vet ./...` clean; `go test ./internal/backendsrv/...` exit 0; full `go test ./...` exit 0 (no v1 watcher regression).** **2 deviations (auto-fixed):** Rule 2 — added the exported `auth.New(db)` constructor (the plan's `<interfaces>` gave `Auth{db unexported}` + `resolveToken` but no constructor; 11-05 is a separate package); Rule 1 — the structural no-PocketBase test initially `strings.Contains`-matched the guard's own doc comment (which names `apis.RequireAuth` to explain it's avoided), rewrote it to AST-parse imports for any `pocketbase` path. No `go.mod` change (stdlib only). All 6 STRIDE `mitigate` threats + ASVS V2/V6/V7 covered at the test tier. 11-05 imports the auth API UNCHANGED and wires the 401-writes-nothing path.

**2026-05-29 (Plan 11-03 executed — UTF-8 parser refactor + atomic replace tx + first-sighting bind, BACKEND-03):** Built the heart of BACKEND-03 in three TDD tasks (RED→GREEN each; 6 atomic commits). **Task 1 (encoding A1):** dropped `charmap.Windows1252` from the shared `Parse`/`ParseSpellbook` (they now treat their `io.Reader` as already UTF-8) and relocated the decode into ONE exported helper `parse.CP1252Reader`; wrapped the v1 watcher's two production call sites (`internal/app/runapp.go` inventory + spellbook) in `parse.CP1252Reader` (zero behavior change off disk); re-homed the 4 CP1252-dependent tests into `reader_test.go` + added `TestParse_UTF8Content_NoDoubleDecode`. **Task 2 (atomic replace):** `store.Store`/`NewStore` + `ReplaceInventory`/`ReplaceSpellbook` = one `BEGIN IMMEDIATE…DELETE-all…INSERT…UPDATE…COMMIT` (RESEARCH Pattern 1); shrinking snapshot drops rows (BACKEND-03); integers persist as real INTEGER via `strconv.Atoi` (Sheets `StringValue` hack DROPPED); `row_ordinal`=line order; spellbook `normalized_name`=`lower(trim(name))`; proven atomic on error (FK-violation rollback, neighbour rows untouched). **Task 3 (binding):** `bindCharacter(ctx, *sql.Tx, name, ownerID)` + `ErrCharOwnedByAnother` — first sighting binds to the uploading owner, same-owner re-bind is a no-op, a DIFFERENT owner is REJECTED + `audit_log` row, owner_id never overwritten (v2 tightens v1's warn-and-allow, D-07/V4); single indexed `SELECT … WHERE name` (UNIQUE COLLATE NOCASE); new forward-only `00002_audit.sql` (00001 untouched; goose idempotent at version 2). **FULL `go test ./...` green** (every watcher AND backend package — no v1 regression); `go build`/`go vet` clean; no `go.mod` change. **No deviations.** `bindCharacter`/`Replace*` take a `*sql.Tx`/`*sql.DB` so 11-05 composes bind + replace in ONE tx behind `POST /api/v1/ingest`.

**2026-05-29 (Plan 11-02 executed — goose schema + modernc DB-open + NewTestDB, BACKEND-02):** Built the verdict-agnostic SQLite persistence foundation. Created `internal/backendsrv/migrations/00001_init.sql` (forward-only goose migration — all ten D-13 tables: `owner`/`character` split with FK + `name UNIQUE COLLATE NOCASE`, `inventory_item`/`spellbook_entry` with `ON DELETE CASCADE`, `guild_code`, and the five EMPTY dimension tables, copied verbatim from RESEARCH §"Migration SQL Sketch") + `embed.go` (`//go:embed *.sql` + `goose.SetDialect("sqlite3")` + `goose.Up`, idempotent). Created `store/db.go` (`Open`/`DSN` on the modernc `"sqlite"` driver with the RESEARCH Pattern 5 DSN — WAL/busy_timeout(5000)/foreign_keys(ON)/synchronous(NORMAL)/_txlock=immediate — + `SetMaxOpenConns(1)` single-writer) and `store/testhelper.go` (`NewTestDB(t)` shared temp-DB fixture: `Open` + `goose.Up`, in a non-`_test.go` file so 11-03/04/05 can import it). Six tests pass (foreign_keys=1 on a fresh conn; all 10 tables created; goose.Up idempotent on re-run; 5 dimension tables empty); `go build ./...`, `go vet ./...` clean; `CGO_ENABLED=0` builds on host + `linux/amd64` (pure-Go modernc, no cgo). Task 2 followed TDD RED→GREEN. 3 atomic commits (`c262816` feat migration, `3117a1a` test RED, `165582a` feat GREEN). **No deviations.** `go mod tidy` promoted `modernc.org/sqlite` + `goose/v3` to direct deps (and reclassified the pre-existing spike's `pocketbase` dep to direct — removed in 11-05 as planned, not touched here). Wave-3 (11-03 store tx / 11-04 auth store / 11-05 ingest+HTTP) unblocked.

**2026-05-29 (Plan 11-01 executed — PocketBase spike):** Ran the time-boxed D-01 PocketBase-as-framework spike (the gating plan of Phase 11). Pinned `github.com/pocketbase/pocketbase@v0.39.0`, confirming it pulls the same no-cgo `modernc.org/sqlite v1.51.0` the hand-rolled fallback uses (the headline RESEARCH finding, now empirical). A throwaway harness (`spike/pocketbase/main.go`) exercised all four probes with recorded PASS evidence: (a) 5 raw-DDL tables coexist with PB system tables; (b) custom `crypto/subtle` bearer guard → 401 no/bad token, 200 valid, with idempotent DELETE+INSERT atomic replace in `RunInTransaction`; (c) `*/1 * * * *` cron fired at the minute boundary while serving; (d) `CGO_ENABLED=0 GOOS=linux GOARCH=amd64` produced a 22.9 MB static ELF x86-64 binary from Windows. **VERDICT = HAND-ROLLED Go fallback** (reject PocketBase): the locked design bypasses PB's auth-record + collection models, so its admin-UI/auto-REST/Discord-OAuth2 leverage is unused — not worth a pre-1.0 framework + migration tax. Verdict recorded in `11-01-SUMMARY.md` and appended to `11-CONTEXT.md`. 3 atomic commits (`b894008` feat, `adeaead` chore, `b797c64` docs). 11-02 (verdict-agnostic) and 11-05 (now `net/http` ServeMux + `time.Ticker`; removes the PB dep + spike tree) unblocked.

**2026-05-28 (v2.0 ROADMAP created):** Roadmapper transformed the 26 v2.0 requirements into a 6-phase structure (Phases 11–16), accepting REQUIREMENTS.md's provisional A–F mapping unchanged (no concrete coverage problem found). Coverage validated 26/26 (P11=5, P12=2, P13=4, P14=6, P15=5, P16=4); no orphans, no duplicates. Dependencies set: P11 has none; P12→P11; P13→P11; P14→P11+P12; P15→P14+P11; P16→P13+P14+P15+P12. Each phase got 2–5 observable success criteria. Honored the locked stack (Oracle Always Free + SQLite + goose; SvelteKit; Discord OAuth2; bearer token) over the research docs' Hetzner+Postgres recommendation, with an explicit "locked overrides research" note in ROADMAP. Front-loaded the ingest path (P11 + P13 restore data flow before the polished P14 frontend). Surfaced the optional PocketBase spike as a P11 decision note. Wrote `.planning/ROADMAP.md` (v2.0 section + Phase Details + progress tables + backlog re-annotated for Sheet-mooting), finalized `.planning/REQUIREMENTS.md` traceability (Phase column finalized), and reset this STATE.md to the v2.0 phase plan (replacing the stale v1.0.2 phase-plan content). Phase numbering starts at 11 (Phase dirs 09+10 exist on disk from the superseded v1.0.2 binary; never reuse 9/10). *(2026-05-29: backend host later switched Oracle → Hetzner — see Phase 11 CONTEXT D-12; the "$0 backend / ~$12/yr total" premise this entry recorded is retired in favour of ~$67/yr.)*

**2026-05-13 (v1.0.2 binary shipped):** Phases 9 + 10 shipped as binary tag `v1.0.2`; HUMAN-UAT was blocked on 999.19 (Google brand verification). Milestone close subsequently superseded by the v2.0 "Off Google" pivot — the Sheet it targeted is being replaced, so a Google-OAuth-dependent UAT close became moot.

### Files of Record

- `.planning/PROJECT.md` — core value, requirements (v1 Validated + v2.0 Active), constraints, Key Decisions (updated 2026-05-28 with v2.0 milestone).
- `.planning/REQUIREMENTS.md` — v2.0 requirements (26 across 7 categories) + finalized traceability (Phases 11–16).
- `.planning/ROADMAP.md` — v2.0 Phases 11–16 (full Phase Details + success criteria) + collapsed v1.0/v1.0.1/v1.0.2 + Backlog (updated 2026-05-28).
- `.planning/MILESTONES.md` — historical record (v1.0 + v1.0.1; v1.0.2 not recorded — close superseded).
- `.planning/explorations/website-milestone/SCOPE.md` — authoritative v2.0 research (A–F phase plan, dependencies, effort, open decisions).
- `.planning/explorations/website-milestone/{01-backend-hosting,02-frontend-stack,03-watcher-auth,04-data-enrichment-migration}.md` — 4 findings docs.
- `.planning/RETROSPECTIVE.md` — living retrospective (v1.0 + v1.0.1 + Cross-Milestone Trends).
- `.planning/STATE.md` — this file.
- `.planning/config.json` — granularity (coarse), mode (yolo), workflow toggles.

### Next Action

`/clear` then `/gsd-execute-phase 11` — continue Phase 11 at **Plan 11-06** (deploy: Hetzner provisioning + Caddy auto-HTTPS + systemd `Restart=always` + `ufw` firewall + TLS via Caddy HTTP-01 + the pending `api.squirebot.quest` DNS A-record). The deploy binary is ready: `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o squirebot-server ./cmd/squirebot-server` produces a static ELF; the systemd unit invokes `squirebot-server serve --addr 127.0.0.1:8090 --db /var/lib/squirebot/squirebot.db`; mint a guild code on the box with `squirebot-server mint-code --owner <label>` (prints once). 11-05 satisfied BACKEND-01 (single-binary + in-process scheduler half) / BACKEND-03 (HTTP delivery) / BACKEND-04 (401-writes-nothing) at the build/test tier; the on-box deploy/TLS/firewall is 11-06, backup is 11-07. The tx-reuse is shared-store `*sql.Tx` (no PB path). **Pending 11-06 chore:** add the `api` A-record → the Hetzner VPS public IPv4 (`5.78.232.85` per CONTEXT D-12) so Caddy issues the cert. **P13 handoff: the watcher must CP1252→UTF-8 decode via `parse.CP1252Reader` before POSTing `content` to `POST /api/v1/ingest` — do NOT double-decode.** Optionally `/gsd-verify-work 11`.

---

*State initialized: 2026-04-30. v1.0 COMPLETE: 2026-05-11. v1.0.1 COMPLETE: 2026-05-12. v1.0.2 binary shipped: 2026-05-13 (close superseded). v2.0 OPENED + ROADMAP created: 2026-05-28 (Phases 11–16).*
