package sheet

// Tests for ValidateWorkbook (Plan 02-01 Task 1 — three-state refactor) +
// EnsureSheet. Use httptest.NewServer + option.WithEndpoint to point the
// Sheets v4 client at a stub server that returns canned JSON for the
// endpoints we touch:
//
//   GET /v4/spreadsheets/{id}                       — list sheets
//   GET /v4/spreadsheets/{id}/values/{range}        — read _meta cells
//   POST /v4/spreadsheets/{id}:batchUpdate          — addSheet
//
// Plan 02-01 Task 1: bootstrapMeta is DELETED from the sheet package; the
// ScaffoldSchemaV1 routine in internal/scaffold owns all _meta row writes
// now. ValidateWorkbook only reads.
//
// No real GCP calls — see RESEARCH.md §2.6 + the Plan 05 test strategy note.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// fakeSheetsHandler captures requests against an in-memory model of the
// stub spreadsheet "SHEET1". metaValues is the response body for the
// values.get on _meta!A1:B20.
type fakeSheetsHandler struct {
	t              *testing.T
	metaValues     [][]any // empty slice (or nil) means "no values"
	sheetsList     []sheetInfo
	addSheetReqs   []string
	getCalls       int
	valuesGetCalls int
	valuesPutCalls int
	batchUpdates   []*sheets.BatchUpdateSpreadsheetRequest
}

type sheetInfo struct {
	Title   string
	SheetID int64
}

