// /notifications → /wishlist redirect stub (Phase 30 / D-02/D-07). The
// notification inbox + prefs rehome onto /wishlist (one mental model: every alert
// is a wishlist ping). ssr=false → client-side load; the router catches the thrown
// redirect + updates the address bar. Leave it uncaught — an exception handler
// would swallow the thrown Redirect (Pitfall 2); prerender=false.
import { redirect } from '@sveltejs/kit';

export const ssr = false;
export const prerender = false;

export function load() {
	redirect(308, '/wishlist');
}
