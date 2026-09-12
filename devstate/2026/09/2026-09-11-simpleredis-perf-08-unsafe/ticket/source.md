# Caller spec: perf-08 unsafe zero-copy

Local dump. Finding file `simpleredisfixes/perf-08-unsafe-zero-copy.md` plus the index context needed to read it. Product intent from the caller: **do not adopt** unsafe zero-copy. Do not add unsafe conversions. Do not set `useUnsafe`. Land a durable spec/devdocs decision plus keep/extend the Yaegi unsafe-variant tests as guards so a future change cannot sneak unsafe in.

Tests MUST run against both Redis and Dragonfly. Both are supported backends. Existing verbs remain proven on both engines (compose + Pester `/redis` `/dragonfly`). Lua 5.1-safe. Dragonfly KEYS required.

## Index context (`simpleredisfixes/README.md`)

Review of `simpleredis/` for hot-path efficiency and test coverage. One file per finding. Nothing here is applied to `simpleredis/simpleredis.go` yet.

RESP encode/decode is **not** the bottleneck: client-side encode plus decode for a `Get` is ~124 ns against a ~17,500 ns round trip, under 1%. The costs that matter are connection management, one round trip per command, ~500 bytes of Lua re-sent on every `EVAL`, and — in the deployment mode this library actually ships in — Yaegi interpretation (~27,000 ns per command), driven by how many interpreted statements and calls each command executes.

Findings table row: [perf-08](perf-08-unsafe-zero-copy.md) — Performance — judgement — `unsafe` zero-copy: measured, and it loses under Yaegi.

Suggested order item 5: **perf-08** — recommended *not* to adopt; the file records why, with numbers.

Reproducing numbers (index): benchmarks live in `simpleredis/bench_test.go` and `simpleredis/interpretedcost_test.go` (Yaegi capability probe, interpreted encode and conversion costs, interpreted `Get`).

```sh
go test ./simpleredis/ -run XXX -bench . -benchtime 100000x -count 3
go test ./simpleredis/ -run "TestConnectionChurn|TestYaegiUnsafeVariants" -v -count 1
```

## Finding (perf-08)

# perf-08 — `unsafe` zero-copy: measured, and it loses under Yaegi

