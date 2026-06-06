# Phase 17: Self-Service Watcher Linking — Discussion Log

**Date:** 2026-06-01
**Mode:** discuss (default, interactive)

> Human-reference record of the discussion. NOT consumed by downstream agents (they read 17-CONTEXT.md).

## Areas offered
Identity linkage model · Eviction rewire scope · Code list & identifiability · Link page UX

**Selected:** Identity linkage model, Code list & identifiability, Link page UX
**Not selected:** Eviction rewire scope (surfaced minimally inside the linkage discussion, since the linkage decision forces a call on it — see Q1.3).

---

## Area 1 — Identity linkage model

**Q1.1 — Existing guildie's identity on first self-mint?**
Options: Adopt existing owner / Always new owner / Backfill then adopt
**→ Adopt existing owner** — find their existing owner row and stamp it with `discord_user_id`; data continuity preserved.

**Q1.2 — How to decide which existing owner is theirs / ambiguity handling?**
Options: Auto-match, new if none / Maintainer pre-links / Auto-match, refuse if none
**→ Auto-match, new if none** — `owner.label == Discord username` (trim/nocase); zero matches → fresh owner from Discord identity; 2+ matches or already-linked-to-another-Discord-id → refuse + log, never guess.

**Q1.3 — How much eviction owner-floor rewire belongs in P17? (the skipped area, surfaced)**
Options: Rewire floor to FK / Add link only, defer rewire / Seed floor by Discord ID
**→ Rewire floor to FK** — `callerMayNotEvictFloor` prefers `owner.discord_user_id`, falls back to the string bridge for unlinked owners. Closes WR-05 for linked guildies.

---

## Area 2 — Code list & identifiability (LINK-05)

**Q2.1 — How is each code identified in the list?**
Options: Optional label at mint / Auto-label only
**→ Auto-label only** — `#N` ordinal + created date (the device-naming UI is deferred per REQUIREMENTS).

**Q2.2 — Add per-code last-seen (stamp guild_code on each ingest) or defer?**
Options: Add per-code last-seen / Defer last-seen
**→ Add per-code last-seen** — new `guild_code.last_seen` column stamped on each authenticated ingest; surfaced as "last used X ago".

---

## Area 3 — "Link your watcher" page UX (LINK-04 / LINK-05)

**Q3.1 — Route + how it's reached?**
Options: /link — nav for all / /account — nav for all / /link — onboarding-linked
**→ /account — nav for all** — personal "your account / watcher codes" area, top-nav entry for every logged-in member (not officer-gated).

**Q3.2 — Page composition + revoke confirmation?**
Options: One page, confirm revoke / One page, instant revoke / You decide
**→ You decide** — single combined page is settled (mint → show-once panel → list/revoke); revoke-confirmation detail left to UI design (lean: confirm-before-commit per EvictionForm).

---

## Wrap-up
Final check → **"I'm ready for context."** CONTEXT.md written.

## Deferred ideas captured
Officer mint-on-behalf · device-naming UX polish · 999.5 self-service eviction · 999.12/WANT wantlist+pinger (pre-paid by the owner↔Discord FK).
