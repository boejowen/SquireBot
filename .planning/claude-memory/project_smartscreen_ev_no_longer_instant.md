---
name: EV cert no longer grants instant SmartScreen reputation (Mar 2024)
description: Microsoft removed EV's SmartScreen instant-reputation perk in March 2024 — EV and OV are now equivalent on the SmartScreen UX axis. Init-research and PITFALLS.md are wrong on this; do not buy EV.
type: project
originSessionId: dfdf0595-b2de-450e-a3e8-15ecb9220949
---
Microsoft removed EV code-signing certificates' SmartScreen instant-reputation grant in March 2024 (OIDs removed from the Trusted Root Program in August 2024). EV and OV certificates are now **equivalent** on the SmartScreen UX axis — both must accumulate reputation through downloads/installs over weeks.

**Why:** This inverts a load-bearing claim in two of the project's own docs:
- `.planning/research/STACK.md` recommends EV "for instant SmartScreen reputation"
- `.planning/research/PITFALLS.md` Pitfall #2 cites the same now-defunct mechanic
- The Phase 1 deferred SmartScreen UX TODO assumes EV would solve it

**How to apply:** When the code-signing question comes up (Phase 2 planning, Phase 5 README polish, any future cert renewal discussion), do NOT recommend EV (~$300-500/yr + hardware token). For SquireBot's 12-person guild distribution, the recommended path is **unsigned + documented SmartScreen walkthrough**, with a parallel **SignPath Foundation OSS** application (free, eligibility-gated, 1-4 week wait). Paid fallback is **Certum OSS** (€69 one-time + €30/yr smartcard). Source: SSL.com FAQ, Microsoft Q&A (March 2024), Sectigo KB — cross-verified in Phase 2 research 2026-05-01. Full reasoning in `.planning/phases/02-watcher-robustness-schema-lock/02-RESEARCH.md` §1.
