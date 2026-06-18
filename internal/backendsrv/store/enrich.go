package store

// enrich.go is the single tested SQL path for Phase 12's five dimension-table
// writes (ENRICH-10/11). Per the 11-05 single-tested-SQL-path rule (WARNING-3),
// the enrichment jobs author ZERO inline DELETE/INSERT/UPDATE SQL — they call
// the exported *Tx methods here and compose them over one *sql.Tx, exactly as
// ingest/handler.go composes BindCharacter + Replace*Tx. Each table's write
// strategy is the Sheet-faithful per-key replace locked in 12-CONTEXT D-12:
//
//	pigparse_price   per-item upsert  ON CONFLICT(item_id)        (D-4 graceful degradation)
//	item_master      per-item upsert  ON CONFLICT(item_id)        (+ SHA-1 short-circuit getter)
//	wiki_spells      per-class replace DELETE WHERE class=? + INSERT
//	wiki_gear_tier   FULL-TABLE replace DELETE (no WHERE) + INSERT  (Pitfall 1: UNIQUE-on-NULL is broken)
//	quest_items      per-item-id replace DELETE WHERE item_id=? + INSERT
//
// The input structs are store-local (NOT imported from enrich/) so store never
// imports enrich — the dependency runs jobs → store, never the reverse. The
// symmetric Store.X / XTx split mirrors replace.go: the public method begins +
// commits its own tx; the *Tx body composes inside a caller's tx.
//
// Every value is bound through a ? placeholder (V5 / Tampering): parsed item
// names, wikitext, and quest names are UNTRUSTED and are NEVER string-concatenated
// into a SQL literal. slog logs counts / ids / err only — never raw content (V7).

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
)

// PigparsePrice is the store-local input shape for one pigparse_price row. The
// daily job hands these in after parsing PigParse's getall response and applying
// the D-9 WTS (t=0) filter. current_avg/blue_volume are the Sheet's a30/t30
// aliases (the job sets current_avg=a30, blue_volume=t30); direction is the
// stringified WTS/WTB flag (strconv.Itoa of the raw t).
type PigparsePrice struct {
	ItemID        int
	Name          string
	Direction     string
	T30           int
	A30           float64
	T60           int
	A60           float64
	T6m           int
	A6m           float64
	Ty            int
	Ay            float64
	LastSeen      string
	LastRefreshed string
}

// ItemMaster is the store-local input shape for one item_master row (the wiki
// item-summary job's output). Only the columns the Sheet persisted are present
// (D-8 parity guard); ac/weight/effect/classes/is_no_drop are intentionally not
// surfaced here.
type ItemMaster struct {
	ItemID        int
	Name          string
	WikiSummary   string
	WikiURL       string
	Slot          string
	IsQuestItem   bool
	WikitextSHA1  string
	LastRefreshed string
	IconID        int // the P1999 wiki icon id (lucy_img_ID); 0 = none yet (INV-04, 00012)
}

// WikiSpell is the store-local input shape for one wiki_spells row. NormalizedName
// may be pre-filled by the job, but UpsertWikiSpellsForClassTx recomputes it from
// SpellName for safety so the P14 spellbook↔wiki join key always matches the
// lower(trim(name)) convention used by ReplaceSpellbookTx.
type WikiSpell struct {
	Class          string
	Level          int
	SpellName      string
	NormalizedName string
	LastRefreshed  string
}

// WikiGearTier is the store-local input shape for one wiki_gear_tier row. ItemID
// is always NULL from the wiki parser (transclusions carry no IDs) — which is
// exactly why this table uses full-table replace, not upsert (Pitfall 1).
type WikiGearTier struct {
	Tier          string
	Class         string
	Slot          string
	ItemID        sql.NullInt64
	ItemName      string
	Rank          int
	LastRefreshed string
}

// QuestItem is the store-local input shape for one quest_items row (one quest
// link for a given item_id). The job groups links by item_id and replaces each
// item_id's link set in one call (D-12 per-item-id replace).
type QuestItem struct {
	ItemID        int
	QuestName     string
	SourceURL     string
	Source        string
	LastRefreshed string
}

