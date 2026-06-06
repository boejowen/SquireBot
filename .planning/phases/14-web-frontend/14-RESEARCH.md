# Phase 14: Web Frontend - Research

**Researched:** 2026-05-30
**Domain:** Greenfield SvelteKit static SPA + Go read API extension over modernc SQLite; port of tested v1 Apps Script pure-logic (search/tooltip/theme) + reimplementation of v1 view builders in Go
**Confidence:** HIGH (stack, schema, port mechanics, both bugs verified against the repo + npm registry; one HIGH-impact correction to the UI-SPEC's assumed TanStack package)

<user_constraints>
## User Constraints (from CONTEXT.md + UI-SPEC.md)

### Locked Decisions (D-01..D-10 — do NOT re-litigate)
- **D-01 — Go backend is the query/compute layer.** Per-view read endpoints under `/api/v1/` return **ready-to-render rows**; SvelteKit client is thin (fetch JSON to render). Suggested shapes: `GET /api/v1/views/view`, `/views/gear_check`, `/views/spell_check`, `/views/bank`, plus `GET /api/v1/meta` (character list, per-char last-synced, available themes). Exact URL/field names are Claude's discretion; pagination almost certainly unneeded (tiny data).
- **D-02 — Gear/spell/view/bank progression logic reimplemented in Go**, using the tested v1 TS builders + their `__tests__` as the behavioral oracle. Go output MUST match v1 semantics exactly. Port the fixtures/expectations across so parity is provable in Go tests.
- **D-03 — Search + tooltip presentation stay client-side**, reusing the tested pure-TS modules. Client fetches `view` rows (enrichment fields inline) and runs ported `searchIndex.ts` in-browser; per-row tooltip HTML composed client-side via ported `composeNotes.ts`. Read endpoints include the raw enrichment fields each row needs (wiki summary, current price, quest info, item ID for the wiki link).
- **D-04 — Public read in P14, Discord-gate in P15.** Site + read API open to anyone with the URL; Go read API allows the static site's origin via CORS; no auth gate in P14. (P15's AUTH-08 walls it.)
- **D-05 — Keep the public site out of search engines:** `<meta name="robots" content="noindex">` + a `robots.txt` disallow. Frontend obligation.
- **D-06 — Ship 5 EQ themes** (Vanilla / Kunark / Velious / Minimalist / Heavy), drop `sheets-default`, default = `velious`. Theme = per-user `localStorage`, picker in shell. Port `THEMES` registry to CSS custom properties on `[data-theme]`, colors derived from `docs/design/eq-aesthetic-theme.md`. Heavy renders fully now (real CSS textures).
- **D-07 (discretion lean) — Self-host webfonts** (Cinzel, Cinzel Decorative, IM Fell English, MedievalSharp, Crimson Text, Inter as woff2) via `@fontsource/*` — "Off Google" ethos. Planner may override if heavy.
- **D-08 — Tooltips open on hover (pointer) + tap/click (touch)** as rich-HTML popover (wiki summary + price + quest + real `<a>` wiki link). Dismiss on outside-tap / Esc. Content via ported `composeNotes.ts`.
- **D-09 — "Did you mean?" = single clickable inline suggestion** of the best fuzzy hit, only when search has no exact match; clicking re-runs search. Fix the two carried bugs during the port: 999.28 (`didYouMean('')` empty-query) + 999.30 (`searchIndex.test.ts` Test 4 Levenshtein contract).
- **D-10 — Read endpoints extend the existing hand-rolled backend** — `net/http` ServeMux (Go 1.22+ method+pattern routing) over `modernc.org/sqlite` in `internal/backendsrv/`, following `/api/v1/...` versioning. Mirror `ingest/{handler,whoami,version}.go`; add read query methods to the `store` package. No new web framework on the backend.

### Claude's Discretion
- Exact read-endpoint URL shapes, JSON field names, pagination/streaming (data is tiny — likely none).
- TanStack Table config specifics — but the UI-SPEC Grid Contract already locks column order/names/sort/filter, so "discretion" here is mostly mechanics.
- Self-hosted vs CDN fonts if D-07 self-hosting proves impractical.
- Whether search runs over the `view` payload already in memory or a dedicated dataset (D-03 leans in-memory).

