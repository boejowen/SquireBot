---
phase: 01-end-to-end-thin-slice
plan: 05
type: execute
wave: 3
depends_on: [01, 03]
files_modified:
  - internal/sheet/client.go
  - internal/sheet/ensure_tab.go
  - internal/sheet/meta.go
  - internal/sheet/meta_test.go
  - internal/sheet/owner.go
  - internal/sheet/write.go
  - internal/sheet/write_test.go
autonomous: true
requirements: [OPS-01, AUTH-06]
must_haves:
  truths:
    - "ValidateWorkbook reads `_meta!B1` (canonical_id) + `_meta!B2` (schema_version). canonical_id MUST equal the baked constant `squirebot-v1-workbook-2026`. schema_version MUST be ≤ WatcherMaxSchemaVersion (=1)."
    - "If `_meta` cells are empty (fresh-from-template workbook), the BOOTSTRAP path writes canonical_id + schema_version=1 in a single batchUpdate"
    - "If canonical_id is set but mismatches, ValidateWorkbook returns the verbatim D-03 rejection error: `This doesn't look like a SquireBot workbook. Pick the one shared by your guild leader.`"
    - "If schema_version > 1, ValidateWorkbook returns an explicit `update SquireBot` error and the watcher REFUSES to write further (Critical Constraint #5 fail-fast)"
    - "WriteInventory issues exactly ONE spreadsheets.batchUpdate API call containing one UpdateCellsRequest with `range = inv:<Char>!A1:F500`, `rows = header + data`, `fields = userEnteredValue` — atomic clear+write per RESEARCH.md §2.3"
    - "WriteInventory writes string values (UserEnteredValue.StringValue) for every cell — never numberValue — so that valueInputOption=RAW semantics apply and no recalc storm fires"
    - "UpsertCharOwner appends `(char_name, owner_email, '', '', first_seen_iso)` if char absent; on existing-row + email mismatch, logs slog.Warn and returns nil (Phase 2 owns the audit-tab surface)"
  artifacts:
    - path: "internal/sheet/client.go"
      provides: "Client wrapping *sheets.Service + spreadsheetID; constructed from an oauth2.TokenSource handed in by Plan 03's RunOAuth result"
      contains: "sheets.NewService"
    - path: "internal/sheet/meta.go"
      provides: "ValidateWorkbook + bootstrapMeta + readMeta — the canonical_id/schema_version handshake per RESEARCH.md §12.3-12.4"
      contains: "squirebot-v1-workbook-2026"
    - path: "internal/sheet/write.go"
      provides: "WriteInventory atomic clear+write for inv:<Char> tabs per RESEARCH.md §2.3"
      contains: "UpdateCellsRequest"
    - path: "internal/sheet/owner.go"
      provides: "UpsertCharOwner per RESEARCH.md §12.5; mismatch policy = log + proceed (Phase 1)"
      contains: "_char_owner"
    - path: "internal/sheet/ensure_tab.go"
      provides: "EnsureSheet creates a tab via addSheet if missing; returns the numeric sheetId for stable A1-free range references"
      contains: "AddSheet"
  key_links:
    - from: "internal/sheet/write.go"
      to: "google.golang.org/api/sheets/v4"
      via: "BatchUpdate(spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{Requests: [{UpdateCells:...}]})"
      pattern: "UpdateCellsRequest|BatchUpdate"
    - from: "internal/sheet/write.go"
      to: "internal/sheet/ensure_tab.go"
      via: "WriteInventory calls EnsureSheet on first sighting of a character"
      pattern: "EnsureSheet"
    - from: "internal/sheet/meta.go"
      to: "internal/sheet/client.go"
      via: "Client.ValidateWorkbook is the public entry from Plan 06 picker callback"
      pattern: "ValidateWorkbook"
    - from: "internal/sheet/owner.go"
      to: "internal/sheet/write.go"
      via: "Both share the same Client and execute against the same spreadsheetID; UpsertCharOwner runs immediately after WriteInventory in the Plan 07 wiring"
      pattern: "Client"
---

