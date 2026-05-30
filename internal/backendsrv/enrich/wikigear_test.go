package enrich

import (
	"strings"
	"testing"
)

func TestParseGearTierPage_PreRaid(t *testing.T) {
	wikitext, _ := loadWikitext(t, "wiki-velious-preraid-gear")
	rows, err := ParseGearTierPage(wikitext, TierVeliousPreRaid)
	if err != nil {
		t.Fatalf("ParseGearTierPage: %v", err)
	}
	// TS: classCount=14, itemCount>400.
	distinctClasses := map[string]bool{}
	for _, r := range rows {
		distinctClasses[r.Class] = true
	}
	if len(distinctClasses) != 14 {
		t.Fatalf("distinct classes = %d, want 14", len(distinctClasses))
	}
	if len(rows) <= 400 {
		t.Fatalf("item count = %d, want > 400", len(rows))
	}

	// item_id is ALWAYS nil (wiki transclusions have no IDs).
	for _, r := range rows {
		if r.ItemID != nil {
			t.Fatalf("row %q has non-nil ItemID", r.ItemName)
		}
	}

	// Iksar tagging: >= 4 Iksar-tagged rows; every Iksar row's name starts
	// "Iksar "; no Pre-Raid row starts "Iksar ".
	iksarCount := 0
	for _, r := range rows {
		if r.Tier == TierIksar {
			iksarCount++
			if !strings.HasPrefix(r.ItemName, "Iksar ") {
				t.Fatalf("Iksar-tagged row %q does not start with 'Iksar '", r.ItemName)
			}
		}
		if r.Tier == TierVeliousPreRaid && strings.HasPrefix(r.ItemName, "Iksar ") {
			t.Fatalf("Pre-Raid row %q wrongly NOT tagged Iksar", r.ItemName)
		}
	}
	if iksarCount < 4 {
		t.Fatalf("iksar count = %d, want >= 4", iksarCount)
	}

	// "Iksar Hide Cap" appears on Cleric + Magician, both tier=Iksar.
	var ikcClasses []string
	for _, r := range rows {
		if r.ItemName == "Iksar Hide Cap" {
			if r.Tier != TierIksar {
				t.Fatalf("Iksar Hide Cap tier = %q, want Iksar", r.Tier)
			}
			ikcClasses = append(ikcClasses, r.Class)
		}
	}
	if len(ikcClasses) != 2 {
		t.Fatalf("Iksar Hide Cap appears %d times, want 2 (CLR+MAG)", len(ikcClasses))
	}
	hasCLR, hasMAG := false, false
	for _, c := range ikcClasses {
		if c == "CLR" {
			hasCLR = true
		}
		if c == "MAG" {
			hasMAG = true
		}
	}
	if !hasCLR || !hasMAG {
		t.Fatalf("Iksar Hide Cap classes = %v, want CLR+MAG", ikcClasses)
	}

	// Rank is 1-based and ascending within a (class,tier,slot) group.
	type key struct {
		class string
		tier  Tier
		slot  string
	}
	groups := map[key][]int{}
	for _, r := range rows {
		k := key{r.Class, r.Tier, r.Slot}
		groups[k] = append(groups[k], r.Rank)
	}
	foundMulti := false
	for _, ranks := range groups {
		if len(ranks) >= 2 {
			foundMulti = true
			if ranks[0] != 1 {
				t.Fatalf("group first rank = %d, want 1", ranks[0])
			}
			for i := 1; i < len(ranks); i++ {
				if ranks[i] <= ranks[i-1] {
					t.Fatalf("ranks not ascending: %v", ranks)
				}
			}
		}
	}
	if !foundMulti {
		t.Fatal("no multi-item slot group found to verify rank ordering")
	}
}

func TestParseGearTierPage_Raiding(t *testing.T) {
	wikitext, _ := loadWikitext(t, "wiki-velious-raiding-gear")
	rows, err := ParseGearTierPage(wikitext, TierVeliousRaiding)
	if err != nil {
		t.Fatalf("ParseGearTierPage: %v", err)
	}
	distinctClasses := map[string]bool{}
	for _, r := range rows {
		distinctClasses[r.Class] = true
		// Every row is tier="Velious Raiding"; no Iksar on the Raiding page.
		if r.Tier != TierVeliousRaiding {
			t.Fatalf("row tier = %q, want Velious Raiding", r.Tier)
		}
		if r.ItemID != nil {
			t.Fatalf("row %q has non-nil ItemID", r.ItemName)
		}
	}
	if len(distinctClasses) != 14 {
		t.Fatalf("distinct classes = %d, want 14", len(distinctClasses))
	}
	if len(rows) <= 400 {
		t.Fatalf("item count = %d, want > 400", len(rows))
	}
}

