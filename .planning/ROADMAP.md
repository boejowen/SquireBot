# Roadmap: SquireBot

**Core Value:** Every guildie can answer "what does my character still need, and where in the guild is it?" without leaving the spreadsheet.

## Milestones

- ✅ **v1.0** — Watcher + Workbook + Onboarding (initial release) — shipped 2026-05-11 as tag `v1.0.0`
- ✅ **v1.0.1** — Installer + Permissions Hardening — shipped 2026-05-12 (binary tag `v1.0.1` pushed 2026-05-11 by Phase 6 ship gate)
- 📋 **Next milestone** — TBD (run `/gsd-new-milestone` to scope)

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

## Progress

| Milestone | Phases | Plans Complete | Status | Completed |
|-----------|--------|----------------|--------|-----------|
| v1.0 | 5 | 31/31 | ✅ Shipped | 2026-05-11 |
| v1.0.1 | 3 | 12/12 | ✅ Shipped | 2026-05-12 |

## Backlog

Carried forward from v1.0 + v1.0.1 (candidates for v1.0.2 / v1.1 / v2). v1.0.1 surfaced 4 new v1.0.2 candidates (999.13–999.16) during Phase 6 UAT.

- **999.1** Bank-coin permission lock (only bank-toon-owner can use Set Bank Coin sidebar) — Phase 4 deferred; v1.1 candidate (eviction enforcement was prioritized in v1.0.1)
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
- **999.17** v1.0.2 candidate — Admin-Mgmt sidebar inline-JS test coverage. Trigger-call coverage exists via Phase 7's `adminMgmtSidebar.test.ts`; co-located inline-JS test deferred per Plan 08-02 TEST-02 historical-correction note. Surfaced by Phase 8, 2026-05-12.
- **999.18** v1.0.2 candidate — Phase 8 advisory test-quality findings (mountSidebar realm leak, weak assertions, leaked pending mock call). 0 critical, 4 warning, 6 info per `phases/08-test-infra-persistence-docs/08-REVIEW.md`. All non-blocking. Surfaced by Phase 8, 2026-05-12.

---

*Roadmap created: 2026-04-30. v1.0 shipped: 2026-05-11. v1.0.1 shipped: 2026-05-12. Last reorganized: 2026-05-12 (v1.0.1 collapsed into milestone-archive form).*
