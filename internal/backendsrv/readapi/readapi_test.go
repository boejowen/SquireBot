package readapi_test

// readapi_test.go is the httptest proof of the P14 read HTTP surface (BACKEND-05
// HTTP half). It does NOT re-prove compute parity (Plan 01's table-tests own
// that) — it proves the HANDLER contract Plan 04 wires to:
//   - each views route → 200 + application/json + the right-shaped body;
//   - /meta → 200 + a {characters:[{name,last_seen}]} object;
//   - the bank body is an object with coin === null (not a bare array);
//   - a non-GET request → 405 (read-only contract, T-14.03-03);
//   - the CORS middleware echoes the EXACT locked origin (never "*") on a GET and
//     answers an OPTIONS preflight with 204 + that header + an empty body.
//
// It seeds a migrated temp DB (store.NewTestDB) with a couple of characters +
// inventory rows + one priced/quest item via self-contained raw INSERT helpers
// (the store/compute packages' own seed helpers are package-private), mirroring
// the verified column layouts in migrations/00001_init.sql + 00003.

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/readapi"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
	"github.com/boejowen/SquireBot/internal/backendsrv/webauth"
)

const testOrigin = "https://squirebot.quest"

// --- seed helpers (raw INSERTs over the migrated temp DB) ---------------------

func seedChar(t *testing.T, db *sql.DB, label, name, class string, level int64, isBank bool) int64 {
	t.Helper()
	res, err := db.Exec(`INSERT INTO owner (label) VALUES (?)`, label)
	if err != nil {
		t.Fatalf("seed owner %q: %v", label, err)
	}
	ownerID, _ := res.LastInsertId()
	bank := 0
	if isBank {
		bank = 1
	}
	var classArg any
	if class != "" {
		classArg = class
	}
	res, err = db.Exec(
		`INSERT INTO character (owner_id, name, class, level, is_bank_toon, last_seen)
		 VALUES (?,?,?,?,?,?)`,
		ownerID, name, classArg, level, bank, "2026-05-09T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("seed character %q: %v", name, err)
	}
	id, _ := res.LastInsertId()
	return id
}

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

// seedPigparse inserts one pigparse_price row. The view/bank price join bridges by
// NORMALIZED NAME (lower(trim(name))) NOT item_id (catalog ids != EQ inventory
// ids), so `name` MUST match the inventory item's name for the price to attach.
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
		t.Fatalf("seed quest_items (item_id=%d): %v", itemID, err)
	}
}

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

