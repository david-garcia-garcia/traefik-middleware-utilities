package iplookup

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

func BenchmarkYaegiLoadIPv4Hosts(b *testing.B) {
	runYaegiHostMapStoreBench(b, "", func(count int) string {
		return fmt.Sprintf(`hostmapstorebench.LoadIPv4(%d)`, count)
	})
}

func BenchmarkYaegiLoadIPv6Hosts(b *testing.B) {
	runYaegiHostMapStoreBench(b, "", func(count int) string {
		return fmt.Sprintf(`hostmapstorebench.LoadIPv6(%d)`, count)
	})
}

func BenchmarkYaegiContainsIPv4Hit(b *testing.B) {
	runYaegiHostMapStoreBench(b, fmt.Sprintf(`hostmapstorebench.SetupIPv4(%d)`, hostMapStoreBenchN), func(count int) string {
		return fmt.Sprintf(`hostmapstorebench.ContainsIPv4Hit(%d)`, count)
	})
}

func BenchmarkYaegiContainsIPv4Miss(b *testing.B) {
	runYaegiHostMapStoreBench(b, fmt.Sprintf(`hostmapstorebench.SetupIPv4(%d)`, hostMapStoreBenchN), func(count int) string {
		return fmt.Sprintf(`hostmapstorebench.ContainsIPv4Miss(%d)`, count)
	})
}

func BenchmarkYaegiContainsIPv6Hit(b *testing.B) {
	runYaegiHostMapStoreBench(b, fmt.Sprintf(`hostmapstorebench.SetupIPv6(%d)`, hostMapStoreBenchN), func(count int) string {
		return fmt.Sprintf(`hostmapstorebench.ContainsIPv6Hit(%d)`, count)
	})
}

func BenchmarkYaegiContainsIPv6Miss(b *testing.B) {
	runYaegiHostMapStoreBench(b, fmt.Sprintf(`hostmapstorebench.SetupIPv6(%d)`, hostMapStoreBenchN), func(count int) string {
		return fmt.Sprintf(`hostmapstorebench.ContainsIPv6Miss(%d)`, count)
	})
}

// runYaegiHostMapStoreBench evaluates a host-map load or Contains loop under Yaegi.
func runYaegiHostMapStoreBench(b *testing.B, setupExpr string, loopExpr func(count int) string) {
	b.Helper()
	goPath := b.TempDir()
	writeGopathIplookupHostMapStore(b, goPath)
	writeGopathFileHostMapStore(b, goPath, "hostmapstorebench", "loop.go", hostMapStoreBenchSrc)

	interpreter := interp.New(interp.Options{GoPath: goPath})
	if err := interpreter.Use(stdlib.Symbols); err != nil {
		b.Fatalf("use stdlib: %v", err)
	}
	if _, err := interpreter.Eval(`import "hostmapstorebench"`); err != nil {
		b.Fatalf("import hostmapstorebench: %v", err)
	}
	if setupExpr != "" {
		evaluated, err := interpreter.Eval(setupExpr)
		if err != nil {
			b.Fatalf("setup: %v", err)
		}
		if got := evaluated.Interface().(string); got != "ok" {
			b.Fatalf("setup = %q, want ok", got)
		}
	}
	if _, err := interpreter.Eval(loopExpr(1)); err != nil {
		b.Fatalf("warmup: %v", err)
	}

	b.ResetTimer()
	evaluated, err := interpreter.Eval(loopExpr(b.N))
	if err != nil {
		b.Fatalf("eval loop: %v", err)
	}
	b.StopTimer()
	if got := evaluated.Interface().(string); got != "ok" {
		b.Fatalf("loop = %q, want ok", got)
	}
}

