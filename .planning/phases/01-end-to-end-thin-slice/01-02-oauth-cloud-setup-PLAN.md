---
phase: 01-end-to-end-thin-slice
plan: 02
type: execute
wave: 1
depends_on: []
files_modified:
  - docs/oauth-setup.md
  - .planning/phases/01-end-to-end-thin-slice/oauth-config.json
autonomous: false
requirements: [AUTH-03]
user_setup:
  - service: google-cloud
    why: "OAuth consent screen MUST flip to Production before any guildie installs (Testing-mode refresh tokens silently expire after 7 days for non-Workspace users — AUTH-03 + Pitfall #1 from 01-RESEARCH.md §11)"
    env_vars:
      - name: SQUIREBOT_OAUTH_CLIENT_ID
        source: "Google Cloud Console -> APIs &amp; Services -> Credentials -> OAuth 2.0 Client IDs (Desktop app type, no client_secret needed for PKCE)"
      - name: SQUIREBOT_PICKER_API_KEY
        source: "Google Cloud Console -> APIs &amp; Services -> Credentials -> API keys (restricted to Google Picker API only)"
      - name: SQUIREBOT_GCP_PROJECT_NUMBER
        source: "Google Cloud Console -> Project Settings (numeric, NOT the project ID)"
    dashboard_config:
      - task: "Create new Google Cloud project (or reuse existing)"
        location: "console.cloud.google.com -> New Project"
      - task: "Enable Google Sheets API + Google Drive API + Google Picker API"
        location: "APIs &amp; Services -> Library"
      - task: "Configure OAuth consent screen with scopes: drive.file + openid + userinfo.email (sensitive-exempt subset)"
        location: "APIs &amp; Services -> OAuth consent screen / Audience tab"
      - task: "Create OAuth 2.0 Client ID of type Desktop app"
        location: "APIs &amp; Services -> Credentials -> Create Credentials -> OAuth client ID"
      - task: "Create API key restricted to Google Picker API"
        location: "APIs &amp; Services -> Credentials -> Create Credentials -> API key -> Restrict key"
      - task: "Click 'Publish app' to flip consent screen from Testing to In Production"
        location: "APIs &amp; Services -> OAuth consent screen / Audience tab -> Publish App button"
must_haves:
  truths:
    - "Google Cloud Console shows OAuth consent screen Publishing status = 'In production' (NOT 'Testing')"
    - "OAuth client ID, Picker API key, and GCP project number are recorded in a non-committed local secrets file (developer machine only)"
    - "A scratch `gcloud` or curl-driven OAuth probe (or manual browser test from Plan 03) succeeds without showing the 'unverified app' Testing-mode banner"
    - "Production turnaround was confirmed immediate (per RESEARCH.md §4.6 / Open Question Q3) — if Google did NOT publish immediately, the developer has logged the actual queue time and a contingency plan in the SUMMARY"
  artifacts:
    - path: "docs/oauth-setup.md"
      provides: "Step-by-step Cloud Console runbook so this is reproducible (e.g., if the dev needs to migrate to a different GCP account)"
      min_lines: 50
    - path: ".planning/phases/01-end-to-end-thin-slice/oauth-config.json"
      provides: "Non-secret build-time constants (client ID, project number, API key) — committed since OAuth client IDs are public per RESEARCH.md §5.4 + Picker API keys restricted to Picker-only are effectively public"
      contains: "oauth_client_id"
  key_links:
    - from: "Google Cloud Console OAuth consent screen"
      to: "Production publishing status"
      via: "User clicks Publish App button"
      pattern: "publishing_status.*=.*PRODUCTION"
    - from: "docs/oauth-setup.md"
      to: ".planning/phases/01-end-to-end-thin-slice/oauth-config.json"
      via: "doc instructs how to populate the JSON values"
      pattern: "oauth_client_id"
---

<objective>
Resolve the load-bearing prerequisite that AUTH-03 places on the entire phase: Google Cloud
project provisioned with the right APIs enabled, OAuth client of type Desktop app created, Picker
API key created, and OAuth consent screen FLIPPED TO PRODUCTION before any guildie installs. This
is RESEARCH.md Open Question Q3 (the "publish-app turnaround" risk) executed as the FIRST task —
per Critical Constraint #4 it is a load-bearing W0/W1 task that gates Plan 03's coding.

