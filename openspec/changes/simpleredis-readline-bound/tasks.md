## 1. Decode

- [ ] 1.1 On `readLine` `ReadSlice` `ErrBufferFull`, return `errIssue` and do not copy-partial or `ReadBytes` the remainder; keep dest CRLF strip; stay on `bufio.NewReader` (4096)

## 2. Unit tests

- [ ] 2.1 Invert `TestLongStatusLineDecodes` so a 5000-byte `+` status is `redis:issue?` and idle 0
- [ ] 2.2 Add a compiled `readLine` test with a counting reader: unterminated stream that fills 4096 → `errIssue` and bytes read ≤ 4096; over-cap terminated line → `errIssue` and bytes read ≤ 4096; do not use `runtime.ReadMemStats`
- [ ] 2.3 Keep shortest-line (`+OK`, `:1`, `$-1`) and missing-CR cases green
- [ ] 2.4 Run `go test ./simpleredis/...` until the bound tests, copy-on-escape, malformed-reply, and Yaegi tests pass

## 3. Usage doc

- [ ] 3.1 Update `knowledge/devdocs/std_go_simpleredis_resp-decode.md` so `ErrBufferFull` is `redis:issue?` with no remainder `ReadBytes`; leave copy-on-escape and `parseLen` usage

## 4. Specs

- [ ] 4.1 Confirm delta `std_go_simpleredis_resp-decode` matches the landed reject-on-full behavior
- [ ] 4.2 Run `openspec validate --change simpleredis-readline-bound --strict`
