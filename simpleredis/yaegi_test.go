package simpleredis

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

// TestYaegi_InitGetSetDel proves interpreted code can Init, Set, Get, and Del
// against a compiled fake TCP Redis. Traefik is not started.
func TestYaegi_InitGetSetDel(t *testing.T) {
	_, addr := startFakeRedis(t, map[string]string{})
	goPath := t.TempDir()
	writeGopathSimpleredis(t, goPath)
	writeGopathFile(t, goPath, "clientprobe", "roundtrip.go", clientprobeSrc)

	got := evalClientprobe(t, goPath, fmt.Sprintf(`clientprobe.RoundTrip(%q)`, addr))
	if got != "ok" {
		t.Fatalf("yaegi round-trip: %q, want ok", got)
	}
}

// evalClientprobe evaluates expr in a GOPATH interp with stdlib only (no unsafe).
func evalClientprobe(t *testing.T, goPath, expr string) string {
	t.Helper()
	interpreter := interp.New(interp.Options{GoPath: goPath})
	if err := interpreter.Use(stdlib.Symbols); err != nil {
		t.Fatalf("use stdlib: %v", err)
	}
	if _, err := interpreter.Eval(`import "clientprobe"`); err != nil {
		t.Fatalf("import clientprobe: %v", err)
	}
	evaluated, err := interpreter.Eval(expr)
	if err != nil {
		t.Fatalf("eval %s: %v", expr, err)
	}
	return evaluated.Interface().(string)
}

// writeGopathSimpleredis copies non-test simpleredis sources into a GOPATH module tree.
func writeGopathSimpleredis(t *testing.T, goPath string) {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller path")
	}
	srcDir := filepath.Dir(thisFile)
	destDir := filepath.Join(goPath, "src", "github.com", "david-garcia-garcia", "traefik-middleware-utilities", "simpleredis")
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
		t.Fatal("no simpleredis sources copied into GOPATH")
	}
}

// writeGopathFile writes one interpreted package file under GOPATH/src/<pkg>.
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

const clientprobeSrc = `package clientprobe

import (
	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// RoundTrip Inits a client, Sets a key, Gets it, Dels it, then Gets a miss.
func RoundTrip(host string) string {
	client := &simpleredis.SimpleRedis{}
	client.Init(host, "", "")
	if err := client.Set("k", []byte("ok"), 60); err != nil {
		return "set:" + err.Error()
	}
	got, err := client.Get("k")
	if err != nil {
		return "get:" + err.Error()
	}
	if err := client.Del("k"); err != nil {
		return "del:" + err.Error()
	}
	return string(got)
}
`
