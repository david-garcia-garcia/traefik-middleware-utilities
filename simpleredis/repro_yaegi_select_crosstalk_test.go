package simpleredis

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// Regression for the Yaegi v0.16.1 shared-select bug (same mechanism as crowdsec-bouncer-traefik-plugin
// PR #208). Each probe in repro_yaegi_select_probe_test.go runs compiled (control) and interpreted.
// Both runs must be clean. The interpreted run failed while session source used select for the
// in-use-turn semaphore and the retry backoff; TestSessionSourceHasNoSelect is the deterministic
// gate that keeps select out, and these pairs are the behavioural evidence under Yaegi.

const selectProbeRounds = 3

// selectProbeInterp builds one interpreter with simpleredis plus the probe file copied in as a
// regular source file, so the probes can call unexported functions under Yaegi.
func selectProbeInterp(t *testing.T) *interp.Interpreter {
	t.Helper()
	goPath := t.TempDir()
	writeGopathSimpleredis(t, goPath)
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller path")
	}
	src, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "repro_yaegi_select_probe_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(goPath, "src", "github.com", "david-garcia-garcia", "traefik-middleware-utilities", "simpleredis", "zz_selectprobe.go")
	if err := os.WriteFile(dest, src, 0o600); err != nil {
		t.Fatal(err)
	}
	interpreter := interp.New(interp.Options{GoPath: goPath})
	if err := interpreter.Use(stdlib.Symbols); err != nil {
		t.Fatalf("use stdlib: %v", err)
	}
	if _, err := interpreter.Eval(`import "github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"`); err != nil {
		t.Fatalf("import simpleredis: %v", err)
	}
	return interpreter
}

// evalSelectProbe evaluates one SelectProbe* call in the interpreter and returns its counter map.
func evalSelectProbe(t *testing.T, interpreter *interp.Interpreter, expr string) map[string]int64 {
	t.Helper()
	value, err := interpreter.Eval(expr)
	if err != nil {
		t.Fatalf("eval %s: %v", expr, err)
	}
	report, ok := value.Interface().(map[string]int64)
	if !ok {
		t.Fatalf("eval %s: got %T, want map[string]int64", expr, value.Interface())
	}
	return report
}

// waitUntilAnomalies names the SelectProbeWaitUntil counters that must be zero; empty means clean.
func waitUntilAnomalies(report map[string]int64) string {
	if report["longEarly"] != 0 || report["shortErr"] != 0 || report["shortSlow"] != 0 ||
		report["cancelNotCanceled"] != 0 || report["deadlineNotExceeded"] != 0 {
		return fmt.Sprintf("waitUntil crossed: %v", report)
	}
	return ""
}

// borrowWaitAnomalies names the SelectProbeBorrowWait counters that must be zero; empty means clean.
func borrowWaitAnomalies(report map[string]int64) string {
	if report["newErr"] != 0 || report["nilNil"] != 0 || report["patientEarly"] != 0 || report["patientNotPoolWait"] != 0 ||
		report["cancelLate"] != 0 || report["impatientNotCtxErr"] != 0 || report["hung"] != 0 {
		return fmt.Sprintf("borrowSocket wait crossed: %v", report)
	}
	return ""
}

// twoPoolsAnomalies reports a SelectProbeTwoPools run whose semaphores are not both full with no over-free; empty means clean.
func twoPoolsAnomalies(report map[string]int64) string {
	if report["newErr"] != 0 || report["freeA"] != report["capA"] || report["freeB"] != report["capB"] ||
		report["overFreesA"] != 0 || report["overFreesB"] != 0 {
		return fmt.Sprintf("in-use turns crossed between clients: %v", report)
	}
	return ""
}

// saturatedGetAnomalies reports a SelectProbeSaturatedGet run that panicked, misclassified, hung or left the semaphore unbalanced; empty means clean.
func saturatedGetAnomalies(report map[string]int64) string {
	if report["newErr"] != 0 || report["calls"] == 0 || report["panics"] != 0 || report["wrongValue"] != 0 || report["unexpectedErr"] != 0 ||
		report["free"] != report["cap"] || report["overFrees"] != 0 || report["hung"] != 0 {
		return fmt.Sprintf("saturated Get crossed: %v", report)
	}
	return ""
}

// skipYaegiUnderRace skips an interpreted probe under -race: the interpreter itself
// (github.com/traefik/yaegi/interp) is not race-clean. The _Compiled control still runs there.
func skipYaegiUnderRace(t *testing.T) {
	t.Helper()
	if raceDetectorOn {
		t.Skip("github.com/traefik/yaegi/interp is not race-clean; the _Compiled control covers -race")
	}
}

