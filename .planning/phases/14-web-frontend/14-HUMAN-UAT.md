---
status: partial
phase: 14-web-frontend
source: [14-VERIFICATION.md]
started: 2026-05-30T00:00:00Z
updated: 2026-05-30T18:40:00Z
---

## Current Test

[blocked — the site is not yet served over HTTPS; see "Deploy Blockers (diagnosed)" below. Re-run `/gsd-verify-work 14` after both deploy steps are done.]

## Tests

### 1. Site loads + all four views render with filter/sort (WEB-01)
expected: After both deploys, opening https://app.squirebot.quest shows all four views (view, gear_check, spell_check, bank), each with a leading **sticky Char column**; every column filters and sorts; non-empty data appears (inventory/gear/spell rows from the live P12-populated SQLite).
result: blocked
blocked_by: deploy
reason: "User hit ERR_SSL_VERSION_OR_CIPHER_MISMATCH. Diagnosed: app.squirebot.quest is a CNAME to pixie.porkbun.com (Porkbun parking, 44.227.76.166/.65.245) — no TLS cert, frontend never published. Not a P14 code defect."

### 2. gear_check / spell_check status badges match v1 (WEB-02 presentation)
expected: On the deployed gear_check + spell_check grids, status badges read OK/MISSING/OTHER (gear) and KNOWN/MISSING (spell), matching the v1 Sheet for the same character.
result: blocked
blocked_by: deploy
reason: "Depends on Test 1 — the site must load first. Compute parity is already automated-proven by Go table-tests; this is the live-render confirmation."

### 3. Cross-character search + "did you mean?" (WEB-03)
expected: Typing a partial/misspelled item name returns cross-character results well under 2s, each listing holders as `↳ <Char>: <Location>, count <n>`; a no-exact-match query shows a clickable `Did you mean <suggestion>?` that re-runs the search.
result: blocked
blocked_by: deploy
reason: "Depends on Test 1 (loaded site + live data). Search logic is in the 68/68 vitest suite."

### 4. Item tooltip + wiki link + theme switching (WEB-04 + WEB-05)
expected: Hover/tap an Item cell → rich-HTML tooltip (summary + price + quest), dismiss on Esc/outside-tap, wiki link opens the correct wiki.project1999.com page in a new tab; theme picker flips the whole site via `[data-theme]` and persists across reload (velious default).
result: blocked
blocked_by: deploy
reason: "Depends on Test 1. Tooltip escaping/scheme-validation + theme registry are automated-proven + security-verified (19/19 threats closed)."

### 5. Bank view empty-state + no fabricated coin (WEB-01 bank / ADMIN-05 stub)
expected: The bank grid renders (or the empty state if `is_bank_toon` is unset until the P16 backfill — empty is expected, not an error); a `Coin: not yet recorded` affordance shows; NO fabricated `0pp`.
result: blocked
blocked_by: deploy
reason: "Depends on Test 1."

## Summary

total: 5
passed: 0
issues: 0
pending: 0
skipped: 0
blocked: 5

## Deploy Blockers (diagnosed 2026-05-30)

UAT cannot proceed until the website is actually served. Two operational gaps, both
in the maintainer's accounts (NOT P14 code defects — the build is correct + tested):

1. **Frontend not published (causes the SSL error).** `app.squirebot.quest` is a CNAME to
   `pixie.porkbun.com` (Porkbun parking) → no TLS cert → `ERR_SSL_VERSION_OR_CIPHER_MISMATCH`.
   FIX: publish `web/build/` to a static host (plan locked Cloudflare Pages) and repoint the
   `app` DNS record (at Porkbun) to the Pages target, replacing the parking CNAME. Wait for the
   host's cert to provision.

2. **Backend running the stale pre-P14 binary.** `api.squirebot.quest` (5.78.232.85, Caddy)
   TLS is healthy but `GET /` → 404 and `/api/v1/views/*` → 404 (no readapi routes).
   FIX: rebuild the `linux/amd64` server binary (with the P14 readapi routes) and deploy +
   restart on the Hetzner VPS. Also verify Caddy does not duplicate `Access-Control-Allow-Origin`
   (deploy-gate DG-1 from 14-SECURITY.md).

## Gaps

[none — all blocks are deploy prerequisites, not code issues]
