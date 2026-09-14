package simpleredis

import (
	"bytes"
	"context"
	"errors"
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
func requireMsg(t *testing.T, h *recHandler, msg string, level slog.Level) {
	t.Helper()
	for _, r := range h.records() {
		if r.Message == msg {
			if r.Level != level {
				t.Fatalf("%s level = %s, want %s", msg, r.Level, level)
			}
			return
		}
	}
	t.Fatalf("missing event %s\n%s", msg, h.dump())
}

// requireMsgAttr fails unless some record for msg carries key=want.
func requireMsgAttr(t *testing.T, h *recHandler, msg, key, want string) {
	t.Helper()
	for _, r := range h.records() {
		if r.Message != msg {
			continue
		}
		matched := false
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == key && a.Value.String() == want {
				matched = true
				return false
			}
			return true
		})
		if matched {
			return
		}
	}
	t.Fatalf("missing event %s with %s=%s\n%s", msg, key, want, h.dump())
}

func TestLogOpen(t *testing.T) {
	h := &recHandler{}
	sr := newTestRedis(t, Config{Host: "127.0.0.1:1", Logger: recLogger(h)})
	requireMsg(t, h, "simpleredis_open", slog.LevelDebug)
	if strings.Contains(h.dump(), "Pass") || strings.Contains(strings.ToLower(h.dump()), " pass=") {
		t.Fatalf("open leaked a pass attr:\n%s", h.dump())
	}
	sr.Close()
}

func TestLogNoAuth(t *testing.T) {
	h := &recHandler{}
	addr := startStaticRedis(t, "-WRONGPASS invalid password\r\n")
	sr := newTestRedis(t, Config{Host: addr, Logger: recLogger(h), MaxRetries: -1})
	_, err := sr.Get(context.Background(), "a")
	if err == nil || err.Error() != RedisNoAuth {
		t.Fatalf("Get = %v, want %s", err, RedisNoAuth)
	}
	requireMsg(t, h, "do: "+RedisNoAuth, slog.LevelError)
	requireMsgAttr(t, h, "do: "+RedisNoAuth, "verb", "GET")
}

func TestLogOverFree(t *testing.T) {
	h := &recHandler{}
	sr := newTestRedis(t, Config{Host: "127.0.0.1:1", PoolSize: 2, Logger: recLogger(h)})
	sr.freeInUseTurn()
	requireMsg(t, h, "simpleredis_over_free", slog.LevelError)
}

func TestLogPoolExhausted(t *testing.T) {
	h := &recHandler{}
	_, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	sr := newTestRedis(t, Config{Host: addr, PoolSize: 1, PoolTimeout: 20 * time.Millisecond, MaxRetries: -1, Logger: recLogger(h)})
	conn, err, _, _ := sr.borrowSocket(context.Background(), false)
	if err != nil {
		t.Fatalf("borrow: %v", err)
	}
	t.Cleanup(func() { sr.release(conn, true) })
	_, waitErr := sr.Get(context.Background(), "hit")
	if !IsPoolWait(waitErr) {
		t.Fatalf("Get = %v, want pool wait", waitErr)
	}
	requireMsg(t, h, "borrowSocket: "+RedisUnreachable, slog.LevelWarn)
	requireMsgAttr(t, h, "borrowSocket: "+RedisUnreachable, "reason", "pool_wait")
}

// TestLogUnreachableNamesItsSite is the point of site logging: one redis:unreachable text, told apart by where it
// was seen. A closed port is dial; a client that never came from New is borrowSocket.
func TestLogUnreachableNamesItsSite(t *testing.T) {
	h := &recHandler{}
	sr := newTestRedis(t, Config{Host: "127.0.0.1:1", Logger: recLogger(h), MaxRetries: -1,
		DialTimeout: 50 * time.Millisecond})
	_, err := sr.Get(context.Background(), "k")
	if err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("Get = %v, want %s", err, RedisUnreachable)
	}
	requireMsg(t, h, "dial: "+RedisUnreachable, slog.LevelDebug)

	sr.Close()
	if _, err := sr.Get(context.Background(), "k"); err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("Get after Close = %v, want %s", err, RedisUnreachable)
	}
	requireMsgAttr(t, h, "borrowSocket: "+RedisUnreachable, "reason", "closed")
}

func TestLogShortBulk(t *testing.T) {
	h := &recHandler{}
	truncated := append([]byte("$100\r\n"), bytes.Repeat([]byte("x"), 40)...)
	addr := startRawReplyRedis(t, []rawReply{{payload: truncated, closeAfter: true}})
	sr := newTestRedis(t, Config{Host: addr, MaxRetries: -1, Logger: recLogger(h)})
	_, err := sr.Get(context.Background(), "k")
	if err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("Get = %v, want %s", err, RedisUnreachable)
	}
	requireMsg(t, h, "do: "+RedisUnreachable, slog.LevelWarn)
	requireMsgAttr(t, h, "do: "+RedisUnreachable, "reason", "read")
}