Purpose: Testing-mode OAuth issues refresh tokens that silently expire 7 days later for
non-Workspace users. Every guildie's watcher would die on day 8 with `invalid_grant` — exactly the
existential pitfall #1 Phase 1 is supposed to validate AGAINST. We have to prove on a throwaway
project that "drive.file + openid + userinfo.email" publishes immediately (sensitive-exempt) and
not into the audit queue.

Output: An "In production" OAuth consent screen, three recorded constants (client_id, project_number,
api_key), a reproducible runbook (docs/oauth-setup.md), and a committed JSON file holding the
non-secret constants Plan 03 will bake into the binary at build time via -ldflags.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/phases/01-end-to-end-thin-slice/01-CONTEXT.md
@.planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md
@.planning/research/STACK.md
</context>

<tasks>

<task type="auto">
  <name>Task 1: Author the Cloud Console runbook (docs/oauth-setup.md)</name>
  <files>docs/oauth-setup.md</files>
  <read_first>
    - .planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md (§4 "OAuth Loopback PKCE — Concrete Recipe", specifically §4.5/§4.6 Production publishing detail and §5.4 "OAuth client console setup for Picker" — both contain the canonical step list)
    - .planning/phases/01-end-to-end-thin-slice/01-CONTEXT.md (decisions D-13 unsigned + manual SmartScreen accepted; Open Questions Q2/Q3 in the canonical_refs section)
    - ./CLAUDE.md (OAuth scope rule: drive.file + openid + userinfo.email ONLY; Production state is non-negotiable)
  </read_first>
  <action>
    Create `docs/oauth-setup.md` with these mandatory sections (use ## headers exactly as listed,
    so future automation can section-grep):

    ## Why This Matters
    Briefly cite Pitfall #1 from 01-RESEARCH.md: Testing-mode = silent 7-day refresh expiry =
    every guildie dies on day 8 = phase failure. ~5 lines.

    ## Prerequisites
    - A Google account with billing enabled (no actual billing for these APIs at our scale, but
      Cloud requires a billing account on file)
    - Browser access to console.cloud.google.com

    ## Step 1 — Create or select a GCP project
    Numbered sub-steps with exact menu paths. Record `project_id` (string) and `project_number`
    (numeric) — both are needed (project_number = the App ID for the Picker API per RESEARCH.md §5.4).

    ## Step 2 — Enable required APIs
    Enable EXACTLY: Google Sheets API, Google Drive API, Google Picker API. List each with the
    direct console search-box term.

    ## Step 3 — Configure the OAuth consent screen
    - User type: External (for guildies on personal Gmail).
    - App name, user support email, developer contact email — fill with sensible values.
    - Scopes — add ALL THREE in one screen, exact strings to paste:
      - `https://www.googleapis.com/auth/drive.file`
      - `openid`
      - `https://www.googleapis.com/auth/userinfo.email`
    - Cite RESEARCH.md §4.2: this combination is sensitive-exempt; no audit needed.
    - Authorized domains: leave empty (desktop client; not required).
    - Test users: skip (we publish before adding any).

    ## Step 4 — Create the OAuth 2.0 Client ID
    - Type: **Desktop app** (NOT Web — RESEARCH.md §5.4).
    - Name: `SquireBot Desktop Client`.
    - On creation, Google offers `client_secret` — IGNORE IT (PKCE replaces; per
      RESEARCH.md §4.1 client_secret is optional for desktop clients).
    - Record `oauth_client_id` (looks like `NNNNNNNNNNNN-xxxxxxxxxxxx.apps.googleusercontent.com`).

    ## Step 5 — Create the Picker API key
    - Create credentials -> API key.
    - Click Edit -> Application restrictions: None (Picker JS doesn't enforce origin checks for
      desktop apps per RESEARCH.md §5.4).
    - API restrictions: Restrict key -> Select APIs -> Google Picker API ONLY.
    - Record `picker_api_key`.

    ## Step 6 — PUBLISH THE CONSENT SCREEN
    THIS IS THE CRITICAL STEP. Cite RESEARCH.md §4.6 + Open Question Q3.
    1. Go to APIs & Services -> OAuth consent screen -> Audience tab.
    2. Click **Publish App**. Confirm.
    3. Status should flip from "Testing" to "In production" IMMEDIATELY (no audit queue for our
       sensitive-exempt scope set).
    4. If Google instead displays "Submitted for verification" or queues the request:
       - STOP. Do NOT proceed with Plan 03.
       - Open a blocker entry in `.planning/STATE.md` -> Active Blockers section.
       - Document the actual response in this Plan's SUMMARY.
       - Wait for Google verification OR consider scope reduction (drop openid; rely on Drive
         file metadata for email — but this loses AUTH-06's clean identity story; treat as
         contingency only).

    ## Step 7 — Record values in oauth-config.json
    Instruct the dev to copy the three values into
    `.planning/phases/01-end-to-end-thin-slice/oauth-config.json` (Task 3 creates the empty file).

    ## Step 8 — Verify by hand
    Open this URL in a browser (substituting `<client_id>`):
    ```
    https://accounts.google.com/o/oauth2/v2/auth?client_id=<client_id>&redirect_uri=http://127.0.0.1:9999/oauth/callback&response_type=code&scope=https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fdrive.file%20openid%20https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fuserinfo.email&access_type=offline&prompt=consent&code_challenge=E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM&code_challenge_method=S256&state=test
    ```
    The consent screen MUST NOT show "This app isn't verified" / "Google hasn't verified this app"
    banner. If the banner appears, publishing did NOT take effect — re-check Step 6.

    The redirect to `127.0.0.1:9999` will fail (nothing listening) — that's expected; we only
    need the consent screen visual check.

    ## Troubleshooting
    Document at minimum: "audit queue triggered" → contingency, "redirect_uri mismatch" → re-check
    Step 4, "Picker API not enabled" → re-check Step 2.

    File should be 50+ lines and read like a runbook a different developer could follow.
  </action>
  <verify>
    <automated>test -s docs/oauth-setup.md &amp;&amp; grep -c "^## " docs/oauth-setup.md | awk '$1 &gt;= 8 {exit 0} {exit 1}' &amp;&amp; grep -q "drive\.file" docs/oauth-setup.md &amp;&amp; grep -q "openid" docs/oauth-setup.md &amp;&amp; grep -q "userinfo\.email" docs/oauth-setup.md &amp;&amp; grep -q -i "publish app" docs/oauth-setup.md &amp;&amp; grep -q "Desktop app" docs/oauth-setup.md</automated>
  </verify>
  <acceptance_criteria>
    - `docs/oauth-setup.md` exists
    - File contains at least 8 H2 (`## `) sections
    - File contains the literal string `drive.file`
    - File contains the literal string `openid`
    - File contains the literal string `userinfo.email`
    - File contains the literal string `Publish App` (case-insensitive ok)
    - File contains the literal string `Desktop app` (the OAuth client type)
    - File explicitly mentions `Testing` and `In production` (states involved)
    - File length ≥ 50 lines
  </acceptance_criteria>
  <done>
    `docs/oauth-setup.md` is a faithful step-by-step runbook a different developer could follow
    end-to-end, ending in a Production-published consent screen.
  </done>
