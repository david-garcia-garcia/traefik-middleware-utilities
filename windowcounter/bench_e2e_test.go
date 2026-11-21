package windowcounter

import (
	"context"
	"strconv"
	"testing"
	"time"
)

// BenchmarkLiveTake measures compiled Take against live Redis and Dragonfly (WINDOWCOUNTER_LIVE_*) in exact mode and at sync_rate 20ms:
// one hot key and 10k rotating keys. It skips under -short or when both addresses are unset. Keys carry a per-run prefix.
func BenchmarkLiveTake(b *testing.B) {
	ctx := context.Background()
	for _, engine := range liveEngineAddrs(b, "WINDOWCOUNTER_LIVE_REDIS", "WINDOWCOUNTER_LIVE_DRAGONFLY") {
		engine := engine
		b.Run(engine.name, func(b *testing.B) {
			client := waitLiveClient(b, engine.addr)
			runPrefix := "windowcounter-bench:" + strconv.FormatInt(time.Now().UnixNano(), 36) + ":"
			for _, mode := range benchTakeModes {
				mode := mode
				b.Run(mode.name+"/hot-key", func(b *testing.B) {
					limiter, err := New(client, mode.syncRate)
					if err != nil {
						b.Fatal(err)
					}
					b.Cleanup(limiter.Close)
					hotKey := runPrefix + mode.name + ":" + benchHotKey
					takeForBench(b, limiter, hotKey)
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						if _, _, err := limiter.Take(ctx, hotKey, benchLimit, benchWindow); err != nil {
							b.Fatal(err)
						}
					}
				})
				b.Run(mode.name+"/rotating-10k", func(b *testing.B) {
					limiter, err := New(client, mode.syncRate)
					if err != nil {
						b.Fatal(err)
					}
					b.Cleanup(limiter.Close)
					keys := benchKeyNames(runPrefix+mode.name+":ip:", benchRotatingKeys)
					for _, key := range keys {
						takeForBench(b, limiter, key)
					}
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						if _, _, err := limiter.Take(ctx, keys[i%len(keys)], benchLimit, benchWindow); err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		})
	}
}
