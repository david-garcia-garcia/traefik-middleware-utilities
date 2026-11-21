package tokenbucket

import (
	"context"
	"strconv"
	"testing"
	"time"
)

// BenchmarkLiveRedisAllow measures compiled Redis Allow against live Redis and Dragonfly (TOKENBUCKET_LIVE_*): one hot key and 10k rotating keys.
// It skips under -short or when both addresses are unset. Keys carry a per-run prefix so reruns start from empty buckets.
func BenchmarkLiveRedisAllow(b *testing.B) {
	ctx := context.Background()
	for _, engine := range liveEngineAddrs(b, "TOKENBUCKET_LIVE_REDIS", "TOKENBUCKET_LIVE_DRAGONFLY") {
		engine := engine
		b.Run(engine.name, func(b *testing.B) {
			client := waitLiveClient(b, engine.addr)
			limiter, err := NewRedis(client, benchRate, benchBurst, benchMaxDelay, benchTTL)
			if err != nil {
				b.Fatal(err)
			}
			runPrefix := "tokenbucket-bench:" + strconv.FormatInt(time.Now().UnixNano(), 36) + ":"
			b.Run("hot-key", func(b *testing.B) {
				hotKey := runPrefix + benchHotKey
				if _, _, err := limiter.Allow(ctx, hotKey); err != nil {
					b.Fatal(err)
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if _, _, err := limiter.Allow(ctx, hotKey); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("rotating-10k", func(b *testing.B) {
				keys := benchKeyNames(runPrefix+"ip:", benchRotatingKeys)
				for _, key := range keys {
					if _, _, err := limiter.Allow(ctx, key); err != nil {
						b.Fatal(err)
					}
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if _, _, err := limiter.Allow(ctx, keys[i%len(keys)]); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}
