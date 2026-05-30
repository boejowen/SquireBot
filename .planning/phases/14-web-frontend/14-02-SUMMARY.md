---
phase: 14-web-frontend
plan: 02
subsystem: ui
tags: [sveltekit, svelte5, adapter-static, tailwindcss-v4, vitest, fontsource, tanstack-table-core, search, levenshtein, tooltip, xss-escaping, themes, css-custom-properties]

# Dependency graph
requires:
  - phase: 14-web-frontend (Plan 14-01)
    provides: "the read-API JSON contract these client modules consume — PriceDetail.direction TEXT '0'=WTS/'1'=WTB/'2'=BOTH, nullable price, prices[].a30/t30, quest_links[].quest_name/source, wiki_summary, is_quest_item (snake_case, locked in compute/types.go)"
provides:
  - "web/ — the repo's first frontend: a scaffolded, building, type-checking SvelteKit static SPA (adapter-static fallback 200.html, Tailwind v4 CSS-first, self-hosted @fontsource woff2, noindex + robots disallow)"
  - "web/src/lib/search/searchIndex.ts — ported levenshtein (verbatim) + didYouMean (999.28 empty-query guard) + groupAndSort + the searchRows in-memory WEB-03 engine over fetched view rows"
  - "web/src/lib/tooltip/composeNotes.ts — ported tooltip composer rewritten plain-text -> escaped rich HTML (escapeHtml on every interpolated value; the HIGH-severity XSS mitigation, test-proven)"
  - "web/src/lib/theme/themes.ts + the [data-theme] blocks in app.css — the 5-theme CSS-custom-property registry (velious default, sheets-default dropped, localStorage helpers)"
affects: [14-04-svelte-client, 14-03-read-handlers-cors, 15-admin-web-forms]

# Tech tracking
tech-stack:
  added:
    - "@sveltejs/kit 2.61.1 + svelte 5.56.0 + @sveltejs/adapter-static 3.0.10 (SPA fallback)"
    - "tailwindcss 4.3.0 + @tailwindcss/vite (CSS-first; NO tailwind.config / postcss.config)"
    - "vite 8.0.14 + vitest 4.1.7"
    - "@tanstack/table-core 8.21.3 (headless; Plan 04 builds the local adapter) — NOT @tanstack/svelte-table (Svelte-4 trap)"
    - "@lucide/svelte 1.17.0 (Plan 04 icons) — NOT lucide-svelte (deprecated)"
    - "@fontsource/{cinzel,cinzel-decorative,im-fell-english,medievalsharp,crimson-text,inter} (self-hosted woff2, Off-Google)"
    - "@types/node ^24 (devDep — silences the svelte-kit-generated 'node' types lib reference)"
  patterns:
    - "adapter-static SPA: svelte.config fallback '200.html' + +layout.ts ssr=false/prerender=false; root +page.ts prerender=true emits index.html shell (safe under ssr=false — no build-time fetch)"
    - "Tailwind v4 CSS-first: @import \"tailwindcss\" in app.css + @tailwindcss/vite plugin; zero config files"
    - "Pure-logic ports keep the v1 trust-boundary contract: the lib returns RAW strings; escaping is the presentation layer's job (composeNotes' escapeHtml + Svelte {})"
    - "Theme = CSS custom properties on [data-theme]; a swap is one attribute write; per-user localStorage; resolveTheme whitelists to the 5-key enum (no injection)"

key-files:
  created:
    - "web/package.json (pinned deps), web/svelte.config.js, web/vite.config.ts, web/vitest.config.ts (scaffold), web/tsconfig.json"
    - "web/src/app.html (noindex), web/src/app.css (@import tailwindcss + 6 fontsource + [data-theme] blocks)"
    - "web/src/routes/+layout.ts (ssr/prerender false), web/src/routes/+page.ts (prerender true)"
    - "web/static/robots.txt (Disallow: /)"
    - "web/src/lib/search/searchIndex.ts + web/src/lib/__tests__/searchIndex.test.ts"
    - "web/src/lib/tooltip/composeNotes.ts + web/src/lib/__tests__/composeNotes.test.ts"
    - "web/src/lib/theme/themes.ts + web/src/lib/__tests__/themes.test.ts"
  modified:
    - "web/src/routes/+layout.svelte (import ../app.css instead of layout.css)"

