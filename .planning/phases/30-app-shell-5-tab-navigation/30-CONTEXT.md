# Phase 30: App Shell + 5-Tab Navigation - Context

**Gathered:** 2026-06-17
**Status:** Ready for planning

<domain>
## Phase Boundary

Web-only routing/chrome rework of squirebot.quest (`web/`, SvelteKit). Reframe the app
around **five persistent top-level tabs** — Characters · Inventory · Banks · Wishlist ·
Settings — each with its own in-context scoped search, a consolidated Settings home, and
the unread-alert badge + notification inbox moved onto the Wishlist tab. The navigation
chrome every later v2.4 surface plugs into.

**In scope (NAV-01..04):**
- The five-tab frame, in fixed order, with the active tab indicated, reachable from any page.
- Per-tab scoped search (functional on the tabs whose content exists in this phase).
- A single consolidated **Settings** tab (Theme · Watcher Codes · Set Class & Level ·
  My Characters · Admin[officer]) with a settings search.
- All notification surfaces (badge + inbox + prefs) relocated onto the **Wishlist** tab.

**Out of phase scope (owned by later phases — do not build the tab *content* here):**
- Characters tab list + in-game inventory window (Phase 31).
- Inventory tab item-centric list + holder drill-down (Phase 32).
- Banks tab list + valuation surface (Phase 33).
- The per-character/per-slot Wishlist *rework* (Phase 34). In Phase 30 the Wishlist tab is
  the EXISTING wantlist surface rehomed (plus notifications), NOT the per-slot rework.
- Any backend/data work (Phase 29 already shipped the data foundation; the watcher is untouched).
- Visual/design spec (exact tab styling, paperdoll, tooltip): `/gsd-ui-phase 30` produces UI-SPEC.

</domain>

<decisions>
## Implementation Decisions

**Resolution rule (user directive, 2026-06-17):** the user reviewed all four gray areas and
instructed: *"choose whichever option produces the simplest end-user experience."* All
decisions below were locked in one pass against that single criterion (see [[feedback_delegate_gray_areas]]).
"Simplest end-user experience" was read as: nothing the user relies on disappears, the URL
always reflects where you are, no surprising/empty/dead affordances, one place per concept.

### Routing & URL model (NAV-01)
- **D-01:** Five **real SvelteKit routes**, one per tab — `/characters`, `/inventory`,
  `/banks`, `/wishlist`, `/settings`. Rationale (simplest end-user experience): each tab is
  deep-linkable + shareable, browser back/forward behaves intuitively, and a refresh keeps
  you on the same tab. Client-side-only tab state was rejected because refresh/back/bookmark
  all surprise the user. The active tab is clearly indicated and the tab strip is reachable
  from any page.
- **D-02 (no broken bookmarks):** Every existing route **redirects** to its new home so no
  guildie's bookmark 404s: `/wantlist` → `/wishlist`; `/notifications` → `/wishlist`;
  `/account` (Watcher codes), `/char-meta` (Set class & level), `/my-characters`, `/admin`,
  `/bank-coin` → `/settings` (or the relevant Settings section); `/` → the default tab (D-04).

### Phase 30 delivery scope — rehome-what's-ready, stub-what's-new, never strand the user (NAV-01)
- **D-03:** Phase 30 wires the tabs whose features ALREADY EXIST to be **fully functional
  immediately**, and only **placeholders** the genuinely-new ones:
  - **Wishlist tab** = today's wantlist surface rehomed, PLUS the notifications badge/inbox/prefs
    moved onto it (D-07). Functional from day one.
  - **Settings tab** = today's Theme / Watcher codes / Set class & level / My characters /
    Admin consolidated (D-05). Functional from day one.
  - **Characters / Inventory / Banks tabs** = friendly "coming soon / building this" placeholders
    (content lands in Phases 31/32/33).
- **D-03b (core value must not regress):** Inventory/bank lookup IS the product's core value.
  The Characters/Inventory/Banks placeholders **preserve access to the existing consolidated
  inventory/gear/spell/bank views** (kept reachable — e.g. linked from the placeholder, or
  retained at a clearly-labeled interim path) until Phases 31–33 replace them. No guildie loses
  the ability to look up "what's in the guild" mid-transition. (Simplest end-user experience =
  the site never gets *less* useful during the rework.)

### Default landing (NAV-01)
- **D-04:** The tab ORDER is spec-fixed (Characters first) and does not change. But during the
  stub window the **default landing (`/`) resolves to a functional surface** — the preserved
  inventory view — NOT a "coming soon" Characters stub. (Greeting an active user with an empty
  placeholder is the worse experience.) Once Phase 31 ships, `/` / default can move to Characters.
  The planner owns the exact mechanism.

