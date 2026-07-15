package store

// portrait.go is the Phase 41 plan 41-01 per-character portrait blob data layer
// (CHARUI-02). It stores the image bytes in the character_portrait side table (00019)
// so the hot roster/inventory reads never pull the blob, and gates every WRITE
// (upsert + delete) with the D-05/D-06 assignee-OR-officer check composed UNDER the
// caller's tx (WR-04 TOCTOU) — the same authorize-under-transaction posture the
// assignment.go officer mutators use, but ORed rather than officer-only:
//
//   - ASSIGNEE-OR-OFFICER (D-05): the character's assignee (IsCharAssignedToTx) OR any
//     officer (isOfficerTx) may set/remove a portrait. A stranger → ErrNotAuthorized.
//   - BANK/BOT OFFICER-ONLY (D-06): a guild bank/bot (charSharedTx=true) has NO assignee,
//     so ONLY an officer may set its portrait — the OR's assignee branch is skipped when
//     the char is shared.
//   - GATE-BEFORE-WRITE: authorizePortraitWriteTx runs BEFORE the DELETE so a stranger
//     cannot probe/remove another char's portrait (no existence leak).
//   - MAGIC-BYTE-AGNOSTIC STORAGE: the store trusts the caller's already-sniffed
//     contentType (the webadmin handler does the PNG/JPEG/WebP magic-byte sniff + the
//     256KB cap BEFORE this call, D-04) — this layer just persists the bytes + type.
//
// Sentinels: ErrPortraitNotFound is declared here; ErrCharNotFound (charmeta.go) and
// ErrNotAuthorized (admins.go) are REUSED — never redefined. Parameterized ? placeholders
// ONLY (V5). All timestamps come from the passed now string (the store column is TEXT ISO,
// distinct from the audit-log int now the handler passes to AppendAuditTx). No slog of the
// char name or the blob content (V7).

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrPortraitNotFound is returned by GetPortrait/PortraitMeta callers-of-record when a
// character has no stored portrait (the common case — an absent portrait is not an error
// for the flag read, but the serve path maps it to a 404). Mirrors the charmeta.go/
// assignment.go typed-sentinel style.
var ErrPortraitNotFound = errors.New("portrait_not_found")

// resolveCharIDTx resolves the character named charName to its id on the tx snapshot
// (binding.go:63-64 pattern; character.name is UNIQUE COLLATE NOCASE ⇒ case-insensitive,
// a single indexed SELECT, V5). An unknown/removed name → ErrCharNotFound.
func resolveCharIDTx(ctx context.Context, tx *sql.Tx, charName string) (int64, error) {
	var charID int64
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM character WHERE name = ?`, charName).Scan(&charID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return 0, ErrCharNotFound
	case err != nil:
		return 0, fmt.Errorf("resolve char id: %w", err)
	}
	return charID, nil
}

// authorizePortraitWriteTx is the D-05/D-06 assignee-OR-officer gate, composed UNDER the
// caller's tx (WR-04 TOCTOU) — the OfficerAssignTx:264-278 posture, but ORed and with the
// D-06 bank/bot flip: a shared char (bank/bot, no assignee) is officer-ONLY, a normal char
// is assignee-OR-officer. A non-authorized caller → ErrNotAuthorized (reused, admins.go).
func authorizePortraitWriteTx(ctx context.Context, tx *sql.Tx, charID int64, callerID string) error {
	shared, err := charSharedTx(ctx, tx, charID) // assignment.go:97 — bank/bot has no assignee (D-06)
	if err != nil {
		return err
	}
	assigned := false
	if !shared {
		assigned, err = IsCharAssignedToTx(ctx, tx, charID, callerID) // assignment.go:119
		if err != nil {
			return err
		}
	}
	if !assigned {
		ok, err := isOfficerTx(ctx, tx, callerID) // admins.go:85 — in-tx officer re-check (WR-04)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotAuthorized
		}
	}
	return nil
}

// SetPortraitTx upserts the portrait blob for charName (D-02: one row per char, PK =
// character_id). Flow: resolve name→id → authorize-under-tx (assignee OR officer) → upsert
// (INSERT … ON CONFLICT(character_id) DO UPDATE, the OfficerAssignTx:279-286 shape). blob +
// contentType are the ALREADY-VALIDATED bytes + sniffed type from the handler (D-04). now is
// the TEXT ISO updated_at (distinct from the audit int now). An unknown char → ErrCharNotFound;
// a stranger → ErrNotAuthorized (no write). Composed inside the caller's withTx alongside the
// portrait_set audit row so they land atomically.
func SetPortraitTx(ctx context.Context, tx *sql.Tx, charName string, blob []byte, contentType, callerID, now string) error {
	charID, err := resolveCharIDTx(ctx, tx, charName)
	if err != nil {
		return err
	}
	if err := authorizePortraitWriteTx(ctx, tx, charID, callerID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO character_portrait (character_id, image_blob, content_type, byte_size, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(character_id) DO UPDATE SET
		   image_blob   = excluded.image_blob,
		   content_type = excluded.content_type,
		   byte_size    = excluded.byte_size,
		   updated_at   = excluded.updated_at`,
		charID, blob, contentType, len(blob), now,
	); err != nil {
		return fmt.Errorf("set portrait (character_id=%d): %w", charID, err)
	}
	return nil
}

