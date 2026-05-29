# Phase 11: Backend Foundation + Ingest API - Research

**Researched:** 2026-05-28
**Domain:** Green-field Go HTTP backend (Oracle Cloud Always Free ARM64) — SQLite + goose + Caddy + bearer-token ingest; PocketBase-as-framework spike feasibility
**Confidence:** HIGH on the stack/versions/PocketBase-cgo question; HIGH on the ingest-transaction and auth patterns; MEDIUM on Oracle Always Free operational specifics (verify on provisioning).

## Summary

This is the first phase of v2.0 and stands up a brand-new Go backend with **zero existing backend code** in the repo. The decisive research finding is that **PocketBase v0.39.0 uses `modernc.org/sqlite v1.51.0` (pure Go, no cgo) and declares `go 1.25.0` in its own `go.mod`** [VERIFIED: github.com/pocketbase/pocketbase/go.mod]. This means the PocketBase spike's single biggest perceived risk — "does PocketBase's SQLite driver break the no-cgo cross-compile-from-Windows ethos?" — is a **non-issue**: PocketBase ships the *exact same driver* the hand-rolled fallback (D-02) would use. PocketBase's modern Go-framework API (`pocketbase.New()` → `app.OnServe().BindFunc()` → `e.Router.POST(...).Bind(...)`, `app.RunInTransaction()`, `app.Cron().MustAdd()`) maps cleanly onto all four spike criteria (D-01 a–d). The spike should pass on technical grounds; the real decision is whether PocketBase's *opinions* (its collections model, its admin UI, its auth-record JWT system you'd be bypassing) feel like leverage or friction for a maintainer who already ships single Go binaries.

Both paths converge on identical infrastructure: a single static Go binary cross-compiled `GOOS=linux GOARCH=arm64 CGO_ENABLED=0` from the Windows dev box, behind Caddy auto-HTTPS, under systemd `Restart=always`, with a SQLite file in WAL mode, goose forward-only migrations embedded via `//go:embed` and run on startup, per-guildie bearer tokens (SHA-256-hashed, constant-time compared), and a nightly `sqlite3 .backup` cron uploaded to Oracle Object Storage. The ingest endpoint reimplements the watcher's v1 full-snapshot atomic clear+write (`internal/sheet/write.go`) as a single SQLite transaction (`DELETE FROM …WHERE character_id=? ; INSERT …`), and the parsers (`internal/parse`) port near-verbatim — with **one encoding subtlety to resolve**: the watcher decodes Windows-1252 because it reads files off disk, but the server receives raw text inside a UTF-8 JSON body (D-03/D-04), so the CP1252 decode step must move to the *watcher's read side* or be made conditional on the wire.

**Primary recommendation:** Run the 11-01 spike against the four concrete PASS/FAIL probes in §"PocketBase Spike Probes" below. Expect technical PASS. Whichever path wins, lock the shared infra (modernc driver, goose-on-startup, Caddy+systemd, the firewall, nightly `.backup` cron) — those decisions stand either way (per D-01).

## Host Change Addendum (2026-05-29): Oracle Cloud Always Free → Hetzner Cloud VPS

**This research was written for an Oracle Cloud Always Free (Ampere A1, ARM64) host. The host was later switched to a Hetzner Cloud VPS (US) at the user's direction** — Heroku was evaluated first and rejected (no free tier + an ephemeral filesystem that destroys an on-disk SQLite store). See CONTEXT D-11/D-12/D-14. **The architecture is unchanged**: everything below about SQLite, `modernc.org/sqlite`, `goose`, the atomic-replace transaction, the bearer auth, Caddy, systemd, and the `sqlite3 .backup` discipline still applies verbatim. Only the host specifics change — **this addendum is authoritative where it conflicts with the Oracle-era text below:**

