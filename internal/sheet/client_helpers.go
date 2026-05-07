package sheet

// Plan 02-03 Task 2 — *Client API helper methods.
//
// Every Sheets API call this package issues MUST go through one of the
// helpers in this file. The helpers do two things: (a) acquire batchMu
// before the round-trip and release it on return (Pitfall D), and (b)
// run the call inside withRetry (WATCH-07). This is the only sanctioned
// way to talk to the Sheets API from inside *Client.
//
// CONTEXT.md (locked): the heartbeat goroutine (Plan 02-05) and the
// watcher goroutine BOTH share a single *sheet.Client. Without these
// helpers, a heartbeat write could non-deterministically interleave with
// an inventory write and produce a stale `last_inventory_upload` cell.
// The mutex closes that gap; the retry envelope ensures transient 429/5xx
// errors recover silently rather than bubble to the user.

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"google.golang.org/api/googleapi"
	"google.golang.org/api/sheets/v4"
)

// batchUpdate wraps c.svc.Spreadsheets.BatchUpdate(...).Do() with batchMu
// + the WATCH-07 retry envelope. The reply is returned to the caller so
// AddSheet's new sheetId (BatchUpdateSpreadsheetResponse.Replies[0].AddSheet
// .Properties.SheetId) is reachable.
func (c *Client) batchUpdate(ctx context.Context, req *sheets.BatchUpdateSpreadsheetRequest) (*sheets.BatchUpdateSpreadsheetResponse, error) {
	if c.spreadsheetID == "" {
		return nil, fmt.Errorf("batchUpdate: spreadsheetID not set")
	}
	c.batchMu.Lock()
	defer c.batchMu.Unlock()
	var resp *sheets.BatchUpdateSpreadsheetResponse
	err := withRetry(ctx, func() error {
		r, err := c.svc.Spreadsheets.BatchUpdate(c.spreadsheetID, req).Context(ctx).Do()
		if err != nil {
			return err
		}
		resp = r
		return nil
	}, c.onRefreshOrNoop())
	return resp, err
}

// valuesGet wraps Spreadsheets.Values.Get(...).Do() with batchMu + retry.
// Reads also acquire the mutex because a read racing a write against the
// same workbook can produce inconsistent snapshots.
func (c *Client) valuesGet(ctx context.Context, a1Range string) (*sheets.ValueRange, error) {
	if c.spreadsheetID == "" {
		return nil, fmt.Errorf("valuesGet: spreadsheetID not set")
	}
	c.batchMu.Lock()
	defer c.batchMu.Unlock()
	var resp *sheets.ValueRange
	err := withRetry(ctx, func() error {
		r, err := c.svc.Spreadsheets.Values.Get(c.spreadsheetID, a1Range).Context(ctx).Do()
		if err != nil {
			return err
		}
		resp = r
		return nil
	}, c.onRefreshOrNoop())
	return resp, err
}

// valuesAppend wraps Spreadsheets.Values.Append(...).Do() with batchMu +
// retry. valueInputOption=RAW + insertDataOption=INSERT_ROWS are baked in
// (Critical Constraint #3 / OPS-01 — never USER_ENTERED on the hot path).
func (c *Client) valuesAppend(ctx context.Context, a1Range string, body *sheets.ValueRange) (*sheets.AppendValuesResponse, error) {
	if c.spreadsheetID == "" {
		return nil, fmt.Errorf("valuesAppend: spreadsheetID not set")
	}
	c.batchMu.Lock()
	defer c.batchMu.Unlock()
	var resp *sheets.AppendValuesResponse
	err := withRetry(ctx, func() error {
		r, err := c.svc.Spreadsheets.Values.Append(c.spreadsheetID, a1Range, body).
			ValueInputOption("RAW").
			InsertDataOption("INSERT_ROWS").
			Context(ctx).Do()
		if err != nil {
			return err
		}
		resp = r
		return nil
	}, c.onRefreshOrNoop())
	return resp, err
}

