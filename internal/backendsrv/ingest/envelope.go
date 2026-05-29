// Package ingest implements the SquireBot backend's single network-exposed write
// surface: POST /api/v1/ingest. This file owns the request envelope (D-04) and
// its decode+validation; handler.go composes the full request flow (bearer guard
// -> decode -> parse -> first-sighting bind -> atomic replace).
//
// Per the 11-01 verdict (HAND-ROLLED Go fallback), the HTTP shell is stdlib
// net/http — there is no PocketBase. The business logic this package composes
// (auth guard, parser, store tx) is verdict-agnostic and imported unchanged from
// 11-02/03/04.
package ingest

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Envelope is the POST /api/v1/ingest request body (D-04, verbatim). The watcher
// (P13) POSTs the RAW /outputfile file text in Content; the SERVER parses it
// (D-03). The character name comes from the watcher's FILENAME (not the file
// body), so it must travel in the envelope. Content is UTF-8 text (encoding
// contract A1) — the server feeds it straight into parse.Parse / ParseSpellbook
// with NO CP1252 decode (the watcher owns that decode on the disk-read side; do
// not double-decode).
type Envelope struct {
	Character      string `json:"character"`
	Kind           string `json:"kind"`            // "inventory" | "spellbook" (enum)
	Content        string `json:"content"`         // RAW /outputfile text, UTF-8 (A1)
	WatcherVersion string `json:"watcher_version"` // accepted now; version-gate reject is P13
}

// Validation errors. The handler maps each to a specific 4xx (V5):
//   - ErrMalformedJSON       -> 400 Bad Request (the body is not valid JSON)
//   - ErrMissingCharacter    -> 422 Unprocessable Entity (required field absent)
//   - ErrInvalidKind         -> 422 Unprocessable Entity (kind not in the enum)
//
// They are sentinel errors so the handler can errors.Is them rather than string-
// matching. NONE of them carry the raw content or any token material (V7).
var (
	// ErrMalformedJSON means the request body could not be decoded as the JSON
	// envelope (syntax error, wrong types, truncated body, etc.).
	ErrMalformedJSON = errors.New("malformed JSON envelope")
	// ErrMissingCharacter means the required `character` field was empty/absent.
	ErrMissingCharacter = errors.New("missing required field: character")
	// ErrInvalidKind means `kind` was not exactly "inventory" or "spellbook".
	ErrInvalidKind = errors.New("invalid kind: must be \"inventory\" or \"spellbook\"")
)

// Kind constants — the only two accepted values for Envelope.Kind (the enum
// validated by DecodeAndValidate). Anything else is ErrInvalidKind.
const (
	KindInventory = "inventory"
	KindSpellbook = "spellbook"
)

// DecodeAndValidate reads a JSON Envelope from r and validates it (V5):
//
//   - the body MUST decode as the Envelope JSON shape (else ErrMalformedJSON);
//   - Character MUST be non-empty (else ErrMissingCharacter);
//   - Kind MUST be exactly "inventory" or "spellbook" (else ErrInvalidKind);
//   - Content is NOT required to be non-empty: an empty snapshot is a valid
//     no-op (parse.Parse / ParseSpellbook return (nil, nil) for empty input,
//     and the atomic replace then just clears the character's rows). This
//     mirrors the watcher's "empty file => clear" semantics.
//
// The caller (handler.go) is responsible for capping the body size with
// http.MaxBytesReader BEFORE calling this (V5: reject >1 MB — a maxed character
// snapshot is <50 KB). DecodeAndValidate uses json.Decoder so a too-large body
// surfaces as a decode error here (mapped to a 4xx) when the cap fires mid-read.
//
// DisallowUnknownFields is intentionally NOT set: the envelope is forward-
// compatible (a newer watcher may add fields the server ignores), consistent
// with WatcherVersion being "accepted now, gated in P13". Unknown fields are
// silently ignored, not rejected.
//
// SECURITY (V7): on any failure the returned error names only the FIELD at
// fault — never the raw Content and never any token/Authorization value.
func DecodeAndValidate(r io.Reader) (Envelope, error) {
	var env Envelope
	dec := json.NewDecoder(r)
	if err := dec.Decode(&env); err != nil {
		// Wrap with the sentinel so the handler maps to 400 while preserving the
		// underlying decode detail for logs (decode errors never echo content).
		return Envelope{}, fmt.Errorf("%w: %v", ErrMalformedJSON, err)
	}

	if env.Character == "" {
		return Envelope{}, ErrMissingCharacter
	}
	if env.Kind != KindInventory && env.Kind != KindSpellbook {
		return Envelope{}, ErrInvalidKind
	}

	return env, nil
}
