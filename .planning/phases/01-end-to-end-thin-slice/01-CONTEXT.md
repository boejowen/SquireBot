# Phase 1: End-to-End Thin Slice — Context

**Gathered:** 2026-04-30
**Status:** Ready for planning

<domain>
## Phase Boundary

A single guildie installs SquireBot via a one-click `.exe`, completes Google OAuth once, picks the shared guild workbook via the Drive Picker, confirms (or browses to) their EQ folder, runs `/outputfile inventory` in P99, and within ~30 seconds sees raw five-column TSV rows appear in an `inv:<CharName>` tab of the workbook. Validates three of five existential pitfalls before any other phase compounds: (1) OAuth Production-mode token survival past 7 days, (2) SmartScreen "Unknown publisher" walkthrough, (3) `drive.file` Picker semantics. No views, no scrapes, no enrichment, no spellbook handling, no auto-update — those are later phases.

</domain>

<decisions>
## Implementation Decisions

### Workbook Onboarding

- **D-01 [informational]: The shared guild workbook is created from a publicly-shared template via "Make a copy."** I (the dev) maintain a master Sheet at a stable URL with the SquireBot Apps Script and starting schema preinstalled. The guild leader clicks "Make a copy" once, ending up with their own private workbook in their Drive. This avoids a chicken-and-egg where SquireBot would need to install itself just to bootstrap the workbook, and it lets schema/script updates flow to existing guild copies via Apps Script self-update later. *(Out of Phase 1 code scope — manual setup task performed by dev/guild-leader. Plan 05's `bootstrapMeta` is the defensive fallback for empty `_meta` cells.)*
- **D-02 [informational]: Workbook sharing model is "anyone with the link → view-only" PLUS each guildie's Google email added as an editor.** View-only link sharing means non-installers (casual guildies on a phone, browser-only users) can still see the bank, gear checks, etc. Edit access is per-email so each watcher's OAuth identity is bound to a real human, and so the workbook is not vandalize-able by random link-haters. *(Out of Phase 1 code scope — workbook sharing is configured by the guild leader in Drive UI, not by the watcher.)*
- **D-03: SquireBot validates the picked workbook is actually a SquireBot workbook before storing the spreadsheet ID.** On first connect after Picker, the watcher reads `_meta.canonical_id` (a fixed marker the template carries). Missing or wrong → reject with: "This doesn't look like a SquireBot workbook. Pick the one shared by your guild leader." Does not retry-loop.
- **D-04: Switching workbooks later is a tray menu item ("Change Workbook…") not a force-reinstall.** Phase 1 minimum: the menu item exists and re-runs Picker. Heavy lift (full state reset) can be Phase 2 polish if needed.
- **D-05: Product is a Google Sheet, not a Google Doc.** Confirmed terminology — all architecture is workbook-centric.

### First-Run UX

- **D-06: After the installer finishes, SquireBot auto-launches a setup wizard window.** Four steps: (1) Connect Google (OAuth), (2) Pick the guild workbook (Drive Picker), (3) Confirm EQ folder, (4) Done. After step 4 the wizard shows a "✓ You're all set" screen for ~3 seconds and minimizes to tray with a one-shot toast notification. This is more idiot-proof than a silent tray-only install — a guildie installing for the first time sees a window walking them through every required action.
- **D-07: The wizard is dismissible mid-flow and resumable.** If the user closes the wizard at step 2, the tray icon shows red "Setup needed" and the tray menu has "Continue setup…". On any error in any step, the wizard surfaces a clear error and offers retry; it does not collapse all progress.
- **D-08: Wizard tech is Claude's discretion** — Go GUI library or local webview, whichever produces a small binary and a clean visual. Plan phase will pick. *(RESOLVED in Plan 07: HTML pages served from the loopback HTTP server — reuses Plan 03's listener, no native UI runtime dep, single-binary install preserved.)*

### EQ Folder Discovery & Fallback

