## 1. Decode

- [ ] 1.1 After a complete `io.ReadFull` of `length+2` in `readBulk`, if the last two bytes are not CR then LF, return `errIssue`. Keep `make([]byte, length+2)` plus `io.ReadFull`. Do not cap length (bug-02)

## 2. Tests

- [ ] 2.1 Same-package `readBulk` units: trailer `\n\r` and two payload bytes as trailer return `redis:issue?`; `$0` with CRLF trailer succeeds
- [ ] 2.2 When `closeAfter` is false, `serveRawReply` waits for the client to disconnect before Close. Leave `closeAfter: true` as write-then-return. Do not export Accept count
- [ ] 2.3 Desync: `PoolSize: 1`, `MaxRetries: -1`, first Accept `$5\r\nhello+OK\r\n` (`closeAfter: false`), second Accept a complete bulk. First Get is `redis:issue?` and idle empty; second Get returns the second payload, not remnants of the first
- [ ] 2.4 Run `go test -short ./simpleredis/...`

## 3. Docs

- [ ] 3.1 Add a trailer-CRLF sentence to `knowledge/devdocs/std_go_simpleredis_resp-decode.md` How to use / Gotchas. Do not invent Language
- [ ] 3.2 Confirm the change deltas `std_go_simpleredis_resp-decode` and `std_go_simpleredis_resp-commands` match the landed tests. Run `openspec validate --change simpleredis-bulk-trailer --strict`
