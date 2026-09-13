# Requirement
IssueKey: 2026-09-13-simpleredis-resilience-test-coverage

## Problem
A bug hunt over `simpleredis` probed roughly a dozen suspected production killers. Three were real and are owned by three other PRs. The rest came back healthy, but dest has no permanent tests for those healthy probes and no `simpleredis/BUGS.md`, so the same hunt can be repeated and the written record is not on dest.

## Current (code)
- `simpleredis/pool.go` `release` — publishes a reusable socket into `idleConns` then calls `freeInUseTurn`. `inUseTurns` is a `PoolSize`-buffered channel; `OverFrees()` lives on `simpleredis/simpleredis.go`. No `chaosFake`. No test drives ~24 goroutines against random delay / close-no-reply / `-LOADING` / truncated bulk.
- `simpleredis/pool_test.go` `TestConcurrentCommandsStayWithinPool`, `TestOverlappingCallersDoNotDialPastLiveCap`, `TestOverFreeAccountingStaysBalanced` — pool-size and turn accounting under honest fakes, not chaos.
- `simpleredis/commands_exec.go` `exec` — `do` then `release` without `defer` (comment: Yaegi panic loses the turn). A decoder panic is a permanent leak.
- `simpleredis/resp.go` `readReply` / `parseLen` — `clean == false` on dirty streams; `parseLen` rejects overflow before `length+2`. No `FuzzReadReply` / `FuzzParseLen`. `simpleredis/resp_test.go` `TestReadReplyUnsupportedAndMalformed`, `TestParseLen`, `TestReadReplyOverCapIsIssue` are table tests, not fuzz targets.
- `simpleredis/simpleredis_test.go` `TestCloseDuringInFlightCommandClosesSocketOnRelease` — one held Get, `Close`, idle 0, `waitOpenSocketsEqual(0)`. Does not run 200 New/use/Close cycles, does not assert `runtime.NumGoroutine()` flat, does not assert `len(inUseTurns) == cap(inUseTurns)` after N held Gets then Close.
- `simpleredis/fake_redis_test.go` — `holdGetsForTest` / `waitHeldGets` / `releaseHeldGetsForTest`, `setRejectMSetEX`, `openSockets`, `readCommand` (length-prefixed RESP bulk args).
- `simpleredis/yaegi_test.go` — interpreted happy paths only: `RoundTrip`, `IncrAndEval`, `EvalNoScript`, `MSetEXNative`, `MSetEXLua`, `MatchSentinels`. Helpers `writeGopathSimpleredis`, `writeGopathFile` exist. No interpreted dial-retry / stall timeout / cancel-mid-command / pool wait / truncated bulk.
- `simpleredis/commands_deadline_test.go` `startStallRedis` — stalling peer for compiled deadline tests; not wired into Yaegi.
- `simpleredis/commands_msetex.go` `cachedGroupWrite` / `storeGroupWrite` — `groupWriteMu` cache. `simpleredis/commands_msetex_test.go` `TestMSetEXUnknownCommandFallsBackAndCaches` is sequential (one client, two calls), not 16 goroutines × 25.
- `simpleredis/resp.go` `writeCommand` — every arg is `$len\r\n` + bytes + `\r\n`. `simpleredis/resp_test.go` `TestValueWithNewlinesSurvives` covers `\n` in a value, not `\r\n` plus an inline `*1\r\n$4\r\nPING\r\n` key/value against a strict RESP fake.
- `simpleredis/racedetector_on_test.go` / `racedetector_off_test.go` — `raceDetectorOn` const. No chaos/lifecycle iteration scaling yet.
- Dest `simpleredis/` has no `BUGS.md`. Reference is `origin/bugfixes20260913:simpleredis/BUGS.md` (Rejected table + previously-fixed table; three real bugs still presented as open on that branch).
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md`, `std_go_simpleredis_resp-decode/spec.md`, `std_go_simpleredis_resp-commands/spec.md` — no chaos / fuzz / injection / interpreted error-path coverage requirement. `openspec/specs/std_go_ci_test-suites/spec.md` — CI unit and race jobs pass `-short`.
- `simpleredis/pool.go` `handshakeFailure` wrapper is on dest (other ticket). Interpreted `IsUnreachable` / `errors.Is` on AUTH/SELECT handshake failure is broken; this ticket must not assert that.

## Desired
1. Permanent tests for the six healthy probes in the BUGS.md Rejected table. Prefer new `simpleredis/*_test.go` files. Do not change non-test `simpleredis/` files. If a probe fails, do not fix the product here — delivery card + `skill:opd-workflow:Issues`.
2. Chaos pool invariants: `chaosFake` per command randomly honest / delay 0–3ms / close no reply / `-LOADING Redis is loading the dataset in memory` / `$100\r\nshort` then close. ~24 goroutines, bounded duration, random ctx (Background, short timeout, or `time.AfterFunc` cancel). Keys k0..k63, `GET kN` → `vN`. Assert (a) no non-error Get returns another key's value; (b) at rest `len(inUseTurns) == cap(inUseTurns)`; (c) `OverFrees() == 0`; (d) after quiescence, server-side open sockets ≤ PoolSize. CODE COMMENT: `len(idleConns)` then `len(inUseTurns)` is not a live-socket metric (non-atomic reads; `release` publishes before `freeInUseTurn`). Do not assert peak live sockets.
3. `FuzzReadReply` over `readReply`: never panics; never returns values with `clean == false`. Seeds via `f.Add` as listed on the ticket (including `$18446744073709551616` and `*1\r\n$18446744073709551616`). `FuzzParseLen` for no `length+2` overflow. No committed testdata corpus. Bounded `-fuzztime` locally; CI keeps seed-corpus unit tests.
4. Lifecycle: (a) 200 New/use/Close cycles, `runtime.NumGoroutine()` flat, server-side open sockets 0. (b) Hold N Gets, Close, release, wait: idle 0, open sockets 0, turns full. Reuse `holdGetsForTest` / `waitHeldGets` / `releaseHeldGetsForTest`.
5. Yaegi error-path locks in a NEW file (e.g. `simpleredis/yaegi_errorpath_test.go`). Do not edit `yaegi_test.go`. Reuse `writeGopathSimpleredis`, `writeGopathFile`, `startStallRedis`. Cover interpreted: dial-failure retry + backoff jitter, I/O timeout vs stalling peer, cancel-mid-command, pool wait, truncated bulk. No interpreter panic; correct classification. Do not assert `IsUnreachable` / `errors.Is` on AUTH or SELECT handshake failures.
6. Concurrent MSetEX unknown-command fallback: 16 goroutines × 25 against `setRejectMSetEX`; 0 failures, turns full, `OverFrees() == 0`.
7. Command injection lock: key and value each containing `\r\n` and `*1\r\n$4\r\nPING\r\n` round-trip as data; a strict-RESP fake must not treat payload bytes as a command.
8. Gate long chaos and lifecycle stress behind `testing.Short()` so `-short` skips them. Default suite stays low seconds. Lower iteration counts when `raceDetectorOn`.
9. Land `simpleredis/BUGS.md` from `origin/bugfixes20260913:simpleredis/BUGS.md`, reframed: the three real bugs point at PRs `2026-09-13-simpleredis-handshake-sentinel-match`, `2026-09-13-simpleredis-lost-turn-recovery`, `2026-09-13-simpleredis-desync-boundary-check` rather than as open. Keep Rejected and Previously-reported tables. Add the locking test name on each Rejected row.
10. OpenSpec coverage requirement only if propose finds a spec gap. Verification later: `go vet ./simpleredis/`; `go test -count=1 -timeout 300s ./simpleredis/`; `go test -short -count=1 -timeout 300s ./simpleredis/`; `go test -count=1 -run XXX -fuzz FuzzReadReply -fuzztime 30s ./simpleredis/`.

## Affected
- New `simpleredis/*_test.go` (chaos, fuzz, lifecycle, yaegi error paths, concurrent MSetEX, injection)
- `simpleredis/BUGS.md` (new on dest)
- Possibly `openspec/specs/std_go_simpleredis_*` if a coverage requirement is added
- Existing test helpers reused, not rewritten: `fake_redis_test.go`, `yaegi_test.go` (read-only), `commands_deadline_test.go` `startStallRedis`, `racedetector_{on,off}_test.go`

## Out of scope
- Any change to `simpleredis/pool.go`, `commands_exec.go`, `resp.go`, `commands*.go`, `simpleredis.go`, `config.go`
- Editing `simpleredis/yaegi_test.go`
- Fixing handshake Yaegi sentinel matching, lost-turn recovery, or desync boundary (other PRs)
- Asserting `IsUnreachable` / `errors.Is` on AUTH/SELECT handshake failure under Yaegi
- Peak live-socket assertions
- Committed fuzz testdata corpus
- Committing `reclaim/BUGS.md`, `tokenbucket/BUGS.md`, `windowcounter/BUGS.md` (untracked in the caller workspace; human decides convention)
- Product source imports beyond stdlib, `unsafe`, cgo, generics

## Unknowns
- Exact names of new test files other than the suggested `simpleredis/yaegi_errorpath_test.go`.
- Whether this repo’s convention is that `BUGS.md` stays local; ticket says land `simpleredis/BUGS.md` and surface the other packages’ untracked files on the delivery card.
- Whether propose adds a spec coverage requirement or tests-only is enough.
- Chaos duration / iteration counts when `raceDetectorOn` (ticket: lower them so the race job does not time out; exact numbers not given).

## Tensions
- Dest still has `handshakeFailure` (`simpleredis/pool.go`). This PR must not assert interpreted handshake matching; that is the other agent’s bug and is currently red interpreted.
- Another agent is adding Yaegi tests in `yaegi_test.go`; this run uses a new file so the branches merge.
- Ticket: probes currently PASS (invariant locks). If implement’s probe fails on dest, do not fix product here.
- CI unit and race jobs pass `-short` (`std_go_ci_test-suites`), so Short-gated chaos/lifecycle skip there. Ticket still wants that gate so the default (no `-short`) suite stays fast. E2E jobs run without `-short`.
- Ticket owns `simpleredis/BUGS.md`; other agents were told not to touch it. Other packages’ `BUGS.md` are untracked local artifacts in the caller workspace — do not silently drop or commit those.
