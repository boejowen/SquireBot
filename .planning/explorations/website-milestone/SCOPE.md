# Milestone Scope — Website Frontend (working title: v2.0 "Off Google")

**Status:** Scoping draft — not yet an open milestone. Feeds a future `/gsd-new-milestone`.
**Created:** 2026-05-20
**Author:** Claude (overnight research fan-out — 4 parallel research agents)

## Why this milestone exists

SquireBot's watcher authenticates to Google via OAuth to write the shared spreadsheet.
Google's **brand verification** gate keeps blocking that OAuth client — and the root
cause is unfixable on the current path: Google requires the consent-screen homepage to
sit on a domain **registered to you** (bought from a registrar). `boejowen.github.io`
is GitHub's domain; it can never satisfy "registered to you" (rejected by Google's
reviewer on 2026-05-15: *"Your home page website is not registered to you"*).

Replacing the Google Sheet — as both the **frontend** and the **data store** — with a
website backed by the maintainer's own server **eliminates Google OAuth entirely**, and
with it the brand-verification gate, permanently. That is the milestone's purpose.

> **This does not unblock v1.0.2.** v1.0.2 still needs a registered domain for the
> current watcher's OAuth (a ~30-minute, ~$12/yr fix). The recommendation at the end
> of this doc is: unblock v1.0.2 with the domain now, then take this website work on
> deliberately as the next major milestone — not as a fire drill.

## Detailed research

Four findings documents accompany this synthesis:

- [`01-backend-hosting.md`](01-backend-hosting.md) — backend architecture, hosting, cost
- [`02-frontend-stack.md`](02-frontend-stack.md) — frontend stack, UI feature rebuild
- [`03-watcher-auth.md`](03-watcher-auth.md) — watcher re-target, auth, brand-verification escape
- [`04-data-enrichment-migration.md`](04-data-enrichment-migration.md) — DB schema, enrichment, cutover

## Recommended architecture

