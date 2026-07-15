---
phase: 41-character-paper-doll-compaction-portrait-photo
plan: 01
subsystem: api
tags: [go, sqlite, goose, blob, base64, image-upload, magic-byte-sniff, idor, audit-log, portrait]

# Dependency graph
requires:
  - phase: 26-character-assignment
    provides: "character_assignment + IsCharAssignedToTx/charSharedTx/isOfficerTx — the assignee-OR-officer in-tx gate reused verbatim"
  - phase: 31-characters-tab
    provides: "compute.StructuredInventory + readapi.NewInventory scaffold + CharacterInventory payload the portrait flag rides"
  - phase: 16-char-meta
    provides: "webadmin JSON-POST handler shape (decode->withTx->audit) + AppendAuditTx + writeJSON/writeJSONError/caller/nowUnix"
provides:
  - "character_portrait side table (migration 00019, schema v18->v19): image_blob + sniffed content_type + byte_size + updated_at, FK ON DELETE CASCADE"
  - "store/portrait.go: SetPortraitTx/DeletePortraitTx (assignee-OR-officer in-tx gate) + GetPortrait (serve) + PortraitMeta (flag) + ErrPortraitNotFound"
  - "webadmin PortraitSetHandler/PortraitDeleteHandler: base64 decode -> 256KB cap -> 3-way magic-byte sniff (PNG/JPEG/WebP only) -> withTx store + audit"
  - "readapi.NewPortrait: raw-byte serve handler (Content-Type from stored sniff + X-Content-Type-Options: nosniff)"
  - "CharacterInventory.has_portrait + portrait_updated_at (D-07 flag, never the bytes inline)"
  - "3 routes: GET/POST/DELETE /api/v1/characters/{name}/portrait under RequireSession"
affects: [41-02-web-portrait-render, characters-tab, inventory-window]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "First raw-byte HTTP response handler in the API (w.Write(blob) + image/* Content-Type; every other handler is json.NewEncoder)"
    - "First image-upload validation pipeline: base64 decode -> size cap -> fixed 3-way magic-byte switch (SVG/GIF excluded, content_type from the sniff not the client)"
    - "Assignee-OR-officer in-tx gate (ORed authorizePortraitWriteTx) — a new composition of the P26 officer-only OfficerAssignTx posture"

key-files:
  created:
    - "internal/backendsrv/migrations/00019_character_portrait.sql"
    - "internal/backendsrv/store/portrait.go"
    - "internal/backendsrv/store/portrait_test.go"
    - "internal/backendsrv/webadmin/portrait.go"
    - "internal/backendsrv/webadmin/portrait_test.go"
    - "internal/backendsrv/readapi/portrait.go"
    - "internal/backendsrv/readapi/portrait_test.go"
  modified:
    - "internal/backendsrv/compute/types.go"
    - "internal/backendsrv/compute/inventory.go"
    - "cmd/squirebot-server/main.go"

key-decisions:
  - "D-07 payload placement: has_portrait + portrait_updated_at ride the CharacterInventory (inventory fetch), NOT the roster — the portrait frame lives inside InventoryWindow which reads the inventory payload"
  - "D-08 removal: a real DELETE /api/v1/characters/{name}/portrait route (RESTful), not a POST /remove"
  - "content_type is SNIFFED server-side (fixed 3-way magic-byte switch) and stored/served from the sniff — never the client claim; SVG+GIF rejected 400"
  - "256KB decoded-byte cap enforced BEFORE the sniff and the store write (reject-early, anti-DoS)"
  - "PortraitMeta wired at the StructuredInventory store-access site (a PK<->PK PortraitMeta read), not inside the pure buildStructuredInventory (plan-checker note honored)"
  - "store updated_at is TEXT RFC3339 (time.Now().UTC().Format) while the audit-log now stays int nowUnix() — the two timestamps are DISTINCT, never conflated (plan-checker note honored)"

patterns-established:
  - "Portrait blob lives in a side table (not the hot character row) so roster/inventory reads never pull it; the flag rides the payload, the bytes stream from a dedicated login-gated GET"
  - "authorizePortraitWriteTx: assignee (IsCharAssignedToTx) OR officer (isOfficerTx) ORed under the tx; a shared bank/bot (charSharedTx=true, no assignee) is officer-ONLY (D-06); the gate runs BEFORE the DELETE so a stranger cannot probe/remove"

requirements-completed: [CHARUI-02]

# Metrics
duration: 25min
completed: 2026-07-15
---

# Phase 41 Plan 01: Portrait Photo Backend Summary

**SQLite-BLOB per-character portrait: migration 00019 side table + assignee-or-officer-gated base64 upload with server-side magic-byte sniffing (PNG/JPEG/WebP, 256KB cap, SVG/GIF rejected), a raw-byte serve endpoint with nosniff, and the additive has_portrait/portrait_updated_at flag on the inventory payload.**

