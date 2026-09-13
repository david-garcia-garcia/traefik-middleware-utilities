package tokenbucket

import (
	"context"
	"sync"
	"time"
)

// memEntry is one in-memory source bucket plus its lazy expire time.
type memEntry struct {
	tokens   float64
	last     int64
	expireAt time.Time
}

// Memory is an in-process Traefik token bucket (Lua formulas, not x/time/rate).
type Memory struct {
	clock clockConfig
	now   func() time.Time

	mu      sync.Mutex
	buckets map[string]*memEntry
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

// Allow consumes one token for key and returns whether it is allowed plus how long to wait.
func (m *Memory) Allow(ctx context.Context, key string) (bool, time.Duration, error) {
	if err := ctx.Err(); err != nil {
		return false, 0, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	nowMicro := now.UnixMicro()
	// Idle longer than ttl is a new bucket (Traefik TTL map). Drop expired keys so the map cannot grow without bound.
	entry := m.buckets[key]
	if entry != nil && !now.Before(entry.expireAt) {
		delete(m.buckets, key)
		entry = nil
	}
	if entry == nil {
		m.dropExpired(now)
		if len(m.buckets) >= maxMemorySources {
			m.dropOne(now)
		}
		entry = &memEntry{}
		m.buckets[key] = entry
	}
	tokens, last, waitMicro := consumeOne(entry.tokens, entry.last, m.clock.limitPerMicro(), float64(m.clock.burst), nowMicro, m.clock.maxDelay.Microseconds())
	entry.tokens = tokens
	entry.last = last
	entry.expireAt = now.Add(m.clock.ttl)
	// Admit uses whole microseconds, same units as consumeOne refund and Lua ARGV.
	allowed := waitMicro <= float64(m.clock.maxDelay.Microseconds())
	wait := time.Duration(0)
	if waitMicro > 0 {
		wait = time.Duration(waitMicro * float64(time.Microsecond))
	}
	return allowed, wait, nil
}

// dropExpired removes buckets whose ttl has elapsed.
func (m *Memory) dropExpired(now time.Time) {
	for source, entry := range m.buckets {
		if !now.Before(entry.expireAt) {
			delete(m.buckets, source)
		}
	}
}

// dropOne removes one map slot when at cap so a new source can be stored.
func (m *Memory) dropOne(now time.Time) {
	m.dropExpired(now)
	if len(m.buckets) < maxMemorySources {
		return
	}
	for source := range m.buckets {
		delete(m.buckets, source)
		return
	}
}
