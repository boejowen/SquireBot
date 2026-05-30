// Package buildinfo carries the backend's build-time version and derives the
// identifying, contactable User-Agent the polite HTTP client (enrich/politefetch)
// sends on every outbound request to the community-run PigParse + P1999 wiki
// services.
//
// Version is settable at link time, mirroring the watcher's main.Version
// (cmd/squirebot/build_constants.go):
//
//	go build -ldflags "-X github.com/boejowen/SquireBot/internal/backendsrv/buildinfo.Version=v2.0.0" ./cmd/squirebot-server
//
// The default "dev" fallback (D-11) keeps an un-stamped build producing a valid,
// identifying UA — the SC-3 politeness requirement is "identifying + contactable",
// which the GitHub URL satisfies regardless of the Version value.
//
// This lives in its own package (rather than as a var in package main) so the
// politefetch client — which is NOT in package main — can import the Version
// without an import cycle and without cmd/squirebot-server depending on
// politefetch's internals. Wiring the -ldflags into the actual build/deploy is
// deferred to Plan 05 / the deploy step; this package just provides the var +
// helper.
package buildinfo

// Version is the backend build version, set at link time via
//
//	-ldflags "-X github.com/boejowen/SquireBot/internal/backendsrv/buildinfo.Version=..."
//
// The "dev" default keeps an un-stamped build's User-Agent valid + identifying.
var Version = "dev"

// UserAgent returns the identifying, contactable User-Agent for every outbound
// request. It mirrors the Apps Script DEFAULT_USER_AGENT shape exactly
// (apps-script/src/lib/politeFetch.ts):
//
//	SquireBot/<Version> (+https://github.com/boejowen/SquireBot)
//
// The product token identifies the client; the GitHub URL is the contactable
// reference an external-service operator can use to attribute + reach the
// maintainer (SC-3).
func UserAgent() string {
	return "SquireBot/" + Version + " (+https://github.com/boejowen/SquireBot)"
}
