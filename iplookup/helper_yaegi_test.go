package iplookup

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

func TestYaegi_FamilyMissAndHit(t *testing.T) {
	goPath := t.TempDir()
	writeGopathIplookup(t, goPath)
	writeGopathFile(t, goPath, "lookupprobe", "roundtrip.go", lookupprobeSrc)

	got := evalLookupprobe(t, goPath, `lookupprobe.HitAndFamilyMiss()`)
	if got != "ok" {
		t.Fatalf("yaegi lookup: %q, want ok", got)
	}
}

// evalLookupprobe evaluates expr in a GOPATH interp with stdlib only (no unsafe).
func evalLookupprobe(t *testing.T, goPath, expr string) string {
	t.Helper()
	interpreter := interp.New(interp.Options{GoPath: goPath})
	if err := interpreter.Use(stdlib.Symbols); err != nil {
		t.Fatalf("use stdlib: %v", err)
	}
	if _, err := interpreter.Eval(`import "lookupprobe"`); err != nil {
		t.Fatalf("import lookupprobe: %v", err)
	}
	evaluated, err := interpreter.Eval(expr)
	if err != nil {
		t.Fatalf("eval %s: %v", expr, err)
	}
	return evaluated.Interface().(string)
}

// writeGopathIplookup copies non-test iplookup sources into a GOPATH module tree.
func writeGopathIplookup(t *testing.T, goPath string) {
	t.Helper()
	copyNonTestGo(t, callerDir(t), goPath, "iplookup")
}

// copyNonTestGo copies non-test .go files from srcDir into GOPATH/src/.../<pkg>.
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

// callerDir is the directory of this Yaegi harness file.
func callerDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller path")
	}
	return filepath.Dir(thisFile)
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

const lookupprobeSrc = `package lookupprobe

import (
	"net"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/iplookup"
)

func HitAndFamilyMiss() string {
	h := iplookup.New()
	if err := h.AddCIDR("1.2.3.4/32", "office"); err != nil {
		return err.Error()
	}
	found, prefixLen, metadata, err := h.Contains(net.ParseIP("1.2.3.4"))
	if err != nil {
		return err.Error()
	}
	if !found || prefixLen != 32 || metadata != "office" {
		return "miss-hit"
	}
	found, _, _, err = h.Contains(net.ParseIP("102:304::1"))
	if err != nil {
		return err.Error()
	}
	if found {
		return "cross-family"
	}
	return "ok"
}
`