</task>

<task type="auto">
  <name>Task 2: Create the empty oauth-config.json scaffold</name>
  <files>.planning/phases/01-end-to-end-thin-slice/oauth-config.json</files>
  <read_first>
    - .planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md (§5.4 — items 4/5/6: API_KEY and APP_ID are baked into the binary as build-time constants OR fetched from latest.json; OAuth client IDs are public per the Picker doc cite)
    - .planning/phases/01-end-to-end-thin-slice/01-CONTEXT.md (Open Question Q2 in canonical_refs explains rationale for committing these as public values)
  </read_first>
  <action>
    Create `.planning/phases/01-end-to-end-thin-slice/oauth-config.json` with this exact structure:
    ```json
    {
      "schema_version": 1,
      "_comment": "These are the THREE Google Cloud Console identifiers Plan 03 will bake into the binary at build time via -ldflags='-X main.OAuthClientID=... -X main.PickerAPIKey=... -X main.GCPProjectNumber=...'. OAuth client IDs are public (visible in the URL during the consent flow). Picker API keys restricted to 'Google Picker API only' are effectively public (per RESEARCH.md §5.4). GCP project number is public (visible in any console URL).",
      "_security_note": "These values are SAFE to commit. The OAuth client_secret is NOT here and MUST NOT BE — desktop clients use PKCE and have no client_secret per RESEARCH.md §4.1. Refresh tokens live ONLY in wincred per AUTH-04.",
      "oauth_client_id": "TODO_FILL_FROM_STEP_4_OF_oauth-setup.md",
      "picker_api_key": "TODO_FILL_FROM_STEP_5_OF_oauth-setup.md",
      "gcp_project_number": "TODO_FILL_FROM_STEP_1_OF_oauth-setup.md",
      "consent_screen_status": "TODO_AFTER_STEP_6_OF_oauth-setup.md (must be 'PRODUCTION')",
      "consent_screen_published_at": "TODO_ISO8601_DATE_AFTER_PUBLISH"
    }
    ```
    Do NOT pre-fill the values. The next Task is the human checkpoint where the dev does the
    Console work and fills these in.
  </action>
  <verify>
    <automated>test -s .planning/phases/01-end-to-end-thin-slice/oauth-config.json &amp;&amp; jq -e '.oauth_client_id' .planning/phases/01-end-to-end-thin-slice/oauth-config.json &amp;&amp; jq -e '.picker_api_key' .planning/phases/01-end-to-end-thin-slice/oauth-config.json &amp;&amp; jq -e '.gcp_project_number' .planning/phases/01-end-to-end-thin-slice/oauth-config.json &amp;&amp; jq -e '.consent_screen_status' .planning/phases/01-end-to-end-thin-slice/oauth-config.json &amp;&amp; jq -e '.schema_version == 1' .planning/phases/01-end-to-end-thin-slice/oauth-config.json</automated>
  </verify>
  <acceptance_criteria>
    - `.planning/phases/01-end-to-end-thin-slice/oauth-config.json` exists
    - Valid JSON (`jq . <file>` exits 0)
    - Contains keys: `schema_version`, `oauth_client_id`, `picker_api_key`, `gcp_project_number`, `consent_screen_status`, `consent_screen_published_at`
    - Does NOT contain a key matching `client_secret`, `secret`, `refresh_token`, `access_token` (case-insensitive)
    - `grep -i "client_secret" .planning/phases/01-end-to-end-thin-slice/oauth-config.json` returns 0 matches
  </acceptance_criteria>
  <done>
    Empty oauth-config.json exists with placeholder TODO values and explicit comments documenting
    why these three values are safe to commit and which secrets are forbidden.
  </done>
