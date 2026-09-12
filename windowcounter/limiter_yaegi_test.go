package windowcounter

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
	_, addr := startTestFakeRedis(t)
	goPath := t.TempDir()
	writeGopathWindowcounter(t, goPath)
	writeGopathFile(t, goPath, "takeprobe", "roundtrip.go", takeprobeSrc)

	got := evalTakeprobe(t, goPath, fmt.Sprintf(`takeprobe.UntilDeny(%q, %q, 3)`, addr, "yaegi-k"))
	if got != "ok" {
		t.Fatalf("yaegi take: %q, want ok", got)
	}
}

// TestYaegi_PeekThenTake proves interpreted Peek does not increment before Take. Traefik is not started.
func TestYaegi_PeekThenTake(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	goPath := t.TempDir()
	writeGopathWindowcounter(t, goPath)
	writeGopathFile(t, goPath, "takeprobe", "roundtrip.go", takeprobeSrc)

	got := evalTakeprobe(t, goPath, fmt.Sprintf(`takeprobe.PeekThenTake(%q, %q)`, addr, "yaegi-peek-k"))
	if got != "ok" {
		t.Fatalf("yaegi peek: %q, want ok", got)
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

// writeGopathWindowcounter copies non-test windowcounter and simpleredis sources into a GOPATH module tree.
func writeGopathWindowcounter(t *testing.T, goPath string) {
	t.Helper()
	src := callerDir(t)
	copyNonTestGo(t, src, goPath, "windowcounter")
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

// callerDir is the directory of the test file that called writeGopathWindowcounter.
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
	"context"
	"strconv"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/windowcounter"
	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// UntilDeny Takes until the limit then expects a deny.
func UntilDeny(host, key string, limit int) string {
	client := simpleredis.New(simpleredis.Config{Host: host})
	limiter, err := windowcounter.New(client, 0)
	if err != nil {
		return "new:" + err.Error()
	}
	n := int64(limit)
	for i := int64(0); i < n; i++ {
		allowed, _, takeErr := limiter.Take(context.Background(), key, n, time.Minute)
		if takeErr != nil {
			return "take:" + takeErr.Error()
		}
		if !allowed {
			return "early"
		}
	}
	allowed, _, err := limiter.Take(context.Background(), key, n, time.Minute)
	if err != nil {
		return "deny:" + err.Error()
	}
	if allowed {
		return "not-denied"
	}
	return "ok"
}

// BufferedShare flushes two clients then expects the shared count to deny the next Take.
func BufferedShare(host, key string) string {
	aClient := simpleredis.New(simpleredis.Config{Host: host})
	bClient := simpleredis.New(simpleredis.Config{Host: host})
	a, err := windowcounter.New(aClient, time.Hour)
	if err != nil {
		return "new-a:" + err.Error()
	}
	b, err := windowcounter.New(bClient, time.Hour)
	if err != nil {
		return "new-b:" + err.Error()
	}
	const limit int64 = 3
	window := time.Minute
	for i := 0; i < 2; i++ {
		allowed, _, takeErr := a.Take(context.Background(), key, limit, window)
		if takeErr != nil {
			return "a:" + takeErr.Error()
		}
		if !allowed {
			return "a-early"
		}
		allowed, _, takeErr = b.Take(context.Background(), key, limit, window)
		if takeErr != nil {
			return "b:" + takeErr.Error()
		}
		if !allowed {
			return "b-early"
		}
	}
	a.Sleep()
	b.Sleep()
	allowed, _, err := a.Take(context.Background(), key, limit, window)
	if err != nil {
		return "share:" + err.Error()
	}
	if allowed {
		return "last-write-wins"
	}
	a.Close()
	b.Close()
	return "ok"
}

// SlidingBoundary fills a window then Takes at the next start and expects the previous hits to still count.
func SlidingBoundary(host, key string) string {
	client := simpleredis.New(simpleredis.Config{Host: host})
	limiter, err := windowcounter.New(client, 0)
	if err != nil {
		return "new:" + err.Error()
	}
	window := 10 * time.Second
	const limit int64 = 2
	start := time.Now().Unix() / 10 * 10
	now := time.Unix(start, 0).Add(9 * time.Second)
	limiter.SetNowForTest(func() time.Time { return now })
	for i := int64(0); i < limit; i++ {
		allowed, _, takeErr := limiter.Take(context.Background(), key, limit, window)
		if takeErr != nil {
			return "fill:" + takeErr.Error()
		}
		if !allowed {
			return "fill-early"
		}
	}
	now = time.Unix(start, 0).Add(window)
	limiter.SetNowForTest(func() time.Time { return now })
	allowed, _, err := limiter.Take(context.Background(), key, limit, window)
	if err != nil {
		return "boundary:" + err.Error()
	}
	if allowed {
		return "doubled"
	}
	return "ok"
}

// PeekThenTake Peeks without incrementing then Takes once and expects estimate 1.
func PeekThenTake(host, key string) string {
	client := simpleredis.New(simpleredis.Config{Host: host})
	limiter, err := windowcounter.New(client, 0)
	if err != nil {
		return "new:" + err.Error()
	}
	const limit int64 = 5
	window := time.Minute
	for i := 0; i < 3; i++ {
		allowed, estimated, peekErr := limiter.Peek(context.Background(), key, limit, window)
		if peekErr != nil {
			return "peek:" + peekErr.Error()
		}
		if !allowed {
			return "peek-denied"
		}
		if estimated != 0 {
			return "peek-est"
		}
	}
	allowed, estimated, err := limiter.Take(context.Background(), key, limit, window)
	if err != nil {
		return "take:" + err.Error()
	}
	if !allowed {
		return "take-denied"
	}
	if estimated != 1 {
		return "take-est"
	}
	return "ok"
}

// PeekDeniedThenSlides fills the window, Peeks denied, then Peeks allowed after the formula cools.
func PeekDeniedThenSlides(host, key string) string {
	client := simpleredis.New(simpleredis.Config{Host: host})
	limiter, err := windowcounter.New(client, 0)
	if err != nil {
		return "new:" + err.Error()
	}
	window := 10 * time.Second
	const limit int64 = 2
	start := time.Unix(1700000000, 0)
	now := start
	limiter.SetNowForTest(func() time.Time { return now })
	for i := int64(0); i < limit+1; i++ {
		if _, _, takeErr := limiter.Take(context.Background(), key, limit, window); takeErr != nil {
			return "fill:" + takeErr.Error()
		}
	}
	allowed, _, err := limiter.Peek(context.Background(), key, limit, window)
	if err != nil {
		return "peek-fill:" + err.Error()
	}
	if allowed {
		return "peek-fill-allowed"
	}
	now = start.Add(window)
	limiter.SetNowForTest(func() time.Time { return now })
	allowed, _, err = limiter.Peek(context.Background(), key, limit, window)
	if err != nil {
		return "peek-boundary:" + err.Error()
	}
	if allowed {
		return "peek-boundary-allowed"
	}
	now = start.Add(window + 4*time.Second)
	limiter.SetNowForTest(func() time.Time { return now })
	allowed, estimated, err := limiter.Peek(context.Background(), key, limit, window)
	if err != nil {
		return "peek-slide:" + err.Error()
	}
	if !allowed {
		return "peek-slide-denied"
	}
	if estimated > float64(limit) {
		return "peek-slide-est"
	}
	return "ok"
}

// BufferedPeek Peeks without incrementing on a buffered limiter, then Takes once.
func BufferedPeek(host, key string) string {
	client := simpleredis.New(simpleredis.Config{Host: host})
	limiter, err := windowcounter.New(client, time.Hour)
	if err != nil {
		return "new:" + err.Error()
	}
	now := time.Unix(1700000000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	const limit int64 = 5
	window := time.Minute
	for i := 0; i < 5; i++ {
		allowed, estimated, peekErr := limiter.Peek(context.Background(), key, limit, window)
		if peekErr != nil {
			return "peek:" + peekErr.Error()
		}
		if !allowed {
			return "peek-denied"
		}
		if estimated != 0 {
			return "peek-est"
		}
	}
	allowed, estimated, err := limiter.Take(context.Background(), key, limit, window)
	if err != nil {
		return "take:" + err.Error()
	}
	if !allowed {
		return "take-denied"
	}
	if estimated != 1 {
		return "take-est"
	}
	limiter.Close()
	return "ok"
}

// ExpireOnFirstHit Takes once then expects a positive TTL on the current window key.
func ExpireOnFirstHit(host, key string) string {
	client := simpleredis.New(simpleredis.Config{Host: host})
	limiter, err := windowcounter.New(client, 0)
	if err != nil {
		return "new:" + err.Error()
	}
	window := 10 * time.Second
	now := time.Unix(1700000000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	if _, _, err := limiter.Take(context.Background(), key, 5, window); err != nil {
		return "take:" + err.Error()
	}
	windowStart := now.Unix() / 10 * 10
	redisKey := key + ":" + strconv.FormatInt(windowStart, 10)
	values, err := client.Eval(context.Background(), "return redis.call('TTL', KEYS[1])", simpleredis.ScriptSHA1Hex("return redis.call('TTL', KEYS[1])"), []string{redisKey}, nil)
	if err != nil {
		return "ttl:" + err.Error()
	}
	if len(values) != 1 {
		return "ttl-slots"
	}
	ttl, err := strconv.ParseInt(string(values[0]), 10, 64)
	if err != nil {
		return "ttl-parse"
	}
	if ttl <= 0 {
		return "ttl-zero"
	}
	return "ok"
}
`
