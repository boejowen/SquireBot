package compute

// inventory.go is the Phase 29 COMPUTE TRANSFORM LAYER — the upper half of the
// compute/view.go ↔ store/readviews.go compute-on-read seam. It holds the genuinely-
// new INV-05 logic (classifySlot + the one-level <Parent>-Slot<N> nesting parser) plus
// the DATA-02 bank valuation + total-platinum aggregation, all as PURE functions over
// the store reads from Plan 29-01. compute authors ZERO SQL — it consumes the typed
// store structs (the dependency runs compute → store; store never imports compute).
//
// The public-entry → pure-helper split mirrors view.go (View/buildViewRows): the
// public fn fetches via *store.Store, the pure helper takes typed rows and returns the
// model with no ctx/store inside, so it is directly table-testable.
//
// Reuse discipline (do not reimplement): pickPrice (view.go) for D-03 valuation;
// ListBankToons (store/coin.go) for D-04 platinum. The price join already happened in
// the store by NORMALIZED NAME (the pp_rep CTE, commit 0a169f3) — never by item_id.

import (
	"regexp"
	"strings"
)

// subSlotRe matches a bag/augment sub-slot suffix ("Slot1", "Slot12", …). The full
// Location of a nested item is "<Parent>-Slot<N>"; we split on the FIRST '-' and test
// the suffix against this. Anchored so only a real "Slot<digits>" suffix nests.
var subSlotRe = regexp.MustCompile(`^Slot\d+$`)

// generalRe / bankRe match the top-level container tokens case-insensitively
// (General<N> / Bank<N>), so live data in any case (A5) classifies correctly.
var (
	generalRe = regexp.MustCompile(`(?i)^general\d+$`)
	bankRe    = regexp.MustCompile(`(?i)^bank\d+$`)
)

// classifySlot maps an inventory Location to its category + canonical slot key. It
// splits the Location on the FIRST '-' into (parent, suffix); the PARENT token decides
// the category (a "General4-Slot1" child is general, a "Bank1-Slot1" child is bank, a
// "Head-Slot1" augment is equipment). Comparison is case-insensitive (A5 / Pitfall 5);
// the emitted canonical key is Title-case.
//
//	^[Gg]eneral\d+  → (general, canonical "GeneralN")
//	^[Bb]ank\d+     → (bank,    canonical "BankN")
//	known equip tok → (equipment, the canonical Title-case token)
//	anything else   → (general, the raw parent) — defensive default, never panics (T-29-05)
func classifySlot(location string) (SlotCategory, string) {
	parent := location
	if i := strings.IndexByte(location, '-'); i >= 0 {
		parent = location[:i]
	}

	switch {
	case generalRe.MatchString(parent):
		return SlotGeneral, canonicalNumbered("General", parent)
	case bankRe.MatchString(parent):
		return SlotBank, canonicalNumbered("Bank", parent)
	}

	if canonical, ok := equipmentSlotsLC[strings.ToLower(parent)]; ok {
		return SlotEquipment, canonical
	}

	// Defensive default (empty token, unknown token, malformed Location): classify as
	// general and pass the raw parent through — never panic (T-29-05 robustness).
	return SlotGeneral, parent
}

// canonicalNumbered emits the canonical "<Prefix><N>" key (Title-case prefix +
// preserved trailing digits) for a case-insensitive General/Bank token match — so
// "general10" → "General10", "BANK8" → "Bank8".
func canonicalNumbered(prefix, parent string) string {
	digits := parent[len(prefix):] // len(parent) >= len(prefix); the regex guaranteed prefix+digits
	return prefix + digits
}
