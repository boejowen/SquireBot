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

import type { RestoreResult } from '$lib/api';

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

/**
 * Map a successful restore reply to the officer-facing success copy (WR-01/WR-02).
 * Extracted pure so the load-bearing "the code is NOT shown in-browser" wording is
 * node-unit-testable without mounting the form.
 *
 * WR-02 is the load-bearing constraint: the re-minted guild code's plaintext is
 * printed to the server's journald ONLY (auth.MintCode, never the HTTP response),
 * so the copy must say the new code is retrieved SERVER-SIDE (logs / `mint-code`)
 * and must NOT imply it is shown here. The two RestoreResult outcomes:
 *   - code_mint_failed → the restore COMMITTED but the follow-on re-mint failed
 *     (WR-01): the guildie is restored but currently codeless; tell the officer to
 *     re-issue the code on the server.
 *   - new_code_issued  → a fresh code now EXISTS on the server (read it from the
 *     server logs / re-run `mint-code`) and must be handed off out-of-band.
 *
 * `label` is passed in (the helper stays pure / DOM-free). It is rendered by the
 * caller via Svelte's auto-escaping `{}` (never `{@html}`, T-15-28).
 */
export function restoreResultMessage(res: RestoreResult, label: string): string {
	if (res.code_mint_failed) {
		return `Restored ${label}, but re-minting the guild code failed — re-issue it on the server with \`mint-code\`.`;
	}
	return `Restored ${label}. A fresh guild code was minted on the SERVER (read it from the server logs / re-run \`mint-code\`) and hand it to them — it is not shown here.`;
}
