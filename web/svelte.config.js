import adapter from '@sveltejs/adapter-static';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	compilerOptions: {
		// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
		runes: ({ filename }) => filename.split(/[/\\]/).includes('node_modules') ? undefined : true
	},
	kit: {
		// SPA mode: adapter-static with a fallback page (no per-route prerender).
		// The 4 views are data-driven from the live read API at runtime, so we
		// ship one fallback document and let client routing take over.
		// Deploy target is LOCKED to Cloudflare Pages at the root subdomain
		// app.squirebot.quest (14-02-PLAN Task 1 step 2) — a root origin, so
		// kit.paths.base is intentionally NOT set (no GH-Pages base-path tax),
		// and it is the CORS allow-origin Plan 03 grants on the Go read API.
		adapter: adapter({ fallback: '200.html' })
	}
};

export default config;
