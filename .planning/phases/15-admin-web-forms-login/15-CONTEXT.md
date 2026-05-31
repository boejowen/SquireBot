# Phase 15: Admin Web Forms + Login - Context

**Gathered:** 2026-05-30
**Status:** Ready for planning

<domain>
## Phase Boundary

Turn the (currently public, read-only) P14 website into a **members-only, write-capable app**:

- **Discord OAuth2 login** gated on guild Discord-server membership, capturing per-user Discord identity (AUTH-08 / AUTH-09).
- Three authenticated web forms porting v1's enforcement: **eviction** (ADMIN-04, officer-only), **bank-coin entry** (ADMIN-05, any authenticated member), **admin/officer management** (ADMIN-06, officer-only + owner-floor protection).

Backend = the existing hand-rolled `net/http` + `modernc.org/sqlite` + `goose` + `/api/v1` server (P11/P14). Login = hand-rolled `golang.org/x/oauth2` (the 11-01 spike verdict explicitly flagged "P15 Discord OAuth2 is hand-rolled, NOT PocketBase"). Frontend = the existing `web/` SvelteKit app (P14) extended with a login flow + auth-gated routing + the three forms.

**Out of scope (later/other phases):** the v2 Wantlist + Discord pinger — this phase only PRE-PAYS the per-user Discord-identity prerequisite via AUTH-09; the pinger itself stays deferred (needs Raid Alliance bot invites). Shadow-soak / human-data backfill / coordinated watcher flip / Sheet + Apps Script + Google-OAuth decommission → P16.

</domain>

<decisions>
## Implementation Decisions

> **Discussion shape (2026-05-30):** the user selected **all four** gray areas and accepted the **recommended option on every question** (the standing [[feedback_delegate_gray_areas]] pattern — same as Phases 11 and 14). Everything below is LOCKED — downstream research/planning should NOT re-ask. Detailed visual/interaction layout of the login flow + 3 forms is deliberately deferred to `/gsd-ui-phase 15`.

### Login gate + identity (AUTH-08 / AUTH-09)
- **D-01: Whole-site gate.** Discord login walls the **entire** site — nothing is viewable without sign-in. The membership gate is enforced at the **read API** (every `/api/v1/...` read endpoint requires a valid session), not just at the SvelteKit frontend (frontend-only gating is trivially bypassable). Non-members of the guild Discord server are refused. P14's public-but-unlisted launch was always the stopgap; this closes it.
- **D-02: Scopes = `identify` + `guilds`.** Membership check = the configured guild ID is present in the user's `guilds` list — **no Discord bot needed** (finding 03 §4.2 confirms this path has no brand-verification / app-review gate). **No `email` scope** (not needed; keeps the consent screen minimal).
- **D-03: Identity stored (AUTH-09) = `discord_user_id` (snowflake) + `username` + `avatar`.** Upserted on each login. The snowflake is the stable key and is exactly the handle the deferred **v2 Discord pinger** will DM — this is that prerequisite, paid down here.
- **D-04: Login = hand-rolled `golang.org/x/oauth2`** authorization-code flow, **server-side code exchange**. The Discord **client secret lives only on the backend** (env / systemd secret) — never in the static frontend bundle. Locked by the 11-01 spike verdict (no PocketBase provider).

