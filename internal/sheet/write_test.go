package sheet

// Tests for WriteInventory (Plan 01-05 Task 2). Verifies the
// "exactly one batchUpdate, exactly one UpdateCellsRequest" contract
// (Critical Constraint #3 + RESEARCH.md §2.3 Pattern 1) and the
// "every cell is StringValue, never NumberValue" contract
// (Critical Constraint #8 + Pitfall #8).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// writeStub is a minimal stub that captures every batchUpdate body.
// It also serves a fixed sheets list for EnsureSheet's pre-flight Get.
type writeStub struct {
	t            *testing.T
	sheetsList   []sheetInfo
	batchUpdates []*sheets.BatchUpdateSpreadsheetRequest
	getCalls     int
}

func (s *writeStub) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case r.Method == "GET" && strings.HasSuffix(path, "/spreadsheets/SHEET1"):
			s.getCalls++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"spreadsheetId": "SHEET1",
				"sheets":        encodeSheetsList(s.sheetsList),
			})
		case r.Method == "POST" && strings.HasSuffix(path, ":batchUpdate"):
			var req sheets.BatchUpdateSpreadsheetRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				s.t.Fatalf("decode batchUpdate: %v", err)
			}
			s.batchUpdates = append(s.batchUpdates, &req)
			// Mint AddSheet replies if asked.
			replies := make([]map[string]any, 0, len(req.Requests))
			for _, rq := range req.Requests {
				if rq.AddSheet != nil {
					title := rq.AddSheet.Properties.Title
					newID := int64(80000 + len(s.sheetsList))
					s.sheetsList = append(s.sheetsList, sheetInfo{Title: title, SheetID: newID})
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
			s.t.Logf("UNHANDLED: %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func newWriteClient(t *testing.T, s *writeStub) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(s.handler())
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

// Test 1 + 2 + 3 + 4 + 7 — atomic single call shape.
func TestWriteInventory_AtomicSingleCall(t *testing.T) {
	s := &writeStub{
		t:          t,
		sheetsList: []sheetInfo{{Title: "inv:Foo", SheetID: 999}},
	}
	c, srv := newWriteClient(t, s)
	defer srv.Close()
	c.tabs["inv:Foo"] = 999 // pre-cache so EnsureSheet does not issue a Get

	err := c.WriteInventory(context.Background(), "Foo", InventoryHeader,
		[][]string{{"General1", "Cloth Cap", "1001", "1", "0"}},
		"2026-04-30T18:00:00Z")
	if err != nil {
		t.Fatalf("WriteInventory: %v", err)
	}

	// Test 1: exactly ONE batchUpdate.
	if len(s.batchUpdates) != 1 {
		t.Fatalf("batchUpdates = %d, want 1", len(s.batchUpdates))
	}
	req := s.batchUpdates[0]
	if len(req.Requests) != 1 {
		t.Fatalf("inner Requests = %d, want 1", len(req.Requests))
	}
	uc := req.Requests[0].UpdateCells
	if uc == nil {
		t.Fatal("expected UpdateCells request")
	}

	// Test 2: GridRange dimensions.
	if uc.Range.SheetId != 999 {
		t.Errorf("SheetId = %d, want 999", uc.Range.SheetId)
	}
	if uc.Range.StartRowIndex != 0 {
		t.Errorf("StartRowIndex = %d, want 0", uc.Range.StartRowIndex)
	}
	if uc.Range.EndRowIndex != 500 {
		t.Errorf("EndRowIndex = %d, want 500", uc.Range.EndRowIndex)
	}
	if uc.Range.StartColumnIndex != 0 {
		t.Errorf("StartColumnIndex = %d, want 0", uc.Range.StartColumnIndex)
	}
	if uc.Range.EndColumnIndex != 6 {
		t.Errorf("EndColumnIndex = %d, want 6", uc.Range.EndColumnIndex)
	}

	// Test 3: Fields exactly "userEnteredValue" — no wildcard, no commas.
	if uc.Fields != "userEnteredValue" {
		t.Errorf("Fields = %q, want \"userEnteredValue\"", uc.Fields)
	}

	// Test 4: rows = header + 1 data row.
	if len(uc.Rows) != 2 {
		t.Fatalf("Rows = %d, want 2 (header + 1 data)", len(uc.Rows))
	}
	header := uc.Rows[0]
	if len(header.Values) != 6 {
		t.Errorf("header cells = %d, want 6", len(header.Values))
	}
	for i, want := range InventoryHeader {
		got := header.Values[i].UserEnteredValue
		if got == nil || got.StringValue == nil {
			t.Errorf("header[%d] not a StringValue", i)
			continue
		}
		if *got.StringValue != want {
			t.Errorf("header[%d] = %q, want %q", i, *got.StringValue, want)
		}
	}

	// Test 4 + 7: each cell is StringValue, last column = uploaded_at.
	data := uc.Rows[1]
	if len(data.Values) != 6 {
		t.Fatalf("data cells = %d, want 6", len(data.Values))
	}
	wantData := []string{"General1", "Cloth Cap", "1001", "1", "0", "2026-04-30T18:00:00Z"}
	for i, want := range wantData {
		ev := data.Values[i].UserEnteredValue
		if ev == nil {
			t.Errorf("data[%d] has nil UserEnteredValue", i)
			continue
		}
		if ev.StringValue == nil || *ev.StringValue != want {
			t.Errorf("data[%d] StringValue = %v, want %q", i, ev.StringValue, want)
		}
		// Pitfall #8: NumberValue MUST be unset for every cell, including ID/Count.
		if ev.NumberValue != nil {
			t.Errorf("data[%d] has NumberValue = %v — Pitfall #8 violation", i, *ev.NumberValue)
		}
		if ev.FormulaValue != nil {
			t.Errorf("data[%d] has FormulaValue = %v — must be StringValue only", i, *ev.FormulaValue)
		}
	}
}

// Test 5: WriteInventory issues EnsureSheet's AddSheet on a fresh character.
// We start with an empty sheets list so EnsureSheet must AddSheet, and we
// expect exactly TWO batchUpdates: one AddSheet, then one UpdateCells.
func TestWriteInventory_EnsureSheetCreatesOnFirstSighting(t *testing.T) {
	s := &writeStub{
		t:          t,
		sheetsList: []sheetInfo{}, // workbook has no inv:Bar yet
	}
	c, srv := newWriteClient(t, s)
	defer srv.Close()

	err := c.WriteInventory(context.Background(), "Bar", InventoryHeader,
		[][]string{{"Pack3", "Spider Silk", "1100", "5", "0"}},
		"2026-04-30T19:00:00Z")
	if err != nil {
		t.Fatalf("WriteInventory: %v", err)
	}
	if len(s.batchUpdates) != 2 {
		t.Fatalf("batchUpdates = %d, want 2 (AddSheet then UpdateCells)", len(s.batchUpdates))
	}
	if s.batchUpdates[0].Requests[0].AddSheet == nil {
		t.Errorf("first batchUpdate not AddSheet")
	}
	if s.batchUpdates[0].Requests[0].AddSheet.Properties.Title != "inv:Bar" {
		t.Errorf("AddSheet title = %q, want inv:Bar",
			s.batchUpdates[0].Requests[0].AddSheet.Properties.Title)
	}
	if s.batchUpdates[1].Requests[0].UpdateCells == nil {
		t.Errorf("second batchUpdate not UpdateCells")
	}
}

// Test 6: 0 dataRows — header-only write must still happen. The atomic
// clear semantic means the rest of A1:F500 is wiped out, which is the
// "user emptied bag" regression case.
func TestWriteInventory_EmptyInventoryClearsRange(t *testing.T) {
	s := &writeStub{
		t:          t,
		sheetsList: []sheetInfo{{Title: "inv:Empty", SheetID: 1234}},
	}
	c, srv := newWriteClient(t, s)
	defer srv.Close()
	c.tabs["inv:Empty"] = 1234

	err := c.WriteInventory(context.Background(), "Empty", InventoryHeader,
		nil, "2026-04-30T20:00:00Z")
	if err != nil {
		t.Fatalf("WriteInventory: %v", err)
	}
	if len(s.batchUpdates) != 1 {
		t.Fatalf("batchUpdates = %d, want 1", len(s.batchUpdates))
	}
	uc := s.batchUpdates[0].Requests[0].UpdateCells
	if uc == nil {
		t.Fatal("expected UpdateCells")
	}
	if len(uc.Rows) != 1 {
		t.Errorf("Rows = %d, want 1 (header only)", len(uc.Rows))
	}
	// Range still spans A1:F500 so cells outside row 0 are cleared atomically.
	if uc.Range.EndRowIndex != 500 {
		t.Errorf("EndRowIndex = %d, want 500 (must still clear stale rows)", uc.Range.EndRowIndex)
	}
}

// Defensive: if the parser hands us a short row (<5 cells), WriteInventory
// pads to 5 and appends uploaded_at. Mirrors the "(defensive — parser
// already filters <5)" comment in write.go.
func TestWriteInventory_ShortRowIsPadded(t *testing.T) {
	s := &writeStub{
		t:          t,
		sheetsList: []sheetInfo{{Title: "inv:Pad", SheetID: 5555}},
	}
	c, srv := newWriteClient(t, s)
	defer srv.Close()
	c.tabs["inv:Pad"] = 5555

	err := c.WriteInventory(context.Background(), "Pad", InventoryHeader,
		[][]string{{"General2", "Short Sword"}}, // only 2 cells
		"2026-04-30T21:00:00Z")
	if err != nil {
		t.Fatalf("WriteInventory: %v", err)
	}
	uc := s.batchUpdates[0].Requests[0].UpdateCells
	if len(uc.Rows[1].Values) != 6 {
		t.Errorf("padded row cells = %d, want 6", len(uc.Rows[1].Values))
	}
	// Cell 5 (column F) is uploaded_at.
	last := uc.Rows[1].Values[5].UserEnteredValue
	if last == nil || last.StringValue == nil || *last.StringValue != "2026-04-30T21:00:00Z" {
		t.Errorf("uploaded_at column F not set correctly: %v", last)
	}
	// Cells 2,3,4 are empty strings (padding).
	for i := 2; i <= 4; i++ {
		ev := uc.Rows[1].Values[i].UserEnteredValue
		if ev == nil || ev.StringValue == nil || *ev.StringValue != "" {
			t.Errorf("padded cell %d = %v, want empty StringValue", i, ev)
		}
	}
}

// Spreadsheet not configured → return error rather than 4xx the API.
func TestWriteInventory_NoSpreadsheetID(t *testing.T) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "fake"})
	c, err := NewClient(ctx, ts, "")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := c.WriteInventory(ctx, "Foo", InventoryHeader, nil, ""); err == nil {
		t.Fatal("expected error when spreadsheetID empty, got nil")
	}
}
