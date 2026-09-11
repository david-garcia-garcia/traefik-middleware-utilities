# Explore
IssueKey: 2026-09-11-simpleredis-test-06-malformed-reply

Verdict: in progress

## Concepts

`readReply` / `readLine` are the only RESP decoder. `do` is the only caller (`simpleredis/simpleredis.go:299`). A dirty decode (`clean=false`) makes `release` close the socket, so it must not re-enter `idle`.

```
  exec → borrow → do → writeCommand
                    → readReply / readLine / readBulk
                    → release(reusable=clean)
         retry once only when reused && !clean && err != timeout
```

Two failure families, not one:

| Family | Wire | `clean` | After `Get`/`Incr` | `idle` |
|---|---|---|---|---|
| Decoder poison | unknown type, missing CR, empty line, bad `*` count (including `*-1`), truncated element/bulk, bad element type | false | `redis:issue?` or I/O (`redis:unreachable` on EOF, `redis:timeout` on deadline) | 0 |
| Verb arity | well-formed `*0` / `*2` into `Get` or `parseIntegerReply` | true | `redis:issue?` from the verb | 1 (conn returned) |

Today only `TestEvalNestedArrayIsIssue` (`*1\r\n*0\r\n`) and `TestIncrGarbageIntegerPayload` (`:not-an-int\r\n`) exist. Nested-array is dirty and does not assert `idle==0`. Garbage integer is a valid `:` line; `ParseInt` fails after a clean decode — not the `len!=1` branch.

Live engines already sit on compose (`redis:7-alpine`, `dragonfly:v1.40.2`) with Pester `/redis` and `/dragonfly` asserting every verb. They will not emit malformed RESP. Eval in the probe is `kongIncrbyExpireatScript` (KEYS declared, no `table.maxn`).

Null array vs null bulk (`knowledge/research/ext_redis_resp_null-array/`):

| Form | Wire | Dest today |
|---|---|---|
| Null bulk | `$-1` | `redis:miss`, `clean=true` (`readBulk:397-398`) |
| Empty array | `*0` | empty `values`, `clean=true` |
| Null array | `*-1` | `count < 0` → `errIssue`, `clean=false` |
| Lua `false` | null bulk, not null array (`ext_redis_eval`) | `redis:miss` |

Official RESP2: a client should return a null object for `*-1` (BLPOP timeout, EXEC abort). This client has no BLPOP / MULTI / EXEC (out of scope). No current verb is specified to receive a null array.

Identity (client address, user, tenant, Host) is not reconstructed. `Init` host is the Redis/Dragonfly address from caller config.

Usage packet `knowledge/devdocs/std_go_simpleredis.md` is enough to call the client. No Language gap. Research indexes already answer RESP2 null-array, Redis EVAL, and Dragonfly KEYS / Lua 5.4 `table.maxn`. No new research folder.

## Decisions

**Keep RESP2 null array (`*-1`) as a protocol violation.** At `readReply` `count < 0` (`:354`), stay `errIssue` `clean=false`. Add a comment there: `*-1` is a legal nil array (BLPOP timeout, EXEC abort); this client has no verb that receives it, so it is not `redis:miss`. Do not map it like `$-1`. Mapping to `errMiss` would make a null collection look like a missing GET key (`windowcounter` `getCount` already treats miss as zero). Official “return a null object” is for verbs this library does not speak.

`readReply` has 1 caller (`do`). Production SimpleRedis verbs that would observe a mapping: `windowcounter/limiter.go` Get/Incr/Eval, `tokenbucket/redis.go` Eval, `e2e/simpleredisprobe/plugin.go` every verb. Keeping dest behavior does not migrate them.

**Malformed coverage is a compiled fake-server table, not live engines.** One table over canned replies via `startStaticRedis` for complete junk (unknown type including HTTP-shaped `HTTP/1.1 …`, `?huh`, missing CR `:42\n`, empty `\r\n`, `*abc`, `*-1`, `*1\r\n?bad\r\n`, empty element line). Truncated array / truncated bulk: a write-then-close helper, not `startStaticRedis` (that keeps the socket open → 1s `ioTimeout` → `redis:timeout`). Assert expected error **and** `len(idle)==0` on every dirty row. Fold nested-array into that table with the idle assert. Do not drive malformed bytes from Redis or Dragonfly.

**`Get` / `parseIntegerReply` arity is a second table (or rows) without `idle==0`.** Canned `*0` and `*2` (bulks or integers). Assert `redis:issue?`. The conn stays pooled because `readReply` succeeded. Do not destroy the socket on arity mismatch (that would add a hygiene branch to verbs whose job is not pooling). `TestIncrGarbageIntegerPayload` stays; it is not `len!=1`.

