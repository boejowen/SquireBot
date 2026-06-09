# Phase 21 — PigParse `getdetails` Feasibility Spike

> ## ⚠ CORRECTION 2026-06-09 — the "server=0 = LIVE Blue" finding below is WRONG
> The spike's headline finding ("§ CRITICAL FINDING — the live Blue tunnel is server=0, NOT server=1") is **disproven.** It only ever proved server=0 was *fresher* (more parsers) — never which P99 server it was. **server=0 is GREEN; server=1 is Blue** (consistent with the catalog convention `getall/1`=Blue). A guildie who plays **Blue** got an EC DM whose listed seller was auctioning on **Green** (2026-06-09); a live probe confirmed server=1 is live Blue (e.g. Cloak of Flames server=1 `max_t` 2026-06-08T23:02, *fresher* than server=0 at that moment). Each getdetails record carries **no per-auction server field**, so the URL server segment is the only lever. **Fixed:** `ec/urls.go` now polls `getdetails/1/` (Blue); the live `ec_auction_cursor` was cleared to re-baseline against Blue (quick `260609-er6`, commit `de15933`, deployed 2026-06-09). Blue coverage is sparser than Green (fewer parsers) — correct-but-intermittent, per the D-07 caveat below. **Treat the "server=0=Blue" claim in §"CRITICAL FINDING" and the hand-off as historical/incorrect.**

**Run:** 2026-06-06 ~02:11–02:13 UTC (live, read-only HTTPS GETs against `https://pigparse.azurewebsites.net`)
**Gate:** ROADMAP Phase 21 criterion 1 / Pitfall 1. D-08 = no checkpoint — threshold applied, path chosen, phase proceeds.
**Verdict:** ✅ **Adopt per-auction `getdetails` (the default path).** Both threshold conditions met.

## Path chosen

**PATH CHOSEN: per-auction `getdetails` poll-and-diff** (cursor on `ItemAuctionDetail.t`).
The coarser `lastWTSSeen` (`getmultiple`) fallback is NOT needed and is NOT adopted.

**Key form: the NAME form, `GET /api/item/getdetails/{server}/{itemname}`** — NOT the id form.

## D-08 threshold rule — both conditions satisfied

| Condition | Required | Observed | Met? |
|-----------|----------|----------|------|
| (a) Presence | ≥1 probed item returns ≥1 `ItemAuctionDetail` with parseable `t` AND `u ∈ {0,2}` | Fungi: 25,727 WTS records; FBSS: 10,136 WTS records — all with parseable `t` | ✅ |
| (b) Advancement | `max(t)` advancing / feed is live, not a frozen snapshot | server=0 `max(t)` was **~3 minutes old at wall-clock poll time** (02:09:39 t vs 02:12 poll) — the feed is demonstrably current and advancing | ✅ |

Advancement note: a 60-second re-poll did NOT bump `max(t)` (expected — PigParse rebuilds aggregates ~every 10 min and not every item gets a new auction every minute). Liveness is proven instead by the **freshness of `max(t)` relative to wall-clock**: the newest record on the live server (server=0) trailed real time by only ~3 minutes. This is strong API-shape + freshness evidence; a ≥10-min same-window double-poll showing a literal bump was not captured, so per D-07 we ship on the freshness evidence and never defer.

## Items probed (live)

| Item | Endpoint | HTTP | Records | WTS (u∈{0,2}) | non-null `p` | `players` | `t` min → max |
|------|----------|------|---------|---------------|--------------|-----------|---------------|
| Fungus Covered Scale Tunic | `getdetails/1/{name}` | 200 | 25,998 | 25,727 | 12,654 | 688 | 2023-06-01 → 2026-05-30 |
| Flowing Black Silk Sash | `getdetails/1/{name}` | 200 | 10,438 | 10,136 | 3,608 | 438 | 2023-06-02 → 2026-06-05 15:33 |
| Rubicite Helm | `getdetails/1/{name}` | 200 | 6 | 6 | 0 | 2 | 2023-11-12 → 2025-01-05 |
| Flowing Black Silk Sash | `getdetails/**0**/{name}` | 200 | 23,875 | — | — | 1,082 | → **2026-06-06 02:09:39** |
| Fungus Covered Scale Tunic | `getdetails/**0**/{name}` | 200 | 35,986 | — | — | 1,348 | → **2026-06-06 01:54:58** |

## ⚠ CRITICAL FINDING — the live Blue tunnel is **server=0**, NOT server=1

The 21-RESEARCH / STACK-v2.2 assumption that "`{server}=1`=Blue" is **WRONG for the `getdetails` per-auction feed.**

- `getdetails/**1**/{name}` returns a stale dataset — FBSS `max(t)` froze at **2026-06-05T15:33** (≈11 h before the spike).
- `getdetails/**0**/{name}` returns the LIVE feed — FBSS `max(t)` = **2026-06-06T02:09:39**, ~3 min before poll, with ~2.3× more records (23,875 vs 10,438) and ~2.5× more resolvable sellers (1,082 vs 438).

