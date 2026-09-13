## ADDED Requirements

### Requirement: Lint job pins golangci-lint v1
The CI `lint` job SHALL invoke golangci-lint at a pinned v1.x version that parses `.golangci.yml` (`v1.63.4`). It MUST NOT use `version: latest`. It MUST NOT use golangci-lint v2 (incompatible config schema).

#### Scenario: Lint action version is pinned
- **WHEN** CI job `lint` runs
- **THEN** `golangci/golangci-lint-action` `version` is `v1.63.4`
- **AND** it is not `latest`

### Requirement: Lint config is the enabled-linter contract
`.golangci.yml` SHALL enable the dest-zero linters (`decorder`, `dogsled`, `durationcheck`, `godot`, `makezero`, `mirror`, `misspell`, `nakedret`, `nestif`, `nilerr`, `nilnesserr`, `perfsprint`, `reassign`, `recvcheck`, `tparallel`, `usestdlibvars`, `whitespace`) plus `gocritic` with `unnamedResult` added to the default checker set (`checkExported: false`), `thelper`, `revive`, `dupword`, `prealloc`, `stylecheck`, `errname`, `forcetypeassert`, and `errorlint`. `gocritic.enabled-checks` MUST add checkers, not replace defaults. `testpackage` MUST NOT be enabled. `goimports` and `gofumpt` MUST NOT be enabled (they need a `diff` binary Windows contributors lack). `issues.exclude-dirs` SHALL include `apm_modules` and `.agents`. `forcetypeassert` SHALL be excluded on `_test.go`. Identity `==` on SimpleRedis retry/decode sentinels SHALL stay `==` with a site-specific `//nolint:errorlint`; those sites MUST NOT be rewritten to `errors.Is`.

#### Scenario: golangci-lint run is clean
- **WHEN** `golangci-lint` v1.63.4 runs with that config
- **THEN** it reports no issues

#### Scenario: testpackage stays off
- **WHEN** an implementer reads `.golangci.yml`
- **THEN** `testpackage` is not in `linters.enable`

### Requirement: Usage packet records lint pin and rejected tooling
`knowledge/devdocs/std_go_test-suites.md` SHALL state that CI lint is pinned to golangci-lint v1.63.4, that `testpackage` is rejected because tests are in-package white-box, that `goimports`/`gofumpt` are omitted, and that `unnamedResult.checkExported: false` is the stronger setting (`true` checks exported functions only).

#### Scenario: Catalog names the pin
- **WHEN** an implementer opens `knowledge/devdocs/std_go_test-suites.md`
- **THEN** that packet names the pinned golangci-lint version
- **AND** it records that `testpackage` stays off
