---
phase: 14-web-frontend
reviewed: 2026-05-30T00:00:00Z
depth: standard
files_reviewed: 47
files_reviewed_list:
  - cmd/squirebot-server/main.go
  - internal/backendsrv/compute/types.go
  - internal/backendsrv/compute/view.go
  - internal/backendsrv/compute/bank.go
  - internal/backendsrv/compute/gearcheck.go
  - internal/backendsrv/compute/spellcheck.go
  - internal/backendsrv/compute/eqconst.go
  - internal/backendsrv/compute/view_test.go
  - internal/backendsrv/compute/bank_test.go
  - internal/backendsrv/compute/gearcheck_test.go
  - internal/backendsrv/compute/spellcheck_test.go
  - internal/backendsrv/compute/pickprice_internal_test.go
  - internal/backendsrv/compute/fixtures_test.go
  - internal/backendsrv/readapi/views.go
  - internal/backendsrv/readapi/meta.go
  - internal/backendsrv/readapi/cors.go
  - internal/backendsrv/readapi/readapi_test.go
  - internal/backendsrv/store/readviews.go
  - internal/backendsrv/store/readviews_test.go
  - web/src/lib/api.ts
  - web/src/lib/columns.ts
  - web/src/lib/search/searchIndex.ts
  - web/src/lib/tooltip/composeNotes.ts
  - web/src/lib/theme/themes.ts
  - web/src/lib/table/createSvelteTable.svelte.ts
  - web/src/lib/table/createSvelteTable.ts
  - web/src/lib/table/FlexRender.svelte
  - web/src/lib/components/DataGrid.svelte
  - web/src/lib/components/ItemTooltip.svelte
  - web/src/lib/components/SearchBox.svelte
  - web/src/lib/components/SearchResults.svelte
  - web/src/lib/components/SiteShell.svelte
  - web/src/lib/components/StateBlock.svelte
  - web/src/lib/components/StatusCell.svelte
  - web/src/lib/components/StatusLegend.svelte
  - web/src/lib/components/ThemePicker.svelte
  - web/src/lib/components/cells/ItemCell.svelte
  - web/src/lib/components/cells/LastSyncedCell.svelte
  - web/src/lib/components/cells/PriceCell.svelte
  - web/src/lib/components/cells/RecommendedCell.svelte
  - web/src/lib/components/cells/WikiCell.svelte
  - web/src/routes/+page.svelte
  - web/src/routes/+page.ts
  - web/src/routes/+layout.svelte
  - web/src/routes/+layout.ts
  - web/src/lib/__tests__/composeNotes.test.ts
  - web/src/lib/__tests__/searchIndex.test.ts
  - web/src/lib/__tests__/themeApply.test.ts
  - web/src/lib/__tests__/tableAdapter.test.ts
findings:
  critical: 0
  warning: 4
  info: 5
  total: 9
status: issues_found
---

# Phase 14: Code Review Report

**Reviewed:** 2026-05-30
**Depth:** standard
**Files Reviewed:** 47 (Go read API: compute / readapi / store; SvelteKit frontend: lib + routes + tests)
**Status:** issues_found

## Summary

Phase 14 is high-quality, defensively-commented work. The four designated high-risk areas hold up well under adversarial reading:

