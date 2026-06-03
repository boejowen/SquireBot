// Command squirebot-server is the SquireBot v2 backend: a single static Go
// binary (cross-compiled CGO_ENABLED=0 GOOS=linux GOARCH=amd64 for the Hetzner
// US VPS) that serves the authenticated ingest API, runs goose migrations on
// startup, and hosts the in-process scheduler skeleton. Per the 11-01 verdict it
// is HAND-ROLLED stdlib net/http — NOT PocketBase.
//
// Subcommand dispatch mirrors the watcher's cmd/squirebot/main.go os.Args shape:
// sniff os.Args[1], run the CLI subcommand, exit early; otherwise fall through
// to serve. The maintainer-run CLI subcommands are out-of-band (no HTTP surface
// in P11/P12 — that is P15). run-job is the D-7 parity-check entrypoint, the Go
// parallel to the Sheet's "Refresh … Now" menu items: it invokes one enrichment
// job once on demand (run on the box) and exits.
//
//	squirebot-server revoke-code    <id|label>       # disable a guild code (D-09; ops backstop — self-service mint is the /account web path, LINK-06)
//	squirebot-server run-job        pigparse|wiki    # run one enrichment job once (D-7 parity)
//	squirebot-server set-owner-floor <discord-id>    # seed the un-removable owner-floor + bootstrap officer (15-02 / D-08)
//	squirebot-server serve --addr 127.0.0.1:8090 --db /var/lib/squirebot/squirebot.db
//
// v2.0 "Off-the-cloud-suite" invariant (CLAUDE.md): the backend introduces NO
// dependency on the retired cloud-auth / spreadsheet APIs — no client id/secret,
// no -X main.ClientID ldflags. The only secret the server ever handles is the
// guild-code hash (in SQLite). The import block + `go list -deps` stay clean of
// those packages (enforced by the threat-model T-12.05-05 grep + the static
// linux/amd64 cross-compile).
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/boejowen/SquireBot/internal/backendsrv/auth"
	"github.com/boejowen/SquireBot/internal/backendsrv/enrich/jobs"
	"github.com/boejowen/SquireBot/internal/backendsrv/enrich/politefetch"
	"github.com/boejowen/SquireBot/internal/backendsrv/ingest"
	"github.com/boejowen/SquireBot/internal/backendsrv/logging"
	"github.com/boejowen/SquireBot/internal/backendsrv/migrations"
	"github.com/boejowen/SquireBot/internal/backendsrv/readapi"
	"github.com/boejowen/SquireBot/internal/backendsrv/scheduler"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
	"github.com/boejowen/SquireBot/internal/backendsrv/webadmin"
	"github.com/boejowen/SquireBot/internal/backendsrv/webauth"
)