</task>

<task type="checkpoint:human-action" gate="blocking">
  <name>Task 3: Developer executes the Cloud Console runbook and publishes consent screen</name>
  <what-built>
    Tasks 1 and 2 produced (a) `docs/oauth-setup.md` — the step-by-step runbook, and (b) an empty
    `.planning/phases/01-end-to-end-thin-slice/oauth-config.json` waiting for three values.
    Nothing is automatable here — the rest is browser-only work in console.cloud.google.com.
  </what-built>
  <how-to-verify>
    1. Open `docs/oauth-setup.md` in your editor and follow Steps 1-6 exactly. Use a NEW Google
       Cloud project (a "throwaway" per RESEARCH.md Open Question Q3) — or a dedicated SquireBot
       project if you prefer permanence.

    2. Specifically at Step 6 ("PUBLISH THE CONSENT SCREEN"), click **Publish App** in the
       Audience tab. Observe what Google does:
       - **Expected (success):** Status flips to "In production" IMMEDIATELY. Cite RESEARCH.md §4.6
         and AUTH-03; this is the entire reason this plan exists.
       - **Unexpected (failure):** "Submitted for verification" or queue. STOP. Document in this
         plan's SUMMARY. Open a blocker in STATE.md. Do NOT proceed to Plan 03.

    3. Edit `.planning/phases/01-end-to-end-thin-slice/oauth-config.json` and replace ALL FIVE
       TODO values:
       - `oauth_client_id` — from Step 4
       - `picker_api_key` — from Step 5
       - `gcp_project_number` — from Step 1
       - `consent_screen_status` — set to `"PRODUCTION"` (or document failure mode)
       - `consent_screen_published_at` — set to current ISO 8601 UTC date (e.g., `"2026-04-30T18:00:00Z"`)

    4. Validate the published consent screen by hand using the URL in Step 8 of the runbook
       (substituting your real `<client_id>`). Confirm NO "This app isn't verified" banner
       appears. The redirect will fail (no listener at :9999) — that's fine.

    5. Confirm the JSON is valid: run `jq . .planning/phases/01-end-to-end-thin-slice/oauth-config.json`
       — should pretty-print without error.

    6. The dev MAY commit oauth-config.json (per Task 2 rationale, all three values are public).
       If the dev prefers not to commit: add `.planning/phases/01-end-to-end-thin-slice/oauth-config.json`
       to .gitignore AND record the values in a 1Password / macOS Keychain entry. Plan 03's
       Task 1 will read this JSON file for build-time constants regardless of whether it is
       committed.
  </how-to-verify>
  <resume-signal>
    Type **"published"** if Step 6 succeeded and the JSON is filled in.

    Type **"queued: &lt;details&gt;"** if Step 6 went into the audit queue. (Will block Plan 03 and
    require contingency planning — see RESEARCH.md Open Question Q3.)

    Type **"deferred: I'll do this myself between now and Plan 03"** if you want to defer the
    Console work. Note: Plan 03 cannot start coding OAuth until this is done; the Cloud client_id
    is required for the loopback flow.
  </resume-signal>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| developer machine ↔ Google Cloud Console | Authenticated browser session creates lasting OAuth client; misconfigured scopes leak privilege |
