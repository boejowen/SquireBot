# Phase 11: Backend Foundation + Ingest API - Pattern Map

**Mapped:** 2026-05-28
**Files analyzed:** 14 new/modified files (green-field Go backend)
**Analogs found:** 7 with real repo analogs / 14 total (7 have NO analog — new infra; use RESEARCH.md sketches)

> **Green-field warning.** This is a brand-new Go HTTP backend with **zero existing backend code** in the repo. Only the four ported/mirrored areas (parser, atomic-replace tx, structured logging, build/cross-compile) have genuine repo analogs. Everything else (Caddyfile, systemd unit, goose migrations, modernc DB-open, bearer-mint CLI, PocketBase bootstrap) is NEW — those rows point at the RESEARCH.md code sketches, NOT a fabricated analog. Do not force-fit a Sheets pattern onto SQLite where the only honest answer is "RESEARCH.md §X."
>
> **Module reuse, not copy.** `internal/parse` is `package parse` under module `github.com/boejowen/SquireBot`. The server lives in the same module, so for the parser the planner should prefer **direct import** (`import "github.com/boejowen/SquireBot/internal/parse"`) over copy — see the Encoding caveat (A1) before deciding.
>
> **Spike branch (D-01).** Plan 11-01 is the PocketBase-as-framework spike. If it PASSES, the route/tx/cron primitives shift to PocketBase APIs (`app.OnServe().BindFunc`, `app.RunInTransaction`, `app.Cron().MustAdd` — RESEARCH.md §"PocketBase Spike Probes"). If it FALLS BACK, they use the hand-rolled stdlib shapes below. **The analogs in this doc are framework-agnostic** (parser logic, the clear+write contract, slog, crypto, the single-writer discipline) — they apply either way.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/backendsrv/parse/` (or import `internal/parse`) | parser/utility | transform (text→rows) | `internal/parse/inventory.go`, `internal/parse/spellbook.go` | **exact** (same code, port/import) |
| `internal/backendsrv/store/replace.go` (atomic replace tx) | store | CRUD (full-snapshot replace) | `internal/sheet/write.go` (`WriteInventory`/`WriteSpellbook`) | role-match (contract port; Sheets→SQLite) |
| `internal/backendsrv/store/db.go` (modernc open + pragmas) | store/config | request-response (DB handle) | `internal/sheet/client.go` (`NewClient` + `batchMu` single-writer) | partial (single-writer discipline only; DSN is new) |
| `internal/backendsrv/auth/guard.go` (bearer guard) | middleware | request-response (authn) | `internal/auth/pkce.go` (sha256), `internal/sheet/owner.go` (first-write-wins) | role-match (crypto + identity policy) |
| `internal/backendsrv/auth/mint.go` (token gen) | utility/CLI | request-response (cred mint) | `internal/auth/pkce.go` (`NewPKCEPair` crypto/rand→base64url) | **exact** (token-gen shape) |
| `internal/backendsrv/auth/store.go` (hash storage/lookup) | store | CRUD (cred persistence) | `internal/auth/store.go` (DPAPI hash-only storage) | role-match (hash-only-storage discipline) |
| `internal/backendsrv/ingest/handler.go` (POST /api/v1/ingest) | controller/handler | request-response → CRUD | none — new (`net/http` ServeMux; see RESEARCH.md §Architecture) | **no analog** |
| `internal/backendsrv/ingest/binding.go` (first-sighting bind) | service | CRUD (identity bind) | `internal/sheet/owner.go` (`UpsertCharOwner` first-write-wins) | role-match (policy port; Sheets→SQLite) |
| `internal/backendsrv/migrations/00001_init.sql` (goose) | migration | schema DDL | none — new (`_meta.schema_version` was hand-rolled; goose replaces it) | **no analog** (use RESEARCH.md §Migration SQL Sketch) |
| `internal/backendsrv/migrations/embed.go` (goose-on-startup) | migration | startup | none — new | **no analog** (RESEARCH.md §Pattern 4) |
| `internal/backendsrv/scheduler/` (in-process cron skeleton) | scheduler | event-driven (no jobs yet) | `internal/heartbeat/heartbeat.go` (ticker goroutine) | partial — see note |
| `cmd/squirebot-server/main.go` (serve/mint/revoke dispatch) | entrypoint/CLI | request-response | `cmd/squirebot/main.go` (`os.Args` subcommand dispatch + `logging.Setup`) | role-match (CLI dispatch shape) |
| `Caddyfile`, `squirebot-server.service`, backup cron (`docs/backend-deploy.md`) | config/ops | request-response / batch | none — new ops artifacts | **no analog** (RESEARCH.md §Code Examples) |
| `internal/backendsrv/**/*_test.go` + temp-DB helper | test | — | `internal/parse/inventory_test.go`, `internal/sheet/write_test.go` | **exact** (table-test style) |

## Pattern Assignments

### `internal/backendsrv/parse/` — or direct import of `internal/parse` (parser, text→rows transform)

**Analog:** `internal/parse/inventory.go` + `internal/parse/spellbook.go` (EXACT — same logic, new home or direct import)

**The parser is the single highest-value reuse in this phase.** D-03 says the server parses raw text; these two pure functions already do exactly that. Both are `package parse`, so `import "github.com/boejowen/SquireBot/internal/parse"` makes "one source of parsing truth" literal — **no copy needed** unless the encoding caveat below forces a fork.

**Core inventory pattern** (`internal/parse/inventory.go` lines 26-55) — returns rows of EXACTLY 5 columns `[Location, Name, ID, Count, Slots]`, header auto-dropped when col 2 is non-numeric, bad-ID rows silently skipped, `(nil, nil)` for empty:
```go
func Parse(r io.Reader) (rows [][]string, err error) {
	decoded := charmap.Windows1252.NewDecoder().Reader(r) // <-- see ENCODING CAVEAT
	cr := csv.NewReader(decoded)
	cr.Comma = '\t'
	cr.FieldsPerRecord = -1 // tolerate any column count
	cr.LazyQuotes = true    // EQ names may contain stray apostrophes ("Tashan's Lance")
	all, err := cr.ReadAll()
	if err != nil { return nil, err }
	if len(all) == 0 { return nil, nil }
	if !isIntField(all[0], 2) { all = all[1:] } // drop header IF col 2 (ID) non-numeric
	out := make([][]string, 0, len(all))
	for _, row := range all {
		if len(row) < 5 { continue }
		if !isIntField(row, 2) { continue } // ID must parse as int; else skip
		out = append(out, row[:5])
	}
	return out, nil
}
```

**Spellbook pattern** (`internal/parse/spellbook.go` lines 43-73) — symmetric, returns `[Level, Name]` (2 cols), header dropped when col 0 (Level) non-numeric, non-int-Level rows skipped. `isIntField` is shared from `inventory.go` (same package).

**⚠️ ENCODING CAVEAT (RESEARCH.md §"Encoding Note" / Assumption A1 — load-bearing, resolve in 11-02 planning):**
The lines `charmap.Windows1252.NewDecoder().Reader(r)` (inventory.go:27, spellbook.go:44) exist **only because the watcher reads raw `.txt` bytes off disk**. Under D-03/D-04 the server receives `content` inside a **UTF-8 JSON body**. RESEARCH.md recommends **resolution 1**: the watcher (P13) pre-decodes CP1252→UTF-8, and the **server parser DROPS the `charmap` decode step** — feed `strings.NewReader(content)` straight into the `csv.Reader`. Because the existing functions hardwire the decoder, this likely means a thin **server-side parser variant** (or a parameterized reader) rather than a verbatim import. The planner must pick one of:
1. **Import + wrap:** keep `internal/parse` as-is for the watcher; add a server entry that hands already-UTF-8 bytes — but the decoder still runs (double-decode risk → mojibake). NOT clean.
2. **Refactor `Parse`/`ParseSpellbook` to take a pre-built `io.Reader`** and move the `charmap` decode to the watcher's read side. Cleanest "one source of truth." Touches existing watcher code (coordinate with P13).
3. **Fork a `backendsrv/parse` copy minus the decoder.** Two sources of truth (the thing D-03 explicitly avoids) — last resort.
Lock the contract as **`content` = UTF-8 text** in 11-02; flag for P13 so it doesn't double-decode.

**Test reuse** (`internal/parse/inventory_test.go` lines 1-183): the existing table tests (`TestParse_EmptyInput`, `_HeaderOnly`, `_HeaderPlusThreeRows`, `_FourColumnsSkipped`, `_SevenColumnsTruncates`, `_LazyQuotesApostrophe`, `_CP1252CurlyApostrophe`) are directly reusable as the parser's CI safety net. **Add UTF-8-`content` cases** (RESEARCH.md §Validation Architecture: "Reuse watcher's `internal/parse` table tests; add UTF-8-`content` cases"). The CP1252 test (lines 104-123) must be re-homed to the watcher under resolution 1/2.

---

### `internal/backendsrv/store/replace.go` (store, full-snapshot replace CRUD)

**Analog:** `internal/sheet/write.go` `WriteInventory` (lines 52-102) + `WriteSpellbook` (lines 131-181) — role-match; this is the **contract port** that turns the Sheets clear+write into one SQLite transaction.

**The contract being ported** (`internal/sheet/write.go` lines 32-51, the docstring): a single atomic `batchUpdate` carrying one `UpdateCellsRequest` over the FULL range — cells not covered by `rows` are **cleared as part of the same request**. This is the "never append, never row-diff, full-snapshot replace, atomic" contract from CLAUDE.md. The locked anti-pattern (lines 11-16): *"do not switch to a two-call values.batchClear + values.update pattern even if it looks simpler"* — the SQLite analog of that mistake is DELETE-then-INSERT in **separate** transactions. RESEARCH.md §Pattern 1 reimplements it as ONE `BEGIN IMMEDIATE … DELETE … INSERT … UPDATE … COMMIT`:

```go
// Source: RESEARCH.md §Pattern 1, derived from internal/sheet/write.go contract.
// Hand-rolled (fallback) shape; the PocketBase shape is app.RunInTransaction (RESEARCH §Criterion b).
tx, err := s.db.BeginTx(ctx, nil) // with _txlock=immediate DSN → BEGIN IMMEDIATE
defer tx.Rollback()               // no-op after Commit
tx.ExecContext(ctx, `DELETE FROM inventory_item WHERE character_id = ?`, charID) // shrink handled free
stmt, _ := tx.PrepareContext(ctx, `INSERT INTO inventory_item
    (character_id, location, name, item_id, count, slots, row_ordinal, uploaded_at)
    VALUES (?,?,?,?,?,?,?,?)`)
