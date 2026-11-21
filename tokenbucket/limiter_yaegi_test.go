package tokenbucket

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

func TestYaegi_AllowBurst(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	goPath := t.TempDir()
	writeGopathTokenbucket(t, goPath)
	writeGopathFile(t, goPath, "allowprobe", "roundtrip.go", allowprobeSrc)

	got := evalAllowprobe(t, goPath, fmt.Sprintf(`allowprobe.BurstAfterIdle(%q, %q)`, addr, "yaegi-k"))
	if got != "ok" {
		t.Fatalf("yaegi allow: %q, want ok", got)
	}
}

func TestYaegi_AllowRefund(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	goPath := t.TempDir()
	writeGopathTokenbucket(t, goPath)
	writeGopathFile(t, goPath, "allowprobe", "roundtrip.go", allowprobeSrc)

	got := evalAllowprobe(t, goPath, fmt.Sprintf(`allowprobe.RefundAfterBurst(%q, %q)`, addr, "yaegi-refund"))
	if got != "ok" {
		t.Fatalf("yaegi refund: %q, want ok", got)
	}
}

// TestYaegi_MemoryAgrees runs the interpreted memory/Redis status agreement probe against the compiled fake.
func TestYaegi_MemoryAgrees(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	goPath := t.TempDir()
	writeGopathTokenbucket(t, goPath)
	writeGopathFile(t, goPath, "allowprobe", "roundtrip.go", allowprobeSrc)

	got := evalAllowprobe(t, goPath, fmt.Sprintf(`allowprobe.MemoryAgrees(%q, %q)`, addr, "yaegi-agree"))
	if got != "ok" {
		t.Fatalf("yaegi memory/redis agreement: %q, want ok", got)
	}
}

// evalAllowprobe evaluates expr in a GOPATH interp with stdlib only (no unsafe).
func evalAllowprobe(t *testing.T, goPath, expr string) string {
	t.Helper()
	interpreter := interp.New(interp.Options{GoPath: goPath})
	if err := interpreter.Use(stdlib.Symbols); err != nil {
		t.Fatalf("use stdlib: %v", err)
	}
	if _, err := interpreter.Eval(`import "allowprobe"`); err != nil {
		t.Fatalf("import allowprobe: %v", err)
	}
	evaluated, err := interpreter.Eval(expr)
	if err != nil {
		t.Fatalf("eval %s: %v", expr, err)
	}
	return evaluated.Interface().(string)
}

// writeGopathTokenbucket copies non-test tokenbucket and simpleredis sources into a GOPATH module tree.
func writeGopathTokenbucket(tb testing.TB, goPath string) {
	tb.Helper()
	src := callerDir(tb)
	copyNonTestGo(tb, src, goPath, "tokenbucket")
	copyNonTestGo(tb, filepath.Join(filepath.Dir(src), "simpleredis"), goPath, "simpleredis")
}

// copyNonTestGo copies non-test .go files from srcDir into GOPATH/src/.../<pkg>.
func copyNonTestGo(tb testing.TB, srcDir, goPath, pkg string) {
	tb.Helper()
	destDir := filepath.Join(goPath, "src", "github.com", "david-garcia-garcia", "traefik-middleware-utilities", pkg)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		tb.Fatal(err)
	}
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		tb.Fatal(err)
	}
	copied := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(srcDir, name))
		if err != nil {
			tb.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(destDir, name), body, 0o600); err != nil {
			tb.Fatal(err)
		}
		copied++
	}
	if copied == 0 {
		tb.Fatalf("no %s sources copied into GOPATH", pkg)
	}
}

// callerDir is the directory of the test file that called writeGopathTokenbucket.
func callerDir(tb testing.TB) string {
	tb.Helper()
	_, thisFile, _, ok := runtime.Caller(1)
	if !ok {
		tb.Fatal("no caller path")
	}
	return filepath.Dir(thisFile)
}

// writeGopathFile writes one interpreted package file under GOPATH/src/<pkg>.
func writeGopathFile(tb testing.TB, goPath, pkg, name, src string) {
	tb.Helper()
	dir := filepath.Join(goPath, "src", pkg)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		tb.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600); err != nil {
		tb.Fatal(err)
	}
}

