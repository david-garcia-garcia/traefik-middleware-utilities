# Explore
IssueKey: 2026-09-11-simpleredis-test-05-non-idempotent
Verdict: in progress

Dest does not contradict the conductor policy. `exec` retries every verb after a dead reused socket (`simpleredis/simpleredis.go` 181–201): no verb check, timeout is the only skip. That is the gap.

## Concepts

- **SimpleRedis** — dest stdlib pooled TCP RESP client (`simpleredis/simpleredis.go`). Wrappers `Get`/`MGet`/`Set`/`Del`/`Incr`/`IncrBy`/`Expire`/`ExpireAt`/`Eval` all call `exec`. Callers match `Error()` text (`redis:unreachable`, `redis:timeout`, …).
- **exec retry** — after `do` fails, retry once when the conn was **reused**, the socket is not reusable, and the error is not `redis:timeout`. A **new** dial that fails is already not retried (`!reused`). AUTH/SELECT in `dial` call `do` directly, not `exec`.
- **Case 1 vs case 2** — (1) write never reached the engine (idle socket already closed: `TestStaleConnectionIsRetried` with `Get`). (2) write landed, engine applied, reply lost. `do` sets `reusable=false` on write **or** read IO failure (`simpleredis.go` 291–307). Dest cannot tell them apart. Distinguishing write-flush from read is **Out of scope**.
- **Non-retry verbs** — `INCR`, `INCRBY`, `EVAL`. Redis INCR/INCRBY mutate in place; missing key starts at 0 (`knowledge/research/ext_redis_incr/`). EVAL runs the script once per send. A lost reply plus a retry double-applies. Window counters and token-bucket scripts are the consumers (`windowcounter/limiter.go` `takeExact` Incr / `flushPending` Eval; `tokenbucket/redis.go` Eval). Those packages already return the SimpleRedis error; they are **Out of scope** to change.
- **Retry-safe verbs (as named)** — `GET`, `MGET`, `SET` (SET+EX on dest), `DEL`, `EXPIRE`, `EXPIREAT`. SET/EXPIRE retries can refresh TTL; the finding still lists them as fine to retry.
- **Fake Redis** — `startFakeRedis` / `fakeRedis.serve` (`simpleredis_test.go`). INCR/INCRBY/EVAL mutate `store` then write the reply. No execute-then-close-before-reply path. Truncated-bulk / test-04 harness: not on dest.
- **Yaegi** — `yaegi_test.go` Init/Get/Set/Del and Incr/Eval **happy path** against the compiled fake. Traefik is not started there. E2E under Traefik is `e2e/simpleredisprobe` + Pester `/redis` `/dragonfly`.
- **Live engines** — compose already has `redis:7-alpine` and `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2`. Probe `Config.Host` selects the engine. Pester asserts happy-path headers only. Neither server exposes an apply-then-drop hook. Eval scripts must be Lua 5.1-safe and list keys in `KEYS` (`knowledge/research/ext_dragonfly_eval/`).
- **tcp-session spec** — “A dead pooled connection SHALL be retried once unless the error is a timeout.” No verb exception. That sentence is the retry owner; `resp-commands` owns wire shapes, not retry.

```
reused conn, do fails, not timeout
        │
        ├─ GET/MGET/SET/DEL/EXPIRE/EXPIREAT → borrow again, send once more
        └─ INCR/INCRBY/EVAL                 → redis:unreachable (no second send)
```

Gap measured from dest (not a runtime crash): `TestStaleConnectionIsRetried` only covers Get after the **client** closed the idle socket. No test pins store value after server-side apply-then-drop. `go test` was not re-run this phase (unattended explore; no product edit).

Usage packet `knowledge/devdocs/std_go_simpleredis.md` does not yet describe retry-by-verb. Do not write it this phase — dest still retries every verb; the Gotcha belongs after apply (`opd-devdocsimpact`).

## Decisions