// TestParseGearTierPage_Synthetic replicates the synthetic edge cases from the
// TS gear-tier test: parenthetical stripping, unknown-slot still emitted, Iksar
// only on Pre-Raid, noise-class skipping, and unclosed <li> handling.
func TestParseGearTierPage_Synthetic(t *testing.T) {
	pad := strings.Repeat("x", 200)

	// Parenthetical notes stripped from item names.
	wt := pad + `

== [[Monk]] ==
<ul><li> '''Head''' - {{:Whetstone (Worn)}}, {{:Plain Helm}} (rare)
</li></ul>
`
	rows, _ := ParseGearTierPage(wt, TierVeliousPreRaid)
	names := gearNameSet(rows)
	if !names["Whetstone"] || !names["Plain Helm"] || names["Whetstone (Worn)"] {
		t.Fatalf("paren strip names = %v, want Whetstone + Plain Helm (no '(Worn)')", names)
	}

	// Unknown slot label: item still emitted, slot preserved.
	wtUnknown := pad + `

== [[Monk]] ==
<ul><li> '''GreaterEars''' - {{:Mystery Earring}}
</li><li> '''Head''' - {{:Real Helm}}
</li></ul>
`
	uRows, _ := ParseGearTierPage(wtUnknown, TierVeliousPreRaid)
	var myst *WikiGearTierRow
	for i := range uRows {
		if uRows[i].ItemName == "Mystery Earring" {
			myst = &uRows[i]
		}
	}
	if myst == nil || myst.Slot != "GreaterEars" {
		t.Fatalf("unknown-slot item = %+v, want slot GreaterEars", myst)
	}

	// Iksar tagging fires only on Pre-Raid, NOT on Raiding.
	wtIksar := pad + `

== [[Monk]] ==
<ul><li> '''Head''' - {{:Iksar Hide Cap}} </li></ul>
`
	rRows, _ := ParseGearTierPage(wtIksar, TierVeliousRaiding)
	if len(rRows) != 1 || rRows[0].Tier != TierVeliousRaiding || rRows[0].ItemName != "Iksar Hide Cap" {
		t.Fatalf("Iksar-on-Raiding = %+v, want one {Velious Raiding, Iksar Hide Cap}", rRows)
	}

	// Noise class (not in CLASS_DISPLAY_TO_ABBREV) is skipped.
	wtNoise := pad + `

== [[Foo Class]] ==
<ul><li> '''Head''' - {{:Should Not Emit}} </li></ul>

== [[Monk]] ==
<ul><li> '''Head''' - {{:Real Item}} </li></ul>
`
	nRows, _ := ParseGearTierPage(wtNoise, TierVeliousPreRaid)
	nNames := gearNameSet(nRows)
	if !nNames["Real Item"] || nNames["Should Not Emit"] {
		t.Fatalf("noise-class names = %v, want Real Item only", nNames)
	}

	// Unclosed <li> tags (wiki sometimes skips the closing tag).
	wtUnclosed := pad + `

== [[Monk]] ==
<ul><li> '''Head''' - {{:Helm A}}
<li> '''Chest''' - {{:Plate B}}
</ul>
`
	ucRows, _ := ParseGearTierPage(wtUnclosed, TierVeliousPreRaid)
	ucNames := gearNameSet(ucRows)
	if !ucNames["Helm A"] || !ucNames["Plate B"] {
		t.Fatalf("unclosed-li names = %v, want Helm A + Plate B", ucNames)
	}

	// Empty / no-class-section input → empty slice, nil error.
	if r, err := ParseGearTierPage("", TierVeliousPreRaid); err != nil || len(r) != 0 {
		t.Fatalf("empty input: rows=%d err=%v, want 0/nil", len(r), err)
	}
	wtNoClass := strings.Repeat("y", 500) + "\n\n== Generic Header ==\nrandom content\n"
	if r, err := ParseGearTierPage(wtNoClass, TierVeliousPreRaid); err != nil || len(r) != 0 {
		t.Fatalf("no-class-section input: rows=%d err=%v, want 0/nil", len(r), err)
	}
}

func gearNameSet(rows []WikiGearTierRow) map[string]bool {
	out := map[string]bool{}
	for _, r := range rows {
		out[r.ItemName] = true
	}
	return out
}
