# Apps Script deploy runbook

> Audience: the workbook owner (typically the guild leader). Every guildie installs the **watcher** (Windows app) — only one person needs to deploy the **Apps Script** (server-side bound to the workbook).

## What this is

The Phase 3+ Apps Script side of SquireBot lives in `apps-script/` in this repo. It powers the consolidated `view` and `bank` tabs, the daily PigParse pricing scrape, the weekly P1999 wiki summary scrape, and the `_meta.theme`-driven aesthetic.

The script is **container-bound** to the workbook (not a standalone library). One workbook = one bound script. If your guild later forks to a new workbook, repeat this whole runbook on that workbook.

## First-time setup (workbook owner)

Prerequisites:

- Node.js 20+ on your machine
- The shared SquireBot Google Sheet exists and you own it (or have edit access)
- The watcher (`SquireBot.exe`) is at v0.3.0 or newer on every guildie's machine — older watchers will refuse to write to a v=2 schema workbook

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
   `npm test` should report 16+ tests passing.
6. **Authenticate clasp** (one-time, opens a browser):
   ```bash
   npx clasp login
   ```
7. **Push the bundle to your workbook's script:**
   ```bash
   npx clasp push
   ```
   First push asks if you want to overwrite the empty default file — say yes.
8. **Run the migration:** open the workbook → SquireBot menu → **Install Triggers** (lands in plan 03-04). Until plan 03-04 ships, run the migration manually:
   - Open the script editor (Extensions → Apps Script).
   - In the function dropdown, select `migrateToV2`. Click Run.
   - Approve the OAuth scopes when prompted (one-time).
   - Verify `_meta.schema_version` is now `2` and `_meta.theme` is `minimalist`.
9. **You're done.** The view + bank tabs will populate after plan 03-04 lands.

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
| v0.3.0+         | 2                               |

## Troubleshooting

- **`npx clasp push` says "user has not enabled the Apps Script API"**: visit https://script.google.com/home/usersettings and toggle "Google Apps Script API" on. One-time per Google account.
- **Migration says "could not acquire document lock within 30s"**: someone else has a script running against the workbook. Wait a minute and retry.
- **Tests fail with "vitest: command not found"**: `npm install` didn't complete. Re-run from a clean `node_modules/` (delete + reinstall).

## What CI does

The `apps-script-build` GitHub Actions workflow runs `npm ci && typecheck && build && test` on every PR that touches `apps-script/**`. It does NOT push to any workbook — that's intentional. Deploy is always manual from the workbook owner's machine. Keeps OAuth credentials out of CI.
