package simpleredis

import (
	"fmt"
	"testing"
)

// TestYaegiLive_RedisAndDragonfly runs interpreted New/Get/Set/Del/Incr/Eval/MSetEX against each live engine.
func TestYaegiLive_RedisAndDragonfly(t *testing.T) {
	runForEachLiveEngine(t, "SIMPLEREDIS_LIVE_REDIS", "SIMPLEREDIS_LIVE_DRAGONFLY", func(t *testing.T, addr string) {
		client := waitLiveSimpleRedis(t, addr)
		t.Cleanup(client.Close)
		goPath := t.TempDir()
		writeGopathSimpleredis(t, goPath)
		writeGopathClientprobe(t, goPath)
		got := evalClientprobe(t, goPath, fmt.Sprintf(`clientprobe.LiveVerbs(%q, %q)`, addr, t.Name()))
		if got != "ok" {
			t.Fatalf("yaegi live verbs: %q, want ok", got)
		}
	})
}
