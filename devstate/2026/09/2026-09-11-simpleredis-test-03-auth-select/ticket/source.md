# test-03 — `dial`'s `AUTH` and `SELECT` failure branches are untested

Source: `simpleredisfixes/test-03-dial-auth-select-failure.md`
Index: `simpleredisfixes/README.md` row test-03 (Coverage, hard, not applied)

## Finding

Both handshake failure branches in `dial` (`simpleredis/simpleredis.go:263-289`) have zero coverage (measured blocks `277.85,280.4` and `283.91,286.4`). `conn.close()` on AUTH and SELECT errors is never executed by any test.

`TestRejectedAuthIsReturned` looks like the covering test, but it initialises the client with no password (`Init(addr, "", "")`). With `sr.pass == ""` the AUTH block is skipped; the fake's canned `-NOAUTH …` reply answers the GET. That test proves `replyError`'s prefix mapping on a command reply, not the handshake. `TestAuthAndSelectOncePerDial` covers only the success case — one AUTH, one SELECT, three GETs.

Operator-visible failure modes that are unproven:

- Wrong password: connection must be closed and the error surfaced as `redis:noauth`. If pooled instead, later commands fail from a socket that looks healthy.
- Bad database index: `SELECT 99` on a 16-database server returns `-ERR DB index is out of range`. Same: close, do not pool.
- Socket leak: `conn.close()` on both branches is the only thing preventing a leaked FD per failed dial. Misconfiguration dials on every request.

`redis:noauth` is a documented contract (`RedisNoAuth`). Middleware is expected to distinguish misconfigured from Redis-down. Only the command-reply route is tested; the handshake route is not.

How to fix (finding):

- AUTH rejected: `Init(addr, "wrong-password", "")` against a fake that replies `-WRONGPASS invalid username-password pair` to AUTH. Assert `redis:noauth`, empty idle pool, peer saw close. Repeat across `replyError` prefixes: `NOAUTH`, `WRONGPASS`, `NOPERM`, `ERR Client sent AUTH`.
- SELECT rejected: `Init(addr, "", "99")` against a fake that answers AUTH with `+OK` and SELECT with `-ERR DB index is out of range`. Assert error to caller, nothing pooled, AUTH succeeded first (ordering comment at `:275`).
- No retry storm: a rejected handshake must not cause `exec` to loop; one error, not repeated dials.

The existing fake already counts `auths` and `selects`; tests mostly need a fake variant whose handshake replies are configurable.

Proof: coverage of those two blocks goes from 0 to non-zero. Each test fails if `conn.close()` is removed (pool empty + peer saw close) or if the error is swallowed and a usable connection returned.

Index context: test-03 sits with test-01 through test-05 as failure modes the pool-cap work exposes. Suggested order is after perf-01/perf-02, with those coverage findings.

## Conductor hard requirement (ask)

- Tests MUST run against both Redis and Dragonfly. Both are supported backends.
- Handshake AUTH/SELECT failure branches need the fake the finding specifies AND live proof on both engines where the engine supports AUTH/SELECT (wrong password / bad DB index).
- Redis and Dragonfly may differ; still run the live cases that each engine supports.
- Extend compose + Pester if dest's harness can take a passworded service.
- Lua 5.1-safe. Dragonfly KEYS required.