key-decisions:
  - "Deploy target LOCKED to Cloudflare Pages at the root subdomain app.squirebot.quest — a root origin, so kit.paths.base is NOT set (no GH-Pages base-path tax); this is also Plan 03's CORS allow-origin"
  - "999.30 fixed via path (a): whole-string Wagner-Fischer Levenshtein is the correct WEB-03 algorithm; the formerly-skipped Test 4 is un-skipped and asserts toEqual([]) (the v1 plan-locked assertion was arithmetically wrong)"
  - "composeNotes consumes the Plan-01 JSON contract: PriceDetail.direction is TEXT '0'=WTS/'1'=WTB/'2'=BOTH (a BOTH row surfaces as both ask + buy); a30/t30 numeric; the top-line nullable price field is not consumed (the per-direction prices[] is)"
  - "themes.ts token VALUES come from the UI-SPEC Theme Catalog (richer mockup values), not the dimmer v1 themes.ts hexes — CLAUDE.md's eq-aesthetic-theme.md derivation is satisfied transitively (the catalog derives from that doc)"
  - "Pinned the exact RESEARCH-verified package versions (re-confirmed via npm view at install); @tanstack/table-core installed now (NOT svelte-table) so Plan 04 has no install step"

patterns-established:
  - "web/ is the repo's first frontend package — its own package.json/node_modules, parallel to apps-script/"
  - "Literal-grep-vs-comment discipline (carried from Plan 14-01): comments are reworded to avoid the exact tokens the acceptance greps forbid (.skip, CacheService/etc., sheets-default) while preserving documented intent"
  - "Each ported module ships with its vitest suite proving v1 parity + the carried-bug/security fixes"

requirements-completed: [WEB-03, WEB-04, WEB-05]

# Metrics
duration: 27min
completed: 2026-05-30
---

# Phase 14 Plan 02: web/ SvelteKit Foundation + Search/Tooltip/Theme Ports Summary

**The repo's first frontend stood up — a building, type-checking SvelteKit static SPA (adapter-static, Tailwind v4, self-hosted fonts, noindex) — plus the three tested v1 pure-logic modules ported to client TS: searchIndex (999.28 + 999.30 fixed), composeNotes (rewritten plain-text -> escaped rich HTML, the HIGH-severity XSS mitigation proven), and the 5-theme CSS-custom-property registry (velious default).**

## Performance

- **Duration:** ~27 min
- **Started:** 2026-05-30T15:59:57Z
- **Completed:** 2026-05-30T16:26:26Z
- **Tasks:** 4 (Task 1 scaffold; Tasks 2-4 TDD ports)
- **Files created:** 18 source/config files (the three ported modules + their three vitest suites + the SvelteKit scaffold)

## Accomplishments

- **Scaffolded `web/`** non-interactively (`npx sv create` with explicit flags) as a SvelteKit minimal + TS + vitest + Tailwind-v4 + adapter-static app. `npm run build` emits `build/index.html` + `build/200.html` (SPA fallback); `npm run check` is 0 errors / 0 warnings; 31 self-hosted woff2 faces bundle with **zero** Google Fonts runtime link (grep-clean in both `src/` and `build/`). `@tanstack/table-core` is installed; the `@tanstack/svelte-table` Svelte-4 trap is **not**.
- **Ported `searchIndex.ts`** to pure client TS: `levenshtein` verbatim (the WEB-03 oracle), `didYouMean` with the **999.28** empty-query guard as its first line, `groupAndSort` + `COLLAPSE_THRESHOLD=5`, and a new `searchRows` in-memory engine (substring match -> group/sort -> didYouMean on no-match) replacing the v1 Sheet-coupled `runSearch`. All Apps Script runtime I/O dropped. **999.30** fixed (path a): the formerly-skipped Test 4 is un-skipped and asserts `toEqual([])`.
- **Ported `composeNotes.ts` to escaped rich HTML** — the one HIGH-severity item in P14 (T-14.02-01). `escapeHtml()` (ampersand-first, ASVS V5) is applied to **every** interpolated value (item/quest names, wiki summary, and the wiki URL inside the `href`); only structural tags are literal. The wiki anchor carries `rel="noopener" target="_blank"`. A vitest assertion proves a malicious `<img src=x onerror=alert(1)>` item name renders as `&lt;img`, never a live tag (plus quest-name / summary / URL escaping cases).
- **Ported the THEMES registry to CSS custom properties** — 5 themes (velious/vanilla/kunark/minimalist/heavy), `sheets-default` dropped, `DEFAULT_THEME = 'velious'`. `app.css` emits a `[data-theme]` block per theme + a `:root` velious fallback + Heavy's inverting parchment row CSS + `prefers-reduced-motion`. `resolveTheme` whitelists localStorage to the 5-key enum (no injection); SSR-safe `loadTheme`/`saveTheme`.
- **43 vitest tests green** across the three ported suites; build + check clean.

