package store

// sqliteconstraint.go holds the two tiny package-shared SQLite helpers that
// several store layers depend on (the typed-conflict detection + the bool→INTEGER
// coercion). They were extracted here from the now-deleted wantlist.go (the v2.4
// clean break removed the item-centric wantlist; these helpers outlived it):
//   - sqliteConstraintUnique is read by assignment.go (RequestTx), guildchannel.go
//     (AddGuildChannelTx), and wishlist.go (AddWishlistTx) to map a unique-index
//     violation to a TYPED sentinel via the modernc driver's extended result code.
//   - boolToInt is read by guildchannel.go (SetMonitorFlagTx) and wishlist.go
//     (SetPingedTx) to store a bool as the INTEGER 0/1 a flag column uses.

// sqliteConstraintUnique is SQLITE_CONSTRAINT_UNIQUE, the extended result code a
// unique-index violation reports. Hard-coded (with this comment) to avoid importing
// the modernc.org/sqlite/lib subpackage just for the one constant; extended result
// codes are enabled on every connection (modernc conn.go:660), so *sqlite.Error.Code()
// returns this extended value, not the primary SQLITE_CONSTRAINT (19).
const sqliteConstraintUnique = 2067

// boolToInt converts a bool to the INTEGER 0/1 a flag column (e.g. muted/pinged/
// enabled) uses.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
