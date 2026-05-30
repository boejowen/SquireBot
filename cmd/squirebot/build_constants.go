package main

// Build-time constants, populated at link time via -ldflags="-X main.<name>=...".
//
// Phase 13 (WATCH-09 / D-2) GUTTED the OAuth/Picker constants — the re-targeted
// watcher bakes in NO Google secret. Only two values remain:
//
//	Version        — the watcher build version, stamped by release.yml
//	                 (-X main.Version=<tag>); travels in every ingest POST + the
//	                 User-Agent so the backend's 426 min-version gate can read it.
//	BackendBaseURL — the canonical backend host. The hardcoded default below IS
//	                 the production target; release.yml MAY override it via
//	                 -X main.BackendBaseURL=... (belt-and-braces), and a guildie
//	                 MAY override it per-machine via the config.json
//	                 backend_base_url field (advanced/self-host only).
var (
	Version        = "0.1.0-dev"
	BackendBaseURL = "https://api.squirebot.quest"
)
