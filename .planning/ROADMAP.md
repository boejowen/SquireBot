# Roadmap: SquireBot

**Core Value:** Every guildie can answer "what does my character still need, and where in the guild is it?" — now delivered via a self-hosted website instead of the Google Sheet.

## Milestones

- ✅ **v1.0** — Watcher + Workbook + Onboarding (initial release) — shipped 2026-05-11 as tag `v1.0.0`
- ✅ **v1.0.1** — Installer + Permissions Hardening — shipped 2026-05-12 (binary tag `v1.0.1`)
- ✅ **v1.0.2** — Robustness Polish — binary shipped 2026-05-13 (tag `v1.0.2`); milestone close superseded by v2.0
- ✅ **v2.0** — "Off Google" — Website Frontend — Phases 11–16 (shipped 2026-05-31 as tag `v2.0.0`) — archive: [`milestones/v2.0-ROADMAP.md`](milestones/v2.0-ROADMAP.md)
- ✅ **v2.1** — Self-Service Watcher Linking — Phases 17–18 (shipped 2026-06-02 as tag `v2.1`) — archive: [`milestones/v2.1-ROADMAP.md`](milestones/v2.1-ROADMAP.md)
- 🔄 **v2.2** — Wantlist + Discord Pinger — Phases 19–25 (**Track 1 SHIPPED LIVE 2026-06-06** — Phases 19–21 deployed to api.squirebot.quest; Phases 24–25 quality/platform shipped; **Track 2 (Phases 22–23) PARKED — invite-gated** on the 3 Raid Alliance bot invites, not abandoned; milestone held open, **no tag** until Track 2 lands)
- ✅ **v2.3** — Character Assignment & Per-Character Wantlists — Phases 26–28 (shipped 2026-06-09) — archive: [`milestones/v2.3-ROADMAP.md`](milestones/v2.3-ROADMAP.md)
- ✅ **v2.4** — Web UI Revamp (5-Tab Restructure) — Phases 29–34 (shipped 2026-06-21; backend/data parse + web; watcher untouched; no tag) — reorganized squirebot.quest around five top tabs (Characters · Inventory · Banks · Wishlist · Settings)
- ✅ **v2.5** — Ownership Cleanup — Phases 35–36 (shipped 2026-06-22; owner-less guild banks/bots + shared-character-safe eviction; backend+web, no `v*` tag) — OWN-01..04 — archive: [`milestones/v2.5-ROADMAP.md`](milestones/v2.5-ROADMAP.md)
- 🔁 **v2.6** — Item Detail & Polish — Phases 37–42 (opened 2026-06-24; item enrichment backbone — flag/effect parse + catalog-wide coverage + icon backfill — faceted Clicky/Haste search w/ holdings↔catalog toggle, flag-coded outlines + named quest links on modern tabs, paper-doll compaction + per-character photo, wishlist compaction + sub-Velious tiers; backend + web, watcher untouched, no `v*` tag) — ENRICH-12..15 / SEARCH-04..06 / ITEMUI-01..02 / CHARUI-01..02 / WISHUI-01..02

## Phases

### 🔁 v2.6 — Item Detail & Polish (Phases 37–42)

**Milestone Goal:** Turn the item data SquireBot already half-captures into usable intelligence — faceted item search (Clicky / Haste), flag-coded outlines, named quest links on the modern tabs, broader icon/tier coverage — and tighten the Characters (paper-doll), Wishlist, and item-display surfaces. Source spec: `Future Features.txt` (2026-06-24) — 1 bug, 4 tweaks, 1 feature.

**Scope:** backend (`internal/backendsrv`) + web (`web/`). The Go **watcher is UNTOUCHED** — ingest already captures everything needed; this milestone re-enables a dropped wiki parse, widens enrichment coverage, and surfaces the data. **No `v*` tag** (consistent with v2.3/v2.4/v2.5). Phases continue from v2.5 (last = 36) → v2.6 starts at **Phase 37**.

**The load-bearing sequencing insight (the shared backbone first):** the wiki item-page parser (`internal/backendsrv/enrich/wikiitem.go`) ALREADY computes the LORE/NO-DROP/MAGIC/TEMPORARY flags + the click-effect/haste lines but the Go port deliberately DISCARDS them (the "D-8 scope guard"); only `is_quest_item` survives, and the full stat block is one TEXT blob (`item_master.statsblock`, migration 00013). **Phase 37 re-enables that parse into discrete, queryable columns** — the shared data layer the flag outlines (P40 ITEMUI-01) and the search facets (P39 SEARCH-04/05) both read. Enrichment is also INVENTORY-DRIVEN today (the weekly wiki job only fetches a page for items some character HOLDS, via `DistinctInventoryItemIDs`); **Phase 38 widens that to the full ~4,300-row PigParse Blue catalog** — the biggest/riskiest piece, which also fixes most missing icons and powers SEARCH-06's full-catalog scope. So: P37 backbone → P38 catalog coverage → P39 faceted search → then the independent display/character/wishlist polish phases (40–42). The held-items faceting (SEARCH-04/05) is sequenced to ship even if the catalog crawl (ENRICH-14) runs long.

**Locked decisions (2026-06-24):** paper-doll = BOTH (compact layout + optional per-character photo) · named quests on ALL modern tabs (plumb the already-harvested `quest_links` into `InventorySlot`/`ItemRollup`; no epic-specific model) · search facets = Clicky + Haste ONLY (not slot/stat/weapon-type); scope toggles holdings ↔ full catalog · flag → color = No-Drop red, Lore gold, Magic blue, Temporary gray · sub-Velious gear tiers INCLUDED (new Kunark/classic wiki source pages + tier values; `wiki_gear_tier` already exists — no migration).

**Phase checklist:**

- [x] **Phase 37: Item enrichment backbone — flags + effects** — re-enable the dropped wiki stat-block parse so LORE/NO-DROP/MAGIC/TEMPORARY flags + click-effect (Clicky) + Haste land in discrete, queryable columns (new migration + the enrich freshness short-circuit updated so existing rows backfill). ENRICH-12, ENRICH-13. **Foundational** — gates P39's facets + P40's outlines. ✅ **COMPLETE 2026-06-24** — verifier PASSED 8/8 (goal-backward); code-review 0 BLOCKER / 0 HIGH (MD-01 latent raw/cleaned flag-divergence fixed-forward; 4 LOW/NIT → backlog 999.41); migration `00016` (9 additive cols, schema bump), no-network statsblock backfill + freshness self-heal; `go build`/`vet`/`go test ./...` green; watcher untouched, no `v*` tag. NOT yet deployed.
- [x] **Phase 38: Catalog-wide enrichment + icon coverage** — widen enrichment beyond held items to the full PigParse Blue catalog (politefetch-paced) and backfill icons for every item whose wiki page provides one, with a maintainer-visible coverage diagnostic for the residue. ENRICH-14, ENRICH-15. The "missing item images" bug + the full-catalog data the SEARCH-06 toggle reads. ✅ **RE-PLAN EXECUTED + VERIFIED 2026-06-25 (name-keyed; D-04 reversed id-keyed → NAME-KEYED).** First build (id-keyed Option A) was held pre-deploy when the prod probe found it drops 43 catalog items (60 raw id-collisions); re-planned + re-executed name-keyed: migration **00017** `catalog_enrichment(norm_name PK,…)`, `item_master` held-only by EQ id (held-reader blast radius re-verified ZERO), covers all 4,343 with NO drop. 2 plans (38-01 store/migration + itemids rework; 38-02 the `wiki.go` `ref.Held` write-branch + both-stores `CatalogIconCoverage`); verifier **4/4 + 2/2** (human_needed only for the post-deploy seed-crawl/browser observables); code-review 0 BLOCKER (WR-01/WR-02 name-keying bug + IN-01 fixed-forward `9735f59`/`c7dae81`; IN-02/IN-03 → backlog 999.43; IN-04 no-action). Full `go test ./...` green; watcher untouched, no `v*` tag. ✅ **DEPLOYED LIVE 2026-06-25** (backend-only binary swap → goose applied 00016 + 00017 on boot → prod schema v15 → v17; service active, apex 200 / `/api/v1/*` 401; `catalog_enrichment` seeds on the next Sunday wiki crawl; P37 flag backfill ran live 16/953). Next: `/gsd-plan-phase 39`.
- [x] **Phase 39: Faceted item search (Clicky / Haste + scope toggle)** — filter item search to Clicky-only or Haste-only, and toggle scope between guild holdings and the full P99 catalog. SEARCH-04, SEARCH-05, SEARCH-06. Depends on P37's parsed effects (facets) + P38's catalog coverage (full-catalog scope). (completed 2026-06-26)
- [x] **Phase 40: Item display polish — flag outlines + named quest links** — color-coded flag outlines (No-Drop red / Lore gold / Magic blue) on inventory/bank/paper-doll tiles, and the named "used in quest X" links surfaced on the modern Characters/Inventory/Wishlist tabs (not just the yes/no QUEST-ITEM badge). ITEMUI-01 (depends on P37 flags), ITEMUI-02 (independent — plumbs the already-harvested `quest_links`). ✅ **COMPLETE + DEPLOYED LIVE + browser-smoke APPROVED 2026-06-26.** 2 plans (40-01 backend flag/quest-link plumbing — `bf55fac`/`0810552`; 40-02 web — `02b563a`/`27b09bd`/`33cd982`: api.ts mirror + 3 lockstep flag tokens, pure priority resolver flags.ts [No-Drop>Lore>Magic], PaperdollSlot `::before` ring [coexists with accent hover/focus, D-03], examine flag chip + clickable `notes_link` quest links via `safeHttpUrl`, composeNotes tooltip links). Verifier **PASSED 6/6** (ITEMUI-01/02 + SC-1..4, security controls confirmed in code); code-review **0 BLOCKER/0 HIGH/0 MEDIUM** (4 Info NITs → backlog 999.45). ⚠ Browser-smoke found a **Phase-37 parser bug** (clustered statsblock flags stored as one map key → exact-lookup missed → ~95% of held flag-bearing items silently mis-flagged, only blue rings rendered) — fixed-forward `39674a8` (`wikiitem.go` `hasFlag` substring + regression test) + **migration 00018** (NULL `flags_json` → the 00016 boot backfill re-derives all 953 rows; data-only, no schema-shape change). ✅ **DEPLOYED LIVE 2026-06-26** (backend 40-01 + fix + web 40-02 together; goose applied 00018 on boot → prod schema **v17 → v18**; re-derived 941 rows: 168 no-drop, 359 lore, 535 magic); user visually confirmed red/gold/blue rings. `go test ./internal/backendsrv/...` all 17 packages green; web check 0-error/399 tests/build OK. Watcher untouched → no `v*` tag.
- [ ] **Phase 41: Character paper-doll — compaction + portrait photo** — compact the paper-doll layout toward the in-game inventory-window idiom (reclaiming the empty portrait dead space) AND add an optional per-character portrait photo upload shown in the portrait area. CHARUI-01, CHARUI-02 (new per-character image reference + migration + upload path + read-API). **Plans:** 2 plans (planned 2026-07-15) — 41-01 backend (migration 00019 character_portrait side table + store/handlers/serve + assignee-or-officer gate + magic-byte sniff + 256KB cap + has_portrait/portrait_updated_at on the inventory payload + 3 routes) / 41-02 web (paper-doll compaction + portrait render + upload/remove control + api.ts, autonomous:false browser-smoke).
- [ ] **Phase 42: Wishlist polish — compaction + sub-Velious tiers** — compact the Wishlist tab for density/visual consistency with the other tabs, and extend suggestions to sub-Velious (Kunark / classic) gear tiers. WISHUI-01, WISHUI-02 (new wiki source pages + tier values; `wiki_gear_tier` already exists — no migration).

