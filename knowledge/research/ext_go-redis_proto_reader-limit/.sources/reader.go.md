---
url: https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/proto/reader.go
title: go-redis internal/proto/reader.go replyLen and allocations
fetched: 2026-09-12
authority: source
ref: github.com/redis/go-redis@7f3b3dffde59329db9fa7a71d650ef373affb599:internal/proto/reader.go
---

`DefaultBufferSize` is `32 * 1024`. `NewReader` uses `bufio.NewReaderSize(rd, DefaultBufferSize)`. `NewReaderSize` uses the caller size. `Size()` returns the bufio buffer size.

`replyLen(line)`:
- `n, err = util.Atoi(line[1:])`
- `n < -1` → `redis: invalid reply`
- for `$` `=` `!` `*` `~` `>` `%` `|`, `n == -1` → `Nil`
- otherwise returns `n` with no upper bound

`readStringReply`: `n := replyLen`; `b := make([]byte, n+2)`; `io.ReadFull`; return `b[:n]` as string.

`readSlice`: `n := replyLen`; `val := make([]interface{}, n)`; loop `ReadReply`.

`ReadStringInto`: on `RespString`, if `n > len(buf)` then `Discard(n+2)` and `redis: buffer too small: need %d bytes, have %d`. No package-wide max.

`Discard` map/attr: loop `n` pairs (not `n*2`) so `n > MaxInt/2` cannot overflow the bound.

`maxPushHeaderPeek` is `4096` (push-name peek window only). `MinRESP3ReadBufferSize` is `128` (minimum bufio size so a RESP3 push header fits).
