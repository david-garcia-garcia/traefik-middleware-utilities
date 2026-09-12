---
url: https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/proto/reader.go
title: go-redis internal/proto/reader.go readLine and ReadReply
fetched: 2026-09-11
authority: source
ref: github.com/redis/go-redis@7f3b3dffde59329db9fa7a71d650ef373affb599:internal/proto/reader.go
---

`DefaultBufferSize` is `32 * 1024` (32 KiB) for go-redis proto readers.

`readLine` (`:314-336`):
- `ReadSlice('\n')`.
- On error other than `bufio.ErrBufferFull`, return the error.
- On `ErrBufferFull`: `make`+`copy` the partial, then one `ReadBytes('\n')`, `append`, use the combined slice.
- Require `len(b) > 2`, last byte `'\n'`, second-last `'\r'`; return `b[:len(b)-2]`.

`ReadReply` status (`+`) returns `string(line[1:])`. Integer (`:`) returns `util.ParseInt(line[1:], 10, 64)`. Neither returns the ReadSlice view to the caller.

`replyLen` (`:837`) uses `util.Atoi(line[1:])` and treats `n == -1` as nil for bulk/array types.
