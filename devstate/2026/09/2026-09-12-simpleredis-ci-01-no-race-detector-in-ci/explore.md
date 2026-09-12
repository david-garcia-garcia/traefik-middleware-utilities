# Explore
IssueKey: 2026-09-12-simpleredis-ci-01-no-race-detector-in-ci

## Concepts

- **Unit Go job `test`** — `.github/workflows/ci.yml` lines 27–38. `ubuntu-latest`, `go test -short -timeout 2m -count=1 -v ./...`. Compiles SimpleRedis. No Redis/Dragonfly services. Live `*_e2e_test.go` skip.
- **Go E2E job `e2e`** — same file lines 40–103. Live Redis 7 + Dragonfly, `go test -timeout 5m -count=1 -v ./...` without `-short`. Also compiles SimpleRedis. No `-race`.
- **Four named suites** — `openspec/specs/std_go_ci_test-suites/spec.md` pins lint, unit, Go E2E, Pester. Usage `knowledge/devdocs/std_go_test-suites.md`. README Tests names the same four. No fifth job on dest.
- **Pool shared state** — `simpleredis/simpleredis.go` `idleConnsMu`/`idleConns`, `closed`, `inUseTurns`, `groupWriteMu`/`groupWrite`. Detector target. Channel length under the idle mutex is not a data race (requirement Out of scope / bug-03).
- **Existing concurrent canaries (unit, fake TCP)** — `TestConcurrentCommandsStayWithinPool` (8×20 Get), `TestBurstGetsStayWithinLiveCap` (5×64), `TestOverlappingCallersDoNotDialPastLiveCap` (32 overlapping), `TestCloseDuringInFlightCommandClosesSocketOnRelease` (Close vs held Get). They run under `-short`.
- **Yaegi unit** — `simpleredis/yaegi_test.go`, `interpretedcost_test.go`. Same `test` job. Ticket: `-race` plus Yaegi can exceed 2m.
- **Local Windows `-race`** — reproduced: `CGO_ENABLED=1 go test -race -short ./simpleredis/` → `cgo: C compiler "gcc" not found`. CI Ubuntu is the only detector host.

```
DestBranch CI
  lint        golangci-lint
  test        go test -short -timeout 2m     ← compiles pool, no -race
  e2e         go test -timeout 5m + engines  ← compiles pool, no -race
  integration Pester
```

## Decisions

- Put `-race` on the existing unit `test` job. Keep `-short` and `-count=1`. Raise that step to `-timeout 10m`. Leave `e2e` without `-race` at 5m (the non-race Ubuntu job). Do not add a fifth job.
- Reuse dest concurrent unit tests as the detector canary. Do not land risk-05 token-conservation / 60–200-round Close-under-traffic loops.
- Spec and usage catalog must name unit `-race` (and the 10m cap) when the pinned `go test` line gains the flag. README Tests sentence that quotes unit `go test -short` follows the same catalog.
- Fuzz, `apm_modules/` gitignore, other `simpleredisfixes2` findings, Windows/macOS legs, and pool runtime behaviour stay out of scope.

## Open questions

- Q: Which Ubuntu job runs `go test -race`?
  Rank: bounded incidental — existing unit `test` `go test` line plus four enumerated catalog sites (`.github/workflows/ci.yml` job `test`, `openspec/specs/std_go_ci_test-suites/spec.md`, `knowledge/devdocs/std_go_test-suites.md`, `README.md` Tests); criterion 1 says at least one Ubuntu `go test` that compiles SimpleRedis, not which job
  Decision: assumed — unit `test` gets `go test -race -short -timeout 10m -count=1 -v ./...`; `e2e` stays without `-race`. A fifth job would rewrite the four-suite pin. Both jobs would drop the ticket's non-race Ubuntu fallback and race live engines.
  By: explore

- Q: Are dest concurrent unit tests enough detector canaries, or must this change add the ticket's token-conservation / Close-under-traffic stresses?
  Rank: additive asked — criterion 4 names land or reuse; dest already has concurrent Get and Close-vs-in-flight tests under `-short`
  Decision: assumed — reuse `TestConcurrentCommandsStayWithinPool`, `TestBurstGetsStayWithinLiveCap`, `TestOverlappingCallersDoNotDialPastLiveCap`, and `TestCloseDuringInFlightCommandClosesSocketOnRelease`. The 60/200-round loops live in out-of-scope risk-05.
  By: explore

- Q: Does Yaegi plus `-race` fit in 10m on the unit job, and would live engines plus `-race` fit in 10m on e2e?
  Rank: additive asked — criterion 2 names raise that job's timeout so `-race` plus Yaegi (and live jobs if they also take `-race`) do not fail the existing 2m/5m caps; ticket example is 10m
  Decision: assumed — 10m on unit only; do not put `-race` on e2e. Local wall time not measured (gcc missing). If the unit race job times out, raise from that CI log.
  By: explore
