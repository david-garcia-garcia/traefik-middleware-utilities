## Why

SimpleRedis idle pooling is LIFO: `borrow` pops the tail and stops at the first socket still inside `idleTimeout`, so a cold head sits past idle while sequential traffic recycles the newest socket. Under a concurrency spike those head sockets are borrowed as likely-dead landmines (failed attempt plus redial). Dest has no reaper, and Traefik reload must not leak a goroutine this package does not already own.

## What Changes

- Sweep-on-release in `release`: before append-or-close, peel idle-head entries older than `idleTimeout` (stop at the first still-valid head). Keep LIFO tail reuse. No background ticker or reaper goroutine.
- Peel on every reusable `release` path, including when the returning conn is then closed because the client is `closed` or `len(idle) >= maxIdleConns`. Close peeled sockets after unlock. `Close` still drains whatever remains.
- Fake-server unit test: two idle entries, backdate **head** `lastUsed`, run a command, assert the stale socket was closed and is not left in `idle`.
- Same two-entry idle-head / no-landmine proof in `simpleredis/live_test.go` against live Redis and Dragonfly (`SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY`). CI sets both; skip on `-short` or empty. Do not extend compose, Pester, or `e2e/simpleredisprobe` for this proof.
- Do not change `idleTimeout` (30s) or `maxIdleConns` (8). No Eval in this proof (Get). Lua 5.1-safe + Dragonfly KEYS remains required if a later proof adds Eval.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: idle pool SHALL close a cold head while a younger tail is recycled (sweep-on-release, head prefix only). Existing “shall not be reused” stays. Prove on the fake server and on live Redis and Dragonfly. Traefik/Pester are not the idle-head proof.

## Impact

- `simpleredis/simpleredis.go` (`release`; `borrow` LIFO `break` stays).
- `simpleredis/simpleredis_test.go` (two-entry idle-head case); new `simpleredis/live_test.go`.
- `.github/workflows/ci.yml` `test` job env `SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY` (reuse existing Redis/Dragonfly services). README Tests paragraph names those vars.
- Main spec `openspec/specs/std_go_simpleredis_tcp-session/spec.md` after archive.
- Usage gotcha on `knowledge/devdocs/std_go_simpleredis.md` after apply (not this proposal).
- No compose, Pester, or probe change. No server `timeout`. No EVALSHA, pipelining, go-redis, TLS, Unix sockets.
