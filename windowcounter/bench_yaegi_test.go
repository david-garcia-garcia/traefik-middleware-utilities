package windowcounter

import (
	"fmt"
	"testing"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// BenchmarkYaegiTake measures interpreted Take on one hot key in exact mode and at sync_rate 20ms against the fake with a warm script cache.
// Each Eval builds one limiter, Takes count times, then Closes it, so a Close that hangs fails the benchmark by timeout.
func BenchmarkYaegiTake(b *testing.B) {
	for _, mode := range benchTakeModes {
		mode := mode
		b.Run(mode.name+"/hot-key", func(b *testing.B) {
			fake, addr := startTestFakeRedis(b)
			fake.enableScriptCache()
			goPath := b.TempDir()
			writeGopathWindowcounter(b, goPath)
			writeGopathFile(b, goPath, "takebench", "loop.go", takebenchSrc)

			// Interpreter with stdlib only (no unsafe), as Traefik loads a plugin.
			interpreter := interp.New(interp.Options{GoPath: goPath})
			if err := interpreter.Use(stdlib.Symbols); err != nil {
				b.Fatalf("use stdlib: %v", err)
			}
			if _, err := interpreter.Eval(`import "takebench"`); err != nil {
				b.Fatalf("import takebench: %v", err)
			}
			loopExpr := func(count int) string {
				return fmt.Sprintf(`takebench.HotKeyLoop(%q, %d, %d)`, addr, mode.syncRate.Milliseconds(), count)
			}
			if _, err := interpreter.Eval(loopExpr(1)); err != nil {
				b.Fatalf("warmup: %v", err)
			}

			// One timed Eval runs b.N Take calls inside the interpreter.
			startCommands := fake.commandCount()
			b.ResetTimer()
			evaluated, err := interpreter.Eval(loopExpr(b.N))
			if err != nil {
				b.Fatalf("eval loop: %v", err)
			}
			b.StopTimer()
			if got := evaluated.Interface().(string); got != "ok" {
				b.Fatalf("HotKeyLoop = %q, want ok", got)
			}
			reportRedisCmdsPerOp(b, fake, startCommands)
		})
	}
}

// takebenchSrc is the interpreted probe: one limiter per loop, count Takes on one key, then Close.
const takebenchSrc = `package takebench

import (
	"context"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
	"github.com/david-garcia-garcia/traefik-middleware-utilities/windowcounter"
)

// HotKeyLoop runs count Take calls on one key with sync_rate syncRateMs, then Closes the limiter.
func HotKeyLoop(host string, syncRateMs int64, count int) string {
	client, err := simpleredis.New(simpleredis.Config{Host: host})
	if err != nil {
		return "client:" + err.Error()
	}
	limiter, err := windowcounter.New(client, time.Duration(syncRateMs)*time.Millisecond)
	if err != nil {
		return "new:" + err.Error()
	}
	ctx := context.Background()
	for i := 0; i < count; i++ {
		if _, _, err := limiter.Take(ctx, "ip:1.2.3.4", int64(1)<<40, time.Minute); err != nil {
			limiter.Close()
			return "take:" + err.Error()
		}
	}
	limiter.Close()
	return "ok"
}
`
