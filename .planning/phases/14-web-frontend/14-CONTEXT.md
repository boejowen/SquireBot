# Phase 14: Web Frontend - Context

**Gathered:** 2026-05-30
**Status:** Ready for planning

<domain>
## Phase Boundary

Give the guild back its **read UI as a real website**: a versioned Go **read API** (`BACKEND-05`) plus a **static SvelteKit app** that renders the four consolidated views (`view`, `gear_check`, `spell_check`, `bank`) — each with a leading `Char` column — as a reusable **filterable/sortable data grid**, with **cross-character fuzzy search** ("did you mean?"), **rich HTML item tooltips** (wiki summary + price + quest + wiki link), and **site-wide EQ theming**. This is the visible replacement for the Sheet's UI.

**In scope:** BACKEND-05 (versioned read API powering the 4 views), WEB-01 (the 4 views as a filterable/sortable grid), WEB-02 (`gear_check` OK/MISSING/OTHER vs Velious tiers + `spell_check` KNOWN/MISSING, matching v1 semantics), WEB-03 (client-side fuzzy search + "did you mean?", <2s), WEB-04 (per-row HTML tooltips + direct wiki link), WEB-05 (EQ aesthetic theme site-wide).

**Out of scope (later phases):** Discord OAuth2 login + any write/admin forms (eviction, bank-coin, admin-mgmt) → **P15**; shadow-soak / human-data backfill / coordinated watcher flip / Sheet decommission → **P16**. P14 is **read-only, no login, no writes**.

</domain>

<decisions>
## Implementation Decisions

