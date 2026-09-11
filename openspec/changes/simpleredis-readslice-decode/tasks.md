## 1. Decode

- [x] 1.1 Switch `readLine` to `ReadSlice('\n')`; on `bufio.ErrBufferFull` copy the partial, one `ReadBytes('\n')`, append; keep dest CRLF strip (`redis:issue?`); do not loop ReadSlice; stay on `bufio.NewReader` (4096)
- [x] 1.2 Copy `line[1:]` / `head[1:]` with `append([]byte(nil), payload...)` wherever `+` or `:` escapes to the caller or `values[i]`; do not copy `$`/`*` headers after length parse; leave `readBulk` `make([]byte, length+2)`
- [x] 1.3 Add unexported `parseLen([]byte) (int, bool)` (optional leading minus; empty/non-digits → false) and use it at the array-count and bulk-length sites; no `unsafe`, no go-redis import

## 2. Unit tests and benches

- [x] 2.1 Add a sequential canned TCP (not `startStaticRedis`, not fakeRedis default EVAL `:0`) that returns distinct `+` or `:` payloads; hold the first slice; assert it is unchanged after a second command on the same connection
- [x] 2.2 Add a compiled test for a status or error line longer than 4096 bytes (`ErrBufferFull`); scripted TCP only
- [x] 2.3 Add `simpleredis/bench_test.go` with `BenchmarkDecodeBulk`, `BenchmarkDecodeArray10`, and `BenchmarkDecodeInteger` against compiled fake RESP (not live engines)
- [x] 2.4 Run `go test ./simpleredis/...` until copy-on-escape, `ErrBufferFull`, existing Get/MGet/Incr/Eval, and Yaegi tests pass

## 3. Live Redis and Dragonfly

- [x] 3.1 Do not edit `e2e/simpleredisprobe`, tokenbucket, or windowcounter Lua; scripts stay Lua 5.1-safe with keys in KEYS; do not add compose services
- [ ] 3.2 Run `./Test-Integration.ps1` until Pester `/redis` and `/dragonfly` assert Get, MGet, Incr, and Eval headers (`e2e/simpleredisprobe`) on both Redis and Dragonfly

## 4. Specs

- [x] 4.1 Confirm deltas `std_go_simpleredis_resp-decode` and `std_go_simpleredis_resp-commands` match the landed decode
- [x] 4.2 Run `openspec validate --change simpleredis-readslice-decode --strict`
