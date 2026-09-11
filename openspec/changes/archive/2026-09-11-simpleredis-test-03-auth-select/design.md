## Context

Dest already closes on AUTH/SELECT dial errors (`simpleredis/simpleredis.go` `dial`) and maps AUTH-class prefixes in `replyError`. The in-process fake always answers AUTH/SELECT `+OK`; `startStaticRedis` cannot AUTH-then-SELECT. Compose `redis` / `dragonfly` have no password; probe `Config` is `Host` only. Research: `knowledge/research/ext_redis_auth/`, `ext_redis_select/`, `ext_dragonfly_auth/`, `ext_dragonfly_select/`. Proceed policies: `devstate/explore.md`. See proposal.md for why.

## Goals / Non-Goals

**Goals:**
- Configurable fake handshake replies plus hangup count so compiled tests hit AUTH/SELECT close-on-error.
- Live proof on dest Redis and Dragonfly for SELECT 99 (existing engines) and WRONGPASS (sibling requirepass).
- Probe `Password`/`Database` and Pester failure routes without touching success `/redis` `/dragonfly`.

**Non-Goals:**
- Changing `dial`, `replyError`, pool, or timeouts unless a new test proves them wrong.
- Mapping Redis 7.4 nopass AUTH text to `redis:noauth`.
- requirepass on existing windowcounter/tokenbucket CI services or compose success engines.
- New Lua, EVALSHA, miniredis, TLS, Unix sockets.

## Decisions

1. **Fake reply fields default to success.** Optional AUTH/SELECT reply strings on `fakeRedis`, default `+OK`, so `TestAuthAndSelectOncePerDial` stays. Handshake-failure tests set those replies. Alternative: `startStaticRedis` for AUTH fail — rejected; it cannot AUTH `+OK` then SELECT error, and has no hangup/accept counters.

2. **Hangup count is serve-loop exit.** Increment when `serve` returns after read error (client `close` → EOF). Assert that counter plus empty idle and `connections()==1`. Alternative: wrap `net.Conn` to count `Close` — extra type; EOF on the existing loop is the peer observation Desired asked for.

3. **SELECT-fail Init uses a password.** `Init(addr, "secret", "99")` so AUTH runs first. Empty-pass SELECT-fail would skip AUTH and would not pin AUTH-before-SELECT. Keep `TestRejectedAuthIsReturned` as command-reply mapping; new test names say AUTH/SELECT failure.

4. **Live SELECT 99 on existing engines.** Env `SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY` (CI already publishes 6379/6380). Skip-if-unset like `windowcounter/live_test.go`. Pester: extra whoami routes with `database=99` against compose `redis` / `dragonfly`. Alternative: a dedicated 16-DB-capped sibling — rejected; dest default `databases`/`dbnum` is already 16.

5. **Live WRONGPASS needs requirepass siblings.** Dragonfly without `--requirepass` accepts any AUTH (`OK`). Redis 7.4 nopass AUTH is not `redis:noauth`. Compose `redis-auth` / `dragonfly-auth`: `redis-server --requirepass secret`, Dragonfly `--requirepass=secret` plus `ulimits.memlock: -1`. Probe labels `password=wrong`. Do not put requirepass on existing success services. Alternative: ACL users — extra surface; dest engines use requirepass.

6. **CI passworded siblings for `go test`.** Add `SIMPLEREDIS_LIVE_REDIS_AUTH` / `SIMPLEREDIS_LIVE_DRAGONFLY_AUTH`. GitHub Actions `services` has no `command` field; dest `redis:7-alpine` does not read a password env. Prefer sibling service containers on new host ports (6381/6382) if process args can be passed; otherwise a `test` job step `docker run` of the **same dest images** with `--requirepass` (Dragonfly also `memlock=-1`). Do not switch to bitnami. Do not add requirepass to 6379/6380.

7. **Probe `Password` and `Database`, empty default.** `New` passes them to `Init`. Success labels stay host-only. Failure routes use `http.Error` 502 (`err.Error()` already). Eval on success routes stays the Kong KEYS snippet. Alternative: extra probe plugins — rejected; one plugin, labels select the handshake.

## Risks / Trade-offs

- [GHA services cannot pass `--requirepass`] → Mitigation: Decision 6 `docker run` fallback with dest images; Pester compose siblings always have `command:`.
- [Dragonfly nopass AUTH is OK] → Mitigation: wrong-password live cases only on requirepass siblings.
- [Coverage counters are fake-only] → Mitigation: live engines prove dest image behavior; `go test` cover profile proves `dial` close blocks.
- [Reclaim Pester stops a/b] → Mitigation: SimpleRedis Describe never stops those services.

## Migration Plan

Test and harness only. Rollback is revert. No production deploy.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
