package webadmin

// eviction.go is the eviction + restore + archive backend (ADMIN-04 / D-09/D-10)
// — the SQLite/HTTP port of v1's eviction sidebar (showEvictionSidebar.ts) and
// the weekly archive job (weeklyEvictionArchive.ts). The route layer wraps every
// handler in webauth.RequireOfficer (the cheap request-time gate); the mutating
// handlers ALSO re-check officer status INSIDE their write tx via
// store.IsOfficerTx — EvictOwnerTx / RestoreOwnerTx do NOT self-authorize (unlike
// the officer-mgmt mutators), so this handler owns the in-tx re-check. That in-tx
// re-check, on the BEGIN IMMEDIATE write-locked tx (the store DSN sets
// _txlock=immediate, so db.BeginTx takes the write lock up front), is the v1 WR-04
// TOCTOU close: a just-removed officer cannot land one final evict/restore.
//
// Eviction is one app-controlled action (D-10): store.EvictOwnerTx cascades
// is_removed=1 across the owner's live characters, stamps a 30-day grace_until,
// AND revokes the owner's guild code (guild_code.disabled_at) — all in the one tx.
// Restore (reversible-during-grace, D-10) un-sets is_removed + clears grace_until
// for the owner's still-in-grace characters AND re-MINTS a fresh guild code (the
// burned one stays disabled — codes are not un-revoked); a restore on an
// archived/past-grace owner is refused with 409 grace_expired (archived data is
// never silently revived — W-2). The DAILY archive job (RegisterEvictionArchive,
// wired into the in-process scheduler) hard-archives past-grace data idempotently
// (W-3).
//
// Owner-floor data protection (D-09): a peer officer cannot evict the maintainer's
// own guildie data. Absent a real owner↔discord FK in the schema (owners are keyed
// by label; web_users by Discord snowflake; no column links them), the floor's
// protected owner is resolved by owner.label == the floor web_user's username —
// the documented best-available textual bridge. If the caller is not the floor and
// the target resolves to that owner, the evict is refused with
// owner_floor_protected BEFORE any write (mirroring v1's check-before-write).

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/boejowen/SquireBot/internal/backendsrv/auth"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// EvictableListHandler (GET) returns the owners with >=1 live character — the
// eviction picker source. Officer-only at the route.
func EvictableListHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		owners, err := store.ListEvictableOwners(r.Context(), db)
		if err != nil {
			slog.Error("evictable list failed", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}
		if owners == nil {
			owners = []store.EvictableOwner{}
		}
		writeJSON(w, owners)
	}
}

// RestorableListHandler (GET) returns the owners with >=1 character still in grace
// (evicted, grace not yet expired, not archived) — the eviction-RESTORE picker
// source, the inverse of EvictableListHandler. Officer-only at the route.
func RestorableListHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		owners, err := store.ListRestorableOwners(r.Context(), db, nowUnix())
		if err != nil {
			slog.Error("restorable list failed", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}
		if owners == nil {
			owners = []store.RestorableOwner{}
		}
		writeJSON(w, owners)
	}
}

// EvictionPreviewHandler (GET ?owner_id=N) returns the owner's live character
// names + a preview grace_until (now + 30d) — what the confirm-before-commit UI
// lists. Officer-only at the route. An empty cascade yields characters:[] so the
// UI shows "No characters found for this guildie."
func EvictionPreviewHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		ownerID, ok := parseOwnerIDQuery(r)
		if !ok {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		names, err := store.PreviewEviction(ctx, db, ownerID)
		if err != nil {
			slog.Error("eviction preview failed", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}
		if names == nil {
			names = []string{}
		}
		graceUntil := nowUnix() + store.EvictionGraceSeconds
		writeJSON(w, map[string]any{
			"owner_id":    ownerID,
			"characters":  names,
			"grace_until": graceUntil,
		})
	}
}

// ownerReq is the {owner_id} body shared by evict/restore.
type ownerReq struct {
	OwnerID int64 `json:"owner_id"`
}

