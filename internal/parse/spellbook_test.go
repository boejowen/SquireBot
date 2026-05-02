package parse

import (
	"bytes"
	"os"
	"strconv"
	"strings"
	"testing"
)

// Test 1: Empty input → (nil, nil), no error.
func TestParseSpellbook_EmptyInput(t *testing.T) {
	rows, err := ParseSpellbook(bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if rows != nil {
		t.Fatalf("expected nil rows on empty input, got %v", rows)
	}
}

// Test 2: Header-only input → 0 rows, no error.
// Header detected by non-numeric column 0 ("Level") and dropped.
func TestParseSpellbook_HeaderOnly(t *testing.T) {
	in := "Level\tName\n"
	rows, err := ParseSpellbook(strings.NewReader(in))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected 0 rows, got %d", len(rows))
	}
}

// Test 3: Header + 3 data rows → 3 rows; header dropped because column 0 = "Level"
// is non-numeric.
func TestParseSpellbook_HeaderPlusThreeRows(t *testing.T) {
	in := "Level\tName\n" +
		"9\tLifetap\n" +
		"15\tFear\n" +
		"30\tHeat Blood\n"
	rows, err := ParseSpellbook(strings.NewReader(in))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	if rows[0][0] != "9" || rows[0][1] != "Lifetap" {
		t.Errorf("expected first data row [9, Lifetap], got %v", rows[0])
	}
}

// Test 4: No-header input (column 0 numeric on first row) → all rows kept.
func TestParseSpellbook_NoHeader_AllKept(t *testing.T) {
	in := "9\tLifetap\n" +
		"15\tFear\n"
	rows, err := ParseSpellbook(strings.NewReader(in))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows (no header to drop), got %d", len(rows))
	}
	if rows[0][0] != "9" {
		t.Errorf("expected first row Level=9, got %q", rows[0][0])
	}
}

// Test 5: Slampeach fixture round-trips with exactly 49 unique (Level, Name) rows
// and every Level is an integer in [1, 60]. This is the load-bearing test that
// validates the parser against a real EQ-produced spellbook.
func TestParseSpellbook_SlampeachFixture(t *testing.T) {
	f, err := os.Open("testdata/Slampeach-Spellbook.txt")
	if err != nil {
		t.Fatalf("open testdata: %v", err)
	}
	defer f.Close()
	rows, err := ParseSpellbook(f)
	if err != nil {
		t.Fatalf("ParseSpellbook: %v", err)
	}
	if len(rows) != 49 {
		t.Fatalf("expected 49 rows from Slampeach fixture, got %d", len(rows))
	}
	seen := map[string]bool{}
	for i, row := range rows {
		if len(row) != 2 {
			t.Errorf("row %d: expected 2 columns, got %d", i, len(row))
			continue
		}
		n, err := strconv.Atoi(row[0])
		if err != nil {
			t.Errorf("row %d: Level %q is not an integer", i, row[0])
			continue
		}
		if n < 1 || n > 60 {
			t.Errorf("row %d: Level %d out of [1,60]", i, n)
		}
		if row[1] == "" {
			t.Errorf("row %d: Name is empty", i)
		}
		key := row[0] + "|" + row[1]
		if seen[key] {
			t.Errorf("duplicate (Level, Name) pair: %s", key)
		}
		seen[key] = true
	}
}

// Test 6: Row with non-integer Level is silently skipped (mirrors inventory's
// row-with-bad-ID behavior).
func TestParseSpellbook_NonIntLevelSkipped(t *testing.T) {
	in := "9\tLifetap\n" +
		"abc\tFoo Spell\n" +
		"15\tFear\n"
	rows, err := ParseSpellbook(strings.NewReader(in))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows (bad-Level row skipped), got %d", len(rows))
	}
	if rows[0][1] != "Lifetap" || rows[1][1] != "Fear" {
		t.Errorf("surviving rows mismatch: %v", rows)
	}
}

// Test 7: Row with only 1 column (no tab) is skipped.
func TestParseSpellbook_ShortRowSkipped(t *testing.T) {
	in := "9\tLifetap\n" +
		"15\n" +
		"30\tHeat Blood\n"
	rows, err := ParseSpellbook(strings.NewReader(in))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows (1-col row skipped), got %d", len(rows))
	}
	if rows[0][1] != "Lifetap" || rows[1][1] != "Heat Blood" {
		t.Errorf("surviving rows mismatch: %v", rows)
	}
}

// Test 8: CP-1252 byte 0x92 in Name (curly apostrophe) decodes to UTF-8 U+2019.
// Smoke-test for the charmap.Windows1252 decoder; mirrors inventory parser's
// CP1252 round-trip (RESEARCH.md §9.2).
func TestParseSpellbook_CP1252CurlyApostrophe(t *testing.T) {
	// "Tashan\x92s Cat" in CP1252 → "Tashan’s Cat" in UTF-8.
	in := []byte("9\tTashan\x92s Cat\n")
	rows, err := ParseSpellbook(bytes.NewReader(in))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	got := rows[0][1]
	want := "Tashan’s Cat" // U+2019 RIGHT SINGLE QUOTATION MARK
	if got != want {
		t.Errorf("CP1252 decode mismatch:\n  got:  %q (% x)\n  want: %q (% x)", got, []byte(got), want, []byte(want))
	}
	if !bytes.Contains([]byte(got), []byte{0xE2, 0x80, 0x99}) {
		t.Errorf("expected UTF-8 bytes E2 80 99 (U+2019) in Name; got bytes % x", []byte(got))
	}
}

// Test 9: Trailing extra columns are tolerated and truncated to 2.
func TestParseSpellbook_ExtraColumnsTruncated(t *testing.T) {
	in := "30\tHeat Blood\textra1\textra2\n"
	rows, err := ParseSpellbook(strings.NewReader(in))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if len(rows[0]) != 2 {
		t.Fatalf("expected truncation to 2 columns, got %d (%v)", len(rows[0]), rows[0])
	}
	if rows[0][0] != "30" || rows[0][1] != "Heat Blood" {
		t.Errorf("expected [30, Heat Blood], got %v", rows[0])
	}
}
