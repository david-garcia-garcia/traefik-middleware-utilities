## Why

`readLine` uses `bufio.Reader.ReadBytes`, so every RESP line allocates a copy. Most copies are discarded after the type byte or length. Traefik traffic with many `MGet`/`Eval` array replies pays that GC for nothing. `ReadSlice` avoids the copy, but its buffer view is invalid after the next read, so escaping `+`/`:` payloads must be copied and lines longer than 4096 bytes must handle `bufio.ErrBufferFull`.

## What Changes

- Switch `readLine` to `ReadSlice('\n')`. On `bufio.ErrBufferFull`, copy the partial, then one `ReadBytes('\n')` and append (go-redis `internal/proto.Reader.readLine` at `7f3b3dff`). Keep the existing CRLF strip. Stay on `bufio.NewReader` (4096).
- Copy `line[1:]` / `head[1:]` wherever a `+` or `:` payload escapes to the caller or into `values[i]`, before the next read and before `release`. Header-only uses (type byte, `parseLen`) MUST NOT keep the slice across a later read. `-` via `replyError` already copies with `string(message)`.
- Add unexported `parseLen([]byte) (int, bool)` at the array-count and bulk-length sites. Accept optional leading minus (`$-1` miss). No `unsafe`, no go-redis `util.Atoi`.
- Leave `readBulk`'s `make([]byte, length+2)` as-is. Do not import `go-redis` or miniredis. Do not change Init/Close/pool, AUTH/SELECT, or exported error strings.
- Unit-test copy-on-escape with distinct `+`/`:` payloads on one connection. Unit-test a status or error line longer than 4096 bytes (`ErrBufferFull`).
- Add `BenchmarkDecodeBulk`, `BenchmarkDecodeArray10`, and `BenchmarkDecodeInteger` (dest has no `bench_test.go`). Compiled fake RESP, not live engines.
- After apply, live Get/MGet/Incr/Eval replies stay correct on both Redis and Dragonfly via compose + Pester `/redis` `/dragonfly` and `e2e/simpleredisprobe`. Eval scripts stay Lua 5.1-safe with keys in KEYS. Do not edit probe / tokenbucket / windowcounter scripts. Do not add compose services.

## Capabilities

### New Capabilities

- `std_go_simpleredis_resp-decode`: RESP line decode via `ReadSlice`, `ErrBufferFull` fallback, copy-on-escape for `+`/`:`, `parseLen`, copy-on-escape and long-line unit tests, decode allocation benches, and live Get/MGet/Incr/Eval remaining correct on Redis and Dragonfly.

### Modified Capabilities

- `std_go_simpleredis_resp-commands`: array `+`/`:` slots MUST be independent copies; Traefik e2e Get/MGet/Incr/Eval on `/redis` and `/dragonfly` MUST still pass; Eval stays Lua 5.1-safe with keys in KEYS.

## Impact

- `simpleredis/simpleredis.go` (`readLine`, `readReply`, `readBulk`, new `parseLen`).
- `simpleredis/simpleredis_test.go` (copy-on-escape + `ErrBufferFull`).
- `simpleredis/bench_test.go` (new decode benches).
- Existing e2e already covers live verbs: `e2e/simpleredisprobe/plugin.go`, `scripts/integration-tests.Tests.ps1`, `docker-compose.yml` (must still pass; no new engines).
- Main specs after archive: `openspec/specs/std_go_simpleredis_resp-decode/spec.md` (new) and `openspec/specs/std_go_simpleredis_resp-commands/spec.md`.
- Session spec `std_go_simpleredis_tcp-session` unchanged (stdlib, no `unsafe`).