// EvictHandler (POST) evicts a whole guildie (D-09/D-10). Officer-only at the
// route; owner-floor data protection BEFORE any write; authorize-under-tx
// (store.IsOfficerTx re-check inside the BEGIN IMMEDIATE tx — WR-04); cascade +
// code-revoke + grace via store.EvictOwnerTx; audit "eviction"; return
// {removed_count, grace_until}.
func EvictHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		var req ownerReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.OwnerID <= 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		callerID := caller(ctx)
		now := nowUnix()

		// Owner-floor data protection (D-09), BEFORE any write: a peer cannot evict
		// the maintainer's own guildie data. Resolve the floor's protected owner via
		// owner.label == the floor web_user's username (documented bridge).
		protected, err := callerMayNotEvictFloor(ctx, db, req.OwnerID, callerID)
		if err != nil {
			slog.Error("evict floor-protection check failed", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}
		if protected {
			writeJSONError(w, http.StatusForbidden, "owner_floor_protected")
			return
		}

		var removedCount int
		var graceUntil int64
		// ONE tx (BEGIN IMMEDIATE via the _txlock=immediate DSN): in-tx officer
		// re-check (WR-04) → EvictOwnerTx → audit, committed together.
		err = withTx(ctx, db, func(tx *sql.Tx) error {
			okOfficer, e := store.IsOfficerTx(ctx, tx, callerID)
			if e != nil {
				return e
			}
			if !okOfficer {
				return store.ErrNotAuthorized // just-removed officer ⇒ rollback, nothing written
			}
			removedCount, graceUntil, e = store.EvictOwnerTx(ctx, tx, req.OwnerID, now)
			if e != nil {
				return e
			}
			return AppendAuditTx(ctx, tx, "eviction", callerID, map[string]any{
				"owner_id":      req.OwnerID,
				"removed_count": removedCount,
			}, now)
		})
		if err != nil {
			mapEvictionErr(w, err, "eviction")
			return
		}

		writeJSON(w, map[string]any{"removed_count": removedCount, "grace_until": graceUntil})
	}
}

// RestoreHandler (POST) reverses an eviction DURING grace (D-10, W-2). Officer-only
// at the route; authorize-under-tx (WR-04). store.RestoreOwnerTx un-sets
// is_removed + clears grace_until for the owner's still-in-grace, non-archived
// characters. If restored == 0 (already archived / past grace / never evicted) →
// 409 grace_expired (archived data is NEVER silently revived). Otherwise re-MINT a
// fresh guild code for the owner (the old one stays disabled — codes are not
// un-revoked, so restore necessarily issues a NEW one); audit "eviction_restore".
//
// WR-01 (post-commit mint is recoverable, not a 500): the owner label is resolved
// BEFORE the tx (it cannot change during a restore), so the ONLY fallible step
// after the commit is auth.MintCode itself. If that mint fails, the restore has
// already committed (the characters are live again) — returning a generic 500
// would leave the guildie restored-but-codeless with the officer believing nothing
// happened. Instead we log at error with owner_id and return a 200 success-with-
// warning shape {restored_count, new_code_issued:false, code_mint_failed:true} so
// the caller can tell the officer the restore succeeded but a code must be
// re-issued out-of-band. OPERATOR RECOVERY: run
//
//	squirebot-server mint-code --owner <label>
//
// to issue the replacement code (its plaintext prints to the server stdout/journald).
//
// WR-02 (the minted code is NOT web-deliverable): auth.MintCode prints the one-time
// plaintext to the server's stdout (journald) ONLY — never the HTTP response (V7).
// So new_code_issued:true means "a fresh code now EXISTS", not "the officer has it";
// the maintainer must read it from journald (or re-run mint-code) and hand it off
// out-of-band. The success copy must not imply the officer holds a deliverable code.
func RestoreHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		var req ownerReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.OwnerID <= 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		callerID := caller(ctx)
		now := nowUnix()

		// WR-01: resolve the owner label BEFORE the tx so the only post-commit
		// fallible step is the mint. A missing owner here means a bad request (it
		// would restore 0 and 409 anyway); fail fast with nothing committed.
		ownerLabel, err := ownerLabelOf(ctx, db, req.OwnerID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeJSONError(w, http.StatusBadRequest, "invalid_input")
				return
			}
			slog.Error("restore: resolve owner label failed", "owner_id", req.OwnerID, "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}

		// errGraceExpired is the in-tx sentinel signalling "nothing was in grace" so
		// the tx rolls back (no audit row) and the handler maps it to 409.
		var restoredCount int
		err = withTx(ctx, db, func(tx *sql.Tx) error {
			okOfficer, e := store.IsOfficerTx(ctx, tx, callerID)
			if e != nil {
				return e
			}
			if !okOfficer {
				return store.ErrNotAuthorized
			}
			restoredCount, e = store.RestoreOwnerTx(ctx, tx, req.OwnerID, now)
			if e != nil {
				return e
			}
			if restoredCount == 0 {
				// Nothing in grace to restore (archived / past grace / never evicted) →
				// refuse. Rolling back leaves zero side effects (no audit, no mint).
				return errGraceExpired
			}
			return AppendAuditTx(ctx, tx, "eviction_restore", callerID, map[string]any{
				"owner_id":       req.OwnerID,
				"restored_count": restoredCount,
			}, now)
		})
		if err != nil {
			mapEvictionErr(w, err, "eviction_restore")
			return
		}

		// Re-mint a FRESH guild code for the owner (D-10). MintCode runs on the bare
		// *sql.DB (it manages its own INSERT) so it is necessarily AFTER the restore
		// tx commits — the restore (un-set is_removed) is the load-bearing reversal;
		// the re-mint is the follow-on that lets the watcher resume. The old code
		// stays disabled (codes are not un-revoked); this issues a NEW one. We never
		// echo the minted plaintext (V7/WR-02 — MintCode prints it to the server's
		// stdout/journald once, as the CLI does).
		//
		// WR-01: a mint failure here is NOT a 500. The restore already committed (the
		// characters are live again), so a generic 500 would strand the guildie
		// restored-but-codeless while the officer believes nothing happened. Surface
		// it as a success-with-warning so the caller can tell the officer to re-issue
		// the code out-of-band (squirebot-server mint-code --owner <label>).
		if _, err := auth.MintCode(db, ownerLabel); err != nil {
			slog.Error("restore: re-mint guild code failed AFTER restore committed; the guildie is restored but has NO active code — re-issue via `mint-code --owner <label>`",
				"owner_id", req.OwnerID, "owner_label", ownerLabel, "err", err)
			writeJSON(w, map[string]any{
				"restored_count":   restoredCount,
				"new_code_issued":  false,
				"code_mint_failed": true,
			})
			return
		}

		writeJSON(w, map[string]any{"restored_count": restoredCount, "new_code_issued": true})
	}
}

