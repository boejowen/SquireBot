---
layout: default
---

# Developer notes

Stack overview, how to build, how to deploy Apps Script.

## Canonical docs in this repo

- [Stack + architecture + conventions](https://github.com/boejowen/SquireBot/blob/main/CLAUDE.md)
- [OAuth Cloud setup runbook](https://github.com/boejowen/SquireBot/blob/main/docs/oauth-setup.md)
- [Build + sideload watcher](https://github.com/boejowen/SquireBot/blob/main/docs/build-and-install.md)
- [Apps Script clasp deploy](https://github.com/boejowen/SquireBot/blob/main/docs/apps-script-deploy.md)
- [Eviction runbook (DOC-02)](https://github.com/boejowen/SquireBot/blob/main/docs/eviction-runbook.md)

## Phase summaries

- [Phase 1 — End-to-End Thin Slice](https://github.com/boejowen/SquireBot/tree/main/.planning/phases/01-end-to-end-thin-slice)
- [Phase 2 — Watcher Robustness](https://github.com/boejowen/SquireBot/tree/main/.planning/phases/02-watcher-robustness-schema-lock)
- [Phase 3 — Apps Script Enrichment](https://github.com/boejowen/SquireBot/tree/main/.planning/phases/03-apps-script-enrichment-foundation)
- [Phase 4 — Differentiator Features](https://github.com/boejowen/SquireBot/tree/main/.planning/phases/04-differentiator-features)
- [Phase 5 — Search + Onboarding + Polish](https://github.com/boejowen/SquireBot/tree/main/.planning/phases/05-search-onboarding-privacy-polish)

## Quick orientation

SquireBot is two halves: a Go watcher that runs on each guildie's Windows PC, and an Apps Script workbook that runs as TypeScript bundled via `clasp + esbuild`. Tab structure, schema-version rules, and the watcher write contract are documented in [CLAUDE.md](https://github.com/boejowen/SquireBot/blob/main/CLAUDE.md). The 200-tab-limit reasoning that locks views as consolidated mega-tabs is in the same doc.

For local development:

- Watcher: `go build ./cmd/squirebot` from the repo root. Live-debug with VS Code's Go extension; the wizard flow runs against a sandbox workbook.
- Apps Script: `cd apps-script && npm install && npm run build` produces `dist/Code.js`. `npm test` runs vitest. Deploy via `npx clasp push` from a workbook-owner machine (see the deploy runbook above).

## Legacy README notes

Earlier versions of `README.md` shipped a long-form tray-menu reference plus filesystem layout. The full content is preserved in git history; the canonical replacement is `CLAUDE.md` for stack/architecture details and `docs/build-and-install.md` for the tray-menu and uninstall flow. The shrunk README is intentional per CONTEXT.md D-12.
