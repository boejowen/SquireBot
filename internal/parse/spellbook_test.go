package parse

import (
	"bytes"
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

// NOTE: TestParseSpellbook_SlampeachFixture (the real on-disk CP1252 fixture)
// was re-homed to reader_test.go (wrapped in CP1252Reader) when encoding
// contract A1 moved the charmap decode OFF the shared ParseSpellbook entry —
// see TestParseSpellbook_CP1252Reader_SlampeachFixture. The UTF-8/ASCII-clean
// spellbook cases below stay here unchanged.

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

// NOTE: TestParseSpellbook_CP1252CurlyApostrophe (raw \x92 → U+2019) was
// re-homed to reader_test.go (wrapped in CP1252Reader) — see
// TestParseSpellbook_CP1252Reader_DecodesCurlyApostrophe. The bare
// ParseSpellbook now trusts UTF-8 input (encoding contract A1).

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
