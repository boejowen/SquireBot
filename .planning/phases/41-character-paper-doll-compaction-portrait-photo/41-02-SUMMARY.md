---
phase: 41-character-paper-doll-compaction-portrait-photo
plan: 02
subsystem: web
tags: [svelte, sveltekit, css, portrait, upload, base64, canvas, paperdoll, compaction, cors, charui, browser-smoke, deploy]

# Dependency graph
requires:
  - phase: 41-01-backend
    provides: "character_portrait side table (migration 00019) + base64 upload/serve/delete endpoints + has_portrait/portrait_updated_at on CharacterInventory + the assignee-or-officer store gate"
  - phase: 31-characters-tab
    provides: "InventoryWindow.svelte paperdoll + the .doll placeholder + PaperdollSlot onImgError fallback pattern"
  - phase: 15-admin-forms
    provides: "CharMetaForm.svelte upload-control shell (postJSON, .primary/.result, spinner, 44px targets, :focus-visible, authGuard) + the CORS middleware the DELETE fix touches"
provides:
  - "Compacted paper-doll (CHARUI-01): .paperdoll gap 24→16 (mobile 16→8), the 260px .doll floor dropped, a square min(190px,100%) portrait frame, dashed(empty)→solid(filled) border on the same --border token"
  - "Per-character portrait render + upload/remove (CHARUI-02): <img> from GET /characters/{name}/portrait (?v= cache-bust) with onImgError→⚔ silhouette fallback; PortraitControl Add/Change/Remove gated client-side on is_mine||isOfficer"
  - "Pure node-tested portrait.ts validator (png/jpeg/webp allow-list, 256KB cap, SVG/GIF excluded)"
affects: [42-wishlist-polish]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Client-side canvas center-crop + downscale to a square (~512px) before base64 upload — the DOM-bound half stays in the component; the pure allow-list/cap validation is node-tested in portrait.ts"
    - "Cross-origin write method must be advertised in Access-Control-Allow-Methods — a DELETE (non-safelisted) preflights, so the CORS middleware had to add DELETE (found by browser-smoke, invisible to Go/node tests)"
    - "Optimistic $derived-over-prop portrait key so a set/remove re-renders immediately and the ?v={updated_at} cache-bust bumps"

key-files:
  created:
    - web/src/lib/components/PortraitControl.svelte
    - web/src/lib/portrait.ts
    - web/src/lib/__tests__/portrait.test.ts
  modified:
    - web/src/lib/api.ts
    - web/src/lib/components/InventoryWindow.svelte
    - web/src/routes/characters/+page.svelte
    - internal/backendsrv/readapi/cors.go
    - internal/backendsrv/readapi/readapi_test.go

key-decisions:
  - "Control visibility gated CLIENT-SIDE on RosterCharacter.is_mine || Session.isOfficer — no new endpoint needed (the checker's simplification); the server store gate remains authoritative on every write"
  - "Client downscale to a 512px square via canvas object-fit:cover center-crop + JPEG q0.85 — keeps the blob well under the 256KB cap; the server re-sniffs → stores image/jpeg"
  - "The DELETE remove path required adding DELETE to the read-API CORS Access-Control-Allow-Methods (fix-forward c08817d) — a cross-origin DELETE is not CORS-safelisted, so the browser preflighted+blocked it"

patterns-established:
  - "PortraitControl is the first file-input/FileReader/canvas path in the web tree; future image-upload surfaces copy its pick→validate→downscale→preview→base64 flow"

requirements-completed: [CHARUI-01, CHARUI-02]

# Metrics
duration: ~20min execute + browser-smoke + 1 fix-forward (CORS)
completed: 2026-07-16
---

# Phase 41 Plan 02: Character paper-doll — web compaction + portrait render/upload Summary

