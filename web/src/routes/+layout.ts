// SPA configuration (14-02-PLAN Task 1 step 4; 14-RESEARCH anti-pattern:
// do NOT prerender data-driven view routes — the data isn't known at build
// time and a cross-origin fetch during prerender would fail). adapter-static
// emits a single fallback document (200.html) and the client router renders
// every route in the browser.
export const ssr = false;
export const prerender = false;
