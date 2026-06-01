# Decommission checklist — v2.0 "Off Google" (CUTOVER-04)

This is the proof artifact for retiring the live Google machinery once the guild
has been flipped onto the self-hosted backend (Phase 16). It is deliberately
**light** (the style follows `docs/eviction-runbook.md`): the guild has been dark
on the Sheet since **2026-05-15** (Google's brand-verification block), the backend
has been the only place data can go since **2026-05-29**, and P13/P14/P15 are
already live at squirebot.quest. So the classic "shadow soak → reconcile → flip"
dance is void — this checklist just kills the two *live* Google assets, records
that the Sheet is abandoned in place, and references the code-level proofs.

**Drafted:** 2026-05-31. **Completed:** 2026-05-31.

## What "done" means (D-13)

CUTOVER-04 is satisfied as **"no *live* Google machinery / no Google dependency
remains."** The system's *code-level* Google-freedom was already proven in P13
(see §4). This document is the operational record that the two remaining *live*
Google assets — the Apps Script triggers and the OAuth client — have been retired.

## 1. Pre-decommission gate — guildies reporting in (CUTOVER-01, D-05)

The original 1–2 week shadow soak is **void**: the Sheet has been frozen since
2026-05-15, so there is nothing to soak against. CUTOVER-01 collapses to a single
maintainer confirmation that onboarded guildies are reporting in on the backend
and their views look right. **No separate soak document.**

- [x] Confirmed guildies are reporting in on the backend: **3 of 11 guildies** (5+ characters; 721+ inventory items + 179+ spellbook rows at confirmation, still climbing), verified **2026-05-31** via `SELECT COUNT(DISTINCT owner_id) FROM character WHERE is_removed=0;` on the VPS. The maintainer accepts this as a representative sample (D-05 — no hard %); the remaining 8 onboard whenever and decommission strands no one (the upgrade path is the GitHub installer + their guild code, not Google). End-to-end validated across multiple real guildies — code→owner binding correct, real inventory + spellbook ingested, zero errors.

## 2. Retire the live Google machinery (CUTOVER-04 / D-10, D-11)

### 2a. Disable the Apps Script triggers (D-10)

These fire on **Google's** time-driven schedulers using the script owner's
authorization — **independent** of the OAuth-blocked watchers (Pitfall 3:
"watchers can't write to the Sheet" is NOT "the script's triggers stopped").
Left running, they keep double-loading the volunteer-run wiki + PigParse APIs the
backend now also serves — so deleting them is both teardown **and** politeFetch
hygiene.

> ⚠️ **Count correction:** the header comment in `installTriggers.ts` says "7
> triggers," but Phase 5 added 3 more and the code actually installs **10** (it
> logs `created: 10` and the install alert says "10 total"). Delete **all 10** or
> three enrichment/maintenance triggers keep firing on Google's infra.

- [x] Deleted **all 10** SquireBot triggers. Path: `script.google.com/home/triggers` (or the container-bound project editor → Triggers panel). Delete each:
  1. `onChange` — sheet change (debounced view rebuild)
  2. `buildView` — hourly backstop
  3. `refreshPigparse` — daily 03:00 PT
  4. `refreshWikiItems` — Sunday 04:00 PT
  5. `refreshWikiSpells` — Sunday 04:00 PT
  6. `refreshWikiGearTier` — Sunday 05:00 PT
  7. `monitorCellCount` — Sunday 03:00 PT (10M cell-cap watchdog)
  8. `weeklySchemaHealthcheck` — Sunday 03:00 PT
  9. `weeklyStaleCharArchive` — Sunday 06:00 PT
  10. `weeklyEvictionArchive` — Sunday 06:00 PT
  **Verify:** the Apps Script Executions dashboard shows no further scheduled `refreshPigparse`/`refreshWikiItems`/… runs after the deletion, and the Triggers dashboard shows **zero** triggers for the project. Action taken + date: **all 10 deleted by the maintainer, 2026-05-31** (zero triggers remain; no further scheduled enrichment runs).

### 2b. Retire the Google OAuth 2.0 Client ID (D-11)

The asset CUTOVER-04 explicitly names. The watcher OAuth **code** was already
deleted in P13, so the client has **no consumer** — deletion is safe and breaks
nothing.

- [x] Deleted (or disabled) the v1.0.2 desktop OAuth 2.0 Client ID. Path: Google Cloud Console → APIs & Services → Credentials/Clients → select the v1.0.2 desktop client → **DELETE** (cite: support.google.com/cloud/answer/15549257).
  **Note:** deletion **immediately revokes** any outstanding tokens; the client is restorable for 30 days, then permanently gone. The 30-day window is benign — no client flow exists to use a token in it. Action taken (deleted / disabled) + date: **deleted by the maintainer, 2026-05-31** (outstanding tokens revoked; 30-day restore window noted, no consumer exists in it).

## 3. Abandon the Sheet in place (D-12) — a deliberate non-action

The fresh-start decision (D-01 — nothing is read from the old Sheet) already made
the Sheet worthless to the system. Deleting or exporting it is busywork.

- [x] Google Sheet: **abandoned in place** — NO export, NO delete, NO read-only freeze (D-12). Pre-checked: this is an intentional no-op, not a pending action.

## 4. Code-level Google-freedom proofs (already established in P13, D-13)

These were proven when the watcher was re-targeted in Phase 13 — referenced here,
not re-run.

- [x] `go list -deps ./cmd/squirebot` is Google-free (P13 / 13-03) — no `google.golang.org/api`, no `golang.org/x/oauth2` in the watcher's dependency closure.
- [x] Watcher binary **57% smaller** (16.44 MB → 7.07 MB) + zero Google OAuth/Sheets/secret strings on a 9-pattern byte scan (P13 / 13-04).
- [x] Backend handles **no** Google secret; no Sheets/Drive client anywhere in the codebase (P13).

## 5. Milestone close

With the reporting-in confirmation (§1) + both live Google assets retired (§2) +
the Sheet abandoned in place (§3) + the code-level proofs (§4), **CUTOVER-01 and
CUTOVER-04 are satisfied** and the v2.0 **"Off Google"** goal is met. No live
Google machinery and no Google dependency remain.

**Completed 2026-05-31.** Post-teardown sweep confirmed the live system is unaffected by the retirement: the backend service is `active`, `api.squirebot.quest` routes return `401` (live + session-gated), `squirebot.quest` serves `200`, and the guild is still ingesting (3 guildies / 7 characters / 909 inventory rows and climbing). The Google teardown touched only Google-side assets the system no longer depends on. ✅ **v2.0 "Off Google" — DONE.**
