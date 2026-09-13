# Nitpicks

1. [hard] Symmetry and consistency — `simpleredis/pool_test.go:644` — the three new lost-turn tests name the `New` result `sr`; sibling tests in this file (including `TestPoolWaitTimesOutWithoutExtraDial`, whose holder/wait shape `TestRecoveryDoesNotFireWhileSocketsBusy` copies) name that same SimpleRedis role `redis`

```go
func TestBugLostInUseTurnBricksPoolPermanently(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	const poolSize = 2
	sr := New(Config{
		Host:        addr,
		PoolSize:    poolSize,
		PoolTimeout: 20 * time.Millisecond,
		MaxRetries:  -1,
	})
	if _, err := sr.Get(context.Background(), "hit"); err != nil {
		t.Fatalf("warm Get: %v", err)
	}
	// ...
}

func TestRecoveryDoesNotFireWhileSocketsBusy(t *testing.T) {
	// ...
	sr := New(Config{Host: addr, PoolSize: 2, PoolTimeout: 50 * time.Millisecond, IOTimeout: time.Second, MaxRetries: -1})
	// ...
}

func TestLostTurnsReportsLeak(t *testing.T) {
	_, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	sr := New(Config{Host: addr, PoolSize: 1, PoolTimeout: 20 * time.Millisecond, MaxRetries: -1})
	// ...
}
```

   → Rename `sr` to `redis` in those three tests (helpers may keep parameter `sr *SimpleRedis`)
   Status: done
   Argument: three new tests now name the client redis.
