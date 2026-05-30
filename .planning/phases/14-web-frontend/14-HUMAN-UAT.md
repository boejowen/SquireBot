---
status: partial
phase: 14-web-frontend
source: [14-VERIFICATION.md]
started: 2026-05-30T00:00:00Z
updated: 2026-05-30T00:00:00Z
---

## Current Test

[awaiting human testing — run after the two deploy steps below are done]

> **Prerequisite (operational, not a code gap):** the website code is built + automated-verified but NOT yet live. Before these tests can pass you must:
> 1. Deploy the new `linux/amd64` server binary (with the P14 `readapi` routes) to the Hetzner VPS and restart — the box currently runs a pre-P14 binary (`/api/v1/views/*` returns 404).
> 2. Publish `web/build/` to Cloudflare Pages at `https://app.squirebot.quest`.
> Then run `/gsd-verify-work 14` and walk these items.

## Tests

### 1. Site loads + all four views render with filter/sort (WEB-01)
expected: After both deploys, opening https://app.squirebot.quest shows all four views (view, gear_check, spell_check, bank), each with a leading **sticky Char column**; every column filters and sorts; non-empty data appears (inventory/gear/spell rows from the live P12-populated SQLite).
result: [pending]

### 2. gear_check / spell_check status badges match v1 (WEB-02 presentation)
expected: On the deployed gear_check + spell_check grids, the status badges read OK/MISSING/OTHER (gear, vs Velious tiers) and KNOWN/MISSING (spell), matching what the v1 Sheet showed for the same character. (Compute parity is already automated-proven by Go table-tests; this confirms the live end-to-end presentation.)
result: [pending]

### 3. Cross-character search + "did you mean?" (WEB-03)
expected: Typing a partial/misspelled item name returns cross-character results in well under 2s, each match listing holders as `↳ <Char>: <Location>, count <n>`; a no-exact-match query shows a single clickable `Did you mean <suggestion>?` that re-runs the search when clicked.
result: [pending]

### 4. Item tooltip + wiki link + theme switching (WEB-04 + WEB-05)
expected: Hover (desktop) and tap (touch) an Item cell → a rich-HTML tooltip opens with wiki summary + price lines + quest info, dismisses on Esc/outside-tap, and its wiki link opens the correct wiki.project1999.com page in a new tab. The theme picker flips the whole site via `[data-theme]` and the choice persists across reload (velious is the default with no stored preference). Check the 5 EQ themes render their distinct aesthetic.
result: [pending]

### 5. Bank view empty-state + no fabricated coin (WEB-01 bank / ADMIN-05 stub)
expected: The bank inventory grid renders (or the per-view empty state if `is_bank_toon` is unset until the P16 backfill — an empty bank is expected, not an error); a `Coin: not yet recorded` affordance shows; NO fabricated `0pp` anywhere.
result: [pending]

## Summary

total: 5
passed: 0
issues: 0
pending: 5
skipped: 0
blocked: 0

## Gaps
