## 1. Client retry gate

- [x] 1.1 Change `exec` to `exec(retryDeadPool bool, args ...[]byte)`. On a dead reused socket that is not a timeout, retry only when `retryDeadPool` is true; otherwise return the first `do` error without a second send. Do not parse the verb inside `exec`. Timeouts stay non-retried for every verb
- [x] 1.2 Pass `true` from Get, MGet, Set, Del, Expire, ExpireAt. Pass `false` from Incr, IncrBy, Eval. Leave exported signatures unchanged

## 2. Fake lost-reply tests

- [x] 2.1 Add a test-named one-shot on dest `fakeRedis` (`closeBeforeReplyOnce` or equivalent): apply the command (mutate INCR/INCRBY/EVAL), then close without writing the reply. Do not add truncated-bulk
- [x] 2.2 Compiled test: warm the pool with Get, then one Incr with close-before-reply → `redis:unreachable`, stored `1`, one INCR on the fake. Mirror: close-before-reply Get still succeeds and the fake sees a second connection. Same IncrBy/Eval close-before-reply pins one apply
- [x] 2.3 Run `go test ./simpleredis/...` until compiled fake tests and existing Yaegi happy-path Incr/Eval pass. Do not add a Yaegi lost-reply harness

## 3. Compose drop-relay and Pester `/redis` `/dragonfly`

- [ ] 3.1 Add `e2e/respdroprelay` (stdlib Go `main`, not imported by `simpleredis`): env `UPSTREAM`, listen `:6379`; forward one RESP command, read the engine reply, close the client without writing that reply for INCR/INCRBY/EVAL, pass other verbs through
- [ ] 3.2 Compose `redis-drop` (`UPSTREAM=redis:6379`) and `dragonfly-drop` (`UPSTREAM=dragonfly:6379`), `depends_on` the matching engine. Keep `reclaim-e2e`, happy-path Host `redis:6379` / `dragonfly:6379`, `/a` `/b`
- [ ] 3.3 Probe `DropHost`: Init a second client in `New` (no dial). After happy-path verbs, warm the drop client with a pass-through Get, then Incr and Eval through the drop client (Kong KEYS snippet, Lua 5.1-safe). Get stored values from the happy-path client. Set `X-SimpleRedis-DropIncr`, `X-SimpleRedis-DropIncrStored`, `X-SimpleRedis-DropEval`, `X-SimpleRedis-DropEvalStored`. Do not `http.Error` on expected `redis:unreachable`. Labels `dropHost=redis-drop:6379` and `dropHost=dragonfly-drop:6379`
- [ ] 3.4 Pester `/redis` and `/dragonfly`: keep existing happy-path headers; assert DropIncr `redis:unreachable`, DropIncrStored `1`, DropEval `redis:unreachable`, DropEvalStored the one-apply script result. Do not stop whoami-a/b. CI logs dump the drop-relay services
- [ ] 3.5 Run `./Test-Integration.ps1` until Redis and Dragonfly Describes pass (lost-reply headers plus happy-path) and reclaim stays green

## 4. Specs

- [ ] 4.1 Confirm the change delta `std_go_simpleredis_tcp-session` matches the landed retry gate, fake tests, and Pester `/redis` `/dragonfly` proof. Do not add a retry delta on `resp-commands`
- [ ] 4.2 Run `openspec validate --change retry-only-idempotent-commands --strict`