for i, r := range rows { // r = [Location, Name, ID, Count, Slots] from parse.Parse
    stmt.ExecContext(ctx, charID, r[0], r[1], atoi(r[2]), atoi(r[3]), atoi(r[4]), i, uploadedAt)
}
tx.ExecContext(ctx, `UPDATE character SET last_seen=?, watcher_version=? WHERE id=?`, uploadedAt, watcherVer, charID)
return tx.Commit()
```

**Mapping notes for the planner:**
- The watcher's `InventoryHeader`/`SpellbookHeader` (`write.go` lines 30, 109) become **column lists in the INSERT**, not header rows — SQLite has a real schema; there is no header row to write.
- The watcher's `toRowData` Pitfall #8 (`write.go` lines 183-197: *always StringValue, never NumberValue, even for ID/Count*) **does NOT carry over** — that hack existed to dodge Sheets recalc storms / leading-zero loss. In SQLite, `item_id`/`count`/`slots` are real `INTEGER` columns; parse explicitly (`strconv.Atoi`) per RESEARCH.md §Migration SQL Sketch. This is one place a Sheets pattern must be deliberately DROPPED, not copied.
- The parser already guarantees `r[2]` (ID) is int; the watcher's defensive 5-col padding (`write.go` lines 68-76) is unnecessary — the parser filters `<5` rows already.
- Add `row_ordinal` (file line order) per RESEARCH.md §Migration SQL Sketch — the Sheet preserved order implicitly via row position; SQLite needs an explicit ordinal column for stable display sort (P14).

---

### `internal/backendsrv/store/db.go` (store/config, DB handle open)

**Analog:** `internal/sheet/client.go` (lines 92-133) — **partial**: only the **single-writer serialization discipline** transfers; the modernc DSN itself is new.

The watcher funnels every Sheets call through one `batchMu sync.Mutex` (`client.go` lines 85-105) so "no two writes can interleave." The SQLite equivalent of that discipline is RESEARCH.md §Pattern 5's `db.SetMaxOpenConns(1)` + `_txlock=immediate` + `_pragma=busy_timeout(5000)` — same goal (serialize writes, kill `SQLITE_BUSY`), different mechanism. The planner should cite the watcher's mutex docstring (`client.go` lines 97-104) as the *precedent* for "this is a single-writer server by design," then use the DSN from RESEARCH.md §Pattern 5:

```go
// Source: RESEARCH.md §Pattern 5 — NEW (no DSN analog in repo).
dsn := "file:/var/lib/squirebot/squirebot.db?" +
    "_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)" +
    "&_pragma=foreign_keys(ON)&_pragma=synchronous(NORMAL)&_txlock=immediate"
