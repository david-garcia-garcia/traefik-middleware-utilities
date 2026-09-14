# Explore
IssueKey: 2026-09-12-simpleredis-risk-01-no-context-uncancellable-latency

## Concepts

**Retry ladder**: `exec` loops `attempt := 0; attempt <= maxRetries`. Zero `MaxRetries` is the go-redis sentinel for 3 extra retries (four sends). Backoff is `time.Sleep` after `release`, so a sleeping retry holds no pool token.

**Per-step clocks**: `dial` uses `net.Dialer{Timeout: DialTimeout}` then `do` AUTH and `do` SELECT, each with a fresh `IOTimeout` `SetDeadline`. `PoolTimeout` only bounds waiting for an in-use turn.

**Uncancellable work**: no public verb takes `context.Context`. A cancelled Traefik request keeps the goroutine in sleep/dial/`do`.

**Overall budget**: one deadline at `exec` entry, remaining time shared by dial + AUTH/SELECT + command so handshake steps cannot each add a full `IOTimeout`.

**`*Context` twins**: keep today’s signatures as `context.Background()` wrappers so in-tree callers (`windowcounter`, `tokenbucket`, `e2e/simpleredisprobe`) do not have to move this change.

Units:

- `simpleredis/commands_exec.go` — `exec`, `retryLimits`, `time.Sleep` backoff
- `simpleredis/config.go` — `defaultDialTimeout` 2s, `defaultIOTimeout` 1s
- `simpleredis/pool.go` — `borrow`, `dial` AUTH/SELECT
- `simpleredis/resp.go` — `do` per-command `SetDeadline`
- `simpleredis/commands.go`, `commands_eval.go`, `commands_msetex.go` — public verbs
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` — 2s dial, 0→3 retries
- `knowledge/devdocs/std_go_simpleredis.md` — retry sentinels, no worst-case product
- `e2e/simpleredisprobe/plugin.go` — Traefik `New` already has `context.Context`; ServeHTTP does not pass `req.Context()`

```
  Get (no ctx)
        │
        ▼
     exec ── for attempt 0..maxRetries (default 3 extra)
        │         time.Sleep(backoff)   ← not cancellable
        ▼
     borrow (PoolTimeout wait only)
        │
        idle miss ── dial (DialTimeout) ── AUTH do (IOTimeout) ── SELECT do (IOTimeout)
        │
        do command (fresh IOTimeout)
        │
        fail unreachable ── retry (dial failures yes; timeout / pool-wait no)
