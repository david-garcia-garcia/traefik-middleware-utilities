package tokenbucket

import (
	"fmt"
	"testing"
)

// TestYaegiLive_RedisAndDragonfly runs the same live limiter scenarios interpreted against each engine.
func TestYaegiLive_RedisAndDragonfly(t *testing.T) {
	runForEachLiveEngine(t, "TOKENBUCKET_LIVE_REDIS", "TOKENBUCKET_LIVE_DRAGONFLY", func(t *testing.T, addr string) {
		goPath := t.TempDir()
		writeGopathTokenbucket(t, goPath)
		writeGopathFile(t, goPath, "allowprobe", "roundtrip.go", allowprobeSrc)
		t.Run("burstAfterIdle", func(t *testing.T) {
			got := evalAllowprobe(t, goPath, fmt.Sprintf(`allowprobe.BurstAfterIdle(%q, %q)`, addr, t.Name()))
			if got != "ok" {
				t.Fatalf("yaegi live burst: %q, want ok", got)
			}
		})
		t.Run("twoInstancesShare", func(t *testing.T) {
			got := evalAllowprobe(t, goPath, fmt.Sprintf(`allowprobe.TwoShare(%q, %q)`, addr, t.Name()))
			if got != "ok" {
				t.Fatalf("yaegi live share: %q, want ok", got)
			}
		})
		t.Run("memoryAgrees", func(t *testing.T) {
			got := evalAllowprobe(t, goPath, fmt.Sprintf(`allowprobe.MemoryAgrees(%q, %q)`, addr, t.Name()))
			if got != "ok" {
				t.Fatalf("yaegi live agree: %q, want ok", got)
			}
		})
		t.Run("refundPastMaxDelay", func(t *testing.T) {
			got := evalAllowprobe(t, goPath, fmt.Sprintf(`allowprobe.RefundAfterBurst(%q, %q)`, addr, t.Name()))
			if got != "ok" {
				t.Fatalf("yaegi live refund: %q, want ok", got)
			}
		})
	})
}
