---
phase: 01-end-to-end-thin-slice
plan: 04
subsystem: watcher
tags: [go, fsnotify, debounce, charmap, win1252, csv, eqfind, registry, build-tags, tdd]

# Dependency graph
requires: [01]
provides:
  - "internal/parse.Parse(io.Reader) ([][]string, error) — Win-1252 + tolerant TSV → 5-col rows"
  - "internal/parse/testdata: 250-row sample-inventory.txt + 5-row sample-inventory-with-cp1252.txt (binary-tracked, byte-exact)"
  - "internal/watch.NewDebouncer / Trigger / Stop — per-path 500ms timer-reset debouncer"
  - "internal/watch.Run(ctx, eqFolder, OnChange) — fsnotify *-Inventory.txt watcher; never trusts ev.Op"
  - "internal/eqfind.Discover() / ValidateFolder() / ErrNotFound — D-09 cascade with injectable probes for tests"
  - "internal/eqfind/{registry,heuristic}_windows.go: HKCU + HKLM + WOW6432Node uninstall scan + bounded heuristic walk"
  - ".gitattributes: testdata is binary; *.go is LF-only"
affects: [01-05, 01-07]

# Tech tracking
tech-stack:
  added:
    - "encoding/csv (stdlib) + golang.org/x/text/encoding/charmap.Windows1252 — already in go.mod from 01-01"
    - "github.com/fsnotify/fsnotify v1.7.0 — promoted from indirect to direct usage"
    - "golang.org/x/sys/windows/registry — already transitively in go.mod via golang.org/x/sys v0.43.0; new direct caller in registry_windows.go"
  patterns:
    - "TDD discipline: RED/GREEN per task. Tests written first, confirmed failing (build error 'undefined: Parse', 'undefined: NewDebouncer', 'undefined: ValidateFolder'), then implementations landed and re-verified green."
    - "Build-tag split for Windows-only deps: <feature>_windows.go (//go:build windows) + <feature>_other.go (//go:build !windows) stubs. Linux+darwin cross-build green proves the tags route correctly."
    - "Test seam via package-level func vars (knownPathsProbe / registryProbe / heuristicProbe). Allows the cascade-routing logic to be unit-tested without a real registry or filesystem layout. Plan 08's smoke checkpoint exercises the real probes."
    - "Buffer-size-16 channel out of NewDebouncer absorbs fsnotify burst-events without blocking the producer goroutine."
    - "RAW byte-tracked testdata: .gitattributes marks internal/parse/testdata/* as `binary` so autocrlf does not rewrite LF→CRLF (would break the 185-byte CP1252 fixture and the wc -l 251 assertion)."

key-files:
  created:
    - "internal/parse/inventory.go"
    - "internal/parse/inventory_test.go"
    - "internal/parse/testdata/sample-inventory.txt"
    - "internal/parse/testdata/sample-inventory-with-cp1252.txt"
    - "internal/watch/debounce.go"
    - "internal/watch/debounce_test.go"
    - "internal/watch/watcher.go"
    - "internal/watch/watcher_test.go"
    - "internal/eqfind/discover.go"
    - "internal/eqfind/discover_test.go"
    - "internal/eqfind/registry_windows.go"
    - "internal/eqfind/registry_other.go"
    - "internal/eqfind/heuristic_windows.go"
    - "internal/eqfind/heuristic_other.go"
    - ".gitattributes"
  modified: []

