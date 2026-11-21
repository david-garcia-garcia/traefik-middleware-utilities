package simpleredis

import (
	"bufio"
	"bytes"
	"math"
	"testing"
)

func FuzzReadReply(f *testing.F) {
	f.Add([]byte("+OK\r\n"))
	f.Add([]byte(":12\r\n"))
	f.Add([]byte("-ERR bad\r\n"))
	f.Add([]byte("$-1\r\n"))
	f.Add([]byte("$0\r\n\r\n"))
	f.Add([]byte("$3\r\nabc\r\n"))
	f.Add([]byte("*2\r\n$3\r\nabc\r\n$-1\r\n"))
	f.Add([]byte("*-1\r\n"))
	f.Add([]byte("*0\r\n"))
	f.Add([]byte("*1\r\n:9\r\n"))
	f.Add([]byte("*1\r\n*1\r\n:9\r\n"))
	f.Add([]byte("$9999999999999999999999\r\n"))
	f.Add([]byte("*99999999999999999999\r\n"))
	f.Add([]byte("$-\r\n"))
	f.Add([]byte("*-\r\n"))
	f.Add([]byte("$"))
	f.Add([]byte("*"))
	f.Add([]byte("\r\n"))
	f.Add([]byte("\n"))
	f.Add([]byte("$2\r\nabcd"))
	f.Add([]byte("%2\r\n"))
	f.Add([]byte(">3\r\n"))
	f.Add([]byte("$18446744073709551616\r\n"))
	f.Add([]byte("*1\r\n$18446744073709551616\r\n"))

	f.Fuzz(func(t *testing.T, wire []byte) {
		values, clean, err := readReply(bufio.NewReader(bytes.NewReader(wire)))
		if !clean && values != nil {
			t.Fatalf("dirty stream returned values %q err=%v", values, err)
		}
	})
}

func FuzzParseLen(f *testing.F) {
	f.Add([]byte("0"))
	f.Add([]byte("1"))
	f.Add([]byte("-1"))
	f.Add([]byte("12"))
	f.Add([]byte("9999999999999999999999"))
	f.Add([]byte("18446744073709551616"))
	f.Add([]byte("-"))
	f.Add([]byte(""))

	f.Fuzz(func(t *testing.T, digits []byte) {
		length, ok := parseLen(digits)
		if !ok {
			return
		}
		if length >= 0 && length > math.MaxInt-2 {
			t.Fatalf("parseLen(%q) = %d, length+2 overflows", digits, length)
		}
	})
}