- **D-09: Auto-discovery runs first, picker is the fallback.** Order: (a) `~/.config/squirebot/config.json` if a prior install left state, (b) known paths (`C:\P99`, `C:\Project1999`, `C:\Games\Project1999`, etc.), (c) registry uninstall keys for "Project 1999" / "EverQuest", (d) recursive heuristic scan for a folder containing both `eqgame.exe` and `eqclient.ini`. If all four fail, the wizard's step-3 "EQ folder" pane shows a "We couldn't find your EverQuest folder — pick it" message with a button that opens Windows' native folder-picker dialog.
- **D-10: Picked folder is validated to contain `eqgame.exe`.** No `eqgame.exe` → "This folder doesn't look like an EverQuest install (no eqgame.exe found). Pick a different folder." Does not silently accept and break later.
- **D-11: Phase 1 is single-folder.** Multi-folder support for multiboxers (WATCH-03) is Phase 2; the wizard accepts exactly one folder in Phase 1.

### Distribution & Update Channel

- **D-12: SquireBot.exe is hosted on public GitHub Releases.** Repo is public. Releases page is the canonical install URL. `goreleaser` produces the binary and the `latest.json` manifest used by Phase 2's auto-updater.
- **D-13: Phase 1 ships unsigned.** Code-signing certificate procurement is a Phase 2 research flag; Phase 1's "clean Win11 VM install" success criterion will hit SmartScreen, and the documented "More info → Run anyway" walkthrough is what gets exercised at this stage. This is intentional — Phase 1 is the dev validating the install flow on their own machine, not a guild rollout.
- **D-14: A minimal README accompanies the GitHub repo.** Download link, the SmartScreen walkthrough, the OAuth flow, the EQ folder picker, "tray turned red, what now?" — populated lightly in Phase 1 and expanded in Phase 5.

### Claude's Discretion

- Wizard library / framework choice (Go-native UI vs. embedded webview) — pick during planning. Constraint: must produce a single-binary install with no runtime dependencies.
- Tray menu surface in Phase 1: minimum is `Status` (read-only string), `Open Workbook` (opens the configured Sheet in a browser), `Continue setup…` (only when needed), `Quit`. Anything richer can wait.
- Wizard's "you're all set" dismissal animation, toast timing, etc. — pick sensibly.
- Specific style of the Picker's "Shared with me" guidance (screenshot, link, or just trust the user) — pick what fits the wizard's visual style.
- Validation error copy in D-03 / D-10 — match the rest of the wizard's tone.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project-level (mandatory)
- `.planning/PROJECT.md` — Core Value, Active requirements, Out of Scope, Key Decisions (consolidated views, OAuth Production flip, manual plat entry, etc.)
- `.planning/REQUIREMENTS.md` — All 69 v1 requirement IDs; Phase 1 owns 12 of them (INST-01..03, AUTH-01..04+06, WATCH-01, WATCH-04, OPS-01, OPS-03)
- `.planning/ROADMAP.md` — Phase 1 goal, success criteria, dependencies; the ringfence for what's in vs. out

### Stack & architecture
- `.planning/research/STACK.md` — locked library versions (Go 1.24.x, fsnotify v1.7+, wincred v1.2.x, NSIS 3.10+, Sheets API v4 client, oauth2/google), the "what NOT to use" list, OAuth pattern (loopback + PKCE + drive.file + 127.0.0.1 literal + manual-paste fallback)
- `.planning/research/SUMMARY.md` — synthesized roadmap-level overview; the canonical override stating consolidated view tabs (per-character views are SUPERSEDED in `ARCHITECTURE.md`)
- `.planning/research/ARCHITECTURE.md` — three-layer pancake schema, `_meta`, `_char_owner`, watcher write contract (`batchUpdate` clear+write, `valueInputOption=RAW`), identity-via-OAuth-userinfo-email rule. **Note:** the per-character view-tab proposal in this doc is superseded by SUMMARY.md's consolidated decision.

### Pitfalls Phase 1 must specifically design against
- `.planning/research/PITFALLS.md` — pitfall #1 (OAuth Testing-mode 7-day expiry; flip to Production), pitfall #2 (SmartScreen wall; documented walkthrough acceptable for Phase 1), pitfall #5 part (drive.file Picker semantics; Picker is mandatory not optional). Pitfalls #3 and #4 (concurrent writes, stale-data trust collapse) belong to Phase 2 schema lock; Phase 1 only writes one watcher to one tab so they're mostly inert here, but the per-character non-overlapping-range pattern (OPS-01) is established now.
- `.planning/research/FEATURES.md` — feature taxonomy and gap analysis; informs scoping but is mostly a Phase 3+ reference.

