# perf-07 — `readLine` copies every line via `ReadBytes`

issueHost: local
issueRef: none
Finding: `simpleredisfixes/perf-07-readslice-decoding.md`
Index: `simpleredisfixes/README.md`

## Conductor HARD REQUIREMENT (desired behavior)

- tests MUST run against both Redis and Dragonfly. Both are supported backends.
- Copy-on-escape and long-line ErrBufferFull must be unit-tested.
- live Get/MGet/Incr/Eval replies on both engines must still be correct after ReadSlice (compose + Pester `/redis` `/dragonfly`, `e2e/simpleredisprobe`).
- Lua 5.1-safe. Dragonfly KEYS required.

## Finding

# perf-07 — `readLine` copies every line via `ReadBytes`

- **Axis**: Performance
- **Severity**: judgement
- **Where**: `simpleredis/simpleredis.go:408-417` (`readLine`), `:329-386` (`readReply`), `:389-405` (`readBulk`)
- **Status**: not applied

## What I found

Every protocol line is read with `bufio.Reader.ReadBytes`, which allocates a fresh
slice and copies into it:

```go
func readLine(reader *bufio.Reader) ([]byte, error) {
	line, err := reader.ReadBytes('\n')
```

Most of those copies are thrown away immediately. `readReply` and `readBulk` use the
line only to look at the type byte and parse a length:

```go
case '*':
	count, convErr := strconv.Atoi(string(line[1:]))
```

```go
length, err := strconv.Atoi(string(head[1:]))
```

So for a `$` header the allocated line is parsed and discarded, and for an array
header the same. Only `+` and `:` replies actually hand the line contents back to
the caller (`return [][]byte{line[1:]}, true, nil` at `:340`), and those are the only
cases that genuinely need a copy.

Measured client-side decode cost:

| Reply shape | ns/op | B/op | allocs/op |
|---|---|---|---|
| Bulk string (`$17`) | 47 | 53 | 3 |
| 10-slot array (`MGET`) | 325 | 408 | 22 |
| Integer (`:1234567`) | 39 | 40 | 2 |

The 10-slot array pays 11 line reads (one array header plus 10 bulk headers) and 10
payload allocations, which is where the 22 allocations come from.

## Why it matters

Not for latency — 325 ns against an 86,088 ns `MGet` round trip is 0.4%. It matters
for allocation rate in a shared process: a Traefik instance doing 20,000 cache reads
per second through `MGet(10)` allocates ~440,000 objects per second in this one
function, all of them short-lived garbage. That is GC pressure paid by every other
middleware in the same process, and it is avoidable.

There is also a modest interpreted-path benefit, for the same reason as
[perf-06](perf-06-single-write-encoding.md): `ReadBytes` is a native call whose
result the interpreter must then wrap, and `strconv.Atoi(string(line[1:]))` is two
more calls plus a conversion per header. Replacing them with `ReadSlice` and a
byte-wise length parse removes calls from the per-reply path, which is what the
interpreter charges for.

## Expected gain

- **Roughly half the decode allocations**: 3 → ~1 for a bulk reply, 22 → ~11 for a
  10-slot array (the payload copies in `readBulk` are unavoidable, since the API
  returns `[][]byte` the caller keeps).
- Proportionally larger for `MGet` and `Eval` array replies, which is exactly what
  the planned limiters read.
- Small interpreted saving from the removed per-line native calls; not separately
  measured, and it should not be the justification.

## How to fix

This is go-redis's `internal/proto.Reader.readLine`, which handles the one hazard
that makes `ReadSlice` different:

- Switch `readLine` to `reader.ReadSlice('\n')`, which returns a view into the
  existing `bufio` buffer and allocates nothing.
- Handle `bufio.ErrBufferFull` explicitly: a line longer than the 4096-byte buffer
  returns that error with a partial slice, and the fallback must accumulate into a
  fresh buffer and keep reading, exactly as go-redis does. Skipping this is a
  correctness bug on long `+`/`-` lines, not just a perf detail.
- **Copy where the result escapes.** The slice is only valid until the next read on
  that reader, so the `+` and `:` cases at `:340` must copy `line[1:]` before
  returning it — those values are handed to the caller and, for `:` replies inside
  arrays (`:376-377`), stored into `values[i]`. Getting this wrong produces
  reply-to-reply corruption that tests may not catch, so it is the part to review
  hardest.
- Parse lengths straight from the bytes instead of `strconv.Atoi(string(...))`. A
  small `parseLen([]byte) (int, bool)` loop avoids both the conversion and the
  `strconv` call. Note that `strconv.Atoi(string(b))` does **not** currently
  allocate for short inputs — the compiler keeps the conversion on the stack, which
  the measured 0 B/op for integer parsing confirms — so this part is about call
  count, not allocations.

`readBulk`'s `make([]byte, length+2)` at `:400` stays: that payload is returned to
the caller. It could shrink to `make([]byte, length)` plus a two-byte CRLF discard
if the trailing slack is ever a concern, but that is cosmetic.

## How to prove it

`BenchmarkDecodeBulk`, `BenchmarkDecodeArray10` and `BenchmarkDecodeInteger` in
`simpleredis/bench_test.go` are the allocation guards; assert the new numbers there.
The correctness risk needs targeted tests: a `+`/`:` reply whose returned value is
still correct after a *subsequent* read on the same connection (this is the aliasing
trap), and a status or error line longer than 4096 bytes to exercise the
`ErrBufferFull` path. Neither exists today.

## Index (simpleredis review)

Review of `simpleredis/` for hot-path efficiency and test coverage. One file per
finding. Nothing here is applied to `simpleredis/simpleredis.go` yet.

RESP encode/decode is **not** the bottleneck: client-side encode plus decode for a
`Get` is ~124 ns against a ~17,500 ns round trip, under 1%. The costs that matter are
connection management, one round trip per command, ~500 bytes of Lua re-sent on
every `EVAL`, and Yaegi interpretation.

Finding [perf-07](perf-07-readslice-decoding.md): Performance, judgement — `readLine` copies every line via `ReadBytes`. Suggested order places perf-07 with perf-06 after pool, failure-mode tests, EVALSHA/pipelining/MSETEX.
