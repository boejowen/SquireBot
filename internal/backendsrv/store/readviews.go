package store

// readviews.go is the read/relational layer of the BACKEND-05 read API (Phase
// 14, D-01). It holds the parameterized SELECT...JOIN read methods that feed the
// four Go view builders in internal/backendsrv/compute (view / bank / gear_check
// / spell_check). It mirrors itemids.go exactly — the repo's only other read-side
// (*Store) method: a plain (*Store) method (reads need no tx-composing variant),
// s.db.QueryContext → rows.Next()/Scan loop → rows.Err() check, with %w-wrapped
// errors. Result structs are store-local (mirroring enrich.go's input structs),
// so the dependency runs compute → store and store never imports its caller.
//
// SQL discipline (carried from binding.go/enrich.go/itemids.go):
//   - `?` placeholders ONLY (V5 / Tampering). P14 reads take no user input
//     server-side (no filters), but the parameterized habit is kept. The only
//     dynamic SQL is a fixed-string WHERE switch on the bankOnly bool — a literal
//     branch, NEVER a value interpolation.
//   - slog is silent on the happy path; on error it logs op + err only, NEVER row
//     content or query values (V7).
//   - Every method is a pure SELECT — zero DELETE/INSERT/UPDATE (V4 read-only).
//
// Nullable-column note: the view/bank join LEFT JOINs item_master + pigparse_price
// and reads several nullable character columns, so item_id, wiki_url, wiki_summary,
// is_quest_item, direction, a30, t30, last_seen, class, level, race are scanned
// into sql.Null* and resolved to zero-values on the result structs.

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
)

// InventoryJoinRow is one row of the view/bank join: an inventory_item row joined
// to its character (Char name + last_seen) and LEFT JOINed to item_master (wiki
// enrichment) and pigparse_price (price). The price join bridges by NORMALIZED
// NAME (lower(trim(name)) — the gear_check/spell_check convention) NOT by item_id:
// the PigParse catalog (pigparse_price) and the EQ /outputfile inventory
// (inventory_item) are DIFFERENT item_id namespaces (only ~58/713 inventory ids
// exist in the catalog by id, vs ~559 names matching by name), so the old
// id-keyed price join (matching pigparse_price.item_id to inventory_item.item_id)
// silently left ~91% of held rows unpriced. The CTE
// (see InventoryJoin) collapses pigparse_price to ONE representative row per
// normalized name BEFORE the LEFT JOIN, so the join still yields AT MOST ONE price
// row per inventory row (no fan-out, no inflated bank counts) even when two catalog
// ids share a normalized name. Direction is TEXT in SQLite ("0"=WTS / "1"=WTB by
// the P12 job's strconv.Itoa) — scanned as a string. Fields that LEFT-JOIN to
// nothing resolve to zero-values.
type InventoryJoinRow struct {
	Char        string
	Location    string
	ItemName    string
	ItemID      int64
	Count       int64
	WikiURL     string
	WikiSummary string
	IsQuestItem bool
	Direction   string  // pigparse_price.direction (TEXT); "" when no price row
	A30         float64 // 30-day average; 0 when no price row
	T30         int64   // 30-day transaction count; 0 when no price row
	HasPrice    bool    // true iff a pigparse_price row joined (direction present)
	LastListed  string  // pigparse_price.last_seen (last-listed-for-sale ISO string); "" when no price row. DISTINCT from LastSeen (char upload freshness). DATA-01.
	LastSeen    string  // character.last_seen (ISO string); "" when null
	RowOrdinal  int64
}

// InventoryRow is one inventory_item row for the per-character INV-05 surface. Unlike
// InventoryJoinRow it carries Slots (container capacity, 0 = not a container) and is NOT
// filtered on item_id (empty slots + container shells + *-Slot* children all survive), so
// compute.StructuredInventory can classify + nest the full slot layout. Price bridges by
// normalized name (the pp_rep CTE — never item_id; PigParse catalog ids != EQ inventory
// ids), and LastListed (pigparse_price.last_seen) is the last-listed-for-sale date —
// distinct from LastSeen (character.last_seen, upload freshness).
type InventoryRow struct {
	Char        string
	Location    string
	ItemName    string
	ItemID      int64
	Count       int64
	Slots       int64 // inventory_item.slots — container capacity; 0 = not a container
	WikiURL     string
	WikiSummary string
	IsQuestItem bool
	IconID      int64   // item_master.icon_id — the P1999 wiki icon id (id-joined); 0 when NULL/no row (INV-04)
	Statsblock  string  // item_master.statsblock — the in-game stat block (id-joined); "" when NULL/no row (INV-02)
	Direction   string  // pigparse_price.direction (TEXT); "" when no price row
	A30         float64 // 30-day average; 0 when no price row
	T30         int64   // 30-day transaction count; 0 when no price row
	HasPrice    bool    // true iff a pigparse_price row joined
	LastListed  string  // pigparse_price.last_seen — last-listed-for-sale; "" when no price row
	LastSeen    string  // character.last_seen — upload freshness; "" when null
	RowOrdinal  int64
}