db, _ := sql.Open("sqlite", dsn) // driver name "sqlite" (modernc) — NOT "sqlite3"
db.SetMaxOpenConns(1)            // single-writer server; matches the watcher's batchMu intent
```
**Foot-gun (RESEARCH.md Pitfall 3):** `sql.Open("sqlite", …)` (driver) vs `goose.SetDialect("sqlite3")` (dialect) — they differ on purpose. No repo analog; cite RESEARCH.md verbatim.

---

### `internal/backendsrv/auth/guard.go` + `mint.go` + `store.go` (middleware/utility/store, authn)

**Analog (token generation — EXACT):** `internal/auth/pkce.go` `NewPKCEPair` (lines 26-35). The deleted-PKCE crypto shape RESEARCH.md cites still exists and is the template for both the mint CLI and the hash compare:
```go
// internal/auth/pkce.go lines 27-34 — the crypto/rand → base64url + sha256 shape.
b := make([]byte, 32)
if _, err = rand.Read(b); err != nil { return "", "", err } // crypto/rand
verifier = base64.RawURLEncoding.EncodeToString(b)          // → 32-byte high-entropy token
sum := sha256.Sum256([]byte(verifier))                       // → stored as hash
```
The mint subcommand (RESEARCH.md §"CLI mint subcommand") is this exact shape: `rand.Read(32)` → `base64.RawURLEncoding` for the plaintext (shown once), `sha256.Sum256` for `guild_code.token_hash`.

**Analog (hash-only storage discipline — role-match):** `internal/auth/store.go` (lines 28-56). The watcher's `StoreToken`/`ReadToken` enforce "the secret never leaves as plaintext — only the hashed/encrypted blob is persisted" (file docstring + AUTH-04 comment). The bearer-token store inherits this discipline: store **only** `sha256(plaintext)` in `guild_code.token_hash`; the plaintext exists once, at mint time (D-05/D-08). Same security posture, different backing store (wincred DPAPI → SQLite BLOB column).

**Bearer guard — the compare** (RESEARCH.md §Pattern 3, stdlib `crypto/subtle`; NO repo analog for the constant-time compare itself):
```go
// Source: RESEARCH.md §Pattern 3 — crypto/sha256 + crypto/subtle. NEW.
sum := sha256.Sum256([]byte(rawToken))
// iterate active rows (disabled_at IS NULL), ConstantTimeCompare each:
if subtle.ConstantTimeCompare(sum[:], stored) == 1 { return ownerID, true }
// missing/malformed/unknown/disabled → (0,false) → 401, writes nothing (D-08/D-09)
```

**The PocketBase branch (D-01):** if the spike passes, the guard is a custom middleware `func(*core.RequestEvent) error` bound via `se.Router.POST(...).Bind(...)` — NOT PocketBase's `apis.RequireAuth()` / JWT auth-record system (guild codes are opaque static tokens). RESEARCH.md §Criterion (b) + Anti-Patterns is explicit: "Don't use PocketBase's auth-record/JWT system for guild codes."

---

### `internal/backendsrv/ingest/binding.go` (service, first-sighting identity bind)

**Analog:** `internal/sheet/owner.go` `UpsertCharOwner` (lines 59-142) — role-match; this is the **identity-policy port**.

The v1 policy (owner.go lines 7-13, the docstring): *"first-write wins for owner_email; subsequent mismatches → slog.Warn → _audit row; the write itself is NOT gated on email match."* D-07 ports this to bearer-token identity with one deliberate **tightening**: in v1 a mismatch was logged-but-allowed (12 racing watchers, OAuth email advisory); in v2 the **backend** owns the write (no race), so a cross-owner upload is **rejected** (409/403 + audit row), not just warned. RESEARCH.md §Pattern 2 is the SQLite shape:

```go
// Source: RESEARCH.md §Pattern 2, tightening internal/sheet/owner.go's first-write-wins.
var ownerID int64
err := tx.QueryRowContext(ctx, `SELECT owner_id FROM character WHERE name = ?`, charName).Scan(&ownerID)
switch {
case errors.Is(err, sql.ErrNoRows): // FIRST SIGHTING → bind char to this token's owner
    res, _ := tx.ExecContext(ctx, `INSERT INTO character (owner_id, name) VALUES (?, ?)`, tokenOwnerID, charName)
    charID, _ = res.LastInsertId()
case ownerID != tokenOwnerID: // CROSS-OWNER → reject + audit (v2 tightens v1's warn-and-allow)
    auditCrossOwnerReject(ctx, tx, charName, tokenOwnerID, ownerID)
    return ErrCharOwnedByAnother // handler maps to 409
default: // owner matches → proceed
}
```
**Mapping note:** the watcher's `UpsertCharOwner` does the lookup with a `valuesGet("_char_owner!A:B")` linear scan over sheet rows (owner.go lines 68-111); the SQLite version is a single indexed `SELECT … WHERE name = ?` (the `character_owner_idx` from the migration). The audit "row" (v1 `_audit` tab) becomes an append-only audit table/row in SQLite. The `CharOwnerServer = "blue"` constant (owner.go line 47) and the 14-column row shape are Sheets-specific and do NOT carry over.

---

### `cmd/squirebot-server/main.go` (entrypoint, subcommand dispatch)

**Analog:** `cmd/squirebot/main.go` (lines 26-76) — role-match for the **`os.Args`-based subcommand dispatch shape**.

The watcher's `main` dispatches sub-behaviors by sniffing `os.Args[1]` (`--uninstall-wipe-credentials` lines 39-55, `--quit` lines 69-76) and exiting early before the main loop. The server's `serve | mint-code | revoke-code` dispatch (RESEARCH.md §Recommended Project Structure) is the same shape — sniff `os.Args[1]`, run the CLI subcommand, exit; otherwise fall through to `goose.Up` + `http.ListenAndServe`. Also reuse the logging bootstrap: `log, _ := logging.Setup(); slog.SetDefault(log)` (main.go lines 111-112). **Note:** the watcher's `logging.Setup` writes to `%LOCALAPPDATA%\SquireBot` (Windows path, `internal/logging/logger.go` line 27) — the server runs on Linux and logs to stdout/journald (systemd captures it), so the server needs a Linux-appropriate slog handler, not the lumberjack-to-LOCALAPPDATA setup verbatim. Reuse the **slog JSON-handler pattern** (logger.go lines 47-51), not the Windows path.

---

### `internal/backendsrv/scheduler/` (scheduler, in-process cron skeleton — no jobs in P11)

**Analog:** `internal/heartbeat/heartbeat.go` — **partial** (ticker-goroutine precedent only).

P11 stands up only the **skeleton** (BACKEND-01: "in-process scheduler"; no real jobs until P12). The watcher's heartbeat is the repo's existing "long-running ticker goroutine" precedent. Under the PocketBase branch this is `app.Cron().MustAdd(id, expr, fn)` (RESEARCH.md §Criterion c); under fallback it's a `time.Ticker` or `robfig/cron` goroutine. Either registers a no-op/heartbeat job only. Low-value analog — RESEARCH.md §Don't-Hand-Roll says don't build a scheduler; this row is just "match the existing ticker-goroutine ergonomics."

## Shared Patterns

### Structured logging (`slog`, JSON, `log(level, op, fields)` convention)
**Source:** `internal/logging/logger.go` lines 47-53 (the JSON-handler construction) + the project's `slog.Info("op", "key", val, …)` call style seen throughout (e.g. `cmd/squirebot/main.go` lines 206-217, `internal/sheet/owner.go` lines 106-109).
**Apply to:** every backend package (handler, store, auth, scheduler, CLI).
```go
// internal/logging/logger.go lines 47-51 — reuse the JSON-handler shape (NOT the Windows path).
handler := slog.NewJSONHandler(w, &slog.HandlerOptions{ Level: slog.LevelInfo, AddSource: true })
logger := slog.New(handler)
slog.SetDefault(logger)
```
CLAUDE.md mandates structured `slog` server-side to keep logs greppable. **Security carry-over (RESEARCH.md §Security Domain V7):** the watcher's parser docstrings repeat *"never logs raw content (T-04-07)"* (`inventory.go` lines 23-25, `spellbook.go` lines 40-42). The server inherits this: **never log the raw bearer token, never log raw `content`.** Server-side on Linux → log to stdout/journald (systemd captures), not lumberjack-to-LOCALAPPDATA.

### Token crypto (`crypto/rand` → base64url; `crypto/sha256`; `crypto/subtle`)
**Source:** `internal/auth/pkce.go` lines 27-34 (rand+sha256+base64url) — the canonical repo shape; RESEARCH.md §Pattern 3 + §"CLI mint subcommand" for the `subtle.ConstantTimeCompare` half (no repo analog for the compare).
**Apply to:** `mint.go` (generate), `guard.go` (hash + compare), `store.go` (store hash only). RESEARCH.md §"Don't Hand-Roll": use `crypto/subtle.ConstantTimeCompare`, never `==` on hashes.

### Hash-only credential storage
**Source:** `internal/auth/store.go` lines 28-56 (the watcher stores only the DPAPI-encrypted blob; the secret never leaves as plaintext).
**Apply to:** `guild_code.token_hash` — store only `sha256(plaintext)`; plaintext shown once at mint (D-05/D-08).

### Single-writer DB serialization
**Source:** `internal/sheet/client.go` lines 85-105 (`batchMu` — "no two writes can interleave," the project's existing single-writer precedent).
**Apply to:** the SQLite handle (`db.SetMaxOpenConns(1)` + `_txlock=immediate`, RESEARCH.md §Pattern 5) — same intent, different mechanism.

### Table-test style + temp-DB helper
**Source:** `internal/parse/inventory_test.go` lines 1-183 (named `TestX_Scenario` funcs, `strings.NewReader`/`bytes.NewReader` inputs, `testdata/` fixtures) — matches the watcher's Go stdlib `testing` convention (RESEARCH.md §Validation Architecture).
**Apply to:** all `internal/backendsrv/**/*_test.go`. **New shared fixture needed (RESEARCH.md §Wave 0 Gaps):** a temp-DB helper that opens modernc with the WAL/foreign_keys DSN and runs `goose.Up` — reused across store/ingest/migration tests. modernc is pure Go, so these run in CI on the Windows dev box (no cgo, no live box).

### Cross-compile from Windows → linux/arm64
**Source:** `.goreleaser.yaml` lines 40-49 (`CGO_ENABLED=0`, `-s -w -trimpath`-style ldflags, `-X main.Version=…` injection) — the watcher's existing single-static-binary build. RESEARCH.md §"Cross-compile from Windows" gives the server's exact incantation.
**Apply to:** the server build. The watcher targets `GOOS=windows GOARCH=amd64`; the server flips to `GOOS=linux GOARCH=arm64` — **same `CGO_ENABLED=0` discipline** (this is why modernc, not mattn/go-sqlite3, is locked — D-02). Note the watcher's `release.yml` (not goreleaser) is the canonical CI build (`.goreleaser.yaml` lines 1-15); the server's CI/deploy story is NEW and deploy is manual (D-10: "drop the new binary + restart").
```powershell
# RESEARCH.md §Code Examples — server cross-compile (mirrors the watcher's CGO_ENABLED=0 ethos)
$env:GOOS="linux"; $env:GOARCH="arm64"; $env:CGO_ENABLED="0"
go build -trimpath -ldflags "-s -w" -o squirebot-server ./cmd/squirebot-server
```

## No Analog Found

Files/artifacts with NO close match in the codebase. The planner should cite the RESEARCH.md sketch in `read_first`, NOT invent a false analog.

| File / Artifact | Role | Data Flow | Reason no analog exists | Reference instead |
|------|------|-----------|-------------------------|-------------------|
| `internal/backendsrv/ingest/handler.go` | HTTP handler | request-response | Repo has **no HTTP server** — only an OAuth loopback + a localhost picker server (`internal/picker/server.go`, `internal/wizard/server.go`), which are transient browser-handshake servers, not a routed API. `net/http` ServeMux 1.22+ method+pattern routing is new here. | RESEARCH.md §Architecture Patterns diagram; §"Don't Hand-Roll" (ServeMux row); go.dev/blog/routing-enhancements |
| `internal/backendsrv/migrations/00001_init.sql` | goose migration | schema DDL | v1 had **no `ALTER TABLE`-capable DB** — schema evolution was the hand-rolled `_meta.schema_version` ↔ `WatcherMaxSchemaVersion` handshake (`internal/sheet/client.go` line 44). goose forward-only migrations are the standard answer that *replaces* that scheme (RESEARCH.md §State of the Art). | RESEARCH.md §"Migration SQL Sketch" (full D-13→SQLite DDL); §Pattern 4 |
| `internal/backendsrv/migrations/embed.go` | goose-on-startup | startup | New mechanism; `//go:embed migrations/*.sql` + `goose.SetDialect("sqlite3")` + `goose.Up`. (`assets/embed.go` embeds icons, NOT migrations — different use.) | RESEARCH.md §Pattern 4 (note the dialect/driver-name foot-gun, Pitfall 3) |
| `internal/backendsrv/store/db.go` (DSN) | DB open | request-response | modernc DSN + WAL/busy_timeout/foreign_keys pragmas are entirely new (v1 had no local DB). Only the *single-writer discipline* has a precedent (client.go `batchMu`). | RESEARCH.md §Pattern 5 |
| `Caddyfile` | reverse-proxy config | request-response | No web-server config in repo (v1 was Apps Script + a Windows tray app). | RESEARCH.md §"Minimal Caddyfile"; Pitfall 2 (Oracle two-layer firewall — iptables, NOT UFW) |
| `squirebot-server.service` (systemd unit) | process supervision | — | Repo's process model is Windows tray + `HKCU\…\Run` autostart — no systemd, no Linux service. | RESEARCH.md §"systemd unit" |
| `docs/backend-deploy.md` backup cron + restore | ops/batch | batch | No backup tooling in repo; `sqlite3 .backup` → Oracle Object Storage is new. **modernc cannot do `.backup`** (no C online-backup API) — must shell to the `sqlite3` CLI (RESEARCH.md Anti-Patterns). | RESEARCH.md §"Nightly backup cron", §"Documented restore", §"Don't Hand-Roll" (backup row) |
| PocketBase bootstrap (Plan 11-01 spike, IF adopted) | framework init | request-response | No PocketBase usage in repo (this is the spike). `pocketbase.New()` + `app.OnServe().BindFunc` + `app.RunInTransaction` + `app.Cron().MustAdd`. | RESEARCH.md §"PocketBase Spike Probes" (all four criteria, post-v0.23 API); Pitfall 1 (pre-1.0 churn) |

