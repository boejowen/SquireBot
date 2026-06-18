package compute

// slotconst_test.go is an INTERNAL test (package compute, not compute_test) because
// classifySlot + the canonical equipment slot set are package-private helpers, not
// part of the exported JSON contract. It covers the INV-05 slot classifier's
// <behavior> cases: case-insensitive equipment match emitting the canonical
// Title-case key, the parent-token-decides-category rule for *-Slot* children
// (general/bank/augment), and the never-panic defensive default for empty/unknown
// tokens (T-29-05 robustness).

import "testing"

func TestClassifySlot(t *testing.T) {
	cases := []struct {
		name     string
		location string
		wantCat  SlotCategory
		wantSlot string
	}{
		// Equipment — canonical Title-case key emitted.
		{"head equipment", "Head", SlotEquipment, "Head"},
		// Case-insensitive: live data may be upper-case (A5) — classifier must
		// still classify it AND emit the canonical Title-case key.
		{"head upper-case", "HEAD", SlotEquipment, "Head"},
		{"finger1 equipment", "Finger1", SlotEquipment, "Finger1"},
		{"ear2 equipment (A4 — fixture omits ears, real dumps have them)", "Ear2", SlotEquipment, "Ear2"},
		{"charm equipment", "Charm", SlotEquipment, "Charm"},

		// General — parent token is the container.
		{"general4 top-level", "General4", SlotGeneral, "General4"},
		{"general4 child nests to parent", "General4-Slot1", SlotGeneral, "General4"},
		{"general lower-case", "general10", SlotGeneral, "General10"},

		// Bank — parent token is the container.
		{"bank1 top-level", "Bank1", SlotBank, "Bank1"},
		{"bank1 child nests to parent", "Bank1-Slot1", SlotBank, "Bank1"},
		{"bank lower-case", "bank8", SlotBank, "Bank8"},

		// Augment on an equipment slot → the parent equipment slot (augment is not
		// its own category — A3).
		{"head augment → equipment parent", "Head-Slot1", SlotEquipment, "Head"},

		// Defensive: empty + unknown tokens default without panicking (T-29-05).
		{"empty token defaults to general (no panic)", "", SlotGeneral, ""},
		{"unknown token defaults to general (no panic)", "Mystery", SlotGeneral, "Mystery"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotCat, gotSlot := classifySlot(c.location)
			if gotCat != c.wantCat || gotSlot != c.wantSlot {
				t.Errorf("classifySlot(%q) = (%q, %q), want (%q, %q)",
					c.location, gotCat, gotSlot, c.wantCat, c.wantSlot)
			}
		})
	}
}
