package compute_test

// spellcheck_test.go proves compute.SpellCheck parity against the v1 vitest oracle
// (apps-script/src/__tests__/buildSpellCheck.test.ts). Each subtest translates the
// v1 seed-array + expected (Char,Level,Spell,Status) tuples into DB rows over
// store.NewTestDB and asserts the same output.
//
// normalized_name parity: in the DB world both spellbook_entry.normalized_name and
// wiki_spells.normalized_name are materialized as lower(trim(name)). The tests
// seed the normalized value via norm() (= lower(trim)) so the join key is the
// exact DB expression — NOT the v1 vitest fixtures' alphanumeric-strip variant
// (which the DB does not use). The "capitalization + whitespace" case still holds
// under lower(trim): "Numb the Dead" and "numb the dead  " both normalize to
// "numb the dead".

import (
	"context"
	"strings"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/compute"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// norm mirrors the DB's normalized_name expression: lower(trim(name)).
func norm(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func countStatus(rows []compute.SpellCheckRow, status string) int {
	n := 0
	for _, r := range rows {
		if r.Status == status {
			n++
		}
	}
	return n
}

// v1: 'happy path: NEC lvl 10 with 5 wiki spells, knows 2 → 2 KNOWN + 3 MISSING'
// (the lvl-12 spell is out of range for a lvl-10 char → 4 in-range rows: 2 KNOWN
// + 2 MISSING).
func TestSpellCheck_HappyPath_KnownMissingAndLevelGate(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	c := seedChar(t, db, "o", "Slampeach", "NEC", 10, "HUM", false)
	seedWikiSpell(t, db, "NEC", 1, "Cavorting Bones", norm("Cavorting Bones"))
	seedWikiSpell(t, db, "NEC", 1, "Coldlight", norm("Coldlight"))
	seedWikiSpell(t, db, "NEC", 4, "Disease Cloud", norm("Disease Cloud"))
	seedWikiSpell(t, db, "NEC", 8, "Locate Corpse", norm("Locate Corpse"))
	seedWikiSpell(t, db, "NEC", 12, "Numb the Dead", norm("Numb the Dead")) // out of range
	seedSpellbook(t, db, c, 1, "Cavorting Bones", norm("Cavorting Bones"))
	seedSpellbook(t, db, c, 1, "Coldlight", norm("Coldlight"))

	rows, err := compute.SpellCheck(ctx, s)
	if err != nil {
		t.Fatalf("SpellCheck: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("got %d rows, want 4 (lvl-12 excluded): %+v", len(rows), rows)
	}
	if k := countStatus(rows, "KNOWN"); k != 2 {
		t.Errorf("KNOWN = %d, want 2", k)
	}
	if m := countStatus(rows, "MISSING"); m != 2 {
		t.Errorf("MISSING = %d, want 2", m)
	}
	// The lvl-12 spell must not appear at all.
	for _, r := range rows {
		if r.Spell == "Numb the Dead" {
			t.Errorf("lvl-12 'Numb the Dead' should be level-gated out: %+v", r)
		}
	}
}

// v1: 'char without metadata (no class) is skipped entirely'
func TestSpellCheck_NoClassSkipped(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	c := seedChar(t, db, "o", "Slampeach", "", 0, "HUM", false) // empty class, level 0
	seedWikiSpell(t, db, "NEC", 1, "Cavorting Bones", norm("Cavorting Bones"))
	seedSpellbook(t, db, c, 1, "Cavorting Bones", norm("Cavorting Bones"))

	rows, err := compute.SpellCheck(ctx, s)
	if err != nil {
		t.Fatalf("SpellCheck: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("got %d rows, want 0 (classless char skipped): %+v", len(rows), rows)
	}
}

// v1: 'char without spell:<char> tab: all spells MISSING'
func TestSpellCheck_NoSpellbookAllMissing(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	seedChar(t, db, "o", "Slampeach", "NEC", 10, "HUM", false)
	seedWikiSpell(t, db, "NEC", 1, "Cavorting Bones", norm("Cavorting Bones"))
	seedWikiSpell(t, db, "NEC", 1, "Coldlight", norm("Coldlight"))
	// no spellbook rows for Slampeach

	rows, err := compute.SpellCheck(ctx, s)
	if err != nil {
		t.Fatalf("SpellCheck: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2: %+v", len(rows), rows)
	}
	for _, r := range rows {
		if r.Status != "MISSING" {
			t.Errorf("row %+v, want MISSING (no spellbook)", r)
		}
	}
}

// v1: 'multiple chars: rows sorted Char asc → Level asc → Spell asc'
func TestSpellCheck_SortOrder(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	seedChar(t, db, "o1", "Bazquux", "PAL", 20, "HUM", false)
	seedChar(t, db, "o2", "Foobar", "NEC", 20, "HUM", false)
	seedWikiSpell(t, db, "NEC", 4, "Disease Cloud", norm("Disease Cloud"))
	seedWikiSpell(t, db, "NEC", 1, "Coldlight", norm("Coldlight"))
	seedWikiSpell(t, db, "NEC", 1, "Cavorting Bones", norm("Cavorting Bones"))
	seedWikiSpell(t, db, "PAL", 9, "Holy Armor", norm("Holy Armor"))
	seedWikiSpell(t, db, "PAL", 9, "Courage", norm("Courage"))

	rows, err := compute.SpellCheck(ctx, s)
	if err != nil {
		t.Fatalf("SpellCheck: %v", err)
	}
	if len(rows) != 5 {
		t.Fatalf("got %d rows, want 5: %+v", len(rows), rows)
	}
	// Bazquux first (char asc), its PAL spells at level 9.
	if rows[0].Char != "Bazquux" || rows[0].Level != 9 {
		t.Errorf("rows[0] = %+v, want Bazquux/level 9", rows[0])
	}
	if rows[1].Char != "Bazquux" {
		t.Errorf("rows[1].Char = %q, want Bazquux", rows[1].Char)
	}
	// Within Bazquux level 9, alpha: Courage before Holy Armor.
	if rows[0].Spell != "Courage" || rows[1].Spell != "Holy Armor" {
		t.Errorf("Bazquux spell order = [%q,%q], want Courage then Holy Armor", rows[0].Spell, rows[1].Spell)
	}
	// Foobar (NEC): lvl 1 first (Cavorting Bones, Coldlight), then lvl 4.
	if rows[2].Char != "Foobar" || rows[2].Level != 1 || rows[2].Spell != "Cavorting Bones" {
		t.Errorf("rows[2] = %+v, want Foobar/1/Cavorting Bones", rows[2])
	}
	if rows[3].Char != "Foobar" || rows[3].Level != 1 || rows[3].Spell != "Coldlight" {
		t.Errorf("rows[3] = %+v, want Foobar/1/Coldlight", rows[3])
	}
	if rows[4].Char != "Foobar" || rows[4].Level != 4 {
		t.Errorf("rows[4] = %+v, want Foobar/level 4", rows[4])
	}
}

// v1: 'Warrior char (no class spells available): zero rows'
func TestSpellCheck_WarriorZeroRows(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	seedChar(t, db, "o", "Tankard", "WAR", 60, "HUM", false)
	// no WAR wiki spells

	rows, err := compute.SpellCheck(ctx, s)
	if err != nil {
		t.Fatalf("SpellCheck: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("got %d rows, want 0 (Warrior has no class spells): %+v", len(rows), rows)
	}
}

// v1: 'normalized-name match handles capitalization + whitespace + apostrophes'
func TestSpellCheck_NormalizedNameMatch(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	c := seedChar(t, db, "o", "Slampeach", "NEC", 10, "HUM", false)
	seedWikiSpell(t, db, "NEC", 4, "Numb the Dead", norm("Numb the Dead"))
	// Spellbook display name differs by caps + trailing space, but normalizes equal.
	seedSpellbook(t, db, c, 4, "numb the dead  ", norm("numb the dead  "))

	rows, err := compute.SpellCheck(ctx, s)
	if err != nil {
		t.Fatalf("SpellCheck: %v", err)
	}
	if len(rows) != 1 || rows[0].Status != "KNOWN" {
		t.Errorf("rows = %+v, want one KNOWN (normalized match)", rows)
	}
}