## Performance

- **Duration:** ~25 min
- **Started:** 2026-07-15T17:05:00Z
- **Completed:** 2026-07-15T17:25:00Z
- **Tasks:** 3
- **Files modified:** 10 (7 created, 3 modified)

## Accomplishments
- **Migration 00019** (`character_portrait` side table, schema v18->v19): one-portrait-per-char PK/FK with `ON DELETE CASCADE`, `image_blob BLOB`, `content_type TEXT` (sniffed), `byte_size INTEGER`, `updated_at TEXT`. Forward-only, backend-only — NO `WatcherMaxSchemaVersion` bump, NO `_meta.schema_version` (goose version is the record).
- **`store/portrait.go`**: `SetPortraitTx`/`DeletePortraitTx` gate every write with `authorizePortraitWriteTx` (assignee OR officer, ORed under the tx, WR-04; bank/bot officer-only per D-06); `GetPortrait` serves the blob; `PortraitMeta` is the PK<->PK flag read; `ErrPortraitNotFound` declared, `ErrCharNotFound`/`ErrNotAuthorized` reused.
- **`webadmin/portrait.go`**: `PortraitSetHandler` decodes base64, enforces the 256KB cap FIRST, sniffs the magic bytes (a fixed 3-way switch, SVG/GIF excluded), stores the sniffed type, and audits `portrait_set` — all in one `withTx`. `PortraitDeleteHandler` audits `portrait_removed`.
- **`readapi/portrait.go`**: the API's first raw-byte serve handler — streams the stored blob with `Content-Type: image/<sniffed>` + `X-Content-Type-Options: nosniff` + a short private cache; 404 for portrait-less/unknown, 405 for non-GET.
- **Additive payload**: `CharacterInventory` gains `has_portrait` + `portrait_updated_at` (flag only, never the bytes), populated at the `StructuredInventory` store-access site.
- **3 routes** registered under `RequireSession` (login-only; the assignee-or-officer gate is enforced in the store tx, never `RequireOfficer` at the route).
- **36 new Go tests** across store/webadmin/readapi/compute — all green; whole backend module builds, vets, and tests clean; the server binary builds.

## Task Commits

Each task was committed atomically:

1. **Task 1: Migration 00019 + store/portrait.go (blob CRUD + assignee-or-officer in-tx gate)** — `792a56b` (feat)
2. **Task 2: webadmin upload/delete handlers (base64 + magic-byte sniff + 256KB cap + audit) + readapi serve handler** — `4635326` (feat)
3. **Task 3: additive has_portrait/portrait_updated_at on CharacterInventory + PortraitMeta read + 3 routes** — `201f096` (feat)

_TDD note: Tasks 1 & 2 were `tdd="true"`; because the table tests must compile against the store/handler under test, each task's test + implementation shipped in one atomic feat commit (the store/handler and its `_test.go` are a single unit), then verified green._

## Files Created/Modified
- `internal/backendsrv/migrations/00019_character_portrait.sql` — the side table (blob + sniffed content_type + byte_size + updated_at, FK CASCADE); forward-only header mirrors 00016.
- `internal/backendsrv/store/portrait.go` — `SetPortraitTx`/`DeletePortraitTx`/`GetPortrait`/`PortraitMeta`/`ErrPortraitNotFound` + the private `resolveCharIDTx` + `authorizePortraitWriteTx`.
- `internal/backendsrv/store/portrait_test.go` — assignee/officer/stranger/bank-bot gate, upsert-overwrites, not-found sentinels, delete idempotency, ON DELETE CASCADE, PortraitMeta.
- `internal/backendsrv/webadmin/portrait.go` — `PortraitSetHandler`/`PortraitDeleteHandler` + `sniffImageType` + `mapPortraitErr` + `maxPortraitBytes`.
- `internal/backendsrv/webadmin/portrait_test.go` — PNG/JPEG/WebP happy, SVG/GIF/oversize/malformed/empty rejects, stranger->403, unknown->400, delete audit, 405, success body shape.
- `internal/backendsrv/readapi/portrait.go` — `PortraitHandler`/`NewPortrait` raw-byte serve.
- `internal/backendsrv/readapi/portrait_test.go` — streams-stored-bytes, 404 (portrait-less + unknown), 405, RequireSession-401.
- `internal/backendsrv/compute/types.go` — `CharacterInventory` gains `has_portrait` + `portrait_updated_at`.
- `internal/backendsrv/compute/inventory.go` — `StructuredInventory` attaches the flag via `s.PortraitMeta` (non-fatal on read error).
- `cmd/squirebot-server/main.go` — GET/POST/DELETE `/api/v1/characters/{name}/portrait` under `RequireSession`.

