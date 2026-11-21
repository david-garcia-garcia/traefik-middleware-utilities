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

func waitUntilAnomalies(r map[string]int64) string {
	if r["longEarly"] != 0 || r["shortErr"] != 0 || r["shortSlow"] != 0 {
		return fmt.Sprintf("waitUntil crossed: %v", r)
	}
	return ""
}

func borrowWaitAnomalies(r map[string]int64) string {
	if r["newErr"] != 0 || r["nilNil"] != 0 || r["patientEarly"] != 0 || r["patientNotPoolWait"] != 0 || r["cancelLate"] != 0 || r["hung"] != 0 {
		return fmt.Sprintf("borrowSocket wait crossed: %v", r)
	}
	return ""
}

func twoPoolsAnomalies(r map[string]int64) string {
	if r["newErr"] != 0 || r["freeA"] != r["capA"] || r["freeB"] != r["capB"] || r["overFreesA"] != 0 || r["overFreesB"] != 0 {
		return fmt.Sprintf("in-use turns crossed between clients: %v", r)
	}
	return ""
}

func saturatedGetAnomalies(r map[string]int64) string {
	if r["newErr"] != 0 || r["calls"] == 0 || r["panics"] != 0 || r["wrongValue"] != 0 || r["unexpectedErr"] != 0 ||
		r["free"] != r["cap"] || r["overFrees"] != 0 || r["hung"] != 0 {
		return fmt.Sprintf("saturated Get crossed: %v", r)
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
		r := evalSelectProbe(t, interpreter, `simpleredis.SelectProbeWaitUntil()`)
		t.Logf("round %d: %v", round, r)
		if msg := waitUntilAnomalies(r); msg != "" {
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
		r := evalSelectProbe(t, interpreter, fmt.Sprintf(`simpleredis.SelectProbeBorrowWait(%q)`, addr))
		t.Logf("round %d: %v", round, r)
		if msg := borrowWaitAnomalies(r); msg != "" {
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
		r := evalSelectProbe(t, interpreter, fmt.Sprintf(`simpleredis.SelectProbeTwoPools(%q)`, addr))
		t.Logf("round %d: %v", round, r)
		if msg := twoPoolsAnomalies(r); msg != "" {
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
		r := evalSelectProbe(t, interpreter, fmt.Sprintf(`simpleredis.SelectProbeSaturatedGet(%q)`, addr))
		t.Logf("round %d: %v", round, r)
		if msg := saturatedGetAnomalies(r); msg != "" {
			t.Fatalf("round %d: %s", round, msg)
		}
	}
}
