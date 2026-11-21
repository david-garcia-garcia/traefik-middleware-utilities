# Nitpicks

1. [hard] Symmetry and consistency — `simpleredis/fake_redis_test.go:882` — sibling `connections()` getters in this file (`fakeRedis`, `peerCloseFake`) use receiver `f`, `defer` unlock, and return the field; this unit uses `fake`, copies into `n`, then unlocks by hand
```
func (fake *strayExtraReplyFake) connections() int {
	fake.mu.Lock()
	n := fake.conns
	fake.mu.Unlock()
	return n
}
```
   → Match the siblings: receiver `f`, `defer f.mu.Unlock()`, `return f.conns`
   Status: done
   Argument: receiver `f`, defer unlock, return `f.conns`. — local `sr` is the type nickname; this body only uses the client
```
func TestDesyncedSocketDoesNotServePreviousReplies(t *testing.T) {
	_, addr := startStrayExtraReplyFake(t, 5)
	sr := New(Config{Host: addr, PoolSize: 1, MaxRetries: -1})
	const commands = 12
	for i := 0; i < commands; i++ {
		key := "k" + strconv.Itoa(i)
		want := "v" + strconv.Itoa(i)
		got, err := sr.Get(context.Background(), key)
		if err != nil {
			continue
		}
		if string(got) != want {
			t.Fatalf("Get(%s) = %q, want %q or an error (command %d)", key, got, want, i)
		}
	}
}
```
   → `redis := New(...)`; call `redis.Get` (same identifier as the other tests in this file)
   Status: done
   Argument: renamed `sr` to `redis` in both tests. — local `sr` is the type nickname; this body only uses the client
```
func TestStrayExtraReplyIsNotPooled(t *testing.T) {
	fake, addr := startStrayExtraReplyFake(t, 5)
	sr := New(Config{Host: addr, PoolSize: 1, MaxRetries: -1})
	for i := 0; i < 5; i++ {
		key := "k" + strconv.Itoa(i)
		got, err := sr.Get(context.Background(), key)
		...
	}
	if got := pooledIdle(sr); got != 0 {
		...
	}
	got, err := sr.Get(context.Background(), "k5")
	...
}
```
   → `redis := New(...)`; call `redis.Get` and `pooledIdle(redis)`
   Status: done
   Argument: renamed `sr` to `redis`.
