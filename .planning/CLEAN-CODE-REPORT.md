# Church of Clean Code — Audit Report

**Date:** 2026-06-02
**Scope:** Go watcher only — `internal/`, `cmd/`, `assets/` (apps-script/, web/, node_modules/ excluded)
**Mode:** Audit-only (read-only; no source files modified during the audit)
**Crusades run (9):** arch · dead-code · size · naming · test · observability · secret · dep · git
**Crusades skipped:** type (TypeScript-only), a11y/copy/adaptive/react (frontend-only — N/A to a Go tray app)

> Headline: this is an unusually clean codebase. Zero dead code, zero leaked secrets, near-immaculate
> git history, idiomatic Go naming, well-instrumented logging. The real findings cluster in two pews:
> **test coverage gaps** and **dependency freshness**.

---

## Verdict by pillar

| Pillar | Verdict | Worst finding |
|---|---|---|
| 🏛️ Architecture | ✅ Secure | 2× LOW (mechanism leaks) |
| 💀 Dead Code | ✅ Zero corpses | None |
| 📏 Size | ✅ Excellent | 1× duplication smell |
| 🏷️ Naming | ✅ Pristine | 1× INFO (cosmetic) |
| 🧪 Tests | ⚠️ Two real holes | 2× CRITICAL |
| 🔭 Observability | ✅ Walks in the light | 2× MEDIUM |
| 🔐 Secrets | ✅ DEFCON 5 clear | 1× LOW (.gitignore) |
| 📦 Dependencies | ⚠️ Tidy but unscanned | 3× WARN |
| 🌿 Git | ✅ Near-immaculate | 1× LOW (.claude/) |

---

## 🔴 CRITICAL — worth fixing first (test coverage)

### C1. `makeOnSpellbookChange` has ZERO tests — `internal/app/runapp.go:389`
A near-byte-for-byte clone of `makeOnInventoryChange` (`runapp.go:314`), which has 4 solid tests. The
spellbook upload path — one of the two reasons this app exists — has no proof its 401 / 426 /
cross-owner / empty-file / mtime-persist branches work. Twin code rots independently: a fix to one path
silently skips the other.

### C2. `internal/eqfind` real discovery is ~15% covered
`discover.go` (`defaultKnownPaths`, `defaultRegistryProbe`, `defaultHeuristicScan`), all of
`heuristic_windows.go` (`heuristicScan`, `candidateDrives`, `walkRoot`), and `registry_windows.go`
(`scanUninstallKeys`) are 0%. The orchestration layer is well-tested with injected fakes, but the actual
filesystem walk that finds a guildie's EQ folder on a fresh install is unproven. `walkRoot` /
sentinel-matching is testable against a `t.TempDir()` tree.

> **Two-birds note (Size + Test agreed):** `runapp.go:314`/`:389` are duplicated twins (~50 copy-pasted
> lines incl. a verbatim error-switch at `:355-372` ≡ `:419-437`). Refactoring them into one
> `makeOnFileChange(kind, suffix, mtimeMap …)` helper **closes C1 AND kills the duplication in one move.**
> Highest-leverage fix in the report.

---

## 🟡 WARN — Dependencies ("tidy but evidentially incomplete")

- **W1. `govulncheck` was not installed**, so no CVE certification was possible. The two least-trusted
  modules both sit in the watcher's most dangerous paths: `x/crypto`/`minisign` (auto-update signature
  check) and `fsnotify` (watch loop). Run `govulncheck ./...` before any release.
- **W2. `fsnotify v1.7.0` is two minors behind v1.10.1** — and v1.8–v1.10 carried Windows
  `ReadDirectoryChangesW` event-loss / buffer fixes directly relevant to this watcher. Bump and re-run
  the WATCH-07 regression suite.
- **W3. CLAUDE.md dependency doctrine is stale**: still lists `google.golang.org/api` + Sheets + oauth2
  as core watcher deps, but the v2.0 "Off Google" cutover made `google.golang.org/api` a ghost
  (`go mod why` → "main module does not need"); oauth2 is now server-only. ✅ **Fixed in this quick task.**