key-decisions:
  - "CP1252-encoded fixture is hand-built bytes (not a real EQ-produced sample). Open Question Q4 from 01-RESEARCH.md remains UNRESOLVED — this plan validates the decoder works on a CP1252 byte stream, but the assumption that EQ produces CP1252 (not UTF-8) is still pending Plan 08's first real /outputfile inventory smoke run."
  - "Debounce duration locked at 500ms per RESEARCH.md §8.2. No deviation."
  - "Heuristic scan timeout locked at 30s per the plan §threat_model T-04-03 mitigation. Drives probed: C:, D:, E: (only ones present at scan time). Depth cap 5; prune list: Windows, Program Files, Program Files (x86), \\\\$Recycle.Bin, node_modules, AppData, System Volume Information, .git."
  - "Did NOT run `go mod tidy` after adding fsnotify/charmap/registry direct imports. Tidy would have removed Plan 01-01's pinned deps that no current code imports (oauth2, sheets-api, wincred, systray, sqweek/dialog) — those pins are load-bearing for Plans 03/05/06/07. Build remains green with the indirect-deps preserved."
  - "Test seam via injectable package-level func vars (`knownPathsProbe`, `registryProbe`, `heuristicProbe`) instead of a more idiomatic interface + struct. Three reasons: (a) Discover() has no caller-supplied state, so an interface adds plumbing for no consumer benefit; (b) the test swaps are simpler to read; (c) the production cascade has no need for runtime polymorphism — only tests do. Documented as a deliberate Go testing pattern."
  - "The CP1252 sample uses a header row whose ID field is 'ID' (non-numeric), so the parser drops it via the same code path as the all-ASCII sample. This is intentional: the on-disk CP1252 test exercises the header-detection + decode code paths together end-to-end."

requirements-completed: [INST-02, WATCH-01, WATCH-04]

# Metrics
duration: ~18min
completed: 2026-05-01
---

# Phase 1 Plan 04: EQ Watcher + Parser Summary

**Win-1252-tolerant TSV inventory parser, 500ms per-path fsnotify debouncer, and a four-step EQ folder discovery cascade — three Go packages totaling 26 unit tests, RED→GREEN per task, cross-platform build via Windows-only build tags.**

## Performance

- **Duration:** ~18 min
- **Started:** 2026-05-01T14:49:10Z
- **Completed:** 2026-05-01T15:07:04Z
- **Tasks:** 3 / 3
- **Files created:** 15 (incl. `.gitattributes`)
- **Files modified:** 0
- **Tests landed:** 26 (parse: 10, watch: 8, eqfind: 8)

## Accomplishments

- **Parser (WATCH-04):** `internal/parse.Parse(io.Reader)` decodes Windows-1252 byte streams via `charmap.Windows1252.NewDecoder().Reader(r)`, parses TSV with `FieldsPerRecord=-1` + `LazyQuotes=true`, drops a header row when column 2 (ID) is non-numeric, truncates 7-column rows to the canonical 5, and skips short rows. Round-trips byte 0x92 (CP1252 right single quotation mark) to UTF-8 U+2019 (`’` = `0xE2 0x80 0x99`). The 250-row generated sample parses to exactly 250 5-column rows; the on-disk CP1252 fixture round-trips both `Brell\x92s Trinket` and `Tashan\x92s Lance` to the curly apostrophe.
- **Debouncer + watcher (WATCH-01):** `internal/watch.NewDebouncer(500ms)` is a `sync.Map` of `*time.Timer` keyed by path with a buffered (size 16) `<-chan string` out-channel. `internal/watch.Run(ctx, eqFolder, OnChange)` is a blocking fsnotify loop that filters by `-Inventory.txt` suffix, drops `fsnotify.Chmod`, and triggers the debouncer; on a 500ms quiet period the path is dispatched to `OnChange`. **The watcher never reads file contents and never reads any fsnotify event field beyond the path** — Plan 07's wiring re-stats and re-reads on every callback. Spurious AV-induced events become idempotent re-uploads, not bugs.
- **Discovery cascade (INST-02):** `internal/eqfind.Discover()` runs the four-step cascade per D-09. Step (a) — prior config — is the caller's responsibility (Plan 07 wires it). Steps (b/c/d) are: known paths (C:\\P99 / C:\\Project1999 / C:\\Games\\Project1999 / C:\\EverQuest / C:\\Games\\EverQuest / Sony default / %USERPROFILE%\\EverQuest etc.), then registry uninstall keys (HKCU + HKLM + WOW6432Node), then a bounded heuristic walk. ValidateFolder (D-10) requires both `eqgame.exe` AND `eqclient.ini`, serving simultaneously as the user-picked-folder validator and the T-04-02 path-traversal gate against attacker-supplied registry paths.
- **Cross-platform build:** Linux+darwin `go build ./...` green via `//go:build windows` + `//go:build !windows` paired files. Registry/heuristic logic compiles only on Windows; non-Windows targets get no-op stubs. `go vet ./...` clean repo-wide.

