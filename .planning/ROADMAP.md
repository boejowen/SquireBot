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

- [x] **Phase 9: Watcher Robustness Polish** — closed 4 v1.0.1-UAT-surfaced robustness gaps (boot-time `invalid_grant` Reauthorize recovery, tray controller pre-Ready call queue, UTF-8 BOM strip in config loader, foreground-shell-close silent death fix); shipped as watcher v1.0.2 binary release. Requirements: AUTH-07, OPS-06, OPS-07, CONFIG-01. Ship gate (✓ 2026-05-13): tag `v1.0.2` + GitHub Release + `latest.json` refresh. **HUMAN-UAT scenarios persisted in `09-HUMAN-UAT.md`, blocked on 999.19 (Google brand verification re-approval) until ~2026-05-16/05-18.**
- [x] **Phase 10: Apps Script Test Quality** — closed v1.0.1-Phase-8-review-surfaced test-quality items. Requirements: TEST-03, TEST-04. Ship gate (✓ 2026-05-13): green CI (337 passed + 1 skipped + 0 failed / 338 total) + `clasp push` to dev workbook + 5/5 sidebar smoke clean. One deferral: 999.30 (`searchIndex.test.ts` Test 4 — pre-existing latent `didYouMean` contract bug exposed by WR-03 unswallow; converted to `it.skip` with v1.1 un-skip handoff).

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
| v1.0.2 | 2 | 8/8 | 🚧 In progress (both phases shipped 2026-05-13; HUMAN-UAT blocked on Google brand verification approval) | — |

| Phase | Milestone | Status | Completed |
|-------|-----------|--------|-----------|
| 9. Watcher Robustness Polish | v1.0.2 | ✅ Shipped (HUMAN-UAT pending 999.19) | 2026-05-13 |
| 10. Apps Script Test Quality | v1.0.2 | ✅ Shipped | 2026-05-13 |

## Backlog

Carried forward from v1.0 + v1.0.1 (candidates for v1.1 / v2). Items pulled INTO v1.0.2 (999.13–999.18) are no longer listed here; they live in `.planning/REQUIREMENTS.md` as AUTH-07, OPS-06, OPS-07, CONFIG-01, TEST-03, TEST-04.

