# SquireBot Backend — Deploy & Operations Runbook

The v2.0 "Off Google" backend is a **single static Go binary** (`squirebot-server`)
behind **Caddy** (auto-HTTPS) on a **Hetzner Cloud VPS (US, amd64)**, supervised by
**systemd** (`Restart=always`), with **SQLite** at `/var/lib/squirebot/squirebot.db`
and a **nightly off-box backup** to **Cloudflare R2**. There is no Docker, no Google,
no OAuth — the only secret the server handles is the guild-code SHA-256 hash in SQLite.

- **Host:** Hetzner Cloud VPS (CPX line, US — Ashburn VA or Hillsboro OR → x86/amd64), Ubuntu, always-on.
- **Domain:** `api.squirebot.quest` (registered at Porkbun; apex/`www` reserved for the P14 frontend).
- **Listen:** the Go server binds loopback `127.0.0.1:8090`; Caddy terminates TLS on `:443` and reverse-proxies to it.
- **Deploy model (D-10):** cross-compile → scp the binary → `systemctl restart`. Migrations are embedded; `goose.Up` runs on startup (idempotent).

> Artifacts referenced below live in the repo: `deploy/Caddyfile`, `deploy/squirebot-server.service`, `deploy/squirebot-backup.sh`.

---

## 1. Cross-compile the binary (on the Windows dev box)

`modernc.org/sqlite` is pure Go, so `CGO_ENABLED=0` cross-compiles with no C toolchain.
Flip `GOARCH=amd64` for the US Hetzner x86 box (use `arm64` only for an EU-only CAX box):

```powershell
$env:GOOS="linux"; $env:GOARCH="amd64"; $env:CGO_ENABLED="0"
go build -trimpath -ldflags "-s -w" -o squirebot-server ./cmd/squirebot-server
$env:GOOS=""; $env:GOARCH=""; $env:CGO_ENABLED=""   # reset env afterward
```

This produces a static `linux/amd64` ELF `squirebot-server` (~10 MB stripped). `scp` it to the box in §2.4.

---

## 2. On-box deploy (run these on the Hetzner VPS over SSH)

> Prereq: the VPS is provisioned and you can `ssh` in; the DNS A-record `api` → the VPS
> public IPv4 is set (Caddy issues the Let's Encrypt cert via the HTTP-01 challenge — no
> registrar API token needed). Document the public IPv4 + region in the evidence section.

### 2.0 Copy the artifacts up (from the dev box)
`scp` the cross-compiled binary + the three deploy artifacts to `/tmp/` on the box (use your
box's SSH user — Hetzner Ubuntu defaults to `root`):
```powershell
scp squirebot-server deploy/Caddyfile deploy/squirebot-server.service deploy/squirebot-backup.sh root@<vps-ip>:/tmp/
ssh root@<vps-ip>          # then run 2.1–2.5 on the box
```

### 2.1 Create the service user + data dir
```bash
sudo useradd --system --no-create-home squirebot
sudo mkdir -p /var/lib/squirebot
sudo chown squirebot:squirebot /var/lib/squirebot
```

### 2.2 Firewall — `ufw` (the load-bearing layer; works normally on Hetzner)
Allow **SSH first** so you don't lock yourself out, then HTTP/HTTPS, then enable:
```bash
sudo ufw allow OpenSSH        # or: sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw --force enable
sudo ufw status               # expect 22, 80, 443 = ALLOW
```
The Go server's `8090` stays loopback-only — **do NOT open it**. (If you also attached a
Hetzner Cloud Firewall, confirm it admits 22/80/443 too — both layers must allow the traffic.)

### 2.3 Install Caddy (+ sqlite3) and drop the Caddyfile
Caddy is **not** in Ubuntu's default repos — add the official Caddy apt repo first, and grab
`sqlite3` while here (needed for the backup/restore in §3–§4 and the `.tables` check below):
```bash
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https curl
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update
sudo apt install -y caddy sqlite3                  # caddy.service + the sqlite3 CLI
sudo cp /tmp/Caddyfile /etc/caddy/Caddyfile        # (copied up in 2.0)
sudo systemctl reload caddy                         # Caddy auto-provisions the Let's Encrypt cert
```
`deploy/Caddyfile` is the entire config:
```
api.squirebot.quest {
    reverse_proxy localhost:8090
}
```