### Deferred Ideas (OUT OF SCOPE for P14)
- Discord-login gate on read access to P15 (AUTH-08). P14 is public.
- Admin write forms (eviction / bank-coin / admin-mgmt) to P15. Coin values are null/0 in P14 until P15's bank-coin form (ADMIN-05).
- Fancy theme-picker preview tiles (Sheet's 3x2 live-preview grid) — optional polish; ship a simple dropdown/swatch picker.
- Cutover / shadow-soak / Sheet decommission to P16.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| BACKEND-05 | Versioned read API powers the 4 views (replaces Sheet view tabs as query layer) | Go Read API + Per-View SQL Read Queries. Extend the existing ServeMux (`main.go:258-260`) with 5 `GET /api/v1/...` handlers mirroring `whoami.go`'s read-only pattern; add read methods to `store`. |
| WEB-01 | Static frontend renders 4 consolidated views with leading Char column, filterable + sortable | SvelteKit Static App + Grid (TanStack). One reusable `DataGrid` over `@tanstack/table-core` (NOT `@tanstack/svelte-table` — see Pitfall 1) instantiated 4x. |
| WEB-02 | gear_check OK/MISSING/OTHER vs Velious tiers; spell_check KNOWN/MISSING — matching v1 | Porting the View Builders to Go (D-02). Exact algorithms documented from `buildGearCheck.ts` / `buildSpellCheck.ts`; fixtures port across as the parity oracle. |
| WEB-03 | Cross-character fuzzy search + "did you mean?" (Wagner-Fischer Levenshtein), <2s | Search Port + Two Carried Bugs. Port `searchIndex.ts` levenshtein/didYouMean to client TS; fix 999.28 + 999.30. ~12 users so <2s trivial. |
| WEB-04 | Per-item tooltip (wiki summary + price + quest) as rich HTML + direct wiki link | Tooltip Port (composeNotes to rich HTML) + Trust Boundary. Rewrite `composeNotes.ts` to emit escaped HTML (was plain-text). |
| WEB-05 | EQ aesthetic theme site-wide per `eq-aesthetic-theme.md` | Theme Port. `THEMES` registry to CSS custom properties on `[data-theme]`; 5 themes, velious default, localStorage; self-hosted `@fontsource` woff2. |
</phase_requirements>

## Summary

P14 is the repo's first frontend. It has two clean halves with a narrow contract between them: (a) a versioned Go read API that extends the already-live hand-rolled `net/http` backend, and (b) a greenfield static SvelteKit app. The backend half is low-risk — the schema (P11), enrichment data (P12), ServeMux wiring, store transaction patterns, and read-only handler precedent (`whoami.go`) all already exist; the work is adding 5 read handlers + a handful of `SELECT ... JOIN` store methods, plus reimplementing the four v1 view-builder algorithms (gear/spell/view/bank) in Go with the v1 vitest expectations ported across as a parity oracle. The frontend half is genuinely new: scaffold SvelteKit with `adapter-static` in SPA-fallback mode (no SSR, no prerendered data), build a reusable filterable/sortable grid, port three tested pure-TS modules (`searchIndex`, `composeNotes`, `themes`) to client TS, and wire the 5-theme CSS-custom-property system with self-hosted fonts.

The single highest-value finding: the UI-SPEC's assumed `@tanstack/svelte-table` package is Svelte-4-only and will not work with Svelte 5 (verified: its published peer dep is `svelte: "^4.0.0 || ^3.49.0"`, last published 2022, imports `svelte/internal`; the v9 line is still alpha). The correct, well-trodden Svelte 5 path is `@tanstack/table-core` directly with a tiny local `createSvelteTable` + `FlexRender` adapter — exactly the shadcn-svelte data-table pattern. This is a swap of package, not of capability (sticky columns, faceted filters, multi-key sort all still work via table-core), but the planner must not write a task that installs `@tanstack/svelte-table` and expects it to compile under Svelte 5. The two carried bugs (999.28, 999.30) are both fully characterized below with the exact intended fix.

**Primary recommendation:** Two parallelizable build streams. Backend stream: add `store` read-query methods + 5 `GET /api/v1/...` handlers (mirror `whoami.go`) + stdlib CORS middleware (D-04) + a Go reimplementation of the 4 builders proven against ported v1 fixtures (D-02/WEB-02). Frontend stream: scaffold `web/` (SvelteKit + `adapter-static` SPA fallback + Tailwind v4 via `@tailwindcss/vite`), build `DataGrid` over `@tanstack/table-core` (NOT svelte-table) with a local FlexRender adapter, port `searchIndex`/`composeNotes`/`themes` to client TS (fixing 999.28 + 999.30 and rewriting composeNotes to escaped HTML), and emit the 5-theme CSS-var system with `@fontsource` woff2. Streams meet at the JSON contract.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Per-view row computation (view/bank join, gear OK/MISSING/OTHER, spell KNOWN/MISSING) | API / Backend (Go) | — | D-01/D-02: server is the compute layer; relational joins + progression logic live in Go over SQLite. Keeps payloads small and centralizes v1 parity. |
| Serving ready-to-render JSON rows | API / Backend (Go) | — | D-01: `/api/v1/views/*` return rows with enrichment fields inline (D-03). |
| CORS allow of the static origin | API / Backend (Go) | — | D-04: stdlib middleware; the static site is a different origin from `api.squirebot.quest`. |
| Grid rendering, sort, filter (incl. faceted), sticky col/header | Browser / Client (Svelte) | — | WEB-01: client-side over fetched rows; data tiny (~12 users) so no server pagination/sort. |
| Cross-character fuzzy search + "did you mean?" | Browser / Client (Svelte) | — | D-03: runs over the already-fetched `view` payload in-memory; ported `searchIndex.ts`. |
| Item tooltip HTML composition + escaping | Browser / Client (Svelte) | — | D-03/D-08: ported `composeNotes.ts` builds escaped rich HTML from payload fields. |
| Theme selection + persistence | Browser / Client (Svelte) | — | D-06: `[data-theme]` attribute + `localStorage`; no server write. |
| Static asset hosting (HTML/JS/CSS/woff2) | CDN / Static (Cloudflare/GH Pages) | — | Off-Google static deploy; API stays on the Hetzner box behind Caddy. |
| Dimension/enrichment data (item_master, pigparse_price, wiki_spells, wiki_gear_tier, quest_items) | Database / Storage (SQLite, P12-populated) | API | The read joins consume these existing tables; P14 adds no enrichment writes. |
| noindex / robots.txt | CDN / Static + Client | — | D-05: meta tag (client) + `static/robots.txt` (static asset). |

**Sanity-check note for the planner:** there is a recurring misassignment risk — do NOT put progression/join logic in the browser (it belongs in Go per D-01/D-02), and do NOT put search/tooltip/theme logic in Go (it belongs in the client per D-03/D-06). The split is deliberate and locked.

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `@sveltejs/kit` | 2.61.1 | App framework (routing, build, static adapter) | [VERIFIED: npm view] Locked stack (PROJECT.md/CONTEXT). |
| `svelte` | 5.56.0 | Component runtime (runes reactivity) | [VERIFIED: npm] Svelte 5 is current; runes (`$state`/`$derived`) are the reactivity model the table adapter integrates with. |
| `@sveltejs/adapter-static` | 3.0.10 | Static/SPA build output | [VERIFIED: npm] The locked deploy target is static (Cloudflare/GH Pages). |
| `@tanstack/table-core` | 8.21.3 | Headless table engine (sort/filter/facet logic) | [VERIFIED: npm] Use THIS, NOT `@tanstack/svelte-table` — see Pitfall 1. Framework-agnostic core; pair with a local Svelte adapter. |
| `tailwindcss` | 4.3.0 | Utility CSS | [VERIFIED: npm] Locked. v4 is CSS-first (no `tailwind.config.js`, no PostCSS) — see Tailwind v4 pitfall. |
| `@tailwindcss/vite` | ships with tailwindcss 4 | Tailwind v4 Vite plugin | [CITED: tailwindcss.com/docs/guides/sveltekit] v4's integration is a Vite plugin, not a PostCSS plugin. |
| `vite` | 8.0.14 | Bundler/dev server (under SvelteKit) | [VERIFIED: npm] SvelteKit 2 runs on Vite. |
| `vitest` | 4.1.7 | Test runner for ported client TS | [VERIFIED: npm] Same runner the apps-script side already uses (v1.6 there; the new `web/` package can use current v4). |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `@fontsource/cinzel` | 5.2.8 | Self-hosted Cinzel woff2 | D-07 self-host. vanilla/kunark/minimalist display font. [VERIFIED: npm] |
| `@fontsource/cinzel-decorative` | 5.2.8 | Self-hosted Cinzel Decorative | velious display font. [VERIFIED: npm] |
| `@fontsource/im-fell-english` | 5.2.6 | Self-hosted IM Fell English | velious/kunark/heavy body-accent. [VERIFIED: npm] |
| `@fontsource/medievalsharp` | 5.2.8 | Self-hosted MedievalSharp | heavy display font. [VERIFIED: npm] |
| `@fontsource/crimson-text` | 5.2.7 | Self-hosted Crimson Text | vanilla body font. [VERIFIED: npm] |
| `@fontsource/inter` | 5.2.8 | Self-hosted Inter | body/UI everywhere; minimalist body. [VERIFIED: npm] |
| `@lucide/svelte` | 1.17.0 | Tree-shakeable SVG icons | UI-SPEC icon library. Use `@lucide/svelte` (the Svelte-5 package), NOT `lucide-svelte` (deprecated, stuck at 1.0.1). [VERIFIED: npm] |

### Backend (already present — no new deps)

| Library | Version | Purpose | Notes |
|---------|---------|---------|-------|
| `modernc.org/sqlite` | 1.51.0 | Pure-Go SQLite driver (no cgo) | [VERIFIED: go.mod] Driver name is `"sqlite"` (NOT `"sqlite3"`). Read queries use the same `*sql.DB` from `store.Open`. |
| Go stdlib `net/http` | toolchain go1.25.7 (go1.26.2 installed) | ServeMux + handlers | [VERIFIED: go.mod + go version] Method+pattern routing (`GET /api/v1/views/view`). No new web framework (D-10). |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `@tanstack/table-core` + local adapter | `@tanstack/svelte-table` (the obvious package) | REJECTED — does not work with Svelte 5 (peer dep `svelte ^4`; imports `svelte/internal`). The UI-SPEC named it; this research corrects it. [VERIFIED: npm view @tanstack/svelte-table@8.21.3 peerDependencies] |
| `@tanstack/table-core` + local adapter | `@tanstack/svelte-table@9.x` (future official Svelte-5 adapter) | REJECTED — still alpha (9.0.0-alpha.51 on 2026-05-29). Do not pin a guild-facing site to an alpha. [VERIFIED: npm view @tanstack/svelte-table time] |
| `@tanstack/table-core` + local adapter | shadcn-svelte `data-table` component (copy-in) | Viable and equivalent — shadcn-svelte's data-table IS exactly this pattern (local `createSvelteTable`+`FlexRender` over table-core). UI-SPEC marks shadcn N/A as a registry gate, but copying the ~40-line adapter idiom (not the registry) is fine. Hand-rolling the tiny adapter avoids any registry dependency. [CITED: shadcn-svelte.com/docs/components/data-table] |
| Stdlib CORS | `rs/cors` middleware | Stdlib is trivial for one known origin (D-04); adding a dep contradicts the backend's stdlib-only posture (11-04 notes). Hand-roll a ~10-line middleware. [ASSUMED] |
| `adapter-static` SPA fallback | Prerendering each view route | Views are data-driven from a live API; prerendering buys nothing (data isn't known at build) and risks the cross-origin-fetch-during-prerender failure mode. SPA fallback is correct. [CITED: svelte.dev/docs/kit/single-page-apps] |

**Installation (frontend, in the new `web/` dir):**
```bash
# scaffold (npx sv = the current Svelte CLI; pick: SvelteKit minimal, TypeScript, Tailwind, Vitest)
npx sv create web
cd web
npm install @tanstack/table-core @lucide/svelte
npm install @fontsource/cinzel @fontsource/cinzel-decorative @fontsource/im-fell-english \
            @fontsource/medievalsharp @fontsource/crimson-text @fontsource/inter
npm install -D @sveltejs/adapter-static
```

**Version verification (run before pinning in package.json):**
```bash
npm view @sveltejs/kit version          # 2.61.1 (2026-05, verified)
npm view svelte version                  # 5.56.0 (verified)
npm view @tanstack/table-core version    # 8.21.3 (verified)
npm view tailwindcss version             # 4.3.0  (verified — v4, CSS-first)
npm view @tanstack/svelte-table peerDependencies   # {svelte:^4||^3.49} => DO NOT USE under Svelte 5
```

## Architecture Patterns

### System Architecture Diagram

```
                         +----------------------- Cloudflare / GitHub Pages (static) ----------------------+
  guildie browser ------>|  index.html + JS bundle + app.css + /fonts/*.woff2 + robots.txt(noindex)        |
                         |                                                                                  |
                         |  SvelteKit SPA (adapter-static, ssr=false, prerender=false, fallback=200.html)  |
                         |    SiteShell [data-theme=<localStorage|velious>]                                 |
                         |      |- ThemePicker --(set [data-theme] + localStorage)                          |
                         |      |- view nav/tabs --> 4x DataGrid (one per view)                             |
                         |      |      DataGrid = @tanstack/table-core + local createSvelteTable/FlexRender |
                         |      |        sort (multi-key default) . global filter . per-col+faceted filter  |
                         |      |        sticky Char col + sticky header . no pagination                    |
                         |      |- SearchBox/Results --> ported searchIndex.ts (in-memory over view rows)   |
                         |      |        levenshtein/didYouMean  [FIX 999.28 empty-query, 999.30 Test4]      |
                         |      +- ItemTooltip --> ported composeNotes.ts -> ESCAPED rich HTML (hover+tap)  |
                         +---------------+------------------------------------------------------------------+
                                         |  fetch JSON  (CORS: Access-Control-Allow-Origin: <static origin>)
                                         v
        +---------------- Hetzner VPS . Caddy auto-HTTPS . api.squirebot.quest -----------------+
        |  squirebot-server (Go, single binary)  net/http ServeMux  [+ CORS middleware, D-04]   |
        |    EXISTING:  POST /api/v1/ingest        GET /api/v1/whoami                            |
        |    NEW (P14): GET /api/v1/meta                                                         |
        |               GET /api/v1/views/view         -+                                        |
        |               GET /api/v1/views/gear_check    |  read-only handlers, mirror whoami.go  |
        |               GET /api/v1/views/spell_check   |  (no bearer guard in P14 -- D-04)       |
        |               GET /api/v1/views/bank         -+                                         |
        |                         | calls store read methods (parameterized SELECT ... JOIN)     |
        |                         v                                                               |
        |   store: view/bank rows = inventory_item join item_master join pigparse_price          |
        |                            join quest_items (+ character for Char/last_seen)            |
        |          gear_check rows = Go reimpl of buildGearCheck (char x wiki_gear_tier           |
        |                            x inventory_item, slot-pair match)                           |
        |          spell_check rows = Go reimpl of buildSpellCheck (char x wiki_spells            |
        |                            x spellbook_entry on normalized_name, lvl<=char)             |
        +---------------------------------------+-------------------------------------------------+
                                                v
                         SQLite squirebot.db (single-writer; WAL; P11 schema, P12-populated)
```

### Recommended Project Structure

The repo today is `apps-script/` (TS) + Go (`cmd/`, `internal/`). The web app is a NEW top-level `web/` dir — short, conventional, parallel to `apps-script/`, and obviously "the website." (Rejected: `frontend/` generic; `site/` collides with deploy nomenclature; nesting under `apps-script/` is wrong — different toolchain.)

```
web/                                  # NEW top-level dir (its own package.json + node_modules)
  svelte.config.js                    # adapter-static + fallback: '200.html'
  vite.config.ts                      # plugins: [tailwindcss(), sveltekit()]
  tsconfig.json
  package.json
  vitest.config.ts                    # or test block in vite.config
  src/
    app.html                          # <meta name="robots" content="noindex"> (D-05)
    app.css                           # @import "tailwindcss"; + @fontsource imports; [data-theme] var blocks
    routes/
      +layout.ts                      # export const ssr = false; export const prerender = false; (SPA)
      +layout.svelte                  # SiteShell (theme attr, nav, footer attribution)
      +page.svelte                    # the 4 views (tabbed) + SearchBox
    lib/
      api.ts                          # fetch wrappers for /api/v1/views/* + /meta; PUBLIC_API_BASE env
      table/                          # local TanStack adapter (the shadcn-svelte idiom)
        createSvelteTable.ts          # wraps @tanstack/table-core w/ Svelte 5 runes
        FlexRender.svelte             # renders header/cell columnDef content
      search/searchIndex.ts           # PORTED from apps-script (levenshtein/didYouMean) — bugs fixed
      tooltip/composeNotes.ts         # PORTED -> escaped rich HTML
      theme/themes.ts                 # PORTED THEMES registry -> CSS-var emitter / [data-theme] keys
      components/{DataGrid,StatusCell,ItemTooltip,SearchBox,SearchResults,ThemePicker,StateBlock}.svelte
      __tests__/                      # vitest: searchIndex.test.ts (ported+fixed), composeNotes.test.ts, themes.test.ts
    __fixtures__/                     # ported view/search fixtures shared with Go parity tests where useful
  static/
    robots.txt                        # Disallow: / (D-05)

internal/backendsrv/
  readapi/                            # NEW package (or fold into ingest/) — the 5 read handlers + CORS
    views.go                          # GET /api/v1/views/{view} handlers (mirror whoami.go read-only shape)
    meta.go                           # GET /api/v1/meta
    cors.go                           # stdlib CORS middleware (allow static origin, D-04)
  store/
    readviews.go                      # NEW: SELECT...JOIN read methods for view/bank + raw rows for gear/spell
    (existing db.go/replace.go/binding.go/enrich.go/itemids.go)
  compute/                            # NEW: Go reimpl of the 4 builders (D-02) + parity tests vs ported fixtures
    gearcheck.go / gearcheck_test.go
    spellcheck.go / spellcheck_test.go
    view.go / bank.go (+ _test.go)
    eqconst.go                        # WIKI_SLOT_TO_INV_SLOTS, tier sort — NOTE enrich/eqconst.go already exists; reuse/extend
```

> **Reuse note:** `internal/backendsrv/enrich/eqconst.go` already exists (P12). Check whether `WIKI_SLOT_TO_INV_SLOTS` and class/race constants live there before duplicating; the gear_check port needs the same slot-pair map (`eq-constants.ts` WIKI_SLOT_TO_INV_SLOTS).

### Pattern 1: Read-only handler mirroring `whoami.go` (minus the bearer guard)

**What:** Each `GET /api/v1/views/*` is a thin handler: method-guard, call a `store` read method, JSON-encode. P14 is public (D-04) so there is NO `ResolveToken` call — the only structural difference from `whoami.go`.
**When to use:** All 5 new endpoints.
**Example (shape, mirroring the verified whoami.go idiom):**
```go
// Source: internal/backendsrv/ingest/whoami.go (read-only handler precedent), adapted (no guard, D-04)
func (h *ViewsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    rows, err := h.store.ViewRows(r.Context())      // parameterized SELECT...JOIN in store
    if err != nil {
        slog.Error("views read failed", "view", "view", "err", err)   // never echo row content (V7)
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(rows)
}
```
Register exactly like the existing routes in `cmd/squirebot-server/main.go` (around lines 258-260):
```go
// Source: cmd/squirebot-server/main.go:258-260 (existing pattern)
mux.Handle("GET /api/v1/views/view",        readapi.NewViews(st, "view"))
mux.Handle("GET /api/v1/views/gear_check",  readapi.NewViews(st, "gear_check"))
// ... wrap the whole mux in CORS middleware before &http.Server{Handler: cors(mux)}
```

### Pattern 2: stdlib CORS middleware (D-04)

**What:** A `func(http.Handler) http.Handler` that sets `Access-Control-Allow-Origin` to the static site origin and short-circuits `OPTIONS` preflights with 204. The static site and the API are different origins, so the browser will preflight any non-simple request and block responses lacking the header.
**When to use:** Wrap the whole `mux` (or just the read routes) in `runServe`.
**Example:**
```go
// Source: stdlib net/http (MDN CORS semantics) — hand-rolled, no dependency
func cors(allowOrigin string, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", allowOrigin) // exact origin, not "*" (future-proof for credentials in P15)
        w.Header().Set("Vary", "Origin")
        w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusNoContent)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```
> **Caddy note:** Caddy fronts 443 and reverse-proxies to loopback `127.0.0.1:8090` (`main.go:50`). CORS is an application-layer response-header concern; set it in Go (above), not in Caddy, so it travels with the handler. [ASSUMED] that Caddy is not itself stripping/duplicating CORS headers — verify on the box, since a duplicated `Access-Control-Allow-Origin` (Caddy + Go both setting it) causes the browser to reject the response.

### Pattern 3: Local Svelte-5 TanStack adapter (`createSvelteTable` over `table-core`)

**What:** A ~30-40 line shim that wraps `@tanstack/table-core`'s framework-agnostic `createTable` with Svelte 5 runes, plus a `FlexRender` component for header/cell rendering. This IS the shadcn-svelte data-table idiom; we vendor it locally (no registry).
**When to use:** Backing the single reusable `DataGrid`.
**Example (the verified working Svelte 5 pattern):**
```svelte
<!-- Source: shadcn-svelte.com/docs/components/data-table + jamesoclaire.com (Svelte5+TanStack v8) -->
<script lang="ts">
  import { getCoreRowModel, getSortedRowModel, getFilteredRowModel,
           type SortingState, type ColumnFiltersState } from '@tanstack/table-core';
  import { createSvelteTable, FlexRender } from '$lib/table';   // local adapter
  let { data, columns } = $props();
  let sorting = $state<SortingState>([{ id: 'char', desc: false }]);          // default Char asc (UI-SPEC)
  let columnFilters = $state<ColumnFiltersState>([]);
  let globalFilter = $state('');
  const table = createSvelteTable({
    get data() { return data; },
    columns,
    state: {
      get sorting() { return sorting; },
      get columnFilters() { return columnFilters; },
      get globalFilter() { return globalFilter; },
    },
    onSortingChange: (u) => (sorting = typeof u === 'function' ? u(sorting) : u),
    onColumnFiltersChange: (u) => (columnFilters = typeof u === 'function' ? u(columnFilters) : u),
    onGlobalFilterChange: (u) => (globalFilter = typeof u === 'function' ? u(globalFilter) : u),
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    enableMultiSort: true,            // multi-key default sort (UI-SPEC secondary sorts)
  });
</script>
<!-- header cells: <FlexRender content={header.column.columnDef.header} context={header.getContext()} /> -->
<!-- body cells:   <FlexRender content={cell.column.columnDef.cell}   context={cell.getContext()} /> -->
```
**Sticky Char column + sticky header** are pure CSS on the rendered table (`position: sticky; left: 0` on the first cell/`th`; `position: sticky; top: 0` on the header row) — TanStack is headless, so styling is yours. **Faceted filter** for Status/Tier/Class uses `getFacetedRowModel` + `getFacetedUniqueValues` from table-core to populate the select options.

### Anti-Patterns to Avoid

- **`npm install @tanstack/svelte-table`** under Svelte 5 — it imports `svelte/internal` and won't compile/run. Use `@tanstack/table-core` + local adapter. (Pitfall 1.)
- **Per-character view tabs / multiple grid components.** CLAUDE.md LOCKED: views are consolidated mega-grids with a leading `Char` column. One reusable `DataGrid` instantiated 4x.
- **`{@html composeNotes(...)}` on un-escaped content.** The v1 `searchIndex.ts` header is explicit: user/wiki-controlled strings (item names, locations, quest names, summary) are the presentation layer's responsibility to escape. The ported `composeNotes.ts` must escape every interpolated string before producing HTML (UI-SPEC Tooltip "Safety").
- **Double-decoding CP1252.** Encoding contract A1 (STATE.md, Plan 11-03): the CP1252-to-UTF-8 decode happens once, on the watcher's disk-read side. The DB already stores UTF-8; the read API returns UTF-8 JSON; the client must not re-decode. (Pitfall 4.)
- **Prerendering data-driven view routes.** Data comes from a live API at runtime; `prerender=true` buys nothing and triggers cross-origin-fetch-during-prerender failures. Use `ssr=false; prerender=false` (SPA).
- **`USER_ENTERED`/formula thinking.** No `=HYPERLINK(...)` cells — those were Sheet artifacts (`buildView.ts:108`). The web emits real `<a href>`. Drop the formula-string machinery entirely.
- **Driver name `"sqlite3"`** in any new Go read code — modernc registers `"sqlite"`. (goose uses `"sqlite3"` as its dialect string; they differ on purpose — `db.go:20-22`.)

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Table sort/filter/facet engine | Custom sort/filter loops in Svelte | `@tanstack/table-core` (+ local adapter) | Faceted uniques, multi-key sort, filter fns, column state — all battle-tested; the UI-SPEC already specs TanStack behavior. |
| Levenshtein / fuzzy "did you mean?" | A new edit-distance impl | Port the v1 `searchIndex.ts` (Wagner-Fischer) | It's tested and is the WEB-03 oracle; reimplementing risks diverging from v1 + reintroducing the very bugs we're fixing. |
| Tooltip content composition | Ad-hoc string building in the component | Port `composeNotes.ts` (rewrite to escaped HTML) | The price/quest/summary ordering + max-5-links rule is already specified and tested; only the output format changes (text to HTML). |
| Theme token system | Inline per-component colors | Port `THEMES` to CSS custom properties on `[data-theme]` | Single-attribute theme swap; colors stay derived from `eq-aesthetic-theme.md` per CLAUDE.md. The picker mockup is already CSS-var structured. |
| Self-hosted webfonts | Manually downloading + @font-face | `@fontsource/*` woff2 packages | npm-managed, versioned, Off-Google; import in `app.css`. |
| CORS | A CORS library | ~10-line stdlib middleware | One known origin; matches the backend's stdlib-only posture. |
| SQLite read driver | Anything cgo | `modernc.org/sqlite` (already a dep) | Pure-Go, already the project's driver; static cross-compile preserved. |
| Atomic view computation in SQL | Materialized view tables / triggers | Compute on read in Go (D-01) | Data tiny; the Sheet's onChange/rebuild/cache machinery is explicitly NOT rebuilt (CONTEXT Established Patterns). |

**Key insight:** P14's "new code" is almost entirely glue + one Go reimplementation. The differentiators (search, tooltips, themes, progression logic) all exist and are tested in v1; the discipline is porting with provable parity, not green-field invention.

## Porting the View Builders to Go (D-02 / WEB-02)

The four builders are pure transforms over the dimension/landing data. In v1 they read Sheet tabs; in Go they read the equivalent SQLite tables. The algorithms below are the contract; the v1 vitest expectations are the parity oracle (port the fixtures + expected outputs).

### `view` (WEB-01) — buildView.ts
- **Source rows:** every `inventory_item` row joined to `item_master` (wiki_url, wiki_summary, is_quest_item), `pigparse_price` (price), `quest_items` (links), and `character` (Char name + last_seen).
- **Price pick (`pickPrice`, buildView.ts:259-265):** prefer WTS (direction WTS, a30>0) then WTB (a30>0) then empty. **Schema caveat:** v1 `_pigparse.direction` was numeric (0=WTS,1=WTB,2=?); the SQLite `pigparse_price.direction` is TEXT (the P12 job stores `strconv.Itoa(t)`, so "0"/"1"/...). The Go pick must compare the stringified direction, and `a30`/`t30` are real columns added in `00003`. Verify the exact stored direction values against `pigparse_price` data on the box before finalizing the comparison.
- **Sort:** Char asc then item asc then location asc.
- **Columns (UI-SPEC, exact):** `Char . Slot . Item . ID . Count . Wiki . Price . Last Synced`. `Wiki` is now a real URL (not `=HYPERLINK`); `Last Synced` is the ISO string (freshness coloring is client-side per UI-SPEC).
- **Enrichment inline (D-03):** the row JSON must also carry `wiki_summary`, the price detail (WTS/WTB a30+t30), `is_quest_item`, and quest links, so the client `composeNotes.ts` can build the tooltip without a second fetch.

### `bank` (WEB-01) — buildBank.ts
- Same join/shape as `view` but scoped to the bank toon's inventory. **Bank toon identity:** v1 read `_meta.bank_toon_name`; in the DB world the equivalent is `character.is_bank_toon = 1`. Use that flag (the `_meta` row doesn't exist in SQLite).
- **Coin is null/0 in P14** (deferred — ADMIN-05 fills it in P15). The endpoint should return a coin object as null/absent; the client renders "Coin: not yet recorded" (UI-SPEC copy) — never fabricate `0 pp` as real data.

### `gear_check` (WEB-02) — buildGearCheck.ts (the load-bearing parity case)
- **Inputs:** characters with `class` set (+ `race`); `wiki_gear_tier` rows grouped by (tier, class); each char's `inventory_item` rows.
- **Tiers shown:** `Velious Pre-Raid/Group` + `Velious Raiding` always; add `Iksar` iff `race == 'IKS'`.
- **Slot-pair match (the subtle part):** wiki uses prose slots (`Ears`/`Fingers`/`Wrists`/...); inv uses tokens (`EAR1`+`EAR2`/...). `WIKI_SLOT_TO_INV_SLOTS` (eq-constants.ts:54-72) maps each wiki slot to one-or-two inv slots; a char is OK if the recommended item (case-insensitive name match) is in EITHER slot of the pair.
- **Status per (char,tier,slot,recommendation):** `OK` (recommended item equipped) then `OTHER` (something else occupies the inv slot(s)) then `MISSING` (slot empty). Order matters: matched => OK; not matched but slot non-empty => OTHER (Have = first item in slot); slot empty => MISSING.
- **Sort:** Char asc then tier rank (Pre-Raid=1, Raiding=2, Iksar=3) then slot asc then recommended asc.
- **Columns:** `Char . Class . Tier . Slot . Have . Recommended . Status`.
- **Parity oracle:** buildGearCheck.test.ts — port its fixtures (char_owner metadata, wiki_gear_tier rows, inv rows) and expected (status, have) tuples into Go table-tests.

### `spell_check` (WEB-02) — buildSpellCheck.ts
- **Inputs:** characters with `class` + `level>=1`; `wiki_spells` filtered by class AND `level <= char.level`; each char's `spellbook_entry` known set.
- **Join key:** `normalized_name` = `lower(trim(name))`. **This is already materialized in the DB:** `spellbook_entry.normalized_name` (replace.go:169) and `wiki_spells.normalized_name` (enrich.go:248) are both written with the identical `strings.ToLower(strings.TrimSpace(...))` expression — so the Go spell_check join is a direct equality on `normalized_name`, no recompute. (Cleaner than v1, which normalized at read time.)
- **Status:** `KNOWN` if the char's spellbook contains the normalized name, else `MISSING`.
- **Sort:** Char asc then level asc then spell asc.
- **Columns:** `Char . Class . Level . Spell . Status`.

### Porting fixtures so parity is provable
- The v1 fixtures are vitest `seed*` arrays inside the `.test.ts` files (e.g. buildGearCheck.test.ts seeds `_char_owner`/`_wiki_gear_tier`/`inv:*` arrays + asserts row tuples). Two viable approaches:
  1. **Translate the seed arrays into Go table-tests** (recommended — small, explicit, no cross-runtime tooling). Each Go test inserts the same rows into a temp SQLite DB (use the existing `store/testhelper.go` temp-DB helper) and asserts the same output tuples.
  2. **Export the v1 expected outputs to JSON** (run the TS builder in Node against a fixture, dump rows) and assert the Go builder produces byte-identical rows — the technique P12 used for parser parity (12-02 ran the TS parsers in Node for exact-count parity). Heavier but maximally faithful.
- **Recommendation:** approach (1) for the discrete-logic builders (gear/spell), with a handful of approach-(2) golden cases for view/bank if the join surface is wide. Either way the acceptance bar is: same input => same status/order as the v1 tests assert.

## The Two Carried Bugs (D-09 / 999.28 + 999.30)

Both live in `apps-script/src/lib/searchIndex.ts` + its test; both must be fixed during the port to client TS. Current behavior is documented from the verified source.

### 999.28 — didYouMean('') empty-query contract
- **Current behavior (searchIndex.ts:89-97):** `didYouMean('')` lowercases to `''`, computes `levenshtein('', name.toLowerCase())` = `name.length` for each candidate, filters `d <= 2 && d > 0`. So an empty query returns every 1-2-character item name as a "suggestion" — nonsensical (an empty query has nothing to "mean").
  - Why it's currently masked: `runSearch` guards empty queries at line 308 (`if (!q) return {...suggestions:[]}`), so the production path never calls `didYouMean('')`. But the function's own contract is wrong, and the web port will call `didYouMean` from a fresh component — the guard won't be inherited unless re-implemented.
- **Intended fix:** make `didYouMean` defensive at its own boundary — `if (!query || !query.trim()) return [];` as the first line. (Keeps the production guard AND fixes the unit contract.) Add a test: `expect(didYouMean('', names)).toEqual([])`.

### 999.30 — searchIndex.test.ts Test 4 Levenshtein contract mismatch
- **Current state (searchIndex.test.ts:107-110):** Test 4 is `it.skip(...)` with a plan-locked assertion that is arithmetically inconsistent with whole-string Levenshtein: it asserts `didYouMean('clok', ['Cloak of Confusion','Cloak of Flames','Sword of X','Cloak Pin'])` equals `['Cloak of Confusion','Cloak of Flames']`, but `levenshtein('clok','cloak of confusion') >= 13` — far above the `<=2` cutoff — so the real function returns `[]`. The test was skipped (not deleted) as a fail-loud regression catcher with two documented fix options (test comment lines 94-106).
- **The two documented fix paths (pick one in the port):**
  - **(a) Cheapest — correct the assertion to the real contract.** Whole-string Levenshtein is the intended algorithm; the test was wrong. Un-skip, change the expectation to `toEqual([])` (or to single-word candidates where the math holds, as Test 4b already does), and rename. This keeps `didYouMean` simple (whole-string distance, `<=2`, exclude exact, cap 3) — fine because the v1 production behavior (Test 15: `didYouMean('clok', ...)` over a corpus containing the single word `'Cloak'` => suggests `'Cloak'`) already works for real single-word item names.
  - **(b) First-word-aware matching** so multi-word candidates score against the query's nearest token. More faithful to "did you mean Cloak of Confusion?" intent, but a behavior change that needs its own tests and could surprise.
- **Recommendation:** (a). The core value framing ("which char/location holds this item") and the production test (Test 15) are satisfied by whole-string distance; the bug is a wrong test assertion, not a wrong algorithm. Document the choice in the port's commit. (If the user later wants smarter multi-word suggestions, that's a clean follow-up.)
- **Verification:** the ported searchIndex.test.ts must have NO `.skip` on the didYouMean cases — that's the SC for 999.30 being resolved.

