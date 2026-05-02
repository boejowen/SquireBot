package picker

// Tests for Plan 01-06 Task 2: GET /picker + POST /picker/result.
//
// Strategy: each test wires a fresh *http.ServeMux + fake-Sheets
// httptest.NewServer + real *sheet.Client (re-uses Plan 05's fake-Sheets
// pattern) + a fake oauth2.TokenSource. Routes are exercised via
// httptest.NewRecorder + manually-constructed *http.Request through
// mux.ServeHTTP — no real network calls anywhere.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/option"

	"github.com/boejowen/SquireBot/internal/auth"
	"github.com/boejowen/SquireBot/internal/config"
	"github.com/boejowen/SquireBot/internal/sheet"
)

// errTokenSource always returns an error from Token(); used to drive Test 6.
type errTokenSource struct{ err error }

func (e errTokenSource) Token() (*oauth2.Token, error) { return nil, e.err }

// pickerStubHandler is the fake-Sheets handler for picker integration tests.
// Mirrors meta_test.go's fakeSheetsHandler but pared down to the routes
// ValidateWorkbook actually exercises (spreadsheets.get, values.get,
// values.update for bootstrap, batchUpdate for AddSheet).
type pickerStubHandler struct {
	t              *testing.T
	metaValues     [][]any
	sheetsList     []sheetInfoStub
	getCalls       int
	valuesGetCalls int
	valuesPutCalls int
	batchUpdates   int
}

type sheetInfoStub struct {
	Title   string
	SheetID int64
}

func (h *pickerStubHandler) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case r.Method == "GET" && strings.HasSuffix(path, "/spreadsheets/SHEET1"):
			h.getCalls++
			out := map[string]any{
				"spreadsheetId": "SHEET1",
				"sheets":        encodeSheetsListStub(h.sheetsList),
			}
			_ = json.NewEncoder(w).Encode(out)
		case r.Method == "GET" && strings.Contains(path, "/values/"):
			h.valuesGetCalls++
			body := map[string]any{"range": "_meta!A1:B2", "majorDimension": "ROWS"}
			if len(h.metaValues) > 0 {
				body["values"] = h.metaValues
			}
			_ = json.NewEncoder(w).Encode(body)
		case r.Method == "PUT" && strings.Contains(path, "/values/"):
			h.valuesPutCalls++
			_, _ = io.Copy(io.Discard, r.Body)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"updatedRange": "_meta!A1:B2",
				"updatedRows":  2,
				"updatedCells": 4,
			})
		case r.Method == "POST" && strings.HasSuffix(path, ":batchUpdate"):
			h.batchUpdates++
			_, _ = io.Copy(io.Discard, r.Body)
			_ = json.NewEncoder(w).Encode(map[string]any{"replies": []map[string]any{{}}})
		default:
			h.t.Logf("UNHANDLED: %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func encodeSheetsListStub(list []sheetInfoStub) []map[string]any {
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

// pickerFixture is the per-test wiring: fake Sheets, real sheet.Client,
// real picker.Server, mux. Use cleanup via t.Cleanup.
type pickerFixture struct {
	mux        *http.ServeMux
	server     *Server
	cfg        *config.Config
	sheetSrv   *httptest.Server
	sheetStub  *pickerStubHandler
	configPath string
}

func newPickerFixture(t *testing.T, ts oauth2.TokenSource, stub *pickerStubHandler) *pickerFixture {
	t.Helper()
	if stub == nil {
		stub = &pickerStubHandler{
			t:          t,
			sheetsList: []sheetInfoStub{{Title: "_meta", SheetID: 12345}},
			metaValues: [][]any{
				{"canonical_id", sheet.CanonicalID},
				{"schema_version", "1"},
			},
		}
	}
	srv := httptest.NewServer(stub.handler())
	t.Cleanup(srv.Close)

	// Real sheet.Client wired at the stub.
	sc, err := sheet.NewClient(context.Background(), oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "fake-sheets"}),
		"",
		option.WithEndpoint(srv.URL),
		option.WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatalf("sheet.NewClient: %v", err)
	}

	// Redirect config writes to a tmp dir so Save() doesn't pollute %LOCALAPPDATA%.
	tdir := t.TempDir()
	t.Setenv("LOCALAPPDATA", tdir)

	cfg := &config.Config{Version: 1, LogLevel: "info"}
	bc := auth.BuildConstants{
		OAuthClientID:     "client-id-stub",
		OAuthClientSecret: "GOCSPX-test-secret",
		PickerAPIKey:      "picker-key-stub",
		GCPProjectNumber:  "1234567890",
	}

	psrv := NewServer(sc, ts, cfg, bc)
	mux := http.NewServeMux()
	psrv.AttachRoutes(mux)

	return &pickerFixture{
		mux:       mux,
		server:    psrv,
		cfg:       cfg,
		sheetSrv:  srv,
		sheetStub: stub,
	}
}

// quietLogger silences slog during tests so error-path slog.Error calls
// don't pollute test output. Restored via t.Cleanup.
func quietLogger(t *testing.T) {
	t.Helper()
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
}