### Settings tab shape + gear-menu fate (NAV-03)
- **D-05:** Settings = **ONE page** composing the existing panels as in-page **sections**
  (Theme · Watcher codes · Set class & level · My characters · Admin[officer-only]), with a
  **settings search** that filters/jumps to sections. Reuses the already-built components
  (ThemePicker, WatcherCodesPanel, CharMetaForm, MyCharactersPanel, AssignmentAdminPanel +
  MonitorAdminPanel). Simplest = everything configurable in one place, fewer hops; rejected the
  "menu of links to separate sub-pages" because it adds navigation round-trips.
- **D-06:** The header **`SettingsMenu` gear dissolves** — all its configuration items move into
  the Settings tab. A **minimal identity + Sign out** affordance stays **top-right** (web
  convention; users expect sign-out there). The **Theme** control moves INTO Settings per the
  spec (no separate top-right theme picker). The `+layout → SiteShell → …` `[data-theme]` writer
  chain stays the single source of truth — only the picker's *location* moves.

### Notifications home — resolve NAV-03 vs. the sketch lock (NAV-04)
- **D-07:** ALL notification surfaces live on the **Wishlist tab**, NOT Settings: the unread
  **badge** sits on the Wishlist tab, and the **alert inbox** + **notification preferences**
  (both the global opt-in/cooldown prefs AND per-item ping toggles) are reached there. Every
  alert is framed as a wishlist-item ping → one mental model, one place (simplest). **This
  overrides the stale NAV-03 wording** ("Notifications prefs" listed under Settings) in favor of
  the LOCKED sketch-001 decision + [[project_consolidated_views]] / CLAUDE.md. The badge moves
  off the header/Settings onto the Wishlist tab.

