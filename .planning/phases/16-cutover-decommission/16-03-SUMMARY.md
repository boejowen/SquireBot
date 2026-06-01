---
phase: 16-cutover-decommission
plan: 03
subsystem: cutover-ops
tags: [deploy, systemd, caddy, mint-code, guild-code, discord-herding, browser-smoke, ops, human-action]

# Dependency graph
requires:
  - phase: 16-cutover-decommission
    provides: "16-01 char-meta endpoint+form (deployed here) + 16-02 published v2.0.0 Release (the flip target)"
  - phase: 11-backend-foundation
    provides: "the live Hetzner VPS / systemd / Caddy + the mint-code CLI + the /api/v1/ingest path"
  - phase: 13-watcher-retarget-onboarding
    provides: "the v2.0 onboarding (PromptGuildCode -> /whoami -> credstore DPAPI -> ingest)"
provides:
  - "the char-meta endpoint+form deployed live to api.squirebot.quest / squirebot.quest"
  - "11 unique active guild codes minted (one per guildie), hashed at rest"
  - "the coordinated watcher flip underway (Discord herding, no %-gate)"
affects: [16-04-decommission, milestone-close-v2.0]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Static web deploy MUST chmod -R a+rX /var/www/squirebot after sudo cp (root umask leaves _app dirs 700 -> caddy can't traverse -> blank screen) — now in backend-deploy.md §7.5"

key-files:
  created:
    - "(external) live deploy: squirebot-server (linux/amd64, 11.23MB) + web/build (91 files) on the Hetzner VPS; systemd active; /var/www/squirebot served by Caddy"
    - "(external, transient secrets) 11 minted guild codes — plaintext shown once on the box, only SHA-256 hashes persist; distributed 1:1 via Discord DM by the maintainer"
  modified:
    - "docs/backend-deploy.md — §7.5 chmod a+rX permission fix (the blank-screen incident)"
    - "docs/install.md, docs/troubleshooting.md, docs/privacy-policy.md, docs/index.md, README.md — user-facing docs updated to the v2.0 guild-code method (no Google)"

key-decisions:
  - "Deploy + mint executed by the orchestrator over SSH (maintainer-authorized; the ssh-agent was loaded), NOT a gsd-executor — 16-03 is autonomous:false. The plaintext codes were minted by the MAINTAINER running a script in their own terminal so secrets never entered the transcript; the orchestrator only verified the count over SSH"
  - "Send order: deploy -> mint -> DM codes -> (16-02 publish) -> announce, so no watcher updates before its guildie has a code"

patterns-established:
  - "Server-side ground-truth verification of every write (the char-meta smoke read-back; the Slampeach ingest read-back) — UI/visual success is not trusted alone (web-tests-node-only-blind-to-dom)"

requirements-completed: [CUTOVER-03]

# Metrics
completed: 2026-05-31
---

# Phase 16 Plan 03: Coordinated Watcher Flip — Deploy + Mint + Herd Summary

**Deployed the Plan-01 char-meta surface live, minted 11 per-guildie unique guild codes, and kicked off the Discord-herded watcher flip onto the published v2.0.0 binary (CUTOVER-03). Validated end-to-end with the first real guildie reporting in.**

> **Reconciliation note:** 16-03 is `autonomous: false`. The deploy + mint were executed conversationally by the orchestrator over SSH (maintainer-authorized, ssh-agent loaded); the plaintext-code minting + the Discord DMs were performed by the maintainer. Not a `gsd-executor` agent. This SUMMARY records it after the fact. **Herding is still in progress (1/11 reporting in) — the phase is NOT closed; Plan 16-04 (decommission) is intentionally deferred until a representative share of the guild has migrated.**

## What was done

### Task 1 — Deploy (live)
- Cross-compiled the server `linux/amd64` (CGO_ENABLED=0, `-trimpath -ldflags "-s -w"` → 11.23 MB ELF) and built the web bundle (`npm run build` → 91 files, char-meta route present).
- scp'd both up; `install` the binary + `systemctl restart` (active; `goose` no-op — no new migration); replaced `/var/www/squirebot` with the new bundle.
- **Blank-screen incident, found + fixed:** `sudo cp -r` left the `_app/` dirs `drwx------` (root umask) so the `caddy` user couldn't traverse them → Caddy served `200.html` for every JS asset → blank page. Fixed with `sudo chmod -R a+rX /var/www/squirebot`; documented in `backend-deploy.md §7.5`.
- Verified: `GET /api/v1/char/meta-list` → **401** (live + session-gated), site serves 200, `/char-meta` in the bundle.
- **Deferred 16-01 browser smoke PASSED:** the maintainer set class/level/race/is_bank_toon on a seeded throwaway char — the CR-01 level input did not crash and the values **persisted server-side** (read back `class=WAR level=50 race=HFL bank=1`). The throwaway char + owner were then deleted (DB back to pristine).

### Task 2 — Mint 11 unique codes
- Cleaned two pre-existing leftover demo rows first (owner `demo-remove-me` + disabled code `VerifyP13`) → DB pristine (0/0/0).
- The maintainer ran a mint script (written to the box, then run in their own terminal so the plaintext codes never entered chat) — one `mint-code --owner <handle>` per guildie. Orchestrator verified over SSH: **11 codes, 11 active, 11 owners**, labels = the 11 Discord handles (`747_8i`, `infinityparalax`, `.littleyo`, `scumy_03029`, `.guandi`, `ancientwolfshirt_49909`, `areyouafraidoftheclark`, `tontinethearistocrat`, `broccolifart`, `bayne7400`, `lern41`). Only SHA-256 hashes persist.

### Task 3 — Herd
- Per-guildie DM + guild-channel announcement drafted (announcement carries the `/releases/latest/download/SquireBot-Setup.exe` permalink for dormant watchers). The maintainer distributes the codes 1:1 and herds; **no migration-% gate** (D-08).

## End-to-end validation
First real guildie reporting in: **`Slampeach` | owner=broccolifart | 135 inventory items + 54 spellbook entries | last_seen 2026-06-01T00:03:47Z.** The full path — EQ `/outputfile` → fsnotify → upload → guild-code auth → SQLite — works, and the code→owner binding is correct. **Reporting in: 1/11** (herding ongoing).

## User Setup Required
- Remaining herd: the other 10 guildies reinstall/update + paste their codes (in progress).
- A watcher stuck on a pre-1.0 prerelease needs a MANUAL reinstall (see 16-02 / `troubleshooting.md`).

## Self-Check: PASSED

Live checks confirm the deploy (`/api/v1/char/meta-list` → 401, site serves, char-meta in the bundle), the mint (`SELECT COUNT(*) FROM guild_code` = 11, all active), and end-to-end ingest (Slampeach: 135 items + 54 spells persisted, bound to owner `broccolifart`). The browser smoke (CR-01 + persistence) passed against production.

---
*Phase: 16-cutover-decommission*
*Completed: 2026-05-31*