// writeGopathIplookupHostMapStore copies non-test iplookup sources into a GOPATH module tree.
func writeGopathIplookupHostMapStore(b *testing.B, goPath string) {
	b.Helper()
	srcDir := callerDirHostMapStore(b)
	destDir := filepath.Join(goPath, "src", "github.com", "david-garcia-garcia", "traefik-middleware-utilities", "iplookup")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		b.Fatal(err)
	}
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		b.Fatal(err)
	}
	copied := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(srcDir, name))
		if err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(destDir, name), body, 0o600); err != nil {
			b.Fatal(err)
		}
		copied++
	}
	if copied == 0 {
		b.Fatal("no iplookup sources copied into GOPATH")
	}
}

// writeGopathFileHostMapStore writes one interpreted package file under GOPATH/src/<pkg>.
func writeGopathFileHostMapStore(b *testing.B, goPath, pkg, name, src string) {
	b.Helper()
	dir := filepath.Join(goPath, "src", pkg)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600); err != nil {
		b.Fatal(err)
	}
}

// callerDirHostMapStore is the directory of this Yaegi bench harness file.
func callerDirHostMapStore(b *testing.B) string {
	b.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		b.Fatal("no caller path")
	}
	return filepath.Dir(thisFile)
}

const hostMapStoreBenchSrc = `package hostmapstorebench

import (
	"fmt"
	"math/rand"
	"net"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/iplookup"
)

var (
	benchHelper *iplookup.Helper
	benchHit    net.IP
	benchMiss   net.IP
)

func LoadIPv4(n int) string {
	return loadTimes(false, n)
}

func LoadIPv6(n int) string {
	return loadTimes(true, n)
}

func loadTimes(v6 bool, n int) string {
	for i := 0; i < n; i++ {
		if got := loadFamily(v6, 100000); got != "ok" {
			return got
		}
	}
	return "ok"
}

func SetupIPv4(n int) string {
	return setupFamily(false, n, net.ParseIP("240.0.0.1"))
}

func SetupIPv6(n int) string {
	return setupFamily(true, n, net.ParseIP("100::1"))
}

func ContainsIPv4Hit(n int) string {
	return containsLoop(n, true)
}

func ContainsIPv4Miss(n int) string {
	return containsLoop(n, false)
}

func ContainsIPv6Hit(n int) string {
	return containsLoop(n, true)
}

func ContainsIPv6Miss(n int) string {
	return containsLoop(n, false)
}

func loadFamily(v6 bool, n int) string {
	h := iplookup.New()
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < n; i++ {
		if err := h.AddCIDR(hostCIDR(rng, v6), ""); err != nil {
			return err.Error()
		}
	}
	if h.Count() == 0 {
		return "empty"
	}
	return "ok"
}

func setupFamily(v6 bool, n int, miss net.IP) string {
	h := iplookup.New()
	rng := rand.New(rand.NewSource(1))
	var last string
	for i := 0; i < n; i++ {
		last = hostCIDR(rng, v6)
		if err := h.AddCIDR(last, ""); err != nil {
			return err.Error()
		}
	}
	_, block, err := net.ParseCIDR(last)
	if err != nil {
		return err.Error()
	}
	benchHelper = h
	benchHit = block.IP
	benchMiss = miss
	return "ok"
}

func containsLoop(n int, hit bool) string {
	query := benchMiss
	if hit {
		query = benchHit
	}
	for i := 0; i < n; i++ {
		found, _, _, err := benchHelper.Contains(query)
		if err != nil {
			return err.Error()
		}
		if hit && !found {
			return "miss-hit"
		}
		if !hit && found {
			return "false-hit"
		}
	}
	return "ok"
}

func hostCIDR(rng *rand.Rand, v6 bool) string {
	if !v6 {
		octetA := rng.Intn(223) + 1
		octetB := rng.Intn(256)
		octetC := rng.Intn(256)
		octetD := rng.Intn(256)
		return fmt.Sprintf("%d.%d.%d.%d/32", octetA, octetB, octetC, octetD)
	}
	addr := make(net.IP, 16)
	rng.Read(addr)
	addr[0] = 0x20
	return addr.String() + "/128"
}
`
