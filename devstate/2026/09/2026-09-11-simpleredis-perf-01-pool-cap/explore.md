# Explore
IssueKey: 2026-09-11-simpleredis-perf-01-pool-cap

## Concepts

**Idle cap** (`maxIdleConns = 8`): how many unused sockets `release` will keep. Dest already has this.

**Live cap** (`poolSize`): idle plus checked-out sockets. Dest has none; `borrow` dials whenever idle is empty.

**Wait queue**: callers above the live cap block on a buffered `chan struct{}` until a slot frees or `poolTimeout` elapses.

**Pool timeout**: fail the command with an existing Error() string rather than queue forever.

**Const-only pool**: `Init(host, pass, database)` stays three arguments; defaults are package constants. Same-package tests set unexported fields when they need a smaller cap or a short timeout.

Units:

- `simpleredis/simpleredis.go` — session, `borrow`, `release`, `dial`
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` — idle-eight plus “concurrent commands SHALL not open more than eight”
- `knowledge/devdocs/std_go_simpleredis.md` — gotcha already says in-flight dials are uncapped
- `e2e/simpleredisprobe/plugin.go` — the only production `Init` (Traefik Yaegi)
- `scripts/integration-tests.Tests.ps1` — Pester `/redis` and `/dragonfly` verb headers
- CI `test` job — already starts Redis `:6379` and Dragonfly `:6380` for windowcounter/tokenbucket live tests; simpleredis has no live env yet

```
  caller Get/Set/...
        │
        ▼
     borrow ── idle hit ──► do ──► release (idle if under idle cap)
        │
        idle miss
        │
        live < poolSize? ──yes──► dial (AUTH/SELECT)
        │
        no ── wait poolTimeout ── timeout ► redis:unreachable
                   │
                   slot frees ► reuse or dial
```

## Decisions

- Keep `Init` as three strings. Production callers are one probe plus libraries that take an already-Inited client. A public pool knob would be a new operator surface this ticket did not ask for.
- Default `poolSize` is **8**, same as the idle cap and the live spec’s “at most eight”. Finding allowed 8–16; 16 would widen the spec without a deploy key.
- Default `poolTimeout` is **200ms**, shorter than `ioTimeout` (1s), so a live command can occupy a slot longer than the wait. That is what lets Redis/Dragonfly prove a waiter error without Init knobs. Fail fast locally.
- Timeout Error() text is **`redis:unreachable`**. Finding allowed a new `redis:pool-timeout`; dest callers already match `redis:unreachable`. Distinct string would be a new public token for the same fail-closed outcome.
- In-use turns are the semaphore (go-redis shape): acquire a slot before pop-idle or dial; idle does not hold a turn. Yaegi-safe: buffered `chan struct{}` plus `select` on a timer. Live sockets still cannot exceed `poolSize` because dial only happens while holding a turn.
- `release` still trims idle to eight, but does not close a reusable socket only because idle is full while live is under `poolSize` (the case tests hit when they set `poolSize` above eight).
- Fake-server tests stay (burst dials, live ≤ poolSize, timeout). They are not the required proof.
- Required proof: dest compose + Pester on `/redis` and `/dragonfly`, plus compiled live tests on both engines (same CI `test` service pattern as windowcounter/tokenbucket). Probe grows a hold query that EVALs a TIME wait with **zero keys**, Lua 5.1-safe, no `table.maxn`. Pester fires concurrent holds, asserts `CLIENT LIST` from that backend stays at most eight, and that a waiter past `poolTimeout` returns `redis:unreachable` (HTTP 502).
- Sibling findings (perf-02 I/O-timeout fan-out, reaper, pipelining, EVALSHA, encode/decode, coverage tickets, MSETEX) stay out.

## Open questions

- Q: What is the default `poolSize`?
  Rank: additive asked — Desired names a total live cap in the finding’s 8–16 range; existing Init callers keep working
  Decision: assumed — 8, matching `maxIdleConns` and the tcp-session spec’s concurrent-eight line.
  By: explore

- Q: What is the default `poolTimeout`?
  Rank: additive asked — Desired names a wait bound so the queue is not unbounded
  Decision: resolved — 200ms, shorter than `ioTimeout` (1s), so a hold on Redis/Dragonfly can outlast the wait and Pester can observe `redis:unreachable` without Init knobs.
  By: implement

- Q: Which Error() text does a pool timeout return?
  Rank: additive asked — Desired allows `redis:unreachable` or a new `redis:pool-timeout`
  Decision: assumed — `redis:unreachable` so existing Error() matchers stay valid.
  By: explore

- Q: Must `Init` grow pool knobs?
  Rank: additive asked — Out of scope says do not change `Init` unless a const-only pool is insufficient; Desired does not name a new Init parameter
  Decision: assumed — const-only is enough. Enumerated production `Init` call sites: 1 (`e2e/simpleredisprobe/plugin.go`). Libraries take `*SimpleRedis`. Same-package tests may set unexported fields. Roots searched: `**/*.go` for `.Init(`.
  By: explore

- Q: How does Pester observe live sockets and pool-timeout on `/redis` and `/dragonfly`?
  Rank: additive asked — Desired e2e addendum requires both engines via dest compose + Pester
  Decision: resolved — probe `?hold=` EVAL TIME-wait (0 KEYS, Lua 5.1-safe). Redis CLIENT LIST is blocked while a script runs, so Pester counts ESTABLISHED sockets on `/proc/net/tcp` port 6379 (`:18EB`) in the engine netns (Redis `cat`; Dragonfly distroless uses a `redis:7-alpine` sidecar). Extra waiter → 502 `redis:unreachable` when eight are held. Keep `/a` `/b` reclaim routes.
  By: implement

- Q: Should CI `test` also run compiled pool tests against live Redis and Dragonfly?
  Rank: additive asked — Desired says CI must exercise both backends; dest `test` job already has both engines for sibling packages
  Decision: assumed — yes. Add `SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY` like windowcounter/tokenbucket. Skip when unset (local `-short` / no env). Pester remains the Yaegi proof; live `go test` is wire/pool proof on both engines. Fake-server is not a substitute.
  By: explore