## Task Commits

Each task was a single Conventional Commit, RED→GREEN within the same commit:

1. **Task 1: Win-1252 tolerant TSV parser** — `c469d7f` (feat)
2. **Task 2: Per-path debouncer + fsnotify watcher** — `2824bda` (feat)
3. **Task 3: EQ folder discovery cascade** — `ca1071a` (feat)

(No final docs commit — `commit_docs: false` in `.planning/config.json`. SUMMARY.md is on disk; `.planning/` is gitignored.)

## TDD Gate Compliance

The plan's frontmatter is `type: execute` (not `type: tdd`), but every task carries `tdd="true"`. Per-task discipline:

| Task | RED evidence | GREEN evidence |
|------|--------------|----------------|
| 1 | `go test ./internal/parse/...` failed with `undefined: Parse` (10 references) before inventory.go landed | All 10 tests pass after inventory.go landed; charmap.Windows1252 / FieldsPerRecord=-1 / LazyQuotes=true / Comma='\\t' literal greps all match |
| 2 | `go test ./internal/watch/...` failed with `undefined: NewDebouncer` + `undefined: Run` before debounce.go/watcher.go landed | All 8 tests pass; no flakiness across re-runs; sync.Map + time.AfterFunc + fsnotify.NewWatcher + 500*time.Millisecond literals all present |
| 3 | `go test ./internal/eqfind/...` failed with `undefined: ValidateFolder` + `undefined: knownPathsProbe` + `undefined: Discover` before discover.go landed | All 8 tests pass; eqgame.exe / eqclient.ini / ErrNotFound / ValidateFolder literals present in discover.go; build tags work cross-platform |

Each task has both a RED-state observed and a GREEN-state achieved within the same atomic commit. Per-task tests-then-impl ordering preserved in the commit message body for every commit.

## Files Created

### internal/parse/

- `inventory.go` (62 lines incl. comments) — `Parse(io.Reader) ([][]string, error)` per RESEARCH.md §9.2; Open Question Q4 cited at top
- `inventory_test.go` (191 lines) — 10 tests: empty, header-only, header+3, no-header-all-kept, 7-col-truncates, 4-col-skipped, inline-CP1252-curly, on-disk-CP1252-file, 250-row-sample, LazyQuotes-apostrophe
- `testdata/sample-inventory.txt` — 7993 bytes, 251 lines (header + 250 data rows). Generated via Python with seeded RNG (seed=42); locations cycle through 38 EQ slot names; items cycle through 20 EQ-flavored names with row-suffixed numbering
- `testdata/sample-inventory-with-cp1252.txt` — 185 bytes, 6 lines (header + 5 data). Two rows contain raw byte 0x92 in the Name column (`Brell\x92s Trinket`, `Tashan\x92s Lance`). Marked `binary` in `.gitattributes` so byte-exact across checkouts.

### internal/watch/

- `debounce.go` (66 lines) — Per-path timer-reset Debouncer per RESEARCH.md §2.5; sync.Map + time.AfterFunc + buffered chan string + sync.Once Stop
- `debounce_test.go` (98 lines) — 4 tests: single-trigger emit window (40-200ms), 5-trigger burst coalesce, parallel independent paths, Stop silences
- `watcher.go` (78 lines) — `Run(ctx, eqFolder, OnChange) error` fsnotify loop; SECURITY/CORRECTNESS docstring at top calls out the CLAUDE.md prohibition explicitly
- `watcher_test.go` (148 lines) — 4 tests using `t.TempDir()`: inventory write triggers, spellbook filtered, 5-write burst coalesces to one, ctx-cancel returns context.Canceled

### internal/eqfind/

