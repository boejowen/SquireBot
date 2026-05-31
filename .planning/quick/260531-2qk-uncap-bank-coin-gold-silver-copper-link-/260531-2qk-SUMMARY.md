---
quick_id: 260531-2qk
title: Uncap bank-coin gold/silver/copper + link wordmark to home
type: quick
status: complete
completed: 2026-05-31
subsystem: web + webadmin (bank-coin write surface + app chrome)
tags: [bank-coin, validation, web-frontend, svelte, security-adjacent, a11y]
dependency-graph:
  requires: [Phase 15 webadmin/coin.go, Phase 15 web/coin.ts + BankCoinForm, Phase 14 SiteShell]
  provides: [uncapped g/s/c coin entry (no 0–999 sub-unit cap), wordmark home link]
  affects: [bank-coin POST contract, BankCoinForm UX copy, header navigation]
tech-stack:
  added: none
  patterns: [server-truth validation mirrored client-side, node-only pure-helper tests, class-selector CSS reused for the anchor]
key-files:
  modified:
    - internal/backendsrv/webadmin/coin.go
    - internal/backendsrv/webadmin/coin_test.go
    - web/src/lib/coin.ts
    - web/src/lib/__tests__/coin.test.ts
    - web/src/lib/components/SiteShell.svelte
  created: none
decisions:
  - "SUBUNIT_ERROR copy is now identical to PLAT_ERROR ('Enter a whole number (0 or more).') but kept as a distinct export so each field surfaces its own message and call-sites stay stable."
  - "validateCoinField's plat/subunit branches collapse to one non-negative-integer check; the per-field error-string distinction is preserved for future divergence."
  - "Backend validCoin now loops all four denominations through a single (v < 0) guard — plat is no longer special-cased."
  - "No new test for Task 2: the node-only suite has no DOM/@testing-library; svelte-check + production build are the gate for the .svelte change (matches the plan's <done>)."
metrics:
  duration: ~6 min
  tasks: 2
  commits: 2
  files-changed: 5
---

# Quick Task 260531-2qk: Uncap bank-coin gold/silver/copper + link wordmark to home Summary

Lifted the 0–999 upper cap on bank-coin gold/silver/copper on BOTH the authoritative server validator (`webadmin/coin.go`) and the client-side UX validator (`web/coin.ts`) so the guild can record large raw-coin amounts (g/s/c now validate identically to platinum — a non-negative integer, no bound), and turned the header "SquireBot" wordmark from a plain `<span>` into a keyboard-focusable `<a href="/">` home link that looks visually identical.

## Tasks Completed

| Task | Name | Commit | Files |
| ---- | ---- | ------ | ----- |
| 1 | Uncap gold/silver/copper (frontend + backend + tests) | `85a116d` | coin.go, coin_test.go, coin.ts, coin.test.ts |
| 2 | Make the header wordmark a home link | `bcfd353` | SiteShell.svelte |

### Task 1 — Uncap g/s/c

- **`internal/backendsrv/webadmin/coin.go`** (authoritative gate): `validCoin` rewritten to reject only negatives across all four denominations (`for v in {plat,gold,silver,copper}: if v < 0 → false`). The old `v > 999` sub-unit upper bound is gone. The `>= 0` guard, the JSON-decoder integer guard, and the bank-toon gate (`store.SetCoinTx` → `ErrNotBankToon`) are all retained. The login-only gating (D-12 — no `RequireOfficer`) is untouched. Doc comments updated.
- **`web/src/lib/coin.ts`** (UX defense-in-depth): `validateCoinField`'s plat/subunit branches collapse to a single `Number.isSafeInteger` check after the existing `/^\d+$/` digits-only guard (which already excludes negatives, signs, and decimals). `SUBUNIT_ERROR` copy changed from `'Enter 0–999.'` to `'Enter a whole number (0 or more).'` (now identical to `PLAT_ERROR`); both kept as distinct exports. Platinum's code path is unchanged.
- **`internal/backendsrv/webadmin/coin_test.go`**: `TestCoinSet_RejectsOutOfRange` cases switched from `gold=1000`/`silver=1000` (now valid) to `gold=-1`/`silver=-5` (still rejected) — all four denominations now have a negative-rejection case. Added `TestCoinSet_AcceptsLargeSubunit` proving g/s/c = 5000 returns 200 and the columns actually change to 5000 (read-back, not just response assertion).
- **`web/src/lib/__tests__/coin.test.ts`**: the "0..999 inclusive" test broadened to assert 1000 / 5000 / 1000000 are valid for g/s/c; the "1000+ → error" test dropped the `'1000'` case and now covers only negative/non-integer; the `coinIsValid` out-of-range example switched from `gold:'1000'` to `gold:'-1'`; the CR-01 number-coercion case `validateCoinField('gold', 1000)` flipped from `SUBUNIT_ERROR` to `undefined`. Plat cases and the CR-01 crash-proof contract preserved.

