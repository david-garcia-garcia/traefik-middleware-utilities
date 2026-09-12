# Standards

1. [hard] One job, one owner — `openspec/changes/simpleredis-test-binary-compile/specs/std_go_simpleredis_resp-commands/spec.md:3` — the ADDED compile requirement also owns interpreter-pass (and no-Traefik), which live `Interpreter tests observe Init Get Set Del` already owns
   ```
   ### Requirement: SimpleRedis test package compiles
   `go test ./simpleredis/` SHALL compile as one binary. Tests that write a temp GOPATH for Yaegi, including interpreted-cost measurements, MUST use the same-package helper the interpreter tests already define. The package MUST NOT fail to compile because that helper is missing. Those tests MUST NOT start Traefik. CI `go test ./...` MUST keep compiling this package.
   ...
   #### Scenario: Interpreter tests still execute
   - **WHEN** `go test -short ./simpleredis/` runs the Yaegi interpreter cases
   - **THEN** Init Get Set Del, Incr and Eval, Eval NOSCRIPT fallback, MSetEX native, and MSetEX Lua fallback all pass
   ```
   → Keep this requirement to one-binary compile and helper presence; leave interpreter pass and no-Traefik on the existing interpreter-tests requirement
   Status: done
   Argument: dropped interpreter-pass scenario and no-Traefik from the ADDED compile requirement; existing Interpreter tests requirement still owns those.
