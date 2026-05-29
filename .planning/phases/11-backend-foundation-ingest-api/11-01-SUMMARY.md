---
phase: 11-backend-foundation-ingest-api
plan: 01
subsystem: infra
tags: [pocketbase, sqlite, modernc, go, spike, cross-compile, bearer-auth, cron]

# Dependency graph
requires: []   # first plan of Phase 11; no prior-phase dependency (green-field backend)
provides:
  - "D-01 verdict: HAND-ROLLED Go fallback (net/http + modernc.org/sqlite + goose); do NOT adopt PocketBase"
  - "Empirical confirmation that PocketBase v0.39.0 ships modernc.org/sqlite v1.51.0 (no cgo) — the headline RESEARCH finding"
  - "Verified amd64 (linux/x86-64) CGO_ENABLED=0 cross-compile from Windows works for the SQLite stack (22.9 MB static ELF)"
  - "Throwaway spike harness under spike/pocketbase/ exercising probes a/b/c/d (to be deleted in 11-05)"
affects: [11-02, 11-03, 11-04, 11-05, 15-admin-web-forms-login]

# Tech tracking
tech-stack:
  added:
    - "github.com/pocketbase/pocketbase@v0.39.0 (SPIKE-ONLY — slated for removal in 11-05 per the FALLBACK verdict)"
  patterns:
    - "Custom bearer guard via crypto/subtle.ConstantTimeCompare (NOT framework JWT auth) — survives the verdict; reused in 11-04"
    - "Atomic full-snapshot replace as one DELETE+INSERT transaction — survives the verdict; reused in 11-03"

key-files:
  created:
    - "spike/pocketbase/main.go (throwaway harness — probes a/b/c)"
    - "spike/pocketbase/README.md (THROWAWAY marker + run/cross-compile instructions)"
    - ".planning/phases/11-backend-foundation-ingest-api/11-01-SUMMARY.md"
  modified:
    - "go.mod / go.sum (pinned pocketbase v0.39.0 + transitive deps — removed in 11-05 under FALLBACK)"
    - ".gitignore (ignore pb_data/ + spike-amd64 throwaway artifacts)"
    - ".planning/phases/11-backend-foundation-ingest-api/11-CONTEXT.md (appended <spike_verdict> note)"

key-decisions:
  - "VERDICT: HAND-ROLLED Go fallback — all 4 probes PASS technically, but the locked design bypasses PocketBase's two biggest leverage points (opaque-token auth, not PB auth-records; plain SQL tables, not PB collections), so PB's 22.9 MB pre-1.0 framework would add a migration tax for near-zero usable leverage"
  - "11-05 wires net/http ServeMux + time.Ticker/robfig/cron — no PocketBase APIs; removes the pocketbase dep + spike/ tree"
  - "P15 Discord OAuth2 (AUTH-08) is now hand-rolled golang.org/x/oauth2, NOT PocketBase's built-in provider"

patterns-established:
  - "Spike-as-decision-gate: throwaway harness exercises concrete PASS/FAIL probes, verdict recorded in SUMMARY + appended to CONTEXT, downstream plans branch on it without re-deciding"

requirements-completed: []   # spike: de-risks D-01 but delivers NO production requirement (BACKEND-* land in 11-02..07)

# Metrics
duration: 19min
completed: 2026-05-29
---

# Phase 11 Plan 01: PocketBase-as-Framework Spike Summary

**All four D-01 probes PASS technically (PocketBase v0.39.0 shares the same no-cgo `modernc.org/sqlite v1.51.0` driver the fallback uses), but the VERDICT is HAND-ROLLED Go fallback — the locked design bypasses PocketBase's auth-record and collection models, so its framework leverage doesn't apply.**

## Performance

- **Duration:** 19 min
- **Started:** 2026-05-29T20:54:00Z
- **Completed:** 2026-05-29T21:12:50Z
- **Tasks:** 3
- **Files modified:** 6 (3 created, 3 modified)

## Accomplishments

- Stood up a throwaway PocketBase-as-framework harness (`spike/pocketbase/main.go`) that boots `pocketbase.New()` as a library and wires probes a/b/c.
- Exercised all four D-01 probes against the pinned `pocketbase@v0.39.0` with **recorded PASS evidence** (not assertion): observed HTTP status codes, the cron log line at the minute boundary, and a verified static ELF binary.
- Empirically confirmed the headline RESEARCH finding: PocketBase v0.39.0 pulls `modernc.org/sqlite v1.51.0` transitively (pure Go, no cgo) — the exact driver the hand-rolled fallback would use — so the cgo-vs-cross-compile risk is a non-issue.
- Produced an **unambiguous verdict** (HAND-ROLLED Go fallback) recorded in this SUMMARY and appended to `11-CONTEXT.md`, so 11-05 can wire the HTTP/cron shell without re-deciding.

## Per-Probe Results (D-01 criteria a/b/c/d)

