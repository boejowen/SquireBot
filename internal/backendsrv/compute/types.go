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
//	  SourceURL   → "source_url"    (P1999 wiki page; "" for an in_game_flag row)
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
// lines without a second fetch (D-03). The one-row guarantee now comes from the
// pp_rep CTE's GROUP BY norm_name + MIN(item_id) fan-out guard (store/readviews.go),
// NOT from the item_id PK — the price join is by NORMALIZED NAME, not item_id
// (commit 0a169f3, 2026-06-06). So a ViewRow's Prices slice holds 0 or 1 PriceDetail.
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
	SourceURL string `json:"source_url"` // P1999 wiki page (quest_items.source_url); "" for an in_game_flag row (ITEMUI-02)
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

// ──────────────────────────────────────────────────────────────────────────────
// Phase 29 (v2.4) — structured inventory + bank valuation contract (APPEND-ONLY)
// ──────────────────────────────────────────────────────────────────────────────
// These structs are the INV-05 / DATA-02 read-API payload shapes consumed by Phases
// 31 (per-character window) / 32 (guild rollup) / 33 (bank valuation). snake_case
// tags, append-only — no existing tag is renamed. Nullable money/price stays
// *float64/*int64 so "unpriced" / "never entered" ≠ "0" (the CoinTotals discipline).

// InventorySlot is one item in the structured inventory model (INV-05). Children holds
// nested bag contents (one level deep). Price/LastListed are name-joined (DATA-01);
// Price is nil when unpriced. Category is equipment/general/bank; CanonicalSlot is the
// paperdoll key for equipment.
type InventorySlot struct {
	Location      string          `json:"location"`       // raw token, e.g. "General4" or "General4-Slot1"
	Category      SlotCategory    `json:"category"`       // equipment|general|bank
	CanonicalSlot string          `json:"canonical_slot"` // "Head"/"Finger1"/"General4"/"Bank1"
	Item          string          `json:"item"`           // "" for an empty slot
	ID            int64           `json:"id"`
	Count         int64           `json:"count"`
	Slots         int64           `json:"slots"`       // container capacity; 0 = not a container
	Price         *float64        `json:"price"`       // pickPrice; null when unpriced
	LastListed    string          `json:"last_listed"` // pigparse_price.last_seen; "" when none
	WikiURL       string          `json:"wiki_url"`
	WikiSummary   string          `json:"wiki_summary"`
	IsQuestItem   bool            `json:"is_quest_item"`
	Prices        []PriceDetail   `json:"prices"`
	Children      []InventorySlot `json:"children"`    // nested bag contents (one level deep); nil when not a container
	IconID        int64           `json:"icon_id"`     // item_master.icon_id (id-joined); 0 = none yet → colored-tile fallback (INV-04, D-02)
	Statsblock    string          `json:"statsblock"`  // item_master.statsblock (id-joined); the in-game stat block for the examine; "" when none (INV-02)
	IsNoDrop      bool            `json:"is_no_drop"`  // item_master flag (00016); ITEMUI-01 tile outline (held source)
	IsLore        bool            `json:"is_lore"`     // item_master flag (00016); ITEMUI-01 tile outline
	IsMagic       bool            `json:"is_magic"`    // item_master flag (00016); ITEMUI-01 tile outline
	QuestLinks    []QuestLink     `json:"quest_links"` // notes_link named quests (ITEMUI-02); same shape as ViewRow.QuestLinks
}

// CharacterInventory is the per-character structured slot model (INV-05).
// Equipment/General/Bank are the three grouped trees; container rows in General/Bank
// carry their Children.
type CharacterInventory struct {
	Char      string          `json:"char"`
	Equipment []InventorySlot `json:"equipment"`
	General   []InventorySlot `json:"general"`
	Bank      []InventorySlot `json:"bank"`
	// LastSeen is the per-character upload freshness (character.last_seen) for the examine
	// "Last synced" footer (D-08 #12) — DISTINCT from per-slot LastListed (the price
	// last-listed date). Same value on every row; "" when never synced.
	LastSeen string `json:"last_seen"`
	// HasPortrait / PortraitUpdatedAt are the additive Phase 41 portrait flag (CHARUI-02,
	// D-07): the FLAG only, NEVER the bytes (which stream from GET …/{name}/portrait). The
	// web renders the portrait in the compacted paper-doll frame and uses PortraitUpdatedAt
	// as the ?v= cache-bust; both are "" / false when the char has no portrait (the existing
	// silhouette placeholder stays the fallback). Set at the StructuredInventory store-access
	// site (a PK↔PK PortraitMeta read), not inside the pure buildStructuredInventory.
	HasPortrait       bool   `json:"has_portrait"`
	PortraitUpdatedAt string `json:"portrait_updated_at"`
}

