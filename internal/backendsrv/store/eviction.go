package store

// eviction.go is the SQLite port of v1's eviction workflow
// (apps-script/src/triggers/showEvictionSidebar.ts + weeklyEvictionArchive.ts —
// ADMIN-04 / D-09/D-10). Eviction targets a whole guildie (owner) and cascades
// across ALL their characters.
//
// Ported + EXTENDED semantics:
//   - PER-OWNER CASCADE (v1): EvictOwnerTx flips is_removed=1 on every one of the
//     owner's not-yet-removed characters and stamps grace_until = now + 30 days
//     (v1's GRACE_MS). RowsAffected = the count of characters actually flipped
//     (v1's affectedChars), so re-evicting an already-evicted owner is a no-op
//     count of 0.
//   - D-10 EXTENSION (DB-world only): the SAME transaction also revokes the
//     owner's guild code (guild_code.disabled_at = now) so their watcher stops
//     uploading — the "one clean app-controlled action" the roadmap highlights,
//     replacing v1's separate Google-Drive un-share. guild_code.disabled_at is
//     TEXT in 00001, so it is set with datetime('now') to match its type; the
//     character grace_until is epoch INTEGER.
//   - REVERSIBLE DURING GRACE (D-10): RestoreOwnerTx un-sets is_removed and
//     clears grace_until for characters not yet archived (re-minting a guild code
//     is a separate CLI action). Past-grace (archived) characters are NOT
//     restored.
//   - ARCHIVE PAST GRACE (weeklyEvictionArchive): ArchiveExpiredEvictions stamps
//     archived_at on characters whose grace_until has passed and that are not yet
//     archived. Idempotent (the archived_at IS NULL guard short-circuits a
//     re-run), mirroring v1's moveCharToArchive idempotency.
//
// Parameterized ? placeholders ONLY (V5). The *Tx methods take the caller's
// *sql.Tx so the 15-03 handler composes evict + an audit_log row in one tx.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// EvictionGraceSeconds is the post-eviction grace window before hard archive: 30
// days (v1's GRACE_MS = 30*24*60*60*1000 ms). Epoch seconds here.
const EvictionGraceSeconds int64 = 30 * 24 * 60 * 60 // 2592000

// ErrCannotEvictSentinel is returned by EvictOwnerTx / RestoreOwnerTx when the
// target is the guild sentinel owner (GuildSentinelOwnerID). OWN-02: the sentinel
// holds owner-less banks/bots and is NEVER evictable; the destructive WRITE path
// guards against it (not just the picker list) because the endpoint accepts an
// untrusted owner_id. The webadmin handler maps this to a clean refusal.
var ErrCannotEvictSentinel = errors.New("cannot_evict_sentinel")

// EvictableOwner is one row of the eviction picker (ListEvictableOwners): an
// owner with at least one live (is_removed=0) character. Label is owner.label —
// there is NO owner_email column in 00001 (owners are keyed by label). CharCount
// is the live-character count shown in the confirm UI. snake_case JSON tags
// (crosses the API boundary in 15-03).
type EvictableOwner struct {
	OwnerID   int64  `json:"owner_id"`
	Label     string `json:"label"`
	CharCount int    `json:"char_count"`
}