const (
	defaultAddr = "127.0.0.1:8090"                  // loopback only — Caddy fronts 443 (11-06)
	defaultDB   = "/var/lib/squirebot/squirebot.db" // matches the RESEARCH systemd unit
	// defaultCORSOrigin is the apex origin the static SvelteKit site is served from
	// — Caddy on the same VPS at https://squirebot.quest (deploy decision 2026-05-30:
	// switched from the planned Cloudflare Pages app. subdomain to apex-on-Caddy; see
	// 14-CONTEXT/STATE). The read API echoes this exact origin in
	// Access-Control-Allow-Origin (D-04) — never a wildcard. Overridable via
	// -cors-origin for a staging/preview deploy.
	defaultCORSOrigin = "https://squirebot.quest"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

// run is the testable entrypoint: main calls os.Exit(run(os.Args[1:])). It
// dispatches the subcommand (revoke-code / run-job / set-owner-floor / serve) and
// returns the process exit code. Splitting the body out of main lets a test
// exercise a subcommand against a temp DB and assert the exit code without spawning
// a process. (LINK-06: the v1 `mint-code` CLI is gone — self-service minting is the
// session-gated /account web path; `revoke-code` is retained as the ops backstop.)
func run(args []string) int {
	// Sniff args[0] (= os.Args[1]) for a CLI subcommand; the two maintainer
	// subcommands run and exit BEFORE the server starts (mirroring the watcher's
	// --uninstall-wipe-credentials / --quit early-exit).
	if len(args) >= 1 {
		switch args[0] {
		case "revoke-code":
			return runRevoke(args[1:])
		case "run-job":
			return runJobCmd(args[1:])
		case "set-owner-floor":
			return runSetOwnerFloor(args[1:])
		case "serve":
			return runServe(args[1:])
		}
	}
	// Default (no subcommand or an unknown one) → serve with defaults. Treat a
	// bare invocation as `serve` so systemd's `squirebot-server serve …` and a
	// plain run both work.
	return runServe(args)
}

// runRevoke implements `revoke-code <id|label>`: open the DB, ensure the schema
// exists, disable the matching guild_code row(s) (idempotent — D-09), and exit 0.
//
// The id/label positional may appear before OR after the --db flag. Go's flag
// package stops parsing at the first non-flag token, so we separate the leading
// flag tokens from the positional ourselves before parsing — otherwise
// `revoke-code bob --db X` would leave --db unparsed (it would silently fall
// back to the default DB path).
func runRevoke(args []string) int {
	flagArgs, positionals := splitFlagsAndPositionals(args)

	fs := flag.NewFlagSet("revoke-code", flag.ContinueOnError)
	dbPath := fs.String("db", defaultDB, "path to the SQLite database file")
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if len(positionals) < 1 || positionals[0] == "" {
		fmt.Fprintln(os.Stderr, "revoke-code: an id or owner label argument is required")
		return 2
	}
	idOrLabel := positionals[0]

	db, err := openMigratedDB(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "revoke-code: %v\n", err)
		return 1
	}
	defer db.Close()

	if err := auth.RevokeCode(db, idOrLabel); err != nil {
		fmt.Fprintf(os.Stderr, "revoke-code: %v\n", err)
		return 1
	}
	fmt.Printf("Revoked guild code(s) matching %q (no-op if already revoked or absent).\n", idOrLabel)
	return 0
}

// runJobCmd implements `run-job pigparse|wiki`: open + migrate the DB (goose.Up so
// a fresh box works), then invoke the named enrichment job ONCE on demand with the
// production politefetch.Fetch and exit. This is the D-7 parity-check entrypoint —
// the maintainer runs it on the box to populate/refresh the dimension tables out
// of band, paralleling the Sheet's "Refresh … Now" menu items. It adds NO HTTP
// surface (mirrors mint-code/revoke-code) and NO new cloud-auth/spreadsheet dep.
//
// The job name is a positional that may appear before OR after the --db flag
// (splitFlagsAndPositionals separates them, same as revoke-code). Exactly one job
// name is required: a missing, unknown, or extra positional is a usage error
// (exit 2). The job's own slog output (counts/status) is visible because we call
// logging.Setup(); a SIGINT/SIGTERM cancels ctx so a long wiki crawl unwinds
// cleanly (the jobs' ctx-aware sleeps + politeFetch backoff respond to it).
func runJobCmd(args []string) int {
	flagArgs, positionals := splitFlagsAndPositionals(args)

	fs := flag.NewFlagSet("run-job", flag.ContinueOnError)
	dbPath := fs.String("db", defaultDB, "path to the SQLite database file")
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if len(positionals) != 1 || positionals[0] == "" {
		// Missing, empty, or more than one job name → usage error. Requiring
		// exactly one keeps `run-job pigparse wiki` from silently running only one.
		fmt.Fprintln(os.Stderr, "run-job: exactly one job name is required: pigparse | wiki")
		return 2
	}
	name := positionals[0]
	if name != "pigparse" && name != "wiki" {
		fmt.Fprintf(os.Stderr, "run-job: unknown job %q (want: pigparse | wiki)\n", name)
		return 2
	}

	logging.Setup() // JSON slog to stdout so the job's counts/status are visible

	db, err := openMigratedDB(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "run-job: %v\n", err)
		return 1
	}
	defer db.Close()

	// SIGINT/SIGTERM cancels ctx so a long wiki run (the per-page 1s sleeps +
	// politeFetch backoff are all ctx-aware) aborts cleanly instead of wedging.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch name {
	case "pigparse":
		err = jobs.RunPigparse(ctx, db, politefetch.Fetch)
	case "wiki":
		err = jobs.RunWiki(ctx, db, politefetch.Fetch)
	}
	if err != nil {
		slog.Error("run-job failed", "job", name, "err", err)
		return 1
	}
	slog.Info("run-job complete", "job", name)
	return 0
}

