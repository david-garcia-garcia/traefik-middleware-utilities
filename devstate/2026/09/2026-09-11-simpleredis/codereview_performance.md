# Performance

1. [hard] Unbounded collection or cache — `simpleredis/simpleredis.go:186` — `borrow` calls `sr.dial()` whenever idle is empty; `release` applies `maxIdleConns` only after the command (`len(sr.idle) >= maxIdleConns`). Concurrent `Get`/`MGet`/`Set`/`Del` each open a socket; in-flight connections grow with concurrent callers, not a live key map
   → Wait or fail at `maxIdleConns` before dial so open sockets never exceed eight
   Status: skipped
   Argument: copied client idle cap is source; adding max-open changes SimpleRedis dial behavior. Bound the ask.
