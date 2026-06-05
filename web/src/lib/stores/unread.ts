// unread — the cross-component unread-alert count store backing the Notifications
// nav badge (20-04 / D-05, the load-bearing "you missed something" signal). The
// SiteShell nav reads `unreadCount`; the NotificationInbox (and any future surface
// that flips read-state) calls `refreshUnread()` after a server-truth mark-read so
// the badge stays authoritative WITHOUT prop-drilling through AuthGate.
//
// SERVER-TRUTH (D-02): the count is re-fetched from the owner-scoped endpoint
// (fetchUnreadCount) — never optimistically decremented. Real-time push is OUT of
// scope this phase (no websocket); a load / route-change / post-mutation refresh
// is sufficient (20-UI-SPEC § Nav Badge).

import { writable } from 'svelte/store';
import { fetchUnreadCount } from '$lib/api';

/** The current unread-alert count (0 ⇒ no badge). Seeded 0; refreshed from the server. */
export const unreadCount = writable<number>(0);

/**
 * Re-fetch the owner's unread count from the server and publish it to the store.
 * A failure leaves the last-known value untouched (the badge is a convenience
 * signal, not a security control — a transient fetch error must not zero it).
 * `f` is injectable for tests.
 */
export async function refreshUnread(f: typeof fetch = fetch): Promise<void> {
	try {
		const { count } = await fetchUnreadCount(f);
		unreadCount.set(Number.isFinite(count) && count > 0 ? count : 0);
	} catch {
		// Leave the prior value — don't flicker the badge to 0 on a network blip.
	}
}
