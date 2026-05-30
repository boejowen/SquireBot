---
phase: 13-watcher-re-target-onboarding
created: 2026-05-30
mode: plan-now (Claude surfaced the forks from 13-RESEARCH.md; user locked FORK 1 + FORK 2/3; remaining decisions = research-derived defaults)
requirements: [WATCH-08, WATCH-09, WATCH-10, WATCH-11]
---

# Phase 13: Watcher Re-Target + Onboarding — CONTEXT

<domain>
Re-point the existing Go watcher from Google Sheets to the v2.0 backend HTTP API (LIVE at `https://api.squirebot.quest`, the P11 `POST /api/v1/ingest` bearer-token endpoint), DELETE the entire Google OAuth/PKCE/Sheets/Drive-Picker stack (~8k LOC across 5 packages — the highest-complexity code in the project), and replace the loopback-OAuth wizard with a one-field **native "paste your guild code"** onboarding, shipped to the ~12 guildies via the existing GitHub-Releases auto-updater. Includes a small (~50-LOC) backend addition (`GET /api/v1/whoami` + a min-watcher-version reject) that ships in one redeploy. This restores the live data pipeline end-to-end (watcher → backend → self-populating DB from P12).
</domain>

<decisions>
Forks surfaced from 13-RESEARCH.md; FORK 1 + FORK 2/3 locked by the user (2026-05-30); the rest are research-derived defaults for a faithful, deletion-heavy re-target.

- **D-1 — SINK swap on an unchanged pipeline.** `internal/watch` (fsnotify 500ms debounce + always-re-read) + the re-stat/re-read flow survive verbatim; only the write SINK changes from Sheets `batchUpdate` to a new ~150-LOC `internal/backend` POST client. **The watcher gets THINNER: it sends the RAW (CP1252-decoded-to-UTF-8) file text as `content` and the BACKEND parses it (the D-03 contract) — the watcher STOPS calling `parse.Parse` on the upload path entirely.** Do NOT parse client-side and send rows (breaks the contract + duplicates parsing truth).

- **D-2 — Deletion map (the load-bearing work).** DELETE `internal/auth` (OAuth/PKCE), `internal/sheet` (Sheets client + the `WatcherMaxSchemaVersion` gate), `internal/scaffold` (workbook scaffolding), `internal/picker` (Drive picker), and the wizard's OAuth/loopback-HTTP/Drive parts. **PRESERVE (relocate as needed):** the wizard's EQ-folder selection (sqweek folder dialog `folderpicker_dialog.go::defaultPickFolder`) + `eqfind` validation + `cfg.Save`; the wincred DPAPI store from `internal/auth/store.go` (repurposed for the guild code, D-6). **DO NOT delete `internal/parse`** — the backend imports it. Rewire the only 3 non-test call sites: `cmd/squirebot/main.go`, `internal/app/runapp.go`, `internal/app/reauth.go` (reauth.go is OAuth-specific → DELETE). Strip the Google client-secret `ldflags` from `build_constants.go` + `.github/workflows/release.yml`.

- **D-3 — Onboarding UX = NATIVE WINDOWS INPUT DIALOG [USER-LOCKED, FORK 1 Option A].** Tray menu item "Enter guild code…" opens a small native Win32 text-input dialog (`golang.org/x/sys/windows`; `console_windows.go` already uses `windows.NewLazySystemDLL`). **NO browser, NO loopback server — DELETE the wizard's HTTP server + all HTML (`server.go`, `start.html`, `done.html`, `eq-folder.html`)** (honors WATCH-09 "shed the loopback/OAuth stack"). The EQ-folder step runs as a second native step via sqweek's existing folder dialog. On paste → `backend.Validate(code)` (GET `/api/v1/whoami`, D-4) → 200: store in wincred + go green; 401: re-prompt. Also re-prompt on a 401 during normal operation. (Cost: ~80–120 LOC of new dialog code — the one piece of genuinely new UI; accepted to keep zero localhost surface.)