- **Axis**: Performance
- **Severity**: judgement — **recommendation is not to adopt**
- **Where**: would affect `simpleredis/simpleredis.go:88-164` (all verbs' `[]byte(...)` conversions), `:167-179` (`parseIntegerReply`)
- **Status**: not applied, and recommended against

## What I found

go-redis avoids copying between `string` and `[]byte` with `internal/util`:

```go
func BytesToString(b []byte) string { return unsafe.String(unsafe.SliceData(b), len(b)) }
func StringToBytes(s string) []byte { return unsafe.Slice(unsafe.StringData(s), len(s)) }
```

Every verb here pays those copies today — `[]byte(name)` in `Get`, `[]byte(script)`
in `Eval`, and `string(values[0])` in `parseIntegerReply` — so the technique looks
directly applicable. I tested it rather than assuming.

**Yaegi does not have the modern primitives at all.** `unsafe.Slice`,
`unsafe.String`, `unsafe.StringData` and `unsafe.SliceData` are absent from Yaegi
0.16.1 in every configuration; `stdlib/unsafe` exports only `Pointer`, `Add`,
`Sizeof`, `Alignof` and `Offsetof`. Interpreted code using them fails at *import*
time:

```
package unsafe "unsafe" has no symbol Slice
```

The pre-generics tricks do work, but only when unsafe symbols are registered
(`TestYaegiUnsafeVariants`):

| Conversion | stdlib only | stdlib+unsafe | +unrestricted |
|---|---|---|---|
| `unsafe.Slice` / `unsafe.String` (go-redis v9) | no | **no** | no |
| `*(*string)(unsafe.Pointer(&b))` | no | yes | yes |
| `*(*[]byte)(unsafe.Pointer(&struct{ string; Cap int }{s, len(s)}))` | no | yes | yes |
| `reflect.StringHeader` | no | yes | yes |

**Compiled, the gain is real but small.** Encoding one `EVAL` with the ~470-byte
token-bucket script:

| | ns/op | B/op | allocs/op |
|---|---|---|---|
| `[]byte(script)` copy | 225 | 544 | 10 |
| zero-copy args | 116 | 32 | 6 |

Integer parsing gains essentially nothing, because the Go compiler already keeps
short `string(b)` conversions on the stack: `strconv.ParseInt(string(b))` measures
9.6 ns and **0 allocations**, versus 8.0 ns for the unsafe form.

**Interpreted, it is a net loss.** Converting the same script inside Yaegi:

| Interpreted conversion | ns/op |
|---|---|
| `[]byte(s)` inline | **218** |
| `[]byte(s)` behind a helper function | 383 |
| unsafe struct-header trick behind a helper | **502** |

Like-for-like (both behind a helper call), the unsafe version is **120 ns slower per
conversion**; against the best available shape (inline conversion) it is 284 ns
slower. The reason shows up in the middle row: an interpreted function call costs
~165 ns, and the unsafe trick needs a helper plus a composite literal plus two
pointer reinterpretations, all executed as interpreted nodes, while `[]byte(s)` is a
single node that hands off to a native conversion.

## Why it matters

The library ships as a Yaegi-interpreted Traefik plugin. Optimising the compiled
path while pessimising the interpreted one optimises the mode nobody runs in
production, and it does so at a real deployment cost:

- Traefik registers unsafe symbols only when **both** the plugin manifest sets
  `useUnsafe` **and** the operator enables `useUnsafe` in static settings
  (`pkg/plugins/middlewareyaegi.go`, `newInterpreter`). If the manifest asks and the
  operator has not opted in, Traefik **refuses to load the plugin** with
  "this plugin uses restricted imports".
- That cost propagates to every downstream consumer of this library, forever, for
  at most ~110 ns and 512 B per `Eval` compiled — and a loss interpreted.
- There is also an unquantified soundness risk. The struct-header trick relies on
  taking a pointer into a value whose lifetime the *interpreter* manages via
  reflect. My probe returned correct data, but a probe is not proof of GC safety
  under pressure; a dangling pointer here would corrupt request data
  non-deterministically. Proving it sound would need dedicated GC-stress testing.

The one place the copy genuinely costs something — shipping a ~470-byte Lua script
on every request — is better solved by [perf-05](perf-05-evalsha.md). `EVALSHA`
removes the copy *and* the 470 bytes on the wire, needs no `unsafe`, no manifest
change and no operator opt-in, and is a bigger win than zero-copy could ever be.

## Expected gain

**Negative in the deployment mode that matters.** Compiled: ~110 ns and 512 B per
`Eval`, ~1.6 ns and 0 B per integer parse. Interpreted: **+120 ns per conversion**,
several conversions per command.

For comparison, [perf-06](perf-06-single-write-encoding.md) saves 1,380 ns per
command interpreted with no flag and no soundness question — 12x the best compiled
case here, in the right direction.

## If you adopt it anyway

You said requiring `useUnsafe` is acceptable, so if it is taken despite the above:

- Use only the legacy forms; the go-redis v9 form cannot work under Yaegi.
- **Inline the conversion at each use site** rather than calling a helper, since the
  measured ~165 ns interpreted call overhead is most of what makes it lose. This
  conflicts with keeping the code readable, which is itself an argument against.
- Apply it only to large arguments (the `Eval` script, large `Set` values). Skip it
  for keys and integers, where the copy is already free or nearly so.
- Declare `useUnsafe: true` in the plugin manifest, document the operator-side
  static-config requirement prominently, and keep a non-unsafe build path so the
  library still loads where unsafe is refused.
- Add GC-stress coverage that holds converted slices across allocations and
  collections, interpreted, before trusting it with request data.

## How to prove it

`TestYaegiUnsafeVariants` records which conversions the interpreter accepts and
should stay as a guard against a future Yaegi bump changing the answer.
`BenchmarkCompiledEncodeEval{Copy,Unsafe}`, `BenchmarkCompiledParseInt{Copy,Unsafe}`
and `BenchmarkYaegiConvert{Copy,CopyCall,Unsafe}` in
`simpleredis/interpretedcost_test.go` reproduce every number above.
