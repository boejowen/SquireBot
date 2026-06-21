// Default-landing redirect (Phase 30 / D-04). The 4-view consolidated home moved
// VERBATIM to /guild-views (D-03b). Phases 31-34 shipped the full 5-tab UI, so `/`
// now lands on the Characters tab (the milestone front door, the documented post-
// Phase-31 flip — v2.4 milestone audit 2026-06-21). The legacy grids remain reachable
// at /guild-views.
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
	redirect(307, '/characters'); // the 5-tab front door (legacy grids remain at /guild-views)
}
