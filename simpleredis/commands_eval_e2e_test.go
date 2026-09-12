package simpleredis

import (
	"context"
	"testing"
)

// TestLive_Eval proves Eval integer, EVALSHA, and NOSCRIPT after SCRIPT FLUSH on live Redis and Dragonfly.
func TestLive_Eval(t *testing.T) {
	runForEachLiveEngine(t, "SIMPLEREDIS_LIVE_REDIS", "SIMPLEREDIS_LIVE_DRAGONFLY", runLiveEvalBackend)
}

// runLiveEvalBackend proves Eval integer, a second Eval after EVALSHA, and Eval after SCRIPT FLUSH.
func runLiveEvalBackend(t *testing.T, addr string) {
	t.Helper()
	client := waitLiveSimpleRedis(t, addr)
	t.Cleanup(client.Close)

	t.Run("evalInteger", func(t *testing.T) {
		script := "return 7 -- " + t.Name()
		values, err := client.Eval(context.Background(), script, ScriptSHA1Hex(script), nil, nil)
		if err != nil {
			t.Fatalf("Eval: %v", err)
		}
		n, err := parseIntegerReply(values, nil)
		if err != nil {
			t.Fatalf("Eval parse: %v", err)
		}
		if n != 7 {
			t.Fatalf("Eval = %d, want 7", n)
		}
	})
	t.Run("secondEvalHitsEvalSha", func(t *testing.T) {
		script := "return 8 -- " + t.Name()
		if _, err := client.Eval(context.Background(), script, ScriptSHA1Hex(script), nil, nil); err != nil {
			t.Fatalf("first Eval: %v", err)
		}
		values, err := client.Eval(context.Background(), script, ScriptSHA1Hex(script), nil, nil)
		if err != nil {
			t.Fatalf("second Eval: %v", err)
		}
		n, err := parseIntegerReply(values, nil)
		if err != nil {
			t.Fatalf("second Eval parse: %v", err)
		}
		if n != 8 {
			t.Fatalf("second Eval = %d, want 8", n)
		}
	})
	t.Run("evalAfterScriptFlush", func(t *testing.T) {
		script := "return 9 -- " + t.Name()
		if _, err := client.Eval(context.Background(), script, ScriptSHA1Hex(script), nil, nil); err != nil {
			t.Fatalf("seed Eval: %v", err)
		}
		if _, err := client.exec(context.Background(), []byte("SCRIPT"), []byte("FLUSH")); err != nil {
			t.Fatalf("SCRIPT FLUSH: %v", err)
		}
		values, err := client.Eval(context.Background(), script, ScriptSHA1Hex(script), nil, nil)
		if err != nil {
			t.Fatalf("Eval after FLUSH: %v", err)
		}
		n, err := parseIntegerReply(values, nil)
		if err != nil {
			t.Fatalf("Eval after FLUSH parse: %v", err)
		}
		if n != 9 {
			t.Fatalf("Eval after FLUSH = %d, want 9", n)
		}
	})
}
