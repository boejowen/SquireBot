---
phase: 11-backend-foundation-ingest-api
artifact: security-verification
state: B   # no prior SECURITY.md; built from PLAN threat models + SUMMARYs + implementation
asvs_level: 1
block_on: high
threats_found: 39
threats_closed: 39
threats_open: 0
unregistered_flags: 0
verified: 2026-05-29
verifier: gsd-secure-phase (verification-only; implementation files READ-ONLY)
authoritative_runtime_evidence: docs/backend-deploy.md §6 (live deploy 2026-05-29, Hetzner 5.78.232.85)
---

# Phase 11 — Security Verification (Backend Foundation + Ingest API)

**Verdict: SECURED.** All 39 declared threats across the 7 plan threat registers resolve to
CLOSED — 37 `mitigate` dispositions verified by a control present in source/deploy artifacts,
and 2 `accept` dispositions recorded in the accepted-risks log below. No threat is OPEN; no
new unregistered attack surface appeared during implementation.

Verification method per disposition: `mitigate` → grep the declared control in the file(s)
named in the mitigation plan, confirming it applies to the real entry point (not just that a
similar-looking line exists); `accept` → entry present in the accepted-risks log; runtime-only
state (TLS / firewall / file perms / R2) → the captured live evidence in
`docs/backend-deploy.md` §6, which the deploy plan designates authoritative for on-box state
that cannot be grepped from source.

Self-reported `## Threat Model Compliance` sections in the plan SUMMARYs were NOT treated as
evidence; every control below was re-confirmed against the implementation tree.

---

## Per-Threat Register

### 11-01 — PocketBase spike (throwaway; verify removed + carried no production risk)

| Threat ID | Category | Disp. | Status | Evidence |
|-----------|----------|-------|--------|----------|
| T-11.01-01 | Information Disclosure | mitigate | CLOSED | Spike used a hardcoded TEST token hash + synthetic rows only (11-01-SUMMARY probe b: `Bearer spike-test-token`); the entire `spike/` tree is now DELETED (Glob `spike/**` → no files), removed in 11-05 (`e45f2c5`). No real `crypto/rand` code or real character data was ever in the spike, and it no longer exists in the tree. |
| T-11.01-02 | Tampering | mitigate | CLOSED | The pre-1.0 PocketBase dep is fully removed under the FALLBACK verdict: `go.mod` grep `pocketbase` → no matches; `go.sum` → 0 occurrences. There is no floated pre-1.0 dependency left to drift. |
| T-11.01-03 | Elevation | accept→CLOSED | CLOSED | The spike's custom guard used `crypto/subtle.ConstantTimeCompare`, NOT `apis.RequireAuth()` (11-01-SUMMARY probe b). The production guard (`internal/backendsrv/auth/guard.go`) is independently custom and imports zero PocketBase (grep `pocketbase`/`RequireAuth`/`apis.` across `internal/backendsrv` → only doc comments + one test negative-assertion string). No PB permission semantics leaked into the opaque-token design. Accepted-risk logged. |

### 11-02 — DB / migrations

| Threat ID | Category | Disp. | Status | Evidence |
|-----------|----------|-------|--------|----------|
| T-11.02-01 | Tampering | mitigate | CLOSED | `foreign_keys(ON)` is in the per-connection DSN — `internal/backendsrv/store/db.go:43`. The pragma is not persistent; placing it in the DSN means every pooled conn enforces FK actions. Schema confirms `ON DELETE CASCADE` on inventory_item/spellbook_entry (`00001_init.sql:26,40`). |
| T-11.02-02 | Denial of Service | mitigate | CLOSED | `busy_timeout(5000)` + `_txlock=immediate` in the DSN (`db.go:42,45`) + `SetMaxOpenConns(1)` (`db.go:61`) — single-writer serialization eliminating SQLITE_BUSY. |
| T-11.02-03 | Tampering | mitigate | CLOSED | goose tracks applied versions in `goose_db_version`; `migrations/embed.go:33-39` `RunMigrations` is idempotent (SetBaseFS + SetDialect("sqlite3") + goose.Up). A partial run replays cleanly. |
| T-11.02-04 | Information Disclosure | accept→CLOSED | CLOSED | DSN carries no secret in P11 (file path + pragmas only); `db.go:54-55` documents the connection string is intentionally not logged. Accepted-risk logged. |

