# Explore
IssueKey: 2026-09-11-simpleredis-perf-07-readslice

## Concepts

```
dial → bufio.NewReader (4096) → writeCommand → readReply
                                              │
                                              ▼
                                         readLine
                                         ReadSlice('\n')
                                              │
                    ┌─────────────────────────┼─────────────────────────┐
                    │ ok, line in buffer      │ ErrBufferFull           │ other err
                    ▼                         ▼                         ▼
              view into buf              copy partial +                 return
              (invalid next read)        one ReadBytes('\n')
                    │                         │
                    └────────────┬────────────┘
                                 ▼
                          strip CRLF
                                 │
              + / :  copy payload before return / values[i]
              -      string(message) already copies (replyError)
              $      parseLen(head[1:]); readBulk make+ReadFull (unchanged)
              *      parseLen; loop elements; copy + / : slots
```

- **SimpleRedis** — stdlib pooled TCP RESP client. Usage: `knowledge/devdocs/std_go_simpleredis.md`. Session spec: `openspec/specs/std_go_simpleredis_tcp-session/spec.md`. Command spec: `openspec/specs/std_go_simpleredis_resp-commands/spec.md`.
- **readLine** — `simpleredis/simpleredis.go:408-417`. Today `ReadBytes('\n')` then strip CRLF. Dest has no `ReadSlice` and no `ErrBufferFull` path.
- **readReply** — `:328-386`. `+` / `:` return `line[1:]` with no copy (`:338-340`). Array `:` / `+` store `head[1:]` into `values[i]` (`:376-377`). `$` / `*` lengths via `strconv.Atoi(string(...))` (`:353`, `:393`).
- **readBulk** — `:388-405`. `make([]byte, length+2)` + `io.ReadFull`; leave as-is (Out of scope: shrink to `length` plus CRLF discard).
- **parseLen** — new unexported helper this change creates. Replaces the two `Atoi` sites. Must accept optional leading minus (`$-1` miss). Must not use `unsafe` (tcp-session spec; go-redis `BytesToString` is `unsafe.String` — `knowledge/research/ext_go-redis_proto_readline/`).
- **Idle conn** — `dial` uses `bufio.NewReader(netConn)` (`:271`). After `do`, `release` may put the same reader back on the pool. An uncopied ReadSlice view is then overwritten by the next command, including another goroutine.
- **Live proof** — compose `redis:7-alpine` @ `redis:6379` and `dragonfly:v1.40.2` @ `dragonfly:6379`; probe `e2e/simpleredisprobe/plugin.go` runs Get/MGet/Incr/Eval (and Set/Del/Expire/ExpireAt) per request; Pester `scripts/integration-tests.Tests.ps1` asserts headers on `/redis` and `/dragonfly`.
- **Eval scripts already in-tree** — probe / `simpleredis_test.go` / `windowcounter/limiter.go` `flushScript` list `KEYS[1]`; `tokenbucket/lua.go` `allowScript` uses `KEYS[1]` and `#rl_source` (no `table.maxn`). This change does not edit those scripts.

`readLine(` call sites: **2**, both in `simpleredis/simpleredis.go` (`readReply` `:330` and the array loop `:359`). Roots searched: `simpleredis/`, `tokenbucket/`, `windowcounter/`, `e2e/`, `reclaim/` for `readLine(`. `readReply(` callers: **1** (`do` `:299`).

## Decisions

