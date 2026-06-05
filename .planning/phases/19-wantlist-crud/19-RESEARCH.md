# Phase 19: Wantlist CRUD - Research

**Researched:** 2026-06-03
**Domain:** Brownfield CRUD on the live Go + SQLite + SvelteKit stack — a personal, private, owner-scoped wantlist with a deep "who-in-the-guild-holds-it" join
**Confidence:** HIGH (every pattern read directly from the shipped P14/P16/P17 code; no greenfield)

## Summary

Phase 19 is the `account.go` / `/account` twin applied to a new domain. It is **almost entirely
composition of existing, tested seams** — not new infrastructure. The backend adds one forward-only
goose migration (`00006_wantlist.sql` → `wantlist_item` + an `alert_log` stub for Phase 20), one
`webadmin/wantlist.go` handler file (mirroring `account.go`'s session-derived-owner / `withTx` /
`AppendAuditTx` / IDOR-scoped-update shape), one `store/wantlist.go` query file (mirroring
`linking.go`'s owner-scoped `*Tx` mutators + plain-`*Store` readers), three new mux routes, and the
frontend adds `web/src/routes/wantlist/+page.svelte` (cloning `/account`'s `.form-card` shell +
`WatcherCodesPanel`'s load→confirm→reload lifecycle) plus a `wantlistColumns` entry in `columns.ts`
(the 5th `DataGrid` instantiation) and three typed wrappers in `api.ts`.

**The single load-bearing research finding** that the planner MUST internalize: **there is no
standalone item catalog.** `item_master` exists, but it is populated ONLY by the weekly wiki job
iterating `store.DistinctInventoryItemIDs()` — i.e. the "catalog" is exactly *the set of items that
have ever been seen in some guildie's inventory.* There is no server-side item-name search endpoint
today; the existing search (`searchIndex.ts`) runs **client-side, in-memory, over the already-fetched
`view` rows.** This is good news: the add-item catalog search, the "did you mean?" fuzzy match, AND
the deep in-bank "who holds it" join can ALL be served by **one existing payload — the `/api/v1/views/view`
data the page already knows how to fetch** — with zero new read endpoints. The catalog corpus and the
in-bank corpus are the same `ViewRow[]`. A custom (NULL-`item_id`) want is, by definition, an item NOT
in that corpus — which is exactly why it cannot be in-bank-joined and cannot be monitor-matched later.

> **SUPERSEDED IN PART by D-10 — see the Addendum at the END of this file.** CONTEXT.md decision D-10
> (revised 2026-06-03) changes the *catalog-search corpus* from the client-side `view` payload to the
> **full Blue item set in `pigparse_price`**, served by a NEW server-side `GET /api/v1/items/search`
> endpoint. The in-bank "who holds it + count" join (D-06) is UNCHANGED — it remains a client-side
> derivation of `fetchView()`. Where this Summary, Pattern 5, Open Question 1, and Assumption A3 say
> "catalog search is client-side over `view`", read the Addendum's override for the search half only.

**Primary recommendation:** Build `wantlist.go` as a verbatim structural clone of `account.go`
(session owner via `caller(ctx)`, `withTx` + `AppendAuditTx`, owner-scoped `UPDATE ... WHERE id=? AND
discord_user_id=?` for remove). Resolve the add-item catalog search, the "did you mean?", and the
in-bank holder lines entirely **client-side over the existing `fetchView()` payload** — do NOT build a
new item-search or in-bank read endpoint. Store wants keyed on `web_user.discord_user_id` (the Discord
identity, NOT `owner`). Snapshot `item_name` at add-time so a custom want and the display label survive.
*(Catalog-search half revised by D-10 — see Addendum.)*

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Entry fields (WANT-01)**
- **D-01:** A `wantlist_item` row captures: `item_id` (nullable — see D-04), `reason` **required**
  (`buy` | `quest`), `priority` (`low` | `med` | `high`, **default `med`**), and an **optional** `note`
  (freeform text, **≤ 280 chars**).
- **D-02:** Priority is a simple Low/Med/High enum — NOT a numeric rank. It drives the default list sort.

**Add-item scope & dedupe (WANT-01)**
- **D-03:** Add-item search is **catalog-first**: search the existing item catalog by name + ID, reusing
  the existing fuzzy/"did you mean?" matching. A catalog hit pins a real `item_id`.
- **D-04:** **Custom text wants are allowed** — a want that does NOT resolve to a catalog item (`item_id`
  NULL, free-text label). Custom wants are **display-only**: excluded from the in-bank join (D-06) and
  CANNOT be auto-matched by the later EC/WTS/raid monitors (Phases 21–23). The UI must clearly mark a
  custom want as "custom — won't trigger alerts".
- **D-05:** **The same item may appear twice with different reasons** (one `buy`, one `quest`).
  Uniqueness key is **`(web_user, item_id, reason)`** for catalog items. For custom wants (NULL
  `item_id`), dedupe on `(web_user, custom_label, reason)` or skip the DB constraint and dedupe in the
  handler — planner's call; the user-visible rule is "no exact duplicate (same item + same reason)".

**In-bank indicator (WANT-02)**
- **D-06:** The "already in the guild bank?" indicator goes **deep**: show **which character(s) across
  the entire guild hold the item and the count**. This is a join against the existing **consolidated
  all-character inventory** data (not bank-toon-only).
- **D-07:** Only catalog wants (with an `item_id`) get an in-bank result; custom wants show "—"/not-
  applicable. The join key is the stable `item_id`.

**Catalog source (added after the body was written)**
- **D-10:** The searchable catalog is the **full Blue item set sourced from the daily PigParse `getall`
  ingest** (`pigparse_price`), NOT `item_master`. This almost certainly requires a **new server-side
  item-search endpoint**. See the **Addendum** at the end of this file for the full resolution. The
  in-bank join (D-06) is unaffected — only the catalog *search* changes.

### Claude's Discretion
- **D-08 (List presentation):** Reuse the existing filterable/sortable **DataGrid** + the existing rich
  HTML item tooltips. **Default sort: priority (high→low) then in-bank status.** Friendly empty state.
  Planner may adjust columns.
- **Security/identity shape (locked upstream):** the `webadmin/account.go` twin — login-only, owner
  derived **server-side** from the Discord session (never client-supplied, per v2.1 D-02), IDOR-safe,
  audited via the existing `audit.go` seam.
- Custom-want storage detail (separate `custom_label` column vs reusing `note`) is the planner's call —
  but a custom want MUST be visually distinct and excluded from matching.

### Deferred Ideas (OUT OF SCOPE)
- **Guild-wide "who wants what" roll-up** — aggregating everyone's wantlists into a shared view.
- **WTB monitoring / price-threshold alerts** — alert-side refinements (Phases 21+).
- **Auto-deriving the catalog entry for a custom want later** (promote a custom text want to a real
  `item_id` once the catalog learns it).
- **All Discord/DM/monitor work** — Phases 20–23. Phase 19 is pure website CRUD.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| WANT-01 | A signed-in guildie can add an item to their personal wantlist by searching the existing item catalog, tagging a reason (buy vs quest), and optionally a priority and note — tied to their Discord identity (`web_user`). | `account.go` mint handler is the write-shape twin; `searchIndex.ts` + the `fetchView()` payload is the catalog-search corpus; `wantlist_item.discord_user_id` FK→`web_user` ties identity (per ARCHITECTURE-v2.2 §State/identity). **Catalog search revised by D-10 → server-side `pigparse_price` search; see Addendum.** |
| WANT-02 | A guildie can view and manage their wantlist on squirebot.quest (list all wanted items, remove any, and see an "already in the guild bank?" indicator for each). | `WatcherCodesPanel` is the list+confirm-remove lifecycle twin; `RevokeOwnCodeTx` is the owner-scoped IDOR-safe remove twin; the `view` payload (`compute.View` / `InventoryJoin(bankOnly=false)`) is the in-bank "who holds it" corpus — a client-side group-by-`item_id`, not a new endpoint. |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Wantlist persistence (CRUD) | API / Backend (`webadmin/wantlist.go` + `store/wantlist.go` + `00006`) | Database | Owner-scoped writes + audit must be server-authoritative; identity from session, never the body (v2.1 D-02). |
| Owner/identity resolution | API / Backend | — | `caller(ctx)` = `webauth.UserFromContext` — the session is the only trusted owner source (IDOR boundary). |
| Add-item catalog search + "did you mean?" | Browser / Client | — | The corpus is the already-fetched `view` rows; `searchIndex.ts` runs in-memory client-side today. NO new server endpoint. **REVISED by D-10 → API/Backend `pigparse_price` search; see Addendum.** |
| In-bank "who holds it + count" join | Browser / Client | API (read: `view` payload only) | D-06's "deep" display is a client-side group-by-`item_id` over the same `ViewRow[]` the page fetches. A join, not a rebuild, not a new endpoint. |
| List render / sort / filter (DataGrid) | Browser / Client | — | 5th `DataGrid` instantiation; `wantlistColumns` + `defaultSorting` seed. |
| XSS escaping of item names / labels / notes | Browser / Client | — | Plain `{}` Svelte auto-escape; the only `{@html}` sink stays `ItemTooltip`→`composeItemNote`. |

## Standard Stack

### Core (all REUSE — Phase 19 adds NO new dependency)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `modernc.org/sqlite` | v1.51.0 | CGO-free SQLite driver | [VERIFIED: go.mod] The only DB; `wantlist_item`/`alert_log` are new tables on it. |
| `github.com/pressly/goose/v3` | v3.27.1 | Forward-only embedded migrations | [VERIFIED: go.mod] `00006_wantlist.sql` is the 6th in the embedded `//go:embed *.sql` set. |
| net/http stdlib `ServeMux` | Go 1.24 | Routing (`mux.Handle("POST /api/v1/...", webauth.RequireSession(...))`) | [VERIFIED: cmd/squirebot-server/main.go:312-314] The established hand-rolled router; method-in-pattern. |
| SvelteKit static site | (existing) | `web/src/routes/wantlist/+page.svelte` | [VERIFIED: web/src/routes/] Clones `/account`. |
| `@tanstack/table-core` (via `createSvelteTable`) | 8.21.3 | Headless grid engine behind `DataGrid.svelte` | [VERIFIED: npm view / package.json] 5th instantiation; `wantlistColumns` ColumnDef[]. |
| `@lucide/svelte` | (existing) | Icons (search, plus, trash-2, loader, triangle-alert, sort caret) | [VERIFIED: WatcherCodesPanel imports `@lucide/svelte/icons/*`] |

### Supporting (REUSE — existing seams the new code calls)
| Seam | Location | Purpose | When to Use |
|------|----------|---------|-------------|
| `caller(ctx)` | webadmin/officers.go:58 | Reads session `discord_user_id` (`webauth.UserFromContext`) | Every handler — the owner source (NEVER the body). |
| `withTx` | webadmin/audit.go:88 | `BEGIN IMMEDIATE` tx wrapper (deferred-rollback panic-safe) | Every mutating handler: write + audit atomically. |
| `AppendAuditTx` | webadmin/audit.go:57 | Append-only `audit_log` INSERT inside the tx | Each add/remove — `event` = `wantlist_add` / `wantlist_remove`. |
| `writeJSON` / `writeJSONError` | webadmin/officers.go:37-48 | `{...}` / `{"error":"code"}` bodies | The frontend `api.ts` routes off these. |
| `nowUnix()` | webadmin/officers.go:52 | Unix epoch-seconds stamp (pinnable in tests) | `created_at` + audit `at` (web-side epoch-secs convention). |
| `searchIndex.ts` (`searchRows` / `didYouMean` / `groupAndSort`) | web/src/lib/search/searchIndex.ts | In-memory fuzzy item search + "did you mean?" | The add-item catalog search corpus. Already has the 999.28 empty-query guard. **Catalog-search use revised by D-10 — see Addendum.** |
| `DataGrid.svelte` + `columns.ts` | web/src/lib/ | The one filterable/sortable grid | 5th instantiation for the wantlist. |
| `ItemTooltip` / `ConfirmDialog` / `StateBlock` / `FormField` | web/src/lib/components/ | Tooltip / destructive-confirm / load-states / form rhythm | Per 19-UI-SPEC component-reuse map. |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Client-side catalog search over `view` payload | A new `GET /api/v1/items/search?q=` endpoint hitting `pigparse_price` | **D-10 SELECTS the endpoint.** The body originally rejected an endpoint (the `view` corpus sufficed); D-10 expands the corpus to the full Blue set, which lives only in `pigparse_price` server-side and is too large to ship client-side — so the endpoint is now REQUIRED. See Addendum. |
| Separate `custom_label` column | Reuse `item_name` (snapshot) for both catalog + custom | Either works (D-04 planner's call). Recommendation below: ONE `item_name TEXT NOT NULL` snapshot column carries both — `item_id` NULL ⇔ custom. Simpler schema, matches ARCHITECTURE-v2.2's sketch. |
| Soft-delete (`active` flag) | Hard `DELETE` | ARCHITECTURE-v2.2 sketched `active INTEGER DEFAULT 1` so `alert_log` FKs survive a removed want. For P19 (no alert_log writes yet) a hard DELETE is simpler, BUT the FK-survival concern is real for Phase 20. **Recommendation: soft-delete now** (`active`), so the remove is `UPDATE ... SET active=0` — and Phase 20's `alert_log.wantlist_item_id` FK never dangles. See Pitfall 4. |

**Installation:** None. Phase 19 adds zero dependencies (confirmed against go.mod + package.json).

## Architecture Patterns

### System Architecture Diagram

```
┌─────────────────────────── Browser (SvelteKit /wantlist) ────────────────────────────┐
│                                                                                       │
│  onMount load:                                                                        │
│    fetchOwnWants()  ─────────────► GET /api/v1/wantlist        (RequireSession)        │
│    fetchView()      ─────────────► GET /api/v1/views/view      (RequireSession, EXISTING)
│         │                                  │                                          │
│         ▼                                  ▼                                          │
│   WantlistRow[]                     ViewRow[]  (the IN-BANK corpus; catalog now server-side per D-10)
│         │                                  │                                          │
│         ├── add-item search ──── GET /api/v1/items/search?q=  (D-10, NEW server endpoint)
│         │                        → CatalogItem[] {item_id,name,price} → pinned item_id OR custom
│         │                                                                             │
│         └── per row: in-bank join = group viewRows by item_id ─► "↳ <Char>: <count>"  │
│                      (client-side; D-06 deep holder lines; custom→"—")  UNCHANGED      │
│                                                                                       │
│  add  ──► POST /api/v1/wantlist        {item_id?|null, item_name, reason, priority, note}
│  remove ► POST /api/v1/wantlist/remove {id}      (owner NEVER in body — D-02)         │
└───────────────────────────────────────────────────┬───────────────────────────────────┘
                                                     │ credentialed fetch (cookie)
                                                     ▼
┌──────────────────── cmd/squirebot-server (net/http mux) ──────────────────────────────┐
│  webauth.RequireSession ─► webadmin/wantlist.go                                        │
│     caller(ctx) = discord_user_id (session; the ONLY owner source)                    │
│     withTx { store.*Tx mutator + AppendAuditTx }   (BEGIN IMMEDIATE, atomic)          │
│                              │                                                         │
│  webauth.RequireSession ─► readapi item-search (D-10) ─► store.SearchCatalog(q) ──┐    │
│                              ▼                                                     ▼    │
│   store/wantlist.go  ── owner-scoped SQL (WHERE discord_user_id=?) ──► SQLite  pigparse_price (catalog)
│     ListOwnWants / AddWantTx / RemoveOwnWantTx          wantlist_item (NEW, 00006)     │
│                                                          alert_log (NEW stub, 00006)   │
└────────────────────────────────────────────────────────────────────────────────────┘
```

### Recommended Project Structure
```
internal/backendsrv/
├── migrations/
│   └── 00006_wantlist.sql        # NEW: wantlist_item + alert_log (stub); forward-only, 00001-00005 untouched
├── store/
│   └── wantlist.go               # NEW: AddWantTx / ListOwnWants / RemoveOwnWantTx (linking.go shape)
│   └── wantlist_test.go          # NEW: NewTestDB-backed store tests
│   └── itemsearch.go             # NEW (D-10): SearchCatalog over pigparse_price (readviews.go shape)
├── webadmin/
│   └── wantlist.go               # NEW: 3 handlers (account.go twin: caller / withTx / AppendAuditTx / IDOR)
│   └── wantlist_test.go          # NEW: table-driven handler tests (withCaller / postJSON helpers)
├── readapi/
│   └── itemsearch.go             # NEW (D-10): GET /api/v1/items/search handler (views.go twin)
└── (migrate_test.go              # EXTEND: TestMigrate_00006_AddsWantlist — same idiom as 00005 test)

cmd/squirebot-server/main.go       # MODIFIED: 3 new RequireSession routes (twin of /account block)
                                    #   + 1 RequireSession route GET /api/v1/items/search (D-10)
                                    #   + main_test.go anon→401 / member→admitted cases

web/src/
├── routes/wantlist/+page.svelte   # NEW: clones /account .form-card shell + intro copy
├── lib/components/WantlistPanel.svelte   # NEW: clones WatcherCodesPanel load→confirm→reload lifecycle
├── lib/components/WantAddForm.svelte     # NEW: catalog search (debounced fetch to /items/search per D-10) + custom escape hatch + FormField detail fields
├── lib/components/SiteShell.svelte       # MODIFIED (nav only): add Wantlist link in session?.authenticated block
├── lib/columns.ts                 # EXTEND: wantlistColumns ColumnDef[] + priority sortingFn (tierSort twin)
├── lib/components/StateBlock.svelte      # EXTEND: add 'no-wants' StateKind
└── lib/api.ts                     # EXTEND: WantlistRow + CatalogItem interfaces + fetchOwnWants/addWant/removeWant/searchCatalog wrappers
```

### Pattern 1: Session-derived owner, never the body (the IDOR boundary)
**What:** The owner is `caller(ctx)` (= `webauth.UserFromContext`), resolved from the session cookie.
The request body carries the want's fields and (for remove) the `id` ONLY — never an owner/discord id.
**When to use:** every handler.
**Example:**
```go
// Source: internal/backendsrv/webadmin/account.go (RevokeOwnCodeHandler), adapted
func RemoveOwnWantHandler(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost { http.Error(w, "method not allowed", 405); return }
        var req struct{ ID int64 `json:"id"` }
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID <= 0 {
            writeJSONError(w, http.StatusBadRequest, "invalid_input"); return
        }
        callerID := caller(r.Context())   // D-02: owner from session, NOT the body
        now := nowUnix()
        var removed bool
        err := withTx(r.Context(), db, func(tx *sql.Tx) error {
            var e error
            removed, e = store.RemoveOwnWantTx(r.Context(), tx, req.ID, callerID) // WHERE id=? AND discord_user_id=?
            if e != nil { return e }
            if removed {
                return AppendAuditTx(r.Context(), tx, "wantlist_remove", callerID,
                    map[string]any{"want_id": req.ID}, now)
            }
            return nil
        })
        if err != nil { /* map → 500 */ }
        writeJSON(w, map[string]any{"removed": removed})  // false = not yours / already gone (silent no-op)
    }
}
```

### Pattern 2: Owner-scoped store mutator (the SQL-level IDOR guard)
**What:** Every owner-scoped write/read carries `WHERE discord_user_id = ?`. A cross-owner remove is
`RowsAffected = 0` → `(false, nil)` — a silent no-op that never leaks the row's existence.
**Example (the exact `RevokeOwnCodeTx` shape — store/linking.go:198):**
```go
func RemoveOwnWantTx(ctx context.Context, tx *sql.Tx, wantID int64, discordID string) (bool, error) {
    // Soft-delete so Phase 20's alert_log.wantlist_item_id FK never dangles (Pitfall 4).
    res, err := tx.ExecContext(ctx,
        `UPDATE wantlist_item SET active = 0 WHERE id = ? AND discord_user_id = ? AND active = 1`,
        wantID, discordID)
    if err != nil { return false, fmt.Errorf("remove own want (id=%d): %w", wantID, err) }
    n, _ := res.RowsAffected()
    return n > 0, nil
}
```

### Pattern 3: Add with server-side validation + dedupe
**What:** Re-validate `reason ∈ {buy,quest}`, `priority ∈ {low,med,high}`, `len(note) ≤ 280` server-side
(NEVER trust the form `<select>` — the `charmeta.go` `validCharMeta` precedent). Enforce the D-05 "no
exact duplicate" rule. **Recommendation:** do the dedupe in SQL via a partial unique index for catalog
wants and an in-handler check for custom wants (see Pitfall 1).
**Example (validation idiom — charmeta.go:155):**
```go
func validWant(req addWantReq) bool {
    if req.Reason != "buy" && req.Reason != "quest" { return false }
    switch req.Priority { case "low","med","high": default: return false }
    if utf8.RuneCountInString(req.Note) > 280 { return false }    // D-01: 280 CHARS, count runes not bytes (Pitfall 2)
    if req.ItemID == nil && strings.TrimSpace(req.ItemName) == "" { return false } // custom needs a label
    return true
}
```

### Pattern 4: Frontend list→confirm→server-truth-reload lifecycle
**What:** `onMount` → `StateBlock` phases (loading/error/ready) → working component. Add/remove always
re-fetch from the server (never optimistic-mutate the local array) so the grid stays authoritative.
A 401 bubbles to `AuthGate` via `authGuard`. This is `WatcherCodesPanel`'s exact lifecycle.
**Source:** web/src/lib/components/WatcherCodesPanel.svelte (`load` / `generate` / `doRevoke` / `route`).

### Pattern 5: One payload serves catalog-search AND in-bank-join
> **CATALOG-SEARCH HALF SUPERSEDED BY D-10 — see Addendum.** The in-bank-join half below is UNCHANGED.

**What:** `fetchView()` returns `ViewRow[]` (`{char, slot, item, id, count, wiki_url, wiki_summary, prices, ...}`).
- **Catalog search (ORIGINAL — now revised):** the body planned to map `ViewRow[]` and run `searchRows`
  client-side. **D-10 moves this server-side** (`GET /api/v1/items/search` over `pigparse_price`) because
  the full Blue catalog is bigger than the held-only `view` corpus. See Addendum Q3.
- **In-bank join (D-06) — UNCHANGED:** `group viewRows by id` → for the want's `item_id`, emit `↳ <char>: <count>`
  per holding row (reuse the `SearchResults` `↳`-line treatment + `COLLAPSE_THRESHOLD=5`).
- **Custom want:** `item_id` NULL ⇒ not in the corpus ⇒ "—". No lookup.
**Why this matters:** the planner should still NOT plan an in-bank read endpoint — that join is a
client-side derivation of the existing `view` payload. ONLY the catalog *search* gains a server endpoint (D-10).

### Anti-Patterns to Avoid
- **Per-character wantlist tabs / views:** CLAUDE.md LOCKED — one consolidated owner-scoped `DataGrid`,
  never per-character (the 200-tab-limit doctrine; here it would also be nonsensical — it's one person's list).
- **A new backend IN-BANK read endpoint:** the in-bank corpus is the `view` payload; the join is client-side
  (Pattern 5). *(The catalog-SEARCH endpoint IS now wanted — D-10. These are different endpoints.)*
- **Owner/discord_user_id in the request body:** the v2.1 D-02 IDOR boundary; owner is session-only.
- **`{@html}` on item names / custom labels / notes:** all are user/wiki-controlled; render via plain `{}`.
  The ONLY sanctioned `{@html}` sink stays `ItemTooltip`→`composeItemNote` (which escapes every value).
- **Adding a UNIQUE column via `ALTER`:** the 00005 landmine — SQLite cannot. Fresh `CREATE TABLE`s here
  avoid it, BUT see Pitfall 1 for the nullable-`item_id` partial-unique-index nuance.
- **`USER_ENTERED`-style implicit coercion / number-input traps:** the P15 frontend lesson (epoch-sec
  + number-input coercion bugs slipped past 165 green node tests). Browser-smoke the form before "verified".
- **Sourcing catalog search from `item_master`:** the D-10 trap — see Addendum Pitfall A1.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Owner-scoped atomic write + audit | A bespoke tx + manual audit INSERT | `withTx` + `AppendAuditTx` | Panic-safe deferred rollback + append-only audit already solved (audit.go). |
| IDOR-safe remove | A `DELETE WHERE id=?` then a separate owner check | `UPDATE ... WHERE id=? AND discord_user_id=?` (RowsAffected) | TOCTOU-free; silent no-op on cross-owner (linking.go:198). |
| Catalog item search | A new search service / FTS engine | A `LIKE`+`COLLATE NOCASE` SELECT over `pigparse_price` (D-10; readviews.go shape) | The full Blue catalog is already ingested daily; a guarded `LIKE` query is enough at ~thousands of rows. See Addendum. |
| Sortable/filterable list | A hand-rolled table | `DataGrid.svelte` + a `wantlistColumns` entry | The 5th instantiation; multi-sort + facet/text filters built in. |
| Confirm-before-destroy | A custom modal | `ConfirmDialog.svelte` | Focus-trap, Esc/backdrop dismiss, focus restore (15-UI-SPEC). |
| Session identity | Reading a header / cookie manually | `caller(ctx)` = `webauth.UserFromContext` | The session machinery + `RequireSession` gate already resolve it. |
| Migration test | Ad-hoc DDL asserts | `migrate_test.go` `columnSet`/`tableExists`/`indexExists` + `NewTestDB` | The established idiom; just add `TestMigrate_00006_*`. |

**Key insight:** Phase 19 is a *composition* phase. Almost every line has a named twin in P14/P16/P17.
The risk is NOT "how do I build X" — it's "did I copy the security shape exactly" (owner from session,
owner-scoped SQL, audit in the tx, plain-`{}` escaping). *The one genuinely net-new piece is the D-10
catalog-search endpoint — and even that clones the readapi GET-handler + readviews SELECT idioms.*

## Runtime State Inventory

> Phase 19 is **additive greenfield schema** (new tables only), not a rename/refactor/migration of
> existing data. No existing runtime state is renamed or re-keyed. For completeness:

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — `wantlist_item`/`alert_log` are brand-new empty tables. No existing rows reference a wantlist. (D-10's catalog source `pigparse_price` is READ-only here — never written by P19.) | None. |
| Live service config | None — no n8n/Datadog/external config touches the wantlist. | None. |
| OS-registered state | None — no new scheduler/task/unit; the existing `squirebot-server` systemd unit serves the new routes after the drop-binary-and-restart deploy. | Deploy = drop binary + restart (existing). Migration runs on startup via `RunMigrations` (idempotent). |
| Secrets/env vars | None — no new secret. (Phase 20 adds `DISCORD_BOT_TOKEN`, not this phase.) | None. |
| Build artifacts | The static SvelteKit `web/` build gains a `/wantlist` route; the Go binary embeds `00006_wantlist.sql` via `//go:embed *.sql`. | Rebuild web + binary (normal); the embed picks up the new `.sql` automatically. |

**Verified:** Searched migrations + store + cmd for any pre-existing wantlist reference — none. This is net-new.

## Common Pitfalls

### Pitfall 1: The nullable-`item_id` uniqueness constraint (D-05)
**What goes wrong:** A naïve `UNIQUE(discord_user_id, item_id, reason)` does NOT dedupe custom wants —
in SQLite, **`NULL` values are distinct in a UNIQUE index**, so two identical custom wants (both
`item_id` NULL, same label, same reason) both insert. Conversely, you can't put the custom-label in the
same unique tuple because catalog wants have no label-uniqueness need.
**Why it happens:** SQL `NULL != NULL`; the catalog and custom dedupe keys are different (`item_id` vs
`item_name`).
**How to avoid (recommended split):**
- Catalog wants: a **partial** unique index — `CREATE UNIQUE INDEX wantlist_catalog_uidx ON
  wantlist_item(discord_user_id, item_id, reason) WHERE item_id IS NOT NULL AND active = 1;` (the partial
  index is the same tool 00005 used for `owner_discord_user_id_uidx`).
- Custom wants: a second partial index keyed on the label — `CREATE UNIQUE INDEX wantlist_custom_uidx ON
  wantlist_item(discord_user_id, item_name, reason) WHERE item_id IS NULL AND active = 1;` — OR an
  in-handler `SELECT` pre-check. Either satisfies the user-visible rule. **The `active = 1` clause in the
  partial index is essential** so a removed-then-re-added want doesn't collide with its own tombstone.
**Warning signs:** duplicate custom rows appearing; or a re-add after remove failing with a constraint error.
[VERIFIED: SQLite NULL-in-unique-index semantics; partial-index precedent in 00005]

### Pitfall 2: Note length is 280 CHARACTERS, not bytes (D-01)
**What goes wrong:** A Go `len(note)` counts BYTES; an EQ item note with any multibyte char would let a
visually-≤280 note be rejected, or a byte-280 cap truncate mid-rune. The frontend `maxlength=280`
counts UTF-16 code units; Go must count runes to match.
**How to avoid:** server-side `utf8.RuneCountInString(note) > 280` (Pattern 3). Mirror the frontend
counter semantics. (Low practical risk for ASCII item notes, but the contract says "≤ 280 chars".)
**Warning signs:** counter says 278 but the server 400s.

### Pitfall 3: Identity is `web_user.discord_user_id`, NOT `owner` (the watcher concept)
**What goes wrong:** `account.go` resolves an `owner` (the watcher-token entity) because codes belong to
watchers. A wantlist belongs to a **person**, and the DM target (Phase 20) is their `discord_user_id`.
Keying the wantlist on `owner.id` would (a) break for a logged-in member who has never linked a watcher,
and (b) misalign with Phase 20's `notify` target.
**Why it happens:** copy-pasting `account.go`'s `ResolveOrCreateOwnerByDiscordTx` wholesale.
**How to avoid:** `wantlist_item.discord_user_id TEXT NOT NULL REFERENCES web_user(discord_user_id) ON
DELETE CASCADE` — and resolve it directly from `caller(ctx)`. Do NOT call the owner-resolve algorithm;
there is no owner involved. [CITED: ARCHITECTURE-v2.2.md §State/identity — "tied to the Discord identity,
NOT to owner ... works for any logged-in member even before they have linked a watcher"]

### Pitfall 4: Hard-DELETE dangles Phase 20's `alert_log` FK
**What goes wrong:** Phase 20 adds `alert_log.wantlist_item_id REFERENCES wantlist_item(id)`. If P19 hard-
DELETEs a want, P20's historical alert rows lose their referent (or `ON DELETE CASCADE` erases alert
history). ARCHITECTURE-v2.2 explicitly chose soft-delete (`active`) for exactly this reason.
**How to avoid:** soft-delete in P19 (`active INTEGER NOT NULL DEFAULT 1`; remove = `SET active = 0`).
Every read filters `WHERE active = 1`. This is a forward-looking decision the planner should lock now
even though `alert_log` is just a stub this phase.
**Warning signs:** Phase 20 review flags an FK that can dangle.

### Pitfall 5: Frontend "green tests ≠ works" (the P15 lesson, carried in MEMORY)
**What goes wrong:** `web/` vitest is node-only (no jsdom, no `@testing-library/svelte` per the toolchain-
install rule). P15 shipped 165 green tests with TWO crashing browser BLOCKERs (number-input coercion, an
epoch-sec date bug). The add-form (a `<textarea>` counter, two `<select>`s, the disabled-until-valid Add
button), the debounced catalog-search fetch (D-10), and the in-bank holder-line rendering are exactly the
kind of DOM-coupled logic node tests are blind to.
**How to avoid:** keep pure logic (search-result mapping, dedupe-check, holder grouping, note-counter math,
debounce timing) in DOM-free module blocks (the `WatcherCodesPanel` `formatLastSeen` precedent) so it IS
node-testable; AND require a manual browser-smoke of the `/wantlist` page before calling it verified. The
planner should add a manual-smoke acceptance step. [CITED: MEMORY web-tests-node-only-blind-to-dom]

### Pitfall 6: Planner clobbering uncommitted docs (process, carried in MEMORY)
**What goes wrong:** the gsd-planner has previously committed despite `commit_docs:false` and overwritten
an uncommitted doc during "recovery."
**How to avoid:** `commit_docs` is `false` in config — the planner must NOT commit. Git-diff planning docs
after the planner/executor runs. [CITED: MEMORY feedback_planner_clobbers_uncommitted_docs]

## Code Examples

### Migration skeleton (`00006_wantlist.sql`) — forward-only, fresh CREATEs
```sql
-- Source: pattern from internal/backendsrv/migrations/00005_self_service_linking.sql
-- +goose Up  (forward-only; 00001..00005 shipped and NOT edited)

CREATE TABLE wantlist_item (
  id              INTEGER PRIMARY KEY,
  discord_user_id TEXT NOT NULL REFERENCES web_user(discord_user_id) ON DELETE CASCADE,
  item_id         INTEGER,                 -- canonical EQ item id (joins pigparse_price/item_master); NULL ⇒ custom want (D-04)
  item_name       TEXT NOT NULL,           -- snapshot at add-time: catalog name OR the custom label (display + future chat-match)
  reason          TEXT NOT NULL,           -- 'buy' | 'quest'  (D-01, server-validated)
  priority        TEXT NOT NULL DEFAULT 'med',  -- 'low' | 'med' | 'high'  (D-01/D-02 enum, NOT numeric)
  note            TEXT,                    -- optional, ≤280 chars (D-01; length enforced in the handler)
  active          INTEGER NOT NULL DEFAULT 1,   -- soft-delete so Phase 20 alert_log FK survives (Pitfall 4)
  created_at      INTEGER NOT NULL          -- unix epoch secs (nowUnix(); web-side convention)
);
CREATE INDEX        wantlist_user_idx        ON wantlist_item(discord_user_id);
CREATE INDEX        wantlist_item_id_idx     ON wantlist_item(item_id);
-- D-05 dedupe (Pitfall 1): partial unique indexes, scoped to active rows.
CREATE UNIQUE INDEX wantlist_catalog_uidx    ON wantlist_item(discord_user_id, item_id, reason)   WHERE item_id IS NOT NULL AND active = 1;
CREATE UNIQUE INDEX wantlist_custom_uidx     ON wantlist_item(discord_user_id, item_name, reason) WHERE item_id IS NULL     AND active = 1;

-- alert_log: created here, CONSUMED by Phase 20 (dedup/cooldown + delivery audit). Stub shape only.
CREATE TABLE alert_log (
  id               INTEGER PRIMARY KEY,
  wantlist_item_id INTEGER NOT NULL REFERENCES wantlist_item(id) ON DELETE CASCADE,
  discord_user_id  TEXT NOT NULL,
  source           TEXT NOT NULL,          -- 'ec_auction' | 'wts' | 'raid_target'  (Phase 20+)
  item_id          INTEGER,
  detail           TEXT,                   -- small JSON (price/channel snippet) — never raw bodies
  sent_at          INTEGER NOT NULL,       -- unix epoch secs
  send_status      TEXT NOT NULL           -- 'sent' | 'dm_blocked' | 'error'
);
CREATE INDEX alert_log_dedup_idx ON alert_log(wantlist_item_id, source, item_id, sent_at);

-- D-10 (optional but recommended): accelerate catalog-search prefix ranking.
CREATE INDEX pigparse_name_idx ON pigparse_price(name COLLATE NOCASE);

-- +goose Down
-- Forward-only in practice (mirrors 00004/00005): explicit no-op.
SELECT 1;
```

### Route registration (twin of the `/account` block)
```go
// Source: cmd/squirebot-server/main.go:308-314 (the /account/codes block)
// Wantlist — LOGIN-ONLY: RequireSession, NEVER RequireOfficer. Every signed-in member
// manages their OWN wantlist (owner from session, D-02).
mux.Handle("GET  /api/v1/wantlist",        webauth.RequireSession(db, webadmin.ListOwnWantsHandler(db)))
mux.Handle("POST /api/v1/wantlist",        webauth.RequireSession(db, webadmin.AddWantHandler(db)))
mux.Handle("POST /api/v1/wantlist/remove", webauth.RequireSession(db, webadmin.RemoveOwnWantHandler(db)))
// D-10: full-catalog item search (session-gated like the view endpoints). See Addendum.
mux.Handle("GET  /api/v1/items/search",    webauth.RequireSession(db, readapi.NewItemSearch(st)))
```

### Frontend typed wrappers (twin of the self-service codes block in api.ts)
```ts
// Source: web/src/lib/api.ts:534-564 (the /account/codes wrappers)
export interface WantlistRow {
  id: number;
  item_id: number | null;     // null ⇒ custom want (D-04, D-07)
  item_name: string;          // catalog name OR custom label
  reason: 'buy' | 'quest';
  priority: 'low' | 'med' | 'high';
  note: string | null;
  created_at: number;
}
export interface CatalogItem { item_id: number; name: string; current_avg?: number; } // D-10 search result
export function fetchOwnWants(f: typeof fetch = fetch): Promise<WantlistRow[]> {
  return getJSON<WantlistRow[]>('/api/v1/wantlist', f);
}
export function searchCatalog(q: string, f: typeof fetch = fetch): Promise<CatalogItem[]> {
  return getJSON<CatalogItem[]>('/api/v1/items/search?q=' + encodeURIComponent(q), f); // D-10
}
export function addWant(body: {
  item_id: number | null; item_name: string;
  reason: 'buy' | 'quest'; priority: 'low' | 'med' | 'high'; note?: string;
}, f: typeof fetch = fetch): Promise<WantlistRow> {
  return postJSON<WantlistRow>('/api/v1/wantlist', body, f);   // owner is session-derived; body has NO owner
}
export function removeWant(id: number, f: typeof fetch = fetch): Promise<{ removed: boolean }> {
  return postJSON<{ removed: boolean }>('/api/v1/wantlist/remove', { id }, f);
}
```

### In-bank holder join (client-side, over the existing `view` payload) — UNCHANGED by D-10
```ts
// Group the already-fetched ViewRow[] by item_id, then look up a want's item_id.
// Reuses the SearchResults "↳ <Char>: <count>" treatment + COLLAPSE_THRESHOLD=5.
import { COLLAPSE_THRESHOLD } from '$lib/search/searchIndex';
function holdersFor(itemId: number | null, viewRows: ViewRow[]): { char: string; count: number }[] {
  if (itemId === null) return [];                                  // custom want → "—" (D-07)
  return viewRows.filter((r) => r.id === itemId)
                 .map((r) => ({ char: r.char, count: r.count }))
                 .sort((a, b) => a.char.localeCompare(b.char));     // SearchResults order
}
// ≥1 holder → "In bank" (--status-ok) + holder lines; 0 → "Not in bank" (--status-missing).
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Per-character view tabs (Sheet era) | One consolidated filterable `DataGrid` | v2.0 (Off-Google) | Wantlist is the 5th instantiation; never per-character. |
| Sheet-coupled search (Apps Script `runSearch`) | Client-side `searchIndex.ts` over the `view` payload | Phase 14 | The in-bank holder grouping reuses this; **catalog search is now a server endpoint (D-10).** |
| `owner`-keyed identity (watcher token) | `web_user.discord_user_id`-keyed identity for personal data | v2.1 (Discord login) | Wantlist keys on the person, not the watcher (Pitfall 3). |
| Held-only catalog (`view` / `item_master`) | Full Blue catalog (`pigparse_price`) via server search | v2.2 / D-10 (2026-06-03) | Any real item is pinnable + alert-capable in Phase 21+ (Addendum). |

**Deprecated/outdated:**
- A backend IN-BANK read endpoint: never existed and is not needed (the in-bank join is client-side).
- A client-side catalog search over the `view` payload: superseded by D-10's server endpoint.
- `goose` Down migrations: the project treats migrations forward-only; Down is an explicit `SELECT 1;`.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Soft-delete (`active`) is the right call for P19's remove, to protect Phase 20's `alert_log` FK. | Stack / Pitfall 4 | LOW — if the planner prefers hard-DELETE, Phase 20 must add `ON DELETE CASCADE`/SET NULL handling instead. Either is viable; soft-delete is the lower-regret default per ARCHITECTURE-v2.2's own sketch. |
| A2 | `priority` stored as a TEXT enum (`'low'/'med'/'high'`), not the `INTEGER` in ARCHITECTURE-v2.2's sketch. | Migration skeleton | LOW — D-02 says "simple enum, NOT a numeric rank," so TEXT matches the locked decision better than the sketch's `priority INTEGER DEFAULT 0`. Frontend sorts via a `sortingFn` mapping high=3/med=2/low=1 regardless. Planner may pick INTEGER; the user-visible behavior is identical. |
| A3 | ~~The `view` payload is an acceptable client-side corpus for BOTH catalog-search and the in-bank join.~~ **REVISED BY D-10:** the `view` payload remains the corpus for the in-bank join ONLY; catalog search now uses a server endpoint over `pigparse_price`. | Pattern 5 / Addendum | RESOLVED — the in-bank half stands (verified); the catalog half is replaced by the Addendum. |

## Open Questions (RESOLVED)

1. ~~**Should catalog search include items in `item_master` that NO ONE currently holds?**~~
   **RESOLVED BY D-10:** catalog search now spans the FULL Blue item set (`pigparse_price`), not just
   held items and not `item_master`. Items nobody holds ARE searchable and pinnable (and correctly show
   "Not in guild"). See the Addendum. The custom-want path (D-04) remains the escape hatch for items not
   even in PigParse.

2. **`alert_log` minimal stub vs. full Phase-20 shape now?**
   **RESOLVED — full shape now, zero rows.** Create the full table shape this phase (per the skeleton
   above, matching ARCHITECTURE-v2.2 Pattern 3) so Phase 20 adds zero migration churn — but write NO rows
   and add NO read path this phase. The migrate test asserts only that the table exists and is empty.
   (Pinned in 19-01-PLAN Task 1: `alert_log` built to full shape, `SELECT COUNT(*) == 0` asserted.)

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Backend build + tests | ✓ (existing project) | 1.24 | — |
| `modernc.org/sqlite` | New tables | ✓ | v1.51.0 | — |
| `pressly/goose/v3` | `00006` migration | ✓ | v3.27.1 | — |
| Node/npm + SvelteKit | `web/` build + vitest | ✓ (existing) | — | — |
| `@tanstack/table-core` | DataGrid | ✓ | 8.21.3 | — |
| `pigparse_price` (populated) | D-10 catalog search | ✓ (live; daily job since P12) | — | empty-result graceful state (Addendum Pitfall A2) |

**Missing dependencies:** None — Phase 19 introduces no new tool, runtime, or library. All probes
resolved against the committed `go.mod` / `package.json` / live route table. The D-10 catalog data
(`pigparse_price`) is already populated daily in production.

## Security Domain

> `security_enforcement: true`, `security_asvs_level: 1`, `block_on: high`. Included.

### Applicable ASVS Categories (L1)

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | `webauth.RequireSession` gate on all 3 wantlist routes AND the D-10 `/items/search` route (no anon access). [VERIFIED: main.go RequireSession precedent] |
| V3 Session Management | yes | Existing opaque hashed `web_session` cookie machinery; the handler reads identity via `webauth.UserFromContext`, never trusts the body. |
| V4 Access Control (IDOR) | **yes — primary** | Owner-scoped SQL (`WHERE discord_user_id = ?`) on list + remove; cross-owner remove is a silent `RowsAffected=0` no-op (linking.go:198 twin). The wantlist is strictly private to its owner this phase. (The catalog search is non-owner-scoped — it's a public-within-the-guild reference list — which is correct: the catalog is not personal data.) |
| V5 Input Validation | yes | Server-side re-validate `reason`/`priority` enums + note rune-length (charmeta.go `validCharMeta` twin); parameterized `?` placeholders ONLY (V5/Tampering), including the D-10 search `q` (bound `LIKE` term with `ESCAPE`, never concatenated — Addendum Pitfall A5). |
| V6 Cryptography | no | No new secrets/tokens minted (unlike `account.go`'s code mint). `alert_log` writes nothing this phase. |
| V7 Error/Logging | yes | slog logs op + ids ONLY, never note/label content or row data; the D-10 search handler logs op + result-count + `len(q)`, NEVER the query string `q`; audit `detail` carries `want_id`/`item_id` only, never the note text. |
| V12/V14 (XSS via stored data) | yes | item names, custom labels, notes, AND catalog-search result names are user/wiki/PigParse-controlled → rendered via plain Svelte `{}` auto-escape; the ONLY `{@html}` sink stays `ItemTooltip`→`composeItemNote` (which escapes every value). [CITED: 19-UI-SPEC § Accessibility — XSS trust boundary] |

### Known Threat Patterns for {Go net/http + SQLite + SvelteKit}

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| IDOR — reading/removing another guildie's want by id | Elevation of Privilege | Owner-scoped `WHERE discord_user_id = ?`; never accept an owner in the body (D-02). |
| Stored XSS via item name / custom label / note / catalog result | Tampering | Plain `{}` auto-escape; inert text; no raw-HTML directive on user/PigParse data. |
| SQL injection (incl. the D-10 search `q`) | Tampering | Parameterized `?` placeholders ONLY; the `LIKE` term is built in Go and bound with `ESCAPE '\'`, never interpolated (readviews.go/linking.go discipline). |
| Note/label as a log-injection or PII leak vector | Information Disclosure | Audit `detail` + slog carry ids/counts only, never free-text content or the search query (audit.go V7 note). |
| Forged owner / privilege via request body | Spoofing | Owner is `caller(ctx)` from the session; the body has no owner field (the v2.1 D-02 boundary). |
| Duplicate-want / constraint bypass via NULL item_id | Tampering | Two partial unique indexes (catalog + custom), both `WHERE active = 1` (Pitfall 1). |
| Unbounded `LIKE` DoS via tiny/empty `q` | Denial of Service | `len(q) >= 2` server guard + `LIMIT` + frontend debounce (Addendum Pitfall A4). |

**Security verdict:** Phase 19's security posture is *inherited* — it is the proven P17 `account.go`
shape (login-only, session-owner, IDOR-scoped, audited, parameterized, plain-`{}` rendered), extended by
the D-10 read endpoint which inherits the readapi GET-handler discipline (session-gated, parameterized,
V7-quiet). The three phase-specific security obligations the planner must verify in tasks: (1) the
owner-scoped remove proven by a cross-owner no-op test (the `account_test.go` IDOR test twin), (2) the
stored-XSS boundary proven by a manual browser-smoke (node vitest is blind to it — Pitfall 5), and (3)
the D-10 search `q` proven bound-not-concatenated (a `q` containing `%`/`_`/`'` returns literal matches,
not a wildcard blowup or an injection — Addendum Pitfall A5).

## Sources

### Primary (HIGH confidence — read directly this session)
- `internal/backendsrv/webadmin/account.go` + `charmeta.go` + `audit.go` + `officers.go` — the handler
  shape, `caller`/`withTx`/`AppendAuditTx`/`writeJSON`/`nowUnix` helpers, server-side validation idiom.
- `internal/backendsrv/store/linking.go` + `readviews.go` + `itemids.go` — owner-scoped `*Tx` mutators,
  the `view`/`bank` join (`InventoryJoin`), the `item_master`-from-inventory catalog truth.
- `internal/backendsrv/compute/view.go` + `bank.go` — the consolidated-inventory builders (the in-bank corpus).
- `internal/backendsrv/migrations/00005_self_service_linking.sql` + `embed.go` + `migrate_test.go` — the
  forward-only/partial-unique-index conventions + the migration test idiom.
- `internal/backendsrv/migrations/00001_init.sql` — `item_master`/`quest_items`/`pigparse_price` schema.
- `cmd/squirebot-server/main.go` (+ `main_test.go`) — route registration + the anon→401/member→admitted gate tests.
- `web/src/lib/search/searchIndex.ts` — the client-side fuzzy search + "did you mean?" engine.
- `web/src/lib/api.ts` — `getJSON`/`postJSON` cores, typed errors, the `/account/codes` wrapper twins.
- `web/src/lib/columns.ts` — the `DataGrid` ColumnDef pattern + `tierSort` custom sortingFn.
- `web/src/routes/account/+page.svelte` + `web/src/lib/components/WatcherCodesPanel.svelte` — the page
  shell + the load→confirm→server-truth-reload lifecycle to clone.
- `go.mod` / `package.json` — version verification (goose v3.27.1, modernc v1.51.0, table-core 8.21.3).
- *(D-10 pass — see the Addendum's own Sources block for the PigParse-ingest files read this session.)*

### Secondary (HIGH — milestone research, cross-checked against code this session)
- `.planning/research/ARCHITECTURE-v2.2.md` — the `wantlist_item`/`alert_log` schema sketch, the
  identity-is-discord_user_id decision, soft-delete rationale, "one match seam" spine, the Phase 21
  `wantmatch.ForItem` + `ec_auction_match`-reads-`pigparse_price` flow (D-10 join-key proof).
- `.planning/research/SUMMARY-v2.2.md` — Phase 19 scope, the "standard patterns, skip a research-phase"
  classification (direct `account.go` twin).
- `.planning/phases/19-wantlist-crud/19-CONTEXT.md` (incl. D-10) + `19-UI-SPEC.md` — locked decisions + UI contract.
- `.planning/REQUIREMENTS.md` / `STATE.md` — WANT-01/02 scope + traceability.
- MEMORY: web-tests-node-only-blind-to-dom; feedback_planner_clobbers_uncommitted_docs; project_consolidated_views.

### Tertiary (LOW — none load-bearing)
- (No unverified web claims. All findings are codebase-verified or cited to milestone research.)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every "library" is an existing, version-pinned dependency; zero new deps.
- Architecture: HIGH — every pattern has a named, read-this-session twin (account.go / linking.go /
  WatcherCodesPanel / columns.ts); the catalog-corpus + in-bank-corpus question is resolved against the
  actual `pigparse_price` ingest path (D-10 Addendum) and `searchIndex.ts`.
- Pitfalls: HIGH — the nullable-UNIQUE nuance is SQLite-semantics + the 00005 partial-index precedent;
  the identity-keying and soft-delete pitfalls are cited to ARCHITECTURE-v2.2; the green-tests-≠-works
  pitfall is the project's own logged P15 incident; the D-10 pitfalls are codebase-verified.

**Research date:** 2026-06-03
**Valid until:** 2026-07-03 (stable — internal brownfield; the only drift risk is unrelated codebase
churn. Re-verify route table + `searchIndex.ts` + the `pigparse_price` schema if Phases 17/18 work landed after this date.)

---

## Addendum: Full Item-Catalog Source (D-10) — Server-Side Search

**Appended:** 2026-06-03 (targeted follow-up after CONTEXT.md **D-10** revised the catalog source).
**Supersedes for the add-item *search* only:** the body's Pattern 5 / Open-Question-1 / Assumption-A3
conclusion that the add-item catalog is the client-side `fetchView()` payload. **D-10 changes only the
catalog *search corpus*.** Everything else in the body stands unchanged — in particular **the in-bank
"who holds it + count" join (D-06) remains a client-side derivation of `fetchView()`** (Pattern 5's
in-bank half, the `holdersFor()` example, and the in-bank security/XSS notes are all still correct). Read
this addendum as a *surgical override* of the catalog-search tier, not a rewrite.

### Why D-10 forces a new server endpoint (the one-line answer)

The shipped client-side search corpus (`view` rows) is *only currently-held guild items*. D-10 requires
the searchable catalog to be **the full Blue item set** so any real item pins a stable `item_id` and is
alert-capable in Phase 21+. That full set already lands daily in the backend as `pigparse_price`
(thousands of rows — see Q2) and is **not shipped to the browser at all today**. Therefore the add-item
search MUST move server-side: a new `GET /api/v1/items/search?q=` over `pigparse_price`. This is net-new
infra, but it is the *minimum* net-new infra — one read handler + one store SELECT, no new table, no new
job, no new dependency.

### Q1 — Where the daily PigParse getall data lands (exact table + columns)

**Table: `pigparse_price`** (created empty in `00001_init.sql:60`, columns added in
`00003_enrich_columns.sql:9-16`). [VERIFIED: migrations read this session]

Full current column set:

| Column | Type | Source | Notes for search |
|--------|------|--------|------------------|
| `item_id` | INTEGER **PRIMARY KEY** | PigParse `i` | **The canonical EQ item id.** PK ⇒ exactly one row per item (no fan-out). The pin key for a catalog want. |
| `name` | TEXT | PigParse `n` (trimmed) | **Present and reliable for ALL rows** — `n` is a REQUIRED_KEY in the parser; a row with an empty/missing `n` is *rejected* (`coerceRow`, `pigparse.go:132-135`). So every `pigparse_price` row HAS a non-empty name. This is the searchable label. |
| `current_avg` | REAL | alias of `a30` | optional secondary display (price). |
| `blue_volume` | INTEGER | alias of `t30` | optional. |
| `last_seen` | TEXT | PigParse `l` (ISO 8601) | freshness of the *price*, not the catalog row. |
| `direction` | TEXT | `strconv.Itoa(t)` | always `"0"` after the job's WTS filter (D-9). |
| `t30,a30,t60,a60,t6m,a6m,ty,ay` | INT/REAL | price history | not needed for search. |
| `last_refreshed` | TEXT | job-run timestamp (RFC3339) | catalog freshness signal (see Pitfall A3). |

**Does it contain `item_id` + name for ALL Blue items (not just guild-held)?** **YES.** [VERIFIED:
`jobs/pigparse.go` + `store/enrich.go`] The daily job (`RunPigparse`) fetches `GET /api/item/getall/1`
(server=1=Blue), parses every row, keeps the WTS (`t==0`) row per item, and upserts each into
`pigparse_price` keyed on `item_id`. This is the **entire Blue market catalog**, completely independent of
guild inventory — exactly the property D-10 needs. Contrast `item_master`, which is populated only from
`DistinctInventoryItemIDs()` (guild-seen items), confirmed in `store/itemids.go`.

**Is `name` present, or only id+price?** Name is **always present** (it is a parser-required field; see
the table). No fallback to `item_master`/wiki is needed for the search label. (`item_master.name` exists
too but covers only the guild-seen subset, so it is the *wrong* source for full-catalog search.)

**Is it refreshed daily / how stale?** Refreshed once per daily scheduler window via `RunPigparse`, with a
304-ETag short-circuit (`jobs/pigparse.go:84-89`) — on a `304` the existing rows are left intact (never
wiped). Upsert is `ON CONFLICT(item_id) DO UPDATE` (`enrich.go:109-118`), so the catalog only *grows or
refreshes* — a row, once seen, is **never deleted** even if it drops off a later getall. Practical
staleness for *catalog membership* (the search use-case): a newly-tradeable item appears within ~1 day of
first showing up in PigParse; an item already in the table never disappears. (Price columns can be stale,
but search doesn't depend on them.)

### Q2 — Catalog size (justifying server-side)

**~7,000+ rows.** [VERIFIED: parser header comment — "7,240 rows on 2026-05-09" in `enrich/pigparse.go:6`,
captured from the live getall.] That 7,240 is the *pre-WTS-filter* row count (the getall returns up to ~2
rows per item: t=0 WTS + t=1 WTB); after the job's `t==0` filter the distinct-item count is on the order
of **~3,500–7,000 rows** (one per item with a sell-side entry). *(The committed
`testdata/pigparse-getall-1.json` fixture is currently 0 bytes — a git-checkout artifact — so the live
count is taken from the verified parser comment, not the empty fixture.)*

**Conclusion:** thousands of rows, each `{item_id, name, +price}`, is **too large to ship wholesale to
every page-load** (it would bloat the `/wantlist` payload by ~hundreds of KB of items the user will never
type, and unlike `view` it is NOT already being fetched for another purpose). This **justifies a
server-side `q=`-filtered search endpoint** that returns only the top-N matches. [CONFIDENCE: MEDIUM-HIGH —
exact post-filter row count is inferred from the pre-filter 7,240 + the documented dual-row structure; the
order of magnitude (thousands) is certain.]

### Q3 — Server-side search endpoint design (mirroring the readapi pattern)

There is **no existing item-search store method or endpoint** — `pigparse_price` is only ever *written*
(upsert) and *LEFT-JOINed* inside `InventoryJoin`. [VERIFIED: grep of `store/` — only
`UpsertPigparsePrices*` + the join read it.] So this is genuinely net-new, but it slots cleanly into the
established read seam.

**Route (mirror `readapi/views.go` registration in `main.go:266-269`):**
```go
// Search the full Blue item catalog (pigparse_price) by name or id. Session-gated
// like the view endpoints (members-only site; see auth note below).
mux.Handle("GET /api/v1/items/search", webauth.RequireSession(db, readapi.NewItemSearch(st)))
```

**Handler shape (a `readapi` GET handler, `ViewsHandler` twin):** GET-only (405 otherwise), reads
`q := r.URL.Query().Get("q")`, trims it, and **short-circuits to an empty array for `len(q) < 2`** (mirror
the body's 999.28 empty-query guard — never run an unbounded `LIKE` over thousands of rows). Calls one new
store method, JSON-encodes the slice (nil→`[]` coercion, exactly as `views.go:87`). V7 logging: op +
result-count + `len(q)` ONLY — **never the query string** `q` itself (it's user input; matches the readapi
"never a query param in logs" rule, `views.go` header).

**Store query (new `store/itemsearch.go`, a plain `(*Store)` read method — `readviews.go` shape):**
```go
// SearchCatalog returns up to `limit` catalog items whose name or id matches q,
// case-insensitively, ranked prefix-first. q is bound through ? placeholders ONLY
// (V5/Tampering) — NEVER concatenated. The LIKE wildcards are built in Go and bound
// as values, with ESCAPE so a user-typed % or _ is a literal, not a wildcard.
func (s *Store) SearchCatalog(ctx context.Context, q string, limit int) ([]CatalogItem, error) {
    like   := "%" + escapeLike(q) + "%"   // escapeLike escapes %, _, backslash ; bound, never concatenated
    prefix := escapeLike(q) + "%"
    rows, err := s.db.QueryContext(ctx,
        "SELECT item_id, name, current_avg "+
        "FROM pigparse_price "+
        "WHERE name LIKE ? ESCAPE '\\' COLLATE NOCASE "+
        "   OR CAST(item_id AS TEXT) = ? "+
        "ORDER BY (name LIKE ? ESCAPE '\\' COLLATE NOCASE) DESC, "+ // prefix hits first
        "         length(name), name COLLATE NOCASE "+
        "LIMIT ?",
        like, q, prefix, limit)
    // ... rows.Next()/Scan loop -> rows.Err() (the readviews.go idiom) ...
}
```
- **`COLLATE NOCASE`** is the repo's established case-insensitive idiom (`coin.go:51`, `admins.go:158`,
  `linking.go:98`). [VERIFIED] This is the milestone-research "pg_trgm -> LIKE/FTS5" call landing on
  **LIKE + COLLATE NOCASE** — correct for thousands of rows; no FTS5 table needed (revisit FTS5 only if
  the catalog grows an order of magnitude or substring latency is measured as a problem).
- **Numeric search by id:** `CAST(item_id AS TEXT) = ?` lets a guildie paste a raw item id (search is "by
  name + ID" per D-03).
- **Response shape:** `{ item_id: number; name: string; current_avg?: number }` per row. `current_avg`
  (price) is a nice-to-have the form can show; `item_id` + `name` are the load-bearing fields (the pin +
  the snapshot label).
- **Result limit/ranking:** `LIMIT` ~25–50; rank exact/prefix matches above mid-string matches (the
  `ORDER BY (name LIKE prefix) DESC, length(name)` clause). This gives "did you mean?"-grade ordering
  server-side.
- **Index:** add `CREATE INDEX pigparse_name_idx ON pigparse_price(name COLLATE NOCASE);` (see Pitfall A4)
  — a leading-wildcard `LIKE` can't use it for the substring scan, but it accelerates the prefix-rank
  expression and any future prefix-only mode, and is cheap insurance. The id-equality path already uses the PK.

**Auth — behind the Discord-login gate, NOT public.** [VERIFIED: every `/api/v1/views/*` and
`/api/v1/account/*` route is `webauth.RequireSession`, `main.go:265-314`; the public-read `readapi` era
ended at P15/AUTH-08, noted in the `views.go` header.] The whole site is members-only post-v2.1; the
search endpoint must match its siblings. Use `RequireSession`, never `RequireOfficer` (any signed-in
member searches), never public.

**Does the client fuzzy "did you mean?" still layer on top?** **Partially — the planner must decide the seam:**
- The server `LIKE`/COLLATE-NOCASE handles substring + case + prefix-ranking — that already covers most of
  what `searchIndex.ts` did over `view` rows.
- The client `searchIndex.ts` "did you mean?" (`didYouMean`) was a *typo/transposition* suggester over an
  in-memory name list. With a server endpoint you **cannot** run it over the full ~7k corpus client-side
  (the corpus isn't shipped). **Recommendation:** drop the client-side `didYouMean` for the catalog field
  and rely on the server's substring+prefix ranking (a too-short/zero-result query shows the friendly
  empty state + "try fewer letters"). If a typo-tolerant suggester is still wanted, run `didYouMean` over
  **the returned top-N result names only** (cheap, no full corpus) — but treat that as optional polish, not
  required by WANT-01. *(This is the one place D-10 genuinely reduces reuse of `searchIndex.ts`; flag it
  for the planner.)*

### Q4 — Wantlist row resolution (id-namespace check)

**Same `item_id` space across all three consumers — VERIFIED, no namespace mismatch.** The canonical EQ
item id is one global integer space; every table uses it as the join key:
- **Catalog source:** `pigparse_price.item_id` <- PigParse `i` (the EQ item id). [VERIFIED: `pigparse.go:43`
  `I int // EQ item ID`]
- **(a) In-bank `fetchView()` holder join (D-06):** `inventory_item.item_id` <- the watcher's parsed item
  id. `InventoryJoin` already does `LEFT JOIN pigparse_price pp ON pp.item_id = ii.item_id`
  (`readviews.go:135`) — i.e. the codebase **already joins inventory.item_id to pigparse_price.item_id
  today**, which is positive proof the two id spaces are identical. A want pinned from the catalog will
  therefore group correctly against `ViewRow.id` client-side (the body's `holdersFor()` is unchanged).
  [VERIFIED]
- **(b) Phase 21's EC matcher:** `wantmatch.ForItem(ctx, db, itemID)` and the `ec_auction_match` job read
  "the freshly-upserted `pigparse_price`" by `item_id`. [CITED: ARCHITECTURE-v2.2.md:163,238-241,404] Same
  id. A want pinned from `pigparse_price` is **exactly** what the Phase 21 matcher keys on — D-10's whole
  purpose ("alert-capable in Phase 21+") is satisfied by construction, because the want's `item_id` and the
  matcher's `item_id` come from the same table.

**Net:** pinning `wantlist_item.item_id` from `pigparse_price` makes the want simultaneously
in-bank-joinable (a) and alert-matchable (b) with zero id translation. **This is strictly better than the
body's `view`-sourced pin** (which could only pin items currently held). The custom-want path (D-04,
`item_id` NULL) remains the escape hatch for items not even in PigParse.

**One edge to note for the planner:** an item can be in `pigparse_price` (so it's searchable + pinnable)
but held by NOBODY -> its in-bank result is correctly "Not in bank" / no holder lines (the `holdersFor()`
filter returns `[]`). That is the intended D-10 behavior, not a bug: you *want* to wishlist things nobody
has yet.

### Q5 — Pitfalls specific to this addition

**Pitfall A1 — `item_master` is the wrong table (the trap D-10 exists to avoid).**
*What goes wrong:* reaching for `item_master` (it has `item_id` + `name` + a wiki summary and feels like
"the catalog").
*Why:* `item_master` is populated ONLY from `DistinctInventoryItemIDs()` (guild-seen items) — it is a
*subset* of `pigparse_price` and exactly the limited corpus D-10 rejects.
*Avoid:* search MUST hit `pigparse_price`. (You MAY LEFT-JOIN `item_master` to enrich a result with a wiki
summary/url for the tooltip, but the *membership* and the *name* come from `pigparse_price`.) [VERIFIED:
`itemids.go` population path]

**Pitfall A2 — Empty catalog on a fresh DB / before the first daily run.**
*What goes wrong:* `pigparse_price` is created empty (`00001`) and only filled by the first successful
`RunPigparse`. On a brand-new deploy, or if the daily job has never succeeded, search returns nothing and
the add-item box looks broken.
*Avoid:* the live prod DB has been running the daily job since Phase 12, so this is a non-issue in
production — but the *handler* and *frontend* must degrade gracefully (empty result -> friendly "no
matches / catalog still loading" state, never an error). Tests that exercise search must seed
`pigparse_price` (the `readviews_test.go:48` seed idiom). [VERIFIED: 00001 creates it empty]

**Pitfall A3 — Stale / never-deleted catalog rows + price staleness.**
*What goes wrong:* because the upsert never deletes, a long-gone item lingers in the catalog (fine for
wishlisting); and `current_avg`/`last_seen` can be days stale if the daily job has been failing (the job
advances `job_run` on every outcome, so a silent string of failures is possible).
*Avoid:* for *search* this is acceptable (membership is the point, not price freshness). If the form shows
a price, label it from `last_refreshed` or omit it when stale. Don't gate search on price freshness.
[VERIFIED: ON CONFLICT upsert never deletes; job_run advances on error — `jobs/pigparse.go:74-89`]

**Pitfall A4 — Substring `LIKE` with no usable index -> full scan of thousands of rows per keystroke.**
*What goes wrong:* a leading-wildcard `LIKE` cannot use any B-tree index, so each query scans the whole
table; fired per-keystroke from the form, that's thousands of row-comparisons × every keypress.
*Avoid:* (1) **debounce** the frontend search input (~200–300 ms) so you don't query per keystroke —
`searchIndex.ts` ran synchronously in-memory and never needed debounce; a network endpoint does. (2)
Enforce the **`len(q) >= 2` server guard** (no unbounded match). (3) Add
`pigparse_name_idx ON pigparse_price(name COLLATE NOCASE)` — it won't serve the mid-string substring scan
but it helps the prefix-rank ordering and a future prefix-mode, and is trivial. (4) `LIMIT 25–50`. At
~thousands of rows a single guarded substring scan is sub-millisecond in SQLite, so this is about
*per-keystroke amplification*, not a single query — debounce is the load-bearing mitigation. [CONFIDENCE:
HIGH — standard SQLite LIKE-index semantics; debounce/limit are the standard fixes.]

**Pitfall A5 — `LIKE` wildcard / case semantics gotchas.**
*What goes wrong:* (a) SQLite's default `LIKE` is *already* case-insensitive for ASCII but NOT for
non-ASCII; relying on bare `LIKE` for case-folding is subtly wrong for any non-ASCII item name. (b) A user
typing a `%` or `_` into the search box becomes a wildcard unless escaped.
*Avoid:* use explicit `COLLATE NOCASE` (the repo idiom) for deterministic case-folding, and **escape the
`%`/`_`/backslash in the bound term with an `ESCAPE` clause** (shown in the Q3 query). Bind the wildcards
as a value — never concatenate `q` into the SQL. [VERIFIED: COLLATE NOCASE is the repo idiom; ESCAPE is
standard SQLite]

**Pitfall A6 — The daily-refresh race (negligible, but stated).**
*What goes wrong:* a search firing during the daily upsert tx reads a partially-updated catalog.
*Avoid:* nothing needed. The upsert runs in one `BEGIN IMMEDIATE` tx (`jobs/pigparse.go:143` +
single-writer DSN); a concurrent reader sees either the pre-tx or post-commit state, never a torn one, and
either is a valid catalog for search. A row's `item_id`+`name` are stable across refreshes anyway (only
price columns churn). No locking or coordination required. [VERIFIED: single-writer immediate-tx DSN, per
the body + `enrich.go:122` comment]

### Addendum — Updated Assumptions

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A4 (new) | Post-WTS-filter `pigparse_price` row count is "thousands" (inferred from the verified pre-filter 7,240 + the documented dual-row t0/t1 structure). | Q2 | LOW — the order of magnitude (too big for client-side, fine for a guarded LIKE) holds across the plausible 3.5k–7k range; only an FTS5-vs-LIKE micro-decision would shift, and LIKE is safe at either end. |
| A5 (new) | Dropping the client-side `didYouMean` for the catalog field (relying on server substring+prefix ranking) is acceptable for WANT-01. | Q3 | LOW — server ranking covers the common case; if users miss typo-tolerance, layering `didYouMean` over the returned top-N is a cheap follow-up. Flag for the planner / a quick UX check. |
| A6 (new) | `current_avg` (price) in the search response is optional polish, not required. | Q3 | LOW — D-03 asks for name+ID search; price display is a bonus. Omitting it removes the only stale-data surface in search. |

### Addendum — Updated Architectural Responsibility Map (catalog-search row only)

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Add-item catalog search — **REVISED from "client-side over `view`"** | **API / Backend** (`readapi` `GET /api/v1/items/search` + `store.SearchCatalog` over `pigparse_price`) | Browser (debounced query + result render) | D-10: the full Blue catalog (~thousands of rows) lives only server-side in `pigparse_price`; it is too large to ship to the client and is not already fetched. The *in-bank join* row of the body's map is UNCHANGED (still client-side over `fetchView()`). |

### Addendum — Sources (this pass, all HIGH unless noted)

- `internal/backendsrv/enrich/jobs/pigparse.go` — the daily `RunPigparse` job: getall/1 fetch, WTS filter,
  upsert into `pigparse_price`, 304 short-circuit, truncation-as-log.
- `internal/backendsrv/enrich/pigparse.go` — the parser: `n` (name) is a REQUIRED_KEY (so name is always
  present); the "7,240 rows on 2026-05-09" live-capture count.
- `internal/backendsrv/store/enrich.go` — `pigparse_price` upsert (`ON CONFLICT(item_id)`, never deletes);
  the full column list.
- `internal/backendsrv/migrations/00001_init.sql` + `00003_enrich_columns.sql` — `pigparse_price` schema
  (PK `item_id`, +8 price columns); `item_master` schema (the wrong-table contrast).
- `internal/backendsrv/store/itemids.go` — proof `item_master` is guild-seen-only
  (`DistinctInventoryItemIDs`).
- `internal/backendsrv/store/readviews.go` — `InventoryJoin` already joins
  `inventory_item.item_id = pigparse_price.item_id` (proof of same id space); the read-method idiom to
  clone for `SearchCatalog`.
- `internal/backendsrv/readapi/views.go` — the GET-handler / nil->`[]` / V7-logging pattern to mirror for
  the search handler.
- `cmd/squirebot-server/main.go:265-314` — every read/account route is `RequireSession` (the auth gate for
  the new endpoint).
- `internal/backendsrv/store/{coin,admins,linking}.go` — `COLLATE NOCASE` is the established
  case-insensitive idiom.
- `.planning/research/ARCHITECTURE-v2.2.md:163,238-241,404` [CITED] — Phase 21 `wantmatch.ForItem(...
  itemID)` + `ec_auction_match` read `pigparse_price` by `item_id` (same id space as the pinned want).
- (No web sources — entirely codebase-verified + milestone-research-cited.)