<objective>
Build the entire Google Sheets write surface for Phase 1: the sheet client constructor, the
canonical_id + schema_version handshake (D-03 + Critical Constraint #5), the atomic single-call
clear+write for `inv:<Char>` tabs (RESEARCH.md §2.3 + Critical Constraint #3), and the
`_char_owner` upsert (AUTH-06 sheet side).

Purpose: Every Phase 1 success criterion #3 depends on this plan being correct. RESEARCH.md
§2.3 lines 298-365 contain the EXACT atomic clear+write contract that all later phases will use
unchanged. Critical Constraint #3 demands a single batchUpdate call (NOT clear-then-write) — this
is the locked-in pattern that scales when consolidated views land in Phase 3+. OPS-01's
"per-character non-overlapping ranges" rule is enforced by always-keying writes on `inv:<Char>`
tab name + the fixed `A1:F500` GridRange — no shared mutable ranges from any watcher.

Output: An `internal/sheet/` package providing `Client`, `ValidateWorkbook`, `WriteInventory`,
`UpsertCharOwner`. Tests use a fake Sheets HTTP server (httptest.NewServer with `option.WithEndpoint`)
to verify the request shape — no real GCP calls in CI.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/phases/01-end-to-end-thin-slice/01-CONTEXT.md
@.planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md
@.planning/research/ARCHITECTURE.md
@./CLAUDE.md
@internal/auth/oauth.go
@internal/config/config.go
</context>

<interfaces>
<!-- Contracts this plan exports for downstream plans (06, 07). -->

From internal/sheet/client.go:
```go
package sheet

const (
    // CanonicalID is baked into the binary AND into the master template's _meta!B1 cell.
    // RESEARCH.md §2.6 + §12.3 — Open Question Q1 resolution.
    CanonicalID = "squirebot-v1-workbook-2026"

    // WatcherMaxSchemaVersion is the highest schema this binary can write against.
    // RESEARCH.md §12.3 + Critical Constraint #5.
    WatcherMaxSchemaVersion = 1

    // InvTabMaxRows is the GridRange.EndRowIndex for inv:<Char> writes.
    // P99 max ~250 rows; 500 gives 2x headroom and is comfortable for atomic clearing.
    InvTabMaxRows = 500

    // InvTabColumns = 6 (A:Location, B:Name, C:ID, D:Count, E:Slots, F:_uploaded_at).
    InvTabColumns = 6
)

// Client wraps the Sheets service and the active spreadsheet ID. Constructed via NewClient.
type Client struct {
    svc           *sheets.Service
    spreadsheetID string
    // tabs caches name -> sheetId (fetched once per client lifetime; refreshed on EnsureSheet).
    tabs map[string]int64
}

// NewClient builds a Client from an oauth2.TokenSource (from Plan 03's RunOAuth result).
// spreadsheetID may be "" — call SetSpreadsheetID after Plan 06 picker validates one.
func NewClient(ctx context.Context, ts oauth2.TokenSource, spreadsheetID string) (*Client, error)

func (c *Client) SetSpreadsheetID(id string) { c.spreadsheetID = id; c.tabs = nil }
func (c *Client) SpreadsheetID() string      { return c.spreadsheetID }
```

From internal/sheet/meta.go:
```go
package sheet

// ValidateWorkbook implements the Plan 06 picker callback's validation step.
// Behavior per RESEARCH.md §2.6 + §12.3-12.4:
//   - reads _meta!A1:B2 (cells: canonical_id key/value, schema_version key/value)
//   - if cells missing/empty → BOOTSTRAP path writes canonical_id + schema_version=1
//   - if canonical_id == "squirebot-v1-workbook-2026" → HEALTHY, check schema_version ≤ WatcherMaxSchemaVersion
//   - else → return ErrWrongWorkbook (verbatim D-03 message)
//
// On schema_version > WatcherMaxSchemaVersion → ErrSchemaTooNew with explicit "update SquireBot" message.
func (c *Client) ValidateWorkbook(ctx context.Context) error

var ErrWrongWorkbook = errors.New("This doesn't look like a SquireBot workbook. Pick the one shared by your guild leader.")
var ErrSchemaTooNew  = errors.New("This workbook uses a newer SquireBot schema. Update SquireBot to continue.")
```

From internal/sheet/write.go:
```go
package sheet

// WriteInventory replaces the entire inv:<charName> tab atomically.
// Single batchUpdate call: clear `inv:<charName>!A1:F500` + write header + data rows.
// Per RESEARCH.md §2.3 Pattern 1, OPS-01 (per-character non-overlapping ranges).
//
// header is the fixed 6-element header row [Location, Name, ID, Count, Slots, _uploaded_at].
// dataRows is the parsed 5-column rows from parse.Parse — WriteInventory appends `uploadedAt` to each.
// All cells are written as StringValue to avoid USER_ENTERED-style recalc (Critical Constraint #8 enforcement).
func (c *Client) WriteInventory(ctx context.Context, charName string, header []string, dataRows [][]string, uploadedAt string) error

// InventoryHeader is the fixed 6-element header row Plan 07 passes to WriteInventory.
var InventoryHeader = []string{"Location", "Name", "ID", "Count", "Slots", "_uploaded_at"}
```

From internal/sheet/owner.go:
```go
package sheet

// UpsertCharOwner appends a row to _char_owner if charName is absent.
// On existing-row + email match: no-op.
// On existing-row + email mismatch: slog.Warn and return nil (Phase 2 surfaces in _audit; D-03 silent).
// Per RESEARCH.md §12.5 + AUTH-06.
func (c *Client) UpsertCharOwner(ctx context.Context, charName, ownerEmail string) error
```

From internal/sheet/ensure_tab.go:
```go
package sheet

// EnsureSheet returns the numeric sheetId of `name`, creating the tab if missing.
// Caches results in c.tabs. Idempotent — safe to call before every write.
func (c *Client) EnsureSheet(ctx context.Context, name string) (int64, error)
```
</interfaces>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Sheets client + EnsureSheet + ValidateWorkbook with canonical_id handshake</name>
  <files>internal/sheet/client.go, internal/sheet/ensure_tab.go, internal/sheet/meta.go, internal/sheet/meta_test.go</files>
  <read_first>
    - .planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md (§2.6 Pattern 4 Two-Step Workbook Validation lines 419-431; §12 entire — §12.3 BOOTSTRAP path lines 1217-1260, §12.4 HEALTHY path lines 1259-1262, §12.6 schema_version overflow lines 1295-1300)
    - .planning/phases/01-end-to-end-thin-slice/01-CONTEXT.md (D-03 verbatim rejection text; D-04 "Change Workbook…" tray menu — Phase 1 minimum)
    - .planning/research/ARCHITECTURE.md (Sheet Schema §_meta — describes the row layout; ARCHITECTURE.md spec is canonical for Phase 1)
    - ./CLAUDE.md (Schema is extend-only; WATCHER_MAX_SCHEMA_VERSION check)
    - .planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md (Open Question Q1 lines 1517-1520 — manual master template + watcher bootstrap fallback)
  </read_first>
  <behavior>
    - Test 1: ValidateWorkbook against a fake server returning empty `_meta!A1:B2` (canonical_id cell empty) writes a batchUpdate setting `_meta!B1=squirebot-v1-workbook-2026` and `_meta!B2=1` (BOOTSTRAP path).
    - Test 2: ValidateWorkbook against a fake server returning `_meta!B1=squirebot-v1-workbook-2026, _meta!B2=1` returns nil (HEALTHY path).
    - Test 3: ValidateWorkbook against a fake server returning `_meta!B1=different-canonical-id` returns ErrWrongWorkbook with the verbatim D-03 message.
    - Test 4: ValidateWorkbook against a fake server returning `_meta!B1=squirebot-v1-workbook-2026, _meta!B2=2` returns ErrSchemaTooNew (Critical Constraint #5).
    - Test 5: EnsureSheet for an existing tab returns the cached/fetched sheetId without issuing AddSheet.
    - Test 6: EnsureSheet for a missing tab issues a batchUpdate with AddSheetRequest and caches the new sheetId.
  </behavior>
  <action>
    Create `internal/sheet/client.go`:
    ```go
    package sheet

    import (
        "context"
        "errors"

        "golang.org/x/oauth2"
        "google.golang.org/api/option"
        "google.golang.org/api/sheets/v4"
    )

    const (
        CanonicalID             = "squirebot-v1-workbook-2026"
        WatcherMaxSchemaVersion = 1
        InvTabMaxRows           = 500
        InvTabColumns           = 6
    )

    type Client struct {
        svc           *sheets.Service
        spreadsheetID string
        tabs          map[string]int64
    }

    var (
        ErrWrongWorkbook = errors.New("This doesn't look like a SquireBot workbook. Pick the one shared by your guild leader.")
        ErrSchemaTooNew  = errors.New("This workbook uses a newer SquireBot schema. Update SquireBot to continue.")
    )

    // NewClient builds a Client. ts is the oauth2.TokenSource from Plan 03's RunOAuth result.
    // For tests, callers can pass option.WithHTTPClient(...) by using NewClientWithOptions.
    func NewClient(ctx context.Context, ts oauth2.TokenSource, spreadsheetID string, opts ...option.ClientOption) (*Client, error) {
        all := append([]option.ClientOption{option.WithTokenSource(ts)}, opts...)
        svc, err := sheets.NewService(ctx, all...)
        if err != nil { return nil, err }
        return &Client{svc: svc, spreadsheetID: spreadsheetID, tabs: map[string]int64{}}, nil
    }

    func (c *Client) SetSpreadsheetID(id string) { c.spreadsheetID = id; c.tabs = map[string]int64{} }
    func (c *Client) SpreadsheetID() string      { return c.spreadsheetID }
    ```

    Create `internal/sheet/ensure_tab.go`:
    ```go
    package sheet

    import (
        "context"
        "fmt"

        "google.golang.org/api/sheets/v4"
    )

    // EnsureSheet returns the numeric sheetId of `name`, creating the tab if missing.
    // Cached in c.tabs. Idempotent.
    func (c *Client) EnsureSheet(ctx context.Context, name string) (int64, error) {
        if id, ok := c.tabs[name]; ok {
            return id, nil
        }
        // Refresh tab list from the spreadsheet.
        ss, err := c.svc.Spreadsheets.Get(c.spreadsheetID).
            Fields("sheets(properties(title,sheetId))").
            Context(ctx).Do()
        if err != nil {
            return 0, fmt.Errorf("get spreadsheet: %w", err)
        }
        for _, s := range ss.Sheets {
            c.tabs[s.Properties.Title] = s.Properties.SheetId
        }
        if id, ok := c.tabs[name]; ok {
            return id, nil
        }
        // Create the tab via batchUpdate AddSheet.
        req := &sheets.BatchUpdateSpreadsheetRequest{
            Requests: []*sheets.Request{{
                AddSheet: &sheets.AddSheetRequest{
                    Properties: &sheets.SheetProperties{Title: name},
                },
            }},
        }
        resp, err := c.svc.Spreadsheets.BatchUpdate(c.spreadsheetID, req).Context(ctx).Do()
        if err != nil {
            return 0, fmt.Errorf("addSheet %q: %w", name, err)
        }
        if len(resp.Replies) == 0 || resp.Replies[0].AddSheet == nil {
            return 0, fmt.Errorf("addSheet %q: empty reply", name)
        }
        id := resp.Replies[0].AddSheet.Properties.SheetId
        c.tabs[name] = id
        return id, nil
    }
    ```

    Create `internal/sheet/meta.go` per RESEARCH.md §12.3 lines 1227-1257, adapted for the
    interface signature. Read `_meta!A1:B2` (4 cells) using `spreadsheets.values.get`. Branch
    per &lt;behavior&gt; cases. The BOOTSTRAP path uses `spreadsheets.values.update` with
    `valueInputOption=RAW` writing `[["canonical_id", "squirebot-v1-workbook-2026"], ["schema_version", "1"]]`
    to `_meta!A1:B2`. Defensive: if `_meta` tab itself doesn't exist (per RESEARCH.md §12.3
    "_meta tab MUST already exist on the master template; if it doesn't, create it (addSheet) — defensive"),
    call EnsureSheet("_meta") first.

    ```go
    package sheet

    import (
        "context"
        "fmt"
        "strconv"

        "google.golang.org/api/sheets/v4"
    )

    // ValidateWorkbook performs the canonical_id + schema_version handshake.
    // Plan 06 picker callback calls this immediately after the user picks a spreadsheet.
    func (c *Client) ValidateWorkbook(ctx context.Context) error {
        if c.spreadsheetID == "" {
            return fmt.Errorf("ValidateWorkbook: spreadsheetID not set")
        }
        cid, sv, err := c.readMeta(ctx)
        if err != nil {
            return fmt.Errorf("read _meta: %w", err)
        }
        switch {
        case cid == "":
            return c.bootstrapMeta(ctx)
        case cid != CanonicalID:
            return ErrWrongWorkbook
        case sv > WatcherMaxSchemaVersion:
            return fmt.Errorf("%w (workbook v%d, watcher max v%d)",
                ErrSchemaTooNew, sv, WatcherMaxSchemaVersion)
        }
        return nil
    }

    // readMeta returns canonical_id (string) and schema_version (int).
    // Returns ("", 0, nil) when cells are empty (BOOTSTRAP signal).
    func (c *Client) readMeta(ctx context.Context) (string, int, error) {
        // Defensive: ensure _meta tab exists before reading.
        if _, err := c.EnsureSheet(ctx, "_meta"); err != nil {
            return "", 0, err
        }
        resp, err := c.svc.Spreadsheets.Values.
            Get(c.spreadsheetID, "_meta!A1:B2").
            Context(ctx).Do()
        if err != nil {
            return "", 0, err
        }
        var cid string
        var sv int
        for _, row := range resp.Values {
            if len(row) < 2 { continue }
            key, _ := row[0].(string)
            val, _ := row[1].(string)
            switch key {
            case "canonical_id":
                cid = val
            case "schema_version":
                if val != "" {
                    n, err := strconv.Atoi(val)
                    if err != nil {
                        return "", 0, fmt.Errorf("_meta.schema_version not an int: %q", val)
                    }
                    sv = n
                }
            }
        }
        return cid, sv, nil
    }

    func (c *Client) bootstrapMeta(ctx context.Context) error {
        if _, err := c.EnsureSheet(ctx, "_meta"); err != nil { return err }
        body := &sheets.ValueRange{
            Values: [][]any{
                {"canonical_id", CanonicalID},
                {"schema_version", strconv.Itoa(WatcherMaxSchemaVersion)},
            },
        }
        _, err := c.svc.Spreadsheets.Values.
            Update(c.spreadsheetID, "_meta!A1:B2", body).
            ValueInputOption("RAW").
            Context(ctx).Do()
        if err != nil {
            return fmt.Errorf("bootstrap _meta: %w", err)
        }
        return nil
    }
    ```

    Create `internal/sheet/meta_test.go` exercising the four ValidateWorkbook behaviors and the
    two EnsureSheet behaviors. Use httptest.NewServer + option.WithEndpoint to point the Sheets
    client at a stub server that returns canned JSON responses for `GET /v4/spreadsheets/{id}` (with
    sheets list), `GET /v4/spreadsheets/{id}/values/{range}` (with _meta cells), `PUT
    /v4/spreadsheets/{id}/values/{range}` (bootstrap write), and `POST /v4/spreadsheets/{id}:batchUpdate`
    (addSheet response). Use a recording http.Handler that captures requests for assertion:

    ```go
    func TestValidateWorkbook_Bootstrap(t *testing.T) {
        var bootstrapWritten bool
        srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            switch {
            case strings.HasSuffix(r.URL.Path, "/spreadsheets/SHEET1") && r.URL.Query().Get("fields") != "":
                // Get with fields filter — return sheet list including _meta
                json.NewEncoder(w).Encode(map[string]any{"sheets": []map[string]any{
                    {"properties": map[string]any{"title": "_meta", "sheetId": 12345}},
                }})
            case strings.Contains(r.URL.Path, "/values/_meta!A1:B2") && r.Method == "GET":
                json.NewEncoder(w).Encode(map[string]any{"values": [][]any{}}) // empty
            case strings.Contains(r.URL.Path, "/values/_meta!A1:B2") && r.Method == "PUT":
                bootstrapWritten = true
                json.NewEncoder(w).Encode(map[string]any{"updatedRange": "_meta!A1:B2", "updatedRows": 2})
            default:
                t.Logf("unhandled: %s %s", r.Method, r.URL.Path)
                w.WriteHeader(404)
            }
        }))
        defer srv.Close()

        ctx := context.Background()
        ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "fake"})
        c, err := NewClient(ctx, ts, "SHEET1",
            option.WithEndpoint(srv.URL),
            option.WithHTTPClient(srv.Client()))
        if err != nil { t.Fatal(err) }

        if err := c.ValidateWorkbook(ctx); err != nil { t.Fatal(err) }
        if !bootstrapWritten { t.Errorf("expected bootstrap write to happen") }
    }
    ```

    Repeat for the other 3 ValidateWorkbook scenarios + 2 EnsureSheet scenarios. (Six tests
    total in this file.)
  </action>
  <verify>
    <automated>go test ./internal/sheet/... -run "TestValidateWorkbook|TestEnsureSheet" -count=1 -timeout 30s &amp;&amp; grep -nE "squirebot-v1-workbook-2026" internal/sheet/client.go &amp;&amp; grep -nE "WatcherMaxSchemaVersion\s*=\s*1" internal/sheet/client.go &amp;&amp; grep -nE "InvTabMaxRows\s*=\s*500" internal/sheet/client.go &amp;&amp; grep -nE "ErrWrongWorkbook" internal/sheet/client.go &amp;&amp; grep -nE "_meta!A1:B2" internal/sheet/meta.go &amp;&amp; grep -nE "AddSheet" internal/sheet/ensure_tab.go &amp;&amp; grep -nE "ValueInputOption.*RAW" internal/sheet/meta.go</automated>
  </verify>
  <acceptance_criteria>
    - `internal/sheet/client.go` exports `Client`, `NewClient`, `CanonicalID`, `WatcherMaxSchemaVersion`, `InvTabMaxRows`, `InvTabColumns`, `ErrWrongWorkbook`, `ErrSchemaTooNew`
    - `client.go` contains literal `"squirebot-v1-workbook-2026"`
    - `client.go` contains literal `WatcherMaxSchemaVersion = 1`
    - `client.go` contains literal `InvTabMaxRows = 500`
    - `client.go` contains literal `This doesn't look like a SquireBot workbook. Pick the one shared by your guild leader.` (verbatim D-03 message)
    - `internal/sheet/meta.go` contains literal `_meta!A1:B2`
    - `meta.go` contains the literal string `"RAW"` (for ValueInputOption)
    - `internal/sheet/ensure_tab.go` contains `AddSheetRequest`
    - `internal/sheet/meta_test.go` contains at least 4 ValidateWorkbook test cases + 2 EnsureSheet cases
    - `go test ./internal/sheet/... -run "TestValidateWorkbook|TestEnsureSheet" -count=1` exits 0
    - `go vet ./internal/sheet/...` exits 0
  </acceptance_criteria>
  <done>
    Client constructor + EnsureSheet + ValidateWorkbook are implemented and tested against a stub
    Sheets server. The four canonical_id branches behave per RESEARCH.md §12.3-12.4 and
    Critical Constraint #5.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Atomic single-call inv:&lt;Char&gt; writer (UpdateCellsRequest with fields=userEnteredValue)</name>
  <files>internal/sheet/write.go, internal/sheet/write_test.go</files>
  <read_first>
    - .planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md (§2.3 Pattern 1 lines 298-365 — full WriteInventory code; §C "Code Examples" reproduces the request shape; §11 Pitfall 8 USER_ENTERED recalc-storm prohibition)
    - .planning/research/ARCHITECTURE.md (Watcher → Sheet Write Contract; OPS-01 per-character non-overlapping ranges)
    - ./CLAUDE.md ("Sheets writes: valueInputOption=RAW for the hot path. Atomic clear+write per character per file. Never append. Never row-diff."; "Per-character non-overlapping ranges only.")
    - internal/sheet/client.go (just created — confirm InvTabMaxRows + InvTabColumns + Client signature)
    - internal/sheet/ensure_tab.go (just created — confirm EnsureSheet signature)
  </read_first>
  <behavior>
    - Test 1: WriteInventory("Foo", InventoryHeader, [["General1","Cloth Cap","1001","1","0"]], "2026-04-30T18:00:00Z") issues exactly ONE call to /v4/spreadsheets/SHEET1:batchUpdate. Body MUST contain exactly one Request with an `updateCells` field.
    - Test 2: The captured request body's `updateCells.range` has SheetId = inv:Foo's sheetId, StartRowIndex=0, EndRowIndex=500, StartColumnIndex=0, EndColumnIndex=6.
    - Test 3: The captured `updateCells.fields` is exactly the literal string `"userEnteredValue"` (single field, not "*", not "userEnteredValue,note").
    - Test 4: The captured `updateCells.rows` has exactly len(dataRows)+1 elements (header + data); each row's cells use `userEnteredValue.stringValue` (NOT numberValue).
    - Test 5: WriteInventory triggers EnsureSheet on first sighting (test by inspecting an addSheet request appearing before the inv-write batchUpdate).
    - Test 6: WriteInventory with 0 dataRows still writes the header row + clears the rest of the range (regression: "user emptied bag" must clear stale rows). Captured rows array length = 1 (just header).
    - Test 7: Each data row has the `_uploaded_at` provenance column appended as the 6th cell.
  </behavior>
  <action>
    Create `internal/sheet/write.go` per RESEARCH.md §2.3 lines 305-363:

    ```go
    package sheet

    import (
        "context"
        "fmt"

        "google.golang.org/api/sheets/v4"
    )

    var InventoryHeader = []string{"Location", "Name", "ID", "Count", "Slots", "_uploaded_at"}

    // WriteInventory replaces the entire inv:<charName> tab atomically via a single batchUpdate.
    // Per RESEARCH.md §2.3 + Critical Constraint #3 + #8 + OPS-01.
    func (c *Client) WriteInventory(ctx context.Context, charName string, header []string, dataRows [][]string, uploadedAt string) error {
        if c.spreadsheetID == "" {
            return fmt.Errorf("WriteInventory: spreadsheetID not set")
        }
        tabName := "inv:" + charName
        sheetID, err := c.EnsureSheet(ctx, tabName)
        if err != nil {
            return fmt.Errorf("ensure %s: %w", tabName, err)
        }

        rows := make([]*sheets.RowData, 0, len(dataRows)+1)
        rows = append(rows, toRowData(header))
        for _, dr := range dataRows {
            // Append the _uploaded_at provenance column so every row carries it.
            // dr is 5 cols from parser; we make it 6.
            full := make([]string, 0, 6)
            full = append(full, dr...)
            // pad to 5 if parser returned a short row (defensive — parser already filters &lt;5)
            for len(full) < 5 { full = append(full, "") }
            full = append(full[:5], uploadedAt)
            rows = append(rows, toRowData(full))
        }

        req := &sheets.BatchUpdateSpreadsheetRequest{
            Requests: []*sheets.Request{{
                UpdateCells: &sheets.UpdateCellsRequest{
                    Range: &sheets.GridRange{
                        SheetId:          sheetID,
                        StartRowIndex:    0,
                        EndRowIndex:      InvTabMaxRows, // 500 — clears anything past N (atomic)
                        StartColumnIndex: 0,
                        EndColumnIndex:   InvTabColumns, // 6 = A:F
                    },
                    Rows:   rows,
                    Fields: "userEnteredValue", // Pitfall #8: NEVER include "note" or "*"
                },
            }},
        }
        _, err = c.svc.Spreadsheets.BatchUpdate(c.spreadsheetID, req).Context(ctx).Do()
        if err != nil {
            return fmt.Errorf("batchUpdate %s: %w", tabName, err)
        }
        return nil
    }

    // toRowData converts a []string to a *sheets.RowData using StringValue for every cell.
    // Pitfall #8: NEVER use NumberValue — even if the cell looks numeric — because that
    // triggers USER_ENTERED-style coercion in consolidated views (Phase 3+ recalc storm risk).
    func toRowData(cells []string) *sheets.RowData {
        vs := make([]*sheets.CellData, len(cells))
        for i, s := range cells {
            v := s // capture
            vs[i] = &sheets.CellData{
                UserEnteredValue: &sheets.ExtendedValue{StringValue: &v},
            }
        }
        return &sheets.RowData{Values: vs}
    }
    ```

    Create `internal/sheet/write_test.go` exercising the seven behaviors. Use the same
    httptest pattern as Task 1 — capture every batchUpdate POST body and assert structure:
    ```go
    func TestWriteInventory_AtomicSingleCall(t *testing.T) {
        var captured []*sheets.BatchUpdateSpreadsheetRequest
        srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            switch {
            case strings.HasSuffix(r.URL.Path, "/SHEET1") && r.Method == "GET":
                // List sheets — return one with inv:Foo (so EnsureSheet does NOT issue addSheet)
                json.NewEncoder(w).Encode(map[string]any{"sheets": []map[string]any{
                    {"properties": map[string]any{"title": "inv:Foo", "sheetId": 999}},
                }})
            case strings.HasSuffix(r.URL.Path, "/SHEET1:batchUpdate"):
                var req sheets.BatchUpdateSpreadsheetRequest
                json.NewDecoder(r.Body).Decode(&req)
                captured = append(captured, &req)
                json.NewEncoder(w).Encode(map[string]any{"replies": []any{}})
            default:
                w.WriteHeader(404)
            }
        }))
        defer srv.Close()

        ctx := context.Background()
        ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "fake"})
        c, _ := NewClient(ctx, ts, "SHEET1",
            option.WithEndpoint(srv.URL),
            option.WithHTTPClient(srv.Client()))
        c.tabs["inv:Foo"] = 999 // pre-cache to skip EnsureSheet's spreadsheets.get

        err := c.WriteInventory(ctx, "Foo", InventoryHeader,
            [][]string{{"General1", "Cloth Cap", "1001", "1", "0"}},
            "2026-04-30T18:00:00Z")
        if err != nil { t.Fatal(err) }

        if len(captured) != 1 {
            t.Fatalf("expected 1 batchUpdate, got %d", len(captured))
        }
        req := captured[0]
        if len(req.Requests) != 1 {
            t.Fatalf("expected 1 inner Request, got %d", len(req.Requests))
        }
        uc := req.Requests[0].UpdateCells
        if uc == nil { t.Fatal("expected UpdateCells request") }
        if uc.Range.SheetId != 999 { t.Errorf("sheetId=%d", uc.Range.SheetId) }
        if uc.Range.EndRowIndex != 500 { t.Errorf("EndRowIndex=%d", uc.Range.EndRowIndex) }
        if uc.Range.EndColumnIndex != 6 { t.Errorf("EndColumnIndex=%d", uc.Range.EndColumnIndex) }
        if uc.Fields != "userEnteredValue" { t.Errorf("Fields=%q", uc.Fields) }
        if len(uc.Rows) != 2 { t.Errorf("rows=%d (expected 1 header + 1 data)", len(uc.Rows)) }
        // Verify last row has _uploaded_at as 6th cell
        last := uc.Rows[1]
        if len(last.Values) != 6 { t.Errorf("data row cells=%d", len(last.Values)) }
        if last.Values[5].UserEnteredValue == nil ||
           last.Values[5].UserEnteredValue.StringValue == nil ||
           *last.Values[5].UserEnteredValue.StringValue != "2026-04-30T18:00:00Z" {
            t.Errorf("uploaded_at not in column F")
        }
        // Verify ID column written as StringValue (not NumberValue)
        if last.Values[2].UserEnteredValue.NumberValue != nil {
            t.Errorf("ID was written as NumberValue — Pitfall #8 violation")
        }
    }
    ```
  </action>
  <verify>
    <automated>go test ./internal/sheet/... -run "TestWriteInventory" -count=1 -timeout 30s &amp;&amp; grep -nE "UpdateCellsRequest" internal/sheet/write.go &amp;&amp; grep -nE 'Fields:\s*"userEnteredValue"' internal/sheet/write.go &amp;&amp; grep -nE "EndRowIndex:\s*InvTabMaxRows" internal/sheet/write.go &amp;&amp; grep -nE "EndColumnIndex:\s*InvTabColumns" internal/sheet/write.go &amp;&amp; ! grep -vE "^\s*//" internal/sheet/write.go | grep -E "NumberValue\s*:" &amp;&amp; grep -nE 'InventoryHeader\s*=\s*\[\]string' internal/sheet/write.go</automated>
  </verify>
  <acceptance_criteria>
    - `internal/sheet/write.go` exports `WriteInventory` and `InventoryHeader`
    - `write.go` contains literal `UpdateCellsRequest`
    - `write.go` contains literal `Fields: "userEnteredValue"` (Critical Constraint #3 + Pitfall #8)
    - `write.go` does NOT use `Fields: "*"` anywhere (would clear notes/formatting too — out of scope and risky)
    - `write.go` does NOT use `NumberValue` anywhere outside of comments (Pitfall #8 enforcement)
    - `write.go` references `InvTabMaxRows` and `InvTabColumns` (the locked constants from Task 1)
    - `InventoryHeader` slice is exactly: `["Location", "Name", "ID", "Count", "Slots", "_uploaded_at"]`
    - `internal/sheet/write_test.go` contains at least 7 test cases or sub-tests
    - `go test ./internal/sheet/... -run TestWriteInventory -count=1` exits 0
    - `go vet ./internal/sheet/...` exits 0
  </acceptance_criteria>
  <done>
    Atomic clear+write is a single batchUpdate call per RESEARCH.md §2.3. fields=userEnteredValue
    is locked. Every cell is StringValue (no numeric coercion). Tests assert request structure
    against a captured stub server.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: _char_owner upsert with email-mismatch policy</name>
  <files>internal/sheet/owner.go</files>
  <read_first>
    - .planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md (§12.5 _char_owner upsert lines 1264-1290 with full UpsertCharOwner code; first-write-wins policy)
    - .planning/research/ARCHITECTURE.md (Sheet Schema §_char_owner — column list, conflict resolution; Phase 1 minimum is char_name + owner_email + first_seen)
    - ./CLAUDE.md ("Identity: OAuth userinfo.email is canonical")
    - .planning/phases/01-end-to-end-thin-slice/01-CONTEXT.md (deferred: Refresh-token failure UX → Phase 2 AUTH-05; AUTH-06 sheet write side IS Phase 1)
    - internal/sheet/client.go (Task 1 — Client signature)
    - internal/sheet/ensure_tab.go (Task 1 — EnsureSheet signature)
  </read_first>
  <behavior>
    - Test 1: UpsertCharOwner on an empty `_char_owner` tab appends a new row `[charName, ownerEmail, "", "", isoTimestamp]` (Phase 1 minimum columns A,B,C,D,E per RESEARCH.md §12.5).
    - Test 2: UpsertCharOwner where charName already exists with the SAME email is a no-op (no append, no update — saves API quota).
    - Test 3: UpsertCharOwner where charName exists with a DIFFERENT email logs a slog.Warn and returns nil (D-03 silent + RESEARCH.md §12.5 "Phase 1 just logs"). Verify via slog.Handler that captures records.
    - Test 4: UpsertCharOwner ensures the `_char_owner` tab exists before reading (calls EnsureSheet defensively).
  </behavior>
  <action>
    Create `internal/sheet/owner.go` per RESEARCH.md §12.5 lines 1267-1287:

    ```go
    package sheet

    import (
        "context"
        "fmt"
        "log/slog"
        "time"

        "google.golang.org/api/sheets/v4"
    )

    // UpsertCharOwner appends a row to _char_owner if charName is absent.
    // Per AUTH-06 + RESEARCH.md §12.5. Phase 1 columns: A=char_name, B=owner_email,
    // C=display_name (blank), D=discord_handle (blank), E=first_seen.
    // (Phase 2 SCHEMA-05 adds class, level, is_bank_toon, is_hidden, is_removed, last_seen,
    // server, watcher_version — extend-only schema policy.)
    func (c *Client) UpsertCharOwner(ctx context.Context, charName, ownerEmail string) error {
        if c.spreadsheetID == "" {
            return fmt.Errorf("UpsertCharOwner: spreadsheetID not set")
        }
        if _, err := c.EnsureSheet(ctx, "_char_owner"); err != nil {
            return err
        }
        // Read columns A:B (char_name + owner_email) for upsert lookup.
        resp, err := c.svc.Spreadsheets.Values.
            Get(c.spreadsheetID, "_char_owner!A:B").
            Context(ctx).Do()
        if err != nil {
            return fmt.Errorf("read _char_owner: %w", err)
        }
        nowISO := time.Now().UTC().Format(time.RFC3339)
        for i, row := range resp.Values {
            if i == 0 { continue } // header (if present)
            if len(row) >= 1 {
                name, _ := row[0].(string)
                if name == charName {
                    var existingEmail string
                    if len(row) >= 2 {
                        existingEmail, _ = row[1].(string)
                    }
                    if existingEmail == ownerEmail {
                        return nil // exact match → no-op
                    }
                    // Mismatch — log and proceed; Phase 2 surfaces in _audit.
                    slog.Warn("char_owner email mismatch",
                        "char", charName,
                        "existing", existingEmail,
                        "current", ownerEmail)
                    return nil
                }
            }
        }
        // Append a new row. Use values.append with valueInputOption=RAW so the email
        // is not interpreted as a hyperlink etc.
        body := &sheets.ValueRange{
            Values: [][]any{
                {charName, ownerEmail, "", "", nowISO},
            },
        }
        _, err = c.svc.Spreadsheets.Values.
            Append(c.spreadsheetID, "_char_owner!A:E", body).
            ValueInputOption("RAW").
            InsertDataOption("INSERT_ROWS").
            Context(ctx).Do()
        if err != nil {
            return fmt.Errorf("append _char_owner: %w", err)
        }
        return nil
    }
    ```

    Add tests to `internal/sheet/write_test.go` (or a new `internal/sheet/owner_test.go`)
    covering the four behaviors. For Test 3 (mismatch logging), use a custom slog.Handler that
    captures records into an in-memory slice and assert the slice contains a `WARN` record with
    the message `char_owner email mismatch`.

    Document at the top of owner.go:
    ```go
    // Conflict policy (RESEARCH.md §12.5):
    //   first-write wins for owner_email
    //   subsequent mismatches → slog.Warn (Phase 1) → _audit row (Phase 2 — AUTH-05 ticket)
    //   inv:<Char> write itself is NOT gated on email match (per RESEARCH.md "we don't gate writes on owner")
    ```
  </action>
  <verify>
    <automated>go test ./internal/sheet/... -run "TestUpsertCharOwner" -count=1 -timeout 30s &amp;&amp; grep -nE "_char_owner" internal/sheet/owner.go &amp;&amp; grep -nE "Append" internal/sheet/owner.go &amp;&amp; grep -nE "slog\.Warn.*mismatch" internal/sheet/owner.go &amp;&amp; grep -nE "ValueInputOption.*RAW" internal/sheet/owner.go    <automated>go test ./internal/sheet/... -run TestUpsertCharOwner -count=1 -timeout 30s</automated>
  </verify>
  <acceptance_criteria>
    - `internal/sheet/owner.go` exports `UpsertCharOwner`
    - `owner.go` contains literal `_char_owner` (the tab name)
    - `owner.go` contains literal `Append` (uses values.append, not values.update — append is the right shape for the upsert miss path)
    - `owner.go` contains `slog.Warn` with literal message `char_owner email mismatch`
    - `owner.go` contains `ValueInputOption("RAW")`
    - `owner.go` writes exactly 5 columns: charName, ownerEmail, blank, blank, nowISO (Phase 1 schema)
    - Test cases for the four behaviors exist (`go test -run TestUpsertCharOwner` finds at least 4 sub-tests or top-level Tests)
    - `go test ./internal/sheet/... -count=1` exits 0 across ALL tests in the package
    - `go vet ./internal/sheet/...` exits 0
  </acceptance_criteria>
  <done>
    UpsertCharOwner appends on first sighting and warns on email mismatch without overwriting.
    AUTH-06 sheet-side is satisfied. Phase 2 will extend the schema with class/level/etc per
    SCHEMA-05 (extend-only).
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Go process ↔ sheets.googleapis.com | OAuth token in transit; HTTPS-only; service-side identity |
| picked spreadsheetId ↔ Sheets API | Untrusted user input; canonical_id check is the workbook-authentication mechanism |
| _meta cell values ↔ canonical_id constant | Drift between watcher and master template canonical_id breaks all bootstrap |
| _char_owner email column ↔ OAuth userinfo.email | Trust boundary: email comes from Google, not user input |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-05-01 | Tampering | User picks an attacker-controlled spreadsheet (D-03 attack scenario) | mitigate | ValidateWorkbook reads `_meta.canonical_id` and rejects unless it equals `"squirebot-v1-workbook-2026"`; verbatim D-03 message returned to wizard |
| T-05-02 | Privilege Escalation | Schema_version > WatcherMaxSchemaVersion → old watcher writes malformed data corrupting newer schema | mitigate | ValidateWorkbook returns ErrSchemaTooNew BEFORE any inv:&lt;Char&gt; write happens; Critical Constraint #5 fail-fast; tray icon will turn red and surface "update SquireBot" via Plan 07 |
| T-05-03 | Tampering | A malicious workbook with canonical_id pre-baked but a hostile schema_version | mitigate | schema_version is also checked; even with matching canonical_id the watcher fails fast on schema mismatch |
| T-05-04 | Information Disclosure | Sheets API errors include sheet content snippets in slog logs | mitigate | error wrapping uses `%w` and only logs the operation name + error message; no Values payload is logged |
| T-05-05 | Information Disclosure | Race condition during clear+write atomic semantics — reader sees post-clear/pre-write state | mitigate | Single batchUpdate UpdateCellsRequest is atomic per Google API contract (RESEARCH.md §2.3 cite); two-call clear+update is explicitly NOT used |
| T-05-06 | Tampering | Hot-path uses USER_ENTERED → recalc storm in consolidated views (Phase 3+) AND value coercion drops leading zeros from item IDs | mitigate | Pitfall #8: Fields="userEnteredValue" + StringValue (never NumberValue); acceptance grep enforces this; Critical Constraint #8 |
| T-05-07 | Spoofing | Two watchers writing the same inv:&lt;Char&gt; tab simultaneously | accept | OPS-01: per-character non-overlapping ranges; the only way to hit this is the same Google account installed on two PCs editing the same character — RESEARCH.md §12 "I have two PCs" case is documented as last-write-wins, acceptable for Phase 1 |
| T-05-08 | Tampering | _char_owner email mismatch silently overwrites correct owner | mitigate | First-write wins; mismatch is NEVER overwritten; only logged via slog.Warn (Phase 2 surfaces in _audit). RESEARCH.md §12.5 conflict resolution |
| T-05-09 | Information Disclosure | Sheets v4 client embeds OAuth Bearer token in HTTP request URLs (logging accident) | mitigate | google-api-go-client uses Authorization header, not URL params, for tokens; logging hygiene already enforced in Plan 03 |
| T-05-10 | Denial of Service | A malicious workbook with millions of rows in _meta crashes readMeta | mitigate | readMeta uses fixed range `_meta!A1:B2` (4 cells) — bounded read regardless of workbook size |
</threat_model>

<verification>
- `go build ./internal/sheet/...` exits 0
- `go vet ./internal/sheet/...` exits 0
- `go test ./internal/sheet/... -count=1 -timeout 60s` exits 0
- All ValidateWorkbook branches have at least one test case
- Captured batchUpdate body in Test 1 contains exactly 1 UpdateCellsRequest with `Fields="userEnteredValue"`, `EndRowIndex=500`, `EndColumnIndex=6`
- `grep -rE "Fields:\s*\"\*\"" internal/sheet/` returns 0 matches (NEVER use wildcard fields — would clear notes)
- `grep -rE "valueInputOption.*USER_ENTERED|ValueInputOption.*USER_ENTERED" internal/sheet/` returns 0 matches (Critical Constraint #8 / Pitfall #8)
- `grep -rE "NumberValue\s*:" internal/sheet/ | grep -v "_test\.go" | grep -v '^[^:]*:\s*//'` returns 0 non-comment matches (StringValue only on hot path)
- `grep -rE "spreadsheets/v4/.*spreadsheets\":\"|values.batchClear" internal/sheet/` returns 0 matches (the explicit anti-pattern from Critical Constraint #3 — "atomic Sheets write is ONE batchUpdate call, not 'clear then write'")
- `grep -rE "squirebot-v1-workbook-2026" internal/sheet/` returns matches (the canonical_id constant must be present)
</verification>

<success_criteria>
- OPS-01 satisfied: every WriteInventory call writes only to `inv:<Char>!A1:F500` with the character-specific GridRange — zero shared mutable ranges
- AUTH-06 satisfied (sheet side): UpsertCharOwner appends `(char_name, owner_email, '', '', first_seen)` on first sighting; never overwrites a mismatched owner_email
- Critical Constraint #3 satisfied: clear+write is ONE batchUpdate call (single UpdateCellsRequest with Fields="userEnteredValue")
- Critical Constraint #5 satisfied: ValidateWorkbook fail-fasts on schema_version > 1 BEFORE any write
- Critical Constraint #8 satisfied: hot path uses StringValue, never NumberValue, and never USER_ENTERED
- D-03 satisfied: workbook canonical_id mismatch returns the verbatim "This doesn't look like a SquireBot workbook…" error
- Bootstrap path (RESEARCH.md §12.3) handles fresh-template case where _meta is empty — first guildie can onboard
- All sheet-package tests pass against an httptest stub server (no real GCP calls in CI)
</success_criteria>

<output>
After completion, create `.planning/phases/01-end-to-end-thin-slice/01-05-SUMMARY.md` documenting:
- The exact UpdateCellsRequest shape used (range, fields, row count for a typical write)
- Whether _char_owner header row is auto-managed or hand-managed in the master template (Open Question Q1 implication)
- Bootstrap behavior observed when run against a fresh-from-template workbook
- Any deviations from RESEARCH.md §2.3 / §12 and why
- A note for Plan 07 explaining the Plan 07 wiring sequence: `parse.Parse → WriteInventory → UpsertCharOwner` per character per file event
- A note for Plan 06 explaining that ValidateWorkbook is the picker callback's validation step
</output>
