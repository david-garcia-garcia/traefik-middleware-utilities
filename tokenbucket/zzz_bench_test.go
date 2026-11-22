package tokenbucket

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// Benchmark limiter settings: a Traefik-style average of 1000 req/s with a burst of 100.
const (
	benchRate         = 1000
	benchBurst        = 100
	benchMaxDelay     = 5 * time.Millisecond
	benchTTL          = 2 * time.Second
	benchHotKey       = "ip:1.2.3.4"
	benchRotatingKeys = 10000
)

// benchKeyNames returns count distinct source keys with prefix.
func benchKeyNames(prefix string, count int) []string {
	names := make([]string, count)
	for i := range names {
		names[i] = prefix + strconv.Itoa(i)
	}
	return names
}

// newBenchMemory builds the in-process limiter every Memory benchmark uses.
func newBenchMemory(tb testing.TB) *Memory {
	tb.Helper()
	limiter, err := NewMemory(benchRate, benchBurst, benchMaxDelay, benchTTL)
	if err != nil {
		tb.Fatal(err)
	}
	return limiter
}

// prefillBenchMemory stores one full bucket per key as of now, without paying Allow's new-source cost.
func prefillBenchMemory(limiter *Memory, keys []string, now time.Time) {
	for _, key := range keys {
		storeMemEntryForTest(limiter, key, &memEntry{
			tokens:   benchBurst,
			last:     now.UnixMicro(),
			expireAt: now.Add(benchTTL),
		})
	}
}

// newBenchRedis builds a Redis limiter on a fake with a warm script cache and loads the script once.
func newBenchRedis(tb testing.TB) (*Redis, *testFakeRedis) {
	tb.Helper()
	fake, addr := startTestFakeRedis(tb)
	fake.enableScriptCache()
	client, err := simpleredis.New(simpleredis.Config{Host: addr})
	if err != nil {
		tb.Fatal(err)
	}
	limiter, err := NewRedis(client, benchRate, benchBurst, benchMaxDelay, benchTTL)
	if err != nil {
		tb.Fatal(err)
	}
	// First Eval is EVALSHA NOSCRIPT then EVAL; later Evals are one EVALSHA, like a warm engine.
	if _, err := limiter.Allow(context.Background(), "warm-up"); err != nil {
		tb.Fatal(err)
	}
	return limiter, fake
}

// reportRedisCmdsPerOp reports the fake's served commands since startCommands divided by b.N.
func reportRedisCmdsPerOp(b *testing.B, fake *testFakeRedis, startCommands int) {
	b.Helper()
	b.ReportMetric(float64(fake.commandCount()-startCommands)/float64(b.N), "redis-cmds/op")
}

// benchStatusSink keeps each timed Allow's Status alive so the compiler cannot drop building it.
var benchStatusSink Status

// BenchmarkMemoryAllow measures in-process Allow: one hot key, 10k rotating keys, a never-seen key against a full table of live sources, and a hot key from parallel goroutines.
func BenchmarkMemoryAllow(b *testing.B) {
	ctx := context.Background()
	b.Run("hot-key", func(b *testing.B) {
		limiter := newBenchMemory(b)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			status, err := limiter.Allow(ctx, benchHotKey)
			if err != nil {
				b.Fatal(err)
			}
			benchStatusSink = status
		}
	})
	b.Run("rotating-10k", func(b *testing.B) {
		limiter := newBenchMemory(b)
		keys := benchKeyNames("ip:", benchRotatingKeys)
		prefillBenchMemory(limiter, keys, time.Now())
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			status, err := limiter.Allow(ctx, keys[i%len(keys)])
			if err != nil {
				b.Fatal(err)
			}
			benchStatusSink = status
		}
	})
	b.Run("new-key-full-table", func(b *testing.B) {
		// Frozen clock: no stored source ever expires, so every new key evicts a live one.
		limiter := newBenchMemory(b)
		now := time.Unix(1_700_000_000, 0)
		limiter.SetNowForTest(func() time.Time { return now })
		prefillBenchMemory(limiter, benchKeyNames("live:", maxMemorySources), now)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			status, err := limiter.Allow(ctx, "new:"+strconv.Itoa(i))
			if err != nil {
				b.Fatal(err)
			}
			benchStatusSink = status
		}
	})
	b.Run("hot-key-parallel", func(b *testing.B) {
		limiter := newBenchMemory(b)
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if _, err := limiter.Allow(ctx, benchHotKey); err != nil {
					b.Error(err)
					return
				}
			}
		})
	})
}

// BenchmarkRedisAllow measures Redis Allow against the in-process fake with a warm script cache: one hot key, 10k rotating keys, and a hot key from parallel goroutines.
func BenchmarkRedisAllow(b *testing.B) {
	ctx := context.Background()
	b.Run("hot-key", func(b *testing.B) {
		limiter, fake := newBenchRedis(b)
		b.ReportAllocs()
		b.ResetTimer()
		startCommands := fake.commandCount()
		for i := 0; i < b.N; i++ {
			status, err := limiter.Allow(ctx, benchHotKey)
			if err != nil {
				b.Fatal(err)
			}
			benchStatusSink = status
		}
		b.StopTimer()
		reportRedisCmdsPerOp(b, fake, startCommands)
	})
	b.Run("rotating-10k", func(b *testing.B) {
		limiter, fake := newBenchRedis(b)
		keys := benchKeyNames("ip:", benchRotatingKeys)
		for _, key := range keys {
			if _, err := limiter.Allow(ctx, key); err != nil {
				b.Fatal(err)
			}
		}
		b.ReportAllocs()
		b.ResetTimer()
		startCommands := fake.commandCount()
		for i := 0; i < b.N; i++ {
			status, err := limiter.Allow(ctx, keys[i%len(keys)])
			if err != nil {
				b.Fatal(err)
			}
			benchStatusSink = status
		}
		b.StopTimer()
		reportRedisCmdsPerOp(b, fake, startCommands)
	})
	b.Run("hot-key-parallel", func(b *testing.B) {
		limiter, fake := newBenchRedis(b)
		b.ReportAllocs()
		b.ResetTimer()
		startCommands := fake.commandCount()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if _, err := limiter.Allow(ctx, benchHotKey); err != nil {
					b.Error(err)
					return
				}
			}
		})
		b.StopTimer()
		reportRedisCmdsPerOp(b, fake, startCommands)
	})
}