**`exec` retry-borrow failure (`:195-197`) is a dedicated helper, not the canned table.** `startStaticRedis` always Accepts again, so a dirty reused conn retries onto a new conn and never hits borrow-fail. Recipe: accept one connection; first command replies a hit (`$1\r\nt\r\n`) so `Get` pools; close the listener; second command on that same conn replies malformed; second `Get` reuses, dirties, retries `borrow`, dial fails → `redis:unreachable`. Do not close the accepted conn instead (that is test-01 EOF redial, out of scope).

**Keep and extend live happy-path on both engines.** Compose already has `redis` + `dragonfly` + `whoami-redis` `/redis` + `whoami-dragonfly` `/dragonfly`. Do not add a third engine or a second compose project. Extend `e2e/simpleredisprobe` + Pester with a Get-miss header (`redis:miss` on a missing key) on **both** `/redis` and `/dragonfly` so live `$-1` stays proven beside fake `*-1` as issue. Every existing verb header stays. Eval stays `kongIncrbyExpireatScript` (KEYS, Lua 5.1-safe, no `table.maxn`). Yaegi `simpleredis/yaegi_test.go` stays compiled-fake happy-path; malformed does not need an interpreter table.

**Spec host is existing `std_go_simpleredis_resp-commands`.** Propose adds: dirty decode → `redis:issue?` or I/O and not pooled; `*-1` is issue not miss; live Traefik e2e on both engines including Get-miss. Do not start a new spec family. Usage Gotcha for `*-1` waits for implement / devdocsimpact.

**Out of scope stays out.** No perf-*, test-01–05, test-07, feat-01, MULTI/EXEC, BLPOP, EVALSHA, pipelining, pool-cap, go-redis, miniredis, TLS.

## Open questions

- Q: Null-array (`*-1`): keep as `redis:issue?` + destroy conn (comment at `:354`), or map to `errMiss` like null bulk?
  Rank: additive asked — comment (or a mapping) on the existing `count < 0` check; Desired and Unknowns name this decision; Current cites `:354`
  Decision: resolved — keep as protocol violation + comment at the `count < 0` check. `*-1` is a legal RESP2 nil array for BLPOP timeout and EXEC abort (`knowledge/research/ext_redis_resp_null-array/`); this client has no such verb; `errMiss` is the missing-key bulk (`$-1`). Do not leave it as an accidental `count < 0`.
  By: explore

- Q: How to exercise `exec` borrow-failure on the retry attempt (`:195`) with a canned reply?
  Rank: additive asked — new test helper this change creates; Problem, Current `:195-197`, and Unknowns name the branch
  Decision: assumed — dedicated one-accept helper (good first reply, then malformed on the reused socket, listener closed before retry dial). Expect `redis:unreachable` and `idle==0`. Not a `startStaticRedis` table row.
  By: explore

- Q: Do `Get` / `parseIntegerReply` count-mismatch rows assert `len(idle)==0` the same as decoder poison?
  Rank: additive asked — Desired names covering those branches via canned replies; Current cites `:93-95` and `:171-173`
  Decision: assumed — cover with canned `*0` / `*2`; assert `redis:issue?`; do **not** assert `idle==0` (conn is clean). Do not change `Get` / `release` to destroy on arity mismatch. Deviation recorded.
  By: explore

- Q: How do live Redis and Dragonfly stay in the proof if malformed cases are fake-server only?
  Rank: additive asked — Desired and conductor HARD REQUIREMENT: both engines; extend compose + Pester `/redis` `/dragonfly`; do not replace live happy-path
  Decision: assumed — keep compose `redis:7-alpine` and `dragonfly:v1.40.2` and both whoami routes. Extend probe + Pester with Get-miss on `/redis` and `/dragonfly`. Table-driven malformed stays in-process. A decoder change still has to pass both live Its.
  By: explore

- Q: Which Eval script may live tests send?
  Rank: additive asked — Desired: Lua 5.1-safe and KEYS listed (Dragonfly)
  Decision: resolved — keep `kongIncrbyExpireatScript` in `e2e/simpleredisprobe` and compiled/Yaegi tests. No `table.maxn`. No undeclared keys (`ext_dragonfly_eval`).
  By: explore

- Q: Which spec leaf takes malformed and null-array?
  Rank: additive asked — Affected names `openspec/specs/std_go_simpleredis_resp-commands/spec.md` if the contract is specified
  Decision: assumed — delta on `std_go_simpleredis_resp-commands` (dirty decode + idle; `*-1` is issue; live Get-miss on both engines). No new spec family.
  By: explore

- Q: Who already owns client address / user / tenant / Host for this client?
  Rank: additive asked — explore requires an owner when identity is in play; this unit does not reconstruct request identity
  Decision: resolved — none. `Init` host is the Redis/Dragonfly server address from caller config. The probe does not read `RemoteAddr` or forwarded headers.
  By: explore
