## Context

Dest `writeCommand` concatenates `"*" + strconv.Itoa(len(args)) + "\r\n"`, then per argument `"$" + strconv.Itoa(len(arg)) + "\r\n"`, `Write(arg)`, `"\r\n"`, then `Flush` on the 4 KiB `bufio.Writer` that `dial` allocates. Sole product caller: `do`. Neighbors call exported verbs only. Usage: `knowledge/devdocs/std_go_simpleredis.md`. Research: `knowledge/research/ext_dragonfly_eval/`. Proceed policies: `devstate/explore.md`.

This change was proposed as "single-write encode" and reduced to "allocation-free encode" after the four encoders below were compiled and measured. The folder is named for what landed. The rejected shape is recorded here, and forbidden in `std_go_simpleredis_resp-commands`, so it cannot be reintroduced as an optimisation without meeting this evidence first.

## Goals / Non-Goals

**Goals:**
- Zero allocations in framing, measured compiled, and gated in CI at zero with no slack.
- Wire-identical framing, dual-engine live proof, compiled encode benches on the production encoder.
- A decision record that explains why the obvious "one write" idea is worse here.

**Non-Goals:**
- Reducing the number of `write` syscalls. Measurement says there is nothing to reduce on this workload.
- Interpreted encode speed. Production is compiled; see Decision 5.
- perf-07 decode, perf-04 pipelining, perf-05 EVALSHA, perf-08 `unsafe`, feat-01 MSETEX.

## The measurement

Four encoders that emit byte-identical RESP, compiled, windows/amd64, with the number of `Write` calls reaching the socket counted alongside:

| Encoder | writes, small GET | writes, 100 KiB SET | GET | 100 KiB SET |
|---|---|---|---|---|
| A. `bufio` + `"$" + strconv.Itoa(n) + "\r\n"` (dest) | 1 | 3 | 55 ns, 3 allocs | 118 ns, 5 allocs |
| B. growable scratch + one `Write` | 1 | 1 | 11 ns, 0 allocs | 1590 ns, 0 allocs |
| C. `bufio` + `strconv.AppendInt` into a **stack** array | 1 | 3 | 39 ns, 1 alloc | 82 ns, 1 alloc |
| D. `bufio` + `strconv.AppendInt` into a **conn-owned** array | 1 | 3 | 28 ns, 0 allocs | 65 ns, 0 allocs |

What the table settles:

1. **The allocation win was never the single write.** It is removing the string concatenation. A is the only row that builds strings and the only row with allocations that framing itself caused.
2. **`bufio` already coalesced a small command into exactly one write.** Dropping it bought zero syscalls on the workload this repo has (counters, TTLs, short Lua). It bought two syscalls only for a large payload, and paid a full memcpy of that payload for them.
3. **D dominates B.** Allocation-free like B, no growable buffer and therefore no 64 KiB clamp to specify and test, and 24x faster than B on a 100 KiB payload (65 ns against 1590 ns) because `bufio` hands a large slice straight to the socket instead of copying it.
4. **C is why the scratch must live on the connection.** A local array escapes to the heap when it is handed to `bufio.Writer.Write`, because that argument can flow to the underlying `io.Writer` interface. That escape is C's 1 alloc / 24 B.
5. **The same escape costs D two allocations per command that B did not pay, outside framing.** Framing is 0 allocs in both. End to end against the in-process fake server, `BenchmarkGet` measures 29 allocs/op on dest, 26 on D, 24 on B. The gap is the argv, not the encoder: `Get` builds `[]byte("GET")` and `[]byte(name)`, D hands those slices to `bufio.Writer.Write`, and the same interface flow that sinks C sinks them (`go build -gcflags=-m` reports `([]byte)("GET") escapes to heap` on D and `does not escape` on B, which copied every argument into its scratch). Accepted: two allocations and 16 B/op per command, against B's 24x penalty on a large payload and the retention machinery in Decision 3.

## Decisions

1. **Ship D.** `writeCommand(conn *pooledConn, args [][]byte)` writes `'*'` with `WriteByte`, the count with `writer.Write(strconv.AppendInt(conn.lenBuf[:0], int64(len(args)), 10))`, `"\r\n"` with `WriteString`, then the same three per argument around the payload, then `Flush`. No string is constructed. Alternative A (dest) allocates one string per header; alternatives B and C are rejected below.

