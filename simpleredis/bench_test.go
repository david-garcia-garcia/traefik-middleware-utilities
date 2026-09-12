package simpleredis

import (
	"bufio"
	"bytes"
	"testing"
)

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
