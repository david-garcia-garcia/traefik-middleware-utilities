## Context

Dest already maps a short bulk read to `clean == false`, closes on `reusable == false`, and does not append that socket to idle. `startStaticRedis` writes one complete canned RESP per command. Compose already has `redis:7-alpine` and `dragonfly:v1.40.2` plus `/redis` `/dragonfly`. Probe Sets `"ok"` on a per-request key, so live own-value is vacuous. Research: `knowledge/research/ext_redis_resp_bulk-string/`. Proceed policies: `devstate/explore.md`. See proposal.md for why.

## Goals / Non-Goals

**Goals:**
- Same-package unit tests that a truncated or protocol-garbage reply is not idle-pooled, with the existing error strings.
- Live Get/MGet on Redis and Dragonfly return each caller’s unique token.
- Keep dest Eval script (Lua 5.1, `KEYS` declared).

**Non-Goals:**
- A compose proxy or extra whoami that injects `$100` then 40 bytes.
- New compose services, routes, or image pins.
- Production `readBulk` / `do` / `release` rewrite when dest already matches.
- `exec` retry after a lost reply (test-05).
- Covering array-element short-read unless the same helper makes it free.

## Decisions

1. **Raw-reply test helper, not `startStaticRedis`.** Add a same-package helper in `simpleredis_test.go` that, per Accept, reads one command, writes caller bytes, and may close without the final CRLF. First Accept: `$100\r\n` + 40 bytes + close. Later Accepts: a complete bulk so the second Get can prove recovery. Name it for that job (`startRawReplyRedis` or equivalent). Tests only. Alternative: change `startStaticRedis` — rejected; Desired keeps it.

2. **Truncate/`clean` is unit-only; live engines prove own-value.** Official RESP is complete values. Do not add a live injection seam. Pester on existing `/redis` and `/dragonfly` only. Alternative: compose toxiproxy — out of scope.

3. **Idle empty after the failed call is load-bearing.** `exec` retries a dead reused socket, so a second Get can succeed even if the dirty conn was pooled. Assert `len(idle)==0` after the failed call; the second Get proves the client still dials a clean socket. Alternative: second Get alone — insufficient.

4. **Probe payload is the per-request prefix.** Set that token (not `"ok"`). Pester: Value equals that token, MGet equals Value; two overlapping GETs per route with distinct tokens. Other verb headers stay. Alternative: extra isolation route — rejected; Desired names `/redis` and `/dragonfly`.

5. **Keep dest `kongIncrbyExpireatScript`.** No new Lua. Dragonfly requires declared `KEYS`; no `table.maxn`. Research: `ext_redis_eval`, `ext_dragonfly_eval`.

6. **Do not edit `simpleredis.go` unless a committed test fails.** Throwaway truncated Get on dest already returned `redis:unreachable`, idle 0, second Get `hello`. Cover `readBulk` non-`$` head with a same-package call; do not widen `readReply`. Coverage proof is the invariant tests plus a measured non-zero `ReadFull`-fail block after implement, not a hardcoded dest block id.

## Risks / Trade-offs

- [Second Get can pass while a dirty conn was pooled] → Mitigation: assert idle empty after the failed call (decision 3).
- [Live Redis/Dragonfly cannot truncate a bulk] → Mitigation: unit fake only (decision 2).
- [Constant `"ok"` hides a swapped key] → Mitigation: unique Set payload (decision 4).
- [Coverage block ids move if `readBulk` is edited] → Mitigation: invariant tests; re-measure the fail block after implement.

## Migration Plan

Test-only plus probe/Pester assertion change. Rollback is revert. No production deploy.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
