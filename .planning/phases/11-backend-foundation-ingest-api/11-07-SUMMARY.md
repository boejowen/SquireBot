---
phase: 11-backend-foundation-ingest-api
plan: 07
subsystem: infra
tags: [sqlite-backup, rclone, cloudflare-r2, cron, restore, ship-gate, tls, smoke-test]

# Dependency graph
requires:
  - phase: 11-backend-foundation-ingest-api (Plan 06)
    provides: "Live Hetzner box behind Caddy TLS; squirebot-server systemd service; /var/lib/squirebot/squirebot.db"
  - phase: 11-backend-foundation-ingest-api (Plan 05)
    provides: "ingest handler + mint-code/revoke-code CLI (the ship-gate exercises both over the wire)"
provides:
  - "deploy/squirebot-backup.sh — nightly sqlite3 .backup hot snapshot -> gzip -> rclone copy to private Cloudflare R2 (squirebot-backups)"
  - "Nightly schedule via /etc/cron.d/squirebot-backup (0 4 * * * root)"
  - "Documented + DRILLED restore (rclone pull -> gunzip -> place -> restart; goose.Up no-ops) reconstituting the DB on a clean box"
  - "Phase 11 ship-gate PASSED: authenticated POST over TLS -> 204 + row queryable back out; unauthenticated POST -> 401 writing nothing"
affects: [13-watcher-re-target, 16-cutover-decommission]

# Tech tracking
tech-stack:
  added: [rclone, cloudflare-r2]
  patterns:
    - "Hot SQLite backup MUST use `sqlite3 .backup` (online backup API, WAL-consistent) — NEVER a raw cp of the live .db; backup is shell, never Go (modernc has no .backup API)"
    - "Off-box backup to private R2 via a bucket-scoped Object Read & Write token in rclone.conf (mode 600) — the one stored credential on the box, revocable/rotatable in Cloudflare"
    - "Restore = rclone copy latest snapshot -> gunzip -> install as /var/lib/squirebot/squirebot.db -> systemctl restart (goose.Up no-ops on the already-migrated DB)"

key-files:
  created:
    - "deploy/squirebot-backup.sh"
  modified:
    - "docs/backend-deploy.md (backup §3, restore §4, ship-gate §5, evidence §6)"

key-decisions:
  - "Schedule via /etc/cron.d/squirebot-backup (declarative drop-in) instead of `crontab -e` — this box's crontab wrapper rejected stdin install over SSH; cron.d is equivalent + more reproducible"
  - "Ship-gate test data (ShipGateChar/ShipGateTest) removed post-gate and the off-box backup refreshed, leaving a clean empty-data production DB"

patterns-established:
  - "Pattern: ship-gate smoke = mint real code -> authed POST over TLS (204) -> unauth POST (401-writes-nothing) -> query row back, proving the full TLS->Caddy->guard->parse->bind->replace->persist path end to end"

requirements-completed: [BACKEND-06]

# Metrics
duration: live ops session
completed: 2026-05-29
---

# Phase 11 — Plan 07: Backup + Ship-Gate Summary

**Nightly off-box SQLite backup to Cloudflare R2 (sqlite3 .backup → gzip → rclone, cron 0 4) with a drilled restore that reconstitutes the DB, and the Phase 11 ship-gate PASSED end-to-end — a real authenticated upload over TLS returns 204 and the row queries back out, while an unauthenticated upload returns 401.**

## Accomplishments
- Installed `deploy/squirebot-backup.sh` to `/usr/local/bin`; ran it manually → `squirebot-2026-05-29.db.gz` landed in the private R2 `squirebot-backups` bucket; `rclone.conf` tightened to mode 600.
- Scheduled the nightly backup via `/etc/cron.d/squirebot-backup` (`0 4 * * * root …`); cron active.
- **Restore drill PASSED:** pulled the snapshot from R2 → gunzip → all 12 D-13 tables + the inventory/guild_code rows present, proving reconstitution.
- **Ship-gate PASSED:** minted a real guild code; authed `POST /api/v1/ingest` over TLS → **204**; unauth POST → **401** (wrote nothing); `SELECT` returned `ShipGateChar|Rusty Dagger|12345`; first-sighting owner-bind → `ShipGateTest`.

## Task Commits
The backup script ships in the deploy-artifacts commit; the on-box backup/restore/ship-gate were executed live over SSH (autonomous: false — user-provisioned R2 + box).

1. **deploy/squirebot-backup.sh + runbook backup/restore/ship-gate sections** — `2d2334e` (feat)

_Live evidence (backup object, cron entry, restore drill, ship-gate statuses) captured in `docs/backend-deploy.md` §6._

## Files Created/Modified
- `deploy/squirebot-backup.sh` — `sqlite3 .backup` → gzip → `rclone copy r2:squirebot-backups/`
- `docs/backend-deploy.md` — backup/restore runbook, ship-gate procedure, captured evidence

## Decisions Made
- `/etc/cron.d/` drop-in for the schedule (crontab-over-SSH was fussy).
- Removed the ship-gate test rows and refreshed the off-box backup so production starts clean.

## Deviations from Plan
- **Schedule mechanism:** `/etc/cron.d/squirebot-backup` instead of a user crontab (functionally identical nightly `0 4` run; more declarative/reproducible).
- User had already created the R2 bucket + scoped token + configured the `r2` rclone remote, so Task 1 (human provisioning) was pre-satisfied; the orchestrator did the script/cron/run/drill.

## Issues Encountered
- This box's `crontab -l`/`crontab -` errored over SSH ("invalid option -- ''"); sidestepped with the `/etc/cron.d/` drop-in.
- UTF-8 BOM prepended to stdin-piped scripts; neutralized with a bait first line so it never corrupts a load-bearing line.

## User Setup Required
The user created the private R2 bucket `squirebot-backups` + a bucket-scoped Object Read & Write API token and configured the `r2` rclone remote (prerequisite for this plan). No further setup.

## Next Phase Readiness
- BACKEND-06 + the Phase 11 ship gate are satisfied. **Phase 11 (Backend Foundation + Ingest API) is COMPLETE** — the authenticated ingest path is live over TLS and disaster-recoverable. P13 (watcher re-target) can now point the watcher at `https://api.squirebot.quest/api/v1/ingest`.

---
*Phase: 11-backend-foundation-ingest-api*
*Completed: 2026-05-29*