// valuesUpdate wraps Spreadsheets.Values.Update(...).Do() with batchMu +
// retry. valueInputOption=RAW (same rationale as valuesAppend).
func (c *Client) valuesUpdate(ctx context.Context, a1Range string, body *sheets.ValueRange) error {
	if c.spreadsheetID == "" {
		return fmt.Errorf("valuesUpdate: spreadsheetID not set")
	}
	c.batchMu.Lock()
	defer c.batchMu.Unlock()
	return withRetry(ctx, func() error {
		_, err := c.svc.Spreadsheets.Values.Update(c.spreadsheetID, a1Range, body).
			ValueInputOption("RAW").
			Context(ctx).Do()
		return err
	}, c.onRefreshOrNoop())
}

// spreadsheetsGet wraps Spreadsheets.Get(...).Fields(...).Do() with batchMu
// + retry. Used by EnsureSheet's title→sheetId discovery and by ListSheets.
func (c *Client) spreadsheetsGet(ctx context.Context, fields googleapi.Field) (*sheets.Spreadsheet, error) {
	if c.spreadsheetID == "" {
		return nil, fmt.Errorf("spreadsheetsGet: spreadsheetID not set")
	}
	c.batchMu.Lock()
	defer c.batchMu.Unlock()
	var resp *sheets.Spreadsheet
	err := withRetry(ctx, func() error {
		r, err := c.svc.Spreadsheets.Get(c.spreadsheetID).Fields(fields).Context(ctx).Do()
		if err != nil {
			return err
		}
		resp = r
		return nil
	}, c.onRefreshOrNoop())
	return resp, err
}

// updateSheetProperties wraps a single UpdateSheetPropertiesRequest as a
// batchUpdate (HideSheet is the only current caller; lifting the mutex+retry
// pattern here keeps scaffold_helpers.go's HideSheet clean).
//
// This is a thin wrapper over batchUpdate — separate name retained for
// readability at the call site.
func (c *Client) updateSheetProperties(ctx context.Context, req *sheets.BatchUpdateSpreadsheetRequest) error {
	_, err := c.batchUpdate(ctx, req)
	return err
}

// onRefreshOrNoop returns c.onRefresh, or a no-op refresher if onRefresh
// is nil. The no-op lets withRetry consume its single refresh allowance
// (so the second auth-flavored 403 still surfaces as ErrPermanentAuth)
// rather than blocking on a nil callback.
func (c *Client) onRefreshOrNoop() func() error {
	if c.onRefresh != nil {
		return c.onRefresh
	}
	return func() error { return nil }
}

// PingNoLock makes a lightweight Spreadsheets.Get call WITHOUT acquiring
// batchMu. It MUST only be called from within onRefresh (i.e., from code
// that already holds batchMu via withRetry).
//
// Purpose: after onRefresh swaps in a fresh TokenSource, the first retry
// attempt is batchUpdate — which hits the Sheets API without any prior
// read of the spreadsheet under the new grant. A Spreadsheets.Get here:
//   - verifies the new access token actually works for this spreadsheet, and
//   - satisfies any drive.file "file was opened" registration requirement
//     so the subsequent batchUpdate retry is accepted.
//
// Returns ErrPermanentAuth on HTTP 401 (token invalid for this resource),
// or the raw error for other failures.
func (c *Client) PingNoLock(ctx context.Context) error {
	if c.spreadsheetID == "" {
		return fmt.Errorf("PingNoLock: spreadsheetID not set")
	}
	_, err := c.svc.Spreadsheets.Get(c.spreadsheetID).Fields("spreadsheetId").Context(ctx).Do()
	if err == nil {
		return nil
	}
	var ge *googleapi.Error
	if errors.As(err, &ge) && ge.Code == http.StatusUnauthorized {
		return ErrPermanentAuth
	}
	return err
}
