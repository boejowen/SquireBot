---
phase: 25-linux-watcher
plan: 03
subsystem: watcher (packaging + auto-update — release pipeline, OS-asset selection)
tags: [linux, packaging, tarball, systemd-user-unit, install-sh, auto-update, runtime-goos, release-ci]
requires:
  - "25-01: CGO-free linux/amd64 compile closure (zero systray) + headless *tray.Controller + XDG dirs"
  - "25-02: --setup/--status CLI control + 0600 credstore + WINE eqfind (install.sh's first-run --setup leans on these)"
provides:
  - "OS-specific manifest assets (binary_url_linux/binary_sha256_linux) + Manifest.binaryAsset() runtime.GOOS selection — a linux watcher self-updates with the bare linux squirebot, never the Windows .exe (LNX-05 / Pitfall 3 / T-25-10)"
  - "packaging/linux/ tarball payload: systemd USER unit (Restart=always, WantedBy=default.target), idempotent POSIX install.sh (enable --now + opt-in --linger + first-run --setup), Linux README"
  - "additive release.yml steps: CGO-free linux/amd64 build, squirebot-linux-amd64.tar.gz assembly, bare-linux-binary SHA-256, binary_url_linux/binary_sha256_linux in latest.json, both linux assets uploaded — Windows NSIS path untouched"
affects:
  - "internal/update/manifest.go + check.go (manifest gains 2 linux fields; the 4 bare-binary read sites in checkOnceWithURL route through binaryAsset)"
  - ".github/workflows/release.yml (5 additive steps/extensions; Windows steps unchanged)"
  - ".gitattributes (force LF on packaging/linux/*.sh + *.service)"
tech-stack:
  added: []   # zero new deps — additive build target + a manifest field pair
  patterns:
    - "Manifest.binaryAsset() runtime.GOOS selector for the auto-updater's bare-binary asset (windows -> .exe, else -> bare linux binary); empty selected pair re-uses the existing 'missing binary_url; skip' no-op so a wrong-arch binary is never applied"
    - "additive new-fields-only manifest evolution (omitempty) — old Windows-only latest.json still parses (the locked schema doctrine)"
    - "force-LF .gitattributes for shell/unit tarball payload (a CRLF shebang breaks #!/bin/sh on the target box)"
key-files:
  created:
    - packaging/linux/squirebot.service
    - packaging/linux/install.sh
    - packaging/linux/README.md
  modified:
    - internal/update/manifest.go        # BinaryURLLinux/BinarySHA256Linux fields + binaryAsset() + runtime import
    - internal/update/manifest_test.go   # linux-field parse, legacy back-compat parse, GOOS-selection tests
    - internal/update/check.go           # binURL,binSHA := m.binaryAsset(); 4 read sites routed through it
    - internal/update/check_test.go      # wrong-OS-asset-only no-op test
    - .github/workflows/release.yml      # linux build + tarball + bare-binary hash + linux manifest fields + asset uploads
    - .gitattributes                     # packaging/linux/*.sh + *.service eol=lf
decisions:
  - "Flat new-field manifest form (binary_url_linux/binary_sha256_linux) over a nested per-OS map (RESEARCH A2) — smaller diff, keeps every Phase-1/2 Windows-only manifest parsing, honors the locked new-fields-only schema doctrine."
  - "binaryAsset() returns the linux pair for EVERY non-windows GOOS (not an explicit linux allow-list) — linux is the only other shipped target this milestone; a darwin build (deferred) would harmlessly get the linux fields, which are empty on a windows-only manifest -> skip path, never a wrong binary."
  - "binary_url_linux points at the BARE linux `squirebot` asset, NOT the .tar.gz — selfupdate.Apply swaps a raw executable in place; it cannot unpack a tarball (RESEARCH §3). The tarball is install-only; the bare binary is the updater target. Both are built from the same `go build` output in the same job (T-25-11 accepted)."
  - "install.sh is POSIX #!/bin/sh + set -eu, idempotent; --linger (default OFF) runs loginctl enable-linger ONLY inside the flag branch (T-25-13). First-run setup is gated on `squirebot --status` exit code so re-running never re-prompts."
  - "systemd unit deliberately omits any $HOME-protection directive (no ProtectHome) — the watcher must READ the EQ folder under $HOME (often a WINE prefix); it is a USER unit with NoNewPrivileges=true (no root — T-25-12)."
  - "Forced LF on packaging/linux/*.sh + *.service via .gitattributes (mirroring the existing deploy/* rule) — a CRLF shebang would break #!/bin/sh with 'bad interpreter' on the Linux box; verified the staged install.sh blob is CR=0/LF-only."
