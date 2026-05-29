# Website Milestone — Backend & Hosting Research

**Research slice:** Backend architecture, database, hosting, scheduled jobs, durability, cost.
**Date:** 2026-05-20
**Status:** Scoping / go-no-go input. No application code touched.
**Confidence:** HIGH on load sizing and free-tier mechanics; HIGH on the recommendation; MEDIUM on exact 2026 pricing (verify at purchase — prices below were checked May 2026).

---

## TL;DR

A small **single-binary Go server on a fixed-price VPS** (Hetzner CX22, ~$4.59/mo / ~$55/yr) with **SQLite** as the database is the recommended backend. It is the only option that simultaneously: (a) genuinely drops the Google OAuth dependency that is the *reason for this milestone*, (b) reuses the maintainer's existing Go expertise, (c) has a flat, predictable cost with no metering surprises, (d) imposes no "free-tier shutdown / pause / 30-day expiry" failure modes, and (e) makes backups a one-line `rsync` of a single file.

**Runner-up: Supabase free tier** ($0/mo) — attractive cost but carries a 7-day inactivity auto-pause, a 500 MB ceiling, a vendor lock-in surface, and *still requires you to build and host a server somewhere for watcher auth + enrichment cron*, so it does not actually eliminate the "host a thing" problem.

The headline number: **~$55/year** for the recommended path, versus **$0/year today**. That is the real cost of escaping Google OAuth brand verification.

---

## Framing: what the backend must actually do

Before comparing options, pin down the workload. The new backend has five jobs:

1. **Receive watcher uploads.** ~12 watchers POST inventory/spellbook snapshots. Each snapshot is at most a few hundred KB; realistically <50 KB (a maxed P99 character is ~250 rows of short TSV). Uploads happen a few times a day per *active* guildie — call it 50–150 writes/day guild-wide, bursty but tiny.
2. **Authenticate watchers** without Google OAuth. This is the whole point of the milestone. Replacement is a per-guildie API token (issued once, stored in `wincred` exactly where the refresh token lives today). No brand verification, no consent screen, no Google.
3. **Run enrichment jobs.** One daily PigParse pull (`GET /api/item/getall/1`), one weekly P1999 wiki MediaWiki scrape. Both are currently Apps Script time-driven triggers; they must move server-side.
4. **Store the relational-ish data** currently spread across landing/dimension/view tabs.
5. **Serve the web frontend** — the consolidated `view` / `gear_check` / `spell_check` / `bank` views plus cross-character search. ~12 users, all read-mostly, no concurrency to speak of.

Key consequence: **a pure managed database (Supabase/Firebase) does not eliminate hosting.** Items 2 and 3 still need *somewhere to run code*. A BaaS replaces the database and maybe auth, but the daily cron and the watcher-token-issuing endpoint still need a process. So the real choice is "custom server + its own DB" vs. "custom server + managed DB" vs. "serverless functions + managed DB" — every path hosts code.

---

## Options Compared

### Option A — Managed Backend-as-a-Service (Supabase / Firebase / PocketHost)

**Supabase free tier (May 2026):** Postgres 500 MB, 1 GB file storage, 5 GB egress, unlimited API requests, built-in Auth, built-in REST/realtime, 2 projects max. **$0/mo.** Pro is **$25/mo** and removes the pause.

- **What you get free:** a real Postgres, row-level-security auth, auto-generated REST API, a dashboard. Genuinely a lot.
- **The killer limit: 7-day inactivity auto-pause.** Free projects pause after one week with *no database activity*. A guild of 12 hobbyists absolutely will have quiet weeks (holidays, between-raid lulls). On pause, the next request eats a ~30 s cold start, and the watcher's scheduled upload would fail until something wakes it. Mitigation is a cron that pings the DB — but you need a process to run that cron, which is the hosting problem you were trying to avoid. It is a papered-over crack.
- **500 MB ceiling.** Fine for current data (the architecture doc sizes the whole workbook at <100 K cells), but PigParse's full item universe is ~5 K rows and wiki tables grow; 500 MB is comfortable for years but it is a ceiling you do not control.
- **Vendor lock-in.** RLS policies, the auth schema, and Supabase-specific client libraries are not portable. Migrating off later is real work.
- **Still need a server for cron + watcher token issuance.** Supabase has "Edge Functions" (Deno) and `pg_cron`, so you *can* technically do the daily PigParse pull in-platform — but now your enrichment logic is Deno/TypeScript in a Supabase-proprietary runtime, not Go, and the maintainer is "comfortable with Go and TypeScript" but the operational story is split across two runtimes.