2. **`lenBuf [24]byte` on `pooledConn`.** 24 bytes takes any `int64` in decimal with room to spare. Per-conn and lock-free: borrow/release already serialises one socket. It is a field rather than a local because of finding 4 above — a local costs one alloc per command. Alternative: package-level buffer — races across concurrent connections.

3. **Reject B, the single `net.Conn.Write` over a growable scratch.** It was implemented, measured, and removed. It buys no syscall on a small command, costs a full payload copy on a large one (1590 ns against 65 ns at 100 KiB), and drags in a retention problem of its own: a 100 KiB SET leaves a 100 KiB scratch attached to a socket that then parks in the idle pool for up to `IdleTimeout`, so it needed `maxIdleEncodeBuf = 64 * 1024`, a trim branch in `parkIdleConn`, a spec requirement, and a test. All of that is machinery in service of a syscall count that was already 1. `std_go_simpleredis_resp-commands` now forbids reintroducing it.

4. **Reject C, the stack array.** Same wire, same call shape, one alloc per command because the array escapes through `bufio.Writer.Write`. It exists in the record only to justify Decision 2.

5. **Compiled is the measurement that decides.** Production is compiled into Traefik. Yaegi matters for two other reasons — the package must stay loadable as a plugin for operators who install it that way, and the e2e suite uses Yaegi plugins because rebuilding Traefik costs minutes — but interpreted speed is not the optimisation target and MUST NOT justify an encoder shape. Interpreted, the shape we ship is the expensive one: measured on a GET, 6330 ns / 166 allocs through `bufio` against 2599 ns / 69 allocs through the rejected scratch, because every call crossing the Yaegi boundary is costly and D makes five per argument. `BenchmarkYaegiEncodeBufio` and `BenchmarkYaegiEncodeSingleWrite` stay in the suite recording exactly that, as a cost taken with open eyes. The `bufio` probe mirrors the shipped encoder; when it still built headers by concatenation it measured 4039 ns / 108 allocs, which is why the older recorded gap was narrower.

6. **Framing alloc ceilings are zero with no slack.** `TestAllocEncodeGet`, `TestAllocEncodeMSetEX`, and `TestAllocEncodeSet100KB` assert 0 allocs/op and 0 B/op. The package convention elsewhere is measured + 1 alloc of slack; framing gets none, because zero is an invariant here and one allocation back means a header went through a string again. `TestAllocEncodeEval` keeps a budget (measured 9 allocs, ceiling 10) because that bench rebuilds its argv each op on purpose; none of those allocations is framing.

7. **Goldens assert the bytes, through the real writer.** `TestWriteCommandGetMatchesDestFraming` and `TestWriteCommandMSetEXMatchesDestFraming` run the production `writeCommand` against a `bufio.Writer` over a `bytes.Buffer` and compare the full frame, including `$27` and the eight-slot `MSETEX`. Alternative: argv-only assertions through the fake server — `readCommand` would not notice extra or missing framing bytes.

8. **Spec host is `std_go_simpleredis_resp-commands` alone.** The encode requirement lives there, with the rejection written into it so the requirement forbids the regression instead of merely describing today. `std_go_simpleredis_tcp-session` goes back to exactly what dest has: the idle-scratch requirement was deleted with the scratch, and its requirement count is unchanged from dest at 27.

## Risks / Trade-offs

- [Interpreted plugins pay more per command than they would under B] → Accepted and recorded in Decision 5. The e2e suite is the only place this session runs interpreted at volume, and a few microseconds per command is invisible next to an HTTP round trip.
- [A future reader sees five writer calls per argument and "optimises" them into one buffer] → Mitigation: the rejection is a SHALL NOT in `std_go_simpleredis_resp-commands`, a comment on `pooledConn`, and this file.
- [`lenBuf` sized wrong] → Mitigation: 24 bytes exceeds the widest `int64` in decimal; `AppendInt` into `conn.lenBuf[:0]` would reallocate rather than corrupt if it ever did not fit, costing an allocation the CI guard would catch.
- [Dragonfly Lua 5.4 vs Redis 5.1] → Mitigation: the probe script already lists KEYS and has no `table.maxn`; e2e on both engines is required.

## Migration Plan

Library-internal encode. Rollback is revert. No production deploy. No public API change. No wire change: the goldens are byte-for-byte identical to dest.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
