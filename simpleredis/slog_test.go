package simpleredis

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// recHandler records slog lines so tests can assert msg, level, and attrs.
type recHandler struct {
	mu   sync.Mutex
	recs []slog.Record
}

// Enabled always captures.
func (h *recHandler) Enabled(context.Context, slog.Level) bool { return true }

// Handle stores a clone of the record.
func (h *recHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	h.recs = append(h.recs, r.Clone())
	h.mu.Unlock()
	return nil
}

// WithAttrs returns the same handler; tests do not attach handler attrs.
func (h *recHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

// WithGroup returns the same handler; tests do not use slog groups.
func (h *recHandler) WithGroup(string) slog.Handler { return h }

// records is a snapshot of captured lines.
func (h *recHandler) records() []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]slog.Record, len(h.recs))
	copy(out, h.recs)
	return out
}

// dump is message plus attrs for failure text.
func (h *recHandler) dump() string {
	var b strings.Builder
	for _, r := range h.records() {
		b.WriteString(r.Message)
		r.Attrs(func(a slog.Attr) bool {
			b.WriteByte(' ')
			b.WriteString(a.Key)
			b.WriteByte('=')
			b.WriteString(a.Value.String())
			return true
		})
		b.WriteByte('\n')
	}
	return b.String()
}

// recLogger wraps h as slog.Logger.
func recLogger(h slog.Handler) *slog.Logger { return slog.New(h) }

// requireMsg fails unless msg was logged at level.
func requireMsg(t *testing.T, h *recHandler, msg string, level slog.Level) slog.Record {
	t.Helper()
	for _, r := range h.records() {
		if r.Message == msg {
			if r.Level != level {
				t.Fatalf("%s level = %s, want %s", msg, r.Level, level)
			}
			return r
		}
	}
	t.Fatalf("missing event %s\n%s", msg, h.dump())
	return slog.Record{}
}

func TestLogOpen(t *testing.T) {
	h := &recHandler{}
	sr := New(Config{Host: "127.0.0.1:1", Logger: recLogger(h)})
	requireMsg(t, h, "simpleredis_open", slog.LevelDebug)
	if strings.Contains(h.dump(), "Pass") || strings.Contains(strings.ToLower(h.dump()), " pass=") {
		t.Fatalf("open leaked a pass attr:\n%s", h.dump())
	}
	sr.Close()
}

func TestLogNoAuth(t *testing.T) {
	h := &recHandler{}
	addr := startStaticRedis(t, "-WRONGPASS invalid password\r\n")
	sr := New(Config{Host: addr, Logger: recLogger(h), MaxRetries: -1})
	_, err := sr.Get(context.Background(), "a")
	if err == nil || err.Error() != RedisNoAuth {
		t.Fatalf("Get = %v, want %s", err, RedisNoAuth)
	}
	requireMsg(t, h, "simpleredis_noauth", slog.LevelError)
}

func TestLogOverFree(t *testing.T) {
	h := &recHandler{}
	sr := New(Config{Host: "127.0.0.1:1", PoolSize: 2, Logger: recLogger(h)})
	sr.freeInUseTurn()
	requireMsg(t, h, "simpleredis_over_free", slog.LevelError)
}

func TestLogPoolExhausted(t *testing.T) {
	h := &recHandler{}
	_, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	sr := New(Config{Host: addr, PoolSize: 1, PoolTimeout: 20 * time.Millisecond, MaxRetries: -1, Logger: recLogger(h)})
	conn, err, _ := sr.borrow(context.Background())
	if err != nil {
		t.Fatalf("borrow: %v", err)
	}
	t.Cleanup(func() { sr.release(conn, true) })
	_, waitErr := sr.Get(context.Background(), "hit")
	if !IsPoolWait(waitErr) {
		t.Fatalf("Get = %v, want pool wait", waitErr)
	}
	requireMsg(t, h, "simpleredis_pool_exhausted", slog.LevelWarn)
}

func TestLogShortBulk(t *testing.T) {
	h := &recHandler{}
	truncated := append([]byte("$100\r\n"), bytes.Repeat([]byte("x"), 40)...)
	addr := startRawReplyRedis(t, []rawReply{{payload: truncated, closeAfter: true}})
	sr := New(Config{Host: addr, MaxRetries: -1, Logger: recLogger(h)})
	_, err := sr.Get(context.Background(), "k")
	if err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("Get = %v, want %s", err, RedisUnreachable)
	}
	requireMsg(t, h, "simpleredis_short_bulk", slog.LevelWarn)
}

func TestLogBadReply(t *testing.T) {
	h := &recHandler{}
	addr := startStaticRedis(t, "$abc\r\n")
	sr := New(Config{Host: addr, MaxRetries: -1, Logger: recLogger(h)})
	_, err := sr.Get(context.Background(), "k")
	if err == nil || err.Error() != RedisIssue {
		t.Fatalf("Get = %v, want %s", err, RedisIssue)
	}
	requireMsg(t, h, "simpleredis_bad_reply", slog.LevelWarn)
}