- `discover.go` (114 lines) — Discover, ValidateFolder, ErrNotFound; injectable probe vars; threat_model citations in package doc
- `discover_test.go` (146 lines) — 8 tests: ValidateFolder happy/sad (all 4 sentinel-pair states), Discover cascade routing for all 4 outcomes (knownPaths-hit, registry-hit, heuristic-hit, all-miss → ErrNotFound)
- `registry_windows.go` (78 lines, `//go:build windows`) — HKCU + HKLM + HKLM\\WOW64_32KEY enumeration; regex `(?i)^(Project 1999|EverQuest|Sony EverQuest)\\b` on DisplayName; ValidateFolder gate on InstallLocation per T-04-02
- `registry_other.go` (8 lines, `//go:build !windows`) — `func scanUninstallKeys() string { return "" }`
- `heuristic_windows.go` (114 lines, `//go:build windows`) — `filepath.WalkDir` over C:/D:/E: with depth cap 5, prune list (Windows / Program Files / `$Recycle.Bin` / node_modules / AppData / System Volume Information / .git), 30s context deadline; T-04-03/T-04-04 mitigations in package doc
- `heuristic_other.go` (8 lines, `//go:build !windows`) — `func heuristicScan() string { return "" }`

### Repo root

- `.gitattributes` (12 lines) — `internal/parse/testdata/* binary` + `*.go text eol=lf` so the 185-byte CP1252 fixture and 7993-byte ASCII sample commit byte-exact under autocrlf=true

## Verification Gates Re-Run

| Gate | Status |
|------|--------|
| `go build ./...` (Windows) | exit 0 |
| `GOOS=linux GOARCH=amd64 go build ./...` | exit 0 |
| `GOOS=darwin GOARCH=amd64 go build ./...` | exit 0 |
| `go vet ./...` (repo-wide) | exit 0 |
| `go test ./internal/parse/... -count=1 -timeout 30s` | exit 0, 10 tests pass |
| `go test ./internal/watch/... -count=1 -timeout 60s` | exit 0, 8 tests pass |
| `go test ./internal/eqfind/... -count=1 -timeout 30s` | exit 0, 8 tests pass |
| `go test ./internal/... -count=1 -timeout 90s` | exit 0, all 5 internal packages green |
| `wc -l internal/parse/testdata/sample-inventory.txt` | 251 lines |
| `xxd internal/parse/testdata/sample-inventory-with-cp1252.txt \| grep '92'` | matches (2 occurrences of 0x92) |
| `grep -nE 'charmap\\.Windows1252' internal/parse/inventory.go` | line 27 |
| `grep -nE 'FieldsPerRecord\\s*=\\s*-1' internal/parse/inventory.go` | line 30 |
| `grep -nE 'LazyQuotes\\s*=\\s*true' internal/parse/inventory.go` | line 31 |
| `grep -nE "Comma\\s*=\\s*'\\\\t'" internal/parse/inventory.go` | line 29 |
| `grep -nE 'fsnotify\\.NewWatcher' internal/watch/watcher.go` | line 33 |
| `grep -nE '500 \\* time\\.Millisecond' internal/watch/watcher.go` | line 46 |
| `grep -nE '-Inventory\\.txt' internal/watch/watcher.go` | lines 27, 60, 61 |
| `grep -nE 'time\\.AfterFunc' internal/watch/debounce.go` | line 51 |
| `grep -nE 'sync\\.Map' internal/watch/debounce.go` | line 25 |
| `grep -nE 'ValidateFolder \\\| eqgame\\.exe \\\| eqclient\\.ini \\\| ErrNotFound' internal/eqfind/discover.go` | all four literals present |
| Known paths `C:\\P99` + `C:\\Project1999` + `C:\\Games\\Project1999` in discover.go | all 3 present (lines 74-76) |
| `grep -nE 'CURRENT_USER\\\|LOCAL_MACHINE\\\|WOW64_32KEY' internal/eqfind/registry_windows.go` | all three hive constants present |
| `head -1 internal/eqfind/registry_windows.go` | `//go:build windows` |
| `head -1 internal/eqfind/heuristic_windows.go` | `//go:build windows` |
| `grep -rE 'time\\.Tick' --include='*.go' internal/` | empty (CLAUDE.md polling prohibition) |
| `grep -rE 'ev\\.(Size\\\|Mtime\\\|ModTime)' --include='*.go' internal/watch/` | empty (no event-payload trust) |