## Decisions Made
None beyond the LOCKED plan decisions (D-07 inventory-payload placement, D-08 real DELETE route) and the two plan-checker notes, both honored: (1) `PortraitMeta` is called at the `StructuredInventory` store-access site, not inside the pure `buildStructuredInventory`; (2) the store `updated_at` (TEXT RFC3339) and the audit `now` (int `nowUnix()`) are kept distinct.

## Security controls implemented (the plan's threat_model)
- **T-41-01 (stored-XSS via content_type):** `sniffImageType` — a fixed 3-way magic-byte switch (PNG `89 50 4E 47 0D 0A 1A 0A`, JPEG `FF D8 FF`, WebP `RIFF..WEBP`); SVG + GIF + anything else rejected `400 invalid_image`; content_type set FROM the sniff; serve sets `Content-Type: image/<sniffed>` + `X-Content-Type-Options: nosniff`. NOT the stdlib broad content-type detector.
- **T-41-02 (IDOR write/delete):** `authorizePortraitWriteTx` — assignee OR officer under the tx (WR-04) on POST + DELETE; bank/bot officer-only (charSharedTx flip); stranger -> `ErrNotAuthorized` -> 403; the delete gate runs BEFORE the DELETE.
- **T-41-03 (DoS decode-bomb):** decoded-byte cap `> 256*1024 -> 400 too_large` enforced BEFORE the sniff; the blob lives in a side table so hot reads never pull it.
- **T-41-04 (path-param injection/existence leak):** `{name}` binds only as a `?` in the name->id lookup; unknown -> `ErrCharNotFound`->400 (write) / `ErrPortraitNotFound`->404 (serve); slog carries op+err+byte-count only, never the name or bytes.
- **T-41-05 (repudiation):** every write appends `portrait_set`/`portrait_removed` to `audit_log` with the caller's discord id, in the SAME tx as the blob write.
- **T-41-06 (bytes in hot reads):** the inventory JSON carries only `has_portrait` + `portrait_updated_at`; the bytes stream from the dedicated login-gated GET.

## Web contract (consumed by 41-02)
The web wave reads two additive fields on the `CharacterInventory` payload:
- `has_portrait: boolean` — render the portrait `<img>` in the compacted `.doll` frame when true, else the silhouette fallback.
- `portrait_updated_at: string` — the `?v=` cache-bust key for the `<img src>`.

And three routes (all `RequireSession`):
- `GET /api/v1/characters/{name}/portrait` — the `<img src>` target (raw bytes; the browser sends the cookie credentialed).
- `POST /api/v1/characters/{name}/portrait` — body `{"image_base64": "..."}` (NO client content_type); response `{"character", "updated_at"}` (the web bumps its local `portrait_updated_at` from `updated_at` so the cache-bust changes).
- `DELETE /api/v1/characters/{name}/portrait` — response `{"character"}`; the web needs a 3-line `deleteJSON` helper (a copy of `postJSON` with `method:'DELETE'`) per D-08.

## Deviations from Plan
None - plan executed exactly as written.

## Issues Encountered
- One cosmetic adjustment: Task 2's `sniffImageType` doc comment originally contained the literal string `http.DetectContentType`, which tripped an acceptance grep gate that asserts that string is absent (0 matches). The comment was reworded to "the stdlib content-type detector" — no code change, the switch never called the stdlib function. Committed within the Task 2 commit.

## Known Stubs
None. The `has_portrait` flag is backed by a real `PortraitMeta` store read, and the serve endpoint streams real stored bytes — no placeholder/empty-value data paths were introduced.

## User Setup Required
None - no external service configuration required. The migration applies forward-only on the next backend boot (goose Up), taking prod schema v18->v19. Watcher is UNTOUCHED (no `v*` tag).

## Next Phase Readiness
- **41-02 (web wave)** is unblocked: the `has_portrait`/`portrait_updated_at` flag + the serve/upload/delete routes are live in code, ready for the compacted `InventoryWindow.svelte` frame + the upload/remove control + `api.ts` type additions.
- **Deploy note:** first prod boot of 00019 takes schema v18->v19; deploy is a binary swap + restart (goose on boot). No `WatcherMaxSchemaVersion` change, no `v*` tag (watcher off the read path).

## Self-Check: PASSED

All 7 created source files + the SUMMARY exist on disk; all 3 task commits (`792a56b`, `4635326`, `201f096`) are present in the git log. `go build ./...`, `go vet ./internal/backendsrv/...`, and `go test ./internal/backendsrv/...` are all green; the server binary builds.

---
*Phase: 41-character-paper-doll-compaction-portrait-photo*
*Completed: 2026-07-15*
