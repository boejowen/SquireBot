package sheet

// ValidateWorkbook + readMeta + bootstrapMeta — the canonical_id +
// schema_version handshake. Plan 01-05 Task 1.
//
// Sequence (RESEARCH.md §2.6 Pattern 4 + §12.3-12.4):
//   1. EnsureSheet("_meta") — defensive create if the master template
//      omitted it; the master template SHOULD ship with _meta but we
//      can't assume it (Open Question Q1, CONTEXT.md D-01 informational).
//   2. Read _meta!A1:B2 (4 cells: key/value pairs for canonical_id and
//      schema_version). Bounded read regardless of workbook size — see
//      threat T-05-10.
//   3. Branch:
//      - canonical_id empty → BOOTSTRAP path (write our values, schema=1).
//        Handles fresh-from-template workbooks where Phase 3 Apps Script
//        hasn't initialised _meta yet (Phase 1 ships before Phase 3).
//      - canonical_id == CanonicalID + schema_version <= max → HEALTHY,
//        return nil.
//      - canonical_id == CanonicalID + schema_version > max → ErrSchemaTooNew
//        (Critical Constraint #5 fail-fast).
//      - canonical_id mismatches → ErrWrongWorkbook (verbatim D-03 message).
//
// We log NOTHING from this function on the error paths beyond the wrapped
// error; threat T-05-04 mandates we don't dump cell payloads to slog.

import (
	"context"
	"fmt"
	"strconv"

	"google.golang.org/api/sheets/v4"
)

// ValidateWorkbook performs the canonical_id + schema_version handshake
// against the spreadsheet currently set on the Client. Plan 06's picker
// callback calls this immediately after the user picks a spreadsheet;
// ErrWrongWorkbook is returned verbatim to the wizard (D-03).
func (c *Client) ValidateWorkbook(ctx context.Context) error {
	if c.spreadsheetID == "" {
		return fmt.Errorf("ValidateWorkbook: spreadsheetID not set")
	}
	cid, sv, err := c.readMeta(ctx)
	if err != nil {
		return fmt.Errorf("read _meta: %w", err)
	}
	switch {
	case cid == "":
		// BOOTSTRAP: empty cells → write canonical_id + schema_version=1.
		return c.bootstrapMeta(ctx)
	case cid != CanonicalID:
		// Wrong workbook — return the sentinel; downstream HTTP handler
		// renders Error.Error() into the rejection page body.
		return ErrWrongWorkbook
	case sv > WatcherMaxSchemaVersion:
		// Forward-compat workbook the watcher can't safely write.
		return fmt.Errorf("%w (workbook v%d, watcher max v%d)",
			ErrSchemaTooNew, sv, WatcherMaxSchemaVersion)
	}
	return nil
}

// readMeta reads _meta!A1:B2 and returns (canonical_id, schema_version, err).
// Returns ("", 0, nil) when the cells are empty — the BOOTSTRAP signal.
//
// We tolerate either string or numeric encodings for schema_version
// (Sheets returns numbers as JSON numbers via valueRenderOption=FORMATTED_VALUE
// when cells were written as a number; bootstrapMeta itself writes "1" as
// a string under valueInputOption=RAW). Anything that isn't parseable as
// an integer surfaces as a wrapped error.
func (c *Client) readMeta(ctx context.Context) (string, int, error) {
	// Defensive: ensure _meta tab exists before reading. Without this the
	// values.get call below 400s on a master template that's missing _meta.
	if _, err := c.EnsureSheet(ctx, "_meta"); err != nil {
		return "", 0, err
	}
	resp, err := c.svc.Spreadsheets.Values.
		Get(c.spreadsheetID, "_meta!A1:B2").
		Context(ctx).Do()
	if err != nil {
		return "", 0, err
	}
	var cid string
	var sv int
	for _, row := range resp.Values {
		if len(row) < 2 {
			continue
		}
		key, _ := row[0].(string)
		switch key {
		case "canonical_id":
			cid, _ = row[1].(string)
		case "schema_version":
			n, err := coerceInt(row[1])
			if err != nil {
				return "", 0, fmt.Errorf("_meta.schema_version not an int: %w", err)
			}
			sv = n
		}
	}
	return cid, sv, nil
}

// coerceInt accepts a Sheets API value (which arrives as either string or
// json.Number / float64 depending on the cell's underlying type) and
// returns its integer value.
func coerceInt(v any) (int, error) {
	switch x := v.(type) {
	case string:
		if x == "" {
			return 0, nil
		}
		return strconv.Atoi(x)
	case float64:
		return int(x), nil
	case int:
		return x, nil
	default:
		return 0, fmt.Errorf("unsupported type %T for int coercion", v)
	}
}

// bootstrapMeta writes canonical_id + schema_version=1 to _meta!A1:B2.
// Single values.update call with valueInputOption=RAW so neither cell is
// coerced (the email-as-hyperlink trap that USER_ENTERED falls into).
func (c *Client) bootstrapMeta(ctx context.Context) error {
	if _, err := c.EnsureSheet(ctx, "_meta"); err != nil {
		return err
	}
	body := &sheets.ValueRange{
		Values: [][]any{
			{"canonical_id", CanonicalID},
			{"schema_version", strconv.Itoa(WatcherMaxSchemaVersion)},
		},
	}
	_, err := c.svc.Spreadsheets.Values.
		Update(c.spreadsheetID, "_meta!A1:B2", body).
		ValueInputOption("RAW").
		Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("bootstrap _meta: %w", err)
	}
	return nil
}
