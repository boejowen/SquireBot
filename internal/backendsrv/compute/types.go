// Package compute is the Go reimplementation of the four v1 Apps Script view
// builders (buildView / buildBank / buildGearCheck / buildSpellCheck), computing
// the consolidated view/bank/gear_check/spell_check rows on read over the live
// SQLite store (D-01 / D-02). It is the compute half of BACKEND-05; the HTTP
// handlers that JSON-encode these structs live in Plan 14-03.
//
// Dependency direction: compute imports internal/backendsrv/store (the read
// methods) and internal/backendsrv/enrich (the WIKI_SLOT_TO_INV_SLOTS constant).
// store never imports compute. compute authors NO SQL — it consumes the typed
// store-local structs the read methods return.
//
// ──────────────────────────────────────────────────────────────────────────────
// FIXED CROSS-PLAN JSON CONTRACT (snake_case — MANDATORY, not optional)
// ──────────────────────────────────────────────────────────────────────────────
// These struct field → JSON-tag mappings are the public read-API payload shape.
// Plan 14-02's composeNotes.ts, Plan 14-03's read handlers, and Plan 14-04's
// Svelte client ALL consume these exact names. Do NOT rename a tag without
// updating those consumers. snake_case matches the existing backend JSON style.
//
//	ViewRow / BankRow (identical shape):
//	  Char        → "char"
//	  Slot        → "slot"          (inventory_item.location)
//	  Item        → "item"          (inventory_item.name)
//	  ID          → "id"            (inventory_item.item_id)
//	  Count       → "count"
//	  WikiURL     → "wiki_url"      (PLAIN url — never an =HYPERLINK formula)
//	  Price       → "price"         (*float64; null when neither WTS nor WTB a30>0)
//	  LastSynced  → "last_synced"   (ISO string; character.last_seen)
//	  WikiSummary → "wiki_summary"  (tooltip; raw wiki text — client escapes)
//	  IsQuestItem → "is_quest_item"
//	  Prices      → "prices"        ([]PriceDetail; the raw WTS/WTB detail)
//	  QuestLinks  → "quest_links"   ([]QuestLink)
//
//	PriceDetail:
//	  Direction   → "direction"     (string; "0"=WTS / "1"=WTB / "2"=BOTH)
//	  A30         → "a30"           (30-day average pp)
//	  T30         → "t30"           (30-day transaction count)
//
//	QuestLink:
//	  QuestName   → "quest_name"
//	  Source      → "source"        ("in_game_flag" | "notes_link")
//
//	BankView:
//	  Rows        → "rows"          ([]ViewRow / BankRow)
//	  Coin        → "coin"          (*CoinTotals; ALWAYS nil in P14 — ADMIN-05
//	                                 fills it in P15; the client renders
//	                                 "Coin: not yet recorded", never 0pp)
//
//	CoinTotals (defined for the stable P15 shape; nil in P14):
//	  PP/GP/SP/CP → "pp"/"gp"/"sp"/"cp"
//
// Direction encoding rationale: v1 used a NUMERIC PigparseDirection (0/1/2). The
// P12 SQLite store changed it to TEXT — the daily job stores strconv.Itoa(t)
// where t is 0=WTS / 1=WTB / 2=BOTH (internal/backendsrv/enrich/pigparse.go:42).
// So the stored direction values are the strings "0"/"1"/"2". pickPrice (view.go)
// compares the STRINGIFIED direction against directionWTS/directionWTB.
package compute

// PriceDetail is the raw price detail for one PigParse direction, carried inline
// on a ViewRow/BankRow so the client tooltip (composeNotes.ts) can render WTS/WTB
// lines without a second fetch (D-03). Because pigparse_price.item_id is the
// PRIMARY KEY, the join yields at most ONE price row per item, so a ViewRow's
// Prices slice holds 0 or 1 PriceDetail.
type PriceDetail struct {
	Direction string  `json:"direction"` // "0"=WTS, "1"=WTB, "2"=BOTH
	A30       float64 `json:"a30"`
	T30       int64   `json:"t30"`
}

// QuestLink is one quest this item is used in (grouped per item by the store's
// QuestLinksByItem). Names are raw/user-controlled — the client escapes them.
type QuestLink struct {
	QuestName string `json:"quest_name"`
	Source    string `json:"source"`
}

// ViewRow is one row of the consolidated `view` (and `bank`) grid: an inventory
// item with its wiki link + picked price + the inline tooltip enrichment (D-03).
// Price is the pickPrice result — nil when neither WTS nor WTB has a 30-day
// average > 0 (the client renders the Price column blank). Prices carries the raw
// per-direction detail for the tooltip. See the package doc for the full
// (field → snake_case json tag) contract.
type ViewRow struct {
	Char        string        `json:"char"`
	Slot        string        `json:"slot"`
	Item        string        `json:"item"`
	ID          int64         `json:"id"`
	Count       int64         `json:"count"`
	WikiURL     string        `json:"wiki_url"`
	Price       *float64      `json:"price"`
	LastSynced  string        `json:"last_synced"`
	WikiSummary string        `json:"wiki_summary"`
	IsQuestItem bool          `json:"is_quest_item"`
	Prices      []PriceDetail `json:"prices"`
	QuestLinks  []QuestLink   `json:"quest_links"`
}

// BankRow is structurally identical to ViewRow (the bank grid uses the same
// columns) — it is an alias so the contract stays single-sourced.
type BankRow = ViewRow

// CoinTotals is the bank toon's manual coin totals. Defined so the BankView JSON
// shape is stable for P15 (ADMIN-05), but it is ALWAYS nil in P14 — /outputfile
// inventory carries no coin data, and the admin web form that records it ships in
// P15. The client renders "Coin: not yet recorded" when coin is null; it must
// never fabricate 0pp as real data.
type CoinTotals struct {
	PP int64 `json:"pp"`
	GP int64 `json:"gp"`
	SP int64 `json:"sp"`
	CP int64 `json:"cp"`
}

// BankView is the bank endpoint's payload: the bank-toon inventory rows plus a
// nullable Coin object (nil in P14).
type BankView struct {
	Rows []BankRow   `json:"rows"`
	Coin *CoinTotals `json:"coin"`
}

// GearCheckRow is one row of the consolidated `gear_check` grid: a Velious-tier
// gear recommendation for a character, with the OK/OTHER/MISSING status of
// whether the recommended item is equipped in the matching slot.
type GearCheckRow struct {
	Char        string `json:"char"`
	Class       string `json:"class"`
	Tier        string `json:"tier"`
	Slot        string `json:"slot"`
	Have        string `json:"have"`
	Recommended string `json:"recommended"`
	Status      string `json:"status"` // OK | OTHER | MISSING
}

// SpellCheckRow is one row of the consolidated `spell_check` grid: a class spell
// the character is eligible for (level <= char level), with KNOWN/MISSING status
// from the character's spellbook.
type SpellCheckRow struct {
	Char   string `json:"char"`
	Class  string `json:"class"`
	Level  int64  `json:"level"`
	Spell  string `json:"spell"`
	Status string `json:"status"` // KNOWN | MISSING
}
