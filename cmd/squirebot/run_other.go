//go:build !windows

package main

import (
	"context"
	"log/slog"
	"os/signal"
	"syscall"

	"github.com/boejowen/SquireBot/internal/tray"
)

// runMainLoop is the headless (Linux) blocking tail. There is NO systray on the
// !windows build (D-01), so it has no menu loop to run — it simply blocks the
// main goroutine until the run context is cancelled, then returns so main()
// exits.
//
// MANDATORY SIGINT/SIGTERM handler (LNX-05): on Linux the watcher runs as a
// systemd user service. `systemctl --user stop` delivers SIGTERM; a clean
// stop / `Restart=always` relaunch / the selfupdate-restart path all depend on
// that SIGTERM unwinding the run context — NOT the process being SIGKILL'd.
//
// internal/system.WaitForShutdown(ctx) on !windows watches ONLY ctx.Done() — it
// registers NO OS-signal handler (verified in shutdown_signal_other.go). So
// without the handler installed below, a SIGTERM would never reach cancel() and
// the daemon would be hard-killed mid-ingest. signal.NotifyContext derives a
// context that is cancelled on SIGINT/SIGTERM; a goroutine then drives the root
// cancel(). The trayCtl pointer is unused here (headless) but kept in the
// signature so main.go's call site is platform-neutral.
func runMainLoop(ctx context.Context, cancel context.CancelFunc, _ *tray.Controller) {
	sigCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-sigCtx.Done()
		// Either a SIGINT/SIGTERM was delivered (systemd stop / Ctrl-C) or the
		// parent ctx was already cancelled (tray-less shutdown from another
		// path). Either way, drive the root cancel() so RunApp unwinds.
		slog.Info("shutdown signal received — cancelling root context")
		cancel()
	}()

	// Block the main goroutine until the run context is cancelled (by the
	// signal handler above, or by any other cancel() caller).
	<-ctx.Done()
}
