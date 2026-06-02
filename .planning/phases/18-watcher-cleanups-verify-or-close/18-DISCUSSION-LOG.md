# Phase 18: Watcher Cleanups — Verify-or-Close - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-06-02
**Phase:** 18-watcher-cleanups-verify-or-close
**Areas discussed:** Stuck-watcher scope, Verification rigor, Regression guard, Phase shape & "done"

---

## Pre-discussion scouting (live-state confirmation)

Before presenting gray areas, scouted the tree to confirm the verify-or-close premise:
- WATCH-12: `gofmt -l cmd/squirebot/console_windows.go` → clean (commit `c930fc2`).
- WATCH-13: `console_windows.go:57` logs `slog.Debug` on the no-console case; `freeConsole()` doc (:22/:36–48) reconciled with impl (`c930fc2`).
- WATCH-14: `IsNewer` SemVer logic in `internal/update/manifest.go`; RED/GREEN tests `e758fb0`/`3e8e53b`. **Caveat surfaced:** the readily-visible `manifest_test.go` functions cover patch/minor/major/malformed but not an obvious pre-release→final case — flagged for the verify pass (CONTEXT D-05).
- `watcher_version` is accepted (`envelope.go:30`) and per `handler.go:234` "stored alongside the rows" — queryability for a stale-version survey left as a research flag (CONTEXT D-02).

Conclusion presented to user: all 3 code fixes confirmed live; Phase 18 is genuinely verify + one ops action (the stuck-watcher reinstall).

---

## All four areas (presented together)

| Option (area) | Description | Selected |
|---------------|-------------|----------|
| Stuck-watcher scope | Maintainer-only vs. survey all guildies via `watcher_version`; manual reinstall vs. in-place self-heal | ✓ |
| Verification rigor | Lightweight confirmation vs. fresh behavioral assertions per WATCH item | ✓ |
| Regression guard | Add a permanent pre-release-comparison lock test vs. rely on existing 13-04 tests | ✓ |
| Phase shape & "done" | Code phase vs. verify+ops phase; how to close WATCH-14's ops residual | ✓ |

**User's choice:** "I have no preference for any of those — please pick whichever you think will make the end-user experience the simplest."

**Notes:** User delegated all four areas with an explicit deciding rule: *optimize for the simplest end-user (guildie) experience.* Per the established "delegate gray areas when given a criterion" pattern, all four were locked in a single pass against that rule (plus the corollary "don't rebuild already-shipped work in a close-out phase") rather than running further question turns. Resulting decisions: D-01..D-08 in CONTEXT.md.

Rule-application summary:
- **Stuck-watcher** → manual reinstall (most reliable; the broken pre-release client can't self-heal); proactively survey all guildies **only if** `watcher_version` is already queryable (free), else maintainer-only — no telemetry-building in a verify phase.
- **Verification rigor** → lightweight confirmation (fixes already shipped), with the single exception that the pre-release→final `IsNewer` case must be confirmed/added (D-05) since that's the exact bug that stranded the maintainer.
- **Regression guard** → existing 13-04 tests are the guard, contingent on D-05 confirming pre-release coverage.
- **Phase shape** → verify + ops, near-zero code; WATCH-14 ops residual closed via a HUMAN-UAT with backend `last_seen` as objective proof.

---

## Claude's Discretion

- Whether `watcher_version` is queryable without new schema (resolves D-02's branch) — left to the researcher.
- Whether WATCH-12/13 warrant their own plan or fold into one verification plan — left to the planner.
- Exact reinstall mechanics — follow `docs/build-and-install.md`.

## Deferred Ideas

None — discussion stayed within phase scope. (A standalone watcher-version telemetry/dashboard would only become a future idea if D-02 research shows `watcher_version` is not queryable; not built here.)