// QuestLinkRow is one quest link for an item (quest_items row), grouped by item_id
// in QuestLinksByItem. Mirrors the v1 builder's Map<number, QuestLinkRow[]>.
type QuestLinkRow struct {
	QuestName string
	Source    string
}

// WikiGearTierRow is one wiki_gear_tier recommendation (item_id is always NULL on
// these rows, so it is not surfaced). Consumed by compute.GearCheck.
type WikiGearTierRow struct {
	Tier     string
	Class    string
	Slot     string
	ItemName string
	Rank     int64
}

// GearTierPriceRow is one wiki_gear_tier recommendation with its PigParse price resolved
// by NORMALIZED NAME. A gear-tier row's id column is ALWAYS NULL (enrich/wikigear.go,
// Pitfall 4), so the price can ONLY bridge via lower(trim(item_name)) → pp_rep — never by
// the gear-tier id. HasPrice is false when no pigparse row name-matches (consumer renders "no price").
// LastListed = pigparse_price.last_seen (last-listed-for-sale, distinct from any char
// freshness). Consumed by Phase 34 (WISH-04) wishlist suggestions; the RESOLUTION read
// lives here (DATA-01 / ROADMAP SC #2 — gear-tier NULL-id rows resolve a name-keyed price).
type GearTierPriceRow struct {
	Tier       string
	Class      string
	Slot       string
	ItemName   string
	Rank       int64
	Direction  string  // pigparse_price.direction (TEXT "0"/"1"); "" when no price row
	A30        float64 // 30-day average; 0 when no price row
	T30        int64   // 30-day transaction count; 0 when no price row
	HasPrice   bool    // true iff a pigparse_price row name-matched
	LastListed string  // pigparse_price.last_seen — last-listed-for-sale; "" when no price row
}

// WikiSpellRow is one wiki_spells row. normalized_name is already materialized in
// the DB as lower(trim(spell_name)) — the same expression spellbook_entry uses —
// so compute.SpellCheck joins on it directly with no recompute.
type WikiSpellRow struct {
	Class          string
	Level          int64
	SpellName      string
	NormalizedName string
}

// CharMeta is one character's identity + metadata (class/level/race nullable in
// the schema → resolved to zero-values). Consumed by compute.GearCheck and
// compute.SpellCheck (which filter on class/level/race themselves so missing
// metadata is observable, mirroring the v1 builders). It ALSO crosses the API
// boundary as the char-meta form's pick-list/pre-fill payload (P16 CharMetaListHandler),
// so it carries snake_case JSON tags matching the frontend CharMetaItem interface;
// the compute consumers use field access (not JSON), unaffected by the tags.
//
// IsBankToon was added at the right edge (extend-only) for the form's pre-fill —
// CharsWithMeta selects it; the compute consumers ignore the extra field.
type CharMeta struct {
	ID         int64  `json:"character_id"`
	Name       string `json:"name"`
	Class      string `json:"class"`
	Level      int64  `json:"level"`
	Race       string `json:"race"`
	IsBankToon bool   `json:"is_bank_toon"`
}

// InvSlotItem is one inventory_item row reduced to the fields gear_check's
// slot-pair match consumes (Location token + display Name + ItemID). Grouped by
// char name in InventoryByChar.
type InvSlotItem struct {
	Location string
	ItemName string
	ItemID   int64
}

// CharFreshness is one character's name + last_seen, for the Plan 03 /api/v1/meta
// endpoint (character list + per-char freshness). last_seen nullable → "".
type CharFreshness struct {
	Name     string
	LastSeen string
}

