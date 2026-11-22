package tokenbucket

import (
	"fmt"
	"testing"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// runYaegiAllowBench interprets the allowbench probe and times one Eval of loopExpr(b.N) after a warm-up Eval of loopExpr(1).
// setupExpr, when not empty, runs once before the warm-up and is not timed. redisAddr is the live engine the loop talks to
// ("" for Memory); its round trips are reported as redis-roundtrips/op.
func runYaegiAllowBench(b *testing.B, redisAddr string, setupExpr string, loopExpr func(count int) string) {
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
	var start int64
	if redisAddr != "" {
		start = liveBenchRoundTrips(b, redisAddr)
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
	if redisAddr != "" {
		reportRoundTripsPerOp(b, redisAddr, start)
	}
}

// BenchmarkYaegiMemoryAllow measures interpreted in-process Allow: one hot key (the tail of the expiry list, never moved),
// 10k rotating sources (every Allow moves a stored source), and a never-seen key against a full table of live sources.
func BenchmarkYaegiMemoryAllow(b *testing.B) {
	b.Run("hot-key", func(b *testing.B) {
		runYaegiAllowBench(b, "", "", func(count int) string {
			return fmt.Sprintf(`allowbench.MemoryHotKeyLoop(%d)`, count)
		})
	})
	b.Run("rotating-10k", func(b *testing.B) {
		runYaegiAllowBench(b, "", fmt.Sprintf(`allowbench.SetupMemoryRotating(%d)`, benchRotatingKeys), func(count int) string {
			return fmt.Sprintf(`allowbench.MemoryRotatingLoop(%d)`, count)
		})
	})
	b.Run("new-key-full-table", func(b *testing.B) {
		runYaegiAllowBench(b, "", fmt.Sprintf(`allowbench.SetupMemoryFullTable(%d)`, maxMemorySources), func(count int) string {
			return fmt.Sprintf(`allowbench.MemoryNewKeyLoop(%d)`, count)
		})
	})
}

// BenchmarkYaegiRedisAllow measures interpreted Redis Allow against each live engine (TOKENBUCKET_LIVE_*): one hot key and
// 10k rotating keys, each Allowed once during setup, untimed. It skips under -short or when no engine address is set.
func BenchmarkYaegiRedisAllow(b *testing.B) {
	for _, engine := range liveEngineAddrs(b) {
		engine := engine
		b.Run(engine.name+"/hot-key", func(b *testing.B) {
			runYaegiAllowBench(b, engine.addr, "", func(count int) string {
				return fmt.Sprintf(`allowbench.RedisHotKeyLoop(%q, %d)`, engine.addr, count)
			})
		})
		b.Run(engine.name+"/rotating-10k", func(b *testing.B) {
			runYaegiAllowBench(b, engine.addr, fmt.Sprintf(`allowbench.SetupRedisRotating(%q, %d)`, engine.addr, benchRotatingKeys), func(count int) string {
				return fmt.Sprintf(`allowbench.RedisRotatingLoop(%d)`, count)
			})
		})
	}
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

// benchMemory, benchRedis, benchKeys, and benchNewKeys are the state a Setup function leaves for the timed loop.
var (
	benchMemory  *tokenbucket.Memory
	benchRedis   *tokenbucket.Redis
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

// SetupRedisRotating builds a Redis limiter and Allows keys ip:0..ip:(count-1) once each for RedisRotatingLoop.
func SetupRedisRotating(host string, count int) string {
	client, err := simpleredis.New(simpleredis.Config{Host: host})
	if err != nil {
		return "client:" + err.Error()
	}
	limiter, err := tokenbucket.NewRedis(client, 1000, 100, 5*time.Millisecond, 2*time.Second)
	if err != nil {
		return "new:" + err.Error()
	}
	benchRedis = limiter
	benchKeys = make([]string, count)
	for i := range benchKeys {
		benchKeys[i] = "ip:" + strconv.Itoa(i)
	}
	return RedisRotatingLoop(count)
}

// RedisRotatingLoop runs count Redis Allow calls over the stored keys in turn.
func RedisRotatingLoop(count int) string {
	ctx := context.Background()
	for i := 0; i < count; i++ {
		if _, err := benchRedis.Allow(ctx, benchKeys[i%len(benchKeys)]); err != nil {
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