## Metadata

**Analog search scope:** entire Go module (`internal/**`, `cmd/**`), root build files (`Makefile`, `.goreleaser.yaml`, `go.mod`), CI workflows (`.github/workflows/*.yml`). Apps-script `node_modules` excluded as noise.
**Files scanned (read in full or targeted):** `internal/parse/inventory.go`, `internal/parse/spellbook.go`, `internal/parse/inventory_test.go`, `internal/sheet/write.go`, `internal/sheet/owner.go`, `internal/sheet/client.go` (targeted), `internal/auth/pkce.go`, `internal/auth/store.go`, `internal/logging/logger.go`, `cmd/squirebot/main.go`, `Makefile`, `.goreleaser.yaml`, `go.mod`.
**Key cross-cutting finding:** the repo gives **strong analogs for the parser, the full-snapshot-replace contract, slog, crypto, single-writer discipline, CLI dispatch, and cross-compile** — but **zero** for HTTP routing, goose migrations, the SQLite DSN, and all ops artifacts (Caddy/systemd/backup/PocketBase). The honest split is ~half port/mirror, half net-new-from-RESEARCH.
**Two deliberate "do NOT copy" carve-outs flagged for the planner:** (1) the Pitfall-#8 StringValue-everywhere hack in `write.go` (Sheets-only; SQLite uses real INTEGER columns); (2) the `charmap.Windows1252` decode in the parser (wire payload is UTF-8 JSON, not disk bytes — Assumption A1, resolve in 11-02).
**Pattern extraction date:** 2026-05-28