> **Discussion shape (2026-05-30):** the user engaged on **one** area — *Read access posture* (D-04/D-05) — and delegated the other three to my recommendation (the standing `feedback_delegate_gray_areas` pattern, same as Phase 11). Everything below is locked; downstream research/planning should NOT re-ask. Fine-grained *visual* layout is deliberately left to `/gsd-ui-phase 14` (the user's chosen next step).

### Read access posture (USER-DECIDED)
- **D-04:** **Public read in P14, Discord-gate in P15.** P14 ships the website + read API **open to anyone with the URL** — the Go read API allows the static site's origin via CORS; no auth gate in P14. This unblocks the guild immediately (front-load principle). **P15's Discord login (AUTH-08, already a P15 requirement) then walls read access to guild members.** → recorded as a P15 forward-dependency in `<deferred>`. The data is low-sensitivity EQ inventory (character names + item lists), consistent with the guild's universal-visibility ethos.
- **D-05:** **Keep the public site out of search engines** — emit a `noindex` meta tag + a `robots.txt` disallow. "Public" means *anyone with the link*, not *anyone who Googles the guild name*. Costs nothing.

### Compute split — server vs client (DELEGATED → server-computed view endpoints)
- **D-01:** **The Go backend is the query/compute layer.** It exposes **per-view read endpoints** under `/api/v1/` that return **ready-to-render rows** for each of the four views; the SvelteKit client is **thin** (fetch JSON → render grid). This matches BACKEND-05's wording ("the API … as the query layer"), keeps payloads small, and avoids shipping the whole dimension dataset to every browser. Suggested shape (planner refines): `GET /api/v1/views/view`, `/views/gear_check`, `/views/spell_check`, `/views/bank`, plus a small `GET /api/v1/meta` (character list, per-character last-synced timestamps, available themes) for the client shell.
- **D-02:** **Gear/spell progression logic is reimplemented in Go**, using the tested v1 TypeScript (`buildGearCheck.ts`, `buildSpellCheck.ts`, `buildView.ts`, `buildBank.ts`) **+ their `__tests__` as the behavioral oracle** — the Go output MUST match v1 semantics exactly (gear OK/MISSING/OTHER vs Velious tiers; spell KNOWN/MISSING) for the same character (WEB-02). Port the test fixtures/expectations across so parity is provable.
- **D-03:** **Search and tooltip *presentation* stay client-side, reusing the tested pure-TS modules.** Server compute = relational/data layer; client = presentation + search-over-already-fetched-rows. Specifically: the client fetches the `view` rows (all characters' inventory, with enrichment fields inline) and runs the **ported `searchIndex.ts`** (Levenshtein / `didYouMean`) in-browser — small data (~12 users), so <2s is trivial (WEB-03); the per-row tooltip HTML is composed client-side via the **ported `composeNotes.ts`** from enrichment fields present in the payload (WEB-04). The read endpoints therefore include the raw enrichment fields each row needs (wiki summary text, current price, quest info, item ID for the wiki link) — the API returns *data*, the client formats *presentation*.

### Theme catalog + default (DELEGATED → 5 EQ themes, Velious default)
- **D-06:** Ship the **five EQ themes** — **Vanilla / Kunark / Velious / Minimalist / Heavy**. **Drop `sheets-default`** (meaningless without Sheets). **Default = `velious`** (the era the guild plays; this is the guild's own site, so lean into identity rather than the Sheet's "don't scare a fresh non-EQ viewer" default of Minimalist). Theme = **per-user `localStorage`** preference (LOCKED upstream; no server write), selected via a picker in the site shell. Port the `THEMES` registry (`apps-script/src/lib/themes.ts`) to **CSS custom properties** (`:root` vars), with colors still derived from `docs/design/eq-aesthetic-theme.md` per the CLAUDE.md convention. **Heavy's parchment/stone textures now render properly** with real CSS (background images/gradients) — the Sheet limitation that degraded Heavy to a solid tan is gone (full CSS freedom, WEB-05).
- **D-07 (Claude's discretion lean):** **Self-host the webfonts** (Cinzel, Cinzel Decorative, IM Fell English, MedievalSharp, Crimson Text, Inter as `woff2`) rather than pulling from Google Fonts at runtime — consistent with the milestone's "Off Google" ethos and better for a static site's performance/offline story. Planner may override if self-hosting proves heavy.

### Tooltip + search behavior (DELEGATED → hover/tap popover + inline "did you mean?")
- **D-08:** **Tooltips open on hover (pointer devices) and on tap/click (touch devices)** as a rich-HTML popover: wiki summary + current price (from `pigparse_price`) + quest info + a real `<a>` link to the item's `wiki.project1999.com` page (WEB-04). Dismiss on outside-tap / Esc. Content via the ported `composeNotes.ts` (rich HTML — no plain-text cell-note cap anymore).
- **D-09:** **"Did you mean?" = a clickable inline suggestion of the single best fuzzy hit** shown only when a search returns no exact match (ported `didYouMean`); clicking it re-runs the search with the suggestion. **Fix the two carried bugs during the port: 999.28 (`didYouMean('')` empty-query contract) and 999.30 (`searchIndex.test.ts` Test 4 Levenshtein contract mismatch).** Search is cross-character and surfaces which character(s)/location(s) hold the item — the "what's missing, where is it in the guild?" core value.

### API consistency
- **D-10:** The read endpoints **extend the existing hand-rolled backend** — `net/http` ServeMux (Go 1.22+ method+pattern routing) over the `modernc.org/sqlite` store in `internal/backendsrv/` — and follow the established `/api/v1/...` versioning. Mirror the existing route/handler patterns (`ingest/handler.go`, `ingest/whoami.go`, `ingest/version.go`) for the new read handlers; add read query methods to the `store` package alongside the existing ones (`replace.go`, `binding.go`, `enrich.go`, `itemids.go`). No new web framework on the backend.

### Claude's Discretion
- Exact read-endpoint URL shapes, JSON field names, and pagination/streaming (data is tiny — likely none needed).
- TanStack Table configuration specifics (per-column vs global filter, sticky header/Char column, default sort) — **high-level grid behavior is in scope for `/gsd-ui-phase 14`**; treat detailed visual/interaction layout as that step's contract, not this one's.
- Self-hosted vs CDN fonts if D-07's self-hosting proves impractical.
- Whether search runs over the `view` payload already in memory or a dedicated lightweight search dataset/endpoint (D-03 leans in-memory).

</decisions>

<specifics>
## Specific Ideas

- **"Off Google" extends to the frontend** — no Google runtime dependency (hence D-07 self-hosted fonts); the static app is deployed on Cloudflare/GitHub Pages, the API on the Hetzner box behind Caddy.
- **Reuse the tested v1 logic, don't reinvent it** — `buildView`/`buildGearCheck`/`buildSpellCheck`/`buildBank` + `searchIndex` + `composeNotes` + `themes` are battle-tested with vitest; their tests are the oracle for parity (Go reimplementation for compute; TS port for search/tooltip/theme presentation).
- **Core value framing for search:** the answer to a search is "*which character(s) and location(s) hold this item*" — cross-character, not per-character.
- **Public-but-unlisted:** D-04 + D-05 together = the guild can use it today without a login wall, while it stays off web search until P15 adds the real Discord gate.

</specifics>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase + milestone planning
- `.planning/ROADMAP.md` § "Phase 14: Web Frontend" — goal, 5 success criteria, the porting Note (pure-logic modules, views-computed-on-read, theme=localStorage)
- `.planning/REQUIREMENTS.md` § WEB + `BACKEND-05` — acceptance detail for WEB-01..05 + the read API
- `.planning/PROJECT.md` § Current Milestone + Constraints + Key Decisions — locked stack (SvelteKit static + TanStack Table + Tailwind), consolidated-view architecture, Off-Google ethos

### v2.0 milestone research
- `.planning/explorations/website-milestone/SCOPE.md` — milestone synthesis; frontend + read-API direction
- `.planning/explorations/website-milestone/04-data-enrichment-migration.md` §6 — DB schema + the Postgres→SQLite port notes (read these for the read-query joins)
- `.planning/phases/11-backend-foundation-ingest-api/11-CONTEXT.md` — backend decisions the read API must stay consistent with (hand-rolled `net/http`, `modernc.org/sqlite`, `/api/v1`, schema D-13)

### Design / theme (WEB-05)
- `docs/design/eq-aesthetic-theme.md` — the 5 EQ themes' palettes/fonts/visual character + the THEMES-registry pattern (now → CSS custom properties on the web)
- `docs/design/mockups/eq-aesthetic-preview.html` — side-by-side theme preview (visual reference for the web port)
- `docs/design/mockups/eq-aesthetic-picker.html` — picker UX reference

### Existing code to PORT / MIRROR (the heart of P14)
- `apps-script/src/tabs/buildView.ts` + `__tests__/buildView.test.ts` — `view` semantics (WEB-01); reimplement compute in Go (D-02)
- `apps-script/src/tabs/buildGearCheck.ts` + `__tests__/buildGearCheck.test.ts` — gear OK/MISSING/OTHER vs Velious tiers (WEB-02); Go reimpl oracle
- `apps-script/src/tabs/buildSpellCheck.ts` + `__tests__/buildSpellCheck.test.ts` — spell KNOWN/MISSING (WEB-02); Go reimpl oracle
- `apps-script/src/tabs/buildBank.ts` + `__tests__/buildBank.test.ts` — `bank` view (WEB-01); coin is null/0 until P15
- `apps-script/src/lib/searchIndex.ts` + `__tests__/searchIndex.test.ts` — Levenshtein/`didYouMean` (WEB-03); port to client TS, **fix 999.28 + 999.30**
- `apps-script/src/tabs/composeNotes.ts` + `__tests__/composeNotes.test.ts` — tooltip composition (WEB-04); port to client TS → rich HTML
- `apps-script/src/lib/themes.ts` + `__tests__/themes.test.ts` — `THEMES` registry (WEB-05); port → CSS custom properties
- `apps-script/src/lib/eq-constants.ts` — EQ constants the views depend on
- `internal/backendsrv/store/*.go` (`db.go`, `replace.go`, `binding.go`, `enrich.go`, `itemids.go`) — SQLite store to extend with read queries (D-01/D-10)
- `internal/backendsrv/ingest/{handler,whoami,version}.go` + `cmd/squirebot-server` — existing `/api/v1` ServeMux/handler patterns to mirror for the read endpoints (D-10)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **v1 view builders** (`buildView`/`buildGearCheck`/`buildSpellCheck`/`buildBank`, TS + vitest) — the semantic oracle for the Go read-endpoint compute (parity-test against their expectations).
- **Pure-logic modules** (`searchIndex`, `composeNotes`, `themes`, TS + vitest) — port near-verbatim to the SvelteKit client (search, tooltip HTML, theme CSS vars).
- **Backend store + ServeMux** (`internal/backendsrv/store`, `internal/backendsrv/ingest`) — read endpoints extend these; same `net/http` + `modernc.org/sqlite` + `/api/v1` patterns.
- **Dimension/enrichment tables** (`item_master`, `pigparse_price`, `wiki_spells`, `wiki_gear_tier`, `quest_items`) — already populated on cadence by P12; the read joins consume them.

### Established Patterns
- **Consolidated views with a leading `Char` column** (LOCKED, CLAUDE.md) — the grid is one filterable mega-view per type, never per-character.
- **Views computed on read** (no `onChange`/`buildView` rebuild, no search-cache machinery) — server computes per request (D-01).
- **`/api/v1/...` API versioning** replaces the retired `_meta.schema_version`/`WatcherMaxSchemaVersion` handshake.

### Integration Points
- Read API ← the SvelteKit static client (CORS, D-04).
- Read API → the SQLite store P11 created + P12 populated.
- (P15) Discord login will gate this read API/site; (P15) admin write forms will add coin to the `bank` view + write actions.

</code_context>

<deferred>
## Deferred Ideas

- **Discord-login gate on read access → P15** (D-04). P14 is public; P15's AUTH-08 Discord login walls reads to guild members. **Forward-dependency the P15 planner must honor.**
- **Admin write forms** (eviction / bank-coin entry / admin-mgmt) → **P15**. The `bank` view shows characters now; **coin values are null/0 in P14** until P15's bank-coin form (ADMIN-05) populates them.
- **Fancy theme-picker preview tiles** (the Sheet's 3×2 live-preview grid) — P14 can ship a simpler dropdown/swatch picker; live-preview tiles are optional polish.
- **Detailed visual/interaction layout** (grid column config, sticky headers, responsive breakpoints, exact tooltip/popover styling) → captured by `/gsd-ui-phase 14` (the next step), not here.
- **Cutover / shadow-soak / Sheet decommission** → **P16**.

</deferred>

---

*Phase: 14-web-frontend*
*Context gathered: 2026-05-30*
