# Explore
IssueKey: 2026-09-11-simpleredis-test-01-eof-redial
Verdict: in progress

Dest already retries a dead pooled socket once unless the error is timeout (`simpleredis/simpleredis.go` `exec` 182-201, `ioError` 430-437). Measured: `TestStaleConnectionIsRetried`, `TestTimeoutOnReusedConnIsNotRetried`, and `TestIdleTimeoutOpensANewConnection` pass on this worktree (`go test -run TestStaleConnectionIsRetried|TestTimeoutOnReusedConnIsNotRetried|TestIdleTimeoutOpensANewConnection ./simpleredis/` — ok). That suite never closes the peer: it `conn.close()`s idle fds so `SetDeadline` fails with `os.ErrClosed` and `ioError` never sees `io.EOF`. Compose/Pester `/redis` and `/dragonfly` are happy-path verbs only.

No active OpenSpec change (`openspec list --json` empty). Fold the peer-close scenario into `openspec/specs/std_go_simpleredis_tcp-session/spec.md` in propose.

## Concepts

```
  dest TestStaleConnectionIsRetried     production idle close
  ─────────────────────────────────     ──────────────────────
  Get → pool                            Get → pool
  client conn.close()                   Redis/Dragonfly closes TCP
  2nd Get: SetDeadline = ErrClosed       2nd Get: write may OK, read EOF
  ioError never runs                    ioError(io.EOF) → redis:unreachable
  exec retries (reusable=false)       exec retries unless timeout
```

- **SimpleRedis** (`knowledge/devdocs/std_go_simpleredis.md`): `Init` does not dial; idle cap 8; `idleTimeout` 30s; `ioTimeout` 1s. `exec` retries once when `!reusable && reused && err != errTimeout`. Second `borrow` failure returns that error (`:195-197`). `release(false)` closes and does not append to `idle`.
- **Peer close vs client close:** peer FIN typically survives `SetDeadline` and fails in `writeCommand` or `readReply` (`io.EOF` → line 436). Client `close()` hits `do` 293-294 (`errUnreachable` without `ioError`). Both are retry-eligible; only peer close covers `:436`.
- **`startFakeRedis`:** `serve` loops until read error; it does not close after the first reply. Sibling custom listeners already exist (`TestTimeoutOnReusedConnIsNotRetried`).
- **Live engines:** CI `test` job already runs `redis:7-alpine` `:6379` and Dragonfly `v1.40.2` `:6380` for windowcounter/tokenbucket. Compose `reclaim-e2e` has the same images, routes `/redis` and `/dragonfly`, probe `Init` in `New` (one client per engine across requests). Pester does not `CLIENT KILL`.
- **Idle close (research):** Redis and dest Dragonfly default `timeout`/`--timeout` **0**. Redis `CLIENT KILL ADDR|ID` and Dragonfly `CLIENT KILL ADDR|ID` (no `SKIPME`/`TYPE`). See `knowledge/research/ext_redis_clients_idle-close/` and `ext_dragonfly_clients_idle-close/`.

## Decisions

