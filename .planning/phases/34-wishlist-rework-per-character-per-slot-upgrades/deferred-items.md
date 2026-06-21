# Phase 34 — Deferred Items

Tracked out-of-scope discoveries. NOT fixed in the plan that logged them.

## From 34-01 (the D-01 clean break dropped `wantlist_item`)

The 00014 migration DROPs the retired `wantlist_item` table (D-01). Every test that
seeds/queries `wantlist_item` at HEAD now fails with `no such table: wantlist_item`.
This is the **EXPECTED 34-02 hand-off** (the plan `<verification>` explicitly designates
it): 34-02 repoints the matcher + retires the wantlist write/read surface. Do NOT patch
these in 34-01.

Failing packages (all fail SOLELY on `no such table: wantlist_item`):

| Package | File(s) | What 34-02 does |
|---------|---------|------------------|
| `internal/backendsrv/wantmatch` | `match.go` + `match_test.go` | repoint `ForItem`/`ForName` `FROM wantlist_item`→`wishlist_item`, `muted=0`→`pinged=1`, `LEFT JOIN`→`JOIN`, drop `note`; reseed the test on `wishlist_item` |
| `internal/backendsrv/webadmin` | `wantlist.go` + `wantlist_test.go` (+ `notifications_test.go`, `monitors.go`) | retire/replace the wantlist write handlers with the wishlist clone; unregister the 4 `/api/v1/wantlist*` routes in `main.go` |
| `internal/backendsrv/store` | `wantlist.go` + `wantlist_test.go`, `alertlog_test.go` (its `seedWant` helper), `eccursor_test.go` | retire `store/wantlist.go` + its tests; reseed the alertlog/eccursor tests on `wishlist_item` |
| `internal/backendsrv/ec` | `ec_test.go` (`seedWant`) | reseed the EC matcher test on `wishlist_item` |
| `internal/backendsrv/notify` | `dm_test.go` (`seedWant`) | reseed the DM test on `wishlist_item` |

NB: production code (`store/wantlist.go`, `wantmatch/match.go`, `webadmin/wantlist.go`)
still COMPILES — `go build ./...` rc=0 — these are SQL strings, schema-checked only at
runtime. Only the test runs fail. 34-02 owns the runtime repoint.
