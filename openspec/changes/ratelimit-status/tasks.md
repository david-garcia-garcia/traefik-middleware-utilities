## 1. Baseline

- [x] 1.1 On dest `ee9f5c11` (a scratch worktree, not this branch), run `go test -run '^$' -bench . -benchtime 1s -count 5 ./tokenbucket/ ./windowcounter/` (includes `BenchmarkYaegi*`); keep the raw output and the machine/Go version for the before column (design D9); verify every benchmark in `README.md` `## Rate limiters compared` has a before value

## 2. `tokenbucket.Status`

- [x] 2.1 Add `tokenbucket/status.go`: `Status` (`Allowed`, `Limit`, `Remaining`, `Period`, `Reset`, `RetryAfter`, `Delay`), `newStatus(clock, tokens, waitMicro)`, and the saturating `durationFromMicros` (design D1–D3), each with its doc comment; add `errEvalTokens` next to `errEvalWait` in `clock.go`; verify `go vet ./tokenbucket/` is clean
- [x] 2.2 Change `Memory.Allow` and `Redis.Allow` to return `(Status, error)` through `newStatus`, `Status{}` on every error; `Redis.Allow` parses `values[2]` after the wait with the same finite-number rule (design D4); `consumeOne`, the admit expression, and `allowScript` unchanged; verify `git diff ee9f5c11 -- tokenbucket/lua.go` is empty
- [x] 2.3 Migrate every `tokenbucket` test, live test, and benchmark caller to `Status` (`Delay + RetryAfter` where a test asserted the old wait; keep a sink in benchmarks); verify `go test -short -count=1 ./tokenbucket/` passes with the same test names as the baseline
- [x] 2.4 Migrate the interpreted probe sources in `limiter_yaegi_test.go`, `memory_yaegi_test.go`, `limiter_yaegi_e2e_test.go`, `bench_yaegi_test.go`; make the probes read `Allowed`, `Remaining`, `RetryAfter`; verify `go test -short -count=1 -run Yaegi ./tokenbucket/` passes with `useunsafe` false
- [x] 2.5 Add status tests: first Allow (`rate` 10, burst 5 → 5/4/500ms/100ms), fractional tokens floor, admitted with `Delay` 1s, denied with `RetryAfter` 1s, debt `Remaining` 0 with `Reset` > `Period`, refunded deny, `Period` saturation (burst `1e12`, rate `0.001`), `RetryAfter` saturation on a huge wait, `Status{}` on error, malformed tokens (`nan`, `+Inf`, `-Inf`, `inf`, `abc`) → `errEvalTokens`, tokens text `4`/`4.0`/`-1.0`/`0.3`/`1e+15` accepted, memory/Redis parity on the fake (`std_go_tokenbucket_status`, `std_go_tokenbucket_allow` deltas); verify they pass
- [x] 2.6 Extend the live parity test in `limiter_e2e_test.go` (and its Yaegi live twin) to compare `Allowed`, `Limit`, `Remaining`, `Period`, `Reset`, and the `Delay`/`RetryAfter` class; verify against a disposable `redis:7-alpine` and `dragonfly:v1.40.2` container, or rely on CI `e2e-redis` / `e2e-dragonfly`
- [x] 2.7 Update `README.md` `## Token bucket` example and `knowledge/devdocs/std_go_tokenbucket.md` (snippet, How to use, Gotchas: `Period` is derived, `RetryAfter` is the full wait, `Delay + RetryAfter` = old wait, Redis 14-digit tokens, `errEvalTokens`, saturation); verify the snippets compile when pasted into a scratch `main` against this branch

## 3. `windowcounter.Status`