// Valuation is a bank valuation result (DATA-02/D-03): the summed pickPrice×count value
// plus the count of unpriced items (the "+N unpriced" annotation so the figure is never
// silently understated).
type Valuation struct {
	TotalValue    float64 `json:"total_value"`
	UnpricedCount int64   `json:"unpriced_count"`
}

// BankValuation is the guild bank aggregate (DATA-02): per-bank valuations keyed by
// char name, the guild-wide total, and the total platinum (literal plat only, D-04).
type BankValuation struct {
	PerBank       map[string]Valuation `json:"per_bank"`
	GuildTotal    Valuation            `json:"guild_total"`
	TotalPlatinum int64                `json:"total_platinum"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Phase 32 (v2.4) — item-centric Inventory tab rollup contract (APPEND-ONLY)
// ──────────────────────────────────────────────────────────────────────────────
// ItemRollup / ItemHolder are the ITEM-01..03 read-API payload shapes consumed by the
// guild-wide Inventory tab (web/src/routes/inventory). snake_case tags, append-only —
// no existing tag is renamed. Identity is the NORMALIZED NAME (D-01), never item_id:
// the EQ-inventory ids and the PigParse/gear-tier catalog ids are different namespaces
// (gear-tier rows have no id at all), so name is the only consistent group/join key.
// Price stays *float64 so "unpriced" ≠ "0" (the CoinTotals discipline).

// ItemRollup is one guild-wide item grouped by normalized name (D-01): every copy held
// anywhere (equipped + general + bag contents + bank) across every character, bank toon,
// and guild bot collapses to ONE rollup. SummedQty is the Σ of stack counts; HolderCount
// is the distinct holding-character count; IsMine is true when ANY holder is on a
// viewer-assigned char (D-02/ITEM-02). Price/Prices/WikiURL/WikiSummary/IsQuestItem are
// copied from the representative (first-seen) holding — name-keyed in the store's pp_rep
// CTE, NOT re-selected here. IconID/Statsblock are looked up id-correctly from item_master
// (the watcher's own EQ namespace). Holders carries one ItemHolder per holding (ITEM-03).
type ItemRollup struct {
	Name        string        `json:"name"`         // representative display name (first-seen casing)
	SummedQty   int64         `json:"summed_qty"`   // Σ Count across all holdings (D-01/D-04)
	HolderCount int64         `json:"holder_count"` // distinct holding characters (D-04)
	IsMine      bool          `json:"is_mine"`      // any holder on a viewer-assigned char (D-02/ITEM-02)
	Price       *float64      `json:"price"`        // pickPrice; null when unpriced (D-04/D-09)
	Prices      []PriceDetail `json:"prices"`       // raw WTS/WTB detail (examine)
	WikiURL     string        `json:"wiki_url"`
	WikiSummary string        `json:"wiki_summary"`
	IsQuestItem bool          `json:"is_quest_item"`
	IconID      int64         `json:"icon_id"`     // 0 → colored-tile fallback (D-02)
	Statsblock  string        `json:"statsblock"`  // "" → examine omits the stats line (D-09)
	IsClicky    bool          `json:"is_clicky"`   // Phase 39 — from item_master (00016); client holdings facet (SC-4)
	HasHaste    bool          `json:"has_haste"`   // Phase 39 — from item_master (00016)
	IsNoDrop    bool          `json:"is_no_drop"`  // Phase 40 — from item_master (00016); ITEMUI-01 examine flag
	IsLore      bool          `json:"is_lore"`     // Phase 40 — from item_master (00016)
	IsMagic     bool          `json:"is_magic"`    // Phase 40 — from item_master (00016)
	QuestLinks  []QuestLink   `json:"quest_links"` // named quests (ITEMUI-02); copied from the representative ViewRow
	Holders     []ItemHolder  `json:"holders"`     // one per holding (ITEM-03)
}

// ItemHolder is one holding of an item (ITEM-03 holders-table row): a character holding it,
// the slot label (from classifySlot — P29), the stack qty, the per-char last-synced day/time
// (= character.last_seen), and the viewer/bank flags for the holder banding + tags.
type ItemHolder struct {
	Char       string `json:"char"`
	SlotLabel  string `json:"slot_label"` // from classifySlot (P29)
	Qty        int64  `json:"qty"`
	LastSynced string `json:"last_synced"` // ViewRow.LastSynced (= character.last_seen)
	IsMine     bool   `json:"is_mine"`
	IsBank     bool   `json:"is_bank"` // is_bank_toon || is_guild_bot
}

// ──────────────────────────────────────────────────────────────────────────────
// Phase 33 (v2.4) — Banks tab valuation contract (APPEND-ONLY)
// ──────────────────────────────────────────────────────────────────────────────
// BanksView / BankRowSummary are the BANK-01/02 read-API payload (GET /api/v1/banks):
// the A-Z bank+bot roster (each with a clean per-bank item count, value, and nullable
// platinum) plus the guild-wide summary (total item value across bank+bot holdings + total
// platinum). snake_case tags, append-only — no existing tag is renamed. The item VALUE
// scope is bank+bot (the guild bot's goods count, D-01/D-02); PLATINUM stays bank-toon-gated
// (a bot's Plat is nil → contributes 0), so Plat stays *int64 (nil ≠ 0 — the coin discipline).

// BanksView is the GET /api/v1/banks payload — the A-Z bank/bot rows + the guild summary.
type BanksView struct {
	Banks         []BankRowSummary `json:"banks"`          // A-Z; one per IsBankToon||IsGuildBot toon
	GuildValue    float64          `json:"guild_value"`    // GuildTotal.TotalValue over bank+bot rows (D-02)
	GuildUnpriced int64            `json:"guild_unpriced"` // GuildTotal.UnpricedCount ("+N unpriced")
	TotalPlatinum int64            `json:"total_platinum"` // Σ bank-toon plat (nil skipped) (D-02/D-04 guild)
}

// BankRowSummary is one bank/bot's clean list row (D-02) + its D-04 detail-header numbers.
type BankRowSummary struct {
	Name      string  `json:"name"`
	ItemCount int64   `json:"item_count"` // Σ flat inventory rows held by this bank (clean-row count, D-02)
	Value     float64 `json:"value"`      // PerBank[name].TotalValue (D-04)
	Unpriced  int64   `json:"unpriced"`   // PerBank[name].UnpricedCount
	Plat      *int64  `json:"plat"`       // BankToon.Plat; null = never recorded (D-04, nullable discipline)
}

// ──────────────────────────────────────────────────────────────────────────────
// Phase 34 (v2.4) — per-character / per-slot Wishlist contract (APPEND-ONLY)
// ──────────────────────────────────────────────────────────────────────────────
// WishlistView / WishlistSlot / WishlistTarget / WishlistSuggestion are the
// WISH-02/03/04 read-API payload (GET /api/v1/wishlist/{char}, 34-02). snake_case
// tags, append-only — no existing tag is renamed. Slots is the fixed 21-worn-slot
// list in paperdoll order (Charm/Power omitted, D-04); each carries its equipped item
// + the viewer's active targets (auto-removal applied — a target whose normalized name
// the char holds anywhere is HIDDEN, D-02) + the class+slot Velious gear-tier
// suggestions (WISH-04). Price stays *float64 so "unpriced" ≠ "0" (the CoinTotals
// discipline). Target price/last-listed resolve by NORMALIZED NAME against the full
// pigparse_price catalog (store.PriceByName), the same name-bridge the examine uses.

// WishlistView is the per-character wishlist payload (WISH-02/03/04).
type WishlistView struct {
	Char  string         `json:"char"`
	Slots []WishlistSlot `json:"slots"`
}

// WishlistSlot is one worn slot's wishlist row: the currently-equipped item +
// the viewer's active targets + the class+slot gear-tier suggestions.
type WishlistSlot struct {
	Slot        string               `json:"slot"`        // canonical worn-slot ("Head"/"Finger1"/…)
	Equipped    string               `json:"equipped"`    // currently-equipped item name; "" = empty slot (D-04)
	Targets     []WishlistTarget     `json:"targets"`     // viewer-added upgrade targets (auto-removal-filtered)
	Suggestions []WishlistSuggestion `json:"suggestions"` // class+slot Velious gear-tier suggestions (WISH-04)
}

// WishlistTarget is one viewer-added upgrade target on a slot's wishlist (WISH-03).
type WishlistTarget struct {
	ID         int64    `json:"id"`
	ItemID     *int64   `json:"item_id"` // null ⇒ typed/custom (no EC match) or gear-tier item
	ItemName   string   `json:"item_name"`
	Pinged     bool     `json:"pinged"`      // WISH-05 ping toggle
	PingedHit  bool     `json:"pinged_hit"`  // WISH-05 EC-hit badge (an alert_log row exists for this id)
	Price      *float64 `json:"price"`       // name-keyed pickPrice over PriceByName; null when genuinely unpriced (D-09)
	LastListed string   `json:"last_listed"` // pigparse last_seen; "" when none
	WikiURL    string   `json:"wiki_url"`
}

// WishlistSuggestion is one class+slot Velious gear-tier suggestion (WISH-04). The
// "Raid" tag is the TIER, not a column: IsRaid = (tier == "Velious Raiding") (Pitfall 3).
type WishlistSuggestion struct {
	ItemName   string   `json:"item_name"`
	IsRaid     bool     `json:"is_raid"` // tier == "Velious Raiding" ⇒ "Raid" tag + not-for-sale (Pitfall 3)
	Price      *float64 `json:"price"`   // null ⇒ "no price"/"Not for sale"
	LastListed string   `json:"last_listed"`
	WikiURL    string   `json:"wiki_url"` // "" when the gear-tier row carries no wiki url
}
