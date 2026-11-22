# Devdocs impact
change: tokenbucket-expired-bucket-reclaim

Pin: `origin/master...HEAD` at `57acb01f` (after `git_sync` merged `origin/master`). Packets read: `knowledge/devdocs/index.md`, `index_std_go.md`, `std_go_tokenbucket.md`, `std_go_test-suites.md`.

## Units
- Memory — subsystem — `tokenbucket/memory.go`, spec "Memory store caps sources and removes expired ones first", packet `std_go_tokenbucket`
- Redis Allow EXPIRE — subsystem — `tokenbucket/lua.go`, `tokenbucket/redis.go`, packet `std_go_tokenbucket`
- Test suites (Traefik-issue regression files) — pattern — `knowledge/devdocs/std_go_test-suites.md`

## Findings
- [x] language-gap  Memory — `std_go_tokenbucket` has NewMemory How-to and cap/expiry Gotcha, no Memory Language term
- [x] stale-usage  Memory — `std_go_tokenbucket` Gotcha does not state idle expired sources below the cap leave on the next new-source Allow (ten idle plus one new = one stored)
- [x] stale-usage  Redis Allow EXPIRE — `std_go_tokenbucket` does not say every EVAL EXPIRE so remaining TTL is in `(0, ttl]`; How-to and Key files omit `traefik_issue13704_*`
- [x] stale-usage  Test suites — `std_go_test-suites` How-to does not name Traefik-issue regression files (`traefik_issue<id>_test.go` / `_e2e_test.go`)

No `missing-packet`, no `wrong-fold`. SimpleRedis: none (tests only; no product change; `Pttl` stays unexported).

## Produced
- `std_go_tokenbucket` Language: added **Memory** (in-process store, cap 65536, expiry-ordered list; `_Avoid_`: ttlmap; treating Redis keys as Memory entries).
- `std_go_tokenbucket` How-to: prove Memory idle-reclaim with `traefik_issue13704_test.go`; remaining Redis TTL with `traefik_issue13704_e2e_test.go`.
- `std_go_tokenbucket` Key files: those two test files.
- `std_go_tokenbucket` Gotchas: expired Memory sources leave below the cap (ten idle plus one new = one stored); Redis EVAL always `EXPIRE`s so remaining TTL is in `(0, ttl]`.
- `std_go_test-suites` How-to, Pattern snippet, Key files: `traefik_issue<id>_test.go` / `_e2e_test.go` in the domain package, helpers suffixed with the id.