```

Measured this session (temp `go run` against `203.0.113.1:6379`, stock `New`): `err=redis:unreachable elapsed=8.047238s`.

## Decisions

- Take numbered fixes 1–3 on `requirement.md` Desired. Circuit breaker stays a follow-up note (finding ranks it after 1–3; Expected gain still names it).
- Defaults: `DialTimeout` 200ms, `IOTimeout` 100ms. `applyDefaults` maps `MaxRetries` 0 → 1 so zero `Config` is one extra retry (two attempts). `retryLimits` keeps `0` → 3 and `-1` off so an explicit `MaxRetries: 3` still means three extra; callers who want the old four-attempt ladder set `MaxRetries: 3`. Spec delta on `std_go_simpleredis_tcp-session` (dial SHALL no longer be two seconds; zero Config retry is one extra).
- No new `CommandTimeout` Config field. Derived overall budget at `exec` entry: `(maxRetries+1)*(DialTimeout+IOTimeout)`. Remaining time is the `SetDeadline` for dial and for each `do` (AUTH/SELECT/command). A `ctx` deadline can only tighten. Sleep uses `select` on remaining/`ctx.Done()`, not `time.Sleep`.
- Every public verb gets a `*Context` twin (`GetContext`, `EvalContext`, `MSetEXContext`, …). Today’s methods wrap `context.Background()`. `exec` / `borrow` / `dial` / `do` take `ctx`. `Eval`’s EVALSHA then EVAL and `MSetEX`’s native-then-Eval each call `exec` with the same `ctx`; each `exec` has its own derived cap (existing two-send shape).
- Cancel: a watcher closes the socket on `ctx.Done()`; turn is freed; socket is not pooled. `ctx.Err()` is returned for cancel / caller deadline. Derived-budget expiry returns `redis:timeout` (existing token). Neither is retried. No `net.Error` assert (Yaegi).
- Proof is compiled tests in `simpleredis/`: black-hole `203.0.113.1:6379` asserting elapsed under the derived budget (this host black-holes; fast-fail still passes the budget); accept-then-stall fake with `Pass`+`Database` for handshake amplification; cancel mid-command asserts prompt return, turn freed, socket closed. Probe stays on unadorned verbs (Background wrappers). windowcounter / tokenbucket stay on unadorned verbs.
- Do not merge or cherry-pick `2026-09-11-simpleredis-perf-02-io-timeout`. Dest already has `Config.DialTimeout` / `IOTimeout`. This change is defaults + overall budget + context.
- In-tree callers of unadorned verbs keep compiling: searched `**/*.go` for `.Get(`, `.Eval(`, `.Set(`, `.MGet(`, `.Incr(`, `.MSetEX(` on `*simpleredis.SimpleRedis` — production: `windowcounter/limiter.go` (Incr/Get/Eval), `tokenbucket/redis.go` (Eval), `e2e/simpleredisprobe/plugin.go` (all verbs). Tests stay on current signatures.

## Open questions

- Q: Exact new defaults vs the finding’s 200ms / 100ms / MaxRetries 1?
  Rank: bounded asked — Desired names that set; zero-Config defaults and tcp-session “dial SHALL be two seconds” / `0`→3 sentinel are existing contracts; call sites of `New(Config{` plus spec/usage (config.go, tcp-session spec, std_go_simpleredis.md, tests) are in this change
  Decision: assumed — DialTimeout 200ms, IOTimeout 100ms; applyDefaults MaxRetries 0→1; retryLimits keeps 0→3 and -1 off; explicit MaxRetries: 3 still three extra.
  By: explore

- Q: How is the overall exec budget derived vs a new Config field?
  Rank: additive asked — Desired “new knob or derived”
  Decision: assumed — derived `(maxRetries+1)*(DialTimeout+IOTimeout)` at exec entry; remaining passed into dial/do; no CommandTimeout field; ctx deadline can only tighten.
  By: explore

- Q: Does every verb get a *Context twin or only exec internally?
  Rank: additive asked — Desired “Keep today’s signatures as context.Background() wrappers”; existing callers keep working
  Decision: assumed — *Context twin on every public verb (Get, MGet, Set, Del, Incr, IncrBy, Expire, ExpireAt, Eval, MSetEX, MSetEXAt); exec/borrow/dial/do take ctx.
  By: explore

- Q: Must this change take the circuit breaker?
  Rank: additive incidental — How to fix ranks it after 1–3; Out of scope of sibling findings; Desired lists it as add-on
  Decision: assumed — do not take; note knowledge/debt/2026-09-12-simpleredis-dial-circuit-breaker.md.
  By: explore

- Q: What error does cancel or overall-budget expiry return?
  Rank: additive asked — Desired names context + overall deadline; Error() token not specified
  Decision: assumed — ctx.Err() for cancel / caller deadline; redis:timeout when the derived budget expires; shouldRetry retries neither.
  By: explore

- Q: How is worst-case elapsed proven if TEST-NET-3 is not black-holed on some CI hosts?
  Rank: additive asked — Desired names black-hole 203.0.113.1:6379 plus AUTH/SELECT stall fake
  Decision: assumed — elapsed-budget Get against 203.0.113.1:6379 (measured 8.047s here) asserting return within derived budget; handshake stall fake with Pass+Database; cancel against stall fake. Do not assert elapsed ≈ 8s.
  By: explore

- Q: Must the probe pass req.Context() for live proof?
  Rank: additive asked — Affected says probe only if live proof must pass request context; named proof is compiled tests
  Decision: assumed — compiled tests only; probe and windowcounter/tokenbucket stay on unadorned verbs this change.
  By: explore
