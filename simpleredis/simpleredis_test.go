package simpleredis

import (
	"context"
	"testing"
	"time"
)

func TestUnreachableHost(t *testing.T) {
	redis := New(Config{Host: "127.0.0.1:1"})

	if _, err := redis.Get(context.Background(), "a"); err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("Get = %v, want %s", err, RedisUnreachable)
	}
	if _, err := redis.MGet(context.Background(), []string{"a"}); err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("MGet = %v, want %s", err, RedisUnreachable)
	}
}

func TestCloseDrainsIdleAndDoesNotRedial(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	redis := New(Config{Host: addr})

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
	redis := New(Config{Host: addr, IOTimeout: 5 * time.Second})

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
	redis := New(Config{Host: addr})
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
