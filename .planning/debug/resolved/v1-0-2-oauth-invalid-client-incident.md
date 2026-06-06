---
status: resolved
slug: v1-0-2-oauth-invalid-client-incident
opened: 2026-05-13T00:50:00Z
updated: 2026-06-06T00:00:00Z
root_cause: Google OAuth brand-verification gate flipped enforcement on the SquireBot Desktop Client between v1.0.1 ship (2026-05-11) and v1.0.2 ship (2026-05-13). Not a code bug. Affects ALL watcher versions (v0.4.0-rc1, v1.0.1, v1.0.2) identically.
severity: critical
impact: every SquireBot watcher in the guild loses auth on next refresh until Google approves brand verification
resolution_path: brand verification submitted to Google review queue 2026-05-13 — typical SLA 3–5 business days
resolution: SUPERSEDED by the v2.0 "Off Google" milestone (shipped 2026-05-31). Rather than wait on Google's brand-verification queue, the entire Google OAuth dependency was removed — the watcher now uploads to the self-hosted backend with a static bearer guild code, and the website uses Discord OAuth. The brand-verification gate no longer applies to any part of the system. Backlog 999.19 is marked SUPERSEDED for the same reason. Closed 2026-06-06.
---

# v1.0.2 invalid_client OAuth incident

> **RESOLVED — SUPERSEDED (2026-06-06).** This incident is moot: the v2.0 "Off Google" milestone (2026-05-31) removed Google OAuth from the system entirely (watcher → self-hosted backend with a static bearer code; website → Discord OAuth). The Google brand-verification gate that caused this no longer exists in the architecture. Retained as an incident record; see ROADMAP backlog 999.19 (SUPERSEDED) and the `project_website_milestone_scoped` / `feedback_oauth_consent_screen_first` memories.

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

Phase 9 verification PASSED programmatically (5/5 must-haves; verifier returned `human_needed` solely for on-VM UAT). Code review found 0 critical, 2 warnings (WR-01 gofmt drift in console_windows.go, WR-02 doc/impl mismatch on FreeConsole no-console path), 4 info-level items — all non-blocking, captured in `09-REVIEW.md`.

**Phase 9 close-out completed 2026-05-13** with OAuth verification status captured as blocking item 999.19. The 5 HUMAN-UAT scenarios cannot run until Google approves brand verification — they are persisted in `09-HUMAN-UAT.md` with `status: blocked_on_999.19` and will be re-tried once verification clears.

## Investigation trail (chronological)

1. **First lead — "OAuth client was not found"** (rejected). Verified the client_id baked into v1.0.1 binary and v1.0.2 binary are byte-identical — the build pipeline is innocent. Both versions hit the same Google wall.
2. **Second lead — multiple client secrets warning** (rejected). User disabled the older `…LKsu` secret; auth still failed. Confirmed the kept secret (`…ZCXH` suffix) matches the binary's baked value.
3. **Third lead — loopback IP migration** (rejected). Tested `http://localhost:<port>` vs `http://127.0.0.1:<port>` redirect URIs against the auth endpoint; both rejected identically with policy violation.
4. **Root cause — Google brand verification expired/flipped** (CONFIRMED). The OAuth consent screen page showed "Your branding needs to be verified before it can be shown to users" — Google's automated brand verification gate.
5. **Resolution path** — submitted to Google review queue 2026-05-13 with these fixes applied:
   - Privacy policy created at `docs/privacy-policy.md`, served at `https://boejowen.github.io/SquireBot/privacy-policy/`
   - Homepage URL changed from `https://github.com/boejowen/SquireBot` to `https://boejowen.github.io/SquireBot/` (GH Pages-hosted; owned-domain via Search Console verification)
   - Authorized domain `boejowen.github.io` added (replacing `github.com` which was unverifiable)
   - Domain ownership verified in Google Search Console via HTML file token committed at `docs/google7ea0696846f966ed.html`
   - Homepage updated to include explicit "Privacy policy →" link (Google reviewer-required)

## Anti-patterns to avoid next time

- Do NOT chase "multiple secrets" or "loopback IP" warnings first when the actual error is `Access blocked: Authorization Error / Error 400: invalid_request` with the link to the OAuth 2.0 policies page. Those are noise. Check **OAuth consent screen → Verification status** FIRST.
- Brand verification is a SEPARATE workflow from consent-screen-publishing-status. `consent_screen_status: PRODUCTION` does NOT mean the app is verified for production use — Google can publish a screen and then flip a verification gate independently.
- The release.yml AUTH-03 gate only checks the JSON field, not Google's actual verification state. Worth adding a live `oauth2.googleapis.com/.well-known/openid-configuration` ping or similar pre-flight check to a future hardening phase.

## Files of record

- `.planning/phases/09-watcher-robustness-polish/09-REVIEW.md` (commit `ecbe463`)
- `.planning/phases/09-watcher-robustness-polish/09-VERIFICATION.md` (commit `101e4d7`)
- `.planning/phases/09-watcher-robustness-polish/09-05-SUMMARY.md` (commit `839a41e`)
- `.planning/debug/v1-0-2-oauth-invalid-client-incident.md` (this file)
- `https://github.com/boejowen/SquireBot/releases/tag/v1.0.2` (live)
- `https://github.com/boejowen/SquireBot/actions/runs/25770477750` (release workflow)