## PATTERN MAPPING COMPLETE

**Phase:** 11 - backend-foundation-ingest-api
**Files classified:** 14
**Analogs found:** 7 / 14 (7 are net-new infra with RESEARCH.md sketches as the reference)

### Coverage
- Files with exact/strong analog: 7 (parser, atomic-replace tx, token crypto, hash-storage, first-sighting bind, CLI dispatch, tests/cross-compile)
- Files with partial analog: 2 (DB-open single-writer discipline; scheduler ticker precedent)
- Files with NO analog (use RESEARCH.md): 7 (ingest HTTP handler, goose `00001_init.sql`, goose-embed, modernc DSN, Caddyfile, systemd unit, backup cron + PocketBase bootstrap)

### Key Patterns Identified
- **The parser ports near-verbatim from `internal/parse`** (same module, prefer direct import) — but the `charmap.Windows1252` decode must be reconsidered because the wire payload is UTF-8 JSON, not disk bytes (RESEARCH.md A1; the one genuine open question).
- **The atomic full-snapshot replace IS the watcher's clear+write contract** (`internal/sheet/write.go`) reimplemented as one `BEGIN IMMEDIATE … DELETE … INSERT … COMMIT` — and it's *stronger* than the Sheets version (real transaction, no reader-visible intermediate state). The Sheets-only StringValue hack must be dropped (real INTEGER columns).
- **Identity policy ports from `internal/sheet/owner.go` first-write-wins, tightened**: v1 warned-and-allowed cross-owner; v2 rejects (409 + audit) because the backend, not racing watchers, owns the write.
- **Crypto, slog, hash-only storage, single-writer discipline, and CGO_ENABLED=0 cross-compile all have direct repo precedents** (`auth/pkce.go`, `logging/logger.go`, `auth/store.go`, `sheet/client.go` `batchMu`, `.goreleaser.yaml`) — these are framework-agnostic and survive the spike verdict either way.
- **All HTTP/goose/DSN/ops artifacts are net-new** — the repo has no HTTP API, no migrations engine, no local DB, and no Linux/systemd/Caddy footprint; the planner must cite RESEARCH.md §§Architecture/Pattern 4/Pattern 5/Code Examples, not a fabricated analog.

### File Created
`.planning/phases/11-backend-foundation-ingest-api/11-PATTERNS.md`

### Ready for Planning
Pattern mapping complete. The planner can cite specific analog files + line ranges in each plan's `read_first`, knows which two Sheets patterns to deliberately drop, knows the parser-encoding decision to lock in 11-02, and knows which seven artifacts have no analog and must reference the RESEARCH.md sketches instead.
