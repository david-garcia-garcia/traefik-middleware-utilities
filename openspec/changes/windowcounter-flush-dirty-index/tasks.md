# Tasks

## 1. Indexes

- [ ] 1.1 Add `dirty` and `expiry` to `Limiter` (initialized in `New`) and the helpers `putWindowLocked`, `setExpireLocked`, `deleteWindowLocked` (design.md Decision 2). Replace every non-test insert into `l.windows` and every `expireAt` write with them. Verify `grep -n "expireAt =" windowcounter/limiter.go` shows only the helper and `go build ./windowcounter/`.
- [ ] 1.2 Maintain the dirty index in `takeBuffered` and `applyFlushOutcomeLocked`; rewrite collect and prune in `flushPendingLocked` per Decisions 1 and 3. Verify `go test -count=1 ./windowcounter/` passes all existing tests (migrate test helpers per Decision 4 first).

## 2. Tests and benchmarks

- [ ] 2.1 Add `checkIndexesLocked` and `windowcounter/zzz_flush_dirty_index_test.go` with cases (a)-(b) of Decision 5, helpers suffixed `FlushDirtyIndex`. Case (b) must fail on the old code (ratio ~20×). Verify `go test -count=1 -run FlushDirtyIndex ./windowcounter/` and `-race` on case (a).
- [ ] 2.2 Add `BenchmarkFlushIdle/{10k,200k}` compiled and a Yaegi equivalent in the existing bench files' style. Run both and record the numbers, plus warm Take compiled and Yaegi before/after, for the delivery card.

## 3. Local gate

- [ ] 3.1 `go vet ./...` and `go test -short -count=1 ./...`; then `go test -count=1 ./windowcounter/` with the live engine env vars (TOKENBUCKET_LIVE_REDIS / TOKENBUCKET_LIVE_DRAGONFLY or the package's own) if available. Verify all exit 0.
