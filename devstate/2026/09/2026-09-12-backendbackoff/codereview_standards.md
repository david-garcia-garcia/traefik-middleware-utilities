# Standards

1. [hard] Name for the scope — `backendbackoff/limiter_yaegi_test.go:1` — file name is the tokenbucket/windowcounter limiter nickname; this package’s type is `Gate`
   ```
   package backendbackoff
   ...
   func TestYaegi_AllowThenReportTrips(t *testing.T) {
   ```
   → Rename to `gate_yaegi_test.go` or `yaegi_test.go` (reclaim/simpleredis)
   Status: done
   Argument: renamed to gate_yaegi_test.go.
2. [hard] Leave a trail — `backendbackoff/gate.go:43` — new types `gateState` and `memEntry`, and unexported `resolveConfig` / `successCredit` / `cooldownDuration`, have no job comment; `cooldownDuration` has no block intros (double-until-cap vs jitter). Tokenbucket’s `memEntry` has a job comment.
   ```
   type gateState int
   ...
   type memEntry struct {
   	credit           float64
   	n                int
   	state            gateState
   	...
   }
   func resolveConfig(cfg Config) (Config, error) { ... }
   func (g *Gate) successCredit() float64 { ... }
   func (g *Gate) cooldownDuration(n int) time.Duration { ... }
   ```
   → One-line job comment on each type and func; intro the cap loop and the jitter block
   Status: done
   Argument: job comments on gateState, memEntry, resolveConfig, successCredit, cooldownDuration.
3. [hard] Leave a trail — `backendbackoff/allow.go:8` — `Allow` and `Report` are multi-block (canceled ctx, closed, TTL, state switch) with no block intros; `loadEntry`, `resetNIfClosedLongEnough`, `dropExpired`, `dropOne` have no job comment. Tokenbucket’s copied `dropExpired` / `dropOne` and the idle-TTL block in `Allow` are commented.
   ```
   // Allow returns whether a backend attempt for key may proceed, plus how long to wait if not.
   func (g *Gate) Allow(ctx context.Context, key string) (bool, time.Duration, error) {
   	if err := ctx.Err(); err != nil {
   		return false, 0, err
   	}
   	...
   	switch entry.state { ... }
   }
   func (g *Gate) loadEntry(key string, now time.Time) *memEntry { ... }
   func (g *Gate) dropExpired(now time.Time) { ... }
   func (g *Gate) dropOne(now time.Time) { ... }
   ```
   → Job comment on each helper; intro Allow (ctx, closed, TTL refresh, state) and Report (missing/expired, OPEN ignore, probe, CLOSED credit)
   Status: done
   Argument: comments on Allow TTL refresh, Report expire, loadEntry, resetN, dropExpired, dropOne.
4. [hard] Leave a trail — `backendbackoff/limiter_yaegi_test.go:25` — Yaegi GOPATH helpers have no job comment; the tokenbucket siblings they copy (`evalAllowprobe`, `writeGopathTokenbucket`, `copyNonTestGo`, `callerDir`, `writeGopathFile`) each have one
   ```
   func evalTripprobe(t *testing.T, goPath, expr string) string { ... }
   func writeGopathBackendbackoff(t *testing.T, goPath string) { ... }
   func copyNonTestGo(t *testing.T, srcDir, goPath, pkg string) { ... }
   func callerDir(t *testing.T) string { ... }
   func writeGopathFile(t *testing.T, goPath, pkg, name, src string) { ... }
   ```
   → Copy the sibling one-liners (interp stdlib-only; copy non-test sources; caller dir; write probe file)
   Status: done
   Argument: one-liners on evalTripprobe and GOPATH helpers.
5. [hard] Leave a trail — `backendbackoff/bench_test.go:9` — `BenchmarkAllowWarm`, `skipAllocCeilingIfRace`, `allocExceedsCeiling`, `assertAllocCeiling` have no job comment (simpleredis bench helpers do); `gate_test.go` helpers `newTestGate` and `trip` have none
   ```
   func BenchmarkAllowWarm(b *testing.B) { ... }
   func skipAllocCeilingIfRace(t *testing.T) { ... }
   func allocExceedsCeiling(...) (allocsOver, bytesOver bool) { ... }
   func assertAllocCeiling(...) { ... }
   func newTestGate(t *testing.T) *Gate { ... }
   func trip(t *testing.T, gate *Gate) { ... }
   ```
   → One-line job comment on each (warm CLOSED Allow; skip under race; ceiling compare; jitter-off gate; B consecutive failure Reports)
   Status: done
   Argument: comments on BenchmarkAllowWarm, alloc helpers, newTestGate, trip.
