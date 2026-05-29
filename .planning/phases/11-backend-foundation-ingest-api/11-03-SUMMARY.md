---
phase: 11-backend-foundation-ingest-api
plan: 03
subsystem: database
tags: [go, sqlite, parser, utf-8, cp1252, transaction, atomic-replace, goose, audit, access-control, tdd]

# Dependency graph
requires:
  - phase: 11-backend-foundation-ingest-api (Plan 02)
    provides: "store.Open (modernc *sql.DB with WAL/_txlock=immediate/SetMaxOpenConns(1)), store.NewTestDB shared fixture, migrations.RunMigrations (//go:embed goose runner), 00001_init.sql (all D-13 tables incl. character/owner/inventory_item/spellbook_entry)"
  - phase: 01-end-to-end-thin-slice (Plan 04)
    provides: "internal/parse.Parse / ParseSpellbook (the tab-separated EQ-file parser being refactored to UTF-8)"
provides:
  - "parse.CP1252Reader(io.Reader) io.Reader — the SINGLE source of the Windows-1252→UTF-8 decode; the shared Parse/ParseSpellbook now treat their reader as already UTF-8 (encoding contract A1 locked)"
  - "store.Store + NewStore(*sql.DB); Store.ReplaceInventory / Store.ReplaceSpellbook — the atomic full-snapshot replace tx (one BEGIN IMMEDIATE…DELETE…INSERT…UPDATE…COMMIT) porting the v1 Sheets clear+write contract to SQLite (BACKEND-03)"
  - "store.ErrCharOwnedByAnother + bindCharacter(ctx, *sql.Tx, name, ownerID) — first-sighting owner bind + cross-owner reject + audit_log row (D-07/V4)"
  - "00002_audit.sql — forward-only goose migration creating the append-only audit_log table (the 2nd migration; 00001 untouched)"
affects: [11-05, 11-04, 13-watcher-re-target, 12-enrichment-job-migration, 14-web-frontend]

# Tech tracking
tech-stack:
  added: []  # no new third-party deps; go.mod unchanged
  patterns:
    - "Encoding contract A1: the shared parser is UTF-8/io.Reader-based; the CP1252 decode lives in ONE place (parse.CP1252Reader) on the caller's disk-read side — the watcher wraps; the backend ingest path (UTF-8 JSON content) does not. Never double-decode."
    - "Atomic full-snapshot replace = one BEGIN IMMEDIATE…DELETE-all…INSERT…UPDATE…COMMIT (RESEARCH Pattern 1); shrinking snapshot drops removed rows; defer tx.Rollback rolls the DELETE back on any INSERT error"
    - "Write-side helpers (bindCharacter) take a *sql.Tx (not *sql.DB) so the ingest handler (11-05) composes bind + replace in ONE transaction"
    - "Real INTEGER columns parsed via strconv.Atoi — the Sheets StringValue-everywhere hack is dropped (SQLite has a real schema)"
    - "v2 access-control tightening: v1 first-write-wins warned-and-allowed cross-owner; v2 REJECTS (ErrCharOwnedByAnother) + audit row, owner_id never overwritten"

key-files:
  created:
    - "internal/parse/reader_test.go"
    - "internal/backendsrv/store/replace.go"
    - "internal/backendsrv/store/replace_test.go"
    - "internal/backendsrv/store/binding.go"
    - "internal/backendsrv/store/binding_test.go"
    - "internal/backendsrv/migrations/00002_audit.sql"
  modified:
    - "internal/parse/inventory.go"
    - "internal/parse/spellbook.go"
    - "internal/parse/inventory_test.go"
    - "internal/parse/spellbook_test.go"
    - "internal/app/runapp.go"