- **Fake peer-close tests** — new listener helper (do not change `startFakeRedis`): accept, reply to the first command, `Close` the **accepted** socket, do not close the client. Success path: first Get ok, second Get ok on a new dial, fake saw 2 accepts, dead conn not in `sr.idle`. Fail path: close the listener after the first accept so retry `borrow` returns `redis:unreachable` (`:195-197`).
- **Rename** `TestStaleConnectionIsRetried` to name the client-fd / `SetDeadline` / `os.ErrClosed` arm. New tests name peer-close / `io.EOF`.
- **Do not change retry policy** unless those tests fail on dest `exec`. Spec: add a peer-closed idle scenario distinct from client-side close.
- **Live close** — `CLIENT KILL ADDR` of `pooledConn.netConn.LocalAddr()` (or `ID` from `CLIENT LIST`) from a sidecar. Not `CONFIG SET timeout` (process-wide, imprecise, shared CI Redis/Dragonfly). Not container restart. Compose stays `timeout` 0.
- **Both harnesses** — (1) `simpleredis` Go live tests, skip `-short` / unset env, env `SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY`, CI env next to the windowcounter ones; (2) Pester Its on `/redis` and `/dragonfly` after kill via `docker compose exec redis redis-cli` (`-h dragonfly` for Dragonfly). Do not skip Dragonfly.
- **Probe** — keep `Init` in `New`. Add `recover=1` Set+Get only and `X-SimpleRedis-Recover: ok`. Do not export pool stats from `SimpleRedis`. Existing verb headers stay on the default path.
- **Lua** — no new script. Existing probe EVAL already lists `KEYS` and is Lua 5.1-safe. `recover=1` does not Eval. `CLIENT KILL` is `noscript`.
- **Usage** — gotcha added on `std_go_simpleredis.md` (peer close vs client close).

## Open questions

- Q: Which live close do both Redis and Dragonfly support in compose/CI (`timeout`, `CLIENT KILL`, or container restart)?
  Rank: additive asked — new test helpers this change creates; Desired live idle/server-close against both compose services; Unknowns line 1
  Decision: resolved — `CLIENT KILL ADDR` or `ID` from a sidecar. Redis and Dragonfly v1.40.2 both document those filters; Dragonfly lacks `SKIPME`/`TYPE`. Do not `CONFIG SET timeout` / `--timeout` on shared CI services. Do not restart compose containers.
  By: explore

- Q: Are CI Go live tests, compose+Pester, or both required so Dragonfly is not skipped?
  Rank: additive asked — new `live_test.go` and Pester Its; Desired compose recovery plus Affected `.github/workflows/ci.yml and/or a simpleredis live test`
  Decision: assumed — both. Go live tests on dest `:6379`/`:6380` (env `SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY`, skip if unset or `-short`, same shape as `windowcounter/live_test.go`). Pester recovery Its against compose `/redis` and `/dragonfly`. Fake TCP stays the unit path that does not need engines.
  By: explore

- Q: How long to wait after server close versus client `idleTimeout` (30s)?
  Rank: additive asked — constraint on the live tests this change adds; Unknowns wait vs `idleTimeout`
  Decision: resolved — `CLIENT KILL` is immediate; do not sleep 30s. `borrow` must still reuse (`lastUsed` younger than 30s) or the test never hits `io.EOF`. If idle `timeout` were used, wait must be greater than the engine timeout and less than 30s.
  By: explore

- Q: Must the probe expose reuse/redial in headers, or can Pester infer recovery from a second 200 after a kill?
  Rank: additive asked — new query/header on `e2e/simpleredisprobe` this change creates; Desired extend Pester and probe
  Decision: assumed — Pester: warmup GET, kill the probe connection by `ID`/`ADDR` (not `TYPE`/`SKIPME`), second GET 200 with verb or recover headers. Probe: `recover=1` runs Set+Get only and sets `X-SimpleRedis-Recover: ok`. Do not add production pool metrics on `SimpleRedis`.
  By: explore

- Q: How do Go live tests send `CLIENT KILL` without a public SimpleRedis API?
  Rank: additive incidental — new test-only helper this change creates; means to the live-recovery criterion (no criterion names a public `ClientKill`)
  Decision: assumed — same-package sidecar (second `SimpleRedis` or raw dial) named with `test` in the identifier. Kill `ADDR` of `sr.idle[0].netConn.LocalAddr()` so parallel windowcounter/tokenbucket clients on the same CI Redis survive. Do not ship a production `ClientKill`.
  By: explore

- Q: Does dest `exec` retry policy need to change for peer `io.EOF`?
  Rank: additive asked — Desired "Do not change the retry policy unless a new test proves dest is wrong"
  Decision: assumed — keep one retry of a dead pooled conn unless peer-close tests fail on dest `exec`. Spec adds a scenario only.
  By: explore