metrics:
  duration: "~1 session"
  completed: 2026-06-06
  task-commits: 3
  files-touched: 9
---

# Phase 25 Plan 03: Packaging + Auto-Update Summary

Closed the Linux watcher phase by making the in-app auto-updater OS-aware and shipping a release tarball a guildie can install. The manifest gained additive `binary_url_linux`/`binary_sha256_linux` fields and a `runtime.GOOS`-keyed `binaryAsset()` selector so a Linux box self-updates with the bare linux `squirebot` (never the Windows `.exe` — the Pitfall-3 brick); a `packaging/linux/` payload (systemd USER unit + idempotent `install.sh` + README) was authored; and `release.yml` grew additive CGO-free linux build, tarball-assembly, bare-binary-hash, linux-manifest-field, and asset-upload steps. The Windows NSIS/installer/`.exe` path is byte-unchanged, `go test ./...` stays green on the host, and the linux/amd64 closure is still CGO-free.

## What Was Built

**Task 1 — OS-specific manifest fields + GOOS asset selection (`6e31ec3`)**
- `manifest.go`: added `BinaryURLLinux`/`BinarySHA256Linux` (`json:"...,omitempty"`, additive/new-fields-only) + a `Manifest.binaryAsset() (url, sha string)` method — `runtime.GOOS == "windows"` returns the Windows pair, else the linux pair; added the `runtime` import. The Windows `BinaryURL`/`BinarySHA256` fields + tags are unchanged.
- `check.go`: `binURL, binSHA := m.binaryAsset()` after `IsNewer`; all four bare-binary read sites (the empty-check, the sidecar `EqualFold`, the download URL/req, the verify `EqualFold` + sidecar write) route through `binURL`/`binSHA`. On a linux box reading an old Windows-only manifest the selected pair is empty, so the existing "missing binary_url; skip (manual upgrade only)" no-op fires — a Linux box never downloads the `.exe`.
- Tests: `manifest_test.go` gains a linux-field-parse case, a legacy Windows-only back-compat parse (linux fields decode `""`), and a GOOS-relative `binaryAsset` selection test; `check_test.go` gains a "wrong-OS-asset-only manifest = no-op (no staging)" test.

**Task 2 — Linux tarball payload (`1a3c239`)**
- `packaging/linux/squirebot.service`: systemd USER unit — `Type=simple`, `ExecStart=%h/.local/bin/squirebot`, `Restart=always`, `RestartSec=5`, `NoNewPrivileges=true`, `WantedBy=default.target`; no $HOME-protection (must read the EQ folder under `$HOME`).
- `packaging/linux/install.sh`: POSIX `#!/bin/sh` + `set -eu`, idempotent — `install -Dm755` the binary to `~/.local/bin`, `install -Dm644` the unit to `~/.config/systemd/user`, `systemctl --user daemon-reload`, first-run `--setup` only when `--status` reports unconfigured, `systemctl --user enable --now`. Opt-in `--linger` (default OFF) runs `loginctl enable-linger "$USER"` inside the flag branch with an explanatory echo.
- `packaging/linux/README.md`: install (`install.sh` / `--linger`), `--setup`/`--status`, `journalctl --user -u squirebot`, XDG paths table, the automatic auto-update note, and the WINE EQ-folder note.
- `.gitattributes`: force LF on `packaging/linux/*.sh` + `*.service`.

