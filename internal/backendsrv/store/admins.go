package store

// admins.go is the SQLite port of v1's admin-policy module
// (apps-script/src/lib/admin.ts — the behavioral oracle, ADMIN-06 / D-06/D-07/
// D-08). It ports the SEMANTICS, not the storage shape: v1's `_meta.guild_admins`
// JSON array + `_meta.workbook_owner_floor` string become the guild_admins table
// (keyed by Discord snowflake) + the app_config['owner_floor_discord_id'] row.
//
// Ported invariants (verbatim behavior):
//   - FAIL-CLOSED authorization: an empty/unknown caller is never an officer
//     (IsOfficer "" → false; the v1 requireAdminOrThrow contract).
//   - AUTHORIZE-UNDER-TRANSACTION (v1 WR-04 TOCTOU fix): AddOfficerTx /
//     RemoveOfficerTx re-check the caller's officer status as their FIRST SELECT
//     inside the *sql.Tx, so an officer removed in a racing tx cannot land one
//     final write. The HTTP call site (15-02/15-03) opens the tx and passes it in.
//   - IDEMPOTENT add/remove: INSERT OR IGNORE / DELETE; a no-op reports added=
//     false / removed=false with no error (v1's alreadyExists / notFound).
//   - OWNER-FLOOR PROTECTION: a peer (caller != floor) cannot remove the floor
//     (ErrOwnerFloorProtected, checked BEFORE any write); self-removal of the
//     floor is allowed and does NOT clear the app_config floor row (v1's
//     documented orphan-pointer rule, admin.ts:241-245).
//
// Error strings match v1 exactly ('not_authorized' / 'owner_floor_protected')
// so the 15-02 handler can map them to the same client messages. Parameterized
// ? placeholders ONLY (V5 / T-15-03).

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrNotAuthorized is returned by the *Tx mutators when the caller is not an
// officer (fail-closed). The string matches v1 admin.ts requireAdminOrThrow's
// Error('not_authorized') so the handler maps it identically. T-15-02.
var ErrNotAuthorized = errors.New("not_authorized")

// ErrOwnerFloorProtected is returned by RemoveOfficerTx when a peer (caller !=
// floor) tries to remove the owner-floor. Matches v1 admin.ts's
// Error('owner_floor_protected'). Checked BEFORE any write. T-15-02.
var ErrOwnerFloorProtected = errors.New("owner_floor_protected")

// ownerFloorKey is the app_config key under which the CLI-seeded owner-floor
// Discord id lives (D-08). One key, generic table — future single-value config
// reuses app_config.
const ownerFloorKey = "owner_floor_discord_id"

// Officer is one row of the officer list (ListOfficers) or the promotable-user
// list (ListPromotableUsers), joined to web_user for display. IsFloor flags the
// un-removable owner-floor (always false in the promotable list). snake_case
// JSON tags — this struct crosses the API boundary in 15-03.
type Officer struct {
	DiscordUserID string  `json:"discord_user_id"`
	Username      string  `json:"username"`
	Avatar        *string `json:"avatar"`
	IsFloor       bool    `json:"is_floor"`
}

// IsOfficer reports whether discordUserID is in the guild_admins allowlist. An
// empty id is fail-closed → (false, nil) without a query (v1's normalizeEmail→""
// → not an admin). This is the read used by both the request-time gate (15-02)
// and the authorize-under-tx re-check (via isOfficerTx below).
func IsOfficer(ctx context.Context, db *sql.DB, discordUserID string) (bool, error) {
	if discordUserID == "" {
		return false, nil // fail-closed
	}
	var one int
	err := db.QueryRowContext(ctx,
		`SELECT 1 FROM guild_admins WHERE discord_user_id = ?`, discordUserID,
	).Scan(&one)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("check officer (discord_user_id=%q): %w", discordUserID, err)
	}
	return true, nil
}

// isOfficerTx is the authorize-under-transaction re-check (v1 WR-04): the same
// fail-closed officer test as IsOfficer, but executed on the caller-supplied
// *sql.Tx so it sees the tx's snapshot. AddOfficerTx/RemoveOfficerTx call this
// as their FIRST statement.
func isOfficerTx(ctx context.Context, tx *sql.Tx, discordUserID string) (bool, error) {
	if discordUserID == "" {
		return false, nil // fail-closed
	}
	var one int
	err := tx.QueryRowContext(ctx,
		`SELECT 1 FROM guild_admins WHERE discord_user_id = ?`, discordUserID,
	).Scan(&one)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("check officer in tx (discord_user_id=%q): %w", discordUserID, err)
	}
	return true, nil
}