// pigparseUpsert is the verbatim 15-column UPSERT from RESEARCH §"Code Examples".
// current_avg/blue_volume/direction are the Sheet aliases; t30..ay are the
// canonical price-history columns 00003 added. ON CONFLICT(item_id) means a
// truncated PigParse response updates the rows it got and leaves the rest (D-4).
const pigparseUpsert = `INSERT INTO pigparse_price
	(item_id, name, current_avg, blue_volume, last_seen, direction,
	 t30, a30, t60, a60, t6m, a6m, ty, ay, last_refreshed)
 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
 ON CONFLICT(item_id) DO UPDATE SET
   name=excluded.name, current_avg=excluded.current_avg,
   blue_volume=excluded.blue_volume, last_seen=excluded.last_seen,
   direction=excluded.direction, t30=excluded.t30, a30=excluded.a30,
   t60=excluded.t60, a60=excluded.a60, t6m=excluded.t6m, a6m=excluded.a6m,
   ty=excluded.ty, ay=excluded.ay, last_refreshed=excluded.last_refreshed`

// UpsertPigparsePrices upserts the given price rows (begins + commits its own tx).
func (s *Store) UpsertPigparsePrices(ctx context.Context, rows []PigparsePrice) error {
	tx, err := s.db.BeginTx(ctx, nil) // _txlock=immediate DSN ⇒ BEGIN IMMEDIATE
	if err != nil {
		return fmt.Errorf("begin pigparse upsert tx: %w", err)
	}
	defer tx.Rollback() // no-op after Commit
	if err := UpsertPigparsePricesTx(ctx, tx, rows); err != nil {
		return err
	}
	return tx.Commit()
}

// UpsertPigparsePricesTx upserts each price row inside the caller's tx via a
// prepared per-item ON CONFLICT(item_id) DO UPDATE. current_avg=a30 and
// blue_volume=t30 (the Sheet's aliases). Begin/Commit/Rollback are the caller's.
func UpsertPigparsePricesTx(ctx context.Context, tx *sql.Tx, rows []PigparsePrice) error {
	stmt, err := tx.PrepareContext(ctx, pigparseUpsert)
	if err != nil {
		return fmt.Errorf("prepare pigparse upsert: %w", err)
	}
	defer stmt.Close()

	for _, r := range rows {
		// current_avg=a30, blue_volume=t30 (Sheet aliases, buildRow).
		if _, err := stmt.ExecContext(ctx,
			r.ItemID, r.Name, r.A30, r.T30, r.LastSeen, r.Direction,
			r.T30, r.A30, r.T60, r.A60, r.T6m, r.A6m, r.Ty, r.Ay, r.LastRefreshed,
		); err != nil {
			slog.Error("pigparse upsert: insert", "item_id", r.ItemID, "err", err)
			return fmt.Errorf("upsert pigparse_price (item_id=%d): %w", r.ItemID, err)
		}
	}
	slog.Info("pigparse upsert", "rows", len(rows))
	return nil
}

const itemMasterUpsert = `INSERT INTO item_master
	(item_id, name, wiki_summary, wiki_url, slot, is_quest_item, wikitext_sha1, last_refreshed, icon_id)
 VALUES (?,?,?,?,?,?,?,?,?)
 ON CONFLICT(item_id) DO UPDATE SET
   name=excluded.name, wiki_summary=excluded.wiki_summary, wiki_url=excluded.wiki_url,
   slot=excluded.slot, is_quest_item=excluded.is_quest_item,
   wikitext_sha1=excluded.wikitext_sha1, last_refreshed=excluded.last_refreshed,
   icon_id=excluded.icon_id`

// UpsertItemMaster upserts one item_master row (begins + commits its own tx).
func (s *Store) UpsertItemMaster(ctx context.Context, item ItemMaster) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin item_master upsert tx (item_id=%d): %w", item.ItemID, err)
	}
	defer tx.Rollback()
	if err := UpsertItemMasterTx(ctx, tx, item); err != nil {
		return err
	}
	return tx.Commit()
}

// UpsertItemMasterTx upserts a single item_master row inside the caller's tx via
// ON CONFLICT(item_id) DO UPDATE. The SHA-1 short-circuit is the JOB's concern
// (it compares via GetItemMasterSHA1Tx and skips calling this when unchanged).
func UpsertItemMasterTx(ctx context.Context, tx *sql.Tx, item ItemMaster) error {
	quest := 0
	if item.IsQuestItem {
		quest = 1
	}
	if _, err := tx.ExecContext(ctx, itemMasterUpsert,
		item.ItemID, item.Name, item.WikiSummary, item.WikiURL, item.Slot,
		quest, item.WikitextSHA1, item.LastRefreshed, item.IconID,
	); err != nil {
		slog.Error("item_master upsert: insert", "item_id", item.ItemID, "err", err)
		return fmt.Errorf("upsert item_master (item_id=%d): %w", item.ItemID, err)
	}
	return nil
}