## Task Commits

Each task was committed atomically (TDD tasks land test + impl together — the TS test can't compile without the module's exported signatures, the same constraint Plan 14-01 documented):

1. **Task 1: Scaffold web/ SvelteKit static SPA** — `2ac5357` (feat)
2. **Task 2: Port searchIndex.ts (999.28 + 999.30 fixed)** — `30123e4` (feat)
3. **Task 3: Port composeNotes.ts -> escaped rich HTML (XSS mitigation)** — `2952b2a` (feat)
4. **Task 4: Port THEMES -> 5-theme CSS custom properties (velious default)** — `269b069` (feat)

**Plan metadata:** _(this SUMMARY + STATE + ROADMAP)_ committed separately.

## Files Created/Modified

- `web/package.json` — pinned RESEARCH-verified versions + the runtime deps (table-core, lucide, 6 fontsource); `@types/node` devDep.
- `web/svelte.config.js` — `adapter-static({ fallback: '200.html' })`; no `kit.paths.base` (root origin).
- `web/vite.config.ts` — `@tailwindcss/vite` + sveltekit plugins (scaffold-generated; Tailwind v4 CSS-first).
- `web/src/app.html` — `<meta name="robots" content="noindex">` (D-05); `lang="en"`.
- `web/src/app.css` — `@import "tailwindcss"` + 6 `@fontsource` woff2 imports + the 5 `[data-theme]` token blocks (+ `:root` velious fallback, Heavy parchment rows, reduced-motion).
- `web/src/routes/+layout.ts` — `ssr = false; prerender = false` (SPA).
- `web/src/routes/+page.ts` — `prerender = true` (emit `index.html`; safe under `ssr=false`).
- `web/src/routes/+layout.svelte` — imports `../app.css` (renamed from the scaffold's `layout.css`).
- `web/static/robots.txt` — `Disallow: /` (D-05).
- `web/src/lib/search/searchIndex.ts` (+ `__tests__/searchIndex.test.ts`) — the WEB-03 logic, both bugs fixed.
- `web/src/lib/tooltip/composeNotes.ts` (+ `__tests__/composeNotes.test.ts`) — the WEB-04 tooltip, escaped HTML.
- `web/src/lib/theme/themes.ts` (+ `__tests__/themes.test.ts`) — the WEB-05 theme registry.

## Decisions Made

- **Deploy target locked to Cloudflare Pages at `app.squirebot.quest` (root subdomain).** A root origin avoids the GH-Pages base-path tax (so `kit.paths.base` is intentionally unset) and gives a clean CORS origin — this is the value Plan 03 must put in the Go read API's `Access-Control-Allow-Origin`.
- **999.30 fix path (a)** — whole-string Wagner-Fischer Levenshtein is the correct algorithm (it is the v1 production behavior and the WEB-03 oracle); the bug was a wrong test assertion, not a wrong algorithm. The un-skipped Test 4 now asserts `toEqual([])` because `levenshtein('clok', 'cloak of confusion') >= 13`, far above the `<=2` cutoff.
- **Price-detail JSON contract** — `composeNotes` reads `prices: PriceDetail[]` with `direction` TEXT `"0"`=WTS / `"1"`=WTB / `"2"`=BOTH (a BOTH row surfaces as both an ask and a buy line), `a30`/`t30` numeric. The top-line nullable `price` field (Plan 01's `*float64`) is not consumed here — the per-direction `prices[]` drives the tooltip. Documented in the file header (the explicit contract with Plans 01/03).
- **Theme token values from the UI-SPEC Theme Catalog**, not the dimmer v1 `themes.ts` hexes (RESEARCH flags the v1 palette as superseded by the richer mockup values). CLAUDE.md's "derive from `eq-aesthetic-theme.md`" rule is satisfied transitively because the catalog derives from that doc (verified: the doc's velious mockup is `bg #0f1729 / accent #a8c5e0 / Cinzel Decorative`).
- **Final pinned package versions** (re-confirmed via `npm view` at install, all matching RESEARCH): kit 2.61.1, svelte 5.56.0, adapter-static 3.0.10, tailwindcss 4.3.0, @tailwindcss/vite 4.3.0, vite 8.0.14, vitest 4.1.7, @tanstack/table-core 8.21.3, @lucide/svelte 1.17.0, @fontsource cinzel/cinzel-decorative/inter/medievalsharp 5.2.8, crimson-text 5.2.7, im-fell-english 5.2.6.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added a root `+page.ts` with `prerender = true` so `build/index.html` emits**
- **Found during:** Task 1 (scaffold verification)
- **Issue:** With adapter-static SPA `fallback: '200.html'` and no prerendered routes, the adapter emitted only `200.html` — but the plan's acceptance criterion requires `web/build/index.html`. The plan also mandates `+layout.ts` keep `prerender = false`, so flipping the layout was not an option.
- **Fix:** Added `web/src/routes/+page.ts` with `export const prerender = true` (a page-level override of the layout default). Under the inherited `ssr = false`, prerendering the index emits only the empty client-hydration shell — NO build-time data fetch runs, so the RESEARCH "cross-origin fetch during prerender" anti-pattern does not apply. Data-driven view routes (Plan 04) keep the layout's `prerender = false` and render via the 200.html SPA fallback.
- **Files modified:** `web/src/routes/+page.ts` (new)
- **Verification:** `npm run build` now emits both `build/index.html` and `build/200.html`; `+layout.ts` still greps `ssr = false` + `prerender = false`.
- **Committed in:** `2ac5357` (Task 1 commit)

**2. [Rule 3 - Blocking] Added `@types/node` devDependency**
- **Found during:** Task 1 (svelte-check)
- **Issue:** The svelte-kit-generated `.svelte-kit/tsconfig.json` injects `"types": ["node"]`, but `@types/node` was not a dependency, so `svelte-check` emitted a `Cannot find type definition file for 'node'` warning.
- **Fix:** Added `@types/node ^24.12.4` (matched to the Node 24 runtime) as a devDependency.
- **Files modified:** `web/package.json`, `web/package-lock.json`
- **Verification:** `npm run check` is now 0 errors / 0 warnings.
- **Committed in:** `2ac5357` (Task 1 commit)

**3. [Rule 1 - Hygiene] Reworded comments to satisfy the literal acceptance greps**
- **Found during:** Tasks 2 + 4
- **Issue:** Several acceptance criteria are literal `grep -c` checks that must return 0 (`.skip` in searchIndex.test.ts; `CacheService|PropertiesService|...` in searchIndex.ts; `sheets-default` in themes.ts). My explanatory comments mentioned those exact tokens (e.g. "the formerly-it.skip Test 4", a DROPPED-list naming the Apps Script services, "DROPPED: 'sheets-default'"), producing false-positive grep hits.
- **Fix:** Reworded the comments to convey the same intent without the forbidden literal substrings (e.g. "formerly-skipped", "the document-cache + user-properties services", "the v1 no-styling opt-out sentinel"). No code behavior changed. (Same discipline Plan 14-01 documented for its `HYPERLINK` comments.)
- **Files modified:** `web/src/lib/search/searchIndex.ts`, `web/src/lib/__tests__/searchIndex.test.ts`, `web/src/lib/theme/themes.ts`
- **Verification:** all three greps now return 0; all suites still pass.
- **Committed in:** `30123e4` (Task 2), `269b069` (Task 4)

**4. [Rule 1 - Hygiene] Removed the scaffold's `vitest-examples` demo files**
- **Found during:** Task 4 (full-suite review)
- **Issue:** `npx sv create` left `src/lib/vitest-examples/greet.ts` + `greet.spec.ts` demo placeholders, which polluted the test suite with a non-deliverable example.
- **Fix:** `git rm`'d the `vitest-examples` directory (kept `lib/index.ts`, the idiomatic `$lib` barrel placeholder).
- **Files modified:** deleted `web/src/lib/vitest-examples/greet.ts` + `greet.spec.ts`
- **Verification:** full suite is 43 tests across the 3 ported files, all green.
- **Committed in:** `269b069` (Task 4 commit)

---

**Total deviations:** 4 auto-fixed (2 blocking, 2 hygiene)
**Impact on plan:** All four were necessary to satisfy the plan's own acceptance criteria (index.html, clean svelte-check, literal greps) or to keep the deliverable clean. No scope creep; no behavior change to the ported logic.

## Issues Encountered

- **The `tailwindcss` sv add-on still prompted for plugins** despite `--no-add-ons`-style flags, hanging the first scaffold attempt. Resolved by pinning the plugin selection explicitly: `--add "tailwindcss=plugins:none"` (the documented "set ALL options to skip prompts" form). The partial `web/` was removed and the scaffold re-run cleanly.
- No other problems. The toolchain (Node v24.14.0, npm 11.9.0, sv 0.15.3) was present; all package versions matched RESEARCH; the v1 sources ported with provable parity.

## User Setup Required

None — no external service configuration required for this plan. (Cloudflare Pages deploy of `web/` and the CORS origin wiring are Plan 03/04/16 concerns, not this plan.)

## Known Stubs

- **`web/src/routes/+page.svelte` still holds the scaffold's "Welcome to SvelteKit" placeholder** — this is an INTENTIONAL, plan-bounded deferral, not an oversight. Plan 14-02 is explicitly the **logic + theme half**: it builds the three pure modules + their suites and stands up the foundation. The actual data-driven view UI (the 4 grids, the SiteShell, the ItemTooltip/SearchBox/ThemePicker components that WIRE these modules against Plan 03's JSON) is **Plan 14-04**'s scope. No data source is wired here because none is in scope; the modules are unit-tested in isolation. This does not block the plan's goal.

