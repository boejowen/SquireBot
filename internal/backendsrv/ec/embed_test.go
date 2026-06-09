package ec

// embed_test.go covers the CWANT-05 display-only "For <char>" embed field: it is
// present (with the character name) only when the matched want is character-tagged
// (hit.CharacterName non-nil AND non-blank), and OMITTED otherwise. The field is
// DISPLAY-ONLY — buildEmbed has no send path, so these tests never touch the DM
// recipient (the owner-target invariant is proven in wantmatch/match_test.go). Reuses
// the buildEmbed test helpers (fieldByName / mkAuction / intp) from ec_test.go.

import (
	"testing"
	"time"

	"github.com/boejowen/SquireBot/internal/backendsrv/wantmatch"
)

// strp is a small *string helper for the optional CharacterName.
func strp(s string) *string { return &s }

// TestBuildEmbed_TaggedWant_HasForField: a character-tagged want renders a "For"
// field whose Value is the character name.
func TestBuildEmbed_TaggedWant_HasForField(t *testing.T) {
	hit := wantmatch.Hit{
		WantID:        9,
		DiscordUserID: "alice",
		ItemName:      "Fungi Tunic",
		Reason:        "buy",
		CharacterName: strp("Tankbert"),
	}
	now := time.Date(2026, 6, 6, 2, 3, 0, 0, time.UTC)
	a := mkAuction(0, "2026-06-06T02:00:00+00:00", intp(1500))
	e := buildEmbed(hit, a, "Seller1", seenAgo(a.T, now))

	forField := fieldByName(e, "For")
	if forField == nil {
		t.Fatal("For field missing for a character-tagged want; want present (CWANT-05)")
	}
	if forField.Value != "Tankbert" {
		t.Errorf("For field Value = %q; want %q", forField.Value, "Tankbert")
	}
}

// TestBuildEmbed_UntaggedWant_NoForField: an untagged want (CharacterName nil) omits
// the "For" field entirely.
func TestBuildEmbed_UntaggedWant_NoForField(t *testing.T) {
	hit := wantmatch.Hit{
		WantID:        10,
		DiscordUserID: "bob",
		ItemName:      "Rubicite Helm",
		Reason:        "quest",
		// CharacterName nil ⇒ account-level want.
	}
	now := time.Date(2026, 6, 6, 2, 3, 0, 0, time.UTC)
	a := mkAuction(0, "2026-06-06T02:00:00+00:00", intp(2000))
	e := buildEmbed(hit, a, "", seenAgo(a.T, now))

	if f := fieldByName(e, "For"); f != nil {
		t.Errorf("For field present (%q) for an untagged want; want OMITTED", f.Value)
	}
}

// TestBuildEmbed_BlankCharacterName_NoForField: a CharacterName pointing at a
// whitespace-only string is treated as untagged (the TrimSpace guard) — no "For" field.
func TestBuildEmbed_BlankCharacterName_NoForField(t *testing.T) {
	hit := wantmatch.Hit{
		WantID:        11,
		DiscordUserID: "carol",
		ItemName:      "Cloak of Flames",
		Reason:        "buy",
		CharacterName: strp("   "),
	}
	now := time.Date(2026, 6, 6, 2, 3, 0, 0, time.UTC)
	a := mkAuction(0, "2026-06-06T02:00:00+00:00", intp(500))
	e := buildEmbed(hit, a, "", seenAgo(a.T, now))

	if f := fieldByName(e, "For"); f != nil {
		t.Errorf("For field present (%q) for a whitespace-only CharacterName; want OMITTED (TrimSpace guard)", f.Value)
	}
}
