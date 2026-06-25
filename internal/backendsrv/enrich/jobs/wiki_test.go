package jobs

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/enrich/politefetch"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// --- fixture-serving wiki test server -------------------------------------

// pageToFixture maps a wiki page title (the `page=` query value, with spaces as
// underscores) to the testdata fixture file (sans .json) the server returns. It
// covers the item pages the seeded inventory references + the 14 class pages we
// have fixtures for + the 2 Velious gear pages.
var pageToFixture = map[string]string{
	// Item pages (page title == inventory name, spaces→underscores).
	"Cloth_Cap":                    "wiki-parse-cloth-cap",
	"Pearl":                        "wiki-parse-pearl",
	"Cloak_of_Flames":              "wiki-parse-cloak-of-flames",
	"Fungus_Covered_Scale_Tunic":   "wiki-parse-fungus-covered-scale-tunic",
	// Class spell pages (display name, spaces→underscores).
	"Necromancer":   "wiki-class-necromancer",
	"Paladin":       "wiki-class-paladin",
	"Warrior":       "wiki-class-warrior",
	// Velious gear pages.
	"Players:Velious_Pre-Raid_Gear": "wiki-velious-preraid-gear",
	"Players:Velious_Raiding_Gear":  "wiki-velious-raiding-gear",
}

// wikiServerOpts lets a test force a specific status for a given page title (to
// exercise 304 / 500 paths).
type wikiServerOpts struct {
	statusForPage map[string]int // page title (underscored) → forced HTTP status
}

// newWikiFixtureServer returns an httptest.Server that, for each request,
// inspects the `page=` param and returns the matching fixture body (the raw
// action=parse envelope JSON). Unknown pages return a MediaWiki "missingtitle"
// error envelope (200 with {"error":...}), mirroring the real API. Forced
// statuses from opts override the body.
func newWikiFixtureServer(t *testing.T, opts wikiServerOpts) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		if forced, ok := opts.statusForPage[page]; ok && forced != http.StatusOK {
			w.WriteHeader(forced)
			return
		}
		fixture, ok := pageToFixture[page]
		if !ok {
			// Unknown page → MediaWiki-style error envelope.
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"error":{"code":"missingtitle","info":"The page you specified doesn't exist."}}`))
			return
		}
		body, err := os.ReadFile("../testdata/" + fixture + ".json")
		if err != nil {
			t.Errorf("read fixture %s: %v", fixture, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", `"`+page+`-v1"`)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// serverFetcher returns a Fetcher that rewrites the production wiki/pigparse URL
// to point at the fixture server (preserving the query string), then delegates
// to the real politefetch.Fetch — so the test exercises the genuine ETag/304/
// body-read path against canned fixtures.
func serverFetcher(srv *httptest.Server) politefetch.Fetcher {
	base, _ := url.Parse(srv.URL)
	return func(ctx context.Context, raw string, opts politefetch.Options) politefetch.FetchResult {
		u, err := url.Parse(raw)
		if err != nil {
			return politefetch.FetchResult{OK: false, Err: err}
		}
		u.Scheme = base.Scheme
		u.Host = base.Host
		u.Path = "/" // collapse api.php → server root; the handler only reads ?page=
		return politefetch.Fetch(ctx, u.String(), opts)
	}
}

// seedInvFor seeds one inventory row so the items pass has a ref to fetch.
func seedInvFor(t *testing.T, db *sql.DB, charID int64, name string, itemID int64, ord int64) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO inventory_item (character_id, location, name, item_id, count, slots, row_ordinal, uploaded_at)
		 VALUES (?,?,?,?,?,?,?,datetime('now'))`,
		charID, "General1", name, itemID, 1, 0, ord,
	); err != nil {
		t.Fatalf("seed inventory_item %q: %v", name, err)
	}
}

// seedAllItemRefs seeds the four item fixtures we have wiki pages for.
func seedAllItemRefs(t *testing.T, db *sql.DB) {
	_, charID := seedOwnerChar(t, db, "owner-a", "Aragorn")
	seedInvFor(t, db, charID, "Cloth Cap", 1001, 1)
	seedInvFor(t, db, charID, "Pearl", 11000, 2)
	seedInvFor(t, db, charID, "Cloak of Flames", 18950, 3)
	seedInvFor(t, db, charID, "Fungus Covered Scale Tunic", 13128, 4)
}

