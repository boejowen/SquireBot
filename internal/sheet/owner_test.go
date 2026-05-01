package sheet

// Tests for UpsertCharOwner (Plan 01-05 Task 3). Exercises:
//   1. Empty _char_owner → append (charName, ownerEmail, "", "", isoTime)
//   2. charName present + email matches → no-op (no append, no warn)
//   3. charName present + email mismatches → slog.Warn, NO append, returns nil
//   4. _char_owner tab missing → EnsureSheet creates it then appends

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

// ownerStub captures values.get + values.append + addSheet calls.
type ownerStub struct {
	t          *testing.T
	sheetsList []sheetInfo
	// What the values.get on _char_owner!A:B returns.
	rows [][]any

	getCalls       int
	appendCalls    int
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

		// values.append on _char_owner!A:E. The Sheets v4 client uses
		// path suffix ":append" for append.
		case r.Method == "POST" && strings.Contains(path, "/values/") && strings.Contains(path, ":append"):
			o.appendCalls++
			var vr sheets.ValueRange
			_ = json.NewDecoder(r.Body).Decode(&vr)
			o.appendedBodies = append(o.appendedBodies, &vr)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"spreadsheetId": "SHEET1",
				"updates": map[string]any{
					"updatedRange": "_char_owner!A2:E2",
					"updatedRows":  1,
					"updatedCells": 5,
				},
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

func TestUpsertCharOwner_AppendsOnFirstSighting(t *testing.T) {
	o := &ownerStub{
		t: t,
		sheetsList: []sheetInfo{
			{Title: "_char_owner", SheetID: 7},
		},
		rows: [][]any{
			{"char_name", "owner_email"}, // header only
		},
	}
	c, srv := newOwnerClient(t, o)
	defer srv.Close()

	before := time.Now().UTC()
	if err := c.UpsertCharOwner(context.Background(), "Foo", "alice@example.com"); err != nil {
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
	if len(cells) != 5 {
		t.Fatalf("appended cells = %d, want 5 (Phase 1 schema)", len(cells))
	}
	if cells[0] != "Foo" {
		t.Errorf("cell[0] = %v, want \"Foo\"", cells[0])
	}
	if cells[1] != "alice@example.com" {
		t.Errorf("cell[1] = %v, want \"alice@example.com\"", cells[1])
	}
	if cells[2] != "" {
		t.Errorf("cell[2] = %v, want empty (Phase 2 column)", cells[2])
	}
	if cells[3] != "" {
		t.Errorf("cell[3] = %v, want empty (Phase 2 column)", cells[3])
	}
	iso, _ := cells[4].(string)
	if iso == "" {
		t.Fatalf("cell[4] (first_seen) = %v, want RFC3339 string", cells[4])
	}
	parsed, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		t.Fatalf("first_seen %q not RFC3339: %v", iso, err)
	}
	// first_seen must be sandwiched between before and after (UTC).
	if parsed.Before(before.Truncate(time.Second)) || parsed.After(after.Add(time.Second)) {
		t.Errorf("first_seen %v outside [%v, %v]", parsed, before, after)
	}
}

func TestUpsertCharOwner_NoOpOnMatch(t *testing.T) {
	o := &ownerStub{
		t: t,
		sheetsList: []sheetInfo{{Title: "_char_owner", SheetID: 7}},
		rows: [][]any{
			{"char_name", "owner_email"},
			{"Foo", "alice@example.com"},
		},
	}
	c, srv := newOwnerClient(t, o)
	defer srv.Close()

	if err := c.UpsertCharOwner(context.Background(), "Foo", "alice@example.com"); err != nil {
		t.Fatalf("UpsertCharOwner: %v", err)
	}
	if o.appendCalls != 0 {
		t.Errorf("appendCalls = %d, want 0 (no-op on match)", o.appendCalls)
	}
}

func TestUpsertCharOwner_LogsAndReturnsNilOnMismatch(t *testing.T) {
	o := &ownerStub{
		t: t,
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

	err := c.UpsertCharOwner(context.Background(), "Foo", "bob@example.com")
	if err != nil {
		t.Fatalf("UpsertCharOwner: %v (mismatch must NOT return error in Phase 1)", err)
	}
	if o.appendCalls != 0 {
		t.Errorf("appendCalls = %d, want 0 (mismatch must NOT overwrite)", o.appendCalls)
	}

	logs := buf.String()
	if !strings.Contains(logs, "char_owner email mismatch") {
		t.Errorf("expected slog.Warn message in output:\n%s", logs)
	}
	// Verify level WARN.
	var rec map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(strings.SplitN(logs, "\n", 2)[0])), &rec); err == nil {
		if rec["level"] != "WARN" {
			t.Errorf("log level = %v, want WARN", rec["level"])
		}
		// Make sure the existing email and the current email both leaked
		// into the log so an officer reading the file later can audit.
		if !strings.Contains(logs, "alice@example.com") || !strings.Contains(logs, "bob@example.com") {
			t.Errorf("expected both emails in log:\n%s", logs)
		}
	}
}

// _char_owner tab missing on a fresh-from-template workbook → EnsureSheet
// creates it via AddSheet (one batchUpdate), then we append our row.
func TestUpsertCharOwner_CreatesTabIfMissing(t *testing.T) {
	o := &ownerStub{
		t:          t,
		sheetsList: []sheetInfo{}, // _char_owner missing
	}
	c, srv := newOwnerClient(t, o)
	defer srv.Close()

	if err := c.UpsertCharOwner(context.Background(), "Foo", "alice@example.com"); err != nil {
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

// Defensive: spreadsheetID empty → error.
func TestUpsertCharOwner_NoSpreadsheetID(t *testing.T) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "fake"})
	c, err := NewClient(ctx, ts, "")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := c.UpsertCharOwner(ctx, "Foo", "alice@example.com"); err == nil {
		t.Fatal("expected error when spreadsheetID empty, got nil")
	}
}
