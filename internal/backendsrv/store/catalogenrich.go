package store

// catalogenrich.go is the single tested SQL path for Phase 38's NAME-KEYED
// catalog enrichment (ENRICH-14/15, D-04 reversed → name-keyed). Per the 11-05
// single-tested-SQL-path rule (WARNING-3), the wiki enrichment job authors ZERO
// inline INSERT/UPDATE SQL — it calls the exported *Tx methods here and composes
// them over one *sql.Tx, exactly as the held path composes UpsertItemMasterTx +
// GetItemMasterFreshnessTx over a tx.
//
// This file is the EXACT parallel of enrich.go's held item_master path, re-keyed
// from item_id (the EQ-inventory namespace) onto norm_name = lower(trim(name)) (the
// cross-namespace bridge). A catalog-only (unheld) item has NO EQ inventory item_id
// to key an item_master row by — the PigParse catalog id is a DIFFERENT namespace and
// numerically COLLIDES with real EQ ids — so its enrichment lives here, name-keyed, in
// catalog_enrichment (migration 00017). item_master stays held-only, keyed by EQ
// item_id, byte-for-byte unchanged — held readers (Phases 31/32/37) have zero blast
// radius (none of them read catalog_enrichment).
//
// Every value is bound through a ? placeholder (V5 / Tampering): parsed item names,
// wikitext, slot, clicky_effect, flags_json, statsblock are UNTRUSTED wiki text and are
// NEVER string-concatenated into a SQL literal. The booleans bind as 0/1 via the
// package-local b2i helper (notifyprefs.go). slog logs counts / norm_names / err only —
// never raw content (V7). flags_json is ALWAYS produced by store.MarshalFlags so a
// flagless item stores "[]" and byte-equals the freshness compare (D-06 idempotency).

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
)

// CatalogEnrichment is the store-local input shape for one catalog_enrichment row
// (the wiki item-summary job's output for a CATALOG-ONLY item). It mirrors ItemMaster
// (enrich.go) but is keyed by NormName = lower(trim(name)) instead of an EQ item_id,
// and carries a representative Name + PigParse ItemID as non-key columns. FlagsJSON is
// ALWAYS produced by store.MarshalFlags (never a local json.Marshal) so a flagless item
// is "[]", byte-equal to the freshness compare (D-06).
type CatalogEnrichment struct {
	NormName    string // lower(trim(name)) — the PK / cross-namespace key
	Name        string // representative display name (first-seen casing)
	ItemID      int    // representative PigParse id (examine/icon URL); NOT a key
	WikiSummary string
	WikiURL     string
	Slot        string
	IsQuestItem bool

	WikitextSHA1  string
	LastRefreshed string
	IconID        int    // the P1999 wiki icon id (lucy_img_ID); 0 = none yet (INV-04, 00012)
	Statsblock    string // the cleaned in-game stat block for the examine; "" when absent

	// Phase 37 derived flags (ENRICH-12 / D-03) — the four queried booleans.
	IsLore      bool
	IsNoDrop    bool
	IsMagic     bool
	IsTemporary bool
	// Phase 37 derived effects (ENRICH-13 / D-01 / D-02).
	IsClicky     bool   // the Effect line is an ACTIVATABLE click (NOT (Worn)/(Combat))
	ClickyEffect string // the clicky's effect/spell display name; "" unless IsClicky
	HasHaste     bool   // a "Haste:" stat line is present
	HastePct     int    // the integer haste % magnitude (0 when absent)
	// FlagsJSON is the FULL detected flag SET marshaled to a JSON array (D-03), ALWAYS
	// via store.MarshalFlags so the upsert + freshness compare byte-equal (D-06): a
	// flagless item is "[]", never NULL/null/"". The caller sets it via MarshalFlags(item.Flags).
	FlagsJSON string
}

// catalogEnrichmentUpsert is itemMasterUpsert (enrich.go) re-keyed on norm_name: it
// leads with norm_name, then name + item_id, then the same enrichment column set, and
// upserts via ON CONFLICT(norm_name) DO UPDATE. Every UNTRUSTED parsed value binds
// through a ? placeholder (V5); booleans via b2i 0/1.
const catalogEnrichmentUpsert = `INSERT INTO catalog_enrichment
	(norm_name, name, item_id, wiki_summary, wiki_url, slot, is_quest_item, wikitext_sha1,
	 icon_id, statsblock, is_lore, is_no_drop, is_magic, is_temporary, is_clicky,
	 clicky_effect, has_haste, haste_pct, flags_json, last_refreshed)
 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
 ON CONFLICT(norm_name) DO UPDATE SET
   name=excluded.name, item_id=excluded.item_id, wiki_summary=excluded.wiki_summary,
   wiki_url=excluded.wiki_url, slot=excluded.slot, is_quest_item=excluded.is_quest_item,
   wikitext_sha1=excluded.wikitext_sha1, icon_id=excluded.icon_id, statsblock=excluded.statsblock,
   is_lore=excluded.is_lore, is_no_drop=excluded.is_no_drop, is_magic=excluded.is_magic,
   is_temporary=excluded.is_temporary, is_clicky=excluded.is_clicky,
   clicky_effect=excluded.clicky_effect, has_haste=excluded.has_haste,
   haste_pct=excluded.haste_pct, flags_json=excluded.flags_json, last_refreshed=excluded.last_refreshed`

