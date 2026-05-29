# Website Milestone — Slice 03: Watcher Re-Targeting & Auth

**Status:** Research / scoping only. No application code changed.
**Date:** 2026-05-20
**Scope of this doc:** What happens to the Go watcher when the upload sink moves
from the Google Sheets API to a custom backend HTTP API; how the watcher
authenticates to that backend; how humans log into the new website; and — the
load-bearing question — whether any chosen path re-introduces the Google OAuth
brand-verification gate this milestone exists to escape.

---

## 1. Watcher Code: Survives vs. Changes vs. Deleted

The watcher is ~14,500 LOC Go across `cmd/squirebot` + `internal/*`. The
re-target is **far smaller than it looks**, because the watcher's coupling to
Google is concentrated in three packages (`auth`, `sheet`, and the picker), all
behind clean seams. The file-watching / parsing / tray / update / installer
machinery has *zero* Google dependency.

### 1.1 SURVIVES UNCHANGED (the bulk of the codebase, ~9,000–10,000 LOC)

These packages do not import `google.golang.org/api`, `golang.org/x/oauth2`, or
anything Sheets-specific. They are pure local-IO / OS / process plumbing:

| Package / file | Why it survives |
|---|---|
| `internal/watch` (`watcher.go`, `debounce.go`) | fsnotify + 500 ms debounce. Knows nothing about where data goes. **0 changes.** |
| `internal/parse` (`inventory.go`, `spellbook.go`) | TSV → `[][]string`. Sink-agnostic. **0 changes.** |
| `internal/eqfind` | EQ-folder discovery heuristics + registry probing. **0 changes.** |
| `internal/system` | Windows named-event shutdown IPC (installer shim). **0 changes.** |
| `internal/tray` | systray UI. Menu items are re-labelled (see §1.3) but the package itself is intact. |
| `internal/update` (`check.go`, `swap.go`, `manifest.go`) | Auto-update via GitHub Releases + `minio/selfupdate` + SHA-256. **No Google coupling — survives. This is the migration vehicle (see §5).** |
| `internal/logging` | lumberjack rotation. **0 changes.** |
| `internal/heartbeat` (the `internal/heartbeat` pkg) | The *scheduling* shell (fire now, then every 24 h, honor `authSuspended`) survives; only the *call target* changes from `sheet.Heartbeat` to an HTTP POST. ~20 LOC touched. |
| NSIS installer, `HKCU\...\Run` autostart, `cmd/squirebot` `--quit` / `--uninstall-wipe-credentials` / console-detach | Installer is unchanged except the uninstall-wipe path (see §1.3). Autostart, shutdown IPC, console handling all survive. |
| `cmd/squirebot/main.go` orchestration skeleton | The goroutine layout (update.Apply → logging → config → tray → RunApp + shutdown listener + systray.Run) survives. The `auth.BuildConstants` block changes (see §1.3). |

**Survives with a one-field tweak — `internal/config`:** The `Config` struct
keeps `EQFolder(s)`, `LogLevel`, `LastKnown*Mtime`, `PendingUpdateVersion`,
`Version`. `SpreadsheetID` is **deleted**; `GoogleEmail` is **deleted**. A new
`BackendBaseURL` (and possibly a cached `DisplayName`/`GuildID`) is added. The
BOM-strip, atomic-save, and back-compat-shim logic is untouched. ~15 LOC.

### 1.2 DELETED ENTIRELY (~2,500–3,000 LOC including tests)

