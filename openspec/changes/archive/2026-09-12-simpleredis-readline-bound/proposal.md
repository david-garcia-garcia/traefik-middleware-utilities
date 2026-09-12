## Why

`readLine` already uses `ReadSlice` for short headers, but on `bufio.ErrBufferFull` it still `ReadBytes` the remainder with no byte ceiling. A peer that never sends a newline, or that sends a huge terminated line, allocates until `IOTimeout` or the Traefik process OOMs. Time is not a substitute for a per-socket allocation bound.

## What Changes

- On `ReadSlice` `ErrBufferFull`, return `redis:issue?` and do not `ReadBytes` the remainder. Keep dest CRLF strip for complete lines. Stay on `bufio.NewReader` (4096). Do not add a post-grow `maxLineLength`.
- Invert `TestLongStatusLineDecodes` so a line longer than the buffer is `redis:issue?` and not pooled. Add an over-cap terminated case and an unterminated stream that never sends `\n`; pin the bound by counting bytes the fake peer delivered (≤4096), not `runtime.ReadMemStats`. Keep shortest legal lines (`+OK`, `:1`, `$-1`) and the existing missing-CR reject.
- Change `std_go_simpleredis_resp-decode` so a full buffer is `redis:issue?` and the caller does not receive `bufio.ErrBufferFull`. Copy-on-escape for short `+`/`:` stays.
- Update `knowledge/devdocs/std_go_simpleredis_resp-decode.md` so the recipe matches reject-on-full. Do not import go-redis. Do not enlarge the reader. Do not retouch `readBulk`.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `std_go_simpleredis_resp-decode`: `ErrBufferFull` is `redis:issue?` with no remainder `ReadBytes`; compiled tests fail if an over-cap or unterminated line is decoded or if the fake peer delivered more than 4096 bytes.

## Impact

- `simpleredis/resp.go` (`readLine` `ErrBufferFull` path).
- `simpleredis/resp_test.go` (invert long-line success; add over-cap and unterminated proofs).
- `openspec/specs/std_go_simpleredis_resp-decode/spec.md` (after archive).
- `knowledge/devdocs/std_go_simpleredis_resp-decode.md` (grow recipe → reject-on-full).