| Layer | Recommendation | Recurring cost |
|-------|----------------|----------------|
| **Backend** | Single-binary Go server (mirrors the watcher's shape; reuses Go expertise) on a fixed-price VPS (Hetzner CX22 ≈ $4.59/mo). Caddy for auto-HTTPS. In-process Go scheduler for enrichment cron. | ~$55/yr |
| **Database** | **SQLite** — embedded, one file, trivial off-box `rsync` backup. The data is tiny (<100 MB for years; ~50–150 writes/day). Postgres is needless ops overhead at this scale. *(See Open Decision 1 — research agents split on this.)* | $0 |
| **Frontend** | SvelteKit (static adapter) + TanStack Table + Tailwind, deployed to Cloudflare Pages (free static CDN, no server runtime). | $0 |
| **Watcher↔backend auth** | Per-guildie opaque bearer token ("guild code"), minted by the maintainer, stored hashed server-side, DM'd over Discord, stored client-side in DPAPI-backed Windows Credential Manager. | $0 |
| **Website human login** | Discord OAuth2, gated on the guild's existing Discord-server membership. No allowlist upkeep; pre-pays a v2 prerequisite. | $0 |

**Total incremental recurring cost: ~$55/yr** (VPS), plus an optional vanity domain.
A domain is still wanted for a memorable website URL — but it would **no longer need
Google verification**, so any cheap domain works and setup is trivial.

### Key architectural wins (beyond escaping Google)

- The Google **200-tab limit vanishes** — landing tabs become plain `inventory_item` /
  `spellbook_entry` rows keyed by `character_id`. The entire "consolidated vs.
  per-character views" locked decision becomes moot.
- The 4 view tabs (`view`, `gear_check`, `spell_check`, `bank`) stop being *storage* —
  they become **SQL queries / API endpoints**. The `onChange`/`buildView` rebuild
  triggers and search-cache machinery are deleted.
- `_char_owner` (which conflates guildie + character) splits into `owner` + `character`
  tables — owner-email changes become a one-row update; the watcher's brittle
  first-write-wins conflict logic disappears.
- The watcher gets **~2,000 LOC smaller** — its highest-complexity code (OAuth/PKCE,
  Sheets client, Drive Picker, wizard, reauth probes) is deleted.
- The `_meta.schema_version` ↔ `WatcherMaxSchemaVersion` handshake retires in favour of
  ordinary forward-only DB migrations (`goose`) + an API version.

## Watcher impact — bounded change, not a rewrite

| Survives untouched (~9–10k LOC) | Deleted (~2.5–3k LOC) | New (~400–600 LOC) |
|---|---|---|
| `internal/watch` (fsnotify + debounce), `internal/parse`, `internal/eqfind`, `internal/tray`, `internal/update` (auto-update pipeline), `internal/logging`, NSIS installer, orchestration skeleton | all of `internal/auth` (Google OAuth/PKCE/wincred), `internal/sheet` (Sheets v4 client), `internal/scaffold`, `internal/picker`, most of `internal/wizard`, the reauth/propagation-probe machinery | `internal/backend` HTTP client |

Existing guildies migrate via the **existing GitHub-Releases auto-updater** — the new
binary ships as an update; each guildie does one manual step: paste their guild code.

## Brand-verification escape — confirmed

The recommended path **fully eliminates every OAuth brand-verification dependency**. No
Google consent screen exists anywhere in the system. Discord OAuth2 login has no
brand-verification gate (Discord's "verification" is a separate bot-only concept). The
*only* option that would re-introduce the gate is "Sign in with Google" — explicitly
rejected. Magic-link email and GitHub OAuth are gate-free fallbacks if Discord login is
ever undesirable.

## Proposed phase roadmap

A genuine full milestone — comparable in size to the original v1.0 (5 phases).

| Phase | Goal | Effort (part-time days) |
|-------|------|--------------------------|
| **A — Backend foundation & ingest API** | VPS + Caddy live; SQLite schema + `goose` migrations; the upload-receiving API; per-guildie bearer-token auth. Gate: server accepts a test upload. | 5–8 |
| **B — Enrichment job migration** | Port the daily PigParse pull + weekly wiki scrape to in-process scheduled jobs (parsers port near-verbatim; politeness controls carry over). | 2–4 |
| **C — Watcher re-target & onboarding** | Swap the Sheets client for the `internal/backend` HTTP client; delete the OAuth machinery; new onboarding (paste guild code). Gate: re-targeted watcher binary on Releases. | 4–6 |
| **D — Web frontend** | SvelteKit app; reusable `<DataGrid>` (filter/sort/the work the Sheet did for free); the 4 views; client-side search + "did you mean?"; HTML tooltips; EQ theming. | 6–9 |
| **E — Admin web forms + login** | Discord OAuth2 login; eviction, bank-coin, admin-management as web forms. | 3–5 |
| **F — Cutover** | Shadow-mode soak (backend runs alongside the live sheet, 1–2 wk calendar); one-time backfill of human-supplied data only (owner/character metadata, bank coin, archives — dimension data self-populates); coordinated watcher self-update flips ingest; decommission the sheet. | 2–3 active + 1–2 wk calendar |

**Total active development: ~22–35 part-time days ≈ 4–7 weeks of evening work.**

### Effort lever — PocketBase

Backend research flagged **PocketBase** (open-source Go + SQLite + auth + REST + admin
UI, single binary) as almost exactly the recommended backend design, pre-built.
Adopting it could compress Phases A and E by an estimated **5–8 days** — at the cost of
a framework dependency and less control. Worth a 1-day spike before committing to a
hand-rolled server. *(See Open Decision 2.)*

## Cost summary

| | Today | Sheet path (v1.0.2) | Website path (v2.0) |
|---|---|---|---|
| Recurring | $0 | ~$12/yr (domain for OAuth) | ~$55/yr (VPS) + optional vanity domain |
| One-time setup | — | ~30 min | 4–7 weeks dev |
| Google dependency | Yes (blocking) | Yes (verified once, then stable) | **None** |
| Ops burden | None | None | Low (one VPS, nightly rsync backup) |

## Cutover safety

The recommended cutover (hybrid shadow-mode) **never writes to the Google Sheet** — the
new system runs in parallel, ingesting in shadow against the still-live sheet for 1–2
weeks before a single coordinated flip. Cutover therefore cannot corrupt the live
product. Both inventory and enrichment data are self-healing (next watcher upload / next
enrichment run), so a botched flip is recoverable.

## Risks

- **Scope creep** — 4–7 weeks is the realistic floor; a solo part-time maintainer should
  expect calendar slippage. The phase gates keep it shippable incrementally.
- **Ops ownership** — Google currently handles uptime/backups for free. A VPS makes the
  maintainer the sysadmin. Mitigated by: fixed-price host (no surprise bills), single
  binary, automated nightly off-box backup. Still a real, permanent new responsibility.
- **Bus factor** — today the data survives in a Google Sheet anyone can open. A
  self-hosted DB needs the backup discipline to actually be followed.
- **Guildie migration friction** — every guildie must paste a guild code once. Low, but
  non-zero coordination for ~12 people.
- **Discord-as-gatekeeper** — website access depends on Discord-server membership; fine
  today, but couples the tool to the guild's Discord.

## Open decisions for the maintainer

1. **SQLite vs Postgres.** Backend research recommends SQLite (lower ops, fits the tiny
   data); data-model research drafted a Postgres schema. The schema is relational either
   way and ports cleanly — **recommendation: SQLite.** Decide before Phase A.
2. **PocketBase vs hand-rolled Go server.** PocketBase could cut ~5–8 days but adds a
   framework dependency. Recommendation: 1-day spike at the start of Phase A.
3. **VPS provider** — Hetzner recommended; any fixed-price host works.
4. **Website login** — Discord OAuth2 recommended (zero allowlist upkeep); magic-link is
   the gate-free fallback.
5. **Timing** — recommended: register the ~$12 domain now to unblock v1.0.2 and its 12
   stranded guildies, ship v1.0.2, *then* open this as v2.0 deliberately. The website
   work is identical whether done now or in a month.

## Recommendation

The website milestone is **viable, well-bounded, and strategically sound** — it
permanently escapes Google, modernizes the architecture, and pre-pays v2 (Wantlist +
Discord pinger) groundwork. But it is a **4–7 week milestone**, not a workaround.

**Do both, in order:**
1. **Now:** register a domain (~$12/yr, ~30 min), pass v1.0.2 brand verification, ship
   v1.0.2. This unblocks the guild today.
2. **Next:** open this as milestone **v2.0** via `/gsd-new-milestone`, using this SCOPE
   doc and the four findings files as the research input.

Treating the website as the *deliberate next milestone* — rather than an emergency
escape from a 30-minute domain task — is the right call.
