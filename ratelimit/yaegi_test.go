package ratelimit

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// TestYaegi_TakeUntilDeny proves interpreted Take against a compiled fake. Traefik is not started.
func TestYaegi_TakeUntilDeny(t *testing.T) {
	_, addr := startTestFakeRedis(t, nil)
	goPath := t.TempDir()
	writeGopathLimiter(t, goPath)
	writeGopathFile(t, goPath, "takeprobe", "roundtrip.go", takeprobeSrc)

	got := evalTakeprobe(t, goPath, fmt.Sprintf(`takeprobe.UntilDeny(%q, 3)`, addr))
	if got != "ok" {
		t.Fatalf("yaegi take: %q, want ok", got)
	}
}

// TestYaegiLive_TakeUntilDeny runs the same UntilDeny against a live engine when addrs are set.
func TestYaegiLive_TakeUntilDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("live engines skipped under -short")
	}
	addr := os.Getenv("RATELIMIT_LIVE_REDIS")
	if addr == "" {
		addr = os.Getenv("RATELIMIT_LIVE_DRAGONFLY")
	}
	if addr == "" {
		t.Skip("live addrs unset")
	}
	goPath := t.TempDir()
	writeGopathLimiter(t, goPath)
	writeGopathFile(t, goPath, "takeprobe", "roundtrip.go", takeprobeSrc)
	got := evalTakeprobe(t, goPath, fmt.Sprintf(`takeprobe.UntilDeny(%q, 3)`, addr))
	if got != "ok" {
		t.Fatalf("yaegi live take: %q, want ok", got)
	}
}

func evalTakeprobe(t *testing.T, goPath, expr string) string {
	t.Helper()
	interpreter := interp.New(interp.Options{GoPath: goPath})
	if err := interpreter.Use(stdlib.Symbols); err != nil {
		t.Fatalf("use stdlib: %v", err)
	}
	if _, err := interpreter.Eval(`import "takeprobe"`); err != nil {
		t.Fatalf("import takeprobe: %v", err)
	}
	evaluated, err := interpreter.Eval(expr)
	if err != nil {
		t.Fatalf("eval %s: %v", expr, err)
	}
	return evaluated.Interface().(string)
}

func writeGopathLimiter(t *testing.T, goPath string) {
	t.Helper()
	src := callerDir(t)
	copyNonTestGo(t, src, goPath, "ratelimit")
	copyNonTestGo(t, filepath.Join(filepath.Dir(src), "simpleredis"), goPath, "simpleredis")
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

const takeprobeSrc = `package takeprobe

import (
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/ratelimit"
	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// UntilDeny Takes until the limit then expects a deny.
func UntilDeny(host string, limit int) string {
	client := &simpleredis.SimpleRedis{}
	client.Init(host, "", "")
	limiter, err := ratelimit.New(client, 0)
	if err != nil {
		return "new:" + err.Error()
	}
	n := int64(limit)
	for i := int64(0); i < n; i++ {
		allowed, _, takeErr := limiter.Take("yaegi-k", n, time.Minute)
		if takeErr != nil {
			return "take:" + takeErr.Error()
		}
		if !allowed {
			return "early"
		}
	}
	allowed, _, err := limiter.Take("yaegi-k", n, time.Minute)
	if err != nil {
		return "deny:" + err.Error()
	}
	if allowed {
		return "not-denied"
	}
	return "ok"
}
`
