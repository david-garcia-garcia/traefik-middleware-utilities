## Context

Dest `readLine` is `ReadBytes('\n')` then strip CRLF (`simpleredis/simpleredis.go`). `+`/`:` return `line[1:]` with no copy; array `:`/`+` store `head[1:]` into `values[i]`. Lengths use `strconv.Atoi(string(...))`. `dial` uses `bufio.NewReader(netConn)` (4096). After `do`, `release` may pool the same reader. See proposal.md for why. Research: `knowledge/research/ext_go-redis_proto_readline/`. Proceed policies: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- `ReadSlice` decode with go-redis `readLine` fallback (copy partial + one `ReadBytes`).
- Copy escaping `+`/`:` before the next read / `release`.
- Byte `parseLen` at the two length sites. No `unsafe`.
- Compiled copy-on-escape and `ErrBufferFull` (>4096) tests; decode benches; live Get/MGet/Incr/Eval on Redis and Dragonfly.

**Non-Goals:**
- Shrinking `readBulk` to `make([]byte, length)` plus a two-byte CRLF discard.
- perf-06 encode, perf-08 `unsafe` zero-copy, pool cap, pipelining, EVALSHA, MSETEX.
- Adding compose services or editing probe / tokenbucket / windowcounter Lua.
- Raising the bufio buffer to go-redis 32 KiB.

## Decisions

1. **Fallback is copy-partial + one `ReadBytes`, not a ReadSlice loop.** Match go-redis `internal/proto.Reader.readLine` at `7f3b3dff`. Then keep dest CRLF strip (`redis:issue?`). Alternative: loop `ReadSlice` until `err == nil` — rejected; the named pattern is one `ReadBytes` for the remainder.

2. **Stay on `bufio.NewReader` (4096).** Do not adopt `proto.DefaultBufferSize` 32 KiB. Unit-test long lines at 4096. Alternative: 32 KiB buffer — rejected; SimpleRedis dial stays dest.

3. **Copy with `append([]byte(nil), payload...)` (or `make`+`copy`).** No `bytes` import. Copy at both `readReply` `+`/`:` return and array `values[i]`. Do not copy `$`/`*` headers after `parseLen`. Alternative: `string` then back to bytes — extra alloc; `string` is enough for `-` via `replyError` only.

4. **`parseLen([]byte) (int, bool)`.** Optional leading minus. Empty / non-digits → false → `redis:issue?`. Replace both `Atoi` sites. Alternative: go-redis `util.Atoi` — rejected (`unsafe.String`); tcp-session forbids `unsafe`.

5. **Copy-on-escape tests use a sequential canned TCP, not `startStaticRedis` and not default `fakeRedis` EVAL `:0`.** `startStaticRedis` repeats one blob; identical `:0` hides aliasing. Script two distinct `+` or `:` replies on one connection; hold the first slice; assert after the second command. Long-line test: status or `-` longer than 4096, still scripted TCP. Alternative: live Pester for long lines — rejected; Pester stays short-reply verbs.

6. **Add `bench_test.go` in this change.** Dest has no file (test-07 not applied). Assert post-change allocs/op toward finding numbers (bulk 3→~1, array10 22→~11, integer 2→ fewer). Alternative: wait on test-07 — rejected; allocation guards would stall.

7. **Live proof is existing compose + Pester.** Run `go test ./simpleredis/...` then `./Test-Integration.ps1` until `/redis` and `/dragonfly` Get/MGet/Incr/Eval headers pass. Do not edit `kongIncrbyExpireatScript` (`KEYS[1]`, no `table.maxn`). Alternative: new engines or a second probe — out of scope.

## Risks / Trade-offs

- [ReadSlice view pooled and overwritten by another goroutine] → Mitigation: copy `+`/`:` before return and before `release`.
- [Long-line `ReadBytes` still allocates] → Mitigation: rare; hot path is short headers. Do not grow the 4096 buffer.
- [Dragonfly Lua 5.4 vs Redis 5.1] → Mitigation: do not touch existing KEYS scripts; Pester both engines.
- [Benches without a hard allocs/op fail] → Mitigation: record numbers in the test comment; fail only if allocs/op regresses above dest baseline after the switch.

## Migration Plan

Library-internal decode. Rollback is revert. No production deploy. No exported API change.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
