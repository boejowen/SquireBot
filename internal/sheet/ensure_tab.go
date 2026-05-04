package sheet

// EnsureSheet returns the numeric sheetId of `name`, creating the tab if
// missing. Plan 01-05 Task 1.
//
// Caching: results are stored in c.tabs and returned without further API
// calls on subsequent invocations. The cache is invalidated by
// SetSpreadsheetID (which zeroes c.tabs) — callers should not mutate it
// directly. SetSpreadsheetID also resets the cache when the picker is
// re-run via the "Change Workbook…" tray menu (CONTEXT.md D-04).
//
// We use the numeric sheetId rather than A1 names because it is stable
// across user-driven tab renames (RESEARCH.md §2.3 final paragraph;
// SCHEMA-06 makes this a global rule in Phase 2, but Phase 1 already
// follows it). The atomic-clear+write contract in write.go relies on
// GridRange{SheetId: ...} which requires the numeric ID.

import (
	"context"
	"fmt"

	"google.golang.org/api/sheets/v4"
)

// SheetMeta describes a tab's identity and visibility state within a
// workbook. Returned by ListSheets so callers can decide both "is this
// tab present?" AND "is it in the visibility state we expect?". The
// scaffold relies on the Hidden flag to fix up dimension tabs that
// were created visible by upstream side-effects (e.g.,
// ValidateWorkbook's defensive EnsureSheet("_meta") call) before
// scaffold ran.
type SheetMeta struct {
	ID     int64
	Hidden bool
}

// ListSheets returns the full title→SheetMeta map for the active
// spreadsheet without creating any tabs. Plan 02-01 Task 2 added this
// so the scaffold package can do a single bulk read of existing tab
// state before deciding which to create or hide. Side effect:
// refreshes the Client's tabs cache so subsequent EnsureSheet calls
// will hit the cache.
//
// Unlike EnsureSheet, ListSheets never mutates the workbook. Callers
// that want lazy creation should still go through EnsureSheet.
func (c *Client) ListSheets(ctx context.Context) (map[string]SheetMeta, error) {
	if c.spreadsheetID == "" {
		return nil, fmt.Errorf("ListSheets: spreadsheetID not set")
	}
	ss, err := c.spreadsheetsGet(ctx, "sheets(properties(title,sheetId,hidden))")
	if err != nil {
		return nil, fmt.Errorf("get spreadsheet: %w", err)
	}
	out := make(map[string]SheetMeta, len(ss.Sheets))
	for _, s := range ss.Sheets {
		if s.Properties == nil {
			continue
		}
		out[s.Properties.Title] = SheetMeta{
			ID:     s.Properties.SheetId,
			Hidden: s.Properties.Hidden,
		}
		c.tabs[s.Properties.Title] = s.Properties.SheetId
	}
	return out, nil
}

// EnsureSheet returns the numeric sheetId for the tab named `name`,
// creating the tab via AddSheetRequest if it does not already exist.
// Idempotent — safe to call before every write.
func (c *Client) EnsureSheet(ctx context.Context, name string) (int64, error) {
	if c.spreadsheetID == "" {
		return 0, fmt.Errorf("EnsureSheet: spreadsheetID not set")
	}
	if id, ok := c.tabs[name]; ok {
		return id, nil
	}
	// Refresh the title→sheetId map from the spreadsheet (one Get call).
	// Use a Fields filter so the response carries only sheets.properties —
	// avoids transferring grid data we don't need.
	ss, err := c.spreadsheetsGet(ctx, "sheets(properties(title,sheetId))")
	if err != nil {
		return 0, fmt.Errorf("get spreadsheet: %w", err)
	}
	for _, s := range ss.Sheets {
		if s.Properties != nil {
			c.tabs[s.Properties.Title] = s.Properties.SheetId
		}
	}
	if id, ok := c.tabs[name]; ok {
		return id, nil
	}
	// Tab is missing — create it. Single batchUpdate, single AddSheetRequest.
	req := &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{{
			AddSheet: &sheets.AddSheetRequest{
				Properties: &sheets.SheetProperties{Title: name},
			},
		}},
	}
	resp, err := c.batchUpdate(ctx, req)
	if err != nil {
		return 0, fmt.Errorf("addSheet %q: %w", name, err)
	}
	if len(resp.Replies) == 0 || resp.Replies[0].AddSheet == nil ||
		resp.Replies[0].AddSheet.Properties == nil {
		return 0, fmt.Errorf("addSheet %q: empty AddSheet reply", name)
	}
	id := resp.Replies[0].AddSheet.Properties.SheetId
	c.tabs[name] = id
	return id, nil
}
