---
phase: 34-wishlist-rework-per-character-per-slot-upgrades
plan: 04
subsystem: deploy
tags: [deploy, goose-migration, binary-swap, web-atomic-swap, r2-backup, browser-smoke, no-v-tag]

# Dependency graph
requires:
  - phase: 34-wishlist-rework-per-character-per-slot-upgrades
    plan: 01
    provides: "migration 00014 (schema v14) + compute.WishlistFor + owner-scoped store"
  - phase: 34-wishlist-rework-per-character-per-slot-upgrades
    plan: 02
    provides: "the wantmatch repoint + owner-scoped write API + GET /api/v1/wishlist/{char}"
  - phase: 34-wishlist-rework-per-character-per-slot-upgrades
    plan: 03
    provides: "the rendered /wishlist per-character per-slot tab"
provides:
  - "the Wishlist tab LIVE at https://squirebot.quest/wishlist over GET /api/v1/wishlist/{char}, on schema v14"
  - "the deploy + browser-smoke verdict record (WISH-01..07 proven on the live build)"
affects: [v2.4 milestone — this is the LAST v2.4 phase; next = milestone audit/close]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Goose-run deploy: a phase WITH a migration applies goose on the restart (00014 → v14); R2 backup taken BEFORE the restart; confirm 'goose: successfully migrated database to version: 14' in the logs"
    - "NO v* tag on this deploy — a v* tag fires the watcher release CI needlessly (the watcher is untouched); plain binary + web swap only (memory watcher-release-versioning)"
    - "Web atomic swap via tarball → squirebot.new → mv-swap keeping squirebot.old (runbook §7.5, 200.html-guarded)"
    - "Deploy driven from the Windows box via the loaded ssh-agent (PowerShell ssh.exe/scp.exe -o BatchMode=yes; remote multi-cmd = [char]10-joined array piped to bash -s with : BOM-bait + : end lines)"

key-files:
  created:
    - .planning/phases/34-wishlist-rework-per-character-per-slot-upgrades/34-04-SUMMARY.md
  modified: []

key-decisions:
  - "This deploy RUNS goose (unlike P32/P33's no-migration swaps) — migration 00014 applies on the restart → schema v14. R2 backup taken BEFORE the restart per the migration-deploy discipline."
  - "NO v* tag — the watcher is untouched; a v* tag would needlessly fire the watcher release CI (memory watcher-release-versioning)."
  - "Migration-aware rollback recorded: the .bak binary's EC matcher references the dropped wantlist_item, so a true rollback restores the R2 pre-migration backup + the .bak binary + squirebot.old; fix-forward strongly preferred."

requirements-completed: [WISH-01, WISH-02, WISH-03, WISH-04, WISH-05, WISH-06, WISH-07]

# Metrics
completed: 2026-06-19
---

# Phase 34 Plan 04: Deploy + Browser-Smoke Summary

**The Wishlist tab is LIVE at https://squirebot.quest/wishlist on schema v14. This deploy RAN goose (migration 00014: wishlist_item created, alert_log FK-rebuilt, wantlist_item dropped — the D-01 clean break), a backend binary swap (restart applied the migration + registered the new routes), and a web atomic swap, with an R2 backup taken BEFORE the restart and NO v* tag. The 10-point browser-smoke PASSED on the live build across all 5 EQ themes on the first deploy (no fix-forward needed).**

## Accomplishments

### Task 1 — Regression gate + build (green)
- `go test ./...` → 0 FAIL (the full clean-break test repair landed in 34-02); `npm --prefix web run check` → 0/0 (508 files); `npm --prefix web test` → 380/380 (29 files); `npm --prefix web run build` → adapter-static ok.
- Cross-compiled the `linux/amd64`, `CGO_ENABLED=0` server binary (~13.0 MB) + the web bundle tarball (~1.17 MB).

