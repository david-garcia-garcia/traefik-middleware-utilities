package tokenbucket

import (
	"testing"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// TestYaegi_MemoryCapAndExpiry proves the in-memory expiry list runs interpreted: expired sources are dropped from the
// head, a full store of live sources evicts one, and a stored source keeps its tokens. Traefik is not started.
func TestYaegi_MemoryCapAndExpiry(t *testing.T) {
	goPath := t.TempDir()
	writeGopathTokenbucket(t, goPath)
	writeGopathFile(t, goPath, "memoryprobe", "cap.go", memoryprobeSrc)

	interpreter := interp.New(interp.Options{GoPath: goPath})
	if err := interpreter.Use(stdlib.Symbols); err != nil {
		t.Fatalf("use stdlib: %v", err)
	}
	if _, err := interpreter.Eval(`import "memoryprobe"`); err != nil {
		t.Fatalf("import memoryprobe: %v", err)
	}
	evaluated, err := interpreter.Eval(`memoryprobe.CapAndExpiry()`)
	if err != nil {
		t.Fatalf("eval CapAndExpiry: %v", err)
	}
	if got := evaluated.Interface().(string); got != "ok" {
		t.Fatalf("yaegi memory cap: %q, want ok", got)
	}
}

// memoryprobeSrc fills the 65536-source store twice on a frozen clock: once to expire it, once to hit the cap with live sources.
const memoryprobeSrc = `package memoryprobe

import (
	"context"
	"strconv"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/tokenbucket"
)

const maxSources = 65536

// CapAndExpiry returns "ok" when every new source is admitted full and a stored source that spent its token still denies.
func CapAndExpiry() string {
	limiter, err := tokenbucket.NewMemory(1e-6, 1, 0, time.Second)
	if err != nil {
		return "new:" + err.Error()
	}
	now := time.Unix(1700000000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	ctx := context.Background()
	// Fill the store, then let every source expire: new sources drop them from the head.
	if msg := allowAll(ctx, limiter, "old:", maxSources); msg != "" {
		return msg
	}
	now = now.Add(time.Second)
	if msg := allowAll(ctx, limiter, "live:", maxSources); msg != "" {
		return msg
	}
	// Store is full of live sources: one more evicts one, and is admitted as a full bucket.
	status, err := limiter.Allow(ctx, "extra")
	if err != nil {
		return "extra:" + err.Error()
	}
	if !status.Allowed {
		return "extra-denied"
	}
	// extra is still stored with its only token spent, so it denies.
	status, err = limiter.Allow(ctx, "extra")
	if err != nil {
		return "extra-again:" + err.Error()
	}
	if status.Allowed {
		return "extra-refilled"
	}
	return "ok"
}

// allowAll Allows count new keys prefix0..prefix(count-1) and wants every one admitted.
func allowAll(ctx context.Context, limiter *tokenbucket.Memory, prefix string, count int) string {
	for i := 0; i < count; i++ {
		status, err := limiter.Allow(ctx, prefix+strconv.Itoa(i))
		if err != nil {
			return prefix + "allow:" + err.Error()
		}
		if !status.Allowed {
			return prefix + strconv.Itoa(i) + "-denied"
		}
	}
	return ""
}
`
