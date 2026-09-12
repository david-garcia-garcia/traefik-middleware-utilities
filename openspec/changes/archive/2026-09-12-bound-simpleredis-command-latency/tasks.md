## 1. Defaults and overall deadline

- [x] 1.1 Set zero-Config `DialTimeout` 200ms and `IOTimeout` 100ms. `applyDefaults` maps `MaxRetries` 0 to 1. Keep `retryLimits` 0→3 and `-1` off. Document the multiplied worst case next to the knobs
- [x] 1.2 `exec` takes `context.Context`, computes overall deadline `(maxRetries+1)*(DialTimeout+IOTimeout)` min'd with `ctx` deadline, stops the ladder when remaining time is gone, and waits on backoff with `select` (not `time.Sleep`)
- [x] 1.3 Pass remaining time into `dial` and `do` so AUTH/SELECT/command share one budget. Map derived-budget expiry to `redis:timeout`. Do not type-assert `net.Error`

## 2. Context twins and cancel

- [x] 2.1 Add `*Context` twins for Get, MGet, Set, Del, Incr, IncrBy, Expire, ExpireAt, Eval, MSetEX, MSetEXAt. Unadorned methods wrap `context.Background()`. Thread `ctx` through `borrow`/`dial`/`do`/`Eval`/`msetex`
- [x] 2.2 On `ctx.Done()` close the in-use socket, do not pool it, free the turn, return `ctx.Err()`. Pool wait MUST return `ctx.Err()` without taking a turn. `shouldRetry` MUST NOT retry `ctx.Err()` or `redis:timeout`

## 3. Tests and usage

- [x] 3.1 Compiled test: zero-Config Get against `203.0.113.1:6379` returns within the overall deadline plus 50ms slack
- [x] 3.2 Compiled test: Pass+Database against a peer that accepts TCP and never replies; elapsed is within the overall deadline (not DialTimeout + 2×IOTimeout per attempt)
- [x] 3.3 Compiled test: cancel mid-command against a stall fake returns `ctx.Err()` promptly, frees the turn, and does not leave the socket idle. Already-cancelled `GetContext` MUST NOT send GET
- [x] 3.4 Update `knowledge/devdocs/std_go_simpleredis.md` (defaults, overall budget, `*Context` twins, worst-case product). Keep Yaegi happy-path on unadorned verbs. Run `go test -short ./simpleredis/...` until it passes

## 4. Specs

- [x] 4.1 Confirm the tcp-session and resp-commands deltas match the landed defaults, overall deadline, and `*Context` twins
- [x] 4.2 Run `openspec validate --change bound-simpleredis-command-latency --strict`