**Web (SvelteKit) half of Phase 41: the CHARUI-01 paper-doll compaction + the CHARUI-02 per-character portrait render/upload/remove, consuming the 41-01 backend (migration 00019 + the base64 upload/serve/delete endpoints + the has_portrait flag). DEPLOYED LIVE (backend 41-01 + web 41-02 together; prod schema v18→v19); browser-smoke found + fixed a cross-origin CORS-DELETE bug (c08817d), re-deployed, and the user APPROVED.**

## Performance
- **Duration:** ~20 min execute (3 code tasks) + a browser-smoke checkpoint + one backend fix-forward
- **Completed:** 2026-07-16
- **Tasks:** 4 (3 code + 1 blocking browser-smoke checkpoint)
- **Files:** 3 created, 5 modified (3 web code + 2 backend for the CORS fix)

## Accomplishments
- **Contract + validator (Task 1, `f8d80fc`):** `api.ts` — `CharacterInventory` gains `has_portrait`/`portrait_updated_at`; `deleteJSON<T>` (the DELETE sibling of `postJSON`, same credentialed + typed-error contract); `setPortrait`/`removePortrait`/`portraitUrl` wrappers (each `encodeURIComponent(name)`). New pure `portrait.ts` — `validateImageFile` (png/jpeg/webp allow-list, 256KB cap, SVG/GIF excluded), `MAX_PORTRAIT_BYTES`, `stripDataUrlPrefix`; **10 new node tests**.
- **Compaction + render + control (Task 2, `e6530ef`):** `InventoryWindow.svelte` — `.paperdoll` gap 24→16 (mobile 16→8), the 260px `.doll` min-height dropped, a `min(190px,100%)` square `aspect-ratio:1/1` frame, dashed(empty)→solid(filled) on the same `--border` token via `class:filled`; the portrait `<img>` overlays the `⚔` silhouette and `onImgError` hides it so the fallback paints through (`alt={char}` plain-interpolated, no `{@html}`); optimistic `$derived`-over-prop key so a set/remove re-renders and the `?v=` cache-bust bumps. New `PortraitControl.svelte` — the assignee/officer-gated Add/Change/Remove flow: file-pick → `validateImageFile` → canvas center-crop square downscale (~512px) → inline preview → base64 `setPortrait`; `ConfirmDialog`-gated `removePortrait`; verbatim UI-SPEC copy; `--status-ok`/`--status-missing`/`--destructive` tokens (Heavy accent-text caveat); 44px targets, `:focus-visible`, `prefers-reduced-motion`; 401/403 → `authGuard`.
- **Edit gate (Task 3, `5e06519`):** `characters/+page.svelte` — `canEdit` derived from the selected roster row (`is_mine` OR `isOfficer`; bank/bot → officer-only, D-05/D-06), passed into the window so the control renders only for an eligible editor.
- **Gates green:** `npm run check` 0 errors, `npm test` **409/409** (incl. the 10 new portrait tests), `npm run build` clean.

## Task Commits
1. **Task 1: api.ts portrait contract + pure portrait.ts validator + node tests** — `f8d80fc` (feat)
2. **Task 2: Compact paper-doll + portrait render + PortraitControl** — `e6530ef` (feat)
3. **Task 3: Wire the assignee/officer portrait-edit gate on the Characters page** — `5e06519` (feat)
4. **Task 4: Browser-smoke checkpoint (blocking human-verify)** — surfaced one real cross-origin bug, fixed-forward (below); user APPROVED.

