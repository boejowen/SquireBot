//go:build !windows

package system

import "context"

// SignalShutdown is a no-op on non-Windows platforms. The Windows
// implementation lives in shutdown_signal_windows.go.
func SignalShutdown() error { return nil }

// WaitForShutdown returns a channel that closes only when ctx is
// cancelled. The Windows implementation in shutdown_signal_windows.go
// also closes the channel when the Local\SquireBot-Shutdown named event
// fires.
func WaitForShutdown(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		close(done)
	}()
	return done
}