// InventoryJoin returns the view/bank join rows. When bankOnly is true the result
// is scoped to bank-toon characters (character.is_bank_toon = 1); otherwise it is
// all non-removed characters. Only inventory rows with a real item_id (> 0) are
// returned — empty-slot rows (item_id 0/NULL) have no wiki page or price and are
// excluded, matching the v1 buildView/buildBank readers which skip id <= 0.
//
// The bankOnly scoping is a FIXED-STRING branch (two complete query literals),
// never a value interpolation — there is nothing user-controlled in the WHERE.
// Rows are ordered Char asc → item asc → location asc (the v1 sort); compute
// preserves this order without re-sorting.
func (s *Store) InventoryJoin(ctx context.Context, bankOnly bool) ([]InventoryJoinRow, error) {
	// pp_rep collapses pigparse_price to ONE representative row per normalized
	// name (lower(trim(name))). Cross-namespace bridge fix: the price join keys on
	// NAME, not item_id (catalog ids != EQ inventory ids). The fan-out guard lives
	// in this CTE — two catalog ids sharing a normalized name yield a single
	// representative (the MIN(item_id) row), so the LEFT JOIN below adds at most one
	// price row per inventory row (no duplicate view rows / inflated bank counts).
	// item_master stays id-keyed (im.item_id = ii.item_id) — it is the watcher's own
	// EQ-namespace enrichment, correctly id-matched.
	// The pp_rep CTE + SELECT/FROM/JOINs + base WHERE live in the shared
	// inventoryJoinBase const (reused verbatim by the Phase 33 banks+bots twin).
	base := inventoryJoinBase
	orderBy := inventoryJoinOrderBy

	// Fixed-string WHERE switch on the bool (NOT a value interpolation).
	query := base + orderBy
	if bankOnly {
		query = base + ` AND c.is_bank_toon = 1` + orderBy
	}

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query inventory join (bankOnly=%t): %w", bankOnly, err)
	}
	defer rows.Close()
	return scanInventoryJoinRows(rows)
}

// inventoryJoinBase is the shared name-bridged inventory-join SQL (the pp_rep CTE +
// SELECT + FROM/JOINs + the base WHERE) reused by InventoryJoin and the Phase 33
// InventoryJoinBanksAndBots twin. The ONLY difference between the two reads is the
// bank-set predicate appended to this base (a fixed-string branch — never a value
// interpolation); the SELECT/scan are byte-identical, so the twin keeps the legacy
// read's exact shape.
const inventoryJoinBase = `WITH pp_rep AS (
	       SELECT lower(trim(name)) AS norm_name, MIN(item_id) AS rep_item_id
	       FROM pigparse_price
	       WHERE name IS NOT NULL AND trim(name) <> ''
	       GROUP BY lower(trim(name))
	)
	SELECT c.name, ii.location, ii.name, ii.item_id, ii.count,
	       im.wiki_url, im.wiki_summary, im.is_quest_item,
	       pp.direction, pp.a30, pp.t30, pp.last_seen,
	       c.last_seen, ii.row_ordinal
	FROM inventory_item ii
	JOIN character c            ON c.id = ii.character_id
	LEFT JOIN item_master im     ON im.item_id = ii.item_id
	LEFT JOIN pp_rep             ON pp_rep.norm_name = lower(trim(ii.name))
	LEFT JOIN pigparse_price pp  ON pp.item_id = pp_rep.rep_item_id
	WHERE c.is_removed = 0 AND ii.item_id IS NOT NULL AND ii.item_id > 0`

const inventoryJoinOrderBy = `
	ORDER BY c.name, ii.name, ii.location`

// scanInventoryJoinRows drains an inventory-join *sql.Rows into []InventoryJoinRow
// with the nullable-column resolution. Factored out so InventoryJoin and the
// Phase 33 InventoryJoinBanksAndBots twin share one scan loop (no behavior drift).
func scanInventoryJoinRows(rows *sql.Rows) ([]InventoryJoinRow, error) {
	var out []InventoryJoinRow
	for rows.Next() {
		var (
			r           InventoryJoinRow
			wikiURL     sql.NullString
			wikiSummary sql.NullString
			isQuest     sql.NullInt64
			direction   sql.NullString
			a30         sql.NullFloat64
			t30         sql.NullInt64
			lastListed  sql.NullString // pigparse_price.last_seen — DISTINCT from c.last_seen below
			lastSeen    sql.NullString // character.last_seen (upload freshness)
			itemID      sql.NullInt64
			count       sql.NullInt64
		)
		if err := rows.Scan(
			&r.Char, &r.Location, &r.ItemName, &itemID, &count,
			&wikiURL, &wikiSummary, &isQuest,
			&direction, &a30, &t30, &lastListed,
			&lastSeen, &r.RowOrdinal,
		); err != nil {
			return nil, fmt.Errorf("scan inventory join row: %w", err)
		}
		r.ItemID = itemID.Int64
		r.Count = count.Int64
		r.WikiURL = wikiURL.String
		r.WikiSummary = wikiSummary.String
		r.IsQuestItem = isQuest.Int64 != 0
		r.Direction = direction.String
		r.HasPrice = direction.Valid
		r.A30 = a30.Float64
		r.T30 = t30.Int64
		r.LastListed = lastListed.String // pp.last_seen (last-listed); never aliased to LastSeen
		r.LastSeen = lastSeen.String
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate inventory join rows: %w", err)
	}
	return out, nil
}

