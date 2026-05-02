# Build and Install (SquireBot, Phase 1)

End-to-end runbook for building `squirebot.exe`, packaging it with the NSIS
per-user installer, and installing the resulting `SquireBot-Setup-X.Y.Z.exe`
on a clean Windows 11 VM. This is the local equivalent of the GitHub
Actions release pipeline (`.github/workflows/release.yml`); both should
produce byte-identical installer behaviour.

Phase 1 ships **unsigned** per [01-CONTEXT.md D-13](../.planning/phases/01-end-to-end-thin-slice/01-CONTEXT.md).
First-run on any machine that has never seen SquireBot will hit Microsoft
Defender SmartScreen with an "Unknown publisher" wall. The
["More info -> Run anyway" walkthrough](#smartscreen-more-info--run-anyway-walkthrough-d-13)
below is the documented Phase 1 install path. Phase 2 adds code signing.

---

## Prerequisites

| Tool | Minimum | Verify |
|------|---------|--------|
| Go | 1.24 | `go version` |
| NSIS | 3.10 | `makensis /VERSION` (must report `v3.10` or later -- 3.12 confirmed working) |
| oauth-config.json | populated, `consent_screen_status: "PRODUCTION"` | `Get-Content .planning/phases/01-end-to-end-thin-slice/oauth-config.json` |
| PowerShell | 7+ (recommended) or built-in 5.1 | `$PSVersionTable.PSVersion` |

`oauth-config.json` is gitignored and lives at
`.planning/phases/01-end-to-end-thin-slice/oauth-config.json`. It is populated
by Plan 01-02; see [docs/oauth-setup.md](./oauth-setup.md) for the
provisioning runbook. The `consent_screen_status` field MUST be `PRODUCTION`
before any release tag is cut -- otherwise refresh tokens will silently
expire after 7 days (Pitfall #1, RESEARCH.md §1).

If you don't have NSIS yet:
- **Scoop:** `scoop install nsis` (installs to `%USERPROFILE%\scoop\apps\nsis\current\`).
- **Chocolatey:** `choco install nsis`.
- **Manual:** download from <https://nsis.sourceforge.io/Download> and put `makensis.exe` on PATH.

The user installs missing toolchains themselves; the build script does not
auto-install. If `makensis` is not on PATH, the local build will fail fast.

---

## Building the binary

### Bash one-liner (Git Bash, WSL, mingw)

```bash
# From repo root.
OAUTH_CLIENT_ID=$(jq -r     '.oauth_client_id'      .planning/phases/01-end-to-end-thin-slice/oauth-config.json)
OAUTH_CLIENT_SECRET=$(jq -r '.oauth_client_secret'  .planning/phases/01-end-to-end-thin-slice/oauth-config.json)
PICKER_API_KEY=$(jq -r      '.picker_api_key'       .planning/phases/01-end-to-end-thin-slice/oauth-config.json)
GCP_PROJECT_NUMBER=$(jq -r  '.gcp_project_number'   .planning/phases/01-end-to-end-thin-slice/oauth-config.json)
VERSION="0.1.0"

GOOS=windows GOARCH=amd64 go build \
  -ldflags="-H=windowsgui -s -w \
            -X main.OAuthClientID=${OAUTH_CLIENT_ID} \
            -X main.OAuthClientSecret=${OAUTH_CLIENT_SECRET} \
            -X main.PickerAPIKey=${PICKER_API_KEY} \
            -X main.GCPProjectNumber=${GCP_PROJECT_NUMBER} \
            -X main.Version=${VERSION}" \
  -o dist/squirebot.exe ./cmd/squirebot
```

`-H=windowsgui` suppresses the console window so the watcher runs as a
background tray app. `-s -w` strips DWARF debug info to keep the binary
under ~17 MiB.

### PowerShell variant (Windows-native)

```powershell
# From repo root.
$cfg                  = Get-Content .planning/phases/01-end-to-end-thin-slice/oauth-config.json -Raw | ConvertFrom-Json
$OAUTH_CLIENT_ID      = $cfg.oauth_client_id
$OAUTH_CLIENT_SECRET  = $cfg.oauth_client_secret
$PICKER_API_KEY       = $cfg.picker_api_key
$GCP_PROJECT_NUMBER   = $cfg.gcp_project_number
$VERSION              = "0.1.0"

if ($cfg.consent_screen_status -ne "PRODUCTION") {
    Write-Error "oauth-config.json consent_screen_status is '$($cfg.consent_screen_status)' -- must be 'PRODUCTION' before building a release. See docs/oauth-setup.md."
    exit 1
}

New-Item -ItemType Directory -Force -Path dist | Out-Null
$env:GOOS   = "windows"
$env:GOARCH = "amd64"
go build -ldflags="-H=windowsgui -s -w `
  -X main.OAuthClientID=$OAUTH_CLIENT_ID `
  -X main.OAuthClientSecret=$OAUTH_CLIENT_SECRET `
  -X main.PickerAPIKey=$PICKER_API_KEY `
  -X main.GCPProjectNumber=$GCP_PROJECT_NUMBER `
  -X main.Version=$VERSION" `
  -o dist/squirebot.exe ./cmd/squirebot
```

### About the client secret

> Despite the name, `oauth_client_secret` is effectively public for desktop
> apps -- it ships in every binary on GitHub Releases. Google still
> requires it as a token-endpoint parameter even with PKCE in use; per
> Google's docs, "When a client runs on a device, the client_secret is no
> longer truly confidential." We treat it the same way as
> `picker_api_key`: bake it in via `-ldflags`, do not persist to disk
> anywhere else, and rely on the API restrictions (Picker API only,
> `drive.file` scope only) to bound blast radius. Do NOT add this value
> to the watcher's wincred entry, the on-disk `config.json`, or any log
> line.

### Verification

```powershell
Get-Item dist/squirebot.exe | Select-Object Name, Length
# Expected: ~16-17 MB (16,864,256 bytes for 0.1.0).
```

If the binary is much smaller (under ~10 MB) the ldflags didn't apply --
inspect the build log for `-X` warnings.

---

## Building the installer

```bash
# From repo root.
makensis -DAPPVERSION=0.1.0 -V2 installer/squirebot.nsi
# Produces dist/SquireBot-Setup-0.1.0.exe (~6-8 MB after LZMA compression).
```

**Why `-V2`:** verbosity level 2 prints warnings + errors only. Use `-V4`
when debugging (loud) or `-V0` for fully silent CI logs.

**Why `-DAPPVERSION=`:** overrides the `!define APPVERSION "0.1.0"` default
in `squirebot.nsi`. CI passes `-DAPPVERSION=$( $tag -replace '^v','' )`.

The installer expects:
- `dist/squirebot.exe` (built above) -- referenced as `..\dist\${EXE_NAME}`.
- `installer/icon.ico` -- referenced as `icon.ico` (cwd is `installer/`
  during makensis evaluation; the file is the same as `assets/icon.ico`).

---

## Local snapshot build (developers)

For a reproducible build matching the CI output, install
[goreleaser](https://goreleaser.com/install/) v2 and run:

```powershell
# Required env vars (substitute real values from
# .planning/phases/01-end-to-end-thin-slice/oauth-config.json).
$env:OAUTH_CLIENT_ID     = "..."
$env:OAUTH_CLIENT_SECRET = "..."
$env:PICKER_API_KEY      = "..."
$env:GCP_PROJECT_NUMBER  = "..."

goreleaser build --snapshot --clean
```

Output lands under `dist/squirebot_windows_amd64_v1/squirebot.exe`. To
also produce the NSIS installer locally, mirror the CI sequence:

```powershell
Copy-Item -Force dist\squirebot_windows_amd64_v1\squirebot.exe dist\squirebot.exe
makensis -V2 -DAPPVERSION=0.2.0-dev installer\squirebot.nsi
```

CI does NOT use goreleaser -- the workflow at
`.github/workflows/release.yml` does explicit `go build` + `makensis` +
manifest steps and uploads via `softprops/action-gh-release`. The
`.goreleaser.yaml` at the repo root exists for local-dev parity only;
its `release:` / `signs:` / `archives:` / `changelog:` blocks are
intentionally absent because CI does not invoke them.

---

## Installing on a clean Windows 11 VM (manual smoke)

This procedure is what gets executed by Plan 08 Task 3 to validate Phase 1
success criteria 1-4. The full step-by-step checklist lives at
`.planning/phases/01-end-to-end-thin-slice/smoke-checklist.md` -- copy it
to the VM and check off each step as you go.

1. **Provision a clean Win11 VM.** Hyper-V, VMware, VirtualBox, Parallels,
   or a reformatted spare laptop -- anything that has never seen
   `squirebot.exe`. Sign in as a **non-admin** user. Admin users see no UAC
   prompts even for things that would normally trigger them, so admin
   smoke runs do not validate INST-01.
2. **Copy `dist/SquireBot-Setup-0.1.0.exe` to the VM.** USB stick, OneDrive,
   `scp`, RDP clipboard -- whichever your VM workflow supports.
3. **Double-click the installer.** Expect SmartScreen "Unknown publisher"
   wall (D-13). Walk through the
   ["More info -> Run anyway"](#smartscreen-more-info--run-anyway-walkthrough-d-13)
   sequence below.
4. **NSIS wizard opens.** Verify NO additional UAC prompt fires -- the
   `RequestExecutionLevel user` directive should suppress it. If a UAC
   prompt fires, the install is broken (record as SC1 FAIL).
5. **Click Install.** Files land in `%LOCALAPPDATA%\Programs\SquireBot\`.
   Verify in PowerShell:
   ```powershell
   Get-ChildItem $env:LOCALAPPDATA\Programs\SquireBot
   # Expected: squirebot.exe, icon.ico, uninstall.exe
   ```
6. **`squirebot.exe` auto-launches.** Tray icon appears (system tray,
   bottom-right). Default browser opens to
   `http://127.0.0.1:<random-port>/start`.
7. **Verify HKCU registration:**
   ```powershell
   Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\SquireBot'
   # Expected fields: DisplayName=SquireBot, DisplayVersion=0.1.0,
   # InstallLocation=...\Programs\SquireBot, UninstallString=...\uninstall.exe
   ```
8. **Walk the OAuth + Picker + EQ-folder wizard.** Drop a
   `<TestChar>-Inventory.txt` file into the chosen EQ folder; within 30 s
   the `inv:<TestChar>` tab should appear in the picked workbook.
9. **Verify wincred token storage (SC4):**
   ```powershell
   cmdkey /list | Select-String SquireBot
   # Expected: Target: SquireBot:<your-email>
   Get-Content $env:LOCALAPPDATA\SquireBot\config.json
   # Expected: google_email is present; refresh_token / access_token / client_secret are NOT.
   ```

---

## Uninstalling

Either path works (both invoke `uninstall.exe`):

- **Settings -> Apps -> Installed apps -> SquireBot -> Uninstall** (uses
  the HKCU `UninstallString` value).
- **Run `%LOCALAPPDATA%\Programs\SquireBot\uninstall.exe` directly.**

The uninstaller deletes:
- `%LOCALAPPDATA%\Programs\SquireBot\` (the binary, icon, uninstaller)
- `%LOCALAPPDATA%\SquireBot\config.json`
- `%LOCALAPPDATA%\SquireBot\squirebot.log*`
- The HKCU Uninstall registry subkey

It deliberately does **not** delete the wincred refresh-token entry; a
re-install reuses the cached token and skips the OAuth round trip. To wipe
the token manually:

```powershell
cmdkey /list | Select-String SquireBot       # find target name
cmdkey /delete:SquireBot:<email>             # delete it
```

---

## SmartScreen "More info -> Run anyway" walkthrough (D-13 Phase 1 documented path)

Phase 1 binaries are unsigned. On any machine that has never seen this
specific binary hash, Microsoft Defender SmartScreen presents a blue
"Microsoft Defender SmartScreen prevented an unrecognized app from
starting" wall when the installer is double-clicked.

The walkthrough:

1. Double-click `SquireBot-Setup-0.1.0.exe`.
2. SmartScreen wall appears with two visible buttons: "Don't run" and
   (depending on Win11 build) a small text link "More info" near the top.
3. Click **More info**. The dialog expands to show:
   - "Publisher: Unknown publisher"
   - "App: SquireBot-Setup-0.1.0.exe"
   - A new button: **Run anyway**.
4. Click **Run anyway**. The NSIS installer wizard opens.

Total elapsed time: 5-15 seconds. This is the documented Phase 1 path.
Phase 2 ships a code-signed binary so this wall disappears for guildies.

If "More info" does not appear: the local Defender policy may be set to
"Block" rather than "Warn". Check Settings -> Privacy & security ->
Windows Security -> App & browser control -> Reputation-based protection
settings -> "Check apps and files" should be "Warn" not "Block". If it's
"Block", the user must temporarily switch it to "Warn" or "Off" -- in
which case Phase 2 code signing is the right long-term fix.

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `makensis` exits with `error opening "..\dist\squirebot.exe"` | binary not built | run the Build the binary step first |
| Installer shows UAC prompt | `RequestExecutionLevel user` removed or filename heuristic still fired | re-add the directive; verify `grep -n 'RequestExecutionLevel user' installer/squirebot.nsi` finds line |
| Installer writes to `Program Files` | `InstallDir` typo | confirm `$LOCALAPPDATA\Programs\${APPNAME}` literal in `.nsi` |
| Tray icon never appears post-install | binary built without `-H=windowsgui`, console window stole focus | rebuild with the documented ldflags |
| Browser does not auto-open | binary missing one of the four OAuth ldflags (OAuthClientID / OAuthClientSecret / PickerAPIKey / GCPProjectNumber), falls into ErrMissingConstants and refuses to start the OAuth flow | rebuild; verify `dist/squirebot.exe` size is ~17 MB not <10 MB |
| `oauth2: invalid_request: client_secret is missing` on token exchange | binary missing the `OAuthClientSecret` ldflag specifically (Google's token endpoint requires it even for desktop PKCE clients) | rebuild with `-X main.OAuthClientSecret=$($cfg.oauth_client_secret)` per the build commands above |
| `inv:<Char>` tab does not appear within 30 s | watcher write contract failure | `Get-Content $env:LOCALAPPDATA\SquireBot\squirebot.log -Tail 50` for the slog.Error |
| `cmdkey /list` shows no SquireBot entry after OAuth | `auth.StoreToken` failed silently | check log for `wincred write failed` |

---

## Release artifacts

Each tagged release on GitHub Releases publishes three files:

| File | Purpose |
|------|---------|
| `SquireBot-Setup-<version>.exe` | NSIS installer for new guildies -- runs the wizard, sets up autostart, installs the watcher. |
| `squirebot.exe` *(bare binary)* | Used by the in-app auto-updater (OPS-04 / Plan 02-06). The `internal/update` package consumes this via `binary_url` in `latest.json`; end users do NOT download this file directly. |
| `latest.json` | Manifest the auto-updater fetches every 24h to detect new versions. Schema: `{ version, installer_url, installer_sha256, binary_url, binary_sha256, released_at, phase, signed }`. |

SHA-256 sums for both `.exe` artifacts are listed in the GitHub Release
body and embedded in `latest.json`.

The `latest.json` schema evolved from Phase 1 (which only had
`installer_url` + `installer_sha256`) to Phase 2 by adding `binary_url`
and `binary_sha256`. Older v0.1.x manifests still parse cleanly; the
auto-updater treats absent `binary_url` as "installer-only release"
and falls back to a manual upgrade prompt.

---

## CI parity

The GitHub Actions release pipeline (`.github/workflows/release.yml`) is
the authoritative producer of release artifacts. It:

1. Materialises `oauth-config.json` from the `OAUTH_CONFIG_JSON` repo
   secret (same shape the local file has).
2. Refuses to proceed if `consent_screen_status != "PRODUCTION"` --
   AUTH-03 gate, preserved verbatim from Phase 1.
3. Runs the same `go build` invocation documented above with `-X` ldflags
   from the secret.
4. Runs `makensis -DAPPVERSION=$tag -V2 installer/squirebot.nsi`.
5. Computes SHA-256 of BOTH the installer AND the bare binary; writes
   `dist/latest.json` with `installer_url`, `installer_sha256`,
   `binary_url`, `binary_sha256`, `released_at`, `phase`, `signed`.
6. Uploads three files (`SquireBot-Setup-<version>.exe`, `squirebot.exe`,
   `latest.json`) as both build artifacts and -- on tag push -- a GitHub
   Release via `softprops/action-gh-release`.

If your local build behaves differently from a CI tag-push build, that is
a regression. Most likely root causes: (a) different `oauth-config.json`
contents, (b) different NSIS version, (c) different Go toolchain version
(CI uses `go-version: '1.24'`).
