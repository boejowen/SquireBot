---
phase: 36-shared-character-safe-eviction
plan: 02
subsystem: web
tags: [sveltekit, svelte5, eviction, officer-admin, pure-helpers, vitest-node, d-06]

# Dependency graph
requires:
  - phase: 36-shared-character-safe-eviction (36-01, backend)
    provides: "the additive snake_case preserved_shared_count int field on the eviction-preview JSON (an all-shared owner sends characters:[] + preserved_shared_count>0) — this plan is the first reader"
  - phase: 15/30 (web)
    provides: "the EvictionForm.svelte officer admin surface + the $lib/eviction pure-helper home + the api.ts EvictionPreview contract + the source-inspection test idiom (node vitest is DOM-blind)"
provides:
  - "web/src/lib/api.ts EvictionPreview.preserved_shared_count: number — the field-for-field snake_case mirror of the 36-01 backend field"
  - "canEvictPreview(preview) — the PURE preview-shape Evict-gate (true iff characters.length>0 OR preserved_shared_count>0; false ONLY for a genuine zero-live-chars owner)"
  - "evictPreviewSummary(preview) — the PURE discriminated render shape (cascade / code-only / empty), the code-only branch carrying the '0 characters removed; {N} shared character(s) preserved; guild code will be revoked' framing"
  - "EvictionForm.svelte: an all-shared owner stays evictable (code-only revoke) with explicit messaging; a zero-live-chars owner stays disabled (unchanged); the form is a thin renderer over the two pure helpers"
affects: [v2.5-milestone-close]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Field-for-field snake_case JSON contract mirror (CLAUDE.md): the Go preserved_shared_count int → EvictionPreview.preserved_shared_count: number, no rename/reorder"
    - "Pure DOM-free node-testable gating/framing logic lives in $lib/eviction (vitest is node-only, no jsdom — the repo's established discipline); the .svelte is a thin renderer over the helpers, asserted via source-inspection"
    - "Discriminated-union render shape (cascade/code-only/empty) so the form switches on a kind rather than re-deriving booleans inline — the gate and the render can never drift"

key-files:
  created: []
  modified:
    - "web/src/lib/api.ts — EvictionPreview gains preserved_shared_count: number (snake_case mirror of the 36-01 Go field)"
    - "web/src/lib/eviction.ts — added pure canEvictPreview + evictPreviewSummary helpers (import EvictionPreview from $lib/api)"
    - "web/src/lib/components/EvictionForm.svelte — gate on canEvictPreview, render block switches on evictPreviewSummary.kind, all-shared owner stays enabled with code-only framing"
    - "web/src/lib/__tests__/eviction.test.ts — 6 new helper cases (cascade/all-shared/empty × 2 helpers) + a source-inspection assertion that the form is wired to the helpers (no cascadeEmpty)"

key-decisions:
  - "preserved_shared_count mirrored field-for-field (snake_case number) — no rename, the existing owner_id/characters/grace_until fields untouched"
  - "the gate is split: canEvictPreview owns ONLY the preview-shape half; floorBlocked/evicting/selectedOwner stay in the .svelte (the helper is preview-shape-only, per the plan)"
  - "doEvict success copy + the ConfirmDialog body left unchanged — both already interpolate removed_count and read accurately at 0 for an all-shared owner"

requirements-completed: [OWN-03]   # web half code-complete; FULLY met only after the prod deploy + browser-smoke PASS

# Metrics
duration: ~6min
completed: 2026-06-22
---

# Phase 36 Plan 02: Shared-Character-Safe Eviction (Web) Summary

**Closed the plan-checker D-06 BLOCKER: after 36-01 narrowed the eviction preview, an ALL-SHARED owner (every live char shared → `characters:[]`) made `EvictionForm.svelte`'s `cascadeEmpty` gate disable the Evict button — so the officer could never revoke that departing member's guild code. This plan mirrors the additive `preserved_shared_count` field into `api.ts`, adds two PURE node-tested helpers (`canEvictPreview` / `evictPreviewSummary`) to `eviction.ts`, and rewires the form so an all-shared owner stays evictable (code-only revoke) with the explicit "0 characters removed; {N} shared character(s) preserved; guild code will be revoked" framing — while a genuine zero-live-chars owner stays disabled (unchanged). Web gates green; the prod deploy + officer browser-smoke is the OUTSTANDING blocking checkpoint.**

## Performance

- **Duration:** ~6 min
- **Tasks:** 2 code tasks executed (Task 1 TDD-style: contract mirror + helpers + tests; Task 2: form wiring + source-inspection tests). Task 3 (deploy + browser-smoke) is the OUTSTANDING blocking human-verify checkpoint — NOT run.
- **Files modified:** 4 (all under `web/`)