const allowprobeSrc = `package allowprobe

import (
	"context"
	"strconv"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/tokenbucket"
	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

func BurstAfterIdle(host, key string) string {
	client, err := simpleredis.New(simpleredis.Config{Host: host})
	if err != nil {
		return "new:" + err.Error()
	}
	limiter, err := tokenbucket.NewRedis(client, 1, 3, time.Hour, 2*time.Second)
	if err != nil {
		return "new:" + err.Error()
	}
	now := time.Unix(1700000000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	for i := 0; i < 3; i++ {
		status, allowErr := limiter.Allow(context.Background(), key)
		if allowErr != nil {
			return "allow:" + allowErr.Error()
		}
		if !status.Allowed {
			return "early"
		}
		if status.Remaining != int64(2-i) {
			return "remaining:" + strconv.FormatInt(status.Remaining, 10)
		}
	}
	// The fourth Allow is admitted with a delay: tokens go negative, so nothing remains and nothing is retried.
	status, err := limiter.Allow(context.Background(), key)
	if err != nil {
		return "delay:" + err.Error()
	}
	if !status.Allowed || status.Delay <= 0 || status.RetryAfter != 0 || status.Remaining != 0 {
		return "not-delayed"
	}
	return "ok"
}

func TwoShare(host, key string) string {
	aClient, err := simpleredis.New(simpleredis.Config{Host: host})
	if err != nil {
		return "new:" + err.Error()
	}
	bClient, err := simpleredis.New(simpleredis.Config{Host: host})
	if err != nil {
		return "new:" + err.Error()
	}
	a, err := tokenbucket.NewRedis(aClient, 1, 3, time.Microsecond, 2*time.Second)
	if err != nil {
		return "new-a:" + err.Error()
	}
	b, err := tokenbucket.NewRedis(bClient, 1, 3, time.Microsecond, 2*time.Second)
	if err != nil {
		return "new-b:" + err.Error()
	}
	now := time.Unix(1700000000, 0)
	a.SetNowForTest(func() time.Time { return now })
	b.SetNowForTest(func() time.Time { return now })
	for i := 0; i < 3; i++ {
		status, allowErr := a.Allow(context.Background(), key)
		if allowErr != nil {
			return "a:" + allowErr.Error()
		}
		if !status.Allowed {
			return "a-early"
		}
	}
	status, err := b.Allow(context.Background(), key)
	if err != nil {
		return "b:" + err.Error()
	}
	if status.Allowed {
		return "double-burst"
	}
	return "ok"
}

func MemoryAgrees(host, key string) string {
	client, err := simpleredis.New(simpleredis.Config{Host: host})
	if err != nil {
		return "new:" + err.Error()
	}
	mem, err := tokenbucket.NewMemory(2, 2, time.Millisecond, 2*time.Second)
	if err != nil {
		return "mem:" + err.Error()
	}
	red, err := tokenbucket.NewRedis(client, 2, 2, time.Millisecond, 2*time.Second)
	if err != nil {
		return "red:" + err.Error()
	}
	now := time.Unix(1700000000, 0)
	mem.SetNowForTest(func() time.Time { return now })
	red.SetNowForTest(func() time.Time { return now })
	for i := 0; i < 4; i++ {
		mStatus, mErr := mem.Allow(context.Background(), key)
		if mErr != nil {
			return "m:" + mErr.Error()
		}
		rStatus, rErr := red.Allow(context.Background(), key)
		if rErr != nil {
			return "r:" + rErr.Error()
		}
		if mStatus.Allowed != rStatus.Allowed {
			return "allowed-mismatch"
		}
		// Tokens stay whole numbers here, so the bucket fields match exactly.
		if mStatus.Limit != rStatus.Limit || mStatus.Remaining != rStatus.Remaining || mStatus.Period != rStatus.Period || mStatus.Reset != rStatus.Reset {
			return "bucket-mismatch"
		}
		if (mStatus.Delay == 0) != (rStatus.Delay == 0) || (mStatus.RetryAfter == 0) != (rStatus.RetryAfter == 0) {
			return "wait-mismatch"
		}
		if (mStatus.Delay+mStatus.RetryAfter > time.Millisecond) != (rStatus.Delay+rStatus.RetryAfter > time.Millisecond) {
			return "class-mismatch"
		}
	}
	return "ok"
}

func RefundAfterBurst(host, key string) string {
	client, err := simpleredis.New(simpleredis.Config{Host: host})
	if err != nil {
		return "new:" + err.Error()
	}
	limiter, err := tokenbucket.NewRedis(client, 1, 3, time.Microsecond, 2*time.Second)
	if err != nil {
		return "new:" + err.Error()
	}
	now := time.Unix(1700000000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	for i := 0; i < 3; i++ {
		status, allowErr := limiter.Allow(context.Background(), key)
		if allowErr != nil {
			return "allow:" + allowErr.Error()
		}
		if !status.Allowed {
			return "early"
		}
	}
	// The fourth Allow is denied: the full wait is RetryAfter, and the refunded bucket has nothing left.
	status, err := limiter.Allow(context.Background(), key)
	if err != nil {
		return "deny:" + err.Error()
	}
	if status.Allowed || status.RetryAfter <= time.Microsecond || status.Delay != 0 || status.Remaining != 0 {
		return "not-denied"
	}
	statusAfterRefund, err := limiter.Allow(context.Background(), key)
	if err != nil {
		return "refund:" + err.Error()
	}
	if statusAfterRefund.Allowed || statusAfterRefund.RetryAfter != status.RetryAfter {
		return "stacked"
	}
	return "ok"
}
`
