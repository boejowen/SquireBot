---
phase: 41-character-paper-doll-compaction-portrait-photo
reviewed: 2026-07-16T00:00:00Z
depth: deep
files_reviewed: 14
files_reviewed_list:
  - cmd/squirebot-server/main.go
  - internal/backendsrv/compute/inventory.go
  - internal/backendsrv/compute/types.go
  - internal/backendsrv/migrations/00019_character_portrait.sql
  - internal/backendsrv/readapi/cors.go
  - internal/backendsrv/readapi/portrait.go
  - internal/backendsrv/store/portrait.go
  - internal/backendsrv/webadmin/portrait.go
  - web/src/lib/api.ts
  - web/src/lib/portrait.ts
  - web/src/lib/components/InventoryWindow.svelte
  - web/src/lib/components/PortraitControl.svelte
  - web/src/routes/characters/+page.svelte
  - (tests: portrait_test.go x2, readapi_test.go, portrait.test.ts — read for coverage)
findings:
  critical: 0
  warning: 1
  info: 5
  total: 6
status: issues_found
---

# Phase 41: Code Review Report

**Reviewed:** 2026-07-16
**Depth:** deep (cross-file: store gate ↔ handlers ↔ routes ↔ web contract; auth chain traced through `charSharedTx`/`IsCharAssignedToTx`/`isOfficerTx`)
**Files Reviewed:** 14 (7 Go, 4 web, 1 SQL, tests spot-checked)
**Status:** issues_found (0 BLOCKER, 1 HIGH, 5 LOW/NIT)

## Summary

Phase 41 adds a per-character portrait photo (base64-in-JSON upload, magic-byte sniff, 256 KB cap, raw-byte serve) plus paper-doll compaction. The security posture is genuinely strong and I could not find an auth, injection, XSS, or IDOR defect:

- **Authorization is correct and TOCTOU-safe.** `authorizePortraitWriteTx` composes `charSharedTx → IsCharAssignedToTx → isOfficerTx` under the caller's tx on BOTH POST and DELETE, BEFORE any state change. The D-06 bank/bot flip (shared → officer-only) is implemented exactly right. An empty caller id fails closed (`isOfficerTx("")→false`). No existence leak that matters (guild char names are already member-visible on the roster).
- **The upload allow-list is tight.** `sniffImageType` is a fixed PNG/JPEG/WebP magic-byte switch (SVG/GIF/everything-else rejected); content-type is set from the sniff and replayed verbatim on serve with `X-Content-Type-Options: nosniff`. Tests cover SVG, GIF, oversize, malformed base64, stranger→403, unknown-char→400, nosniff, and stored-content-type.
- **No `{@html}`** anywhere; `alt={char}` and the char name render via plain Svelte interpolation (auto-escaped). SQL is fully parameterized. The PK↔PK 1:1 portrait join cannot fan out. Migration 00019 matches the project's bare-`CREATE TABLE` + `ON DELETE CASCADE` convention with `foreign_keys(ON)` in the DSN. The CORS fix correctly adds DELETE while keeping the exact origin + credentials.
- `go build ./...` clean; store/webadmin/readapi/compute test packages green.

The one non-trivial finding is a DoS-amplification gap: the 256 KB cap is enforced only AFTER the entire request body is read into memory and fully base64-decoded, with no `MaxBytesReader` on the handler and no server-wide body limit. The code comment claims "SIZE CAP FIRST … reject-early/anti-DoS," but the reject is not actually early. This is the portrait endpoint specifically — it is the one web-write surface designed to accept a large blob, so unlike the tiny-JSON `charmeta` sibling it is a real memory-amplification vector. Given the endpoint is login-gated (guild-only, ~12 trusted members) I rate it HIGH, not BLOCKER, and fixed-forward is acceptable.

## Warnings

### WR-01 (HIGH): 256 KB cap is enforced after the full body is read + decoded — no early reject, no `MaxBytesReader`

