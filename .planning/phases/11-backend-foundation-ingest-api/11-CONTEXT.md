# Phase 11: Backend Foundation + Ingest API - Context

**Gathered:** 2026-05-28
**Status:** Ready for planning

<domain>
## Phase Boundary

Stand up the self-hosted backend so guildie watchers have a live, authenticated place to upload to again: a Hetzner Cloud VPS (US region) behind Caddy auto-HTTPS, a SQLite schema under `goose` forward-only migrations, per-guildie bearer-token auth, and the ingest endpoint that atomically replaces a character's rows. Foundation for everything after.

**In scope:** BACKEND-01 (box + Caddy + systemd + TLS), BACKEND-02 (SQLite + goose schema), BACKEND-03 (atomic full-snapshot ingest), BACKEND-04 (bearer-token auth + minting), BACKEND-06 (nightly backup + restore).

**Out of scope (later phases):** enrichment cron jobs and dimension-data population → P12 · read API + views → P14 (BACKEND-05) · watcher changes → P13 · admin HTTP forms / Discord login → P15. P11 stands up the *empty* dimension-table schema + the in-process scheduler skeleton, but populates nothing.

</domain>

<decisions>
## Implementation Decisions

> **User delegated all gray areas** (2026-05-28: *"I have no preference… use your recommendations for each"*). Everything below is locked at Claude's recommendation per the standing delegation pattern — downstream planning/research should NOT re-ask. See `feedback_delegate_gray_areas` (user memory).

> **⚠ Host change (2026-05-29):** the backend host was switched **Oracle Cloud Always Free → Hetzner Cloud VPS** at the user's direction. (Heroku was evaluated first and rejected: no free tier + an ephemeral filesystem that destroys an on-disk SQLite store.) Hetzner keeps the **entire architecture intact** — SQLite + `goose` + Caddy + systemd + single static binary; only the host, firewall, backup target, cross-compile arch, and cost change. Plans **11-01…11-05 are unaffected** (host-agnostic Go/SQL); only **11-06** (provisioning/firewall) and **11-07** (backup target) changed. The milestone's **"$0 backend" premise is retired** — cost is now ~$55/yr VPS + ~$12/yr domain ≈ **$67/yr**. See D-11/D-12/D-14 below.

