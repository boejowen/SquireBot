# THROWAWAY — PocketBase-as-framework spike (Phase 11, D-01)

**This is not production code.** It is a time-boxed (~1 day) spike that exercises
the four PASS/FAIL probes from `11-CONTEXT.md` §D-01 to decide whether the v2.0
backend adopts **PocketBase** (`pocketbase.New()` as a library) or falls back to a
**hand-rolled Go server** (`net/http` ServeMux + `modernc.org/sqlite`).

Delete this `spike/pocketbase/` tree (and, if the verdict is FALLBACK, the
`github.com/pocketbase/pocketbase` dependency in `go.mod`/`go.sum`) as a cleanup
step in 11-05 once the verdict is recorded. See `11-01-SUMMARY.md` for the verdict.

## What it probes

| Probe | What it proves |
|-------|----------------|
| (a) tables | Plain SQL tables (`owner`/`character`/`inventory_item`/`spellbook_entry` + `item_master`) created via raw DDL coexist with PocketBase's own SQLite file. |
| (b) guarded ingest | A custom **bearer guard** (`crypto/subtle.ConstantTimeCompare`, NOT `apis.RequireAuth()`) gates `POST /api/v1/ingest`, which does an atomic full-snapshot replace inside `app.RunInTransaction`. |
| (c) cron | `app.Cron().MustAdd("spike-heartbeat", "*/1 * * * *", …)` fires while serving. |
| (d) amd64 build | `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build` produces a single static binary (the only SQLite dep is pure-Go `modernc.org/sqlite`). |

## Run locally (probes a/b/c)

```powershell
go run ./spike/pocketbase serve
```

Then, in another shell, drive probe (b):

```powershell
# 401 — no token (writes nothing)
curl.exe -i -X POST http://127.0.0.1:8090/api/v1/ingest

# 200 — valid test token, runs the atomic replace
curl.exe -i -X POST http://127.0.0.1:8090/api/v1/ingest -H "Authorization: Bearer spike-test-token-do-not-use-in-prod"
```

Probe (c) logs `spike: cron fired (probe c)` within ~1 minute.

## Cross-compile (probe d)

```powershell
$env:GOOS="linux"; $env:GOARCH="amd64"; $env:CGO_ENABLED="0"
go build -trimpath -ldflags "-s -w" -o spike-amd64 ./spike/pocketbase
$env:GOOS=""; $env:GOARCH=""; $env:CGO_ENABLED=""
```

## Security note

The bearer guard compares against a **hardcoded TEST token** and the handler
inserts **synthetic rows only** — no `crypto/rand` real guild code is minted and
no real character data is touched (threats T-11.01-01 / T-11.01-03). The
production auth guard and ingest handler land in 11-04 / 11-05.
