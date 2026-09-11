## 1. Client verbs

- [x] 1.1 Add pair-count cap `1024` next to `maxIdleConns`; add a tri-state native/lua field on `SimpleRedis` guarded by existing `mu`; package/`Close` comments list `MSetEX` / `MSetEXAt`
- [x] 1.2 Add one Lua 5.1-safe fallback script constant (`for i = 1, #KEYS`, no unpack/table.maxn) and one unexported group-write: validate empty/mismatch/cap (`errIssue`, no dial), native `MSETEX` argv, `parseIntegerReply` then require `1` (`0` is `errIssue`), `ERR unknown command` → cache lua and `Eval`, cached lua skips native; `MSetEX` / `MSetEXAt` are thin wrappers. No EVALSHA, NX/XX, pool change
- [x] 1.3 Extend `startFakeRedis`: `MSETEX` records argv, stores pairs, replies `:1` (delete on past EXAT); `rejectMSetEX` replies `-ERR unknown command 'MSETEX'`; EVAL special-cases the fallback script and applies SET+store; keep default `+OK` for other unknown verbs
- [x] 1.4 Add compiled tests: native argv EX and EXAT; integer `0` → `redis:issue?`; empty/mismatch/over-cap do not dial; unknown-command then cached EVAL (second call sends no `MSETEX`); past EXAT Get miss on the fake; existing Get/Set/Del/MGet/Incr/Eval still pass
- [x] 1.5 Run `go test ./simpleredis/...` until the new compiled fake tests pass

## 2. Yaegi

- [x] 2.1 Extend `clientprobe` / `yaegi_test.go` with interpreted MSetEX against a compiled fake that implements MSETEX, and against a fake with `rejectMSetEX` (second call does not send `MSETEX`). GOPATH, stdlib only, `useunsafe` false, no Traefik
- [x] 2.2 Run `go test ./simpleredis/...` until Yaegi tests pass

## 3. Live Redis and Dragonfly

- [x] 3.1 Add `simpleredis/live_test.go` table-driven on `SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY`: skip `-short` or unset; `MSetEX` then Get plus Eval TTL on a KEYS key (TTL > 0); past `MSetEXAt` then Get miss. Copy `waitLiveClient` shape from `windowcounter/live_test.go` (probe with Set/Get, not Incr-only)
- [x] 3.2 Set CI `test` job env `SIMPLEREDIS_LIVE_REDIS: 127.0.0.1:6379` and `SIMPLEREDIS_LIVE_DRAGONFLY: 127.0.0.1:6380` (existing services). Do not add Valkey or Redis 8
- [x] 3.3 Run `go test ./simpleredis/...` with both env vars against local Redis 7 and Dragonfly when those listeners exist; `-short` still skips

## 4. Traefik e2e Redis and Dragonfly

- [x] 4.1 Extend `e2e/simpleredisprobe` `ServeHTTP` to call `MSetEX` and `MSetEXAt` on per-request keys, then Eval `return redis.call('TTL', KEYS[1])`; headers `X-SimpleRedis-MSetEX` and `X-SimpleRedis-MSetEX-TTL`. Keep existing verbs and `X-SimpleRedis-Value`
- [x] 4.2 Confirm compose still has `/redis` and `/dragonfly` probe routes on `redis:7-alpine` and `dragonfly:v1.40.2`. Do not add a Valkey service. Pester asserts the new headers on both routes and does not stop whoami-a/b
- [x] 4.3 Run `./Test-Integration.ps1` until Redis and Dragonfly Describes pass and reclaim stays green

## 5. Specs and usage

- [x] 5.1 Update `knowledge/devdocs/std_go_simpleredis.md` Language verb list, snippet, and gotchas (pair cap, hash tags, Dragonfly KEYS fallback, Lua is the Redis 7/Dragonfly path)
- [x] 5.2 Update main spec Purpose on `openspec/specs/std_go_simpleredis_resp-commands/spec.md` to include MSetEX/MSetEXAt
- [x] 5.3 Confirm the change delta `std_go_simpleredis_resp-commands` matches the landed API
- [x] 5.4 Run `openspec validate simpleredis-msetex --type change --strict`
