package simpleredis

import (
	"testing"
	"time"
)

func TestUnreachableHost(t *testing.T) {
	redis := New(Config{Host: "127.0.0.1:1"})

	if _, err := redis.Get("a"); err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("Get = %v, want %s", err, RedisUnreachable)
	}
	if _, err := redis.MGet([]string{"a"}); err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("MGet = %v, want %s", err, RedisUnreachable)
	}
}

func TestCloseDrainsIdleAndDoesNotRepool(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	redis := New(Config{Host: addr})

	if _, err := redis.Get("hit"); err != nil {
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

	if _, err := redis.Get("hit"); err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("Get after Close = %v, want %s", err, RedisUnreachable)
	}
	if fake.connections() != 1 {
		t.Fatalf("Get after Close opened %d connections, want 1", fake.connections())
	}
	if len(redis.idleConns) != 0 {
		t.Fatalf("release after Close idle = %d, want 0", len(redis.idleConns))
	}
}

func TestClosedClientUnreachableIsNotRetried(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	redis := New(Config{Host: addr})
	if _, err := redis.Get("hit"); err != nil {
		t.Fatalf("Get: %v", err)
	}
	redis.Close()

	started := time.Now()
	_, err := redis.Get("hit")
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
