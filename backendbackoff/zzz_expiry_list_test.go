package backendbackoff

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

// storeEntryForTestExpiryList stores a hand-built entry under key the way Allow stores a new key, so tests and
// benchmarks can prefill a Gate without calling Allow.
func storeEntryForTestExpiryList(g *Gate, key string, entry *memEntry) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if previous := g.keys[key]; previous != nil {
		g.removeLocked(previous)
	}
	entry.key = key
	g.bumpGen(entry)
	g.storeLocked(entry)
}

// prefillKeysExpiryList stores count CLOSED keys named prefix+i that expire one TTL after now.
func prefillKeysExpiryList(g *Gate, prefix string, count int, now time.Time) {
	expireAt := now.Add(g.cfg.TTL)
	for i := 0; i < count; i++ {
		storeEntryForTestExpiryList(g, prefix+strconv.Itoa(i), &memEntry{
			credit:      g.budget,
			state:       stateClosed,
			closedSince: now,
			expireAt:    expireAt,
		})
	}
}

// expiryListKeysExpiryList walks the expiry list head to tail, checks every back link and that the list holds exactly
// the stored keys, and returns the keys in list order.
func expiryListKeysExpiryList(t *testing.T, g *Gate) []string {
	t.Helper()
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.keys == nil {
		if g.head != nil || g.tail != nil {
			t.Fatal("closed gate still has list pointers")
		}
		return nil
	}
	var keys []string
	var previous *memEntry
	for entry := g.head; entry != nil; entry = entry.next {
		if entry.prev != previous {
			t.Fatalf("key %q back link does not point at the key before it", entry.key)
		}
		if g.keys[entry.key] != entry {
			t.Fatalf("listed key %q is not the stored entry", entry.key)
		}
		keys = append(keys, entry.key)
		previous = entry
	}
	if g.tail != previous {
		t.Fatal("tail is not the last listed key")
	}
	if len(keys) != len(g.keys) {
		t.Fatalf("list holds %d keys, map holds %d", len(keys), len(g.keys))
	}
	return keys
}

// TestCapExpiredDropFirstExpiryList: at the cap, one expired head drops first, live keys stay, and len(keys) stays 65536.
func TestCapExpiredDropFirstExpiryList(t *testing.T) {
	gate := newTestGate(t)
	now := time.Unix(1_700_000_000, 0)
	gate.SetNowForTest(func() time.Time { return now })
	storeEntryForTestExpiryList(gate, "expired:0", &memEntry{
		credit:      gate.budget,
		state:       stateClosed,
		closedSince: now,
		expireAt:    now,
	})
	prefillKeysExpiryList(gate, "live:", maxMemorySources-1, now)

	p, _, err := gate.Allow("new")
	if err != nil || !p.Allowed() {
		t.Fatalf("admit new: allowed=%v err=%v", p.Allowed(), err)
	}

	if got := len(gate.keys); got != maxMemorySources {
		t.Fatalf("stored keys %d, want the cap %d", got, maxMemorySources)
	}
	if gate.keys["expired:0"] != nil {
		t.Fatal("expired key still stored")
	}
	if gate.keys["new"] == nil {
		t.Fatal("new key was not admitted")
	}
	for i := 0; i < maxMemorySources-1; i++ {
		key := "live:" + strconv.Itoa(i)
		if gate.keys[key] == nil {
			t.Fatalf("live key %q was dropped", key)
		}
	}
}

