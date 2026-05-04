package scaffold

// Tests for ScaffoldSchemaV1 (Plan 02-01 Task 2). httptest stub mirrors
// the pattern from internal/sheet/meta_test.go: a fakeSheetsHandler that
// captures every Sheets v4 call and lets each test assert against the
// recorded calls. No real GCP calls.
//
// Test matrix per <behavior>:
//
//   Test 1: empty workbook → all 13 expected scaffold tabs created
//           (9 hidden dimension + 4 visible view) plus the original
//           Sheet1 from the fake spreadsheet.
//   Test 2: header row written to each created tab, matching DimensionTabs
//           / ViewTabs definitions.
//   Test 3: 13 _meta KV rows appended with the locked schema_version=1
//           and canonical_id values.
//   Test 4: idempotent — second run performs zero AddSheet API requests.
//   Test 5: idempotent partial — pre-existing _meta + _char_owner are
//           NOT recreated; remaining 11 tabs ARE created; pre-existing
//           _meta rows NOT overwritten.
//   Test 6: hide-on-create — every _-prefixed tab has UpdateSheetProperties
//           with Hidden:true emitted.

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

	"github.com/boejowen/SquireBot/internal/sheet"
)

type sheetInfo struct {
	Title   string
	SheetID int64
	Hidden  bool
}

// fakeSheetsHandler records every Sheets API request the scaffold makes
// and serves canned responses. Because the scaffold issues many calls
// (one ListSheets, multiple AddSheet via EnsureSheet, multiple Update for
// headers, multiple BatchUpdate for hide, one ReadColumn on _meta!A:A,
// multiple Append for _meta rows), the handler dispatches on path +
// method.
type fakeSheetsHandler struct {
	t *testing.T

	// Mutable state — represents the workbook.
	sheets        []sheetInfo
	metaColumnA   []string             // values of _meta column A keys
	metaRows      [][]string           // appended rows (key, value)
	tabHeaders    map[string][]string  // first-row headers per tab
	hiddenChanges []int64              // sheetIds passed to UpdateSheetProperties{Hidden:true}

	// Counters.
	getCalls       int // Spreadsheets.Get (ListSheets / EnsureSheet)
	addSheetCalls  int // BatchUpdate AddSheetRequest count
	updateCalls    int // values.Update (header writes)
	appendCalls    int // values.Append (_meta row appends)
	hideUpdateReqs int // BatchUpdate UpdateSheetPropertiesRequest count
	metaGetCalls   int // values.Get on _meta!A:A
}

func newHandler(t *testing.T, initial []sheetInfo, initialMetaA []string) *fakeSheetsHandler {
	return &fakeSheetsHandler{
		t:           t,
		sheets:      initial,
		metaColumnA: initialMetaA,
		tabHeaders:  map[string][]string{},
	}
}

