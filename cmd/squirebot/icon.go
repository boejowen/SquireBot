package main

import "github.com/boejowen/SquireBot/assets"

// iconBytes is the embedded tray icon, re-exported from the assets package.
//
// Why the indirection: Go's //go:embed directive cannot traverse `..`, so the
// embed itself lives next to the file at assets/embed.go. main.go (and Plan 07's
// systray wiring) refer to iconBytes here so the symbol remains stable.
var iconBytes = assets.IconBytes
