# Phase 9: Watcher Robustness Polish — Discussion Log

**Date:** 2026-05-12
**Mode:** `/gsd-discuss-phase 9` — initial gray-area presentation interrupted; user supplied a phase-wide tiebreaker doctrine and delegated all decisions

---

## How this discussion went

The orchestrator analyzed Phase 9 and identified 4 gray areas (OPS-06 mechanism, OPS-07 path, AUTH-07 detection trigger, CONFIG-01 scope + plan structure) — each surfaced by v1.0.1 Phase 6 UAT findings with 2-3 candidate fix options apiece.

Before the multi-select question fully landed, the user supplied a phase-wide tiebreaker rule:

> "I have no preference for any of the questions you asked me. For each: please make whichever decision will make the end-user experience the simplest and most invisible."

This is effectively `--auto` mode with an explicit selection criterion. Claude locked all 4 decisions against that criterion, plus the supporting plan structure + ship gate + schema-impact assertions. See `09-CONTEXT.md` for the full decision rationale.

---

## Gray area: OPS-06 fix mechanism (Tray controller pre-Ready behavior)

**Options presented:**

(a) Controller queues `SetStatus` / `Show*` / `SetIconHealth` calls and replays in `OnReady`
(b) `app.RunApp` retries on the fast-fail path
(c) `systray.Quit()` deterministically on fast-fail so the user sees the process exit rather than a frozen tray

**User input:** "make whichever decision will make the end-user experience the simplest and most invisible"

**Decision:** (a) Queue-and-replay.

**Why:** Only (a) achieves invisible recovery — pre-Ready calls land in a FIFO queue and drain in `OnReady`, so the tray menu opens already showing the correct state regardless of what happened during the dark window. (b) leaves a transient bad-state window visible to the user; (c) makes the failure visible by killing the process.

**Captured as:** D-01 in `09-CONTEXT.md`.

---

## Gray area: OPS-07 path — FreeConsole vs docs

**Options presented:**

(a) `windows.FreeConsole()` early in `cmd/squirebot/main.go` (functional fix)
(b) Document `Start-Process` requirement prominently in `docs/build-and-install.md` (docs-only)
(c) Both

**User input:** "make whichever decision will make the end-user experience the simplest and most invisible"

**Decision:** (a) `FreeConsole()`.

**Why:** Functional fix means the guildie launches the exe any way (double-click, Start menu, `& exe` in PS, batch file, scheduler) and the watcher detaches cleanly. No incantation to learn, no docs to read. Docs-only forces the guildie to know about `Start-Process` — visible cognitive burden. Option (c) is fine but unnecessary; the planner may add a single-line belt-and-suspenders note in `docs/build-and-install.md` at their discretion (D-02 Claude's Discretion).

**Captured as:** D-02 in `09-CONTEXT.md`.

---

## Gray area: AUTH-07 detection trigger

**Options presented:**

- Probe Sheets API early at startup (extra cold-start latency)
- Treat any token-rebuild failure classified by `auth.IsRevokedRefreshToken` as the trigger
- Always show Reauthorize from boot with a status-bar error label

**User input:** "make whichever decision will make the end-user experience the simplest and most invisible"

**Decision:** Classify on rebuild-error, fire red icon + visible Reauthorize from boot (reuses the AUTH-05 click handler).

**Why:** The pre-watch probe adds latency to every cold start (visible to healthy users) for a failure mode that fires for one guildie in 100 — fails the invisibility tiebreaker. "Always show Reauthorize" pollutes the menu for healthy users — also fails. Classifying matches the existing AUTH-05 running-state behavior: Reauthorize appears IFF auth is broken. Symmetric and predictable. Depends on D-01 to be functional (calls land pre-Ready; queue replays them in `OnReady`).

**Captured as:** D-03 in `09-CONTEXT.md`.

---

## Gray area: CONFIG-01 scope + plan structure

**Sub-question A: BOM-strip scope**

**Options:**
- Strip only the leading UTF-8 BOM in `config.Load()`
- Broader robustness sweep across all JSON readers in the watcher

**Decision:** Strip only the leading BOM in `config.Load()`. ~3 LOC + 1 unit test. The other JSON readers (wincred-stored OAuth token, `latest.json` from the update server) are programmatically written by trusted code paths and cannot have BOMs.

**Captured as:** D-04 in `09-CONTEXT.md`.

**Sub-question B: Plan structure**

**Decision:** 5 plans mirroring Phase 6's release shape — 1 plan per requirement (09-01 OPS-06, 09-02 OPS-07, 09-03 CONFIG-01, 09-04 AUTH-07) + 1 release-tag plan (09-05). Wave 1 = 01/02/03 parallel; Wave 2 = 04 (depends on 01 for the queue); Wave 3 = 05.

**Captured as:** D-05 in `09-CONTEXT.md`.

---

## Scope creep handled

None surfaced. The 4 fixes are tightly scoped by REQUIREMENTS.md and the UAT log; no temptation to expand. Hard non-goals (no schema bump, no apps-script changes, no SignPath, no new tray menu items, no broader doc backfill) restated in `<domain>` for downstream agents.

---

## Deferred ideas

- Belt-and-suspenders docs note for OPS-07 (planner's discretion)
- Pre-watch Sheets API probe for AUTH-07 (rejected by invisibility tiebreaker; revisit if telemetry ever shows the rebuild-classifier misses real cases)
- Robustness sweep of other JSON readers (out of scope; not user-edited)
- Test coverage threshold gates in Go CI (defer to v1.1)
- Consolidating 4 fix plans into 1 mega-plan (rejected per D-05; per-REQ plans match Phase 6 shape and milestone-archive cleanliness)

Full deferred list in `09-CONTEXT.md` `<deferred>` section.

---

## Claude's Discretion items

(Listed in `09-CONTEXT.md` D-08 "Claude's Discretion" — closures vs typed actions for queue, build-tag-file vs inline runtime check for FreeConsole, exact wording of AUTH-07 status string, whether to include the belt-and-suspenders docs note, smoke evidence format for OPS-07.)

---

*Discussion log generated: 2026-05-12.*