// DeletePortraitTx removes charName's portrait (D-08). Flow: resolve name→id → authorize-
// under-tx (assignee OR officer, the SAME gate — runs BEFORE the DELETE so a stranger cannot
// probe/remove) → DELETE. Idempotent: a missing portrait affects 0 rows and is NOT an error
// (revert-to-placeholder is the same outcome whether or not one existed). An unknown char →
// ErrCharNotFound; a stranger → ErrNotAuthorized (no delete).
func DeletePortraitTx(ctx context.Context, tx *sql.Tx, charName, callerID string) error {
	charID, err := resolveCharIDTx(ctx, tx, charName)
	if err != nil {
		return err
	}
	if err := authorizePortraitWriteTx(ctx, tx, charID, callerID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM character_portrait WHERE character_id = ?`, charID,
	); err != nil {
		return fmt.Errorf("delete portrait (character_id=%d): %w", charID, err)
	}
	return nil
}

// GetPortrait reads charName's stored blob + sniffed content_type for the serve path — a
// *sql.DB read (NO tx: a guild-wide member GET, gated at the route by RequireSession, not
// per-character ownership). One name→blob join; an absent portrait (or unknown char) →
// (nil, "", ErrPortraitNotFound) — the handler maps it to 404 (never an existence leak). The
// content_type is the STORED sniffed value, so the serve handler can set it verbatim (never
// a client claim). V5: name binds only as a `?`. V7: no name/bytes ever logged here.
func (s *Store) GetPortrait(ctx context.Context, charName string) ([]byte, string, error) {
	var blob []byte
	var ct string
	err := s.db.QueryRowContext(ctx,
		`SELECT p.image_blob, p.content_type
		   FROM character_portrait p
		   JOIN character c ON c.id = p.character_id
		  WHERE c.name = ?`, charName).Scan(&blob, &ct)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, "", ErrPortraitNotFound
	case err != nil:
		return nil, "", fmt.Errorf("get portrait: %w", err)
	}
	return blob, ct, nil
}

// PortraitMeta reports whether charName has a portrait and its updated_at (for the inventory
// payload flag, D-07). Missing → (false, "", nil) — NOT an error (an absent portrait is the
// common case). PK↔PK 1:1 join (character.id / character_portrait.character_id) — CANNOT fan
// out (unlike the name-keyed catalog joins, P39 memory), so NO aggregation. A *sql.DB read;
// V5 `?` bind; V7 no name/bytes logged.
func (s *Store) PortraitMeta(ctx context.Context, charName string) (bool, string, error) {
	var updatedAt string
	err := s.db.QueryRowContext(ctx,
		`SELECT p.updated_at FROM character_portrait p
		   JOIN character c ON c.id = p.character_id
		  WHERE c.name = ?`, charName).Scan(&updatedAt)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return false, "", nil
	case err != nil:
		return false, "", fmt.Errorf("portrait meta: %w", err)
	}
	return true, updatedAt, nil
}
