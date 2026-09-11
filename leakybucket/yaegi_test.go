package leakybucket

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

func TestYaegi_TakePourToCap(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	goPath := t.TempDir()
	writeGopathLeakybucket(t, goPath)
	writeGopathFile(t, goPath, "takeprobe", "roundtrip.go", takeprobeSrc)

	got := evalTakeprobe(t, goPath, fmt.Sprintf(`takeprobe.PourToCapThenDeny(%q, %q)`, addr, "yaegi-k"))
	if got != "ok" {
		t.Fatalf("yaegi take: %q, want ok", got)
	}
}

func TestYaegi_IdleThenTake(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	goPath := t.TempDir()
	writeGopathLeakybucket(t, goPath)
	writeGopathFile(t, goPath, "takeprobe", "roundtrip.go", takeprobeSrc)

	got := evalTakeprobe(t, goPath, fmt.Sprintf(`takeprobe.IdleThenTake(%q, %q)`, addr, "yaegi-idle"))
	if got != "ok" {
		t.Fatalf("yaegi idle: %q, want ok", got)
	}
}

func TestYaegiLive_RedisAndDragonfly(t *testing.T) {
	if testing.Short() {
		t.Skip("live engines skipped under -short")
	}
	backends := []struct {
		name string
		addr string
	}{
		{"redis", os.Getenv("LEAKYBUCKET_LIVE_REDIS")},
		{"dragonfly", os.Getenv("LEAKYBUCKET_LIVE_DRAGONFLY")},
	}
	anyAddr := false
	for _, backend := range backends {
		if backend.addr == "" {
			continue
		}
		anyAddr = true
		backend := backend
		t.Run(backend.name, func(t *testing.T) {
			goPath := t.TempDir()
			writeGopathLeakybucket(t, goPath)
			writeGopathFile(t, goPath, "takeprobe", "roundtrip.go", takeprobeSrc)
			t.Run("exactPourThenLeak", func(t *testing.T) {
				got := evalTakeprobe(t, goPath, fmt.Sprintf(`takeprobe.IdleThenTake(%q, %q)`, backend.addr, t.Name()))
				if got != "ok" {
					t.Fatalf("yaegi live pour: %q, want ok", got)
				}
			})
			t.Run("bufferedTwoInstances", func(t *testing.T) {
				got := evalTakeprobe(t, goPath, fmt.Sprintf(`takeprobe.BufferedShare(%q, %q)`, backend.addr, t.Name()))
				if got != "ok" {
					t.Fatalf("yaegi live buffered: %q, want ok", got)
				}
			})
			t.Run("memoryAgrees", func(t *testing.T) {
				got := evalTakeprobe(t, goPath, fmt.Sprintf(`takeprobe.MemoryAgrees(%q, %q)`, backend.addr, t.Name()))
				if got != "ok" {
					t.Fatalf("yaegi live agree: %q, want ok", got)
				}
			})
		})
	}
	if !anyAddr {
		t.Skip("live addrs unset")
	}
}

// evalTakeprobe evaluates expr in a GOPATH interp with stdlib only (no unsafe).
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

