## Purpose

Names the four CI proof suites in this repo (lint, unit Go, Go E2E against live Redis and Dragonfly, Pester Traefik) and the skip versus run rules that keep unit tests free of backends. Unit Go has two jobs: plain `-short` and `-race -short`. Go E2E is two CI jobs.

### Requirement: Four named CI suites
CI SHALL run eight jobs: `lint` (golangci-lint), `test` named `Unit` (compiled Go tests that MUST NOT need Redis or Dragonfly and MUST NOT pass `-race`), `race` named `Unit race` (the same unit suite under the race detector), `e2e-redis` named `Go E2E Redis` (compiled Go tests against live Redis 7), `e2e-dragonfly` named `Go E2E Dragonfly` (compiled Go tests against live Dragonfly), `integration` named `Integration Tests` (Pester reclaim via `./Test-Integration.ps1 -Suite reclaim`), `integration-redis` named `Integration Tests Redis` (Pester SimpleRedis via `./Test-Integration.ps1 -Suite simpleredis -Engine redis`), and `integration-dragonfly` named `Integration Tests Dragonfly` (the same SimpleRedis file with `-Engine dragonfly`). Pester MUST NOT be the behaviour proof for compiled client or limiter live files. The unit `test` job MUST run `go test` with `-short` and MUST NOT start Redis or Dragonfly service containers. The `race` job MUST also pass `-short` and MUST NOT start Redis or Dragonfly. There MUST NOT be a job id `e2e`. The Redis live job MUST start `redis:7-alpine` on `:6379` and a passworded Redis on `:6381`, set every `*_LIVE_REDIS` and `SIMPLEREDIS_LIVE_REDIS_AUTH` address, MUST NOT set Dragonfly LIVE addresses, and MUST run `go test` without `-short`. The Dragonfly live job MUST start `dragonfly:v1.40.2` published as `:6380` and a passworded Dragonfly on `:6382`, set every `*_LIVE_DRAGONFLY` and `SIMPLEREDIS_LIVE_DRAGONFLY_AUTH` address, MUST NOT set Redis LIVE addresses, and MUST run `go test` without `-short`. SimpleRedis Pester SHALL be one file that reads `INTEGRATION_ENGINE` (`redis` or `dragonfly`) and MUST NOT duplicate Its or `-TestCases` per engine. `integration-redis` MUST NOT be treated as Dragonfly proof. `integration-dragonfly` MUST NOT be treated as Redis proof. `integration` MUST NOT run the SimpleRedis file.

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

#### Scenario: Go E2E Redis starts Redis only
- **WHEN** the CI `e2e-redis` job runs
- **THEN** Redis 7 is reachable at `127.0.0.1:6379`
- **AND** passworded Redis is reachable at `127.0.0.1:6381`
- **AND** Dragonfly LIVE addresses are unset
- **AND** `go test` runs without `-short`
- **AND** compiled Redis live tests MUST NOT skip

#### Scenario: Go E2E Dragonfly starts Dragonfly only
- **WHEN** the CI `e2e-dragonfly` job runs
- **THEN** Dragonfly is reachable at `127.0.0.1:6380`
- **AND** passworded Dragonfly is reachable at `127.0.0.1:6382`
- **AND** Redis LIVE addresses are unset
- **AND** `go test` runs without `-short`
- **AND** compiled Dragonfly live tests MUST NOT skip

#### Scenario: Pester reclaim stays Integration Tests
- **WHEN** CI runs `integration`
- **THEN** it invokes `./Test-Integration.ps1 -Suite reclaim`
- **AND** that job is not a substitute for compiled live files
- **AND** it does not run `scripts/integration-tests.simpleredis.Tests.ps1`

#### Scenario: Pester SimpleRedis Redis is its own job
- **WHEN** CI runs `integration-redis`
- **THEN** it invokes `./Test-Integration.ps1 -Suite simpleredis -Engine redis`
- **AND** `scripts/integration-tests.simpleredis.Tests.ps1` runs once with `INTEGRATION_ENGINE=redis`
- **AND** that job is not a substitute for compiled live files or for Dragonfly Pester

#### Scenario: Pester SimpleRedis Dragonfly is its own job
- **WHEN** CI runs `integration-dragonfly`
- **THEN** it invokes `./Test-Integration.ps1 -Suite simpleredis -Engine dragonfly`
- **AND** `scripts/integration-tests.simpleredis.Tests.ps1` runs once with `INTEGRATION_ENGINE=dragonfly`
- **AND** that job is not a substitute for compiled live files or for Redis Pester

### Requirement: Unit race job runs the race detector
The `race` job SHALL invoke `go test` with `-race`, `-short`, `-count=1`, and a timeout of at least 10 minutes. That job MUST compile SimpleRedis. The unit `test` job MUST NOT pass `-race` so `TestAlloc*` run. The `e2e-redis` and `e2e-dragonfly` jobs MUST NOT be required to pass `-race`. A passing race job MUST NOT be treated as proof of in-use-turn length or idle-cap accounting.

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

#### Scenario: Go E2E Redis stays without -race
- **WHEN** the CI `e2e-redis` job runs
- **THEN** `go test` runs without `-short`
- **AND** it is not required to pass `-race`

#### Scenario: Go E2E Dragonfly stays without -race
- **WHEN** the CI `e2e-dragonfly` job runs
- **THEN** `go test` runs without `-short`
- **AND** it is not required to pass `-race`

### Requirement: Live tests run the engines whose addresses are set
Compiled and Yaegi live tests SHALL skip when `testing.Short` is set or when both Redis and Dragonfly addresses for that package are unset. When exactly one of those two addresses is set and tests are not `-short`, the test MUST run against that engine and MUST NOT fail for the missing engine. When both addresses are set, each live case SHALL execute against Redis and against Dragonfly.

#### Scenario: Local go test without Docker skips
- **WHEN** both LIVE addresses for a package are unset
- **AND** tests are not `-short`
- **THEN** that package’s live tests skip
- **AND** they MUST NOT fail for missing engines

#### Scenario: One engine configured is a one-engine run
- **WHEN** tests are not `-short`
- **AND** only the Redis live address is set
- **THEN** that package’s live test runs against Redis
- **AND** it MUST NOT fail because Dragonfly is unset
- **WHEN** only the Dragonfly live address is set
- **THEN** that package’s live test runs against Dragonfly
- **AND** it MUST NOT fail because Redis is unset

### Requirement: Usage packet catalogs the suites
`knowledge/devdocs` SHALL include `std_go_test-suites.md` that names lint, unit Go, Go E2E Redis, Go E2E Dragonfly, Pester reclaim, Pester Redis, and Pester Dragonfly and states what each represents. That packet SHALL state that unit CI has a plain `-short` job and a separate `race` job, that Go E2E is two CI jobs, and that Pester SimpleRedis is two CI jobs that switch `INTEGRATION_ENGINE` on one file. README Tests SHALL name the same suites and SHALL mention the unit `race` job. Per-library usage packets SHALL point prove-with at that catalog for the live jobs.

#### Scenario: Catalog exists
- **WHEN** an implementer opens `knowledge/devdocs/index_std_go.md`
- **THEN** a row points at `std_go_test-suites.md`
- **AND** that packet distinguishes unit `go test` from Go E2E Redis, Go E2E Dragonfly, Pester reclaim, Pester Redis, and Pester Dragonfly
- **AND** that packet states unit CI has a plain `-short` job and a separate `race` job
