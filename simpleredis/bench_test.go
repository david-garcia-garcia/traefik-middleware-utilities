package simpleredis

import (
	"bufio"
	"bytes"
	"io"
	"strconv"
	"testing"
)

// BenchmarkEncodeGet is the client-side encode cost of Get: appendRESP plus one Write.
func BenchmarkEncodeGet(b *testing.B) {
	args := [][]byte{[]byte("GET"), []byte("session:9f2c1ab4-user-token")}
	buf := make([]byte, 0, 64)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf = appendRESP(buf[:0], args)
		if _, err := io.Discard.Write(buf); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEncodeEval is the client-side encode cost of one Eval with a real script.
func BenchmarkEncodeEval(b *testing.B) {
	keys := []string{"bucket:1.2.3.4"}
	args := []string{"10", "1700000000"}
	buf := make([]byte, 0, 512)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wire := make([][]byte, 0, 3+len(keys)+len(args))
		wire = append(wire, []byte("EVAL"), []byte(kongIncrbyExpireatScript), []byte(strconv.Itoa(len(keys))))
		for _, key := range keys {
			wire = append(wire, []byte(key))
		}
		for _, arg := range args {
			wire = append(wire, []byte(arg))
		}
		buf = appendRESP(buf[:0], wire)
		if _, err := io.Discard.Write(buf); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEncodeMSetEX is the client-side encode cost of native MSETEX argv: appendRESP plus one Write.
func BenchmarkEncodeMSetEX(b *testing.B) {
	args := msetexArgs([]string{"a", "b"}, [][]byte{[]byte("1"), []byte("2")}, "EX", 60)
	buf := make([]byte, 0, 128)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf = appendRESP(buf[:0], args)
		if _, err := io.Discard.Write(buf); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDecodeBulk measures decode allocs for one bulk string reply (compiled RESP).
// Post-ReadSlice: ~2 allocs/op (payload + [][]byte); dest ReadBytes baseline was 3.
func BenchmarkDecodeBulk(b *testing.B) {
	benchDecode(b, []byte("$17\r\n0123456789abcdefg\r\n"))
}

// BenchmarkDecodeArray10 measures decode allocs for a 10-slot bulk array (compiled RESP).
// Post-ReadSlice: ~11 allocs/op (values slice + 10 payloads); dest ReadBytes baseline was 22.
func BenchmarkDecodeArray10(b *testing.B) {
	var buf bytes.Buffer
	buf.WriteString("*10\r\n")
	for i := 0; i < 10; i++ {
		buf.WriteString("$1\r\n")
		buf.WriteByte(byte('a' + i))
		buf.WriteString("\r\n")
	}
	benchDecode(b, buf.Bytes())
}

// BenchmarkDecodeInteger measures decode allocs for one integer reply (compiled RESP).
// Post-ReadSlice: ~2 allocs/op (payload copy + [][]byte); dest ReadBytes baseline was 2.
func BenchmarkDecodeInteger(b *testing.B) {
	benchDecode(b, []byte(":1234567\r\n"))
}

// benchDecode runs readReply against compiled RESP, resetting the reader each op.
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
