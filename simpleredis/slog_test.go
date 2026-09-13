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

func (h *recHandler) Enabled(context.Context, slog.Level) bool { return true }

// Handle stores a clone of the record.
func (h *recHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	h.recs = append(h.recs, r.Clone())
	h.mu.Unlock()
	return nil
}

func (h *recHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *recHandler) WithGroup(string) slog.Handler { return h }

func (h *recHandler) records() []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]slog.Record, len(h.recs))
	copy(out, h.recs)
	return out
}

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

func recLogger(h slog.Handler) *slog.Logger { return slog.New(h) }

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

func attrString(r slog.Record, key string) string {
	var got string
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == key {
			got = a.Value.String()
		}
		return true
	})
	return got
}

func TestLogOpenAndClose(t *testing.T) {
	h := &recHandler{}
	sr := New(Config{Host: "127.0.0.1:1", Logger: recLogger(h)})
	open := requireMsg(t, h, MsgOpen, slog.LevelDebug)
	if attrString(open, "host") != "127.0.0.1:1" {
		t.Fatalf("open host = %q", attrString(open, "host"))
	}
	if strings.Contains(h.dump(), "Pass") || strings.Contains(strings.ToLower(h.dump()), " pass=") {
		t.Fatalf("open leaked a pass attr:\n%s", h.dump())
	}
	sr.Close()
	requireMsg(t, h, MsgClose, slog.LevelDebug)
}

func TestLogNoAuth(t *testing.T) {
	h := &recHandler{}
	addr := startStaticRedis(t, "-WRONGPASS invalid password\r\n")
	sr := New(Config{Host: addr, Logger: recLogger(h), MaxRetries: -1})
	_, err := sr.Get(context.Background(), "a")
	if err == nil || err.Error() != RedisNoAuth {
		t.Fatalf("Get = %v, want %s", err, RedisNoAuth)
	}
	rec := requireMsg(t, h, MsgNoAuth, slog.LevelError)
	if attrString(rec, "error") != RedisNoAuth {
		t.Fatalf("noauth error attr = %q, want %s", attrString(rec, "error"), RedisNoAuth)
	}
}

func TestLogNotFromNew(t *testing.T) {
	h := &recHandler{}
	sr := &SimpleRedis{logger: recLogger(h), host: "x"}
	_, err := sr.Get(context.Background(), "k")
	if !IsUnreachable(err) {
		t.Fatalf("Get = %v, want unreachable", err)
	}
	requireMsg(t, h, MsgNotFromNew, slog.LevelError)
}

func TestLogOverFree(t *testing.T) {
	h := &recHandler{}
	sr := New(Config{Host: "127.0.0.1:1", PoolSize: 2, Logger: recLogger(h)})
	sr.freeInUseTurn()
	rec := requireMsg(t, h, MsgOverFree, slog.LevelError)
	if attrString(rec, "over_frees") != "1" {
		t.Fatalf("over_frees = %q", attrString(rec, "over_frees"))
	}
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
	requireMsg(t, h, MsgPoolExhausted, slog.LevelWarn)
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
	rec := requireMsg(t, h, MsgShortBulk, slog.LevelWarn)
	if attrString(rec, "announced") != "100" {
		t.Fatalf("announced = %q", attrString(rec, "announced"))
	}
}

func TestLogBadReply(t *testing.T) {
	h := &recHandler{}
	addr := startStaticRedis(t, "$abc\r\n")
	sr := New(Config{Host: addr, MaxRetries: -1, Logger: recLogger(h)})
	_, err := sr.Get(context.Background(), "k")
	if err == nil || err.Error() != RedisIssue {
		t.Fatalf("Get = %v, want %s", err, RedisIssue)
	}
	requireMsg(t, h, MsgBadReply, slog.LevelWarn)
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
	requireMsg(t, h, MsgHandshakeFailed, slog.LevelWarn)
}

func TestLogSocketPoisonedAndAuthLeftover(t *testing.T) {
	h := &recHandler{}
	_, addr := startStrayExtraReplyFake(t, 1)
	sr := New(Config{Host: addr, PoolSize: 1, MaxRetries: -1, Logger: recLogger(h)})
	if _, err := sr.Get(context.Background(), "k0"); err != nil {
		t.Fatalf("Get: %v", err)
	}
	requireMsg(t, h, MsgSocketPoisoned, slog.LevelWarn)

	h2 := &recHandler{}
	authFake, authAddr := startFakeRedis(t, map[string]string{"hit": "t"})
	authFake.setHandshakeReplies("+OK\r\n$5\r\nSTRAY\r\n", statusOKReply)
	auth := New(Config{Host: authAddr, Pass: "secret", Database: "2", MaxRetries: -1, Logger: recLogger(h2)})
	_, err := auth.Get(context.Background(), "hit")
	if err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("auth leftover Get = %v, want %s", err, RedisUnreachable)
	}
	requireMsg(t, h2, MsgAuthLeftover, slog.LevelWarn)
}

