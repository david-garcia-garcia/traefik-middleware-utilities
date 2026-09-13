## Why

Dest `do` stamps one `SetDeadline(now+IOTimeout)` for the whole command. A compliant peer that streams a bulk steadily still loses if the bytes need more wall time than that one window. Default `IOTimeout` is 100 ms while the decoder accepts `$` payloads up to 64 MiB. A 4 MiB GET at 60 ms failed 5/5 with `redis:timeout`, allocated ~20 MiB, and burned a fresh dial each time. Raising `IOTimeout` is not a fix: the same knob bounds every small command.

## What Changes

- Wrap the TCP conn at dial, under `bufio`, so each `Read` and `Write` refreshes `SetReadDeadline` / `SetWriteDeadline` to `clampTimeout(ctx, IOTimeout)`. `IOTimeout` becomes a stall (quiet-peer) bound.
- Keep `bindCommandDeadline` and `watchConnClose`. A drip peer MUST NOT pin an in-use turn past `(MaxRetries+1)*(DialTimeout+IOTimeout)`.
- Keep start-of-command `ioBound` for `ioOrContext` so a caller deadline stays `context.DeadlineExceeded` and the library budget stays `redis:timeout`.
- Leave `maxBulkLength` at 64 MiB (deviation: no bandwidth model, no static shrink).
- Default-suite tests with a `bug3` helper prefix: streaming bulk beyond one `IOTimeout` returns intact; silent mid-reply times out promptly; a drip returns within the overall budget and frees the turn.
- Do not change `readBulk` allocation. Do not type-assert `net.Error`.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: per-command I/O refreshes the socket deadline on progress; overall command budget still closes the socket.

## Impact

- `simpleredis/resp.go` (`stallConn`, `do` stops one-shot `SetDeadline`).
- `simpleredis/pool.go` (`dial` wraps before `bufio`).
- `simpleredis/config.go` (`IOTimeout` comment).
- `simpleredis/bug3_stall_deadline_test.go` (new).
- `knowledge/devdocs/std_go_simpleredis.md` (stall vs total-transfer sentence).
- Main spec `openspec/specs/std_go_simpleredis_tcp-session/spec.md` after archive.