func TestLogHandshakeFailed(t *testing.T) {
	h := &recHandler{}
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	fake.setHandshakeReplies("-LOADING Redis is loading the dataset in memory\r\n", statusOKReply)
	sr := New(Config{Host: addr, Pass: "p", MaxRetries: -1, Logger: recLogger(h)})
	_, err := sr.Get(context.Background(), "hit")
	if err == nil || !strings.HasPrefix(err.Error(), "LOADING ") {
		t.Fatalf("Get = %v, want LOADING", err)
	}
	requireMsg(t, h, "simpleredis_handshake_failed", slog.LevelWarn)
}

func TestLogSocketPoisonedAndAuthLeftover(t *testing.T) {
	h := &recHandler{}
	_, addr := startStrayExtraReplyFake(t, 1)
	sr := New(Config{Host: addr, PoolSize: 1, MaxRetries: -1, Logger: recLogger(h)})
	if _, err := sr.Get(context.Background(), "k0"); err != nil {
		t.Fatalf("Get: %v", err)
	}
	requireMsg(t, h, "simpleredis_socket_poisoned", slog.LevelWarn)

	h2 := &recHandler{}
	authFake, authAddr := startFakeRedis(t, map[string]string{"hit": "t"})
	authFake.setHandshakeReplies("+OK\r\n$5\r\nSTRAY\r\n", statusOKReply)
	auth := New(Config{Host: authAddr, Pass: "secret", Database: "2", MaxRetries: -1, Logger: recLogger(h2)})
	_, err := auth.Get(context.Background(), "hit")
	if err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("auth leftover Get = %v, want %s", err, RedisUnreachable)
	}
	requireMsg(t, h2, "simpleredis_auth_leftover", slog.LevelWarn)
}

func TestLogDialRetryCapabilityNoScript(t *testing.T) {
	h := &recHandler{}
	fake, addr := startFakeRedis(t, map[string]string{})
	sr := New(Config{Host: addr, Logger: recLogger(h)})
	if err := sr.Set(context.Background(), "k", []byte("v"), 60); err != nil {
		t.Fatalf("Set: %v", err)
	}
	requireMsg(t, h, "simpleredis_dial", slog.LevelDebug)

	if _, err := sr.Eval(context.Background(), "return 1", ScriptSHA1Hex("return 1"), nil, nil); err != nil {
		t.Fatalf("Eval: %v", err)
	}
	requireMsg(t, h, "simpleredis_noscript", slog.LevelDebug)

	fake.setRejectMSetEX()
	if err := sr.MSetEX(context.Background(), []string{"a"}, [][]byte{[]byte("1")}, 60); err != nil {
		t.Fatalf("MSetEX: %v", err)
	}
	requireMsg(t, h, "simpleredis_capability", slog.LevelDebug)
}

func TestLogCanceled(t *testing.T) {
	h := &recHandler{}
	_, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	sr := New(Config{Host: addr, Logger: recLogger(h), MaxRetries: -1, IOTimeout: time.Second})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := sr.Get(ctx, "hit")
	if !errorsIsCanceled(err) {
		t.Fatalf("Get = %v, want canceled", err)
	}
	requireMsg(t, h, "simpleredis_canceled", slog.LevelDebug)
}

func TestLogCanceledWaitForTurn(t *testing.T) {
	h := &recHandler{}
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	hold := make(chan struct{})
	fake.mu.Lock()
	fake.holdCh = hold
	fake.mu.Unlock()
	t.Cleanup(func() { close(hold) })
	sr := New(Config{Host: addr, PoolSize: 1, PoolTimeout: time.Second, IOTimeout: 5 * time.Second, MaxRetries: -1, Logger: recLogger(h)})
	go func() {
		_, _ = sr.Get(context.Background(), "hit")
	}()
	deadline := time.Now().Add(time.Second)
	for {
		fake.mu.Lock()
		held := fake.heldGets
		fake.mu.Unlock()
		if held > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("holder never held")
		}
		time.Sleep(time.Millisecond)
	}
	ctx, cancel := context.WithCancel(context.Background())
	waitErr := make(chan error, 1)
	go func() {
		_, err := sr.Get(ctx, "hit")
		waitErr <- err
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	err := <-waitErr
	if !errorsIsCanceled(err) {
		t.Fatalf("waiter Get = %v, want canceled", err)
	}
	requireMsg(t, h, "simpleredis_canceled", slog.LevelDebug)
}

