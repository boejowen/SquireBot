---
phase: 32-inventory-tab-item-centric
plan: 03
subsystem: deploy
tags: [deploy, binary-swap, web-atomic-swap, r2-backup, browser-smoke, no-migration, fix-forward]

# Dependency graph
requires:
  - phase: 32-inventory-tab-item-centric
    plan: 01
    provides: "the server binary that registers GET /api/v1/items (RequireSession)"
  - phase: 32-inventory-tab-item-centric
    plan: 02
    provides: "the rendered /inventory master-detail tab bundle that calls fetchItems → GET /api/v1/items"
provides:
  - "the item-centric Inventory tab LIVE at https://squirebot.quest/inventory over the new GET /api/v1/items route"
  - "the deploy + browser-smoke verdict record (ITEM-01/02/03 proven on the live build)"
affects: [33 (Banks tab — same deploy path + reusable rollup-list/holders shape), 34 (Wishlist)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Binary swap (NOT goose run) when a phase adds a route but no migration: the server MUST restart to register the route; schema stays put"
    - "Web atomic swap via tarball → squirebot.new → mv-swap keeping squirebot.old (runbook §7.5); load-bearing chmod u=rwX,go=rX"
    - "Deploy driven from the Windows box via the already-loaded ssh-agent: PowerShell ssh.exe/scp.exe -o BatchMode=yes; remote multi-cmd = string-array joined [char]10 piped to bash -s with : BOM-bait first + : end last lines"
    - "Fix-forward browser-smoke commit (the P31 close pattern): smoke finds a real-data UI bug → patch → web-only redeploy → re-smoke"

key-files:
  created:
    - .planning/phases/32-inventory-tab-item-centric/32-03-SUMMARY.md
  modified:
    - web/src/routes/inventory/+page.svelte

key-decisions:
  - "No goose run this phase — icon_id/statsblock shipped in P31's 00012/00013; schema stays v13. The deploy is a binary swap (route registration needs a restart) + a web atomic swap, NOT a migration."
  - "Pre-deploy R2 backup taken anyway (squirebot-2026-06-18.db.gz, 617KB) — the standard safety net even with no schema change."
  - "Fix-forward: the reused ExaminePanel's position:sticky (correct for the P31 character window) covered the holders table on scroll in the inventory detail column; overridden to static IN THE INVENTORY TAB ONLY via a scoped :global(.examine) wrapper — the shared component + P31 are unchanged."

requirements-completed: [ITEM-01, ITEM-02, ITEM-03]

# Metrics
completed: 2026-06-18
---

# Phase 32 Plan 03: Deploy + Browser-Smoke Summary

**The item-centric Inventory tab is LIVE at https://squirebot.quest/inventory over the new `GET /api/v1/items` route. Deploy = a backend binary swap (server restarted to register the route — NO goose run; schema stays v13) + a web atomic swap, with a pre-deploy R2 backup. The 7-point browser-smoke PASSED on the live build across all 5 EQ themes after one fix-forward (un-sticking the examine panel so it stopped covering the holders table on scroll).**

## Accomplishments

### Task 1 — Regression gate + build artifacts (green)
- `go test ./...` → 0 (all 31 packages ok, incl. the new `compute`/`readapi`); `npm --prefix web run check` → 0 errors/0 warnings (507 files); `npm --prefix web test` → 359/28; `npm --prefix web run build` → adapter-static ok.
- Cross-compiled the `linux/amd64`, `CGO_ENABLED=0` server binary (~12.96 MB) + packaged the web bundle tarball (~1.17 MB).

### Task 2 — Deploy to prod (R2 backup → binary swap + restart → web atomic swap; NO goose run)
- **R2 backup:** `/usr/local/bin/squirebot-backup.sh` → `squirebot-2026-06-18.db.gz` (617,151 B) in `r2:squirebot-backups`.
- **Binary swap:** `scp` → `cp` running binary to `/usr/local/bin/squirebot-server.bak` → `install -m0755` → `systemctl restart squirebot-server`. Startup logs clean: `goose: no migrations to run. current version: 13`, `bot connected`, `listening addr=127.0.0.1:8090`, `scheduler started jobs:4`.
- **Web atomic swap:** tarball → `/var/www/squirebot.new` → `chmod -R u=rwX,go=rX` → `mv` live → `squirebot.old` → `mv` new → live (runbook §7.5).
- **External verify (from outside the box):** `GET /api/v1/items` → **401** (registered + login-gated; was **404** pre-deploy — the route is now live), `GET /api/v1/characters` → 401 (backend healthy), apex `/` → 200, `/inventory` → 200, entry JS `Content-Type: text/javascript` (fresh bundle, no blank-screen). Schema confirmed still **v13**.

### Task 3 — Browser-smoke on the live build (human-verify, PASSED)
Operator-verified all 7 points at https://squirebot.quest/inventory across **Velious · Vanilla · Kunark · Minimalist · Heavy**:
1. list = one row per item, `{qty} · {N} holders` headline + inline PigParse price (omitted cleanly when unpriced) + working Wiki ↗;
2. viewer's items first at rest; viewer-priority search; escaped no-results echo;
3. select → pin examine (D-08 order, omission) → replace-on-click (single panel);
4. holders table Character · slot · Qty · Last-synced, viewer's chars first;
5. **holder row deep-links to `/characters?c=<name>` and opens that character's inventory window** (the load-bearing cross-tab jump);
6. wiki PNG icons + colored-tile fallback (no broken-image glyphs);
7. all 5 themes render legibly.

## Fix-forward (round 1 — browser-smoke)
- **`c5b2bc4` fix(32): un-stick the inventory examine panel so it stops covering holders.** Smoke found that the reused `ExaminePanel`'s `position: sticky; top: 60px` (correct for the P31 character window, where it's beside the paperdoll) pinned the panel in the inventory detail column — which stacks the examine ABOVE the holders table in one scroll flow — so on scroll the stuck panel slid over the holder rows and hid who-holds-the-item. Fix = a scoped `.examine-wrap :global(.examine){ position: static; max-height: none; overflow: visible; }` override **in the inventory tab only**; the shared `ExaminePanel` and its P31 `PaperdollSlot`/`InventoryWindow` usage are untouched. Re-deployed web-only (new entry hash `start.BQTGCWPH.js`, content-type `text/javascript`), re-smoked, operator re-approved.

