package simpleredis

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"testing"
	"unsafe"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
	yaegiunsafe "github.com/traefik/yaegi/stdlib/unsafe"
)

// evalUnsafeProbe interprets src and returns Probe(). withUnsafe registers the
// yaegi unsafe symbols; unrestricted sets interp's Unrestricted option.
func evalUnsafeProbe(t *testing.T, src string, withUnsafe, unrestricted bool) (string, error) {
	t.Helper()
	goPath := t.TempDir()
	writeGopathFile(t, goPath, "unsafeprobe", "probe.go", src)

	interpreter := interp.New(interp.Options{GoPath: goPath, Unrestricted: unrestricted})
	if err := interpreter.Use(stdlib.Symbols); err != nil {
		t.Fatalf("use stdlib: %v", err)
	}
	if withUnsafe {
		if err := interpreter.Use(yaegiunsafe.Symbols); err != nil {
			t.Fatalf("use unsafe: %v", err)
		}
	}
	if _, err := interpreter.Eval(`import "unsafeprobe"`); err != nil {
		return "", err
	}
	evaluated, err := interpreter.Eval(`unsafeprobe.Probe()`)
	if err != nil {
		return "", err
	}
	return evaluated.Interface().(string), nil
}

// TestYaegiUnsafeVariants reports which zero-copy conversion the interpreter accepts.
func TestYaegiUnsafeVariants(t *testing.T) {
	variants := map[string]string{
		"go1.20 unsafe.Slice/unsafe.String": `package unsafeprobe

import "unsafe"

func Probe() string {
	b := unsafe.Slice(unsafe.StringData("hello"), len("hello"))
	return unsafe.String(unsafe.SliceData(b), len(b))
}
`,
		"legacy bytes->string via unsafe.Pointer": `package unsafeprobe

import "unsafe"

func Probe() string {
	b := []byte("hello")
	return *(*string)(unsafe.Pointer(&b))
}
`,
		"legacy string->bytes via struct header": `package unsafeprobe

import "unsafe"

func Probe() string {
	s := "hello"
	b := *(*[]byte)(unsafe.Pointer(&struct {
		string
		Cap int
	}{s, len(s)}))
	if len(b) != 5 || b[0] != 'h' {
		return "bad-bytes"
	}
	return string(b)
}
`,
		"reflect.StringHeader": `package unsafeprobe

import (
	"reflect"
	"unsafe"
)

func Probe() string {
	s := "hello"
	header := (*reflect.StringHeader)(unsafe.Pointer(&s))
	var out []byte
	slice := (*reflect.SliceHeader)(unsafe.Pointer(&out))
	slice.Data = header.Data
	slice.Len = header.Len
	slice.Cap = header.Len
	return string(out)
}
`,
	}

	modes := []struct {
		label        string
		withUnsafe   bool
		unrestricted bool
	}{
		{"stdlib only", false, false},
		{"stdlib+unsafe", true, false},
		{"stdlib+unsafe+unrestricted", true, true},
	}

	for _, mode := range modes {
		for name, src := range variants {
			got, err := evalUnsafeProbe(t, src, mode.withUnsafe, mode.unrestricted)
			if err != nil {
				t.Logf("[%-26s] UNSUPPORTED %-40s", mode.label, name)
				continue
			}
			t.Logf("[%-26s] OK          %-40s Probe() = %q", mode.label, name, got)
		}
	}
}

// stringToBytesUnsafe is the pre-v9 go-redis trick, the only form Yaegi accepts.
func stringToBytesUnsafe(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(&struct {
		string
		Cap int
	}{s, len(s)}))
}

