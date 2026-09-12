# Explore
IssueKey: 2026-09-11-simpleredis-test-04-truncated-bulk

Verdict: in progress

## Concepts

SimpleRedis already has the anti-corruption chain the finding names. A truncated bulk is a **transport close**, not a Redis/Dragonfly command (`knowledge/research/ext_redis_resp_bulk-string/`). Dest usage `knowledge/devdocs/std_go_simpleredis.md` does not document `clean` / pool discard; middleware authors do not need that Language. No client address, user, tenant, or request Host is reconstructed.

```
  GET/MGET
     │
     ▼
  exec ──► borrow idle-or-dial
     │
     ▼
  do → readReply ──► readBulk ($len, then io.ReadFull length+2)
     │                  miss ($-1) → clean true, redis:miss
     │                  short read / bad $len / bad array head → clean false
     ▼
  reusable false → release closes; does not append idle
  reusable true  → idle (cap 8)
     │
     ▼
  exec retries once only when reused && !reusable && not timeout
```

Measured dest coverage (`go test ./simpleredis -covermode=set`, 86.1%):

| Block | Dest lines | Count |
|---|---|---|
| `io.ReadFull` fail | `401.52,403.3` | 0 |
| `readBulk` non-`$` head | `390.38,392.3` | 0 |
| unparseable `$` length | `394.16,396.3` | 0 |
| top-level bulk non-miss error (`clean` false) | `348.21,350.4` | 0 |
| array-element bulk error | `372.23,374.6` | 0 |
| array-element head not `$`/`: `/`+` | `378.12,379.32` | 1 (`TestEvalNestedArrayIsIssue`, `*1\r\n*0\r\n`) |
| `length < 0` miss | `397.16,399.3` | 1 |

`readReply` only calls `readBulk` after it has seen `$`, so `390-392` is unreachable from Get/MGet/Eval. `$abc\r\n` is the public malformed-header path (`394-396` + `348.21`).

`startStaticRedis` writes one complete canned RESP per command and never closes mid-payload (`simpleredis_test.go:228-256`). `fakeRedis` always emits a full `bulk()`. Both stay. Truncate needs a **new test helper** that writes raw bytes and closes.

Live engines: compose already has `redis:7-alpine` and `dragonfly:v1.40.2` plus `/redis` `/dragonfly`. Probe Sets `"ok"` on a per-request key (`e2e/simpleredisprobe/plugin.go:61-92`). Pester asserts both Value and MGet headers are `"ok"` (`scripts/integration-tests.Tests.ps1:90-114`). Two callers of those headers (those two Its). If request A read request B’s key, both would still be `"ok"` — the live own-value check is vacuous.

Throwaway (temp test, deleted): dest Get against `$100\r\n` + 40 bytes + close returned `redis:unreachable`, `len(idle)==0`, second Accept served `$5\r\nhello\r\n` and Get returned `hello`. Production `readBulk`/`do`/`release` already matches Out of scope. The gap is the missing committed tests (and a non-vacuous live assertion).

`exec` retries a dead **reused** socket (`simpleredis.go:190-200`). After truncate+close, a wrongly pooled conn would fail the next Get and then redial. A second Get that returns the right value can pass even if `release` pooled the dirty socket. `len(idle)==0` after the failed call is the load-bearing anti-corruption assert. Out of scope: changing that retry (test-05).

## Decisions

**Unit fake for truncate/`clean`; live Redis and Dragonfly for own-value Get/MGet.** Do not add a compose proxy or extra whoami to inject `$100` then 40 bytes. Research: complete bulk is `$<n>\r\n` + n bytes + CRLF; a lying length plus close is not a server command.

**Keep `startStaticRedis` and `fakeRedis`.** Add a same-package helper in `simpleredis/simpleredis_test.go` that, per Accept, reads one command, writes caller bytes, and may close without the final CRLF. First Accept: `$100\r\n` + 40 bytes + close. Later Accepts: a complete bulk for a known key so the second Get can prove recovery. Name it for that job (`startRawReplyRedis` or equivalent). Tests only — no production identifier.

**Committed unit tests (same package, so `idle` is visible):**

1. Truncated Get: error `redis:unreachable` (not `redis:issue?`), then `len(idle)==0`.
2. Second Get on that client against the next Accept returns that key’s own bytes.
3. `$abc\r\n`: `redis:issue?`, `len(idle)==0`.
4. Array element head neither `$`, `:`, nor `+` (for example `*1\r\n#x\r\n`): `redis:issue?`, `len(idle)==0`. Leave `TestEvalNestedArrayIsIssue` as-is.
5. Same-package `readBulk` with a non-`$` head so `390-392` is executed. Do not change `readReply` to make that branch reachable from Get.

Do not add Yaegi or a second test file for this. Do not cover array-element short-read (`372.23`) unless the same helper makes it free; the finding’s truncated example is a top-level `$100` Get.

**Do not edit `simpleredis.go` unless a committed test fails on dest.** Throwaway passed. Out of scope forbids a production rewrite when dest already returns `clean == false` on short read.

**Live isolation on existing `/redis` and `/dragonfly`.** No new compose services, routes, or image pin changes. Probe Set/Get/MGet store the request prefix (not `"ok"`). Pester: Value equals that unique token, MGet equals Value; two overlapping requests per route with distinct tokens. Other verb headers stay. Eval script stays the dest Kong snippet (`KEYS[1]`, Lua 5.1-safe). Searched `X-SimpleRedis-Value` / `X-SimpleRedis-MGet`: 2 Pester Its + `plugin.go` (archive trees not migrated).

