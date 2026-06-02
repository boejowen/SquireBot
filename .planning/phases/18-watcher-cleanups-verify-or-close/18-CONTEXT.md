# Phase 18: Watcher Cleanups — Verify-or-Close - Context

**Gathered:** 2026-06-02
**Status:** Ready for planning

<domain>
## Phase Boundary

Confirm three already-shipped watcher polish fixes (999.20/21/22, landed in v2.0 Plan 13-04) are live-correct in the current tree, and get the maintainer's stuck `0.4.0-rc1` watcher back onto the current release. This is a **verify-or-close phase: near-zero new code** — the three code fixes are already present and (for WATCH-14) test-backed. The only guaranteed real work is an operational reinstall, plus confirming no hidden gap remains.

Scope anchor: WATCH-12 (gofmt), WATCH-13 (freeConsole doc/impl + Debug-not-Warn on no-console launch), WATCH-14 (SemVer-aware `IsNewer` + un-stick the maintainer's watcher). HARD CONSTRAINT (milestone-locked): **never add Discord OAuth to the watcher**; the credential stays a static bearer token. No watcher onboarding change.

</domain>

<decisions>
## Implementation Decisions

> **Deciding rule (user-supplied, applies to all areas):** "No preference — pick whatever makes the end-user (guildie) experience the simplest." Every decision below applies that rule, with the corollary "don't rebuild already-shipped work in a close-out phase." Downstream agents should use *simplest-guildie-experience + no-feature-building-in-a-verify-phase* as the tie-breaker for any sub-decision not spelled out here.

### Stuck-Watcher Remediation (WATCH-14 ops residual)
- **D-01:** The maintainer's stuck `0.4.0-rc1` watcher is fixed by a **one-time manual reinstall** (re-run the current v2.0.0 installer; Phase 6's overwrite-running shim makes reinstall-over-a-running-watcher painless). In-place self-heal is explicitly NOT relied upon: a watcher on `0.4.0-rc1` runs `0.4.0-rc1`'s own pre-SemVer-fix comparison logic, so the fix that shipped in a later release cannot reach it (the broken client is the thing that's broken). Simplest + most reliable end-user outcome.
- **D-02 (conditional — RESEARCH FLAG):** Proactively confirm no *other* guildie is silently stuck, but only if it's free. `ingest/handler.go:234` says `watcher_version` is "stored alongside the rows" — IF it is already persisted in a queryable column, the verify pass runs ONE read-only DB query listing any guildie on a pre-release/stale `watcher_version` (cross-referenced with `guild_code.last_seen`), and each identified watcher gets the same one-time reinstall nudge. IF `watcher_version` is NOT queryable without new schema/code, SKIP the survey and remediate only the maintainer — do NOT build telemetry plumbing in a close-out phase. Researcher must determine which branch applies.
- **D-03:** Zero new *watcher* code for the ops residual. At most a read-only backend query + an installer re-run. No changes to `cmd/squirebot` onboarding/update flow.

### Verification Rigor (WATCH-12/13/14 code)
- **D-04:** Lightweight confirmation, because all three fixes are already shipped (and WATCH-14 is test-backed). Confirm, don't re-implement: `gofmt -l cmd/squirebot/console_windows.go` is clean; `freeConsole()`'s doc matches its implementation and the no-console (GUI/Explorer) launch logs at `slog.Debug` (not a spurious Warn) — currently at `console_windows.go:57`; the SemVer `IsNewer` tests pass.
- **D-05 (the one non-trivial verify — load-bearing):** The verify pass MUST confirm the **pre-release→final-release** `IsNewer` case is actually present and passing in `internal/update/manifest_test.go` (commits `e758fb0`/`3e8e53b` claim to add it, but the readily-visible test functions only cover patch/minor/major/malformed). Required assertions: `IsNewer("0.4.0-rc1", "0.4.0") == true` (a parked pre-release recognizes its final as newer) and `IsNewer("0.4.0", "0.4.0-rc1") == false`. If that specific coverage is missing or weakened, ADD the minimal assertion. This is the exact bug that stranded the maintainer — it is the one place a real gap could still hide.

### Regression Guard
- **D-06:** Rely on the existing 13-04 RED/GREEN tests as the permanent guard, **contingent on D-05** confirming they actually cover pre-release→final. No additional regression tests beyond closing any gap D-05 surfaces. Existing coverage is the guard; don't gold-plate.

