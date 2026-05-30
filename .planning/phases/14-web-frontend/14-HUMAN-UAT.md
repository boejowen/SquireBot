---
status: partial
phase: 14-web-frontend
source: [14-VERIFICATION.md]
started: 2026-05-30T00:00:00Z
updated: 2026-05-30T20:05:00Z
---

## Current Test

[UAT walk done 2026-05-30 over the demo data (`Demoknight`, SHD L60). Outcome:
**Test 1 PASS** (views render, sticky Char, filter/sort). **Test 2 PASS** (gear OK/OTHER/MISSING + spell KNOWN/MISSING badges over live data). **Test 3 = not a bug** — `didYouMean` is whole-name, edit-distance ≤2 (ported verbatim from v1, the WEB-03 oracle); the test query `frozn` is ~15 edits from any full item name so it correctly yields no suggestion. A proper near-miss (e.g. `Rusty Helmet` → `Rusty Helm`, 2 edits) does suggest. Working as designed. **Test 4 — WAS AN ISSUE, NOW FIXED** (`f12ad9b`, deployed): item rows had no wiki link because the URL came from the empty `item_master.wiki_url`; now the link is derived from the item name at render time (`wikiUrlFor`, per FEATURES.md/WEB-04) so every item links. **Test 5** (bank empty-state) not yet eyeballed by the user. Hard-refresh https://squirebot.quest to re-confirm 4 + walk 5.]

## Deploy status: LIVE (2026-05-30) — both gaps RESOLVED + verified end-to-end

- **Frontend:** served by **Caddy on the VPS at the apex `https://squirebot.quest`** (`file_server /var/www/squirebot`), valid Let's Encrypt cert (issued 2026-05-30, full ISRG chain), `noindex` present. The earlier `ERR_SSL_VERSION_OR_CIPHER_MISMATCH` is gone — the apex DNS was repointed at Porkbun (parking ALIAS → `A 5.78.232.85`).
- **Backend:** the new P14 binary is live (`/usr/local/bin/squirebot-server`, systemd `active`); `/api/v1/meta` + `/api/v1/views/*` return **200** (were 404 on the stale binary); CORS = exactly-one `https://squirebot.quest`, OPTIONS preflight 204 (DG-1 clean — Caddy adds no CORS header). Rollback kept: `/usr/local/bin/squirebot-server.bak` + `/etc/caddy/Caddyfile.bak`.
- **Remaining (operational, not a P14 gap):** per-character views populate as guildies' re-targeted (P13) watchers auto-update and upload.

## Tests

### 1. Site loads + all four views render with filter/sort (WEB-01)
expected: https://squirebot.quest shows all four views (view, gear_check, spell_check, bank), each with a leading **sticky Char column**; every column filters and sorts. (Rows are empty until watcher uploads — confirm the grid chrome + empty state now; re-confirm with data once uploads land.)
result: [pending]

### 2. gear_check / spell_check status badges match v1 (WEB-02 presentation)
expected: On the deployed gear_check + spell_check grids, status badges read OK/MISSING/OTHER (gear) and KNOWN/MISSING (spell), matching the v1 Sheet for the same character. (Needs a character's data — pending watcher uploads.)
result: [pending]

### 3. Cross-character search + "did you mean?" (WEB-03)
expected: Typing a partial/misspelled item name returns cross-character results well under 2s, each listing holders as `↳ <Char>: <Location>, count <n>`; a no-exact-match query shows a clickable `Did you mean <suggestion>?` that re-runs the search. (Needs data to search — pending watcher uploads.)
result: [pending]

### 4. Item tooltip + wiki link + theme switching (WEB-04 + WEB-05)
expected: Hover/tap an Item cell → rich-HTML tooltip (summary + price + quest), dismiss on Esc/outside-tap, wiki link opens the correct wiki.project1999.com page in a new tab; theme picker flips the whole site via `[data-theme]` and persists across reload (velious default). (Theme switching is testable now; the tooltip needs an item row — pending uploads.)
result: [pending]

### 5. Bank view empty-state + no fabricated coin (WEB-01 bank / ADMIN-05 stub)
expected: The bank grid renders (or the empty state if `is_bank_toon` is unset until the P16 backfill — empty is expected, not an error); a `Coin: not yet recorded` affordance shows; NO fabricated `0pp`. (Testable now — should show the empty state cleanly.)
result: [pending]

## Summary

total: 5
passed: 0
issues: 0
pending: 5
skipped: 0
blocked: 0

## Gaps

[none — deploy blockers resolved; remaining empty-data is an operational dependency on guildie watcher uploads, not a code issue]
