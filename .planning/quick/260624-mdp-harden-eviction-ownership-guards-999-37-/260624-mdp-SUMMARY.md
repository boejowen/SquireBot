---
quick_id: 260624-mdp
title: Harden eviction/ownership guards (backlog 999.37–39)
status: complete
date: 2026-06-24
commits: [71fed2c, d98da18, 470b670]
---

# Quick Task 260624-mdp — Summary

Three related backend defense-in-depth cleanups deferred from the v2.5 (Phase 35/36)
code review. Backend-only; **no schema change, no watcher change, no `v*` tag.** All
gates green (`go build ./...`, full `go test ./...`).

## What changed

### 999.38 / WR-02 — repoint skips already-evicted stewards (commit `71fed2c`)
`store/eviction.go`: `recentOtherSharerSubquery` gains a live-steward filter —
`AND EXISTS (SELECT 1 FROM character c2 WHERE c2.owner_id = a.attempting_owner_id
AND c2.is_removed = 0)`. A surviving shared char is no longer repointed onto an owner
who is themselves evicted; if no candidate sharer is live, the char is left on its
current steward (NULL subquery → IS-NOT-NULL guard skips it).
- Tests: gave the two existing repoint tests' sharer owners a live char (they must now
  qualify); added `TestEvictOwnerTx_RepointSkipsEvictedSharer` (a more-recent-but-evicted
  sharer is skipped for an earlier live one); re-scoped
  `TestRepointSubquery_LocksPredicateToSharedPredicate` to lock the shared tokens in BOTH
  consts **and** the new live-steward clause into the repoint subquery only (it must not
  leak into the survival predicate, which counts even an evicted sharer as making a char
  shared).

### 999.39 / IN-02 — label bridges exclude the reserved sentinel (commits `d98da18`, `470b670`)
A guildie whose Discord username / `mint-code --owner` arg is literally "guild" could
resolve the reserved sentinel owner (id 1000000, label `guild`). Each owner-by-label
SELECT now excludes it by id (`AND id <> store.GuildSentinelOwnerID`):
- `store/linking.go` ResolveOrCreateOwnerByDiscordTx label bridge.
- `auth/store.go` `upsertOwner` (mint path) — `auth` now imports `store`; verified **no
  import cycle** (`store` does not import `auth`; `go build ./...` clean).
- `webadmin/eviction.go` `callerMayNotEvictFloor` TRIM(label) floor bridge.

### 999.37 / WR-01 — preview-handler guard parity (commit `470b670`)
`webadmin/eviction.go` `EvictionPreviewHandler` now mirrors the guards
`EvictHandler`/`EvictOwnerTx` enforce, before any store read:
- sentinel guard → 403 `cannot_evict_sentinel`;
- owner-floor guard (`callerMayNotEvictFloor`) → 403 `owner_floor_protected`.
So a peer can no longer use the read-only preview to enumerate a floor-protected (or
guild-sentinel) roster.
- Tests: `TestEvictionPreview_OfficerSeesRoster` (happy path intact),
  `_RefusesGuildSentinel`, `_PeerCannotPreviewFloorData`.

## Deploy status
**✅ DEPLOYED LIVE 2026-06-24** (binary-swap only; no migration). Sequence: cross-compile
linux/amd64 (`-trimpath -ldflags "-s -w"`, 13 MB) → R2 backup first (`squirebot-2026-06-25.db.gz`,
box is UTC) → scp → hash-verify (`0a9f2658…`, differs from prior `01198c37…`) → `cp .bak` +
`install -m0755` + `systemctl restart`. Boot log: `goose: no migrations to run. current
version: 15`, `bot connected`, `listening 127.0.0.1:8090 pid=545312`, scheduler 4 jobs, no
errors. External smoke (full TLS→Caddy→binary): `/api/v1/admin/eviction/preview`→**401**,
`POST /api/v1/admin/evict`→**401** (fail-closed, route live), api-root→404, web apex→200.
Rollback: `/usr/local/bin/squirebot-server.bak` (prior binary) kept on the box. No `v*` tag
(watcher untouched).

## Remaining backlog
- **999.40** (v2.4 MD-01) — dead wantlist client wrappers in `web/src/lib/api.ts` +
  orphaned `WantAddForm`/`WantMuteCell` + stale `api.test.ts`. Web-only; next quick task.