### Phase Shape & "Done"
- **D-07:** Treat Phase 18 as a **verify + ops phase**. WATCH-12 and WATCH-13 close by confirmation (read-only checks, largely already performed during discussion). WATCH-14's *code* closes by confirmation + D-05's gap-check; its *ops* residual closes via a **HUMAN-UAT checklist**: the maintainer reinstalls the stuck watcher(s) the survey identifies (or just their own, per D-02's branch) and confirms each lands on the current release and resumes uploading.
- **D-08:** "Done" evidence: `gofmt -l` clean confirmed; `freeConsole` doc/impl reconciled + Debug-level confirmed; `IsNewer` pre-release→final test green; the maintainer's watcher (and any other surveyed-stuck ones) on the current release and ingesting again — verifiable via a fresh `guild_code.last_seen` on the backend (the stamp shipped in Phase 17).

### Claude's Discretion
The user delegated all four areas with the rule above. Beyond it: the researcher resolves D-02's branch (is `watcher_version` queryable?); the planner decides whether WATCH-12/13 even warrant their own plan or fold into a single verification plan; the exact reinstall mechanics follow `docs/build-and-install.md`.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements & roadmap
- `.planning/REQUIREMENTS.md` — WATCH-12 / WATCH-13 / WATCH-14 definitions + the "verify-or-close" note (line 32) naming the 13-04 fix commits.
- `.planning/ROADMAP.md` §"Phase 18: Watcher Cleanups — Verify-or-Close" — goal + the 4 falsifiable success criteria.

### The code being verified
- `cmd/squirebot/console_windows.go` — WATCH-12 (gofmt subject) + WATCH-13 (`freeConsole()` doc at :22/:36–48, impl at :50, `slog.Debug` no-console log at :57).
- `internal/update/manifest.go` + `internal/update/manifest_test.go` — WATCH-14 `IsNewer` SemVer logic + its tests (the D-05 pre-release coverage check).
- `internal/update/check.go` — the auto-update path that consumes `IsNewer` / `latest.json`.

### Telemetry feasibility (for D-02's survey branch)
- `internal/backendsrv/ingest/handler.go` §:234 + `internal/backendsrv/ingest/envelope.go` §:30 — how `watcher_version` is received and (per the comment) stored; researcher confirms whether it is queryable without new schema.
- Phase 17 `guild_code.last_seen` (migration `00005`) — the "watcher is alive again" signal for the close-out evidence.

### The original fixes (do NOT re-do)
- Plan 13-04 commits: `c930fc2` (gofmt + freeConsole doc/impl), `e758fb0` (IsNewer pre-release RED test), `3e8e53b` (IsNewer pre-release GREEN). Confirmed present in git log 2026-06-02.

### Ops
- `docs/build-and-install.md` — the installer / reinstall flow (Phase 6 overwrite-running shim) for the WATCH-14 ops residual.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **Phase 6 overwrite-running installer shim** — lets a reinstall land on top of a running watcher without a manual stop; the painless path for the WATCH-14 reinstall.
- **`IsNewer` / `IsOlder` SemVer compare** (`internal/update`, `parseVersion` strips leading `v`, handles pre-release) — already implemented; verify, don't rewrite.
- **`guild_code.last_seen` stamp** (Phase 17, `00005`) — the post-reinstall "watcher resumed uploading" confirmation signal, no new instrumentation needed.

### Established Patterns
- SemVer-aware comparison via `parseVersion`; structured `slog` with deliberate Debug-vs-Warn level discipline (the WATCH-13 subject).
- TDD RED/GREEN test commits for update-logic changes (13-01, 13-04 precedent).

### Integration Points
- The optional D-02 survey reads backend SQLite (`watcher_version` storage + `guild_code.last_seen`) — read-only, no migration.
- The reinstall connects to the existing `latest.json` auto-update + NSIS installer distribution.

</code_context>

<specifics>
## Specific Ideas

- The maintainer's watcher is stuck on `0.4.0-rc1` specifically; the current shipped watcher line is the v2.0.x "Off Google" build. The reinstall target is the current release, not a hotfix.
- The close-out should *prove* the fix took, not just assert it — use the backend `last_seen` as objective evidence the reinstalled watcher is uploading again.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope. (The "survey all guildies for stale versions" is in-scope-but-conditional per D-02, not a deferred future phase. If D-02's research shows `watcher_version` is NOT queryable, a proper watcher-version telemetry/dashboard would be a separate future idea — note it then, don't build it here.)

</deferred>

---

*Phase: 18-watcher-cleanups-verify-or-close*
*Context gathered: 2026-06-02*
