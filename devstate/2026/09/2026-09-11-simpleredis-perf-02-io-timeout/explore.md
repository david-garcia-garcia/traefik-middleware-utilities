# Explore

## Concepts

```
  caller                         SimpleRedis
  ──────                         ───────────
  Init(host,pass,db)      ──►   store strings; timeouts = package defaults
  InitWithOptions(..., opts) ─► store strings + knobs (0 = default)
  Get/Set/Eval/...        ──►   borrow → dial/do(SetDeadline) → release
```

**SimpleRedis** already lives on `knowledge/devdocs/std_go_simpleredis.md`. This ticket adds caller-set dial / I/O / idle timeout and idle cap. It is not a pool rewrite, not retries, not go-redis.

```
  defaults (today's constants, zero-value of Options)
    DialTimeout  = 2s
    IoTimeout    = 1s     → one SetDeadline covering write+read
    IdleTimeout  = 30s
    MaxIdleConns = 8      → idle list after release, not live sockets

  do: SetDeadline(now+IoTimeout) → writeCommand → readReply
  dial: net.Dialer{Timeout: DialTimeout}
  exec: still does not retry errTimeout
```

Identity (client address, user, tenant, Host, trust hop) is not reconstructed. The caller already owns Redis `host`.

**Yaegi:** plain `Options` struct of `time.Duration` and `int`. No functional-option closures. `Init(host, pass, database)` stays.

**Live stall:** `BLPOP` on a unique empty list (`knowledge/research/ext_redis_blpop/`, `ext_dragonfly_blpop/`). Lua `EVAL` blocks the whole server; `CLIENT PAUSE` pauses every client; both would race CI `go test ./...` against the same Redis as windowcounter/tokenbucket. `DEBUG SLEEP` is out of scope and undocumented on Dragonfly.

Production `Init` caller: `e2e/simpleredisprobe/plugin.go` (1). Limiters take an already-Inited `*SimpleRedis`. Tests keep the three-arg `Init`. No caller migration if `Init` is unchanged.

Usage packet Language is still true. How to use / Pattern snippet will need `InitWithOptions` after apply (implement / devdocs-impact). No Language write now.

## Decisions

- Keep `Init(host, pass, database)`. Add `InitWithOptions(host, pass, database, Options)`. `Init` delegates with `Options{}`. Zero / negative duration and `MaxIdleConns==0` mean today's constants. Do not export host/pass/database. Do not add read vs write deadlines.
- Spec host: delta on `std_go_simpleredis_tcp-session` (defaults stay two seconds / 1s I/O / thirty / eight). No new spec family. `std_go_simpleredis_resp-commands` unchanged.
- Live proof: `simpleredis/live_test.go` table-driven on `SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY`, skip on `-short` or unset. CI already runs both engines; add those two env vars next to the limiter ones. Same-package `exec("BLPOP", uniqueKey, "10")` with `IoTimeout` tens of milliseconds → `redis:timeout`. Do not export BLPOP. Do not stall Traefik/Pester.
- Fake: `TestIoTimeout` stays on defaults. New fake never-reply test with configured `IoTimeout`. IdleTimeout / MaxIdleConns proven by extending existing fake tests with configured values.
- Probe `Config` stays `Host` only. Pester Describe stays happy-path verbs. Yaegi `clientprobe` keeps `Init`; optional later smoke of `InitWithOptions` is not required for this ticket.
- Update `knowledge/devdocs/std_go_simpleredis.md` How to use after apply (zero = default; Init unchanged).

## Open questions

- Q: Options surface — InitWithOptions, extra Init args, or exported fields on SimpleRedis?
  Rank: additive asked — new InitWithOptions/Options this change creates; Desired names those three shapes and requires Init(host, pass, database) unchanged
  Decision: assumed — `InitWithOptions(host, pass, database, Options)` with unexported session fields copied from a plain `Options` (`DialTimeout`, `IoTimeout`, `IdleTimeout` `time.Duration`; `MaxIdleConns` int). `Init` calls it with `Options{}`. Not extra Init args (would break every existing call). Not exported timeout fields on `SimpleRedis` (would mix pool internals with settings).
  By: explore

