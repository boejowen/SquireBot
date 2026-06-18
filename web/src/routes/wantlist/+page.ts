// /wantlist → /wishlist redirect stub (Phase 30 / D-02). The wantlist surface
// rehomes onto /wishlist; this keeps old bookmarks from 404ing. ssr=false →
// client-side load; the router catches the thrown redirect + updates the address
// bar. Leave redirect() uncaught — an exception handler would swallow the thrown
// Redirect (RESEARCH Pitfall 2); keep prerender=false.
import { redirect } from '@sveltejs/kit';

export const ssr = false;
export const prerender = false;

export function load() {
	redirect(308, '/wishlist');
}
