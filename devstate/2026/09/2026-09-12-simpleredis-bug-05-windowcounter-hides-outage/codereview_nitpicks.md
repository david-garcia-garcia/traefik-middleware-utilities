# Nitpicks

1. [hard] Clear conditions — `windowcounter/limiter_test.go:507` — `if msg != RedisUnreachable && msg != RedisTimeout && !strings.Contains(msg, RedisUnreachable) && !strings.Contains(msg, RedisTimeout)` wraps `Fatalf`; the helper’s accepted cases are Unreachable or Timeout (exact or substring)
   → Early-return when `msg == Unreachable || msg == Timeout || strings.Contains(msg, Unreachable) || strings.Contains(msg, Timeout)` (or `isRedisOutage(err)`); then `t.Fatalf(...)`
   Status: done
   Argument: wantRedisOutage early-returns on Unreachable/Timeout membership via isRedisOutageMessage.