// UpsertCatalogEnrichment upserts one catalog_enrichment row (begins + commits its
// own tx). Mirrors UpsertItemMaster (enrich.go) for the catalog path.
func (s *Store) UpsertCatalogEnrichment(ctx context.Context, e CatalogEnrichment) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin catalog_enrichment upsert tx (norm_name=%q): %w", e.NormName, err)
	}
	defer tx.Rollback()
	if err := UpsertCatalogEnrichmentTx(ctx, tx, e); err != nil {
		return err
	}
	return tx.Commit()
}

// UpsertCatalogEnrichmentTx upserts a single catalog_enrichment row inside the
// caller's tx via ON CONFLICT(norm_name) DO UPDATE. The 4-field freshness
// short-circuit is the JOB's concern (it compares via GetCatalogEnrichmentFreshnessTx
// and skips calling this when unchanged). The bind order matches the column order in
// catalogEnrichmentUpsert; booleans bind as 0/1 via b2i; UNTRUSTED text (NormName,
// Name, WikiSummary, Slot, ClickyEffect, FlagsJSON, Statsblock) binds through ?
// placeholders — NEVER string-concatenated (V5 / Tampering).
func UpsertCatalogEnrichmentTx(ctx context.Context, tx *sql.Tx, e CatalogEnrichment) error {
	if _, err := tx.ExecContext(ctx, catalogEnrichmentUpsert,
		e.NormName, e.Name, e.ItemID, e.WikiSummary, e.WikiURL, e.Slot,
		b2i(e.IsQuestItem), e.WikitextSHA1, e.IconID, e.Statsblock,
		b2i(e.IsLore), b2i(e.IsNoDrop), b2i(e.IsMagic), b2i(e.IsTemporary),
		b2i(e.IsClicky), e.ClickyEffect, b2i(e.HasHaste), e.HastePct, e.FlagsJSON, e.LastRefreshed,
	); err != nil {
		slog.Error("catalog_enrichment upsert: insert", "norm_name", e.NormName, "err", err)
		return fmt.Errorf("upsert catalog_enrichment (norm_name=%q): %w", e.NormName, err)
	}
	return nil
}

// GetCatalogEnrichmentFreshnessTx returns the stored wikitext_sha1, icon_id, statsblock
// AND flags_json for normName (zero values when the row is absent or a column is NULL).
// This is the EXACT parallel of GetItemMasterFreshnessTx, re-keyed on norm_name: the
// catalog write path compares ALL FOUR (sha OR icon OR statsblock OR flags_json differs
// ⇒ re-write) so a row written BEFORE icon/stats/flags backfills on the NEXT weekly pass
// — the identical self-heal item_master gets, by name. SHA-1 alone is NOT sufficient
// (the same 00012-icon argument, one field at a time).
//
// The caller MUST compute the "freshly-parsed" flags_json it compares against via
// store.MarshalFlags (the SAME helper the upsert uses), so a flagless row's stored "[]"
// byte-equals the freshly-marshaled "[]" and is NOT mistaken for stale.
func GetCatalogEnrichmentFreshnessTx(ctx context.Context, tx *sql.Tx, normName string) (sha string, iconID int64, statsblock, flagsJSON string, err error) {
	var s, sb, fj sql.NullString
	var icon sql.NullInt64
	qerr := tx.QueryRowContext(ctx,
		`SELECT wikitext_sha1, icon_id, statsblock, flags_json FROM catalog_enrichment WHERE norm_name = ?`, normName).Scan(&s, &icon, &sb, &fj)
	switch {
	case qerr == sql.ErrNoRows:
		return "", 0, "", "", nil
	case qerr != nil:
		return "", 0, "", "", fmt.Errorf("read catalog_enrichment freshness (norm_name=%q): %w", normName, qerr)
	}
	return s.String, icon.Int64, sb.String, fj.String, nil // NullX zero-values when NULL
}
