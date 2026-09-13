## Why

Dest `simpleredis` already survived a production-killer hunt. Three real defects are owned by other PRs. The probes that came back healthy live only as prose on `origin/bugfixes20260913:simpleredis/BUGS.md`, so dest CI cannot stop a later edit from reintroducing a turn leak, a cross-key reply, a decoder panic, or a Yaegi-only error-path crash.

## What Changes

- Add permanent same-package tests that lock the healthy probes: chaos pool invariants, `FuzzReadReply` / `FuzzParseLen`, New/use/Close lifecycle plus Close-during-held-Gets, interpreted error paths in a new Yaegi file, concurrent MSetEX unknown-command fallback, and RESP command-injection as data.
- Gate long chaos and lifecycle stress behind `testing.Short()`. Lower iteration counts when `raceDetectorOn`. Do not commit a fuzz testdata corpus.
- Land `simpleredis/BUGS.md` reframed so the three real bugs point at their PRs, keep the Rejected and Previously-reported tables, and name the locking test on each Rejected row.
- Do not change any non-test file under `simpleredis/`. Do not edit `yaegi_test.go`. If a probe fails on dest, do not patch product source.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: chaos pool and lifecycle tests MUST prove at-rest turns, `OverFrees() == 0`, and settled open sockets; they MUST NOT assert peak live sockets.
- `std_go_simpleredis_resp-decode`: `readReply` MUST have a fuzz target that never panics and never returns values on a dirty stream; `parseLen` MUST have a fuzz target that never overflows `length+2`.
- `std_go_simpleredis_resp-commands`: keys/values containing CRLF and an inline PING payload MUST round-trip as data; concurrent MSetEX unknown-command fallback MUST stay balanced; interpreted error paths (dial retry, I/O timeout, cancel, pool wait, truncated bulk) MUST run without an interpreter panic.

## Impact

- New `simpleredis/*_test.go` files only (chaos, fuzz, lifecycle, Yaegi error paths, concurrent MSetEX, injection).
- `simpleredis/BUGS.md` (new on dest).
- Main specs `std_go_simpleredis_tcp-session`, `std_go_simpleredis_resp-decode`, and `std_go_simpleredis_resp-commands` after archive.
- Existing helpers reused: `fake_redis_test.go`, `yaegi_test.go` (read-only), `startStallRedis`, `raceDetectorOn`.
