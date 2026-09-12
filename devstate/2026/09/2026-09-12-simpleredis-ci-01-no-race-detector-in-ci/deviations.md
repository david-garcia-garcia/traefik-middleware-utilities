# Deviations

- [x] taken  unit `-race` on the existing four-suite `test` job instead of the ticket's single 2m step
  Asked: one `Run Tests` step `go test -race -timeout 10m -count=1 -v ./...` (ticket cited a single 2m job; windows/macos could stay plain).
  Instead: dest already split unit (`-short` 2m) and e2e (live 5m), Ubuntu only; this run first put `-race` and 10m on `test` and left `e2e` plain.
  Owner: `.github/workflows/ci.yml` job `test` / `openspec/specs/std_go_ci_test-suites`
  Why: honouring a fifth race job, or `-race` on both Go jobs, would add a suite or race live engines against a catalog whose job is four named suites and one Ubuntu detector leg.
  By: explore
  Requester: not asked
- [x] taken  fifth job `race` (`Unit race`) instead of stacking `-race` on `test`
  Asked: isolate `-short` and `-race` so `TestAlloc*` still have a non-race CI job.
  Instead: `test` is dest `go test -short -timeout 2m`; `race` is `go test -race -short -timeout 10m`; four suites, five jobs.
  Owner: `.github/workflows/ci.yml` jobs `test` / `race` / `openspec/specs/std_go_ci_test-suites`
  Why: stacking `-race` on `test` skips `TestAlloc*` on that job; requester asked to split.
  By: implement
  Requester: asked
