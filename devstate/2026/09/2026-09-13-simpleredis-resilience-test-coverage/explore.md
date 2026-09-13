# Explore
IssueKey: 2026-09-13-simpleredis-resilience-test-coverage

## Concepts

This ticket does not change SimpleRedis behaviour. Dest already has the pool, decoder, and command paths. The gap is proof: the 2026-09-13 hunt measured a dozen suspected production killers; three real bugs are owned by other PRs; the healthy probes exist only as prose on `origin/bugfixes20260913:simpleredis/BUGS.md`. Dest has no `simpleredis/BUGS.md` and no tests that lock those healthy results.

```
hunt (scratch, not in dest)
        │
        ├── real (other PRs) ── handshake sentinel match
        │                     ── lost-turn recovery
        │                     ── desync boundary check
        └── healthy ── this PR: permanent tests + written record
```

Same-package tests can see `inUseTurns`, `OverFrees()`, and fake helpers (`holdGetsForTest`, `waitHeldGets`, `releaseHeldGetsForTest`, `setRejectMSetEX`, `openSockets`, `waitOpenSocketsEqual`). `writeGopathSimpleredis` / `writeGopathFile` live in `yaegi_test.go`; `startStallRedis` lives in `commands_deadline_test.go`. A new file in package `simpleredis` can call them without editing those files.

`readCommand` on the fake already parses strict RESP bulk args (`$len` then `ReadFull` of `length+2`). That is the injection oracle: a key/value containing `\r\n` and `*1\r\n$4\r\nPING\r\n` must round-trip as one GET/SET argument, not as a second command.

Live-socket sampling pitfall (encode as a code comment, not an assertion): `len(idleConns)` then `len(inUseTurns)` is not atomic, and `release` publishes into `idleConns` before `freeInUseTurn`, so a releasing socket is counted in both. Server-side peak `open` over-counts because the serve goroutine exits after the client's `Close`. Only at-rest values are sound.

CI `Unit` and `Unit race` pass `-short` (`std_go_ci_test-suites`). Short-gated chaos/lifecycle therefore skip those jobs. They still run on a default `go test ./simpleredis/` and on Go E2E (no `-short`). Seed-corpus `Fuzz*` functions run as ordinary tests in CI; a local `-fuzztime` is verification, not a CI job.

Usage packets `knowledge/devdocs/std_go_simpleredis.md` and `std_go_simpleredis_resp-decode.md` are enough to call the subsystem. Research `ext_redis_resp_bulk-string` already states length-prefixed bulk form. No new research folder. No identity reconstruction (no client address / tenant / Host / trust hop).

Existing specs require interpreted happy paths and compiled truncated-bulk / timeout / Close-during-inflight. They do not require chaos, decoder fuzz, interpreted error paths, concurrent MSetEX fallback, or CRLF injection as data.

## Decisions

- New test files only. Do not edit `yaegi_test.go`, `fake_redis_test.go`, or any non-test `simpleredis/` source. `chaosFake` lives in the chaos test file.
- Track `simpleredis/BUGS.md` on dest, reframed so the three real bugs point at the other IssueKeys rather than as open. Keep both tables. Put the locking test name on each Rejected row. Do not stage `reclaim/BUGS.md`, `tokenbucket/BUGS.md`, or `windowcounter/BUGS.md`.
- Propose adds coverage requirements onto the three existing spec leaves (`std_go_simpleredis_tcp-session`, `std_go_simpleredis_resp-decode`, `std_go_simpleredis_resp-commands`). No new spec family.
- Short-gate only the long chaos and lifecycle stress. Fuzz seeds, injection, concurrent MSetEX, and Yaegi error paths stay in the default/`-short` unit suite so CI actually runs them. Lower those iteration counts when `raceDetectorOn`.
- If a probe fails on dest, do not patch product source. Record `skill:opd-workflow:Issues` and put the measurement on the card.

## Open questions

- Q: What are the new test file and function names?
  Rank: additive asked — new files this change creates; Desired 1 and 5 name a new Yaegi file and prefer new `*_test.go`
  Decision: assumed — `simpleredis/chaos_pool_test.go` (`TestChaosPoolInvariants`), `simpleredis/resp_fuzz_test.go` (`FuzzReadReply`, `FuzzParseLen`), `simpleredis/lifecycle_test.go` (`TestLifecycleNewUseCloseDoesNotLeak`, `TestCloseDuringHeldGetsReturnsTurns`), `simpleredis/yaegi_errorpath_test.go` (dial-retry, stall timeout, cancel-mid-command, pool wait, truncated bulk), `simpleredis/commands_msetex_concurrent_test.go` (`TestConcurrentMSetEXUnknownCommandFallback`), `simpleredis/resp_injection_test.go` (`TestWriteCommandDoesNotInjectCommands`).
  By: explore

- Q: Is `BUGS.md` a tracked package record or a local-only artifact?
  Rank: additive asked — Desired 9 names landing `simpleredis/BUGS.md`; other packages' files are Out of scope
  Decision: assumed — commit `simpleredis/BUGS.md` because this ticket owns that file and dest has none; leave `reclaim/`, `tokenbucket/`, and `windowcounter/` `BUGS.md` untracked; say so on the delivery card so the human can set a repo convention.
  By: explore

- Q: Does propose add an OpenSpec coverage requirement, or are tests-only enough?
  Rank: additive asked — Desired 10; existing specs have no chaos / fuzz / injection / interpreted error-path requirement
  Decision: assumed — add coverage requirements on the three existing SimpleRedis spec leaves (tcp-session: chaos + lifecycle; resp-decode: fuzz; resp-commands: injection, concurrent MSetEX fallback, interpreted error paths). Do not create a new capability folder.
  By: explore

- Q: What chaos duration and lifecycle iteration counts keep the default suite in the low seconds and keep `go test -race` from timing out?
  Rank: additive asked — Desired 8; exact numbers not given
  Decision: assumed — `testing.Short()` skips chaos and lifecycle. Default: 24 goroutines × 1s chaos (PoolSize 4, keys k0..k63); lifecycle 200 New/use/Close cycles. When `raceDetectorOn`: chaos 8 goroutines × 250ms; lifecycle 40 cycles; concurrent MSetEX 4 goroutines × 8 instead of 16×25. Quiesce with `waitOpenSocketsEqual` / a short settle sleep before at-rest asserts. No peak-socket assertion.
  By: explore

- Q: Who already owns client identity (address, user, tenant, Host, trust hop)?
  Rank: additive asked — explore rule when work would reconstruct identity; this change does not
  Decision: assumed — none; this PR adds tests and a hunt record on a stdlib Redis client and does not set or reconstruct identity. Reuse nothing.
  By: explore