## Browser-smoke outcome (Task 4)
The blocking browser-smoke ran deploy-then-smoke-on-prod (web vitest is DOM-blind and node tests can't exercise a real cross-origin browser preflight). One bug:

- **Remove photo failed with "Couldn't save the photo. No changes were made": REAL cross-origin CORS bug, fixed-forward `c08817d`.** The portrait remove is a cross-origin `DELETE` (`squirebot.quest` → `api.squirebot.quest`). `DELETE` is not a CORS-safelisted method, so the browser preflights it — and `readapi/cors.go` advertised only `Access-Control-Allow-Methods: GET, POST, OPTIONS`, so the browser **blocked** the actual DELETE. The fetch threw a network error, which `PortraitControl` surfaced as its generic write-error copy. Add/Change (POST, already allowed) worked; only Remove failed. The Go handler tests + node tests all passed because none exercise a real browser preflight. **Fix:** `cors.go` → `"GET, POST, DELETE, OPTIONS"` + a `readapi_test.go` assertion that DELETE is advertised. Re-deployed (binary-only, no migration); the live OPTIONS preflight now returns `Access-Control-Allow-Methods: GET, POST, DELETE, OPTIONS`. User re-smoked → APPROVED.

## Deploy
- **Backend (41-01) + web (41-02) deployed together to prod 2026-07-16** (ssh-agent workaround: `Start-Service ssh-agent` auto-loads the persisted `squirebot-hetzner` ed25519 key; PowerShell `ssh`/`scp`/`curl.exe`). R2 backup first → cross-compile linux/amd64 → scp binary → `install` (keep `.bak`) + `systemctl restart` → **goose applied 00019 on boot → schema v18→v19** (character_portrait table present) → `npm run build` → tarball → §7.5 atomic swap (chmod, keep `.old`).
- **Verified:** apex 200 · entry JS `text/javascript` (blank-screen canary) · API + the new `GET/POST/DELETE /characters/{name}/portrait` 401-registered-not-404 · schema 19.
- **CORS fix re-deploy (binary-only, no migration, schema stays 19):** the DELETE preflight now advertises DELETE (verified live).
- Watcher untouched → **no `v*` tag**.

## Decisions Made
- **Client-side visibility gate (`is_mine || isOfficer`)** — the roster already carries `is_mine`; no extra endpoint. The server store gate stays authoritative on every write (defense in depth).
- **512px square canvas downscale + JPEG q0.85** — keeps the uploaded blob small (well under 256KB); the server re-sniffs and stores `image/jpeg`.
- **CORS must advertise DELETE** — a cross-origin non-safelisted method preflights; the fix is one line + a regression test.

## Deviations from Plan
- **One backend fix-forward beyond the plan's web scope: the CORS DELETE allow-methods (`c08817d`).** The plan assumed the read-API CORS middleware already admitted the portrait DELETE; the browser smoke proved it advertised only GET/POST/OPTIONS. This is a legitimate cross-origin-only bug the plan/tests could not surface (Go handler tests + node tests don't run a browser preflight). Backend-only, no migration.

## Issues Encountered
- Root-causing the Remove failure required tracing web (path construction identical to the working POST) → the `deleteJSON` contract (returns a JSON body, not 204 — fine) → the backend DELETE handler + route (registered, correct) → the CORS middleware allow-methods (missing DELETE). The generic error copy ("Couldn't save the photo…") for the remove op is technically imprecise (it says "save" on a remove) — noted for a future polish, but the underlying bug was CORS, now fixed.

## User Setup Required
None — deployed. The feature is live: guildies can set/change/remove a portrait on characters they're assigned to (officers on any, incl. banks/bots).

## Next Phase Readiness
- CHARUI-01 + CHARUI-02 delivered and live; Phase 41 closes v2.6 to **5/6**.
- Phase 42 (Wishlist compaction + sub-Velious tiers; WISHUI-01/02, no migration) is the last v2.6 phase.

## Self-Check: PASSED
- All created/modified files exist on disk (`PortraitControl.svelte`, `portrait.ts`, `portrait.test.ts` created; `api.ts`, `InventoryWindow.svelte`, `characters/+page.svelte`, `cors.go`, `readapi_test.go` modified).
- Task commits exist: `f8d80fc`, `e6530ef`, `5e06519`; fix-forward `c08817d`.
- Web gates green (check 0, 409 tests, build); backend build/readapi tests green after the CORS fix.

---
*Phase: 41-character-paper-doll-compaction-portrait-photo*
*Completed: 2026-07-16*