| Cloud Console ↔ committed oauth-config.json | Public client_id and Picker API key recorded; misuse = abuse of dev's Cloud project quota |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-02-01 | Privilege Escalation | OAuth consent scope set drifting beyond drive.file + openid + userinfo.email | mitigate | Runbook Step 3 lists the EXACT three scopes verbatim; Plan 03 acceptance criteria greps Go source for any other scope string and rejects on match |
| T-02-02 | Information Disclosure | Developer accidentally records client_secret in oauth-config.json | mitigate | oauth-config.json schema explicitly omits a client_secret field and acceptance criteria greps for `client_secret` substring; runbook Step 4 explicitly says "IGNORE the client_secret Google offers" |
| T-02-03 | Spoofing | Picker API key abused by someone else against our quota | accept | Restriction to "Google Picker API only" + per-day quotas in our Cloud project bound the blast radius. Worst case: dev rotates the key in 5 min. Per RESEARCH.md §5.4 these keys are effectively public for desktop clients |
| T-02-04 | Tampering | Wrong project published (e.g., dev publishes a "test" project but builds against a "prod" project) | mitigate | oauth-config.json records `gcp_project_number` AND `consent_screen_status`; Plan 03 Task 1 greps oauth-config.json for `consent_screen_status == "PRODUCTION"` and refuses to build otherwise |
| T-02-05 | Repudiation | No audit trail of who clicked "Publish App" or when | mitigate | `consent_screen_published_at` ISO timestamp recorded in oauth-config.json; SUMMARY documents who performed the action |
| T-02-06 | Denial of Service | Google's audit queue stalls Phase 1 indefinitely (Open Question Q3 negative outcome) | mitigate | Runbook Step 6 contingency block forces an explicit STOP and STATE.md blocker entry; Phase 1 cannot ship a Production binary until resolved; this is itself the "validation of three existential pitfalls" Phase 1 exists to perform |
</threat_model>

<verification>
- `docs/oauth-setup.md` exists and follows the 8-section structure
- `.planning/phases/01-end-to-end-thin-slice/oauth-config.json` exists, is valid JSON, and after Task 3 has all five TODO values replaced
- `jq -r '.consent_screen_status' .planning/phases/01-end-to-end-thin-slice/oauth-config.json` outputs `PRODUCTION` (NOT `Testing`, NOT a TODO sentinel)
- `jq -r '.oauth_client_id' .planning/phases/01-end-to-end-thin-slice/oauth-config.json` outputs a value matching the regex `^[0-9]+-[a-z0-9]+\.apps\.googleusercontent\.com$`
- `jq -r '.gcp_project_number' .planning/phases/01-end-to-end-thin-slice/oauth-config.json` outputs a numeric string
- The consent screen visible in browser does NOT show "This app isn't verified"
</verification>

<success_criteria>
- OAuth consent screen status = "In production" in console.cloud.google.com (AUTH-03 satisfied)
- Three constants recorded in oauth-config.json with non-TODO values (Plan 03 will read these)
- docs/oauth-setup.md is a complete, self-contained runbook for reproduction
- Open Question Q3 from RESEARCH.md is resolved (or its failure mode is documented and a blocker is open in STATE.md)
- No client_secret committed to the repo
</success_criteria>

<output>
After completion, create `.planning/phases/01-end-to-end-thin-slice/01-02-SUMMARY.md` documenting:
- The actual GCP project name + number used
- The actual time it took for "Publish App" to flip to "In production" (validates Open Question Q3)
- Whether oauth-config.json was committed or kept local-only
- Any deviations from the runbook (e.g., chosen alternative project name, different developer email on consent screen)
- A pointer to the runbook for future re-execution
</output>