// ListEvictableOwners returns owners that have >=1 live character, grouped, with
// the live-character count (v1's getEvictionEmails, owner-keyed here). Owners
// whose characters are all already removed are excluded. Ordered by label.
//
// OWN-02: the guild sentinel owner (store.GuildSentinelOwnerID) holds owner-less
// banks/bots and must NEVER be offered as an evictable guildie — the `o.id <> ?`
// exclusion stops an officer from accidentally evicting the whole guild bank.
func ListEvictableOwners(ctx context.Context, db *sql.DB) ([]EvictableOwner, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT o.id, o.label, COUNT(c.id) AS char_count
		   FROM owner o
		   JOIN character c ON c.owner_id = o.id AND c.is_removed = 0
		  WHERE o.id <> ?
		  GROUP BY o.id, o.label
		 HAVING char_count > 0
		  ORDER BY o.label COLLATE NOCASE`, GuildSentinelOwnerID)
	if err != nil {
		return nil, fmt.Errorf("list evictable owners: %w", err)
	}
	defer rows.Close()

	var out []EvictableOwner
	for rows.Next() {
		var e EvictableOwner
		if err := rows.Scan(&e.OwnerID, &e.Label, &e.CharCount); err != nil {
			return nil, fmt.Errorf("scan evictable owner: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate evictable owners: %w", err)
	}
	return out, nil
}

// RestorableOwner is one row of the eviction-RESTORE picker
// (ListRestorableOwners): an owner with at least one character that is still in
// grace (is_removed=1, grace_until in the future, not yet archived) — the inverse
// of EvictableOwner. CharCount is how many of the owner's characters the restore
// would un-remove; GraceUntil is the SOONEST deadline across them (MIN), so the UI
// shows how long the officer still has. snake_case JSON tags (crosses the API
// boundary).
type RestorableOwner struct {
	OwnerID    int64  `json:"owner_id"`
	Label      string `json:"label"`
	CharCount  int    `json:"char_count"`
	GraceUntil int64  `json:"grace_until"`
}

// ListRestorableOwners returns owners that have >=1 character STILL IN GRACE —
// is_removed=1 AND grace_until IS NOT NULL AND grace_until > now AND archived_at IS
// NULL — the set RestoreOwnerTx can still reverse (the inverse of
// ListEvictableOwners). Live owners (nothing removed) and past-grace/archived
// owners are excluded. char_count is the count of still-in-grace characters;
// grace_until is the soonest (MIN) deadline. Ordered by label. Parameterized `?`
// only (V5).
//
// OWN-02: the guild sentinel owner (store.GuildSentinelOwnerID) holds owner-less
// banks/bots and must NEVER be offered as a restorable guildie — the `o.id <> ?`
// exclusion keeps it out of the restore picker (a bank should never reach the
// in-grace state via the app, but the list must still never surface the sentinel).
func ListRestorableOwners(ctx context.Context, db *sql.DB, now int64) ([]RestorableOwner, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT o.id, o.label, COUNT(c.id) AS char_count, MIN(c.grace_until) AS grace_until
		   FROM owner o
		   JOIN character c ON c.owner_id = o.id
		          AND c.is_removed = 1
		          AND c.grace_until IS NOT NULL
		          AND c.grace_until > ?
		          AND c.archived_at IS NULL
		  WHERE o.id <> ?
		  GROUP BY o.id, o.label
		 HAVING char_count > 0
		  ORDER BY o.label COLLATE NOCASE`, now, GuildSentinelOwnerID)
	if err != nil {
		return nil, fmt.Errorf("list restorable owners: %w", err)
	}
	defer rows.Close()

	var out []RestorableOwner
	for rows.Next() {
		var r RestorableOwner
		if err := rows.Scan(&r.OwnerID, &r.Label, &r.CharCount, &r.GraceUntil); err != nil {
			return nil, fmt.Errorf("scan restorable owner: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate restorable owners: %w", err)
	}
	return out, nil
}