### 11-03 — parser + atomic replace + first-sighting bind / cross-owner audit

| Threat ID | Category | Disp. | Status | Evidence |
|-----------|----------|-------|--------|----------|
| T-11.03-01 | Tampering (SQLi) | mitigate | CLOSED | All SQL in `store/binding.go` (lines 58, 62, 91-92) and `store/replace.go` (lines 91, 96-98, 116, 152, 157-159, 176) uses `?` placeholders only; no string-concatenated SQL. |
| T-11.03-02 | Tampering | mitigate | CLOSED | `bindCharacter` rejects a cross-owner write with `ErrCharOwnedByAnother` (`binding.go:73-80`); `owner_id` is never overwritten on mismatch (the mismatch branch INSERTs nothing into character). |
| T-11.03-03 | Tampering | mitigate | CLOSED | One `BeginTx` (`replace.go:64,131`) + `defer tx.Rollback()` (`replace.go:68,135`) + single `Commit`; on an INSERT error the DELETE rolls back — no partial state visible. |
| T-11.03-04 | Information Disclosure | mitigate | CLOSED | Write-path `slog` calls log `char_id` + `row_ordinal` + `err` only (`replace.go:92,111,118,153,171,178`); raw `content` is never logged (V7). |
| T-11.03-05 | Repudiation | mitigate | CLOSED | `auditCrossOwnerReject` appends a row to `audit_log` before the reject returns (`binding.go:89-96`); the `audit_log` table ships in `migrations/00002_audit.sql` (`CREATE TABLE audit_log`, both goose markers). The ingest handler COMMITS the tx on the cross-owner branch so the audit row is durable (`ingest/handler.go:183-191`). |
| T-11.03-06 | Tampering (encoding) | mitigate | CLOSED | `charmap` decode removed from `Parse`/`ParseSpellbook` bodies (`inventory.go:53-81`, `spellbook.go:47-76` — no `NewDecoder().Reader`); single `CP1252Reader` helper at `inventory.go:35-37`. Both live v1 watcher call sites wrapped: `internal/app/runapp.go:463` `parse.Parse(parse.CP1252Reader(f))` and `:548` `parse.ParseSpellbook(parse.CP1252Reader(f))`. Grep across non-test `internal/` confirms the ONLY production callers are runapp.go (CP1252-wrapped) and the UTF-8 ingest handler (correctly unwrapped) — no bare raw-CP1252 caller remains. |

### 11-04 — bearer auth (crypto/rand token, SHA-256 hash-at-rest, constant-time compare)

| Threat ID | Category | Disp. | Status | Evidence |
|-----------|----------|-------|--------|----------|
| T-11.04-01 | Spoofing | mitigate | CLOSED | 32-byte token: `raw := make([]byte, 32); rand.Read(raw)` (`auth/mint.go:31-32`); a non-match returns `(0,false)` revealing nothing (`guard.go:101`). |
| T-11.04-02 | Information Disclosure | mitigate | CLOSED | `subtle.ConstantTimeCompare(sum[:], stored) == 1` on the hash bytes (`auth/guard.go:91`). |
| T-11.04-03 | Information Disclosure | mitigate | CLOSED | Only `sha256(plaintext)` (`sum[:]`) is persisted (`mint.go:36,46-47`); plaintext crosses to the maintainer once via `fmt.Printf` to stdout (`mint.go:52`), never `slog`; auth-reject `slog` records carry no token material (`guard.go:71,80,96,100`). `guild_code.token_hash BLOB NOT NULL UNIQUE` (`00001_init.sql:52`). |
| T-11.04-04 | Elevation | mitigate | CLOSED | `WHERE disabled_at IS NULL` excludes revoked rows (`guard.go:78`); `RevokeCode` sets `disabled_at` (`auth/store.go:64-71`); schema `disabled_at` column (`00001_init.sql:54`). |
| T-11.04-05 | Tampering (SQLi) | mitigate | CLOSED | The presented token is `sha256`-hashed before any DB use (`guard.go:75`); the active-rows query interpolates no user SQL; mint/revoke/upsert use `?` placeholders only. |
| T-11.04-06 | Spoofing | mitigate | CLOSED | `crypto/rand` only (`mint.go:4,32`); no `math/rand` import anywhere in `internal/backendsrv` (grep → only "NOT math/rand" doc comments). |