## Next Phase / Next Plan Readiness

- **Plan 14-04 (Svelte client)** can now import `searchRows`/`didYouMean` (WEB-03), `composeItemNote` (WEB-04, already escaped — the consumer `{@html}`s it safely), and `THEMES`/`resolveTheme`/`loadTheme`/`saveTheme` + the `[data-theme]` CSS (WEB-05), and build the `DataGrid` over the already-installed `@tanstack/table-core` with a local Svelte-5 adapter (Plan 04 builds the adapter — no install step needed). The `.eq-row`/`.eq-header` class hooks for Heavy's parchment rows are already in `app.css`.
- **Plan 14-03 (read handlers + CORS)** should set the CORS `Access-Control-Allow-Origin` to `https://app.squirebot.quest` (the locked deploy origin), and can rely on the documented price-detail field consumption (`prices[].direction` string `"0"/"1"/"2"`, `a30`/`t30`).
- **No blockers.** `npm run build` exits 0 (adapter-static, both index.html + 200.html); `npm run check` 0/0; `npx vitest run` 43/43; no Google Fonts link; table-core present / svelte-table absent.

## Self-Check: PASSED

- All 14 created files verified present on disk (the SvelteKit scaffold config + the three ported modules + their three vitest suites + this SUMMARY).
- All 4 task commits verified in git log: `2ac5357`, `30123e4`, `2952b2a`, `269b069`.
- `npm run build` exits 0 (adapter-static; emits `build/index.html` + `build/200.html`); `npm run check` 0 errors / 0 warnings; `npx vitest run` 43/43 across the 3 ported suites; 0 Google Fonts references in `src/` or `build/`; `@tanstack/table-core` present, `@tanstack/svelte-table` absent.

---
*Phase: 14-web-frontend*
*Completed: 2026-05-30*
