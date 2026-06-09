package store

// assignment.go is the Phase 26 character→user assignment data layer (ASSIGN-01..06,
// 26-01). It sits BESIDE the untouched character.owner_id upload provenance (D-03):
// owner_id stays the watcher-binding column; this layer answers "which characters are
// mine" via the character_assignment table (00009) keyed on discord_user_id (the
// PERSON / web_user / wantlist+notify identity, NOT owner_id).
//
// Invariants enforced here (the schema + these mutators are the truth, the API plan
// 26-02 is pure glue):
//   - SINGLE ASSIGNEE (D-01): character_assignment.character_id is the PRIMARY KEY, so
//     two-members-one-char is structurally impossible. A claim of an unassigned char is
//     a plain INSERT (pre-checked for an existing row → ErrCharAlreadyAssigned); an
//     officer reassign is INSERT … ON CONFLICT(character_id) DO UPDATE (override, D-09).
//   - SHARED-CHAR EXEMPTION (D-02, bidirectional — Pitfall 6): a guild bank
//     (is_bank_toon=1) or guild bot (is_guild_bot=1) char has NO assignee and is not
//     claimable. ClaimCharTx/OfficerAssignTx reject such a char (ErrCharShared);
//     DesignateCharTx, in the SAME tx, clears any assignment + denies pending requests.
//   - CONTESTED-CLAIM QUEUE (D-07): RequestTx files a pending assignment_request; the
//     partial-unique pending index stops a single member double-filing
//     (→ ErrDuplicateRequest). ApproveRequestTx upserts the assignment to the requester
//     AND denies ALL OTHER pending requests for that char in the same tx (Pitfall 3,
//     double-approval defense).
//   - OWNER-SCOPED SILENT-NO-OP (IDOR defense — Pitfall 1/5): ReleaseCharTx/
//     CancelRequestTx scope `AND discord_user_id=caller` / `AND requester=caller`; a
//     foreign-row mutation affects 0 rows → returns false, never an error/leak.
//   - AUTHORIZE-UNDER-TRANSACTION (WR-04 TOCTOU): every officer mutator
//     (OfficerAssignTx/RemoveAssignTx/ApproveRequestTx/DenyRequestTx/DesignateCharTx)
//     calls isOfficerTx(ctx, tx, callerID) as its FIRST statement → ErrNotAuthorized
//     when !ok, so a just-demoted officer cannot land a final mutation (admins.go
//     precedent).
//
// Parameterized ? placeholders ONLY (V5). Audit (ASSIGN-06) is composed by the handler
// (26-02) in the same withTx, NOT here. All timestamps come from the passed now int64.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	// NAMED import so RequestTx can inspect the concrete *sqlite.Error for its
	// extended result code (the partial-unique pending conflict → ErrDuplicateRequest),
	// mirroring wantlist.go's AddWantTx. NO new dependency — modernc is already required.
	sqlite "modernc.org/sqlite"
)

// Typed sentinels mirroring charmeta.go:29 (ErrCharNotFound) — the handler (26-02)
// errors.Is these to map HTTP codes (ErrCharAlreadyAssigned/ErrCharShared → 409,
// ErrNotAuthorized → 403). Officer-gate failures reuse the EXISTING store.ErrNotAuthorized
// (admins.go) — do NOT redefine it here.
var (
	// ErrCharAlreadyAssigned: ClaimCharTx on a char that already has an assignee.
	ErrCharAlreadyAssigned = errors.New("char_already_assigned")
	// ErrCharShared: claim/assign of a guild bank/bot char (D-02 exemption).
	ErrCharShared = errors.New("char_shared")
	// ErrCharNotContested: RequestTx on a char that is NOT contested — i.e. UNASSIGNED
	// (the caller should /claim it, not /request it) or assigned to the CALLER themselves
	// (they already hold it). D-07 scopes a request to a char already assigned to ANOTHER
	// member; the non-contested cases are pure queue noise and are rejected here.
	ErrCharNotContested = errors.New("char_not_contested")
	// ErrDuplicateRequest: a second pending request for the same (char, requester)
	// collides on the partial-unique pending index.
	ErrDuplicateRequest = errors.New("duplicate_request")
	// ErrCharNotAssigned: the IDOR guard for the character-tagged wantlist (Phase 28,
	// T-28-01). A member tried to tag a want to a character that is NOT assigned to them.
	// IsCharAssignedToTx reports the (false, nil) condition; the handler (28-02) maps this
	// sentinel to HTTP 403 when it decides to reject the tag.
	ErrCharNotAssigned = errors.New("char_not_assigned")
)

