package app

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/boejowen/SquireBot/internal/config"
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

func TestNeedsWizard(t *testing.T) {
	cases := []struct {
		name string
		cfg  *config.Config
		want bool
	}{
		{"empty", &config.Config{}, true},
		{"only email", &config.Config{GoogleEmail: "a@b.com"}, true},
		{"only spreadsheet", &config.Config{SpreadsheetID: "X"}, true},
		{"only folder", &config.Config{EQFolder: `C:\P99`}, true},
		{"email+spreadsheet missing folder", &config.Config{GoogleEmail: "a@b.com", SpreadsheetID: "X"}, true},
		{"all three (legacy EQFolder)", &config.Config{GoogleEmail: "a@b.com", SpreadsheetID: "X", EQFolder: `C:\P99`}, false},
		// Plan 02-02 (WATCH-03): EQFolders satisfies folder requirement too.
		{"all three (Phase 2 EQFolders)", &config.Config{GoogleEmail: "a@b.com", SpreadsheetID: "X", EQFolders: []string{`C:\P99`}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := needsWizard(tc.cfg)
			if got != tc.want {
				t.Errorf("needsWizard(%+v) = %v, want %v", tc.cfg, got, tc.want)
			}
		})
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

	var invPaths, spbPaths []string
	var invCount, spbCount int64
	onInv := func(p string) {
		atomic.AddInt64(&invCount, 1)
		invPaths = append(invPaths, p)
	}
	onSpb := func(p string) {
		atomic.AddInt64(&spbCount, 1)
		spbPaths = append(spbPaths, p)
	}

	rescanCatchUp(context.Background(), cfg, cfg.EQFolders, onInv, onSpb)

	if got := atomic.LoadInt64(&invCount); got != 1 {
		t.Errorf("inventory callback fired %d times; want 1 (Fresh only)", got)
	}
	if got := atomic.LoadInt64(&spbCount); got != 1 {
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

	var fired int64
	onInv := func(string) { atomic.AddInt64(&fired, 1) }
	onSpb := func(string) {}

	rescanCatchUp(context.Background(), cfg, cfg.EQFolders, onInv, onSpb)

	if got := atomic.LoadInt64(&fired); got != 1 {
		t.Errorf("expected 1 callback (Good only), got %d", got)
	}
}