// TestCapLiveEvictsOneExpiryList: at the cap with none expired, one key is dropped and the new key is admitted.
func TestCapLiveEvictsOneExpiryList(t *testing.T) {
	gate := newTestGate(t)
	now := time.Unix(1_700_000_000, 0)
	gate.SetNowForTest(func() time.Time { return now })
	prefillKeysExpiryList(gate, "live:", maxMemorySources, now)

	p, _, err := gate.Allow("new")
	if err != nil || !p.Allowed() {
		t.Fatalf("admit new: allowed=%v err=%v", p.Allowed(), err)
	}

	if got := len(gate.keys); got != maxMemorySources {
		t.Fatalf("stored keys %d, want the cap %d", got, maxMemorySources)
	}
	if gate.keys["new"] == nil {
		t.Fatal("new key was not admitted")
	}
	removed := ""
	for i := 0; i < maxMemorySources; i++ {
		key := "live:" + strconv.Itoa(i)
		if gate.keys[key] == nil {
			if removed != "" {
				t.Fatalf("keys %q and %q both removed, want one", removed, key)
			}
			removed = key
		}
	}
	if removed == "" {
		t.Fatal("no live key removed at the cap")
	}
}

// TestAllowDenyAndReportMoveToTailExpiryList: Allow deny and Report move the entry to the tail.
func TestAllowDenyAndReportMoveToTailExpiryList(t *testing.T) {
	gate := newTestGate(t)
	now := time.Unix(1_700_000_000, 0)
	gate.SetNowForTest(func() time.Time { return now })
	for _, key := range []string{"a", "b", "c", "d"} {
		storeEntryForTestExpiryList(gate, key, &memEntry{
			credit:      gate.budget,
			state:       stateClosed,
			closedSince: now,
			expireAt:    now.Add(gate.cfg.TTL),
		})
	}
	gate.mu.Lock()
	gate.keys["a"].state = stateOpen
	gate.keys["a"].openUntil = now.Add(time.Second)
	gate.mu.Unlock()

	p, _, err := gate.Allow("a")
	if err != nil {
		t.Fatal(err)
	}
	if p.Allowed() {
		t.Fatal("OPEN key must deny")
	}
	if got := strings.Join(expiryListKeysExpiryList(t, gate), ","); got != "b,c,d,a" {
		t.Fatalf("after Allow deny list %s, want b,c,d,a", got)
	}

	p, _, err = gate.Allow("b")
	if err != nil || !p.Allowed() {
		t.Fatalf("admit b: allowed=%v err=%v", p.Allowed(), err)
	}
	if err := gate.Report(p, true); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(expiryListKeysExpiryList(t, gate), ","); got != "c,d,a,b" {
		t.Fatalf("after Report list %s, want c,d,a,b", got)
	}
}

// TestCloseResetsListExpiryList: Close nils head/tail/keys, and a new Gate's list stays consistent with its map.
func TestCloseResetsListExpiryList(t *testing.T) {
	gate := newTestGate(t)
	now := time.Unix(1_700_000_000, 0)
	storeEntryForTestExpiryList(gate, "a", &memEntry{
		credit:      gate.budget,
		state:       stateClosed,
		closedSince: now,
		expireAt:    now.Add(gate.cfg.TTL),
	})
	storeEntryForTestExpiryList(gate, "b", &memEntry{
		credit:      gate.budget,
		state:       stateClosed,
		closedSince: now,
		expireAt:    now.Add(gate.cfg.TTL),
	})
	if got := strings.Join(expiryListKeysExpiryList(t, gate), ","); got != "a,b" {
		t.Fatalf("before Close list %s, want a,b", got)
	}

	gate.Close()
	if keys := expiryListKeysExpiryList(t, gate); keys != nil {
		t.Fatalf("after Close list %v, want empty", keys)
	}

	reused := newTestGate(t)
	storeEntryForTestExpiryList(reused, "x", &memEntry{
		credit:      reused.budget,
		state:       stateClosed,
		closedSince: now,
		expireAt:    now.Add(reused.cfg.TTL),
	})
	storeEntryForTestExpiryList(reused, "y", &memEntry{
		credit:      reused.budget,
		state:       stateClosed,
		closedSince: now,
		expireAt:    now.Add(reused.cfg.TTL),
	})
	if got := strings.Join(expiryListKeysExpiryList(t, reused), ","); got != "x,y" {
		t.Fatalf("reused Gate list %s, want x,y", got)
	}
}
