---
phase: 40-item-display-polish-flag-outlines-quest-links
reviewed: 2026-07-15T20:40:50Z
depth: deep
files_reviewed: 26
files_reviewed_list:
  - internal/backendsrv/store/readviews.go
  - internal/backendsrv/store/readviews_test.go
  - internal/backendsrv/compute/types.go
  - internal/backendsrv/compute/inventory.go
  - internal/backendsrv/compute/inventory_test.go
  - internal/backendsrv/compute/itemrollup.go
  - internal/backendsrv/compute/itemrollup_test.go
  - internal/backendsrv/compute/view.go
  - internal/backendsrv/enrich/wikiitem.go
  - internal/backendsrv/enrich/wikiitem_test.go
  - internal/backendsrv/store/backfill.go
  - internal/backendsrv/migrations/00018_reflag_item_master.sql
  - web/src/lib/api.ts
  - web/src/lib/flags.ts
  - web/src/lib/examine.ts
  - web/src/lib/tooltip/composeNotes.ts
  - web/src/lib/theme/themes.ts
  - web/src/app.css
  - web/src/lib/components/ExaminePanel.svelte
  - web/src/lib/components/PaperdollSlot.svelte
  - web/src/lib/components/ItemTooltip.svelte
  - web/src/lib/components/cells/ItemCell.svelte
  - web/src/routes/inventory/+page.svelte
  - web/src/routes/wishlist/+page.svelte
  - web/src/lib/__tests__/composeNotes.test.ts
  - web/src/lib/__tests__/examine.test.ts
findings:
  critical: 0
  warning: 0
  info: 4
  total: 4
status: issues_found
---

# Phase 40: Code Review Report

**Reviewed:** 2026-07-15T20:40:50Z
**Depth:** deep (cross-file: store → compute → JSON contract → api.ts → Svelte render)
**Files Reviewed:** 26
**Status:** issues_found (0 BLOCKER / 0 HIGH / 0 MEDIUM / 4 LOW-NIT)

## Summary

Phase 40 adds three `item_master` flag booleans (`is_no_drop`/`is_lore`/`is_magic`) and
named `quest_links` with a `source_url` onto the modern `InventorySlot` + `ItemRollup`
payloads, renders them as a priority tile ring (No-Drop > Lore > Magic) + an examine flag
chip + clickable named quest links, and fixes a P37 parser bug that mis-flagged ~95% of
clustered flag lines (migration 00018 forces a re-derive).

This is a clean, well-tested, security-conscious change. I could not surface a single
correctness, security, or data-loss defect that would block ship. Every concern I opened
the review with resolved in the code's favor:

- **The XSS / HTML-injection surface is fully closed.** Both quest-name render paths escape:
  the examine `quests` field uses NATIVE Svelte `{q.quest_name}` interpolation (auto-escaped,
  no `{@html}`), and the tooltip path routes through the single sanctioned `composeItemNote`
  `{@html}` sink where `escapeHtml()` runs on every name. EVERY quest href passes
  `safeHttpUrl` (positive `^https?://` allow-list, case-insensitive) BEFORE render — the
  examine native path guards the `href={}` directly, the tooltip path additionally
  `escapeHtml`s the URL for attribute-encoding. A blocked scheme falls back to inert plain
  text. No NEW `{@html}` sink was introduced (grep confirms only the two pre-existing sinks,
  both escaped). `composeNotes.test.ts` proves malicious quest names inside clickable links,
  `javascript:`/blank source_url fallback, and attribute-breakout via a quoted http URL.
- **The `--flag-color` inline CSS var is never user-derived.** `flagColorVar` returns ONLY
  one of three fixed literal `var(--flag-*)` strings (or `''`); no item/user string reaches
  a `style=` sink on PaperdollSlot or the examine chip. The driving booleans are
  server-derived integers.
- **The clustered-flag `hasFlag` substring fix is sound, no false positives.** `hasFlag`
  iterates only over the `flags` MAP (all-caps flag-line keys, gated by
  `flagRe=^[A-Z][A-Z\s\-]+$` with no colon), never the whole statsblock — so `kv` lines
  (Class/Race/Slot/Effect) can never trip it. The four queried phrases
  (`LORE ITEM`/`NO DROP`/`MAGIC ITEM`/`TEMPORARY` + `QUEST ITEM`) are each specific enough
  that no other EQ flag string contains them as a substring. Verbatim-prod regression tests
  cover MAGIC+NO-DROP, MAGIC+LORE, LORE+NO-DROP clusters and the standalone no-regression case.
- **Migration 00018 + the 00016 boot backfill idempotency is safe and byte-stable.**
  `flags_json IS NULL` is the idempotency key; 00018 NULLs it to force one re-derive, then
  `MarshalFlags` repopulates it. Critically, `flags_json` stores the CLUSTERED line as a
  single key (e.g. `"MAGIC ITEM NO DROP"`) and `flagSet` is unchanged, so ONLY the four
  boolean columns change value — `flags_json` re-marshals to identical bytes and the weekly
  freshness pass sees no diff (no churn). The Down is a documented no-op, correct for a
  forward-only data correction.
