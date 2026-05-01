package parse

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// Test 1: Empty input → (nil, nil).
func TestParse_EmptyInput(t *testing.T) {
	rows, err := Parse(bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if rows != nil {
		t.Fatalf("expected nil rows on empty input, got %v", rows)
	}
}

// Test 2: Header row only → 0 rows, no error.
func TestParse_HeaderOnly(t *testing.T) {
	in := "Location\tName\tID\tCount\tSlots\n"
	rows, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected 0 rows, got %d", len(rows))
	}
}

// Test 3: Header + 3 data rows → 3 rows; header is dropped (column 2 = "ID" is non-numeric).
func TestParse_HeaderPlusThreeRows(t *testing.T) {
	in := "Location\tName\tID\tCount\tSlots\n" +
		"General1\tCloth Cap\t1001\t1\t0\n" +
		"General2\tCloth Sandals\t1002\t1\t0\n" +
		"Bank1\tFungi Tunic\t13128\t1\t0\n"
	rows, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	// Row 0 should be the first data row, not "Location/Name/ID/...".
	if rows[0][1] != "Cloth Cap" {
		t.Errorf("expected first data row Name=Cloth Cap, got %q", rows[0][1])
	}
}

// Test 4: No header (first row's column 2 is numeric) → all rows kept.
func TestParse_NoHeader_AllKept(t *testing.T) {
	in := "General1\tCloth Cap\t1234\t1\t0\n" +
		"General2\tCloth Sandals\t1235\t1\t0\n"
	rows, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows (no header to drop), got %d", len(rows))
	}
	if rows[0][2] != "1234" {
		t.Errorf("expected first row ID=1234, got %q", rows[0][2])
	}
}

// Test 5: 7-column row truncates to first 5.
func TestParse_SevenColumnsTruncates(t *testing.T) {
	in := "General1\tCloth Cap\t1001\t1\t0\textra1\textra2\n"
	rows, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if len(rows[0]) != 5 {
		t.Fatalf("expected truncation to 5 columns, got %d", len(rows[0]))
	}
	if rows[0][4] != "0" {
		t.Errorf("expected last kept column = '0', got %q", rows[0][4])
	}
}

// Test 6: 4-column row is skipped (does not panic, does not pad).
func TestParse_FourColumnsSkipped(t *testing.T) {
	in := "General1\tCloth Cap\t1001\t1\n" +
		"General2\tCloth Sandals\t1002\t1\t0\n"
	rows, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row (short row skipped), got %d", len(rows))
	}
	if rows[0][1] != "Cloth Sandals" {
		t.Errorf("expected the surviving row to be the 5-col one, got %q", rows[0][1])
	}
}

// Test 7: CP1252 byte 0x92 in Name field round-trips to UTF-8 right single quotation mark U+2019.
// See RESEARCH.md §9.2 Pitfall: UTF-8 decoding produces mojibake; charmap.Windows1252 decodes correctly.
func TestParse_CP1252CurlyApostrophe(t *testing.T) {
	// "Brell\x92s Trinket" in CP1252 → "Brell’s Trinket" in UTF-8.
	in := []byte("General1\tBrell\x92s Trinket\t13128\t1\t0\n")
	rows, err := Parse(bytes.NewReader(in))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	got := rows[0][1]
	want := "Brell’s Trinket" // U+2019 RIGHT SINGLE QUOTATION MARK
	if got != want {
		t.Errorf("CP1252 decode mismatch:\n  got:  %q (% x)\n  want: %q (% x)", got, []byte(got), want, []byte(want))
	}
	// Also explicitly assert the UTF-8 byte sequence E2 80 99 is present.
	if !bytes.Contains([]byte(got), []byte{0xE2, 0x80, 0x99}) {
		t.Errorf("expected UTF-8 bytes E2 80 99 (U+2019) in Name; got bytes % x", []byte(got))
	}
}

// Test 7b (load-bearing supplement): Parse the on-disk CP1252 sample file end-to-end.
func TestParse_CP1252SampleFile(t *testing.T) {
	f, err := os.Open("testdata/sample-inventory-with-cp1252.txt")
	if err != nil {
		t.Fatalf("open testdata: %v", err)
	}
	defer f.Close()
	rows, err := Parse(f)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(rows) != 5 {
		t.Fatalf("expected 5 rows from CP1252 sample (header dropped), got %d", len(rows))
	}
	// Row 0 = "Brell’s Trinket"; row 2 = "Tashan’s Lance".
	if !strings.Contains(rows[0][1], "’") {
		t.Errorf("row 0 Name should contain U+2019, got %q (% x)", rows[0][1], []byte(rows[0][1]))
	}
	if !strings.Contains(rows[2][1], "’") {
		t.Errorf("row 2 Name should contain U+2019, got %q (% x)", rows[2][1], []byte(rows[2][1]))
	}
}

// Test 8: 250-row sample → exactly 250 rows.
func TestParse_250RowSample(t *testing.T) {
	f, err := os.Open("testdata/sample-inventory.txt")
	if err != nil {
		t.Fatalf("open testdata: %v", err)
	}
	defer f.Close()
	rows, err := Parse(f)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(rows) != 250 {
		t.Fatalf("expected exactly 250 rows from sample-inventory.txt, got %d", len(rows))
	}
	for i, row := range rows {
		if len(row) != 5 {
			t.Errorf("row %d: expected 5 columns, got %d", i, len(row))
		}
	}
}

// Test 9: LazyQuotes accepts an unescaped apostrophe in Name (e.g., Tashan's Lance).
func TestParse_LazyQuotesApostrophe(t *testing.T) {
	in := "General1\tTashan's Lance\t1001\t1\t0\n"
	rows, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("unexpected error (LazyQuotes should tolerate stray '): %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0][1] != "Tashan's Lance" {
		t.Errorf("expected Name='Tashan's Lance', got %q", rows[0][1])
	}
}
