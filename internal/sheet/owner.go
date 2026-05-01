package sheet

// UpsertCharOwner — _char_owner upsert per AUTH-06 + RESEARCH.md §12.5.
// Plan 01-05 Task 3.
//
// Conflict policy (RESEARCH.md §12.5):
//
//	first-write wins for owner_email
//	subsequent mismatches → slog.Warn (Phase 1) → _audit row (Phase 2 — AUTH-05)
//	inv:<Char> write itself is NOT gated on email match (per RESEARCH.md
//	"we don't gate writes on owner")
//
// Phase 1 schema (RESEARCH.md §12.5 + ARCHITECTURE.md _char_owner):
//
//	A: char_name
//	B: owner_email
//	C: display_name (blank — Phase 2 SCHEMA-05 populates)
//	D: discord_handle (blank — Phase 2 SCHEMA-05 populates)
//	E: first_seen (RFC3339 UTC, set on first sighting only — never overwritten)
//
// Schema is extend-only: Phase 2 will append class, level, is_bank_toon,
// is_hidden, is_removed, last_seen, server, watcher_version columns at
// the right edge without bumping schema_version (per CLAUDE.md
// "extend-only" rule).

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/api/sheets/v4"
)

// UpsertCharOwner appends a row `(charName, ownerEmail, "", "", first_seen)`
// to _char_owner if charName is not present. If charName is present and
// the existing email matches, this is a no-op. If charName is present
// and the existing email does NOT match, we log a slog.Warn and return
// nil — Phase 1 deliberately does not overwrite existing rows (first-write
// wins). Phase 2's _audit work surfaces the mismatch for officer review.
func (c *Client) UpsertCharOwner(ctx context.Context, charName, ownerEmail string) error {
	if c.spreadsheetID == "" {
		return fmt.Errorf("UpsertCharOwner: spreadsheetID not set")
	}
	if _, err := c.EnsureSheet(ctx, "_char_owner"); err != nil {
		return err
	}
	// Read columns A:B for upsert lookup. We don't need C-E for the
	// dedup check — they're only written on append.
	resp, err := c.svc.Spreadsheets.Values.
		Get(c.spreadsheetID, "_char_owner!A:B").
		Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("read _char_owner: %w", err)
	}
	for i, row := range resp.Values {
		if i == 0 {
			// Header row (if present). The master template MAY ship
			// with a header; the bootstrap fallback below works whether
			// it does or doesn't because the lookup is by char_name in
			// column A, never by row index.
			continue
		}
		if len(row) < 1 {
			continue
		}
		name, _ := row[0].(string)
		if name != charName {
			continue
		}
		var existingEmail string
		if len(row) >= 2 {
			existingEmail, _ = row[1].(string)
		}
		if existingEmail == ownerEmail {
			// Exact match → no-op. Saves an API call on every event.
			return nil
		}
		// Mismatch — log and proceed without overwriting. Phase 2's
		// _audit tab is where this becomes visible to officers.
		slog.Warn("char_owner email mismatch",
			"char", charName,
			"existing", existingEmail,
			"current", ownerEmail)
		return nil
	}
	// Not present → append new row. Use values.append with valueInputOption=RAW
	// so the email cell is never auto-linkified (USER_ENTERED would turn
	// "foo@example.com" into a hyperlink with display text "foo").
	body := &sheets.ValueRange{
		Values: [][]any{
			{
				charName,
				ownerEmail,
				"",                                  // C: display_name (Phase 2)
				"",                                  // D: discord_handle (Phase 2)
				time.Now().UTC().Format(time.RFC3339), // E: first_seen
			},
		},
	}
	if _, err := c.svc.Spreadsheets.Values.
		Append(c.spreadsheetID, "_char_owner!A:E", body).
		ValueInputOption("RAW").
		InsertDataOption("INSERT_ROWS").
		Context(ctx).Do(); err != nil {
		return fmt.Errorf("append _char_owner: %w", err)
	}
	return nil
}
