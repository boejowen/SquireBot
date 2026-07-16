---
phase: 41-character-paper-doll-compaction-portrait-photo
verified: 2026-07-16T15:35:00Z
status: passed
score: 17/17 must-haves verified
overrides_applied: 0
re_verification:
  # initial verification — no prior VERIFICATION.md existed
---

# Phase 41: Character paper-doll — compaction + portrait photo Verification Report

**Phase Goal:** The character paper-doll stops wasting its reserved portrait space — the layout is compacted toward the in-game inventory-window idiom — AND that portrait area becomes useful: a guildie can set an optional portrait photo per character that displays there.
**Verified:** 2026-07-16T15:35:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

Merged from ROADMAP §Phase 41 Success Criteria (SC-1..SC-4, the contract) + 41-01/41-02 PLAN `must_haves`.

| #  | Truth | Status | Evidence |
| -- | ----- | ------ | -------- |
| SC-1 | Paper-doll compacted — portrait dead space reclaimed, tighter in-game idiom | ✓ VERIFIED | `InventoryWindow.svelte:409` `.paperdoll gap 24→16`; `:437-438` `min-height:260px` REMOVED → `width:min(190px,100%)` + `aspect-ratio:1/1`; `:583` mobile gap→8px. Git `e6530ef` diff shows `-min-height: 260px` / `-gap: 24px` |
| SC-2 | A guildie can upload an optional portrait for a char they own; stored + associated | ✓ VERIFIED | `PortraitControl.svelte:142-157` savePhoto→`setPortrait`; `webadmin/portrait.go:70-123` decode→cap→sniff→`SetPortraitTx`; `store/portrait.go:91-112` upsert into `character_portrait` PK=character_id |
| SC-3 | Char with portrait shows it (replaces placeholder); char with none keeps placeholder | ✓ VERIFIED | `InventoryWindow.svelte:204-220` `{#if hasPortrait && !imgHidden}<img>` overlays the `⚔` silhouette under-layer; `class:filled` dashed→solid; `hasPortrait` from `inventory.has_portrait` (`:46`) |
| SC-4 | Extend-only migration + upload path + read-API; `go test ./...` green; watcher untouched, no `v*` tag | ✓ VERIFIED | migration `00019` (highest, extend-only); `go test ./...` exit 0; phase diff `792a56b^..c08817d` touches ZERO watcher files; no `v*` tag points at `c08817d` (latest tag v2.1.2 predates phase) |
| 5 | Assignee OR officer can upload PNG/JPEG/WebP via base64-in-JSON POST | ✓ VERIFIED | `store/portrait.go:60-82` `authorizePortraitWriteTx` (assignee `IsCharAssignedToTx` OR `isOfficerTx`); `webadmin/portrait.go:43-45` `image_base64` body |
| 6 | Stranger upload/delete → 403 not_authorized | ✓ VERIFIED | `store/portrait.go:77-79` non-authorized → `ErrNotAuthorized`; `webadmin/portrait.go:161-162` maps to 403; `webadmin/portrait_test.go` stranger→403 test present |
| 7 | Bank/bot char (no assignee) accepts portrait ONLY from an officer | ✓ VERIFIED | `store/portrait.go:61-71` `charSharedTx` true → assignee branch skipped → officer-only (D-06); client mirror `characters/+page.svelte:211-213` |
| 8 | SVG/GIF/oversize/non-image rejected 400; server sniffs magic bytes, never client claim | ✓ VERIFIED | `webadmin/portrait.go:52-63` fixed 3-way `sniffImageType` (PNG/JPEG/WebP only); `:90-99` 256KB cap FIRST then sniff; `portrait_test.go` `TestPortraitSet_RejectsSVG` present; no `image_base64`-adjacent client content_type field |
| 9 | GET .../portrait streams stored blob with sniffed content_type + nosniff | ✓ VERIFIED | `readapi/portrait.go:55-73` `GetPortrait`→`w.Write(blob)`; `:69` Content-Type from stored `ct`; `:70` `X-Content-Type-Options: nosniff` |
| 10 | Inventory payload carries has_portrait + portrait_updated_at (flag only, never bytes) | ✓ VERIFIED | `compute/types.go:201-202` fields; `compute/inventory.go:136-141` populated via real `s.PortraitMeta` read; bytes never inline (dedicated serve endpoint) |
| 11 | Portrait removable via DELETE, reverts has_portrait to false | ✓ VERIFIED | `store/portrait.go:119-133` `DeletePortraitTx` (gate-before-delete); route `main.go:387`; `PortraitControl.svelte:165-179` confirmRemove→`removePortrait`→`onchanged({has_portrait:false})` |
| 12 | Portrait renders in reclaimed frame; on img error falls back to ⚔ silhouette | ✓ VERIFIED | `InventoryWindow.svelte:209-214` `<img onerror={onImgError}>`; `:70-72` `imgHidden=true`→silhouette paints through (PaperdollSlot idiom) |
| 13 | Assignee OR officer sees Add/Change/Remove; everyone else read-only frame (no controls) | ✓ VERIFIED | `characters/+page.svelte:209-214` `canEdit` derived (assignee OR officer, bank/bot officer-only); `:301` passed to window; `InventoryWindow.svelte:222` `{#if canEdit}<PortraitControl>` |
| 14 | Pick PNG/JPEG/WebP → downscale square → preview → Save uploads base64; ?v= bumps | ✓ VERIFIED | `PortraitControl.svelte:99-133` canvas center-crop→512px square; `:204-217` preview + Save/Cancel; `InventoryWindow.svelte:48-52,211` `portraitVersion`→`?v=` cache-bust |
| 15 | Wrong-type/oversize file rejected client-side with exact UI-SPEC error copy before request | ✓ VERIFIED | `PortraitControl.svelte:79-86` `validateImageFile` gate BEFORE any fetch; error strings match UI-SPEC verbatim; `portrait.ts:20-28` pure allow-list + cap (10 node tests green) |
| 16 | Remove asks confirmation, then DELETEs and reverts to silhouette | ✓ VERIFIED | `PortraitControl.svelte:159-179` `askRemove`→ConfirmDialog→`confirmRemove`; `:267-274` ConfirmDialog with verbatim copy |
| 17 | Cross-origin DELETE preflight advertised (the browser-smoke fix) | ✓ VERIFIED | `readapi/cors.go:60` `Access-Control-Allow-Methods: "GET, POST, DELETE, OPTIONS"`; `readapi_test.go:386-387` regression asserts DELETE advertised; git `c08817d` |