// GetOwnerFloor reads the CLI-seeded owner-floor Discord id from
// app_config['owner_floor_discord_id'] ("" when unset — the floor has not been
// seeded yet). D-08.
func GetOwnerFloor(ctx context.Context, db *sql.DB) (string, error) {
	var v string
	err := db.QueryRowContext(ctx,
		`SELECT value FROM app_config WHERE key = ?`, ownerFloorKey,
	).Scan(&v)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return "", nil // unset
	case err != nil:
		return "", fmt.Errorf("get owner floor: %w", err)
	}
	return v, nil
}

// getOwnerFloorTx is the tx-scoped owner-floor read used by RemoveOfficerTx so
// the floor check sees the same snapshot as the authorize-under-tx re-check.
func getOwnerFloorTx(ctx context.Context, tx *sql.Tx) (string, error) {
	var v string
	err := tx.QueryRowContext(ctx,
		`SELECT value FROM app_config WHERE key = ?`, ownerFloorKey,
	).Scan(&v)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return "", nil
	case err != nil:
		return "", fmt.Errorf("get owner floor in tx: %w", err)
	}
	return v, nil
}

// ListOfficers returns every officer joined to their web_user display fields,
// with IsFloor set on the owner-floor row. Ordered by username for a stable UI.
func ListOfficers(ctx context.Context, db *sql.DB) ([]Officer, error) {
	floor, err := GetOwnerFloor(ctx, db)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		`SELECT a.discord_user_id, u.username, u.avatar
		   FROM guild_admins a
		   JOIN web_user u ON u.discord_user_id = a.discord_user_id
		  ORDER BY u.username COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list officers: %w", err)
	}
	defer rows.Close()

	var out []Officer
	for rows.Next() {
		var o Officer
		var av sql.NullString
		if err := rows.Scan(&o.DiscordUserID, &o.Username, &av); err != nil {
			return nil, fmt.Errorf("scan officer row: %w", err)
		}
		if av.Valid {
			v := av.String
			o.Avatar = &v
		}
		o.IsFloor = o.DiscordUserID == floor && floor != ""
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate officers: %w", err)
	}
	return out, nil
}

// ListPromotableUsers returns the web_user rows NOT already in guild_admins —
// the D-07 "promote by pick" source for the admin-mgmt form (no snowflakes
// typed). IsFloor is always false here (the floor is, by definition, already an
// officer). Ordered by username.
func ListPromotableUsers(ctx context.Context, db *sql.DB) ([]Officer, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT u.discord_user_id, u.username, u.avatar
		   FROM web_user u
		  WHERE u.discord_user_id NOT IN (SELECT discord_user_id FROM guild_admins)
		  ORDER BY u.username COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list promotable users: %w", err)
	}
	defer rows.Close()

	var out []Officer
	for rows.Next() {
		var o Officer
		var av sql.NullString
		if err := rows.Scan(&o.DiscordUserID, &o.Username, &av); err != nil {
			return nil, fmt.Errorf("scan promotable row: %w", err)
		}
		if av.Valid {
			v := av.String
			o.Avatar = &v
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate promotable users: %w", err)
	}
	return out, nil
}

// SetOwnerFloor designates discordUserID as the un-removable owner-floor (D-08,
// the `set-owner-floor` CLI). It upserts app_config['owner_floor_discord_id']
// AND makes the floor the bootstrap officer (INSERT OR IGNORE into guild_admins).
//
// FK choice (documented): guild_admins.discord_user_id REFERENCES web_user, so
// if the floor has not logged in yet we INSERT a minimal placeholder web_user
// (username = the snowflake, first_seen/last_login = now) — INSERT OR IGNORE so
// a real prior login is never clobbered. This keeps the FK valid and surfaces
// the floor in ListOfficers immediately, replacing v1's onOpen/getOwner()
// bootstrap (which no longer exists in the DB world). On the floor's first real
// login, UpsertWebUser refreshes the placeholder username/avatar in place.
func SetOwnerFloor(ctx context.Context, db *sql.DB, discordUserID string, now int64) error {
	if discordUserID == "" {
		return fmt.Errorf("set owner floor: empty discord_user_id")
	}
	// Minimal placeholder web_user so the guild_admins FK holds even pre-login.
	if _, err := db.ExecContext(ctx,
		`INSERT OR IGNORE INTO web_user (discord_user_id, username, avatar, first_seen, last_login)
		 VALUES (?, ?, NULL, ?, ?)`,
		discordUserID, discordUserID, now, now,
	); err != nil {
		return fmt.Errorf("seed placeholder web_user for floor (discord_user_id=%q): %w", discordUserID, err)
	}
	// Upsert the floor pointer.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO app_config (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
		ownerFloorKey, discordUserID, now,
	); err != nil {
		return fmt.Errorf("upsert owner floor pointer: %w", err)
	}
	// Bootstrap officer (idempotent).
	if _, err := db.ExecContext(ctx,
		`INSERT OR IGNORE INTO guild_admins (discord_user_id, added_at, added_by) VALUES (?, ?, 'cli')`,
		discordUserID, now,
	); err != nil {
		return fmt.Errorf("seed floor as bootstrap officer (discord_user_id=%q): %w", discordUserID, err)
	}
	return nil
}

// AddOfficerTx promotes targetID to officer, composed inside the caller's tx so
// the authorization is TOCTOU-safe (v1 WR-04). Flow:
//  1. FIRST: re-check the caller is an officer on the tx snapshot →
//     ErrNotAuthorized if not (fail-closed).
//  2. INSERT OR IGNORE the target (idempotent): added=true on a fresh insert,
//     added=false if the row already existed (v1's alreadyExists, no write).
//
// added_by records the promoting officer's id. The web_user row for targetID
// must already exist (D-07: you can only promote someone who has logged in) —
// the FK enforces it; a missing target surfaces as an INSERT error.
func AddOfficerTx(ctx context.Context, tx *sql.Tx, targetID, callerID string, now int64) (added bool, err error) {
	ok, err := isOfficerTx(ctx, tx, callerID) // authorize UNDER the tx (WR-04)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, ErrNotAuthorized
	}
	res, err := tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO guild_admins (discord_user_id, added_at, added_by) VALUES (?, ?, ?)`,
		targetID, now, callerID,
	)
	if err != nil {
		return false, fmt.Errorf("add officer (target=%q): %w", targetID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("add officer rows-affected (target=%q): %w", targetID, err)
	}
	return n > 0, nil // n==0 ⇒ already existed (idempotent, no-op)
}

