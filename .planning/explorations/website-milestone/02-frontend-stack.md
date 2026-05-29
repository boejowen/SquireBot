# Website Milestone — Slice 02: Frontend Stack & UI Rebuild

**Scope:** Research only. Evaluate replacing the Google Sheet frontend with a real website. This
slice covers the user-facing UI: what exists today, the web-equivalent of each feature, the
rebuild effort, and a single recommended frontend stack.

**Researched:** 2026-05-20 against `apps-script/src/` at v1.0.1 ship state (~13,266 LOC TS).

**Confidence:** HIGH for the feature inventory (read directly from source). HIGH for the
"what the Sheet gave for free" analysis. MEDIUM-HIGH for the stack recommendation (a judgment
call weighted to maintainer constraints, not an objective optimum).

---

## TL;DR

- The current "UI" is **4 consolidated tab builders + 5 HtmlService sidebars + 1 theme modal +
  1 custom menu**, all rendering into a Google Sheet. Total user-facing surface is modest:
  ~7 distinct screens.
- The Sheet hands you **four expensive things for free**: the data grid itself, per-column
  filtering, per-column sorting, and multi-user concurrent access with auth. A website must
  hand-build or buy all four. This is the single biggest cost driver of the milestone.
- **Recommended stack: SvelteKit (static adapter) + TanStack Table (Svelte adapter) + Tailwind,
  hosted on Cloudflare Pages, talking to a small JSON API.** Rationale below. The data is
  read-mostly, the dataset is tiny (~150k cells worst case), and the maintainer values low build
  and operational complexity — this combination minimizes both.
- **Total rebuild estimate: ~6–9 working days** for a part-time solo maintainer comfortable with
  TS, *excluding* the backend API (separate slice). Most of the cost is the grid/filter/sort
  plumbing the Sheet used to provide, not the views themselves.

---

## Current UI Feature Inventory

Read from the apps-script source. "Screen" = a distinct thing a guildie looks at or interacts
with.

| # | Feature | Source file | What it does today | Columns / shape |
|---|---------|-------------|--------------------|-----------------|
| 1 | **`view` tab** — consolidated inventory | `tabs/buildView.ts` | Every character's every inventory row in one filterable grid. Wiki hyperlink per item, PigParse price, cell-note tooltip, conditional "Last Synced" coloring. | `Char, Slot, Item, ID, Count, Wiki, Price, Last Synced` |
| 2 | **`gear_check` tab** — Velious gear progression | `tabs/buildGearCheck.ts` | Per char, per tier (Pre-Raid, Raiding, Iksar-if-IKS), per slot: what's recommended vs. what's equipped, with `OK / MISSING / OTHER` status. Slot-pair aware (EAR1/EAR2 etc.). | `Char, Class, Tier, Slot, Have, Recommended, Status` |
| 3 | **`spell_check` tab** — spell progression | `tabs/buildSpellCheck.ts` | Per char, joins spellbook against per-class/per-level wiki spell list ≤ char level, `KNOWN / MISSING` per spell. | `Char, Class, Level, Spell, Status` |
| 4 | **`bank` tab** — shared bank view | `tabs/buildBank.ts` | Same shape as `view` but filtered to the designated bank toon, plus a fixed COIN row (PP/GP/SP/CP from `_meta`). | `Char, Slot, Item, ID, Count, Wiki, Price, Last Synced` + COIN row |
| 5 | **Search sidebar** | `triggers/showSearchSidebar.ts` + `lib/searchIndex.ts` | Cross-character item search. Substring match, group-by-item, auto-collapse groups >5 chars, char/slot dropdown filters, Levenshtein "did you mean?" fuzzy fallback, recent-3 MRU, per-item wiki link + price + summary tooltip. | 300px panel |
| 6 | **Character Info sidebar** | `triggers/showCharInfoSidebar.ts` | Admin/owner form: set class/level/race per char in `_char_owner`. Editable table with class/race dropdowns + level number input, batch save with validation. | 360px panel — **write action** |
| 7 | **Eviction sidebar** | `triggers/showEvictionSidebar.ts` | Officer-only: pick a departed guildie's email, preview affected chars, commit `is_removed=TRUE` + 30-day grace. Admin-gated server-side. | 300px panel — **write action** |
| 8 | **Bank Coin sidebar** | `triggers/showBankCoinSidebar.ts` | Form to set bank toon's PP/GP/SP/CP into `_meta` (file format has no coin). | panel — **write action** |
| 9 | **Admin Management sidebar** | `triggers/showAdminMgmtSidebar.ts` | Officer-only: manage the `_meta.guild_admins` allowlist, with owner-floor lockout protection. | panel — **write action** |
| 10 | **Theme picker modal** | `triggers/onOpen.ts` (`showThemePickerModal`) | Pick one of 6 themes; rewrites `_meta.theme`, triggers a view rebuild. | modal |
| 11 | **EQ-aesthetic theming** | `lib/themes.ts` | 6 themes (vanilla / kunark / velious / minimalist / heavy / sheets-default), each = header bg/fg, row colors, font family, accent colors. Applied to view tabs AND injected as CSS custom properties into every sidebar. | registry |
| 12 | **Item tooltips** | `tabs/composeNotes.ts` | Per inventory row: cell-note string = wiki summary + PigParse ask/buy 30d averages + quest-item flag + quest-name list. Plain text only (cell-note limitation). | note on Item column |
| 13 | **Conditional formatting** | `buildView.ts` `applyLastSyncedConditionalFormatting` | "Last Synced" column: green <7d, orange <30d, red ≥30d. | 3 rules |
| 14 | **Custom menu** | `triggers/onOpen.ts` | `SquireBot` menu: rebuild/refresh actions, sidebar openers, migration runners. Most are maintainer-only ops. | menu |
| 15 | **Filtering & sorting** | (native Sheets) | Every consumer of tabs 1–4 uses Google Sheets' built-in column filters, filter views, and sort. **Not in the codebase — it's a free Sheets feature.** | — |
| 16 | **Multi-user concurrent access** | (native Sheets) | 12 guildies open the same workbook simultaneously; Google handles auth, presence, live updates. **Not in the codebase.** | — |

