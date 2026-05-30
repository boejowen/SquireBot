---
phase: 14-web-frontend
verified: 2026-05-30T00:00:00Z
status: human_needed
score: 6/6 must-haves code-verified (build/test/parity green); 4/5 ROADMAP success criteria require deployed-site UAT
overrides_applied: 0
human_verification:
  - test: "Deploy web/build/ to Cloudflare Pages at app.squirebot.quest, deploy the new linux/amd64 server binary (with readapi routes) to the Hetzner VPS + restart, then open the static site in a browser"
    expected: "All four views (view, gear_check, spell_check, bank) render, each with a leading sticky Char column; every column filters and sorts; non-empty data appears (inventory/gear/spell rows from the live P12-populated SQLite)"
    why_human: "WEB-01 SC-2 is inherently visual/interactive over the DEPLOYED site with LIVE data. Confirmed NOT deployed: the live read API at api.squirebot.quest returns 404 (no CORS header) for /api/v1/views/view and /api/v1/meta — the box runs a pre-P14 binary (whoami still 401s, so the server is up, just stale). Cloudflare Pages static deploy is also pending. Code is built + tested but cannot render in a browser until both deploy steps (which the executor explicitly flagged as operational, mirroring P11's manual-deploy posture) are done."
  - test: "On the deployed gear_check + spell_check grids, eyeball the status badges against a known character's v1 Sheet output"
    expected: "gear_check shows OK/MISSING/OTHER vs Velious tiers and spell_check shows KNOWN/MISSING, matching what the v1 Sheet showed for the same character"
    why_human: "WEB-02 compute parity is AUTOMATED-PROVEN (Go table-tests translated from the v1 vitest fixtures pass: OK/OTHER/MISSING/Iksar/pair-slot/sort cases). But the end-to-end presentation against LIVE data (the StatusCell badges rendering the live read-API status) needs a human to confirm the colored badges appear and read correctly over real character data on the deployed site."
  - test: "On the deployed site, type a partial/misspelled item name in the search box and time the response; try a query with no exact match"
    expected: "Cross-character results return in well under 2s, each match lists holders as '↳ <Char>: <Location>, count <n>'; a no-exact-match query shows a single clickable 'Did you mean <suggestion>?' that re-runs the search when clicked"
    why_human: "WEB-03 logic is AUTOMATED-PROVEN (searchIndex 999.28 + 999.30 fixes, searchRows engine, didYouMean — all in the 60/60 vitest suite). The <2s feel, the holder rendering over live data, and the interactive did-you-mean click are interactive behaviors over the deployed site that code inspection cannot assert."
  - test: "On the deployed site, hover (desktop) and tap (touch) an Item cell; click the wiki link; switch themes via the picker"
    expected: "A rich-HTML tooltip popover opens with wiki summary + price lines + quest info, dismisses on Esc/outside-tap, and its wiki link opens the correct wiki.project1999.com page in a new tab; the theme picker flips the whole site's look via [data-theme] and the choice persists across reload (velious is the default with no stored pref)"
    why_human: "WEB-04 (escaped composeItemNote, rel=noopener, scheme-validated href) and WEB-05 (5-theme [data-theme] registry, velious default, localStorage) are AUTOMATED-PROVEN at the logic/registry level. But hover/tap popover positioning (IN-05 flags no viewport-collision handling), the actual EQ visual aesthetic per theme, and the persistence round-trip are visual/interactive over the deployed site — human UAT only."
  - test: "On the deployed bank view"
    expected: "The bank inventory grid renders (or the per-view-empty state if is_bank_toon is unset until P16); a 'Coin: not yet recorded' affordance shows above/with it; NO fabricated '0pp' anywhere"
    why_human: "Code-verified that the affordance string is rendered (StateBlock.svelte:63) and no 0pp is fabricated (grep 0 in +page.svelte). But whether the live bank view is non-empty depends on the P16 backfill setting is_bank_toon — the executor flagged an empty bank as expected-not-a-bug; a human should confirm the empty-state (not an error) renders if the data isn't there yet."
deferred:
  - truth: "The public read site is walled to guild members (Discord login gates read access)"
    addressed_in: "Phase 15"
    evidence: "ROADMAP P15 SC-1 (AUTH-08): 'A website visitor signs in with Discord OAuth2 and is admitted only if they are a member of the guild's Discord server'. CONTEXT D-04 explicitly makes P14 public read, P15 the gate. NOT a P14 gap."
  - truth: "Bank coin (platinum/gold/silver/copper) values display real numbers"
    addressed_in: "Phase 15"
    evidence: "ROADMAP P15 SC-4 (ADMIN-05): 'Manual bank-coin entry ... persists the four values'. CONTEXT <deferred>: coin is null/0 in P14 until P15's bank-coin form. The JSON {coin:null} shape + 'Coin: not yet recorded' affordance are the intentional P14 stub."
