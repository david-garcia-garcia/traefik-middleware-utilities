package tokenbucket

import (
	"bufio"
	"context"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
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

// liveBenchRoundTrips returns how many limiter round trips the engine at addr has served: the EVAL, EVALSHA, GET, and MGET
// calls in INFO commandstats. The commands a Lua script runs inside the engine are not round trips and are not counted; none
// of the limiter scripts calls GET or MGET. The engine must serve only this benchmark for the count to be per limiter.
func liveBenchRoundTrips(tb testing.TB, addr string) int64 {
	tb.Helper()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		tb.Fatalf("INFO dial %s: %v", addr, err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("*2\r\n$4\r\nINFO\r\n$12\r\ncommandstats\r\n")); err != nil {
		tb.Fatalf("INFO write: %v", err)
	}
	reader := bufio.NewReader(conn)
	header, err := reader.ReadString('\n')
	if err != nil || !strings.HasPrefix(header, "$") {
		tb.Fatalf("INFO reply header %q: %v", header, err)
	}
	size, err := strconv.Atoi(strings.TrimSpace(header[1:]))
	if err != nil {
		tb.Fatalf("INFO reply size %q: %v", header, err)
	}
	body := make([]byte, size+2)
	if _, err := io.ReadFull(reader, body); err != nil {
		tb.Fatalf("INFO reply body: %v", err)
	}
	var total int64
	for _, line := range strings.Split(string(body), "\n") {
		name, stats, found := strings.Cut(strings.TrimSpace(line), ":")
		if !found {
			continue
		}
		switch strings.ToLower(name) {
		case "cmdstat_eval", "cmdstat_evalsha", "cmdstat_get", "cmdstat_mget":
		default:
			continue
		}
		calls, _, _ := strings.Cut(stats, ",")
		count, err := strconv.ParseInt(strings.TrimPrefix(calls, "calls="), 10, 64)
		if err != nil {
			tb.Fatalf("INFO %s: %v", line, err)
		}
		total += count
	}
	return total
}

// reportRoundTripsPerOp reports the engine's round trips since start divided by b.N.
func reportRoundTripsPerOp(b *testing.B, addr string, start int64) {
	b.Helper()
	b.ReportMetric(float64(liveBenchRoundTrips(b, addr)-start)/float64(b.N), "redis-roundtrips/op")
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

// BenchmarkRedisAllow measures Redis Allow against each live engine (TOKENBUCKET_LIVE_*): one hot key, 10k rotating keys,
// and a hot key from parallel goroutines. It skips under -short or when no engine address is set. Keys carry a per-run prefix.
func BenchmarkRedisAllow(b *testing.B) {
	ctx := context.Background()
	for _, engine := range liveEngineAddrs(b) {
		engine := engine
		b.Run(engine.name, func(b *testing.B) {
			client := waitLiveClient(b, engine.addr)
			limiter, err := NewRedis(client, benchRate, benchBurst, benchMaxDelay, benchTTL)
			if err != nil {
				b.Fatal(err)
			}
			runPrefix := "tokenbucket-bench:" + strconv.FormatInt(time.Now().UnixNano(), 36) + ":"
			b.Run("hot-key", func(b *testing.B) {
				hotKey := runPrefix + benchHotKey
				if _, err := limiter.Allow(ctx, hotKey); err != nil {
					b.Fatal(err)
				}
				b.ReportAllocs()
				start := liveBenchRoundTrips(b, engine.addr)
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					status, err := limiter.Allow(ctx, hotKey)
					if err != nil {
						b.Fatal(err)
					}
					benchStatusSink = status
				}
				b.StopTimer()
				reportRoundTripsPerOp(b, engine.addr, start)
			})
			b.Run("rotating-10k", func(b *testing.B) {
				keys := benchKeyNames(runPrefix+"ip:", benchRotatingKeys)
				for _, key := range keys {
					if _, err := limiter.Allow(ctx, key); err != nil {
						b.Fatal(err)
					}
				}
				b.ReportAllocs()
				start := liveBenchRoundTrips(b, engine.addr)
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					status, err := limiter.Allow(ctx, keys[i%len(keys)])
					if err != nil {
						b.Fatal(err)
					}
					benchStatusSink = status
				}
				b.StopTimer()
				reportRoundTripsPerOp(b, engine.addr, start)
			})
			b.Run("hot-key-parallel", func(b *testing.B) {
				hotKey := runPrefix + "parallel:" + benchHotKey
				if _, err := limiter.Allow(ctx, hotKey); err != nil {
					b.Fatal(err)
				}
				b.ReportAllocs()
				start := liveBenchRoundTrips(b, engine.addr)
				b.ResetTimer()
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						if _, err := limiter.Allow(ctx, hotKey); err != nil {
							b.Error(err)
							return
						}
					}
				})
				b.StopTimer()
				reportRoundTripsPerOp(b, engine.addr, start)
			})
		})
	}
}
