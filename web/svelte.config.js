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
		// Deploy target: served by Caddy on the VPS at the apex https://squirebot.quest
		// (deploy decision 2026-05-30 — switched from the planned Cloudflare Pages
		// app. subdomain). It's a root origin, so kit.paths.base is intentionally NOT
		// set, and the apex is the CORS allow-origin the Go read API grants.
		adapter: adapter({ fallback: '200.html' })
	}
};

export default config;
