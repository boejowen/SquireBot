package sheet

// Tests for WriteHeartbeat (Plan 02-05 Task 1). Exercises the eight
// behaviours from the plan's <behavior> block:
//
//   1. Single batchUpdate with the expected request shape (1 UpdateCells
//      per existing _char_owner row + 3 narrow UpdateCells per existing
//      _status row + 1 AppendCells per missing _status row).
//   2. _char_owner.last_seen update targets column K of the row matching
//      char_name in column A.
//   3. _status row upsert: existing rows updated via three narrow
//      UpdateCellsRequest blocks (A:A, B:C, F:F); missing rows appended.
//   4. (W6 preservation) heartbeat run against a _status row with non-
//      empty D and E cells emits NO UpdateCellsRequest covering columns
//      3 (D) or 4 (E).
//   5. All written cells are StringValue + every UpdateCellsRequest has
//      Fields="userEnteredValue" (Pitfall #8).
//   6. Empty charNames -> nil error, ZERO HTTP calls.
//   7. char missing from _char_owner -> last_seen update skipped for that
//      char; _status append still runs.
//   8. Append branch writes all 6 cells (A through F) for net-new _status
//      rows; D and E are empty strings (no clobber concern -- no prior
//      value to preserve).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// hbStub captures every HTTP call. The test bodies inspect appendedRequests
// (the captured BatchUpdateSpreadsheetRequest payloads) to assert request
// shape per the eight behaviour clauses.
type hbStub struct {
	t *testing.T

	// What the values.get on _char_owner!A:A returns. First row is header.
	charOwnerRows [][]any

	// What the values.get on _status!A:B returns. First row is header.
	statusRows [][]any

	// Pre-existing tabs (returned by spreadsheets.Get).
	sheetsList []sheetInfo

	httpCalls         int64
	batchUpdateCount  int
	batchUpdateBodies []*sheets.BatchUpdateSpreadsheetRequest
}

func (h *hbStub) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&h.httpCalls, 1)
		path := r.URL.Path
		switch {
		case r.Method == "GET" && strings.HasSuffix(path, "/spreadsheets/SHEET1"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"spreadsheetId": "SHEET1",
				"sheets":        encodeSheetsList(h.sheetsList),
			})
		case r.Method == "GET" && strings.Contains(path, "/values/_char_owner"):
			body := map[string]any{"range": "_char_owner!A:A", "majorDimension": "ROWS"}
			if len(h.charOwnerRows) > 0 {
				body["values"] = h.charOwnerRows
			}
			_ = json.NewEncoder(w).Encode(body)
		case r.Method == "GET" && strings.Contains(path, "/values/_status"):
			body := map[string]any{"range": "_status!A:B", "majorDimension": "ROWS"}
			if len(h.statusRows) > 0 {
				body["values"] = h.statusRows
			}
			_ = json.NewEncoder(w).Encode(body)
		case r.Method == "POST" && strings.HasSuffix(path, ":batchUpdate"):
			var req sheets.BatchUpdateSpreadsheetRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			h.batchUpdateBodies = append(h.batchUpdateBodies, &req)
			h.batchUpdateCount++
			replies := make([]map[string]any, 0, len(req.Requests))
			for _, rq := range req.Requests {
				if rq.AddSheet != nil {
					title := rq.AddSheet.Properties.Title
					newID := int64(90000 + len(h.sheetsList))
					h.sheetsList = append(h.sheetsList, sheetInfo{Title: title, SheetID: newID})
					replies = append(replies, map[string]any{
						"addSheet": map[string]any{
							"properties": map[string]any{"title": title, "sheetId": newID},
						},
					})
				} else {
					replies = append(replies, map[string]any{})
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"replies": replies})
		default:
			h.t.Logf("UNHANDLED: %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func newHBClient(t *testing.T, h *hbStub) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h.handler())
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "fake"})
	c, err := NewClient(ctx, ts, "SHEET1",
		option.WithEndpoint(srv.URL),
		option.WithHTTPClient(srv.Client()))
	if err != nil {
		srv.Close()
		t.Fatalf("NewClient: %v", err)
	}
	return c, srv
}