## Accomplishments

- **Contract mirror (D-06 / CLAUDE.md "JSON mirrored field-for-field"):** `web/src/lib/api.ts`'s `EvictionPreview` gains `preserved_shared_count: number` — the snake_case, field-for-field mirror of the 36-01 Go field. No existing field renamed or reordered.
- **Pure gating helper (`canEvictPreview`):** returns `true` iff `characters.length > 0 || preserved_shared_count > 0` (cascade OR all-shared code-only revoke); returns `false` ONLY for a genuine zero-live-chars owner (`characters:[]` AND `preserved_shared_count == 0`). It owns the preview-shape half of the gate only — `floorBlocked`/`evicting`/`selectedOwner` stay in the `.svelte`.
- **Pure framing helper (`evictPreviewSummary`):** a discriminated `{kind:'cascade'} | {kind:'code-only'; message} | {kind:'empty'; message}` shape. The `code-only` branch carries `"0 characters removed; {N} shared character(s) preserved; guild code will be revoked."`; the `empty` branch keeps the existing `"No characters found for this guildie."`. Pure / DOM-free / label-free (formats counts only) — node-testable.
- **Form rewired (the BLOCKER fix):** `EvictionForm.svelte` drops the standalone `cascadeEmpty`, gates `canEvict` on `canEvictPreview(preview)`, and switches the preview render block on `evictPreviewSummary(preview).kind`. An all-shared owner's Evict button is now **ENABLED** (code-only revoke) with the code-only framing; a zero-live-chars owner stays **DISABLED**; a normal sole-owned owner keeps the existing enabled cascade-list behaviour. `doEvict`'s success copy + the `ConfirmDialog` body are unchanged (both already `removed_count`-driven, accurate at 0).
- **No new XSS sink (T-15-28):** every interpolation stays plain `{}` (Svelte auto-escape); no `{@html}` directive added. Theme tokens only (reused `.cascade-empty`/`.preview` styles; no color literals).
- **Tests:** 6 new node cases cover all three branches (`chars>0`, `chars=0+preserved>0`, `chars=0+preserved=0`) of BOTH helpers; the source-inspection block asserts the form is wired to `canEvictPreview`/`evictPreviewSummary` (and no longer references `cascadeEmpty`), and the existing no-`{@html}` guard stays green.

## Task Commits

Each code task committed atomically on `master`:

1. **Task 1: mirror `preserved_shared_count` + add the pure evict-preview helpers (with node tests)** — `b1aab94` (feat)
2. **Task 2: wire `EvictionForm.svelte` to the helpers — all-shared owner stays evictable with code-only framing** — `8c77086` (feat)

_Plan metadata commit (SUMMARY/STATE/ROADMAP/REQUIREMENTS) is separate._

## Files Modified

- `web/src/lib/api.ts` — `EvictionPreview` gains `preserved_shared_count: number` with a doc-comment mirroring the backend.
- `web/src/lib/eviction.ts` — extended the `$lib/api` import to include `EvictionPreview`; added the two pure exported helpers `canEvictPreview` + `evictPreviewSummary`.
- `web/src/lib/components/EvictionForm.svelte` — imported the two helpers; replaced `cascadeEmpty`/`canEvict` with `previewSummary` + a `canEvictPreview`-backed `canEvict`; the preview render block now switches on `previewSummary.kind`.
- `web/src/lib/__tests__/eviction.test.ts` — added `describe('eviction preview gating + framing (D-06)')` (6 cases) + a source-inspection assertion that the form uses the helpers and no longer references `cascadeEmpty`.

## Decisions Made

None beyond the plan — executed as written. The locked CONTEXT decisions (D-04/D-06) and the 36-01 contract are carried in frontmatter `key-decisions`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Reworded a render-block comment that tripped the no-`{@html}` source-inspection guard**
- **Found during:** Task 2 verification (`npm run test:unit`).
- **Issue:** The explanatory comment I added inside the preview render block literally contained the token `{@html}` ("...via auto-escaping `{}` (never `{@html}`, T-15-28)..."). The existing T-15-28 test asserts `EVICTION_FORM_SOURCE` does NOT contain the substring `{@html`, so my own comment failed that guard (a false positive — there was no actual sink).
- **Fix:** Reworded the comment to "never the raw-HTML directive, T-15-28" — no behavior change, no literal `{@html` token anywhere in the file.
- **Files modified:** `web/src/lib/components/EvictionForm.svelte`
- **Commit:** `8c77086` (folded into the Task 2 commit, before it was committed)

## Authentication Gates

None — no auth gates during the code tasks. (Officer auth IS required for the browser-smoke, which is the outstanding checkpoint, not part of the code tasks.)

