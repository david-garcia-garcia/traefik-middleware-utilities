## 1. Chaos and lifecycle

- [ ] 1.1 Add `simpleredis/chaos_pool_test.go` with `chaosFake` (honest / delay 0–3ms / close no reply / LOADING / truncated bulk) and `TestChaosPoolInvariants` (24 goroutines, 1s, keys k0..k63, random ctx). Assert no cross-key Get, full turns, OverFrees 0, settled open sockets ≤ PoolSize. Comment why idle+in-use is not a live-socket metric. Skip on `testing.Short()`. Lower to 8 goroutines × 250ms when `raceDetectorOn`
- [ ] 1.2 Add `simpleredis/lifecycle_test.go`: 200 New/use/Close cycles (40 when `raceDetectorOn`) with flat `runtime.NumGoroutine()` and open sockets 0; Hold N Gets, Close, release, wait — idle 0, open 0, turns full. Skip on `testing.Short()`. Reuse `holdGetsForTest` / `waitHeldGets` / `releaseHeldGetsForTest`

## 2. Decoder fuzz and injection

- [ ] 2.1 Add `simpleredis/resp_fuzz_test.go` `FuzzReadReply` and `FuzzParseLen` with the listed `f.Add` seeds. No testdata corpus
- [ ] 2.2 Add `simpleredis/resp_injection_test.go` `TestWriteCommandDoesNotInjectCommands`: key and value each contain CRLF plus inline PING; Set/Get round-trip; strict-RESP fake does not treat payload as a command

## 3. Concurrent MSetEX and Yaegi error paths

- [ ] 3.1 Add `simpleredis/commands_msetex_concurrent_test.go`: 16×25 MSetEX against `setRejectMSetEX` (4×8 when `raceDetectorOn`); 0 failures, turns full, OverFrees 0
- [ ] 3.2 Add `simpleredis/yaegi_errorpath_test.go`. Do not edit `yaegi_test.go`. Cover interpreted dial-retry+backoff, stall timeout via `startStallRedis`, cancel-mid-command, pool wait, truncated bulk. No interpreter panic. Do not assert AUTH/SELECT `IsUnreachable` / `errors.Is`

## 4. Written record and verification

- [ ] 4.1 Land `simpleredis/BUGS.md` from `origin/bugfixes20260913:simpleredis/BUGS.md`, reframed: three real bugs point at PRs `2026-09-13-simpleredis-handshake-sentinel-match`, `2026-09-13-simpleredis-lost-turn-recovery`, `2026-09-13-simpleredis-desync-boundary-check`. Keep Rejected and Previously-reported tables. Name the locking test on each Rejected row
- [ ] 4.2 Measure default-suite wall-clock before/after. Run `go vet ./simpleredis/`, `go test -count=1 -timeout 300s ./simpleredis/`, `go test -short -count=1 -timeout 300s ./simpleredis/`, and `go test -count=1 -run XXX -fuzz FuzzReadReply -fuzztime 30s ./simpleredis/`. Do not edit non-test `simpleredis/` files