**Key observation:** the *visible* product is only screens 1–9 plus theming. The maintainer-only
plumbing (menu items 14, migration runners, "rebuild now" buttons) does not need a web
equivalent — on a website those become backend cron jobs / admin endpoints, not UI.

---

## Web-Equivalent Mapping & Effort

Effort scale per feature: **Trivial** (<½ day), **Moderate** (½–1.5 days), **Significant**
(1.5+ days).

| Feature | Web equivalent | Effort | Notes |
|---------|----------------|--------|-------|
| 1. `view` inventory | One route `/inventory` rendering a TanStack Table over the inventory JSON | **Moderate** | Logic is a flat list render. Cost is grid wiring (see "for free" below), not the view. |
| 2. `gear_check` | Route `/gear` — same grid, status-colored cells | **Moderate** | The join logic moves server-side (or is precomputed by the API). Front end just renders rows + a status badge cell renderer. |
| 3. `spell_check` | Route `/spells` — same grid | **Trivial→Moderate** | Identical pattern to gear_check; once one progression grid exists the second is cheap. |
| 4. `bank` | Route `/bank` — grid + a coin summary banner | **Trivial→Moderate** | Same grid component as `view` filtered to one char; coin banner is a small static component. |
| 5. Search | `/search` page or global search box; substring filter client-side over already-loaded data; Levenshtein in a tiny TS util | **Moderate** | The fuzzy match (`levenshtein`, `didYouMean`) is pure TS — **port verbatim**, ~30 lines. Group-by + collapse is straightforward. With all data loaded client-side, search can be instant (no API round-trip, no 60s cache). |
| 6. Character Info | An editable form/table page → `PUT /api/char/:name` | **Moderate** | A write action. Needs auth gating + validation (port the class/level/race rules). |
| 7. Eviction | Admin page → `POST /api/evict` | **Moderate** | Admin-gated. UI is a dropdown + preview + confirm. The 30-day grace + cascade logic belongs in the backend. |
| 8. Bank Coin | Small admin form → `PUT /api/bank-coin` | **Trivial** | Four number inputs + save. |
| 9. Admin Management | Admin page → `/api/admins` CRUD | **Moderate** | Allowlist editor + owner-floor protection (backend-enforced). |
| 10. Theme picker | A settings dropdown writing a `localStorage` value (or per-user pref) | **Trivial** | On a website, theme is a *per-user client preference* — no server write, no rebuild needed. Strictly simpler than today. |
| 11. EQ theming | 6 Tailwind/CSS theme classes or CSS-variable sets, derived from `docs/design/eq-aesthetic-theme.md` | **Moderate** | One-time CSS authoring. The `THEMES` registry values port directly into CSS custom properties — they're *already* used that way in the sidebars today. |
| 12. Item tooltips | Native hover tooltip / popover component; can now be **rich HTML** (links, formatting) instead of plain-text cell notes | **Trivial→Moderate** | This is an *upgrade*: cell notes were plain-text-only (an explicit anti-pattern workaround in ARCHITECTURE.md). A web popover removes that limitation for free. `composeNotes.ts` logic ports as a render function. |
| 13. Conditional formatting | A cell renderer that colors the "Last Synced" cell by age | **Trivial** | 3-line date comparison in a TanStack cell renderer. |
| 14. Custom menu | Not a UI feature on a website — refresh jobs become backend cron; "rebuild" disappears (views are computed on read). | **Trivial** (deletion) | Net *reduction* in scope. |
| 15. Column filtering | TanStack Table column filters (built-in) | **Moderate** | Free with TanStack but you must wire filter UI per column (text/select inputs). This is real work the Sheet did for nothing. |
| 16. Column sorting | TanStack Table sorting (built-in) | **Trivial** | Genuinely near-free with TanStack — `getSortedRowModel()` + clickable headers. |
| — Multi-user access | Static site + JSON API; everyone hits the same URL | **Moderate** (mostly backend) | Concurrency moves to the API/DB layer (other slice). Frontend just fetches. No live-presence/co-editing needed — data is read-mostly. |
| — Auth | A login (Google sign-in is natural, or a guild password) | **Moderate** (mostly backend) | Today Google *is* the auth. A website must add a thin auth layer. Mostly a backend concern; frontend just gates admin routes. |

