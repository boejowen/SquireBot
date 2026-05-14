---
name: User installs missing toolchains themselves
description: When a build/runtime toolchain is missing (Go, Node, NSIS, etc.), do not attempt to install it — wait for the user to install and signal ready
type: feedback
originSessionId: cb67c961-840b-4188-bcb9-5015f9fc22fc
---
When the user's machine is missing a required toolchain (Go, Node, Python, NSIS, gcloud, clasp, etc.), do NOT attempt to install it via winget/scoop/choco/curl, do NOT write install scripts, and do NOT proceed speculatively. Stop, surface the blocker clearly with install options for reference, and wait for the user to install it themselves and tell you they're done.

**Why:** User explicitly stated this preference on 2026-04-30 when Phase 1 execution hit a missing-Go gate ("I want to install Go 1.24 myself and then tell you when I've done that so you can resume the GSD process"). They want control over what gets installed on their machine and want to manage it through their own package-manager preference.

**How to apply:**
- When a tool isn't found, halt the current GSD step (no partial commits, no speculative file authoring that depends on the tool's output).
- Present the user with the install options (winget / MSI / scoop / brew etc.) as a reference, not as a "should I run this for you?" prompt.
- Tell them you'll wait, and pause cleanly. Update task status to "pending — awaiting user install" so the resume point is obvious.
- When the user signals readiness, verify the tool is on PATH (e.g., `go version`, `node --version`) before resuming, and remind them they may need a fresh shell if the just-installed tool isn't visible yet.
