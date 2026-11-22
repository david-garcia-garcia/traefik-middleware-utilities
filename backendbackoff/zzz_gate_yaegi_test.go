package backendbackoff

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

func TestYaegi_AllowThenReportTrips(t *testing.T) {
	goPath := t.TempDir()
	writeGopathBackendbackoff(t, goPath)
	writeGopathFile(t, goPath, "tripprobe", "roundtrip.go", tripprobeSrc)

	got := evalTripprobe(t, goPath, `tripprobe.TripAfterFailures()`)
	if got != "ok" {
		t.Fatalf("yaegi trip: %q, want ok", got)
	}
}

// evalTripprobe evaluates expr in a GOPATH interp with stdlib only (no unsafe).
func evalTripprobe(t *testing.T, goPath, expr string) string {
	t.Helper()
	interpreter := interp.New(interp.Options{GoPath: goPath})
	if err := interpreter.Use(stdlib.Symbols); err != nil {
		t.Fatalf("use stdlib: %v", err)
	}
	if _, err := interpreter.Eval(`import "tripprobe"`); err != nil {
		t.Fatalf("import tripprobe: %v", err)
	}
	evaluated, err := interpreter.Eval(expr)
	if err != nil {
		t.Fatalf("eval %s: %v", expr, err)
	}
	return evaluated.Interface().(string)
}

// writeGopathBackendbackoff copies non-test backendbackoff sources into a GOPATH module tree.
func writeGopathBackendbackoff(tb testing.TB, goPath string) {
	tb.Helper()
	copyNonTestGo(tb, callerDir(tb), goPath, "backendbackoff")
}

// copyNonTestGo copies non-test .go files from srcDir into GOPATH/src/.../<pkg>.
func copyNonTestGo(tb testing.TB, srcDir, goPath, pkg string) {
	tb.Helper()
	destDir := filepath.Join(goPath, "src", "github.com", "david-garcia-garcia", "traefik-middleware-utilities", pkg)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		tb.Fatal(err)
	}
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		tb.Fatal(err)
	}
	copied := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(srcDir, name))
		if err != nil {
			tb.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(destDir, name), body, 0o600); err != nil {
			tb.Fatal(err)
		}
		copied++
	}
	if copied == 0 {
		tb.Fatalf("no %s sources copied into GOPATH", pkg)
	}
}

// callerDir is the directory of the test file that called writeGopathBackendbackoff.
func callerDir(tb testing.TB) string {
	tb.Helper()
	_, thisFile, _, ok := runtime.Caller(1)
	if !ok {
		tb.Fatal("no caller path")
	}
	return filepath.Dir(thisFile)
}

// writeGopathFile writes one interpreted package file under GOPATH/src/<pkg>.
func writeGopathFile(tb testing.TB, goPath, pkg, name, src string) {
	tb.Helper()
	dir := filepath.Join(goPath, "src", pkg)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		tb.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600); err != nil {
		tb.Fatal(err)
	}
}

const tripprobeSrc = `package tripprobe

import (
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/backendbackoff"
)

func TripAfterFailures() string {
	gate, err := backendbackoff.New(backendbackoff.Config{
		FailureRatio: 0.30,
		TripFailures: 5,
		BaseCooldown: time.Second,
		MaxCooldown:  10 * time.Second,
		Jitter:       0,
		TTL:          time.Minute,
	})
	if err != nil {
		return "new:" + err.Error()
	}
	for i := 0; i < 5; i++ {
		allowed, _, allowErr := gate.Allow("k")
		if allowErr != nil {
			return "allow:" + allowErr.Error()
		}
		if !allowed {
			return "early"
		}
		if reportErr := gate.Report("k", false); reportErr != nil {
			return "report:" + reportErr.Error()
		}
	}
	allowed, _, err := gate.Allow("k")
	if err != nil {
		return "deny:" + err.Error()
	}
	if allowed {
		return "not-denied"
	}
	return "ok"
}
`
