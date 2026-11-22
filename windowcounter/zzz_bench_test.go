package windowcounter

import (
	"context"
	"strconv"
	"testing"
	"time"
)

// Benchmark Take settings: a limit no benchmark reaches, so every Take admits, on a one-minute window.
const (
	benchLimit        = int64(1) << 40
	benchWindow       = time.Minute
	benchHotKey       = "ip:1.2.3.4"
	benchRotatingKeys = 10000
	benchFlushKeys    = 1000
	benchSyncRate     = minSyncRate
)

// benchTakeMode is one sync_rate the Take benchmarks run under.
type benchTakeMode struct {
	name     string
	syncRate time.Duration
}

// benchTakeModes are exact mode, the 20ms floor (the buffered rate with the most Redis traffic), and a one-second buffered rate.
var benchTakeModes = []benchTakeMode{
	{name: "sync-rate-0", syncRate: 0},
	{name: "sync-rate-20ms", syncRate: benchSyncRate},
	{name: "sync-rate-1s", syncRate: time.Second},
}

// benchKeyNames returns count distinct opaque keys with prefix.
func benchKeyNames(prefix string, count int) []string {
	names := make([]string, count)
	for i := range names {
		names[i] = prefix + strconv.Itoa(i)
	}
	return names
}

// newBenchLimiter builds a limiter on a fake with a warm script cache. Close runs after the benchmark, before the fake stops.
func newBenchLimiter(tb testing.TB, syncRate time.Duration) (*Limiter, *testFakeRedis) {
	tb.Helper()
	fake, addr := startTestFakeRedis(tb)
	fake.enableScriptCache()
	limiter, err := New(newSimpleRedisForTest(tb, addr), syncRate)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(limiter.Close)
	return limiter, fake
}

// reportRedisCmdsPerOp reports the fake's served commands since startCommands divided by b.N.
func reportRedisCmdsPerOp(b *testing.B, fake *testFakeRedis, startCommands int) {
	b.Helper()
	b.ReportMetric(float64(fake.commandCount()-startCommands)/float64(b.N), "redis-cmds/op")
}

// benchStatusSink keeps each timed Take's Status alive so the compiler cannot drop building it.
var benchStatusSink Status

// takeForBench is one Take that fails the benchmark on error and stores its Status in benchStatusSink.
func takeForBench(b *testing.B, limiter *Limiter, key string) {
	b.Helper()
	status, err := limiter.Take(context.Background(), key, benchLimit, benchWindow)
	if err != nil {
		b.Fatal(err)
	}
	benchStatusSink = status
}

// BenchmarkTake measures Take in exact mode and at sync_rate 20ms and 1s against the fake with a warm script cache:
// one hot key, 10k rotating keys, and a hot key from parallel goroutines. The buffered flush timer runs for real in the background.
func BenchmarkTake(b *testing.B) {
	for _, mode := range benchTakeModes {
		mode := mode
		b.Run(mode.name+"/hot-key", func(b *testing.B) {
			limiter, fake := newBenchLimiter(b, mode.syncRate)
			takeForBench(b, limiter, benchHotKey)
			b.ReportAllocs()
			b.ResetTimer()
			startCommands := fake.commandCount()
			for i := 0; i < b.N; i++ {
				takeForBench(b, limiter, benchHotKey)
			}
			b.StopTimer()
			reportRedisCmdsPerOp(b, fake, startCommands)
		})
		b.Run(mode.name+"/rotating-10k", func(b *testing.B) {
			limiter, fake := newBenchLimiter(b, mode.syncRate)
			keys := benchKeyNames("ip:", benchRotatingKeys)
			for _, key := range keys {
				takeForBench(b, limiter, key)
			}
			b.ReportAllocs()
			b.ResetTimer()
			startCommands := fake.commandCount()
			for i := 0; i < b.N; i++ {
				takeForBench(b, limiter, keys[i%len(keys)])
			}
			b.StopTimer()
			reportRedisCmdsPerOp(b, fake, startCommands)
		})
		b.Run(mode.name+"/hot-key-parallel", func(b *testing.B) {
			limiter, fake := newBenchLimiter(b, mode.syncRate)
			takeForBench(b, limiter, benchHotKey)
			b.ReportAllocs()
			b.ResetTimer()
			startCommands := fake.commandCount()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					if _, err := limiter.Take(context.Background(), benchHotKey, benchLimit, benchWindow); err != nil {
						b.Error(err)
						return
					}
				}
			})
			b.StopTimer()
			reportRedisCmdsPerOp(b, fake, startCommands)
		})
	}
}

// BenchmarkFlush measures one buffered flush of 1000 dirty window keys against the fake with a warm script cache.
// sync_rate is one hour so no timer tick fires; the keys are re-dirtied in memory with the timer stopped.
func BenchmarkFlush(b *testing.B) {
	b.Run("dirty-keys-1000", func(b *testing.B) {
		limiter, fake := newBenchLimiter(b, time.Hour)
		now := time.Unix(1_700_000_000, 0)
		limiter.SetNowForTest(func() time.Time { return now })
		for _, key := range benchKeyNames("ip:", benchFlushKeys) {
			takeForBench(b, limiter, key)
		}
		dirtyKeys := dirtyWindowKeysForBench(limiter)
		if len(dirtyKeys) != benchFlushKeys {
			b.Fatalf("dirty windows %d, want %d", len(dirtyKeys), benchFlushKeys)
		}
		ctx := context.Background()
		// First flush loads the script, so timed flushes are one EVALSHA per EVAL.
		if err := limiter.flushPending(ctx); err != nil {
			b.Fatal(err)
		}
		b.ReportAllocs()
		b.ResetTimer()
		startCommands := fake.commandCount()
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			redirtyWindowsForBench(limiter, dirtyKeys)
			b.StartTimer()
			if err := limiter.flushPending(ctx); err != nil {
				b.Fatal(err)
			}
		}
		b.StopTimer()
		reportRedisCmdsPerOp(b, fake, startCommands)
	})
}

// dirtyWindowKeysForBench returns the Redis window keys that hold a pending local delta.
func dirtyWindowKeysForBench(limiter *Limiter) []string {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	var dirty []string
	for redisKey, state := range limiter.windows {
		if state.localDelta > 0 {
			dirty = append(dirty, redisKey)
		}
	}
	return dirty
}

// redirtyWindowsForBench gives each listed window one pending hit, as one Take per key would, without a Redis round trip.
func redirtyWindowsForBench(limiter *Limiter, redisKeys []string) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	for _, redisKey := range redisKeys {
		limiter.windows[redisKey].localDelta++
	}
}
