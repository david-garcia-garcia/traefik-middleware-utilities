# Explore
IssueKey: 2026-09-11-simpleredis-feat-01-msetex

## Concepts

- **SimpleRedis** — dest Yaegi-safe stdlib RESP client (`simpleredis/simpleredis.go`). Exported today: `Init`, `Get`, `MGet`, `Set` (SET+EX), `Del`, `Incr`, `IncrBy`, `Expire`, `ExpireAt`, `Eval`, `Close`. `mu` guards pool/closed only. No `MSetEX` / `MSetEXAt`. No capability flag. `exec` is the only wire path; `Eval` already builds `EVAL` + `numkeys` + keys + args.
- **Group write** — limiter flush needs many keys, one shared TTL, one atomic round trip. Dest `Set` is one key. `MSET` then `EXPIRE` is not atomic. `MULTI`/`EXEC` and pipelining are out of scope.
- **MSETEX** — Valkey 9.1 and Redis 8.4 native: `MSETEX <numkeys> k1 v1 … EX|EXAT <n>` → integer `1` all set, `0` none set (NX/XX). This repo’s live engines are Redis 7 (`redis:7-alpine`) and Dragonfly v1.40.2; both lack the verb (`knowledge/research/ext_redis_msetex`, `ext_dragonfly_msetex`, `ext_valkey_msetex`). Lua `EVAL` is the default path those engines will take.
- **Fallback script** — one `EVAL`: keys in `KEYS` (Dragonfly rejects undeclared keys), values then expire token then TTL in `ARGV`, numeric `for i = 1, #KEYS` calling `SET … EX|EXAT`. No `unpack` / `table.unpack` / `table.maxn` (`ext_dragonfly_eval`). Reuse `Eval`; do not add `EVALSHA` (dest spec forbids it; sibling perf-05).
- **Capability cache** — first `MSetEX`/`MSetEXAt` sends native. `ERR unknown command` → cache miss on `SimpleRedis` under existing `mu`, then `EVAL` that call. Cached miss never probes again. Cached hit that later sees unknown-command recaches miss and `EVAL`s that call (failover). No `INFO server`.
- **Fake Redis** — `startFakeRedis` default unknown verbs reply `+OK`. Native tests need an `MSETEX` case that records argv and replies `:1`. Fallback tests need `-ERR unknown command 'MSETEX'`. Other unknown verbs stay `+OK`. Fake has no Lua VM; special-case the fallback script string (same pattern as Kong EVAL) or `startStaticRedis` canned `:1`.
- **Call sites of SimpleRedis** — `windowcounter/`, `tokenbucket/`, `e2e/simpleredisprobe/plugin.go`, `simpleredis` tests. None call `MSetEX` today. Limiter flush callers are out of scope. Adding methods and one cache field leaves those callers working.
- **Usage packet** — `knowledge/devdocs/std_go_simpleredis.md` lists today’s verbs. Do not rewrite it in explore (API unshipped). Implement / devdocsimpact add `MSetEX` / `MSetEXAt`, the pair cap, hash-tag cluster note, and Dragonfly KEYS fallback.
- **Identity** — this change does not set or reconstruct client address, user, tenant, Host, or trust hop.

```
  MSetEX / MSetEXAt
       │
       ├─ validate slices (empty / mismatch / cap) ── redis:issue?  (no dial)
       │
       ├─ cache unknown or native ──► MSETEX numkeys k v … EX|EXAT n
       │         │                        │
       │         │                        ├─ :1  → cache native, nil
       │         │                        ├─ :0  → redis:issue?
       │         │                        └─ ERR unknown command → cache lua, EVAL
       │
       └─ cache lua ──► Eval(script, names, values+token+ttl)  → :1 success
```

## Decisions

