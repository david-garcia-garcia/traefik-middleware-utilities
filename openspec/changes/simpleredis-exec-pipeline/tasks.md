## 1. Client pipeline

- [x] 1.1 Split `writeCommand` so it encodes only; `do` Flushes after one frame; AUTH/SELECT in `dial` still go through `do`
- [x] 1.2 Add exported `PipelineSlot`, `maxPipelineCommands = 64`, `ExecPipeline`, and unexported `execPipeline` sibling of `exec`; empty/nil returns nil, nil without dial; over cap is `redis:issue?` without send; one `SetDeadline`; Flush once after N encodes; read N slots; `-ERR` on a slot does not fail the batch error; leave `exec` retry as-is
- [x] 1.3 Same-package flush-counting helper (`countingConn` on the client `net.Conn`, seed idle; no production flush counter). Compiled tests: N small commands → one Write and N ordered slots; empty does not dial; 65 commands → `redis:issue?` and no send; element-3 `-ERR` returns the other 9 and a later command reuses the conn; mid-pipeline truncation destroys the conn and is not retried; dead idle socket retried once only before successful Flush
- [x] 1.4 Run `go test ./simpleredis/...` until the new compiled tests pass

## 2. Yaegi

- [x] 2.1 Extend `clientprobe` / `yaegi_test.go` so interpreted code calls `ExecPipeline` against the compiled fake (GOPATH, stdlib only, `useunsafe` false, no Traefik)
- [x] 2.2 Run `go test ./simpleredis/...` until Yaegi tests pass

## 3. Traefik e2e Redis and Dragonfly

- [ ] 3.1 After existing sequential verbs in `e2e/simpleredisprobe`, run one mixed `ExecPipeline` on unique per-request keys: INCR, EXPIRE, GET of that incr key, EVAL of `kongIncrbyExpireatScript` with `KEYS[1]` on a distinct eval key (Lua 5.1-safe); set `X-SimpleRedis-Pipeline` to `1:ok:1:3`; keep every existing `X-SimpleRedis-*` header; no new compose service, route, or Dragonfly pipeline flag
- [ ] 3.2 Pester asserts `X-SimpleRedis-Pipeline` is `1:ok:1:3` on `/redis` and `/dragonfly` and does not stop whoami-a/b
- [ ] 3.3 Run `./Test-Integration.ps1` until Redis and Dragonfly Describes pass (live mixed-verb pipeline on both engines). Fake flush-count tests MUST NOT substitute for this

## 4. Specs

- [ ] 4.1 Confirm the change deltas `std_go_simpleredis_resp-commands` and `std_go_simpleredis_tcp-session` match the landed API
- [ ] 4.2 Run `openspec validate simpleredis-exec-pipeline --strict`