func (h *fakeSheetsHandler) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		// GET /v4/spreadsheets/SHEET1 (with fields filter from EnsureSheet)
		case r.Method == "GET" && strings.HasSuffix(path, "/spreadsheets/SHEET1"):
			h.getCalls++
			out := map[string]any{
				"spreadsheetId": "SHEET1",
				"sheets":        encodeSheetsList(h.sheetsList),
			}
			_ = json.NewEncoder(w).Encode(out)

		// GET /v4/spreadsheets/SHEET1/values/_meta!A1:B20
		case r.Method == "GET" && strings.Contains(path, "/values/"):
			h.valuesGetCalls++
			body := map[string]any{"range": "_meta!A1:B20", "majorDimension": "ROWS"}
			if len(h.metaValues) > 0 {
				body["values"] = h.metaValues
			}
			_ = json.NewEncoder(w).Encode(body)

		// PUT /v4/spreadsheets/SHEET1/values/{range} — should NEVER be hit
		// from ValidateWorkbook in the new design (bootstrapMeta deleted).
		case r.Method == "PUT" && strings.Contains(path, "/values/"):
			h.valuesPutCalls++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"updatedRange": "_meta!A1:B20",
				"updatedRows":  0,
				"updatedCells": 0,
			})

		// POST /v4/spreadsheets/SHEET1:batchUpdate
		case r.Method == "POST" && strings.HasSuffix(path, ":batchUpdate"):
			var req sheets.BatchUpdateSpreadsheetRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			h.batchUpdates = append(h.batchUpdates, &req)
			replies := make([]map[string]any, 0, len(req.Requests))
			for _, rq := range req.Requests {
				if rq.AddSheet != nil {
					title := rq.AddSheet.Properties.Title
					h.addSheetReqs = append(h.addSheetReqs, title)
					newID := int64(70000 + len(h.sheetsList))
					h.sheetsList = append(h.sheetsList, sheetInfo{Title: title, SheetID: newID})
					replies = append(replies, map[string]any{
						"addSheet": map[string]any{
							"properties": map[string]any{
								"title":   title,
								"sheetId": newID,
							},
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

func encodeSheetsList(list []sheetInfo) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for _, s := range list {
		out = append(out, map[string]any{
			"properties": map[string]any{
				"title":   s.Title,
				"sheetId": s.SheetID,
			},
		})
	}
	return out
}

// newTestClient spins up the stub server and returns a Client wired to it.
func newTestClient(t *testing.T, h *fakeSheetsHandler) (*Client, *httptest.Server) {
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

// ----- ValidateWorkbook three-state tests (Plan 02-01 Task 1) -----

// Test 1 (per <behavior>): _meta tab absent → caller must EnsureSheet,
// then read returns zero rows → WorkbookStateEmpty, no error.
func TestValidateWorkbook_EmptyNoMetaTab(t *testing.T) {
	h := &fakeSheetsHandler{
		t:          t,
		sheetsList: []sheetInfo{}, // _meta missing — EnsureSheet will create it
		metaValues: nil,           // newly-created tab has no rows
	}
	c, srv := newTestClient(t, h)
	defer srv.Close()

	state, err := c.ValidateWorkbook(context.Background())
	if err != nil {
		t.Fatalf("ValidateWorkbook: unexpected error %v", err)
	}
	if state != WorkbookStateEmpty {
		t.Errorf("state = %v, want WorkbookStateEmpty", state)
	}
	// CRITICAL: must NOT auto-bootstrap (bootstrapMeta is deleted).
	if h.valuesPutCalls != 0 {
		t.Errorf("ValidateWorkbook wrote %d values.update calls; must be 0 (scaffold owns _meta writes now)", h.valuesPutCalls)
	}
}

// Test 2: _meta tab exists but contains zero rows → Empty.
func TestValidateWorkbook_EmptyMetaTabZeroRows(t *testing.T) {
	h := &fakeSheetsHandler{
		t:          t,
		sheetsList: []sheetInfo{{Title: "_meta", SheetID: 12345}},
		metaValues: nil, // tab exists, no rows
	}
	c, srv := newTestClient(t, h)
	defer srv.Close()

	state, err := c.ValidateWorkbook(context.Background())
	if err != nil {
		t.Fatalf("ValidateWorkbook: unexpected error %v", err)
	}
	if state != WorkbookStateEmpty {
		t.Errorf("state = %v, want WorkbookStateEmpty", state)
	}
	if h.valuesPutCalls != 0 {
		t.Errorf("valuesPutCalls = %d, want 0", h.valuesPutCalls)
	}
}

// Test 3: canonical_id matches and schema_version=1 → Matches.
func TestValidateWorkbook_MatchesHealthy(t *testing.T) {
	h := &fakeSheetsHandler{
		t:          t,
		sheetsList: []sheetInfo{{Title: "_meta", SheetID: 12345}},
		metaValues: [][]any{
			{"schema_version", "1"},
			{"canonical_id", CanonicalID},
		},
	}
	c, srv := newTestClient(t, h)
	defer srv.Close()

	state, err := c.ValidateWorkbook(context.Background())
	if err != nil {
		t.Fatalf("ValidateWorkbook: unexpected error %v", err)
	}
	if state != WorkbookStateMatches {
		t.Errorf("state = %v, want WorkbookStateMatches", state)
	}
	if h.valuesPutCalls != 0 {
		t.Errorf("valuesPutCalls = %d, want 0 (healthy path)", h.valuesPutCalls)
	}
}

// Test 4: canonical_id mismatches → Wrong, ErrWrongWorkbook.
func TestValidateWorkbook_WrongCanonicalID(t *testing.T) {
	h := &fakeSheetsHandler{
		t:          t,
		sheetsList: []sheetInfo{{Title: "_meta", SheetID: 12345}},
		metaValues: [][]any{
			{"canonical_id", "different-canonical-id"},
			{"schema_version", "1"},
		},
	}
	c, srv := newTestClient(t, h)
	defer srv.Close()

	state, err := c.ValidateWorkbook(context.Background())
	if state != WorkbookStateWrong {
		t.Errorf("state = %v, want WorkbookStateWrong", state)
	}
	if err == nil {
		t.Fatal("expected ErrWrongWorkbook, got nil")
	}
	if !errors.Is(err, ErrWrongWorkbook) {
		t.Errorf("err = %v; want ErrWrongWorkbook", err)
	}
	const verbatim = "This doesn't look like a SquireBot workbook. Pick the one shared by your guild leader."
	if !strings.Contains(err.Error(), verbatim) {
		t.Errorf("error text missing D-03 verbatim message:\ngot:  %q\nwant contains: %q", err.Error(), verbatim)
	}
}

// Test 5: canonical_id matches but schema_version > max → Matches state
// (the workbook IS ours), but with ErrSchemaTooNew error so caller refuses
// to write.
func TestValidateWorkbook_MatchesSchemaTooNew(t *testing.T) {
	h := &fakeSheetsHandler{
		t:          t,
		sheetsList: []sheetInfo{{Title: "_meta", SheetID: 12345}},
		metaValues: [][]any{
			{"canonical_id", CanonicalID},
			{"schema_version", "2"},
		},
	}
	c, srv := newTestClient(t, h)
	defer srv.Close()

	state, err := c.ValidateWorkbook(context.Background())
	if state != WorkbookStateMatches {
		t.Errorf("state = %v, want WorkbookStateMatches (schema-too-new is still our workbook)", state)
	}
	if err == nil {
		t.Fatal("expected ErrSchemaTooNew, got nil")
	}
	if !errors.Is(err, ErrSchemaTooNew) {
		t.Errorf("err = %v; want ErrSchemaTooNew", err)
	}
}

// Test 6 (Pitfall C defensive): _meta tab has rows but no canonical_id
// row → Wrong (refuse rather than scaffold over user data).
func TestValidateWorkbook_MetaWithoutCanonicalIDIsWrong(t *testing.T) {
	h := &fakeSheetsHandler{
		t:          t,
		sheetsList: []sheetInfo{{Title: "_meta", SheetID: 12345}},
		metaValues: [][]any{
			// _meta tab has unrelated content but no canonical_id row.
			{"some_other_key", "some_value"},
			{"another_key", "another_value"},
		},
	}
	c, srv := newTestClient(t, h)
	defer srv.Close()

	state, err := c.ValidateWorkbook(context.Background())
	if state != WorkbookStateWrong {
		t.Errorf("state = %v, want WorkbookStateWrong (Pitfall C — _meta with rows but no canonical_id is suspect)", state)
	}
	if !errors.Is(err, ErrWrongWorkbook) {
		t.Errorf("err = %v; want ErrWrongWorkbook", err)
	}
}

// ----- EnsureSheet tests (Phase 1 — preserved) -----

func TestEnsureSheet_Existing(t *testing.T) {
	h := &fakeSheetsHandler{
		t: t,
		sheetsList: []sheetInfo{
			{Title: "_meta", SheetID: 1},
			{Title: "inv:Foo", SheetID: 99},
		},
	}
	c, srv := newTestClient(t, h)
	defer srv.Close()

	id, err := c.EnsureSheet(context.Background(), "inv:Foo")
	if err != nil {
		t.Fatalf("EnsureSheet: %v", err)
	}
	if id != 99 {
		t.Errorf("sheetId = %d, want 99", id)
	}
	if len(h.addSheetReqs) != 0 {
		t.Errorf("expected zero AddSheet requests, got %d", len(h.addSheetReqs))
	}
	prevGet := h.getCalls
	if _, err := c.EnsureSheet(context.Background(), "inv:Foo"); err != nil {
		t.Fatal(err)
	}
	if h.getCalls != prevGet {
		t.Errorf("EnsureSheet cache miss: getCalls grew from %d to %d", prevGet, h.getCalls)
	}
}

func TestEnsureSheet_Creates(t *testing.T) {
	h := &fakeSheetsHandler{
		t:          t,
		sheetsList: []sheetInfo{{Title: "_meta", SheetID: 1}},
	}
	c, srv := newTestClient(t, h)
	defer srv.Close()

	id, err := c.EnsureSheet(context.Background(), "inv:Bar")
	if err != nil {
		t.Fatalf("EnsureSheet: %v", err)
	}
	if id == 0 {
		t.Errorf("sheetId = 0, expected non-zero from AddSheet reply")
	}
	if len(h.addSheetReqs) != 1 || h.addSheetReqs[0] != "inv:Bar" {
		t.Errorf("addSheetReqs = %v, want [inv:Bar]", h.addSheetReqs)
	}
	prevBatch := len(h.batchUpdates)
	if _, err := c.EnsureSheet(context.Background(), "inv:Bar"); err != nil {
		t.Fatal(err)
	}
	if len(h.batchUpdates) != prevBatch {
		t.Errorf("EnsureSheet did not cache after AddSheet — extra batchUpdate")
	}
}