### 11-05 — ingest HTTP handler (guard-first ordering, MaxBytesReader, 4xx mapping, one-tx)

| Threat ID | Category | Disp. | Status | Evidence |
|-----------|----------|-------|--------|----------|
| T-11.05-01 | Spoofing / Elevation | mitigate | CLOSED | `ResolveToken` is called FIRST (`ingest/handler.go:85`); `!ok` → 401 and `return` at `:86-90`, before the first `store.` call (`bindAndReplace` at `:116`). 401 writes nothing by construction. |
| T-11.05-02 | Denial of Service | mitigate | CLOSED | `http.MaxBytesReader(w, r.Body, maxBodyBytes)` at `handler.go:79` BEFORE decode; `maxBodyBytes = 1 << 20` (`:50`). |
| T-11.05-03 | Tampering | mitigate | CLOSED | `DecodeAndValidate` enforces non-empty `Character` + kind enum (`ingest/envelope.go:88-93`); typed sentinels mapped to 4xx (`handler.go:96-100,224-239`) before any store call. |
| T-11.05-04 | Tampering / Elevation | mitigate | CLOSED | `store.ErrCharOwnedByAnother` → 409 (`handler.go:118-124`); the cross-owner reject is audited in `BindCharacter` and the original owner's rows are untouched (no replace runs on that branch). |
| T-11.05-05 | Information Disclosure | mitigate | CLOSED | Handler `slog` logs `reason`/`char`/`kind`/`status`/`err` only (`handler.go:87,109,122,128,133`); never the raw `content` or the `Authorization` header. |
| T-11.05-06 | Information Disclosure | mitigate | CLOSED | The only registered route is `mux.Handle("POST /api/v1/ingest", …)` (`cmd/squirebot-server/main.go:183`) — no unauthenticated health/debug route returning data; server binds loopback `127.0.0.1:8090` (`main.go:42,149`). |
| T-11.05-07 | Tampering | mitigate | CLOSED | No Google/OAuth/Sheets dependency: `main.go` import block is stdlib + internal only (`:21-39`); the only google/oauth/sheets grep hits are the "Off Google" doc comment (`:16-17`); no `-X main.OAuthClientID` ldflag. (11-05-SUMMARY: `go list -deps ./cmd/squirebot-server` → zero google/oauth2/sheets/pocketbase.) |
| T-11.05-08 | Tampering | mitigate | CLOSED | Handler authors NO inline bind/replace SQL — it calls 11-03's exported Tx functions over ONE `db.BeginTx` (`handler.go:168,183,204,206`); the public `Store.Replace*` delegate to the same `*Tx` bodies (`replace.go:70,137`), so 11-03's store tests are the single coverage path. |

### 11-06 — deploy (TLS, least-priv, restart, firewall, data-dir perms) — LIVE infra

