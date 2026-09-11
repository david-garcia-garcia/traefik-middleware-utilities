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

// evalSizeScript is a ~470-byte Eval payload used only to size encode/convert benches.
const evalSizeScript = `local rl_source = redis.call("hgetall", KEYS[1])
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

// evalUnsafeProbe interprets src and returns Probe(). withUnsafe registers the
// Yaegi unsafe symbols; unrestricted sets interp's Unrestricted option.
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

// TestYaegiUnsafeVariants asserts which string/[]byte conversions Yaegi accepts.
func TestYaegiUnsafeVariants(t *testing.T) {
	variants := []struct {
		name         string
		okWithUnsafe bool
		src          string
	}{
		{
			name:         "go-redis v9 unsafe.Slice/unsafe.String",
			okWithUnsafe: false,
			src: `package unsafeprobe

import "unsafe"

func Probe() string {
	b := unsafe.Slice(unsafe.StringData("hello"), len("hello"))
	return unsafe.String(unsafe.SliceData(b), len(b))
}
`,
		},
		{
			name:         "legacy pointer-cast",
			okWithUnsafe: true,
			src: `package unsafeprobe

import "unsafe"

func Probe() string {
	b := []byte("hello")
	return *(*string)(unsafe.Pointer(&b))
}
`,
		},
		{
			name:         "legacy struct-header",
			okWithUnsafe: true,
			src: `package unsafeprobe

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
		},
		{
			name:         "reflect.StringHeader",
			okWithUnsafe: true,
			src: `package unsafeprobe

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
		},
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
		for _, variant := range variants {
			got, err := evalUnsafeProbe(t, variant.src, mode.withUnsafe, mode.unrestricted)
			supported := err == nil
			wantSupported := variant.okWithUnsafe && mode.withUnsafe
			if supported != wantSupported {
				t.Fatalf("%s / %s: supported=%v want %v (err=%v got=%q)",
					mode.label, variant.name, supported, wantSupported, err, got)
			}
			if wantSupported && got != "hello" {
				t.Fatalf("%s / %s: Probe()=%q, want hello", mode.label, variant.name, got)
			}
		}
	}
}

// stringToBytesUnsafe is the pre-v9 go-redis trick, the only form Yaegi accepts.
func stringToBytesUnsafe(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(&struct { //nolint:gosec
		string
		Cap int
	}{s, len(s)}))
}

// bytesToStringUnsafe is the pre-v9 go-redis reverse trick.
func bytesToStringUnsafe(b []byte) string {
	return *(*string)(unsafe.Pointer(&b)) //nolint:gosec
}

// BenchmarkCompiledEncodeEvalCopy is today's Eval encode: the script is copied.
func BenchmarkCompiledEncodeEvalCopy(b *testing.B) {
	writer := bufio.NewWriter(io.Discard)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := writeCommand(writer, [][]byte{
			[]byte("EVAL"), []byte(evalSizeScript), []byte("1"), []byte("bucket:1.2.3.4"),
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
			stringToBytesUnsafe("EVAL"), stringToBytesUnsafe(evalSizeScript),
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
func benchmarkYaegiConvert(b *testing.B, fn string) {
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
	if _, err := interpreter.Eval(fmt.Sprintf(`convertprobe.%s(1)`, fn)); err != nil {
		b.Fatalf("warmup: %v", err)
	}

	b.ResetTimer()
	if _, err := interpreter.Eval(fmt.Sprintf(`convertprobe.%s(%d)`, fn, b.N)); err != nil {
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

const script = ` + "`" + evalSizeScript + "`" + `

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