// countUpdateCellsByColumn walks every UpdateCellsRequest in the captured
// batches and returns how many requests touch the half-open [startCol,
// endCol) range. Used by Test 4 (D/E preservation) and Test 1 (request
// count).
func countUpdateCellsByColumn(reqs []*sheets.BatchUpdateSpreadsheetRequest, startCol, endCol int64, sheetID int64) int {
	count := 0
	for _, batch := range reqs {
		for _, r := range batch.Requests {
			if r.UpdateCells == nil || r.UpdateCells.Range == nil {
				continue
			}
			rng := r.UpdateCells.Range
			if rng.SheetId != sheetID {
				continue
			}
			// Half-open intersection: [rng.StartColumnIndex, rng.EndColumnIndex)
			// vs [startCol, endCol).
			if rng.StartColumnIndex < endCol && rng.EndColumnIndex > startCol {
				count++
			}
		}
	}
	return count
}

func countAppendCells(reqs []*sheets.BatchUpdateSpreadsheetRequest, sheetID int64) int {
	count := 0
	for _, batch := range reqs {
		for _, r := range batch.Requests {
			if r.AppendCells == nil {
				continue
			}
			if r.AppendCells.SheetId == sheetID {
				count++
			}
		}
	}
	return count
}

func countUpdateCells(reqs []*sheets.BatchUpdateSpreadsheetRequest) int {
	count := 0
	for _, batch := range reqs {
		for _, r := range batch.Requests {
			if r.UpdateCells != nil {
				count++
			}
		}
	}
	return count
}

// Test 1: 2 chars, both pre-existing in _char_owner AND _status -> single
// batchUpdate with 2 _char_owner UpdateCells + 6 _status UpdateCells (3 per
// row) = 8 UpdateCellsRequest total; 0 AppendCells.
func TestWriteHeartbeat_SingleBatchUpdate_BothCharsPresent(t *testing.T) {
	const coSheetID = int64(7)
	const statusSheetID = int64(9)
	h := &hbStub{
		t: t,
		sheetsList: []sheetInfo{
			{Title: "_char_owner", SheetID: coSheetID},
			{Title: "_status", SheetID: statusSheetID},
		},
		charOwnerRows: [][]any{
			{"char_name"}, // header
			{"Slampeach"}, // row 2 (index 1)
			{"Foo"},       // row 3 (index 2)
		},
		statusRows: [][]any{
			{"owner_email", "char_name"},        // header
			{"alice@example.com", "Slampeach"},  // row 2
			{"alice@example.com", "Foo"},        // row 3
		},
	}
	c, srv := newHBClient(t, h)
	defer srv.Close()

	if err := c.WriteHeartbeat(context.Background(), "alice@example.com",
		[]string{"Slampeach", "Foo"}, "0.2.0"); err != nil {
		t.Fatalf("WriteHeartbeat: %v", err)
	}

	if h.batchUpdateCount != 1 {
		t.Fatalf("batchUpdateCount = %d, want 1 (single fire)", h.batchUpdateCount)
	}
	got := countUpdateCells(h.batchUpdateBodies)
	if got != 8 {
		t.Errorf("UpdateCellsRequest count = %d, want 8 (2 char_owner + 2*3 status)", got)
	}
	if a := countAppendCells(h.batchUpdateBodies, statusSheetID); a != 0 {
		t.Errorf("AppendCells count = %d, want 0 (both rows pre-existing)", a)
	}
}

