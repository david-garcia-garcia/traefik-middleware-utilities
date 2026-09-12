## MODIFIED Requirements

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

### Requirement: Usage packet catalogs the suites
`knowledge/devdocs` SHALL include `std_go_test-suites.md` that names lint, unit Go, Go E2E Redis, Go E2E Dragonfly, Pester reclaim, Pester Redis, and Pester Dragonfly and states what each represents. That packet SHALL state that unit CI has a plain `-short` job and a separate `race` job, that Go E2E is two CI jobs, and that Pester SimpleRedis is two CI jobs that switch `INTEGRATION_ENGINE` on one file. README Tests SHALL name the same suites and SHALL mention the unit `race` job. Per-library usage packets SHALL point prove-with at that catalog for the live jobs.

#### Scenario: Catalog exists
- **WHEN** an implementer opens `knowledge/devdocs/index_std_go.md`
- **THEN** a row points at `std_go_test-suites.md`
- **AND** that packet distinguishes unit `go test` from Go E2E Redis, Go E2E Dragonfly, Pester reclaim, Pester Redis, and Pester Dragonfly
- **AND** that packet states unit CI has a plain `-short` job and a separate `race` job
