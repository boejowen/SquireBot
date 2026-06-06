//go:build windows

package main

import (
	"context"

	"fyne.io/systray"

	"github.com/boejowen/SquireBot/internal/system"
	"github.com/boejowen/SquireBot/internal/tray"
)

// runMainLoop is the Windows blocking tail extracted from main.go. Behavior is
// byte-for-byte equivalent to the pre-Phase-25 main.go tail (D-07: Windows
// unchanged):
//
//   - A listener goroutine blocks on the Local\SquireBot-Shutdown named event
//     (system.WaitForShutdown). On signal it cancels the root ctx and calls
//     systray.Quit() to unblock systray.Run. It also exits on ctx.Done() so it
//     cannot leak when shutdown comes from another path (tray Quit).
//   - systray.Run(trayCtl.OnReady, trayCtl.OnExit) then blocks the main
//     goroutine until systray.Quit fires.
//   - On return, cancel() tears down background work.
//
// No drain coordination: in-flight backend ingest POSTs observe ctx cancellation
// through the http.Client request context and abandon; WATCH-09 catch-up
// re-uploads any missed file changes on next launch.
func runMainLoop(ctx context.Context, cancel context.CancelFunc, trayCtl *tray.Controller) {
	go func() {
		select {
		case <-system.WaitForShutdown(ctx):
			cancel()
			systray.Quit()
		case <-ctx.Done():
			// Normal shutdown from another path (tray Quit). WaitForShutdown's
			// internal goroutine also observes ctx.Done and cleans up its event
			// handle via defer.
			return
		}
	}()

	// Main goroutine: systray.Run blocks until systray.Quit fires.
	systray.Run(trayCtl.OnReady, trayCtl.OnExit)

	// Tray quit → tear down background work.
	cancel()
}
