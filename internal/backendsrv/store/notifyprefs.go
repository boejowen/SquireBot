package store

// notifyprefs.go is the Phase 20 (WANT-04) per-user notification-preferences
// store layer. It mirrors wantlist.go's grain: an owner-scoped *Tx upserter +
// a plain *sql.DB reader, %w-wrapped errors, parameterized ? placeholders.
//
// D-01 default-ON: a guildie is alerted by DEFAULT once monitors go live —
// adding a want IS the opt-in. So an ABSENT notify_prefs row reads as ALL-ON;
// GetPrefs returns NotifyPrefs{true,true,true,true} on sql.ErrNoRows rather
// than treating "no row" as "everything off". A row is only written when the
// user explicitly changes a toggle (UpsertPrefsTx).
//
// Identity keys on web_user.discord_user_id — the PERSON, resolved from the
// session upstream, NEVER the request body (v2.1 D-02 / the wantlist.go twin).

import (
	"context"
	"database/sql"
	"fmt"
)

// NotifyPrefs is a guildie's notification opt-in state: a master toggle plus the
// three per-monitor toggles (EC-tunnel auctions / WTS cross-server / raid
// targets — D-02). The JSON tags match the frontend NotifyPrefs interface
// (api.ts); the booleans map to the notify_prefs INTEGER 0/1 columns.
type NotifyPrefs struct {
	Master bool `json:"master"`
	EC     bool `json:"ec"`
	WTS    bool `json:"wts"`
	Raid   bool `json:"raid"`
}

// GetPrefs returns the caller's notification preferences, OWNER-SCOPED (WHERE
// discord_user_id = ?). On sql.ErrNoRows it returns the D-01 default-ON prefs
// (all four true) rather than an error: an absent row means the guildie has
// never touched a toggle, which is "alert me about everything" (adding a want
// was the opt-in). discordID MUST come from the session upstream, never the body.
func GetPrefs(ctx context.Context, db *sql.DB, discordID string) (NotifyPrefs, error) {
	var master, ec, wts, raid int
	err := db.QueryRowContext(ctx,
		`SELECT master, ec, wts, raid FROM notify_prefs WHERE discord_user_id = ?`,
		discordID).Scan(&master, &ec, &wts, &raid)
	if err == sql.ErrNoRows {
		// D-01: absent row ⇒ all-ON (default-ON opt-in).
		return NotifyPrefs{Master: true, EC: true, WTS: true, Raid: true}, nil
	}
	if err != nil {
		return NotifyPrefs{}, fmt.Errorf("get prefs (user=%s): %w", discordID, err)
	}
	return NotifyPrefs{
		Master: master != 0,
		EC:     ec != 0,
		WTS:    wts != 0,
		Raid:   raid != 0,
	}, nil
}

// UpsertPrefsTx writes the caller's notification preferences, OWNER-SCOPED to
// discordID (resolved from the session upstream — NEVER the body, D-02). It is an
// INSERT … ON CONFLICT(discord_user_id) DO UPDATE so the first toggle change
// creates the row and every later change overwrites it. The four booleans are
// stored as INTEGER 0/1. now is the epoch-secs write timestamp.
func UpsertPrefsTx(ctx context.Context, tx *sql.Tx, discordID string, p NotifyPrefs, now int64) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO notify_prefs (discord_user_id, master, ec, wts, raid, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(discord_user_id) DO UPDATE SET
		   master=excluded.master, ec=excluded.ec, wts=excluded.wts,
		   raid=excluded.raid, updated_at=excluded.updated_at`,
		discordID, b2i(p.Master), b2i(p.EC), b2i(p.WTS), b2i(p.Raid), now)
	if err != nil {
		return fmt.Errorf("upsert prefs (user=%s): %w", discordID, err)
	}
	return nil
}

// b2i converts a bool to the INTEGER 0/1 the notify_prefs/alert_log columns use.
func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
