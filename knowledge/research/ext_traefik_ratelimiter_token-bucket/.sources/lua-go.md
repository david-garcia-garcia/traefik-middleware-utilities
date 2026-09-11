---
url: https://github.com/traefik/traefik/blob/903e8a965795db5e750004ff74932983e269b85f/pkg/middlewares/ratelimiter/lua.go
title: pkg/middlewares/ratelimiter/lua.go
fetched: 2026-09-11
authority: source
ref: traefik/traefik@903e8a965795db5e750004ff74932983e269b85f:pkg/middlewares/ratelimiter/lua.go
---

AllowTokenBucketRaw EVAL script. KEYS[1] is the hash. ARGV: limit, burst, ttl, t, max_delay.
HGETALL; load last/tokens when table.maxn(rl_source)==4.
Refill limit*elapsed, cap burst, consume 1, wait if tokens<0, refund if wait>max_delay.
HSET last/tokens, EXPIRE ttl. Return {true, wait_duration, tokens} as strings.
Rediser includes Eval and EvalSha. LoadTokenBucketScript wraps redis.NewScript.
