# Issues

- [ ] note large  `knowledge/debt/2026-09-12-backendbackoff-shared-layer.md`
  Why: a shared layer so peers learn an unhealthy backend without request-path Redis I/O; deferred from this in-memory first version.
- [x] take small  `reclaim/yaegi_test.go` skip `TestYaegi_OpenHooksRunSleepWakeClose` under `-race`
  Why: Yaegi v0.16.1 select races on context cancel; Unit race failed after Sync with master. Unit without -race still runs the hooks proof.
