package main

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/boejowen/SquireBot/internal/backendsrv/migrations"
	"github.com/boejowen/SquireBot/internal/backendsrv/readapi"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
	"github.com/boejowen/SquireBot/internal/backendsrv/webadmin"
	"github.com/boejowen/SquireBot/internal/backendsrv/webauth"
)

// TestRun_RevokeDispatch: revoke-code disables an existing code (exit 0) and the
// row is no longer active. The v1 `mint-code` CLI is gone (LINK-06 — self-service
// minting is the /account web path), so this test seeds a code directly via the
// store before exercising the retained revoke-code ops backstop.
func TestRun_RevokeDispatch(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "squirebot-revoke.db")

	// Seed an owner + active guild_code labeled "bob" directly (no mint-code CLI).
	seedActiveCode(t, dbPath, "bob")

	if code := run([]string{"revoke-code", "bob", "--db", dbPath}); code != 0 {
		t.Fatalf("revoke exit = %d, want 0", code)
	}

	db := openForAssert(t, dbPath)
	defer db.Close()
	var active int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM guild_code gc
		JOIN owner o ON o.id = gc.owner_id
		WHERE o.label = ? AND gc.disabled_at IS NULL`, "bob").Scan(&active); err != nil {
		t.Fatalf("query: %v", err)
	}
	if active != 0 {
		t.Errorf("active guild_code rows for bob after revoke = %d, want 0", active)
	}
}

// seedActiveCode opens (and migrates) the DB at dbPath and inserts an owner labeled
// `label` plus one active guild_code for it — the fixture the revoke-code dispatch
// test needs now that the mint-code CLI is gone (LINK-06).
func seedActiveCode(t *testing.T, dbPath, label string) {
	t.Helper()
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("seedActiveCode: open db: %v", err)
	}
	defer db.Close()
	if err := migrations.RunMigrations(db); err != nil {
		t.Fatalf("seedActiveCode: migrate: %v", err)
	}
	res, err := db.Exec(`INSERT INTO owner (label) VALUES (?)`, label)
	if err != nil {
		t.Fatalf("seedActiveCode: insert owner: %v", err)
	}
	ownerID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("seedActiveCode: owner last insert id: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO guild_code (owner_id, token_hash, label) VALUES (?, ?, ?)`,
		ownerID, []byte("hash-"+label), label); err != nil {
		t.Fatalf("seedActiveCode: insert guild_code: %v", err)
	}
}

// TestRun_RevokeDispatch_MissingArg: revoke-code without an id/label is a usage
// error (exit 2).
func TestRun_RevokeDispatch_MissingArg(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "squirebot-revoke.db")
	if code := run([]string{"revoke-code", "--db", dbPath}); code != 2 {
		t.Fatalf("run(revoke-code without arg) exit = %d, want 2", code)
	}
}

// TestRun_RunJob_BadName: run-job with an unknown job name is a usage error
// (exit 2) and makes no live fetch. The live pigparse/wiki success paths are
// covered by Plan 04's job tests (httptest-backed) + the manual D-7 parity check;
// here we assert only the dispatch/arg-handling layer so the unit test needs no
// network. (The bad name is rejected BEFORE the DB is opened, so the --db value
// is irrelevant — but we pass a temp path to mirror real invocation.)
func TestRun_RunJob_BadName(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "squirebot-runjob.db")
	if code := run([]string{"run-job", "bogus", "--db", dbPath}); code != 2 {
		t.Fatalf("run(run-job bogus) exit = %d, want 2", code)
	}
}

// TestRun_RunJob_MissingName: run-job with no positional job name is a usage error
// (exit 2).
func TestRun_RunJob_MissingName(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "squirebot-runjob.db")
	if code := run([]string{"run-job", "--db", dbPath}); code != 2 {
		t.Fatalf("run(run-job with no job name) exit = %d, want 2", code)
	}
}

// TestRun_RunJob_ExtraPositional: run-job with TWO job names is a usage error
// (exit 2). Documented choice: exactly one job name is required, so
// `run-job pigparse wiki` is rejected rather than silently running only the first
// (avoids an ambiguous "which one ran?" surprise in the D-7 parity check).
func TestRun_RunJob_ExtraPositional(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "squirebot-runjob.db")
	if code := run([]string{"run-job", "pigparse", "wiki", "--db", dbPath}); code != 2 {
		t.Fatalf("run(run-job pigparse wiki) exit = %d, want 2 (exactly one job name required)", code)
	}
}

