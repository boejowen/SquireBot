package sheet

// Plan 02-01 Task 2: helper methods that the internal/scaffold package
// needs to drive ScaffoldSchemaV1. The scaffold package cannot reach into
// *Client.svc directly (private), so this file exposes the minimum API
// surface needed: write a header row to A1, hide a sheet, read a column,
// append a row.
//
// All writes use valueInputOption=RAW per Critical Constraint #3 / OPS-01.
// No USER_ENTERED for hot-path or scaffold writes — recalc storms +
// auto-link of email cells.

import (
	"context"
	"fmt"

	"google.golang.org/api/sheets/v4"
)

// WriteHeaderRow writes a single row of cells to `tab!A1:<lastCol>1` via
// values.Update with valueInputOption=RAW. Used by ScaffoldSchemaV1 to
// stamp the header row on a freshly-created tab.
//
// Cells are written as raw strings — Sheets does not auto-coerce them to
// links/numbers under RAW.
func (c *Client) WriteHeaderRow(ctx context.Context, tab string, headers []string) error {
	if c.spreadsheetID == "" {
		return fmt.Errorf("WriteHeaderRow: spreadsheetID not set")
	}
	row := make([]any, len(headers))
	for i, h := range headers {
		row[i] = h
	}
	rng := fmt.Sprintf("%s!A1", tab)
	body := &sheets.ValueRange{Values: [][]any{row}}
	if err := c.valuesUpdate(ctx, rng, body); err != nil {
		return fmt.Errorf("write header %s: %w", tab, err)
	}
	return nil
}

// HideSheet sets sheets.SheetProperties.Hidden = true on the given sheet
// id via a single batchUpdate / UpdateSheetPropertiesRequest. Used by
// ScaffoldSchemaV1 to hide the underscore-prefixed dimension tabs after
// creation. Idempotent — calling on an already-hidden sheet is a no-op
// from the user's perspective.
func (c *Client) HideSheet(ctx context.Context, sheetID int64) error {
	if c.spreadsheetID == "" {
		return fmt.Errorf("HideSheet: spreadsheetID not set")
	}
	req := &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{{
			UpdateSheetProperties: &sheets.UpdateSheetPropertiesRequest{
				Properties: &sheets.SheetProperties{
					SheetId: sheetID,
					Hidden:  true,
				},
				Fields: "hidden",
			},
		}},
	}
	if err := c.updateSheetProperties(ctx, req); err != nil {
		return fmt.Errorf("hide sheet %d: %w", sheetID, err)
	}
	return nil
}

// ReadColumn reads a single column from a tab via values.Get. Returns
// the column values as a []string (cells past the last populated row are
// trimmed by the API). Used by ScaffoldSchemaV1 to discover which _meta
// keys are already present before appending new ones.
func (c *Client) ReadColumn(ctx context.Context, rangeA1 string) ([]string, error) {
	if c.spreadsheetID == "" {
		return nil, fmt.Errorf("ReadColumn: spreadsheetID not set")
	}
	resp, err := c.valuesGet(ctx, rangeA1)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", rangeA1, err)
	}
	out := make([]string, 0, len(resp.Values))
	for _, row := range resp.Values {
		if len(row) == 0 {
			out = append(out, "")
			continue
		}
		s, _ := row[0].(string)
		out = append(out, s)
	}
	return out, nil
}

// AppendRow appends a single row to `tab!A:<lastCol>` via values.Append
// with valueInputOption=RAW + insertDataOption=INSERT_ROWS. Used by
// ScaffoldSchemaV1 to add missing _meta KV rows without overwriting any
// existing ones.
func (c *Client) AppendRow(ctx context.Context, rangeA1 string, cells []string) error {
	if c.spreadsheetID == "" {
		return fmt.Errorf("AppendRow: spreadsheetID not set")
	}
	row := make([]any, len(cells))
	for i, s := range cells {
		row[i] = s
	}
	body := &sheets.ValueRange{Values: [][]any{row}}
	if _, err := c.valuesAppend(ctx, rangeA1, body); err != nil {
		return fmt.Errorf("append %s: %w", rangeA1, err)
	}
	return nil
}
