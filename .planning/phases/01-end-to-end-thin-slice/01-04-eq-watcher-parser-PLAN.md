---
phase: 01-end-to-end-thin-slice
plan: 04
type: execute
wave: 2
depends_on: [01]
files_modified:
  - internal/eqfind/discover.go
  - internal/eqfind/discover_test.go
  - internal/parse/inventory.go
  - internal/parse/inventory_test.go
  - internal/parse/testdata/sample-inventory.txt
  - internal/parse/testdata/sample-inventory-with-cp1252.txt
  - internal/watch/debounce.go
  - internal/watch/debounce_test.go
  - internal/watch/watcher.go
autonomous: true
requirements: [INST-02, WATCH-01, WATCH-04]
must_haves:
  truths:
    - "eqfind.Discover() probes (in order) known paths, then registry uninstall keys (HKCU + HKLM + WOW6432Node), then a heuristic recursive scan, and returns the first folder containing both eqgame.exe and eqclient.ini"
    - "When all three discovery layers fail, eqfind.Discover() returns ErrNotFound (callers — Plan 07 wizard — surface the native folder picker per D-09)"
    - "watch.Run() registers the EQ folder via fsnotify.Add(parent), filters events by *-Inventory.txt suffix, debounces 500ms, and ALWAYS re-stats + re-reads the file fresh on timer-fire (NEVER trusts ev.Op or any payload field)"
    - "parse.Parse() decodes Windows-1252 bytes via charmap.Windows1252.NewDecoder() and parses TSV with FieldsPerRecord=-1 + LazyQuotes=true; tolerates extra trailing columns; skips header row when column 2 is non-numeric"
    - "An EQ-style sample file with 250 inventory rows + a CP1252 curly apostrophe in an item name parses cleanly into 250 5-column rows with the apostrophe preserved as a UTF-8 right single quotation mark"
  artifacts:
    - path: "internal/eqfind/discover.go"
      provides: "Discover() (string, error) following the four-step cascade per CONTEXT.md D-09 + RESEARCH.md §6.5"
      min_lines: 80
      contains: "eqgame.exe"
    - path: "internal/parse/inventory.go"
      provides: "Parse(io.Reader) ([][]string, error) tolerant TSV parser with Win-1252 decode"
      contains: "charmap.Windows1252"
    - path: "internal/watch/watcher.go"
      provides: "Run(ctx, eqFolder, onChange) blocking fsnotify loop with 500ms debounce"
      contains: "fsnotify"
    - path: "internal/watch/debounce.go"
      provides: "Per-path timer-reset Debouncer per RESEARCH.md §2.5"
      contains: "time.AfterFunc"
  key_links:
    - from: "internal/watch/watcher.go"
      to: "internal/parse/inventory.go"
      via: "watcher does NOT call parse directly — it just signals onChange(path); the caller (Plan 05/07 wiring) calls parse.Parse"
      pattern: "fsnotify\\.NewWatcher|watcher\\.Add"
    - from: "internal/watch/watcher.go"
      to: "github.com/fsnotify/fsnotify"
      via: "fsnotify.NewWatcher / w.Add(eqFolder) / w.Events / w.Errors"
      pattern: "fsnotify\\.New(Watcher|BufferedWatcher)"
    - from: "internal/parse/inventory.go"
      to: "golang.org/x/text/encoding/charmap"
      via: "charmap.Windows1252.NewDecoder().Reader(r)"
      pattern: "charmap\\.Windows1252"
---

<objective>
Build the local-side data pipeline of Phase 1: discover the EverQuest folder, watch it for
inventory file writes, decode Win-1252 TSV reliably. This plan is the entire I/O surface from
"user runs /outputfile inventory" up to (but not including) the Sheets API call — Plan 05 owns the
sheets side; Plan 07 wires watcher events to sheets writes.

Purpose: Three of Phase 1's twelve requirements (INST-02, WATCH-01, WATCH-04) are owned here.
Combined, they make the difference between "the watcher reliably notices file changes and decodes
them correctly" and "the watcher misses bursts, double-uploads, or mojibake-corrupts item names."
The CLAUDE.md rules around fsnotify behavior on Windows (always re-read fresh, never trust event
payload) and the WATCH-04 column tolerance (extra trailing columns, optional header row, Win-1252)
are load-bearing — every later phase depends on this pipeline being correct.

Output: Three Go packages (`eqfind`, `parse`, `watch`) with unit tests. Plan 07 will wire them
together with the sheets package from Plan 05 inside `runApp(ctx)`.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/phases/01-end-to-end-thin-slice/01-CONTEXT.md
@.planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md
@.planning/research/ARCHITECTURE.md
@./CLAUDE.md
@internal/config/config.go
@internal/logging/logger.go
</context>

