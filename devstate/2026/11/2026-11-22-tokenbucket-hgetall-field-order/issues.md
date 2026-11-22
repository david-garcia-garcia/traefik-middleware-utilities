# Issues

- [x] note large  `knowledge/debt/2026-11-22-simpleredis-bench-tokenbucket-hgetall.md`
  Why: `simpleredis/bench_test.go` conserva una copia de `tokenBucketScript` que lee `HGETALL` por posición.
  Taken: bench payload is now a verbatim copy of allowScript, guarded by TestSimpleredisBenchUsesAllowScript.
