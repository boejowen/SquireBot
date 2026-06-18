package compute_test

// fixtures_test.go holds the raw-insert seed helpers shared by the compute parity
// tests (view/bank/gearcheck/spellcheck). The store package's own _test.go seed
// helpers are package-private, so the external compute_test package re-defines
// thin INSERT helpers here over store.NewTestDB's migrated *sql.DB.
//
// The column layouts mirror migrations/00001_init.sql + 00003_enrich_columns.sql.
// These deliberately use raw INSERTs (not the watcher Replace* paths) so a test
// can seed exactly the rows + item_ids (including 0/NULL) the v1 vitest fixtures
// seed, translating each v1 seed-array into the equivalent DB rows.

import (
	"database/sql"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// newTestDB returns a migrated temp-DB handle (the shared backend fixture).
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	return store.NewTestDB(t)
}

// seedChar inserts an owner + a character with the given metadata and a fixed
// last_seen, returning the character id. class/level/race map to the v1
// _char_owner cols (class=E, level=F, race=N); isBank maps to is_bank_toon.
func seedChar(t *testing.T, db *sql.DB, ownerLabel, name, class string, level int64, race string, isBank bool) int64 {
	t.Helper()
	res, err := db.Exec(`INSERT INTO owner (label) VALUES (?)`, ownerLabel)
	if err != nil {
		t.Fatalf("seed owner %q: %v", ownerLabel, err)
	}
	ownerID, _ := res.LastInsertId()
	bank := 0
	if isBank {
		bank = 1
	}
	res, err = db.Exec(
		`INSERT INTO character (owner_id, name, class, level, race, is_bank_toon, last_seen)
		 VALUES (?,?,?,?,?,?,?)`,
		ownerID, name, nullable(class), nullableInt(level), nullable(race), bank, "2026-05-09T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("seed character %q: %v", name, err)
	}
	charID, _ := res.LastInsertId()
	return charID
}

// seedInv inserts one inventory_item row with an explicit item_id (pass 0 for an
// empty slot — stored as the literal 0, matching the watcher's parse of a blank/0
// ID). The empty-slot exclusion in the view/bank join keys on item_id > 0.
func seedInv(t *testing.T, db *sql.DB, charID int64, location, name string, itemID, ordinal int64) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO inventory_item (character_id, location, name, item_id, count, slots, row_ordinal, uploaded_at)
		 VALUES (?,?,?,?,?,?,?,datetime('now'))`,
		charID, location, name, itemID, 1, 0, ordinal,
	); err != nil {
		t.Fatalf("seed inventory_item %q: %v", name, err)
	}
}

// seedInvFull seeds one inventory_item row with explicit count + slots + location, so
// container shells (slots>0), stacked items (count>1), and nested children (location with
// a "-SlotN" suffix) are all testable. Mirrors seedInv but exposes count/slots/location —
// seedInv hardcodes count=1, slots=0 and cannot seed a container shell or a nested child.
func seedInvFull(t *testing.T, db *sql.DB, charID int64, location, name string, itemID, count, slots, ordinal int64) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO inventory_item (character_id, location, name, item_id, count, slots, row_ordinal, uploaded_at)
		 VALUES (?,?,?,?,?,?,?,datetime('now'))`,
		charID, location, name, itemID, count, slots, ordinal,
	); err != nil {
		t.Fatalf("seed inventory_item %q: %v", name, err)
	}
}

// loadInventoryFixture reads a real-name watcher-format inventory dump from testdata/
// (tab-separated, header `Location\tName\tID\tCount\tSlots`) and seeds each data line
// into inventory_item via seedInvFull, assigning an incrementing row_ordinal so file
// (slot) order is preserved. It skips the header line and any line whose ID is not an
// integer (mirroring the watcher's own non-int-ID skip in internal/parse/inventory.go),
// so container shells (Slots>0), stacked items (Count>1), empty slots (ID 0), and nested
// `<Parent>-Slot<N>` children all land in the store for the INV-05 nesting/value tests.
func loadInventoryFixture(t *testing.T, db *sql.DB, charID int64, name string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read inventory fixture %q: %v", name, err)
	}
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	var ordinal int64
	for i, line := range lines {
		if i == 0 {
			continue // header
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 5 {
			t.Fatalf("inventory fixture %q line %d has %d cols, want 5: %q", name, i+1, len(cols), line)
		}
		itemID, err := strconv.ParseInt(strings.TrimSpace(cols[2]), 10, 64)
		if err != nil {
			continue // non-int ID — matches the watcher's parse skip
		}
		count, err := strconv.ParseInt(strings.TrimSpace(cols[3]), 10, 64)
		if err != nil {
			t.Fatalf("inventory fixture %q line %d bad Count %q: %v", name, i+1, cols[3], err)
		}
		slots, err := strconv.ParseInt(strings.TrimSpace(cols[4]), 10, 64)
		if err != nil {
			t.Fatalf("inventory fixture %q line %d bad Slots %q: %v", name, i+1, cols[4], err)
		}
		ordinal++
		seedInvFull(t, db, charID, cols[0], cols[1], itemID, count, slots, ordinal)
	}
}

