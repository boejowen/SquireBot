package app

import (
	"testing"

	"github.com/boejowen/SquireBot/internal/config"
)

func TestExtractCharName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		// Plain ASCII char names — the common case.
		{`C:\P99\Foo-Inventory.txt`, "Foo"},
		{`/tmp/eq/Bar-Inventory.txt`, "Bar"},
		// Spaces in char names (rare on P99 but multi-word names exist).
		{`C:\P99\Cool Toon-Inventory.txt`, "Cool Toon"},
		// Unicode-ish character (sanity-check the regex doesn't choke).
		{`C:\P99\Mörk-Inventory.txt`, "Mörk"},
		// Non-inventory file → ""
		{`C:\P99\Foo-Spellbook.txt`, ""},
		// Path doesn't end in -Inventory.txt → ""
		{`C:\P99\Foo-Inventory.bak`, ""},
		// Empty.
		{``, ""},
		// Just the suffix → "" (regex requires at least one char in the prefix group)
		{`-Inventory.txt`, ""},
	}
	for _, tc := range cases {
		got := extractCharName(tc.in)
		if got != tc.want {
			t.Errorf("extractCharName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNeedsWizard(t *testing.T) {
	cases := []struct {
		name string
		cfg  *config.Config
		want bool
	}{
		{"empty", &config.Config{}, true},
		{"only email", &config.Config{GoogleEmail: "a@b.com"}, true},
		{"only spreadsheet", &config.Config{SpreadsheetID: "X"}, true},
		{"only folder", &config.Config{EQFolder: `C:\P99`}, true},
		{"email+spreadsheet missing folder", &config.Config{GoogleEmail: "a@b.com", SpreadsheetID: "X"}, true},
		{"all three", &config.Config{GoogleEmail: "a@b.com", SpreadsheetID: "X", EQFolder: `C:\P99`}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := needsWizard(tc.cfg)
			if got != tc.want {
				t.Errorf("needsWizard(%+v) = %v, want %v", tc.cfg, got, tc.want)
			}
		})
	}
}