// InventoryJoinBanksAndBots returns the flat name-joined inventory rows for every guild-bank
// AND guild-bot character (is_bank_toon=1 OR is_guild_bot=1). It mirrors InventoryJoin's body
// verbatim — the same pp_rep name-bridge CTE, the same SELECT/scan — differing ONLY in the
// WHERE predicate. Dedicated read (Phase 33 Option B): the shared InventoryJoin(ctx,true)
// bankOnly branch has two callers (compute.BankValuationFor + the legacy compute.Bank grid at
// /api/v1/views/bank), so widening it in place would silently pull bots into that legacy grid;
// this twin leaves it untouched. (BANK-02 — the guild item-value total includes bot holdings.)
//
// The bank-set predicate is a compile-time fixed string (no name, no value, no fmt.Sprintf
// enters the WHERE — V5 / Tampering), matching the InventoryJoin discipline.
func (s *Store) InventoryJoinBanksAndBots(ctx context.Context) ([]InventoryJoinRow, error) {
	query := inventoryJoinBase + ` AND (c.is_bank_toon = 1 OR c.is_guild_bot = 1)` + inventoryJoinOrderBy
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query inventory join (banks+bots): %w", err)
	}
	defer rows.Close()
	return scanInventoryJoinRows(rows)
}

// InventoryForChar returns EVERY inventory_item row for one character — including
// empty-slot rows (item_id 0/NULL), container shells (slots>0), and *-Slot* bag-content
// children — ordered by row_ordinal (file/slot order), so compute.StructuredInventory can
// classify + nest the full INV-05 slot layout. Unlike InventoryJoin it does NOT filter on
// item_id (the paperdoll/nesting needs the empty + container rows InventoryJoin drops).
//
// The price join is the SAME name-keyed pp_rep bridge as InventoryJoin (commit 0a169f3) —
// NEVER an item_id price join (PigParse catalog ids != EQ inventory ids; the id-join
// silently left ~91% of held rows unpriced). item_master stays id-keyed (the watcher's own
// EQ namespace). pp.last_seen is projected as the DATA-01 last-listed-for-sale date,
// distinct from c.last_seen (upload freshness).
//
// char is the only user-controlled value; it is bound via a `?` placeholder (never
// concatenated). slog is silent on the happy path; the error path logs op+err only.
func (s *Store) InventoryForChar(ctx context.Context, char string) ([]InventoryRow, error) {
	const query = `WITH pp_rep AS (
	       SELECT lower(trim(name)) AS norm_name, MIN(item_id) AS rep_item_id
	       FROM pigparse_price
	       WHERE name IS NOT NULL AND trim(name) <> ''
	       GROUP BY lower(trim(name))
	)
	SELECT c.name, ii.location, ii.name, ii.item_id, ii.count, ii.slots,
	       im.wiki_url, im.wiki_summary, im.is_quest_item, im.icon_id, im.statsblock,
	       pp.direction, pp.a30, pp.t30, pp.last_seen,
	       c.last_seen, ii.row_ordinal
	FROM inventory_item ii
	JOIN character c            ON c.id = ii.character_id
	LEFT JOIN item_master im     ON im.item_id = ii.item_id
	LEFT JOIN pp_rep             ON pp_rep.norm_name = lower(trim(ii.name))
	LEFT JOIN pigparse_price pp  ON pp.item_id = pp_rep.rep_item_id
	WHERE c.is_removed = 0 AND c.name = ?
	ORDER BY ii.row_ordinal`

	rows, err := s.db.QueryContext(ctx, query, char)
	if err != nil {
		return nil, fmt.Errorf("query inventory for char: %w", err)
	}
	defer rows.Close()

	var out []InventoryRow
	for rows.Next() {
		var (
			r            InventoryRow
			wikiURL      sql.NullString
			wikiSummary  sql.NullString
			isQuest      sql.NullInt64
			iconID       sql.NullInt64  // item_master.icon_id — NULL until enrichment runs (INV-04)
			statsblock   sql.NullString // item_master.statsblock — NULL until enrichment runs (INV-02)
			direction    sql.NullString
			a30          sql.NullFloat64
			t30          sql.NullInt64
			lastListed   sql.NullString // pigparse_price.last_seen — DISTINCT from charLastSeen
			charLastSeen sql.NullString // character.last_seen (upload freshness)
			itemID       sql.NullInt64
			count        sql.NullInt64
			slots        sql.NullInt64
		)
		if err := rows.Scan(
			&r.Char, &r.Location, &r.ItemName, &itemID, &count, &slots,
			&wikiURL, &wikiSummary, &isQuest, &iconID, &statsblock,
			&direction, &a30, &t30, &lastListed,
			&charLastSeen, &r.RowOrdinal,
		); err != nil {
			return nil, fmt.Errorf("scan inventory row: %w", err)
		}
		r.ItemID = itemID.Int64
		r.Count = count.Int64
		r.Slots = slots.Int64
		r.WikiURL = wikiURL.String
		r.WikiSummary = wikiSummary.String
		r.IsQuestItem = isQuest.Int64 != 0
		r.IconID = iconID.Int64          // 0 when NULL (no icon yet) or no item_master row joined
		r.Statsblock = statsblock.String // "" when NULL (no stats yet) or no item_master row joined
		r.Direction = direction.String
		r.HasPrice = direction.Valid
		r.A30 = a30.Float64
		r.T30 = t30.Int64
		r.LastListed = lastListed.String // pp.last_seen (last-listed); never aliased to LastSeen
		r.LastSeen = charLastSeen.String // character.last_seen (upload freshness)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate inventory rows: %w", err)
	}
	return out, nil
}

