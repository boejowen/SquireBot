---
phase: 09-watcher-robustness-polish
plan: 09-03
name: config-bom-strip
type: standard
requirements: [CONFIG-01]
autonomous: true
wave: 1
depends_on: []
---

# Plan 09-03: UTF-8 BOM Strip in `config.Load()` (CONFIG-01)

## Objective

Strip a leading UTF-8 BOM (`\xEF\xBB\xBF`) from `%LOCALAPPDATA%\SquireBot\config.json` before `json.Unmarshal`, so guildies who hand-edit the file with Notepad or PowerShell 5.1 `Set-Content -Encoding utf8` (both write a BOM by default) don't see the failure mode `invalid character 'ï' looking for beginning of value`. Implementation per Phase 9 CONTEXT D-04: minimal `bytes.TrimPrefix` after `os.ReadFile`, before `json.Unmarshal`. ≤5 LOC source change + 1 new unit test.

## Context

- `.planning/phases/09-watcher-robustness-polish/09-CONTEXT.md` § D-04 — locked implementation (bytes.TrimPrefix after ReadFile, before Unmarshal)
- `.planning/REQUIREMENTS.md` § CONFIG-01 — acceptance text (≤5 LOC + 1 unit test)
- `internal/config/config.go` — current Load() lives at lines 52–85; BOM strip lands between line 54 (`os.ReadFile`) and line 62 (`json.Unmarshal`)
- `internal/config/config_test.go` — existing test patterns (uses `withTempConfig`, seeds via `os.WriteFile`)
- `./CLAUDE.md` — Go conventions (slog, never trust fsnotify event payloads on Windows; neither relevant to this plan)

## Tasks

### Task 1 — TDD RED: failing test for BOM strip

**Type:** auto (TDD red gate)

**Files:** `internal/config/config_test.go`

**Behavior:** Add a new test `TestLoad_StripsUTF8BOM` that:
1. Uses `withTempConfig` for the temp file path
2. Writes valid JSON bytes prefixed with the UTF-8 BOM (`\xEF\xBB\xBF`) via `os.WriteFile`
3. Calls `Load()` and asserts no error
4. Asserts that fields parsed correctly (e.g., `EQFolder == "C:\\P99"`, `SpreadsheetID == "abc123"`, `LogLevel == "info"`)

**Verification:** `go test ./internal/config/ -run TestLoad_StripsUTF8BOM` MUST fail with the JSON unmarshal error (the BOM bytes confuse `json.Unmarshal` — current behavior).

**Done when:** The test exists and fails with a JSON parse error proving the bug.

### Task 2 — TDD GREEN: strip leading BOM in `Load()`

**Type:** auto (TDD green gate)

**Files:** `internal/config/config.go`

**Implementation:**
1. Add `"bytes"` to the import block (alphabetic order: between `"errors"` is replaced — actually `"bytes"` sorts before `"encoding/json"`)
2. Between the `data, err := os.ReadFile(p)` block (line 54) and the existing `var c Config` / `json.Unmarshal(data, &c)` block (line 61–62), insert: `data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})`
3. Add a brief comment above the trim line explaining the foot-gun (Notepad/PS5.1 default to BOM-on-write)

**Verification:**
- `go test ./internal/config/` — all tests pass, including the new `TestLoad_StripsUTF8BOM`
- `go vet ./internal/config/` — clean
- `go build ./...` — builds

**Done when:** Full `go test ./internal/config/` is green; the BOM strip line is in place; no other files modified.

### Task 3 — Negative-path test: malformed BOM-only file still errors cleanly

**Type:** auto

**Files:** `internal/config/config_test.go`

**Behavior:** Add `TestLoad_BOMPrefixedInvalidJSONStillErrors` asserting that a file containing ONLY the BOM (no JSON body) returns an error (BOM strip should not magically transform an invalid file into success). This guards against accidentally over-broad fix scope.

**Verification:** `go test ./internal/config/ -run TestLoad_BOMPrefixedInvalidJSONStillErrors` passes.

**Done when:** Test exists, passes, and the error message is the standard `encoding/json` unexpected-EOF error path.

## Success Criteria

- [ ] `bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})` line lands in `internal/config/config.go` between `os.ReadFile` and `json.Unmarshal`
- [ ] `bytes` package imported (no other new imports)
- [ ] Source diff ≤ 10 LOC (REQUIREMENTS says ≤5; comment + import + 1 trim line = ~5)
- [ ] One new positive-path test (`TestLoad_StripsUTF8BOM`) — asserts BOM-prefixed config loads cleanly
- [ ] One new negative-path test (`TestLoad_BOMPrefixedInvalidJSONStillErrors`) — asserts the fix doesn't over-strip
- [ ] All existing config tests still pass
- [ ] `go vet ./...` clean for the package
- [ ] No changes to any other watcher files (no scope creep)

## Out of Scope

- BOM handling in `auth/StoredToken` JSON, `latest.json`, or any other JSON readers (per D-04 scope-discipline note; those files aren't user-edited)
- Whitespace stripping or any other input normalization beyond the documented BOM foot-gun
- BOM emission discipline in `Save()` — Go's `json.MarshalIndent` does not emit a BOM by default; nothing to change there
- Any tray, runapp, FreeConsole, or release-tag work (those are 09-01, 09-02, 09-04, 09-05)

## Verification (overall)

```bash
go test ./internal/config/ -v
go vet ./internal/config/
go build ./...
```

All three must succeed before commit of Task 3 / plan completion.
