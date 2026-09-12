package simpleredis

import (
	"bufio"
	"bytes"
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

// BenchmarkGet measures end-to-end Get against the in-process fake server.
func BenchmarkGet(b *testing.B) {
	_, addr := startFakeRedis(b, map[string]string{"hit": "some-cached-value"})
	redis := New(Config{Host: addr})
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

// BenchmarkMGet10 measures end-to-end MGet of ten keys against the fake server.
func BenchmarkMGet10(b *testing.B) {
	store := map[string]string{}
	names := make([]string, 10)
	for i := range names {
		names[i] = "key:" + strconv.Itoa(i)
		store[names[i]] = "value-" + strconv.Itoa(i)
	}
	_, addr := startFakeRedis(b, store)
	redis := New(Config{Host: addr})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := redis.MGet(names); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkIncr measures end-to-end Incr against the fake server.
func BenchmarkIncr(b *testing.B) {
	_, addr := startFakeRedis(b, map[string]string{})
	redis := New(Config{Host: addr})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := redis.Incr("counter"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEval measures end-to-end Eval of tokenBucketScript against the fake server.
func BenchmarkEval(b *testing.B) {
	_, addr := startFakeRedis(b, map[string]string{})
	redis := New(Config{Host: addr})
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

// encodeToDiscard encodes args with the production encoder and one Write to Discard.
func encodeToDiscard(b *testing.B, buf []byte, args [][]byte) []byte {
	b.Helper()
	buf = appendRESP(buf[:0], args)
	if _, err := io.Discard.Write(buf); err != nil {
		b.Fatal(err)
	}
	return buf
}

// encodeGet encodes a GET argv to Discard so CI can gate encode allocs/op and B/op.
func encodeGet(b *testing.B) {
	const name = "session:9f2c1ab4-user-token"
	buf := make([]byte, 0, 64)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf = encodeToDiscard(b, buf, [][]byte{[]byte("GET"), []byte(name)})
	}
}

// encodeEval rebuilds and encodes an EVAL argv each op (KEYS declared, script copied).
func encodeEval(b *testing.B) {
	keys := []string{"bucket:1.2.3.4"}
	args := []string{"10", "10", "1", "1700000000"}
	buf := make([]byte, 0, 512)

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
		buf = encodeToDiscard(b, buf, wire)
	}
}

// cannedBulkGET builds a `$<n>` bulk payload of length bytes plus the trailing CRLF.
func cannedBulkGET(length int) []byte {
	payload := make([]byte, 0, 16+length+2)
	payload = append(payload, '$')
	payload = strconv.AppendInt(payload, int64(length), 10)
	payload = append(payload, '\r', '\n')
	payload = append(payload, make([]byte, length)...)
	payload = append(payload, '\r', '\n')
	return payload
}

// array10Reply is a 10-slot bulk array of one-byte values (DestBranch ReadSlice fixture).
func array10Reply() []byte {
	var buf bytes.Buffer
	buf.WriteString("*10\r\n")
	for i := 0; i < 10; i++ {
		buf.WriteString("$1\r\n")
		buf.WriteByte(byte('a' + i))
		buf.WriteString("\r\n")
	}
	return buf.Bytes()
}

// benchDecode runs readReply against compiled RESP, resetting the reader each op
// so a ReadSlice view is not reused across iterations.
func benchDecode(b *testing.B, resp []byte) {
	b.Helper()
	src := bytes.NewReader(resp)
	reader := bufio.NewReader(src)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		src.Reset(resp)
		reader.Reset(src)
		if _, _, err := readReply(reader); err != nil {
			b.Fatal(err)
		}
	}
}

// encodeSet100KB encodes a 100 KiB SET argv to Discard so buffer growth is gated.
func encodeSet100KB(b *testing.B) {
	value := make([]byte, largeBulkBytes)
	buf := make([]byte, 0, largeBulkBytes+32)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf = encodeToDiscard(b, buf, [][]byte{[]byte("SET"), []byte("k"), value})
	}
}

// encodeMSetEX encodes native MSETEX argv to Discard.
func encodeMSetEX(b *testing.B) {
	args := msetexArgs([]string{"a", "b"}, [][]byte{[]byte("1"), []byte("2")}, "EX", 60)
	buf := make([]byte, 0, 128)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf = encodeToDiscard(b, buf, args)
	}
}

func BenchmarkEncodeGet(b *testing.B) { encodeGet(b) }

func BenchmarkEncodeEval(b *testing.B) { encodeEval(b) }

func BenchmarkEncodeMSetEX(b *testing.B) { encodeMSetEX(b) }

// BenchmarkDecodeBulk measures decode allocs for one bulk string reply (compiled RESP).
// Post-ReadSlice: ~2 allocs/op (payload + [][]byte); dest ReadBytes baseline was 3.
func BenchmarkDecodeBulk(b *testing.B) {
	benchDecode(b, []byte("$17\r\n0123456789abcdefg\r\n"))
}

// BenchmarkDecodeArray10 measures decode allocs for a 10-slot bulk array (compiled RESP).
// Post-ReadSlice: ~11 allocs/op (values slice + 10 payloads); dest ReadBytes baseline was 22.
func BenchmarkDecodeArray10(b *testing.B) {
	benchDecode(b, array10Reply())
}

// BenchmarkDecodeInteger measures decode allocs for one integer reply (compiled RESP).
// Post-ReadSlice: ~2 allocs/op (payload copy + [][]byte); dest ReadBytes baseline was 2.
func BenchmarkDecodeInteger(b *testing.B) {
	benchDecode(b, []byte(":1234567\r\n"))
}

// BenchmarkDecodeBulk100KB measures a canned 100 KiB bulk GET (`$102400`) through readReply.
func BenchmarkDecodeBulk100KB(b *testing.B) {
	benchDecode(b, cannedBulkGET(largeBulkBytes))
}

func BenchmarkEncodeSet100KB(b *testing.B) { encodeSet100KB(b) }

// BenchmarkGetParallel shows how many TCP sessions the pool burns above MaxIdleConns.
func BenchmarkGetParallel(b *testing.B) {
	fake, addr := startFakeRedis(b, map[string]string{"hit": "some-cached-value"})
	redis := New(Config{Host: addr, PoolSize: 64})

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
// MaxIdleConns against a server with realistic per-command latency. PoolSize is
// the burst width so the live cap does not turn waiters into redis:unreachable.
func TestConnectionChurnUnderLatency(t *testing.T) {
	const goroutines = 64
	const perGoroutine = 20
	fake, addr := startSlowRedis(t, 500*time.Microsecond)
	redis := New(Config{Host: addr, PoolSize: goroutines})

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
		goroutines, total, fake.connections(), redis.MaxIdleConns(), 100*float64(fake.connections())/float64(total))
}

// TestConnectionChurnAcrossBursts measures dials when traffic arrives in bursts
// wider than MaxIdleConns: everything above the idle cap is closed on release
// and redialed (TCP handshake, plus AUTH and SELECT when configured) next burst.
// PoolSize is the burst width so the live cap does not turn waiters into redis:unreachable.
func TestConnectionChurnAcrossBursts(t *testing.T) {
	const bursts = 5
	const width = 64
	fake, addr := startSlowRedis(t, 500*time.Microsecond)
	redis := New(Config{Host: addr, PoolSize: width})

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
		bursts, width, total, fake.connections(), width, redis.MaxIdleConns())
}

