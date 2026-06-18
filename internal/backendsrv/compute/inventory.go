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
	"context"
	"log/slog"
	"regexp"
	"strings"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
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

// StructuredInventory fetches one character's full inventory and builds the INV-05
// structured slot model (equipment/general/bank + canonical slot + one-level bag nesting
// + name-joined price/last-listed). The fetch is in InventoryForChar (Plan 29-01); the
// transform (buildStructuredInventory) is pure (no store access) so it is directly
// unit-testable — mirroring the View/buildViewRows split in view.go.
func StructuredInventory(ctx context.Context, s *store.Store, char string) (CharacterInventory, error) {
	rows, err := s.InventoryForChar(ctx, char)
	if err != nil {
		return CharacterInventory{}, err
	}
	return buildStructuredInventory(char, rows), nil
}

// buildStructuredInventory is the pure transform: classify each row, build the
// parent→children nesting tree (one level deep — bags-in-bags don't exist in classic EQ),
// and group into Equipment/General/Bank. Empty slots (item_id 0) are KEPT (the paperdoll
// renders empty positions). Rows arrive in row_ordinal order and that order is preserved.
//
// Two passes: pass 1 indexes the TOP-LEVEL container slots by raw Location; pass 2 nests
// the *-Slot* children under their parent container. Augments on equipment (Head-Slot1,
// A3) are flattened/ignored for the INV-05 paperdoll window. A grandchild (a *-Slot* whose
// parent is itself a child — A2) is slog.Warn'd (op + the two Locations only, never item
// content — V7) and flattened to top-level rather than panicking (T-29-05).
func buildStructuredInventory(char string, rows []store.InventoryRow) CharacterInventory {
	inv := CharacterInventory{Char: char}

	// Build every slot once, remembering which are top-level containers (by raw Location)
	// so children can find their parent in pass 2.
	type indexed struct {
		slot     InventorySlot
		category SlotCategory
		isChild  bool   // has a "-Slot<N>" suffix
		parent   string // the parent container Location for a child
	}
	all := make([]indexed, 0, len(rows))
	topLevel := make(map[string]bool, len(rows)) // raw Location → is a top-level (non-child) slot

	for _, row := range rows {
		cat, canonical := classifySlot(row.Location)
		slot := slotFromRow(row, cat, canonical)

		parent, isChild := splitChild(row.Location)
		if !isChild {
			topLevel[row.Location] = true
		}
		all = append(all, indexed{slot: slot, category: cat, isChild: isChild, parent: parent})
	}

	// Pass A: append every top-level slot to its destination group (Equipment/General/Bank),
	// preserving row_ordinal order within each group.
	for i := range all {
		it := &all[i]
		if it.isChild {
			continue
		}
		switch it.category {
		case SlotEquipment:
			inv.Equipment = append(inv.Equipment, it.slot)
		case SlotBank:
			inv.Bank = append(inv.Bank, it.slot)
		default: // SlotGeneral
			inv.General = append(inv.General, it.slot)
		}
	}

	// Index the top-level slots by raw Location AFTER Pass A completes — the group slices
	// are now stable (no further appends in Pass A), so these element pointers stay valid
	// while Pass B appends to Children (a different, per-slot slice). Capturing the pointers
	// mid-append in Pass A would dangle when a later append reallocates the backing array.
	parentRef := make(map[string]*InventorySlot)
	for _, group := range []*[]InventorySlot{&inv.Equipment, &inv.General, &inv.Bank} {
		for i := range *group {
			parentRef[(*group)[i].Location] = &(*group)[i]
		}
	}

	// Pass B: nest children under their parent container.
	for i := range all {
		it := &all[i]
		if !it.isChild {
			continue
		}
		// Augment on an equipment slot (Head-Slot1, A3): flatten/ignore for the INV-05
		// paperdoll window — not nested under equipment, not promoted to top-level.
		if it.category == SlotEquipment {
			continue
		}
		parent, ok := parentRef[it.parent]
		if !ok || !topLevel[it.parent] {
			// Grandchild or orphan (A2 — bags-in-bags don't exist in classic EQ). Flatten to
			// top-level rather than panic; log op + the two Locations only (never content, V7).
			slog.Warn("compute.structured_inventory.orphan_child",
				"op", "buildStructuredInventory", "child", it.slot.Location, "parent", it.parent)
			switch it.category {
			case SlotBank:
				inv.Bank = append(inv.Bank, it.slot)
			default:
				inv.General = append(inv.General, it.slot)
			}
			continue
		}
		parent.Children = append(parent.Children, it.slot)
	}

	return inv
}

