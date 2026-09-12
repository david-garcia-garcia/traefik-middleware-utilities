package simpleredis

import (
	"strings"
	"testing"
	"time"
)

func TestMSetEXNativeArgv(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{})
	redis := New(Config{Host: addr})

	if err := redis.MSetEX([]string{"a", "b"}, [][]byte{[]byte("1"), []byte("2")}, 60); err != nil {
		t.Fatalf("MSetEX: %v", err)
	}
	got := fake.lastMSetEXCommand()
	want := []string{"MSETEX", "2", "a", "1", "b", "2", "EX", "60"}
	if len(got) != len(want) {
		t.Fatalf("MSETEX argv %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("MSETEX argv %q, want %q", got, want)
		}
	}

	hit, err := redis.Get("a")
	if err != nil {
		t.Fatalf("Get a: %v", err)
	}
	if string(hit) != "1" {
		t.Fatalf("Get a = %q, want 1", hit)
	}
}

func TestMSetEXAtArgv(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{})
	redis := New(Config{Host: addr})

	if err := redis.MSetEXAt([]string{"k"}, [][]byte{[]byte("v")}, 1700000000); err != nil {
		t.Fatalf("MSetEXAt: %v", err)
	}
	got := fake.lastMSetEXCommand()
	want := []string{"MSETEX", "1", "k", "v", "EXAT", "1700000000"}
	if len(got) != len(want) {
		t.Fatalf("MSETEX argv %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("MSETEX argv %q, want %q", got, want)
		}
	}
}

func TestMSetEXIntegerZeroIsIssue(t *testing.T) {
	addr := startStaticRedis(t, ":0\r\n")
	redis := New(Config{Host: addr})
	if err := redis.MSetEX([]string{"k"}, [][]byte{[]byte("v")}, 60); err == nil || err.Error() != RedisIssue {
		t.Fatalf("MSetEX :0 = %v, want %s", err, RedisIssue)
	}
}

func TestMSetEXRejectsEmptyMismatchAndCap(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{})
	redis := New(Config{Host: addr})

	if err := redis.MSetEX(nil, nil, 60); err == nil || err.Error() != RedisIssue {
		t.Fatalf("empty = %v, want %s", err, RedisIssue)
	}
	if err := redis.MSetEX([]string{}, [][]byte{}, 60); err == nil || err.Error() != RedisIssue {
		t.Fatalf("empty slice = %v, want %s", err, RedisIssue)
	}
	if err := redis.MSetEX([]string{"a"}, nil, 60); err == nil || err.Error() != RedisIssue {
		t.Fatalf("mismatch = %v, want %s", err, RedisIssue)
	}
	names := make([]string, maxMSetEXPairs+1)
	values := make([][]byte, maxMSetEXPairs+1)
	for i := range names {
		names[i] = "k"
		values[i] = []byte("v")
	}
	if err := redis.MSetEX(names, values, 60); err == nil || err.Error() != RedisIssue {
		t.Fatalf("over cap = %v, want %s", err, RedisIssue)
	}
	if fake.connections() != 0 {
		t.Fatalf("opened %d connections, want 0", fake.connections())
	}
}

func TestMSetEXUnknownCommandFallsBackAndCaches(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{})
	fake.setRejectMSetEX()
	redis := New(Config{Host: addr})

	if err := redis.MSetEX([]string{"a", "b"}, [][]byte{[]byte("1"), []byte("2")}, 60); err != nil {
		t.Fatalf("first MSetEX: %v", err)
	}
	got := fake.lastEvalCommand()
	if len(got) != 9 || got[0] != evalVerb || got[1] != msetexFallbackScript || got[2] != "2" || got[3] != "a" || got[4] != "b" || got[5] != "1" || got[6] != "2" || got[7] != "EX" || got[8] != "60" {
		t.Fatalf("EVAL argv = %q", got)
	}
	if fake.msetexSendCount() != 1 {
		t.Fatalf("MSETEX sends after first = %d, want 1", fake.msetexSendCount())
	}

	if err := redis.MSetEX([]string{"c"}, [][]byte{[]byte("3")}, 30); err != nil {
		t.Fatalf("second MSetEX: %v", err)
	}
	if fake.msetexSendCount() != 1 {
		t.Fatalf("MSETEX sends after cache = %d, want 1", fake.msetexSendCount())
	}

	hit, err := redis.Get("a")
	if err != nil {
		t.Fatalf("Get a: %v", err)
	}
	if string(hit) != "1" {
		t.Fatalf("Get a = %q, want 1", hit)
	}
}

func TestMSetEXLuaScriptIs51Safe(t *testing.T) {
	if !strings.Contains(msetexFallbackScript, "for i = 1, #KEYS") {
		t.Fatalf("script missing numeric KEYS loop: %s", msetexFallbackScript)
	}
	if !strings.Contains(msetexFallbackScript, "redis.call('SET', KEYS[i], ARGV[i], token, ttl)") {
		t.Fatalf("script missing SET call: %s", msetexFallbackScript)
	}
	for _, banned := range []string{"table.unpack", "table.maxn"} {
		if strings.Contains(msetexFallbackScript, banned) {
			t.Fatalf("script contains %s", banned)
		}
	}
	if strings.Contains(msetexFallbackScript, "unpack(") {
		t.Fatalf("script contains unpack: %s", msetexFallbackScript)
	}
}

func TestMSetEXAtPastIsMiss(t *testing.T) {
	_, addr := startFakeRedis(t, map[string]string{})
	redis := New(Config{Host: addr})

	if err := redis.MSetEXAt([]string{"gone"}, [][]byte{[]byte("v")}, time.Now().Unix()-10); err != nil {
		t.Fatalf("MSetEXAt past: %v", err)
	}
	if _, err := redis.Get("gone"); err == nil || err.Error() != RedisMiss {
		t.Fatalf("Get after past EXAT = %v, want %s", err, RedisMiss)
	}
}
