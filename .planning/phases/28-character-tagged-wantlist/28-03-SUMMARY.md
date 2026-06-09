---
phase: 28-character-tagged-wantlist
plan: 03
subsystem: ui
tags: [svelte, sveltekit, wantlist, character-tag, tanstack-table, vitest]

# Dependency graph
requires:
  - phase: 28-character-tagged-wantlist (Plan 01)
    provides: schema v10, AddWantTx character_id, ListGuildWants/GuildWantRow store layer
  - phase: 28-character-tagged-wantlist (Plan 02)
    provides: POST /api/v1/wantlist optional character_id + IsCharAssignedToTx 403 gate, GET /api/v1/wantlist/guild (login-gated, note excluded)
  - phase: 26-character-assignment
    provides: fetchMyCharacters() / MyCharacter (the tag-select option source)
  - phase: 27 (my-characters view filter)
    provides: the pure DOM-free helper + node-test precedent (myview.ts) and the single-control filter-bar pattern
provides:
  - WantlistRow.character_id + character_name client fields
  - addWant body optional character_id (the UI tag write path)
  - GuildWantRow interface + fetchGuildWants() (the guildwide roll-up reader, NO note)
  - groupByChar() pure helper + node test (CWANT-06 own-list group/filter)
  - WantAddForm optional character <select> (CWANT-01 UI)
  - WantlistPanel My/Guild toggle + group-by-character control (CWANT-03/04/06 UI)
  - guildWantlistColumns (Owner/Character/Priority/Item/Reason, no Note)
affects: [28-VERIFICATION, gsd-ui-review 28, v2.3 wantlist UI]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Pure DOM-free filter helper + co-located node test (groupByChar mirrors myview.ts/priority.ts) — the node-testable surface; <select> rendering stays a browser-smoke gap"
    - "My/Guild client toggle over ONE DataGrid (consolidated-views LOCK) — a view switch, never a per-character tab/route"
    - "Guildwide grid renders only the GuildWantRow shape (no note) — the client cannot surface a field the API does not send (T-28-12)"

key-files:
  created:
    - web/src/lib/wantlist/groupByChar.ts
    - web/src/lib/wantlist/groupByChar.test.ts
  modified:
    - web/src/lib/api.ts
    - web/src/lib/columns.ts
    - web/src/lib/components/WantAddForm.svelte
    - web/src/lib/components/WantlistPanel.svelte

key-decisions:
  - "ACCOUNT_LEVEL is a unique Symbol sentinel (distinct from null=all) so the group-by-char control can filter to untagged wants without colliding with a real character_id"
  - "Guild view lazy-loads fetchGuildWants() on first toggle, then caches (server-truth, never optimistic-mutate); a 401 routes to AuthGate via the existing route() helper"
  - "Guild grid uses a dedicated guildWantlistColumns (no Remove/Mute — not the caller's rows; no Note — private/excluded server-side) instead of overloading wantlistColumns"
  - "Char <select> value bound as a STRING ('' = (no character)) coerced to number|null at submit; the server IsCharAssignedToTx is the real gate (the select is UX)"

patterns-established:
  - "groupByChar: null=passthrough(same ref), number=that character_id, ACCOUNT_LEVEL=untagged — pure, node-tested, never a security boundary (T-28-11)"
  - "Segmented My/Guild toggle + conditional group-by-char select sharing one filter-bar above a single grid (mirrors P27 +page.svelte)"

requirements-completed: [CWANT-01, CWANT-03, CWANT-04, CWANT-06]

# Metrics
duration: ~12min
completed: 2026-06-08
---

# Phase 28 Plan 03: Character-Tagged Wantlist (Web) Summary

**Surfaced the character-tagged wantlist in the web UI — an optional character `<select>` on add, a My/Guild toggle showing the guildwide roll-up with owner + character attribution (no note), and a per-user group/filter-by-character control — all client-side over the Plan-02 API shapes, names auto-escaped, one DataGrid.**

## Performance

- **Duration:** ~12 min
- **Started:** 2026-06-08 (this session)
- **Completed:** 2026-06-08
- **Tasks:** 3 / 3
- **Files modified:** 6 (2 created, 4 modified) — all under `web/src/`

## Accomplishments

