# Roadmap: SquireBot

**Core Value:** Every guildie can answer "what does my character still need, and where in the guild is it?" without leaving the spreadsheet.

## Milestones

- ✅ **v1.0** — Watcher + Workbook + Onboarding (initial release) — shipped 2026-05-11 as tag `v1.0.0`
- ✅ **v1.0.1** — Installer + Permissions Hardening — shipped 2026-05-12 (binary tag `v1.0.1` pushed 2026-05-11 by Phase 6 ship gate)
- 🚧 **v1.0.2** — Robustness Polish — in progress (started 2026-05-12)

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
- [x] Phase 7: Admin Allowlist + Eviction Enforcement (3 plans) — shipped + UAT-verified via dev-workbook 5-hook smoke, 2026-05-12
- [x] Phase 8: Test Infra + Persistence + Docs Backfill (4 plans) — shipped (verifier PASSED 5/5 must-haves; 336/336 vitest green), 2026-05-12

**Total:** 12 plans · 3 phases · 2 days kickoff to ship · 63 commits since v1.0.0.

</details>

### 🚧 v1.0.2 — Robustness Polish (in progress)

- [ ] **Phase 9: Watcher Robustness Polish** — close 4 v1.0.1-UAT-surfaced robustness gaps (boot-time `invalid_grant` Reauthorize recovery, tray controller pre-Ready call queue, UTF-8 BOM strip in config loader, foreground-shell-close silent death fix); ship as watcher v1.0.2 binary release. Requirements: AUTH-07, OPS-06, OPS-07, CONFIG-01. Ship gate: tag `v1.0.2` + GitHub Release + `latest.json` refresh.
- [ ] **Phase 10: Apps Script Test Quality** — close v1.0.1-Phase-8-review-surfaced test-quality items (Admin-Mgmt sidebar inline-JS coverage + `mountSidebar` realm leak + weak assertions + leaked pending mock call). Requirements: TEST-03, TEST-04. Ship gate: `clasp push` to dev workbook + green CI.

## Phase Details

