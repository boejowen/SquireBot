package sheet

// WriteInventory — atomic single-call clear+write of an inv:<Char> tab.
// Plan 01-05 Task 2. Implements RESEARCH.md §2.3 Pattern 1 verbatim,
// satisfies Critical Constraint #3 (one batchUpdate, not clear-then-write),
// Critical Constraint #8 / Pitfall #8 (StringValue cells, never NumberValue
// or FormulaValue), and OPS-01 (per-character non-overlapping ranges —
// every write keys on inv:<Char>!A1:F500 with the character-specific
// numeric SheetId).
//
// The threat-model rationale (T-05-05, T-05-06, T-05-07) for the single-
// batchUpdate-with-fields="userEnteredValue" shape is enumerated in the
// plan's <threat_model>; do not switch to a two-call values.batchClear +
// values.update pattern even if it looks simpler. The single-call form
// is atomic per the API contract and avoids the race window where a
// reader sees post-clear / pre-write state.

import (
	"context"
	"fmt"

	"google.golang.org/api/sheets/v4"
)

// InventoryHeader is the fixed 6-element header row Plan 07's wiring
// passes to WriteInventory. The shape (Location, Name, ID, Count, Slots,
// _uploaded_at) matches the Phase 1 schema in RESEARCH.md §6 and is
// extend-only — additional columns may be appended at the right edge in
// later phases without requiring a schema_version bump.
var InventoryHeader = []string{"Location", "Name", "ID", "Count", "Slots", "_uploaded_at"}

// WriteInventory replaces the entire inv:<charName> tab atomically via a
// single spreadsheets.batchUpdate call carrying one UpdateCellsRequest.
//
// The Range covers inv:<charName>!A1:F500 (InvTabMaxRows × InvTabColumns).
// Cells in the range NOT covered by `rows` are CLEARED as part of the
// same request — see the API contract:
//
//	"If the data in rows does not cover the entire requested range, the
//	 fields matching those set in fields will be cleared."
//	[CITED in RESEARCH.md §2.3]
//
// Every cell is written as UserEnteredValue.StringValue — never
// NumberValue, even for the numeric-looking ID and Count columns. This
// enforces Pitfall #8: numeric coercion in consolidated views (Phase 3+)
// would trigger a recalc storm and drop leading zeros from item IDs.
//
// dataRows is the parser's output (typically 5 columns: Location, Name,
// ID, Count, Slots). WriteInventory pads short rows to 5 then appends
// `uploadedAt` as the 6th column. The parser already filters rows with
// fewer than 5 cells; the padding here is defensive belt-and-braces.
func (c *Client) WriteInventory(ctx context.Context, charName string, header []string, dataRows [][]string, uploadedAt string) error {
	if c.spreadsheetID == "" {
		return fmt.Errorf("WriteInventory: spreadsheetID not set")
	}
	tabName := "inv:" + charName
	sheetID, err := c.EnsureSheet(ctx, tabName)
	if err != nil {
		return fmt.Errorf("ensure %s: %w", tabName, err)
	}

	rows := make([]*sheets.RowData, 0, len(dataRows)+1)
	rows = append(rows, toRowData(header))
	for _, dr := range dataRows {
		// Pad to 5 cells (defensive — parser already filters <5) then
		// append uploadedAt as the 6th cell. The result is exactly 6
		// cells regardless of how short or long the input row is.
		full := make([]string, 0, 6)
		full = append(full, dr...)
		for len(full) < 5 {
			full = append(full, "")
		}
		// If the parser ever hands us >5 cells (it shouldn't, but be
		// defensive), truncate to 5 before appending uploadedAt.
		full = append(full[:5], uploadedAt)
		rows = append(rows, toRowData(full))
	}

	req := &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{{
			UpdateCells: &sheets.UpdateCellsRequest{
				Range: &sheets.GridRange{
					SheetId:          sheetID,
					StartRowIndex:    0,
					EndRowIndex:      InvTabMaxRows, // 500 — clears anything past N (atomic)
					StartColumnIndex: 0,
					EndColumnIndex:   InvTabColumns, // 6 = A:F
				},
				Rows: rows,
				// Pitfall #8: ALWAYS exactly "userEnteredValue" — never
				// "*" (would clear notes/formatting too) and never
				// "userEnteredValue,note" (would carry note semantics
				// we don't manage from the watcher side).
				Fields: "userEnteredValue",
			},
		}},
	}
	if _, err := c.svc.Spreadsheets.BatchUpdate(c.spreadsheetID, req).Context(ctx).Do(); err != nil {
		return fmt.Errorf("batchUpdate %s: %w", tabName, err)
	}
	return nil
}

// toRowData converts a []string to a *sheets.RowData using StringValue
// for every cell. Pitfall #8 enforcement: do NOT use NumberValue here,
// even for cells that look numeric (item ID, Count). Numeric coercion
// in consolidated views (Phase 3+) would drop leading zeros and trigger
// a workbook-wide recalc storm at guild scale.
func toRowData(cells []string) *sheets.RowData {
	vs := make([]*sheets.CellData, len(cells))
	for i, s := range cells {
		v := s // capture per-iter — sheets API holds the pointer
		vs[i] = &sheets.CellData{
			UserEnteredValue: &sheets.ExtendedValue{StringValue: &v},
		}
	}
	return &sheets.RowData{Values: vs}
}
