# Apps Script deploy runbook

> Audience: the workbook owner (typically the guild leader). Every guildie installs the **watcher** (Windows app) — only one person needs to deploy the **Apps Script** (server-side bound to the workbook).

## What this is

The Phase 3+ Apps Script side of SquireBot lives in `apps-script/` in this repo. It powers the consolidated `view` and `bank` tabs, the daily PigParse pricing scrape, the weekly P1999 wiki summary scrape, and the `_meta.theme`-driven aesthetic.

The script is **container-bound** to the workbook (not a standalone library). One workbook = one bound script. If your guild later forks to a new workbook, repeat this whole runbook on that workbook.

## First-time setup (workbook owner)

Prerequisites:

- Node.js 20+ on your machine
- The shared SquireBot Google Sheet exists and you own it (or have edit access)
- The watcher (`SquireBot.exe`) is at v0.4.0 or newer on every guildie's machine — older watchers will refuse to write to a v=3 schema workbook

Steps:

1. **Open the workbook → Extensions → Apps Script.** This creates a new container-bound script project. Leave the script editor tab open — you'll need the URL.
2. **Copy the script ID** from the URL bar. The URL looks like `https://script.google.com/u/0/home/projects/<SCRIPT_ID>/edit`. Copy the `<SCRIPT_ID>` portion.
3. **Clone SquireBot locally** (if you haven't already) and `cd apps-script`.
4. **Create your `.clasp.json`:**
   ```bash
   cp .clasp.json.example .clasp.json
   ```
   Edit `.clasp.json` and paste your script ID into the `scriptId` field.
5. **Install + build + test:**
   ```bash
   npm install
   npm run build
   npm test
   ```
   `npm test` should report 200+ tests passing across 21+ test files.
6. **Authenticate clasp** (one-time, opens a browser):
   ```bash
   npx clasp login
   ```
7. **Push the bundle to your workbook's script:**
   ```bash
   npx clasp push
   ```
   First push asks if you want to overwrite the empty default file — say yes.
8. **Refresh the workbook tab in your browser** so Apps Script picks up the new menu. The **SquireBot** menu now appears in the menu bar.
9. **Run the migration once:** SquireBot menu → **Run Migration (v=3)** (or from the script editor, select `migrateToV3` in the function dropdown and click Run). Approve the OAuth scopes when prompted (one-time). `migrateToV3` chains v1→v2 if needed via the legacy `migrateToV2` (still wired under the menu). Verify `_meta.schema_version` is now `3` and `_meta.theme` is `minimalist`.
10. **Install the triggers:** SquireBot menu → **Install Triggers**. This creates **seven** triggers (onChange, hourly view backstop, daily PigParse refresh, weekly wiki-items refresh, weekly wiki-spells refresh, weekly wiki-gear-tier refresh, weekly cell-count watchdog). It also (idempotently) re-applies Range protection to the `_meta.bank_coin_*` cells. Re-runnable safely.
11. **(Optional) First syncs:**
    - SquireBot menu → **Refresh PigParse Now** (otherwise waits until 03:00 PT)
    - SquireBot menu → **Refresh Wiki Items Now** (otherwise waits until Sunday 04:00 PT)
    - SquireBot menu → **Rebuild Views Now** (rebuilds `view` + `bank` + `gear_check` + `spell_check` against current data)
12. **(Optional) Set bank coin:** SquireBot menu → **Set Bank Coin…** opens a sidebar with PP/GP/SP/CP inputs. Save populates the `bank` tab's COIN row at row 2. /outputfile inventory does not include coin totals — this is the manual entry point.
13. **(Optional) Set per-character info:** SquireBot menu → **Set Character Info…** opens a sidebar with class/level/race for each character. Race is required for the Iksar racial gear tier in `gear_check`.
14. **(Optional) Pick a theme:** SquireBot menu → **Set Theme…** opens a minimal modal with all 6 themes. The view + bank tabs rebuild automatically on theme change. The polished 6-tile picker lands in Phase 5.
15. **You're done.** The `view` / `bank` / `gear_check` / `spell_check` tabs will populate as guildies' watchers upload data.

### Range.protect smoke check (recommended, ~30s)

After **Install Triggers** runs, the four `_meta.bank_coin_*` cells are protected. Verify:

1. Click into `_meta` cell `B<row>` for `bank_coin_pp` and try to type a new value. Sheets should show a **protection warning prompt** ("Heads up — you're trying to edit a protected cell. Are you sure?"). Cancel.
2. Open SquireBot → **Set Bank Coin…**, type a new platinum value, click Save. The save should succeed with no protection prompt (the script-side write bypasses Range.protect).

If step 1 does not show a prompt, the protection wasn't applied — re-run **Install Triggers** (it's idempotent) and try again. If it still doesn't prompt, run `protectBankCoinCells` directly from the script editor's Run dropdown.

## Update flow

When SquireBot ships a new Apps Script version:

```bash
cd apps-script
git pull
npm ci         # update deps if package-lock changed
npm run build
npx clasp push
```

The migration is idempotent — if `_meta.schema_version` is already at the latest version, `migrateToV2` is a no-op.

## Schema-version coordination

The watcher refuses to write to a workbook whose `_meta.schema_version` exceeds its `WatcherMaxSchemaVersion` constant. **Always upgrade the watcher (`SquireBot.exe`) for every guildie BEFORE running a new schema migration.** If you push first and a guildie has an older watcher, their tray will go red with `ErrSchemaTooNew`.

| Watcher version | Max schema version it can write |
|-----------------|---------------------------------|
| v0.1.x – v0.2.x | 1                               |
| v0.3.x          | 2                               |
| v0.4.0+         | 3                               |

## Troubleshooting

- **`npx clasp push` says "user has not enabled the Apps Script API"**: visit https://script.google.com/home/usersettings and toggle "Google Apps Script API" on. One-time per Google account.
- **Migration says "could not acquire document lock within 30s"**: someone else has a script running against the workbook. Wait a minute and retry.
- **Tests fail with "vitest: command not found"**: `npm install` didn't complete. Re-run from a clean `node_modules/` (delete + reinstall).

## What CI does

The `apps-script-build` GitHub Actions workflow runs `npm ci && typecheck && build && test` on every PR that touches `apps-script/**`. It does NOT push to any workbook — that's intentional. Deploy is always manual from the workbook owner's machine. Keeps OAuth credentials out of CI.