## Fix-forward (round 2 — code review, `2637db9`)
The post-deploy code review (`32-REVIEW.md`) found 1 BLOCKER + 2 WARN, all in `web/src/routes/inventory/+page.svelte`; all fixed web-only and redeployed:
- **CR-01 (BLOCKER — latent crash):** the holders `{#each}` keyed on `char + slot_label`, but `slotLabel` collapses every bagged copy to the literal `"Bag"` — so one character holding the same stackable item in two bags (routine P99) produced two holders with an identical `{char,"Bag"}` key → Svelte `each_key_duplicate` crashed the detail panel for that item (the same class as P31's `2e13d9e`). The smoke missed it (no such item was opened). Fixed by including the holding index in the key. **This is the headline fix — a real crash on real data.**
- **WR-01:** the shared detail-header `<img>` kept a stale `display:none` (set imperatively by `onImgError`) when switching from a broken-icon item to a valid one → a valid icon stayed hidden. `{#key selectedRollup.name}` recreates the node per selection.
- **WR-02:** holder "Last synced" rendered the raw ISO string (the misleading `00:00:00Z`); swapped to the shared `LastSyncedCell` (friendly date + freshness dot), consistent with the view/bank tables.
- The 2 INFO items (a dead `RowOrdinal`; minor aria-label phrasing) are advisory only — no change. `check` 0/0 + 359 web tests + build green; redeployed (entry `start.yZLXx6ug.js`, `text/javascript`).

## Deviations from Plan
- **Deploy-doc drift (worked around, not yet fixed in-repo):** `docs/backend-deploy.md` §7.5's web-swap guard is `test -f "$NEW/index.html"`, but this SPA build (adapter-static, SPA fallback) emits **`200.html`, NO `index.html`** — the guard failed `EXTRACT_FAILED` and aborted **before** touching the live tree (safe). Re-ran with `test -f "$NEW/200.html"`. The runbook §7.5 guard + its verify step should be updated to `200.html` (a footgun for the P33/P34 web deploys). Filed as a follow-up doc fix.
- One fix-forward smoke commit (above). No scope change.

## Rollback
- **Backend:** `cp /usr/local/bin/squirebot-server.bak /usr/local/bin/squirebot-server && systemctl restart squirebot-server` (the `.bak` is the P31 binary; schema-v13-safe since no migration ran — it simply lacks the new route).
- **Web:** `rm -rf /var/www/squirebot.bad; mv /var/www/squirebot /var/www/squirebot.bad; mv /var/www/squirebot.old /var/www/squirebot`.

## Verification (must_haves)
- ✅ `GET /api/v1/items` is live (server rebuilt + restarted) — external 401 (was 404), local 8090 401.
- ✅ The new web bundle is live (atomic swap) and `/inventory` renders the item-centric master-detail (200 + `text/javascript` entry; operator-confirmed render).
- ✅ Pre-deploy R2 backup exists (`squirebot-2026-06-18.db.gz`).
- ✅ Browser-smoke confirms the load-bearing interactions on the live build across all 5 themes (operator "approved", incl. the holder `/characters?c=` deep-link and the post-fix holders-not-covered scroll).
- ✅ No migration applied — schema stays v13.

## Self-Check: PASSED
The Inventory tab is live and the route is registered (401, not 404); the 7-point browser-smoke passed across all 5 themes (closing ITEM-01/02/03); the fix-forward commit `c5b2bc4` is in the git log; rollback recipe recorded; this SUMMARY exists.

---
*Phase: 32-inventory-tab-item-centric*
*Completed: 2026-06-18*