func seedSpellbook(t *testing.T, db *sql.DB, charID, level int64, name, normalized string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO spellbook_entry (character_id, level, name, normalized_name, uploaded_at)
		 VALUES (?,?,?,?,datetime('now'))`,
		charID, level, name, normalized,
	); err != nil {
		t.Fatalf("seed spellbook_entry (char_id=%d): %v", charID, err)
	}
}

// seedStore builds a small but representative world: two characters (one a bank
// toon) with inventory, one priced+quest item, one wiki spell the char knows, so
// every endpoint returns at least one row.
func seedStore(t *testing.T) *store.Store {
	t.Helper()
	db := store.NewTestDB(t)

	enchID := seedChar(t, db, "owner-a", "Alpha", "Enchanter", 60, false)
	bankID := seedChar(t, db, "owner-b", "Banker", "Warrior", 60, true)

	// A priced + quest item so a ViewRow carries a non-null price + quest link.
	seedItemMaster(t, db, 1001, "Jade Reaver", "A fine blade.", "https://wiki.project1999.com/Jade_Reaver", false)
	seedPigparse(t, db, 1001, "Jade Reaver", "0", 1500.0, 7) // WTS, a30>0 → non-null price (name-bridged)
	seedQuest(t, db, 1001, "Sword Quest", "in_game_flag")

	seedInv(t, db, enchID, "General1", "Jade Reaver", 1001, 0)
	seedInv(t, db, enchID, "General2", "Bone Chips", 13073, 1)
	seedInv(t, db, bankID, "Bank1", "Jade Reaver", 1001, 0)

	// spell_check input: a class spell the char knows (KNOWN) + one it lacks.
	seedWikiSpell(t, db, "Enchanter", 1, "Minor Illusion", "minor illusion")
	seedWikiSpell(t, db, "Enchanter", 4, "Pendril's Animation", "pendril's animation")
	seedSpellbook(t, db, enchID, 1, "Minor Illusion", "minor illusion")

	return store.NewStore(db)
}

// --- the handler-contract tests ----------------------------------------------

func TestViewsView_OK(t *testing.T) {
	h := readapi.NewViews(seedStore(t), "view")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/views/view", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	var rows []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode view body as JSON array: %v (body=%s)", err, rec.Body.String())
	}
	if len(rows) == 0 {
		t.Fatalf("view returned 0 rows, want >=1")
	}
	// The leading Char column is the consolidated-view contract — assert the key.
	if _, ok := rows[0]["char"]; !ok {
		t.Fatalf("view row missing \"char\" key; got keys %v", keysOf(rows[0]))
	}
	// snake_case enrichment fields the client tooltip consumes.
	for _, k := range []string{"slot", "item", "id", "count", "wiki_url", "price", "last_synced"} {
		if _, ok := rows[0][k]; !ok {
			t.Errorf("view row missing %q key; got keys %v", k, keysOf(rows[0]))
		}
	}
}

func TestViewsGearCheck_OK(t *testing.T) {
	h := readapi.NewViews(seedStore(t), "gear_check")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/views/gear_check", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var rows []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode gear_check body as JSON array: %v (body=%s)", err, rec.Body.String())
	}
	// gear_check may legitimately be empty (no Velious tier rows seeded), but it
	// MUST decode as an array, not null/object.
	if rows == nil {
		t.Fatalf("gear_check body decoded to nil, want a JSON array")
	}
}

func TestViewsSpellCheck_OK(t *testing.T) {
	h := readapi.NewViews(seedStore(t), "spell_check")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/views/spell_check", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var rows []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode spell_check body as JSON array: %v (body=%s)", err, rec.Body.String())
	}
	if len(rows) == 0 {
		t.Fatalf("spell_check returned 0 rows, want >=1 (seeded Enchanter spells)")
	}
	for _, k := range []string{"char", "class", "level", "spell", "status"} {
		if _, ok := rows[0][k]; !ok {
			t.Errorf("spell_check row missing %q key; got keys %v", k, keysOf(rows[0]))
		}
	}
}

func TestViewsBank_OK_CoinNull(t *testing.T) {
	h := readapi.NewViews(seedStore(t), "bank")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/views/bank", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	// The bank body is an OBJECT {rows:[...], coin:null}, NOT a bare array.
	var body struct {
		Rows []map[string]any `json:"rows"`
		Coin json.RawMessage  `json:"coin"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode bank body as object: %v (body=%s)", err, rec.Body.String())
	}
	// coin === null in P14 (ADMIN-05 fills it in P15). Assert the key is present
	// AND its value is JSON null.
	if string(body.Coin) != "null" {
		t.Fatalf("bank coin = %q, want JSON null", string(body.Coin))
	}
	if len(body.Rows) == 0 {
		t.Fatalf("bank returned 0 rows, want >=1 (seeded a bank toon with inventory)")
	}
	if _, ok := body.Rows[0]["char"]; !ok {
		t.Fatalf("bank row missing \"char\" key; got keys %v", keysOf(body.Rows[0]))
	}
}

