package webadmin

// portrait.go is the per-character portrait upload + delete write backend (Phase 41 /
// CHARUI-02, plan 41-01). It mirrors CharMetaSetHandler's decode → validate → withTx → audit
// JSON-POST shape (charmeta.go:87-150) — base64-in-JSON (D-03) so the codebase keeps its
// zero-multipart-handler invariant — but swaps the value-set check for the D-04 image
// validation pipeline:
//
//   base64.StdEncoding.DecodeString → SIZE CAP FIRST (len(decoded) > 256KB → 400 too_large,
//   reject-early/anti-DoS) → MAGIC-BYTE SNIFF (a fixed 3-way switch: PNG/JPEG/WebP only,
//   SVG+GIF+anything else rejected 400 invalid_image; content_type is set FROM the sniff,
//   NEVER the client claim) → store the bytes + sniffed type.
//
// AUTHORIZATION lives in the store gate (store.SetPortraitTx/DeletePortraitTx run the
// assignee-OR-officer check UNDER the tx, WR-04) — this handler is LOGIN-ONLY at the route
// (RequireSession); it records the writer's discord id for the audit row but does NOT itself
// consult officer/assignee status. The blob write + its portrait_set/portrait_removed audit
// row compose in ONE withTx (BEGIN IMMEDIATE) so they land atomically.
//
// TIMESTAMP note (plan-checker): the store column updated_at is TEXT (RFC3339), while the
// audit-log `at` stays the int nowUnix() — the two are DISTINCT and never conflated. slog
// carries op + err ONLY, never the char name or the blob bytes (V7).

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// maxPortraitBytes is the D-04 decoded-blob size cap (256KB). Enforced BEFORE the sniff and
// the store write (reject-early, anti-DB-bloat / anti-decode-bomb).
const maxPortraitBytes = 256 * 1024

// maxPortraitReqBytes bounds the RAW request body (WR-01) so an oversized base64 payload is
// rejected AS IT IS READ (http.MaxBytesReader) rather than after being fully buffered + decoded
// — the reject-early/anti-DoS the decoded-cap comment promised. 384KB comfortably fits a 256KB
// image's base64 (~342KB) plus the tiny {"image_base64":"…"} JSON envelope; anything larger
// trips before the decode allocates. The decoded-byte cap (maxPortraitBytes) remains the
// authoritative image-size limit.
const maxPortraitReqBytes = 384 * 1024

// portraitReq is the base64 upload body. There is NO client content_type field (D-04) — the
// server sniffs the actual magic bytes and sets the stored type from the sniff.
type portraitReq struct {
	ImageBase64 string `json:"image_base64"`
}

// sniffImageType returns the image/* content-type for b's leading magic bytes, or ("", false)
// for anything not PNG/JPEG/WebP. A fixed 3-way switch — deliberately NOT the stdlib
// content-type detector, which is too broad and admits SVG/GIF (the XSS/animated vectors D-04
// excludes). PNG: 89 50 4E 47 0D
// 0A 1A 0A; JPEG: FF D8 FF; WebP: "RIFF"...."WEBP".
func sniffImageType(b []byte) (string, bool) {
	switch {
	case len(b) >= 8 && bytes.HasPrefix(b, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}):
		return "image/png", true
	case len(b) >= 3 && bytes.HasPrefix(b, []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg", true
	case len(b) >= 12 && bytes.HasPrefix(b, []byte("RIFF")) && bytes.Equal(b[8:12], []byte("WEBP")):
		return "image/webp", true
	default:
		return "", false // SVG, GIF, anything else → rejected
	}
}