// --- tests ----------------------------------------------------------------

func TestRunWiki_PopulatesAllTables(t *testing.T) {
	restore := setWikiSleepNoop()
	defer restore()

	db := store.NewTestDB(t)
	ctx := context.Background()
	seedAllItemRefs(t, db)

	srv := newWikiFixtureServer(t, wikiServerOpts{})
	if err := RunWiki(ctx, db, serverFetcher(srv)); err != nil {
		t.Fatalf("RunWiki: %v", err)
	}

	for _, table := range []string{"item_master", "wiki_spells", "wiki_gear_tier", "quest_items"} {
		if n := countRows(t, db, table); n == 0 {
			t.Errorf("%s is empty after RunWiki, want > 0", table)
		}
	}

	// A known item's summary + SHA-1 + statsblock must be populated (the job must carry the
	// parsed ParsedWikiItem fields all the way into item_master — INV-02/04 end-to-end).
	var summary, sha, statsblock string
	if err := db.QueryRow(`SELECT wiki_summary, wikitext_sha1, statsblock FROM item_master WHERE item_id = ?`, 1001).
		Scan(&summary, &sha, &statsblock); err != nil {
		t.Fatalf("query item_master 1001: %v", err)
	}
	if summary == "" || sha == "" {
		t.Errorf("item 1001 wiki_summary=%q wikitext_sha1=%q, want both populated", summary, sha)
	}
	// INV-02: the job must persist the cleaned stat block (regression — the store.ItemMaster
	// literal once omitted the Statsblock field, so every write stored "").
	if statsblock == "" {
		t.Errorf("item 1001 statsblock is empty — the job did not persist the parsed stat block")
	}

	// wiki_gear_tier must contain at least one Iksar-tagged row (Pre-Raid page).
	var iksar int
	if err := db.QueryRow(`SELECT count(*) FROM wiki_gear_tier WHERE tier = 'Iksar'`).Scan(&iksar); err != nil {
		t.Fatalf("count iksar gear rows: %v", err)
	}
	if iksar == 0 {
		t.Errorf("wiki_gear_tier has no Iksar-tagged rows, want > 0")
	}

	assertJobStatus(t, db, "wiki_weekly", "ok")
}

func TestRunWiki_SHA1ShortCircuit(t *testing.T) {
	restore := setWikiSleepNoop()
	defer restore()

	db := store.NewTestDB(t)
	ctx := context.Background()
	seedAllItemRefs(t, db)
	srv := newWikiFixtureServer(t, wikiServerOpts{})

	if err := RunWiki(ctx, db, serverFetcher(srv)); err != nil {
		t.Fatalf("RunWiki #1: %v", err)
	}
	firstCount := countRows(t, db, "item_master")

	// Second run: same fixtures → same SHA-1 → the per-item upsert is skipped.
	if err := RunWiki(ctx, db, serverFetcher(srv)); err != nil {
		t.Fatalf("RunWiki #2: %v", err)
	}
	secondCount := countRows(t, db, "item_master")

	if secondCount != firstCount {
		t.Errorf("item_master rows changed across identical runs: %d -> %d (SHA-1 short-circuit/dedup broken)", firstCount, secondCount)
	}
}

