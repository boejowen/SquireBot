---
quick_id: 260618-rh1
slug: fix-backend-deploy-web-swap-guard-to-200
date: 2026-06-19
status: complete
files_changed: [docs/backend-deploy.md]
---

# Quick Task 260618-rh1 Summary

Fixed the `docs/backend-deploy.md` §7.5 deploy-runbook drift discovered during the Phase 32 deploy
(2026-06-18): the web-bundle atomic-swap guard and the `npm run build` output comment both referenced
`index.html`, but the adapter-static SPA build emits `200.html` (no `index.html` — `/` is a client
redirect Caddy serves via `try_files {path} /200.html`). The guard therefore aborted with
`EXTRACT_FAILED` on a real deploy; I worked around it manually during P32 and have now corrected the
runbook so the P33/P34 web deploys won't hit it.

## Changes (docs/backend-deploy.md §7.5)
- Build-output comment: `(index.html + 200.html + _app/ + assets)` → `(200.html + robots.txt + _app/ — SPA fallback, NO index.html)`.
- The load-bearing extract guard: `test -f "$NEW/index.html"` → `test -f "$NEW/200.html"`, with an inline note that adapter-static emits `200.html`, not `index.html`.

## Verification
- §7.5 guard (line 310) now tests `test -f "$NEW/200.html"`; the build-output comment (line 295) lists `200.html`.
- The two remaining `index.html` occurrences are the NEW *explanatory* notes ("SPA fallback, NO index.html" / "NOT index.html — the `/` route is a client redirect") — documentation, not checks.
- Doc-only change; no code, no behavior change. The P33/P34 web deploys will no longer abort at the guard. Committed `17e50e7`.

## Self-Check: PASSED
The two functional `index.html` uses (the guard + the build comment) now reference `200.html`; the only `index.html` text left is the clarifying notes; SUMMARY + PLAN written.
