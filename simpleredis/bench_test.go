package simpleredis

import (
	"bufio"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"
)

// tokenBucketScript is a realistic hot-path Lua payload (size matters for Eval encoding).
//
//nolint:gosec // G101: Redis hash field name "tokens", not a credential
const tokenBucketScript = `local rl_source = redis.call("hgetall", KEYS[1])
local tokens = tonumber(ARGV[1])
local burst = tonumber(ARGV[2])
local rate = tonumber(ARGV[3])
local now = tonumber(ARGV[4])
if #rl_source == 4 then
  tokens = tonumber(rl_source[2]) + (now - tonumber(rl_source[4])) * rate
end
if tokens > burst then tokens = burst end
tokens = tokens - 1
redis.call("hset", KEYS[1], "tokens", tokens, "last", now)
redis.call("expire", KEYS[1], 60)
return tostring(tokens)`

const largeBulkBytes = 100 * 1024

func BenchmarkGet(b *testing.B) {
	_, addr := startFakeRedis(b, map[string]string{"hit": "some-cached-value"})
	var redis SimpleRedis
	redis.Init(addr, "", "")
	if _, err := redis.Get("hit"); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := redis.Get("hit"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMGet10(b *testing.B) {
	store := map[string]string{}
	names := make([]string, 10)
	for i := range names {
		names[i] = "key:" + strconv.Itoa(i)
		store[names[i]] = "value-" + strconv.Itoa(i)
	}
	_, addr := startFakeRedis(b, store)
	var redis SimpleRedis
	redis.Init(addr, "", "")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := redis.MGet(names); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkIncr(b *testing.B) {
	_, addr := startFakeRedis(b, map[string]string{})
	var redis SimpleRedis
	redis.Init(addr, "", "")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := redis.Incr("counter"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEval(b *testing.B) {
	_, addr := startFakeRedis(b, map[string]string{})
	var redis SimpleRedis
	redis.Init(addr, "", "")
	keys := []string{"bucket:1.2.3.4"}
	args := []string{"10", "10", "1", "1700000000"}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := redis.Eval(tokenBucketScript, keys, args); err != nil {
			b.Fatal(err)
		}
	}
}

// repeatReader replays one canned RESP payload forever so decode cost is client-only.
type repeatReader struct {
	payload []byte
	pos     int
}

func (r *repeatReader) Read(p []byte) (int, error) {
	written := 0
	for written < len(p) {
		n := copy(p[written:], r.payload[r.pos:])
		written += n
		r.pos += n
		if r.pos == len(r.payload) {
			r.pos = 0
		}
	}
	return written, nil
}

func encodeGet(b *testing.B) {
	writer := bufio.NewWriter(io.Discard)
	const name = "session:9f2c1ab4-user-token"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := writeCommand(writer, [][]byte{[]byte("GET"), []byte(name)}); err != nil {
			b.Fatal(err)
		}
	}
}

func encodeEval(b *testing.B) {
	writer := bufio.NewWriter(io.Discard)
	keys := []string{"bucket:1.2.3.4"}
	args := []string{"10", "10", "1", "1700000000"}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wire := make([][]byte, 0, 3+len(keys)+len(args))
		wire = append(wire, []byte("EVAL"), []byte(tokenBucketScript), []byte(strconv.Itoa(len(keys))))
		for _, key := range keys {
			wire = append(wire, []byte(key))
		}
		for _, arg := range args {
			wire = append(wire, []byte(arg))
		}
		if err := writeCommand(writer, wire); err != nil {
			b.Fatal(err)
		}
	}
}

func decodeBulk(b *testing.B) {
	reader := bufio.NewReader(&repeatReader{payload: []byte("$17\r\nsome-cached-value\r\n")})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := readReply(reader); err != nil {
			b.Fatal(err)
		}
	}
}

func decodeArray10(b *testing.B) {
	payload := []byte("*10\r\n")
	for i := 0; i < 10; i++ {
		payload = append(payload, "$8\r\nvalue-00\r\n"...)
	}
	reader := bufio.NewReader(&repeatReader{payload: payload})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := readReply(reader); err != nil {
			b.Fatal(err)
		}
	}
}

func decodeInteger(b *testing.B) {
	reader := bufio.NewReader(&repeatReader{payload: []byte(":1234567\r\n")})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		values, _, err := readReply(reader)
		if _, err = parseIntegerReply(values, err); err != nil {
			b.Fatal(err)
		}
	}
}

