# perf-06 — Per-argument write pattern costs 1,380 ns per command interpreted

issueHost: local. issueRef: none. Finding and index from the caller (files in the main checkout; not on origin/master).

- Finding: `simpleredisfixes/perf-06-single-write-encoding.md`
- Index: `simpleredisfixes/README.md`

## HARD REQUIREMENT (conductor)

Tests MUST run against both Redis and Dragonfly. Both are supported backends. Wire bytes must stay identical; prove every existing verb still works live on both engines after the encoder change (compose + Pester `/redis` `/dragonfly`, `e2e/simpleredisprobe`). Keep compiled allocation guards and Yaegi encode benchmarks. Lua 5.1-safe. Dragonfly KEYS required.

## Finding

- **Axis**: Performance
- **Severity**: judgement (but the largest client-CPU win in the interpreted path)
- **Where**: `simpleredis/simpleredis.go:310-326` (`writeCommand`), `:271-272` (`bufio` allocation in `dial`)
- **Status**: not applied

`writeCommand` builds each length header by string concatenation and issues three separate writer calls per argument. A two-argument `GET` therefore performs 7 writer calls, 3 `strconv.Itoa` calls and 3 string concatenations, then a `Flush`.

Measured two ways. Compiled, this is cheap — 77 ns, 48 B, 5 allocs. Interpreted under Yaegi, encoding the same `GET` against `io.Discard`:

| Encoding strategy | Interpreted |
|---|---|
| Current per-argument `bufio` path | **4,014 ns** |
| One scratch buffer, `strconv.AppendInt`, single `Write` | **2,635 ns** |

That is a **1,380 ns saving per command, 34% of the encode cost**, and it is 52x larger than the entire compiled encode cost of 77 ns.

Each call from interpreted code into a native stdlib symbol crosses the interpreter's reflect boundary. Collapsing 7 writer calls plus 3 `Itoa` calls into a handful of `append` operations and one `Write` removes most of those crossings.

This is the finding that inverts the usual intuition. The allocation savings are worth almost nothing on their own. The call-count reduction is worth 1,380 ns per command in the mode this library actually ships in as a Traefik plugin.

Secondary benefit: writing a complete frame in one call makes the `bufio.Writer` redundant, so `dial` stops allocating a 4096-byte write buffer per connection (`bufio.NewWriter(netConn)` at `:272`). Keep the `bufio.Reader`.

### How to fix

go-redis's `internal/proto.Writer` pattern (`writeLen`, `numBuf`, `lenBuf`) without `interface{}` dispatch:

- Add a reusable `buf []byte` field to `pooledConn`.
- Build the whole frame with `append` and `strconv.AppendInt`, then one `netConn.Write(buf)`.
- Cap the retained capacity when releasing a connection (for example, drop the buffer if it grew past 64 KB after a large `SET`).
- Drop `bufio.Writer` from `pooledConn` entirely. Keep the `bufio.Reader`.

### How to prove it (finding)

`BenchmarkYaegiEncodeBufio` and `BenchmarkYaegiEncodeSingleWrite` in `simpleredis/interpretedcost_test.go` already compare the two strategies interpreted; after the change, the production path should match the latter. Keep `BenchmarkEncodeGet` and `BenchmarkEncodeEval` as compiled allocation guards. All existing protocol tests must pass unchanged — the bytes on the wire are identical, which `TestValueWithNewlinesSurvives` and the `argv` assertions already check.

## Index (one-paragraph + this row)

RESP encode/decode is not the bottleneck compiled. In the deployment mode this library ships in, Yaegi interpretation adds ~27,000 ns per command and is driven by how many interpreted statements and calls each command executes.

| # | Axis | Severity | Finding |
|---|---|---|---|
| perf-06 | Performance | judgement | Per-argument write pattern costs 1,380 ns per command interpreted |

Suggested order places perf-06 / perf-07 after pool, coverage, EVALSHA/pipelining/MSETEX. This ticket is perf-06 only.
