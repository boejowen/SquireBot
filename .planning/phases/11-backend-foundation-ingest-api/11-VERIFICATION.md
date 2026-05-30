---
phase: 11-backend-foundation-ingest-api
verified: 2026-05-29T00:00:00Z
status: passed
score: 5/5 must-haves verified
overrides_applied: 0
re_verification:
  # No previous VERIFICATION.md existed — this is initial verification.
overrides:
  - must_have: "BACKEND-01 / BACKEND-06 / ship gate live on-box state (running server, valid TLS cert, systemd active, nightly cron, R2 backup object, restore drill)"
    reason: "11-06 and 11-07 are autonomous:false LIVE-INFRASTRUCTURE plans. Their on-box deliverables (running binary, Caddy-issued LE cert, systemd unit active+enabled, /etc/cron.d backup, R2 object, restore drill, ship-gate POST/401) are not greppable codebase state by design. The committed artifacts (deploy/Caddyfile, deploy/squirebot-server.service, deploy/squirebot-backup.sh) exist, are substantive, and match the binary's real CLI (serve --addr 127.0.0.1:8090 --db ...). The live deploy was executed on the real Hetzner box (5.78.232.85, Hillsboro OR) on 2026-05-29 and the evidence is captured in docs/backend-deploy.md §6 + corroborated by 11-06-SUMMARY and 11-07-SUMMARY. Per the phase verification directive, §6 is treated as accepted captured live evidence."
    accepted_by: "gsd-verifier (per phase verification directive)"
    accepted_at: "2026-05-29T00:00:00Z"
---

# Phase 11: Backend Foundation + Ingest API — Verification Report