- Switch `readLine` to `ReadSlice('\n')`. On `bufio.ErrBufferFull`, copy the partial into a fresh buffer, then **one** `ReadBytes('\n')` and append — go-redis `internal/proto.Reader.readLine` at `7f3b3dff`, not a ReadSlice loop. Then keep the existing CRLF strip (`len < 2` or missing `\r` → `redis:issue?`). Do not adopt go-redis `DefaultBufferSize` 32 KiB; SimpleRedis stays on `bufio.NewReader` (4096). Evidence: `knowledge/research/ext_go-redis_proto_readline/`.
- Copy `line[1:]` / `head[1:]` wherever `+` or `:` escapes to the caller or into `values[i]`, **before** the next read and **before** `release`. Use `append([]byte(nil), payload...)` or `make`+`copy` (no `bytes` import required; no `unsafe`). Do not copy `$` / `*` headers after `parseLen` — those slices are discarded before the next read.
- Add `parseLen([]byte) (int, bool)` and use it at the array-count and bulk-length sites. Accept optional leading minus so `$-1` remains `redis:miss`. Reject empty and non-digits as `redis:issue?`. Do not copy go-redis `util.Atoi` (`unsafe`).
- Leave `readBulk`'s `make([]byte, length+2)` unchanged. Do not import `go-redis` or miniredis. Do not change Init/Close/pool, AUTH/SELECT, or exported error strings.
- Unit-test copy-on-escape with **distinct** `+` or `:` payloads on one connection: hold the first returned slice, issue a second command, assert the first slice is unchanged. Do not use `fakeRedis` default EVAL `:0\r\n` (identical payloads hide aliasing). `startFakeRedis` in `simpleredis/simpleredis_test.go` has no copy-after-next-read test and no line longer than 4096.
- Unit-test a status or error line longer than **4096** bytes (`ErrBufferFull` at `bufio.NewReader` size, not go-redis 32 KiB). Scripted TCP is enough; live Pester stays the existing short-reply verbs.
- After apply, run `go test ./simpleredis/...` and compose + Pester `/redis` and `/dragonfly` (`e2e/simpleredisprobe`). Tests MUST run against both Redis and Dragonfly. Do not add compose services.
- Eval scripts stay Lua 5.1-safe with keys in KEYS. Do not edit probe / tokenbucket / windowcounter scripts in this change.
- dest has no `simpleredis/bench_test.go`. Add `BenchmarkDecodeBulk`, `BenchmarkDecodeArray10`, and `BenchmarkDecodeInteger` in this change so allocation guards are not blocked on test-07. Assert post-change allocs/op (finding: bulk 3→~1, array10 22→~11, integer 2→ fewer). Compiled fake RESP, not live engines.
- Do not reconstruct client address, user, tenant, Host, or trust hop — this decode change does not set identity.
- Usage packet `knowledge/devdocs/std_go_simpleredis.md` is enough to **call** Get/MGet/Eval; ReadSlice lifetime is decode internals. No Language/usage write this phase. `opd-devdocsimpact` after apply can add a gotcha if implementers of `readReply` need it in the packet.

## Open questions

- Q: What is the exact go-redis `readLine` fallback for `bufio.ErrBufferFull`?
  Rank: additive asked — new ErrBufferFull branch inside `readLine` this change creates; Desired: "On `bufio.ErrBufferFull`, accumulate into a fresh buffer and keep reading until a full line (go-redis `internal/proto.Reader.readLine` pattern)"
  Decision: resolved — `ReadSlice('\n')`; on `ErrBufferFull` copy the partial, then one `ReadBytes('\n')`, append, strip CRLF. Not a ReadSlice loop. Test long lines at 4096 (SimpleRedis `NewReader`), not 32 KiB. Source: `knowledge/research/ext_go-redis_proto_readline/`.
  By: explore

- Q: Will `simpleredis/bench_test.go` exist before this change, or should this change add `BenchmarkDecode*`?
  Rank: additive incidental — new bench file this change would create; no Desired/HARD-REQUIREMENT line names the benches (they name copy-on-escape, ErrBufferFull unit tests, and live Get/MGet/Incr/Eval on both engines)
  Decision: resolved — this change added `BenchmarkDecodeBulk`, `BenchmarkDecodeArray10`, and `BenchmarkDecodeInteger` in `simpleredis/bench_test.go`. Dest had none. Measured post-ReadSlice: bulk 2 allocs/op, array10 11, integer 2.
  By: implement

- Q: Do `-` error payloads that escape via `replyError` → `string(message)` also need an explicit copy before the next read?
  Rank: bounded incidental — 1 existing `replyError(` call site in `simpleredis/simpleredis.go` (`readReply` `case '-'`); searched `simpleredis/` for `replyError`; Desired copy line names `+` / `:`, not `-`
  Decision: resolved — `replyError` does `text := string(message)` then prefix match or `errors.New(text)`. `string([]byte)` copies, so the next `ReadSlice` cannot alias the error text. No extra copy.
  By: explore