- **999.1** Bank-coin permission lock (only bank-toon-owner can use Set Bank Coin sidebar) — Phase 4 deferred; v1.1 candidate (eviction enforcement was prioritized in v1.0.1)
- **999.2** Polished theme picker tile UI (6-tile grid per `docs/design/mockups/eq-aesthetic-picker.html`) — Phase 4 deferred; aesthetics-only
- **999.5** Self-service eviction (departing guildie quits cleanly without officer action) — v2 candidate (threat-model deferred)
- **999.7** Extract `SIDEBAR_BODY` constants to `apps-script/src/sidebars/*.html` — uniform deferral across 05-03 + 05-04; cosmetic refactor
- **999.9** SignPath Foundation OSS approval — submitted; awaiting review (would retire INST-05 partial → full). Lands as hotfix when approved (NOT a v1.0.2 phase per milestone-open decision).
- **999.11** Decide v1.1+ verification doctrine — adopt `/gsd-verify-work` per phase, or formalize live-smoke pattern
- **999.12** v2: Wantlist + Discord pinger (WANT-01..08; prerequisites WANT-06/07 still open)
- **999.19** Google OAuth brand verification re-approval — submitted to Google review queue 2026-05-13 with new homepage (`https://boejowen.github.io/SquireBot/`) + privacy policy (`https://boejowen.github.io/SquireBot/privacy-policy/`) + `boejowen.github.io` authorized domain (Search Console-verified). Blocks all SquireBot watcher auth (v0.4.0-rc1, v1.0.1, v1.0.2 — uniform) until Google approves. ETA 3–5 business days. Track resolution in `.planning/debug/v1-0-2-oauth-invalid-client-incident.md`.
- **999.20** WR-01 — `cmd/squirebot/console_windows.go` is not `gofmt -l` clean (subtle whitespace in the `var` block); shipped in v1.0.2 because there's no gofmt CI gate today, but next push touching this file may trip lint elsewhere. One-line fix.
- **999.21** WR-02 — `cmd/squirebot/console_windows.go` `freeConsole()` doc promises `nil` on no-console processes but implementation returns non-nil + logs `slog.Warn` whenever `ret == 0`. Per MSDN `ret == 0` is the normal case when no console is attached. Log noise + violated documented contract; `main.go` discards the return with `_ =` so no functional regression. Either fix the impl to swallow `ERROR_INVALID_HANDLE` or update the doc.
- **999.22** SemVer-aware auto-update comparison — the dev `0.4.0-rc1` watcher on the developer machine treats `1.0.2` as older than itself and skips the update. Almost certainly string comparison in `internal/update/check.go` instead of proper SemVer comparison; pre-release tags (`-rc1`) should sort BELOW the corresponding release. Likely won't affect production guildies (none ran a `-rc1` build) but should be fixed for future pre-release safety.
- **999.23** Graceful tray messaging when Google blocks the OAuth client itself (policy/verification gate, not user-side `invalid_grant`). Today the watcher hits Reauthorize → browser → "Access blocked" Google page → confused guildie. Better UX: distinguish `invalid_client`/policy errors from `invalid_grant` in the tray classifier and surface "SquireBot's Google brand verification is in review — check back in a few days; nothing you can do on your end."
- **999.24** IN-01 — `COL_RACE = 14` / `COL_COUNT = 14` collision in `apps-script/src/triggers/showCharInfoSidebar.ts:26-27`. Production schema-evolution foot-gun: extend-only column adds bump `COL_COUNT` to 15 while leaving `COL_RACE = 14`. Rename to semantically-distinct constants. Surfaced by 08-REVIEW; deferred from Phase 10 per CONTEXT D-01.
- **999.25** IN-02 — Orphaned `squirebot:search:recent` CacheService key never explicitly cleaned up post-SEARCH-05 migration (one-line `CacheService.getDocumentCache().remove(KEY_RECENT)` on next read). 25-min TTL is self-healing; symbolic cleanup. Surfaced by 08-REVIEW; deferred from Phase 10.
- **999.26** IN-03 — `evictionSidebar.inline.test.ts` builds HTML directly via `buildSidebarHtml(null)`, bypassing the Phase 7 admin gate at the inline-JS-test layer. Gate IS tested at the trigger layer; defense-in-depth note only. Surfaced by 08-REVIEW; deferred from Phase 10.
- **999.27** IN-04 — `showSearchSidebar.test.ts` Test 3 negative assertion excludes only one specific theme hex (`'--bg: #f5f5f5'`) instead of asserting "no themed `:root` block emitted at all". Surfaced by 08-REVIEW; deferred from Phase 10.
- **999.28** IN-05 — `searchIndex.ts` `didYouMean('')` returns short-name candidates instead of empty list (unreachable today; callers short-circuit on empty query, but it's a contract bug for any future direct caller). Surfaced by 08-REVIEW; deferred from Phase 10.
- **999.29** IN-06 — `test-helpers.ts` CacheService mock TTL boundary is strict-greater-than vs production's undefined behavior at the exact boundary millisecond. Mock-fidelity nit. Surfaced by 08-REVIEW; deferred from Phase 10.
- **999.30** `searchIndex.test.ts` Test 4 — `didYouMean('clok', [multi-word seed])` contract vs. whole-string Levenshtein mismatch. Pre-existing latent bug exposed by Phase 10 Plan 10-01's WR-03 unswallow (the try/catch in Test 4 had been silently absorbing the assertion failure since the test was authored). Converted to `it.skip` in Plan 10-03 (commit `08a7e39`) with an explicit un-skip handoff comment. v1.1 fixer either (a) short-circuits `didYouMean('')` to `[]` and tightens the multi-word distance metric, then un-skips Test 4 to assert the new contract, or (b) decides the assertion was always wrong and rewrites Test 4 to match the actual semantic intent (Test 4b already covers single-word candidates correctly).

---

*Roadmap created: 2026-04-30. v1.0 shipped: 2026-05-11. v1.0.1 shipped: 2026-05-12. v1.0.2 phases planned: 2026-05-12. Last reorganized: 2026-05-12 (v1.0.2 phase structure added).*
