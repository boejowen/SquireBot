package sheet

// WriteHeartbeat — Plan 02-05 Task 1.
//
// One batchUpdate that:
//
//  1. Updates _char_owner.last_seen (column K, index 10) for every
//     charName whose row is present in _char_owner. Rows are looked up
//     via a single _char_owner!A:A read; the index of the row in that
//     read is the GridRange.StartRowIndex (0-indexed; row 0 is the
//     header). Chars not present in _char_owner are skipped for the
//     last_seen update -- the watcher's per-event UpsertCharOwner is
//     responsible for inserting that row.
//
//  2. Upserts a _status row keyed on (owner_email, char_name). For
//     existing rows, partial in-place update via THREE narrow
//     UpdateCellsRequest blocks covering ONLY columns A (owner_email,
//     index 0), B+C (char_name + watcher_version, indices 1:3), and F
//     (last_heartbeat, index 5). Columns D (last_inventory_upload,
//     index 3) and E (last_spellbook_upload, index 4) are physically
//     untouched -- they are owned by WriteInventory / WriteSpellbook
//     respectively. For missing rows, AppendCells with a 6-cell row
//     where D and E are empty strings (correct: no prior value to
//     preserve, so writing empty is not a clobber).
//
// Pitfall #8 (StringValue cells, Fields="userEnteredValue") and Pitfall
// D (single batchUpdate via the mutex-funneled c.batchUpdate helper) are
// both enforced here.
//
// Empty charNames is a no-op fast path -- returns nil with NO API call.

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/api/sheets/v4"
)

