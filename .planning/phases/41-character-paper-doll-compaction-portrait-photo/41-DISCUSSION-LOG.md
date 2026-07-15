# Phase 41: Character paper-doll — compaction + portrait photo - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-15
**Phase:** 41-character-paper-doll-compaction-portrait-photo
**Areas discussed:** Portrait storage, Portrait input method, Who can set a portrait, Compaction target (CHARUI-01)

---

## Portrait storage (D-02)

| Option | Description | Selected |
|--------|-------------|----------|
| SQLite BLOB | Image bytes in the DB (a `character_portrait` side table). No new infra/creds, atomic, free nightly R2 backup. Fine at ~120 small images. | ✓ |
| New R2/S3 object store | Scalable, lean DB, but needs a new Go S3 client + creds + public-serve path + its own backup. Overkill for guild scale. | |
| URL-only (external hosting) | Store a link the user hosts elsewhere. Zero storage, but rot + uncontrolled quality + URL/SSRF-injection surface. | |

**User's choice:** SQLite BLOB (recommended default).
**Notes:** Scout confirmed R2 is backup-only (rclone shell, not wired into Go) and there is no reusable object-put path; BLOB avoids a whole new credential surface. Stored in a side table to keep the hot `character` row lean.

---

## Portrait input method (D-03)

| Option | Description | Selected |
|--------|-------------|----------|
| Upload a file, base64-in-JSON | Post the file base64-encoded through the existing JSON-POST write pattern — no new multipart infra. | ✓ |
| Upload a file, multipart/form-data | Classic upload; needs the codebase's first multipart handler. | |
| Preset EQ portrait gallery | Curated race/class art, no upload — but generic, not a personal photo. | |

**User's choice:** File upload, base64-in-JSON (recommended default).
**Notes:** Reuses the `charmeta.go` decode→validate→WithTx→audit shape; keeps the JSON-only-write invariant. Pairs with BLOB storage.

---

## Who can set a portrait (D-05 / D-06)

| Option | Description | Selected |
|--------|-------------|----------|
| Assignee + officers | The character's assignee OR an officer; IDOR-safe, matches character_assignment. Banks/bots (no assignee) → officers only. | ✓ |
| Any signed-in member | Matches today's open class/level metadata. Simplest, but anyone could troll anyone's portrait. | |
| Officers only | Too restrictive for a personalize-your-own-character feature. | |

**User's choice:** Assignee + officers (recommended default).
**Notes:** A personal, abusable image field warrants the tighter gate vs the open charmeta posture. Reuses `IsCharAssignedToTx` + `RequireOfficer`.

---

## Compaction target — CHARUI-01 (D-01)

| Option | Description | Selected |
|--------|-------------|----------|
| Tighten toward in-game feel | Reduce the 24px gaps + padding, shrink the 260px placeholder into a proportioned portrait frame; keep the 3-col structure. Reclaims dead space, low risk, web-only. | ✓ |
| Fuller in-game-window rework | Restructure proportions/columns toward the real EQ window. Bigger change, more risk. | |
| Minimal — just add the photo | Leave layout as-is, only swap the placeholder for the photo. Doesn't deliver the "compact" half. | |

**User's choice:** Tighten toward the in-game feel (recommended default).
**Notes:** Delivers CHARUI-01's dead-space reclaim without a structural teardown.

---

## Claude's Discretion

The user delegated the remaining sub-decisions (all four areas answered with the recommended default — consistent with yolo mode). Locked as sensible defaults in CONTEXT.md:
- Image constraints (D-04): PNG/JPEG/WebP only (SVG excluded as an XSS vector), server-side magic-byte sniff, ≤256KB stored-blob cap, client-side downscale to ~256–512px square-ish.
- Serve shape (D-07): dedicated `GET .../portrait` streaming endpoint + a `has_portrait`/`portrait_updated_at` flag on the roster/inventory payload (bytes never inline); exact payload placement + endpoint verbs left to the planner.
- Removal (D-08): a DELETE/clear path, same gate.
- Exact compaction CSS values + the fallback placeholder art.

## Deferred Ideas

- multipart/form-data upload (only if base64-in-JSON is ever outgrown).
- R2/S3 object store for images (only if SQLite BLOB becomes a real weight problem).
- Preset EQ race/class portrait gallery.
- Portraits on the dense Inventory/Wishlist list rows.
- A crop/zoom/rotate editing UI beyond a simple square downscale.
- Animated/GIF portraits (static formats only).
