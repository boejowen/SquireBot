# Roadmap: SquireBot

**Core Value:** Every guildie can answer "what does my character still need, and where in the guild is it?" — now delivered via a self-hosted website instead of the Google Sheet.

## Milestones

- ✅ **v1.0** — Watcher + Workbook + Onboarding (initial release) — shipped 2026-05-11 as tag `v1.0.0`
- ✅ **v1.0.1** — Installer + Permissions Hardening — shipped 2026-05-12 (binary tag `v1.0.1`)
- ✅ **v1.0.2** — Robustness Polish — binary shipped 2026-05-13 (tag `v1.0.2`); milestone close superseded by v2.0
- ✅ **v2.0** — "Off Google" — Website Frontend — Phases 11–16 (shipped 2026-05-31 as tag `v2.0.0`) — archive: [`milestones/v2.0-ROADMAP.md`](milestones/v2.0-ROADMAP.md)
- 🔄 **v2.1** — Self-Service Watcher Linking — opened 2026-06-01; Phases 17–18 (LINK-01..06 + WATCH-12/13/14)

## Phases

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

## v2.1 — Self-Service Watcher Linking (Phases 17–18)

**Milestone Goal:** Let any guildie link their own watcher from squirebot.quest via Discord login — no maintainer hand-minting + DMing codes — while keeping the watcher credential a static, reusable bearer token (no watcher change).

### Phase 17: Self-Service Watcher Linking (web feature)
**Goal**: A signed-in guildie can mint, view, and revoke their own watcher codes from squirebot.quest — owner derived from their Discord session — with no maintainer involvement and no watcher change.
**Depends on**: v2.0 Phase 15 (Discord OAuth2 login + sessions + per-user Discord identity) and Phase 11 (`auth.MintCode` hashed-token minting + `guild_code`/`owner`/`web_user` tables). No intra-milestone dependency.
**Requirements**: LINK-01, LINK-02, LINK-03, LINK-04, LINK-05, LINK-06
**Success Criteria** (what must be TRUE):
  1. A signed-in guildie can click "Link my watcher" and receive a freshly minted code, with the owner derived server-side from their Discord session (never free-typed or client-supplied).
  2. The page shows the new code's plaintext exactly once with copy-to-clipboard and clear paste-into-watcher instructions; reloading or revisiting never re-reveals it (hash-only at rest).
  3. Minting again issues an additional valid code without invalidating any existing one — a guildie running watchers on two PCs sees both keep uploading.
  4. A guildie can see a list of their own active codes (each identifiable by label / created date / last-seen) and revoke any single one; the revoked watcher stops uploading while the others continue.
  5. The minted code's owner is tied to the guildie's Discord identity (`web_user`), so the website's eviction/owner-floor path resolves ownership through that link rather than the loose `owner.label == web_user.username` string match.
  6. The `mint-code` CLI subcommand no longer exists; the self-service endpoint is the only way to mint a watcher code (the `revoke-code` CLI is retained as an ops backstop).
**Plans**: 3 plans
- [ ] 17-01-PLAN.md — Foundation: 00005 migration (owner.discord_user_id FK + guild_code.last_seen) + bearer-guard codeID thread + ingest last-seen stamp
- [ ] 17-02-PLAN.md — Backend: resolve-or-create-owner + MintCodeForOwnerTx + the 3 login-only account handlers + routes + D-05 eviction-floor FK rewire + mint-code CLI removal
- [ ] 17-03-PLAN.md — Frontend: /account page + WatcherCodesPanel (show-once copy-to-clipboard + list + confirm-revoke) + api.ts wrappers + Account nav (browser-smoke checkpoint)
**UI hint**: yes

### Phase 18: Watcher Cleanups — Verify-or-Close
**Goal**: Confirm the live watcher state and close any residual from the v2.0 carry-forward cleanups (gofmt, freeConsole log noise, SemVer-aware auto-update).
**Depends on**: none (independent watcher cleanups).
**Requirements**: WATCH-12, WATCH-13, WATCH-14
**Plans**: TBD

## Progress

**Execution Order:** Phases execute in numeric order: 11 → 12 → 13 → 14 → 15 → 16.

| Milestone | Phases | Plans Complete | Status | Completed |
|-----------|--------|----------------|--------|-----------|
| v1.0 | 5 | 31/31 | ✅ Shipped | 2026-05-11 |
| v1.0.1 | 3 | 12/12 | ✅ Shipped | 2026-05-12 |
| v1.0.2 | 2 | 8/8 | ✅ Binary shipped (milestone close superseded by v2.0) | 2026-05-13 |
| v2.0 | 6 | 29/29 | ✅ Shipped (tag `v2.0.0`; Google decommissioned) | 2026-05-31 |

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 11. Backend Foundation + Ingest API | v2.0 | 7/7 | ✅ Complete | 2026-05-29 |
| 12. Enrichment Job Migration | v2.0 | 5/5 | ✅ Complete | 2026-05-29 |
| 13. Watcher Re-Target + Onboarding | v2.0 | 4/4 | ✅ Complete | 2026-05-30 |
| 14. Web Frontend | v2.0 | 4/4 | ✅ Complete (deployed live) | 2026-05-30 |
| 15. Admin Web Forms + Login | v2.0 | 5/5 | ✅ Complete (deployed live; most UAT smokes exercised during the 16-03 deploy) | 2026-05-31 |
| 16. Cutover + Decommission | v2.0 | 4/4 | ✅ Complete (Google decommissioned; guild migrating) | 2026-05-31 |

## Backlog