| Package / file | Reason |
|---|---|
| `internal/auth` (`oauth.go`, `oauthconfig.go`, `pkce.go`, `userinfo.go`, `refresh.go`, `browser.go`, `store.go` + all `*_test.go`) | The entire Google OAuth 2.0 loopback-PKCE machinery, `userinfo.email` lookup, `IsRevokedRefreshToken` classifier, and wincred-token store. **This package is the brand-verification dependency. Deleting it is the milestone's whole point.** Note: `browser.go` (OpenBrowser) and the wincred wrapper concept may be *retained in a slimmed form* for the new credential flow — see §1.3 / §2. |
| `internal/sheet` (`client.go`, `write.go`, `meta.go`, `owner.go`, `ensure_tab.go`, `retry.go`, `heartbeat.go`, `scaffold_helpers.go`, `client_helpers.go` + tests) | The whole Sheets v4 API client: atomic `batchUpdate` clear+write, tab-cache, `ValidateWorkbook`, `ErrSchemaTooNew`, `WatcherMaxSchemaVersion`, `_char_owner` upsert, schema scaffolding. Replaced by a ~200-LOC HTTP-POST client (see §1.4). |
| `internal/scaffold` (`scaffold.go`) | `ScaffoldSchemaV1` builds dimension/view tabs in the workbook. The backend now owns its own schema (a real DB). **Deleted.** |
| `internal/picker` (`server.go`, `picker_html.go`) | Google Drive Picker — only exists because `drive.file` scope grants per-file access and the user must point at the shared workbook. With a single backend there is no workbook to pick. **Deleted.** |
| `internal/wizard` (`server.go`, `pages.go`, `folderpicker_dialog.go`, `pages/*.html`) | The 4-step loopback-HTTP wizard is *mostly* deleted — the OAuth step and the picker step vanish. The EQ-folder discovery + native-folder-dialog step survives and is re-homed into a much simpler setup flow (see §1.3 / §2.4). ~60% deleted, ~40% re-homed. |
| `internal/app/reauth.go` + the post-reauth propagation probe in `runapp.go` (`runPostReauthProbe`, `swappableTS`, `globalReauthTSCh`, the `drive.file`-write-propagation 90-min probe) | All of this exists *solely* to cope with Google's `invalid_grant` revocation and the documented ~50-min `drive.file` write-access propagation delay. A backend API key does not silently expire or have a propagation delay. **Deleted — this is a meaningful complexity win, not just a line-count win.** |

### 1.3 CHANGES (re-wired, not rewritten — ~800–1,200 LOC touched)

| Area | Change |
|---|---|
| `cmd/squirebot/build_constants.go` + `main.go` | `OAuthClientID/Secret`, `PickerAPIKey`, `GCPProjectNumber` ldflags are deleted. The build no longer bakes in any Google secret. A backend base URL may be a build constant or config field. |
| `internal/app/runapp.go` | This is the orchestrator and takes the most edits. `buildTokenSourceFromWincred`, `swappableTS`, `applyBootAuthError`, `runPostReauthProbe`, `suspendForAuth`, and the `globalReauth*` / `globalPostReauthPending` machinery all go away. `needsWizard` becomes "is a backend credential present?" `runWatcher` keeps its shape: validate-then-watch, launch heartbeat goroutine, launch update goroutine, `rescanCatchUp`, `watch.Run`. The `makeOnInventoryChange` / `makeOnSpellbookChange` callbacks keep their **entire structure** (stat → open → parse → empty-check → upload → persist mtime → `cfg.Save`); only the upload call changes from `sc.WriteInventory(...)` to `backend.UploadInventory(...)`. The `_char_owner` `UpsertCharOwner` call is **deleted** (the backend derives ownership from the authenticated credential — see §2). |
| `internal/tray` | "Reauthorize…", "Change Workbook…", "Continue setup…" menu items lose their Google meaning. "Reauthorize" becomes "Re-enter guild code" only if a credential is ever rejected; "Change Workbook" is deleted; "Continue setup" stays (re-runs the simplified setup). Status strings change ("Connected as bob@gmail.com" → "Connected to <guild>"). |
| `internal/heartbeat` | Call target swap only (Sheets → HTTP POST `/api/heartbeat`). Scheduling shell intact. |
| `cmd/squirebot/main.go` `--uninstall-wipe-credentials` | Still wanted — wipes the locally-stored backend credential instead of the wincred Google token. ~10 LOC. |
| `internal/wizard` (re-homed remnant) | The EQ-folder discovery + `eqfind.ValidateFolder` + native folder dialog survive and move into the new setup flow. |

### 1.4 NEW CODE (~400–600 LOC + tests)

