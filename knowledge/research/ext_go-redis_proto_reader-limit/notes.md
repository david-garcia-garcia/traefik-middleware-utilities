# proto.Reader bulk and array size

Whether `github.com/redis/go-redis` `internal/proto.Reader` caps bulk-string or array allocations taken from a reply header. Pinned clone: `https://github.com/redis/go-redis` @ `7f3b3dffde59329db9fa7a71d650ef373affb599` (shallow `master`, tag `v9.23.0-beta.1`). Same pin as `ext_go-redis_proto_readline/`.

SimpleRedis does **not** import go-redis. This folder is the outside-system fact: there is no client-side analogue of Redis `proto-max-bulk-len` on this pin.

## No payload cap on `$` or `*`

`replyLen` is the only length gate. It `util.Atoi`s the header digits and rejects `n < -1`. `n == -1` on bulk/array/map types is `Nil`. There is no comparison against 512 MiB, `MaxInt` below overflow, or a Reader field. ([go-redis@7f3b3dff:internal/proto/reader.go](https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/proto/reader.go) `replyLen`, [.sources/reader.go.md](.sources/reader.go.md))

`readStringReply` then `make([]byte, n+2)` and `io.ReadFull`. `readSlice` then `make([]interface{}, n)` and loops `ReadReply`. `readRawReplyBuf` `append`s `make([]byte, n+2)`. A lying `$268435456` or `*1048576` allocates before any payload byte, same shape as an uncapped client. ([go-redis@7f3b3dff:internal/proto/reader.go](https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/proto/reader.go) `readStringReply` / `readSlice`, [.sources/reader.go.md](.sources/reader.go.md))

Grep of `internal/proto` on this pin finds no `MaxBulk`, `proto-max-bulk-len`, or 512 MiB constant. Stream `MaxLen` elsewhere is Redis `XTRIM MAXLEN`, not a reader bound.

## Buffer size is not a bulk limit

`proto.DefaultBufferSize` is **32 KiB**. `NewReader` / `NewReaderSize` size the `bufio.Reader`. `Options.ReadBufferSize` (0 → that default) is the same I/O buffer. A bulk larger than the buffer still allocates `n+2` and reads through `ReadFull`. `readLine` `ErrBufferFull` + `ReadBytes` is for header lines, not `$` payloads. ([go-redis@7f3b3dff:internal/proto/reader.go](https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/proto/reader.go) `DefaultBufferSize`, [.sources/reader.go.md](.sources/reader.go.md); [go-redis@7f3b3dff:options.go](https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/options.go) `ReadBufferSize`, [.sources/options.go.md](.sources/options.go.md))

See `ext_go-redis_proto_readline/` for the ReadSlice / `ErrBufferFull` control flow.

## The only caller-side size check is ReadStringInto

`ReadStringInto(buf)` compares bulk `n` to `len(buf)`. If `n` is larger it `Discard`s `n+2` and returns `redis: buffer too small`. That is the caller’s buffer, not a protocol sanity cap. Ordinary `Get` / `ReadString` / `ReadReply` do not take that path. ([go-redis@7f3b3dff:internal/proto/reader.go](https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/proto/reader.go) `ReadStringInto`, [.sources/reader.go.md](.sources/reader.go.md))

## Map-count overflow is not a bulk cap

`Discard` / raw-reply map and attribute loops iterate `n` key/value pairs (not `n*2`) so a count above `MaxInt/2` cannot wrap the loop bound. That prevents a silent desync. It does not reject a large `n` before allocating. ([go-redis@7f3b3dff:internal/proto/reader.go](https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/proto/reader.go) `Discard`, [.sources/reader.go.md](.sources/reader.go.md))
