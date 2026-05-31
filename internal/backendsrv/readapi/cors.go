package readapi

// cors.go is the read API's CORS middleware (D-04). The static SvelteKit site
// (https://squirebot.quest, served by Caddy on the same VPS — deploy decision
// 2026-05-30, switched from the planned Cloudflare Pages app. subdomain) and this
// API (https://api.squirebot.quest, Caddy → loopback 127.0.0.1:8090) are DIFFERENT
// origins, so the browser preflights any non-simple cross-origin request and
// blocks the response unless the API echoes an allowed origin. This middleware
// is the only cross-origin control in P14 (the data is intentionally public per
// D-04; P15's Discord login walls it).
//
// SECURITY (T-14.03-01 / T-15-10): the allow-origin is the EXACT locked origin,
// never a wildcard — a wildcard leaks the data to any site and, combined with
// Access-Control-Allow-Credentials, is an outright spec violation the browser
// rejects. P15 (D-05) turned the read API into a members-only surface: the
// session rides a cross-subdomain httpOnly cookie, so this middleware now ALSO
// sets Access-Control-Allow-Credentials:true (on both the actual response and
// the preflight) and admits POST for the write forms. Echoing the exact origin
// is what makes that credentialed upgrade safe — a wildcard could never carry
// credentials. `Vary: Origin` keeps any shared cache from serving one origin's
// CORS decision to another.
//
// DEPLOY-TIME VERIFICATION (Pitfall 5 / T-14.03-06): CORS is set ONCE, here in
// Go. The on-box Caddyfile fronting 443 MUST NOT also emit
// Access-Control-Allow-Origin — a duplicated header ("origin, origin") makes the
// browser reject the response. Verify on the VPS that Caddy's reverse_proxy block
// adds no CORS headers (operational step, mirroring P11's manual-deploy posture).

import "net/http"

// CORS wraps next so every response carries the cross-origin headers the static
// site needs and answers the OPTIONS preflight with 204 (no body). allowOrigin is
// the EXACT static-site origin (e.g. https://squirebot.quest), never a wildcard.
//
// It is mounted once around the whole mux in cmd/squirebot-server (so it travels
// with every route). The extra headers are harmless on the ingest/whoami routes —
// those keep their own bearer guard; CORS does not weaken them.
func CORS(allowOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", allowOrigin) // exact origin, never a wildcard
		// P15 (D-05 / T-15-10): the session cookie rides cross-subdomain
		// (squirebot.quest -> api.squirebot.quest, same registrable domain) on
		// credentialed fetches, so the API MUST advertise Allow-Credentials:true.
		// This is REQUIRED on BOTH the actual response AND the preflight, so it is
		// set BEFORE the OPTIONS short-circuit below. The wildcard-origin +
		// credentials combination is an outright spec violation the browser
		// rejects — which is exactly why the allow-origin above is the EXACT
		// locked origin, never a wildcard.
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		// IN-05: Add (append), not Set (overwrite) — the defensive idiom for Vary, so
		// a future handler or middleware layer that adds its own Vary value (e.g.
		// Accept-Encoding from a compressor) is not clobbered. Nothing upstream sets
		// Vary today, so this is behavior-neutral now but future-proof.
		w.Header().Add("Vary", "Origin")
		// POST added for the P15 write forms (bank-coin / eviction / admin-mgmt).
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		// Sessions ride the httpOnly cookie, NOT an Authorization header, so the
		// allowed request headers stay Content-Type only (no custom auth header).
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		// Preflight: answer before the handler body, no content. The headers above
		// (incl. Allow-Credentials) are already written, so the browser sees the
		// full allow decision on the 204.
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