// bytesToStringUnsafe is the pre-v9 go-redis reverse trick.
func bytesToStringUnsafe(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// BenchmarkCompiledEncodeEvalCopy is today's Eval encode: the script is copied.
func BenchmarkCompiledEncodeEvalCopy(b *testing.B) {
	writer := bufio.NewWriter(io.Discard)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := writeCommand(writer, [][]byte{
			[]byte("EVAL"), []byte(tokenBucketScript), []byte("1"), []byte("bucket:1.2.3.4"),
		}); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCompiledEncodeEvalUnsafe is the same encode with zero-copy args.
func BenchmarkCompiledEncodeEvalUnsafe(b *testing.B) {
	writer := bufio.NewWriter(io.Discard)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := writeCommand(writer, [][]byte{
			stringToBytesUnsafe("EVAL"), stringToBytesUnsafe(tokenBucketScript),
			stringToBytesUnsafe("1"), stringToBytesUnsafe("bucket:1.2.3.4"),
		}); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCompiledParseIntCopy is today's integer reply parse.
func BenchmarkCompiledParseIntCopy(b *testing.B) {
	payload := []byte("1234567")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := strconv.ParseInt(string(payload), 10, 64); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCompiledParseIntUnsafe is the same parse without the copy.
func BenchmarkCompiledParseIntUnsafe(b *testing.B) {
	payload := []byte("1234567")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := strconv.ParseInt(bytesToStringUnsafe(payload), 10, 64); err != nil {
			b.Fatal(err)
		}
	}
}

// benchmarkYaegiConvert runs one interpreted conversion loop from convertprobe.
func benchmarkYaegiConvert(b *testing.B, loopName string) {
	goPath := b.TempDir()
	writeGopathFile(b, goPath, "convertprobe", "convert.go", convertprobeSrc)

	interpreter := interp.New(interp.Options{GoPath: goPath})
	if err := interpreter.Use(stdlib.Symbols); err != nil {
		b.Fatalf("use stdlib: %v", err)
	}
	if err := interpreter.Use(yaegiunsafe.Symbols); err != nil {
		b.Fatalf("use unsafe: %v", err)
	}
	if _, err := interpreter.Eval(`import "convertprobe"`); err != nil {
		b.Fatalf("import convertprobe: %v", err)
	}
	if _, err := interpreter.Eval(fmt.Sprintf(`convertprobe.%s(1)`, loopName)); err != nil {
		b.Fatalf("warmup: %v", err)
	}

	b.ResetTimer()
	if _, err := interpreter.Eval(fmt.Sprintf(`convertprobe.%s(%d)`, loopName, b.N)); err != nil {
		b.Fatalf("eval loop: %v", err)
	}
}

// BenchmarkYaegiConvertCopy is the builtin conversion, interpreted.
func BenchmarkYaegiConvertCopy(b *testing.B) { benchmarkYaegiConvert(b, "CopyLoop") }

// BenchmarkYaegiConvertCopyCall is the builtin conversion behind a helper call,
// so it pays the same interpreted call overhead as the unsafe variant.
func BenchmarkYaegiConvertCopyCall(b *testing.B) { benchmarkYaegiConvert(b, "CopyCallLoop") }

// BenchmarkYaegiConvertUnsafe is the pre-v9 unsafe trick, interpreted.
func BenchmarkYaegiConvertUnsafe(b *testing.B) { benchmarkYaegiConvert(b, "UnsafeLoop") }

const convertprobeSrc = `package convertprobe

import "unsafe"

const script = ` + "`" + tokenBucketScript + "`" + `

func stringToBytesUnsafe(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(&struct {
		string
		Cap int
	}{s, len(s)}))
}

func stringToBytesCopy(s string) []byte {
	return []byte(s)
}

// CopyLoop converts the script with the builtin conversion count times.
func CopyLoop(count int) int {
	total := 0
	for i := 0; i < count; i++ {
		b := []byte(script)
		total += len(b) + int(b[0])
	}
	return total
}

// CopyCallLoop is CopyLoop behind a helper call, matching UnsafeLoop's shape.
func CopyCallLoop(count int) int {
	total := 0
	for i := 0; i < count; i++ {
		b := stringToBytesCopy(script)
		total += len(b) + int(b[0])
	}
	return total
}

// UnsafeLoop converts the script with the unsafe trick count times.
func UnsafeLoop(count int) int {
	total := 0
	for i := 0; i < count; i++ {
		b := stringToBytesUnsafe(script)
		total += len(b) + int(b[0])
	}
	return total
}
`

// benchmarkYaegiEncode runs one interpreted encode loop from encodeprobe.
func benchmarkYaegiEncode(b *testing.B, loopName string) {
	goPath := b.TempDir()
	writeGopathFile(b, goPath, "encodeprobe", "encode.go", encodeprobeSrc)

	interpreter := interp.New(interp.Options{GoPath: goPath})
	if err := interpreter.Use(stdlib.Symbols); err != nil {
		b.Fatalf("use stdlib: %v", err)
	}
	if _, err := interpreter.Eval(`import "encodeprobe"`); err != nil {
		b.Fatalf("import encodeprobe: %v", err)
	}
	if _, err := interpreter.Eval(fmt.Sprintf(`encodeprobe.%s(1)`, loopName)); err != nil {
		b.Fatalf("warmup: %v", err)
	}

	b.ResetTimer()
	evaluated, err := interpreter.Eval(fmt.Sprintf(`encodeprobe.%s(%d)`, loopName, b.N))
	if err != nil {
		b.Fatalf("eval loop: %v", err)
	}
	b.StopTimer()
	if got := evaluated.Interface().(string); got != "ok" {
		b.Fatalf("%s = %q, want ok", loopName, got)
	}
}

// BenchmarkYaegiEncodeBufio is today's writeCommand shape, interpreted.
func BenchmarkYaegiEncodeBufio(b *testing.B) { benchmarkYaegiEncode(b, "BufioLoop") }

// BenchmarkYaegiEncodeSingleWrite builds the frame in one scratch buffer, interpreted.
func BenchmarkYaegiEncodeSingleWrite(b *testing.B) { benchmarkYaegiEncode(b, "SingleWriteLoop") }

const encodeprobeSrc = `package encodeprobe

import (
	"bufio"
	"io"
	"strconv"
)

// writeBufio is the current per-argument bufio.Writer encoding.
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

// BufioLoop encodes a GET count times through the current path.
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

// BenchmarkYaegiGet measures interpreted Get against the same fake server the
// compiled BenchmarkGet uses, so the interpretation multiplier is visible.
func BenchmarkYaegiGet(b *testing.B) {
	_, addr := startFakeRedis(b, map[string]string{"hit": "some-cached-value"})

	goPath := b.TempDir()
	writeGopathSimpleredis(b, goPath)
	writeGopathFile(b, goPath, "loopprobe", "loop.go", loopprobeSrc)

	interpreter := interp.New(interp.Options{GoPath: goPath})
	if err := interpreter.Use(stdlib.Symbols); err != nil {
		b.Fatalf("use stdlib: %v", err)
	}
	if _, err := interpreter.Eval(`import "loopprobe"`); err != nil {
		b.Fatalf("import loopprobe: %v", err)
	}
	if _, err := interpreter.Eval(fmt.Sprintf(`loopprobe.GetLoop(%q, 1)`, addr)); err != nil {
		b.Fatalf("warmup: %v", err)
	}

	b.ResetTimer()
	evaluated, err := interpreter.Eval(fmt.Sprintf(`loopprobe.GetLoop(%q, %d)`, addr, b.N))
	if err != nil {
		b.Fatalf("eval loop: %v", err)
	}
	b.StopTimer()
	if got := evaluated.Interface().(string); got != "ok" {
		b.Fatalf("GetLoop = %q, want ok", got)
	}
}

const loopprobeSrc = `package loopprobe

import (
	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// GetLoop builds one client with New and runs count Get calls.
func GetLoop(host string, count int) string {
	client := simpleredis.New(simpleredis.Config{Host: host})
	for i := 0; i < count; i++ {
		if _, err := client.Get("hit"); err != nil {
			return "get:" + err.Error()
		}
	}
	return "ok"
}
`