### External APIs (touched in Phase 1 only minimally — full use is Phase 3)
- Google Sheets API v4: `spreadsheets.batchUpdate` semantics — https://developers.google.com/workspace/sheets/api/reference/rest/v4/spreadsheets/batchUpdate
- Google OAuth 2.0 desktop loopback flow — https://developers.google.com/identity/protocols/oauth2/native-app
- Google `drive.file` scope — https://developers.google.com/workspace/sheets/api/scopes
- Google OAuth consent screen Production status — https://support.google.com/cloud/answer/15549945

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

Greenfield project. Phase 1 starts from a `git init` directory. No reusable Go modules, components, or utilities exist yet.

### Established Patterns

None yet. Phase 1 establishes the patterns later phases inherit:
- Go module layout: `cmd/squirebot/main.go` + `internal/<feature>/...` subpackages (`watch`, `parse`, `sheet`, `auth`, `tray`, `wizard`)
- Logging: `log/slog` everywhere, output to `%LOCALAPPDATA%\SquireBot\squirebot.log` via `lumberjack.v2`
- Config file: `%LOCALAPPDATA%\SquireBot\config.json` (NEVER stores refresh token)
- Errors: surface in tray via colored icon + tooltip; never silent

### Integration Points

External integrations introduced in Phase 1:
- Google OAuth 2.0 endpoints (loopback redirect on a random ephemeral port `49152..65535`, PKCE)
- Google Drive Picker API (workbook selection)
- Google Sheets API v4 (`spreadsheets.batchUpdate` only — clear range + write range)
- Windows Credential Manager (via `wincred`) for refresh token under target name `SquireBot:<google-email>`
- Windows Registry under `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` for autostart

</code_context>

<specifics>
## Specific Ideas

- **Workbook sharing semantics:** the link-share-view-only model is unusual for "guild internal tooling" but explicitly chosen — a casual guildie on a phone should be able to glance at the bank without installing anything or being added as an editor.
- **"Idiot-proof" defines success as much as the success criteria do:** the user's words were "install the app, maybe give Windows permission, give Google Drive a permission or two, and that's it." Phase 1's wizard should literally be those four taps and nothing more.
- **The dev is a guildie of this guild.** The "developer" using SquireBot to validate Phase 1 is the same person who will onboard the other 11 guildies in Phase 5. There's no separate QA path.
- **`drive.file` is non-negotiable.** The Picker step exists *because* of the scope choice; resist any future plan that proposes pre-filling a sheet ID via config to skip the Picker — that path returns `403` despite OAuth success.

</specifics>

<deferred>
## Deferred Ideas

- **Multi-folder watcher support for multiboxers** — explicitly deferred to Phase 2 (WATCH-03). Phase 1 wizard accepts exactly one folder.
- **Code-signing certificate (EV vs OV vs unsigned-with-walkthrough)** — Phase 2 research flag. Phase 1 ships unsigned.
- **Auto-updater plumbing** — Phase 2 (OPS-04). Phase 1's GitHub Releases hosting just establishes the URL shape that Phase 2 builds on top of.
- **Spellbook watcher** — Phase 2 (WATCH-02).
- **Daily heartbeat write** — Phase 2 (WATCH-08).
- **Refresh-token failure UX (`invalid_grant` → tray red → reauth)** — Phase 2 (AUTH-05).
- **Sheets-API retry/backoff (2/4/8/16/32/60s exp; honor `Retry-After`)** — Phase 2 (WATCH-07).
- **Catch-up on watcher restart** — Phase 2 (WATCH-09).
- **Schema dimension/view tab creation** — Phase 2 (SCHEMA-01..08); Phase 1 only writes to `inv:<Char>` (and minimally bootstraps `_meta.canonical_id` + `_char_owner` for identity).
- **Multi-character / multi-account on the same PC** — supported by the file-watching design, but not the focus of Phase 1 validation.
- **Richer wizard styling, animations, branding** — Phase 1 should be functional and unmistakable; aesthetic polish is Phase 5 onboarding work.

</deferred>

---

*Phase: 1 — End-to-End Thin Slice*
*Context gathered: 2026-04-30*