func TestLogDialRetryCapabilityNoScript(t *testing.T) {
	h := &recHandler{}
	fake, addr := startFakeRedis(t, map[string]string{})
	sr := New(Config{Host: addr, Logger: recLogger(h)})
	if err := sr.Set(context.Background(), "k", []byte("v"), 60); err != nil {
		t.Fatalf("Set: %v", err)
	}
	requireMsg(t, h, MsgDial, slog.LevelDebug)

	if _, err := sr.Eval(context.Background(), "return 1", ScriptSHA1Hex("return 1"), nil, nil); err != nil {
		t.Fatalf("Eval: %v", err)
	}
	requireMsg(t, h, MsgNoScript, slog.LevelDebug)

	fake.setRejectMSetEX()
	if err := sr.MSetEX(context.Background(), []string{"a"}, [][]byte{[]byte("1")}, 60); err != nil {
		t.Fatalf("MSetEX: %v", err)
	}
	requireMsg(t, h, MsgCapability, slog.LevelDebug)
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
	requireMsg(t, h, MsgCanceled, slog.LevelDebug)
}

func TestLogTimeout(t *testing.T) {
	h := &recHandler{}
	addr := startStallRedis(t)
	sr := New(Config{Host: addr, Logger: recLogger(h), MaxRetries: -1, IOTimeout: 30 * time.Millisecond})
	_, err := sr.Get(context.Background(), "k")
	if err == nil || err.Error() != RedisTimeout {
		t.Fatalf("Get = %v, want %s", err, RedisTimeout)
	}
	requireMsg(t, h, MsgTimeout, slog.LevelDebug)
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
	requireMsg(t, h, MsgRetry, slog.LevelDebug)
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
	rec := requireMsg(t, h, MsgIdleSwept, slog.LevelDebug)
	if attrString(rec, "swept") != "1" {
		t.Fatalf("swept = %q", attrString(rec, "swept"))
	}
	var staleDial slog.Record
	foundStale := false
	for _, record := range h.records() {
		if record.Message == MsgDial && attrString(record, "reason") == dialReasonStale {
			staleDial = record
			foundStale = true
			break
		}
	}
	if !foundStale {
		t.Fatalf("missing %s reason=%s\n%s", MsgDial, dialReasonStale, h.dump())
	}
	if staleDial.Level != slog.LevelDebug {
		t.Fatalf("stale dial level = %s", staleDial.Level)
	}
}

func TestLogSocketClosedIdleCap(t *testing.T) {
	h := &recHandler{}
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	fake.mu.Lock()
	fake.getDelay = 20 * time.Millisecond
	fake.mu.Unlock()
	sr := New(Config{Host: addr, PoolSize: 2, MaxIdleConns: 1, Logger: recLogger(h)})
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := sr.Get(context.Background(), "hit"); err != nil {
				t.Errorf("Get: %v", err)
			}
		}()
	}
	wg.Wait()
	rec := requireMsg(t, h, MsgSocketClosed, slog.LevelDebug)
	if attrString(rec, "reason") != closedReasonIdleCap {
		t.Fatalf("reason = %q, want %s", attrString(rec, "reason"), closedReasonIdleCap)
	}
}

func TestLogSocketClosedCancel(t *testing.T) {
	h := &recHandler{}
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	hold := make(chan struct{})
	fake.mu.Lock()
	fake.holdCh = hold
	fake.mu.Unlock()
	t.Cleanup(func() { close(hold) })
	sr := New(Config{Host: addr, PoolSize: 1, IOTimeout: 5 * time.Second, MaxRetries: -1, Logger: recLogger(h)})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := sr.Get(ctx, "hit")
		done <- err
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
			t.Fatal("Get never held")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	err := <-done
	if !errorsIsCanceled(err) {
		t.Fatalf("Get = %v, want canceled", err)
	}
	rec := requireMsg(t, h, MsgSocketClosed, slog.LevelDebug)
	if attrString(rec, "reason") != closedReasonCancel {
		t.Fatalf("reason = %q, want %s", attrString(rec, "reason"), closedReasonCancel)
	}
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

type levelGate struct {
	min slog.Level
}

func (h *levelGate) Enabled(_ context.Context, l slog.Level) bool { return l >= h.min }
func (h *levelGate) Handle(context.Context, slog.Record) error    { return nil }
func (h *levelGate) WithAttrs([]slog.Attr) slog.Handler           { return h }
func (h *levelGate) WithGroup(string) slog.Handler                { return h }

func TestAllocDebugGuardNilAndDisabled(t *testing.T) {
	skipAllocCeilingIfRace(t)
	nilClient := New(Config{Host: "127.0.0.1:1"})
	nilAllocs := testing.AllocsPerRun(1000, func() {
		nilClient.logDebugTimeout(context.Background(), errTimeout)
		nilClient.logDebugCanceled(context.Background())
		nilClient.logDebugDial(context.Background(), dialReasonIdleMiss)
		nilClient.logDebugRetry(context.Background(), 1, time.Millisecond, errUnreachable)
	})
	if nilAllocs != 0 {
		t.Fatalf("nil logger Debug allocs/run = %v, want 0", nilAllocs)
	}
	t.Logf("nil logger Debug allocs/run = %v", nilAllocs)

	quiet := New(Config{Host: "127.0.0.1:1", Logger: slog.New(&levelGate{min: slog.LevelError})})
	quietAllocs := testing.AllocsPerRun(1000, func() {
		quiet.logDebugTimeout(context.Background(), errTimeout)
		quiet.logDebugCanceled(context.Background())
		quiet.logDebugDial(context.Background(), dialReasonIdleMiss)
		quiet.logDebugRetry(context.Background(), 1, time.Millisecond, errUnreachable)
	})
	if quietAllocs != 0 {
		t.Fatalf("Debug-disabled logger allocs/run = %v, want 0", quietAllocs)
	}
	t.Logf("Debug-disabled logger allocs/run = %v", quietAllocs)
}
