package compute

// wishlist.go is the Phase 34 (WISH-02/03/04) COMPUTE TRANSFORM LAYER for the
// per-character / per-slot upgrade wishlist. Like inventory.go it authors ZERO SQL —
// WishlistFor composes typed store reads (StructuredInventory, ListOwnWishlist,
// GearTierPrices, PriceByName, AlertedWishlistIDs, CharsWithMeta) and hands them to a
// PURE buildWishlistView so the slot-bridge + suggestion filter + auto-removal + the
// name-keyed target price are directly table-testable without a DB (the
// StructuredInventory/buildStructuredInventory split).
//
// Three genuinely-novel pieces, each kept pure + unit-tested:
//   - wikiSlotFor: the canonical-worn-slot → wiki-prose-slot bridge (Pitfall 2, the
//     HIGHEST-RISK spot — three vocabularies coexist; get it wrong and EVERY suggestion
//     list silently empties). Built ONCE by inverting enrich.WIKI_SLOT_TO_INV_SLOTS.
//   - auto-removal (D-02): a held-name set from StructuredInventory hides any target
//     whose normalized name the char holds ANYWHERE (equipment+general+bank+children).
//   - the target price: resolved by NORMALIZED NAME against the WHOLE pigparse_price
//     catalog (store.PriceByName) — the same name-bridge the examine uses — NOT just the
//     gear-tier slice (WARNING-3); nil ONLY when no catalog price genuinely exists.
//
// The "Raid" tag is the TIER, not a column (Pitfall 3): IsRaid = (row.Tier ==
// enrich.TierVeliousRaiding). There is no no_drop column — do not invent one.