### Per-tab search — no dead search bars (NAV-02)
- **D-08:** Each tab owns a search bar **scoped to its own content**, placed consistently at the
  top of the tab body (Variant A pattern from sketch 001). A search bar is shown **only when the
  tab has searchable content**: Wishlist + Settings get a functional scoped search in Phase 30;
  Characters/Inventory/Banks receive their scoped search **with their content** in Phases
  31/32/33. A search box that does nothing is the worse experience, so the stub tabs get NO inert
  search bar. The current home cross-character `SearchBox` becomes the **Inventory tab's** search
  when Phase 32 builds it. → NAV-02 is *pattern-established* in Phase 30 and *completed per-tab*
  as each content phase lands; the planner/verifier should treat it that way (not as "all five
  search bars functional in 30").

### Claude's Discretion (planner/researcher owns these)
- Exact redirect mechanism (SvelteKit `+page.ts` redirect vs. server redirect vs. reroute hook).
- Where/how the preserved classic inventory view is retained (interim `/legacy`-style path, an
  embedded body, or a link from the placeholder) — as long as access is never lost (D-03b).
- Whether the 5 tabs render via a shared layout (`+layout.svelte` tab strip + nested `+page`s) or
  a single shell component — pick the SvelteKit-idiomatic structure.
- Mobile/responsive collapse of the 5-tab strip (horizontal scroll vs. wrap vs. menu) — follow
  the UI-SPEC; the existing header already `flex-wrap`s.
- Exact Settings-search behavior (live section filter vs. jump-to-section).

</decisions>

<specifics>
## Specific Ideas

- Tab order is fixed and spec-literal: **Characters · Inventory · Banks · Wishlist · Settings**
  (`Future Features.txt` "a series of tabs along the top").
- Sketch 001 winner = **Variant A (top tabs)** — horizontal tab strip under the wordmark,
  per-tab search below; NOT the left sidebar (B) or hero-search (C) variants.
- Identity + sign-out placement = top-right "Joe ▾"-style affordance (sketch 001 "What to Look For").
- The unread badge currently sits in the header `SiteShell` next to a Notifications nav link →
  it moves onto the **Wishlist** tab (D-07 / NAV-04).

</specifics>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope & requirements
- `.planning/ROADMAP.md` — "Phase 30: App Shell + 5-Tab Navigation" section (goal, 4 success
  criteria) + the strict 29 → 30 → 31 → 32 → 33 → 34 chain (Phase 30 reframes routing for every
  later tab; can land with placeholder bodies).
- `.planning/REQUIREMENTS.md` — NAV-01, NAV-02, NAV-03, NAV-04 (note: D-07 resolves the NAV-03
  "Notifications prefs in Settings" wording in favor of Wishlist).

### Locked design direction
- `.planning/sketches/MANIFEST.md` — Locked design decisions (App shell = top tabs / Variant A;
  notifications belong to the Wishlist tab; Settings keeps Theme/Watcher Codes/Set Class & Level/
  My Characters/Admin; consolidated-views lock relaxed).
- `.planning/sketches/001-app-shell-5tab-nav/README.md` + `index.html` — the chosen app-shell
  variant, the notifications-move-to-Wishlist decision, and the identity/sign-out placement question.
- `Future Features.txt` (user's desktop, 2026-06-17) — authoritative target-UX spec; the five
  tab definitions and "each user sees a series of tabs along the top."
- `CLAUDE.md` — Architecture section: consolidated-views lock **relaxed** (per-character
  master-detail allowed); the EQ-theme system (single `[data-theme]` attribute on the shell root).

### Prior phase context (for continuity)
- `.planning/phases/29-data-foundation-inventory-parse-price-value-aggregation/29-CONTEXT.md` —
  the data foundation Phases 31–34 read (slot taxonomy, name-keyed price, bank valuation). Not
  consumed by Phase 30 directly but defines the model the tab content will render.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets (the surfaces Phase 30 reorganizes)
- `web/src/routes/+layout.svelte` — root layout; owns `[data-theme]` state + `AuthGate` + `SiteShell`.
  The natural home for (or sibling of) the new 5-tab strip.
- `web/src/lib/components/SiteShell.svelte` — current chrome (wordmark + header nav links
  [Inventory→`/`, Wantlist→`/wantlist`, Notifications→`/notifications`+badge] + `SettingsMenu` gear
  + footer). Phase 30 replaces the header nav links with the 5-tab strip; the gear dissolves (D-06).
- `web/src/lib/components/SettingsMenu.svelte` — the gear dropdown that already AGGREGATES the
  Settings surfaces as links (identity, ThemePicker, /account, /char-meta, /my-characters, /admin,
  Sign out). Its link list is the exact inventory the Settings tab composes as sections (D-05);
  identity + Sign out are what stay top-right (D-06).
- Settings-section components (compose onto `/settings`): `ThemePicker.svelte`,
  `WatcherCodesPanel.svelte` (`/account`), `CharMetaForm.svelte` (`/char-meta`),
  `MyCharactersPanel.svelte` (`/my-characters`), `AssignmentAdminPanel.svelte` +
  `MonitorAdminPanel.svelte` (`/admin`).
- Wishlist-tab content (already functional): `web/src/routes/wantlist/+page.svelte` +
  `WantlistPanel.svelte`; notifications to move onto it: `web/src/routes/notifications/+page.svelte`,
  `NotificationInbox.svelte`, `NotificationPrefsPanel.svelte`, `$lib/stores/unread.ts` (badge store).
- `web/src/routes/+page.svelte` — today's consolidated 4-view home (Inventory/Gear/Spell/Bank +
  SearchBox + My/Guild scope). This is the surface D-03b must keep reachable during the transition.
- `web/src/lib/components/SearchBox.svelte` — the cross-character item search; becomes the
  Inventory tab's search in Phase 32 (D-08).

### Established Patterns
- The tab pattern already exists in-page: `+page.svelte`'s `.view-nav` / `.tab` buttons + the
  `?view=bank` query-seed (260610-fm5) — informs the new top-level tab strip styling/active state.
- EQ-theme tokens only (`--accent`/`--panel`/`--font-display`); 44px touch targets; focus-visible
  2px accent outline — every new nav affordance follows this (see SiteShell/SettingsMenu styles).
- **Web tests are node-only / DOM-blind** ([[web-tests-node-only-blind-to-dom]]) — green vitest ≠
  works in the browser. A routing/nav rework MUST be browser-smoked on a deployed build (or a full
  local stack — localhost can't auth against prod, [[web-local-dev-cant-auth-against-prod]]).

### Integration Points
- New `/characters`, `/inventory`, `/banks`, `/wishlist`, `/settings` routes + redirects from the
  old paths (D-02).
- The `unread` badge store moves from the header onto the Wishlist tab (D-07 / NAV-04).
- `AuthGate` still wraps everything (session-gated); the per-tab visibility is UX, the server
  `RequireSession`/officer gates remain the real boundary.

</code_context>

<deferred>
## Deferred Ideas

- **Characters tab list + in-game inventory window** — Phase 31 (CHAR-01..03, INV-01..04).
- **Inventory tab item-centric list + holder drill-down** — Phase 32 (ITEM-01..03); the home
  SearchBox becomes this tab's search.
- **Banks tab list + valuation totals** — Phase 33 (BANK-01..03; reads Phase 29 `BankValuationFor`/
  `TotalPlatinum`).
- **Per-character/per-slot Wishlist rework** (open-ended upgrade targets + wiki suggestions +
  EC-hit badge) — Phase 34 (WISH-01..07). Phase 30 only rehomes the EXISTING wantlist surface.
- **WISH-07 full wishlist search** (across all wishlists + non-bank/bot characters) — Phase 34; the
  Wishlist tab's Phase-30 search is whatever the current wantlist offers, not the full rework search.
- **Retiring the preserved classic inventory/gear/spell view** — happens when Phases 31–33 replace
  it; do NOT delete it in Phase 30 (D-03b).
- **Gear Check / Spell Check views' long-term home** — Gear Check folds into the Phase 34 Wishlist;
  Spell Check is not in the v2.4 spec. Their disposition is decided in those phases, not Phase 30 —
  Phase 30 only keeps them reachable (D-03b).

None of the above is dropped — each is owned by its mapped downstream phase.

</deferred>

---

*Phase: 30-app-shell-5-tab-navigation*
*Context gathered: 2026-06-17*