func TestMeta_OK(t *testing.T) {
	h := readapi.NewMeta(seedStore(t))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/meta", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	var body struct {
		Characters []struct {
			Name     string `json:"name"`
			LastSeen string `json:"last_seen"`
		} `json:"characters"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode meta body: %v (body=%s)", err, rec.Body.String())
	}
	if len(body.Characters) != 2 {
		t.Fatalf("meta characters = %d, want 2", len(body.Characters))
	}
	if body.Characters[0].Name == "" {
		t.Fatalf("meta character missing name; got %+v", body.Characters[0])
	}
	if body.Characters[0].LastSeen == "" {
		t.Fatalf("meta character missing last_seen; got %+v", body.Characters[0])
	}
}

func TestViews_NonGET_405(t *testing.T) {
	h := readapi.NewViews(seedStore(t), "view")
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, "/api/v1/views/view", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s status = %d, want 405", method, rec.Code)
		}
	}
}

func TestMeta_NonGET_405(t *testing.T) {
	h := readapi.NewMeta(seedStore(t))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/meta", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /meta status = %d, want 405", rec.Code)
	}
}

// --- CORS middleware contract -------------------------------------------------

func TestCORS_GET_EchoesExactOrigin(t *testing.T) {
	inner := readapi.NewViews(seedStore(t), "view")
	h := readapi.CORS(testOrigin, inner)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/views/view", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (CORS must pass GET through to the handler)", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != testOrigin {
		t.Fatalf("Access-Control-Allow-Origin = %q, want exact %q", got, testOrigin)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got == "*" {
		t.Fatalf("Access-Control-Allow-Origin must never be the wildcard")
	}
	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Errorf("Vary = %q, want Origin", got)
	}
}

func TestCORS_OPTIONS_Preflight204(t *testing.T) {
	// The inner handler must NOT run on a preflight — use one that fails the test
	// if invoked, proving the middleware short-circuits OPTIONS.
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("inner handler ran on an OPTIONS preflight; CORS must short-circuit it")
	})
	h := readapi.CORS(testOrigin, inner)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodOptions, "/api/v1/views/view", nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("OPTIONS status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != testOrigin {
		t.Fatalf("preflight Access-Control-Allow-Origin = %q, want %q", got, testOrigin)
	}
	body, _ := io.ReadAll(rec.Body)
	if len(body) != 0 {
		t.Fatalf("OPTIONS preflight body = %q, want empty", string(body))
	}
}

// TestCORS_Credentials_OnGETandPreflight proves the P15 credential-aware upgrade
// (D-05 / T-15-10): the credentialed cross-subdomain cookie requires
// Access-Control-Allow-Credentials:true on BOTH the actual response and the
// preflight, the origin must be the EXACT origin (never "*", which the browser
// rejects with credentials), and POST must be allowed (15-03 write forms).
func TestCORS_Credentials_OnGETandPreflight(t *testing.T) {
	inner := readapi.NewViews(seedStore(t), "view")
	h := readapi.CORS(testOrigin, inner)

	// Actual GET.
	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/api/v1/views/view", nil))
	if got := getRec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("GET Access-Control-Allow-Credentials = %q, want true", got)
	}
	if got := getRec.Header().Get("Access-Control-Allow-Origin"); got == "*" {
		t.Errorf("GET Access-Control-Allow-Origin must never be the wildcard with credentials")
	}
	if got := getRec.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "POST") {
		t.Errorf("GET Access-Control-Allow-Methods = %q, want it to include POST", got)
	}

	// Preflight OPTIONS — credentials header REQUIRED here too.
	optRec := httptest.NewRecorder()
	preInner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	readapi.CORS(testOrigin, preInner).ServeHTTP(optRec, httptest.NewRequest(http.MethodOptions, "/api/v1/views/view", nil))
	if got := optRec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("preflight Access-Control-Allow-Credentials = %q, want true", got)
	}
	if got := optRec.Header().Get("Access-Control-Allow-Origin"); got == "*" {
		t.Errorf("preflight Access-Control-Allow-Origin must never be the wildcard with credentials")
	}
}

// --- inventory + characters handlers (Phase 31-02) ---------------------------

func seedWebUser(t *testing.T, db *sql.DB, discordUserID, username string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO web_user (discord_user_id, username, avatar, first_seen, last_login)
		 VALUES (?,?,NULL,0,0)`, discordUserID, username,
	); err != nil {
		t.Fatalf("seed web_user %q: %v", discordUserID, err)
	}
}

