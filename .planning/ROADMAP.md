# Roadmap: SquireBot

**Core Value:** Every guildie can answer "what does my character still need, and where in the guild is it?" — now delivered via a self-hosted website instead of the Google Sheet.

## Milestones

- ✅ **v1.0** — Watcher + Workbook + Onboarding (initial release) — shipped 2026-05-11 as tag `v1.0.0`
- ✅ **v1.0.1** — Installer + Permissions Hardening — shipped 2026-05-12 (binary tag `v1.0.1`)
- ✅ **v1.0.2** — Robustness Polish — binary shipped 2026-05-13 (tag `v1.0.2`); milestone close superseded by v2.0
- 🚧 **v2.0** — "Off Google" — Website Frontend — Phases 11–16 (in progress, started 2026-05-28)

## Phases

<details>
<summary>✅ v1.0 — Watcher + Workbook + Onboarding (Phases 1–5) — SHIPPED 2026-05-11</summary>

Full details in [`milestones/v1.0-ROADMAP.md`](milestones/v1.0-ROADMAP.md).

- [x] Phase 1: End-to-End Thin Slice (8 plans) — shipped v0.1.0, 2026-05-02
- [x] Phase 2: Watcher Robustness + Schema Lock (10 plans) — shipped v0.2.0 + v0.2.1 hotfix, 2026-05-09
- [x] Phase 3: Apps Script Enrichment Foundation (4 plans) — shipped v0.3.0, 2026-05-10
- [x] Phase 4: Differentiator Features (4 plans) — shipped v0.4.0, 2026-05-11
- [x] Phase 5: Search + Onboarding + Privacy Polish (5 plans) — shipped v1.0.0, 2026-05-11

**Total:** 31 plans · 5 phases · 11 days kickoff to ship · 203 commits.

</details>

<details>
<summary>✅ v1.0.1 — Installer + Permissions Hardening (Phases 6–8) — SHIPPED 2026-05-12</summary>

Full details in [`milestones/v1.0.1-ROADMAP.md`](milestones/v1.0.1-ROADMAP.md).

- [x] Phase 6: Installer Overwrite-Running Shim (5 plans) — shipped + UAT-verified as tag `v1.0.1`, 2026-05-11
- [x] Phase 7: Admin Allowlist + Eviction Enforcement (3 plans) — shipped via dev-workbook 5-hook smoke, 2026-05-12
- [x] Phase 8: Test Infra + Persistence + Docs Backfill (4 plans) — shipped (5/5 must-haves; 336/336 vitest), 2026-05-12

**Total:** 12 plans · 3 phases · 2 days kickoff to ship · 63 commits since v1.0.0.

</details>

<details>
<summary>✅ v1.0.2 — Robustness Polish (Phases 9–10) — binary shipped 2026-05-13; milestone close superseded by v2.0</summary>

Binary `v1.0.2` shipped 2026-05-13; its milestone close was superseded by the v2.0 "Off Google" pivot (the Sheet it targeted is being replaced). Phase directories 09 and 10 exist on disk; the next milestone continues at Phase 11 (never reuses 9 or 10).

- [x] Phase 9: Watcher Robustness Polish (5 plans) — AUTH-07, OPS-06, OPS-07, CONFIG-01; shipped as watcher v1.0.2 binary, 2026-05-13. HUMAN-UAT was blocked on 999.19 (Google brand verification), then superseded.
- [x] Phase 10: Apps Script Test Quality (3 plans) — TEST-03, TEST-04; shipped via `clasp push` + green CI, 2026-05-13.

</details>

### 🚧 v2.0 — "Off Google" — Website Frontend (In Progress)

**Milestone Goal:** Replace the shared Google Sheet (both the UI *and* the data store) with a self-hosted Go + SQLite backend and a static web frontend — permanently eliminating the Google OAuth dependency that currently blocks the guild.