---

## What the Sheet Gave For Free (the real cost of this milestone)

These are not "features" anyone wrote — they are properties of Google Sheets that the rebuild
must replace. This list is the honest answer to "why isn't this trivial?"

1. **The data grid itself.** A scrollable, virtualized, resizable, copy-pasteable table.
   Replaced by a table component (TanStack Table) + your own markup/CSS. The single biggest
   chunk of work.
2. **Per-column filtering.** Every guildie today clicks the filter funnel to narrow by Char,
   item name, status, etc. On a website this is per-column filter inputs you build and wire.
   TanStack provides the *engine*; you provide the *controls*.
3. **Per-column sorting.** Clicking a header to sort. Near-free with TanStack (`getSortedRowModel`),
   but still has to be enabled and styled.
4. **Multi-user concurrent access + identity.** 12 people open one Google URL; Google handles
   sessions, permissions, and live data. The website must add: hosting, an auth mechanism, and
   a backend serving shared data. (Backend is a separate slice — but the frontend's auth-gating
   and "fetch shared state" code is in-scope here.)
5. **Freeze panes, frozen header row, cell selection, find-in-page (Ctrl+F).** Minor, but each
   is a small thing users silently rely on. Sticky header is one CSS line; browser Ctrl+F still
   works on rendered rows.
6. **Zero hosting / zero ops.** The Sheet has no uptime to manage. A website needs hosting
   (cheap/free, see below) and a deploy pipeline.
7. **Built-in mobile rendering.** The Sheet is usable (badly) on phones for free. A website must
   choose to be responsive — though for a table-heavy guild tool, "desktop-first, tolerable on
   mobile" is an acceptable stance.

