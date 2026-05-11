// Package system provides cross-process IPC primitives for the SquireBot
// watcher. Phase 6 (INST-06) introduces a Windows named-event based
// graceful-shutdown channel used by the NSIS pre-install shim to stop a
// running watcher before file overwrite.
//
// Public surface (Windows + stubs on other platforms):
//   - SignalShutdown() error                                — signal side
//   - WaitForShutdown(ctx context.Context) <-chan struct{}  — listener side
//
// The named event uses the Local\ namespace (per-session, not Global) so
// signals from one logon session never bleed into another — matches the
// per-user-installation model locked in Phase 1 (INST-01).
package system
