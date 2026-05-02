# Inject: corrupted auto-update payload

**Validates:** SC-1 + SC-5 — auto-update SHA-256 rejection of corrupted .new files; no broken install state.

## Why this approach

The auto-updater (Plan 02-06) verifies the staged `.new` against an `.expected-sha256` sidecar at startup, BEFORE invoking `minio/selfupdate.Apply`. The unit tests in `internal/update/swap_test.go` already exercise the hash-mismatch path. This live test proves the same behavior end-to-end on the user's actual install.

Two ways to inject corruption:

- **Option A (recommended; simpler):** directly write a garbage `.new` + a sidecar with a wrong hash next to the binary, then restart. No HTTP server required. This tests the post-download swap path.
- **Option B (advanced; full pipeline):** stand up a local HTTP server with a tampered `latest.json` whose `binary_sha256` doesn't match the `binary_url` payload, then point the watcher at it. This tests the download-time SHA verification AND the swap-time SHA verification. Procedure C is documented at the bottom for completeness; most users only need Option A.

Both are documented below.

## Procedure (Option A — Direct corrupt-payload injection)

1. Stop the running SquireBot. Tray → Quit.

2. Locate the binary:
   ```powershell
   $exe = "$env:LOCALAPPDATA\Programs\SquireBot\squirebot.exe"
   ```

3. Stage a corrupted .new file. The "corruption" is just bytes that don't match the sidecar SHA — easiest is to write garbage:
   ```powershell
   $newPath  = "$exe.new"
   $hashPath = "$exe.expected-sha256"
   # Write 1024 bytes of garbage to .new
   [byte[]]$garbage = (1..1024)
   [System.IO.File]::WriteAllBytes($newPath, $garbage)
   # Write a sidecar hash that does NOT match the garbage (any 64-hex string works)
   Set-Content -Path $hashPath -Value "0000000000000000000000000000000000000000000000000000000000000000" -NoNewline -Encoding ASCII
   ```

4. Restart SquireBot:
   ```powershell
   & $exe
   ```
   (Or simply sign out + sign back in, which fires the autostart.)

5. The startup-swap routine in `update.Apply` runs early. It computes the SHA-256 of the .new file, compares against the sidecar `0000...`, sees a mismatch, and deletes BOTH .new and the sidecar.

6. The OLD binary continues normally; you should see the tray icon appear and the watcher start.

7. Verify cleanup:
   ```powershell
   Test-Path "$exe.new"
   Test-Path "$exe.expected-sha256"
   ```
   Both should return `False`.

8. Verify log:
   ```powershell
   Get-Content -Tail 30 "$env:LOCALAPPDATA\SquireBot\squirebot.log"
   ```
   Expect (early in the log): `staged hash mismatch: have <real-hash>, want 0000...`.

9. The 24h auto-update goroutine continues to run; if a real new version exists upstream, the next legitimate cycle will re-stage cleanly.

## Pass criteria

- The .new file and sidecar are both deleted after startup.
- The log shows `staged hash mismatch`.
- The watcher continues running normally on the OLD binary.
- No tray transition to red (corrupt update is not user-actionable).

## Run the assertion script

```powershell
.\scripts\soak\grep-log-assertions.ps1 -Scenario CorruptUpdate
```

---

## Option B — Full-pipeline injection via local HTTP server (advanced; optional)

Use this only if you specifically want to exercise the download-time SHA-256 check. Most validation is satisfied by Option A.

1. Build a tampered `latest.json`. Take the real release manifest (e.g., from <https://github.com/boejowen/SquireBot/releases/latest/download/latest.json>), bump the `version` to something newer than the running binary, and corrupt the `binary_sha256` field (flip one hex digit).

2. Save it locally as `dist/latest.json`. Also save a copy of the bare `squirebot.exe` from the same release as `dist/squirebot.exe` (UNMODIFIED — the SHA mismatch is between the file and the manifest, not the file itself).

3. Serve the dist directory on localhost:
   ```powershell
   # From repo root, in a separate terminal:
   cd dist
   python -m http.server 8765
   # Or: go run github.com/m3ng9i/ran@latest -p 8765 -r .
   ```

4. Temporarily redirect the watcher's manifest URL via DNS or hosts-file shim. This is INVASIVE — the cleaner approach is to add a temporary debug build of the watcher with a hard-coded local manifest URL, run it, observe the rejection, then restore. Given the complexity, prefer Option A unless you specifically need the download-time path covered.

5. Expected log lines on the watcher's next 24h auto-update tick:
   - `auto-update: staged` does NOT appear (the SHA check failed before staging).
   - An `auto-update check failed` warning DOES appear with the wrapping error containing `mismatch`.
   - The `.new.tmp` file is cleaned up; no `.new` is renamed into place.
   - On the next cold start, `update.Apply` finds no `.new` and proceeds normally (no swap).

6. After the test: stop the local HTTP server, restore the manifest URL, restart SquireBot.

This option is informational; the assertion script (`grep-log-assertions.ps1 -Scenario CorruptUpdate`) targets Option A's signals (`staged hash mismatch` line + `.new`/`.expected-sha256` cleanup), which are the canonical path.
