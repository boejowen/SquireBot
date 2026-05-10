package sheet

// UpsertCharOwner — _char_owner upsert per AUTH-06 + RESEARCH.md §12.5.
// Plan 01-05 Task 3, extended to 13 columns by Plan 02-01 Task 3,
// then to 14 columns by Plan 04-01 Task 1 (added `race`).
//
// Conflict policy (RESEARCH.md §12.5):
//
//	first-write wins for owner_email
//	subsequent mismatches → slog.Warn (Phase 1) → _audit row (Phase 2 — AUTH-05)
//	inv:<Char> write itself is NOT gated on email match (per RESEARCH.md
//	"we don't gate writes on owner")
//
// v1 schema (locked by Plan 02-01 ScaffoldSchemaV1; matches the
// internal/scaffold DimensionTabs[_char_owner].Headers exactly):
//
//	A: char_name
//	B: owner_email
//	C: display_name (blank — Phase 5+ populates)
//	D: discord_handle (blank — v2 deferred)
//	E: class (blank — Phase 4 sidebar populates)
//	F: level (blank — Phase 4 sidebar populates)
//	G: is_bank_toon (FALSE — Phase 4 sidebar populates)
//	H: is_hidden (FALSE — Phase 5 UI populates)
//	I: is_removed (FALSE — Phase 5 UI populates)
//	J: first_seen (RFC3339 UTC, set on first sighting only)
//	K: last_seen (RFC3339 UTC, refreshed on every UpsertCharOwner / heartbeat)
//	L: server ("blue" — P99 Blue is the only supported server)
//	M: watcher_version (from build-time Version constant; plumbed via
//	   auth.BuildConstants.WatcherVersion)
//
// Schema is extend-only: any future column appends at the right edge
// (column N+) and does NOT bump _meta.schema_version (per CLAUDE.md
// "extend-only" rule).

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/api/sheets/v4"
)

// CharOwnerServer is the only supported server value. P99 Blue is the
// only server SquireBot targets (per PROJECT.md Out of Scope).
const CharOwnerServer = "blue"

// UpsertCharOwner appends a 14-column row when charName is not present.
// On match (charName + ownerEmail both equal), refreshes column K
// (last_seen) only — preserves first_seen, class, level, soft-delete
// flags. On mismatch (charName present with different email), logs
// slog.Warn and returns nil (Phase 1 first-write-wins policy).
//
// watcherVersion is the build-time Version constant from cmd/squirebot,
// plumbed through auth.BuildConstants.WatcherVersion. An empty string
// is acceptable (writes "" into column M); callers wire the field via
// -ldflags='-X main.Version=...'.
func (c *Client) UpsertCharOwner(ctx context.Context, charName, ownerEmail, watcherVersion string) error {
	if c.spreadsheetID == "" {
		return fmt.Errorf("UpsertCharOwner: spreadsheetID not set")
	}
	if _, err := c.EnsureSheet(ctx, "_char_owner"); err != nil {
		return err
	}
	// Read columns A:B for upsert lookup. We don't need C-M for the
	// dedup check — they're only written on append / refresh.
	resp, err := c.valuesGet(ctx, "_char_owner!A:B")
	if err != nil {
		return fmt.Errorf("read _char_owner: %w", err)
	}
	for i, row := range resp.Values {
		if i == 0 {
			// Header row (if present). Lookup is by char_name in column A,
			// never by row index.
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
			// Match → refresh last_seen only (column K, 1-based row i+1).
			rowNum := i + 1
			rng := fmt.Sprintf("_char_owner!K%d", rowNum)
			now := time.Now().UTC().Format(time.RFC3339)
			body := &sheets.ValueRange{
				Values: [][]any{{now}},
			}
			if err := c.valuesUpdate(ctx, rng, body); err != nil {
				return fmt.Errorf("refresh last_seen %s: %w", rng, err)
			}
			return nil
		}
		// Mismatch — log and proceed without overwriting. Phase 2's
		// _audit tab is where this becomes visible to officers. Do NOT
		// touch last_seen on a mismatch (the existing owner is still
		// the canonical writer).
		slog.Warn("char_owner email mismatch",
			"char", charName,
			"existing", existingEmail,
			"current", ownerEmail)
		return nil
	}
	// Not present → append a 14-column row. valueInputOption=RAW so the
	// email cell is never auto-linkified (USER_ENTERED would turn
	// "foo@example.com" into a hyperlink with display text "foo").
	// Phase 4 plan 04-01 added column N (race), populated lazily via
	// the Apps Script sidebar form (showCharInfoSidebar).
	now := time.Now().UTC().Format(time.RFC3339)
	body := &sheets.ValueRange{
		Values: [][]any{
			{
				charName,        // A char_name
				ownerEmail,      // B owner_email
				"",              // C display_name
				"",              // D discord_handle
				"",              // E class
				"",              // F level
				"FALSE",         // G is_bank_toon
				"FALSE",         // H is_hidden
				"FALSE",         // I is_removed
				now,             // J first_seen
				now,             // K last_seen
				CharOwnerServer, // L server ("blue")
				watcherVersion,  // M watcher_version
				"",              // N race (Phase 4)
			},
		},
	}
	if _, err := c.valuesAppend(ctx, "_char_owner!A:N", body); err != nil {
		return fmt.Errorf("append _char_owner: %w", err)
	}
	return nil
}
