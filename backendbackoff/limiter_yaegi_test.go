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

func writeGopathBackendbackoff(t *testing.T, goPath string) {
	t.Helper()
	copyNonTestGo(t, callerDir(t), goPath, "backendbackoff")
}

func copyNonTestGo(t *testing.T, srcDir, goPath, pkg string) {
	t.Helper()
	destDir := filepath.Join(goPath, "src", "github.com", "david-garcia-garcia", "traefik-middleware-utilities", pkg)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		t.Fatal(err)
	}
	copied := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(srcDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(destDir, name), body, 0o600); err != nil {
			t.Fatal(err)
		}
		copied++
	}
	if copied == 0 {
		t.Fatalf("no %s sources copied into GOPATH", pkg)
	}
}

func callerDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(1)
	if !ok {
		t.Fatal("no caller path")
	}
	return filepath.Dir(thisFile)
}

func writeGopathFile(t *testing.T, goPath, pkg, name, src string) {
	t.Helper()
	dir := filepath.Join(goPath, "src", pkg)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
}

const tripprobeSrc = `package tripprobe

import (
	"context"
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
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		allowed, _, allowErr := gate.Allow(ctx, "k")
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
	allowed, _, err := gate.Allow(ctx, "k")
	if err != nil {
		return "deny:" + err.Error()
	}
	if allowed {
		return "not-denied"
	}
	return "ok"
}
`
