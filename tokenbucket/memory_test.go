package tokenbucket

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"
)

// storeMemEntryForTest stores a hand-built entry under key the way Allow stores a new source, so tests and benchmarks can prefill a Memory without calling Allow.
func storeMemEntryForTest(m *Memory, key string, entry *memEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if previous := m.buckets[key]; previous != nil {
		m.removeLocked(previous)
	}
	entry.key = key
	m.buckets[key] = entry
	m.pushBackLocked(entry)
}

// capTestTokens is the token count prefilled sources hold, so a changed source is visible.
const capTestTokens = 1.5

// newCapTestMemory builds a burst-2 limiter on a frozen clock at now.
func newCapTestMemory(t *testing.T, now *time.Time) *Memory {
	t.Helper()
	limiter, err := NewMemory(1, 2, 0, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	limiter.SetNowForTest(func() time.Time { return *now })
	return limiter
}

// prefillCapTestSources stores count sources named prefix+i that expire at expireAt.
func prefillCapTestSources(limiter *Memory, prefix string, count int, last, expireAt time.Time) {
	for i := 0; i < count; i++ {
		storeMemEntryForTest(limiter, prefix+strconv.Itoa(i), &memEntry{
			tokens:   capTestTokens,
			last:     last.UnixMicro(),
			expireAt: expireAt,
		})
	}
}

// allowNewFullBucket Allows key and fails unless it is admitted as a full bucket (burst - 1 tokens left).
func allowNewFullBucket(t *testing.T, limiter *Memory, key string) {
	t.Helper()
	status, err := limiter.Allow(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Allowed {
		t.Fatalf("Allow(%q) denied, want a full bucket", key)
	}
	if tokens := limiter.buckets[key].tokens; tokens != 1 {
		t.Fatalf("Allow(%q) left %v tokens, want burst-1 = 1", key, tokens)
	}
}

// TestMemory_FullStoreDropsExpiredBeforeLive: at the cap, a new source removes every expired source and keeps every live one unchanged.
func TestMemory_FullStoreDropsExpiredBeforeLive(t *testing.T) {
	start := time.Unix(1_700_000_000, 0)
	now := start
	limiter := newCapTestMemory(t, &now)
	half := maxMemorySources / 2
	// Expired sources expire at start+ttl; live ones a ttl later. Together they fill the store.
	prefillCapTestSources(limiter, "expired:", half, start, start.Add(testTTL))
	prefillCapTestSources(limiter, "live:", maxMemorySources-half, start, start.Add(2*testTTL))
	now = start.Add(testTTL)

	allowNewFullBucket(t, limiter, "new")

	if got, want := len(limiter.buckets), maxMemorySources-half+1; got != want {
		t.Fatalf("stored sources %d, want %d (live plus new)", got, want)
	}
	for key, entry := range limiter.buckets {
		if strings.HasPrefix(key, "expired:") {
			t.Fatalf("expired source %q still stored", key)
		}
		if strings.HasPrefix(key, "live:") && entry.tokens != capTestTokens {
			t.Fatalf("live source %q tokens %v, want %v", key, entry.tokens, capTestTokens)
		}
	}
}

// TestMemory_FullStoreOfLiveStaysAtCap: at the cap with nothing expired, a new source removes one live source, and that source comes back as a full bucket.
func TestMemory_FullStoreOfLiveStaysAtCap(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	limiter := newCapTestMemory(t, &now)
	prefillCapTestSources(limiter, "live:", maxMemorySources, now, now.Add(testTTL))

	allowNewFullBucket(t, limiter, "new")

	if got := len(limiter.buckets); got != maxMemorySources {
		t.Fatalf("stored sources %d, want the cap %d", got, maxMemorySources)
	}
	// Exactly one live source made room; it is not specified which.
	removed := ""
	for i := 0; i < maxMemorySources; i++ {
		key := "live:" + strconv.Itoa(i)
		if limiter.buckets[key] == nil {
			if removed != "" {
				t.Fatalf("sources %q and %q both removed, want one", removed, key)
			}
			removed = key
		}
	}
	if removed == "" {
		t.Fatal("no live source removed at the cap")
	}

	allowNewFullBucket(t, limiter, removed)
	if last := limiter.buckets[removed].last; last != now.UnixMicro() {
		t.Fatalf("removed source came back with last %d, want now %d", last, now.UnixMicro())
	}
	if got := len(limiter.buckets); got != maxMemorySources {
		t.Fatalf("stored sources %d after re-admit, want the cap %d", got, maxMemorySources)
	}
}

// TestMemory_ReusedSourceMovesBehindExpiring: a stored source that is not the list tail moves behind the sources that expire
// before it, so at the cap a new source removes those expired sources and keeps the reused live one unchanged.
func TestMemory_ReusedSourceMovesBehindExpiring(t *testing.T) {
	start := time.Unix(1_700_000_000, 0)
	now := start
	limiter := newCapTestMemory(t, &now)
	allowNewFullBucket(t, limiter, "hot")
	// The other sources expire with hot's first Allow and are stored behind it, so hot is the list head.
	prefillCapTestSources(limiter, "other:", maxMemorySources-1, start, start.Add(testTTL))
	now = start.Add(testTTL / 2)
	if _, err := limiter.Allow(context.Background(), "hot"); err != nil {
		t.Fatal(err)
	}
	hotEntry := limiter.buckets["hot"]
	hotTokens := hotEntry.tokens
	now = start.Add(testTTL)

	allowNewFullBucket(t, limiter, "new")

	if limiter.buckets["hot"] != hotEntry {
		t.Fatal("reused live source hot was removed while expired sources were stored behind it")
	}
	if hotEntry.tokens != hotTokens {
		t.Fatalf("hot tokens %v, want unchanged %v", hotEntry.tokens, hotTokens)
	}
	if got := len(limiter.buckets); got != 2 {
		t.Fatalf("stored sources %d, want 2 (hot and new)", got)
	}
}

// expiryListKeysForTest walks the expiry list head to tail, checks every back link and that the list holds exactly the
// stored sources, and returns the keys in list order.
func expiryListKeysForTest(t *testing.T, limiter *Memory) []string {
	t.Helper()
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	var keys []string
	var previous *memEntry
	for entry := limiter.head; entry != nil; entry = entry.next {
		if entry.prev != previous {
			t.Fatalf("source %q back link does not point at the source before it", entry.key)
		}
		if limiter.buckets[entry.key] != entry {
			t.Fatalf("listed source %q is not the stored entry", entry.key)
		}
		keys = append(keys, entry.key)
		previous = entry
	}
	if limiter.tail != previous {
		t.Fatal("tail is not the last listed source")
	}
	if len(keys) != len(limiter.buckets) {
		t.Fatalf("list holds %d sources, map holds %d", len(keys), len(limiter.buckets))
	}
	return keys
}

// TestMemory_AllowRelinksExpiryList: Allow on the head, a middle source, and the tail moves that source to the tail and keeps
// every link, so no stored source drops out of the list.
func TestMemory_AllowRelinksExpiryList(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	limiter := newCapTestMemory(t, &now)
	for _, key := range []string{"a", "b", "c", "d"} {
		allowNewFullBucket(t, limiter, key)
	}
	for _, step := range []struct {
		key  string
		want string
	}{
		{key: "b", want: "a,c,d,b"}, // middle
		{key: "a", want: "c,d,b,a"}, // head
		{key: "a", want: "c,d,b,a"}, // tail stays
		{key: "d", want: "c,b,a,d"}, // middle next to the head
	} {
		if _, err := limiter.Allow(context.Background(), step.key); err != nil {
			t.Fatal(err)
		}
		if got := strings.Join(expiryListKeysForTest(t, limiter), ","); got != step.want {
			t.Fatalf("after Allow(%q) list %s, want %s", step.key, got, step.want)
		}
	}
}
