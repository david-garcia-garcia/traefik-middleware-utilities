# go-redis proto Reader.readLine

How `github.com/redis/go-redis` reads one RESP line with `bufio.Reader.ReadSlice` and recovers when the line is longer than the buffer. Pinned clone: `https://github.com/redis/go-redis` @ `7f3b3dffde59329db9fa7a71d650ef373affb599` (shallow `master`, tag `v9.23.0-beta.1`, 2026-09-11).

SimpleRedis does **not** import go-redis. Copy the control flow into stdlib-only code. Do not copy `internal/util.Atoi` / `BytesToString` (`unsafe`).

Caller packet for this product's client: `knowledge/devdocs/std_go_simpleredis.md` (Init/Get/Eval). Decode ownership stays in `simpleredis/simpleredis.go`.

## ReadSlice view is invalid after the next read

`ReadSlice(delim)` returns a slice into the `bufio.Reader` buffer. Those bytes stop being valid at the next read. `ReadSlice` returns a non-nil error if and only if the line does not end in `delim`. If the buffer fills before `delim`, the error is `bufio.ErrBufferFull` and the returned slice is the data already in the buffer (no delimiter). ([Go 1.21 `bufio.Reader.ReadSlice`](https://pkg.go.dev/bufio@go1.21.13#Reader.ReadSlice), [.sources/readslice-docs.md](.sources/readslice-docs.md); [golang/go@go1.21.13:src/bufio/bufio.go](https://github.com/golang/go/blob/go1.21.13/src/bufio/bufio.go), [.sources/bufio.go.md](.sources/bufio.go.md))

`bufio.NewReader` uses buffer size **4096** (`defaultBufSize` in Go 1.21). SimpleRedis dials with `bufio.NewReader(netConn)` (`simpleredis/simpleredis.go` `dial`). A status or error line longer than 4096 bytes is the `ErrBufferFull` case to test. ([golang/go@go1.21.13:src/bufio/bufio.go](https://github.com/golang/go/blob/go1.21.13/src/bufio/bufio.go) `defaultBufSize` / `NewReader`, [.sources/bufio.go.md](.sources/bufio.go.md))

## go-redis fallback is ReadSlice, then one ReadBytes — not a ReadSlice loop

`internal/proto.Reader.readLine` (`reader.go:314-336`):

1. `b, err := r.rd.ReadSlice('\n')`.
2. If `err != nil` and `err != bufio.ErrBufferFull`, return that error.
3. On `ErrBufferFull`: `full := make([]byte, len(b)); copy(full, b)` — the partial **must** be copied before the next read, because it aliases the buffer.
4. Then **one** `r.rd.ReadBytes('\n')` for the remainder (ReadBytes allocates until it sees `'\n'`; it is not another ReadSlice).
5. `full = append(full, b...)`; use that as `b`.
6. Require `len(b) > 2`, last byte `'\n'`, second-last `'\r'`; return `b[:len(b)-2]` (CRLF stripped).

([go-redis@7f3b3dff:internal/proto/reader.go](https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/proto/reader.go), [.sources/reader.go.md](.sources/reader.go.md))

Do not implement a loop of `ReadSlice` until `err == nil`. The named pattern is copy-partial + `ReadBytes` once.

## Buffer size is not the same as SimpleRedis

go-redis `proto.DefaultBufferSize` is **32 KiB**, not 4096. Their `ErrBufferFull` path is for lines longer than 32 KiB. SimpleRedis still hits it at **4096**. Unit-test a line longer than 4096, not 32 KiB. ([go-redis@7f3b3dff:internal/proto/reader.go](https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/proto/reader.go) `DefaultBufferSize`, [.sources/reader.go.md](.sources/reader.go.md))

## Escaping + / : payloads

go-redis `ReadReply` converts `+` to `string(line[1:])` and `:` through `ParseInt` before returning, so it does not hand a ReadSlice view to the caller. ([go-redis@7f3b3dff:internal/proto/reader.go](https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/proto/reader.go) `ReadReply` `RespStatus` / `RespInt`, [.sources/reader.go.md](.sources/reader.go.md))

SimpleRedis returns `[][]byte`. After `ReadSlice`, `line[1:]` and array `head[1:]` for `+` / `:` **must** be copied before the next read or before the connection returns to the idle pool. Header-only uses (type byte, length parse) must not keep the slice across a later read.

## Length parse: do not copy go-redis Atoi

go-redis `replyLen` calls `util.Atoi(line[1:])`, and `Atoi` is `strconv.Atoi(BytesToString(b))`. `BytesToString` is `unsafe.String` (`internal/util/unsafe.go`). ([go-redis@7f3b3dff:internal/util/strconv.go](https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/util/strconv.go), [.sources/strconv.go.md](.sources/strconv.go.md); [go-redis@7f3b3dff:internal/util/unsafe.go](https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/util/unsafe.go), [.sources/unsafe.go.md](.sources/unsafe.go.md))

SimpleRedis session source MUST NOT use `unsafe` (`openspec/specs/std_go_simpleredis_tcp-session/spec.md`). Parse bulk/array lengths with a byte loop (`parseLen`) that accepts an optional leading minus so `$-1` stays a miss. Do not import go-redis.