### Phase 9: Watcher Robustness Polish
**Goal**: Eliminate the 4 v1.0.1-UAT-surfaced foot-guns in the Go watcher and ship a clean v1.0.2 binary release that's the new recommended download for every guildie
**Depends on**: v1.0.1 (Phase 6 binary release foundation; this milestone reuses the same release workflow + NSIS installer wrapper)
**Requirements**: AUTH-07, OPS-06, OPS-07, CONFIG-01
**Success Criteria** (what must be TRUE):
  1. A guildie whose refresh token was revoked between sessions (boot-time `invalid_grant`) sees a red tray icon AND a visible Reauthorize menu item from boot — clicking Reauthorize reopens the OAuth flow without restart (AUTH-07)
  2. A guildie whose wincred rebuild fails on boot (RunApp fast-fail path) sees a working tray menu, not "Initialising…" with no recovery path — either pre-Ready calls are queued and replayed once OnReady fires, or RunApp retries on the fast-fail path (OPS-06)
  3. A guildie who launches `squirebot.exe` from cmd.exe or PowerShell without `Start-Process` either has the watcher detach from the inherited console (FreeConsole path) OR finds the `Start-Process` requirement documented above the fold in `docs/build-and-install.md` (OPS-07)
  4. A guildie who hand-edits `%LOCALAPPDATA%\SquireBot\config.json` with Notepad or PowerShell `Set-Content -Encoding utf8` (both write a UTF-8 BOM by default) sees the watcher start normally — no `invalid character 'ï' looking for beginning of value` error from `json.Unmarshal` (CONFIG-01)
  5. Watcher binary v1.0.2 is built, tagged, and published on GitHub Releases; the `latest.json` manifest is updated so existing v1.0.1 watchers auto-update to it cleanly (binary release ship gate; reuses v1.0.1's release.yml workflow unchanged where possible)
**UI hint**: no (no apps-script work in this phase; all Go-side)
**Ship gate**: tag `v1.0.2` (watcher binary release) — mirrors v1.0.1 Phase 6 ship gate structure

### Phase 10: Apps Script Test Quality
**Goal**: Close v1.0.1's two remaining test-quality items so the apps-script suite has clean coverage for all 5 shipping sidebars and the advisory findings from Phase 8 code review are retired
**Depends on**: Phase 9 (sequential by convention; not blocked technically but milestone ships as a single v1.0.2 unit)
**Requirements**: TEST-03, TEST-04
**Success Criteria** (what must be TRUE):
  1. `apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts` exists and covers Admin-Mgmt sidebar inline-JS — DOM event handlers + `google.script.run` callback wiring + error-display path — mirroring the 4 inline-JS test files landed in v1.0.1 Plan 08-02 (TEST-03)
  2. `mountSidebar` no longer leaks JSDOM realm state across tests (cleanup hook OR test-isolation fix), and the v1.0.1 Phase 8 `08-REVIEW.md` 4 warning-level findings are addressed: weak assertions upgraded from `toBeTruthy`/`toBeDefined` to specific equality or structural matchers; leaked pending mock call in `searchSidebar.inline.test.ts` cleaned up (TEST-04)
  3. 5/5 shipping sidebars now have inline-JS test coverage (Search, Eviction, Bank-Coin, Char-Info from v1.0.1; Admin-Mgmt added this milestone); the v1.0.1 TEST-02 "admin-mgmt inline-JS tests deferred to v1.1" wording in `REQUIREMENTS.md` is retired
  4. Full apps-script vitest suite passes green (336 → ~340+; net positive; no test deletions); `npm run build` clean; typecheck clean
  5. `_meta.schema_version` remains at 3 and `WatcherMaxSchemaVersion` remains at 3 (no schema impact; test-only changes do not touch tab schemas)
**UI hint**: yes (sidebar test coverage; no UI changes)
**Ship gate**: `clasp push` of the apps-script bundle to the dev workbook + green CI

## Progress

| Milestone | Phases | Plans Complete | Status | Completed |
|-----------|--------|----------------|--------|-----------|
| v1.0 | 5 | 31/31 | ✅ Shipped | 2026-05-11 |
| v1.0.1 | 3 | 12/12 | ✅ Shipped | 2026-05-12 |
| v1.0.2 | 2 | 0/? | 🚧 In progress (plans defined per phase by `/gsd-plan-phase`) | — |

| Phase | Milestone | Status | Completed |
|-------|-----------|--------|-----------|
| 9. Watcher Robustness Polish | v1.0.2 | Not started | — |
| 10. Apps Script Test Quality | v1.0.2 | Not started | — |

## Backlog

Carried forward from v1.0 + v1.0.1 (candidates for v1.1 / v2). Items pulled INTO v1.0.2 (999.13–999.18) are no longer listed here; they live in `.planning/REQUIREMENTS.md` as AUTH-07, OPS-06, OPS-07, CONFIG-01, TEST-03, TEST-04.

- **999.1** Bank-coin permission lock (only bank-toon-owner can use Set Bank Coin sidebar) — Phase 4 deferred; v1.1 candidate (eviction enforcement was prioritized in v1.0.1)
- **999.2** Polished theme picker tile UI (6-tile grid per `docs/design/mockups/eq-aesthetic-picker.html`) — Phase 4 deferred; aesthetics-only
- **999.5** Self-service eviction (departing guildie quits cleanly without officer action) — v2 candidate (threat-model deferred)
- **999.7** Extract `SIDEBAR_BODY` constants to `apps-script/src/sidebars/*.html` — uniform deferral across 05-03 + 05-04; cosmetic refactor
- **999.9** SignPath Foundation OSS approval — submitted; awaiting review (would retire INST-05 partial → full). Lands as hotfix when approved (NOT a v1.0.2 phase per milestone-open decision).
- **999.11** Decide v1.1+ verification doctrine — adopt `/gsd-verify-work` per phase, or formalize live-smoke pattern
- **999.12** v2: Wantlist + Discord pinger (WANT-01..08; prerequisites WANT-06/07 still open)

---

*Roadmap created: 2026-04-30. v1.0 shipped: 2026-05-11. v1.0.1 shipped: 2026-05-12. v1.0.2 phases planned: 2026-05-12. Last reorganized: 2026-05-12 (v1.0.2 phase structure added).*