---

# Phase 14: Web Frontend Verification Report

**Phase Goal:** Give the guild back its read UI as a real website — a static SvelteKit app over a versioned Go read API that renders the four consolidated views (`view`, `gear_check`, `spell_check`, `bank`) — each with a leading `Char` column — as a reusable filterable/sortable data grid, with cross-character fuzzy search + "did you mean?", rich-HTML item tooltips (+ direct wiki link), and site-wide EQ theming. P14 is READ-ONLY (no login, no writes — deferred to P15).

**Verified:** 2026-05-30
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| #   | Truth (Success Criterion)                                                                                                              | Status        | Evidence |
| --- | -------------------------------------------------------------------------------------------------------------------------------------- | ------------- | -------- |
| 1   | **BACKEND-05** — backend exposes a versioned read API (`/api/v1/...`) returning the data powering all four views                        | ✓ VERIFIED (code) / ⚠ deploy-pending | 5 routes wired in `main.go:274-278` (`/api/v1/meta` + `/api/v1/views/{view,gear_check,spell_check,bank}`), each → `compute.*` over the SQLite store; `readapi` httptests green (405/CORS/coin-null/body-shape); `go test ./internal/backendsrv/...` all pass; whole repo builds. **Live box returns 404 (not yet deployed).** |
| 2   | **WEB-01** — guildie sees all four views, each with leading Char column, can filter+sort every column                                   | ? UNCERTAIN (human) | One reusable `DataGrid.svelte` (sticky Char col + sticky header, lines 291/335/340; faceted filters; multi-sort), instantiated 4× in `+page.svelte` (grep count 7, "never per-character tabs" comments); 4 column-def arrays each leading with `id:'char'` (`columns.ts:63/104/134`, bank=view). `npm run check` 0/0, `npm run build` emits index.html. **Visual/interactive over deployed site → human.** |
| 3   | **WEB-02** — gear_check OK/MISSING/OTHER vs Velious tiers + spell_check KNOWN/MISSING, matching v1 semantics                            | ✓ VERIFIED (compute) / ? presentation (human) | `compute/gearcheck.go` + `spellcheck.go` are full Go reimpls (load-bearing 3-branch OK→OTHER→MISSING order; Iksar-iff-IKS; level-gate; normalized-name set membership). **Go parity table-tests translated from v1 vitest fixtures PASS** — TestGearCheck_{HappyPath,Other,IksarTierOnlyForIKS,PairSlotMatchEAR2,SortOrder} + TestSpellCheck_{HappyPath,NormalizedNameMatch,SortOrder,...}. Live-data presentation → human. |
| 4   | **WEB-03** — cross-character search <2s + "did you mean?" (Wagner-Fischer Levenshtein) on no exact hit                                  | ✓ VERIFIED (logic) / ? feel (human) | `searchIndex.ts`: levenshtein verbatim, `didYouMean` with 999.28 guard (`:89`), `searchRows` engine; 999.30 fixed (0 `.skip` in test). `SearchBox` runs `searchRows` (`:33`); `SearchResults` renders holders + clickable `Did you mean` re-run via `onSuggest` (`:103`). In 60/60 vitest. <2s feel + interactive → human. |
| 5   | **WEB-04 + WEB-05** — per-item rich-HTML tooltip (summary+price+quest) + working wiki link; EQ theme site-wide                          | ✓ VERIFIED (logic) / ? visual (human) | `composeNotes.ts` emits escaped HTML, `{@html}` confined to `ItemTooltip.svelte:106`, `safeHttpUrl` scheme allow-list + `rel=noopener` (WR-01 fix, commit `d14e4ab`, 60/60). `themes.ts`: 5 keys, `DEFAULT_THEME='velious'`, no sheets-default; 8 `[data-theme]` blocks in app.css; `applyTheme` single-attr write + localStorage. Theme visuals + hover/tap positioning → human. |

**Score:** 6/6 must-haves code-verified (all artifacts exist, substantive, wired, tested; build + Go suite + 60 web tests green). 4/5 ROADMAP success criteria additionally require deployed-site UAT before they can be asserted end-to-end.

### Deferred Items

Items not met in P14 but explicitly scheduled for a later milestone phase (Step 9b).

