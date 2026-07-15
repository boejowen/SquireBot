# Phase 41: Character paper-doll — compaction + portrait photo - Context

**Gathered:** 2026-07-15
**Status:** Ready for planning

<domain>
## Phase Boundary

Tighten the Characters-tab paper-doll toward the in-game inventory-window feel **and** let a player attach an optional portrait photo to each character, shown in the reclaimed portrait area.

- **CHARUI-01** — compact the paper-doll layout (`InventoryWindow.svelte`): reclaim the dead space of the oversized 260px dashed `⚔` placeholder + the loose 24px grid gaps, moving toward the denser in-game inventory-window idiom. Web-only.
- **CHARUI-02** — optional per-character portrait photo: a **new persisted-data piece** (a goose migration for the image reference + a base64-in-JSON upload path + read-API plumbing + the portrait render in the compacted frame).

**In scope:** the paper-doll compaction (web CSS/layout), the portrait storage + upload + serve + render, the assignee/officer permission gate. **Out of scope:** any watcher change (→ **no `v*` tag**), portraits on the dense Inventory/Wishlist list rows, a full EQ-window structural teardown, R2/object-store infra, a cropping/editing UI beyond a simple square fit, animated portraits.

</domain>

<decisions>
## Implementation Decisions

### CHARUI-01 — Paper-doll compaction
- **D-01 (compaction target):** **Tighten toward the in-game feel** — reduce the `.paperdoll` 24px grid gaps + surrounding padding and shrink the oversized 260px dashed `.doll` placeholder into a **proportioned portrait frame**, keeping the current 3-column `equip-col / doll / equip-col` structure (`InventoryWindow.svelte:339-372`). This reclaims the dead space (the goal) at low risk, web-only. Explicitly NOT the "fuller in-game-window rework" (declined — too much visual churn/risk) and NOT "minimal, just add the photo" (declined — must actually deliver the compact half of CHARUI-01).

### CHARUI-02 — Portrait storage
- **D-02 (storage backend):** **SQLite BLOB.** Store the image bytes in the DB in a dedicated `character_portrait` **side table** (`character_id` PK/FK → `character(id)`, `image_blob BLOB`, `content_type TEXT`, `byte_size INTEGER`, `updated_at TEXT`), NOT a column on the hot `character` row — so the frequently-read roster/inventory queries stay lean and the blob loads only on the portrait-serve path. New goose migration (**→ 00019**, extend-only). Backed up for free by the nightly R2 DB snapshot; **no new infra or credentials**. The R2/S3 object store and URL-only options were both declined (overkill / rot + injection risk at guild scale — ~12 guildies × ~10 chars ≈ ~120 small images).

### CHARUI-02 — Portrait input
- **D-03 (input method):** **File upload, base64-in-JSON.** The browser reads the chosen file and POSTs it base64-encoded through the **existing JSON-POST write pattern** (`webadmin` handler, `json.NewDecoder(r.Body)` — the `charmeta.go` model), so **no new multipart/form-data handler** is introduced. Pairs with the BLOB storage (D-02). multipart/form-data and the preset-EQ-gallery options were declined.
- **D-04 (image constraints — bounded discretion):** Accept **PNG / JPEG / WebP only** (**SVG explicitly excluded** — it can carry script). The server **sniffs the actual magic bytes** and sets `content_type` from the sniff, never trusting the client-declared type. Enforce a **stored-blob size cap** (default **≤256 KB** on the decoded bytes; the planner may finalize the exact cap). Recommend **client-side downscale/crop to a square-ish max dimension (~256–512px)** before encoding so the stored blob stays small; reject non-image / oversize input with a 4xx.

### CHARUI-02 — Permissions
- **D-05 (who can set):** The character's **assignee** (`character_assignment.discord_user_id == caller`, `store.IsCharAssignedToTx`) **OR an officer** (`RequireOfficer` / `store.IsOfficer`). IDOR-safe, mirrors the `OfficerAssignTx` override pattern. "Any signed-in member" (the current open `charmeta` posture) and "officers only" were both declined — a personal, abusable image field warrants the assignee-or-officer gate.
- **D-06 (guild banks/bots):** Banks (`is_bank_toon=1`) / bots (`is_guild_bot=1`) have no assignee (the assignment.go D-02 exemption) → **officers only** may set their portrait; a regular member cannot.

