// Package scaffold owns the one-time, idempotent workbook bootstrap that
// brings a freshly-shared SquireBot workbook to schema_version=1. Plan
// 02-01 Task 2 — the schema lock entry point.
//
// ScaffoldSchemaV1 is called by internal/app/runapp.go on every cold
// start, immediately after ValidateWorkbook returns Empty or Matches.
// The function is idempotent: against a workbook that already has every
// scaffold tab and every _meta key, it issues only the bulk-read
// ListSheets call and the single _meta!A:A column read — zero writes.
//
// Locked schema (DO NOT EDIT after Phase 2 freezes — extend right only):
//
//   - 9 hidden dimension tabs ("_-prefixed) — DimensionTabs.
//   - 4 visible consolidated mega-tab placeholders — ViewTabs.
//   - 13 _meta KV rows — MetaRows.
//
// These three slices are the one source of truth for the v1 schema. Any
// future column or _meta key MUST be appended at the end of its
// containing slice (right edge), never inserted in the middle. Phase 3+
// scripts read by column index, so reordering breaks them.
//
// Pitfall C reminder (RESEARCH §Pitfall C): ValidateWorkbook returns
// WorkbookStateWrong for a workbook with _meta but no canonical_id, so
// ScaffoldSchemaV1 will never run against a non-SquireBot workbook the
// user happened to pick. We rely on that gate; we do not re-check.
package scaffold

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/boejowen/SquireBot/internal/sheet"
)

// dimensionTab describes a tab the scaffold is responsible for creating
// if absent. Hidden flag is implicit by membership: every tab in
// DimensionTabs is hidden after creation; every tab in ViewTabs stays
// visible.
type dimensionTab struct {
	Name    string
	Headers []string // row 1 column headers
}

// DimensionTabs is the locked list of 9 hidden _-prefixed dimension tabs.
// Every column listed here exists at v1 freeze even if no v1 UI populates
// it (per CONTEXT.md SCHEMA-05 — soft-delete + discord_handle scaffolded).
//
// Source of truth: 02-RESEARCH.md §Pattern 5 schema-scaffold table.
var DimensionTabs = []dimensionTab{
	{Name: "_meta", Headers: []string{"key", "value"}},
	{Name: "_char_owner", Headers: []string{
		"char_name", "owner_email", "display_name", "discord_handle",
		"class", "level", "is_bank_toon", "is_hidden", "is_removed",
		"first_seen", "last_seen", "server", "watcher_version",
	}},
	{Name: "_item_master", Headers: []string{
		"item_id", "name", "wiki_summary", "wiki_url",
		"slot", "is_quest_item", "last_refreshed",
	}},
	{Name: "_pigparse", Headers: []string{
		"item_id", "name", "current_avg", "last_seen",
		"blue_volume", "last_refreshed",
	}},
	{Name: "_wiki_spells", Headers: []string{
		"class", "level", "spell_name", "normalized_name", "last_refreshed",
	}},
	{Name: "_wiki_gear_tier", Headers: []string{
		"tier", "class", "slot", "item_id", "item_name", "rank", "last_refreshed",
	}},
	{Name: "_quest_items", Headers: []string{
		"item_id", "quest_name", "source_url", "last_refreshed",
	}},
	{Name: "_audit", Headers: []string{
		"timestamp", "owner_email", "char_name", "file_type",
		"rows_written", "watcher_version",
	}},
	{Name: "_status", Headers: []string{
		"owner_email", "char_name", "watcher_version",
		"last_inventory_upload", "last_spellbook_upload", "last_heartbeat",
	}},
}

// ViewTabs is the locked list of 4 visible consolidated mega-tab
// placeholders. Phase 2 only writes the header row; Phase 3+ populates.
// Each tab leads with `Char` (CONTEXT.md SCHEMA-04 — consolidated, never
// per-character).
var ViewTabs = []dimensionTab{
	{Name: "view", Headers: []string{
		"Char", "Slot", "Item", "ID", "Count", "Wiki", "Price", "Last Synced",
	}},
	{Name: "gear_check", Headers: []string{
		"Char", "Class", "Tier", "Slot", "Have", "Recommended", "Status",
	}},
	{Name: "spell_check", Headers: []string{
		"Char", "Class", "Level", "Spell", "Status",
	}},
	{Name: "bank", Headers: []string{
		"Char", "Slot", "Item", "ID", "Count", "Wiki", "Price", "Last Synced",
	}},
}

// MetaRows is the locked list of 13 _meta KV rows written exactly once
// at v1 freeze. After scaffold, no row is overwritten; later phases write
// new keys at the END of the slice (extend-only). The first two are the
// load-bearing schema_version + canonical_id pair the picker validates
// against; the remaining 11 are placeholders Phase 3+ populates.
var MetaRows = [][]string{
	{"schema_version", "1"},
	{"canonical_id", sheet.CanonicalID},
	{"bank_toon_name", ""},
	{"bank_coin_pp", ""},
	{"bank_coin_gp", ""},
	{"bank_coin_sp", ""},
	{"bank_coin_cp", ""},
	{"last_pigparse_refresh", ""},
	{"last_wiki_summary_refresh", ""},
	{"last_wiki_spell_refresh", ""},
	{"last_wiki_gear_refresh", ""},
	{"last_quest_items_refresh", ""},
	{"last_error", ""},
}

