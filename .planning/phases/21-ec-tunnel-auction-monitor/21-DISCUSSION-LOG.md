# Phase 21: EC-Tunnel Auction Monitor — Discussion Log

**Date:** 2026-06-05
**Mode:** discuss (default, interactive)

> Human-reference record of the discuss-phase session. NOT consumed by downstream agents — they read `21-CONTEXT.md`.

## Areas selected

User selected all four offered gray areas: **Match scope, DM content, Spike outcome, Cooldown & re-list**.

## Area 1 — Match scope

- **Want reasons** — Options: (a) Buy + quest (any catalog want) [rec], (b) Buy-reason only. → **Chose (a)**: any catalog want with a real `item_id` fires regardless of buy/quest. (D-01)
- **WTS/WTB direction** — Options: (a) WTS only [rec], (b) WTS + WTB. → **Chose (a)**: WTS-only; WTB rejected as non-actionable noise. (D-02)
- **Custom wants** — Options: (a) Silently skip, defer to P22 [rec], (b) Skip + flag in UI. → **Chose (a)**: NULL-`item_id` wants silently skipped (can't exact-ID-match); no UI warning. (D-03)

## Area 2 — DM content

- **Fields (multi)** — Offered: seller (best-effort), why-you-wanted-it, item link, auction time. → **Chose ALL FOUR** on top of the essentials (item + price + WTS tag). (D-05)
- **Format** — Options: (a) Rich embed [rec], (b) Plain text line. → **Chose (a)**: discordgo rich embed. (D-04)
- **Link target** (follow-up) — Options: (a) squirebot.quest item view [rec], (b) P1999 wiki, (c) decide-at-build. → **Chose (a)**: deep-link to the project's own item view. Flagged: planner must verify a stable per-item frontend URL exists. (D-06)

## Area 3 — Spike outcome & risk posture

- **Thin coverage** — Options: (a) Ship anyway, document the gap [rec], (b) Ship `lastWTSSeen` fallback, (c) Defer the phase. → **Chose (a)**: best-effort framing; don't defer on thin coverage. (D-07)
- **Go/no-go** — Options: (a) Delegate to Claude on a threshold [rec], (b) Checkpoint me on the result. → **Chose (a)**: Claude runs the spike, applies a stated `getdetails`-vs-`lastWTSSeen` coverage rule, proceeds without stopping; documents findings. (D-08)
- **Poll courtesy** — Options: (a) No contact, stay polite in code [rec], (b) Courtesy-contact first. → **Chose (a)**: consistent with waived ENRICH-09; politeness via backoff/cadence/wanted-items-only. (D-09)

## Area 4 — Cooldown & re-list

- **Cooldown window** — Options: (a) ~24h [rec], (b) ~6–12h, (c) You decide (tunable constant). → **Chose (c)**: Claude picks a sane placeholder (roughly daily) as a per-source tunable constant, soak-adjusted. (D-10)
- **Re-list behavior** — Options: (a) Re-alert (time-based only) [rec], (b) Price-drop breaks cooldown. → **Chose (a)**: new auction past the window re-alerts; cooldown purely time-based, price-threshold logic stays deferred. (D-11)

## Wrap-up

User chose **"Ready for context"** — no additional gray areas. CONTEXT.md written.

## Deferred ideas captured

Price-threshold alerts (REQUIREMENTS.md:47), custom-want matching (→P22 `ForName`), `lastWTSSeen` permanent fallback (only if spike forces it), WTB-direction alerts (rejected), retry/digest/quiet-hours (P20-deferred), P22/P23 monitors.

## Claude's discretion items

Cooldown value, spike coverage threshold numbers, embed layout/wording, seller-resolution effort, `ec_auction_cursor` shape + exact PigParse endpoint (pinned by spike), `/healthz` EC job state.