// TestRunWiki_BackfillsStaleIcon is the INV-04 icon-backfill regression (2026-06-18): a
// row written BEFORE the icon_id column (migration 00012) has the correct wikitext SHA-1
// but a 0 icon. The OLD short-circuit skipped on SHA-1 alone and left such a row's icon
// permanently 0; the fix must re-write and backfill the icon even though the wikitext is
// unchanged. (The one-time production backfill clears etag_cache so the pages re-fetch —
// mirrored here.)
func TestRunWiki_BackfillsStaleIcon(t *testing.T) {
	restore := setWikiSleepNoop()
	defer restore()

	db := store.NewTestDB(t)
	ctx := context.Background()
	seedAllItemRefs(t, db)
	srv := newWikiFixtureServer(t, wikiServerOpts{})

	// Run 1 populates item_master incl. icon_id from the fixture's lucy_img_ID.
	if err := RunWiki(ctx, db, serverFetcher(srv)); err != nil {
		t.Fatalf("RunWiki #1: %v", err)
	}
	const cofID = 18950 // Cloak of Flames — its fixture carries a lucy_img_ID
	var icon0 int64
	if err := db.QueryRow(`SELECT icon_id FROM item_master WHERE item_id=?`, cofID).Scan(&icon0); err != nil {
		t.Fatalf("read icon after run 1: %v", err)
	}
	if icon0 == 0 {
		t.Fatalf("precondition: Cloak of Flames icon_id is 0 after run 1 — fixture lacks a lucy_img_ID")
	}

	// Simulate a pre-00012 row: same wikitext SHA-1, but a 0 icon. Clear etag_cache so the
	// page re-fetches (the production backfill clears it for exactly this reason).
	if _, err := db.Exec(`UPDATE item_master SET icon_id=0 WHERE item_id=?`, cofID); err != nil {
		t.Fatalf("zero the icon: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM etag_cache`); err != nil {
		t.Fatalf("clear etag_cache: %v", err)
	}

	// Run 2: identical fixtures → unchanged SHA-1 → the OLD code skipped and left icon 0.
	if err := RunWiki(ctx, db, serverFetcher(srv)); err != nil {
		t.Fatalf("RunWiki #2: %v", err)
	}
	var icon1 int64
	if err := db.QueryRow(`SELECT icon_id FROM item_master WHERE item_id=?`, cofID).Scan(&icon1); err != nil {
		t.Fatalf("read icon after run 2: %v", err)
	}
	if icon1 != icon0 {
		t.Errorf("icon_id after backfill run = %d, want %d (an unchanged-SHA1 row must still backfill its icon)", icon1, icon0)
	}
}

// TestRunWiki_BackfillsStaleFlags is the ENRICH-12/13 flags-backfill regression (Phase
// 37): a row written BEFORE the flags_json column (migration 00016) has the correct
// wikitext SHA-1 + icon + statsblock but a NULL flags_json. The freshness short-circuit
// must NOT skip such a row on SHA-1 alone — it must re-write and backfill the flag/effect
// columns even though the wikitext is unchanged (exactly the icon-backfill argument, one
// more field). Mirrors TestRunWiki_BackfillsStaleIcon. (The one-time production backfill +
// the weekly freshness pass both heal these rows; this test exercises the weekly path.)
func TestRunWiki_BackfillsStaleFlags(t *testing.T) {
	restore := setWikiSleepNoop()
	defer restore()

	db := store.NewTestDB(t)
	ctx := context.Background()
	seedAllItemRefs(t, db)
	srv := newWikiFixtureServer(t, wikiServerOpts{})

	// Run 1 populates item_master incl. flags_json from the Cloak of Flames fixture
	// (MAGIC ITEM + Haste +36%).
	if err := RunWiki(ctx, db, serverFetcher(srv)); err != nil {
		t.Fatalf("RunWiki #1: %v", err)
	}
	const cofID = 18950 // Cloak of Flames — its fixture carries MAGIC ITEM + Haste
	var flags0 string
	var magic0 int
	if err := db.QueryRow(`SELECT flags_json, is_magic FROM item_master WHERE item_id=?`, cofID).
		Scan(&flags0, &magic0); err != nil {
		t.Fatalf("read flags after run 1: %v", err)
	}
	if magic0 != 1 || flags0 == "" || flags0 == "[]" {
		t.Fatalf("precondition: Cloak of Flames is_magic=%d flags_json=%q after run 1 — fixture lacks flags", magic0, flags0)
	}

	// Simulate a pre-00016 row: same wikitext SHA-1 + icon + statsblock, but NULL
	// flags_json (and 0 is_magic). Clear etag_cache so the page re-fetches (the
	// production backfill clears it for exactly this reason).
	if _, err := db.Exec(`UPDATE item_master SET flags_json=NULL, is_magic=0, has_haste=0, haste_pct=0 WHERE item_id=?`, cofID); err != nil {
		t.Fatalf("null the flags: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM etag_cache`); err != nil {
		t.Fatalf("clear etag_cache: %v", err)
	}

	// Run 2: identical fixtures → unchanged SHA-1 → the OLD (SHA-1-only) skip would have
	// left flags_json NULL; the fix must re-write and backfill the flag/effect columns.
	if err := RunWiki(ctx, db, serverFetcher(srv)); err != nil {
		t.Fatalf("RunWiki #2: %v", err)
	}
	var flags1 string
	var magic1, haste1, hastePct1 int
	if err := db.QueryRow(`SELECT flags_json, is_magic, has_haste, haste_pct FROM item_master WHERE item_id=?`, cofID).
		Scan(&flags1, &magic1, &haste1, &hastePct1); err != nil {
		t.Fatalf("read flags after run 2: %v", err)
	}
	if flags1 != flags0 {
		t.Errorf("flags_json after backfill run = %q, want %q (an unchanged-SHA1 row must still backfill its flags)", flags1, flags0)
	}
	if magic1 != 1 {
		t.Errorf("is_magic after backfill run = %d, want 1 (the flag must re-derive)", magic1)
	}
	if haste1 != 1 || hastePct1 != 36 {
		t.Errorf("has_haste/haste_pct after backfill run = %d/%d, want 1/36 (Cloak of Flames Haste +36%%)", haste1, hastePct1)
	}
}

// seedCatalogItem inserts one pigparse_price catalog row (the jobs package cannot
// call the unexported store.seedPigparse, so it inserts the same columns directly).
// The catalog id is in the PigParse namespace — pass an id that does NOT collide
// with any held EQ id and is NOT already in item_master.
func seedCatalogItem(t *testing.T, db *sql.DB, itemID int64, name string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO pigparse_price (item_id, name, current_avg, blue_volume, last_seen, direction, t30, a30, last_refreshed)
		 VALUES (?,?,?,?,?,?,?,?,datetime('now'))`,
		itemID, name, 1000.0, 5, "2026-06-25", "up", 5, 1000.0,
	); err != nil {
		t.Fatalf("seed pigparse_price (item_id=%d, name=%q): %v", itemID, name, err)
	}
}

// TestRunWiki_EnrichesUnheldCatalogItem is the ENRICH-14/15 win condition (Phase
// 38): the widened crawl iterates the held∪catalog union, so an UNHELD pigparse_price
// catalog item whose wiki page carries a lucy_img_ID gets an item_master row with a
// non-zero icon_id — even though no character holds it. A junk catalog name with no
// wiki page must NOT abort the run (it lands in the icon-less residue, Pitfall 3).
func TestRunWiki_EnrichesUnheldCatalogItem(t *testing.T) {
	restore := setWikiSleepNoop()
	defer restore()

	db := store.NewTestDB(t)
	ctx := context.Background()

	// Seed a HELD item so the held arm of the union is non-empty (and is byte-for-byte
	// the prior behavior). Cloth Cap (1001) has a wiki fixture.
	_, charID := seedOwnerChar(t, db, "owner-a", "Aragorn")
	seedInvFor(t, db, charID, "Cloth Cap", 1001, 1)

	// Seed an UNHELD catalog item whose name matches a fixture with a lucy_img_ID
	// (Cloak of Flames). Its id 90950 is in the PigParse namespace — no collision
	// with any held EQ id and not already in item_master.
	const cofCatalogID = 90950
	seedCatalogItem(t, db, cofCatalogID, "Cloak of Flames")

	// Seed a junk catalog name with NO wiki fixture → the fixture server returns a
	// MediaWiki missingtitle envelope → the page is logged-and-skipped (residue),
	// and the run must still complete with status "ok".
	seedCatalogItem(t, db, 90999, "Totally Not A Real Item 9000")

	srv := newWikiFixtureServer(t, wikiServerOpts{})
	if err := RunWiki(ctx, db, serverFetcher(srv)); err != nil {
		t.Fatalf("RunWiki: %v", err)
	}

	// The unheld catalog item now has an item_master row. It is keyed by its PigParse
	// id, so look it up by normalized name (the held∪catalog bridge), not by id.
	var icon int64
	var name string
	if err := db.QueryRow(
		`SELECT name, icon_id FROM item_master WHERE lower(trim(name)) = lower(trim(?))`,
		"Cloak of Flames",
	).Scan(&name, &icon); err != nil {
		t.Fatalf("unheld catalog item Cloak of Flames has no item_master row after the widened crawl: %v", err)
	}
	if icon == 0 {
		t.Errorf("Cloak of Flames icon_id = 0 after enrichment, want > 0 (ENRICH-15 icon backfill — its fixture carries a lucy_img_ID)")
	}

	// The held item is still enriched (the held arm is preserved).
	var heldN int
	if err := db.QueryRow(`SELECT count(*) FROM item_master WHERE item_id = ?`, 1001).Scan(&heldN); err != nil {
		t.Fatalf("count held Cloth Cap: %v", err)
	}
	if heldN == 0 {
		t.Errorf("held Cloth Cap (1001) absent from item_master — the held arm of the union was lost")
	}

	// The junk catalog name did NOT abort the run; the job completed "ok".
	assertJobStatus(t, db, "wiki_weekly", "ok")
}

func TestRunWiki_304SkipsResource(t *testing.T) {
	restore := setWikiSleepNoop()
	defer restore()

	db := store.NewTestDB(t)
	ctx := context.Background()
	seedAllItemRefs(t, db)

	// Force the Paladin class page to 304 (unchanged); all others 200.
	srv := newWikiFixtureServer(t, wikiServerOpts{
		statusForPage: map[string]int{"Paladin": http.StatusNotModified},
	})
	if err := RunWiki(ctx, db, serverFetcher(srv)); err != nil {
		t.Fatalf("RunWiki: %v", err)
	}

	// Necromancer (200) populated; Paladin (304) absent.
	var necN, palN int
	if err := db.QueryRow(`SELECT count(*) FROM wiki_spells WHERE class = 'NEC'`).Scan(&necN); err != nil {
		t.Fatalf("count NEC: %v", err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM wiki_spells WHERE class = 'PAL'`).Scan(&palN); err != nil {
		t.Fatalf("count PAL: %v", err)
	}
	if necN == 0 {
		t.Errorf("NEC spells = 0, want > 0 (200 should populate)")
	}
	if palN != 0 {
		t.Errorf("PAL spells = %d, want 0 (304 should skip the write)", palN)
	}
}

func TestRunWiki_GearFullReplaceNoDuplicates(t *testing.T) {
	restore := setWikiSleepNoop()
	defer restore()

	db := store.NewTestDB(t)
	ctx := context.Background()
	seedAllItemRefs(t, db)
	srv := newWikiFixtureServer(t, wikiServerOpts{})

	if err := RunWiki(ctx, db, serverFetcher(srv)); err != nil {
		t.Fatalf("RunWiki #1: %v", err)
	}
	first := countRows(t, db, "wiki_gear_tier")

	if err := RunWiki(ctx, db, serverFetcher(srv)); err != nil {
		t.Fatalf("RunWiki #2: %v", err)
	}
	second := countRows(t, db, "wiki_gear_tier")

	if first == 0 {
		t.Fatalf("wiki_gear_tier empty after first run")
	}
	if second != first {
		t.Errorf("wiki_gear_tier rows = %d on 2nd run, want %d (full-replace, not 2x — Pitfall 1)", second, first)
	}
}

func TestRunWiki_OneBadPageDoesNotAbort(t *testing.T) {
	restore := setWikiSleepNoop()
	defer restore()

	db := store.NewTestDB(t)
	ctx := context.Background()
	seedAllItemRefs(t, db)

	// One item page (Pearl) returns 500; the run must still complete + populate
	// the other items/tables.
	srv := newWikiFixtureServer(t, wikiServerOpts{
		statusForPage: map[string]int{"Pearl": http.StatusInternalServerError},
	})
	if err := RunWiki(ctx, db, serverFetcher(srv)); err != nil {
		t.Fatalf("RunWiki: want nil error despite one bad page, got %v", err)
	}

	// Cloth Cap (1001) populated; Pearl (11000) absent.
	var clothN, pearlN int
	if err := db.QueryRow(`SELECT count(*) FROM item_master WHERE item_id = ?`, 1001).Scan(&clothN); err != nil {
		t.Fatalf("count cloth: %v", err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM item_master WHERE item_id = ?`, 11000).Scan(&pearlN); err != nil {
		t.Fatalf("count pearl: %v", err)
	}
	if clothN == 0 {
		t.Errorf("item 1001 absent, want present (one bad page must not abort the run)")
	}
	if pearlN != 0 {
		t.Errorf("item 11000 present (%d), want absent (its page 500'd)", pearlN)
	}
	// Spells + gear still populated.
	if n := countRows(t, db, "wiki_spells"); n == 0 {
		t.Errorf("wiki_spells empty, want populated (a bad ITEM page must not block the spells pass)")
	}
	if n := countRows(t, db, "wiki_gear_tier"); n == 0 {
		t.Errorf("wiki_gear_tier empty, want populated")
	}

	assertJobStatus(t, db, "wiki_weekly", "ok")
}