func (h *fakeSheetsHandler) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		switch {
		// Spreadsheets.Get — ListSheets (or EnsureSheet's pre-flight).
		case r.Method == "GET" && strings.HasSuffix(path, "/spreadsheets/SHEET1"):
			h.getCalls++
			out := map[string]any{
				"spreadsheetId": "SHEET1",
				"sheets":        encodeSheetsList(h.sheets),
			}
			_ = json.NewEncoder(w).Encode(out)

		// values.Get on _meta!A:A — ScaffoldSchemaV1's existing-keys probe.
		case r.Method == "GET" && strings.Contains(path, "/values/_meta"):
			h.metaGetCalls++
			body := map[string]any{
				"range":          "_meta!A:A",
				"majorDimension": "ROWS",
			}
			if len(h.metaColumnA) > 0 {
				values := make([][]any, 0, len(h.metaColumnA))
				for _, k := range h.metaColumnA {
					values = append(values, []any{k})
				}
				body["values"] = values
			}
			_ = json.NewEncoder(w).Encode(body)

		// values.Update — header row write (PUT method).
		case r.Method == "PUT" && strings.Contains(path, "/values/"):
			h.updateCalls++
			tab := extractTabFromValuesURL(path)
			var vr sheets.ValueRange
			_ = json.NewDecoder(r.Body).Decode(&vr)
			if len(vr.Values) > 0 {
				row := vr.Values[0]
				cells := make([]string, len(row))
				for i, v := range row {
					s, _ := v.(string)
					cells[i] = s
				}
				h.tabHeaders[tab] = cells
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"updatedRange": tab + "!A1",
				"updatedRows":  1,
			})

		// values.Append — _meta row append.
		case r.Method == "POST" && strings.Contains(path, "/values/") && strings.Contains(path, ":append"):
			h.appendCalls++
			var vr sheets.ValueRange
			_ = json.NewDecoder(r.Body).Decode(&vr)
			if len(vr.Values) > 0 {
				row := vr.Values[0]
				cells := make([]string, len(row))
				for i, v := range row {
					s, _ := v.(string)
					cells[i] = s
				}
				h.metaRows = append(h.metaRows, cells)
				if len(cells) > 0 {
					h.metaColumnA = append(h.metaColumnA, cells[0])
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"spreadsheetId": "SHEET1",
				"updates": map[string]any{
					"updatedRange": "_meta!A2:B2",
					"updatedRows":  1,
				},
			})

		// BatchUpdate — AddSheet or UpdateSheetProperties.
		case r.Method == "POST" && strings.HasSuffix(path, ":batchUpdate"):
			var req sheets.BatchUpdateSpreadsheetRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			replies := make([]map[string]any, 0, len(req.Requests))
			for _, rq := range req.Requests {
				if rq.AddSheet != nil {
					h.addSheetCalls++
					title := rq.AddSheet.Properties.Title
					newID := int64(70000 + len(h.sheets))
					h.sheets = append(h.sheets, sheetInfo{Title: title, SheetID: newID})
					replies = append(replies, map[string]any{
						"addSheet": map[string]any{
							"properties": map[string]any{
								"title":   title,
								"sheetId": newID,
							},
						},
					})
				} else if rq.UpdateSheetProperties != nil {
					h.hideUpdateReqs++
					if rq.UpdateSheetProperties.Properties != nil &&
						rq.UpdateSheetProperties.Properties.Hidden {
						h.hiddenChanges = append(h.hiddenChanges,
							rq.UpdateSheetProperties.Properties.SheetId)
					}
					replies = append(replies, map[string]any{})
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
				"hidden":  s.Hidden,
			},
		})
	}
	return out
}

// extractTabFromValuesURL pulls the tab name out of a path like
// `/v4/spreadsheets/SHEET1/values/_meta!A1` → `_meta`.
func extractTabFromValuesURL(path string) string {
	idx := strings.Index(path, "/values/")
	if idx < 0 {
		return ""
	}
	tail := path[idx+len("/values/"):]
	// Trim any trailing :append / :clear suffix.
	if i := strings.Index(tail, ":"); i >= 0 {
		tail = tail[:i]
	}
	// URL-decoded already (httptest uses default decoder). The values API
	// uses `tab!A1` form; tab name is everything before the first `!`.
	if i := strings.Index(tail, "!"); i >= 0 {
		return tail[:i]
	}
	return tail
}

func newTestClient(t *testing.T, h *fakeSheetsHandler) (*sheet.Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h.handler())
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "fake"})
	c, err := sheet.NewClient(ctx, ts, "SHEET1",
		option.WithEndpoint(srv.URL),
		option.WithHTTPClient(srv.Client()))
	if err != nil {
		srv.Close()
		t.Fatalf("NewClient: %v", err)
	}
	return c, srv
}

// expectedTabs returns the 13 tab names ScaffoldSchemaV1 must ensure exist.
func expectedTabs() []string {
	out := make([]string, 0, len(DimensionTabs)+len(ViewTabs))
	for _, d := range DimensionTabs {
		out = append(out, d.Name)
	}
	for _, v := range ViewTabs {
		out = append(out, v.Name)
	}
	return out
}

// Test 1: empty workbook → all 13 scaffold tabs created.
func TestScaffoldSchemaV1_EmptyWorkbookCreatesAllTabs(t *testing.T) {
	h := newHandler(t, []sheetInfo{{Title: "Sheet1", SheetID: 0}}, nil)
	c, srv := newTestClient(t, h)
	defer srv.Close()

	if err := ScaffoldSchemaV1(context.Background(), c); err != nil {
		t.Fatalf("ScaffoldSchemaV1: %v", err)
	}

	titles := map[string]bool{}
	for _, s := range h.sheets {
		titles[s.Title] = true
	}
	for _, want := range expectedTabs() {
		if !titles[want] {
			t.Errorf("expected tab %q to be created; final titles: %v", want, titles)
		}
	}
	// Sheet1 must still be present.
	if !titles["Sheet1"] {
		t.Errorf("original Sheet1 missing from final state")
	}
	// addSheetCalls = 13 (9 dimension + 4 view).
	if h.addSheetCalls != len(DimensionTabs)+len(ViewTabs) {
		t.Errorf("addSheetCalls = %d, want %d", h.addSheetCalls, len(DimensionTabs)+len(ViewTabs))
	}
}