### Task 2 — Deploy to prod (goose-run: R2 backup → binary swap + restart [00014 applies] → web atomic swap)
- **R2 backup (pre-migration):** `squirebot-2026-06-19.db.gz` in `r2:squirebot-backups`, taken BEFORE the restart.
- **Binary swap + goose-run:** `scp` → `cp` running binary to `.bak` → `install -m0755` → `systemctl restart`. Startup log: **`goose: successfully migrated database to version: 14`**, `listening addr=127.0.0.1:8090`. On-box confirm: schema `14`; `sqlite_master` shows `wishlist_item` present, `wantlist_item` **gone**.
- **Web atomic swap:** tarball → `/var/www/squirebot.new` → `chmod -R u=rwX,go=rX` → `mv` live → `squirebot.old` → `mv` new → live.
- **External verify:** `GET /api/v1/wishlist/{char}` → **401** (registered; was **404** pre-deploy), `GET /api/v1/wantlist` → **404** (retired), `/wishlist` → 200, `/wantlist` → 200 (308→/wishlist), apex → 200, entry JS `Content-Type: text/javascript`.

### Task 3 — Browser-smoke on the live build (human-verify, PASSED)
Operator-verified the checklist at https://squirebot.quest/wishlist across **Velious · Vanilla · Kunark · Minimalist · Heavy**:
1. char list viewer-first A-Z, banks/bots excluded, search filters;
2. select → the 21 worn equipped slots (incl. empty) with the equipped item;
3. add a target — typed + chosen-from-suggestion; "No targets yet" empty state (clean break);
4. per-slot complete Velious Pre-raid/Grouping + Raiding suggestions for class+slot, each price+wiki+last-listed, "Raid" tag + not-for-sale;
5. ping toggle + EC-hit badge;
6. examine on hover/tap (reused ExaminePanel);
7. auto-hide of held targets + remove-with-confirm;
8. WISH-07 search finds an item on a DIFFERENT character's wishlist + the non-bank/bot characters;
9. owner-scoping — non-owned characters are read-only;
10. all 5 themes render.

## Fix-forward
None — the smoke passed clean on the first deploy.

## Deviations from Plan
None.

## Rollback (migration-aware — fix-forward strongly preferred)
This deploy ran a migration, so rollback is NOT just the binary:
- **True rollback:** restore the R2 pre-migration backup `squirebot-2026-06-19.db.gz` (gunzip → install over `/var/lib/squirebot/squirebot.db`) AND `cp /usr/local/bin/squirebot-server.bak /usr/local/bin/squirebot-server` (the `.bak` EC matcher references the dropped `wantlist_item`, so it needs the pre-migration DB) AND `mv /var/www/squirebot.old /var/www/squirebot`, then `systemctl restart`.
- **Fix-forward** (preferred): patch + redeploy, the P31/P32 pattern.

## Verification (must_haves)
- ✅ `GET /api/v1/wishlist/{char}` live (server rebuilt + restarted) — external 401 (was 404); `/api/v1/wantlist` retired (404).
- ✅ Migration 00014 applied — `goose → version 14` in the log; `wantlist_item` dropped, `wishlist_item` present.
- ✅ The new web bundle is live (atomic swap) and `/wishlist` renders the per-slot tab (200 + `text/javascript`; operator-confirmed).
- ✅ Pre-migration R2 backup exists (`squirebot-2026-06-19.db.gz`).
- ✅ Browser-smoke confirms WISH-01..07 across all 5 themes (operator "approved"), incl. add/remove, ping toggle, suggestion add + "Raid" tag, auto-hide, the cross-character search, and owner-scoped read-only.
- ✅ NO v* tag (the watcher is untouched).

## Self-Check: PASSED
The Wishlist tab is live and the routes are registered (wishlist 401, wantlist 404); migration 00014 applied (schema v14); the 10-point browser-smoke passed across all 5 themes (closing WISH-01..07); no fix-forward needed; migration-aware rollback recipe recorded; this SUMMARY exists. This is the LAST v2.4 phase — the milestone is now feature-complete.

---
*Phase: 34-wishlist-rework-per-character-per-slot-upgrades*
*Completed: 2026-06-19*
