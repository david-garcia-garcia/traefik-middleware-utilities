## prepare (2026-09-14)
phase: prepare
findings: qualified-with-gaps (coverage figure unmeasured here; parallel locking branch coupling)
fixed: n/a
skipped: table_gaps_test.go intentionally not committed in prepare

## explore (2026-09-14)
phase: explore
findings: measured 94.4% / nine count-0 blocks; waitCtx poll fast path already count 2; two Reset contracts are missing tests not missing statements
fixed: n/a
skipped: did not delete unreachable waitCtx Done or drop busy-wait