**Why now:** Google's brand-verification gate rejected the OAuth client (2026-05-15: *"home page not registered to you"* — `github.io` can't satisfy it), walling off all ~12 guildie watchers. v2.0 removes Google from the system entirely.

**Stack (LOCKED + costed, ≈ $67/yr total):** Hetzner Cloud shared-vCPU VPS (US, amd64) backend (~$55/yr) · SQLite + `goose` forward-only migrations ($0) · SvelteKit static on Cloudflare/GitHub Pages ($0) · Discord OAuth2 website login ($0) · per-guildie opaque bearer token for watcher↔backend ($0) · ~$12/yr **website-only** domain (no Google verification needed).

> **Stack note — reverted to the research-recommended Hetzner VPS; SQLite retained (not Postgres):** the four findings docs and SCOPE.md recommended a **Hetzner VPS + PostgreSQL**. v2.0 initially **overrode** the host to Oracle Cloud Always Free for $0, then **reverted to the research-recommended Hetzner Cloud VPS (US, amd64)** on 2026-05-29 (see `PROJECT.md` Key Decisions + Phase 11 CONTEXT D-12) — while **keeping SQLite + `goose`** rather than the research's Postgres. Where a findings doc says "Postgres", read "SQLite" (the DDL ports with only `CITEXT`→`TEXT COLLATE NOCASE` + identity-column syntax changes, and `pg_trgm` search becomes SQLite FTS5 / `LIKE` — already noted as an acceptable fallback in finding 04 §6). The findings docs' Hetzner host recommendation is now the chosen host; cost is ~$55/yr VPS + ~$12/yr domain ≈ $67/yr (the "$0 backend" premise is retired).

**Sequencing principle — FRONT-LOAD THE INGEST PATH:** the ~12 guildies are dark on the Sheet during the build (Google has walled off their watchers; v2.0 replaces the Sheet rather than placating Google). So P11 (ingest endpoint) and P13 (watcher re-target) restore the data pipeline first; the polished frontend (P14) and admin forms (P15) follow once data is flowing again.

**Schema-evolution change:** the old `_meta.schema_version` ↔ `WatcherMaxSchemaVersion` handshake **retires** in favour of forward-only `goose` DB migrations + an explicit **API version** (`/api/v1/...`). The watcher's Sheets schema gate is removed in P13.

- [ ] **Phase 11: Backend Foundation + Ingest API** - Hetzner Cloud VPS (US) + Caddy auto-HTTPS live; SQLite schema with `goose`; bearer-token auth; the upload-receiving endpoint. *Front-loaded so guild data can flow again ASAP.*
- [ ] **Phase 12: Enrichment Job Migration** - daily PigParse pull + weekly P1999 wiki scrape run as in-process scheduled jobs, parsers and `politeFetch` controls carried over.
- [ ] **Phase 13: Watcher Re-Target + Onboarding** - watcher uploads to the backend HTTP API; all Google OAuth/Sheets/Picker machinery deleted; "paste your guild code" onboarding shipped via the existing auto-updater.
- [ ] **Phase 14: Web Frontend** - SvelteKit static app + read API; the 4 views as a reusable filterable/sortable data grid; client-side search + "did you mean?"; rich HTML tooltips; EQ theming.
- [ ] **Phase 15: Admin Web Forms + Login** - Discord OAuth2 login gated on guild Discord membership; eviction, bank-coin, and admin-management as authenticated web forms.
- [ ] **Phase 16: Cutover + Decommission** - shadow-mode soak alongside the live Sheet (1–2 wk), one-time human-data backfill, one coordinated watcher self-update flip, then decommission the Sheet + Apps Script + Google OAuth client.

## Phase Details

