# ci-01 — CI never runs the race detector on a hand-rolled concurrent pool

- **Axis**: Test coverage / CI
- **Severity**: hard
- **Where**: `.github/workflows/ci.yml:67`
- **Status**: not applied

## What I found

The test step has no `-race`:

```yaml
- name: Run Tests
  run: go test -timeout 2m -count=1 -v ./...
```

Meanwhile `simpleredis` coordinates shared state four different ways:

| Mechanism | Where |
|---|---|
| `sync.Mutex` over the idle list | `simpleredis.go:49-50` |
| `atomic.Bool` for the closed flag | `simpleredis.go:51` |
| Buffered channel as a counting semaphore | `simpleredis.go:53` |
| A second `sync.Mutex` for the capability cache | `simpleredis.go:56-57` |

None of it has ever been checked by the detector. Nor could I compensate locally:

```
go: -race requires cgo; enable cgo by setting CGO_ENABLED=1
cgo: C compiler "gcc" not found: exec: "gcc": executable file not found in %PATH%
```

So **every finding in this backlog was produced without race-detector coverage**,
including the concurrency ones. Ubuntu runners have gcc, so CI is the only place
this can realistically run.

There is one specific latent hazard the detector would speak to. `release` reads
`len(sr.inUseTurns)` while holding `idleConnsMu`, but borrowers take tokens from
that channel **without** it (see
[bug-03](bug-03-maxidleconns-not-enforced.md)). Channel length is not a data race,
so `-race` will *not* flag it — worth stating so nobody concludes a green run
validates that logic. What the detector does cover is the `idleConns` slice, the
`groupWrite` cache, and every field touched during `Close` racing with `release`.

## Why it matters

A hand-written pool is the highest-value target the detector has. The bugs it finds
are the ones that never reproduce locally, appear only under production
concurrency, and corrupt state rather than failing cleanly.

The package is also unusually exposed to them: `borrow` and `release` maintain the
token invariant by hand across eight early returns, `Close` mutates the idle list
concurrently with in-flight releases, and every finding in this backlog adds a new
path — `context` cancellation ([risk-01](risk-01-no-context-uncancellable-latency.md))
adds an entirely new early-exit route through both functions, and pipelining
(`../simpleredisfixes/perf-04-pipelining.md`) adds concurrent use of a single
socket. Landing those without detector coverage is how a subtle race ships.

I did verify the concurrency behaviour that matters by other means — 60 rounds of
`Close` under traffic and 200 forced `release`/`Close` interleavings leaked no
socket and always refilled the semaphore — but stress testing without the detector
proves absence of *symptoms*, not absence of races.

## Expected gain

Race coverage on every CI run, on the code most likely to have one. Also a
meaningful jump in the *effective* value of the existing suite: the concurrency
tests already in the package currently exercise interleavings without checking
memory access.

Cost is roughly 2–10× slower tests, so the `-timeout 2m` needs raising.

## How to fix

Add `-race` and give it room:

```yaml
- name: Run Tests
  run: go test -race -timeout 10m -count=1 -v ./...
```

Three practical notes:

1. **Raise the timeout in the same commit.** `-race` on the Yaegi tests
   (which interpret whole packages) plus the live Redis/Dragonfly jobs will exceed
   2 minutes. A timeout failure looks like a hang and will get the flag reverted.
2. **Keep a non-race job if wall time matters.** The `windows`/`macos` legs can stay
   plain; one Ubuntu leg with `-race` gets essentially all the value, since races
   are not platform-specific here.
3. **`-count=1` is already right** — caching a race-detector pass would defeat it.

Two adjacent gaps worth closing in the same pass:

- **No fuzz target exists.** The parser is a pure byte-in/value-out function, which
  is the ideal fuzz shape, and fuzzing is how
  [bug-02](bug-02-unbounded-reply-allocation.md) and
  [risk-03](risk-03-readline-unbounded.md) would have been found immediately. Add
  `FuzzReadReply` over a `bufio.Reader`, seed it with the shapes tabled in
  [risk-04](risk-04-non-basic-resp2-replies-rejected.md), and run a short
  `-fuzztime` in CI with the corpus committed.
- **`go test ./...` currently fails outside the module's own packages.** The
  untracked `apm_modules/` tree contains Go files that fail to load
  (`found packages pool (idle-lock.go) and geo (label-with-debug.go)`). It is
  untracked and *not* git-ignored, so CI is green only because nobody has committed
  it. Add `apm_modules/` to `.gitignore` before that happens. See also
  [build-01](build-01-test-binary-does-not-build.md).

## How to prove it

The fix is the proof: a CI run with `-race` that passes. To confirm the flag is
actually doing something rather than silently no-op'ing, land it alongside one
deliberately concurrent test that the detector would flag if the pool regressed —
the token-conservation stress described in
[risk-05](risk-05-freeinuseturn-blocks-when-full.md) plus a `Close`-under-traffic
test are the two to run under it.

A cheap sanity check when introducing it: temporarily remove a lock in a scratch
branch and confirm CI goes red. If it does not, the flag is not reaching the code
under test.
