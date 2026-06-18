package compute

// slotconst.go holds the canonical EQ paperdoll equipment-slot set + a
// case-insensitive lookup index, for the INV-05 slot classifier (classifySlot,
// inventory.go). It mirrors the dependency-free var-map shape of eqconst.go.
//
// This is a NEW, inventory-Location-native table — deliberately DISTINCT from the
// UPPERCASE wiki-vocab slot map in enrich/eqconst.go:65-83. That map is wiki-vocab-keyed
// and emits UPPERCASE tokens ("EAR1","FINGER1","HEAD") that do NOT match the Title-case
// inventory_item.location tokens ("Ear1","Finger1","Head") — so reusing it as the
// inventory classifier would silently leave every equipment row unclassified
// (29-RESEARCH Pitfall 5 / Landmine 5). The classifier here compares case-insensitively
// (robust to whatever case live data uses — A5) but EMITS the canonical Title-case key
// the web expects.
//
// Source of the slot vocabulary: the /outputfile inventory Location tokens
// (internal/parse/testdata/sample-inventory.txt:20-40, Title-case). Ear1/Ear2 are
// INCLUDED even though the synthetic fixture omits them — real dumps carry two ear
// slots (29-RESEARCH A4).

import "strings"

// SlotCategory classifies an inventory Location into the three INV-05 buckets.
type SlotCategory string

const (
	SlotEquipment SlotCategory = "equipment"
	SlotGeneral   SlotCategory = "general"
	SlotBank      SlotCategory = "bank"
)

// equipmentSlots is the canonical EQ paperdoll slot set, in Title-case to match the
// inventory_item.location tokens. Includes Ear1/Ear2 even though the synthetic
// fixture omits them (A4). Source: /outputfile inventory Location vocabulary
// (sample-inventory.txt:20-40).
var equipmentSlots = map[string]bool{
	"Charm": true, "Head": true, "Face": true, "Ear1": true, "Ear2": true, "Neck": true,
	"Shoulders": true, "Arms": true, "Back": true, "Wrist1": true, "Wrist2": true,
	"Range": true, "Hands": true, "Primary": true, "Secondary": true, "Finger1": true,
	"Finger2": true, "Chest": true, "Legs": true, "Feet": true, "Waist": true,
	"Power": true, "Ammo": true,
}

// equipmentSlotsLC maps lower(token) → the canonical Title-case token, so the
// classifier accepts whatever case live data uses (A5) while still EMITTING the
// canonical key. Built once at package init from equipmentSlots (the single source
// of truth) so the two never drift.
var equipmentSlotsLC = buildEquipmentSlotsLC()

func buildEquipmentSlotsLC() map[string]string {
	out := make(map[string]string, len(equipmentSlots))
	for slot := range equipmentSlots {
		out[strings.ToLower(slot)] = slot
	}
	return out
}