### CHARUI-02 — Read-API + serve
- **D-07 (serve + payload — bounded discretion):** A dedicated **`GET /api/v1/characters/{name}/portrait`** streams the blob with the stored `content_type` (**`image/*` only**, `RequireSession`-gated like other reads). The roster (`rosterChar`, `readapi/characters.go:34`) and/or inventory (`CharacterInventory`, `compute/types.go:186`) payloads gain a lightweight **`has_portrait` bool** (or a `portrait_url` string pointing at the serve endpoint) **+ `portrait_updated_at`** for cache-busting — **never the bytes inline**. Web renders the portrait in the compacted frame; the existing placeholder stays the **fallback** when `has_portrait` is false. (Exact payload placement — roster vs inventory vs both — is the planner's call.)
- **D-08 (removal):** A portrait can be removed (revert to the placeholder) via **`DELETE /api/v1/characters/{name}/portrait`** (or an equivalent clear), same assignee-or-officer gate.

### Claude's Discretion
- The exact size cap and whether downscale happens client-side vs server-side; the precise endpoint shape (single upsert POST + DELETE vs one PUT); whether the portrait flag rides the roster payload, the inventory payload, or both; the exact compaction CSS values (gap/padding/frame dimensions); the fallback placeholder art (keep the `⚔` silhouette vs a class/race-derived one).

### Security posture (feeds the planner's `<threat_model>`)
- **Upload:** base64-decode → **magic-byte sniff** (PNG `89 50 4E 47`, JPEG `FF D8 FF`, WebP `RIFF….WEBP`) → reject anything else; **SVG rejected** (script vector). Store + serve `content_type` from the sniff, not the client claim.
- **Size:** enforce the decoded-byte cap server-side (anti-DB-bloat / anti-DoS).
- **IDOR:** assignee-or-officer check on every write/delete (reuse the wantlist/assignment IDOR precedent).
- **Serve:** `Content-Type: image/<sniffed>` + `X-Content-Type-Options: nosniff`; the `{name}` path segment validated; 404-vs-401 discipline preserved.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope + requirements
- `.planning/ROADMAP.md` § "Phase 41" (the checklist line + the v2.6 phase table + the milestone locked-decisions block) — goal, "compact **and** photo," new-migration expectation.
- `.planning/REQUIREMENTS.md` — CHARUI-01 (compaction) + CHARUI-02 (portrait upload: "new per-character image reference + migration + upload path + read-API").

### CHARUI-01 — paper-doll layout (the compaction target)
- `web/src/lib/components/InventoryWindow.svelte:159-162` (the `.doll` placeholder div) + `:339-372` (the `.paperdoll` grid + `.equip-col` + `.doll` CSS) + `:474-483` (existing responsive breakpoints) — the exact layout to tighten.
- `web/src/lib/components/PaperdollSlot.svelte:120` (the 62px tile `<img>` + `onImgError` hide-and-fall-through to the colored tile) — the icon-render + graceful-fallback pattern the portrait `<img>` should mirror.
- `web/src/routes/characters/+page.svelte:206-279` — the master-detail container that embeds `InventoryWindow` (no structural change expected).

### CHARUI-02 — character data model + read-API (the attach points)
- `internal/backendsrv/migrations/00001_init.sql:8-21` — the `character` table (the new `character_portrait` side table FKs to `character(id)`).
- `internal/backendsrv/migrations/00009_character_assignment.sql` + `internal/backendsrv/store/assignment.go` (`IsCharAssignedToTx:119`, `OfficerAssignTx`, `charSharedTx:97`) — the assignee/officer/bank-exemption ownership model (D-05/D-06).
- `internal/backendsrv/migrations/00004_web_auth.sql` (`guild_admins`) + `internal/backendsrv/webauth/session.go:194` (`RequireSession`) / `:217` (`RequireOfficer`) + `store.IsOfficer` — the session identity + role gates.
- `internal/backendsrv/webadmin/charmeta.go:87-150` — the reference JSON-POST write handler (decode → validate → `WithTx` → audit) the base64 upload path (D-03) mirrors.
- `internal/backendsrv/readapi/characters.go:30-103` (`rosterChar:34`) + `internal/backendsrv/readapi/inventory.go:50-92` + `internal/backendsrv/compute/types.go:186-195` (`CharacterInventory`) — where `has_portrait`/`portrait_updated_at` attach (D-07).
- `web/src/lib/api.ts:159-171` (`RosterCharacter`) + `:209-215` (`CharacterInventory`) — the TS mirror.
- `cmd/squirebot-server/main.go:320-364` — route registration (the new portrait GET/POST/DELETE routes register here with the webauth gates).

### Storage / ops (what exists, what's new)
- `deploy/squirebot-backup.sh:1-21` — R2 is **backup-only** (rclone shell cron); confirms there is NO reusable Go object-put path (D-02 rationale).
- `internal/config/config.go:20-39` — config carries no R2 creds (so BLOB avoids a whole new credential surface).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`charmeta.go` JSON-POST handler** (`webadmin/charmeta.go:87-150`) — the exact decode→validate→`WithTx`→audit shape the base64 portrait upload reuses (D-03). No new multipart infra.
- **Ownership gates** — `store.IsCharAssignedToTx` (`assignment.go:119`, already used for wantlist IDOR) + `webauth.RequireOfficer` (`session.go:217`) + `store.IsOfficer` give D-05/D-06 for free.
- **`PaperdollSlot` `<img>` + `onImgError`** (`PaperdollSlot.svelte:93,120`) — the load-error-hides-image-falls-through-to-placeholder pattern the portrait render mirrors for its own fallback.
- **`WithTx` + `audit_log`** — the atomic write + audit trail every existing officer/member mutation uses.

### Established Patterns
- **Extend-only schema** — new `character_portrait` side table + additive `has_portrait`/`portrait_updated_at` JSON fields; nothing renamed/broken. Migration is the milestone's second new-data piece (after P37's 00016).
- **Compute-on-read** — read-API assembles the payload from store rows; the portrait flag is a cheap additive read, the bytes stream from a dedicated endpoint (never inline in the roster/inventory JSON).
- **JSON-only write endpoints** — the codebase has zero multipart handlers; D-03's base64-in-JSON keeps that invariant.