- **Retry only idempotent commands.** After a dead reused socket, retry `GET`/`MGET`/`SET`/`Del`/`EXPIRE`/`EXPIREAT`. Do **not** retry `INCR`/`INCRBY`/`EVAL`; return `redis:unreachable` instead of sending the args again. Timeouts stay non-retried for every verb (already true). Dest retries every verb today; that is the bug, not a competing policy. Caller packages keep failing closed on that error (windowcounter Take, tokenbucket Allow).
- Accept the false-negative: an INCR whose write **never** landed on a reused socket also returns `redis:unreachable` and is not retried. Out of scope forbids using write-vs-read as the gate.
- Verb policy, not script analysis: `Eval("return 1", nil, nil)` is still not retried. `IncrBy` with delta `0` is still not retried.
- Tag retry-safety at each `exec` call site with a bool (propose names the identifier). Not a command-descriptor table. Not a client option (Out of scope).
- Nine `sr.exec(` call sites, all in `simpleredis/simpleredis.go` (Get, MGet, Set, Del, Incr, IncrBy, Expire, ExpireAt, Eval). No `exec` callers outside that file. External `Incr`/`Eval` signatures stay; only the lost-reply error path changes.
- Fake: one-shot close-before-reply on dest `fakeRedis` — mutate store, then close without writing the reply. One `Incr` leaves stored `1` and returns `redis:unreachable`. Mirror: an idempotent verb (Get) still retries and the caller sees success. Do **not** invent the test-04 truncated-bulk harness.
- Live proof: compose RESP drop-relay in front of Redis and in front of Dragonfly. Relay forwards the command, waits for the engine reply (so the write applied), and closes the client without writing that reply when the verb is INCR, INCRBY, or EVAL. Other verbs pass through. Probe keeps happy-path `Host` (`redis:6379` / `dragonfly:6379`) and adds a second client to the drop-relay; same `/redis` and `/dragonfly` requests set extra headers. Pester asserts unreachable plus stored `1` (Incr) / stored script result (Eval, Kong KEYS snippet, Lua 5.1-safe).
- Spec: narrow the tcp-session dead-pool retry requirement by verb. Do not duplicate the retry rule on `resp-commands`.
- Yaegi compiled tests stay happy-path. Lost-reply is proven by `go test` fake tests plus Pester live engines.
- Do not change windowcounter, tokenbucket, pipelining, EVALSHA, go-redis, or miniredis.

## Open questions

- Q: What is the retry policy after a dead reused socket?
  Rank: bounded asked — 9 existing `sr.exec(` sites in `simpleredis/simpleredis.go` (Get, MGet, Set, Del, Incr, IncrBy, Expire, ExpireAt, Eval; searched that file for `sr.exec(`); Desired “Retry only idempotent verbs… INCR, INCRBY, and EVAL MUST NOT be retried”
  Decision: resolved — retry only GET/MGET/SET/DEL/EXPIRE/EXPIREAT; INCR/INCRBY/EVAL return `redis:unreachable` without a second send. Dest does not contradict (it retries every verb; that is the gap).
  By: explore

- Q: How to inject apply-then-drop on live Redis and Dragonfly?
  Rank: additive asked — new compose sidecar this change creates; Desired and HARD REQUIREMENT name live Redis and Dragonfly proof of Incr/Eval retry policy; compose + Pester `/redis` `/dragonfly`
  Decision: assumed — one RESP drop-relay service in front of each engine (upstream `redis:6379` and `dragonfly:6379`). Forward command, read engine reply, close client without writing the reply for INCR/INCRBY/EVAL; pass other verbs through. Probe uses a second SimpleRedis client to that relay on the existing `/redis` and `/dragonfly` routes. Do not use DEBUG SLEEP, CLIENT KILL, or a timeout (timeout is already non-retry). Propose owns binary layout and header names.
  By: explore

- Q: Exact `exec` tagging shape (bool vs descriptor)?
  Rank: bounded asked — same 9 `exec` call sites; Desired “Tag retry-safety at the exec call site (bool or small descriptor)”
  Decision: assumed — a bool at each wrapper (`true` for the six retry-safe verbs, `false` for INCR/INCRBY/EVAL). Propose names it. No descriptor type, no exported client option.
  By: explore

- Q: Invent the test-04 truncated-bulk fake here, or only close-before-reply?
  Rank: additive asked — Desired fake “execute, mutate store, close before writing the reply”; Out of scope names test-01–04
  Decision: resolved — only close-before-reply on dest `fakeRedis`. Do not build truncated-bulk.
  By: explore

- Q: Must Yaegi tests cover the lost-reply error, or do compiled fake tests plus Pester suffice?
  Rank: additive asked — Affected “`simpleredis/yaegi_test.go` if interpreted Incr/Eval must see the same error”
  Decision: assumed — compiled fake tests plus Pester `/redis` `/dragonfly` prove the policy. Yaegi keeps existing happy-path Incr/Eval (`TestYaegi_IncrAndEval`). Same `Error()` string; no new interp harness for close-before-reply.
  By: explore

- Q: Which spec leaf owns the verb exception on retry?
  Rank: bounded asked — Desired names `openspec/specs/std_go_simpleredis_tcp-session/spec.md`; Affected allows `resp-commands` “if command-level retry is specified there”. Existing tcp-session requirement “A dead pooled connection SHALL be retried once unless the error is a timeout.”
  Decision: assumed — tcp-session owns retry (narrow that sentence by verb). resp-commands stays command shapes only.
  By: explore