key-decisions:
  - "Locked encoding contract A1 to RESEARCH resolution 1/2: content on the wire is UTF-8; the watcher owns the CP1252→UTF-8 decode; the shared server parser drops charmap. Chose ONE exported CP1252Reader helper in package parse (resolution 2 — 'one source of truth') over a forked backendsrv/parse copy."
  - "bindCharacter / ReplaceInventory / ReplaceSpellbook operate on a *sql.Tx (bind) / *sql.DB (replace) so 11-05 can compose first-sighting bind + atomic replace inside ONE transaction (a rejected upload rolls back cleanly)."
  - "TestReplaceInventory_AtomicOnError forces the mid-loop INSERT failure via an FK violation (foreign_keys(ON)): delete the target character out-of-band, then re-replace so the in-tx INSERT fails AFTER the in-tx DELETE — proves atomic rollback AND that a neighbour character's rows are untouched. (Schema has no string-reachable NOT NULL/UNIQUE to violate per-row; FK is the deterministic lever.)"
  - "audit_log shipped as a NEW forward-only 00002_audit.sql (11-02's 00001_init.sql left untouched; goose is forward-only and //go:embed *.sql auto-includes the new file)."

patterns-established:
  - "Pattern: wrap a CP1252 source in parse.CP1252Reader exactly ONCE on the disk-read side; UTF-8 callers feed the reader straight in (no double-decode → no mojibake)"
  - "Pattern: every atomic full-snapshot replace is one BEGIN IMMEDIATE transaction with defer tx.Rollback; never DELETE+INSERT in separate transactions (the SQLite analog of the locked write.go anti-pattern)"
  - "Pattern: slog on the write path logs operation + char_id + row_ordinal + err only — NEVER raw content, NEVER the bearer token (V7)"

requirements-completed: [BACKEND-03]

# Metrics
duration: 11 min
completed: 2026-05-29
---

# Phase 11 Plan 03: Atomic Replace Tx + First-Sighting Owner Binding + UTF-8 Parser Refactor Summary

**Encoding contract A1 locked (shared parser is UTF-8/`io.Reader`-based; the CP1252 decode moved to a single `parse.CP1252Reader` helper that the v1 watcher now wraps its disk reads in), the atomic full-snapshot replace tx (`ReplaceInventory`/`ReplaceSpellbook` — one `BEGIN IMMEDIATE…DELETE…INSERT…COMMIT`) ported from the v1 Sheets clear+write contract, and first-sighting owner binding with a cross-owner reject (`ErrCharOwnedByAnother`) + append-only `audit_log` — all TDD, all verdict-agnostic, the full `go test ./...` (watcher included) green.**

## Performance

- **Duration:** 11 min
- **Started:** 2026-05-29T21:39:08Z
- **Completed:** 2026-05-29T21:50:39Z
- **Tasks:** 3 (all TDD: RED → GREEN)
- **Files created:** 6; **modified:** 5 (11 total; no `go.mod` change)

## Accomplishments