**What the website gives back for free** (worth noting so the trade is fair): rich HTML tooltips
(no plain-text cell-note cap), instant client-side search (no 60s cache, no Apps Script
round-trip), no 200-tab limit anxiety, no Apps Script 6-min execution cap, no recurring Google
OAuth verification incidents (the milestone's actual motivation), and far better control over
layout and theming.

---

## Stack Options Compared

The maintainer is a part-time solo dev, comfortable with **Go and TypeScript**, values **low
build/operational complexity**, currently spends **$0**, serves **~12 users**, and the data is
**read-mostly**.

| Option | Build simplicity | TS fit | Hosting | Verdict for SquireBot |
|--------|-----------------|--------|---------|----------------------|
| **Plain static HTML + `fetch`** | Highest — no build step | Loose (no components, manual DOM) | Any static host | Tempting for 12 users, but you'd hand-roll the table, filters, sorting, and routing. That's *exactly* the work TanStack/a framework saves. Ends up *more* code, not less. **Rejected.** |
| **React + Vite (SPA)** | Good — Vite is fast, mature | Excellent | Static host | Solid, boring, well-documented. TanStack Table's primary target. Bigger bundle and more ceremony (hooks, effects) than Svelte for a simple read-mostly app. Viable runner-up. |
| **Next.js** | Lower — App Router, RSC, server/client split | Excellent | Needs Node host or Vercel for full features | Overkill. RSC and the server runtime add concepts and ops burden for an app that is a handful of static pages over a JSON API. The 2026 consensus is Next.js is the wrong call for a simple hobby dashboard. **Rejected.** |
| **SvelteKit (static adapter)** | High — minimal boilerplate, batteries included (routing, forms), small bundle | Excellent — first-class TS | Static host (Cloudflare/Netlify Pages) via `adapter-static` | Compiles to lean vanilla JS, no virtual DOM. Routing, transitions, and reactivity built in with little ceremony. 2026 framework reviews specifically favor it for small dashboards and solo/small teams avoiding RSC overhead. **Recommended.** |
| **Astro** | High for content, but fights you for an app | Excellent | Static host | Great for the *existing* Jekyll onboarding site; poor for an interactive, stateful, filterable data tool. You'd end up shipping a Svelte/React island for every page anyway. **Rejected for the app** (fine if the onboarding site ever moves). |
| **SolidStart / others** | Good | Excellent | Static host | Smaller ecosystem, fewer table-component options, more niche knowledge to carry. Not worth the bet for a hobby project. **Rejected.** |

### Grid / table component

| Component | Fit |
|-----------|-----|
| **TanStack Table v8** | **Recommended.** Headless (logic only: sorting, filtering, grouping, pagination), fully free, no community/enterprise split, ~3M weekly downloads, has a Svelte adapter. For <10k rows it is the lightweight, flexible default. You write the markup once and reuse it across all 4 grid views. SquireBot's worst case is ~150k cells / a few thousand rows — comfortably inside TanStack's sweet spot. |
| AG Grid Community | Batteries-included Excel-like grid. Genuinely powerful, but it's a heavy dependency, has an opinionated look, and its best features (pivot, clipboard, server row model) are paywalled Enterprise. Overkill for 12 users and a few thousand rows. Reasonable fallback only if hand-wiring filter UIs proves annoying. |
| react-data-grid / others | Tied to React; no reason to prefer over TanStack here. |

For ~12 users and a few thousand rows, **virtualization is not even required** — TanStack
without row virtualization will render fine. Add `@tanstack/svelte-virtual` later only if a
maxed-guild dataset feels sluggish.

---

## Static-vs-Server Hosting

The frontend should be a **fully static build** (`adapter-static`) — pre-rendered HTML/CSS/JS,
no server-side rendering needed, because every page is generic and the per-guild data arrives
via `fetch` from the API at runtime.

- **Frontend host:** Cloudflare Pages (or Netlify / GitHub Pages). Free tier, global CDN, Git
  push-to-deploy, custom domain, HTTPS — zero ops, $0. GitHub Pages is already in use for the
  onboarding site, so the maintainer knows the pattern; Cloudflare Pages is preferred only
  because it pairs naturally with Cloudflare Workers if the API ever lands there.
- **Pairing with the backend:** static site (CDN) + a small JSON API on a separate origin.
  The frontend calls `https://api.squirebot.<domain>/...`. The watcher pushes to that same API
  instead of Google Sheets — which is the entire point of the milestone.
- **Why not SSR:** SSR buys SEO and first-paint-with-data. This is a private guild tool — SEO is
  irrelevant, and a 12-person audience will not notice a 200ms client-side data fetch. SSR would
  add a server runtime to host and operate, directly against the "low operational burden"
  constraint. **Static wins decisively.**
- **CORS / auth note for the API slice:** a static frontend on a different origin means the API
  needs CORS headers and a token/cookie auth scheme. Flag for the backend slice; not frontend
  work, but it constrains the API design.

---

## Effort Estimate

Solo part-time maintainer, comfortable with TypeScript, learning-curve for SvelteKit assumed
mild (it is a small framework). **Excludes the backend API and the watcher rewrite** — those are
separate slices. Estimates in ideal working days.

| Work item | Effort | Notes |
|-----------|--------|-------|
| Project scaffold: SvelteKit + `adapter-static` + Tailwind + TanStack Svelte adapter; deploy pipeline to Cloudflare Pages | 0.5 d | One-time. |
| Shared reusable `<DataGrid>` component (TanStack wiring: columns, sorting, per-column filter inputs, sticky header) | 1.5–2 d | **The core cost.** Replaces "the Sheet for free." Everything else reuses this. |
| Theme system: port `THEMES` registry → 6 CSS-variable themes + a per-user picker | 0.5–1 d | Values port directly from `lib/themes.ts`. |
| `/inventory` view (consumes `<DataGrid>` + tooltip popover) | 0.5 d | Tooltip = port `composeNotes.ts` as a render fn; now rich HTML. |
| `/gear` gear_check view | 0.5 d | Reuse grid; add status-badge cell renderer. |
| `/spells` spell_check view | 0.25 d | Reuse grid + badge. |
| `/bank` view + coin banner | 0.25 d | Reuse grid. |
| Search: page/box, client-side substring + group/collapse, port `levenshtein`/`didYouMean` | 1 d | Pure-TS logic ports verbatim; UI is new. |
| Admin write screens: Character Info, Bank Coin, Eviction, Admin Mgmt (4 forms) | 1.5–2 d | Forms + validation + auth-gated routes. Backend does the actual mutation. |
| "Last Synced" freshness coloring, frozen header, responsive polish, empty/error states | 0.5–1 d | Spread across views. |
| **Total** | **~6–9 days** | Front end only. |

**Sensitivity:** the estimate is dominated by the `<DataGrid>` component and the 4 admin forms.
If the maintainer accepts AG Grid Community (less custom wiring, heavier dep, opinionated look),
the grid item shrinks ~1 day but theming gets harder — roughly a wash. The biggest *un-budgeted*
risk is the backend/auth slice, which this estimate explicitly excludes.

---

## Recommendation

**Build the new frontend as a fully static SvelteKit app (`adapter-static`), using TanStack
Table (Svelte adapter) for the four grid views and Tailwind CSS for styling and the 6 EQ themes,
deployed to Cloudflare Pages, talking to a small JSON API.**

Rationale, against the stated constraints:

- **Low build complexity:** SvelteKit has the least boilerplate of the real frameworks, built-in
  routing/forms, and a tiny compiled bundle. No RSC, no server runtime, no virtual DOM. A
  part-time maintainer can hold the whole thing in their head.
- **TypeScript fit:** first-class TS throughout; the existing pure-logic modules
  (`searchIndex.ts`'s `levenshtein`/`didYouMean`, `composeNotes.ts`, the `THEMES` registry,
  validation rules) **port across with little to no change** — they were already framework-free.
- **Low operational burden:** static hosting on a free CDN tier = $0, no servers to patch, no
  uptime to babysit. Matches today's $0 cost and near-zero ops.
- **Right-sized:** read-mostly data, ~12 users, a few thousand rows. SvelteKit + TanStack with no
  virtualization is comfortably inside spec. Next.js and AG Grid Enterprise solve scale problems
  this project does not have.
- **Honest about the cost:** the milestone's real expense is rebuilding the four things the
  Sheet gave free — the grid, filtering, sorting, multi-user/auth. TanStack covers the grid
  engine and sorting cheaply; the per-column filter UI and the admin auth are the genuine new
  work. Budget ~6–9 frontend days plus a separate backend slice.

If the maintainer would rather not learn Svelte at all, **React + Vite + TanStack Table** is the
acceptable fallback — same architecture, slightly more boilerplate and bundle, and TanStack's
best-supported target. Either is a sound choice; SvelteKit is the better fit for *this*
maintainer and *this* app.

---

## Sources

- [TanStack Table vs AG Grid vs react-data-grid 2026 — PkgPulse](https://www.pkgpulse.com/guides/tanstack-table-vs-ag-grid-vs-react-data-grid-2026)
- [Best React Table Libraries 2026 — Simple Table](https://www.simple-table.com/blog/best-react-table-libraries-2026)
- [Next.js vs Remix vs Astro vs SvelteKit in 2026 — DEV Community](https://dev.to/pockit_tools/nextjs-vs-remix-vs-astro-vs-sveltekit-in-2026-the-definitive-framework-decision-guide-lp5)
- [Astro vs SvelteKit: Static-First vs App-First in 2026 — PkgPulse](https://www.pkgpulse.com/blog/astro-vs-sveltekit-2026)
- [SvelteKit vs Next.js vs Astro: Which Framework Wins in 2026 — Gigson](https://www.gigson.co/blog/sveltekit-vs-next-js-vs-astro-which-framework-wins-in-2026)
- Repo source read directly: `apps-script/src/tabs/{buildView,buildGearCheck,buildSpellCheck,buildBank,composeNotes}.ts`, `apps-script/src/triggers/{showSearchSidebar,showCharInfoSidebar,showEvictionSidebar,onOpen}.ts`, `apps-script/src/lib/{searchIndex,themes}.ts`, `.planning/research/ARCHITECTURE.md`, `.planning/PROJECT.md`
