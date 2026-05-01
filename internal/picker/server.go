package picker

// Server is the Plan 01-06 Drive Picker HTTP surface. It exposes two
// routes attached to a caller-owned *http.ServeMux:
//
//   GET  /picker         — renders picker.html with AccessToken/AppID/APIKey
//                          substituted at request time. Content-Type:
//                          text/html; Cache-Control: no-store (T-06-01:
//                          AccessToken-bearing page must not be cached).
//   POST /picker/result  — JSON body {spreadsheetId, name}; calls
//                          sheet.Client.SetSpreadsheetID + ValidateWorkbook;
//                          on nil → persists to config and replies 204
//                          with Location: <redirectAfterPick>; on
//                          ErrWrongWorkbook / ErrSchemaTooNew → 400 with
//                          err.Error() (verbatim D-03 / newer-schema text);
//                          on other errors → 500.
//
// Server itself does not own a listener. Plan 07's wizard hands in the
// Plan-03 oauth.OAuthResult.Listener / mux pair via AttachRoutes(mux),
// so the user's single browser tab carries them OAuth → Picker →
// /eq-folder without a second os.Exec to launch the browser
// (INST-03 collapse to one tab).
//
// SECURITY (T-06-01, T-06-02):
//   - The AccessToken is short-lived (~1h) and bound to the user's
//     OAuth session. Embedding it in HTML served from 127.0.0.1 is the
//     documented Picker pattern (RESEARCH.md §5.4).
//   - We use html/template (not text/template) so the AccessToken is
//     HTML-escaped if it ever contained metacharacters — defense in
//     depth, even though Google's tokens are alphanumeric+dash+slash.
//   - No slog call references AccessToken. The token bytes leave this
//     package only via the rendered HTML response.
//   - Cache-Control: no-store on every /picker response.

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"

	"golang.org/x/oauth2"

	"github.com/jbowen-mn/squirebot/internal/auth"
	"github.com/jbowen-mn/squirebot/internal/config"
	"github.com/jbowen-mn/squirebot/internal/sheet"
)

// defaultRedirectAfterPick is the wizard step Plan 07 owns. The handler
// emits this as a Location header on /picker/result success; the JS in
// picker.html reads resp.headers.get('Location') and navigates the same
// tab to it.
const defaultRedirectAfterPick = "/eq-folder"

// Server holds the dependencies the picker routes need. Construct via
// NewServer; mount on a mux via AttachRoutes.
type Server struct {
	sheetClient       *sheet.Client
	tokenSource       oauth2.TokenSource
	cfg               *config.Config
	bc                auth.BuildConstants
	redirectAfterPick string
	onPicked          func()
	tmpl              *template.Template
}

// NewServer constructs a Server. All four arguments are required:
//   - sc: the *sheet.Client (Plan 05) that will hold the picked
//     spreadsheet ID and run ValidateWorkbook on it.
//   - ts: the oauth2.TokenSource issued by Plan 03 (RunOAuth result).
//     Server.handlePicker calls Token() on every GET to pick up
//     refresh-rotated access tokens (the source caches; cheap).
//   - cfg: the config that will receive the validated SpreadsheetID
//     before being saved to disk.
//   - bc: build-time OAuth/Picker constants. PickerAPIKey and
//     GCPProjectNumber are baked into the rendered HTML; OAuthClientID
//     is unused here (Plan 03 owns it).
//
// Panics only if the embedded picker.html template fails to parse —
// that's a build-time bug and will be caught by `go test`.
func NewServer(sc *sheet.Client, ts oauth2.TokenSource, cfg *config.Config, bc auth.BuildConstants) *Server {
	tmpl := template.Must(template.New("picker").Parse(pickerHTMLTemplate))
	return &Server{
		sheetClient:       sc,
		tokenSource:       ts,
		cfg:               cfg,
		bc:                bc,
		redirectAfterPick: defaultRedirectAfterPick,
		tmpl:              tmpl,
	}
}

// SetRedirectAfterPick overrides the default Location header sent on
// /picker/result success. Plan 07's wizard sets this to its step-3 URL
// when running in wizard mode.
func (s *Server) SetRedirectAfterPick(p string) {
	if p == "" {
		return
	}
	s.redirectAfterPick = p
}

// OnPicked registers a callback that fires after a successful pick AND
// successful config save. Plan 07 uses this to advance the wizard state
// without coupling to the HTTP response cycle.
func (s *Server) OnPicked(f func()) { s.onPicked = f }

