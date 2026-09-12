## 1. Sentinel and decode

- [ ] 1.1 Export `RedisUnsupportedReply` / `errUnsupportedReply` in `simpleredis/simpleredis.go`
- [ ] 1.2 In `readReply`, return that sentinel `clean false` for unknown type bytes (including HTTP-shaped and RESP3) and for array elements that are not `$`, `:`, or `+` (nested `*`, `-`)
- [ ] 1.3 Pass `errUnsupportedReply` through `do` like `errIssue` so it does not become `redis:unreachable`
- [ ] 1.4 Keep `*-1`, empty line, missing CR, and unparseable length as `redis:issue?` `clean false`

## 2. Eval contract

- [ ] 2.1 Document on `Eval` that replies are flat arrays of bulk strings or integers and that Lua authors wrap slots with `tostring`

## 3. Tests

- [ ] 3.1 Table over `readReply` for ticket rows: specific error + `clean` (unsupported vs issue vs miss)
- [ ] 3.2 Split `TestMalformedReplyIsIssueAndNotPooled` so unknown type, nested array, and bad element expect `redis:unsupported-reply` and idle 0
- [ ] 3.3 Eval nested-array fake: Error() is `redis:unsupported-reply` not `redis:unreachable`; next command on the same client raises accept count (redial)
- [ ] 3.4 Guard: three bulk strings (token-bucket `tostring` shape) parse through Eval
- [ ] 3.5 Run `go test -short ./simpleredis/...`

## 4. Usage

- [ ] 4.1 Update `knowledge/devdocs/std_go_simpleredis.md`: sixth error string; Eval flat-array / `tostring` gotcha
