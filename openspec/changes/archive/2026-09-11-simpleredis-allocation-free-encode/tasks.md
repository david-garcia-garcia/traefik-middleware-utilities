## 1. Encoder

- [x] 1.1 Keep `pooledConn.writer *bufio.Writer` and `bufio.NewWriter(netConn)` in `dial`; add `lenBuf [24]byte` to `pooledConn` for length digits
- [x] 1.2 Rewrite `writeCommand(conn *pooledConn, args)` to frame through `conn.writer` with `WriteByte`, `writer.Write(strconv.AppendInt(conn.lenBuf[:0], int64(n), 10))`, `WriteString("\r\n")`, the payload, then `Flush`; no string concatenation anywhere in framing
- [x] 1.3 Do not add a growable encode scratch, a single `net.Conn.Write`, or an idle-retention cap for one; the measurement in `design.md` rejects that shape
- [x] 1.4 Comment the encoder with where the allocation win comes from and why the digit scratch is conn-owned; comment `pooledConn` with the measured rejection of the single-write variant

## 2. Proof

- [x] 2.1 Compiled goldens through a `bufio.Writer` over a buffer: GET `session:9f2c1ab4-user-token` equals `*2\r\n$3\r\nGET\r\n$27\r\nsession:9f2c1ab4-user-token\r\n`, and native MSETEX for two pairs with EX 60 equals its eight-slot frame
- [x] 2.2 `BenchmarkEncodeGet`, `BenchmarkEncodeEval`, `BenchmarkEncodeMSetEX`, `BenchmarkEncodeSet100KB` run on the production encoder; hoist the argv where the bench is meant to gate framing
- [x] 2.3 `TestAllocEncodeGet` / `TestAllocEncodeMSetEX` / `TestAllocEncodeSet100KB` assert 0 allocs/op and 0 B/op with no slack; `TestAllocEncodeEval` keeps an argv-build budget
- [x] 2.4 Hoist `b.Helper()` out of the encode bench inner loop; per iteration it cost about 83 ns and swamped the encode being measured
- [x] 2.5 Keep `TestValueWithNewlinesSurvives`, the RESP injection test, and the fake-server argv assertions green; `go build ./...`, `go vet ./...`, `go test ./simpleredis/ -count=1`, `go test ./... -count=1 -short`

## 3. Yaegi

- [x] 3.1 Keep `BenchmarkYaegiEncodeBufio` and `BenchmarkYaegiEncodeSingleWrite`; point the `bufio` probe at the shape production actually ships
- [x] 3.2 Record in code, spec and devdoc that the rejected shape is cheaper interpreted and that we accept that cost because production is compiled; do not delete or soften the result
- [x] 3.3 Interpreted suites that exercise the encode path stay green: `TestYaegi_*` happy paths, `TestYaegiErrorpath_*`, and `TestYaegiLive*`

## 4. Dual-engine e2e

- [x] 4.1 Run compose + Pester `/redis` and `/dragonfly` plus `e2e/simpleredisprobe` until every existing verb succeeds on both engines; do not rewrite `kongIncrbyExpireatScript` or the Dragonfly compose pin
- [x] 4.2 Confirm Eval stays Lua 5.1-safe with KEYS listed; reclaim `/a` `/b` stay green; the SimpleRedis Describe does not stop `whoami-a` or `whoami-b`

## 5. Specs and record

- [x] 5.1 `std_go_simpleredis_resp-commands`: framing allocates nothing measured compiled, `AppendInt` never concatenation, `bufio` retained, single-write rejected by name with the numbers that settle it
- [x] 5.2 `std_go_simpleredis_tcp-session`: remove the 64 KiB idle-scratch requirement and its scenario; the file is otherwise identical to dest and its requirement count is unchanged at 27
- [x] 5.3 `knowledge/devdocs/std_go_simpleredis.md`: encode bullet describes the shipped encoder, keeps "do not justify this with the interpreted benches", drops the 64 KiB sentence
- [x] 5.4 Name this change for what landed, not for the approach that was rejected
