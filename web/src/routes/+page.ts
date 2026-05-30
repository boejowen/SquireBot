// The root route prerenders to a static index.html so the deploy has a real
// entry document (Cloudflare Pages serves /index.html for "/"). This is a
// page-level override of the layout's prerender=false default: it is SAFE
// because ssr=false (inherited) means prerendering emits only the empty
// client-hydration shell — NO data fetch runs at build time, so the
// 14-RESEARCH "cross-origin fetch during prerender" anti-pattern does not
// apply here. Data-driven view routes (added in Plan 14-04) keep the
// layout's prerender=false and render fully client-side via the 200.html
// SPA fallback.
export const prerender = true;
