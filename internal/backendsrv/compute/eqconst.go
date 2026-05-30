package compute

// eqconst.go holds ONLY the gear-tier sort-rank map — the 3-entry TIER_SORT from
// buildGearCheck.ts:29-33. The slot-pair map (WIKI_SLOT_TO_INV_SLOTS) is NOT
// re-typed here: it already exists, ported verbatim from eq-constants.ts, in
// internal/backendsrv/enrich/eqconst.go, and gearcheck.go references it as
// enrich.WIKI_SLOT_TO_INV_SLOTS (no import cycle — enrich imports nothing from
// compute/store).

// tierRank is the gear_check tier sort order (buildGearCheck.ts TIER_SORT): the
// two always-shown Velious tiers then the Iksar racial tier last, so an Iksar
// character's racial section groups visually at the bottom. An unknown tier sorts
// after these (handled in gearcheck.go's sort with a 999 fallback, matching the
// v1 `?? 999`).
var tierRank = map[string]int{
	"Velious Pre-Raid/Group": 1,
	"Velious Raiding":         2,
	"Iksar":                   3,
}