// Test 2: header row written to each tab matches DimensionTabs/ViewTabs.
func TestScaffoldSchemaV1_HeaderRowsMatchLockedSchema(t *testing.T) {
	h := newHandler(t, []sheetInfo{{Title: "Sheet1", SheetID: 0}}, nil)
	c, srv := newTestClient(t, h)
	defer srv.Close()

	if err := ScaffoldSchemaV1(context.Background(), c); err != nil {
		t.Fatalf("ScaffoldSchemaV1: %v", err)
	}

	for _, dt := range DimensionTabs {
		got := h.tabHeaders[dt.Name]
		if !equalStrings(got, dt.Headers) {
			t.Errorf("header for %s = %v, want %v", dt.Name, got, dt.Headers)
		}
	}
	for _, vt := range ViewTabs {
		got := h.tabHeaders[vt.Name]
		if !equalStrings(got, vt.Headers) {
			t.Errorf("header for %s = %v, want %v", vt.Name, got, vt.Headers)
		}
	}
	// Specifically: _char_owner has 13 columns including discord_handle,
	// is_hidden, is_removed, watcher_version (load-bearing for SCHEMA-05).
	co := h.tabHeaders["_char_owner"]
	if len(co) != 13 {
		t.Fatalf("_char_owner header len = %d, want 13", len(co))
	}
	wantCols := []string{"discord_handle", "is_hidden", "is_removed", "watcher_version"}
	for _, w := range wantCols {
		found := false
		for _, c := range co {
			if c == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("_char_owner missing column %q", w)
		}
	}
}

// Test 3: 13 _meta KV rows appended with locked values.
func TestScaffoldSchemaV1_MetaRowsAppendedWithLockedValues(t *testing.T) {
	h := newHandler(t, []sheetInfo{{Title: "Sheet1", SheetID: 0}}, nil)
	c, srv := newTestClient(t, h)
	defer srv.Close()

	if err := ScaffoldSchemaV1(context.Background(), c); err != nil {
		t.Fatalf("ScaffoldSchemaV1: %v", err)
	}

	if len(h.metaRows) != len(MetaRows) {
		t.Errorf("metaRows count = %d, want %d", len(h.metaRows), len(MetaRows))
	}
	// Spot-check load-bearing values: schema_version=1 + canonical_id.
	var sawSchemaVersion, sawCanonicalID bool
	for _, row := range h.metaRows {
		if len(row) < 2 {
			continue
		}
		switch row[0] {
		case "schema_version":
			sawSchemaVersion = true
			if row[1] != "1" {
				t.Errorf("schema_version value = %q, want \"1\"", row[1])
			}
		case "canonical_id":
			sawCanonicalID = true
			if row[1] != sheet.CanonicalID {
				t.Errorf("canonical_id value = %q, want %q", row[1], sheet.CanonicalID)
			}
		}
	}
	if !sawSchemaVersion {
		t.Errorf("schema_version row missing")
	}
	if !sawCanonicalID {
		t.Errorf("canonical_id row missing")
	}
}