- Add `MSetEX(names []string, values [][]byte, seconds int64) error` and `MSetEXAt(..., unixSeconds int64) error` on `SimpleRedis`. Parallel slices, Yaegi-safe, no NX/XX/PX/KEEPTTL on the Go API. Always send `EX` or `EXAT` (do not omit expiration).
- Native argv: `MSETEX`, decimal `numkeys`, pairs in order, then `EX` or `EXAT`, then the decimal TTL. Integer `1` → `nil`. Integer `0` → `redis:issue?` (not swallowed as success; Expire’s `:0` success does not apply here). Other integers / garbage `:` → `redis:issue?`.
- Fallback: one Lua 5.1-safe script, `Eval` with `keys = names`. Do not send values as KEYS. Token `EX`/`EXAT` and TTL are ARGV after the values.
- Detect once per client by sending the real native command (no extra probe, no `INFO`). Cache under existing `mu`. Redis 7 and Dragonfly 1.40.2 take Lua; Redis 8.4+ / Valkey 9.1+ take native via the same detector.
- Spec host remains `std_go_simpleredis_resp-commands`. Fold new requirements there. EVALSHA stays forbidden. Do not change `MGet` empty-input `nil, nil`.
- Tests: fake native argv; fake unknown-command then cached EVAL; live Redis 7 + Dragonfly Lua with TTL landed (`SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY`, CI services already on `:6379` / `:6380`); Yaegi both paths against the compiled fake; Pester `/redis` and `/dragonfly` headers for the new verbs. No Valkey compose service.
- Past `EXAT`: do not hide integer `1`. Live: `MSetEXAt` in the past then `Get` is miss. Official MSETEX pages do not document past-delete; `EXPIREAT` does (`ext_redis_expire`). Prove on live engines, not from SET docs.
- Cluster: document hash tags only. No cluster client.

## Open questions

- Q: What pair-count cap rejects an unbounded frame before I/O?
  Rank: additive asked — Desired names a cap so one call cannot build an unbounded frame; the number is not on requirement.md
  Decision: assumed — 1024 pairs. Above the ticket’s 200-key flush example; small enough for the 1s `ioTimeout`. Constant next to `maxIdleConns`. Over cap → `redis:issue?`, no dial.
  By: propose

- Q: How is integer `0` spelled as a Go error?
  Rank: additive asked — Desired: integer 1 success; integer 0 must be returned to the caller (not swallowed)
  Decision: assumed — `redis:issue?`. `0` without NX/XX is an unexpected integer for this API. Do not add a sixth exported string. Do not use `errors.New("0")` (not in the stable set; Expire treats `:0` as success).
  By: propose

- Q: Do empty or nil slices error, or match dest `MGet` (`nil, nil`)?
  Rank: additive asked — Desired: reject empty input; Out of scope: changing `MGet` empty-input; Tensions say do not change `MGet`
  Decision: resolved — `MSetEX` / `MSetEXAt` reject empty or nil `names` (and mismatched `len(values)`) with `redis:issue?` before dial. `MGet` stays `nil, nil`.
  By: explore

- Q: What is the official `MSETEX` argv and reply?
  Rank: additive asked — Desired native path names `MSETEX <numkeys> k1 v1 … EX|EXAT <n>` and integer 1/0
  Decision: resolved — Valkey 9.1 and Redis 8.4: `MSETEX numkeys key value [key value …] [NX|XX] [EX|PX|EXAT|PXAT|KEEPTTL]`; integer `1` all set, `0` none set. This client always emits `EX` or `EXAT` after the pairs, no NX/XX. Sources: `knowledge/research/ext_valkey_msetex`, `knowledge/research/ext_redis_msetex`.
  By: explore

- Q: Do Redis and Dragonfly support native `MSETEX`?
  Rank: additive asked — Problem and Desired: Redis and Dragonfly have no native MSETEX; Lua is the default path; human confirmed both backends
  Decision: resolved — Compose/CI Redis 7 (`redis:7-alpine`) and Dragonfly v1.40.2 have no `MSETEX`. Redis 8.4+ and Valkey 9.1+ do. Live tests prove Lua+TTL on Redis 7 and Dragonfly. Detector still tries native first so 8.4+/Valkey take the native path without a new service. Sources: `knowledge/research/ext_redis_msetex`, `knowledge/research/ext_dragonfly_msetex`.
  By: explore

