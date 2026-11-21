# Standards

1. [hard] Leave a trail — `simpleredis/repro_yaegi_select_crosstalk_test.go:50` — five new functions have no job comment: `evalSelectProbe` (line 50), `waitUntilAnomalies` (63), `borrowWaitAnomalies` (70), `twoPoolsAnomalies` (77) and `saturatedGetAnomalies` (84). The sibling interpreter helpers in `yaegi_errorpath_test.go` (`evalErrorpath`) and `yaegi_test.go` (`evalClientprobe`) each have one
   Fix: Add a one-line job comment to each one, for example "evalSelectProbe evaluates one SelectProbe* call in the interpreter and returns its counter map" and "waitUntilAnomalies names the waitUntil counters that must be zero; empty means clean"
   Status: done
   Argument: Added a one-line job comment to evalSelectProbe and the four *Anomalies helpers in repro_yaegi_select_crosstalk_test.go.
   Quote:
   ```go
   func evalSelectProbe(t *testing.T, interpreter *interp.Interpreter, expr string) map[string]int64 {
   ...
   func waitUntilAnomalies(r map[string]int64) string {
   ...
   func borrowWaitAnomalies(r map[string]int64) string {
   ```
2. [hard] One job, one owner — `simpleredis/pool_test.go:927` — `assertTurnsFullAndNoOverFrees` re-implements the locked `len(sr.turnWaiters)` read that this diff already gives to `queuedTurnWaiters` (`pool_test.go:679`) in the same file
   Fix: Replace the inline `inUseMu.Lock` / `len(sr.turnWaiters)` / `Unlock` block with `queued := queuedTurnWaiters(sr)`
   Status: done
   Argument: assertTurnsFullAndNoOverFrees now calls queuedTurnWaiters(sr) instead of the inline locked read (same change as Nitpicks 6).
   Quote:
   ```go
   sr.inUseMu.Lock()
   queued := len(sr.turnWaiters)
   sr.inUseMu.Unlock()
   if queued != 0 {
   	t.Fatalf("turnWaiters = %d, want 0 at rest", queued)
   }
   ```
3. [judgement] Duplicated Code — `simpleredis/repro_yaegi_select_crosstalk_test.go:36` — `selectProbeInterp` hard-codes the interpreted simpleredis GOPATH directory that `writeGopathSimpleredis` (`yaegi_test.go:263`) already builds. If the module path changes, both copies have to be edited, and a mismatch would put the probe in a different package from the sources
   Fix: Let one helper own the GOPATH simpleredis directory (have `writeGopathSimpleredis` return `destDir`, or add `gopathSimpleredisDir(goPath)`), and write `zz_selectprobe.go` into the directory it returns
   Status: skipped
   Argument: Judgement, not applied unattended: sharing the GOPATH dir means changing the pre-existing writeGopathSimpleredis helper used by the other Yaegi suites; the literal is test-only and a mismatch fails loudly at import.
   Quote:
   ```go
   dest := filepath.Join(goPath, "src", "github.com", "david-garcia-garcia", "traefik-middleware-utilities", "simpleredis", "zz_selectprobe.go")
   ```
