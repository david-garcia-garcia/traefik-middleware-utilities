# Exact Take never retries EXPIRE after first-hit failure

issueHost: local. issueRef: none. Bound to this bug only.

## Problem

takeExact only Expire when Incr returns 1. If that Expire fails, key exists with count>=1 and no TTL. Later Takes Incr to 2+ and never Expire. Key lives forever.

## Agreed how

Exact Take uses one EVAL on KEYS[1]: INCR, then EXPIRE if PTTL<0 (no TTL), not only when increment is 1. Do not refresh TTL on every hit. Do not DEL on expire failure. Buffered flushScript unchanged. Test must prove a later Take still sets TTL (or atomic EVAL never leaves a no-TTL key).

## Implement order (required, do not implement in prepare)

1. CREATE the failing repro FIRST. Example: `windowcounter/repro_expire_not_retried_test.go`. Port fake helpers failNextExpireCommands / expireCommandCount from parent `windowcounter/fake_redis_test.go`. Confirm FAIL on unfixed code.
2. Then implement the EVAL. You may need the fake to support PTTL / EVAL expire-if-no-ttl for the test to still prove TTL is set.
3. Confirm that test PASSES and `go test -short -count=1 -timeout 60s ./windowcounter` passes. Keep TestTake_ExpireOnFirstHit.
