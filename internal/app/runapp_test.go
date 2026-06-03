package app

import (
	"context"
	"io"
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

func TestExtractCharName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		// Plain ASCII char names — the common case.
		{`C:\P99\Foo-Inventory.txt`, "Foo"},
		{`/tmp/eq/Bar-Inventory.txt`, "Bar"},
		// Spaces in char names (rare on P99 but multi-word names exist).
		{`C:\P99\Cool Toon-Inventory.txt`, "Cool Toon"},
		// Unicode-ish character (sanity-check the regex doesn't choke).
		{`C:\P99\Mörk-Inventory.txt`, "Mörk"},
		// Non-inventory file → ""
		{`C:\P99\Foo-Spellbook.txt`, ""},
		// Path doesn't end in -Inventory.txt → ""
		{`C:\P99\Foo-Inventory.bak`, ""},
		// Empty.
		{``, ""},
		// Just the suffix → "" (regex requires at least one char in the prefix group)
		{`-Inventory.txt`, ""},
	}
	for _, tc := range cases {
		got := extractCharName(tc.in)
		if got != tc.want {
			t.Errorf("extractCharName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// Plan 02-02 Task 5: extractCharNameForSuffix must distinguish inventory
// from spellbook files and reject anything that doesn't match.
func TestExtractCharNameForSuffix(t *testing.T) {
	cases := []struct {
		path   string
		suffix string
		want   string
	}{
		{`C:\P99\Slampeach-Inventory.txt`, watch.InventorySuffix, "Slampeach"},
		{`C:\P99\Slampeach-Spellbook.txt`, watch.SpellbookSuffix, "Slampeach"},
		{`C:\P99\Slampeach-Spellbook.txt`, watch.InventorySuffix, ""},
		{`C:\P99\Slampeach-Inventory.txt`, watch.SpellbookSuffix, ""},
		{`/tmp/eq/Cool Toon-Spellbook.txt`, watch.SpellbookSuffix, "Cool Toon"},
		{`-Inventory.txt`, watch.InventorySuffix, ""},
		{``, watch.InventorySuffix, ""},
	}
	for _, tc := range cases {
		got := extractCharNameForSuffix(tc.path, tc.suffix)
		if got != tc.want {
			t.Errorf("extractCharNameForSuffix(%q, %q) = %q, want %q",
				tc.path, tc.suffix, got, tc.want)
		}
	}
}

// Plan 02-02 Task 5 / WATCH-09: rescanCatchUp walks every folder, fires the
// inventory or spellbook callback for each file whose mtime is newer than
// the cached LastKnown*Mtime entry, and skips files that already match the
// cached mtime.
func TestRescanCatchUp_FiresOnNewerFiles(t *testing.T) {
	dir := t.TempDir()

	// Stale character: mtime cached, should NOT fire.
	stalePath := filepath.Join(dir, "Stale-Inventory.txt")
	if err := os.WriteFile(stalePath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	staleInfo, err := os.Stat(stalePath)
	if err != nil {
		t.Fatal(err)
	}
	staleMtime := staleInfo.ModTime().UTC().Format(time.RFC3339)

	// Fresh character: no entry in mtime map → MUST fire.
	freshInvPath := filepath.Join(dir, "Fresh-Inventory.txt")
	if err := os.WriteFile(freshInvPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Spellbook with no cache entry → MUST fire onSpellbook.
	freshSpbPath := filepath.Join(dir, "Fresh-Spellbook.txt")
	if err := os.WriteFile(freshSpbPath, []byte("9\tLifetap\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Unrelated file → MUST NOT fire either callback.
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		EQFolders: []string{dir},
		LastKnownInventoryMtime: map[string]string{
			"Stale": staleMtime,
		},
		LastKnownSpellbookMtime: map[string]string{},
	}

	// rescanCatchUp invokes callbacks synchronously on the calling goroutine
	// (no goroutines spawned), so plain slices/counters are correct here.
	var invPaths, spbPaths []string
	onInv := func(p string) { invPaths = append(invPaths, p) }
	onSpb := func(p string) { spbPaths = append(spbPaths, p) }

	rescanCatchUp(context.Background(), cfg, cfg.EQFolders, onInv, onSpb)

	if got := len(invPaths); got != 1 {
		t.Errorf("inventory callback fired %d times; want 1 (Fresh only)", got)
	}
	if got := len(spbPaths); got != 1 {
		t.Errorf("spellbook callback fired %d times; want 1 (Fresh only)", got)
	}
	sort.Strings(invPaths)
	sort.Strings(spbPaths)
	if len(invPaths) != 1 || invPaths[0] != freshInvPath {
		t.Errorf("inventory paths = %v; want [%s]", invPaths, freshInvPath)
	}
	if len(spbPaths) != 1 || spbPaths[0] != freshSpbPath {
		t.Errorf("spellbook paths = %v; want [%s]", spbPaths, freshSpbPath)
	}
}

// Plan 02-02 Task 5: rescanCatchUp must span every folder in cfg.EQFolders.
func TestRescanCatchUp_MultiFolderScan(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	pathA := filepath.Join(dirA, "Alpha-Inventory.txt")
	pathB := filepath.Join(dirB, "Beta-Spellbook.txt")
	if err := os.WriteFile(pathA, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pathB, []byte("9\tLifetap\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		EQFolders:               []string{dirA, dirB},
		LastKnownInventoryMtime: map[string]string{},
		LastKnownSpellbookMtime: map[string]string{},
	}

	var seenInv, seenSpb string
	onInv := func(p string) { seenInv = p }
	onSpb := func(p string) { seenSpb = p }

	rescanCatchUp(context.Background(), cfg, cfg.EQFolders, onInv, onSpb)

	if seenInv != pathA {
		t.Errorf("inventory callback path = %q; want %q", seenInv, pathA)
	}
	if seenSpb != pathB {
		t.Errorf("spellbook callback path = %q; want %q", seenSpb, pathB)
	}
}

// Plan 02-02 Task 5: rescanCatchUp must tolerate a non-existent folder
// without blowing up the rest of the scan.
func TestRescanCatchUp_MissingFolderIsSkipped(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "does-not-exist")

	good := filepath.Join(dir, "Good-Inventory.txt")
	if err := os.WriteFile(good, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		EQFolders:               []string{missing, dir},
		LastKnownInventoryMtime: map[string]string{},
		LastKnownSpellbookMtime: map[string]string{},
	}

	fired := 0
	onInv := func(string) { fired++ }
	onSpb := func(string) {}

	rescanCatchUp(context.Background(), cfg, cfg.EQFolders, onInv, onSpb)

	if fired != 1 {
		t.Errorf("expected 1 callback (Good only), got %d", fired)
	}
}

// ---------------------------------------------------------------------------
// Phase 13 (WATCH-08): the rewritten makeOnInventoryChange SINK — POSTs the
// raw UTF-8 content to the backend and handles the status switch.
// ---------------------------------------------------------------------------

// ingestRecorder is an httptest handler that records the request count + body
// and returns a fixed status. Mirrors internal/backend's client_test pattern.
type ingestRecorder struct {
	status   int
	count    int32
	lastBody string
}

func (ir *ingestRecorder) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&ir.count, 1)
		b, _ := readAll(r)
		ir.lastBody = b
		w.WriteHeader(ir.status)
	}
}

func (ir *ingestRecorder) requests() int { return int(atomic.LoadInt32(&ir.count)) }

func readAll(r *http.Request) (string, error) {
	b, err := io.ReadAll(r.Body)
	return string(b), err
}

// fastBackend returns a *backend.Client whose retry backoff is near-zero so the
// 5xx-path test does not sleep for real.
func fastBackend(t *testing.T, srv *httptest.Server) *backend.Client {
	t.Helper()
	c := backend.NewWithHTTPClient(srv.URL, srv.Client())
	c.SetBackoffForTest([]time.Duration{0, 0, 0})
	return c
}

// ---------------------------------------------------------------------------
// Phase 13 (HIGH-01 regression): the watcherRunning CAS guard ensures only ONE
// watcher phase runs at a time. A second concurrent RunApp invocation while a
// watcher is already up (a guildie re-clicking "Enter guild code…" while
// connected) must lose the CAS and no-op — no second watch.Run, no second
// daily-update goroutine, and no second unsynchronized writer of the shared
// cfg.LastKnown*Mtime maps (which would risk `fatal error: concurrent map
// writes`).
// ---------------------------------------------------------------------------

// TestWatcherRunningGuard_SecondEntryNoOps proves the CAS guard at the boundary
// it protects, headlessly (no real fsnotify watcher, no tray syscalls, no
// backend round-trip). We simulate a first watcher already being up by holding
// watcherRunning true, then call RunApp; its watcher phase must lose the CAS and
// return early WITHOUT starting a second watcher. Observable proof: with a guild
// code + EQ folder present (so RunApp falls THROUGH onboarding to the guarded
// watcher phase), the tray is never flipped to green and the ingest backend
// records ZERO requests — the second watcher's rescanCatchUp/watch.Run never ran.
func TestWatcherRunningGuard_SecondEntryNoOps(t *testing.T) {
	// The guard is exercised through credstore.Read (real DPAPI on Windows). On
	// a box without a usable credential store, skip — the credstore round-trip
	// test makes the same platform assumption.
	const probeCode = "high01-guard-probe-code"
	if err := credstore.Store(probeCode); err != nil {
		t.Skipf("credstore unavailable on this platform: %v", err)
	}
	t.Cleanup(func() { _ = credstore.Delete() })

	withTempLOCALAPPDATA(t) // redirect cfg.Save() under tmp (belt-and-braces)

	// Hold the guard as if a first watcher were already running.
	if !watcherRunning.CompareAndSwap(false, true) {
		t.Fatal("watcherRunning was already held at test start; tests must leave it false")
	}
	t.Cleanup(func() { watcherRunning.Store(false) })

	// A backend that records any request. If the second watcher started, its
	// rescanCatchUp would POST the seeded inventory file here.
	ir := &ingestRecorder{status: http.StatusNoContent}
	srv := httptest.NewServer(ir.handler())
	defer srv.Close()

	// Seed an EQ folder with one inventory file so the guarded watcher phase,
	// had it run, would have something to upload via catch-up.
	eqDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(eqDir, "Foo-Inventory.txt"),
		[]byte("Belt\tThing\t1\t1\t0\n"), 0o644); err != nil {
		t.Fatalf("seed inv: %v", err)
	}

	cfg := &config.Config{
		Version:                 1,
		LogLevel:                "info",
		EQFolders:               []string{eqDir},
		LastKnownInventoryMtime: map[string]string{},
		LastKnownSpellbookMtime: map[string]string{},
	}
	tc := tray.NewController(tray.Config{})

	// RunApp must return promptly (the CAS-fail path is a synchronous early
	// return). Guard against a hang/regression with a timeout.
	done := make(chan struct{})
	go func() {
		RunApp(context.Background(), cfg, srv.URL, "2.0.0", tc)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("RunApp did not return — the watcherRunning CAS guard did not no-op the second entry")
	}

	if ir.requests() != 0 {
		t.Fatalf("second RunApp made %d backend request(s); want 0 — a second watcher started despite the guard", ir.requests())
	}

	// Sanity: the guard must still be held (RunApp's `defer watcherRunning.Store(false)`
	// belongs to the FIRST watcher; the no-op second entry must NOT have cleared it).
	if !watcherRunning.Load() {
		t.Fatal("the no-op second entry cleared watcherRunning; it must leave the first watcher's flag intact")
	}
}

// TestMakeOnInventoryChange_204PersistsMtime: a 204 from the backend persists
// the file's mtime into cfg.LastKnownInventoryMtime and saves config.
func TestMakeOnInventoryChange_204PersistsMtime(t *testing.T) {
	p := withTempLOCALAPPDATA(t) // redirect cfg.Save() under tmp
	dir := filepath.Dir(filepath.Dir(p))

	ir := &ingestRecorder{status: http.StatusNoContent}
	srv := httptest.NewServer(ir.handler())
	defer srv.Close()
	bc := fastBackend(t, srv)

	invPath := filepath.Join(dir, "Foo-Inventory.txt")
	if err := os.WriteFile(invPath, []byte("Belt\tFine Steel Long Sword\t5616\t1\t0\n"), 0o644); err != nil {
		t.Fatalf("write inv: %v", err)
	}

	cfg := &config.Config{Version: 1, LogLevel: "info", LastKnownInventoryMtime: map[string]string{}}
	tc := tray.NewController(tray.Config{})

	cb := makeOnInventoryChange(context.Background(), bc, cfg, "CODE", "2.0.0", tc)
	cb(invPath)

	if ir.requests() != 1 {
		t.Fatalf("backend saw %d requests, want 1", ir.requests())
	}
	if cfg.LastKnownInventoryMtime["Foo"] == "" {
		t.Errorf("mtime not persisted for Foo after a 204")
	}
}

// TestMakeOnInventoryChange_401NoLoopSetsRed: a 401 from the backend does NOT
// retry (the handler sees exactly one request) and the callback does not loop.
func TestMakeOnInventoryChange_401NoLoopSetsRed(t *testing.T) {
	p := withTempLOCALAPPDATA(t)
	dir := filepath.Dir(filepath.Dir(p))

	ir := &ingestRecorder{status: http.StatusUnauthorized}
	srv := httptest.NewServer(ir.handler())
	defer srv.Close()
	bc := fastBackend(t, srv)

	invPath := filepath.Join(dir, "Foo-Inventory.txt")
	if err := os.WriteFile(invPath, []byte("Belt\tThing\t1\t1\t0\n"), 0o644); err != nil {
		t.Fatalf("write inv: %v", err)
	}

	cfg := &config.Config{Version: 1, LogLevel: "info", LastKnownInventoryMtime: map[string]string{}}
	tc := tray.NewController(tray.Config{})

	cb := makeOnInventoryChange(context.Background(), bc, cfg, "BADCODE", "2.0.0", tc)
	cb(invPath)

	if ir.requests() != 1 {
		t.Fatalf("401 path made %d requests, want exactly 1 (no retry loop — D-5/Pitfall 5)", ir.requests())
	}
	// On a 401 the mtime must NOT be persisted (the upload did not succeed).
	if cfg.LastKnownInventoryMtime["Foo"] != "" {
		t.Errorf("mtime persisted on a 401; want untouched")
	}
}

// TestMakeOnInventoryChange_EmptyFileSkipsNoRequest: an empty (whitespace-only)
// file is skipped with NO backend request (T-07-05 carry-over).
func TestMakeOnInventoryChange_EmptyFileSkipsNoRequest(t *testing.T) {
	p := withTempLOCALAPPDATA(t)
	dir := filepath.Dir(filepath.Dir(p))

	ir := &ingestRecorder{status: http.StatusNoContent}
	srv := httptest.NewServer(ir.handler())
	defer srv.Close()
	bc := fastBackend(t, srv)

	invPath := filepath.Join(dir, "Foo-Inventory.txt")
	if err := os.WriteFile(invPath, []byte("   \n\t\n"), 0o644); err != nil {
		t.Fatalf("write inv: %v", err)
	}

	cfg := &config.Config{Version: 1, LogLevel: "info", LastKnownInventoryMtime: map[string]string{}}
	tc := tray.NewController(tray.Config{})

	cb := makeOnInventoryChange(context.Background(), bc, cfg, "CODE", "2.0.0", tc)
	cb(invPath)

	if ir.requests() != 0 {
		t.Fatalf("empty-file path made %d requests, want 0 (skip-empty guard)", ir.requests())
	}
}

// TestMakeOnInventoryChange_426UpdateNeeded: a 426 surfaces "update needed" and
// does NOT loop (single request).
func TestMakeOnInventoryChange_426UpdateNeeded(t *testing.T) {
	p := withTempLOCALAPPDATA(t)
	dir := filepath.Dir(filepath.Dir(p))

	ir := &ingestRecorder{status: http.StatusUpgradeRequired}
	srv := httptest.NewServer(ir.handler())
	defer srv.Close()
	bc := fastBackend(t, srv)

	invPath := filepath.Join(dir, "Foo-Inventory.txt")
	if err := os.WriteFile(invPath, []byte("Belt\tThing\t1\t1\t0\n"), 0o644); err != nil {
		t.Fatalf("write inv: %v", err)
	}

	cfg := &config.Config{Version: 1, LogLevel: "info", LastKnownInventoryMtime: map[string]string{}}
	tc := tray.NewController(tray.Config{})

	cb := makeOnInventoryChange(context.Background(), bc, cfg, "CODE", "1.9.0", tc)
	cb(invPath)

	if ir.requests() != 1 {
		t.Fatalf("426 path made %d requests, want exactly 1 (no retry)", ir.requests())
	}
	if cfg.LastKnownInventoryMtime["Foo"] != "" {
		t.Errorf("mtime persisted on a 426; want untouched")
	}
}

// TestMakeOnSpellbookChange_204PersistsMtime: a 204 persists the file's mtime
// into cfg.LastKnownSpellbookMtime and saves config. Mirrors the inventory 204 test.
func TestMakeOnSpellbookChange_204PersistsMtime(t *testing.T) {
	p := withTempLOCALAPPDATA(t)
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

// TestMakeOnSpellbookChange_401NoLoopSetsRed: a 401 does NOT retry (exactly one
// request) and the mtime is NOT persisted. Mirrors the inventory 401 test.
func TestMakeOnSpellbookChange_401NoLoopSetsRed(t *testing.T) {
	p := withTempLOCALAPPDATA(t)
	dir := filepath.Dir(filepath.Dir(p))

	ir := &ingestRecorder{status: http.StatusUnauthorized}
	srv := httptest.NewServer(ir.handler())
	defer srv.Close()
	bc := fastBackend(t, srv)

	spbPath := filepath.Join(dir, "Foo-Spellbook.txt")
	if err := os.WriteFile(spbPath, []byte("9\tLifetap\n"), 0o644); err != nil {
		t.Fatalf("write spb: %v", err)
	}

	cfg := &config.Config{Version: 1, LogLevel: "info", LastKnownSpellbookMtime: map[string]string{}}
	tc := tray.NewController(tray.Config{})

	cb := makeOnSpellbookChange(context.Background(), bc, cfg, "BADCODE", "2.0.0", tc)
	cb(spbPath)

	if ir.requests() != 1 {
		t.Fatalf("401 path made %d requests, want exactly 1 (no retry loop — D-5/Pitfall 5)", ir.requests())
	}
	if cfg.LastKnownSpellbookMtime["Foo"] != "" {
		t.Errorf("mtime persisted on a 401; want untouched")
	}
}

// TestMakeOnSpellbookChange_EmptyFileSkipsNoRequest: a whitespace-only spellbook
// file is skipped with NO backend request. Mirrors the inventory empty-file test.
func TestMakeOnSpellbookChange_EmptyFileSkipsNoRequest(t *testing.T) {
	p := withTempLOCALAPPDATA(t)
	dir := filepath.Dir(filepath.Dir(p))

	ir := &ingestRecorder{status: http.StatusNoContent}
	srv := httptest.NewServer(ir.handler())
	defer srv.Close()
	bc := fastBackend(t, srv)

	spbPath := filepath.Join(dir, "Foo-Spellbook.txt")
	if err := os.WriteFile(spbPath, []byte("   \n\t\n"), 0o644); err != nil {
		t.Fatalf("write spb: %v", err)
	}

	cfg := &config.Config{Version: 1, LogLevel: "info", LastKnownSpellbookMtime: map[string]string{}}
	tc := tray.NewController(tray.Config{})

	cb := makeOnSpellbookChange(context.Background(), bc, cfg, "CODE", "2.0.0", tc)
	cb(spbPath)

	if ir.requests() != 0 {
		t.Fatalf("empty-file path made %d requests, want 0 (skip-empty guard)", ir.requests())
	}
}

// TestMakeOnSpellbookChange_426UpdateNeeded: a 426 surfaces "update needed" and
// does NOT loop (single request). Mirrors the inventory 426 test.
func TestMakeOnSpellbookChange_426UpdateNeeded(t *testing.T) {
	p := withTempLOCALAPPDATA(t)
	dir := filepath.Dir(filepath.Dir(p))

	ir := &ingestRecorder{status: http.StatusUpgradeRequired}
	srv := httptest.NewServer(ir.handler())
	defer srv.Close()
	bc := fastBackend(t, srv)

	spbPath := filepath.Join(dir, "Foo-Spellbook.txt")
	if err := os.WriteFile(spbPath, []byte("9\tLifetap\n"), 0o644); err != nil {
		t.Fatalf("write spb: %v", err)
	}

	cfg := &config.Config{Version: 1, LogLevel: "info", LastKnownSpellbookMtime: map[string]string{}}
	tc := tray.NewController(tray.Config{})

	cb := makeOnSpellbookChange(context.Background(), bc, cfg, "CODE", "1.9.0", tc)
	cb(spbPath)

	if ir.requests() != 1 {
		t.Fatalf("426 path made %d requests, want exactly 1 (no retry)", ir.requests())
	}
	if cfg.LastKnownSpellbookMtime["Foo"] != "" {
		t.Errorf("mtime persisted on a 426; want untouched")
	}
}

// TestMakeOnSpellbookChange_409CrossOwnerNoPersist: a 409 is terminal — exactly
// one request, no retry, and the mtime is NOT persisted (upload was rejected).
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