## Verification (web gates all green, run from `web/`)

- `npm run check` — **0 errors / 0 warnings** (497 files; the new `EvictionPreview` field + helper types resolve).
- `npm run test:unit -- --run src/lib/__tests__/eviction.test.ts` — **17 passed** (the 6 new gating/framing cases + the existing CR-02/WR-01/WR-02 + source-inspection tests).
- `npm test` (full web suite) — **369 passed (27 files)** — no regression.
- `npm run build` (vite / adapter-static) — succeeds (wrote `build/`).
- Contract-mirror greps: `preserved_shared_count` in `web/src/lib/api.ts` = 2, in `web/src/lib/eviction.ts` = 3.
- No raw-HTML sink: `{@html` in `web/src/lib/components/EvictionForm.svelte` = 0 (T-15-28).
- Scope: `git status` shows changes ONLY under `web/` (+ planning docs); backend / watcher / migrations UNTOUCHED (empty diff across both code commits); NO `v*` tag (latest stays `v2.1.2`).

## Known Stubs

None. `previewing`/`previewSummary` are fully wired; the helpers read real `EvictionPreview` data from `previewEviction()`. The deploy is intentionally deferred (the blocking checkpoint), not a stub.

## OUTSTANDING Checkpoint (Task 3 — deploy + browser-smoke, NOT run by the executor)

**Type:** checkpoint:human-verify (blocking). Left for the orchestrator/human — a live prod deploy to squirebot.quest + visual verification across themes under officer auth is outside an executor's scope. Code is GREEN + committed but NOT deployed.

### Deploy (orchestrator/Claude drives via the ssh-agent workaround — `root@5.78.232.85`)

1. **Build the web bundle:** `cd web && npm run build` (vite / adapter-static → `web/build/`). _(Already verified green by the executor.)_
2. **Web atomic-swap** the `web/build/` output onto `root@5.78.232.85` via the runbook §7 path (PowerShell `ssh.exe`/`scp.exe`; the §7.5 `200.html` guard is in place).
3. **Backend binary swap** — the 36-01 backend deliverable (the narrowed cascade + the `preserved_shared_count` preview field) must be live BEFORE the smoke. Swap the rebuilt backend binary in.
4. **R2 backup** per the runbook before/around the swap.
5. **NO `goose-run`** (no migration — schema stays **v15**), **NO `v*` tag**, **watcher untouched**.
6. **Deploy-correctness proof:** confirm a removed/protected route returns **401-not-404** (the existing deploy smoke).

### Browser-smoke (OFFICER auth on PROD — localhost dev can't auth against prod)

On https://squirebot.quest, logged in as an OFFICER, open **Settings → Admin → Evict guildie**:

1. **Normal owner (sole-owned chars):** pick a guildie with ≥1 non-shared char → the preview shows "Characters affected (N):" + the char list + grace + the consequence callout; the Evict button is **ENABLED**. (Verifying the preview + button state is enough; don't evict a real guildie unless intended — use a throwaway/test owner + restore after if you want a live commit.)
2. **All-shared owner (the load-bearing BLOCKER check):** pick a guildie whose every live char is shared (one other guildies also upload) → the preview shows **"0 characters removed; {N} shared character(s) preserved; guild code will be revoked."** and the Evict button is **ENABLED** (the fix — before this change it was disabled). If committed on a test all-shared owner: it succeeds, the success copy reads "Marked 0 character(s) as removed and revoked the guild code…", the code is revoked (their watcher stops uploading), and the shared chars stay live/un-removed.
3. **Empty owner (zero live chars):** if pickable, the preview shows "No characters found for this guildie." and Evict stays **DISABLED** (unchanged).
4. **No layout/theme regression:** the form renders cleanly under ≥2 themes (theme tokens only); no console error on the eviction page.

**Resume signal:** "approved" if all four pass, or describe the issue.

### Milestone close-out (after the checkpoint PASSES)

With 36-01 (backend) + 36-02 (web) deployed and the browser-smoke PASS, OWN-03 is fully met — and with OWN-01/02/04 from Phase 35, **v2.5 "Ownership Cleanup" is feature-complete** and ready for milestone audit/close. No watcher change → **no `v*` tag for v2.5.**

## Self-Check: PASSED

- All 4 modified web files exist on disk.
- Both task commit hashes (`b1aab94`, `8c77086`) exist in git history.
- `preserved_shared_count` present in `web/src/lib/api.ts` (2) + `web/src/lib/eviction.ts` (3); `{@html` count in `EvictionForm.svelte` == 0.

---
*Phase: 36-shared-character-safe-eviction*
*Code-complete: 2026-06-22 (deploy + browser-smoke OUTSTANDING)*
