package enrich

import (
	"strings"
	"testing"
)

func TestNormalizeSpellName(t *testing.T) {
	// The plan's contract: NormalizeSpellName == lower(trim) (the store's
	// normalized_name join expression), NOT the TS alphanumeric-strip variant.
	cases := []struct{ in, want string }{
		{"  Gate ", "gate"},
		{"Coldlight", "coldlight"},
		{"  Cavorting Bones  ", "cavorting bones"}, // spaces preserved (lower+trim only)
		{"GATE", "gate"},
	}
	for _, c := range cases {
		if got := NormalizeSpellName(c.in); got != c.want {
			t.Errorf("NormalizeSpellName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseClassPage_Necromancer(t *testing.T) {
	wikitext, _ := loadWikitext(t, "wiki-class-necromancer")
	rows, err := ParseClassPage(wikitext, "NEC")
	if err != nil {
		t.Fatalf("ParseClassPage: %v", err)
	}
	// TS: spellCount >= 100, level headers >= 20.
	if len(rows) < 100 {
		t.Fatalf("spell count = %d, want >= 100", len(rows))
	}
	distinctLevels := map[int]bool{}
	for _, r := range rows {
		distinctLevels[r.Level] = true
	}
	if len(distinctLevels) < 20 {
		t.Fatalf("distinct levels = %d, want >= 20", len(distinctLevels))
	}
	// Every row carries the class abbrev.
	for _, r := range rows {
		if r.Class != "NEC" {
			t.Fatalf("row class = %q, want NEC", r.Class)
		}
	}
	// Spot-check known Level-1 spells (TS assertion).
	lvl1 := map[string]bool{}
	lvl1Count := 0
	for _, r := range rows {
		if r.Level == 1 {
			lvl1[r.SpellName] = true
			lvl1Count++
		}
	}
	if lvl1Count <= 5 {
		t.Fatalf("level-1 spell count = %d, want > 5", lvl1Count)
	}
	for _, want := range []string{"Cavorting Bones", "Coldlight", "Disease Cloud"} {
		if !lvl1[want] {
			t.Errorf("level-1 spells missing %q", want)
		}
	}
	// NormalizedName is the store-faithful lower(trim) of the spell_name.
	for _, r := range rows {
		if r.NormalizedName != strings.ToLower(strings.TrimSpace(r.SpellName)) {
			t.Fatalf("NormalizedName(%q) = %q, want lower(trim)", r.SpellName, r.NormalizedName)
		}
	}
}

func TestParseClassPage_Paladin(t *testing.T) {
	wikitext, _ := loadWikitext(t, "wiki-class-paladin")
	rows, err := ParseClassPage(wikitext, "PAL")
	if err != nil {
		t.Fatalf("ParseClassPage: %v", err)
	}
	// TS: spellCount >= 50, level headers >= 15.
	if len(rows) < 50 {
		t.Fatalf("spell count = %d, want >= 50", len(rows))
	}
	distinctLevels := map[int]bool{}
	for _, r := range rows {
		distinctLevels[r.Level] = true
		if r.Class != "PAL" {
			t.Fatalf("row class = %q, want PAL", r.Class)
		}
	}
	if len(distinctLevels) < 15 {
		t.Fatalf("distinct levels = %d, want >= 15", len(distinctLevels))
	}
}

func TestParseClassPage_Warrior(t *testing.T) {
	wikitext, _ := loadWikitext(t, "wiki-class-warrior")
	rows, err := ParseClassPage(wikitext, "WAR")
	if err != nil {
		t.Fatalf("ParseClassPage: %v", err)
	}
	// Degenerate no-spell page → empty slice, NOT an error.
	if len(rows) != 0 {
		t.Fatalf("Warrior rows = %d, want 0", len(rows))
	}
}

// TestParseClassPage_BardInlineLevel replicates the synthetic {{SongRow}}
// inline-level cases from wiki-spell-parser.test.ts verbatim (there is no Bard
// fixture). Proves the header-empty fallback fires and extracts |level=N.
func TestParseClassPage_BardInlineLevel(t *testing.T) {
	pad := strings.Repeat("x", 200)

	// {{Template:SongRow}} with inline level=, no headers (TS asserts 3 rows).
	wt := pad + `
=Description=
The Bard sings songs.

=Songs=
{{Template:SongRow|name=Chant of Battle|level=1|instrument=Percussion}}
{{Template:SongRow|name=Lullaby|level=5|instrument=String}}
{{Template:SongRow
|name=Multi-line Song
|level=12
|instrument=Wind
|description=A test
}}
`
	rows, err := ParseClassPage(wt, "BRD")
	if err != nil {
		t.Fatalf("ParseClassPage: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("song count = %d, want 3", len(rows))
	}
	byName := map[string]int{}
	for _, r := range rows {
		byName[r.SpellName] = r.Level
	}
	if byName["Chant of Battle"] != 1 || byName["Lullaby"] != 5 || byName["Multi-line Song"] != 12 {
		t.Fatalf("levels = %v, want Chant=1 Lullaby=5 Multi-line=12", byName)
	}

	// {{SongRow}} without Template: prefix (TS asserts 1 row, level 8).
	wt2 := pad + `
=Songs=
{{SongRow|name=Plain Form Song|level=8|instrument=String}}
`
	rows2, _ := ParseClassPage(wt2, "BRD")
	if len(rows2) != 1 || rows2[0].SpellName != "Plain Form Song" || rows2[0].Level != 8 {
		t.Fatalf("plain-form song = %+v, want one {Plain Form Song,8}", rows2)
	}

	// Out-of-range levels are skipped (TS asserts only the level-20 song).
	wt3 := pad + `
=Songs=
{{SongRow|name=Real Song|level=20}}
{{SongRow|name=Bogus Level|level=99}}
{{SongRow|name=Negative Level|level=0}}
`
	rows3, _ := ParseClassPage(wt3, "BRD")
	if len(rows3) != 1 || rows3[0].SpellName != "Real Song" {
		t.Fatalf("range-guarded songs = %+v, want one {Real Song}", rows3)
	}

	// Inline fallback fires ONLY when the header pass is empty (TS): a page with
	// a ==Level== header AND a stray SongRow emits only the header spell.
	wt4 := pad + `
==Level 1==
{{SpellRow|name=HeaderSpell}}
=Songs=
{{SongRow|name=ShouldNotAppear|level=10}}
`
	rows4, _ := ParseClassPage(wt4, "CLR")
	if len(rows4) != 1 || rows4[0].SpellName != "HeaderSpell" {
		t.Fatalf("mixed page = %+v, want one {HeaderSpell}", rows4)
	}
}

// TestParseClassPage_TemplateVariants replicates the SpellRow/RadSpellRow/
// RadSpellRow2 + name-not-first + section-termination synthetic cases from the
// TS test, proving the template regex + header-section logic ported faithfully.
func TestParseClassPage_TemplateVariants(t *testing.T) {
	pad := strings.Repeat("x", 200)

	// name= NOT in first position; mixed first/last placement.
	wt := pad + `

==Level 5==
<table>
{{SpellRow
|type=Summon
|name=Reorder-First Test
|description=Whatever
}}
{{SpellRow|description=X|school=Foo|name=Order-Last Test|mana=10}}
</table>

==Level 9==
{{SpellRow|name=Simple First Test}}
`
	rows, _ := ParseClassPage(wt, "TST")
	names := spellNameSet(rows)
	for _, want := range []string{"Reorder-First Test", "Order-Last Test", "Simple First Test"} {
		if !names[want] {
			t.Errorf("variant names missing %q (got %v)", want, names)
		}
	}

	// {{RadSpellRow}} parsed same as {{SpellRow}}; levels from headers.
	wtRad := pad + `

==Level 1==
{{RadSpellRow|name=Frost Bolt|kind=Damage|targ=Single|mana=10}}
{{RadSpellRow|name=Minor Shielding|targ=Self|kind=Buff}}

==Level 4==
{{RadSpellRow|name=Gate|kind=Teleport|targ=Self|mana=70}}
`
	radRows, _ := ParseClassPage(wtRad, "WIZ")
	if len(radRows) != 3 {
		t.Fatalf("RadSpellRow count = %d, want 3", len(radRows))
	}
	lvlOf := map[string]int{}
	for _, r := range radRows {
		lvlOf[r.SpellName] = r.Level
	}
	if lvlOf["Frost Bolt"] != 1 || lvlOf["Gate"] != 4 {
		t.Fatalf("Rad levels = %v, want Frost Bolt=1 Gate=4", lvlOf)
	}

	// {{RadSpellRow2}} (DRU numbered variant).
	wtRad2 := pad + `

==Level 1==
{{RadSpellRow2|name=Burst of Flame|kind=Damage|targ=Single|mana=7}}

==Level 5==
{{RadSpellRow2|name=Skin like Wood|kind=Buff|targ=Self|mana=15}}
`
	rad2Rows, _ := ParseClassPage(wtRad2, "DRU")
	if len(rad2Rows) != 2 {
		t.Fatalf("RadSpellRow2 count = %d, want 2", len(rad2Rows))
	}

	// A Level section is terminated by a subsequent non-Level header.
	wtTerm := pad + `

==Level 1==
{{SpellRow|name=KeepThis}}

==Class Notes==
{{SpellRow|name=ShouldNotEmitBecauseInDifferentSection}}
`
	termRows, _ := ParseClassPage(wtTerm, "NEC")
	if len(termRows) != 1 || termRows[0].SpellName != "KeepThis" {
		t.Fatalf("section termination = %+v, want one {KeepThis}", termRows)
	}
}

func spellNameSet(rows []WikiSpellRow) map[string]bool {
	out := map[string]bool{}
	for _, r := range rows {
		out[r.SpellName] = true
	}
	return out
}