- Q: Re-detect native support after reconnect?
  Rank: additive asked — Desired: detect once per client, cache under existing mutex, cached miss goes straight to EVAL; Unknowns: re-detect after reconnect only if cheap
  Decision: assumed — once per `SimpleRedis` value. Cached miss stays miss for the client lifetime (Lua works on Valkey too). Cached native that later gets `ERR unknown command` recaches miss and `EVAL`s that call. Do not reset the flag on `dial`. A new `Init`/value is a new cache.
  By: propose

- Q: Where are native argv tests run?
  Rank: additive asked — Desired fake-server native argv; human: native argv tests stay on the fake
  Decision: resolved — fake only (`startFakeRedis` MSETEX case records argv, replies `:1`/`:0`). Do not add Valkey or Redis 8 to compose or CI services for native proof.
  By: explore

- Q: How does the Lua fallback declare keys for Dragonfly?
  Rank: additive asked — Desired: one EVAL script, loop SET EX/EXAT; Gotchas already require KEYS; human: Dragonfly KEYS required
  Decision: resolved — `Eval(script, names, argv)` with `numkeys = len(names)`. Values, then `EX` or `EXAT`, then the TTL decimal, in ARGV. Do not put values or the TTL key-token in KEYS. Do not use `allow-undeclared-keys`.
  By: explore

- Q: How is the Lua loop 5.1- and 5.4-safe?
  Rank: additive asked — Desired: no unpack/table.unpack; human: Lua 5.1-safe loop (no unpack/table.maxn)
  Decision: resolved — `for i = 1, #KEYS do redis.call('SET', KEYS[i], ARGV[i], token, ttl) end` with `token`/`ttl` from ARGV. No `unpack`, `table.unpack`, `table.maxn`. Source: `knowledge/research/ext_dragonfly_eval`.
  By: explore

- Q: How is Lua fallback proven live on Redis and Dragonfly with TTL landed?
  Rank: additive asked — Desired live e2e Redis and Dragonfly asserting TTL landed; human: both backends, TTL actually landed
  Decision: resolved — (1) `simpleredis/live_test.go` table-driven on `SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY`: `MSetEX` then `Get` plus `Eval` of `TTL` on a declared KEYS key, assert TTL > 0; past `MSetEXAt` then `Get` miss; skip on `-short` or unset addrs. CI `test` job already runs Redis 7 `:6379` and Dragonfly `:6380` — set both env vars (same pattern as windowcounter/tokenbucket). (2) Pester `/redis` and `/dragonfly` assert new headers including a positive TTL. Pester is Yaegi-under-Traefik, not a substitute for the Go live file.
  By: explore

- Q: Do we add a Valkey service to compose?
  Rank: additive incidental — Desired says Valkey native when available; human: native argv stays on the fake; compose already has Redis and Dragonfly
  Decision: resolved — no. Extend compose only if a probe label is missing (it is not). Keep `redis:7-alpine` and `dragonfly:v1.40.2`. Extend `e2e/simpleredisprobe` and Pester `/redis` `/dragonfly` with the new verbs.
  By: explore

- Q: Build EVALSHA / SCRIPT LOAD with this fallback (finding + perf-05)?
  Rank: additive incidental — finding suggested building them together; Out of scope names EVALSHA / SCRIPT LOAD / NOSCRIPT; dest spec forbids EVALSHA
  Decision: resolved — do not add a second script-load path. Fallback calls existing `Eval`. DestBranch now sends EVALSHA then EVAL on NOSCRIPT; MSetEX reuses that.
  By: implement

