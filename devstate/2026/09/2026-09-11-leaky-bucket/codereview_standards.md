# Standards

1. [judgement] Duplicated Code — `leakybucket/redis.go:105` — `evalPour` and `flushPending` both build the same `pourScript` EVAL (KEYS plus leak, capacity, ttl, now, delta); they differ only by `poured` vs `state.localPours`. Miss one site and ARGV order drifts from Lua.
   → Extract one helper `(key, poured, nowMicro)` that Eval+parseEvalReply; call it from `evalPour` and from the flush loop
   Status: skipped
   Argument: judgement; ARGV order already matches Lua at both sites; unattended does not extract a helper.