| # | Item | Addressed In | Evidence |
|---|------|-------------|----------|
| 1 | Public read site walled to guild members (Discord login) | Phase 15 | ROADMAP P15 SC-1 (AUTH-08); CONTEXT D-04 makes P14 public by design, P15 the gate |
| 2 | Bank coin (pp/gp/sp/cp) shows real values | Phase 15 | ROADMAP P15 SC-4 (ADMIN-05); CONTEXT `<deferred>`: coin null/0 in P14; `{coin:null}` + "Coin: not yet recorded" is the intentional stub |

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `internal/backendsrv/store/readviews.go` | 8 parameterized read methods | ✓ VERIFIED | Exists; `?`-placeholders only; mirrors itemids.go; readviews_test.go green |
| `internal/backendsrv/compute/{types,view,bank,gearcheck,spellcheck,eqconst}.go` | Go reimpl of 4 v1 builders + snake_case row contract | ✓ VERIFIED | All exist + substantive; parity tests pass; `pickPrice` TEXT-direction fix; bank Coin nil |
| `internal/backendsrv/readapi/{views,meta,cors}.go` | 5 read handlers + stdlib CORS | ✓ VERIFIED | `ViewsHandler.ServeHTTP` dispatches 4 views; `MetaHandler`; CORS exact-origin (never `*`), OPTIONS 204; httptests green |
| `cmd/squirebot-server/main.go` | 5 routes registered + mux CORS-wrapped | ✓ VERIFIED | Lines 274-278 routes; 290 `readapi.CORS(*corsOrigin, mux)`; `-cors-origin` default `https://app.squirebot.quest` |
| `web/` SvelteKit static app | builds adapter-static, noindex, robots, no Google Fonts | ✓ VERIFIED | `npm run build` emits index.html + 200.html + robots.txt; noindex in index.html; 0 Google Fonts refs in build/ |
| `web/src/lib/search/searchIndex.ts` | levenshtein + didYouMean (999.28) + searchRows | ✓ VERIFIED | 999.28 guard `:89`; 0 `.skip` (999.30); in 60/60 vitest |
| `web/src/lib/tooltip/composeNotes.ts` | escaped rich HTML + scheme-safe href | ✓ VERIFIED | escapeHtml + safeHttpUrl; XSS + scheme tests pass |
| `web/src/lib/theme/themes.ts` + app.css | 5-theme [data-theme] registry, velious default | ✓ VERIFIED | 5 keys, no sheets-default, DEFAULT_THEME velious; 8 [data-theme] blocks |
| `web/src/lib/table/createSvelteTable{.ts,.svelte.ts}` | local table-core adapter (NOT svelte-table) | ✓ VERIFIED | table-core import; resolveUpdater Pitfall-2 unwrap; svelte-table absent; tableAdapter.test 8/8 |
| `web/src/lib/components/DataGrid.svelte` | one reusable grid, sticky Char+header | ✓ VERIFIED | position:sticky ×3; instantiated 4× |
| `web/src/lib/components/{ItemTooltip,SearchBox,SearchResults,StatusCell,StatusLegend,ThemePicker,SiteShell,StateBlock}.svelte` | the WEB-01..05 UI surface | ✓ VERIFIED | All 9 exist; `{@html}` confined to ItemTooltip; coin affordance in StateBlock:63 |
| `web/src/routes/{+layout,+page}.svelte` | SiteShell [data-theme] + 4 views/search wired | ✓ VERIFIED | data-theme + applyTheme; 5 fetch calls; 4 DataGrid instances; CC-BY-SA footer |
| `web/src/lib/api.ts` | 5 typed fetch wrappers over PUBLIC_API_BASE | ✓ VERIFIED | fetchView/GearCheck/SpellCheck/Bank/Meta; ApiError on non-2xx |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | -- | --- | ------ | ------- |
| `compute/view.go` | `store/readviews.go` | Store read methods | ✓ WIRED | compute imports store (one-directional); parity tests exercise the join |
| `compute/gearcheck.go` | `enrich/eqconst.go` | `enrich.WIKI_SLOT_TO_INV_SLOTS` | ✓ WIRED | Reused not re-typed (gearcheck.go:102) |
| `readapi/views.go` | `compute` | compute.View/Bank/GearCheck/SpellCheck | ✓ WIRED | switch dispatch; httptests decode each shape |
| `main.go` | `readapi.CORS` | `Handler: readapi.CORS(*corsOrigin, mux)` | ✓ WIRED | main.go:290 |
| `ItemTooltip.svelte` | `composeNotes.ts` | `{@html composeItemNote(...)}` | ✓ WIRED | Sole `{@html}` sink; input fully escaped |
| `DataGrid.svelte` | `@tanstack/table-core` | local createSvelteTable adapter | ✓ WIRED | adapter test proves sort/filter |
| `+page.svelte` | `api.ts` | parallel fetch of 4 views + meta | ✓ WIRED | Promise.all of 5 fetchers (:86-90) |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| -------- | ------------- | ------ | ------------------ | ------ |
| `compute/view.go` (View rows) | InventoryJoin + QuestLinks | real SQLite SELECT...JOIN over inventory_item/item_master/pigparse_price/quest_items | Yes (DB query, P12-populated dims) | ✓ FLOWING (in-process; parity tests seed real temp DB) |
| `readapi/views.go` JSON | compute.* return | compute over store | Yes (no static/hardcoded returns; nil→[] coercion only) | ✓ FLOWING (httptest decodes non-empty seeded bodies) |
| `+page.svelte` grids | fetchView/etc. | `https://api.squirebot.quest/api/v1/...` | **Not over the wire yet** — live endpoint 404 (binary not deployed) | ⚠ DISCONNECTED at runtime (deploy-pending, not a code defect) |
| `bank.coin` | BankView.Coin | always nil in P14 | No (intentional P15/ADMIN-05 deferral) | ⚠ STATIC by design (deferred, "Coin: not yet recorded" rendered, no fabricated 0pp) |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Go backend suite (compute+readapi+store) | `go test ./internal/backendsrv/...` | all packages `ok` | ✓ PASS |
| Whole-repo Go build | `go build ./...` | exit 0 | ✓ PASS |
| Gear/spell v1 parity table-tests (WEB-02 oracle) | `go test ./compute/ -run 'GearCheck\|SpellCheck' -v` | all PASS (OK/OTHER/MISSING/Iksar/EAR2/sort) | ✓ PASS |
| Web type-check | `npm run check` | 398 files, 0 errors, 0 warnings | ✓ PASS |
| Web unit suite (incl. WR-01 scheme tests) | `npx vitest run` | 60/60 across 5 suites | ✓ PASS |
| Static build emits SPA shell | `npm run build` | writes index.html + 200.html + robots.txt | ✓ PASS |
| noindex / robots / no Google Fonts | grep build/ | noindex present, Disallow:/, 0 Google Fonts | ✓ PASS |
| Live read API serves views | `curl https://api.squirebot.quest/api/v1/views/view` | **HTTP 404, no CORS header** | ✗ FAIL (not deployed; server up — whoami 401s) |

