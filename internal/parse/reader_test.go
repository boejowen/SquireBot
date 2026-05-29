package parse

import (
	"bytes"
	"os"
	"strconv"
	"strings"
	"testing"
)

// reader_test.go is the home for the CP1252-decode coverage AFTER the shared
// Parse/ParseSpellbook entries stopped decoding CP1252 (encoding contract A1 /
// RESEARCH "Encoding Note" resolution 1/2). The watcher (P13 + the existing v1
// watcher in internal/app/runapp.go) now wraps its disk reader in CP1252Reader
// BEFORE parsing; the server parser trusts its io.Reader is already UTF-8.
//
// The four CP1252-dependent inventory+spellbook tests that used to live in
// inventory_test.go / spellbook_test.go are re-homed here (wrapped in
// CP1252Reader), plus a NEW UTF-8-content case proving the bare Parse no longer
// double-decodes.

// TestCP1252Reader_DecodesCurlyApostrophe re-homes the old inventory
// TestParse_CP1252CurlyApostrophe: raw CP1252 byte 0x92 in a Name decodes to
// UTF-8 U+2019 when the reader is wrapped in CP1252Reader (the watcher's path).
func TestCP1252Reader_DecodesCurlyApostrophe(t *testing.T) {
	// "Brell\x92s Trinket" in CP1252 → "Brell’s Trinket" in UTF-8.
	in := []byte("General1\tBrell\x92s Trinket\t13128\t1\t0\n")
	rows, err := Parse(CP1252Reader(bytes.NewReader(in)))
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
	if !bytes.Contains([]byte(got), []byte{0xE2, 0x80, 0x99}) {
		t.Errorf("expected UTF-8 bytes E2 80 99 (U+2019) in Name; got bytes % x", []byte(got))
	}
}

// TestCP1252Reader_DecodesSampleFile re-homes the old inventory
// TestParse_CP1252SampleFile: the on-disk CP1252 sample decodes to U+2019 names
// when wrapped in CP1252Reader (the watcher's disk-read path).
func TestCP1252Reader_DecodesSampleFile(t *testing.T) {
	f, err := os.Open("testdata/sample-inventory-with-cp1252.txt")
	if err != nil {
		t.Fatalf("open testdata: %v", err)
	}
	defer f.Close()
	rows, err := Parse(CP1252Reader(f))
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

// TestParse_UTF8Content_NoDoubleDecode is the NEW server contract (A1): a UTF-8
// curly apostrophe in content survives byte-clean through the bare Parse (no
// charmap step → no double-decode → no mojibake). This is the case that fails
// while Parse still hardwires the CP1252 decoder.
func TestParse_UTF8Content_NoDoubleDecode(t *testing.T) {
	// Already-UTF-8 content (what the watcher POSTs under A1): U+2019 = E2 80 99.
	in := "General1\tTashan’s Lance\t1001\t1\t0\n"
	rows, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	got := rows[0][1]
	want := "Tashan’s Lance"
	if got != want {
		t.Errorf("UTF-8 content should pass through byte-clean (no double-decode):\n  got:  %q (% x)\n  want: %q (% x)", got, []byte(got), want, []byte(want))
	}
	// The exact UTF-8 bytes must be preserved (not re-encoded as CP1252 mojibake).
	if !bytes.Contains([]byte(got), []byte{0xE2, 0x80, 0x99}) {
		t.Errorf("expected UTF-8 bytes E2 80 99 (U+2019) preserved; got bytes % x", []byte(got))
	}
}

// TestParseSpellbook_CP1252Reader_DecodesCurlyApostrophe re-homes the old
// spellbook TestParseSpellbook_CP1252CurlyApostrophe: raw \x92 → U+2019 via
// CP1252Reader (symmetric with the inventory re-home).
func TestParseSpellbook_CP1252Reader_DecodesCurlyApostrophe(t *testing.T) {
	// "Tashan\x92s Cat" in CP1252 → "Tashan’s Cat" in UTF-8.
	in := []byte("9\tTashan\x92s Cat\n")
	rows, err := ParseSpellbook(CP1252Reader(bytes.NewReader(in)))
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

// TestParseSpellbook_CP1252Reader_SlampeachFixture re-homes the old spellbook
// TestParseSpellbook_SlampeachFixture: the real on-disk CP1252 spellbook returns
// 49 byte-clean rows when wrapped in CP1252Reader (the watcher's disk path).
func TestParseSpellbook_CP1252Reader_SlampeachFixture(t *testing.T) {
	f, err := os.Open("testdata/Slampeach-Spellbook.txt")
	if err != nil {
		t.Fatalf("open testdata: %v", err)
	}
	defer f.Close()
	rows, err := ParseSpellbook(CP1252Reader(f))
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
