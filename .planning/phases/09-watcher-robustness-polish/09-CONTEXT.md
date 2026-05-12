# Phase 9: Watcher Robustness Polish — Context

**Gathered:** 2026-05-12
**Status:** Ready for planning
**Mode:** `/gsd-discuss-phase 9` with user-supplied tiebreaker: "make whichever decision produces the simplest, most invisible end-user experience"

<domain>
## Phase Boundary

Eliminate the 4 v1.0.1-UAT-surfaced foot-guns in the Go watcher and ship a clean v1.0.2 binary release that becomes the new recommended download. Pure Go-side work; zero apps-script changes; no schema bump.

The 4 fixes (locked by REQUIREMENTS.md AUTH-07, OPS-06, OPS-07, CONFIG-01):

1. **AUTH-07** — Boot-time `invalid_grant` recovery: a guildie whose refresh token was revoked between sessions sees a red tray icon AND a visible Reauthorize menu item from boot. Today these only surface after a running-state failure (AUTH-05 path).
2. **OPS-06** — Tray controller pre-Ready calls (`SetStatus`, `Show*`, `SetIconHealth`) currently silently no-op until `OnReady` fires. When `RunApp` fast-fails before Ready (e.g., wincred rebuild failure), the tray strands at "Initialising…" with no working menu.
3. **OPS-07** — Foreground-launched watcher (started from cmd.exe / PowerShell `& exe` without `Start-Process`) dies silently when the parent shell closes. No `squirebot exit` log line.
4. **CONFIG-01** — `config.Load()` rejects hand-edited `config.json` with a UTF-8 BOM (Notepad / PowerShell 5.1 `Set-Content -Encoding utf8` both write a BOM by default). Surfaces as `invalid character 'ï' looking for beginning of value`.

**Ship gate:** tag `v1.0.2` (watcher binary release) + GitHub Release + `latest.json` manifest refresh so existing v1.0.1 watchers auto-update cleanly. Mirrors v1.0.1 Phase 6 release-tag pattern.

**Hard non-goals (push back if surfaced):**

- Any apps-script work (TEST-03 / TEST-04 belong to Phase 10).
- Schema changes. `_meta.schema_version` stays at 3; `WatcherMaxSchemaVersion` stays at 3 (verification hook 5 grep gate, same as Phase 6/7/8).
- SignPath OSS signing (999.9 — third-party-gated; hotfix-when-approved, not a v1.0.2 phase).
- Any new tray menu items, new sidebars, or behavior changes beyond making the 4 documented failure modes recoverable / invisible.
- Bundling docs work beyond what OPS-07 needs (DOC-04 stays closed; no broader doc backfill).
- Refactoring the tray controller beyond what OPS-06 requires (no "while we're in here" cleanups).

</domain>

<decisions>
## Implementation Decisions

Tiebreaker rule applied throughout: when multiple paths meet the requirement, pick the one that minimizes end-user awareness of the failure mode. The end user is a non-technical guildie running the tray app; "invisible" means they never have to learn a new launch incantation, never have to read docs, never see a frozen tray.

### D-01 — OPS-06 fix: tray controller buffers pre-Ready calls and replays in-order in `OnReady`

`internal/tray/tray.go` Controller grows a pending-calls queue (a slice of closures or a small typed-action struct) guarded by `t.mu`. Every public mutator — `SetStatus`, `SetIconHealth`, `ShowContinueSetup`, `HideContinueSetup`, `ShowReauthorize`, `HideReauthorize`, `SetSpreadsheetID` — checks an `t.ready` boolean under the mutex:

- If `ready == false`: append the call to the pending queue and return. The current nil-check no-ops (`if t.mStatus != nil`) become "queue + return" instead of silent drop.
- If `ready == true`: execute against the live systray menu items (current behavior).

`OnReady` (after building the menu) sets `ready = true` and drains the queue in FIFO order, applying each action against the now-live menu items. Drain happens under the mutex to prevent races with concurrent mutators.

**Why option (a) and not (b) "RunApp retries on fast-fail" or (c) "systray.Quit() on fast-fail":**

- (a) Queue-and-replay is the only path where the guildie never sees a transient bad state. If `RunApp` fast-fails BEFORE Ready, fires red icon + Reauthorize + error status, then `OnReady` arrives, all three calls land in the correct order and the tray opens already showing "Auth error" with a clickable Reauthorize. Invisible recovery.
- (b) Retry-on-fast-fail still has a window where the user might see "Initialising…" briefly between fast-fail and retry. Less invisible.
- (c) `systray.Quit()` is the worst path for invisibility — the user sees the process die. Deterministic, but visible failure.

