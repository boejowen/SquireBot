# Requirements: SquireBot v2.0 — "Off Google" (Website Frontend)

> v1.0 shipped 2026-05-11 (69 effective requirements; `milestones/v1.0-REQUIREMENTS.md`). v1.0.1 shipped 2026-05-12 (8 requirements; `milestones/v1.0.1-REQUIREMENTS.md`). v1.0.2 shipped as binary `v1.0.2` 2026-05-13 (6 requirements; milestone close superseded by this pivot). This file is scoped to v2.0 only.

**Milestone goal:** Replace the shared Google Sheet (both UI and data store) with a self-hosted Go + SQLite backend and a static web frontend — permanently eliminating the Google OAuth dependency that currently blocks the guild.

**Why now:** Google's brand-verification gate rejected the OAuth client 2026-05-15 ("home page not registered to you" — `github.io` can't satisfy it), walling off all ~12 guildie watchers. v2.0 removes Google from the system entirely rather than renting a domain to placate it. Full research: `.planning/explorations/website-milestone/SCOPE.md` + 4 findings docs.

**Stack (LOCKED + costed, ≈ $67/yr total):** Hetzner Cloud shared-vCPU VPS (US, amd64) backend (~$55/yr) · SQLite ($0) · SvelteKit static on Cloudflare/GitHub Pages ($0) · Discord OAuth2 website login ($0) · per-guildie opaque bearer token for watcher↔backend ($0) · ~$12/yr website-only domain.

**Architecture impact:** The `_meta.schema_version` ↔ `WatcherMaxSchemaVersion` handshake retires in favour of forward-only DB migrations (`goose`) + an API version. Google's 200-tab limit disappears; the 4 view tabs become SQL/API endpoints; the watcher sheds ~2.5–3k LOC of OAuth/Sheets/Picker code.

---

## v2.0 Requirements

### Backend — self-hosted server (BACKEND)

- [x] **BACKEND-01**: The backend runs on a self-hosted always-on VPS (currently a Hetzner Cloud VPS, US) with Caddy auto-HTTPS, reachable at the website domain over TLS. Single Go binary; in-process scheduler for cron jobs. _(HALF delivered + test-proven in 11-05: `cmd/squirebot-server` is a single Go binary serving `POST /api/v1/ingest` on loopback with an in-process `time.Ticker` scheduler skeleton; static `linux/amd64` cross-compile verified. The remaining half — on-box VPS provisioning + Caddy auto-HTTPS + systemd + TLS reachability — delivered + verified LIVE in 11-06: serving over HTTPS at `api.squirebot.quest` from the Hetzner US VPS with a valid Let's Encrypt cert, systemd `Restart=always` with reboot-survival proven.)_ ✅ Plans 11-05 + 11-06 (2026-05-29)
- [x] **BACKEND-02**: A SQLite schema with `goose` forward-only migrations models owners, characters, inventory items, spellbook entries, and dimension/enrichment data. `owner` and `character` are separate tables (owner-email change is a one-row update; no first-write-wins conflict logic). ✅ Plan 11-02 (2026-05-29)
- [x] **BACKEND-03**: An ingest endpoint accepts a watcher's full-snapshot inventory or spellbook upload and atomically replaces that character's rows (mirrors the v1 clear+write contract; never row-diffs). _(Atomic-replace tx + first-sighting bind delivered in 11-03; the `POST /api/v1/ingest` HTTP surface composing them — guard-first, one-tx bind+replace, round-trip test-proven via httptest — wired in 11-05.)_ ✅ Plan 11-05 (2026-05-29)
- [x] **BACKEND-04**: Each guildie authenticates to the ingest API with an opaque per-guildie bearer token ("guild code"), minted by the maintainer and stored hashed server-side. _(Mint/revoke CLI logic + `resolveToken` bearer guard delivered in 11-04; the `Authorization: Bearer` HTTP transport + 401-writes-nothing wiring — guard called FIRST, returns before any store call, proven by row-count-unchanged tests — wired in 11-05.)_ ✅ Plan 11-05 (2026-05-29)
- [x] **BACKEND-05**: The backend exposes a versioned read API that powers the website's four views (replacing the Sheet's view tabs as the query layer). _(Compute/data half in 14-01; the 5 public read handlers + CORS in 14-03; the SvelteKit client that fetches all 5 endpoints — `api.ts` over `PUBLIC_API_BASE` — in 14-04.)_ ✅ Plans 14-01 + 14-03 + 14-04 (2026-05-30)
- [x] **BACKEND-06**: The SQLite database is backed up nightly off-box (rsync or block-volume snapshot), with a documented restore procedure. _(Nightly `sqlite3 .backup` → gzip → Cloudflare R2 via `rclone` on a cron schedule; documented + drilled restore reconstitutes the DB on a clean box — delivered + verified LIVE in 11-07.)_ ✅ Plan 11-07 (2026-05-29)