// Assignment is one assigned character joined to its identity (ListMyAssignments /
// ListAllAssignments). snake_case JSON tags — crosses the API boundary in 26-02.
type Assignment struct {
	CharacterID   int64  `json:"character_id"`
	Name          string `json:"name"`
	DiscordUserID string `json:"discord_user_id"`
	AssignedAt    int64  `json:"assigned_at"`
	AssignedBy    string `json:"assigned_by"`
}

// PendingRequest is one pending assignment_request joined to the contested character's
// name (ListPendingRequests — the officer queue). snake_case JSON tags.
type PendingRequest struct {
	ID              int64   `json:"id"`
	CharacterID     int64   `json:"character_id"`
	CharacterName   string  `json:"character_name"`
	Requester       string  `json:"requester"`
	CurrentAssignee *string `json:"current_assignee"`
	CreatedAt       int64   `json:"created_at"`
}

// charSharedTx reports whether characterID is a guild bank (is_bank_toon=1) OR guild
// bot (is_guild_bot=1) on the tx snapshot — the D-02 exemption gate. A missing char is
// reported as not-shared (false); the assignment write's FK / the caller's own checks
// surface a bad id.
func charSharedTx(ctx context.Context, tx *sql.Tx, characterID int64) (bool, error) {
	var isBank, isBot int
	err := tx.QueryRowContext(ctx,
		`SELECT is_bank_toon, is_guild_bot FROM character WHERE id = ?`, characterID,
	).Scan(&isBank, &isBot)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("check char shared (character_id=%d): %w", characterID, err)
	}
	return isBank == 1 || isBot == 1, nil
}

// IsCharAssignedToTx is the character-tagged-wantlist IDOR guard (Phase 28, T-28-01): it
// reports whether characterID is assigned to callerID on the tx snapshot. The handler
// (28-02) calls it inside its withTx BEFORE inserting a tagged want, so a member can only
// tag a want to a character they actually hold — a `character_id` in the request body is
// untrusted and authorized HERE, not in the handler glue. It mirrors charSharedTx exactly:
// a missing/foreign assignment → (false, nil) (NOT an error — never leak existence); a real
// (characterID, callerID) row → (true, nil); any other error is %w-wrapped. *sql.Tx (not
// *sql.DB) so it runs under the same transaction as the subsequent insert (TOCTOU-safe).
func IsCharAssignedToTx(ctx context.Context, tx *sql.Tx, characterID int64, callerID string) (bool, error) {
	var one int
	err := tx.QueryRowContext(ctx,
		`SELECT 1 FROM character_assignment WHERE character_id = ? AND discord_user_id = ?`,
		characterID, callerID,
	).Scan(&one)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("check char assigned (character_id=%d): %w", characterID, err)
	}
	return true, nil
}

