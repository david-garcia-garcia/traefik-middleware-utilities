# Standards

1. [judgement] Duplicated Code — `simpleredis/simpleredis_e2e_test.go:12-45`, `tokenbucket/limiter_e2e_test.go:11-45`, `windowcounter/limiter_e2e_test.go:12-46` — the same `liveEngineForTest`, `liveEngineAddrs`, and `runForEachLiveEngine` block is pasted verbatim into three packages; any change to skip/fail or engine table-drive policy must be edited three times
   ```go
   type liveEngineForTest struct { name string; addr string }
   func liveEngineAddrs(t *testing.T, redisEnv, dragonflyEnv string) []liveEngineForTest { ... }
   func runForEachLiveEngine(t *testing.T, redisEnv, dragonflyEnv string, run func(t *testing.T, addr string)) { ... }
   ```
   → Extract one shared owner (for example `internal/livee2e`) and call it from each `*_e2e_test.go`
   Status: skipped
   Argument: judgement; each package owns its LIVE env names. Unattended will not add `internal/livee2e`.
