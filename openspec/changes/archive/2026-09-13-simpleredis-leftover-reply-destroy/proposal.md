## Why

A pooled SimpleRedis socket that still has unread RESP after one well-formed reply is returned to idle and then serves the previous command's reply as the next command's value, with `err == nil`. That is a silent wrong-key Get until the process restarts.

## What Changes

- In `do`, a socket is reusable only when `conn.reader.Buffered() == 0` after a successful `readReply`. Leftover bytes: return the decoded value (this command's reply was well-formed) and `reusable = false` so `release` destroys the socket. Do not drain or resynchronise.
- If `Buffered() != 0` before the next write on that same connection (AUTH leftover then SELECT), refuse the write inside `do` so SELECT is not parsed from leftover. Do not edit `pool.go` or `commands_exec.go`.
- Permanent untagged tests in the default `go test ./simpleredis/` suite: own-key or error after a stray extra reply; idle empty then a new dial; 25 sequential Gets still open exactly one TCP connection.
- Do not create or edit `simpleredis/BUGS.md`.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: a socket the decoder cannot prove is on a reply boundary (`Buffered() != 0` after one complete value) MUST NOT return to the idle pool. A compliant peer that leaves the buffer empty MUST still reuse one connection.
- `std_go_simpleredis_resp-commands`: Get MUST return the value for its own key or an error, never another key's bytes, after a peer writes an extra well-formed reply.

## Impact

- `simpleredis/resp.go` (`do` only).
- `simpleredis/fake_redis_test.go` (stray-extra fake). `simpleredis/resp_test.go` (regression, discard, no-churn proofs). Optional Yaegi Get against that fake via existing GOPATH helpers.
- Main specs `std_go_simpleredis_tcp-session` and `std_go_simpleredis_resp-commands` after archive.
- Usage `knowledge/devdocs/std_go_simpleredis.md` / `std_go_simpleredis_resp-decode.md` if they still imply a clean parse is always reusable (devdocs-impact).