**File:** `internal/backendsrv/webadmin/portrait.go:80-93`
**Issue:** The handler does `json.NewDecoder(r.Body).Decode(&req)` against an **unbounded** `r.Body`, then `base64.StdEncoding.DecodeString(req.ImageBase64)`, and only THEN checks `len(decoded) > maxPortraitBytes`. There is no `http.MaxBytesReader` on this handler and no server-level body cap (`cmd/squirebot-server/main.go:476` sets only `Addr`+`Handler`; no `MaxHeaderBytes`, no read limit). An authenticated member can POST a multi-hundred-MB `image_base64` string; the server buffers the entire JSON body into `req.ImageBase64` (one allocation) and then allocates ~75% again for the decoded bytes — both held in memory concurrently — before the cap ever runs. The file's own header comment (lines 9-12) advertises "SIZE CAP FIRST … reject-early/anti-DoS," which the code does not deliver.

Note: the tiny-JSON siblings (`charmeta.go`, coin/eviction) also lack `MaxBytesReader`, so this is a pre-existing package pattern — but those payloads are bounded by their fixed scalar fields. The portrait endpoint is the first web-write surface that accepts a large blob by design, which is what turns the missing cap from theoretical into a real amplification vector.

**Fix:** Cap the reader before decode so an oversized body is rejected without full buffering. Size the cap to the max base64 expansion of 256 KB (~4/3 + JSON envelope, e.g. 384 KB):

```go
const maxPortraitBodyBytes = 384 * 1024 // ~4/3 * 256KB decoded + JSON envelope slack
// ... inside the handler, first thing after method check:
r.Body = http.MaxBytesReader(w, r.Body, maxPortraitBodyBytes)
// json.Decode now surfaces an oversized body as a decode error → 400 invalid_input,
// bounding memory BEFORE DecodeString. Keep the existing len(decoded) > maxPortraitBytes
// check as the exact decoded-size gate.
```

This mirrors the ingest handler's established `http.MaxBytesReader(w, r.Body, maxBodyBytes)` discipline (`internal/backendsrv/ingest/handler.go:93`), which the portrait handler should have copied.

## Info

### IN-01 (LOW): `handleWriteError` `_op` param is dead — a failed *Remove* shows "Couldn't save the photo"

**File:** `web/src/lib/components/PortraitControl.svelte:184-198`
**Issue:** `handleWriteError(err, _op: 'save' | 'remove')` takes the op but never reads it (leading underscore signals "intentionally unused"). The generic branch always sets `"Couldn't save the photo. No changes were made."`, so a failed DELETE (e.g. an unknown-char 400 → `invalid_input`, or a transport error) surfaces "save" copy on a remove action — the exact UX defect called out in the task focus list. Both call sites (`savePhoto` line 154, `confirmRemove` line 176) pass the op, so the plumbing exists; only the branch is missing.
**Fix:** Use `_op` in the generic branch:
```ts
errorMsg = _op === 'remove'
  ? "Couldn't remove the photo. No changes were made."
  : "Couldn't save the photo. No changes were made.";
```
Rename the param to `op` once used.

### IN-02 (LOW): optimistic `portraitOverride` never clears for the same char — stale `?v=` after a re-fetch

**File:** `web/src/lib/components/InventoryWindow.svelte:36-52`
**Issue:** `portraitOverride` is keyed by `char` and wins over the prop whenever `portraitOverride.char === inventory.char`. If the user uploads for char A, switches to B, then back to A, a fresh `inventory` for A flows in from the server (now carrying the real `has_portrait`/`portrait_updated_at`), but the stale override still matches `char === 'A'` and shadows the server value. If another officer changed A's portrait in between, the `?v=` cache-bust is stale and the browser may serve a cached older image. Self-heals on full page reload; the `InventoryWindow` instance is reused (not keyed) across selections so component-local state persists.
**Fix:** Clear the override when a new `inventory` object arrives for the same char, e.g. reset it in the same `$effect` that resets `imgHidden` on `imgHiddenFor` change, or compare `inventory.portrait_updated_at` and drop the override once the server's value catches up. Low urgency (single-editor is the common case).

