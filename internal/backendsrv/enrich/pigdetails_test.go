package enrich

import (
	"os"
	"testing"
)

// pigdetails_test.go covers ParseItemDetail — the NEW per-item PigParse getdetails
// parser (Phase 21 plan 21-01, WANT-05). It is a DIFFERENT shape from ParseToRows
// (the getall aggregate): here `t` is a TIMESTAMP and `u` is the direction (the
// getall feed's `t` is the direction). The happy path uses the real captured
// fixture (__fixtures__/pigparse-getdetails-fungi.json); the edge cases use small
// inline literals.

// loadDetailFixture reads the captured real getdetails body.
func loadDetailFixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("__fixtures__/pigparse-getdetails-fungi.json")
	if err != nil {
		t.Fatalf("read getdetails fixture: %v", err)
	}
	return b
}

// TestParseItemDetail_RealFixture parses the real captured body: every record is
// preserved in source order, U/I/P/T map correctly, and a null `p` yields P==nil
// while a present `p` yields the int (NOT 0).
func TestParseItemDetail_RealFixture(t *testing.T) {
	d, err := ParseItemDetail(loadDetailFixture(t))
	if err != nil {
		t.Fatalf("ParseItemDetail(real fixture): %v", err)
	}
	if d.ItemName != "Fungus Covered Scale Tunic" {
		t.Errorf("ItemName = %q, want %q", d.ItemName, "Fungus Covered Scale Tunic")
	}
	if len(d.Items) != 12 {
		t.Fatalf("len(Items) = %d, want 12 (the fixture's record count)", len(d.Items))
	}
	// Source order: first record is the most-recent t with u=0 (WTS), p=null.
	first := d.Items[0]
	if first.U != 0 {
		t.Errorf("Items[0].U = %d, want 0 (WTS)", first.U)
	}
	if first.T != "2026-06-06T01:54:58.615+00:00" {
		t.Errorf("Items[0].T = %q, want the preserved RFC3339 timestamp string (NOT a direction)", first.T)
	}
	if first.P != nil {
		t.Errorf("Items[0].P = %v, want nil (the fixture's first record has p:null)", *first.P)
	}
	// A record with a non-null price keeps the int (the fixture has p=38000 somewhere).
	var sawNonNullPrice bool
	for _, r := range d.Items {
		if r.P != nil {
			sawNonNullPrice = true
			if *r.P <= 0 {
				t.Errorf("non-null price = %d, want a positive int", *r.P)
			}
		}
	}
	if !sawNonNullPrice {
		t.Error("expected at least one non-null price in the real fixture")
	}
	// players map present (anonymized in the fixture) — parser surfaces it.
	if len(d.Players) == 0 {
		t.Error("expected a non-empty players map from the fixture")
	}
}

// TestParseItemDetail_NullItemsYieldsEmptySlice: a null/missing items array yields
// a non-nil empty slice, no error (the API returns items:null for unseen items).
func TestParseItemDetail_NullItemsYieldsEmptySlice(t *testing.T) {
	for _, body := range []string{
		`{"items":null,"itemName":null,"players":{}}`,
		`{"itemName":"X"}`,
	} {
		d, err := ParseItemDetail([]byte(body))
		if err != nil {
			t.Fatalf("ParseItemDetail(%s): unexpected error %v", body, err)
		}
		if d.Items == nil {
			t.Errorf("ParseItemDetail(%s): Items is nil, want non-nil empty slice", body)
		}
		if len(d.Items) != 0 {
			t.Errorf("ParseItemDetail(%s): len(Items) = %d, want 0", body, len(d.Items))
		}
	}
}

// TestParseItemDetail_NullabledPrice: a JSON null `p` yields P==nil (NOT 0); a
// present `p` yields the int.
func TestParseItemDetail_NullablePrice(t *testing.T) {
	body := `{"items":[{"u":0,"i":1,"p":null,"t":"2026-06-06T01:00:00+00:00"},{"u":0,"i":1,"p":2000,"t":"2026-06-06T02:00:00+00:00"}],"itemName":"X","players":{}}`
	d, err := ParseItemDetail([]byte(body))
	if err != nil {
		t.Fatalf("ParseItemDetail: %v", err)
	}
	if len(d.Items) != 2 {
		t.Fatalf("len(Items) = %d, want 2", len(d.Items))
	}
	if d.Items[0].P != nil {
		t.Errorf("Items[0].P = %v, want nil (JSON null must NOT become 0)", *d.Items[0].P)
	}
	if d.Items[1].P == nil || *d.Items[1].P != 2000 {
		t.Errorf("Items[1].P = %v, want 2000", d.Items[1].P)
	}
}

