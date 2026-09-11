# Standards

1. [hard] Name for the scope — `reclaim/table.go:376`, `reclaim/default.go:34`, `reclaim/default.go:40` — exported `(*Table).Reset`, package `Reset`, and `ResetWith` are test-only teardown entry points (`// Tests only` on each); the commandment requires the identifier to say test (`ResetForTest`), and a comment does not replace the name
   → Rename to `ResetForTest` / `ResetWithForTest` (and the method equivalent on `*Table`), then update `table_test.go` and `knowledge/devdocs/std_go_reclaim.md` callers
   Status: skipped
   Argument: explore assumed keep `Reset` so imported specs and geoblock import switch stay aligned; public API not renamed unattended.
