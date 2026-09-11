## Context

Dest `writeCommand` concatenates `"*" + strconv.Itoa` then per-arg `"$" + strconv.Itoa`, `Write(arg)`, `"\r\n"`, then `Flush` on a 4 KiB `bufio.Writer` allocated in `dial`. Sole product caller: `do`. Neighbors call exported verbs only. See proposal.md for why. Usage: `knowledge/devdocs/std_go_simpleredis.md`. Research: `knowledge/research/ext_dragonfly_eval/`. Proceed policies: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- One scratch encode + one `net.Conn.Write` on the production path.
- Wire-identical framing, idle scratch trim, dual-engine live proof, four encode benches.

**Non-Goals:**
- Encode-call-count or `bufio.Writer` as a spec SHALL (implementation only).
- perf-07 decode, perf-04 pipelining, perf-05 EVALSHA, perf-08 `unsafe`, feat-01 MSETEX.
- Copying untracked review bench files as-is.

## Decisions

1. **`appendRESP` then one Write.** Unexported `appendRESP(buf []byte, args [][]byte) []byte` is the dest framing (`append` + `strconv.AppendInt`). `writeCommand(conn *pooledConn, args [][]byte)` resets `conn.buf[:0]`, appends, one `conn.netConn.Write`, stores the grown slice. Compiled encode benches call `appendRESP` then `io.Discard.Write` so they measure production framing without a live socket. Alternative: keep `bufio.Writer` and only batch strings — rejected; dest tax is N writer calls plus Flush under Yaegi.

2. **Scratch on `pooledConn.buf`.** Per-conn, no lock: borrow/release already serializes a socket. Drop `writer *bufio.Writer` from `pooledConn` and `bufio.NewWriter` from `dial`. Keep `bufio.Reader`. Alternative: package-level buffer — races under concurrent conns.

3. **`maxIdleEncodeBuf = 64 * 1024`.** On `release` of a reusable conn, if `cap(conn.buf) > maxIdleEncodeBuf` set `conn.buf = nil`. Same-package test: large SET then idle reuse leaves cap ≤ 64 KiB. Alternative: always keep the grown slice — a large SET pins that idle conn for the process lifetime.

4. **One Write, no write-all loop.** `n != len(buf)` or non-nil err → return that error (`do` already marks the socket dirty). `net.Conn` follows the `io.Writer` all-or-error contract. Alternative: loop the remainder — half-sent RESP plus deadline retry would desync; extra interpreted calls.

5. **Rewrite benches, do not copy review files.** Land `BenchmarkEncodeGet` / `BenchmarkEncodeEval` on the production encoder. Land `BenchmarkYaegiEncodeBufio` / `BenchmarkYaegiEncodeSingleWrite` as strategy probes in a standalone interpreted package (encodeprobe keeps its own copies of both strategies). Widen `writeGopathFile` to `testing.TB`. Do not land decode, unsafe, pool-churn, or `BenchmarkYaegiGet`. Alternative: copy untracked `bench_test.go` / `interpretedcost_test.go` — they still take `*bufio.Writer` and pull in out-of-scope work.

6. **Wire golden + dual-engine e2e.** Compiled test that encoded GET `session:9f2c1ab4-user-token` equals dest framing. Keep `TestValueWithNewlinesSurvives` and argv assertions. Run compose + Pester `/redis` `/dragonfly` plus `e2e/simpleredisprobe`. Do not rewrite `kongIncrbyExpireatScript` (already `KEYS[1]`, no `table.maxn`) or the Dragonfly compose pin. Do not import `go-redis`. Alternative: argv-only proof — `readCommand` would miss extra framing.

7. **Spec host is the two existing leaves.** Fold wire-identical + dual-engine + benches into `std_go_simpleredis_resp-commands`. Fold idle scratch trim into `std_go_simpleredis_tcp-session`. Do not add an encode-call-count or `bufio.Writer` requirement. Usage docs: no produce (encoder is internal).

## Risks / Trade-offs

- [Short `Write` leaves a half-sent RESP] → Mitigation: treat as dirty; `do` already sets `reusable false`; no remainder loop.
- [Yaegi encodeprobe drifts from production `appendRESP`] → Mitigation: compiled golden pins dest bytes; production path calls `appendRESP`; encodeprobe copies are strategy probes only.
- [Large SET after trim reallocates next command] → Mitigation: 64 KiB cap is the idle bound; hot reuse below the cap keeps the scratch.
- [Dragonfly Lua 5.4 vs Redis 5.1] → Mitigation: probe script already lists KEYS and has no `table.maxn`; e2e on both engines is required.

## Migration Plan

Library-internal encode. Rollback is revert. No production deploy. No public API change.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
