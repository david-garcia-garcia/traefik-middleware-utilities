package simpleredis

import (
	"context"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestUnreachableHost(t *testing.T) {
	redis := newTestRedis(t, Config{Host: "127.0.0.1:1"})

	if _, err := redis.Get(context.Background(), "a"); err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("Get = %v, want %s", err, RedisUnreachable)
	}
	if _, err := redis.MGet(context.Background(), []string{"a"}); err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("MGet = %v, want %s", err, RedisUnreachable)
	}
}

func TestCloseDrainsIdleAndDoesNotRedial(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	redis := newTestRedis(t, Config{Host: addr})

	if _, err := redis.Get(context.Background(), "hit"); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(redis.idleConns) != 1 {
		t.Fatalf("after Get idle = %d, want 1", len(redis.idleConns))
	}

	redis.Close()
	if len(redis.idleConns) != 0 {
		t.Fatalf("after Close idle = %d, want 0", len(redis.idleConns))
	}
	redis.Close()

	if _, err := redis.Get(context.Background(), "hit"); err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("Get after Close = %v, want %s", err, RedisUnreachable)
	}
	if fake.connections() != 1 {
		t.Fatalf("Get after Close opened %d connections, want 1", fake.connections())
	}
}

// TestCloseDuringInFlightCommandClosesSocketOnRelease closes while a Get is held, then asserts idle empty and no redial.
func TestCloseDuringInFlightCommandClosesSocketOnRelease(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	fake.holdGetsForTest(t)
	redis := newTestRedis(t, Config{Host: addr, CommandTimeout: 5 * time.Second})

	errCh := make(chan error, 1)
	go func() {
		_, err := redis.Get(context.Background(), "hit")
		errCh <- err
	}()
	fake.waitHeldGets(t, 1)
	redis.Close()
	fake.releaseHeldGetsForTest()
	if err := <-errCh; err != nil {
		t.Fatalf("in-flight Get: %v", err)
	}
	if got := len(redis.idleConns); got != 0 {
		t.Fatalf("idle after in-flight Close = %d, want 0", got)
	}
	fake.waitOpenSocketsEqual(t, 0)
	if _, err := redis.Get(context.Background(), "hit"); err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("Get after Close = %v, want %s", err, RedisUnreachable)
	}
	if fake.connections() != 1 {
		t.Fatalf("Get after Close accepted %d, want 1", fake.connections())
	}
}

func TestClosedClientUnreachableIsNotRetried(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	redis := newTestRedis(t, Config{Host: addr})
	if _, err := redis.Get(context.Background(), "hit"); err != nil {
		t.Fatalf("Get: %v", err)
	}
	redis.Close()

	started := time.Now()
	_, err := redis.Get(context.Background(), "hit")
	elapsed := time.Since(started)
	if err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("Get after Close = %v, want %s", err, RedisUnreachable)
	}
	if elapsed >= 50*time.Millisecond {
		t.Fatalf("Get after Close took %v, want no retry backoff", elapsed)
	}
	if fake.connections() != 1 {
		t.Fatalf("opened %d connections, want 1", fake.connections())
	}
}

// TestSessionSourceImports fails if a non-test session file imports unsafe, cgo, or a dotted path.
func TestSessionSourceImports(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	scanned := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		scanned++
		file, parseErr := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", name, parseErr)
		}
		for _, imp := range file.Imports {
			path, unquoteErr := strconv.Unquote(imp.Path.Value)
			if unquoteErr != nil {
				t.Fatalf("unquote %s: %v", imp.Path.Value, unquoteErr)
			}
			if path == "unsafe" || path == "C" || strings.Contains(path, ".") {
				t.Fatalf("%s imports %q", name, path)
			}
		}
	}
	if scanned == 0 {
		t.Fatal("no session source files scanned")
	}
}

// TestSimpleRedisProbeUseUnsafeOff fails if the probe manifest or compose turns useUnsafe on.
func TestSimpleRedisProbeUseUnsafeOff(t *testing.T) {
	manifest, err := os.ReadFile(filepath.Join("..", "e2e", "simpleredisprobe", ".traefik.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if manifestUseUnsafeTrue(string(manifest)) {
		t.Fatal("simpleredisprobe manifest useUnsafe is true")
	}

	compose, err := os.ReadFile(filepath.Join("..", "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, line := range strings.Split(string(compose), "\n") {
		if strings.Contains(line, "reclaimprobe") {
			continue
		}
		const needle = "simpleredisprobe.settings.useunsafe="
		lower := strings.ToLower(line)
		idx := strings.Index(lower, needle)
		if idx < 0 {
			continue
		}
		found = true
		value := strings.TrimSpace(line[idx+len(needle):])
		value = strings.Trim(value, `"'`)
		if strings.EqualFold(value, "true") {
			t.Fatal("compose simpleredisprobe.settings.useunsafe is true")
		}
	}
	if !found {
		t.Fatal("compose has no simpleredisprobe.settings.useunsafe assignment")
	}
}

// manifestUseUnsafeTrue reports whether a Traefik plugin manifest sets useUnsafe true.
func manifestUseUnsafeTrue(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		lower := strings.ToLower(trimmed)
		const key = "useunsafe:"
		if !strings.HasPrefix(lower, key) {
			continue
		}
		value := strings.TrimSpace(trimmed[len(key):])
		value = strings.Trim(value, `"'`)
		return strings.EqualFold(value, "true")
	}
	return false
}
