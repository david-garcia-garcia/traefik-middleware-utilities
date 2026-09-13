# Tokenbucket Memory vs Redis TTL: fractional Duration accepted; stores disagree

THIS BUG ONLY.

## Problem
Memory expires at now.Add(ttl) (full Duration). Redis ARGV is int64(ttl / time.Second). ttl=1500ms is legal at New; Redis EXPIRE 1; Memory still holds until 1.5s. Stores disagree. Memory is not the source of truth — Redis is Traefik integer-second EXPIRE. New accepted a Duration Redis cannot represent.

## Agreed how
validateClock requires ttl to be a whole number of seconds (still >= 1s). Same errTTL sentinel; error text can say whole seconds. Then Memory now.Add(ttl) and Redis EXPIRE ttlSeconds are the same lifetime.
Do not PEXPIRE. Do not rewrite Lua. Do not silently floor Memory to 1s while New(1500ms) succeeds.

Source: d:\repositories\traefik-middleware-utilities\tokenbucket\BUGS.md item 4.

## Tests first (hard)
CREATE tests, confirm FAIL on dest (NewMemory/NewRedis accept 1500ms; Redis ARGV ttl is 1; Memory not expired at +1200ms), then fix so New(1500ms) returns errTTL, then PASS.
Copy/adapt: d:\repositories\traefik-middleware-utilities\tokenbucket\repro_ttl_truncation_test.go — after the fix the test must assert New rejects 1500ms (reshape the repro from “Memory vs Redis disagree” to “fractional ttl rejected”). Also keep a whole-second ttl still accepted (2s).