- **D-4 — Backend addition ALLOWED in P13 (one redeploy) [USER-LOCKED, FORK 2+3 Option A].** Add to the live backend: **(a) `GET /api/v1/whoami`** — authed, reuses `auth.ResolveToken` verbatim, returns `200 {owner_label}` on a valid code / `401` otherwise (the onboarding validation endpoint; side-effect-free; also pre-pays P15). **(b) A min-watcher-version gate in the ingest handler** — a `MIN_WATCHER_VERSION` const + after decode, `if IsOlder(env.WatcherVersion, MIN)` → `426 Upgrade Required` with a clear message ("Your SquireBot is too old; it will auto-update shortly."). The `Envelope.watcher_version` field + `inventory_item.watcher_version` column + handler storage ALREADY ship ("gated in P13") — so this is only the *reject* (~30 LOC + a 3-part version-compare helper, porting `update/manifest.go::parseVersion` server-side or into `buildinfo`). **This faithfully satisfies SC-5.** Both additions ship with the P12 DEPLOY-PENDING redeploy (one trip).

- **D-5 — `internal/backend` client design.** `POST {backend_base_url}/api/v1/ingest`, `Authorization: Bearer <guild-code>` (read from wincred, cache in memory after first read), JSON body `Envelope{Character, Kind, Content, WatcherVersion}` (`content` = UTF-8 CP1252-decoded raw `/outputfile` text; `watcher_version` travels in BOTH the body field and a UA/header — belt-and-braces, the gate can read either). Base URL = config field `backend_base_url` (default `https://api.squirebot.quest` via a hardcoded fallback const). **Error classification:** `204→nil`; `401→ErrUnauthorized` (re-prompt onboarding); `409→ErrCrossOwner` (log + surface); `426→ErrVersionTooOld` (tray "update needed" — let the auto-updater handle); `400/422→ErrBadPayload` (log, shouldn't happen); `5xx`/network`→retryable`. **Retry:** bounded — 3 attempts `[1s,2s,4s]` on retryable only; NEVER on 401/409/426. The idempotent full-snapshot replace makes re-POST safe (no unbounded loop).

- **D-6 — Token storage (WATCH-10).** The guild code lives ONLY in DPAPI wincred — NEVER in `config.json` (the v1 "no secret in config" rule carries over verbatim). Reuse/rename the surviving wincred helper from `internal/auth/store.go` into a small `credstore` for the guild code (`Store`/`Read`/`Delete`).

- **D-7 — Auto-update migration (WATCH-11).** On first launch of the re-targeted binary after the GitHub-Releases auto-update: `credstore.Read()` finds no backend credential → trigger the native onboarding prompt (D-3) ONCE (the "one manual step" of WATCH-11). Clean up stale v1 state: DELETE the old Google OAuth-refresh-token wincred entry + drop dead `config.json` fields (OAuth client config, `spreadsheet_id`, `email`, token source). The `internal/update` selfupdate transport is unchanged (direct `net/http` to the GitHub CDN — never Google).

- **D-8 — A1 encoding: decode ONCE.** CP1252→UTF-8 decode happens exactly once on the watcher read side via `parse.CP1252Reader` (the `runapp.go` disk-read sites already wrap); the new backend client POSTs the resulting UTF-8 `content`; the server does NOT re-decode. Do NOT double-decode.

- **D-9 — Ride-along nits (now in scope, all small).** 999.20 (`console_windows.go` not `gofmt -l` clean), 999.21 (`freeConsole()` doc-vs-impl mismatch — both in `console_windows.go`), and **999.22 (SemVer-aware auto-update comparison in `internal/update/manifest.go` — `IsNewer`/`parseVersion` does a strict 3-part numeric compare with NO pre-release handling; LOAD-BEARING for the P16 coordinated self-update flip)**. Fix all three.

- **D-10 — DROP the heartbeat.** `internal/heartbeat` wrote a liveness signal to the Sheet's `_status` tab; there is no backend `_status` consumer. Remove it (planner: confirm the Sheet was its only consumer before deleting).
</decisions>

<canonical_refs>
MUST read before/while planning (full relative paths):
- `.planning/phases/13-watcher-re-target-onboarding/13-RESEARCH.md` — the deletion map (file → delete | preserve-which-parts | rewire), the `internal/backend` client design, the resolved forks, the token-storage + auto-update-migration sequences, the Runtime State Inventory (the stale per-machine OAuth wincred entry is the key migration item). **Primary ref.**
- `.planning/ROADMAP.md` §Phase 13 — goal + the 5 success criteria + the Note (~2.5–3k LOC deleted; "old watcher refuses to corrupt data" survives the move to API versioning).
- `.planning/REQUIREMENTS.md` — WATCH-08, WATCH-09, WATCH-10, WATCH-11.
- **The ingest contract you target (P11):** `internal/backendsrv/ingest/envelope.go` (the `Envelope{character, kind, content, watcher_version}` shape — `watcher_version` already accepted), `internal/backendsrv/ingest/handler.go` (where the version-gate reject lands; `ReplaceInventoryTx(..., env.WatcherVersion)` already stores it), `internal/backendsrv/auth` (`ResolveToken` — reused unchanged by `/whoami`).
- **Watcher survivors to reuse:** `internal/watch` (the pipeline), `internal/parse` (UTF-8 parser + `CP1252Reader` — NOT deleted), `internal/app/runapp.go` (the watch→sink wiring; the Sheets write is the sink to replace), `internal/config`, `internal/tray`, `internal/update/manifest.go` (999.22).
- **Deletion targets:** `internal/auth`, `internal/sheet`, `internal/scaffold`, `internal/picker`, `internal/wizard` (PRESERVE `folderpicker_dialog.go` + the EQ-folder logic). `internal/app/reauth.go` (delete). `cmd/squirebot/main.go` + `cmd/squirebot/build_constants.go` (rewire; strip OAuth ldflags).
- **Onboarding dialog basis:** `internal/wizard/folderpicker_dialog.go` (sqweek), `golang.org/x/sys/windows` (Win32 input dialog), `internal/system/console_windows.go` (existing `windows.NewLazySystemDLL` usage + the 999.20/21 nits).
- `.github/workflows/release.yml` — confirm + strip the Google OAuth client-secret `ldflags` (research assumption A5 — planner MUST read).
</canonical_refs>

<code_context>
- The backend is LIVE (P11) at `https://api.squirebot.quest`; the SQLite DB self-populates dimension data on cadence (P12, deploy-pending). P13 makes the WATCHER feed it.
- The watcher pipeline (`watch` → re-read → CP1252-decode) is proven v1 code; this phase swaps its terminal sink and deletes the Google auth/output half.
- The deleted packages are imported by only 3 non-test files (`cmd/squirebot/main.go`, `internal/app/runapp.go`, `internal/app/reauth.go`) — a bounded rewire.
- `Envelope.watcher_version` + `inventory_item.watcher_version` + handler storage already exist (P11), explicitly "gated in P13" — D-4's reject is the missing half.
- Tests run on the Windows dev box (`go test ./...`); the watcher is Windows-only (Win32 dialog, DPAPI, console).
</code_context>

<deferred>
- The **P12 DEPLOY-PENDING redeploy now BUNDLES with P13's backend additions** — one redeploy ships goose `00003` (P12) + `/whoami` + the version-gate (P13). Coordinate the deploy once P13's backend half is built.
- The actual guild-wide watcher flip (all ~12 watchers onto the backend) is **P16** (CUTOVER-03, the coordinated self-update) — P13 only SHIPS the re-targeted binary to GitHub Releases + proves a clean install works. Do not scope the coordinated flip here.
- Discord OAuth2 website login / admin web forms = P15. `/whoami` here is watcher-validation only (though it pre-pays a P15 need).
</deferred>

<open_for_planner>
1. The exact Win32 input-dialog implementation (a `DialogBox` template vs a `walk`-style helper vs the least-code pragmatic path) — research flagged ~80–120 LOC; planner picks the cleanest robust approach using the already-present `golang.org/x/sys/windows`.
2. Confirm `internal/heartbeat`'s only consumer was the Sheet `_status` before deleting (D-10).
3. The initial `MIN_WATCHER_VERSION` value + where the 3-part version-compare helper lives (port `update/manifest.go::parseVersion` into `buildinfo`, or share it) — coordinate with the 999.22 SemVer fix so there's ONE version-compare truth.
4. Whether any survivor (e.g. parts of `internal/system`/`tray`) references a deleted package and needs a small rewire beyond the 3 known call sites.
</open_for_planner>
