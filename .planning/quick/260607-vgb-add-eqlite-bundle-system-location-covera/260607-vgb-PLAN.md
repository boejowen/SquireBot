---
phase: 260607-vgb
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/eqfind/knownpaths_other.go
  - internal/eqfind/heuristic_other.go
  - internal/eqfind/heuristic_other_test.go
  - internal/eqfind/knownpaths_other_test.go
  - internal/eqfind/knownpaths_windows.go
  - internal/eqfind/knownpaths_windows_test.go
autonomous: true
requirements: [QUICK-260607-vgb]
commit_docs: false

must_haves:
  truths:
    - "On Linux, a P99 EQLite bundle at a system path (/opt/everquest/EQLite) is found by eqfind.Discover() without the manual folder prompt"
    - "On Linux, an EQLite bundle inside a WINE prefix drive_c is found via the direct-hit known-paths layer"
    - "On Linux, an EQLite bundle under $HOME (~/EQLite, ~/Desktop/EQLite, ~/everquest/EQLite) is found via the known-paths layer"
    - "On Windows, an EQLite bundle at %USERPROFILE%\\Desktop\\EQLite (the maintainer's real location) is found by defaultKnownPaths()"
    - "All existing scan bounds are preserved: depth cap 5, 30s timeout, no symlink follow, prune lists, first-ValidateFolder-match-wins, depth `>` boundary unchanged"
  artifacts:
    - path: "internal/eqfind/knownpaths_other.go"
      provides: "EQLite WINE-subdir hits, HOME hits, injectable systemKnownHits, all gated by ValidateFolder"
      contains: "systemKnownHits"
    - path: "internal/eqfind/heuristic_other.go"
      provides: "injectable systemCandidateRoots walked after wineCandidateRoots() under the same ctx deadline"
      contains: "systemCandidateRoots"
    - path: "internal/eqfind/knownpaths_windows.go"
      provides: "C:\\EQLite + USERPROFILE EQLite/Desktop/Downloads EQLite candidates"
      contains: "EQLite"
    - path: "internal/eqfind/heuristic_other_test.go"
      provides: "systemCandidateRoots-override test asserting heuristicScan() finds tmp/everquest/EQLite"
    - path: "internal/eqfind/knownpaths_other_test.go"
      provides: "WINE-subdir EQLite hit, ~/Desktop/EQLite HOME hit, systemKnownHits override hit"
    - path: "internal/eqfind/knownpaths_windows_test.go"
      provides: "USERPROFILE\\Desktop\\EQLite hit via t.Setenv + planted tree"
  key_links:
    - from: "heuristic_other.go heuristicScan()"
      to: "systemCandidateRoots via walkWineRoot"
      via: "second loop after wineCandidateRoots, same ctx.Done() check between roots"
      pattern: "systemCandidateRoots"
    - from: "knownpaths_other.go defaultKnownPaths()"
      to: "systemKnownHits via ValidateFolder"
      via: "first-match-wins ordering: WINE subdir hits -> HOME hits -> systemKnownHits"
      pattern: "systemKnownHits"
---