func TestLogBadReply(t *testing.T) {
	h := &recHandler{}
	addr := startStaticRedis(t, "$abc\r\n")
	sr := newTestRedis(t, Config{Host: addr, MaxRetries: -1, Logger: recLogger(h)})
	_, err := sr.Get(context.Background(), "k")
	if err == nil || err.Error() != RedisIssue {
		t.Fatalf("Get = %v, want %s", err, RedisIssue)
	}
	requireMsg(t, h, "do: redis error", slog.LevelWarn)
}

func TestLogHandshakeFailed(t *testing.T) {
	h := &recHandler{}
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	fake.setHandshakeReplies("-LOADING Redis is loading the dataset in memory\r\n", statusOKReply)
	sr := newTestRedis(t, Config{Host: addr, Pass: "p", MaxRetries: -1, Logger: recLogger(h)})
	_, err := sr.Get(context.Background(), "hit")
	if err == nil || !strings.HasPrefix(err.Error(), "LOADING ") {
		t.Fatalf("Get = %v, want LOADING", err)
	}
	// The tail of a Redis error reply is peer text and must not appear on the line.
	if strings.Contains(h.dump(), "loading the dataset") {
		t.Fatalf("peer reply text published:\n%s", h.dump())
	}
	requireMsgAttr(t, h, "dial: redis error", "verb", verbAuth)
	requireMsg(t, h, "dial: redis error", slog.LevelWarn)
	requireMsg(t, h, "do: redis error", slog.LevelDebug)
}

// TestLogPeerReplyIsNotPublished pins that Redis key names stay off log lines:
// Redis refuses an unknown verb by quoting the command's own arguments back.
func TestLogPeerReplyIsNotPublished(t *testing.T) {
	const key = "tenant-user-KEY-7c2e"
	h := &recHandler{}
	addr := startStaticRedis(t, "-ERR unknown command 'GET', with args beginning with: '"+key+"'\r\n")
	sr := newTestRedis(t, Config{Host: addr, MaxRetries: -1, Logger: recLogger(h)})
	_, err := sr.Get(context.Background(), key)
	if err == nil || !strings.Contains(err.Error(), key) {
		t.Fatalf("Get = %v, want the peer text (the caller still gets the full error)", err)
	}
	if strings.Contains(h.dump(), key) {
		t.Fatalf("key name leaked in logs:\n%s", h.dump())
	}
	requireMsg(t, h, "do: redis error", slog.LevelDebug)
}

// TestLogHandshakeFailedNeverEchoesPass pins the one handshake reply that carries the password:
// Redis 7.4 answers AUTH against a nopass default user with "ERR AUTH <password> called without…",
// which is not AUTH-class, so a handshake event that logged the error text would publish Pass.
func TestLogHandshakeFailedNeverEchoesPass(t *testing.T) {
	const pass = "PassW0rd-UNIQUE-9f3a"
	h := &recHandler{}
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	fake.setHandshakeReplies("-ERR AUTH "+pass+" called without any password configured for the default user\r\n", statusOKReply)
	sr := newTestRedis(t, Config{Host: addr, Pass: pass, MaxRetries: -1, Logger: recLogger(h)})

	_, err := sr.Get(context.Background(), "hit")
	if err == nil || !strings.Contains(err.Error(), pass) {
		t.Fatalf("Get = %v, want the peer text (the caller still gets the full error)", err)
	}
	// The leak check comes first on purpose: it must fail on its own if a site logs the peer text.
	if strings.Contains(h.dump(), pass) {
		t.Fatalf("password leaked in logs:\n%s", h.dump())
	}
	requireMsgAttr(t, h, "dial: redis error", "verb", verbAuth)
	requireMsg(t, h, "dial: redis error", slog.LevelWarn)
}

func TestLogSocketPoisonedAndAuthLeftover(t *testing.T) {
	h := &recHandler{}
	_, addr := startStrayExtraReplyFake(t, 1)
	sr := newTestRedis(t, Config{Host: addr, PoolSize: 1, MaxRetries: -1, Logger: recLogger(h)})
	if _, err := sr.Get(context.Background(), "k0"); err != nil {
		t.Fatalf("Get: %v", err)
	}
	requireMsg(t, h, "simpleredis_socket_poisoned", slog.LevelWarn)

	h2 := &recHandler{}
	authFake, authAddr := startFakeRedis(t, map[string]string{"hit": "t"})
	authFake.setHandshakeReplies("+OK\r\n$5\r\nSTRAY\r\n", statusOKReply)
	auth := newTestRedis(t, Config{Host: authAddr, Pass: "secret", Database: "2", MaxRetries: -1, Logger: recLogger(h2)})
	_, err := auth.Get(context.Background(), "hit")
	if err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("auth leftover Get = %v, want %s", err, RedisUnreachable)
	}
	requireMsg(t, h2, "do: "+RedisUnreachable, slog.LevelWarn)
	requireMsgAttr(t, h2, "do: "+RedisUnreachable, "reason", "unread_before_write")
	// The stray arrived on the AUTH reply, so the verb refused before its write is SELECT.
	requireMsgAttr(t, h2, "do: "+RedisUnreachable, "verb", verbSelect)
}

