package store

// owner.go holds the reserved "guild" sentinel owner (Phase 35, OWN-01/02).
//
// character.owner_id is a NOT-NULL first-sighting steward marker (binding.go): the
// first guildie to upload a character is recorded as its owner, but that is NOT a
// write gate (cross-owner uploads are allowed + audited since 260621-u6j). The
// per-owner eviction cascade (EvictOwnerTx, `WHERE owner_id = ?`) sweeps every one
// of an evicted guildie's characters — which previously swept a guild bank away with
// its first uploader.
//
// To make a designated guild bank/bot GUILD-HELD (owner-less, eviction-proof), a
// reserved sentinel owner row is seeded by migration 00015 (INSERT OR IGNORE INTO
// owner (id, label) VALUES (1000000, 'guild')). DesignateCharTx repoints a bank/bot's
// owner_id to it; the 00015 backfill repointed existing banks/bots. Because the
// sentinel id belongs to no guildie, no eviction ever targets it, so banks/bots
// survive eviction by construction (OWN-02). The eviction owner-lists exclude it so
// an officer can never pick "evict the guild bank" (eviction.go).
//
// The id is the FIXED literal 1000000 — far above the organic owner autoincrement
// range at a guild scale of ~12 owners, so it can never collide with an INSERTed
// owner row (owner.id is INTEGER PRIMARY KEY). This constant MUST equal the migration
// literal in 00015_guild_owner.sql.

// GuildSentinelOwnerID is the reserved owner id that holds owner-less (guild-held)
// banks/bots. It is the single Go-side source of truth for the sentinel id; it MUST
// equal the literal seeded by migration 00015 (00015_guild_owner.sql). The migration
// guarantees this row exists, so callers can reference the id directly — no DB
// resolver / SELECT is needed.
const GuildSentinelOwnerID int64 = 1000000
