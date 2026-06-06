---
phase: 14-web-frontend
fixed_at: 2026-05-30T00:00:00Z
review_path: .planning/phases/14-web-frontend/14-REVIEW.md
iteration: 1
fix_scope: critical_warning
findings_in_scope: 4
fixed: 3
skipped: 1
status: all_fixed
---

# Phase 14: Code Review Fix Report

**Fixed at:** 2026-05-30
**Source review:** `.planning/phases/14-web-frontend/14-REVIEW.md`
**Iteration:** 1
**Scope:** Critical + Warning (the 5 Info findings IN-01..IN-05 are out of scope this pass — no `--all` flag)

**Summary:**
- Findings in scope (Critical + Warning): 4 (CR: 0, WR: 4)
- Fixed: 3 (WR-02, WR-03, WR-04)
- Skipped: 1 (WR-01 — already resolved before this run)
- Status: `all_fixed` (every in-scope finding is resolved; the one skip is a pre-existing fix, not a deferral)

## Build / Test Verification

Run from `web/` after all three fixes landed:

- `npm run check` (svelte-kit sync + svelte-check): **0 errors, 0 warnings** (400 files). Baseline before fixes was also 0/0.
- `npx vitest run`: **7 test files, 68 tests, all passing.** Baseline before fixes was 5 files / 60 tests; the +2 files / +8 tests are the new WR-02 and WR-04 coverage.

No regressions. The phase stack (read-only SvelteKit + Svelte 5 runes + `@tanstack/table-core` via the local adapter) is unchanged — no new dependencies introduced (`@tanstack/svelte-table` was NOT added).

## Fixed Issues

### WR-04: `getJSON` malformed-2xx body now surfaces as a branded `ApiError`

**Files modified:** `web/src/lib/api.ts`, `web/src/lib/__tests__/api.test.ts` (new)
**Commit:** `a768c8a`
**Applied fix:** Wrapped the `await res.json()` parse in `getJSON` in a try/catch that throws `new ApiError(`malformed JSON from ${path}`, res.status)`. Previously a 2xx response with a non-JSON or empty body (an interposing proxy / Cloudflare error page served with 200) rejected with a raw `SyntaxError` that escaped the fetch-only try/catch and broke the `ApiError` contract callers rely on for status classification. Added `api.test.ts` (4 tests) exercising `getJSON` through the injectable-`fetchFn` seam of `fetchView`: well-formed 2xx resolves, non-2xx throws `ApiError` with the status, transport failure throws `ApiError` status 0, and the new path — a malformed 2xx body throws an `ApiError` (not a `SyntaxError`) carrying status 200.

### WR-02: Global filter scoped to user-visible columns

**Files modified:** `web/src/lib/columns.ts`, `web/src/lib/__tests__/columns.test.ts` (new)
**Commit:** `5371cca`
**Applied fix:** Set `enableGlobalFilter: false` on the four `viewColumns` whose raw accessor value diverges from the rendered cell — `id` (numeric, cross-contaminates with price/count digit-runs), `wiki` (full wiki URL, renders as an icon with no visible text), `price` (raw number, renders `1,234pp`), and `last_synced` (raw ISO `2026-05-09T00:00:00Z`, renders `2026-05-09`). Verified against the installed `@tanstack/table-core@8.21.3` source that `getFilteredRowModel` builds the global-filter column set from `column.getCanGlobalFilter()`, which returns false when `columnDef.enableGlobalFilter === false` — so the global "Filter all columns" box now matches only the visible text columns (Char / Slot / Item) while per-column filtering is unaffected. `bankColumns` inherits the fix (it aliases `viewColumns`). Added `columns.test.ts` (4 tests) pinning the per-column `enableGlobalFilter` flags and proving the behavior through the real adapter: a search for `09T00` (only in the raw ISO) and `project1999` (only in the hidden URL) now match zero rows, while `Fungi` still matches the visible Item text.

### WR-03: One-shot initial fetch moved from `$effect` to `onMount`

**Files modified:** `web/src/routes/+page.svelte`
**Commit:** `756e720`
**Applied fix:** Imported `onMount` from `svelte` and replaced the bare `$effect(() => { void load(); })` with `onMount(() => { void load(); })`. The effect fired once only because `load()`'s synchronous body reads no reactive state before its first `await`; that is refactor-fragile (scoping the fetch by `active` or a query param would silently re-fire the whole five-endpoint parallel fetch). `onMount` makes the fire-once intent explicit and refactor-safe. The Retry handler `refetch()` already calls `load()` directly, so the effect was not needed for retry. Loading / error / empty state handling (`status`, `noCharacters`) is unchanged.

## Skipped Issues

### WR-01: Wiki-URL `href` scheme allow-list — already resolved before this run

**Files:** `web/src/lib/tooltip/composeNotes.ts`, `web/src/lib/components/cells/WikiCell.svelte`
**Status:** `already_resolved` / skipped (pre-fixed in `d14e4ab`)
**Reason:** The scheme allow-list this finding asks for already exists. `safeHttpUrl()` (an http(s)-only allow-list, `composeNotes.ts:69-72`) is applied at both sinks: `composeItemNote` runs the wiki URL through `safeHttpUrl()` before building the anchor and renders no link when it returns `''` (`composeNotes.ts:99-107`), and `WikiCell.svelte` imports and `$derived`s `safeHttpUrl(wikiUrl)`, rendering the `<a>` only `{#if safeUrl}` (`WikiCell.svelte:9,14,17`). A genuine scheme test exists — `composeNotes.test.ts:163-169` (`'drops a javascript: wiki URL entirely — no href rendered, scheme absent (WR-01)'`) plus a dedicated `safeHttpUrl` describe block. The associated IN-01 test-name fix is also already applied (the quote-breakout test is now correctly named). No re-fix performed; not duplicated.

## Out of Scope (Info — not addressed this pass)

These five Info findings are out of scope for this Critical+Warning pass (no `--all` flag) and were left untouched:

- **IN-01** — Misleading XSS test name in `composeNotes.test.ts`. (Note: this is effectively already addressed alongside the pre-existing WR-01 fix — the test is now correctly named and a real scheme test was added — but it is not re-confirmed as a fix here.)
- **IN-02** — `[data-theme]` written twice on the root element (`+layout.svelte`); harmless/idempotent.
- **IN-03** — Synthesized search-result tooltip reports `t30: 0` ("0 transactions") (`SearchResults.svelte`); cosmetic.
- **IN-04** — `createSvelteTable.svelte.ts` comment claims a reactivity re-trigger the adapter doesn't perform; comment-accuracy only, no bug.
- **IN-05** — Tooltip popover has no viewport-collision / flip handling (`ItemTooltip.svelte`); UX robustness, explicitly deferrable.

---

_Fixed: 2026-05-30_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