## Tooltip & Search & Theme Port Details

### composeNotes.ts -> escaped rich HTML (WEB-04 / D-08)
- **v1 output is plain text** (composeNotes.ts:64 joins parts with newlines). The web port changes the output format to HTML while preserving the content order (UI-SPEC Tooltip): item name + wiki `<a>` then wiki summary `<p>` then price lines (WTS "Recent ask" / WTB "Buy posts" / or "No recent transactions on PigParse.") then quest flag then "Used in quests: ..." (max 5).
- **Trust boundary (carry forward verbatim from searchIndex.ts:15-18):** every interpolated string (item/char/location/quest names, summary) is user/wiki-controlled and MUST be HTML-escaped at the presentation layer before injection. The port adds an `escapeHtml()` and applies it to every dynamic value; only the structural HTML is literal. No `{@html}` on raw values. This is an ASVS V5 output-encoding control (see Security Domain).
- **Input fields needed in the payload (D-03):** `wiki_summary`, `is_quest_item`, the WTS/WTB price detail (`a30`+`t30` for each direction), and the quest-link list — all of which the view/bank read endpoints must include inline.

### searchIndex.ts -> client TS (WEB-03 / D-03)
- Port `levenshtein` (verbatim — it's correct), `didYouMean` (with the 999.28 fix), and the grouping/collapse logic (`groupAndSort`, `COLLAPSE_THRESHOLD=5`). **Drop the Apps Script I/O** (`CacheService`, `PropertiesService`, `getActiveSpreadsheet`, `prewarmSearchCache`, `getRecentSearches`/`pushRecentSearch`, `enrichResults` reading Sheet tabs) — the web runs over the already-fetched `view` rows in memory.
- **Results shape (UI-SPEC Search):** per matched item — item name (+id), then one line per holder, surfacing which character(s)/location(s) hold it; groups >5 holders auto-collapse. Each result name is itself a tooltip trigger + wiki link.
- **<2s (WEB-03):** trivial — ~12 users x ~10 chars x ~150 rows is a few thousand in-memory rows; substring scan + Levenshtein over distinct names is sub-millisecond. No cache/prewarm needed.

### themes.ts -> CSS custom properties (WEB-05 / D-06)
- **v1 registry is Sheet-API-shaped** (`headerBg`/`rowFg`/`setFontFamily` calls). The web port keeps the 5 theme keys + the velious default, but token consumption changes: emit a CSS block per `[data-theme="<key>"]` with custom properties (`--bg`, `--panel`, `--text`, `--accent`, `--status-ok/missing/other`, `--font-display`, `--font-body`, `--weight-display`). The authoritative token values are in the UI-SPEC Theme Catalog (already ported from themes.ts + the mockup) — use those, not the dimmer v1 themes.ts palette (which predates the richer mockup values).
  - Caveat for the planner: the v1 `apps-script/src/lib/themes.ts` `DEFAULT_THEME` is `minimalist` and includes `sheets-default` — both are superseded by D-06 (default `velious`, drop `sheets-default`). Use the UI-SPEC catalog as the source of truth; CLAUDE.md's "derive from `eq-aesthetic-theme.md`" rule is satisfied because the UI-SPEC catalog itself derives from that doc.
- **Per-theme font + weight override:** minimalist uses `--weight-display: 400` (UI-SPEC Typography note); all others 600. Heavy uses MedievalSharp + parchment data rows (light rows on dark frame — the one inverting theme). The mockup CSS (`eq-aesthetic-preview.html` lines ~118-156, referenced by UI-SPEC) is the texture source for Heavy.
- **Self-host fonts (D-07):** import the 6 `@fontsource/*` packages in `app.css`; `font-display: swap`; preload the default (velious) faces.
- **Persistence:** theme swap = `localStorage.setItem('theme', key)` + set `[data-theme]` on the shell root; default (no stored pref) => `velious`. Honor `prefers-reduced-motion` for transitions.

## Per-View SQL Read Queries (BACKEND-05 — the joins)

All over `modernc.org/sqlite`, parameterized, single-writer `*sql.DB` from `store.Open`. The schema (verified, 00001_init.sql + 00003):

| Table | Key columns the reads use |
|-------|---------------------------|
| `character` | `id, name (Char), class, level, race, is_bank_toon, is_removed, last_seen` |
| `inventory_item` | `character_id, location (Slot), name (Item), item_id, count, row_ordinal, uploaded_at` |
| `spellbook_entry` | `character_id, level, name, normalized_name` |
| `item_master` | `item_id, wiki_summary, wiki_url, is_quest_item` |
| `pigparse_price` | `item_id, direction (TEXT), a30, t30` (+ a60/t60/...) |
| `wiki_spells` | `class, level, spell_name, normalized_name` |
| `wiki_gear_tier` | `tier, class, slot, item_name, rank` (item_id always NULL) |
| `quest_items` | `item_id, quest_name, source, source_url` |

**view (and bank) — the join:**
```sql
-- view: all characters; bank: add `WHERE c.is_bank_toon = 1`. Consider filtering is_removed=0.
-- pigparse direction is TEXT; "0"=WTS by the P12 job's strconv(t). Verify stored values on the box.
SELECT c.name AS char, ii.location AS slot, ii.name AS item, ii.item_id, ii.count,
       im.wiki_url, im.wiki_summary, im.is_quest_item,
       pp.direction, pp.a30, pp.t30,
       c.last_seen, ii.row_ordinal
FROM inventory_item ii
JOIN character c            ON c.id = ii.character_id
LEFT JOIN item_master im     ON im.item_id = ii.item_id
LEFT JOIN pigparse_price pp  ON pp.item_id = ii.item_id
WHERE c.is_removed = 0
ORDER BY c.name, ii.name, ii.location;
-- quest_items has 0..N rows per item_id -> fetch separately and group in Go (avoids row fan-out),
--   OR group_concat(quest_name). Separate fetch mirrors the v1 builder's Map approach.
```
- **Price in SQL vs Go:** the WTS-then-WTB pick is a small branch; do it in Go after the join (mirrors `pickPrice`). **Note `pigparse_price.item_id` is the PRIMARY KEY** (one row per item, 00001_init.sql:60), so the join yields one price row per item — direction is whatever the daily job last wrote (the D-9 WTS t=0 filter in P12 means it's typically the WTS row). This simplifies the pick: there is no both-directions fan-out at the row level. Confirm against live data, but the schema says single-row-per-item.
- **Enrichment inline (D-03):** the SELECT already pulls `wiki_summary`/`is_quest_item`; add the grouped quest links per item in Go so the JSON row is tooltip-complete.

**gear_check / spell_check:** these aren't a single SQL join — they're the Go reimplementation (above). The store provides the raw inputs (a SELECT of `wiki_gear_tier` rows; a SELECT of `wiki_spells` rows; per-char inventory/spellbook), and `compute/` produces the status rows. This matches D-02 and keeps the SQL simple. (A pure-SQL spell_check via `LEFT JOIN spellbook_entry ON normalized_name` is tempting and would work since the key is materialized; gear_check's slot-pair logic resists clean SQL, so keep both in Go for consistency and to share the parity-tested code path.)

**/api/v1/meta:** `SELECT name, last_seen FROM character WHERE is_removed=0` (character list + freshness). Theme list can be a compile-time client constant; the endpoint need only carry character/freshness data. Keep it minimal.

## Common Pitfalls

### Pitfall 1: @tanstack/svelte-table is Svelte-4-only (HIGH — corrects the UI-SPEC)
**What goes wrong:** A task installs `@tanstack/svelte-table` (named in the UI-SPEC) and the app fails to build/run under Svelte 5 — the package imports `svelte/internal`, its peer dep is `svelte: "^4.0.0 || ^3.49.0"`, and it was last published 2022.
**Why it happens:** The package name is the obvious choice and the UI-SPEC assumed it; the official Svelte-5 adapter (`@tanstack/svelte-table@9`) is still alpha.
**How to avoid:** Use `@tanstack/table-core` + a ~40-line local `createSvelteTable`/`FlexRender` adapter (the shadcn-svelte idiom; Pattern 3). Capability is identical.
**Warning signs:** Build error referencing `svelte/internal`; peer-dep warning on `svelte@5`; runtime "lifecycle_outside_component".
[VERIFIED: npm view @tanstack/svelte-table@8.21.3 peerDependencies + time]

### Pitfall 2: TanStack updater functions break reactivity if not unwrapped (MEDIUM)
**What goes wrong:** Sorting/filtering silently doesn't update because `onSortingChange` was handed the raw value instead of applying the updater function.
**Why it happens:** TanStack passes an updater that's either a value or a `(old) => new` function; with Svelte 5 runes you must detect and call it.
**How to avoid:** Every `onXChange` uses `(u) => (state = typeof u === 'function' ? u(state) : u)` (Pattern 3) and `state` is `$state(...)` consumed via `get`-getters in the `state:` block.
**Warning signs:** Clicking a header does nothing; filter input has no effect.
[CITED: shadcn-svelte.com/docs/components/data-table]

### Pitfall 3: Tailwind v4 has no config file / no PostCSS (MEDIUM)
**What goes wrong:** A task scaffolds `tailwind.config.js` + `postcss.config.js` + `@tailwind base` directives (the v3 way); v4 ignores/breaks on these.
**Why it happens:** Most tutorials and training data describe v3. v4 (current, 4.3.0) is CSS-first: `@import "tailwindcss"` in `app.css`, `@tailwindcss/vite` plugin in `vite.config.ts`, no config file, no `content` paths, `@apply` only works against global styles.
**How to avoid:** Follow the v4 SvelteKit guide: `plugins: [tailwindcss(), sveltekit()]`; `@import "tailwindcss";` in `app.css`; theme tokens via `@theme` or plain CSS custom properties.
**Warning signs:** "unknown at-rule @tailwind"; utilities not generated; `@apply` failing in a component `<style>`.
[CITED: tailwindcss.com/docs/guides/sveltekit]

### Pitfall 4: Double-decoding CP1252 (MEDIUM — already resolved upstream, do not reintroduce)
**What goes wrong:** Item names with extended chars (accented item names, special glyphs) render as mojibake because the row text gets CP1252-decoded a second time.
**Why it happens:** The watcher historically dealt with CP1252; a porter might assume the read path also needs to decode.
**How to avoid:** Encoding contract A1 (STATE.md / Plan 11-03): the CP1252-to-UTF-8 decode lives in exactly one place (`parse.CP1252Reader`) on the watcher's disk-read side. The DB stores UTF-8; the read API serializes UTF-8 JSON; the SvelteKit client renders UTF-8 directly. No decode anywhere in P14.
**Warning signs:** Garbled characters in item/char names that look correct in the DB.
[VERIFIED: STATE.md Accumulated Context A1 + ingest/handler.go:125-135]

### Pitfall 5: CORS preflight + header duplication (MEDIUM)
**What goes wrong:** The browser blocks the fetch with "No 'Access-Control-Allow-Origin' header" OR "header contains multiple values."
**Why it happens:** (a) the Go handler never sets the header; or (b) BOTH Caddy and Go set it, producing a duplicate the browser rejects.
**How to avoid:** Set CORS once, in Go (Pattern 2). Verify Caddy's reverse-proxy config does not also add CORS headers. Handle `OPTIONS` preflight with 204 before the handler body. Echo the exact origin (not `*`) so a P15 credentialed upgrade is a one-line change.
**Warning signs:** Works in curl/Postman, fails in the browser; "multiple values 'X, X'" console error.
[CITED: developer.mozilla.org CORS] [ASSUMED: Caddy passthrough behavior — verify on box]

### Pitfall 6: pigparse_price direction type drift (MEDIUM — schema reality vs v1 mental model)
**What goes wrong:** The Go `pickPrice` compares `direction == 0` (numeric, the v1 type) but the SQLite column is TEXT ("0"/"1"), so the comparison never matches and every price renders empty.
**Why it happens:** The v1 builders used a numeric `PigparseDirection`; the P12 SQLite store changed it to a stringified flag (`enrich.go` PigparsePrice.Direction string).
**How to avoid:** Compare the stringified direction in Go; confirm the exact stored values by querying `SELECT DISTINCT direction FROM pigparse_price` on the box. The P12 daily job (12-04) applies a "WTS t=0 filter" so most rows are the WTS direction.
**Warning signs:** Prices uniformly blank in the view/bank grids despite `pigparse_price` being populated (the STATE.md says `pigparse_price=4338` rows exist).
[VERIFIED: store/enrich.go PigparsePrice.Direction + 00001_init.sql:60]

### Pitfall 7: SvelteKit base path on GitHub Pages (LOW-MEDIUM, deploy-target dependent)
**What goes wrong:** On GitHub Pages project sites (served under `/<repo>/`), assets 404 because the app assumes root `/`.
**Why it happens:** GH Pages project pages have a non-root base path; Cloudflare Pages serves at root.
**How to avoid:** If deploying to GH Pages project site, set `kit.paths.base` to the repo path and use the `BASE_URL`/`base` import for links; if Cloudflare Pages (or a custom domain), root is fine. Decide the deploy target early (it affects `svelte.config.js`). The API base URL is a separate `PUBLIC_API_BASE` env (the static origin differs from `api.squirebot.quest`).
**Warning signs:** Blank page + 404s for `/_app/...` chunks on GH Pages.
[CITED: svelte.dev/docs/kit/adapter-static]

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `@tanstack/svelte-table` (Svelte 4 adapter) | `@tanstack/table-core` + local `createSvelteTable`/`FlexRender` (or `@tanstack/svelte-table@9` once stable) | Svelte 5 (2024); v9 adapter still alpha as of 2026-05 | Must vendor a tiny adapter; the obvious package name is a trap. |
| Tailwind v3 (`tailwind.config.js` + PostCSS + `@tailwind` directives) | Tailwind v4 (`@tailwindcss/vite` + `@import "tailwindcss"`, no config file) | Tailwind v4 GA (2025) | Setup is CSS-first; v3 tutorials will mislead. |
| `lucide-svelte` | `@lucide/svelte` (Svelte-5 package) | 2024-2025 | Use the `@lucide/svelte` scope; old package frozen at 1.0.1. |
| Sheet cell-note plain-text tooltips (50KB cap, newline-joined) | Rich escaped HTML popover (no cap) | P14 (this phase) | composeNotes output format changes text->HTML; escaping becomes mandatory. |
| `=HYPERLINK("url","wiki")` Sheet formula | real `<a href>` | P14 | Drop formula-string machinery; emit anchors. |
| Views rebuilt on onChange + CacheService search cache | Compute-on-read in Go; in-memory client search | P14 (CONTEXT) | No rebuild/cache/prewarm machinery; simpler. |

**Deprecated/outdated:**
- `apps-script/src/lib/themes.ts` palette values + `DEFAULT_THEME=minimalist` + `sheets-default` — superseded by the UI-SPEC Theme Catalog (richer mockup values, `velious` default, no sheets-default). Port the catalog, not the registry's exact hexes.
- The Sheet `_meta.theme` single-workbook theme — replaced by per-user `localStorage` (D-06).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Hand-rolled stdlib CORS (no `rs/cors` dep) is preferred | Standard Stack / Pattern 2 | Low — easy to swap to a lib; stdlib matches project posture. |
| A2 | Caddy does not add/strip CORS headers (passthrough) | Pattern 2 / Pitfall 5 | Medium — duplicated header breaks the browser; verify on the box (`api.squirebot.quest` Caddyfile). |
| A3 | `pigparse_price` has one row per item (item_id PK) so the price pick has no both-directions fan-out | Per-View SQL / Pitfall 6 | Medium — schema (00001:60) says PK; if a migration changed it, the join could fan out. Verify `SELECT DISTINCT direction` + row counts. |
| A4 | Deploy target affects `kit.paths.base` (GH Pages vs Cloudflare) but is otherwise orthogonal | Pitfall 7 | Low-Medium — wrong base path = blank page; decide target before scaffolding. |
| A5 | Fix path (a) for 999.30 (correct the test assertion, keep whole-string Levenshtein) is acceptable to the user | Two Carried Bugs | Low — both fix paths are pre-documented in the test; D-09 only mandates "fix the bug," not which way. Surface in discuss/plan. |
| A6 | The `web/` dir name (vs `frontend/`/`site/`) is acceptable | Project Structure | Low — cosmetic; planner can rename. |
| A7 | bank toon is identified by `character.is_bank_toon=1` (the SQLite analog of `_meta.bank_toon_name`) | Bank builder | Medium — if no character has the flag set yet (backfill is P16/CUTOVER-02), bank returns empty in P14; that's acceptable (coin also null/0), but confirm the flag is set for the guild bank toon or document the empty-until-backfill behavior. |

## Open Questions (RESOLVED during planning 2026-05-30)

> All four were closed by the Phase 14 plans (none blocking). Resolutions inline below.

1. **Which exact `direction` values does `pigparse_price` store, and can both WTS and WTB coexist per item?**
   - **RESOLVED → Plan 14-01 Task 2:** default mapping pinned `directionWTS="0"` / `directionWTB="1"` (stringified TEXT), with an on-box `SELECT DISTINCT direction FROM pigparse_price` confirmation step folded into the task's acceptance criteria. PK-per-item ⇒ 0-or-1 PriceDetail per item.
   - What we know: column is TEXT, P12 job stringifies the raw `t` flag and applies a WTS t=0 filter; item_id is PK (one row/item).
   - What's unclear: the precise stored strings ("0"/"1"/"WTS"?) and whether the PK-per-item means only one direction survives per item.
   - Recommendation: a Wave-0 spike: `SELECT DISTINCT direction, count(*) FROM pigparse_price` on the box; pin the Go `pickPrice` comparison to the observed values. Cheap and de-risks Pitfall 6.

2. **Deploy target: Cloudflare Pages or GitHub Pages (and what static origin)?**
   - **RESOLVED → LOCKED to Cloudflare Pages at `https://app.squirebot.quest`** (Plans 14-02/03/04). This fixes the CORS allow-origin (exact, not `*`), `kit.paths.base` (root, no GH-Pages base-path tax), and `PUBLIC_API_BASE` (`https://api.squirebot.quest`) consistently across the scaffold, the read handlers, and the client.

3. **999.30 fix path (a) vs (b):** **RESOLVED → path (a)** (correct the test assertion, keep the whole-string Levenshtein) — locked in Plan 14-02. Cheaper, behavior-preserving, and pre-sanctioned by the test comments.

4. **Is the guild bank toon's `is_bank_toon` flag already set in the live DB, or does it await the P16 backfill?** **RESOLVED → behavior documented in Plan 14-04:** if unset, the `bank` view is legitimately empty in P14; the client renders the empty-state copy (not an error) and the "Coin: not yet recorded" affordance. P16/CUTOVER-02 backfill + P15 ADMIN-05 coin entry fill it later.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | Backend read API + compute reimpl | yes | go1.26.2 (toolchain go1.25.7 per go.mod) | — |
| Node.js | SvelteKit build + vitest | yes | v24.14.0 | — |
| npm | Frontend deps | yes | 11.9.0 | — |
| `modernc.org/sqlite` | Read queries | yes | 1.51.0 (in go.mod) | — |
| Live SQLite DB w/ P12 data | Meaningful views (parity spot-check) | yes (on the VPS) | pigparse_price=4338, wiki_spells=1562, wiki_gear_tier=1183 populated; item_master/quest_items await re-targeted-watcher uploads (STATE.md) | Local temp DB seeded from fixtures for tests |
| SvelteKit/TanStack/Tailwind/fontsource | Frontend | not yet installed (greenfield `web/`) | versions verified via npm registry (this research) | — |
| Cloudflare/GH Pages account + static origin | Deploy (WEB-01..05 "static site") | unknown — user-owned | — | Local `npm run preview` proves the build; deploy is a separate step (can mirror P11's manual-deploy posture) |

**Missing dependencies with no fallback:** none that block development. The frontend toolchain installs via npm; per the `feedback_toolchain_installs` memory, if `npx sv`/npm hits a missing global toolchain, STOP and let the user install — do not run installers.

**Missing dependencies with fallback:**
- `item_master`/`quest_items` are still sparse in the live DB (await re-targeted-watcher uploads). For tooltip/view development and parity tests, seed a local temp DB from fixtures (the `store/testhelper.go` helper exists). Live spot-check parity (ENRICH-style) can wait until data lands.
- Static-host account: build + `vite preview` locally proves WEB-01..05 functionally; the actual public deploy can follow P11's "manual deploy from the maintainer's machine" pattern.

## Project Constraints (from CLAUDE.md)

- **Consolidated views are LOCKED** — never per-character view tabs. One reusable `DataGrid` instantiated 4x (the 200-tab limit that drove this is gone in the DB world, but the consolidated-grid decision stands and is reaffirmed by the UI-SPEC).
- **Off-Google ethos** — no Google runtime dependency in the frontend (hence D-07 self-hosted fonts, no Google Fonts `<link>`). The static app deploys on Cloudflare/GitHub Pages; the API on Hetzner behind Caddy.
- **Structured logging both sides** — Go side uses `slog` (the read handlers must log op + status + err, never row content or tokens — V7). The client has no server logging obligation, but keep any client diagnostics structured/minimal.
- **`/api/v1/...` versioning** — the read endpoints extend the existing versioned surface (replaces the retired `_meta.schema_version`/`WatcherMaxSchemaVersion` handshake).
- **Theme-derivation rule** — theme palettes derive from `docs/design/eq-aesthetic-theme.md`; the UI-SPEC Theme Catalog already encodes that derivation, so port the catalog (satisfies the rule transitively).
- **No new web framework on the backend** — stdlib `net/http` ServeMux only (D-10).
- **modernc driver name is `"sqlite"`** (not `"sqlite3"`); goose dialect is `"sqlite3"` — do not conflate.
- **GSD workflow enforcement** — file edits go through a GSD command; this is a research artifact only.

## Security Domain

`security_enforcement: true`, `security_asvs_level: 1`, `security_block_on: high` (config.json). P14 is read-only + public (D-04), which narrows the surface, but the tooltip HTML-injection path is a real XSS vector.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no (P14) | None — P14 is public by D-04; auth is P15 (AUTH-08 Discord OAuth2). |
| V3 Session Management | no (P14) | None in P14 (no login/session). P15 adds sessions. |
| V4 Access Control | partial | The read API is intentionally open (D-04, low-sensitivity EQ inventory). The only control: it is READ-ONLY (GET handlers author no writes — mirror whoami.go's read-only contract). No write/admin endpoints in P14. |
| V5 Input Validation / Output Encoding | YES (primary) | HTML-escape all interpolated strings in the ported `composeNotes.ts` before building tooltip HTML (item/char/location/quest names, wiki summary). Parameterized SQL only in the new store read methods (? placeholders) — never string-concat values. The search query is used only for in-memory substring/Levenshtein (no SQL), but still render it escaped where echoed ("No matches for <query>"). |
| V6 Cryptography | no (P14) | No secrets handled by the read API (public). The guild-code hash stays in the ingest/auth path, untouched here. |
| V7 Logging | YES | Go read handlers log op + status + err only — never row content, never any token/Authorization material (the existing handlers' V7 discipline; P14 has no token but keep the habit). |
| V13 API | YES | Versioned `/api/v1/...`; method+pattern routing rejects wrong methods (405); CORS scoped to the known static origin (not `*`); body-less GETs so no body-size concern. |

### Known Threat Patterns for SvelteKit client + Go read API

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Stored XSS via wiki summary / item names in the tooltip (`{@html}`) | Tampering / Elevation | Escape every dynamic string in `composeNotes.ts`; only structural HTML literal. Carry the v1 trust-boundary note forward. Optionally a CSP meta tag. |
| SQL injection in the new read queries | Tampering | Parameterized `?` placeholders exclusively (matches the existing store discipline). The read methods take no user input in P14 (no filters server-side), but keep the parameterized habit for any future `WHERE`. |
| Reflected XSS via search query echoed in "No matches for <query>" | Tampering | Svelte escapes `{query}` by default; only a deliberate `{@html}` would break it — don't. |
| CORS misconfig (wildcard / credentialed) | Spoofing / Info disclosure | Exact-origin allow (not `*`); no `Access-Control-Allow-Credentials` in P14 (no cookies). P15 will tighten when sessions arrive. |
| Information exposure (the data is public by design) | Info disclosure | Accepted per D-04 (low-sensitivity EQ inventory, guild universal-visibility ethos). D-05 noindex keeps it off search engines. Re-gated in P15. |
| Wiki link tab-nabbing | Tampering | `rel="noopener"` (and `target="_blank"`) on the wiki `<a>` (UI-SPEC already specifies `rel="noopener"`). |

> **Security block gate:** the one HIGH-severity item is tooltip XSS (V5). The mitigation (escape-before-inject in the ported composeNotes) is mandatory and testable — a port test should assert that a malicious item name like `<img src=x onerror=...>` is rendered escaped, not executed.

## Code Examples

### Read handler registration + CORS wrap (Go, mirrors existing main.go)
```go
// Source: cmd/squirebot-server/main.go:258-265 (existing ServeMux wiring), extended
mux := http.NewServeMux()
mux.Handle("POST /api/v1/ingest", ingest.New(auth.New(db), db))        // existing
mux.Handle("GET /api/v1/whoami",  ingest.NewWhoami(auth.New(db), db))  // existing
st := store.NewStore(db)
mux.Handle("GET /api/v1/meta",              readapi.NewMeta(st))
mux.Handle("GET /api/v1/views/view",        readapi.NewView(st))
mux.Handle("GET /api/v1/views/gear_check",  readapi.NewGearCheck(st))
mux.Handle("GET /api/v1/views/spell_check", readapi.NewSpellCheck(st))
mux.Handle("GET /api/v1/views/bank",        readapi.NewBank(st))
srv := &http.Server{Addr: *addr, Handler: cors(staticOrigin, mux)}     // CORS wraps everything
```

### SvelteKit SPA config (svelte.config.js + +layout.ts)
```js
// Source: svelte.dev/docs/kit/single-page-apps + adapter-static docs
// svelte.config.js
import adapter from '@sveltejs/adapter-static';
export default { kit: { adapter: adapter({ fallback: '200.html' }) /*, paths: { base } if GH Pages */ } };
```
```ts
// src/routes/+layout.ts — true SPA (no SSR, no prerender of data routes)
export const ssr = false;
export const prerender = false;
```

### Tailwind v4 wiring (vite.config.ts + app.css)
```ts
// Source: tailwindcss.com/docs/guides/sveltekit
// vite.config.ts
import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';
export default defineConfig({ plugins: [tailwindcss(), sveltekit()] });
```
```css
/* src/app.css */
@import "tailwindcss";
@import "@fontsource/cinzel-decorative/400.css";   /* + the other 5 faces, woff2 */
[data-theme="velious"]   { --bg:#060b18; --panel:#0f1729; --text:#d4dee8; --accent:#a8c5e0;
                           --status-ok:#6fc8b0; --status-missing:#e88a8a; --status-other:#a8c5e0;
                           --font-display:"Cinzel Decorative"; --font-body:"IM Fell English"; --weight-display:600; }
/* ... vanilla / kunark / minimalist (--weight-display:400) / heavy (inverting rows) ... */
```

### didYouMean with the 999.28 fix (ported client TS)
```ts
// Source: apps-script/src/lib/searchIndex.ts:89-97, with the empty-query guard added (999.28)
export function didYouMean(query: string, itemNames: string[]): string[] {
  if (!query || !query.trim()) return [];          // 999.28 FIX — empty query suggests nothing
  const q = query.toLowerCase();
  return itemNames
    .map((n) => ({ n, d: levenshtein(q, n.toLowerCase()) }))
    .filter((x) => x.d <= 2 && x.d > 0)
    .sort((a, b) => a.d - b.d)
    .slice(0, 3)
    .map((x) => x.n);
}
```

## Sources

### Primary (HIGH confidence)
- Repo source (read directly): `apps-script/src/tabs/{buildView,buildGearCheck,buildSpellCheck,buildBank,composeNotes}.ts`; `apps-script/src/lib/{searchIndex,themes,eq-constants}.ts` + `__tests__/searchIndex.test.ts`; `internal/backendsrv/ingest/{handler,whoami,envelope}.go`; `internal/backendsrv/store/{db,binding,replace,enrich}.go`; `internal/backendsrv/migrations/{00001_init,00003_enrich_columns}.sql`; `cmd/squirebot-server/main.go`; `docs/design/eq-aesthetic-theme.md`; `.planning/{REQUIREMENTS,ROADMAP,STATE}.md`; `.planning/phases/14-web-frontend/{14-CONTEXT,14-UI-SPEC}.md`; `.planning/config.json`.
- npm registry (verified versions + peer deps): `@sveltejs/kit` 2.61.1, `svelte` 5.56.0, `@sveltejs/adapter-static` 3.0.10, `@tanstack/table-core` 8.21.3, `@tanstack/svelte-table` 8.21.3 (peerDeps `svelte ^4||^3.49`, published 2022; v9 alpha line), `tailwindcss` 4.3.0, `vite` 8.0.14, `vitest` 4.1.7, `@fontsource/*` 5.2.x, `@lucide/svelte` 1.17.0.
- Toolchain: `go version` go1.26.2 (go.mod toolchain go1.25.7); `node` v24.14.0; `npm` 11.9.0.

### Secondary (MEDIUM confidence)
- tailwindcss.com/docs/guides/sveltekit — Tailwind v4 + SvelteKit Vite-plugin setup (cross-checked with dev.to v4 guide).
- svelte.dev/docs/kit/single-page-apps + /adapter-static — SPA fallback config.
- shadcn-svelte.com/docs/components/data-table — the `createSvelteTable`/`FlexRender` over `@tanstack/table-core` Svelte-5 pattern.
- jamesoclaire.com (2025) + github.com/walker-tx/svelte5-tanstack-table-reference — corroborate that `@tanstack/svelte-table` v8 doesn't work with Svelte 5 (custom/local adapter needed).

### Tertiary (LOW confidence — flagged for validation)
- Caddy CORS-passthrough behavior (Assumption A2) — not verified against the live Caddyfile; verify on the box.
- Exact `pigparse_price.direction` stored values (Open Q1) — inferred from P12 job description; verify with a live `SELECT DISTINCT`.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every version verified against the npm registry this session; the `@tanstack/svelte-table` Svelte-5 incompatibility verified via published peerDependencies.
- Architecture / backend patterns: HIGH — read directly from the live, shipped backend source (handler.go, whoami.go, main.go, store/*, migrations/*).
- View-builder port semantics (WEB-02): HIGH — algorithms transcribed from the verified v1 builders + their tests; parity oracle identified.
- Two carried bugs: HIGH — exact current behavior + intended fix read from source + the test's own documented fix options.
- Theme port: HIGH — UI-SPEC catalog + eq-aesthetic-theme.md + v1 themes.ts all read; the superseded v1 values flagged.
- Pitfalls 5/6 + deploy target: MEDIUM — depend on live-box config (Caddy, pigparse data, hosting choice) not inspected this session; called out as Open Questions / Assumptions.

**Research date:** 2026-05-30
**Valid until:** ~2026-06-29 for the npm stack (fast-moving — re-verify `@tanstack/svelte-table@9` GA status + Svelte/Kit/Tailwind versions at plan time); the repo-internal facts (schema, builders, bugs) are stable until the code changes.
