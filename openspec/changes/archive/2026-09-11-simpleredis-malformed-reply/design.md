## Context

Dest already has `readReply` / `readLine` / `readBulk`, `startStaticRedis`, nested-array and garbage-integer tests, compose `redis:7-alpine` + `dragonfly:v1.40.2`, and Pester `/redis` `/dragonfly`. See proposal.md for why. Research: `knowledge/research/ext_redis_resp_null-array/`. Proceed policies: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- Prove decoder poison and retry-borrow failure in-process without live engines emitting junk.
- Keep `*-1` as `redis:issue?`; document it at the `count < 0` check.
- Extend live Get-miss on both engines beside fake `*-1`.

**Non-Goals:**
- Mapping `*-1` to `errMiss`.
- Yaegi table for malformed rows.
- Renaming compose project `reclaim-e2e`.
- Driving malformed bytes from Redis or Dragonfly.

## Decisions

1. **Keep `*-1` as protocol violation.** At `readReply` `count < 0`, stay `errIssue` `clean=false`. Comment: `*-1` is a legal nil array (BLPOP timeout, EXEC abort); this client has no verb that receives it, so it is not `redis:miss`. Alternative: map to `errMiss` like `$-1` — rejected; a null collection is not a missing GET key (`windowcounter` `getCount` treats miss as zero).

2. **Complete junk via `startStaticRedis`; truncated via write-then-close.** One table: HTTP-shaped, `?huh`, `:42\n`, `\r\n`, `*abc`, `*-1`, `*1\r\n?bad\r\n`, nested `*1\r\n*0\r\n`. Truncated array/bulk: helper that reads one command, writes a partial reply, closes — `startStaticRedis` keeps the socket open and those rows would hit 1s `ioTimeout`. Assert error **and** `len(idle)==0` on every dirty row. Fold `TestEvalNestedArrayIsIssue` into the table. Alternative: one helper for all rows — rejected; open-socket replay cannot produce EOF.

3. **Arity is a second table without `idle==0`.** Canned `*0` and `*2` into Get and Incr. Assert `redis:issue?`. `TestIncrGarbageIntegerPayload` stays (clean `:` line, not `len!=1`). Alternative: destroy the socket on arity mismatch — rejected; that adds a hygiene branch to verbs whose job is not pooling.

4. **Retry-borrow is a dedicated one-accept helper.** Accept once; first command replies `$1\r\nt\r\n` so Get pools; close the listener; second command on that conn replies malformed; second Get reuses, dirties, retries `borrow`, dial fails → `redis:unreachable` and `idle==0`. Do not close the accepted conn (test-01 EOF redial). Alternative: `startStaticRedis` row — rejected; it Accepts again so retry never fails.

5. **Live Get-miss on both engines.** Probe Gets a never-set key, matches `simpleredis.RedisMiss`, sets `X-SimpleRedis-GetMiss` to `redis:miss`. Pester asserts that header on `/redis` and `/dragonfly`. Keep every existing verb header. Eval stays `kongIncrbyExpireatScript`. Yaegi stays compiled-fake happy-path. Alternative: fake-server only for miss — rejected; a decoder change must still pass both live Its.

## Risks / Trade-offs

- [Truncated rows vs 1s timeout] → Mitigation: write-then-close; do not reuse `startStaticRedis` for those rows.
- [Arity `idle==0` vs Desired wording] → Mitigation: deviation on explore; spec asserts pool retain on clean decode.
- [`*-1` vs official null object] → Mitigation: no BLPOP/MULTI/EXEC; comment at `count < 0`; live `$-1` stays proven as miss.

## Migration Plan

Tests and one comment. Rollback is revert. No production API change.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