- **XSS / `{@html}` sink:** `{@html}` appears in exactly ONE place app-wide (`ItemTooltip.svelte:106`), fed only by `composeItemNote`. Every dynamic *text/attribute* value (item/quest names, wiki summary, price numerics, and the href's HTML metacharacters) is run through `escapeHtml`, and the vitest suite proves attribute-breakout via `"` is neutralized. Every other user/wiki string in the app flows through Svelte `{}` auto-escaping. No stored/reflected XSS via text or attribute injection was found.
- **CORS (`cors.go`):** echoes the exact locked origin, never `*`, never sets `Access-Control-Allow-Credentials`, sets `Vary: Origin`, and short-circuits OPTIONS with a bodiless 204. Tests assert all of this including the never-wildcard property.
- **SQL (`readviews.go`):** every query is a literal SELECT with `?` placeholders; the only dynamic SQL is a fixed-string `bankOnly` branch (two complete query literals, no value interpolation). No injection surface.
- **Parity (`compute/*.go`):** gear OK→OTHER→MISSING branch ordering, spell KNOWN/MISSING + level-gating, the sort orders, nil-price/nil-coin handling, and the TEXT-`direction` `pickPrice` all match the v1 builders. I specifically verified `pickPrice` matching only `"0"`/`"1"` (not `"2"`) is **exact v1 parity** — the v1 `buildView.ts:259-265` also matched only `0` then `1`, and the daily job filters to `T==0` before upsert anyway. Not a bug.
- **searchIndex carried fixes:** the 999.28 empty-query guard (`didYouMean` first line + `searchRows` guard) and the 999.30 whole-string Levenshtein correction are both present, correct, and have real (non-skipped) assertions.

The findings below are a defense-in-depth gap whose live exploitability is currently blocked by an upstream invariant (WR-01), two robustness/UX issues in the grid and fetch flow (WR-02, WR-03), one error-handling gap (WR-04), and five minor quality items. **No Critical issues.**

## Warnings

### WR-01: Wiki-URL `href` scheme is not validated — `escapeHtml` does not neutralize `javascript:`

**Files:** `web/src/lib/tooltip/composeNotes.ts:83-87`, `web/src/lib/components/cells/WikiCell.svelte:13-14`
**Issue:** Both the tooltip's wiki anchor and the `Wiki` column cell place a server-supplied URL directly into an `href`:

```ts
// composeNotes.ts
`<a class="tooltip-wiki-link" href="${escapeHtml(wikiUrl)}" target="_blank" rel="noopener">wiki</a>`
```
```svelte
<!-- WikiCell.svelte -->
<a class="wiki-link" href={wikiUrl} target="_blank" rel="noopener">
```

`escapeHtml` only replaces `& < > " '`. A URL like `javascript:alert(document.domain)` contains none of those characters, so it passes through intact and becomes a live `href` — clicking the link executes script in the app origin. Svelte's `{}` attribute binding in `WikiCell` likewise escapes HTML metacharacters but does **not** sanitize the URL scheme. This is precisely the threat the `composeNotes.ts` header claims to cover ("the URL scheme of the wiki href") — but the code does not actually defend the scheme, only the surrounding HTML.

The reason this is a **Warning and not a Blocker:** the only producer of `item_master.wiki_url` is `enrich.wikiURLFor`, which hardcodes `"https://wiki.project1999.com/" + pageNameToSlug(...)` (`internal/backendsrv/enrich/wikiitem.go:145-148`). There is no path in the current pipeline that can write a `javascript:`/`data:` URL into the column, so the gap is not exploitable today. But it is a latent output-encoding defect: the defense is asserted in the header and the test name ("escapes a javascript: wiki URL inside the href", `composeNotes.test.ts:156`) yet **no test actually feeds a `javascript:` scheme** — the test only checks a `"`-breakout payload. Any future change that lets a wiki redirect, an admin form (P15 ADMIN-05), or a different enrichment source set `wiki_url` would silently open a stored-XSS hole.

**Fix:** Add a scheme allow-list at the single choke point and reuse it in both sinks. Example:

```ts
// composeNotes.ts
/** Allow only http(s) URLs; anything else (javascript:, data:, vbscript:) -> ''. */
export function safeHttpUrl(u: string): string {
  try {
    const parsed = new URL(u, 'https://wiki.project1999.com/');
    return parsed.protocol === 'https:' || parsed.protocol === 'http:' ? parsed.href : '';
  } catch {
    return '';
  }
}
// then: const href = safeHttpUrl(wikiUrl); if (href) { ...escapeHtml(href)... }
```
And in `WikiCell.svelte`: `let safeUrl = $derived(safeHttpUrl(wikiUrl));` rendered only `{#if safeUrl}`. Then add a real test asserting `composeItemNote('Item', 'javascript:alert(1)', ...)` produces no `href="javascript:` substring.

### WR-02: Global filter matches hidden/raw column values, producing confusing "phantom" matches

**File:** `web/src/lib/components/DataGrid.svelte:83` (`globalFilterFn: 'includesString'`), `web/src/lib/columns.ts:87-93`
**Issue:** `globalFilterFn: 'includesString'` runs across the *accessor* value of every column, including columns the user cannot see/filter individually. The `last_synced` column carries the raw ISO string (`"2026-05-09T00:00:00Z"`) even though the cell renders a friendly `YYYY-MM-DD` date and sets `enableColumnFilter: false`. So typing `2026` or `09T00` in the global filter matches rows by a substring of a value the user never sees rendered that way — the row appears to match "nothing." Similarly the `id`/`price` numeric columns coerce to strings, so a search for a price digit-run can match item IDs. This is not a crash, but it degrades the core search/filter UX (a stated WEB-01 deliverable) and will read as a bug to guildies.
**Fix:** Constrain the global filter to the user-visible text columns, e.g. set `enableGlobalFilter: false` on `id`, `price`, `last_synced`, `wiki` in `columns.ts`, or supply a custom `globalFilterFn` that only inspects the string-typed display columns. At minimum, exclude `last_synced` from the global filter so the rendered date and the filtered value agree.

### WR-03: `load()` is driven by a bare `$effect`, coupling data fetch to the reactive graph

**File:** `web/src/routes/+page.svelte:109-111` (and `load()` at 82-102)
**Issue:** The initial fetch is wired as `$effect(() => { void load(); })`. It happens to fire exactly once today because `load()`'s synchronous body reads no reactive state before the first `await`. But this is fragile: `$effect` re-runs whenever any signal read *synchronously* in its body changes, and `load()` is one refactor away (e.g. reading `active` or a query param to scope the fetch) from silently re-firing the full five-endpoint parallel fetch on every interaction. `onMount` is the correct primitive for a fire-once load; `$effect` advertises "re-run me on dependency change," which is not the intent.
**Fix:** Use lifecycle, not an effect, for the one-shot load:

```ts
import { onMount } from 'svelte';
onMount(() => { void load(); });
```
`refetch()` (the Retry handler) already calls `load()` directly, so the effect is not needed for retry.

### WR-04: `getJSON` assumes every 2xx body is valid JSON of shape `T` — a malformed/empty body throws an unclassified error

**File:** `web/src/lib/api.ts:122-125`
**Issue:** On a 2xx response the code does `return (await res.json()) as T` with no guard. If the server (or an interposing proxy/Cloudflare error page) returns `200` with a non-JSON or empty body, `res.json()` rejects with a raw `SyntaxError` that is **not** an `ApiError`. That error escapes the `getJSON` try/catch (which only wraps the `fetch` call, not the `.json()` parse), propagates as an unbranded exception, and is swallowed by the bare `catch {}` in `+page.svelte:98` — the user lands on the generic error state, but any caller that distinguishes `ApiError.status` (e.g. a future 401 handler in P15) loses the classification. The `as T` cast also means a well-formed-but-wrong-shape payload is accepted silently.
**Fix:** Wrap the parse and normalize to `ApiError`:

```ts
if (!res.ok) throw new ApiError(`unexpected ${res.status} fetching ${path}`, res.status);
try {
  return (await res.json()) as T;
} catch {
  throw new ApiError(`malformed JSON from ${path}`, res.status);
}
```

## Info

### IN-01: Misleading test name overstates the XSS coverage

**File:** `web/src/lib/__tests__/composeNotes.test.ts:156-161`
**Issue:** The test is named `'escapes a javascript: wiki URL inside the href'` but its payload is `https://x/"><script>alert(1)</script>` — an attribute-breakout test, not a scheme test. It never exercises a `javascript:`/`data:` scheme, so the suite gives false confidence that scheme-based XSS is covered (see WR-01).
**Fix:** Rename to reflect what it tests (e.g. `'escapes a quote-breakout wiki URL inside the href'`) and add a separate, genuinely scheme-focused test once WR-01's `safeHttpUrl` lands.

### IN-02: `[data-theme]` is written twice on the root element

**File:** `web/src/routes/+layout.svelte:23-30`
**Issue:** The root `<div>` sets `data-theme={theme}` reactively AND the `$effect` calls `applyTheme(theme, rootEl)`, which also writes `rootEl.dataset.theme`. Two writers of the same attribute with the same value — harmless and idempotent, but redundant and a maintenance trap (a future edit to one path can desync them). The `$effect`'s real job is the localStorage persist + the resolve-to-default coercion; the attribute write is incidental.
**Fix:** Pick one source of truth. Simplest: keep `applyTheme` as the sole writer and drop the inline `data-theme={theme}` (bind only `bind:this={rootEl}`); or keep the inline binding and have the effect persist only.

### IN-03: Synthesized search-result tooltip reports `t30: 0`, rendering "(30d avg, 0 transactions)"

**File:** `web/src/lib/components/SearchResults.svelte:53-57`
**Issue:** `priceRows` fabricates a single WTS row `{ direction: '0', a30: pricePp, t30: 0 }` because the search row only carries `pricePp`. `composeItemNote` then renders `Recent ask: <n>pp (30d avg, 0 transactions)`. Showing "0 transactions" next to a real average is mildly self-contradictory (the average came from >0 transactions upstream). Cosmetic, but visible.
**Fix:** Either omit the transaction-count clause when `t30` is unknown/0, or carry `t30` through the `SearchResultRow` from the `view` payload so the tooltip shows the true count.

### IN-04: `mergeObjects` Proxy is allocated per option-merge; comment claims a reactivity re-trigger that the code does not perform

**File:** `web/src/lib/table/createSvelteTable.svelte.ts:83-91`
**Issue:** The `onStateChange` override comment says "Re-trigger reactivity, then forward to the caller's setter," but the body only forwards to `options.onStateChange` — there is no explicit re-trigger. Reactivity actually works because the DataGrid's `onXChange` setters reassign the `$state` sorting/filter signals (the real trigger lives in `DataGrid.svelte:80-82`), so the comment describes intent the adapter doesn't implement. Not a bug (the table sorts/filters correctly, proven by `tableAdapter.test.ts`), but the comment will mislead the next maintainer into thinking the adapter owns reactivity.
**Fix:** Correct the comment to state that reactivity is owned by the caller's `$state` setters, and that this wrapper only forwards the updater.

### IN-05: Tooltip popover has no viewport-collision / clipping handling

**File:** `web/src/lib/components/ItemTooltip.svelte:127-147`
**Issue:** The popover is absolutely positioned `top:100%; left:0` with `max-width:360px`. For an `Item` cell near the right edge or bottom of the `max-height:70vh` scroll region (`DataGrid.svelte:275`), the popover can overflow the viewport or be clipped by the scroll container's `overflow:auto`, with no flip/shift. Functional but can render the tooltip partially unreadable on narrow screens — a UX robustness gap for the WEB-04 deliverable.
**Fix (optional / non-blocking):** Consider a small flip-on-overflow (detect `getBoundingClientRect` past the viewport and switch `left`/`top`), or render the popover in a portal layer above the scroll region. Acceptable to defer if out of scope for P14.

---

_Reviewed: 2026-05-30_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