// ----- Test 1: GET /picker renders template with substituted values -----

func TestHandlePicker_RendersTemplateWithAccessToken(t *testing.T) {
	quietLogger(t)
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "fake-tok-123"})
	fx := newPickerFixture(t, ts, nil)

	req := httptest.NewRequest(http.MethodGet, "/picker", nil)
	rr := httptest.NewRecorder()
	fx.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%q", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/html; charset=utf-8", got)
	}
	if got := rr.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "fake-tok-123") {
		t.Errorf("body missing AccessToken substitution; got:\n%s", body)
	}
	if !strings.Contains(body, "1234567890") {
		t.Errorf("body missing AppID (GCPProjectNumber); got body len=%d", len(body))
	}
	if !strings.Contains(body, "picker-key-stub") {
		t.Errorf("body missing APIKey substitution")
	}
	// Template tokens MUST have been rendered (no raw {{...}} left behind).
	if strings.Contains(body, "{{.AccessToken}}") || strings.Contains(body, "{{.AppID}}") || strings.Contains(body, "{{.APIKey}}") {
		t.Errorf("template tokens not substituted; body still contains raw {{...}}")
	}
	// Sanity: the static page chrome that proves it's the embedded HTML.
	if !strings.Contains(body, "apis.google.com/js/api.js") {
		t.Error("body missing api.js script tag")
	}
	if !strings.Contains(body, "google.picker.PickerBuilder") {
		t.Error("body missing PickerBuilder")
	}
	if !strings.Contains(body, "application/vnd.google-apps.spreadsheet") {
		t.Error("body missing spreadsheet mime filter")
	}
	// Hotfix #3: Picker must show BOTH owned and shared spreadsheets via two
	// DocsView instances. setOwnedByMe(true) covers the user's own copies of
	// the SquireBot template (D-01: "Make a copy" path); setOwnedByMe(false)
	// covers workbooks shared with them. Without both views, owned-but-not-
	// recent files don't appear in the Picker.
	if !strings.Contains(body, "setOwnedByMe(true)") {
		t.Error("body missing setOwnedByMe(true) — owned-spreadsheets view absent")
	}
	if !strings.Contains(body, "setOwnedByMe(false)") {
		t.Error("body missing setOwnedByMe(false) — shared-spreadsheets view absent")
	}
	if !strings.Contains(body, "DocsView") {
		t.Error("body missing DocsView — must use DocsView (not bare ViewId.SPREADSHEETS) to filter views")
	}
}

// ----- Test 2: POST /picker/result happy path -----

func TestHandleResult_HappyPathPersistsAndRedirects(t *testing.T) {
	quietLogger(t)
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "fake-tok"})
	fx := newPickerFixture(t, ts, nil) // default stub returns healthy _meta

	body := bytes.NewBufferString(`{"spreadsheetId":"SHEET1","name":"Test Workbook"}`)
	req := httptest.NewRequest(http.MethodPost, "/picker/result", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	fx.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%q", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Location"); got != "/eq-folder" {
		t.Errorf("Location header = %q, want /eq-folder", got)
	}
	if fx.cfg.SpreadsheetID != "SHEET1" {
		t.Errorf("config.SpreadsheetID = %q, want SHEET1", fx.cfg.SpreadsheetID)
	}
}

// ----- Test 3: POST /picker/result wrong canonical_id → verbatim D-03 body -----

func TestHandleResult_WrongCanonicalReturnsVerbatimD03(t *testing.T) {
	quietLogger(t)
	stub := &pickerStubHandler{
		t:          t,
		sheetsList: []sheetInfoStub{{Title: "_meta", SheetID: 12345}},
		metaValues: [][]any{
			{"canonical_id", "definitely-not-squirebot"},
			{"schema_version", "1"},
		},
	}
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "fake-tok"})
	fx := newPickerFixture(t, ts, stub)

	body := bytes.NewBufferString(`{"spreadsheetId":"SHEET1","name":"Mystery Sheet"}`)
	req := httptest.NewRequest(http.MethodPost, "/picker/result", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	fx.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%q", rr.Code, rr.Body.String())
	}
	const verbatim = "This doesn't look like a SquireBot workbook. Pick the one shared by your guild leader."
	got := strings.TrimSpace(rr.Body.String())
	if !strings.Contains(got, verbatim) {
		t.Errorf("body missing verbatim D-03 message:\ngot:  %q\nwant: %q", got, verbatim)
	}
	// Errors.Is sanity: the sheet sentinel must round-trip via err.Error.
	if !errors.Is(sheet.ErrWrongWorkbook, sheet.ErrWrongWorkbook) {
		t.Fatal("test invariant: sheet.ErrWrongWorkbook should be itself")
	}
	// Config must NOT have been updated.
	if fx.cfg.SpreadsheetID != "" {
		t.Errorf("config.SpreadsheetID = %q on rejection; want empty", fx.cfg.SpreadsheetID)
	}
}

