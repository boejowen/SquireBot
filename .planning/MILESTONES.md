# SquireBot Milestones

## v2.4 Web UI Revamp (Shipped: 2026-06-21)

**Phases completed:** 11 phases, 34 plans, 79 tasks

**Key accomplishments:**

- The SvelteKit /wantlist surface — a debounced catalog-search add form (with a custom-want escape hatch flagged "won't trigger alerts"), a server-truth DataGrid of the owner's wants with the deep "In guild" holder display (one summed ↳ line per character), and ConfirmDialog removal — composed from existing components atop DOM-free reduce-by-char-and-SUM logic. DEPLOYED + browser-verified live at squirebot.quest/wantlist (schema v6).
- Forward-only goose migration 00007 (notify_prefs/guild_channel/monitor_flag + an alert_log rebuild making wantlist_item_id NULLABLE plus read_at + wantlist_item.muted) and the owner/officer-scoped store layer the bot seam, inbox, prefs, monitor controls, and per-want mute all read and write.
- The alerting spine's Go core: the discordgo dependency + a recover-isolated non-fatal `bot` gateway package, a `notify` package that opens+sends DMs with first-class 50007→dm_blocked handling and a local-tx alert_log audit behind a two-gate+dedup wall, and the single shared `wantmatch` ForItem/ForName seam — all three packages unit-tested against a mocked Discord sender (no live gateway).
- The notifications/monitors HTTP spine — 12 gated routes (prefs/inbox/mute + officer monitors + the WantID=nil test-alert) plus a non-fatal in-process Discord bot wired into runServe; code-complete and verified bot-disconnected, live DM proof deferred to the end-of-phase deploy.
- The guildie-facing /notifications page — server-truth alert preferences (master + 3 per-monitor Toggles, default-ON) stacked over the full alert inbox with word+icon delivery badges, the CAN'T-DM safety-net hint, and the unread-count nav badge — built on one new accessible Toggle primitive + the notifications api.ts block, all node-green (svelte-check 0 errors, 241 vitest, build emits /notifications).
- Officer /admin Monitors section (three guild-wide kill-switch Toggles, an add-channel form, a ConfirmDialog-gated channel list, and the D-10 "Send me a test alert" bot-pulse with three feedback states) plus a per-want bell/bell-off mute column on the /wantlist grid — all node-verified green, browser-smoke deferred to the end-of-phase prod-deploy live smoke.
- Live PigParse spike picked the per-auction `getdetails` path (correcting the research's server number AND key form), plus the NEW t/u-collision-aware getdetails parser, the `ec_auction_cursor` diff-cursor migration, and the first-sight/upsert/poll-set store layer the producer job needs.
- The integration finale (WANT-05): a new `internal/backendsrv/ec` package whose `RunMatch` polls PigParse `getdetails/0/{name}` per wanted item, diffs new WTS auctions against the per-item `ec_auction_cursor`, matches via `wantmatch.ForItem`, and DMs a rich discordgo embed through `notify.Send` — re-implementing none of the Phase 20 spine — registered as the `ec_auction_match` scheduler job with the live bot session threaded `main.go → scheduler → ec`.
- Task 1 — tray split (`bb8e214`)
- Task 1 — 0600-file credstore (`09d6d72`)
- Task 1 — OS-specific manifest fields + GOOS asset selection (`6e31ec3`)
- 1. [Rule 3 - Blocking] Reworded the InventoryJoinRow historical doc note to satisfy the acceptance grep
- The INV-05 `classifySlot` + one-level `<Parent>-Slot<N>` nesting parser, the DATA-02 flat bank valuation (`Σ pickPrice×count`, +N unpriced) + nil-safe total-platinum aggregation as pure transforms over the 29-01 store reads, and the DATA-01 `store.GearTierPrices` name-join that resolves a price for the NULL-item_id `wiki_gear_tier` rows — closing ROADMAP success criterion #2 in this phase, all unit-tested over the real-name nested-bag fixture with zero schema migration and the watcher untouched.
- The persistent 5-tab navigation strip, the dissolved gear → top-right identity affordance with a theme-context bridge, the Wishlist-tab unread badge, and 8 old-path redirects + a preserved /guild-views home with 3 coming-soon placeholders — the routing/chrome foundation Plan 02's Wishlist + Settings bodies plug into.
- The two functional tab bodies on Plan 01's spine: the Wishlist tab (rehomed wantlist + notifications inbox/prefs, the wantlist filter serving as the NAV-02 scoped search), and the Settings tab (the 6 existing panels composed as in-page id'd sections behind an officer-gated Admin section, with a live settings search and the theme picker relocated via the Plan-01 THEME_KEY context bridge) — code-complete + all node gates green, the load-bearing deployed browser-smoke (Task 4) PENDING a human.
- `item_master.icon_id` populated from the wiki `lucy_img_ID` (migration 00012) and plumbed — alongside the per-character "Last synced" value — through `InventoryForChar` → `compute.StructuredInventory` into the `InventorySlot.icon_id` / `CharacterInventory.last_seen` JSON contract the web inventory window will render.
- Two new `webauth.RequireSession`-gated read endpoints — `GET /api/v1/inventory/{char}` (one character's `compute.StructuredInventory`, including the Plan 31-01 `icon_id`/`last_seen`) and `GET /api/v1/characters` (the viewer-first band-tagged roster) — plus the `RosterFor` store read that returns the full roster shape (`is_mine` + bank/bot flags + meta + `last_seen`) no existing read returned together.
- The SvelteKit Characters tab over the Plan 31-02 read API: typed `fetchCharacters`/`fetchInventory` wrappers + `RosterCharacter`/`CharacterInventory`/`InventorySlot` interfaces in `api.ts`, a pure node-tested `roster.ts` (viewer-first banding + viewer-priority search), and the rebuilt `/characters` page — a bespoke 3-band viewer-first list, a scoped search, and `?c=<name>` selection wiring that drives the inventory window (the window component itself lands in Plan 31-04, so the window column prompts "Pick a character" until then).
- The in-game-style inventory window (INV-01..04): a pure node-tested `examine.ts` (the D-08 field order + D-09 omission, `last_seen` NOT `last_listed`), a `PaperdollSlot` tile (wiki `Item_<iconId>.png` over a deterministic colored-tile `onerror` fallback + stack count + bag marker + a11y), an `ExaminePanel` (the D-08 rows with the single escaped `composeItemNote` `{@html}` sink), and a GENERIC prop-driven `InventoryWindow` (23-slot paperdoll + general/bank grids on one renderer + INLINE bag expand + hover-preview/click-to-pin examine) — wired into `/characters` over the 31-03 `?c=` selection with per-character loading/error/no-inventory states. CODE COMPLETE + web gates green; the mandatory backend+migration+web DEPLOY and the browser-smoke are PENDING (the Task-3 human-verify checkpoint — node vitest is DOM-blind).
- A new compute.Items(ctx, store, viewerID) that groups every guild holding by normalized name into one-row-per-item rollups (summed qty, distinct holder count, viewer is_mine, name-keyed price/wiki, id-correct icon/stats, per-holder slot/qty/last-synced), served at a new session-gated GET /api/v1/items.
- The SvelteKit web half of the item-centric Inventory tab over `GET /api/v1/items`: the `api.ts` ItemRollup/ItemHolder interfaces + `fetchItems()` wrapper, a pure node-tested `items.ts` (viewer-first sort / name filter / holder band sort), and the rebuilt `/inventory/+page.svelte` master-detail tab — a bespoke viewer-first selectable item list whose detail is the REUSED P31 `ExaminePanel` plus a holders table whose rows deep-link into the live `/characters?c=<name>` window.
- The item-centric Inventory tab is LIVE at https://squirebot.quest/inventory over the new `GET /api/v1/items` route. Deploy = a backend binary swap (server restarted to register the route — NO goose run; schema stays v13) + a web atomic swap, with a pre-deploy R2 backup. The 7-point browser-smoke PASSED on the live build across all 5 EQ themes after one fix-forward (un-sticking the examine panel so it stopped covering the holders table on scroll).
- `GET /api/v1/banks` surfaces a WIDENED bank+bot valuation — `compute.Banks` over the new `InventoryJoinBanksAndBots`/`ListBankAndBotToons` reads (Option B) so a guild bot's goods finally count toward the guild item-value total, with per-bank item count + value + nullable platinum A-Z, session-gated, no new migration.
- The `/banks` placeholder is replaced with the real master-detail Banks tab — a D-02 guild valuation summary header (total item value pp + total platinum), an A-Z bank/bot list whose left column toggles to a BANK-03 item-search scoped to bank holders (with the bank-slice qty RECOMPUTED off the guild-wide P32 rollup), and a D-04 per-bank value/plat header above the reused P31 InventoryWindow — pinned in-tab by clicking either a list row or a search holder; web-only, typecheck + build + 370 node tests green.
- The Banks tab is LIVE at https://squirebot.quest/banks over the new `GET /api/v1/banks` route. Deploy = a backend binary swap (server restarted to register the route — NO goose run; schema stays v13) + a web atomic swap, with a pre-deploy R2 backup. The 10-point browser-smoke PASSED on the live build across all 5 EQ themes on the first deploy (no fix-forward needed).
- Migration 00014 (schema v14, the D-01 clean break) + owner-scoped store/wishlist.go CRUD + store.PriceByName + compute.WishlistFor — the per-character/per-slot wishlist data + transform layer (schema, store reads, the WishlistFor compute) every later Phase-34 plan reads. NO route/web/watcher change.
- The EC matcher repointed to the new `wishlist_item` table (one SELECT each in `wantmatch.ForItem`/`ForName` + the `ECPollSet` pre-filter), the owner-scoped write API (`webadmin/wishlist.go` add/remove/ping — the `wantlist.go` clone, with the in-tx `IsCharAssignedToTx` 403 + 409 dup + silent IDOR no-op), the per-character read route (`GET /api/v1/wishlist/{char}` over `compute.WishlistFor`), and the full clean-break test repair so `go test ./...` is GREEN again. The DM-target-is-owner invariant (T-34-08) is preserved + regression-tested. NO web/watcher change; NO new migration.
- The `/wishlist` SvelteKit tab built to the 34-UI-SPEC: a viewer-first character list (banks/bots excluded), the WISH-07 two-group scoped search whose WISHLIST-ITEMS corpus is EVERY non-bank/bot character's wishlist (lazy-fetch + cache, no scope-down), the 21-slot accordion (equipped + auto-removal-filtered targets + class/slot suggestions + ping Toggle + EC-hit badge + the reused ExaminePanel), and server-truth owner-scoped add/remove/ping — plus the `api.ts` interfaces/wrappers (mirroring the Go WishlistView contract) and a pure node-tested `wishlist.ts`. The superseded WantlistPanel + groupByChar are deleted; priority.ts/holders.ts are kept. NO backend/watcher change; NO deploy (34-04).

---

Historical record of shipped versions. Each entry links to the milestone archive in `.planning/milestones/`.

---

## v1.0 — Watcher + Workbook + Onboarding (initial release)

**Shipped:** 2026-05-11
**Tag:** `v1.0.0` (additionally `phase5-complete`)
**Archive:** [`milestones/v1.0-ROADMAP.md`](milestones/v1.0-ROADMAP.md) · [`milestones/v1.0-REQUIREMENTS.md`](milestones/v1.0-REQUIREMENTS.md) · [`milestones/v1.0-MILESTONE-AUDIT.md`](milestones/v1.0-MILESTONE-AUDIT.md)

**Stats:** 5 phases · 31 plans · 203 commits · 14k LOC Go (watcher) + 11k LOC TypeScript (apps-script) · 11 days kickoff to ship (2026-04-30 → 2026-05-11)

**Phases:** All shipped sequentially.

| Phase | Name | Shipped As | Date |
|-------|------|-----------|------|
| 1 | End-to-End Thin Slice | v0.1.0 | 2026-05-02 |
| 2 | Watcher Robustness + Schema Lock | v0.2.0 / v0.2.1 hotfix | 2026-05-09 |
| 3 | Apps Script Enrichment Foundation | v0.3.0 | 2026-05-10 |
| 4 | Differentiator Features | v0.4.0 | 2026-05-11 |
| 5 | Search + Onboarding + Privacy Polish | v1.0.0 | 2026-05-11 |

**Key accomplishments:**

1. **End-to-end thin slice** — installer + OAuth + watcher + workbook in 5-step user flow, no UAC, ~12 MB binary, Azure VM smoke 16/17 PASS
2. **Hardened watcher** — spellbook + multi-folder + retry/backoff + heartbeat + auto-update + schema lock at `schema_version=1`; survived 7-day soak with deliberate `invalid_grant` injection
3. **Apps Script enrichment layer** — daily PigParse + weekly P1999 wiki summary triggers; consolidated `view` tab with hyperlinks, prices, conditional Last-Synced formatting, cell-note tooltips; `politeFetch` helper with ETag/If-Modified-Since
4. **Gear + spell differentiators** — wiki gear-tier scraper (Velious Pre-Raid + Raiding + Iksar) + per-class spell-list scraper (all 4 P1999 template variants, 11 caster classes, 1,562 spells); `gear_check` / `spell_check` consolidated tabs with OK/MISSING/OTHER status; manual bank coin sidebar with `Range.protect()`
5. **Search + eviction + onboarding** — 300px HtmlService cross-character search sidebar (Wagner-Fischer Levenshtein "Did you mean?"); lock-guarded eviction workflow with 30-day grace + lazy `_archive` tab + `_meta.eviction_log` envelope contract; Jekyll Pages onboarding site live at https://boejowen.github.io/SquireBot/
6. **End-to-end validation** — chained live smokes v0.1.0 → v0.4.0 + DOC-02 fake-guildie eviction smoke (all 7 checkpoints PASS) + 297/297 vitest tests at code-complete

**Effective requirements coverage:** 69 / 69 (66 satisfied + 2 partial via user-authorized scope reductions + 1 waived by user). See archive for full reconciliation.

**Status:** SHIPPED. Tech-debt acknowledged in audit (REQUIREMENTS.md drift, Phases 3+4 SUMMARY.md gaps, no VERIFICATION.md workflow). Cleanup carried into archive; not blocking.

**Deferred to v1.0.1 / v2:** Self-service eviction, `_meta.guild_admins` allowlist, polished theme picker tile UI, sidebar inline-JS unit tests, installer-driven upgrade UX (NSIS overwrite-running shim), PropertiesService recent-query persistence, SignPath OSS approval (in flight), v2 Wantlist + Discord pinger (WANT-01..08).

---

## v1.0.1 — Installer + Permissions Hardening

**Shipped:** 2026-05-12
**Tag:** `v1.0.1` (binary release pushed 2026-05-11 by Phase 6 ship gate)
**Archive:** [`milestones/v1.0.1-ROADMAP.md`](milestones/v1.0.1-ROADMAP.md) · [`milestones/v1.0.1-REQUIREMENTS.md`](milestones/v1.0.1-REQUIREMENTS.md) · _no audit (skipped per yolo-mode close; phase verifier already PASSED 5/5 must-haves on Phase 8 + 5/5 hooks on Phase 7 dev-workbook smoke)_

**Stats:** 3 phases · 12 plans · 63 commits · 2 calendar days kickoff to ship (2026-05-11 → 2026-05-12) · +318 LOC Go watcher (14,507 total) · +~2k LOC TypeScript (13,266 total) · +39 vitest tests (297 → 336)

**Phases:** All shipped sequentially.

| Phase | Name | Shipped As | Date |
|-------|------|-----------|------|
| 6 | Installer Overwrite-Running Shim | Watcher binary `v1.0.1` (GitHub Release) | 2026-05-11 |
| 7 | Admin Allowlist + Eviction Enforcement | `clasp push` to dev workbook (5/5 hooks PASS) | 2026-05-12 |
| 8 | Test Infra + Persistence + Docs Backfill | `clasp push` + 336/336 vitest green | 2026-05-12 |

**Key accomplishments:**

1. **Installer upgrade path hardened** — NSIS pre-install shim signals the running watcher to exit gracefully (new `internal/system` Windows named-event IPC package), polls for process exit up to 10s, falls back to `taskkill /F`. Manual stop workaround retired from `docs/troubleshooting.md`. Shipped + UAT-verified as tag `v1.0.1` on Azure VM (both `--quit` and legacy paths exercised against real running watchers).
2. **Officer-only eviction enforced by code** — `_meta.guild_admins` allowlist + `_meta.workbook_owner_floor` + `_meta.admin_log` extend-only `_meta` rows; eviction sidebar opener + all 3 `google.script.run` callbacks gated by `requireAdminOrThrow` as first statement. Non-admins see "Not authorized" modal + zero writes (verified live in dev-workbook).
3. **Admin-management UX with owner-floor lockout protection** — `showAdminMgmtSidebar` HtmlService sidebar; workbook owner cannot be removed by anyone other than themselves (server-side `'owner_floor_protected'` throw + client-side Remove-button suppression + visual `(owner)` annotation).
4. **Sidebar inline-JS test infrastructure** — JSDOM in vitest at top level; `mountSidebar()` helper with indirect-eval + nested-`<script>` extraction; 4 net-new sidebar inline-JS test files (Search, Eviction, Bank-Coin, Char-Info) → 5/5 shipping sidebars covered (Admin-Mgmt via Phase 7's trigger-call test).
5. **Recent-search MRU survives CacheService TTL** — `searchIndex.ts` MRU migrated from `CacheService.getDocumentCache()` (25-min, document-scoped) to `PropertiesService.getUserProperties()` (durable per-user); confirmed by new `vi.useFakeTimers()` 25-min persistence test.
6. **v1.0 documentation debt retired** — 8 retroactive Phase 3+4 SUMMARY.md files following the Phase 5 template byte-for-byte; every `metrics.commits` SHA resolves under `git cat-file -e` (0 invented hashes).
7. **Latent v1.0 OAuth-scope bug retired** — `apps-script/appsscript.json` was missing `userinfo.email`; under consumer @gmail.com `Session.getEffectiveUser().getEmail()` silently returned empty. Fix in commit `544bef8`. Side effect: closes the v1.0-era silent `initiated_by='unknown'` audit-log fallback for all post-Phase-7 deploys.

**Requirements coverage:** 8 / 8 (all complete; zero partials; zero waivers; zero orphans). See archive for full reconciliation.

**Status:** SHIPPED. Schema gates untouched throughout (`_meta.schema_version=3`, `WatcherMaxSchemaVersion=3`). Code review: 0 critical, 4 warning, 6 info (test-quality advisory only; none blocking).

**Deferred to v1.0.2 / v1.1 / v2:** 4 v1.0.2 candidates surfaced during Phase 6 UAT (999.13–999.16: Reauthorize on boot-time `invalid_grant`, tray `OnReady` queuing, UTF-8 BOM stripping in config loader, console-detach or `Start-Process` documentation); Admin-Mgmt sidebar inline-JS test coverage (999.17); Phase 8 advisory test-quality findings (999.18); SignPath OSS approval still in flight (999.9); v1.1 polish (bank-coin permission lock 999.1, theme picker tile UI 999.2, `SIDEBAR_BODY` extraction 999.7); v2 Wantlist + Discord pinger (999.12 / WANT-01..08).

> Note: v1.0.2 (Robustness Polish, Phases 9–10, binary tag `v1.0.2` 2026-05-13) shipped as a binary but its milestone close was **superseded by the v2.0 "Off Google" pivot** (the Sheet it targeted was being replaced). It was never written up as a standalone MILESTONES entry; its 6 robustness requirements are reconciled in the v2.0 archive's Validated block.

---

## v2.0 — "Off Google" — Website Frontend

**Shipped:** 2026-05-31
**Tag:** `v2.0.0` (the clean Google-free watcher binary; published 2026-05-31 by the Phase 16 cutover)
**Archive:** [`milestones/v2.0-ROADMAP.md`](milestones/v2.0-ROADMAP.md) · [`milestones/v2.0-REQUIREMENTS.md`](milestones/v2.0-REQUIREMENTS.md) · _no audit (closed via `/gsd-complete-milestone`; every phase verifier PASSED, the cutover was validated end-to-end live, and the 16-REVIEW code review flagged no Critical/High — MD-01/LR-01 fixed post-close)_

**Stats:** 6 phases · 29 plans · 4 days kickoff to ship (2026-05-28 → 2026-05-31) · watcher binary 57% smaller (16.44 MB → 7.07 MB, Google-free) · backend a static linux/amd64 ELF on a Hetzner VPS · SQLite schema at `goose` migration `00004` · the old apps-script suite (336 vitest) retired with the Sheet; web tests node-only (200 vitest at close)

**Phases:** All shipped sequentially.

| Phase | Name | Shipped As | Date |
|-------|------|-----------|------|
| 11 | Backend Foundation + Ingest API | LIVE at `api.squirebot.quest` (Hetzner VPS) | 2026-05-29 |
| 12 | Enrichment Job Migration | in-process scheduled jobs (deployed in the bundled redeploy) | 2026-05-29 |
| 13 | Watcher Re-Target + Onboarding | Google-free watcher (built; shipped as `v2.0.0` at cutover) | 2026-05-30 |
| 14 | Web Frontend | LIVE at `squirebot.quest` (apex on Caddy) | 2026-05-30 |
| 15 | Admin Web Forms + Login | Discord login + officer forms (deployed live) | 2026-05-31 |
| 16 | Cutover + Decommission | `v2.0.0` published + Google decommissioned | 2026-05-31 |

**Key accomplishments:**

1. **Self-hosted Go + SQLite backend, live.** A single Go binary on a Hetzner Cloud VPS (US, amd64) behind Caddy auto-HTTPS, systemd `Restart=always`, a `goose`-migrated SQLite schema, per-guildie hashed bearer-token auth, and the atomic-replace ingest endpoint — with a nightly Cloudflare R2 off-box backup + a drilled restore. Live at `api.squirebot.quest`.
2. **Enrichment migrated to in-process scheduled jobs.** Daily PigParse + weekly P1999 wiki run as db-backed in-process jobs (job_run cursor, immediate-check-on-startup, per-job mutex); the 4 pure parsers + `politeFetch` were byte-parity-ported from Apps Script (SHA-1 byte-identical; exact-count parity cross-checked against the TS parsers in Node).
3. **Watcher re-targeted fully off Google.** Swapped the Sheets client for an `internal/backend` HTTP client, DELETED ~8k LOC of Google OAuth/PKCE/Sheets/Drive-Picker code (41 files), shed the entire Google dependency tree, and shipped native "paste your guild code" onboarding via the existing auto-updater. The binary is 57% smaller with zero Google secret.
4. **Read UI rebuilt as a static SvelteKit site.** The 4 views render as one reusable filterable/sortable `DataGrid` (instantiated 4×, never per-character) over a versioned Go read API, with cross-character fuzzy search + "did you mean?", rich HTML item tooltips (XSS-escaped), and a 5-theme EQ aesthetic. Live at `squirebot.quest`.
5. **Discord login + officer web forms.** Discord OAuth2 login gated on guild Discord membership (capturing per-user Discord identity — pre-paying a v2 prerequisite) + eviction / bank-coin / admin-management as authenticated web forms porting v1 enforcement (owner-floor, 30-day grace), with authorize-under-transaction + an `audit_log`.
6. **Cutover + decommission — "Off Google" goal met.** Published the clean `v2.0.0` release, minted 11 per-guildie codes, flipped the guild via auto-update + Discord herding, and decommissioned Google (all 10 Apps Script triggers + the OAuth client deleted; Sheet abandoned in place) — proven by a committed decommission checklist.

**Requirements coverage:** 26 / 26 (all complete; zero partials; zero waivers; zero orphans). CUTOVER-01..04 were reframed by 16-CONTEXT (fresh-start char-meta form, no Sheet backfill; abandon-Sheet-in-place) and satisfied. See archive for full reconciliation.

**Status:** SHIPPED. The "Off Google" goal is MET — no Google dependency remains anywhere in the system. The guild migration is operational (3/11 reporting in at close, climbing).

Known deferred items at close: deferred HUMAN-UAT smokes for P12/P14/P15 (see STATE.md Deferred Items); 0 pending todos.

**Deferred / carried forward:** 999.31 self-service Discord watcher-linking (⭐ top next-milestone candidate); 999.22 prerelease-stuck-watcher manual-reinstall caveat; 999.9 SignPath OSS (still in flight); 999.12 / WANT-01..08 v2 Wantlist + Discord pinger (AUTH-09 pre-paid the per-user Discord-identity prerequisite). The Sheet-side backlog items (999.1/2/7/24/25/26/27/29) are mooted by the decommission.

---

## v2.1 — Self-Service Watcher Linking

**Shipped:** 2026-06-02
**Tag:** `v2.1`
**Archive:** [`milestones/v2.1-ROADMAP.md`](milestones/v2.1-ROADMAP.md) · [`milestones/v2.1-REQUIREMENTS.md`](milestones/v2.1-REQUIREMENTS.md)

**Stats:** 2 phases · 4 plans · 20 commits since `v2.0` · 35 files changed (+3,991 / −333) · 2-day kickoff to ship (2026-06-01 → 2026-06-02)

| Phase | Name | Outcome | Date |
|-------|------|---------|------|
| 17 | Self-Service Watcher Linking (web feature) | Deployed live; 15/15 verified; browser-smoke approved | 2026-06-02 |
| 18 | Watcher Cleanups — Verify-or-Close | All 3 fixes confirmed live (zero new code); stuck-watcher residual debunked | 2026-06-02 |

**Key accomplishments:**

1. **Self-service watcher linking live** — guildies mint/list/revoke their own codes at `squirebot.quest/account` via Discord login; owner derived server-side from the session (no maintainer hand-minting).
2. **Identity unification (LINK-02)** — minted codes tie to the Discord `web_user`; eviction owner-floor now resolves via `owner.discord_user_id` FK instead of the loose label/username string match.
3. **Additive multi-PC tokens + per-token revoke** — run watchers on multiple PCs; revoke one without affecting the others.
4. **Show-once credential UX** — plaintext revealed exactly once with copy-to-clipboard; never persisted/re-fetched/logged (verified live).
5. **`mint-code` CLI retired** — self-service is the only mint path; no more plaintext through the maintainer's hands.
6. **Watcher cleanups verified-closed** — gofmt / freeConsole / SemVer-`IsNewer` confirmed live; the long-carried "stuck maintainer watcher" residual found to be a disposable Azure test VM, not a production watcher.

**Effective requirements coverage:** 9 / 9 (LINK-01..06 + WATCH-12/13/14) — all Done.

**Status:** SHIPPED. Code review 0-critical; 4 advisory warnings fixed + redeployed same milestone. Phase 18 confirmed verify-or-close work needs *verification against live data*, not trust in carried-forward notes (the stuck-watcher premise was stale).

**Deferred / carried forward:** Officer mint-on-behalf; per-code device-naming UX; 999.5 self-service eviction; 999.12 / WANT-01..08 (Wantlist + Discord pinger); 999.9 SignPath OSS (in flight). Ops follow-up: decommission the Azure PAYG `0.4.0-rc1` test VM to stop billing.

---

## v2.3 — Character Assignment & Per-Character Wantlists

**Shipped:** 2026-06-09
**Archive:** [`milestones/v2.3-ROADMAP.md`](milestones/v2.3-ROADMAP.md) · [`milestones/v2.3-REQUIREMENTS.md`](milestones/v2.3-REQUIREMENTS.md) · [`milestones/v2.3-MILESTONE-AUDIT.md`](milestones/v2.3-MILESTONE-AUDIT.md)

**Stats:** 3 phases · 7 plans (26: 3, 27: 1, 28: 3) · 2-day timeline (2026-06-08 → 2026-06-09) · 14/14 requirements satisfied (ASSIGN-01..06, MYVIEW-01/02, CWANT-01..06) · schema **v9** (`00009`, P26) + **v10** (`00010`, P28) · deployed live to `squirebot.quest` · backend + web only (the Go **watcher is UNTOUCHED**)

| Phase | Name | Outcome | Date |
|-------|------|---------|------|
| 26 | Character Assignment | Live, schema v9 (`00009`); browser-smoke PASS | 2026-06-08 |
| 27 | My-Characters Inventory Filter | Live (web-only); browser-smoke PASS | 2026-06-08 |
| 28 | Character-Tagged Wantlist | Live, schema v10 (`00010`); browser-smoke PASS | 2026-06-09 |

**Key accomplishments:**

1. **Character assignment, live** — members self-claim/release the characters they play (including ones uploaded under an unlinked/legacy owner); officers assign/reassign/remove + approve/deny requests + designate bank/bot characters from the admin panel. Backed by `00009` (schema v9): `character_assignment` (single-assignee PK), `assignment_request`, `character.is_guild_bot`, idempotent auto-seed; every change audited.
2. **"My characters" inventory filter** — an additive client-side quick-filter + single-character drill-down over the existing all-members consolidated views, with all-members visibility preserved (consolidated-views architecture rule LOCKED and intact — zero backend change).
3. **Character-tagged wantlist** — wants gain an optional character dimension (`00010` → schema v10; nullable `character_id` with COALESCE dedup, existing wants backfill to NULL with no data loss); tagged wants roll up into the guildwide wantlist with per-want character + owner attribution.
4. **EC embed names the character, owner-DM invariant preserved** — the EC-tunnel monitor DM still targets the want's OWNER (`discord_user_id`); the P28 LEFT JOIN supplies only a display-only "For <char>" embed field.
5. **IDOR-guarded tagging** — `AddWantHandler`'s `IsCharAssignedToTx` reads P26's `character_assignment` table; a forged tag (tagging a character not yours) returns 403; the tag `<select>` only ever offers the caller's own characters.
6. **999.33 officer-reversible-designation fix** — surfaced a "Designated characters / Clear designation" section in the officer panel (`ListDesignatedChars` read + officer-only `GET /api/v1/admin/characters/designated`) so a bank/bot character can be returned to `mode:none` from the UI; closed the original one-way-door UI reachability gap. Resolved + deployed live 2026-06-09 (quick `260609-d2o`).

**Requirements coverage:** 14/14 (ASSIGN-01..06 → P26 · MYVIEW-01/02 → P27 · CWANT-01..06 → P28) — all satisfied + UAT-verified.

**Status:** SHIPPED. Milestone audit PASSED 2026-06-09 (14/14 requirements · 3/3 phases · 6/6 integration · 3/3 flows). Cross-phase integration verdict CLEAN.

**Known deferred items:** 999.34 cosmetic LOWs/NITs (forged-tag generic error, account-level `"character_id":null` audit noise, naming/comment nits) + 2 account-specific UATs (Phase 27 zero-claimed-characters hint, Phase 26 non-officer `/admin` 403 collapse) + the organic EC-embed confirmation — none affect correctness, security, or data (see ROADMAP backlog).

---

*This file accumulates one entry per shipped milestone. Next entry: the post-v2.3 milestone (next milestone undefined; v2.2 Track 2 — the Discord pinger WTS/raid monitors, Phases 22–23 — remains parked on the 3 Raid Alliance bot invites) — start via `/gsd-new-milestone`.*
