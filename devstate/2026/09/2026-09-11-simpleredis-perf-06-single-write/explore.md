# Explore
IssueKey: 2026-09-11-simpleredis-perf-06-single-write

## Concepts

- **writeCommand (dest)** — `simpleredis/simpleredis.go:309-326`. Builds `"*" + strconv.Itoa(len)` then per arg `"$" + strconv.Itoa(len)`, `Write(arg)`, `"\r\n"`, then `Flush`. A two-arg GET is 7 writer calls + 3 `Itoa` + 3 concatenations. Sole product caller: `do` at `:296`. Neighbors (`tokenbucket`, `windowcounter`, `e2e/simpleredisprobe`) call exported verbs only; they do not call `writeCommand`.
- **pooledConn write side** — field `writer *bufio.Writer` (`:44`); `dial` allocates `bufio.NewWriter(netConn)` (default 4 KiB) at `:272`; `do` passes `conn.writer`. Three sites in `simpleredis.go`. Keep `bufio.Reader`. No scratch `buf` today.
- **Yaegi call tax** — finding (`simpleredisfixes/perf-06-single-write-encoding.md`) measured interpreted GET encode vs `io.Discard`: bufio path 4,014 ns vs scratch+one Write 2,635 ns (~1,380 ns, 34%). This run **did not re-run** those Yaegi benches (they are untracked in the dirty main checkout, not on `origin/master`). Dest still has the per-arg path (read of `writeCommand`). Compiled encode is cheap (~77 ns); the ticket exists for the interpreted Traefik plugin path.
- **Wire path** — every exported verb (`Get`, `MGet`, `Set`, `Del`, `Incr`, `IncrBy`, `Expire`, `ExpireAt`, `Eval`) plus AUTH/SELECT in `dial` goes `exec` → `do` → `writeCommand`. Encoder switch is one function; argv stays the same RESP array of bulk strings.
- **Proof today** — `TestValueWithNewlinesSurvives` (`simpleredis_test.go:338`) round-trips newline bytes. `lastSetCommand` / `lastExpireCommand` / `lastEvalCommand` assert parsed argv, not a raw-byte golden. Fake `readCommand` is lenient enough that identical argv does not by itself prove identical framing bytes.
- **Live engines** — compose `redis:7-alpine` at `redis:6379` and `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2` at `dragonfly:6379`. Probe `ServeHTTP` runs Set, Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, Eval. Eval body `kongIncrbyExpireatScript` lists `KEYS[1]`, no `table.maxn` (Lua 5.1-safe; Dragonfly KEYS required — `knowledge/research/ext_dragonfly_eval/`). Pester asserts every verb header on `/redis` and `/dragonfly`.
- **Benches on dest** — `simpleredis/bench_test.go` and `interpretedcost_test.go` **not found**. Review copies in the main checkout still take `*bufio.Writer` and also contain decode/unsafe/pool benches that belong to other findings (perf-07, perf-08, perf-01).
- **Specs** — `std_go_simpleredis_resp-commands` owns command shapes + dual-engine e2e; does not specify encode call count. `std_go_simpleredis_tcp-session` owns stdlib-only session and forbids a Redis client module; does not mention `bufio.Writer`. Usage packet `knowledge/devdocs/std_go_simpleredis.md` is Init/verb contract; encoder is internal. No research gap: wire bytes are defined by today’s `writeCommand`; Dragonfly EVAL facts already in `ext_dragonfly_eval`.
- **go-redis** — finding points at `internal/proto.Writer` (`writeLen`, `numBuf`, `lenBuf`). tcp-session forbids importing that package. Copy the append+AppendInt idea, not the module, not `interface{}` dispatch.

```
dest encode (Yaegi-expensive)
  args → Itoa strings → N bufio.Writer calls → Flush → TCP

intended encode
  conn.buf[:0] → append '*' / '$' + AppendInt + payload → one netConn.Write
```

## Decisions

- Replace `writeCommand` with the finding’s scratch loop: `append` + `strconv.AppendInt`, then one `netConn.Write`. Drop `bufio.Writer` from `pooledConn` / `dial`. Keep `bufio.Reader`. Production encode must match `BenchmarkYaegiEncodeSingleWrite`’s strategy (scratch + one Write), not the bufio path.
- Scratch lives on `pooledConn.buf` (per-conn, no lock: borrow/release already serializes a socket). `release` drops the slice when `cap` exceeds 64 KiB so a large SET cannot pin that idle conn for the process lifetime.
- **Wire bytes identical to dest today.** No new verbs. Do not change Eval script language. Lua 5.1-safe. Dragonfly KEYS required. Do not rewrite `e2e/simpleredisprobe` Eval body or the Dragonfly compose pin unless the encoder change forces it (expected: run, not rewrite).
- **Tests MUST run against both Redis and Dragonfly.** Prove every existing verb still works live on both engines after the encoder change: compose + Pester `/redis` and `/dragonfly`, `e2e/simpleredisprobe`. Keep compiled allocation guards `BenchmarkEncodeGet` / `BenchmarkEncodeEval` and Yaegi encode benches `BenchmarkYaegiEncodeBufio` / `BenchmarkYaegiEncodeSingleWrite`.
- Land those four benches (or equivalent) rewritten against the new encoder. Do not copy the untracked review files as-is (they still take `*bufio.Writer` and pull in out-of-scope decode/unsafe/pool work). Widen `writeGopathFile` to `testing.TB` so Yaegi encode benches can use it. Do not widen `startFakeRedis` unless a landed bench needs the fake.
- One `Write`; if `n != len(buf)` or `err != nil`, treat the socket dirty (`do` already maps that to `reusable false`). Do not add a write-all helper (extra interpreted calls; `net.Conn` follows the `io.Writer` all-or-error contract).
- Spec host stays the two existing SimpleRedis leaves. Do not add an encode-call-count requirement; tests + dual-engine e2e + the four benches are the proof. Do not import `go-redis`. Do not edit `tokenbucket` / `windowcounter` except they stay green through the exported verbs.
- Add a compiled golden of encoded GET bytes (finding’s example argv) so “identical wire bytes” is not only argv-parsed. Keep `TestValueWithNewlinesSurvives` and argv assertions green.
- Usage docs: no produce. Encoder is internal; Language and gotchas (Eval KEYS, no `table.maxn`) stay true.

