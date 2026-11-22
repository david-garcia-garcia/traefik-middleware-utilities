package windowcounter

import (
	"fmt"
	"testing"
	"time"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// BenchmarkYaegiTake measures interpreted Take in every benchTakeModes mode against the fake with a warm script cache:
// one hot key and 10k rotating keys, the interpreted twins of BenchmarkTake's hot-key and rotating-10k. Setup Takes every
// key once, untimed; the limiter is Closed in cleanup, so a Close that hangs fails the benchmark by timeout.
func BenchmarkYaegiTake(b *testing.B) {
	for _, mode := range benchTakeModes {
		mode := mode
		b.Run(mode.name+"/hot-key", func(b *testing.B) {
			runYaegiTakeBench(b, mode.syncRate, 1)
		})
		b.Run(mode.name+"/rotating-10k", func(b *testing.B) {
			runYaegiTakeBench(b, mode.syncRate, benchRotatingKeys)
		})
	}
}

// runYaegiTakeBench interprets the takebench probe with keyCount keys at syncRate and times one Eval of b.N Takes.
func runYaegiTakeBench(b *testing.B, syncRate time.Duration, keyCount int) {
	b.Helper()
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
	evaluated, err := interpreter.Eval(fmt.Sprintf(`takebench.Setup(%q, %d, %d)`, addr, syncRate.Milliseconds(), keyCount))
	if err != nil {
		b.Fatalf("setup: %v", err)
	}
	if got := evaluated.Interface().(string); got != "ok" {
		b.Fatalf("Setup = %q, want ok", got)
	}
	b.Cleanup(func() { _, _ = interpreter.Eval(`takebench.Close()`) })

	// One timed Eval runs b.N Take calls inside the interpreter.
	startCommands := fake.commandCount()
	b.ResetTimer()
	evaluated, err = interpreter.Eval(fmt.Sprintf(`takebench.Loop(%d)`, b.N))
	if err != nil {
		b.Fatalf("eval loop: %v", err)
	}
	b.StopTimer()
	if got := evaluated.Interface().(string); got != "ok" {
		b.Fatalf("Loop = %q, want ok", got)
	}
	reportRedisCmdsPerOp(b, fake, startCommands)
}

// BenchmarkYaegiFlush measures one interpreted buffered flush of 1000 dirty window keys against the fake with a warm script
// cache. sync_rate is one hour so no timer tick fires. The probe's Flush and Redirty are called from Go as interpreted
// function values, so re-dirtying the keys runs with the timer stopped, as in BenchmarkFlush.
func BenchmarkYaegiFlush(b *testing.B) {
	b.Run("dirty-keys-1000", func(b *testing.B) {
		fake, addr := startTestFakeRedis(b)
		fake.enableScriptCache()
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
		evaluated, err := interpreter.Eval(fmt.Sprintf(`flushbench.Setup(%q, %d)`, addr, benchFlushKeys))
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
		startCommands := fake.commandCount()
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			redirty()
			b.StartTimer()
			if got := flush(); got != "ok" {
				b.Fatalf("Flush = %q, want ok", got)
			}
		}
		b.StopTimer()
		reportRedisCmdsPerOp(b, fake, startCommands)
	})
}

