package main

import "github.com/boejowen/SquireBot/assets"

// iconGreenBytes / iconRedBytes are the embedded tray icons, re-exported
// from the assets package.
//
// Why the indirection: Go's //go:embed directive cannot traverse `..`, so
// the embeds themselves live next to the data files at assets/embed.go.
// main.go (and Plan 07's systray wiring) refer to the local symbols here
// so their identity stays stable across embed-package refactors.
var (
	iconGreenBytes = assets.IconGreenBytes
	iconRedBytes   = assets.IconRedBytes
)
