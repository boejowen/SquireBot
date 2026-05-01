package wizard

// pages.go embeds the three wizard HTML templates. Per Plan 07 Task 1
// the wizard server attaches them to a loopback HTTP listener Plan 03's
// auth.Manager already shares with Plan 06's picker. Single-binary
// installation is preserved — no external file dependencies at runtime.

import "embed"

//go:embed pages/start.html pages/eq-folder.html pages/done.html
var pagesFS embed.FS
