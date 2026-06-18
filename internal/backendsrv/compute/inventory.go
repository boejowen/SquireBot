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
	"strconv"
	"strings"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// subSlotRe matches a bag/augment sub-slot suffix ("Slot1", "Slot12", "SLOT1", …). The
// full Location of a nested item is "<Parent>-Slot<N>"; we split on the FIRST '-' and test
// the suffix against this. Anchored so only a real "Slot<digits>" suffix nests. Match is
// case-INSENSITIVE (A5), consistent with generalRe/bankRe/equipmentSlotsLC — otherwise
// uppercase live data ("GENERAL4-SLOT1") would fail to nest and surface as a phantom
// top-level slot colliding on the container's canonical key (WR-01).
var subSlotRe = regexp.MustCompile(`(?i)^slot\d+$`)

// generalRe / bankRe / sharedBankRe match the top-level container tokens case-insensitively
// (General<N> / Bank<N> / SharedBank<N>), so live data in any case (A5) classifies correctly.
// sharedBankRe is checked BEFORE bankRe: real /outputfile inventory writes the account-wide
// shared-bank slots as "SharedBank<N>" alongside the personal "Bank<N>"; without its own
// pattern they fall to the defensive-general default and surface as loose general items.
var (
	generalRe    = regexp.MustCompile(`(?i)^general\d+$`)
	bankRe       = regexp.MustCompile(`(?i)^bank\d+$`)
	sharedBankRe = regexp.MustCompile(`(?i)^sharedbank\d+$`)
)