### Server foundation — PocketBase vs hand-rolled Go
- **D-01:** **Plan 11-01 is a time-boxed 1-day PocketBase spike** (do it first). Pass criteria — all four must hold: (a) models `owner`/`character`/`inventory_item`/`spellbook_entry` + empty dimension tables; (b) supports the per-guildie bearer-token ingest contract via a custom route/hook doing atomic full-snapshot replace; (c) can host the P12 in-process enrichment cron via Go hooks (PocketBase-as-framework, `pocketbase.New()`); (d) runs on the deployment-target arch (Hetzner US = `linux/amd64`; amd64 is the low-risk common target, so this probe is even safer than the old arm64 one). All four pass → adopt PocketBase. Any hard blocker → hand-rolled Go fallback. Host (Hetzner Cloud VPS) + DB (SQLite) stand either way. **Capture the verdict in an 11-01 SUMMARY / appended CONTEXT note.**
- **D-02 (hand-rolled fallback shape):** Go 1.24, stdlib `net/http` ServeMux (1.22+ method+pattern routing — no router dependency), pure-Go SQLite driver **`modernc.org/sqlite` (NO cgo)** so the binary cross-compiles `GOOS=linux GOARCH=amd64` from the Windows dev box into a single static binary (mirrors the watcher's ethos; `amd64` for the Hetzner US x86 host — was `arm64` on Oracle Ampere).

### Ingest payload shape
- **D-03:** The watcher POSTs the **RAW `/outputfile` file text**; the **server parses**. Thinnest watcher; inventory/spellbook parsers live server-side (one source of parsing truth, shared with P12 enrichment parsing).
- **D-04:** Contract = `POST /api/v1/ingest` with a small JSON envelope: `{ "character": "<name>", "kind": "inventory"|"spellbook", "content": "<raw file text>", "watcher_version": "x.y.z" }`. (Char name comes from the watcher's *filename*, not the file body, so it must travel in the envelope.) Server parses `content`, validates, and **atomically replaces** that `(character, kind)`'s rows in one transaction (delete-all-then-insert; a shrinking snapshot drops removed rows). `watcher_version` is accepted now; the version-gate *reject* lands in P13.

### Guild-code lifecycle
- **D-05:** Mint via a **server-side CLI subcommand** (e.g. `squirebot-server mint-code --owner <label>`) that prints the plaintext **once**; store only a **SHA-256 hash** server-side. No admin HTTP endpoint in P11 (that's P15).
- **D-06:** Distribution: maintainer **DMs the code over Discord** (per SCOPE).
- **D-07:** **First-sighting binds** each new character name to the uploading code's owner (mirrors v1 first-write-wins). A character already owned by a *different* owner → reject with a clear error + audit log (reassignment is an admin action, P15).
- **D-08:** `Authorization: Bearer <code>`; constant-time hash compare; missing/malformed/unknown → **401, writes nothing**.
- **D-09:** Revocation: disable/delete the hashed token row (CLI subcommand); a revoked code → 401.

### Deploy & backup
- **D-10:** **Bare Go binary + systemd (`Restart=always`) + Caddy** (automatic HTTPS for the domain, reverse-proxy to the Go server on localhost). **No Docker** (single-binary ethos; less overhead on the small ARM box). Migrations **embedded via `//go:embed`** and run (`goose.Up`) on startup — deploy = "drop the new binary + restart."
- **D-11 (backup — Hetzner-adjusted 2026-05-29):** **Nightly cron `sqlite3 .backup`** (consistent hot snapshot) → gzip → upload off-box via **`rclone` to an S3-compatible object store — Cloudflare R2 (10 GB free tier, zero egress) recommended** (Backblaze B2 free tier is an equivalent fallback). A **scoped, write-only API token** lives on the box (file mode `600`) — unlike Oracle's keyless Instance Principal, `rclone` needs a stored credential (called out in the 11-07 threat model). Documented restore = pull latest snapshot, place file, start binary (`goose.Up` no-ops). Litestream (continuous replication) noted as a future upgrade if the nightly RPO proves too loose. **rclone remote config (verified working 2026-05-29 on the box):** `type=s3, provider=Cloudflare, endpoint=https://<acct-id>.r2.cloudflarestorage.com, region=auto, no_check_bucket=true`. **`no_check_bucket=true` is REQUIRED** — a bucket-scoped token can't perform rclone's default bucket-existence check (a bucket-level op) → `403 AccessDenied` on upload without it. **Do NOT set `acl`** — R2 doesn't implement S3 ACLs (→ `501 NotImplemented`). Token = **Object Read & Write** scoped to `squirebot-backups`. A benign `501`-then-retry-success on upload can occur on older rclone (checksum-trailer negotiation); the cron still exits 0, but `curl https://rclone.org/install.sh | sudo bash` (upgrade rclone) silences it.
- **D-12 (host — Hetzner, switched 2026-05-29):** **Hetzner Cloud shared-vCPU VPS** (2 vCPU / ~4 GB is ample for ~12 users), **US location (Ashburn VA or Hillsboro OR)** → x86/AMD (the `CPX` line), so the cross-compile target is **`GOARCH=amd64`**. *(Alternative: the ARM `CAX` line is EU-only — pick it and keep `GOARCH=arm64` if EU latency is acceptable; uploads aren't latency-sensitive, but a US box is better for the P14 interactive frontend.)* Always-on — **no idle-reclamation** (you pay for the VPS), so the Oracle PAYG/reclamation dance is gone, and **`ufw` works normally** (no Oracle iptables trap). ~€4–8/mo (≈ **$55–95/yr**). User provisions (see Prerequisites). **✅ Provisioned 2026-05-29 — `CPX11` (2 vCPU AMD / 2 GB / 40 GB), US, public IPv4 `5.78.232.85`; `api.squirebot.quest` A-record → that IP is live (verified via public resolvers). SSH key installed; box `apt`-updated.**
- **D-14 (domain — registered 2026-05-29):** Domain **`squirebot.quest`** registered at **Porkbun**. The backend API serves at **`api.squirebot.quest`** — the apex + `www` are reserved for the P14 SvelteKit frontend (Cloudflare/Pages), so the apex never has to be re-pointed at cutover. At deploy (11-06) a single DNS **A-record `api` → the Hetzner VPS public IPv4** is added wherever DNS is managed (Porkbun now, or Cloudflare later if nameservers move for the frontend); **Caddy issues the Let's Encrypt cert via the HTTP-01 challenge — no registrar API token needed** (DNS-01/wildcard not used). `.quest` is a standard public TLD; LE issues normally. The A-record is **blocked until the Hetzner VPS is provisioned** (needs its public IPv4). *(Locked at Claude's recommendation per the standing delegation; redirect if the backend should live elsewhere.)*

### Schema detail (owner/character split locked in SCOPE; shape here)
- **D-13:** Tables: `owner`(id, label, created_at) · `character`(id, owner_id FK, name UNIQUE, + nullable metadata: class/level/race/is_bank_toon/is_hidden/is_removed — populated later/by backfill) · `inventory_item`(character_id FK, location, name, item_id, count, slots, uploaded_at) · `spellbook_entry`(character_id FK, level, name, uploaded_at) · `guild_code`(id, owner_id FK, token_hash, label, disabled_at). **Empty** dimension tables created here too (`item_master`, `pigparse_price`, `wiki_spells`, `wiki_gear_tier`, `quest_items`) for P12 to populate. `goose` forward-only; `goose up` idempotent on a fresh DB.

### Claude's Discretion
The user delegated **every** gray area. Everything above is locked at Claude's recommendation. The researcher/planner may refine exact library versions and migration SQL, but the **choices** (spike-first; `modernc.org/sqlite`; raw-text ingest + JSON envelope; CLI mint + Discord DM + first-sighting bind; bare-binary + systemd + Caddy; nightly cron → Object Storage; the schema shape) are **locked**.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### v2.0 milestone research
- `.planning/explorations/website-milestone/SCOPE.md` — milestone synthesis; A–F phase plan; locked-vs-research deltas
- `.planning/explorations/website-milestone/01-backend-hosting.md` — backend architecture + the PocketBase flag. **finding 01's Hetzner VPS recommendation is now the chosen host (D-12); keep reading "Postgres" as "SQLite" — SQLite stays.**
- `.planning/explorations/website-milestone/03-watcher-auth.md` — bearer-token auth model + brand-verification escape
- `.planning/explorations/website-milestone/04-data-enrichment-migration.md` §6 — DB schema draft + the Postgres→SQLite port notes (`CITEXT`→`TEXT COLLATE NOCASE`, `pg_trgm`→FTS5/`LIKE`) + ingest/cutover contract

### Phase + milestone planning
- `.planning/ROADMAP.md` § "Phase 11" — goal, 5 success criteria, PocketBase spike note, ship gate
- `.planning/REQUIREMENTS.md` § BACKEND — BACKEND-01..06 acceptance detail
- `.planning/PROJECT.md` § Current Milestone + Constraints + Key Decisions — locked stack, owner/character split, watcher write contract

### Existing code to port / mirror
- `internal/parse` (watcher) — pure inventory/spellbook parser fns (`parseToRows`) to port **server-side** for ingest; same logic, new home
- `internal/sheet` (watcher) — the v1 full-snapshot atomic clear+write contract the ingest endpoint reimplements as one SQLite transaction
- `docs/apps-script-deploy.md` / apps-script parsers — referenced for schema parity (dimension tables) and P12 port targets

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **Watcher parser functions** (`internal/parse` `parseToRows`) — pure, host-agnostic; port to the server for raw-text→rows ingest parsing.
- **Structured logging** (Go `slog`, `log(level, op, fields)`) — reuse server-side; keeps logs greppable per project convention.

### Established Patterns
- **Full-snapshot replace** (clear+write per character per file, never row-diff) — ingest reimplements this as a single SQLite transaction.
- **Identity = the writer's credential** (v1: OAuth `userinfo.email` → `_char_owner`; v2: bearer token → owner, first-sighting binds character→owner).

### Integration Points
- Ingest API ← (P13) the re-targeted watcher's `internal/backend` client.
- Schema + in-process scheduler skeleton ← (P12) enrichment jobs, (P14) read API, (P15) admin forms all build on this.

</code_context>

<specifics>
## Specific Ideas

- **"Off Google" is absolute** — no Google SDK / OAuth / dependency anywhere in the backend.
- **Pure-Go, no-cgo SQLite** (`modernc.org/sqlite`) is deliberate: preserves the cross-compile-from-Windows + single-static-binary workflow the watcher established.
- **The PocketBase spike is Plan 11-01** (first); its verdict gates 11-02+.

</specifics>

<deferred>
## Deferred Ideas

- Admin HTTP endpoints for code minting / character reassignment → **P15** (P11 uses a server CLI).
- Read API + the 4 views → **P14** (BACKEND-05).
- Enrichment cron jobs + dimension-data population → **P12** (P11 only stands up the scheduler skeleton + empty tables).
- Watcher re-target / OAuth deletion / guild-code onboarding → **P13**.
- `999.22` SemVer-aware auto-update → P13/P16 (the coordinated flip relies on the updater).
- Litestream continuous replication → future backup upgrade if nightly RPO insufficient.

</deferred>

---

*Phase: 11-backend-foundation-ingest-api*
*Context gathered: 2026-05-28*
