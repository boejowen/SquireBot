// Package assets bundles SquireBot's runtime resources (icons) into the
// final binary so the .exe is self-contained.
//
// The package lives under assets/ rather than cmd/squirebot/ because Go's
// embed directive cannot reach files via `..` (the pattern is confined to
// the package's own subtree). cmd/squirebot/icon.go re-exports the bytes
// to preserve the historical symbol surface used by main.go.
//
// Two icons ship in the binary: one for the healthy/green state (default
// at startup), one for the red alert state (shown after permanent auth
// failure suspends writes). The tray.Controller swaps between them via
// SetIconHealth. Icon sources: 16x16 BMP-in-ICO solid-color placeholders
// (1118 bytes each), generated to match the byte-for-byte format
// specified by Plan 01-01. Day-1 soak finding 2026-05-03 promoted these
// from the original single magenta placeholder per the deferred Phase 5
// polish note in internal/tray/tray.go.
package assets

import _ "embed"

// IconGreenBytes holds the bytes of assets/icon-green.ico, embedded at
// compile time. Used as the tray icon when the watcher is healthy.
//
//go:embed icon-green.ico
var IconGreenBytes []byte

// IconRedBytes holds the bytes of assets/icon-red.ico, embedded at
// compile time. Used as the tray icon when the watcher has surfaced a
// permanent auth failure (writes suspended; user action required).
//
//go:embed icon-red.ico
var IconRedBytes []byte