**Task 3 — Additive linux build/tarball/manifest in release.yml (`5b09a4f`)**
- New "Build bare squirebot (linux/amd64, CGO-free)" step: `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.Version=... -X main.BackendBaseURL=..." -o dist/squirebot ./cmd/squirebot` (no `-H=windowsgui`).
- New "Assemble Linux tarball" step: stages `squirebot` + `README.md` + `squirebot.service` + `install.sh` into `dist/squirebot-linux-amd64/` and `tar -czf`s `dist/squirebot-linux-amd64.tar.gz`.
- "Compute SHA-256 sums" extended to hash the BARE `dist/squirebot` → `binary_sha256_linux` (the updater swaps the bare binary, not the tarball).
- "Write latest.json" `$manifest` extended with `binary_url_linux` (→ the bare `squirebot` asset) + `binary_sha256_linux`; Windows `installer_*`/`binary_*` fields untouched.
- Both linux assets (`dist/squirebot`, `dist/squirebot-linux-amd64.tar.gz`) added to the upload-artifact and GitHub-Release file lists; release body gains a Linux install blurb. NSIS/installer/`.exe` steps unchanged.

## Verification Results

| Gate | Result |
|------|--------|
| `go test ./internal/update/...` (Windows host) | PASS — incl. linux-field parse, legacy back-compat, GOOS selection, wrong-OS no-op |
| `GOOS=linux go vet ./internal/update/...` | PASS (exit 0) |
| `grep 'm.BinaryURL\b\|m.BinarySHA256\b' check.go` (download/verify path) | **0** — all routed through `binaryAsset` |
| `sh -n packaging/linux/install.sh` | exit 0 (POSIX clean) |
| `grep -c 'ProtectHome' packaging/linux/squirebot.service` | **0** |
| `grep 'WantedBy=default.target' / 'Restart=always'` (unit) | both match |
| `grep 'systemctl --user enable --now' / '--linger' / '--setup' / 'enable-linger'` (install.sh) | each match; `enable-linger` is INSIDE the `--linger` flag branch |
| staged `install.sh` blob line endings | CR=0, LF-only (force-LF .gitattributes verified) |
| `release.yml` YAML parse (yaml.v3) | YAML_VALID |
| `grep 'GOOS: linux' / 'squirebot-linux-amd64.tar.gz' / 'binary_url_linux' / 'binary_sha256_linux'` (release.yml) | all present (build step, tarball step, manifest, asset lists) |
| Windows path intact | `squirebot.exe` ×12, `makensis` ×12, windows `binary_url` (.exe) line present, `-H=windowsgui` ×2 |
| Cross-compile build | `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath ... -o squirebot ./cmd/squirebot` → static **ELF** (magic `\x7fELF`) |
| `go list -deps ./cmd/squirebot` (linux) systray / sqweek count | **0 / 0** (still CGO-free) |
| `go test ./...` (Windows host, whole module) | PASS — 0 failures (additive guarantee) |
| `GOOS=windows ... go build ./cmd/squirebot` | PASS (Windows watcher still compiles) |

## Test Verification: Compile- vs Run-Verified (Windows dev box)

The new `internal/update` tests are platform-agnostic Go and RUN on the Windows host (no ELF execution involved — they hit `httptest.Server` + struct methods), so they are **run-verified** here. The GOOS-relative tests assert the windows branch of `binaryAsset` on the windows box and the symmetric "other-platform-only manifest = no-op" invariant; the linux branch runs-proves on a linux runner. `install.sh` was **`sh -n` syntax-verified** (git-bash on the dev box); its live `systemctl --user`/`loginctl` behavior is a human UAT on a real Linux box (no `systemd` here — per the environment note). The cross-compiled binary was **build-verified as a static ELF** and the closure **proven CGO-free**.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing critical functionality] Force LF on the packaging/linux shell + unit files**
- **Found during:** Task 2 (staging)
- **Issue:** The dev box's autocrlf would rewrite `install.sh` + `squirebot.service` LF→CRLF on commit. A CRLF shebang (`#!/bin/sh\r`) breaks with "bad interpreter" on the target Linux box, and systemd units prefer LF — the tarball would ship broken files. The plan's task list named only the three packaging files, but correct operation requires the line-ending guarantee.
- **Fix:** Added `packaging/linux/*.sh text eol=lf` + `packaging/linux/*.service text eol=lf` to `.gitattributes` (mirroring the existing `deploy/*` rule), re-staged, and verified the staged `install.sh` blob is CR=0 / LF-only.
- **Files modified:** `.gitattributes`
- **Commit:** `1a3c239`