// RemoveOfficerTx demotes targetID, composed inside the caller's tx (WR-04).
// Flow:
//  1. FIRST: re-check the caller is an officer → ErrNotAuthorized if not.
//  2. Read the owner-floor; if targetID==floor && callerID!=floor →
//     ErrOwnerFloorProtected BEFORE any write (v1's owner-floor protection).
//  3. DELETE the target (idempotent): removed=true on a real delete,
//     removed=false if absent (v1's notFound, no write).
//
// Self-removal of the floor is allowed and does NOT clear the app_config floor
// row — v1's documented orphan-pointer rule (admin.ts:241-245): the floor row is
// the "who is protected" pointer, not the "who is currently an officer" pointer.
func RemoveOfficerTx(ctx context.Context, tx *sql.Tx, targetID, callerID string, now int64) (removed bool, err error) {
	ok, err := isOfficerTx(ctx, tx, callerID) // authorize UNDER the tx (WR-04)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, ErrNotAuthorized
	}
	floor, err := getOwnerFloorTx(ctx, tx)
	if err != nil {
		return false, err
	}
	// Owner-floor protection — BEFORE any write (v1 admin.ts:222-229).
	if floor != "" && targetID == floor && callerID != floor {
		return false, ErrOwnerFloorProtected
	}
	res, err := tx.ExecContext(ctx,
		`DELETE FROM guild_admins WHERE discord_user_id = ?`, targetID,
	)
	if err != nil {
		return false, fmt.Errorf("remove officer (target=%q): %w", targetID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("remove officer rows-affected (target=%q): %w", targetID, err)
	}
	// NOTE: app_config floor pointer is INTENTIONALLY left intact on self-removal
	// of the floor (documented orphan, matches v1).
	return n > 0, nil // n==0 ⇒ not found (idempotent, no-op)
}
