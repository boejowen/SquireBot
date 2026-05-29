// Command squirebot-server is the SquireBot v2 backend: a single static Go
// binary (cross-compiled CGO_ENABLED=0 GOOS=linux GOARCH=amd64 for the Hetzner
// US VPS) that serves the authenticated ingest API, runs goose migrations on
// startup, and hosts the in-process scheduler skeleton. Per the 11-01 verdict it
// is HAND-ROLLED stdlib net/http — NOT PocketBase.
//
// Subcommand dispatch mirrors the watcher's cmd/squirebot/main.go os.Args shape:
// sniff os.Args[1], run the CLI subcommand, exit early; otherwise fall through
// to serve. The maintainer-run CLI subcommands are out-of-band (no HTTP surface
// in P11 — that is P15):
//
//	squirebot-server mint-code   --owner <label>     # print a guild code ONCE (D-05)
//	squirebot-server revoke-code <id|label>          # disable a guild code (D-09)
//	squirebot-server serve --addr 127.0.0.1:8090 --db /var/lib/squirebot/squirebot.db
//
// "Off Google" (CLAUDE.md): the backend has NO Google/OAuth/Sheets dependency —
// no OAuth client id/secret, no -X main.OAuthClientID ldflags. The only secret
// the server ever handles is the guild-code hash (in SQLite).
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
	"github.com/boejowen/SquireBot/internal/backendsrv/ingest"
	"github.com/boejowen/SquireBot/internal/backendsrv/logging"
	"github.com/boejowen/SquireBot/internal/backendsrv/migrations"
	"github.com/boejowen/SquireBot/internal/backendsrv/scheduler"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

const (
	defaultAddr = "127.0.0.1:8090"                 // loopback only — Caddy fronts 443 (11-06)
	defaultDB   = "/var/lib/squirebot/squirebot.db" // matches the RESEARCH systemd unit
)

func main() {
	os.Exit(run(os.Args[1:]))
}

// run is the testable entrypoint: main calls os.Exit(run(os.Args[1:])). It
// dispatches the subcommand (mint-code / revoke-code / serve) and returns the
// process exit code. Splitting the body out of main lets a test drive the mint
// dispatch against a temp DB and assert the exit code without spawning a process.
func run(args []string) int {
	// Sniff args[0] (= os.Args[1]) for a CLI subcommand; the two maintainer
	// subcommands run and exit BEFORE the server starts (mirroring the watcher's
	// --uninstall-wipe-credentials / --quit early-exit).
	if len(args) >= 1 {
		switch args[0] {
		case "mint-code":
			return runMint(args[1:])
		case "revoke-code":
			return runRevoke(args[1:])
		case "serve":
			return runServe(args[1:])
		}
	}
	// Default (no subcommand or an unknown one) → serve with defaults. Treat a
	// bare invocation as `serve` so systemd's `squirebot-server serve …` and a
	// plain run both work.
	return runServe(args)
}

// runMint implements `mint-code --owner <label>`: open the DB, ensure the schema
// exists (goose.Up so a fresh box can mint before the first serve), mint a code
// (MintCode prints the plaintext to stdout ONCE — D-05), and exit 0.
func runMint(args []string) int {
	fs := flag.NewFlagSet("mint-code", flag.ContinueOnError)
	owner := fs.String("owner", "", "owner label for the new guild code (required)")
	dbPath := fs.String("db", defaultDB, "path to the SQLite database file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *owner == "" {
		fmt.Fprintln(os.Stderr, "mint-code: --owner <label> is required")
		return 2
	}

	db, err := openMigratedDB(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mint-code: %v\n", err)
		return 1
	}
	defer db.Close()

	// MintCode prints the plaintext code to stdout once (and returns it). We do
	// not re-print or log it (V7) — the stdout print IS the one-time disclosure.
	if _, err := auth.MintCode(db, *owner); err != nil {
		fmt.Fprintf(os.Stderr, "mint-code: %v\n", err)
		return 1
	}
	return 0
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

// runServe is the default path: set up the Linux stdout logger, open the DB, run
// goose.Up on startup (D-10 — idempotent, no-ops on an up-to-date DB), register
// the in-process scheduler skeleton (no real jobs — P12), and serve POST
// /api/v1/ingest on a net/http ServeMux bound to loopback. The server runs until
// SIGINT/SIGTERM, which cancels the root context and unwinds the scheduler.
func runServe(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", defaultAddr, "loopback address to bind (Caddy fronts 443)")
	dbPath := fs.String("db", defaultDB, "path to the SQLite database file")
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

	// Root context cancelled on SIGINT/SIGTERM — drives a clean shutdown of the
	// scheduler goroutine and the HTTP server.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// In-process scheduler SKELETON — registers NO real jobs (P12 fills it in).
	scheduler.Start(ctx)

	// Route the single network surface this milestone introduces. Go 1.22+
	// method+pattern routing ("POST /api/v1/ingest"); the handler composes the
	// bearer guard + bind + atomic replace (11-02/03/04).
	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/ingest", ingest.New(auth.New(db), db))

	srv := &http.Server{
		Addr:    *addr,
		Handler: mux,
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