// Test 2: _char_owner.last_seen update targets column K (index 10) of the
// row matching char_name in column A. For "Foo" sitting at row index 2
// (third row, 0-indexed), the GridRange must be StartRow=2 EndRow=3,
// StartCol=10 EndCol=11.
func TestWriteHeartbeat_CharOwnerLastSeenColumnAndRow(t *testing.T) {
	const coSheetID = int64(7)
	const statusSheetID = int64(9)
	h := &hbStub{
		t: t,
		sheetsList: []sheetInfo{
			{Title: "_char_owner", SheetID: coSheetID},
			{Title: "_status", SheetID: statusSheetID},
		},
		charOwnerRows: [][]any{
			{"char_name"},
			{"Slampeach"},
			{"Foo"},
		},
		statusRows: [][]any{
			{"owner_email", "char_name"},
		},
	}
	c, srv := newHBClient(t, h)
	defer srv.Close()

	if err := c.WriteHeartbeat(context.Background(), "alice@example.com",
		[]string{"Foo"}, "0.2.0"); err != nil {
		t.Fatalf("WriteHeartbeat: %v", err)
	}

	// Find the UpdateCells against _char_owner.
	var found bool
	for _, batch := range h.batchUpdateBodies {
		for _, r := range batch.Requests {
			if r.UpdateCells == nil || r.UpdateCells.Range == nil {
				continue
			}
			if r.UpdateCells.Range.SheetId != coSheetID {
				continue
			}
			rng := r.UpdateCells.Range
			if rng.StartRowIndex != 2 || rng.EndRowIndex != 3 {
				t.Errorf("_char_owner row range = [%d, %d), want [2, 3)",
					rng.StartRowIndex, rng.EndRowIndex)
			}
			if rng.StartColumnIndex != 10 || rng.EndColumnIndex != 11 {
				t.Errorf("_char_owner col range = [%d, %d), want [10, 11) (K only)",
					rng.StartColumnIndex, rng.EndColumnIndex)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("did not find a UpdateCellsRequest against _char_owner")
	}
}

// Test 3: existing _status row -> THREE narrow UpdateCellsRequest blocks
// (col A=0:1, cols B:C=1:3, col F=5:6). Append branch NOT taken.
func TestWriteHeartbeat_StatusUpdate_ThreeNarrowBlocks(t *testing.T) {
	const coSheetID = int64(7)
	const statusSheetID = int64(9)
	h := &hbStub{
		t: t,
		sheetsList: []sheetInfo{
			{Title: "_char_owner", SheetID: coSheetID},
			{Title: "_status", SheetID: statusSheetID},
		},
		charOwnerRows: [][]any{
			{"char_name"},
			{"Foo"},
		},
		statusRows: [][]any{
			{"owner_email", "char_name"},
			{"alice@example.com", "Foo"},
		},
	}
	c, srv := newHBClient(t, h)
	defer srv.Close()

	if err := c.WriteHeartbeat(context.Background(), "alice@example.com",
		[]string{"Foo"}, "0.2.0"); err != nil {
		t.Fatalf("WriteHeartbeat: %v", err)
	}

	if a := countAppendCells(h.batchUpdateBodies, statusSheetID); a != 0 {
		t.Errorf("AppendCells against _status = %d, want 0 (row exists)", a)
	}

	// Count three specific column ranges against _status sheet.
	gotA := countUpdateCellsByColumn(h.batchUpdateBodies, 0, 1, statusSheetID)
	gotBC := countUpdateCellsByColumn(h.batchUpdateBodies, 1, 3, statusSheetID)
	gotF := countUpdateCellsByColumn(h.batchUpdateBodies, 5, 6, statusSheetID)

	// gotA includes the A-only block; gotBC includes the B:C block (which
	// also overlaps [0,1) -> nope, B:C is [1,3) so it does not overlap col 0).
	// We want exactly one block whose range is exactly [0,1), one [1,3), one [5,6).
	exact := func(start, end int64) int {
		count := 0
		for _, batch := range h.batchUpdateBodies {
			for _, r := range batch.Requests {
				if r.UpdateCells == nil || r.UpdateCells.Range == nil {
					continue
				}
				rng := r.UpdateCells.Range
				if rng.SheetId != statusSheetID {
					continue
				}
				if rng.StartColumnIndex == start && rng.EndColumnIndex == end {
					count++
				}
			}
		}
		return count
	}
	if got := exact(0, 1); got != 1 {
		t.Errorf("_status A-only blocks = %d, want 1; (range-overlap counts: A=%d, BC=%d, F=%d)",
			got, gotA, gotBC, gotF)
	}
	if got := exact(1, 3); got != 1 {
		t.Errorf("_status B:C blocks = %d, want 1", got)
	}
	if got := exact(5, 6); got != 1 {
		t.Errorf("_status F-only blocks = %d, want 1", got)
	}
}

// Test 4 (W6 preservation): heartbeat against a _status row with pre-
// populated D and E values must NOT emit any UpdateCellsRequest covering
// column 3 (D) or column 4 (E). Read-side bit-for-bit preservation is
// enforced by the absence of writes targeting those columns.
func TestWriteHeartbeat_PreservesStatusDAndE(t *testing.T) {
	const coSheetID = int64(7)
	const statusSheetID = int64(9)
	h := &hbStub{
		t: t,
		sheetsList: []sheetInfo{
			{Title: "_char_owner", SheetID: coSheetID},
			{Title: "_status", SheetID: statusSheetID},
		},
		charOwnerRows: [][]any{
			{"char_name"},
			{"Foo"},
		},
		// Pre-populated D=last_inventory_upload, E=last_spellbook_upload.
		// (Note our valuesGet only reads columns A:B, so the actual on-disk
		// D/E values aren't visible in the captured request -- but they exist
		// in the sheet. The contract is that the heartbeat MUST NOT emit any
		// UpdateCellsRequest that would clobber columns 3 or 4.)
		statusRows: [][]any{
			{"owner_email", "char_name"},
			{"alice@example.com", "Foo"},
		},
	}
	c, srv := newHBClient(t, h)
	defer srv.Close()

	if err := c.WriteHeartbeat(context.Background(), "alice@example.com",
		[]string{"Foo"}, "0.2.0"); err != nil {
		t.Fatalf("WriteHeartbeat: %v", err)
	}

	// Hard assertion: ZERO UpdateCellsRequests against _status overlap
	// columns 3 (D) or 4 (E).
	gotD := countUpdateCellsByColumn(h.batchUpdateBodies, 3, 4, statusSheetID)
	gotE := countUpdateCellsByColumn(h.batchUpdateBodies, 4, 5, statusSheetID)
	if gotD != 0 {
		t.Errorf("UpdateCells covering col D (3) = %d, want 0 (D=last_inventory_upload must be preserved)", gotD)
	}
	if gotE != 0 {
		t.Errorf("UpdateCells covering col E (4) = %d, want 0 (E=last_spellbook_upload must be preserved)", gotE)
	}
}

// Test 5: all written cells are StringValue + every UpdateCellsRequest has
// Fields="userEnteredValue" (Pitfall #8).
func TestWriteHeartbeat_StringValueAndFieldsContract(t *testing.T) {
	const coSheetID = int64(7)
	const statusSheetID = int64(9)
	h := &hbStub{
		t: t,
		sheetsList: []sheetInfo{
			{Title: "_char_owner", SheetID: coSheetID},
			{Title: "_status", SheetID: statusSheetID},
		},
		charOwnerRows: [][]any{
			{"char_name"},
			{"Foo"},
		},
		statusRows: [][]any{
			{"owner_email", "char_name"},
		},
	}
	c, srv := newHBClient(t, h)
	defer srv.Close()

	if err := c.WriteHeartbeat(context.Background(), "alice@example.com",
		[]string{"Foo"}, "0.2.0"); err != nil {
		t.Fatalf("WriteHeartbeat: %v", err)
	}

	for _, batch := range h.batchUpdateBodies {
		for _, r := range batch.Requests {
			if r.UpdateCells != nil {
				if r.UpdateCells.Fields != "userEnteredValue" {
					t.Errorf("UpdateCells.Fields = %q, want \"userEnteredValue\"",
						r.UpdateCells.Fields)
				}
				for _, row := range r.UpdateCells.Rows {
					for _, cell := range row.Values {
						if cell.UserEnteredValue == nil {
							t.Errorf("UpdateCells cell missing UserEnteredValue")
							continue
						}
						if cell.UserEnteredValue.StringValue == nil {
							t.Errorf("cell not StringValue (Pitfall #8): %+v", cell.UserEnteredValue)
						}
						if cell.UserEnteredValue.NumberValue != nil {
							t.Errorf("cell has NumberValue set: %+v", cell.UserEnteredValue)
						}
					}
				}
			}
			if r.AppendCells != nil {
				if r.AppendCells.Fields != "userEnteredValue" {
					t.Errorf("AppendCells.Fields = %q, want \"userEnteredValue\"",
						r.AppendCells.Fields)
				}
				for _, row := range r.AppendCells.Rows {
					for _, cell := range row.Values {
						if cell.UserEnteredValue == nil {
							t.Errorf("AppendCells cell missing UserEnteredValue")
							continue
						}
						if cell.UserEnteredValue.StringValue == nil {
							t.Errorf("Append cell not StringValue: %+v", cell.UserEnteredValue)
						}
					}
				}
			}
		}
	}
}

// Test 6: empty charNames -> nil error AND zero HTTP calls (no-op fast
// path).
func TestWriteHeartbeat_EmptyCharNamesIsNoOp(t *testing.T) {
	h := &hbStub{
		t: t,
		sheetsList: []sheetInfo{
			{Title: "_char_owner", SheetID: 7},
			{Title: "_status", SheetID: 9},
		},
	}
	c, srv := newHBClient(t, h)
	defer srv.Close()

	if err := c.WriteHeartbeat(context.Background(), "alice@example.com", nil, "0.2.0"); err != nil {
		t.Fatalf("WriteHeartbeat empty: %v", err)
	}
	if got := atomic.LoadInt64(&h.httpCalls); got != 0 {
		t.Errorf("HTTP calls = %d, want 0 (empty charNames must be no-op)", got)
	}
}

// Test 7: char NOT in _char_owner -> last_seen update skipped for that
// char; _status row still upserted (appended, since absent).
func TestWriteHeartbeat_SkipsCharOwnerForUnknownChar(t *testing.T) {
	const coSheetID = int64(7)
	const statusSheetID = int64(9)
	h := &hbStub{
		t: t,
		sheetsList: []sheetInfo{
			{Title: "_char_owner", SheetID: coSheetID},
			{Title: "_status", SheetID: statusSheetID},
		},
		charOwnerRows: [][]any{
			{"char_name"},
			{"Foo"},
		},
		statusRows: [][]any{
			{"owner_email", "char_name"},
		},
	}
	c, srv := newHBClient(t, h)
	defer srv.Close()

	// "NewChar" is in neither _char_owner nor _status.
	if err := c.WriteHeartbeat(context.Background(), "alice@example.com",
		[]string{"NewChar"}, "0.2.0"); err != nil {
		t.Fatalf("WriteHeartbeat: %v", err)
	}

	// 0 UpdateCells against _char_owner (char not there).
	if u := countUpdateCellsByColumn(h.batchUpdateBodies, 10, 11, coSheetID); u != 0 {
		t.Errorf("_char_owner UpdateCells for unknown char = %d, want 0", u)
	}
	// 1 AppendCells against _status (row not there).
	if a := countAppendCells(h.batchUpdateBodies, statusSheetID); a != 1 {
		t.Errorf("_status AppendCells = %d, want 1", a)
	}
}

// Test 8: AppendCells branch writes all 6 cells (A through F) for net-new
// _status rows; D and E are empty strings.
func TestWriteHeartbeat_AppendBranchWritesAllSixCells(t *testing.T) {
	const coSheetID = int64(7)
	const statusSheetID = int64(9)
	h := &hbStub{
		t: t,
		sheetsList: []sheetInfo{
			{Title: "_char_owner", SheetID: coSheetID},
			{Title: "_status", SheetID: statusSheetID},
		},
		charOwnerRows: [][]any{
			{"char_name"},
			{"Foo"},
		},
		statusRows: [][]any{
			{"owner_email", "char_name"},
		},
	}
	c, srv := newHBClient(t, h)
	defer srv.Close()

	const ownerEmail = "alice@example.com"
	const watcherVersion = "0.2.0"
	if err := c.WriteHeartbeat(context.Background(), ownerEmail,
		[]string{"Foo"}, watcherVersion); err != nil {
		t.Fatalf("WriteHeartbeat: %v", err)
	}

	var appendReq *sheets.AppendCellsRequest
	for _, batch := range h.batchUpdateBodies {
		for _, r := range batch.Requests {
			if r.AppendCells != nil && r.AppendCells.SheetId == statusSheetID {
				appendReq = r.AppendCells
			}
		}
	}
	if appendReq == nil {
		t.Fatal("no AppendCellsRequest against _status (expected for net-new row)")
	}
	if len(appendReq.Rows) != 1 {
		t.Fatalf("Append rows = %d, want 1", len(appendReq.Rows))
	}
	cells := appendReq.Rows[0].Values
	if len(cells) != 6 {
		t.Fatalf("Append cell count = %d, want 6 (A..F)", len(cells))
	}

	want := func(idx int, val string, name string) {
		if cells[idx].UserEnteredValue == nil || cells[idx].UserEnteredValue.StringValue == nil {
			t.Errorf("cell %s missing StringValue", name)
			return
		}
		if got := *cells[idx].UserEnteredValue.StringValue; got != val {
			t.Errorf("cell %s = %q, want %q", name, got, val)
		}
	}
	want(0, ownerEmail, "A=owner_email")
	want(1, "Foo", "B=char_name")
	want(2, watcherVersion, "C=watcher_version")
	want(3, "", "D=last_inventory_upload (empty for net-new)")
	want(4, "", "E=last_spellbook_upload (empty for net-new)")
	// F=last_heartbeat must be RFC3339.
	if cells[5].UserEnteredValue == nil || cells[5].UserEnteredValue.StringValue == nil {
		t.Fatal("cell F missing StringValue")
	}
	iso := *cells[5].UserEnteredValue.StringValue
	if _, err := time.Parse(time.RFC3339, iso); err != nil {
		t.Errorf("cell F=last_heartbeat %q not RFC3339: %v", iso, err)
	}
}
