# Go e2e tests against live backends

Review how tests are run in this project. Right now we have lint, test, and integration tests:
- test → regular Go tests that depend on nothing
- lint → lint tests
- integration → Pester tests
- [NEW] Go integration / e2e → Go tests that NEED backend services running (Redis or Dragonfly). These tests run against LIVE backends.

Make the new e2e tests cover as much as possible. Document our test coverage suites and what each one represents in knowledge/devdocs. The new e2e suite must all run both against Redis and Dragonfly, to ensure compatibility.