### Phase 11: Backend Foundation + Ingest API
**Goal**: Stand up the self-hosted backend so guildie watchers have a live, authenticated place to upload to again — Hetzner Cloud VPS (US) behind Caddy auto-HTTPS, a SQLite schema under `goose` forward-only migrations, per-guildie bearer-token auth, and the upload-receiving endpoint that atomically replaces a character's rows.
**Depends on**: Nothing (first phase of v2.0; foundation for everything after)
**Requirements**: BACKEND-01, BACKEND-02, BACKEND-03, BACKEND-04, BACKEND-06
**Success Criteria** (what must be TRUE):
  1. The backend serves over HTTPS at the website domain from the Hetzner Cloud VPS (US) — a single Go binary behind Caddy, restart-on-reboot via systemd, reachable with a valid TLS certificate (BACKEND-01)
  2. `goose up` applies the SQLite schema cleanly on a fresh database, creating separate `owner` and `character` tables plus inventory/spellbook/dimension tables; re-running it is a no-op (forward-only, idempotent) (BACKEND-02)
  3. A `POST` of a full-snapshot inventory or spellbook payload, carrying a valid `Authorization: Bearer <guild-code>`, atomically replaces that character's rows (delete-all-then-insert in one transaction) and the rows are then visible via a query — and a shrinking snapshot drops the removed rows (BACKEND-03)
  4. A request with a missing, malformed, or unknown bearer token is rejected (401) and writes nothing; the maintainer can mint a per-guildie token whose plaintext is shown once and stored only hashed server-side (BACKEND-04)
  5. A nightly off-box backup of the SQLite file runs on a schedule, and the documented restore procedure reconstitutes the database from a snapshot on a clean box (BACKEND-06)
