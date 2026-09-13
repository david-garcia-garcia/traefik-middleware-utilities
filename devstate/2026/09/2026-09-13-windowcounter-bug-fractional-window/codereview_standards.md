# Standards

1. [judgement] Leave a trail — `windowcounter/repro_fractional_window_test.go:35-118` — after the fix, `takeErr != nil` returns before subtest B, so B and `redisCountOrZero` never run in green CI but stay in the product tree as dest-fail diagnostics
   ```go
   	if takeErr != nil {
   		return
   	}

   	t.Run("B", func(t *testing.T) {
   ```
   → Drop subtest B and `redisCountOrZero` now that A locks rejection, or move A into `limiter_test.go` and remove the repro-only file
   Status: skipped
   Argument: judgement; explore resolved A as the post-fix lock and B as dest-fail diagnostic that returns when Take errors. Caller named this repro file and subtest A; keeping B is bound to that how. — `slidingAt` now rejects invalid windows first, but the method comment still describes only key/weight construction
   ```go
   // slidingAt builds the current and previous window keys and the previous-window weight.
   func (l *Limiter) slidingAt(key string, window time.Duration) (slidingWindow, error) {
   	// Reject sub-second and fractional-second windows; do not truncate into buckets.
   	if window < time.Second {
   ```
   → Extend the method comment to state whole-second validation and errors; keep the block intro for the key/weight block only
   Status: skipped
   Argument: judgement; block comment already names reject/do-not-truncate. Method comment still names the construction job. Not applied unattended.