- **Encoding contract A1 locked (Task 1):** `Parse`/`ParseSpellbook` now treat their `io.Reader` as ALREADY UTF-8 — the `charmap.Windows1252.NewDecoder().Reader(r)` step is removed from the shared entries and relocated into ONE exported helper, `parse.CP1252Reader(io.Reader) io.Reader`. The v1 watcher's two production call sites (`internal/app/runapp.go` inventory + spellbook) now wrap their raw-CP1252 `*os.File` in `parse.CP1252Reader` — so the watcher keeps decoding CP1252 off disk with ZERO behavior change, while the backend ingest path (UTF-8 JSON `content`, P13) feeds the reader straight in with no double-decode. The four CP1252-dependent inventory+spellbook tests were re-homed symmetrically into `reader_test.go` (wrapped in `CP1252Reader`); a new `TestParse_UTF8Content_NoDoubleDecode` proves UTF-8 content survives byte-clean through the bare `Parse`.
- **Atomic full-snapshot replace (Task 2):** `store.Store` wraps the `*sql.DB` from `store.Open`; `ReplaceInventory`/`ReplaceSpellbook` do DELETE-all-then-INSERT in ONE `BEGIN IMMEDIATE…COMMIT` (RESEARCH Pattern 1) — the SQLite port of the v1 Sheets clear+write contract. A shrinking snapshot drops removed rows for free via the DELETE (BACKEND-03); the transaction is proven atomic on error (an INSERT failure rolls the DELETE back, no partial state, neighbour rows untouched). `item_id`/`count`/`slots`/`level` persist as real INTEGER columns via `strconv.Atoi` (the Sheets `StringValue` hack is DROPPED); `row_ordinal` = file line order; spellbook `normalized_name` = `lower(trim(name))` computed at insert.
- **First-sighting bind + cross-owner reject + audit (Task 3):** `bindCharacter(ctx, *sql.Tx, name, ownerID)` INSERTs a new character bound to the uploading token's owner on first sighting, returns the existing id on a same-owner re-bind (no-op), and REJECTS a cross-owner upload with `ErrCharOwnedByAnother` + an append-only `audit_log` row — owner_id is NEVER overwritten (D-07 / V4, tightening v1's warn-and-allow). The lookup is a single indexed `SELECT … WHERE name = ?` (UNIQUE COLLATE NOCASE → case-insensitive). `audit_log` ships as a new forward-only `00002_audit.sql` (11-02's `00001` untouched; goose stays idempotent across two migrations, now at version 2).
- **No v1 watcher regression:** the FULL `go test ./...` passes (watcher packages `internal/app`, `internal/parse`, `internal/watch`, `internal/sheet`, `internal/wizard`, … AND backend `internal/backendsrv/store` + `internal/backendsrv/migrations`); `go build ./...` and `go vet ./...` clean.

## Task Commits

Each task was committed atomically, all following the TDD RED → GREEN gate:

1. **Task 1 (RED): re-home CP1252 parse tests + add UTF-8 no-double-decode test** — `4ab7d47` (test)
2. **Task 1 (GREEN): lock encoding contract A1 — UTF-8 parser + CP1252Reader helper + watcher call sites** — `f67496e` (feat)
3. **Task 2 (RED): failing atomic-replace tests** — `5142dee` (test)
4. **Task 2 (GREEN): atomic full-snapshot replace tx (ReplaceInventory/ReplaceSpellbook)** — `6c20a58` (feat)
5. **Task 3 (RED): failing first-sighting bind / cross-owner reject tests** — `80e151c` (test)
6. **Task 3 (GREEN): first-sighting bind + cross-owner reject + audit (00002_audit.sql)** — `8f0615a` (feat)

**Plan metadata:** committed separately (this SUMMARY + STATE + ROADMAP).

_No REFACTOR commits — each GREEN implementation was minimal and clean._

## Files Created/Modified

- `internal/parse/inventory.go` — dropped `charmap` decode from `Parse`; added exported `CP1252Reader` helper + the package-level encoding-contract-A1 / P13 handoff docstring.
- `internal/parse/spellbook.go` — dropped `charmap` decode from `ParseSpellbook`; removed the now-unused `golang.org/x/text/encoding/charmap` import; updated docstring.
- `internal/parse/inventory_test.go` — deleted the two CP1252-asserting tests (re-homed); UTF-8/ASCII-clean cases unchanged.
- `internal/parse/spellbook_test.go` — deleted the two CP1252-dependent tests (re-homed); pruned now-unused `os`/`strconv` imports.
- `internal/parse/reader_test.go` (NEW) — `CP1252Reader`-wrapped re-homes of the four CP1252 tests + `TestParse_UTF8Content_NoDoubleDecode`.
- `internal/app/runapp.go` — wrapped both v1-watcher production parse call sites (inventory ~463, spellbook ~548) in `parse.CP1252Reader`.
- `internal/backendsrv/store/replace.go` (NEW) — `Store`/`NewStore`/`DB` + `ReplaceInventory`/`ReplaceSpellbook` atomic replace tx.
- `internal/backendsrv/store/replace_test.go` (NEW) — 5 replace tests + the shared `seedOwnerChar` helper.
- `internal/backendsrv/store/binding.go` (NEW) — `ErrCharOwnedByAnother` + `bindCharacter` + `auditCrossOwnerReject`.
- `internal/backendsrv/store/binding_test.go` (NEW) — 4 binding tests + `seedOwner`/`bindInTx` helpers.
- `internal/backendsrv/migrations/00002_audit.sql` (NEW) — forward-only goose migration creating `audit_log`.

## Decisions Made

- **Encoding A1 → resolution 1/2 (UTF-8 content; one `CP1252Reader`):** RESEARCH offered three resolutions. Resolution 1 ("watcher decodes; server parser drops charmap") is the locked contract; resolution 2's mechanism ("refactor `Parse`/`ParseSpellbook` to a pre-built `io.Reader` + move the decode to the caller") gives one source of truth without forking a `backendsrv/parse` copy. Implemented as a single exported `parse.CP1252Reader` the watcher wraps and the backend does not.
- **`*sql.Tx` on the bind path:** `bindCharacter` takes a `*sql.Tx` so 11-05 can compose first-sighting bind + atomic replace in ONE transaction. The audit row is written in the same tx (so a cross-owner reject's audit record is durable even though the ingest is refused).
- **Atomicity test via FK violation:** the schema has no per-row, string-reachable NOT NULL / UNIQUE / CHECK to violate inside a same-character loop (`strconv.Atoi` swallows bad ints → 0; empty strings ≠ NULL). The deterministic lever is the FK constraint (`foreign_keys(ON)`): delete the target character out-of-band, then re-replace so the in-tx INSERT fails AFTER the in-tx DELETE — rigorously proving the DELETE rolls back AND a neighbour character's rows are untouched.
- **`00002_audit.sql` not `ALTER`-into-00001:** goose is forward-only and 11-02 owns `00001_init.sql`; the `//go:embed *.sql` glob auto-includes the new file and `NewTestDB` applies it. Confirmed 11-02's idempotency test still passes at version 2.

## Deviations from Plan

None - plan executed exactly as written.

The plan's interface sketch and RESEARCH Pattern 2 differed cosmetically on the binding SELECT (`SELECT owner_id` vs `SELECT owner_id, id`); the plan's `<action>` text specifies `SELECT owner_id, id FROM character WHERE name = ?` scanning both columns, which is what was implemented (the id is needed to return the existing charID on a same-owner match) — this is following the plan as written, not a deviation. No bugs, missing-critical, or blocking issues arose (Rules 1–3 not triggered); no architectural questions (Rule 4 not triggered).

**Total deviations:** 0.
**Impact on plan:** None — clean TDD execution; BACKEND-03 (replace contract) + the D-07/V4 access-control half satisfied at the build/test tier, both PATTERNS carve-outs honored (no charmap on the server parser; no StringValue hack).

## Issues Encountered

None. The one design question — how to force a deterministic mid-loop INSERT failure to prove transactional atomicity when the schema exposes no string-reachable per-row constraint — was resolved by using the FK constraint (the character is deleted out-of-band so the re-INSERT fails after the in-tx DELETE), which proves both rollback and neighbour-isolation. The Git LF→CRLF normalization warning on `00002_audit.sql` is benign (autocrlf), not an error.

## Threat Model Compliance

All `mitigate`-disposition threats from the plan's STRIDE register are covered and test-proven:

- **T-11.03-01 (SQL injection):** `replace.go` + `binding.go` use `?` parameterized placeholders exclusively; no SQL built by string concatenation (V5). Grep-confirmed.
- **T-11.03-02 (cross-guildie overwrite):** first-sighting bind + cross-owner reject (`ErrCharOwnedByAnother`); `owner_id` never overwritten on mismatch. `TestBindCharacter_CrossOwnerRejects` proves the reject, the audit row, AND the unchanged owner_id.
- **T-11.03-03 (partial state after failed replace):** one `BEGIN IMMEDIATE…COMMIT` with `defer tx.Rollback`. `TestReplaceInventory_AtomicOnError` proves no partial state on an INSERT failure.
- **T-11.03-04 (content/data leak in logs):** the write-path `slog` calls log operation + `char_id` + `row_ordinal` + `err` only — never raw `content` (V7).
- **T-11.03-05 (cross-owner takeover leaves no trace):** `auditCrossOwnerReject` writes an append-only `audit_log` row before the reject returns.
- **T-11.03-06 (encoding move silently mojibakes the live v1 watcher):** both watcher call sites wrapped in `parse.CP1252Reader`; the re-homed `TestCP1252Reader_*` tests + the watcher's own `go test ./internal/app/...` prove disk decode is unchanged; a grep gate confirms no bare raw-CP1252 caller remains.

No new security-relevant surface beyond the plan's threat register (the HTTP/token/envelope controls are 11-04/11-05).

## Known Stubs

None. All three deliverables are fully wired and test-proven business logic. They are imported unchanged by 11-05 (the HTTP shell), which composes `bindCharacter` + `ReplaceInventory`/`ReplaceSpellbook` in one transaction behind the `POST /api/v1/ingest` handler.

## P13 Handoff (load-bearing)

**The watcher (P13) MUST CP1252→UTF-8 decode (via `parse.CP1252Reader`) before POSTing `content`; the server parser assumes UTF-8 and will mojibake raw CP1252. Do NOT double-decode** (wrap a CP1252 source exactly once). The existing v1 watcher in `internal/app/runapp.go` already wraps its disk reads in `CP1252Reader` as of this plan — when P13 re-targets the watcher at the ingest API, it should read the file via `CP1252Reader` to produce a UTF-8 Go `string`, put that clean UTF-8 in the JSON `content`, and let the server parse it WITHOUT re-decoding.

## User Setup Required

None - no external service configuration required. All tests run in CI on the Windows dev box (pure-Go modernc, no cgo, no live box; no `go.mod` change).

## Next Phase Readiness

- **BACKEND-03 satisfied at the build/test tier.** The atomic replace tx + first-sighting bind + audit are exported, verdict-agnostic (survive the 11-01 HAND-ROLLED verdict), and ready for 11-05 to compose behind the `net/http` ingest handler.
- **11-05 (ingest + HTTP/cron wiring)** can now: resolve the owner from the bearer token (11-04), `BeginTx`, call `bindCharacter(ctx, tx, name, ownerID)` (mapping `ErrCharOwnedByAnother` → 409), then `ReplaceInventory`/`ReplaceSpellbook` on the same logical operation, and commit — full-snapshot atomic ingest. It still carries the two cleanup chores (remove the `pocketbase` dep + delete `spike/pocketbase/`, then `go mod tidy`).
- **11-04 (auth store)** is unaffected and proceeds in parallel (it builds `guild_code` hash storage/lookup; the owner id it resolves feeds `bindCharacter`).
- **No blockers.**

## Self-Check: PASSED

- Files on disk: `reader_test.go` FOUND, `store/replace.go` FOUND, `store/replace_test.go` FOUND, `store/binding.go` FOUND, `store/binding_test.go` FOUND, `migrations/00002_audit.sql` FOUND, `11-03-SUMMARY.md` FOUND.
- Commits exist: `4ab7d47` (T1 RED), `f67496e` (T1 GREEN), `5142dee` (T2 RED), `6c20a58` (T2 GREEN), `80e151c` (T3 RED), `8f0615a` (T3 GREEN) — all FOUND.
- Plan `<verification>` re-run: `go build ./...` exit 0; `go vet ./...` exit 0; FULL `go test ./...` exit 0 (every watcher AND backend package green — no v1 regression). Encoding: no `NewDecoder().Reader` inside `Parse`/`ParseSpellbook`; `CP1252Reader` helper exists; both runapp.go call sites + no bare raw-CP1252 caller (grep-confirmed). Replace: one `BeginTx`/`Commit` pair per call; `DELETE FROM inventory_item WHERE character_id`; `strconv.Atoi` integers; no `StringValue`/`NumberValue` in code. Binding: `ErrCharOwnedByAnother` + `SELECT owner_id, id FROM character WHERE name`; cross-owner reject + audit row + no overwrite; case-insensitive match. `00002_audit.sql` has `CREATE TABLE audit_log` + both goose markers; goose idempotent at version 2. No raw token/content in logs.

## TDD Gate Compliance

All three tasks followed the RED → GREEN gate in git history: `test(11-03)` RED precedes `feat(11-03)` GREEN for each (T1 `4ab7d47`→`f67496e`, T2 `5142dee`→`6c20a58`, T3 `80e151c`→`8f0615a`). Each RED commit was confirmed red (the function/type under test undefined at compile time, or behavior the bare code lacked); each corresponding GREEN commit turned its tests green. No REFACTOR commits — implementations were minimal and clean.

---
*Phase: 11-backend-foundation-ingest-api*
*Completed: 2026-05-29*