### Requirements Coverage

All 6 declared requirement IDs accounted for. Each is mapped in REQUIREMENTS.md Traceability (lines 97-102) to P14 plans and is code-satisfied; the 5 web-facing ones additionally need deployed-site UAT.

| Requirement | Source Plan(s) | Description | Status | Evidence |
| ----------- | -------------- | ----------- | ------ | -------- |
| BACKEND-05 | 14-01, 14-03, 14-04 | Versioned read API powering the 4 views | ✓ SATISFIED (code) / deploy-pending | readviews + compute + 5 readapi routes + api.ts client; httptests green; live 404 (deploy step) |
| WEB-01 | 14-04 | 4 consolidated views, leading Char, filter+sort | ✓ SATISFIED (code) / ? UAT | one DataGrid ×4, sticky Char+header, 4 column defs, faceted filters, multi-sort |
| WEB-02 | 14-01, 14-04 | gear OK/MISSING/OTHER + spell KNOWN/MISSING vs v1 | ✓ SATISFIED | Go parity table-tests from v1 fixtures PASS; StatusCell badges |
| WEB-03 | 14-02, 14-04 | cross-char search + did-you-mean <2s | ✓ SATISFIED (logic) / ? UAT | searchIndex (999.28+999.30 fixed), searchRows, SearchBox/Results clickable did-you-mean |
| WEB-04 | 14-02, 14-04 | per-row tooltip + wiki link | ✓ SATISFIED (logic) / ? UAT | composeNotes escaped HTML + safeHttpUrl, ItemTooltip hover/tap, rel=noopener |
| WEB-05 | 14-02, 14-04 | EQ aesthetic theme site-wide | ✓ SATISFIED (registry) / ? UAT | 5-theme [data-theme] registry, velious default, applyTheme + localStorage |

No ORPHANED requirements: REQUIREMENTS.md coverage check maps exactly P14 = 6 (BACKEND-05 + WEB-01/02/03/04/05); all 6 appear in plan frontmatter (`requirements:` fields across 14-01..04). No P14 IDs missing from any plan.