// AttachRoutes registers /picker (GET) and /picker/result (POST) on the
// caller-owned mux. Plan 07 calls this on the same *http.ServeMux Plan
// 03 used for /oauth/callback so the browser stays on a single tab
// across OAuth → Picker → /eq-folder.
func (s *Server) AttachRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/picker", s.handlePicker)
	mux.HandleFunc("/picker/result", s.handleResult)
}

// handlePicker renders picker.html with AccessToken/AppID/APIKey
// substituted at request time. We fetch a fresh access token from the
// TokenSource on every GET so a refresh during the wizard's lifetime
// doesn't hand the page a stale token.
func (s *Server) handlePicker(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tok, err := s.tokenSource.Token()
	if err != nil {
		// IMPORTANT: do NOT include err in the HTTP response body — the
		// underlying oauth2 error can carry a refresh-token-failure
		// payload that leaks scope or other internal state. Log the
		// detail, return a generic message to the user.
		slog.Error("picker token fetch failed", "err", err)
		http.Error(w, "OAuth token unavailable. Please retry from the start.", http.StatusInternalServerError)
		return
	}

	data := struct {
		AccessToken string
		AppID       string
		APIKey      string
	}{
		AccessToken: tok.AccessToken,
		AppID:       s.bc.GCPProjectNumber,
		APIKey:      s.bc.PickerAPIKey,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// T-06-01: never cache a page that carries a bearer token, even on
	// a loopback origin where caching is unlikely.
	w.Header().Set("Cache-Control", "no-store")
	if err := s.tmpl.Execute(w, data); err != nil {
		// Header already sent; nothing to do but log.
		slog.Error("picker template render failed", "err", err)
	}
}

// pickerResultBody is the JSON shape the picker.html JS POSTs to
// /picker/result on a successful Picker action.
type pickerResultBody struct {
	SpreadsheetID string `json:"spreadsheetId"`
	Name          string `json:"name"`
}

// handleResult validates the picked spreadsheet and either persists the
// ID + redirects, or rejects with the verbatim D-03 message.
func (s *Server) handleResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body pickerResultBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if body.SpreadsheetID == "" {
		http.Error(w, "spreadsheetId required", http.StatusBadRequest)
		return
	}

	// Provisionally point the Sheets client at the picked workbook.
	s.sheetClient.SetSpreadsheetID(body.SpreadsheetID)

	if err := s.sheetClient.ValidateWorkbook(r.Context()); err != nil {
		// We log the rejection so an officer reading the log can see
		// "Bob picked workbook X and was rejected with reason Y." The
		// workbook NAME (not ID) is logged because:
		//   - the ID is moderately sensitive (grants drive.file access),
		//   - the name is human-meaningful in the log file.
		slog.Warn("picked workbook rejected",
			"err", err,
			"name", body.Name)

		// Reset the Sheets client's spreadsheetID so subsequent unrelated
		// calls don't accidentally land on a rejected workbook.
		s.sheetClient.SetSpreadsheetID("")

		switch {
		case errors.Is(err, sheet.ErrWrongWorkbook), errors.Is(err, sheet.ErrSchemaTooNew):
			// Plan 05 baked the verbatim D-03 / "newer schema" text into
			// the sentinel's Error() output. Surface it directly as the
			// HTTP 400 body so the JS in picker.html displays it
			// unchanged in the #status div.
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			http.Error(w, "Failed to validate workbook. Please try again.", http.StatusInternalServerError)
		}
		return
	}

	// Persist on success. T-01-03 (atomic config write) is handled by
	// config.Save itself.
	s.cfg.SpreadsheetID = body.SpreadsheetID
	if err := s.cfg.Save(); err != nil {
		// On save failure, undo the in-memory state so a retry starts
		// clean. Returning 500 here is correct: the client will surface
		// "Failed to save…" and the user can re-pick.
		s.sheetClient.SetSpreadsheetID("")
		s.cfg.SpreadsheetID = ""
		slog.Error("config save after picker", "err", err)
		http.Error(w, fmt.Sprintf("Failed to save workbook selection: %v", err), http.StatusInternalServerError)
		return
	}

	slog.Info("workbook picked", "name", body.Name)

	if s.onPicked != nil {
		s.onPicked()
	}

	// The JS in picker.html reads resp.headers.get('Location') and
	// navigates the same tab to it; we use 204 (no body) instead of a
	// 30x because fetch() in browsers automatically follows 30x for
	// same-origin and would re-fire the JS, but we want the client-side
	// navigation to be explicit.
	w.Header().Set("Location", s.redirectAfterPick)
	w.WriteHeader(http.StatusNoContent)
}
