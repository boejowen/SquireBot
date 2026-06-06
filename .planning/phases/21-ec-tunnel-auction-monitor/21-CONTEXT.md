# Phase 21: EC-Tunnel Auction Monitor - Context

**Gathered:** 2026-06-05
**Status:** Ready for planning

<domain>
## Phase Boundary

The **first real end-to-end alert**. A new `scheduler.ec_auction_match` job polls PigParse per wanted item (~10-min cadence), diffs on an auction-timestamp cursor (`ec_auction_cursor`), exact-item-ID matches against guildie wantlists, and DMs the wantlister (price + WTS tag; seller best-effort) — riding the Phase 20 `wantmatch` + `notify` + `alert_log` spine, all on the guild's own Discord. Delivers **WANT-05**.

**Gated by a MANDATORY upfront PigParse feasibility spike** (the phase's first task, ROADMAP criterion 1): confirm auction timestamps advance during a live tunnel + measure coverage, which decides whether the trigger is per-auction (`getdetails`) or coarser new-sighting (`lastWTSSeen`) **before the plan commits**.

**Explicitly NOT in this phase** (redirect, don't fold): WTS cross-server reading (P22), quest-target raid detection (P23), `MESSAGE_CONTENT` intent, the quest→NPC table, price-threshold filtering (deferred — REQUIREMENTS.md:47), name/alias fuzzy matching (that's `wantmatch.ForName`, a P22 concern), custom (NULL-`item_id`) want matching.
</domain>

<decisions>
## Implementation Decisions

### Match scope — which wants/auctions fire (WANT-05)
- **D-01 (Want reasons):** EC fires for **any catalog want regardless of reason** — both `buy` AND `quest` (`reason IN ('buy','quest')`), as long as the row has a real `item_id`. Rationale: a quest item showing up for sale in the tunnel is often cheaper/faster than questing it, so the wantlister wants to know either way. The buy-vs-quest split does NOT gate the EC monitor.
- **D-02 (Auction direction — WTS only):** Alert **only on WTS** (item being sold). WTB sightings **never** alert (research explicitly rejects WTB-matching for buy-wants as noise — you can't buy from someone who's also trying to buy). The DM still carries a direction tag for clarity, but in practice it always reads WTS.
- **D-03 (Custom wants skipped):** Custom wants (`item_id IS NULL`) **cannot** be exact-ID-matched against PigParse auctions, so the EC job **silently skips** them — no user-facing warning. They are the WTS name-matcher's job (`wantmatch.ForName`, P22). EC only ever matches catalog wants (`item_id` present).

### DM content & format (WANT-05; P20 deferred per-monitor format to here)
- **D-04 (Format = rich embed):** The EC alert is a **discordgo rich embed**, not a plain string — item as a titled card with price / WTS tag / seller / link / time as fields. (More polished + scannable than a string; slightly more code but worth it for the flagship alert.)
- **D-05 (Fields):** Essentials = **item name + price + WTS tag**. Plus, when available: **seller name** (best-effort — `ItemAuctionDetail` has no seller field; resolve only via the `players` map, omit silently when unresolvable), **why-you-wanted-it** (echo the want's reason and/or saved `note` so a ping months later still makes sense), **a clickable item link**, and **auction time** (how fresh the listing is, e.g. "~3 min ago").
- **D-06 (Link target = P1999 wiki — RESOLVED 2026-06-05):** Originally chose the project's own item view, BUT research (21-RESEARCH.md) confirmed **no per-item route exists** on the SvelteKit frontend (`web/src/routes/` has no `item`/`[id]` route), and the existing idiom is already a wiki-link helper (`wikiUrlFor`). User decision on the surfaced fork: **link to the P1999 wiki via `wikiUrlFor`**, keeping Phase 21 **backend-only** (zero frontend scope). A native `/item/[id]` deep-link is deferred to its own small web task. The embed still carries item/price/WTS/seller/reason/time; the link points to the wiki.

### Spike outcome & risk posture (WANT-05 criterion 1)
- **D-07 (Thin coverage → ship anyway, document the gap):** The EC tunnel is only fed when a human is parked in EC running PigParse (weekend-bursty). If the spike confirms timestamps advance but coverage is thin/intermittent, **ship the monitor anyway and document the limitation** ("EC alerts depend on someone parsing the tunnel"). Coverage is bonus, not a guarantee — best-effort framing matches the data source's nature. Do NOT defer the phase on thin coverage.
- **D-08 (Go/no-go delegated to a threshold — no checkpoint):** Claude runs the spike, applies a **stated coverage rule** (default: use per-auction `getdetails` if per-auction timestamps are present and advancing; else fall back to coarser `lastWTSSeen` new-sighting), and **proceeds to plan without stopping** (matches `yolo` mode). The spike findings + which path was taken MUST be documented in the plan/spike artifact. No manual user gate on the result.
- **D-09 (No courtesy-contact — stay polite in code):** Do **not** human-contact the PigParse operator about the new ~10-min poll cadence (consistent with the previously-waived ENRICH-09). Stay courteous in code instead: conditional requests / backoff, a sane cadence, and **only poll for items that are actually on someone's wantlist** (never a blanket sweep).

### Cooldown & re-list behavior (rides P20's dedup/cooldown policy)
- **D-10 (Cooldown = tunable per-source constant):** EC cooldown is a **per-source tunable constant** — Claude picks a sane placeholder (research suggests a roughly daily window, ~20–24h) and exposes it as a constant adjustable in soak. The exact number is Claude's discretion; the mechanism (suppress repeat DMs for the same `(wantlist_item, source, item_id)` within the window) is locked.
- **D-11 (Re-list re-alerts, time-based only):** A genuinely NEW later auction of the same item **past the cooldown window re-DMs** the wantlister — it's a fresh buying opportunity. Cooldown is **purely time-based**; price is NOT part of the dedup key and a cheaper re-list does NOT break the cooldown early (price-threshold/price-drop logic stays deferred — REQUIREMENTS.md:47).

### Locked upstream (v2.2 research + P20 — DO NOT re-decide; planner must honor)
- **Poll-and-diff on an auction-timestamp cursor** (`ec_auction_cursor`), **exact item-ID match** (`wantmatch.ForItem`), ~10-min cadence. **Advance the cursor only after a successful poll**; **never replay backlog on restart** (a standing auction is not re-DMed every poll — ROADMAP criteria 2 & 4).
- **In-process scheduler job** in the existing registry (rides alongside daily-PigParse / weekly-wiki); reuses `enrich`/`politefetch`, no new HTTP client, no cron daemon.
- **Dedup + cooldown** via `alert_log` (the P20 spine); every attempt recorded.
- Both gates from P20 still apply: the **officer monitor flag** (guild-wide EC enable) AND the **user opt-in/prefs** (per-monitor + per-want mute) must both allow before a DM fires.
- **No Discord bot/OAuth in the watcher** (carried HARD CONSTRAINT) — all EC work is backend + the guild's own Discord.

### Claude's Discretion
- The exact **cooldown value** (D-10) and the **spike coverage threshold** numbers (D-08).
- Exact **embed layout/wording**, the WTS tag copy, and the "why-you-wanted-it" phrasing (D-04/D-05).
- **Seller-resolution effort** — best-effort via the `players` map; how hard to try is Claude's call (omit silently when unresolvable).
- The **`ec_auction_cursor` table/column shape** and the precise PigParse endpoint(s) (`getdetails` / `getmultiple` / `lastWTSSeen`) — pinned by the spike result.
- Surfacing **EC job state on `/healthz`** and the job's structured-log fields (observability).
</decisions>

<specifics>
## Specific Ideas

- This is the milestone's flagship "it actually works" moment — the embed should feel like a real, useful trade alert: at a glance "**Fungi Tunic — WTS ~2000pp — seen ~3m ago**", expandable to seller/reason/link.
- "Best-effort" is the honest contract for both **seller** (often unresolvable) and **coverage** (tunnel only fed when someone's parsing) — say so plainly rather than over-promising.
- Only poll items that are genuinely wanted — the poll set is derived from live wantlist rows, never a blanket auction sweep (both politeness and efficiency at ~12 users).
</specifics>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Milestone scope & locked decisions
- `.planning/REQUIREMENTS.md` — **WANT-05** (the requirement this phase delivers; note line 47 — price-threshold alerts are DEFERRED) + v2.2 locked decisions.
- `.planning/ROADMAP.md` §"Phase 21: EC-Tunnel Auction Monitor" — goal + 4 success criteria (the acceptance contract; criterion 1 = mandatory spike).
- `.planning/PROJECT.md` — v2.2 milestone scope, delivery/reading split, the HARD CONSTRAINT (no bot/OAuth in the watcher).
- `CLAUDE.md` — item-ID join key, in-process scheduler, structured `slog` logging, PigParse API doctrine (`GET /api/item/getall/1` once daily — NOT scraping), schema-evolution (extend-only) rules.

### v2.2 research (authoritative technical guidance — research likely skippable, but READ for the EC specifics)
- `.planning/research/SUMMARY-v2.2.md` — §"Phase 21 — EC-tunnel auction monitor" + §"Critical Pitfalls" #1 (PigParse spike mandatory) & #5 (dedup/cooldown, advance-cursor-only-after-success, no backlog replay) + Gaps (seller resolution, coverage, cooldown interval).
- `.planning/research/ARCHITECTURE-v2.2.md` — `scheduler.ec_auction_match` design, `wantmatch.ForItem`, the one-match-seam pattern, `ec_auction_cursor`.
- `.planning/research/PITFALLS-v2.2.md` — Pitfall 1 (PigParse assumed-not-verified → spike first; `lastWTSSeen` fallback) + Pitfall 5 (alert spam / cursor / restart-replay).
- `.planning/research/STACK-v2.2.md` — PigParse endpoint inventory (`getdetails/{server}/{item}` → `ItemAuctionDetail[]` with `u/i/p/t`; `Item.lastWTSSeen`/`lastWTBSeen`; ~10-min rebuild; **no global auctions-since-T feed**).

### Pattern twins (closest existing analogs to copy)
- `internal/backendsrv/scheduler/scheduler.go` — the in-process job registry the EC poll job joins (the daily-PigParse / weekly-wiki precedent + recover-isolated pattern).
- `internal/backendsrv/enrich/jobs/pigparse.go` + `enrich`/`politefetch` — the PigParse fetch + parse + polite-fetch (backoff/conditional) machinery to reuse for the per-item poll.
- `internal/backendsrv/migrations/00006_wantlist.sql` — `wantlist_item` (the poll set: `item_id`, `reason`, `active`) + `alert_log` (the dedup target; dedup index `(wantlist_item_id, source, item_id, sent_at)`).
- `internal/backendsrv/migrations/00007_notify.sql` — the P20 spine schema (notify-prefs, monitor flags, `guild_channel`, `alert_log.read_at`, `wantlist_item.muted`) the EC job must honor (both gates).
- **P20 `notify` + `wantmatch` + `alert_log` packages** (Phase 20 deliverables) — the EC job calls `wantmatch.ForItem` then `notify`; do NOT re-implement DM send / dedup.
- `web/src/routes/` (item/tooltip view) — the deep-link target for D-06; verify a stable per-item URL exists.
- `.planning/phases/20-bot-dm-notification-infrastructure/20-CONTEXT.md` — the spine decisions (D-08 both-gates, cooldown mechanism, 50007 handling) the EC monitor rides.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`scheduler` in-process job registry** — the EC poll is one new job beside the existing daily-PigParse / weekly-wiki jobs; same recover-isolated pattern, no new daemon.
- **`enrich` / `politefetch`** — existing PigParse fetch + backoff/conditional-request machinery; the per-item poll reuses it (D-09 "polite in code").
- **P20 `notify` / `wantmatch` / `alert_log`** — the EC job is a thin producer: derive poll set from `wantlist_item` → poll PigParse → diff on cursor → `wantmatch.ForItem` → `notify` (which handles DM send, 50007, dedup/cooldown, `alert_log` write). The spine is already built and tested.
- **`wantlist_item`** — the poll set source (`active = 1`, `item_id NOT NULL`); `reason` does NOT filter (D-01); `muted` is consulted by the spine.

### Established Patterns
- In-process recover-isolated goroutine/job started in `runServe()` (the bot + scheduler precedent).
- Extend-only, idempotent `goose` migrations (`00001`→`00007`); the EC cursor is the next forward-only migration (e.g. `00008`).
- `item_id` is the stable join key — `wantmatch.ForItem` keys on it (custom NULL-item wants can't participate — D-03).
- Advance-cursor-only-after-success + no-backlog-replay (the dedup/restart safety).
- Structured `slog` logging (never log PII).

### Integration Points
- New `ec_auction_match` job registered in `scheduler` (~10-min ticker), reading `wantlist_item` for the poll set and writing through the P20 `notify` path.
- New `ec_auction_cursor` table/column (next migration) — the per-item auction-timestamp diff cursor.
- The EC monitor's officer enable flag + per-user prefs (P20 `monitor_flag` / notify-prefs) are the two gates a would-be DM passes.
- The DM embed links into the SvelteKit item view (D-06) — confirm/establish a stable per-item URL.
</code_context>

<deferred>
## Deferred Ideas

- **Price-threshold / "only DM if under X pp"** — explicitly deferred (REQUIREMENTS.md:47, "ships later if noise warrants"); D-11 keeps EC cooldown purely time-based, no price awareness.
- **Custom (free-text) want matching on EC** — D-03 skips them; they belong to `wantmatch.ForName` (P22 WTS name/alias matcher).
- **`lastWTSSeen` coarse fallback as a permanent mode** — only adopted if the spike shows per-auction `getdetails` coverage is too thin (D-08); not a default.
- **"Retry delivery" / digest / quiet-hours** — P20-deferred notification polish, unaffected here.
- **WTB-direction alerts** — rejected as noise (D-02).
- **WTS cross-server (P22) and quest-target raid (P23) monitors** — the other two sources that fan into the same spine; out of scope here.

### Reviewed Todos (not folded)
None — no pending todos matched this phase.
</deferred>

---

*Phase: 21-ec-tunnel-auction-monitor*
*Context gathered: 2026-06-05*