// PortraitSetHandler (POST) uploads a base64 PNG/JPEG/WebP portrait for the {name} character.
// Login-only at the route (RequireSession); the assignee-OR-officer gate is in the store tx.
// Decode → 256KB cap → magic-byte sniff → store (sniffed type) + audit "portrait_set", all in
// one tx. Error mapping (charmeta.go): store.ErrNotAuthorized → 403 not_authorized;
// store.ErrCharNotFound → 400 invalid_input; else 500 internal.
func PortraitSetHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		name := r.PathValue("name")

		// WR-01: bound the raw body BEFORE decode so an oversized payload can't force a large
		// buffer/allocation ahead of the decoded-byte cap — an over-limit body trips here as
		// it is read, not after being fully buffered.
		r.Body = http.MaxBytesReader(w, r.Body, maxPortraitReqBytes)

		var req portraitReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			// An over-limit body surfaces as *http.MaxBytesError → the too_large client copy;
			// any other decode failure is malformed input.
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				writeJSONError(w, http.StatusBadRequest, "too_large")
				return
			}
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		if req.ImageBase64 == "" {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		decoded, err := base64.StdEncoding.DecodeString(req.ImageBase64)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		// Size cap FIRST (reject-early, anti-DoS) — before the sniff and any store touch.
		if len(decoded) > maxPortraitBytes {
			writeJSONError(w, http.StatusBadRequest, "too_large")
			return
		}
		// Magic-byte sniff — PNG/JPEG/WebP only; SVG/GIF/anything else → invalid_image.
		ct, ok := sniffImageType(decoded)
		if !ok {
			writeJSONError(w, http.StatusBadRequest, "invalid_image")
			return
		}

		// The acting identity is recorded for audit/accountability; the AUTHORIZATION is in
		// the store gate (assignee OR officer, under the tx), NOT here.
		writer := caller(ctx)
		// The store updated_at column is TEXT ISO (distinct from the audit int nowUnix()).
		nowISO := time.Now().UTC().Format(time.RFC3339)

		err = withTx(ctx, db, func(tx *sql.Tx) error {
			if e := store.SetPortraitTx(ctx, tx, name, decoded, ct, writer, nowISO); e != nil {
				return e
			}
			return AppendAuditTx(ctx, tx, "portrait_set", writer, map[string]any{
				"character": name,
			}, nowUnix())
		})
		if err != nil {
			mapPortraitErr(w, err, "portrait_set")
			return
		}

		// The web bumps its local portrait_updated_at from this so the ?v= cache-bust changes.
		writeJSON(w, map[string]any{"character": name, "updated_at": nowISO})
	}
}

// PortraitDeleteHandler (DELETE) removes the {name} character's portrait (D-08). Login-only at
// the route; the same assignee-OR-officer gate in the store tx (runs BEFORE the delete). Audits
// "portrait_removed". Idempotent (a portrait-less char is a no-op success). Same error mapping.
func PortraitDeleteHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		name := r.PathValue("name")

		writer := caller(ctx)
		err := withTx(ctx, db, func(tx *sql.Tx) error {
			if e := store.DeletePortraitTx(ctx, tx, name, writer); e != nil {
				return e
			}
			return AppendAuditTx(ctx, tx, "portrait_removed", writer, map[string]any{
				"character": name,
			}, nowUnix())
		})
		if err != nil {
			mapPortraitErr(w, err, "portrait_removed")
			return
		}

		writeJSON(w, map[string]any{"character": name})
	}
}

// mapPortraitErr maps the store's typed errors to the HTTP codes the frontend routes
// (charmeta.go:121-130 shape): ErrNotAuthorized → 403, ErrCharNotFound → 400 invalid_input,
// else 500 internal. A failed write writes no audit row (the tx rolled back). V7: slog carries
// op + err only, never the char name or bytes.
func mapPortraitErr(w http.ResponseWriter, err error, op string) {
	switch {
	case errors.Is(err, store.ErrNotAuthorized):
		writeJSONError(w, http.StatusForbidden, "not_authorized")
	case errors.Is(err, store.ErrCharNotFound):
		writeJSONError(w, http.StatusBadRequest, "invalid_input")
	default:
		slog.Error("portrait write failed", "op", op, "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal")
	}
}
