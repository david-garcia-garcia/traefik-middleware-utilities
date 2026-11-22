package tokenbucket

import (
	"fmt"
	"testing"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// runYaegiAllowBench interprets the allowbench probe and times one Eval of loopExpr(b.N) after a warm-up Eval of loopExpr(1).
// setupExpr, when not empty, runs once before the warm-up and is not timed. fake is the Redis the loop talks to (nil for
// Memory); its served commands are reported as redis-cmds/op.
func runYaegiAllowBench(b *testing.B, fake *testFakeRedis, setupExpr string, loopExpr func(count int) string) {
	b.Helper()
	goPath := b.TempDir()
	writeGopathTokenbucket(b, goPath)
	// Test-only prefill hook, written into the interpreter's copy of the package only.
	writeGopathFile(b, goPath, "github.com/david-garcia-garcia/traefik-middleware-utilities/tokenbucket", "zz_bench_hooks.go", memoryBenchHooksSrc+storeBenchEntrySrc)
	writeGopathFile(b, goPath, "allowbench", "loop.go", allowbenchSrc)

	// Interpreter with stdlib only (no unsafe), as Traefik loads a plugin.
	interpreter := interp.New(interp.Options{GoPath: goPath})
	if err := interpreter.Use(stdlib.Symbols); err != nil {
		b.Fatalf("use stdlib: %v", err)
	}
	if _, err := interpreter.Eval(`import "allowbench"`); err != nil {
		b.Fatalf("import allowbench: %v", err)
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

// BenchmarkYaegiMemoryAllow measures interpreted in-process Allow: one hot key (the tail of the expiry list, never moved),
// 10k rotating sources (every Allow moves a stored source), and a never-seen key against a full table of live sources.
func BenchmarkYaegiMemoryAllow(b *testing.B) {
	b.Run("hot-key", func(b *testing.B) {
		runYaegiAllowBench(b, nil, "", func(count int) string {
			return fmt.Sprintf(`allowbench.MemoryHotKeyLoop(%d)`, count)
		})
	})
	b.Run("rotating-10k", func(b *testing.B) {
		runYaegiAllowBench(b, nil, fmt.Sprintf(`allowbench.SetupMemoryRotating(%d)`, benchRotatingKeys), func(count int) string {
			return fmt.Sprintf(`allowbench.MemoryRotatingLoop(%d)`, count)
		})
	})
	b.Run("new-key-full-table", func(b *testing.B) {
		runYaegiAllowBench(b, nil, fmt.Sprintf(`allowbench.SetupMemoryFullTable(%d)`, maxMemorySources), func(count int) string {
			return fmt.Sprintf(`allowbench.MemoryNewKeyLoop(%d)`, count)
		})
	})
}

// BenchmarkYaegiRedisAllow measures interpreted Redis Allow on one hot key against the fake with a warm script cache.
func BenchmarkYaegiRedisAllow(b *testing.B) {
	b.Run("hot-key", func(b *testing.B) {
		fake, addr := startTestFakeRedis(b)
		fake.enableScriptCache()
		runYaegiAllowBench(b, fake, "", func(count int) string {
			return fmt.Sprintf(`allowbench.RedisHotKeyLoop(%q, %d)`, addr, count)
		})
	})
}

// allowbenchSrc is the interpreted probe: each loop builds one limiter with the bench settings and Allows count times.
const allowbenchSrc = `package allowbench

import (
	"context"
	"strconv"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
	"github.com/david-garcia-garcia/traefik-middleware-utilities/tokenbucket"
)

// benchMemory, benchKeys, and benchNewKeys are the state a Setup function leaves for the timed loop.
var (
	benchMemory  *tokenbucket.Memory
	benchKeys    []string
	benchNewKeys int
)

// newBenchMemory builds an in-process limiter with the bench settings.
func newBenchMemory() (*tokenbucket.Memory, string) {
	limiter, err := tokenbucket.NewMemory(1000, 100, 5*time.Millisecond, 2*time.Second)
	if err != nil {
		return nil, "new:" + err.Error()
	}
	return limiter, ""
}

// SetupMemoryRotating stores count live sources ip:0..ip:(count-1) on the real clock for MemoryRotatingLoop.
func SetupMemoryRotating(count int) string {
	limiter, msg := newBenchMemory()
	if msg != "" {
		return msg
	}
	tokenbucket.PrefillForBench(limiter, "ip:", count, time.Now())
	benchKeys = make([]string, count)
	for i := range benchKeys {
		benchKeys[i] = "ip:" + strconv.Itoa(i)
	}
	benchMemory = limiter
	return "ok"
}

// MemoryRotatingLoop runs count Allow calls over the stored sources in turn.
func MemoryRotatingLoop(count int) string {
	ctx := context.Background()
	for i := 0; i < count; i++ {
		if _, err := benchMemory.Allow(ctx, benchKeys[i%len(benchKeys)]); err != nil {
			return "allow:" + err.Error()
		}
	}
	return "ok"
}

// SetupMemoryFullTable fills the store with count live sources on a frozen clock for MemoryNewKeyLoop.
func SetupMemoryFullTable(count int) string {
	limiter, msg := newBenchMemory()
	if msg != "" {
		return msg
	}
	now := time.Unix(1700000000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	tokenbucket.PrefillForBench(limiter, "live:", count, now)
	benchMemory = limiter
	return "ok"
}

// MemoryNewKeyLoop runs count Allow calls, each on a never-seen key, so every one evicts a live source.
func MemoryNewKeyLoop(count int) string {
	ctx := context.Background()
	for i := 0; i < count; i++ {
		benchNewKeys++
		if _, err := benchMemory.Allow(ctx, "new:"+strconv.Itoa(benchNewKeys)); err != nil {
			return "allow:" + err.Error()
		}
	}
	return "ok"
}

// MemoryHotKeyLoop runs count in-process Allow calls on one key.
func MemoryHotKeyLoop(count int) string {
	limiter, err := tokenbucket.NewMemory(1000, 100, 5*time.Millisecond, 2*time.Second)
	if err != nil {
		return "new:" + err.Error()
	}
	ctx := context.Background()
	for i := 0; i < count; i++ {
		if _, err := limiter.Allow(ctx, "ip:1.2.3.4"); err != nil {
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
		if _, err := limiter.Allow(ctx, "ip:1.2.3.4"); err != nil {
			return "allow:" + err.Error()
		}
	}
	return "ok"
}
`

// memoryBenchHooksSrc is a test-only file written into the interpreter's copy of package tokenbucket (never into the product
// package): it prefills a Memory without paying Allow's new-source cost, the interpreted twin of prefillBenchMemory.
const memoryBenchHooksSrc = `package tokenbucket

import (
	"strconv"
	"time"
)

// PrefillForBench stores count full buckets named prefix0..prefix(count-1) that expire one ttl after now.
func PrefillForBench(m *Memory, prefix string, count int, now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := 0; i < count; i++ {
		entry := &memEntry{tokens: float64(m.clock.burst), last: now.UnixMicro(), expireAt: now.Add(m.clock.ttl)}
		storeBenchEntryLocked(m, prefix+strconv.Itoa(i), entry)
	}
}
`

// storeBenchEntrySrc stores one entry the way Allow stores a new source, matching storeMemEntryForTest.
const storeBenchEntrySrc = `
// storeBenchEntryLocked stores entry under key in the map and at the tail of the expiry list. Caller holds m.mu.
func storeBenchEntryLocked(m *Memory, key string, entry *memEntry) {
	entry.key = key
	m.storeLocked(entry)
}
`
