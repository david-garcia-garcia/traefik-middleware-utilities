# Test coverage

1. [hard] Critical path untested — `simpleredis/simpleredis.go:230` — `execPipeline` skips wholesale retry when `err == errTimeout`; `TestTimeoutOnReusedConnIsNotRetried` only calls `Get`
   → Drive ExecPipeline on a reused idle socket that stalls after the first reply and assert `redis:timeout` with one accept
   Status: done
   Argument: added TestPipelineTimeoutOnReusedConnIsNotRetried (d044e6e).
2. [hard] Assertion does not prove the job — `simpleredis/simpleredis.go:373` — truncated read returns `reusable=false`; `TestPipelineTruncationIsNotRetried` only asserts `err != nil` and `accepts == 1`
   → After the truncated ExecPipeline, assert idle is empty so the dirty conn is closed, not pooled
   Status: done
   Argument: TestPipelineTruncationIsNotRetried now asserts idle is empty (d044e6e).