// GetItemMasterSHA1Tx returns the stored wikitext_sha1 for itemID ("" when the
// row is absent), so the wiki job can compare against a freshly-computed digest
// and skip the upsert when unchanged (mirrors the Sheet's readItemMasterSha
// early-return). Read inside the caller's tx for a consistent view.
func GetItemMasterSHA1Tx(ctx context.Context, tx *sql.Tx, itemID int64) (string, error) {
	var sha sql.NullString
	err := tx.QueryRowContext(ctx,
		`SELECT wikitext_sha1 FROM item_master WHERE item_id = ?`, itemID).Scan(&sha)
	switch {
	case err == sql.ErrNoRows:
		return "", nil
	case err != nil:
		return "", fmt.Errorf("read item_master wikitext_sha1 (item_id=%d): %w", itemID, err)
	}
	return sha.String, nil // NullString.String is "" when NULL
}

// GetItemMasterFreshnessTx returns the stored wikitext_sha1 AND icon_id for itemID
// (sha "" and iconID 0 when the row is absent or the column is NULL). The wiki job's
// short-circuit compares BOTH: an unchanged wikitext alone is not enough to skip the
// upsert, because a row written BEFORE the INV-04 icon_id column (migration 00012) has
// the same SHA-1 yet a 0 icon — skipping on SHA-1 alone would leave its icon permanently
// unbackfilled. Writing whenever sha OR icon differs backfills those rows and keeps icon
// changes propagating, while a row whose sha+icon both match is still skipped.
func GetItemMasterFreshnessTx(ctx context.Context, tx *sql.Tx, itemID int64) (sha string, iconID int64, err error) {
	var s sql.NullString
	var icon sql.NullInt64
	qerr := tx.QueryRowContext(ctx,
		`SELECT wikitext_sha1, icon_id FROM item_master WHERE item_id = ?`, itemID).Scan(&s, &icon)
	switch {
	case qerr == sql.ErrNoRows:
		return "", 0, nil
	case qerr != nil:
		return "", 0, fmt.Errorf("read item_master freshness (item_id=%d): %w", itemID, qerr)
	}
	return s.String, icon.Int64, nil // NullX zero-values when NULL
}

// UpsertWikiSpellsForClass replaces all wiki_spells rows for class (begins +
// commits its own tx).
func (s *Store) UpsertWikiSpellsForClass(ctx context.Context, class string, rows []WikiSpell) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin wiki_spells replace tx (class=%q): %w", class, err)
	}
	defer tx.Rollback()
	if err := UpsertWikiSpellsForClassTx(ctx, tx, class, rows); err != nil {
		return err
	}
	return tx.Commit()
}

// UpsertWikiSpellsForClassTx does a per-class DELETE+INSERT inside the caller's
// tx (D-12, Sheet-faithful): every existing row for class is deleted, then the
// given rows are inserted. This drops spells removed from the wiki (a pure upsert
// would leave them stale). Rows for OTHER classes are untouched (scoped DELETE).
// normalized_name is recomputed as lower(trim(spell_name)) — the exact P14
// spellbook↔wiki join-key expression from ReplaceSpellbookTx (replace.go:169).
func UpsertWikiSpellsForClassTx(ctx context.Context, tx *sql.Tx, class string, rows []WikiSpell) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM wiki_spells WHERE class = ?`, class); err != nil {
		slog.Error("wiki_spells replace: delete", "class", class, "err", err)
		return fmt.Errorf("delete wiki_spells (class=%q): %w", class, err)
	}

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO wiki_spells
		(class, level, spell_name, normalized_name, last_refreshed)
		VALUES (?,?,?,?,?)`)
	if err != nil {
		return fmt.Errorf("prepare wiki_spells insert (class=%q): %w", class, err)
	}
	defer stmt.Close()

	for _, r := range rows {
		normalized := strings.ToLower(strings.TrimSpace(r.SpellName))
		if _, err := stmt.ExecContext(ctx, r.Class, r.Level, r.SpellName, normalized, r.LastRefreshed); err != nil {
			slog.Error("wiki_spells replace: insert", "class", class, "level", r.Level, "err", err)
			return fmt.Errorf("insert wiki_spells (class=%q, level=%d): %w", class, r.Level, err)
		}
	}
	slog.Info("wiki_spells replace", "class", class, "rows", len(rows))
	return nil
}

