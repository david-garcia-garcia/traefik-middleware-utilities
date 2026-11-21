## Why

Dest `writeCommand` builds every RESP length header by concatenating a string: `"*" + strconv.Itoa(len(args)) + "\r\n"`, then `"$" + strconv.Itoa(len(arg)) + "\r\n"` per argument. Each of those is a heap allocation on a path every Get, Set, Incr and Eval walks — three on a compiled GET, five on a SET, nineteen on the EVAL this repo's token bucket sends. Traefik middleware runs that path per request.

The change removes those allocations by appending the digits into a buffer that already exists (`strconv.AppendInt`), and changes nothing else about how bytes reach the socket.

This proposal originally asked for something larger, and was reduced after measurement: see Decisions in `design.md` for the four encoders that were compiled and counted, and why the single-`net.Conn.Write` variant this change was first named after is now explicitly forbidden rather than shipped.

## What Changes

- Frame each command through the pooled connection's existing `bufio.Writer`, writing every length header as `strconv.AppendInt` into a 24-byte array owned by the connection (`pooledConn.lenBuf`). No string is built anywhere in framing.
- Keep `bufio.Writer` on `pooledConn` and in `dial`. Keep `bufio.Reader`.
- Wire bytes identical to dest. Compiled goldens for the `$27` GET frame and the native `MSETEX` frame, both asserted on the bytes that leave the writer.
- Compiled encode benches (`BenchmarkEncodeGet`, `BenchmarkEncodeEval`, `BenchmarkEncodeMSetEX`, `BenchmarkEncodeSet100KB`) plus `TestAlloc*` guards that fail CI on a regression. The framing ceilings are zero with no slack.
- Keep `BenchmarkYaegiEncodeBufio` / `BenchmarkYaegiEncodeSingleWrite` and record honestly that the rejected shape is the cheaper one interpreted. That cost is accepted, not hidden.
- Tests MUST run against both Redis and Dragonfly (both supported backends): compose + Pester `/redis` and `/dragonfly` plus `e2e/simpleredisprobe` prove every existing verb after the encoder change. Lua 5.1-safe, Dragonfly KEYS required. Do not rewrite the probe Eval script or the Dragonfly compose pin.
- No new verbs. Do not import `go-redis`. Do not change New/Close/pool size, TLS, or Unix sockets.

### Not landed (measured and rejected)

- A growable per-connection encode scratch with a single `net.Conn.Write`. `bufio` already coalesces a small command into one write; the scratch bought no syscall on this workload and cost a full payload copy.
- `maxIdleEncodeBuf = 64 * 1024` and the `parkIdleConn` trim branch, which existed only to stop that scratch pinning memory on a parked socket. With no scratch, there is nothing to cap.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_resp-commands`: framing allocates nothing, measured compiled; length headers via `strconv.AppendInt` and never via string concatenation; `bufio.Writer` retained and the single-write variant rejected by name; encoded bytes still match dest framing; every verb proved live on Redis and Dragonfly.
- `std_go_simpleredis_tcp-session`: unchanged. The idle-scratch requirement this change once added was removed with the scratch it described.

## Impact

- `simpleredis/resp.go` (`writeCommand`), `simpleredis/pool.go` (`pooledConn`, `dial`).
- `simpleredis/resp_test.go` (wire goldens through a `bufio.Writer` over a buffer), `simpleredis/bench_test.go` (encode benches and alloc ceilings), `simpleredis/interpretedcost_test.go` (compiled Eval-encode benches, Yaegi strategy probes).
- Run, do not rewrite: `e2e/simpleredisprobe/`, `docker-compose.yml`, `scripts/integration-tests.simpleredis.Tests.ps1`.
- Neighbors (`tokenbucket`, `windowcounter`) stay green through exported verbs.
- Main spec `std_go_simpleredis_resp-commands` after archive. `std_go_simpleredis_tcp-session` is untouched.