// Go 1.21 linux/amd64 measured allocs/op and B/op (CI toolchain) on the ReadSlice
// decode path. Slack: +1 allocs/op; B/op = measured + 64, or +20% when that is larger.
const (
	encodeGetAllocs      int64 = 6      // measured 5
	encodeGetBytes       int64 = 112    // measured 48 + 64
	encodeEvalAllocs     int64 = 20     // measured 19
	encodeEvalBytes      int64 = 864    // measured 800 + 64
	decodeBulkAllocs     int64 = 3      // measured 2
	decodeBulkBytes      int64 = 112    // measured 48 + 64
	decodeArrayAllocs    int64 = 12     // measured 11
	decodeArrayBytes     int64 = 344    // measured 280 + 64
	decodeIntegerAllocs  int64 = 3      // measured 2
	decodeIntegerBytes   int64 = 112    // measured 48 + 64
	decode100KBAllocs    int64 = 3      // measured 2
	decode100KBBytes     int64 = 127843 // measured 106536 + 20%
	encodeSet100KBAllocs int64 = 8      // measured 7
	encodeSet100KBBytes  int64 = 96     // measured 32 + 64
)

// allocExceedsCeiling reports whether allocs/op or B/op exceeded the recorded ceilings.
func allocExceedsCeiling(result testing.BenchmarkResult, maxAllocs, maxBytes int64) (allocsOver, bytesOver bool) {
	return result.AllocsPerOp() > maxAllocs, result.AllocedBytesPerOp() > maxBytes
}

// assertAllocCeiling fails the Test when the bench result is over the Go 1.21 allocs/op or B/op ceiling.
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
	assertAllocCeiling(t, "decode bulk", testing.Benchmark(BenchmarkDecodeBulk), decodeBulkAllocs, decodeBulkBytes)
}

func TestAllocDecodeArray10(t *testing.T) {
	assertAllocCeiling(t, "decode array10", testing.Benchmark(BenchmarkDecodeArray10), decodeArrayAllocs, decodeArrayBytes)
}

func TestAllocDecodeInteger(t *testing.T) {
	assertAllocCeiling(t, "decode integer", testing.Benchmark(BenchmarkDecodeInteger), decodeIntegerAllocs, decodeIntegerBytes)
}

func TestAllocDecodeBulk100KB(t *testing.T) {
	assertAllocCeiling(t, "decode 100KB bulk", testing.Benchmark(BenchmarkDecodeBulk100KB), decode100KBAllocs, decode100KBBytes)
}

func TestAllocEncodeSet100KB(t *testing.T) {
	assertAllocCeiling(t, "encode 100KB SET", testing.Benchmark(encodeSet100KB), encodeSet100KBAllocs, encodeSet100KBBytes)
}