**Phase Goal:** Stand up the self-hosted backend so guildie watchers have a live, authenticated place to upload to again — Hetzner Cloud VPS (US) behind Caddy auto-HTTPS, a SQLite schema under goose forward-only migrations, per-guildie bearer-token auth, and the upload-receiving endpoint that atomically replaces a character's rows.
**Verified:** 2026-05-29
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (the 5 ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | **BACKEND-01** — backend serves over HTTPS at the domain from the Hetzner VPS; single Go binary behind Caddy; restart-on-reboot via systemd; valid TLS cert | ✓ VERIFIED | Binary half: `cmd/squirebot-server/main.go` is a single net/http binary wiring `POST /api/v1/ingest` + `scheduler.Start(ctx)` + `goose.Up` on startup; `go build ./...` exits 0; `linux/amd64 CGO_ENABLED=0` cross-compile in 11-06-SUMMARY (10.4 MB static ELF). Deploy half: `deploy/Caddyfile` (`api.squirebot.quest { reverse_proxy localhost:8090 }`) + `deploy/squirebot-server.service` (`ExecStart=…serve --addr 127.0.0.1:8090 --db …`, `Restart=always`, `After=network-online.target`) match the binary's real CLI. Live (docs/backend-deploy.md §6): `curl https://api.squirebot.quest/` → 404 w/ valid LE cert (TLS_VERIFY=0); systemd `active`+`enabled`; reboot-survival confirmed; ufw 22/80/443 (8090 loopback-only); Caddy v2.11.3. |
| 2 | **BACKEND-02** — `goose up` applies the SQLite schema cleanly on a fresh DB (separate owner+character + inventory/spellbook/dimension tables); re-run is a no-op | ✓ VERIFIED | `00001_init.sql` creates all 10 D-13 tables (owner, character w/ `owner_id REFERENCES owner(id)` + `name UNIQUE COLLATE NOCASE`, inventory_item + spellbook_entry w/ `ON DELETE CASCADE`, guild_code, + 5 EMPTY dimension tables); `00002_audit.sql` adds audit_log. `embed.go` uses `goose.SetDialect("sqlite3")` + `//go:embed *.sql`. Tests PASS: `TestRunMigrations_CreatesAllTables`, `TestRunMigrations_Idempotent`, `TestDimensionTables_Empty`, `TestOpen_ForeignKeysEnabled`. Live §6: `.tables` shows all 12 tables; `goose: successfully migrated database to version: 2`. |
| 3 | **BACKEND-03** — POST of a full-snapshot inventory/spellbook w/ valid Bearer token atomically replaces that char's rows (delete-all-then-insert in one tx), rows queryable; shrinking snapshot drops rows | ✓ VERIFIED | `store/replace.go` `ReplaceInventoryTx`/`ReplaceSpellbookTx`: `DELETE FROM … WHERE character_id` + prepared INSERT loop + UPDATE character, in ONE `*sql.Tx`; integers via `strconv.Atoi` (StringValue hack dropped). `handler.go` composes guard→decode→parse→`BindCharacter`→`Replace*Tx` over one `BeginTx`. Tests PASS: `TestIngest_ValidInventory_ReplacesRows` (round-trip, row queried back), `TestIngest_ShrinkingSnapshot_DropsRows`, `TestReplaceInventory_AtomicOnError`, `TestReplaceSpellbook_NormalizedName`. Server parser is UTF-8-only (A1); v1 watcher call sites in `runapp.go:463/548` wrapped in `parse.CP1252Reader`. Live §6 ship gate: POST→204, row queried back `ShipGateChar\|Rusty Dagger\|12345`. |
| 4 | **BACKEND-04** — missing/malformed/unknown bearer → 401, writes nothing; maintainer mints per-guildie token shown once, stored only hashed | ✓ VERIFIED | `auth/guard.go` `resolveToken`: `sha256.Sum256` + `subtle.ConstantTimeCompare` against `WHERE disabled_at IS NULL`; missing prefix → `(0,false)`. `handler.go` calls `ResolveToken` FIRST, `!ok` → 401 + `return` before any store call. `auth/mint.go`: `crypto/rand` 32-byte token → `base64.RawURLEncoding` (shown once via stdout) → stores `sum[:]` (SHA-256), never plaintext. `store.go` `RevokeCode` sets `disabled_at`. Tests PASS: `TestIngest_NoAuthHeader_401_WritesNothing` (count=0), `TestIngest_UnknownToken_401`, `TestIngest_RevokedToken_401`, `TestMintCode_StoresHashNotPlaintext`, `TestResolveToken_Table`. Live §6: unauth POST → 401 (wrote nothing); mint printed code once. |
| 5 | **BACKEND-06** — nightly off-box backup on a schedule + documented restore that reconstitutes the DB on a clean box | ✓ VERIFIED | `deploy/squirebot-backup.sh`: `sqlite3 "$DB" ".backup '$SNAP'"` (online backup API, not raw cp) → `gzip` → `rclone copy "$SNAP.gz" r2:squirebot-backups/`. `docs/backend-deploy.md §4` documents the clean-box restore (rclone pull → gunzip → place → `systemctl restart`, goose.Up no-ops). Live §6: object `squirebot-2026-05-29.db.gz` present in R2; cron `/etc/cron.d/squirebot-backup` (`0 4 * * *`); restore drill reconstituted all 12 tables + rows. |

**Score:** 5/5 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/backendsrv/migrations/00001_init.sql` | All D-13 tables incl. 5 empty dimension tables | ✓ VERIFIED | All 10 tables present; owner/character split w/ FK; CASCADE on child tables; +goose Up/Down |
| `internal/backendsrv/migrations/00002_audit.sql` | audit_log (forward-only, 00001 untouched) | ✓ VERIFIED | `CREATE TABLE audit_log` + Up/Down; auto-picked by `//go:embed *.sql` |
| `internal/backendsrv/migrations/embed.go` | `//go:embed` + `goose.SetDialect("sqlite3")` + RunMigrations | ✓ VERIFIED | Correct dialect string (≠ driver "sqlite"); idempotent goose.Up |
| `internal/backendsrv/store/db.go` | modernc `sql.Open("sqlite")` + DSN pragmas + SetMaxOpenConns(1) | ✓ VERIFIED | Full DSN (WAL/busy_timeout/foreign_keys/synchronous/_txlock=immediate); single-writer |
| `internal/backendsrv/store/replace.go` | Atomic full-snapshot replace tx (+ Tx-taking variants) | ✓ VERIFIED | `ReplaceInventoryTx`/`ReplaceSpellbookTx` = the single SQL path the handler reuses |
| `internal/backendsrv/store/binding.go` | First-sighting bind + cross-owner reject + audit | ✓ VERIFIED | `ErrCharOwnedByAnother`, indexed name lookup, audit row, no owner overwrite |
| `internal/backendsrv/auth/mint.go` + `store.go` | crypto/rand token, hash-only storage, revoke | ✓ VERIFIED | `rand.Read` + `sha256` + `base64url`; `RevokeCode` sets disabled_at; no math/rand |
| `internal/backendsrv/auth/guard.go` | SHA-256 + constant-time compare | ✓ VERIFIED | `subtle.ConstantTimeCompare`, `disabled_at IS NULL`, no `apis.RequireAuth` |
| `internal/backendsrv/ingest/handler.go` | guard-first → bind → atomic replace; 401 writes nothing; MaxBytesReader | ✓ VERIFIED | Ordered flow; one tx; no inline DELETE/INSERT SQL; 409 on cross-owner |
| `internal/backendsrv/ingest/envelope.go` | D-04 envelope + kind-enum validation | ✓ VERIFIED | required-field + enum checks; empty-content no-op allowed |
| `internal/backendsrv/scheduler/scheduler.go` | In-process skeleton, no real jobs (P12) | ✓ VERIFIED | time.Ticker goroutine, clean ctx-cancel shutdown; skeleton is in-scope |
| `cmd/squirebot-server/main.go` | serve/mint-code/revoke-code dispatch + goose.Up on startup | ✓ VERIFIED | Subcommand dispatch; `RunMigrations` on startup; loopback bind; no Google deps |
| `deploy/Caddyfile` | `reverse_proxy localhost:8090` | ✓ VERIFIED | `api.squirebot.quest { reverse_proxy localhost:8090 }` |
| `deploy/squirebot-server.service` | systemd Restart=always, loopback ExecStart | ✓ VERIFIED | ExecStart flags match main.go exactly; Restart=always; NoNewPrivileges |
| `deploy/squirebot-backup.sh` | sqlite3 .backup → gzip → rclone copy to R2 | ✓ VERIFIED | online-backup API (not raw cp); bucket-scoped rclone remote |
| `docs/backend-deploy.md` | deploy + ufw + restore runbook + §6 live evidence | ✓ VERIFIED | All sections present; §6 captures the 2026-05-29 live deploy evidence |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `handler.go` | `auth` (ResolveToken) + `store` (Bind/ReplaceTx) | guard first → bind → replace in one *sql.Tx | ✓ WIRED | `h.guard.ResolveToken` before any store call; `store.BindCharacter` + `store.Replace*Tx` over one BeginTx; no inline SQL |
| `main.go` | `migrations.RunMigrations` + `store.Open` + `ingest.New` + `scheduler.Start` | goose.Up on startup; route registration; serve | ✓ WIRED | `mux.Handle("POST /api/v1/ingest", ingest.New(auth.New(db), db))`; `RunMigrations(db)`; `scheduler.Start(ctx)` |
| `embed.go` | `00001_init.sql` / `00002_audit.sql` | `//go:embed *.sql` + goose.SetBaseFS | ✓ WIRED | Both migrations embedded + applied |
| `runapp.go` (v1 watcher) | `parse.CP1252Reader` → Parse/ParseSpellbook | wrap disk file before parse (decode moved off shared entry) | ✓ WIRED | `parse.Parse(parse.CP1252Reader(f))` at L463; `parse.ParseSpellbook(parse.CP1252Reader(f))` at L548 |
| `deploy/squirebot-server.service` | the cross-compiled binary | `ExecStart=…serve --addr 127.0.0.1:8090 --db …` | ✓ WIRED | Flags match main.go's serve FlagSet + defaults exactly |
| `deploy/Caddyfile` | squirebot-server on localhost:8090 | reverse_proxy localhost:8090 | ✓ WIRED | Matches main.go loopback bind |
| `deploy/squirebot-backup.sh` | Cloudflare R2 (squirebot-backups) | rclone copy to r2: remote | ✓ WIRED | Live §6: object landed in bucket; cron installed |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `handler.go` ingest path | `rows` | `parse.Parse(strings.NewReader(env.Content))` → `ReplaceInventoryTx` INSERT → SQLite | Yes — real INSERTs proven by round-trip test querying rows back | ✓ FLOWING |
| `guard.go` resolveToken | `ownerID` | `SELECT owner_id, token_hash FROM guild_code WHERE disabled_at IS NULL` + constant-time match | Yes — resolves to the minting owner (TestResolveToken_ReturnsMintingOwner) | ✓ FLOWING |
| `scheduler.go` | (heartbeat only) | n/a — skeleton, no real jobs until P12 | N/A — intentional skeleton (in-scope; P12 populates) | ✓ (deferred by design) |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full module compiles | `go build ./...` | exit 0 | ✓ PASS |
| Backend + parse + server tests | `go test ./internal/backendsrv/... ./internal/parse/... ./cmd/squirebot-server/...` | all `ok` | ✓ PASS |
| Ingest round-trip + 401-writes-nothing + shrink + cross-owner | `go test ./internal/backendsrv/ingest/ -run TestIngest -v` | 11/11 PASS | ✓ PASS |
| Migrations idempotent + all tables + dimension empty | `go test ./internal/backendsrv/migrations/ -v` | 3/3 PASS | ✓ PASS |
| Auth mint hash-only + guard table (incl. revoked/malformed) | `go test ./internal/backendsrv/auth/ -v` | all PASS | ✓ PASS |
| PocketBase dep removed (FALLBACK verdict) | `grep pocketbase go.mod` / `test -d spike/pocketbase` | none / absent | ✓ PASS |
| Live ship-gate (authed POST→204, unauth→401, row queried back) | docs/backend-deploy.md §6 (executed on box 2026-05-29) | 204 / 401 / `ShipGateChar\|Rusty Dagger\|12345` | ✓ PASS (live evidence) |

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|----------------|-------------|--------|----------|
| BACKEND-01 | 11-05, 11-06 | Self-hosted always-on VPS + Caddy auto-HTTPS at the domain; single Go binary; in-process scheduler | ✓ SATISFIED | Binary half (build+tests) + deploy artifacts + live §6 cert/systemd/reboot |
| BACKEND-02 | 11-02 | SQLite + goose forward-only schema; separate owner/character; idempotent | ✓ SATISFIED | 00001/00002 SQL + migration tests + live §6 12 tables @ version 2 |
| BACKEND-03 | 11-03, 11-05 | Atomic full-snapshot ingest (clear+write, never row-diff) | ✓ SATISFIED | replace.go tx + handler round-trip tests + live §6 row queried back |
| BACKEND-04 | 11-04, 11-05 | Opaque per-guildie bearer token, minted + stored hashed; 401-writes-nothing | ✓ SATISFIED | mint/guard + 401 tests + live §6 unauth→401 |
| BACKEND-06 | 11-07 | Nightly off-box backup + documented restore | ✓ SATISFIED | backup.sh + restore runbook + live §6 R2 object/cron/restore drill |

All 5 Phase-11 requirements claimed by ≥1 plan; no orphaned requirements. 11-01 (spike) declares `requirements: []` intentionally (no production requirement; verdict = HAND-ROLLED Go, recorded in 11-CONTEXT).

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (none) | — | — | — | No TODO/FIXME/panic/placeholder-data anti-patterns in the hot path. "placeholder" matches are SQL `?` placeholders (the secure pattern) or the intentional, in-scope scheduler skeleton (P12 fills jobs). |

### Human Verification Required

None. The only items that would normally route to human verification (BACKEND-01/06 + ship-gate live on-box state) are covered by an accepted override: the autonomous:false LIVE-INFRASTRUCTURE deploy was actually executed on the real Hetzner box on 2026-05-29, the committed artifacts exist + match the binary's real CLI, and docs/backend-deploy.md §6 is the captured live evidence (corroborated by 11-06/11-07 SUMMARYs). Per the phase verification directive, §6 is treated as accepted captured evidence rather than an open uncertainty.

### Gaps Summary

No gaps. All 5 success criteria (BACKEND-01/02/03/04/06) are verified at every level: artifacts exist, are substantive, are wired, and data flows through them. The full module builds and the backend/parse/server test suites pass (round-trip, 401-writes-nothing, shrinking snapshot, cross-owner reject, idempotent migrations, hash-only mint, constant-time guard). The 11-01 PocketBase spike correctly resolved to HAND-ROLLED Go and the dep + spike tree were removed. The on-box deploy (Caddy/systemd/TLS) and backup/restore + ship gate are proven on the live Hetzner box with evidence in docs/backend-deploy.md §6.

**Observations (non-blocking, informational):**
- `.planning/REQUIREMENTS.md` still shows BACKEND-01 as "🚧 Partial (11-06 pending)" and BACKEND-06 as "Pending" in its traceability table — this is documentation staleness predating the 11-06/11-07 live completion, not a goal gap. Recommend updating the REQUIREMENTS.md status markers to ✅ for BACKEND-01/06 at phase close (and ROADMAP plan checkboxes for 11-06/11-07).
- The nightly backup schedule was installed via `/etc/cron.d/squirebot-backup` rather than a user crontab (the 11-07 acceptance criterion referenced `crontab -l`). This is a transparently-documented, functionally-equivalent deviation (same `0 4 * * *` schedule, more declarative) explained in both §6 and 11-07-SUMMARY — it satisfies the "nightly cron on a schedule" truth.

---

_Verified: 2026-05-29_
_Verifier: Claude (gsd-verifier)_
