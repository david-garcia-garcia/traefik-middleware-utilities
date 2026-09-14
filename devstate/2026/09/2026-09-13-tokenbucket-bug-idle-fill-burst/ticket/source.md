# New key fills to burst then consume 1 (idle fill burst)

You run a FULL OpenDev workflow (prepare → explore → propose → implement → codereview → devdocsimpact → archive → pullrequest, deliver review after each) for ONE tokenbucket bug. Unattended. Do not stop after prepare to ask permission. Do not stall on OpenSpec chat approval.

Identifiers:
- IssueKey / JobName: 2026-09-13-tokenbucket-bug-idle-fill-burst
- issueHost: local  issueRef: none
- prHost: github  destBranch: master  repoSlug: traefik-middleware-utilities
- Caller spec = this prompt.

THIS BUG ONLY. You WILL change the copied Traefik Lua (empty hash → full burst) on purpose; keep the MIT notice. Do not import x/time/rate. Do not also fix wait mapping, NaN rate, ttl, or last rewind unless required for this fill.

## Problem
Spec: new key filled to burst then consume 1. Implementation uses tokens=0 last=0 and refill from elapsed since Unix epoch. Fails at Unix(0,0) and when burst > elapsed*rate (e.g. burst 1e12).

## Agreed how
Missing state is a full bucket, then consume 1. Both stores.
- Lua: empty HGETALL (#rl_source ~= 4) sets tokens = burst and last = t (elapsed 0), then existing refill/consume. Do not rely on last=0 + elapsed-from-epoch.
- Memory: a new memEntry (including after TTL delete) starts tokens = burst, last = nowMicro, not zeros. Same formulas after that.
- Do not fill Memory only — Redis must match.

Fake Redis in tokenbucket/fake_redis_test.go uses consumeOne, not Lua. If you change consumeOne for new last=0 entries, also seed Memory new entries AND Lua empty hash. Agreement tests must still pass. You may need the fake to treat missing hash like Lua (tokens=burst, last=now) OR Memory never calls consumeOne with zeros — prefer Memory starting full so consumeOne stays the consume math.

Source: d:\repositories\traefik-middleware-utilities\tokenbucket\BUGS.md item 5.

## Tests first (hard)
CREATE tests, confirm FAIL, then fix, then PASS.
Copy/adapt: d:\repositories\traefik-middleware-utilities\tokenbucket\repro_epoch_idle_burst_test.go — TestRepro_NewKeyFillsToBurstAtEpoch (epoch_clock burst 5; huge_burst_elapsed_below_burst burst 1e12).
Also prove Redis/fake or document that Lua empty hash matches (Eval encoding / agreement if you can without live engines; -short).
