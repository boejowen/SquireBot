package main

// ownerfloor.go implements `squirebot-server set-owner-floor <discord-id>` (15-02
// / D-08): the deploy-time CLI that designates the maintainer's Discord USER id
// as the un-removable owner-floor AND the first/bootstrap officer. It mirrors the
// revoke-code subcommand shape (splitFlagsAndPositionals so the positional may
// appear before OR after --db; openMigratedDB so a fresh box works), and calls
// the 15-01 store.SetOwnerFloor (which upserts app_config['owner_floor_discord_id'],
// seeds a placeholder web_user so the guild_admins FK holds pre-login, and
// idempotently inserts the floor into guild_admins).
//
// Run ONCE on the box at deploy:  squirebot-server set-owner-floor <discord-user-id>
// (the id is a Discord snowflake — NOT a secret; nothing sensitive is printed or
// logged here).

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// runSetOwnerFloor implements the set-owner-floor subcommand. A missing/empty
// positional <discord-id> is a usage error (exit 2); a store/DB failure exits 1;
// success prints a confirmation and exits 0.
func runSetOwnerFloor(args []string) int {
	flagArgs, positionals := splitFlagsAndPositionals(args)

	fs := flag.NewFlagSet("set-owner-floor", flag.ContinueOnError)
	dbPath := fs.String("db", defaultDB, "path to the SQLite database file")
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if len(positionals) < 1 || positionals[0] == "" {
		fmt.Fprintln(os.Stderr, "set-owner-floor: a discord user id is required")
		return 2
	}
	discordID := positionals[0]

	db, err := openMigratedDB(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "set-owner-floor: %v\n", err)
		return 1
	}
	defer db.Close()

	if err := store.SetOwnerFloor(context.Background(), db, discordID, time.Now().Unix()); err != nil {
		fmt.Fprintf(os.Stderr, "set-owner-floor: %v\n", err)
		return 1
	}
	fmt.Printf("owner-floor set to %s (also bootstrap officer)\n", discordID)
	return 0
}