- **Task 1 (api.ts):** `WantlistRow` gains `character_id` + `character_name`; `addWant` body accepts optional `character_id`; new `GuildWantRow` interface (owner + character_name, NO note) + `fetchGuildWants()` for `GET /api/v1/wantlist/guild`. Commit `e573730`.
- **Task 2 (groupByChar):** new pure DOM-free `web/src/lib/wantlist/groupByChar.ts` (mirrors `myview.ts` doctrine header — presentation NOT a security boundary, `import type` only) with the `ACCOUNT_LEVEL` Symbol sentinel; co-located `groupByChar.test.ts` with **5 node tests** (all / by-char / no-match / account-level / non-mutation), all green. Commit `2b987a9`.
- **Task 3 (components):** `WantAddForm` gains an optional character `<select>` sourced ONLY from `fetchMyCharacters()` with a "(no character)" default → null, threading `character_id` into the add body; `WantlistPanel` gains a My/Guild segmented toggle (Guild lazy-loads the roll-up) + a My-only group-by-character `<select>` driven by `groupByChar`; new `guildWantlistColumns` (Owner · Character · Priority · Item · Reason, no Note). All names render via plain auto-escaped braces, never raw-HTML. Commit `74b7031`.

## Gate Results

- `cd web && npm run check` → **0 errors / 0 warnings** (486 files)
- `cd web && npm test` → **303 passed (24 files)**, including the 5 new `groupByChar` tests
- `cd web && npm run build` → **built in ~30s**, site written (adapter-static)
- No `@html` in either edited component (grep clean)
- Guild grid surfaces an `owner` column + a `character` column (`character_name ?? ''`); no note column anywhere

## Deviations from Plan

None — plan executed exactly as written. Web-only scope honored: every code change is under `web/src/` (api.ts, columns.ts, WantAddForm.svelte, WantlistPanel.svelte, groupByChar.ts + test). No backend/migration change was needed (Waves 1–2 shipped the backend).

Note: `columns.ts` was extended (`guildWantlistColumns`) — it is the natural home for the consolidated-grid column factories alongside the existing `wantlistColumns`, so the new guild grid stays one DataGrid (consolidated-views LOCK) rather than a new component. This was implicit in Task 3's `<action>` ("extend the wantlistColumns shape with an owner column + a character column for the guild grid").

## Browser-Smoke Gap (OPEN — required before phase "verified")

node vitest is **DOM-blind** (no `@testing-library/svelte` — the P15/P26/P27 trap). The pure `groupByChar.ts` is node-tested, but the interactive `<select>` rendering/onchange in WantAddForm + WantlistPanel, the My/Guild toggle, and the guildwide display **CANNOT be proven by `npm test`**. Smoke on a **DEPLOYED build** (or full local stack — `npm run dev` bounces login against prod) per the plan's `<browser_smoke_gap>`, then run `/gsd-ui-review 28`. Checklist:

1. Add-want form shows a character `<select>` populated from my assigned characters + a "(no character)" default.
2. Adding a want with a character tag succeeds; adding with "(no character)" succeeds (account-level).
3. Tagging a character NOT mine is impossible from the UI (select lists only mine) AND the server rejects a forged body with **403** (negative test).
4. The /wantlist My/Guild toggle switches between own list and the all-members guildwide list.
5. The guildwide list shows owner (username) + character name per want; account-level wants show no character.
6. The guildwide list NEVER shows a note column.
7. Group/filter own wantlist by character via the `<select>` narrows to the chosen character (and "All characters" / "Account-level" options behave).
8. Character + owner names render escaped (no HTML injection) — never raw-HTML.

## Threat Surface

No new security-relevant surface beyond the plan's `<threat_model>`. T-28-10 (XSS) mitigated via plain `{}` auto-escape in both components (grep-verified no `@html`). T-28-11 (IDOR) — the char `<select>` is UX; the server `IsCharAssignedToTx` (Plan 02) is the gate (forged-body 403 is browser-smoke item 3). T-28-12 (info disclosure) — `GuildWantRow` carries no note; the client cannot render a field the API omits.

## Commits

- `e573730` — feat(28-03): api.ts char fields on WantlistRow + addWant body + GuildWantRow/fetchGuildWants
- `2b987a9` — feat(28-03): pure groupByChar.ts helper + node test (CWANT-06)
- `74b7031` — feat(28-03): WantAddForm char select + WantlistPanel My/Guild toggle & group-by-char

## Self-Check: PASSED

- FOUND: web/src/lib/wantlist/groupByChar.ts
- FOUND: web/src/lib/wantlist/groupByChar.test.ts
- FOUND: .planning/phases/28-character-tagged-wantlist/28-03-SUMMARY.md
- FOUND: commits e573730, 2b987a9, 74b7031