// Test 4: second run is a no-op — zero AddSheet calls + zero appends.
func TestScaffoldSchemaV1_IdempotentSecondRun(t *testing.T) {
	// Pre-populate with all expected tabs + all _meta keys.
	tabs := []sheetInfo{{Title: "Sheet1", SheetID: 0}}
	id := int64(100)
	for _, dt := range DimensionTabs {
		tabs = append(tabs, sheetInfo{Title: dt.Name, SheetID: id, Hidden: true})
		id++
	}
	for _, vt := range ViewTabs {
		tabs = append(tabs, sheetInfo{Title: vt.Name, SheetID: id})
		id++
	}
	keys := make([]string, 0, len(MetaRows))
	for _, m := range MetaRows {
		keys = append(keys, m[0])
	}
	h := newHandler(t, tabs, keys)
	c, srv := newTestClient(t, h)
	defer srv.Close()

	if err := ScaffoldSchemaV1(context.Background(), c); err != nil {
		t.Fatalf("ScaffoldSchemaV1: %v", err)
	}

	if h.addSheetCalls != 0 {
		t.Errorf("addSheetCalls = %d, want 0 (idempotent)", h.addSheetCalls)
	}
	if h.updateCalls != 0 {
		t.Errorf("updateCalls = %d, want 0 (no header overwrites)", h.updateCalls)
	}
	if h.appendCalls != 0 {
		t.Errorf("appendCalls = %d, want 0 (all _meta keys present)", h.appendCalls)
	}
	if h.hideUpdateReqs != 0 {
		t.Errorf("hideUpdateReqs = %d, want 0 (already hidden)", h.hideUpdateReqs)
	}
}

// Test 5: partial scaffold — _meta + _char_owner pre-exist, others created.
// Pre-existing _meta rows are NOT overwritten.
func TestScaffoldSchemaV1_PartialScaffoldNoOverwrite(t *testing.T) {
	tabs := []sheetInfo{
		{Title: "Sheet1", SheetID: 0},
		{Title: "_meta", SheetID: 100, Hidden: true},
		{Title: "_char_owner", SheetID: 101, Hidden: true},
	}
	// _meta already has all 13 keys.
	keys := make([]string, 0, len(MetaRows))
	for _, m := range MetaRows {
		keys = append(keys, m[0])
	}
	h := newHandler(t, tabs, keys)
	c, srv := newTestClient(t, h)
	defer srv.Close()

	if err := ScaffoldSchemaV1(context.Background(), c); err != nil {
		t.Fatalf("ScaffoldSchemaV1: %v", err)
	}

	// Should have created the remaining 11 tabs (9 dim + 4 view - 2 already
	// present = 11).
	wantAdds := len(DimensionTabs) + len(ViewTabs) - 2
	if h.addSheetCalls != wantAdds {
		t.Errorf("addSheetCalls = %d, want %d", h.addSheetCalls, wantAdds)
	}
	// _meta + _char_owner headers must NOT be re-written.
	if _, wrote := h.tabHeaders["_meta"]; wrote {
		t.Errorf("_meta header was overwritten on partial-scaffold run")
	}
	if _, wrote := h.tabHeaders["_char_owner"]; wrote {
		t.Errorf("_char_owner header was overwritten on partial-scaffold run")
	}
	// No _meta rows appended (all keys present).
	if h.appendCalls != 0 {
		t.Errorf("appendCalls = %d, want 0 (all keys already present)", h.appendCalls)
	}
}

// Test 6: every dimension tab created has Hidden:true emitted via
// UpdateSheetPropertiesRequest.
func TestScaffoldSchemaV1_HidesDimensionTabsOnCreate(t *testing.T) {
	h := newHandler(t, []sheetInfo{{Title: "Sheet1", SheetID: 0}}, nil)
	c, srv := newTestClient(t, h)
	defer srv.Close()

	if err := ScaffoldSchemaV1(context.Background(), c); err != nil {
		t.Fatalf("ScaffoldSchemaV1: %v", err)
	}

	if h.hideUpdateReqs != len(DimensionTabs) {
		t.Errorf("hideUpdateReqs = %d, want %d (one per dimension tab)",
			h.hideUpdateReqs, len(DimensionTabs))
	}
	if len(h.hiddenChanges) != len(DimensionTabs) {
		t.Errorf("hiddenChanges = %d, want %d", len(h.hiddenChanges), len(DimensionTabs))
	}
	// Each hidden sheetId must correspond to a tab whose name is
	// underscore-prefixed.
	idToTitle := map[int64]string{}
	for _, s := range h.sheets {
		idToTitle[s.SheetID] = s.Title
	}
	for _, sid := range h.hiddenChanges {
		title := idToTitle[sid]
		if !strings.HasPrefix(title, "_") {
			t.Errorf("hidden sheetId %d corresponds to tab %q (not _-prefixed)", sid, title)
		}
	}
}

