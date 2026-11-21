# Nitpicks

1. [hard] Name for the scope — `windowcounter/limiter_test.go:581` — `TestTake_BufferedFlushThenKillFailsClosed` still names dest fail-closed; the edited body is `takeUntilLocalDeny` then `peekNilError` (nil error, local cap)

```go
func TestTake_BufferedFlushThenKillFailsClosed(t *testing.T) {
	// ...
	fake.Kill()
	takeUntilLocalDeny(t, limiter, "k", 1, limit, window)
	peekNilError(t, limiter, "k", limit, window)
}
```

   → `TestTake_BufferedFlushThenKillKeepsLocalCap` (or another identifier that names nil-error per-node cap)
   Status: done
   Argument: renamed to `TestTake_BufferedFlushThenKillKeepsLocalCap`.
2. [hard] Name for the scope — `windowcounter/limiter_test.go:515` — parameter `already` is an adjective; the body uses it as the hit count already taken toward `limit`

```go
func takeUntilLocalDeny(t *testing.T, limiter *Limiter, key string, already, limit int64, window time.Duration) {
	t.Helper()
	ctx := context.Background()
	for hit := already; hit < limit; hit++ {
		allowed, estimated, err := limiter.Take(ctx, key, limit, window)
		// ...
	}
```

   → `hitsTaken` (call sites pass `1` or `2`)
   Status: done
   Argument: renamed parameter to `hitsTaken`.