**Plan 03 MUST poll `getdetails/0/{itemname}`** (server segment = `0`) to get current auctions. Polling server=1 would silently miss every live event. This corrects the research's `server=1` recommendation — record it loudly; it is the single most important spike output.

## Key form decision — NAME, not ID (corrects RESEARCH Open Q2)

RESEARCH Open Q2 recommended the **id form** (`getdetails/{itemid}`) to avoid name-drift. The spike disproves it:

- `getdetails/{itemid}` (bare, no server) → **HTTP 400** (e.g. `/getdetails/10561` → 400).
- `getdetails/1/{itemid}` (id in the name slot) → **HTTP 200 but `{"items":[],"itemName":null,"players":{}}`** — the endpoint treats the segment as a *name* and finds nothing. There is **no working id-keyed lookup**.

So the only functional query key is the **item NAME** via `getdetails/{server}/{itemname}`. The `i` field *inside* each `ItemAuctionDetail` is an auction-instance/listing id (it varies record-to-record for the same item — e.g. 16247, 81424, 80278 all under "Fungus Covered Scale Tunic"), **NOT** the EQ catalog item id and **NOT** a query key. Plan 03 keys the poll on `wantlist_item.item_name` (the snapshot name) and keeps `wantlist_item.item_id` as the `wantmatch.ForItem` join key after a hit. Name-drift risk is accepted (D-07 best-effort posture); if a wanted item's name drifts, that one item silently mis-polls — a coverage gap, not a correctness break.

## Seller resolution (D-05 best-effort) — aggregate hit-rate only (V7, no PII committed)

The `players` map is populated and sizeable on live items (server=0: 1,082 keys for FBSS, 1,348 for Fungi). It is a `{stringKey → playerName}` map whose key→specific-auction-record relationship is **undocumented** — `ItemAuctionDetail` carries no seller field, so a per-auction seller is not deterministically resolvable from the record alone. Treat seller as genuinely best-effort: attempt a `players`-map lookup, omit the embed field silently when unresolvable (D-05). **No raw player names are committed** — the captured fixture's `players` values are anonymized to `Seller1/2/3` placeholders.

## Price `p` nullability (confirmed live)

`p` is genuinely nullable in real data: of 25,998 Fungi records only 12,654 carried a non-null `p`; of 10,438 FBSS records only 3,608 did. The parser MUST type `p` as `*int` (nil-distinct from 0) and downstream MUST render "price unknown" / omit, never `0pp` (D-05, threat T-21-04).

## Timestamp shape (note for the parser/cursor)

`t` is an ISO-8601 / RFC3339-ish string with a `+00:00` offset and **variable fractional-second precision** (e.g. `2026-06-05T15:33:29.35+00:00` — 2 fractional digits — vs `...29.279+00:00` — 3 digits). Because the offset is always `+00:00` and the format is fixed-shape, **lexical string comparison is a sound monotonic cursor** (the plan's `last_seen_t TEXT` + `t > cursor` string compare is correct). The parser keeps `t` as the raw string; it does NOT need to `time.Parse` it for the diff (parse only for the "~3 min ago" embed rendering in Plan 03, tolerating the variable precision via `time.RFC3339`-family layouts).

## D-07 coverage caveat

**EC alerts are best-effort and bursty.** The EC tunnel is only fed when a human is parked in EC running a PigParse parser — coverage is real but intermittent and weekend-heavy. The monitor ships anyway and documents this honestly ("EC alerts depend on someone parsing the tunnel — best-effort, not a guarantee"). The spike confirms the data is live and rich *right now*; it does not and cannot guarantee a given future auction is captured. Do NOT promise instant/real-time delivery — the ~10-min PigParse rebuild + the ~10-min poll cadence make "near-real-time, within ~10–20 min" the honest design point.

## Captured fixture

`internal/backendsrv/enrich/__fixtures__/pigparse-getdetails-fungi.json` — the 12 most-recent real `ItemAuctionDetail` records from the **live server=0** Fungi feed (mix of null and non-null `p`, all `u=0` WTS), with `players` trimmed to 3 anonymized placeholder entries (V7 — no real player names committed). Used as the Task-2 parser happy-path fixture.

## Hand-off to Plan 03 (the producer job)

1. **Poll `getdetails/0/{itemname}`** — server segment `0` (the live Blue feed); server `1` is stale. URL-escape the name with `enrich.EncodeURIComponent`, hardcoded host (SSRF, T-21-01).
2. **Key the poll on `item_name`** (the only working lookup key); keep `item_id` for the `wantmatch.ForItem` join after a hit.
3. **Diff on `t`** via lexical string compare against `ec_auction_cursor.last_seen_t`; WTS = `u ∈ {0,2}` (D-02); WTB-only `u=1` never alerts.
4. **First-sight baseline:** when `GetECCursor` returns `ok=false`, set the cursor to `max(t)` of the first poll WITHOUT DMing (anti-replay).
5. **Seller best-effort** via `players`; **price nullable** (`*int`, omit when nil); **near-real-time** framing only.
