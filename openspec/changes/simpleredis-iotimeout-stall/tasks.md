## 1. Product

- [x] 1.1 Add `stallConn` in `simpleredis/resp.go` with every `net.Conn` method declared (Yaegi does not promote embeds); `Read`/`Write` call `clampTimeout(ctx, stall)` then `SetReadDeadline`/`SetWriteDeadline`; `bound <= 0` returns `os.ErrDeadlineExceeded`
- [x] 1.2 Wrap the TCP conn as `stallConn` in `dial` before `bufio.NewReader`/`NewWriter`
- [x] 1.3 In `do`, assign `ctx` and `stall = IOTimeout` on `*stallConn`; keep start-of-command `ioBound` for `ioOrContext`; do not one-shot `SetDeadline` on the wrapped conn
- [x] 1.4 Update `Config.IOTimeout` comment from per-command `SetDeadline` to stall (quiet-peer) bound

## 2. Tests

- [x] 2.1 Add `simpleredis/bug3_stall_deadline_test.go` with `bug3`-prefixed fakes: streaming bulk beyond `IOTimeout` returns intact; silent mid-reply times out promptly; drip returns within overall budget, `OverFrees()==0`, later Get obtains a turn
- [x] 2.2 `go vet ./simpleredis/`
- [x] 2.3 `go test ./simpleredis/ -count=1`
- [x] 2.4 `go test ./... -count=1 -short`
- [x] 2.5 `go test -tags simpleredis_bugs ./simpleredis/ -run TestBugValueLargerThanIOTimeout -v` from the caller checkout that has the tagged file (expect pass after the fix)

## 3. Spec and usage

- [x] 3.1 Update `knowledge/devdocs/std_go_simpleredis.md` so remaining-time wording says stall refresh, not one-shot per-command `SetDeadline`
- [x] 3.2 `openspec validate --change simpleredis-iotimeout-stall --strict`