## Open questions

- Q: After the encoder change, how do we prove every existing verb still works live on both Redis and Dragonfly with identical wire bytes?
  Rank: additive asked — Desired 3 names identical wire bytes and live Redis and Dragonfly; conductor hard-requirement names compose + Pester `/redis` `/dragonfly`, `e2e/simpleredisprobe`, compiled and Yaegi encode benches, Lua 5.1-safe, Dragonfly KEYS
  Decision: assumed — run compose + Pester `/redis` and `/dragonfly` plus `e2e/simpleredisprobe` after the switch; do not rewrite the probe Eval script (`kongIncrbyExpireatScript` already lists `KEYS[1]`, no `table.maxn`) or the Dragonfly compose pin; keep `BenchmarkEncodeGet` / `BenchmarkEncodeEval` / `BenchmarkYaegiEncodeBufio` / `BenchmarkYaegiEncodeSingleWrite`; production encode matches the single-write strategy; existing protocol tests stay green
  By: explore

- Q: Exact retained-buffer cap on `release`?
  Rank: additive asked — Desired 2 names the trim; finding example is 64 KB
  Decision: assumed — named const `maxIdleEncodeBuf = 64 * 1024`; on `release` of a reusable conn, if `cap(conn.buf) > maxIdleEncodeBuf` set `conn.buf = nil`; next command reallocates. Same package test: large SET then idle reuse leaves cap ≤ 64 KiB
  By: explore

- Q: Copy the untracked review `bench_test.go` / `interpretedcost_test.go` as-is, or rewrite them against the new `writeCommand`?
  Rank: additive asked — Desired 4 names the four encode benches and says dest must land them; Unknowns name copy-vs-rewrite; Affected names `yaegi_test.go` `testing.TB` widening
  Decision: assumed — rewrite, do not copy as-is. Land `BenchmarkEncodeGet` and `BenchmarkEncodeEval` on the production encoder (scratch + one Write to `io.Discard`). Land `BenchmarkYaegiEncodeBufio` and `BenchmarkYaegiEncodeSingleWrite` as strategy probes (encodeprobe stays a standalone interpreted package). Widen `writeGopathFile` to `testing.TB`. Do not land decode, unsafe, pool-churn, or `BenchmarkYaegiGet` from those files (perf-07 / perf-08 / perf-01)
  By: explore

- Q: Does any spec leaf need an encode-path requirement, or are tests+e2e enough?
  Rank: additive incidental — Unknowns name the choice; no criterion requires a spec sentence about call count; Desired 3 is wire-identical behavior already specified as command shapes + dual-engine e2e
  Decision: resolved — fold wire-identical golden, dual-engine live proof (compose + Pester `/redis` `/dragonfly`, `e2e/simpleredisprobe`), and encode benches into `std_go_simpleredis_resp-commands`; fold idle encode-scratch trim into `std_go_simpleredis_tcp-session`. Do not add an encode-call-count or `bufio.Writer` SHALL. tcp-session stdlib-only / no `go-redis` stays as-is
  By: propose

- Q: What is `writeCommand`’s signature after `bufio.Writer` is gone?
  Rank: bounded asked — Desired 1 names scratch + one `netConn.Write` and dropping `bufio.Writer`; 1 product caller of `writeCommand` (`do` at `simpleredis.go:296`); 3 `pooledConn.writer` sites in `simpleredis.go` (`:44`, `:272`, `:296`); 0 dest test callers; neighbors use exported verbs only (searched `simpleredis/`, `tokenbucket/`, `windowcounter/`, `e2e/simpleredisprobe/` for `writeCommand` and `conn.writer`)
  Decision: assumed — unexported `writeCommand` takes `conn *pooledConn` and `args [][]byte`, appends into `conn.buf`, one `conn.netConn.Write`, stores the grown slice back. Compiled encode benches share the append loop via an unexported `appendRESP(buf []byte, args [][]byte) []byte` so they measure production framing without a live socket. Yaegi encodeprobe keeps its own copies of both strategies
  By: explore

- Q: How should a short `Write` be handled once `bufio.Writer.Flush` is gone?
  Rank: additive incidental — no criterion names write-all vs one Write; it is a means to Desired 1
  Decision: assumed — one `Write`; `n != len(buf)` or non-nil err → return that error (`do` already marks the socket dirty). Do not loop to write the remainder (half-sent RESP + deadline retry would desync; extra interpreted calls)
  By: explore

- Q: How do we prove wire bytes are identical, not only parsed argv?
  Rank: additive asked — Desired 3 names identical wire bytes; current argv helpers parse RESP (`readCommand`) and would miss extra framing if the parser stayed aligned
  Decision: assumed — add a compiled test that `appendRESP` of GET `session:9f2c1ab4-user-token` equals the dest framing `*2\r\n$3\r\nGET\r\n$28\r\n…\r\n`; keep `TestValueWithNewlinesSurvives` and argv assertions
  By: explore