### IN-03 (NIT): migration comment says "idempotent" but uses bare `CREATE TABLE` (no `IF NOT EXISTS`)

**File:** `internal/backendsrv/migrations/00019_character_portrait.sql:6, 21`
**Issue:** The header comment calls the migration idempotent, but `CREATE TABLE character_portrait` (not `IF NOT EXISTS`) would error on a literal re-run. In practice goose's version ledger guarantees single application, and all 11 prior CREATE-TABLE migrations use the identical bare form, so behavior is correct and consistent with the project norm — the word "idempotent" just overstates the SQL. No functional impact.
**Fix:** None required (convention-consistent). Optionally soften the comment to "goose applies this once (version-ledger)" to avoid implying the DDL itself is replay-safe.

### IN-04 (NIT): client `too_large` / `invalid_image` error branches are effectively unreachable on the happy path

**File:** `web/src/lib/components/PortraitControl.svelte:191-194`
**Issue:** The canvas downscale (`downscaleToSquare`, 512px, JPEG q0.85) always produces a small `image/jpeg`, so the server's `too_large` and `invalid_image` responses can't be triggered by the normal flow — these branches only fire for a hand-crafted request bypassing the UI. That's correct defense-in-depth (the mapped copy is still good to have), just noting the paths are dead for real users. No action needed.

### IN-05 (NIT): `PortraitResult.updated_at` is optional but the POST handler always returns it

**File:** `web/src/lib/api.ts` (`PortraitResult`, ~line 665) vs `internal/backendsrv/webadmin/portrait.go:121, 151`
**Issue:** `updated_at?: string` is declared optional to accommodate the DELETE reply (`{character}` only), but `savePhoto` uses `res.updated_at ?? ''`. On the POST path the server always includes `updated_at` (line 121), so the `?? ''` fallback is dead for set; on DELETE the caller passes `updated_at: ''` explicitly (line 171) and never reads `res.updated_at`. Contract is fine; the optionality is just a mild imprecision (one interface covering two shapes). No action needed — or split into `PortraitSetResult`/`PortraitDeleteResult` if strictness is wanted later.

---

## Verified clean (no findings)

- **Auth gate (D-05/D-06):** `authorizePortraitWriteTx` under-tx on POST+DELETE, before write; bank/bot officer-only flip correct; empty-caller fail-closed. (`store/portrait.go:56-133`)
- **Upload validation:** magic-byte sniff restricts to PNG/JPEG/WebP; SVG/GIF rejected; content-type from sniff never client claim; WebP sniff correctly requires "WEBP" fourCC at [8:12] (distinguishes from WAV/AVI RIFF); all length-guards precede slice access. (`webadmin/portrait.go:47-99`)
- **Serve hardening:** `Content-Type` from stored sniff, `X-Content-Type-Options: nosniff`, 404-not-401 discipline, raw-byte write only, V7 logging (no name/bytes). (`readapi/portrait.go`)
- **XSS:** no `{@html}`; `alt`/name via plain interpolation; `img` error falls through to silhouette. (`InventoryWindow.svelte`, `PortraitControl.svelte`)
- **SQL:** every query parameterized `?`; name-lookup matches `binding.go` UNIQUE-COLLATE-NOCASE convention; no injection surface.
- **Migration:** PK/FK `ON DELETE CASCADE` with `foreign_keys(ON)` in DSN; PK↔PK 1:1 join can't fan out; extend-only; no `WatcherMaxSchemaVersion` bump needed (watcher off this path — correct).
- **CORS fix:** DELETE added; exact origin preserved (never `*`); `Allow-Credentials: true` retained; regression test added. (`readapi/cors.go`, `readapi_test.go`)
- **`deleteJSON` contract:** matches the 200-with-JSON-body DELETE handler; credential + typed-error contract mirrors `postJSON`.
- Build clean; store/webadmin/readapi/compute Go tests green.

---

_Reviewed: 2026-07-16_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