func cannedBulkGET(length int) []byte {
	payload := make([]byte, 0, 16+length+2)
	payload = append(payload, '$')
	payload = strconv.AppendInt(payload, int64(length), 10)
	payload = append(payload, '\r', '\n')
	payload = append(payload, make([]byte, length)...)
	payload = append(payload, '\r', '\n')
	return payload
}

func decodeBulk100KB(b *testing.B) {
	reader := bufio.NewReader(&repeatReader{payload: cannedBulkGET(largeBulkBytes)})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := readReply(reader); err != nil {
			b.Fatal(err)
		}
	}
}

func encodeSet100KB(b *testing.B) {
	writer := bufio.NewWriter(io.Discard)
	value := make([]byte, largeBulkBytes)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := writeCommand(writer, [][]byte{[]byte("SET"), []byte("k"), value}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncodeGet(b *testing.B) { encodeGet(b) }

func BenchmarkEncodeEval(b *testing.B) { encodeEval(b) }

func BenchmarkDecodeBulk(b *testing.B) { decodeBulk(b) }

func BenchmarkDecodeArray10(b *testing.B) { decodeArray10(b) }

func BenchmarkDecodeInteger(b *testing.B) { decodeInteger(b) }

func BenchmarkDecodeBulk100KB(b *testing.B) { decodeBulk100KB(b) }

func BenchmarkEncodeSet100KB(b *testing.B) { encodeSet100KB(b) }

// BenchmarkGetParallel shows how many TCP sessions the pool burns above maxIdleConns.
func BenchmarkGetParallel(b *testing.B) {
	fake, addr := startFakeRedis(b, map[string]string{"hit": "some-cached-value"})
	var redis SimpleRedis
	redis.Init(addr, "", "")

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := redis.Get("hit"); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.StopTimer()
	b.ReportMetric(float64(fake.connections()), "dials")
}

// startSlowRedis answers every command with a canned bulk after latency, so
// concurrent callers actually overlap the way they do against a real server.
func startSlowRedis(t testing.TB, latency time.Duration) (*fakeRedis, string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	fake := &fakeRedis{store: map[string]string{}}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			fake.mu.Lock()
			fake.conns++
			fake.mu.Unlock()
			go func(conn net.Conn) {
				defer conn.Close()
				reader := bufio.NewReader(conn)
				for {
					if _, err := readCommand(reader); err != nil {
						return
					}
					time.Sleep(latency)
					if _, err := io.WriteString(conn, "$1\r\nt\r\n"); err != nil {
						return
					}
				}
			}(conn)
		}
	}()
	return fake, listener.Addr().String()
}