**Queue ordering guarantee:** `Show*` / `Hide*` for the same menu item could pair up oddly if not drained in order (e.g., `ShowReauthorize` then later `HideReauthorize` should net to hidden). FIFO drain under the mutex preserves last-write-wins semantics for these pairs, same as the live-call path.

**Capacity:** unbounded. The dark window is short (milliseconds to seconds in the worst case); even a pathological 1000 queued calls is trivial memory.

**Reauthorize click safety:** if a user could somehow click Reauthorize during the dark window (they can't — the menu isn't built yet), the click handler is a separate code path inside `OnReady`'s menu builder, so this is structurally impossible. No special handling needed.

### D-02 — OPS-07 fix: `windows.FreeConsole()` early in `cmd/squirebot/main.go`

Add a Windows-only build-tagged file `cmd/squirebot/console_windows.go` (or inline behind `runtime.GOOS == "windows"` with the existing `golang.org/x/sys/windows` import — pick whichever produces less new code) that calls `windows.FreeConsole()` immediately after the `--uninstall-wipe-credentials` and `--quit` short-circuit checks but BEFORE logging init.

**Ordering rationale:** `--uninstall-wipe-credentials` and `--quit` print to stderr and must inherit the console (they're invoked by NSIS/the shutdown shim and need their output captured by the parent). For the normal watcher path, `FreeConsole` detaches before any structured logging starts; subsequent slog writes go to the log file only (unchanged behavior).

**Why (a) FreeConsole and not (b) docs-only or (c) both:**

- (a) FreeConsole means the guildie can launch the exe any way they want — double-click, Start-menu shortcut, `& exe` in PowerShell, batch file, scheduler — and the watcher detaches cleanly. Closing the launching shell does not kill it. No docs to read, no incantation to remember.
- (b) Docs-only forces every guildie to learn `Start-Process` (or worse, hit the silent-death failure mode first, then read docs). Not invisible.
- (c) Both is fine, but (a) alone is sufficient. The fix itself eliminates the failure mode; a small note in `docs/build-and-install.md` § "Manual debug aids" is cheap to add and helps developers who want to see foreground output, but it's NOT required for acceptance. Planner decides whether to include the doc tweak as a single-line note.

**Verification:** existing `cmd/squirebot/main.go:213` exit log line (`slog.Info("squirebot exit")`) MUST still emit after FreeConsole — confirmed by reading the file, this happens late in the goroutine teardown long after detach.

**Side-effect check:** the wizard's browser-opening path (Plan 03 / Phase 6 Finding G) does NOT depend on inherited stdio. The wizard uses `exec.Command("rundll32", "url.dll,FileProtocolHandler", url)` (or equivalent) which spawns a new process unaffected by FreeConsole. Confirmed safe.

### D-03 — AUTH-07 fix: any boot-path token-rebuild failure classified by `auth.IsRevokedRefreshToken` triggers red icon + visible Reauthorize from boot

In `internal/app/runapp.go`, the existing `rebuildTokenFromWincred` (or whatever the function is named — planner confirms during research) path that runs on cold-start currently returns an error and lets RunApp fast-fail. The fix:

1. If the error returned from token rebuild matches the existing `auth.IsRevokedRefreshToken` classifier (the same one AUTH-05's running-state path uses), call `t.SetIconHealth(tray.HealthRed)` + `t.SetStatus("Auth error — sign in again")` + `t.ShowReauthorize()` BEFORE returning from RunApp.
2. These three calls happen pre-OnReady on the fast-fail path, which is exactly the scenario D-01 enables — they get queued, then replayed in `OnReady`, so the tray menu opens already showing the auth-error state with a clickable Reauthorize.
3. The Reauthorize click handler is unchanged from AUTH-05 — it invokes the existing `app.RunReauthorize` flow, which on success swaps the wincred token, re-enters `RunApp`, and clears the red icon + hides Reauthorize via the normal post-success path.

**Why classify on rebuild error and not "always show Reauthorize on every boot":**

- "Always show" pollutes the menu for healthy guildies (every boot looks like an auth error). Worse UX.
- Classifying matches the running-state behavior — Reauthorize appears IFF auth is broken. Symmetric, predictable.

**Why classify on rebuild error and not "probe Sheets API early":**

- A pre-watch probe adds latency to every cold start (network call before the watcher is ready to work) for a failure mode that fires for one guildie in 100. Invisible-UX tiebreaker prefers no extra probe.
- The rebuild path is the natural detection point: if the refresh token is revoked, `oauth2.TokenSource.Token()` returns `invalid_grant` synchronously when invoked, which is exactly what the rebuild function calls. No extra round-trip.

**Why this depends on D-01:** without the queue, these three calls land in the silent-no-op window and the user sees "Initialising…" with no recovery (the current bug). With the queue, the calls land correctly in `OnReady`. So plan ordering must enforce D-01 lands before AUTH-07's runapp wiring; the planner notes this dependency.

**Status string copy:** `"Auth error — sign in again"` is the canonical phrasing. Mirrors the running-state AUTH-05 status string if one exists; planner confirms by reading existing tray usage and aligns (no new copy variant introduced).

### D-04 — CONFIG-01 fix: strip a leading UTF-8 BOM in `config.Load()` after `os.ReadFile`, before `json.Unmarshal`

In `internal/config/config.go` `Load()` (note: REQUIREMENTS.md says `load.go` but the actual file is `config.go` — planner confirms during research), between line 54 (`data, err := os.ReadFile(p)`) and line 62 (`json.Unmarshal(data, &c)`), add:

```go
data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
```

That's it. ~3 LOC including the new `bytes` import.

**Why strip vs. switch decoder:**

- The Go `encoding/json` package does not auto-strip BOMs; the only choices are strip-before-unmarshal or use a different decoder. Strip-before-unmarshal is the minimal, idiomatic fix. No new dependencies; no behavior change for BOM-free files.

**Why only the leading BOM and not all whitespace / mid-document BOMs:**

- The failure mode is specifically the Notepad / PS5.1 BOM-on-write behavior, which writes a single leading BOM. JSON spec doesn't allow mid-document BOMs anyway (`json.Unmarshal` would correctly reject them as "invalid character" mid-token). Stripping the leading BOM closes the documented failure mode; over-stripping risks masking real corruption.

**Test:** one new unit test in `internal/config/config_test.go` — `TestLoad_StripsUTF8BOM` — writes a BOM-prefixed valid JSON config to a temp file via `os.WriteFile`, calls `Load()`, asserts the returned Config has the expected field values and no error.

**Scope discipline:** do NOT extend BOM handling to other file readers in this plan. The other JSON readers in the watcher (`auth/StoredToken` JSON in wincred, `latest.json` from the update server) are programmatically written by trusted code paths and won't have BOMs. If a future user-edit foot-gun surfaces for another file, fix it there separately.

### D-05 — Plan structure: 5 plans, mirrors Phase 6's release shape

| Plan | REQ | Files touched | Depends on | Wave |
|------|-----|---------------|------------|------|
| 09-01 — Tray controller pre-Ready queue (OPS-06) | OPS-06 | `internal/tray/tray.go`, `internal/tray/tray_test.go` | — | 1 |
| 09-02 — `windows.FreeConsole()` foreground detach (OPS-07) | OPS-07 | `cmd/squirebot/main.go` (new `console_windows.go` build-tagged file), optional 1-line note in `docs/build-and-install.md` | — | 1 |
| 09-03 — UTF-8 BOM strip in `config.Load()` (CONFIG-01) | CONFIG-01 | `internal/config/config.go`, `internal/config/config_test.go` | — | 1 |
| 09-04 — Boot-time `invalid_grant` → red + Reauthorize (AUTH-07) | AUTH-07 | `internal/app/runapp.go`, `internal/app/runapp_test.go`, possibly thin additions in `internal/tray/tray.go` only if a new helper is needed (unlikely — D-01 makes existing methods sufficient) | 09-01 | 2 |
| 09-05 — v1.0.2 binary release tag + `latest.json` refresh + GitHub Release | (ship gate) | `internal/update/version.go` constant bump, GitHub Actions release workflow trigger, `latest.json` manifest update | 09-01..09-04 | 3 |

**File overlap check (Wave 1 parallelism):**
- 09-01 ↔ 09-02: zero overlap (`internal/tray/*` vs `cmd/squirebot/*`)
- 09-01 ↔ 09-03: zero overlap (`internal/tray/*` vs `internal/config/*`)
- 09-02 ↔ 09-03: zero overlap (`cmd/squirebot/*` vs `internal/config/*`)
- Wave 1 is safely parallel; 3 plans run concurrently.

**Wave 2 (09-04) depends on 09-01** because AUTH-07's boot-time `ShowReauthorize` / `SetIconHealth` / `SetStatus` calls land in the pre-Ready window. Without the queue, they no-op and the requirement fails acceptance.

**Wave 3 (09-05) depends on all** — release shouldn't tag until the fixes are in main.

**Plan size estimates** (informs whether to split further):
- 09-01: ~80 LOC (queue + drain + ready flag + mutex updates) + ~120 LOC tests (FIFO order, multi-mutator, drain-on-Ready) = ~200 LOC. Single plan, fits coarse granularity.
- 09-02: ~30 LOC (one-line FreeConsole call + build-tag scaffolding) + ~10 LOC test or smoke note (hard to unit-test FreeConsole; smoke acceptance via manual launch from PS). Single plan.
- 09-03: ~5 LOC src + ~30 LOC test. Single plan.
- 09-04: ~40 LOC src (error classification + 3 tray calls + log line) + ~60 LOC tests (mocked tray controller, simulated `invalid_grant` from rebuild). Single plan.
- 09-05: pure release-engineering — version constant bump, tag push, workflow dispatch, `latest.json` PR. Single plan. Reuses Phase 6's release.yml unchanged.

### D-06 — Schema impact assertion: NONE (load-bearing assertion for verifier)

`_meta.schema_version` stays at 3. `WatcherMaxSchemaVersion` in `internal/sheet/client.go` stays at 3. No new `_meta` rows; no new tab columns; no `apps-script/` source files touched. Verifier MUST run the same grep gate Phase 6/7/8 used:

```bash
grep -nE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go
grep -nE 'SCRIPT_MIN_SCHEMA_VERSION.*3' apps-script/src/lib/migrations.ts
# Both must match exactly. Any change is a phase-scope violation.
```

### D-07 — Test discipline

- All 4 fix plans land unit tests in the same package as the code change (`tray_test.go`, `config_test.go`, `runapp_test.go`). No new test infrastructure introduced.
- OPS-07 is the only fix without a clean unit-test path (`FreeConsole` is OS-level; mocking it adds more code than the fix itself). Acceptance is via manual smoke in the v1.0.2 release plan: launch v1.0.2 from a PowerShell session via `& exe`, close the shell, confirm the watcher continues running and emits a structured-log line proving detachment. Smoke evidence captured in 09-05.
- The existing `336/336 vitest green` on the apps-script side is untouched (Phase 9 makes zero apps-script changes). The Go test suite count grows by ~5-10 tests across the 4 fix plans.

### D-08 — Ship gate: tag `v1.0.2` + GitHub Release + `latest.json` refresh

Identical shape to v1.0.1 Phase 6 ship gate (Plan 06-05). Reuse `release.yml` GitHub Action unchanged where possible. The only delta from Phase 6's release plan: this is a fresh binary, no installer-bundling concerns beyond what v1.0.1 already solved (NSIS pre-install shim, INST-06 graceful shutdown — both shipped in v1.0.1, untouched here).

**Auto-update path:** existing v1.0.1 watchers periodically fetch `latest.json` (Plan 02-06 OPS-04). Once `latest.json` advertises v1.0.2, the next fetch by any v1.0.1 watcher downloads `squirebot.exe.new`, swaps on next launch, and the user wakes up on v1.0.2 without doing anything. Invisible-UX tiebreaker is naturally satisfied by the existing auto-update infrastructure.

### Claude's Discretion

(Implementation details the planner picks during research/planning, not pre-locked here:)

- Whether the OPS-06 queue stores closures (`func()`) or typed action structs (`pendingAction{kind, payload}`). Closures are shorter; typed actions are easier to reason about in tests. Planner's call after looking at the existing tray test patterns.
- Whether `FreeConsole` lives inline in `main.go` with a runtime-OS check or in a build-tagged `console_windows.go` + stub `console_other.go`. Build-tagged is cleaner; planner picks based on existing build-tag patterns in the repo.
- Exact wording of the AUTH-07 status string (`"Auth error — sign in again"` vs `"Sign-in required"` vs whatever AUTH-05 uses). Mirror AUTH-05 if it has a canonical phrasing; otherwise pick the shorter option.
- Whether to add a one-line note to `docs/build-and-install.md` about the `Start-Process` gotcha alongside D-02. Optional belt-and-suspenders; not required for OPS-07 acceptance (the functional fix is sufficient).
- Whether the `bytes` import in `config.go` D-04 also drags along `bytes.TrimPrefix` use elsewhere. Don't speculatively refactor — just the one use.
- Smoke evidence format for OPS-07: a `.planning/phases/09-watcher-robustness-polish/09-02-SMOKE.md` capturing PS-launch log fragments is consistent with Phase 7's 07-03-SMOKE.md pattern, but a 5-line inline note in 09-02-SUMMARY.md is also fine. Planner picks.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents (researcher, planner, executor) MUST read these before acting:**

### Phase scope + requirements
- `.planning/ROADMAP.md` §"Phase 9: Watcher Robustness Polish" (lines 48–59) — goal, success criteria, ship gate
- `.planning/REQUIREMENTS.md` §"Recovery & Identity (AUTH-07)" + §"Tray Controller Robustness (OPS-06, OPS-07)" + §"Config Loader Robustness (CONFIG-01)" — exact acceptance text for all 4 fixes

### v1.0.1 UAT source of these findings (read before designing fixes — Phase 6 UAT log has empirical reproduction context)
- `.planning/phases/06-installer-overwrite-running-shim/06-05-SUMMARY.md` §"Finding C" (boot-time `invalid_grant`), §"Finding D" (T-06-20 wider impact = OPS-06), §"Finding F" (BOM intolerance), §"Finding H" (foreground-launched watcher death) — empirical logs + the original 2-3 fix-option menus that informed the decisions above
- `.planning/phases/06-installer-overwrite-running-shim/06-CONTEXT.md` — Phase 6's discussion log; contains the T-06-20 history that OPS-06 widens

### v1.0.1 release shape to mirror (09-05 reuses this)
- `.planning/phases/06-installer-overwrite-running-shim/06-05-release-tag-PLAN.md` — the canonical "tag + GitHub Release + latest.json refresh" plan template
- `.planning/phases/06-installer-overwrite-running-shim/06-05-SUMMARY.md` — what shipped and how
- `.github/workflows/release.yml` (if it exists at that path — planner confirms during research) — the reusable release workflow

### Watcher source-of-truth (surfaces under modification)
- `internal/tray/tray.go` (317 LOC, current) — Controller struct, `OnReady`, `SetStatus`, `SetIconHealth`, `ShowContinueSetup` / `HideContinueSetup`, `ShowReauthorize` / `HideReauthorize`, `SetSpreadsheetID`. The OPS-06 queue lands here.
- `internal/tray/tray_test.go` — existing offline test patterns; new queue tests follow the same shape (no `systray.Run`; assert on the Controller directly)
- `cmd/squirebot/main.go` (214 LOC, current) — entry point, `--uninstall-wipe-credentials` + `--quit` short-circuits, logging init, RunApp goroutine launch, `systray.Run` blocking call. D-02's `FreeConsole` lands here.
- `internal/app/runapp.go` (735 LOC, current) — `RunApp`, `rebuildTokenFromWincred` (or equivalent — planner confirms exact name), fast-fail path. D-03's classification + tray-call wiring lands here.
- `internal/app/reauth.go` (290 LOC, current) — existing AUTH-05 Reauthorize flow that AUTH-07's click handler reuses
- `internal/auth/` — `IsRevokedRefreshToken` classifier (referenced by D-03; planner confirms exact symbol name and signature during research)
- `internal/config/config.go` (114 LOC, current) — `Load()` and `Save()`; D-04's BOM strip lands in `Load()` between lines 54 and 62
- `internal/config/config_test.go` — existing config tests; the new `TestLoad_StripsUTF8BOM` lands here

### Release plumbing
- `internal/update/version.go` (or wherever the watcher version constant lives — planner confirms during research) — bump from `v1.0.1` to `v1.0.2`
- The `latest.json` manifest source-of-truth location (planner confirms during research; likely a static file in the release-workflow output)

### Project rules
- `./CLAUDE.md` — Go-side conventions (slog structured logging, fsnotify rules, schema-evolution guards, never trust fsnotify event payloads on Windows)
- `.planning/PROJECT.md` — core value, locked decisions (16 decisions as of 2026-05-12), constraints
- `.planning/research/PITFALLS.md` — 27 pitfalls catalogue; relevant for Phase 9: P12-ish (DPAPI / wincred), P13 (`Session.getActiveUser` vs `userinfo.email`) — neither directly tripped by these fixes but planner should re-scan during research
- `.planning/research/STACK.md` — locked stack (Go 1.24, fsnotify v1.7+, systray, wincred, lumberjack); none of these constraints change in Phase 9

### v1.0.1 ship history (context, not modified)
- `.planning/milestones/v1.0.1-ROADMAP.md` — full v1.0.1 archive
- `.planning/milestones/v1.0.1-REQUIREMENTS.md` — 8-REQ reconciliation
- `docs/build-and-install.md` — optional 1-line `Start-Process` belt-and-suspenders note from D-02 lands here

</canonical_refs>

<specifics>
## Specific Ideas

- **The "invisible UX" tiebreaker is a user-supplied phase-wide doctrine for v1.0.2.** Every gray area was decided by picking the path that minimizes guildie awareness of the failure mode. Carry this forward to Plan 10 if any test-quality decision turns out to be user-facing (it almost certainly won't be — apps-script tests are dev-facing).

- **OPS-06 queue is the load-bearing primitive that makes AUTH-07 invisible.** Without the queue, AUTH-07's boot-time tray calls land in the silent-no-op window and the fix doesn't work. This is the only cross-plan dependency in the phase; flag it prominently in the AUTH-07 plan header.

- **`FreeConsole` ordering nuance:** the watcher has TWO non-tray short-circuit paths (`--uninstall-wipe-credentials` writes to stderr; `--quit` opens a named event and writes a result line). Both must keep their inherited stdio. FreeConsole MUST fire AFTER both short-circuit checks return without exiting. The fix is structurally simple — just place the call after the `if len(os.Args) >= 2 ...` blocks at the top of `main()`. Confirmed by reading `cmd/squirebot/main.go:26-55`.

- **CONFIG-01 minimal-LOC discipline:** REQUIREMENTS.md says "≤5 LOC + 1 unit test". Don't blow past that. If the planner finds themselves writing more than 10 LOC for the source change, something is wrong. The bytes.TrimPrefix idiom + one new import line is the whole fix.

- **AUTH-07 reuses the existing AUTH-05 Reauthorize click handler.** No new wiring on the click side. The only new code is the BOOT-time error classification and tray-call sequence; the click handler is unchanged from Plan 02-04.

- **No `.continue-here.md` exists in `.planning/phases/09-watcher-robustness-polish/`** — there are no blocking anti-patterns to acknowledge before planning.

</specifics>

<deferred>
## Deferred Ideas

- **Pre-Ready button-click safety in tray controller** — currently structurally impossible (clicks can't fire until the menu is built in OnReady), but if Phase 11+ ever adds an alternate input path (hotkey, IPC), the queue's drain-under-mutex behavior should be re-examined. Defer until that surfaces.

- **Belt-and-suspenders `docs/build-and-install.md` `Start-Process` note for OPS-07** — optional alongside D-02's functional fix. Planner may include or skip per their judgment; not required for acceptance. Tiny risk-free addition.

- **Robustness sweep of other JSON readers** (auth/StoredToken JSON in wincred, latest.json manifest) for hypothetical user-edit foot-guns — out of scope; those files aren't user-edited. Re-evaluate if a similar foot-gun ever surfaces.

- **Pre-watch Sheets API health probe for boot-time auth confirmation** — explicitly rejected by D-03's invisibility tiebreaker (adds latency to every cold start for a rare failure mode). If telemetry ever shows the rebuild-error classifier misses real cases, revisit; for now the classifier is sufficient.

- **OPS-07 fix path (b) docs-only** — explicitly rejected as primary fix per D-02. May surface as backlog if the FreeConsole approach has unforeseen consequences (e.g., breaks a developer workflow), but no such risk identified during scout.

- **Migration shim for v1.0.1 users with revoked tokens BEFORE auto-updating to v1.0.2** — these users currently see "Initialising…" forever. Once v1.0.2 ships and they auto-update (next launch), the new boot-detection kicks in and they see the Reauthorize prompt. No special migration needed; the natural auto-update path resolves the population.

- **Test coverage threshold gates in Go CI** — none currently exist; not introduced in Phase 9. Defer to v1.1.

- **Consolidating the 4 fix plans into 1 mega-plan** — explicitly rejected per D-05. Per-requirement plans match Phase 6 shape and keep traceability clean for the v1.0.2 milestone archive.

</deferred>

---

*Phase: 09-watcher-robustness-polish*
*Context gathered: 2026-05-12 via `/gsd-discuss-phase 9` with user-supplied tiebreaker rule: "simplest and most invisible end-user experience"*