| Threat ID | Category | Disp. | Status | Evidence |
|-----------|----------|-------|--------|----------|
| T-11.06-01 | Information Disclosure / Tampering | mitigate | CLOSED | `deploy/Caddyfile` terminates TLS + reverse-proxies loopback. §6 live: `curl https://api.squirebot.quest/` → valid Let's Encrypt cert (`TLS_VERIFY=0`), Caddy v2.11.3. HTTP→HTTPS is Caddy default. |
| T-11.06-02 | Elevation | mitigate | CLOSED | `deploy/squirebot-server.service:10-11` `User=squirebot` + `NoNewPrivileges=true`; ExecStart binds loopback `127.0.0.1:8090` (`:7`) — no privileged port. |
| T-11.06-03 | Denial of Service | mitigate | CLOSED | Host is a paid always-on Hetzner VPS (no Oracle-style idle reclamation); §6 confirms the live box at `5.78.232.85` (Hillsboro OR). Reclamation risk eliminated by host choice. |
| T-11.06-04 | Denial of Service | mitigate | CLOSED | `Restart=always` + `RestartSec=3` + `WantedBy=multi-user.target` (`squirebot-server.service:8-9,14`). §6 live: reboot survival confirmed — service `active`/`enabled` after `systemctl reboot`, ship-gate row still present. |
| T-11.06-05 | Spoofing / Tampering | mitigate | CLOSED | §6 live: `ufw status` active; `22/tcp`, `80/tcp`, `443/tcp` = ALLOW (v4+v6); port `8090` NOT opened (loopback-only). Runbook §2.2 allows SSH before `ufw enable` (no lockout). |
| T-11.06-06 | Information Disclosure | mitigate | CLOSED | Runbook §2.1 `chown squirebot:squirebot /var/lib/squirebot`; binary runs `User=squirebot`; §6 confirms `sudo -u squirebot sqlite3 …` is the access path (DB owned by the service user, not world-readable). |

### 11-07 — backup (R2 at-rest, scoped token mode 600, .backup-not-cp, unauth-write blocked) — LIVE infra

| Threat ID | Category | Disp. | Status | Evidence |
|-----------|----------|-------|--------|----------|
| T-11.07-01 | Information Disclosure | mitigate | CLOSED | Private R2 bucket (no public access — runbook §3.1); R2 encrypts at rest by default. §6 live: `squirebot-2026-05-29.db.gz` present in the private `squirebot-backups` bucket. |
| T-11.07-02 | Spoofing / Information Disclosure | mitigate | CLOSED | §6 live: `rclone.conf` at `/root/.config/rclone/rclone.conf`, mode `600`; bucket-scoped Object Read & Write token (runbook §3.1), revocable in the Cloudflare dashboard. |
| T-11.07-03 | Tampering / Data loss | mitigate | CLOSED | `deploy/squirebot-backup.sh:18` `sqlite3 "$DB" ".backup '$SNAP'"` (online backup API — WAL-consistent), never a raw `cp` of the live `.db`. §6 live: restore drill pulled the snapshot → all 12 D-13 tables + rows present (reconstitution proven). |
| T-11.07-04 | Repudiation / Data loss | accept→CLOSED | CLOSED | Cron logs to `/var/log/squirebot-backup.log` (`backup.sh:13`, §3.2); automated freshness alerting deferred (Litestream/monitoring is a future upgrade). Accepted-risk logged. |
| T-11.07-05 | Spoofing / Elevation | mitigate | CLOSED | §6 live ship-gate: unauthenticated `POST /api/v1/ingest` over TLS → **401** (wrote nothing); authenticated → **204**; queried row back `ShipGateChar|Rusty Dagger|12345`. BACKEND-04 / V2 validated end-to-end over the wire, not just in unit tests. |

---

## Accepted-Risks Log

Threats whose declared disposition is `accept` (or accept-by-design). Each is recorded here as
required for an `accept` disposition to count CLOSED. None has HIGH realistic severity; under
`block_on: high` none blocks phase advancement.

