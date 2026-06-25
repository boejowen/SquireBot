package store

// backfill.go is the ONE-TIME, no-network item-flags backfill (Phase 37 / D-05).
// When migration 00016 adds the nine flag/effect columns, every ALREADY-enriched
// item_master row has them NULL even though its statsblock TEXT (00013) already
// contains everything needed to derive them. Rather than wait for the weekly wiki
// crawl to revisit each item, BackfillItemFlags re-parses the STORED statsblock at
// boot — pure CPU, zero wiki refetch — so flags/effects "light up now".
//
// This is the ONE place store imports enrich: the backfill MUST re-use the live
// parser's enrich.DeriveFlagsAndEffects so the derivation is identical to the
// weekly path (no second, drifting copy of the flag/clicky/haste rules). enrich is
// the pure parser package (no DB, no store import), so store→enrich is acyclic.
//
// V5 (Tampering): every derived value is bound through a ? placeholder — the parsed
// names/flags are UNTRUSTED and are NEVER string-concatenated into SQL. V7 (info
// disclosure): slog logs counts/item_id/err only, never raw statsblock/flag content.

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/boejowen/SquireBot/internal/backendsrv/enrich"
)

// backfillSelect reads the rows that still need backfilling: a non-empty stored
// statsblock to re-parse AND flags_json still NULL. The flags_json IS NULL guard is
// the idempotency key — once a row is backfilled (flags_json becomes "[]" or a real
// array, never NULL), it drops out of this set, so a second boot scans it again but
// updates nothing. A row with no statsblock has nothing to derive and is skipped.
const backfillSelect = `SELECT item_id, statsblock
	FROM item_master
	WHERE statsblock IS NOT NULL AND statsblock != '' AND flags_json IS NULL`

// backfillUpdate writes the nine derived columns for one item_id. flags_json is
// produced by the SAME MarshalFlags the upsert + freshness compare use, so an empty
// flag set stores "[]" (never NULL) and byte-matches the weekly freshness check —
// the row counts as backfilled and is not re-scanned or re-written next pass (D-06).
const backfillUpdate = `UPDATE item_master SET
	is_lore=?, is_no_drop=?, is_magic=?, is_temporary=?,
	is_clicky=?, clicky_effect=?, has_haste=?, haste_pct=?, flags_json=?
	WHERE item_id=?`

// BackfillItemFlags re-derives the Phase 37 flag/effect columns from each row's
// STORED statsblock (D-05) — a one-time, no-network, idempotent boot pass. It scans
// every item_master row that has a statsblock but a NULL flags_json, runs
// enrich.DeriveFlagsAndEffects on the stored (cleaned, bracket-stripped, newline-
// separated) block, and UPDATEs the nine columns in ONE transaction (the table is
// small — held items only, pre-P38). Returns (scanned, updated, err): scanned is the
// number of candidate rows, updated the number actually re-written (== scanned on a
// first run, 0 on every subsequent run since flags_json is now populated). A DB error
// rolls the whole pass back; the caller (main.go) logs it and continues serving — the
// weekly freshness pass heals anything the backfill missed.
func (s *Store) BackfillItemFlags(ctx context.Context) (scanned, updated int, err error) {
	tx, err := s.db.BeginTx(ctx, nil) // _txlock=immediate ⇒ BEGIN IMMEDIATE
	if err != nil {
		return 0, 0, fmt.Errorf("begin item-flags backfill tx: %w", err)
	}
	defer tx.Rollback() // no-op after Commit

	rows, err := tx.QueryContext(ctx, backfillSelect)
	if err != nil {
		return 0, 0, fmt.Errorf("select item-flags backfill candidates: %w", err)
	}
	// Materialize the candidates first so the UPDATEs don't run while the SELECT
	// cursor is still open on the same single connection.
	type cand struct {
		itemID     int64
		statsblock string
	}
	var cands []cand
	for rows.Next() {
		var c cand
		if scanErr := rows.Scan(&c.itemID, &c.statsblock); scanErr != nil {
			rows.Close()
			return 0, 0, fmt.Errorf("scan item-flags backfill candidate: %w", scanErr)
		}
		cands = append(cands, c)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		rows.Close()
		return 0, 0, fmt.Errorf("iterate item-flags backfill candidates: %w", rowsErr)
	}
	rows.Close()

	scanned = len(cands)
	for _, c := range cands {
		d := enrich.DeriveFlagsAndEffects(c.statsblock) // pure, no network (D-05)
		if _, uerr := tx.ExecContext(ctx, backfillUpdate,
			b2i(d.IsLore), b2i(d.IsNoDrop), b2i(d.IsMagic), b2i(d.IsTemporary),
			b2i(d.IsClicky), d.ClickyEffect, b2i(d.HasHaste), d.HastePct,
			MarshalFlags(d.Flags), // empty → "[]" (idempotent / byte-matches freshness)
			c.itemID,
		); uerr != nil {
			return scanned, updated, fmt.Errorf("backfill item flags (item_id=%d): %w", c.itemID, uerr)
		}
		updated++
	}

	if err := tx.Commit(); err != nil {
		return scanned, updated, fmt.Errorf("commit item-flags backfill: %w", err)
	}
	slog.Info("item flags backfill", "scanned", scanned, "updated", updated)
	return scanned, updated, nil
}