// errGraceExpired is the sentinel the restore tx returns when there is nothing in
// grace to restore — mapped to 409 grace_expired (W-2: archived data is never
// revived).
var errGraceExpired = errors.New("grace_expired")

// mapEvictionErr maps the eviction/restore typed errors to the exact HTTP codes
// the frontend routes; anything else is a 500. A failed write writes no audit row
// (the tx rolled back).
func mapEvictionErr(w http.ResponseWriter, err error, op string) {
	switch {
	case errors.Is(err, store.ErrNotAuthorized):
		writeJSONError(w, http.StatusForbidden, "not_authorized")
	case errors.Is(err, store.ErrCannotEvictSentinel):
		// OWN-02: the guild sentinel is NEVER evictable/restorable. Refuse the
		// destructive write even when an untrusted owner_id targets it directly —
		// 403 mirrors the not_authorized guard convention for this mapper.
		writeJSONError(w, http.StatusForbidden, "cannot_evict_sentinel")
	case errors.Is(err, errGraceExpired):
		writeJSONError(w, http.StatusConflict, "grace_expired")
	default:
		slog.Error("eviction write failed", "op", op, "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal")
	}
}

// callerMayNotEvictFloor reports whether the eviction must be refused by the D-09
// owner-floor data protection: true iff the target owner is the floor's own owner
// (owner.label == the floor web_user's username) AND the caller is not the floor
// itself. The floor may evict its own data (self-action allowed, mirroring v1's
// self-removal rule); a peer may not. An unset floor (no maintainer seeded) yields
// false (nothing to protect).
func callerMayNotEvictFloor(ctx context.Context, db *sql.DB, targetOwnerID int64, callerID string) (bool, error) {
	floor, err := store.GetOwnerFloor(ctx, db)
	if err != nil {
		return false, err
	}
	if floor == "" || callerID == floor {
		return false, nil // nothing seeded, or the floor acting on its own data
	}
	// D-05 (LINK-02 payoff): prefer the real owner.discord_user_id FK. Once the
	// maintainer has self-minted once (stamping their owner.discord_user_id via the
	// Phase-17 resolve-or-create), the floor's protected owner resolves DIRECTLY by
	// the FK — closing the WR-05 fail-open for that linked owner. Only when the
	// floor's owner is not-yet-linked (no FK row) do we fall back to the legacy
	// TRIM(label) COLLATE NOCASE username bridge below.
	var fkOwnerID int64
	err = db.QueryRowContext(ctx,
		`SELECT id FROM owner WHERE discord_user_id = ?`, floor,
	).Scan(&fkOwnerID)
	switch {
	case err == nil:
		// The floor's owner is linked — the FK is authoritative (no fail-open).
		return targetOwnerID == fkOwnerID, nil
	case !errors.Is(err, sql.ErrNoRows):
		return false, err
	}
	// Not-yet-linked floor owner → fall back to the legacy label bridge (WR-05).

	// Resolve the floor's username (the owner-label bridge). A missing web_user or
	// an empty username ⇒ the bridge has nothing to match on. This is the WR-05
	// fail-OPEN risk: if we cannot resolve which owner the floor protects, a peer
	// would be able to evict the maintainer's data. We CANNOT fully close this in
	// phase (there is no owner↔discord FK — the longer-term fix is an explicit
	// owner_floor_owner_id seeded by set-owner-floor), so we (a) harden the textual
	// match (trim + case-insensitive, since Discord usernames and watcher-supplied
	// owner.labels drift in case/whitespace) and (b) emit a LOUD warning whenever a
	// floor is seeded but the protection cannot be resolved to an owner row — so the
	// operator knows the D-09 guarantee is inert rather than silently failing open.
	var floorUsername string
	err = db.QueryRowContext(ctx,
		`SELECT username FROM web_user WHERE discord_user_id = ?`, floor,
	).Scan(&floorUsername)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	floorUsername = strings.TrimSpace(floorUsername)
	if errors.Is(err, sql.ErrNoRows) || floorUsername == "" {
		// The floor is seeded but has no display username yet (e.g. it never logged
		// in, so username is still the snowflake placeholder, or the row is missing).
		// Protection is inert — say so loudly (WR-05).
		slog.Warn("owner-floor protection INERT: floor seeded but its web_user has no username to bridge to an owner.label; a peer can evict the maintainer's data until the floor logs in or an explicit owner link is added",
			"floor_discord_user_id", floor)
		return false, nil
	}
	// Match owner.label to the floor username, case-insensitively and trimmed (the
	// best-available textual bridge — WR-05 hardening). TRIM() both sides in SQL so
	// stored leading/trailing whitespace on either column does not defeat the match.
	var floorOwnerID int64
	err = db.QueryRowContext(ctx,
		`SELECT id FROM owner WHERE TRIM(label) = TRIM(?) COLLATE NOCASE`, floorUsername,
	).Scan(&floorOwnerID)
	if errors.Is(err, sql.ErrNoRows) {
		// Floor seeded + has a username, but no owner.label matches it ⇒ the
		// protection points at no data. Loud warning so the operator can reconcile
		// the floor's Discord username with the watcher's owner label (WR-05).
		slog.Warn("owner-floor protection INERT: no owner.label matches the floor's username; the maintainer's guildie data is NOT floor-protected against peer eviction",
			"floor_discord_user_id", floor, "floor_username", floorUsername)
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return targetOwnerID == floorOwnerID, nil
}

// ownerLabelOf resolves an owner's label (for the restore re-mint). A missing
// owner is an error (a restore of a non-existent owner would have restored 0 and
// returned 409 already, so reaching here with a missing owner is unexpected).
func ownerLabelOf(ctx context.Context, db *sql.DB, ownerID int64) (string, error) {
	var label string
	if err := db.QueryRowContext(ctx,
		`SELECT label FROM owner WHERE id = ?`, ownerID,
	).Scan(&label); err != nil {
		return "", err
	}
	return label, nil
}

// parseOwnerIDQuery reads ?owner_id=N as a positive int64.
//
// WR-04: strconv.ParseInt (not a hand-rolled id=id*10+digit accumulator) so an
// oversized digit string is REJECTED rather than silently wrapping int64 — a
// wrap to a positive, in-range value would otherwise be accepted as a legitimate
// owner id on a destructive endpoint. ParseInt returns ErrRange on overflow and
// ErrSyntax on any non-digit/sign, both of which fail the parse here; the >0
// check then rejects zero and negatives.
func parseOwnerIDQuery(r *http.Request) (int64, bool) {
	raw := r.URL.Query().Get("owner_id")
	if raw == "" {
		return 0, false
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}
