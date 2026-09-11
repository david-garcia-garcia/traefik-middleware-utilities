## 1. Decoder comment and malformed table

- [ ] 1.1 At `readReply` `count < 0`, add a comment that `*-1` is a legal RESP2 nil array (BLPOP timeout, EXEC abort) and this client has no verb that receives it, so it is `redis:issue?` not `redis:miss`. Do not map to `errMiss`
- [ ] 1.2 Add a compiled table over `startStaticRedis`: HTTP-shaped (`HTTP/1.1 …`), `?huh`, `:42\n`, `\r\n`, `*abc`, `*-1`, `*1\r\n?bad\r\n`, `*1\r\n\r\n`, nested `*1\r\n*0\r\n`. Each row asserts the expected error (`redis:issue?`; `*-1` is not `redis:miss`) **and** `len(idle)==0`. Fold `TestEvalNestedArrayIsIssue` into this table
- [ ] 1.3 Run `go test ./simpleredis/...` until the malformed table passes

## 2. Truncated I/O and retry-borrow

- [ ] 2.1 Add a write-then-close helper (read one command, write a partial array or bulk, close). Table truncated array and truncated bulk; assert `redis:unreachable` and `len(idle)==0`. Do not use `startStaticRedis` for these rows
- [ ] 2.2 Add a one-accept helper: first command `$1\r\nt\r\n` so Get pools; close the listener; second command on that conn is malformed; second Get expects `redis:unreachable` and `idle==0`. Do not close the accepted conn
- [ ] 2.3 Run `go test ./simpleredis/...` until truncated and retry-borrow tests pass

## 3. Verb arity

- [ ] 3.1 Add a second compiled table: Get `*0`, Get `*2` (two bulks), Incr `*0`. Assert `redis:issue?` and `len(idle)==1`. Do not destroy the socket. Keep `TestIncrGarbageIntegerPayload`
- [ ] 3.2 Run `go test ./simpleredis/...` until arity rows pass and existing Get/Incr tests still pass

## 4. Live Get-miss on Redis and Dragonfly

- [ ] 4.1 In `e2e/simpleredisprobe`, Get a never-set key; on `simpleredis.RedisMiss` set `X-SimpleRedis-GetMiss` to `redis:miss`. Keep every existing verb header. Keep `kongIncrbyExpireatScript` (KEYS, no `table.maxn`)
- [ ] 4.2 Pester asserts `X-SimpleRedis-GetMiss` is `redis:miss` on `/redis` and `/dragonfly`. Keep existing verb Its. Do not stop `whoami-a` or `whoami-b`. Do not add a third engine
- [ ] 4.3 Run `./Test-Integration.ps1` until Redis and Dragonfly Describes pass and reclaim stays green

## 5. Specs

- [ ] 5.1 Confirm the change delta `std_go_simpleredis_resp-commands` matches the landed tests and comment
- [ ] 5.2 Run `openspec validate simpleredis-malformed-reply --strict --type change`
