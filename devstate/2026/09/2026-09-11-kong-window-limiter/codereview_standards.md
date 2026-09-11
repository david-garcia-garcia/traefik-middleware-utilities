# Standards

1. [hard] Consume before produce — `ratelimit/fake_redis_test.go:13` — `testFakeRedis`/`startTestFakeRedis` reimplements the in-process RESP map fake that `simpleredis/simpleredis_test.go` already owns as `fakeRedis`/`startFakeRedis`
   ```
   // testFakeRedis is an in-process RESP server for limiter unit tests.
   type testFakeRedis struct {
   	mu         sync.Mutex
   	store      map[string]string
   	lastExpire []string
   }
   ```
   → Reuse or export the existing simpleredis fake (or one shared test helper) instead of a parallel copy
   Status: skipped
   Argument: design decision 9 — simpleredis fake is unexported `_test.go`; exporting it is outside this package. Not applied unattended.

2. [hard] Symmetry and consistency — `ratelimit/limiter.go:17` — production `flushScript` is the same Kong incrby+expireat Lua role named `kongIncrbyExpireatScript` in `simpleredis/simpleredis_test.go:191`, `simpleredis/yaegi_test.go:132`, and `e2e/simpleredisprobe/plugin.go:17`
   ```
   const flushScript = `local exists = redis.call("exists", KEYS[1])
   local value = redis.call("incrby", KEYS[1], ARGV[1])
   if exists == 0 then
     redis.call("expireat", KEYS[1], ARGV[2])
   end
   return value`
   ```
   → Use the same identifier (`kongIncrbyExpireatScript`) or one shared owner for the script across sibling packages
   Status: skipped
   Argument: limiter owns flushScript; simpleredis tests own a fixture. Sharing would invert package deps. Not applied unattended.

3. [hard] Leave a trail — `ratelimit/limiter.go:239` — `stopFlushLocked` comment is internal reasoning with questions, not the mutex handoff job
   ```
   // stopFlushLocked stops the ticker and waits for the goroutine. Caller holds l.mu until Stop, then waits outside? Wait on wg while holding mu deadlocks the loop if it needs mu. Unlock first.
   func (l *Limiter) stopFlushLocked() {
   ```
   → Replace with one succinct line: unlock before `wg.Wait` so `flushLoop` can finish without deadlocking
   Status: done
   Argument: `takeFlushTickerLocked` / `stopFlushAndWait` comments (`1979b40`).

4. [hard] Leave a trail — `ratelimit/limiter.go:102` — `Allow` comment restates the identifier instead of the alias job
   ```
   // Allow is Take.
   func (l *Limiter) Allow(key string, limit int64, window time.Duration) (bool, float64, error) {
   ```
   → Say it is an alias for callers who prefer Allow over Take
   Status: done
   Argument: Allow comment (`1979b40`).

5. [hard] Leave a trail — `ratelimit/fake_redis_test.go:101` — `testBulk`, `incrementTestStore`, and `readTestCommand` have no job comments; sibling fake helpers in simpleredis are commented (`bulk`, `incrementStored`, `readCommand`)
   ```
   func testBulk(store map[string]string, name string) string {
   func incrementTestStore(store map[string]string, name string, delta int64) (int64, error) {
   func readTestCommand(reader *bufio.Reader) ([]string, error) {
   ```
   → Add one-line job comments matching the simpleredis fake helper roles
   Status: done
   Argument: helper comments (`1979b40`).

6. [hard] Symmetry and consistency — `ratelimit/yaegi_test.go:49` — GOPATH interp helpers lack the job comments their simpleredis yaegi siblings carry (`evalClientprobe`, `writeGopathSimpleredis`, `writeGopathFile`)
   ```
   func evalTakeprobe(t *testing.T, goPath, expr string) string {
   func writeGopathLimiter(t *testing.T, goPath string) {
   func copyNonTestGo(t *testing.T, srcDir, goPath, pkg string) {
   func writeGopathFile(t *testing.T, goPath, pkg, name, src string) {
   ```
   → Mirror the simpleredis yaegi helper comments (eval, GOPATH copy, write probe file)
   Status: done
   Argument: yaegi helper comments (`1979b40`).

7. [hard] Leave a trail — `ratelimit/live_test.go:38` — `runLiveBackend` and `waitLiveClient` are new multi-block helpers with no job comment
   ```
   func runLiveBackend(t *testing.T, addr string) {
   ...
   func waitLiveClient(t *testing.T, addr string) *simpleredis.SimpleRedis {
   ```
   → One-line job comment on each (table-driven live scenarios; retry Incr until engine is up)
   Status: done
   Argument: live helper comments (`1979b40`).
