package sheet

// ValidateWorkbook + readMeta — three-state canonical_id + schema_version
// handshake. Plan 02-01 Task 1 (refactor of Plan 01-05 Task 1).
//
// State machine (RESEARCH.md §2.6 Pattern 4 + Phase 02 RESEARCH §Pitfall C):
//
//   1. EnsureSheet("_meta") — defensive create if absent. Read _meta!A1:B20
//      (extended from A1:B2 in Phase 1; we now write up to ~13 keys).
//   2. Branch on what we found:
//      - Zero rows                                          → WorkbookStateEmpty
//      - Rows present but no `canonical_id` row             → WorkbookStateWrong
//        (Pitfall C: a workbook with _meta but no canonical_id is suspect;
//        refuse rather than scaffold over user data.)
//      - canonical_id == CanonicalID + schema_version <= max → WorkbookStateMatches
//      - canonical_id == CanonicalID + schema_version  > max → WorkbookStateMatches,
//        ErrSchemaTooNew (it IS our workbook — but caller must refuse to write).
//      - canonical_id != CanonicalID                        → WorkbookStateWrong
//
// What changed from Phase 1: ValidateWorkbook NEVER writes any longer.
// bootstrapMeta is deleted; ScaffoldSchemaV1 in internal/scaffold owns
// every _meta row write. The caller (runWatcher) reads the (state, err)
// pair and then invokes ScaffoldSchemaV1 for both Empty and Matches.
//
// We log NOTHING from this function on the error paths beyond the wrapped
// error; threat T-05-04 mandates we don't dump cell payloads to slog.

import (
	"context"
	"fmt"
	"strconv"
)

// WorkbookState is the three-state result of ValidateWorkbook. Plan 02-01
// Task 1 introduced this enum to replace Phase 1's "empty triggers
// bootstrap" path which conflated Empty with WrongCanonical. The runWatcher
// caller now branches explicitly: Wrong → refuse; Empty or Matches →
// proceed to ScaffoldSchemaV1.
type WorkbookState int

const (
	// WorkbookStateUnknown is the zero value; never returned.
	WorkbookStateUnknown WorkbookState = iota
	// WorkbookStateEmpty means there is no _meta tab, or _meta exists with
	// zero rows. Caller may safely scaffold.
	WorkbookStateEmpty
	// WorkbookStateMatches means _meta.canonical_id == CanonicalID. Healthy.
	// (Caller still must check err for ErrSchemaTooNew.)
	WorkbookStateMatches
	// WorkbookStateWrong means _meta has rows but canonical_id is missing
	// OR canonical_id != CanonicalID. REFUSE to write.
	WorkbookStateWrong
)

// ValidateWorkbook performs the canonical_id + schema_version handshake
// against the spreadsheet currently set on the Client. It does NOT write.
// Returns one of three WorkbookState values plus an error sentinel for
// the Wrong and SchemaTooNew paths.
func (c *Client) ValidateWorkbook(ctx context.Context) (WorkbookState, error) {
	if c.spreadsheetID == "" {
		return WorkbookStateUnknown, fmt.Errorf("ValidateWorkbook: spreadsheetID not set")
	}
	rows, err := c.readMeta(ctx)
	if err != nil {
		return WorkbookStateUnknown, fmt.Errorf("read _meta: %w", err)
	}
	if len(rows) == 0 {
		return WorkbookStateEmpty, nil
	}
	// Scan rows for canonical_id and schema_version keys.
	var (
		cid       string
		sv        int
		haveCID   bool
		svErr     error
	)
	for _, row := range rows {
		if len(row) < 2 {
			continue
		}
		key, _ := row[0].(string)
		switch key {
		case "canonical_id":
			cid, _ = row[1].(string)
			haveCID = true
		case "schema_version":
			n, perr := coerceInt(row[1])
			if perr != nil {
				svErr = fmt.Errorf("_meta.schema_version not an int: %w", perr)
			}
			sv = n
		}
	}
	if svErr != nil {
		return WorkbookStateUnknown, svErr
	}
	if !haveCID {
		// Pitfall C defensive: _meta has rows but no canonical_id row.
		// Refuse — this is some other tool's workbook, not ours.
		return WorkbookStateWrong, ErrWrongWorkbook
	}
	if cid != CanonicalID {
		return WorkbookStateWrong, ErrWrongWorkbook
	}
	// Canonical ID matches — this IS our workbook. Check schema version.
	if sv > WatcherMaxSchemaVersion {
		return WorkbookStateMatches, fmt.Errorf("%w (workbook v%d, watcher max v%d)",
			ErrSchemaTooNew, sv, WatcherMaxSchemaVersion)
	}
	return WorkbookStateMatches, nil
}

// readMeta reads _meta!A1:B20 and returns the raw row slice. Returns
// nil rows when the tab is empty — the Empty signal. The 20-row range
// matches the locked _meta KV row set in internal/scaffold (13 rows in
// v1, with headroom for additive growth before any future schema bump).
func (c *Client) readMeta(ctx context.Context) ([][]any, error) {
	if _, err := c.EnsureSheet(ctx, "_meta"); err != nil {
		return nil, err
	}
	resp, err := c.valuesGet(ctx, "_meta!A1:B20")
	if err != nil {
		return nil, err
	}
	return resp.Values, nil
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
