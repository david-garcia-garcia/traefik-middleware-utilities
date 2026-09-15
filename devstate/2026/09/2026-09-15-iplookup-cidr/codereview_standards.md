# Standards

1. [hard] Leave a trail — `iplookup/helper_yaegi_test.go:25` — `evalLookupprobe`, `writeGopathIplookup`, `copyNonTestGo`, `callerDir`, and `writeGopathFile` are new functions with no godoc while the sibling Yaegi harness documents the same roles on `backendbackoff/gate_yaegi_test.go`
   → Add one-line doc comments on each helper, aligned with `evalTripprobe` / `writeGopathBackendbackoff` / `copyNonTestGo` / `callerDir` / `writeGopathFile` there
   Status: done
   Argument: godoc on evalLookupprobe, writeGopathIplookup, copyNonTestGo, callerDir, writeGopathFile in helper_yaegi_test.go