// ----- Test 4: POST /picker/result schema_version too new -----

func TestHandleResult_SchemaTooNewReturns400(t *testing.T) {
	quietLogger(t)
	stub := &pickerStubHandler{
		t:          t,
		sheetsList: []sheetInfoStub{{Title: "_meta", SheetID: 12345}},
		metaValues: [][]any{
			{"canonical_id", sheet.CanonicalID},
			{"schema_version", "99"},
		},
	}
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "fake-tok"})
	fx := newPickerFixture(t, ts, stub)

	body := bytes.NewBufferString(`{"spreadsheetId":"SHEET1","name":"Future Workbook"}`)
	req := httptest.NewRequest(http.MethodPost, "/picker/result", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	fx.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%q", rr.Code, rr.Body.String())
	}
	got := rr.Body.String()
	if !strings.Contains(got, "newer SquireBot schema") {
		t.Errorf("body missing 'newer SquireBot schema' phrase; got=%q", got)
	}
	if fx.cfg.SpreadsheetID != "" {
		t.Errorf("config.SpreadsheetID = %q on rejection; want empty", fx.cfg.SpreadsheetID)
	}
}

// ----- Test 5: POST /picker/result with malformed JSON -----

func TestHandleResult_MalformedJSONReturns400(t *testing.T) {
	quietLogger(t)
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "fake-tok"})
	fx := newPickerFixture(t, ts, nil)

	body := bytes.NewBufferString(`{not-json`)
	req := httptest.NewRequest(http.MethodPost, "/picker/result", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	fx.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%q", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "invalid JSON") {
		t.Errorf("body = %q, want 'invalid JSON'", rr.Body.String())
	}
}

// ----- Test 6: GET /picker when TokenSource fails -----

func TestHandlePicker_TokenSourceErrorReturns500(t *testing.T) {
	quietLogger(t)
	ts := errTokenSource{err: errors.New("simulated refresh failure with secret data 0xCAFEBABE")}
	fx := newPickerFixture(t, ts, nil)

	req := httptest.NewRequest(http.MethodGet, "/picker", nil)
	rr := httptest.NewRecorder()
	fx.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%q", rr.Code, rr.Body.String())
	}
	got := rr.Body.String()
	// Generic message to the user; the underlying error must NOT leak.
	if !strings.Contains(got, "OAuth token unavailable") {
		t.Errorf("body missing 'OAuth token unavailable' guidance; got=%q", got)
	}
	if strings.Contains(got, "0xCAFEBABE") {
		t.Errorf("body leaked underlying token error contents; got=%q", got)
	}
}

// ----- Bonus coverage: method enforcement -----

func TestHandlePicker_RejectsNonGET(t *testing.T) {
	quietLogger(t)
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "fake-tok"})
	fx := newPickerFixture(t, ts, nil)

	req := httptest.NewRequest(http.MethodPost, "/picker", nil)
	rr := httptest.NewRecorder()
	fx.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rr.Code)
	}
}

func TestHandleResult_RejectsNonPOST(t *testing.T) {
	quietLogger(t)
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "fake-tok"})
	fx := newPickerFixture(t, ts, nil)

	req := httptest.NewRequest(http.MethodGet, "/picker/result", nil)
	rr := httptest.NewRecorder()
	fx.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rr.Code)
	}
}

func TestHandleResult_EmptySpreadsheetIDReturns400(t *testing.T) {
	quietLogger(t)
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "fake-tok"})
	fx := newPickerFixture(t, ts, nil)

	body := bytes.NewBufferString(`{"spreadsheetId":"","name":"Empty"}`)
	req := httptest.NewRequest(http.MethodPost, "/picker/result", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	fx.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "spreadsheetId required") {
		t.Errorf("body = %q, want 'spreadsheetId required'", rr.Body.String())
	}
}

// ----- SetRedirectAfterPick override -----

func TestSetRedirectAfterPick_OverridesLocation(t *testing.T) {
	quietLogger(t)
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "fake-tok"})
	fx := newPickerFixture(t, ts, nil)
	fx.server.SetRedirectAfterPick("/wizard-step-3")

	body := bytes.NewBufferString(`{"spreadsheetId":"SHEET1","name":"X"}`)
	req := httptest.NewRequest(http.MethodPost, "/picker/result", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	fx.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%q", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Location"); got != "/wizard-step-3" {
		t.Errorf("Location = %q, want /wizard-step-3", got)
	}
}

// ----- OnPicked callback fires on success -----

func TestOnPicked_FiresOnSuccess(t *testing.T) {
	quietLogger(t)
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "fake-tok"})
	fx := newPickerFixture(t, ts, nil)
	fired := false
	fx.server.OnPicked(func() { fired = true })

	body := bytes.NewBufferString(`{"spreadsheetId":"SHEET1","name":"X"}`)
	req := httptest.NewRequest(http.MethodPost, "/picker/result", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	fx.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%q", rr.Code, rr.Body.String())
	}
	if !fired {
		t.Error("OnPicked callback did not fire")
	}
}