### Session
- **D-05: Server-side opaque session.** On a successful OAuth + membership pass, mint a random opaque session id, store it **hashed** in a new `web_session` table (→ `discord_user_id`), and set it as an **httpOnly + Secure + SameSite=Lax cookie scoped to `Domain=squirebot.quest`** so it rides cross-subdomain to `api.squirebot.quest` (same registrable domain ⇒ same-site for SameSite=Lax). The read API's CORS already pins the exact origin (14-03); add `Access-Control-Allow-Credentials: true`. **Session TTL ≈ 30 days, rolling** (low-sensitivity hobby data; re-login friction is bad for 12 people). A departed guildie loses access naturally: **membership is re-checked at each login** and the bounded session expires.
  - *Exact cookie attributes, the OAuth redirect/callback route shape, and the cross-subdomain cookie/CORS-credentials mechanics are a flagged research item (see canonical_refs + Claude's Discretion) — the choice (opaque server-side session, cross-subdomain cookie) is locked; the mechanics are for research to nail down.*

### Officer model + owner-floor (ADMIN-06)
- **D-06: Officers = a ported `guild_admins` allowlist keyed by Discord user ID** (snowflake), stored in the **DB** (not v1's `_meta` JSON row). Port v1 `admin.ts` enforcement **semantics verbatim**: fail-closed `requireAdmin`; **authorize inside the write transaction** (close the TOCTOU window v1's WR-04 fix addressed — a just-removed admin must not get one final write through); idempotent add/remove; **append-only admin audit log** (reuse/extend the existing `audit_log` table).
- **D-07: Add-officer UX = pick from users who've already logged in.** The admin-mgmt form lists captured Discord identities (D-03); the admin clicks to promote. No snowflakes typed, no typos. Trade-off accepted: a guildie must sign in once before they can be promoted.
- **D-08: Owner-floor = a CLI-seeded maintainer Discord ID.** A `squirebot-server set-owner-floor <discord-id>` subcommand (mirrors P11's `mint-code` / `revoke-code` CLI), run **once at deploy**, designates the un-removable super-admin: a peer admin cannot remove the floor (self-removal follows v1's documented orphan-pointer rule). The seeded ID is also the **first/bootstrap admin**. This replaces v1's `onOpen` + `getOwner()` bootstrap, which no longer exists.

### Eviction (ADMIN-04 — officer-only)
- **D-09: Eviction targets a whole guildie (owner).** Pick a guildie → cascade `is_removed = 1` across **all** their `character` rows. Exactly v1's behavior (v1 evicted by owner email and flipped every matching character). Officer-only (`guild_admins` gate); port the owner-floor protection so a peer cannot evict the floor's own guildie data.
- **D-10: Eviction also revokes the owner's guild code immediately** (set `guild_code.disabled_at`) so their watcher stops uploading — the DB-world "one clean app-controlled action" the roadmap Note highlights (vs v1's separate Google-Drive un-share). **AND keeps v1's 30-day grace + auto-archive** of the data. **Reversible during grace:** un-set `is_removed` + re-mint a guild code. Requires a grace/archive mechanism in the schema (e.g. `grace_until` + `archived_at`) plus a scheduled archive step — port `weeklyEvictionArchive`'s 30-day logic.

### Bank-coin (ADMIN-05 — any authenticated member)
- **D-11: Coin entry limited to `is_bank_toon` characters.** Matches v1's "manual platinum for the bank toon" intent. Stored as **nullable `plat` / `gold` / `silver` / `copper` integer columns on `character`** (the `/outputfile` format still carries no coin — manual entry remains the only honest path). The form writes them; the **bank view surfaces them**, replacing P14's null/0 coin placeholder.
- **D-12: Bank-coin entry is gated by login only, NOT officer-only.** Deliberate read of the requirement wording: ADMIN-04 (eviction) and ADMIN-06 (admin-mgmt) say *officer-only*; **ADMIN-05 says only "authenticated"** — so any signed-in guild member may update the shared bank's coin count. *(Tighten to officer-only later if the guild prefers; flagged as the one interpretation call, not re-asked.)*