// classifySlot maps an inventory Location to its category + canonical slot key. It
// splits the Location on the FIRST '-' into (parent, suffix); the PARENT token decides
// the category (a "General4-Slot1" child is general, a "Bank1-Slot1" child is bank, a
// "Head-Slot1" augment is equipment). Comparison is case-insensitive (A5 / Pitfall 5);
// the emitted canonical key is Title-case.
//
//	^[Gg]eneral\d+   → (general,   canonical "GeneralN")
//	^sharedbank\d+   → (bank,      canonical "SharedBankN") — account-wide shared bank
//	^[Bb]ank\d+      → (bank,      canonical "BankN")
//	known equip tok  → (equipment, the canonical Title-case token)
//	doubled-slot base→ (equipment, the canonical PREFIX "Ear"/"Finger"/"Wrist" — the
//	                    occurrence is numbered later in buildStructuredInventory)
//	anything else    → (general,   the raw parent) — defensive default, never panics (T-29-05)
func classifySlot(location string) (SlotCategory, string) {
	parent := location
	if i := strings.IndexByte(location, '-'); i >= 0 {
		parent = location[:i]
	}

	switch {
	case generalRe.MatchString(parent):
		return SlotGeneral, canonicalNumbered("General", parent)
	case sharedBankRe.MatchString(parent):
		return SlotBank, canonicalNumbered("SharedBank", parent)
	case bankRe.MatchString(parent):
		return SlotBank, canonicalNumbered("Bank", parent)
	}

	if canonical, ok := equipmentSlotsLC[strings.ToLower(parent)]; ok {
		return SlotEquipment, canonical
	}
	// Doubled equipment slot written as its BASE token ("Ear"/"Fingers"/"Wrist", both of a
	// pair) → equipment with the canonical PREFIX; buildStructuredInventory numbers the two
	// occurrences into Ear1/Ear2, Finger1/Finger2, Wrist1/Wrist2 (slotconst.go pairedBaseSlots).
	if prefix, ok := pairedBaseSlots[strings.ToLower(parent)]; ok {
		return SlotEquipment, prefix
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

// numberPairedSlot turns a DOUBLED-equipment base canonical ("Ear"/"Finger"/"Wrist") into
// its occurrence-numbered canonical — the 1st becomes "<base>1", the 2nd "<base>2" — so each
// of a paired slot renders in its own paperdoll position (and the two same-named rows no
// longer collide on an identical Location downstream). Any already-complete canonical
// (e.g. "Head", "Ear1", "General4") passes through unchanged. seq is the per-character
// occurrence counter (base → count); the caller seeds one fresh map per inventory.
func numberPairedSlot(canonical string, seq map[string]int) string {
	switch canonical {
	case "Ear", "Finger", "Wrist":
		seq[canonical]++
		return canonical + strconv.Itoa(seq[canonical])
	}
	return canonical
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
// CR-01 robustness — never retain a pointer into a slice that is later appended to. The
// nesting is resolved entirely via a side map keyed by parent Location (childrenByParent),
// never via element pointers into inv.Equipment/General/Bank. We do NOT touch the group
// slices after Pass A; orphan/grandchild rows that must flatten to top-level are collected
// into separate slices and appended to the groups only at the very END, after every
// Children attachment is done — so no append can ever dangle a parent we are still writing.
//
// Three steps: (1) classify every row, recording top-level containers and routing
// *-Slot* children into childrenByParent (or the orphan slices); (2) attach the collected
// children onto their top-level container by stable Location key; (3) append the flattened
// orphans onto General/Bank last. Augments on equipment (Head-Slot1, A3) are
// flattened/ignored for the INV-05 paperdoll window. A grandchild / orphan (a *-Slot* whose
// parent is itself a child or absent — A2) is slog.Warn'd (op + the two Locations only,
// never item content — V7) and flattened to top-level rather than panicking (T-29-05).
func buildStructuredInventory(char string, rows []store.InventoryRow) CharacterInventory {
	inv := CharacterInventory{Char: char}

	// LastSeen (the examine "Last synced", D-08 #12) is the per-CHARACTER upload freshness
	// (character.last_seen) — the SAME value on every row, so the first row is sufficient
	// and stable. It is DISTINCT from a slot's per-item LastListed (the price last-listed
	// date) — Pitfall 2; do NOT cross the two. "" when there are no rows (never synced).
	if len(rows) > 0 {
		inv.LastSeen = rows[0].LastSeen
	}

	// Build every slot once, remembering which are top-level containers (by raw Location)
	// so children can find their parent.
	type indexed struct {
		slot     InventorySlot
		category SlotCategory
		isChild  bool   // has a "-Slot<N>" suffix
		parent   string // the parent container Location for a child
	}
	all := make([]indexed, 0, len(rows))
	topLevel := make(map[string]bool, len(rows)) // raw Location → is a top-level (non-child) slot
	pairSeq := make(map[string]int)              // canonical prefix → occurrences, numbering the doubled equipment slots

	for _, row := range rows {
		// The "Held" slot is the in-game CURSOR (a transient item being carried), not stored
		// inventory — exclude it from the window entirely (it would otherwise classify to the
		// general default and render as a loose/empty tile). Covers a bare "Held" and any
		// "Held-Slot<N>" (a bag held on the cursor). 2026-06-18.
		if isHeldCursor(row.Location) {
			continue
		}

		cat, canonical := classifySlot(row.Location)

		parent, isChild := splitChild(row.Location)
		if !isChild {
			topLevel[row.Location] = true
			// Number a doubled-slot base (Ear/Finger/Wrist → Ear1/Ear2, …) by occurrence.
			// Only TOP-LEVEL equipment rows count; an augment child (e.g. "Ear-Slot1") is
			// flattened later (it never reaches the paperdoll) and must not consume an ordinal.
			if cat == SlotEquipment {
				canonical = numberPairedSlot(canonical, pairSeq)
			}
		}
		slot := slotFromRow(row, cat, canonical)
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

	// Pass B: route every child WITHOUT touching the group slices or holding any pointer
	// into them. Real children accumulate in childrenByParent (keyed by parent Location, a
	// stable string — immune to slice reallocation); orphans/grandchildren accumulate in
	// the flatten slices for an end-of-function append.
	childrenByParent := make(map[string][]InventorySlot)
	var orphanGeneral, orphanBank []InventorySlot
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
		if !topLevel[it.parent] {
			// Grandchild or orphan (A2 — bags-in-bags don't exist in classic EQ). Flatten to
			// top-level rather than panic; log op + the two Locations only (never content, V7).
			slog.Warn("compute.structured_inventory.orphan_child",
				"op", "buildStructuredInventory", "child", it.slot.Location, "parent", it.parent)
			switch it.category {
			case SlotBank:
				orphanBank = append(orphanBank, it.slot)
			default:
				orphanGeneral = append(orphanGeneral, it.slot)
			}
			continue
		}
		childrenByParent[it.parent] = append(childrenByParent[it.parent], it.slot)
	}

	// Step 2: attach the collected children onto their container by stable Location key.
	// We index by Location now (the group slices will NOT grow again until the orphan
	// append below, which is the last mutation), and write Children straight onto the
	// element — no retained cross-step pointer that a later append could invalidate.
	for _, group := range []*[]InventorySlot{&inv.Equipment, &inv.General, &inv.Bank} {
		for i := range *group {
			if kids, ok := childrenByParent[(*group)[i].Location]; ok {
				(*group)[i].Children = kids
			}
		}
	}

	// Step 3: append the flattened orphans LAST — after every Children write is done, so
	// this final (re)allocation cannot dangle a parent we still need to mutate.
	inv.General = append(inv.General, orphanGeneral...)
	inv.Bank = append(inv.Bank, orphanBank...)

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
		IconID:        row.IconID,     // id-joined item_master.icon_id; 0 = no icon yet (INV-04, D-02)
		Statsblock:    row.Statsblock, // id-joined item_master.statsblock; "" = no stats yet (INV-02)
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

// isHeldCursor reports whether a Location is the in-game CURSOR slot ("Held", or a
// "Held-Slot<N>" for a bag carried on the cursor) — transient, not stored inventory, so
// buildStructuredInventory drops it from the window. Split on the FIRST '-' so the bag case
// is caught; compared case-insensitively (A5), mirroring the other slot matchers.
func isHeldCursor(location string) bool {
	parent := location
	if i := strings.IndexByte(location, '-'); i >= 0 {
		parent = location[:i]
	}
	return strings.EqualFold(parent, "Held")
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
//
// MR-02: PerBank is seeded from the bank-toon list FIRST, so a coin-only bank toon (plat
// entered but no inventory_item rows yet — freshly flagged, mid-upload, or emptied) still
// gets a zero-Valuation PerBank entry. Otherwise its platinum would land in TotalPlatinum
// (via ListBankToons) while its per-bank row silently vanished, so a consumer iterating
// PerBank to render per-bank lines would drop that toon even though the guild total counts
// it. The valuation scope (item rows) and the platinum scope (bank toons) now agree on the
// set of bank toons. Display-vs-valuation note: the structured INV-05 model
// (buildStructuredInventory) deliberately drops augments + re-homes orphans for the
// paperdoll, but valuation here sums the FLAT InventoryJoin row list (every real
// inventory_item row, augments included) — the two are intentionally different scopes
// (Pitfall 3); a per-character bank value must use this flat list, never the display tree.
func buildBankValuation(rows []store.InventoryJoinRow, toons []store.BankToon) BankValuation {
	bv := BankValuation{PerBank: make(map[string]Valuation, len(toons))}
	for _, t := range toons {
		bv.PerBank[t.Name] = Valuation{} // ensure every live bank toon has a row (MR-02)
	}
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
