// Default-landing redirect (Phase 30 / D-04). The 4-view consolidated home moved
// VERBATIM to /guild-views (D-03b); during the stub window `/` resolves to that
// functional surface, NOT the Characters "coming soon" placeholder (greeting an
// active user with an empty stub is the worse experience). Once Phase 31 ships the
// Characters tab, flip this target to /characters (one-line change).
//
// ssr=false (inherited) → the load runs client-side; the SvelteKit client router
// catches the thrown redirect and updates the address bar (verified against
// @sveltejs/kit 2.61.1). prerender=false: do NOT prerender a redirect page (this
// REPLACES the former prerendered apex). Leave redirect() uncaught — wrapping it
// in an exception handler would swallow the thrown Redirect (RESEARCH Pitfall 2).
import { redirect } from '@sveltejs/kit';

export const ssr = false;
export const prerender = false;

export function load() {
	redirect(307, '/guild-views'); // 307 temporary — flips to /characters post-Phase-31
}