<interfaces>
<!-- Contracts this plan exports for downstream plans (07). -->

From internal/eqfind/discover.go:
```go
package eqfind

import "errors"

var ErrNotFound = errors.New("eqfind: no EQ folder found in known paths, registry, or heuristic scan")

// Discover runs the cascade: known paths → registry uninstall keys → heuristic scan.
// Returns the first folder containing both eqgame.exe AND eqclient.ini per D-10.
// Returns ErrNotFound if all three layers fail — caller (Plan 07) shows native folder picker.
func Discover() (folder string, err error)

// ValidateFolder confirms `dir` contains both eqgame.exe and eqclient.ini.
// Used by Plan 07 to validate user-picked folders per D-10 ("This folder doesn't look like an EQ install").
func ValidateFolder(dir string) error
```

From internal/parse/inventory.go:
```go
package parse

// Parse reads a <Char>-Inventory.txt file (Win-1252, tab-separated, optional header).
// Returns rows of EXACTLY 5 columns each: [Location, Name, ID, Count, Slots].
// Rows with fewer than 5 columns are skipped; rows with more than 5 are truncated to 5.
// Header row (column 2 non-numeric) is detected and dropped.
// Returns an error only on encoding-decoder or csv-reader hard failures.
// Returns (nil, nil) for an empty file (caller should treat as a no-op write).
func Parse(r io.Reader) (rows [][]string, err error)
```

From internal/watch/debounce.go:
```go
package watch

type Debouncer struct { /* unexported */ }

func NewDebouncer(delay time.Duration) (*Debouncer, <-chan string)

// Trigger resets the per-path timer. After `delay` of quiet, the path is sent on the
// channel returned by NewDebouncer. Safe for concurrent calls.
func (d *Debouncer) Trigger(path string)

// Stop cancels all in-flight timers. Subsequent Trigger calls are dropped.
func (d *Debouncer) Stop()
```