// QuestLinksByItem returns all quest_items links grouped by item_id (mirrors the
// v1 builder's Map<number, QuestLinkRow[]>; grouping in Go avoids fanning out the
// view join). Within each item_id the links keep (quest_name) order.
func (s *Store) QuestLinksByItem(ctx context.Context) (map[int64][]QuestLinkRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT item_id, quest_name, source FROM quest_items ORDER BY item_id, quest_name`)
	if err != nil {
		return nil, fmt.Errorf("query quest links: %w", err)
	}
	defer rows.Close()

	out := make(map[int64][]QuestLinkRow)
	for rows.Next() {
		var (
			itemID int64
			link   QuestLinkRow
			source sql.NullString
		)
		if err := rows.Scan(&itemID, &link.QuestName, &source); err != nil {
			return nil, fmt.Errorf("scan quest link row: %w", err)
		}
		link.Source = source.String
		out[itemID] = append(out[itemID], link)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate quest link rows: %w", err)
	}
	return out, nil
}

// WikiGearTiers returns all wiki_gear_tier recommendation rows, ordered for stable
// grouping in compute.GearCheck. item_id is always NULL on these rows (not read).
func (s *Store) WikiGearTiers(ctx context.Context) ([]WikiGearTierRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT tier, class, slot, item_name, rank
		 FROM wiki_gear_tier
		 ORDER BY tier, class, slot, rank`)
	if err != nil {
		return nil, fmt.Errorf("query wiki gear tiers: %w", err)
	}
	defer rows.Close()

	var out []WikiGearTierRow
	for rows.Next() {
		var (
			r        WikiGearTierRow
			itemName sql.NullString
			rank     sql.NullInt64
		)
		if err := rows.Scan(&r.Tier, &r.Class, &r.Slot, &itemName, &rank); err != nil {
			return nil, fmt.Errorf("scan wiki gear tier row: %w", err)
		}
		r.ItemName = itemName.String
		r.Rank = rank.Int64
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate wiki gear tier rows: %w", err)
	}
	return out, nil
}

