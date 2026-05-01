// Package assets bundles SquireBot's runtime resources (icon, etc.) into the
// final binary so the .exe is self-contained.
//
// The package lives under assets/ rather than cmd/squirebot/ because Go's
// embed directive cannot reach files via `..` (the pattern is confined to
// the package's own subtree). cmd/squirebot/icon.go re-exports IconBytes
// to preserve the historical iconBytes symbol used in the plan interfaces.
package assets

import _ "embed"

// IconBytes holds the bytes of assets/icon.ico, embedded at compile time.
//
//go:embed icon.ico
var IconBytes []byte
