# Phase 24: Watcher test hardening (C1/C2 coverage) - Pattern Map

**Mapped:** 2026-06-03
**Files analyzed:** 3 (1 modified, 2 created/extended)
**Analogs found:** 3 / 3

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/app/runapp.go` (refactor twin handlers + error switch) | utility / orchestrator | event-driven (file→POST) | itself (`makeOnInventoryChange`/`makeOnSpellbookChange` are each other's analog) | exact (intra-file dedup) |
| `internal/app/runapp_test.go` (add spellbook tests + cross-owner) | test | event-driven (file→POST) | the EXISTING `TestMakeOnInventoryChange_*` tests in the same file | exact |
| `internal/eqfind/<heuristic_windows_test.go>` (walkRoot/sentinel walk) | test | file-I/O (filesystem walk) | `internal/eqfind/discover_test.go` (`makeFakeEQDir` + `t.TempDir()`) | role-match (TempDir tree builder reusable; probe-swap pattern N/A for walkRoot) |

---

## Pattern Assignments

### `internal/app/runapp.go` — refactor (utility, event-driven). NO behavior change.

**This is a pure dedup refactor, not a new file.** The two handlers
`makeOnInventoryChange` (runapp.go:314-385) and `makeOnSpellbookChange`
(runapp.go:389-449) are byte-for-byte twins except for five tokens. The error
switch (`:355-372` ≡ `:419-437`) is verbatim-identical.

**The five token-level differences between the twin handlers (the ONLY things that vary by `kind`):**

| Concern | Inventory variant | Spellbook variant |
|---|---|---|
| suffix const | `watch.InventorySuffix` | `watch.SpellbookSuffix` |
| `Ingest` kind arg | `"inventory"` | `"spellbook"` |
| mtime map field | `cfg.LastKnownInventoryMtime` | `cfg.LastKnownSpellbookMtime` |
| slog `op` strings | `"stat inventory"`, `"open inventory"`, `"read inventory"`, `"upload inventory"`, `"uploaded inventory"`, `"inventory empty; skipping upload"` | same with `spellbook` |
| tray success/fail text | `"Last upload: %s at %s"` / `"Last upload failed: " + charName` | `"Last upload: %s spellbook at %s"` / `"Last upload failed: " + charName + " spellbook"` |

**Recommended target shape** — one `makeOnFileChange` parameterized by a small
descriptor, plus a `handleIngestErr` helper. Keep the success/fail tray strings
behavior-identical (the spellbook variant appends `" spellbook"`), so the
descriptor must carry a label fragment, NOT just a kind enum:

```go
// fileKind bundles the five tokens that vary between the inventory and
// spellbook upload paths. Adding a third /outputfile type later = one literal.
type fileKind struct {
	kind     string                       // backend.Ingest "kind" arg: "inventory" | "spellbook"
	suffix   string                       // watch.InventorySuffix | watch.SpellbookSuffix
	mtimeMap func(*config.Config) *map[string]string // &cfg.LastKnownInventoryMtime | &cfg.LastKnownSpellbookMtime
	// label is the human/op fragment used in slog ops and the tray "spellbook"
	// suffix. Inventory uses "" for the tray suffix to preserve the exact v1
	// wording ("Last upload: Foo at 15:04"); spellbook uses " spellbook".
}
```

> NOTE for planner: the inventory tray success line is `"Last upload: %s at %s"`
> (no kind word) while spellbook is `"Last upload: %s spellbook at %s"`. The
> refactor MUST preserve this asymmetry exactly (it is user-visible tray text).
> Cleanest: store a `traySuffix string` (`""` vs `" spellbook"`) and a
> `slogNoun string` (`"inventory"` vs `"spellbook"`) in the descriptor.

**Error-switch to extract verbatim** (runapp.go:355-372, identical at :419-437).
This is the `handleIngestErr(...)` helper. Move it out unchanged:

```go
// handleIngestErr maps a backend.Ingest error to its terminal tray/log
// reaction and reports whether the caller should stop (true) before persisting
// the mtime. nil err → returns false (proceed to persist). Extracted verbatim
// from the twin handlers (was runapp.go:355-372 ≡ :419-437); NO behavior change.
func handleIngestErr(err error, charName, traySuffix string, t *tray.Controller) (stop bool) {
	switch {
	case errors.Is(err, backend.ErrUnauthorized):
		slog.Warn("upload 401 — guild code invalid", "char", charName)
		t.SetIconHealth(tray.HealthRed)
		t.SetStatus("Guild code invalid — re-enter via the tray menu")
		return true // terminal; NO retry (D-5 / Pitfall 5)
	case errors.Is(err, backend.ErrVersionTooOld):
		slog.Warn("upload 426 — watcher too old", "char", charName)
		t.SetStatus("Update needed — SquireBot will auto-update")
		return true
	case errors.Is(err, backend.ErrCrossOwner):
		slog.Warn("cross-owner reject", "char", charName)
		return true
	case err != nil:
		slog.Error("upload", "char", charName, "err", err)
		t.SetStatus("Last upload failed: " + charName + traySuffix)
		return true
	}
	return false
}
```

> CAUTION: the original 5xx/default arm logs op `"upload inventory"` /
> `"upload spellbook"` (noun-specific). Folding to a single `"upload"` op is a
> log-string change, not a behavior change, but `handleIngestErr` is invoked by
> BOTH paths — either thread `slogNoun` in, or accept the one-word-shorter op.
> Recommend threading `slogNoun` to keep logs perfectly greppable (CLAUDE.md:
> "structured logging … keeps logs greppable").

**Backend error sentinels** (from `internal/backend/client.go:49-60`) used by the switch:
`backend.ErrUnauthorized` (401), `backend.ErrCrossOwner` (409), `backend.ErrVersionTooOld` (426).

**Constraint:** `extractCharNameForSuffix` (runapp.go:465) and the `charName == ""`
guard at the top of each handler must stay as-is — they already take the suffix as
a param, so they fold cleanly into the shared body. `rescanCatchUp` (runapp.go:255)
already demonstrates the desired "switch on suffix, pick the right map + cb" shape
and should be left untouched (it is the consumer of the two callbacks).

---

### `internal/app/runapp_test.go` — add spellbook tests + cross-owner (test, event-driven)

**Analog:** the four EXISTING inventory tests in the SAME file:
`TestMakeOnInventoryChange_204PersistsMtime` (:336), `_401NoLoopSetsRed` (:366),
`_EmptyFileSkipsNoRequest` (:397), `_426UpdateNeeded` (:424). Mirror these 1:1
for the spellbook path, then add ONE cross-owner (409) case.

**Reusable seam types already defined in this file — REUSE, do not re-create:**

- `ingestRecorder` (runapp_test.go:216-231) — httptest handler recording request
  count + body, returns a fixed status. Construct with `&ingestRecorder{status: <code>}`.
- `ingestRecorder.handler()` → `http.HandlerFunc` for `httptest.NewServer`.
- `ingestRecorder.requests()` → count assertion.
- `fastBackend(t, srv)` (runapp_test.go:244-249) — `*backend.Client` with
  near-zero retry backoff (`SetBackoffForTest([]time.Duration{0,0,0})`). Use for
  the 5xx/cross-owner path so the test does not sleep.
- `withTempLOCALAPPDATA(t)` (defined in `internal/app/migrate_test.go:18-23`, same
  package `app`) — `t.Setenv("LOCALAPPDATA", tmp)`; returns the config.json path so
  `cfg.Save()` lands under tmp. Idiom in every inventory test:
  `p := withTempLOCALAPPDATA(t); dir := filepath.Dir(filepath.Dir(p))` then write
  the `<Char>-Spellbook.txt` into `dir`.

**Exact harness shape to copy** (verbatim from `_204PersistsMtime`, swap inventory→spellbook):

```go
// TestMakeOnSpellbookChange_204PersistsMtime: a 204 from the backend persists
// the file's mtime into cfg.LastKnownSpellbookMtime and saves config.
func TestMakeOnSpellbookChange_204PersistsMtime(t *testing.T) {
	p := withTempLOCALAPPDATA(t) // redirect cfg.Save() under tmp
	dir := filepath.Dir(filepath.Dir(p))

	ir := &ingestRecorder{status: http.StatusNoContent}
	srv := httptest.NewServer(ir.handler())
	defer srv.Close()
	bc := fastBackend(t, srv)

	spbPath := filepath.Join(dir, "Foo-Spellbook.txt")
	if err := os.WriteFile(spbPath, []byte("9\tLifetap\n"), 0o644); err != nil {
		t.Fatalf("write spb: %v", err)
	}

	cfg := &config.Config{Version: 1, LogLevel: "info", LastKnownSpellbookMtime: map[string]string{}}
	tc := tray.NewController(tray.Config{})

	cb := makeOnSpellbookChange(context.Background(), bc, cfg, "CODE", "2.0.0", tc)
	cb(spbPath)

	if ir.requests() != 1 {
		t.Fatalf("backend saw %d requests, want 1", ir.requests())
	}
	if cfg.LastKnownSpellbookMtime["Foo"] == "" {
		t.Errorf("mtime not persisted for Foo after a 204")
	}
}
```

**Spellbook-vs-inventory swaps the new tests must make** (the ONLY diffs):

| Token | Inventory test | Spellbook test |
|---|---|---|
| filename | `Foo-Inventory.txt` | `Foo-Spellbook.txt` |
| file body | `"Belt\tThing\t1\t1\t0\n"` (5-col TSV) | `"9\tLifetap\n"` (Level\tName — see existing rescanCatchUp tests :101) |
| constructor | `makeOnInventoryChange(...)` | `makeOnSpellbookChange(...)` |
| mtime map field | `cfg.LastKnownInventoryMtime` | `cfg.LastKnownSpellbookMtime` |
| cfg seed map | `LastKnownInventoryMtime: map[string]string{}` | `LastKnownSpellbookMtime: map[string]string{}` |

> NOTE: if the refactor lands first, `makeOnSpellbookChange` may become a thin
> wrapper over `makeOnFileChange(spellbookKind, ...)`. Keep the test calling the
> SAME public-ish entry point the inventory tests call so the two suites stay
> symmetric. Do NOT have the spellbook tests call `makeOnFileChange` directly if
> the inventory tests call `makeOnInventoryChange` — mirror 1:1.

**New cross-owner (409) case** — there is currently NO inventory cross-owner test,
so add it for BOTH paths (or at least spellbook, per the prompt). Status code:
`http.StatusConflict` (409 → `backend.ErrCrossOwner`). Assertions mirror the 401
test (`_401NoLoopSetsRed`, :366): exactly 1 request (terminal, no retry) and the
mtime is NOT persisted:

```go
// TestMakeOnSpellbookChange_409CrossOwnerNoPersist: a 409 is terminal — exactly
// one request, no retry, and the mtime is NOT persisted (the upload was rejected).
func TestMakeOnSpellbookChange_409CrossOwnerNoPersist(t *testing.T) {
	p := withTempLOCALAPPDATA(t)
	dir := filepath.Dir(filepath.Dir(p))

	ir := &ingestRecorder{status: http.StatusConflict}
	srv := httptest.NewServer(ir.handler())
	defer srv.Close()
	bc := fastBackend(t, srv)

	spbPath := filepath.Join(dir, "Foo-Spellbook.txt")
	if err := os.WriteFile(spbPath, []byte("9\tLifetap\n"), 0o644); err != nil {
		t.Fatalf("write spb: %v", err)
	}

	cfg := &config.Config{Version: 1, LogLevel: "info", LastKnownSpellbookMtime: map[string]string{}}
	tc := tray.NewController(tray.Config{})

	cb := makeOnSpellbookChange(context.Background(), bc, cfg, "CODE", "2.0.0", tc)
	cb(spbPath)

	if ir.requests() != 1 {
		t.Fatalf("409 path made %d requests, want exactly 1 (terminal, no retry)", ir.requests())
	}
	if cfg.LastKnownSpellbookMtime["Foo"] != "" {
		t.Errorf("mtime persisted on a 409; want untouched")
	}
}
```

**Import block already present in runapp_test.go:1-19** (no new imports needed for
the spellbook+409 cases — `net/http` covers `StatusConflict`):
```go
import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/boejowen/SquireBot/internal/backend"
	"github.com/boejowen/SquireBot/internal/config"
	"github.com/boejowen/SquireBot/internal/credstore"
	"github.com/boejowen/SquireBot/internal/tray"
	"github.com/boejowen/SquireBot/internal/watch"
)
```

---

### `internal/eqfind/heuristic_windows_test.go` — walkRoot / sentinel-walk (test, file-I/O)

**Analog:** `internal/eqfind/discover_test.go` — specifically the `makeFakeEQDir`
helper (:13-25) and its `t.TempDir()` + sentinel-file planting idiom. The
function-var probe-swap pattern (`knownPathsProbe = func()...`) is the OTHER
eqfind test idiom but is NOT applicable here: `walkRoot`/`heuristicScan` are the
real filesystem walker, not swappable probes — you exercise them directly against
a planted `t.TempDir()` tree.

**CRITICAL BUILD-TAG CONSTRAINT (determines where this file lives + whether CI runs it):**
`walkRoot`, `candidateDrives`, `pruneNames`, and `heuristicScan` are ALL defined
in `internal/eqfind/heuristic_windows.go`, which is `//go:build windows`
(heuristic_windows.go:1). The non-Windows counterpart `heuristic_other.go`
(`//go:build !windows`) defines ONLY `heuristicScan() string { return "" }` — it
does NOT define `walkRoot`. Therefore:

