package windowcounter

import (
	"fmt"
	"testing"
	"time"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// BenchmarkYaegiTake measures interpreted Take against each live engine (WINDOWCOUNTER_LIVE_*) in every benchTakeModes mode:
// one hot key and 10k rotating keys, the interpreted twins of BenchmarkTake's hot-key and rotating-10k. Setup Takes every
// key once, untimed; the limiter is Closed in cleanup, so a Close that hangs fails the benchmark by timeout. It skips under
// -short or when no engine address is set.
func BenchmarkYaegiTake(b *testing.B) {
	for _, engine := range liveEngineAddrs(b) {
		engine := engine
		for _, mode := range benchTakeModes {
			mode := mode
			b.Run(engine.name+"/"+mode.name+"/hot-key", func(b *testing.B) {
				runYaegiTakeBench(b, engine.addr, mode.syncRate, 1)
			})
			b.Run(engine.name+"/"+mode.name+"/rotating-10k", func(b *testing.B) {
				runYaegiTakeBench(b, engine.addr, mode.syncRate, benchRotatingKeys)
			})
		}
	}
}

// runYaegiTakeBench interprets the takebench probe with keyCount keys at syncRate against the engine at addr and times one
// Eval of b.N Takes.
func runYaegiTakeBench(b *testing.B, addr string, syncRate time.Duration, keyCount int) {
	b.Helper()
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
	evaluated, err := interpreter.Eval(fmt.Sprintf(`takebench.Setup(%q, %d, %d, %q)`, addr, syncRate.Milliseconds(), keyCount, benchRunPrefix()))
	if err != nil {
		b.Fatalf("setup: %v", err)
	}
	if got := evaluated.Interface().(string); got != "ok" {
		b.Fatalf("Setup = %q, want ok", got)
	}
	b.Cleanup(func() { _, _ = interpreter.Eval(`takebench.Close()`) })

	// One timed Eval runs b.N Take calls inside the interpreter.
	start := liveBenchRoundTrips(b, addr)
	b.ResetTimer()
	evaluated, err = interpreter.Eval(fmt.Sprintf(`takebench.Loop(%d)`, b.N))
	if err != nil {
		b.Fatalf("eval loop: %v", err)
	}
	b.StopTimer()
	if got := evaluated.Interface().(string); got != "ok" {
		b.Fatalf("Loop = %q, want ok", got)
	}
	reportRoundTripsPerOp(b, addr, start)
}

// BenchmarkYaegiFlush measures one interpreted buffered flush of 1000 dirty window keys against each live engine
// (WINDOWCOUNTER_LIVE_*). sync_rate is one hour so no timer tick fires. The probe's Flush and Redirty are called from Go as interpreted
// function values, so re-dirtying the keys runs with the timer stopped, as in BenchmarkFlush.
func BenchmarkYaegiFlush(b *testing.B) {
	for _, engine := range liveEngineAddrs(b) {
		addr := engine.addr
		b.Run(engine.name+"/dirty-keys-1000", func(b *testing.B) {
			goPath := b.TempDir()
			writeGopathWindowcounter(b, goPath)
			// Test-only flush hooks, written into the interpreter's copy of the package only.
			writeGopathFile(b, goPath, "github.com/david-garcia-garcia/traefik-middleware-utilities/windowcounter", "zz_bench_hooks.go", flushBenchHooksSrc)
			writeGopathFile(b, goPath, "flushbench", "flush.go", flushbenchSrc)

			// Interpreter with stdlib only (no unsafe), as Traefik loads a plugin.
			interpreter := interp.New(interp.Options{GoPath: goPath})
			if err := interpreter.Use(stdlib.Symbols); err != nil {
				b.Fatalf("use stdlib: %v", err)
			}
			if _, err := interpreter.Eval(`import "flushbench"`); err != nil {
				b.Fatalf("import flushbench: %v", err)
			}
			// Setup Takes every key once and flushes once, so the script is loaded and the timed flushes are one EVALSHA per EVAL.
			evaluated, err := interpreter.Eval(fmt.Sprintf(`flushbench.Setup(%q, %d, %q)`, addr, benchFlushKeys, benchRunPrefix()))
			if err != nil {
				b.Fatalf("setup: %v", err)
			}
			if got := evaluated.Interface().(string); got != "ok" {
				b.Fatalf("setup = %q, want ok", got)
			}
			b.Cleanup(func() { _, _ = interpreter.Eval(`flushbench.Close()`) })
			redirty := interpretedFuncForBench(b, interpreter, `flushbench.Redirty`).(func())
			flush := interpretedFuncForBench(b, interpreter, `flushbench.Flush`).(func() string)

			// Each timed iteration is one interpreted flush; the re-dirty before it is untimed.
			b.ResetTimer()
			start := liveBenchRoundTrips(b, addr)
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				redirty()
				b.StartTimer()
				if got := flush(); got != "ok" {
					b.Fatalf("Flush = %q, want ok", got)
				}
			}
			b.StopTimer()
			reportRoundTripsPerOp(b, addr, start)
		})
	}
}