// ReplaceWikiGearTier replaces the ENTIRE wiki_gear_tier table (begins + commits
// its own tx).
func (s *Store) ReplaceWikiGearTier(ctx context.Context, rows []WikiGearTier) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin wiki_gear_tier replace tx: %w", err)
	}
	defer tx.Rollback()
	if err := ReplaceWikiGearTierTx(ctx, tx, rows); err != nil {
		return err
	}
	return tx.Commit()
}

// ReplaceWikiGearTierTx does a FULL-TABLE replace inside the caller's tx: DELETE
// every row (no WHERE) then INSERT all rows. This is mandatory because the
// declared UNIQUE(tier,class,slot,item_id) never fires — item_id is always NULL
// and SQLite treats NULLs as distinct in a UNIQUE constraint, so a per-row upsert
// would duplicate every row on every weekly run (Pitfall 1). Only ~1000 rows from
// 2 wiki pages, so a full replace is trivial and Sheet-faithful (replaceAllWikiGearTier).
func ReplaceWikiGearTierTx(ctx context.Context, tx *sql.Tx, rows []WikiGearTier) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM wiki_gear_tier`); err != nil {
		slog.Error("wiki_gear_tier replace: delete", "err", err)
		return fmt.Errorf("delete wiki_gear_tier: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO wiki_gear_tier
		(tier, class, slot, item_id, item_name, rank, last_refreshed)
		VALUES (?,?,?,?,?,?,?)`)
	if err != nil {
		return fmt.Errorf("prepare wiki_gear_tier insert: %w", err)
	}
	defer stmt.Close()

	for i, r := range rows {
		if _, err := stmt.ExecContext(ctx, r.Tier, r.Class, r.Slot, r.ItemID, r.ItemName, r.Rank, r.LastRefreshed); err != nil {
			slog.Error("wiki_gear_tier replace: insert", "row_ordinal", i, "err", err)
			return fmt.Errorf("insert wiki_gear_tier (row_ordinal=%d): %w", i, err)
		}
	}
	slog.Info("wiki_gear_tier replace", "rows", len(rows))
	return nil
}

// ReplaceQuestItemsForID replaces all quest_items rows for itemID (begins +
// commits its own tx).
func (s *Store) ReplaceQuestItemsForID(ctx context.Context, itemID int64, rows []QuestItem) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin quest_items replace tx (item_id=%d): %w", itemID, err)
	}
	defer tx.Rollback()
	if err := ReplaceQuestItemsForIDTx(ctx, tx, itemID, rows); err != nil {
		return err
	}
	return tx.Commit()
}

// ReplaceQuestItemsForIDTx does a per-item-id DELETE+INSERT inside the caller's
// tx (D-12): every existing quest link for itemID is deleted, then the given
// links are inserted. Links for OTHER item_ids are untouched (scoped DELETE).
// Mirrors the Sheet's replaceQuestItemRowsForId.
func ReplaceQuestItemsForIDTx(ctx context.Context, tx *sql.Tx, itemID int64, rows []QuestItem) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM quest_items WHERE item_id = ?`, itemID); err != nil {
		slog.Error("quest_items replace: delete", "item_id", itemID, "err", err)
		return fmt.Errorf("delete quest_items (item_id=%d): %w", itemID, err)
	}

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO quest_items
		(item_id, quest_name, source_url, source, last_refreshed)
		VALUES (?,?,?,?,?)`)
	if err != nil {
		return fmt.Errorf("prepare quest_items insert (item_id=%d): %w", itemID, err)
	}
	defer stmt.Close()

	for _, r := range rows {
		if _, err := stmt.ExecContext(ctx, r.ItemID, r.QuestName, r.SourceURL, r.Source, r.LastRefreshed); err != nil {
			slog.Error("quest_items replace: insert", "item_id", itemID, "err", err)
			return fmt.Errorf("insert quest_items (item_id=%d, quest=%q): %w", itemID, r.QuestName, err)
		}
	}
	slog.Info("quest_items replace", "item_id", itemID, "rows", len(rows))
	return nil
}