### Comment-only adjustment for the grep gate
The systemd unit's hardening comment originally contained the literal word `ProtectHome` (explaining why it is NOT set), which tripped the `grep -c 'ProtectHome' == 0` acceptance gate. Reworded to "we deliberately do NOT sandbox $HOME … any home-protection directive would break ingestion" — same documented rationale, no `ProtectHome` substring, no behavior change. (Same comment-rewording pattern 25-01/25-02 used for their grep gates.)

## Threat Mitigations Applied

- **T-25-10 (Tampering — wrong-arch/forged auto-update binary):** `binaryAsset()` selects the bare linux `squirebot` by `runtime.GOOS` so a Linux box only ever fetches the linux asset; a linux box reading a windows-only manifest gets `("","")` → the existing skip path fires (never downloads the `.exe`). The mandatory SHA-256 verify in `check.go` (now keyed on `binSHA`) + `swap.go` is unchanged and still gates the swap — a hash mismatch aborts before `Apply`. Proven by `TestCheckOnce_WrongOSAssetOnlyIsNoop` + `TestManifest_BinaryAssetSelectsByGOOS`.
- **T-25-11 (Integrity — tarball binary vs auto-update binary):** ACCEPTED — both the tarball's `squirebot` and the bare `squirebot` asset come from the SAME `go build` output in the same job; the manifest hashes the bare binary (the updater's target). The tarball is install-only.
- **T-25-12 (Elevation — systemd unit privilege):** USER unit (no root), `NoNewPrivileges=true`, no `ProtectHome` only because the EQ-folder read needs `$HOME` access (documented trade-off).
- **T-25-13 (Tampering — unexpected 24/7 linger):** `loginctl enable-linger` runs ONLY inside the explicit `--linger` branch (default OFF); README documents the trade-off.
- **T-25-14 (Repudiation — re-tagged release hash drift):** ACCEPTED — carried known issue; `latest.json` is regenerated each run with current SHAs, so the auto-updater is always correct (the linux fields ride that same regeneration).

## No Stubs / No New Threat Surface

This plan creates no UI-facing stubs. The release pipeline introduces a new GitHub-Release asset → auto-updater download surface, but it is already in the plan's `<threat_model>` (T-25-10/11) and is mitigated by the GOOS asset selection + the unchanged mandatory SHA-256 verify. No `## Known Stubs` or `## Threat Flags` needed.

## Phase 25 Closeout

This is the FINAL plan of Phase 25 (3/3). The Linux watcher is now built (CGO-free static ELF, 25-01), functionally complete (credstore / eqfind / onboarding / CLI, 25-02), and packaged + self-updating (tarball + systemd user unit + install.sh + OS-aware auto-update, 25-03). All six requirements (LNX-01..06) are code-complete. The remaining confirmation is a human UAT on a real Linux+WINE box ("watches the WINE EQ folder + uploads + autostarts + self-updates"), exactly as the Windows watcher's on-machine UATs — the deliverable from this phase is the built, unit-tested, packaged tarball.

## Self-Check: PASSED

All 3 created files (`packaging/linux/{squirebot.service,install.sh,README.md}`) verified present on disk; all 3 task commits (`6e31ec3`, `1a3c239`, `5b09a4f`) verified in git history.
