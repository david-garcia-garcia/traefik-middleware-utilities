# simpleredis review: findings index

Review of `simpleredis/` for hot-path efficiency and test coverage. One file per
finding. Nothing here is applied to `simpleredis/simpleredis.go` yet.

## The one-paragraph version

RESP encode/decode is **not** the bottleneck: client-side encode plus decode for a
`Get` is ~124 ns against a ~17,500 ns round trip, under 1%. The costs that matter are
connection management (measured: 86% of commands paid a fresh TCP handshake under
bursty load), one round trip per command with no pipelining, ~500 bytes of Lua re-sent
on every `EVAL`, and — in the deployment mode this library actually ships in — Yaegi
interpretation, which adds ~27,000 ns per command and is driven by how many
interpreted statements and calls each command executes, not by how many bytes it
allocates.

## Measured baseline

Go 1.25.6, windows/amd64, Intel Core Ultra 7 265K, against the in-process fake RESP
server on loopback. End-to-end rows include the fake server's own work, so the
client-only column is the part this library controls.

| Path | End-to-end | Client-only |
|---|---|---|
| `Get` | 17,481 ns, 216 B, 19 allocs | 124 ns, 101 B, 8 allocs |
| `MGet` (10 keys) | 86,088 ns, 1,765 B, 111 allocs | 325 ns, 408 B, 22 allocs |
| `Incr` | 18,357 ns, 172 B, 16 allocs | 39 ns, 40 B, 2 allocs |
| `Eval` (~470 B script) | 20,002 ns, 2,237 B, 47 allocs | 365 ns, 800 B, 19 allocs |
| `Get`, interpreted under Yaegi | 44,417 ns | — |

Interpretation costs ~26,900 ns per `Get` (44,417 interpreted vs 17,481 compiled, same
compiled fake server on both sides). Within that, an interpreted function call costs
~165 ns (measured: 383 ns vs 218 ns for the same conversion inline vs behind a helper),
which is why *call and statement count per command* is the lever in the interpreted
path.

Statement coverage was 86.1% before this review and 87.6% after adding the
measurement tests below.

## Findings

| # | Axis | Severity | Finding |
|---|---|---|---|
| [perf-01](perf-01-connection-pool-cap.md) | Performance | hard | No cap on total connections; bursty traffic redials constantly |
| [perf-02](perf-02-io-timeout-fan-out.md) | Performance | hard | Fixed 1 s I/O timeout plus no pool cap fans out under Redis slowness |
| [perf-03](perf-03-idle-reaper-tail-only.md) | Performance | judgement | Idle reaper only ever inspects the newest connection |
| [perf-04](perf-04-pipelining.md) | Performance | judgement | No pipelining: one round trip per command |
| [perf-05](perf-05-evalsha.md) | Performance | judgement | `Eval` ships the whole script on every call |
| [perf-06](perf-06-single-write-encoding.md) | Performance | judgement | Per-argument write pattern costs 1,380 ns per command interpreted |
| [perf-07](perf-07-readslice-decoding.md) | Performance | judgement | `readLine` copies every line via `ReadBytes` |
| [perf-08](perf-08-unsafe-zero-copy.md) | Performance | judgement | `unsafe` zero-copy: measured, and it loses under Yaegi |
| [test-01](test-01-server-closed-eof-redial.md) | Coverage | hard | The realistic stale-connection case (server closed, `io.EOF`) is untested |
| [test-02](test-02-idle-cap-and-release-after-close.md) | Coverage | hard | Pool cap and release-after-`Close` never execute |
| [test-03](test-03-dial-auth-select-failure.md) | Coverage | hard | `dial`'s `AUTH` and `SELECT` failure branches are untested |
| [test-04](test-04-truncated-bulk-payload.md) | Coverage | hard | Truncated bulk payload — the anti-corruption guard — is untested |
| [test-05](test-05-non-idempotent-retry.md) | Coverage | hard | `exec` retries `INCR`/`INCRBY`/`EVAL` after a lost reply |
| [test-06](test-06-malformed-reply-branches.md) | Coverage | judgement | Malformed-reply branches unexercised |
| [test-07](test-07-hot-path-benchmarks.md) | Coverage | judgement | No benchmark or allocation guard existed |
| [feat-01](feat-01-msetex.md) | Feature | judgement | `MSETEX`: many keys, one TTL, one round trip (Lua fallback for Redis and Dragonfly) |

## Suggested order

1. **perf-01 / perf-02** together — one bounded-pool change fixes both, and they are the
   only findings that turn a slow dependency into a Traefik-wide failure.
2. **test-01 through test-05** — these are the failure modes perf-01 exposes, and
   test-05 is a policy decision that should be made before the rate limiters land.
3. **perf-05 / perf-04 / feat-01** — biggest wire and latency wins for the planned
   limiters. Do these together: `EVALSHA`, pipelining and the `MSETEX` fallback all
   need the same script-and-fallback plumbing.
4. **perf-06 / perf-07** — real interpreted-path CPU savings, contained changes.
5. **perf-08** — recommended *not* to adopt; the file records why, with numbers.

## Reproducing every number

Benchmarks and measurement tests live in `simpleredis/bench_test.go` (compiled
encode/decode, end-to-end, connection churn) and `simpleredis/interpretedcost_test.go`
(Yaegi capability probe, interpreted encode and conversion costs, interpreted `Get`).

```sh
go test ./simpleredis/ -run XXX -bench . -benchtime 100000x -count 3
go test ./simpleredis/ -run "TestConnectionChurn|TestYaegiUnsafeVariants" -v -count 1
go test ./simpleredis/ -count 1 -coverprofile=cover.out && go tool cover -func=cover.out
```
