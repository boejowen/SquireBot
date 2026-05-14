---
name: User delegates gray-area decisions when given an objective criterion
description: When user has no preference on choices and supplies a tiebreaker rule, lock all decisions yourself — don't re-prompt
type: feedback
originSessionId: b842cd87-962e-4c52-b766-4aeb75967ce2
---
When asked to choose between implementation options (e.g., during `/gsd-discuss-phase`), if the user says they have no preference and supplies a tiebreaker criterion ("simplest end-user experience", "least invasive", "fastest to ship"), do NOT keep asking AskUserQuestion. Lock every decision against that criterion in one pass and write CONTEXT.md / proceed directly.

**Why:** During Phase 9 discuss (2026-05-12), I asked a 4-option multi-select AskUserQuestion for gray areas. User interrupted with: "I have no preference for any of the questions you asked me. For each: please make whichever decision will make the end-user experience the simplest and most invisible." Translation: stop asking, apply the rule, deliver the artifact.

**How to apply:**

1. Treat the user-supplied criterion as an explicit phase-wide doctrine — capture it in CONTEXT.md `<decisions>` so future readers know why each decision went the way it did.
2. For each gray area, write 1-2 sentences explaining which option the criterion picked and why the rejected options fail the criterion. Don't be terse — downstream agents (researcher, planner) need to understand the reasoning to not re-litigate.
3. Note any user discretion still needed (smoke evidence format, exact string copy, etc.) under "Claude's Discretion" — items that don't affect the user-facing outcome.
4. Do NOT call AskUserQuestion again for the same set of gray areas after the tiebreaker rule arrives. If a NEW gray area surfaces mid-flow that the rule doesn't cleanly answer, then ask.
5. This is effectively `--auto` mode with a custom selection rule (vs `--auto`'s default "recommended option"). Same shape, different scoring function.

**When NOT to apply:** if the user is asking exploratory questions ("what could we do about X?"), keep dialogue open. The doctrine activates when the user explicitly delegates with a stated criterion.
