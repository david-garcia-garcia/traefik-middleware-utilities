package tokenbucket

import (
	"context"
	"sync"
	"time"
)

// memEntry is one in-memory source bucket, its lazy expire time, and its place in Memory's expiry list.
type memEntry struct {
	tokens   float64
	last     int64
	expireAt time.Time

	// key, prev, and next link the entry into the expiry list; key is the buckets map key, so the list head can be deleted from the map.
	key  string
	prev *memEntry
	next *memEntry
}

// Memory is an in-process Traefik token bucket (Lua formulas, not x/time/rate).
type Memory struct {
	clock clockConfig
	now   func() time.Time

	mu      sync.Mutex
	buckets map[string]*memEntry
	// head and tail are the expiry list over buckets. Every Allow sets expireAt = now + ttl (ttl is fixed) and moves its
	// entry to the tail, so the head expires first while the clock does not move backwards. Concrete pointers, not
	// container/list: its any-typed Value is a Yaegi risk.
	head *memEntry
	tail *memEntry
}

// maxMemorySources is Traefik's in-memory source cap (ttlmap maxSources).
const maxMemorySources = 65536

// NewMemory builds an in-process limiter. rate is requests per second.
func NewMemory(rate float64, burst int64, maxDelay, ttl time.Duration) (*Memory, error) {
	if err := validateClock(rate, burst, maxDelay, ttl); err != nil {
		return nil, err
	}
	return &Memory{
		clock: clockConfig{
			rate:     rate,
			burst:    burst,
			maxDelay: maxDelay,
			ttl:      ttl,
			period:   refillPeriod(rate, burst),
		},
		now:     time.Now,
		buckets: map[string]*memEntry{},
	}, nil
}

// SetNowForTest replaces the clock. Production callers must not use this.
func (m *Memory) SetNowForTest(now func() time.Time) {
	if now == nil {
		m.now = time.Now
		return
	}
	m.now = now
}

// Allow consumes one token for key and returns its Status: whether it is allowed, the wait as Delay or RetryAfter, and the
// bucket after this consume. The Status is the zero value when the error is non-nil.
func (m *Memory) Allow(ctx context.Context, key string) (Status, error) {
	if err := ctx.Err(); err != nil {
		return Status{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	nowMicro := now.UnixMicro()
	// Idle longer than ttl is a new bucket (Traefik TTL map).
	entry := m.buckets[key]
	if entry != nil && !now.Before(entry.expireAt) {
		m.removeLocked(entry)
		entry = nil
	}
	if entry == nil {
		// Drop expired sources (and one live source at the cap) so the map cannot grow without bound.
		m.makeRoomLocked(now)
		// Missing or TTL-expired key is a full bucket, then consumeOne.
		entry = &memEntry{tokens: float64(m.clock.burst), last: nowMicro, key: key}
		m.storeLocked(entry)
	} else {
		// Its expireAt is about to become the latest, so it moves to the tail.
		m.moveToBackLocked(entry)
	}
	tokens, last, waitMicro := consumeOne(entry.tokens, entry.last, m.clock.limitPerMicro(), float64(m.clock.burst), nowMicro, m.clock.maxDelay.Microseconds())
	entry.tokens = tokens
	entry.last = last
	entry.expireAt = now.Add(m.clock.ttl)
	return newStatus(m.clock, tokens, waitMicro), nil
}

// makeRoomLocked removes expired sources from the head of the expiry list, then the head itself when the store is
// still at maxMemorySources, so a new source can be stored. Caller holds m.mu.
func (m *Memory) makeRoomLocked(now time.Time) {
	for m.head != nil && !now.Before(m.head.expireAt) {
		m.removeLocked(m.head)
	}
	if len(m.buckets) >= maxMemorySources && m.head != nil {
		m.removeLocked(m.head)
	}
}

// storeLocked adds a new source to the map and at the tail of the expiry list. Caller holds m.mu.
func (m *Memory) storeLocked(entry *memEntry) {
	m.buckets[entry.key] = entry
	m.pushBackLocked(entry)
}

// removeLocked deletes a stored source from the map and the expiry list. Caller holds m.mu.
func (m *Memory) removeLocked(entry *memEntry) {
	m.unlinkLocked(entry)
	delete(m.buckets, entry.key)
}

// unlinkLocked takes a linked entry out of the expiry list. Caller holds m.mu.
func (m *Memory) unlinkLocked(entry *memEntry) {
	if entry.prev != nil {
		entry.prev.next = entry.next
	} else {
		m.head = entry.next
	}
	if entry.next != nil {
		entry.next.prev = entry.prev
	} else {
		m.tail = entry.prev
	}
	entry.prev = nil
	entry.next = nil
}

// moveToBackLocked moves a linked entry to the tail of the expiry list. The tail itself stays put, so a hot key costs no
// relink. Caller holds m.mu.
//
// It relinks in place instead of calling unlinkLocked then pushBackLocked: this runs on every Allow of a stored source that
// is not the tail, and under Yaegi each method call costs microseconds (BenchmarkYaegiMemoryAllow/rotating-10k).
func (m *Memory) moveToBackLocked(entry *memEntry) {
	if entry == m.tail {
		return
	}
	// Unlink. entry is not the tail, so entry.next is set.
	if entry.prev != nil {
		entry.prev.next = entry.next
	} else {
		m.head = entry.next
	}
	entry.next.prev = entry.prev
	// Append after the current tail.
	entry.prev = m.tail
	entry.next = nil
	m.tail.next = entry
	m.tail = entry
}

// pushBackLocked appends an unlinked entry at the tail of the expiry list. Caller holds m.mu.
func (m *Memory) pushBackLocked(entry *memEntry) {
	entry.prev = m.tail
	entry.next = nil
	if m.tail != nil {
		m.tail.next = entry
	} else {
		m.head = entry
	}
	m.tail = entry
}