// GearTierPrices returns every wiki_gear_tier recommendation row with its PigParse price +
// last-listed resolved by NORMALIZED ITEM NAME (the same pp_rep CTE InventoryJoin /
// InventoryForChar use, commit 0a169f3) — a gear-tier row's id column is ALWAYS NULL, so the
// price can ONLY bridge by name (Pitfall 4; the join keys solely on lower(trim(item_name))
// and NEVER references the gear-tier id column). This is
// the DATA-01 / ROADMAP SC #2 read: a NULL-id gear-tier row whose item_name name-matches a
// pigparse_price row resolves a price (hit); an unmatched row resolves nil (miss). Consumed
// by Phase 34 (WISH-04). The read takes no user param (it is a full-table gear-tier read),
// so there is nothing to bind; slog is silent on the happy path, the error path logs op+err.
func (s *Store) GearTierPrices(ctx context.Context) ([]GearTierPriceRow, error) {
	const query = `WITH pp_rep AS (
	       SELECT lower(trim(name)) AS norm_name, MIN(item_id) AS rep_item_id
	       FROM pigparse_price
	       WHERE name IS NOT NULL AND trim(name) <> ''
	       GROUP BY lower(trim(name))
	)
	SELECT wgt.tier, wgt.class, wgt.slot, wgt.item_name, wgt.rank,
	       pp.direction, pp.a30, pp.t30, pp.last_seen
	FROM wiki_gear_tier wgt
	LEFT JOIN pp_rep             ON pp_rep.norm_name = lower(trim(wgt.item_name))
	LEFT JOIN pigparse_price pp  ON pp.item_id = pp_rep.rep_item_id
	ORDER BY wgt.tier, wgt.class, wgt.slot, wgt.rank`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query gear tier prices: %w", err)
	}
	defer rows.Close()

	var out []GearTierPriceRow
	for rows.Next() {
		var (
			r          GearTierPriceRow
			itemName   sql.NullString
			rank       sql.NullInt64
			direction  sql.NullString
			a30        sql.NullFloat64
			t30        sql.NullInt64
			lastListed sql.NullString // pigparse_price.last_seen — last-listed-for-sale
		)
		if err := rows.Scan(
			&r.Tier, &r.Class, &r.Slot, &itemName, &rank,
			&direction, &a30, &t30, &lastListed,
		); err != nil {
			return nil, fmt.Errorf("scan gear tier price row: %w", err)
		}
		r.ItemName = itemName.String
		r.Rank = rank.Int64
		r.Direction = direction.String
		r.HasPrice = direction.Valid
		r.A30 = a30.Float64
		r.T30 = t30.Int64
		r.LastListed = lastListed.String
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate gear tier price rows: %w", err)
	}
	return out, nil
}

// WikiSpells returns all wiki_spells rows, ordered for stable grouping in
// compute.SpellCheck. normalized_name is read as-is (already materialized as
// lower(trim(spell_name)) in the DB).
func (s *Store) WikiSpells(ctx context.Context) ([]WikiSpellRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT class, level, spell_name, normalized_name
		 FROM wiki_spells
		 ORDER BY class, level, spell_name`)
	if err != nil {
		return nil, fmt.Errorf("query wiki spells: %w", err)
	}
	defer rows.Close()

	var out []WikiSpellRow
	for rows.Next() {
		var r WikiSpellRow
		if err := rows.Scan(&r.Class, &r.Level, &r.SpellName, &r.NormalizedName); err != nil {
			return nil, fmt.Errorf("scan wiki spell row: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate wiki spell rows: %w", err)
	}
	return out, nil
}

// CharsWithMeta returns every non-removed character with its identity + metadata
// (class/level/race nullable → zero-values). It does NOT filter on class/level/
// race — compute.GearCheck and compute.SpellCheck apply those filters themselves
// so missing-metadata characters are observable (matching the v1 builders, which
// read all char rows and skip the metadata-less ones in the builder body).
func (s *Store) CharsWithMeta(ctx context.Context) ([]CharMeta, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, class, level, race, is_bank_toon
		 FROM character
		 WHERE is_removed = 0
		 ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query chars with meta: %w", err)
	}
	defer rows.Close()

	var out []CharMeta
	for rows.Next() {
		var (
			c          CharMeta
			class      sql.NullString
			level      sql.NullInt64
			race       sql.NullString
			isBankToon int
		)
		if err := rows.Scan(&c.ID, &c.Name, &class, &level, &race, &isBankToon); err != nil {
			return nil, fmt.Errorf("scan char meta row: %w", err)
		}
		c.Class = class.String
		c.Level = level.Int64
		c.Race = race.String
		c.IsBankToon = isBankToon == 1
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate char meta rows: %w", err)
	}
	return out, nil
}

// InventoryByChar returns every non-removed character's inventory rows grouped by
// char name (gear_check's slot-pair match consumes this per character). Unlike
// InventoryJoin this does NOT filter on item_id — gear_check matches on the
// location slot token + item name, not the id (mirrors the v1 readInventoriesByChar,
// which keeps all rows with a non-empty name). Rows with an empty item name are
// skipped (an empty EQ slot), matching v1.
func (s *Store) InventoryByChar(ctx context.Context) (map[string][]InvSlotItem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT c.name, ii.location, ii.name, ii.item_id
		 FROM inventory_item ii
		 JOIN character c ON c.id = ii.character_id
		 WHERE c.is_removed = 0`)
	if err != nil {
		return nil, fmt.Errorf("query inventory by char: %w", err)
	}
	defer rows.Close()

	out := make(map[string][]InvSlotItem)
	for rows.Next() {
		var (
			charName string
			item     InvSlotItem
			itemID   sql.NullInt64
		)
		if err := rows.Scan(&charName, &item.Location, &item.ItemName, &itemID); err != nil {
			return nil, fmt.Errorf("scan inventory by char row: %w", err)
		}
		if item.ItemName == "" {
			continue // empty EQ slot — v1 readInventoriesByChar skips these
		}
		item.ItemID = itemID.Int64
		out[charName] = append(out[charName], item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate inventory by char rows: %w", err)
	}
	return out, nil
}