### 2.4 Deploy the binary + systemd unit
```bash
sudo install -m 0755 /tmp/squirebot-server /usr/local/bin/squirebot-server
sudo cp /tmp/squirebot-server.service /etc/systemd/system/    # (copied up in 2.0)
sudo systemctl daemon-reload
sudo systemctl enable --now squirebot-server      # first start runs goose.Up → creates the schema
```

### 2.5 Verify HTTPS end-to-end (from OUTSIDE the box)
```bash
curl -v https://api.squirebot.quest/              # valid Let's Encrypt cert; a 404/405 here is FINE
                                                  # (the only real route is the authenticated POST ingest)
```
A `404`/`405` on `/` with a **valid TLS cert** proves Caddy + the binary are live (the binary
only registers `POST /api/v1/ingest`). Then confirm the service + schema + reboot survival:
```bash
sudo systemctl status squirebot-server            # active (running), Restart=always
sudo -u squirebot sqlite3 /var/lib/squirebot/squirebot.db '.tables'   # D-13 tables present
sudo reboot                                       # then re-check status after it comes back
```

**Acceptance (BACKEND-01):** external `curl` gets a valid cert + the binary answers behind Caddy;
`systemctl status` is `active (running)` with `Restart=always`; the service returns after reboot;
`ufw` shows 22/80/443 ALLOW; port 8090 is NOT externally reachable; `goose.Up` created the schema.

---

## 3. Nightly off-box backup (BACKEND-06)

`deploy/squirebot-backup.sh` takes a **consistent** hot snapshot via the `sqlite3` **online
backup API** (never a raw `cp` of the live WAL-mode `.db` — Pitfall 6), gzips it, and `rclone copy`s
it to the private R2 bucket. The backup is **shell, never Go** (modernc has no `.backup` API).

### 3.1 Cloudflare R2 prerequisites (one-time)
1. Create a **private** R2 bucket `squirebot-backups` (no public access).
2. Mint an **R2 API token scoped to that bucket** with **Object Read & Write** (not account-wide);
   record the Access Key ID, Secret, and account ID. This is the one stored credential on the box —
   scope it tight; it's revocable/rotatable in the Cloudflare dashboard.
3. On the box, install `rclone` and configure the `r2` remote:
   ```bash
   sudo apt install -y rclone
   rclone config    # new remote: name=r2, type=s3, provider=Cloudflare,
                    # access_key_id + secret_access_key = the token,
                    # endpoint=https://<accountid>.r2.cloudflarestorage.com
   sudo chmod 600 ~/.config/rclone/rclone.conf   # (or root's rclone.conf — owned by the cron user)
   rclone ls r2:squirebot-backups                # confirms the remote + token work
   ```

### 3.2 Install the script + cron
```bash
sudo install -m 0755 /tmp/squirebot-backup.sh /usr/local/bin/squirebot-backup.sh   # (copied up in 2.0)
sudo /usr/local/bin/squirebot-backup.sh                       # run once manually to verify
rclone ls r2:squirebot-backups                                # expect squirebot-<DATE>.db.gz present
sudo crontab -e   # add:  0 4 * * * /usr/local/bin/squirebot-backup.sh >> /var/log/squirebot-backup.log 2>&1
sudo crontab -l                                               # confirm the entry
```

---

## 4. Restore (the BACKEND-06 "reconstitutes on a clean box" proof)

**Clean-box restore:** provision per §2, then instead of an empty DB, pull the latest snapshot:
```bash
rclone copy r2:squirebot-backups/squirebot-<DATE>.db.gz /tmp/
gunzip /tmp/squirebot-<DATE>.db.gz
sudo install -o squirebot -g squirebot /tmp/squirebot-<DATE>.db /var/lib/squirebot/squirebot.db
sudo systemctl restart squirebot-server     # goose.Up no-ops on the already-migrated DB
```

**Restore drill (proves it without clobbering prod):** pull a snapshot to a scratch path and inspect:
```bash
rclone copy r2:squirebot-backups/squirebot-<DATE>.db.gz /tmp/
gunzip /tmp/squirebot-<DATE>.db.gz
sqlite3 /tmp/squirebot-<DATE>.db '.tables'                    # D-13 tables present
sqlite3 /tmp/squirebot-<DATE>.db 'SELECT COUNT(*) FROM inventory_item;'   # rows survived
```

---

## 5. Phase 11 ship-gate smoke (real authenticated upload over TLS)

This is the ROADMAP ship gate: the server accepts a real upload over TLS and the row queries back out.