// slotFromRow maps a store.InventoryRow into an InventorySlot, attaching the name-joined
// price (pickPrice over the inline PriceDetail) + last-listed. Price is nil when unpriced.
func slotFromRow(row store.InventoryRow, cat SlotCategory, canonical string) InventorySlot {
	prices := pricesFromRow(row)
	return InventorySlot{
		Location:      row.Location,
		Category:      cat,
		CanonicalSlot: canonical,
		Item:          row.ItemName,
		ID:            row.ItemID,
		Count:         row.Count,
		Slots:         row.Slots,
		Price:         pickPrice(prices),
		LastListed:    row.LastListed,
		WikiURL:       row.WikiURL,
		WikiSummary:   row.WikiSummary,
		IsQuestItem:   row.IsQuestItem,
		Prices:        prices,
	}
}

// pricesFromRow builds the inline PriceDetail for an InventoryRow — the InventoryRow twin
// of view.go's pricesFromJoin. The price already bridged by NORMALIZED NAME in the store
// (the pp_rep CTE, commit 0a169f3), so this is 0-or-1 PriceDetail.
func pricesFromRow(row store.InventoryRow) []PriceDetail {
	if !row.HasPrice {
		return nil
	}
	return []PriceDetail{{Direction: row.Direction, A30: row.A30, T30: row.T30}}
}

// splitChild splits a "-Slot<N>" child Location into (parentLocation, true); a top-level
// Location (no "-Slot<N>" suffix) returns ("", false). It splits on the FIRST '-' and
// requires the suffix to match ^Slot\d+$ — so a hypothetical hyphenated equipment token
// without a Slot suffix is NOT treated as a child.
func splitChild(location string) (parent string, isChild bool) {
	i := strings.IndexByte(location, '-')
	if i < 0 {
		return "", false
	}
	suffix := location[i+1:]
	if !subSlotRe.MatchString(suffix) {
		return "", false
	}
	return location[:i], true
}

// BankValuationFor computes per-bank + guild-wide item value (Σ pickPrice×count) with a
// "+N unpriced" count, plus the total platinum (D-02/D-03/D-04). It reuses the existing
// bankOnly InventoryJoin (already is_bank_toon-scoped + name-joined) for the FLAT item-row
// list and ListBankToons for platinum. The transform (buildBankValuation) is pure.
//
// Named BankValuationFor (not BankValuation) because the result struct is type
// BankValuation — Go forbids a func and a type sharing a name in one package. The public
// API consumers (Phases 31-33) call BankValuationFor.
func BankValuationFor(ctx context.Context, s *store.Store) (BankValuation, error) {
	rows, err := s.InventoryJoin(ctx, true) // bankOnly — flat list incl. *-Slot* children (each is its own row)
	if err != nil {
		return BankValuation{}, err
	}
	toons, err := store.ListBankToons(ctx, s.DB())
	if err != nil {
		return BankValuation{}, err
	}
	return buildBankValuation(rows, toons), nil
}

// buildBankValuation is the pure transform. Pitfall 3 / D-02: valuation is a FLAT sum over
// ALL bank rows — the container (bag) itself AND its *-Slot* contents both count, because
// each is an independent inventory_item row. Do NOT walk the nesting tree to sum (children
// are already their own rows; the flat row list IS the valuation scope). An unpriced row
// contributes 0 value and increments UnpricedCount (the "+N unpriced" annotation).
func buildBankValuation(rows []store.InventoryJoinRow, toons []store.BankToon) BankValuation {
	bv := BankValuation{PerBank: make(map[string]Valuation)}
	for _, r := range rows {
		price := pickPrice(pricesFromJoin(r)) // reuse view.go's selector + name-joined prices
		per := bv.PerBank[r.Char]
		if price == nil {
			per.UnpricedCount++
			bv.GuildTotal.UnpricedCount++
		} else {
			value := *price * float64(r.Count)
			per.TotalValue += value
			bv.GuildTotal.TotalValue += value
		}
		bv.PerBank[r.Char] = per
	}
	bv.TotalPlatinum = TotalPlatinum(toons)
	return bv
}

// TotalPlatinum sums the literal plat column over live bank toons (D-04). gp/sp/cp are
// excluded (they stay available separately); a nil Plat (never entered) is skipped, never
// treated as 0.
func TotalPlatinum(banks []store.BankToon) int64 {
	var sum int64
	for _, b := range banks {
		if b.Plat != nil {
			sum += *b.Plat
		}
	}
	return sum
}
