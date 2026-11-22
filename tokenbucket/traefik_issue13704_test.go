package tokenbucket

import (
	"context"
	"testing"
	"time"
)

// TestTraefik13704_MemoryReclaimsExpiredBuckets: https://github.com/traefik/traefik/issues/13704.
// Traefik's ttlmap keeps expired buckets until the map reaches 65536 entries, so memory tracks every
// source ever seen. Here ten sources go idle past the TTL and one new source arrives: only the new one
// may remain.
func TestTraefik13704_MemoryReclaimsExpiredBuckets(t *testing.T) {
	const ttl = 2 * time.Second
	limiter, err := NewMemory(15, 10, 0, ttl)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	limiter.SetNowForTest(func() time.Time { return now })
	for _, source := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"} {
		status, allowErr := limiter.Allow(context.Background(), source)
		if allowErr != nil {
			t.Fatal(allowErr)
		}
		if !status.Allowed {
			t.Fatalf("Allow(%q) denied, want admit of a new source", source)
		}
	}
	now = now.Add(ttl + 9*time.Second)
	status, err := limiter.Allow(context.Background(), "fresh")
	if err != nil {
		t.Fatal(err)
	}
	if !status.Allowed {
		t.Fatal(`Allow("fresh") denied, want admit of a new source`)
	}
	limiter.mu.Lock()
	held := len(limiter.buckets)
	_, freshHeld := limiter.buckets["fresh"]
	limiter.mu.Unlock()
	if held != 1 {
		t.Fatalf("map holds %d buckets after ten expired and one new source, want 1", held)
	}
	if !freshHeld {
		t.Fatal("stored source is not the new key")
	}
}