func seedAssignment(t *testing.T, db *sql.DB, charID int64, discordUserID string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO character_assignment (character_id, discord_user_id, assigned_at, assigned_by)
		 VALUES (?,?,0,'test')`, charID, discordUserID,
	); err != nil {
		t.Fatalf("seed character_assignment (char=%d, %s): %v", charID, discordUserID, err)
	}
}

// TestInventory_UnknownChar_EmptyNot404 proves the V4/D-11 empty-not-404 contract:
// an unknown character returns 200 with the CharacterInventory shape whose three
// slot arrays are empty `[]` (NOT null, NOT a 404), so the client renders
// "no inventory synced yet".
func TestInventory_UnknownChar_EmptyNot404(t *testing.T) {
	h := readapi.NewInventory(seedStore(t))
	rec := httptest.NewRecorder()
	// PathValue is populated by the ServeMux {char} pattern in production; set it
	// explicitly for the direct handler test (Go 1.22+ Request.SetPathValue).
	req := httptest.NewRequest(http.MethodGet, "/api/v1/inventory/Nobody", nil)
	req.SetPathValue("char", "Nobody")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (empty-not-404 for an unknown char)", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	var body struct {
		Char      string            `json:"char"`
		Equipment []json.RawMessage `json:"equipment"`
		General   []json.RawMessage `json:"general"`
		Bank      []json.RawMessage `json:"bank"`
		LastSeen  string            `json:"last_seen"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode inventory body: %v (body=%s)", err, rec.Body.String())
	}
	// The three arrays must be present + empty (not null). json.RawMessage slices
	// unmarshal to non-nil empty slices for `[]` and nil for `null`/absent.
	if body.Equipment == nil || body.General == nil || body.Bank == nil {
		t.Fatalf("inventory arrays must be [] not null: %s", rec.Body.String())
	}
	if len(body.Equipment) != 0 || len(body.General) != 0 || len(body.Bank) != 0 {
		t.Fatalf("unknown char must have empty inventory: %s", rec.Body.String())
	}
}

