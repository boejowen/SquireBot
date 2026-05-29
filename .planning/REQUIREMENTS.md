# Requirements: SquireBot v2.0 — "Off Google" (Website Frontend)

> v1.0 shipped 2026-05-11 (69 effective requirements; `milestones/v1.0-REQUIREMENTS.md`). v1.0.1 shipped 2026-05-12 (8 requirements; `milestones/v1.0.1-REQUIREMENTS.md`). v1.0.2 shipped as binary `v1.0.2` 2026-05-13 (6 requirements; milestone close superseded by this pivot). This file is scoped to v2.0 only.

**Milestone goal:** Replace the shared Google Sheet (both UI and data store) with a self-hosted Go + SQLite backend and a static web frontend — permanently eliminating the Google OAuth dependency that currently blocks the guild.

**Why now:** Google's brand-verification gate rejected the OAuth client 2026-05-15 ("home page not registered to you" — `github.io` can't satisfy it), walling off all ~12 guildie watchers. v2.0 removes Google from the system entirely rather than renting a domain to placate it. Full research: `.planning/explorations/website-milestone/SCOPE.md` + 4 findings docs.

**Stack (LOCKED + costed, ≈ $67/yr total):** Hetzner Cloud shared-vCPU VPS (US, amd64) backend (~$55/yr) · SQLite ($0) · SvelteKit static on Cloudflare/GitHub Pages ($0) · Discord OAuth2 website login ($0) · per-guildie opaque bearer token for watcher↔backend ($0) · ~$12/yr website-only domain.

**Architecture impact:** The `_meta.schema_version` ↔ `WatcherMaxSchemaVersion` handshake retires in favour of forward-only DB migrations (`goose`) + an API version. Google's 200-tab limit disappears; the 4 view tabs become SQL/API endpoints; the watcher sheds ~2.5–3k LOC of OAuth/Sheets/Picker code.

---

## v2.0 Requirements

### Backend — self-hosted server (BACKEND)

- [ ] **BACKEND-01**: The backend runs on a self-hosted always-on VPS (currently a Hetzner Cloud VPS, US) with Caddy auto-HTTPS, reachable at the website domain over TLS. Single Go binary; in-process scheduler for cron jobs.
- [ ] **BACKEND-02**: A SQLite schema with `goose` forward-only migrations models owners, characters, inventory items, spellbook entries, and dimension/enrichment data. `owner` and `character` are separate tables (owner-email change is a one-row update; no first-write-wins conflict logic).
- [ ] **BACKEND-03**: An ingest endpoint accepts a watcher's full-snapshot inventory or spellbook upload and atomically replaces that character's rows (mirrors the v1 clear+write contract; never row-diffs).
- [ ] **BACKEND-04**: Each guildie authenticates to the ingest API with an opaque per-guildie bearer token ("guild code"), minted by the maintainer and stored hashed server-side.
- [ ] **BACKEND-05**: The backend exposes a versioned read API that powers the website's four views (replacing the Sheet's view tabs as the query layer).
- [ ] **BACKEND-06**: The SQLite database is backed up nightly off-box (rsync or block-volume snapshot), with a documented restore procedure.

### Enrichment jobs (ENRICH)

- [ ] **ENRICH-10**: The daily PigParse price pull (server=1=Blue) runs as an in-process scheduled job, reusing the existing parser and `politeFetch` controls (User-Agent, ETag/If-Modified-Since, backoff).
- [ ] **ENRICH-11**: The weekly P1999 wiki scrape (per-item summaries, per-class spell lists, Velious gear tiers, quest items) runs as an in-process scheduled job with the same politeness controls.

### Watcher re-target (WATCH)

- [ ] **WATCH-08**: The watcher uploads inventory/spellbook snapshots to the backend HTTP API instead of Google Sheets, preserving the 500 ms debounce + always-re-read behavior.
- [ ] **WATCH-09**: All Google OAuth/PKCE/Sheets/Drive-Picker machinery is removed from the watcher (~2.5–3k LOC: `internal/auth`, `internal/sheet`, `internal/scaffold`, `internal/picker`, most of `internal/wizard`, reauth/propagation probes).
- [ ] **WATCH-10**: First-run onboarding is "paste your guild code" — the bearer token is stored in Windows Credential Manager (DPAPI); no browser OAuth flow.
- [ ] **WATCH-11**: Existing guildies receive the re-targeted watcher via the existing GitHub-Releases auto-updater (one manual step: paste guild code).

### Web frontend (WEB)

