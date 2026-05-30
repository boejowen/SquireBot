package readapi

// cors.go is the read API's CORS middleware (D-04). The static SvelteKit site
// (https://app.squirebot.quest, Cloudflare Pages — Plan 02 Task 1) and this API
// (https://api.squirebot.quest, Caddy → loopback 127.0.0.1:8090) are DIFFERENT
// origins, so the browser preflights any non-simple cross-origin request and
// blocks the response unless the API echoes an allowed origin. This middleware
// is the only cross-origin control in P14 (the data is intentionally public per
// D-04; P15's Discord login walls it).
//
// SECURITY (T-14.03-01): the allow-origin is the EXACT locked origin, never a
// wildcard — a wildcard leaks the data to any site and, combined with
// Access-Control-Allow-Credentials, is an outright spec violation the browser
// rejects. We deliberately do NOT set Access-Control-Allow-Credentials in P14
// (there are no cookies/sessions yet), so echoing the exact origin keeps the P15
// credentialed upgrade a one-line change (a wildcard could never carry
// credentials). `Vary: Origin` keeps any shared cache from serving one origin's
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
// the EXACT static-site origin (e.g. https://app.squirebot.quest), never a wildcard.
//
// It is mounted once around the whole mux in cmd/squirebot-server (so it travels
// with every route). The extra headers are harmless on the ingest/whoami routes —
// those keep their own bearer guard; CORS does not weaken them.
func CORS(allowOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", allowOrigin) // exact origin, never a wildcard
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		// Preflight: answer before the handler body, no content. The headers above
		// are already written, so the browser sees the allow decision on the 204.
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
