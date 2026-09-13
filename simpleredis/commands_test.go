package simpleredis

import (
	"context"
	"errors"
	"testing"
)

func TestGetHitAndMiss(t *testing.T) {
	_, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	redis := New(Config{Host: addr})

	got, err := redis.Get(context.Background(), "hit")
	if err != nil {
		t.Fatalf("Get hit: %v", err)
	}
	if string(got) != "t" {
		t.Fatalf("Get hit = %q, want %q", got, "t")
	}

	if _, err = redis.Get(context.Background(), "missing"); err == nil || err.Error() != RedisMiss {
		t.Fatalf("Get missing = %v, want %s", err, RedisMiss)
	}
}

func TestGetNullBulkIsMiss(t *testing.T) {
	addr := startStaticRedis(t, "$-1\r\n")
	client := New(Config{Host: addr, MaxRetries: -1})
	got, err := client.Get(context.Background(), "missing")
	if !errors.Is(err, ErrMiss) {
		t.Fatalf("Get $-1: got=%q err=%v, want redis:miss", got, err)
	}
}

func TestGetEmptyBulkIsNotMiss(t *testing.T) {
	addr := startStaticRedis(t, "$0\r\n\r\n")
	client := New(Config{Host: addr, MaxRetries: -1})
	got, err := client.Get(context.Background(), "empty")
	if errors.Is(err, ErrMiss) {
		t.Fatalf("Get $0: got=%q err=%v, want empty bytes not redis:miss", got, err)
	}
	if err != nil {
		t.Fatalf("Get $0: err=%v, want nil error", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("Get $0: got=%q, want non-nil empty slice", got)
	}
}

func TestSetSendsExpire(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{})
	redis := New(Config{Host: addr})
	if err := redis.Set(context.Background(), "k", []byte("v"), 60); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got := fake.lastSetCommand()
	want := []string{"SET", "k", "v", "EX", "60"}
	if len(got) != len(want) {
		t.Fatalf("SET argv %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SET argv %q, want %q", got, want)
		}
	}
}

func TestMGetHitsMissesAndEmpty(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"a": "t", "c": "f"})
	redis := New(Config{Host: addr})

	got, err := redis.MGet(context.Background(), []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("MGet: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("MGet returned %d values, want 3", len(got))
	}
	if string(got[0]) != "t" || got[1] != nil || string(got[2]) != "f" {
		t.Fatalf("MGet = %q, want [t <nil> f]", got)
	}

	empty, err := redis.MGet(context.Background(), nil)
	if empty != nil || err != nil {
		t.Fatalf("MGet(nil) = %v, %v, want nil, nil", empty, err)
	}
	if fake.connections() != 1 {
		t.Fatalf("MGet opened %d connections, want 1", fake.connections())
	}
}

func TestSetReturnsReplyError(t *testing.T) {
	addr := startStaticRedis(t, "-ERR value is not an integer or out of range\r\n")
	redis := New(Config{Host: addr})

	err := redis.Set(context.Background(), "k", []byte("v"), -1)
	if err == nil {
		t.Fatal("Set swallowed the error reply")
	}
	if err.Error() != "ERR value is not an integer or out of range" {
		t.Fatalf("Set = %v", err)
	}
}

func TestDelSucceeds(t *testing.T) {
	addr := startStaticRedis(t, "+OK\r\n")
	redis := New(Config{Host: addr})

	if err := redis.Del(context.Background(), "k"); err != nil {
		t.Fatalf("Del = %v", err)
	}
}

func TestDelIntegerReplySucceeds(t *testing.T) {
	addr := startStaticRedis(t, ":1\r\n")
	redis := New(Config{Host: addr})
	if err := redis.Del(context.Background(), "k"); err != nil {
		t.Fatalf("Del against :1 = %v", err)
	}
}

func TestIncrMissingThenPresent(t *testing.T) {
	_, addr := startFakeRedis(t, map[string]string{})
	redis := New(Config{Host: addr})

	first, err := redis.Incr(context.Background(), "counter")
	if err != nil {
		t.Fatalf("first Incr: %v", err)
	}
	if first != 1 {
		t.Fatalf("first Incr = %d, want 1", first)
	}
	second, err := redis.Incr(context.Background(), "counter")
	if err != nil {
		t.Fatalf("second Incr: %v", err)
	}
	if second != 2 {
		t.Fatalf("second Incr = %d, want 2", second)
	}
}

func TestIncrByMissing(t *testing.T) {
	_, addr := startFakeRedis(t, map[string]string{})
	redis := New(Config{Host: addr})

	got, err := redis.IncrBy(context.Background(), "counter", 5)
	if err != nil {
		t.Fatalf("IncrBy: %v", err)
	}
	if got != 5 {
		t.Fatalf("IncrBy = %d, want 5", got)
	}
}

func TestIncrNonIntegerValue(t *testing.T) {
	_, addr := startFakeRedis(t, map[string]string{"k": "abc"})
	redis := New(Config{Host: addr})

	_, err := redis.Incr(context.Background(), "k")
	if err == nil {
		t.Fatal("Incr non-integer: want error")
	}
	if err.Error() == RedisIssue {
		t.Fatalf("Incr non-integer = %v, want Redis error text", err)
	}
}

func TestExpireAndExpireAtArgv(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"k": "1"})
	redis := New(Config{Host: addr})

	if err := redis.Expire(context.Background(), "k", 60); err != nil {
		t.Fatalf("Expire: %v", err)
	}
	got := fake.lastExpireCommand()
	if len(got) != 3 || got[0] != "EXPIRE" || got[1] != "k" || got[2] != "60" {
		t.Fatalf("Expire argv = %v, want EXPIRE k 60", got)
	}

	if err := redis.ExpireAt(context.Background(), "k", 1700000000); err != nil {
		t.Fatalf("ExpireAt: %v", err)
	}
	got = fake.lastExpireCommand()
	if len(got) != 3 || got[0] != "EXPIREAT" || got[1] != "k" || got[2] != "1700000000" {
		t.Fatalf("ExpireAt argv = %v, want EXPIREAT k 1700000000", got)
	}
}

func TestExpireZeroReplyIsSuccess(t *testing.T) {
	addr := startStaticRedis(t, ":0\r\n")
	redis := New(Config{Host: addr})
	if err := redis.Expire(context.Background(), "missing", 30); err != nil {
		t.Fatalf("Expire :0: %v", err)
	}
}

func TestVerbArityMismatchIsIssueAndPooled(t *testing.T) {
	cases := []struct {
		name    string
		reply   string
		command func(*SimpleRedis) error
	}{
		{"get-empty-array", "*0\r\n", func(redis *SimpleRedis) error {
			_, err := redis.Get(context.Background(), "k")
			return err
		}},
		{"get-two-bulks", "*2\r\n$1\r\na\r\n$1\r\nb\r\n", func(redis *SimpleRedis) error {
			_, err := redis.Get(context.Background(), "k")
			return err
		}},
		{"incr-empty-array", "*0\r\n", func(redis *SimpleRedis) error {
			_, err := redis.Incr(context.Background(), "k")
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			addr := startStaticRedis(t, tc.reply)
			redis := New(Config{Host: addr})
			err := tc.command(redis)
			if err == nil || err.Error() != RedisIssue {
				t.Fatalf("command = %v, want %s", err, RedisIssue)
			}
			if len(redis.idleConns) != 1 {
				t.Fatalf("idle = %d, want 1", len(redis.idleConns))
			}
		})
	}
}
