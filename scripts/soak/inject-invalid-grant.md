# Inject: invalid_grant via Google Account console

**Validates:** SC-1 + AUTH-05 — refresh-token-death detection.

## Procedure

1. Sign in to <https://myaccount.google.com> using the same Google account the soak watcher is OAuth'd to.

   > **Safety note:** use a dedicated **throwaway Google account** for the soak. Revoking the OAuth grant of your real account will force a re-auth on every other Google integration tied to it. The runbook in `docs/soak-runbook.md` step 1 expects you to have provisioned this throwaway account and a separate test workbook.

2. Navigate to **Security → Your connections to third-party apps & services**.

3. Find the entry for **SquireBot** (the OAuth client name set in Cloud Console).

4. Click **Details → See details**, then **Remove access**. Confirm the dialog.

5. Wait approximately **2 minutes** for Google's revocation to propagate to the token-introspection endpoint.

6. Trigger a watcher upload by saving an inventory file:
   ```powershell
   # Touch the file's mtime to trigger fsnotify. Adjust the path to wherever
   # the soak install is watching (the EQ folder you picked during setup).
   (Get-Item "$env:LOCALAPPDATA\Soak\Slampeach-Inventory.txt").LastWriteTime = Get-Date
   ```

7. Within ~30 seconds, the watcher attempts the upload. The token refresh fails with `invalid_grant` (or one of the OAuth-spec siblings: `unauthorized_client`, `invalid_client`).

8. Observe the tray (should turn red) and the log:
   ```powershell
   Get-Content -Tail 30 "$env:LOCALAPPDATA\SquireBot\squirebot.log" -Wait
   ```
   Expect: `permanent auth failure — suspending writes` followed by `auth suspended; skipping inventory` (or `... skipping spellbook`) for any subsequent file changes.

9. Click the tray's **Reauthorize…** menu item. Browser opens. Complete the OAuth consent.

10. Verify recovery:
    - Tray returns to green.
    - Log shows `Reauthorize complete`.
    - The next file change (touch the mtime again) uploads normally — log shows an `uploaded` line.
    - `cmdkey /list | findstr SquireBot:` shows the wincred entry (re-stored under the same name; old token replaced).

## Cleanup (post-test)

- The OAuth grant should now be re-issued (you re-consented in step 9). Nothing to roll back.
- The wincred entry is the new refresh token; do NOT delete it (the soak continues).

## Pass criteria

- Log shows `permanent auth failure` AND `auth suspended` lines.
- No silent retry-loop after the suspend trigger (no repeated 401/403 lines).
- Tray transition: green → red within 5 minutes of the revoke.
- Reauthorize click → log shows `Reauthorize start` and `Reauthorize complete`.
- After re-auth: at least one `uploaded` log line for a new file change.
- Tray returns to green; Reauthorize menu item hidden.

## Run the assertion script

```powershell
.\scripts\soak\grep-log-assertions.ps1 -Scenario InvalidGrant
```

Expected output: `OVERALL: PASS InvalidGrant` plus a per-criterion `PASS` line for each grep target.
