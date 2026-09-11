package tokenbucket

import (
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
func (m *Memory) Allow(key string) (bool, time.Duration, error) {
	now := m.now()
	nowMicro := now.UnixMicro()
	m.mu.Lock()
	defer m.mu.Unlock()
	// Idle longer than ttl is a new bucket (Traefik TTL map).
	entry := m.buckets[key]
	if entry == nil || !now.Before(entry.expireAt) {
		entry = &memEntry{}
		m.buckets[key] = entry
	}
	tokens, last, waitMicro := consumeOne(entry.tokens, entry.last, m.clock.limitPerMicro(), float64(m.clock.burst), nowMicro, m.clock.maxDelay.Microseconds())
	entry.tokens = tokens
	entry.last = last
	entry.expireAt = now.Add(m.clock.ttl)
	wait := waitDuration(waitMicro)
	return allowedFromWait(wait, m.clock.maxDelay), wait, nil
}