// PreviewEviction returns the sorted names of the owner's live characters — what
// the confirm-before-commit UI lists (v1's previewEviction.chars). Read-only.
func PreviewEviction(ctx context.Context, db *sql.DB, ownerID int64) ([]string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT name FROM character WHERE owner_id = ? AND is_removed = 0 ORDER BY name COLLATE NOCASE`,
		ownerID,
	)
	if err != nil {
		return nil, fmt.Errorf("preview eviction (owner_id=%d): %w", ownerID, err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, fmt.Errorf("scan preview char: %w", err)
		}
		names = append(names, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate preview chars: %w", err)
	}
	return names, nil
}

// EvictOwnerTx evicts a whole guildie inside the caller's tx (D-09/D-10):
//   - grace_until = now + EvictionGraceSeconds.
//   - UPDATE character SET is_removed=1, grace_until=? for the owner's live
//     characters (RowsAffected → removedCount; an already-fully-removed owner
//     yields 0).
//   - UPDATE guild_code SET disabled_at=datetime('now') for the owner's active
//     codes (D-10 revoke — the watcher stops uploading), in the SAME tx.
//
// Returns the count of characters flipped and the graceUntil stamp so the 15-03
// handler can echo them + write the audit_log row in the same tx.
func EvictOwnerTx(ctx context.Context, tx *sql.Tx, ownerID, now int64) (removedCount int, graceUntil int64, err error) {
	// OWN-02: the guild sentinel holds owner-less banks/bots and is NEVER evictable;
	// guard the WRITE path, not just the picker list (the endpoint accepts an untrusted owner_id).
	if ownerID == GuildSentinelOwnerID {
		return 0, 0, ErrCannotEvictSentinel
	}
	graceUntil = now + EvictionGraceSeconds

	res, err := tx.ExecContext(ctx,
		`UPDATE character SET is_removed = 1, grace_until = ? WHERE owner_id = ? AND is_removed = 0`,
		graceUntil, ownerID,
	)
	if err != nil {
		return 0, 0, fmt.Errorf("evict cascade (owner_id=%d): %w", ownerID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, 0, fmt.Errorf("evict cascade rows-affected (owner_id=%d): %w", ownerID, err)
	}
	removedCount = int(n)

	// D-10: revoke the owner's active guild code(s) in the SAME tx. disabled_at is
	// TEXT (00001) → datetime('now'); the `disabled_at IS NULL` guard makes a
	// re-evict idempotent on the code side too.
	if _, err := tx.ExecContext(ctx,
		`UPDATE guild_code SET disabled_at = datetime('now') WHERE owner_id = ? AND disabled_at IS NULL`,
		ownerID,
	); err != nil {
		return 0, 0, fmt.Errorf("evict code-revoke (owner_id=%d): %w", ownerID, err)
	}

	return removedCount, graceUntil, nil
}

// RestoreOwnerTx reverses an eviction during grace (D-10): un-sets is_removed and
// clears grace_until for the owner's characters that are NOT yet archived
// (archived_at IS NULL). Past-grace (archived) characters are intentionally left
// removed. Re-minting a guild code is a separate CLI action (not done here).
// Returns the count of characters restored.
func RestoreOwnerTx(ctx context.Context, tx *sql.Tx, ownerID, now int64) (restoredCount int, err error) {
	// OWN-02: the guild sentinel holds owner-less banks/bots and is NEVER evictable;
	// guard the WRITE path, not just the picker list (the endpoint accepts an untrusted owner_id).
	if ownerID == GuildSentinelOwnerID {
		return 0, ErrCannotEvictSentinel
	}
	res, err := tx.ExecContext(ctx,
		`UPDATE character SET is_removed = 0, grace_until = NULL
		  WHERE owner_id = ? AND is_removed = 1 AND archived_at IS NULL`,
		ownerID,
	)
	if err != nil {
		return 0, fmt.Errorf("restore owner (owner_id=%d): %w", ownerID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("restore owner rows-affected (owner_id=%d): %w", ownerID, err)
	}
	return int(n), nil
}

// ArchiveExpiredEvictions hard-archives every character whose grace has expired
// (the lazy/scheduled archive step, v1's weeklyEvictionArchive). It stamps
// archived_at=now where is_removed=1 AND grace_until < now AND archived_at IS
// NULL. Idempotent: the archived_at IS NULL guard means a second run over the
// same set is a no-op (mirrors v1's moveCharToArchive idempotency). Returns the
// count archived this call. Plain *sql.DB (single-statement, no tx composition
// needed — like jobstate.go).
func ArchiveExpiredEvictions(ctx context.Context, db *sql.DB, now int64) (archived int, err error) {
	res, err := db.ExecContext(ctx,
		`UPDATE character SET archived_at = ?
		  WHERE is_removed = 1 AND grace_until IS NOT NULL AND grace_until < ? AND archived_at IS NULL`,
		now, now,
	)
	if err != nil {
		return 0, fmt.Errorf("archive expired evictions: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("archive expired evictions rows-affected: %w", err)
	}
	return int(n), nil
}
