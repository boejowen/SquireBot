package sheet

// Tests for UpsertCharOwner (Plan 02-01 Task 3 — extended from 5 to 13
// columns). Exercises:
//
//   1. Empty _char_owner → append 13-column row with locked defaults.
//   2. charName + email both match → refresh last_seen ONLY (no append,
//      no overwrite of class/level/first_seen).
//   3. charName present + email mismatch → slog.Warn, NO append, NO
//      last_seen touch, returns nil.
//   4. watcher_version column populated from caller-supplied param.
//   5. server column hard-coded to "blue" (P99 Blue is the only target).
//   6. _char_owner tab missing → EnsureSheet creates it then appends.
//   7. spreadsheetID empty → error.

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// ownerStub captures values.get + values.append + values.update + AddSheet.
type ownerStub struct {
	t          *testing.T
	sheetsList []sheetInfo
	// What the values.get on _char_owner!A:B returns.
	rows [][]any

	getCalls       int
	appendCalls    int
	updateCalls    int
	updatedRanges  []string
	updatedValues  [][]any
	appendedBodies []*sheets.ValueRange
	batchUpdates   []*sheets.BatchUpdateSpreadsheetRequest
}

func (o *ownerStub) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		// EnsureSheet's pre-flight Get on the spreadsheet.
		case r.Method == "GET" && strings.HasSuffix(path, "/spreadsheets/SHEET1"):
			o.getCalls++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"spreadsheetId": "SHEET1",
				"sheets":        encodeSheetsList(o.sheetsList),
			})

		// values.get on _char_owner!A:B.
		case r.Method == "GET" && strings.Contains(path, "/values/_char_owner"):
			body := map[string]any{"range": "_char_owner!A:B", "majorDimension": "ROWS"}
			if len(o.rows) > 0 {
				body["values"] = o.rows
			}
			_ = json.NewEncoder(w).Encode(body)

		// values.append on _char_owner!A:M.
		case r.Method == "POST" && strings.Contains(path, "/values/") && strings.Contains(path, ":append"):
			o.appendCalls++
			var vr sheets.ValueRange
			_ = json.NewDecoder(r.Body).Decode(&vr)
			o.appendedBodies = append(o.appendedBodies, &vr)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"spreadsheetId": "SHEET1",
				"updates": map[string]any{
					"updatedRange": "_char_owner!A2:M2",
					"updatedRows":  1,
					"updatedCells": 13,
				},
			})

		// values.update on _char_owner!K{row} (last_seen refresh).
		case r.Method == "PUT" && strings.Contains(path, "/values/_char_owner"):
			o.updateCalls++
			var vr sheets.ValueRange
			_ = json.NewDecoder(r.Body).Decode(&vr)
			// Decode the range from the URL.
			idx := strings.Index(path, "/values/")
			rng := path[idx+len("/values/"):]
			o.updatedRanges = append(o.updatedRanges, rng)
			if len(vr.Values) > 0 {
				o.updatedValues = append(o.updatedValues, vr.Values[0])
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"updatedRange": rng,
				"updatedRows":  1,
			})

		// AddSheet via batchUpdate.
		case r.Method == "POST" && strings.HasSuffix(path, ":batchUpdate"):
			var req sheets.BatchUpdateSpreadsheetRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			o.batchUpdates = append(o.batchUpdates, &req)
			replies := make([]map[string]any, 0, len(req.Requests))
			for _, rq := range req.Requests {
				if rq.AddSheet != nil {
					title := rq.AddSheet.Properties.Title
					newID := int64(90000 + len(o.sheetsList))
					o.sheetsList = append(o.sheetsList, sheetInfo{Title: title, SheetID: newID})
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
			o.t.Logf("UNHANDLED: %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func newOwnerClient(t *testing.T, o *ownerStub) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(o.handler())
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

// captureLogs swaps the default slog handler for one writing into buf
// for the duration of the returned cleanup func.
func captureLogs(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	prev := slog.Default()
	buf := &bytes.Buffer{}
	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	return buf, func() { slog.SetDefault(prev) }
}

// Test 1: empty _char_owner → append a 14-column row (Phase 4 added `race`).
func TestUpsertCharOwner_AppendsThirteenColumnsOnFirstSighting(t *testing.T) {
	o := &ownerStub{
		t: t,
		sheetsList: []sheetInfo{
			{Title: "_char_owner", SheetID: 7},
		},
		rows: [][]any{
			// header only — full 14-col header from scaffold
			{"char_name", "owner_email", "display_name", "discord_handle",
				"class", "level", "is_bank_toon", "is_hidden", "is_removed",
				"first_seen", "last_seen", "server", "watcher_version", "race"},
		},
	}
	c, srv := newOwnerClient(t, o)
	defer srv.Close()

	before := time.Now().UTC()
	if err := c.UpsertCharOwner(context.Background(), "Foo", "alice@example.com", "0.2.0"); err != nil {
		t.Fatalf("UpsertCharOwner: %v", err)
	}
	after := time.Now().UTC()

	if o.appendCalls != 1 {
		t.Fatalf("appendCalls = %d, want 1", o.appendCalls)
	}
	if len(o.appendedBodies) != 1 {
		t.Fatalf("captured bodies = %d, want 1", len(o.appendedBodies))
	}
	row := o.appendedBodies[0].Values
	if len(row) != 1 {
		t.Fatalf("Values rows = %d, want 1", len(row))
	}
	cells := row[0]
	if len(cells) != 14 {
		t.Fatalf("appended cells = %d, want 14 (v3 schema with race)", len(cells))
	}
	// Spot-check the load-bearing cells.
	checks := []struct {
		idx  int
		want any
		name string
	}{
		{0, "Foo", "char_name (A)"},
		{1, "alice@example.com", "owner_email (B)"},
		{2, "", "display_name (C)"},
		{3, "", "discord_handle (D)"},
		{4, "", "class (E)"},
		{5, "", "level (F)"},
		{6, "FALSE", "is_bank_toon (G)"},
		{7, "FALSE", "is_hidden (H)"},
		{8, "FALSE", "is_removed (I)"},
		{11, "blue", "server (L)"},
		{12, "0.2.0", "watcher_version (M)"},
		{13, "", "race (N)"},
	}
	for _, c := range checks {
		if cells[c.idx] != c.want {
			t.Errorf("%s = %v, want %v", c.name, cells[c.idx], c.want)
		}
	}
	// first_seen + last_seen must both be RFC3339 within the test window.
	for _, idx := range []int{9, 10} {
		iso, _ := cells[idx].(string)
		parsed, perr := time.Parse(time.RFC3339, iso)
		if perr != nil {
			t.Fatalf("cell[%d] = %q not RFC3339: %v", idx, iso, perr)
		}
		if parsed.Before(before.Truncate(time.Second)) || parsed.After(after.Add(time.Second)) {
			t.Errorf("cell[%d] %v outside [%v, %v]", idx, parsed, before, after)
		}
	}
}

// Test 2: charName + email both match → last_seen refresh ONLY.
func TestUpsertCharOwner_RefreshesLastSeenOnMatch(t *testing.T) {
	o := &ownerStub{
		t:          t,
		sheetsList: []sheetInfo{{Title: "_char_owner", SheetID: 7}},
		rows: [][]any{
			{"char_name", "owner_email"},                 // header (row 1)
			{"Foo", "alice@example.com", "", "", "Mage"}, // existing row 2
		},
	}
	c, srv := newOwnerClient(t, o)
	defer srv.Close()

	if err := c.UpsertCharOwner(context.Background(), "Foo", "alice@example.com", "0.2.0"); err != nil {
		t.Fatalf("UpsertCharOwner: %v", err)
	}
	if o.appendCalls != 0 {
		t.Errorf("appendCalls = %d, want 0 (match → refresh, not append)", o.appendCalls)
	}
	if o.updateCalls != 1 {
		t.Fatalf("updateCalls = %d, want 1 (last_seen refresh)", o.updateCalls)
	}
	if len(o.updatedRanges) != 1 {
		t.Fatalf("updatedRanges len = %d, want 1", len(o.updatedRanges))
	}
	// Row 2 → range _char_owner!K2.
	wantRange := "_char_owner!K2"
	if o.updatedRanges[0] != wantRange {
		t.Errorf("updatedRange = %q, want %q (column K = last_seen, row 2)", o.updatedRanges[0], wantRange)
	}
	// Single cell, RFC3339 string.
	if len(o.updatedValues[0]) != 1 {
		t.Errorf("updated cell count = %d, want 1", len(o.updatedValues[0]))
	}
	iso, _ := o.updatedValues[0][0].(string)
	if _, err := time.Parse(time.RFC3339, iso); err != nil {
		t.Errorf("last_seen value %q not RFC3339: %v", iso, err)
	}
}

// Test 3: charName + email mismatch → log, no overwrite, no last_seen touch.
func TestUpsertCharOwner_LogsAndReturnsNilOnMismatch(t *testing.T) {
	o := &ownerStub{
		t:          t,
		sheetsList: []sheetInfo{{Title: "_char_owner", SheetID: 7}},
		rows: [][]any{
			{"char_name", "owner_email"},
			{"Foo", "alice@example.com"},
		},
	}
	c, srv := newOwnerClient(t, o)
	defer srv.Close()

	buf, restore := captureLogs(t)
	defer restore()

	err := c.UpsertCharOwner(context.Background(), "Foo", "bob@example.com", "0.2.0")
	if err != nil {
		t.Fatalf("UpsertCharOwner: %v (mismatch must NOT return error)", err)
	}
	if o.appendCalls != 0 {
		t.Errorf("appendCalls = %d, want 0 (mismatch must NOT overwrite)", o.appendCalls)
	}
	if o.updateCalls != 0 {
		t.Errorf("updateCalls = %d, want 0 (mismatch must NOT touch last_seen)", o.updateCalls)
	}

	logs := buf.String()
	if !strings.Contains(logs, "char_owner email mismatch") {
		t.Errorf("expected slog.Warn message in output:\n%s", logs)
	}
	if !strings.Contains(logs, "alice@example.com") || !strings.Contains(logs, "bob@example.com") {
		t.Errorf("expected both emails in log:\n%s", logs)
	}
}

// Test 4 & 5 are subsumed by Test 1's spot-checks — server="blue" + watcher_version
// from the param. Spell out as a focused regression test for clarity.
func TestUpsertCharOwner_ServerHardCodedAndWatcherVersionPlumbed(t *testing.T) {
	o := &ownerStub{
		t:          t,
		sheetsList: []sheetInfo{{Title: "_char_owner", SheetID: 7}},
		rows:       [][]any{{"char_name", "owner_email"}},
	}
	c, srv := newOwnerClient(t, o)
	defer srv.Close()

	const wantVersion = "1.2.3-test"
	if err := c.UpsertCharOwner(context.Background(), "Bar", "joe@example.com", wantVersion); err != nil {
		t.Fatalf("UpsertCharOwner: %v", err)
	}
	if len(o.appendedBodies) != 1 {
		t.Fatalf("captured bodies = %d, want 1", len(o.appendedBodies))
	}
	cells := o.appendedBodies[0].Values[0]
	if cells[11] != "blue" {
		t.Errorf("server (col L) = %v, want \"blue\"", cells[11])
	}
	if cells[12] != wantVersion {
		t.Errorf("watcher_version (col M) = %v, want %q", cells[12], wantVersion)
	}
}

// Test 6: _char_owner tab missing → EnsureSheet creates it then appends.
func TestUpsertCharOwner_CreatesTabIfMissing(t *testing.T) {
	o := &ownerStub{
		t:          t,
		sheetsList: []sheetInfo{}, // _char_owner missing
	}
	c, srv := newOwnerClient(t, o)
	defer srv.Close()

	if err := c.UpsertCharOwner(context.Background(), "Foo", "alice@example.com", "0.2.0"); err != nil {
		t.Fatalf("UpsertCharOwner: %v", err)
	}
	if len(o.batchUpdates) != 1 {
		t.Fatalf("batchUpdates = %d, want 1 (AddSheet for _char_owner)", len(o.batchUpdates))
	}
	if o.batchUpdates[0].Requests[0].AddSheet == nil ||
		o.batchUpdates[0].Requests[0].AddSheet.Properties.Title != "_char_owner" {
		t.Errorf("expected AddSheet for _char_owner, got %+v", o.batchUpdates[0].Requests[0])
	}
	if o.appendCalls != 1 {
		t.Errorf("appendCalls = %d, want 1 (append after EnsureSheet)", o.appendCalls)
	}
}

// Test 7: spreadsheetID empty → error.
func TestUpsertCharOwner_NoSpreadsheetID(t *testing.T) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "fake"})
	c, err := NewClient(ctx, ts, "")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := c.UpsertCharOwner(ctx, "Foo", "alice@example.com", "0.2.0"); err == nil {
		t.Fatal("expected error when spreadsheetID empty, got nil")
	}
}
