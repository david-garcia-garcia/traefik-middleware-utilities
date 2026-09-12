## Purpose

Names the four CI proof suites in this repo (lint, unit Go, Go E2E against live Redis and Dragonfly, Pester Traefik) and the skip versus fail rules that keep unit tests free of backends. Unit Go has two jobs: plain `-short` and `-race -short`.

### Requirement: Four named CI suites
CI SHALL run five jobs: `lint` (golangci-lint), `test` named `Unit` (compiled Go tests that MUST NOT need Redis or Dragonfly and MUST NOT pass `-race`), `race` named `Unit race` (the same unit suite under the race detector), `e2e` named `Go E2E` (compiled Go tests against live Redis 7 and Dragonfly), and `integration` named `Integration Tests` (Pester via `Test-Integration.ps1`). Pester MUST NOT be the behaviour proof for compiled client or limiter live files. The unit `test` job MUST run `go test` with `-short` and MUST NOT start Redis or Dragonfly service containers. The `race` job MUST also pass `-short` and MUST NOT start Redis or Dragonfly. The `e2e` job MUST start `redis:7-alpine` on `:6379` and `dragonfly:v1.40.2` published as `:6380`, set every `WINDOWCOUNTER_LIVE_*`, `TOKENBUCKET_LIVE_*`, and `SIMPLEREDIS_LIVE_*` address, and run `go test` without `-short`.

#### Scenario: Unit job has no engines
- **WHEN** the CI `test` job runs
- **THEN** it does not start Redis or Dragonfly
- **AND** it invokes `go test` with `-short`
- **AND** compiled live tests skip

#### Scenario: Unit race job has no engines
- **WHEN** the CI `race` job runs
- **THEN** it does not start Redis or Dragonfly
- **AND** it invokes `go test` with `-short`
- **AND** compiled live tests skip

#### Scenario: Go E2E job starts both engines
- **WHEN** the CI `e2e` job runs
- **THEN** Redis 7 is reachable at `127.0.0.1:6379`
- **AND** Dragonfly is reachable at `127.0.0.1:6380`
- **AND** all six LIVE address env vars are set
- **AND** `go test` runs without `-short`
- **AND** compiled live tests MUST NOT skip

#### Scenario: Pester stays a separate job
- **WHEN** CI runs `integration`
- **THEN** it invokes `./Test-Integration.ps1`
- **AND** that job is not a substitute for compiled live files

### Requirement: Unit race job runs the race detector
The `race` job SHALL invoke `go test` with `-race`, `-short`, `-count=1`, and a timeout of at least 10 minutes. That job MUST compile SimpleRedis. The unit `test` job MUST NOT pass `-race` so `TestAlloc*` run. The `e2e` job MUST NOT be required to pass `-race`. A passing race job MUST NOT be treated as proof of in-use-turn length or idle-cap accounting.

#### Scenario: Unit race job passes -race
- **WHEN** the CI `race` job runs
- **THEN** it invokes `go test` with `-race`
- **AND** it still passes `-short`
- **AND** it still passes `-count=1`
- **AND** the `go test` timeout is at least 10 minutes
- **AND** it does not start Redis or Dragonfly

#### Scenario: Unit job does not pass -race
- **WHEN** the CI `test` job runs
- **THEN** it invokes `go test` with `-short`
- **AND** it does not pass `-race`

#### Scenario: Go E2E stays without -race
- **WHEN** the CI `e2e` job runs
- **THEN** `go test` runs without `-short`
- **AND** it is not required to pass `-race`

### Requirement: Live tests fail when only one engine address is set
Compiled and Yaegi live tests SHALL skip when `testing.Short` is set or when both Redis and Dragonfly addresses for that package are unset. When exactly one of those two addresses is set and tests are not `-short`, the test MUST fail. Each live case that runs SHALL execute against Redis and against Dragonfly.

#### Scenario: Local go test without Docker skips
- **WHEN** both LIVE addresses for a package are unset
- **AND** tests are not `-short`
- **THEN** that package’s live tests skip
- **AND** they MUST NOT fail for missing engines

#### Scenario: One engine configured is a fail
- **WHEN** tests are not `-short`
- **AND** only the Redis live address is set
- **THEN** that package’s live test fails
- **WHEN** only the Dragonfly live address is set
- **THEN** that package’s live test fails

### Requirement: Usage packet catalogs the suites
`knowledge/devdocs` SHALL include `std_go_test-suites.md` that names lint, unit Go, Go E2E, and Pester and states what each represents. That packet SHALL state that unit CI has a plain `-short` job and a separate `race` job. README Tests SHALL name the same four suites and SHALL mention the unit `race` job. Per-library usage packets SHALL point prove-with at that catalog for the live job.

#### Scenario: Catalog exists
- **WHEN** an implementer opens `knowledge/devdocs/index_std_go.md`
- **THEN** a row points at `std_go_test-suites.md`
- **AND** that packet distinguishes unit `go test` from Go E2E and from Pester
- **AND** that packet states unit CI has a plain `-short` job and a separate `race` job