func TestLogDialRetryCapabilityNoScript(t *testing.T) {
	h := &recHandler{}
	fake, addr := startFakeRedis(t, map[string]string{})
	sr := newTestRedis(t, Config{Host: addr, Logger: recLogger(h)})
	if err := sr.Set(context.Background(), "k", []byte("v"), 60); err != nil {
		t.Fatalf("Set: %v", err)
	}
	requireMsg(t, h, "simpleredis_dial", slog.LevelDebug)

	if _, err := sr.Eval(context.Background(), "return 1", ScriptSHA1Hex("return 1"), nil, nil); err != nil {
		t.Fatalf("Eval: %v", err)
	}
	requireMsg(t, h, "do: redis error", slog.LevelDebug)
	requireMsgAttr(t, h, "do: redis error", "verb", evalShaVerb)

	fake.setRejectMSetEX()
	if err := sr.MSetEX(context.Background(), []string{"a"}, [][]byte{[]byte("1")}, 60); err != nil {
		t.Fatalf("MSetEX: %v", err)
	}
	requireMsg(t, h, "simpleredis_capability", slog.LevelDebug)
}

func TestLogCanceled(t *testing.T) {
	h := &recHandler{}
	_, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	sr := newTestRedis(t, Config{Host: addr, Logger: recLogger(h), MaxRetries: -1, CommandTimeout: time.Second})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := sr.Get(ctx, "hit")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Get = %v, want canceled", err)
	}
	requireMsg(t, h, "exec: canceled", slog.LevelDebug)
}

func TestLogCanceledWaitForTurn(t *testing.T) {
	h := &recHandler{}
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	hold := make(chan struct{})
	fake.mu.Lock()
	fake.holdCh = hold
	fake.mu.Unlock()
	t.Cleanup(func() { close(hold) })
	sr := newTestRedis(t, Config{Host: addr, PoolSize: 1, PoolTimeout: time.Second, CommandTimeout: 5 * time.Second, MaxRetries: -1, Logger: recLogger(h)})
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
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("waiter Get = %v, want canceled", err)
	}
	requireMsg(t, h, "exec: canceled", slog.LevelDebug)
}

func TestLogTimeout(t *testing.T) {
	h := &recHandler{}
	addr := startStallRedis(t)
	sr := newTestRedis(t, Config{Host: addr, Logger: recLogger(h), MaxRetries: -1, CommandTimeout: 30 * time.Millisecond})
	_, err := sr.Get(context.Background(), "k")
	if err == nil || err.Error() != RedisTimeout {
		t.Fatalf("Get = %v, want %s", err, RedisTimeout)
	}
	// One owner: libraryTimeout in exec decides the command budget expired, so exec is the only site that says so.
	requireMsg(t, h, "exec: "+RedisTimeout, slog.LevelDebug)
	for _, r := range h.records() {
		if r.Message == "do: "+RedisTimeout {
			t.Fatalf("do also classified the timeout:\n%s", h.dump())
		}
	}
}

func TestLogRetry(t *testing.T) {
	h := &recHandler{}
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	sr := newTestRedis(t, Config{Host: addr, Logger: recLogger(h), MinRetryBackoff: -1})
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
	sr := newTestRedis(t, Config{Host: addr, Logger: recLogger(h)})
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

// TestLogDialReasonSkipIdleAfterPeerDrop proves simpleredis_dial says why it dialled: idle_miss while the
// unused list has nothing to reuse, skip_idle once a command found a dropped socket in that list and bypassed it.
func TestLogDialReasonSkipIdleAfterPeerDrop(t *testing.T) {
	const poolSize = 2
	h := &recHandler{}
	fake, addr := startPeerDropAllFake(t, map[string]string{"hit": "t"})
	sr := newTestRedis(t, Config{Host: addr, PoolSize: poolSize, MaxIdleConns: poolSize,
		MinRetryBackoff: -1, MaxRetryBackoff: -1, Logger: recLogger(h)})
	t.Cleanup(sr.Close)

	warmPeerDropAllIdle(t, fake, sr, poolSize)
	requireMsgAttr(t, h, "simpleredis_dial", "reason", "idle_miss")

	fake.dropEveryAcceptedSocket()
	time.Sleep(50 * time.Millisecond)
	if _, err := sr.Get(context.Background(), "hit"); err != nil {
		t.Fatalf("Get after peer drop: %v", err)
	}
	requireMsgAttr(t, h, "simpleredis_dial", "reason", "skip_idle")
}

func TestNilLoggerDoesNotPanic(t *testing.T) {
	_, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	sr := newTestRedis(t, Config{Host: addr})
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
	sr := newTestRedis(t, Config{Host: addr, Pass: pass, Logger: recLogger(h), MaxRetries: -1})
	_, _ = sr.Get(context.Background(), key)

	okFake, okAddr := startFakeRedis(t, map[string]string{})
	_ = okFake
	ok := newTestRedis(t, Config{Host: okAddr, Pass: pass, Logger: recLogger(h)})
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