From internal/watch/watcher.go:
```go
package watch

// OnChange is called by Run when a debounced quiet period elapses for `path`.
// The caller MUST re-stat and re-read `path` fresh — Run never reads file contents.
// Per CLAUDE.md: never trust fsnotify event payload data on Windows.
type OnChange func(path string)

// Run blocks. It watches `eqFolder` for *-Inventory.txt events, debounces 500ms per path,
// and dispatches OnChange after each quiet period. Returns when ctx is cancelled
// or fsnotify channels close.
func Run(ctx context.Context, eqFolder string, onChange OnChange) error
```
</interfaces>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Win-1252 tolerant TSV parser with sample-driven tests</name>
  <files>internal/parse/inventory.go, internal/parse/inventory_test.go, internal/parse/testdata/sample-inventory.txt, internal/parse/testdata/sample-inventory-with-cp1252.txt</files>
  <read_first>
    - .planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md (§9 entire — file format §9.1, encoding §9.2 with full Parse code lines 996-1020, validation rules §9.3, what's NOT in file §9.4; §3 "Don't Hand-Roll" rows for TSV parsing and Win-1252 decode)
    - .planning/research/ARCHITECTURE.md (Landing Tabs §inv:&lt;CharName&gt; — confirms 5 columns Location/Name/ID/Count/Slots, ID=0 for empty slots, P99 max ~250 rows)
    - ./CLAUDE.md ("Never use" list: trusting fsnotify payloads — relevant to the watcher task; here we're parsing files which is independent)
  </read_first>
  <behavior>
    - Test 1: Parse on an empty `io.Reader` returns (nil, nil) — no error.
    - Test 2: Parse on a single header row only (`Location\tName\tID\tCount\tSlots\n`) returns 0 rows + nil err.
    - Test 3: Parse on header + 3 data rows with exactly 5 columns returns 3 rows; the header is dropped because column 2 ("Name") is non-numeric.
    - Test 4: Parse on data with NO header row (first row's column 2 is `1234` — numeric) returns ALL rows including the first.
    - Test 5: Parse on a row with 7 columns truncates to first 5.
    - Test 6: Parse on a row with 4 columns SKIPS that row (does not panic, does not pad).
    - Test 7: Parse on a row containing a CP1252 curly apostrophe byte 0x92 in the Name field returns the row with the Name decoded to UTF-8 right single quotation mark (`’` = U+2019, encoded as 3 UTF-8 bytes `0xE2 0x80 0x99`). Cite Pitfall in RESEARCH.md §9.2.
    - Test 8: Parse on testdata/sample-inventory.txt (a 250-row generated sample) returns exactly 250 rows.
    - Test 9: Parse with LazyQuotes=true accepts a Name containing an unescaped apostrophe (e.g., `Tashan's Lance`).
  </behavior>
  <action>
    Create `internal/parse/inventory.go` containing the EXACT code from RESEARCH.md §9.2 lines
    996-1020:
    ```go
    package parse

    import (
        "encoding/csv"
        "io"
        "strconv"

        "golang.org/x/text/encoding/charmap"
    )

    // Parse reads a <Char>-Inventory.txt file (Win-1252, tab-separated, optional header).
    // Returns rows of EXACTLY 5 columns each: [Location, Name, ID, Count, Slots].
    // Per WATCH-04: tolerate extra trailing columns, decode Windows-1252, accept header row.
    // Per RESEARCH.md §9.3: rows with non-int IDs are silently skipped.
    func Parse(r io.Reader) (rows [][]string, err error) {
        decoded := charmap.Windows1252.NewDecoder().Reader(r)
        cr := csv.NewReader(decoded)
        cr.Comma = '\t'
        cr.FieldsPerRecord = -1   // tolerate any column count
        cr.LazyQuotes = true      // EQ names may contain stray apostrophes
        all, err := cr.ReadAll()
        if err != nil {
            return nil, err
        }
        if len(all) == 0 {
            return nil, nil
        }
        // Drop header IF the first row's column 2 (ID position) is NOT numeric.
        if !isIntField(all[0], 2) {
            all = all[1:]
        }
        out := make([][]string, 0, len(all))
        for _, row := range all {
            if len(row) < 5 {
                continue
            }
            // ID must parse as int; skip rows where it doesn't.
            if !isIntField(row, 2) {
                continue
            }
            out = append(out, row[:5])
        }
        return out, nil
    }

    // isIntField reports whether row[col] is a base-10 integer (or "0").
    // col is 0-indexed; "column 2" in our schema is the ID at index 2.
    func isIntField(row []string, col int) bool {
        if col >= len(row) {
            return false
        }
        _, err := strconv.Atoi(row[col])
        return err == nil
    }
    ```

    Create `internal/parse/testdata/sample-inventory.txt` — a generated 250-row TSV file with a
    realistic header row. Use `\t` separators, `\n` line endings, all-ASCII content. Schema:
    ```
    Location\tName\tID\tCount\tSlots\n
    General1\tCloth Cap\t1001\t1\t0\n
    General2\tCloth Sandals\t1002\t1\t0\n
    ...
    ```
    Generate row N as `(General1..General10|Bank1..Bank8|Charm|Head|...)\t(SampleItem-N)\t(2000+N)\t(1)\t(0)`
    so the file has 250 data rows + 1 header = 251 lines. Vary location and counts naturally.
    Use a Python or shell one-liner to generate; commit the result.

    Create `internal/parse/testdata/sample-inventory-with-cp1252.txt` — a small (5-row) TSV file
    encoded in CP1252 (NOT UTF-8 — write raw bytes). One Name field MUST contain byte 0x92 (the
    CP1252 right single quotation mark / curly apostrophe). The remaining rows are pure ASCII.
    Verify by hexdump: `xxd internal/parse/testdata/sample-inventory-with-cp1252.txt` should show
    a `92` byte in the second column of one row.

    Create `internal/parse/inventory_test.go` with the 9 tests in &lt;behavior&gt; above. Use
    `embed.FS` or `os.Open` to load testdata. For Test 7 specifically, after parsing, assert that
    the affected row's Name field equals the Go string `"Brell’s Trinket"` (or similar) —
    NOT a string starting with byte 0x92.

    Add a comment at the top of inventory.go citing RESEARCH.md Open Question Q4: "Encoding lock-in:
    if a real EQ-produced sample later proves UTF-8 instead of CP1252, swap charmap.Windows1252
    with utf8 — see Q4 in 01-RESEARCH.md §11."

    Run `go test ./internal/parse/... -count=1 -v` and confirm all 9 tests pass.
  </action>
  <verify>
    <automated>go test ./internal/parse/... -count=1 -timeout 30s &amp;&amp; grep -nE "charmap\.Windows1252" internal/parse/inventory.go &amp;&amp; grep -nE "FieldsPerRecord\s*=\s*-1" internal/parse/inventory.go &amp;&amp; grep -nE "LazyQuotes\s*=\s*true" internal/parse/inventory.go &amp;&amp; test -s internal/parse/testdata/sample-inventory.txt &amp;&amp; test -s internal/parse/testdata/sample-inventory-with-cp1252.txt &amp;&amp; xxd internal/parse/testdata/sample-inventory-with-cp1252.txt | grep -q '92'</automated>
  </verify>
  <acceptance_criteria>
    - `internal/parse/inventory.go` exports `Parse(io.Reader) ([][]string, error)`
    - `inventory.go` contains literal `charmap.Windows1252`
    - `inventory.go` contains literal `FieldsPerRecord = -1`
    - `inventory.go` contains literal `LazyQuotes = true`
    - `inventory.go` contains literal `Comma = '\t'`
    - `internal/parse/inventory_test.go` contains at least 9 `func Test` declarations
    - `go test ./internal/parse/... -count=1` exits 0
    - `internal/parse/testdata/sample-inventory.txt` has at least 251 lines (`wc -l`)
    - `internal/parse/testdata/sample-inventory-with-cp1252.txt` contains the byte 0x92 (`xxd | grep '92'`)
    - `go vet ./internal/parse/...` exits 0
  </acceptance_criteria>
  <done>
    Parser is sample-test verified. Win-1252 decode round-trips at least one curly apostrophe.
    A 250-row file parses into 250 5-column rows. Tolerance for header / no-header / extra
    columns / short rows is exercised.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Per-path timer-reset Debouncer + fsnotify watcher loop</name>
  <files>internal/watch/debounce.go, internal/watch/debounce_test.go, internal/watch/watcher.go</files>
  <read_first>
    - .planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md (§2.5 Pattern 3 Per-Path Timer-Reset Debouncer lines 391-415 with full Debouncer code; §8 entire — §8.1 EQ /outputfile semantics, §8.2 debounce loop lines 924-955 with full Run code, §8.3 always re-read rule, §8.4 OneDrive note)
    - ./CLAUDE.md ("Never use" rules: trusting fsnotify event payloads on Windows; polling with time.Tick)
    - .planning/phases/01-end-to-end-thin-slice/01-CONTEXT.md (D-11 single-folder Phase 1)
  </read_first>
  <behavior>
    - Debouncer Test 1: NewDebouncer(50ms); Trigger("a") once; channel emits "a" after ~50ms (assert within 40-200ms window).
    - Debouncer Test 2: NewDebouncer(50ms); Trigger("a") 5 times rapidly (10ms apart); channel emits "a" exactly ONCE after the last trigger.
    - Debouncer Test 3: NewDebouncer(50ms); Trigger("a") and Trigger("b") in parallel; channel emits both, independently.
    - Debouncer Test 4: After Stop(), subsequent Trigger calls do NOT cause emissions.
    - Watcher Test 1: Run(ctx, tempDir, onChange) — write a `Foo-Inventory.txt` file with one byte → onChange fires within 800ms with the absolute path. (Use t.TempDir + os.WriteFile.)
    - Watcher Test 2: Write a `Foo-Spellbook.txt` (NOT inventory) → onChange does NOT fire (filtered by suffix). Wait 1s; assert callback not called.
    - Watcher Test 3: Write `Foo-Inventory.txt` 5 times in 100ms → onChange fires exactly ONCE.
    - Watcher Test 4: Cancel ctx → Run returns ctx.Err().
  </behavior>
  <action>
    Create `internal/watch/debounce.go` per RESEARCH.md §2.5 lines 395-414, adapted to expose
    a channel and Stop():
    ```go
    package watch

    import (
        "sync"
        "time"
    )

    type Debouncer struct {
        delay  time.Duration
        timers sync.Map // path -> *time.Timer
        out    chan string
        stopped chan struct{}
        once   sync.Once
    }

    // NewDebouncer returns a Debouncer and the read-end of its channel.
    // Buffer size 16 absorbs bursts without blocking the producer.
    func NewDebouncer(delay time.Duration) (*Debouncer, <-chan string) {
        out := make(chan string, 16)
        d := &Debouncer{delay: delay, out: out, stopped: make(chan struct{})}
        return d, out
    }

    func (d *Debouncer) Trigger(path string) {
        select {
        case <-d.stopped:
            return
        default:
        }
        if t, ok := d.timers.Load(path); ok {
            t.(*time.Timer).Reset(d.delay)
            return
        }
        t := time.AfterFunc(d.delay, func() {
            d.timers.Delete(path)
            select {
            case d.out <- path:
            case <-d.stopped:
            }
        })
        d.timers.Store(path, t)
    }

    func (d *Debouncer) Stop() {
        d.once.Do(func() {
            close(d.stopped)
            d.timers.Range(func(_, v any) bool {
                v.(*time.Timer).Stop()
                return true
            })
        })
    }
    ```

    Create `internal/watch/debounce_test.go` with the four Debouncer tests.

    Create `internal/watch/watcher.go` per RESEARCH.md §8.2 lines 925-955, adapted to the
    OnChange signature:
    ```go
    package watch

    import (
        "context"
        "errors"
        "log/slog"
        "path/filepath"
        "strings"
        "time"

        "github.com/fsnotify/fsnotify"
    )

    type OnChange func(path string)

    func Run(ctx context.Context, eqFolder string, onChange OnChange) error {
        w, err := fsnotify.NewWatcher()
        if err != nil { return err }
        defer w.Close()

        // Per CLAUDE.md / Pitfall #4: watch the parent directory, never individual files.
        if err := w.Add(eqFolder); err != nil { return err }

        deb, out := NewDebouncer(500 * time.Millisecond)
        defer deb.Stop()

        slog.Info("watcher started", "folder", eqFolder)

        for {
            select {
            case <-ctx.Done():
                return ctx.Err()
            case ev, ok := <-w.Events:
                if !ok { return errors.New("fsnotify Events channel closed") }
                base := filepath.Base(ev.Name)
                // Phase 1: ONLY *-Inventory.txt. WATCH-02 (spellbook) is Phase 2.
                if !strings.HasSuffix(base, "-Inventory.txt") { continue }
                // Drop pure Chmod (Windows: rare but possible).
                if ev.Op == fsnotify.Chmod { continue }
                // Per CLAUDE.md / RESEARCH.md §8.3: NEVER trust ev.Op or any payload.
                // Trigger the debouncer; on quiet, the receiver re-stats + re-reads.
                deb.Trigger(ev.Name)
            case e, ok := <-w.Errors:
                if !ok { return errors.New("fsnotify Errors channel closed") }
                slog.Warn("fsnotify error", "err", e)
            case path := <-out:
                // 500ms quiet — caller must re-stat and read fresh.
                slog.Info("watcher debounced", "path", filepath.Base(path))
                onChange(path)
            }
        }
    }
    ```

    Create or extend `internal/watch/watcher_test.go` with the four Watcher tests. Tests 1-3 use
    `t.TempDir()` as the EQ folder, run `Run` in a goroutine with `context.WithCancel`, exercise
    file writes, and use a `chan string` to capture onChange invocations. Test 4 cancels and
    asserts the goroutine exits.

    NOTE: t.TempDir on Windows may have antivirus interference — set per-test timeout to 5s and
    document any flaky-on-CI tendencies in test comments. If a test proves flaky on Windows CI,
    skip it on `runtime.GOOS == "windows"` with a TODO citing fsnotify Issue #214 (AV-induced
    spurious events).

    Add a comment in watcher.go header citing CLAUDE.md "trusting fsnotify event payloads on
    Windows" prohibition explicitly:
    ```go
    // SECURITY/CORRECTNESS: This watcher follows the CLAUDE.md / RESEARCH.md §8.3 rule:
    // it filters events purely by filename suffix and triggers a debouncer; the timer-fire
    // dispatches a path to OnChange. The OnChange callback (Plan 07 wiring) re-stats and
    // re-reads the file fresh. We NEVER read ev.Op for ordering, ev.Name for content, or
    // any other event payload field beyond the path. Spurious AV events become idempotent
    // re-uploads — negligible cost.
    ```
  </action>
  <verify>
    <automated>go test ./internal/watch/... -count=1 -timeout 60s &amp;&amp; grep -nE "fsnotify\.NewWatcher" internal/watch/watcher.go &amp;&amp; grep -nE "500 \* time\.Millisecond|500\*time\.Millisecond" internal/watch/watcher.go &amp;&amp; grep -nE "-Inventory\.txt" internal/watch/watcher.go &amp;&amp; grep -nE "time\.AfterFunc" internal/watch/debounce.go &amp;&amp; ! grep -vE "^\s*//" internal/watch/watcher.go | grep -E "ev\.Size|ev\.Mtime|ev\.ModTime"</automated>
  </verify>
  <acceptance_criteria>
    - `internal/watch/debounce.go` exports `NewDebouncer`, `Trigger`, `Stop`
    - `debounce.go` contains literal `time.AfterFunc`
    - `debounce.go` contains literal `sync.Map`
    - `internal/watch/watcher.go` exports `Run` with the OnChange signature
    - `watcher.go` contains literal `fsnotify.NewWatcher`
    - `watcher.go` contains the 500ms debounce duration: `500 * time.Millisecond` or `500*time.Millisecond`
    - `watcher.go` filters by suffix `-Inventory.txt`
    - `watcher.go` does NOT reference `.Size` / `.Mtime` / `.ModTime` of fsnotify events (CLAUDE.md compliance — these don't exist on fsnotify.Event but the grep is defensive)
    - `go test ./internal/watch/... -count=1 -timeout 60s` exits 0
    - `go vet ./internal/watch/...` exits 0
  </acceptance_criteria>
  <done>
    Debouncer + fsnotify watcher are wired correctly. Watcher emits exactly one onChange per
    burst of inventory file writes. Spellbook files are correctly ignored (Phase 1 scope). The
    "always re-read fresh" discipline is encoded in code comments and downstream contract.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: EQ folder discovery cascade (known paths → registry → heuristic scan)</name>
  <files>internal/eqfind/discover.go, internal/eqfind/discover_test.go</files>
  <read_first>
    - .planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md (§6.5 EQ folder discovery lines 851-892, including the discover.go skeleton and the registry uninstall key list with HKCU + HKLM + WOW6432Node paths; §6.6 native folder picker via sqweek/dialog lines 894-901)
    - .planning/phases/01-end-to-end-thin-slice/01-CONTEXT.md (D-09 cascade order; D-10 eqgame.exe validation; D-11 Phase 1 single-folder)
    - ./CLAUDE.md (no specific rule, but the project description notes EQ folder is the user's local installation)
  </read_first>
  <behavior>
    - Test 1: ValidateFolder on a tempDir containing both `eqgame.exe` and `eqclient.ini` returns nil.
    - Test 2: ValidateFolder on a tempDir missing eqgame.exe returns a non-nil error mentioning "eqgame.exe".
    - Test 3: ValidateFolder on a tempDir missing eqclient.ini returns a non-nil error mentioning "eqclient.ini".
    - Test 4: ValidateFolder on a non-existent dir returns a non-nil error.
    - Test 5: Discover with the known-paths layer overridden (via injected probe function) to "find" a stub returns that stub's path.
    - Test 6: Discover with all three layers returning empty/error returns ErrNotFound.
  </behavior>
  <action>
    Create `internal/eqfind/discover.go` per RESEARCH.md §6.5 lines 856-883, refactored for
    testability with injectable probe functions (otherwise unit tests can't exercise the cascade
    without a real registry):

    ```go
    package eqfind

    import (
        "errors"
        "fmt"
        "os"
        "path/filepath"
        "runtime"
    )

    var ErrNotFound = errors.New("eqfind: no EQ folder found in known paths, registry, or heuristic scan")

    // For tests: each layer is a func var so tests can swap implementations.
    var (
        knownPathsProbe = defaultKnownPaths
        registryProbe   = defaultRegistryProbe
        heuristicProbe  = defaultHeuristicScan
    )

    func Discover() (string, error) {
        if p := knownPathsProbe(); p != "" {
            return p, nil
        }
        if p := registryProbe(); p != "" {
            return p, nil
        }
        if p := heuristicProbe(); p != "" {
            return p, nil
        }
        return "", ErrNotFound
    }

    // ValidateFolder enforces D-10: folder must contain BOTH eqgame.exe AND eqclient.ini.
    func ValidateFolder(dir string) error {
        if dir == "" {
            return errors.New("eqfind: empty path")
        }
        for _, fname := range []string{"eqgame.exe", "eqclient.ini"} {
            p := filepath.Join(dir, fname)
            if _, err := os.Stat(p); err != nil {
                return fmt.Errorf("eqfind: %s missing in %q (%w)", fname, dir, err)
            }
        }
        return nil
    }

    func defaultKnownPaths() string {
        candidates := []string{
            `C:\P99`, `C:\Project1999`, `C:\Games\Project1999`,
            `C:\Program Files (x86)\Sony\EverQuest`,
            filepath.Join(os.Getenv("USERPROFILE"), "EverQuest"),
            filepath.Join(os.Getenv("USERPROFILE"), "P99"),
        }
        for _, p := range candidates {
            if ValidateFolder(p) == nil {
                return p
            }
        }
        return ""
    }

    func defaultRegistryProbe() string {
        if runtime.GOOS != "windows" {
            return ""
        }
        return scanUninstallKeys()
    }

    func defaultHeuristicScan() string {
        if runtime.GOOS != "windows" {
            return ""
        }
        return heuristicScan()
    }
    ```

    Create a separate `internal/eqfind/registry_windows.go` (build tag `//go:build windows`)
    that uses `golang.org/x/sys/windows/registry` (stdlib-x — already in the indirect dep tree
    via Go 1.24) to enumerate:
    - `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall`
    - `HKLM\Software\Microsoft\Windows\CurrentVersion\Uninstall`
    - `HKLM\Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`

    For each subkey, read `DisplayName` and `InstallLocation`. If `DisplayName` matches regex
    `(?i)^(Project 1999|EverQuest|Sony EverQuest)$` AND `InstallLocation` exists AND
    ValidateFolder(InstallLocation) returns nil → return that location. First match wins.

    Add `golang.org/x/sys/windows/registry` to go.mod via `go get golang.org/x/sys/windows/registry`
    if not already present.

    Create `internal/eqfind/registry_other.go` (build tag `//go:build !windows`) with a stub:
    ```go
    package eqfind
    func scanUninstallKeys() string { return "" }
    ```

    Create `internal/eqfind/heuristic_windows.go` (build tag `//go:build windows`) implementing
    the §6.5 heuristic scan: walk root drives `C:`, `D:`, `E:` with depth ≤ 5, prune `Windows`,
    `Program Files`, `Program Files (x86)`, `$Recycle.Bin`, `node_modules`, `AppData`. For each
    visited dir, ValidateFolder; on first match return. Use `filepath.WalkDir` with a custom
    fs.WalkDirFunc that returns `filepath.SkipDir` on pruned entries. Bound execution to 30
    seconds total via context — this MUST NOT hang the wizard if the user has many drives.

    Create `internal/eqfind/heuristic_other.go` stub for non-Windows (`func heuristicScan() string { return "" }`).

    Create `internal/eqfind/discover_test.go` exercising the six behaviors. Test 5 swaps the
    package-level `knownPathsProbe` var:
    ```go
    func TestDiscover_KnownPathsLayerHit(t *testing.T) {
        dir := t.TempDir()
        os.WriteFile(filepath.Join(dir, "eqgame.exe"), []byte{0}, 0644)
        os.WriteFile(filepath.Join(dir, "eqclient.ini"), []byte{0}, 0644)

        orig := knownPathsProbe
        knownPathsProbe = func() string { return dir }
        t.Cleanup(func() { knownPathsProbe = orig })

        got, err := Discover()
        if err != nil { t.Fatal(err) }
        if got != dir { t.Errorf("got %q want %q", got, dir) }
    }
    ```

    Document in a comment that the heuristic_windows.go scan is not unit-tested in CI because it
    requires a real Windows filesystem with realistic structure. It is exercised in Plan 08's
    smoke checkpoint.
  </action>
  <verify>
    <automated>go test ./internal/eqfind/... -count=1 -timeout 30s &amp;&amp; grep -nE "ValidateFolder" internal/eqfind/discover.go &amp;&amp; grep -nE "eqgame\.exe" internal/eqfind/discover.go &amp;&amp; grep -nE "eqclient\.ini" internal/eqfind/discover.go &amp;&amp; grep -nE "ErrNotFound" internal/eqfind/discover.go &amp;&amp; (grep -nE "scanUninstallKeys|registry\.OpenKey" internal/eqfind/registry_windows.go || echo "registry_windows.go must contain scanUninstallKeys") &amp;&amp; go vet ./internal/eqfind/...</automated>
  </verify>
  <acceptance_criteria>
    - `internal/eqfind/discover.go` exports `Discover`, `ValidateFolder`, `ErrNotFound`
    - `discover.go` contains literal strings `eqgame.exe` AND `eqclient.ini`
    - `discover.go` known-paths list contains all of: `C:\P99`, `C:\Project1999`, `C:\Games\Project1999`
    - `internal/eqfind/registry_windows.go` exists with `//go:build windows` build tag at top
    - `registry_windows.go` references all three uninstall hive paths: `HKCU`, `HKLM`, `WOW6432Node` (literal substrings or `registry.LOCAL_MACHINE` enum constants)
    - `internal/eqfind/heuristic_windows.go` exists with `//go:build windows` build tag
    - `internal/eqfind/discover_test.go` contains at least 6 `func Test` declarations
    - `go test ./internal/eqfind/... -count=1` exits 0
    - `go vet ./internal/eqfind/...` exits 0
    - `go build ./...` exits 0 on Linux (registry+heuristic stubs handle non-Windows)
  </acceptance_criteria>
  <done>
    EQ folder discovery cascade is implemented per D-09, D-10, D-11. ValidateFolder enforces both
    eqgame.exe + eqclient.ini per D-10. Tests cover the cascade-routing logic with injected probes;
    real registry/heuristic scans run only on Windows.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| filesystem ↔ parser | Untrusted file content; malformed TSV could DoS the parser |
| Windows registry ↔ eqfind | Registry strings are trusted-but-validate (path traversal risk) |
| filesystem ↔ heuristic scan | Walking arbitrary drives must not exhaust resources or follow symlinks into loops |
| fsnotify event payload ↔ Go process | Event Op/Name fields are unreliable on Windows; trust ONLY the path |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-04-01 | Denial of Service | Malformed inventory file with billions of empty rows exhausts memory in csv.ReadAll | mitigate | Add `cr.LazyQuotes = true` and rely on csv.Reader's bounded-by-input semantics; Plan 05 caller adds an upper-bound size check (file ≤ 1 MB warning per RESEARCH.md §9.3); plan 04 parser does not pre-allocate beyond input size |
| T-04-02 | Tampering | Path traversal via registry-supplied InstallLocation (e.g., `C:\..\..\Windows\System32`) | mitigate | ValidateFolder requires both eqgame.exe AND eqclient.ini exist in the named directory — neither file exists in System32; per RESEARCH.md §11 "Path traversal in EQ folder discovery" pattern |
| T-04-03 | Information Disclosure | Heuristic scan walks and indexes user's entire C: drive (privacy / log spam) | mitigate | Bounded depth (5 levels), pruned standard exclusions (Windows, Program Files, AppData, $Recycle.Bin, node_modules), 30-second total timeout, no logging of visited paths beyond DEBUG level (Phase 1 default is INFO) |
| T-04-04 | Tampering | Symbolic link loop in EQ folder area causes filepath.WalkDir to spin forever | mitigate | filepath.WalkDir does not follow symlinks by default; explicit `os.Lstat` + `info.IsDir()` check; combined with 30s timeout |
| T-04-05 | Spoofing | Attacker plants an `eqgame.exe`+`eqclient.ini` pair in a writable directory to redirect watcher target | accept | Phase 1 watcher is local-only; user controls their own filesystem; if attacker has write access to user's drive, watcher targeting is the least of the user's problems. ValidateFolder does NOT check eqgame.exe signature/hash; documenting as accepted risk |
| T-04-06 | Tampering | fsnotify event payload Op/Size used to make trust decisions | mitigate | watcher.go ignores all fields except `ev.Name`; comment + acceptance grep for `ev.Size`/`ev.Mtime`/`ev.ModTime` references (none allowed) |
| T-04-07 | Information Disclosure | parser logs raw inventory content to slog | mitigate | parser.Parse never calls slog; Plan 05 caller logs only row count + char name (RESEARCH.md §10.2 logging policy: no raw content) |
| T-04-08 | Denial of Service | Inventory file written mid-stream → parser reads truncated bytes mid-row | mitigate | 500ms debounce comfortably exceeds EQ flush window (~50ms per RESEARCH.md §8.1 / Pitfall #10); on truncation parser skips the short row (Test 6) instead of crashing |
| T-04-09 | Tampering | Mojibake on non-CP1252 file silently produces wrong item names | mitigate | RESEARCH.md Open Question Q4 explicitly flagged for Plan 04 verification with a real EQ-produced sample; if Q4 reveals UTF-8 instead of CP1252, swap charmap.Windows1252 for utf8.NewDecoder |
</threat_model>

<verification>
- `go build ./...` exits 0 on Linux (Windows-only files compile under build tags)
- `go vet ./...` exits 0
- `go test ./internal/parse/... ./internal/eqfind/... ./internal/watch/... -count=1 -timeout 60s` exits 0
- `internal/parse/testdata/sample-inventory.txt` parses to exactly 250 rows
- `internal/parse/testdata/sample-inventory-with-cp1252.txt` parses with the curly apostrophe correctly decoded to UTF-8 `’`
- Watcher test confirms one onChange per burst of 5 writes within 100ms
- Watcher test confirms NO onChange for `*-Spellbook.txt` writes (Phase 1 scope discipline)
- `grep -rE "time\\.Tick" --include="*.go" internal/` returns 0 matches (CLAUDE.md polling prohibition)
- `grep -rE "ev\\.(Size|Mtime|ModTime)" --include="*.go" internal/watch/` returns 0 matches (no event-payload trust)
</verification>

<success_criteria>
- INST-02 satisfied: Discover() runs the cascade and returns ErrNotFound only after all four layers fail; Plan 07 wizard step-3 catches ErrNotFound and shows the native folder picker
- WATCH-01 satisfied: fsnotify watch on the EQ folder, *-Inventory.txt suffix filter, 500ms per-path debounce, never trusts event payload
- WATCH-04 satisfied: parser is Win-1252 + tolerant of header/no-header + tolerant of extra columns + 5-column truncation; tested against a 250-row sample
- All three packages are unit-tested where automatable; cross-platform builds work via build tags
- No package logs raw file content or trusts fsnotify event payloads
</success_criteria>

<output>
After completion, create `.planning/phases/01-end-to-end-thin-slice/01-04-SUMMARY.md` documenting:
- The actual encoding observed in the dev's real EQ-produced sample (resolves Open Question Q4 — CP1252 or UTF-8)
- The exact debounce duration chosen (500ms confirmed; otherwise document deviation)
- Any registry uninstall key paths that turned out NOT to match real P99 installs (and what was used instead)
- Heuristic scan timeout chosen (30s default) and any drives skipped
- Watcher test flakiness observed (if any) and whether tests are skipped on Windows CI
</output>