// writeGopathLeakybucket copies non-test leakybucket and simpleredis sources into a GOPATH module tree.
func writeGopathLeakybucket(t *testing.T, goPath string) {
	t.Helper()
	src := callerDir(t)
	copyNonTestGo(t, src, goPath, "leakybucket")
	copyNonTestGo(t, filepath.Join(filepath.Dir(src), "simpleredis"), goPath, "simpleredis")
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

// callerDir is the directory of the test file that called writeGopathLeakybucket.
func callerDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(1)
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

const takeprobeSrc = `package takeprobe

import (
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/leakybucket"
	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

func PourToCapThenDeny(host, key string) string {
	client := &simpleredis.SimpleRedis{}
	client.Init(host, "", "")
	limiter, err := leakybucket.NewRedis(client, 1, 3, 0, 2*time.Second)
	if err != nil {
		return "new:" + err.Error()
	}
	now := time.Unix(1700000000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	for i := 0; i < 3; i++ {
		allowed, _, _, takeErr := limiter.Take(key)
		if takeErr != nil {
			return "take:" + takeErr.Error()
		}
		if !allowed {
			return "early"
		}
	}
	allowed, _, _, err := limiter.Take(key)
	if err != nil {
		return "deny:" + err.Error()
	}
	if allowed {
		return "not-denied"
	}
	return "ok"
}

func IdleThenTake(host, key string) string {
	client := &simpleredis.SimpleRedis{}
	client.Init(host, "", "")
	limiter, err := leakybucket.NewRedis(client, 1, 3, 0, 2*time.Second)
	if err != nil {
		return "new:" + err.Error()
	}
	now := time.Unix(1700000000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	for i := 0; i < 3; i++ {
		allowed, _, _, takeErr := limiter.Take(key)
		if takeErr != nil {
			return "fill:" + takeErr.Error()
		}
		if !allowed {
			return "early"
		}
	}
	now = now.Add(3 * time.Second)
	limiter.SetNowForTest(func() time.Time { return now })
	allowed, _, _, err := limiter.Take(key)
	if err != nil {
		return "drain:" + err.Error()
	}
	if !allowed {
		return "still-full"
	}
	return "ok"
}

func BufferedShare(host, key string) string {
	aClient := &simpleredis.SimpleRedis{}
	aClient.Init(host, "", "")
	bClient := &simpleredis.SimpleRedis{}
	bClient.Init(host, "", "")
	a, err := leakybucket.NewRedis(aClient, 1, 3, time.Hour, 2*time.Second)
	if err != nil {
		return "new-a:" + err.Error()
	}
	b, err := leakybucket.NewRedis(bClient, 1, 3, time.Hour, 2*time.Second)
	if err != nil {
		return "new-b:" + err.Error()
	}
	now := time.Unix(1700000000, 0)
	a.SetNowForTest(func() time.Time { return now })
	b.SetNowForTest(func() time.Time { return now })
	allowed, _, _, takeErr := a.Take(key)
	if takeErr != nil || !allowed {
		return "a-early"
	}
	allowed, _, _, takeErr = b.Take(key)
	if takeErr != nil || !allowed {
		return "b-early"
	}
	a.Sleep()
	b.Sleep()
	reader, err := leakybucket.NewRedis(aClient, 1, 3, 0, 2*time.Second)
	if err != nil {
		return "reader:" + err.Error()
	}
	reader.SetNowForTest(func() time.Time { return now })
	level, err := reader.Level(key)
	if err != nil {
		return "level:" + err.Error()
	}
	if level != 2 {
		return "not-sum"
	}
	a.Close()
	b.Close()
	return "ok"
}

func MemoryAgrees(host, key string) string {
	client := &simpleredis.SimpleRedis{}
	client.Init(host, "", "")
	mem, err := leakybucket.NewMemory(2, 2, 2*time.Second)
	if err != nil {
		return "mem:" + err.Error()
	}
	red, err := leakybucket.NewRedis(client, 2, 2, 0, 2*time.Second)
	if err != nil {
		return "red:" + err.Error()
	}
	now := time.Unix(1700000000, 0)
	mem.SetNowForTest(func() time.Time { return now })
	red.SetNowForTest(func() time.Time { return now })
	for i := 0; i < 4; i++ {
		mAllowed, mLevel, _, mErr := mem.Take(key)
		if mErr != nil {
			return "m:" + mErr.Error()
		}
		rAllowed, rLevel, _, rErr := red.Take(key)
		if rErr != nil {
			return "r:" + rErr.Error()
		}
		if mAllowed != rAllowed {
			return "allowed-mismatch"
		}
		if (mLevel <= 1e-6) != (rLevel <= 1e-6) {
			return "empty-mismatch"
		}
		if (mLevel >= 2-1e-6) != (rLevel >= 2-1e-6) {
			return "full-mismatch"
		}
	}
	return "ok"
}
`
