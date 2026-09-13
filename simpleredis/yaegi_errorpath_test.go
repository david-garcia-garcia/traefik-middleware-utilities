package simpleredis

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

const errorpathprobeSrc = `package errorpathprobe

import (
	"context"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// DialRetry Gets a refusing host so the retry loop and backoff jitter run.
func DialRetry(host string) string {
	client := simpleredis.New(simpleredis.Config{Host: host, MaxRetries: 1, MinRetryBackoff: time.Millisecond, MaxRetryBackoff: 4 * time.Millisecond})
	_, err := client.Get(context.Background(), "k")
	if err == nil {
		return "ok-unexpected"
	}
	if err.Error() != simpleredis.RedisUnreachable {
		return err.Error()
	}
	return "ok"
}

// StallTimeout Gets a peer that never replies.
func StallTimeout(host string) string {
	client := simpleredis.New(simpleredis.Config{Host: host, MaxRetries: -1, IOTimeout: 40 * time.Millisecond, DialTimeout: 40 * time.Millisecond})
	_, err := client.Get(context.Background(), "k")
	if err == nil {
		return "ok-unexpected"
	}
	if err.Error() != simpleredis.RedisTimeout {
		return err.Error()
	}
	return "ok"
}

// CancelMidCommand Gets with a short caller deadline.
func CancelMidCommand(host string) string {
	client := simpleredis.New(simpleredis.Config{Host: host, PoolSize: 1, IOTimeout: 5 * time.Second, MaxRetries: -1})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err := client.Get(ctx, "hit")
	if err == nil {
		return "ok-unexpected"
	}
	if err.Error() != context.DeadlineExceeded.Error() {
		return err.Error()
	}
	return "ok"
}

// PoolWait Gets when this client's live cap is already held by another Get.
func PoolWait(host string) string {
	client := simpleredis.New(simpleredis.Config{Host: host, PoolSize: 1, PoolTimeout: 40 * time.Millisecond, IOTimeout: time.Second, MaxRetries: -1})
	started := make(chan struct{})
	go func() {
		close(started)
		_, _ = client.Get(context.Background(), "hit")
	}()
	<-started
	time.Sleep(20 * time.Millisecond)
	_, err := client.Get(context.Background(), "hit")
	if err == nil {
		return "ok-unexpected"
	}
	if err.Error() != simpleredis.RedisUnreachable {
		return "text:" + err.Error()
	}
	if !simpleredis.IsPoolWait(err) {
		return "IsPoolWait"
	}
	return "ok"
}

// TruncatedBulk Gets a short bulk then close.
func TruncatedBulk(host string) string {
	client := simpleredis.New(simpleredis.Config{Host: host, MaxRetries: -1})
	_, err := client.Get(context.Background(), "k")
	if err == nil {
		return "ok-unexpected"
	}
	if err.Error() == simpleredis.RedisIssue {
		return "issue"
	}
	if err.Error() != simpleredis.RedisUnreachable {
		return err.Error()
	}
	return "ok"
}
`

func writeGopathErrorpath(t *testing.T, goPath string) {
	t.Helper()
	writeGopathFile(t, goPath, "errorpathprobe", "errorpath.go", errorpathprobeSrc)
}

func evalErrorpath(t *testing.T, goPath, expr string) string {
	t.Helper()
	interpreter := interp.New(interp.Options{GoPath: goPath})
	if err := interpreter.Use(stdlib.Symbols); err != nil {
		t.Fatalf("use stdlib: %v", err)
	}
	if _, err := interpreter.Eval(`import "errorpathprobe"`); err != nil {
		t.Fatalf("import errorpathprobe: %v", err)
	}
	evaluated, err := interpreter.Eval(expr)
	if err != nil {
		t.Fatalf("eval %s: %v", expr, err)
	}
	return evaluated.Interface().(string)
}

func TestYaegiErrorpath_DialRetry(t *testing.T) {
	goPath := t.TempDir()
	writeGopathSimpleredis(t, goPath)
	writeGopathErrorpath(t, goPath)
	got := evalErrorpath(t, goPath, `errorpathprobe.DialRetry("127.0.0.1:1")`)
	if got != "ok" {
		t.Fatalf("yaegi dial retry: %q, want ok", got)
	}
}

func TestYaegiErrorpath_StallTimeout(t *testing.T) {
	addr := startStallRedis(t)
	goPath := t.TempDir()
	writeGopathSimpleredis(t, goPath)
	writeGopathErrorpath(t, goPath)
	got := evalErrorpath(t, goPath, fmt.Sprintf(`errorpathprobe.StallTimeout(%q)`, addr))
	if got != "ok" {
		t.Fatalf("yaegi stall timeout: %q, want ok", got)
	}
}

func TestYaegiErrorpath_CancelMidCommand(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	fake.holdGetsForTest(t)
	goPath := t.TempDir()
	writeGopathSimpleredis(t, goPath)
	writeGopathErrorpath(t, goPath)

	done := make(chan string, 1)
	go func() {
		done <- evalErrorpath(t, goPath, fmt.Sprintf(`errorpathprobe.CancelMidCommand(%q)`, addr))
	}()
	fake.waitHeldGets(t, 1)
	got := <-done
	if got != "ok" {
		t.Fatalf("yaegi cancel-mid-command: %q, want ok", got)
	}
}

func TestYaegiErrorpath_PoolWait(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	fake.mu.Lock()
	fake.getDelay = 300 * time.Millisecond
	fake.mu.Unlock()
	goPath := t.TempDir()
	writeGopathSimpleredis(t, goPath)
	writeGopathErrorpath(t, goPath)
	got := evalErrorpath(t, goPath, fmt.Sprintf(`errorpathprobe.PoolWait(%q)`, addr))
	if got != "ok" {
		t.Fatalf("yaegi pool wait: %q, want ok", got)
	}
}

func TestYaegiErrorpath_TruncatedBulk(t *testing.T) {
	truncated := append([]byte("$100\r\n"), bytes.Repeat([]byte("x"), 40)...)
	addr := startRawReplyRedis(t, []rawReply{{payload: truncated, closeAfter: true}})
	goPath := t.TempDir()
	writeGopathSimpleredis(t, goPath)
	writeGopathErrorpath(t, goPath)
	got := evalErrorpath(t, goPath, fmt.Sprintf(`errorpathprobe.TruncatedBulk(%q)`, addr))
	if got != "ok" {
		t.Fatalf("yaegi truncated bulk: %q, want ok", got)
	}
}
