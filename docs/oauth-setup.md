# Google Cloud OAuth Setup — SquireBot Runbook

**Audience:** the developer (or a future maintainer) provisioning the Google Cloud project that backs SquireBot's OAuth + Picker flows.

**Time:** ~20 minutes if you're already logged into a Google account with a billing-enabled Cloud project. ~45 minutes if you're starting from scratch.

**Output:** an "In production" OAuth consent screen + three recorded constants (`oauth_client_id`, `picker_api_key`, `gcp_project_number`) saved into `.planning/phases/01-end-to-end-thin-slice/oauth-config.json`.

---

## Why This Matters

This is the load-bearing prerequisite for every other Phase 1 plan. Per **Pitfall #1** in `.planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md` §11:

- Google OAuth consent screens default to **Testing** mode.
- In Testing mode, refresh tokens issued to non-Workspace Gmail users **silently expire after 7 days**.
- If we ship the watcher to a guildie while Testing mode is in effect, their installation works for one week, then dies on day 8 with `invalid_grant` and they have to re-OAuth — except by then they've forgotten what SquireBot is and uninstall it.
- Therefore: the consent screen MUST be flipped to **In production** before any guildie installs.

Our scope set (`drive.file` + `openid` + `userinfo.email`) is *sensitive-exempt* (see RESEARCH.md §4.2 + §4.6 — drive.file is non-sensitive, the openid+userinfo.email combo is the basic-identity subset that bypasses the sensitive-scope review queue). Publishing should therefore be **immediate** — no Google audit, no waiting weeks. Step 6 of this runbook validates that assumption on a real project. If Google instead routes us into the verification queue, that is a Phase 1 blocker and the plan's Task 3 contingency kicks in.

---

## Prerequisites

- A Google account you control. (Recommended: an account that the rest of the guild does not depend on for anything else, in case Cloud project access ever needs to be transferred.)
- A Google Cloud **billing account** linked to that Google account. The APIs we use (Sheets, Drive, Picker) are free at our scale (~12 users × a handful of API calls/day), but Cloud requires a billing account on file before you can enable APIs that *could* incur cost.
- Browser access to https://console.cloud.google.com.
- A text editor open to `.planning/phases/01-end-to-end-thin-slice/oauth-config.json` so you can paste values as you go.

---

## Step 1 — Create or select a GCP project

1. Open https://console.cloud.google.com.
2. Top-left project dropdown → **New Project**.
3. **Project name:** `SquireBot` (or `SquireBot-prod` if you want headroom for a future test project).
4. **Organization / Location:** leave default (No organization is fine for a personal project).
5. Click **Create**. Wait for the project to be created (~10s) and switched to.
6. **Record two values:**
   - **`project_id`** — the human-readable string Google generates (e.g. `squirebot-prod-461923`). Visible in the project picker dropdown and at the top of the dashboard. *(Not strictly required by the watcher, but useful for `gcloud` commands and audit trails.)*
   - **`project_number`** — the **numeric** value (e.g. `194829382913`). Visible at **Cloud Overview → Dashboard** under "Project info" or **Settings → Project Settings**. **This is the App ID** for the Picker API per RESEARCH.md §5.4 — paste it into `oauth-config.json` as `gcp_project_number`.

> ⚠️ `project_id` and `project_number` are **different**. The Picker API needs the *number*. Do not paste the string.

---

## Step 2 — Enable required APIs

For each API below: **APIs & Services → Library**, search for the exact name, click the result, click **Enable**. Wait for "API enabled" toast before moving on (a few seconds each).

| API | Search term | Why we need it |
|---|---|---|
| Google Sheets API | `Google Sheets API` | Plan 05's `spreadsheets.batchUpdate` writes |
| Google Drive API | `Google Drive API` | Required by `drive.file` scope; metadata-only access to the picked workbook |
| Google Picker API | `Google Picker API` | Plan 06's workbook picker JS |

You do **not** need to enable People API, Identity Toolkit, or any "Google+ API" — `userinfo.email` is served by the OpenID Connect endpoints which are always-on for OAuth-enabled projects.

---

## Step 3 — Configure the OAuth consent screen

1. **APIs & Services → OAuth consent screen**.
2. **User type:** **External**. (Internal is Google Workspace–only and we have personal Gmail guildies.) Click **Create**.
3. **App information:**
   - **App name:** `SquireBot`
   - **User support email:** your own Gmail address (Google requires a real, monitored mailbox).
   - **App logo:** optional. Skip in Phase 1.
4. **App domain:**
   - **Application home page:** the GitHub repo URL (e.g. `https://github.com/boejowen/SquireBot`). Optional but Google nudges for it.
   - **Application privacy policy / Terms of service:** leave blank for now.