### Integration Points
- **Backend:** new `store/portrait.go` (upsert/get/delete blob + `IsCharAssignedTo`/officer gate) → new `webadmin` upload/delete handlers + a `readapi` serve handler → additive fields on `rosterChar`/`CharacterInventory` → 3 routes in `main.go`.
- **Web:** the compacted `InventoryWindow.svelte` layout (CHARUI-01) + a portrait `<img>` in the frame with the `has_portrait`/`portrait_updated_at` cache-bust + an upload control (assignee/officer-gated) + `api.ts` type additions.
- **Tests:** Go store/handler table tests (blob roundtrip, magic-byte reject, size cap, IDOR assignee-vs-officer-vs-stranger, bank/bot officer-only); web vitest is DOM-blind → the compaction + portrait render is a **browser-smoke checkpoint** (deploy-then-smoke, the established discipline). Expect a `/gsd-ui-phase 41` gate before planning (UI: yes).

</code_context>

<specifics>
## Specific Ideas

- The portrait sits in the **reclaimed `.doll` frame** — CHARUI-01's compaction and CHARUI-02's photo are two halves of the same visual: the dead placeholder becomes a real, proportioned portrait (or the fallback silhouette when unset).
- **SVG is excluded** from allowed formats on purpose (it is an XSS vector); PNG/JPEG/WebP only, content-type set from a server-side magic-byte sniff.
- Base64-in-JSON is chosen specifically to **avoid introducing the codebase's first multipart handler** — it reuses the proven `charmeta` write shape.
- Keep the portrait bytes **out of the roster/inventory JSON** — a `has_portrait` flag + a dedicated streaming endpoint keeps the hot reads cheap.

</specifics>

<deferred>
## Deferred Ideas

- **multipart/form-data upload** — only if portraits ever need to exceed the base64-in-JSON comfort zone (bigger media). Not now.
- **R2/S3 object store for images** — revisit only if SQLite BLOB storage becomes a real weight problem (far beyond guild scale). Not now.
- **Preset EQ race/class portrait gallery** — a curated fallback set; nice-to-have, not the personal-photo feature this phase ships.
- **Portraits on the dense Inventory/Wishlist list rows** — out of scope; portrait is the paper-doll frame only.
- **A crop/zoom/rotate editing UI** — beyond a simple square client-side downscale; a future polish.
- **Animated/GIF portraits** — excluded (static image formats only).

None of the above were scope creep into other domains — they are explicitly-narrowed alternatives within CHARUI-02, parked here so the choices are not re-litigated.

</deferred>

---

*Phase: 41-character-paper-doll-compaction-portrait-photo*
*Context gathered: 2026-07-15*
