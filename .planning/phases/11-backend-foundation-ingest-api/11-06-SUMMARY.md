---
phase: 11-backend-foundation-ingest-api
plan: 06
subsystem: infra
tags: [hetzner, caddy, systemd, ufw, tls, lets-encrypt, cross-compile, linux-amd64, deploy, ssh]

# Dependency graph
requires:
  - phase: 11-backend-foundation-ingest-api (Plan 05)
    provides: "cmd/squirebot-server single binary (serve --addr/--db, mint-code, revoke-code; goose.Up on startup); CGO_ENABLED=0 linux/amd64 cross-compile"
provides:
  - "Live backend over HTTPS at api.squirebot.quest from a Hetzner Cloud VPS (US, amd64) with a valid Let's Encrypt cert"
  - "deploy/Caddyfile (api.squirebot.quest { reverse_proxy localhost:8090 }) — Caddy terminates TLS, proxies to the loopback Go server"
  - "deploy/squirebot-server.service — systemd unit, Restart=always, User=squirebot, NoNewPrivileges, survives reboot"
  - "docs/backend-deploy.md — reproducible deploy/ops runbook (cross-compile, scp, ufw, Caddy apt-repo, systemd, verify) with captured live evidence"
affects: [11-07, 13-watcher-re-target, 14-web-frontend, 15-admin-web-forms-login, 16-cutover-decommission]

# Tech tracking
tech-stack:
  added: [caddy-2.11.3, sqlite3-cli-3.45.1]
  patterns:
    - "Deploy = cross-compile (CGO_ENABLED=0 GOOS=linux GOARCH=amd64 -trimpath -ldflags '-s -w') -> scp -> systemctl restart (D-10); migrations embedded, goose.Up on first start"
    - "Caddy auto-HTTPS: 2-line Caddyfile, Let's Encrypt HTTP-01, reverse_proxy to loopback:8090; the Go server never binds a public port"
    - "on-box ufw (allow OpenSSH BEFORE enable) is the load-bearing firewall — Hetzner has no Oracle-style two-layer iptables trap; 8090 stays loopback-only"
    - "/etc/cron.d declarative drop-in chosen over `crontab -e` (see 11-07) — stdin crontab install was fussy over SSH"

key-files:
  created:
    - "deploy/Caddyfile"
    - "deploy/squirebot-server.service"
    - "docs/backend-deploy.md"
  modified:
    - ".gitattributes (force LF on deploy/*.sh, Caddyfile, *.service)"
    - ".gitignore (ignore the cross-compiled squirebot-server binary)"

key-decisions:
  - "Used the official Caddy cloudsmith apt repo (Caddy is NOT in Ubuntu's default repos) + installed sqlite3 in the same step"
  - "Service user `squirebot` (system, no home) owns /var/lib/squirebot; binary runs NoNewPrivileges on loopback:8090"
  - "Deploy driven by the orchestrator over SSH (user delegated it after provisioning the VPS + DNS + loading the ssh key into the agent)"

patterns-established:
  - "Pattern: pipe complex remote scripts via stdin to `ssh bash -s` (CR-stripped, BOM-bait first line) to avoid PowerShell->ssh quoting corruption"

requirements-completed: [BACKEND-01]

# Metrics
duration: live deploy session
completed: 2026-05-29
---

# Phase 11 — Plan 06: On-Box Deploy Summary

**The single cross-compiled Go binary is live over HTTPS at api.squirebot.quest on a Hetzner Cloud VPS (US/amd64), behind Caddy auto-HTTPS with a valid Let's Encrypt cert, supervised by systemd (Restart=always, reboot-survival proven), firewalled by ufw (22/80/443; 8090 loopback-only).**

## Accomplishments
- Cross-compiled `squirebot-server` (CGO_ENABLED=0 linux/amd64, ~10.4 MB static ELF) from the Windows dev box and deployed it to `/usr/local/bin` on the VPS.
- Stood up Caddy 2.11.3 (official apt repo) with the 2-line Caddyfile; valid Let's Encrypt cert auto-issued (`curl https://api.squirebot.quest/` → `404 TLS_VERIFY=0`).
- systemd unit enabled + started; `goose.Up` created all 12 D-13 tables on first start; **reboot survival confirmed** (service auto-started, data persisted).
- `ufw` active allowing 22/80/443 (v4+v6); port 8090 not externally reachable.

## Task Commits
Deploy artifacts were authored + committed locally; the on-box steps were executed live over SSH (autonomous: false — user-provisioned infra).

1. **Deploy artifacts + runbook** — `2d2334e` (feat)
2. **LF line-ending pin for deploy artifacts** — `63fb078` (build)
3. **gitignore cross-compiled binary** — `b3d4e3e` (build)
4. **Caddy apt-repo install fix in runbook** — `04ea7b5` (fix)

_Live evidence captured in `docs/backend-deploy.md` §6 (not a code commit — infrastructure state)._

## Files Created/Modified
- `deploy/Caddyfile` — `api.squirebot.quest { reverse_proxy localhost:8090 }`
- `deploy/squirebot-server.service` — systemd unit (Restart=always, loopback ExecStart, NoNewPrivileges)
- `docs/backend-deploy.md` — full deploy/ops runbook + captured evidence
- `.gitattributes`, `.gitignore` — LF pinning + binary ignore

## Decisions Made
- Caddy via the official cloudsmith apt repo (default Ubuntu repos lack it); sqlite3 installed alongside.
- The orchestrator drove the deploy over SSH after the user provisioned the VPS/DNS and loaded the `squirebot-hetzner` ed25519 key into ssh-agent.

## Deviations from Plan
- **Runbook Caddy install corrected:** the original `apt install caddy` would fail on stock Ubuntu — fixed to add the official Caddy apt repo (+ sqlite3). (`04ea7b5`)
- **Cross-compile artifact gitignored + deploy files pinned to LF** (`.gitattributes`) so the bash backup script's shebang doesn't break with CRLF when checked out on Windows.

## Issues Encountered
- PowerShell→ssh argument quoting mangled inline remote commands with `()`/`""`; resolved by piping scripts via stdin to `ssh bash -s` (with CR-strip + a BOM-bait first line).
- A background reboot-poll loop stalled on connection timing during the reboot window; a direct probe confirmed the box returned healthy.

## User Setup Required
The user provisioned the Hetzner VPS + DNS A-record (`api.squirebot.quest` → `5.78.232.85`, Hillsboro OR) and loaded the SSH key — prerequisites for this plan (autonomous: false). No further setup for 11-06.

## Next Phase Readiness
- BACKEND-01 satisfied. The live box is ready for 11-07 (backup + ship-gate) — which is also complete.

---
*Phase: 11-backend-foundation-ingest-api*
*Completed: 2026-05-29*