### Anti-Patterns Found

Code review (`14-REVIEW.md`, standard depth, 47 files) found 0 Critical, 4 Warning, 5 Info. The one security Warning (WR-01) is FIXED on disk (verified). The rest are non-blocking robustness/UX, suitable for an optional `/gsd-code-review-fix` pass.

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| composeNotes.ts / WikiCell.svelte | — | WR-01 wiki-URL scheme not validated | ⚠ Warning (FIXED) | `safeHttpUrl` http(s) allow-list added at choke point + both sinks (commit `d14e4ab`); genuine scheme tests added; 60/60. Not live-exploitable (sole producer hardcodes https). RESOLVED. |
| DataGrid.svelte:83 / columns.ts | 87-93 | WR-02 global filter matches hidden raw column values (last_synced ISO, id/price) | ⚠ Warning (open) | Degrades search/filter UX (phantom matches on the raw ISO date); non-blocking, deferred |
| +page.svelte:109-111 | — | WR-03 `load()` driven by bare `$effect` instead of `onMount` | ⚠ Warning (open) | Fires once today; fragile if `load()` later reads reactive state; non-blocking |
| api.ts:122-125 | — | WR-04 `getJSON` assumes every 2xx body is valid JSON of shape T | ⚠ Warning (open) | Malformed/empty 2xx body throws unclassified SyntaxError; lands on generic error state; non-blocking |
| composeNotes.test.ts:156 | — | IN-01 misleading test name (since corrected by WR-01 fix) | ℹ Info | RESOLVED by d14e4ab (real scheme tests added) |
| +layout.svelte:23-30 | — | IN-02 `[data-theme]` written twice (idempotent) | ℹ Info | Harmless redundancy; maintenance trap |
| SearchResults.svelte:53-57 | — | IN-03 synthesized search tooltip shows "0 transactions" | ℹ Info | Cosmetic |
| createSvelteTable.svelte.ts:83-91 | — | IN-04 comment overstates reactivity ownership | ℹ Info | Doc-only; behavior correct (adapter test proves) |
| ItemTooltip.svelte:127-147 | — | IN-05 tooltip no viewport-collision/flip handling | ℹ Info | UX robustness gap on narrow screens; acceptable to defer |

### Human Verification Required

The phase goal and 4 of the 5 ROADMAP success criteria are written as live, in-browser guildie behaviors ("a guildie opens the static site and sees all four views and can filter/sort/search/hover"). These are inherently visual/interactive over the **deployed** site with **live** data, and two deploy steps the executor explicitly flagged as operational (mirroring P11's manual-deploy posture) are NOT yet done:

1. **The static frontend is built (`web/build/`) but not published** to Cloudflare Pages at `app.squirebot.quest`.
2. **The read-API server binary is not deployed** — the live box at `api.squirebot.quest` returns **404 (no CORS header)** for `/api/v1/views/view` and `/api/v1/meta` (the server is healthy — `/api/v1/whoami` still 401s — it is just running a pre-P14 binary without the `readapi` routes).

Until both deploy, the views render no data over the wire and the visual/interactive criteria cannot be asserted from code alone. See the `human_verification` frontmatter for the five concrete UAT items (open the site; eyeball gear/spell badges vs v1; time + click search/did-you-mean; hover/tap tooltip + click wiki link + switch themes; confirm the bank no-coin affordance / empty-state-not-error).

**This is NOT a code gap.** Every must-have artifact exists, is substantive, is wired, and is covered by green automated tests (Go parity + 60 web unit tests + build). The blocker to a `passed` verdict is purely the deferred deployment + live UAT, which is correct for this phase's manual-deploy posture.

### Gaps Summary

No code gaps. No must-have is FAILED. The Go backend (compute/readapi/store) and the web suite (60 tests) both pass; the whole repo builds; the static SPA emits; the WEB-02 parity oracle (the load-bearing proof) is green. The single security Warning (WR-01) is fixed and re-tested on disk. The three remaining code-review Warnings (WR-02/03/04) are non-blocking robustness/UX items deferred to an optional review-fix pass.

The only reason this is `human_needed` rather than `passed`: the visual/interactive success criteria require the static site published to Cloudflare Pages **and** the new server binary deployed to the VPS (the live read API currently 404s), after which a human performs the five UAT checks above. Discord-gated read (AUTH-08) and real bank-coin values (ADMIN-05) are correctly deferred to P15 and do not count against P14.

---

_Verified: 2026-05-30_
_Verifier: Claude (gsd-verifier)_