// ClaimCharTx self-claims an UNASSIGNED, non-shared char for callerID (D-06). It
// rejects a guild bank/bot char (ErrCharShared, D-02) and an already-assigned char
// (ErrCharAlreadyAssigned). assigned_by='self'. The character_id PK is the single-
// assignee guarantee; the pre-check turns the would-be conflict into a typed error.
func ClaimCharTx(ctx context.Context, tx *sql.Tx, characterID int64, callerID string, now int64) error {
	shared, err := charSharedTx(ctx, tx, characterID)
	if err != nil {
		return err
	}
	if shared {
		return ErrCharShared
	}
	var existing string
	err = tx.QueryRowContext(ctx,
		`SELECT discord_user_id FROM character_assignment WHERE character_id = ?`, characterID,
	).Scan(&existing)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// unassigned — fall through to the INSERT
	case err != nil:
		return fmt.Errorf("check existing assignment (character_id=%d): %w", characterID, err)
	default:
		return ErrCharAlreadyAssigned
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO character_assignment (character_id, discord_user_id, assigned_at, assigned_by)
		 VALUES (?, ?, ?, 'self')`,
		characterID, callerID, now,
	); err != nil {
		return fmt.Errorf("claim char (character_id=%d, caller=%s): %w", characterID, callerID, err)
	}
	return nil
}

// ReleaseCharTx owner-scoped DELETE: removes callerID's own assignment of characterID
// (D-08). Scoped `AND discord_user_id=caller` so a foreign-row release affects 0 rows →
// returns (false, nil): a silent IDOR no-op that never leaks the row's existence
// (the wantlist RemoveOwnWantTx precedent).
func ReleaseCharTx(ctx context.Context, tx *sql.Tx, characterID int64, callerID string) (removed bool, err error) {
	res, err := tx.ExecContext(ctx,
		`DELETE FROM character_assignment WHERE character_id = ? AND discord_user_id = ?`,
		characterID, callerID,
	)
	if err != nil {
		return false, fmt.Errorf("release char (character_id=%d, caller=%s): %w", characterID, callerID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("release char rows-affected (character_id=%d): %w", characterID, err)
	}
	return n > 0, nil
}

// RequestTx files a pending assignment_request for a contested char (D-07: a char
// already assigned to someone else). current_assignee snapshots who held it at request
// time (nullable). A second pending request from the same requester for the same char
// collides on the partial-unique pending index → ErrDuplicateRequest (detected via the
// modernc extended result code, NOT a string-match — wantlist precedent). A guild
// bank/bot char is not requestable (ErrCharShared).
func RequestTx(ctx context.Context, tx *sql.Tx, characterID int64, callerID string, now int64) error {
	shared, err := charSharedTx(ctx, tx, characterID)
	if err != nil {
		return err
	}
	if shared {
		return ErrCharShared
	}
	// Snapshot the current assignee (nullable — the char may be unassigned by approval
	// time, or unassigned now and contested only by intent).
	var current sql.NullString
	err = tx.QueryRowContext(ctx,
		`SELECT discord_user_id FROM character_assignment WHERE character_id = ?`, characterID,
	).Scan(&current)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("snapshot current assignee (character_id=%d): %w", characterID, err)
	}
	// D-07: a request is ONLY for a char already assigned to ANOTHER member. Reject the
	// two non-contested cases so the officer queue never accrues claim-able / self-held
	// noise (the frontend only offers Request for assignedToOthers, but the endpoint is
	// directly POST-able by any session).
	if !current.Valid {
		return ErrCharNotContested // unassigned — the caller should /claim, not /request.
	}
	if current.String == callerID {
		return ErrCharNotContested // the caller already holds it.
	}
	var currentArg any
	if current.Valid {
		currentArg = current.String
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO assignment_request (character_id, requester, current_assignee, status, created_at)
		 VALUES (?, ?, ?, 'pending', ?)`,
		characterID, callerID, currentArg, now,
	); err != nil {
		var sqliteErr *sqlite.Error
		if errors.As(err, &sqliteErr) && sqliteErr.Code() == sqliteConstraintUnique {
			return ErrDuplicateRequest
		}
		return fmt.Errorf("file request (character_id=%d, requester=%s): %w", characterID, callerID, err)
	}
	return nil
}

