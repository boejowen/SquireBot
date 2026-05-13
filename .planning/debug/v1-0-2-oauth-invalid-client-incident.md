---
status: investigating
slug: v1-0-2-oauth-invalid-client-incident
opened: 2026-05-13T00:50:00Z
updated: 2026-05-13T00:50:00Z
hypothesis: OAuth client deleted or quarantined in Google Cloud Console (project 262087828393) between v1.0.1 ship (2026-05-11 17:39Z) and v1.0.2 ship (2026-05-13 00:30Z)
severity: critical
impact: every v1.0.1 and v1.0.2 watcher in the guild dies on next token refresh — guild-wide OAuth wall
---

# v1.0.2 invalid_client OAuth incident

## Symptom

User reports ~4-5 Google OAuth browser prompts in the ~12-minute window after v1.0.2 published, all showing:

> Access blocked: Authorization error
> The OAuth client was not found.
> Error 401: invalid_client

## Diagnostic — build pipeline is innocent

Phase 9 source diff against pre-Phase-9 baseline `4450f9a..839a41e` touched:
- `cmd/squirebot/main.go`, `console_windows.go`, `console_other.go` — FreeConsole detach only
- `internal/app/runapp.go` — `applyBootAuthError` AUTH-07 helper; uses existing `auth.IsRevokedRefreshToken` from internal/auth
- `internal/config/config.go` — `bytes.TrimPrefix` BOM strip
- `internal/tray/tray.go` — pre-Ready queue
- `internal/app/runapp_test.go`, `internal/config/config_test.go`, `internal/tray/tray_test.go` — tests only

**No production OAuth code was modified.** Only test-fixture reference to `oauth2.RetrieveError{ErrorCode: "invalid_grant"}` for `TestApplyBootAuthError_Revoked`.

## Diagnostic — client_id baked into binaries

Extracted via `grep -aoE "[0-9]+-[a-z0-9]+\.apps\.googleusercontent\.com"` on the shipped binaries downloaded from `github.com/boejowen/SquireBot/releases`:

| Release | client_id baked into squirebot.exe |
|---------|------------------------------------|
| v1.0.1 (2026-05-11, worked) | `262087828393-8obvbca97eb1q73f2kna7nef4adhq7bu.apps.googleusercontent.com` |
| v1.0.2 (2026-05-13, fails)  | `262087828393-8obvbca97eb1q73f2kna7nef4adhq7bu.apps.googleusercontent.com` |

**Identical.** Same client_secret (suffix `…ZCXH`, full value redacted from this file but available in the gitignored `.planning/phases/01-end-to-end-thin-slice/oauth-config.json` and extractable from the shipped binaries) and same picker_api_key (suffix `…qcG8`, full value redacted). The release.yml AUTH-03 PRODUCTION consent_screen gate passed for v1.0.2 (workflow run 25770477750 green in 2m2s).

## Diagnostic — error class

Google's `invalid_client` (not `invalid_grant`) means the client_id sent to Google's OAuth endpoint isn't recognized by Google's records. This is distinct from `invalid_grant` (revoked refresh token), `access_denied` (user/admin denied), and `unauthorized_client` (client exists but not allowed for this grant type).

`invalid_client` ≡ "this client_id does not exist in Google's database for this issuer."

## Interaction with AUTH-07 (Phase 9)

AUTH-07's `applyBootAuthError` calls `auth.IsRevokedRefreshToken(err)`, which matches on `invalid_grant`. The `invalid_client` error class FALLS THROUGH the AUTH-07 classifier into the existing ContinueSetup branch (non-revoked recovery), which relaunches the wizard, which restarts the OAuth flow, which then hits the same Google wall. This is correct behavior given the existing classifier scope — AUTH-07 was not designed to handle `invalid_client` and it shouldn't (different cause, different remediation).

## Likely root causes (ranked)

1. **OAuth client deleted in Google Cloud Console** — accidental delete in past ~24h. Most likely.
2. **GCP project 262087828393 disabled** — billing/ToS/abuse signal.
3. **Consent screen reverted to Testing or admin-unpublished** — usually returns `access_denied` not `invalid_client`, but possible if the client object itself was orphaned.
4. **Google-side transient quarantine** after the v1.0.2 release triggered ~13 simultaneous reauths.

## Triage URL

https://console.cloud.google.com/apis/credentials?project=262087828393

If client `262087828393-8obvbca97eb1q73f2kna7nef4adhq7bu` is missing or marked deleted, the bug is confirmed Google-side and requires either:
- Restore from Google's 30-day soft-delete window (if available)
- Create a new Desktop OAuth client, rotate `OAUTH_CONFIG_JSON` GitHub secret, ship a v1.0.3 with the new client_id (rebuilds will not affect v1.0.x already in the field — those need to be re-installed by guildies)

## State of Phase 9 close-out

Phase 9 verification PASSED programmatically (5/5 must-haves; verifier returned `human_needed` solely for on-VM UAT). Code review found 0 critical, 2 warnings (WR-01 gofmt drift in console_windows.go, WR-02 doc/impl mismatch on FreeConsole no-console path), 4 info-level items — all non-blocking, captured in `09-REVIEW.md`. ROADMAP not yet marked complete; STATE.md last updated at Wave 2.

**Decision pending user check of Google Cloud Console** before any further close-out commits or remote pushes. v1.0.2 tag remains pushed (cannot be reverted destructively).

## Files of record

- `.planning/phases/09-watcher-robustness-polish/09-REVIEW.md` (commit `ecbe463`)
- `.planning/phases/09-watcher-robustness-polish/09-VERIFICATION.md` (commit `101e4d7`)
- `.planning/phases/09-watcher-robustness-polish/09-05-SUMMARY.md` (commit `839a41e`)
- `.planning/debug/v1-0-2-oauth-invalid-client-incident.md` (this file)
- `https://github.com/boejowen/SquireBot/releases/tag/v1.0.2` (live)
- `https://github.com/boejowen/SquireBot/actions/runs/25770477750` (release workflow)
