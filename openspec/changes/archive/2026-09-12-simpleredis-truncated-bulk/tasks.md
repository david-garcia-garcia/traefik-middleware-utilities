## 1. Unit tests

- [x] 1.1 Add a same-package helper in `simpleredis/simpleredis_test.go` that, per Accept, reads one command, writes caller bytes, and may close mid-stream. Keep `startStaticRedis` and `fakeRedis`. Name it for that job (`startRawReplyRedis` or equivalent)
- [x] 1.2 Truncated Get: `$100\r\n` + 40 bytes + close → `redis:unreachable` (not `redis:issue?`) and `len(idle)==0`
- [x] 1.3 After that failed Get, a second Get on the same client against the next Accept’s complete bulk returns those bytes
- [x] 1.4 `$abc\r\n` via Get → `redis:issue?`, `len(idle)==0`. Array element head neither `$`, `:`, nor `+` (for example `*1\r\n#x\r\n`) → `redis:issue?`, `len(idle)==0`. Leave `TestEvalNestedArrayIsIssue` as-is
- [x] 1.5 Same-package `readBulk` with a non-`$` head → `redis:issue?`. Do not change `readReply`
- [x] 1.6 Run `go test ./simpleredis/...`. Edit `simpleredis.go` only if a committed test fails. Measure that the `ReadFull` fail block is non-zero

## 2. Live Get/MGet own-value

- [x] 2.1 Probe Set/Get/MGet store the per-request prefix as the payload (not `"ok"`); keep other verb headers; keep dest `kongIncrbyExpireatScript` (`KEYS[1]`, Lua 5.1-safe). No new script
- [x] 2.2 Pester `/redis` and `/dragonfly`: Value equals that token, MGet equals Value; two overlapping GETs per route with distinct tokens; other verb headers stay. Do not stop `whoami-a`/`whoami-b`. Do not add compose services, routes, or image pins
- [x] 2.3 Run `./Test-Integration.ps1` until Redis and Dragonfly Describes pass and reclaim stays green

## 3. Specs

- [x] 3.1 Confirm the change deltas `std_go_simpleredis_tcp-session` and `std_go_simpleredis_resp-commands` match the landed tests
- [x] 3.2 Run `openspec validate --change simpleredis-truncated-bulk --strict`
