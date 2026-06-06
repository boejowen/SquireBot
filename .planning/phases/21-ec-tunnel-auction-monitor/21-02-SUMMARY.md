---
phase: 21-ec-tunnel-auction-monitor
plan: 02
subsystem: backend-notify-wantmatch
tags: [notify, wantmatch, discordgo, embed, alert_log, WANT-05]
requires:
  - notify.Send two-gate + dedup/cooldown + alert_log core (Phase 20)
  - wantmatch.ForItem / ForName shared matcher (Phase 20)
  - wantlist_item.note column (migration 00006_wantlist.sql:27)
provides:
  - notify.Sender.ChannelMessageSendEmbed — the rich-embed seam (D-04)
  - notify.Alert.Embed — the embed payload field; rides the SAME Send core
  - wantmatch.Hit.Note — the want's nullable "why you wanted it" note (D-05)
affects:
  - 21-03 (ec.RunMatch composes the embed + reads Hit.Note, calls notify.Send with Embed set)
tech-stack:
  added: []
  patterns:
    - "extend-in-place: one new Sender method + one Alert field reuse the single Send gate/dedup/alert_log block (no duplicate send path)"
    - "shared scanHits carries the new nullable note column for BOTH ForItem and ForName via one change"
key-files:
  created: []
  modified:
    - internal/backendsrv/notify/dm.go
    - internal/backendsrv/notify/dm_test.go
    - internal/backendsrv/wantmatch/match.go
    - internal/backendsrv/wantmatch/match_test.go
decisions:
  - "D-04 embed routes through the existing two-gate + dedup/cooldown + alert_log core — Alert.Embed is the seam, NOT a parallel SendEmbed func (PATTERNS); the EC job sets Alert.Embed and calls Send"
  - "Send-step branch is a single inline closure so the 50007/error/recordAttempt/slog handling stays verbatim for both the embed and string branches — exactly one send/record block"
  - "Hit.Note is *string (nullable) populated by the shared scanHits, so ForItem AND ForName both carry it with one scanner change (only ADD the column; no P22 name-matching behavior enabled)"
metrics:
  duration: ~25m
  completed: 2026-06-06
  tasks: 2
  files-changed: 4
  commits: 2
---

# Phase 21 Plan 02: notify embed send-path + wantmatch Hit.Note Summary

Extended the Phase 20 `notify` spine with a discordgo rich-embed send-path (D-04) that rides the EXACT same two-gate + dedup/cooldown + `alert_log` core as the plain-string `Send`, and added a nullable `Note` to `wantmatch.Hit` (D-05) carried by the shared scanner so Plan 03's EC producer can compose a polished "why-you-wanted-it" embed without any new gate logic.

## What Was Built

### Task 1 — notify embed send-path (commit 72edf87)
- Added `ChannelMessageSendEmbed(channelID string, embed *discordgo.MessageEmbed, options ...discordgo.RequestOption) (*discordgo.Message, error)` to the `Sender` interface. The real `*discordgo.Session` already satisfies it (restapi.go:1812), so the `var _ Sender = (*discordgo.Session)(nil)` compile-time assertion still holds — no adapter needed.
- Added `Embed *discordgo.MessageEmbed` to `Alert`. Nil ⇒ the P20 plain-string path is unchanged.
- Branched the single SEND step: `a.Embed != nil` delivers via `ChannelMessageSendEmbed`, else via `ChannelMessageSend`. The branch is an inline closure that returns the send error, so ALL surrounding logic (the 50007 `isDMBlocked` check, `recordAttempt("dm_blocked"/"error"/"sent")`, and the V7 slog) is identical and runs ONCE. No second gate/dedup block was added.
- `dm_test.go`: `fakeSender` gained `ChannelMessageSendEmbed` (counts `embeds` separately from `sends`), plus `TestSendEmbed_*` cases — embed-path success (asserts the embed branch fires and the string branch does NOT), nil-embed string fallback, 50007 → dm_blocked, officer-flag gate, user-pref gate, and cooldown skip.

### Task 2 — wantmatch.Hit.Note (commit 92b8fa8)
- Added `Note *string` to `Hit` (nullable — `wantlist_item.note` is a nullable TEXT column).
- Added `note` to BOTH the `ForItem` and `ForName` SELECT lists (after `reason`).
- `scanHits` (the single shared scanner used by both entry points) now scans `note` via `sql.NullString` into `Hit.Note`, mirroring the existing nullable `item_id` handling — one change covers both callers.
- `match_test.go`: `seedWant` now delegates to a new `seedWantNote` helper; `TestForItem_CarriesNote` and `TestForName_CarriesNote` assert the note is populated when set and nil when NULL; the existing ForItem assertion gained a nil-note check.

## Deviations from Plan

None — plan executed exactly as written. Both tasks (`tdd="true"`) followed RED (compile-failing test) → GREEN (minimal impl). No REFACTOR was needed.

## Verification

- `go test ./internal/backendsrv/notify/ ./internal/backendsrv/wantmatch/` — green.
- `go build ./...` — exit 0 (the `var _ Sender = (*discordgo.Session)(nil)` assertion compiles with the extended interface).
- `go vet ./internal/backendsrv/...` — exit 0.
- `grep -c 'GetMonitorFlags' internal/backendsrv/notify/dm.go` → **1** (exactly one gate/dedup block; no duplicated/bypassed gate logic — T-21-05/T-21-06 mitigation holds).
- `grep -c 'reason, note' internal/backendsrv/wantmatch/match.go` → **2** (both SELECTs carry the note column).

## Security / Threat Notes

- T-21-05 (ungated DM) and T-21-06 (DM spam): the embed rides the SAME `Send` core — both gates (officer `monitor_flag` + user `notify_prefs`), the upstream mute gate (`wantmatch muted=0`), the `RecentAlertExists` dedup, and `cooldownEC=22h` all apply unchanged. There is exactly one gate/dedup block; the EC job MUST set `Alert.Embed` and call `Send`, never call `ChannelMessageSendEmbed` directly.
- T-21-07 (info disclosure): the SEND-step slog still logs `source + want + status` only. The new `Embed` field carries the V7 "never logged" doc-comment, matching the existing `Body` discipline.

## Self-Check: PASSED

- FOUND: internal/backendsrv/notify/dm.go (ChannelMessageSendEmbed + Embed field)
- FOUND: internal/backendsrv/wantmatch/match.go (Hit.Note + note in both SELECTs)
- FOUND commit 72edf87 (Task 1)
- FOUND commit 92b8fa8 (Task 2)
