package tokenbucket

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSimpleredisBenchUsesAllowScript keeps the Eval benchmark payload in simpleredis identical to allowScript,
// so the benchmark encodes the script this package actually sends. simpleredis cannot import tokenbucket, so it
// keeps a copy; this test fails when the two drift.
func TestSimpleredisBenchUsesAllowScript(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "simpleredis", "zzz_bench_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	want := "const tokenBucketScript = `" + allowScript + "`"
	if !strings.Contains(strings.ReplaceAll(string(src), "\r\n", "\n"), want) {
		t.Fatal("simpleredis/zzz_bench_test.go tokenBucketScript differs from tokenbucket allowScript; copy allowScript verbatim")
	}
}