// SpellbookNormalizedByChar returns each non-removed character's set of known
// spell normalized_names (spell_check's KNOWN/MISSING join). The set membership
// is a direct equality on the materialized normalized_name — no recompute, since
// both spellbook_entry.normalized_name and wiki_spells.normalized_name are written
// as lower(trim(name)).
func (s *Store) SpellbookNormalizedByChar(ctx context.Context) (map[string]map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT c.name, se.normalized_name
		 FROM spellbook_entry se
		 JOIN character c ON c.id = se.character_id
		 WHERE c.is_removed = 0`)
	if err != nil {
		return nil, fmt.Errorf("query spellbook normalized by char: %w", err)
	}
	defer rows.Close()

	out := make(map[string]map[string]bool)
	for rows.Next() {
		var charName, normalized string
		if err := rows.Scan(&charName, &normalized); err != nil {
			return nil, fmt.Errorf("scan spellbook normalized row: %w", err)
		}
		set := out[charName]
		if set == nil {
			set = make(map[string]bool)
			out[charName] = set
		}
		set[normalized] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate spellbook normalized rows: %w", err)
	}
	return out, nil
}

// CharFreshness returns each non-removed character's name + last_seen for the Plan
// 03 /api/v1/meta endpoint. last_seen nullable → "".
func (s *Store) CharFreshness(ctx context.Context) ([]CharFreshness, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT name, last_seen
		 FROM character
		 WHERE is_removed = 0
		 ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query char freshness: %w", err)
	}
	defer rows.Close()

	var out []CharFreshness
	for rows.Next() {
		var (
			c        CharFreshness
			lastSeen sql.NullString
		)
		if err := rows.Scan(&c.Name, &lastSeen); err != nil {
			return nil, fmt.Errorf("scan char freshness row: %w", err)
		}
		c.LastSeen = lastSeen.String
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate char freshness rows: %w", err)
	}
	return out, nil
}

// IconStats is item_master's per-item icon + stat-block, id-correct in the watcher's
// own EQ namespace (item_master is keyed by the EQ inventory item_id — distinct from
// the PigParse catalog ids). It carries the INV-04 icon_id (0 = none yet → colored-tile
// fallback) + the INV-02 statsblock ("" = none yet → examine omits the stats line).
type IconStats struct {
	IconID     int64
	Statsblock string
}

// ItemMasterIconStats returns icon_id + statsblock per item_id (P31 schema v13 columns
// 00012/00013). The id-key is correct HERE because item_master is the watcher's own EQ
// namespace — NOT the PigParse catalog (the Phase 32 rollup uses the representative
// ViewRow.ID ONLY for this id-correct lookup, NEVER for price). Full-table SELECT, `?`-free
// (no untrusted input); a NULL icon_id maps to 0 and a NULL statsblock to "" (the
// nullable-scan idiom at InventoryForChar).
func (s *Store) ItemMasterIconStats(ctx context.Context) (map[int64]IconStats, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT item_id, icon_id, statsblock FROM item_master`)
	if err != nil {
		return nil, fmt.Errorf("query item_master icon/stats: %w", err)
	}
	defer rows.Close()

	out := make(map[int64]IconStats)
	for rows.Next() {
		var (
			id    int64
			icon  sql.NullInt64
			stats sql.NullString
		)
		if err := rows.Scan(&id, &icon, &stats); err != nil {
			return nil, fmt.Errorf("scan item_master icon/stats row: %w", err)
		}
		out[id] = IconStats{IconID: icon.Int64, Statsblock: stats.String} // NULL → 0 / ""
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate item_master icon/stats rows: %w", err)
	}
	return out, nil
}