**Execution order:** dependency chain **37 → 38 → 39**, then **40 · 41 · 42** (independent of each other once 37/38 land; 40's outlines need 37's flags, 40's quest links are independent). 37 (parse backbone) gates 39's facets + 40's outlines; 38 (catalog coverage) gates 39's full-catalog scope. Sequence the held-items faceting (SEARCH-04/05) to land even if the ENRICH-14 catalog crawl runs long.

### v2.5 — Ownership Cleanup (Phases 35–36) — ✅ SHIPPED + ARCHIVED 2026-06-22

Full archive: [`milestones/v2.5-ROADMAP.md`](milestones/v2.5-ROADMAP.md) · [`milestones/v2.5-REQUIREMENTS.md`](milestones/v2.5-REQUIREMENTS.md). Promoted backlog 999.35 + 999.36 (deferred from quick `260621-u6j`). One schema migration (00015, Phase 35); watcher untouched → no `v*` tag. Both phases DEPLOYED LIVE (prod schema v14→v15 on the Phase-36 deploy boot); officer browser-smoke approved. (Detail below retained inline; fully collapsed copy in the archive.)

- [x] **Phase 35: Owner-less guild banks & bots** — OWN-01, OWN-02, OWN-04 — ✅ COMPLETE 2026-06-22 (verifier PASSED 6/6 must-haves + 3/3 req IDs; code-review found 1 BLOCKER [CR-01: sentinel guarded in the picker reads but NOT the destructive evict write path] fixed-forward `11ebcc6` + WR-01/IN-01 test hardening `62acc78` + WR-02 doc `96ea4ad`; IN-02 label-bridge collision deferred to backlog; schema v15; `go test ./internal/backendsrv/...` all 18 packages green; watcher untouched, no `v*` tag)
  - **Goal:** a designated bank/bot is guild-held, not tied to whoever uploaded it first. **RESOLVED: Option A — a reserved "guild" sentinel owner row (id 1000000, label `guild`), NOT nullable `owner_id`** (smallest blast radius — `character.owner_id` stays `NOT NULL`, every existing `owner(id)` join keeps working; only 2 production consumers — binding.go + eviction.go).
  - **Success criteria:** (1) an officer designates any char as bank/bot without "claiming"/owning it (DesignateCharTx repoints `owner_id` to the sentinel, gated only by the officer re-check); (2) a designated bank/bot has no individual owner (`owner_id` = sentinel); (3) existing owner-bound banks (e.g. Findom) migrate automatically (00015 backfill, no manual fixup); (4) `go test ./...` green; watcher untouched.
  - **Plans:** 1 plan (complete)
    - [x] 35-01-PLAN.md — migration 00015 (seed guild sentinel owner + backfill existing bank/bot chars) + store/owner.go GuildSentinelOwnerID + DesignateCharTx owner_id repoint + ListEvictableOwners/ListRestorableOwners sentinel exclusion + the OWN-02 survives-eviction proof + EvictOwnerTx/RestoreOwnerTx sentinel write-path guard (CR-01 fix) (OWN-01/02/04) — EXECUTED 2026-06-22 (7a238b8/c8305c2/4d38389 + fixes 11ebcc6/62acc78/96ea4ad); schema v15; build rc=0, vet clean, all backendsrv tests green; watcher untouched, no `v*` tag
- [x] **Phase 36: Shared-character-safe eviction** — OWN-03 (depends on Phase 35) — ✅ COMPLETE + DEPLOYED LIVE 2026-06-22 (verifier PASSED 13/13 must-haves; plan-checker caught 1 BLOCKER [narrowed preview emptied the Evict button for an all-shared owner → can't revoke their code] → user RATIFIED expanding scope backend→web; code-review 0 BLOCKER [2 non-blocking WARNINGs backlogged]; officer browser-smoke APPROVED across themes; deployed via backend binary swap + web atomic swap, R2 backup, no goose [00015 already shipped]; `go test ./...` + web 369 green; watcher untouched, no `v*` tag)
  - **Goal:** eviction removes only the evicted member's own characters — never shared characters or guild banks/bots.
  - **Success criteria:** (1) evicting a member preserves shared characters other guildies play; (2) guild banks/bots are never removed by an eviction; (3) the officer eviction-preview reflects the narrowed set; (4) `go test ./...` green.
  - **Plans:** 2 plans (backend + web; the web change depends on the backend preview contract — wave 2) — both complete + live
    - [x] 36-01-PLAN.md (wave 1, backend) — narrowed EvictOwnerTx cascade + parity-locked PreviewEviction via one shared `cross_owner_write` predicate + additive `preserved_shared_count` + drift-locked steward repoint — EXECUTED 2026-06-22 (652f6d2/6969080/a1d0861); no migration (schema v15)
    - [x] 36-02-PLAN.md (wave 2, web) — mirror `preserved_shared_count` in api.ts + keep all-shared owners evictable in EvictionForm.svelte (code-only-revoke framing) via pure node-tested helpers — EXECUTED + DEPLOYED 2026-06-22 (b1aab94/8c77086); browser-smoke APPROVED
    - [x] 36-01-PLAN.md (wave 1) ✅ EXECUTED 2026-06-22 (`652f6d2` + `6969080` + `a1d0861`, backend-only) — narrowed EvictOwnerTx cascade + PreviewEviction via the single shared `sharedCharPredicate` `cross_owner_write` const (shared chars survive; preview==cascade parity; surviving-shared repoint via the drift-locked `recentOtherSharerSubquery` const) + the additive `preserved_shared_count` preview field + `CountPreservedShared` — OWN-03; no migration (schema v15); watcher + web untouched; no `v*` tag. Gates green (build/vet/full backendsrv test suite). See `36-01-SUMMARY.md`.
    - [~] 36-02-PLAN.md (wave 2, depends_on 36-01) — ✅ CODE-COMPLETE 2026-06-22 (`b1aab94` + `8c77086`, web-only): mirrored `preserved_shared_count` in api.ts + pure node-tested `canEvictPreview`/`evictPreviewSummary` helpers (6 cases) + wired EvictionForm so an all-shared owner stays evictable with the "0 characters removed; {N} shared character(s) preserved; guild code will be revoked" code-only-revoke framing (genuine zero-live-chars stays disabled). Web gates green (check 0/0, 369 tests, build ok); no `{@html}` sink; no `v*` tag; watcher untouched. **DEPLOYED LIVE + officer browser-smoke APPROVED 2026-06-22** (backend binary swap + web atomic-swap, no goose; canonical 36-02 entry above — this duplicate retained from execution history). See `36-02-SUMMARY.md`. — OWN-03

<details>
<summary>✅ v1.0 — Watcher + Workbook + Onboarding (Phases 1–5) — SHIPPED 2026-05-11</summary>

Full details in [`milestones/v1.0-ROADMAP.md`](milestones/v1.0-ROADMAP.md).

- [x] Phase 1: End-to-End Thin Slice (8 plans) — shipped v0.1.0, 2026-05-02
- [x] Phase 2: Watcher Robustness + Schema Lock (10 plans) — shipped v0.2.0 + v0.2.1 hotfix, 2026-05-09
- [x] Phase 3: Apps Script Enrichment Foundation (4 plans) — shipped v0.3.0, 2026-05-10
- [x] Phase 4: Differentiator Features (4 plans) — shipped v0.4.0, 2026-05-11
- [x] Phase 5: Search + Onboarding + Privacy Polish (5 plans) — shipped v1.0.0, 2026-05-11

**Total:** 31 plans · 5 phases · 11 days kickoff to ship · 203 commits.

</details>

<details>
<summary>✅ v1.0.1 — Installer + Permissions Hardening (Phases 6–8) — SHIPPED 2026-05-12</summary>

Full details in [`milestones/v1.0.1-ROADMAP.md`](milestones/v1.0.1-ROADMAP.md).

- [x] Phase 6: Installer Overwrite-Running Shim (5 plans) — shipped + UAT-verified as tag `v1.0.1`, 2026-05-11
- [x] Phase 7: Admin Allowlist + Eviction Enforcement (3 plans) — shipped via dev-workbook 5-hook smoke, 2026-05-12
- [x] Phase 8: Test Infra + Persistence + Docs Backfill (4 plans) — shipped (5/5 must-haves; 336/336 vitest), 2026-05-12

**Total:** 12 plans · 3 phases · 2 days kickoff to ship · 63 commits since v1.0.0.

</details>

<details>
<summary>✅ v1.0.2 — Robustness Polish (Phases 9–10) — binary shipped 2026-05-13; milestone close superseded by v2.0</summary>

Binary `v1.0.2` shipped 2026-05-13; its milestone close was superseded by the v2.0 "Off Google" pivot (the Sheet it targeted is being replaced). Phase directories 09 and 10 exist on disk; the next milestone continues at Phase 11 (never reuses 9 or 10).

- [x] Phase 9: Watcher Robustness Polish (5 plans) — AUTH-07, OPS-06, OPS-07, CONFIG-01; shipped as watcher v1.0.2 binary, 2026-05-13. HUMAN-UAT was blocked on 999.19 (Google brand verification), then superseded.
- [x] Phase 10: Apps Script Test Quality (3 plans) — TEST-03, TEST-04; shipped via `clasp push` + green CI, 2026-05-13.

</details>

<details>
<summary>✅ v2.0 — “Off Google” — Website Frontend (Phases 11–16) — SHIPPED 2026-05-31</summary>

Full details in [`milestones/v2.0-ROADMAP.md`](milestones/v2.0-ROADMAP.md).

**Milestone Goal:** Replace the shared Google Sheet (both the UI *and* the data store) with a self-hosted Go + SQLite backend and a static web frontend — permanently eliminating the Google OAuth dependency that blocked the guild after Google's 2026-05-15 brand-verification rejection. Goal MET: backend + website live, Google fully decommissioned.

- [x] Phase 11: Backend Foundation + Ingest API (7 plans) — Hetzner Cloud VPS (US) + Caddy auto-HTTPS live at api.squirebot.quest; SQLite + `goose`; bearer-token auth; ingest endpoint; nightly R2 backup. ✅ 2026-05-29
- [x] Phase 12: Enrichment Job Migration (5 plans) — daily PigParse + weekly P1999 wiki as in-process scheduled jobs; 4 parsers + politeFetch byte-parity-ported from Apps Script. ✅ 2026-05-29
- [x] Phase 13: Watcher Re-Target + Onboarding (4 plans) — watcher off Google (~8k LOC deleted, binary 57% smaller, no Google secret); native “paste your guild code” onboarding via the auto-updater. ✅ 2026-05-30
- [x] Phase 14: Web Frontend (4 plans) — static SvelteKit site (4 views as one reusable filterable/sortable DataGrid, fuzzy search + “did you mean?”, rich HTML tooltips, 5-theme EQ aesthetic) over a versioned Go read API; live at squirebot.quest. ✅ 2026-05-30
- [x] Phase 15: Admin Web Forms + Login (5 plans) — Discord OAuth2 login (guild-membership-gated, capturing per-user Discord identity) + officer web forms (eviction, bank-coin, admin management) porting v1 enforcement. ✅ 2026-05-31
- [x] Phase 16: Cutover + Decommission (4 plans) — published the clean `v2.0.0` release, minted 11 per-guildie codes, flipped the guild via auto-update, decommissioned Google (triggers + OAuth client retired; Sheet abandoned in place). ✅ 2026-05-31

**Total:** 29 plans · 6 phases · 4 days kickoff to ship (2026-05-28 → 2026-05-31). The cutover was REFRAMED by 16-CONTEXT: fresh-start char-meta form (no Sheet backfill) + abandon-Sheet-in-place. All 26 v2.0 requirements shipped.

</details>

<details>
<summary>✅ v2.1 — Self-Service Watcher Linking (Phases 17–18) — SHIPPED 2026-06-02</summary>

Full details in [`milestones/v2.1-ROADMAP.md`](milestones/v2.1-ROADMAP.md).

**Milestone Goal:** Let any guildie link their own watcher from squirebot.quest via the existing P15 Discord login — no maintainer hand-minting + DMing codes — while keeping the watcher credential a static, reusable bearer token (the v2.0 model; **no watcher change**). Goal MET.

- [x] Phase 17: Self-Service Watcher Linking (web feature) (3 plans) — `/account` self-mint (owner from Discord session) + Discord↔owner FK linkage + additive codes + per-token revoke + show-once copy-to-clipboard panel; `mint-code` CLI removed. Deployed live; 15/15 verified; browser-smoke approved. ✅ 2026-06-02
- [x] Phase 18: Watcher Cleanups — Verify-or-Close (1 plan) — confirmed 999.20/21/22 all live-correct with zero new code; the "stuck 0.4.0-rc1 watcher" residual was found to be the disposable Azure test VM, not a production watcher. ✅ 2026-06-02

**Total:** 4 plans · 2 phases · 2 days kickoff to ship · 20 commits since `v2.0`. All 9 requirements (LINK-01..06 + WATCH-12/13/14) shipped.

</details>

### 🔄 v2.2 — Wantlist + Discord Pinger (Phases 19–23)

**Milestone Goal:** Guildies maintain a personal wantlist on squirebot.quest and get DMed on Discord when a wanted item appears — at EC-tunnel auction (PigParse), in cross-server WTS channels, or as a raid-target tied to a wanted item's quest. (Backlog 999.12 / WANT-01..08 — the long-deferred "v2 feature," unblocked now that per-user Discord identity is paid by AUTH-09 + LINK-02.)

**Delivery/reading split (the load-bearing insight):** DM *delivery* is UNBLOCKED — the bot lives in the guild's OWN Discord, where every guildie already is via the v2.0 membership-gated login, so it can DM all of them. Only WTS/raid-channel *reading* is invite-gated on the 3 un-negotiated Raid Alliance server invites. **Track 1** (Phases 19–21: wantlist → DM/notification infra → EC monitor) ships complete, valuable value with zero dependency on the external invites. **Track 2** (Phases 22–23: WTS monitor → quest-target raid monitor) is hard-gated on the invites; a bot feature-flag + `guild_channel` rows let Track 1 ship with WTS/raid dark and flip on per-server as invites arrive — no rebuild.

**Locked decisions (v2.2 research, 2026-06-02 — see `research/SUMMARY-v2.2.md`):** in-process bot goroutine (NOT a separate process — avoids two SQLite writers), `recover()`-isolated + non-fatal start; `bwmarrin/discordgo` v0.29.0 (CGO-free, the only new dependency); one match seam (`wantmatch` + `notify` + `alert_log` dedup/cooldown) three sources fan in; EC = PigParse poll-and-diff (~10-min cadence, live spike first); error 50007 (can't-DM) first-class with an in-site notification inbox; `MESSAGE_CONTENT` a self-serve dev-portal toggle (no Discord audit under 100 servers); HARD CONSTRAINT — never put a Discord bot/OAuth in the watcher (untouched this milestone).

**Track 1 — ✅ SHIPPED LIVE** (deployed to api.squirebot.quest 2026-06-06, schema v8; Phases 19–21 complete — wantlist + EC-tunnel Discord pinger is a complete, standalone feature):

- [x] **Phase 19: Wantlist CRUD** — per-user wantlist (website CRUD, item-ID-keyed, buy/quest reason, Discord-identity-tied, already-in-bank flag); the product surface, no Discord yet (completed 2026-06-05)
- [x] **Phase 20: Bot + DM + Notification Infrastructure** — in-process discordgo gateway goroutine + `notify` DM sender + `wantmatch` + `alert_log` dedup/cooldown + opt-in/prefs + in-site notification inbox (50007 fallback) + per-monitor enable/disable + `guild_channel` config; the keystone spine all monitors ride (completed 2026-06-05)
- [x] **Phase 21: EC-Tunnel Auction Monitor** — first real alert; PigParse poll-and-diff → wantmatch → DM, gated behind an upfront PigParse feasibility spike (confirm timestamps advance + coverage) (completed 2026-06-06; **deployed live 2026-06-06**, schema v8, bot connected + `ec_auction_match` job running; WANT-05; live-DM smoke confirms organically per D-07 coverage)

**Track 2 — INVITE-GATED (entry-precondition: the 3 Raid Alliance bot invites confirmed in writing + `MESSAGE_CONTENT` enabled):**

- [ ] **Phase 22: WTS Cross-Server Monitor** — bot reads the 3 Raid Alliance WTS channels → name/alias matcher → DM; matcher built/tested against fixtures so it's testable without live servers *(invite-gated)*
- [ ] **Phase 23: Quest-Target Raid Monitor** — bot detects a raid-target NPC tied to a wanted item's quest → curated `quest → NPC` lookup → existing `quest_items` → DM *(invite-gated + needs the curated quest→NPC table)*
- [x] **Phase 24: Watcher test hardening (C1/C2 coverage)** — quality/tech-debt; close the Church audit's two CRITICAL coverage gaps (spellbook-upload path tests + `eqfind` walk tests) via a twin-handler refactor. Independent of the wantlist track — can run anytime. ✅ 2026-06-03
- [x] **Phase 25: Linux Watcher** — cross-cutting platform; a headless, fully-static (CGO-free) `linux/amd64` watcher build for guildies running P99 under WINE — Linux impls behind the existing build-tag seams (0600-file credential, WINE-prefix EQ discovery, CLI onboarding, XDG paths) + a tarball/systemd-user-unit/install.sh. Additive — Windows build unchanged. (LNX-01..06)

<details>
<summary>✅ v2.3 — Character Assignment & Per-Character Wantlists (Phases 26–28) — SHIPPED 2026-06-09</summary>

Full details in [`milestones/v2.3-ROADMAP.md`](milestones/v2.3-ROADMAP.md).

**Milestone Goal:** Associate SquireBot users with specific characters, let them view those characters' inventory, and create character-tagged wantlist items that roll up to the guildwide wantlist. Goal MET — all 3 phases shipped + deployed live; backend + web only, **watcher untouched**; migrations `00009` (P26 assignment, schema v9) + `00010` (P28 wantlist `character_id`, schema **v10**). Milestone audit PASSED 2026-06-09.

- [x] **Phase 26: Character Assignment** (3 plans) — `character_assignment` layer (`00009`, schema v9, single-assignee PK); member self-claim/release + officer assign/reassign/designate API (IDOR-safe, audited); "My characters" + officer admin panel. (ASSIGN-01..06) ✅ 2026-06-08
- [x] **Phase 27: My-Characters Inventory Filter** (1 plan) — additive "my characters" quick-filter + single-character drill-down over the all-members consolidated views; consolidated-views rule intact, zero backend change. (MYVIEW-01/02) ✅ 2026-06-08
- [x] **Phase 28: Character-Tagged Wantlist** (3 plans) — optional `character_id` on `wantlist_item` (`00010`, schema v10, NULL backfill, COALESCE dedup); tagged wants roll up to the guildwide list with owner/char attribution; EC-monitor DM still targets the owner (embed names the character). (CWANT-01..06) ✅ 2026-06-09

**Total:** 7 plans · 3 phases · 14/14 requirements satisfied. Plus carry-forward fix 999.33 (officer-reversible bank/bot designation) shipped via quick `260609-d2o` 2026-06-09.

</details>

### 🔁 v2.4 — Web UI Revamp (5-Tab Restructure) (Phases 29–34)

**Milestone Goal:** Reorganize squirebot.quest around five persistent top-level tabs — **Characters · Inventory · Banks · Wishlist · Settings** — each answering one user question, backed by the new data architecture this requires. Source spec: `Future Features.txt` (2026-06-17) + the locked sketch decisions (`.planning/sketches/MANIFEST.md`, sketches 001–004).

**Scope:** backend (`internal/backendsrv`) + web (`web/`). The Go **watcher is UNTOUCHED** — it already uploads the inventory `Location | Name | ID | Count | Slots` data; this milestone parses and surfaces it. Reworks the v2.2/v2.3 wantlist into the per-character, per-equipment-slot Wishlist; pings reuse the SHIPPED EC-monitor + notification spine (v2.2 Track 2 WTS/raid monitors stay PARKED — out of scope).

**Locked architecture decisions (2026-06-17):** consolidated-views lock **RELAXED** — per-character master-detail drill-down allowed (CLAUDE.md updated). Gear-tier/wiki items carry **no item_id**, so PigParse price + last-listed join by **normalized name**. Existing 5 EQ themes reused unchanged (no re-skin). Real item icons from the P1999 wiki (`Item_<iconId>.png`).

**The load-bearing sequencing insight:** the **data foundation comes first**. INV-05 (parse the watcher's `Location`/`Slots` into a slot taxonomy + container nesting) and DATA-01/02 (name-keyed PigParse joins + bank valuation) are foundational — the Characters inventory window (INV), the Inventory tab (ITEM), the Banks tab (BANK), and the Wishlist's equipped-slot detection (WISH) all read parsed data. So the backend parse/aggregation phase lands before any web surface that consumes it; the app shell (NAV) reframes routing for every tab and lands second; the four tab surfaces follow.

**Phase checklist:**

- [x] **Phase 29: Data Foundation — Inventory Parse + Price/Value Aggregation** — backend: parse `Location`/`Slots` into a slot taxonomy + container nesting (INV-05), name-keyed PigParse price + last-listed joins (DATA-01), bank valuation + total platinum aggregation (DATA-02). No new user surface; powers every v2.4 web tab. ✅ COMPLETE 2026-06-17 (2 plans; verifier PASSED 4/4; code-review clean after BLOCKER+HIGH fixed; `go test ./...` green; compute-on-read, no migration, watcher untouched).
- [x] **Phase 30: App Shell + 5-Tab Navigation** — the five persistent top tabs (Characters · Inventory · Banks · Wishlist · Settings) with per-tab in-context search; Settings consolidates Theme/Notifications/Watcher Codes/Set Class & Level/My Characters/Admin; notifications + unread badge move onto the Wishlist tab. (NAV-01..04) ✅ COMPLETE 2026-06-18 (2 plans; deployed live to squirebot.quest + browser-smoke 8/8 PASS; verifier PASSED 12/12; web-only, watcher/backend untouched).
- [x] **Phase 31: Characters Tab + In-Game Inventory Window** — the guild character list (viewer's first A-Z, then others, then banks/bots) + per-character search; selecting a character opens an in-game-style paperdoll inventory window with stacks, bag drill-down, bank-below, real wiki icons, and a click-to-pin right-click examine. (CHAR-01..03, INV-01..04) ✅ COMPLETE 2026-06-18 (4 plans + 8 browser-smoke fix commits; deployed live to squirebot.quest + browser-smoke approved across 5 themes; verifier PASSED 4/4 + 7/7 req IDs; code-review 0 BLOCKER/0 HIGH; web 347 tests + `go test ./...` green; shipped migrations 00012_item_icon + 00013_item_statsblock, both extend-only)
- [x] **Phase 32: Inventory Tab (Item-Centric)** — a guild-wide item list (name, guild-wide quantity, wiki + PigParse links); selecting an item shows which characters hold it, the slot on each, quantity, and last-synced (master-detail). (ITEM-01..03)
 (completed 2026-06-19)
- [x] **Phase 33: Banks Tab + Valuation** — a guild-banks-only list (each opens its inventory window), the total PigParse item value across banks + total platinum, and per-item bank search. (BANK-01..03)
 (completed 2026-06-19)
- [x] **Phase 34: Wishlist Rework — Per-Character Per-Slot Upgrades** — reworks the v2.2/v2.3 wantlist into a per-character, per-equipment-slot, open-ended upgrade list with complete Velious `_wiki_gear_tier` suggestions (price + wiki + last-listed; Raid tag for no-drop/raid-only), a Discord ping toggle + EC-hit badge (reusing the shipped EC-monitor + notification spine), the examine tooltip, and wishlist search. (WISH-01..07) (completed 2026-06-21)

**Execution order:** strict dependency chain **29 → 30 → 31 → 32 → 33 → 34**. Phase 29 (data) unblocks 31/32/33/34; Phase 30 (shell) reframes routing for all four tab phases; the Wishlist (34) additionally depends on Phase 31's equipped-slot detection + the already-shipped notification/EC spine. 32 and 33 are independent of each other once 29+30 land (could parallelize), but numbered sequentially.


## Phase Details

### Phase 19: Wantlist CRUD
**Goal**: A signed-in guildie can maintain a personal, Discord-identity-tied wantlist on squirebot.quest — add items from the existing catalog with a buy/quest reason, view and remove them, and see whether each is already in the guild bank.
**Track**: 1 (UNBLOCKED)
**Depends on**: Nothing in v2.2 (builds on the live v2.0/v2.1 platform — `webadmin/account.go` pattern, Discord-OAuth session identity, item catalog)
**Requirements**: WANT-01, WANT-02
**Success Criteria** (what must be TRUE):
  1. A signed-in guildie can add an item to their wantlist by searching the existing item catalog, tagging it buy vs quest, and optionally setting a priority and note — the entry is item-ID-keyed and tied to their Discord identity (`web_user`).
  2. A guildie can view their full wantlist on squirebot.quest and remove any entry; another guildie cannot see or mutate it (IDOR-safe, owner-scoped, audited — the `account.go` security shape).
  3. Each wantlist row shows an "already in the guild bank?" indicator joined from the existing consolidated bank/view data.
  4. The `00006_wantlist.sql` goose migration applies idempotently on the live DB, creating at least `wantlist_item` and `alert_log` without disturbing the existing schema.
**Plans**: 3 plans
  - [x] 19-01-PLAN.md — migration 00006 (wantlist_item + alert_log + dedupe indexes) + owner-scoped store CRUD + D-10 catalog search store
  - [x] 19-02-PLAN.md — webadmin add/list/remove handlers (IDOR-safe, audited, validated) + readapi item-search handler + 4 RequireSession routes
  - [x] 19-03-PLAN.md — SvelteKit /wantlist page (catalog-search add form, deep in-bank holder display, remove) + api/columns/StateBlock/nav + browser-smoke
**UI hint**: yes

### Phase 20: Bot + DM + Notification Infrastructure
**Goal**: The keystone alerting spine exists — an in-process Discord bot can DM any guildie from the guild's own server; alerts are opt-in, deduplicated/cooled-down, and recorded in an in-site inbox that serves as the can't-DM fallback; officers can enable/disable each monitor and register source channels per server.
**Track**: 1 (UNBLOCKED — DM send is pure REST on the guild's own server; needs NO privileged intent and NO external invites)
**Depends on**: Phase 19 (wantlist must exist before there is anything to match; reuses `wantlist_item` + `alert_log`)
**Requirements**: WANT-03, WANT-04, WANT-08
**Success Criteria** (what must be TRUE):
  1. The `squirebot-server` binary starts an in-process discordgo gateway goroutine behind an `Enabled` feature flag — `recover()`-isolated, non-fatal on start (the HTTP API + scheduler serve even if the bot can't connect), with reconnect managed by discordgo; a bot panic can never take down the live website/ingest.
  2. The bot can DM a guildie from the guild's own Discord; when a DM is undeliverable (error 50007), the alert is marked `dm_blocked` and surfaced in the in-site notification inbox rather than silently dropped.
  3. A guildie can opt in/out of alerts and set notification preferences; repeat matches for the same `(wantlist_item, source, item)` are suppressed within a tunable per-source cooldown window; every alert attempt is recorded in `alert_log`.
  4. An officer can enable/disable each monitor and register source channels per server (feature flags + `guild_channel` rows), so Track-1 features ship with the invite-gated monitors dark and flip on as invites arrive — no rebuild.
  5. `wantmatch` (the single shared matcher, `ForItem` + `ForName`) is exercised end-to-end via a manual/test trigger that DMs a real guildie, proving the spine all three monitors will ride.
**Plans**: 5 plans
  - [x] 20-01-PLAN.md — migration 00007 (notify_prefs + monitor_flag + guild_channel + alert_log.read_at + wantlist_item.muted) + owner/officer-scoped store layer (WANT-03/04/08)
  - [x] 20-02-PLAN.md — discordgo bot package (recover-isolated, non-fatal, env config) + notify (DM + 50007→dm_blocked + dedup/cooldown + two-gate) + wantmatch seam (WANT-03/04)
  - [x] 20-03-PLAN.md — notifications + monitors handlers + per-want mute + runServe bot wiring + 12 routes + deploy-doc token + Discord-setup checkpoint (WANT-03/04/08)
  - [x] 20-04-PLAN.md — /notifications page (prefs + inbox), Toggle primitive, unread nav badge, api.ts wrappers + browser-smoke (WANT-04)
  - [x] 20-05-PLAN.md — /admin Monitors section (kill-switches + add-channel + test-alert) + /wantlist mute bell + browser-smoke (WANT-08/03)

### Phase 21: EC-Tunnel Auction Monitor
**Goal**: The first real end-to-end alert ships — when a wanted item is auctioned in the EC tunnel, the wantlister gets a DM (price + WTS/WTB; seller best-effort), all on the guild's own Discord.
**Track**: 1 (UNBLOCKED)
**Depends on**: Phase 20 (rides the `wantmatch` + `notify` + `alert_log` spine), Phase 19 (wantlist data)
**Requirements**: WANT-05
**Success Criteria** (what must be TRUE):
  1. An upfront PigParse feasibility spike (the phase's first task/gate) confirms auction timestamps advance during a live tunnel and measures coverage — defining whether the trigger is per-auction (`getdetails`) or coarser new-sighting (`lastWTSSeen`) before the plan commits.
  2. A new `scheduler.ec_auction_match` job polls PigParse per wanted item on a ~10-min cadence, diffing on the auction-timestamp cursor (`ec_auction_cursor`), and matches on exact item ID.
  3. When a wanted item is newly auctioned, the wantlister receives a DM carrying the item, price, and WTS/WTB tag (seller resolved only when resolvable); the alert is deduped/cooled per Phase 20's policy.
  4. The cursor advances only after a successful poll and the job does not replay backlog on restart — a standing auction is not re-DMed every poll.
**Plans**: 3 plans
  - [x] 21-01-PLAN.md — feasibility spike (GATE; path=getdetails, server=0, NAME key) + getdetails parser + 00008 ec_auction_cursor migration + cursor/poll-set store (completed 2026-06-06)
  - [x] 21-02-PLAN.md — notify embed send-path (D-04, same gates/dedup/alert_log) + wantmatch Hit.note (D-05) (completed 2026-06-06)
  - [x] 21-03-PLAN.md — ec package RunMatch (poll→diff→match→embed→send) + scheduler ec_auction_match job + main.go bot/scheduler reorder (completed 2026-06-06)

### Phase 22: WTS Cross-Server Monitor
**Goal**: When a wanted item is posted for sale in a Raid Alliance WTS channel, the wantlister gets a DM — a new event source wired into the Phase 20 spine.
**Track**: 2 (INVITE-GATED)
**Entry-precondition (HARD GATE)**: the 3 Raid Alliance bot invites confirmed **in writing** (admin/read permission on the WTS channels) + `MESSAGE_CONTENT` privileged intent toggled on in the Discord dev portal. The live monitor does NOT start without the invites; the matcher is built/tested against fixtures meanwhile.
**Depends on**: Phase 20 (reuses `notify`/`alert_log`/`wantmatch`) + the external invite prerequisite
**Requirements**: WANT-06
**Success Criteria** (what must be TRUE):
  1. The name/alias matcher (`wantmatch.ForName`: exact item-ID + a curated alias table for FBSS/Fungi-class abbreviations + bounded substring, WTS-filtered) is built and unit-tested against a recorded fixture corpus of real WTS lines — testable with zero live servers.
  2. With the bot invited and `MESSAGE_CONTENT` on, a `bot/wts.go` MESSAGE_CREATE handler reads the registered WTS channels (`guild_channel` rows) and a content-non-empty smoke test confirms message text arrives (not silently empty).
  3. A WTS line for a wanted item DMs the matching guildie via the existing `notify`/`alert_log` path, deduped/cooled per Phase 20's WTS-window policy; until the invites land, the monitor stays dark behind its feature flag with Track 1 unaffected.
**Plans**: TBD

### Phase 23: Quest-Target Raid Monitor
**Goal**: When a raid-target NPC tied to a wanted item's quest is announced in a Raid Alliance channel, the wantlister gets a DM — the last event source on the spine, chaining NPC → quest → item → wantlister.
**Track**: 2 (INVITE-GATED)
**Entry-precondition (HARD GATE)**: the 3 Raid Alliance bot invites confirmed **in writing** + `MESSAGE_CONTENT` enabled (as Phase 22) AND the curated `quest → raid-target NPC(s)` lookup populated (seeded from the wiki; curation can start in parallel during Track 1).
**Depends on**: Phase 20 (reuses `notify`/`alert_log`/`wantmatch`) + the existing `quest_items` table + the external invite + curated-table prerequisites
**Requirements**: WANT-07
**Success Criteria** (what must be TRUE):
  1. A curated `quest_target` table (`quest → raid-target NPC(s)`, seeded from the wiki, scoped to items guildies actually quest-want) is populated and queryable, reusing the existing `quest_items` (item ↔ quest) table for the inverse hop.
  2. A `bot/raidtarget.go` MESSAGE_CREATE handler detects a raid-target NPC name in a registered raid-announce channel and resolves NPC → quest → wanted item(s) via `quest_target` + `quest_items`.
  3. When the resolved item is on a guildie's wantlist (reason = quest), that guildie receives a DM via the existing `notify`/`alert_log` path, deduped/cooled per Phase 20's policy; until the invites + curated table land, the monitor stays dark behind its feature flag.
**Plans**: TBD

### Phase 24: Watcher test hardening (C1/C2 coverage)
**Goal**: Close the two CRITICAL test-coverage gaps from the Church of Clean Code audit (`.planning/CLEAN-CODE-REPORT.md`) so the watcher's spellbook-upload path and EQ-folder discovery are provably correct, not assumed.
**Track**: Quality / tech-debt (cross-cutting — NOT part of the v2.2 wantlist feature theme; appended to the active milestone by sequential numbering).
**Depends on**: Nothing — touches only watcher code (`internal/app/runapp.go`, `internal/eqfind`) that is orthogonal to the v2.2 wantlist/Discord track (web/server/bot). Can run before, during, or after Phases 19–23 without conflict.
**Requirements**: Audit findings C1, C2 (+ the Size/Test twin-handler refactor). No new product requirement.
**Success Criteria** (what must be TRUE):
  1. The duplicated twin upload handlers `makeOnInventoryChange`/`makeOnSpellbookChange` (`internal/app/runapp.go:314`/`:389`) are collapsed into one shared `makeOnFileChange(kind, suffix, mtimeMap …)` helper with an extracted `handleIngestErr(...)` — removing ~50 lines of copy-paste and the verbatim error-switch (`:355-372` ≡ `:419-437`), with no behavior change to the inventory path (existing tests stay green).
  2. The spellbook upload path has behavior tests mirroring the four existing inventory tests — 401-no-loop-sets-red, 426-update-needed, cross-owner reject, empty-file-skips-no-request, and 204-persists-mtime — so a future change to one path can no longer silently rot the other.
  3. `internal/eqfind`'s real filesystem-walk discovery is tested: `walkRoot` + sentinel-matching exercised against a `t.TempDir()` tree with planted `eqgame.exe`/`eqclient.ini` at varying depths plus decoy dirs, lifting the package off its ~15% floor (orchestration-only) onto the actual discovery logic.
**Plans**: 2 plans
  - [x] 24-01-PLAN.md — refactor twin upload handlers (makeOnFileChange + handleIngestErr) + spellbook behavior tests (C1, REFACTOR)
  - [x] 24-02-PLAN.md — eqfind walkRoot/sentinel-walk tests against a t.TempDir() tree (C2)

### Phase 25: Linux Watcher
**Goal**: Guildies who run Project 1999 under WINE on Linux can install and run the watcher — a headless, fully-static (`CGO_ENABLED=0`) `linux/amd64` build that watches the WINE-prefix EQ folder, uploads `/outputfile` `.txt` files over HTTPS with the static bearer guild code, and auto-updates — with the same robustness as the Windows watcher, minus the Windows GUI/installer chrome.
**Track**: Quality / platform (cross-cutting — NOT part of the v2.2 wantlist/Discord theme; appended to the active milestone by sequential numbering).
**Depends on**: Nothing — touches only watcher code + the build/release pipeline; orthogonal to the backend/web/bot. All Linux work is additive behind `//go:build` tags / `runtime.GOOS` so the Windows build is unchanged.
**Requirements**: LNX-01, LNX-02, LNX-03, LNX-04, LNX-05, LNX-06
**Success Criteria** (what must be TRUE):
  1. The watcher cross-compiles `GOOS=linux GOARCH=amd64 CGO_ENABLED=0` to a single static binary and runs **headless** (no systray) — the tray controller is a no-op on Linux and `RunApp` is unchanged; `go test ./...` and the Windows artifact are unaffected.
  2. The bearer guild code persists in a `0600` file under `$XDG_CONFIG_HOME/squirebot/` (no keyring/secret-service dependency); config + logs follow XDG base dirs; the Windows `wincred`/`%LOCALAPPDATA%` paths are untouched.
  3. EQ-folder discovery finds the install inside a WINE prefix (`$WINEPREFIX` → `~/.wine/drive_c` → common Lutris/Proton/Bottles paths via the bounded `eqfind` walk for the `eqgame.exe`/`eqclient.ini` sentinels), falling back to a CLI prompt that persists the chosen path.
  4. First-run onboarding + control are CLI (`--setup` prompts for the guild code + EQ folder over stdin; `--status` prints health/config) — no Win32 dialog, no localhost/browser surface (the watcher stays browser-free).
  5. A `.tar.gz` ships the binary + README + a systemd **user** unit + `install.sh` (installs to `~/.local/bin`, enables the unit for autostart, runs first-time `--setup`); the existing `minio/selfupdate` auto-update works on Linux with the linux asset in the manifest.
  6. The fsnotify watch + debounce + full-snapshot-replace upload + schema-version gate + log rotation all work on Linux (they are platform-agnostic) — verified by the existing suite plus new Linux-path unit tests for credstore/eqfind/config.
**Plans**: 3 plans
  - [x] 25-01-PLAN.md — CGO-free headless build seam: tray split + main.go systray split + config/logging XDG branch (LNX-01, LNX-02) ✅ 2026-06-06
  - [x] 25-02-PLAN.md — Linux runtime impls: 0600-file credstore + WINE-prefix eqfind walk + CLI --setup/--status onboarding + new unit tests (LNX-02/03/04/06) ✅ 2026-06-06
  - [x] 25-03-PLAN.md — packaging + auto-update: release.yml linux build/tarball/systemd-unit/install.sh + manifest OS-asset selection (LNX-05) ✅ 2026-06-06

_v2.3 Phase Details (26 Character Assignment, 27 My-Characters Inventory Filter, 28 Character-Tagged Wantlist) — SHIPPED 2026-06-09; collapsed into the v2.3 `<details>` block above. Full per-phase goals, success criteria, and plan lists in [`milestones/v2.3-ROADMAP.md`](milestones/v2.3-ROADMAP.md)._

---

### Phase 29: Data Foundation — Inventory Parse + Price/Value Aggregation
**Goal**: The backend turns the watcher's raw `Location | Name | ID | Count | Slots` inventory rows into structured, query-ready data — a slot taxonomy with container nesting, name-keyed PigParse price + last-listed joins, and bank valuation totals — so every v2.4 web tab reads from a clean, computed model rather than re-parsing strings.
**Milestone**: v2.4
**Depends on**: Nothing new (builds on the live v2.0+ backend, the existing landing-tab ingest, the `_wiki_gear_tier` scrape, and the daily PigParse enrichment). **Watcher untouched.**
**Requirements**: INV-05, DATA-01, DATA-02
**Success Criteria** (what must be TRUE):
  1. A character's stored inventory is exposed through the backend as a structured slot taxonomy — equipment slots in EQ paperdoll positions, general-inventory slots, and bank items — with each general-inventory container's contents nested under it and stack counts preserved (the watcher's `Location`/`Slots` columns parsed server-side; the watcher is unchanged).
  2. Any item surfaced by the backend carries its PigParse price + last-listed-for-sale date joined by **normalized name** (so gear-tier/wiki rows that carry no item_id still resolve a price), available to examine views, suggestion lists, and item lists.
  3. The backend computes, for the guild banks, the summed PigParse value of all bank-held items and the total platinum from the manual bank-coin entries — both queryable as guild-wide totals that power the Banks tab.
  4. The parse + joins + aggregation are covered by Go unit tests against real-name inventory fixtures (slot positions, nested-bag contents, name-join hits/misses, value/platinum sums) and apply over the live data with no schema-breaking change to existing tables (extend-only).
**Plans**: 2 plans (both complete)
  - [x] 29-01-PLAN.md — store read layer: InventoryForChar (all rows, name-join price + last-listed) + real-name nested-bag fixture + stale-comment fixes (INV-05, DATA-01)
  - [x] 29-02-PLAN.md — compute transforms: classifySlot + container nesting + BankValuation (Σ pickPrice×count, +N unpriced) + TotalPlatinum + GearTierPrices + full parity tests (INV-05, DATA-01, DATA-02)

### Phase 30: App Shell + 5-Tab Navigation
**Goal**: squirebot.quest is reframed around five persistent top-level tabs, each answering one question, with per-tab search and a consolidated Settings home — the navigation chrome every other v2.4 surface plugs into.
**Milestone**: v2.4
**Depends on**: Nothing new (web-only routing/chrome rework over the existing SvelteKit app; can land before the tab content is fully built, with placeholder tab bodies).
**Requirements**: NAV-01, NAV-02, NAV-03, NAV-04
**Success Criteria** (what must be TRUE):
  1. The site presents five persistent top-level tabs — Characters, Inventory, Banks, Wishlist, Settings — in that order, with the active tab clearly indicated and reachable from any page.
  2. Each tab has its own in-context search bar scoped to that tab's content (a Characters search behaves differently from an Inventory search, etc.).
  3. The Settings tab consolidates the previously-scattered surfaces — Theme, Notifications preferences, Watcher Codes, Set Class & Level, My Characters, and (officers only) Admin — reachable from one place with a settings search.
  4. The unread-alert badge sits on the **Wishlist** tab (not Settings), and the alert inbox + per-item ping preferences are reached there — every alert is framed as a wishlist-item ping.
**Plans**: 2 plans (both complete — deployed live + browser-smoke 8/8 PASS 2026-06-18)
  - [x] 30-01-PLAN.md — routing spine + chrome: 5-tab strip + active indicator, dissolve the gear to identity+Sign-out, theme-context bridge, badge → Wishlist tab, all old-path redirects, /guild-views preserved home + 3 placeholders (NAV-01/NAV-04 chrome) ✅ 2026-06-18 (e68299e/7f76e54/e7cac71; check 0/0, 309 tests, build green; web-only)
  - [x] 30-02-PLAN.md — tab bodies: /wishlist (rehomed wantlist + notifications inbox/prefs) + /settings (6 panels as sections, officer-gated Admin, settings search, relocated theme picker) (NAV-02/NAV-03/NAV-04 content) ✅ 2026-06-18 (0e4aaac/e9f71a3/8a5ba58; check 0/0, 317 tests, build green; web-only) — DEPLOYED LIVE + browser-smoke 8/8 PASS
**UI hint**: yes

### Phase 31: Characters Tab + In-Game Inventory Window
**Goal**: A guildie can find any character and open an inventory window that looks and behaves like the in-game EQ inventory menu — paperdoll equipment slots, general inventory with openable bags, the character's bank below, real wiki item icons, and a right-click-style examine.
**Milestone**: v2.4
**Depends on**: Phase 29 (the slot taxonomy + container nesting + name-keyed price/last-synced data the window renders), Phase 30 (the Characters tab + its scoped search live in the app shell)
**Requirements**: CHAR-01, CHAR-02, CHAR-03, INV-01, INV-02, INV-03, INV-04
**Success Criteria** (what must be TRUE):
  1. The Characters tab lists all guild characters with name, level, race, and class, ordered the viewer's own characters first (A-Z), then other guild characters, then guild banks/bots; the per-character search prioritizes the viewer's characters.
  2. Selecting a character — from the list or a search result — opens that character's inventory window: equipment slots in the EQ paperdoll arrangement, general-inventory slots, and the character's bank items listed below, with stacked slots showing their count.
  3. A general-inventory container (bag) can be opened to view its contents, which behave like the inventory grid (the Phase 29 container nesting surfaced as drill-down).
  4. Item icons render from the P1999 wiki item-icon images; hovering or tapping an item shows a click-to-pin right-click-style examine — name + stats from the stored wiki data, PigParse price, wiki link, and last-synced.
**Plans**: 4 plans (4 waves — strict chain 31-01 → 31-02 → 31-03 → 31-04)
  - [x] 31-01-PLAN.md — icon enrichment + migration 00012: lucy_img_ID parse → item_master.icon_id (extend-only) + icon_id/last_seen carry-through store→compute→JSON contract (INV-04 backend) ✅ EXECUTED 2026-06-18 (ab8be50/5a88a36; gates green; SUMMARY written; deploy of 00012 deferred to 31-04)
  - [x] 31-02-PLAN.md — read-API: RosterFor viewer-first store read + GET /api/v1/inventory/{char} + GET /api/v1/characters, both RequireSession-gated (CHAR-01/02/03, INV-01..04 data plumbing) ✅ EXECUTED 2026-06-18 (4c43a0c/506e623; gates green; SUMMARY written; both routes RequireSession-wrapped + {char} ?-bound + T-31-05 401 gate proved; deploy deferred to 31-04)
  - [x] 31-03-PLAN.md — Characters tab: api.ts fetch wrappers + pure roster.ts (node-tested) + the bespoke 3-band viewer-first list + scoped search + ?c=<name> selection wiring (CHAR-01/02/03) ✅ EXECUTED 2026-06-18 (d53fe6f/72ec4ab; web gates green — check 0/0, test 331 incl 14 new roster cases, build ok; DOM not browser-verified — deferred to 31-04 deploy-then-smoke)
  - [x] 31-04-PLAN.md — inventory window: examine.ts + PaperdollSlot + ExaminePanel + InventoryWindow (23-slot paperdoll, general+bank grids, inline bag expand, wiki icons + colored-tile fallback, hover/pin examine) + deploy-then-browser-smoke (INV-01..04) ✅ EXECUTED 2026-06-18 (7a6268a/68b9f49 + 8 smoke-fix commits; deployed live + browser-smoke approved across 5 themes; smoke added migration 00013_item_statsblock, bag detection → children-based, 21-of-23 paperdoll slots [Charm/Power Source omitted — post-Velious, never holds items])
**UI hint**: yes

### Phase 32: Inventory Tab (Item-Centric)
**Goal**: A guildie can answer "which characters have item X?" — a guild-wide item list with quantities and links, where selecting an item reveals exactly who holds it, in which slot, how many, and how fresh the data is.
**Milestone**: v2.4
**Depends on**: Phase 29 (guild-wide item rollups + holder-with-slot data + name-keyed PigParse price), Phase 30 (the Inventory tab + its scoped search live in the app shell)
**Requirements**: ITEM-01, ITEM-02, ITEM-03
**Success Criteria** (what must be TRUE):
  1. The Inventory tab lists all guild items with name, guild-wide quantity, a wiki link, and a PigParse price that links to PigParse when applicable.
  2. The per-item name search prioritizes items held on the viewer's own characters.
  3. Selecting an item shows which characters hold it, the inventory slot on each, the quantity, and the last-synced day/time — a master-detail drill-down consistent with the character-window examine.
**Plans**: 3 plans
  - [x] 32-01-PLAN.md - Backend: compute.Items rollup (group-by-normalized-name + per-holder detail) + ItemMasterIconStats read + GET /api/v1/items (RequireSession) + main.go registration (ITEM-01/02/03 data) — EXECUTED 2026-06-18 (39b6660/8bc2cbd/446fa2d); go test ./... all packages ok
  - [x] 32-02-PLAN.md - Web: api.ts ItemRollup/ItemHolder + fetchItems() + pure items.ts (viewer-first/filter/holder-sort) + the /inventory master-detail tab (reused ExaminePanel + holders deep-linking to /characters?c=) (ITEM-01/02/03 UI) — EXECUTED 2026-06-18 (e7314bb/2d36dbc); web check 0/0, test 359/28, build ok; DOM-blind render → 32-03 browser-smoke
  - [x] 32-03-PLAN.md - Deploy (binary swap to register the route + web atomic swap + R2 backup, NO goose run) + browser-smoke the live tab across all 5 themes (closes ITEM-01/02/03)
**UI hint**: yes

### Phase 33: Banks Tab + Valuation
**Goal**: A guildie can answer "what's in the guild banks, and what is it worth?" — a banks-only list that opens each bank's inventory window, plus the total item value and total platinum held across the guild banks.
**Milestone**: v2.4
**Depends on**: Phase 29 (bank valuation + total-platinum aggregation, the inventory-window data), Phase 31 (reuses the in-game inventory window for each bank), Phase 30 (the Banks tab + its scoped search live in the app shell)
**Requirements**: BANK-01, BANK-02, BANK-03
**Success Criteria** (what must be TRUE):
  1. The Banks tab lists only guild-bank characters (same ordering style as the Characters tab), and selecting one opens its inventory window.
  2. The tab shows the total PigParse value of all items held by bank characters and the total platinum held across the guild banks (from the manual bank-coin entries).
  3. A per-item name search runs across the items held by the guild banks.
**Plans**: 3 plans
  - [x] 33-01-PLAN.md — Backend: store.InventoryJoinBanksAndBots + ListBankAndBotToons (Option B widen — value includes bots, plat stays bank-toon-gated) + compute.Banks/BanksView/BankRowSummary (per-bank value + nullable plat + item count + guild totals) + GET /api/v1/banks (RequireSession, no viewer id) + main.go registration + tests (BANK-01/02 data) ✅ EXECUTED 2026-06-19 (e7ddfa7/bfda028/7e43687; go test ./... all green; Pitfall-1 bot-in-guild-value regression proven; legacy /views/bank unaffected; schema stays v13)
  - [x] 33-02-PLAN.md — Web: api.ts BanksView/BankRowSummary + fetchBanks() + pure banks.ts (A-Z sort + is_bank item-search with bank-slice qty recompute + bankByName) + the /banks master-detail tab (D-02 summary header + list/search toggle + D-04 per-bank header + reused InventoryWindow + in-tab holder deep-link) (BANK-01/02/03 UI) ✅ EXECUTED 2026-06-19 (577d503/900c4d6/cc0a7b8; web-only; check 0/0, test 370/29 incl. 11 banks cases w/ the Pitfall-3 recompute, build ok; ?b= consistent, no new {@html}, holder deep-link in-tab; bank/bot tag = Option (a) neutral "bank"; nil plat → "not recorded"; DOM browser-smoke deferred to 33-03)
  - [x] 33-03-PLAN.md — Deploy (binary swap to register /api/v1/banks + web atomic swap + R2 backup, NO goose run) + browser-smoke the live tab across all 5 themes (closes BANK-01/02/03)
**UI hint**: yes

### Phase 34: Wishlist Rework — Per-Character Per-Slot Upgrades
**Goal**: The just-shipped wantlist becomes a per-character, per-equipment-slot upgrade wishlist — open-ended targets per slot, complete Velious wiki suggestions with price/wiki/last-listed, a Discord ping toggle with an EC-hit badge, and the right-click examine — answering "what can I get to improve my characters?"
**Milestone**: v2.4
**Depends on**: Phase 31 (equipped-slot detection — the currently-equipped item per slot comes from the inventory-window parse), Phase 29 (name-keyed price/last-listed on suggestions + the `_wiki_gear_tier` data), Phase 30 (the Wishlist tab + its scoped search + the notifications badge/inbox living here), and the ALREADY-SHIPPED v2.2 EC-monitor + notification spine (reused, not rebuilt)
**Requirements**: WISH-01, WISH-02, WISH-03, WISH-04, WISH-05, WISH-06, WISH-07
**Success Criteria** (what must be TRUE):
  1. The Wishlist tab lists characters (the viewer's first A-Z, then others), excludes guild banks/bots, and selecting one shows that character's equipped slots with the currently-equipped item per slot.
  2. Each equipped slot holds an open-ended set of user-entered upgrade targets (typed or chosen from suggestions); an item leaves the slot's wishlist automatically when SquireBot sees it on that character, or when the user removes it.
  3. Per slot, SquireBot suggests upgrades from the complete Velious Pre-raid/Grouping + Raiding lists for that character's class+slot (from the existing `_wiki_gear_tier` data); each suggestion shows its PigParse price, wiki link, and last-listed-for-sale date, with no-drop/raid-only items tagged "Raid" and shown as not-for-sale.
  4. Each wishlisted item has a Discord ping toggle; when SquireBot pings the user (e.g. the item appeared in the EC tunnel via the shipped EC-monitor + notification spine), a badge appears beside that item in the wishlist.
  5. Hovering or tapping any item shows the right-click-style examine (stats, price, wiki, last-synced), and a wishlist search covers all items on any wishlist plus the non-bank/bot characters.
**Plans**: 4 plans
  - [x] 34-01-PLAN.md — Backend data foundation: migration 00014 (wishlist_item + alert_log FK rebuild + drop wantlist_item) + owner-scoped store CRUD/ping + compute-on-read WishlistFor (equipped + auto-removal + class+slot suggestions via the slot-vocab bridge) ✅ EXECUTED 2026-06-19 (ea0223a/6f1eea3/00574e9; schema v14; WISH-02/03/04 backend half; migrations+store-wishlist+compute gates green, go build rc=0; the 5 retired-wantlist test packages [wantmatch/webadmin/store/ec/notify] are the EXPECTED 34-02 hand-off)
  - [x] 34-02-PLAN.md — Backend matcher repoint (wantmatch -> wishlist_item) + owner-scoped write API (add/remove/ping) + GET /api/v1/wishlist/{char} read route + main.go (register 4 wishlist routes, remove 5 wantlist routes) ✅ EXECUTED 2026-06-19 (cbbc208/58587e1/03de8af; WISH-01/05/07; wantmatch+ECPollSet repointed [pinged gate, INNER JOIN, no note], DM-target-is-owner regression ported, owner-scoped webadmin/wishlist.go [in-tx IsCharAssignedToTx 403 / 409 dup / 21-slot 400 / silent IDOR no-op], readapi/wishlist.go, main.go 4-registered/5-removed; the full clean-break test repair → go test ./... GREEN, go vet clean, go build rc=0; 2 Rule-3 auto-fixes [ECPollSet, whyWanted]; NO web/watcher change, NO migration)
  - [x] 34-03-PLAN.md — Web /wishlist tab: viewer-first char list (banks/bots excluded) + WISH-07 search + per-slot accordion (equipped + targets + suggestions + ping toggle + EC badge + reused ExaminePanel) + api.ts/wishlist.ts; delete the old WantlistPanel + wantlist/* ✅ EXECUTED 2026-06-19 (ab4a2af/18fdd61/0e46d65; WISH-01..07 code-shipped; api.ts WishlistView interfaces+wrappers [mirror the Go contract] · pure node-tested wishlist.ts [banks/bots-excluded viewer-first + WISH-07 cross-wishlist search over the WHOLE lazy-fetched+cached corpus, no scope-down] · /wishlist per-character per-slot master-detail [server-ordered 21-slot accordion, target rows w/ price+Wiki+last-listed+Raid tag+ping Toggle+"Seen in EC" badge+ExaminePanel, cloned-debounce typed-entry add + suggestion picker, server-truth add/remove/ping, ConfirmDialog remove, read-only on non-owned chars] · KEPT the NAV-04 Notifications region · DELETED WantlistPanel+groupByChar, KEPT priority.ts/holders.ts; web check 0/0 [508 files] + npm test 380 [29 files, +15 wishlist] + build green; 0 deviations; NO backend/watcher change, NO deploy [that's 34-04 — node vitest is DOM-blind])
  - [x] 34-04-PLAN.md — Deploy (goose-run 00014 + web atomic swap; R2 backup BEFORE the restart; NO v* tag) + human browser-smoke across the 5 EQ themes

### Phase 35: Owner-less guild banks & bots
**Goal**: A designated guild bank/bot is GUILD-HELD, not tied to whoever uploaded it first. Designating a character as bank/bot must not require "claiming"/owning it, and a guild bank/bot must survive any individual member's eviction.
**Milestone**: v2.5
**Depends on**: Nothing new (builds on the live backend; the cross-owner write-gate relaxation `260621-u6j` already shipped). **Watcher untouched.**
**Requirements**: OWN-01, OWN-02, OWN-04
**Resolved design (Option A — sentinel owner):** a reserved owner row (id `1000000`, label `guild`) seeded by migration 00015 holds designated banks/bots, so `character.owner_id` stays `NOT NULL` and every existing `owner(id)` join works unchanged. Rejected Option B (nullable `owner_id`) — it would force a SQLite table rebuild to drop NOT NULL and make every `owner_id` consumer handle NULL. Grep confirmed only two production consumers of `character.owner_id`: `store/binding.go` (first-sighting bind, untouched) + `store/eviction.go` (the cascade + owner lists).
**Success Criteria** (what must be TRUE):
  1. An officer can designate any character as a guild bank/bot WITHOUT first owning it — `DesignateCharTx` repoints `owner_id` to the guild sentinel and is gated only by the in-tx officer re-check (no claim step).
  2. A designated bank/bot is owner-less (guild-held): its `character.owner_id` equals the reserved sentinel id, not any individual guildie.
  3. A designated bank/bot is NOT removed when its first-uploader is evicted — `EvictOwnerTx` (`WHERE owner_id = realOwner`) never touches the sentinel-owned bank.
  4. Existing designated banks/bots bound to an individual owner (e.g. Findom→owner 9) migrate automatically via the 00015 backfill, with no manual fixup; the guild sentinel never appears in the officer eviction picker.
  5. `go test ./internal/backendsrv/...` + `go build ./...` green; watcher untouched; no `v*` tag.
**Sets up Phase 36 (OWN-03):** with banks parked under a single sentinel owner no eviction targets, banks are eviction-safe by construction — Phase 36 then narrows the cascade purely for shared NON-bank characters.
**Plans**: 1 plan (complete)
  - [x] 35-01-PLAN.md — migration 00015 (sentinel-owner seed + bank/bot backfill) + store/owner.go GuildSentinelOwnerID + DesignateCharTx owner_id repoint + eviction-list sentinel exclusion + OWN-02 survives-eviction proof (OWN-01/02/04) — EXECUTED 2026-06-22 (7a238b8/c8305c2/4d38389)

---

### Phase 37: Item enrichment backbone — flags + effects
**Goal**: SquireBot captures each item's attribute flags (LORE / NO DROP / MAGIC / TEMPORARY) and its click-effect (Clicky) + Haste as discrete, queryable fields — re-enabling the wiki stat-block parse the Go port deliberately discarded (the `wikiitem.go` D-8 scope guard) so the rest of the milestone (flag outlines, faceted search) has a clean data layer to read from.
**Milestone**: v2.6
**Depends on**: Nothing new (the parser already COMPUTES these values; this re-surfaces + persists them). **Watcher untouched.** **Foundational — gates Phase 39 (facets) + Phase 40 (flag outlines).**
**Codebase facts**: `internal/backendsrv/enrich/wikiitem.go` already derives `is_no_drop/is_lore/is_magic/is_temporary` + the click-effect/haste lines but the D-8 scope guard drops them (only `is_quest_item` is kept; the full stat block is one TEXT blob in `item_master.statsblock`, migration 00013). This phase adds a goose migration for the discrete columns + updates the enrich freshness short-circuit (`GetItemMasterFreshnessTx` in `store/enrich.go`) so existing rows re-parse and backfill on the next weekly wiki pass.
**Requirements**: ENRICH-12, ENRICH-13
**Success Criteria** (what must be TRUE):
  1. After the weekly wiki enrichment runs, an item's LORE / NO-DROP / MAGIC / TEMPORARY attribute flags are stored as discrete boolean fields (not buried in the free-text stat block), and an item known to carry each flag reads back correctly.
  2. An item's click-effect (Clicky) and Haste are stored as discrete queryable fields — an item with a click effect is marked as a Clicky and a haste item is marked as Haste, both distinguishable in a query without re-parsing text.
  3. Items already enriched under the old (flag-discarding) parse are backfilled with the new fields on the next enrichment pass — the migration + the freshness short-circuit re-trigger a re-parse rather than leaving existing rows blank forever.
  4. The new columns ship as an extend-only goose migration (added at the right edge, no break to existing reads) and `go test ./...` + `go build ./...` are green; watcher untouched, no `v*` tag.
**Plans**: 2 plans / 2 waves
- [x] 37-01-PLAN.md — Extend the wiki-item parser to surface flags + Clicky/Haste (parser-only; no DB) — ✅ EXECUTED 2026-06-24 (`3ab6463`/`f656a7e`/`d55fe28`)
- [x] 37-02-PLAN.md — Migration 00016 + discrete columns + no-network statsblock backfill + freshness self-heal + job wiring — ✅ EXECUTED 2026-06-24 (`e590f72`/`dd42fc3`/`2dcc0b3`/`b90a872`; MD-01 fix `+ follow-up`)

### Phase 38: Catalog-wide enrichment + icon coverage
**✅ STATUS (2026-06-25): RE-PLAN EXECUTED + VERIFIED — D-04 NAME-KEYED (Option B).** The id-keyed Option A was held pre-deploy when the prod probe disproved its ≈0-collision assumption (60 raw id-collisions / 43 genuinely-unheld catalog items dropped). Re-planned + re-executed name-keyed: migration **00017** `catalog_enrichment(norm_name PK,…)` + `store/catalogenrich.go` (`UpsertCatalogEnrichmentTx`/`GetCatalogEnrichmentFreshnessTx`, the name-keyed parallel of the held path) + `itemids.go` rework (Option-A guard dropped, `EnrichmentRef.Held`, both-stores `CatalogIconCoverage`) + the `wiki.go` `ref.Held` write-branch (held → `item_master`+`quest_items`; catalog-only → `catalog_enrichment` by `norm_name`, no quest write). `item_master` + the 5 held readers byte-for-byte unchanged (blast radius ZERO). Covers all 4,343, no drop. Verifier 4/4 SC + 2/2 reqs (human_needed = post-deploy seed crawl + browser render only); code-review 0 BLOCKER (WR-01/WR-02 name-keying bug keyed catalog rows on the wiki name instead of `ref.Name` → fixed-forward `9735f59` + regression test; IN-01 docstring `c7dae81`; IN-02/IN-03 → 999.43; IN-04 no-action). Full `go test ./...` green. See `38-CONTEXT.md` ⚠ D-04 REVERSED block + `38-DISCUSSION-LOG.md` + `38-VERIFICATION.md` + `38-REVIEW.md`. ✅ **DEPLOYED LIVE 2026-06-25** (37+38 together: goose applied 00016 + 00017 on boot, prod schema v15 → v17; service active, 0 restarts; apex 200 / `/api/v1/*` 401; `catalog_enrichment` seeded empty — populates on the next Sunday UTC wiki crawl; `.bak` rollback kept).
**Goal**: Item enrichment stops being limited to items some character currently holds — it covers the full PigParse Blue catalog (politefetch-paced) — which both fixes most of the "not all items have images" bug and provides the full-catalog data the faceted-search scope toggle (Phase 39) will read; a maintainer can also see exactly which items remain genuinely icon-less.
**Milestone**: v2.6
**Depends on**: Nothing hard new (extends the existing weekly wiki job + the `pigparse_price` catalog). Lands before Phase 39's full-catalog scope. **Watcher untouched.** **The biggest / riskiest piece of the milestone** — the held-items work in other phases is sequenced so it ships even if this crawl runs long.
**Codebase facts**: enrichment is INVENTORY-DRIVEN today — the weekly wiki job iterates `store.DistinctInventoryItemIDs` (`store/itemids.go`), so only currently-held items get a wiki page fetched. This phase widens the source set to the full ~4,300-row `pigparse_price` Blue catalog, paced by the shared `politefetch` helper so the crawl stays a courteous wiki citizen. Icons come from the wiki `lucy_img_ID` param into `item_master.icon_id` (migration 00012); they go missing when the page lacks the param OR (more commonly) the item was never enriched under the held-items gate — so widening coverage fixes most of them. The colored-tile fallback (`PaperdollSlot.svelte`) stays for genuinely icon-less items.
**Requirements**: ENRICH-14, ENRICH-15
**Success Criteria** (what must be TRUE):
  1. The weekly enrichment fetches and stores wiki data for items across the full PigParse Blue catalog, not only items currently held by a guild character — a catalog item nobody holds still gets enriched.
  2. Items that previously showed only the colored-tile fallback because they were never enriched now render their real wiki icon (the "missing item images" bug is closed for every item whose wiki page actually provides an icon).
  3. A maintainer can see a coverage diagnostic listing which items are still icon-less, distinguishing "genuinely has no wiki icon" (fallback is correct) from any remaining gap.
  4. The catalog crawl is politefetch-paced (User-Agent, ETag/If-Modified-Since, backoff honoring Retry-After, the inter-request sleep) so the widened scope does not hammer the wiki; `go test ./...` green; watcher untouched, no `v*` tag.
**Plans**: 2 plans (name-keyed re-plan — REPLACES the retired Option-A 38-01)
  - [ ] 38-01-PLAN.md (wave 1) — migration 00017 `catalog_enrichment(norm_name PK,…)` + store/catalogenrich.go (UpsertCatalogEnrichmentTx + GetCatalogEnrichmentFreshnessTx, mirrors item_master re-keyed on norm_name) + itemids.go rework (drop the Option-A `item_id NOT IN item_master` guard, add EnrichmentRef.Held, both-stores CatalogIconCoverage) — the name-keyed foundation; item_master untouched (ENRICH-14/15)
  - [ ] 38-02-PLAN.md (wave 2, depends_on 38-01) — branch the wiki.go items-pass write on ref.Held (held → item_master+quest_items unchanged; catalog-only → catalog_enrichment by norm_name, no quest write) + re-point the D-03 coverage slog to CatalogIconCoverage (both stores) + wiki_test asserts the unheld item lands in catalog_enrichment by norm_name, not item_master (ENRICH-14/15)

### Phase 39: Faceted item search (Clicky / Haste + scope toggle)
**Goal**: A guildie can narrow item search to just the item types they care about — Clicky items or Haste items — and flip the search scope between "who in the guild has one" (holdings) and "what exists in P99" (the full catalog), turning the flat name/id search into a useful discovery tool.
**Milestone**: v2.6
**Depends on**: Phase 37 (the parsed Clicky / Haste effect fields the facets filter on), Phase 38 (the full-catalog enrichment coverage the scope toggle's "full catalog" mode needs to be meaningful). **Watcher untouched.**
**Codebase facts**: search today is `GET /api/v1/items/search` → `store.SearchCatalog` over `pigparse_price` (name/id LIKE only — no stat facets, no facet UI). This phase adds the Clicky / Haste facets (reading Phase 37's parsed effects) + the holdings↔catalog scope toggle (the full-catalog facet results depend on Phase 38's coverage). Facets are Clicky + Haste ONLY (locked — not slot / stat / weapon-type; those are deferred).
**Requirements**: SEARCH-04, SEARCH-05, SEARCH-06
**Success Criteria** (what must be TRUE):
  1. A user can filter item search to show only Clicky items (items with a click effect), and the result set excludes non-clicky items.
  2. A user can filter item search to show only Haste items, and the result set excludes non-haste items.
  3. A user can toggle the search scope between guild holdings ("who has one") and the full P99 catalog ("what exists"); the holdings scope answers from items guildies hold and the catalog scope answers from the full enriched catalog.
  4. The held-items faceting (Clicky/Haste over holdings) works on its own even if the full-catalog scope is still filling in — the holdings path does not block on the catalog crawl completing.
**Plans**: 2 plans (2 waves; backend contract first, web UI second)
  - [x] 39-01-PLAN.md (wave 1) — backend: widen IconStats/ItemRollup with is_clicky/has_haste (client holdings facet, SC-4); add clicky/haste params to SearchCatalog via the name-keyed item_master ∪ catalog_enrichment union; parse ?clicky=/?haste= on /api/v1/items/search (guard+LIMIT+V7 preserved). No migration; watcher untouched (SEARCH-04/05/06)
  - [x] 39-02-PLAN.md (wave 2, depends_on 39-01) — web: pure facetItems() helper + api.ts contract mirror; FacetBar.svelte chips; Inventory-tab Holdings↔Catalog .seg scope toggle (D-03 lens-not-reset) + catalog-row ExaminePanel reuse + holders-by-name (D-04); wishlist add-form facets (D-01, catalog-only); browser-smoke checkpoint (SEARCH-04/05/06)
**UI hint**: yes

### Phase 40: Item display polish — flag outlines + named quest links
**Goal**: Items become readable at a glance and traceable to their quests — every item tile carries a color-coded outline by its attribute flag (No-Drop red, Lore gold, Magic blue), and the modern Characters / Inventory / Wishlist tabs surface the named quests an item is used in ("used in quest X"), not just a yes/no QUEST-ITEM badge.
**Milestone**: v2.6
**Depends on**: Phase 37 (ITEMUI-01's flag outlines read the discrete flag fields). ITEMUI-02 is independent of Phase 37 — the named quest data already exists. **Watcher untouched.**
**Codebase facts**: flag outlines (ITEMUI-01) attach to the grid/paperdoll tiles (`PaperdollSlot.svelte`, the inventory/bank list rows) and depend on Phase 37's discrete flags. Named quest links (ITEMUI-02): the `quest_items` table + `store.QuestLinksByItem` already exist and named `quest_links` already surface on the LEGACY `guild-views` tab, but the modern tabs' `InventorySlot` / `ItemRollup` models carry only a yes/no `is_quest_item` bool — so this is plumbing the existing named links into those models + the examine panel (compute + `web/src/lib/api.ts` + `examine.ts`), no new data pipeline.
**Requirements**: ITEMUI-01, ITEMUI-02
**Success Criteria** (what must be TRUE):
  1. Items in the inventory, bank, and paper-doll views show a color-coded outline by flag — No-Drop = red, Lore = gold, Magic = blue — while an item with no special flag keeps the existing neutral border.
  2. The flag-coded outline is consistent across the tile surfaces (paper-doll slots and the inventory/bank list rows) so the same item reads the same way wherever it appears.
  3. On the modern Characters, Inventory, and Wishlist tabs, an item used in one or more quests shows the named quest(s) it's used in ("used in quest X"), not just the yes/no QUEST-ITEM badge.
  4. The named quest links surface in the item examine panel as well as the list/grid views, drawing on the already-harvested `quest_links` (no new enrichment pipeline) — and `go test ./...` is green; watcher untouched.
**Plans**: 2 plans (2 waves; backend contract first, web UI second)
  - [x] 40-01-PLAN.md (wave 1) — backend: carry item_master is_no_drop/is_lore/is_magic (P37 00016, the P39 is_clicky/has_haste precedent) + quest_items.source_url through compute -> InventorySlot/ItemRollup JSON; extend QuestLinksByItem SELECT + QuestLink.SourceURL; attach quest_links to the modern builders (the view.go/bank.go pattern). No migration; watcher untouched (ITEMUI-01/02) (completed 2026-06-26)
  - [ ] 40-02-PLAN.md (wave 2, depends_on 40-01) — web: api.ts mirror + 3 per-theme flag tokens --flag-nodrop/--flag-lore/--flag-magic (app.css+themes.ts lockstep, parity tests); pure priority resolver (No-Drop>Lore>Magic); PaperdollSlot ::before flag ring (D-03 survives hover); examine flag chip + clickable named quest links (notes_link via source_url through safeHttpUrl) on Characters/Inventory/Wishlist; composeNotes tooltip links; browser-smoke checkpoint (ITEMUI-01/02)
**UI hint**: yes

### Phase 41: Character paper-doll — compaction + portrait photo
**Goal**: The character paper-doll stops wasting its reserved portrait space — the layout is compacted toward the in-game inventory-window idiom — AND that portrait area becomes useful: a guildie can set an optional portrait photo per character that displays there.
**Milestone**: v2.6
**Depends on**: Nothing hard new for the compaction (web-only over the existing `InventoryWindow.svelte`); the photo (CHARUI-02) needs a new per-character image reference + migration + upload path + read-API. **Watcher untouched.**
**Codebase facts**: `InventoryWindow.svelte` has a ~260px dashed placeholder `.doll` box (a `⚔` emoji) reserved for a portrait; NO per-character image support exists today. CHARUI-01 compacts that layout; CHARUI-02 adds the per-character photo (a new image column/table + migration + an upload path + read-API plumbing — the new persisted-data piece of this milestone alongside Phase 37's flags).
**Requirements**: CHARUI-01, CHARUI-02
**Success Criteria** (what must be TRUE):
  1. The character paper-doll view is compacted — the empty portrait dead space is reclaimed and the layout reads tighter, closer to the in-game inventory-window arrangement.
  2. A guildie can upload an optional portrait photo for a character they own, and it is stored and associated with that character.
  3. A character with a portrait photo set shows it in the paper-doll's portrait area (replacing the placeholder); a character with none keeps the default placeholder.
  4. The per-character image reference ships as an extend-only migration with a server-side upload path + read-API plumbing; `go test ./...` is green; watcher untouched, no `v*` tag.
**Plans**: TBD
**UI hint**: yes

### Phase 42: Wishlist polish — compaction + sub-Velious tiers
**Goal**: The Wishlist tab matches the density and visual idiom of the other tabs, and its upgrade suggestions stop being Velious-only — sub-Velious (Kunark / classic) gear tiers are included so lower-progression characters get useful suggestions too.
**Milestone**: v2.6
**Depends on**: Nothing hard new — the Wishlist UI already uses the same two-pane pattern as the other tabs (so WISHUI-01 is density/visual-consistency work, not an architecture swap), and WISHUI-02 adds wiki source pages + tier values to the existing `wiki_gear_tier` table. **Watcher untouched.**
**Codebase facts**: WISHUI-01 compacts the bespoke wishlist UI (`web/src/routes/wishlist/+page.svelte`) — it already shares the two-pane pattern, so this is density work. WISHUI-02: the `wiki_gear_tier` table exists but is populated ONLY from two Velious wiki pages (`wikiVeliousGearSources`); this adds Kunark/classic source pages + their tier values — no migration (`wiki_gear_tier` already exists).
**Requirements**: WISHUI-01, WISHUI-02
**Success Criteria** (what must be TRUE):
  1. The Wishlist tab is visually consistent with the other tabs' density and layout idiom — tighter spacing, the shared two-pane pattern, no bespoke-looking density relative to Characters / Inventory / Banks.
  2. Wishlist per-slot suggestions include sub-Velious (Kunark / classic) gear tiers, not only Velious tiers — a character below Velious-readiness gets suggestions from the lower tiers.
  3. The sub-Velious tiers come from new wiki source pages + tier values added to the existing `wiki_gear_tier` data (no schema migration), enriched via the same weekly wiki job.
  4. `go test ./...` is green; the wishlist compaction reuses the existing design system (no re-skin); watcher untouched, no `v*` tag.
**Plans**: TBD
**UI hint**: yes

## Progress

**Execution Order:** Phases execute in numeric order. v2.0: 11 → 12 → 13 → 14 → 15 → 16 (complete). v2.1: 17 → 18 (complete). v2.2: 19 → 20 → 21 (Track 1, unblocked) → 22 → 23 (Track 2, invite-gated; can slot earlier if invites land, but Track 1 ships independently). v2.3: 26 → 27 → 28 (complete). v2.4: 29 → 30 → 31 → 32 → 33 → 34 (29 data foundation first; 30 shell; then the four tab surfaces; 34 wishlist last). v2.5: 35 → 36 (complete). v2.6: 37 → 38 → 39, then 40 · 41 · 42 (37 parse backbone first, gates 39+40; 38 catalog coverage gates 39's full-catalog scope; 40/41/42 independent once 37/38 land).

| Milestone | Phases | Plans Complete | Status | Completed |
|-----------|--------|----------------|--------|-----------|
| v1.0 | 5 | 31/31 | ✅ Shipped | 2026-05-11 |
| v1.0.1 | 3 | 12/12 | ✅ Shipped | 2026-05-12 |
| v1.0.2 | 2 | 8/8 | ✅ Binary shipped (milestone close superseded by v2.0) | 2026-05-13 |
| v2.0 | 6 | 29/29 | ✅ Shipped (tag `v2.0.0`; Google decommissioned) | 2026-05-31 |
| v2.1 | 2 | 4/4 | ✅ Complete (Phases 17–18 shipped) | 2026-06-02 |
| v2.2 | 6 | 13/TBD | 🔄 **Track 1 SHIPPED LIVE** (Phases 19–21 deployed 2026-06-06 + Phase 24 quality done); Track 2 (22–23) invite-gated, parked | — |
| v2.3 | 3 | 7/7 | ✅ Feature-complete — all 3 phases SHIPPED + deployed live (schema v10); pending milestone audit/close | 2026-06-09 |
| v2.4 | 6 | 6/6 phases | ✅ **Feature-complete** — all 6 phases (29 Data Foundation · 30 App Shell · 31 Characters+Window · 32 Inventory · 33 Banks · 34 Wishlist) SHIPPED + DEPLOYED LIVE to squirebot.quest + browser-smoke PASS across 5 themes; schema v14 (migrations 00012/00013/00014). Each phase: verifier PASSED + code-review clean/fixed. Pending: milestone audit/close. | 2026-06-21 |
| v2.5 | 2 | 3/3 plans | ✅ **Shipped + archived** — Phase 35 (owner-less guild banks/bots, OWN-01/02/04, schema v15 via migration 00015) + Phase 36 (shared-character-safe eviction, OWN-03, no migration) both deployed live + officer browser-smoke approved; backend + web, no `v*` tag | 2026-06-22 |
| v2.6 | 6 | 0/TBD | 🔁 **Planning** — Phases 37–42 (37 enrichment backbone flags/effects · 38 catalog-wide enrichment + icon coverage · 39 faceted Clicky/Haste search + scope toggle · 40 flag outlines + named quest links · 41 paper-doll compaction + portrait photo · 42 wishlist compaction + sub-Velious tiers); 13/13 requirements mapped; backend + web, watcher untouched, no `v*` tag | — |

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 11. Backend Foundation + Ingest API | v2.0 | 7/7 | ✅ Complete | 2026-05-29 |
| 12. Enrichment Job Migration | v2.0 | 5/5 | ✅ Complete | 2026-05-29 |
| 13. Watcher Re-Target + Onboarding | v2.0 | 4/4 | ✅ Complete | 2026-05-30 |
| 14. Web Frontend | v2.0 | 4/4 | ✅ Complete (deployed live) | 2026-05-30 |
| 15. Admin Web Forms + Login | v2.0 | 5/5 | ✅ Complete (deployed live; most UAT smokes exercised during the 16-03 deploy) | 2026-05-31 |
| 16. Cutover + Decommission | v2.0 | 4/4 | ✅ Complete (Google decommissioned; guild migrating) | 2026-05-31 |
| 17. Self-Service Watcher Linking | v2.1 | 3/3 | ✅ Complete (deployed live; browser-smoke approved; 15/15 verified) | 2026-06-02 |
| 18. Watcher Cleanups — Verify-or-Close | v2.1 | 1/1 | ✅ Complete (verify-or-close; zero new code; 0.4.0-rc1 = Azure test VM, not production) | 2026-06-02 |
| 19. Wantlist CRUD | v2.2 | 3/3 | Complete    | 2026-06-05 |
| 20. Bot + DM + Notification Infrastructure | v2.2 | 5/5 | Complete    | 2026-06-05 |
| 21. EC-Tunnel Auction Monitor | v2.2 | 3/3 | ✅ Complete + **DEPLOYED LIVE** (schema v8; bot connected, ec_auction_match job running; WANT-05) | 2026-06-06 |
| 22. WTS Cross-Server Monitor | v2.2 | 0/TBD | Not started (INVITE-GATED) | — |
| 23. Quest-Target Raid Monitor | v2.2 | 0/TBD | Not started (INVITE-GATED) | — |
| 24. Watcher test hardening (C1/C2 coverage) | v2.2 | 2/2 | ✅ Complete (C1/C2/REFACTOR closed; 10/10 verified) | 2026-06-03 |
| 25. Linux Watcher | v2.2 | 3/3 | ✅ Complete (25-03 done — OS-specific manifest assets + runtime.GOOS auto-update selection, tarball + systemd user unit + install.sh, additive release.yml linux build; linux closure CGO-free, Windows NSIS path + `go test ./...` unchanged) | Human UAT on a real Linux+WINE box (watch→upload→autostart→self-update) |
| 26. Character Assignment | v2.3 | 3/3 | ✅ SHIPPED — deployed live + browser-smoke PASS (ASSIGN-01..06) | 2026-06-08 |
| 27. My-Characters Inventory Filter | v2.3 | 1/1 | ✅ SHIPPED — deployed live + browser-smoke PASS (MYVIEW-01/02) | 2026-06-08 |
| 28. Character-Tagged Wantlist | v2.3 | 3/3 | ✅ SHIPPED — deployed live v10 + browser-smoke PASS (CWANT-01..06) | 2026-06-09 |
| 29. Data Foundation — Inventory Parse + Price/Value Aggregation | v2.4 | 2/2 | ✅ Complete (INV-05/DATA-01/DATA-02; verifier 4/4; compute-on-read, no migration; watcher untouched) | 2026-06-17 |
| 30. App Shell + 5-Tab Navigation | v2.4 | 2/2 | ✅ Complete — DEPLOYED LIVE to squirebot.quest + browser-smoke 8/8 PASS; verifier 12/12 (NAV-01..04; web-only) | 2026-06-18 |
| 31. Characters Tab + In-Game Inventory Window | v2.4 | 4/4 | ✅ Complete — DEPLOYED LIVE + browser-smoke approved across 5 themes; verifier 4/4 + 7/7 req IDs; migrations 00012/00013 (CHAR-01..03, INV-01..04) | 2026-06-18 |
| 32. Inventory Tab (Item-Centric) | v2.4 | 3/3 | Complete    | 2026-06-19 |
| 33. Banks Tab + Valuation | v2.4 | 3/3 | Complete    | 2026-06-19 |
| 34. Wishlist Rework — Per-Character Per-Slot Upgrades | v2.4 | 4/4 | Complete    | 2026-06-21 |
| 35. Owner-less guild banks & bots | v2.5 | 1/1 | ✅ Complete (35-01 EXECUTED 2026-06-22 — 7a238b8/c8305c2/4d38389; OWN-01/02/04; schema v15 migration 00015 sentinel-owner seed + backfill, DesignateCharTx repoint, eviction-list exclusion, OWN-02 survives-eviction proof; build/vet/tests green; backend-only, no `v*` tag) | 2026-06-22 |
| 36. Shared-character-safe eviction | v2.5 | 2/2 | ✅ Complete + DEPLOYED LIVE 2026-06-22 (OWN-03; narrowed cascade + `preserved_shared_count` contract + web EvictionForm fix; verifier 13/13; no migration) | 2026-06-22 |
| 37. Item enrichment backbone — flags + effects | v2.6 | 0/TBD | Not started (ENRICH-12/13; re-enable the dropped flag/effect parse into discrete columns + migration; foundational) | — |
| 38. Catalog-wide enrichment + icon coverage | v2.6 | 0/TBD | Not started (ENRICH-14/15; widen enrichment to the full PigParse catalog + icon backfill/diagnostic) | — |
| 39. Faceted item search (Clicky / Haste + scope toggle) | v2.6 | 2/2 | Complete    | 2026-06-26 |
| 40. Item display polish — flag outlines + named quest links | v2.6 | 0/2 | 🔁 Planned (2 plans, 2 waves; backend plumbing → web render; no migration, watcher untouched) | — |
| 41. Character paper-doll — compaction + portrait photo | v2.6 | 0/TBD | Not started (CHARUI-01/02; migration for the photo) | — |
| 42. Wishlist polish — compaction + sub-Velious tiers | v2.6 | 0/TBD | Not started (WISHUI-01/02; no migration — `wiki_gear_tier` exists) | — |

## Backlog

Carried forward from v1.0 / v1.0.1 / v1.0.2 (candidates for a future Sheet-orthogonal patch or v2.x). Note: several Sheet-side items below are likely **mooted by v2.0** (the Sheet + Apps Script are being decommissioned in Phase 16) — they are retained here for the record and to assess at v2.0 close.

- **999.1** Bank-coin permission lock (Sheet sidebar) — likely mooted by ADMIN-05 (web form replaces the sidebar).
- **999.2** Polished theme picker tile UI (Sheet) — mooted by WEB-05 (theme becomes a per-user client preference on the website).
- **999.5** Self-service eviction (departing guildie quits cleanly without officer action) — v2.x candidate; threat-model deferred.
- **999.7** Extract `SIDEBAR_BODY` constants to external `.html` (Sheet) — mooted by the frontend rebuild.
- **999.9** SignPath Foundation OSS approval — submitted; awaiting review (would retire INST-05 partial → full). Lands as a hotfix when approved; orthogonal to the backend swap.
- **999.11** Decide v2.x verification doctrine — adopt `/gsd-verify-work` per phase, or formalize a live-smoke pattern.
- **999.12 / WANT-01..08** v2: Wantlist + Discord pinger — prerequisites WANT-06/07 still open (Raid Alliance Discord bot invites). v2.0 pre-pays the per-user Discord-identity prerequisite via AUTH-09; v2.1's Discord-tied ownership (LINK-02) further dovetails with it.
- **999.19** Google OAuth brand verification re-approval — **SUPERSEDED by v2.0** (Google removed from the system entirely; see STATE.md). Retained for incident-trail linkage only.
- **999.20** `console_windows.go` not `gofmt -l` clean — RESOLVED in v2.0 Plan 13-04 (`c930fc2`); ✅ **CLOSED in v2.1 Phase 18 / WATCH-12** (confirmed gofmt-clean live).
- **999.21** `freeConsole()` doc-vs-impl contract mismatch (log noise) — RESOLVED in v2.0 Plan 13-04 (`c930fc2`); ✅ **CLOSED in v2.1 Phase 18 / WATCH-13** (confirmed `slog.Debug` not Warn; doc/impl reconciled).
- **999.22** SemVer-aware auto-update comparison — RESOLVED in v2.0 Plan 13-04 (`e758fb0`/`3e8e53b`); ✅ **CLOSED in v2.1 Phase 18 / WATCH-14** (confirmed `IsNewer` pre-release test green). Stuck-watcher reinstall was a stale premise — the `0.4.0-rc1` box is the disposable Azure test VM, not a production watcher; no reinstall needed.
- **999.23** Graceful tray messaging for Google policy/verification block — largely mooted by P13 (the Google OAuth path is deleted); the tray-classifier UX pattern may inform the bearer-token-rejected path.
- **999.24** `COL_RACE`/`COL_COUNT` collision (Sheet `showCharInfoSidebar.ts`) — mooted by the frontend rebuild.
- **999.25** Orphaned `squirebot:search:recent` CacheService key (Sheet) — mooted by decommission.
- **999.26** `evictionSidebar.inline.test.ts` bypasses admin gate at inline-JS layer (Sheet) — mooted by decommission.
- **999.27** `showSearchSidebar.test.ts` narrow negative assertion (Sheet) — mooted by decommission.
- **999.28** `searchIndex.ts` `didYouMean('')` contract bug — **port-relevant**: the search logic ports to the frontend in P14 (WEB-03); fix the empty-query contract during the port.
- **999.29** `test-helpers.ts` CacheService mock TTL nit (Sheet) — mooted by decommission.
- **999.30** `searchIndex.test.ts` Test 4 `didYouMean` Levenshtein contract mismatch — **port-relevant**: resolve when porting `didYouMean` to the frontend in P14 (WEB-03).
- **999.31** Self-service **"Link your watcher via Discord"** onboarding — ✅ **SHIPPED in v2.1 Phase 17 (LINK-01..06), 2026-06-02.** Guildie logs into squirebot.quest with the P15 Discord login → a "Link my watcher" action mints a guild code tied to their Discord identity → paste once into the watcher. Replaces the maintainer manually minting + DMing ~12 codes (no plaintext through the maintainer's hands; unifies web + watcher identity; self-service scales as the guild grows). **HARD CONSTRAINT:** the watcher credential stays a static bearer token — do NOT put Discord OAuth *in the watcher* (that reintroduces the exact v2.0 "Off Google" fragility: ~7-day token expiry/refresh on an unattended uploader, a public desktop client secret, a browser/loopback flow; P13 made the watcher browser-free on purpose). Discord is the identity at **link-time only**.
- **999.34** Phase 28 deferred cosmetic review items (from `28-REVIEW.md`, all **LOW/NIT, non-blocking**, captured at v2.3 audit 2026-06-09): **LOW-01** — a forged-tag `403` (tagging a character not yours) surfaces a generic add-form error rather than a specific message; reachable only via a tampered request since the `<select>` only offers the caller's own characters. **LOW-03** — account-level wantlist adds write `"character_id":null` into the audit-log detail (cosmetic audit noise; account-level is the common case). **NIT-01/02** — minor naming/comment nits. Also the Phase 27 zero-claimed-characters hint affordance (#6) + the Phase 26 non-officer `/admin` 403 UI-collapse remain un-browser-smoked (logic/server-gate code-verified; need specific account states). Batch-fix in a future polish pass; none affect correctness, security, or data.
- **999.33** Officer panel — guild-bank/bot designation is a one-way door in the UI (Phase 26 browser-smoke UAT, 2026-06-08). **MEDIUM, no data loss.** ✅ **RESOLVED + DEPLOYED LIVE 2026-06-09 (quick `260609-d2o`, commits b7b0a48/2b18d65; binary+web swap, new route 401-gated verified)** — added a `ListDesignatedChars` read + `GET /api/v1/admin/characters/designated` (officer-only) + a "Designated characters / Clear designation" section in `AssignmentAdminPanel`, so an officer can return a bank/bot char to `mode:none` from the UI. Original problem: `DesignateCharTx` clears the `character_assignment` row, so a designated char drops out of `ListAllAssignments` — the ONLY list `AssignmentAdminPanel` renders, and the host of the per-row Designate (bank/bot/none) + Reassign/Remove controls. It's also excluded from member claimable, so bank/bot is a UI one-way door (data layer correct/reversible; fix = surface designated chars in the panel). Relates to Phase 26 (v2.3); good candidate to fold into Phase 27/28 web work or fix before v2.3 close.
- **999.32** Single-bank-toon invariant for the char-meta form (Phase 16 code-review **MD-01**) — ✅ **RESOLVED.** `SetCharMetaTx` (`internal/backendsrv/store/charmeta.go`) enforced no uniqueness on `is_bank_toon`, but `compute.Bank` assumes exactly one bank toon; flagging 2+ silently merged bank-view rows. **Fixed in commit `0e31023`:** setting `is_bank_toon=true` now clears it on all other characters in the same tx (matches v1's single-value `_meta.bank_toon_name`) + store regression tests. The same code review's route-gate test gap (**LR-01**) was fixed in `9b608a4`. The originally-bundled **LO-01** (level JSON-null contract — Go `int64` can't emit `null` vs TS `number | null`) and **LO-02** (empty-name success copy on read-back failure) were reclassified as Info/parity in the independent re-review and intentionally left as-is. Full findings in `16-REVIEW.md` / `16-REVIEW-FIX.md`.
- **999.35** Owner-less / eviction-safe guild banks & bots — **✅ PROMOTED to milestone v2.5 / Phase 35 (2026-06-22).** Deferred from quick `260621-u6j` (the cross-owner write-gate relaxation, deployed live 2026-06-22). A guild bank (e.g. Findom) currently gets owned by whoever uploads it first, so an officer must "claim" a character to designate it as a bank, tying the bank to that person's owner record. Decouple designated banks/bots from single ownership — a reserved "guild" sentinel owner or nullable `owner_id` (note `character.owner_id` is `NOT NULL REFERENCES owner(id)` → needs a small schema migration) — so designating never requires claiming and the bank is conceptually guild property. Backend-only; **tackle with 999.36** (both reconcile post-relaxation ownership semantics). Context: memory `cross-owner-character-misbinding.md`, `.planning/quick/260621-u6j-*`.
- **999.36** Owner-scoped eviction over-deletes shared characters — **✅ PROMOTED to milestone v2.5 / Phase 36 (2026-06-22).** Deferred from quick `260621-u6j`. Eviction is `UPDATE character SET is_removed=1 … WHERE owner_id=?` (`internal/backendsrv/store/eviction.go`). Now that characters are shared across guildies (shared P99 logins) but still carry a single first-uploader `owner_id`, evicting that member would also remove/archive shared + bank characters other guildies rely on. Rework eviction so removal isn't keyed on the first-uploader `owner_id` (tie it to assignment or an explicit per-character action). Backend-only; **tackle with 999.35**.
- **999.37** ✅ **RESOLVED + DEPLOYED LIVE — quick `260624-mdp`, 2026-06-24** (commit `470b670`; `EvictionPreviewHandler` now mirrors the action's sentinel + owner-floor guards, 3 regression tests). Eviction *preview* lacks the floor/sentinel guards the evict *action* enforces (Phase 36 code-review **WR-01**, non-blocking) — `EvictionPreviewHandler` (`internal/backendsrv/webadmin/eviction.go`) returns an owner's roster without applying the owner-floor / `GuildSentinelOwnerID` guards that `EvictHandler`/`EvictOwnerTx` do, so a peer/officer can preview a floor-protected owner's characters (read-only info-disclosure asymmetry). Pre-existing — Phase 36 only narrowed the preview's *content*. Fix: mirror the action's floor/sentinel guards in the preview handler. Backend-only; no data risk.
- **999.38** ✅ **RESOLVED + DEPLOYED LIVE — quick `260624-mdp`, 2026-06-24** (commit `71fed2c`; `recentOtherSharerSubquery` gains a live-steward `EXISTS` filter, drift-lock re-scoped, +`TestEvictOwnerTx_RepointSkipsEvictedSharer`). Eviction steward-repoint can target an already-evicted sharer (Phase 36 code-review **WR-02**, cosmetic) — the D-03 `recentOtherSharerSubquery` repoint in `EvictOwnerTx` picks the most-recent *other* `cross_owner_write` uploader without excluding evicted/in-grace owners, so a surviving shared char can be repointed to an already-evicted steward. Cosmetic only (`owner_id` is non-binding; the char survives `is_removed=0` and is never archived) but it's the stale-attribution outcome D-03 meant to avoid. Fix: add `AND EXISTS (SELECT 1 FROM character c2 WHERE c2.owner_id = a.attempting_owner_id AND c2.is_removed = 0)` to the subselect (note: diverges it from `sharedCharPredicate` → re-scope the `TestRepointSubquery_LocksPredicateToSharedPredicate` drift-lock). Backend-only.
- **999.39** ✅ **RESOLVED + DEPLOYED LIVE — quick `260624-mdp`, 2026-06-24** (commits `d98da18`+`470b670`; `store/linking.go`, `auth/store.go`, and the `webadmin/eviction.go` floor bridge exclude the sentinel by id). Label-bridge lookups can resolve the reserved `'guild'` sentinel label (Phase 35 code-review **IN-02**, astronomically unlikely) — owner-by-label `SELECT`s in `store/linking.go`, `auth/store.go`, and the `eviction.go` label bridge can match the sentinel owner (id 1000000, label `guild`, seeded by migration 00015) IF a guildie's Discord username / mint `--owner` arg were ever literally "guild". Newly reachable since the reserved label was introduced. Fix: exclude `o.id <> store.GuildSentinelOwnerID` (or `label <> 'guild'`) from those label SELECTs. Backend-only; defense-in-depth.
- **999.40** ✅ **ALREADY RESOLVED by quick `260621-px6` (2026-06-21) — this backlog entry was STALE** (logged 2026-06-22 during the v2.5 close from the pre-px6 MD-01 description, without reconciling that px6 had already landed). Verified 2026-06-24: the 5 wrappers + `WantlistRow`/`GuildWantRow` interfaces are gone (only a documentary `api.ts` comment remains explaining the removal), `WantAddForm`/`WantMuteCell`/`WantItemCell`/`WantRemoveCell`/`InGuildCell` deleted, `web/src/lib/wantlist/` removed, `api.test.ts` stale assertions gone, zero dangling imports/usage. The `/wantlist` route is an INTENTIONAL 308→`/wishlist` redirect stub (keeps old bookmarks alive — not dead code). Dead wantlist client/server divergence (v2.4 Phase 34 code-review **MD-01**, carried from the v2.4 close) — `web/src/lib/api.ts` retains 5 orphaned wantlist fetch wrappers (`fetchOwnWants`/`fetchGuildWants`/`addWant`/`removeWant`/`muteWant`) pointing at the deleted `/api/v1/wantlist*` routes (the P34 clean break dropped both the routes and the `wantlist_item` table), still imported by the now-orphaned `WantAddForm.svelte` / `WantMuteCell.svelte` cluster + a stale `api.test.ts` asserting `/wantlist/mute`. Latent dead code (the `/wantlist` route is a redirect stub → unreachable) but a real client→deleted-server divergence. Fix: delete the wrappers + the orphaned component cluster + the stale test (an entangled cleanup). Web-only; good for a `/gsd-quick`.
- **999.42** ~~Phase 38 code-review residual NIT (NIT-02 — `normEnrich` test-helper sync comment)~~ — **✅ DISSOLVED 2026-06-25 by the Phase 38 name-keyed re-plan.** The NIT referenced the Option-A `normEnrich` helper / `38-REVIEW.md`, both of which are retired as part of the D-04 reversal (id-keyed → name-keyed); the entire Option-A test surface it pointed at is being replaced. No standalone follow-up remains.
- **999.43** Phase 38 (name-keyed) code-review cosmetic INFOs (non-blocking, deferred 2026-06-25 — `38-REVIEW.md`): **IN-02** — `UpsertCatalogEnrichmentTx` (`store/catalogenrich.go:115`) emits a `slog.Error` AND returns a wrapped error, and the job caller (`wiki.go`) then `slog.Warn`s the same failure — one DB failure logs twice; the held path (`UpsertItemMasterTx`) does NOT self-log, so drop the inner `slog.Error` (note: removing it orphans the `log/slog` import — drop that too) to mirror the held path exactly. **IN-03** — `ItemID: int(ref.ItemID)` (`wiki.go:314`) narrows an `int64` to `CatalogEnrichment.ItemID int` (no truncation on the 64-bit target; identical to the held path at `wiki.go:361`); optionally make `CatalogEnrichment.ItemID` an `int64` to drop the conversion. Both cosmetic, no correctness impact; a future `/gsd-quick`. (WR-01/WR-02 + IN-01 were fixed-forward in Phase 38 itself — `9735f59`/`c7dae81`; IN-04 = no-action intentional scope.)
- **999.41** Phase 37 code-review LOW/NIT cleanups (non-blocking, deferred 2026-06-24 — `37-REVIEW.md`): **LW-01** the boot item-flags backfill runs on `context.Background()` so it's uninterruptible by SIGTERM (a long backfill could delay shutdown — wire it to the server's root ctx); **LW-02** `GetItemMasterSHA1Tx` is now production-dead, superseded by `GetItemMasterFreshnessTx` (delete it); **NIT-01** `parseClicky` silently drops trailing text after the `(...)` qualifier; **NIT-02** `parseHastePct` accepts a nonsense `+-`/`-+` sign-prefix combo. All cosmetic/maintainability; the substantive MD-01 finding was fixed-forward in Phase 37 itself. Backend-only; a future `/gsd-quick`.
- **999.44** Phase 39 code-review advisory web findings (non-blocking, deferred 2026-06-26 — `39-REVIEW.md`): **MD-01** a pending catalog-search debounce timer survives a Holdings↔Catalog scope flip, so a stray catalog fetch can fire after switching to Holdings (clear the timer in `setScope`); **MD-02** an unheld catalog row sets `price` from an empty `prices` array (display edge); **LO-01** dead `CatalogItem.is_clicky?/has_haste?` optional fields in `api.ts` (the catalog facet is server-side; the web never reads them — delete); **LO-02** no "searching…" affordance during the debounced catalog fetch; **LO-03** reactive-state param shadowing in `runCatalogSearch`. All web-only UX/cleanup, no crash; a future `/gsd-quick` (would need a web re-deploy). (The BLOCKER BL-01 — catalog facet LEFT JOIN row fan-out on same-name held items — was fixed-forward in Phase 39 itself: `bda8289`, GROUP BY norm + MAX(flag) + `TestSearchCatalog_SameNameHeldNoFanout`, backend re-deployed.)
- **999.45** Phase 40 code-review Info NITs (non-blocking, deferred 2026-07-15 — `40-REVIEW.md`; the review found 0 BLOCKER/0 HIGH/0 MEDIUM): **IN-01** the examine `{#each f.quests}` template calls `safeHttpUrl(q.source_url)` twice per link (once in the `{#if}`, once in the `href`) — hoist to a single `{@const safeUrl = safeHttpUrl(q.source_url)}` (pure fn → no correctness impact, just redundant per-render work; `web/src/lib/components/ExaminePanel.svelte`); **IN-03** the app.css ↔ themes.ts `--flag-*` hex lockstep has NO automated parity guard (they match byte-for-byte today, but a future one-sided edit wouldn't fail a test) — add a regex-extract parity test mirroring the `REQUIRED_TOKENS` loop; **IN-04** the `ItemMasterIconStats`/`IconStats` doc comment (`internal/backendsrv/store/readviews.go:787-790`) is stale — still says "icon_id + statsblock", now also returns the five `item_master` flag columns. (IN-02 = NO ACTION — both the examine native `<a>` and the composeNotes tooltip `<a>` already carry `target="_blank" rel="noopener"`; optional `noreferrer` only.) All cosmetic/defense-in-depth; a future `/gsd-quick` (IN-01/IN-03 web, IN-04 backend).

---

*Roadmap created: 2026-04-30. v1.0 shipped: 2026-05-11. v1.0.1 shipped: 2026-05-12. v1.0.2 binary shipped: 2026-05-13. v2.0 "Off Google" shipped 2026-05-31, Phases 11–16; milestone archived (`milestones/v2.0-ROADMAP.md`). v2.1 "Self-Service Watcher Linking" shipped 2026-06-02 (Phases 17–18). Last reorganized: 2026-06-02 — appended v2.2 "Wantlist + Discord Pinger" milestone (Phases 19–23); 8/8 v2.2 requirements mapped (Track 1 unblocked: WANT-01/02/03/04/05/08 across Phases 19–21; Track 2 invite-gated: WANT-06/07 across Phases 22–23); Phase 19 ready to plan. **2026-06-09: v2.3 "Character Assignment & Per-Character Wantlists" (Phases 26–28) SHIPPED + deployed live (schema v10); milestone audit PASSED (14/14 requirements; ASSIGN-01..06 → P26 · MYVIEW-01/02 → P27 · CWANT-01..06 → P28); detail collapsed + archived to `milestones/v2.3-{ROADMAP,REQUIREMENTS,MILESTONE-AUDIT}.md`. v2.2 Track 2 (Phases 22–23) remains PARKED/invite-gated. 2026-06-18: v2.4 Phase 31 PLANNED (4 plans / 4 waves; CHAR-01..03 + INV-01..04). 2026-06-24: v2.6 "Item Detail & Polish" (Phases 37–42) ROADMAP APPENDED — 13/13 requirements mapped (ENRICH-12/13 → P37 · ENRICH-14/15 → P38 · SEARCH-04/05/06 → P39 · ITEMUI-01/02 → P40 · CHARUI-01/02 → P41 · WISHUI-01/02 → P42); backend + web, watcher untouched, no `v*` tag; Phase 37 ready to plan.***