func seedItemMaster(t *testing.T, db *sql.DB, itemID int64, name, summary, url string, isQuest bool) {
	t.Helper()
	q := 0
	if isQuest {
		q = 1
	}
	if _, err := db.Exec(
		`INSERT INTO item_master (item_id, name, wiki_summary, wiki_url, slot, is_quest_item, wikitext_sha1, last_refreshed)
		 VALUES (?,?,?,?,?,?,?,datetime('now'))`,
		itemID, name, summary, url, "", q, "sha",
	); err != nil {
		t.Fatalf("seed item_master (item_id=%d): %v", itemID, err)
	}
}

// seedPigparse inserts one pigparse_price row. direction is the TEXT flag
// ("0"=WTS / "1"=WTB / "2"=BOTH). It sets both a30/t30 and the current_avg/
// blue_volume aliases (the P12 job writes current_avg=a30, blue_volume=t30). The
// view/bank price join bridges by NORMALIZED NAME (lower(trim(name))) NOT item_id
// (catalog ids != EQ inventory ids), so `name` MUST match the inventory item's
// name for the price to attach.
func seedPigparse(t *testing.T, db *sql.DB, itemID int64, name, direction string, a30 float64, t30 int64) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO pigparse_price (item_id, name, current_avg, blue_volume, last_seen, direction, t30, a30, last_refreshed)
		 VALUES (?,?,?,?,?,?,?,?,datetime('now'))`,
		itemID, name, a30, t30, "2026-05-09", direction, t30, a30,
	); err != nil {
		t.Fatalf("seed pigparse_price (item_id=%d, name=%q): %v", itemID, name, err)
	}
}

func seedQuest(t *testing.T, db *sql.DB, itemID int64, questName, source string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO quest_items (item_id, quest_name, source_url, source, last_refreshed)
		 VALUES (?,?,?,?,datetime('now'))`,
		itemID, questName, "http://example/q", source,
	); err != nil {
		t.Fatalf("seed quest_items (item_id=%d, quest=%q): %v", itemID, questName, err)
	}
}

// seedGear inserts one wiki_gear_tier recommendation (item_id always NULL, as the
// wiki parser produces).
func seedGear(t *testing.T, db *sql.DB, tier, class, slot, itemName string, rank int64) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO wiki_gear_tier (tier, class, slot, item_id, item_name, rank, last_refreshed)
		 VALUES (?,?,?,NULL,?,?,datetime('now'))`,
		tier, class, slot, itemName, rank,
	); err != nil {
		t.Fatalf("seed wiki_gear_tier (%s/%s/%s/%s): %v", tier, class, slot, itemName, err)
	}
}

// seedWikiSpell inserts one wiki_spells row. normalized_name is materialized in
// the DB as lower(trim(spell_name)) — the tests pass it explicitly so the join
// key is exact.
func seedWikiSpell(t *testing.T, db *sql.DB, class string, level int64, name, normalized string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO wiki_spells (class, level, spell_name, normalized_name, last_refreshed)
		 VALUES (?,?,?,?,datetime('now'))`,
		class, level, name, normalized,
	); err != nil {
		t.Fatalf("seed wiki_spells (%s/%d/%s): %v", class, level, name, err)
	}
}

// seedSpellbook inserts one spellbook_entry row (the known-spell side of the
// KNOWN/MISSING join). normalized_name passed explicitly.
func seedSpellbook(t *testing.T, db *sql.DB, charID, level int64, name, normalized string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO spellbook_entry (character_id, level, name, normalized_name, uploaded_at)
		 VALUES (?,?,?,?,datetime('now'))`,
		charID, level, name, normalized,
	); err != nil {
		t.Fatalf("seed spellbook_entry (char_id=%d, name=%q): %v", charID, name, err)
	}
}

// nullable returns nil for an empty string so the column stores SQL NULL (the v1
// _char_owner empty class/race cells map to NULL here).
func nullable(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// nullableInt returns nil for level <= 0 so the column stores SQL NULL (a char
// with no level set).
func nullableInt(n int64) interface{} {
	if n <= 0 {
		return nil
	}
	return n
}
