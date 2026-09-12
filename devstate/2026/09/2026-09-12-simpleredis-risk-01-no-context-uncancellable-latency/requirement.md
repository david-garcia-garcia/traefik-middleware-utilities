# Requirement
IssueKey: 2026-09-12-simpleredis-risk-01-no-context-uncancellable-latency

## Problem
A stock SimpleRedis command against an unreachable Redis can spend ~8.1s (four dials × 2s `DialTimeout` plus backoff). No public verb takes `context.Context`, `exec` sleeps with `time.Sleep`, and there is no overall deadline across attempts. A cancelled Traefik request keeps the goroutine working. AUTH/SELECT after a completed TCP handshake each add a fresh `IOTimeout`, so a passworded stall can reach ~16s. `PoolTimeout` only bounds waiting for a turn, not work after the turn is held.

## Current (code)
- `simpleredis/commands.go` — `Get`, `MGet`, `Set`, `Del`, `Incr`, `IncrBy`, `Expire`, `ExpireAt` take no `context.Context`.
- `simpleredis/commands_eval.go:18` — `Eval` takes no context; both EVALSHA and NOSCRIPT→EVAL go through `exec`.
- `simpleredis/commands_msetex.go:29-34` — `MSetEX` / `MSetEXAt` take no context.
- `simpleredis/commands_exec.go:11-37` — `exec` loops `attempt := 0; attempt <= maxRetries`; backoff is `time.Sleep`; no overall deadline; no context.
- `simpleredis/commands_exec.go:39-45` — `MaxRetries` `0` maps to 3 extra retries (four attempts); `-1` is one send.
- `simpleredis/commands_exec.go:83-95` — `shouldRetry` does not retry `isCommandTimeout` or `errPoolWait`; dial/`redis:unreachable` is retried.
- `simpleredis/config.go:9-12` — defaults `DialTimeout` 2s, `IOTimeout` 1s, `PoolTimeout` 200ms; `MaxRetries` zero-value is the 3-retry sentinel.
- `simpleredis/pool.go:62-105` — `borrow` waits up to `PoolTimeout` for a turn, then `dial()` while holding the turn; `PoolTimeout` does not cap dial/AUTH/SELECT.
- `simpleredis/pool.go:157-183` — `dial` uses `net.Dialer{Timeout: sr.dialTimeout}`, then `do` AUTH and `do` SELECT when set (each a fresh `IOTimeout`).
- `simpleredis/resp.go:14-17` — `do` `SetDeadline(now+ioTimeout)` per command.
- `simpleredis/resp.go:152-158` — `ioError` uses `errors.Is(os.ErrDeadlineExceeded)`; comment documents Yaegi panic on `net.Error` assert.
- `simpleredis/simpleredis.go:22-23` — `errPoolWait` is distinct from `errUnreachable` with the same `Error()` text.
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` — zero knobs mean 2s dial, 1s I/O, 3 extra retries; dial timeout SHALL be two seconds; no command-wide budget; verbs have no context.
- `knowledge/devdocs/std_go_simpleredis.md` — retry sentinels and “do not retry timeout / pool-wait”; does not document total worst-case latency or a caller deadline.
- `e2e/simpleredisprobe/plugin.go` — Traefik `New` receives `context.Context`; ServeHTTP verbs (`Get`, `Eval`, …) do not pass `req.Context()`.
- `GetContext` / `exec` overall deadline / `commandTimeout` / circuit breaker — not found.
- Black-hole elapsed-budget test (`203.0.113.1:6379`) — not found.

## Desired
- Shorter proxy-shaped defaults (finding’s defensible set: `DialTimeout` 200ms, `IOTimeout` 100ms, `MaxRetries` 1) and document the multiplied worst case next to the knobs.
- One overall deadline for a whole `exec` (new knob or derived), enforced across attempts; remaining budget passed into `dial` so AUTH/SELECT cannot each add a full `IOTimeout`.
- Accept `context.Context` on the command path (Yaegi stdlib `context`; cancel via `SetDeadline` plus close-on-`ctx.Done()`). Keep today’s signatures as `context.Background()` wrappers (`Get` → `GetContext`). Do not type-assert `net.Error`.
- Prove worst-case elapsed time (black-hole address), AUTH/SELECT amplification vs overall budget, and (once context lands) cancel mid-command returns promptly, frees the pool turn, and closes the socket.
- Circuit breaker after *k* consecutive dial failures (finding: Yaegi-safe `atomic` counter + last-failure time) is in this finding as the highest-leverage add-on, ranked after the three numbered fixes.

## Affected
- `simpleredis/config.go`, `simpleredis/commands_exec.go`, `simpleredis/pool.go`, `simpleredis/commands.go`, `simpleredis/commands_eval.go`, `simpleredis/commands_msetex.go`, `simpleredis/resp.go` as needed for deadline/cancel
- matching `*_test.go` (black-hole / stall fake / cancel)
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` (defaults, overall budget, context)
- `openspec/specs/std_go_simpleredis_resp-commands/spec.md` if verb signatures grow `*Context` wrappers
- `knowledge/devdocs/std_go_simpleredis.md`
- `e2e/simpleredisprobe/plugin.go` only if live proof must pass request context or new knobs

## Out of scope
- Other `simpleredisfixes2/` findings (idle reaper, bulk trailer, windowcounter, exported sentinels, non-basic RESP2, FreeInUseTurn, zero-value client, readline bounds, MaxIdleConns).
- Re-doing dest timeout *configurability* (already on `origin/master` via `Config.DialTimeout` / `IOTimeout`); this ticket changes defaults and adds overall budget + context.
- Merging or depending on branch `2026-09-11-simpleredis-perf-02-io-timeout` (not an ancestor of `origin/master`).
- Importing go-redis or miniredis; TLS; Unix sockets; functional options.
- Rate limiters, window counter, token bucket.

## Unknowns
- Exact new defaults vs the finding’s “defensible” 200ms / 100ms / MaxRetries 1 (whether `MaxRetries` 1 is a new zero-value or a non-zero Config default that breaks the go-redis `0`→3 sentinel).
- How `commandTimeout` is derived from existing knobs vs a new Config field, and its default.
- Whether every verb gets a `*Context` twin or only `exec` takes context internally.
- Circuit-breaker threshold *k*, cooldown, and whether implement must take it (How to fix ranks it after 1–3).
- go-redis `GetContext` / context-deadline shape is not in `knowledge/research/` (indexes consumed; no write this phase).
- How Pester / the probe observes cancel or overall budget on `/redis` and `/dragonfly` if live proof is required later.

## Tensions
- Finding How to fix lists three numbered changes “worth doing regardless,” then a circuit breaker “beyond this”; Expected gain still names the breaker. Bound to this file: 1–3 are the ask; breaker is optional unless explore ranks it in.
- Related first-review perf-02 (configurable I/O timeouts) is **not** on `origin/master` as that branch; dest already has the knobs from the live-pool `New(Config)` change (`config.go`). This finding extends dest: uncancellable 8.1s ladder / no context — do not re-implement perf-02.
- Spec still says dial timeout SHALL be two seconds and `MaxRetries` 0 means 3 extra retries; shorter defaults and a new sentinel meaning would be a spec delta, not a silent break of existing `MaxRetries: 0` callers.
- Finding proof is compiled black-hole / stall-fake tests; dest e2e is Redis+Dragonfly Pester. Fake tests are the named proof; live e2e only if explore finds the probe must pass request context.
