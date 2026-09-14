## Why

Dest `do` stamps one `SetDeadline(now+IOTimeout)` for the whole command, so `IOTimeout` is both an input to the command budget and a second cap on the same socket. A compliant peer that streams a bulk steadily loses when the bytes need more wall time than that one window. The default was 100 ms while the decoder accepts `$` payloads up to 64 MiB, a size no 100 ms window can carry. A 4 MiB GET at 60 ms failed 5/5 with `redis:timeout`, allocated ~20 MiB, and burned a fresh dial each time; because `redis:timeout` is not retried, that value was permanently unreadable rather than slow. Two bounds on one socket can only disagree, and the shorter one was winning for the wrong reason.

## What Changes

- `do` stamps the socket deadline from the time remaining on the command context instead of from `IOTimeout`. `bindCommandDeadline` already set that context to `(MaxRetries+1)*(DialTimeout+IOTimeout)` or the caller's sooner deadline, so the budget is the single bound and `IOTimeout` shapes it.
- Raise default `IOTimeout` from 100 ms to 250 ms. 100 ms is too tight for a default; the zero-Config budget becomes 900 ms.
- `ioOrContext` drops its `ioBound` / `ioTimeout` arguments. The socket deadline is now the command deadline, so a fired socket deadline reports `context.DeadlineExceeded` and `libraryTimeout` decides library (`redis:timeout`) versus caller (`Err()`) as before.
- Keep `watchConnClose`. A drip peer MUST NOT pin an in-use turn past the budget.
- Leave `maxBulkLength` at 64 MiB (deviation: no bandwidth model, no static shrink).
- Default-suite tests with a `bug3` helper prefix: a streaming bulk beyond one `IOTimeout` returns intact; a silent mid-reply times out at the budget; a drip returns within the budget and frees the turn.
- Do not change `readBulk` allocation. Do not type-assert `net.Error`.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: the command socket deadline is the command budget's remainder rather than a separate `IOTimeout` cap.

## Impact

- `simpleredis/resp.go` (`do` deadline source, `commandBudgetLeft`, `ioOrContext` signature).
- `simpleredis/config.go` (`defaultIOTimeout` 250 ms, `IOTimeout` doc).
- `simpleredis/simpleredis.go` (`IOTimeout` accessor doc).
- `simpleredis/bug3_command_budget_deadline_test.go` (new).
- `simpleredis/commands_deadline_test.go` (default assertion 250 ms).
- `knowledge/devdocs/std_go_simpleredis.md` (budget numbers, `IOTimeout` semantics gotcha).
- Main spec `openspec/specs/std_go_simpleredis_tcp-session/spec.md` after archive.

## Behavior Trade-off

A peer that goes quiet mid-reply now ends the command at the budget (900 ms at zero Config) rather than after `IOTimeout`. That is accepted: the failure it replaces was silent, permanent data loss on healthy peers, and operators who need a tighter ceiling lower `IOTimeout` or `DialTimeout`, which lowers the budget itself.
