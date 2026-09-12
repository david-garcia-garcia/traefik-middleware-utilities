package windowcounter

import (
	"fmt"
	"testing"
)

// TestYaegiLive_RedisAndDragonfly runs the same live limiter scenarios interpreted against each engine.
func TestYaegiLive_RedisAndDragonfly(t *testing.T) {
	runForEachLiveEngine(t, "WINDOWCOUNTER_LIVE_REDIS", "WINDOWCOUNTER_LIVE_DRAGONFLY", func(t *testing.T, addr string) {
		goPath := t.TempDir()
		writeGopathWindowcounter(t, goPath)
		writeGopathFile(t, goPath, "takeprobe", "roundtrip.go", takeprobeSrc)
		t.Run("exactNThenDeny", func(t *testing.T) {
			got := evalTakeprobe(t, goPath, fmt.Sprintf(`takeprobe.UntilDeny(%q, %q, 3)`, addr, t.Name()))
			if got != "ok" {
				t.Fatalf("yaegi live take: %q, want ok", got)
			}
		})
		t.Run("bufferedTwoClients", func(t *testing.T) {
			got := evalTakeprobe(t, goPath, fmt.Sprintf(`takeprobe.BufferedShare(%q, %q)`, addr, t.Name()))
			if got != "ok" {
				t.Fatalf("yaegi live buffered: %q, want ok", got)
			}
		})
		t.Run("slidingBoundary", func(t *testing.T) {
			got := evalTakeprobe(t, goPath, fmt.Sprintf(`takeprobe.SlidingBoundary(%q, %q)`, addr, t.Name()))
			if got != "ok" {
				t.Fatalf("yaegi live sliding: %q, want ok", got)
			}
		})
		t.Run("peekThenTake", func(t *testing.T) {
			got := evalTakeprobe(t, goPath, fmt.Sprintf(`takeprobe.PeekThenTake(%q, %q)`, addr, t.Name()))
			if got != "ok" {
				t.Fatalf("yaegi live peek: %q, want ok", got)
			}
		})
		t.Run("peekDeniedThenSlides", func(t *testing.T) {
			got := evalTakeprobe(t, goPath, fmt.Sprintf(`takeprobe.PeekDeniedThenSlides(%q, %q)`, addr, t.Name()))
			if got != "ok" {
				t.Fatalf("yaegi live peek-slide: %q, want ok", got)
			}
		})
		t.Run("bufferedPeek", func(t *testing.T) {
			got := evalTakeprobe(t, goPath, fmt.Sprintf(`takeprobe.BufferedPeek(%q, %q)`, addr, t.Name()))
			if got != "ok" {
				t.Fatalf("yaegi live buffered peek: %q, want ok", got)
			}
		})
		t.Run("expireOnFirstHit", func(t *testing.T) {
			got := evalTakeprobe(t, goPath, fmt.Sprintf(`takeprobe.ExpireOnFirstHit(%q, %q)`, addr, t.Name()))
			if got != "ok" {
				t.Fatalf("yaegi live expire: %q, want ok", got)
			}
		})
	})
}