| Criterion | Probe | Outcome | Concrete Evidence |
|-----------|-------|---------|-------------------|
| **(a)** models `owner`/`character`/`inventory_item`/`spellbook_entry` + empty dimension table | Raw DDL via `app.DB().NewQuery("CREATE TABLE …").Execute()` inside `OnServe` | **PASS** | Server log shows all 5 `CREATE TABLE IF NOT EXISTS` (owner, character, inventory_item, spellbook_entry, item_master) executing on PocketBase's own SQLite handle, coexisting with PB's `_collections`/`_migrations` system tables. Seed `INSERT OR IGNORE` rows for owner/character also ran. |
| **(b)** per-guildie bearer-token ingest via a custom route doing atomic full-snapshot replace | `se.Router.POST("/api/v1/ingest", ingestHandler).Bind(&hook.Handler[*core.RequestEvent]{Func: bearerGuard})` + `RunInTransaction` DELETE+INSERT | **PASS** | `curl` results: **no token → HTTP 401**, **bad token → HTTP 401**, **valid `Bearer spike-test-token… → HTTP 200** `{"ok":true,"replaced":2}`. Server log shows each ingest ran `DELETE FROM inventory_item WHERE character_id = 1` then 2× `INSERT INTO inventory_item` inside the transaction. Re-POST returned `replaced:2` again — idempotent full-snapshot replace (shrink handled by the DELETE). Guard is a custom `crypto/subtle.ConstantTimeCompare`, **NOT** `apis.RequireAuth()`. |
| **(c)** host the P12 in-process enrichment cron via Go hooks | `app.Cron().MustAdd("spike-heartbeat", "*/1 * * * *", …)` | **PASS** | Server started `2026/05/29 16:06:28`; cron logged `spike: cron fired (probe c)` at `2026/05/29 16:07:00` — fired at the next minute boundary while serving, auto-started by PB. |
| **(d)** runs on the deployment-target arch (Hetzner US = `linux/amd64`) | `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build` from Windows | **PASS** (build-level) | Produced `spike-amd64`: **ELF 64-bit LSB executable, x86-64, statically linked, stripped** (ELF magic `7f 45 4c 46`), **22.9 MB**, `build_exit=0`. No C cross-toolchain needed (only SQLite dep is pure-Go modernc). Running on real hardware deferred to the 11-06/11-07 on-box smoke per the plan. |

**No HARD BLOCKER on any probe.** The technical feasibility question is answered YES across the board.

## VERDICT

**VERDICT: HAND-ROLLED Go fallback** — `net/http` ServeMux 1.22+ (`"POST /api/v1/ingest"` method+pattern routing, no router dependency) + `modernc.org/sqlite` + `goose`. **Do NOT adopt PocketBase.**

Reasoning (leverage-vs-opinion axis, per RESEARCH §"Spike verdict guidance"): All four probes pass, so this is judgment, not a technical block. It tips to the fallback because the **locked architecture bypasses PocketBase's two biggest leverage points** — (1) guild codes are opaque static tokens guarded by a custom `crypto/subtle` middleware, **not** PB's `apis.RequireAuth()`/JWT auth-record system (probe b is deliberately custom); and (2) the domain/dimension tables are **plain SQL tables** (A5), **not** PB collections, so they are invisible to PB's admin UI and auto-REST layer anyway. The product UI is locked to **SvelteKit static** (P14), not PB's admin UI, so PB's UI leverage doesn't reach the product; and P15's Discord login is a contained `golang.org/x/oauth2` task either way. Adopting PocketBase would therefore mean carrying a **22.9 MB pre-1.0 framework** with a standing migration tax (Pitfall 1: v0.23 was a near-total rewrite, "1h to a weekend"; pre-1.0 makes no stability promise) in exchange for **near-zero usable leverage**. The fallback is ~600 LOC, churn-free (stdlib + two stable libs: modernc v1.51.0, goose v3.27.1), fully owned, and matches the project's single-static-binary ethos (CLAUDE.md).

## What 11-05 branches on

The verdict is **FALLBACK**, so:

- **HTTP wiring (11-05):** stdlib `net/http` `ServeMux` with Go 1.22+ method+pattern routes — `mux.HandleFunc("POST /api/v1/ingest", handler)`. No `app.OnServe().BindFunc` / `e.Router.POST(...).Bind(...)`.
- **Scheduler skeleton (11-05):** a single `time.Ticker` goroutine (or `robfig/cron` if cron-expression scheduling is wanted) registering a no-op/heartbeat job. No `app.Cron().MustAdd`.
- **Transaction (11-03):** hand-rolled `*sql.Tx` via `db.BeginTx` (`_txlock=immediate` DSN → `BEGIN IMMEDIATE`), `DELETE … ; INSERT …`. No `app.RunInTransaction`.
- **Dependency disposition (11-05 cleanup):** **remove** `github.com/pocketbase/pocketbase` from `go.mod`/`go.sum` and **delete the `spike/pocketbase/` tree** — do not ship a pre-1.0 dep the production server won't use. (`go mod tidy` after deleting the spike's only importer drops the entire PB dependency subtree.)
- **Framework-agnostic business logic (11-02/03/04):** the bearer guard (`crypto/subtle.ConstantTimeCompare`), the parser port, and the atomic-replace store are **unaffected by the verdict** — they were written against stdlib/SQL shapes the spike already validated and are imported unchanged regardless of framework.

**P15 note (AUTH-08):** with PB rejected, Discord OAuth2 login is hand-rolled via `golang.org/x/oauth2` (Discord provider), not PocketBase's built-in OAuth2 provider. Surfaced in the appended `11-CONTEXT.md` `<spike_verdict>` note for the P15 planner.

## Task Commits

Each task was committed atomically:

1. **Task 1: Scaffold the spike harness and pin the spike-only dependency** — `b894008` (feat)
2. **Task 2: Run the four probes and cross-compile for amd64** — `adeaead` (chore — `.gitignore` hygiene; probe evidence recorded here)
3. **Task 3: Write the verdict to SUMMARY + append the CONTEXT note** — this commit (docs)

**Plan metadata:** committed with Task 3 (docs: complete plan).

## Files Created/Modified

- `spike/pocketbase/main.go` — throwaway harness: `pocketbase.New()`, raw-DDL tables (probe a), custom-guarded `POST /api/v1/ingest` + `RunInTransaction` atomic replace (probe b), `app.Cron().MustAdd` heartbeat (probe c).
- `spike/pocketbase/README.md` — THROWAWAY marker, probe descriptions, local-run + cross-compile instructions, security note.
- `go.mod` / `go.sum` — pinned `github.com/pocketbase/pocketbase@v0.39.0` (+ transitive `modernc.org/sqlite v1.51.0`); **to be removed in 11-05** under the FALLBACK verdict.
- `.gitignore` — ignore `pb_data/` (throwaway SQLite) and `spike-amd64` (throwaway build output).
- `.planning/phases/11-backend-foundation-ingest-api/11-CONTEXT.md` — appended a `<spike_verdict>` section with the VERDICT line + downstream consequences (original `<domain>`/`<decisions>`/`<deferred>` sections intact).
- `.planning/phases/11-backend-foundation-ingest-api/11-01-SUMMARY.md` — this file.

## Decisions Made

- **HAND-ROLLED Go fallback over PocketBase** (the verdict) — rationale above. Decisive enough that 11-05 picks `net/http`/`time.Ticker` without re-deciding.
- **Plain SQL tables in the spike (A5)** rather than PB collections — sufficient to prove coexistence (probe a) and consistent with the FALLBACK direction; no admin-UI/auto-REST dependency was created.
- **Custom `crypto/subtle` bearer guard, not `apis.RequireAuth()`** — guild codes are opaque static tokens (D-08), and this pattern is framework-agnostic so it survives the verdict and is reused in 11-04.

## Deviations from Plan

None — plan executed exactly as written. (The plan's `<read_first>` blocks quote the Oracle-era `arm64` cross-compile target; per CONTEXT D-12 and the RESEARCH "Host Change Addendum" — both authoritative — the target is `linux/amd64` for the Hetzner US x86 host, which is exactly what the plan's `<action>` and `<verify>` specify. Targeting amd64 followed the plan as written, not a deviation.)

## Issues Encountered

- **Stale changelog docs via the Context7 CLI:** the `ctx7 docs` fallback surfaced pre-v0.23 PocketBase API snippets (mixed `CHANGELOG_16_22` content). Resolved by reading the exact post-v0.23 API signatures directly from the pinned `pocketbase@v0.39.0` source in the module cache (`ServeEvent.Router`, `Route.Bind(*hook.Handler[T])`, `RequestEvent`, `App.RunInTransaction`, `Cron.MustAdd`, `dbx.Params`) — so the harness compiled against the real pinned API on the first build.

## User Setup Required

None — no external service configuration required (the spike runs entirely on the dev box; the throwaway artifacts are gitignored).

## Next Phase Readiness

- **D-01 is resolved.** 11-02 (verdict-agnostic business logic: parser port, store, auth) and 11-05 (verdict-dependent HTTP/cron wiring → now `net/http` + `time.Ticker`) can both proceed.
- **11-05 carries two cleanup chores:** remove the `pocketbase` dependency (`go mod tidy` after deleting `spike/pocketbase/`) and delete the spike tree.
- **No blockers.** Host (Hetzner Cloud VPS, US, amd64) + DB (SQLite) decisions reaffirmed as verdict-independent.

## Self-Check: PASSED

- Files on disk: `spike/pocketbase/main.go` FOUND, `spike/pocketbase/README.md` FOUND, `11-01-SUMMARY.md` FOUND.
- Commits exist: `b894008` (Task 1) FOUND, `adeaead` (Task 2) FOUND, `b797c64` (Task 3) FOUND.
- `go build ./spike/...` → exit 0 (the throwaway spike compiles in-tree against the pinned PocketBase v0.39.0).
- Plan `<verify>`: `go build ./spike/pocketbase/` exit 0; `go list -m modernc.org/sqlite` → `v1.51.0`; amd64 cross-compile → `AMD64_BUILD_OK` (22.9 MB static ELF); all four probes PASS with evidence; `VERDICT:` present in BOTH 11-01-SUMMARY.md and 11-CONTEXT.md (`VERDICT_RECORDED_BOTH`).

---
*Phase: 11-backend-foundation-ingest-api*
*Completed: 2026-05-29*