```bash
# 1. Mint a real guild code on the box (plaintext printed ONCE):
sudo -u squirebot /usr/local/bin/squirebot-server mint-code --owner "ShipGateTest"

# 2. Authenticated upload over TLS (from any machine) — expect HTTP 204:
curl -i -X POST https://api.squirebot.quest/api/v1/ingest \
  -H "Authorization: Bearer <code>" \
  -H "Content-Type: application/json" \
  -d '{"character":"ShipGateChar","kind":"inventory","content":"General1\tRusty Dagger\t12345\t1\t0\n","watcher_version":"2.0.0-smoke"}'

# 3. Negative check (BACKEND-04 over the wire) — same POST, NO Authorization header — expect HTTP 401:
curl -i -X POST https://api.squirebot.quest/api/v1/ingest \
  -H "Content-Type: application/json" \
  -d '{"character":"ShipGateChar","kind":"inventory","content":"General1\tRusty Dagger\t12345\t1\t0\n","watcher_version":"2.0.0-smoke"}'

# 4. Row queryable back out (the gate) — on the box:
sudo -u squirebot sqlite3 /var/lib/squirebot/squirebot.db \
  "SELECT c.name, i.name, i.item_id FROM inventory_item i JOIN character c ON c.id=i.character_id WHERE c.name='ShipGateChar';"
# expect:  ShipGateChar|Rusty Dagger|12345
```

**Pass criteria:** the authenticated POST returns **204**, the unauthenticated POST returns **401**
(and wrote nothing), and the row queries back out — proving the full path
(TLS → Caddy → bearer guard → parse → first-sighting bind → atomic replace → persisted) works end to end.

---

## 6. Deploy evidence

> Captured from the live deploy on **2026-05-29** (Hetzner Cloud VPS, Ubuntu 24.04, root).

