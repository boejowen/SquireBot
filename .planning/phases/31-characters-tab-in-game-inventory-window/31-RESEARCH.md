# Phase 31: Characters Tab + In-Game Inventory Window - Research

**Researched:** 2026-06-18
**Domain:** SvelteKit web rendering of a Go compute-on-read model + one new session-gated read-API + extend-only wiki icon-id enrichment (P1999 MediaWiki API)
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01 (icon source):** Item→icon-id obtained by **extending the existing weekly P1999 wiki enrichment job** to capture each item's wiki icon id, **cached server-side**. No manual curation, no fetch-on-demand.
- **D-02 (icon render + fallback):** Icons render from `https://wiki.project1999.com/images/Item_<iconId>.png`; **colored-tile fallback on image load error is LOCKED** (sketch 002). Coverage ships **incrementally** — the window is never blocked on 100% icon coverage.
- **D-03 (storage = extend-only):** Icon id stored alongside the existing wiki enrichment via an **extend-only** schema change (new column/table; never a breaking change). The exact wiki field/parse was the research item → **RESOLVED below** (`lucy_img_ID` inside `{{Itempage}}`).
- **D-04 (bag drill-down):** Opening a general-inventory bag **expands its contents INLINE** (beneath/within the grid). NOT a modal. Fed by `InventorySlot.Children`.
- **D-05 (bank fidelity):** Character's own bank renders as a **faithful EQ bank-window grid** directly below the paperdoll, **reusing the same grid + inline-drill-down component** as general inventory. No Inventory/Bank toggle.
- **D-06 (examine interaction):** **Hover preview + click-to-pin.** Desktop hover = transient preview; click = pins full examine into a single side panel. Touch: tap = pin (no hover).
- **D-07 (single pinned panel):** Single, **replace-on-click** pinned panel. Not a multi-pin compare board.
- **D-08 (examine content + order — LOCKED):** flags → slot/skill → DMG/DLY → AC → stats → wt/size → class/race → **PigParse price** → **wiki link** → **last-synced**. Stats from stored wiki data; price/last-listed from the Phase 29 name-keyed join.
- **D-09 (graceful missing data):** Missing wiki stats or PigParse price → fields **simply omitted** — never blank/broken/"null". An examine always renders at least name + whatever is known.
- **D-10 (character list ordering & data):** Default order = **viewer's own characters first (A-Z)**, then other guild characters, then guild banks/bots; search **prioritizes the viewer's characters**. Viewer identity = the Discord session; "the viewer's characters" = their `character_assignment` rows (v2.3). Rows show name, level, race, class.
- **D-11 (empty/sparse states):** Empty equipment slots render as empty paperdoll positions (Phase 29 keeps `item_id 0`). A character with no synced inventory → friendly "no inventory synced yet". Missing level/race/class → show what's available (— / blank), never an error.

### Claude's Discretion
- The exact wiki field/parse that yields the icon id (**now resolved**), and whether icon-id storage is a new column on `item_master` vs. a small mapping table (extend-only either way).
- The new read-API surface: pick the endpoint shape for `StructuredInventory` (e.g. `GET /api/v1/inventory/{char}`) + the character-list/roster endpoint, following the existing `readapi` pattern.
- Whether the hover preview reuses `ItemTooltip.svelte` while the pinned panel is a fuller examine, or both share one component.
- Paperdoll grid arrangement, slot labels, mobile reflow, icon sizing → resolved by the APPROVED 31-UI-SPEC.
- Whether the character list reuses `DataGrid.svelte` or a purpose-built list (UI-SPEC recommends bespoke).

### Deferred Ideas (OUT OF SCOPE)
- **Item-centric Inventory tab** (which characters hold item X) — Phase 32 (ITEM-01..03).
- **Banks tab** (banks-only list + guild-wide value + total platinum) — Phase 33 (BANK-01..03); reuses THIS window per bank toon.
- **Per-character/per-slot Wishlist rework** — Phase 34 (WISH-01..07); depends on this phase's equipped-slot rendering.
- **Modal/draggable bag pop-out** — rejected for inline expand (D-04).
- **Multi-pin examine compare board** — rejected for single replace-on-click panel (D-07).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CHAR-01 | List all guild characters (name, level, race, class); order viewer-first A-Z, then guild, then banks/bots | NEW roster read-API endpoint composing `CharsWithMeta` (name/level/race/class/is_bank_toon) + `ListMyAssignments`(viewer) + `is_guild_bot` (gap — see below) + viewer-first sort. UI-SPEC §B bespoke list. |
| CHAR-02 | Per-character search prioritizing the viewer's characters | Client-side filter over the roster (tiny corpus); UI-SPEC §C scoped-search bar at top of tab body, D-10 viewer-first ranking preserved. |
| CHAR-03 | Selecting a character opens its inventory window | NEW `GET /api/v1/inventory/{char}` returning `compute.StructuredInventory`; master-detail on ONE route (consolidated-views lock RELAXED). |
| INV-01 | In-game window: paperdoll equipment + general slots + bank below; stacked slots show count | `CharacterInventory{Equipment[],General[],Bank[]}` renders directly; 23 canonical slots per `slotconst.go`; `Count`/`Slots` on each `InventorySlot`. UI-SPEC §D/§E/§F. |
| INV-02 | Hover/tap examine: name + stats (stored wiki) + PigParse price + wiki link + last-synced; click-to-pin | `InventorySlot` carries `WikiSummary`/`WikiURL`/`Price`/`Prices`/`LastListed`; `ItemTooltip.svelte` + `composeItemNote` reusable for the safe `{@html}` body. **LastSeen gap** for "last synced" — see Open Questions. UI-SPEC §G, D-08 order. |
| INV-03 | General containers (bags) open to reveal contents that behave like the inventory grid | `InventorySlot.Children[]` (one level), `Slots > 0` ⇒ openable; INLINE expand (D-04). UI-SPEC §F. |
| INV-04 | Item icons render from P1999 wiki item-icon images | NEW `icon_id` enrichment (`lucy_img_ID` from `{{Itempage}}`); `Item_<iconId>.png`; colored-tile `onerror` fallback (D-02). |
</phase_requirements>