**Plans**: 7 plans (1 spike + 4 autonomous build/test + 2 on-box ops)
- [x] 11-01-PLAN.md — PocketBase-as-framework spike (Wave 1, gating): ran the four D-01 PASS/FAIL probes (all PASS); **VERDICT = HAND-ROLLED Go fallback** (reject PocketBase — design bypasses PB's auth-record + collection models). Recorded in 11-01-SUMMARY + 11-CONTEXT. ✅ 2026-05-29
- [x] 11-02-PLAN.md — goose schema + 00001_init.sql + modernc DB-open (DSN pragmas) + shared temp-DB test helper (BACKEND-02). ✅ 2026-05-29
- [x] 11-03-PLAN.md — parser port to UTF-8 content (A1) + atomic full-snapshot replace tx + first-sighting bind/cross-owner reject (BACKEND-03). ✅ 2026-05-29
- [ ] 11-04-PLAN.md — bearer guard (SHA-256 + constant-time compare) + mint/revoke CLI, hash-only storage (BACKEND-04)
- [ ] 11-05-PLAN.md — POST /api/v1/ingest handler + cmd/squirebot-server entrypoint + scheduler skeleton (verdict-dependent wiring) (BACKEND-01/03/04)
- [ ] 11-06-PLAN.md — Hetzner VPS provisioning + Cloud Firewall + ufw + Caddy + systemd + cross-compile deploy (on-box; BACKEND-01)
- [ ] 11-07-PLAN.md — nightly sqlite3 .backup -> Cloudflare R2 via rclone + restore drill + ship-gate smoke (on-box; BACKEND-06)
**UI hint**: no (backend/infrastructure only; no frontend in this phase)
**Decision — PocketBase spike (optional, 1 day, at phase start):** finding 01 §Recommendation flags PocketBase (open-source single Go binary = SQLite + auth + REST + admin UI) as almost exactly this design, pre-built. A 1-day spike at the start of P11 could compress P11 **and** P15 by ~5–8 days. Evaluate self-hosted PocketBase on the same Hetzner VPS before committing to a hand-rolled Go server; if its auth/extension model fits the bearer-token + enrichment-hook needs, prefer it; if it chafes, the hand-rolled Go server is the fallback. Either way the host (Hetzner Cloud VPS) + DB (SQLite) decisions stand. Capture the verdict in the phase CONTEXT.
**Ship gate**: server accepts a real test upload over TLS and the row is queryable back out.

### Phase 12: Enrichment Job Migration
**Goal**: Move the two enrichment feeds off Apps Script triggers and into the backend as in-process scheduled jobs so the new DB self-populates its dimension data — the daily PigParse price pull and the weekly P1999 wiki scrape, with the existing parsers and politeness controls carried over verbatim.
**Depends on**: Phase 11 (writes into the SQLite dimension tables and the in-process scheduler the backend established)
**Requirements**: ENRICH-10, ENRICH-11
**Success Criteria** (what must be TRUE):
  1. The daily PigParse pull (`GET /api/item/getall/1`, server=1=Blue) runs on its in-process schedule and upserts current prices into the price table; a truncated/partial response degrades gracefully (updates what it got) rather than clobbering good data (ENRICH-10)
  2. The weekly wiki scrape runs as a single uninterrupted in-process job — per-item summaries, per-class spell lists, Velious gear tiers, and quest items all land in their tables — with no Apps Script 6-minute cap and therefore no resumable-cursor workaround (ENRICH-11)
  3. The carried-over `politeFetch` controls are observably in force on both jobs: identifying User-Agent, ETag/If-Modified-Since (304 short-circuit), exponential backoff honoring `Retry-After`, and the 1-second inter-request wiki sleep (ENRICH-10, ENRICH-11)
  4. After one daily + one weekly cycle, the backend's dimension data matches the live Sheet's `_item_master` / `_pigparse` / `_wiki_spells` / `_wiki_gear_tier` / `_quest_items` to spot-check parity (ENRICH-10, ENRICH-11)
**Plans**: TBD
**UI hint**: no (backend scheduled jobs only)
**Note**: parsers are pure, host-agnostic functions (`parseToRows`, `parseItempage`, wiki-spell / gear-tier parsers) and port near-verbatim to Go; only the I/O wrappers (`UrlFetchApp`/`PropertiesService`/`CacheService`/`LockService`) are replaced. The `monitorCellCount` and `weeklySchemaHealthcheck` watchdogs are Sheets-specific and are dropped.

### Phase 13: Watcher Re-Target + Onboarding
**Goal**: Re-point the Go watcher from Google Sheets to the backend HTTP API and ship it to the existing guild — swap the Sheets client for a small `internal/backend` client, delete the entire Google OAuth/PKCE/Sheets/Drive-Picker stack (~2.5–3k LOC, the highest-complexity code in the project), and replace the loopback-OAuth wizard with a one-field "paste your guild code" setup, delivered via the existing auto-updater. This restores the live data pipeline end-to-end.
**Depends on**: Phase 11 (the ingest API + bearer-token contract the watcher now targets)
**Requirements**: WATCH-08, WATCH-09, WATCH-10, WATCH-11
**Success Criteria** (what must be TRUE):
  1. The watcher uploads inventory/spellbook snapshots to the backend's ingest API instead of Google Sheets, preserving the fsnotify 500 ms debounce + always-re-read behavior; an edited file lands in the backend within seconds (WATCH-08)
  2. The re-targeted binary contains no Google OAuth/PKCE/Sheets/Drive-Picker code — `internal/auth`, `internal/sheet`, `internal/scaffold`, `internal/picker`, the reauth/propagation-probe machinery, and most of `internal/wizard` are gone; the build bakes in no Google secret; the binary is materially smaller (WATCH-09)
  3. First-run onboarding is "paste your guild code" — the watcher prompts for the code, validates it against the backend, stores the bearer token in Windows Credential Manager (DPAPI), and goes green; no browser OAuth flow exists anywhere (WATCH-10)
  4. An existing v1.x watcher auto-updates to the re-targeted binary via the GitHub-Releases pipeline with no binary action by the guildie; on first launch it finds no backend credential, prompts once for the guild code, and the stale Google wincred entry + dead `config.json` fields are cleaned up (WATCH-11)
  5. The watcher sends its version and the backend rejects-with-clear-message a watcher too old for the API version — the "old watcher refuses to corrupt data" guarantee survives the move from `WatcherMaxSchemaVersion` to API versioning (WATCH-08, WATCH-09)
**Plans**: TBD
**UI hint**: no (Go watcher + tray-driven native/loopback setup prompt; not the web frontend)
**Ship gate**: re-targeted watcher binary published on GitHub Releases; a clean install + a guild-code paste results in a successful upload to the backend.

### Phase 14: Web Frontend
**Goal**: Give the guild back its read UI as a real website — a static SvelteKit app over a versioned read API that renders the four consolidated views (`view`, `gear_check`, `spell_check`, `bank`) as a reusable filterable/sortable data grid, with cross-character fuzzy search, rich HTML item tooltips, and site-wide EQ theming. This is the visible replacement for the Sheet's UI.
**Depends on**: Phase 11 (the backend exposes the read API here — BACKEND-05) and Phase 12 (the views need enrichment/dimension data to be meaningful)
**Requirements**: BACKEND-05, WEB-01, WEB-02, WEB-03, WEB-04, WEB-05
**Success Criteria** (what must be TRUE):
  1. The backend exposes a versioned read API (`/api/v1/...`) that returns the data powering all four views, replacing the Sheet's view tabs as the query layer (BACKEND-05)
  2. A guildie opens the static site and sees all four views — `view`, `gear_check`, `spell_check`, `bank` — each with the leading `Char` column, and can filter and sort every column (the work the Sheet did for free) (WEB-01)
  3. `gear_check` shows gear OK/MISSING/OTHER vs. Velious tiers and `spell_check` shows spell KNOWN/MISSING, matching v1 semantics for the same character (WEB-02)
  4. Cross-character item search returns matches in under 2 seconds and offers a "did you mean?" fuzzy suggestion (ported Wagner-Fischer Levenshtein) when a query has no exact hit (WEB-03)
  5. Every inventory row exposes a per-item tooltip (wiki summary + price + quest info) as rich HTML and a working direct wiki link; the site carries the EQ aesthetic theme site-wide per `docs/design/eq-aesthetic-theme.md` (WEB-04, WEB-05)
**Plans**: TBD
**UI hint**: yes
**Note**: pure-logic modules port across with little change — `searchIndex.ts` (`levenshtein`/`didYouMean`), `composeNotes.ts` (now rich HTML, no plain-text cell-note cap), and the `THEMES` registry → CSS custom properties. The Sheet's `onChange`/`buildView` rebuild + search-cache machinery is not rebuilt (views are computed on read). Theme picker becomes a per-user `localStorage` preference (no server write). UI safety gate applies.

### Phase 15: Admin Web Forms + Login
**Goal**: Add authenticated human access and the officer-only write actions to the website — Discord OAuth2 login gated on guild Discord-server membership (which also captures per-user Discord identity, pre-paying a v2 prerequisite), plus eviction, bank-coin entry, and admin/officer management as authenticated web forms that port the v1 enforcement rules.
**Depends on**: Phase 14 (the frontend/read-API surface the login gates and the forms write into) and Phase 11 (the backend auth layer + owner/character + admin schema)
**Requirements**: AUTH-08, AUTH-09, ADMIN-04, ADMIN-05, ADMIN-06
**Success Criteria** (what must be TRUE):
  1. A website visitor signs in with Discord OAuth2 and is admitted only if they are a member of the guild's Discord server — a non-member is refused, with no hand-maintained allowlist (AUTH-08)
  2. Each signed-in user's Discord identity is captured and stored, satisfying the v2 Wantlist/Discord-pinger per-user-identity prerequisite (AUTH-09)
  3. Eviction is an authenticated, officer-only web form that ports v1 enforcement — the `guild_admins` gate plus owner-floor lockout protection — and applies the 30-day grace + archive; a non-officer cannot reach or fire it (ADMIN-04)
  4. Manual bank-coin entry (platinum/gold/silver/copper) is an authenticated web form that persists the four values (the file format still carries no coin data) (ADMIN-05)
  5. Admin/officer management (the `guild_admins` allowlist + owner-floor protection) is an authenticated web form; the workbook-owner-floor equivalent cannot be removed by a peer admin (ADMIN-06)
**Plans**: TBD
**UI hint**: yes
**Note**: Discord OAuth2 login has no brand-verification or app-review gate (finding 03 §4 — confirmed); "Sign in with Google" is rejected outright because it would re-introduce the exact gate v2.0 exists to escape. In the DB world, eviction/admin enforcement gets *cleaner and more enforceable* — access revocation becomes one app-controlled action, not a separate Google Drive un-share. UI safety gate applies.

### Phase 16: Cutover + Decommission
**Goal**: Move the guild fully off Google with zero risk to the live product — run the backend in shadow mode alongside the still-live Sheet for a 1–2 week soak, backfill only the irreplaceable human-supplied data, flip all watchers to the backend in one coordinated self-update, then decommission the Sheet, the Apps Script project, and the Google OAuth client.
**Depends on**: Phase 13 (re-targetable watcher), Phase 14 (the website the guild lands on), Phase 15 (login + admin forms must exist before the Sheet retires), and Phase 12 (enrichment must be self-populating during the soak)
**Requirements**: CUTOVER-01, CUTOVER-02, CUTOVER-03, CUTOVER-04
**Success Criteria** (what must be TRUE):
  1. For a 1–2 week soak the backend ingests real uploads and runs its enrichment jobs while never writing to the Google Sheet, and the maintainer can compare the website's four views against the live Sheet's tabs for parity (CUTOVER-01)
  2. A one-time backfill imports only human-supplied data — owner/character metadata (class/level/race/is_bank_toon/is_hidden/is_removed), bank coin, and archive entries — read-only from the Sheet and idempotently re-runnable; inventory + dimension data self-populates from the next upload / next enrichment run (CUTOVER-02)
  3. A single coordinated watcher self-update flips ingest from the Sheet to the backend across the guild, and within the rollout window all ~12 watchers report in on the new endpoint (CUTOVER-03)
  4. After a confirmed-successful cutover, the Google Sheet, the Apps Script project, and the Google OAuth client are decommissioned — no Google dependency remains anywhere in the system (CUTOVER-04)
**Plans**: TBD
**UI hint**: no (operational cutover; no new UI built)
**Note**: cutover is the hybrid shadow-mode path (finding 04 §4.2) — the new system never writes to the Sheet, so a backend bug cannot corrupt the live product; both inventory and enrichment data are self-healing, so a botched flip is recoverable. The old Sheet can stay read-only through the 24-h auto-update window before retirement. Includes ~1–2 weeks of calendar soak beyond active development.
**Ship gate**: all ~12 watchers confirmed on the backend; Sheet + Apps Script + Google OAuth client decommissioned — the milestone's "Off Google" goal is met.

## Progress

**Execution Order:** Phases execute in numeric order: 11 → 12 → 13 → 14 → 15 → 16.

| Milestone | Phases | Plans Complete | Status | Completed |
|-----------|--------|----------------|--------|-----------|
| v1.0 | 5 | 31/31 | ✅ Shipped | 2026-05-11 |
| v1.0.1 | 3 | 12/12 | ✅ Shipped | 2026-05-12 |
| v1.0.2 | 2 | 8/8 | ✅ Binary shipped (milestone close superseded by v2.0) | 2026-05-13 |
| v2.0 | 6 | 1/TBD | 🚧 In progress | — |

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 11. Backend Foundation + Ingest API | v2.0 | 3/7 | 🚧 In Progress | - |
| 12. Enrichment Job Migration | v2.0 | 0/TBD | Not started | - |
| 13. Watcher Re-Target + Onboarding | v2.0 | 0/TBD | Not started | - |
| 14. Web Frontend | v2.0 | 0/TBD | Not started | - |
| 15. Admin Web Forms + Login | v2.0 | 0/TBD | Not started | - |
| 16. Cutover + Decommission | v2.0 | 0/TBD | Not started | - |

## Backlog

Carried forward from v1.0 / v1.0.1 / v1.0.2 (candidates for a future Sheet-orthogonal patch or v2.x). Note: several Sheet-side items below are likely **mooted by v2.0** (the Sheet + Apps Script are being decommissioned in Phase 16) — they are retained here for the record and to assess at v2.0 close.

- **999.1** Bank-coin permission lock (Sheet sidebar) — likely mooted by ADMIN-05 (web form replaces the sidebar).
- **999.2** Polished theme picker tile UI (Sheet) — mooted by WEB-05 (theme becomes a per-user client preference on the website).
- **999.5** Self-service eviction (departing guildie quits cleanly without officer action) — v2.x candidate; threat-model deferred.
- **999.7** Extract `SIDEBAR_BODY` constants to external `.html` (Sheet) — mooted by the frontend rebuild.
- **999.9** SignPath Foundation OSS approval — submitted; awaiting review (would retire INST-05 partial → full). Lands as a hotfix when approved; orthogonal to the backend swap.
- **999.11** Decide v2.x verification doctrine — adopt `/gsd-verify-work` per phase, or formalize a live-smoke pattern.
- **999.12 / WANT-01..08** v2: Wantlist + Discord pinger — prerequisites WANT-06/07 still open (Raid Alliance Discord bot invites). v2.0 pre-pays the per-user Discord-identity prerequisite via AUTH-09.
- **999.19** Google OAuth brand verification re-approval — **SUPERSEDED by v2.0** (Google removed from the system entirely; see STATE.md). Retained for incident-trail linkage only.
- **999.20** `console_windows.go` not `gofmt -l` clean — one-line fix; watcher carries into P13.
- **999.21** `freeConsole()` doc-vs-impl contract mismatch (log noise) — watcher carries into P13.
- **999.22** SemVer-aware auto-update comparison (`internal/update/check.go`) — relevant to P13/P16 (the coordinated self-update flip relies on the updater; pre-release sort safety).
- **999.23** Graceful tray messaging for Google policy/verification block — largely mooted by P13 (the Google OAuth path is deleted); the tray-classifier UX pattern may inform the bearer-token-rejected path.
- **999.24** `COL_RACE`/`COL_COUNT` collision (Sheet `showCharInfoSidebar.ts`) — mooted by the frontend rebuild.
- **999.25** Orphaned `squirebot:search:recent` CacheService key (Sheet) — mooted by decommission.
- **999.26** `evictionSidebar.inline.test.ts` bypasses admin gate at inline-JS layer (Sheet) — mooted by decommission.
- **999.27** `showSearchSidebar.test.ts` narrow negative assertion (Sheet) — mooted by decommission.
- **999.28** `searchIndex.ts` `didYouMean('')` contract bug — **port-relevant**: the search logic ports to the frontend in P14 (WEB-03); fix the empty-query contract during the port.
- **999.29** `test-helpers.ts` CacheService mock TTL nit (Sheet) — mooted by decommission.
- **999.30** `searchIndex.test.ts` Test 4 `didYouMean` Levenshtein contract mismatch — **port-relevant**: resolve when porting `didYouMean` to the frontend in P14 (WEB-03).

---

*Roadmap created: 2026-04-30. v1.0 shipped: 2026-05-11. v1.0.1 shipped: 2026-05-12. v1.0.2 binary shipped: 2026-05-13. Last reorganized: 2026-05-28 — v2.0 "Off Google" Website Frontend (Phases 11–16) added; coverage 26/26 v2.0 requirements.*