5. **Authorized domains:** **leave empty.** Desktop OAuth clients with loopback redirect_uri do not need authorized domains. (Adding one will not hurt, but is unnecessary noise.)
6. **Developer contact information:** your Gmail address.
7. Click **Save and Continue** → you're now on the **Scopes** page.
8. Click **Add or Remove Scopes** and paste/check exactly these three (use the search box to find each):
   - `https://www.googleapis.com/auth/drive.file`
   - `openid`
   - `https://www.googleapis.com/auth/userinfo.email`

   These are the three scopes called out in `CLAUDE.md` and RESEARCH.md §4.2. The combination is **sensitive-exempt** — `drive.file` is non-sensitive, and `openid + userinfo.email` is the basic-identity subset that bypasses the sensitive-scope review queue. Do not add any other scope. In particular, do **not** add `https://www.googleapis.com/auth/spreadsheets` or `https://www.googleapis.com/auth/drive` — those are sensitive and would trigger the audit queue.
9. Click **Update** then **Save and Continue** → you're now on the **Test users** page.
10. **Skip — leave Test users empty.** We are about to publish, which makes the test-user list moot.
11. Click **Save and Continue** → **Back to Dashboard**.

The consent screen is now created in **Testing** state. We publish it in Step 6 — but we still need the OAuth client and API key first.

---

## Step 4 — Create the OAuth 2.0 Client ID

1. **APIs & Services → Credentials → Create Credentials → OAuth client ID**.
2. **Application type:** **Desktop app**. *(Not Web. Not Android. Desktop. PKCE replaces the client secret for desktop clients per RESEARCH.md §4.1.)*
3. **Name:** `SquireBot Desktop Client`.
4. Click **Create**.
5. A modal pops up showing **Client ID** and **Client secret**.
   - **Copy `Client ID`** — looks like `123456789012-abcdef0123456789abcdef.apps.googleusercontent.com`. Paste into `oauth-config.json` as `oauth_client_id`.
   - **IGNORE `Client secret`.** PKCE replaces it. Per RESEARCH.md §4.1 the client_secret is *optional* for desktop clients; we will not use it and it must not be committed anywhere. If anyone ever asks "where do we store the client secret?" the answer is **we don't have one**.

> ⚠️ If you click the modal away before copying: re-open the OAuth client from the Credentials list and the Client ID is shown there permanently. The Client secret is also retrievable but **stays unused**.

---

## Step 5 — Create the Picker API key

1. **APIs & Services → Credentials → Create Credentials → API key**.
2. A modal shows the freshly-generated key. **Copy it** — paste into `oauth-config.json` as `picker_api_key`.
3. Click **Edit API key** (or open it from the Credentials list).
4. **Application restrictions:** **None**. Per RESEARCH.md §5.4, the Picker JS doesn't enforce origin checks for desktop apps — adding HTTP-referer or IP restrictions here will silently break the Picker.
5. **API restrictions:** click **Restrict key** → **Select APIs** → check **only** `Google Picker API`. This is the entire mitigation for T-02-03 (key-abuse blast radius).
6. Click **Save**.

> Why the key is safe to commit even though it's "an API key": Picker API keys restricted to the Picker API only are effectively public — the worst case is someone calls our Picker quota, and the per-day limit is generous. We commit it because Plan 06 will bake it into the binary at build time via `-ldflags` and the binary itself is published on GitHub Releases (i.e. inherently public). See RESEARCH.md §5.4 + the security note in `oauth-config.json` itself.

---

## Step 6 — PUBLISH THE CONSENT SCREEN

**This is the critical step.** Up to here, nothing about SquireBot has actually changed in Google's eyes — the consent screen is in Testing mode and the refresh-token-expiry bomb is armed.