Carried forward from v1.0 / v1.0.1 / v1.0.2 (candidates for a future Sheet-orthogonal patch or v2.x). Note: several Sheet-side items below are likely **mooted by v2.0** (the Sheet + Apps Script are being decommissioned in Phase 16) — they are retained here for the record and to assess at v2.0 close.

- **999.1** Bank-coin permission lock (Sheet sidebar) — likely mooted by ADMIN-05 (web form replaces the sidebar).
- **999.2** Polished theme picker tile UI (Sheet) — mooted by WEB-05 (theme becomes a per-user client preference on the website).
- **999.5** Self-service eviction (departing guildie quits cleanly without officer action) — v2.x candidate; threat-model deferred.
- **999.7** Extract `SIDEBAR_BODY` constants to external `.html` (Sheet) — mooted by the frontend rebuild.
- **999.9** SignPath Foundation OSS approval — submitted; awaiting review (would retire INST-05 partial → full). Lands as a hotfix when approved; orthogonal to the backend swap.
- **999.11** Decide v2.x verification doctrine — adopt `/gsd-verify-work` per phase, or formalize a live-smoke pattern.
- **999.12 / WANT-01..08** v2: Wantlist + Discord pinger — prerequisites WANT-06/07 still open (Raid Alliance Discord bot invites). v2.0 pre-pays the per-user Discord-identity prerequisite via AUTH-09.
- **999.19** Google OAuth brand verification re-approval — **SUPERSEDED by v2.0** (Google removed from the system entirely; see STATE.md). Retained for incident-trail linkage only.
- **999.20** `console_windows.go` not `gofmt -l` clean — one-line fix; watcher carries into P13.
- **999.21** `freeConsole()` doc-vs-impl contract mismatch (log noise) — watcher carries into P13.
- **999.22** SemVer-aware auto-update comparison (`internal/update/check.go`) — relevant to P13/P16 (the coordinated self-update flip relies on the updater; pre-release sort safety).
- **999.23** Graceful tray messaging for Google policy/verification block — largely mooted by P13 (the Google OAuth path is deleted); the tray-classifier UX pattern may inform the bearer-token-rejected path.
- **999.24** `COL_RACE`/`COL_COUNT` collision (Sheet `showCharInfoSidebar.ts`) — mooted by the frontend rebuild.
- **999.25** Orphaned `squirebot:search:recent` CacheService key (Sheet) — mooted by decommission.
- **999.26** `evictionSidebar.inline.test.ts` bypasses admin gate at inline-JS layer (Sheet) — mooted by decommission.
- **999.27** `showSearchSidebar.test.ts` narrow negative assertion (Sheet) — mooted by decommission.
- **999.28** `searchIndex.ts` `didYouMean('')` contract bug — **port-relevant**: the search logic ports to the frontend in P14 (WEB-03); fix the empty-query contract during the port.
- **999.29** `test-helpers.ts` CacheService mock TTL nit (Sheet) — mooted by decommission.
- **999.30** `searchIndex.test.ts` Test 4 `didYouMean` Levenshtein contract mismatch — **port-relevant**: resolve when porting `didYouMean` to the frontend in P14 (WEB-03).
- **999.31** Self-service **"Link your watcher via Discord"** onboarding — ⭐ **TOP PRIORITY / first feature after the v2.0 cutover** (user, 2026-05-31). Guildie logs into squirebot.quest with the P15 Discord login → a "Link my watcher" action mints a guild code tied to their Discord identity → paste once into the watcher. Replaces the maintainer manually minting + DMing ~12 codes (no plaintext through the maintainer's hands; unifies web + watcher identity; self-service scales as the guild grows). **HARD CONSTRAINT:** the watcher credential stays a static bearer token — do NOT put Discord OAuth *in the watcher* (that reintroduces the exact v2.0 "Off Google" fragility: ~7-day token expiry/refresh on an unattended uploader, a public desktop client secret, a browser/loopback flow; P13 made the watcher browser-free on purpose). Discord is the identity at **link-time only**. Scope: a session-scoped, self-service `mint-code` backend endpoint + a frontend "link your watcher" page (ideally tying the minted owner to the Discord identity / owner-floor) + redeploy + tests. Dovetails with **999.12** (v2 Wantlist/Discord-pinger) — AUTH-09 already pre-paid the Discord-identity prerequisite.
- **999.32** Single-bank-toon invariant for the char-meta form (Phase 16 code-review **MD-01**) — ✅ **RESOLVED.** `SetCharMetaTx` (`internal/backendsrv/store/charmeta.go`) enforced no uniqueness on `is_bank_toon`, but `compute.Bank` assumes exactly one bank toon; flagging 2+ silently merged bank-view rows. **Fixed in commit `0e31023`:** setting `is_bank_toon=true` now clears it on all other characters in the same tx (matches v1's single-value `_meta.bank_toon_name`) + store regression tests. The same code review's route-gate test gap (**LR-01**) was fixed in `9b608a4`. The originally-bundled **LO-01** (level JSON-null contract — Go `int64` can't emit `null` vs TS `number | null`) and **LO-02** (empty-name success copy on read-back failure) were reclassified as Info/parity in the independent re-review and intentionally left as-is. Full findings in `16-REVIEW.md` / `16-REVIEW-FIX.md`.

---

*Roadmap created: 2026-04-30. v1.0 shipped: 2026-05-11. v1.0.1 shipped: 2026-05-12. v1.0.2 binary shipped: 2026-05-13. v2.0 "Off Google" shipped 2026-05-31, Phases 11–16; milestone archived (`milestones/v2.0-ROADMAP.md`). Last reorganized: 2026-05-31 — v2.0 collapsed into `<details>`; 26/26 v2.0 requirements shipped; Google fully decommissioned.*