**Score:** 17/17 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `internal/backendsrv/migrations/00019_character_portrait.sql` | side table (blob+content_type+byte_size+updated_at, FK CASCADE) | ✓ VERIFIED | `CREATE TABLE character_portrait` w/ `character_id PK REFERENCES character(id) ON DELETE CASCADE`; extend-only; highest migration |
| `internal/backendsrv/store/portrait.go` | blob CRUD + assignee-or-officer in-tx gate | ✓ VERIFIED | `SetPortraitTx`/`DeletePortraitTx`/`GetPortrait`/`PortraitMeta`/`ErrPortraitNotFound` all exported; real gate + PK-PK join |
| `internal/backendsrv/webadmin/portrait.go` | base64→sniff→256KB cap→withTx→audit | ✓ VERIFIED | `PortraitSetHandler`/`PortraitDeleteHandler`; cap-first then 3-way sniff; audit `portrait_set`/`portrait_removed` |
| `internal/backendsrv/readapi/portrait.go` | raw-byte serve + nosniff | ✓ VERIFIED | `NewPortrait`/`PortraitHandler`; content-type from stored sniff; nosniff; 404 discipline |
| `web/src/lib/portrait.ts` | pure validator (type sniff + 256KB cap) | ✓ VERIFIED | `validateImageFile`/`MAX_PORTRAIT_BYTES`/`stripDataUrlPrefix`; SVG/GIF excluded; 10 node tests green |
| `web/src/lib/api.ts` | has_portrait/portrait_updated_at + setPortrait/removePortrait/portraitUrl | ✓ VERIFIED | fields on `CharacterInventory`; `deleteJSON`; all 3 wrappers `encodeURIComponent(name)` |
| `web/src/lib/components/PortraitControl.svelte` | gated Add/Change/Remove flow | ✓ VERIFIED | file input `accept="image/png,image/jpeg,image/webp"`; downscale/preview/upload/delete; no `{@html}` |
| `web/src/lib/components/InventoryWindow.svelte` | compacted layout + `<img>` + fallback + control mount | ✓ VERIFIED | gap 24→16, 260px floor dropped, square frame; `portrait-img` + onImgError; `{#if canEdit}<PortraitControl>` |
| `web/src/routes/characters/+page.svelte` | canEdit visibility gate | ✓ VERIFIED | `canEdit` (assignee OR officer, bank/bot officer-only) passed to window |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | -- | --- | ------ | ------- |
| `main.go` | GET/POST/DELETE .../portrait | `RequireSession` route reg | ✓ WIRED | `main.go:385-387` all 3 routes under `webauth.RequireSession` |
| `store/portrait.go` | `character_portrait` table | INSERT…ON CONFLICT / SELECT / DELETE | ✓ WIRED | `:99-107` upsert, `:127-128` delete, `:144-148`/`:165-168` reads |
| `compute/inventory.go` | `CharacterInventory.HasPortrait` | `s.PortraitMeta` attached to assembled inventory | ✓ WIRED | `:136-141` real read populates flag, non-fatal on error |
| `InventoryWindow.svelte` | GET .../portrait?v={updated_at} | `portraitUrl(char, version)` in `<img src>` | ✓ WIRED | `:211` `src={portraitUrl(inventory.char, portraitVersion)}` |
| `PortraitControl.svelte` | POST/DELETE .../portrait | `setPortrait`/`removePortrait` over postJSON/deleteJSON | ✓ WIRED | `:148,170` |
| `characters/+page.svelte` | PortraitControl visibility | `is_mine` OR `isOfficer`, bank/bot officer-only | ✓ WIRED | `:209-214,301` |
| `readapi/cors.go` | browser DELETE preflight | `Access-Control-Allow-Methods` | ✓ WIRED | `:60` includes DELETE (fix `c08817d`) |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| -------- | ------------- | ------ | ------------------ | ------ |
| `InventoryWindow.svelte` portrait `<img>` | `hasPortrait` / `portraitVersion` | `inventory.has_portrait` / `.portrait_updated_at` ← backend `compute.StructuredInventory` → real `s.PortraitMeta` DB read | ✓ Yes — PK-PK join on `character_portrait` | ✓ FLOWING |
| `PortraitControl` upload | `previewDataUrl` | real File → canvas downscale → base64 → `setPortrait` POST → SQLite BLOB | ✓ Yes | ✓ FLOWING |
| `canEdit` prop | `selectedRow.is_mine`/`isOfficer`/`is_bank_toon` | `RosterCharacter` (api.ts:166-168) + Session (auth.ts:27) — real server fields | ✓ Yes — not hardcoded | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Backend portrait tests pass (uncached) | `go test -count=1 -run Portrait ./…/store ./…/webadmin ./…/readapi` | store 11.0s / webadmin 12.5s / readapi 6.4s all `ok` | ✓ PASS |
| Full backend suite green (SC-4) | `go test ./...` | exit 0, no failures | ✓ PASS |
| Web portrait validator tests | `npx vitest run …/portrait.test.ts` | 10/10 passed | ✓ PASS |
| `go build ./...` | build | exit 0 | ✓ PASS |
| Watcher untouched | `git diff --name-only 792a56b^..c08817d \| grep watcher/sheet` | 0 watcher files | ✓ PASS |
| No `v*` tag on phase | `git tag --points-at c08817d` | none (latest v2.1.2 predates) | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ----------- | ----------- | ------ | -------- |
| CHARUI-01 | 41-02 | Paper-doll compacted to reclaim empty portrait space | ✓ SATISFIED | `InventoryWindow.svelte:409,437-438,583` (SC-1, Truth SC-1) |
| CHARUI-02 | 41-01, 41-02 | Optional per-character portrait: image ref + migration + upload path + read-API | ✓ SATISFIED | migration 00019 + store/webadmin/readapi + web control (Truths 2-17) |

