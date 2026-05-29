package ingest

import (
	"errors"
	"strings"
	"testing"
)

// TestDecodeAndValidate is the V5 envelope-validation table test: it covers the
// happy paths (valid inventory, valid spellbook, empty-content no-op) and every
// rejection path (missing character, empty kind, unknown kind, malformed JSON),
// asserting the right sentinel error is returned for each so handler.go can map
// it to the correct 4xx.
func TestDecodeAndValidate(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr error // nil = expect success; otherwise errors.Is target
		// On success, the decoded fields we assert round-tripped.
		wantChar string
		wantKind string
	}{
		{
			name:     "valid inventory envelope",
			body:     `{"character":"Slampeach","kind":"inventory","content":"General1\tThing\t1234\t1\t0\n","watcher_version":"2.0.0"}`,
			wantErr:  nil,
			wantChar: "Slampeach",
			wantKind: KindInventory,
		},
		{
			name:     "valid spellbook envelope",
			body:     `{"character":"Slampeach","kind":"spellbook","content":"1\tMinor Healing\n","watcher_version":"2.0.0"}`,
			wantErr:  nil,
			wantChar: "Slampeach",
			wantKind: KindSpellbook,
		},
		{
			name:     "empty content with valid character+kind is a valid no-op snapshot",
			body:     `{"character":"Slampeach","kind":"inventory","content":"","watcher_version":"2.0.0"}`,
			wantErr:  nil,
			wantChar: "Slampeach",
			wantKind: KindInventory,
		},
		{
			name:     "unknown fields are ignored (forward-compatible envelope)",
			body:     `{"character":"Slampeach","kind":"inventory","content":"","future_field":"x"}`,
			wantErr:  nil,
			wantChar: "Slampeach",
			wantKind: KindInventory,
		},
		{
			name:    "missing character is rejected",
			body:    `{"kind":"inventory","content":"x"}`,
			wantErr: ErrMissingCharacter,
		},
		{
			name:    "empty character is rejected",
			body:    `{"character":"","kind":"inventory","content":"x"}`,
			wantErr: ErrMissingCharacter,
		},
		{
			name:    "empty kind is rejected",
			body:    `{"character":"Slampeach","kind":"","content":"x"}`,
			wantErr: ErrInvalidKind,
		},
		{
			name:    "unknown kind is rejected (enum)",
			body:    `{"character":"Slampeach","kind":"badkind","content":"x"}`,
			wantErr: ErrInvalidKind,
		},
		{
			name:    "malformed JSON is rejected",
			body:    `{"character":"Slampeach","kind":"inventory",`,
			wantErr: ErrMalformedJSON,
		},
		{
			name:    "non-JSON garbage is rejected as malformed",
			body:    `this is not json`,
			wantErr: ErrMalformedJSON,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := DecodeAndValidate(strings.NewReader(tt.body))

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("DecodeAndValidate() error = %v, want errors.Is(%v)", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("DecodeAndValidate() unexpected error = %v", err)
			}
			if env.Character != tt.wantChar {
				t.Errorf("Character = %q, want %q", env.Character, tt.wantChar)
			}
			if env.Kind != tt.wantKind {
				t.Errorf("Kind = %q, want %q", env.Kind, tt.wantKind)
			}
		})
	}
}

// TestEnvelope_JSONTags asserts the D-04 wire shape: the struct's json tags are
// exactly character/kind/content/watcher_version (the watcher and server must
// agree on these). A drift here would silently break ingest.
func TestEnvelope_JSONTags(t *testing.T) {
	const body = `{"character":"C","kind":"spellbook","content":"1\tA","watcher_version":"9.9.9"}`
	env, err := DecodeAndValidate(strings.NewReader(body))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Character != "C" || env.Kind != "spellbook" || env.Content != "1\tA" || env.WatcherVersion != "9.9.9" {
		t.Errorf("json tag mapping wrong: got %+v", env)
	}
}
