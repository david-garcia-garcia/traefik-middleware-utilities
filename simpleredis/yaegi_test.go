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

// TestYaegi_NewGetSetDel proves interpreted code can New, Set, Get, and Del
// against a compiled fake TCP Redis. Traefik is not started.
func TestYaegi_NewGetSetDel(t *testing.T) {
	_, addr := startFakeRedis(t, map[string]string{})
	goPath := t.TempDir()
	writeGopathSimpleredis(t, goPath)
	writeGopathFile(t, goPath, "clientprobe", "roundtrip.go", clientprobeSrc)

	got := evalClientprobe(t, goPath, fmt.Sprintf(`clientprobe.RoundTrip(%q)`, addr))
	if got != "ok" {
		t.Fatalf("yaegi round-trip: %q, want ok", got)
	}
}

// TestYaegi_IncrAndEval proves interpreted Incr and Eval against a compiled fake. Traefik is not started.
func TestYaegi_IncrAndEval(t *testing.T) {
	_, addr := startFakeRedis(t, map[string]string{})
	goPath := t.TempDir()
	writeGopathSimpleredis(t, goPath)
	writeGopathFile(t, goPath, "clientprobe", "roundtrip.go", clientprobeSrc)

	got := evalClientprobe(t, goPath, fmt.Sprintf(`clientprobe.IncrAndEval(%q)`, addr))
	if got != "ok" {
		t.Fatalf("yaegi incr+eval: %q, want ok", got)
	}
}

// TestYaegi_EvalNoScriptFallback proves interpreted Eval recovers from a NOSCRIPT miss. Traefik is not started.
func TestYaegi_EvalNoScriptFallback(t *testing.T) {
	_, addr := startFakeRedis(t, map[string]string{})
	goPath := t.TempDir()
	writeGopathSimpleredis(t, goPath)
	writeGopathFile(t, goPath, "clientprobe", "roundtrip.go", clientprobeSrc)

	got := evalClientprobe(t, goPath, fmt.Sprintf(`clientprobe.EvalNoScript(%q)`, addr))
	if got != "ok" {
		t.Fatalf("yaegi eval noscript fallback: %q, want ok", got)
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
func writeGopathSimpleredis(t testing.TB, goPath string) {
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
func writeGopathFile(t testing.TB, goPath, pkg, name, src string) {
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
	"fmt"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// RoundTrip builds a client, Sets a key, Gets it, and Dels it.
func RoundTrip(host string) string {
	client := simpleredis.New(simpleredis.Config{Host: host})
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

const kongIncrbyExpireatScript = ` + "`" + `local exists = redis.call("exists", KEYS[1])
local value = redis.call("incrby", KEYS[1], ARGV[1])
if exists == 0 then
  redis.call("expireat", KEYS[1], ARGV[2])
end
return value` + "`" + `

// IncrAndEval Incs a missing key then Evals the Kong incrby+expireat snippet.
func IncrAndEval(host string) string {
	client := simpleredis.New(simpleredis.Config{Host: host})
	afterIncr, err := client.Incr("yaegi-incr")
	if err != nil {
		return "incr:" + err.Error()
	}
	if afterIncr != 1 {
		return fmt.Sprintf("incr:%d", afterIncr)
	}
	values, err := client.Eval(kongIncrbyExpireatScript, []string{"yaegi-eval"}, []string{"3", "1700000000"})
	if err != nil {
		return "eval:" + err.Error()
	}
	if len(values) != 1 || string(values[0]) != "3" {
		return fmt.Sprintf("eval:%q", values)
	}
	return "ok"
}

// EvalNoScript Evals once so the compiled fake's first EVALSHA miss must fall back.
func EvalNoScript(host string) string {
	client := simpleredis.New(simpleredis.Config{Host: host})
	values, err := client.Eval(kongIncrbyExpireatScript, []string{"yaegi-noscript"}, []string{"3", "1700000000"})
	if err != nil {
		return "eval:" + err.Error()
	}
	if len(values) != 1 || string(values[0]) != "3" {
		return fmt.Sprintf("eval:%q", values)
	}
	return "ok"
}
`