### Claude's Discretion
- Session/cookie exact attributes + the OAuth redirect-callback route shape + cross-subdomain cookie + CORS-credentials mechanics (flagged research item; the posture is locked in D-05).
- Exact new `goose` migration DDL (web-user / `web_session` / `guild_admins` / coin columns / eviction grace+archive) — extend-only, forward-only, mirrors P11's `00001` shape; lands as `00004_*` (next after `00003`).
- Whether `guild_admins` is its own table or an `is_admin`/`role` column on the web-user table; whether owner-floor is a singleton row or read from config.
- Coin input validation/units (gold/silver/copper bounded 0–999 vs free integers).
- Whether the eviction archive is a new job in the existing in-process scheduler or a lazy-on-read archive (port `weeklyEvictionArchive`'s approach either way).
- All form + login-screen visual/interaction layout → `/gsd-ui-phase 15` (the UI safety gate applies — roadmap "UI hint: yes").

</decisions>

<specifics>
## Specific Ideas

- **"Off Google" stays absolute.** Discord OAuth2 was chosen precisely because it has **no brand-verification / app-review gate** (finding 03 §4.2); "Sign in with Google" is rejected outright — it re-introduces the exact gate v2.0 exists to escape.
- **The DB world makes enforcement cleaner.** Eviction becomes one app-controlled action (mark removed + revoke code + grace), not a Sheet edit plus a separate Drive un-share. Lean into that (D-10).
- **Reuse the battle-tested v1 enforcement, port the SEMANTICS not the storage.** `admin.ts` (allowlist + owner-floor + TOCTOU-safe, lock-wrapped mutators) and `showEvictionSidebar.ts` (per-owner cascade + 30-day grace + `eviction_log` envelope) are the behavioral oracle — reimplement in Go/SQL transactions; don't reinvent the rules.
- **The login choice pre-pays a v2 prerequisite for free** — per-user Discord identity capture (AUTH-09) is the Wantlist/pinger groundwork.

</specifics>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase + milestone planning
- `.planning/ROADMAP.md` § "Phase 15: Admin Web Forms + Login" — goal, the 5 success criteria, and the Note (Discord = no brand-verification gate; eviction/admin enforcement gets cleaner in the DB world)
- `.planning/REQUIREMENTS.md` § AUTH (AUTH-08/09) + § ADMIN (ADMIN-04/05/06) — acceptance detail; note ADMIN-04/06 are officer-only, ADMIN-05 is authenticated-only (D-12)
- `.planning/PROJECT.md` § Current Milestone + Constraints + Key Decisions — Discord-not-Google, owner/character split, Off-Google ethos

### v2.0 milestone research (THE key auth doc)
- `.planning/explorations/website-milestone/03-watcher-auth.md` §3 (login options compared) + §4 (brand-verification escape analysis) — Discord OAuth2 scopes (`identify`+`guilds`), membership-gate-via-guilds-list, the definitive "no gate" confirmation, and why Google sign-in is rejected
- `.planning/explorations/website-milestone/SCOPE.md` — milestone synthesis; login + admin-forms direction
- `.planning/explorations/website-milestone/04-data-enrichment-migration.md` §6 — DB schema notes (read for the new migration's table shapes / joins)

### Backend — extend / mirror
- `internal/backendsrv/migrations/00001_init.sql` — current schema (`owner` / `character` (has `is_bank_toon`, `is_removed`) / `guild_code` (has `disabled_at`) / dimension tables). The new `00004_*` goose migration adds web-user, `web_session`, `guild_admins`, coin columns, and eviction grace/archive.
- `internal/backendsrv/migrations/00002_audit.sql` — `audit_log` table to **reuse/extend** for admin + eviction + coin write audit (don't invent a parallel log)
- `internal/backendsrv/auth/*.go` — the existing bearer `ResolveToken` guard; the Discord OAuth + session layer lives **alongside** it (separate human-session path vs the watcher bearer path)
- `internal/backendsrv/readapi/*.go` + the exact-origin CORS (14-03) — the session gate now **wraps** these; CORS gains `Allow-Credentials: true`
- `internal/backendsrv/ingest/{handler,whoami,version}.go` — established `/api/v1` ServeMux/handler patterns to mirror for the new auth + form endpoints
- `cmd/squirebot-server` (`serve` / `mint-code` / `revoke-code`) — where the new `set-owner-floor` subcommand lands (D-08)
- `.planning/phases/11-backend-foundation-ingest-api/11-CONTEXT.md` § `<spike_verdict>` — "P15 Discord OAuth2 is hand-rolled `golang.org/x/oauth2`, not PocketBase" (D-04)

### v1 enforcement to PORT (behavioral oracle)
- `apps-script/src/lib/admin.ts` — `guild_admins` allowlist + owner-floor + `admin_log`; fail-closed `requireAdminOrThrow`, authorize-under-lock (TOCTOU/WR-04), idempotent add/remove, owner-floor protection (D-06/D-07/D-08)
- `apps-script/src/triggers/showEvictionSidebar.ts` — eviction: per-owner cascade `is_removed` + 30-day grace + `eviction_log` envelope shape + confirm-before-commit UX (D-09/D-10)
- `apps-script/src/triggers/weeklyEvictionArchive.ts` — the 30-day-grace archive job to port (D-10)
- `apps-script/src/triggers/showAdminMgmtSidebar.ts` — admin-mgmt UX reference for ADMIN-06

### Frontend — extend (P14)
- `.planning/phases/14-web-frontend/14-CONTEXT.md` — the read-API/CORS/SiteShell/theme decisions this phase builds on (esp. D-04 public-now/gate-in-P15 forward dependency, now honored by D-01)
- `web/` SvelteKit app (the P14 tree) — add the login flow + auth-gated routing + the 3 forms; the `SiteShell` gains auth state + a login screen
- `docs/design/eq-aesthetic-theme.md` — the forms + login screen carry the EQ theme site-wide (consistent with P14 / WEB-05)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`internal/backendsrv/auth`** (bearer `ResolveToken`) — the Discord-OAuth + human-session layer is a sibling auth path in the same package home.
- **`internal/backendsrv/readapi` + CORS** (14-03) — the session gate wraps these; CORS is already exact-origin, just add credentials.
- **`internal/backendsrv/migrations`** (goose, `//go:embed *.sql`) — add `00004_*` forward-only; `goose.Up` runs on startup (deploy = drop binary + restart).
- **`audit_log`** (00002) — reuse/extend for admin + eviction + coin write audit.
- **`cmd/squirebot-server`** (`serve`/`mint-code`/`revoke-code`) — add `set-owner-floor`.
- **v1 `admin.ts` / `showEvictionSidebar.ts` / `weeklyEvictionArchive.ts` / `showAdminMgmtSidebar.ts`** — semantic oracle for ADMIN-04/06.
- **`web/` SvelteKit app** (P14) — login + auth routing + 3 forms extend it.

### Established Patterns
- `/api/v1/...` versioned endpoints; hand-rolled `net/http` ServeMux; `modernc.org/sqlite` store methods; **no web framework** on the backend.
- **Fail-closed authorization + authorize-under-transaction** (v1 `admin.ts` TOCTOU fix) — port to SQL transactions.
- **Owner/character split**: eviction cascades owner → characters; coin attaches to `is_bank_toon` characters.
- **CLI-as-ops** (`mint-code`/`revoke-code`) — `set-owner-floor` follows the same pattern.

### Integration Points
- Discord OAuth callback → backend session mint → cross-subdomain cookie → SvelteKit reads auth state.
- The session gate now **fronts the P14 read API** (D-01) — previously public.
- Bank-coin form → `character` coin columns → bank view (fills P14's null/0 placeholder).
- Eviction → `is_removed` cascade + `guild_code.disabled_at` revoke + grace/archive.
- (P16) the login + write forms must exist **before** the Sheet's admin sidebars can retire.

</code_context>

<deferred>
## Deferred Ideas

- **v2 Wantlist + EC/WTS Discord pinger** (WANT-01..08) — this phase only captures the Discord-identity prerequisite (AUTH-09); the pinger stays deferred (needs Raid Alliance bot invites).
- **Per-owner / per-member visibility tiers** — universal visibility remains (every member sees everything). The DB makes tiers cheap to add later, but out of scope (REQUIREMENTS Out-of-Scope).
- **Tighten bank-coin to officer-only** — D-12 reads ADMIN-05 as authenticated-member; the guild can restrict it later if desired.
- **Detailed login-screen + form visual/interaction layout** → `/gsd-ui-phase 15` (UI hint: yes; UI safety gate applies).
- **Discord-app provisioning is a maintainer prerequisite** — create the Discord application, register the redirect URI, obtain client id + secret (secret server-side only). Like the Hetzner/domain prereqs; **flag in research** so the planner surfaces it before build.
- **Shadow-soak / human-data backfill** (incl. coin + archive entries) / **coordinated watcher flip** / **Sheet + Apps Script + Google-OAuth decommission** → **P16**.

</deferred>

---

*Phase: 15-admin-web-forms-login*
*Context gathered: 2026-05-30*