// TestInventory_KnownChar_RendersSlots proves a seeded char's inventory comes back
// with its general slots (the seedStore Alpha char holds two general items).
func TestInventory_KnownChar_RendersSlots(t *testing.T) {
	h := readapi.NewInventory(seedStore(t))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/inventory/Alpha", nil)
	req.SetPathValue("char", "Alpha")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Char    string           `json:"char"`
		General []map[string]any `json:"general"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode inventory body: %v (body=%s)", err, rec.Body.String())
	}
	if body.Char != "Alpha" {
		t.Errorf("char = %q, want Alpha", body.Char)
	}
	if len(body.General) == 0 {
		t.Fatalf("Alpha general slots = 0, want >=1 (seeded two general items)")
	}
	// The icon_id contract field (Plan 31-01) is present on each slot.
	if _, ok := body.General[0]["icon_id"]; !ok {
		t.Errorf("inventory slot missing \"icon_id\" key; got keys %v", keysOf(body.General[0]))
	}
}

func TestInventory_NonGET_405(t *testing.T) {
	h := readapi.NewInventory(seedStore(t))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/inventory/Alpha", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /inventory status = %d, want 405", rec.Code)
	}
}

// TestCharacters_ViewerFirstRoster proves the roster endpoint returns the
// viewer-aware roster (the viewer's chars first), session identity read from the
// request context (RequireSession injects it in production; the test injects it
// via webauth.WithUser exactly as the gate would).
func TestCharacters_ViewerFirstRoster(t *testing.T) {
	db := store.NewTestDB(t)
	const viewer = "discord-viewer-1"
	seedWebUser(t, db, viewer, "Viewer")

	// Two non-bank chars: "Zzz" (the viewer's) must sort before "Aaa" (not theirs)
	// because the viewer's band comes first despite Z > A alphabetically.
	mine := seedChar(t, db, "owner-m", "Zzz", "Enchanter", 60, false)
	seedChar(t, db, "owner-o", "Aaa", "Warrior", 55, false)
	seedAssignment(t, db, mine, viewer)

	h := readapi.NewCharacters(store.NewStore(db))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/characters", nil).
		WithContext(webauth.WithUser(context.Background(), viewer))
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	var roster []struct {
		Name       string `json:"name"`
		Level      int64  `json:"level"`
		Race       string `json:"race"`
		Class      string `json:"class"`
		IsMine     bool   `json:"is_mine"`
		IsBankToon bool   `json:"is_bank_toon"`
		IsGuildBot bool   `json:"is_guild_bot"`
		LastSeen   string `json:"last_seen"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &roster); err != nil {
		t.Fatalf("decode characters body as JSON array: %v (body=%s)", err, rec.Body.String())
	}
	if len(roster) != 2 {
		t.Fatalf("roster = %d, want 2: %+v", len(roster), roster)
	}
	// Viewer-first: the viewer's "Zzz" sorts before the non-viewer "Aaa".
	if roster[0].Name != "Zzz" || !roster[0].IsMine {
		t.Errorf("roster[0] = %+v, want Zzz (the viewer's, is_mine=true) first", roster[0])
	}
	if roster[1].Name != "Aaa" || roster[1].IsMine {
		t.Errorf("roster[1] = %+v, want Aaa (not the viewer's, is_mine=false)", roster[1])
	}
	if roster[0].Class != "Enchanter" || roster[0].Level != 60 {
		t.Errorf("roster[0] meta = {class:%q level:%d}, want Enchanter/60", roster[0].Class, roster[0].Level)
	}
}

// TestCharacters_EmptyRoster_ArrayNotNull proves an empty roster encodes as `[]`.
func TestCharacters_EmptyRoster_ArrayNotNull(t *testing.T) {
	db := store.NewTestDB(t)
	h := readapi.NewCharacters(store.NewStore(db))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/characters", nil).
		WithContext(webauth.WithUser(context.Background(), "discord-x"))
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
		t.Fatalf("empty roster body = %q, want []", got)
	}
}

func TestCharacters_NonGET_405(t *testing.T) {
	db := store.NewTestDB(t)
	h := readapi.NewCharacters(store.NewStore(db))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/characters", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /characters status = %d, want 405", rec.Code)
	}
}

// TestNewRoutes_RequireSession_401WithoutCookie proves the BLOCKING T-31-05 gate:
// BOTH new routes, registered under webauth.RequireSession, return 401 (NOT the
// inner 200) when the request carries no session cookie — the data-exposure gate
// is at the API, fail-closed, not just the UI. This exercises the SAME wrap the
// production registration in cmd/squirebot-server/main.go applies.
func TestNewRoutes_RequireSession_401WithoutCookie(t *testing.T) {
	db := store.NewTestDB(t)
	st := store.NewStore(db)

	routes := map[string]http.Handler{
		"/api/v1/inventory/Alpha": webauth.RequireSession(db, readapi.NewInventory(st)),
		"/api/v1/characters":      webauth.RequireSession(db, readapi.NewCharacters(st)),
	}
	for path, h := range routes {
		rec := httptest.NewRecorder()
		// No sb_session cookie → RequireSession must reject with 401, fail-closed.
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s without a session = %d, want 401 (RequireSession fail-closed)", path, rec.Code)
		}
	}
}

// keysOf returns the keys of a decoded JSON object for clearer failure messages.
func keysOf(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