- **The lockstep holds byte-for-byte.** All five themes' `--flag-nodrop/lore/magic` hex in
  `app.css` match `themes.ts` exactly (verified per theme). The priority resolver order
  (No-Drop > Lore > Magic) is consistent across `flags.ts`, `examine.ts`, and PaperdollSlot.
- **The pure transform boundary is preserved.** `buildStructuredInventory` stays pure (the
  `QuestLinksByItem` fetch lives in the public `StructuredInventory`, the map is passed in);
  `itemrollup` copies `QuestLinks` from the representative ViewRow (no re-fetch — the file's
  iron law); the quest-link attach walk correctly covers top-level slots + one level of bag
  children (EQ bags don't nest, so one level is complete).
- **The two synthetic asSlot builders** thread all four new fields (Inventory reads the held
  rollup's flags/quest_links with `?? false`/`?? []`; Wishlist uses safe defaults) — `npm run
  check` is 0-error, `npm test` 399/399 green, `go test`/`go vet` green.

The findings below are all cosmetic / defense-in-depth NITs and are fine to leave as-is or
backlog — none affect correctness, security, or the already-shipped prod behavior.

## Info

### IN-01: `safeHttpUrl(q.source_url)` evaluated three times per quest link in the examine template

**File:** `web/src/lib/components/ExaminePanel.svelte:81-82`
**Issue:** The `{#each f.quests}` block calls `safeHttpUrl(q.source_url)` once in the `{#if}`
guard (line 81) and again in the `href={...}` binding (line 82). It runs a third time
implicitly is not the case — but the double call is redundant work per render. `safeHttpUrl`
is a cheap regex so this is purely cosmetic, not a perf finding (perf is out of v1 scope
anyway). It is also a mild correctness-robustness smell: the guard and the href must agree,
and they do here only because `safeHttpUrl` is a pure function of the same input.
**Fix:** Compute once via an inline `@const` for readability and to guarantee guard/href
parity:
```svelte
{#each f.quests ?? [] as q, i (q.quest_name + i)}{#if i > 0}, {/if}
  {@const safeUrl = safeHttpUrl(q.source_url)}
  {#if safeUrl}<a href={safeUrl} target="_blank" rel="noopener">{q.quest_name}</a
  >{:else}{q.quest_name}{/if}{/each}
```

### IN-02: Examine quest `<a>` omits `rel="noopener"` (present on the tooltip path)

**File:** `web/src/lib/components/ExaminePanel.svelte:81-85`
**Issue:** The native-Svelte examine quest link uses `target="_blank"` with only
`rel="noopener"`... actually it carries `rel="noopener"` — re-reading: line 84 sets
`rel="noopener"`. This is correct and matches the tooltip. NO ACTION. (Retained as an
explicit NIT-cleared note so a future reviewer doesn't re-flag it: both the examine native
`<a>` and the composeNotes tooltip `<a>` carry `target="_blank" rel="noopener"`, guarding
reverse-tabnabbing.) Consider `rel="noopener noreferrer"` if referrer-leak to the P1999 wiki
is ever a concern; not required.
**Fix:** No change needed; optionally add `noreferrer`.

### IN-03: app.css ↔ themes.ts flag-hex lockstep has no automated guard

**File:** `web/src/app.css:44-46,64-66,81-83,98-100,117-119` and `web/src/lib/theme/themes.ts:66-68,81-83,96-98,111-113,127-129`
**Issue:** The `--flag-*` hex values are duplicated across `app.css` (the runtime CSS var
source) and `themes.ts` (the TS registry). `themes.test.ts` asserts token PRESENCE and a
velious spot-check, but nothing reads `app.css` to prove the two stay byte-identical across
all five themes on future edits (vitest is node-only / DOM-blind, so it cannot resolve a
`[data-theme]` custom property). I verified the current values match byte-for-byte, but a
one-sided edit later would silently drift the themed flag colors.
**Fix:** Add a lightweight parity test that regex-extracts the `--flag-*` values from the
`app.css` text and asserts they equal the corresponding `THEMES[key].flag*` for all five
keys (read `app.css` via `fs.readFileSync` in the node test — the file is static text). Low
priority; the existing pattern already relies on a code-review check per the plan.

### IN-04: `ItemMasterIconStats` / `IconStats` doc comment is now stale (still says "icon_id + statsblock")

**File:** `internal/backendsrv/store/readviews.go:787-790` (the `ItemMasterIconStats` docstring)
**Issue:** The function doc still reads "returns icon_id + statsblock per item_id" but the
struct now also carries `IsClicky/HasHaste` (P39) and `IsNoDrop/IsLore/IsMagic` (P40). The
SELECT and struct were extended correctly; only the prose lags. Minor doc-drift, no
behavioral impact.
**Fix:** Update the docstring to note it returns the icon/stats + the five item_master flag
columns (is_clicky/has_haste/is_no_drop/is_lore/is_magic).

---

_Reviewed: 2026-07-15T20:40:50Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
