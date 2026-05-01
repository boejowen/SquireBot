package sheet

// Tests for ValidateWorkbook (Plan 01-05 Task 1) + EnsureSheet.
// Use httptest.NewServer + option.WithEndpoint to point the Sheets v4 client
// at a stub server that returns canned JSON for the four endpoints we touch:
//   GET /v4/spreadsheets/{id}                       — list sheets
//   GET /v4/spreadsheets/{id}/values/{range}        — read _meta cells
//   PUT /v4/spreadsheets/{id}/values/{range}        — bootstrap write
//   POST /v4/spreadsheets/{id}:batchUpdate          — addSheet
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
// values.get on _meta!A1:B2; sheetsList is the addSheet starting state.
type fakeSheetsHandler struct {
	t              *testing.T
	metaValues     [][]any   // empty slice (or nil) means "no values"
	sheetsList     []sheetInfo
	bootstrapWrote *sheets.ValueRange
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

		// GET /v4/spreadsheets/SHEET1/values/_meta!A1:B2
		case r.Method == "GET" && strings.Contains(path, "/values/"):
			h.valuesGetCalls++
			body := map[string]any{"range": "_meta!A1:B2", "majorDimension": "ROWS"}
			if len(h.metaValues) > 0 {
				body["values"] = h.metaValues
			}
			_ = json.NewEncoder(w).Encode(body)

		// PUT /v4/spreadsheets/SHEET1/values/_meta!A1:B2
		case r.Method == "PUT" && strings.Contains(path, "/values/"):
			h.valuesPutCalls++
			var vr sheets.ValueRange
			_ = json.NewDecoder(r.Body).Decode(&vr)
			h.bootstrapWrote = &vr
			_ = json.NewEncoder(w).Encode(map[string]any{
				"updatedRange": "_meta!A1:B2",
				"updatedRows":  2,
				"updatedCells": 4,
			})

		// POST /v4/spreadsheets/SHEET1:batchUpdate
		case r.Method == "POST" && strings.HasSuffix(path, ":batchUpdate"):
			var req sheets.BatchUpdateSpreadsheetRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			h.batchUpdates = append(h.batchUpdates, &req)
			// If any requests are AddSheet, mint a new sheetId and reply with it.
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

// ----- ValidateWorkbook tests -----

func TestValidateWorkbook_Bootstrap(t *testing.T) {
	// _meta tab exists; A1:B2 is empty → BOOTSTRAP path runs.
	h := &fakeSheetsHandler{
		t:          t,
		sheetsList: []sheetInfo{{Title: "_meta", SheetID: 12345}},
		metaValues: nil, // empty
	}
	c, srv := newTestClient(t, h)
	defer srv.Close()

	if err := c.ValidateWorkbook(context.Background()); err != nil {
		t.Fatalf("ValidateWorkbook: %v", err)
	}
	if h.bootstrapWrote == nil {
		t.Fatal("expected bootstrap PUT to _meta!A1:B2 — none captured")
	}
	if got := h.valuesPutCalls; got != 1 {
		t.Errorf("valuesPutCalls = %d, want 1", got)
	}
	// Verify the body shape: 2 rows, [canonical_id, squirebot-v1-workbook-2026], [schema_version, "1"]
	if len(h.bootstrapWrote.Values) != 2 {
		t.Fatalf("bootstrap rows = %d, want 2", len(h.bootstrapWrote.Values))
	}
	r0 := h.bootstrapWrote.Values[0]
	if len(r0) != 2 || r0[0] != "canonical_id" || r0[1] != CanonicalID {
		t.Errorf("row 0 = %v, want [canonical_id, %s]", r0, CanonicalID)
	}
	r1 := h.bootstrapWrote.Values[1]
	if len(r1) != 2 || r1[0] != "schema_version" || r1[1] != "1" {
		t.Errorf("row 1 = %v, want [schema_version, 1]", r1)
	}
}

func TestValidateWorkbook_Healthy(t *testing.T) {
	h := &fakeSheetsHandler{
		t:          t,
		sheetsList: []sheetInfo{{Title: "_meta", SheetID: 12345}},
		metaValues: [][]any{
			{"canonical_id", CanonicalID},
			{"schema_version", "1"},
		},
	}
	c, srv := newTestClient(t, h)
	defer srv.Close()

	if err := c.ValidateWorkbook(context.Background()); err != nil {
		t.Fatalf("ValidateWorkbook: %v", err)
	}
	if h.valuesPutCalls != 0 {
		t.Errorf("expected zero PUTs (healthy path), got %d", h.valuesPutCalls)
	}
}

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

	err := c.ValidateWorkbook(context.Background())
	if err == nil {
		t.Fatal("expected ErrWrongWorkbook, got nil")
	}
	if !errors.Is(err, ErrWrongWorkbook) {
		t.Errorf("err = %v; want ErrWrongWorkbook (errors.Is)", err)
	}
	// Verbatim D-03 message must be present in the error text.
	const verbatim = "This doesn't look like a SquireBot workbook. Pick the one shared by your guild leader."
	if !strings.Contains(err.Error(), verbatim) {
		t.Errorf("error text missing D-03 verbatim message:\ngot:  %q\nwant contains: %q", err.Error(), verbatim)
	}
}

func TestValidateWorkbook_SchemaTooNew(t *testing.T) {
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

	err := c.ValidateWorkbook(context.Background())
	if err == nil {
		t.Fatal("expected ErrSchemaTooNew, got nil")
	}
	if !errors.Is(err, ErrSchemaTooNew) {
		t.Errorf("err = %v; want ErrSchemaTooNew (errors.Is)", err)
	}
}

// ----- EnsureSheet tests -----

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
	// Cached: a second call must not hit the wire.
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
	// Cached after creation.
	prevBatch := len(h.batchUpdates)
	if _, err := c.EnsureSheet(context.Background(), "inv:Bar"); err != nil {
		t.Fatal(err)
	}
	if len(h.batchUpdates) != prevBatch {
		t.Errorf("EnsureSheet did not cache after AddSheet — extra batchUpdate")
	}
}
