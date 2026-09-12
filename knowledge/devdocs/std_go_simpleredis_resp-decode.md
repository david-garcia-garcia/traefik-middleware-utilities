# RESP decode

## Language

**ReadSlice view**:
A `[]byte` from `bufio.Reader.ReadSlice` that aliases that connection reader's buffer. It is invalid after the next read on that reader, including after `release` puts the connection on the idle pool.
_Avoid_: returning it from `readReply`; storing it in an array slot; treating it as caller-owned

**Escaping payload**:
A `+` or `:` payload that leaves `readReply` (returned to the caller or stored in an array slot). It is a copy of the header bytes, not a ReadSlice view. A `-` error MAY use `string(message)` (that already copies). `$` bulk stays owned by `readBulk`'s allocated buffer.
_Avoid_: `line[1:]` or `head[1:]` without a copy; copying `$` / `*` headers after the length parse

## Overview

SimpleRedis reads each RESP line with `ReadSlice('\n')` so short headers do not allocate. Anything that must survive the next read on that connection is copied first.

## How to use

- Read a line with `ReadSlice('\n')`. On `bufio.ErrBufferFull`, copy the partial, then one `ReadBytes('\n')`, and append. Do not loop `ReadSlice`. Stay on `bufio.NewReader` (4096).
- Copy a `+` or `:` payload with `append([]byte(nil), payload...)` before the next read and before `release`.
- Parse bulk and array lengths from the bytes after the type byte (`parseLen`). Accept optional leading minus (`$-1` miss). Empty or non-digits is `redis:issue?`. Do not use `unsafe` or `strconv.Atoi(string(...))`.
- Leave `readBulk` as `make([]byte, length+2)` plus `io.ReadFull`. Do not keep a `$` or `*` header slice across a later read.

## Pattern snippet

```go
line, err := reader.ReadSlice('\n')
if err == bufio.ErrBufferFull {
	full := make([]byte, len(line))
	copy(full, line)
	remainder, remainderErr := reader.ReadBytes('\n')
	if remainderErr != nil {
		return nil, remainderErr
	}
	line = append(full, remainder...)
}
return [][]byte{append([]byte(nil), line[1:]...)}, true, nil
```

## Key files

- `simpleredis/resp.go` — `readLine`, `readReply`, `readBulk`, `parseLen`
- `simpleredis/resp_test.go` — copy-on-escape and `ErrBufferFull` (>4096)
- `simpleredis/fake_redis_test.go` — `startSequentialRedis` for distinct later-read payloads
- `simpleredis/bench_test.go` — `BenchmarkDecodeBulk`, `BenchmarkDecodeArray10`, `BenchmarkDecodeInteger`
- `openspec/specs/std_go_simpleredis_resp-decode/spec.md`

## Gotchas

- A held `+` or `:` slice that is still a ReadSlice view is overwritten by the next command on the same connection, including another goroutine after idle release.
- `ErrBufferFull` at 4096 (`bufio.NewReader` default), not go-redis 32 KiB. The remainder is one `ReadBytes`, not another `ReadSlice` loop.
- Identical EVAL `:0` replies hide aliasing. Prove copy-on-escape with distinct payloads on one connection.
- `parseLen` overflow is `false` (`redis:issue?`), not a wrap.
