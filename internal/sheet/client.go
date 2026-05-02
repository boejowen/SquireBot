// Package sheet wraps the Google Sheets v4 API for SquireBot's hot-path
// landing-tab writes (Plan 01-05). It exposes:
//
//   - Client + NewClient: thin wrapper around *sheets.Service holding the
//     active spreadsheet ID and a tab-name → sheetId cache.
//   - CanonicalID + WatcherMaxSchemaVersion: locked constants for the
//     Plan 06 picker callback's two-step workbook validation
//     (RESEARCH.md §2.6 + §12.3-12.4).
//   - InvTabMaxRows + InvTabColumns: locked GridRange bounds for the
//     atomic clear+write contract (RESEARCH.md §2.3).
//   - ErrWrongWorkbook + ErrSchemaTooNew: the two error sentinels
//     ValidateWorkbook returns; ErrWrongWorkbook carries the verbatim
//     D-03 rejection message (CONTEXT.md §Workbook Onboarding D-03).
//
// All sheet writes from this package use a single batchUpdate call with
// fields="userEnteredValue" and StringValue cells (Critical Constraint
// #3 + #8). See internal/sheet/write.go for the hot path.
package sheet

import (
	"context"
	"errors"
	"sync"

	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

const (
	// CanonicalID is baked into the binary AND into the master template's
	// _meta!B1 cell. RESEARCH.md §2.6 + §12.3, Open Question Q1 resolution,
	// CONTEXT.md D-03. Drift between this constant and the master template
	// breaks all bootstrap — pin it.
	CanonicalID = "squirebot-v1-workbook-2026"

	// WatcherMaxSchemaVersion is the highest schema this binary can write
	// against. RESEARCH.md §12.3 + Critical Constraint #5 (fail-fast on
	// forward-compat workbook).
	WatcherMaxSchemaVersion = 1

	// InvTabMaxRows is the GridRange.EndRowIndex (exclusive) for inv:<Char>
	// writes. P99 max ~250 rows; 500 gives 2x headroom and is comfortable
	// for atomic clearing per RESEARCH.md §2.3.
	InvTabMaxRows = 500

	// InvTabColumns = 6 (A:Location, B:Name, C:ID, D:Count, E:Slots,
	// F:_uploaded_at).
	InvTabColumns = 6

	// SpellTabMaxRows is the GridRange.EndRowIndex (exclusive) for spell:<Char>
	// writes. P99 max ~520 scribed spells for a level-65 paragon (Necro/Mage/
	// Wizard span the largest libraries); 600 gives headroom and matches the
	// atomic-clear contract semantics of InvTabMaxRows.
	SpellTabMaxRows = 600

	// SpellTabColumns = 3 (A:Level, B:Name, C:_uploaded_at). Schema-locked at
	// schema_version=1 per 02-CONTEXT.md §Schema Lock.
	SpellTabColumns = 3
)

// ErrWrongWorkbook is returned by ValidateWorkbook when the picked
// spreadsheet's _meta.canonical_id does not equal CanonicalID.
// The error string is the verbatim D-03 message (CONTEXT.md §Workbook
// Onboarding D-03) — do not edit it; downstream callers (Plan 06's
// pickerResult HTTP handler) return it as the HTTP 400 body verbatim.
var ErrWrongWorkbook = errors.New("This doesn't look like a SquireBot workbook. Pick the one shared by your guild leader.")

// ErrSchemaTooNew is returned by ValidateWorkbook when
// _meta.schema_version > WatcherMaxSchemaVersion. Critical Constraint #5:
// the watcher must refuse to write to a workbook whose schema is newer
// than what this binary understands. Plan 02's auto-updater (deferred)
// resolves this; Phase 1 just blocks safely.
var ErrSchemaTooNew = errors.New("This workbook uses a newer SquireBot schema. Update SquireBot to continue.")

// Client wraps a *sheets.Service plus the spreadsheet ID we're writing to.
// The tabs map caches title → sheetId lookups so EnsureSheet can avoid
// re-fetching the spreadsheet on every WriteInventory call.
//
// Concurrency: a single Client is safe for concurrent use BY DESIGN —
// batchMu serializes every Sheets API call. The heartbeat goroutine
// (Plan 02-05) and the watcher goroutine BOTH go through this mutex.
// Pitfall D (Plan 02-03): do NOT add code paths that bypass the helpers
// (batchUpdate, valuesGet, valuesAppend, valuesUpdate, spreadsheetsGet) —
// the bare svc.* calls do not acquire the mutex. Every API call this
// package issues MUST go through one of the helpers in
// internal/sheet/client_helpers.go.
type Client struct {
	svc           *sheets.Service
	spreadsheetID string
	tabs          map[string]int64

	// batchMu serializes ALL Sheets API calls from this Client. Pitfall D
	// (Plan 02-03): heartbeat goroutine + watcher goroutine share a Client;
	// this mutex prevents BatchUpdate ordering races. The mutex is held
	// for the duration of the API round-trip (including the WATCH-07 retry
	// schedule sleeps), so no two writes can interleave. The mutex is also
	// the right place to enforce backoff: a transient error → re-acquire
	// delay → retry, with the mutex preventing a second goroutine from
	// blowing through the same backoff slot.
	batchMu sync.Mutex

	// onRefresh is the callback withRetry invokes on a 403 with auth-flavored
	// reason. Set via SetOnRefresh after construction; nil means "no refresh
	// available," in which case the first such 403 still gets one retry
	// (via a no-op refresh) before propagating ErrPermanentAuth. Plan 02-04
	// will wire this to oauth2.TokenSource.Token() + re-StoreToken so the
	// retry actually swaps in a fresh access token.
	onRefresh func() error
}

// NewClient builds a Client from an oauth2.TokenSource (handed in by
// Plan 03's RunOAuth result). spreadsheetID may be "" — call
// SetSpreadsheetID after Plan 06's Picker validates one. Extra
// option.ClientOption values are appended after WithTokenSource so tests
// can inject WithEndpoint + WithHTTPClient pointing at an httptest stub
// (RESEARCH.md test strategy: no real GCP calls in CI).
func NewClient(ctx context.Context, ts oauth2.TokenSource, spreadsheetID string, opts ...option.ClientOption) (*Client, error) {
	all := append([]option.ClientOption{option.WithTokenSource(ts)}, opts...)
	svc, err := sheets.NewService(ctx, all...)
	if err != nil {
		return nil, err
	}
	return &Client{
		svc:           svc,
		spreadsheetID: spreadsheetID,
		tabs:          map[string]int64{},
	}, nil
}

// SetSpreadsheetID switches the active spreadsheet and clears the
// tab cache. Plan 06's "Change Workbook…" tray menu (CONTEXT.md D-04)
// will call this after re-running the Picker.
func (c *Client) SetSpreadsheetID(id string) {
	c.spreadsheetID = id
	c.tabs = map[string]int64{}
}

// SpreadsheetID returns the currently configured spreadsheet ID.
func (c *Client) SpreadsheetID() string { return c.spreadsheetID }

// SetOnRefresh installs the token-refresh callback. Plan 02-04 wires this
// to oauth2.TokenSource.Token() + wincred.StoreToken so withRetry can
// recover from a transient 403 by swapping in a fresh access token.
//
// Goroutine-safe semantics: callers MUST set this once at startup, before
// the watcher loop or heartbeat goroutine starts. After that point the
// field is read-only — no synchronization needed because every reader
// holds batchMu (which writers do not, by intent — SetOnRefresh is a
// startup-only init).
func (c *Client) SetOnRefresh(f func() error) { c.onRefresh = f }
