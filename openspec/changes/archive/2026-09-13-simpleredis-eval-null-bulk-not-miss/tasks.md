## 1. Reproducing tests (MUST fail on dest)

- [x] 1.1 In `simpleredis/commands_eval_test.go`, add a compiled test that Eval against `startStaticRedis` canned `$-1\r\n` with `Eval(ctx, script, ScriptSHA1Hex(script), nil, nil)` returns one nil slot and `errors.Is(err, ErrMiss)` is false. No `//go:build bugrepro`. Do not change the public Eval signature
- [x] 1.2 Run `go test -short ./simpleredis/` and confirm that new test MUST fail on current dest

## 2. Decode + Get mapping

- [x] 2.1 In `simpleredis/resp.go` `readReply` top-level `$`, when `readBulk` returns `errMiss`, return `[][]byte{nil}, true, nil` (same as an array null slot). `readBulk` may still use `errMiss` internally for `length < 0`
- [x] 2.2 In `simpleredis/commands.go` `Get`, after a 1-slot reply, `values[0]==nil` → `errMiss`. Empty bulk `$0` stays a non-nil empty slice. Do not remap `ErrMiss` only inside `Eval`. Do not change MGET nil slots

## 3. Confirm

- [x] 3.1 Confirm the Eval `$-1` test now passes (`[][]byte{nil}, nil`; not `ErrMiss`)
- [x] 3.2 Keep `TestGetHitAndMiss` passing. Add a `startStaticRedis` Get `$-1` still `ErrMiss` if the fake-store miss is not enough after decode
- [x] 3.3 Confirm `TestMGetHitsMissesAndEmpty` still returns aligned nil slots
- [x] 3.4 Add Get `$0` not-miss via `startStaticRedis` in `simpleredis/commands_test.go` if no verb-level test exists (`readBulk` empty-ok is not enough)
- [x] 3.5 Run `go test -short ./simpleredis/` and confirm the suite passes

## 4. Usage

- [x] 4.1 Update `knowledge/devdocs/std_go_simpleredis.md` gotcha so null bulk `$-1` is `redis:miss` for Get only, and `knowledge/devdocs/std_go_simpleredis_resp-decode.md` so `parseLen` optional minus is a nil slot, not miss for every verb
