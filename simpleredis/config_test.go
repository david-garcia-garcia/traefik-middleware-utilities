package simpleredis

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestConfigFrozenAtNew(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	fake.mu.Lock()
	fake.getDelay = 300 * time.Millisecond
	fake.mu.Unlock()
	cfg := Config{Host: addr, PoolSize: 1, MaxIdleConns: 1, PoolTimeout: 50 * time.Millisecond, IOTimeout: time.Second}
	redis := newTestRedis(t, cfg)
	cfg.PoolSize = 16
	redis.poolSize = 16

	started := make(chan struct{}, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		started <- struct{}{}
		if _, err := redis.Get(context.Background(), "hit"); err != nil {
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
	_, err := redis.Get(context.Background(), "hit")
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

// TestNewRejectsMaxIdleConnsAbovePoolSize proves New does not create a client when the idle trim cannot be reached.
// No fake Redis: New decides before it dials.
func TestNewRejectsMaxIdleConnsAbovePoolSize(t *testing.T) {
	rows := []struct {
		name string
		cfg  Config
	}{
		{"explicit idle above pool", Config{Host: "127.0.0.1:1", PoolSize: 4, MaxIdleConns: 100}},
		{"explicit idle one above pool", Config{Host: "127.0.0.1:1", PoolSize: 4, MaxIdleConns: 5}},
	}
	for _, row := range rows {
		row := row
		t.Run(row.name, func(t *testing.T) {
			client, err := New(row.cfg)
			if client != nil {
				t.Fatalf("New returned a client")
			}
			if !errors.Is(err, ErrMaxIdleConnsAbovePoolSize) {
				t.Fatalf("New err = %v, want %v", err, ErrMaxIdleConnsAbovePoolSize)
			}
		})
	}
}

// TestNewKeepsValidPoolIdleKnobs proves New freezes idle trim at or below PoolSize without rewriting it.
func TestNewKeepsValidPoolIdleKnobs(t *testing.T) {
	rows := []struct {
		name        string
		cfg         Config
		wantPool    int
		wantMaxIdle int
	}{
		{"explicit idle below pool is kept", Config{Host: "127.0.0.1:1", PoolSize: 8, MaxIdleConns: 2}, 8, 2},
		{"explicit idle equal to pool is kept", Config{Host: "127.0.0.1:1", PoolSize: 3, MaxIdleConns: 3}, 3, 3},
		{"zero Config keeps both defaults", Config{Host: "127.0.0.1:1"}, defaultPoolSize, defaultMaxIdleConns},
		{"default pool keeps a smaller explicit idle", Config{Host: "127.0.0.1:1", MaxIdleConns: 3}, defaultPoolSize, 3},
		// A caller who only shrinks PoolSize never asked for an idle trim of 8, so it follows the pool.
		{"default idle follows a smaller pool", Config{Host: "127.0.0.1:1", PoolSize: 2}, 2, 2},
		{"default idle follows a pool of one", Config{Host: "127.0.0.1:1", PoolSize: 1}, 1, 1},
	}
	for _, row := range rows {
		row := row
		t.Run(row.name, func(t *testing.T) {
			redis := newTestRedis(t, row.cfg)
			if got := redis.PoolSize(); got != row.wantPool {
				t.Fatalf("PoolSize() = %d, want %d", got, row.wantPool)
			}
			if got := redis.MaxIdleConns(); got != row.wantMaxIdle {
				t.Fatalf("MaxIdleConns() = %d, want %d", got, row.wantMaxIdle)
			}
		})
	}
}
