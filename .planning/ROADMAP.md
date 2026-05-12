# Roadmap: SquireBot

**Core Value:** Every guildie can answer "what does my character still need, and where in the guild is it?" without leaving the spreadsheet.

## Milestones

- ✅ **v1.0** — Watcher + Workbook + Onboarding (initial release) — shipped 2026-05-11 as tag `v1.0.0`
- 🚧 **v1.0.1** — Installer + Permissions Hardening (in progress, started 2026-05-11)

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

### 🚧 v1.0.1 — Installer + Permissions Hardening (in progress)

- [x] **Phase 6: Installer Overwrite-Running Shim** — NSIS detects + gracefully stops a running watcher before file overwrite, retires manual stop workaround; ships as watcher v1.0.1 binary release. **SHIPPED + UAT-VERIFIED 2026-05-11** as [tag `v1.0.1`](https://github.com/boejowen/SquireBot/releases/tag/v1.0.1). Both graceful `--quit` and `taskkill /F` legacy paths exercised against real watchers on Azure VM; 8 findings recorded (3 are v1.0.2 candidates).
- [ ] **Phase 7: Admin Allowlist + Eviction Enforcement** — `_meta.guild_admins` row + bootstrap; eviction sidebar refuses non-admins by code; admin-management UX with owner-floor lockout protection
- [ ] **Phase 8: Test Infra + Persistence + Docs Backfill** — JSDOM in vitest + ≥1 sidebar inline-JS test per shipping sidebar; SEARCH recent-MRU migrated to PropertiesService; 8 missing Phase 3+4 SUMMARY.md files backfilled

## Phase Details

### Phase 6: Installer Overwrite-Running Shim
**Goal**: Guildies can re-run the installer over a running watcher and have it upgrade cleanly with no manual stop step
**Depends on**: Nothing (first phase of v1.0.1; v1.0 shipped foundation is intact)
**Requirements**: INST-06
**Success Criteria** (what must be TRUE):
  1. Running the v1.0.1 installer on a machine that already has the tray-app running upgrades the binary without prompting the user to quit the tray manually first
  2. NSIS pre-install step signals `squirebot.exe` to exit gracefully, waits for it, and falls back to a hard kill if the process does not exit within the timeout
  3. Post-install the new watcher binary autostarts (or is launched by the installer) and successfully resumes writes to the same workbook with no token re-auth required
  4. `docs/troubleshooting.md` no longer instructs users to manually stop the tray app before reinstalling (the manual workaround block is removed)
  5. Watcher binary v1.0.1 is built, tagged, and published on GitHub Releases; the `latest.json` manifest is updated so existing watchers can self-update to it
**Plans**: 5 plans
  - [x] 06-01-shutdown-signal-package-PLAN.md -- internal/system package: SignalShutdown + WaitForShutdown (Windows named event, paired build-tag files + Windows-only round-trip tests). Wave 1. **SHIPPED 2026-05-11** (commit `a705f4e`; 5/5 tests pass; SUMMARY at `.planning/phases/06-installer-overwrite-running-shim/06-01-SUMMARY.md`).
  - [x] 06-02-main-go-wiring-PLAN.md -- cmd/squirebot/main.go: --quit CLI flag handler + named-event listener goroutine wired through cancel()+systray.Quit(). Wave 2. **SHIPPED 2026-05-11** (commits `5256382` + `a36e72f`; 13 tests pass; SUMMARY at `.planning/phases/06-installer-overwrite-running-shim/06-02-SUMMARY.md`).
  - [x] 06-03-nsis-preinstall-shim-PLAN.md -- installer/squirebot.nsi: version-gated ExecWait --quit + 10s poll loop + taskkill /F fallback at top of Section Install. Wave 3. **SHIPPED 2026-05-11** (commit `9a179bd`; +99 NSIS lines; 14/14 acceptance greps PASS; NSIS local build deferred to CI; SUMMARY at `.planning/phases/06-installer-overwrite-running-shim/06-03-SUMMARY.md`).
  - [x] 06-04-docs-update-PLAN.md -- docs/troubleshooting.md: delete manual-stop section; docs/build-and-install.md: add Manual debug aids subsection documenting --quit. Wave 1 (parallel with Plan 01). **SHIPPED 2026-05-11** (commit `4465836`; SUMMARY at `.planning/phases/06-installer-overwrite-running-shim/06-04-SUMMARY.md`).
  - [x] 06-05-release-tag-PLAN.md -- git tag v1.0.1 + CI verification + GitHub Release smoke + end-to-end UAT (v1.0.0 -> v1.0.1 upgrade on clean Win11 VM). Wave 4 (ship gate). **SHIPPED 2026-05-11** as tag `v1.0.1` at commit `265bbd9` (CI run [25686757380](https://github.com/boejowen/SquireBot/actions/runs/25686757380), 1m55s success; Release [`v1.0.1`](https://github.com/boejowen/SquireBot/releases/tag/v1.0.1) with installer+binary+latest.json published; UAT deferred; SUMMARY at `.planning/phases/06-installer-overwrite-running-shim/06-05-SUMMARY.md`).
**Ship gate**: tag `v1.0.1` (watcher binary release)

### Phase 7: Admin Allowlist + Eviction Enforcement
**Goal**: Officer-only eviction is enforced by code (not by social convention), and admins can manage the allowlist without risk of locking themselves out
**Depends on**: Phase 6 (sequential by convention; no hard code dependency, but milestone ships as a single coherent v1.0.1 unit)
**Requirements**: ADMIN-01, ADMIN-02, ADMIN-03
**Success Criteria** (what must be TRUE):
  1. On first deploy after the v1.0.1 apps-script bundle lands, `_meta.guild_admins` exists and contains the workbook owner's email; re-running the bootstrap is idempotent (no duplicates)
  2. A non-admin guildie who opens `Evict Guildie…` sees a clear "not authorized" message; clicking the action is a safe no-op (no `_char_owner` write, no `_meta.eviction_log` envelope appended, no archive triggered)
  3. An admin can open the admin-management UI and add another guildie's email to the allowlist; the new admin's eviction calls succeed immediately on next sidebar invocation
  4. An admin can remove another admin from the allowlist, but the workbook-owner email cannot be removed by anyone other than the owner themselves (owner-floor lockout protection)
  5. `_meta.schema_version` remains at 3 and `WatcherMaxSchemaVersion` remains at 3 (the `guild_admins` row is an extend-only `_meta` addition; no migration, no watcher rebuild)
**Plans**: 3 plans
  - [x] 07-01-admin-policy-module-PLAN.md -- apps-script/src/lib/admin.ts policy primitives (normalizeEmail, getAdminList, isAdmin, requireAdminOrThrow, addAdmin, removeAdmin, bootstrapGuildAdmins, bootstrapGuildAdminsManual, appendAdminLogEntry) + 20-scenario vitest unit suite. Wave 1. **SHIPPED 2026-05-12** (commits `dfc3533` test + `8780222` feat; 357 LOC admin.ts + 462 LOC admin.test.ts; 20/20 tests PASS; full suite 297→317 GREEN; typecheck clean; verification hook 5 schema_version untouched; SUMMARY at `.planning/phases/07-admin-allowlist-eviction-enforcement/07-01-SUMMARY.md`).
  - [x] 07-02-admin-mgmt-sidebar-PLAN.md -- apps-script/src/triggers/showAdminMgmtSidebar.ts (300px HtmlService sidebar, 3 google.script.run callbacks, owner-floor enforcement) + 7-test vitest suite + Code.ts + build.mjs TRIGGER_GLOBALS re-exports for the 5 new globals. Wave 2 (parallel with 07-03). **SHIPPED 2026-05-12** (commits `2ea0cd2` trigger + `a5d3cb4` test+helpers + `d84d684` Code.ts+build wiring; 263 LOC sidebar + 219 LOC test; full suite 317→324 GREEN; typecheck + build clean; all 5 new globals lifted to dist/Code.js; verification hook 5 schema_version untouched; SUMMARY at `.planning/phases/07-admin-allowlist-eviction-enforcement/07-02-SUMMARY.md`).
  - [~] 07-03-eviction-guard-onopen-smoke-PLAN.md -- showEvictionSidebar admin guard (opener + 3 callbacks); onOpen lazy bootstrap + 2 new menu items; eviction-sidebar tests reseeded; clasp push + dev-workbook 5-hook smoke. Wave 2 (parallel with 07-02). Ship gate. **CODE-COMPLETE-PENDING-SMOKE 2026-05-12** (commits `1937fd0` test-seed + `7f7ffb0` eviction guards + `c3c0033` onOpen wiring; full suite 324/324 GREEN; typecheck + build clean; dist/Code.js contains all Phase 7 surface; awaiting user `clasp push` + 5-hook dev-workbook smoke before Phase 7 ship verdict; INTERIM SUMMARY at `.planning/phases/07-admin-allowlist-eviction-enforcement/07-03-SUMMARY.md`).
**UI hint**: yes
**Ship gate**: `clasp push` of the apps-script bundle to the dev workbook + admin-management UX smoke

### Phase 8: Test Infra + Persistence + Docs Backfill
**Goal**: The safety net under shipping sidebars is real (every sidebar has an executable test), recent-search MRU survives the CacheService TTL, and the Phase 3+4 documentation debt from v1.0 is closed
**Depends on**: Phase 7 (sequential by convention; TEST-02 tests cover the eviction + admin sidebars landing in Phase 7, so this phase comes after)
**Requirements**: TEST-01, TEST-02, SEARCH-05, DOC-04
**Success Criteria** (what must be TRUE):
  1. `npm test` exercises sidebar inline-JS via a JSDOM environment configured in `apps-script/vitest.config.ts`; no separate test command is required
  2. Each shipping sidebar (Search, Eviction, Bank-Coin, Char-Info, Theme Picker) has at least one co-located `__tests__/*-sidebar.test.ts` companion file that exercises real DOM event handlers + payload assembly + error display; the existing 297+/297+ vitest suite remains green
  3. A guildie's last 3 search queries persist across the CacheService 25-min TTL and across browser-tab restarts; reopening the Search sidebar the next day (or next week) still shows their recent queries because the MRU lives in per-user PropertiesService scope
  4. Every Phase 3 plan directory (4 plans) and every Phase 4 plan directory (4 plans) contains a `SUMMARY.md` following the Phase 5 template (Decisions, Outcomes, Deferred); the v1.0 milestone audit's documentation-debt entry can be retired
  5. `_meta.schema_version` remains at 3 and `WatcherMaxSchemaVersion` remains at 3 (no schema impact; persistence migration is a Properties read/write change, not a tab schema change)
**Plans**: TBD
**UI hint**: yes
**Ship gate**: `clasp push` of the apps-script bundle to the dev workbook + green CI + milestone retrospective (v1.0.1 ship)

## Progress

| Milestone | Phases | Plans Complete | Status | Completed |
|-----------|--------|----------------|--------|-----------|
| v1.0 | 5 | 31/31 | ✅ Shipped | 2026-05-11 |
| v1.0.1 | 3 | 7/8 (Phase 6 + Plans 07-01..02) | 🚧 In progress (1/3 phases shipped; Phase 7 in progress) | — |

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 6. Installer Overwrite-Running Shim | 5/5 | ✅ Shipped + UAT-verified as [tag `v1.0.1`](https://github.com/boejowen/SquireBot/releases/tag/v1.0.1) | 2026-05-11 |
| 7. Admin Allowlist + Eviction Enforcement | 2/3 | 🚧 In progress (Plans 07-01 + 07-02 shipped 2026-05-12) | — |
| 8. Test Infra + Persistence + Docs Backfill | 0/0 | Not started | — |

## Backlog

Carried forward from v1.0 (candidates for v1.1 / v2; see `milestones/v1.0-MILESTONE-AUDIT.md` § Tech Debt and `milestones/v1.0-ROADMAP.md` § Issues Deferred for context). Items pulled INTO v1.0.1 (INST-06, ADMIN-01..03, TEST-01/02, SEARCH-05, DOC-04) are no longer listed here.

- **999.1** Bank-coin permission lock (only bank-toon-owner can use Set Bank Coin sidebar) — Phase 4 deferred; v1.1 candidate (eviction enforcement was prioritized this cycle)
- **999.2** Polished theme picker tile UI (6-tile grid per `docs/design/mockups/eq-aesthetic-picker.html`) — Phase 4 deferred; aesthetics-only
- **999.5** Self-service eviction (departing guildie quits cleanly without officer action) — v2 candidate (threat-model deferred)
- **999.7** Extract `SIDEBAR_BODY` constants to `apps-script/src/sidebars/*.html` — uniform deferral across 05-03 + 05-04; cosmetic refactor
- **999.9** SignPath Foundation OSS approval — submitted; awaiting review (would retire INST-05 partial → full). Lands as hotfix when approved.
- **999.11** Decide v1.1+ verification doctrine — adopt `/gsd-verify-work` per phase, or formalize live-smoke pattern
- **999.12** v2: Wantlist + Discord pinger (WANT-01..08; prerequisites WANT-06/07 still open)
- **999.13** v1.0.2 candidate — Reauthorize tray item should unhide on boot-time `invalid_grant` (currently AUTH-05 covers running-state revocation only; boot-time revocation traps user with no in-tray recovery). Surfaced by Phase 6 UAT Finding C, 2026-05-11.
- **999.14** v1.0.2 candidate — Defer/queue tray controller `SetStatus`/`Show*`/`SetIconHealth` calls until `OnReady` fires, OR have `app.RunApp` retry on fast-fail path. Pre-Ready calls silently no-op when RunApp returns early via wincred-rebuild failure, stranding the user at "Initialising…" with no recovery menu items. Surfaced by Phase 6 UAT Finding D, 2026-05-11. Wider impact than the T-06-20 accept disposition covered.
- **999.15** v1.0.2 candidate — Strip leading UTF-8 BOM in `internal/config/load.go` before `json.Unmarshal`. ≤5 LOC; closes a foot-gun for users hand-editing `config.json` with Notepad or PowerShell 5.1 `Set-Content -Encoding utf8`. Surfaced by Phase 6 UAT Finding F, 2026-05-11.
- **999.16** v1.0.2 candidate — Either call `windows.FreeConsole()` early in `cmd/squirebot/main.go` to detach from any inherited console, OR document the `Start-Process` requirement prominently in `docs/build-and-install.md`. Foreground-launched watcher dies silently when parent shell closes (no `squirebot exit` log line). Surfaced by Phase 6 UAT Finding H, 2026-05-11.

---

*Roadmap created: 2026-04-30. v1.0 shipped: 2026-05-11. v1.0.1 phases planned: 2026-05-11. Last reorganized: 2026-05-11 (v1.0.1 phase structure added).*