- Q: How is `ERR unknown command` matched?
  Rank: additive asked — Desired: treat ERR unknown command as not supported
  Decision: assumed — `strings.HasPrefix(err.Error(), "ERR unknown command")`. Covers Redis 7 quote styles (`ext_redis_msetex`). Do not match Lua `Error running script`. AUTH-class stays `redis:noauth` before this check.
  By: propose

- Q: One Lua script or two (EX vs EXAT)?
  Rank: additive asked — Desired: one EVAL script, EX vs EXAT variants
  Decision: assumed — one script. Last two ARGV are the token (`EX` or `EXAT`) and the decimal TTL. One body to special-case in the fake. Two Go wrappers share it.
  By: propose

- Q: How does the fake distinguish native MSETEX from unknown-command fallback?
  Rank: additive asked — Desired fake native argv and fake unknown-command then cached EVAL; dest fake replies +OK for unknown including MSETEX
  Decision: assumed — add `case "MSETEX"` that stores pairs, records argv, replies `:1` (and deletes on past EXAT when tests need it). A test-only `rejectMSetEX` (or equivalent) replies `-ERR unknown command 'MSETEX'`. Do not change the default `+OK` for other unknown verbs. Fake EVAL special-cases the fallback script string and applies SET+store (or tests use `startStaticRedis` `:1` plus argv assert via a recording fake).
  By: propose

- Q: What error do mismatched lengths and over-cap use?
  Rank: additive asked — Desired: validate len(names)==len(values), reject empty, cap pair count, all before I/O
  Decision: assumed — `redis:issue?` for empty, length mismatch, and over-cap. Same matchable string as other unexpected-call/reply cases. No dial.
  By: propose

- Q: Cluster hash-slot constraint?
  Rank: additive asked — Desired: document clustered engines need all keys in one hash slot (hash tags)
  Decision: resolved — usage/spec note only. Client does not hash-tag, split, or retry cross-slot. Cross-slot `-` error is returned as `errors.New` of that text.
  By: explore

- Q: What does the Traefik probe and Pester assert for the new verb?
  Rank: additive asked — Affected `e2e/simpleredisprobe` and `scripts/integration-tests.Tests.ps1`; human: extend compose + Pester `/redis` `/dragonfly` and the probe with the new verb
  Decision: assumed — `ServeHTTP` calls `MSetEX` and `MSetEXAt` on per-request keys, then `Eval` `return redis.call('TTL', KEYS[1])` with that key in KEYS. Headers `X-SimpleRedis-MSetEX` (value ok) and `X-SimpleRedis-MSetEX-TTL` (positive decimal). Same on `/redis` and `/dragonfly`. Do not stop whoami-a/b. Compose services unchanged.
  By: propose

- Q: How is Yaegi coverage provided for both paths?
  Rank: additive asked — Desired: Yaegi coverage for both paths; human same
  Decision: resolved — two interpreted tests against the compiled fake (existing GOPATH / stdlib / `useunsafe` false, no Traefik): (1) fake implements MSETEX, probe `MSetEX` succeeds (native); (2) fake rejects MSETEX, probe `MSetEX` succeeds via EVAL and a second call does not send `MSETEX`. Extend `clientprobeSrc`; keep existing Init/Get/Set/Del/Incr/Eval tests.
  By: explore

- Q: Are zero or negative TTL values rejected in Go?
  Rank: additive asked — Desired: require the TTL argument (do not omit; do not default KEEPTTL); dest `Set` already passes duration -1 through to the server
  Decision: assumed — pass the integer through, same as `Set`/`Expire`. Do not clamp. Do not omit the option. Server error text (or delete-on-past for EXAT) is the engine’s.
  By: propose

- Q: Where does the native-support flag live?
  Rank: additive asked — Desired: cache under the existing mutex; Current: mu guards pool/closed only
  Decision: resolved — one field on `SimpleRedis` (tri-state unknown / native / lua), read and written only while holding `groupWriteMu`. DestBranch has no generic `mu`; `idleConnsMu` is the unused-socket list. Do not put it on `pooledConn`. Existing borrow/release stay as they are.
  By: implement
