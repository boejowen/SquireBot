// Package enrich holds the pure, host-agnostic enrichment parsers ported 1:1
// from the Apps Script TypeScript sources (apps-script/src/lib/*). These
// functions are I/O-free — no net/http, no database/sql, no os beyond test
// fixture reads — so they are fast, deterministic, and byte-parity-checkable
// against the same JSON fixtures the TS tests use (copied into testdata/).
//
// The four parsers are:
//   - ParseToRows       (pigparse.go) — PigParse getall response → price rows
//   - ParseItempage     (wikiitem.go) — {{Itempage}} wikitext → item summary + SHA-1
//   - ParseClassPage    (wikispell.go) — per-class spell page → (class,level,name) rows
//   - ParseGearTierPage (wikigear.go) — Velious gear-tier page → (tier,class,slot) rows
//
// The orchestration (HTTP fetch, DB upsert, scheduler cadence) lives in sibling
// packages (enrich/jobs, enrich/politefetch) and in store/ — NOT here. Keeping
// the parsers pure is the D-7 acceptance proof: same input → same field values.
//
// eqconst.go is the dependency-free lookup-table file (ported from
// apps-script/src/lib/eq-constants.ts). It is imported by wikispell.go and
// wikigear.go for class validation; it imports nothing within this package, so
// no import cycle can form.
package enrich

// CLASSES is the canonical ordered list of the 14 P1999 class abbreviations,
// ported verbatim from eq-constants.ts (`CLASSES`).
var CLASSES = []string{
	"WAR", "CLR", "PAL", "RNG", "SHD", "DRU", "MNK", "BRD",
	"ROG", "SHM", "NEC", "WIZ", "MAG", "ENC",
}

// CLASS_DISPLAY_TO_ABBREV maps a wiki class display name (e.g. "Necromancer")
// to its 3-letter abbreviation. Used by the spell + gear-tier parsers when
// normalizing class names from wiki page section headers. Ported verbatim from
// eq-constants.ts (`CLASS_DISPLAY_TO_ABBREV`).
var CLASS_DISPLAY_TO_ABBREV = map[string]string{
	"Bard":          "BRD",
	"Cleric":        "CLR",
	"Druid":         "DRU",
	"Enchanter":     "ENC",
	"Magician":      "MAG",
	"Monk":          "MNK",
	"Necromancer":   "NEC",
	"Paladin":       "PAL",
	"Ranger":        "RNG",
	"Rogue":         "ROG",
	"Shadow Knight": "SHD",
	"Shaman":        "SHM",
	"Warrior":       "WAR",
	"Wizard":        "WIZ",
}

// WIKI_SLOT_TO_INV_SLOTS maps the wiki gear-tier pages' prose slot labels
// (Ears, Fingers, Wrists, Head, etc.) to the EQ in-game inventory slot tokens
// (EAR1/EAR2, etc.). Pair slots map to TWO inv slots. Ported verbatim from
// eq-constants.ts (`WIKI_SLOT_TO_INV_SLOTS`).
var WIKI_SLOT_TO_INV_SLOTS = map[string][]string{
	"Ears":      {"EAR1", "EAR2"},
	"Fingers":   {"FINGER1", "FINGER2"},
	"Wrists":    {"WRIST1", "WRIST2"},
	"Neck":      {"NECK"},
	"Head":      {"HEAD"},
	"Face":      {"FACE"},
	"Chest":     {"CHEST"},
	"Arms":      {"ARMS"},
	"Back":      {"BACK"},
	"Waist":     {"WAIST"},
	"Shoulders": {"SHOULDERS"},
	"Legs":      {"LEGS"},
	"Hands":     {"HANDS"},
	"Feet":      {"FEET"},
	"Primary":   {"PRIMARY"},
	"Secondary": {"SECONDARY"},
	"Range":     {"RANGE"},
}
