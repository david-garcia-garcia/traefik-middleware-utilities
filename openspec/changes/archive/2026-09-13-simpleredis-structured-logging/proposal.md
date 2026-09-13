## Why

SimpleRedis already classifies AUTH rejection, pool wait, truncated bulk, malformed RESP, handshake failure, NOSCRIPT reload, over-free, and a panic inside `do`, then returns those as errors (or lets the panic reach Traefik) with no slog line. Operators cannot tell a defended peer glitch from a broken in-use-turn invariant. `reclaim` already emits `reclaim_*` on a `*slog.Logger`; this package has neither.

## What Changes

- Public `Config.Logger *slog.Logger`, frozen at `New` like other knobs. `New` still returns only `*SimpleRedis` and MUST NOT error. nil logger means completely silent. Do not install a discard handler.
- Exported `Msg*` constants with `simpleredis_` event strings, slog key/value attrs, reclaim shape. Emit the owner-approved inventory at the decision site, not from the generic `release` dirty-close arm. `release(conn, reusable bool)` signature stays. Idle-cap `MsgSocketClosed` fires on the existing idle-cap branch inside `release` because that is the detection site.
- Error (4): panic (recover, log, re-panic), noauth, not-from-New, over-free. Warn (4 of 6 on dest): pool exhausted, short bulk, bad reply, handshake failed (non-auth). Debug (10): dial, idle swept, retry, timeout, canceled, socket closed, capability, noscript, open, close. No Info. Two Warn events (`simpleredis_socket_poisoned`, `simpleredis_auth_leftover`) wait on PR #69.
- Never log `Config.Pass`, Redis key names, or values. Log counts, reasons, byte lengths, host (`Config.Host` frozen at `New`), and error text.
- Call-site nil check. Per-command Debug also `logger.Enabled(ctx, slog.LevelDebug)` before any attr construction.
- Panic: existing release defer first, then recover/log/re-panic defer. Turn returned, socket closed, `OverFrees()==0`, panic still reaches the caller.
- Tests: capturing handler per dest-detectable event; nil-logger on every public verb; Debug alloc=0 when nil or Debug off; secrets test; Yaegi logging; `interpretedcost_test.go` must not regress.
- Usage packet `knowledge/devdocs/std_go_simpleredis.md`.

## Capabilities

### New Capabilities

- `std_go_simpleredis_slog-events`: message constants, levels, attributes, security (no Pass/keys/values), call-site guards, panic recover-log-repanic, dest-detectable event sites.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: `New` copies `Config.Logger`; nil is silent; stdlib `log/slog` is an allowed import; `New` still does not error or dial.

## Impact

- `simpleredis/config.go`, `simpleredis.go`, `log.go` (new), `pool.go`, `resp.go`, `commands_exec.go`, `commands_eval.go`, `commands_msetex.go`, new/extended tests.
- Main specs after archive: `openspec/specs/std_go_simpleredis_tcp-session/spec.md`, `openspec/specs/std_go_simpleredis_slog-events/spec.md`.
- Usage: `knowledge/devdocs/std_go_simpleredis.md`.
- Out of scope: leftover-RESP detection; `release` signature change; discard-handler default; Info tier; logging Pass/keys/values; other packages' loggers; swallowing panics.
