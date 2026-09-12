## ADDED Requirements

### Requirement: SimpleRedis test package compiles
`go test ./simpleredis/` SHALL compile as one binary. Tests that write a temp GOPATH for Yaegi, including interpreted-cost measurements, MUST use the same-package helper the interpreter tests already define. The package MUST NOT fail to compile because that helper is missing. CI `go test ./...` MUST keep compiling this package.

#### Scenario: Test binary compiles
- **WHEN** `go test -c ./simpleredis/` runs
- **THEN** the compile succeeds