// runServe is the default path: set up the Linux stdout logger, open the DB, run
// goose.Up on startup (D-10 — idempotent, no-ops on an up-to-date DB), register
// the in-process scheduler with the two real enrichment jobs (P12), and serve POST
// /api/v1/ingest on a net/http ServeMux bound to loopback. The server runs until
// SIGINT/SIGTERM, which cancels the root context and unwinds the scheduler.
func runServe(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", defaultAddr, "loopback address to bind (Caddy fronts 443)")
	dbPath := fs.String("db", defaultDB, "path to the SQLite database file")
	corsOrigin := fs.String("cors-origin", defaultCORSOrigin,
		"exact origin allowed to read the API via CORS (the static site origin; never a wildcard)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	logging.Setup() // JSON slog to stdout (journald captures it) — D-10

	db, err := store.Open(*dbPath)
	if err != nil {
		slog.Error("serve: open db failed", "db", *dbPath, "err", err)
		return 1
	}
	defer db.Close()

	// goose.Up on startup (D-10 / BACKEND-02). Idempotent: a no-op on an
	// up-to-date DB. "Deploy = drop the new binary + restart" relies on this.
	if err := migrations.RunMigrations(db); err != nil {
		slog.Error("serve: migrations failed", "err", err)
		return 1
	}

	// Root context cancelled on SIGINT/SIGTERM — triggers a clean shutdown of the
	// scheduler goroutine and the HTTP server.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// In-process scheduler — registers the real PigParse + wiki jobs (P12). It
	// reads each job's job_run cursor for the due-check, runs an immediate check
	// pass on startup, and advances the cursor after each run; ctx cancel unwinds
	// it on SIGINT/SIGTERM.
	scheduler.Start(ctx, db)

	// Route the network surfaces. Go 1.22+ method+pattern routing. The ingest
	// handler composes the bearer guard + bind + atomic replace (11-02/03/04); the
	// whoami handler (13-01 / D-4) is the authed, side-effect-free validation
	// endpoint the watcher onboarding calls to verify a pasted guild code. Both
	// reuse the SAME bearer guard (a second thin auth.New(db) wrapper is fine).
	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/ingest", ingest.New(auth.New(db), db))
	mux.Handle("GET /api/v1/whoami", ingest.NewWhoami(auth.New(db), db))

	// P15 Discord-login auth routes (D-01..D-05). These are registered UNGATED —
	// login/callback/whoami-web/logout MUST be reachable WITHOUT a session (they
	// are how a visitor obtains one). The Discord OAuth config comes from the env
	// (DISCORD_* systemd secrets); the client secret is backend-only (never the
	// static bundle, never logged — T-15-09). whoami-web is the always-200
	// AuthGate feed; the other read routes below are session-gated.
	cfg := webauth.ConfigFromEnv()
	mux.Handle("GET /api/v1/auth/login", webauth.LoginHandler(db, cfg))
	mux.Handle("GET /api/v1/auth/callback", webauth.CallbackHandler(db, cfg))
	mux.Handle("GET /api/v1/auth/whoami-web", webauth.WhoamiWebHandler(db, cfg))
	mux.Handle("POST /api/v1/auth/logout", webauth.LogoutHandler(db, cfg))

	// P14 read API (BACKEND-05): the four consolidated views + a small meta feed.
	// P15 / D-01 (T-15-11) closes P14's public-but-unlisted stopgap: EVERY read
	// route is now wrapped in webauth.RequireSession, so a request with no valid
	// session cookie gets 401 — the membership gate is at the API, not just the
	// (bypassable) SvelteKit frontend. NO read route is left un-gated. The ingest
	// (bearer) + whoami (bearer) + the four auth routes above stay ungated.
	// readapi composes Plan 14-01's compute package over this read-side store.
	st := store.NewStore(db)
	mux.Handle("GET /api/v1/meta", webauth.RequireSession(db, readapi.NewMeta(st)))
	mux.Handle("GET /api/v1/views/view", webauth.RequireSession(db, readapi.NewViews(st, "view")))
	mux.Handle("GET /api/v1/views/gear_check", webauth.RequireSession(db, readapi.NewViews(st, "gear_check")))
	mux.Handle("GET /api/v1/views/spell_check", webauth.RequireSession(db, readapi.NewViews(st, "spell_check")))
	mux.Handle("GET /api/v1/views/bank", webauth.RequireSession(db, readapi.NewViews(st, "bank")))

	// P15 / 15-03 write surface (ADMIN-04/05/06). The SERVER is the authorization
	// boundary (D-01) — these gates are the real ones; the frontend (15-04/15-05)
	// only gates UX. The gate choice is load-bearing (D-01/D-12):
	//
	//   - OFFICER-ONLY (webauth.RequireOfficer): eviction + officer-management. A
	//     valid-but-non-officer session → 403 not_authorized. The officer-only
	//     MUTATORS additionally re-authorize INSIDE their write tx (store.*Tx /
	//     the eviction handler's store.IsOfficerTx re-check) to close the v1 WR-04
	//     TOCTOU window — this middleware is the cheap outer gate, not a substitute.
	//   - LOGIN-ONLY (webauth.RequireSession): bank-coin. D-12 (B-1) — ADMIN-05 says
	//     "authenticated", so ANY signed-in member may record the shared bank's coin;
	//     the coin handler NEVER checks officer status (proven by
	//     TestCoinSet_NonOfficerCanWrite). The writer's discord id is audited.
	//
	// All are POST/GET on the same mux, so the outer CORS wrap (credential-aware,
	// 15-02) covers them too.
	mux.Handle("GET /api/v1/admin/officers", webauth.RequireOfficer(db, webadmin.OfficersListHandler(db)))
	mux.Handle("POST /api/v1/admin/officers/add", webauth.RequireOfficer(db, webadmin.OfficerAddHandler(db)))
	mux.Handle("POST /api/v1/admin/officers/remove", webauth.RequireOfficer(db, webadmin.OfficerRemoveHandler(db)))
	mux.Handle("GET /api/v1/admin/evictable", webauth.RequireOfficer(db, webadmin.EvictableListHandler(db)))
	mux.Handle("GET /api/v1/admin/restorable", webauth.RequireOfficer(db, webadmin.RestorableListHandler(db)))
	mux.Handle("GET /api/v1/admin/eviction/preview", webauth.RequireOfficer(db, webadmin.EvictionPreviewHandler(db)))
	mux.Handle("POST /api/v1/admin/evict", webauth.RequireOfficer(db, webadmin.EvictHandler(db)))
	mux.Handle("POST /api/v1/admin/eviction/restore", webauth.RequireOfficer(db, webadmin.RestoreHandler(db)))

	// Bank-coin — LOGIN-ONLY (D-12): RequireSession, NOT RequireOfficer.
	mux.Handle("GET /api/v1/coin/bank-toons", webauth.RequireSession(db, webadmin.BankToonsHandler(db)))
	mux.Handle("POST /api/v1/coin", webauth.RequireSession(db, webadmin.CoinSetHandler(db)))

	// Char-meta — LOGIN-ONLY (D-03): RequireSession. Any signed-in member sets any
	// existing character's class/level/race/is_bank_toon (non-sensitive shared data,
	// the bank-coin precedent). NEVER RequireOfficer — the officer-only block is
	// above (lines 319-326); char-meta belongs with the login-only coin block.
	mux.Handle("GET /api/v1/char/meta-list", webauth.RequireSession(db, webadmin.CharMetaListHandler(db)))
	mux.Handle("POST /api/v1/char/meta", webauth.RequireSession(db, webadmin.CharMetaSetHandler(db)))

	// Self-service watcher codes (Phase 17 / LINK-01/03/05 / D-09) — LOGIN-ONLY:
	// RequireSession, NEVER RequireOfficer. Every signed-in member mints/lists/
	// revokes their OWN codes; the owner is derived server-side from the Discord
	// session (D-02), so the request body carries no owner. The replaced v1
	// `mint-code --owner <label>` free-text CLI path is gone (LINK-06).
	mux.Handle("POST /api/v1/account/codes", webauth.RequireSession(db, webadmin.MintOwnCodeHandler(db)))
	mux.Handle("GET /api/v1/account/codes", webauth.RequireSession(db, webadmin.ListOwnCodesHandler(db)))
	mux.Handle("POST /api/v1/account/codes/revoke", webauth.RequireSession(db, webadmin.RevokeOwnCodeHandler(db)))

	// Wantlist (Phase 19 / WANT-01/02 / D-02) — LOGIN-ONLY (RequireSession, NEVER
	// RequireOfficer): every signed-in member manages their OWN wantlist; the owner
	// is derived server-side from the Discord session, never the request body.
	mux.Handle("GET /api/v1/wantlist", webauth.RequireSession(db, webadmin.ListOwnWantsHandler(db)))
	mux.Handle("POST /api/v1/wantlist", webauth.RequireSession(db, webadmin.AddWantHandler(db)))
	mux.Handle("POST /api/v1/wantlist/remove", webauth.RequireSession(db, webadmin.RemoveOwnWantHandler(db)))
	// D-10 full-catalog item search — session-gated like the view endpoints
	// (readapi, takes the read-side st):
	mux.Handle("GET /api/v1/items/search", webauth.RequireSession(db, readapi.NewItemSearch(st)))

	// Wrap the WHOLE mux in CORS so the allow-origin header travels with every
	// route (D-04). P15 made CORS credential-aware (Access-Control-Allow-Credentials:
	// true + POST) so the cross-subdomain session cookie rides the credentialed
	// fetches (D-05). The ingest/whoami routes are functionally unaffected — they
	// still require their bearer guard; the extra CORS headers are harmless on a
	// POST/authed-GET. CORS is set ONCE here, in Go: the on-box Caddyfile fronting
	// 443 MUST NOT also emit Access-Control-Allow-Origin — a duplicated header
	// makes the browser reject the response (Pitfall 5 / T-14.03-06). Verify on the
	// VPS that Caddy's reverse_proxy block adds no CORS headers (deploy-time check,
	// mirroring P11's manual-deploy posture).
	//
	// LIVE DEPLOY ENV (set on the box, not here): SQUIREBOT_WEB_ORIGIN=
	// https://squirebot.quest (the W-4 callback redirect target) + SQUIREBOT_COOKIE_DOMAIN=
	// squirebot.quest (the cross-subdomain cookie scope). cors-origin already
	// defaults to the apex.
	srv := &http.Server{
		Addr:    *addr,
		Handler: readapi.CORS(*corsOrigin, mux),
	}

	// Run ListenAndServe in a goroutine so we can wait on ctx for shutdown.
	serveErr := make(chan error, 1)
	go func() {
		slog.Info("squirebot-server listening", "addr", *addr, "db", *dbPath, "pid", os.Getpid())
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if err != nil && err != http.ErrServerClosed {
			slog.Error("serve: listen failed", "err", err)
			return 1
		}
		return 0
	case <-ctx.Done():
		slog.Info("squirebot-server shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("serve: graceful shutdown failed", "err", err)
			return 1
		}
		return 0
	}
}

// splitFlagsAndPositionals separates leading-dash flag tokens (and their values)
// from bare positional arguments, so a subcommand's positional can appear in any
// position relative to its flags. It understands `--flag=value` (one token) and
// `--flag value` (two tokens) for the flags this binary defines (currently only
// --db, which takes a value). A token starting with "-" is treated as a flag; if
// it is not in `--flag=value` form, the following token is consumed as its value.
//
// This is a deliberately small shim for the backend's tiny CLI surface (only
// --db takes a value); it is not a general flag parser. The actual validation
// still happens in flag.FlagSet.Parse(flagArgs).
func splitFlagsAndPositionals(args []string) (flagArgs, positionals []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if len(a) > 0 && a[0] == '-' {
			flagArgs = append(flagArgs, a)
			// If it's not --flag=value and a value token follows, consume it as
			// the flag's value (all value-taking flags here are string flags).
			if !containsEquals(a) && i+1 < len(args) {
				i++
				flagArgs = append(flagArgs, args[i])
			}
			continue
		}
		positionals = append(positionals, a)
	}
	return flagArgs, positionals
}

// containsEquals reports whether s contains an '=' (i.e. is a --flag=value token).
func containsEquals(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '=' {
			return true
		}
	}
	return false
}

// openMigratedDB opens the DB and runs goose.Up so the CLI subcommands operate
// on an existing schema (a fresh box can mint a code before the first serve).
func openMigratedDB(dbPath string) (*sql.DB, error) {
	db, err := store.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db %s: %w", dbPath, err)
	}
	if err := migrations.RunMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}
	return db, nil
}