- **`internal/backend`** — a small HTTP client: `UploadInventory(ctx, char, rows, uploadedAt)`, `UploadSpellbook(...)`, `Heartbeat(...)`. Reuses the *retry/backoff philosophy* from `internal/sheet/retry.go` (exponential backoff, classify 401 vs 429 vs 5xx) but against plain `net/http` + JSON. Sends the per-guildie credential as a bearer header.
- **Credential store** — the wincred wrapper concept survives in slimmed form: store the backend API token in Windows Credential Manager under `SquireBot:backend` (DPAPI at rest). This is ~30 LOC vs. the deleted `auth/store.go`'s JSON-blob `StoredToken`.
- **Simplified setup flow** — replace the loopback OAuth+picker wizard with a single tray-driven prompt: "Paste your SquireBot guild code." (See §2.4.)

### 1.5 Size Estimate

| Bucket | Approx LOC |
|---|---|
| Survives unchanged | ~9,000–10,000 |
| Changes (re-wired) | ~800–1,200 touched |
| Deleted | ~2,500–3,000 (incl. tests) |
| New | ~400–600 (incl. tests) |

**Net: the watcher gets ~2,000 LOC smaller and materially simpler.** The
deleted code is the *highest-complexity* code in the project (PKCE, token
refresh, revocation classification, propagation-delay probes, Drive Picker JS).
This is a re-target, not a rewrite — the data pipeline (watch → debounce →
parse → upload) is untouched in shape; only the final hop changes.

---

## 2. Watcher ↔ Backend Auth Design

### 2.1 Requirement

Each of ~12 watchers must authenticate every upload to the backend so the
backend can (a) reject strangers and (b) attribute uploaded characters to a
guildie — replacing today's `OAuth userinfo.email` → `_char_owner` identity
model. The current architecture treats OAuth email as canonical identity; the
backend equivalent is **the credential itself maps 1:1 to a guildie row**.

### 2.2 Recommended: per-guildie opaque bearer token ("guild code")

- The maintainer runs a tiny admin action on the backend ("add guildie") which
  inserts a `members` row and mints a **high-entropy random token** (e.g. 32
  bytes base64url — the exact shape the existing `auth/pkce.go` /
  `newState()` already generate). Token is stored **hashed** (SHA-256 / argon2)
  in the backend DB; the plaintext is shown once to the maintainer.
- The maintainer hands the token to the guildie over Discord DM (the guild
  already coordinates there). Optionally a short human-friendly *claim code*
  (e.g. `SQUIRE-7H2K-9QXM`) that the watcher exchanges once for the real
  long-lived token — friendlier to paste, and the claim code can be
  single-use + short-TTL.
- The watcher sends `Authorization: Bearer <token>` on every
  `/api/inventory`, `/api/spellbook`, `/api/heartbeat` call.
- The backend resolves token → member row → that member owns the characters in
  the payload. **Character ownership is derived server-side from the
  credential** — the watcher no longer writes a `_char_owner` mapping, which
  deletes `UpsertCharOwner` and its whole class of "two watchers disagree on
  owner" edge cases.

**Why a static bearer token over the alternatives:**

- *Device tokens / per-install tokens (OAuth device-authorization grant):*
  Real OAuth device flow needs an identity provider and a polling endpoint —
  more infrastructure than a 12-person hobby project should run, and would
  re-introduce a consent screen. Overkill.
- *mTLS client certs:* Strong, but distributing and renewing client certs to
  non-technical guildies is worse UX than the OAuth flow we're trying to
  escape.
- *Username+password per guildie:* Requires the watcher to hold a password and
  the backend to do password hashing + login throttling for a *machine*
  client. A random bearer token is a password without the human-memorability
  baggage — strictly better for a daemon.

