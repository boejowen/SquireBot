---
phase: 33-banks-tab-valuation
plan: 03
subsystem: deploy
tags: [deploy, binary-swap, web-atomic-swap, r2-backup, browser-smoke, no-migration]

# Dependency graph
requires:
  - phase: 33-banks-tab-valuation
    plan: 01
    provides: "the server binary that registers GET /api/v1/banks (RequireSession)"
  - phase: 33-banks-tab-valuation
    plan: 02
    provides: "the rendered /banks master-detail tab that calls fetchBanks → GET /api/v1/banks"
provides:
  - "the Banks tab LIVE at https://squirebot.quest/banks over the new GET /api/v1/banks route"
  - "the deploy + browser-smoke verdict record (BANK-01/02/03 proven on the live build)"
affects: [34 (Wishlist — the last v2.4 phase)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Binary swap (NOT goose run) when a phase adds a route but no migration; server restart registers the route; schema stays put"
    - "Web atomic swap via tarball → squirebot.new → mv-swap keeping squirebot.old (runbook §7.5, now 200.html-guarded)"
    - "Deploy driven from the Windows box via the already-loaded ssh-agent (PowerShell ssh.exe/scp.exe -o BatchMode=yes; remote multi-cmd = [char]10-joined array piped to bash -s with : BOM-bait + : end lines)"

key-files:
  created:
    - .planning/phases/33-banks-tab-valuation/33-03-SUMMARY.md
  modified: []

key-decisions:
  - "No goose run — designation/bank-coin shipped in v2.3, icon/statsblock in P31's 00012/00013; schema stays v13. Deploy = binary swap (route registration needs a restart) + web atomic swap."
  - "Pre-deploy R2 backup taken (squirebot-2026-06-19.db.gz) — standard safety net even with no schema change."

requirements-completed: [BANK-01, BANK-02, BANK-03]

# Metrics
completed: 2026-06-19
---

# Phase 33 Plan 03: Deploy + Browser-Smoke Summary

**The Banks tab is LIVE at https://squirebot.quest/banks over the new `GET /api/v1/banks` route. Deploy = a backend binary swap (server restarted to register the route — NO goose run; schema stays v13) + a web atomic swap, with a pre-deploy R2 backup. The 10-point browser-smoke PASSED on the live build across all 5 EQ themes on the first deploy (no fix-forward needed).**

## Accomplishments

### Task 1 — Regression gate + build artifacts (green)
- `go test ./...` → 0 (all packages, incl. the new `compute`/`store`/`readapi` banks tests); `npm --prefix web run check` → 0/0 (509 files); `npm --prefix web test` → 370/370 (29 files); `npm --prefix web run build` → adapter-static ok.
- Cross-compiled the `linux/amd64`, `CGO_ENABLED=0` server binary (~12.98 MB) + the web bundle tarball (~1.17 MB).

### Task 2 — Deploy to prod (R2 backup → binary swap + restart → web atomic swap; NO goose run)
- **R2 backup:** `squirebot-2026-06-19.db.gz` in `r2:squirebot-backups`.
- **Binary swap:** `scp` → `cp` running binary to `.bak` → `install -m0755` → `systemctl restart`. Startup logs clean: `goose: no migrations to run. current version: 13`, `bot connected`, `listening addr=127.0.0.1:8090`, `scheduler started jobs:4`.
- **Web atomic swap:** tarball → `/var/www/squirebot.new` → `chmod -R u=rwX,go=rX` → `mv` live → `squirebot.old` → `mv` new → live (runbook §7.5; the `200.html` guard fixed in quick task 260618-rh1).
- **External verify:** `GET /api/v1/banks` → **401** (registered + login-gated; was **404** pre-deploy), `GET /api/v1/items` → 401 (prior route healthy), apex `/` → 200, `/banks` → 200, entry JS `Content-Type: text/javascript`. Schema confirmed still **v13**.

### Task 3 — Browser-smoke on the live build (human-verify, PASSED)
Operator-verified all 10 points at https://squirebot.quest/banks across **Velious · Vanilla · Kunark · Minimalist · Heavy**:
1. valuation summary header (GUILD BANKS · value pp · platinum) — correct numbers, clean zeros;
2. bank/bot list A-Z, item counts, no per-row value, no non-bank chars;
3. select → per-bank header (per-bank slice) + reused inventory window; replace-on-select;
4. zero-inventory bank lists + empty-state; never-recorded coin → "not recorded";
5. query toggles to item-search with bank-slice qty (not guild-wide);
6. search holder-click pins that bank's window IN-TAB (no route change);
7. no-match escaped query;
8. "Pick a bank" prompt + error/Retry;
9. all 5 themes render (accent numbers/focus/border on Heavy + Minimalist);
10. mobile/narrow reflow.

## Fix-forward
None — the 10-point smoke passed clean on the first deploy.

## Deviations from Plan
None. (The runbook §7.5 `200.html` guard drift was fixed ahead of this phase in quick task `260618-rh1`, so the web swap ran without the EXTRACT_FAILED workaround Phase 32 needed.)

## Rollback
- **Backend:** `cp /usr/local/bin/squirebot-server.bak /usr/local/bin/squirebot-server && systemctl restart squirebot-server` (the `.bak` is the P32 binary; schema-v13-safe — it simply lacks the new route).
- **Web:** `rm -rf /var/www/squirebot.bad; mv /var/www/squirebot /var/www/squirebot.bad; mv /var/www/squirebot.old /var/www/squirebot`.

## Verification (must_haves)
- ✅ `GET /api/v1/banks` is live (server rebuilt + restarted) — external 401 (was 404), local 8090 401.
- ✅ The new web bundle is live (atomic swap) and `/banks` renders the master-detail tab (200 + `text/javascript`; operator-confirmed render).
- ✅ Pre-deploy R2 backup exists (`squirebot-2026-06-19.db.gz`).
- ✅ Browser-smoke confirms the load-bearing interactions across all 5 themes (operator "approved"), incl. the value/platinum numbers, the bank-slice search qty, and the in-tab holder deep-link.
- ✅ No migration applied — schema stays v13.

## Self-Check: PASSED
The Banks tab is live and the route is registered (401, not 404); the 10-point browser-smoke passed across all 5 themes (closing BANK-01/02/03); no fix-forward needed; rollback recipe recorded; this SUMMARY exists.

---
*Phase: 33-banks-tab-valuation*
*Completed: 2026-06-19*