Other dep notes (INFO): minor drift on `fyne.io/systray` (v1.10.0→v1.12.1), `wincred` (v1.2.0→v1.2.3),
`aead.dev/minisign` (v0.2.0→v0.3.0); `sqweek/dialog` pinned to an untagged pseudo-version dragging in the
8-year-old `TheTitanrain/w32` shim (supply-chain note, onboarding path only). `go mod tidy -diff` clean,
`go mod verify` passes on all 110 modules. Watcher binary footprint = only 12 external modules; the
ClickHouse/moby/pgx bloat is server-side and adds zero bytes to the watcher.

---

## 🟢 Smaller / optional polish

- **Observability M2** — `ErrBadPayload` (400/422) is exported as a distinct sentinel but the consumer in
  `runapp.go:355-372` collapses it into the generic "upload failed", losing the "malformed envelope = bug,
  not network blip" signal. (M1: retry window is invisible to the tray until the budget exhausts —
  acceptable for a 12-person guild.)
- **Observability L1** — `update/swap.go` doc says "cannot use slog" but calls `slog.Info` at `:152`.
  L2 — auto-update *failures* never surface to the tray the way auto-update *readiness* does.
- **Git/Secret LOW** — `.claude/` was untracked and not gitignored (pollutes `git status`, `git add .`
  risk); `.gitignore` lacked `*.pem`/`*.key`/`.env`/`rclone.conf` defense-in-depth. ✅ **Fixed in this quick task.**
- **Architecture LOW** — `app/migrate.go` reaches `wincred` directly (bypassing `credstore`); `tray.go`
  shells `explorer.exe` inline (couples UI to platform I/O). Both documented; optional tidy-ups.
- **Naming INFO** — `minWikitextLength` vs `maxSummaryLen` asymmetry in `enrich/wikiitem.go`.
  ✅ **Fixed in this quick task** (renamed to `maxSummaryLength`).

---

## Commended (the fortress is well-built)

- Strict downward-only dependency DAG, zero import cycles, clean platform-split files; sink (`backend`)
  and identity store (`credstore`) properly isolated; load-bearing watcher→server write contract and the
  fsnotify-payload-distrust rule each confined to a single package.
- Zero dead code across 85 files / 26 packages / 2 binaries. `go vet` + dual-platform builds clean.
- Battle-ready hot path: `credstore` 100%, ingest client (401/409/426 terminal-no-retry, 5xx bounded
  retry, UTF-8 byte fidelity, **no-secret-in-logs**), self-update swap state machine, debouncer — all
  proven with specific assertions and proper test seams.
- DEFCON-5 secrets: no real credential ever committed (source or history). Bearer guild code lives only in
  DPAPI/`wincred`, never in `config.json` (regression-tested), never logged (V7, regression-tested). The
  Google client *ID* in planning docs is public by design, not a leak.
- Structured `slog` everywhere (zero string-concat logs), graded levels, lumberjack rotation to OPS-03
  spec, failures surfaced to the tray (with queue-and-replay before `OnReady`).
- 99.5% Conventional Commits compliance (543/546), atomic RED/GREEN TDD pairing, isolated `gofmt` commits,
  zero tracked build artifacts.

---

## Disposition

**Fixed in quick task `260602-u7m`:**
- `.gitignore` — added `.claude/` + `*.pem`/`*.key`/`.env`/`.env.*`/`!.env.example`/`credentials.json`/`rclone.conf`
- CLAUDE.md — reconciled stale watcher dependency doctrine (post-v2.0 "Off Google")
- `enrich/wikiitem.go` — `maxSummaryLen` → `maxSummaryLength`

**Deferred to a planned phase / backlog** (see `260602-u7m-deferred-items.md`):
- C1 + C2 + the twin-handler refactor (test work — not a quick fix)
- W1 `govulncheck` run + W2 `fsnotify` bump (needs toolchain; user installs toolchains)
- Observability M2 / L1 / L2 (optional)
- Architecture LOW tidy-ups (optional)
- Broader CLAUDE.md / STACK.md reconciliation (the Sheet-side, three-layer-pancake, and OAuth-scope
  sections still describe the pre-v2.0 Google Sheets world — a documentation overhaul, not a quick fix)
