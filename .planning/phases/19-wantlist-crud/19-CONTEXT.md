# Phase 19: Wantlist CRUD - Context

**Gathered:** 2026-06-03
**Status:** Ready for planning

<domain>
## Phase Boundary

A signed-in guildie maintains a **personal, private** wantlist on squirebot.quest. They can:
- **Add** an item to their wantlist — searching the existing item catalog (or entering a custom text want), tagging a reason (buy vs quest), an optional Low/Med/High priority, and an optional note.
- **View** their full wantlist and **remove** any entry.
- See, per row, **whether and where** the item already exists in the guild (which character(s) hold it + count).

Backed by one goose migration `00006_wantlist.sql` (≥ `wantlist_item`; `alert_log` is created here but consumed by Phase 20). The wantlist is owner-scoped and tied to the Discord `web_user` — strictly private to its owner this phase.

**This phase is pure website CRUD — NO Discord bot, NO DM, NO monitors.** Those are Phases 20–23. The wantlist is the data surface those later monitors will match against.

**Explicitly NOT in this phase** (deferred — redirect, don't fold): guild-wide "who wants what" roll-up, WTB monitoring, price-threshold alerts, any Discord alerting.
</domain>

<decisions>
## Implementation Decisions

### Entry fields (WANT-01)
- **D-01:** A `wantlist_item` row captures: `item_id` (nullable — see D-04), `reason` **required** (`buy` | `quest`), `priority` (`low` | `med` | `high`, **default `med`**), and an **optional** `note` (freeform text, **≤ 280 chars**).
- **D-02:** Priority is a simple Low/Med/High enum — NOT a numeric rank. It drives the default list sort (see D-08).

### Add-item scope & dedupe (WANT-01)
- **D-03:** Add-item search is **catalog-first**: search the item catalog by name + ID, reusing the fuzzy/"did you mean?" matching idiom. A catalog hit pins a real `item_id`.
- **D-04:** **Custom text wants are allowed** — a guildie can add a want that does NOT resolve to a catalog item (`item_id` NULL, a free-text label instead). Custom wants are **display-only**: they are excluded from the in-bank join (D-06) and CANNOT be auto-matched by the later EC/WTS/raid monitors (Phases 21–23). The UI must clearly mark a custom want as "custom — won't trigger alerts".
- **D-10 (catalog source — REVISED after research, supersedes the "existing catalog" assumption):** The searchable catalog is the **full Blue item set sourced from the daily PigParse `getall` ingest** — NOT `item_master` (which only holds items the guild has *seen in inventory*). Rationale: an item nobody owns yet is exactly what a guildie most wants alerted on; sourcing from the full PigParse catalog means **any real Blue item resolves to a stable `item_id` at add-time, so it is alert-capable in Phase 21+**. This is a deliberate scope expansion beyond the roadmap's "existing item catalog" wording (user decision 2026-06-03), accepted because it is the milestone's reason for existing. The custom-want path (D-04) remains the escape hatch only for items not even in PigParse.
  - **Implication for the planner:** the full Blue catalog (~thousands of items) is too large for the current client-side-over-`fetchView()` search model — this almost certainly requires a **new server-side item-search endpoint** (e.g. `GET /api/v1/items/search?q=`) backed by the PigParse-getall data already ingested daily. Confirm the exact storage of the getall ingest in RESEARCH.md before planning the endpoint. The in-bank join (D-06) stays a client-side derivation of `fetchView()` — only the catalog *search* changes.
- **D-05:** **The same item may appear twice with different reasons** (one `buy`, one `quest`). Uniqueness key is **`(web_user, item_id, reason)`** for catalog items. For custom wants (NULL `item_id`), dedupe on `(web_user, custom_label, reason)` or skip the DB constraint and dedupe in the handler — planner's call; the user-visible rule is "no exact duplicate (same item + same reason)".

### In-bank indicator (WANT-02)
- **D-06:** The "already in the guild bank?" indicator goes **deep**: show **which character(s) across the entire guild hold the item and the count** — directly serving the project core value ("what do I need, and WHERE in the guild is it"). This is a join against the existing **consolidated all-character inventory** data (not bank-toon-only).
- **D-07:** Only catalog wants (with an `item_id`) get an in-bank result; custom wants show "—"/not-applicable. The join key is the stable `item_id` (per PROJECT.md: item Name strings drift, IDs are stable).

### Claude's Discretion
- **D-08 (List presentation — defaulted, not discussed):** Reuse the existing filterable/sortable **DataGrid** component + the existing rich HTML item tooltips (twin of the 4 views). **Default sort: priority (high→low) then in-bank status.** Friendly empty state ("Your wantlist is empty — search the catalog to add what you're after"). Planner may adjust columns.
- **Security/identity shape (locked upstream, not re-discussed):** the `webadmin/account.go` twin — login-only, owner derived **server-side** from the Discord session (never client-supplied, per v2.1 D-02), IDOR-safe, audited via the existing `audit.go` seam.
- Custom-want storage detail (separate `custom_label` column vs reusing `note`) is the planner's call — but a custom want MUST be visually distinct and excluded from matching.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Milestone scope & locked decisions
- `.planning/REQUIREMENTS.md` — WANT-01, WANT-02 (the two requirements this phase delivers) + the v2.2 locked decisions.
- `.planning/ROADMAP.md` §"Phase 19: Wantlist CRUD" — goal + 4 success criteria (the acceptance contract).
- `.planning/research/SUMMARY-v2.2.md` — synthesized v2.2 research (the `wantlist_item` + `alert_log` schema sketch, the "one match seam, three sources" spine that Phase 19's data must feed).
- `.planning/research/ARCHITECTURE-v2.2.md` — component/table/build-order detail for the `00006_wantlist.sql` migration.

### Pattern twins (the closest existing analogs to copy)
- `internal/backendsrv/webadmin/account.go` (+ `account_test.go`) — **the security-shape twin**: login-only, owner-scoped, IDOR-safe, audited. `wantlist.go` should mirror its structure.
- `internal/backendsrv/webadmin/charmeta.go` (+ `charmeta_test.go`) — login-only per-user write form; another close analog for a personal CRUD surface.
- `internal/backendsrv/webadmin/audit.go` — the audit-logging seam to reuse.
- `internal/backendsrv/migrations/00005_self_service_linking.sql` — the most recent migration; `00006_wantlist.sql` follows its idempotent/extend-only conventions. `embed.go` + `migrate_test.go` cover the migration test pattern.
- `web/src/routes/account/` and `web/src/routes/char-meta/` — the SvelteKit route twins for a login-gated personal page; `web/src/routes/wantlist/` should mirror them.

### Reused data/compute
- `internal/backendsrv/compute/` (`compute.Bank` and the consolidated inventory builders) + `internal/backendsrv/readapi/` — the source of the all-character inventory data the in-bank indicator (D-06) joins against, and the item-catalog search behind add-item (D-03).
- `CLAUDE.md` — extend-only schema evolution, structured-logging conventions, item-ID-as-join-key.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`webadmin/account.go`** — direct structural template for `webadmin/wantlist.go`: session-derived owner, owner-scoped queries, IDOR guards, audit calls, table-driven handler tests.
- **`web/src/routes/account` / `char-meta`** — SvelteKit login-gated page scaffolding (load function, form actions / fetch to the Go handler) to clone for `web/src/routes/wantlist`.
- **Existing item catalog + fuzzy "did you mean?" search** (frontend) and the read-API item lookups (backend) — power the add-item search (D-03) without new catalog infrastructure.
- **Consolidated all-character inventory / `compute.Bank`** — already aggregates who-holds-what; the in-bank indicator (D-06) is a join, not new aggregation.
- **DataGrid + HTML item tooltip components** (`web/`) — reuse for the wantlist table (D-08).

### Established Patterns
- **Server-side owner resolution** (v2.1 D-02): owner comes from `caller(ctx)` / the Discord session, NEVER from the request body.
- **Extend-only, idempotent goose migrations** (`00001`→`00005`); `00006` adds `wantlist_item` (+ `alert_log`) without altering existing tables.
- **`item_id` is the stable join key** — names drift; the in-bank join and future monitors key on `item_id`, which is exactly why custom (NULL-`item_id`) wants are unmatched (D-04).

### Integration Points
- New `webadmin/wantlist.go` mounts on the existing authed webadmin router (twin of `account`/`charmeta` registration).
- New `web/src/routes/wantlist/` page behind the existing Discord-login gate.
- `00006_wantlist.sql` added to `internal/backendsrv/migrations/` (embedded via `embed.go`).
- `wantlist_item.item_id` + the consolidated inventory data → the in-bank indicator; the same rows are what Phase 20's `wantmatch` spine will read.

</code_context>

<specifics>
## Specific Ideas

- The in-bank indicator should feel like the product's core value made concrete — not just a checkmark, but "Borticus has 1, guild bank has 2." Lean on the existing cross-character inventory so this is a join, not a rebuild.
- Custom wants are a deliberate escape hatch for items the catalog doesn't know yet — but they must visibly read as "won't trigger alerts" so a guildie isn't surprised later when Phase 21+ never DMs them about a custom want.

</specifics>

<deferred>
## Deferred Ideas

- **Guild-wide "who wants what" roll-up** — aggregating everyone's wantlists into a shared view. Valuable, but a separate feature once individual wantlists exist (already in REQUIREMENTS.md "Future Requirements"). Not this phase.
- **WTB monitoring / price-threshold alerts** — refinements on the alert side (Phases 21+); out of scope for CRUD.
- **Auto-deriving the catalog entry for a custom want later** (promote a custom text want to a real `item_id` once the catalog learns it) — nice future polish; not required now.

</deferred>

---

*Phase: 19-Wantlist CRUD*
*Context gathered: 2026-06-03*