- [ ] **WEB-01**: A static web frontend renders the four consolidated views (`view`, `gear_check`, `spell_check`, `bank`) with the leading Char column, filterable and sortable (the work the Sheet did for free).
- [ ] **WEB-02**: `gear_check` and `spell_check` compute and display progression status — gear OK/MISSING/OTHER vs. Velious tiers; spell KNOWN/MISSING — matching the v1 semantics.
- [ ] **WEB-03**: Cross-character item search with "did you mean?" fuzzy matching (ports the Wagner-Fischer Levenshtein logic), returning results in <2s.
- [ ] **WEB-04**: Every inventory row shows a per-item tooltip (wiki summary + price + quest info) and a direct wiki link.
- [ ] **WEB-05**: The site carries an EQ aesthetic theme site-wide, building on `docs/design/eq-aesthetic-theme.md` (now with full CSS freedom vs. the Sheet's cell-styling limits).

### Website login (AUTH)

- [ ] **AUTH-08**: Website visitors sign in with Discord OAuth2, gated on membership in the guild's Discord server (no allowlist upkeep).
- [ ] **AUTH-09**: Each signed-in user's Discord identity is captured and stored, pre-paying the v2 Wantlist/Discord-pinger prerequisite (per-user Discord identity).

### Admin web forms (ADMIN)

- [ ] **ADMIN-04**: Eviction (30-day grace + archive) is available as an authenticated, officer-only web form (ports v1 eviction enforcement: `guild_admins` gate + owner-floor protection).
- [ ] **ADMIN-05**: Manual bank-coin entry (platinum/gold/silver/copper) is an authenticated web form (the file format still has no coin data; manual entry remains the only honest path).
- [ ] **ADMIN-06**: Admin/officer management (the `guild_admins` allowlist + owner-floor lockout protection) is an authenticated web form.

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
| BACKEND-01 | P11 Backend Foundation + Ingest API | (filled by `/gsd-plan-phase 11`) | Pending |
| BACKEND-02 | P11 Backend Foundation + Ingest API | (filled by `/gsd-plan-phase 11`) | Pending |
| BACKEND-03 | P11 Backend Foundation + Ingest API | (filled by `/gsd-plan-phase 11`) | Pending |
| BACKEND-04 | P11 Backend Foundation + Ingest API | (filled by `/gsd-plan-phase 11`) | Pending |
| BACKEND-06 | P11 Backend Foundation + Ingest API | (filled by `/gsd-plan-phase 11`) | Pending |
| ENRICH-10 | P12 Enrichment Job Migration | (filled by `/gsd-plan-phase 12`) | Pending |
| ENRICH-11 | P12 Enrichment Job Migration | (filled by `/gsd-plan-phase 12`) | Pending |
| WATCH-08 | P13 Watcher Re-Target + Onboarding | (filled by `/gsd-plan-phase 13`) | Pending |
| WATCH-09 | P13 Watcher Re-Target + Onboarding | (filled by `/gsd-plan-phase 13`) | Pending |
| WATCH-10 | P13 Watcher Re-Target + Onboarding | (filled by `/gsd-plan-phase 13`) | Pending |
| WATCH-11 | P13 Watcher Re-Target + Onboarding | (filled by `/gsd-plan-phase 13`) | Pending |
| BACKEND-05 | P14 Web Frontend | (filled by `/gsd-plan-phase 14`) | Pending |
| WEB-01 | P14 Web Frontend | (filled by `/gsd-plan-phase 14`) | Pending |
| WEB-02 | P14 Web Frontend | (filled by `/gsd-plan-phase 14`) | Pending |
| WEB-03 | P14 Web Frontend | (filled by `/gsd-plan-phase 14`) | Pending |
| WEB-04 | P14 Web Frontend | (filled by `/gsd-plan-phase 14`) | Pending |
| WEB-05 | P14 Web Frontend | (filled by `/gsd-plan-phase 14`) | Pending |
| AUTH-08 | P15 Admin Web Forms + Login | (filled by `/gsd-plan-phase 15`) | Pending |
| AUTH-09 | P15 Admin Web Forms + Login | (filled by `/gsd-plan-phase 15`) | Pending |
| ADMIN-04 | P15 Admin Web Forms + Login | (filled by `/gsd-plan-phase 15`) | Pending |
| ADMIN-05 | P15 Admin Web Forms + Login | (filled by `/gsd-plan-phase 15`) | Pending |
| ADMIN-06 | P15 Admin Web Forms + Login | (filled by `/gsd-plan-phase 15`) | Pending |
| CUTOVER-01 | P16 Cutover + Decommission | (filled by `/gsd-plan-phase 16`) | Pending |
| CUTOVER-02 | P16 Cutover + Decommission | (filled by `/gsd-plan-phase 16`) | Pending |
| CUTOVER-03 | P16 Cutover + Decommission | (filled by `/gsd-plan-phase 16`) | Pending |
| CUTOVER-04 | P16 Cutover + Decommission | (filled by `/gsd-plan-phase 16`) | Pending |

> Phase column **finalized by roadmap creation (2026-05-28)** — the provisional mapping was accepted unchanged (no concrete coverage problem found). Success criteria (2–5 observable behaviors per phase) live in `.planning/ROADMAP.md` § Phase Details. Plan column filled by `/gsd-plan-phase` per phase.

**Coverage check (2026-05-28, roadmap-finalized):** 26/26 requirements mapped to exactly one phase. No orphans, no duplicates. P11=5 (BACKEND-01/02/03/04/06), P12=2 (ENRICH-10/11), P13=4 (WATCH-08/09/10/11), P14=6 (BACKEND-05 + WEB-01/02/03/04/05), P15=5 (AUTH-08/09 + ADMIN-04/05/06), P16=4 (CUTOVER-01/02/03/04).

---

*Defined: 2026-05-28 at v2.0 milestone start. 26 requirements across 7 categories (Backend, Enrichment, Watcher, Web, Auth, Admin, Cutover). Phase mapping FINALIZED by roadmap creation 2026-05-28 (Phases 11–16); success criteria in ROADMAP.md § Phase Details.*