No orphaned requirements — REQUIREMENTS.md maps only CHARUI-01/CHARUI-02 to Phase 41, both claimed and satisfied.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| — | — | No stubs, no `{@html}`, no hardcoded-empty rendered data, no placeholder returns | — | none |

Scan notes: `has_portrait` flag is backed by a real `PortraitMeta` DB read (not a hardcoded `false`); the serve endpoint streams real stored bytes; the client `canEdit` gate reads real roster/session fields. The `⚔` silhouette is the intended designed fallback (D-07), not a stub. Zero `{@html}` in either touched component (XSS-safe alt via plain interpolation confirmed). One noted cosmetic (from 41-02-SUMMARY, non-blocking): the generic remove-failure copy says "save" on a remove op — a wording nit, not a functional gap.

### Human Verification Required

None outstanding. The browser-smoke checkpoint (the one item that could not be verified programmatically — DOM render, file-pick/downscale/preview, real cross-origin round-trip) was already executed against prod by the user, surfaced + fixed the CORS-DELETE bug (`c08817d`), re-smoked, and was **APPROVED** (per 41-02-SUMMARY + the established-facts brief). The CORS fix that unblocked Remove is now covered by an automated regression assertion (`readapi_test.go:386-387`), so this is no longer human-gated.

### Gaps Summary

No gaps. All 17 observable truths verify against the codebase with file:line evidence, all 9 artifacts are substantive and wired, all 7 key links are connected, the backend data flows end-to-end (real `PortraitMeta` DB read → inventory payload → `<img src>`), and the client edit-gate reads real server fields. Both requirements (CHARUI-01 compaction + CHARUI-02 portrait) are satisfied. `go test ./...` and the web portrait suite are green, `go build ./...` is clean, the watcher is untouched with no `v*` tag, and the one behavior that needed a real browser (the cross-origin round-trip) was smoked + approved on prod, with the CORS fix now locked by a regression test.

---

_Verified: 2026-07-16T15:35:00Z_
_Verifier: Claude (gsd-verifier)_
