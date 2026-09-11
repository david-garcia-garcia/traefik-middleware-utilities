## Context

Dest `exec` already retries once when `!reusable && reused && err != errTimeout`. `ioError` maps `io.EOF` to `redis:unreachable`. `TestStaleConnectionIsRetried` closes idle fds so `SetDeadline` fails with `os.ErrClosed` and never reaches `ioError`. Sibling custom listeners already exist (`TestTimeoutOnReusedConnIsNotRetried`). CI `test` already runs Redis `:6379` and Dragonfly `:6380`. Compose `reclaim-e2e` routes `/redis` and `/dragonfly`; probe `Init` is in `New`. Research: `knowledge/research/ext_redis_clients_idle-close/`, `ext_dragonfly_clients_idle-close/`. Proceed policies: `devstate/explore.md`. See proposal.md for why.

## Goals / Non-Goals

**Goals:**
- Fake-TCP peer-close success and retry-borrow-fail tests without changing `startFakeRedis`.
- Live `CLIENT KILL ADDR|ID` recovery on both engines (Go + Pester), Dragonfly not skipped.
- Probe `recover=1` Set+Get header; default verbs unchanged.

**Non-Goals:**
- Changing retry policy unless those tests fail on dest `exec`.
- `CONFIG SET timeout`, `--timeout`, or container restart.
- Production `ClientKill` or pool-stat headers.
- Isolated `writeCommand` write-error tests; EVALSHA; new Lua.

## Decisions

1. **New peer-close listener, leave `startFakeRedis`.** Accept, reply to the first command, `Close` the accepted socket, keep serving a second accept on the success path. Fail path: close the listener after the first accept. Alternative: teach `startFakeRedis` to close after one reply — rejected; that helper serves until read error and other tests share it.

2. **Rename the client-close test.** `TestStaleConnectionIsRetried` becomes a name that says client-fd / `SetDeadline` / `os.ErrClosed`. New tests name peer-close / `io.EOF`. Alternative: keep the old name — rejected; it hides which arm it covers.

3. **Keep dest `exec`.** One retry of a dead pooled conn unless the new tests fail. Spec adds the peer-close scenario only. Alternative: always redial — out of scope.

4. **`CLIENT KILL ADDR` from a sidecar.** Go: same-package helper with `test` in the identifier; raw RESP `CLIENT KILL ADDR` of `sr.idle[0].netConn.LocalAddr()` so parallel windowcounter/tokenbucket clients on the shared CI engine survive. Pester: warmup GET, `docker compose exec redis redis-cli` (`-h dragonfly` for Dragonfly), kill the Traefik client's `ADDR`/`ID` from `CLIENT LIST` (match the Traefik compose IP; do not use `TYPE`/`SKIPME`). Compose stays `timeout` 0. Alternative: `CONFIG SET timeout` — process-wide and imprecise. Alternative: container restart — kills the engine, not the pooled socket.

5. **Both harnesses.** `simpleredis/live_test.go` mirrors `windowcounter/live_test.go` (skip `-short` / both env unset). Env `SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY`. CI `test` job sets them next to the windowcounter addrs. Pester Its in the existing SimpleRedis Describe. Alternative: only one harness — Desired and HARD REQUIREMENT ask both.

6. **Probe `recover=1`.** Keep `Init` in `New`. Query `recover=1` runs Set+Get only and sets `X-SimpleRedis-Recover: ok`. Default path keeps every verb header. Do not export pool stats. Alternative: infer recovery from a second 200 on the default path — weaker (many verbs can redial for other reasons); recover header names the job.

7. **No new Lua.** Existing probe EVAL already lists `KEYS` and is Lua 5.1-safe. `CLIENT KILL` is `noscript`. `recover=1` does not Eval.

## Risks / Trade-offs

- [Dragonfly has no `SKIPME`/`TYPE`] → Mitigation: kill by Traefik `ADDR`/`ID` only; never `CLIENT KILL TYPE NORMAL`.
- [Shared CI Redis/Dragonfly with windowcounter/tokenbucket] → Mitigation: kill only the pooled `LocalAddr`; do not `CONFIG SET timeout`.
- [Kill count 0] → Mitigation: fail the test; do not treat a no-op kill as recovery.
- [`idleTimeout` 30s] → Mitigation: `CLIENT KILL` is immediate; do not sleep 30s or `borrow` drops the socket locally.
- [Pester reclaim Describe stops a/b] → Mitigation: SimpleRedis Describe never stops those services.

## Migration Plan

Tests and probe query only. Rollback is revert. No production deploy.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