// TestParseItemDetail_TCollisionRegression: a u=0 record's `t` is preserved as the
// RFC3339 timestamp string and is NOT interpreted as a direction (the getall `t`
// collision). u stays the direction.
func TestParseItemDetail_TCollisionRegression(t *testing.T) {
	body := `{"items":[{"u":0,"i":42,"p":1500,"t":"2026-06-05T20:37:32.726+00:00"}],"itemName":"Y","players":{}}`
	d, err := ParseItemDetail([]byte(body))
	if err != nil {
		t.Fatalf("ParseItemDetail: %v", err)
	}
	if len(d.Items) != 1 {
		t.Fatalf("len(Items) = %d, want 1", len(d.Items))
	}
	r := d.Items[0]
	if r.U != 0 {
		t.Errorf("U = %d, want 0 (WTS direction)", r.U)
	}
	if r.T != "2026-06-05T20:37:32.726+00:00" {
		t.Errorf("T = %q, want the raw timestamp string preserved", r.T)
	}
}

// TestParseItemDetail_TolerateMalformedUnderThreshold: a body with ≤1% malformed
// records skips them (no error). Build 200 good + 1 malformed (0.5% < 1%).
func TestParseItemDetail_TolerateMalformedUnderThreshold(t *testing.T) {
	var sb []byte
	sb = append(sb, []byte(`{"items":[`)...)
	for i := 0; i < 200; i++ {
		if i > 0 {
			sb = append(sb, ',')
		}
		sb = append(sb, []byte(`{"u":0,"i":1,"p":null,"t":"2026-06-06T01:00:00+00:00"}`)...)
	}
	// one malformed record (t is a number, not a string ⇒ invalid)
	sb = append(sb, []byte(`,{"u":0,"i":1,"p":null,"t":12345}`)...)
	sb = append(sb, []byte(`],"itemName":"X","players":{}}`)...)

	d, err := ParseItemDetail(sb)
	if err != nil {
		t.Fatalf("ParseItemDetail(<=1%% malformed): unexpected error %v", err)
	}
	if len(d.Items) != 200 {
		t.Errorf("len(Items) = %d, want 200 (the 1 malformed record skipped)", len(d.Items))
	}
}

// TestParseItemDetail_RejectMalformedOverThreshold: >1% malformed returns an error.
// 10 good + 5 malformed = 33% malformed.
func TestParseItemDetail_RejectMalformedOverThreshold(t *testing.T) {
	var sb []byte
	sb = append(sb, []byte(`{"items":[`)...)
	for i := 0; i < 10; i++ {
		if i > 0 {
			sb = append(sb, ',')
		}
		sb = append(sb, []byte(`{"u":0,"i":1,"p":null,"t":"2026-06-06T01:00:00+00:00"}`)...)
	}
	for i := 0; i < 5; i++ {
		sb = append(sb, []byte(`,{"u":0,"i":1,"p":null,"t":999}`)...)
	}
	sb = append(sb, []byte(`],"itemName":"X","players":{}}`)...)

	if _, err := ParseItemDetail(sb); err == nil {
		t.Error("ParseItemDetail(>1%% malformed): expected an error, got nil")
	}
}

// TestParseItemDetail_TruncatedBodyErrorsNoPanic: a non-object / truncated body
// returns an error and does NOT panic.
func TestParseItemDetail_TruncatedBodyErrorsNoPanic(t *testing.T) {
	for _, body := range []string{
		`[1,2,3]`,            // a top-level ARRAY, not the ItemDetail object
		`{"items":[{"u":0`,   // truncated mid-object
		`not json at all`,    // garbage
	} {
		if _, err := ParseItemDetail([]byte(body)); err == nil {
			t.Errorf("ParseItemDetail(%q): expected an error, got nil", body)
		}
	}
}