| Threat ID | Risk | Why accepted | Residual severity |
|-----------|------|--------------|-------------------|
| T-11.01-03 | Spike's custom guard could have modeled PB's JWT auth-record semantics | The spike was throwaway and is now deleted; the production guard is independently custom `crypto/subtle` with zero PocketBase imports (verified). No semantics leaked. | Negligible (spike removed) |
| T-11.02-04 | DSN / DB path logged with a future secret | The DSN holds no secret in P11 (file path + pragmas); `db.go` documents the standing habit of not logging connection strings. Revisit if a secret ever enters the DSN. | Low |
| T-11.07-04 | Backups silently stop unnoticed until a disaster | Cron logs to a file; at ~12 users a periodic eyeball suffices. Automated backup-freshness alerting (Litestream / monitoring) is a documented future upgrade (D-11). | Low |

---

## Unregistered Flags (new attack surface with no threat mapping)

**None.** No plan SUMMARY carried a `## Threat Flags` section; the executor surfaced no new
attack surface during implementation. The only network-exposed surface introduced this phase is
the single authenticated `POST /api/v1/ingest` route (mapped by T-11.05-01..08); the in-process
scheduler is a `time.Ticker` heartbeat with no network surface (`scheduler/scheduler.go`); the
mint/revoke CLI subcommands are out-of-band (no HTTP surface — D-05). All discovered surface maps
to an existing registered threat.

---

## ASVS L1 Coverage Summary

| ASVS area | Where satisfied (this phase) |
|-----------|------------------------------|
| V2 (authentication) | Opaque 256-bit `crypto/rand` token, SHA-256 hash-at-rest, constant-time compare, 401-writes-nothing — 11-04 guard + 11-05 guard-first ordering; validated over TLS by the 11-07 ship-gate negative check. |
| V4 (access control) | First-sighting owner bind + cross-owner reject (409) + append-only audit — 11-03 binding; composed in one tx by 11-05. |
| V5 (input validation) | `MaxBytesReader` body cap, envelope required-field + kind-enum validation, parameterized SQL everywhere — 11-05 handler/envelope + 11-03/11-04 SQL. |
| V6 (crypto / TLS) | stdlib `crypto/rand`+`crypto/sha256`+`crypto/subtle` (never hand-rolled); Caddy auto-HTTPS valid Let's Encrypt cert; R2 encrypted-at-rest backups with a mode-600 bucket-scoped token. |
| V7 (logging hygiene) | No raw `content`, no bearer token / Authorization value ever reaches `slog` — verified across guard / store / handler. |

---

## Audit Trail

- **Loaded:** all 7 PLAN `<threat_model>` registers (11-01..07), all 7 SUMMARYs (11-01..07),
  and the implementation tree (`internal/backendsrv/{auth,ingest,store,migrations,scheduler,logging}`,
  `internal/parse`, `internal/app/runapp.go`, `cmd/squirebot-server/main.go`, `deploy/*`,
  `docs/backend-deploy.md`). No project skills present (`.claude/skills`, `.agents/skills` → none).
- **Method:** each threat classified by disposition, then verified — `mitigate` by locating the
  named control at the real entry point; `accept` by recording it above; runtime-only state by the
  authoritative `docs/backend-deploy.md` §6 live evidence. Implementation files were treated as
  READ-ONLY; nothing was modified.
- **Cross-checks:** confirmed the throwaway spike tree is GONE and the pre-1.0 PocketBase dep is
  absent from `go.mod`/`go.sum`; confirmed no `math/rand` and no `apis.RequireAuth` in production
  backend code; confirmed the only production callers of `parse.Parse`/`parse.ParseSpellbook` are
  the CP1252-wrapped watcher sites and the UTF-8 ingest handler; confirmed the only registered HTTP
  route is the authenticated ingest on loopback.
- **Result:** 39/39 CLOSED (37 mitigate verified in code/artifacts, 2 accept logged; the 11-01-03
  accept is double-counted into the 37-by-control because its control — custom guard, no PB — is
  also verifiable in code). threats_open = 0. No BLOCKER, no ESCALATE.

---
*Phase 11 (Backend Foundation + Ingest API) — SECURED. The authenticated ingest path is live over
TLS at https://api.squirebot.quest, every declared mitigation is present in the implemented code
or the live deploy artifacts, and the two accepted risks are documented above.*
