# Standards

1. [hard] Leave a trail — `tokenbucket/traefik_issue12091_test.go:39` — `newRedisLimiterForTest12091` comment restates the identifier
   Fix: Name the job: Redis limiter on this refusing client with the 12091 clock (rate 1, burst 3, maxDelay 0, ttl 2s)
   Quote:
      ```go
      // newRedisLimiterForTest12091 returns a Redis limiter on client.
      ```
   Status: done
   Argument: Comment on `newRedisLimiterForTest12091` now names the refusing client and 12091 clock (rate 1, burst 3, maxDelay 0, ttl 2s).