- Q: How to stall a live Redis and Dragonfly command to prove a configured I/O timeout without DEBUG SLEEP, Lua 5.1-safe, KEYS-declared?
  Rank: additive asked — Desired live proof on both engines; Unknowns name the stall
  Decision: assumed — compiled same-package live test: `exec` of `BLPOP` on a unique empty key with server timeout 10s and `IoTimeout` ~50ms. Redis and Dragonfly both block that one connection (`ext_redis_blpop`, `ext_dragonfly_blpop`). Not Eval (Lua blocks the whole server). Not CLIENT PAUSE (all clients). Not DEBUG SLEEP. Not a public BLPOP method.
  By: explore

- Q: Whether unused poolSize/poolTimeout fields are stored as no-ops or omitted from the struct and only documented?
  Rank: additive asked — Desired names the finding's poolSize/poolTimeout knobs versus perf-01 out of scope
  Decision: assumed — omit from `Options`. Document live-socket cap / wait queue as perf-01 in spec and usage. Deviation recorded.
  By: explore

- Q: Must the Traefik probe Config expose the new timeouts for Pester, or are live Go tests against compose/CI ports enough?
  Rank: additive asked — Desired says compose+Pester and/or live Go tests; Unknowns name the probe Config
  Decision: assumed — live Go tests are enough. Probe stays `Host` only. Pester stays `/redis` and `/dragonfly` happy-path. CI `test` job sets `SIMPLEREDIS_LIVE_REDIS=127.0.0.1:6379` and `SIMPLEREDIS_LIVE_DRAGONFLY=127.0.0.1:6380`. Do not stall the Yaegi plugin (would 502 `/redis` for other Describes).
  By: explore

- Q: Separate read vs write deadlines, or one combined SetDeadline?
  Rank: additive asked — Desired says separate deadlines are optional and one combined SetDeadline is enough
  Decision: resolved — keep one `SetDeadline(now+IoTimeout)` in `do` covering write and read. No IoReadTimeout / IoWriteTimeout fields.
  By: explore

- Q: How is configured DialTimeout proven if a blackhole SYN is not reliable in CI?
  Rank: additive asked — Desired names dial timeout as a knob to prove
  Decision: assumed — same-package test after `InitWithOptions` that `dial` uses the stored duration (field visible in package tests) plus a hanging-SYN attempt to `192.0.2.1` asserting `redis:unreachable` in well under the 2s default. Dial is TCP, not engine-specific; do not duplicate it per Redis/Dragonfly. I/O timeout is the live-engine proof.
  By: explore

- Q: Can MaxIdleConns 0 mean "keep no idle sockets", or is 0 the default eight?
  Rank: additive asked — Desired names zero-value defaults so Init is unchanged
  Decision: assumed — `MaxIdleConns <= 0` means eight. Callers cannot express "idle cap zero" on this Options type. Idle cap of zero is not in the ask. Negative is treated as zero so a literal cap of -1 cannot close every idle socket.
  By: propose

- Q: Rewrite the spec's "concurrent commands SHALL not open more than eight connections" now that MaxIdleConns is configurable?
  Rank: bounded incidental — existing tcp-session requirement; 1 spec leaf `openspec/specs/std_go_simpleredis_tcp-session/spec.md` plus the 8-concurrent fake test in `simpleredis/simpleredis_test.go`; perf-01 is Out of scope
  Decision: assumed — do not rewrite the live-socket claim. Spec delta: idle list at most MaxIdleConns (default 8); dial/I/O/idle defaults when unset. Concurrent-eight stays as today (already idle-cap wording vs unbounded in-flight). Leave the live cap to perf-01.
  By: explore
