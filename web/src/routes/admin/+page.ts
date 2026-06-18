// /admin → /settings redirect stub (Phase 30 / D-02). The officer admin panels
// rehome behind the officer-gated Settings → Admin section; a non-officer who
// bookmarked /admin lands on /settings where that section simply isn't rendered
// (the Go API stays the real boundary — T-30-03). ssr=false → client-side load;
// the router catches the thrown redirect + updates the address bar. Leave it
// uncaught — an exception handler would swallow the thrown Redirect (Pitfall 2);
// prerender=false.
import { redirect } from '@sveltejs/kit';

export const ssr = false;
export const prerender = false;

export function load() {
	redirect(308, '/settings');
}
