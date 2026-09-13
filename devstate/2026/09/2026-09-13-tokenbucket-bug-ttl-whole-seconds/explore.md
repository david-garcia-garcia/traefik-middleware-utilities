# Explore
IssueKey: 2026-09-13-tokenbucket-bug-ttl-whole-seconds

## Concepts

**ttl (constructor)**: `time.Duration` on `NewMemory` / `NewRedis`. One `validateClock` owns rate, burst, maxDelay, and ttl. Dest rejects only `ttl < time.Second`.

**ttlSeconds**: Redis EXPIRE argument. `int64(ttl / time.Second)`. Traefik Lua `redis.call('expire', key, ttl)` takes integer seconds (`knowledge/research/ext_redis_expire/`, `knowledge/research/ext_traefik_ratelimiter_token-bucket/`).

**Memory expireAt**: `now.Add(m.clock.ttl)` — the full Duration. Lazy drop when `!now.Before(entry.expireAt)`.

**errTTL**: sentinel `errors.New("tokenbucket: ttl must be at least 1s")`. Ticket keeps the sentinel; text may say whole seconds.

**Stores agree**: spec `std_go_tokenbucket_lua-eval` same sequence for the same ttl. Dest 1500ms is not a representable same ttl.

```
  New(ttl=1500ms)  dest: ok
       │
       ├─ Memory  expireAt = now + 1.5s   still live at +1200ms
       └─ Redis   ARGV ttl = "1"          EXPIRE 1s
```

## Decisions

- Redis integer-second EXPIRE is the owner of lifetime, not Memory's Duration. Do not PEXPIRE. Do not rewrite Lua.
- Reject unrepresentable ttl at `validateClock` so both stores never see it. Same `errTTL`. Still `>= 1s`.
- Tests first: dest-failing proof that New accepts 1500ms, Redis ARGV is `"1"`, Memory not expired at +1200ms; then reshape that test so New(1500ms) is `errTTL` and 2s still constructs.
- Usage packet `knowledge/devdocs/std_go_tokenbucket.md` currently documents `ttl < 1s` only — that is true on dest; after apply the gotcha must say whole seconds. Spec `std_go_tokenbucket_allow` construction line likewise. No new packet; fold into those hosts.

## Open questions

- Q: What exact `errTTL` string after the whole-second gate?
  Rank: additive asked — Desired 3 names the same `errTTL` sentinel and allows the text to say whole seconds
  Decision: assumed — keep sentinel `errTTL`; set the text to `tokenbucket: ttl must be a whole number of seconds (at least 1s)`. One check: `ttl < time.Second || ttl%time.Second != 0`.
  By: explore

- Q: Where does the adapted repro live after the reshape?
  Rank: additive asked — Desired 2 names copy/adapt of `repro_ttl_truncation_test.go`; usage packet `std_go_test-suites` uses `{domain}_test.go`
  Decision: assumed — land `tokenbucket/ttl_truncation_test.go` (drop `repro_` once it is the contract). Adapt `Allow` to dest `(ctx, key) (bool, time.Duration, error)`. After the fix, assert `errors.Is(err, errTTL)` for 1500ms on both constructors; keep 2s accepted. Do not keep ARGV=`1` / Memory-at-+1200ms as the post-fix contract.
  By: explore

- Q: Is NewRedis(1500ms) a dedicated case or only Memory plus shared `validateClock`?
  Rank: additive asked — Desired 1 names both NewMemory and NewRedis
  Decision: assumed — both constructors call `validateClock`; the test calls both NewMemory and NewRedis with 1500ms and `errors.Is(..., errTTL)`. 2s still succeeds on both. Dest ARGV/`+1200ms` proof exists only in the failing-first version of that test.
  By: explore