// RosterRow is one row of the Phase 31-02 viewer-aware Characters-tab roster: a
// character's identity + metadata + the bank/bot designation flags + the per-char
// upload freshness + whether the row is assigned to the VIEWING member (is_mine).
// No existing read returns this full shape together (CharsWithMeta lacks
// is_guild_bot+last_seen; CharFreshness lacks meta — RESEARCH Pitfall 4). Class/
// Race are "" and Level 0 when the metadata is unset (NULL → zero-values, D-11).
type RosterRow struct {
	ID         int64
	Name       string
	Class      string // "" when unknown
	Level      int64  // 0 when unknown
	Race       string // "" when unknown
	IsBankToon bool
	IsGuildBot bool
	LastSeen   string // character.last_seen ("" when never synced)
	IsMine     bool   // assigned to the viewer (v2.3 character_assignment)
}

// RosterFor returns every non-removed character as a RosterRow, already in the
// D-10 viewer-first order (the Characters tab renders the slice directly). The
// VIEWER's own characters come first (A-Z), then other guild characters, then
// guild banks/bots — A-Z within each band.
//
// viewerDiscordID is the authenticated discord_user_id (webauth.UserFromContext
// in the handler). It is the SINGLE `?` bind in the LEFT JOIN to
// character_assignment that computes is_mine — NEVER string-concatenated into the
// SQL (V5 / Tampering). An empty viewer (no session id) simply flags nothing
// is_mine and the "yours" band collapses; the read still returns every char.
//
// The SQL fetches A-Z (ORDER BY name COLLATE NOCASE); the three-band viewer-first
// ordering is then applied in Go (a pure, table-testable stable sort) so the
// banding logic stays observable to a unit test rather than buried in SQL.
func (s *Store) RosterFor(ctx context.Context, viewerDiscordID string) ([]RosterRow, error) {
	const query = `
		SELECT c.id, c.name, c.class, c.level, c.race,
		       c.is_bank_toon, c.is_guild_bot, c.last_seen,
		       (a.discord_user_id IS NOT NULL) AS is_mine
		  FROM character c
		  LEFT JOIN character_assignment a
		    ON a.character_id = c.id AND a.discord_user_id = ?
		 WHERE c.is_removed = 0
		 ORDER BY c.name COLLATE NOCASE`

	rows, err := s.db.QueryContext(ctx, query, viewerDiscordID)
	if err != nil {
		return nil, fmt.Errorf("query roster: %w", err)
	}
	defer rows.Close()

	var out []RosterRow
	for rows.Next() {
		var (
			r          RosterRow
			class      sql.NullString
			level      sql.NullInt64
			race       sql.NullString
			lastSeen   sql.NullString
			isBankToon int
			isGuildBot int
			isMine     int
		)
		if err := rows.Scan(
			&r.ID, &r.Name, &class, &level, &race,
			&isBankToon, &isGuildBot, &lastSeen, &isMine,
		); err != nil {
			return nil, fmt.Errorf("scan roster row: %w", err)
		}
		r.Class = class.String
		r.Level = level.Int64
		r.Race = race.String
		r.LastSeen = lastSeen.String
		r.IsBankToon = isBankToon == 1
		r.IsGuildBot = isGuildBot == 1
		r.IsMine = isMine == 1
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate roster rows: %w", err)
	}

	sortRosterViewerFirst(out)
	return out, nil
}

// sortRosterViewerFirst stable-sorts rows into the three D-10 bands in place,
// preserving the SQL's A-Z (COLLATE NOCASE) order WITHIN each band. The input is
// already A-Z, so a STABLE sort keyed only on the band index reorders the bands
// without disturbing the alphabetical order inside them.
//
// Tie-break: a char that is BOTH the viewer's AND a bank/bot sorts as MINE
// (band 0) — IsMine wins over the banks/bots designation, so a guildie who has
// claimed a bank toon still finds it at the top.
func sortRosterViewerFirst(rows []RosterRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		return rosterBand(rows[i]) < rosterBand(rows[j])
	})
}

// rosterBand returns the D-10 band index for a row: 0 = yours (IsMine, wins the
// tie-break), 2 = banks/bots (IsBankToon || IsGuildBot), 1 = everyone else.
func rosterBand(r RosterRow) int {
	switch {
	case r.IsMine:
		return 0
	case r.IsBankToon || r.IsGuildBot:
		return 2
	default:
		return 1
	}
}
