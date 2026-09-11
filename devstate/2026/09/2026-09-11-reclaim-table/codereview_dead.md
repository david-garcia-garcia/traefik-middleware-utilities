# Dead

1. [hard] Test-only new symbol — `reclaim/default.go:34` — `Reset` has no production callers; only `ResetWith` (same file) and `reclaim/table_test.go` call it
   ```go
   // Reset tears down the process table (sleeps then closes every incarnation) and installs a fresh
   // one. Tests only.
   func Reset() {
   	ResetWith(DefaultGrace)
   }
   ```
   → Unexport into `table_test.go` as `resetForTest`, or rename `ResetForTest` / `ResetWithForTest` so the identifier says test; drop package-level export.
   Grep: `\bReset\b` in `*.go` — hits `default.go:34–35`, `default.go:44` (via `ResetWith`), and `table_test.go` only; no hit in `e2e/reclaimprobe/plugin.go` or other non-test sources.
   Status: skipped
   Argument: explore assumed keep `Reset` for spec/geoblock import alignment; not renamed unattended.

2. [hard] Test-only new symbol — `reclaim/default.go:40` — `ResetWith` has no production callers; only `Reset` (same file) and `reclaim/table_test.go` call it
   ```go
   // ResetWith replaces the process table after ending every incarnation on the current one, with
   // grace as how long a sleeping value is kept. Tests only.
   func ResetWith(grace time.Duration) {
   	defaultMu.Lock()
   	defer defaultMu.Unlock()
   	if defaultTable != nil {
   		defaultTable.Reset()
   	}
   	defaultTable = NewTable(grace)
   }
   ```
   → Move into `_test.go` or rename `ResetWithForTest`; tests that need a custom grace should call the unexported helper directly.
   Grep: `\bResetWith\b` in `*.go` — hits `default.go:35,40` and `table_test.go:1079` only.
   Status: skipped
   Argument: explore assumed keep `Reset` for spec/geoblock import alignment; not renamed unattended.

3. [hard] Test-only new symbol — `reclaim/table.go:376` — `(*Table).Reset` has no production callers; only `ResetWith` in `default.go` and `reclaim/table_test.go` call it
   ```go
   // Reset ends every incarnation on this table. An awake value is slept first, so Close never sees
   // a live value and orphan still precedes dispose. Tests only: it must not race an Open on the
   // same key. A slot that is mid-transition is ended by the goroutine that owns that transition.
   func (t *Table) Reset() {
   ```
   → Unexport into `_test.go` as `resetForTest`, or rename `ResetForTest`; keep teardown logic beside the tests that drive it.
   Grep: `\.Reset\(\)` in `*.go` — hits `default.go:44` and `table_test.go` only; `reclaim.Open` / `e2e/reclaimprobe` never call it.
   Status: skipped
   Argument: explore assumed keep `Reset` for spec/geoblock import alignment; not renamed unattended.