// TestConnectionChurnUnderLatency measures dials when in-flight callers exceed
// maxIdleConns against a server with realistic per-command latency.
func TestConnectionChurnUnderLatency(t *testing.T) {
	const goroutines = 64
	const perGoroutine = 20
	fake, addr := startSlowRedis(t, 500*time.Microsecond)
	var redis SimpleRedis
	redis.Init(addr, "", "")

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perGoroutine; j++ {
				if _, err := redis.Get("hit"); err != nil {
					t.Errorf("Get: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()

	total := goroutines * perGoroutine
	t.Logf("%d concurrent callers, %d commands: %d dials (idle cap %d) = %.1f%% of commands paid a TCP handshake",
		goroutines, total, fake.connections(), maxIdleConns, 100*float64(fake.connections())/float64(total))
}

// TestConnectionChurnAcrossBursts measures dials when traffic arrives in bursts
// wider than maxIdleConns: everything above the idle cap is closed on release
// and redialed (TCP handshake, plus AUTH and SELECT when configured) next burst.
func TestConnectionChurnAcrossBursts(t *testing.T) {
	const bursts = 5
	const width = 64
	fake, addr := startSlowRedis(t, 500*time.Microsecond)
	var redis SimpleRedis
	redis.Init(addr, "", "")

	for burst := 0; burst < bursts; burst++ {
		var wg sync.WaitGroup
		for i := 0; i < width; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := redis.Get("hit"); err != nil {
					t.Errorf("Get: %v", err)
				}
			}()
		}
		wg.Wait()
	}

	total := bursts * width
	t.Logf("%d bursts of %d concurrent Get (%d commands): %d dials, ideal %d (idle cap %d)",
		bursts, width, total, fake.connections(), width, maxIdleConns)
}

// Go 1.21 linux/amd64 measured allocs/op and B/op (CI toolchain). Slack: +1
// allocs/op; B/op = measured + 64, or +20% when that is larger.
const (
	encodeGetAllocs      int64 = 6      // measured 5
	encodeGetBytes       int64 = 112    // measured 48 + 64
	encodeEvalAllocs     int64 = 20     // measured 19
	encodeEvalBytes      int64 = 864    // measured 800 + 64
	decodeBulkAllocs     int64 = 4      // measured 3
	decodeBulkBytes      int64 = 117    // measured 53 + 64
	decodeArrayAllocs    int64 = 23     // measured 22
	decodeArrayBytes     int64 = 472    // measured 408 + 64
	decodeIntegerAllocs  int64 = 3      // measured 2
	decodeIntegerBytes   int64 = 104    // measured 40 + 64
	decode100KBAllocs    int64 = 4      // measured 3
	decode100KBBytes     int64 = 127843 // measured 106536 + 20%
	encodeSet100KBAllocs int64 = 8      // measured 7
	encodeSet100KBBytes  int64 = 96     // measured 32 + 64
)

func allocExceedsCeiling(result testing.BenchmarkResult, maxAllocs, maxBytes int64) (allocsOver, bytesOver bool) {
	return result.AllocsPerOp() > maxAllocs, result.AllocedBytesPerOp() > maxBytes
}

func assertAllocCeiling(t *testing.T, name string, result testing.BenchmarkResult, maxAllocs, maxBytes int64) {
	t.Helper()
	allocs := result.AllocsPerOp()
	bytesPerOp := result.AllocedBytesPerOp()
	t.Logf("%s: %d allocs/op, %d B/op (ceilings %d allocs/op, %d B/op)", name, allocs, bytesPerOp, maxAllocs, maxBytes)
	allocsOver, bytesOver := allocExceedsCeiling(result, maxAllocs, maxBytes)
	if allocsOver {
		t.Errorf("%s: %d allocs/op exceeds Go 1.21 ceiling %d", name, allocs, maxAllocs)
	}
	if bytesOver {
		t.Errorf("%s: %d B/op exceeds Go 1.21 ceiling %d", name, bytesPerOp, maxBytes)
	}
}

func TestAllocCeilingFailsWhenOverBudget(t *testing.T) {
	overAllocs := testing.BenchmarkResult{N: 1, MemAllocs: uint64(encodeGetAllocs) + 1}
	overBytes := testing.BenchmarkResult{N: 1, MemBytes: uint64(encodeGetBytes) + 1}
	allocsOver, _ := allocExceedsCeiling(overAllocs, encodeGetAllocs, encodeGetBytes)
	_, bytesOver := allocExceedsCeiling(overBytes, encodeGetAllocs, encodeGetBytes)
	if !allocsOver {
		t.Fatal("over-budget AllocsPerOp must fail the guard")
	}
	if !bytesOver {
		t.Fatal("over-budget AllocedBytesPerOp must fail the guard")
	}
}

func TestAllocEncodeGet(t *testing.T) {
	assertAllocCeiling(t, "encode GET", testing.Benchmark(encodeGet), encodeGetAllocs, encodeGetBytes)
}

func TestAllocEncodeEval(t *testing.T) {
	assertAllocCeiling(t, "encode EVAL", testing.Benchmark(encodeEval), encodeEvalAllocs, encodeEvalBytes)
}

func TestAllocDecodeBulk(t *testing.T) {
	assertAllocCeiling(t, "decode bulk", testing.Benchmark(decodeBulk), decodeBulkAllocs, decodeBulkBytes)
}

func TestAllocDecodeArray10(t *testing.T) {
	assertAllocCeiling(t, "decode array10", testing.Benchmark(decodeArray10), decodeArrayAllocs, decodeArrayBytes)
}

func TestAllocDecodeInteger(t *testing.T) {
	assertAllocCeiling(t, "decode integer", testing.Benchmark(decodeInteger), decodeIntegerAllocs, decodeIntegerBytes)
}

func TestAllocDecodeBulk100KB(t *testing.T) {
	assertAllocCeiling(t, "decode 100KB bulk", testing.Benchmark(decodeBulk100KB), decode100KBAllocs, decode100KBBytes)
}

func TestAllocEncodeSet100KB(t *testing.T) {
	assertAllocCeiling(t, "encode 100KB SET", testing.Benchmark(encodeSet100KB), encodeSet100KBAllocs, encodeSet100KBBytes)
}
