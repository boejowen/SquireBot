package enrich

import (
	"os"
	"testing"
)

// TestParseToRows_Fixture asserts byte-parity with the TS parseToRows against
// the real captured getall fixture: same row count + same decoded field values
// the TS test (pigparse-types.test.ts) asserts.
func TestParseToRows_Fixture(t *testing.T) {
	body, err := os.ReadFile("testdata/pigparse-getall-1.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	rows, err := ParseToRows(body)
	if err != nil {
		t.Fatalf("ParseToRows: %v", err)
	}
	// The TS test asserts 7,240 rows from the live fixture.
	if len(rows) != 7240 {
		t.Fatalf("row count = %d, want 7240", len(rows))
	}

	// First row matches the known fixture shape (TS: first row decoded).
	first := rows[0]
	if first.I != 19178 || first.T != 0 ||
		first.N != "10 Dose Adrenaline Tap" ||
		first.L != "2026-01-02T22:56:07.581+00:00" {
		t.Fatalf("first row = %+v, want {I:19178 T:0 N:%q L:%q}",
			first, "10 Dose Adrenaline Tap", "2026-01-02T22:56:07.581+00:00")
	}

	// Distinct t values in the fixture are exactly {0,1} (TS assertion).
	seenT := map[int]bool{}
	for _, r := range rows {
		seenT[r.T] = true
	}
	if !seenT[0] || !seenT[1] || len(seenT) != 2 {
		t.Fatalf("distinct t = %v, want exactly {0,1}", seenT)
	}

	// Spot-check item 19450's t=0 (WTS) row carries the exact price fields the
	// fixture holds — proves the price-history columns decode correctly.
	var wts *PigparseRow
	for i := range rows {
		if rows[i].I == 19450 && rows[i].T == 0 {
			wts = &rows[i]
			break
		}
	}
	if wts == nil {
		t.Fatal("item 19450 t=0 row not found")
	}
	if wts.N != "10 Dose Ant's Potion" ||
		wts.T30 != 30 || wts.A30 != 239 ||
		wts.T60 != 90 || wts.A60 != 240 ||
		wts.T6m != 908 || wts.A6m != 245 ||
		wts.Ty != 1669 || wts.Ay != 238 {
		t.Fatalf("item 19450 t=0 row = %+v, want T30=30 A30=239 T60=90 A60=240 T6m=908 A6m=245 Ty=1669 Ay=238", *wts)
	}
}

// TestParseToRows_KeepsBothDirections proves the parser does NOT dedup the
// t=0/t=1 duplicate item_id rows — item 19450 must appear TWICE (once each
// direction). The WTS-wins dedup (D-9) is the job's responsibility, not the
// parser's; keeping both preserves byte-parity with the TS output.
func TestParseToRows_KeepsBothDirections(t *testing.T) {
	body, err := os.ReadFile("testdata/pigparse-getall-1.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	rows, err := ParseToRows(body)
	if err != nil {
		t.Fatalf("ParseToRows: %v", err)
	}
	var t0, t1 int
	for _, r := range rows {
		if r.I != 19450 {
			continue
		}
		switch r.T {
		case 0:
			t0++
		case 1:
			t1++
		}
	}
	if t0 != 1 || t1 != 1 {
		t.Fatalf("item 19450 directions: t0=%d t1=%d, want exactly one of each (no dedup)", t0, t1)
	}
}

// TestParseToRows_NonArray asserts a non-array body errors (mirrors the TS
// Array.isArray guard).
func TestParseToRows_NonArray(t *testing.T) {
	if _, err := ParseToRows([]byte(`{}`)); err == nil {
		t.Fatal("expected error for non-array body, got nil")
	}
	// An object literal masquerading as the response should also error.
	if _, err := ParseToRows([]byte(`{"items": []}`)); err == nil {
		t.Fatal("expected error for object body, got nil")
	}
}

// TestParseToRows_Empty asserts an empty array yields an empty slice, no error
// (TS: returns empty array for empty input).
func TestParseToRows_Empty(t *testing.T) {
	rows, err := ParseToRows([]byte(`[]`))
	if err != nil {
		t.Fatalf("ParseToRows([]): %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("len = %d, want 0", len(rows))
	}
}

// TestParseToRows_TolerableMalformation asserts <=1% malformed rows are skipped
// and the valid rows returned (TS: 200 good + 1 bad in 201 → 200 returned).
func TestParseToRows_TolerableMalformation(t *testing.T) {
	// 200 valid rows + 1 malformed (i is a string) = 1/201 < 1%.
	var sb []byte
	sb = append(sb, '[')
	for k := 0; k < 200; k++ {
		if k > 0 {
			sb = append(sb, ',')
		}
		sb = append(sb, []byte(`{"i":1,"t":0,"n":"X"}`)...)
	}
	sb = append(sb, []byte(`,{"i":"not-a-number","t":0,"n":"X"}`)...)
	sb = append(sb, ']')

	rows, err := ParseToRows(sb)
	if err != nil {
		t.Fatalf("ParseToRows: %v", err)
	}
	if len(rows) != 200 {
		t.Fatalf("len = %d, want 200 (1 malformed skipped under tolerance)", len(rows))
	}
}

// TestParseToRows_TooMuchMalformation asserts >1% malformed rows error (TS:
// 100 good + 5 bad → throws).
func TestParseToRows_TooMuchMalformation(t *testing.T) {
	var sb []byte
	sb = append(sb, '[')
	for k := 0; k < 100; k++ {
		sb = append(sb, []byte(`{"i":1,"t":0,"n":"X"},`)...)
	}
	for k := 0; k < 5; k++ {
		if k > 0 {
			sb = append(sb, ',')
		}
		sb = append(sb, []byte(`{"i":"bad","t":0,"n":"X"}`)...)
	}
	sb = append(sb, ']')

	if _, err := ParseToRows(sb); err == nil {
		t.Fatal("expected error when malformed rows exceed 1% tolerance, got nil")
	}
}

// TestParseToRows_RejectsBadDirection asserts a row whose t is outside {0,1,2}
// is treated as malformed (TS: t=7 single-row → throws "too many malformed").
func TestParseToRows_RejectsBadDirection(t *testing.T) {
	if _, err := ParseToRows([]byte(`[{"i":1,"t":7,"n":"X"}]`)); err == nil {
		t.Fatal("expected error for t outside {0,1,2}, got nil")
	}
}

// TestParseToRows_CoercesMissingNumerics asserts absent numeric fields coerce
// to 0 (TS: row with no t30/ay → both 0).
func TestParseToRows_CoercesMissingNumerics(t *testing.T) {
	rows, err := ParseToRows([]byte(`[{"i":1,"t":0,"n":"X"}]`))
	if err != nil {
		t.Fatalf("ParseToRows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len = %d, want 1", len(rows))
	}
	if rows[0].T30 != 0 || rows[0].Ay != 0 {
		t.Fatalf("coerced numerics = T30:%d Ay:%v, want 0/0", rows[0].T30, rows[0].Ay)
	}
}

// TestParseToRows_TrimsName asserts the item name is trimmed (TS coerceRow:
// String(obj.n).trim()).
func TestParseToRows_TrimsName(t *testing.T) {
	rows, err := ParseToRows([]byte(`[{"i":1,"t":0,"n":"  Spiderling Silk  "}]`))
	if err != nil {
		t.Fatalf("ParseToRows: %v", err)
	}
	if rows[0].N != "Spiderling Silk" {
		t.Fatalf("name = %q, want %q", rows[0].N, "Spiderling Silk")
	}
}

// TestCLASSES_Has14 is the eqconst.go sanity assertion: the ported CLASSES
// list has exactly the 14 P1999 class abbreviations in order.
func TestCLASSES_Has14(t *testing.T) {
	if len(CLASSES) != 14 {
		t.Fatalf("len(CLASSES) = %d, want 14", len(CLASSES))
	}
	want := []string{"WAR", "CLR", "PAL", "RNG", "SHD", "DRU", "MNK", "BRD", "ROG", "SHM", "NEC", "WIZ", "MAG", "ENC"}
	for i, c := range want {
		if CLASSES[i] != c {
			t.Fatalf("CLASSES[%d] = %q, want %q", i, CLASSES[i], c)
		}
	}
	// Spot-check the lookup tables ported alongside.
	if CLASS_DISPLAY_TO_ABBREV["Necromancer"] != "NEC" {
		t.Fatalf("CLASS_DISPLAY_TO_ABBREV[Necromancer] = %q, want NEC", CLASS_DISPLAY_TO_ABBREV["Necromancer"])
	}
	if got := WIKI_SLOT_TO_INV_SLOTS["Ears"]; len(got) != 2 || got[0] != "EAR1" || got[1] != "EAR2" {
		t.Fatalf("WIKI_SLOT_TO_INV_SLOTS[Ears] = %v, want [EAR1 EAR2]", got)
	}
}
