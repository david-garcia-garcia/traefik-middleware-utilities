## Why

Dest `readBulk` consumes `length+2` bytes and returns the payload without checking that the last two bytes are CRLF. A mis-framed bulk still parses as a clean value, so `do` pools a socket that is no longer on a reply boundary and the next command can read leftover bytes from the previous reply.

## What Changes

- After a complete `io.ReadFull` of `length+2`, if the last two bytes are not CRLF, `readBulk` returns `redis:issue?` so `readReply` is `clean == false` and `do` does not pool.
- Same-package unit tests: trailer `\n\r`, two payload bytes used as trailer, and legitimate `$0\r\n\r\n` still succeeding.
- Desync: `PoolSize: 1`, `MaxRetries: -1`, first Accept a trailer-less bulk with leftover bytes, second Accept a normal bulk. First Get is `redis:issue?` and idle empty; second Get returns the second payload, not remnants of the first. Reuse `startRawReplyRedis`. When `closeAfter` is false, keep the socket until the client disconnects.
- Do not export the helper’s Accept counter. Do not apply the bug-02 length cap. Do not add a second scripted-server helper. Do not change `readLine`, RESP3, nested arrays, or `*-1`.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_resp-decode`: after a complete `io.ReadFull` of `length+2`, `readBulk` SHALL verify the last two bytes are CRLF; otherwise `redis:issue?`. Keep `make([]byte, length+2)` plus `io.ReadFull`.
- `std_go_simpleredis_resp-commands`: a complete `$` payload whose trailer is not CRLF is malformed RESP (`redis:issue?`, not pooled), distinct from a short `ReadFull` (`redis:unreachable`).

## Impact

- `simpleredis/resp.go` (`readBulk` trailer check)
- `simpleredis/resp_test.go` (unit + desync)
- `simpleredis/fake_redis_test.go` (`closeAfter: false` keep-alive only)
- Usage packet `knowledge/devdocs/std_go_simpleredis_resp-decode.md` (trailer sentence)
- Main specs `std_go_simpleredis_resp-decode` and `std_go_simpleredis_resp-commands` after archive