### Enrichment jobs (ENRICH)

- [x] **ENRICH-10**: The daily PigParse price pull (server=1=Blue) runs as an in-process scheduled job, reusing the existing parser and `politeFetch` controls (User-Agent, ETag/If-Modified-Since, backoff). ✅ 2026-05-29 (P12 — scheduler registers `pigparse_daily`, due when now-last>=24h)
- [x] **ENRICH-11**: The weekly P1999 wiki scrape (per-item summaries, per-class spell lists, Velious gear tiers, quest items) runs as an in-process scheduled job with the same politeness controls. ✅ 2026-05-29 (P12 — scheduler registers `wiki_weekly`, due Sunday UTC)

### Watcher re-target (WATCH)

- [x] **WATCH-08**: The watcher uploads inventory/spellbook snapshots to the backend HTTP API instead of Google Sheets, preserving the 500 ms debounce + always-re-read behavior. ✅ 13-03 (sink rewire: read→CP1252-decode→POST raw UTF-8 to `backend.Ingest`; `parse.Parse` grep-absent from the upload path; fsnotify debounce + always-re-read untouched).
- [x] **WATCH-09**: All Google OAuth/PKCE/Sheets/Drive-Picker machinery is removed from the watcher (~2.5–3k LOC: `internal/auth`, `internal/sheet`, `internal/scaffold`, `internal/picker`, most of `internal/wizard`, reauth/propagation probes). ✅ 13-03 (6 packages + reauth.go DELETED; `go mod tidy` drops the oauth2/google-api tree; OAuth ldflags stripped from build_constants.go + release.yml — no Google secret baked; `go list -deps ./cmd/squirebot` Google-free). 13-04 confirms the materially-smaller-binary SC-2 byte measure.
- [x] **WATCH-10**: First-run onboarding is "paste your guild code" — the bearer token is stored in Windows Credential Manager (DPAPI); no browser OAuth flow. ✅ 13-03 (native `PromptGuildCode` → `backend.Validate(/whoami)` → `credstore.Store` (DPAPI) → `PickEQFolder` → green; zero browser, zero loopback).
- [x] **WATCH-11**: Existing guildies receive the re-targeted watcher via the existing GitHub-Releases auto-updater (one manual step: paste guild code). ✅ 13-03 (`MigrateFromV1` first-launch: deletes the stale `SquireBot:<google-email>` wincred entry + drops dead config fields, idempotent, preserves EQFolders+mtime; the `internal/update` GitHub-Releases transport is unchanged). 13-04's 999.22 SemVer compare de-risks the P16 coordinated flip.

### Web frontend (WEB)