// CancelRequestTx requester-scoped: marks callerID's own PENDING request for
// characterID cancelled (resolved_by=caller, resolved_at=now). Scoped
// `AND requester=caller AND status='pending'` so a foreign or non-pending request
// affects 0 rows → (false, nil): a silent no-op (the release IDOR pattern).
func CancelRequestTx(ctx context.Context, tx *sql.Tx, characterID int64, callerID string, now int64) (cancelled bool, err error) {
	res, err := tx.ExecContext(ctx,
		`UPDATE assignment_request
		    SET status = 'cancelled', resolved_at = ?, resolved_by = ?
		  WHERE character_id = ? AND requester = ? AND status = 'pending'`,
		now, callerID, characterID, callerID,
	)
	if err != nil {
		return false, fmt.Errorf("cancel request (character_id=%d, caller=%s): %w", characterID, callerID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("cancel request rows-affected (character_id=%d): %w", characterID, err)
	}
	return n > 0, nil
}

// OfficerAssignTx assigns/reassigns/overrides characterID to assignee (D-09). FIRST
// statement is the in-tx officer re-check (WR-04 TOCTOU) → ErrNotAuthorized when !ok.
// Rejects a guild bank/bot char (ErrCharShared). Otherwise upserts
// INSERT … ON CONFLICT(character_id) DO UPDATE (reassign/override), assigned_by=the
// officer's id.
func OfficerAssignTx(ctx context.Context, tx *sql.Tx, characterID int64, assignee, callerID string, now int64) error {
	ok, err := isOfficerTx(ctx, tx, callerID) // authorize UNDER the tx (WR-04)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotAuthorized
	}
	shared, err := charSharedTx(ctx, tx, characterID)
	if err != nil {
		return err
	}
	if shared {
		return ErrCharShared
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO character_assignment (character_id, discord_user_id, assigned_at, assigned_by)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(character_id) DO UPDATE SET
		   discord_user_id = excluded.discord_user_id,
		   assigned_at     = excluded.assigned_at,
		   assigned_by     = excluded.assigned_by`,
		characterID, assignee, now, callerID,
	); err != nil {
		return fmt.Errorf("officer assign (character_id=%d, assignee=%s): %w", characterID, assignee, err)
	}
	return nil
}

// RemoveAssignTx removes characterID's assignment entirely (D-09 officer remove). FIRST
// statement is the in-tx officer re-check → ErrNotAuthorized when !ok. Idempotent: a
// missing assignment affects 0 rows → (false, nil).
func RemoveAssignTx(ctx context.Context, tx *sql.Tx, characterID int64, callerID string) (removed bool, err error) {
	ok, err := isOfficerTx(ctx, tx, callerID) // authorize UNDER the tx (WR-04)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, ErrNotAuthorized
	}
	res, err := tx.ExecContext(ctx,
		`DELETE FROM character_assignment WHERE character_id = ?`, characterID,
	)
	if err != nil {
		return false, fmt.Errorf("officer remove assignment (character_id=%d): %w", characterID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("officer remove rows-affected (character_id=%d): %w", characterID, err)
	}
	return n > 0, nil
}

// ApproveRequestTx approves request requestID: it (1) re-checks the officer in-tx
// (→ ErrNotAuthorized); (2) reads the request's character_id + requester; (3) upserts
// the assignment to the requester (ON CONFLICT override, assigned_by=the officer);
// (4) marks THIS request approved; and (5) — Pitfall 3, double-approval defense — marks
// ALL OTHER pending requests for the same character_id as denied, ALL in the same tx.
// A missing/non-pending request affects 0 rows on the approve UPDATE → (false, nil)
// and performs no assignment write (it short-circuits before the upsert).
func ApproveRequestTx(ctx context.Context, tx *sql.Tx, requestID int64, callerID string, now int64) (approved bool, err error) {
	ok, err := isOfficerTx(ctx, tx, callerID) // authorize UNDER the tx (WR-04)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, ErrNotAuthorized
	}
	var characterID int64
	var requester string
	err = tx.QueryRowContext(ctx,
		`SELECT character_id, requester FROM assignment_request WHERE id = ? AND status = 'pending'`,
		requestID,
	).Scan(&characterID, &requester)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return false, nil // no pending request with this id — no-op
	case err != nil:
		return false, fmt.Errorf("read request (id=%d): %w", requestID, err)
	}
	// Upsert the assignment to the requester (approval = immediate reassignment, D-09).
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO character_assignment (character_id, discord_user_id, assigned_at, assigned_by)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(character_id) DO UPDATE SET
		   discord_user_id = excluded.discord_user_id,
		   assigned_at     = excluded.assigned_at,
		   assigned_by     = excluded.assigned_by`,
		characterID, requester, now, callerID,
	); err != nil {
		return false, fmt.Errorf("approve: upsert assignment (character_id=%d): %w", characterID, err)
	}
	// Mark THIS request approved.
	if _, err := tx.ExecContext(ctx,
		`UPDATE assignment_request SET status = 'approved', resolved_at = ?, resolved_by = ?
		   WHERE id = ?`,
		now, callerID, requestID,
	); err != nil {
		return false, fmt.Errorf("approve: resolve request (id=%d): %w", requestID, err)
	}
	// Deny ALL OTHER pending requests for the same char (Pitfall 3 — no double-approval).
	if _, err := tx.ExecContext(ctx,
		`UPDATE assignment_request SET status = 'denied', resolved_at = ?, resolved_by = ?
		   WHERE character_id = ? AND status = 'pending' AND id <> ?`,
		now, callerID, characterID, requestID,
	); err != nil {
		return false, fmt.Errorf("approve: deny sibling requests (character_id=%d): %w", characterID, err)
	}
	return true, nil
}