**Propose extends existing specs.** `std_go_simpleredis_tcp-session`: a short-read or protocol-garbage reply MUST NOT return the socket to idle; callers match `redis:unreachable` vs `redis:issue?` as today. `std_go_simpleredis_resp-commands`: Pester own-value (unique bytes), not a hardcoded `"ok"`. No new spec family.

## Open questions

- Q: Can live Redis or Dragonfly announce `$100` and close after 40 bytes so truncated/`clean == false` is an e2e test?
  Rank: additive asked — Unknowns name the missing live injection seam; Desired still requires truncated unit coverage
  Decision: assumed — no. Official RESP is complete values (`ext_redis_resp_bulk-string`). Truncate stays unit-only on the raw-reply fake. Live engines prove Get/MGet own-value only. No new compose proxy.
  By: explore

- Q: Is compose “extend” new services/routes or new assertions on `/redis` and `/dragonfly`?
  Rank: additive asked — Affected names compose only if live isolation needs more than today’s whoami+probe; Desired names those two routes
  Decision: assumed — existing `whoami-redis` / `whoami-dragonfly` and image pins stay. New Pester assertions (and probe payload) only.
  By: explore

- Q: How far must Pester go past today’s unique-key Get/MGet `"ok"` to prove the caller’s own value?
  Rank: bounded asked — 2 Pester Its + `e2e/simpleredisprobe/plugin.go` Set/Get/MGet (searched `X-SimpleRedis-Value` / `X-SimpleRedis-MGet` in `*.go` `*.ps1` `*.yml`); Desired live Get/MGet own-value; constant `"ok"` makes a swapped key still pass
  Decision: assumed — Set the per-request prefix as the payload; Pester requires Value == that token and MGet == Value; two overlapping GETs per route with distinct tokens. Do not add routes.
  By: explore

- Q: What fake covers truncated bytes without changing `startStaticRedis`?
  Rank: additive asked — Desired: startStaticRedis stays; add a fake that writes raw bytes and closes mid-stream
  Decision: assumed — new helper in `simpleredis_test.go` only. First Accept truncates; later Accepts return a complete bulk. Malformed complete lines may use that helper or `startStaticRedis`; both assert `len(idle)==0`.
  By: explore

- Q: Is `len(idle)==0` still required if the second Get already returns the right key?
  Rank: additive asked — Desired unit leak invariant and idle==0; Out of scope forbids exec retry rewrite (test-05)
  Decision: assumed — yes. `exec` retries a dead reused socket, so a second Get can succeed even if the dirty conn was pooled. Idle==0 after the failed call is the anti-corruption assert; the second Get proves the client still dials a clean socket.
  By: explore

- Q: Do dest `readBulk` / `do` / `release` need a production change?
  Rank: additive asked — Out of scope: no production change when dest already returns `clean == false` on short read; Affected allows `simpleredis.go` only if tests prove it wrong
  Decision: resolved — no. Committed truncated Get: `redis:unreachable`, idle 0, second Get `hello`. `simpleredis.go` not edited.
  By: implement

- Q: Coverage block ids `401.52,403.3` will move if `readBulk` is edited — what is the proof?
  Rank: additive asked — Unknowns name the dest-line ids; Desired: that block becomes non-zero and the invariant fails if `clean==true` or `release` pools `reusable==false`
  Decision: resolved — invariant tests landed; `go test ./simpleredis -covermode=set` shows `401.52,403.3` count 1 (was 0). Production not edited so the dest id still matches.
  By: implement

- Q: How is `readBulk`’s non-`$` head (`390-392`) covered when `readReply` only calls it for `$`?
  Rank: additive asked — Desired malformed bulk header plus finding `:390-392`; public Get cannot reach that branch
  Decision: assumed — same-package test calls `readBulk` with a non-`$` head and expects `redis:issue?`. Do not widen `readReply`. `$abc\r\n` covers the unparseable-length branch via Get.
  By: explore

- Q: Which spec leaves take the dirty-conn and live own-value requirements?
  Rank: additive asked — Desired tests of pool discard and live Get/MGet own-value; existing families already own session vs commands
  Decision: assumed — fold into `std_go_simpleredis_tcp-session` (dirty reply not idle-pooled) and `std_go_simpleredis_resp-commands` (Pester unique own-value). No new spec folder. No rename.
  By: explore

- Q: Does this change add or keep Lua, and must it stay Lua 5.1 / Dragonfly KEYS?
  Rank: additive asked — Desired Lua 5.1-safe and Dragonfly KEYS even if no new script
  Decision: resolved — keep dest `kongIncrbyExpireatScript` (`e2e/simpleredisprobe/plugin.go:17-23`). No new script. Research: `ext_redis_eval`, `ext_dragonfly_eval`.
  By: explore

- Q: Who already owns client address / user / tenant / Host for this work?
  Rank: additive asked — explore requires an owner when identity is in play; this unit does not reconstruct request identity
  Decision: resolved — none. The probe prefix is `time.Now().UnixNano()`, not `RemoteAddr` or a forwarded header. Traefik still owns plugin `New` ctx. Redis keys are opaque bytes.
  By: explore
