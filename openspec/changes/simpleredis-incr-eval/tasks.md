## 1. Client verbs

- [ ] 1.1 Extend `readReply` `*` so each element may be `$`, `:`, or `+` (null bulk stays a nil slot); nested `*`/`-` → `redis:issue?` and `reusable false` when the stream cannot be skipped
- [ ] 1.2 Add unexported `parseIntegerReply`; add `Incr`, `IncrBy`, `Expire`, `ExpireAt`, `Eval` as `exec` wrappers; update package/`Close` comments; no EVALSHA, pipeline, or pool change
- [ ] 1.3 Extend `startFakeRedis` for INCR/INCRBY (digit store, `:<n>`), EXPIRE/EXPIREAT (`lastExpire`, `:1`), and the Kong incrby+expireat EVAL script; keep `startStaticRedis` for canned Eval encoder tests
- [ ] 1.4 Add compiled tests: Incr 1 then 2; IncrBy 5; non-integer `-ERR`; Expire/ExpireAt argv; Eval argv + integer `"7"`; existing Get/Set/Del/MGet still pass
- [ ] 1.5 Run `go test ./simpleredis/...` (excluding Yaegi file if needed) until the new compiled tests pass

## 2. Yaegi

- [ ] 2.1 Extend `clientprobe` / `yaegi_test.go` so interpreted code calls Incr and Eval against the compiled fake (GOPATH, stdlib only, `useunsafe` false, no Traefik)
- [ ] 2.2 Run `go test ./simpleredis/...` until Yaegi tests pass

## 3. Traefik e2e Redis and Dragonfly

- [ ] 3.1 Extend `e2e/simpleredisprobe` `ServeHTTP` to run Set, Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, Eval (Kong KEYS snippet) on per-request keys; keep `X-SimpleRedis-Value`; one header per other verb
- [ ] 3.2 Add compose `dragonfly` (`docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2`, hostname `dragonfly`, `ulimits.memlock: -1`) and `whoami-dragonfly` PathPrefix `/dragonfly` with plugin host `dragonfly:6379`. Keep `reclaim-e2e`, `/redis`, `/a` `/b`
- [ ] 3.3 Wait for Dragonfly (`redis-cli -h dragonfly ping` via the redis service) and HTTP `/dragonfly`; Pester asserts every verb header on `/redis` and `/dragonfly` and does not stop whoami-a/b; CI logs dump dragonfly and whoami-dragonfly
- [ ] 3.4 Run `./Test-Integration.ps1` until Redis and Dragonfly Describes pass and reclaim stays green

## 4. Specs

- [ ] 4.1 Confirm the change delta `std_go_simpleredis_resp-commands` matches the landed API
- [ ] 4.2 Run `openspec validate --change simpleredis-incr-eval --strict`