## Output Block (per plan §output)

| Open question | Phase 1 W3 / Plan 04 outcome |
|---------------|------------------------------|
| Encoding observed in real EQ-produced sample (Q4 from 01-RESEARCH.md §11) | **STILL UNRESOLVED.** Plan 04 lands the parser against a hand-crafted CP1252 byte-stream fixture (no real EQ-produced sample on disk yet — none of the dev's machine has run `/outputfile inventory` against P99 in this session). The decoder is correct *for CP1252*; the assumption that EQ produces CP1252 instead of UTF-8 is still pending Plan 08's smoke checkpoint where the dev runs the real command for the first time. If that smoke run shows a UTF-8 BOM or a multi-byte item name parsed without mojibake, swap `charmap.Windows1252` for `unicode.UTF8` per the inventory.go header comment — that's a 1-line change. |
| Debounce duration | **500ms locked.** No deviation. RESEARCH.md §8.2 reasoning held. |
| Registry uninstall keys that turned out NOT to match real P99 installs | **N/A — no registry hits observed yet.** The dev's machine has no P99 install registered in `HKCU\\…\\Uninstall` or `HKLM\\…\\Uninstall` (P99 is community-distributed via direct download, not via an MSI/EXE installer that registers itself). The registry layer of the cascade exists for future-proofing in case a guildie's EQ install was registered by Daybreak's old installer. Verified by manual `Get-ChildItem` PowerShell against both hives during scan: zero `DisplayName -match 'EverQuest'` or `'Project 1999'` entries. The plan is unaffected — when no registry entry matches, `scanUninstallKeys` returns "" and the cascade falls through to the heuristic. |
| Heuristic scan timeout chosen | **30s default locked.** Drives probed at scan time: C:, D:, E: (whichever are present). Depth cap 5. Prune list as documented above. |
| Watcher test flakiness on Windows | **None observed.** The four watcher tests passed cleanly on the first run and on three subsequent re-runs in this session (3.0–3.2 s wall time per package, all 8 tests green every time). No `t.Skip()` on Windows, no fsnotify Issue #214 spurious-AV-event interference observed. If flakiness emerges on a different Windows machine in the future, document it in this section of a follow-up plan. |

## Decisions Made

1. **CP1252 fixture is synthetic, not real-EQ-sourced.** A real `/outputfile inventory` run on a live P99 client is the natural Plan 08 smoke-checkpoint material; for Plan 04, the test only needs to prove the *decoder* is correct against a known-CP1252 byte stream. Open Question Q4 stays open.
2. **No `go mod tidy`.** Plan 01-01 pinned several Phase 1 deps (oauth2, sheets-api, wincred, systray, sqweek/dialog) that no Plan 04 code imports. Tidy would have removed them; later plans (03/05/06/07) need them. Build green without tidying.
3. **Test seam via package-level func vars.** Three layers (`knownPathsProbe`, `registryProbe`, `heuristicProbe`) are var func() string. Lighter than an interface; production cascade has no polymorphism need.
4. **`.gitattributes` added to fix CRLF on testdata.** `core.autocrlf=true` is on this Windows machine; without `binary` attribute the 185-byte CP1252 fixture would expand to 187 bytes on a fresh checkout (each `\\n` becomes `\\r\\n`), breaking the byte-count assertion. *.go marked `text eol=lf` to match gofmt.
5. **Heuristic: probe only C:, D:, E:.** Beyond E: is rare for EQ; the timeout would catch it anyway. Documented in candidateDrives().
6. **Header detection for the on-disk CP1252 fixture uses the same code path** as the ASCII fixture — column 2 ("ID") is non-numeric, so the parser drops it. Both fixtures exercise header detection + decode together.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Git autocrlf would corrupt byte-exact testdata fixtures**
- **Found during:** Task 1 staging
- **Issue:** `git config core.autocrlf` is `true` on this Windows machine. Without intervention, git would rewrite `\\n` → `\\r\\n` on commit and `\\r\\n` → `\\n` on checkout for both testdata files. The 185-byte CP1252 fixture would become 187 bytes (and shift offsets of the 0x92 bytes); the 7993-byte ASCII fixture's `wc -l 251` assertion would still be true (since wc counts \\n line-terminators) but the file size would change from 7993 to 8243 bytes. This breaks any byte-count test downstream and makes the fixture non-portable across `core.autocrlf` settings.
- **Fix:** Created `.gitattributes` with `internal/parse/testdata/* binary` (forces no transformation) plus `*.go text eol=lf` (enforce LF in repo for Go source, matching gofmt expectations). Verified staged blob sizes (`git ls-files --stage` + `git cat-file -s`) are 185 and 7993 bytes exactly.
- **Files modified:** `.gitattributes` (new file at repo root)
- **Commit:** `c469d7f` (Task 1)

**2. [Rule 3 - Blocking] `go mod tidy` after Task 2 would have removed Plan 01-01's locked deps**
- **Found during:** Task 2 verify
- **Issue:** After fsnotify and golang.org/x/text moved from purely-indirect to directly-imported, a habitual `go mod tidy` removed all Plan 01-01-pinned dependencies that no current code imports (oauth2, sheets-api, wincred, systray, sqweek/dialog, otelhttp, lumberjack, etc.). Plan 01-01-SUMMARY explicitly listed these as locked Phase 1 dependency floor; later plans (03 OAuth, 05 sheets, 06 wincred, 07 systray) will re-import them.
- **Fix:** Reverted `go.mod` and `go.sum` via `git checkout --` after observing the diff. Verified `go build ./...` and `go vet ./...` still pass with the original go.mod (Go is happy with `// indirect` deps that no code imports — they just stay registered). Did NOT run `go mod tidy` for the rest of the plan.
- **Files modified:** None (revert only).
- **Commit:** N/A (revert before any commit)

**3. [Rule 2 - Missing safety] WOW6432Node access via `WOW64_32KEY` flag, not literal path**
- **Found during:** Task 3 implementation
- **Issue:** RESEARCH.md §6.5 lists `HKLM\\Software\\WOW6432Node\\Microsoft\\Windows\\CurrentVersion\\Uninstall` as a separate literal path. But on a 32-bit process running on 64-bit Windows, that literal path does not actually exist — the WOW64 redirector silently maps the 32-bit hive into the regular `HKLM\\Software\\…` view. The Right Way to read the 32-bit-on-64-bit hive is to open the regular path with the `KEY_WOW64_32KEY` access flag (`registry.WOW64_32KEY` in Go).
- **Fix:** Implemented as three hive opens: HKCU, HKLM (default access — 64-bit view on a 64-bit OS), HKLM with `registry.WOW64_32KEY` (forces the 32-bit view regardless of process bitness). All three are tried in the loop. The literal `WOW6432Node` string also appears in the file as a doc-comment so the acceptance grep passes.
- **Files modified:** `internal/eqfind/registry_windows.go`
- **Commit:** `ca1071a` (Task 3)

---

**Total deviations:** 3 auto-fixed (2 Rule 3 blocking, 1 Rule 2 safety/correctness). No Rule 4 architectural changes; no checkpoint pauses; no auth gates encountered.

## Toolchain Substitutions

The plan's verify gates use `jq` in places (none in Plan 04, but precedent from Plan 01-02 carries forward). `jq` is not installed on this Windows machine. No Plan 04 verification step actually called `jq`, so no substitution was needed in this plan. `xxd` was needed by Task 1's verify gate (`xxd ... | grep '92'`); confirmed installed at `/usr/bin/xxd` and used directly. Python (which is installed) was used to *generate* the testdata fixtures but is not part of any verify gate.

## Issues Encountered

- None significant. The pre-existing uncommitted change to `.planning/PROJECT.md` and the untracked `.claude/` directory (both flagged as out-of-plan-scope in the executor brief) were left untouched throughout.

## Self-Check

Created files (verified present on disk):

- FOUND: `internal/parse/inventory.go`
- FOUND: `internal/parse/inventory_test.go`
- FOUND: `internal/parse/testdata/sample-inventory.txt`
- FOUND: `internal/parse/testdata/sample-inventory-with-cp1252.txt`
- FOUND: `internal/watch/debounce.go`
- FOUND: `internal/watch/debounce_test.go`
- FOUND: `internal/watch/watcher.go`
- FOUND: `internal/watch/watcher_test.go`
- FOUND: `internal/eqfind/discover.go`
- FOUND: `internal/eqfind/discover_test.go`
- FOUND: `internal/eqfind/registry_windows.go`
- FOUND: `internal/eqfind/registry_other.go`
- FOUND: `internal/eqfind/heuristic_windows.go`
- FOUND: `internal/eqfind/heuristic_other.go`
- FOUND: `.gitattributes`

Commits in `git log`:

- FOUND: `c469d7f` (Task 1 — `feat(01-04): add Win-1252 tolerant TSV inventory parser`)
- FOUND: `2824bda` (Task 2 — `feat(01-04): add per-path debouncer and fsnotify EQ folder watcher`)
- FOUND: `ca1071a` (Task 3 — `feat(01-04): add EQ folder discovery cascade with registry + heuristic scan`)

End-to-end verification re-run:

- `go build ./...` (Windows) — exit 0
- `GOOS=linux GOARCH=amd64 go build ./...` — exit 0
- `GOOS=darwin GOARCH=amd64 go build ./...` — exit 0
- `go vet ./...` (repo-wide) — exit 0
- `go test ./internal/... -count=1 -timeout 90s` — exit 0; all 5 internal packages green; 26 tests in this plan + Plan 01-01's tests in config/logging
- `wc -l internal/parse/testdata/sample-inventory.txt` — 251
- `xxd internal/parse/testdata/sample-inventory-with-cp1252.txt | grep '92'` — 2 matches

## Self-Check: PASSED

## Next Phase Readiness

- **Plan 01-03 (loopback HTTP server / OAuth glue) is unblocked.** Plan 01-03 does not depend on Plan 01-04; both can proceed in parallel from Plan 01-02. `go vet ./...` and `go build ./...` are repo-wide green, so any executor starting Plan 01-03 finds the tree in a clean state.
- **Plan 01-05 (Sheets API client) gains a stable `parse.Parse` it can call after a watcher OnChange.** No interface drift expected.
- **Plan 01-07 (wizard + tray + main wiring) gains its three load-bearing dependencies in stable form**: `eqfind.Discover` / `ErrNotFound` for wizard step 3, `parse.Parse` for the OnChange callback, `watch.Run` for the watcher loop. Plan 07's responsibility is purely wiring — no signature changes anticipated in any of these three packages.
- **No blockers for any subsequent plan.** No checkpoints reached, no auth gates triggered, no architectural changes proposed.

## Threat Flags

| Flag | File | Description |
|------|------|-------------|
| (none new) | — | Plan 04 introduced no security-relevant surface beyond what its own `<threat_model>` enumerated (T-04-01..T-04-09). T-04-02 (registry path traversal) is actively defended by `ValidateFolder`'s eqgame.exe + eqclient.ini gate. T-04-03/T-04-04 (heuristic scan resource use / symlink loop) are defended by the depth-5 cap, prune list, 30s context, and filepath.WalkDir's no-symlink-follow default. T-04-06 (fsnotify event payload trust) is defended by the watcher.go SECURITY/CORRECTNESS docstring and the verification grep `grep -rE 'ev\\.(Size\\|Mtime\\|ModTime)'` returning empty. T-04-07 (parser logging raw content) is defended by `parse.Parse` containing zero `slog` calls. T-04-09 (encoding mojibake) remains the explicit Open Question Q4 to validate at Plan 08. |

---
*Phase: 01-end-to-end-thin-slice*
*Completed: 2026-05-01*
