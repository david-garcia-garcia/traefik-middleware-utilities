package tokenbucket

import (
	"fmt"
	"testing"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// runYaegiAllowBench interprets the allowbench probe and times one Eval of loopExpr(b.N) after a warm-up Eval of loopExpr(1).
// fake is the Redis the loop talks to (nil for Memory); its served commands are reported as redis-cmds/op.
func runYaegiAllowBench(b *testing.B, fake *testFakeRedis, loopExpr func(count int) string) {
	b.Helper()
	goPath := b.TempDir()
	writeGopathTokenbucket(b, goPath)
	writeGopathFile(b, goPath, "allowbench", "loop.go", allowbenchSrc)

	// Interpreter with stdlib only (no unsafe), as Traefik loads a plugin.
	interpreter := interp.New(interp.Options{GoPath: goPath})
	if err := interpreter.Use(stdlib.Symbols); err != nil {
		b.Fatalf("use stdlib: %v", err)
	}
	if _, err := interpreter.Eval(`import "allowbench"`); err != nil {
		b.Fatalf("import allowbench: %v", err)
	}
	if _, err := interpreter.Eval(loopExpr(1)); err != nil {
		b.Fatalf("warmup: %v", err)
	}

	// One timed Eval runs b.N Allow calls inside the interpreter.
	startCommands := 0
	if fake != nil {
		startCommands = fake.commandCount()
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
	if fake != nil {
		reportRedisCmdsPerOp(b, fake, startCommands)
	}
}

// BenchmarkYaegiMemoryAllow measures interpreted in-process Allow on one hot key.
func BenchmarkYaegiMemoryAllow(b *testing.B) {
	b.Run("hot-key", func(b *testing.B) {
		runYaegiAllowBench(b, nil, func(count int) string {
			return fmt.Sprintf(`allowbench.MemoryHotKeyLoop(%d)`, count)
		})
	})
}

// BenchmarkYaegiRedisAllow measures interpreted Redis Allow on one hot key against the fake with a warm script cache.
func BenchmarkYaegiRedisAllow(b *testing.B) {
	b.Run("hot-key", func(b *testing.B) {
		fake, addr := startTestFakeRedis(b)
		fake.enableScriptCache()
		runYaegiAllowBench(b, fake, func(count int) string {
			return fmt.Sprintf(`allowbench.RedisHotKeyLoop(%q, %d)`, addr, count)
		})
	})
}

// allowbenchSrc is the interpreted probe: each loop builds one limiter with the bench settings and Allows count times.
const allowbenchSrc = `package allowbench

import (
	"context"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
	"github.com/david-garcia-garcia/traefik-middleware-utilities/tokenbucket"
)

// MemoryHotKeyLoop runs count in-process Allow calls on one key.
func MemoryHotKeyLoop(count int) string {
	limiter, err := tokenbucket.NewMemory(1000, 100, 5*time.Millisecond, 2*time.Second)
	if err != nil {
		return "new:" + err.Error()
	}
	ctx := context.Background()
	for i := 0; i < count; i++ {
		if _, _, err := limiter.Allow(ctx, "ip:1.2.3.4"); err != nil {
			return "allow:" + err.Error()
		}
	}
	return "ok"
}

// RedisHotKeyLoop runs count Redis Allow calls on one key through one client.
func RedisHotKeyLoop(host string, count int) string {
	client, err := simpleredis.New(simpleredis.Config{Host: host})
	if err != nil {
		return "client:" + err.Error()
	}
	limiter, err := tokenbucket.NewRedis(client, 1000, 100, 5*time.Millisecond, 2*time.Second)
	if err != nil {
		return "new:" + err.Error()
	}
	ctx := context.Background()
	for i := 0; i < count; i++ {
		if _, _, err := limiter.Allow(ctx, "ip:1.2.3.4"); err != nil {
			return "allow:" + err.Error()
		}
	}
	return "ok"
}
`
