---
gsd_state_version: 1.0
milestone: v2.1
milestone_name: — Self-Service Watcher Linking
status: executing
last_updated: "2026-06-02T00:00:00.000Z"
last_activity: 2026-06-02 -- Executed 17-03 (/account page + WatcherCodesPanel show-once/list/revoke + nav) — browser smoke APPROVED live
progress:
  total_phases: 2
  completed_phases: 0
  total_plans: 3
  completed_plans: 3
  percent: 50
---

# State: SquireBot

**Initialized:** 2026-04-30
**Last updated:** 2026-06-01 (v2.1 "Self-Service Watcher Linking" roadmap created — Phases 17–18)

## Project Reference

See: `.planning/PROJECT.md` (updated 2026-06-01 with v2.1 milestone scope)

- **Core value:** Every guildie can answer "what does my character still need, and where in the guild is it?" — delivered via the self-hosted website (squirebot.quest).
- **Current focus:** v2.1 — let any guildie self-serve linking their watcher via Discord login, retiring the maintainer's manual code-minting.
- **Mode:** yolo
- **Granularity:** coarse

## Current Position

Phase: 17 — Self-Service Watcher Linking (executing — all 3 plans complete; awaiting orchestrator phase-level verification)
Plan: 17-01 + 17-02 + 17-03 complete (3/3)
Status: Executing
Last activity: 2026-06-02 -- Executed 17-03 (/account form-card page + WatcherCodesPanel: generate→show-once copy-to-clipboard plaintext panel→active-codes list→confirm-before-commit revoke + fetchOwnCodes/mintOwnCode/revokeOwnCode wrappers + StateBlock no-codes kind + member-visible Account nav). Deployed to live VPS (api.squirebot.quest schema v5; squirebot.quest serves /account, 401-gated); blocking human browser-smoke APPROVED — all 7 steps passed. npm run check clean (443 files, 0 errors).
Progress: [#####.....] 50% — 3/3 Phase 17 plans complete (0/2 phases; phase completion owned by orchestrator)

## v2.1 Phase Plan (created 2026-06-01)

Phases continue from v2.0 (which ended at Phase 16). Phase dirs 11–16 exist on disk — never reuse them. v2.1 starts at **17**.

**Execution order:** 17 → 18 (Phase 18 is independent of 17 — it touches the watcher, which Phase 17 does not — but runs second since the LINK feature is the milestone's reason for being).

| Phase | Name | Requirements | Success Criteria | UI |
|-------|------|--------------|------------------|----|
| 17 | Self-Service Watcher Linking (web feature) | LINK-01, LINK-02, LINK-03, LINK-04, LINK-05, LINK-06 | 6 | yes |
| 18 | Watcher Cleanups — Verify-or-Close | WATCH-12, WATCH-13, WATCH-14 | 4 | no |

**Phase 17 — Self-Service Watcher Linking:** session-scoped mint endpoint (owner derived server-side from the Discord session) + Discord-identity↔owner linkage (LINK-02) + additive minting (LINK-03) + per-token list/revoke (LINK-05) + the "Link your watcher" page with show-once plaintext / copy-to-clipboard / paste instructions (LINK-04) + remove the `mint-code` CLI (LINK-06). Backend (`internal/backendsrv/{webadmin,store}`, `auth.MintCode`) + frontend (`web/`). Combined web-feature phase per coarse granularity — backend + frontend are tightly coupled and the feature is small.

**Phase 18 — Watcher Cleanups (verify-or-close):** 999.20/21/22 were recorded RESOLVED in v2.0 Plan 13-04 (commits `c930fc2` gofmt+freeConsole, `e758fb0`/`3e8e53b` SemVer IsNewer — confirmed present in git log 2026-06-01). Confirm live state; the only likely real residual is WATCH-14's maintainer stuck-on-`0.4.0-rc1` watcher (one-time manual reinstall). Do NOT re-implement the shipped fixes.

## Milestone v2.1 Scope (locked 2026-06-01)

**Goal:** Self-service watcher linking from squirebot.quest via Discord login — no maintainer hand-minting codes — while the watcher credential stays a static bearer token.

**Locked decisions:**

- **Re-link = additive** — each link issues an *additional* valid token (multiple PCs per guildie work simultaneously); revocation is per-token.
- **Link code = the bearer token itself (v2.0 model, reusable)** — `auth.MintCode` shows the plaintext once at mint time (hash-only at rest); the watcher pastes it and reuses it forever. No expiry. **NO watcher change** — onboarding identical to v2.0; the feature is a website + backend-endpoint change only.
- **Manual `mint-code` CLI removed** — self-service fully replaces it; the ~11 existing guildies are already linked from the v2.0 cutover, so nobody is stranded. (`revoke-code` CLI retained as an ops backstop.)
- **HARD CONSTRAINT:** Discord identity captured at link-time only; NEVER Discord OAuth *in the watcher* (P13 made it browser-free on purpose; OAuth there re-introduces the ~7-day-expiry / public-secret / loopback fragility v2.0 escaped).

**Scope bundle:** core feature (999.31 → Phase 17 / LINK-01..06) + watcher cleanups 999.20 (`console_windows.go` gofmt), 999.21 (`freeConsole()` doc-vs-impl mismatch), 999.22 (SemVer-aware auto-update — fixes the prerelease-stuck-watcher case the maintainer hit) → Phase 18 / WATCH-12/13/14.

> NOTE: 999.20/21/22 were recorded as RESOLVED during v2.0 Plan 13-04 (commits confirmed in git log 2026-06-01). Phase 18 is verify-or-close — if already fixed, fold the remaining gap (the maintainer's own stuck `0.4.0-rc1` watcher needing a manual reinstall) rather than re-doing closed work.

## Carried-forward / Deferred (from v2.0 close)

Non-blocking; recorded so they aren't lost:

- **P12/P14/P15 HUMAN-UAT smokes** — most were exercised live during the 16-03 deploy but the checklists were never formally ticked (see the v2.0 phase `*-HUMAN-UAT.md` files). The live system is verified working.
- **999.22 stuck-prerelease watcher** — the maintainer's own watcher is stuck on `0.4.0-rc1` (won't auto-update); a manual reinstall fixes it. Folded into Phase 18 / WATCH-14.
- **999.9 SignPath OSS** — still in flight; lands as a hotfix when approved (orthogonal).
- **999.12 / WANT-01..08** — v2 Wantlist + Discord pinger still deferred; AUTH-09 pre-paid the per-user Discord-identity prerequisite, and v2.1's Discord-tied linking (LINK-02) further dovetails with it.

## Files of Record

- `.planning/PROJECT.md` — core value, requirements, constraints, Key Decisions (updated 2026-06-01 with v2.1 milestone).
- `.planning/REQUIREMENTS.md` — v2.1 requirements + Traceability (9/9 mapped to Phases 17–18).
- `.planning/ROADMAP.md` — phase structure (v2.1 Phases 17–18 appended) + Backlog.
- `.planning/MILESTONES.md` — historical record (v1.0 + v1.0.1 + v2.0).
- `.planning/milestones/v2.0-{ROADMAP,REQUIREMENTS}.md` — the v2.0 archive.
- `.planning/RETROSPECTIVE.md` — living retrospective.
- `.planning/config.json` — granularity (coarse), mode (yolo), workflow toggles.

> The full v2.0 execution narrative lives in `.planning/MILESTONES.md` (§ v2.0), the milestone archive, and the per-plan SUMMARYs under `.planning/phases/11..16/`.

---

*State reset: 2026-06-01 for v2.1 "Self-Service Watcher Linking". Roadmap created 2026-06-01 (Phases 17–18). Prior: v1.0 (2026-05-11), v1.0.1 (2026-05-12), v1.0.2 binary (2026-05-13, superseded), v2.0 (2026-05-31).*