// TestRun_RunJob_NameAroundFlag: the job-name positional may appear AFTER the --db
// flag (splitFlagsAndPositionals handles ordering, same as revoke-code). A bad
// name in that position still exits 2 — proving the arg split, not a live run.
func TestRun_RunJob_NameAroundFlag(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "squirebot-runjob.db")
	if code := run([]string{"run-job", "--db", dbPath, "bogus"}); code != 2 {
		t.Fatalf("run(run-job --db X bogus) exit = %d, want 2", code)
	}
}

// openForAssert opens the DB the CLI wrote (it already has the schema via the
// subcommand's goose.Up) for read-back assertions, registering cleanup.
func openForAssert(t *testing.T, dbPath string) *sql.DB {
	t.Helper()
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open db for assert: %v", err)
	}
	return db
}

// TestRun_SetOwnerFloorDispatch drives the set-owner-floor subcommand through
// run() against a temp DB and asserts exit 0 plus BOTH the app_config owner-floor
// pointer AND a guild_admins row for that id (the floor is the bootstrap officer
// — D-08). This is the unit-testable proof of the os.Args dispatch + the store
// wiring (the on-box run is the deploy step).
func TestRun_SetOwnerFloorDispatch(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "squirebot-floor.db")
	const floorID = "123456789012345678"

	if code := run([]string{"set-owner-floor", floorID, "--db", dbPath}); code != 0 {
		t.Fatalf("run(set-owner-floor) exit = %d, want 0", code)
	}

	db := openForAssert(t, dbPath)
	defer db.Close()

	// app_config['owner_floor_discord_id'] points at the seeded id.
	var floorVal string
	if err := db.QueryRow(`SELECT value FROM app_config WHERE key = 'owner_floor_discord_id'`).Scan(&floorVal); err != nil {
		t.Fatalf("query app_config owner_floor_discord_id: %v", err)
	}
	if floorVal != floorID {
		t.Errorf("owner_floor_discord_id = %q, want %q", floorVal, floorID)
	}

	// The floor is the bootstrap officer (guild_admins row exists).
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM guild_admins WHERE discord_user_id = ?`, floorID).Scan(&n); err != nil {
		t.Fatalf("query guild_admins: %v", err)
	}
	if n != 1 {
		t.Errorf("guild_admins rows for floor = %d, want 1 (bootstrap officer)", n)
	}
}

// TestRun_SetOwnerFloorDispatch_MissingArg: set-owner-floor without a discord id
// is a usage error (exit 2).
func TestRun_SetOwnerFloorDispatch_MissingArg(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "squirebot-floor.db")
	if code := run([]string{"set-owner-floor", "--db", dbPath}); code != 2 {
		t.Fatalf("run(set-owner-floor without id) exit = %d, want 2", code)
	}
}

// TestReadRoutes_RequireSession_401 is the W-1 (D-01 / T-15-11) read-gate proof:
// EVERY read route must be wrapped in webauth.RequireSession so a request with NO
// session cookie returns 401. The table MUST list all FIVE read routes the serve
// mux registers (the 4 views + meta) — leaving any one un-gated is the
// frontend-only-gating bypass D-01 forbids. The handler wiring here MIRRORS the
// serve mux's RequireSession(db, ...) wrap in runServe.
func TestReadRoutes_RequireSession_401(t *testing.T) {
	db := store.NewTestDB(t)
	st := store.NewStore(db)

	// Mirror the serve mux's gated read routes exactly (same wrap, same handlers).
	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/meta", webauth.RequireSession(db, readapi.NewMeta(st)))
	mux.Handle("GET /api/v1/views/view", webauth.RequireSession(db, readapi.NewViews(st, "view")))
	mux.Handle("GET /api/v1/views/gear_check", webauth.RequireSession(db, readapi.NewViews(st, "gear_check")))
	mux.Handle("GET /api/v1/views/spell_check", webauth.RequireSession(db, readapi.NewViews(st, "spell_check")))
	mux.Handle("GET /api/v1/views/bank", webauth.RequireSession(db, readapi.NewViews(st, "bank")))

	// The table MUST enumerate all five read routes (W-1).
	routes := []string{
		"/api/v1/meta",
		"/api/v1/views/view",
		"/api/v1/views/gear_check",
		"/api/v1/views/spell_check",
		"/api/v1/views/bank",
	}
	for _, route := range routes {
		t.Run(route, func(t *testing.T) {
			rec := httptest.NewRecorder()
			// No session cookie attached → must be 401.
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, route, nil))
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("GET %s with no session = %d, want 401 (read route not session-gated)", route, rec.Code)
			}
		})
	}
}

// newMemberSession mints a real (member, non-officer) session and returns the
// opaque session id to attach as the sb_session cookie. The web_user is NOT added
// to guild_admins, so RequireOfficer must 403 it while RequireSession admits it —
// the exact pair the 15-03 gate test needs.
func newMemberSession(t *testing.T, ctx context.Context, db *sql.DB, discordUserID string) string {
	t.Helper()
	// Use the REAL current time so expires_at (now+TTL) is in the future —
	// ResolveSession checks the row against time.Now(), so a fixed past `now` would
	// mint an already-expired session and the gate would (correctly) 401 it.
	now := time.Now().Unix()
	if err := store.UpsertWebUser(ctx, db, discordUserID, "Member", "", now); err != nil {
		t.Fatalf("upsert web_user: %v", err)
	}
	sid, err := store.GenerateSessionID()
	if err != nil {
		t.Fatalf("generate session id: %v", err)
	}
	if err := store.CreateSession(ctx, db, discordUserID, sid, now, store.SessionTTLSeconds); err != nil {
		t.Fatalf("create session: %v", err)
	}
	return sid
}

// TestWriteRoutes_Gates is the 15-03 / 16-01 / D-01 / D-12 (B-1) / D-03 route-gate
// proof:
//   - the officer-only admin routes are RequireOfficer-wrapped (a MEMBER session →
//     403), and unauthenticated → 401;
//   - the bank-coin routes are RequireSession-wrapped (no session → 401), and a
//     plain MEMBER session is ADMITTED past the gate (NOT 401/403 — D-12).
//   - the char-meta routes are RequireSession-wrapped (no session → 401), and a
//     plain MEMBER session is ADMITTED past the gate (NOT 401/403 — D-03 login-only;
//     LR-01, P16 review). A future edit that swapped in RequireOfficer would fail
//     here — which is the bypass this route-level test exists to catch (the handler
//     layer is gate-agnostic, so it cannot).
//
// The wiring here MIRRORS runServe's RequireOfficer/RequireSession wraps exactly.
func TestWriteRoutes_Gates(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()

	st := store.NewStore(db) // read-side store for the item-search route
	// Mirror the serve mux's write-surface wiring (same wraps, same handlers).
	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/admin/evict", webauth.RequireOfficer(db, webadmin.EvictHandler(db)))
	mux.Handle("GET /api/v1/admin/restorable", webauth.RequireOfficer(db, webadmin.RestorableListHandler(db)))
	mux.Handle("POST /api/v1/admin/officers/add", webauth.RequireOfficer(db, webadmin.OfficerAddHandler(db)))
	mux.Handle("GET /api/v1/coin/bank-toons", webauth.RequireSession(db, webadmin.BankToonsHandler(db)))
	mux.Handle("POST /api/v1/coin", webauth.RequireSession(db, webadmin.CoinSetHandler(db)))
	mux.Handle("GET /api/v1/char/meta-list", webauth.RequireSession(db, webadmin.CharMetaListHandler(db)))
	mux.Handle("POST /api/v1/char/meta", webauth.RequireSession(db, webadmin.CharMetaSetHandler(db)))
	mux.Handle("POST /api/v1/account/codes", webauth.RequireSession(db, webadmin.MintOwnCodeHandler(db)))
	mux.Handle("GET /api/v1/account/codes", webauth.RequireSession(db, webadmin.ListOwnCodesHandler(db)))
	mux.Handle("POST /api/v1/account/codes/revoke", webauth.RequireSession(db, webadmin.RevokeOwnCodeHandler(db)))
	mux.Handle("GET /api/v1/wishlist/{char}", webauth.RequireSession(db, readapi.NewWishlist(st)))
	mux.Handle("POST /api/v1/wishlist", webauth.RequireSession(db, webadmin.AddWishlistHandler(db)))
	mux.Handle("POST /api/v1/wishlist/remove", webauth.RequireSession(db, webadmin.RemoveOwnWishlistHandler(db)))
	mux.Handle("POST /api/v1/wishlist/ping", webauth.RequireSession(db, webadmin.SetWishlistPingHandler(db)))
	mux.Handle("GET /api/v1/items/search", webauth.RequireSession(db, readapi.NewItemSearch(st)))

	// A plain member session (NOT an officer).
	member := "555555555555555555"
	sid := newMemberSession(t, ctx, db, member)
	cookie := &http.Cookie{Name: webauth.SessionCookieName, Value: sid}

	// 1) Officer route, MEMBER session → 403 (RequireOfficer).
	t.Run("admin/evict member→403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/evict", nil)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("POST /api/v1/admin/evict (member) = %d, want 403", rec.Code)
		}
	})

	// 2) Officer route, NO session → 401.
	t.Run("admin/evict anon→401", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/admin/evict", nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("POST /api/v1/admin/evict (anon) = %d, want 401", rec.Code)
		}
	})

	// 2a) Restorable list (officer-only GET), MEMBER session → 403 (RequireOfficer).
	t.Run("admin/restorable member→403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/restorable", nil)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("GET /api/v1/admin/restorable (member) = %d, want 403", rec.Code)
		}
	})

	// 2b) Restorable list, NO session → 401.
	t.Run("admin/restorable anon→401", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/restorable", nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("GET /api/v1/admin/restorable (anon) = %d, want 401", rec.Code)
		}
	})

	// 3) Coin route, NO session → 401 (RequireSession).
	t.Run("coin anon→401", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/coin", nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("POST /api/v1/coin (anon) = %d, want 401", rec.Code)
		}
	})

	// 4) Coin route, MEMBER session → ADMITTED past the gate (D-12: NOT 401/403).
	// The bank-toons GET with a member session returns 200 (empty []), proving a
	// non-officer is allowed through RequireSession.
	t.Run("coin/bank-toons member→admitted", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/coin/bank-toons", nil)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
			t.Errorf("GET /api/v1/coin/bank-toons (member) = %d, want admitted (D-12 login-only)", rec.Code)
		}
	})

	// 5) Char-meta route, NO session → 401 (RequireSession — D-03; LR-01).
	t.Run("char/meta anon→401", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/char/meta", nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("POST /api/v1/char/meta (anon) = %d, want 401", rec.Code)
		}
	})

	// 6) Char-meta route, MEMBER session → ADMITTED past the gate (D-03: NOT
	// 401/403). The meta-list GET with a member session returns 200 (empty []),
	// proving a non-officer is allowed through RequireSession — and that the route
	// is NOT RequireOfficer-wrapped (which would 403 this member).
	t.Run("char/meta-list member→admitted", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/char/meta-list", nil)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
			t.Errorf("GET /api/v1/char/meta-list (member) = %d, want admitted (D-03 login-only)", rec.Code)
		}
	})

	// 7) Account-codes route, NO session → 401 (RequireSession — Phase 17 / D-09).
	t.Run("account/codes anon→401", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/account/codes", nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("GET /api/v1/account/codes (anon) = %d, want 401", rec.Code)
		}
	})

	// 8) Account-codes route, MEMBER session → ADMITTED past the gate (D-09: every
	// signed-in member, NOT officer-gated). A never-minted member's list returns 200
	// ([]), proving RequireSession admits a non-officer (a RequireOfficer swap would
	// 403 this member — the bypass this route-level test exists to catch).
	t.Run("account/codes member→admitted", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/account/codes", nil)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
			t.Errorf("GET /api/v1/account/codes (member) = %d, want admitted (D-09 login-only)", rec.Code)
		}
	})

	// 9) Wishlist route, NO session → 401 (RequireSession — Phase 34 / D-02).
	t.Run("wishlist anon→401", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/wishlist/Tankbert", nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("GET /api/v1/wishlist/{char} (anon) = %d, want 401", rec.Code)
		}
	})

	// 10) Wishlist route, MEMBER session → ADMITTED past the gate (D-02: every
	// signed-in member, NOT officer-gated). An unknown char returns 200 (the 21 empty
	// slots), proving RequireSession admits a non-officer (a RequireOfficer swap would
	// 403 this member — the bypass this route-level test exists to catch).
	t.Run("wishlist member→admitted", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/wishlist/Tankbert", nil)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
			t.Errorf("GET /api/v1/wishlist/{char} (member) = %d, want admitted (D-02 login-only)", rec.Code)
		}
	})

	// 11) Item-search route, NO session → 401 (RequireSession — D-10 session-gated).
	t.Run("items/search anon→401", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/items/search?q=xx", nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("GET /api/v1/items/search (anon) = %d, want 401", rec.Code)
		}
	})

	// 12) Item-search route, MEMBER session → ADMITTED past the gate (D-10 login-
	// only). With an empty corpus the search returns 200 ([]), proving RequireSession
	// admits a non-officer (a RequireOfficer swap would 403 this member).
	t.Run("items/search member→admitted", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/items/search?q=xx", nil)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
			t.Errorf("GET /api/v1/items/search (member) = %d, want admitted (D-10 login-only)", rec.Code)
		}
	})
}