// TestReproSelect_WaitUntil_Compiled is the control: compiled waitUntil never crosses.
func TestReproSelect_WaitUntil_Compiled(t *testing.T) {
	for round := 0; round < selectProbeRounds; round++ {
		if msg := waitUntilAnomalies(SelectProbeWaitUntil()); msg != "" {
			t.Fatalf("round %d: %s", round, msg)
		}
	}
}

// TestReproSelect_WaitUntil_Yaegi: interpreted waitUntil wakes on its own timer or ctx only.
func TestReproSelect_WaitUntil_Yaegi(t *testing.T) {
	skipYaegiUnderRace(t)
	interpreter := selectProbeInterp(t)
	for round := 0; round < selectProbeRounds; round++ {
		report := evalSelectProbe(t, interpreter, `simpleredis.SelectProbeWaitUntil()`)
		t.Logf("round %d: %v", round, report)
		if msg := waitUntilAnomalies(report); msg != "" {
			t.Fatalf("round %d: %s", round, msg)
		}
	}
}

// TestReproSelect_BorrowWait_Compiled is the control: compiled borrowSocket waits on its own channels.
func TestReproSelect_BorrowWait_Compiled(t *testing.T) {
	_, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	for round := 0; round < selectProbeRounds; round++ {
		if msg := borrowWaitAnomalies(SelectProbeBorrowWait(addr)); msg != "" {
			t.Fatalf("round %d: %s", round, msg)
		}
	}
}

// TestReproSelect_BorrowWait_Yaegi: interpreted borrowSocket never returns (nil, nil), wakes early, or hangs.
func TestReproSelect_BorrowWait_Yaegi(t *testing.T) {
	skipYaegiUnderRace(t)
	_, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	interpreter := selectProbeInterp(t)
	for round := 0; round < selectProbeRounds; round++ {
		report := evalSelectProbe(t, interpreter, fmt.Sprintf(`simpleredis.SelectProbeBorrowWait(%q)`, addr))
		t.Logf("round %d: %v", round, report)
		if msg := borrowWaitAnomalies(report); msg != "" {
			t.Fatalf("round %d: %s", round, msg)
		}
	}
}

// TestReproSelect_TwoPools_Compiled is the control: compiled clients keep their own in-use turns.
func TestReproSelect_TwoPools_Compiled(t *testing.T) {
	_, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	for round := 0; round < selectProbeRounds; round++ {
		if msg := twoPoolsAnomalies(SelectProbeTwoPools(addr)); msg != "" {
			t.Fatalf("round %d: %s", round, msg)
		}
	}
}

// TestReproSelect_TwoPools_Yaegi: interpreted clients never move in-use turns between their semaphores.
func TestReproSelect_TwoPools_Yaegi(t *testing.T) {
	skipYaegiUnderRace(t)
	_, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	interpreter := selectProbeInterp(t)
	for round := 0; round < selectProbeRounds; round++ {
		report := evalSelectProbe(t, interpreter, fmt.Sprintf(`simpleredis.SelectProbeTwoPools(%q)`, addr))
		t.Logf("round %d: %v", round, report)
		if msg := twoPoolsAnomalies(report); msg != "" {
			t.Fatalf("round %d: %s", round, msg)
		}
	}
}

// TestReproSelect_SaturatedGet_Compiled is the control: compiled Get under pool saturation stays classified.
func TestReproSelect_SaturatedGet_Compiled(t *testing.T) {
	_, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	for round := 0; round < selectProbeRounds; round++ {
		if msg := saturatedGetAnomalies(SelectProbeSaturatedGet(addr)); msg != "" {
			t.Fatalf("round %d: %s", round, msg)
		}
	}
}

// TestReproSelect_SaturatedGet_Yaegi: interpreted Get under pool saturation never panics, never loses
// or duplicates a turn, and returns only pool wait, ctx, or unreachable errors.
func TestReproSelect_SaturatedGet_Yaegi(t *testing.T) {
	skipYaegiUnderRace(t)
	_, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	interpreter := selectProbeInterp(t)
	for round := 0; round < selectProbeRounds; round++ {
		report := evalSelectProbe(t, interpreter, fmt.Sprintf(`simpleredis.SelectProbeSaturatedGet(%q)`, addr))
		t.Logf("round %d: %v", round, report)
		if msg := saturatedGetAnomalies(report); msg != "" {
			t.Fatalf("round %d: %s", round, msg)
		}
	}
}