// Test 7: regression — pre-existing dimension tabs that are visible
// (e.g., _meta created by ValidateWorkbook's EnsureSheet side-effect,
// or _status created by heartbeat's EnsureSheet before scaffold ran)
// must be hidden by scaffold rather than left visible. Day-0 finding
// from soak validation 2026-05-02.
func TestScaffoldSchemaV1_HidesPreExistingVisibleDimensionTabs(t *testing.T) {
	// Pre-populate the workbook with two dimension tabs in the visible
	// state (Hidden:false) — mirroring the Day-0 production state where
	// _meta and _status existed but were not hidden.
	tabs := []sheetInfo{
		{Title: "Sheet1", SheetID: 0},
		{Title: "_meta", SheetID: 100, Hidden: false},
		{Title: "_status", SheetID: 101, Hidden: false},
	}
	h := newHandler(t, tabs, nil)
	c, srv := newTestClient(t, h)
	defer srv.Close()

	if err := ScaffoldSchemaV1(context.Background(), c); err != nil {
		t.Fatalf("ScaffoldSchemaV1: %v", err)
	}

	// Both pre-existing visible dimension tabs MUST appear in
	// hiddenChanges (scaffold must have called HideSheet for each).
	preExistingHidden := map[int64]bool{}
	for _, sid := range h.hiddenChanges {
		preExistingHidden[sid] = true
	}
	if !preExistingHidden[100] {
		t.Errorf("pre-existing visible _meta (sheetId=100) was NOT hidden by scaffold; hiddenChanges=%v", h.hiddenChanges)
	}
	if !preExistingHidden[101] {
		t.Errorf("pre-existing visible _status (sheetId=101) was NOT hidden by scaffold; hiddenChanges=%v", h.hiddenChanges)
	}

	// Pre-existing tabs must NOT have their headers re-written (the user's
	// existing data, if any, is preserved per the partial-scaffold contract).
	if _, wrote := h.tabHeaders["_meta"]; wrote {
		t.Errorf("_meta header was overwritten — pre-existing tab headers must be preserved")
	}
	if _, wrote := h.tabHeaders["_status"]; wrote {
		t.Errorf("_status header was overwritten — pre-existing tab headers must be preserved")
	}

	// addSheetCalls = 11 (7 missing dimension tabs + 4 view tabs).
	wantAdds := len(DimensionTabs) + len(ViewTabs) - 2
	if h.addSheetCalls != wantAdds {
		t.Errorf("addSheetCalls = %d, want %d (skip 2 pre-existing dim tabs)", h.addSheetCalls, wantAdds)
	}

	// Total hideUpdateReqs = 7 newly-created dimension tabs + 2
	// pre-existing visible ones = 9 = len(DimensionTabs).
	wantHides := len(DimensionTabs)
	if h.hideUpdateReqs != wantHides {
		t.Errorf("hideUpdateReqs = %d, want %d (7 new + 2 pre-existing visible)", h.hideUpdateReqs, wantHides)
	}
}

// Test 8: regression companion — pre-existing dimension tabs that are
// ALREADY hidden must NOT trigger a redundant HideSheet call (avoids
// burning an API call per scaffold run on a healthy workbook).
func TestScaffoldSchemaV1_DoesNotRehidePreExistingHiddenDimensionTabs(t *testing.T) {
	tabs := []sheetInfo{
		{Title: "Sheet1", SheetID: 0},
		{Title: "_meta", SheetID: 100, Hidden: true},   // already hidden
		{Title: "_status", SheetID: 101, Hidden: true}, // already hidden
	}
	h := newHandler(t, tabs, nil)
	c, srv := newTestClient(t, h)
	defer srv.Close()

	if err := ScaffoldSchemaV1(context.Background(), c); err != nil {
		t.Fatalf("ScaffoldSchemaV1: %v", err)
	}

	// _meta and _status should NOT appear in hiddenChanges (no redundant hides).
	for _, sid := range h.hiddenChanges {
		if sid == 100 {
			t.Errorf("pre-existing already-hidden _meta was redundantly re-hidden")
		}
		if sid == 101 {
			t.Errorf("pre-existing already-hidden _status was redundantly re-hidden")
		}
	}

	// Total hideUpdateReqs = 7 (only the newly-created dimension tabs).
	wantHides := len(DimensionTabs) - 2
	if h.hideUpdateReqs != wantHides {
		t.Errorf("hideUpdateReqs = %d, want %d (7 new only; 2 pre-existing skipped)", h.hideUpdateReqs, wantHides)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