// DenyRequestTx denies the one PENDING request requestID (D-09). FIRST statement is the
// in-tx officer re-check → ErrNotAuthorized when !ok. A missing/non-pending request
// affects 0 rows → (false, nil).
func DenyRequestTx(ctx context.Context, tx *sql.Tx, requestID int64, callerID string, now int64) (denied bool, err error) {
	ok, err := isOfficerTx(ctx, tx, callerID) // authorize UNDER the tx (WR-04)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, ErrNotAuthorized
	}
	res, err := tx.ExecContext(ctx,
		`UPDATE assignment_request SET status = 'denied', resolved_at = ?, resolved_by = ?
		   WHERE id = ? AND status = 'pending'`,
		now, callerID, requestID,
	)
	if err != nil {
		return false, fmt.Errorf("deny request (id=%d): %w", requestID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("deny request rows-affected (id=%d): %w", requestID, err)
	}
	return n > 0, nil
}

// DesignateMode is the officer 3-state designation for DesignateCharTx: a char is a
// guild bank, a guild bot, or neither (mutually exclusive — never both 1). Open
// Question 3 resolution: mutual exclusion enforced in the store.
type DesignateMode int

const (
	// DesignateNeither clears both flags (a normal, claimable char).
	DesignateNeither DesignateMode = iota
	// DesignateBank marks the char a guild bank (is_bank_toon=1, is_guild_bot=0).
	DesignateBank
	// DesignateBot marks the char a guild bot (is_guild_bot=1, is_bank_toon=0).
	DesignateBot
)

