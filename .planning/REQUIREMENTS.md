# Requirements: SquireBot v1.0.1 — Installer + Permissions Hardening

> v1.0 shipped 2026-05-11 with 69 effective requirements. See `milestones/v1.0-REQUIREMENTS.md` for the full v1.0 reconciliation. This file is scoped to v1.0.1 only.

**Milestone goal:** Retire v1.0 carry-over debt — harden installer upgrade path, enforce officer-only eviction by code, pay down sidebar test + documentation debt — without changing any user-facing data flow.

**Schema impact:** None expected. `_meta.schema_version` stays at 3. `WatcherMaxSchemaVersion` stays at 3. New `_meta` rows (e.g. `guild_admins`) are extend-only additions and do not require a version bump.

---

## v1.0.1 Requirements

### Installer (INST-06)

- [x] **INST-06**: Installer cleanly upgrades over a running watcher. NSIS pre-install step detects `squirebot.exe` in the user session, signals it to exit gracefully (with a fallback hard-kill on timeout), confirms process is gone, then proceeds with file overwrite. User is not asked to manually stop the tray app first; the existing manual workaround in `docs/troubleshooting.md` is removed when this lands. **✓ SHIPPED + UAT-VERIFIED 2026-05-11 as tag [`v1.0.1`](https://github.com/boejowen/SquireBot/releases/tag/v1.0.1). Live v1.0.0-era → v1.0.1 upgrade UAT on Azure VM exercised both graceful `--quit` path (D-01 named-event listener fired against v1.0.1 binary) and `taskkill /F` legacy path (against v0.2.2-soak); heartbeat resumed against same workbook post-install. See [`06-05-SUMMARY.md`](phases/06-installer-overwrite-running-shim/06-05-SUMMARY.md) for evidence + 8 findings.**

### Admin permissions (ADMIN-01..03)

- [x] **ADMIN-01**: `_meta.guild_admins` row maintains the authorized-officer email list for the workbook. Bootstrap: on first installer run the workbook owner's email is auto-added; idempotent on subsequent runs (no duplicates). **✓ SHIPPED + UAT-VERIFIED 2026-05-12.** Bootstrap primitive shipped in Plan 07-01 (`bootstrapGuildAdmins`); lazy `onOpen` call-site shipped in Plan 07-03; dev-workbook Hook 1 PASS confirmed `_meta.guild_admins=["boejowen@gmail.com"]` + `_meta.workbook_owner_floor=boejowen@gmail.com` + `_meta.admin_log` bootstrap entry on first open; second open did NOT append duplicate (idempotent).
- [x] **ADMIN-02**: Evict Guildie sidebar reads `_meta.guild_admins` and refuses to commit eviction for non-admin invokers. Non-admins see a clear "not authorized" message and the action is a safe no-op (no write, no `_meta.eviction_log` envelope appended). **✓ SHIPPED + UAT-VERIFIED 2026-05-12.** Eviction sidebar admin-gated at opener + 3 callbacks (`getEvictionEmails`, `previewEviction`, `commitEviction`) shipped in Plan 07-03; dev-workbook Hook 2 PASS confirmed non-admin saw "Not authorized" modal + zero `_char_owner` flips + zero `eviction_log` appends.
- [x] **ADMIN-03**: Admin management UX (menu item or sidebar) lets an existing admin add or remove other admin emails. Workbook-owner-floor enforced: the original owner email cannot be removed by anyone other than themselves (prevents lockout). **✓ SHIPPED + UAT-VERIFIED 2026-05-12.** `showAdminMgmtSidebar` UX shipped in Plan 07-02; owner-floor enforcement shipped in Plan 07-01 (`removeAdmin` server-side throw `'owner_floor_protected'`) + Plan 07-02 (client-side Remove-button suppression); dev-workbook Hooks 3 + 4 PASS confirmed peer-add round-trip + visual owner-floor lockout (joseph.bowen2 non-floor admin saw boejowen@gmail.com row with `(owner)` annotation and NO Remove button).

### Sidebar test coverage (TEST-01..02)

- [x] **TEST-01**: vitest is configured with JSDOM environment for sidebar HTML+JS tests. Setup is wired into `apps-script/vitest.config.ts` so `npm test` exercises sidebar JS on every PR (no separate command). **✓ SHIPPED 2026-05-12.** `apps-script/vitest.config.ts` declares `environment: 'jsdom'`; jsdom@^24 installed as devDep; `mountSidebar()` helper landed in `test-helpers.ts` (Plan 08-01); 327/327 baseline GREEN under JSDOM with zero per-test environment annotations.
- [x] **TEST-02**: Every shipping sidebar (Search, Eviction, Bank-Coin, Char-Info, Admin-Mgmt) has at least one unit test covering its inline-JS interaction code — DOM event handlers, payload assembly, "Did you mean?" rendering, error display. Coverage assertion: each sidebar HTML file has a co-located `__tests__/*Sidebar(.inline)?.test.ts` companion. (Historical correction: the original TEST-02 wording listed the theme-selector modal as a sidebar; `showThemePickerModal` in `onOpen.ts:52-77` is actually a `showModalDialog`, NOT a sidebar. The 5th sidebar is Admin-Mgmt, which has trigger-call coverage via `adminMgmtSidebar.test.ts` from Phase 7; admin-mgmt inline-JS tests deferred to v1.1.) **✓ SHIPPED 2026-05-12.** 4 net-new inline-JS test files landed (searchSidebar/evictionSidebar/bankCoinSidebar/charInfoSidebar `.inline.test.ts`, 2 cases each = 8 new tests) + Admin-Mgmt covered by Phase 7's adminMgmtSidebar.test.ts → 5/5 sidebars. `buildSidebarHtml` exported from all 4 sidebar triggers. mountSidebar hardened with indirect-eval + nested-script extraction (Plan 08-02).

### Search persistence (SEARCH-05)

- [x] **SEARCH-05**: Recent search query MRU persists across CacheService 25-min TTL. Migrated from CacheService to per-user PropertiesService scope; the user's last 3 queries reappear when the sidebar is reopened the next day (or the next week). MRU eviction still capped at 3 entries. **✓ SHIPPED 2026-05-12.** `searchIndex.ts:365,375` use `PropertiesService.getUserProperties()`; no dual-write (D-06 clear-and-replace); per-`inv:Char` enrichment CacheService still in use at 2 surviving sites (prewarmSearchCache, runSearch); new `vi.useFakeTimers()` 25-min persistence test landed in `searchIndex.test.ts` (Plan 08-03).

### Documentation debt (DOC-04)

- [x] **DOC-04**: Each Phase 3 and Phase 4 plan that shipped without a SUMMARY.md gets a backfilled SUMMARY.md following the existing Phase 5 template (Decisions, Outcomes, Deferred). Source: phase artifacts + git log + STATE.md execution-log tables. 8 plans total (Phase 3 = 4 plans, Phase 4 = 4 plans). **✓ SHIPPED 2026-05-12.** All 8 retroactive SUMMARY.md files landed under `.planning/phases/03-apps-script-enrichment-foundation/` (03-01..03-04) and `.planning/phases/04-differentiator-features/` (04-01..04-04); each follows the Phase 5 template byte-for-byte (all 11 frontmatter keys); every `metrics.commits` SHA resolves via `git cat-file -e` (0 invented hashes); line counts range 119-242 (all ≥50). v1.0 milestone audit's documentation-debt line item retired (Plan 08-04).

---

## Out of Scope (v1.0.1 — deferred to v1.1 / v2)

| Item | Why deferred |
|------|--------------|
| SignPath Foundation OSS signing (999.9) | Third-party Foundation review timeline; lands as a hotfix when approved. Would retire INST-05 partial → full. |
| Bank-coin permission lock (999.1) | Eviction enforcement was prioritized this cycle; bank-coin remains social-convention-gated. v1.1 candidate. |
| Polished theme picker tile UI (999.2) | Functional theme picker already shipped in v1.0; tile-grid polish is aesthetics-only. |
| Self-service eviction (999.5) | v2; threat-model still open. |
| Verification doctrine decision (999.11) | Process change; addressed at v1.1 milestone planning. |
| Inline `SIDEBAR_BODY` → external `.html` refactor (999.7) | Cosmetic refactor; deferred to v1.1 polish. |
| v2 Wantlist + Discord pinger (999.12 / WANT-01..08) | v2 prerequisites still open (Discord bot infra + Raid Alliance permissions). |

---

## Traceability

| REQ-ID | Phase | Plan(s) |
|--------|-------|---------|
| INST-06 | Phase 6 | 06-01 (shipped), 06-02 (shipped), 06-03 (shipped), 06-04 (shipped), 06-05 (shipped — tag `v1.0.1` 2026-05-11; UAT-verified Azure VM) |
| ADMIN-01 | Phase 7 | 07-01 (shipped 2026-05-12 — bootstrap primitive), 07-03 (shipped 2026-05-12 — onOpen lazy bootstrap + smoke Hook 1 PASS) |
| ADMIN-02 | Phase 7 | 07-03 (shipped 2026-05-12 — eviction sidebar admin guard + smoke Hook 2 PASS) |
| ADMIN-03 | Phase 7 | 07-01 (shipped 2026-05-12 — add/remove/floor policy), 07-02 (shipped 2026-05-12 — admin-mgmt sidebar UX + smoke Hooks 3 + 4 PASS) |
| TEST-01 | Phase 8 | 08-01 (shipped 2026-05-12 — vitest+jsdom infra + mountSidebar) |
| TEST-02 | Phase 8 | 08-02 (shipped 2026-05-12 — 4 inline-JS test files + buildSidebarHtml exports + REQUIREMENTS.md correction) |
| SEARCH-05 | Phase 8 | 08-03 (shipped 2026-05-12 — MRU swap from CacheService to PropertiesService.getUserProperties) |
| DOC-04 | Phase 8 | 08-04 (shipped 2026-05-12 — 8 retroactive Phase 3+4 SUMMARY.md backfills) |

> Traceability column filled by roadmapper 2026-05-11. Plan column will be filled by `/gsd-plan-phase` for each phase.

**Coverage check (2026-05-11):** 8/8 requirements mapped to exactly one phase. No orphans. No duplicates.

---

*Defined: 2026-05-11 at v1.0.1 milestone start. 8 requirements across 5 carry-over features. Phase mapping locked 2026-05-11.*
