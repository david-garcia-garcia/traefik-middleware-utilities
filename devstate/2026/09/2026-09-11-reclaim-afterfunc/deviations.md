# Deviations

- [x] taken  bound Test job timeout and wait for Redis/Dragonfly
  Asked: AfterFunc wait only; do not rewrite dest windowcounter or ci.yml.
  Instead: Test waits until Redis and Dragonfly answer PING, and `go test` uses `-timeout 2m -count=1`.
  Owner: `.github/workflows/ci.yml`
  Why: after merging dest live engines, the Test job sat on Go's 10-minute package timeout; GitHub MCP has no job logs, so the hang cannot be named; local `go test ./...` against those engines passed in 12s.
  By: pullrequest
  Requester: not asked