- [x] **WEB-01**: A static web frontend renders the four consolidated views (`view`, `gear_check`, `spell_check`, `bank`) with the leading Char column, filterable and sortable (the work the Sheet did for free). _(One reusable `DataGrid` over a local `@tanstack/table-core` Svelte-5 adapter, instantiated 4× with the exact v1 column orders, a sticky Char column + sticky header, global + per-column/faceted filters, and multi-sort — never per-character tabs.)_ ✅ Plan 14-04 (2026-05-30)
- [x] **WEB-02**: `gear_check` and `spell_check` compute and display progression status — gear OK/MISSING/OTHER vs. Velious tiers; spell KNOWN/MISSING — matching the v1 semantics. _(Compute parity in 14-01; the colored `StatusCell` badges + `StatusLegend` rendering them from the read API in 14-04.)_ ✅ Plans 14-01 + 14-04 (2026-05-30)
- [x] **WEB-03**: Cross-character item search with "did you mean?" fuzzy matching (ports the Wagner-Fischer Levenshtein logic), returning results in <2s. _(Logic in 14-02; the `SearchBox`/`SearchResults` UI — holders surfaced, >5 auto-collapse, clickable inline "did you mean?" that re-runs — in 14-04.)_ ✅ Plans 14-02 + 14-04 (2026-05-30)
- [x] **WEB-04**: Every inventory row shows a per-item tooltip (wiki summary + price + quest info) and a direct wiki link. _(Escaped `composeItemNote` in 14-02; the hover/tap `ItemTooltip` popover (Esc/outside dismiss) injecting it via the sole `{@html}` in 14-04.)_ ✅ Plans 14-02 + 14-04 (2026-05-30)
- [x] **WEB-05**: The site carries an EQ aesthetic theme site-wide, building on `docs/design/eq-aesthetic-theme.md` (now with full CSS freedom vs. the Sheet's cell-styling limits). _(5-theme CSS-var registry in 14-02; the `[data-theme]` SiteShell + ThemePicker applying it via a single attribute write + localStorage (velious default, `applyTheme`) in 14-04.)_ ✅ Plans 14-02 + 14-04 (2026-05-30)

### Website login (AUTH)

- [x] **AUTH-08**: Website visitors sign in with Discord OAuth2, gated on membership in the guild's Discord server (no allowlist upkeep). _(15-02: hand-rolled `golang.org/x/oauth2` login/callback, server-side code exchange, fail-closed `IsGuildMember` via `/users/@me/guilds`, non-member refused with NO session; read API walled behind RequireSession. Code-complete + httptest-verified; live login smoke deferred to deploy per the build-only directive. 15-04: the frontend AuthGate wraps the whole site — LoginScreen for the unauthenticated, the NotMemberScreen refusal for an authed non-member, credentialed fetch + server-truth 401/403 re-routing.)_ ✅ Plan 15-02 + 15-04 (2026-05-30)
- [x] **AUTH-09**: Each signed-in user's Discord identity is captured and stored, pre-paying the v2 Wantlist/Discord-pinger prerequisite (per-user Discord identity). _(15-01 schema `web_user` + 15-02 `UpsertWebUser` captures discord_user_id/username/avatar on each login. 15-04: the SessionIndicator surfaces the captured Discord avatar+username in the shell, with a Sign-out control.)_ ✅ Plan 15-02 + 15-04 (2026-05-30)

### Admin web forms (ADMIN)

- [x] **ADMIN-04**: Eviction (30-day grace + archive) is available as an authenticated, officer-only web form (ports v1 eviction enforcement: `guild_admins` gate + owner-floor protection). _(15-05 EvictionForm: pick a guildie → preview the character cascade + the 30-day grace + the D-10 guild-code-revoke consequence → ConfirmDialog confirm `Evict <guildie>` → evict; officer-only via the /admin gate + server-truth — a 403 collapses to Officers-only, owner_floor_protected → the inline floor block; composes 15-03's officer-only evict/restore/preview. Local build+verify; live evict/restore smoke deferred to deploy.)_ ✅ Plan 15-05 (2026-05-31)
- [x] **ADMIN-05**: Manual bank-coin entry (platinum/gold/silver/copper) is an authenticated web form (the file format still has no coin data; manual entry remains the only honest path). _(15-05 BankCoinForm: login-only [D-12, NO officer guard], pick an is_bank_toon character → pre-filled, range-validated [plat≥0, gold/silver/copper 0–999] four inputs → Save coin; the recorded coin SURFACES in the bank view, replacing P14's null placeholder [D-11]. Composes 15-03's login-only coin POST. Local build+verify.)_ ✅ Plan 15-05 (2026-05-31)
- [x] **ADMIN-06**: Admin/officer management (the `guild_admins` allowlist + owner-floor lockout protection) is an authenticated web form. _(15-05 AdminMgmtForm: `Current officers (N):` list with the `(owner)` floor annotation + Remove suppressed for a peer; promote-by-pick [D-07] + Add officer; Remove via ConfirmDialog; the exact owner-floor/lock-busy/not-authorized error routing [a not_authorized 403 collapses to Officers-only]. Composes 15-03's officer-only add/remove/list. Local build+verify; live smoke deferred to deploy.)_ ✅ Plan 15-05 (2026-05-31)

### Cutover & decommission (CUTOVER)

- [ ] **CUTOVER-01**: The backend runs in shadow mode alongside the live Google Sheet for a 1–2 week soak — ingesting real uploads while never writing to the Sheet (cutover cannot corrupt the live product).
- [ ] **CUTOVER-02**: A one-time backfill imports human-supplied data only (owner/character metadata, bank coin, archive entries); inventory + dimension/enrichment data self-populates from the next upload / next enrichment run.
- [ ] **CUTOVER-03**: A single coordinated watcher self-update flips ingest from the Sheet to the backend across the guild.
- [ ] **CUTOVER-04**: After a successful cutover, the Google Sheet, Apps Script project, and Google OAuth client are decommissioned.

---

## Out of Scope (v2.0 — explicitly deferred at milestone open)

| Item | Why deferred |
|------|--------------|
| v2 Wantlist + EC/WTS Discord pinger (WANT-01..08) | Still needs Raid Alliance Discord bot invites (unnegotiated). v2.0 pre-pays the per-user Discord-identity prerequisite (AUTH-09) but does not build the pinger. |
| Unblocking the v1.0.2 Google OAuth client | The guild stays dark on the Sheet during the build; v2.0 replaces the Sheet, so renting a domain to placate Google would be throwaway work. |
| SignPath OSS / installer code-signing polish | Orthogonal to the backend swap; the watcher binary changes here are for the re-target, not signing. |
| Remaining v1.1 Sheet-side polish (theme picker tile UI 999.2, inline SIDEBAR_BODY 999.7, etc.) | The Sheet is being retired; Sheet-side polish is wasted effort. |
| Postgres / managed DB | SQLite fits the tiny data (<100 MB, ~50–150 writes/day); Postgres is needless ops overhead at this scale (SCOPE Open Decision 1). |
| "Sign in with Google" website login | Re-introduces the exact brand-verification gate v2.0 exists to escape. Discord OAuth2 (gate-free) is the choice; magic-link/GitHub are gate-free fallbacks. |
| Mobile app | The website is reachable from any browser; native mobile remains unnecessary scope (carried from v1). |
| Inventory history / per-item time-series | Not in the Core Value ("what's missing, where is it"); adds real complexity. Parked as backlog (finding 04 §1.2). |
| Per-owner visibility tiers | Universal visibility remains the v1 decision; the DB makes tiers cheap to add later if the guild asks, but not v2.0 scope (finding 04 §5.2). |

---

## Traceability

| REQ-ID | Phase (finalized) | Plan(s) | Status |
|--------|-------------------|---------|--------|
| BACKEND-01 | P11 Backend Foundation + Ingest API | 11-05 (single binary + in-process scheduler half); 11-06 (VPS + Caddy + systemd + TLS half) | ✅ Complete (2026-05-29) |
| BACKEND-02 | P11 Backend Foundation + Ingest API | 11-02 | ✅ Complete (2026-05-29) |
| BACKEND-03 | P11 Backend Foundation + Ingest API | 11-03 (tx + bind); 11-05 (HTTP surface) | ✅ Complete (2026-05-29) |
| BACKEND-04 | P11 Backend Foundation + Ingest API | 11-04 (mint/revoke + guard); 11-05 (HTTP transport) | ✅ Complete (2026-05-29) |
| BACKEND-06 | P11 Backend Foundation + Ingest API | 11-07 (sqlite3 .backup → R2 via rclone + restore drill) | ✅ Complete (2026-05-29) |
| ENRICH-10 | P12 Enrichment Job Migration | 12-01 (00003 migration + `pigparse_price` upsert store method + `job_run`/`etag_cache` cursors — foundation) + 12-02 (`ParseToRows` ported, byte-parity) + 12-03 (politeFetch) + 12-04 (`RunPigparse` — D-9 WTS filter, D-4 truncation-guard-as-LOG, 304-skip; composes Wave-1 over one tx, zero inline SQL) + 12-05 (scheduler registers `pigparse_daily`, due now-last>=24h, immediate-check-on-startup + advance-always cursor + per-job mutex; `run-job pigparse` D-7 entrypoint) | ✅ Complete (2026-05-29) |
| ENRICH-11 | P12 Enrichment Job Migration | 12-01 (00003 migration + `wiki_spells`/`wiki_gear_tier`/`item_master`/`quest_items` store methods + cursors — foundation) + 12-02 (`ParseItempage`/`ParseClassPage`/`ParseGearTierPage` ported, byte-parity) + 12-03 (politeFetch) + 12-04 (`RunWiki` — single uninterrupted run, 1s sleep, SHA-1 short-circuit, gear full-replace, log-but-continue; zero inline SQL) + 12-05 (scheduler registers `wiki_weekly`, due Sunday UTC; `run-job wiki` D-7 entrypoint) | ✅ Complete (2026-05-29) |
| WATCH-08 | P13 Watcher Re-Target + Onboarding | 13-01 (`/whoami` validate + 426 version-gate) + 13-02 (`internal/backend` POST client + UA version) + 13-03 (sink rewire: watch→read→POST raw UTF-8, debounce preserved) | ✅ Complete (2026-05-30) |
| WATCH-09 | P13 Watcher Re-Target + Onboarding | 13-03 (DELETE internal/auth/sheet/scaffold/picker/wizard/heartbeat + reauth.go, strip OAuth ldflags, `go mod tidy` drops the Google tree) + 13-04 (gofmt/freeConsole nits 999.20/21 + 999.22 SemVer compare + binary materially-smaller/no-secret SC-2 confirmation) | ✅ Complete (2026-05-30; 13-04 confirms the SC-2 binary-size byte measure) |
| WATCH-10 | P13 Watcher Re-Target + Onboarding | 13-02 (`internal/credstore` DPAPI guild-code store + `internal/onboarding` native Win32 dialog, no browser) + 13-03 (onboarding flow: prompt→`/whoami` validate→credstore.Store→green) | ✅ Complete (2026-05-30) |
| WATCH-11 | P13 Watcher Re-Target + Onboarding | 13-03 (first-launch `MigrateFromV1`: delete stale Google wincred + drop dead config fields, idempotent, preserve EQFolders+mtime; auto-update transport unchanged) + 13-04 (999.22 SemVer compare de-risks the P16 coordinated self-update flip) | ✅ Complete (2026-05-30; 13-04's 999.22 de-risks the P16 flip) |
| BACKEND-05 | P14 Web Frontend | 14-01 (compute/data half: `store/readviews.go` + `compute/` Go reimpl of the 4 view builders, parity test-proven) → 14-03 (HTTP read handlers + CORS) → 14-04 (`api.ts` client fetches all 5 endpoints over `PUBLIC_API_BASE`) | ✅ Complete (2026-05-30) |
| WEB-01 | P14 Web Frontend | 14-04 (one reusable `DataGrid` over the local `@tanstack/table-core` adapter, 4 instances, exact v1 column orders, sticky Char+header, faceted filters, multi-sort) | ✅ Complete (2026-05-30) |
| WEB-02 | P14 Web Frontend | 14-01 (gear OK/OTHER/MISSING + spell KNOWN/MISSING **compute** parity-proven by Go table-tests translated from the v1 vitest fixtures) → 14-04 (grid **display**: `StatusCell` badges + `StatusLegend` from the read API) | ✅ Complete (2026-05-30) |
| WEB-03 | P14 Web Frontend | 14-02 (**logic**: searchIndex ported, 999.28+999.30 fixed, `searchRows` in-memory engine, 17 tests) → 14-04 (**UI**: SearchBox/SearchResults — holders surfaced, >5 collapse, clickable inline did-you-mean re-runs) | ✅ Complete (2026-05-30) |
| WEB-04 | P14 Web Frontend | 14-02 (**logic**: composeNotes → escaped rich HTML, malicious-name XSS test-proven, 15 tests) → 14-04 (**UI**: hover/tap ItemTooltip popover, Esc/outside dismiss, the sole `{@html}` on the escaped output, wiki `rel=noopener`) | ✅ Complete (2026-05-30) |
| WEB-05 | P14 Web Frontend | 14-02 (**theme**: 5-theme CSS-custom-property registry, velious default, `[data-theme]` blocks in app.css, 11 tests) → 14-04 (**UI**: SiteShell+ThemePicker applies it via `applyTheme` — single `[data-theme]` write + localStorage, velious default) | ✅ Complete (2026-05-30) |
| AUTH-08 | P15 Admin Web Forms + Login | 15-02 (**backend**: oauth2 login/callback, fail-closed guild-membership gate, RequireSession on the read API) → 15-04 (**UI**: the whole-site AuthGate — LoginScreen, the NotMemberScreen refusal, credentialed fetch, server-truth 401/403 re-routing) | ✅ Complete (2026-05-30; live smoke deferred to deploy) |
| AUTH-09 | P15 Admin Web Forms + Login | 15-01 (web_user schema) → 15-02 (UpsertWebUser identity capture on login) → 15-04 (**UI**: the SessionIndicator surfaces the captured Discord avatar+username in the shell) | ✅ Complete (2026-05-30) |
| ADMIN-04 | P15 Admin Web Forms + Login | 15-01 (**store**: EvictOwnerTx cascade+revoke+grace, RestoreOwnerTx, ArchiveExpiredEvictions) → 15-03 (**backend**: officer-only evict/restore/preview + re-mint-on-restore + 409 grace_expired + DAILY archive job, authorize-under-tx, owner-floor protected, audited) → 15-05 (**UI**: EvictionForm — preview+D-10 consequence+ConfirmDialog; 403→Officers-only collapse) | ✅ Complete (2026-05-31; local build+verify, live smoke deferred) |
| ADMIN-05 | P15 Admin Web Forms + Login | 15-01 (**store**: SetCoinTx bank-toon-gated, ListBankToons/GetCoin, coin columns) → 15-03 (**backend**: login-only [D-12] coin POST + range validation + bank-toon gate + audit, non-officer write proven) → 15-05 (**UI**: BankCoinForm [login-only, range-validated, pre-filling] + bank-view coin surfacing) | ✅ Complete (2026-05-31; local build+verify) |
| ADMIN-06 | P15 Admin Web Forms + Login | 15-01 (**store**: Add/RemoveOfficerTx authorize-under-tx, owner-floor protection, ListOfficers/ListPromotableUsers) → 15-03 (**backend**: officer-only add/remove/list, idempotent, owner-floor protected, audited) → 15-05 (**UI**: AdminMgmtForm — list+promote-by-pick+remove+owner-floor) | ✅ Complete (2026-05-31; local build+verify, live smoke deferred) |
| CUTOVER-01 | P16 Cutover + Decommission | (filled by `/gsd-plan-phase 16`) | Pending |
| CUTOVER-02 | P16 Cutover + Decommission | (filled by `/gsd-plan-phase 16`) | Pending |
| CUTOVER-03 | P16 Cutover + Decommission | (filled by `/gsd-plan-phase 16`) | Pending |
| CUTOVER-04 | P16 Cutover + Decommission | (filled by `/gsd-plan-phase 16`) | Pending |

> Phase column **finalized by roadmap creation (2026-05-28)** — the provisional mapping was accepted unchanged (no concrete coverage problem found). Success criteria (2–5 observable behaviors per phase) live in `.planning/ROADMAP.md` § Phase Details. Plan column filled by `/gsd-plan-phase` per phase.

**Coverage check (2026-05-28, roadmap-finalized):** 26/26 requirements mapped to exactly one phase. No orphans, no duplicates. P11=5 (BACKEND-01/02/03/04/06), P12=2 (ENRICH-10/11), P13=4 (WATCH-08/09/10/11), P14=6 (BACKEND-05 + WEB-01/02/03/04/05), P15=5 (AUTH-08/09 + ADMIN-04/05/06), P16=4 (CUTOVER-01/02/03/04).

---

*Defined: 2026-05-28 at v2.0 milestone start. 26 requirements across 7 categories (Backend, Enrichment, Watcher, Web, Auth, Admin, Cutover). Phase mapping FINALIZED by roadmap creation 2026-05-28 (Phases 11–16); success criteria in ROADMAP.md § Phase Details.*