**Both layers agree:** 5000 g/s/c is accepted on the frontend (passes `/^\d+$/` + `isSafeInteger`) and the backend (passes `v < 0`); negatives and non-integers are rejected on both; platinum is unchanged everywhere.

### Task 2 — Wordmark home link

- **`web/src/lib/components/SiteShell.svelte`**: `<span class="wordmark">SquireBot</span>` → `<a href="/" class="wordmark">SquireBot</a>`. The `.wordmark` CSS was already a class selector (not `span.wordmark`) so it targets the anchor unchanged; added `text-decoration: none` (so the anchor stays visually identical — no underline; `color: var(--accent)` was already present, overriding default link blue) and a `.wordmark:focus-visible` outline matching the existing `.admin-nav:focus-visible` pattern for keyboard accessibility. Renders identical, is clickable, and is keyboard-focusable.

## Verification

All commands run exactly as the plan's `<verify>` blocks specify. Real results:

| Command | Result |
| ------- | ------ |
| `cd web && npm run check` | PASS — 432 files, 0 errors, 0 warnings (run after both tasks) |
| `cd web && npm run test:unit -- --run` | PASS — 14 files, 172/172 tests (run after both tasks) |
| `cd web && npm run build` | PASS — adapter-static wrote site to `build/`; `web/build/index.html` confirmed present |
| `go test ./...` | PASS — all packages ok; `webadmin` ran fresh (20.4s) with the new + edited cases |
| `go build ./...` | PASS |
| `GOOS=linux GOARCH=amd64 go build ./cmd/squirebot-server` | PASS (exit 0) |

The tree is left green.

## Deviations from Plan

None affecting behavior. Two minor, in-scope refinements worth noting:

1. **Backend negative-case coverage broadened (in the spirit of the plan).** The plan said "Update/replace the old 999-cap case." Rather than only inverting the two `=1000` cases, I replaced them with `gold=-1` and `silver=-5` so every denomination (plat/gold/silver/copper) has an explicit negative-rejection case, and added a dedicated `TestCoinSet_AcceptsLargeSubunit` (g/s/c=5000, read-back-verified) for the positive uncap path. This is squarely within the task's stated intent ("a >999 g/s/c value (e.g. 5000) is now accepted ... a negative is still rejected").
2. **Sub-unit error copy unified with platinum.** Per the plan ("a '0–999' hint becomes the same wording plat uses — a whole number ≥ 0"), `SUBUNIT_ERROR` now equals `PLAT_ERROR`. Kept as a separate export (not deleted) so `validateCoinField` can still report per-field copy and existing imports/tests referencing `SUBUNIT_ERROR` keep working.

No architectural changes, no auth gates, no new dependencies installed (node-only test philosophy honored), no docs/ROADMAP commits.

## Known Stubs

None.

## Self-Check: PASSED

- Files modified (all confirmed present on disk):
  - `internal/backendsrv/webadmin/coin.go` — FOUND
  - `internal/backendsrv/webadmin/coin_test.go` — FOUND
  - `web/src/lib/coin.ts` — FOUND
  - `web/src/lib/__tests__/coin.test.ts` — FOUND
  - `web/src/lib/components/SiteShell.svelte` — FOUND
- Commits (confirmed in `git log`):
  - `85a116d` fix(260531-2qk): uncap bank-coin gold/silver/copper (frontend + backend) — FOUND
  - `bcfd353` feat(260531-2qk): link header wordmark to home — FOUND
- Neither commit introduced file deletions (`git diff --diff-filter=D HEAD~1 HEAD` empty for both).
