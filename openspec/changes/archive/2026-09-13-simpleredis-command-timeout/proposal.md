## Why

Two defects in one knob. First, dest `do` stamped one `SetDeadline(now+IOTimeout)` for the whole command, so a compliant peer streaming a large bulk lost whenever the bytes needed more wall time than that single window; the default was 100 ms while the decoder accepts `$` payloads to 64 MiB, and because `redis:timeout` is not retried the value was permanently unreadable rather than slow. A 4 MiB GET at 60 ms failed 5/5 with `redis:timeout`, allocated ~20 MiB, and burned a fresh dial each time.

Second, once `do` stamps the socket deadline from the remaining command budget, `IOTimeout` stops bounding any I/O operation. It survives only as a multiplicand inside `(MaxRetries+1)*(DialTimeout+IOTimeout)` and as a fallback that the `exec` path never reaches, because `bindCommandDeadline` always returns a ctx with a deadline. A public knob named `IOTimeout` that bounds no I/O is a wrong contract, and the one bound that does apply was a product of three knobs: an operator had to do arithmetic to learn the latency ceiling, and raising `MaxRetries` silently inflated it.

## What Changes

- `do` stamps the socket deadline from the time remaining on the command context instead of from `IOTimeout`. That remainder is the single bound on the socket.
- `Config.CommandTimeout` replaces `Config.IOTimeout`. It bounds the whole command: all attempts, plus the dial, AUTH, SELECT and command I/O inside them. `bindCommandDeadline` binds `now + CommandTimeout`, still deferring to a sooner caller deadline and still reporting `libraryOwnsDeadline` the same way.
- `MaxRetries` leaves the deadline formula. It bounds the attempt count only; retries stop when either the count is exhausted or `CommandTimeout` expires.
- `DialTimeout` is unchanged. It keeps real individual meaning as the per-dial-attempt cap (`net.Dialer.Timeout`) and is not folded into `CommandTimeout`.
- Default `CommandTimeout` is 900 ms, which is exactly dest's worst case `(1+1)*(200ms+250ms)`.
- Accessor `IOTimeout()` becomes `CommandTimeout()`. `IOTimeout` is removed with no deprecated alias; the package is internal to this module.
- `ioOrContext` drops its `ioBound` / `ioTimeout` arguments; `commandBudgetLeft` drops its `fallback` argument and becomes a method that reads `CommandTimeout` for the deadline-free direct-`do` path.
- Keep `watchConnClose`. A drip peer MUST NOT pin an in-use turn past the budget.
- Leave `maxBulkLength` at 64 MiB (deviation: no bandwidth model, no static shrink).
- Do not change `readBulk` allocation. Do not type-assert `net.Error`.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: the command socket deadline is the command budget's remainder, and that budget is one `CommandTimeout` knob rather than a product of three.
- `std_go_simpleredis_resp-commands`: the per-hop budget each Eval / MSetEX fallback binds is `CommandTimeout`.

## Impact

- `simpleredis/config.go` (`CommandTimeout` replaces `IOTimeout`, `defaultCommandTimeout` 900 ms, doc).
- `simpleredis/simpleredis.go` (`commandTimeout` field, `CommandTimeout()` accessor).
- `simpleredis/commands_exec.go` (`bindCommandDeadline` loses its `maxRetries` parameter and the product).
- `simpleredis/resp.go` (`do` deadline source, `commandBudgetLeft`, `ioOrContext` signature).
- `simpleredis/bug3_command_budget_deadline_test.go`, `commands_deadline_test.go`, and every test that set `IOTimeout`.
- `windowcounter/repro_lock_during_get_test.go` (compiles against the renamed knob).
- `e2e/simpleredisprobe/plugin.go` (both `New` calls: `CommandTimeout: 3s` for the 500 ms TIME-wait Eval).
- `knowledge/devdocs/std_go_simpleredis.md` (budget numbers, knob semantics gotcha).
- Main specs `openspec/specs/std_go_simpleredis_tcp-session/spec.md` and `std_go_simpleredis_resp-commands/spec.md`.

## Behavior Trade-off

A peer that goes quiet mid-reply ends the command at the budget (900 ms at zero Config) rather than after a shorter per-operation window. That is accepted: the failure it replaces was silent, permanent data loss on healthy peers, and operators who need a tighter ceiling now lower one number, `CommandTimeout`, which *is* the ceiling.

The worst case does not move: 900 ms before, 900 ms after. What moves is that raising `MaxRetries` no longer raises it.