- **VPS public IPv4 / region:** `5.78.232.85` — Hetzner Hillsboro OR (`us-west`), x86/amd64. DNS A-record `api.squirebot.quest` → `5.78.232.85` confirmed.
- **External HTTPS / cert:** `curl https://api.squirebot.quest/` → `HTTP=404 TLS_VERIFY=0` (valid Let's Encrypt cert; 404 is expected — only `POST /api/v1/ingest` is routed).
- **`systemctl status squirebot-server`:** `active (running)`, `enabled`; logs show `goose: successfully migrated database to version: 2` and `squirebot-server listening addr=127.0.0.1:8090`.
- **Reboot survival:** after `systemctl reboot`, the box returned (`uptime` reset) with `squirebot-server` = `active`, `enabled`, HTTPS 404, and the ship-gate row still present — auto-start confirmed.
- **`ufw status`:** active; `22/tcp (OpenSSH)`, `80/tcp`, `443/tcp` = ALLOW (v4 + v6); port `8090` NOT opened (loopback-only).
- **`.tables` on the box (goose.Up schema):** all 12 D-13 tables present — `owner character inventory_item spellbook_entry guild_code item_master pigparse_price quest_items wiki_gear_tier wiki_spells audit_log goose_db_version`.
- **Caddy / sqlite3 versions:** Caddy `v2.11.3`, sqlite3 `3.45.1` (installed via the official Caddy apt repo).
- **Backup object in R2 (`rclone ls r2:squirebot-backups`):** `squirebot-2026-05-29.db.gz` present (clean-state snapshot, 3648 bytes). `rclone.conf` at `/root/.config/rclone/rclone.conf`, mode `600`.
- **Backup schedule:** `/etc/cron.d/squirebot-backup` → `0 4 * * * root /usr/local/bin/squirebot-backup.sh >> /var/log/squirebot-backup.log 2>&1` (cron `active`). *(Used `/etc/cron.d/` rather than `crontab -e` — this box's `crontab` wrapper rejected stdin install over SSH; the cron.d drop-in is equivalent and more declarative.)*
- **Restore drill:** pulled `squirebot-2026-05-29.db.gz` from R2 → gunzip → `.tables` showed all 12 D-13 tables; `inventory_item` row + `guild_code` row present (before cleanup) — reconstitution proven.
- **Ship-gate (2026-05-29):** authenticated `POST /api/v1/ingest` over TLS → **204**; unauthenticated POST → **401** (wrote nothing); queried row back → `ShipGateChar|Rusty Dagger|12345|1`; first-sighting owner-bind → `ShipGateTest`. Test data removed post-gate (DB returned to a clean empty-data state; backup refreshed).

---

## 7. Phase 15 — auth deploy (Discord login + admin forms)

Phase 15 adds **Discord OAuth2 login** + the **officer-only admin write forms**. The
server now reads four Discord secrets + two origin settings from the environment; the
frontend gains the authenticated `/admin` + `/bank-coin` write surface. This section
is the redeploy after Phases 11–14 — it does NOT replace §2.

### 7.1 The auth env file (root-only, chmod 600)
The six auth settings ride in a root-only `EnvironmentFile` — **never** in the repo, the
static bundle, or the logs. Create it on the box:
```bash
sudo mkdir -p /etc/squirebot
sudo tee /etc/squirebot/squirebot.env >/dev/null <<'EOF'
DISCORD_CLIENT_ID=<your-discord-app-client-id>
DISCORD_CLIENT_SECRET=<your-discord-app-client-secret>
DISCORD_GUILD_ID=<your-guild-snowflake>
DISCORD_REDIRECT_URI=https://api.squirebot.quest/api/v1/auth/callback
SQUIREBOT_WEB_ORIGIN=https://squirebot.quest
SQUIREBOT_COOKIE_DOMAIN=squirebot.quest
EOF
sudo chown root:root /etc/squirebot/squirebot.env
sudo chmod 600 /etc/squirebot/squirebot.env      # backend-only; secret never leaves the box
```
- `DISCORD_*` come from the Discord application (the redirect URI MUST be registered on the app's OAuth2 page **exactly** as above).
- `SQUIREBOT_WEB_ORIGIN` is the static-site origin (CORS allow-origin + credentialed-fetch target, D-04/D-05).
- `SQUIREBOT_COOKIE_DOMAIN=squirebot.quest` lets the session cookie ride cross-subdomain from `squirebot.quest` → `api.squirebot.quest` (D-05).

### 7.2 The systemd unit reads it (already in `deploy/squirebot-server.service`)
The committed unit carries the optional reference (re-copy + reload after editing the env):
```ini
EnvironmentFile=-/etc/squirebot/squirebot.env   # leading `-` = optional (boots without it)
```
```bash
sudo cp /tmp/squirebot-server.service /etc/systemd/system/   # if re-copying the unit
sudo systemctl daemon-reload
sudo systemctl restart squirebot-server                      # picks up the env + runs goose.Up
```

### 7.3 Migration applies on boot
`00004_web_auth.sql` is embedded; `goose.Up` runs it on the restart above (idempotent —
schema **v4**: `web_user`, `web_session`, `guild_admins`, `meta` owner-floor). Confirm:
```bash
sudo -u squirebot sqlite3 /var/lib/squirebot/squirebot.db \
  "SELECT version_id FROM goose_db_version ORDER BY version_id DESC LIMIT 1;"   # expect 4
sudo -u squirebot sqlite3 /var/lib/squirebot/squirebot.db '.tables'            # web_user/web_session/guild_admins present
```

### 7.4 Seed the owner-floor once
The maintainer is the un-removable owner-floor (the bootstrap officer + D-09 data
protection). Seed it once with the maintainer's **Discord USER id** (the snowflake, NOT
the username):
```bash
sudo -u squirebot /usr/local/bin/squirebot-server set-owner-floor <maintainer-discord-USER-id>
```
This is idempotent — re-running it re-asserts the same floor.

### 7.5 Deploy the frontend bundle
The web app is a static SvelteKit build (`@sveltejs/adapter-static`) served by Caddy's
`file_server` at the apex `squirebot.quest`. Build on the dev box, ship `web/build/`:
```powershell
cd web; npm run build           # emits web/build/ (index.html + 200.html + assets)
scp -r web/build/* root@<vps-ip>:/var/www/squirebot/
```
`deploy/Caddyfile` currently holds only the `api.squirebot.quest` reverse-proxy block;
add an apex `file_server` block to serve the bundle (with the SPA fallback to `200.html`):
```
squirebot.quest {
    root * /var/www/squirebot
    try_files {path} /200.html
    file_server
}
```
Then reload Caddy:
```bash
sudo systemctl reload caddy
```

**Acceptance (AUTH-08/09, ADMIN-04/05/06):** login via Discord works at `squirebot.quest`;
a member can record bank coin; an officer sees `/admin` (evict + restore + officer-mgmt);
a non-officer gets the Officers-only refusal; the session cookie rides cross-subdomain to
`api.squirebot.quest`. Live/visual smokes are tracked in `15-HUMAN-UAT.md`.
