# Review

## prepare (2026-09-13)
phase: prepare
findings: qualified-with-gaps — spec says copied Traefik script; ticket changes Lua HSET last to max(previous, t)
fixed: bus, requirement, stub PR #55
skipped: product apply; other tokenbucket bugs

## explore (2026-09-13)
phase: explore
findings: dest FAIL sequential + goroutines_stale_samples_before_fresh_lock (last=1700000010000000); persist-max + lock-order clock; Lua HSET sibling
fixed: explore.md with Rank/Decision on three open questions
skipped: product apply

## propose (2026-09-13)
phase: propose
findings: FindSpecHost fold allow + lua-eval; change tokenbucket-last-not-rewind valid
fixed: proposal, specs, design, tasks
skipped: product apply

## implement (2026-09-13)
phase: implement
findings: persist-max consumeOne + Lua HSET; Memory now after lock; FAIL then PASS with Hour TTL
fixed: clock.go, lua.go, memory.go, repro test, usage gotcha
skipped: builtin max (Yaegi undefined)

