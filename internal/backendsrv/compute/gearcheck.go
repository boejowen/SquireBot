package compute

// gearcheck.go is the Go reimplementation of apps-script/src/tabs/buildGearCheck.ts
// — the consolidated `gear_check` grid and one of the two WEB-02 parity hearts.
// Per character (with a class set, + race), it reads the Velious gear-tier
// recommendations for the relevant tiers and emits OK/OTHER/MISSING per
// (char, tier, slot, recommendation), exactly matching the v1 semantics.
//
// Tiers shown: 'Velious Pre-Raid/Group' + 'Velious Raiding' always; 'Iksar' iff
// race == 'IKS' (buildGearCheck.ts:87-89).
//
// Slot-pair match (the subtle part, buildGearCheck.ts:101-125): wiki uses prose
// slots ('Ears'/'Fingers'/'Wrists'/...); inv uses tokens (EAR1+EAR2/...).
// enrich.WIKI_SLOT_TO_INV_SLOTS maps each wiki slot to one-or-two inv slots; a
// char is OK if the recommended item (case-insensitive name match) is in EITHER
// slot of the pair. The ORDER of the three status branches is load-bearing:
// matched → OK; else slot non-empty → OTHER (Have = first item in slot); else
// → MISSING. enrich.WIKI_SLOT_TO_INV_SLOTS is REUSED (not re-typed) from the
// sibling enrich package.

import (
	"context"
	"sort"
	"strings"

	"github.com/boejowen/SquireBot/internal/backendsrv/enrich"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// alwaysShownTiers are the two Velious tiers every classed character sees
// (buildGearCheck.ts:87). "Iksar" is appended only for race == "IKS".
var alwaysShownTiers = []string{"Velious Pre-Raid/Group", "Velious Raiding"}

const iksarTier = "Iksar"
const iksarRace = "IKS"

// GearCheck computes the consolidated `gear_check` grid over the store. It mirrors
// buildGearCheck.ts: characters with a class set get OK/OTHER/MISSING rows for
// each (tier, slot, recommendation) of their relevant Velious tiers; output is
// sorted Char asc → tier rank → slot asc → recommended asc.
func GearCheck(ctx context.Context, s *store.Store) ([]GearCheckRow, error) {
	chars, err := s.CharsWithMeta(ctx)
	if err != nil {
		return nil, err
	}
	tiers, err := s.WikiGearTiers(ctx)
	if err != nil {
		return nil, err
	}
	invByChar, err := s.InventoryByChar(ctx)
	if err != nil {
		return nil, err
	}
	return buildGearCheckRows(chars, tiers, invByChar), nil
}

// gearKey groups recommendations by (tier, class).
type gearKey struct {
	tier  string
	class string
}

// buildGearCheckRows is the pure transform (no store access) shared with the
// parity tests. It groups the flat tier rows by (tier, class) → slot →
// recommendations, then walks each classed character's relevant tiers.
func buildGearCheckRows(chars []store.CharMeta, tiers []store.WikiGearTierRow, invByChar map[string][]store.InvSlotItem) []GearCheckRow {
	// Group recommendations by (tier, class) → slot → []itemName (recommended).
	byTierClassSlot := make(map[gearKey]map[string][]string)
	for _, g := range tiers {
		// Mirror v1 readWikiGearByTierClass: skip rows missing tier/class/slot/item.
		if g.Tier == "" || g.Class == "" || g.Slot == "" || g.ItemName == "" {
			continue
		}
		key := gearKey{tier: g.Tier, class: g.Class}
		bySlot := byTierClassSlot[key]
		if bySlot == nil {
			bySlot = make(map[string][]string)
			byTierClassSlot[key] = bySlot
		}
		bySlot[g.Slot] = append(bySlot[g.Slot], g.ItemName)
	}

	var out []GearCheckRow
	for _, c := range chars {
		if c.Class == "" {
			continue // char without metadata is skipped (buildGearCheck.ts:84)
		}

		tiersToShow := make([]string, len(alwaysShownTiers))
		copy(tiersToShow, alwaysShownTiers)
		if c.Race == iksarRace {
			tiersToShow = append(tiersToShow, iksarTier)
		}

		charItems := invByChar[c.Name]
		for _, tier := range tiersToShow {
			bySlot := byTierClassSlot[gearKey{tier: tier, class: c.Class}]
			if bySlot == nil {
				continue
			}
			for slot, recommendations := range bySlot {
				invSlots := enrich.WIKI_SLOT_TO_INV_SLOTS[slot]
				charItemsInSlots := itemsInSlots(charItems, invSlots)
				for _, rec := range recommendations {
					status, have := matchRecommendation(rec, charItemsInSlots)
					out = append(out, GearCheckRow{
						Char:        c.Name,
						Class:       c.Class,
						Tier:        tier,
						Slot:        slot,
						Have:        have,
						Recommended: rec,
						Status:      status,
					})
				}
			}
		}
	}

	sortGearCheckRows(out)
	return out
}

// itemsInSlots returns the character's items whose location is one of invSlots
// (the one-or-two inv slot tokens for a wiki slot). Preserves charItems order so
// "first item in slot" (the OTHER Have value) is deterministic per char.
func itemsInSlots(charItems []store.InvSlotItem, invSlots []string) []store.InvSlotItem {
	if len(invSlots) == 0 {
		return nil
	}
	var out []store.InvSlotItem
	for _, it := range charItems {
		for _, slot := range invSlots {
			if it.Location == slot {
				out = append(out, it)
				break
			}
		}
	}
	return out
}

// matchRecommendation ports the load-bearing 3-branch status logic
// (buildGearCheck.ts:105-123): matched (case-insensitive name equals a recommended
// item in EITHER inv slot of the pair) → OK; else slot non-empty → OTHER (Have =
// first item in slot); else → MISSING (Have = "").
func matchRecommendation(rec string, charItemsInSlots []store.InvSlotItem) (status, have string) {
	for _, it := range charItemsInSlots {
		if strings.EqualFold(it.ItemName, rec) {
			return "OK", it.ItemName
		}
	}
	if len(charItemsInSlots) > 0 {
		return "OTHER", charItemsInSlots[0].ItemName
	}
	return "MISSING", ""
}

// sortGearCheckRows sorts Char asc → tier rank (Pre-Raid=1/Raiding=2/Iksar=3,
// unknown=999) → slot asc → recommended asc (buildGearCheck.ts:131-141).
func sortGearCheckRows(rows []GearCheckRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Char != b.Char {
			return a.Char < b.Char
		}
		ra, rb := tierRankOrDefault(a.Tier), tierRankOrDefault(b.Tier)
		if ra != rb {
			return ra < rb
		}
		if a.Slot != b.Slot {
			return a.Slot < b.Slot
		}
		return a.Recommended < b.Recommended
	})
}

func tierRankOrDefault(tier string) int {
	if r, ok := tierRank[tier]; ok {
		return r
	}
	return 999 // unknown tier sorts last (v1 `?? 999`)
}
