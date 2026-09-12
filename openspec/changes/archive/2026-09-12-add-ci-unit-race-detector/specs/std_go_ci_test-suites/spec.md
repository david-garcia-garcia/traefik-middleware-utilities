## ADDED Requirements

### Requirement: Unit job runs the race detector
The unit `test` job SHALL invoke `go test` with `-race`, `-short`, `-count=1`, and a timeout of at least 10 minutes. That job MUST compile SimpleRedis. The `e2e` job MUST NOT be required to pass `-race`. A passing race job MUST NOT be treated as proof of in-use-turn length or idle-cap accounting.

#### Scenario: Unit job passes -race
- **WHEN** the CI `test` job runs
- **THEN** it invokes `go test` with `-race`
- **AND** it still passes `-short`
- **AND** it still passes `-count=1`
- **AND** the `go test` timeout is at least 10 minutes
- **AND** it does not start Redis or Dragonfly

#### Scenario: Go E2E stays without -race
- **WHEN** the CI `e2e` job runs
- **THEN** `go test` runs without `-short`
- **AND** it is not required to pass `-race`

## MODIFIED Requirements

### Requirement: Usage packet catalogs the suites
`knowledge/devdocs` SHALL include `std_go_test-suites.md` that names lint, unit Go, Go E2E, and Pester and states what each represents. That packet SHALL state that unit CI passes `-race`. README Tests SHALL name the same four suites and SHALL mention unit `-race`. Per-library usage packets SHALL point prove-with at that catalog for the live job.

#### Scenario: Catalog exists
- **WHEN** an implementer opens `knowledge/devdocs/index_std_go.md`
- **THEN** a row points at `std_go_test-suites.md`
- **AND** that packet distinguishes unit `go test` from Go E2E and from Pester
- **AND** that packet states unit CI passes `-race`