// ScaffoldSchemaV1 brings the workbook to schema_version=1. Called from
// runWatcher after ValidateWorkbook returns Empty or Matches. Idempotent.
//
// Sequence:
//  1. ListSheets — single bulk read of existing tab titles + sheetIds + hidden flag.
//  2. For each DimensionTab:
//     - missing → EnsureSheet, WriteHeaderRow, HideSheet
//     - present but visible → HideSheet (fixes up tabs created visible
//       by upstream EnsureSheet side-effects, e.g. ValidateWorkbook's
//       defensive EnsureSheet("_meta") and the heartbeat code path
//       that creates "_status" before scaffold's loop reaches it)
//     - present and hidden → no-op
//  3. For each ViewTab missing → EnsureSheet, WriteHeaderRow (no hide).
//  4. ReadColumn _meta!A:A — collect existing keys.
//  5. For each MetaRow whose key is NOT in the existing set → AppendRow.
//
// On a fully-scaffolded workbook with all dimension tabs already hidden
// (second-run / steady-state), steps 2-3 hit zero AddSheet, zero
// WriteHeaderRow, and zero HideSheet calls. Step 5 hits zero AppendRow
// calls. Net cost: one Get + one Get = two API calls.
func ScaffoldSchemaV1(ctx context.Context, sc *sheet.Client) error {
	existing, err := sc.ListSheets(ctx)
	if err != nil {
		return fmt.Errorf("list sheets: %w", err)
	}

	// Step 2: hidden dimension tabs.
	for _, tab := range DimensionTabs {
		if meta, present := existing[tab.Name]; present {
			// Tab exists. Ensure it's hidden — it may have been
			// created visible by an upstream EnsureSheet side-effect
			// (Day-0 finding from soak validation 2026-05-02:
			// ValidateWorkbook's defensive EnsureSheet("_meta") and
			// heartbeat's EnsureSheet("_status") run BEFORE this
			// loop, so without this branch _meta and _status remain
			// visible on every fresh install).
			if !meta.Hidden {
				if err := sc.HideSheet(ctx, meta.ID); err != nil {
					return fmt.Errorf("hide pre-existing %s: %w", tab.Name, err)
				}
				slog.Info("scaffold: hid pre-existing dimension tab",
					"tab", tab.Name)
			}
			continue
		}
		id, err := sc.EnsureSheet(ctx, tab.Name)
		if err != nil {
			return fmt.Errorf("ensure %s: %w", tab.Name, err)
		}
		if err := sc.WriteHeaderRow(ctx, tab.Name, tab.Headers); err != nil {
			return fmt.Errorf("write header %s: %w", tab.Name, err)
		}
		if err := sc.HideSheet(ctx, id); err != nil {
			return fmt.Errorf("hide %s: %w", tab.Name, err)
		}
		slog.Info("scaffold: created dimension tab",
			"tab", tab.Name, "cols", len(tab.Headers))
	}

	// Step 3: visible view-tab placeholders.
	for _, tab := range ViewTabs {
		if _, present := existing[tab.Name]; present {
			continue
		}
		if _, err := sc.EnsureSheet(ctx, tab.Name); err != nil {
			return fmt.Errorf("ensure %s: %w", tab.Name, err)
		}
		if err := sc.WriteHeaderRow(ctx, tab.Name, tab.Headers); err != nil {
			return fmt.Errorf("write header %s: %w", tab.Name, err)
		}
		slog.Info("scaffold: created view tab",
			"tab", tab.Name, "cols", len(tab.Headers))
	}

	// Steps 4 + 5: append missing _meta rows. Read column A from _meta to
	// build the set of existing keys; append each MetaRow whose key is
	// not in the set. Note: the header row "key" itself is the first
	// entry in column A, so it's correctly treated as "already present"
	// (none of MetaRows uses the literal key "key"). The header was
	// written in step 2 only if _meta was created in this run; if _meta
	// pre-existed, the user's existing header (if any) is preserved.
	existingKeys, err := sc.ReadColumn(ctx, "_meta!A:A")
	if err != nil {
		return fmt.Errorf("read _meta!A:A: %w", err)
	}
	have := make(map[string]bool, len(existingKeys))
	for _, k := range existingKeys {
		have[strings.TrimSpace(k)] = true
	}

	appended := 0
	for _, row := range MetaRows {
		key := row[0]
		if have[key] {
			continue
		}
		if err := sc.AppendRow(ctx, "_meta!A:B", row); err != nil {
			return fmt.Errorf("append _meta key %q: %w", key, err)
		}
		have[key] = true
		appended++
	}
	if appended > 0 {
		slog.Info("scaffold: _meta rows appended", "count", appended)
	}
	return nil
}