## Summary

This is a **brownfield phase**: the data model (`compute.StructuredInventory` + the 23-slot taxonomy + container nesting + name-keyed price), the 5-tab shell, and the Characters route slot all SHIPPED in Phases 29-30. Phase 31 fills three concrete gaps: **(1)** wire two new session-gated read-API endpoints (one character's inventory window; the character roster), **(2)** add an extend-only `icon_id` enrichment to the weekly wiki job, and **(3)** build the SvelteKit Characters tab + in-game window over the existing component idioms.

All three gaps map cleanly onto established patterns with one verified new fact: **the P1999 wiki exposes an item's icon id as the `lucy_img_ID` parameter inside the `{{Itempage}}` template** — verified live for Cloak of Flames (658), Wurmslayer (736), and Ring of the Ancients (563), all matching sketch-002's known mappings, and all three `Item_<iconId>.png` URLs serve as PNGs (HTTP 200, `image/png`). The existing `enrich.ParseItempage` already splits `{{Itempage}}` params, so capturing `lucy_img_ID` is a one-line extraction on the existing parse path. Storage is a single extend-only `ALTER TABLE item_master ADD COLUMN icon_id INTEGER` (migration `00012`) — `item_master` is item_id-keyed in the EQ namespace, the same key the inventory window joins on, so the icon flows through `StructuredInventory` cleanly.

The read-API gap is the largest backend item but is pure pattern-replication: `readapi/views.go`/`meta.go` show the exact handler shape (`NewX(st)` constructor, `ServeHTTP` with a defensive 405, nil→`[]` coercion, V7 logging of op + count only). Every read route is wrapped in `webauth.RequireSession` at registration in `cmd/squirebot-server/main.go` — the read API has been login-gated since P15, and the viewer's identity is read via `webauth.UserFromContext(ctx)`. The web side reuses `api.ts` (credentialed fetch + typed errors), `ItemTooltip.svelte`/`composeItemNote` (the ONE sanctioned `{@html}` sink), `StateBlock.svelte` (loading/empty/error copy), and the EQ theme tokens.

**Primary recommendation:** Add `GET /api/v1/inventory/{char}` (RequireSession-wrapped, returns `compute.StructuredInventory`) + `GET /api/v1/characters` (the viewer-aware roster); extend the weekly wiki job to capture `lucy_img_ID` into a new extend-only `item_master.icon_id` column (migration `00012`); build the window as a generic, prop-driven Svelte component over `CharacterInventory` (reused by Phase 33, feeds Phase 34); reuse `ItemTooltip`/`composeItemNote` for the examine body; render icons with an `onerror` colored-tile fallback so coverage ships incrementally.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Inventory slot model (parse, nest, price-join) | API / Backend (compute) | Database (store) | SHIPPED Phase 29 — compute-on-read; nothing new here |
| Icon-id enrichment (wiki fetch + parse + store) | API / Backend (enrich job) | Database (item_master) | The wiki is an external API; the icon id is cached server-side per D-01 (matches the existing weekly job) |
| Inventory window HTTP exposure | API / Backend (readapi) | — | `StructuredInventory` has no route yet; this is THE new backend surface, session-gated |
| Character roster (meta + assignment + flags) | API / Backend (readapi + store) | Database | Viewer-aware ordering needs the session identity (server-side) + assignment join |
| Session gating / viewer identity | Frontend Server (webauth middleware) | — | `RequireSession` resolves the Discord session cookie; `UserFromContext` exposes it to handlers |
| Window/paperdoll/grid/examine rendering | Browser / Client (SvelteKit) | — | Pure client render of the `CharacterInventory` payload; "a view is a client render, not a Sheet tab" |
| Item-icon image load | Browser / Client + CDN | — | `<img>` from `wiki.project1999.com/images/`; `onerror` → colored-tile fallback (client-only) |
| Character search / viewer-first filter | Browser / Client | — | Roster is tiny (~12 members × ~10 chars); client-side filter preserves D-10 ranking |

**Tier-correctness note for the planner/checker:** the icon URL is a *content image* fetched by the browser directly from the wiki CDN, NOT proxied through the backend. Only the icon *id* is computed/stored server-side. Do not plan a backend image proxy.

## Standard Stack

This phase introduces **no new libraries**. It is composition over the existing, shipped stack.

### Core (existing — already in the repo)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go | 1.25.7 | backend (readapi, enrich, store, compute) | [VERIFIED: go.mod] — the whole backend |
| `net/http` (stdlib ServeMux) | stdlib | route registration (`mux.Handle("GET /api/v1/...")`) | [VERIFIED: cmd/squirebot-server/main.go] — hand-rolled mux, Go 1.22+ method+path patterns |
| `goose` | (existing) | extend-only migrations (`00001`–`00011` live) | [VERIFIED: internal/backendsrv/migrations/] — next is `00012` |
| SvelteKit + Svelte 5 (runes) | (existing) | web (`web/src/`) | [VERIFIED: web/vite.config.ts, +page.svelte uses `$props`/`$state`/`$derived`] |
| Tailwind v4 (CSS-first) + hand-rolled EQ-theme tokens | (existing) | styling via `--accent`/`--panel`/`--font-display` | [VERIFIED: web/vite.config.ts `@tailwindcss/vite`; SiteShell.svelte] |
| `@lucide/svelte` | (existing) | icons (Search, Chevron…) | [VERIFIED: StateBlock.svelte, SiteShell.svelte] — UI-SPEC adds at most a `Package`/`Backpack` glyph, no new dep |
| vitest | (existing) | node-only tests (`web/`) + Go `go test` (backend) | [VERIFIED: web/vite.config.ts — single `server`/`node` project] |

### Supporting (existing patterns to reuse, not install)
| Asset | Path | Purpose | When to Use |
|-------|------|---------|-------------|
| `compute.StructuredInventory` | `internal/backendsrv/compute/inventory.go` | the window's data model | the new inventory endpoint calls this directly |
| `readapi` handler idiom | `internal/backendsrv/readapi/{views,meta,itemsearch}.go` | the exact HTTP shape to copy | both new endpoints |
| `webauth.RequireSession` / `UserFromContext` | `internal/backendsrv/webauth/session.go` | session gate + viewer identity | wrap both new routes; read viewer for D-10 |
| `enrich.ParseItempage` | `internal/backendsrv/enrich/wikiitem.go` | already parses `{{Itempage}}` params | add the `lucy_img_ID` extraction here |
| `store.UpsertItemMasterTx` / `ItemMaster` | `internal/backendsrv/store/enrich.go` | item_master writer | extend with `IconID` |
| `api.ts` (`getJSON`, typed rows, `ApiError`) | `web/src/lib/api.ts` | credentialed fetch + typed errors | add `fetchInventory`, `fetchCharacters` + row interfaces |
| `ItemTooltip.svelte` + `composeItemNote` | `web/src/lib/components/ItemTooltip.svelte`, `web/src/lib/tooltip/composeNotes.ts` | the ONE safe `{@html}` examine body | hover preview + pinned panel (share the composer) |
| `StateBlock.svelte` | `web/src/lib/components/StateBlock.svelte` | loading/empty/error/no-results copy | every fetch state (UI-SPEC §J) |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `item_master.icon_id` column | a separate `item_icon(item_id, icon_id)` mapping table | A new table is also extend-only, but `icon_id` is a 1:1 attribute of an item the wiki job already writes (`item_master` upsert) — a column rides the existing write path with zero new join. **Recommend the column.** |
| Reusing `ItemTooltip` for the hover preview | a bespoke preview popover | `ItemTooltip` already implements hover-on-pointer + tap-on-touch + Esc/outside-dismiss + the safe `{@html}` sink. Reuse its mechanics for the transient preview; render the SAME composed body in a sticky card for the pinned panel (UI-SPEC §G "share one `examineBody`"). |
| Bespoke roster list | `DataGrid.svelte` | UI-SPEC §B recommends bespoke (the master-detail "selected row" affordance + D-10 grouping bands are cleaner than a flat sortable table). DataGrid allowed as fallback if it preserves the D-10 order + row-selection. |

**Installation:** None. `npm`/`go` deps unchanged. (Watcher untouched — backend + web only.)

**Version verification:** No new packages. Go 1.25.7 [VERIFIED: go.mod]. Migrations at `00011` [VERIFIED: ls internal/backendsrv/migrations/]; the new one is `00012`.

## Architecture Patterns

### System Architecture Diagram

```
                          ┌─────────────────────────────────────────────┐
   weekly wiki job        │  enrich.ParseItempage({{Itempage}} wikitext) │
   (RunWiki, jobs/wiki.go)│  → already extracts itemname/notes/slot/...  │
        │                 │  ADD: lucy_img_ID → ParsedWikiItem.IconID    │
        ▼                 └───────────────────┬─────────────────────────┘
  P1999 MediaWiki API                         │  UpsertItemMasterTx(IconID)
  action=parse&prop=wikitext                  ▼
                              ┌──────────────────────────────────┐
                              │ item_master (item_id PK, ...,     │
                              │   icon_id INTEGER  ← NEW col 00012)│
                              └───────────────┬──────────────────┘
                                              │ im.item_id = ii.item_id (EQ namespace)
   watcher upload (unchanged) ──► inventory_item ──┐
                                              ▼      │ store.InventoryForChar (name-keyed price join)
                              ┌──────────────────────────────────┐
                              │ compute.StructuredInventory(char) │
                              │  → CharacterInventory{Equipment[],│
                              │     General[], Bank[]}            │
                              │     (each InventorySlot carries   │
                              │      icon_id, price, wiki, kids)  │
                              └───────────────┬──────────────────┘
                                              │
  Discord session cookie ──► webauth.RequireSession ──► UserFromContext(viewer)
                                              │
   ┌──────────────────────────────────────────┼──────────────────────────────────┐
   ▼ NEW                                       ▼ NEW                               │
 GET /api/v1/characters                  GET /api/v1/inventory/{char}              │
 (roster: meta + assignment(viewer)      (one char's StructuredInventory,          │
  + bank/bot flags, viewer-first)         session-gated)                           │
   │                                       │                                        │
   └───────────────── CORS (exact origin + credentials) ──────────────────────────┘
                                              │  (api.squirebot.quest → squirebot.quest)
                                              ▼
            ┌──────────────────────────────────────────────────────────┐
            │  SvelteKit /characters (browser)                          │
            │   • scoped search (viewer-first filter, client-side)      │
            │   • bespoke roster list (3 bands: yours/guild/banks-bots) │
            │   • InventoryWindow (generic over CharacterInventory)     │
            │       paperdoll(23 slots) + general grid + bank grid      │
            │       inline bag expand (Children[])                      │
            │       <img Item_<iconId>.png> onerror→colored-tile        │
            │       examine: ItemTooltip hover preview + pinned panel   │
            │         (shared composeItemNote {@html}, D-08 order)      │
            └──────────────────────────────────────────────────────────┘
```

### Recommended Project Structure
```
internal/backendsrv/
├── migrations/00012_item_icon.sql      # NEW: ALTER TABLE item_master ADD COLUMN icon_id INTEGER
├── enrich/wikiitem.go                  # EDIT: extract lucy_img_ID → ParsedWikiItem.IconID
├── enrich/jobs/wiki.go                 # EDIT: pass item.IconID into the UpsertItemMaster call
├── store/enrich.go                     # EDIT: ItemMaster.IconID + UpsertItemMasterTx writes icon_id
├── store/readviews.go                  # EDIT: InventoryRow gains IconID (SELECT im.icon_id); NEW roster read
├── compute/types.go                    # EDIT: InventorySlot gains IconID (append-only tag); maybe CharacterInventory.LastSeen
├── compute/inventory.go                # EDIT: slotFromRow copies IconID; StructuredInventory unchanged shape
└── readapi/
    ├── inventory.go                    # NEW: GET /api/v1/inventory/{char} handler
    └── characters.go                   # NEW: GET /api/v1/characters roster handler
cmd/squirebot-server/main.go           # EDIT: register both routes under RequireSession

web/src/
├── lib/api.ts                          # EDIT: fetchInventory(char), fetchCharacters() + row interfaces
├── lib/components/
│   ├── InventoryWindow.svelte          # NEW: generic over CharacterInventory (reused by P33)
│   ├── PaperdollSlot.svelte            # NEW: one .slot tile (icon + onerror fallback + count + bag marker)
│   ├── ExaminePanel.svelte             # NEW: the pinned panel (shares composeItemNote body)
│   └── (reuse ItemTooltip.svelte for the hover preview)
└── routes/characters/+page.svelte      # REPLACE the placeholder: list + search + window
```

### Pattern 1: The session-gated read handler (copy from views.go)
**What:** A struct holding `*store.Store`, a `NewX(st)` constructor, and a `ServeHTTP` with a defensive 405, nil→`[]` coercion, and V7 logging (op + count, never row content).
**When to use:** Both new endpoints.
**Example (the existing idiom — `inventory.go` mirrors it):**
```go
// Source: internal/backendsrv/readapi/views.go (verified)
type ViewsHandler struct { store *store.Store; view string }
func NewViews(s *store.Store, view string) *ViewsHandler { return &ViewsHandler{store: s, view: view} }
func (h *ViewsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
    ctx := r.Context()
    result, err := compute.View(ctx, h.store) // or compute.StructuredInventory(ctx, h.store, char)
    if err != nil { slog.Error("... read failed", "err", err); http.Error(w, "internal error", 500); return }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    _ = json.NewEncoder(w).Encode(result) // nil→[] coercion for slices
}
```

### Pattern 2: The `{char}` path parameter (Go 1.22+ ServeMux wildcard)
**What:** Register `GET /api/v1/inventory/{char}` and read `r.PathValue("char")`. The stdlib ServeMux supports `{name}` wildcards (the repo is on Go 1.25.7).
**When to use:** The inventory endpoint (one char per request).
**Security:** The `char` value is used ONLY as a parameterized `?` bind in `InventoryForChar` (verified — `store/readviews.go` uses `?` placeholders, never string-concat). Never interpolate it into SQL or a log line containing row content (V7). An unknown char returns an empty `CharacterInventory` (not a 404) so D-11's "no inventory synced yet" renders client-side.

### Pattern 3: Viewer identity for D-10 ordering
**What:** Inside the RequireSession-gated handler, `uid, ok := webauth.UserFromContext(ctx)` returns the authenticated `discord_user_id`. Compare it against `character_assignment.discord_user_id` (the v2.3 "my characters" key) to flag "yours" and sort viewer-first.
**When to use:** The roster endpoint.
**Example:**
```go
// Source: internal/backendsrv/webauth/session.go (verified)
uid, ok := webauth.UserFromContext(ctx) // the viewer's discord_user_id
// "the viewer's characters" = ListMyAssignments(ctx, db, uid) — character_assignment rows (v2.3)
```

### Pattern 4: Extend-only icon_id capture on the existing wiki parse
**What:** `enrich.ParseItempage` already calls `parseTemplateParams(blockBody)` → a `map[string]string` of `{{Itempage}}` params. Add one line: `iconID := parseIconID(getParam(params, "lucy_img_ID", ""))` (strip non-digits, atoi, 0 when absent/unparseable). Surface it on `ParsedWikiItem.IconID int`. The job (`jobs/wiki.go` `upsertItemAndQuests`) copies it into the `store.ItemMaster` it already constructs.
**Why this is cheap:** The parse already happens; the upsert already happens; this rides both with zero new fetch and zero new tx.

### Anti-Patterns to Avoid
- **Backend image proxy.** The icon `<img>` loads directly from the wiki CDN client-side. Do NOT plan a `/api/v1/icon/...` proxy — D-02's `onerror` fallback handles missing icons, and proxying would add latency + a backend dependency for a static image.
- **Materializing N per-character routes/tabs.** The consolidated-views lock is RELAXED for ONE reusable window rendered on selection (master-detail). Do NOT create `/characters/<name>` as N physical routes that explode at guild scale — selection MAY be URL-reflected (`?c=<name>`), but it renders the single reusable component (UI-SPEC §A).
- **Raw-interpolating item/character names into HTML.** Names are guildie-controlled (a guildie names a character). The ONLY sanctioned `{@html}` sink is `composeItemNote`/`escapeHtml` (the live T-14.04-01 gate). Plain `{}` interpolation auto-escapes; never `{@html}` anything else.
- **Joining icon by normalized name.** `icon_id` lives on `item_master` (EQ-namespace item_id PK) and the inventory window joins `im.item_id = ii.item_id` — an ID join in ONE namespace, which is correct. (The name-key rule is ONLY for the PigParse *price* cross-namespace join — already handled by Phase 29.)
- **Trusting node vitest green for the window.** Web tests are node-only/DOM-blind — see Common Pitfalls.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Examine HTML body | A new tooltip composer | `composeItemNote` (`composeNotes.ts`) | It's the ONE XSS-audited `{@html}` sink (escapeHtml + safeHttpUrl scheme allow-list); a new one re-opens the HIGH-severity T-14.02-01 gate |
| Hover/tap popover + Esc/outside-dismiss | A new popover | `ItemTooltip.svelte` mechanics | Already implements pointer-hover + touch-tap + focus + Esc + outside-pointerdown dismiss + aria |
| Credentialed fetch + auth-error routing | A raw `fetch` | `getJSON` from `api.ts` | Carries `credentials:'include'` (the cross-subdomain cookie), typed `Unauthenticated`/`Forbidden`, malformed-JSON guard — AuthGate re-routes 401/403 |
| Loading/empty/error/no-results copy | New strings | `StateBlock.svelte` | UI-SPEC §J reuses its `kind="loading"/"empty"/"error"/"no-results"` verbatim |
| Wiki page-name → URL slug | `encodeURIComponent`/`url.PathEscape` ad hoc | `enrich.EncodeURIComponent` (Go) / `wikiUrlFor` (TS) | Both already match JS `encodeURIComponent` byte-for-byte (apostrophes/parens preserved) — the wiki URL contract |
| Migration writing | An inline ALTER outside goose | a `00012_*.sql` goose file | The schema-evolution rule: extend-only, version-stamped; SQLite allows one ADD COLUMN per ALTER |
| Slot taxonomy / container nesting / price join | Re-parse `Location`/`Slots` | `compute.StructuredInventory` | SHIPPED Phase 29 (compute-on-read); the window renders it directly |

**Key insight:** Nearly every "hard" part of this phase is already solved and shipped. The genuine new code is small and mechanical: two read handlers (copy `views.go`), one parser line + one column (the icon), and the Svelte rendering of an existing typed payload. The risk is in the rendering correctness (paperdoll geometry, inline bag expand vs. the sketch's modal, the colored-tile fallback) — which is exactly what the browser-smoke catches.

## Common Pitfalls

### Pitfall 1: Web vitest is node-only / DOM-blind — green ≠ works
**What goes wrong:** `npm test` passes (node `server` project only — verified in `web/vite.config.ts`: a single `environment: 'node'` project, no `@testing-library/svelte`, no jsdom) yet the window crashes or mis-renders in a real browser. P15 shipped 165 green tests with 2 crashing browser blockers.
**Why it happens:** No component/DOM test layer (toolchain-install rule — the project doesn't add `@testing-library/svelte`). The paperdoll render, hover-vs-pin interaction, inline bag expand, remote icon load + `onerror` fallback, and master-detail selection are all DOM behaviors invisible to node tests.
**How to avoid:** The plan MUST include a **deploy-then-browser-smoke** task (localhost can't auth against prod — cookie `Domain=squirebot.quest` + apex-only CORS; `npm run dev` login bounces to prod). Smoke checklist (from UI-SPEC §Build Notes): list orders viewer→guild→banks/bots; search prioritizes viewer; select renders the window; hover preview (desktop) + click pins + clicking another REPLACES the pin; a ⊞ bag expands INLINE (no modal) and collapses; stack counts show; a known iconId loads the PNG and a bogus/empty iconId falls back to the colored tile; examine omits missing fields (no "null"); an unsynced char shows "No inventory synced yet"; renders under ALL 5 themes (Heavy + Minimalist contrast spot-check).
**Warning signs:** Calling the phase "verified" off green node tests; any "it works locally" claim made against `npm run dev`.

### Pitfall 2: The "last synced" examine field (D-08 #12) is not on `InventorySlot`
**What goes wrong:** The examine's last line is "Last synced: {timestamp}", but `compute.InventorySlot` (types.go) has `LastListed` (the *price* last-seen) and NO per-item `LastSeen` (the *character upload* freshness). A planner could wire `LastListed` by mistake — that's the last-listed-for-sale date, a DIFFERENT value.
**Why it happens:** `store.InventoryRow` DOES carry `LastSeen` (character.last_seen) [VERIFIED: readviews.go:339 `r.LastSeen = charLastSeen.String`], but `slotFromRow` doesn't copy it onto the slot (the slot model predates this phase's examine need).
**How to avoid:** "Last synced" is a per-CHARACTER value (same for every slot in the window). Surface it ONCE — add `LastSeen string` to `CharacterInventory` (append-only, sourced from any row) OR return it on the inventory endpoint envelope — and render it in the examine footer for every item. Do NOT confuse it with `LastListed`. (See Open Questions Q1.)
**Warning signs:** The examine showing a price-listing date as "last synced"; a per-item last_seen that varies within one window.

### Pitfall 3: Icon-id namespace — join by ID in the item_master namespace, NOT by name
**What goes wrong:** Trying to key the icon off the PigParse catalog or by normalized name (the price-join rule) — wrong namespace.
**Why it happens:** The repo carries a strong "join by normalized NAME, never raw item_id" memory — but that rule is ONLY for the cross-namespace PigParse *price* join (catalog ids ≠ EQ inventory ids). The wiki `item_master` is the watcher's OWN EQ-namespace enrichment, correctly id-matched (`im.item_id = ii.item_id` — verified in `InventoryJoin`/`InventoryForChar`).
**How to avoid:** Store `icon_id` on `item_master` (item_id PK). It flows to the inventory window via the existing id join. The icon and the price reach the same item by DIFFERENT joins — icon by id (item_master), price by name (pp_rep CTE). Both are already correct; don't unify them.
**Warning signs:** An icon-id mapping table keyed by name; missing icons for items that have a wiki page (would indicate a name-join attempt).

### Pitfall 4: The roster needs `is_guild_bot` + `last_seen` that `CharsWithMeta` doesn't return
**What goes wrong:** Reusing `CharsWithMeta` for the roster gives name/class/level/race/is_bank_toon but NOT `is_guild_bot` (the D-10 "banks/bots" band needs both) and NOT `last_seen`. `CharFreshness` gives name+last_seen but no meta. No single existing read returns the full roster shape.
**Why it happens:** Those reads were built for different surfaces (the char-meta form, the /meta freshness feed).
**How to avoid:** Write a NEW roster store read selecting `id, name, class, level, race, is_bank_toon, is_guild_bot, last_seen FROM character WHERE is_removed = 0` (extend-only, `?`-only, single tested SQL path per the store seam), then LEFT-compose the viewer's `character_assignment` rows in the handler (or join `character_assignment` filtered to the viewer for the "yours" flag). Bands: yours = assigned to viewer; banks/bots = is_bank_toon OR is_guild_bot; guild = everyone else.
**Warning signs:** A roster missing the banks/bots band, or with no per-char freshness; reusing `CharsWithMeta` verbatim.

### Pitfall 5: Inline bag expand is the biggest delta from the sketch HTML (which used a modal)
**What goes wrong:** Copying the sketch-002 `index.html` literally reintroduces the `#bagscrim` modal — explicitly REJECTED in D-04.
**Why it happens:** The sketch's visual reference used a modal pop-out; the discussion overrode it to inline expand.
**How to avoid:** Render the bag's `Children[]` as an INLINE sub-grid beneath the grid row (UI-SPEC §F): a nested indented `--panel` region, a Label sub-header `{Bag name} — {used} of {capacity} slots`, `aria-expanded` on the bag tile, re-click collapses. No modal, no scrim. Same `.slot` renderer for children.
**Warning signs:** A `#bagscrim`/modal/scrim element; a bag opening over the grid instead of in-flow.

### Pitfall 6: 23 slots, not 21 — `slotconst.go` is authoritative
**What goes wrong:** 31-CONTEXT prose says "21 canonical slots"; rendering 21 drops Ear1/Ear2 (or Power/Ammo).
**Why it happens:** An earlier list omitted the two ear slots.
**How to avoid:** Render all **23** from `compute/slotconst.go` (verified): Charm, Head, Face, Ear1, Ear2, Neck, Shoulders, Arms, Back, Wrist1, Wrist2, Range, Hands, Primary, Secondary, Finger1, Finger2, Chest, Legs, Feet, Waist, Power, Ammo. The data foundation keeps every slot (empty included, D-11). UI-SPEC §E encodes the 23-slot arrangement.
**Warning signs:** A paperdoll with 21 tiles; missing ear/wrist/finger pairs.

## Code Examples

### Extracting `lucy_img_ID` from the parsed `{{Itempage}}` params (the one-line gap)
```go
// Source: extends internal/backendsrv/enrich/wikiitem.go (params already parsed)
// In ParseItempage, after `params := parseTemplateParams(blockBody)`:
item := ParsedWikiItem{
    ItemName:     itemname,
    // ... existing fields ...
    IconID:       parseIconID(getParam(params, "lucy_img_ID", "")), // NEW
}
// parseIconID: atoi of the trimmed value; 0 when absent/blank/unparseable
// (0 is the "no icon yet" sentinel — the client falls back to the colored tile, D-02).
```

### The extend-only migration (verified pattern from 00003)
```sql
-- Source: pattern from internal/backendsrv/migrations/00003_enrich_columns.sql (verified)
-- +goose Up
ALTER TABLE item_master ADD COLUMN icon_id INTEGER;  -- nullable, no DEFAULT, no UNIQUE (extend-only)
-- +goose Down
ALTER TABLE item_master DROP COLUMN icon_id;
```

### Registering the new routes (the verified RequireSession wrap)
```go
// Source: cmd/squirebot-server/main.go (verified — every read route is RequireSession-wrapped)
mux.Handle("GET /api/v1/inventory/{char}", webauth.RequireSession(db, readapi.NewInventory(st)))
mux.Handle("GET /api/v1/characters",       webauth.RequireSession(db, readapi.NewCharacters(st, db)))
// (the roster handler may need db too, for the assignment join — like the assignment handlers)
```

### Web: typed fetch + colored-tile fallback (the locked D-02 pattern)
```svelte
<!-- Source: pattern from web/src/lib/api.ts getJSON + UI-SPEC §E/Color -->
<!-- api.ts: export const fetchInventory = (char, f=fetch) =>
       getJSON<CharacterInventory>(`/api/v1/inventory/${encodeURIComponent(char)}`, f); -->
<img
  src={`https://wiki.project1999.com/images/Item_${slot.icon_id}.png`}
  alt=""                                  
  style="image-rendering: pixelated; object-fit: contain;"
  onerror={(e) => (e.currentTarget.style.display = 'none')}  // reveal the colored-tile under-layer
/>
<!-- icon_id == 0 (or null): skip the <img> entirely; render the deterministic
     hsl(hue 45% 30%)→hsl(hue+40 40% 18%) gradient tile keyed on item name/id (D-02). -->
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Per-character view *tabs* (banned by Google's 200-tab limit) | Per-character master-detail drill-down (ONE reusable window rendered on selection) | v2.4, 2026-06-17 (CLAUDE.md) | This window IS the canonical drill-down — exactly the relaxed pattern, not a violation |
| Read API public (P14) | Read API session-gated (`RequireSession`) | P15, 2026-05-31 | Both new endpoints MUST be RequireSession-wrapped (NOT public) |
| Stored `wiki_url`/`wiki_summary` only | + `icon_id` (this phase) | Phase 31 | Extend-only column on the same weekly job |

**Deprecated/outdated:**
- The sketch-002 `#bagscrim` MODAL bag-open: superseded by D-04 inline expand. Do not port.
- "21 slots" (31-CONTEXT prose): superseded by the 23-slot `slotconst.go`.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The roster's "last_seen" + "is_guild_bot" warrant a NEW store read (no existing read returns the full shape) | Pitfall 4 | LOW — verified `CharsWithMeta`/`CharFreshness` shapes; a planner could instead compose two reads client-side, but one read is cleaner |
| A2 | `lucy_img_ID` is consistently present in `{{Itempage}}` for items with icons | D-03 / Code Examples | LOW — verified on 3 items; some pages (redirects/disambiguation/no-itempage) lack it, which is exactly why D-02 fallback exists; coverage ships incrementally |
| A3 | Storing `icon_id` as a column on `item_master` (vs. a mapping table) | Standard Stack / Don't Hand-Roll | LOW — both extend-only; column is the discretion default and rides the existing upsert; a mapping table is a valid alternative |
| A4 | Surfacing per-character `last_seen` once (on `CharacterInventory` or the endpoint envelope) for the examine "last synced" | Pitfall 2 / Open Q1 | LOW — it's a per-char value; the exact carrier is a planner choice |

**Note:** The single highest-risk research item (the wiki icon-id field) was VERIFIED live, not assumed — it is in the Sources, not here.

## Open Questions

1. **Where to carry the examine's "Last synced" value.**
   - What we know: `store.InventoryRow.LastSeen` (character.last_seen) exists and is fetched by `InventoryForChar`, but `compute.InventorySlot` has no `LastSeen` field; it's a per-character value.
   - What's unclear: add `LastSeen` to `CharacterInventory` (append-only struct field) vs. return it on the endpoint JSON envelope alongside the inventory.
   - Recommendation: add `LastSeen string` to `CharacterInventory` (append-only snake_case tag, sourced from the first non-empty row in `buildStructuredInventory`) — keeps the window component self-contained over one payload. Planner's discretion.

2. **Roster handler signature — `*store.Store` vs. `*sql.DB`.**
   - What we know: `readapi` handlers hold `*store.Store`; the assignment reads (`ListMyAssignments`) are `*sql.DB`-shaped functions; the existing roster meta source (`CharsWithMeta`) is a `*Store` method.
   - What's unclear: whether to add the new roster read as a `*Store` method (consistent with `readapi`) and do the viewer-assignment join inside it, or compose `*Store` + `*sql.DB` in the handler.
   - Recommendation: a single `*Store` method `RosterFor(ctx, viewerDiscordID)` returning the band-tagged, viewer-first rows — one tested SQL path, the handler stays thin (mirrors `views.go`). Planner's discretion.

3. **URL-reflecting the selected character.**
   - What we know: UI-SPEC §A allows `/characters?c=<name>` or `/characters/<name>` for deep-linking, planner's discretion; the tab strip keeps `aria-current` via `startsWith`.
   - Recommendation: `?c=<name>` query param (no new route file, deep-linkable, the single reusable window renders for the selected char). Avoids any per-character route file.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| P1999 MediaWiki API (`wiki.project1999.com/api.php`) | icon-id enrichment (D-01) | ✓ | live | weekly job logs+skips a failed page; coverage ships incrementally (D-02) |
| `wiki.project1999.com/images/Item_<id>.png` | icon render (D-02) | ✓ | live (200 image/png) | colored-tile `onerror` fallback (locked) |
| Go 1.25.7 toolchain | backend build | ✓ | 1.25.7 | — |
| goose migrations runner | `00012` apply | ✓ | (in repo, applies on boot) | — |
| Node/npm + vitest | web build + tests | ✓ | (in repo) | — |
| prod deploy (Hetzner VPS, atomic-swap per docs/backend-deploy.md) | browser-smoke (localhost can't auth) | ✓ | live api.squirebot.quest | full local stack (local backend + `SQUIREBOT_COOKIE_INSECURE` + `PUBLIC_API_BASE` + seeded `sb_session`) |

**Missing dependencies with no fallback:** None.
**Missing dependencies with fallback:** Wiki icon coverage is incremental by design (D-02) — not a blocker.

## Security Domain

> `security_enforcement: true`, ASVS level 1, block_on=high [VERIFIED: .planning/config.json]

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | `webauth.RequireSession` (Discord session cookie) — both new endpoints; NOT public (read API login-gated since P15) |
| V3 Session Management | yes | Existing rolling-TTL session (`resolveSessionUser` + `TouchSession`); no new session code this phase |
| V4 Access Control | yes | The roster + inventory are guild-wide reads (every member sees every character — the consolidated-views model); the gate is membership (RequireSession), NOT per-character ownership. The `{char}` param is NOT an access-control boundary — a member may view any character. No IDOR surface (read-only, no per-owner scoping to bypass). |
| V5 Input Validation / Output Encoding | yes | `{char}` path value → parameterized `?` bind only (verified `?`-only store seam); never string-concat into SQL or a content log. Examine HTML → `composeItemNote`/`escapeHtml` + `safeHttpUrl` scheme allow-list (the ONE `{@html}` sink). Search query → plain `{}` interpolation (auto-escaped) per StateBlock no-results. |
| V6 Cryptography | no | No new crypto. (The wiki SHA-1 is a content fingerprint, not a security hash — unchanged.) |

### Known Threat Patterns for {SvelteKit + Go net/http + SQLite}

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Stored XSS via guildie-named character/item in the examine/list | Tampering / Elevation | `escapeHtml` on every interpolated value in `composeItemNote`; plain `{}` interpolation elsewhere; NO new `{@html}` sink (T-14.02-01 / T-14.04-01 gates) |
| SQL injection via the `{char}` path param | Tampering | Parameterized `?` bind in `InventoryForChar` (the store is `?`-only by convention); the handler passes `char` as a value, never builds SQL |
| Reflected XSS via the search query | Tampering | Search is client-side filter; any echoed query renders via `{}` (auto-escaped), never `{@html}` (the StateBlock no-results precedent) |
| Unauthenticated data exfiltration | Info Disclosure | `RequireSession` wrap (401 fail-closed at the API, not just the UI); CORS echoes the EXACT origin + credentials (never wildcard) |
| Row content / query leaking into logs | Info Disclosure | V7: slog records carry op + count + status + err ONLY — never item/char names or the `char`/`q` value (the views.go/itemsearch.go discipline) |
| `javascript:`/`data:` URL in a wiki link | Tampering | `safeHttpUrl` http(s) scheme allow-list before any href (existing in `composeNotes.ts`) |
| Malicious icon `iconId` driving an arbitrary `<img>` URL | Tampering | `icon_id` is an INTEGER from the trusted weekly job; the URL is `Item_${int}.png` (no user string in the path); a bad/absent id → colored-tile fallback |

**No new security primitives required** — every control already exists and is reused. The phase adds two read endpoints that inherit the shipped `RequireSession` + CORS + V7-logging + `{@html}`-escaping posture.

## Validation Architecture

> `nyquist_validation: false` [VERIFIED: .planning/config.json] — this section is informational, not a required gate.

### Test Framework
| Property | Value |
|----------|-------|
| Backend framework | Go `testing` (`go test ./...`) |
| Web framework | vitest, **node-only `server` project** [VERIFIED: web/vite.config.ts — `environment: 'node'`, no jsdom/@testing-library] |
| Web config file | `web/vite.config.ts` |
| Quick run (web) | `npm test` (in `web/`) |
| Quick run (backend) | `go test ./internal/backendsrv/...` |
| Full suite | `go test ./...` + `npm test` + `npm run check` + `npm run build` (the established green-gate set) |

### Sampling Rate
- **Per task commit:** the touched package's tests (`go test ./internal/backendsrv/{readapi,enrich,store,compute}/...` or `npm test` for web).
- **Per wave merge:** `go test ./...` + `npm run check` (0/0) + `npm test` + `npm run build`.
- **Phase gate:** full suite green, THEN the mandatory **deploy-then-browser-smoke** (node tests are DOM-blind — Pitfall 1).

### Wave 0 Gaps
- Backend: extend existing `_test.go` (e.g. `enrich/wikiitem_test.go` for the `lucy_img_ID` parse incl. absent/blank/non-numeric; `readapi` handler tests mirroring `readapi_test.go` for 405 / 401-without-session / shape; a roster read test for the three-band ordering + viewer-first). No new framework — existing fixtures (`enrich/testdata/`, the store test helpers) cover it.
- Web: node tests can cover the PURE pieces (a viewer-first sort/filter helper like `myview.ts`/`groupByChar.ts` precedent, an icon-url builder, the examine field-omission logic) but CANNOT cover the DOM (paperdoll, hover/pin, inline expand, `onerror`). Extract logic into pure `web/src/lib/` modules with node tests (the established pattern), and rely on browser-smoke for the DOM.
- **The window-rendering correctness is browser-smoke-only** — plan the smoke explicitly.

## Sources

### Primary (HIGH confidence)
- **Codebase (grep/read, verified this session):**
  - `internal/backendsrv/readapi/{views.go,meta.go,itemsearch.go,cors.go}` — the handler idiom, nil→[] coercion, V7 logging, CORS.
  - `internal/backendsrv/webauth/session.go` — `RequireSession` / `RequireOfficer` / `UserFromContext` (viewer identity).
  - `internal/backendsrv/compute/{inventory.go,types.go,slotconst.go}` — `StructuredInventory`, `CharacterInventory`/`InventorySlot` JSON contract, the 23-slot set.
  - `internal/backendsrv/store/{readviews.go,assignment.go,charmeta.go,enrich.go}` — `InventoryRow.LastSeen`, `CharsWithMeta`/`CharFreshness`/`ListMyAssignments` shapes, `ItemMaster`/`UpsertItemMasterTx`, `item_master` id-join.
  - `internal/backendsrv/enrich/{wikiitem.go,jobs/wiki.go,jobs/urls.go}` — `ParseItempage`/`parseTemplateParams`, the weekly job upsert path, `wikiParseURL`.
  - `internal/backendsrv/migrations/{00001_init.sql,00003_enrich_columns.sql}` — `item_master` schema (`item_id PK, ..., last_refreshed`), the ADD COLUMN pattern; latest migration `00011`.
  - `cmd/squirebot-server/main.go` — every read route `RequireSession`-wrapped; route registration form.
  - `web/src/lib/{api.ts,tooltip/composeNotes.ts}`, `web/src/lib/components/{ItemTooltip.svelte,StateBlock.svelte,SiteShell.svelte}`, `web/src/routes/characters/+page.svelte`, `web/vite.config.ts` — fetch/typed-error core, the `{@html}` sink, state copy, the scoped-search shell, the node-only test config.
  - `.planning/config.json` — workflow toggles (nyquist off, security on/L1).
- **P1999 MediaWiki API (verified live this session):**
  - `GET https://wiki.project1999.com/api.php?action=parse&prop=wikitext&format=json&page=Cloak_of_Flames` → `{{Itempage}}` param `lucy_img_ID = 658`.
  - …`page=Wurmslayer` → `lucy_img_ID = 736`. …`page=Ring_of_the_Ancients` → `lucy_img_ID = 563`. (All match sketch-002.)
  - `curl` of `https://wiki.project1999.com/images/Item_{658,736,563}.png` → HTTP 200, `image/png`.

### Secondary (MEDIUM confidence)
- `.planning/phases/31-characters-tab-in-game-inventory-window/{31-CONTEXT.md,31-UI-SPEC.md}` — locked decisions D-01..D-11 + the APPROVED visual/interaction contract.
- `.planning/{REQUIREMENTS.md,ROADMAP.md,STATE.md}` — CHAR/INV requirements, the 29→30→31 chain, the consolidated-views relaxation.

### Tertiary (LOW confidence)
- None — every load-bearing claim was verified against the codebase or the live wiki this session.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new deps; all reuse verified by reading the files.
- Architecture (read-API + enrich + web): HIGH — every pattern read from shipped code; the icon-id field verified live.
- Pitfalls: HIGH — each traces to a verified code fact (node-only tests, `LastSeen` absence on the slot, `is_guild_bot`/`last_seen` absence on `CharsWithMeta`, the id-vs-name join, the sketch modal).

**Research date:** 2026-06-18
**Valid until:** 2026-07-18 (stable — internal codebase + a stable wiki API; re-verify the `lucy_img_ID` field only if wiki coverage looks low at build time)
