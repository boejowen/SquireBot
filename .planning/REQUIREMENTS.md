# Requirements: SquireBot v1.0.2 — Robustness Polish

> v1.0 shipped 2026-05-11 (69 effective requirements; `milestones/v1.0-REQUIREMENTS.md`). v1.0.1 shipped 2026-05-12 (8 requirements; `milestones/v1.0.1-REQUIREMENTS.md`). This file is scoped to v1.0.2 only.

**Milestone goal:** Close 4 v1.0.1-UAT-surfaced robustness gaps in the Go watcher + 2 test-quality items in the apps-script suite — no new user-facing features, no schema change, no third-party gating.

**Schema impact:** None. `_meta.schema_version` stays at 3. `WatcherMaxSchemaVersion` stays at 3. No new `_meta` rows; no migrations.

---

## v1.0.2 Requirements

### Recovery & Identity (AUTH-07)

- [ ] **AUTH-07**: When the watcher boots and its refresh token is invalid (`invalid_grant` raised before the first successful Sheets API call), the tray icon turns red AND the Reauthorize menu item is visible from boot, NOT hidden behind a successful first API call. AUTH-05 in v1.0 covered the running-state revocation path (token works at boot, fails later); this requirement extends the same recovery UX to the boot-time path. Surfaced by Phase 6 UAT Finding C, 2026-05-11 (backlog 999.13).

### Tray Controller Robustness (OPS-06, OPS-07)

- [ ] **OPS-06**: Tray controller calls (`SetStatus`, `Show*`, `SetIconHealth`) that fire before the systray `OnReady` callback runs MUST either (a) be queued and replayed in-order once `OnReady` fires, OR (b) be detected by `app.RunApp` and trigger a single retry on the fast-fail path. The fail mode is: when `RunApp` returns early due to wincred-rebuild failure (or any other pre-Ready abort), the tray menu has no working items because every pre-Ready call silently no-op'd, stranding the user at "Initialising…". Wider impact than the v0.2.x T-06-20 "accept" disposition covered. Surfaced by Phase 6 UAT Finding D, 2026-05-11 (backlog 999.14).

- [ ] **OPS-07**: A foreground-launched watcher (started from cmd.exe or PowerShell without `Start-Process`) MUST NOT die silently when the parent shell closes. Acceptance is satisfied by EITHER: (a) `windows.FreeConsole()` called early in `cmd/squirebot/main.go` so the watcher detaches from any inherited console, OR (b) `docs/build-and-install.md` documents the `Start-Process` requirement prominently (above the fold, not in a footnote) with an explanation of why. Discuss-phase chooses the path. Surfaced by Phase 6 UAT Finding H, 2026-05-11 (backlog 999.16).

### Config Loader Robustness (CONFIG-01)

- [ ] **CONFIG-01**: `internal/config/load.go` strips a leading UTF-8 BOM (`\xEF\xBB\xBF`) from the file contents before `json.Unmarshal`. Closes a foot-gun for users who hand-edit `%LOCALAPPDATA%\SquireBot\config.json` with Notepad (writes BOM by default) or PowerShell 5.1 `Set-Content -Encoding utf8` (writes BOM by default). Estimated ≤5 LOC + 1 unit test. Surfaced by Phase 6 UAT Finding F, 2026-05-11 (backlog 999.15).

### Sidebar Test Coverage (TEST-03)

- [ ] **TEST-03**: Admin-Mgmt sidebar (`showAdminMgmtSidebar`) has at least one inline-JS test covering its DOM event handlers + `google.script.run` callback wiring + error-display path — co-located as `apps-script/src/__tests__/showAdminMgmtSidebar.inline.test.ts`, mirroring the 4 inline-JS test files landed in v1.0.1 Plan 08-02. Closes the v1.0.1 TEST-02 deferral note (5/5 sidebars with inline-JS coverage; currently 4 inline-JS + 1 trigger-call-only). Surfaced by Plan 08-02 historical-correction deferral, 2026-05-12 (backlog 999.17).

### Test Quality (TEST-04)

- [ ] **TEST-04**: Phase 8 advisory test-quality findings are addressed: (a) `mountSidebar` JSDOM realm leak (each test creates a new realm but prior tests' DOM may bleed in), (b) weak assertions in 2+ sidebar inline-JS tests upgraded from `toBeTruthy` / `toBeDefined` to specific equality or structural matchers, (c) leaked pending mock call in `searchSidebar.inline.test.ts` cleaned up. 0 critical + 4 warning + 6 info per `.planning/phases/08-test-infra-persistence-docs/08-REVIEW.md`; this requirement covers the 4 warnings, with discuss-phase deciding whether to also fold in the 6 info-level items. Surfaced by Phase 8 code review, 2026-05-12 (backlog 999.18).

---

## Out of Scope (v1.0.2 — explicitly deferred at milestone open)

| Item | Why deferred |
|------|--------------|
| SignPath Foundation OSS signing (999.9) | Third-party Foundation review timeline; lands as a hotfix when approved (NOT a v1.0.2 phase). Would retire INST-05 partial → full. |
| Bank-coin permission lock (999.1) | v1.1 polish candidate (eviction enforcement was prioritized in v1.0.1; bank-coin remains social-convention-gated). |
| Polished theme picker tile UI (999.2) | v1.1 polish candidate; aesthetics-only. |
| Self-service eviction (999.5) | v2; threat-model still open. |
| Verification doctrine decision (999.11) | Process change; addressed at v1.1 milestone planning. |
| Inline `SIDEBAR_BODY` → external `.html` refactor (999.7) | Cosmetic refactor; v1.1 polish candidate. |
| v2 Wantlist + Discord pinger (999.12 / WANT-01..08) | v2; prerequisites still open (Raid Alliance Discord bot invites). |

---

## Traceability

| REQ-ID | Phase | Plan(s) | Status |
|--------|-------|---------|--------|
| AUTH-07 | (TBD) | (filled by `/gsd-plan-phase`) | Pending |
| OPS-06 | (TBD) | (filled by `/gsd-plan-phase`) | Pending |
| OPS-07 | (TBD) | (filled by `/gsd-plan-phase`) | Pending |
| CONFIG-01 | (TBD) | (filled by `/gsd-plan-phase`) | Pending |
| TEST-03 | (TBD) | (filled by `/gsd-plan-phase`) | Pending |
| TEST-04 | (TBD) | (filled by `/gsd-plan-phase`) | Pending |

> Phase column to be filled by the roadmapper in `/gsd-new-milestone` step 10. Plan column filled by `/gsd-plan-phase` per phase.

**Coverage check (2026-05-12, requirements-stage):** 6/6 requirements defined; mapping to phases pending roadmap creation.

---

*Defined: 2026-05-12 at v1.0.2 milestone start. 6 requirements across 5 backlog items + 2 categories (recovery, tray, config, sidebar tests, test quality). Phase mapping pending roadmap creation.*
