package windowcounter

import (
	"bufio"
	"context"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
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

// newBenchLimiter builds a limiter at syncRate on a live engine client. Close runs after the benchmark.
func newBenchLimiter(tb testing.TB, client *simpleredis.SimpleRedis, syncRate time.Duration) *Limiter {
	tb.Helper()
	limiter, err := New(client, syncRate)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(limiter.Close)
	return limiter
}

// benchRunPrefix returns a key prefix unique to this run, so reruns against the same engine start from fresh windows.
func benchRunPrefix() string {
	return "windowcounter-bench:" + strconv.FormatInt(time.Now().UnixNano(), 36) + ":"
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

// BenchmarkTake measures Take against each live engine (WINDOWCOUNTER_LIVE_*) in every benchTakeModes mode: one hot key,
// 10k rotating keys, and a hot key from parallel goroutines. The buffered flush timer runs for real in the background. It skips
// under -short or when no engine address is set.
func BenchmarkTake(b *testing.B) {
	for _, engine := range liveEngineAddrs(b) {
		engine := engine
		b.Run(engine.name, func(b *testing.B) {
			client := waitLiveClient(b, engine.addr)
			for _, mode := range benchTakeModes {
				mode := mode
				b.Run(mode.name+"/hot-key", func(b *testing.B) {
					limiter := newBenchLimiter(b, client, mode.syncRate)
					hotKey := benchRunPrefix() + benchHotKey
					takeForBench(b, limiter, hotKey)
					b.ReportAllocs()
					start := liveBenchRoundTrips(b, engine.addr)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						takeForBench(b, limiter, hotKey)
					}
					b.StopTimer()
					reportRoundTripsPerOp(b, engine.addr, start)
				})
				b.Run(mode.name+"/rotating-10k", func(b *testing.B) {
					limiter := newBenchLimiter(b, client, mode.syncRate)
					keys := benchKeyNames(benchRunPrefix()+"ip:", benchRotatingKeys)
					for _, key := range keys {
						takeForBench(b, limiter, key)
					}
					b.ReportAllocs()
					start := liveBenchRoundTrips(b, engine.addr)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						takeForBench(b, limiter, keys[i%len(keys)])
					}
					b.StopTimer()
					reportRoundTripsPerOp(b, engine.addr, start)
				})
				b.Run(mode.name+"/hot-key-parallel", func(b *testing.B) {
					limiter := newBenchLimiter(b, client, mode.syncRate)
					hotKey := benchRunPrefix() + benchHotKey
					takeForBench(b, limiter, hotKey)
					b.ReportAllocs()
					start := liveBenchRoundTrips(b, engine.addr)
					b.ResetTimer()
					b.RunParallel(func(pb *testing.PB) {
						for pb.Next() {
							if _, err := limiter.Take(context.Background(), hotKey, benchLimit, benchWindow); err != nil {
								b.Error(err)
								return
							}
						}
					})
					b.StopTimer()
					reportRoundTripsPerOp(b, engine.addr, start)
				})
			}
		})
	}
}

// BenchmarkFlush measures one buffered flush of 1000 dirty window keys against each live engine (WINDOWCOUNTER_LIVE_*).
// sync_rate is one hour so no timer tick fires, and the clock is frozen at the start so every Take lands in one window; the
// keys are re-dirtied in memory with the timer stopped.
func BenchmarkFlush(b *testing.B) {
	for _, engine := range liveEngineAddrs(b) {
		engine := engine
		b.Run(engine.name+"/dirty-keys-1000", func(b *testing.B) {
			limiter := newBenchLimiter(b, waitLiveClient(b, engine.addr), time.Hour)
			now := time.Now()
			limiter.SetNowForTest(func() time.Time { return now })
			for _, key := range benchKeyNames(benchRunPrefix()+"ip:", benchFlushKeys) {
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
			start := liveBenchRoundTrips(b, engine.addr)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				redirtyWindowsForBench(limiter, dirtyKeys)
				b.StartTimer()
				if err := limiter.flushPending(ctx); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			reportRoundTripsPerOp(b, engine.addr, start)
		})
	}
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
