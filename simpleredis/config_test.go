package simpleredis

import (
	"sync"
	"testing"
	"time"
)

func TestConfigFrozenAtNew(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	fake.mu.Lock()
	fake.getDelay = 300 * time.Millisecond
	fake.mu.Unlock()
	cfg := Config{Host: addr, PoolSize: 1, PoolTimeout: 50 * time.Millisecond, IOTimeout: time.Second}
	redis := New(cfg)
	cfg.PoolSize = 16
	redis.poolSize = 16

	started := make(chan struct{}, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		started <- struct{}{}
		if _, err := redis.Get("hit"); err != nil {
			t.Errorf("holder Get: %v", err)
		}
	}()
	<-started
	deadline := time.Now().Add(time.Second)
	for fake.connections() < 1 {
		if time.Now().After(deadline) {
			t.Fatal("holder did not dial")
		}
		time.Sleep(time.Millisecond)
	}
	_, err := redis.Get("hit")
	if err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("waiter Get = %v, want %s after post-New PoolSize writes", err, RedisUnreachable)
	}
	wg.Wait()
	if got := fake.connections(); got > 1 {
		t.Fatalf("post-New PoolSize write opened %d connections, want at most 1", got)
	}
	if got := redis.PoolSize(); got != 1 {
		t.Fatalf("PoolSize() = %d, want 1", got)
	}
}