func TestLogTimeout(t *testing.T) {
	h := &recHandler{}
	addr := startStallRedis(t)
	sr := New(Config{Host: addr, Logger: recLogger(h), MaxRetries: -1, IOTimeout: 30 * time.Millisecond})
	_, err := sr.Get(context.Background(), "k")
	if err == nil || err.Error() != RedisTimeout {
		t.Fatalf("Get = %v, want %s", err, RedisTimeout)
	}
	requireMsg(t, h, "simpleredis_timeout", slog.LevelDebug)
}

func TestLogRetry(t *testing.T) {
	h := &recHandler{}
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	sr := New(Config{Host: addr, Logger: recLogger(h), MinRetryBackoff: -1})
	fake.armErrorReplyOnceForTest("-LOADING Redis is loading the dataset in memory\r\n")
	got, err := sr.Get(context.Background(), "hit")
	if err != nil {
		t.Fatalf("Get after LOADING: %v", err)
	}
	if string(got) != "t" {
		t.Fatalf("Get = %q, want t", got)
	}
	requireMsg(t, h, "simpleredis_retry", slog.LevelDebug)
}

func TestLogIdleSwept(t *testing.T) {
	h := &recHandler{}
	_, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	sr := New(Config{Host: addr, Logger: recLogger(h)})
	if _, err := sr.Get(context.Background(), "hit"); err != nil {
		t.Fatalf("first Get: %v", err)
	}
	sr.idleConnsMu.Lock()
	sr.idleConns[0].lastUsed = time.Now().Add(-sr.IdleTimeout() - time.Second)
	sr.idleConnsMu.Unlock()
	if _, err := sr.Get(context.Background(), "hit"); err != nil {
		t.Fatalf("Get after idle timeout: %v", err)
	}
	requireMsg(t, h, "simpleredis_idle_swept", slog.LevelDebug)
}

func TestNilLoggerDoesNotPanic(t *testing.T) {
	_, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	sr := New(Config{Host: addr})
	ctx := context.Background()
	if err := sr.Set(ctx, "k", []byte("v"), 60); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if _, err := sr.Get(ctx, "k"); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if _, err := sr.MGet(ctx, []string{"k"}); err != nil {
		t.Fatalf("MGet: %v", err)
	}
	if err := sr.Del(ctx, "k"); err != nil {
		t.Fatalf("Del: %v", err)
	}
	if _, err := sr.Incr(ctx, "n"); err != nil {
		t.Fatalf("Incr: %v", err)
	}
	if _, err := sr.IncrBy(ctx, "n", 2); err != nil {
		t.Fatalf("IncrBy: %v", err)
	}
	if err := sr.Expire(ctx, "n", 60); err != nil {
		t.Fatalf("Expire: %v", err)
	}
	if err := sr.ExpireAt(ctx, "n", time.Now().Add(time.Minute).Unix()); err != nil {
		t.Fatalf("ExpireAt: %v", err)
	}
	if _, err := sr.Eval(ctx, "return 1", ScriptSHA1Hex("return 1"), nil, nil); err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if err := sr.MSetEX(ctx, []string{"a"}, [][]byte{[]byte("1")}, 60); err != nil {
		t.Fatalf("MSetEX: %v", err)
	}
	if err := sr.MSetEXAt(ctx, []string{"b"}, [][]byte{[]byte("2")}, time.Now().Add(time.Minute).Unix()); err != nil {
		t.Fatalf("MSetEXAt: %v", err)
	}
	sr.Close()
	sr.Close()
	zero := &SimpleRedis{}
	_, _ = zero.Get(ctx, "k")
	zero.Close()
}

func TestSecretsNeverAppearInLogs(t *testing.T) {
	const pass = "PassW0rd-UNIQUE-9f3a"
	const key = "tenant-user-KEY-7c2e"
	h := &recHandler{}
	fake, addr := startFakeRedis(t, map[string]string{})
	fake.setHandshakeReplies("-WRONGPASS invalid password\r\n", statusOKReply)
	sr := New(Config{Host: addr, Pass: pass, Logger: recLogger(h), MaxRetries: -1})
	_, _ = sr.Get(context.Background(), key)

	okFake, okAddr := startFakeRedis(t, map[string]string{})
	_ = okFake
	ok := New(Config{Host: okAddr, Pass: pass, Logger: recLogger(h)})
	_ = ok.Set(context.Background(), key, []byte("secret-value-should-not-log"), 60)
	_, _ = ok.Get(context.Background(), key)
	_, _ = ok.Eval(context.Background(), "return 1", ScriptSHA1Hex("return 1"), []string{key}, []string{"x"})
	ok.Close()
	sr.Close()

	dump := h.dump()
	if strings.Contains(dump, pass) {
		t.Fatalf("password leaked in logs:\n%s", dump)
	}
	if strings.Contains(dump, key) {
		t.Fatalf("key name leaked in logs:\n%s", dump)
	}
	if strings.Contains(dump, "secret-value-should-not-log") {
		t.Fatalf("value leaked in logs:\n%s", dump)
	}
}
