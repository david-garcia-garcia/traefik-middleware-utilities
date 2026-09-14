# Spec

1. [extra] `openspec/changes/harden-golangci-lint/specs/std_go_ci_test-suites/spec.md` — Requirement: Lint config is the enabled-linter contract / `tasks.md` 1.1 — `.golangci.yml` adds `run.timeout: 5m`; no change artifact or SHALL names a run timeout.
   Status: skipped
   Argument: dest already had `run.timeout: 5m`; kept as-is, not a new contract.
2. [extra] `openspec/changes/harden-golangci-lint/specs/std_go_ci_test-suites/spec.md` — Requirement: Usage packet records lint pin and rejected tooling / `tasks.md` 5.1 — `knowledge/devdocs/std_go_test-suites.md` adds an Identity `==` / `//nolint:errorlint` section; the usage SHALL and task 5.1 only require the pin, `testpackage`, goimports/gofumpt, and `checkExported` trap (errorlint identity is bound in the lint-config requirement on code, not in the usage packet SHALL).
   Status: skipped
   Argument: identity `==` gotcha is the DestBranch failure this PR guards; keep it in usage even though task 5.1 did not name that sentence.