| Topic in this doc | Oracle-era text (superseded) | Hetzner reality (authoritative) |
|---|---|---|
| Host / instance | Oracle Always Free `VM.Standard.A1.Flex`, ARM64 | Hetzner Cloud shared-vCPU VPS, **US** location, **x86/AMD** (`CPX` line) — paid, always-on |
| Cross-compile | `GOARCH=arm64` | **`GOARCH=amd64`** (US x86 box; use `arm64` only if you pick the EU-only `CAX` line) |
| **Pitfall 2 — two-layer Oracle firewall / iptables-not-UFW** | the multi-hour trap | **DOES NOT APPLY.** `ufw` works normally on Hetzner: `ufw allow 22/80/443` then `ufw enable` (**allow SSH first** to avoid lockout). Optional Hetzner Cloud Firewall for defense-in-depth. |
| **Pitfall 5 — Oracle idle-instance reclamation** | 95th-pct CPU <20% reclaim risk; PAYG-exempt mitigation | **DOES NOT APPLY.** A paid always-on VPS — no reclamation, no keep-alive cron, no account-type decision. |
| Backup target (Backup & Restore, Don't-Hand-Roll) | Oracle Object Storage via `oci-cli` + keyless Instance Principal | **Cloudflare R2** (10 GB free, zero egress) via **`rclone`** (Backblaze B2 fallback). R2 needs a **bucket-scoped, write-only API token** on the box (`rclone.conf`, mode 600) — there is no keyless equivalent (security delta captured in the 11-07 threat model). |
| Cost | $0 backend / ~$12/yr total | ~$55/yr VPS + ~$12/yr domain ≈ **$67/yr** (the "$0 backend" premise is retired) |
| DNS | A-record → A1 public IP | A-record → **Hetzner VPS public IPv4** (shape unchanged) |

Everything else in this document is host-agnostic and stands as written.

<user_constraints>
## User Constraints (from CONTEXT.md)

> The user delegated **every** gray area (2026-05-28: *"I have no preference… use your recommendations for each"*). Everything below is LOCKED at Claude's recommendation. Research refines library versions and migration SQL only — the **choices** are fixed. Do NOT re-litigate.

### Locked Decisions

- **D-01 — Spike first.** Plan 11-01 is a time-boxed **1-day PocketBase-as-framework spike** (the FIRST plan). PASS requires all four: (a) models `owner`/`character`/`inventory_item`/`spellbook_entry` + empty dimension tables; (b) per-guildie bearer-token ingest via a custom route/hook doing atomic full-snapshot replace; (c) can host the P12 in-process enrichment cron via Go hooks (`pocketbase.New()`); (d) runs on Oracle ARM64. All four pass → adopt PocketBase. Any **hard blocker** → hand-rolled Go fallback. Host (Oracle Always Free) + DB (SQLite) stand either way. **Capture the verdict in an 11-01 SUMMARY / appended CONTEXT note.**
- **D-02 — Hand-rolled fallback shape.** Go (1.24+ per CONTEXT; repo `go.mod` is `go 1.25.0`, toolchain 1.26 installed — see Assumptions), stdlib `net/http` ServeMux (1.22+ method+pattern routing, **no router dependency**), pure-Go SQLite driver **`modernc.org/sqlite` (NO cgo)** → cross-compile `GOOS=linux GOARCH=arm64` from Windows into a single static binary.
- **D-03 — Server parses.** Watcher POSTs the **RAW `/outputfile` file text**; the server parses. Thinnest watcher; one source of parsing truth, shared with P12.
- **D-04 — Ingest contract.** `POST /api/v1/ingest` with JSON envelope `{ "character": "<name>", "kind": "inventory"|"spellbook", "content": "<raw file text>", "watcher_version": "x.y.z" }`. Char name comes from the watcher's *filename*, not the file body. Server parses `content`, validates, **atomically replaces** that `(character, kind)`'s rows in one transaction (delete-all-then-insert; shrinking snapshot drops removed rows). `watcher_version` accepted now; version-gate *reject* lands in P13.
- **D-05 — Mint via server CLI.** `squirebot-server mint-code --owner <label>` prints plaintext **once**; store only a **SHA-256 hash**. No admin HTTP endpoint in P11 (that's P15).
- **D-06 — Distribution.** Maintainer **DMs the code over Discord**.
- **D-07 — First-sighting binds** each new character name to the uploading code's owner (mirrors v1 first-write-wins). A character already owned by a *different* owner → reject with clear error + audit log (reassignment is P15 admin action).
- **D-08 — Auth header.** `Authorization: Bearer <code>`; constant-time hash compare; missing/malformed/unknown → **401, writes nothing**.
- **D-09 — Revocation.** Disable/delete the hashed token row (CLI subcommand); a revoked code → 401.
- **D-10 — Deploy.** Bare Go binary + systemd (`Restart=always`) + Caddy (auto-HTTPS, reverse-proxy to localhost). **No Docker.** Migrations embedded via `//go:embed`, run `goose.Up` on startup. Deploy = "drop the new binary + restart."
- **D-11 — Backup.** Nightly cron `sqlite3 .backup` (consistent snapshot) → Oracle Object Storage (Always Free 10 GB). Documented restore. Litestream noted as a future upgrade.
- **D-12 — Instance.** Oracle `VM.Standard.A1.Flex`, 1 OCPU / 6 GB, US home region. User provisions.
- **D-13 — Schema shape.** `owner`(id, label, created_at) · `character`(id, owner_id FK, name UNIQUE, + nullable class/level/race/is_bank_toon/is_hidden/is_removed) · `inventory_item`(character_id FK, location, name, item_id, count, slots, uploaded_at) · `spellbook_entry`(character_id FK, level, name, uploaded_at) · `guild_code`(id, owner_id FK, token_hash, label, disabled_at). **Empty** dimension tables too (`item_master`, `pigparse_price`, `wiki_spells`, `wiki_gear_tier`, `quest_items`). goose forward-only; idempotent on a fresh DB.

### Claude's Discretion

- Exact library **versions** (this doc pins current ones), exact **migration SQL**, exact column types/indexes within the D-13 shape, the in-process scheduler skeleton's exact shape, and the structure of the CLI subcommands. The **choices** above are locked.

### Deferred Ideas (OUT OF SCOPE for P11)

- Admin HTTP endpoints for minting / reassignment → **P15** (P11 uses a server CLI).
- Read API + the 4 views → **P14** (BACKEND-05).
- Enrichment cron *jobs* + dimension-data *population* → **P12** (P11 stands up only the scheduler **skeleton** + **empty** tables — populates nothing).
- Watcher re-target / OAuth deletion / guild-code onboarding → **P13**.
- `999.22` SemVer-aware auto-update → P13/P16.
- Litestream continuous replication → future backup upgrade.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| BACKEND-01 | Oracle Always Free (Ampere A1, ARM64) + Caddy auto-HTTPS, reachable over TLS; single Go binary; in-process scheduler | §Ops & Deploy (Caddy minimal Caddyfile, systemd unit, two-layer Oracle firewall, cross-compile), §Architecture Patterns (scheduler skeleton) |
| BACKEND-02 | SQLite schema under `goose` forward-only migrations; separate `owner`/`character`; `goose up` idempotent on fresh DB | §Standard Stack (goose v3.27.1, modernc v1.51.0), §Code Examples (embed + SetDialect("sqlite3") + Up), §Migration SQL Sketch |
| BACKEND-03 | Atomic full-snapshot ingest — replaces a character's rows; never row-diffs; shrinking snapshot drops rows | §Atomic Ingest Transaction (DELETE+INSERT in one tx), §Code Examples (both PocketBase `RunInTransaction` and hand-rolled `*sql.Tx`), parser port from `internal/parse` |
| BACKEND-04 | Per-guildie opaque bearer token; minted by maintainer; stored hashed | §Bearer-Token Auth (SHA-256 + `subtle.ConstantTimeCompare`, CLI mint pattern, first-sighting bind, cross-owner reject) |
| BACKEND-06 | Nightly off-box backup + documented restore | §Backup & Restore (`sqlite3 .backup` hot snapshot, gzip, oci-cli/rclone upload, restore steps) |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| TLS termination + HTTP→HTTPS + cert renewal | Caddy (reverse proxy) | — | Caddy owns ACME/Let's Encrypt; the Go binary never touches TLS. Reverse-proxy to `localhost:<port>`. |
| HTTP routing + request parsing | Go backend (API tier) | — | `net/http` ServeMux (fallback) or PocketBase router (spike). |
| Bearer-token authn (custom guard) | Go backend (API tier) | DB (hash lookup) | Custom middleware reads `Authorization` header, hashes, constant-time compares against `guild_code.token_hash`. **NOT** PocketBase's built-in JWT auth. |
| `/outputfile` text → rows parsing | Go backend (API tier) | — | D-03: server parses. Ported `internal/parse` logic. |
| Atomic full-snapshot replace | DB / Storage tier | API tier (tx orchestration) | One SQLite transaction (`DELETE` then bulk `INSERT`) keyed on `character_id`. |
| Persistence (owner/char/inventory/spellbook/dimension) | DB / Storage tier | — | SQLite file on local disk, WAL mode. |
| Schema migration on deploy | Go backend (startup) | DB tier | `goose.Up` from embedded FS at process start. |
| Token minting / revocation | Go backend (CLI subcommand) | DB tier | Out-of-band CLI (`mint-code`/`revoke-code`); no HTTP surface in P11. |
| In-process scheduling (skeleton only) | Go backend (long-running process) | — | `app.Cron()` (PocketBase) or a `time.Ticker`/`robfig/cron` goroutine (fallback). Registers no real jobs in P11. |
| Nightly backup + off-box copy | OS (cron) + `sqlite3` CLI + oci-cli/rclone | Oracle Object Storage | Shell cron invokes `sqlite3 .backup`; NOT done from Go (modernc lacks the C backup API). |
| Process supervision / restart-on-reboot | systemd | — | `Restart=always`, `After=network-online.target`. |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go toolchain | 1.25+ (repo `go.mod` = `go 1.25.0`; 1.26.2 installed) | Backend language; 1.22+ required for ServeMux method+pattern routing | [VERIFIED: `go version` → go1.26.2; `go.mod` line 3] Matches the watcher's single-static-binary ethos. |
| `modernc.org/sqlite` | **v1.51.0** | Pure-Go SQLite driver (no cgo) | [VERIFIED: `go list -m -versions` → latest v1.51.0] Enables `CGO_ENABLED=0` cross-compile from Windows → linux/arm64. Driver name registered as **`"sqlite"`**. **PocketBase v0.39.0 depends on this exact version** [VERIFIED: pocketbase go.mod]. |
| `github.com/pressly/goose/v3` | **v3.27.1** | Forward-only DB migrations, embeddable, `//go:embed` support | [VERIFIED: `go list -m -versions` → latest v3.27.1] Per finding 04 §3.2 it is "the friendlier choice for a solo maintainer." Go-native, supports Go-code migrations for backfills. |
| Caddy | v2.x (apt package) | Auto-HTTPS reverse proxy | [CITED: caddyserver.com/docs] Zero-config Let's Encrypt; minimal 2-line Caddyfile; single static binary; runs as `caddy.service` systemd unit on apt install. |

### Spike-only (Plan 11-01)
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/pocketbase/pocketbase` | **v0.39.0** | Go-framework backend (SQLite + REST + admin UI + hooks + cron), used as a *library* via `pocketbase.New()` | [VERIFIED: `go list -m -versions` → latest v0.39.0] ONLY if the 11-01 spike PASSes all four D-01 criteria. **Still v0.x — pre-1.0 API churn is a real risk (see Pitfall 1).** Uses `modernc.org/sqlite v1.51.0` internally (no cgo) [VERIFIED: pocketbase go.mod]. |

### Supporting (standard library — no dependency needed)
| Package | Purpose | When to Use |
|---------|---------|-------------|
| `net/http` (`ServeMux`) | HTTP routing with `"POST /api/v1/ingest"` method+pattern syntax, `r.PathValue(...)` | Fallback path (D-02). No router dep. [CITED: go.dev/blog/routing-enhancements] |
| `crypto/sha256` | Hash the bearer token for storage + compare | Both paths. D-05/D-08. |
| `crypto/subtle` (`ConstantTimeCompare`) | Timing-safe hash comparison | Both paths. D-08. |
| `crypto/rand` | Generate the high-entropy guild code (32 bytes → base64url) | CLI mint subcommand. The watcher's deleted `auth/pkce.go` `newState()` already used this shape (finding 03 §2.2). |
| `database/sql` + `embed` | DB handle + embedded migration FS | Fallback path; also used alongside goose. |
| `encoding/csv` + `golang.org/x/text/encoding/charmap` | Parser port (see Encoding Note) | Server-side parsing of `content`. Already in repo for the watcher. |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `modernc.org/sqlite` (no cgo) | `mattn/go-sqlite3` (cgo) | **Rejected by D-02 and CLAUDE.md no-cgo ethos.** cgo breaks the cross-compile-from-Windows workflow (needs an ARM64 C cross-toolchain). modernc is the locked choice. |
| `net/http` ServeMux | chi / gin / echo | **Rejected by D-02** (no router dependency). Go 1.22+ ServeMux covers method+pattern routing natively; a 12-user backend needs nothing more. |
| `goose` | `golang-migrate` | Both fine (finding 04 §3.2); goose locked for Go-code-migration support + solo-maintainer friendliness. |
| In-process scheduler (`app.Cron` / `robfig/cron` / `time.Ticker`) | System `cron` invoking the binary with a flag | BACKEND-01 explicitly says "in-process scheduler." In-process keeps everything in one supervised process and one log stream. (System cron is still used for the *backup* per D-11 — that's shell, not a Go job.) |
| Caddy | nginx + certbot | Caddy's auto-HTTPS is materially less config than nginx+certbot; locked by D-10. |

**Installation (fallback path):**
```bash
go get modernc.org/sqlite@v1.51.0
go get github.com/pressly/goose/v3@v3.27.1
# net/http, crypto/* are stdlib — no install
```

**Installation (spike path, 11-01 only):**
```bash
go get github.com/pocketbase/pocketbase@v0.39.0
# pulls modernc.org/sqlite v1.51.0 transitively — no cgo
```

**Version verification performed (2026-05-28):**
- `modernc.org/sqlite` → v1.51.0 (latest) [VERIFIED: go module proxy]
- `github.com/pressly/goose/v3` → v3.27.1 (latest) [VERIFIED]
- `github.com/pocketbase/pocketbase` → v0.39.0 (latest) [VERIFIED]
- Go toolchain installed → go1.26.2; repo declares `go 1.25.0` [VERIFIED: `go version`, `go.mod`]

## PocketBase Spike Probes (Plan 11-01 — the gating plan)

The spike answers D-01's four criteria. Below are **concrete PASS/FAIL probes** and what a **hard blocker** looks like. The headline finding (no-cgo driver shared with the fallback) means criteria (a) and (d) are essentially pre-validated; criteria (b) and (c) are where genuine fit-or-friction lives.

### Criterion (a) — Models owner/character/inventory_item/spellbook_entry + empty dimension tables
**Probe:** Define collections via a Go migration (`m.Register`) or the admin UI, OR run raw DDL via `app.DB().NewQuery("CREATE TABLE …").Execute()` inside a migration. PocketBase stores everything in its own SQLite file; you can mix native PB "collections" with plain SQL tables.
**PASS:** Tables exist; `goose`-style forward-only migration shape is expressible (PocketBase has its own Go-migration registry — `migrations.Register(up, down)` — which is forward/back like goose).
**Possible friction (not a hard blocker):** PocketBase wants its records to be *collections* (with auto `id`/`created`/`updated` system fields and a JSON-schema definition) if you want the admin UI + auto-REST to see them. Plain SQL tables created via raw DDL are invisible to the admin UI and the auto-REST layer. **Decision input:** if you want the dimension tables manageable in PB's admin UI later (P15), define them as collections; if they're purely cron-populated (P12) and API-served (P14), plain tables are fine. Either works technically.
**Source:** [CITED: pocketbase.io/docs/go-migrations, go-collections]

### Criterion (b) — Per-guildie bearer-token ingest via custom route doing atomic full-snapshot replace
**Probe (route + custom guard):**
```go
app.OnServe().BindFunc(func(se *core.ServeEvent) error {
    se.Router.POST("/api/v1/ingest", ingestHandler).
        Bind(&hook.Handler[*core.RequestEvent]{ // custom bearer guard, NOT apis.RequireAuth()
            Func: bearerGuard,
        })
    return se.Next()
})
```
The custom guard reads `e.Request.Header.Get("Authorization")`, strips `Bearer `, SHA-256-hashes, constant-time compares against `guild_code` rows, and either calls `e.Next()` or returns `e.UnauthorizedError(...)`. **You do NOT use PocketBase's auth-record/JWT system** — guild codes are opaque static tokens, not PB auth records. This is fully supported: PB middleware is just `func(*core.RequestEvent) error` that calls `e.Next()`. [VERIFIED via Context7: "register a global middleware … `se.Router.BindFunc`"; "auth token loader retrieves the token from the `Authorization` header" — you bypass that loader by not binding `RequireAuth`.]
**Probe (atomic replace):**
```go
app.RunInTransaction(func(txApp core.App) error {
    if _, err := txApp.DB().NewQuery(
        "DELETE FROM inventory_item WHERE character_id = {:cid}").
        Bind(dbx.Params{"cid": charID}).Execute(); err != nil { return err }
    for _, row := range parsedRows {
        rec := core.NewRecord(invCollection)
        rec.Set("character_id", charID); rec.Set("location", row.Location) // …
        if err := txApp.SaveNoValidate(rec); err != nil { return err }
    }
    return nil // commit; any error rolls back
})
```
[VERIFIED via Context7: `app.RunInTransaction(fn)` — "operations persisted only if the fn returns nil"; "always use `txApp` within the transaction to avoid deadlocks"; raw `DELETE` via `app.DB().NewQuery(...).Execute()`; `core.NewRecord` + `Save`.]
**PASS:** Custom-guarded route returns 401 on bad token, 200 on good; a re-upload of a shrunk snapshot leaves only the new rows.
**HARD BLOCKER would look like:** PB forcing its JWT auth on all routes (it does not — `RequireAuth` is opt-in per route), or `RunInTransaction` not exposing raw SQL (it does, via `txApp.DB()`).

### Criterion (c) — Host the P12 in-process enrichment cron via Go hooks
**Probe:**
```go
app.Cron().MustAdd("pigparse-daily", "0 3 * * *", func() { /* P12 job */ })
```
[VERIFIED via Context7: `app.Cron().MustAdd(id, cronExpr, handler)`; "each scheduled job runs in its own goroutine"; "automatically started when the app serves."]
**PASS:** A test job registered with a `*/1 * * * *` expr fires on schedule while the server runs. (P11 registers only a no-op/heartbeat skeleton; P12 fills it in.)
**HARD BLOCKER:** none plausible — `app.Cron()` is a documented first-class API.

### Criterion (d) — Runs on Oracle ARM64
**Probe:** Cross-compile `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build` from Windows; scp to the A1 box; run.
**PASS:** Binary runs; admin UI reachable; SQLite file created.
**Pre-validated:** PocketBase's only SQLite dependency is `modernc.org/sqlite` (pure Go) [VERIFIED: pocketbase go.mod], so `CGO_ENABLED=0` arm64 builds work exactly like the watcher's. PocketBase publishes official linux/arm64 release binaries, confirming arm64 is a supported target. [CITED: github.com/pocketbase/pocketbase/releases]
**HARD BLOCKER:** none plausible given the no-cgo driver.

### Spike verdict guidance
All four criteria are technically achievable with PB v0.39.0. The honest decision axis is **leverage vs. opinion**:
- **Adopt PB if:** you value the free admin UI (useful for P15 forms / eyeballing data during cutover), auto-REST for the read API (P14), and built-in Discord OAuth2 (P15 / AUTH-08) — PB has OAuth2 providers including Discord, which could compress P15.
- **Fall back to hand-rolled if:** bypassing PB's auth-record model for opaque guild codes, mixing raw SQL tables with PB collections, and pinning a pre-1.0 dependency that "took 1h–a weekend to migrate" at v0.23 [CITED: pocketbase.io/v023upgrade/go] feels like more friction than writing ~600 LOC of `net/http`. The fallback is genuinely small and the maintainer already ships Go binaries.

**Record the verdict + reasoning in `11-01-SUMMARY.md` and append a note to `11-CONTEXT.md` (per D-01).** Plans 11-02+ branch on this verdict.

## Architecture Patterns

### System Architecture Diagram

```
                          Internet (HTTPS :443)
                                  │
                                  ▼
                        ┌───────────────────┐
                        │   Caddy (systemd)  │  auto-HTTPS / Let's Encrypt
                        │   your.domain {…}  │  TLS termination
                        └─────────┬─────────┘
                                  │ reverse_proxy localhost:8090 (plain HTTP, loopback)
                                  ▼
        ┌──────────────────────────────────────────────────────────┐
        │   squirebot-server  (single Go binary, systemd Restart=always) │
        │                                                            │
        │   startup ──► goose.Up(embedded FS) ──► migrations applied  │
        │                                                            │
        │   POST /api/v1/ingest                                      │
        │     │                                                      │
        │     ├─[1] bearerGuard middleware ─► hash(Authorization)    │
        │     │      └─ constant-time vs guild_code.token_hash       │
        │     │         └─ miss/disabled ─► 401 (writes nothing) ────┼──► (end)
        │     │                                                      │
        │     ├─[2] decode JSON envelope {character,kind,content,ver}│
        │     ├─[3] parse content ─► [][]string  (ported internal/parse)│
        │     ├─[4] resolve owner (from token) + first-sighting bind │
        │     │      └─ char owned by OTHER owner ─► 409/403 + audit  │
        │     └─[5] RunInTransaction:                                │
        │             DELETE FROM inventory_item WHERE character_id=? │
        │             INSERT … (all parsed rows)                      │
        │             UPDATE character SET last_seen, watcher_version │
        │                                                            │
        │   app.Cron()  ─► (skeleton; no real jobs until P12)        │
        │                                                            │
        │   CLI subcommands (out-of-band, not HTTP):                 │
        │     mint-code --owner <label>  ─► print plaintext ONCE     │
        │     revoke-code <id|label>     ─► set disabled_at           │
        └───────────────────────────┬──────────────────────────────┘
                                     │
                                     ▼
                        ┌────────────────────────┐
                        │  SQLite file (WAL mode) │  squirebot.db + -wal + -shm
                        │  owner, character,      │
                        │  inventory_item,        │
                        │  spellbook_entry,       │
                        │  guild_code,            │
                        │  + EMPTY dimension tbls │
                        └───────────┬────────────┘
                                    │
                  nightly OS cron   │ sqlite3 db ".backup '/tmp/snap.db'" → gzip
                                    ▼
                        ┌────────────────────────┐
                        │  Oracle Object Storage  │  (Always Free 10 GB)
                        │  via oci-cli / rclone   │  off-box snapshot
                        └────────────────────────┘
```

### Recommended Project Structure (fallback path; spike path is similar)
```
cmd/
  squirebot-server/
    main.go              # flag parse: serve | mint-code | revoke-code; goose.Up; http.ListenAndServe
internal/
  backendsrv/            # NEW server packages (mirrors watcher's internal/ layout)
    ingest/              # the POST /api/v1/ingest handler + envelope decode
    auth/                # bearer guard, SHA-256, constant-time compare, token mint
    store/               # *sql.DB open (modernc DSN), the atomic-replace tx, queries
    parse/               # ported from watcher internal/parse (or import it directly)
    scheduler/           # in-process cron skeleton (no jobs yet)
    migrations/          # *.sql goose files, //go:embed'd
      00001_init.sql
  parse/                 # EXISTING watcher parser — candidate to share (see Reuse note)
docs/
  backend-deploy.md      # NEW: Oracle provisioning, Caddy, systemd, firewall, restore runbook
```
**Reuse note:** `internal/parse` is `package parse` under module `github.com/boejowen/SquireBot` — the server can **import it directly** (`import "github.com/boejowen/SquireBot/internal/parse"`) since both live in the same module. No copy needed. This makes "one source of parsing truth" (D-03) literal. ⚠️ But see the Encoding Note — the *decode* step must be reconsidered for the wire path.

### Pattern 1: Atomic full-snapshot replace (the core of BACKEND-03)
**What:** Reimplement the watcher's `WriteInventory`/`WriteSpellbook` (one `batchUpdate` clear+write) as one SQLite transaction.
**When to use:** Every ingest. Never row-diff; never append.
**Hand-rolled example:**
```go
// Source: derived from internal/sheet/write.go contract + finding 04 §1.2
func (s *Store) ReplaceInventory(ctx context.Context, charID int64, rows [][]string, uploadedAt time.Time, watcherVer string) error {
    tx, err := s.db.BeginTx(ctx, nil) // with _txlock=immediate DSN, this is BEGIN IMMEDIATE
    if err != nil { return err }
    defer tx.Rollback() // no-op after Commit
    if _, err := tx.ExecContext(ctx, `DELETE FROM inventory_item WHERE character_id = ?`, charID); err != nil {
        return err
    }
    stmt, err := tx.PrepareContext(ctx, `INSERT INTO inventory_item
        (character_id, location, name, item_id, count, slots, row_ordinal, uploaded_at)
        VALUES (?,?,?,?,?,?,?,?)`)
    if err != nil { return err }
    defer stmt.Close()
    for i, r := range rows { // r = [Location, Name, ID, Count, Slots] from parse.Parse
        itemID, _ := strconv.Atoi(r[2]) // parser guarantees r[2] is int
        cnt, _ := strconv.Atoi(r[3])
        slots, _ := strconv.Atoi(r[4])
        if _, err := stmt.ExecContext(ctx, charID, r[0], r[1], itemID, cnt, slots, i, uploadedAt); err != nil {
            return err
        }
    }
    if _, err := tx.ExecContext(ctx, `UPDATE character SET last_seen=?, watcher_version=? WHERE id=?`,
        uploadedAt, watcherVer, charID); err != nil {
        return err
    }
    return tx.Commit()
}
```
A shrinking snapshot (item dropped) is handled for free by the `DELETE`. This is *better* than the sheet — a real `BEGIN/COMMIT` guarantees no reader-visible intermediate state (the sheet's single `batchUpdate` only approximated it). [CITED: finding 04 §1.2]

### Pattern 2: First-sighting owner binding (D-07)
**What:** On ingest, resolve the owner from the bearer token. Look up `character` by `name`. If absent → insert bound to this owner. If present and `owner_id` matches → proceed. If present and `owner_id` differs → reject (409/403) + write an audit log row; do NOT overwrite.
**Why:** Mirrors v1 first-write-wins (`internal/sheet/owner.go` `UpsertCharOwner`), but the backend — not 12 racing watchers — owns the write, so the race class disappears (finding 04 §1.1).
```go
// inside the ingest tx, before ReplaceInventory
var ownerID int64
err := tx.QueryRowContext(ctx, `SELECT owner_id FROM character WHERE name = ?`, charName).Scan(&ownerID)
switch {
case errors.Is(err, sql.ErrNoRows):
    res, _ := tx.ExecContext(ctx, `INSERT INTO character (owner_id, name) VALUES (?, ?)`, tokenOwnerID, charName)
    charID, _ = res.LastInsertId()
case err != nil:
    return err
case ownerID != tokenOwnerID:
    auditCrossOwnerReject(ctx, tx, charName, tokenOwnerID, ownerID) // append-only audit
    return ErrCharOwnedByAnother // handler maps to 409 + clear message
default:
    // charID lookup, proceed
}
```

### Pattern 3: Bearer guard with constant-time compare (D-08)
```go
// Source: crypto/sha256 + crypto/subtle (stdlib); finding 03 §2.2
func (a *Auth) resolveToken(ctx context.Context, authHeader string) (ownerID int64, ok bool) {
    const p = "Bearer "
    if !strings.HasPrefix(authHeader, p) { return 0, false }       // malformed → 401
    raw := strings.TrimPrefix(authHeader, p)
    sum := sha256.Sum256([]byte(raw))                              // hash the presented code
    // Fetch candidate active rows; compare each hash in constant time.
    rows, err := a.db.QueryContext(ctx, `SELECT owner_id, token_hash FROM guild_code WHERE disabled_at IS NULL`)
    if err != nil { return 0, false }
    defer rows.Close()
    for rows.Next() {
        var oid int64; var stored []byte
        if err := rows.Scan(&oid, &stored); err != nil { continue }
        if subtle.ConstantTimeCompare(sum[:], stored) == 1 {       // timing-safe
            return oid, true
        }
    }
    return 0, false                                                // unknown → 401
}
```
Note: iterating all active rows keeps the compare constant-time per row; at ~12 codes this is trivially cheap. Alternatively store `token_hash` as the PK and do a direct lookup — a hash lookup is not itself timing-sensitive (the secret is already hashed), but using `ConstantTimeCompare` on the final bytes is belt-and-braces and matches the ASVS expectation.

### Pattern 4: goose-on-startup with embedded FS (D-10, BACKEND-02)
```go
// Source: Context7 /pressly/goose — "Bundle Migrations with Go Embed"
//go:embed migrations/*.sql
var embedMigrations embed.FS

func runMigrations(db *sql.DB) error {
    goose.SetBaseFS(embedMigrations)
    if err := goose.SetDialect("sqlite3"); err != nil { return err } // ⚠️ "sqlite3" dialect, NOT "sqlite"
    return goose.Up(db, "migrations")
}
```
⚠️ **Foot-gun (see Pitfall 3):** the **driver name** passed to `sql.Open` is **`"sqlite"`** (modernc), but the **goose dialect string** is **`"sqlite3"`**. They are independent. [VERIFIED: pkg.go.dev/github.com/pressly/goose/v3 dialect list; modernc registers driver `"sqlite"`].

### Pattern 5: modernc DSN with the right pragmas
```go
// Source: WebSearch (modernc DSN pragma syntax) + sqlite.org/pragma.html
dsn := "file:/var/lib/squirebot/squirebot.db?" +
    "_pragma=journal_mode(WAL)" +      // concurrent read-during-write
    "&_pragma=busy_timeout(5000)" +    // wait 5s on lock instead of erroring (single-writer safety)
    "&_pragma=foreign_keys(ON)" +      // NOT persistent — must be in DSN so every pooled conn gets it
    "&_pragma=synchronous(NORMAL)" +   // safe with WAL; faster than FULL
    "&_txlock=immediate"               // BEGIN IMMEDIATE for write txns — avoids SQLITE_BUSY upgrade deadlock
db, err := sql.Open("sqlite", dsn)
db.SetMaxOpenConns(1) // OPTIONAL but recommended for a single-writer server: serializes writes, kills SQLITE_BUSY entirely
```
The `_txlock=immediate` + `SetMaxOpenConns(1)` combination is the canonical cure for SQLite "database is locked" under a single-writer server. [CITED: berthub.eu, tenthousandmeters.com on SQLITE_BUSY]

### Anti-Patterns to Avoid
- **Don't decode CP1252 on the server unconditionally** (see Encoding Note) — the wire payload is UTF-8 JSON, not a raw CP1252 file. Wrong decode = mojibake.
- **Don't use PocketBase's auth-record/JWT system for guild codes** — they're opaque static tokens; use a custom middleware (the spike's criterion b).
- **Don't do the `.backup` from Go** — modernc doesn't expose the C online-backup API; shell out to the `sqlite3` CLI (D-11). Plain file-copy of a WAL-mode DB while it's being written is **unsafe**; `.backup`/`VACUUM INTO` are the safe hot-snapshot methods.
- **Don't `USER_ENTERED`-style coerce types** — N/A here (no Sheets), but keep `item_id`/`count` as INTEGER columns and parse explicitly; the parser already guarantees `r[2]` (ID) is an int.
- **Don't append or row-diff** — full-snapshot replace only (CLAUDE.md write contract, BACKEND-03).
- **Don't open port 443 with UFW on Oracle** — Oracle Ubuntu images use iptables and disable UFW; UFW changes silently fail to take effect (see Pitfall 2).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| TLS certs / renewal | Custom ACME client, manual certbot | **Caddy** (D-10) | Auto-HTTPS in 2 lines of Caddyfile; auto-renews. |
| DB schema versioning | A `_meta.schema_version`-style hand-rolled migrator | **goose** (D-10) | The v1 hand-rolled scheme existed *only* because Apps Script can't `ALTER TABLE`; a real DB + goose is the standard answer (finding 04 §3.1). |
| HTTP method/path routing | Regex route matcher | **`net/http` ServeMux 1.22+** | Native method+pattern routing + `PathValue`; D-02 forbids a router dep anyway. |
| SQLite hot backup | File-copy the .db while live | **`sqlite3 .backup` CLI** | File-copy of a WAL DB mid-write corrupts; `.backup` uses the online backup API for a consistent snapshot. |
| Object-storage upload | Hand-rolled OCI REST + request signing | **oci-cli (`oci os object put`) or rclone** | OCI request signing is fiddly; both tools support Instance Principal auth (no stored keys). |
| Constant-time compare | `==` on hashes, or `hmac.Equal` reinvented | **`crypto/subtle.ConstantTimeCompare`** | Timing-attack-safe; stdlib. |
| Cron scheduling | `time.Sleep` loops with manual next-run math | **`app.Cron()` (PB) / `robfig/cron` / a single `time.Ticker`** | P11 only needs a skeleton; don't build a scheduler. |
| Random token generation | `math/rand`, timestamps | **`crypto/rand`** (32 bytes → base64url) | Cryptographic entropy; the watcher's deleted `pkce.go` already used this shape. |

**Key insight:** Almost every "hard" part of this backend has a one-liner standard answer (Caddy for TLS, goose for migrations, ServeMux for routing, `sqlite3 .backup` for snapshots, `crypto/subtle` for compares). The actual bespoke code is small: the ingest handler, the atomic-replace tx, the bearer guard, and the first-sighting bind — a few hundred lines. This is why the fallback is genuinely viable and the spike is time-boxed to one day.

## Encoding Note (load-bearing — resolve in 11-02 planning)

The watcher's `internal/parse.Parse` / `ParseSpellbook` decode **Windows-1252** (`charmap.Windows1252.NewDecoder()`) because they read raw `.txt` files off the EQ folder. Under D-03/D-04 the watcher instead POSTs the file text inside a **JSON envelope** — and **JSON is UTF-8 by spec**. Three viable resolutions (a planning decision, not a locked one):

1. **Watcher decodes CP1252 → UTF-8 before POST (recommended).** The watcher reads the file, decodes CP1252 → Go `string` (UTF-8), and puts clean UTF-8 in `content`. The server parser then runs on already-UTF-8 text — so the **server parser drops the `charmap` decode step** and treats `content` as UTF-8. Keeps "server parses rows" (D-03) while putting the encoding knowledge where the bytes originate. This is a P13 watcher change, but P11 must define the contract assuming UTF-8 `content`.
2. **Watcher base64-encodes raw bytes; server decodes CP1252.** `content` carries base64 of the raw CP1252 bytes; the server base64-decodes then runs the existing `charmap` decoder unchanged. Maximally faithful to the original bytes, but fatter payload and the watcher must not "helpfully" re-encode.
3. **Watcher sends raw bytes as a latin1-safe string.** Fragile across JSON encoders; not recommended.

**For P11 planning:** define the ingest contract as **`content` = UTF-8 text** (resolution 1), and port the parser *minus* the CP1252 decode (i.e., feed `strings.NewReader(content)` straight into a `csv.Reader` with `Comma='\t'`, `FieldsPerRecord=-1`, `LazyQuotes=true`). Document that the watcher (P13) owns the CP1252→UTF-8 decode. Flag this in the 11-RESEARCH handoff so P13 doesn't double-decode. [ASSUMED: that UTF-8 `content` is the intended contract — CONTEXT says "raw file text" without specifying encoding; confirm in discuss/plan.]

## Migration SQL Sketch (D-13 → SQLite DDL)

A single forward-only `00001_init.sql` (goose format). Ports finding 04 §1 from Postgres → SQLite: `BIGINT GENERATED ALWAYS AS IDENTITY` → `INTEGER PRIMARY KEY` (SQLite rowid alias, auto-increments), `CITEXT` → `TEXT COLLATE NOCASE`, `timestamptz` → `TEXT` (ISO-8601 UTC) or `INTEGER` (unix), `gin_trgm` → omitted (FTS5/LIKE is P14's concern, not P11). All dimension tables created **empty**.

```sql
-- +goose Up
CREATE TABLE owner (
  id          INTEGER PRIMARY KEY,
  label       TEXT NOT NULL,
  created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE character (
  id            INTEGER PRIMARY KEY,
  owner_id      INTEGER NOT NULL REFERENCES owner(id),
  name          TEXT NOT NULL UNIQUE COLLATE NOCASE,  -- char name unique (D-13)
  class         TEXT,    -- nullable; set later / by backfill (P16)
  level         INTEGER,
  race          TEXT,
  is_bank_toon  INTEGER NOT NULL DEFAULT 0,
  is_hidden     INTEGER NOT NULL DEFAULT 0,
  is_removed    INTEGER NOT NULL DEFAULT 0,
  last_seen     TEXT,
  watcher_version TEXT,
  created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX character_owner_idx ON character(owner_id);

CREATE TABLE inventory_item (
  id           INTEGER PRIMARY KEY,
  character_id INTEGER NOT NULL REFERENCES character(id) ON DELETE CASCADE,
  location     TEXT NOT NULL,
  name         TEXT NOT NULL,
  item_id      INTEGER,            -- 0/NULL for empty slot
  count        INTEGER NOT NULL DEFAULT 1,
  slots        INTEGER,
  row_ordinal  INTEGER NOT NULL,   -- file line order; stable display sort
  uploaded_at  TEXT NOT NULL
);
CREATE INDEX inventory_char_idx ON inventory_item(character_id);
CREATE INDEX inventory_item_idx ON inventory_item(item_id);

CREATE TABLE spellbook_entry (
  id              INTEGER PRIMARY KEY,
  character_id    INTEGER NOT NULL REFERENCES character(id) ON DELETE CASCADE,
  level           INTEGER NOT NULL,
  name            TEXT NOT NULL,
  normalized_name TEXT NOT NULL,   -- lower(trim(name)) — P12/P14 join key to wiki_spells
  uploaded_at     TEXT NOT NULL
);
CREATE INDEX spellbook_char_idx ON spellbook_entry(character_id);
CREATE INDEX spellbook_norm_idx ON spellbook_entry(normalized_name);

CREATE TABLE guild_code (
  id          INTEGER PRIMARY KEY,
  owner_id    INTEGER NOT NULL REFERENCES owner(id),
  token_hash  BLOB NOT NULL UNIQUE,  -- sha256(plaintext); 32 bytes
  label       TEXT,
  disabled_at TEXT,                  -- NULL = active; non-NULL = revoked (D-09)
  created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- EMPTY dimension tables (P12 populates; P11 only creates).
CREATE TABLE item_master    (item_id INTEGER PRIMARY KEY, name TEXT, wiki_summary TEXT, wiki_url TEXT, slot TEXT, is_quest_item INTEGER NOT NULL DEFAULT 0, wikitext_sha1 TEXT, last_refreshed TEXT);
CREATE TABLE pigparse_price (item_id INTEGER PRIMARY KEY, name TEXT, current_avg REAL, blue_volume INTEGER, last_seen TEXT, direction TEXT, last_refreshed TEXT);
CREATE TABLE wiki_spells     (id INTEGER PRIMARY KEY, class TEXT NOT NULL, level INTEGER NOT NULL, spell_name TEXT NOT NULL, normalized_name TEXT NOT NULL, last_refreshed TEXT, UNIQUE(class, level, spell_name));
CREATE TABLE wiki_gear_tier  (id INTEGER PRIMARY KEY, tier TEXT NOT NULL, class TEXT NOT NULL, slot TEXT NOT NULL, item_id INTEGER, item_name TEXT, rank INTEGER, last_refreshed TEXT, UNIQUE(tier, class, slot, item_id));
CREATE TABLE quest_items     (id INTEGER PRIMARY KEY, item_id INTEGER NOT NULL, quest_name TEXT NOT NULL, source_url TEXT, source TEXT, last_refreshed TEXT, UNIQUE(item_id, quest_name));

-- +goose Down
DROP TABLE quest_items; DROP TABLE wiki_gear_tier; DROP TABLE wiki_spells;
DROP TABLE pigparse_price; DROP TABLE item_master; DROP TABLE guild_code;
DROP TABLE spellbook_entry; DROP TABLE inventory_item; DROP TABLE character; DROP TABLE owner;
```
Notes for the planner:
- `goose up` on a fresh DB applies this once; re-running is a no-op (goose tracks applied versions in its own `goose_db_version` table) — satisfies BACKEND-02's idempotency. [CITED: pressly.github.io/goose]
- `ON DELETE CASCADE` requires `foreign_keys(ON)` in the DSN (Pattern 5) — without it, SQLite silently ignores FK actions.
- `normalized_name` on `spellbook_entry` is computed at insert time (server-side `strings.ToLower(strings.TrimSpace(name))`), not a generated column, to keep the migration portable. P12/P14 need it; cheap to populate now.

## Common Pitfalls

### Pitfall 1: PocketBase pre-1.0 API churn
**What goes wrong:** PocketBase is v0.x. The v0.23 release (late 2024) was a near-total rewrite — removed the `Dao` abstraction (folded into `app`/`$app`), switched routing to Go 1.22 mux (`{param}` not `:param`), and refactored every hook to the `e.Next()` chain model. The official upgrade guide says it took users "from 1h to entire weekend." [CITED: pocketbase.io/v023upgrade/go]
**Why it happens:** Pre-1.0 software makes no stability promise; PocketBase explicitly warns of breaking changes between minor versions.
**How to avoid:** (1) Pin the exact version (`v0.39.0`) in `go.mod` and never float it. (2) All code examples in this doc are the *post-v0.23* API (current for v0.39) — use them, ignore older blog posts showing `Dao`/`:param`. (3) Factor the decision into the spike: adopting PB means signing up for periodic migration work on every upgrade. The hand-rolled fallback has no such churn (stdlib + two stable libs). **Warning sign:** any tutorial mentioning `app.Dao()`, `apis.ActivityLogger`, or `:id` route params is stale.

### Pitfall 2: Oracle's two-layer firewall (the multi-hour trap)
**What goes wrong:** You add a VCN Security List ingress rule for 443 and the site is *still* unreachable, OR you `ufw allow 443` and it has no effect. Hours lost.
**Why it happens:** Oracle Cloud has **two independent firewall layers** that BOTH must allow traffic: (1) the VCN **Security List / NSG** (cloud-side), and (2) the **OS host firewall**. Critically, Oracle's Ubuntu images use **iptables** (not UFW — UFW is *disabled and discouraged*), and the default `/etc/iptables/rules.v4` has a broad `REJECT` rule; a new ACCEPT rule appended *after* it is silently ignored due to rule precedence. [CITED: blogs.oracle.com "Enabling Network Traffic to Ubuntu Images in OCI"; syncbricks.com]
**How to avoid:** In the deploy runbook (BACKEND-01): (a) add VCN Security List ingress for TCP 80 and 443 from `0.0.0.0/0`; (b) insert iptables ACCEPT rules for 80/443 **before** the REJECT line in `/etc/iptables/rules.v4` (e.g. `iptables -I INPUT <n> -p tcp --dport 443 -j ACCEPT` then persist with `netfilter-persistent save`); (c) do NOT use UFW. **Warning sign:** `curl` from the box to `localhost:443` works but external `curl` times out → it's the firewall, not Caddy.

### Pitfall 3: goose dialect string vs. modernc driver name mismatch
**What goes wrong:** `goose.SetDialect("sqlite")` returns an "unknown dialect" error, OR `sql.Open("sqlite3", dsn)` fails because modernc registers as `"sqlite"`.
**Why it happens:** Two different name spaces. modernc.org/sqlite registers the `database/sql` **driver** under `"sqlite"`. goose's **dialect** for SQLite is `"sqlite3"`. They do not have to match and in this stack they deliberately don't. [VERIFIED: pkg.go.dev/github.com/pressly/goose/v3; modernc driver registration]
**How to avoid:** `sql.Open("sqlite", dsn)` (driver) **and** `goose.SetDialect("sqlite3")` (dialect). Encode both in Pattern 4/5 exactly. **Warning sign:** "unknown dialect sqlite" or "unknown driver sqlite3 (forgotten import?)".

### Pitfall 4: SQLite "database is locked" (SQLITE_BUSY) under concurrency
**What goes wrong:** Intermittent `SQLITE_BUSY` errors when an ingest write overlaps a read, or two writes race.
**Why it happens:** SQLite allows one writer at a time. The default deferred-transaction mode upgrades a read-tx to a write-tx mid-flight, which can deadlock against another writer and error immediately (no wait).
**How to avoid:** (1) `_pragma=busy_timeout(5000)` (wait, don't error). (2) `_txlock=immediate` (acquire the write lock up front via BEGIN IMMEDIATE). (3) For a single-writer server, `db.SetMaxOpenConns(1)` serializes all writes and eliminates BUSY entirely — at 50–150 writes/day this costs nothing. WAL mode (also in the DSN) lets readers proceed during a write. [CITED: berthub.eu, tenthousandmeters.com]
**Warning sign:** `database is locked` in logs under bursty uploads.

### Pitfall 5: Oracle Always Free idle-instance reclamation
**What goes wrong:** The A1 instance is reclaimed/stopped by Oracle after a quiet period — the backend vanishes.
**Why it happens:** Oracle deems Always Free compute "idle" and reclaimable if, over a 7-day window, **95th-percentile CPU < 20%** (some reports: CPU < 10% AND network < 10% AND memory < 10% for A1 shapes). A 12-person guild backend is *extremely* low-utilization. [CITED: docs.oracle.com FreeTier; lowendtalk]
**How to avoid:** (1) The nightly backup cron + (from P12) the daily/weekly enrichment jobs generate periodic activity — likely enough, but not guaranteed. (2) **Strongest mitigation:** upgrade the tenancy to **Pay-As-You-Go** while staying within Always Free resource limits — PAYG/"upgraded" accounts are **exempt from idle reclamation** and incur $0 as long as you stay in the free allotment. The user's own infra note already runs the Azure sub as PAYG; the same pattern fits Oracle. (3) A lightweight keep-alive cron (e.g. a tiny CPU spike) is the fallback if staying on a pure free account. **Flag this as a provisioning decision (D-12 owner action).** [ASSUMED: PAYG-exempt-from-reclamation is current Oracle policy — verify at provisioning; it has been stable but Oracle's free-tier terms shift.]
**Warning sign:** instance shows "Stopped" in the OCI console with a reclamation notice email.

### Pitfall 6: WAL file growth / backup completeness
**What goes wrong:** The `-wal` file grows unbounded, or a backup misses recent committed data still in the WAL.
**Why it happens:** WAL mode defers checkpointing; `.backup` handles the WAL correctly but the live `-wal`/`-shm` files must not be the only thing copied.
**How to avoid:** Use `sqlite3 db ".backup '/path/snap.db'"` (it produces a single consistent file including WAL contents — never copy the raw .db alone). Optionally `PRAGMA wal_checkpoint(TRUNCATE)` periodically for hygiene. [CITED: sqlite.org forum; sqlite.work]
**Warning sign:** restored DB missing the last few uploads → you copied the raw file instead of using `.backup`.

## Code Examples

### Cross-compile from Windows → linux/arm64 (both paths)
```powershell
# PowerShell on the Windows dev box
$env:GOOS="linux"; $env:GOARCH="arm64"; $env:CGO_ENABLED="0"
go build -trimpath -ldflags "-s -w" -o squirebot-server ./cmd/squirebot-server
# Produces a single static binary; scp to the A1 box. No C toolchain needed (modernc = pure Go).
```
[CITED: standard Go cross-compile; confirmed viable because the only SQLite dep is modernc (no cgo).]

### Minimal Caddyfile (D-10, BACKEND-01)
```
# /etc/caddy/Caddyfile
your.squirebot.domain {
    reverse_proxy localhost:8090
}
```
That is the entire config — Caddy provisions and auto-renews a Let's Encrypt cert for the domain and reverse-proxies to the Go server on loopback. [CITED: caddyserver.com/docs/quick-starts/reverse-proxy]

### systemd unit (D-10, BACKEND-01)
```ini
# /etc/systemd/system/squirebot-server.service
[Unit]
Description=SquireBot backend
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/squirebot-server serve --addr 127.0.0.1:8090 --db /var/lib/squirebot/squirebot.db
Restart=always
RestartSec=3
User=squirebot
AmbientCapabilities=          # no privileged ports needed — Caddy fronts 443
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
```
`sudo systemctl enable --now squirebot-server`. Caddy is its own `caddy.service` (installed via apt). [CITED: caddyserver.com/docs/running; freedesktop systemd]

### Nightly backup cron (D-11, BACKEND-06)
```bash
#!/usr/bin/env bash
# /usr/local/bin/squirebot-backup.sh  (chmod +x; run via cron 0 4 * * *)
set -euo pipefail
DB=/var/lib/squirebot/squirebot.db
STAMP=$(date -u +%F)
SNAP=/tmp/squirebot-$STAMP.db
sqlite3 "$DB" ".backup '$SNAP'"          # consistent hot snapshot via online backup API
gzip -f "$SNAP"
# Upload to Oracle Object Storage (Instance Principal auth — no stored keys):
oci os object put --auth instance_principal \
  -bn squirebot-backups --file "$SNAP.gz" --name "squirebot-$STAMP.db.gz" --force
rm -f "$SNAP.gz"
# Optional: prune local + keep last N in the bucket via lifecycle policy.
```
`crontab -e`: `0 4 * * * /usr/local/bin/squirebot-backup.sh >> /var/log/squirebot-backup.log 2>&1`. (rclone is the alternative to oci-cli; either supports Instance Principal on an Always Free box.) [CITED: sqlite.org backup; rclone.org/oracleobjectstorage; docs.oracle.com object put]

### Documented restore (D-11, BACKEND-06)
```bash
# On a clean box (runbook in docs/backend-deploy.md):
oci os object get --auth instance_principal -bn squirebot-backups \
  --name "squirebot-<DATE>.db.gz" --file /tmp/restore.db.gz
gunzip /tmp/restore.db.gz
sudo install -o squirebot -g squirebot /tmp/restore.db /var/lib/squirebot/squirebot.db
sudo systemctl restart squirebot-server   # goose.Up is a no-op on an already-migrated DB
# Verify: curl an authenticated read / query a row back out (ship gate).
```

### CLI mint subcommand (D-05) — shape
```go
// squirebot-server mint-code --owner "Bob"
func mintCode(db *sql.DB, ownerLabel string) error {
    raw := make([]byte, 32)
    if _, err := rand.Read(raw); err != nil { return err }          // crypto/rand
    code := base64.RawURLEncoding.EncodeToString(raw)               // the plaintext, shown ONCE
    sum := sha256.Sum256([]byte(code))
    ownerID := upsertOwner(db, ownerLabel)
    _, err := db.Exec(`INSERT INTO guild_code (owner_id, token_hash, label) VALUES (?,?,?)`,
        ownerID, sum[:], ownerLabel)
    if err != nil { return err }
    fmt.Printf("Guild code for %s (store now — not shown again):\n\n  %s\n\n", ownerLabel, code)
    return nil
}
```

## Validation Architecture

> `workflow.nyquist_validation` is **false** in `.planning/config.json`, so the Nyquist sampling-rate discipline is OFF. This section is included because the orchestrator keys off the heading and because the plan-vs-on-box test split is genuinely useful for this phase. Treat it as guidance, not a sampling mandate.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (+ `go test`) — matches the watcher's existing test style (`*_test.go`, table tests) |
| Config file | none (Go convention) |
| Quick run command | `go test ./internal/backendsrv/...` |
| Full suite command | `go test ./...` (whole module, incl. existing watcher tests) |

### What is unit-testable in CI (no live box, no network)
| Behavior | Test approach | Notes |
|----------|--------------|-------|
| Parser port (content → rows) | Reuse watcher's `internal/parse` table tests; add UTF-8-`content` cases | Pure functions; highest-value tests. Cover header detection, bad-ID skip, shrinking, apostrophes. |
| Atomic full-snapshot replace | Open an in-memory/temp-file modernc SQLite DB, ingest snapshot A, then shrunk snapshot B, assert only B's rows remain | modernc runs in CI on any OS (no cgo) — can even run on the Windows dev box. Use `file::memory:?cache=shared` or a temp file. |
| First-sighting bind + cross-owner reject | Seed two owners; ingest same char name under each; assert second is rejected + audit row written | Pure DB logic. |
| Bearer guard | Table test: missing / malformed / unknown / valid / disabled token → expected (ownerID, ok) | `crypto/subtle` + sha256; deterministic. |
| Token mint round-trip | mint → hash stored → presenting the printed plaintext authenticates | Verifies the hash-only-storage contract. |
| goose migrations apply + idempotent | `goose.Up` on a fresh temp DB; run twice; assert second is a no-op and all D-13 tables exist | Confirms BACKEND-02. |
| Envelope decode + validation | Bad JSON, missing fields, unknown `kind` → 4xx | `httptest.NewRequest` against the handler. |

### What requires the live Oracle box (on-box manual smoke, NOT CI)
| Behavior | Why not CI | How to verify |
|----------|-----------|---------------|
| TLS reachable at the domain (BACKEND-01) | Needs DNS + Let's Encrypt + the box | `curl https://your.domain/...` from outside; check cert. |
| systemd Restart=always + restart-on-reboot | Needs the box | `sudo systemctl restart`; reboot; confirm it comes back. |
| Two-layer firewall correctly open | Oracle-specific | external `curl` succeeds where loopback already did. |
| Nightly backup cron + restore (BACKEND-06) | Needs cron + Object Storage creds | run the script manually; confirm object lands; do a full restore on a scratch box. |
| arm64 binary actually runs (spike crit. d) | Needs ARM hardware | run on the A1 box; hit the health/ingest route. |

### Phase ship gate (from ROADMAP)
> "server accepts a real test upload over TLS and the row is queryable back out." This is an **on-box** check: mint a code, `curl -H "Authorization: Bearer <code>" -d '{…}' https://your.domain/api/v1/ingest`, then query the row out (a temporary debug query or `sqlite3` on the box, since the read API is P14).

### Wave 0 Gaps
- [ ] `internal/backendsrv/.../*_test.go` — no backend test files exist yet (green-field). Create alongside each package.
- [ ] A temp-DB test helper (open modernc with the WAL/foreign_keys DSN, run goose.Up) — shared fixture for store/ingest tests.
- [ ] No framework install needed (Go stdlib `testing`).

## Security Domain

> `security_enforcement: true`, `security_asvs_level: 1`, `security_block_on: high` in config. This phase introduces the project's first network-exposed authenticated write endpoint, so security is squarely in scope.

### Applicable ASVS Categories (Level 1)
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | **yes** | Opaque high-entropy bearer token (32 bytes, `crypto/rand`); stored only as SHA-256 hash; `subtle.ConstantTimeCompare`; missing/malformed/unknown/revoked → 401 writing nothing (D-08/D-09). |
| V3 Session Management | partial | Stateless bearer auth (no sessions). Token = long-lived credential; revocation via `disabled_at` (D-09). No cookies in P11. |
| V4 Access Control | **yes** | First-sighting owner binding; cross-owner upload rejected (D-07). Each token maps 1:1 to an owner; a token can only write its own (or newly-bound) characters. |
| V5 Input Validation | **yes** | Validate JSON envelope (required fields, `kind` enum); parser tolerates/skips malformed rows (existing behavior); parameterized SQL only (`?` placeholders — never string-concat). Cap `content` size (e.g. reject >1 MB; a maxed char is <50 KB per finding 01). |
| V6 Cryptography | **yes** | SHA-256 (stdlib `crypto/sha256`); `crypto/rand` for token gen; **never hand-roll**. TLS provided by Caddy (modern defaults). At-rest: the SQLite file + backups contain guild data — encrypt backups (finding 04 §5.3) or rely on Object Storage server-side encryption (OCI encrypts at rest by default). |
| V7 Errors/Logging | partial | Structured logging (`slog`) per CLAUDE.md; audit row on cross-owner reject (D-07); **never log the raw token or raw `content`** (mirrors the watcher's T-04-07 "never log raw content" rule). |

### Known Threat Patterns for a Go + SQLite ingest API
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| SQL injection via character name / content | Tampering | Parameterized queries (`?` placeholders) everywhere; never build SQL by concatenation. modernc/database/sql enforce this when used correctly. |
| Token brute-force / guessing | Spoofing | 32-byte (256-bit) random tokens are infeasible to brute-force; 401 reveals nothing; consider rate-limiting at Caddy if paranoid (optional at 12 users). |
| Timing attack on token compare | Information Disclosure | `subtle.ConstantTimeCompare` (D-08). |
| Token leakage in logs/backups | Information Disclosure | Store only the hash; never log plaintext; the printed plaintext exists only at mint time. |
| Oversized / malicious payload (DoS) | Denial of Service | `http.MaxBytesReader` on the body; reject `content` over a sane cap; `busy_timeout` bounds DB waits. |
| Cross-guildie data overwrite | Tampering / Elevation | First-sighting bind + cross-owner reject (D-07); the backend (not racing watchers) owns writes. |
| Unauthenticated reads | Information Disclosure | N/A in P11 (no read API until P14); the only route is the authenticated ingest. Ensure no debug/health route leaks data. |
| Backup at rest exposure | Information Disclosure | OCI Object Storage encrypts at rest by default; restrict bucket to the tenancy; consider client-side gzip+encrypt if stricter (finding 04 §5.3). |

## State of the Art

| Old Approach (v1 / Apps Script) | Current Approach (v2 backend) | When Changed | Impact |
|---------------------------------|-------------------------------|--------------|--------|
| `_meta.schema_version` ↔ `WatcherMaxSchemaVersion` handshake | goose forward-only migrations + `/api/v1` API version | This milestone | Kills a recurring incident class (finding 04 §3); watcher no longer knows table shapes. |
| `batchUpdate` clear+write (atomic-ish) to Sheets | Real `BEGIN/COMMIT` SQLite transaction | This milestone | True atomicity; no reader-visible intermediate state. |
| OAuth `userinfo.email` → `_char_owner` identity | Bearer token → owner; first-sighting binds char | This milestone | No Google; no racing-watcher owner conflicts. |
| Apps Script 6-min cap → resumable wiki cursor | Long-running in-process job (P12) | This milestone | Deletes the cursor hack (P12 concern; P11 only builds the scheduler skeleton). |
| Mojibake-prone CP1252 read on the watcher's disk | (resolve) CP1252→UTF-8 decode on watcher; UTF-8 `content` on the wire | This phase (contract) | See Encoding Note. |

**Deprecated/outdated to ignore while researching PocketBase:**
- Any PocketBase tutorial using `app.Dao()`, `daos.New()`, `:param` routes, or `apis.ActivityLogger()` — all pre-v0.23, removed. The current API is `app.<method>()`, `{param}`, `app.OnServe().BindFunc`.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The ingest `content` is intended to be **UTF-8 text** (watcher pre-decodes CP1252), not base64 raw bytes. | Encoding Note | If raw CP1252 bytes are intended instead, the server parser must keep the `charmap` decode and the watcher must base64. Medium — affects the P11 contract def and the P13 watcher. **Confirm in discuss/plan.** |
| A2 | Oracle PAYG/upgraded accounts are **exempt from idle-instance reclamation** (current policy). | Pitfall 5 | If wrong, the box could be reclaimed during quiet weeks; mitigation falls back to a keep-alive cron. Low-medium — verify at provisioning. |
| A3 | "Go 1.24" in CONTEXT is superseded by the repo's actual `go 1.25.0` / installed 1.26.2 — use ≥1.25. | Standard Stack | None material (1.22+ is all that's required for ServeMux routing); just don't pin 1.24 in CI if the repo is 1.25. Low. |
| A4 | An in-memory/temp-file modernc SQLite DB runs in CI on the dev box for the atomic-replace/migration tests. | Validation Architecture | If CI is constrained, use a temp file instead of `:memory:`. Low — modernc is pure Go and runs anywhere. |
| A5 | Defining the dimension tables as **plain SQL tables** (not PB collections) is acceptable in the spike path. | Spike Criterion (a) | If PB's admin UI / auto-REST is wanted over those tables later (P15), they'd need to be collections. Low — a P11 plain-table choice can be revisited; doesn't block P11. |

## Open Questions

1. **Encoding contract (A1).**
   - What we know: D-03/D-04 say "raw file text" in a JSON envelope; JSON is UTF-8.
   - What's unclear: whether the watcher pre-decodes CP1252→UTF-8 (recommended) or ships raw bytes base64.
   - Recommendation: lock "UTF-8 `content`" in 11-02 planning; port the parser minus the CP1252 decoder; flag for P13.

2. **Oracle account type (A2 / D-12).**
   - What we know: pure Always Free risks idle reclamation; PAYG-within-free-limits is the robust escape.
   - What's unclear: which the user will provision.
   - Recommendation: surface as a provisioning decision in the BACKEND-01 deploy runbook; default-recommend PAYG-within-free-limits (user already runs Azure as PAYG).

3. **Spike outcome shapes downstream plans.**
   - What we know: PB passes technically; the decision is leverage-vs-opinion.
   - What's unclear: the verdict (by design — that's the spike).
   - Recommendation: write 11-02+ to branch on the verdict; keep the shared-infra plans (Oracle/Caddy/systemd/backup) verdict-independent so they proceed regardless.

4. **`character` metadata population.**
   - What we know: D-13 says class/level/race/flags are nullable, "populated later/by backfill."
   - What's unclear: nothing for P11 — they stay NULL until P15 forms / P16 backfill.
   - Recommendation: create columns nullable now; no population logic in P11.

## Environment Availability

> The dev-time build runs on the Windows box (cross-compile). The runtime deps live on the Oracle box (user-provisioned per D-12), so most are "✗ here / required there."

| Dependency | Required By | Available (dev box) | Version | Fallback |
|------------|------------|---------------------|---------|----------|
| Go toolchain | build (cross-compile) | ✓ | go1.26.2 | — |
| `modernc.org/sqlite` | DB driver | ✓ (go get) | v1.51.0 | — |
| `goose/v3` | migrations | ✓ (go get) | v3.27.1 | — |
| `pocketbase` | spike only | ✓ (go get) | v0.39.0 | hand-rolled fallback |
| Oracle A1 instance | runtime host (BACKEND-01) | ✗ (user provisions) | — | none — required (D-12) |
| Caddy | TLS/proxy (BACKEND-01) | ✗ (install on box) | v2.x apt | none — required (D-10) |
| `sqlite3` CLI | backup `.backup` (BACKEND-06) | ✗ (install on box) | 3.x | none — required (D-11; modernc can't do `.backup`) |
| oci-cli or rclone | backup upload (BACKEND-06) | ✗ (install on box) | current | either works |
| A registered domain | TLS hostname (BACKEND-01) | ✗ (user registers ~$12/yr) | — | none — Caddy needs a real hostname for Let's Encrypt; `.duckdns.org` is a free fallback if needed |
| Discord (DM channel) | code distribution (D-06) | n/a (manual) | — | — |

**Missing dependencies with no fallback (block runtime, not the build):**
- Oracle A1 instance, Caddy, `sqlite3` CLI, a domain. All are user-provisioning / on-box-install steps the BACKEND-01 plan must enumerate. None block the **build/test** plans (which run on the dev box with modernc in CI).

**Missing dependencies with fallback:**
- Object-storage upload tool: oci-cli OR rclone (either).
- Domain: a paid ~$12/yr domain OR a free `*.duckdns.org` subdomain (Caddy auto-HTTPS works with both).

## Sources

### Primary (HIGH confidence)
- **Go module proxy** (`go list -m -versions`) — verified latest: `modernc.org/sqlite v1.51.0`, `pressly/goose/v3 v3.27.1`, `pocketbase v0.39.0`; `go version` → go1.26.2; repo `go.mod` → `go 1.25.0`.
- **PocketBase `go.mod`** (github.com/pocketbase/pocketbase/master/go.mod) — confirmed `modernc.org/sqlite v1.51.0` (no `mattn/go-sqlite3`, no cgo); declares `go 1.25.0`.
- **Context7 `/websites/pocketbase_io`** — `app.OnServe().BindFunc`, `e.Router.GET/POST(...).Bind(apis.RequireAuth/RequireSuperuserAuth)`, custom middleware via `se.Router.BindFunc`, `app.RunInTransaction(fn)` + `txApp`, raw SQL via `app.DB().NewQuery(...).Execute()`, `core.NewRecord`+`app.Save`, `app.Cron().MustAdd`, auth-token-loader middleware reads `Authorization` header.
- **Context7 `/pressly/goose`** — `//go:embed migrations/*.sql` + `SetBaseFS` + `SetDialect` + `Up(db, "migrations")`; programmatic API.
- **Context7 `/websites/pkg_go_dev_modernc_org_sqlite`** — driver name `"sqlite"`; DSN `_pragma=`, `_txlock=` params; `sql.Open("sqlite", dsnURI)`.
- **pkg.go.dev/github.com/pressly/goose/v3** — dialect list; SQLite dialect = `"sqlite3"` (distinct from driver name).
- `internal/parse/inventory.go`, `internal/parse/spellbook.go`, `internal/sheet/write.go` (repo) — parser contracts + v1 full-snapshot write contract being ported.
- `.planning/explorations/website-milestone/{01,03,04}.md` — backend/auth/data-model research (read "Hetzner/Postgres" as "Oracle/SQLite" per ROADMAP override).

### Secondary (MEDIUM confidence — verified against official docs where possible)
- go.dev/blog/routing-enhancements — Go 1.22 ServeMux method+pattern routing, `PathValue`, 405+Allow.
- caddyserver.com/docs (quick-starts/reverse-proxy, running, install) — minimal Caddyfile, `caddy.service`, apt install.
- docs.oracle.com FreeTier + blogs.oracle.com "Enabling Network Traffic to Ubuntu Images in OCI" — two-layer firewall, iptables-not-UFW, idle reclamation thresholds.
- sqlite.org (pragma.html, backup forum threads) + sqlite.work — `.backup` vs `VACUUM INTO`, WAL hot-backup safety.
- pocketbase.io/v023upgrade/go — the v0.23 rewrite scope (API-churn pitfall).
- rclone.org/oracleobjectstorage + docs.oracle.com object put — Object Storage upload via Instance Principal.
- berthub.eu / tenthousandmeters.com — SQLITE_BUSY mitigation (busy_timeout, BEGIN IMMEDIATE, single conn).

### Tertiary (LOW confidence — flagged for validation)
- WebSearch summaries of modernc DSN pragma string syntax (`_pragma=journal_mode(WAL)&…`) — cross-checked against the Context7 modernc page (which confirms `_pragma`/`_txlock` exist) but the exact `(VALUE)` parenthesized form should be smoke-tested in the spike/11-02.
- Oracle "PAYG exempt from reclamation" (A2) — widely reported, stable, but Oracle free-tier terms shift; verify at provisioning.

## Metadata

**Confidence breakdown:**
- Standard stack + versions: HIGH — all versions verified against the live module proxy and PocketBase's own go.mod.
- PocketBase-as-framework feasibility (the gating question): HIGH — the cgo concern is resolved (shared modernc driver), and every spike-criterion API is documented in current PocketBase docs. The *decision* (adopt vs. fallback) is judgment, not a fact gap.
- Atomic ingest transaction + parser port: HIGH — derived directly from the repo's v1 contract and confirmed SQLite tx semantics; one MEDIUM item (encoding contract, A1).
- Bearer auth + crypto: HIGH — stdlib primitives, well-trodden patterns.
- Ops (Caddy/systemd/Oracle/backup): MEDIUM — patterns are standard and cited, but Oracle Always Free specifics (firewall layering, reclamation, account type) need on-box validation at provisioning.

**Research date:** 2026-05-28
**Valid until:** ~2026-06-27 for the stack versions (PocketBase is fast-moving and pre-1.0 — re-verify its version + API at spike time if more than ~2 weeks elapse); ~2026-07-28 for the stable libs (modernc/goose/Caddy/SQLite patterns).

## RESEARCH COMPLETE

**Phase:** 11 - Backend Foundation + Ingest API
**Confidence:** HIGH (stack/versions/PocketBase-cgo question, ingest/auth patterns); MEDIUM (Oracle Always Free ops specifics — validate on provisioning)

### Key Findings
- **PocketBase v0.39.0 uses `modernc.org/sqlite v1.51.0` (pure Go, no cgo) and declares `go 1.25.0`** — the spike's biggest perceived risk (cgo vs. cross-compile-from-Windows ethos) is a non-issue; PocketBase ships the *same driver* the fallback would. All four spike criteria are technically achievable with the current (post-v0.23) API (`OnServe().BindFunc` + `e.Router.POST(...).Bind` custom guard, `RunInTransaction`, `app.Cron().MustAdd`). The real decision is leverage (admin UI / auto-REST / built-in Discord OAuth2 for P15) vs. pre-1.0 API-churn friction.
- **Verified current versions:** modernc.org/sqlite v1.51.0 · pressly/goose/v3 v3.27.1 · pocketbase v0.39.0 · Go toolchain 1.26.2 (repo go.mod 1.25.0; CONTEXT's "Go 1.24" is stale — flagged A3).
- **Two foot-guns the planner must encode exactly:** (1) `sql.Open("sqlite", …)` driver name vs. `goose.SetDialect("sqlite3")` dialect string — they differ on purpose; (2) Oracle's two-layer firewall + iptables-not-UFW + insert-before-REJECT — a multi-hour trap.
- **Encoding contract is the one genuine open question (A1):** the watcher decodes CP1252 off disk, but the wire payload is UTF-8 JSON. Recommend the watcher pre-decode CP1252→UTF-8 and the server parser drop the `charmap` step; confirm in 11-02 planning, flag for P13.
- **Oracle idle-reclamation is real** (95th-pct CPU <20% over 7 days); strongest mitigation is a PAYG-within-free-limits tenancy (exempt) — surface as a D-12 provisioning decision.

### File Created
`.planning/phases/11-backend-foundation-ingest-api/11-RESEARCH.md`

### Confidence Assessment
| Area | Level | Reason |
|------|-------|--------|
| Standard Stack | HIGH | Versions verified against live module proxy + PocketBase go.mod |
| Architecture (ingest tx, auth, schema) | HIGH | Derived from repo's v1 contract + confirmed SQLite/crypto semantics |
| PocketBase spike feasibility | HIGH | cgo question resolved; all criteria APIs documented current |
| Ops (Caddy/systemd/Oracle/backup) | MEDIUM | Standard patterns cited; Oracle specifics need on-box validation |
| Encoding contract | MEDIUM | One open question (A1) — UTF-8 vs base64; recommended path given |

### Open Questions (carry to discuss/plan)
1. Encoding contract: UTF-8 `content` (recommended) vs base64 raw CP1252 (A1) — affects P11 contract + P13 watcher.
2. Oracle account type: pure Always Free (reclaim risk) vs PAYG-within-free-limits (recommended) (A2 / D-12).
3. Spike verdict branches 11-02+; keep shared-infra plans verdict-independent.

### Ready for Planning
Research complete. The planner can structure 11-01 as the spike (with the four concrete PASS/FAIL probes above), then branch 11-02+ on the verdict while proceeding independently on the verdict-agnostic infra (Oracle provisioning + firewall, Caddy, systemd, goose schema, nightly backup). All five requirements (BACKEND-01/02/03/04/06) have concrete, cited implementation support.
