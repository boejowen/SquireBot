# Phase 20: Bot + DM + Notification Infrastructure - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-06-05
**Phase:** 20-bot-dm-notification-infrastructure
**Areas discussed:** Opt-in model & default, Notification inbox scope/UX, Officer controls (WANT-08), Preference depth / snooze

---

## Opt-in model & default

### Default state at launch
| Option | Description | Selected |
|--------|-------------|----------|
| Default ON | Building a wantlist is the opt-in signal; alerts fire when monitors go live; global mute always available | ✓ |
| Default OFF (opt-in) | No DMs until explicitly enabled; safest vs surprise DMs but risks a dead feature | |

### Granularity of user's own control
| Option | Description | Selected |
|--------|-------------|----------|
| Per-monitor + global master | Master on/off + separate EC/WTS/raid toggles (different noise profiles) | ✓ |
| Single global on/off | One switch for everything | |
| Per-want only | Mute individual wants; no monitor-level control | |

### Where prefs are managed
| Option | Description | Selected |
|--------|-------------|----------|
| New /notifications page | Dedicated login-gated page hosting prefs + inbox; nav link beside Wantlist | ✓ |
| On /account | Fold toggles into the existing account page | |
| On /wantlist | Prefs bar atop the wantlist page | |

**User's choice:** Default ON · per-monitor + global master · new /notifications page.
**Notes:** The wantlist itself is treated as the opt-in signal; the /notifications page becomes the single "my alerts" home (prefs + inbox together).

---

## Notification inbox scope/UX

### Scope
| Option | Description | Selected |
|--------|-------------|----------|
| Every alert / full history | All attempts (delivered + dm_blocked + error), can't-DM flagged; reuses alert_log | ✓ |
| Only can't-DM fallbacks | Just the undeliverable safety net; loses the "what was I pinged about" history | |

### UX richness
| Option | Description | Selected |
|--------|-------------|----------|
| Unread badge + mark-read | Nav unread-count badge + per-row/all mark-read; ensures can't-DM alerts get noticed | ✓ |
| Simple reverse-chron list | Newest-first, no read state, no badge | |

**User's choice:** Full alert history + unread badge & mark-read.
**Notes:** Requires a read-state on alerts (e.g. alert_log.read_at). Can't-DM rows should carry an actionable "enable server DMs" hint (copy = Claude's discretion).

---

## Officer controls (WANT-08)

### Location
| Option | Description | Selected |
|--------|-------------|----------|
| Extend /admin (Monitors section) | New section on the existing officer surface | ✓ |
| New /admin/monitors page | Dedicated officer sub-page | |

### Enable + channel-registration method
| Option | Description | Selected |
|--------|-------------|----------|
| Officer UI form, DB-backed | Per-monitor toggles + add-channel form → guild_channel rows; flip on with no redeploy | ✓ |
| Small CLI | squirebot-server enable-monitor / register-channel over SSH | |
| Env/config file | Channels + flags in squirebot.env, applied on restart | |

**User's choice:** Monitors section on /admin · DB-backed officer UI form.
**Notes:** WTS/raid default OFF (dark); channel-registration UI is built slightly ahead of its invite-gated consumers (P22/P23) by design. Officer flag is a guild-wide kill-switch, distinct from a user's own per-monitor opt-in — both must allow an alert for it to fire.

---

## Preference depth / snooze

### Per-want control scope
| Option | Description | Selected |
|--------|-------------|----------|
| Per-want mute now | `muted` boolean on wantlist_item; bell toggle on /wantlist grid; defers timed snooze | ✓ |
| Full snooze (mute + snooze-until) | Adds timed snooze-until column + un-snooze logic | |
| Defer all per-want control | Global + per-monitor only this phase | |

### DM end-to-end proof (WANT-03)
| Option | Description | Selected |
|--------|-------------|----------|
| /admin "Send me a test alert" button | Officer self-serve DM proof + bot-health pulse; exercises 50007 path | ✓ |
| CLI test-dm command | squirebot-server test-dm <discord-id> over SSH | |
| Both | CLI + button | |

**User's choice:** Per-want mute now · /admin "Send me a test alert" button.
**Notes:** Timed snooze-until deferred. The test button doubles as a bot-health check and naturally surfaces the 50007 path if the officer has DMs off.

---

## Claude's Discretion
- Per-source cooldown default *values* (mechanism is a per-source tunable constant here; EC value finalized in Phase 21's spike).
- Exact DM message format (the test-alert is a representative sample; real per-monitor formats are Phase 21+).
- Bot presence/status/activity string + exposing bot connection state on /healthz (PITFALLS-v2.2 Pitfall 7).
- The Phase-20 goose migration shape (guild_channel, per-user notify-prefs, alert_log.read_at, wantlist_item.muted) — extend-only.
- Exact copy for the can't-DM hint and the /notifications page.

## Deferred Ideas
- Timed snooze-until per want.
- Digest mode + quiet hours (research-deferred unless soak shows noise).
- CLI test-dm (button covers the proof).
- "Retry delivery" action from the inbox.
- Auto-suggest wants from gear_check/spell_check MISSING rows.
- Read-only guild-aggregate wantlist.
- The actual EC/WTS/raid monitors (Phases 21/22/23) — Phase 20 is the spine only.