- [x] 3.1 Add `windowcounter/status.go`: `Status` (`Allowed`, `Limit`, `Remaining`, `Period`, `Reset`, `RetryAfter`, `Estimated`) and `newStatus(sliding, limit, window, current, previous)` with the retry-after closed form and its predicate check (design D5–D6), each with its doc comment; verify `go vet ./windowcounter/` is clean
- [x] 3.2 In `limiter.go`: add `now`, `windowSec`, `windowStart` to `slidingWindow`; move the weight expression unchanged into `weightAt` used by `slidingAt` and the retry check; pass `sliding` and `limit` to `takeExact`, `takeBuffered`, `peekExact`, `peekBuffered`, which end in `newStatus`; `Take`/`Peek`/`Allow` return `(Status, error)` with `Status{}` on error; verify the `Estimated` expression and operand order are unchanged in the diff and no Redis call was added
- [x] 3.3 Migrate every `windowcounter` test, live test, and benchmark caller to `Status`; verify `go test -short -count=1 ./windowcounter/` passes with the same test names as the baseline, including `TestBuffered_*`, `repro_*`, and the fake's command assertions
- [x] 3.4 Migrate the interpreted probe sources in `limiter_yaegi_test.go`, `limiter_yaegi_e2e_test.go`, `bench_yaegi_test.go`; make the deny probe read `Allowed` false, `Remaining` 0, `RetryAfter` > 0; verify `go test -short -count=1 -run Yaegi ./windowcounter/` passes with `useunsafe` false
- [x] 3.5 Add status tests on the fake with a fixed clock: fresh-window Takes (`Remaining` 7 of 10), Peek `limit − 2.3` → 2, `Reset` 44.75s at 15.25s into a 60s window and `window` at the start, same-window retry (limit 10, window 10s, previous 10 → `RetryAfter` 2s, Take admitted 2s later), next-window retry (limit 2, window 10s, three Takes at 5s → 12s), denied Peek retry, Peek at exactly `limit` (`Allowed`, `Remaining` 0, `RetryAfter` 0), limit 0 → `RetryAfter == Reset`, buffered outage at `limit − 1` → `Allowed`, `Remaining` 0, nil error, `Status{}` on error, and a property check that for random counts the reported retry second admits and the second before it does not (`std_go_windowcounter_status`, `std_go_windowcounter_sliding-take` deltas); verify they pass
- [x] 3.6 Add a live denied-Take status case to `limiter_e2e_test.go` (unique key per run: `Remaining` 0, `Reset` in `(0, window]`, `RetryAfter` > 0); verify against disposable Redis and Dragonfly containers or rely on CI `e2e-redis` / `e2e-dragonfly`
- [x] 3.7 Update `README.md` `## Window counter` example and `knowledge/devdocs/std_go_windowcounter.md` (snippet, How to use, Gotchas: `Reset` is the fixed-window end and not full quota, denied Takes push `RetryAfter` further, buffered values are this instance's view, `limit < 1` → `RetryAfter == Reset`); verify the snippets compile when pasted into a scratch `main` against this branch

## 4. Numbers

- [x] 4.1 Re-run the 1.1 command on this branch on the same machine and session; compare medians (ns/op, allocs/op) and confirm `redis-cmds/op` is unchanged for every fake-backed benchmark
- [x] 4.2 If any median in `README.md` `## Rate limiters compared` moved, update the table from the 4.1 output with the commit it came from; verify every number in the table appears in the recorded output

## 5. Integration

- [x] 5.1 `go test -short -count=1 ./...`, `go vet ./...`, and `golangci-lint run` green; `grep -rnE 'allowed, (wait|estimated)|_, _, err = counter' README.md knowledge/devdocs/` returns nothing; CI `test`, `e2e-redis`, and `e2e-dragonfly` green

## Workflow follow-up

- Devdocs impact: define `Status` and `Remaining` in `## Language` of `std_go_tokenbucket.md` and `std_go_windowcounter.md`, and drop `remaining quota` from `_Avoid_` where it contradicts the new field (keep the Peek-as-reservation and Peek-as-Allow avoids).
- Archive: fold the deltas into `std_go_tokenbucket_allow` and `std_go_windowcounter_sliding-take`; create `std_go_tokenbucket_status` and `std_go_windowcounter_status`.
- Release: pick the tag for a breaking change (design Open Questions).