import (
	"context"
	"strings"

	"github.com/boejowen/SquireBot/internal/backendsrv/enrich"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// wornSlots is the fixed 21-worn-slot paperdoll order (D-04): LEFT, then RIGHT, then
// WORN — the SAME taxonomy as the P31 InventoryWindow (Charm/Power Source omitted,
// post-Velious). Every slot gets a wishlist + suggestions, even an empty one.
var wornSlots = []string{
	"Head", "Face", "Ear1", "Ear2", "Neck", "Shoulders", "Arms", "Back",
	"Wrist1", "Wrist2", "Hands", "Finger1", "Finger2", "Chest", "Legs", "Feet", "Waist",
	"Primary", "Secondary", "Range", "Ammo",
}

// invSlotToWiki is the inverse of enrich.WIKI_SLOT_TO_INV_SLOTS: an UPPERCASE inventory
// token ("FINGER1") → its wiki-prose slot ("Fingers"). Built ONCE at package init by
// inverting + case-folding the canonical map. A canonical worn-slot upper-cases to its
// UPPERCASE token to look up the wiki slot (Pitfall 2). Ammo/Charm/Power have no key in
// WIKI_SLOT_TO_INV_SLOTS → they map to "" → empty suggestion list (correct, A5).
var invSlotToWiki = buildInvSlotToWiki()

// buildInvSlotToWiki inverts enrich.WIKI_SLOT_TO_INV_SLOTS (wiki-prose → UPPERCASE inv
// tokens) into UPPERCASE inv token → wiki-prose slot. Each value token (already
// UPPERCASE in the source map) is upper-cased defensively so the lookup is exact.
func buildInvSlotToWiki() map[string]string {
	out := make(map[string]string)
	for wiki, invTokens := range enrich.WIKI_SLOT_TO_INV_SLOTS {
		for _, tok := range invTokens {
			out[strings.ToUpper(tok)] = wiki
		}
	}
	return out
}

// wikiSlotFor maps a canonical worn-slot ("Finger1"/"Head"/…) to its wiki-prose gear-
// tier slot ("Fingers"/"Head"/…). Returns "" for a slot with no gear-tier vocabulary
// (Ammo/Charm/Power) → the caller emits an empty suggestion list. This is the load-
// bearing bridge: "Finger1" AND "Finger2" BOTH map to "Fingers" (Pitfall 2).
func wikiSlotFor(canonical string) string {
	return invSlotToWiki[strings.ToUpper(canonical)]
}

// norm normalizes an item name for the cross-namespace name join (the pp_rep
// convention: lower(trim(name))). Used for both the auto-removal held-set membership
// and the PriceByName catalog lookup.
func norm(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// WishlistFor builds the per-character wishlist view (WISH-02/03/04). It composes the
// store reads (equipped item per slot, the viewer's active targets, the gear-tier
// suggestions, the full catalog price map, the EC-hit badge set, the char's class) and
// hands them to the pure buildWishlistView. discordID is the SESSION caller (owner-
// scoped targets + badge); char is the selected character.
func WishlistFor(ctx context.Context, s *store.Store, discordID, char string) (WishlistView, error) {
	inv, err := StructuredInventory(ctx, s, char) // equipped item per slot + the held set
	if err != nil {
		return WishlistView{}, err
	}
	targets, err := store.ListOwnWishlist(ctx, s.DB(), discordID, char) // active per-slot targets
	if err != nil {
		return WishlistView{}, err
	}
	tiers, err := s.GearTierPrices(ctx) // every gear-tier row + name-keyed price (suggestions)
	if err != nil {
		return WishlistView{}, err
	}
	prices, err := s.PriceByName(ctx) // FULL catalog name→price map (target price, WARNING-3)
	if err != nil {
		return WishlistView{}, err
	}
	alerted, err := store.AlertedWishlistIDs(ctx, s.DB(), discordID) // EC-hit badge set
	if err != nil {
		return WishlistView{}, err
	}
	metas, err := s.CharsWithMeta(ctx) // find this char's Class for the suggestion filter
	if err != nil {
		return WishlistView{}, err
	}

	var class string
	for _, m := range metas {
		if m.Name == char {
			class = m.Class
			break
		}
	}

	return buildWishlistView(char, inv, targets, tiers, prices, alerted, class), nil
}

// buildWishlistView is the PURE transform (no ctx/store) — directly table-testable. For
// each of the 21 worn slots it emits the equipped item + the viewer's active targets
// (auto-removal applied — a target whose normalized name the char holds ANYWHERE is
// HIDDEN, D-02) + the class+slot gear-tier suggestions. The target price/last-listed
// resolves by normalized name against the FULL catalog (prices), the suggestion
// price/last-listed comes from the name-keyed gear-tier row.
func buildWishlistView(
	char string,
	inv CharacterInventory,
	targets []store.WishlistTargetRow,
	tiers []store.GearTierPriceRow,
	prices map[string]store.PriceByNameRow,
	alerted map[int64]bool,
	class string,
) WishlistView {
	// Auto-removal held-set (D-02): every normalized name the char holds ANYWHERE
	// (equipment + general + bank + each container's children).
	held := make(map[string]bool)
	addHeld := func(slots []InventorySlot) {
		for _, sl := range slots {
			if sl.Item != "" {
				held[norm(sl.Item)] = true
			}
			for _, ch := range sl.Children {
				if ch.Item != "" {
					held[norm(ch.Item)] = true
				}
			}
		}
	}
	addHeld(inv.Equipment)
	addHeld(inv.General)
	addHeld(inv.Bank)

	// Index the equipped item per canonical worn-slot (Item is "" for an empty slot).
	equippedBySlot := make(map[string]string, len(inv.Equipment))
	for _, sl := range inv.Equipment {
		equippedBySlot[sl.CanonicalSlot] = sl.Item
	}

	out := WishlistView{Char: char, Slots: make([]WishlistSlot, 0, len(wornSlots))}
	for _, canonical := range wornSlots {
		ws := WishlistSlot{
			Slot:        canonical,
			Equipped:    equippedBySlot[canonical],
			Targets:     make([]WishlistTarget, 0),
			Suggestions: make([]WishlistSuggestion, 0),
		}

		// Targets for this slot, auto-removal applied (hide a held target — D-02).
		for _, tr := range targets {
			if tr.Slot != canonical {
				continue
			}
			if held[norm(tr.ItemName)] {
				continue // auto-hide (not deleted; the view self-heals if it later leaves)
			}
			t := WishlistTarget{
				ID:        tr.ID,
				ItemID:    tr.ItemID,
				ItemName:  tr.ItemName,
				Pinged:    tr.Pinged,
				PingedHit: alerted[tr.ID],
			}
			// Target price/last-listed by NORMALIZED NAME against the FULL catalog
			// (WARNING-3) — the same source the examine uses, NOT just gear-tier rows.
			if pr, ok := prices[norm(tr.ItemName)]; ok {
				t.Price = pickPrice([]PriceDetail{{Direction: pr.Direction, A30: pr.A30, T30: pr.T30}})
				t.LastListed = pr.LastListed
			}
			ws.Targets = append(ws.Targets, t)
		}

		// Suggestions: the class+slot gear-tier rows (empty when the slot has no wiki
		// gear-tier vocabulary — Ammo/Charm/Power → wikiSlotFor == "").
		if wiki := wikiSlotFor(canonical); wiki != "" {
			for _, row := range tiers {
				if row.Class != class || row.Slot != wiki {
					continue
				}
				sug := WishlistSuggestion{
					ItemName:   row.ItemName,
					IsRaid:     row.Tier == string(enrich.TierVeliousRaiding),
					LastListed: row.LastListed,
				}
				if row.HasPrice {
					sug.Price = pickPrice([]PriceDetail{{Direction: row.Direction, A30: row.A30, T30: row.T30}})
					sug.LastListed = row.LastListed
				}
				ws.Suggestions = append(ws.Suggestions, sug)
			}
		}

		out.Slots = append(out.Slots, ws)
	}
	return out
}