A static token's one weakness — no built-in expiry/rotation — is acceptable at
this scale and is fully mitigable: the backend can rotate a token on demand
(maintainer clicks "regenerate", DMs the new one; watcher shows "credential
rejected — paste new guild code"). That rejection path reuses the *exact* tray
UX shape the deleted `suspendForAuth` had, minus the propagation probe.

### 2.3 Client-side storage

Keep the **DPAPI-backed Windows Credential Manager** pattern that already
works. Store the bearer token under target `SquireBot:backend` via a ~30-LOC
slimmed `wincred` wrapper. Rationale unchanged from AUTH-04: never put the
credential in `config.json` (which is plaintext and hand-editable). This is the
one genuinely good idea from the deleted `internal/auth` that should be
preserved.

### 2.4 Onboarding without the OAuth browser flow

Today's onboarding is: installer → loopback wizard → **Google consent in
browser** → Drive Picker → EQ-folder dialog. The re-targeted flow is shorter:

1. Installer runs (unchanged, per-user, no UAC).
2. Watcher starts, sees no credential, tray goes red, opens a **tiny local
   prompt** (a one-field loopback HTML page reusing the wizard's
   listener/template machinery, OR a native input dialog): *"Paste your
   SquireBot guild code."*
3. Guildie pastes the code the maintainer DM'd them. Watcher POSTs it once to
   `/api/claim` (if using claim-code exchange) or validates it with a
   `/api/whoami` probe, then stores the token in wincred.
4. EQ-folder discovery + native-folder-dialog step (the surviving remnant of
   the old wizard) runs.
5. Done. Watcher goes green.

**No browser-based third-party consent screen exists anywhere in this flow.**
That is the structural win. The guildie's most complex action is a copy-paste
from Discord — strictly simpler than "click Allow on a Google consent screen,
then pick a file in the Drive Picker."

---

## 3. Website Login Options Compared

The website shows the guild's aggregated data. Who may view it, and how they
sign in. Audience is ~12 known people; the bar is "keep strangers out," not
"enterprise SSO."

| Option | How a guildie signs in | Infra the maintainer runs | Stranger-proof? | Re-introduces a 3rd-party verification gate? |
|---|---|---|---|---|
| **Discord OAuth2** (`identify` [+ `email`]) | "Login with Discord" → Discord consent → backend checks the user is in the guild's Discord server (via `guilds` scope or a bot membership check) | A Discord application (free, instant); optionally a bot token for membership check | **Yes** — gate on Discord-server membership; the guild *is* a Discord server | **No.** See §4. |
| **Magic-link email** | Enter email → backend emails a one-time link → click to log in | An SMTP/transactional-email sender (e.g. a free-tier provider); an allowlist of ~12 emails | Yes — allowlist | No third-party OAuth at all |
| **Single shared guild password** | Everyone types the same password | None beyond the backend | Weak — one leak compromises everyone; no per-user attribution; no revocation without rotating for all | No |
| **GitHub OAuth** | "Login with GitHub" → GitHub consent → backend allowlists ~12 GitHub usernames | A GitHub OAuth app (free, instant) | Yes — allowlist | No — GitHub OAuth apps need **no** verification/review for `read:user` |
| **"Sign in with Google"** | Google consent → backend allowlists ~12 emails | A Google Cloud OAuth client + consent screen | Yes — allowlist | **YES — this re-introduces exactly the gate the milestone exists to escape.** See §4. |

### 3.1 Recommendation: Discord OAuth2

- The guild **already lives in a Discord server** — Discord identity *is* guild
  identity. Membership-in-the-server is a natural, zero-maintenance allowlist:
  the maintainer never curates an email list; adding/removing a guildie is just
  adding/removing them from Discord, which already happens.
- It also seeds the deferred **v2 Discord pinger** (per-user Discord identity
  capture is already a listed v2 prerequisite) — choosing Discord auth now
  pays that prerequisite down for free.
- Discord OAuth2 for a login (`identify`, `email`, `guilds`) requires **no app
  review and no verification** to function — see §4.

Magic-link email is the solid fallback if the maintainer wants zero dependence
on Discord's API; it also avoids *all* third-party OAuth. Its only cost is
running an email sender and hand-maintaining a 12-entry allowlist. Shared
password is rejected (no attribution, no revocation). Google sign-in is
rejected outright (§4).

---

## 4. Brand-Verification Escape Analysis (the critical question)

**The milestone's explicit goal is to escape Google's OAuth brand-verification
gate. Each website-login option must be checked for whether it re-introduces a
verification gate.**

### 4.1 "Sign in with Google" on a website — DOES re-introduce the gate

Confirmed against Google's current (May 2026) documentation: *"If your
application utilizes Sign in with Google for authentication only, you must
submit your application for brand verification."* Brand verification is
triggered when **both**: (a) the OAuth client's user type is **External**, and
(b) the consent screen **displays a logo or display name**.

Critically, **brand verification verifies the domains** behind the consent
screen's home-page / privacy-policy / ToS / redirect URIs — and that domain
verification is *exactly* the gate that is currently blocking SquireBot (the
existing incident: GitHub Pages won't pass; a registrar-owned domain is
required). "Authentication only" / "no sensitive scopes" reduces the *scope*
audit, but it does **not** exempt you from **brand** verification once you show
branding to External users.

There is a narrow technical escape — configure the consent screen to display
**no logo and no display name** — but Google's flow strongly pushes branding,
the UX is poor (an unbranded "a third-party app" consent screen), and it is a
fragile, against-the-grain configuration that Google can tighten. **Verdict:
choosing Google sign-in for the website re-creates the precise problem this
milestone exists to eliminate. Reject it.**

### 4.2 Discord OAuth2 — does NOT re-introduce any verification gate

- Discord OAuth2 for **logging a user into a website** (scopes `identify`,
  `email`, `guilds`) works the moment you create a Discord application. There
  is **no brand-verification step and no app-review step** for OAuth login.
- Discord's "verification" is a *separate, bot-only* concept: a **bot** must be
  verified to join more than 100 servers, and apps must be verified to be
  *listed in the App Directory*. **Neither applies here** — SquireBot's website
  is an OAuth *client* logging users in, not a bot joining servers, and it does
  not need App Directory listing. No 100-server threshold is in play.
- Discord's consent screen shows the app name/icon you set; there is no
  domain-ownership verification of your privacy-policy/home-page URLs as a
  precondition to functioning.
- **Verdict: Discord OAuth2 fully avoids any third-party verification gate.**

### 4.3 The other options

- **Magic-link email:** no OAuth provider at all → **no verification gate of
  any kind.** The only "gate" is whatever a transactional-email provider asks
  for (domain/sender verification for deliverability), which is unrelated to
  OAuth consent screens and does not block the auth flow from working.
- **GitHub OAuth:** GitHub OAuth Apps require **no verification or review** to
  function for basic `read:user` login. (GitHub *Apps* have a separate
  optional verification badge; standard OAuth Apps do not need it.) → **no
  gate.**
- **Single shared password:** no third party → no gate (but rejected on
  security/attribution grounds in §3).

### 4.4 Definitive answer

**The recommended path — Discord OAuth2 for website login + a per-guildie
opaque bearer token for the watcher — fully eliminates every OAuth
brand-verification dependency.** The watcher side deletes Google OAuth entirely
(no consent screen exists). The website side uses Discord, which has no
brand-verification or app-review gate for OAuth login. There is **no residual
Google dependency** and **no third-party verification gate** anywhere in the
recommended design. The *only* option that would re-introduce the gate is
"Sign in with Google," and it is explicitly rejected. Magic-link and GitHub
OAuth are also gate-free fallbacks.

---

## 5. Migration of Existing Watchers

~12 guildies already run installed watchers authenticated to Google. They must
transition to the re-targeted watcher. The migration vehicle already exists and
survives the re-target untouched: **`internal/update` (GitHub Releases +
`minio/selfupdate` + SHA-256-verified swap).**

### 5.1 Mechanics

1. **Backend stands up first** and is seeded with the ~12 `members` rows;
   each guildie's bearer token (or claim code) is minted.
2. **Maintainer DMs each guildie their guild code** over Discord (the channel
   already used for support).
3. **A new watcher version is published** to GitHub Releases with a
   `binary_url` + `binary_sha256` in `latest.json`. The existing
   `update.RunDailyCheck` goroutine on every running watcher picks it up within
   24 h, stages `<exe>.new`, and `update.Apply` swaps it in on next launch —
   **no guildie action required for the binary itself.**
4. On first launch of the re-targeted binary, the watcher finds **no backend
   credential** (the old wincred entry is a Google token under
   `SquireBot:<email>`, not `SquireBot:backend`), so it goes red and shows the
   one-field "paste your guild code" prompt (§2.4). The guildie pastes the code
   from Discord. Done.

### 5.2 Concerns to flag

- **Auto-update pushes the binary; it cannot push a credential.** Every guildie
  *must* perform exactly one manual step (paste the guild code). This is
  unavoidable and is the *entire* manual cost of migration — acceptable for 12
  known people, and simpler than the original OAuth onboarding they already
  completed once.
- **Schema-version interlock changes meaning.** Today `WatcherMaxSchemaVersion`
  / `_meta.schema_version` gate watcher-vs-sheet compatibility. Post-migration
  the backend should expose an API version; the watcher should send its
  version and the backend should reject-with-clear-message a watcher that is
  too old — preserving the "old watcher refuses to corrupt data" guarantee in
  a new venue. The new watcher must handle a stale pre-migration peer
  gracefully during the rollout window.
- **Cleanup of the old Google credential.** The stale `SquireBot:<email>`
  wincred entry and the now-dead `config.json` fields (`spreadsheet_id`,
  `google_email`) should be wiped on first re-targeted launch — a tiny
  one-shot migration in `config.Load` / a startup step. Low risk, do it for
  hygiene.
- **The Google Sheet does not have to die on day one.** The old workbook can
  remain read-only during the cutover so nothing is lost while guildies
  trickle through the 24-h auto-update window. Decommission it once all 12 are
  confirmed on the new backend.
- **Rollback:** because the swap is `minio/selfupdate` (which keeps `<exe>.old`)
  and the backend is additive, a botched release can be rolled back by
  publishing a corrected `latest.json`. Keep the backend tolerant of both old
  and new watchers for the rollout window.

---

## 6. Recommendation

1. **Watcher re-target is a moderate, well-bounded change, not a rewrite.**
   ~9–10k LOC of file-watching / parsing / tray / update / installer survives
   untouched. Delete `internal/auth`, `internal/sheet`, `internal/scaffold`,
   `internal/picker`, most of `internal/wizard`, and all the reauth /
   propagation-probe machinery in `internal/app` (~2.5–3k LOC, the
   *highest-complexity* code in the project). Add a ~400–600 LOC
   `internal/backend` HTTP client. Net: the watcher gets smaller and simpler.

2. **Watcher ↔ backend auth: per-guildie opaque bearer token** ("guild code"),
   minted by the maintainer, stored hashed server-side, delivered over Discord
   DM, stored client-side in DPAPI-backed Windows Credential Manager (keep the
   one good idea from `internal/auth`). Optionally front it with a single-use
   short claim code for friendlier pasting. Character ownership is derived
   server-side from the credential — `_char_owner`/`UpsertCharOwner` is deleted.

3. **Website human login: Discord OAuth2** (`identify` + `guilds`), gating on
   membership of the guild's existing Discord server. Zero allowlist
   maintenance, and it pre-pays a v2 Discord-pinger prerequisite. Magic-link
   email is the gate-free fallback if Discord-API dependence is unwanted.

4. **Brand-verification escape — definitive:** the recommended design
   (Discord login + bearer-token watcher) **fully eliminates every OAuth
   brand-verification dependency.** No Google consent screen exists anywhere;
   Discord OAuth login has no brand-verification or app-review gate. The *only*
   option that re-introduces the gate is "Sign in with Google" — **reject it.**

5. **Migration:** ship the re-targeted binary through the existing
   GitHub-Releases auto-update pipeline (no per-guildie binary action). Each
   guildie performs exactly one manual step — paste the Discord-DM'd guild code
   once. Keep the old Sheet read-only through the 24-h rollout window, then
   decommission.

### Sources

- [Google — Submit for brand verification](https://developers.google.com/identity/protocols/oauth2/production-readiness/brand-verification)
- [Google — OAuth App Verification Help Center](https://support.google.com/cloud/answer/13463073)
- [Google — Comply with OAuth 2.0 policies](https://developers.google.com/identity/protocols/oauth2/production-readiness/policy-compliance)
- [Discord — OAuth2 documentation](https://docs.discord.com/developers/topics/oauth2)