1. **APIs & Services → OAuth consent screen** (you'll land on the dashboard for it).
2. Look for **Publishing status: Testing** with a **Publish App** button next to it. (As of 2026 the layout puts this on an "Audience" subtab — if you don't see it, click the "Audience" tab on the left rail.)
3. Click **Publish App**.
4. A confirmation dialog warns about needing to comply with Google's policies. Click **Confirm**.
5. **Watch the status flag.** One of two things happens:
   - **✅ Expected (success):** Status flips to **In production** immediately (within seconds). No queue, no email, no further action. This is what RESEARCH.md §4.6 predicts for our sensitive-exempt scope set.
   - **❌ Unexpected (failure):** Google instead displays "Submitted for verification" / "Pending review" / queues the request. **STOP.**
     - Do NOT proceed to Plan 03.
     - Open a blocker entry in `.planning/STATE.md` → **Active Blockers** section: short title, link to this runbook, what Google actually showed, what scope appears to have triggered it.
     - Document the actual response (verbatim copy of Google's UI text) in `01-02-SUMMARY.md`.
     - Possible contingency: drop `openid` and rely on Drive file metadata for email. But this loses AUTH-06's clean identity story — treat as last resort.
6. **Record the publish timestamp** as `consent_screen_published_at` in `oauth-config.json` (ISO 8601 UTC, e.g. `2026-05-01T15:30:00Z`).
7. **Set** `consent_screen_status` **to** `"PRODUCTION"` in `oauth-config.json`. (If you hit the failure case in 5b, set it to `"FAILED:<short reason>"` and stop.)

---

## Step 7 — Record values in oauth-config.json

By this point `.planning/phases/01-end-to-end-thin-slice/oauth-config.json` should have **no remaining `TODO_` sentinels**. Open it and confirm all five fields are filled:

```json
{
  "schema_version": 1,
  "_comment": "...",
  "_security_note": "...",
  "oauth_client_id": "123456789012-abcdef…apps.googleusercontent.com",
  "picker_api_key": "AIzaSy…",
  "gcp_project_number": "194829382913",
  "consent_screen_status": "PRODUCTION",
  "consent_screen_published_at": "2026-05-01T15:30:00Z"
}
```

Validate the JSON parses (any of these works):
- PowerShell: `Get-Content .planning/phases/01-end-to-end-thin-slice/oauth-config.json | ConvertFrom-Json`
- Python: `python -c "import json; print(json.load(open(r'.planning/phases/01-end-to-end-thin-slice/oauth-config.json')))"`
- jq (if installed): `jq . .planning/phases/01-end-to-end-thin-slice/oauth-config.json`

---

## Step 8 — Verify by hand (browser test)

This proves the consent screen is genuinely published — not just locally cached as published. Open this URL in a browser, replacing `<client_id>` with your real value from Step 4:

```
https://accounts.google.com/o/oauth2/v2/auth?client_id=<client_id>&redirect_uri=http://127.0.0.1:9999/oauth/callback&response_type=code&scope=https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fdrive.file%20openid%20https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fuserinfo.email&access_type=offline&prompt=consent&code_challenge=E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM&code_challenge_method=S256&state=test
```

What you should see:

- ✅ Google account picker → SquireBot consent screen with the three scopes listed (Drive (App-Created Files), Email Address, OpenID).
- ✅ **NO** banner saying "This app isn't verified" or "Google hasn't verified this app". If that banner appears, publishing did NOT take effect — go back to Step 6.
- ⚠️ After clicking "Allow", the browser tries to redirect to `http://127.0.0.1:9999/oauth/callback?…` and the connection fails. **That's expected** — nothing is listening on port 9999. We only care about the consent screen visual; the failed redirect is the real PKCE flow, but it requires a running watcher (Plan 03's job).

If the consent screen looks right, publishing succeeded.

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| "Submitted for verification" after clicking Publish App | Scope set unexpectedly contains a sensitive scope | Re-check Step 3.8 — only `drive.file`, `openid`, `userinfo.email` should be present. Remove anything else and republish. |
| `redirect_uri_mismatch` error during the Step 8 browser test | The client is configured as Web instead of Desktop, OR the consent screen has authorized JavaScript origins set | Re-check Step 4.2 (Desktop app) and Step 3.5 (no authorized domains). |
| `403: Picker API has not been used in project N before…` later in Plan 06 | Picker API not enabled | Re-check Step 2 — enable Google Picker API. |
| `403: API key not valid` later in Plan 06 | API key restrictions are too tight or wrong API restricted | Re-check Step 5.4 (Application restrictions = None) and Step 5.5 (API restrictions = Google Picker API only). |
| "This app isn't verified" banner appears in Step 8 | Publishing did not take effect | Reload the OAuth consent screen page in console. If Status still says Testing, click Publish App again. If Status says In production but the banner appears, wait 60s and reload — propagation can lag briefly. |
| Browser refuses to load Google sign-in: `client_id` is wrong | Pasted the project_id or numeric project_number instead of the OAuth client ID | The OAuth client ID always ends in `.apps.googleusercontent.com`. Re-copy from Credentials → OAuth 2.0 Client IDs. |

---

## Reproduction Note

If you ever need to migrate to a different Google account or a fresh Cloud project (e.g. someone else takes over Phase 1, or this becomes a Workspace-org-owned project), repeat Steps 1–6 verbatim and overwrite the values in `oauth-config.json`. Plan 03 reads `oauth_client_id` and `gcp_project_number` from this JSON at build time via `-ldflags`, so changing the file + rebuilding the binary is the entire migration.
