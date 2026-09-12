## 1. Caps

- [ ] 1.1 Add package consts `maxBulkLength = 64 << 20` and `maxArrayCount = 1 << 20` in `simpleredis/resp.go`
- [ ] 1.2 In `readBulk`, after the miss check, return `errIssue` when `length > maxBulkLength` before `make`
- [ ] 1.3 In `readReply` `*`, after the negative/unparseable check, return `errIssue`, `clean == false` when `count > maxArrayCount` before `make([][]byte, count)`

## 2. Tests

- [ ] 2.1 Same-package `readReply` tests with `bufio.NewReader(strings.NewReader(...))`: `$` and `*` just over each cap → `errIssue`, `clean == false`
- [ ] 2.2 MaxInt64 header digits via `strconv.FormatInt(math.MaxInt64, 10)` for `$` and `*`; lengths that currently panic; all `errIssue`, `clean == false`
- [ ] 2.3 `$268435456\r\n` returns `errIssue` and MUST fail if `make` of that payload still ran
- [ ] 2.4 Array `$` element over `maxBulkLength` (`*1` plus over-cap `$`) → `errIssue`, `clean == false`. Keep `TestParseLen` overflow-digit case
- [ ] 2.5 Run `go test ./simpleredis/...`

## 3. Specs

- [ ] 3.1 Confirm deltas `std_go_simpleredis_resp-decode` and `std_go_simpleredis_resp-commands` match the landed code
- [ ] 3.2 Run `openspec validate --change simpleredis-reply-alloc-caps --strict`
