package simpleredis

import (
	"context"
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

// TestMaxIdleConnsClampedToPoolSize proves the frozen idle trim is a value the idle list can reach.
// No fake Redis: New freezes both knobs without dialing, and the accessors are the surface an operator reads.
func TestMaxIdleConnsClampedToPoolSize(t *testing.T) {
	rows := []struct {
		name        string
		cfg         Config
		wantPool    int
		wantMaxIdle int
	}{
		{"explicit idle above pool clamps down", Config{Host: "127.0.0.1:1", PoolSize: 4, MaxIdleConns: 100}, 4, 4},
		{"explicit idle below pool is kept", Config{Host: "127.0.0.1:1", PoolSize: 8, MaxIdleConns: 2}, 8, 2},
		{"explicit idle equal to pool is kept", Config{Host: "127.0.0.1:1", PoolSize: 3, MaxIdleConns: 3}, 3, 3},
		{"zero Config keeps both defaults", Config{Host: "127.0.0.1:1"}, defaultPoolSize, defaultMaxIdleConns},
		{"default idle clamps to a smaller pool", Config{Host: "127.0.0.1:1", PoolSize: 2}, 2, 2},
		{"default pool keeps a smaller explicit idle", Config{Host: "127.0.0.1:1", MaxIdleConns: 3}, defaultPoolSize, 3},
	}
	for _, row := range rows {
		row := row
		t.Run(row.name, func(t *testing.T) {
			redis := New(row.cfg)
			if got := redis.PoolSize(); got != row.wantPool {
				t.Fatalf("PoolSize() = %d, want %d", got, row.wantPool)
			}
			if got := redis.MaxIdleConns(); got != row.wantMaxIdle {
				t.Fatalf("MaxIdleConns() = %d, want %d", got, row.wantMaxIdle)
			}
		})
	}
}