// interpretedFuncForBench evaluates name to an interpreted function and returns it as a Go func value.
func interpretedFuncForBench(b *testing.B, interpreter *interp.Interpreter, name string) any {
	b.Helper()
	evaluated, err := interpreter.Eval(name)
	if err != nil {
		b.Fatalf("eval %s: %v", name, err)
	}
	return evaluated.Interface()
}

// flushBenchHooksSrc is a test-only file written into the interpreter's copy of package windowcounter (never into the product
// package): it exposes the in-package flush and the re-dirty step BenchmarkFlush uses, so the interpreted probe can call them.
const flushBenchHooksSrc = `package windowcounter

import "context"

// DirtyWindowKeysForBench returns the Redis window keys that hold a pending local delta.
func DirtyWindowKeysForBench(l *Limiter) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var dirty []string
	for redisKey, state := range l.windows {
		if state.localDelta > 0 {
			dirty = append(dirty, redisKey)
		}
	}
	return dirty
}

// RedirtyWindowsForBench gives each listed window one pending hit without a Redis round trip.
func RedirtyWindowsForBench(l *Limiter, redisKeys []string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, redisKey := range redisKeys {
		l.windows[redisKey].localDelta++
	}
}

// FlushForBench runs one buffered flush.
func FlushForBench(l *Limiter) error {
	return l.flushPending(context.Background())
}
`

// flushbenchSrc is the interpreted probe for BenchmarkYaegiFlush: Setup leaves one buffered limiter with keyCount dirty windows.
const flushbenchSrc = `package flushbench

import (
	"context"
	"strconv"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
	"github.com/david-garcia-garcia/traefik-middleware-utilities/windowcounter"
)

// benchLimiter and benchDirty are the state Setup leaves for Redirty and Flush.
var (
	benchLimiter *windowcounter.Limiter
	benchDirty   []string
)

// Setup builds a buffered limiter on a clock frozen at the current time, Takes keyCount keys prefix0..prefix(keyCount-1)
// once, records their windows, and flushes once.
func Setup(host string, keyCount int, prefix string) string {
	client, err := simpleredis.New(simpleredis.Config{Host: host})
	if err != nil {
		return "client:" + err.Error()
	}
	limiter, err := windowcounter.New(client, time.Hour)
	if err != nil {
		return "new:" + err.Error()
	}
	now := time.Now()
	limiter.SetNowForTest(func() time.Time { return now })
	ctx := context.Background()
	for i := 0; i < keyCount; i++ {
		if _, err := limiter.Take(ctx, prefix+strconv.Itoa(i), int64(1)<<40, time.Minute); err != nil {
			return "take:" + err.Error()
		}
	}
	benchDirty = windowcounter.DirtyWindowKeysForBench(limiter)
	if len(benchDirty) != keyCount {
		return "dirty:" + strconv.Itoa(len(benchDirty))
	}
	if err := windowcounter.FlushForBench(limiter); err != nil {
		return "flush:" + err.Error()
	}
	benchLimiter = limiter
	return "ok"
}

// Redirty gives every recorded window one pending hit.
func Redirty() {
	windowcounter.RedirtyWindowsForBench(benchLimiter, benchDirty)
}

// Flush runs one buffered flush of the recorded windows.
func Flush() string {
	if err := windowcounter.FlushForBench(benchLimiter); err != nil {
		return "flush:" + err.Error()
	}
	return "ok"
}

// Close stops the limiter.
func Close() {
	benchLimiter.Close()
}
`

// takebenchSrc is the interpreted probe: Setup builds one limiter and Takes every key once, Loop Takes the keys in turn.
const takebenchSrc = `package takebench

import (
	"context"
	"strconv"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
	"github.com/david-garcia-garcia/traefik-middleware-utilities/windowcounter"
)

// benchLimiter and benchKeys are the state Setup leaves for Loop and Close.
var (
	benchLimiter *windowcounter.Limiter
	benchKeys    []string
)

// Setup builds a limiter with sync_rate syncRateMs and Takes keys prefix0..prefix(keyCount-1) once each.
func Setup(host string, syncRateMs int64, keyCount int, prefix string) string {
	client, err := simpleredis.New(simpleredis.Config{Host: host})
	if err != nil {
		return "client:" + err.Error()
	}
	limiter, err := windowcounter.New(client, time.Duration(syncRateMs)*time.Millisecond)
	if err != nil {
		return "new:" + err.Error()
	}
	benchLimiter = limiter
	benchKeys = make([]string, keyCount)
	for i := range benchKeys {
		benchKeys[i] = prefix + strconv.Itoa(i)
	}
	return Loop(keyCount)
}

// Loop runs count Take calls over the keys in turn.
func Loop(count int) string {
	ctx := context.Background()
	for i := 0; i < count; i++ {
		if _, err := benchLimiter.Take(ctx, benchKeys[i%len(benchKeys)], int64(1)<<40, time.Minute); err != nil {
			return "take:" + err.Error()
		}
	}
	return "ok"
}

// Close stops the limiter.
func Close() {
	if benchLimiter != nil {
		benchLimiter.Close()
	}
}
`