// DesignateCharTx sets characterID's guild bank/bot designation (D-09 officer-only).
// FIRST statement is the in-tx officer re-check → ErrNotAuthorized when !ok. It sets
// is_bank_toon/is_guild_bot per mode (mutually exclusive). It does NOT demote other
// bank toons (multiple guild banks allowed — the single-bank invariant is relaxed). When
// designating bank OR bot (a SHARED char), in the SAME tx it DELETEs any existing
// character_assignment for the char AND marks its pending assignment_requests denied
// (D-02 exemption, Pitfall 6 — bidirectional). Designating 'neither' clears the flags
// and performs no assignment/request cleanup (the char becomes a normal claimable char).
// A missing/removed char → ErrCharNotFound (the UPDATE affected 0 rows).
func DesignateCharTx(ctx context.Context, tx *sql.Tx, characterID int64, mode DesignateMode, callerID string, now int64) error {
	ok, err := isOfficerTx(ctx, tx, callerID) // authorize UNDER the tx (WR-04)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotAuthorized
	}
	bank, bot := 0, 0
	switch mode {
	case DesignateBank:
		bank = 1
	case DesignateBot:
		bot = 1
	case DesignateNeither:
		// both 0
	}
	res, err := tx.ExecContext(ctx,
		`UPDATE character SET is_bank_toon = ?, is_guild_bot = ? WHERE id = ? AND is_removed = 0`,
		bank, bot, characterID,
	)
	if err != nil {
		return fmt.Errorf("designate char (character_id=%d): %w", characterID, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrCharNotFound
	}
	// Bidirectional exemption (D-02, Pitfall 6): a shared char has NO assignee and no
	// open requests. Clear them in the SAME tx when designating bank/bot.
	if mode == DesignateBank || mode == DesignateBot {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM character_assignment WHERE character_id = ?`, characterID,
		); err != nil {
			return fmt.Errorf("designate: clear assignment (character_id=%d): %w", characterID, err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE assignment_request SET status = 'denied', resolved_at = ?, resolved_by = ?
			   WHERE character_id = ? AND status = 'pending'`,
			now, callerID, characterID,
		); err != nil {
			return fmt.Errorf("designate: deny pending requests (character_id=%d): %w", characterID, err)
		}
	}
	return nil
}

// ListMyAssignments returns the characters assigned to discordUserID (ASSIGN-01, the
// "My characters" read). Joined to character.name; live chars only (is_removed=0).
// Ordered by name. Empty → nil (the handler normalizes nil → []).
func ListMyAssignments(ctx context.Context, db *sql.DB, discordUserID string) ([]Assignment, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT a.character_id, c.name, a.discord_user_id, a.assigned_at, a.assigned_by
		   FROM character_assignment a
		   JOIN character c ON c.id = a.character_id
		  WHERE a.discord_user_id = ? AND c.is_removed = 0
		  ORDER BY c.name COLLATE NOCASE`,
		discordUserID,
	)
	if err != nil {
		return nil, fmt.Errorf("list my assignments (user=%s): %w", discordUserID, err)
	}
	defer rows.Close()
	return scanAssignments(rows)
}

// MyPendingRequest is one of the CALLER's own pending assignment_request rows joined to
// the contested character's name (ListMyPendingRequests — the member "my outstanding
// requests" read). It lets MyCharactersPanel rehydrate the Request→Cancel affordance
// across a reload instead of relying on session-only optimistic state. snake_case JSON
// tags — crosses the API boundary in 26-02.
type MyPendingRequest struct {
	CharacterID   int64  `json:"character_id"`
	CharacterName string `json:"character_name"`
	CreatedAt     int64  `json:"created_at"`
}

// ListMyPendingRequests returns the caller's OWN pending assignment_request rows
// (requester-scoped — never another member's), joined to the contested character's name.
// Live chars only (is_removed=0). Ordered by name. Empty → nil (the handler normalizes
// nil → []). This is the read behind GET /api/v1/assignments/requests/mine; the caller
// is always derived server-side from the session (the requester arg, IDOR-safe).
func ListMyPendingRequests(ctx context.Context, db *sql.DB, requester string) ([]MyPendingRequest, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT r.character_id, c.name, r.created_at
		   FROM assignment_request r
		   JOIN character c ON c.id = r.character_id
		  WHERE r.requester = ? AND r.status = 'pending' AND c.is_removed = 0
		  ORDER BY c.name COLLATE NOCASE`,
		requester,
	)
	if err != nil {
		return nil, fmt.Errorf("list my pending requests (requester=%s): %w", requester, err)
	}
	defer rows.Close()

	var out []MyPendingRequest
	for rows.Next() {
		var pr MyPendingRequest
		if err := rows.Scan(&pr.CharacterID, &pr.CharacterName, &pr.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan my pending request row: %w", err)
		}
		out = append(out, pr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate my pending requests: %w", err)
	}
	return out, nil
}