// WriteHeartbeat performs a single batchUpdate fanning out to:
//   - _char_owner.last_seen (column K) for every charName found in the
//     _char_owner roster.
//   - _status row upsert keyed on (owner_email, char_name) carrying
//     watcher_version + last_heartbeat (and preserving any existing
//     last_inventory_upload / last_spellbook_upload values).
//
// One batchUpdate, one quota debit, one mutex acquisition. CONTEXT.md
// (locked): the heartbeat MUST not clobber _status.D / _status.E -- the
// freshness signal Phase 3+ views depend on lives in those cells.
func (c *Client) WriteHeartbeat(ctx context.Context, ownerEmail string, charNames []string, watcherVersion string) error {
	if c.spreadsheetID == "" {
		return fmt.Errorf("WriteHeartbeat: spreadsheetID not set")
	}
	if len(charNames) == 0 {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339)

	// 1. Look up _char_owner row indices keyed by char_name.
	coSheetID, err := c.EnsureSheet(ctx, "_char_owner")
	if err != nil {
		return fmt.Errorf("ensure _char_owner: %w", err)
	}
	coVR, err := c.valuesGet(ctx, "_char_owner!A:A")
	if err != nil {
		return fmt.Errorf("read _char_owner: %w", err)
	}
	// Map of char_name -> 0-indexed row in _char_owner. Index 0 is the
	// header row "char_name"; we deliberately skip it (the heartbeat
	// never writes to row 0).
	coRows := map[string]int{}
	for i, row := range coVR.Values {
		if i == 0 || len(row) < 1 {
			continue
		}
		name, _ := row[0].(string)
		if name != "" {
			coRows[name] = i
		}
	}

	// 2. Look up _status rows keyed by (owner_email, char_name).
	statusSheetID, err := c.EnsureSheet(ctx, "_status")
	if err != nil {
		return fmt.Errorf("ensure _status: %w", err)
	}
	statusVR, err := c.valuesGet(ctx, "_status!A:B")
	if err != nil {
		return fmt.Errorf("read _status: %w", err)
	}
	statusRows := map[string]int{} // key = "email|char" -> row index
	for i, row := range statusVR.Values {
		if i == 0 || len(row) < 2 {
			continue
		}
		email, _ := row[0].(string)
		name, _ := row[1].(string)
		statusRows[email+"|"+name] = i
	}

	// 3. Build the single BatchUpdate.
	var requests []*sheets.Request

	// 3a. _char_owner.last_seen updates (col K, index 10).
	for _, charName := range charNames {
		rowIdx, ok := coRows[charName]
		if !ok {
			slog.Info("heartbeat: char missing from _char_owner; skipping last_seen update",
				"char", charName)
			continue
		}
		nowSv := now
		requests = append(requests, &sheets.Request{
			UpdateCells: &sheets.UpdateCellsRequest{
				Range: &sheets.GridRange{
					SheetId:          coSheetID,
					StartRowIndex:    int64(rowIdx),
					EndRowIndex:      int64(rowIdx + 1),
					StartColumnIndex: 10, // K = last_seen
					EndColumnIndex:   11,
				},
				Rows: []*sheets.RowData{{Values: []*sheets.CellData{
					{UserEnteredValue: &sheets.ExtendedValue{StringValue: &nowSv}},
				}}},
				Fields: "userEnteredValue",
			},
		})
	}

	// 3b. _status row updates / appends.
	for _, charName := range charNames {
		key := ownerEmail + "|" + charName
		if rowIdx, ok := statusRows[key]; ok {
			// Existing row: emit THREE narrow UpdateCellsRequest blocks so
			// columns D (3) and E (4) are physically untouched.
			aSv := ownerEmail
			requests = append(requests, &sheets.Request{
				UpdateCells: &sheets.UpdateCellsRequest{
					Range: &sheets.GridRange{
						SheetId:          statusSheetID,
						StartRowIndex:    int64(rowIdx),
						EndRowIndex:      int64(rowIdx + 1),
						StartColumnIndex: 0, // A
						EndColumnIndex:   1,
					},
					Rows: []*sheets.RowData{{Values: []*sheets.CellData{
						{UserEnteredValue: &sheets.ExtendedValue{StringValue: &aSv}},
					}}},
					Fields: "userEnteredValue",
				},
			})
			bSv, cSv := charName, watcherVersion
			requests = append(requests, &sheets.Request{
				UpdateCells: &sheets.UpdateCellsRequest{
					Range: &sheets.GridRange{
						SheetId:          statusSheetID,
						StartRowIndex:    int64(rowIdx),
						EndRowIndex:      int64(rowIdx + 1),
						StartColumnIndex: 1, // B
						EndColumnIndex:   3, // through C (exclusive end)
					},
					Rows: []*sheets.RowData{{Values: []*sheets.CellData{
						{UserEnteredValue: &sheets.ExtendedValue{StringValue: &bSv}},
						{UserEnteredValue: &sheets.ExtendedValue{StringValue: &cSv}},
					}}},
					Fields: "userEnteredValue",
				},
			})
			fSv := now
			requests = append(requests, &sheets.Request{
				UpdateCells: &sheets.UpdateCellsRequest{
					Range: &sheets.GridRange{
						SheetId:          statusSheetID,
						StartRowIndex:    int64(rowIdx),
						EndRowIndex:      int64(rowIdx + 1),
						StartColumnIndex: 5, // F
						EndColumnIndex:   6,
					},
					Rows: []*sheets.RowData{{Values: []*sheets.CellData{
						{UserEnteredValue: &sheets.ExtendedValue{StringValue: &fSv}},
					}}},
					Fields: "userEnteredValue",
				},
			})
		} else {
			// Append branch: no prior row exists. Write all 6 cells with empty
			// strings for D and E -- WriteInventory / WriteSpellbook will
			// populate them on their first run.
			row := []string{ownerEmail, charName, watcherVersion, "", "", now}
			cells := make([]*sheets.CellData, 0, 6)
			for _, s := range row {
				sv := s
				cells = append(cells, &sheets.CellData{
					UserEnteredValue: &sheets.ExtendedValue{StringValue: &sv},
				})
			}
			requests = append(requests, &sheets.Request{
				AppendCells: &sheets.AppendCellsRequest{
					SheetId: statusSheetID,
					Rows:    []*sheets.RowData{{Values: cells}},
					Fields:  "userEnteredValue",
				},
			})
		}
	}

	if len(requests) == 0 {
		return nil
	}

	if _, err := c.batchUpdate(ctx, &sheets.BatchUpdateSpreadsheetRequest{Requests: requests}); err != nil {
		return fmt.Errorf("WriteHeartbeat batchUpdate: %w", err)
	}
	return nil
}
