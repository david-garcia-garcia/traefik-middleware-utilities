## 1. Encoder

- [x] 1.1 Add unexported `appendRESP` that builds dest framing (`*` count, per-arg `$` length, payload, CRLF) with `append` + `strconv.AppendInt`; add compiled golden that GET `session:9f2c1ab4-user-token` equals `*2\r\n$3\r\nGET\r\n$27\r\nsession:9f2c1ab4-user-token\r\n`
- [x] 1.2 Replace `pooledConn.writer` with `buf []byte`; drop `bufio.NewWriter` from `dial`; keep `bufio.Reader`; `writeCommand(conn *pooledConn, args)` appends into `conn.buf[:0]`, one `netConn.Write`, stores the grown slice; short write or err returns that error
- [x] 1.3 Add `maxIdleEncodeBuf = 64 * 1024`; on reusable `release`, if `cap(conn.buf) > maxIdleEncodeBuf` set `conn.buf = nil`; add same-package test that a large SET then idle reuse leaves cap ≤ 64 KiB
- [x] 1.4 Keep `TestValueWithNewlinesSurvives` and argv assertions green; run `go test ./simpleredis/...` (excluding Yaegi benches if needed) until compiled tests pass

## 2. Benches

- [x] 2.1 Land `BenchmarkEncodeGet` and `BenchmarkEncodeEval` on the production encoder (`appendRESP` + one Write to `io.Discard`); do not copy untracked review files
- [x] 2.2 Widen `writeGopathFile` to `testing.TB`; land `BenchmarkYaegiEncodeBufio` and `BenchmarkYaegiEncodeSingleWrite` as encodeprobe strategy probes; do not land decode, unsafe, pool-churn, or `BenchmarkYaegiGet`
- [x] 2.3 Run compiled encode benches and Yaegi encode benches until they execute; production encode matches the single-write strategy

## 3. Dual-engine e2e

- [x] 3.1 Run compose + Pester `/redis` and `/dragonfly` plus `e2e/simpleredisprobe` until every existing verb (Set, Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, Eval) succeeds on both Redis and Dragonfly; do not rewrite `kongIncrbyExpireatScript` or the Dragonfly compose pin unless the encoder change forces it
- [x] 3.2 Confirm Eval stays Lua 5.1-safe with KEYS listed; reclaim `/a` `/b` stay green; SimpleRedis Describe does not stop `whoami-a` or `whoami-b`

## 4. Specs

- [x] 4.1 Confirm deltas `std_go_simpleredis_resp-commands` and `std_go_simpleredis_tcp-session` match the landed encoder, benches, idle trim, and dual-engine proof
- [x] 4.2 Run `openspec validate simpleredis-single-write-encode --type change --strict`
