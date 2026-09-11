package simpleredis

import (
	"fmt"
	"testing"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// benchmarkYaegiEncode runs one interpreted encode loop from encodeprobe.
func benchmarkYaegiEncode(b *testing.B, fn string) {
	goPath := b.TempDir()
	writeGopathFile(b, goPath, "encodeprobe", "encode.go", encodeprobeSrc)

	interpreter := interp.New(interp.Options{GoPath: goPath})
	if err := interpreter.Use(stdlib.Symbols); err != nil {
		b.Fatalf("use stdlib: %v", err)
	}
	if _, err := interpreter.Eval(`import "encodeprobe"`); err != nil {
		b.Fatalf("import encodeprobe: %v", err)
	}
	if _, err := interpreter.Eval(fmt.Sprintf(`encodeprobe.%s(1)`, fn)); err != nil {
		b.Fatalf("warmup: %v", err)
	}

	b.ResetTimer()
	evaluated, err := interpreter.Eval(fmt.Sprintf(`encodeprobe.%s(%d)`, fn, b.N))
	if err != nil {
		b.Fatalf("eval loop: %v", err)
	}
	b.StopTimer()
	if got := evaluated.Interface().(string); got != "ok" {
		b.Fatalf("%s = %q, want ok", fn, got)
	}
}

// BenchmarkYaegiEncodeBufio is dest writeCommand shape, interpreted.
func BenchmarkYaegiEncodeBufio(b *testing.B) { benchmarkYaegiEncode(b, "BufioLoop") }

// BenchmarkYaegiEncodeSingleWrite builds the frame in one scratch buffer, interpreted.
func BenchmarkYaegiEncodeSingleWrite(b *testing.B) { benchmarkYaegiEncode(b, "SingleWriteLoop") }

const encodeprobeSrc = `package encodeprobe

import (
	"bufio"
	"io"
	"strconv"
)

// writeBufio is the dest per-argument bufio.Writer encoding.
func writeBufio(writer *bufio.Writer, args [][]byte) error {
	if _, err := writer.WriteString("*" + strconv.Itoa(len(args)) + "\r\n"); err != nil {
		return err
	}
	for _, arg := range args {
		if _, err := writer.WriteString("$" + strconv.Itoa(len(arg)) + "\r\n"); err != nil {
			return err
		}
		if _, err := writer.Write(arg); err != nil {
			return err
		}
		if _, err := writer.WriteString("\r\n"); err != nil {
			return err
		}
	}
	return writer.Flush()
}

// writeSingle builds the whole frame in buf and issues one Write.
func writeSingle(writer io.Writer, buf []byte, args [][]byte) ([]byte, error) {
	buf = buf[:0]
	buf = append(buf, '*')
	buf = strconv.AppendInt(buf, int64(len(args)), 10)
	buf = append(buf, '\r', '\n')
	for _, arg := range args {
		buf = append(buf, '$')
		buf = strconv.AppendInt(buf, int64(len(arg)), 10)
		buf = append(buf, '\r', '\n')
		buf = append(buf, arg...)
		buf = append(buf, '\r', '\n')
	}
	_, err := writer.Write(buf)
	return buf, err
}

// BufioLoop encodes a GET count times through the dest path.
func BufioLoop(count int) string {
	writer := bufio.NewWriter(io.Discard)
	args := [][]byte{[]byte("GET"), []byte("session:9f2c1ab4-user-token")}
	for i := 0; i < count; i++ {
		if err := writeBufio(writer, args); err != nil {
			return "bufio:" + err.Error()
		}
	}
	return "ok"
}

// SingleWriteLoop encodes a GET count times through one reused buffer.
func SingleWriteLoop(count int) string {
	args := [][]byte{[]byte("GET"), []byte("session:9f2c1ab4-user-token")}
	buf := make([]byte, 0, 256)
	var err error
	for i := 0; i < count; i++ {
		if buf, err = writeSingle(io.Discard, buf, args); err != nil {
			return "single:" + err.Error()
		}
	}
	return "ok"
}
`