**Firebase:** Firestore is a document store — a poor fit for the relational join-heavy schema (item_id joins, gear-tier-vs-equipped joins). The Spark (free) plan has tightened repeatedly and the Blaze pay-as-you-go plan is metered (read/write/egress) — exactly the unpredictable-bill failure mode a hobby project should avoid. **Not recommended.**

**PocketHost (hosted PocketBase):** PocketBase is a single-Go-binary backend (SQLite + auth + REST + admin UI + JS hooks) — architecturally a *very* close fit. PocketHost hosts it free with daily backups and SFTP export. Two caveats: (1) it is a small, single-operator free service — its own durability/longevity is a risk for a project that wants low operational anxiety; (2) self-hosting PocketBase on the same VPS as Option B is strictly more robust and barely more work. PocketBase itself is worth knowing about (see "Recommendation" note), but *PocketHost as the host* is not the pick.

### Option B — Lightweight custom Go server on a fixed-price VPS  ✅ RECOMMENDED

A single statically-linked Go binary (the maintainer already ships one of these — the watcher) running on a small VPS behind Caddy (automatic HTTPS, ~10 lines of config). SQLite file on local disk. `cron` or an in-process Go ticker for the daily/weekly enrichment.

- **Hetzner CX22:** 2 vCPU, 4 GB RAM, 40 GB SSD, 20 TB traffic — **€3.79 / ~$4.59 per month** (~$55/yr). Wildly over-provisioned for 12 users, which is the point: zero scaling anxiety for the life of the project.
- **Reuses Go expertise directly.** The enrichment jobs (PigParse JSON client, MediaWiki API client, the `politeFetch` etiquette helper) port from Apps Script TypeScript to Go cleanly — same HTTP shapes. The frontend can be Go-templated HTML or a small TS/React SPA served as static files; the maintainer is comfortable with both.
- **Drops Google entirely.** Watcher auth becomes a bearer token. No OAuth, no consent screen, no brand verification, no `drive.file` propagation delays. *This is the milestone's whole reason for existing, and only the custom-server paths fully deliver it.*
- **Flat, predictable cost.** No metering. No "you got popular" surprise (you won't, but also: no surprise). No free-tier expiry.
- **PaaS variant (Render / Railway / Fly):** same custom-Go-server idea on a managed platform instead of a raw VPS. Costs more and reintroduces failure modes — see Cost Projection. The VPS is preferred *because* it has no spin-down and no DB-expiry clock.

### Option C — Serverless / edge functions (Cloudflare Workers + D1)

Cloudflare Workers free tier: 100 K requests/day, cron triggers included free (3 per worker), D1 (SQLite-backed) free tier 5 GB / 5 M row-reads/day / 100 K row-writes/day.

- **Genuinely $0 and no cold-pause** (Workers have near-zero cold start; D1 does not "pause").
- **Cron triggers are first-class and free** — the daily PigParse / weekly wiki jobs fit cleanly.
- **But:** Workers is a JavaScript/TypeScript-and-WASM runtime — the maintainer's Go does not run natively (TinyGo→WASM exists but is a sharp edge, not a happy path). So this path means rewriting backend + enrichment in TypeScript. The maintainer knows TS, but it abandons the existing Go investment.
- **Request/subrequest limits and the 10 ms–30 ms CPU budget** make the weekly wiki scrape (many sequential polite-fetches with 1 s sleeps) awkward — long-running sequential work is the *anti-pattern* for Workers; you would need Durable Objects or Queues, adding architecture.
- **Verdict:** technically free and durable, but it trades the maintainer's strongest skill for platform-shaped complexity. Reasonable third place; not the pick.

### Quick comparison

| | A: Supabase free | B: Go on Hetzner VPS | C: CF Workers + D1 |
|---|---|---|---|
| Monthly cost | $0 (Pro $25 to kill pause) | ~$4.59 | $0 |
| Drops Google OAuth | Partially* | **Fully** | Fully |
| Reuses Go skill | No (Deno/SQL) | **Yes** | No (TS/WASM) |
| Cold-pause / spin-down | **Yes, 7-day** | No | No |
| Free-tier expiry clock | No (but pause) | N/A | No |
| Cron support | pg_cron / Edge Fn | cron / Go ticker | **Native, free** |
| Backups | Managed (7-day on free) | DIY (one file) | D1 Time Travel (30-day) |
| Long sequential wiki scrape | OK (Edge Fn) | **Trivial** | Awkward |
| Vendor lock-in | High | **None** | Medium-high |
| Babysitting | Low-ish | Low-medium | Low |

\* Supabase still needs a process somewhere for cron + token issuance, so "hosting" is not actually eliminated.

---

## Database Choice

**Recommendation: SQLite** (a single file on the server's disk), accessed from Go via `modernc.org/sqlite` (pure-Go, no CGO — keeps the single-static-binary property the maintainer already values).

Rationale:

- **The schema is relational-ish but tiny.** The sheet's "tabs" are tables; the joins (inventory `item_id` → `_item_master` → `_pigparse` price; equipped slot → `_wiki_gear_tier`) are exactly relational joins. A document store (Firestore) would force denormalization and client-side joins for no benefit. So: relational, not document.
- **Postgres is the "correct" relational answer but is operational overhead the project does not need.** A separate Postgres process means another thing to install, secure, back up, patch, and monitor. At 12 users / <100 MB of data / single-writer-ish load, Postgres buys nothing SQLite lacks.
- **SQLite at this scale is not a compromise — it is the right tool.** Hundreds of writes/day and a dozen concurrent readers is *trivial* for SQLite (it handles far more). Enable WAL mode for concurrent read-during-write. The whole database is one file.
- **Backups become trivial** (see below) and **durability is excellent** — SQLite is one of the most-tested pieces of software in existence.
- **It matches the maintainer's instincts.** The watcher is a single Go binary with local state; a Go server with an embedded SQLite file is the same philosophy. PocketBase (the BaaS in Option A) is literally Go+SQLite — that the closest-fitting managed product is built on exactly this stack is a strong signal.

If the project ever genuinely outgrew SQLite (it will not at guild scale), `modernc.org/sqlite` → `pgx`/Postgres is a contained migration. Design the data layer behind a small Go interface and even that is cheap insurance.

---

## Load Sizing

| Dimension | Realistic figure | Headroom on recommended path |
|---|---|---|
| Users | ~12 | VPS handles thousands |
| Characters | ~120 | trivial |
| Inventory snapshot size | <50 KB typical, "few hundred KB" worst case | trivial |
| Watcher uploads | ~50–150 writes/day guild-wide, bursty | SQLite does this in milliseconds |
| Daily job | 1 PigParse pull (~5 K item rows JSON) | one HTTP call + one transaction |
| Weekly job | 1 wiki scrape (dozens–low-hundreds of polite fetches) | minutes of wall-clock, no execution cap on a VPS |
| Total DB size | <100 MB for years (5 K PigParse rows + wiki tables + 120 chars × ~250 rows) | CX22 has 40 GB disk; D1/Supabase free both >500 MB |
| Concurrent web readers | ~12, read-mostly | nothing |
| Egress | a dozen people loading a small web app — well under 1 GB/mo | CX22 includes 20 TB; CF/Supabase free both fine |

**Do the free tiers cover this load? Yes — comfortably, on every option.** The data and traffic are genuinely tiny. The deciding factors are therefore *not* capacity. They are: (1) does the option actually remove Google OAuth, (2) does it reuse Go, (3) does it have a hidden failure mode (pause / spin-down / expiry / metered bill). On those axes the fixed-price VPS wins despite not being the cheapest.

---

## Cost Projection

All figures May 2026; verify at purchase.

| Option | Monthly | Yearly | Notes |
|---|---|---|---|
| **Hetzner CX22 VPS (recommended)** | **~$4.59** | **~$55** | Flat. 2 vCPU/4 GB/40 GB. Domain optional, see below. |
| Hetzner CX22 + domain name | ~$4.59 + ~$1–1.25 | **~$67–70** | A `*.duckdns.org` or a free subdomain avoids even this; a real domain is ~$12–15/yr. |
| Supabase free | $0 | $0 | Until a quiet week pauses it; Pro is $25/mo = $300/yr to remove pause. |
| Cloudflare Workers + D1 free | $0 | $0 | Genuinely free at this scale; cost is the TS rewrite, not dollars. |
| Render (web service + Postgres) | $7 (service) + $7 (DB) | ~$168 | Free Postgres **expires 30 days after creation** — unusable for a persistent project. Paid tier required. |
| Railway | ~$5 + usage | ~$60+ | Hobby plan is $5/mo *including* $5 usage credit; predictable but no cheaper than the VPS and metered above the credit. Free plan is only $1 credit/mo — not viable. |
| Fly.io | ~$4–10 | ~$50–120 | No real free tier since Oct 2024 (pay-as-you-go). A tiny always-on VM (~$2/mo) + volume + managed Postgres lands $5–10/mo. Comparable to the VPS but with a more complex billing surface. |
| Oracle Cloud Always Free (ARM VM) | $0 | $0 | 4 OCPU / 24 GB ARM "always free" — *technically* a free Option-B host. But: notorious ARM-capacity unavailability at signup, periodic "reclaim idle instances" sweeps, and a billing relationship with Oracle that can surprise. High-anxiety for a low-anxiety project. Mentioned for completeness; not recommended. |

**Recommended path: ~$55/year** (VPS only) **to ~$70/year** (VPS + a vanity domain). Against today's $0, that is the price of the milestone. It is small, flat, and predictable — the most important property for a single part-time maintainer is *no bill surprises and no expiry clock*, and a fixed-price VPS delivers exactly that.

---

## Durability / Backups — and "what if the maintainer stops paying?"

**On the recommended VPS path:**

- **The database is one file.** Back it up with `sqlite3 db .backup` (or the `VACUUM INTO` snapshot) on a nightly cron, then `rsync`/`rclone` the snapshot off-box — to a second cheap location, a Backblaze B2 bucket (pennies/month or within free allowance at this size), or even the maintainer's own PC. A second copy in a private GitHub repo is also viable given the data is <100 MB and non-secret.
- **Durability of the live data:** Hetzner volumes are redundant; SQLite itself is extremely robust. WAL mode + a nightly off-box snapshot means worst-case data loss is "since last snapshot," i.e. <24 h. That is *better* than the current Google Sheet, which has no maintainer-controlled backup at all beyond Google's version history.
- **"Stops paying" scenario:** if the VPS lapses, the server goes offline but **the maintainer holds the data** (the latest off-box snapshot). Spin up any new VPS, drop the file in, restart the binary — back online in minutes. Nothing is *lost*, nothing is *held hostage*. This is the single biggest durability advantage over every managed option.

**On managed options, the "stops paying / tier shuts down" story is worse:**

- **Supabase free:** data is *preserved* across a pause, but a paused project that is never resumed, or a free tier discontinued, puts the data behind a login and a possible upgrade wall. You can `pg_dump` proactively — but that, again, needs a process to run it on schedule.
- **Render free Postgres:** **deletes the database 30 days after creation** (14-day grace). This is an outright disqualifier for a persistent hobby project on the *free* tier — only the paid DB is durable.
- **Cloudflare D1:** "Time Travel" gives 30-day point-in-time restore; reasonably durable, but the data lives in Cloudflare's account and a closed account takes it.
- **PocketHost:** daily backups + SFTP export — good — but it is a single-operator free service; if *it* shuts down you are migrating on its timeline, not yours.

**Net:** the VPS+SQLite path is the only one where the maintainer is never more than one file-copy away from full control of the data. For a project whose stated value is low operational anxiety, that is decisive.

---

## Scheduled-Job Support (daily PigParse / weekly wiki)

- **VPS (recommended):** trivial. Either system `cron` invoking the binary with a flag, or — cleaner — an in-process `time` ticker / a small scheduler library inside the long-running Go server. **No 6-minute execution cap** (the Apps Script constraint that forced the re-entrant cursor design in `refresh_wiki.ts` simply disappears — the weekly scrape can run start-to-finish, politely, for as long as it needs). This is a genuine *simplification* over today's architecture.
- **Cloudflare Workers:** native cron triggers, free, 3 per worker — clean fit, but the long sequential wiki scrape fights the CPU/subrequest model.
- **Supabase:** `pg_cron` for DB-side jobs or scheduled Edge Functions for the fetches — works, but splits enrichment into a proprietary runtime.
- **Render/Railway/Fly:** all support cron jobs / scheduled tasks; fine, but you are paying PaaS prices for it.

The VPS gives the most freedom here and removes an existing constraint — a point in its favor beyond cost.

---

## Operational Burden — what the maintainer babysits

**VPS (recommended) — low-medium:**

- *One-time:* provision the VPS, point a domain (or DuckDNS) at it, install the Go binary + Caddy (auto-HTTPS, ~10 lines), set up a systemd unit so the server restarts on reboot/crash, set up the nightly backup cron. A weekend of work, well within the maintainer's skill set.
- *Ongoing:* OS security updates (`unattended-upgrades` makes this near-zero), occasionally eyeball that the backup ran and the daily job succeeded (a healthcheck ping to a free service like healthchecks.io closes that loop hands-off). Realistically **a few minutes a month.**
- *The honest cost:* the maintainer now owns a Linux box. That is *more* than "own nothing" (today's Apps Script answer). But it is a box running one Go binary the maintainer wrote — not a fleet, not Kubernetes. This is the most operationally-heavy part of the whole milestone and it is still genuinely light.

**Supabase — low, until it isn't:** nothing to patch, managed backups — but you babysit the *pause* (a keep-alive cron, which needs a host), watch the 500 MB ceiling, and absorb dashboard/API changes the vendor ships. Lower steady-state burden, higher "platform changed under me" surprise risk.

**Cloudflare Workers — low:** no servers to patch; burden is staying within limits and the mental cost of a TS rewrite.

A blunt point for the go/no-go: **the current Apps Script architecture has *zero* server operational burden.** Every option in this milestone *adds* operational burden — that is intrinsic to "run our own backend." The VPS adds the most but in exchange gives the cleanest escape from Google OAuth and full data ownership. The question for the maintainer is whether escaping brand-verification pain is worth ~$55/yr and a few minutes/month of babysitting. If the OAuth blockage keeps recurring, it almost certainly is.

---

## Recommendation

**Primary: a single-binary Go server on a fixed-price Hetzner CX22 VPS, with an embedded SQLite database, Caddy for automatic HTTPS, and either system `cron` or an in-process Go scheduler for the daily PigParse / weekly wiki enrichment. ~$55/year (~$67–70 with a vanity domain).**

Why this and not the $0 options:

1. **It actually completes the milestone's mission.** The milestone exists to *drop Google OAuth*. Custom-server paths replace watcher auth with a simple bearer token — no consent screen, no brand verification, ever again. Supabase/Firebase only partially escape (they still need a host for cron + token issuance).
2. **It reuses the maintainer's strongest skill.** Go server, Go enrichment clients, single static binary — the same shape as the watcher the maintainer already maintains. No new proprietary runtime to learn (Deno Edge Functions, Workers/WASM).
3. **No hidden failure modes.** No 7-day pause, no 30-day DB expiry, no spin-down latency, no metered bill. A flat ~$4.59/mo is the *entire* cost story. For a single part-time maintainer, predictability beats $0-with-asterisks.
4. **Data ownership and durability.** The database is one file; a nightly off-box `rsync` means the maintainer is always one copy away from full control. If payment ever lapses, the data is in hand — not behind a vendor login. This is *better* durability than the current Google Sheet.
5. **It removes an existing constraint.** No Apps Script 6-minute cap — the weekly wiki scrape stops needing its re-entrant cursor hack.

The cost is real but small: ~$55/yr against $0 today, plus a one-weekend setup and a few minutes/month of babysitting. Given that OAuth brand verification has *repeatedly* blocked the watcher, that trade is sound.

**Implementation note:** before hand-rolling the Go server, spend an afternoon evaluating **PocketBase** (open-source, single Go binary = SQLite + auth + REST API + admin UI + JS/Go hooks) *self-hosted on the same CX22 VPS*. It is architecturally almost exactly the recommended design, pre-built, and would cut the backend to mostly configuration + the two enrichment jobs as hooks. Self-hosted (not PocketHost) it keeps the cost, durability, and ownership story identical to the recommendation while saving build time. If PocketBase's auth/extension model fits the watcher-token and enrichment needs, prefer it; if it chafes, the hand-rolled Go server is the fallback. Either way the host (CX22 VPS) and database (SQLite) recommendations stand.

**Runner-up: Supabase free tier ($0/mo).** Pick this only if the ~$55/yr is a genuine blocker. Accept in exchange: the 7-day inactivity pause (needs a keep-alive cron, which needs a host anyway), a 500 MB ceiling, real vendor lock-in, and the fact that you *still* must host a server process for the watcher-token endpoint and enrichment cron — so it does not actually deliver the "we host one thing" simplicity its $0 price implies. It is the runner-up because the price is right and the Postgres is real, not because the architecture is cleaner.

**Not recommended:** Firebase (document store mis-fits the relational schema; metered billing), Render free tier (Postgres self-deletes at 30 days; paid tier ~$168/yr is pricier than the VPS for less control), Oracle Always Free (capacity-unavailability and reclaim-sweep anxiety undermine the whole "low operational burden" goal).

---

## Sources

- [Supabase Pricing](https://supabase.com/pricing) — free $0 / Pro $25; Pro does not pause; free 500 MB DB / 1 GB storage
- [Supabase Free Tier Limits 2026 (AI Agency Plus)](https://aiagencyplus.com/supabase-free-tier-limits/) — 7-day inactivity pause, ~30 s resume
- [Prevent Supabase Free Tier Pausing (Medium)](https://shadhujan.medium.com/how-to-keep-supabase-free-tier-projects-active-d60fd4a17263) — keep-alive workaround
- [Fly.io Resource Pricing](https://fly.io/docs/about/pricing/) — shared-cpu-1x 256 MB ~$2/mo; volumes $0.15/GB; no permanent free tier
- [Fly.io Free Tier 2026 (SaaS Price Pulse)](https://www.saaspricepulse.com/tools/flyio) — pay-as-you-go since Oct 2024
- [Render Deploy for Free docs](https://render.com/docs/free) — web service spin-down after 15 min idle
- [Render changelog: free Postgres expires after 30 days](https://render.com/changelog/free-postgresql-instances-now-expire-after-30-days-previously-90) — 30-day expiry + 14-day grace then deletion
- [Render Pricing](https://render.com/pricing) — paid service/DB tiers
- [Hetzner new CX plans](https://www.hetzner.com/pressroom/new-cx-plans/) / [Hetzner Cloud Pricing 2026 (bestusavps)](https://bestusavps.com/reviews/hetzner/) — CX22 €3.79 / ~$4.59 per month, 2 vCPU / 4 GB / 40 GB / 20 TB
- [Cloudflare Workers Limits](https://developers.cloudflare.com/workers/platform/limits/) / [Pricing](https://developers.cloudflare.com/workers/platform/pricing/) — 100 K req/day free; CPU budget
- [Cloudflare Workers Cron Triggers](https://developers.cloudflare.com/workers/configuration/cron-triggers/) — free, 3 per worker
- [Cloudflare D1 Pricing](https://developers.cloudflare.com/d1/platform/pricing/) — free 5 GB / 5 M row-reads/day / 100 K row-writes/day
- [Oracle Cloud Free Tier](https://www.oracle.com/cloud/free/) / [Always Free Resources docs](https://docs.oracle.com/en-us/iaas/Content/FreeTier/freetier_topic-Always_Free_Resources.htm) — 4 OCPU / 24 GB ARM always-free
- [Railway Pricing Plans](https://docs.railway.com/pricing/plans) / [Railway Free Trial](https://docs.railway.com/pricing/free-trial) — $5/mo Hobby incl. $5 credit; $1/mo free credit after trial
- [PocketHost pricing](https://pockethost.io/pricing) / [PocketBase FAQ](https://pocketbase.io/faq/) — PocketBase is single-binary Go+SQLite; PocketHost free hosted tier