- The new test file **MUST** carry `//go:build windows` as its first line, or it
  will fail to compile on Linux/macOS (`undefined: walkRoot`).
- It will **only run on the Windows CI leg**. `discover.go:108-110` already
  documents that the Windows heuristic scan "is not unit-tested in CI because it
  requires a real Windows filesystem" — this Phase 24 test partially closes that
  gap by driving `walkRoot` against a synthetic `t.TempDir()` tree (depth +
  prune + match), which DOES work on Windows CI.
- Suggested filename: `internal/eqfind/heuristic_windows_test.go` (the `_windows`
  infix gives implicit `GOOS=windows` constraint, but ALSO add the explicit
  `//go:build windows` line to match the convention in heuristic_windows.go and
  to be unambiguous).

> Planner decision point: if you want a test that runs on the dev's box too and
> the dev is on Windows (env shows `win32` / Windows 10), the `//go:build windows`
> test runs locally. The user's local `go test ./...` WILL exercise it. Good.

**File header to copy** (verbatim build-tag + package from heuristic_windows.go:1-12,
trimmed to the test's needs):

```go
//go:build windows

package eqfind

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)
```

**Sentinel-planting helper to copy/adapt** (from discover_test.go:13-25) — but
extend it to plant the pair at an arbitrary DEPTH under a root so `walkRoot`'s
depth+prune logic is exercised:

```go
// plantEQAt creates root/sub.../{eqgame.exe,eqclient.ini}. Returns the leaf dir
// holding the sentinel pair. Mirrors discover_test.go:makeFakeEQDir but lets the
// caller bury the install N levels deep to drive walkRoot's depth cap + prune.
func plantEQAt(t *testing.T, root string, sub ...string) string {
	t.Helper()
	leaf := filepath.Join(append([]string{root}, sub...)...)
	if err := os.MkdirAll(leaf, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, fname := range []string{"eqgame.exe", "eqclient.ini"} {
		if err := os.WriteFile(filepath.Join(leaf, fname), []byte{0}, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return leaf
}
```

**Test shape for walkRoot** (drive directly against a TempDir as the "root"; pass
a live `context.Background()` since the scan honors `ctx.Done()` at
heuristic_windows.go:76-80):

```go
func TestWalkRoot_FindsSentinelPairAtDepth(t *testing.T) {
	root := t.TempDir()
	want := plantEQAt(t, root, "Games", "Project1999")

	got := walkRoot(context.Background(), root)
	if got != want {
		t.Errorf("walkRoot = %q, want %q", got, want)
	}
}
```

**Suggested cases (per the prompt's "varying depths + decoys"):**
- Pair at depth 1, 2, 3 → found (within `maxHeuristicDepth = 5`, heuristic_windows.go:20).
- Pair buried BEYOND depth 5 → NOT found (depth cap prunes it; assert `""`).
- A pruned dir name (`node_modules`, `$Recycle.Bin`, `AppData` — from
  `pruneNames`, heuristic_windows.go:59-68) containing a full pair → NOT found
  (subtree pruned; assert `""`).
- A "decoy" dir with ONLY `eqgame.exe` (missing `eqclient.ini`) → NOT matched;
  the REAL pair elsewhere IS returned. Reuse the `omit` idea from
  `makeFakeEQDir(t, "eqclient.ini")` (discover_test.go:48).
- Empty tree (no sentinels anywhere) → `walkRoot` returns `""`.

> NOTE: `candidateDrives()` enumerates real `C:\ D:\ E:\` and is NOT TempDir-driven
> — do NOT test `heuristicScan()` end-to-end (it would walk the real C: drive and
> is the exact thing discover.go:108 calls un-unittestable). Test `walkRoot`
> DIRECTLY with a TempDir root. `walkRoot` is package-private but the test is in
> `package eqfind`, so it is reachable.

---

## Shared Patterns

### Test harness: httptest backend recorder (app package)
**Source:** `internal/app/runapp_test.go:216-249` (`ingestRecorder`, `fastBackend`)
**Apply to:** all new `makeOnSpellbookChange_*` tests
**Reuse, don't re-declare** — these live in the same `package app` test binary.
```go
ir := &ingestRecorder{status: http.StatusNoContent}
srv := httptest.NewServer(ir.handler())
defer srv.Close()
bc := fastBackend(t, srv)
// ... assert ir.requests()
```

### Test harness: temp LOCALAPPDATA for cfg.Save()
**Source:** `internal/app/migrate_test.go:18-23` (`withTempLOCALAPPDATA`)
**Apply to:** any test whose code path calls `cfg.Save()` (all spellbook 204 + the persist-vs-not assertions)
```go
p := withTempLOCALAPPDATA(t)
dir := filepath.Dir(filepath.Dir(p)) // the LOCALAPPDATA tmp dir; drop char files here
```

### Test harness: t.TempDir() sentinel-pair planting (eqfind package)
**Source:** `internal/eqfind/discover_test.go:13-25` (`makeFakeEQDir`)
**Apply to:** the new `walkRoot` test (extend to depth-aware `plantEQAt`)

### Build-tag discipline for platform-specific code
**Source:** `internal/eqfind/heuristic_windows.go:1` (`//go:build windows`) vs `heuristic_other.go:1` (`//go:build !windows`)
**Apply to:** the new eqfind test file — MUST be `//go:build windows`; it references `walkRoot` which exists only in the windows TU.

### Structured logging op-string greppability
**Source:** CLAUDE.md Conventions ("structured logging both Go side (slog) … keeps logs greppable")
**Apply to:** the `handleIngestErr` refactor — preserve noun-specific slog ops
(`"upload inventory"` / `"upload spellbook"`) by threading a `slogNoun` param,
rather than collapsing to a single `"upload"` op.

---

## No Analog Found

None. All three targets have a close in-repo analog (two are intra-file).

---

## Metadata

**Analog search scope:** `internal/app/`, `internal/eqfind/`, `internal/watch/`, `internal/config/`, `internal/backend/`
**Files scanned:** runapp.go, runapp_test.go, migrate_test.go, heuristic_windows.go, heuristic_other.go, discover.go, discover_test.go, watch/watcher.go, config/config.go, backend/client.go
**Pattern extraction date:** 2026-06-03