<objective>
Widen the watcher's EQ-folder autodetect (`internal/eqfind`) so a P99 "EQLite"
portable bundle is found without the manual folder prompt on BOTH Linux and
Windows. Today on Linux `eqfind.Discover()` ONLY scans WINE-prefix `drive_c`
roots, so an EQLite bundle at a system path (real case: `/opt/everquest/EQLite`)
is invisible -> the guildie gets the manual prompt. EQLite is also a real
relocatable bundle on Windows (maintainer's copy at `%USERPROFILE%\Desktop\EQLite`).

The validation sentinel is UNCHANGED: a folder with BOTH `eqgame.exe` AND
`eqclient.ini` (`eqfind.ValidateFolder`). The fix is name-aware ("EQLite") and
widens roots on Linux. SCOPE IS `internal/eqfind` ONLY (+ its tests).

Purpose: fresh-setup guildies on EQLite bundles skip the manual folder prompt.
Output: extended known-paths + heuristic coverage, build-tagged, fully test-covered.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@./CLAUDE.md

<scope_guards>
- DO NOT touch onboarding / config / cmd. `cmd/squirebot/main.go` (the `--setup`
  path) and `internal/app`/`internal/onboarding` are CONTEXT ONLY.
- NO schema/version/migration changes. NO `WatcherMaxSchemaVersion` bump (this is
  detection logic only — no write-contract change).
- DO NOT change the depth `>` boundary in heuristic_other.go. Switching
  `curDepth > maxHeuristicDepthOther` to `>=` is FORBIDDEN — the at-cap /
  beyond-cap tests pin Windows/Linux parity and would break.
- Keep ALL existing bounds: depth cap 5, 30s timeout, no-symlink-follow, prune
  lists, first-ValidateFolder-match-wins.
- `commit_docs` is FALSE — commit CODE ONLY (internal/eqfind/*.go). NEVER commit
  .planning/ files (the planner clobbers-uncommitted-docs lesson; verify with
  `git status` before each commit and stage only the eqfind files explicitly).
</scope_guards>

<interfaces>
<!-- Contracts the executor needs — extracted from the codebase. Use directly; no exploration. -->

From internal/eqfind/discover.go:
```go
// ValidateFolder enforces D-10: folder must contain BOTH eqgame.exe AND eqclient.ini.
func ValidateFolder(dir string) error  // returns nil iff both sentinels exist
```

From internal/eqfind/knownpaths_other.go (//go:build !windows) — CURRENT state:
```go
func defaultKnownPaths() string {
    var roots []string
    if wp := os.Getenv("WINEPREFIX"); wp != "" {
        roots = append(roots, filepath.Join(wp, "drive_c"))
    }
    if home := os.Getenv("HOME"); home != "" {
        roots = append(roots, filepath.Join(home, ".wine", "drive_c"))
    }
    subdirs := []string{
        "P99", "Project1999",
        filepath.Join("Program Files", "Project1999"),
        filepath.Join("Program Files (x86)", "Project1999"),
        "EverQuest",
        filepath.Join("Program Files", "Sony", "EverQuest"),
    }
    for _, root := range roots {
        for _, sub := range subdirs {
            p := filepath.Join(root, sub)
            if ValidateFolder(p) == nil { return p }
        }
    }
    return ""
}
```

From internal/eqfind/heuristic_other.go (//go:build !windows) — CURRENT heuristicScan:
```go
func heuristicScan() string {
    ctx, cancel := context.WithTimeout(context.Background(), heuristicScanTimeoutOther)
    defer cancel()
    for _, root := range wineCandidateRoots() {
        select {
        case <-ctx.Done():
            return ""
        default:
        }
        if got := walkWineRoot(ctx, root); got != "" { return got }
    }
    return ""
}
// walkWineRoot(ctx, root) string — depth-capped, prune-listed, no-symlink, first-match.
// wineCandidateRoots()'s add() guard: skip "" / os.Stat err / !IsDir / dup.
```

From internal/eqfind/knownpaths_windows.go (//go:build windows) — CURRENT candidates:
```go
candidates := []string{
    `C:\P99`, `C:\Project1999`, `C:\Games\Project1999`,
    `C:\EverQuest`, `C:\Games\EverQuest`,
    `C:\Program Files (x86)\Sony\EverQuest`,
}
if userProfile != "" {
    candidates = append(candidates,
        filepath.Join(userProfile, "EverQuest"),
        filepath.Join(userProfile, "P99"),
        filepath.Join(userProfile, "Project1999"),
    )
}
```

Test idioms to MIRROR (from heuristic_other_test.go):
- `plantSentinels(t, dir, "eqgame.exe", "eqclient.ini")` — already defined in the
  _other test file; reuse it.
- `isolateWineEnv(t, tmp)` — sets WINEPREFIX=tmp/.wine and HOME=tmp; reuse it to
  stop real roots leaking.
- save/restore an injectable package var: `orig := X; X = override; t.Cleanup(func(){ X = orig })`
  (see discover_test.go probe-var pattern).
</interfaces>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Linux (_other) EQLite + system-location coverage + tests</name>
  <files>internal/eqfind/knownpaths_other.go, internal/eqfind/heuristic_other.go, internal/eqfind/heuristic_other_test.go, internal/eqfind/knownpaths_other_test.go</files>
  <behavior>
    - knownpaths_other: $WINEPREFIX/drive_c/EQLite (planted) -> defaultKnownPaths() returns it.
    - knownpaths_other: ~/Desktop/EQLite (planted, HOME isolated) -> defaultKnownPaths() returns it.
    - knownpaths_other: systemKnownHits overridden to []string{tmpHit} (planted) -> defaultKnownPaths() returns tmpHit.
    - heuristic_other: systemCandidateRoots overridden to []string{tmp}, tmp/everquest/EQLite planted -> heuristicScan() returns that dir.
    - Negative coverage stays intact: existing _other tests still pass (no behavior regressions, ordering preserved).
  </behavior>
  <action>
    EDIT internal/eqfind/knownpaths_other.go (//go:build !windows):
    1. Extend the `subdirs` slice with EQLite WINE-prefix hits:
       `"EQLite"` and `filepath.Join("everquest", "EQLite")`.
    2. After the WINE-root subdir loop, add a HOME-based direct-hit block: if
       `home := os.Getenv("HOME")` is non-empty, probe in order via ValidateFolder:
       `filepath.Join(home, "EQLite")`, `filepath.Join(home, "Desktop", "EQLite")`,
       `filepath.Join(home, "everquest", "EQLite")`; return the first that validates.
    3. Add a package-level INJECTABLE var (above defaultKnownPaths) for system hits:
       `var systemKnownHits = []string{"/opt/everquest/EQLite", "/opt/EQLite", "/usr/local/games/EQLite"}`.
       After the HOME block, loop `systemKnownHits` and return the first that
       ValidateFolder accepts.
    ORDERING (first-match-wins, do NOT reorder): WINE-prefix subdir hits ->
    HOME hits -> systemKnownHits. Return "" only when all miss.

    EDIT internal/eqfind/heuristic_other.go (//go:build !windows):
    4. Add a package-level INJECTABLE var (near pruneNamesOther):
       `var systemCandidateRoots = []string{"/opt", "/usr/local/games", "/games"}`.
    5. In heuristicScan(), AFTER the existing wineCandidateRoots() loop and BEFORE
       `return ""`, add a second loop over the EXISTING dirs in systemCandidateRoots:
       - Stat-filter first (mirror wineCandidateRoots' add() guard): skip "" /
         os.Stat error / !fi.IsDir(). A small inline filter or a tiny helper is fine.
       - For each surviving root: `select { case <-ctx.Done(): return ""; default: }`
         (EXACTLY like the wine loop — honor the SAME ctx deadline between roots),
         then `if got := walkWineRoot(ctx, root); got != "" { return got }`.
       - Reuse walkWineRoot (depth cap 5, prune list, no-symlink, first-match) and
         pruneNamesOther as-is. Do NOT change maxHeuristicDepthOther or the `>` boundary.
       This generically catches /opt/everquest/EQLite at relative depth 2.

    ADD tests (build-tagged //go:build !windows; reuse plantSentinels + isolateWineEnv):
    6. heuristic_other_test.go — TestHeuristicScan_FindsEQUnderSystemRoot:
       `tmp := t.TempDir()`; `isolateWineEnv(t, tmp)` (so real wine/system roots
       can't leak and the planted system root is the only hit);
       save+override `systemCandidateRoots = []string{tmp}` with t.Cleanup restore;
       plant `tmp/everquest/EQLite/{eqgame.exe,eqclient.ini}`;
       assert `heuristicScan() == filepath.Join(tmp,"everquest","EQLite")`.
    7. knownpaths_other_test.go (NEW file, //go:build !windows, package eqfind) — three tests:
       (a) TestDefaultKnownPaths_WinePrefixEQLiteDirectHit: isolateWineEnv, plant
           `$WINEPREFIX/drive_c/EQLite` (= tmp/.wine/drive_c/EQLite), assert
           defaultKnownPaths() == that dir.
       (b) TestDefaultKnownPaths_HomeDesktopEQLiteHit: isolateWineEnv (HOME=tmp),
           plant `tmp/Desktop/EQLite`, assert defaultKnownPaths() == that dir.
       (c) TestDefaultKnownPaths_SystemKnownHitsOverride: isolateWineEnv,
           `hit := filepath.Join(t.TempDir(),"sys","EQLite")`, plant it, save+override
           `systemKnownHits = []string{hit}` with t.Cleanup restore, assert
           defaultKnownPaths() == hit.
       (plantSentinels + isolateWineEnv are already defined in heuristic_other_test.go
        in the same package+build-tag — reuse, do NOT redefine.)
  </action>
  <verify>
    <automated>$env:GOOS="linux"; go vet ./internal/eqfind/...; $env:GOOS=""</automated>
  </verify>
  <done>
    GOOS=linux go vet compiles the _other source AND the _other tests with no
    errors. New vars systemKnownHits + systemCandidateRoots present and injectable.
    Ordering preserved. Depth `>` boundary unchanged. Commit CODE ONLY:
    `git add internal/eqfind/knownpaths_other.go internal/eqfind/heuristic_other.go internal/eqfind/heuristic_other_test.go internal/eqfind/knownpaths_other_test.go`
    then commit `feat(eqfind): add EQLite + system-location coverage to Linux autodetect`.
    NOTE in commit body: _other tests are compile-verified via GOOS=linux vet on
    this Windows host; they EXECUTE on Linux/CI (Phase 25 pattern).
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Windows (_windows) EQLite coverage + test</name>
  <files>internal/eqfind/knownpaths_windows.go, internal/eqfind/knownpaths_windows_test.go</files>
  <behavior>
    - Windows: %USERPROFILE%\Desktop\EQLite (planted, USERPROFILE=tmp) -> defaultKnownPaths() returns it.
    - C:\ literals are present in the candidate list but are not unit-tested (not creatable in CI); rely on the env case.
  </behavior>
  <action>
    EDIT internal/eqfind/knownpaths_windows.go (//go:build windows):
    1. Append EQLite literals to the `candidates` slice:
       `` `C:\EQLite` ``, `` `C:\everquest\EQLite` ``, `` `C:\Games\EQLite` ``.
    2. In the existing `if userProfile != "" { ... }` block, also append:
       `filepath.Join(userProfile, "EQLite")`,
       `filepath.Join(userProfile, "Desktop", "EQLite")`,
       `filepath.Join(userProfile, "Downloads", "EQLite")`.
    First-match-wins; ValidateFolder is the gate (existing loop unchanged).

    ADD internal/eqfind/knownpaths_windows_test.go (NEW; //go:build windows; package eqfind):
    3. TestDefaultKnownPaths_UserProfileDesktopEQLiteHit:
       `tmp := t.TempDir()`; `t.Setenv("USERPROFILE", tmp)`;
       plant the sentinel pair (eqgame.exe + eqclient.ini, []byte{0}, via os.MkdirAll
       + os.WriteFile — this file is windows-tagged so plantSentinels from the _other
       file is NOT in scope; write a local helper or inline the two writes) into
       `filepath.Join(tmp, "Desktop", "EQLite")`;
       assert `defaultKnownPaths() == filepath.Join(tmp,"Desktop","EQLite")`.
       (No isolation of C:\ is needed: USERPROFILE points at tmp and a fresh CI
        runner has no C:\EQLite, so the Desktop hit is reached.)
  </action>
  <verify>
    <automated>go build ./... ; go vet ./internal/eqfind/... ; go test ./internal/eqfind/...</automated>
  </verify>
  <done>
    Native (windows) build + vet + test pass on this Windows host, including the
    new TestDefaultKnownPaths_UserProfileDesktopEQLiteHit (RUNS green here — it is
    NOT build-tagged out). Commit CODE ONLY:
    `git add internal/eqfind/knownpaths_windows.go internal/eqfind/knownpaths_windows_test.go`
    then `feat(eqfind): add EQLite coverage to Windows autodetect known-paths`.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| filesystem -> watcher autodetect | Local dirs/files are scanned; a planted sentinel pair would be "trusted" |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-vgb-01 | Tampering/Spoofing | system-root walk (/opt, /usr/local/games, /games) finding an attacker-planted eqgame.exe+eqclient.ini | accept | Inherits the Phase 25 T-25-07 accepted risk: local-only watcher, user owns their FS. ValidateFolder gate + depth cap 5 + 30s timeout + no-symlink-follow all preserved; system roots are stat-filtered to existing dirs and walked under the SAME ctx deadline (no new unbounded surface). |
| T-vgb-02 | DoS | extra system roots inflating scan time | mitigate | systemCandidateRoots is short (3 entries) and stat-filtered to existing dirs; shares the single 30s ctx deadline with ctx.Done() checks between roots; no new timeout budget added. |
</threat_model>

<verification>
Run from repo root on this Windows host. ALL must pass:
- `go build ./...`  AND  `$env:GOOS="linux"; go build ./...; $env:GOOS=""`
- `go vet ./internal/eqfind/...`  AND  `$env:GOOS="linux"; go vet ./internal/eqfind/...; $env:GOOS=""`
  (the GOOS=linux vet COMPILES the _other tests too — that is the only build-check
  the _other tests get on Windows)
- `go test ./internal/eqfind/... ./internal/app/... ./internal/onboarding/...`
- whole-module `go test ./...`

KNOWN LIMITATION (also state in SUMMARY): the new Linux (_other) tests CANNOT
EXECUTE on this Windows host (build-tagged !windows). They are compile-verified
via `GOOS=linux go vet` and run for real on a Linux box / CI (the Phase 25
pattern). Do NOT claim _other tests ran green on Windows. The Windows
(_windows) test DOES run green natively here.
</verification>

<success_criteria>
- Linux: EQLite found at WINE-prefix drive_c, under $HOME, and at system paths
  (/opt/everquest/EQLite etc.) without the manual prompt.
- Windows: EQLite found at C:\ literals and under %USERPROFILE% (incl. Desktop).
- All existing eqfind tests still pass; new tests cover each added path.
- Bounds preserved: depth cap 5, 30s timeout, no-symlink, prune lists,
  first-match-wins, depth `>` boundary unchanged.
- Two atomic CODE-ONLY commits (Task 1 Linux, Task 2 Windows). No .planning/
  files committed.
</success_criteria>

<output>
After completion, create
`.planning/quick/260607-vgb-add-eqlite-bundle-system-location-covera/260607-vgb-SUMMARY.md`.
In the SUMMARY, explicitly record the KNOWN LIMITATION (Linux _other tests
compile-verified via GOOS=linux vet only on this Windows host; execute on
Linux/CI) and the OUT-OF-SCOPE note (no version bump / tagged release — this
task only lands code; auto-update ships later; an already-configured watcher with
cfg.eq_folder set won't re-detect — this only helps fresh setups).
</output>