// BenchmarkYaegiFlushIdle measures one interpreted idle flush with no dirty windows and no due expiry, at 10k and 200k live windows.
func BenchmarkYaegiFlushIdle(b *testing.B) {
	for _, live := range []struct {
		name  string
		count int
	}{
		{name: "10k", count: 10_000},
		{name: "200k", count: 200_000},
	} {
		live := live
		b.Run(live.name, func(b *testing.B) {
			fake, addr := startTestFakeRedis(b)
			fake.enableScriptCache()
			goPath := b.TempDir()
			writeGopathWindowcounter(b, goPath)
			writeGopathFile(b, goPath, "github.com/david-garcia-garcia/traefik-middleware-utilities/windowcounter", "zz_bench_hooks.go", flushBenchHooksSrc)
			writeGopathFile(b, goPath, "idlebench", "idle.go", idlebenchSrc)

			interpreter := interp.New(interp.Options{GoPath: goPath})
			if err := interpreter.Use(stdlib.Symbols); err != nil {
				b.Fatalf("use stdlib: %v", err)
			}
			if _, err := interpreter.Eval(`import "idlebench"`); err != nil {
				b.Fatalf("import idlebench: %v", err)
			}
			evaluated, err := interpreter.Eval(fmt.Sprintf(`idlebench.Setup(%q, %d)`, addr, live.count))
			if err != nil {
				b.Fatalf("setup: %v", err)
			}
			if got := evaluated.Interface().(string); got != "ok" {
				b.Fatalf("setup = %q, want ok", got)
			}
			b.Cleanup(func() { _, _ = interpreter.Eval(`idlebench.Close()`) })
			flush := interpretedFuncForBench(b, interpreter, `idlebench.Flush`).(func() string)

			b.ResetTimer()
			startCommands := fake.commandCount()
			for i := 0; i < b.N; i++ {
				if got := flush(); got != "ok" {
					b.Fatalf("Flush = %q, want ok", got)
				}
			}
			b.StopTimer()
			reportRedisCmdsPerOp(b, fake, startCommands)
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

import (
	"context"
	"strconv"
)

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
		state := l.windows[redisKey]
		if state.localDelta == 0 {
			l.dirty[redisKey] = state
		}
		state.localDelta++
	}
}

// FlushForBench runs one buffered flush.
func FlushForBench(l *Limiter) error {
	return l.flushPending(context.Background())
}

// SeedIdleWindowsForBench inserts count idle windows sharing expireAt.
func SeedIdleWindowsForBench(l *Limiter, count int, expireAt int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := 0; i < count; i++ {
		l.putWindowLocked("idle:"+strconv.Itoa(i), &windowState{expireAt: expireAt})
	}
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

// Setup builds a buffered limiter on a frozen clock, Takes keyCount keys once, records their windows, and flushes once.
func Setup(host string, keyCount int) string {
	client, err := simpleredis.New(simpleredis.Config{Host: host})
	if err != nil {
		return "client:" + err.Error()
	}
	limiter, err := windowcounter.New(client, time.Hour)
	if err != nil {
		return "new:" + err.Error()
	}
	now := time.Unix(1700000000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	ctx := context.Background()
	for i := 0; i < keyCount; i++ {
		if _, err := limiter.Take(ctx, "ip:"+strconv.Itoa(i), int64(1)<<40, time.Minute); err != nil {
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

// idlebenchSrc is the interpreted probe for BenchmarkYaegiFlushIdle: Setup leaves one buffered limiter with count idle windows.
const idlebenchSrc = `package idlebench

import (
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
	"github.com/david-garcia-garcia/traefik-middleware-utilities/windowcounter"
)

// benchLimiter is the state Setup leaves for Flush.
var benchLimiter *windowcounter.Limiter

// Setup builds a buffered limiter on a frozen clock and seeds count idle windows that are not due.
func Setup(host string, count int) string {
	client, err := simpleredis.New(simpleredis.Config{Host: host})
	if err != nil {
		return "client:" + err.Error()
	}
	limiter, err := windowcounter.New(client, time.Hour)
	if err != nil {
		return "new:" + err.Error()
	}
	now := time.Unix(1700000000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	windowcounter.SeedIdleWindowsForBench(limiter, count, now.Unix()+3600)
	if err := windowcounter.FlushForBench(limiter); err != nil {
		return "flush:" + err.Error()
	}
	benchLimiter = limiter
	return "ok"
}

// Flush runs one idle buffered flush.
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

// Setup builds a limiter with sync_rate syncRateMs and Takes keys ip:0..ip:(keyCount-1) once each.
func Setup(host string, syncRateMs int64, keyCount int) string {
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
		benchKeys[i] = "ip:" + strconv.Itoa(i)
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
