// Pure eviction-form display helpers (CR-02). Extracted from EvictionForm.svelte
// so the grace-deadline formatting is node-unit-testable WITHOUT a DOM (the
// repo's established philosophy — see coin.ts / admin.ts; vitest runs node-only,
// no jsdom). The form .svelte is a thin renderer over this.
//
// CONTRACT (CR-02 root cause): the backend emits grace_until as unix epoch
// SECONDS (a JSON number — see webadmin/eviction.go: nowUnix()+EvictionGraceSeconds
// and store.EvictionGraceSeconds). JavaScript's `new Date(n)` interprets its
// numeric argument as MILLISECONDS since epoch, so feeding the raw epoch-seconds
// value (~1.78e9) yields ~20 days after 1970-01-01 ("Wed Jan 21 1970") instead of
// the intended ~30-days-from-now date. The seconds→ms multiply below is the fix.

/**
 * Format a unix epoch-SECONDS grace deadline as a human date string (e.g.
 * "Wed Jul 30 2026"). Multiplies by 1000 to convert seconds→ms before
 * constructing the Date (CR-02). A non-finite / NaN value falls back to the
 * raw value's string form rather than rendering "Invalid Date".
 */
export function graceDate(epochSeconds: number): string {
	if (!Number.isFinite(epochSeconds)) return String(epochSeconds);
	const d = new Date(epochSeconds * 1000);
	return Number.isNaN(d.getTime()) ? String(epochSeconds) : d.toDateString();
}