// ClaimableChar is one unassigned, non-shared, live character a member may self-claim
// (ASSIGN-02, the "claim a character" pick-list). snake_case JSON tags — crosses the
// API boundary in 26-02.
type ClaimableChar struct {
	CharacterID int64  `json:"character_id"`
	Name        string `json:"name"`
}

// ListClaimable returns the live characters that are self-claimable (ASSIGN-02): not
// shared (is_bank_toon=0 AND is_guild_bot=0), not removed, and with NO row in
// character_assignment (unassigned). Ordered by name. Empty → nil (the handler
// normalizes nil → []). This is the read behind GET /assignments/claimable; a member
// claims one of these via ClaimCharTx (which re-checks shared/already-assigned in-tx,
// closing the list→claim TOCTOU).
func ListClaimable(ctx context.Context, db *sql.DB) ([]ClaimableChar, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT c.id, c.name
		   FROM character c
		  WHERE c.is_removed = 0
		    AND c.is_bank_toon = 0
		    AND c.is_guild_bot = 0
		    AND NOT EXISTS (SELECT 1 FROM character_assignment a WHERE a.character_id = c.id)
		  ORDER BY c.name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list claimable: %w", err)
	}
	defer rows.Close()

	var out []ClaimableChar
	for rows.Next() {
		var cc ClaimableChar
		if err := rows.Scan(&cc.CharacterID, &cc.Name); err != nil {
			return nil, fmt.Errorf("scan claimable row: %w", err)
		}
		out = append(out, cc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate claimable: %w", err)
	}
	return out, nil
}

// ListAllAssignments returns every live-char assignment (ASSIGN-04, the officer view).
// Joined to character.name; live chars only. Ordered by name.
func ListAllAssignments(ctx context.Context, db *sql.DB) ([]Assignment, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT a.character_id, c.name, a.discord_user_id, a.assigned_at, a.assigned_by
		   FROM character_assignment a
		   JOIN character c ON c.id = a.character_id
		  WHERE c.is_removed = 0
		  ORDER BY c.name COLLATE NOCASE`,
	)
	if err != nil {
		return nil, fmt.Errorf("list all assignments: %w", err)
	}
	defer rows.Close()
	return scanAssignments(rows)
}

// ListPendingRequests returns the pending assignment_request queue (D-07, the officer
// approve/deny surface), joined to the contested character's name. Live chars only.
// Ordered oldest-first (created_at) so the queue is FIFO.
func ListPendingRequests(ctx context.Context, db *sql.DB) ([]PendingRequest, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT r.id, r.character_id, c.name, r.requester, r.current_assignee, r.created_at
		   FROM assignment_request r
		   JOIN character c ON c.id = r.character_id
		  WHERE r.status = 'pending' AND c.is_removed = 0
		  ORDER BY r.created_at ASC, r.id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list pending requests: %w", err)
	}
	defer rows.Close()

	var out []PendingRequest
	for rows.Next() {
		var pr PendingRequest
		var current sql.NullString
		if err := rows.Scan(&pr.ID, &pr.CharacterID, &pr.CharacterName, &pr.Requester, &current, &pr.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan pending request row: %w", err)
		}
		if current.Valid {
			v := current.String
			pr.CurrentAssignee = &v
		}
		out = append(out, pr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending requests: %w", err)
	}
	return out, nil
}

// scanAssignments drains a *sql.Rows of (character_id, name, discord_user_id,
// assigned_at, assigned_by) into []Assignment.
func scanAssignments(rows *sql.Rows) ([]Assignment, error) {
	var out []Assignment
	for rows.Next() {
		var a Assignment
		if err := rows.Scan(&a.CharacterID, &a.Name, &a.DiscordUserID, &a.AssignedAt, &a.AssignedBy); err != nil {
			return nil, fmt.Errorf("scan assignment row: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate assignments: %w", err)
	}
	return out, nil
}
