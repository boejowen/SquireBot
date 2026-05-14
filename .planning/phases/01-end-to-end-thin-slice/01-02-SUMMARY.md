---
phase: 01-end-to-end-thin-slice
plan: 02
status: complete
completed_at: 2026-05-01T14:17:22Z
requirements_closed: [AUTH-03]
---

# Plan 01-02 — OAuth Cloud Setup — SUMMARY

## What Got Built

| # | Task | Output | Tracked? |
|---|---|---|---|
| 1 | Author Cloud Console runbook | `docs/oauth-setup.md` (194 lines, 12 H2 sections) | ✅ committed `fd4ff3c` |
| 2 | Scaffold oauth-config.json | `.planning/phases/01-end-to-end-thin-slice/oauth-config.json` (TODO sentinels) | ❌ gitignored (`.planning/`) — by project policy |
| 3 | Human checkpoint: GCP Console + publish | Filled-in `oauth-config.json` (5 real values, no TODOs) | ❌ gitignored — local-only |

Both files exist on disk. The runbook is committed; the config-with-real-values is intentionally local-only per the project's pre-existing `.gitignore` rule on `.planning/`. Plan 03 reads `oauth-config.json` for build-time `-ldflags` regardless of tracked state — this matches the "if the dev prefers not to commit" branch in Plan 01-02 Task 3 itself.

## GCP Project Recorded

- **GCP project number:** `262087828393` (numeric — public; this is the App ID for the Picker API per RESEARCH.md §5.4)
- **OAuth client ID:** `262087828393-8obvbca97eb1q73f2kna7nef4adhq7bu.apps.googleusercontent.com` (Desktop app type — public per RESEARCH.md §4.1)
- **Picker API key:** `AIzaSy…cG8` (39 chars, starts with `AIzaSy` — restricted to Google Picker API only; effectively public per RESEARCH.md §5.4)
- **OAuth client secret:** *not retained anywhere* — desktop client uses PKCE per RESEARCH.md §4.1

The OAuth client ID's numeric prefix matches `gcp_project_number` exactly — Google embeds the project number into every issued client ID, so this is an internal consistency check that the right project was used.

## Open Question Q3 — RESOLVED

> *"How long does Google take to flip the consent screen from Testing to In production for our sensitive-exempt scope set?"*

**Answer: instantaneous.** Status flipped from `Testing` to `In production` immediately when **Publish App** was clicked. No verification queue, no email, no audit. This validates RESEARCH.md §4.6's prediction that the `drive.file + openid + userinfo.email` combination is sensitive-exempt and bypasses Google's review pipeline.

- **Click time:** `2026-05-01T14:17:22Z`
- **Publish completion:** same timestamp (no measurable lag)
- **Implication for Phase 5 rollout:** no buffer time needs to be reserved for "wait for Google." Phase 1 can ship a Production-mode binary as soon as Plan 08 produces an installer.

## AUTH-03 Closed

> *"OAuth consent screen flipped to Production before any guildie installs (Testing-mode silent 7-day refresh expiry)"*

Consent screen `Publishing status = In production` confirmed in Cloud Console. Pitfall #1 (silent 7-day refresh expiry on Testing-mode tokens) is now structurally impossible for any guildie installing this watcher — refresh tokens issued post-publish do not expire on a 7-day timer.

## Verification Gate (all PASS)

Ran on `2026-05-01T14:21:xxZ`:

```
[PASS] schema_version == 1
[PASS] oauth_client_id matches regex ^[0-9]+-[a-z0-9]+\.apps\.googleusercontent\.com$
[PASS] gcp_project_number is numeric
[PASS] project_number == client_id prefix (internal consistency)
[PASS] picker_api_key starts AIzaSy + 33 chars (Google API key shape)
[PASS] consent_screen_status == PRODUCTION
[PASS] consent_screen_published_at is ISO 8601 UTC
[PASS] no TODO_ sentinels remain
[PASS] no client_secret / refresh_token / access_token substring (case-insensitive)
[PASS] runbook has ≥ 8 H2 sections, ≥ 50 lines, contains drive.file/openid/userinfo.email/Publish App/Desktop app/Testing/In production
```

## Deviations from Plan

### 1. Plan-spec contradiction in Task 2 — auto-fixed

The plan's `<action>` block instructs us to write a `_security_note` field whose body literally contains the substring `client_secret` (with underscore). The plan's `<acceptance_criteria>` block then forbids any case-insensitive match of `client_secret` anywhere in the file via grep. These are mutually exclusive.

**Fix:** rephrased the prose to use "client secret" (with a space, no underscore) while preserving the security guarantee. The note still explains exactly why the file is safe to commit and which credentials must never be added — just without tripping the regex.

**Recommendation for the planner-loop:** when a future plan template emits `_security_note` boilerplate that mentions credential field names by their `snake_case` form, change to space-separated form so the verify regex still catches accidental real values without false-positiving on documentation.

### 2. `jq` not available on this machine — substituted PowerShell

The plan's Task 2 verify block uses `jq -e '.field' file` (six invocations). `jq` is not on this machine's PATH. Substituted PowerShell `Get-Content … | ConvertFrom-Json` for parse + key-presence checks. All assertions equivalent to the original `jq` checks.

**Recommendation:** consider `scoop install jq` or `choco install jq` for future plans (Plan 03+ may use jq for OAuth response inspection during ad-hoc debugging). Not a blocker — PowerShell handles JSON natively.

### 3. `oauth-config.json` not committed (per existing project policy)

Plan 01-02 Task 3 explicitly contemplates both branches ("dev MAY commit oauth-config.json" / "if the dev prefers not to commit, add to .gitignore + record in 1Password"). The project's existing `.gitignore` already has `.planning/` from `eb719da chore: ignore planning docs (local-only)`, predating this plan. The "don't commit" branch is therefore the active policy and was applied unmodified.

**Backup recommendation for the developer:** record `oauth_client_id`, `picker_api_key`, and `gcp_project_number` in 1Password / a personal password manager, since `.planning/` is local-only and would not survive a clean repo clone. Plan 03's build relies on these values being present in the file.

## Pointer to Runbook

For re-provisioning (account migration, fresh Cloud project, Workspace org transfer), follow `docs/oauth-setup.md` Steps 1–6 verbatim and overwrite the values in `oauth-config.json`. Plan 03's `-ldflags` will pick up the new values on the next build with no code changes required.

## Files Modified

- `docs/oauth-setup.md` (created — 194 lines, committed `fd4ff3c`)
- `.planning/phases/01-end-to-end-thin-slice/oauth-config.json` (created + filled — gitignored)
- `.planning/phases/01-end-to-end-thin-slice/01-02-SUMMARY.md` (this file — gitignored)

## What Plan 03 Now Has Available

The next plan (`01-03-oauth-loopback-pkce-PLAN.md`) can:

1. Read `oauth-config.json` at build time and bake `OAuthClientID`, `PickerAPIKey`, `GCPProjectNumber` into the binary via `-ldflags='-X main.OAuthClientID=… -X main.PickerAPIKey=… -X main.GCPProjectNumber=…'`.
2. Trust that `consent_screen_status == "PRODUCTION"` and refuse to build (per T-02-04 mitigation) if it ever isn't.
3. Open the OAuth consent flow with confidence that no "This app isn't verified" banner will appear for guildies.

## Wave 1 — Status

Wave 1 of Phase 1 (`branching_strategy: none`, parallel-eligible) is complete. Plans 01-01 and 01-02 are both done. Wave 2 (`01-03 oauth-loopback-pkce` and `01-04 eq-watcher-parser`) is unblocked.
