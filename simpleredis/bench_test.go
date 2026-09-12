package simpleredis

import (
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
