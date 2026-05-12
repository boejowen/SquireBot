---
phase: 09-watcher-robustness-polish
plan: 09-03
subsystem: watcher/config
tags: [config, robustness, foot-gun-fix, tdd]
requirements: [CONFIG-01]
dependency_graph:
  requires: []
  provides: [bom-tolerant-config-load]
  affects: [internal/config/config.go, internal/config/config_test.go]
tech_stack:
  added: []
  patterns: [tdd-red-green, bytes.TrimPrefix-leading-BOM-strip]
key_files:
  created:
    - .planning/phases/09-watcher-robustness-polish/09-03-config-bom-strip-PLAN.md
    - .planning/phases/09-watcher-robustness-polish/09-03-config-bom-strip-SUMMARY.md
  modified:
    - internal/config/config.go
    - internal/config/config_test.go
decisions: []
metrics:
  duration_minutes: ~5
  completed_date: "2026-05-12"
  source_loc_added: 5
  test_loc_added: 69
  tests_added: 2
  commits: 3
---

# Phase 9 Plan 09-03: Config UTF-8 BOM Strip Summary

**One-liner:** `bytes.TrimPrefix` removes a leading UTF-8 BOM (0xEF 0xBB 0xBF) from `config.json` before `json.Unmarshal`, closing the Notepad / PowerShell 5.1 `Set-Content -Encoding utf8` foot-gun (encoding/json does NOT auto-strip BOMs).

## What Shipped

CONFIG-01 satisfied per Phase 9 CONTEXT D-04:

1. **Source change** (`internal/config/config.go`, 5 LOC including comment):
   - New `"bytes"` import
   - `data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})` between `os.ReadFile` and `json.Unmarshal` in `Load()`
   - 3-line explanatory comment naming the foot-gun (Notepad + PS5.1 BOM-on-write)
2. **Positive-path test** (`TestLoad_StripsUTF8BOM`): seeds a BOM-prefixed valid JSON config via `os.WriteFile`, calls `Load()`, asserts no error and all fields parse correctly. Failed before the source change (RED gate confirmed with exact error: `invalid character 'ï' looking for beginning of value`); passes after.
3. **Negative-path scope-discipline test** (`TestLoad_BOMPrefixedInvalidJSONStillErrors`): writes a BOM-only file (no JSON body), asserts `Load()` still errors with the `config load <path>:` wrapper preserved. Guards against over-broad BOM handling silently masking real corruption.

Both tests use the existing `withTempConfig` helper — no new test infrastructure introduced.

## How It Works (Implementation Notes)

- `bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})` is a no-op for files without a BOM (the prefix-match returns the original slice unchanged). Behavior for BOM-free `config.json` is byte-identical to pre-fix — zero risk of regression for the 99% case.
- Only the LEADING BOM is stripped, by design. JSON spec doesn't allow mid-document BOMs anyway (`json.Unmarshal` would still correctly reject them mid-token).
- `Save()` is unchanged. Go's `json.MarshalIndent` does not emit a BOM. Round-trip Save → hand-edit-in-Notepad → Load now works whether or not Notepad adds a BOM.
- The fix lands BEFORE the `Unmarshal` step but AFTER the file-not-exist short-circuit, so the missing-file zero-value Config path is unaffected.

## Verification Run

```bash
$ go test ./internal/config/ -v
=== RUN   TestLoadMissingReturnsZeroValue          --- PASS
=== RUN   TestSaveLoadRoundTrip                    --- PASS
=== RUN   TestSaveDoesNotEmitRefreshToken          --- PASS
=== RUN   TestAtomicSaveLeavesNoTmp                --- PASS
=== RUN   TestLoad_Phase1ConfigBackCompat          --- PASS
=== RUN   TestLoad_Phase2ConfigEQFoldersOnly       --- PASS
=== RUN   TestLoad_BothFieldsPresent               --- PASS
=== RUN   TestSaveLoad_BothFieldsPersist           --- PASS
=== RUN   TestLoad_MissingMtimeMapsAreInitialized  --- PASS
=== RUN   TestLoad_StripsUTF8BOM                   --- PASS  (new)
=== RUN   TestLoad_BOMPrefixedInvalidJSONStillErrors --- PASS  (new)
=== RUN   TestPathPointsUnderLOCALAPPDATA          --- PASS
ok      github.com/boejowen/SquireBot/internal/config  1.754s

$ go vet ./internal/config/          # clean (no output)
$ go build ./...                     # green (no output)
```

12/12 config tests pass. `go vet` clean. Full `go build ./...` succeeds.

**RED-gate evidence (pre-fix run on `0d6a864`):**

```
--- FAIL: TestLoad_StripsUTF8BOM (0.03s)
    config_test.go:288: Load with BOM-prefixed config should not error;
    got: config load <tmp>\config.json:
    invalid character 'ï' looking for beginning of value
```

This is exactly the user-facing error the requirement names. The fix in `24bdced` makes it disappear.

## Schema Impact

**NONE.** `_meta.schema_version` stays at 3. `WatcherMaxSchemaVersion` in `internal/sheet/client.go` stays at 3 (verified — `grep -nE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go` returns line 44). No apps-script files touched. No new `_meta` rows or tab columns introduced. CONFIG-01 is a Go-side parse-tolerance fix only.

## Commits

| Hash | Message |
|------|---------|
| `0147582` | `docs(09-03): create config BOM strip plan (CONFIG-01)` |
| `0d6a864` | `test(09-03): add failing test for UTF-8 BOM tolerance in config.Load() (CONFIG-01)` (TDD RED) |
| `24bdced` | `fix(09-03): strip leading UTF-8 BOM in config.Load() (CONFIG-01)` (TDD GREEN) |

TDD gate sequence (RED → GREEN) satisfied. No REFACTOR commit was needed — the 5-LOC fix is already minimal.

## Deviations from Plan

None. Plan executed exactly as authored from CONTEXT D-04:

- 5 LOC source change (target was "≤5 LOC + 1 unit test" per REQUIREMENTS.md; landed at 5 LOC + 2 unit tests — one positive-path required, one negative-path added per plan Task 3 for scope discipline)
- 2 unit tests added (positive-path + scope-discipline negative-path); slight expansion over REQUIREMENTS.md's "1 unit test" minimum, justified by the scope-discipline guard documented in CONTEXT D-04 ("over-stripping risks masking real corruption")
- All existing config tests still pass
- No other watcher files modified
- No new dependencies

## Out of Scope (untouched, per plan)

- BOM handling in `auth/StoredToken` JSON, `latest.json`, or any other JSON readers — those files aren't user-edited (D-04 scope-discipline note)
- Any tray, runapp, FreeConsole, release-tag work (separate plans 09-01, 09-02, 09-04, 09-05)
- Apps-script side (Phase 10)

## Threat Flags

None. The fix tightens input tolerance for an existing trusted local file (`%LOCALAPPDATA%\SquireBot\config.json`); it does not introduce any new network surface, auth path, file access pattern, or trust boundary. The CLAUDE.md `SECURITY: NEVER add a refresh_token` invariant on `Config` is unchanged; `TestSaveDoesNotEmitRefreshToken` continues to pass.

## Self-Check

**Files claimed to exist:**
- `.planning/phases/09-watcher-robustness-polish/09-03-config-bom-strip-PLAN.md` — FOUND
- `internal/config/config.go` — FOUND (modified with BOM strip)
- `internal/config/config_test.go` — FOUND (modified with 2 new tests)

**Commits claimed to exist:**
- `0147582` — verified via `git log`
- `0d6a864` — verified via `git log`
- `24bdced` — verified via `git log`

## Self-Check: PASSED
